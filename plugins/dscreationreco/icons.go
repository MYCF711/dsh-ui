package dscreationreco

// icons.go —— 本包用到的图标素材（两枚，都不是手画的）。
//
//  1. **info-outline**（viewBox=14，三段填充）：官方 `IconInfoOutlineRegular`。
//     素材原件：`$DSH_HOME/dsh-ui-official/assets/icons/agent-team/info-outline.jsx`
//     （来源 rc.2 sprite `dsh-web-frontend/dist/assets/index-ghOpC9uV.js`，
//     viewBox=0 0 14 14、sha256=a9079c9f9a111cac22d4de9baf689be693cf1c75c97929be47efd1fef630a086、
//     字节偏移 718994；本文件的路径数据与那份 jsx 逐字一致）。
//     `plugins/dsagentteam/icons.go:42` 也存了同一份（同一个 info 按钮）。
//  2. **chevron-down**：官方 `IconChevronDownOutlineRegular`，直接复用共享素材包
//     `plugins/dsassets` 的 `ChevronDownOutline()`（不复制第二份 path 数据）。

import (
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
)

// infoIconViewBox 是官方 info-outline 的 viewBox（**14 不是 16**：套 16 会让它在
// 14px 下小一号，`dsagentteam/icons.go` 文件头点名过这个坑）。
const infoIconViewBox float32 = 14

// infoIcon 是官方 `IconInfoOutlineRegular`（viewBox=14，三段填充，颜色由调用方给）。
//
// 官方这张卡这里用的是 `IconWarningTriangleOutlineRegular`——**那一枚没有可复用的
// 素材**（`$DSH_HOME/dsh-ui-official/assets/icons/` 下没有，official-rc2 解包树里
// 只有它的使用处、没有 path 数据），按仓规不许手画近似形状冒充官方图标。
// 所以这里用同为官方素材的 info 图标顶替：语义（`creationInfo` = "创建方式与 token
// 消耗"）比形状更贴近，形状差异登记在包注释「没搬」第 3 条。
func infoIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: infoIconViewBox,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(12.5757, 7.00012),
			gfx.CubicTo(12.5757, 3.92085, 10.0794, 1.42463, 7.00012, 1.42456),
			gfx.CubicTo(3.9208, 1.42456, 1.42456, 3.9208, 1.42456, 7.00012),
			gfx.CubicTo(1.42463, 10.0794, 3.92085, 12.5757, 7.00012, 12.5757),
			gfx.CubicTo(10.0793, 12.5756, 12.5756, 10.0793, 12.5757, 7.00012),
			gfx.ClosePath(),
			gfx.MoveTo(13.8002, 7.00012),
			gfx.CubicTo(13.8001, 10.7559, 10.7559, 13.8001, 7.00012, 13.8002),
			gfx.CubicTo(3.2443, 13.8002, 0.199291, 10.7559, 0.199219, 7.00012),
			gfx.CubicTo(0.199219, 3.24426, 3.24426, 0.199219, 7.00012, 0.199219),
			gfx.CubicTo(10.7559, 0.199291, 13.8002, 3.2443, 13.8002, 7.00012),
			gfx.ClosePath(),
			gfx.MoveTo(7.6127, 3.18921),
			gfx.LineTo(7.6127, 4.55986),
			gfx.LineTo(6.38735, 4.55986),
			gfx.LineTo(6.38735, 3.18921),
			gfx.ClosePath(),
			gfx.MoveTo(7.6127, 5.68921),
			gfx.LineTo(7.6127, 10.8109),
			gfx.LineTo(6.38735, 10.8109),
			gfx.LineTo(6.38735, 5.68921),
			gfx.ClosePath(),
		},
	}
}

// chevronIcon 是官方 `IconChevronDownOutlineRegular`（14px 默认、纯描边）。
//
// 官方在位置选择按钮的右端画它（`IconChevronDownOutlineRegular size={14}`，
// OFF:Card.tsx:60）。路径来自共享素材包，颜色由调用方给。
func chevronIcon() (gfx.Icon, bool) {
	commands, ok := dsassets.ChevronDownOutline()
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{
		ViewBox: 14, Stroke: 1, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: commands,
	}, true
}
