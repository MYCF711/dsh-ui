package dsplan

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// PlanPreview.module.css:26,36：卡片/审阅链接 outline-offset:3px。
const planCardFocusOffset float32 = 3

// PlanModeControl.module.css:36-39：chip outline-offset:2px。
const planChipFocusOffset float32 = 2

// drawPlanFocusRing 按 CSS outline 的宽度与 offset 画键盘焦点环。
// Viva Ring 只能贴边，不能表达官方 offset，所以这里使用布局后的 Rect。
func drawPlanFocusRing(d *ui.DrawContext, offset, radius float32, color gfx.Color) {
	if !d.State.FocusVisible {
		return
	}
	rect := d.Rect
	rect.X -= offset
	rect.Y -= offset
	rect.Width += 2 * offset
	rect.Height += 2 * offset
	d.Canvas.RoundedOutline(rect, radius+offset, planFocusRing, d.Fade(color))
}
