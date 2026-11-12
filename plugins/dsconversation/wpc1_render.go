package dsconversation

import (
	"strconv"

	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	pubapi "github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

type messageCode struct {
	block    mdlite.CodeBlock
	height   float32
	geometry mdlite.CodeBlockGeometry
}

func (p *Plugin) codeStyle() mdlite.CodeBlockStyle {
	style := mdlite.DefaultCodeBlockStyle(dscore.VivaTheme(p.theme))
	style.Look = p.markdownLook()
	style.BannerFill = p.theme.BgLayer1
	style.ButtonFill = p.theme.Hover
	style.Success = p.theme.Success
	style.Failure = p.theme.Error
	style.FontSize = p.theme.Fonts.Caption
	return style
}

func (p *Plugin) rebuildCodeBlocks(canvas *ui.Canvas, index int, width float32) {
	sources := p.views[index].Codes()
	if len(sources) == 0 {
		delete(p.codeBlocks, index)
		return
	}
	blocks := make([]markdown.Block, 0, len(sources))
	for _, block := range p.docs[index].Blocks {
		if block.Kind == markdown.Code {
			blocks = append(blocks, block)
		}
	}
	old := p.codeBlocks[index]
	out := make([]messageCode, len(sources))
	style := p.codeStyle()
	for i, source := range sources {
		var block markdown.Block
		if i < len(blocks) {
			block = blocks[i]
		} else {
			block = markdown.Block{Kind: markdown.Code, Text: source.Text}
		}
		current := mdlite.NewCodeBlock(block, nil)
		if i < len(old) && old[i].block.Code == current.Code &&
			old[i].block.Language == current.Language && old[i].block.Title == current.Title {
			current = old[i].block
		}
		w := width
		if w < 80 {
			w = source.Bounds.Width
		}
		out[i] = messageCode{block: current, height: current.Height(canvas, w, style)}
	}
	p.codeBlocks[index] = out
}

func (p *Plugin) markdownExtraHeight(index int) float32 {
	var extra float32
	for i, code := range p.codeBlocks[index] {
		extra += code.height - p.views[index].Codes()[i].Bounds.Height
	}
	return extra
}

func (p *Plugin) HandleScroll(event ui.ScrollEvent) ui.EventResult {
	// 官方 transcript（`conversation.view` 座位）的代码块自己接管横滚：
	// 它只认本视口上的横向手势，纵向滚轮照旧落到 `m.view.Wheel`。
	if p.officialTranscript != nil {
		if result := p.officialTranscript.HandleScroll(event); result.Handled {
			return result
		}
	}
	for _, codes := range p.codeBlocks {
		for i := range codes {
			if result := codes[i].block.HandleScroll(event); result.Handled {
				return result
			}
		}
	}
	return ui.EventResult{}
}

func (p *Plugin) paintMarkdown(canvas *ui.Canvas, env *pluginapi.Env, slot mdSlot, origin ui.Rect, clip ui.Rect) {
	view := &p.views[slot.idx]
	links := p.linkOpen(env, slot.idx)
	var scope ui.Scope
	if links != nil && env.Tree != nil {
		scope = env.Tree.Feature("markdown-links-" + slot.key)
	}
	sources := view.Codes()
	codes := p.codeBlocks[slot.idx]
	if len(sources) != len(codes) {
		codes = nil
	}

	cursor, shift := float32(0), float32(0)
	for i, source := range sources {
		if codes == nil {
			break
		}
		p.paintMarkdownBand(canvas, view, slot, origin, cursor, source.Bounds.Y, shift, scope, clip)
		w := origin.Width
		if w < 80 {
			w = source.Bounds.Width
		}
		bounds := ui.Rect{
			X: origin.X + source.Bounds.X, Y: origin.Y + source.Bounds.Y + shift,
			Width: w, Height: codes[i].height,
		}
		copyText := env.Act.CopyResult
		if copyText == nil && env.Act.Copy != nil {
			copyText = func(text string) error { env.Act.Copy(text); return nil }
		}
		codes[i].block.Copy = copyText
		ids := mdlite.CodeBlockIDs{}
		if env.Tree != nil {
			base := slot.key + ":" + strconv.Itoa(i)
			ids = mdlite.CodeBlockIDs{
				Copy: env.Tree.ID(env.Feat, "code-copy-"+base), Wrap: env.Tree.ID(env.Feat, "code-wrap-"+base),
				Scroll: env.Tree.ID(env.Feat, "code-scroll-"+base), Scrollbar: env.Tree.ID(env.Feat, "code-scrollbar-"+base),
			}
		}
		codes[i].geometry = codes[i].block.Draw(canvas, bounds, ids, p.codeStyle())
		shift += codes[i].height - source.Bounds.Height
		cursor = source.Bounds.Y + source.Bounds.Height
	}
	p.paintMarkdownBand(canvas, view, slot, origin, cursor, view.Height(), shift, scope, clip)
	if !scope.IsZero() {
		p.dispatchMarkdownLink(canvas, env, slot.idx, view, scope, links)
	}
}

func (p *Plugin) paintMarkdownBand(canvas *ui.Canvas, view *components.MarkdownView, slot mdSlot, origin ui.Rect, from, to, shift float32, scope ui.Scope, clip ui.Rect) {
	if to <= from {
		return
	}
	band := ui.Rect{X: origin.X, Y: origin.Y + from + shift, Width: origin.Width, Height: to - from}
	band = intersectWPC1Rect(band, clip)
	if band.Width <= 0 || band.Height <= 0 {
		return
	}
	view.Paint(canvas, components.MarkdownPaint{
		Look: p.markdownLook(), X: origin.X, Y: origin.Y + shift,
		Clip: band, Links: scope, Base: 0,
	})
}

func intersectWPC1Rect(a, b ui.Rect) ui.Rect {
	if b.Width <= 0 || b.Height <= 0 {
		return a
	}
	x1, y1 := maxFloat32(a.X, b.X), maxFloat32(a.Y, b.Y)
	x2 := minFloat32(a.X+a.Width, b.X+b.Width)
	y2 := minFloat32(a.Y+a.Height, b.Y+b.Height)
	return ui.Rect{X: x1, Y: y1, Width: maxFloat32(0, x2-x1), Height: maxFloat32(0, y2-y1)}
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

func (p *Plugin) linkOpen(env *pluginapi.Env, index int) mdlite.OpenLinkFunc {
	if env.Act.OpenExternal == nil && env.Act.OpenFile == nil {
		return nil
	}
	return func(target mdlite.LinkTarget) {
		if original, ok := p.fileLinks[index][target.Destination]; ok {
			target = mdlite.LinkTarget{Kind: mdlite.LinkKindFile, Destination: original}
		}
		if target.Kind == mdlite.LinkKindFile {
			if env.Act.OpenFile != nil {
				actionFailure(env, "打开文件", env.Act.OpenFile(target.Destination))
			}
			return
		}
		if env.Act.OpenExternal != nil {
			actionFailure(env, "打开链接", env.Act.OpenExternal(target.Destination))
		}
	}
}

func (p *Plugin) dispatchMarkdownLink(canvas *ui.Canvas, env *pluginapi.Env, index int, view *components.MarkdownView, scope ui.Scope, open mdlite.OpenLinkFunc) {
	clicked, ok := canvas.ClickedIn(scope)
	if !ok || clicked < 0 || clicked >= view.LinkCount() {
		return
	}
	target, ok := mdlite.LinkTargetForURL(view.Link(clicked))
	if !ok {
		return
	}
	canvas.AfterPaint(func() { open(target) })
}

var _ pubapi.ScrollHandler = (*Plugin)(nil)
