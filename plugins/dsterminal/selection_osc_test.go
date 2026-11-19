package dsterminal

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// 这一组钉住「程序用 OSC 11 换过默认背景之后，选区与滚回区的默认底格都跟着它」。
//
// 官方那条链是 `update(background, foreground)`（terminal-theme.ts:57-64，调用点
// terminal.tsx:124-127）：xterm 的 backgroundColor / foregroundColor 就是
// OSC 11 / 10 写下的值，而 `terminal-theme.ts:61` 又把
//
//	selectionBackground: foreground, selectionForeground: background
//
// 交给 xterm —— 所以**选区底色与字色都是那两个会变的默认色算出来的**，
// 不是主题常量。同理整屏默认底格也是 xterm 的 `theme.background`
// （xterm 把它画满整个 viewport），滚回窗口里一样要铺。
//
// 两条测试都从**程序写下的字节**出发（不是直接塞 Defaults），这样「OSC 序列 →
// 模拟器状态 → 渲染取值」整条链都在断言范围内。

// osc11DefaultBackground 是程序在测试里写的默认背景（一个不会与主题撞上的色）。
const osc11DefaultBackground = "#101010"

// osc11Set 是 OSC 11 的 SET 序列（`OSC 11 ; <color> ST`）。
func osc11Set(color string) string { return "\x1b]11;" + color + "\x1b\\" }

// osc11WantBlend 用官方口径算一遍「底色 = blend(background, foreground, 0.3)」，
// 其中 background/foreground 是程序改过之后的那两个默认色。
//
// 0.3 写成字面量而不是引用常量：引用常量的断言会跟着常量一起漂。
func osc11WantBlend(foreground, background gfx.Color) gfx.Color {
	mix := func(b, fg float32) float32 { return b + ((fg - b) * 0.3) }
	return gfx.Color{
		Red:   mix(background.Red, foreground.Red),
		Green: mix(background.Green, foreground.Green),
		Blue:  mix(background.Blue, foreground.Blue),
		Alpha: 1,
	}
}

// TestTerminalSelectionPaintFollowsOSC11Background 钉住选区两件事：
//
//   - 选中底色 = blend(OSC 11 背景, 默认前景, 0.3)，而不是 blend(主题 bg-base, …)；
//   - 选中字色 = OSC 11 背景（官方 selectionForeground: background）。
//
// 判据用**两个色都算一遍**的方式写死：只要实现里丢了 `def`，断言拿到的就是主题
// 那一套，测试必红（见任务书的变异要求）。
func TestTerminalSelectionPaintFollowsOSC11Background(t *testing.T) {
	f := newSelectionFixture(t, 10, 3, osc11Set(osc11DefaultBackground), "abcdef\r\n")
	def := f.emuGrid().Default
	if !def.Bg.Set {
		t.Fatalf("OSC 11 之后屏幕默认背景该是程序写的颜色，实际没被设置：%+v", def)
	}
	bg := gridColor(def.Bg)
	fg := f.plugin.theme.LabelPrimary // def.Fg 没被 OSC 10 改过 → 默认前景仍是主题的
	if bg == f.plugin.theme.BgBase {
		t.Fatalf("测试前提不成立：OSC 11 写的 %#v 与主题 bg-base 相同", bg)
	}

	f.paint()
	f.press(1, 0, gfx.PointerButtonPrimary, 0, 1)
	f.move(3, 0)
	f.release(3, 0)
	f.paint()

	want := osc11WantBlend(fg, bg)
	themeWant := osc11WantBlend(f.plugin.theme.LabelPrimary, f.plugin.theme.BgBase)
	if want == themeWant {
		t.Fatalf("测试前提不成立：两种口径算出的选中底色相同（%#v）", want)
	}

	var fills []selectionFill
	for _, rect := range f.canvas.Rectangles() {
		if rect.Color == want {
			fills = append(fills, selectionFill{rect: rect.Bounds, color: rect.Color})
		}
	}
	if len(fills) != 1 {
		t.Fatalf("列 1..3 该是一整块「基于 OSC 11 背景」的选中底色 %#v，实际 %d 块（主题口径的 %#v 也算不进）",
			want, len(fills), themeWant)
	}
	originX, originY := gridTestOrigin()
	if wantX := originX + 1*gridTestCellW; fills[0].rect.X != wantX {
		t.Fatalf("选中底色该从列 1（x=%v）起，实际 %v", wantX, fills[0].rect.X)
	}
	if fills[0].rect.Y != originY {
		t.Fatalf("选中底色该在第 0 行（y=%v），实际 %v", originY, fills[0].rect.Y)
	}
	if fills[0].rect.Width != 2*gridTestCellW {
		t.Fatalf("列 1..3 该宽两格 %v，实际 %v", 2*gridTestCellW, fills[0].rect.Width)
	}

	// 选中格的字色换成 OSC 11 写下的背景色（官方 selectionForeground: background）。
	seen := 0
	for _, run := range gridRuns(f.canvas, 0) {
		column := run.column
		if column < 1 || column >= 3 {
			continue
		}
		seen++
		if run.color != bg {
			t.Fatalf("选中的列 %d 字色该是 OSC 11 背景 %#v，实际 %#v", column, bg, run.color)
		}
	}
	if seen == 0 {
		t.Fatalf("第 0 行没有取到选中列 1..3 的字形 run：%+v", gridRuns(f.canvas, 0))
	}
}

// TestTerminalScrollbackDefaultBackgroundFollowsOSC11 钉住滚回窗口的默认底格：
// 往上滚进历史之后，整屏仍然要先铺 OSC 11 的背景。
//
// 这条覆盖的是 `paintGrid` 里「滚回窗口分支提前 return、不铺整屏底色」那个缺口：
// 不铺的话，没显式底色的历史格会露出面板自己的 bg-base，与程序要的颜色差一层。
func TestTerminalScrollbackDefaultBackgroundFollowsOSC11(t *testing.T) {
	const cols, rows = 12, 4
	h := newScrollHarness(t, cols, rows, 100)
	h.write(osc11Set(osc11DefaultBackground))
	h.lines(12) // 12 行写进 4 行屏 → 滚回 8 行
	if got := h.state().ScrollbackLen; got != 8 {
		t.Fatalf("12 行写进 4 行屏该有 8 行滚回，实际 %d", got)
	}
	def := h.state().Grid.Default
	if !def.Bg.Set {
		t.Fatalf("OSC 11 之后屏幕默认背景该是程序写的颜色，实际没被设置：%+v", def)
	}
	bg := gridColor(def.Bg)

	// 先画一帧声明滚轮命中区，再往上滚到顶（这一帧必须真的落在滚回分支里）。
	h.draw()
	for i := 0; i < 3; i++ {
		h.scroll(-48, false)
	}
	h.draw()
	if h.plugin.wheelYdisp >= h.plugin.wheelScrollback {
		t.Fatalf("往上滚之后该在滚回分支里（ydisp < 滚回行数），实际 ydisp=%d 滚回=%d",
			h.plugin.wheelYdisp, h.plugin.wheelScrollback)
	}

	// 判据：有一块**颜色正好是 OSC 11 背景**、面积覆盖整屏格数、并且包含网格左上角
	// 的矩形。不钉死它的起点坐标——面板内距由壳给，这条测试钉的是「铺过整屏」。
	x0, y0 := gridTestOrigin()
	covers := func(r gfx.Rect) bool {
		return x0 >= r.X && x0 <= r.X+r.Width && y0 >= r.Y && y0 <= r.Y+r.Height
	}
	for _, rect := range h.canvas.Rectangles() {
		if rect.Color != bg {
			continue
		}
		if rect.Bounds.Width < float32(cols)*gridTestCellW || rect.Bounds.Height < float32(rows)*gridTestCellH {
			continue
		}
		if !covers(rect.Bounds) {
			continue
		}
		return // 找到了：整屏铺过 OSC 11 的背景
	}
	t.Fatalf("滚回窗口没有整屏铺 OSC 11 的默认背景 %#v（该覆盖 %d×%d 格 = %v×%v）：图元=%v",
		bg, cols, rows, float32(cols)*gridTestCellW, float32(rows)*gridTestCellH, h.canvas.Rectangles())
}
