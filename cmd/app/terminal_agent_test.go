// terminal_agent_test.go —— #54 条①（Agent / 沙箱徽标）的**数据通路**证据。
//
// 徽标本身由 `plugins/dsterminal` 画（`RightTabBadge`，逐条照
// OFF:dsh-client-ui-sidebar-terminal/src/client/TerminalTitle.tsx:17-20,44-50），
// 但那条链上原来缺两段，缺任何一段徽标都**永远不会出现**：
//
//	A. 没有任何东西把 Agent 开的终端变成一枚右栏 tab。官方是
//	   `AgentTerminalOpener`（只返回 null 的组件）在会话头显示期间挂着
//	   `webTerminals.watch`，收到 roster 就对 `origin==='agent' && state==='running'`
//	   的终端 `openTab('terminal', { params: { terminalId: info.id } })`
//	   （OFF:…/src/client/index.ts:103-112）。
//	B. 带 `terminalId` 打开的 occurrence 被当成一条**新**终端（照 occurrence id
//	   create），于是它接不到 Agent 那条终端，徽标的 origin/control/sandboxMode
//	   永远是 user（OFF:dsh-api-terminal-controller/src/client/index.ts:104-119
//	   「saved = terminalId ?? bindings.get(...)」）。
//
// 本文件把 A + B 钉住：roster 一出现 → 壳铸 occurrence → 视图按 Agent 那条终端
// attach（**不** create）→ 快照里 origin/control/sandboxMode 与宿主回包一致。
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsterminal"
)

// agentTerminalHost 是一个"宿主上已经有一条 Agent 终端"的假宿主。
type agentTerminalHost struct {
	mu        sync.Mutex
	calls     []string
	created   bool
	attachIDs []string
	// raise：roster 里是否还列着那条 Agent 终端（测「被 Agent 结束 → tab 关掉」用）。
	raise bool
}

const agentTerminalID = "agent-7f3c"

func agentTerminalInfoJSON() string {
	// 逐字段照 rc.2 `WebTerminalInfo`（types.ts:53-70）。`sandboxMode` 只在
	// agent 终端上出现，它是**徽标第二档**的判据。
	return `{"id":"` + agentTerminalID + `","title":"agent shell",` +
		`"shell":{"path":"/bin/bash","args":[],"name":"bash"},"cwd":"/work",` +
		`"cols":80,"rows":24,"state":"running","exitCode":null,` +
		`"origin":"agent","control":"agent","sandboxMode":"workspace-write"}`
}

func (h *agentTerminalHost) rosterJSON() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.raise {
		return `[]`
	}
	return `[` + agentTerminalInfoJSON() + `]`
}

func (h *agentTerminalHost) handler(w http.ResponseWriter, r *http.Request) {
	method := strings.TrimPrefix(r.URL.Path, "/api/")
	h.mu.Lock()
	h.calls = append(h.calls, method)
	if method == "terminal.attach" {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		id, _ := body["id"].(string)
		h.attachIDs = append(h.attachIDs, id)
	}
	h.mu.Unlock()
	switch method {
	case "terminal.environment":
		writeTerminalTestResult(w, `{"cwd":"/work","maxInputBytes":65536,"maxBufferedBytes":65536,"maxCols":500,"maxRows":200,"scrollback":1000}`)
	case "terminal.shells":
		writeTerminalTestResult(w, `[{"path":"/bin/bash","args":[],"name":"bash"}]`)
	case "terminal.list":
		// 与真实宿主一致：list 是快照，agent 终端也在这份名单里。
		writeTerminalTestResult(w, h.rosterJSON())
	case "terminal.create":
		h.mu.Lock()
		h.created = true
		h.mu.Unlock()
		// 若真被调到，说明壳把"接管"做成了"新开"，回一条 user 终端让断言看见。
		writeTerminalTestResult(w, `{"id":"tab1","title":"bash","shell":{"path":"/bin/bash","args":[],"name":"bash"},"cwd":"/work","cols":80,"rows":24,"state":"running","exitCode":null,"origin":"user","control":"user"}`)
	case "terminal.retain":
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"type":"retained"}`)
		flushTerminalTest(w)
		<-r.Context().Done()
	case "terminal.attach":
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"type":"snapshot","sequence":0,"screen":"agent$ ","info":`+agentTerminalInfoJSON()+`,"writable":true}`)
		fmt.Fprintln(w, `{"type":"output","sequence":1,"data":"done\r\n"}`)
		flushTerminalTest(w)
		<-r.Context().Done()
	case "terminal.watch":
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, h.rosterJSON())
		flushTerminalTest(w)
		<-r.Context().Done()
	case "terminal.close", "terminal.rename":
		writeTerminalTestResult(w, `null`)
	default:
		http.NotFound(w, r)
	}
}

func (h *agentTerminalHost) called(method string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, one := range h.calls {
		if one == method {
			return true
		}
	}
	return false
}

// newAgentTerminalModel 造一个只加载了 ds-core + ds-terminal 的壳 model。
func newAgentTerminalModel(t *testing.T, baseURL string) *model {
	t.Helper()
	kernel := uikernel.New("agent-terminal")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dsterminal.Plugin{}); err != nil {
		t.Fatalf("装配内核：%v", err)
	}
	m := &model{cli: apiclient.New(baseURL), cur: "s1", kernel: kernel}
	m.rightTabLoaded = map[string]bool{"s1": true}
	return m
}

// agentTab 找出当前会话里绑定到 Agent 那条终端的 occurrence。
func agentTab(m *model, hostID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	surface := m.rightTabSurfaces["s1"]
	if surface == nil {
		return ""
	}
	for _, tab := range surface.Tabs {
		if params, ok := m.rightTabParams[rightTabParamsKey("s1", tab.ID)]; ok && params.TerminalID == hostID {
			return tab.ID
		}
	}
	return ""
}

// TestAgentTerminalRosterOpensTabThatAdoptsTheHostTerminal 是 #54 条①的主证据。
func TestAgentTerminalRosterOpensTabThatAdoptsTheHostTerminal(t *testing.T) {
	host := &agentTerminalHost{raise: true}
	srv := httptest.NewServer(http.HandlerFunc(host.handler))
	defer closeStreamingServer(srv)
	m := newAgentTerminalModel(t, srv.URL)

	m.ensureTerminalWatch("s1")

	var tabID string
	waitTerminalCondition(t, func() bool {
		tabID = agentTab(m, agentTerminalID)
		return tabID != ""
	})
	// 官方 `openTabIn(sessionId, 'terminal', { params: { terminalId: info.id } })`
	// 的那一份参数，必须真的落在 occurrence 上——它就是"接管哪条终端"的身份。
	if got := m.rightTabParamsFor("s1", tabID).TerminalID; got != agentTerminalID {
		t.Fatalf("occurrence 绑定 = %q，want %q", got, agentTerminalID)
	}

	// 接管：attach 用的必须是 Agent 那条终端的身份，而不是新铸的 occurrence id。
	m.terminalEnsure(tabID)
	waitTerminalCondition(t, func() bool {
		state := m.terminalSnapshot(tabID)
		return state.Phase == "connected" && strings.Contains(state.Screen, "done")
	})

	state := m.terminalSnapshot(tabID)
	if state.Info == nil {
		t.Fatal("接管后没有 info：徽标的数据源为空")
	}
	// 这三项就是徽标的三个输入（TerminalTitle.tsx:18-20）。壳一个都不许丢。
	if state.Info.Origin != "agent" || state.Info.Control != "agent" || state.Info.SandboxMode != "workspace-write" {
		t.Fatalf("快照丢了徽标输入：origin=%q control=%q sandboxMode=%q",
			state.Info.Origin, state.Info.Control, state.Info.SandboxMode)
	}
	if host.created {
		t.Fatal("接管路径调了 terminal.create：那是新开一条 user 终端，Agent 徽标就成了假的")
	}
	host.mu.Lock()
	attachIDs := append([]string(nil), host.attachIDs...)
	host.mu.Unlock()
	if len(attachIDs) == 0 || attachIDs[0] != agentTerminalID {
		t.Fatalf("attach 的身份 = %v，want [%s]", attachIDs, agentTerminalID)
	}
}

// TestAgentTerminalTabClosesWhenTheAgentEndsIt 钉住官方 `closeEndedAgentTerminals`：
// 受控终端（control==='agent'）离开 roster = Agent 或它的会话结束了它 → 关掉绑定的 tab。
func TestAgentTerminalTabClosesWhenTheAgentEndsIt(t *testing.T) {
	host := &agentTerminalHost{raise: true}
	srv := httptest.NewServer(http.HandlerFunc(host.handler))
	defer closeStreamingServer(srv)
	m := newAgentTerminalModel(t, srv.URL)

	m.ensureTerminalWatch("s1")
	var tabID string
	waitTerminalCondition(t, func() bool {
		tabID = agentTab(m, agentTerminalID)
		return tabID != ""
	})
	m.terminalEnsure(tabID)
	waitTerminalCondition(t, func() bool { return m.terminalSnapshot(tabID).Phase == "connected" })

	// 宿主那头把终端撤掉：这是一条**新的** roster 帧。watch 流只推一帧，所以
	// 这里直接调用同一段逻辑（`syncAgentTerminals`）喂新名单——与 runTerminalWatch
	// 收到第二帧时走的是同一条路径。
	host.mu.Lock()
	host.raise = false
	host.mu.Unlock()
	m.syncAgentTerminals("s1", nil)

	if got := agentTab(m, agentTerminalID); got != "" {
		t.Fatalf("Agent 结束终端后 tab 仍在：%q", got)
	}
	m.mu.Lock()
	_, stillView := m.terminalViews[userTerminalKey("s1", tabID)]
	m.mu.Unlock()
	if stillView {
		t.Fatal("Agent 结束终端后视图缓存仍在（会继续 attach 一条死终端）")
	}
}
