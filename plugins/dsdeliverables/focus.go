// focus.go —— 键盘焦点环的**局部**实现（不跨包 import，按仓里既有惯例每个包自带一份）。
//
// 官方 rc.2 的焦点环是**全局**规则，不是逐组件写死的：
//
//	focus.css：
//	  :root{--dsw-focus-ring-width:2px}
//	  :focus-visible{outline-color:var(--dsw-focus-ring-color,
//	                 var(--dsw-alias-state-business-primary));
//	                 outline-width:var(--dsw-focus-ring-width)}
//	  html[data-input-modality=pointer] body :focus-visible:not(:read-write){
//	                 --dsw-focus-ring-color:transparent;outline-color:#0000}
//
// `--dsw-alias-state-business-primary` 在 rc.2 取 `--dsw-static-deepseek-400`（dark）/
// `-500`（light），就是主题里的 `Info`（plugins/dscore/dscore.go:217,319）。
//
// # 两种几何别混
//
//   - **inset**（`box-shadow:inset 0 0 0 2px`，本包的 `.cardPreview:focus-visible`）：
//     环带落在盒子**内缘 0..2px**。
//   - **outline**（`outline:Npx` + `outline-offset:O`）：环带在盒子**外侧** O..O+N。
//
// `RoundedOutline` 的描边以路径为中心，所以两种都要先把路径挪到环带中线。
package dsdeliverables

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// cardFocusRingWidth 是 rc.2 `focus.css` 的 `--dsw-focus-ring-width:2px`；
// `.cardPreview:focus-visible{box-shadow:inset 0 0 0 2px …}`（Deliverables.module.css）用同一宽度。
const cardFocusRingWidth float32 = 2

// drawCardFocusRing 画官方 `.cardPreview:focus-visible` 那条 **2px 内描边**。
//
// 路径取内缩 width/2 = 1px：2px 宽的描边以路径为中心，去掉内缘半宽后正好落在
// 盒子内缘 0..2px（与 `box-shadow:inset 0 0 0 2px` 同形）。
func drawCardFocusRing(d *ui.DrawContext, radius float32, color gfx.Color) {
	if !d.State.FocusVisible {
		return
	}
	inset := cardFocusRingWidth / 2
	rect := d.Rect
	rect.X += inset
	rect.Y += inset
	rect.Width -= 2 * inset
	rect.Height -= 2 * inset
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	d.Canvas.RoundedOutline(rect, max(radius-inset, 0), cardFocusRingWidth, d.Fade(color))
}
