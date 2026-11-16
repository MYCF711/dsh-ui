// dirpicker_wiring_test.go —— 壳按宿主组合**二选一**接线的判据。
//
// 这里钉四件事（对应 `cmd/app/dirpicker_wiring.go` 的三种结局）：
//
//  1. 探到 browse ⇒ 走现有 `plugins/dsdirpicker` 路径（数据源注入了，pick 一次不碰）；
//  2. 探到 native ⇒ 点入口直接 `directoryPicker.pick`，选到就 `workspace.create`；
//     取消（官方 null）**静默**，失败把宿主原文交给状态栏；
//  3. 探针失败 ⇒ 动作是 **nil**（侧栏据此整块隐藏入口），不退化到某一套；
//  4. 探针**只在启动时打一次**：之后再读动作/再打开都不再发 `directoryPicker.list`。
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/plugins/dsdirpicker"
)

// dirPickerHost 是假宿主：按 kind 回答 directoryPicker 三个动词 + workspace.create。
type dirPickerHost struct {
	mu      sync.Mutex
	kind    string // browse / native
	pick    string // pick 的响应体（默认 {"result":"C:\\picked"}）
	status  int    // pick 的状态码（0 = 200），用来演失败
	fail    string // pick 失败时的 message
	calls   []string
	created []string // workspace.create 收到的 path
}

func (h *dirPickerHost) record(m string) {
	h.mu.Lock()
	h.calls = append(h.calls, m)
	h.mu.Unlock()
}

func (h *dirPickerHost) count(m string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, c := range h.calls {
		if c == m {
			n++
		}
	}
	return n
}

func (h *dirPickerHost) createdPaths() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.created))
	copy(out, h.created)
	return out
}

func (h *dirPickerHost) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, "/api/")
		h.record(method)
		w.Header().Set("Content-Type", "application/json")
		browse := h.kind == "browse"
		switch method {
		case "directoryPicker.list":
			if browse {
				_, _ = w.Write([]byte(`{"result":{"path":"C:\\u","home":"C:\\u","crumbs":[],"entries":[],"truncated":false}}`))
				return
			}
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"directory-picker/unavailable: directoryPicker.list needs the browse capability; the composed picker serves \"native\"","hint":"","retryable":false},"details":{"capability":"native"}}`))
		case "directoryPicker.pick":
			if h.status != 0 {
				w.WriteHeader(h.status)
				body, _ := json.Marshal(map[string]any{"error": map[string]any{
					"code": "internal", "message": h.fail, "hint": "", "retryable": false,
				}})
				_, _ = w.Write(body)
				return
			}
			body := h.pick
			if body == "" {
				body = `{"result":"C:\\picked"}`
			}
			_, _ = w.Write([]byte(body))
		case "workspace.create":
			var req struct {
				Path string `json:"path"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			h.mu.Lock()
			h.created = append(h.created, req.Path)
			h.mu.Unlock()
			_, _ = w.Write([]byte(`{"result":{"workspace":{"workspaceId":"ws-picked","path":"C:\\picked","title":"picked","sessionIds":[],"createdAt":"1","updatedAt":"1"},"created":true}}`))
		default:
			// refreshList 那一族（老壳会顺手拉）与其它方法：给个空对象，别把测试挂住。
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newDirPickerModel 造一个"装配到一半"的壳：有 cli、没有 kernel（本组用例只走
// 组合判定与 native 那条直连路径，不需要插件树）。
func newDirPickerModel(t *testing.T, srv *httptest.Server) *model {
	t.Helper()
	// dsdirpicker 的数据源/回调是**包级**的，用例之间必须互相收干净。
	t.Cleanup(func() {
		dsdirpicker.SetSource(nil)
		dsdirpicker.SetOnPicked(nil)
	})
	return &model{cli: apiclient.New(srv.URL)}
}

// TestWireDirPickerBrowseKeepsExistingPath：200 ⇒ browse，走现有对话框路径。
func TestWireDirPickerBrowseKeepsExistingPath(t *testing.T) {
	host := &dirPickerHost{kind: "browse"}
	srv := host.serve(t)
	m := newDirPickerModel(t, srv)

	wireDirPicker(m)

	if m.dirPicker.mode != dirPickerBrowse {
		t.Fatalf("探到 browse 后 mode = %v，期望 dirPickerBrowse", m.dirPicker.mode)
	}
	if m.dirPicker.err != nil {
		t.Fatalf("browse 组合不该有探针错误：%v", m.dirPicker.err)
	}
	// 数据源真的注入了：对话框能打开（Open 返回 true 的唯一判据就是数据源非 nil）。
	p := &dsdirpicker.Plugin{}
	if !p.Open() {
		t.Fatal("browse 组合下 dsdirpicker 应当有数据源（Open 必须返回 true）")
	}
	p.Close()
	// 探针与这条路径都**不许碰 pick**（那会真弹框）。
	if n := host.count("directoryPicker.pick"); n != 0 {
		t.Fatalf("browse 组合下调了 pick %d 次（会真弹宿主原生框）", n)
	}
	// 探针只在启动时打一次。
	if n := host.count("directoryPicker.list"); n != 1 {
		t.Fatalf("启动时应当只探一次（directoryPicker.list 打了 %d 次）", n)
	}
	if m.dirPickerAction() == nil {
		t.Fatal("browse 组合下动作不能是 nil（否则入口被隐藏）")
	}
	if n := host.count("directoryPicker.list"); n != 1 {
		t.Fatalf("读动作不该再探一次（directoryPicker.list 打了 %d 次）", n)
	}
}

// TestWireDirPickerNativeRoutesToPick：501+capability=native ⇒ native，
// 点入口 = pick → workspace.create → 采用。
func TestWireDirPickerNativeRoutesToPick(t *testing.T) {
	host := &dirPickerHost{kind: "native"}
	srv := host.serve(t)
	m := newDirPickerModel(t, srv)

	wireDirPicker(m)

	if m.dirPicker.mode != dirPickerNative {
		t.Fatalf("探到 native 后 mode = %v，期望 dirPickerNative", m.dirPicker.mode)
	}
	if m.dirPickerAction() == nil {
		t.Fatal("native 组合下动作不能是 nil（入口要画出来）")
	}
	// native 组合**没有应用内对话框**：不该给 dsdirpicker 注入数据源，否则会画出
	// 一个 list 必被宿主 501 拒的浏览器。
	if (&dsdirpicker.Plugin{}).Open() {
		t.Fatal("native 组合不该给 browse 对话框注入数据源")
	}
	if n := host.count("directoryPicker.pick"); n != 0 {
		t.Fatalf("接线阶段不许碰 pick（探针禁用 pick），实际 %d 次", n)
	}

	var picked []apiclient.WorkspaceView
	m.dirPicker.onPicked = func(workspace apiclient.WorkspaceView) { picked = append(picked, workspace) }
	m.pickNativeDirectory()

	if got := host.createdPaths(); len(got) != 1 || got[0] != `C:\picked` {
		t.Fatalf("workspace.create 收到的路径 = %v，期望 [C:\\picked]", got)
	}
	if len(picked) != 1 || picked[0].WorkspaceID != "ws-picked" || picked[0].Path != `C:\picked` {
		t.Fatalf("onPicked 收到 %+v，期望官方返回的 ws-picked/C:\\picked", picked)
	}
	// 顺序也要对：pick 在 workspace.create 之前。
	if host.count("directoryPicker.pick") != 1 {
		t.Fatalf("pick 应当恰好一次，实际 %d", host.count("directoryPicker.pick"))
	}
	if n := host.count("directoryPicker.list"); n != 1 {
		t.Fatalf("打开选择器不该重新探针（list 打了 %d 次）", n)
	}
}

// TestWireDirPickerNativeCancelIsSilent：官方 null = 操作员取消 —— 不采用、
// 不报错、不打扰（`OFF:dpn/src/client/flow.ts:63-66` 的 onCancel）。
func TestWireDirPickerNativeCancelIsSilent(t *testing.T) {
	host := &dirPickerHost{kind: "native", pick: `{"result":null}`}
	srv := host.serve(t)
	m := newDirPickerModel(t, srv)
	wireDirPicker(m)

	var picked []apiclient.WorkspaceView
	m.dirPicker.onPicked = func(workspace apiclient.WorkspaceView) { picked = append(picked, workspace) }
	m.pickNativeDirectory()

	if len(host.createdPaths()) != 0 {
		t.Fatalf("取消不该建工作区，实际 %v", host.createdPaths())
	}
	if len(picked) != 0 {
		t.Fatalf("取消不该回调 onPicked，实际 %v", picked)
	}
	m.mu.Lock()
	status, loading := m.status, m.loading
	m.mu.Unlock()
	if status != "" || loading {
		t.Fatalf("取消是静默的，不该写状态（status=%q loading=%v）", status, loading)
	}
}

// TestWireDirPickerNativeErrorKeepsHostText：失败走官方 `onError(message)`，
// 宿主原文进状态栏，不另编文案。
func TestWireDirPickerNativeErrorKeepsHostText(t *testing.T) {
	const hostMsg = "原生目录选择器不可用：当前进程没有交互桌面"
	host := &dirPickerHost{kind: "native", status: http.StatusInternalServerError, fail: hostMsg}
	srv := host.serve(t)
	m := newDirPickerModel(t, srv)
	wireDirPicker(m)

	var picked []apiclient.WorkspaceView
	m.dirPicker.onPicked = func(workspace apiclient.WorkspaceView) { picked = append(picked, workspace) }
	m.pickNativeDirectory()

	m.mu.Lock()
	status := m.status
	m.mu.Unlock()
	if !strings.Contains(status, hostMsg) {
		t.Fatalf("状态栏里应当是宿主原文 %q，实际 %q", hostMsg, status)
	}
	if len(picked) != 0 || len(host.createdPaths()) != 0 {
		t.Fatalf("失败路径不许采用任何目录（onPicked=%v created=%v）", picked, host.createdPaths())
	}
}

// TestWireDirPickerUnavailableHidesEntry：探针失败 ⇒ 动作 **nil** ⇒ 侧栏整块隐藏。
//
// 这是"不许静默降级"的机械判据：`plugins/dssidebar/frame.go` 只在
// `env.Act.OpenDirectoryPicker != nil` 时画那一枚按钮，所以 nil 就是"不出现"。
func TestWireDirPickerUnavailableHidesEntry(t *testing.T) {
	// mockd / 老宿主那种 404 unsupported：既不是 200 也不是官方那条 501。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"unsupported","message":"未知 method: directoryPicker.list"}}`))
	}))
	t.Cleanup(srv.Close)
	m := newDirPickerModel(t, srv)

	wireDirPicker(m)

	if m.dirPicker.mode != dirPickerUnavailable {
		t.Fatalf("探针失败时 mode = %v，期望 dirPickerUnavailable", m.dirPicker.mode)
	}
	if m.dirPicker.err == nil {
		t.Fatal("探针失败必须留下原因（如实报错）")
	}
	if m.dirPickerAction() != nil {
		t.Fatal("一套后端都不可用时动作必须是 nil（入口整块隐藏），不许画死按钮")
	}
	if m.openDirectoryPicker() {
		t.Fatal("不可用时打开必须返回 false")
	}
	// 也不许偷偷给 browse 对话框注数据源（那等于退化成默认）。
	if (&dsdirpicker.Plugin{}).Open() {
		t.Fatal("不可用时不该给 dsdirpicker 注入数据源（不许静默退化成 browse）")
	}
}

// TestNativeDirectoryPickRearmsOncePerOpen 钉住官方 armed ref：在飞期间重复点击
// 不启动第二个原生框；选中结算后下一次点击才允许重新打开。
func TestNativeDirectoryPickRearmsOncePerOpen(t *testing.T) {
	started := make(chan struct{}, 2)
	gates := make(chan chan struct{}, 2)
	gate1, gate2 := make(chan struct{}), make(chan struct{})
	gates <- gate1
	gates <- gate2
	var mu sync.Mutex
	picks := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch strings.TrimPrefix(r.URL.Path, "/api/") {
		case "directoryPicker.list":
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"directory-picker/unavailable: native","hint":"","retryable":false},"details":{"capability":"native"}}`))
		case "directoryPicker.pick":
			mu.Lock()
			picks++
			mu.Unlock()
			started <- struct{}{}
			gate := <-gates
			<-gate
			_, _ = w.Write([]byte(`{"result":"C:\\picked"}`))
		case "workspace.create":
			_, _ = w.Write([]byte(`{"result":{"workspace":{"workspaceId":"ws-picked","path":"C:\\picked","title":"picked","sessionIds":[]},"created":true}}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	m := newDirPickerModel(t, srv)
	wireDirPicker(m)
	m.dirPicker.onPicked = func(apiclient.WorkspaceView) {}

	if !m.openDirectoryPicker() {
		t.Fatal("第一次 open 应启动原生目录流")
	}
	<-started
	if m.openDirectoryPicker() {
		t.Fatal("open 尚未结算时不得启动第二个原生目录流")
	}
	mu.Lock()
	gotPicks := picks
	mu.Unlock()
	if gotPicks != 1 {
		t.Fatalf("在飞期间 pick 次数=%d，want 1", gotPicks)
	}
	close(gate1)
	waitNativePickerIdle(t, m)

	if !m.openDirectoryPicker() {
		t.Fatal("结算后应 re-arm，允许下一次 open")
	}
	<-started
	close(gate2)
	waitNativePickerIdle(t, m)
	mu.Lock()
	gotPicks = picks
	mu.Unlock()
	if gotPicks != 2 {
		t.Fatalf("两次独立 open 的 pick 总次数=%d，want 2", gotPicks)
	}
}

func waitNativePickerIdle(t *testing.T, m *model) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		m.mu.Lock()
		busy := m.dirPicker.nativeBusy
		m.mu.Unlock()
		if !busy {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("原生目录流结算后没有 re-arm")
		}
		time.Sleep(time.Millisecond)
	}
}
