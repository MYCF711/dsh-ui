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
	box := ui.Stack(f).FillWidth().MinHeight(0).Clip()
	if height > 0 {
		box = box.Height(height)
	} else {
		box = box.Grow(1)
	}
	box.Draw(func(d *ui.DrawContext) {
		p.paintGrid(d, grid, maxCols, maxRows, func(cols, rows int) {
			// 官方 fitScreen 的后半段：量出来的尺寸交给 model.resize。
			// 去重（同尺寸/算不出来不发）在 requestTerminalResize 里按官方判据做。
			p.requestTerminalResize(env, tabID, state, cols, rows)
		})
	})
}

// paintGrid 逐格画：背景块、字形 run、光标。
//
// onFit 拿到的是「这块面板按官方 fitScreen 口径能放几列几行」，在裁剪之前调用：
// 屏幕比面板小时也该把 PTY 放大到面板尺寸，那正是 fit 的意义。
func (p *Plugin) paintGrid(d *ui.DrawContext, grid *vterm.Grid, maxCols, maxRows int, onFit func(cols, rows int)) {
	if d == nil || d.Canvas == nil || grid == nil {
		return
	}
	canvas := d.Canvas
	cellW, cellH := canvas.MonoCellSize(terminalFontSize, gfx.MonoSystem)
	if cellW <= 0 || cellH <= 0 {
		return
	}
	// 这块面板按官方 fitScreen 口径能放几列几行（含 environment 上限）。
	// 屏幕比绘制面大时按整格裁掉：不缩放、不半格偏移——那是自造几何。
	wantCols, wantRows := terminalFitSize(d.Rect.Width, d.Rect.Height, cellW, cellH, maxCols, maxRows)
	if wantCols <= 0 || wantRows <= 0 {
		return
	}
	if onFit != nil {
		onFit(wantCols, wantRows)
	}
	cols := min(grid.Width, wantCols)
	rows := min(grid.Height, wantRows)
	if cols <= 0 || rows <= 0 {
		return
	}
	x0, y0 := d.Rect.X, d.Rect.Y
	for y := 0; y < rows; y++ {
		top := y0 + float32(y)*cellH
		p.paintGridBackground(canvas, grid, y, cols, x0, top, cellW, cellH)
		p.paintGridText(canvas, grid, y, cols, x0, top, cellW, cellH)
	}
	p.paintGridCursor(canvas, grid, cols, rows, x0, y0, cellW, cellH)
}

// paintGridBackground 画一行的背景块：同色相邻格合并成一个矩形，
// 默认背景（就是 screen 自己的底色）不画。
func (p *Plugin) paintGridBackground(canvas *gfx.Canvas, grid *vterm.Grid, y, cols int, x0, top, cellW, cellH float32) {
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
		cell, ok := grid.Cell(x, y)
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
func (p *Plugin) paintGridText(canvas *gfx.Canvas, grid *vterm.Grid, y, cols int, x0, top, cellW, cellH float32) {
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
		cell, ok := grid.Cell(x, y)
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
