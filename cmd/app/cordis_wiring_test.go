// cordis_wiring_test.go —— #12 壳侧数据面的四条判据（事件驱动 / 去重 / 失败保行 / 动词折算）。
//
// 这里钉的都是"错了也看不出来"的地方：
//
//	① 两条 cordis 事件帧必须**真的**触发重读（事件是唯一的事实变化信号，
//	   inventory 不带标签，官方也只能整份重读）；
//	② 同一帧重放/补页再来一次**不许**再读一次（seq 水位去重）；
//	③ 一次读失败**不许**清空已有清单（否则"宿主抖了一下"会显示成"什么都没定义"）；
//	④ stop 的 `not-running` 按官方折算成成功，`plugin-missing` 才是失败；
//	   remove 成功后那一帧行就消失（官方 `inventory.retire()`）。
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// cordisHost 是假宿主：按方法名回可变的响应体，并数调用次数。
type cordisHost struct {
	mu       sync.Mutex
	calls    map[string]int
	inventor string // inventory 的响应体
	stop     string // stopFromPanel 的响应体
	undefine string // undefineFromPanel 的响应体
}

func (h *cordisHost) count(method string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.calls[method]
}

func (h *cordisHost) setInventory(body string) {
	h.mu.Lock()
	h.inventor = body
	h.mu.Unlock()
}

func (h *cordisHost) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, "/api/")
		h.mu.Lock()
		if h.calls == nil {
			h.calls = map[string]int{}
		}
		h.calls[method]++
		inventory, stop, undefine := h.inventor, h.stop, h.undefine
		h.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch method {
		case apiclient.MethodCordisInventory:
			if inventory == "" {
				inventory = `{"result":[]}`
			}
			_, _ = w.Write([]byte(inventory))
		case apiclient.MethodCordisStopFromPanel:
			if stop == "" {
				stop = `{"result":{"ok":true}}`
			}
			_, _ = w.Write([]byte(stop))
		case apiclient.MethodCordisUndefineFromPanel:
			if undefine == "" {
				undefine = `{"result":{"ok":true,"wasRunning":true}}`
			}
			_, _ = w.Write([]byte(undefine))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newCordisModel(srv *httptest.Server) *model {
	return &model{cli: apiclient.New(srv.URL)}
}

// cordisPump 反复调 tick 直到条件成立或超时（**pump 到条件成立再断言**：
// 读是后台 goroutine，立刻断言就是和调度器赛跑）。
func cordisPump(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等不到条件成立：%s", what)
}

const cordisRowJSON = `{"result":[{"pluginId":"hello","agentId":"s-1","packages":[{"packageId":"pkg-1","name":"hello","purpose":"","hasHostHalf":true,"hasClientHalf":false}],"currentPackageId":"pkg-1","activeRun":{"pluginRunId":"run-1","packageId":"pkg-1"},"latestRun":{"pluginRunId":"run-1","packageId":"pkg-1","mode":"run","status":"running","host":{"status":"running","waitingFor":[]},"client":{"status":"absent","waitingFor":[]}}}]}`

// cordisTestSession / cordisTestEpoch 是"同一序号域"的默认取值。
//
// 水位按 `(sessionID, journalEpoch)` 分域（见 cordis_wiring.go 的 journalWatermark），
// 所以每条需要"同一域内 seq 可比"的用例都必须显式给出域标识；
// 传空 sessionID 时 consume 直接跳过（没有可信域就不记水位）。
const (
	cordisTestSession = "s-cordis"
	cordisTestEpoch   = int64(1)
)

// TestCordisViewNilWithoutClient：没有客户端（无头/测试）时整块不画（nil，不是空视图）。
func TestCordisViewNilWithoutClient(t *testing.T) {
	m := &model{}
	if view := m.cordisFor(cordisTestSession, cordisTestEpoch, nil); view != nil {
		t.Fatalf("没有 apiclient 时必须是 nil（插件据此整块不画），实得 %+v", view)
	}
}

// TestCordisFirstFrameReadsInventory：第一帧就触发一次读，读到之后 Known=true。
func TestCordisFirstFrameReadsInventory(t *testing.T) {
	host := &cordisHost{inventor: cordisRowJSON}
	m := newCordisModel(host.serve(t))

	first := m.cordisFor(cordisTestSession, cordisTestEpoch, nil)
	if first == nil || first.Known {
		t.Fatalf("第一帧应当是 Known=false 的视图（还没读到），实得 %+v", first)
	}
	cordisPump(t, "首帧读", func() bool { return host.count(apiclient.MethodCordisInventory) == 1 })
	cordisPump(t, "known 变真", func() bool { return m.cordisFor(cordisTestSession, cordisTestEpoch, nil).Known })

	view := m.cordisFor(cordisTestSession, cordisTestEpoch, nil)
	if len(view.Rows) != 1 || view.Rows[0].PluginID != "hello" {
		t.Fatalf("rows = %+v", view.Rows)
	}
	if view.Error != "" {
		t.Fatalf("成功读不该带 error：%q", view.Error)
	}
	// 已知之后再画帧不该重复读（每帧一次读 = 每 16ms 打一次宿主）。
	before := host.count(apiclient.MethodCordisInventory)
	for i := 0; i < 5; i++ {
		m.cordisFor(cordisTestSession, cordisTestEpoch, nil)
	}
	time.Sleep(20 * time.Millisecond)
	if got := host.count(apiclient.MethodCordisInventory); got != before {
		t.Fatalf("已知之后每帧仍在读：%d -> %d", before, got)
	}
}

// TestCordisEventFrameTriggersReread 是本条的主规格：
// 一条 `cordis/dynamic-package` 帧（WS → 归属会话 journal）必须触发一次重读，
// 并把它的 seq 记成水位；同一帧重放**不许**再读。
func TestCordisEventFrameTriggersReread(t *testing.T) {
	host := &cordisHost{inventor: cordisRowJSON}
	m := newCordisModel(host.serve(t))

	m.cordisFor(cordisTestSession, cordisTestEpoch, nil)
	cordisPump(t, "首帧读完成", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.cordis.known
	})
	before := host.count(apiclient.MethodCordisInventory)

	// 宿主 `emitFrameAt` 写进归属会话 journal 的就是这个形状。
	event := apiclient.Frame{Seq: 7, Type: cordisEventPackage}
	view := m.cordisFor(cordisTestSession, cordisTestEpoch, []apiclient.Frame{event})
	if view.EventSeq != 7 {
		t.Fatalf("EventSeq = %d，want 7（事件水位要能被界面看见）", view.EventSeq)
	}
	cordisPump(t, "事件触发重读", func() bool {
		return host.count(apiclient.MethodCordisInventory) == before+1
	})

	// 同一帧再来一次（补页/重放）：水位不变、不重复读。
	again := host.count(apiclient.MethodCordisInventory)
	m.cordisFor(cordisTestSession, cordisTestEpoch, []apiclient.Frame{event})
	time.Sleep(30 * time.Millisecond)
	if got := host.count(apiclient.MethodCordisInventory); got != again {
		t.Fatalf("重放帧又触发了一次读：%d -> %d", again, got)
	}
}

// TestCordisRetractEventAlsoTriggers：retract（stop/undefine 撤下）走同一条路。
func TestCordisRetractEventAlsoTriggers(t *testing.T) {
	host := &cordisHost{inventor: `{"result":[]}`}
	m := newCordisModel(host.serve(t))
	m.cordisFor(cordisTestSession, cordisTestEpoch, nil)
	cordisPump(t, "首帧读完成", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.cordis.known
	})
	before := host.count(apiclient.MethodCordisInventory)
	m.cordisFor(cordisTestSession, cordisTestEpoch, []apiclient.Frame{{Seq: 9, Type: cordisEventRetract}})
	cordisPump(t, "retract 触发重读", func() bool {
		return host.count(apiclient.MethodCordisInventory) == before+1
	})
}

// TestCordisReadFailureKeepsRows 钉住"读失败不清行"：
// 把一次失败画成"什么都没定义"，是这两件事里更坏的一种（官方 inventory.ts:88-99）。
func TestCordisReadFailureKeepsRows(t *testing.T) {
	host := &cordisHost{inventor: cordisRowJSON}
	m := newCordisModel(host.serve(t))
	m.cordisFor(cordisTestSession, cordisTestEpoch, nil)
	cordisPump(t, "首帧读完成", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.cordis.known
	})

	// 让下一次读失败（四字段错误信封）。
	host.setInventory(`{"error":{"code":"internal","message":"宿主正在重启","hint":"","retryable":true},"details":{}}`)
	m.cordisRefresh(true)
	cordisPump(t, "读失败被记下", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.cordis.err != ""
	})

	view := m.cordisFor(cordisTestSession, cordisTestEpoch, nil)
	if !view.Known {
		t.Fatal("一次读失败不许把 Known 打回 false（那会让面板退回「读取中」）")
	}
	if len(view.Rows) != 1 {
		t.Fatalf("读失败必须保留上一份清单，实得 %d 行", len(view.Rows))
	}
	if !strings.Contains(view.Error, "宿主正在重启") {
		t.Fatalf("error 里要有宿主原文，实得 %q", view.Error)
	}
}

// TestCordisStopTreatsNotRunningAsSuccess：官方 index.ts:46-51 的折算。
func TestCordisStopTreatsNotRunningAsSuccess(t *testing.T) {
	host := &cordisHost{
		inventor: `{"result":[]}`,
		stop:     `{"result":{"ok":false,"reason":"not-running","message":"插件没有在跑"}}`,
	}
	m := newCordisModel(host.serve(t))
	if err := m.cordisStopAction("s-1", "hello"); err != nil {
		t.Fatalf("not-running 要按官方折算成成功，实得 %v", err)
	}
	if host.count(apiclient.MethodCordisStopFromPanel) != 1 {
		t.Fatal("stopFromPanel 没被调到")
	}

	// plugin-missing 才是失败，且错误文本要是宿主原文。
	host.mu.Lock()
	host.stop = `{"result":{"ok":false,"reason":"plugin-missing","message":"插件 hello 不存在"}}`
	host.mu.Unlock()
	err := m.cordisStopAction("s-1", "hello")
	if err == nil || !strings.Contains(err.Error(), "插件 hello 不存在") {
		t.Fatalf("plugin-missing 必须报错并带宿主原文，实得 %v", err)
	}
}

// TestCordisRemoveDropsRowImmediately：官方 `inventory.retire()` 的等价行为。
func TestCordisRemoveDropsRowImmediately(t *testing.T) {
	host := &cordisHost{inventor: cordisRowJSON}
	m := newCordisModel(host.serve(t))
	m.cordisFor(cordisTestSession, cordisTestEpoch, nil)
	cordisPump(t, "首帧读完成", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.cordis.known
	})

	if err := m.cordisRemoveAction("s-1", "hello"); err != nil {
		t.Fatalf("undefine 应当成功：%v", err)
	}
	// 紧接着那一帧：行必须已经不在了（不等下一次读）。
	view := m.cordisFor(cordisTestSession, cordisTestEpoch, nil)
	for _, row := range view.Rows {
		if row.PluginID == "hello" {
			t.Fatalf("移除成功后行还在：%+v", view.Rows)
		}
	}
	if !view.CordisRemoved("hello") {
		t.Fatal("移除过的 pluginId 要记进 removed（官方 snapshot.removed 同一语义）")
	}

	// 重新定义回来：下一次读到的清单里有它 ⇒ 从 removed 里摘掉。
	m.mu.Lock()
	m.cordis.at = time.Time{}
	m.mu.Unlock()
	m.cordisRefresh(true)
	cordisPump(t, "重新读到 hello", func() bool {
		v := m.cordisFor(cordisTestSession, cordisTestEpoch, nil)
		for _, row := range v.Rows {
			if row.PluginID == "hello" {
				return !v.CordisRemoved("hello")
			}
		}
		return false
	})
}

// TestCordisRunHostHalfCarriesHostText：run 的业务失败要带上宿主原文。
func TestCordisRunHostHalfCarriesHostText(t *testing.T) {
	host := &cordisHost{inventor: `{"result":[]}`}
	srv := host.serve(t)
	m := newCordisModel(srv)
	err := m.cordisRunAction("s-1", "hello", "pkg-1", apiclient.CordisRunModeRun)
	if err != nil {
		// 假宿主对未实现的方法回 `{}`，解码出 ok:false + 空 message → 兜底文案。
		// 这里只要求"不是 nil 也不算成功"，不钉具体文案。
		if err.Error() == "" {
			t.Fatal("失败必须带错误文本（不许静默成功）")
		}
		return
	}
	t.Fatal("假宿主回 `{}` 时不该被当成成功")
}

// ── 水位分域（跨会话 / 跨世代）──────────────────────────────────────────────
//
// 这一组钉的是「2026-11-16 清扫查出的跨会话水位 bug」以及它的同族问题。
// 三者都**不报错、不丢帧**，只表现为"面板不刷新"——是最难从日志里看出来的一类。
// 判据：新域里的低 seq 事件必须**仍然**触发一次重读。

// TestCordisWatermarkIsPerSession 钉住跨会话不互相压制。
//
// 宿主在每会话各自的 journal 追加时分配 seq（transport-contract §3），
// 所以 A 的 seq=900 与 B 的 seq=3 **不可比**。全局单值水位会让 B 的 3 被判成
// "早就消费过"，于是 B 里 cordis 激活了、面板不刷新。
func TestCordisWatermarkIsPerSession(t *testing.T) {
	host := &cordisHost{inventor: cordisRowJSON}
	m := newCordisModel(host.serve(t))
	m.cordisFor(cordisTestSession, cordisTestEpoch, nil)
	cordisPump(t, "首帧读完成", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.cordis.known
	})

	// A 会话一路涨到 900。
	m.cordisFor("s-A", cordisTestEpoch, []apiclient.Frame{{Seq: 900, Type: cordisEventPackage}})

	// 切到 B 会话，它的 journal 才刚到 seq=3 —— 必须触发重读。
	before := host.count(apiclient.MethodCordisInventory)
	view := m.cordisFor("s-B", cordisTestEpoch, []apiclient.Frame{{Seq: 3, Type: cordisEventPackage}})
	if view.EventSeq != 3 {
		t.Fatalf("B 会话的 EventSeq = %d，want 3（水位必须按会话分记，不能被 A 的 900 压住）", view.EventSeq)
	}
	cordisPump(t, "B 会话的低 seq 事件触发重读", func() bool {
		return host.count(apiclient.MethodCordisInventory) == before+1
	})

	// 回到 A：A 自己的水位还是 900，3 与 900 都不该再造一次读。
	settled := host.count(apiclient.MethodCordisInventory)
	m.cordisFor("s-A", cordisTestEpoch, []apiclient.Frame{{Seq: 900, Type: cordisEventPackage}})
	time.Sleep(30 * time.Millisecond)
	if got := host.count(apiclient.MethodCordisInventory); got != settled {
		t.Fatalf("A 会话的重放帧又触发了一次读：%d -> %d", settled, got)
	}
}

// TestCordisWatermarkIsPerEpoch 钉住宿主重启后的世代切换。
//
// 同一 sessionID 在宿主重启后会换一份 journal（journalEpoch 变），
// 新 journal 的 seq 从 1 重新开始。只按 sessionID 记水位的话，
// 重启前的 900 会继续压住重启后的 1..n —— 同一个 bug 换个触发条件。
func TestCordisWatermarkIsPerEpoch(t *testing.T) {
	host := &cordisHost{inventor: cordisRowJSON}
	m := newCordisModel(host.serve(t))
	m.cordisFor(cordisTestSession, cordisTestEpoch, nil)
	cordisPump(t, "首帧读完成", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.cordis.known
	})

	// 世代 1 涨到 900。
	m.cordisFor(cordisTestSession, 1, []apiclient.Frame{{Seq: 900, Type: cordisEventPackage}})

	// 宿主重启：同会话、世代 2、seq 从 1 重新开始。
	before := host.count(apiclient.MethodCordisInventory)
	view := m.cordisFor(cordisTestSession, 2, []apiclient.Frame{{Seq: 1, Type: cordisEventPackage}})
	if view.EventSeq != 1 {
		t.Fatalf("新世代的 EventSeq = %d，want 1（换世代即换序号域，水位从 0 重新开始）", view.EventSeq)
	}
	cordisPump(t, "新世代的低 seq 事件触发重读", func() bool {
		return host.count(apiclient.MethodCordisInventory) == before+1
	})
}

// TestCordisWatermarkSkipsUnknownSession 钉住"没有可信域标识时不记水位"。
//
// 空 sessionID（壳还没加载会话 / 宿主没给 id）时，记一个"空 id 域"会把
// 不同会话的水位混成一份，等于退回全局单值——所以宁可这一帧不记，
// 也不要记一份错的。
func TestCordisWatermarkSkipsUnknownSession(t *testing.T) {
	host := &cordisHost{inventor: cordisRowJSON}
	m := newCordisModel(host.serve(t))
	view := m.cordisFor("", cordisTestEpoch, []apiclient.Frame{{Seq: 7, Type: cordisEventPackage}})
	if view.EventSeq != 0 {
		t.Fatalf("空会话的 EventSeq = %d，want 0（没有可信序号域就不记水位）", view.EventSeq)
	}
	m.mu.Lock()
	n := len(m.cordis.watermark.seq)
	m.mu.Unlock()
	if n != 0 {
		t.Fatalf("空会话不该建水位条目，实得 %d 条", n)
	}
}
