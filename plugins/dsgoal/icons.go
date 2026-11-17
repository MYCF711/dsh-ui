// icons.go —— 目标条 glyph 的唯一来源是 dsassets 中的官方 sprite 原件。
//
// 官方 GoalBar.tsx:137 使用 <IconGoalOutlineRegular size={14} />；
// assets/official/icons/tools/goal.jsx:1-4 保存四条原始 path。
// dsassets.GoalOutline 负责无损解析，本包不再逐点转写坐标。
package dsgoal

import (
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
)

// goalGlyphSize 是官方 GoalBar.tsx:137 的 size={14}；viewBox 仍为 16。
const goalGlyphSize float32 = 14

func goalOutlineIcon() gfx.Icon {
	icon, ok := dsassets.GoalOutline()
	if !ok {
		// 官方原件全是 M/L/C/Z，正常不会走到这里；返回空图标也不自造替代形状。
		return gfx.Icon{}
	}
	return icon
}
