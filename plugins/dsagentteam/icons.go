// icons.go —— 本包用到的官方 sprite 符号（rc.2 `dsh-client-ui-primitives` 的 sprite 表）。
//
//   - users-outline（16×16，`IconUsersOutlineRegular size=14`）：TeamCommandRow 的图标
//   - info-outline（**14×14**，`IconInfoOutlineRegular size=11`）：TeamModeToggle 的 info 按钮
//   - checklist-outline（16×16，`IconChecklistOutlineRegular size=14`）：TeamBoard 的 lead
//   - chevron-down-outline（16×16，`IconChevronDownOutlineRegular size=14`）：任务行折叠箭头
//
// 素材来源与 sha256 见 `$DSH_HOME/dsh-ui-official/assets/icons/agent-team/*.jsx` 与
// `structure/official-assets-reuse.md §7`。**viewBox 一律官方原值**，
// 显示尺寸由调用方按官方 `size` 传（11 / 14）——`info-outline` 是 14 不是 16，
// 套 16 会让它在 11px 下缩一号（H1 报告 §3.3 点名过的坑）。
package dsagentteam

import (
	"math"

	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
)

// usersIcon 是官方 `IconUsersOutlineRegular`（4 段描边）。
func usersIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(6.18, 8.2275),
			gfx.CubicTo(7.5620898, 8.2275, 8.6825, 7.1070898, 8.6825, 5.725),
			gfx.CubicTo(8.6825, 4.3429102, 7.5620898, 3.2225, 6.18, 3.2225),
			gfx.CubicTo(4.7979102, 3.2225, 3.6775, 4.3429102, 3.6775, 5.725),
			gfx.CubicTo(3.6775, 7.1070898, 4.7979102, 8.2275, 6.18, 8.2275),
			gfx.ClosePath(),
			gfx.MoveTo(1.63, 13.915),
			gfx.CubicTo(1.63, 11.185, 3.905, 10.0475, 6.18, 10.0475),
			gfx.CubicTo(8.455, 10.0475, 10.73, 11.185, 10.73, 13.915),
			gfx.MoveTo(10.275, 3.359),
			gfx.CubicTo(11.3215, 3.7685, 12.0495, 4.6785, 12.0495, 5.725),
			gfx.CubicTo(12.0495, 6.7715, 11.3215, 7.6815, 10.275, 8.091),
			gfx.MoveTo(12.004, 10.366),
			gfx.CubicTo(13.369, 11.003, 14.37, 12.186, 14.37, 13.915),
		},
	}
}

// infoIcon 是官方 `IconInfoOutlineRegular`（viewBox=14，三段填充）。
func infoIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 14,
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
			gfx.MoveTo(7.6127, 3.18921), gfx.LineTo(7.6127, 4.55986),
			gfx.LineTo(6.38735, 4.55986), gfx.LineTo(6.38735, 3.18921),
			gfx.ClosePath(),
			gfx.MoveTo(7.6127, 5.68921), gfx.LineTo(7.6127, 10.8109),
			gfx.LineTo(6.38735, 10.8109), gfx.LineTo(6.38735, 5.68921),
			gfx.ClosePath(),
		},
	}
}

// checklistIcon 是官方 `IconChecklistOutlineRegular`（两环 + 两条线）。
func checklistIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(3.75, 6.25),
			gfx.CubicTo(4.7165, 6.25, 5.5, 5.4665, 5.5, 4.5),
			gfx.CubicTo(5.5, 3.5335, 4.7165, 2.75, 3.75, 2.75),
			gfx.CubicTo(2.7835, 2.75, 2, 3.5335, 2, 4.5),
			gfx.CubicTo(2, 5.4665, 2.7835, 6.25, 3.75, 6.25),
			gfx.ClosePath(),
			gfx.MoveTo(7.5, 4.5), gfx.LineTo(13.5, 4.5),
			gfx.MoveTo(3.75, 13.25),
			gfx.CubicTo(4.7165, 13.25, 5.5, 12.4665, 5.5, 11.5),
			gfx.CubicTo(5.5, 10.5335, 4.7165, 9.75, 3.75, 9.75),
			gfx.CubicTo(2.7835, 9.75, 2, 10.5335, 2, 11.5),
			gfx.CubicTo(2, 12.4665, 2.7835, 13.25, 3.75, 13.25),
			gfx.ClosePath(),
			gfx.MoveTo(7.5, 11.5), gfx.LineTo(13.5, 11.5),
		},
	}
}

// chevronDownIcon 是官方 `IconChevronDownOutlineRegular`。
func chevronDownIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(4, 6),
			gfx.LineTo(7.29289, 9.29289),
			gfx.CubicTo(7.68342, 9.68342, 8.31658, 9.68342, 8.70711, 9.29289),
			gfx.LineTo(12, 6),
		},
	}
}

// chevronUpIcon 是官方 sprite `chevron-up-outline`，用于任务行展开态的
// rotate(180deg) 静态等价方向（index-ghOpC9uV.js:129）。
func chevronUpIcon() gfx.Icon {
	commands, ok := dsassets.ParsePath(`M12 10L8.70711 6.70711C8.31658 6.31658 7.68342 6.31658 7.29289 6.70711L4 10`)
	if !ok {
		return gfx.Icon{}
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}
}

// chevronRightIcon 复用 dsassets 已登记的官方 `chevron-right-outline`。
// 成员行的 CSS 把 svg 固定为 12×12（SubagentHeaderLineage.module.css:329-333）。
func chevronRightIcon() gfx.Icon {
	icon, ok := dsassets.PlanReviewChevron()
	if !ok {
		return gfx.Icon{}
	}
	return icon
}

// checkOutlineIcon 复用官方 `check-outline` 原件
// （assets/official/icons/composer/check.jsx:1；SubagentHeaderLineage.tsx:406 size=16）。
func checkOutlineIcon() gfx.Icon {
	commands, ok := dsassets.CheckOutline()
	if !ok {
		return gfx.Icon{}
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}
}

// refreshOutlineIcon 是官方 `refresh-outline` 的两条原始 path
// （index-ghOpC9uV.js:129；目录错误行 SubagentHeaderLineage.tsx:276 size=14）。
func refreshOutlineIcon() gfx.Icon {
	var commands []gfx.PathCommand
	for _, path := range []string{
		`M14.5001 8C14.5 9.28552 14.1188 10.5422 13.4045 11.611C12.6903 12.6799 11.6752 13.5129 10.4875 14.0049C9.29982 14.4968 7.99295 14.6255 6.73212 14.3747C5.4713 14.124 4.31314 13.505 3.4041 12.596C2.49514 11.687 1.87614 10.5288 1.62537 9.26798C1.37459 8.00716 1.50331 6.70028 1.99525 5.51261C2.48719 4.32494 3.32025 3.30981 4.3891 2.59557C5.45795 1.88134 6.71458 1.50008 8.0001 1.5C9.9001 1.5 11.7001 2.3 13.0001 3.6L14.5001 5.1`,
		`M14.4999 1.5L14.4999 5.1L10.8999 5.1`,
	} {
		parsed, ok := dsassets.ParsePath(path)
		if !ok {
			return gfx.Icon{}
		}
		commands = append(commands, parsed...)
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}
}

// StateDot ongoing 的官方 SVG/CSS 几何（不是新造图标）：
// index-ghOpC9uV.js:129：viewBox=24、cx=cy=12、r=9.5、默认 14px；
// index-CSAdOBx6.css:1：stroke=2、linecap=round、track opacity=.25。
// Viva 静态帧采用官方 prefers-reduced-motion 档：dash=18、offset=-3。
const (
	stateDotSpinnerViewBox = 24
	stateDotSpinnerCenter  = 12
	stateDotSpinnerRadius  = 9.5
	stateDotSpinnerStroke  = 2
	stateDotSpinnerDash    = 18
	stateDotSpinnerOffset  = -3
	stateDotTrackAlpha     = 0.25
)

func stateDotOngoingIcon(tint gfx.Color) gfx.Icon {
	circumference := 2 * math.Pi * stateDotSpinnerRadius
	start := -stateDotSpinnerOffset / circumference * 360
	sweep := stateDotSpinnerDash / circumference * 360
	track := tint
	track.Alpha *= stateDotTrackAlpha
	return gfx.Icon{
		ViewBox: stateDotSpinnerViewBox,
		Stroke:  stateDotSpinnerStroke,
		Cap:     gfx.CapRound,
		Commands: stateDotArcCommands(stateDotSpinnerCenter, stateDotSpinnerCenter,
			stateDotSpinnerRadius, float32(start), float32(sweep)),
		Layers: []gfx.IconLayer{{
			Fill: track, FillRule: gfx.PathFillEvenOdd,
			Commands: stateDotRingCommands(stateDotSpinnerCenter, stateDotSpinnerCenter,
				stateDotSpinnerRadius+stateDotSpinnerStroke/2,
				stateDotSpinnerRadius-stateDotSpinnerStroke/2),
		}},
	}
}

// stateDotArcCommands 按官方 circle+dasharray 参数换算静态弧；每段 ≤90°。
func stateDotArcCommands(cx, cy, r, startDeg, sweepDeg float32) []gfx.PathCommand {
	if sweepDeg <= 0 {
		return nil
	}
	segments := int(math.Ceil(float64(sweepDeg / 90)))
	step := sweepDeg / float32(segments)
	angle := float64(startDeg) * math.Pi / 180
	commands := make([]gfx.PathCommand, 0, segments*4)
	commands = append(commands, gfx.MoveTo(
		cx+r*float32(math.Cos(angle)), cy+r*float32(math.Sin(angle)),
	))
	for i := 0; i < segments; i++ {
		a0, a1 := angle, angle+float64(step)*math.Pi/180
		k := 4.0 / 3.0 * math.Tan((a1-a0)/4)
		p0x, p0y := float64(cx)+float64(r)*math.Cos(a0), float64(cy)+float64(r)*math.Sin(a0)
		p3x, p3y := float64(cx)+float64(r)*math.Cos(a1), float64(cy)+float64(r)*math.Sin(a1)
		commands = append(commands, gfx.CubicTo(
			float32(p0x-k*float64(r)*math.Sin(a0)), float32(p0y+k*float64(r)*math.Cos(a0)),
			float32(p3x+k*float64(r)*math.Sin(a1)), float32(p3y-k*float64(r)*math.Cos(a1)),
			float32(p3x), float32(p3y),
		))
		angle = a1
	}
	return commands
}

func stateDotRingCommands(cx, cy, outer, inner float32) []gfx.PathCommand {
	commands := stateDotCircleCommands(cx, cy, outer)
	return append(commands, stateDotCircleCommands(cx, cy, inner)...)
}

func stateDotCircleCommands(cx, cy, r float32) []gfx.PathCommand {
	k := 0.5522847498 * r
	return []gfx.PathCommand{
		gfx.MoveTo(cx+r, cy),
		gfx.CubicTo(cx+r, cy+k, cx+k, cy+r, cx, cy+r),
		gfx.CubicTo(cx-k, cy+r, cx-r, cy+k, cx-r, cy),
		gfx.CubicTo(cx-r, cy-k, cx-k, cy-r, cx, cy-r),
		gfx.CubicTo(cx+k, cy-r, cx+r, cy-k, cx+r, cy),
		gfx.ClosePath(),
	}
}
