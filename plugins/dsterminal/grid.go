package dsterminal

import (
	"math"
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/vterm"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 这个文件是**网格渲染路径**：把 `internal/vterm` 的屏幕快照逐格画到画布上。
//
// 与旧的文本投影（terminalLines + ui.Text）的区别就是「屏幕」与「读到的字」：
//   - 光标定位绘制（vim / htop）在文本投影里只剩乱序碎片，在这里是原位覆盖
//   - 颜色、反显、下划线在剥序列那一层已经丢了，这里逐格带属性
//   - 全角字占两格，文本投影里宽度靠字体自己猜，这里按单元格位置排
//
// 几何与颜色**不在这里定**：格子的宽高来自画布对等宽字体的实测
// （`Canvas.MonoCellSize`，官方是 xterm 测量同一米字格），颜色来自官方调色板
// （internal/vterm 已把索引色解析成 RGB），默认前景/背景来自主题
// （官方 terminal-theme.ts:60 把 fg/bg 交给界面：cursor=foreground）。

const (
	// 光标闪烁：官方 CSS `animation: terminal-cursor-blink 1s step-end infinite`
	// （terminal.module.css:18-19）：0% 画光标色、50% 画单元格自己的颜色。
	// bar / underline 两档官方由 xterm 自带的 CSS 挂同一条 1s step-end 动画。
	terminalCursorBlinkPeriod = time.Second

	// bar 形光标宽 1px：terminal.module.css:16
	// `box-shadow:1px 0 0 var(--terminal-cursor) inset`；xterm 的 cursorWidth 默认 1。
	terminalCursorBarWidth float32 = 1
	// underline 形光标厚 1px：xterm 的 `.xterm-cursor-underline{border-bottom:1px …}`，
	// 官方 module CSS 只覆盖它的 border-bottom-color（terminal.module.css:17）。
	terminalCursorUnderlineHeight float32 = 1

	// terminalDimAlpha 是 faint（SGR 2）的前景透明度。
	//
	// 官方 xterm 的 dim **不是**给整行加 opacity：内联进官方包的 xterm.css 里有
	// `.xterm-dim{opacity:1!important}` 把那条关掉
	// （`dsh-client-ui-sidebar-terminal/lib/client.terminal.js` byte 491841），
	// 改由主题样式表**逐类**下发一个半透明的前景色：
	//
	//	`${selector} .xterm-dim { color: ${multiplyOpacity(foreground,.5).css} }`
	//	`${selector} .xterm-fg-N.xterm-dim { color: ${multiplyOpacity(ansi[N],.5).css} }`
	//	`${selector} .xterm-fg-INVERTED_DEFAULT_COLOR.xterm-dim { … }`
	//
	// （同文件 byte 76534 / 78917 / 79214；`_setTheme` 里生成这段 CSS 的那条语句
	// 就是 `.xterm-dim` 的唯一实现）。所以「faint = 前景色 alpha 0.5，由浏览器
	// 合成在格子底色上」——这个 0.5 是官方值，不是我们估的亮度。
	//
	// 我们走同一条合成链：底色由 paintGridBackground 先画好，字形 run 带 0.5 alpha。
	// `gfx.Color` 有 Alpha（`viva@v0.6.4/gfx/canvas.go:14-19`，`ui.Color` 是它的别名
	// `ui/vocab.go:10`），Viva 的 Windows 文本路径逐 run 取用
	// （`internal/platform/windows/direct_grid_windows.go:44` → `renderer_windows.go:687`
	// 的 `frame.TextColor = […A]`），所以这里不需要自己算混色。
	terminalDimAlpha float32 = 0.5
)

// gridColor 把 vterm 解析好的颜色换成画布颜色。
func gridColor(c vterm.Color) ui.Color {
	return gfx.RGBA(c.R, c.G, c.B, float32(c.A)/255)
}

// terminalCellPaint 是一格「画出来」的样子：颜色已解析、反显已换好。
type terminalCellPaint struct {
	fg, bg                                   ui.Color
	bold, italic, underline, strike, conceal bool
}

// paintOf 解析一格的颜色与属性。
//
// 默认前景/背景来自主题：官方把 xterm 的 foreground/background 设成页面自己的
// 颜色（terminal-theme.ts:57-64 的 `update(background, foreground)`，调用点在
// terminal.tsx:124-127 用 `getComputedStyle` 取 `.screen` 的那两个值）。
// 反显在这里换色——只有这里同时知道两个默认值。
func (p *Plugin) paintOf(style vterm.Style) terminalCellPaint {
	fg, bg := p.theme.LabelPrimary, p.theme.BgBase
	if style.Fg.Set {
		fg = gridColor(style.Fg)
	}
	if style.Bg.Set {
		bg = gridColor(style.Bg)
	}
	if style.Attrs.Has(vterm.AttrReverse) {
		fg, bg = bg, fg
	}
	out := terminalCellPaint{
		fg: fg, bg: bg,
		bold:      style.Attrs.Has(vterm.AttrBold),
		italic:    style.Attrs.Has(vterm.AttrItalic),
		underline: style.Attrs.Has(vterm.AttrUnderline),
		strike:    style.Attrs.Has(vterm.AttrStrike),
		conceal:   style.Attrs.Has(vterm.AttrConceal),
	}
	// SGR 2 faint：淡化的是**最终**前景色。次序与 xterm 的 createRow 一致——
	// 先反显换色（上面那次 swap）、粗体的亮色映射已在 internal/vterm 的 styleOf
	// 里做过，最后才把 alpha 降到 0.5（terminalDimAlpha）。conceal 排在它后面：
	// SGR 8 的格子本来就画成底色，不该被 faint 再调一次。
	if style.Attrs.Has(vterm.AttrFaint) {
		out.fg = out.fg.WithAlpha(terminalDimAlpha)
	}
	if out.conceal {
		// SGR 8：字形不可见。官方 xterm 画成与底色同色（不是跳过不画），
		// 这样下划线/反显的几何仍然在。
		out.fg = out.bg
	}
	return out
}

// gridRowHeight 是网格一行的像素高：等宽字体的行盒，来自画布实测。
//
// 官方这边是 xterm 量出来的 cell height（fontSize × lineHeight，lineHeight 默认 1，
// 见官方终端包里内联的 xterm DEFAULT_OPTIONS），不是 13px 本身——13 是字号。
func (p *Plugin) gridRowHeight(env *pluginapi.Env) float32 {
	if env != nil && env.Canvas != nil {
		if _, h := env.Canvas.MonoCellSize(terminalFontSize, gfx.MonoSystem); h > 0 {
			return h
		}
	}
	// 画布还量不了字体（无引擎的静态预览）：退回旧文本路径同一档行盒，
	// 保证浮层高度不会因为量不到而塌成 0。
	return terminalFontSize
}

// drawTerminalGrid 声明网格的绘制面。height<=0 表示由容器分配剩余高度
// （右栏面板有确定高度）；>0 是浮层那种由内容决定高度的情形。
//
// maxCols/maxRows 是宿主 `terminal.environment` 给的上限（<=0 = 没给上限）。
// 它们既决定「这块面板放得下多少格」（裁绘），也决定「该向宿主申请多大」
// （resize），见 resize.go 的 terminalFitSize / terminalResizeWanted。
//
// 绘制面是唯一知道**真实矩形**的地方，所以 resize 的判断也挂在这里：
// 面板量出来的尺寸与当前 PTY 不同时，paintGrid 会经 onFit 回调把它发出去。
func (p *Plugin) drawTerminalGrid(f ui.C, env *pluginapi.Env, tabID string, state pluginapi.TerminalViewState, height float32, maxCols, maxRows int) {
	grid := state.Grid
	if grid == nil {
		return
	}
	// 滚动条槽宽与 fit 用的是**同一个数**（官方 scrollback===0 ? 0 : 14）。
	gutter := terminalScrollbarGutter(terminalScrollbackCapacity(state))
	box := ui.Stack(f).FillWidth().MinHeight(0).Clip()
	if height > 0 {
		box = box.Height(height)
	} else {
		box = box.Grow(1)
	}
	box.Draw(func(d *ui.DrawContext) {
		p.paintGrid(d, env, tabID, state, grid, gutter, maxCols, maxRows)
	})
}

// paintGrid 逐格画：背景块、字形 run、光标，外加官方那条滚动条。
//
// 两块屏幕在这里分叉：**贴底**画 `state.Grid`（当前屏，带光标），**滚上去了**
// 就画壳按坐标切出来的窗口（`TerminalScrollbackWindow`，没有光标）。官方那边
// 这一步是 xterm 自己的 viewport 按 `ydisp` 选行；滚动位置在界面侧，所以取哪
// 一段由这里说，壳只负责切（见 pluginapi.Actions.TerminalScrollbackWindow 的
// 成本说明）。
func (p *Plugin) paintGrid(d *ui.DrawContext, env *pluginapi.Env, tabID string, state pluginapi.TerminalViewState, grid *vterm.Grid, gutter float32, maxCols, maxRows int) {
	if d == nil || d.Canvas == nil || grid == nil {
		return
	}
	canvas := d.Canvas
	cellW, cellH := canvas.MonoCellSize(terminalFontSize, gfx.MonoSystem)
	if cellW <= 0 || cellH <= 0 {
		return
	}
	// 这块面板按官方 fitScreen 口径能放几列几行（含 environment 上限与滚动条槽）。
	// 屏幕比绘制面大时按整格裁掉：不缩放、不半格偏移——那是自造几何。
	wantCols, wantRows := terminalFitSize(d.Rect.Width, d.Rect.Height, cellW, cellH, maxCols, maxRows, gutter)
	if wantCols <= 0 || wantRows <= 0 {
		return
	}
	// 官方 fitScreen 的后半段：量出来的尺寸交给 model.resize。
	// 去重（同尺寸/算不出来不发）在 requestTerminalResize 里按官方判据做。
	p.requestTerminalResize(env, tabID, state, wantCols, wantRows)
	cols := min(grid.Width, wantCols)
	rows := min(grid.Height, wantRows)
	if cols <= 0 || rows <= 0 {
		return
	}
	x0, y0 := d.Rect.X, d.Rect.Y
	p.rememberWheelTarget(canvas, env, tabID, cols, rows, cellW, cellH, gutter, d.Rect, state.ScrollbackLen)

	// 滚回：只有壳给了窗口才滚得动（老壳没有那条动作，界面按「没有历史」画）。
	scrollback := max(state.ScrollbackLen, 0)
	view := p.scrollViewFor(tabID)
	ydisp := scrollback
	if scrollback > 0 && env != nil && env.Act.TerminalScrollbackWindow != nil {
		ydisp = view.resolve(scrollback)
	}
	p.wheelYdisp = ydisp
	if ydisp < scrollback {
		if window := env.Act.TerminalScrollbackWindow(tabID, ydisp, rows); window != nil {
			if windowRows := min(window.Height, rows); windowRows > 0 {
				p.paintGridRows(canvas, window, 0, windowRows, cols, x0, y0, cellW, cellH)
				p.paintScrollbar(canvas, d.Rect, gutter, rows, scrollback+grid.Height, ydisp, cellH)
				return
			}
		}
	}
	p.paintGridRows(canvas, grid, 0, rows, cols, x0, y0, cellW, cellH)
	p.paintGridCursor(canvas, grid, cols, rows, x0, y0, cellW, cellH)
	p.paintScrollbar(canvas, d.Rect, gutter, rows, scrollback+grid.Height, scrollback, cellH)
}

// rememberWheelTarget 记下这一帧的滚轮几何，并声明命中区。
//
// 命中区是**整块终端**（官方 xterm 在整块上 `preventDefault` 吃掉滚轮，
// 不只是文字行），这样 HandleScroll 用 `event.Target` 就能判「手势落在我身上」。
func (p *Plugin) rememberWheelTarget(canvas *gfx.Canvas, env *pluginapi.Env, tabID string, cols, rows int, cellW, cellH, gutter float32, rect ui.Rect, scrollback int) {
	if env != nil && env.Tree != nil {
		p.wheelHit = env.Tree.ID(env.Feat, "terminal-screen")
	}
	p.wheelTab, p.wheelCellH = tabID, cellH
	p.wheelScrollback = max(scrollback, 0)
	if p.wheelHit.IsZero() || p.wheelCellH <= 0 {
		return
	}
	p.wheelRect = ui.Rect{
		X: rect.X, Y: rect.Y,
		Width:  float32(cols)*cellW + gutter,
		Height: float32(rows) * cellH,
	}
	if canvas != nil {
		// 语义是「一块不带动作的区域」而不是按钮：它只为把滚轮路由到终端，
		// 按下去不该触发任何东西，也不该变成键盘 Tab 站（SkipFocus）。用
		// RoleTerminal 会更贴切，但那会让焦点变成**裸键盘**（Viva 的 RoleTerminal
		// 语义），而本仓终端还没有原始键输入这条链路——那会把用户的按键吞掉。
		canvas.HitWithOptions(p.wheelHit, p.wheelRect, terminalWheelLabel, gfx.HitOptions{
			Role: gfx.RoleGroup, SkipFocus: true,
		})
	}
}

// paintGridRows 把 source 的第 offset 行起共 rows 行画到 (x0,y0)。
func (p *Plugin) paintGridRows(canvas *gfx.Canvas, source *vterm.Grid, offset, rows, cols int, x0, y0, cellW, cellH float32) {
	for y := 0; y < rows; y++ {
		top := y0 + float32(y)*cellH
		p.paintGridBackground(canvas, source, offset+y, cols, x0, top, cellW, cellH)
		p.paintGridText(canvas, source, offset+y, cols, x0, top, cellW, cellH)
	}
}

// paintGridBackground 画一行的背景块：同色相邻格合并成一个矩形，
// 默认背景（就是 screen 自己的底色）不画。
func (p *Plugin) paintGridBackground(canvas *gfx.Canvas, grid *vterm.Grid, row, cols int, x0, top, cellW, cellH float32) {
	start := -1
	var current ui.Color
	flush := func(end int) {
		if start < 0 || end <= start {
			return
		}
		canvas.Fill(gfx.Rect{
			X:      x0 + float32(start)*cellW,
			Y:      top,
			Width:  float32(end-start) * cellW,
			Height: cellH,
		}, current)
		start = -1
	}
	for x := 0; x < cols; x++ {
		cell, ok := grid.Cell(x, row)
		if !ok {
			flush(x)
			continue
		}
		paint := p.paintOf(cell.Style)
		if paint.bg == p.theme.BgBase {
			flush(x)
			continue
		}
		if start >= 0 && current != paint.bg {
			flush(x)
		}
		if start < 0 {
			start, current = x, paint.bg
		}
	}
	flush(cols)
}

// paintGridText 画一行的字形。
//
// 走 `Canvas.LabelRuns`——这正是画布给终端准备的口子：一条 label 是整行，
// 每个 run 自带起始列、跨几格、颜色、粗细、斜体、下划线。
//   - **全角字单独成 run**，跨 2 格，渲染器把它居中放在那两格里；
//   - 宽字形的第二格（Width==0 的占位）**跳过**，否则全角后面会多出一格空白；
//   - 相邻的半角同风格字合并成一个 run，长行不会碎成几十个 label。
func (p *Plugin) paintGridText(canvas *gfx.Canvas, grid *vterm.Grid, row, cols int, x0, top, cellW, cellH float32) {
	var value strings.Builder
	runs := make([]gfx.TextRun, 0, 16)
	var current terminalCellPaint
	open := false
	prevWide := false
	startByte, startCol, cells := 0, 0, 0

	flush := func() {
		if !open {
			return
		}
		if built := value.Len() - startByte; built > 0 {
			runs = append(runs, gfx.TextRun{
				Column:    int32(startCol),
				Bytes:     int32(built),
				Cells:     int32(cells),
				Color:     current.fg,
				Weight:    gridWeight(current.bold),
				Italic:    current.italic,
				Underline: current.underline,
				Strike:    current.strike,
			})
		}
		open = false
	}

	for x := 0; x < cols; x++ {
		cell, ok := grid.Cell(x, row)
		if !ok || cell.Width == 0 || cell.Content == "" {
			continue // 宽字形的占位尾巴
		}
		paint := p.paintOf(cell.Style)
		// 空格不画字形（渲染器画出来也只是一格空白），除非它自己有下划线/删除线
		// —— vim 的下划线空白就是这种。空白**打断 run**：run 的列宽必须与实际
		// 跨过的格子数一致，否则后面的字会整体错位。
		if cell.Content == " " && !paint.underline && !paint.strike {
			flush()
			prevWide = false
			continue
		}
		wide := cell.Width > 1
		same := open && !wide && !prevWide && paint.fg == current.fg &&
			paint.bold == current.bold && paint.italic == current.italic &&
			paint.underline == current.underline && paint.strike == current.strike
		if !same {
			flush()
			open = true
			current = paint
			startByte, startCol, cells = value.Len(), x, 0
		}
		value.WriteString(cell.Content)
		cells += cell.Width
		prevWide = wide
	}
	flush()

	if len(runs) == 0 {
		return
	}
	// Mono 让每个字符站在自己的格子上（宽字符两格、组合符零格）；
	// 行的基线由画布按 `MonoBaseline` 放进这一格高的行盒里。
	canvas.LabelRuns(value.String(), gfx.Rect{
		X: x0, Y: top, Width: float32(cols) * cellW, Height: cellH,
	}, gfx.TextStyle{
		Size: terminalFontSize, Mono: true, MonoFace: gfx.MonoSystem,
		Color: p.theme.LabelPrimary,
	}, cellW, runs)
}

// gridWeight 把粗体位换成字重。
func gridWeight(bold bool) float32 {
	if bold {
		return gfx.WeightBold
	}
	return gfx.WeightRegular
}

// paintGridCursor 画光标：形状按 DECSCUSR（block / bar / underline），
// 颜色按官方 terminal-cursor.ts 的对比度规则，闪烁按官方 CSS 的 1s 相位。
func (p *Plugin) paintGridCursor(canvas *gfx.Canvas, grid *vterm.Grid, cols, rows int, x0, y0, cellW, cellH float32) {
	cursor := grid.Cursor
	if !cursor.Visible || cursor.X < 0 || cursor.Y < 0 || cursor.X >= cols || cursor.Y >= rows {
		return
	}
	if cursor.Blink && !p.cursorVisibleNow(canvas) {
		return
	}
	cell, ok := grid.Cell(cursor.X, cursor.Y)
	width := 1
	if ok && cell.Width > 1 {
		width = cell.Width // 块光标盖住整个全角字
	}
	paint := p.paintOf(cell.Style)
	fill, accent := terminalCursorColors(p.theme.LabelPrimary, paint.bg)

	rect := gfx.Rect{
		X:      x0 + float32(cursor.X)*cellW,
		Y:      y0 + float32(cursor.Y)*cellH,
		Width:  float32(width) * cellW,
		Height: cellH,
	}
	switch cursor.Shape {
	case vterm.CursorBar:
		// 官方是格内左侧 1px 的 inset 影子（terminal.module.css:16）。
		canvas.Fill(gfx.Rect{X: rect.X, Y: rect.Y, Width: terminalCursorBarWidth, Height: rect.Height}, fill)
	case vterm.CursorUnderline:
		// 官方是格子底边上的一条线（terminal.module.css:17）。
		canvas.Fill(gfx.Rect{
			X:      rect.X,
			Y:      rect.Y + rect.Height - terminalCursorUnderlineHeight,
			Width:  rect.Width,
			Height: terminalCursorUnderlineHeight,
		}, fill)
	default:
		// 块光标：底色换光标色，格子里的字换成对比色重画一遍
		// （terminal.module.css:13-14 `background-color:var(--terminal-cursor);
		//  color:var(--terminal-cursor-accent)`）。
		canvas.Fill(rect, fill)
		if ok && cell.Content != "" && cell.Content != " " && !paint.conceal {
			canvas.LabelRuns(cell.Content, rect, gfx.TextStyle{
				Size: terminalFontSize, Mono: true, MonoFace: gfx.MonoSystem, Color: accent,
			}, cellW, []gfx.TextRun{{
				Column: 0,
				Bytes:  int32(len(cell.Content)),
				Cells:  int32(width),
				Color:  accent,
				Weight: gridWeight(paint.bold),
				Italic: paint.italic,
			}})
		}
	}
}

// cursorVisibleNow 报告这一帧该不该画光标，并且**预约下一帧**。
//
// 官方是 CSS 动画：`terminal-cursor-blink 1s step-end infinite`，0% 是光标色、
// 50% 是单元格色（terminal.module.css:18-19）。这里用画布时钟算同一条相位；
// 没有时钟（静态预览、单测）时停在动画 0% 那一档——**静止可见**，且不排帧，
// 免得无时钟的画布反复要帧。官方这个组件的 CSS 没有 reduced-motion 分支，
// 所以这里也不按 `ReduceMotion` 关闪烁（那是官方没有的行为）。
func (p *Plugin) cursorVisibleNow(canvas *gfx.Canvas) bool {
	if canvas == nil || !canvas.HasClock() {
		return true
	}
	phase := canvas.Now() % terminalCursorBlinkPeriod
	if phase < terminalCursorBlinkPeriod/2 {
		canvas.RequestFrameIn(terminalCursorBlinkPeriod/2 - phase)
		return true
	}
	canvas.RequestFrameIn(terminalCursorBlinkPeriod - phase)
	return false
}

// terminalCursorColors 是官方 `observeTerminalCursor` 的对比度规则
// （terminal-cursor.ts:26-35）：光标色默认取前景色，但当它与光标所在格的底色
// 对比度不足 3:1 时改成黑或白；字用 accent，按光标色的亮度取黑或白。
func terminalCursorColors(preferred, cellBackground ui.Color) (fill, accent ui.Color) {
	background := gridLuminance(cellBackground)
	requested := gridLuminance(preferred)
	contrast := (math.Max(background, requested) + 0.05) / (math.Min(background, requested) + 0.05)
	fill = preferred
	if contrast < 3 {
		if background > 0.179 {
			fill = gfx.RGB(0, 0, 0)
		} else {
			fill = gfx.RGB(255, 255, 255)
		}
	}
	if gridLuminance(fill) > 0.179 {
		accent = gfx.RGB(0, 0, 0)
	} else {
		accent = gfx.RGB(255, 255, 255)
	}
	return fill, accent
}

// gridLuminance 是 sRGB 相对亮度（WCAG / terminal-cursor.ts:40-46 同一套公式）。
func gridLuminance(c ui.Color) float64 {
	channel := func(v float32) float64 {
		x := float64(v)
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.Red) + 0.7152*channel(c.Green) + 0.0722*channel(c.Blue)
}
