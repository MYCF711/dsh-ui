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
// 本包用的是 **inset** 语义，与 outline 不同：
//
//	Card.module.css:6 `.details:focus-visible{outline:none;
//	  box-shadow:inset 0 0 0 2px var(--dsw-focus-ring-color,…)}`
//
// `box-shadow:inset 0 0 0 2px` 的环带落在盒子**内缘 0..2px**；viva 的
// `RoundedOutline` 描边以路径为中心，所以路径要内缩 width/2 = 1px，
// 环带才落在 0..2 而不是 1..3（`Look.Ring` 画在外侧，形状不对，不能用）。
package dspluginreco

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// recoFocusRingWidth 是 rc.2 `focus.css` 的 `--dsw-focus-ring-width:2px`；
// `.details:focus-visible` 的 inset 盒阴影用同一宽度。
const recoFocusRingWidth float32 = 2

// drawRecoDetailsFocusRing 画 `.details:focus-visible` 的 2px 内描边。
func drawRecoDetailsFocusRing(d *ui.DrawContext, radius float32, color gfx.Color) {
	if !d.State.FocusVisible {
		return
	}
	inset := recoFocusRingWidth / 2
	rect := d.Rect
	rect.X += inset
	rect.Y += inset
	rect.Width -= 2 * inset
	rect.Height -= 2 * inset
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	d.Canvas.RoundedOutline(rect, max(radius-inset, 0), recoFocusRingWidth, d.Fade(color))
}
