package main

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/vterm"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

// TestTerminalViewStateCarriesMouseReporting 钉住壳这一侧唯一的那条投影：
// 程序在 PTY 那头用 DECSET 打开的鼠标上报，必须出现在每帧的视图快照里。
//
// 为什么这条非有不可：模式是**程序写进 PTY 的字节**解出来的，只有壳里那块 VT
// 屏幕知道（`internal/vterm/mouse.go`）；而"这一下按下归 PTY 还是归本地选区"
// 是界面那一层的判断（官方 `bindMouse`：上报开着且没按 Shift 就上报）。两边
// 的判据必须来自同一个状态，否则会出现"界面在选字、程序在等鼠标"的错位。
//
// 与 terminal_scrollback_test.go 同一个形状：直接用一块真 VT 屏幕造视图，
// 不起 HTTP——契约就是"从 view 状态里读"，与 attach 从哪来无关。
func TestTerminalViewStateCarriesMouseReporting(t *testing.T) {
	m := &model{cur: "s1"}
	emu := vterm.New(20, 4)
	m.terminalViews = map[string]*terminalViewState{
		userTerminalKey("s1", "tab1"): {
			public: pluginapi.TerminalViewState{Phase: "connected"},
			emu:    emu,
		},
	}
	if got := m.terminalSnapshot("tab1").Mouse; got.Reporting() {
		t.Fatalf("没开鼠标模式时快照该是零值（不报），实际 %+v", got)
	}

	// 程序打开 DRAG + SGR：官方 `setModePrivate` 的 ?1002h / ?1006h。
	if _, err := emu.Write([]byte("\x1b[?1002h\x1b[?1006h")); err != nil {
		t.Fatalf("写入 DECSET 失败：%v", err)
	}
	got := m.terminalSnapshot("tab1").Mouse
	if got.Protocol != vterm.MouseProtocolDrag || got.Encoding != vterm.MouseEncodingSGR {
		t.Fatalf("快照该带 DRAG + SGR，实际 %+v", got)
	}

	// DECRST 1003 这一档关掉上报（官方：9/1000/1002/1003 任一复位都回 NONE）。
	if _, err := emu.Write([]byte("\x1b[?1003l")); err != nil {
		t.Fatalf("写入 DECRST 失败：%v", err)
	}
	if got := m.terminalSnapshot("tab1").Mouse; got.Reporting() {
		t.Fatalf("DECRST 之后该关掉上报，实际 %+v", got)
	}
}
