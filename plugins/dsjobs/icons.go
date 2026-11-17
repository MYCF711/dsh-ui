// icons.go —— 本包用到的官方 sprite 符号（rc.2 `dsh-client-ui-primitives` 的 sprite 表）。
//
//   - layers-outline（16×16，`IconLayersOutlineRegular size=14`）：JobListAction 触发器的
//     空闲态图标（`OFF:dsh-client-ui-jobs/src/client/JobListAction.tsx:549-552`）；
//   - stop-fill（16×16，`IconStopFillRegular size=10`）：运行中行停止钮（`JobListAction.tsx:280-287`）。
//
// 素材来源：官方 rc.2 的 sprite 表在 `dsh-web-frontend/dist/assets/index-ghOpC9uV.js`
// 的 `"layers-outline":{size:16,svg:...}` / `"stop-fill":{size:16,svg:...}` 段；
// **viewBox 一律官方原值 16**，显示尺寸由调用方按官方 `size` 传（layers 14 / stop 10）。
//
// 注意：`layers-outline` 的原路径用的是 SVG `a`（圆弧）命令，而 viva 的 gfx.Icon
// 只有 Move/Line/Quad/Cubic——这里的曲线是把圆弧按**标准 arc→cubic（每段 ≤90°）**
// 机械换算出来的，换算脚本与输入在 `tmp/GO8-RC2/_gp2/svg2gfx.txt`。
package dsjobs

import (
	"math"

	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
)

// layersIcon 是官方 `IconLayersOutlineRegular`（3 段描边：顶层圆角菱形 + 两条层线）。
func layersIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Cap:     gfx.CapRound,
		Join:    gfx.JoinRound,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(8.48137, 2.30437),
			gfx.CubicTo(8.17561, 2.16491, 7.82439, 2.16491, 7.51863, 2.30437),
			gfx.LineTo(2.54797, 4.56637),
			gfx.CubicTo(2.3377, 4.65911, 2.20202, 4.86726, 2.20202, 5.09707),
			gfx.CubicTo(2.20202, 5.32689, 2.3377, 5.53503, 2.54797, 5.62777),
			gfx.LineTo(7.52437, 7.8956),
			gfx.CubicTo(7.83016, 8.03509, 8.18141, 8.03509, 8.4872, 7.8956),
			gfx.LineTo(13.4636, 5.6336),
			gfx.CubicTo(13.6739, 5.54086, 13.8096, 5.33271, 13.8096, 5.1029),
			gfx.CubicTo(13.8096, 4.87309, 13.6739, 4.66494, 13.4636, 4.5722),
			gfx.ClosePath(),
			gfx.MoveTo(2.19997, 8),
			gfx.CubicTo(2.19941, 8.22655, 2.3308, 8.43268, 2.5364, 8.52783),
			gfx.LineTo(7.52437, 10.7956),
			gfx.CubicTo(7.82851, 10.9333, 8.17724, 10.9333, 8.48137, 10.7956),
			gfx.LineTo(13.4578, 8.53357),
			gfx.CubicTo(13.6676, 8.4393, 13.8018, 8.22998, 13.8, 8),
			gfx.MoveTo(2.19997, 10.9),
			gfx.CubicTo(2.19941, 11.1265, 2.3308, 11.3327, 2.5364, 11.4278),
			gfx.LineTo(7.52437, 13.6956),
			gfx.CubicTo(7.82851, 13.8334, 8.17724, 13.8334, 8.48137, 13.6956),
			gfx.LineTo(13.4578, 11.4336),
			gfx.CubicTo(13.6676, 11.3393, 13.8019, 11.13, 13.8, 10.9),
		},
	}
}

// stopIcon 是官方 `IconStopFillRegular`（实心圆角方块；Stroke=0 表示按填充画）。
func stopIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(12.5, 2.5),
			gfx.LineTo(3.5, 2.5),
			gfx.CubicTo(2.94772, 2.5, 2.5, 2.94772, 2.5, 3.5),
			gfx.LineTo(2.5, 12.5),
			gfx.CubicTo(2.5, 13.0523, 2.94772, 13.5, 3.5, 13.5),
			gfx.LineTo(12.5, 13.5),
			gfx.CubicTo(13.0523, 13.5, 13.5, 13.0523, 13.5, 12.5),
			gfx.LineTo(13.5, 3.5),
			gfx.CubicTo(13.5, 2.94772, 13.0523, 2.5, 12.5, 2.5),
			gfx.ClosePath(),
		},
	}
}

// jobChevronDownIcon 复用共享素材包的 IconChevronDownOutlineRegular。
func jobChevronDownIcon() gfx.Icon {
	commands, ok := dsassets.ChevronDownOutline()
	if !ok {
		return gfx.Icon{}
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}
}

// jobChevronRightIcon 是官方 chevron-right-outline，等价于 down 的 rotate(-90deg)。
func jobChevronRightIcon() gfx.Icon {
	icon, ok := dsassets.PlanReviewChevron()
	if !ok {
		return gfx.Icon{}
	}
	return icon
}

// jobChevronUpIcon 是官方 chevron-up-outline，等价于 down 的 rotate(180deg)。
// 路径来源同为 index-ghOpC9uV.js:129 的 sprite 表。
func jobChevronUpIcon() gfx.Icon {
	commands, ok := dsassets.ParsePath(`M12 10L8.70711 6.70711C8.31658 6.31658 7.68342 6.31658 7.29289 6.70711L4 10`)
	if !ok {
		return gfx.Icon{}
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}
}

func jobRowChevronIcon(open bool) gfx.Icon {
	if open {
		return jobChevronUpIcon()
	}
	return jobChevronRightIcon()
}

func jobSectionChevronIcon(open bool) gfx.Icon {
	if open {
		return jobChevronDownIcon()
	}
	return jobChevronRightIcon()
}

// StateDot ongoing 的官方 SVG/CSS 几何：index-ghOpC9uV.js:129 给出
// viewBox=24、cx=cy=12、r=9.5、默认 14px；index-CSAdOBx6.css:1 给出
// stroke=2、linecap=round、track opacity=.25。静态帧采用官方
// prefers-reduced-motion 档：dash=18、offset=-3，不手画替代图标。
const (
	jobStateDotSpinnerViewBox = 24
	jobStateDotSpinnerCenter  = 12
	jobStateDotSpinnerRadius  = 9.5
	jobStateDotSpinnerStroke  = 2
	jobStateDotSpinnerDash    = 18
	jobStateDotSpinnerOffset  = -3
	jobStateDotTrackAlpha     = 0.25
)

func jobStateDotOngoingIcon(tint gfx.Color) gfx.Icon {
	circumference := 2 * math.Pi * jobStateDotSpinnerRadius
	start := -jobStateDotSpinnerOffset / circumference * 360
	sweep := jobStateDotSpinnerDash / circumference * 360
	track := tint
	track.Alpha *= jobStateDotTrackAlpha
	return gfx.Icon{
		ViewBox: jobStateDotSpinnerViewBox,
		Stroke:  jobStateDotSpinnerStroke,
		Cap:     gfx.CapRound,
		Commands: jobStateDotArcCommands(jobStateDotSpinnerCenter, jobStateDotSpinnerCenter,
			jobStateDotSpinnerRadius, float32(start), float32(sweep)),
		Layers: []gfx.IconLayer{{
			Fill: track, FillRule: gfx.PathFillEvenOdd,
			Commands: jobStateDotRingCommands(jobStateDotSpinnerCenter, jobStateDotSpinnerCenter,
				jobStateDotSpinnerRadius+jobStateDotSpinnerStroke/2,
				jobStateDotSpinnerRadius-jobStateDotSpinnerStroke/2),
		}},
	}
}

// jobStateDotArcCommands 把官方 circle+dasharray 机械换算成静态弧，每段 ≤90°。
func jobStateDotArcCommands(cx, cy, r, startDeg, sweepDeg float32) []gfx.PathCommand {
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

func jobStateDotRingCommands(cx, cy, outer, inner float32) []gfx.PathCommand {
	commands := jobStateDotCircleCommands(cx, cy, outer)
	return append(commands, jobStateDotCircleCommands(cx, cy, inner)...)
}

func jobStateDotCircleCommands(cx, cy, r float32) []gfx.PathCommand {
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
