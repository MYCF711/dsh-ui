package dsgoal

// icons_test.go —— 钉住「目标条那枚图标是官方 sprite，不是自造图形」。
//
// 这条断言存在的理由：这一格原来是**手画的 4px 圆点**，注释还写着
// "官方没有这枚 glyph"。事实相反——官方 `GoalBar.tsx:137` 用的就是
// `IconGoalOutlineRegular`，rc.2 里它是 sprite 表的一条 `goal-outline`
// （viewBox 16、regular 权重 = stroke-width 1）。所以这里断言的四个数
// （viewBox / stroke / 段数 / 首点）全部来自 sprite 原文，改任何一个都该红。

import (
	"math"
	"testing"

	"github.com/vivaui/viva/ui"
)

// iconInk 是图标**着墨范围**相对图标框的偏移与尺寸。
//
// 官方 sprite 的墨迹不铺满 viewBox（`goal-outline` 落在 1.5..14.5 之间），
// 所以"画出来的三角形包围盒"要按 `size/viewBox` 缩放后再加一个线宽才对得上。
// 用它当期望值，而不是把截图里的像素写死（写死就等于按截图调测试）。
//
// 返回 (dx, dy, w, h)：墨迹左上角相对图标框左上角的偏移，以及墨迹宽高。
func iconInk(icon ui.Icon, size float32) (float32, float32, float32, float32) {
	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	visit := func(p ui.Point) {
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	for _, command := range icon.Commands {
		// Close 命令没有坐标（Point 是零值）：把它当点算会把包围盒拉到原点。
		if command.Kind == ui.PathCommandClose {
			continue
		}
		visit(command.Point)
		switch command.Kind {
		case ui.PathCommandQuad:
			visit(command.Control1)
		case ui.PathCommandCubic:
			visit(command.Control1)
			visit(command.Control2)
		}
	}
	scale := size / icon.ViewBox
	stroke := icon.Stroke * scale
	// 描边在轮廓两侧各溢出半个线宽。
	return minX*scale - stroke/2, minY*scale - stroke/2,
		(maxX-minX)*scale + stroke, (maxY-minY)*scale + stroke
}

func TestGoalGlyphIsOfficialSprite(t *testing.T) {
	icon := goalOutlineIcon()

	// `"goal-outline":{size:16,svg:e=>`<svg … viewBox="0 0 16 16" fill="none"
	// stroke-width="${e}">` —— viewBox 是 16，不是 14（14 是显示尺寸）。
	if icon.ViewBox != 16 {
		t.Fatalf("viewBox = %v，want 16（官方 sprite `goal-outline` 的 viewBox）", icon.ViewBox)
	}
	// `p7={regular:1,…}` → stroke-width="1"。
	if icon.Stroke != 1 {
		t.Fatalf("stroke = %v，want 1（官方 regular 权重）", icon.Stroke)
	}
	// Cap/Join 的零值就是 SVG 默认（butt / miter）：官方 svg 没写 linecap/linejoin。
	if icon.Cap != ui.CapButt || icon.Join != ui.JoinMiter {
		t.Fatalf("Cap/Join = %v/%v，want butt/miter（官方 svg 未声明，即默认值）", icon.Cap, icon.Join)
	}

	var moves, lines, cubics, closes int
	for _, command := range icon.Commands {
		switch command.Kind {
		case ui.PathCommandMove:
			moves++
		case ui.PathCommandLine:
			lines++
		case ui.PathCommandCubic:
			cubics++
		case ui.PathCommandClose:
			closes++
		default:
			t.Fatalf("出现官方 sprite 里没有的命令种类：%v", command.Kind)
		}
		// 所有坐标必须落在官方 viewBox 里（0..16）；自造/错抄的路径经常越界。
		for _, point := range []ui.Point{command.Point, command.Control1, command.Control2} {
			if point.X < -0.001 || point.X > 16.001 || point.Y < -0.001 || point.Y > 16.001 {
				t.Fatalf("坐标 %+v 超出官方 viewBox 0..16", point)
			}
		}
	}
	// 官方 path 是 4 段：外环 / 内环 / 指针 / 箭头头。四段 => 4 个 MoveTo。
	if moves != 4 || closes != 1 {
		t.Fatalf("轮廓数 = %d（Move）/ %d（Close），want 4 / 1（官方 4 段 path）", moves, closes)
	}
	if lines != 7 || cubics != 22 {
		t.Fatalf("命令数 = %d 直线 / %d 三次曲线，want 7 / 22（官方 sprite 原文：1 段 8 曲线、1 段 8 曲线、1 段直线、1 段 6 曲线 6 直线）",
			lines, cubics)
	}

	// 首点是官方 `d` 的第一个坐标：`M14.5001 8`。
	first := icon.Commands[0]
	if first.Kind != ui.PathCommandMove || first.Point.X != 14.5001 || first.Point.Y != 8 {
		t.Fatalf("首命令 = %+v，want MoveTo(14.5001, 8)（官方 path #1 的起点）", first)
	}

	// 显示尺寸是官方的 `size={14}`，而 viewBox 仍是 16——两者不能混。
	if goalGlyphSize != 14 {
		t.Fatalf("goalGlyphSize = %v，want 14（GoalBar.tsx:137 的 size={14}）", goalGlyphSize)
	}
}
