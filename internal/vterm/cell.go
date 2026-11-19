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

// ScrollbackLen 报告滚回缓冲里有多少行（屏幕上方已经滚出去的历史行）。
//
// 上游**正常换行滚屏**就会把丢掉的顶行推进 scrollback：`x/vt@v0.1.0/cc.go:29`
// 那句 `// XXX: Handle scrollback whenever we add it.` 是**过期注释**，真正落盘在
// `Screen.ScrollUp` → `Screen.DeleteLine`（`screen.go:297-303` 与 `screen.go:359-366`
// 的 `scrollback.PushN`）。探针（2026-11-19，20 行写进 5 行屏）读到 16 行——
// 与 xterm 的语义一致：滚回不含当前屏。
//
// 成本：一次读锁，可以每帧调。
func (e *Emulator) ScrollbackLen() int {
	if e.scrollbackOff {
		return 0
	}
	return e.inner.ScrollbackLen()
}

// Scrollback 返回滚回缓冲的快照，投影与 Grid **完全相同**（同一套 Cell / Style，
// 颜色、反显、下划线、全角宽度都按同一份代码算出来），渲染器可以直接拿它当
// 「屏幕上方那几行」画。
//
//   - 行 0 是最老的一行，行 ScrollbackLen()-1 是紧挨屏幕顶端的一行；
//   - Width 取**当前屏幕宽度**：上游按行存 cells，短行由调用方按 Width 补齐，
//     补出来的格子是零值 Cell（Width==0），与 Grid 对未写格子的约定一致；
//   - Cursor 恒不可见：滚回不是屏幕，那里没有光标。
//
// **不要每帧调**：这是 O(行数×列数)，而且上游每取一格要拿一次读锁
// （1000 行 × 90 列 = 9 万次）。每帧只该问 ScrollbackLen()，用户真的往回滚时
// 才取整块。
func (e *Emulator) Scrollback() *Grid {
	w, _ := e.Size()
	n := e.ScrollbackLen()
	grid := &Grid{Width: w, Height: n}
	if w <= 0 || n <= 0 {
		return grid
	}
	grid.Cells = make([]Cell, w*n)
	for y := 0; y < n; y++ {
		for x := 0; x < w; x++ {
			c := e.inner.ScrollbackCellAt(x, y)
			if c == nil {
				continue
			}
			grid.Cells[y*w+x] = Cell{Content: c.Content, Width: c.Width, Style: styleOf(&c.Style)}
		}
	}
	return grid
}

// SetScrollbackSize 设滚回缓冲最多存几行（官方在这里是 xterm 的
// `scrollback` 选项：`terminal.tsx:104` 把它设成 `environment.scrollback ?? 0`，
// 也就是**宿主说多少就是多少**，xterm 自己的默认 1000 被宿主的真值覆盖）。
//
// 为什么必须由壳体显式设：上游 `Screen` 默认 10000 行（`screen.go:26` 的
// `NewScrollback(DefaultScrollbackSize)`），比官方多一个数量级；
// 滚回越长，一次窗口快照要碰的格子越多，也越偏离官方（xterm 里滚到底就**看不到**
// 更老的行）。传 <=0 = 关掉滚回（官方 `scrollback: 0` 的语义）。
func (e *Emulator) SetScrollbackSize(maxLines int) {
	if maxLines <= 0 {
		// 关掉：上游 `SetMaxLines(<=0)` 是**空操作**（默认 10000 会留着），
		// 所以这里先清空、把上限压到 1 行（内存有界），再由 scrollbackOff
		// 让本包的投影报 0 行——界面据此不留滚动条槽（官方 FitAddon 同样
		// 在 `scrollback === 0` 时扣 0px）。
		e.scrollbackOff = true
		e.inner.ClearScrollback()
		e.inner.SetScrollbackSize(1)
		return
	}
	e.scrollbackOff = false
	e.inner.SetScrollbackSize(maxLines)
}

// Window 返回「滚回 + 当前屏」这条**虚拟缓冲**里第 first 行起共 lines 行的快照，
// 投影与 Grid 相同（同一套 Cell / Style），专门给「用户往上滚」这一种取帧用。
//
// 为什么不是「先取整块 Scrollback 再自己切」：滚回 1000 行 × 90 列 = 9 万格，
// 实测 1.73 ms 一帧（每格一次读锁，见 Scrollback 的注），而窗口只要 24 行
// ≈ 0.04 ms——差 40 倍。渲染器每帧只需要**看得见的那几行**。
//
// 坐标：0 是滚回里最老的一行，ScrollbackLen() 是第一行**屏幕**行；也就是
// `ScrollbackLen()+屏幕高` 是虚拟缓冲的总行数。参数越界按夹住处理：
// first<0 → 0，窗口底边超过总行数就截短（返回的 Height 可能小于 lines）。
//
// 光标恒不可见：调用方只在**离开底部**时用这个窗口（滚到底就该画 Grid()，
// 那里才有光标——官方 xterm 同样只在 ydisp==ybase 时画光标）。
func (e *Emulator) Window(first, lines int) *Grid {
	w, h := e.Size()
	n := e.ScrollbackLen()
	grid := &Grid{Width: w}
	if w <= 0 || lines <= 0 {
		return grid
	}
	total := n + h
	if first < 0 {
		first = 0
	}
	if first > total {
		first = total
	}
	if first+lines > total {
		lines = total - first
	}
	if lines <= 0 {
		return grid
	}
	grid.Height = lines
	grid.Cells = make([]Cell, w*lines)
	for y := 0; y < lines; y++ {
		line := first + y
		for x := 0; x < w; x++ {
			// 前半段在滚回里，后半段就是当前屏——两段用同一套投影，
			// 所以滚到底与画屏幕接缝处不会出现两种样式。
			var c *uv.Cell
			if line < n {
				c = e.inner.ScrollbackCellAt(x, line)
			} else {
				c = e.inner.CellAt(x, line-n)
			}
			if c == nil {
				continue
			}
			grid.Cells[y*w+x] = Cell{Content: c.Content, Width: c.Width, Style: styleOf(&c.Style)}
		}
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
