package dsassets

import "math"

// 本文件把 **SVG 的椭圆弧**换算成 viva 路径词汇里有的三次贝塞尔。
//
// # 为什么这不是"近似凑合"
//
// 圆弧（以及任意椭圆弧）**不能用一段三次贝塞尔精确表示**——这是代数事实：三次贝塞尔
// 的有理参数形式是二次的，圆是二次曲线但没有非有理的参数化。但把它切成 **≤ 90° 的段**
// 之后，每段用一条三次贝塞尔逼近，误差是**可计算、可事先给界**的（见
// `TestQuarterArcKIsTheDerivedConstant` 与 `TestArcRadialErrorStaysUnderCriterion`：
// 每段径向误差 < 0.03% of r）。这与"看着差不多"的差别在于：**界是数学给出来的**，
// 不是"我画完觉得像"。SVG 规范自己也是这么定义的（SVG 1.1 Appendix F.6），浏览器
// 渲染圆弧时做的正是这件事。
//
// # k = 4/3 · tan(π/8) = 0.5522847498307936 是数学常数，不是设计值
//
// 它不是从某个图标里量出来的、也不是调参调出来的。推导是三步：
//
//  1. 让三次贝塞尔在起点/终点与四分之一圆弧**相切**（端点处切线方向一致）；
//  2. 让中点 t=1/2 落在圆上（这就是"中点误差为 0"的那个条件）；
//  3. 由对称性，两个控制点分别落在两条端切线、距端点同一距离 k·r 处。
//
// 解出来 k = 4/3·tan(π/8) = 4/3·(√2−1) = 0.5522847498307936…
//
// 一般角度 Δθ 的段用同一个推导，只是 k 换成 `4/3·tan(Δθ/4)`——四分之一圆是
// Δθ = π/2 的特例。**所以代码里没有手调的魔数**：`arcQuarterK` 只在测试里被
// `math.Tan(math.Pi/8)` 现场复算核对，几何计算本身一律走 `arcTangentScale`。

// arcQuarterK 是四分之一圆弧 → 一段三次贝塞尔的控制点系数。
//
// k = 4/3 · tan(π/8) = 0.5522847498307936…
//
// 来源见文件头：由"端点相切 + 中点落圆 + 对称"唯一确定，等价于 SVG 1.1 规范
// Appendix F.6 的弧→贝塞尔推导。**这是数学常数**——不是设计值、不是实测拟合值、
// 更不是本仓自造的。任何"把它调成 0.55 更顺眼"的改动都是把几何画错。
const arcQuarterK = 0.5522847498307936

// arcTangentScale 返回扫过 dTheta 弧度的一段弧所用的控制点缩放系数。
//
// 就是 4/3·tan(Δθ/4)；Δθ = π/2 时它退化成 arcQuarterK。分成 ≤π/2 的段之后
// 每段的这个系数都不超过四分之一圆那一档，径向误差也就不会超过它。
func arcTangentScale(dTheta float64) float64 {
	return 4.0 / 3.0 * math.Tan(dTheta/4)
}

// arcAction 是一条 `A` 命令换算后的三种去向（SVG 1.1 F.6.2 的退化规则）。
type arcAction uint8

const (
	// arcCubic 是真弧：按 Segments 逐段画三次贝塞尔。
	arcCubic arcAction = iota
	// arcLine 是半径退化为 0（或负）：按规范画一条直线到终点。
	arcLine
	// arcSkip 是起止点重合：按规范整段省略，什么都不画。
	arcSkip
)

// cubicSegment 是一段三次贝塞尔的绝对坐标（两个控制点 + 终点）。
//
// 起点由上一段决定（第一段由当前点决定），所以这里不存起点——与 viva 的
// `gfx.CubicTo` 一样，几何是"接着画"的。
type cubicSegment struct {
	C1X, C1Y float64
	C2X, C2Y float64
	X, Y     float64
}

// arcToCubics 把 **SVG 端点式椭圆弧**换算成一组 ≤90° 的三次贝塞尔段。
//
// 参数就是 SVG `A` 命令的七个参数（起止点、两半径、x 轴旋转角、large-arc、sweep）。
// 换算按 SVG 1.1 Appendix F.6.5（端点式 → 圆心式）与 F.6.6（半径不足时放大）逐步做，
// 再用参数方程求每段端点与切向、乘 arcTangentScale(Δθ) 得两个控制点。
//
// 三条退化规则（规范原文 F.6.2）由返回的 arcAction 表达：
// 起止点重合 → arcSkip；半径为 0 → arcLine；其余都是 arcCubic。
//
// 最后一段的终点**吸附**到调用方给的 (x2,y2)：规范保证数学上它就该落在那里，
// 但浮点累加会留下 1e-16 量级的漂移，而"这条弧的终点就是这条命令的终点"是
// 调用方要拿来当锚点的性质（否则后续 `L` 会从漂移后的位置起算）。
func arcToCubics(x1, y1, x2, y2, rx, ry, phiDeg float64, largeArc, sweep bool) ([]cubicSegment, arcAction) {
	if x1 == x2 && y1 == y2 {
		return nil, arcSkip
	}
	rx, ry = math.Abs(rx), math.Abs(ry)
	if rx == 0 || ry == 0 {
		return nil, arcLine
	}

	// ── F.6.5 第 1 步：把椭圆转正，并换到以"端点中点"为原点的坐标系。
	phi := phiDeg * math.Pi / 180
	cosPhi, sinPhi := math.Cos(phi), math.Sin(phi)
	dx2 := (x1 - x2) / 2
	dy2 := (y1 - y2) / 2
	x1p := cosPhi*dx2 + sinPhi*dy2
	y1p := -sinPhi*dx2 + cosPhi*dy2

	// ── F.6.6：半径太小放不下这两个端点时按比例放大。
	// 这是规范要求的修正（浏览器同样这么做），不是"容错兜底"。
	lambda := x1p*x1p/(rx*rx) + y1p*y1p/(ry*ry)
	if lambda > 1 {
		s := math.Sqrt(lambda)
		rx *= s
		ry *= s
	}

	// ── F.6.5 第 2 步：求转正坐标系里的圆心。
	rx2, ry2 := rx*rx, ry*ry
	den := rx2*y1p*y1p + ry2*x1p*x1p
	coef := 0.0
	if den > 0 {
		coef = math.Sqrt(math.Max(0, (rx2*ry2-rx2*y1p*y1p-ry2*x1p*x1p)/den))
	}
	// large-arc 与 sweep 同号取负号，异号取正号（规范 F.6.5 的符号规则）。
	if largeArc == sweep {
		coef = -coef
	}
	cxp := coef * rx * y1p / ry
	cyp := -coef * ry * x1p / rx

	// 转回原坐标系。
	cx := cosPhi*cxp - sinPhi*cyp + (x1+x2)/2
	cy := sinPhi*cxp + cosPhi*cyp + (y1+y2)/2

	// ── F.6.5 第 3/4 步：起始角与扫过的角度。
	theta1 := math.Atan2((y1p-cyp)/ry, (x1p-cxp)/rx)
	theta2 := math.Atan2((-y1p-cyp)/ry, (-x1p-cxp)/rx)
	dTheta := theta2 - theta1
	if !sweep && dTheta > 0 {
		dTheta -= 2 * math.Pi
	}
	if sweep && dTheta < 0 {
		dTheta += 2 * math.Pi
	}

	// 每段不超过 90°：这是 arcTangentScale 的适用区间，也是误差界的适用范围。
	//
	// 减 1e-9 是**必要的**：恰好 90° 的弧（四分之一圆、圆角矩形的每个角）在浮点里
	// 会算成 π/2 + 1e-16，ceil 于是给出 2 段——多切一刀不但没必要，还会让"圆的命令数
	// 是 6 条"这种可核对的数字随机器抖动。1e-9 远大于浮点噪声、远小于任何真实的
	// 分段需求（90.0000001° 与 90° 在 16px 图标上差 3e-7 px）。
	n := int(math.Ceil(math.Abs(dTheta)/(math.Pi/2) - 1e-9))
	if n < 1 {
		n = 1
	}
	step := dTheta / float64(n)
	alpha := arcTangentScale(step)

	segs := make([]cubicSegment, 0, n)
	theta := theta1
	for k := 0; k < n; k++ {
		next := theta + step
		p0x, p0y := ellipsePoint(cx, cy, rx, ry, cosPhi, sinPhi, theta)
		p1x, p1y := ellipsePoint(cx, cy, rx, ry, cosPhi, sinPhi, next)
		d0x, d0y := ellipseDerivative(rx, ry, cosPhi, sinPhi, theta)
		d1x, d1y := ellipseDerivative(rx, ry, cosPhi, sinPhi, next)
		segs = append(segs, cubicSegment{
			// 起点切线方向 × alpha，终点切线方向 × alpha 取反（控制点在端点的外侧）。
			C1X: p0x + alpha*d0x, C1Y: p0y + alpha*d0y,
			C2X: p1x - alpha*d1x, C2Y: p1y - alpha*d1y,
			X: p1x, Y: p1y,
		})
		theta = next
	}
	// 吸附终点：数学上它就该等于 (x2,y2)，这里去掉浮点漂移。
	segs[len(segs)-1].X, segs[len(segs)-1].Y = x2, y2
	return segs, arcCubic
}

// ellipsePoint 返回椭圆在参数角 theta 处的点。
//
// 椭圆 = 圆心 + R(φ)·(rx·cosθ, ry·sinθ)；θ 是**参数角**，不是几何极角
// （rx≠ry 时两者不同，混用是这类换算最常见的错）。
func ellipsePoint(cx, cy, rx, ry, cosPhi, sinPhi, theta float64) (float64, float64) {
	ct, st := math.Cos(theta), math.Sin(theta)
	return cx + rx*ct*cosPhi - ry*st*sinPhi,
		cy + rx*ct*sinPhi + ry*st*cosPhi
}

// ellipseDerivative 返回椭圆参数方程对 theta 的导数（切向，未归一化）。
func ellipseDerivative(rx, ry, cosPhi, sinPhi, theta float64) (float64, float64) {
	ct, st := math.Cos(theta), math.Sin(theta)
	return -rx*st*cosPhi - ry*ct*sinPhi,
		-rx*st*sinPhi + ry*ct*cosPhi
}
