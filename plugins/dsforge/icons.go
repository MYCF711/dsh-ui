// icons.go —— 本插件用到的官方图标（都走 `plugins/dsassets` 的路径编译器）。
//
// 官方用到的字形（逐条，行号按 rc.2 随包的 `src/client/*.tsx`）：
//
//	IconBranchOutlineRegular      创建进度卡的 `.glyph`（WorktreeProgress.tsx:156 创建卡 /
//	                              :221 恢复卡）与工作树芯片（WorktreeToggle.tsx:146），size={14}
//	IconChevronDownOutlineRegular base 菜单的箭头（WorktreeToggle.tsx:90）与本地卡的开合箭头
//	                              （LocalChangesCard.tsx:49），size={12}
//	IconCheckOutlineRegular       步骤完成的勾（WorktreeProgress.tsx:84），size={14}
//
// `branch` 那枚图标里有**两枚 `<circle r=1.56962>`**（官方素材
// `assets/official/icons/messages/branch.jsx`，sha256 `5090854a…`，viewBox 16）。
// 2026-11-19 起 `plugins/dsassets` 把 SVG 椭圆弧换算成三次贝塞尔（`plugins/dsassets/arc.go`，
// 每段径向误差 < 0.03% of r），`Branch` 素材已从 `Unsupported` 变成可画
// （`plugins/dsassets/registry.go` 的 `Branch`：24 条描边命令、viewBox 16、描边 1、
// 包围盒逐项等于官方 `cx±r / cy±r`），所以这里用 `dsassets.Build` 直接编译，
// **不再退回纯文字**。
//
// 另外两枚是登记表里的"图标内部的一笔"（`dsassets.ChevronDownOutline` /
// `CheckOutline` 两个函数，出处写在 `plugins/dsassets/paths.go`），这里只管按
// 官方 size 缩放。
package dsforge

import (
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
)

const (
	// 官方 `IconChevronDownOutlineRegular size={12}`（WorktreeToggle.tsx:87）。
	chevronSize float32 = 12
	// 官方 `IconCheckOutlineRegular size={14}`（LocalChangesCard / fileToggle 14px 档）。
	checkSize float32 = 14
	// 官方 `IconListPenOutlineRegular size={15}`（LocalChangesCard.tsx:60）。
	listPenSize float32 = 15
)

// chevronDownIcon 是官方 `IconChevronDownOutlineRegular`（描边 1、viewBox 16）。
func chevronDownIcon() (gfx.Icon, bool) {
	commands, ok := dsassets.ChevronDownOutline()
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}, true
}

// branchIcon 是官方 `IconBranchOutlineRegular`（viewBox 16、描边 1、纯描边无填充）。
//
// 素材 `Branch` 的两条支线是 `<path>`、两枚端点是 `<circle r=1.56962>`；
// `dsassets.Build` 把圆换算成四段三次贝塞尔（`plugins/dsassets/arc.go`），所以这枚
// 图标是**精确画**出来的，不是近似圆点（账本 #64 ④ 的原差异）。
//
// 颜色不在素材里：官方这几处都是 `stroke="currentColor"`，墨色由调用方 `.Color()` 给。
func branchIcon() (gfx.Icon, bool) {
	return dsassets.Build(dsassets.Branch, gfx.Color{})
}

// checkIcon 是官方 `IconCheckOutlineRegular`。
func checkIcon() (gfx.Icon, bool) {
	commands, ok := dsassets.CheckOutline()
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}, true
}

// chevronRightIcon 是官方 `IconChevronRightOutlineRegular`
// （创建进度卡的"更多详情"披露箭头，`WorktreeProgress.tsx:166` size={12}）。
// 素材已在 dsassets 登记（`chevron-right-outline`，viewBox 16）。
func chevronRightIcon() (gfx.Icon, bool) {
	return dsassets.PlanReviewChevron()
}

// closeIcon 是官方 `IconCloseOutlineRegular`
// （`WorktreeProgress.tsx:86` 失败步骤 / `:178` "取消"按钮，size={14}）。
// 同一路径已存于素材库（`assets/official/icons/settings/close-outline.jsx`）。
func closeIcon(fill gfx.Color) (gfx.Icon, bool) {
	return dsassets.ScheduleCloseOutline(fill)
}

// listPenIcon 复用官方 `IconListPenOutlineRegular` 的五条原始路径。
// rc.2 sprite 的 `list-pen-outline` 与仓内 `PlanModeOutline` 图元逐条相同
// （两条横线描边 + 页面/铅笔/下划线填充），因此直接复用，不另抄一份坐标。
func listPenIcon(color gfx.Color) (gfx.Icon, bool) {
	return dsassets.PlanModeOutline(color)
}
