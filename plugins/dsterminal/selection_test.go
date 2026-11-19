package dsterminal

import (
	"runtime"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/vterm"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 选区与复制的测试：真模拟器造屏幕 → 真无头画布画一遍 → 用手势驱动指针事件 →
// 断言选区状态、选中态图元与剪贴板收到的字节。
//
// 剪贴板是注入的假函数（`p.clipboard`）：单测不许往真剪贴板里写字，
// 真实现是 `app.SetClipboardText`（见 terminal.go 的 Register）。
//
// 颜色断言里写的是字面量 0.3（xterm 给不透明 selectionBackground 的强制透明度，
// 见 selection.go 的 terminalSelectionAlpha）——引用常量的断言会跟着常量一起漂。

type selectionFixture struct {
	t       *testing.T
	emu     *vterm.Emulator
	plugin  *Plugin
	env     *pluginapi.Env
	canvas  *gfx.Canvas
	page    *ui.Page
	rect    gfx.Rect
	hit     gfx.ID
	clipped []string
	// wrote 是插件写给 PTY 的字节（鼠标上报那条路，见 mouse_test.go）。
	wrote []terminalWrite
}

func newSelectionFixture(t *testing.T, w, h int, writes ...string) *selectionFixture {
	t.Helper()
	emu := vterm.New(w, h)
	for _, s := range writes {
		if _, err := emu.Write([]byte(s)); err != nil {
			t.Fatalf("写入模拟器失败：%v", err)
		}
	}
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	canvas.SetCellMeasurer(func(size float32, face gfx.MonoFace) (float32, float32) {
		return gridTestCellW, gridTestCellH
	})
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	feat := tree.Feature("terminal-selection")
	fixture := &selectionFixture{t: t, emu: emu, canvas: canvas, page: page,
		hit: tree.ID(feat, "terminal-screen")}
	state := func() pluginapi.TerminalViewState {
		grid := emu.Grid()
		return pluginapi.TerminalViewState{
			Phase: "connected", Writable: true,
			Grid:          grid,
			ScrollbackLen: emu.ScrollbackLen(),
			Mouse:         emu.MouseState(),
			Info:          &apiclient.TerminalInfo{Title: "终端 1", State: "running", Cols: grid.Width, Rows: grid.Height},
		}
	}
	plugin := &Plugin{theme: theme}
	plugin.clipboard = func(value string) error {
		fixture.clipped = append(fixture.clipped, value)
		return nil
	}
	fixture.plugin = plugin
	fixture.env = &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: feat, Canvas: canvas,
		Data:               pluginapi.Snapshot{ShowRightbar: true},
		ActiveRightTabKind: RightTabTerminal,
		Act: pluginapi.Actions{
			TerminalEnsure:           func(string) {},
			TerminalSnapshot:         func(string) pluginapi.TerminalViewState { return state() },
			TerminalScrollbackWindow: func(_ string, first, lines int) *vterm.Grid { return emu.Window(first, lines) },
			TerminalRetry:            func(string) {},
			TerminalWriteOfficial: func(tabID, data string, typed bool) {
				fixture.wrote = append(fixture.wrote, terminalWrite{tabID: tabID, data: data, typed: typed})
			},
			OpenRightTab:  func(string) {},
			CloseRightTab: func(string) {},
			Notify:        func(string) {},
		},
	}
	return fixture
}

// paint 走一次普通绘制：声明命中区、记下几何、按当前选区画。
func (f *selectionFixture) paint() {
	f.rect = gfx.Rect{Width: gridTestPageW, Height: gridTestPageH}
	f.canvas.Reset()
	f.page.Draw(f.canvas, f.rect, func(c ui.C) {
		f.plugin.drawPanelBody(c, f.env)
	})
}

// pointer 把一次指针事件送进绘制（与运行时同一条路：绘制里用 PointerOn 取事件）。
//
// 返回控件有没有 Consume 掉这次事件。
func (f *selectionFixture) pointer(event gfx.PointerEvent) bool {
	return f.canvas.DeliverPointer(event, gfx.Size{Width: gridTestPageW, Height: gridTestPageH}, func(c *gfx.Canvas, _ gfx.Size) {
		f.page.Draw(c, gfx.Rect{Width: gridTestPageW, Height: gridTestPageH}, func(ui ui.C) {
			f.plugin.drawPanelBody(ui, f.env)
		})
	})
}

// cellCenter 返回第 (col,row) 格中心的窗口坐标（row 是**视口行**）。
func (f *selectionFixture) cellCenter(col, row int) (x, y float32) {
	originX, originY := gridTestOrigin()
	return originX + (float32(col)+0.5)*gridTestCellW, originY + (float32(row)+0.5)*gridTestCellH
}

// press/move/release 是一次完整手势的三段。
func (f *selectionFixture) press(col, row int, button gfx.PointerButton, modifiers gfx.Modifiers, clicks int) {
	x, y := f.cellCenter(col, row)
	f.pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: button, Modifiers: modifiers, X: x, Y: y, ClickCount: clicks, Target: f.hit})
}

func (f *selectionFixture) move(col, row int) {
	x, y := f.cellCenter(col, row)
	f.pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Button: gfx.PointerButtonPrimary, X: x, Y: y, Target: f.hit})
}

func (f *selectionFixture) release(col, row int) {
	x, y := f.cellCenter(col, row)
	f.pointer(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, X: x, Y: y, Target: f.hit})
}

// sep 是官方选区文本的行分隔符（`isWindows ? CRLF : LF`，client.terminal.js byte 117677）。
var sep = func() string {
	if runtime.GOOS == "windows" {
		return "\r\n"
	}
	return "\n"
}()

// selectionText 是当前选区在界面上该给出的文本。
func (f *selectionFixture) selectionText() string {
	state := f.plugin.selectionFor("terminal")
	return f.plugin.terminalSelectionText(f.env, "terminal", f.emuGrid().Width, state.sel)
}

func (f *selectionFixture) emuGrid() *vterm.Grid { return f.emu.Grid() }

// TestTerminalDragSelectsAndRightClickCopies 钉住主链路：拖动选出「bc / de」，
// 右键把这段文本写进剪贴板（官方是右键→原生编辑菜单→复制，见 selection.go 头）。
func TestTerminalDragSelectsAndRightClickCopies(t *testing.T) {
	f := newSelectionFixture(t, 10, 4, "abc\r\ndef\r\nghi\r\n")
	f.paint()
	f.press(1, 0, gfx.PointerButtonPrimary, 0, 1)
	f.move(2, 1)
	f.release(2, 1)
	// 终点列是**开区间**（xterm 的 selectionEnd 不含本体），所以到列 2 为止 = "de"。
	want := "bc" + sep + "de"
	if got := f.selectionText(); got != want {
		t.Fatalf("拖动该选出 %q，实际 %q", want, got)
	}
	f.press(2, 1, gfx.PointerButtonSecondary, 0, 1)
	if len(f.clipped) != 1 {
		t.Fatalf("右键该写一次剪贴板，实际 %d 次：%v", len(f.clipped), f.clipped)
	}
	fromClipboard := f.selectionText()
	if f.clipped[0] != fromClipboard {
		t.Fatalf("剪贴板该收到 %q，实际 %q", fromClipboard, f.clipped[0])
	}
}

// TestTerminalDoubleClickSelectsWord 钉住双击选词：官方 `_handleDoubleClick`
// → `_selectWordAtCursor`，词分隔符是 xterm 的默认值。
func TestTerminalDoubleClickSelectsWord(t *testing.T) {
	f := newSelectionFixture(t, 20, 3, "foo bar baz\r\n")
	f.paint()
	f.press(5, 0, gfx.PointerButtonPrimary, 0, 2) // ClickCount=2 = 双击
	f.release(5, 0)
	if got := f.selectionText(); got != "bar" {
		t.Fatalf("双击该选中 %q，实际 %q", "bar", got)
	}
	f.press(5, 0, gfx.PointerButtonSecondary, 0, 1)
	if len(f.clipped) != 1 || f.clipped[0] != "bar" {
		t.Fatalf("右键复制该收到 %q，实际 %v", "bar", f.clipped)
	}
}

// TestTerminalTripleClickSelectsLine 钉住三击整行：官方 `_selectLineAt`。
func TestTerminalTripleClickSelectsLine(t *testing.T) {
	f := newSelectionFixture(t, 16, 4, "one\r\ntwo\r\n")
	f.paint()
	f.press(1, 1, gfx.PointerButtonPrimary, 0, 3) // ClickCount=3 = 三击
	f.release(1, 1)
	if got := f.selectionText(); got != "two" {
		t.Fatalf("三击该选中整行 %q，实际 %q", "two", got)
	}
}

// TestTerminalAltDragSelectsColumn 钉住 Alt 拖动 = 矩形选区
// （官方 `shouldColumnSelect` 在 Windows 上就是 Alt）。
func TestTerminalAltDragSelectsColumn(t *testing.T) {
	f := newSelectionFixture(t, 12, 4, "abcdef\r\nghijkl\r\nmnopqr\r\n")
	f.paint()
	f.press(1, 0, gfx.PointerButtonPrimary, gfx.ModifierAlt, 1)
	f.move(4, 2)
	f.release(4, 2)
	want := "bcd" + "\r\n" + "hij" + "\r\n" + "nop"
	if got := f.selectionText(); got != want && got != "bcd\nhij\nnop" {
		t.Fatalf("矩形选区该是 %q，实际 %q", want, got)
	}
}

// TestTerminalSelectionPaintMatchesOfficialColors 钉住选中态的两支颜色：
// 底色 = blend(bg-base, label-primary, 0.3)（xterm 的 selectionBackgroundOpaque），
// 字色 = bg-base（官方 selectionForeground: background，terminal-theme.ts:61）。
func TestTerminalSelectionPaintMatchesOfficialColors(t *testing.T) {
	f := newSelectionFixture(t, 10, 3, "abcdef\r\n")
	f.paint()
	// 没选中时这一行没有选中底色。
	if before := f.selectionFills(); len(before) != 0 {
		t.Fatalf("没选东西不该有选中底色，实际 %d 块", len(before))
	}
	f.press(1, 0, gfx.PointerButtonPrimary, 0, 1)
	f.move(3, 0)
	f.release(3, 0)
	// 拖动结束之后的那一帧：选区还在（官方松手不会清空）。
	f.paint()
	fills := f.selectionFills()
	if len(fills) != 1 {
		t.Fatalf("列 1..3 该是一整块选中底色，实际 %d 块：%+v", len(fills), fills)
	}
	want := selectionWantFill(f.plugin.theme)
	if fills[0].color != want {
		t.Fatalf("选中底色该是 %v（blend(bg-base,label-primary,0.3)），实际 %v", want, fills[0].color)
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
	// 选中格的字色换成 bg-base。
	for _, run := range gridRuns(f.canvas, 0) {
		column := run.column
		selected := column >= 1 && column < 3
		if selected && run.color != f.plugin.theme.BgBase {
			t.Fatalf("选中的列 %d 字色该是 bg-base %v，实际 %v", column, f.plugin.theme.BgBase, run.color)
		}
		if !selected && run.color == f.plugin.theme.BgBase {
			t.Fatalf("没选中的列 %d 不该换成 bg-base", column)
		}
	}
}

// selectionFill 是一块「可能是选中底色」的矩形。
type selectionFill struct {
	rect  gfx.Rect
	color gfx.Color
}

// selectionFills 取画布上颜色正好等于选中底色的填充块。
func (f *selectionFixture) selectionFills() []selectionFill {
	want := selectionWantFill(f.plugin.theme)
	out := []selectionFill{}
	for _, fill := range f.canvas.Rectangles() {
		if fill.Color == want {
			out = append(out, selectionFill{rect: fill.Bounds, color: fill.Color})
		}
	}
	return out
}

// selectionWantFill 用官方口径算一遍期望的选中底色，断言里的 0.3 是字面量
// （xterm 给不透明 selectionBackground 的强制透明度）。
func selectionWantFill(theme *pluginapi.Theme) gfx.Color {
	mix := func(b, fg float32) float32 { return b + ((fg - b) * 0.3) }
	return gfx.Color{
		Red:   mix(theme.BgBase.Red, theme.LabelPrimary.Red),
		Green: mix(theme.BgBase.Green, theme.LabelPrimary.Green),
		Blue:  mix(theme.BgBase.Blue, theme.LabelPrimary.Blue),
		Alpha: 1,
	}
}

// TestTerminalSelectionSnapsOffWideCharTail 钉住 CJK 双宽：从宽字的尾巴格起拖，
// 起点被挪到下一格（官方 `_handleSingleClick` 的 `hasWidth(col) === 0 → col++`）。
func TestTerminalSelectionSnapsOffWideCharTail(t *testing.T) {
	f := newSelectionFixture(t, 12, 3, "中文abc\r\n")
	f.paint()
	f.press(1, 0, gfx.PointerButtonPrimary, 0, 1) // 列 1 = 「中」的尾巴
	f.move(4, 0)
	f.release(4, 0)
	if got, want := f.selectionText(), "文"; got != want && got != "文a" {
		t.Fatalf("从尾巴格起重拖该从「文」起，实际 %q", got)
	}
	state := f.plugin.selectionFor("terminal")
	if state.sel.Start.X != 2 {
		t.Fatalf("起点该被挪到列 2，实际列 %d", state.sel.Start.X)
	}
}

// TestTerminalInputClearsSelection 钉住「用户一输入就清空选区」：
// 官方 `_coreService.onUserInput(() => hasSelection && clearSelection())`（byte 116835）。
func TestTerminalInputClearsSelection(t *testing.T) {
	f := newSelectionFixture(t, 10, 3, "abc\r\n")
	f.paint()
	f.press(0, 0, gfx.PointerButtonPrimary, 0, 1)
	f.move(2, 0)
	f.release(2, 0)
	if f.selectionText() == "" {
		t.Fatal("拖动之后选区该是非空的")
	}
	f.plugin.field.SetText("ls")
	if !f.plugin.HandleKey(gfx.KeyEvent{Key: gfx.KeyF6}) {
		t.Fatal("F6 没被终端接住")
	}
	if f.plugin.selectionFor("terminal").sel.Empty() == false {
		t.Fatal("用户输入之后选区该被清空（官方 onUserInput → clearSelection）")
	}
}

// TestTerminalDragScrollAmountFollowsXterm 钉住贴边滚动的换算
// （官方 `_getMouseEventScrollAmount`，byte 120966）：
//
//	画布内 → 0；出去先夹到 ±50px，然后 sign + round(14 * y/50)
func TestTerminalDragScrollAmountFollowsXterm(t *testing.T) {
	const top, height float32 = 100, 160
	cases := []struct {
		name string
		y    float32
		want int
	}{
		{name: "画布内不滚", y: 180, want: 0},
		{name: "上边界内不滚", y: 100, want: 0},
		{name: "下边界内不滚", y: 260, want: 0},
		{name: "下边外 16px：1+round(14*0.32)=5", y: 276, want: 5},
		{name: "下边外 50px 到底：1+14=15", y: 310, want: 15},
		{name: "下边外超过 50px 仍按 15", y: 400, want: 15},
		{name: "上边外 16px：-1-round(4.48)=-5", y: 84, want: -5},
		{name: "上边外到底：-15", y: 50, want: -15},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := terminalDragScrollAmount(tc.y, top, height); got != tc.want {
				t.Fatalf("y=%v 该滚 %d 行，实际 %d", tc.y, tc.want, got)
			}
		})
	}
}

// TestTerminalDragScrollsScrollback 钉住「拖出上边界时把画面往上滚」：
// 滚完之后选区终点贴住新的视口首行（官方 `_dragScroll`，byte 124320）。
func TestTerminalDragScrollsScrollback(t *testing.T) {
	f := newSelectionFixture(t, 10, 2)
	if _, err := f.emu.Write([]byte("one\r\ntwo\r\nthree\r\nfour\r\nfive\r\n")); err != nil {
		t.Fatalf("写入失败：%v", err)
	}
	if f.emu.ScrollbackLen() <= 0 {
		t.Fatal("这个夹具该有滚回行")
	}
	f.paint()
	// 贴底时 ydisp 就是滚回行数（官方 xterm 的 ybase）：先记下来。
	before := f.plugin.scrollViewFor("terminal").ydisp
	if before <= 0 {
		t.Fatalf("贴底时 ydisp 该等于滚回行数，实际 %d", before)
	}
	f.press(0, 1, gfx.PointerButtonPrimary, 0, 1)
	// 指针拖到画布上方 20px（上边外）：应触发往上滚。
	originX, originY := gridTestOrigin()
	f.pointer(gfx.PointerEvent{
		Kind: gfx.PointerMove, Button: gfx.PointerButtonPrimary,
		X: originX + 1, Y: originY - 20, Target: f.hit,
	})
	state := f.plugin.selectionFor("terminal")
	if state.dragScroll >= 0 {
		t.Fatalf("拖出上边界该算出负数滚动量，实际 %d", state.dragScroll)
	}
	after := f.plugin.scrollViewFor("terminal").ydisp
	if after >= before {
		t.Fatalf("贴边拖动该把视图往上滚（ydisp 变小），实际 before=%d after=%d", before, after)
	}
	if !state.sel.HasEnd {
		t.Fatal("滚动之后终点该贴住新的视口边界")
	}
}

// TestTerminalCopyWithoutSelectionDoesNothing 钉住空选区不碰剪贴板。
func TestTerminalCopyWithoutSelectionDoesNothing(t *testing.T) {
	f := newSelectionFixture(t, 10, 3, "abc\r\n")
	f.paint()
	f.press(2, 0, gfx.PointerButtonSecondary, 0, 1) // 只是点一下右键
	if len(f.clipped) != 0 {
		t.Fatalf("没有选区时右键不该写剪贴板，实际 %v", f.clipped)
	}
	if f.plugin.selectionFor("terminal").sel.Empty() != true {
		t.Fatal("没有选区时状态该保持空")
	}
}
