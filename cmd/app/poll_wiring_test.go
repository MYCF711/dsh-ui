// poll_wiring_test.go —— 壳侧两条"后台轮询"的接线判据。
//
// 两条都是**只增不减**的形状，出错时没有任何界面提示，只能靠用例钉：
//  1. 后台轮询 ticker 只能起一份（refreshList 有十来个调用点）；
//  2. 终端还没打开时不许把在途标记置上（否则永远拉不到终端正文）。
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// configOKHost 造一个假宿主：config.get 成功（壳的轮询 ticker 挂在它后面），
// 其余方法一律 500（这些失败路径都只记错误、不打扰用户）。
func configOKHost(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/config.get") {
			_, _ = fmt.Fprint(w, `{"result":{"values":{}}}`)
			return
		}
		http.Error(w, `{"error":{"code":"internal","message":"test host"}}`, http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	return server
}

// TestRefreshListStartsBackgroundPollOnce 钉「后台轮询只起一份」。
//
// refreshList 的调用点：新建/派生/删除会话、导入、切工作区……每次调用都各起
// 一个 `for range t.C` 的 goroutine（循环体内没有退出路径），于是
// ① goroutine 数随操作次数单调增长；② 每个 ticker 各自发 jobs.list/terminal.output，
// 操作十次之后宿主每 5 秒被问十遍。
func TestRefreshListStartsBackgroundPollOnce(t *testing.T) {
	oldEvery := jobsPollEvery
	jobsPollEvery = 20 * time.Millisecond
	defer func() { jobsPollEvery = oldEvery }()

	server := configOKHost(t)
	m := &model{cli: apiclient.New(server.URL)}

	// 模拟"新建会话 + 派生 + 删除"三次入口调用。
	m.refreshList()
	m.refreshList()
	m.refreshList()

	deadline := time.Now().Add(3 * time.Second)
	for {
		m.mu.Lock()
		runs := m.pollRuns
		m.mu.Unlock()
		if runs > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("后台轮询从未启动——后台任务/终端面板再也不会刷新")
		}
		time.Sleep(2 * time.Millisecond)
	}
	// 至少再跑过几个 tick，让"多份轮询"有机会各自 +1。
	time.Sleep(100 * time.Millisecond)

	m.mu.Lock()
	runs := m.pollRuns
	m.mu.Unlock()
	if runs != 1 {
		t.Fatalf("refreshList 调用了 3 次，后台轮询却起了 %d 份（每份都是永不退出的 goroutine + 一套轮询请求）", runs)
	}
}

// TestRefreshTerminalNotPoisonedBeforeTerminalOpens 钉「终端还没开时不留在途标记」。
//
// 修之前的时序：启动后第一个 tick 时 termID 还是零值 → 先置 termPending=true
// 再 return；复位只发生在请求返回之后，而这条路径根本没发请求 ⇒ 标记永久为 true。
// 之后用户点开终端、`go m.refreshTerminal()` 全部走 pending 分支直接返回，
// terminal.output 一次都不发，TerminalRead 永远空串（面板看起来像"终端没输出"）。
func TestRefreshTerminalNotPoisonedBeforeTerminalOpens(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"result":{"output":"","next":0}}`)
	}))
	t.Cleanup(server.Close)

	m := &model{cli: apiclient.New(server.URL)}

	// ① 启动时终端还没开：这一步不许留下任何在途标记。
	m.refreshTerminal()
	m.mu.Lock()
	pending := m.termPending
	m.mu.Unlock()
	if pending {
		t.Fatal("termID 为空时 refreshTerminal 不许置在途标记——复位只在请求返回后，这条路径不会返回")
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("没有终端时不该发 terminal.output，实际发了 %d 次", n)
	}

	// ② 用户开了终端：必须真的发一次请求（旧实现这里恒为 0 次）。
	m.mu.Lock()
	m.termID, m.termText, m.termNext = "t1", "", 0
	m.mu.Unlock()
	m.refreshTerminal()
	if n := hits.Load(); n != 1 {
		t.Fatalf("开了终端之后必须发一次 terminal.output，实际 %d 次", n)
	}
	m.mu.Lock()
	pending = m.termPending
	m.mu.Unlock()
	if pending {
		t.Fatal("请求返回后在途标记必须复位")
	}
}
