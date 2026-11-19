package dsassets

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// 本文件是"弧 → 三次贝塞尔"的**精度判据**与语义判据。
//
// # 精度判据是怎么来的
//
// 一段扫过 Δθ 的椭圆弧用一条三次贝塞尔表达时，控制点取
//
//	α = 4/3 · tan(Δθ/4)
//
// 于是 t=0 与 t=1 处与弧相切、t=1/2 处落回弧上（这就是 α 的来源）。剩下的是
// **中间那段**的径向偏差。Δθ = π/2 时这个偏差的最大值是
//
//	≈ 2.73e-4 · r        （0.0273% of r）
//
// 也就是 16px 图标上 **0.0044px**——远低于一个像素的 1/200。
//
// 所以判据写成 **每段最大径向误差 < 0.03% of r**（3e-4），比理论值留了约 10% 余量：
// 既不会因为浮点噪声误红，也紧到"把换算改坏"必红。
//
// **这不是"看着像"**：判据是数学给的上界。测试里每个用例的**圆心是题目先给的**，
// 再从圆心算出起止点喂给换算，所以量出来的径向误差不依赖被测代码的任何中间量。

// arcRadialErrorCriterion 是每段弧换算的允许最大径向相对误差（0.03% of r）。
const arcRadialErrorCriterion = 3e-4

// arcTheoreticalMaxError 是 Δθ = π/2 那一档的理论最大径向相对误差。
//
// 测试用它给实测值划一个可信带：实测应当落在 [0.9, 1.1] 倍之内，否则说明换算
// 已经不是"每段 ≤90° 的标准换算"（改成了直线、改了系数、或段数被悄悄换掉）。
const arcTheoreticalMaxError = 2.7300e-4

// samplesPerSegment 是每段贝塞尔的采样点数。2 万点对 2.7e-4 量级的极值足够密
// （误差曲线在段内是光滑的单峰）。
const samplesPerSegment = 20000

// cubicAt 求一段三次贝塞尔在参数 t 处的点（起点由调用方给）。
func cubicAt(p0x, p0y float64, s cubicSegment, t float64) (float64, float64) {
	mt := 1 - t
	a := mt * mt * mt
	b := 3 * mt * mt * t
	c := 3 * mt * t * t
	d := t * t * t
	return a*p0x + b*s.C1X + c*s.C2X + d*s.X,
		a*p0y + b*s.C1Y + c*s.C2Y + d*s.Y
}

// maxNormalizedRadialError 在 [0,1] 上密集采样，返回最大的"归一化径向误差"。
//
// 归一化口径：把点换到椭圆自己的坐标系（先反旋 φ，再各除以 rx/ry），真椭圆于是
// 变成以圆心为心的单位圆，误差就是 |√(u²+v²) − 1|。rx≠ry 的椭圆与圆共用一条判据。
//
// 圆心与半径由调用方给出——它们是**题目条件**，不是从被测代码里取的。
func maxNormalizedRadialError(cx, cy, rx, ry, phiDeg, startX, startY float64, segs []cubicSegment) float64 {
	phi := phiDeg * math.Pi / 180
	cosPhi, sinPhi := math.Cos(phi), math.Sin(phi)
	worst := 0.0
	px, py := startX, startY
	for _, s := range segs {
		for i := 0; i <= samplesPerSegment; i++ {
			t := float64(i) / float64(samplesPerSegment)
			x, y := cubicAt(px, py, s, t)
			dx, dy := x-cx, y-cy
			u := cosPhi*dx + sinPhi*dy
			v := -sinPhi*dx + cosPhi*dy
			if e := math.Abs(math.Hypot(u/rx, v/ry) - 1); e > worst {
				worst = e
			}
		}
		px, py = s.X, s.Y
	}
	return worst
}

// ellipsePointAt 是测试侧的独立椭圆求点（只依赖题目给的圆心/半径/旋转/参数角）。
func ellipsePointAt(cx, cy, rx, ry, phiDeg, thetaDeg float64) (float64, float64) {
	phi := phiDeg * math.Pi / 180
	theta := thetaDeg * math.Pi / 180
	ct, st := math.Cos(theta), math.Sin(theta)
	return cx + rx*ct*math.Cos(phi) - ry*st*math.Sin(phi),
		cy + rx*ct*math.Sin(phi) + ry*st*math.Cos(phi)
}

// TestQuarterArcKIsTheDerivedConstant 钉住 k = 4/3·tan(π/8) 的**出处**。
//
// 这条防的不是算错，而是"后来者把它当成一个可调的设计值"。它必须等于
// math.Tan(math.Pi/8) 的 4/3——现场复算，不抄字面量。
func TestQuarterArcKIsTheDerivedConstant(t *testing.T) {
	derived := 4.0 / 3.0 * math.Tan(math.Pi/8)
	if math.Abs(arcQuarterK-derived) > 1e-15 {
		t.Fatalf("arcQuarterK = %.17g，但 4/3·tan(π/8) = %.17g（差 %g）", arcQuarterK, derived, arcQuarterK-derived)
	}
	// 也等于 4/3·(√2−1)：两条经典写法必须同值，说明来源是几何不是拟合。
	if alt := 4.0 / 3.0 * (math.Sqrt2 - 1); math.Abs(arcQuarterK-alt) > 1e-15 {
		t.Fatalf("arcQuarterK 与 4/3·(√2−1)=%.17g 不一致", alt)
	}
	// 一般式在 Δθ=π/2 时必须退化成这个常数。
	if got := arcTangentScale(math.Pi / 2); math.Abs(got-arcQuarterK) > 1e-15 {
		t.Fatalf("arcTangentScale(π/2) = %.17g，want arcQuarterK = %.17g", got, arcQuarterK)
	}
	// 四分之一圆弧换算出来的两个控制点必须就是 (1,k) 与 (k,1)。
	segs, action := arcToCubics(1, 0, 0, 1, 1, 1, 0, false, true)
	if action != arcCubic {
		t.Fatalf("四分之一圆弧的 action = %v，want arcCubic", action)
	}
	if len(segs) != 1 {
		t.Fatalf("四分之一圆弧段数 = %d，want 1", len(segs))
	}
	s := segs[0]
	near := func(got, want float64, what string) {
		if math.Abs(got-want) > 1e-12 {
			t.Errorf("%s = %.15g，want %.15g", what, got, want)
		}
	}
	near(s.C1X, 1, "C1X")
	near(s.C1Y, arcQuarterK, "C1Y")
	near(s.C2X, arcQuarterK, "C2X")
	near(s.C2Y, 1, "C2Y")
	near(s.X, 0, "X")
	near(s.Y, 1, "Y")
}

// TestArcRadialErrorStaysUnderCriterion 是**精度判据本身**。
//
// 每个用例先给圆心/半径/旋转/起止参数角（题目条件），再由它们算出起止点，
// 于是量出来的径向误差完全不依赖被测代码。覆盖：圆、官方节点的真实半径、
// rx≠ry 椭圆、带旋转的椭圆、半圆、>180° 的大弧、sweep=false 的反向弧。
func TestArcRadialErrorStaysUnderCriterion(t *testing.T) {
	cases := []struct {
		name           string
		cx, cy         float64
		rx, ry, phi    float64
		theta1, theta2 float64 // 度
		wantSegments   int
	}{
		{name: "四分之一单位圆弧", cx: 0, cy: 0, rx: 1, ry: 1, theta1: 0, theta2: 90, wantSegments: 1},
		// 官方分支图标两枚节点的真实半径（`<circle r=1.56962>`）。
		{name: "官方分支节点 r=1.56962", cx: 12.4502, cy: 3.3079, rx: 1.56962, ry: 1.56962, theta1: 0, theta2: 90, wantSegments: 1},
		// rx ≠ ry：10:3 的椭圆，四分之一弧。
		{name: "rx≠ry 椭圆 10:3", cx: 0, cy: 0, rx: 10, ry: 3, theta1: 0, theta2: 90, wantSegments: 1},
		// 带 30° x 轴旋转的椭圆。
		{name: "旋转 30° 的椭圆", cx: 5, cy: 5, rx: 6, ry: 2, phi: 30, theta1: 0, theta2: 90, wantSegments: 1},
		// 半圆：必须拆成两段 90°。
		{name: "半圆拆两段", cx: 1, cy: 0, rx: 1, ry: 1, theta1: 0, theta2: 180, wantSegments: 2},
		// 300° 大弧：必须拆成四段。
		{name: "300° 大弧拆四段", cx: 0, cy: 0, rx: 2, ry: 2, theta1: 0, theta2: 300, wantSegments: 4},
		// 反向（sweep=false）的 180° 弧，带负旋转角。
		{name: "反向弧 sweep=false", cx: 4, cy: 4, rx: 4, ry: 1.5, phi: -45, theta1: 200, theta2: 20, wantSegments: 2},
		// 45°→135° 的一小段。
		{name: "90° 非对齐起点", cx: 3, cy: 3, rx: 3, ry: 3, theta1: 45, theta2: 135, wantSegments: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dTheta := tc.theta2 - tc.theta1
			startX, startY := ellipsePointAt(tc.cx, tc.cy, tc.rx, tc.ry, tc.phi, tc.theta1)
			endX, endY := ellipsePointAt(tc.cx, tc.cy, tc.rx, tc.ry, tc.phi, tc.theta2)
			largeArc := math.Abs(dTheta) > 180
			sweep := dTheta > 0

			segs, action := arcToCubics(startX, startY, endX, endY, tc.rx, tc.ry, tc.phi, largeArc, sweep)
			if action != arcCubic {
				t.Fatalf("action = %v，want arcCubic", action)
			}
			if len(segs) != tc.wantSegments {
				t.Fatalf("段数 = %d，want %d（每段必须 ≤90°）", len(segs), tc.wantSegments)
			}
			worst := maxNormalizedRadialError(tc.cx, tc.cy, tc.rx, tc.ry, tc.phi, startX, startY, segs)
			if worst > arcRadialErrorCriterion {
				t.Fatalf("最大归一化径向误差 = %.3e，超过判据 %.3e（0.03%% of r）", worst, arcRadialErrorCriterion)
			}
			t.Logf("最大归一化径向误差 = %.4e（判据 %.0e，理论 %.2e）", worst, arcRadialErrorCriterion, arcTheoreticalMaxError)
		})
	}
}

// TestCircleRadialErrorStaysUnderCriterion 用最直接的圆判据复核，并把实测值钉在理论值附近。
//
// 这里量的是 |B(t) − C| − r（真径向误差），圆心与半径都是题目给的。
func TestCircleRadialErrorStaysUnderCriterion(t *testing.T) {
	for _, r := range []float64{1, 1.56962, 7.5} {
		cx, cy := 8.0, 8.0
		segs, action := arcToCubics(cx+r, cy, cx, cy+r, r, r, 0, false, true)
		if action != arcCubic || len(segs) != 1 {
			t.Fatalf("r=%v：action=%v 段数=%d", r, action, len(segs))
		}
		worst := 0.0
		for i := 0; i <= samplesPerSegment; i++ {
			tt := float64(i) / float64(samplesPerSegment)
			x, y := cubicAt(cx+r, cy, segs[0], tt)
			if e := math.Abs(math.Hypot(x-cx, y-cy)-r) / r; e > worst {
				worst = e
			}
		}
		if worst > arcRadialErrorCriterion {
			t.Errorf("r=%v 的最大径向误差 = %.3e，超过判据 %.3e", r, worst, arcRadialErrorCriterion)
		}
		// 实测必须落在理论值附近：明显更小说明段数/系数被改过，明显更大说明算错了。
		if worst < 0.9*arcTheoreticalMaxError || worst > 1.1*arcTheoreticalMaxError {
			t.Errorf("r=%v 的实测最大径向误差 = %.4e，偏离理论值 %.4e 太多", r, worst, arcTheoreticalMaxError)
		}
		t.Logf("r=%v 最大径向误差 = %.4e（%.2f%% of r）", r, worst, worst*100)
	}
}

// TestRoundedRectCornersAreExactWithinCriterion 验圆角矩形的四个角。
//
// 判据用的是圆角矩形的**定义性质**：rx==ry==r 时，圆角矩形的边界恰好是
// "到内矩形 [x+r,x+w−r]×[y+r,y+h−r] 的距离等于 r" 的点集。于是对路径上任意
// 采样点 P：
//
//	d(P, 内矩形) == r
//
// 直边、四个圆角、拐点都被这一条同时覆盖，比"分别检查四条弧"强得多，而且
// 完全不依赖换算内部怎么算。
func TestRoundedRectCornersAreExactWithinCriterion(t *testing.T) {
	cases := []struct {
		name       string
		x, y, w, h float64
		rx         float64
		wantCmds   int
	}{
		// 官方 composer-stop 的真实几何。
		{name: "composer-stop rect rx=3", x: 3, y: 3, w: 10, h: 10, rx: 3, wantCmds: 10},
		// 官方 copy 的真实几何。
		{name: "copy rect rx=2", x: 1.52075, y: 4.07373, w: 10.3932, h: 10.3932, rx: 2, wantCmds: 10},
		{name: "非正方形", x: 0, y: 0, w: 12, h: 5, rx: 2, wantCmds: 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmds, ok := roundedRectCommands(tc.x, tc.y, tc.w, tc.h, tc.rx, 0)
			if !ok {
				t.Fatal("圆角矩形换算失败")
			}
			if len(cmds) != tc.wantCmds {
				t.Fatalf("命令数 = %d，want %d", len(cmds), tc.wantCmds)
			}
			r := math.Min(tc.rx, math.Min(tc.w, tc.h)/2)
			innerMinX, innerMaxX := tc.x+r, tc.x+tc.w-r
			innerMinY, innerMaxY := tc.y+r, tc.y+tc.h-r
			worst := 0.0
			var px, py float64
			for i, c := range cmds {
				switch c.Kind {
				case gfx.PathCommandMove:
					px, py = float64(c.Point.X), float64(c.Point.Y)
				case gfx.PathCommandLine:
					nx, ny := float64(c.Point.X), float64(c.Point.Y)
					for k := 0; k <= 200; k++ {
						f := float64(k) / 200
						if e := rectBoundaryError(px+(nx-px)*f, py+(ny-py)*f, innerMinX, innerMinY, innerMaxX, innerMaxY, r); e > worst {
							worst = e
						}
					}
					px, py = nx, ny
				case gfx.PathCommandCubic:
					s := cubicSegment{
						C1X: float64(c.Control1.X), C1Y: float64(c.Control1.Y),
						C2X: float64(c.Control2.X), C2Y: float64(c.Control2.Y),
						X: float64(c.Point.X), Y: float64(c.Point.Y),
					}
					for k := 0; k <= samplesPerSegment; k++ {
						tt := float64(k) / float64(samplesPerSegment)
						x, y := cubicAt(px, py, s, tt)
						if e := rectBoundaryError(x, y, innerMinX, innerMinY, innerMaxX, innerMaxY, r); e > worst {
							worst = e
						}
					}
					px, py = s.X, s.Y
				case gfx.PathCommandClose:
					// Z 之后回到起点，无需再采样。
				default:
					t.Fatalf("第 %d 条命令种类 %v 不在预期内", i, c.Kind)
				}
			}
			if worst > arcRadialErrorCriterion {
				t.Fatalf("圆角矩形最大边界偏差 = %.3e·r，超过判据 %.3e", worst, arcRadialErrorCriterion)
			}
			t.Logf("圆角矩形最大边界偏差 = %.4e·r", worst)
		})
	}
}

// rectBoundaryError 返回点 P 到"内矩形距离"与 r 的相对偏差（见调用处）。
func rectBoundaryError(x, y, minX, minY, maxX, maxY, r float64) float64 {
	qx := math.Min(math.Max(x, minX), maxX)
	qy := math.Min(math.Max(y, minY), maxY)
	return math.Abs(math.Hypot(x-qx, y-qy)-r) / r
}

// TestArcFlagsSelectTheRightArc 钉住 large-arc / sweep 两个标志的语义。
//
// 同一条 `A` 命令换标志会得到完全不同的弧——这是"画出来像那么回事但形状错"的
// 高发处，所以四种组合都要有可判别的几何特征。
func TestArcFlagsSelectTheRightArc(t *testing.T) {
	// 弦长 2、半径 2：小弧扫 60°（1 段）、大弧扫 300°（4 段）。
	const x1, y1, x2, y2, r = 0.0, 0.0, 2.0, 0.0, 2.0

	small, act := arcToCubics(x1, y1, x2, y2, r, r, 0, false, true)
	if act != arcCubic || len(small) != 1 {
		t.Fatalf("小弧：action=%v 段数=%d，want 1 段（60° ≤ 90°）", act, len(small))
	}
	large, act := arcToCubics(x1, y1, x2, y2, r, r, 0, true, true)
	if act != arcCubic || len(large) != 4 {
		t.Fatalf("大弧：action=%v 段数=%d，want 4 段（300° 需拆 4 段）", act, len(large))
	}
	// 小弧弓高 = r(1−cos30°) ≈ 0.268，贴着弦；大弧跑到弦的另一侧、远得多。
	if got := maxAbsYOf(x1, y1, small); got > 0.5 {
		t.Errorf("小弧最大 |y| = %.3f，应当贴着弦（< 0.5）", got)
	}
	if got := maxAbsYOf(x1, y1, large); got < 1.5 {
		t.Errorf("大弧最大 |y| = %.3f，应当远离弦（> 1.5）", got)
	}

	// sweep 取反：两条半圆弧必须分居弦的两侧。
	up, act := arcToCubics(0, 0, 2, 0, 1, 1, 0, false, true)
	if act != arcCubic || len(up) != 2 {
		t.Fatalf("半圆 sweep=1：action=%v 段数=%d", act, len(up))
	}
	down, act := arcToCubics(0, 0, 2, 0, 1, 1, 0, false, false)
	if act != arcCubic || len(down) != 2 {
		t.Fatalf("半圆 sweep=0：action=%v 段数=%d", act, len(down))
	}
	// 半圆的段间连接点就是离弦最远的那一点：(1, ∓1)。
	if math.Abs(up[0].X-1) > 1e-12 || math.Abs(up[0].Y-(-1)) > 1e-12 {
		t.Errorf("sweep=1 的中继点 = (%.15f, %.15f)，want (1, −1)", up[0].X, up[0].Y)
	}
	if math.Abs(down[0].X-1) > 1e-12 || math.Abs(down[0].Y-1) > 1e-12 {
		t.Errorf("sweep=0 的中继点 = (%.15f, %.15f)，want (1, 1)", down[0].X, down[0].Y)
	}
}

// maxAbsYOf 返回一组段采样点上最大的 |y|。
func maxAbsYOf(startX, startY float64, segs []cubicSegment) float64 {
	worst := 0.0
	px, py := startX, startY
	for _, s := range segs {
		for i := 0; i <= samplesPerSegment; i++ {
			tt := float64(i) / float64(samplesPerSegment)
			_, y := cubicAt(px, py, s, tt)
			if math.Abs(y) > worst {
				worst = math.Abs(y)
			}
		}
		px, py = s.X, s.Y
	}
	return worst
}

// TestArcDegenerateActions 钉住 SVG 1.1 F.6.2 的两条退化规则。
//
// 起止点重合 → 整段省略（arcSkip，**不画**一条零长线）；
// 半径为 0 → 退化成直线（arcLine）。
// 把这两种混成"画一条线"或"画一条弧"都会在屏幕上多出一个点或一段怪线。
func TestArcDegenerateActions(t *testing.T) {
	if _, act := arcToCubics(3, 4, 3, 4, 2, 2, 0, false, true); act != arcSkip {
		t.Errorf("起止点重合：action = %v，want arcSkip", act)
	}
	if _, act := arcToCubics(0, 0, 5, 0, 0, 2, 0, false, true); act != arcLine {
		t.Errorf("rx=0：action = %v，want arcLine", act)
	}
	if _, act := arcToCubics(0, 0, 5, 0, 2, 0, 0, false, true); act != arcLine {
		t.Errorf("ry=0：action = %v，want arcLine", act)
	}
	// 负半径按绝对值处理（SVG 规范：负半径是"错误值"，但浏览器按 |r| 画）。
	a, act1 := arcToCubics(1, 0, 0, 1, -1, -1, 0, false, true)
	b, act2 := arcToCubics(1, 0, 0, 1, 1, 1, 0, false, true)
	if act1 != arcCubic || act2 != arcCubic || len(a) != len(b) {
		t.Fatalf("负半径：action=%v/%v 段数=%d/%d", act1, act2, len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("负半径第 %d 段 = %+v，want 与正半径一致 %+v", i, a[i], b[i])
		}
	}
}

// TestArcEndpointIsSnappedToCommandEnd 钉住"弧的终点就是命令的终点"。
//
// 调用方拿这条端点当锚点（后续 L 从这里起算）。浮点累加会留 1e-16 量级漂移，
// 所以换算最后一步必须吸附——否则一条长路径的终点会以肉眼看不见的方式挪走，
// 而 Bounds/命中区测试会以"偶尔差一点点"的形式抖动。
func TestArcEndpointIsSnappedToCommandEnd(t *testing.T) {
	segs, act := arcToCubics(1.01503, 8.0001, 11.3281, 4.40564, 3.3, 2.7, 17, true, false)
	if act != arcCubic || len(segs) == 0 {
		t.Fatalf("action=%v 段数=%d", act, len(segs))
	}
	last := segs[len(segs)-1]
	if last.X != 11.3281 || last.Y != 4.40564 {
		t.Fatalf("末段终点 = (%.17g, %.17g)，want 逐位等于命令终点 (11.3281, 4.40564)", last.X, last.Y)
	}
}

// TestParseArcCommandExpandsToCubics 验 `A` 命令在路径解析里真的变成了 C。
func TestParseArcCommandExpandsToCubics(t *testing.T) {
	cmds, ok := ParsePath("M0 0A1 1 0 0 1 2 0Z")
	if !ok {
		t.Fatal("带 A 的路径解析失败")
	}
	// M + 2×C（半圆拆两段）+ Z。
	if len(cmds) != 4 {
		t.Fatalf("命令数 = %d，want 4（M + 2C + Z）", len(cmds))
	}
	if cmds[0].Kind != gfx.PathCommandMove || cmds[3].Kind != gfx.PathCommandClose {
		t.Fatalf("首尾命令种类不对：%v / %v", cmds[0].Kind, cmds[3].Kind)
	}
	for i := 1; i <= 2; i++ {
		if cmds[i].Kind != gfx.PathCommandCubic {
			t.Fatalf("第 %d 条命令 = %v，want Cubic（弧必须换算成 C）", i, cmds[i].Kind)
		}
	}
	// 半圆终点必须精确回到 (2,0)。
	if math.Abs(float64(cmds[2].Point.X)-2) > 1e-6 || math.Abs(float64(cmds[2].Point.Y)) > 1e-6 {
		t.Errorf("半圆终点 = (%v, %v)，want (2, 0)", cmds[2].Point.X, cmds[2].Point.Y)
	}
}

// TestParseArcFlagsAcceptPackedForm：SVG 允许 `01` 这样把两个标志紧贴写。
//
// 用通用数字扫描会把 `01` 读成 1、把 `11` 读成 11——那会让大弧/小弧选反，
// 形状看着还在、但明显不是官方的弧。所以标志必须按**单字符**读。
func TestParseArcFlagsAcceptPackedForm(t *testing.T) {
	packed, ok := ParsePath("M0 0A2 2 0 012 0")
	if !ok {
		t.Fatal("紧贴标志写法解析失败")
	}
	spaced, ok := ParsePath("M0 0A2 2 0 0 1 2 0")
	if !ok {
		t.Fatal("空格分隔写法解析失败")
	}
	if len(packed) != len(spaced) {
		t.Fatalf("命令数不同：紧贴=%d 空格=%d", len(packed), len(spaced))
	}
	for i := range packed {
		if packed[i] != spaced[i] {
			t.Fatalf("第 %d 条命令不同：紧贴=%+v 空格=%+v", i, packed[i], spaced[i])
		}
	}
	// 大弧（large-arc=1）必须多出三段：M + 4C。
	large, ok := ParsePath("M0 0A2 2 0 112 0")
	if !ok {
		t.Fatal("紧贴大弧标志解析失败")
	}
	if len(large) != 1+4 {
		t.Fatalf("大弧命令数 = %d，want 5（M + 4C）", len(large))
	}
}

// TestEllipseCommandsShape 验 `<ellipse>` 的几何语义（圆心、半径、起笔点）。
func TestEllipseCommandsShape(t *testing.T) {
	cmds, ok := ellipseCommands(8, 8, 5, 2)
	if !ok {
		t.Fatal("椭圆换算失败")
	}
	if len(cmds) != 6 {
		t.Fatalf("命令数 = %d，want 6（M + 4C + Z）", len(cmds))
	}
	if cmds[0].Point.X != 13 || cmds[0].Point.Y != 8 {
		t.Errorf("起笔点 = (%v, %v)，want (13, 8)（最右点）", cmds[0].Point.X, cmds[0].Point.Y)
	}
	want := [4][2]float32{{8, 10}, {3, 8}, {8, 6}, {13, 8}}
	for i, w := range want {
		c := cmds[1+i]
		if math.Abs(float64(c.Point.X-w[0])) > 1e-6 || math.Abs(float64(c.Point.Y-w[1])) > 1e-6 {
			t.Errorf("第 %d 段终点 = (%v, %v)，want (%v, %v)", i, c.Point.X, c.Point.Y, w[0], w[1])
		}
	}
	if cmds[5].Kind != gfx.PathCommandClose {
		t.Errorf("末命令 = %v，want Close", cmds[5].Kind)
	}
}

// TestRoundedRectClampsRadii 钉住 SVG 1.1 §9.2 的钳制与回填规则。
func TestRoundedRectClampsRadii(t *testing.T) {
	// 直角矩形：rx=ry=0 → 5 条命令（M + 3L + Z）。
	plain, ok := roundedRectCommands(0, 0, 10, 10, 0, 0)
	if !ok || len(plain) != 5 {
		t.Fatalf("直角矩形：ok=%v 命令数=%d，want 5", ok, len(plain))
	}
	// rx 超过半边长 → 钳到 5（半圆端），命令数仍是 10。
	clamped, ok := roundedRectCommands(0, 0, 10, 10, 100, 0)
	if !ok || len(clamped) != 10 {
		t.Fatalf("钳制后：ok=%v 命令数=%d，want 10", ok, len(clamped))
	}
	if clamped[0].Point.X != 5 || clamped[0].Point.Y != 0 {
		t.Errorf("钳制后起笔点 = (%v, %v)，want (5, 0)（rx 被钳到 w/2）", clamped[0].Point.X, clamped[0].Point.Y)
	}
	// 只给 ry → rx 取同值（与只给 rx 的结果逐命令一致）。
	fromRY, _ := roundedRectCommands(0, 0, 10, 10, 0, 3)
	fromRX, _ := roundedRectCommands(0, 0, 10, 10, 3, 0)
	if len(fromRY) != len(fromRX) {
		t.Fatalf("只给 ry 与只给 rx 命令数不同：%d / %d", len(fromRY), len(fromRX))
	}
	for i := range fromRY {
		if fromRY[i] != fromRX[i] {
			t.Fatalf("第 %d 条命令不同：只给 ry=%+v 只给 rx=%+v", i, fromRY[i], fromRX[i])
		}
	}
	// 非法宽高 → 失败（不许画出一个退化图形）。
	if _, ok := roundedRectCommands(0, 0, 0, 10, 2, 0); ok {
		t.Error("宽度为 0 的矩形不该换算成功")
	}
}

// TestElementCommandsCoversCircleAndEllipse：登记表里 `CircleElement` 的两条分支。
//
// 官方现有两枚都是 `<circle r>`，`<ellipse rx ry>` 只有词汇支持、没有素材用到；
// 但"词汇支持"必须真的接通，否则将来加一枚椭圆素材时会静默走成圆。
// 所以这里直接喂 `Element`：`R` 一路与 `RX/RY` 一路都要走通，且 rx≠ry 时不能退化成圆。
func TestElementCommandsCoversCircleAndEllipse(t *testing.T) {
	// 只给 R：圆，四段四分之一弧。
	circle, ok := elementCommands(Element{Kind: CircleElement, CX: 8, CY: 8, R: 3})
	if !ok || len(circle) != 6 {
		t.Fatalf("`<circle r=3>`：ok=%v 命令数=%d，want 6", ok, len(circle))
	}
	if circle[0].Point.X != 11 || circle[0].Point.Y != 8 {
		t.Errorf("圆起笔点 = (%v, %v)，want (11, 8)", circle[0].Point.X, circle[0].Point.Y)
	}
	// rx≠ry：椭圆。第二段终点必须是 (cx, cy+ry) = (8,10)；rx/ry 互换会变成 (8,13)。
	ellipse, ok := elementCommands(Element{Kind: CircleElement, CX: 8, CY: 8, RX: 5, RY: 2})
	if !ok || len(ellipse) != 6 {
		t.Fatalf("`<ellipse rx=5 ry=2>`：ok=%v 命令数=%d，want 6", ok, len(ellipse))
	}
	want := [4][2]float32{{8, 10}, {3, 8}, {8, 6}, {13, 8}}
	for i, w := range want {
		c := ellipse[1+i]
		if math.Abs(float64(c.Point.X-w[0])) > 1e-6 || math.Abs(float64(c.Point.Y-w[1])) > 1e-6 {
			t.Errorf("椭圆第 %d 段终点 = (%v, %v)，want (%v, %v)", i, c.Point.X, c.Point.Y, w[0], w[1])
		}
	}
	// 只给 RY：按 SVG 规则 rx 取 ry，不能退化成 R。
	ryOnly, ok := elementCommands(Element{Kind: CircleElement, CX: 8, CY: 8, R: 9, RY: 2})
	if !ok {
		t.Fatal("只给 RY 的椭圆换算失败")
	}
	if ryOnly[0].Point.X != 10 || ryOnly[0].Point.Y != 8 {
		t.Errorf("只给 RY 的起笔点 = (%v, %v)，want (10, 8)（rx 应取 ry=2，而不是 R=9）", ryOnly[0].Point.X, ryOnly[0].Point.Y)
	}
}

// TestEllipseRejectsZeroRadius：半径为 0 的圆/椭圆画不出任何东西，必须失败而不是画点。
func TestEllipseRejectsZeroRadius(t *testing.T) {
	for _, c := range [][2]float64{{0, 0}, {0, 3}, {3, 0}, {-1, -1}} {
		if _, ok := ellipseCommands(8, 8, c[0], c[1]); ok {
			t.Errorf("rx=%v ry=%v 不该换算成功", c[0], c[1])
		}
	}
}

// TestBuiltIconsMeetCriterion 在**真正装进 gfx.Icon 的那份 float32 数据**上复验判据。
//
// 前面的测试量的是 float64 的换算结果；这里走完整条链
// （登记表 → elementCommands → gfx.PathCommand 的 float32），证明精度收窄之后
// 仍然满足判据——不然"换算很准、落库掉精度"就是白测。
func TestBuiltIconsMeetCriterion(t *testing.T) {
	// 分支图标的第二枚节点：官方 cx=12.4502 cy=12.6921 r=1.56962。
	icon, ok := Build(Branch, gfx.RGB(0, 0, 0))
	if !ok {
		t.Fatal("分支图标应当能构建")
	}
	const cx, cy, r = 12.4502, 12.6921, 1.56962
	worst := 0.0
	found := 0
	for i := 0; i+4 < len(icon.Commands); i++ {
		start := icon.Commands[i]
		if start.Kind != gfx.PathCommandMove {
			continue
		}
		sx, sy := float64(start.Point.X), float64(start.Point.Y)
		if math.Abs(sx-(cx+r)) > 1e-3 || math.Abs(sy-cy) > 1e-3 {
			continue
		}
		found++
		px, py := sx, sy
		for k := 1; k <= 4; k++ {
			c := icon.Commands[i+k]
			if c.Kind != gfx.PathCommandCubic {
				t.Fatalf("圆组第 %d 条不是 Cubic：%v", k, c.Kind)
			}
			s := cubicSegment{
				C1X: float64(c.Control1.X), C1Y: float64(c.Control1.Y),
				C2X: float64(c.Control2.X), C2Y: float64(c.Control2.Y),
				X: float64(c.Point.X), Y: float64(c.Point.Y),
			}
			for j := 0; j <= samplesPerSegment; j++ {
				tt := float64(j) / float64(samplesPerSegment)
				x, y := cubicAt(px, py, s, tt)
				if e := math.Abs(math.Hypot(x-cx, y-cy)-r) / r; e > worst {
					worst = e
				}
			}
			px, py = s.X, s.Y
		}
	}
	if found != 1 {
		t.Fatalf("在 Build 输出里找到 %d 组以 (12.4502+1.56962, 12.6921) 起笔的圆，want 1", found)
	}
	if worst > arcRadialErrorCriterion {
		t.Fatalf("落库后最大径向误差 = %.3e，超过判据 %.3e", worst, arcRadialErrorCriterion)
	}
	t.Logf("分支节点落库后最大径向误差 = %.4e", worst)
}
