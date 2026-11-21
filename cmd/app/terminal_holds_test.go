// terminal_holds_test.go —— #54 条 `retainTabs` 全量保命面的证据。
//
// 官方两段原件（见 terminal_holds.go 文件头）：
//
//	retainTabs(openTabs.filter(kind==='terminal'))   —— 成员表是**全量**、跨会话
//	reconcileHolds(): 缺的补 hold、多的 release（差集即释放）
//
// 本文件盯三件事：
//
//	① 不是当前会话的 tab 也必须有 retain 寿命流（这正是旧账那条缺口）；
//	② tab 一关，对应那条流必须**真的关掉**（不释放就是 retain 泄漏）；
//	③ 同一条终端只有**一条**流（官方视图的 retain 就是窗口 hold，不是第二条）。
//
// 判据落在假宿主那侧的"当前活着的流条数"，不是壳自己的 map：map 说释放了、
// HTTP 连接还挂着，那还是泄漏。
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsterminal"
)

// terminalHoldHost 是"宿主上确实有几条终端"的假宿主，专测 hold 这条寿命流。
type terminalHoldHost struct {
	mu sync.Mutex
	// roster 是宿主上活着的终端：session -> ids（terminal.list / terminal.watch 同源）。
	roster map[string][]string
	// openStreams 记**当前还活着**的 retain 寿命流条数，键 = session\x00id。
	openStreams map[string]int
	// streamStarts 记累计起过几条（抓"同一条终端开两条流"与重复开流）。
	streamStarts map[string]int
	closes       map[string]int
	// kill 关闭即掐断所有**活着的** retain 流，用来模拟宿主重启/断网。
	kill chan struct{}
}

func newTerminalHoldHost(roster map[string][]string) *terminalHoldHost {
	if roster == nil {
		roster = map[string][]string{}
	}
	return &terminalHoldHost{
		roster:       roster,
		openStreams:  map[string]int{},
		streamStarts: map[string]int{},
		closes:       map[string]int{},
		kill:         make(chan struct{}),
	}
}

// restart 模拟宿主重启：现存那条 retain 寿命流断掉，新连接正常。
func (h *terminalHoldHost) restart() {
	h.mu.Lock()
	old := h.kill
	h.kill = make(chan struct{})
	h.mu.Unlock()
	close(old)
}

func (h *terminalHoldHost) killChannel() <-chan struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.kill
}

func terminalHoldInfoJSON(id string) string {
	return `{"id":"` + id + `","title":"` + id + `",` +
		`"shell":{"path":"/bin/bash","args":[],"name":"bash"},"cwd":"/work",` +
		`"cols":80,"rows":24,"state":"running","exitCode":null,"origin":"user","control":"user"}`
}

func (h *terminalHoldHost) rosterJSON(sessionID string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	ids := h.roster[sessionID]
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, terminalHoldInfoJSON(id))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func (h *terminalHoldHost) handler(w http.ResponseWriter, r *http.Request) {
	method := strings.TrimPrefix(r.URL.Path, "/api/")
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	sessionID, _ := body["sessionId"].(string)
	if sessionID == "" {
		sessionID, _ = body["agentId"].(string)
	}
	id, _ := body["id"].(string)
	switch method {
	case "terminal.environment":
		writeTerminalTestResult(w, `{"cwd":"/work","maxInputBytes":65536,"maxBufferedBytes":65536,"maxCols":500,"maxRows":200,"scrollback":1000}`)
	case "terminal.shells":
		writeTerminalTestResult(w, `[{"path":"/bin/bash","args":[],"name":"bash"}]`)
	case "terminal.list":
		writeTerminalTestResult(w, h.rosterJSON(sessionID))
	case "terminal.create":
		request, _ := body["request"].(map[string]any)
		created, _ := request["id"].(string)
		h.mu.Lock()
		h.roster[sessionID] = append(h.roster[sessionID], created)
		h.mu.Unlock()
		writeTerminalTestResult(w, terminalHoldInfoJSON(created))
	case "terminal.retain":
		key := sessionID + "\x00" + id
		h.mu.Lock()
		h.openStreams[key]++
		h.streamStarts[key]++
		h.mu.Unlock()
		defer func() {
			h.mu.Lock()
			h.openStreams[key]--
			h.mu.Unlock()
		}()
		// 寿命流：一帧 retained 之后保持打开，客户端取消即结束
		// （宿主侧同一套帧机，DSH-GO cmd/dshd/serve_terminal_api.go:815-860）。
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"type":"retained"}`)
		flushTerminalTest(w)
		select {
		case <-r.Context().Done():
		case <-h.killChannel():
			// 宿主重启：这条寿命流直接结束（响应体 EOF）。
		}
	case "terminal.attach":
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"type":"snapshot","sequence":0,"screen":"ready\r\n","info":`+terminalHoldInfoJSON(id)+`,"writable":true}`)
		flushTerminalTest(w)
		<-r.Context().Done()
	case "terminal.watch":
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, h.rosterJSON(sessionID))
		flushTerminalTest(w)
		<-r.Context().Done()
	case "terminal.close":
		h.mu.Lock()
		h.closes[sessionID+"\x00"+id]++
		kept := h.roster[sessionID][:0]
		for _, one := range h.roster[sessionID] {
			if one != id {
				kept = append(kept, one)
			}
		}
		h.roster[sessionID] = kept
		h.mu.Unlock()
		writeTerminalTestResult(w, `null`)
	case "terminal.rename", "terminal.input", "terminal.resize":
		writeTerminalTestResult(w, `null`)
	default:
		http.NotFound(w, r)
	}
}

func (h *terminalHoldHost) activeStreams(key string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.openStreams[key]
}

func (h *terminalHoldHost) starts(key string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.streamStarts[key]
}

// waitStreams 等某个 (会话, 终端) 的**活着**的 retain 流条数变成 want。
func (h *terminalHoldHost) waitStreams(t *testing.T, key string, want int) {
	t.Helper()
	waitTerminalConditionDump(t, func() bool { return h.activeStreams(key) == want }, func() string {
		h.mu.Lock()
		defer h.mu.Unlock()
		return fmt.Sprintf("key=%q want=%d streams=%v starts=%v", key, want, h.openStreams, h.streamStarts)
	})
}

// 右栏布局的持久化格式就是壳自己写的那个信封（`dsh.sidebar-right.v1.<session>`），
// 官方那侧对应 `openTabs` 里 dormant Session 的 tab 来源。
func rightTabLayoutJSON(tabs ...string) string {
	parts := make([]string, 0, len(tabs))
	for _, id := range tabs {
		parts = append(parts, `{"id":"`+id+`","kind":"terminal"}`)
	}
	// minted 必须 >= 已分配的 tab<N>：壳的解码器拿它做一致性校验（见
	// decodeRightTabSurface「minted=%d 小于已分配 tab id」），少写就被判坏数据。
	minted := 0
	for _, id := range tabs {
		if n, err := strconv.Atoi(strings.TrimPrefix(id, "tab")); err == nil && n > minted {
			minted = n
		}
	}
	return `{"bySession":{"__SESSION__":{"tabs":[` + strings.Join(parts, ",") +
		`],"activeTabId":"` + tabs[0] + `","minted":` + strconv.Itoa(minted) + `}}}`
}

// setDormantRightTabLayout 把某个**没被展示过**的会话的布局塞进宿主 ui-config 缓存。
// 返回还原函数：uiConfig 是包级缓存，测试结束必须恢复，否则污染同包其它用例。
func setDormantRightTabLayout(t *testing.T, sessionID string, tabIDs ...string) {
	t.Helper()
	previous := uiConfig
	raw := strings.ReplaceAll(rightTabLayoutJSON(tabIDs...), "__SESSION__", sessionID)
	next := map[string]string{}
	for k, v := range previous {
		next[k] = v
	}
	next[rightSidebarConfigKey(sessionID)] = raw
	uiConfig = next
	t.Cleanup(func() { uiConfig = previous })
}

// newTerminalHoldModel 造一个只加载 ds-core + ds-terminal 的壳 model。
func newTerminalHoldModel(baseURL string, loaded ...string) *model {
	m := &model{cli: apiclient.New(baseURL), cur: "s1"}
	if len(loaded) > 0 {
		m.cur = loaded[0]
	}
	kernel := uikernel.New("terminal-holds")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dsterminal.Plugin{}); err != nil {
		panic(err)
	}
	m.kernel = kernel
	m.rightTabLoaded = map[string]bool{}
	m.rightTabSurfaces = map[string]*rightTabSurface{}
	// 这三个 map 由 shell 正常路径懒初始化；裸 model 必须自己给，
	// 否则 `dropRightTabLocked` → `markRightTabSelectionLocked` 往 nil map 写会 panic
	// （panic 时 m.mu 还被持有，defer 里再 Lock 就死锁 —— 2026-11-21 本文件的第一次跑）。
	m.rightTabSel = map[string]string{}
	m.rightTabDirty = map[string]bool{}
	m.rightTabParams = map[string]pluginapi.RightTabParams{}
	// 用户关 tab 那条路要经过插件的 close handler（`dsterminal.CloseRightTab` →
	// `actions.TerminalCloseOfficial`）；裸 model 里 Actions 是零值，不接就是
	// "用户终端 close 未接线"，壳会保留 tab（那是另一条既有语义，见 #54 行）。
	m.actions.TerminalCloseOfficial = m.terminalCloseOfficial
	for _, sessionID := range loaded {
		m.rightTabLoaded[sessionID] = true
	}
	return m
}

func (m *model) stopTerminalTestStreams() {
	m.mu.Lock()
	for _, watch := range m.terminalWatchers {
		if watch.cancel != nil {
			watch.cancel()
		}
	}
	m.terminalWatchers = nil
	for _, view := range m.terminalViews {
		if view.cancel != nil {
			view.cancel()
		}
	}
	m.mu.Unlock()
	m.releaseAllTerminalHolds()
}

// TestTerminalHoldsRetainEveryOpenTabNotJustCurrentSession 是这条缺口的正面证据：
// 当前会话是 s1，s1 的终端 tab 已经 attach；**s2 的 terminal tab 一次都没被展示过**
// （只存在于持久化布局里，就是官方的 dormant Session），它也必须有一条 retain
// 寿命流。旧实现只在 attach 时 retain 当前会话，这条流根本不会出现。
func TestTerminalHoldsRetainEveryOpenTabNotJustCurrentSession(t *testing.T) {
	setDormantRightTabLayout(t, "s2", "tab2")
	host := newTerminalHoldHost(map[string][]string{"s2": {"tab2"}})
	srv := httptest.NewServer(http.HandlerFunc(host.handler))
	defer closeStreamingServer(srv)

	m := newTerminalHoldModel(srv.URL, "s1")
	defer m.stopTerminalTestStreams()
	m.rightTabSurfaces["s1"] = &rightTabSurface{
		Tabs:        []rightTabRecord{{ID: "tab1", Kind: rightTabKindTerminal}},
		ActiveTabID: "tab1",
	}
	m.terminalEnsure("tab1")
	waitTerminalCondition(t, func() bool { return m.terminalSnapshot("tab1").Phase == "connected" })
	// 官方 `sync()`：布局一到就把全量成员交给 retainTabs。
	m.syncTerminalHolds()

	// ① 别的会话里开着的 tab（本窗口从没展示过 s2）
	host.waitStreams(t, terminalHoldKey("s2", "tab2"), 1)
	// ② 当前会话里 attach 着的那条
	host.waitStreams(t, terminalHoldKey("s1", "tab1"), 1)
	// ③ 同一条终端只有一条流：视图的 retain 就是窗口 hold（官方 1007-1010 的
	//    `hold(...).ready()`），不是另开一条。
	if got := host.starts(terminalHoldKey("s1", "tab1")); got != 1 {
		t.Fatalf("s1/tab1 的 retain 流起了 %d 次，want 1（视图与窗口 hold 必须是同一条）", got)
	}
	// 全量对账再跑一次（官方每次 openTabs 变化都跑）：不许多开流。
	m.syncTerminalHolds()
	time.Sleep(50 * time.Millisecond)
	if got := host.starts(terminalHoldKey("s1", "tab1")); got != 1 {
		t.Fatalf("重复对账把 s1/tab1 的 retain 流开成了 %d 条", got)
	}
	if got := host.starts(terminalHoldKey("s2", "tab2")); got != 1 {
		t.Fatalf("重复对账把 s2/tab2 的 retain 流开成了 %d 条", got)
	}
}

// TestTerminalHoldsReleaseWhenTheTabCloses 是**释放语义**的证据：关掉一枚 tab，
// 它那条 retain 寿命流必须真的关掉（否则终端被保活、界面上却没有它的 tab）；
// 同时别的 tab 的流不许被误伤。
func TestTerminalHoldsReleaseWhenTheTabCloses(t *testing.T) {
	host := newTerminalHoldHost(map[string][]string{"s2": {"host2"}})
	srv := httptest.NewServer(http.HandlerFunc(host.handler))
	defer closeStreamingServer(srv)

	m := newTerminalHoldModel(srv.URL, "s1", "s2")
	defer m.stopTerminalTestStreams()
	m.rightTabSurfaces["s1"] = &rightTabSurface{
		Tabs:        []rightTabRecord{{ID: "tab1", Kind: rightTabKindTerminal}},
		ActiveTabID: "tab1",
	}
	m.rightTabSurfaces["s2"] = &rightTabSurface{
		Tabs:        []rightTabRecord{{ID: "tab2", Kind: rightTabKindTerminal}},
		ActiveTabID: "tab2",
	}
	// s2 那枚是"接管一条宿主上已有的 terminal"（官方 `openTab('terminal', {params:
	// {terminalId}})`），所以它没有活视图也带着显式宿主身份。
	m.mu.Lock()
	m.rightTabParams = map[string]pluginapi.RightTabParams{
		rightTabParamsKey("s2", "tab2"): {TerminalID: "host2"},
	}
	m.mu.Unlock()

	m.terminalEnsure("tab1")
	waitTerminalCondition(t, func() bool { return m.terminalSnapshot("tab1").Phase == "connected" })
	m.syncTerminalHolds()
	host.waitStreams(t, terminalHoldKey("s1", "tab1"), 1)
	host.waitStreams(t, terminalHoldKey("s2", "host2"), 1)

	// 关掉 s2 那枚（不是当前会话的 tab，也不是 attach 着的视图）。
	m.forgetRightTab("s2", "tab2")
	host.waitStreams(t, terminalHoldKey("s2", "host2"), 0)
	if got := host.activeStreams(terminalHoldKey("s1", "tab1")); got != 1 {
		t.Fatalf("关掉 s2 的 tab 误伤了 s1 的 hold：s1/tab1 流 = %d", got)
	}
}

// TestTerminalHoldsReleaseOnUserClose 走**用户关 tab** 那条路（插件 close handler →
// 宿主 terminal.close → 壳删 occurrence），落点 `closeRightTab`。
func TestTerminalHoldsReleaseOnUserClose(t *testing.T) {
	host := newTerminalHoldHost(nil)
	srv := httptest.NewServer(http.HandlerFunc(host.handler))
	defer closeStreamingServer(srv)

	m := newTerminalHoldModel(srv.URL, "s1")
	defer m.stopTerminalTestStreams()
	m.rightTabSurfaces["s1"] = &rightTabSurface{
		Tabs:        []rightTabRecord{{ID: "tab1", Kind: rightTabKindTerminal}},
		ActiveTabID: "tab1",
	}
	m.terminalEnsure("tab1")
	waitTerminalCondition(t, func() bool { return m.terminalSnapshot("tab1").Phase == "connected" })
	host.waitStreams(t, terminalHoldKey("s1", "tab1"), 1)

	m.closeRightTab("tab1")
	host.waitStreams(t, terminalHoldKey("s1", "tab1"), 0)
	host.mu.Lock()
	closes := host.closes[terminalHoldKey("s1", "tab1")]
	host.mu.Unlock()
	if closes != 1 {
		t.Fatalf("关 tab 没有走宿主 terminal.close：%d 次", closes)
	}
}

// TestTerminalHoldsTeardownReleasesEverything 覆盖官方 effect 拆除时的
// `retainTabs([])`（本仓落点是 `main()` 里 `app.RunCanvas` 之后）。
func TestTerminalHoldsTeardownReleasesEverything(t *testing.T) {
	setDormantRightTabLayout(t, "s2", "tab2")
	host := newTerminalHoldHost(map[string][]string{"s2": {"tab2"}})
	srv := httptest.NewServer(http.HandlerFunc(host.handler))
	defer closeStreamingServer(srv)

	m := newTerminalHoldModel(srv.URL, "s1")
	defer m.stopTerminalTestStreams()
	m.rightTabSurfaces["s1"] = &rightTabSurface{
		Tabs:        []rightTabRecord{{ID: "tab1", Kind: rightTabKindTerminal}},
		ActiveTabID: "tab1",
	}
	m.terminalEnsure("tab1")
	waitTerminalCondition(t, func() bool { return m.terminalSnapshot("tab1").Phase == "connected" })
	m.syncTerminalHolds()
	host.waitStreams(t, terminalHoldKey("s1", "tab1"), 1)
	host.waitStreams(t, terminalHoldKey("s2", "tab2"), 1)

	m.releaseAllTerminalHolds()
	host.waitStreams(t, terminalHoldKey("s1", "tab1"), 0)
	host.waitStreams(t, terminalHoldKey("s2", "tab2"), 0)
}

// TestTerminalHoldsReviveAfterTheStreamEnds 钉住"断了要能重开"：官方那条 hold 由
// gateway 的 `$stream` 自动重连（retention 段的 "reconnecting stream owner"），
// 本仓没有那层，所以流 EOF 后必须把凭证摘掉，让下一次全量对账重开一条——
// 否则宿主重启一次，界面上 tab 还开着、宿主那头已经不再保活了。
func TestTerminalHoldsReviveAfterTheStreamEnds(t *testing.T) {
	host := newTerminalHoldHost(nil)
	srv := httptest.NewServer(http.HandlerFunc(host.handler))
	defer closeStreamingServer(srv)

	m := newTerminalHoldModel(srv.URL, "s1")
	defer m.stopTerminalTestStreams()
	m.rightTabSurfaces["s1"] = &rightTabSurface{
		Tabs:        []rightTabRecord{{ID: "tab1", Kind: rightTabKindTerminal}},
		ActiveTabID: "tab1",
	}
	m.terminalEnsure("tab1")
	waitTerminalCondition(t, func() bool { return m.terminalSnapshot("tab1").Phase == "connected" })
	key := terminalHoldKey("s1", "tab1")
	host.waitStreams(t, key, 1)

	// 宿主重启：活着的那条流断掉。
	host.restart()
	waitTerminalCondition(t, func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.terminalHolds[key] == nil
	})
	host.waitStreams(t, key, 0)

	// 下一次对账（任何成员表变化都会触发）要把它重开。
	m.syncTerminalHolds()
	waitTerminalCondition(t, func() bool {
		return host.activeStreams(key) == 1 && host.starts(key) == 2
	})
}
