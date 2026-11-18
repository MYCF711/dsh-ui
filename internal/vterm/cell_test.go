package vterm

import (
	"strings"
	"testing"
)

// 这一组测试钉住**渲染器能看见的东西**：单元格的前景/背景/属性、字形宽度、
// 光标。Lines/CellText 那条路测的是「文本对不对」，这里测「画出来对不对」。

func write(t *testing.T, e *Emulator, s string) {
	t.Helper()
	if _, err := e.Write([]byte(s)); err != nil {
		t.Fatalf("写入失败：%v", err)
	}
}

func mustCell(t *testing.T, g *Grid, x, y int) Cell {
	t.Helper()
	c, ok := g.Cell(x, y)
	if !ok {
		t.Fatalf("格子 (%d,%d) 越界（屏幕 %dx%d）", x, y, g.Width, g.Height)
	}
	return c
}

func wantColor(t *testing.T, got Color, wantR, wantG, wantB uint8, label string) {
	t.Helper()
	if !got.Set {
		t.Fatalf("%s：颜色未设置", label)
	}
	if got.R != wantR || got.G != wantG || got.B != wantB {
		t.Fatalf("%s：期望 #%02x%02x%02x，实际 #%02x%02x%02x", label, wantR, wantG, wantB, got.R, got.G, got.B)
	}
}

// TestGridExposesTerminalColors 钉住三种颜色写法都落到官方调色板的同一批值上：
// 基本 8 色（SGR 31）、256 色（38;5;n）、真彩（38;2;r;g;b）。
//
// 取值来自官方终端渲染器自带的 `DEFAULT_ANSI_COLORS`
// （dsh-client-ui-sidebar-terminal/lib/client.terminal.js 内联的 xterm core），
// 与 x/ansi 自带的经典 xterm 表**不同色**（红是 #cc0000 而不是 #800000）。
func TestGridExposesTerminalColors(t *testing.T) {
	e := New(20, 2)
	write(t, e, "\x1b[31mR\x1b[38;5;196mX\x1b[38;2;1;2;3mT\x1b[0m")

	g := e.Grid()
	red := mustCell(t, g, 0, 0)
	wantColor(t, red.Style.Fg, 0xcc, 0x00, 0x00, "SGR 31 基本红")

	indexed := mustCell(t, g, 1, 0)
	if indexed.Content != "X" {
		t.Fatalf("第 2 格内容应是 X，实际 %q", indexed.Content)
	}
	// 196 → 16 + 180 → 立方体第 180 格 = (5,0,0) → (255,0,0)。
	wantColor(t, indexed.Style.Fg, 0xff, 0x00, 0x00, "256 色 196")

	truecolor := mustCell(t, g, 2, 0)
	wantColor(t, truecolor.Style.Fg, 0x01, 0x02, 0x03, "真彩 1;2;3")

	// 没有显式颜色时必须是「未设置」，由渲染器用主题色——不是黑底白字。
	if c := mustCell(t, g, 5, 0); c.Style.Fg.Set || c.Style.Bg.Set {
		t.Fatalf("空格的默认色必须交给渲染器：%+v", c.Style)
	}
}

// TestGridBoldDrawsBright 钉住官方 xterm 的默认 `drawBoldTextInBrightColors: true`：
// 粗体 + 0–7 号索引色画成 8–15 号高亮色。官方没有覆盖这个默认值。
func TestGridBoldDrawsBright(t *testing.T) {
	e := New(10, 2)
	write(t, e, "\x1b[1;31mB\x1b[0m\x1b[1;38;5;1mI\x1b[0m")

	g := e.Grid()
	bold := mustCell(t, g, 0, 0)
	if !bold.Style.Attrs.Has(AttrBold) {
		t.Fatal("SGR 1 应当置上粗体位")
	}
	wantColor(t, bold.Style.Fg, 0xef, 0x29, 0x29, "粗体基本红 → brightRed")

	// 索引色的同一条规则（38;5;1 也是 1 号色）。
	wantColor(t, mustCell(t, g, 1, 0).Style.Fg, 0xef, 0x29, 0x29, "粗体 256 色 1 → brightRed")

	// 非粗体不受影响。
	e2 := New(10, 1)
	write(t, e2, "\x1b[31mR")
	wantColor(t, mustCell(t, e2.Grid(), 0, 0).Style.Fg, 0xcc, 0x00, 0x00, "非粗体保持 base red")
}

// TestGridAttributes 钉住任务要求的五个属性位都能透出来。
func TestGridAttributes(t *testing.T) {
	e := New(20, 1)
	write(t, e, "\x1b[1mB\x1b[0m\x1b[3mI\x1b[0m\x1b[4mU\x1b[0m\x1b[7mV\x1b[0m\x1b[9mS\x1b[0m")

	g := e.Grid()
	for _, want := range []struct {
		x int
		a Attr
		n string
	}{
		{0, AttrBold, "粗体"},
		{1, AttrItalic, "斜体"},
		{2, AttrUnderline, "下划线"},
		{3, AttrReverse, "反显"},
		{4, AttrStrike, "删除线"},
	} {
		if got := mustCell(t, g, want.x, 0).Style.Attrs; !got.Has(want.a) {
			t.Fatalf("%s属性没透出来：attrs=%#x", want.n, uint8(got))
		}
	}
}

// TestGridWideCells 钉住等宽字形宽度：全角字占 2 格，后面那一格是**占位尾巴**
// （Width==0），渲染器必须跳过它。上游 `Line.Set` 就是这么标记的。
func TestGridWideCells(t *testing.T) {
	e := New(10, 2)
	write(t, e, "中a")

	g := e.Grid()
	if c := mustCell(t, g, 0, 0); c.Content != "中" || c.Width != 2 {
		t.Fatalf("全角字必须 Content=中 Width=2，实际 %+v", c)
	}
	if c := mustCell(t, g, 1, 0); c.Width != 0 || c.Content != "" {
		t.Fatalf("全角字的第二格必须是占位（Width=0），实际 %+v", c)
	}
	if c := mustCell(t, g, 2, 0); c.Content != "a" || c.Width != 1 {
		t.Fatalf("全角之后的半角字应从第 3 格开始（列号含双宽），实际 %+v", c)
	}
}

// TestGridCursorState 钉住光标的位置、可见性、形状与闪烁（DECSCUSR 三档）。
func TestGridCursorState(t *testing.T) {
	e := New(10, 4)
	write(t, e, "abc\x1b[2;3H")
	g := e.Grid()
	if g.Cursor.X != 2 || g.Cursor.Y != 1 {
		t.Fatalf("光标应在 (2,1)，实际 (%d,%d)", g.Cursor.X, g.Cursor.Y)
	}
	if !g.Cursor.Visible {
		t.Fatal("默认光标应当可见")
	}

	for _, c := range []struct {
		seq   string
		shape CursorShape
		blink bool
	}{
		{"\x1b[2 q", CursorBlock, false},     // 2 = steady block
		{"\x1b[1 q", CursorBlock, true},      // 1 = blinking block
		{"\x1b[4 q", CursorUnderline, false}, // 4 = steady underline
		{"\x1b[3 q", CursorUnderline, true},  // 3 = blinking underline
		{"\x1b[6 q", CursorBar, false},       // 6 = steady bar
		{"\x1b[5 q", CursorBar, true},        // 5 = blinking bar
	} {
		write(t, e, c.seq)
		got := e.Grid().Cursor
		if got.Shape != c.shape || got.Blink != c.blink {
			t.Fatalf("%q：期望 %v blink=%v，实际 %v blink=%v", c.seq, c.shape, c.blink, got.Shape, got.Blink)
		}
	}

	write(t, e, "\x1b[?25l")
	if e.Grid().Cursor.Visible {
		t.Fatal("DECTCEM 隐藏之后光标不应可见")
	}
	write(t, e, "\x1b[?25h")
	if !e.Grid().Cursor.Visible {
		t.Fatal("DECTCEM 恢复之后光标应可见")
	}
}

// TestGridRedrawKeepsOnlyFinalFrame 用**网格**再钉一遍「\\r 重画只留最后帧」：
// 上一版是按 Lines 断言，这一版看每格内容，确保中间帧没有残留在格子边缘。
func TestGridRedrawKeepsOnlyFinalFrame(t *testing.T) {
	e := New(20, 2)
	write(t, e, "progress  0%\rprogress 50%\rprogress 100%\n")

	g := e.Grid()
	var row string
	for x := 0; x < g.Width; x++ {
		row += mustCell(t, g, x, 0).Content
	}
	if row = strings.TrimRight(row, " "); row != "progress 100%" {
		t.Fatalf("第 1 行应只剩最后帧 %q，实际 %q", "progress 100%", row)
	}
}

// TestScrollbackKeepsScrolledOffLines 钉住滚回缓冲的**内容与投影**：滚出屏幕顶端
// 的行必须能原样取回来，颜色/属性跟它当时在屏幕上时一致（不是重新解析文本），
// 并且当前屏幕不受影响。
//
// 上游依据见 cell.go 的 Scrollback 注释：正常换行滚屏就会把顶行推进 scrollback
// （`x/vt@v0.1.0/screen.go:359-366`），所以这里不是造一段历史，而是取上游的。
func TestScrollbackKeepsScrolledOffLines(t *testing.T) {
	e := New(10, 3)
	write(t, e, "\x1b[31mred\x1b[0m\r\n") // 第 1 行：索引红
	write(t, e, "\x1b[1mtwo\x1b[0m\r\n")  // 第 2 行：粗体
	write(t, e, "three\r\n")
	write(t, e, "four\r\n")
	write(t, e, "five\r\n")

	// 5 行写进 3 行屏：最后那次换行又滚掉一行，所以是 3 行历史。
	if got := e.ScrollbackLen(); got != 3 {
		t.Fatalf("滚回应有 3 行，实际 %d", got)
	}
	sb := e.Scrollback()
	if sb.Width != 10 || sb.Height != 3 {
		t.Fatalf("滚回投影应为 10×3，实际 %d×%d", sb.Width, sb.Height)
	}
	rows := []string{"red", "two", "three"}
	for y, want := range rows {
		var row string
		for x := 0; x < sb.Width; x++ {
			row += mustCell(t, sb, x, y).Content
		}
		if row = strings.TrimRight(row, " "); row != want {
			t.Fatalf("滚回第 %d 行应为 %q，实际 %q", y, want, row)
		}
	}
	// 颜色与属性跟着一起回来：SGR 31 是官方调色板的 #cc0000，SGR 1 是粗体。
	wantColor(t, mustCell(t, sb, 0, 0).Style.Fg, 0xcc, 0x00, 0x00, "滚回里的红字")
	if attrs := mustCell(t, sb, 0, 1).Style.Attrs; !attrs.Has(AttrBold) {
		t.Fatalf("滚回里的粗体应当保留，实际 attrs=%#x", uint16(attrs))
	}
	// 行比屏幕窄：补出来的格子是零值（Width==0），与 Grid 的约定一致。
	if c := mustCell(t, sb, 5, 0); c.Width != 0 || c.Content != "" {
		t.Fatalf("滚回行尾的填充格应是零值 Cell，实际 %+v", c)
	}
	// 滚回不是屏幕：没有光标。
	if sb.Cursor.Visible {
		t.Fatalf("滚回快照不该有可见光标，实际 %+v", sb.Cursor)
	}
	// 当前屏幕只剩后两行 + 空行。
	if got := e.Lines(); len(got) < 2 || got[0] != "four" || got[1] != "five" {
		t.Fatalf("当前屏幕应只剩 four/five，实际 %q", got)
	}
}

// TestScrollbackEmptyWithoutScroll 没有滚出任何行时，滚回是**空且不报错**：
// 渲染器不必先判断有没有历史。
func TestScrollbackEmptyWithoutScroll(t *testing.T) {
	e := New(8, 4)
	write(t, e, "one\r\ntwo\r\n")

	if got := e.ScrollbackLen(); got != 0 {
		t.Fatalf("没有滚出行为时应为 0 行，实际 %d", got)
	}
	sb := e.Scrollback()
	if sb.Width != 8 || sb.Height != 0 || len(sb.Cells) != 0 {
		t.Fatalf("空滚回应是 8×0，实际 %d×%d（cells=%d）", sb.Width, sb.Height, len(sb.Cells))
	}
	if _, ok := sb.Cell(0, 0); ok {
		t.Fatal("空滚回的 (0,0) 必须越界")
	}
}
