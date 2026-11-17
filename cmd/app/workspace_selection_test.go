package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsdirpicker"
)

func TestBlankSessionInWorkspaceUsesOnlyConfirmedBlank(t *testing.T) {
	var mu sync.Mutex
	var loads []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session.load" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		id, _ := req["sessionId"].(string)
		mu.Lock()
		loads = append(loads, id)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if id == "blank" {
			_, _ = w.Write([]byte(`{"result":{"id":"blank","cwd":"C:\\work","messages":[],"total":0}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":{"id":"history","cwd":"C:\\work","messages":[{"role":"user","text":"old"}],"total":1}}`))
	}))
	t.Cleanup(srv.Close)
	m := &model{cli: apiclient.New(srv.URL)}
	workspace := pluginapi.Workspace{WorkspaceID: "w1", Path: `C:\work`, SessionIDs: []string{"history", "blank"}}
	id, reused, err := m.blankSessionInWorkspace(context.Background(), workspace, nil)
	if err != nil || !reused || id != "blank" {
		t.Fatalf("应复用宿主证实 Total=0 的 blank，实际 id=%q reused=%v err=%v", id, reused, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(loads) != 2 || loads[0] != "history" || loads[1] != "blank" {
		t.Fatalf("候选顺序应跟随 registry.sessionIds，实际 %v", loads)
	}
}

// 没有可复用的空白成员会话是**正常情况**：官方这时发一次 create
// （`ui-workspace/src/client/navigation.ts:245-250`），不该把"这个工作区还没会话"
// 渲染成错误。这条也只许读：建会话是 selectWorkspace 的下一步。
func TestBlankSessionInWorkspaceNoCandidateIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/session.create" {
			t.Fatal("这一步只查空白候选，不能顺手建会话")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"id":"archived","cwd":"C:\\work","messages":[],"total":0}}`))
	}))
	t.Cleanup(srv.Close)
	m := &model{cli: apiclient.New(srv.URL)}
	workspace := pluginapi.Workspace{WorkspaceID: "w1", Path: `C:\work`, SessionIDs: []string{"archived"}}
	id, reused, err := m.blankSessionInWorkspace(context.Background(), workspace, map[string]bool{"archived": true})
	if err != nil || reused || id != "" {
		t.Fatalf("归档空白会话不能复用；没有候选不是错误：id=%q reused=%v err=%v", id, reused, err)
	}
}

// fakeHostForSelect 是 selectWorkspace / createSessionIn* 需要的宿主替身。
//
// create 的入参原样记下来（断言"客户端发了什么"），load 按 id 回一份 cwd 真值
// （断言"宿主读回什么"）。两个方向都是协议事实，不看实现细节。
type fakeHostForSelect struct {
	mu       sync.Mutex
	creates  []map[string]any
	status   int    // create 的 HTTP 状态（0 = 200）
	raw      string // 非空 = create 原样回这段响应体
	loadCwd  map[string]string
	listBody string // 非空 = session.list 原样回这段（refreshList 用）
}

func (h *fakeHostForSelect) handler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch r.URL.Path {
		case "/api/session.create":
			h.mu.Lock()
			h.creates = append(h.creates, body)
			status, raw := h.status, h.raw
			h.mu.Unlock()
			if raw != "" {
				if status >= 400 {
					w.WriteHeader(status)
				}
				_, _ = w.Write([]byte(raw))
				return
			}
			_, _ = w.Write([]byte(`{"result":{"sessionId":"fresh"}}`))
		case "/api/session.load":
			id, _ := body["sessionId"].(string)
			h.mu.Lock()
			cwd := h.loadCwd[id]
			h.mu.Unlock()
			out, _ := json.Marshal(map[string]any{"result": map[string]any{
				"id": id, "cwd": cwd, "messages": []any{}, "total": 0,
			}})
			_, _ = w.Write(out)
		case "/api/session.list":
			h.mu.Lock()
			list := h.listBody
			h.mu.Unlock()
			if list == "" {
				list = `{"result":{"sessions":[]}}`
			}
			_, _ = w.Write([]byte(list))
		case "/api/config.get":
			// 与产品同语义：拉不到就用默认值，不要启动周期 ticker。
			w.WriteHeader(http.StatusInternalServerError)
		default:
			_, _ = w.Write([]byte(`{"result":{}}`))
		}
	})
}

// TestSelectWorkspaceCreatesWithOfficialWorkspaceID 钉官方组合规则：
// workspaceId 给了就**连 cwd 都不发**（`manager.ts:498-504`），也不发 title。
func TestSelectWorkspaceCreatesWithOfficialWorkspaceID(t *testing.T) {
	host := &fakeHostForSelect{loadCwd: map[string]string{"fresh": `C:\new`}}
	srv := httptest.NewServer(host.handler(t))
	t.Cleanup(srv.Close)
	m := &model{
		cli:        apiclient.New(srv.URL),
		mux:        apiclient.NewMux(srv.URL),
		cache:      map[string]*cachedSession{},
		seqs:       map[string]int{},
		epochs:     map[string]int64{},
		workspaces: []pluginapi.Workspace{{WorkspaceID: "w-new", Path: `C:\new`}},
	}
	cancelSubscriptionsOnCleanup(t, m)
	m.selectWorkspace("w-new")
	waitFor(t, "没有打开宿主新建的会话", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.cur == "fresh"
	})

	host.mu.Lock()
	defer host.mu.Unlock()
	if len(host.creates) != 1 {
		t.Fatalf("选中一个没有空白会话的工作区应当恰好建一次会话，实际 %d 次", len(host.creates))
	}
	body := host.creates[0]
	if body["workspaceId"] != "w-new" {
		t.Fatalf("session.create.workspaceId = %v，want w-new（官方 SessionCreateRequest）", body["workspaceId"])
	}
	if _, ok := body["cwd"]; ok {
		t.Fatalf("给了 workspaceId 就不能再发 cwd（manager.ts:498-504 的互斥组合）：%v", body)
	}
	if _, ok := body["title"]; ok {
		t.Fatalf("官方 SessionCreateRequest 没有 title，不该发：%v", body)
	}
}

// TestSelectWorkspaceReportsHostIgnoringWorkspaceID 钉"老宿主静默忽略 workspaceId"：
// 宿主照建不误、但会话落在进程 cwd —— 那是"收下却没照做"，必须如实报错，不许当成功。
func TestSelectWorkspaceReportsHostIgnoringWorkspaceID(t *testing.T) {
	host := &fakeHostForSelect{loadCwd: map[string]string{"fresh": `/tmp/process-cwd`}}
	srv := httptest.NewServer(host.handler(t))
	t.Cleanup(srv.Close)
	m := &model{
		cli:        apiclient.New(srv.URL),
		mux:        apiclient.NewMux(srv.URL),
		cache:      map[string]*cachedSession{},
		seqs:       map[string]int{},
		epochs:     map[string]int64{},
		workspaces: []pluginapi.Workspace{{WorkspaceID: "w-new", Path: `C:\new`}},
	}
	cancelSubscriptionsOnCleanup(t, m)
	m.selectWorkspace("w-new")
	waitFor(t, "宿主没按 workspaceId 建会话时必须如实报错", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.workspaceSelectionPending == "" && strings.Contains(m.status, errWorkspaceSessionCwdMismatch.Error())
	})
}

// TestCreateSessionInWorkspaceExposesPublishedButUnattached 钉官方"发布先于挂载"：
// `session/workspace-attach-failed` 的 details.sessionId 是一份**真实存在**的会话，
// 客户端把它当 Ungrouped 暴露（`manager.ts:505-523,855-857`；本仓宿主把官方码放在
// message 前缀里，`HOST:docs/transport-contract.md` §18）。
func TestCreateSessionInWorkspaceExposesPublishedButUnattached(t *testing.T) {
	host := &fakeHostForSelect{
		status: 500,
		raw: `{"error":{"code":"internal","message":"session/workspace-attach-failed: ` +
			`session \\"published-1\\" could not attach to workspace \\"w-a\\""},` +
			`"details":{"sessionId":"published-1","workspaceId":"w-a"}}`,
		listBody: `{"result":{"sessions":[]}}`,
	}
	srv := httptest.NewServer(host.handler(t))
	t.Cleanup(srv.Close)
	m := &model{cli: apiclient.New(srv.URL)}
	_, err := m.createSessionInWorkspace(context.Background(),
		pluginapi.Workspace{WorkspaceID: "w-a", Path: `C:\a`, Title: "a"})
	if err == nil {
		t.Fatal("attach 失败不能当成成功")
	}
	if !strings.Contains(err.Error(), "published-1") {
		t.Fatalf("错误里要带上那份已发布的会话 id（已当 Ungrouped 暴露）：%v", err)
	}
}

// TestCreateSessionInWorkspaceRejectsEmptySessionID 钉"成功却没有 id"：
// 拿空串去 session.load 只会变成"会话不存在"，现场全丢，所以必须响亮失败。
func TestCreateSessionInWorkspaceRejectsEmptySessionID(t *testing.T) {
	host := &fakeHostForSelect{raw: `{"result":{}}`}
	srv := httptest.NewServer(host.handler(t))
	t.Cleanup(srv.Close)
	m := &model{cli: apiclient.New(srv.URL)}
	if _, err := m.createSessionInWorkspace(context.Background(),
		pluginapi.Workspace{WorkspaceID: "w-a", Path: `C:\a`}); err == nil {
		t.Fatal("宿主没给 sessionId 时必须报错")
	}
}

// TestCreateSessionInDirectorySendsCwd 钉官方 `{cwd}`：没有工作区时会话自己拥有该目录
// （`types.ts:271-273`）。
func TestCreateSessionInDirectorySendsCwd(t *testing.T) {
	host := &fakeHostForSelect{loadCwd: map[string]string{"fresh": `C:\picked`}}
	srv := httptest.NewServer(host.handler(t))
	t.Cleanup(srv.Close)
	m := &model{cli: apiclient.New(srv.URL)}
	id, err := m.createSessionInDirectory(context.Background(), `C:\picked`)
	if err != nil || id != "fresh" {
		t.Fatalf("按 {cwd} 建会话失败：id=%q err=%v", id, err)
	}
	host.mu.Lock()
	defer host.mu.Unlock()
	body := host.creates[0]
	if body["cwd"] != `C:\picked` {
		t.Fatalf("session.create.cwd = %v，want C:\\picked", body["cwd"])
	}
	if _, ok := body["workspaceId"]; ok {
		t.Fatalf("没有工作区时不能发 workspaceId：%v", body)
	}
}

// TestDirSourceFallsBackToCwdWhenRegistryUnsupported 钉目录选择器的 `{cwd}` 接线：
// 宿主没有工作区注册表（501 not_implemented）时，壳按官方 `{cwd}` 让会话拥有该目录，
// 并告诉对话框"已处理"（不是失败）。
func TestDirSourceFallsBackToCwdWhenRegistryUnsupported(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/workspace.create" {
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"workspace.create 未接线"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	t.Cleanup(srv.Close)
	src := dirSource{cli: apiclient.New(srv.URL), pickDirectory: func(path string) { gotPath = path }}
	_, err := src.Register(context.Background(), `C:\picked`)
	if !errors.Is(err, dsdirpicker.ErrPickedHandled) {
		t.Fatalf("注册表不可用时应回落成 {cwd} 并报 ErrPickedHandled，实际 %v", err)
	}
	if gotPath != `C:\picked` {
		t.Fatalf("回落动作没拿到选中的目录：%q", gotPath)
	}
}

// TestDirSourceKeepsHostErrorWhenRegistryFailsForThePath 钉"只有注册表不可用才回落"：
// 其余失败（bad_request / not_found）是这次路径的问题，必须把宿主原文交回去。
func TestDirSourceKeepsHostErrorWhenRegistryFailsForThePath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"bad_request","message":"path 不是绝对路径"}}`))
	}))
	t.Cleanup(srv.Close)
	fellBack := false
	src := dirSource{cli: apiclient.New(srv.URL), pickDirectory: func(string) { fellBack = true }}
	_, err := src.Register(context.Background(), `C:\picked`)
	if err == nil || errors.Is(err, dsdirpicker.ErrPickedHandled) {
		t.Fatalf("非注册表失败不能回落，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "path 不是绝对路径") {
		t.Fatalf("宿主原文必须原样交出：%v", err)
	}
	if fellBack {
		t.Fatal("非注册表失败不该触发 {cwd} 回落")
	}
}

// cancelSubscriptionsOnCleanup 在用例结束时取消 openSession 建出来的订阅，
// 免得 WS 重连 goroutine 活到下一个用例（与 session_start_test 同一套收尾）。
func cancelSubscriptionsOnCleanup(t *testing.T, m *model) {
	t.Helper()
	t.Cleanup(func() {
		m.mu.Lock()
		for _, sub := range m.subs {
			if sub.cancel != nil {
				sub.cancel()
			}
		}
		m.mu.Unlock()
	})
}
