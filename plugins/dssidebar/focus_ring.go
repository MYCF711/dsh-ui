package dssidebar

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方 rc.2 的键盘焦点环是**全局**规则，不是逐组件写死的：
//
//	focus.css：
//	  :root{--dsw-focus-ring-width:2px}
//	  :focus-visible{outline-color:var(--dsw-focus-ring-color,
//	                 var(--dsw-alias-state-business-primary));
//	                 outline-width:var(--dsw-focus-ring-width)}
//	  html[data-input-modality=pointer] body :focus-visible:not(:read-write){
//	                 --dsw-focus-ring-color:transparent;outline-color:#0000}
//
// 即：**键盘**焦点一律 2px、business-primary；指针点出来的焦点不画环。
// `--dsw-alias-state-business-primary` 在 rc.2 取 `--dsw-static-deepseek-400/500`，
// 就是主题里的 Info（plugins/dscore/dscore.go:217,319）。
//
// 侧栏只有两处额外改 offset（rc.2 `128__SidebarRoot`）：
//
//	.panelRow:focus-visible{outline: ...;outline-offset:-2px}
//	.iconButton.toggle:focus-visible{outline: ...;outline-offset:-2px}
const (
	sidebarFocusRingWidth float32 = 2  // rc.2 focus.css `--dsw-focus-ring-width:2px`
	panelRowFocusOffset   float32 = -2 // rc.2 SidebarRoot.module.css `.panelRow:focus-visible{outline-offset:-2px}`
)

// drawInsetFocusRing 画 CSS `outline + outline-offset:<负值>` 的等效环。
//
// CSS 的 outline 从 offset 边向**外**画 width 宽：offset:-2 / width:2 的环带
// 正好贴住盒子内缘 0..2px。RoundedOutline 的描边以路径为中心，所以路径取
// 内缩 `-offset - width/2` = 1px，环带才落在 0..2 而不是 1..3。
func drawInsetFocusRing(d *ui.DrawContext, offset float32, radius float32, color gfx.Color) {
	if !d.State.FocusVisible {
		return
	}
	inset := -offset - sidebarFocusRingWidth/2
	rect := d.Rect
	rect.X += inset
	rect.Y += inset
	rect.Width -= 2 * inset
	rect.Height -= 2 * inset
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	d.Canvas.RoundedOutline(rect, max(radius-inset, 0), sidebarFocusRingWidth, d.Fade(color))
}
