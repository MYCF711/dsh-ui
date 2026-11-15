package dsentrycustom

// icons.go —— #17 三行菜单用到的图标（**不是手画的**，逐字节来自 rc.2 sprite）。
//
// # 取法
//
// 官方 primitives 的图标不是组件源码，而是运行时从 sprite 表生成 `<svg>`：
//
//	dist: /dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-ghOpC9uV.js
//	      整文件 1093231 B，sha256 061de8cefa15e1fbb87d301463ad4ebb79259b197ca08e83406f6997c1af8bb8
//	权重表 regular = 1（`p7={regular:1,...}`）→ `<svg stroke-width="1">`
//	sprite 没写 stroke-linecap / stroke-linejoin 的符号，Go 侧 Cap/Join 必须留零值
//	（与 `docs/official-icon-sprite-register.md` §0 同一条规矩）
//
// # 三枚符号
//
//	pin-outline              OFF:EntryMenuContent.tsx:57  `<IconPinOutlineRegular size={14}/>`
//	                         行尾「固定」按钮。第一条 path 只描边（`stroke-linejoin="round"`），
//	                         第二条只填充——所以是"描边一条 + 一个填充层"的 icon。
//	plugin-pinwheel-outline  OFF:EntryMenuContent.tsx:62  `<IconPluginPinwheelOutlineRegular size={16}/>`
//	                         「浏览更多插件」行首。4 条纯描边曲线。
//	check-outline            OFF:EntryMenuContent.tsx:50  `<IconCheckOutlineRegular size={16}/>`
//	                         当前条目行尾的对勾。1 条纯描边。
//
// 显示尺寸：官方三处传 14 / 16 / 16，但 primitives 菜单行的 CSS 把图标框与 svg 都
// 夹到 **14×14**（`._itemIcon_70kcw_233{width:14px;height:14px}` +
// `._itemIcon svg,._check{width:14px;height:14px}`，dist CSS `index-CSAdOBx6.css`），
// 所以调用处一律 `.Size(14, 14)`——与官方**渲染出来的**像素一致，而不是与属性值一致。

import (
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
)

// pin-outline 的两条 `d` 原文（sprite，逐字节，未重排）。
const (
	pinOutlineHeadPath = `M9.96976 1.70572L13.1554 3.93629L10.9019 8.12317L11.5158 11.605L10.7192 12.7427L2.52767 7.00693L3.3243 5.86922L6.80612 5.25528L9.96976 1.70572Z`
	pinOutlineTailPath = `M6.05285 9.47511C6.27284 9.16094 6.70586 9.08458 7.02003 9.30457C7.3342 9.52455 7.41055 9.95757 7.19057 10.2717L3.98587 14.4708L3.21223 13.9291L6.05285 9.47511Z`
)

// plugin-pinwheel-outline 的四条 `d` 原文。
var pluginPinwheelPaths = []string{
	`M7.84457 5.06199C11.6605 4.93876 14.7962 6.14848 14.8484 7.76397C14.8875 8.97461 13.1838 10.0696 10.7215 10.5942`,
	`M5.12742 8.07731C5.00419 4.26138 6.21391 1.12568 7.8294 1.07351C9.04004 1.03441 10.135 2.73808 10.6596 5.20037`,
	`M8.02457 10.6802C4.20865 10.8034 1.07294 9.5937 1.02077 7.97821C0.981678 6.76758 2.68535 5.67262 5.14763 5.14798`,
	`M10.7476 7.89535C10.8708 11.7113 9.66109 14.847 8.0456 14.8991C6.83496 14.9382 5.74 13.2346 5.21536 10.7723`,
}

// check-outline 的那一条 `d` 原文。
const checkOutlinePath = `M2.25 8.5L5.49732 11.7473C5.90519 12.1552 6.57263 12.1344 6.95426 11.7018L13.75 4`

// pinOutlineIcon 造官方 `IconPinOutlineRegular`；color 是**填充层**的颜色
// （描边色由调用方 `.Color()` 给）。
//
// 返回 false = 路径解析失败。与 dsassets 的约定一致：调用方自己决定不画，
// 不许拿近似形状顶上。
func pinOutlineIcon(color gfx.Color) (gfx.Icon, bool) {
	head, ok := dsassets.ParsePath(pinOutlineHeadPath)
	if !ok {
		return gfx.Icon{}, false
	}
	tail, ok := dsassets.ParsePath(pinOutlineTailPath)
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{
		ViewBox:  16,
		Stroke:   1,
		Join:     gfx.JoinRound, // 官方这一条 path 带 `stroke-linejoin="round"`
		Commands: head,
		Layers:   []gfx.IconLayer{{Fill: color, Commands: tail}},
	}, true
}

// pluginPinwheelIcon 造官方 `IconPluginPinwheelOutlineRegular`（4 条纯描边曲线）。
func pluginPinwheelIcon() (gfx.Icon, bool) {
	var commands []gfx.PathCommand
	for _, d := range pluginPinwheelPaths {
		part, ok := dsassets.ParsePath(d)
		if !ok {
			return gfx.Icon{}, false
		}
		commands = append(commands, part...)
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}, true
}

// checkOutlineIcon 造官方 `IconCheckOutlineRegular`（1 条纯描边曲线）。
//
// 与 `dsstatusbar` 里那一枚是同一个符号的两份转写：图标在官方是 sprite 表，
// 哪个包用就哪个包转写（本仓既有的 pin-fill / ellipsis 也是这个做法），
// 免得为一个 14px 符号让业务包互相 import。
func checkOutlineIcon() (gfx.Icon, bool) {
	commands, ok := dsassets.ParsePath(checkOutlinePath)
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}, true
}
