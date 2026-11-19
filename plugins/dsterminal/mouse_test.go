package dsterminal

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// 这一组钉住**鼠标上报**：指针手势什么时候变成 xterm 序列写回 PTY、什么时候
// 归本地选区。判据全在 mouse.go 的文件头（每条带官方 byte 偏移）。
//
// 断言写的是**字面量字节**（不引用常量），序列形状被改坏时测试要做的是红，
// 不是跟着常量一起漂。

// terminalWrite 是插件写给 PTY 的一次调用（数据 + typed）。
type terminalWrite struct {
	tabID string
	data  string
	typed bool
}

// mouseOn 让模拟器收到程序的 DECSET（与真终端同一条路：字节喂进 VT 屏幕）。
func (f *selectionFixture) mouseOn(seq string) {
	f.t.Helper()
	if _, err := f.emu.Write([]byte(seq)); err != nil {
		f.t.Fatalf("写入 DECSET 失败：%v", err)
	}
}

// wroteData 取这一段手势写出去的序列。
func (f *selectionFixture) wroteData() []string {
	out := make([]string, 0, len(f.wrote))
	for _, w := range f.wrote {
		out = append(out, w.data)
	}
	return out
}

// wantWrote 断言写出去的序列**逐条相等**。
func (f *selectionFixture) wantWrote(stage string, want ...string) {
	f.t.Helper()
	got := f.wroteData()
	if len(got) != len(want) {
		f.t.Fatalf("%s：该写 %d 条 %q，实际 %d 条 %q", stage, len(want), want, len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			f.t.Fatalf("%s：第 %d 条该是 %q，实际 %q（全部 %q）", stage, i, want[i], got[i], got)
		}
	}
}

// TestTerminalMouseReportsSGRSequence 是本单主链路：1002 + 1006 打开之后，
// 按下 → 拖动 → 抬起变成三条 SGR 序列（格坐标 1-based、拖动带 bit5，
// 抬起是小写 m），而**本地选区一条都不产生**（官方协议一开就 disable 选区）。
func TestTerminalMouseReportsSGRSequence(t *testing.T) {
	f := newSelectionFixture(t, 12, 4, "abc\r\ndef\r\n")
	f.mouseOn("\x1b[?1002h\x1b[?1006h")
	f.paint()
	f.press(1, 0, gfx.PointerButtonPrimary, 0, 1)
	f.move(3, 1)
	f.release(3, 1)
	// 按下 (1,0) → Cb=0、Cx=2、Cy=1；拖动 (3,1) → Cb=32|0=32；
	// 抬起 (3,1) → 真实键号 0 + 小写 m（官方 byte 266313）。
	f.wantWrote("按下/拖动/抬起",
		"\x1b[<0;2;1M", "\x1b[<32;4;2M", "\x1b[<0;4;2m")
	if txt := f.selectionText(); txt != "" {
		t.Fatalf("鼠标上报开着时本地选区该是空的，实际 %q", txt)
	}
}

// TestTerminalMouseOffKeepsSelection 钉住反向：没开鼠标模式时一个字节都不写，
// 拖动照旧是本地选区（既有行为不能被这条改动带坏）。
func TestTerminalMouseOffKeepsSelection(t *testing.T) {
	f := newSelectionFixture(t, 10, 4, "abc\r\ndef\r\n")
	f.paint()
	f.press(1, 0, gfx.PointerButtonPrimary, 0, 1)
	f.move(2, 1)
	f.release(2, 1)
	f.wantWrote("没开鼠标模式")
	if got, want := f.selectionText(), "bc"+sep+"de"; got != want {
		t.Fatalf("拖动该选出 %q，实际 %q", want, got)
	}
}

// TestTerminalMouseShiftForcesSelection 钉住官方那条冲突规则
// （`shouldForceSelection`：非 mac 就是 Shift，byte 120764）：
//
//   - 按下**不上报**（这一下归本地选区）；
//   - 选区照常拖出来；
//   - 移动**照发**（官方 mousemove 监听器只看 buttons，byte 27240，不看 Shift）；
//   - 抬起也不上报（没报过按下就没有抬起）。
func TestTerminalMouseShiftForcesSelection(t *testing.T) {
	f := newSelectionFixture(t, 10, 4, "abc\r\ndef\r\n")
	f.mouseOn("\x1b[?1002h\x1b[?1006h")
	f.paint()
	f.press(1, 0, gfx.PointerButtonPrimary, gfx.ModifierShift, 1)
	f.move(2, 1)
	f.release(2, 1)
	f.wantWrote("Shift 强制选区", "\x1b[<32;3;2M") // 只有拖动那一帧（列 2 → Cx=3）
	if got, want := f.selectionText(), "bc"+sep+"de"; got != want {
		t.Fatalf("Shift 按下该归本地选区（%q），实际 %q", want, got)
	}
}

// TestTerminalMouseVT200SkipsMotion 钉住协议能力表：1000 = VT200 只报
// 按下/抬起/滚轮（events 19），拖动那帧**不发**（官方 restrict，byte 265818）。
func TestTerminalMouseVT200SkipsMotion(t *testing.T) {
	f := newSelectionFixture(t, 12, 4, "abc\r\n")
	f.mouseOn("\x1b[?1000h\x1b[?1006h")
	f.paint()
	f.press(1, 0, gfx.PointerButtonPrimary, 0, 1)
	f.move(3, 1)
	f.release(3, 1)
	f.wantWrote("VT200", "\x1b[<0;2;1M", "\x1b[<0;4;2m")
}

// TestTerminalMouseDedupesMotion 钉住官方的移动去重（byte 268572）：
// 同一格连着两帧只发一次。
func TestTerminalMouseDedupesMotion(t *testing.T) {
	f := newSelectionFixture(t, 12, 5, "abc\r\n")
	f.mouseOn("\x1b[?1003h\x1b[?1006h")
	f.paint()
	f.press(1, 0, gfx.PointerButtonPrimary, 0, 1)
	f.move(3, 1)
	f.move(3, 1) // 同一格再来一帧：官方 `_equalEvents` 判重，不发
	f.move(4, 1)
	f.release(4, 1)
	f.wantWrote("拖动去重",
		"\x1b[<0;2;1M", "\x1b[<32;4;2M", "\x1b[<32;5;2M", "\x1b[<0;5;2m")
}

// TestTerminalMouseSGRPixelsUsesPixels 钉住 1016：坐标换成**像素、不加一**
// （官方 SGR_PIXELS，byte 266393）。测试格宽格高是 10×16，格中心就是
// (col+0.5)*10 像素。
func TestTerminalMouseSGRPixelsUsesPixels(t *testing.T) {
	f := newSelectionFixture(t, 12, 4, "abc\r\n")
	f.mouseOn("\x1b[?1002h\x1b[?1016h")
	f.paint()
	f.press(1, 0, gfx.PointerButtonPrimary, 0, 1)
	f.release(1, 0)
	f.wantWrote("1016 像素坐标", "\x1b[<0;15;8M", "\x1b[<0;15;8m")
}

// TestTerminalMouseRightClickReportsAndCopies 钉住右键的两件事一起做：
// 官方 contextmenu 那条不看鼠标模式（byte 17027），当鼠标模式开着时，
// 右键既上报给程序，又把选区写进剪贴板。
func TestTerminalMouseRightClickReportsAndCopies(t *testing.T) {
	f := newSelectionFixture(t, 10, 4, "abc\r\n")
	f.paint()
	// 先在没有鼠标模式时选一段。
	f.press(0, 0, gfx.PointerButtonPrimary, 0, 1)
	f.move(2, 0)
	f.release(2, 0)
	if len(f.clipped) != 0 {
		t.Fatalf("选字不该写剪贴板，实际 %v", f.clipped)
	}
	f.mouseOn("\x1b[?1000h\x1b[?1006h")
	f.paint()
	f.press(0, 0, gfx.PointerButtonSecondary, 0, 1)
	x, y := f.cellCenter(0, 0)
	f.pointer(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonSecondary,
		X: x, Y: y, Target: f.hit})
	f.wantWrote("右键", "\x1b[<2;1;1M", "\x1b[<2;1;1m")
	if len(f.clipped) != 1 || f.clipped[0] != "ab" {
		t.Fatalf("右键该把选区 %q 写进剪贴板，实际 %v", "ab", f.clipped)
	}
}

// TestTerminalMouseWheelGoesToPTY 钉住滚轮：鼠标上报开着时滚轮是给程序的
// （Cb 64 往上、65 往下），本地滚回不动（官方 `s.wheel` 那支无条件吃掉）。
func TestTerminalMouseWheelGoesToPTY(t *testing.T) {
	f := newSelectionFixture(t, 12, 4, "row-0\r\nrow-1\r\nrow-2\r\nrow-3\r\nrow-4\r\nrow-5\r\n")
	f.mouseOn("\x1b[?1002h\x1b[?1006h")
	f.paint()
	x, y := f.cellCenter(2, 1)
	result := f.plugin.HandleScroll(gfx.ScrollEvent{Target: f.hit, X: x, Y: y, DeltaY: -48})
	if !result.Handled || result.Repaint {
		t.Fatalf("鼠标上报开着时滚轮该被吃掉且不重画，实际 %+v", result)
	}
	f.wantWrote("滚上", "\x1b[<64;3;2M")
	// 往下滚是 65（官方 `r=i<0?0:1` + `i|=64`，byte 25864 / 265934）。
	if result := f.plugin.HandleScroll(gfx.ScrollEvent{Target: f.hit, X: x, Y: y, DeltaY: 48}); !result.Handled {
		t.Fatalf("滚下也该被吃掉，实际 %+v", result)
	}
	f.wantWrote("滚上+滚下", "\x1b[<64;3;2M", "\x1b[<65;3;2M")
}

// TestTerminalMouseWheelX10FallsBackToScrollback 钉住 X10 那一档**不收滚轮**
// （events 没有 bit16，官方 byte 26995 的 `16&e?s.wheel`）：照旧滚本地滚回。
func TestTerminalMouseWheelX10FallsBackToScrollback(t *testing.T) {
	f := newSelectionFixture(t, 12, 4, "row-0\r\nrow-1\r\nrow-2\r\nrow-3\r\nrow-4\r\nrow-5\r\n")
	f.mouseOn("\x1b[?9h")
	f.paint()
	x, y := f.cellCenter(2, 1)
	result := f.plugin.HandleScroll(gfx.ScrollEvent{Target: f.hit, X: x, Y: y, DeltaY: -48})
	if !result.Handled || !result.Repaint {
		t.Fatalf("X10 下滚轮该走本地滚回（有历史 → 重画），实际 %+v", result)
	}
	f.wantWrote("X10 滚轮不上报")
}

// TestTerminalMouseReportsAreProtocolReplies 钉住 typed：鼠标上报是协议回复，
// 不是人员敲的字，必须 typed=false。
func TestTerminalMouseReportsAreProtocolReplies(t *testing.T) {
	f := newSelectionFixture(t, 12, 4, "abc\r\n")
	f.mouseOn("\x1b[?1002h\x1b[?1006h")
	f.paint()
	f.press(1, 0, gfx.PointerButtonPrimary, 0, 1)
	if len(f.wrote) != 1 {
		t.Fatalf("该写一条按下，实际 %v", f.wrote)
	}
	if f.wrote[0].typed {
		t.Fatalf("鼠标上报该是 typed=false（协议回复），实际 %+v", f.wrote[0])
	}
	if f.wrote[0].tabID != "terminal" {
		t.Fatalf("上报该写到这条 occurrence（terminal），实际 %q", f.wrote[0].tabID)
	}
}

// TestTerminalMouseSideButtonsAreNotReported 钉住侧键：官方把 DOM button>=3
// 折成 3，而 3 的按下被丢掉（byte 25458 / 268461），所以侧键不上报、
// 也不该吃掉本地选区那一下。
func TestTerminalMouseSideButtonsAreNotReported(t *testing.T) {
	f := newSelectionFixture(t, 12, 4, "abc\r\n")
	f.mouseOn("\x1b[?1002h\x1b[?1006h")
	f.paint()
	f.press(1, 0, gfx.PointerButtonBack, 0, 1)
	f.wantWrote("侧键")
}
