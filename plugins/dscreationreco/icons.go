package dscreationreco

// icons.go —— 本包用到的两枚图标，都不是手画的。
//
//  1. **warning-triangle-outline**（官方 `IconWarningTriangleOutlineRegular`）：
//     info 按钮那一枚（OFF:src/client/Card.tsx:112 的 `<IconWarningTriangleOutlineRegular size={14} />`）。
//     素材原件是 rc.2 sprite 里的一格：
//
//		OFF:web/dist/assets/index-ghOpC9uV.js
//		sha256=061de8cefa15e1fbb87d301463ad4ebb79259b197ca08e83406f6997c1af8bb8
//		`"warning-triangle-outline":{size:16,svg:…}`，字节偏移 717952
//
//     viewBox=0 0 16 16（**16 不是 14**；显示尺寸才是 14）、纯描边，
//     stroke-width=1（sprite 里 regular 档 = 1、medium 档 = 1.3），
//     linecap/linejoin 都是 round。片段已存成
//     `assets/official/icons/tools/warning-triangle.jsx`（两条 path 逐字）。
//
//     **修正一条旧结论**：本文件此前写着"官方那一枚没有可复用的素材（解包树里只有
//     使用处、没有 path 数据）"，因此曾用 `IconInfoOutlineRegular` 顶替。rc.2 的
//     sprite 其实**带完整 SVG 正文**（`size:16,svg:e=>…`），只是没有以 `Icon…Regular`
//     的字面量出现——现按真实原文改用官方那一枚，顶替撤销。
//
//  2. **chevron-down**（官方 `IconChevronDownOutlineRegular`）：位置按钮右端那一枚
//     （OFF:src/client/Card.tsx:59 的 `<IconChevronDownOutlineRegular size={14} />`），
//     直接复用共享素材包 `plugins/dsassets` 的 `ChevronDownOutline()`（不复制第二份 path）。
//
// # 圆弧换算（warning-triangle 的两条 path 里各有 SVG `A/a`）
//
// `gfx.PathCommand` 只有 M/L/Q/C/Z，没有椭圆弧。这里的三段弧（顶角 + 两个底角，
// 半径都是 1.33）按**标准 arc→cubic 机械换算**：端点参数 → 圆心参数，按 ≤90° 拆段，
// 每段用 k=4/3·tan(θ/4) 的三次贝塞尔逼近；官方原文、换算脚本与输出在
// `DSH-GO/tmp/GO8-RC2/_vccr/svg2gfx_warning_triangle.{py,txt}`。
// 规则与 `plugins/dsjobs/icons.go`（layers-outline）、`plugins/dsstatusbar/icons.go:332`
// 的 `arcCommands` 是同一套。换算后曲线到真实圆弧的最大径向误差 4.1e-5（16 单位
// 坐标系）——在 14px 显示尺寸下 < 0.0001px，肉眼与像素都分不出来。

import (
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
)

// warningIconViewBox 是官方 warning-triangle 的 viewBox（16）。
//
// **不能套 14**：官方这枚的 `size` 是 16，卡片按 `size={14}` 缩到 14px 显示——
// viewBox 与显示尺寸是两个值，混了就会让三角在 14px 下小一号。
const warningIconViewBox float32 = 16

// warningIcon 是官方 `IconWarningTriangleOutlineRegular`（viewBox=16、纯描边，
// 颜色由调用方给）。
//
// 两条 path 的分工：第一条是三角轮廓（三段弧即三个圆角），第二条是感叹号
// （竖线 `M8 6v3` + 圆点 `m0 2.33h.01`，靠 round cap 画成点）。
func warningIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: warningIconViewBox,
		Stroke:  1,
		Cap:     gfx.CapRound,
		Join:    gfx.JoinRound,
		Commands: []gfx.PathCommand{
			// d="M6.87 2.6a1.33 1.33 0 0 1 2.26 0l5.34 9.33A1.33 1.33 0 0 1 13.33 14H2.67a1.33 1.33 0 0 1-1.14-2.07Z"
			gfx.MoveTo(6.87, 2.6),
			gfx.CubicTo(7.11261, 2.20915, 7.53997, 1.97143, 8, 1.97143),
			gfx.CubicTo(8.46003, 1.97143, 8.88739, 2.20915, 9.13, 2.6),
			gfx.LineTo(14.47, 11.93),
			gfx.CubicTo(14.74693, 12.34319, 14.77015, 12.87637, 14.5302, 13.31207),
			gfx.CubicTo(14.29025, 13.74777, 13.82723, 14.01316, 13.33, 14),
			gfx.LineTo(2.67, 14),
			gfx.CubicTo(2.17277, 14.01316, 1.70975, 13.74777, 1.4698, 13.31207),
			gfx.CubicTo(1.22985, 12.87637, 1.25307, 12.34319, 1.53, 11.93),
			gfx.ClosePath(),
			// d="M8 6v3m0 2.33h.01"
			gfx.MoveTo(8, 6),
			gfx.LineTo(8, 9),
			gfx.MoveTo(8, 11.33),
			gfx.LineTo(8.01, 11.33),
		},
	}
}

// chevronIcon 是官方 `IconChevronDownOutlineRegular`（viewBox=14、纯描边）。
//
// 官方在位置选择按钮的右端画它（`IconChevronDownOutlineRegular size={14}`，
// OFF:Card.tsx:59）。路径来自共享素材包，颜色由调用方给。
func chevronIcon() (gfx.Icon, bool) {
	commands, ok := dsassets.ChevronDownOutline()
	if !ok {
		return gfx.Icon{}, false
	}
	return gfx.Icon{
		ViewBox: 14, Stroke: 1, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: commands,
	}, true
}
