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

// compareSplitIcon 是官方 `IconCompareSplitOutlineRegular`（视图开关里的
// "切换为左右对比"，ChangesTab.tsx:1221-1225）。
//
// 素材来自官方图标 sprite `"compare-split-outline"`：
// `dsh-web-frontend/dist/assets/index-ghOpC9uV.js`（rc.2 安装包内，
// sprite 定义在 byte 723552 附近；`size:16`、`stroke-width` 由调用方给，
// Regular 档 = 1）。两条 `<path>` 是两个圆角矩形，**描边不填充**。
func compareSplitIcon() (gfx.Icon, bool) {
	return dsassets.Build(dsassets.Asset{
		ID: "compare-split-outline", Title: "左右对比", ViewBox: 16, StrokeWidth: 1,
		Elements: []dsassets.Element{
			{Kind: dsassets.PathElement, D: `M6 1.5H2.5C1.94772 1.5 1.5 1.94772 1.5 2.5V13.5C1.5 14.0523 1.94772 14.5 2.5 14.5H6C6.55228 14.5 7 14.0523 7 13.5V2.5C7 1.94772 6.55228 1.5 6 1.5Z`, Stroke: true},
			{Kind: dsassets.PathElement, D: `M13.5 1.5H10C9.44772 1.5 9 1.94772 9 2.5V13.5C9 14.0523 9.44772 14.5 10 14.5H13.5C14.0523 14.5 14.5 14.0523 14.5 13.5V2.5C14.5 1.94772 14.0523 1.5 13.5 1.5Z`, Stroke: true},
		},
	}, gfx.Color{})
}

// nowrapFillIcon / wrapFillIcon 是官方 `IconNowrapFillRegular` / `IconWrapFillRegular`
// （ChangesTab.tsx:1227-1230：wrap 开着时画 nowrap，表示"点它关闭换行"）。
//
// 素材来自官方 sprite `"nowrap-fill"` / `"wrap-fill"`
// （`dsh-web-frontend/dist/assets/index-ghOpC9uV.js`，byte 722560 / 722991 附近；
// `size:16`，纯 `fill="currentColor"`，无描边）。颜色由调用方给。
func nowrapFillIcon(color gfx.Color) (gfx.Icon, bool) {
	return dsassets.Build(dsassets.Asset{
		ID: "nowrap-fill", Title: "关闭自动换行", ViewBox: 16,
		Elements: []dsassets.Element{
			{Kind: dsassets.PathElement, D: `M2 15H1V1H2V15Z`, Fill: true},
			{Kind: dsassets.PathElement, D: `M12.3535 7.64645C12.5487 7.84171 12.5487 8.15829 12.3535 8.35355L9.85352 10.8535L9.14648 10.1465L10.793 8.5H3.5V7.5H10.793L9.14648 5.85352L9.85352 5.14648L12.3535 7.64645Z`, Fill: true},
			{Kind: dsassets.PathElement, D: `M15 15H14V1H15V15Z`, Fill: true},
		},
	}, color)
}

func wrapFillIcon(color gfx.Color) (gfx.Icon, bool) {
	return dsassets.Build(dsassets.Asset{
		ID: "wrap-fill", Title: "开启自动换行", ViewBox: 16,
		Elements: []dsassets.Element{
			{Kind: dsassets.PathElement, D: `M10.9999 8C10.9999 6.89543 10.1046 6 9 6H4.5V5H9C10.6568 5 11.9999 6.34315 11.9999 8C11.9999 9.65685 10.6568 11 9 11H6.20703L6.85351 11.6465L6.14648 12.3535L4.64652 10.8536C4.45126 10.6583 4.45126 10.3417 4.64652 10.1464L6.14648 8.64648L6.85351 9.35352L6.20703 10H9C10.1046 10 10.9999 9.10457 10.9999 8Z`, Fill: true},
			{Kind: dsassets.PathElement, D: `M2 15H1V1H2V15Z`, Fill: true},
			{Kind: dsassets.PathElement, D: `M15 15H14V1H15V15Z`, Fill: true},
		},
	}, color)
}

// listPenIcon 复用官方 `IconListPenOutlineRegular` 的五条原始路径。
// rc.2 sprite 的 `list-pen-outline` 与仓内 `PlanModeOutline` 图元逐条相同
// （两条横线描边 + 页面/铅笔/下划线填充），因此直接复用，不另抄一份坐标。
func listPenIcon(color gfx.Color) (gfx.Icon, bool) {
	return dsassets.PlanModeOutline(color)
}

// chevronsUpDownIcon 是官方 `IconChevronsUpDownOutlineRegular`（未变更行 gap 条纹
// 与工具栏「全部展开/折叠」的图标，`OFF:src/client/DiffFile.tsx:413`、`:488`、
// `ChangesTab.tsx:1218`）。
//
// 素材来自官方图标 sprite `"chevrons-up-down-outline"`
// （`dsh-web-frontend/dist/assets/index-ghOpC9uV.js`，rc.2 安装包内，sprite 定义在
// byte 759571 附近：`size:16`、`fill="none"`、`stroke="currentColor"`、
// `stroke-linecap/linejoin="round"`、`stroke-width` 由调用方给，Regular 档 = 1）。
// 官方原文两条 `<path>` 是上下两个 V 形折线，**描边不填充**：
//
//	d="m5.1 6 2.9-2.9L10.9 6"        （相对 m + 隐式 lineto 对）
//	d="m5.1 10 2.9 2.9 2.9-2.9"
//
// `dsassets` 的路径解析器只吃显式命令（不吃 `m` 后面的隐式坐标对），所以这里写成
// **几何完全等价**的绝对形式：`M5.1 6 L8 3.1 L10.9 6` / `M5.1 10 L8 12.9 L10.9 10`。
// 端点逐个相同，不是近似。
var chevronsUpDownAsset = dsassets.Asset{
	ID: "chevrons-up-down-outline", Title: "展开未变更行", ViewBox: 16, StrokeWidth: 1,
	Elements: []dsassets.Element{
		{Kind: dsassets.PathElement, D: `M5.1 6 L8 3.1 L10.9 6`, Stroke: true,
			Cap: gfx.CapRound, CapSet: true, Join: gfx.JoinRound, JoinSet: true},
		{Kind: dsassets.PathElement, D: `M5.1 10 L8 12.9 L10.9 10`, Stroke: true,
			Cap: gfx.CapRound, CapSet: true, Join: gfx.JoinRound, JoinSet: true},
	},
}

func chevronsUpDownIcon() (gfx.Icon, bool) {
	return dsassets.Build(chevronsUpDownAsset, gfx.Color{})
}
