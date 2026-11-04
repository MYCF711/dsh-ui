package mdlite

import (
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

// CopyFunc 是宿主注入的块级复制动作。text 始终是 CodeBlock.Code 的原始字节，
// 组件不做 TrimSpace、换行转换、路径解析，也不直接访问系统剪贴板。
type CopyFunc func(text string) error

// CopyStatus 是最近一次块级复制的可见结果。
type CopyStatus uint8

const (
	CopyIdle CopyStatus = iota
	CopySucceeded
	CopyFailed
)

// CodeBlockState 是可以随消息/会话保存的本地视图状态。复制反馈不属于它：
// 恢复旧布局不应把一次新复制伪装成已经成功。
type CodeBlockState struct {
	Wrap    bool
	OffsetX float32
}

// CodeBlockIDs 给三个真实交互目标提供稳定身份。Scroll 是横滚视口，Scrollbar
// 可留空；Copy/Wrap 留空时仍画预览按钮，但不声明死命中区。
type CodeBlockIDs struct {
	Copy      ui.ID
	Wrap      ui.ID
	Scroll    ui.ID
	Scrollbar ui.ID
}

// CodeBlockStyle 只含绘制令牌，不含宿主行为。
type CodeBlockStyle struct {
	Theme        ui.Theme
	Look         components.MarkdownLook
	BannerFill   ui.Color
	ButtonFill   ui.Color
	Success      ui.Color
	Failure      ui.Color
	FontSize     float32
	BannerHeight float32
	PadX         float32
	PadY         float32
	MonoFace     ui.MonoFace
}

// DefaultCodeBlockStyle 从 Viva 主题派生代码块样式，并补齐 Highlight 的颜色。
func DefaultCodeBlockStyle(theme ui.Theme) CodeBlockStyle {
	look := components.MarkdownLookFor(theme)
	look.Code[components.CodeComment-1] = theme.MutedForeground
	look.Code[components.CodeString-1] = theme.Chart[2]
	look.Code[components.CodeNumber-1] = theme.Chart[3]
	look.Code[components.CodeKeyword-1] = theme.Chart[1]
	look.Code[components.CodeFunction-1] = theme.Chart[4]
	look.Code[components.CodeVariable-1] = theme.Chart[0]
	size := theme.Metrics.FontSizeM - 1
	if size <= 0 {
		size = 12
	}
	return CodeBlockStyle{
		Theme: theme, Look: look,
		BannerFill: theme.Card, ButtonFill: theme.Hover,
		Success: theme.Success, Failure: theme.Danger,
		FontSize: size, BannerHeight: 32, PadX: 10, PadY: 8,
	}
}

// CodeBlockGeometry 是最近一次 Draw 的真实几何，供调用方占位和测试命中。
type CodeBlockGeometry struct {
	Bounds       ui.Rect
	Banner       ui.Rect
	Content      ui.Rect // 可见代码视口，不含 banner；横滚时已裁剪
	CopyBounds   ui.Rect
	WrapBounds   ui.Rect
	ContentWidth float32
	MaxOffsetX   float32
	VisualLines  int
}

type codeBlockRow struct {
	text  string
	marks []components.MarkdownCodeRun
}

type codeBlockLayout struct {
	valid                       bool
	code, language              string
	width, fontSize             float32
	padX, padY, bannerHeight    float32
	monoFace                    ui.MonoFace
	wrap                        bool
	rows                        []codeBlockRow
	cell, lineHeight            float32
	contentWidth, contentHeight float32
	bodyHeight                  float32
}

// CodeBlock 是一个独立的代码块视图。调用方通常先 Height 占位，再 Draw 到该矩形。
// 一个实例只由 UI  goroutine 使用；Copy 在 AfterPaint 中调用，避免阻塞绘制。
type CodeBlock struct {
	Language string
	Title    string
	Code     string
	Copy     CopyFunc

	overflow   Overflow
	wrap       bool
	feedback   CopyStatus
	layout     codeBlockLayout
	scrollID   ui.ID
	maxOffsetX float32
	runs       []ui.TextRun
}

// NewCodeBlock 从 Viva 的 fenced block 取原文和 info。block.Text 是什么就复制什么。
func NewCodeBlock(block markdown.Block, copy CopyFunc) CodeBlock {
	language, title := ParseCodeFenceInfo(block.Info)
	return CodeBlock{Language: language, Title: title, Code: block.Text, Copy: copy}
}

// ParseCodeFenceInfo 支持常见的 go,title=main.go 与 go title="main.go" 形式。
// 空语言显示 text；title 只是 banner 数据，不会被当作路径打开。
func ParseCodeFenceInfo(info string) (language, title string) {
	info = strings.TrimSpace(info)
	if info == "" {
		return "text", ""
	}
	language = info
	rest := ""
	if i := strings.IndexAny(info, " \t,"); i >= 0 {
		language, rest = info[:i], info[i:]
	}
	if language == "" {
		language = "text"
	} else {
		language = normalizeLanguage(language)
	}
	for rest != "" {
		rest = strings.TrimLeft(rest, " \t,")
		if rest == "" {
			break
		}
		if !strings.HasPrefix(rest, "title=") {
			if i := strings.IndexByte(rest, ','); i >= 0 {
				rest = rest[i+1:]
				continue
			}
			break
		}
		value := strings.TrimPrefix(rest, "title=")
		if value != "" && (value[0] == '"' || value[0] == '\'') {
			quote := value[0]
			value = value[1:]
			if i := strings.IndexByte(value, quote); i >= 0 {
				return language, value[:i]
			}
			return language, value
		}
		if i := strings.IndexAny(value, " \t,"); i >= 0 {
			return language, value[:i]
		}
		return language, value
	}
	return language, ""
}

// State 返回可序列化的 wrap/横滚状态。
func (b *CodeBlock) State() CodeBlockState {
	return CodeBlockState{Wrap: b.wrap, OffsetX: b.overflow.OffsetX}
}

// Restore 恢复布局状态；负位移按 0 处理，实际最大值在下一次 Draw 夹紧。
func (b *CodeBlock) Restore(state CodeBlockState) {
	b.wrap = state.Wrap
	b.overflow.OffsetX = maxFloat(0, state.OffsetX)
	b.feedback = CopyIdle
	b.layout.valid = false
}

// CopyStatus 返回最近一次复制结果。
func (b *CodeBlock) CopyStatus() CopyStatus { return b.feedback }

// Height 测量指定宽度下的完整高度（banner、全部代码行和必要横滚条）。
func (b *CodeBlock) Height(canvas *ui.Canvas, width float32, style CodeBlockStyle) float32 {
	layout := b.layoutFor(canvas, width, style)
	return layout.bannerHeight + layout.bodyHeight
}

// HandleScroll 处理宿主转发的真实滚轮/触控板事件。只响应本代码块视口上的
// 横向手势（或 Shift+纵向滚轮），绝不抢普通纵向消息滚动。
func (b *CodeBlock) HandleScroll(event ui.ScrollEvent) ui.EventResult {
	if b.wrap || b.scrollID.IsZero() || event.Target != b.scrollID {
		return ui.EventResult{}
	}
	delta := event.StepX(32)
	if delta == 0 && event.Modifiers&ui.ModifierShift != 0 {
		delta = event.StepY(32)
	}
	if delta == 0 {
		return ui.EventResult{}
	}
	b.overflow.Wheel(delta)
	if b.overflow.OffsetX > b.maxOffsetX {
		b.overflow.OffsetX = b.maxOffsetX
	}
	return ui.EventResult{Handled: true, Repaint: true}
}

// Draw 实画 banner、复制/wrap 按钮、代码及横滚视口，并声明真实 hit region。
func (b *CodeBlock) Draw(canvas *ui.Canvas, bounds ui.Rect, ids CodeBlockIDs, style CodeBlockStyle) CodeBlockGeometry {
	style = normalizeCodeBlockStyle(style)
	layout := b.layoutFor(canvas, bounds.Width, style)
	if bounds.Height <= 0 {
		bounds.Height = layout.bannerHeight + layout.bodyHeight
	}
	geometry := CodeBlockGeometry{
		Bounds: bounds, VisualLines: len(layout.rows), ContentWidth: layout.contentWidth,
	}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return geometry
	}
	canvas.Clip(bounds)
	defer canvas.Unclip()
	canvas.RoundedBorder(bounds, style.Look.Radius, 1, style.Look.Line, style.Look.CodeFill)

	geometry.Banner = ui.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: minFloat(layout.bannerHeight, bounds.Height)}
	bannerFill := style.BannerFill
	if bannerFill.Alpha == 0 {
		bannerFill = style.Look.CodeFill
	}
	canvas.Rounded(geometry.Banner, style.Look.Radius, bannerFill)
	canvas.Fill(ui.Rect{X: bounds.X, Y: geometry.Banner.Y + geometry.Banner.Height - 1, Width: bounds.Width, Height: 1}, style.Look.Line)

	geometry.CopyBounds, geometry.WrapBounds = b.drawBanner(canvas, geometry.Banner, ids, style)
	body := ui.Rect{
		X: bounds.X, Y: bounds.Y + geometry.Banner.Height,
		Width: bounds.Width, Height: maxFloat(0, bounds.Height-geometry.Banner.Height),
	}
	paintRows := func(content ui.Rect) {
		for i, row := range layout.rows {
			if row.text == "" {
				continue
			}
			rect := ui.Rect{
				X: content.X + layout.padX, Y: content.Y + layout.padY + float32(i)*layout.lineHeight,
				Width: float32(codeMonoCells(row.text)) * layout.cell, Height: layout.lineHeight,
			}
			b.runs = appendCodeBlockTextRuns(b.runs[:0], row, style)
			canvas.LabelRuns(row.text, rect, ui.TextStyle{
				Size: layout.fontSize, Color: style.Look.Text, Mono: true, MonoFace: layout.monoFace,
			}, layout.cell, b.runs)
		}
	}

	if b.wrap {
		b.scrollID = ui.ID{}
		b.maxOffsetX = 0
		geometry.Content = body
		geometry.ContentWidth = body.Width
		if body.Width > 0 && body.Height > 0 {
			canvas.Clip(body)
			paintRows(body)
			canvas.Unclip()
		}
	} else {
		b.scrollID = ids.Scroll
		scroll := b.overflow.DrawInteractive(
			canvas, style.Theme, body, layout.contentWidth, layout.contentHeight,
			ids.Scroll, ids.Scrollbar, "代码横向滚动", paintRows,
		)
		b.maxOffsetX = scroll.MaxOffsetX
		geometry.Content = scroll.Viewport
		geometry.MaxOffsetX = scroll.MaxOffsetX
	}
	return geometry
}

func (b *CodeBlock) drawBanner(canvas *ui.Canvas, banner ui.Rect, ids CodeBlockIDs, style CodeBlockStyle) (copyRect, wrapRect ui.Rect) {
	fontSize := style.Theme.Metrics.FontSizeS
	if fontSize <= 0 {
		fontSize = 11
	}
	buttonHeight := minFloat(24, maxFloat(0, banner.Height-8))
	y := banner.Y + (banner.Height-buttonHeight)/2
	copyLabel := "复制"
	copyColor := style.Look.Muted
	switch b.feedback {
	case CopySucceeded:
		copyLabel, copyColor = "已复制", style.Success
	case CopyFailed:
		copyLabel, copyColor = "复制失败", style.Failure
	}
	copyWidth := maxFloat(64, canvas.TextWidth("复制失败", fontSize)+18)
	wrapWidth := maxFloat(56, canvas.TextWidth("横滚", fontSize)+18)
	right := banner.X + banner.Width - 8
	copyRect = ui.Rect{X: maxFloat(banner.X, right-copyWidth), Y: y, Width: minFloat(copyWidth, banner.Width), Height: buttonHeight}
	right = copyRect.X - 6
	wrapRect = ui.Rect{X: maxFloat(banner.X, right-wrapWidth), Y: y, Width: minFloat(wrapWidth, maxFloat(0, right-banner.X)), Height: buttonHeight}

	textRight := wrapRect.X - 8
	language := b.Language
	if language == "" {
		language = "text"
	}
	languageWidth := canvas.TextWidth(language, fontSize)
	if languageWidth > 0 && banner.X+8 < textRight {
		rect := ui.Rect{X: banner.X + 8, Y: banner.Y, Width: minFloat(languageWidth, maxFloat(0, textRight-banner.X-8)), Height: banner.Height}
		canvas.Label(language, rect, ui.TextStyle{Size: fontSize, Color: style.Look.Muted, Weight: ui.WeightMedium})
		if b.Title != "" && rect.Width >= languageWidth {
			x := rect.X + languageWidth + 8
			canvas.Label("·", ui.Rect{X: x, Y: banner.Y, Width: 10, Height: banner.Height}, ui.TextStyle{Size: fontSize, Color: style.Look.Muted})
			canvas.Label(b.Title, ui.Rect{X: x + 12, Y: banner.Y, Width: maxFloat(0, textRight-x-12), Height: banner.Height}, ui.TextStyle{Size: fontSize, Color: style.Look.Text})
		}
	}

	wrapLabel := "折行"
	if b.wrap {
		wrapLabel = "横滚"
	}
	drawCodeBlockButton(canvas, wrapRect, ids.Wrap, "切换折行", wrapLabel, wrapLabel, fontSize, style.Look.Text, style, false)
	drawCodeBlockButton(canvas, copyRect, ids.Copy, "复制代码", copyLabel, copyLabel, fontSize, copyColor, style, b.Copy == nil)

	if canvas.Clicked(ids.Wrap) {
		canvas.AfterPaint(func() {
			b.wrap = !b.wrap
			b.layout.valid = false
		})
	}
	if canvas.Clicked(ids.Copy) && b.Copy != nil {
		source, copy := b.Code, b.Copy
		canvas.AfterPaint(func() {
			if err := copy(source); err != nil {
				b.feedback = CopyFailed
			} else {
				b.feedback = CopySucceeded
			}
		})
	}
	return copyRect, wrapRect
}

func drawCodeBlockButton(canvas *ui.Canvas, bounds ui.Rect, id ui.ID, accessibility, visual, label string, fontSize float32, textColor ui.Color, style CodeBlockStyle, disabled bool) {
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	fill := style.ButtonFill
	if canvas.Hovered() == id && !disabled {
		fill = style.Theme.Hover
	}
	canvas.Rounded(bounds, minFloat(style.Look.Radius, bounds.Height/2), fill)
	canvas.LabelCenter(visual, bounds, ui.TextStyle{Size: fontSize, Color: textColor})
	if !id.IsZero() {
		canvas.HitWithOptions(id, bounds, accessibility, ui.HitOptions{
			Role: ui.RoleButton, Cursor: ui.PointerCursorHand, Hint: label, Disabled: disabled,
		})
	}
}

func normalizeCodeBlockStyle(style CodeBlockStyle) CodeBlockStyle {
	if style.FontSize <= 0 {
		style.FontSize = 12
	}
	if style.BannerHeight <= 0 {
		style.BannerHeight = 32
	}
	if style.PadX < 0 {
		style.PadX = 0
	} else if style.PadX == 0 {
		style.PadX = 10
	}
	if style.PadY < 0 {
		style.PadY = 0
	} else if style.PadY == 0 {
		style.PadY = 8
	}
	return style
}

func (b *CodeBlock) layoutFor(canvas *ui.Canvas, width float32, style CodeBlockStyle) *codeBlockLayout {
	style = normalizeCodeBlockStyle(style)
	width = maxFloat(width, 1)
	if b.layout.valid && b.layout.code == b.Code && b.layout.language == b.Language &&
		b.layout.width == width && b.layout.fontSize == style.FontSize &&
		b.layout.padX == style.PadX && b.layout.padY == style.PadY &&
		b.layout.bannerHeight == style.BannerHeight && b.layout.monoFace == style.MonoFace && b.layout.wrap == b.wrap {
		return &b.layout
	}
	if b.layout.valid && b.layout.code != b.Code {
		b.feedback = CopyIdle
	}
	layout := &b.layout
	layout.valid = true
	layout.code, layout.language = b.Code, b.Language
	layout.width, layout.fontSize = width, style.FontSize
	layout.padX, layout.padY, layout.bannerHeight = style.PadX, style.PadY, style.BannerHeight
	layout.monoFace, layout.wrap = style.MonoFace, b.wrap
	layout.rows = layout.rows[:0]
	layout.cell, layout.lineHeight = canvas.MonoCellSize(style.FontSize, style.MonoFace)
	if layout.cell <= 0 {
		layout.cell = style.FontSize * 0.6
	}
	if layout.lineHeight <= 0 {
		layout.lineHeight = style.FontSize * 1.4
	}

	physicalLines := strings.Split(b.Code, "\n")
	maxCells := 0
	for _, source := range physicalLines {
		source = strings.TrimSuffix(source, "\r")
		line := spreadCodeBlockTabs(source)
		var marked []components.MarkdownCodeRun
		if len(line) <= 4000 {
			marked = Highlight(b.Language, line, nil)
		}
		if line == "" {
			layout.rows = append(layout.rows, codeBlockRow{})
			continue
		}
		if !b.wrap {
			layout.rows = append(layout.rows, codeBlockRow{text: line, marks: marked})
			maxCells = maxInt(maxCells, codeMonoCells(line))
			continue
		}
		cols := maxInt(int((width-2*layout.padX)/layout.cell), 1)
		for start := 0; start < len(line); {
			end, done := codeMonoBreak(line[start:], cols)
			end += start
			if end <= start { // 始终前进，即使调用方给了极窄宽度。
				_, size := utf8.DecodeRuneInString(line[start:])
				end = start + maxInt(size, 1)
			}
			row := codeBlockRow{text: line[start:end]}
			for _, mark := range marked {
				lo, hi := maxInt(mark.Start, start), minInt(mark.End, end)
				if hi > lo {
					row.marks = append(row.marks, components.MarkdownCodeRun{Start: lo - start, End: hi - start, Class: mark.Class})
				}
			}
			layout.rows = append(layout.rows, row)
			start = end
			if done {
				break
			}
		}
	}
	layout.contentHeight = float32(len(layout.rows))*layout.lineHeight + 2*layout.padY
	layout.contentWidth = width
	if !b.wrap {
		layout.contentWidth = float32(maxCells)*layout.cell + 2*layout.padX
	}
	layout.bodyHeight = layout.contentHeight
	if !b.wrap && layout.contentWidth > width {
		layout.bodyHeight += 10 // components.ScrollArea 的横滚条厚度。
	}
	return layout
}

func spreadCodeBlockTabs(line string) string {
	if !strings.ContainsRune(line, '\t') {
		return line
	}
	var out strings.Builder
	out.Grow(len(line) + 8)
	cells := 0
	for i := 0; i < len(line); {
		r, size := utf8.DecodeRuneInString(line[i:])
		if r == '\t' {
			fill := 4 - cells%4
			out.WriteString(strings.Repeat(" ", fill))
			cells += fill
			i += size
			continue
		}
		if r == utf8.RuneError && size == 1 {
			out.WriteByte(line[i])
		} else {
			out.WriteRune(r)
		}
		cells += codeMonoRuneCells(r)
		i += size
	}
	return out.String()
}

func codeMonoBreak(s string, cols int) (end int, done bool) {
	used := 0
	for index, r := range s {
		width := codeMonoRuneCells(r)
		if used+width > cols && used > 0 {
			return index, false
		}
		used += width
	}
	return len(s), true
}

func codeMonoCells(s string) int {
	cells := 0
	for _, r := range s {
		cells += codeMonoRuneCells(r)
	}
	return cells
}

func codeMonoRuneCells(r rune) int {
	switch {
	case r < 0x1100:
		return 1
	case r >= 0x1100 && r <= 0x115F, r >= 0x2E80 && r <= 0xA4CF, r >= 0xAC00 && r <= 0xD7A3,
		r >= 0xF900 && r <= 0xFAFF, r >= 0xFE30 && r <= 0xFE6F, r >= 0xFF00 && r <= 0xFF60,
		r >= 0xFFE0 && r <= 0xFFE6, r >= 0x1F300 && r <= 0x1FAFF, r >= 0x20000 && r <= 0x3FFFD:
		return 2
	}
	return 1
}

func appendCodeBlockTextRuns(dst []ui.TextRun, row codeBlockRow, style CodeBlockStyle) []ui.TextRun {
	at, column := 0, 0
	emit := func(start, end int, class components.MarkdownCodeClass) {
		color := style.Look.Text
		if class > 0 && int(class) <= len(style.Look.Code) {
			candidate := style.Look.Code[class-1]
			if candidate.Alpha > 0 {
				color = candidate
			}
		}
		for start < end {
			stop := start
			for stop < end && row.text[stop] < utf8.RuneSelf {
				stop++
			}
			if stop > start {
				cells := stop - start
				dst = append(dst, ui.TextRun{Column: int32(column), Bytes: int32(cells), Cells: int32(cells), Color: color})
				column += cells
				start = stop
				continue
			}
			r, size := utf8.DecodeRuneInString(row.text[start:end])
			cells := codeMonoRuneCells(r)
			dst = append(dst, ui.TextRun{Column: int32(column), Bytes: int32(size), Cells: int32(cells), Color: color})
			column += cells
			start += size
		}
	}
	for _, mark := range row.marks {
		start, end := maxInt(mark.Start, at), minInt(mark.End, len(row.text))
		if end <= start {
			continue
		}
		emit(at, start, 0)
		emit(start, end, mark.Class)
		at = end
	}
	emit(at, len(row.text), 0)
	return dst
}

func minFloat(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
