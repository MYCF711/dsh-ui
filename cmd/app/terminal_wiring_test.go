package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

type terminalHostFixture struct {
	t *testing.T

	mu       sync.Mutex
	calls    []string
	create   map[string]any
	write    map[string]any
	closeID  string
	listInfo bool
}

func (f *terminalHostFixture) handler(w http.ResponseWriter, r *http.Request) {
	method := strings.TrimPrefix(r.URL.Path, "/api/")
	f.mu.Lock()
	f.calls = append(f.calls, method)
	f.mu.Unlock()
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	switch method {
	case "terminal.environment":
		writeTerminalTestResult(w, `{"cwd":"/work","maxInputBytes":65536,"maxBufferedBytes":65536,"maxCols":500,"maxRows":200,"scrollback":1000}`)
	case "terminal.shells":
		writeTerminalTestResult(w, `[{"path":"/bin/bash","args":[],"name":"bash"}]`)
	case "terminal.list":
		if f.listInfo {
			writeTerminalTestResult(w, `[{"id":"tab1","title":"saved","shell":{"path":"/bin/bash","args":[],"name":"bash"},"cwd":"/work","cols":80,"rows":24,"state":"running","exitCode":null,"origin":"user","control":"user"}]`)
		} else {
			writeTerminalTestResult(w, `[]`)
		}
	case "terminal.create":
		f.mu.Lock()
		f.create = body
		f.mu.Unlock()
		writeTerminalTestResult(w, `{"id":"tab1","title":"bash","shell":{"path":"/bin/bash","args":[],"name":"bash"},"cwd":"/work","cols":80,"rows":24,"state":"running","exitCode":null,"origin":"user","control":"user"}`)
	case "terminal.retain":
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"type":"retained"}`)
		flushTerminalTest(w)
		<-r.Context().Done()
	case "terminal.attach":
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"type":"snapshot","sequence":0,"screen":"ready\r\n","info":{"id":"tab1","title":"bash","shell":{"path":"/bin/bash","args":[],"name":"bash"},"cwd":"/work","cols":80,"rows":24,"state":"running","exitCode":null,"origin":"user","control":"user"},"writable":true}`)
		fmt.Fprintln(w, `{"type":"output","sequence":1,"data":"hello\r\n"}`)
		flushTerminalTest(w)
		<-r.Context().Done()
	case "terminal.watch":
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `[]`)
		flushTerminalTest(w)
		<-r.Context().Done()
	case "terminal.input":
		f.mu.Lock()
		f.write = body
		f.mu.Unlock()
		writeTerminalTestResult(w, `{"ok":true}`)
	case "terminal.rename":
		writeTerminalTestResult(w, `null`)
	case "terminal.close":
		f.mu.Lock()
		f.closeID, _ = body["id"].(string)
		f.mu.Unlock()
		writeTerminalTestResult(w, `null`)
	default:
		http.NotFound(w, r)
	}
}

func writeTerminalTestResult(w http.ResponseWriter, result string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"result":%s}`, result)
}

func flushTerminalTest(w http.ResponseWriter) {
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// closeStreamingServer 关掉 httptest server，先断客户端连接。
//
// 为什么不能直接 srv.Close()：本文件的 fixture 提供 terminal.watch 的**流式**响应
// （w.(http.Flusher)，见下面 handler），而 httptest.Server.Close() 会一直等所有
// 未完成的请求收尾。客户端那头若还挂着 watch 流，Close 就永远不返回——
// 2026-11-17 CI 上真出现过：TestTerminalWiringCreateRetainAttachWriteRenameClose
// 卡了 9m57s 直到 go test 的 10 分钟包超时，整条 main CI 因此变红。
// CloseClientConnections 先掐断连接，Close 才有东西可等。
func closeStreamingServer(srv *httptest.Server) {
	srv.CloseClientConnections()
	srv.Close()
}

func waitTerminalCondition(t *testing.T, condition func() bool) {
	t.Helper()
	// 30s 而不是 5s：这里等的是**进程内**状态跃迁（正常毫秒级），
	// 但 CI 高峰期整机负载能把 5s 吃满 -> 2026-11-18 在 dsh-ui#157 上
	// 报「等待终端状态超时」，而同分支根本没碰终端代码。
	// 提高截止时间不改判据（条件仍然必须成立），只是给负载留余量。
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("等待终端状态超时")
}

func TestTerminalWiringCreateRetainAttachWriteRenameClose(t *testing.T) {
	fixture := &terminalHostFixture{t: t}
	srv := httptest.NewServer(http.HandlerFunc(fixture.handler))
	defer closeStreamingServer(srv)
	m := &model{cli: apiclient.New(srv.URL), cur: "s1"}
	m.terminalEnsure("tab1")
	waitTerminalCondition(t, func() bool {
		state := m.terminalSnapshot("tab1")
		return state.Phase == "connected" && strings.Contains(state.Screen, "hello")
	})
	state := m.terminalSnapshot("tab1")
	if state.Info == nil || state.Info.ID != "tab1" || !state.Writable || !state.Retained {
		t.Fatalf("create→retain→attach 状态不完整：%+v", state)
	}
	if state.Environment == nil || state.Environment.Scrollback != 1000 || len(state.Shells) != 1 {
		t.Fatalf("environment/shells 未接入：%+v", state)
	}
	fixture.mu.Lock()
	create := fixture.create
	fixture.mu.Unlock()
	request, _ := create["request"].(map[string]any)
	if create["agentId"] != "s1" || request["id"] != "tab1" || request["shellPath"] != "/bin/bash" {
		t.Fatalf("create 参数错误：%v", create)
	}

	m.terminalWriteOfficial("tab1", "echo hi\n", true)
	waitTerminalCondition(t, func() bool {
		fixture.mu.Lock()
		defer fixture.mu.Unlock()
		return fixture.write != nil
	})
	fixture.mu.Lock()
	write := fixture.write
	fixture.mu.Unlock()
	if write["sessionId"] != "s1" || write["typed"] != true || write["data"] != "echo hi\n" {
		t.Fatalf("typed input 错误：%v", write)
	}
	if err := m.terminalRenameOfficial("tab1", "构建"); err != nil {
		t.Fatal(err)
	}
	if got := m.terminalSnapshot("tab1").Info.Title; got != "构建" {
		t.Fatalf("rename 未更新真实缓存：%q", got)
	}
	if err := m.terminalCloseOfficial("tab1"); err != nil {
		t.Fatal(err)
	}
	fixture.mu.Lock()
	closeID := fixture.closeID
	fixture.mu.Unlock()
	if closeID != "tab1" {
		t.Fatalf("close 错 id：%q", closeID)
	}
	if got := m.terminalSnapshot("tab1"); got.Info != nil || got.Phase != "idle" {
		t.Fatalf("close 成功后本地 occurrence 未释放：%+v", got)
	}
	m.mu.Lock()
	for _, watch := range m.terminalWatchers {
		watch.cancel()
	}
	m.mu.Unlock()
}

func TestTerminalWiringRecoversListedIdentityWithoutCreate(t *testing.T) {
	fixture := &terminalHostFixture{t: t, listInfo: true}
	srv := httptest.NewServer(http.HandlerFunc(fixture.handler))
	defer closeStreamingServer(srv)
	m := &model{cli: apiclient.New(srv.URL), cur: "s1"}
	m.terminalEnsure("tab1")
	waitTerminalCondition(t, func() bool { return m.terminalSnapshot("tab1").Phase == "connected" })
	fixture.mu.Lock()
	create := fixture.create
	fixture.mu.Unlock()
	if create != nil {
		t.Fatalf("list 中已有的终端不该重复 create：%v", create)
	}
	m.mu.Lock()
	for _, watch := range m.terminalWatchers {
		watch.cancel()
	}
	for _, view := range m.terminalViews {
		view.cancel()
	}
	m.mu.Unlock()
}
