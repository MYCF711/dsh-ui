package dsterminal

import (
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
)

// terminalTitleIcon 复用官方 TerminalIcon.tsx:9 的两条原始 path。
// viewBox="0 0 16 16"、width/height=16、stroke="currentColor"，Regular 描边为 1。
// 原文件 SHA-256：62e47b45856a202c4b99cb2e3af7c19e650dc8d30eca4d498edaf083ef391ea3。
func terminalTitleIcon() gfx.Icon {
	icon, ok := dsassets.Build(dsassets.Asset{
		ID: "terminal-title",
		Source: dsassets.Source{
			Package: "@deepseek-ai/dsh-client-ui-sidebar-terminal",
			File:    "src/client/TerminalIcon.tsx",
			Symbol:  "TerminalIcon",
			SHA256:  "62e47b45856a202c4b99cb2e3af7c19e650dc8d30eca4d498edaf083ef391ea3",
		},
		ViewBox: 16, Size: terminalTabIconSize, StrokeWidth: 1,
		Elements: []dsassets.Element{
			{Kind: dsassets.PathElement, D: "M3 4L7 8L3 12", Stroke: true},
			{Kind: dsassets.PathElement, D: "M9 12H13", Stroke: true},
		},
	}, gfx.Color{})
	if !ok {
		return gfx.Icon{}
	}
	return icon
}

// terminalPlusIcon 直接复用 dsassets 已登记的 IconPlusOutlineRegular 原路径，
// 与 terminal.tsx:5,31 的新建终端按钮相同，不复制第二份坐标。
func terminalPlusIcon() (gfx.Icon, bool) {
	return dsassets.MarketPlusOutline(gfx.Color{})
}

// terminalChevronDownIcon 复用共享素材包的 IconChevronDownOutlineRegular
// （官方向导卡 `.trigger svg` 那枚 14px 箭头，TerminalGuide.module.css:10）。
func terminalChevronDownIcon() gfx.Icon {
	commands, ok := dsassets.ChevronDownOutline()
	if !ok {
		return gfx.Icon{}
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}
}

// terminalCheckIcon 复用共享素材包的 IconCheckOutlineRegular：官方 Menu 用它在
// `selectedId` 那一行前面打勾（TerminalGuide.tsx:65 的 `selectedId`）。
func terminalCheckIcon() gfx.Icon {
	commands, ok := dsassets.CheckOutline()
	if !ok {
		return gfx.Icon{}
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}
}
