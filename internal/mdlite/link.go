package mdlite

import (
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

// LinkKind 是调用方明确给出的链接语义。组件不会把未知字符串猜成宿主文件路径。
type LinkKind uint8

const (
	LinkKindHTTP LinkKind = iota + 1
	LinkKindHTTPS
	LinkKindMailto
	LinkKindFile
)

// LinkTarget 原样携带目的地。文件目标可含 #L24-L30，但解析/授权/打开全归调用方。
type LinkTarget struct {
	Kind        LinkKind
	Destination string
}

// OpenLinkFunc 是宿主注入的打开动作；组件不执行 OS 命令、不解析宿主根目录。
type OpenLinkFunc func(target LinkTarget)

// Link 是一个独立、可复用的文本链接。文件链接必须显式使用 LinkKindFile；
// 这样相对路径只是回调数据，不会被组件擅自映射到某个工作区。
type Link struct {
	Text   string
	Target LinkTarget
	Open   OpenLinkFunc
}

// NewFileLink 构造显式文件链接，Destination（含行号片段）原样交给 open。
func NewFileLink(text, destination string, open OpenLinkFunc) Link {
	return Link{Text: text, Target: LinkTarget{Kind: LinkKindFile, Destination: destination}, Open: open}
}

// LinkTargetForURL 只识别明确 scheme，不把任意字符串猜成文件。
func LinkTargetForURL(destination string) (LinkTarget, bool) {
	colon := strings.IndexByte(destination, ':')
	if colon <= 0 {
		return LinkTarget{}, false
	}
	kind := LinkKind(0)
	switch strings.ToLower(destination[:colon]) {
	case "http":
		kind = LinkKindHTTP
	case "https":
		kind = LinkKindHTTPS
	case "mailto":
		kind = LinkKindMailto
	default:
		return LinkTarget{}, false
	}
	return LinkTarget{Kind: kind, Destination: destination}, true
}

// Draw 画文本链接并返回真实可见 hit 矩形。区域宽度按文字测量并受 bounds
// 裁剪，不给整行空白处制造“幽灵链接”。点击通过 AfterPaint 调宿主回调。
func (l Link) Draw(canvas *ui.Canvas, bounds ui.Rect, id ui.ID, style ui.TextStyle) ui.Rect {
	if bounds.Width <= 0 || bounds.Height <= 0 || l.Text == "" {
		return ui.Rect{}
	}
	if style.Size <= 0 {
		style.Size = 13
	}
	width := minFloat(canvas.TextWidthWeight(l.Text, style.Size, style.Weight), bounds.Width)
	region := ui.Rect{X: bounds.X, Y: bounds.Y, Width: width, Height: bounds.Height}
	canvas.Clip(bounds)
	defer canvas.Unclip()
	textBounds := region
	canvas.Label(l.Text, textBounds, style)
	if !id.IsZero() {
		canvas.HitWithOptions(id, region, l.Text, ui.HitOptions{
			Role: ui.RoleLink, Cursor: ui.PointerCursorHand,
			Value: l.Target.Destination, Disabled: l.Open == nil,
		})
	}
	if canvas.Hovered() == id && l.Open != nil {
		canvas.Fill(ui.Rect{X: region.X, Y: region.Y + region.Height - 2, Width: region.Width, Height: 1}, style.Color)
	}
	if canvas.Clicked(id) && l.Open != nil {
		target, open := l.Target, l.Open
		canvas.AfterPaint(func() { open(target) })
	}
	return region
}

// MarkdownLinkActions 给现有 components.MarkdownView 补上 Viva 原生链接区域
// 和回调分发；它不解析文档、不接管流式 Markdown 入口。多个 view 可共享
// Scope，调用方用 Base 传入此前所有 view 的 LinkCount 总和。
type MarkdownLinkActions struct {
	Scope ui.Scope
	Base  int
	Open  OpenLinkFunc
}

// Paint 实画 Markdown 并把点击映射回同一 fragment 的 Destination。
// Viva parser 本身只生成 HTTP(S)/mailto 节点；文件链接由调用方用 Link/
// NewFileLink 显式实画，避免从普通文本猜宿主路径。
func (a MarkdownLinkActions) Paint(canvas *ui.Canvas, view *components.MarkdownView, paint components.MarkdownPaint) {
	if view == nil {
		return
	}
	if a.Open == nil || a.Scope.IsZero() {
		paint.Links = ui.Scope(0)
		view.Paint(canvas, paint)
		return
	}
	paint.Links = a.Scope
	paint.Base = a.Base
	view.Paint(canvas, paint)
	clicked, ok := canvas.ClickedIn(a.Scope)
	if !ok {
		return
	}
	index := clicked - a.Base
	if index < 0 || index >= view.LinkCount() {
		return
	}
	target, ok := LinkTargetForURL(view.Link(index))
	if !ok {
		return
	}
	open := a.Open
	canvas.AfterPaint(func() { open(target) })
}
