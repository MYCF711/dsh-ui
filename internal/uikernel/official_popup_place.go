package uikernel

import "github.com/vivaui/viva/gfx"

// official_popup_place.go —— 浮层"贴锚点上方、放不下翻到下方"的**唯一判定**。
//
// # 为什么需要它
//
// v0.5.1 的 `flex.Popup().Above(anchor, gap, align)` 自带翻面语义：放不下就往
// 另一面翻（`ui/popup.go` 的 `sideOf`：请求的那一侧放得下就用它，否则去"另一侧
// 空间更大"的那边）。v0.6.0 删掉了 `frame.Popup()`，应用的自由浮层改写成
// `ui.Overlay(...)` + 绝对定位子面板——**Absolute 不翻面**，面板会顶出窗口上沿
// （顶出去的那部分既看不见也点不着）。
//
// 两个站点因此需要同一份等价行为：
//
//	internal/uikernel/official_model_menu.go      模型菜单（Above + End 对齐）
//	plugins/dsstatusbar/official_stats_dialog.go  Activity/Usage 弹窗（Above + Start 对齐）
//
// # 只判"哪一侧"，不判坐标
//
// 这一层只回答"翻不翻"。**坐标由调用方按边钉**：
//
//	上方（PopupAbove）→ `.Bottom(bounds.Height - anchor.Y + gap)`（底边贴锚点顶）
//	下方（PopupBelow）→ `.Top(anchor.Y + anchor.Height + gap)`（顶边贴锚点底）
//
// 为什么不让它直接算 top：v0.6.0 的 `Absolute()` 子元素高度由**内容**决定，
// 声明期拿不到。钉 `Bottom` 时内容往下长（正确），钉 `Top` 时内容往下长（也正确）；
// 而拿一个估出来的 `top` 去钉，首帧（高度未知=0）就会让面板从锚点顶开始往下长、
// 压住 trigger。**用边钉，不用坐标钉** —— 这样"没量过高度"也不会有错帧。
//
// # 高度为什么要"上一帧的"
//
// 翻面判定要知道面板有多高，而高度声明期拿不到，所以调用方传上一帧量到的高度；
// 0 = 还没量过，按"放得下"处理——与 v0.5.1 首帧行为一致。
//
// # 边界（写清，别当成"永远不会发生"）
//
// 两处锚点在**正常产品形态**下都贴窗口底部（composer 的模型 chip / 状态栏的
// 活动胶囊），所以"上方放不下"只在**很矮的窗口**时才发生。翻面仍要实现对，
// 因为窗口可以被拖到任意高度。

// PopupSide 是浮层最后落在锚点的哪一侧。
type PopupSide int

const (
	// PopupAbove 表示浮层在锚点上方（调用方钉 `Bottom(bounds.Height-anchor.Y+gap)`）。
	PopupAbove PopupSide = iota
	// PopupBelow 表示浮层翻到了锚点下方（调用方钉 `Top(anchor.Y+anchor.Height+gap)`）。
	PopupBelow
)

// String 让失败信息可读（`%v` 打出 "above"/"below" 而不是数字）。
func (s PopupSide) String() string {
	if s == PopupBelow {
		return "below"
	}
	return "above"
}

// PlaceAbove 判定一个"贴在锚点上方 gap 处"的浮层该落在哪一侧：
// 请求的那一侧（上方）放得下就用它；放不下时去"另一侧空间更大"的那边；
// 两侧一样宽时留在请求的那侧（稳定，不做无谓抖动）。
//
//	anchor       trigger / 胶囊的**窗口坐标**矩形
//	height       浮层的已排版高度（0 = 还没量过 → 按放得下处理）
//	gap          浮层与锚点之间的间距
//	boundsHeight 整窗高度（浮层被夹在 [inset, boundsHeight-inset] 内）
//	inset        浮层与窗口上下沿的最小间距（官方 12px）
func PlaceAbove(anchor gfx.Rect, height, gap, boundsHeight, inset float32) PopupSide {
	lo, hi := inset, boundsHeight-inset
	// 上方可用高度（从窗口上沿的 inset 到锚点顶再减间距）。
	roomAbove := anchor.Y - gap - lo
	// 下方可用高度（从锚点底加间距到窗口下沿的 inset）。
	roomBelow := hi - (anchor.Y + anchor.Height + gap)

	// 与 v0.5.1 sideOf 的判据逐字一致。
	if height <= roomAbove || roomBelow <= roomAbove {
		return PopupAbove
	}
	return PopupBelow
}
