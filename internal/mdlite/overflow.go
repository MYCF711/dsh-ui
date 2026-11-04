// Package mdlite 是 U1 用的轻量组件补齐层。
//
// Viva 的 `flex` 只做纵向滚动（`flex/scroll.go` 里没有横向 API，`Box` 也没有
// Offset/Translate），所以代码块、diff、长命令行在窄窗口下会被右边缘硬切。
// `components` 层则有完整的二维几何，`Overflow` 就是对 `components.ScrollArea`
// 的一层薄封装：**在 flex 占好位的矩形里，自己接管 Clip / 平移 / 滚动条**。
//
// 用法（和 components.MarkdownView 一样是「占位 + Draw 之后画」）：
//
//	slot := f.Slot().Height(150)          // flex 里占位
//	// Draw 返回之后，布局已定：
//	o.Draw(canvas, theme, slot.Rect(), contentW, contentH, id, "", func(c ui.Rect) {
//		// 从 c.X / c.Y 开始画内容；c 已经按 OffsetX 平移过
//	})
package mdlite

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

// Overflow 是一个横向滚动容器的状态：调用方持有，随内容一起活在 model 里。
type Overflow struct {
	// OffsetX 是当前横向位移，由 Draw 按实际上限夹紧后回写。
	OffsetX float32
}

// Wheel 处理一次滚轮或触控板的横向位移。dy 为正向的竖向滚动会被忽略
// （横向容器不抢竖向手势，那是外层 ScrollArea 的事）。
func (o *Overflow) Wheel(dx float32) {
	o.OffsetX = maxFloat(0, o.OffsetX+dx)
}

// Draw 把一个可横向滚动的内容画进 bounds。
//
//   - 内层：Clip 到视口，回调拿到的 rect 已经按 OffsetX 平移；
//   - 外层：画横向滚动条（内容比视口高时也画纵向的）。
//
// 返回完整几何，调用方可以用它做命中测试（例如拖 thumb）。
func (o *Overflow) Draw(
	canvas *ui.Canvas,
	theme ui.Theme,
	bounds ui.Rect,
	contentWidth, contentHeight float32,
	id ui.ID,
	label string,
	draw func(content ui.Rect),
) components.ScrollAreaGeometry {
	return o.DrawInteractive(canvas, theme, bounds, contentWidth, contentHeight, ui.ID{}, id, label, draw)
}

// DrawInteractive 比 Draw 多一个可聚焦视口 ID。宿主把这个 ID 对应的
// ui.ScrollEvent 转发给组件（例如 CodeBlock.HandleScroll），即可由真实
// hit-test 路由横向手势；没有视口 ID 时行为与旧 Draw 完全一致。
func (o *Overflow) DrawInteractive(
	canvas *ui.Canvas,
	theme ui.Theme,
	bounds ui.Rect,
	contentWidth, contentHeight float32,
	viewportID, scrollbarID ui.ID,
	label string,
	draw func(content ui.Rect),
) components.ScrollAreaGeometry {
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return components.ScrollAreaGeometry{Bounds: bounds}
	}
	area := components.BeginScrollArea(canvas, theme, bounds, ui.Size{
		Width:  contentWidth,
		Height: contentHeight,
	}).
		Offset(o.OffsetX, 0).
		Viewport(viewportID, label).
		HorizontalScrollbar(scrollbarID, label, "").
		ReserveScrollbars()

	geo := area.BeginContent()
	o.OffsetX = geo.OffsetX // 回写夹紧后的值，免得滚过头再画时位置跳
	if draw != nil {
		draw(geo.Content)
	}
	return area.EndContent()
}

// MaxOffset 返回当前内容宽度下允许的最大横向位移；内容不溢出时是 0。
func MaxOffset(bounds ui.Rect, contentWidth float32) float32 {
	geo := components.BeginScrollArea(nil, ui.Theme{}, bounds, ui.Size{Width: contentWidth}).
		ReserveScrollbars().
		Geometry()
	return geo.MaxOffsetX
}

func maxFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
