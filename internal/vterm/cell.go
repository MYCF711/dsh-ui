package vterm

import (
	"fmt"
	"image/color"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

// 这一文件把「屏幕上的每个单元格长什么样」暴露出去，仍然**不让上游类型外泄**：
// 对外只有本包自己的 Attr / Color / Style / Cell / Grid，`uv.Cell`、`uv.Style`、
// `ansi.BasicColor` 都只出现在本文件里。上游把字段名或类型改掉时，只改这里。

// Attr 是单元格的文字属性位，与上游 `uv.Attrs` 一位一对应。
//
// 之所以自己再定义一份而不是 `type Attr = uint8`：位序是**契约**，
// 万一上游挪了位，编译不会红，只会画错——独立常量让映射走 attrsOf 一处。
type Attr uint16

const (
	AttrBold Attr = 1 << iota
	AttrFaint
	AttrItalic
	AttrUnderline
	AttrBlink
	AttrRapidBlink
	AttrReverse
	AttrConceal
	AttrStrike
)

// Has 报告是否带某个属性位（Attr(0) 恒 false）。
func (a Attr) Has(want Attr) bool { return a&want != 0 }

// Color 是一个**已解析**的颜色：8 位通道 + 是否显式设置。
//
// 索引色（SGR 30–37 / 90–97 / 38;5;n）在模拟器里就落到 RGB 上：调色板本来就是
// 模拟器的状态——xterm 的 OSC 4 能改它，渲染器再持有一份就会与屏幕对不上。
// Set=false 表示这一格没有显式颜色，用调用方的默认色（官方是 label-primary /
// bg-base，见 terminal-theme.ts:60）。
type Color struct {
	R, G, B, A uint8
	Set        bool
}

// RGB 造一个不透明的显式颜色。
func RGB(r, g, b uint8) Color { return Color{R: r, G: g, B: b, A: 0xff, Set: true} }

// Style 是单元格的前景/背景/属性。
//
// 反显（AttrReverse）**不在这里换色**：换色要用调用方的默认前景背景，
// 只有渲染器两边都知道，所以只报属性，由它换。
type Style struct {
	Fg, Bg Color
	Attrs  Attr
}

// Cell 是一个单元格的可绘制投影。
//
// Width 的取值来自上游：1 = 半角、2 = 全角（CJK 等 East Asian Width F/W）。
// **Width==0 表示它是前一个宽字形的占位尾巴**（上游 `Line.Set` 用零值 Cell 标记），
// 渲染器必须跳过它，否则双宽字符后面会多出一格空白。
type Cell struct {
	Content string
	Width   int
	Style   Style
}

// CursorShape 是光标形状，对应 DECSCUSR（CSI Ps SP q）的三档。
type CursorShape uint8

const (
	CursorBlock CursorShape = iota
	CursorBar
	CursorUnderline
)

// String 便于测试与日志读。
func (s CursorShape) String() string {
	switch s {
	case CursorBlock:
		return "block"
	case CursorBar:
		return "bar"
	case CursorUnderline:
		return "underline"
	default:
		return fmt.Sprintf("CursorShape(%d)", uint8(s))
	}
}

// Cursor 是光标状态：位置、可见性、形状、是否闪烁。
//
// Blink 只表达**程序允许它闪**（DECSCUSR 的 1/3/5 档）；实际有没有闪是界面的事
// （官方 `terminalCursorBlink` 还要 `.xterm-focus`，terminal.module.css:18-19）。
type Cursor struct {
	X, Y    int
	Visible bool
	Shape   CursorShape
	Blink   bool
}

// Grid 是屏幕在某一刻的**不可变快照**，供渲染器逐格绘制。
//
// 为什么给快照而不是让渲染器直接调 Cell(x,y)：渲染线程与读取循环不是一条
// goroutine，逐格回调会跨锁几百次（一次 resize 之后 80×24 就是 1920 次），
// 快照一次拿完，之后随便画。
type Grid struct {
	Width, Height int
	Cells         []Cell
	Cursor        Cursor
}

// Cell 返回 (x,y) 这一格；越界或未分配的格子返回 false。
func (g *Grid) Cell(x, y int) (Cell, bool) {
	if g == nil || x < 0 || y < 0 || x >= g.Width || y >= g.Height {
		return Cell{}, false
	}
	i := y*g.Width + x
	if i >= len(g.Cells) {
		return Cell{}, false
	}
	return g.Cells[i], true
}

// Grid 返回当前屏幕的单元格快照（含光标）。
//
// 与 Lines/String 的区别：那条路给的是**读文本**用的投影，这条给的是**画**用的
// ——每格的字形宽度、前景背景、属性与光标都在里面。
func (e *Emulator) Grid() *Grid {
	b := e.inner.Bounds()
	w, h := b.Dx(), b.Dy()
	grid := &Grid{Width: w, Height: h}
	if w <= 0 || h <= 0 {
		return grid
	}
	grid.Cells = make([]Cell, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := e.inner.CellAt(x, y)
			if c == nil {
				continue
			}
			grid.Cells[y*w+x] = Cell{
				Content: c.Content,
				Width:   c.Width,
				Style:   styleOf(&c.Style),
			}
		}
	}
	pos := e.inner.CursorPosition()
	shape, blink := e.inner.CursorStyle()
	grid.Cursor = Cursor{
		X: pos.X, Y: pos.Y,
		Visible: !e.inner.CursorHidden(),
		Shape:   cursorShape(shape),
		Blink:   blink,
	}
	return grid
}

// styleOf 把上游样式投影成本包的类型。
func styleOf(s *uv.Style) Style {
	fg, fgIndex := resolve(s.Fg)
	bg, _ := resolve(s.Bg)
	attrs := attrsOf(s.Attrs)
	if s.Underline != uv.UnderlineNone {
		attrs |= AttrUnderline
	}
	// 官方的 xterm 默认 `drawBoldTextInBrightColors: true`（打进官方终端包的
	// xterm core 的 DEFAULT_OPTIONS；官方没有覆盖它），所以粗体的 0–7 号索引色
	// 画成 8–15 那一档的高亮色。这是**官方默认行为**，不是我们选的近似。
	if attrs.Has(AttrBold) && fgIndex >= 0 && fgIndex < 8 {
		fg = palette[fgIndex+8]
	}
	return Style{Fg: fg, Bg: bg, Attrs: attrs}
}

// attrsOf 把上游属性位映射成我们自己的位（逐位显式，不假设两边同序）。
func attrsOf(a uint8) Attr {
	var out Attr
	for _, m := range []struct {
		by   Attr
		from uint8
	}{
		{AttrBold, uv.AttrBold},
		{AttrFaint, uv.AttrFaint},
		{AttrItalic, uv.AttrItalic},
		{AttrBlink, uv.AttrBlink},
		{AttrRapidBlink, uv.AttrRapidBlink},
		{AttrReverse, uv.AttrReverse},
		{AttrConceal, uv.AttrConceal},
		{AttrStrike, uv.AttrStrikethrough},
	} {
		if a&m.from != 0 {
			out |= m.by
		}
	}
	return out
}

// cursorShape 把上游的 DECSCUSR 三档映射成本包的形状。
func cursorShape(style vt.CursorStyle) CursorShape {
	switch style {
	case vt.CursorBar:
		return CursorBar
	case vt.CursorUnderline:
		return CursorUnderline
	default:
		return CursorBlock
	}
}

// resolve 把上游颜色解析成 8 位通道，并返回它在 256 色调色板里的下标
// （-1 = 真彩、未知类型或未设置）。
func resolve(c color.Color) (Color, int) {
	switch v := c.(type) {
	case nil:
		return Color{}, -1
	case ansi.BasicColor:
		index := int(v)
		if index < 0 || index >= 16 {
			return Color{}, -1
		}
		return palette[index], index
	case ansi.IndexedColor:
		return palette[int(v)], int(v)
	case ansi.RGBColor:
		return RGB(v.R, v.G, v.B), -1
	}
	r, g, b, a := c.RGBA()
	return Color{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8), Set: true}, -1
}

// palette 是 256 色调色板。
//
// 取值**照抄官方终端渲染器自带的那张表**：`@xterm/xterm` 的 DEFAULT_ANSI_COLORS，
// 打进官方包 `dsh-client-ui-sidebar-terminal/lib/client.terminal.js`（rc.2 asar）。
// 官方这段就是：
//
//	DEFAULT_ANSI_COLORS = [#2e3436 #cc0000 #4e9a06 #c4a000 #3465a4 #75507b
//	                       #06989a #d3d7cf #555753 #ef2929 #8ae234 #fce94f
//	                       #729fcf #ad7fa8 #34e2e2 #eeeeec]
//	levels = [0, 95, 135, 175, 215, 255]        // 16–231 的 6×6×6 立方
//	gray_i = 8 + 10*i, i = 0..23                 // 232–255 灰阶
//
// 注意这**不是** x/ansi 自带的那张经典 xterm 表：`ansi.BasicColor(1).RGBA()` 是
// xterm 经典红，官方界面实际画的是 #cc0000。两边差一个色号，用错就会与官方不同色。
var palette = buildPalette()

func buildPalette() [256]Color {
	var p [256]Color
	base := [16]Color{
		RGB(0x2e, 0x34, 0x36), // 0 black
		RGB(0xcc, 0x00, 0x00), // 1 red
		RGB(0x4e, 0x9a, 0x06), // 2 green
		RGB(0xc4, 0xa0, 0x00), // 3 yellow
		RGB(0x34, 0x65, 0xa4), // 4 blue
		RGB(0x75, 0x50, 0x7b), // 5 magenta
		RGB(0x06, 0x98, 0x9a), // 6 cyan
		RGB(0xd3, 0xd7, 0xcf), // 7 white
		RGB(0x55, 0x57, 0x53), // 8 brightBlack
		RGB(0xef, 0x29, 0x29), // 9 brightRed
		RGB(0x8a, 0xe2, 0x34), // 10 brightGreen
		RGB(0xfc, 0xe9, 0x4f), // 11 brightYellow
		RGB(0x72, 0x9f, 0xcf), // 12 brightBlue
		RGB(0xad, 0x7f, 0xa8), // 13 brightMagenta
		RGB(0x34, 0xe2, 0xe2), // 14 brightCyan
		RGB(0xee, 0xee, 0xec), // 15 brightWhite
	}
	copy(p[:16], base[:])
	levels := [6]uint8{0, 95, 135, 175, 215, 255}
	for i := 0; i < 216; i++ {
		p[16+i] = RGB(levels[(i/36)%6], levels[(i/6)%6], levels[i%6])
	}
	for i := 0; i < 24; i++ {
		gray := uint8(8 + 10*i)
		p[232+i] = RGB(gray, gray, gray)
	}
	return p
}
