// icons.go —— 本插件用到的两枚官方图标（都走 `plugins/dsassets` 的路径编译器）。
//
// 官方 A 档用到的字形（逐条）：
//
//	IconBranchOutlineRegular   WorktreeToggle.current / 分支芯片（WorktreeToggle.tsx:118）
//	IconChevronDownOutlineRegular  base 按钮的箭头（WorktreeToggle.tsx:87）与本地卡的开合箭头
//	IconCheckOutlineRegular    LocalChangesCard 的"已读"打勾（官方 fileToggle 用同一枚）
//
// `branch` **画不出来**：官方那枚图标含两枚 `<circle r=1.56962>`，而 viva 的路径词汇
// 只有 M/L/Q/C/Z（弧要椭圆弧）。素材库已把它登记成 `Unsupported`
// （`plugins/dsassets/registry.go:174-186`），所以这里**不画**分支图标，分支名用
// 代码体文字表达——画一个近似圆点顶替官方的官方图形，比不画更糟。
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
)

// chevronDownIcon 是官方 `IconChevronDownOutlineRegular`（描边 1、viewBox 16）。
func chevronDownIcon() (gfx.Icon, bool) {
	commands, ok := dsassets.ChevronDownOutline()
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}, true
}

// checkIcon 是官方 `IconCheckOutlineRegular`。
func checkIcon() (gfx.Icon, bool) {
	commands, ok := dsassets.CheckOutline()
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}, true
}
