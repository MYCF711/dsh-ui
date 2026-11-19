// changes_split.go —— 官方 `DiffFile` 的左右对比（`data-split`）与换行（`data-wrap`），
// GUI 账本 #64 ③。
//
// # 官方规格（逐条 file:line，rc.2 随包 src）
//
//	OFF:…/src/client/DiffFile.tsx:191      `columns = split && file.additions>0 && file.deletions>0`
//	OFF:…/src/client/DiffFile.tsx:422-427  每个 hunk 的 rows 经 `splitRows()` 配对后画 `.pair`
//	OFF:…/src/client/DiffFile.tsx:354-359  `.cell` 里是 `number` + `Code`（左右对比没有符号列）
//	OFF:…/src/client/DiffFile.tsx:450      `data-split={columns?'':undefined}` / `data-wrap={wrap?'':undefined}`
//	OFF:…/src/client/diff-rows.ts:69-97    splitRows：连续删除块与随后连续新增块逐行配对，
//	                                       context 两列同号同文；`\ No newline` 不是一行（:53-60）
//	OFF:…/src/client/ChangesTab.module.css:613-616 `.code{overflow-x:auto;font:--dsw-font-markdown-code-block}`
//	OFF:…/src/client/ChangesTab.module.css:625-630 `.pair{grid-template-columns:minmax(0,1fr) minmax(0,1fr);min-height:22px;line-height:22px;white-space:pre}`
//	OFF:…/src/client/ChangesTab.module.css:633-639 `.cell{grid-template-columns:var(--forge-number-width,3.5em) minmax(0,1fr);min-width:0}`
//	OFF:…/src/client/ChangesTab.module.css:641-643 `.cell+.cell{border-left:.5px solid --dsw-alias-border-l3}`
//	OFF:…/src/client/ChangesTab.module.css:657-664 `.code[data-wrap] .line,.pair{white-space:pre-wrap}` / `.text{overflow-wrap:anywhere}`
//	OFF:…/src/client/ChangesTab.module.css:648-655 `.number{padding-right:6px;color:label-tertiary;text-align:right}`
//	OFF:…/src/client/ChangesTab.module.css:702-704 `.text{padding:0 16px 0 6px}`
//
// # 「格内独立横向滚动」怎么做到
//
// 官方 `.code{overflow-x:auto}` 是**整块**横向滚动；任务书要求「格内独立横向滚动
// （单元格自己的 bounds + 自己的 offset），其余列固定」。Viva 的
// `components.BeginScrollArea(canvas, theme, bounds, contentSize)` 接收**显式
// bounds**，并且自己 `canvas.Clip(geometry.Viewport)` / `Unclip`：
//
//	OFF:viva@v0.6.5/components/scroll_area.go:57-61  BeginScrollArea 收 bounds + contentSize
//	OFF:viva@v0.6.5/components/scroll_area.go:63-72  Offset(x,y) 是调用方持有的偏移
//	OFF:viva@v0.6.5/components/scroll_area.go:81-85  HorizontalScrollbar(id,label,value)
//	OFF:viva@v0.6.5/components/scroll_area.go:308-333 BeginContent/EndContent：Clip(Viewport) → Unclip → 画滚动条
//
// 所以「左右两列各自一个 BeginScrollArea，各自一个 Offsets 与 ScrollbarID」就是
// 字面意义上的「格内独立」：拖动左边列滚动条只改左边列的 offset，右边列不动。
// 两态：
//
//   - `wrap=false`：每列一个横向滚动视口，内容宽度 = max(行号列 + 正文实宽 + padding)，
//     滚动条用 BeginScrollArea 的 `HorizontalScrollbar` 画（**不手画**）；
//   - `wrap=true`：不做横向滚动，正文按列宽 `WrapLinesStyle` 折行（官方 `pre-wrap`），
//     偶数行高按折行数累加（`.pair{line-height:22px}`）。
//
// 换行模式的行高由文本测量得出，所以 Draw 阶段就要先排版：文本宽度需要 canvas，
// 用 `env.Canvas`（壳每帧都给，`pluginapi.Env.Canvas` 注释）。AfterDraw 拿到真实
// slot 矩形后再按真实宽度复排一次（宽度与上一帧估计不同时）。
package dsforge

import (
	"math"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方 ChangesTab.module.css:625-630 / :657-664 的三种行高。
const (
	changesDiffRowH  float32 = 22 // `.line,.pair{min-height:22px;line-height:22px}`
	changesDiffGapH  float32 = 26 // `.gap,.fileNote{min-height:26px}`
	changesDiffHalfP float32 = 0.5
)

const (
	changesDiffSideLeft  = "left"
	changesDiffSideRight = "right"
)

// changesLayoutCacheMax 是排版缓存条数上限（超出整表丢弃，简单且不会无限长）。
const changesLayoutCacheMax = 64

// changesSplitCell 是左右对比里的一格（官方 `SplitCell`，diff-rows.ts:16-20）。
type changesSplitCell struct {
	kind string // "context" / "add" / "del"
	no   int
	text string
}

// changesSplitPair 是左右对比的一行（官方 `SplitRow`，diff-rows.ts:22-26）。
type changesSplitPair struct {
	left  *changesSplitCell
	right *changesSplitCell
}

// changesSplitRows 是官方 `splitRows()`（OFF:src/client/diff-rows.ts:69-97）的逐行移植：
//
//   - 连续删除块与**随后**的连续新增块逐行配对（左删右增）；
//   - 删除块之后又出现新增块之外的删除时先 flush（所以 "删/增/删" 是两个块）；
//   - context 行 flush 当前块，并同时出现在左右两列；
//   - `\ No newline at end of file` 不产生行（官方 hunkRows 的 sign=='\\' 分支跳过）。
func changesSplitRows(rows []changesRow) []changesSplitPair {
	paired := make([]changesSplitPair, 0, len(rows))
	deleted := make([]changesSplitCell, 0, 4)
	added := make([]changesSplitCell, 0, 4)
	flush := func() {
		count := len(deleted)
		if len(added) > count {
			count = len(added)
		}
		for i := 0; i < count; i++ {
			pair := changesSplitPair{}
			if i < len(deleted) {
				cell := deleted[i]
				pair.left = &cell
			}
			if i < len(added) {
				cell := added[i]
				pair.right = &cell
			}
			paired = append(paired, pair)
		}
		deleted = deleted[:0]
		added = added[:0]
	}
	for _, row := range rows {
		if row.old == nil && row.new == nil {
			continue // `\ No newline at end of file`
		}
		switch row.sign {
		case "−":
			if len(added) > 0 {
				flush()
			}
			deleted = append(deleted, changesSplitCell{kind: "del", no: *row.old, text: row.text})
		case "+":
			added = append(added, changesSplitCell{kind: "add", no: *row.new, text: row.text})
		default:
			flush()
			left := changesSplitCell{kind: "context", no: *row.old, text: row.text}
			right := changesSplitCell{kind: "context", no: *row.new, text: row.text}
			paired = append(paired, changesSplitPair{left: &left, right: &right})
		}
	}
	flush()
	return paired
}

// changesDiffPaintCell 是一格排好版的内容：折行后的每一行 + 行号。
type changesDiffPaintCell struct {
	kind  string
	no    *int
	lines []string
	// width 是**不折行**时这一格的实宽（行号列 + 正文 + padding），
	// 用它算该列的横向滚动内容宽度。
	width float32
}

// changesDiffPaintRow 是排版后的一行：hunk 头 / 未变更提示 / 左右一对。
type changesDiffPaintRow struct {
	kind   string // "hunk" / "gap" / "pair"
	height float32
	text   string
	left   *changesDiffPaintCell
	right  *changesDiffPaintCell
}

// changesDiffLayout 是一份文件在左右对比下的完整排版。
type changesDiffLayout struct {
	width      float32 // 排版用的正文宽（slot 宽）
	wrap       bool
	rows       []changesDiffPaintRow
	height     float32
	leftWidth  float32 // 左列不折行内容宽
	rightWidth float32 // 右列不折行内容宽
}

// diffSlot 是本帧为一份文件正文占下的位置；AfterDraw 在布局完成后画内容。
type diffSlot struct {
	file   apiclient.ForgeDiffFile
	box    ui.Box
	layout *changesDiffLayout
}

// changesFrame 是 Draw 阶段交给 AfterDraw 的本帧几何。
// **只在同一帧内有效**（pluginapi.Painter.AfterDraw 的既有约定）。
type changesFrame struct {
	root        ui.Box
	filesScroll ui.Box
	// diffWidth 是本帧 diff 那一栏的宽度（宽栏时已扣掉树宽与 8px 分栏）；
	// 左右对比的排版按它算，才与 Viva 实际分给这一栏的宽度一致。
	diffWidth float32
	slots     []diffSlot
}

// changesBodyWidth 是本帧正文可用宽度。
//
// 官方 `.root{container:forge-changes / inline-size}` 的容器查询看的就是这个宽度；
// immediate mode 下没有容器查询，用「上一帧量到的 root.Rect().Width」作输入，
// 首帧退回壳算好的 `Geometry.RightWidth`（0 = 右栏不在，按窄栏处理）。
func (p *Plugin) changesBodyWidth(env *pluginapi.Env) float32 {
	if p.changesWidth > 0 {
		return p.changesWidth
	}
	if env != nil && env.Geometry.RightWidth > 0 {
		return env.Geometry.RightWidth
	}
	return 0
}

// changesFileBodyWidth 是文件卡里正文的宽度：diff 栏宽减去 `.files` 的 10px 内距
// （ChangesTab.module.css:361-372 的 `--forge-files-inset:10px`）与文件卡 0.5px 描边。
//
// **必须**用 `frame.diffWidth`（宽栏时已扣掉树与分栏），否则排版按整栏宽算、
// Viva 实际只给半栏宽，折行数会对不上，占位高度每帧都偏小（实测 879 vs 631）。
func (p *Plugin) changesFileBodyWidth(env *pluginapi.Env) float32 {
	width := p.frame.diffWidth
	if width <= 0 {
		width = p.changesBodyWidth(env)
	}
	width -= 2*changesFilesInset + 2*changesFileBorder
	if width < 1 {
		return 1
	}
	return width
}

// layoutSplitDiff 排一份文件的左右对比内容。canvas 为 nil 时量不了文本，返回 nil。
func (p *Plugin) layoutSplitDiff(canvas *gfx.Canvas, t *pluginapi.Theme, file apiclient.ForgeDiffFile, width float32, wrap bool) *changesDiffLayout {
	if canvas == nil || t == nil || width <= 0 {
		return nil
	}
	layout := &changesDiffLayout{width: width, wrap: wrap}
	numberW := changesNumberW(file, t.Fonts.Code)
	half := width / 2
	textWidth := half - numberW - changesTextPadL - changesTextPadR
	if textWidth < 1 {
		textWidth = 1
	}
	codeStyle := gfx.TextStyle{Size: t.Fonts.Code, Mono: true}

	appendNote := func(kind, text string, height float32) {
		layout.rows = append(layout.rows, changesDiffPaintRow{kind: kind, text: text, height: height})
		layout.height += height
	}
	appendPair := func(pair changesSplitPair) {
		row := changesDiffPaintRow{kind: "pair"}
		if pair.left != nil {
			cell := p.layoutSplitCell(canvas, codeStyle, *pair.left, numberW, textWidth, wrap)
			row.left = &cell
			if cell.width > layout.leftWidth {
				layout.leftWidth = cell.width
			}
		}
		if pair.right != nil {
			cell := p.layoutSplitCell(canvas, codeStyle, *pair.right, numberW, textWidth, wrap)
			row.right = &cell
			if cell.width > layout.rightWidth {
				layout.rightWidth = cell.width
			}
		}
		lines := 1
		if row.left != nil && len(row.left.lines) > lines {
			lines = len(row.left.lines)
		}
		if row.right != nil && len(row.right.lines) > lines {
			lines = len(row.right.lines)
		}
		row.height = float32(lines) * changesDiffRowH
		layout.rows = append(layout.rows, row)
		layout.height += row.height
	}

	prevNewEnd := 0
	drawn := 0
	for _, hunk := range file.Hunks {
		if drawn >= changesMaxLinesPerFile {
			break
		}
		if gap := hunk.NewStart - prevNewEnd; gap > 1 {
			appendNote("gap", sprintf(copyDiffUnchanged, itoa(gap-1)), changesDiffGapH)
			drawn++
		}
		appendNote("hunk", hunkHeader(hunk), changesDiffRowH)
		drawn++
		oldNo, newNo := hunk.OldStart, hunk.NewStart
		rows := make([]changesRow, 0, len(hunk.Lines))
		for _, line := range hunk.Lines {
			if drawn >= changesMaxLinesPerFile {
				break
			}
			row := changesDiffRowOf(line, oldNo, newNo)
			if row.old != nil {
				oldNo++
			}
			if row.new != nil {
				newNo++
			}
			rows = append(rows, row)
			drawn++
		}
		for _, pair := range changesSplitRows(rows) {
			appendPair(pair)
		}
		prevNewEnd = hunk.NewStart + hunk.NewLines
	}
	return layout
}

// layoutSplitCell 排一格：不折行时一行、并算出实宽；折行时按列宽折成多行。
func (p *Plugin) layoutSplitCell(canvas *gfx.Canvas, style gfx.TextStyle, cell changesSplitCell, numberW, textWidth float32, wrap bool) changesDiffPaintCell {
	painted := changesDiffPaintCell{kind: cell.kind, lines: []string{cell.text}}
	no := cell.no
	painted.no = &no
	if wrap {
		lines := canvas.WrapLinesStyle(nil, cell.text, style, textWidth, math.MaxInt32, nil)
		if len(lines) == 0 {
			lines = []string{""}
		}
		painted.lines = lines
		painted.width = textWidth
		return painted
	}
	painted.width = numberW + changesTextPadL + changesTextPadR + canvas.TextWidthStyle(cell.text, style)
	return painted
}

// splitLayout 取（并缓存）一份文件的左右对比排版。
//
// 排版要逐行量文本，diff 大时每帧重排很贵；键是「路径 + digest + 宽度 + 折行 + 字号」，
// 证据变了才重排。缓存只存活在内存，且总条数封顶（换会话/刷新时不会无限长）。
func (p *Plugin) splitLayout(env *pluginapi.Env, file apiclient.ForgeDiffFile, width float32, wrap bool) *changesDiffLayout {
	t := p.themeOf(env)
	if t == nil || env == nil || env.Canvas == nil || width <= 0 {
		return nil
	}
	key := file.Path + "|" + file.Digest + "|" + itoa(int(width)) + "|" + itoa(boolDigit(wrap)) + "|" + itoa(int(t.Fonts.Code*100))
	if file.Digest != "" {
		if layout := p.open.layouts[key]; layout != nil {
			return layout
		}
	}
	layout := p.layoutSplitDiff(env.Canvas, t, file, width, wrap)
	if layout == nil {
		return nil
	}
	if file.Digest == "" {
		return layout // 没有 digest 就不缓存（同一路径可能换了内容）
	}
	if p.open.layouts == nil || len(p.open.layouts) >= changesLayoutCacheMax {
		p.open.layouts = map[string]*changesDiffLayout{}
	}
	p.open.layouts[key] = layout
	return layout
}

func boolDigit(value bool) int {
	if value {
		return 1
	}
	return 0
}

// drawSplitHunks 给一份文件的正文占位；内容在 AfterDraw 画（左右两列各自滚动）。
func (p *Plugin) drawSplitHunks(f ui.C, env *pluginapi.Env, file apiclient.ForgeDiffFile) {
	wrap := p.open.wrapOn(fileSession(env))
	layout := p.splitLayout(env, file, p.changesFileBodyWidth(env), wrap)
	if layout == nil {
		// 量不了文本（无 canvas）：退回单栏（官方两态之一，不是自造第三态）。
		p.drawHunks(f, env, file, wrap)
		return
	}
	slot := ui.Slot(f).FillWidth().Height(layout.height)
	p.frame.slots = append(p.frame.slots, diffSlot{file: file, box: slot, layout: layout})
}

// fileSession 取本帧会话 id（diff 数据面已判过同一会话）。
func fileSession(env *pluginapi.Env) string {
	if env == nil {
		return ""
	}
	return env.Data.Cur
}

// afterDrawChanges 是 `forge-changes-tab` 的 AfterDraw：布局已定，画左右对比正文。
func (p *Plugin) afterDrawChanges(canvas *gfx.Canvas, env *pluginapi.Env) {
	frame := p.frame
	if canvas == nil || env == nil || len(frame.slots) == 0 {
		p.frame = changesFrame{}
		p.diffTargets = nil // 本帧没有左右对比：上一帧的视口映射不许留着
		return
	}
	if frame.root.Valid() {
		if width := frame.root.Rect().Width; width > 0 {
			p.changesWidth = width
		}
	}
	scroll := env.Bounds
	if frame.filesScroll.Valid() {
		scroll = frame.filesScroll.Rect()
	}
	targets := map[gfx.ID]diffScrollTarget{}
	for _, slot := range frame.slots {
		rect := slot.box.Rect()
		if rect.Width <= 1 || rect.Height <= 0 {
			continue
		}
		visible := intersectRects(rect, scroll)
		if visible.Width <= 0 || visible.Height <= 0 {
			continue
		}
		layout := slot.layout
		if layout == nil || !nearlyEqual(layout.width, rect.Width) {
			env.Canvas = canvas
			layout = p.splitLayout(env, slot.file, rect.Width, p.open.wrapOn(fileSession(env)))
			if layout == nil {
				continue
			}
		}
		p.paintSplit(canvas, env, slot.file, rect, visible, layout, targets)
	}
	p.diffTargets = targets
	p.frame = changesFrame{}
}

// diffScrollTarget 把 canvas 命中 id 映射回（会话，文件，左/右列）。
type diffScrollTarget struct {
	session string
	path    string
	side    string
}

// diffViewportID / diffScrollbarID 是左右两列各自的稳定命中 id。
func diffViewportID(env *pluginapi.Env, path, side string) gfx.ID {
	if env == nil || env.Tree == nil {
		return gfx.ID{}
	}
	return env.Tree.ID(env.Feat, "forge-diff-lines:"+side+":"+path)
}

func diffScrollbarID(env *pluginapi.Env, path, side string) gfx.ID {
	if env == nil || env.Tree == nil {
		return gfx.ID{}
	}
	return env.Tree.ID(env.Feat, "forge-diff-hscroll:"+side+":"+path)
}

// paintSplit 画一份文件：先铺所有行底色与整宽行，再画左右两列（各自滚动/折行）。
func (p *Plugin) paintSplit(canvas *gfx.Canvas, env *pluginapi.Env, file apiclient.ForgeDiffFile, rect, visible gfx.Rect, layout *changesDiffLayout, targets map[gfx.ID]diffScrollTarget) {
	t := p.themeOf(env)
	if t == nil {
		return
	}
	canvas.Clip(visible)
	defer canvas.Unclip()

	numberW := changesNumberW(file, t.Fonts.Code)
	half := rect.Width / 2
	leftRect := gfx.Rect{X: rect.X, Y: rect.Y, Width: half, Height: rect.Height}
	rightRect := gfx.Rect{X: rect.X + half, Y: rect.Y, Width: rect.Width - half, Height: rect.Height}

	y := rect.Y
	for _, row := range layout.rows {
		switch row.kind {
		case "hunk":
			canvas.Label(row.text, gfx.Rect{X: rect.X + changesTextPadL, Y: y, Width: rect.Width - changesTextPadL - changesTextPadR, Height: row.height},
				gfx.TextStyle{Size: t.Fonts.Code, Mono: true, Color: t.CodeComment})
		case "gap":
			canvas.Fill(gfx.Rect{X: rect.X, Y: y, Width: rect.Width, Height: row.height}, t.CardFill)
			canvas.Label(row.text, gfx.Rect{X: rect.X + changesNotePadX, Y: y, Width: rect.Width - 2*changesNotePadX, Height: row.height},
				gfx.TextStyle{Size: changesFont, Color: t.LabelTertiary})
		case "pair":
			if row.left != nil {
				if bg, gutter := diffCellColors(t, row.left.kind); bg.Alpha > 0 {
					canvas.Fill(gfx.Rect{X: leftRect.X, Y: y, Width: leftRect.Width, Height: row.height}, bg)
					canvas.Fill(gfx.Rect{X: leftRect.X, Y: y, Width: numberW, Height: row.height}, gutter)
				}
			}
			if row.right != nil {
				if bg, gutter := diffCellColors(t, row.right.kind); bg.Alpha > 0 {
					canvas.Fill(gfx.Rect{X: rightRect.X, Y: y, Width: rightRect.Width, Height: row.height}, bg)
					canvas.Fill(gfx.Rect{X: rightRect.X, Y: y, Width: numberW, Height: row.height}, gutter)
				}
			}
			canvas.Fill(gfx.Rect{X: rect.X + half, Y: y, Width: changesDiffHalfP, Height: row.height}, t.BorderL3)
		}
		y += row.height
	}

	if layout.wrap {
		y = rect.Y
		for _, row := range layout.rows {
			if row.kind == "pair" {
				if row.left != nil {
					p.paintSplitCell(canvas, t, numberW, *row.left, leftRect.X, y)
				}
				if row.right != nil {
					p.paintSplitCell(canvas, t, numberW, *row.right, rightRect.X, y)
				}
			}
			y += row.height
		}
		return
	}

	p.paintSplitColumn(canvas, env, file, changesDiffSideLeft, leftRect, layout, targets, numberW)
	p.paintSplitColumn(canvas, env, file, changesDiffSideRight, rightRect, layout, targets, numberW)
}

// diffCellColors 返回一格的底色与行号槽底色（官方 `.add` / `.del`；context 无底色）。
func diffCellColors(t *pluginapi.Theme, kind string) (bg, gutter ui.Color) {
	switch kind {
	case "add":
		return t.FileDiffAddedBg, t.FileDiffAddedGutter
	case "del":
		return t.FileDiffDeletedBg, t.FileDiffDeletedGutter
	}
	return ui.Color{}, ui.Color{}
}

// paintSplitCell 画一格：行号（右对齐）+ 折行后的正文。
func (p *Plugin) paintSplitCell(canvas *gfx.Canvas, t *pluginapi.Theme, numberW float32, cell changesDiffPaintCell, x, y float32) {
	numberColor := t.LabelTertiary
	textColor := t.LabelSecondary
	switch cell.kind {
	case "add":
		numberColor, textColor = t.FileDiffAddedMarker, t.FileDiffAddedMarker
	case "del":
		numberColor, textColor = t.FileDiffDeletedMarker, t.FileDiffDeletedMarker
	}
	if cell.no != nil {
		canvas.LabelAligned(itoa(*cell.no), gfx.Rect{X: x, Y: y, Width: numberW - changesNumberPadR, Height: changesDiffRowH},
			gfx.TextStyle{Size: t.Fonts.Code, Mono: true, Color: numberColor}, gfx.AlignRight)
	}
	style := gfx.TextStyle{Size: t.Fonts.Code, Mono: true, Color: textColor}
	for i, line := range cell.lines {
		canvas.Label(line, gfx.Rect{X: x + numberW + changesTextPadL, Y: y + float32(i)*changesDiffRowH, Width: cell.width, Height: changesDiffRowH}, style)
	}
}

// paintSplitColumn 画一列：BeginScrollArea 提供「自己的 bounds + 自己的 offset」，
// 横向滚动条由 `HorizontalScrollbar` 画（不手画）。
func (p *Plugin) paintSplitColumn(canvas *gfx.Canvas, env *pluginapi.Env, file apiclient.ForgeDiffFile, side string, bounds gfx.Rect, layout *changesDiffLayout, targets map[gfx.ID]diffScrollTarget, numberW float32) {
	contentWidth := layout.leftWidth
	if side == changesDiffSideRight {
		contentWidth = layout.rightWidth
	}
	scroll := p.open.diffScroll(fileSession(env), file.Path)
	state := scroll.side(side)
	area := components.BeginScrollArea(canvas, p.vivaThemeOf(env), bounds, gfx.Size{Width: contentWidth, Height: bounds.Height}).
		Offset(state.offset, 0).
		Viewport(diffViewportID(env, file.Path, side), "Diff lines").
		HorizontalScrollbar(diffScrollbarID(env, file.Path, side), "Horizontal scrollbar", "")
	geometry := area.BeginContent()
	state.offset = geometry.OffsetX
	state.max = geometry.MaxOffsetX

	t := p.themeOf(env)
	y := bounds.Y
	for _, row := range layout.rows {
		if row.kind == "pair" {
			cell := row.left
			if side == changesDiffSideRight {
				cell = row.right
			}
			if cell != nil {
				p.paintSplitCell(canvas, t, numberW, *cell, geometry.Content.X, y)
			}
		}
		y += row.height
	}
	area.EndContent()
	p.dragDiffScrollbar(canvas, geometry, diffScrollbarID(env, file.Path, side), state)

	viewportID := diffViewportID(env, file.Path, side)
	if !viewportID.IsZero() {
		targets[viewportID] = diffScrollTarget{session: fileSession(env), path: file.Path, side: side}
	}
}

// dragDiffScrollbar 拖动横向滚动条：按下抓拇指、移动按轨道映射改 offset、松开结束。
// 与 viva 自己的 `frame.dragScrollbar`（ui/scrollbars.go:46-78）同一套算法。
func (p *Plugin) dragDiffScrollbar(canvas *gfx.Canvas, geometry components.ScrollAreaGeometry, barID gfx.ID, state *diffSideScroll) {
	if barID.IsZero() {
		return
	}
	event, ok := canvas.PointerOn(barID)
	if !ok {
		return
	}
	thumb := geometry.HorizontalThumb
	switch {
	case event.Kind == gfx.PointerDown && event.Button <= gfx.PointerButtonPrimary:
		state.dragging = true
		state.grab = thumb.Width / 2
		if event.X >= thumb.X && event.X <= thumb.X+thumb.Width {
			state.grab = event.X - thumb.X
		}
		canvas.Consume()
	case event.Kind == gfx.PointerMove && state.dragging:
		state.offset = geometry.HorizontalOffsetForThumb(event.X - state.grab)
		canvas.Consume()
	case event.Kind == gfx.PointerUp && state.dragging:
		state.dragging = false
		canvas.Consume()
	}
}

// HandleScroll 让左右两列各自吃掉自己视口上的横向手势；竖向手势留给外层
// 文件列表滚动（pluginapi.ScrollHandler 的约定：Handled=false 继续走 View.Wheel）。
func (p *Plugin) HandleScroll(event gfx.ScrollEvent) gfx.EventResult {
	if event.Target.IsZero() || p.diffTargets == nil {
		return gfx.EventResult{}
	}
	target, ok := p.diffTargets[event.Target]
	if !ok {
		return gfx.EventResult{}
	}
	delta := event.StepX(32)
	if delta == 0 && event.Modifiers&gfx.ModifierShift != 0 {
		delta = event.StepY(32)
	}
	if delta == 0 {
		return gfx.EventResult{}
	}
	scroll := p.open.diffScroll(target.session, target.path)
	state := scroll.side(target.side)
	state.offset = clampFloat(state.offset+delta, 0, state.max)
	return gfx.EventResult{Handled: true, Repaint: true}
}

// diffSideScroll 是一列自己的滚动状态（offset 由调用方持有）。
type diffSideScroll struct {
	offset   float32
	max      float32
	dragging bool
	grab     float32
}

// changesDiffScroll 是一份文件左右两列的滚动状态。
type changesDiffScroll struct {
	left  diffSideScroll
	right diffSideScroll
}

func (s *changesDiffScroll) side(name string) *diffSideScroll {
	if name == changesDiffSideRight {
		return &s.right
	}
	return &s.left
}

// intersectRects 求两个矩形的交（任一为空返回零值）。
func intersectRects(a, b gfx.Rect) gfx.Rect {
	left := maxFloat32(a.X, b.X)
	top := maxFloat32(a.Y, b.Y)
	right := minFloat32(a.X+a.Width, b.X+b.Width)
	bottom := minFloat32(a.Y+a.Height, b.Y+b.Height)
	if right <= left || bottom <= top {
		return gfx.Rect{}
	}
	return gfx.Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func minFloat32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func clampFloat(v, lo, hi float32) float32 {
	if hi < lo {
		hi = lo
	}
	if v > hi {
		return hi
	}
	if v < lo {
		return lo
	}
	return v
}

func nearlyEqual(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 0.5
}
