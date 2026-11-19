package dsterminal

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/vterm"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 这一组钉住滚回的三件事：
//
//  1. 换算：14px 槽与 fit **一起**算（官方 FitAddon 的 proposeDimensions）；
//  2. 交互：滚轮走几行（官方 `scrollSensitivity:1` + 像素→行）；
//  3. 呈现：往上滚真的画出历史行、滚到底回到当前屏（带光标）、resize 之后历史还在。
//
// 数字全部来自官方包里内联的 xterm（见 scrollback.go 顶部逐条字节偏移），
// 不是我们估的：格宽格高由测试给（10×16），只验证算术与走位。

// TestTerminalFitSizeReservesScrollbarGutter 钉住「14px 槽与 fit 一起算」。
//
// 官方 `proposeDimensions` 是 `cols = floor((宽度 - 内距 - t) / 格宽)`，
// 其中 `t = scrollback===0 ? 0 : overviewRuler?.width||14`。少扣一列会被滑块
// 盖住，多扣一列会留一条空槽——所以这条同时钉住「扣」和「不扣」两种面板。
func TestTerminalFitSizeReservesScrollbarGutter(t *testing.T) {
	for _, tc := range []struct {
		name         string
		rectW        float32
		gutter       float32
		wantCols     int
		scrollback   int
		gutterWanted float32
	}{
		{name: "scrollback=0 不留槽：344/10=34 列", rectW: 344, scrollback: 0, gutterWanted: 0, wantCols: 34},
		{name: "scrollback≠0 留 14px：344-14=330 → 33 列", rectW: 344, scrollback: 1000, gutterWanted: 14, wantCols: 33},
		{name: "槽宽是官方 14，不是 15/16", rectW: 344, scrollback: 1, gutterWanted: 14, wantCols: 33},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := terminalScrollbarGutter(tc.scrollback); got != tc.gutterWanted {
				t.Fatalf("scrollback=%d 的槽宽该是 %v，实际 %v", tc.scrollback, tc.gutterWanted, got)
			}
			cols, rows := terminalFitSize(tc.rectW, 400, 10, 16, 0, 0, terminalScrollbarGutter(tc.scrollback))
			if cols != tc.wantCols {
				t.Fatalf("扣掉 %v 槽之后该是 %d 列，实际 %d", terminalScrollbarGutter(tc.scrollback), tc.wantCols, cols)
			}
			if rows != 25 {
				t.Fatalf("行数不该被槽影响（槽只扣宽度），实际 %d", rows)
			}
		})
	}
}

// TestTerminalFitSizeWithoutRoomForGutter 钉住「扣完槽连一格都不剩」时不发 resize，
// 而不是发出 0 列或负数列去撞宿主。
func TestTerminalFitSizeWithoutRoomForGutter(t *testing.T) {
	if cols, rows := terminalFitSize(18, 400, 10, 16, 0, 0, 14); cols != 0 || rows != 0 {
		t.Fatalf("18px 面板扣 14px 槽后放不下一格，该回 (0,0)，实际 %d×%d", cols, rows)
	}
}

// TestTerminalWheelLinesFollowsXterm 钉住滚轮换算：像素 → 行，含小数余量与
// 50px 阻尼（官方 `_getLinesFromWheelEvent`，byte 268232）。
//
// 本仓平台给的是逻辑像素：经典一档 = 系统设置的 N 行 × 16px（默认 3 行 = 48px），
// 所以「一档 = 3 行」这个结论是从算术里出来的，不是拍出来的。
func TestTerminalWheelLinesFollowsXterm(t *testing.T) {
	for _, tc := range []struct {
		name        string
		deltaY      float32
		precise     bool
		modifiers   gfx.Modifiers
		partial     float64
		wantLines   int
		wantPartial float64
	}{
		// 48px / 16px = 3 行：Windows 默认「每档 3 行」。
		{name: "经典一档 = 3 行", deltaY: -48, wantLines: -3},
		{name: "经典两档 = 6 行", deltaY: -96, wantLines: -6},
		{name: "往下滚是正的", deltaY: 48, wantLines: 3},
		// 5 行设置：80px / 16 = 5 行。
		{name: "5 行一档 = 5 行", deltaY: 80, wantLines: 5},
		// Alt 是官方的 fastScrollModifier：×5。
		{name: "Alt 快滚 ×5", deltaY: -48, modifiers: gfx.ModifierAlt, wantLines: -15},
		{name: "Ctrl 同样快滚 ×5", deltaY: -16, modifiers: gfx.ModifierControl, wantLines: -5},
		// 触控板的精确增量：先按像素→行，|delta|<50 再 ×0.3，不足一行攒着。
		{name: "触控板 30px → 0 行（0.5625 攒着）", deltaY: -30, precise: true, wantLines: 0, wantPartial: -0.5625},
		{name: "触控板攒够一行才走", deltaY: -30, precise: true, partial: -0.5625, wantLines: -1, wantPartial: -0.125},
		// 没有阻尼时（非精确）同样距离：30/16 = 1.875 → 1 行，余下 -0.875 攒着。
		{name: "非精确 30px = 1 行", deltaY: -30, wantLines: -1, wantPartial: -0.875},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines, partial := terminalWheelLines(gfx.ScrollEvent{DeltaY: tc.deltaY, Precise: tc.precise, Modifiers: tc.modifiers}, 16, tc.partial)
			if lines != tc.wantLines {
				t.Fatalf("deltaY=%v precise=%v 该走 %d 行，实际 %d", tc.deltaY, tc.precise, tc.wantLines, lines)
			}
			if diff := partial - tc.wantPartial; diff > 1e-9 || diff < -1e-9 {
				t.Fatalf("小数余量该是 %v，实际 %v", tc.wantPartial, partial)
			}
		})
	}
}

// TestTerminalScrollbarThumbFollowsOfficial 钉住滚动条几何：官方
// `ScrollbarState._computeValues` 的 `max(20, floor(visible*available/scrollSize))`
// 与 `(o-slider)/(scrollSize-visible)` 的比例。
func TestTerminalScrollbarThumbFollowsOfficial(t *testing.T) {
	// 可见 400px、内容 8000px（1000 行 × 8px 看不见的那部分）：滑块先撞 20px 下限。
	// 这里写死 20（不是读常量）：官方 `Math.max(20, …)` 的那个 20 是契约的一部分，
	// 读常量的话「常量被改小」这种变异就测不出来了。
	offset, size := terminalScrollbarThumb(400, 8000, 0)
	if size != 20 {
		t.Fatalf("40 行内容里的 1 屏滑块该撞 20px 下限，实际 %v", size)
	}
	if offset != 0 {
		t.Fatalf("贴顶时滑块该在 0，实际 %v", offset)
	}
	// 内容 50 屏（可见 400、内容 20000）：几何值 floor(400*400/20000) = 8，
	// 被官方 20px 下限顶上去——这一条专门钉「下限真的存在」。
	if _, size = terminalScrollbarThumb(400, 20000, 0); size != 20 {
		t.Fatalf("50 屏内容时滑块该被 20px 下限顶住，实际 %v", size)
	}
	// 内容 = 2 屏：滑块 = max(20, floor(400*400/800)) = 200。
	offset, size = terminalScrollbarThumb(400, 800, 0)
	if size != 200 || offset != 0 {
		t.Fatalf("两屏内容该是 200px 滑块、位置 0，实际 %v/%v", offset, size)
	}
	// 滚到一半：position = round(400/2 * (400-200)/(800-400)) = 100。
	if offset, _ = terminalScrollbarThumb(400, 800, 200); offset != 100 {
		t.Fatalf("滚一半时滑块位置该是 100，实际 %v", offset)
	}
	// 不可滚（内容不比可见区高）：不画。
	if offset, size = terminalScrollbarThumb(400, 400, 0); size != 0 || offset != 0 {
		t.Fatalf("不可滚时不该有滑块，实际 %v/%v", offset, size)
	}
}

// scrollHarness 是一条「真模拟器 + 真绘制路径」的终端面板：
//
//	emu 写字节 → Grid()/ScrollbackLen() 进 state → drawPanelBody 画一帧
//	→ HandleScroll 走真实手势 → 再画一帧。
//
// 每帧用新画布：断言读的是**这一帧**画了什么，不会把上一帧的图元混进来。
type scrollHarness struct {
	t      *testing.T
	emu    *vterm.Emulator
	plugin *Plugin
	tree   *gfx.Tree
	feat   gfx.Scope
	canvas *gfx.Canvas
	env    *pluginapi.Env
	hit    gfx.ID
	calls  []terminalSize

	scrollback int // 容量（environment.scrollback）
	cols       int // 模拟器列数
	rows       int // 模拟器行数
}

func newScrollHarness(t *testing.T, cols, rows, scrollback int) *scrollHarness {
	t.Helper()
	h := &scrollHarness{t: t, emu: vterm.New(cols, rows), scrollback: scrollback, cols: cols, rows: rows}
	h.emu.SetScrollbackSize(scrollback)
	theme := dscore.Dark()
	h.tree = &gfx.Tree{}
	h.feat = h.tree.Feature("terminal-scroll")
	h.hit = h.tree.ID(h.feat, "terminal-screen")
	h.plugin = &Plugin{theme: theme}
	h.env = &pluginapi.Env{
		Theme: theme, Tree: h.tree, Feat: h.feat,
		Data: pluginapi.Snapshot{ShowRightbar: true},
	}
	h.env.Act = pluginapi.Actions{
		TerminalEnsure:        func(string) {},
		TerminalSnapshot:      func(string) pluginapi.TerminalViewState { return h.state() },
		TerminalRetry:         func(string) {},
		OpenRightTab:          func(string) {},
		CloseRightTab:         func(string) {},
		TerminalWriteOfficial: func(string, string, bool) {},
		TerminalScrollbackWindow: func(tabID string, first, lines int) *vterm.Grid {
			return h.emu.Window(first, lines)
		},
		TerminalResizeOfficial: func(tabID string, cols, rows int) {
			h.calls = append(h.calls, terminalSize{cols: cols, rows: rows})
		},
	}
	return h
}

// write 往模拟器里真写字节（滚回就是这样攒出来的）。
func (h *scrollHarness) write(s string) {
	h.t.Helper()
	if _, err := h.emu.Write([]byte(s)); err != nil {
		h.t.Fatalf("写入模拟器失败：%v", err)
	}
}

// state 是壳每帧交给插件的那份快照（Grid + ScrollbackLen + environment）。
func (h *scrollHarness) state() pluginapi.TerminalViewState {
	grid := h.emu.Grid()
	return pluginapi.TerminalViewState{
		Phase: "connected", Writable: true,
		Grid:          grid,
		ScrollbackLen: h.emu.ScrollbackLen(),
		Info:          &apiclient.TerminalInfo{ID: "tab1", Title: "终端 1", State: "running", Cols: h.cols, Rows: h.rows},
		Environment:   &apiclient.TerminalEnvironment{MaxCols: 500, MaxRows: 200, Scrollback: h.scrollback},
	}
}

// draw 画一帧并返回这一帧的行文本（行号 0 = 屏幕顶端）。
func (h *scrollHarness) draw() map[int]string {
	h.t.Helper()
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	canvas.SetCellMeasurer(func(size float32, face gfx.MonoFace) (float32, float32) {
		return gridTestCellW, gridTestCellH
	})
	h.canvas = canvas
	env := *h.env
	env.Canvas = canvas
	pageTheme := dscore.VivaTheme(h.plugin.theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	page.Draw(canvas, gfx.Rect{Width: gridTestPageW, Height: gridTestPageH}, func(f ui.C) {
		h.plugin.drawPanelBody(f, &env)
	})
	return gridRowTexts(canvas)
}

// scroll 发一次落在终端上的滚轮。
func (h *scrollHarness) scroll(deltaY float32, precise bool) gfx.EventResult {
	return h.plugin.HandleScroll(gfx.ScrollEvent{Target: h.hit, DeltaY: deltaY, Precise: precise})
}

// lines 造 n 行可读文本（行号写在内容里，方便断言看到了哪几行）。
//
// 最后一行**不**带换行：带了会多滚出一条空行，滚回行数就不是 n-屏幕高 了
// （那是终端语义，不是测试要钉的东西）。
func (h *scrollHarness) lines(n int) {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("row-")
		b.WriteString(itoa(i))
		if i != n-1 {
			b.WriteString("\r\n")
		}
	}
	h.write(b.String())
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var digits []byte
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}

// wantRows 断言这一帧画出来的第 row 行就是 want（行尾空白已去掉）。
func wantRow(t *testing.T, rows map[int]string, row int, want string) {
	t.Helper()
	if got := rows[row]; got != want {
		t.Fatalf("第 %d 行该是 %q，实际 %q（整屏 %v）", row, want, got, rows)
	}
}

// TestTerminalScrollbackShowsHistory 是本单的主渲染测试：
// **往上滚能看见历史行，滚到底回到当前屏**。
func TestTerminalScrollbackShowsHistory(t *testing.T) {
	const cols, rows = 12, 4
	h := newScrollHarness(t, cols, rows, 100)
	// 12 行写进 4 行屏：滚回 8 行，当前屏是 row-8..row-11。
	h.lines(12)
	state := h.state()
	if state.ScrollbackLen != 8 {
		t.Fatalf("12 行写进 4 行屏该有 8 行滚回，实际 %d", state.ScrollbackLen)
	}

	// 贴底：画当前屏，最后一行是刚写进去的 row-11。
	bottom := h.draw()
	wantRow(t, bottom, 0, "row-8")
	wantRow(t, bottom, 3, "row-11")

	// 滚轮往上 3 行（经典一档 48px / 16px 格高）：窗口第一行变成 row-5。
	if result := h.scroll(-48, false); !result.Handled || !result.Repaint {
		t.Fatalf("滚轮往上该被终端接住并重画，实际 %+v", result)
	}
	up := h.draw()
	wantRow(t, up, 0, "row-5")
	wantRow(t, up, 3, "row-8")

	// 再往上滚到顶：第一行是最老的一行，且不越界（再滚还是它）。
	h.scroll(-48, false)
	h.scroll(-48, false)
	if result := h.scroll(-48, false); result.Repaint {
		t.Fatal("已经滚到顶了，再往上不该动")
	}
	top := h.draw()
	wantRow(t, top, 0, "row-0")
	wantRow(t, top, 3, "row-3")

	// 往下滚到底：回到当前屏（最后一帧是 row-8..row-11）。
	for i := 0; i < 4; i++ {
		h.scroll(48, false)
	}
	back := h.draw()
	wantRow(t, back, 0, "row-8")
	wantRow(t, back, 3, "row-11")
	// 贴底之后新输出继续跟随（不会停在历史的某一层）。
	// 贴底之后新输出继续跟随（不会停在历史的某一层）。上一行末尾没有换行，
	// 所以这里补一个换行再写下一行。
	h.write("\r\nrow-12")
	followed := h.draw()
	wantRow(t, followed, 3, "row-12")
}

// TestTerminalScrollbackDeclaresWheelTarget 钉住「滚轮真的落在终端上」：
// 绘制时声明了命中区，HandleScroll 才可能收到这个 id；没有命中区就等于
// 官方没把 wheel 监听挂到 xterm 上——手势会被外层当成页面滚动。
func TestTerminalScrollbackDeclaresWheelTarget(t *testing.T) {
	h := newScrollHarness(t, 12, 4, 100)
	h.lines(12)
	h.draw()
	found := false
	for _, hit := range h.canvas.Hits() {
		if hit.ID == h.hit {
			found = true
			if hit.Bounds.Width <= 0 || hit.Bounds.Height <= 0 {
				t.Fatalf("终端命中区没有面积：%+v", hit.Bounds)
			}
			// 它同时是滚轮路由区**和**焦点判据（官方 .xterm-focus 的对应物，
			// 见 grid.go 的 screenFocused），所以必须可聚焦；但**不占 Tab 站**
			// （SkipTab），也不该被当成按钮读给读屏软件。
			if !hit.Focusable {
				t.Fatalf("终端屏幕命中区必须可聚焦，否则光标永远是失焦态（outline）：%+v", hit)
			}
			if !hit.SkipTab {
				t.Fatalf("终端屏幕命中区不该占一个 Tab 站（SkipTab）：%+v", hit)
			}
			if hit.Role != gfx.RoleGroup {
				t.Fatalf("终端屏幕命中区该是 group 角色，实际 %v", hit.Role)
			}
		}
	}
	if !found {
		t.Fatalf("终端没有声明滚轮命中区 %v；Hits=%v", h.hit, h.canvas.Hits())
	}
	// 别处的滚轮不该被终端吃掉（例如消息区）。
	other := h.plugin.HandleScroll(gfx.ScrollEvent{Target: gfx.Item(999), DeltaY: -48})
	if other.Handled {
		t.Fatalf("落在别处的手势不该被终端接管，实际 %+v", other)
	}
}

// TestTerminalScrollbarDrawnInGutter 钉住「14px 槽里真的画了滑块」：
// fit 扣掉的宽度必须被用上，否则右边留一条空槽（官方那条 14px 是滚动条的）。
func TestTerminalScrollbarDrawnInGutter(t *testing.T) {
	const cols, rows = 12, 4
	h := newScrollHarness(t, cols, rows, 100)
	h.lines(40) // 滚回 36 行，滑块会撞 20px 下限
	h.draw()
	gutter := float32(0)
	if h.state().Environment != nil && h.state().Environment.Scrollback > 0 {
		gutter = terminalScrollbarWidth
	}
	// 槽贴着**面板右边缘**（官方滚动条是 .xterm 元素的右边缘，不是「最后一列
	// 右边」）：screen 内容宽 = 页宽 - 两侧 8px 内距，滑块 x = 右边缘 - 14。
	right := gridTestPageW - 2*terminalScreenPad
	var slider *gfx.Rectangle
	for _, rect := range h.canvas.Rectangles() {
		if rect.Bounds.Width == gutter &&
			rect.Bounds.X >= gridTestOriginX()+right-gutter-1 && rect.Bounds.X <= gridTestOriginX()+right {
			copy := rect
			slider = &copy
		}
	}
	if slider == nil {
		t.Fatalf("14px 槽里没有滑块（x 从 %v 起，槽宽 %v）；图元=%v", right, gutter, h.canvas.Rectangles())
	}
	if slider.Bounds.Height < terminalMinSliderSize {
		t.Fatalf("滑块高度不该低于官方 20px 下限，实际 %v", slider.Bounds.Height)
	}
}

// gridTestOriginX 是网格左边缘（与 grid_test.go 的 gridTestOrigin 对应）。
func gridTestOriginX() float32 {
	x, _ := gridTestOrigin()
	return x
}

// TestTerminalScrollbackSurvivesResize 钉住验收里的第三条：
// **resize 之后滚回不丢**。壳那边（vterm）已经证明历史行还在，这里证明
// 界面侧的滚动位置与窗口请求在 resize 之后仍然指着历史行。
func TestTerminalScrollbackSurvivesResize(t *testing.T) {
	const cols, rows = 12, 4
	h := newScrollHarness(t, cols, rows, 100)
	h.lines(12)
	// 先画一帧：滚轮事件本身不带几何，格子高与命中区是绘制时记下的。
	h.draw()
	// 再滚到顶。
	for i := 0; i < 3; i++ {
		h.scroll(-48, false)
	}
	top := h.draw()
	wantRow(t, top, 0, "row-0")

	// 壳把 PTY 拉宽拉高（官方 xterm.resize 的等价物），历史必须还在。
	h.emu.Resize(20, 6)
	h.cols, h.rows = 20, 6
	state := h.state()
	if state.ScrollbackLen == 0 {
		t.Fatal("resize 之后滚回被清空了")
	}
	after := h.draw()
	if got := after[0]; got != "row-0" {
		t.Fatalf("resize 之后仍该停在历史最老的那一行，实际 %q（整屏 %v）", got, after)
	}
	// 滚到底仍能回到当前屏。
	for i := 0; i < 6; i++ {
		h.scroll(48, false)
	}
	bottom := h.draw()
	live := h.emu.Grid()
	if want := terminalLineText(live, 0); bottom[0] != want {
		t.Fatalf("resize 后回到当前屏第一行该是 %q，实际 %q（整屏 %v）", want, bottom[0], bottom)
	}
}

// terminalLineText 把 Grid 的第 y 行读成去掉尾部空白的文本（与绘制断言同一口径）。
func terminalLineText(g *vterm.Grid, y int) string {
	var b strings.Builder
	for x := 0; x < g.Width; x++ {
		cell, ok := g.Cell(x, y)
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

// TestTerminalScrollbackIgnoresWheelWithoutScrollback 钉住「没有滚回就滚不动」：
// 官方 `scrollback:0` 时 xterm 的 viewport 没有可滚区间，滚轮什么也不做。
func TestTerminalScrollbackIgnoresWheelWithoutScrollback(t *testing.T) {
	h := newScrollHarness(t, 12, 4, 0)
	h.lines(12)
	state := h.state()
	if state.ScrollbackLen != 0 {
		t.Fatalf("scrollback=0 时不该有历史，实际 %d 行", state.ScrollbackLen)
	}
	bottom := h.draw()
	wantRow(t, bottom, 0, "row-8")
	// 仍然要接住手势（官方 xterm 吃掉滚轮），但画面不动。
	result := h.scroll(-48, false)
	if !result.Handled {
		t.Fatalf("落在终端上的滚轮仍该被接住（否则页面会跟着滚），实际 %+v", result)
	}
	if result.Repaint {
		t.Fatal("没有滚回时滚轮不该改变画面")
	}
	again := h.draw()
	wantRow(t, again, 0, "row-8")
}
