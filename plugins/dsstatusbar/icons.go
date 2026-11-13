// icons.go —— 连接指示 / 读数胶囊 / 详情弹窗用到的官方 sprite 符号
// （rc.2 `dsh-client-ui-primitives` 的 sprite 表 `Vw`）。
//
// # 官方出处（逐字）
//
// 所有图标在 rc.2 都是 sprite 表的一条（`M(name,weight)` + `<use href="#dsw-icon-…">`），
// 生成器 `svg({viewBox, "data-icon":"dsw-icon-<name>-<weight>"})`。dist：
// `dsh-web-frontend/dist/assets/index-ghOpC9uV.js`，1,093,231 B，
// sha256 `061de8cefa15e1fbb87d301463ad4ebb79259b197ca08e83406f6997c1af8bb8`。
//
//	name              viewBox      sprite 偏移  官方调用点
//	check-outline     0 0 16 16   671570      ConnectionIndicator（recovered）
//	refresh-outline   0 0 16 16   675822      ConnectionIndicator（disconnected/connecting 的按钮）
//	loading-outline   0 0 16 16   693206      DesktopUpdateIndicator（busy 态 14px）
//	download-outline  0 0 16 16   693771      DesktopUpdateIndicator（非 busy 态 14px）
//	gauge-outline     0 0 16 16   710395      ActivityPill / Activity 弹窗标题（`IconGaugeOutlineRegular`）
//	database-outline  0 0 16 16   708742      UsagePill（`IconDatabaseOutlineRegular`）
//
// `check-outline` 素材库内有同物：assets/icons/composer/check.jsx
// （sha256 3b47a8c21d5bf626867f2d6b45a6ff8b2f7362123cfa76de5a395a2b2916bb8c），
// 其 d 与 rc.2 sprite 逐字节相同。`gauge`/`database` 库内有
// assets/icons/dock/{gauge,database}.jsx（ACTIVITY/USAGE 胶囊专用），
// 本文件按 rc.2 sprite 原文逐点转写（path 数据逐字节一致，见各自注释）。
//
// 显示尺寸一律由调用方给官方 CSS 的 14（`._icon_16cyo_71{width:14px;height:14px}` /
// `.pill svg{width:14px;height:14px}` / `.titleLabel svg{width:14px;height:14px}`），
// **viewBox 保持官方 16**，不自行缩放。
package dsstatusbar

import (
	"math"
	"time"

	"github.com/vivaui/viva/gfx"
)

// connectionIconSize 是官方 ._icon_16cyo_71{width:14px;height:14px}。
const connectionIconSize float32 = 14

// checkOutlineIcon 是官方 sprite `check-outline`（成功态：已连接 / 已恢复）。
func checkOutlineIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []gfx.PathCommand{
			// 第 1 段（官方 path #1）
			gfx.MoveTo(2.25, 8.5),
			gfx.LineTo(5.49732, 11.7473),
			gfx.CubicTo(5.90519, 12.1552, 6.57263, 12.1344, 6.95426, 11.7018),
			gfx.LineTo(13.75, 4),
		},
	}
}

// refreshOutlineIcon 是官方 sprite `refresh-outline`（异常态：断开 / 正在重试）。
func refreshOutlineIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []gfx.PathCommand{
			// 第 1 段（官方 path #1）
			gfx.MoveTo(14.5001, 8),
			gfx.CubicTo(14.5, 9.28552, 14.1188, 10.5422, 13.4045, 11.611),
			gfx.CubicTo(12.6903, 12.6799, 11.6752, 13.5129, 10.4875, 14.0049),
			gfx.CubicTo(9.29982, 14.4968, 7.99295, 14.6255, 6.73212, 14.3747),
			gfx.CubicTo(5.4713, 14.124, 4.31314, 13.505, 3.4041, 12.596),
			gfx.CubicTo(2.49514, 11.687, 1.87614, 10.5288, 1.62537, 9.26798),
			gfx.CubicTo(1.37459, 8.00716, 1.50331, 6.70028, 1.99525, 5.51261),
			gfx.CubicTo(2.48719, 4.32494, 3.32025, 3.30981, 4.3891, 2.59557),
			gfx.CubicTo(5.45795, 1.88134, 6.71458, 1.50008, 8.0001, 1.5),
			gfx.CubicTo(9.9001, 1.5, 11.7001, 2.3, 13.0001, 3.6),
			gfx.LineTo(14.5001, 5.1),
			// 第 2 段（官方 path #2）
			gfx.MoveTo(14.4999, 1.5),
			gfx.LineTo(14.4999, 5.1),
			gfx.LineTo(10.8999, 5.1),
		},
	}
}

// gaugeOutlineIcon 是官方 sprite `gauge-outline`（ActivityPill / Activity 弹窗标题）。
//
// # 官方原文（sprite `gauge-outline`，dist 偏移 710395）
//
//	<svg viewBox="0 0 16 16" fill="none" stroke-width="1">
//	  <path d="M3.4041 13.096C…12.5961 13.096" stroke="currentColor"/>
//	  <path d="M8 8.49994L11.6114 4.88855" stroke="currentColor"/>
//	  <path d="M8 9.75C8.69036 9.75 9.25 9.19036 9.25 8.5C9.25 7.80964 8.69036 7.25 8 7.25
//	           C7.30964 7.25 6.75 7.80964 6.75 8.5C6.75 9.19036 7.30964 9.75 8 9.75Z"
//	        fill="currentColor"/>
//	</svg>
//
// 与库内 assets/icons/dock/gauge.jsx 的路径数据逐字节相同（两条 stroke + 一条 fill）。
// 第三条是**实心圆点**（`fill:currentColor`），`gfx.Icon` 的 stroke 命令画不了 fill，
// 所以按 viva 的 `IconLayer{Fill: tint}` 表达——颜色等于同一枚 tint，语义与
// `currentColor` 一致（层色的 alpha 会再乘 tint 的 alpha，见 viva `paintIconShapes`）。
func gaugeOutlineIcon(tint gfx.Color) gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []gfx.PathCommand{
			// 官方 path #1（表盘外弧）
			gfx.MoveTo(3.4041, 13.096),
			gfx.CubicTo(2.49514, 12.187, 1.87614, 11.0288, 1.62537, 9.76798),
			gfx.CubicTo(1.37459, 8.50716, 1.50331, 7.20028, 1.99525, 6.01261),
			gfx.CubicTo(2.48719, 4.82494, 3.32025, 3.80981, 4.3891, 3.09557),
			gfx.CubicTo(5.45795, 2.38134, 6.71458, 2.00008, 8.0001, 2),
			gfx.CubicTo(9.28563, 2.00008, 10.5423, 2.38134, 11.6111, 3.09557),
			gfx.CubicTo(12.68, 3.80981, 13.513, 4.82494, 14.005, 6.01261),
			gfx.CubicTo(14.4969, 7.20028, 14.6256, 8.50716, 14.3748, 9.76798),
			gfx.CubicTo(14.1241, 11.0288, 13.5051, 12.187, 12.5961, 13.096),
			// 官方 path #2（指针）
			gfx.MoveTo(8, 8.49994),
			gfx.LineTo(11.6114, 4.88855),
		},
		Layers: []gfx.IconLayer{{
			Fill: tint, // 官方第三条 path 是 fill="currentColor" 的圆点
			Commands: []gfx.PathCommand{
				gfx.MoveTo(8, 9.75),
				gfx.CubicTo(8.69036, 9.75, 9.25, 9.19036, 9.25, 8.5),
				gfx.CubicTo(9.25, 7.80964, 8.69036, 7.25, 8, 7.25),
				gfx.CubicTo(7.30964, 7.25, 6.75, 7.80964, 6.75, 8.5),
				gfx.CubicTo(6.75, 9.19036, 7.30964, 9.75, 8, 9.75),
				gfx.ClosePath(),
			},
		}},
	}
}

// databaseOutlineIcon 是官方 sprite `database-outline`（UsagePill）。
//
// # 官方原文（sprite `database-outline`，dist 偏移 708742）
//
// 五条 `stroke="currentColor"` 的 path（顶盖椭圆 + 左右两条母线 + 两条层线），
// viewBox `0 0 16 16`、`stroke-width=1`。与库内 assets/icons/dock/database.jsx
// 的路径数据逐字节相同。没有 fill 命令，所以只要普通 stroke 图标。
func databaseOutlineIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []gfx.PathCommand{
			// 官方 path #1（顶盖椭圆）
			gfx.MoveTo(13.1967, 5.1869),
			gfx.CubicTo(13.7232, 4.77378, 14.0003, 4.30517, 14.0001, 3.82819),
			gfx.CubicTo(14.0003, 3.3512, 13.7232, 2.88259, 13.1967, 2.46947),
			gfx.CubicTo(12.6702, 2.05635, 11.9128, 1.71328, 11.0006, 1.47475),
			gfx.CubicTo(10.0885, 1.23621, 9.05371, 1.11062, 8.00039, 1.1106),
			gfx.CubicTo(6.94707, 1.11057, 5.9123, 1.23612, 5.00009, 1.47461),
			gfx.CubicTo(4.08742, 1.71301, 3.32948, 2.05604, 2.80249, 2.46919),
			gfx.CubicTo(2.2755, 2.88235, 1.99805, 3.35106, 1.99805, 3.82819),
			gfx.CubicTo(1.99805, 4.30531, 2.2755, 4.77402, 2.80249, 5.18718),
			gfx.CubicTo(3.32948, 5.60033, 4.08742, 5.94336, 5.00009, 6.18176),
			gfx.CubicTo(5.9123, 6.42025, 6.94707, 6.5458, 8.00039, 6.54578),
			gfx.CubicTo(9.05371, 6.54575, 10.0885, 6.42016, 11.0006, 6.18163),
			gfx.CubicTo(11.9128, 5.94309, 12.6702, 5.60002, 13.1967, 5.1869),
			gfx.ClosePath(),
			// 官方 path #2/#3（左右母线）
			gfx.MoveTo(2, 3.80371),
			gfx.LineTo(2, 11.7848),
			gfx.MoveTo(14, 3.80371),
			gfx.LineTo(14, 11.7848),
			// 官方 path #4（第二层）
			gfx.MoveTo(2, 7.81396),
			gfx.CubicTo(2, 8.60524, 2.63214, 9.36411, 3.75736, 9.92363),
			gfx.CubicTo(4.88258, 10.4832, 6.4087, 10.7975, 8, 10.7975),
			gfx.CubicTo(9.5913, 10.7975, 11.1174, 10.4832, 12.2426, 9.92363),
			gfx.CubicTo(13.3679, 9.36411, 14, 8.60524, 14, 7.81396),
			// 官方 path #5（第三层 / 底盖）
			gfx.MoveTo(2, 11.7847),
			gfx.CubicTo(2, 12.6081, 2.63214, 13.3977, 3.75736, 13.98),
			gfx.CubicTo(4.88258, 14.5622, 6.4087, 14.8893, 8, 14.8893),
			gfx.CubicTo(9.5913, 14.8893, 11.1174, 14.5622, 12.2426, 13.98),
			gfx.CubicTo(13.3679, 13.3977, 14, 12.6081, 14, 11.7847),
		},
	}
}

// ── spinner（官方 primitives `StateDot state="ongoing"`）────────────────────
//
// # 官方原文（dist 偏移 743670 的 `zd` 组件）
//
//	e === "ongoing" &&
//	  <svg className={spinner} data-state="ongoing" width={n} height={n}
//	       viewBox="0 0 24 24" aria-hidden="true">
//	    <g className={spinnerMotion}>
//	      <circle className={spinnerTrack} cx="12" cy="12" r="9.5" />
//	      <circle className={spinnerArc}  cx="12" cy="12" r="9.5" />
//	    </g>
//	  </svg>
//
// 默认 `n = 14`（ongoing 的默认 size）。CSS（`index-CSAdOBx6.css` 的
// `_spinner_1i3xo_37` / `_spinnerMotion_1i3xo_42` / `_spinnerTrack_1i3xo_47` /
// `_spinnerArc_1i3xo_48` + 两个 keyframes，逐字）：
//
//	.spinner{flex:none;color:var(--dsw-alias-label-tertiary)}
//	.spinnerMotion{transform-origin:center;animation:dsh-state-dot-spin 1.5s linear infinite}
//	.spinnerTrack,.spinnerArc{fill:none;stroke:currentColor;stroke-width:2;stroke-linecap:round}
//	.spinnerTrack{opacity:.25}
//	.spinnerArc{stroke-dasharray:12 150;animation:dsh-state-dot-dash 1.5s ease-in-out infinite}
//	@keyframes dsh-state-dot-spin{to{transform:rotate(360deg)}}
//	@keyframes dsh-state-dot-dash{
//	  0%{stroke-dasharray:12 150;stroke-dashoffset:0}
//	  50%{stroke-dasharray:24 150;stroke-dashoffset:-6}
//	  to{stroke-dasharray:12 150;stroke-dashoffset:0}}
//	@media(prefers-reduced-motion:reduce){
//	  .spinnerMotion,.spinnerArc{animation:none}
//	  .spinnerArc{stroke-dasharray:18 150;stroke-dashoffset:-3}}
//
// # 本仓的转写（逐条对齐，不是近似图形）
//
// viva 没有 `stroke-dasharray`，所以把"同一段弧 + dash 动画 + 整体旋转"折算成
// **每帧算出的弧起角/弧长**再画（几何等价）：
//
//	弧长(度) = dasharray ÷ 周长 × 360，周长 = 2π·9.5 = 59.6903（官方 r=9.5）
//	起角(度) = 旋转角 + (-dashoffset) ÷ 周长 × 360
//
// 于是三个官方量都有了唯一出处：
//
//	旋转  1.5s linear infinite            → rotateDeg = 360 × phase
//	弧长  12 → 24 → 12（1.5s ease-in-out）  → dashUnits
//	偏移  0 → -6 → 0（同一缓动）            → offsetUnits
//	减动效：不转、弧长 18、偏移 -3（官方 media query 的静态档）
//
// track 那条 25% 的整圈用 `IconLayer{EvenOdd}` 画成圆环（同样 2 宽、同样 tint×0.25），
// 与官方 `stroke:currentColor;opacity:.25` 等价。
const (
	// spinnerViewBox 是官方 `viewBox="0 0 24 24"`。
	spinnerViewBox float32 = 24
	// spinnerCenter / spinnerRadius 是官方 `<circle cx=12 cy=12 r=9.5>`。
	spinnerCenter float32 = 12
	spinnerRadius float32 = 9.5
	// spinnerStroke 是官方 `stroke-width:2`。
	spinnerStroke float32 = 2
	// spinnerDashGap 是官方 dasharray 的 gap（恒 150，远大于周长 → 只留一段弧）。
	spinnerDashGap float32 = 150
	// 官方 keyframes 的两个端点值。
	spinnerDashFrom  float32 = 12 // 0% / to
	spinnerDashMid   float32 = 24 // 50%
	spinnerOffsetTo  float32 = 0
	spinnerOffsetMid float32 = -6
	// 减动效档（`prefers-reduced-motion`）的静态取值。
	spinnerReducedDash   float32 = 18
	spinnerReducedOffset float32 = -3
	// spinnerTrackAlpha 是官方 `.spinnerTrack{opacity:.25}`。
	spinnerTrackAlpha float32 = 0.25
)

// spinnerPeriod 是官方两个动画的时长：`1.5s`。
const spinnerPeriod = 1500 * time.Millisecond

// spinnerCircumference 是官方圆的周长（弧长/偏移都按它换算成角度）。
var spinnerCircumference = 2 * math.Pi * float64(spinnerRadius)

// spinnerFrame 是一帧 spinner 的几何（单位：度 / 官方 dash 单位）。
//
// 之所以单独成类型：真机验收与单测都要能直接断言"不同时间点角度不同"，
// 而不是去比对一串 path 命令。
type spinnerFrame struct {
	// Rotate 是官方 `dsh-state-dot-spin` 的旋转角（度，顺时针）。
	Rotate float32
	// Dash 是官方 `stroke-dasharray` 的可见段长（官方单位，非角度）。
	Dash float32
	// Offset 是官方 `stroke-dashoffset`（官方单位；负值 = 沿路径前移）。
	Offset float32
}

// startDeg 是这一段弧在屏幕坐标系里的起角（度；0 = 3 点钟方向，顺时针为正）。
func (f spinnerFrame) startDeg() float32 {
	return f.Rotate + float32(-float64(f.Offset)/spinnerCircumference*360)
}

// sweepDeg 是这一段弧的张角（度）。
func (f spinnerFrame) sweepDeg() float32 {
	return float32(float64(f.Dash) / spinnerCircumference * 360)
}

// spinnerFrameAt 算某一时刻的 spinner 几何（官方两个 1.5s 动画的合成）。
//
// reducedMotion 为真时走官方 `prefers-reduced-motion` 分支：不转、弧长 18、偏移 -3。
func spinnerFrameAt(now time.Time, reducedMotion bool) spinnerFrame {
	if reducedMotion {
		return spinnerFrame{Rotate: 0, Dash: spinnerReducedDash, Offset: spinnerReducedOffset}
	}
	phase := spinnerPhase(now)
	// 官方 keyframes 是 0% → 50% → 100% 两段，**每段各自**套一次 `ease-in-out`
	// （CSS 的 timing function 按 keyframe 段生效，不是跨整条动画）。
	// progress: 0 = 0% 关键帧（dash 12 / offset 0），1 = 50% 关键帧（dash 24 / offset -6）。
	var progress float32
	if phase < 0.5 {
		progress = easeInOut(phase / 0.5)
	} else {
		progress = 1 - easeInOut((phase-0.5)/0.5)
	}
	return spinnerFrame{
		Rotate: 360 * phase,
		Dash:   spinnerDashFrom + (spinnerDashMid-spinnerDashFrom)*progress,
		Offset: spinnerOffsetTo + (spinnerOffsetMid-spinnerOffsetTo)*progress,
	}
}

// spinnerPhase 把时刻折算成 1.5s 周期里的相位 [0,1)。
func spinnerPhase(now time.Time) float32 {
	period := int64(spinnerPeriod)
	nanos := now.UnixNano() % period
	if nanos < 0 {
		nanos += period
	}
	return float32(nanos) / float32(period)
}

// spinnerGlyph 把某一帧的 spinner 几何变成可画的 `gfx.Icon`（弧 + 25% 圆环）。
//
// 颜色按官方 `.spinner{color:label-tertiary}`：调用方传 tint。
func spinnerGlyph(frame spinnerFrame, tint gfx.Color) gfx.Icon {
	return gfx.Icon{
		ViewBox: spinnerViewBox,
		Stroke:  spinnerStroke,
		Cap:     gfx.CapRound,
		Commands: arcCommands(spinnerCenter, spinnerCenter, spinnerRadius,
			frame.startDeg(), frame.sweepDeg()),
		Layers: []gfx.IconLayer{{
			// 官方 `.spinnerTrack{fill:none;stroke:currentColor;stroke-width:2;opacity:.25}`
			// → 同宽圆环（外 r+1 / 内 r-1）+ 25% alpha。
			Fill:     withAlpha(tint, spinnerTrackAlpha),
			FillRule: gfx.PathFillEvenOdd,
			Commands: ringCommands(spinnerCenter, spinnerCenter,
				spinnerRadius+spinnerStroke/2, spinnerRadius-spinnerStroke/2),
		}},
	}
}

// arcCommands 用三次贝塞尔逼近一段圆弧（末段 ≤90°，误差 < 0.02%）。
//
// k = 4/3·tan(θ/4) 是圆弧到单段三次贝塞尔的标准系数；超过 90° 的弧拆成多段，
// 否则系数失真。
func arcCommands(cx, cy, r, startDeg, sweepDeg float32) []gfx.PathCommand {
	if sweepDeg <= 0 {
		return nil
	}
	const maxSweep = 90
	segments := int(math.Ceil(float64(sweepDeg) / maxSweep))
	step := sweepDeg / float32(segments)
	commands := make([]gfx.PathCommand, 0, segments*4)
	angle := float64(startDeg) * math.Pi / 180
	commands = append(commands, gfx.MoveTo(
		cx+r*float32(math.Cos(angle)),
		cy+r*float32(math.Sin(angle)),
	))
	for i := 0; i < segments; i++ {
		a0 := angle
		a1 := angle + float64(step)*math.Pi/180
		k := 4.0 / 3.0 * math.Tan((a1-a0)/4)
		p0x, p0y := float64(cx)+float64(r)*math.Cos(a0), float64(cy)+float64(r)*math.Sin(a0)
		p3x, p3y := float64(cx)+float64(r)*math.Cos(a1), float64(cy)+float64(r)*math.Sin(a1)
		c1x, c1y := p0x-k*float64(r)*math.Sin(a0), p0y+k*float64(r)*math.Cos(a0)
		c2x, c2y := p3x+k*float64(r)*math.Sin(a1), p3y-k*float64(r)*math.Cos(a1)
		commands = append(commands, gfx.CubicTo(
			float32(c1x), float32(c1y), float32(c2x), float32(c2y), float32(p3x), float32(p3y),
		))
		angle = a1
	}
	return commands
}

// ringCommands 是两个同心圆组成的闭合轮廓（配合 EvenOdd 填充规则 = 圆环）。
func ringCommands(cx, cy, outer, inner float32) []gfx.PathCommand {
	commands := circleCommands(cx, cy, outer)
	return append(commands, circleCommands(cx, cy, inner)...)
}

// circleCommands 用四段三次贝塞尔画一个整圆（标准 k = 0.5523·r）。
func circleCommands(cx, cy, r float32) []gfx.PathCommand {
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

// spinnerSize 是官方 ongoing 态的默认 size（`n = e === "ongoing" ? 14 : 10`），
// 连接胶囊的 `._icon{width:14px;height:14px}` 也是同一个数。
const spinnerSize float32 = 14

// easeInOut 是 CSS `ease-in-out` = cubic-bezier(.4, 0, .2, 1) 的求值器
// （`--ds-ease-in-out` 也是这一条，见结构树 §动效）。
func easeInOut(t float32) float32 {
	return cubicBezierY(0.4, 0, 0.2, 1, clamp01(t))
}

// easeOut 是 CSS `ease-out` = cubic-bezier(0, 0, .58, 1)。
// 官方 `._indicator` 的入场/离场都是这一条（`animation:_indicator-enter .15s ease-out`
// 与 `transition:…opacity .15s ease-out`）。
func easeOut(t float32) float32 {
	return cubicBezierY(0, 0, 0.58, 1, clamp01(t))
}

// cubicBezierY 解出 CSS cubic-bezier(x1,y1,x2,y2) 在给定时间 t 的进度。
//
// 先按 x 反解参数 u（牛顿迭代 + 二分兜底），再按 u 求 y——这正是浏览器
// 对 timing function 的定义，不能直接把 t 当 u 用（否则 0.5 处差 8%）。
func cubicBezierY(x1, y1, x2, y2, t float32) float32 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	bezier := func(u, p1, p2 float32) float32 {
		// B(u) = 3(1-u)²u·p1 + 3(1-u)u²·p2 + u³（端点 0/1）
		oneMinus := 1 - u
		return 3*oneMinus*oneMinus*u*p1 + 3*oneMinus*u*u*p2 + u*u*u
	}
	slope := func(u, p1, p2 float32) float32 {
		oneMinus := 1 - u
		return 3*oneMinus*oneMinus*p1 + 6*oneMinus*u*(p2-p1) + 3*u*u*(1-p2)
	}
	u := t
	for i := 0; i < 8; i++ {
		x := bezier(u, x1, x2) - t
		if math.Abs(float64(x)) < 1e-6 {
			return bezier(u, y1, y2)
		}
		derivative := slope(u, x1, x2)
		if math.Abs(float64(derivative)) < 1e-6 {
			break
		}
		u -= x / derivative
	}
	lo, hi := float32(0), float32(1)
	u = t
	for i := 0; i < 32; i++ {
		x := bezier(u, x1, x2)
		if math.Abs(float64(x-t)) < 1e-6 {
			break
		}
		if x < t {
			lo = u
		} else {
			hi = u
		}
		u = (lo + hi) / 2
	}
	return bezier(u, y1, y2)
}

// clamp01 把进度夹到 [0,1]。
func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
