package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/vterm"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

// 这一组钉住**壳这一侧的滚回**：
//
//  1. 容量按官方 xterm 的 `scrollback` 选项（= 宿主 environment.scrollback），
//     不是上游默认的 10000；
//  2. TerminalViewState 每帧带上 ScrollbackLen（一次读锁，便宜）；
//  3. TerminalScrollbackWindow 按坐标切窗口（贵的那一步只碰可见的几行）。
//
// 渲染与交互在 plugins/dsterminal 里；这里只验壳给出去的那三样东西是对的。

// scrollbackScreen 造 n 行屏幕文本（行号写在内容里）。
func scrollbackScreen(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "l%02d", i)
		if i != n-1 {
			b.WriteString("\r\n")
		}
	}
	return b.String()
}

// TestTerminalViewStateCarriesScrollback 钉住「快照里带滚回长度、窗口按坐标切」。
//
// 直接用一条已经攒了历史的 VT 屏幕造视图（不起 HTTP）：这三个接口的契约就是
// 「从 view 状态里读」，与 attach 从哪来无关。
func TestTerminalViewStateCarriesScrollback(t *testing.T) {
	m := &model{cur: "s1"}
	emu := newScrollbackEmulator(t, 20, 4, 100, 12)
	m.terminalViews = map[string]*terminalViewState{
		userTerminalKey("s1", "tab1"): {
			public: pluginapi.TerminalViewState{Phase: "connected"},
			emu:    emu,
		},
	}
	state := m.terminalSnapshot("tab1")
	if state.ScrollbackLen != emu.ScrollbackLen() || state.ScrollbackLen != 8 {
		t.Fatalf("快照该带 8 行滚回（12 行写进 4 行屏），实际 %d", state.ScrollbackLen)
	}
	if state.Grid == nil {
		t.Fatal("当前屏网格丢了")
	}

	// 窗口的坐标口径：0 = 最老的一行，ScrollbackLen = 第一行屏幕行。
	oldest := m.terminalScrollbackWindow("tab1", 0, 1)
	if oldest == nil || oldest.Height != 1 {
		t.Fatalf("窗口该有 1 行，实际 %+v", oldest)
	}
	if got := gridFirstLine(oldest); got != "l00" {
		t.Fatalf("窗口第一行该是最老的 l00，实际 %q", got)
	}
	screenFirst := m.terminalScrollbackWindow("tab1", state.ScrollbackLen, 1)
	if screenFirst == nil || gridFirstLine(screenFirst) != gridFirstLine(state.Grid) {
		t.Fatalf("从 ScrollbackLen 起的窗口该是屏幕第一行，实际 %+v", screenFirst)
	}
	// 老壳/未知 tab：返回 nil（界面据此按「没有历史」画，不假装能滚）。
	if got := m.terminalScrollbackWindow("tab1-missing", 0, 4); got != nil {
		t.Fatalf("没有这条终端时窗口该是 nil，实际 %+v", got)
	}

	// resize 之后历史不丢（验收第三条）：上游 Resize 不碰 scrollback。
	emu.Resize(30, 6)
	if got := m.terminalSnapshot("tab1").ScrollbackLen; got == 0 {
		t.Fatal("resize 之后快照里的滚回被清空了")
	}
	afterResize := m.terminalScrollbackWindow("tab1", 0, 1)
	if afterResize == nil || gridFirstLine(afterResize) != "l00" {
		t.Fatalf("resize 之后窗口第一行仍该是 l00，实际 %+v", afterResize)
	}
}

// newScrollbackEmulator 造一块写了 lines 行的屏幕（用真 vterm，不造假快照）。
func newScrollbackEmulator(t *testing.T, cols, rows, capacity, lines int) *vterm.Emulator {
	t.Helper()
	emu := vterm.New(cols, rows)
	emu.SetScrollbackSize(capacity)
	if _, err := emu.Write([]byte(scrollbackScreen(lines))); err != nil {
		t.Fatalf("写入模拟器失败：%v", err)
	}
	return emu
}

// gridFirstLine 把网格第一行读成去掉尾部空白的文本。
func gridFirstLine(g *vterm.Grid) string {
	if g == nil || g.Height == 0 {
		return ""
	}
	var b strings.Builder
	for x := 0; x < g.Width; x++ {
		cell, ok := g.Cell(x, 0)
		if !ok {
			break
		}
		if cell.Width == 0 {
			continue
		}
		if cell.Content == "" {
			b.WriteByte(' ')
			continue
		}
		b.WriteString(cell.Content)
	}
	return strings.TrimRight(b.String(), " ")
}

// TestTerminalWiringAppliesHostScrollbackCapacity 钉住**容量来自宿主**：
// 官方 `terminal.tsx:104` 把 xterm 的 `scrollback` 设成 `environment?.scrollback ?? 0`，
// 上游默认 10000 行必须被这个数盖掉——否则滚回比官方深一个数量级，
// 一次全量取帧要碰的格子也多一个数量级。
//
// 用一个只想「留下 8 行」的宿主验：20 行写进 4 行屏本该有 16 行滚回，
// 容量生效后只能剩 8 行，且留下的必须是**最后** 8 行（l08..l15）。
func TestTerminalWiringAppliesHostScrollbackCapacity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, "/api/")
		switch method {
		case "terminal.environment":
			writeTerminalTestResult(w, `{"cwd":"/work","maxInputBytes":65536,"maxBufferedBytes":65536,"maxCols":500,"maxRows":200,"scrollback":8}`)
		case "terminal.shells":
			writeTerminalTestResult(w, `[{"path":"/bin/bash","args":[],"name":"bash"}]`)
		case "terminal.list":
			writeTerminalTestResult(w, `[]`)
		case "terminal.create":
			writeTerminalTestResult(w, `{"id":"tab1","title":"bash","shell":{"path":"/bin/bash","args":[],"name":"bash"},"cwd":"/work","cols":20,"rows":4,"state":"running","exitCode":null,"origin":"user","control":"user"}`)
		case "terminal.retain":
			w.Header().Set("Content-Type", "application/x-ndjson")
			fmt.Fprintln(w, `{"type":"retained"}`)
			flushTerminalTest(w)
			<-r.Context().Done()
		case "terminal.attach":
			w.Header().Set("Content-Type", "application/x-ndjson")
			fmt.Fprintf(w, `{"type":"snapshot","sequence":0,"screen":%q,"info":{"id":"tab1","title":"bash","shell":{"path":"/bin/bash","args":[],"name":"bash"},"cwd":"/work","cols":20,"rows":4,"state":"running","exitCode":null,"origin":"user","control":"user"},"writable":true}`+"\n", scrollbackScreen(20))
			flushTerminalTest(w)
			<-r.Context().Done()
		case "terminal.watch":
			w.Header().Set("Content-Type", "application/x-ndjson")
			fmt.Fprintln(w, `[{"id":"tab1","shell":{"path":"/bin/bash","args":[]},"cwd":"/work","cols":20,"rows":4,"state":"running","origin":"user","control":"user"}]`)
			flushTerminalTest(w)
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer closeStreamingServer(srv)
	m := &model{cli: apiclient.New(srv.URL), cur: "s1"}
	m.terminalEnsure("tab1")
	waitTerminalConditionDump(t, func() bool {
		return m.terminalSnapshot("tab1").Phase == "connected"
	}, func() string {
		st := m.terminalSnapshot("tab1")
		return fmt.Sprintf("phase=%q screen=%q sb=%d", st.Phase, st.Screen, st.ScrollbackLen)
	})
	state := m.terminalSnapshot("tab1")
	if state.Environment == nil || state.Environment.Scrollback != 8 {
		t.Fatalf("environment.scrollback 该是宿主给的 8，实际 %+v", state.Environment)
	}
	if state.ScrollbackLen != 8 {
		t.Fatalf("容量 8 行时滚回只能是 8 行（20 行写进 4 行屏本该 16 行），实际 %d", state.ScrollbackLen)
	}
	oldest := m.terminalScrollbackWindow("tab1", 0, 1)
	if oldest == nil || gridFirstLine(oldest) != "l08" {
		t.Fatalf("留下的必须是最新 8 行（l08 起），实际 %+v", oldest)
	}
	m.mu.Lock()
	for _, watch := range m.terminalWatchers {
		watch.cancel()
	}
	for _, view := range m.terminalViews {
		if view.cancel != nil {
			view.cancel()
		}
	}
	m.mu.Unlock()
}
