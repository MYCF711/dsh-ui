package dsassets

import (
	"math"

	"github.com/vivaui/viva/gfx"
)

// 本文件把官方的**非路径图元**（`<rect rx>`、`<circle>`、`<ellipse>`）换算成 viva
// 路径命令。几何全部走 arc.go 的弧→贝塞尔换算——本文件不另写一份圆弧代码，
// 圆角矩形的四个角就是四条四分之一椭圆弧，圆就是四段四分之一圆弧。
//
// 这样"圆的误差界"与"圆角矩形的误差界"是**同一个界**（每段径向误差 < 0.03% of r），
// 不会出现两条各自调参的路径。测试见 arc_test.go。

// elementCommands 把一条登记图元换算成 viva 路径命令。
//
// 返回 false 表示这条图元换算不出来（路径里有不支持的命令、半径不合法等）。
// 调用方不许拿近似形状顶上去——那正是本包存在的理由。
func elementCommands(el Element) ([]gfx.PathCommand, bool) {
	switch el.Kind {
	case PathElement:
		if el.D == "" {
			return nil, false
		}
		return ParsePath(el.D)
	case RectElement:
		return roundedRectCommands(
			float64(el.X), float64(el.Y), float64(el.Width), float64(el.Height),
			float64(el.RX), float64(el.RY),
		)
	case CircleElement:
		rx, ry := float64(el.RX), float64(el.RY)
		if rx <= 0 && ry <= 0 {
			// 官方这套图标里只有 `<circle r>`；`<ellipse>` 走 RX/RY。
			rx, ry = float64(el.R), float64(el.R)
		}
		if rx <= 0 && ry > 0 {
			rx = ry
		}
		if ry <= 0 && rx > 0 {
			ry = rx
		}
		return ellipseCommands(float64(el.CX), float64(el.CY), rx, ry)
	}
	return nil, false
}

// roundedRectCommands 把官方 `<rect x y width height rx ry>` 换算成圆角矩形路径。
//
// # 语义（SVG 1.1 §9.2）
//
//   - `ry` 没写（=0）时取 `rx`；`rx` 没写时取 `ry`；两者都没写就是直角矩形。
//   - `rx`/`ry` 各自被钳到**半边长**：`rx > w/2` 取 `w/2`，`ry > h/2` 取 `h/2`。
//     浏览器就是这么画的，不钳会让两段圆弧交叉、矩形自交。
//   - 负半径按 0 处理（规范：负值是错误值，按不合法忽略 → 退化成直角）。
//
// # 路径顺序
//
// 从上边中点左侧起笔，**顺时针**（屏幕坐标 y 向下）绕一圈：
// 上边 → 右上角 → 右边 → 右下角 → 下边 → 左下角 → 左边 → 左上角 → 闭合。
// 每个角是一条 `large-arc=0, sweep=1` 的四分之一椭圆弧，所以四个角方向一致、
// 不会出现"两个角外凸两个角内凹"那种典型错法。
func roundedRectCommands(x, y, w, h, rx, ry float64) ([]gfx.PathCommand, bool) {
	if !(w > 0) || !(h > 0) {
		return nil, false
	}
	if rx < 0 {
		rx = 0
	}
	if ry < 0 {
		ry = 0
	}
	if rx == 0 && ry != 0 {
		rx = ry
	}
	if ry == 0 && rx != 0 {
		ry = rx
	}
	rx = math.Min(rx, w/2)
	ry = math.Min(ry, h/2)

	if rx == 0 || ry == 0 {
		// 直角矩形：四条边，没有弧。
		return []gfx.PathCommand{
			gfx.MoveTo(f32(x), f32(y)),
			gfx.LineTo(f32(x+w), f32(y)),
			gfx.LineTo(f32(x+w), f32(y+h)),
			gfx.LineTo(f32(x), f32(y+h)),
			gfx.ClosePath(),
		}, true
	}

	out := make([]gfx.PathCommand, 0, 10)
	out = append(out, gfx.MoveTo(f32(x+rx), f32(y)))
	corners := [4]struct{ fromX, fromY, toX, toY float64 }{
		{x + w - rx, y, x + w, y + ry},         // 右上角
		{x + w, y + h - ry, x + w - rx, y + h}, // 右下角
		{x + rx, y + h, x, y + h - ry},         // 左下角
		{x, y + ry, x + rx, y},                 // 左上角
	}
	for _, c := range corners {
		// 角与角之间那条直边（第一段的上边在 MoveTo 之后、弧之前）。
		out = append(out, gfx.LineTo(f32(c.fromX), f32(c.fromY)))
		segs, action := arcToCubics(c.fromX, c.fromY, c.toX, c.toY, rx, ry, 0, false, true)
		if action != arcCubic {
			// rx、ry 都已钳到 (0, w/2]/[0, h/2]，这里不可能退化；真退化了说明
			// 上游算错了，宁可整条失败也不画一个缺角的矩形。
			return nil, false
		}
		for _, s := range segs {
			out = append(out, cubicCommand(s))
		}
	}
	return append(out, gfx.ClosePath()), true
}

// ellipseCommands 把 `<circle cx cy r>` / `<ellipse cx cy rx ry>` 换算成椭圆路径。
//
// 四段四分之一弧、顺时针（屏幕坐标 y 向下），起笔在**最右点** `(cx+rx, cy)`。
// 起点选最右点是 SVG 圈内一致的做法（也是本仓圆弧误差测试的参照角 0），
// 换成最上点几何上等价、但会让"第 0 段从哪个角开始"随实现漂移，没必要。
func ellipseCommands(cx, cy, rx, ry float64) ([]gfx.PathCommand, bool) {
	if !(rx > 0) || !(ry > 0) {
		return nil, false
	}
	pts := [4][2]float64{
		{cx + rx, cy}, {cx, cy + ry}, {cx - rx, cy}, {cx, cy - ry},
	}
	out := make([]gfx.PathCommand, 0, 6)
	out = append(out, gfx.MoveTo(f32(pts[0][0]), f32(pts[0][1])))
	for i := 0; i < 4; i++ {
		from, to := pts[i], pts[(i+1)%4]
		segs, action := arcToCubics(from[0], from[1], to[0], to[1], rx, ry, 0, false, true)
		if action != arcCubic {
			return nil, false
		}
		for _, s := range segs {
			out = append(out, cubicCommand(s))
		}
	}
	return append(out, gfx.ClosePath()), true
}

// cubicCommand 把一段换算好的三次贝塞尔转成 viva 命令（float32 落库）。
func cubicCommand(s cubicSegment) gfx.PathCommand {
	return gfx.CubicTo(f32(s.C1X), f32(s.C1Y), f32(s.C2X), f32(s.C2Y), f32(s.X), f32(s.Y))
}

// f32 是换算里唯一的精度收窄点：几何全部用 float64 算完再落到 viva 的 float32。
//
// 放在一个具名函数里而不是到处写 `float32(...)`，是为了让"哪里掉了精度"只有一处。
func f32(v float64) float32 { return float32(v) }
