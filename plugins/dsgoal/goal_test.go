package dsgoal

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// dsgoal 以前一个测试都没有。它的全部逻辑就是**一条守卫**：
// 没有目标就整块不画。守卫生效时看起来"什么都没有"，失效时看起来
// "本来就该有东西"——所以两条路都得断言，否则这个插件等于没被测。

// renderGoal 画一帧目标条，返回这一帧的画布（矩形/三角形/文字都在上面）。
func renderGoal(t *testing.T, p *Plugin, act pluginapi.Actions) *gfx.Canvas {
	t.Helper()
	if p.theme == nil {
		p.theme = dscore.Dark()
	}
	tree := &gfx.Tree{}
	feat := tree.Feature("goal-test")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: p.theme, Data: pluginapi.Snapshot{Cur: "s"}, Act: act,
		Tree: tree, Feat: feat, Canvas: canvas,
		Bounds: ui.Rect{Width: 1182, Height: 832}, MainWidth: 902,
	}
	var view ui.Page
	view.Draw(canvas, dscore.VivaTheme(p.theme), env.Bounds, func(f ui.C) {
		p.draw(f, env)
	})
	return canvas
}

// TestDrawsNothingWithoutGoalCapability 钉住老宿主那条路。
func TestDrawsNothingWithoutGoalCapability(t *testing.T) {
	canvas := renderGoal(t, &Plugin{}, pluginapi.Actions{})
	if rects := canvas.Rectangles(); len(rects) != 0 {
		t.Fatalf("壳没接 Goal 时不该画任何东西，画了 %+v", rects)
	}
	if tris := canvas.Triangles(); len(tris) != 0 {
		t.Fatalf("壳没接 Goal 时连图标都不该画，画了 %d 个三角形", len(tris))
	}
}

// TestDrawsNothingForEmptyGoal 是这条插件最要紧的一条。
//
// "没有目标"不是一种状态，是"还没有这回事"——画一条空白的横条会占掉一行高度
// 却什么都不说。它和 dsterminal 的「有终端但没输出要画」是**相反**的决定：
// 那边"空着"本身是个信息，这边不是。
func TestDrawsNothingForEmptyGoal(t *testing.T) {
	canvas := renderGoal(t, &Plugin{}, pluginapi.Actions{Goal: func() string { return "" }})
	if rects := canvas.Rectangles(); len(rects) != 0 {
		t.Fatalf("目标为空时不该画任何东西，画了 %+v", rects)
	}
	if tris := canvas.Triangles(); len(tris) != 0 {
		t.Fatalf("目标为空时连图标都不该画，画了 %d 个三角形", len(tris))
	}
}

// TestDrawsOfficialGoalGlyph 是有目标那一侧。
//
// 目标条上唯一的图形是官方 sprite `goal-outline` 那枚 14px glyph（文字是文字）。
// 判据三条：
//
//	① **不再有 4×4 圆点**（那是一枚自造占位，官方无此物）；
//	② 真的画了描边图形（icon 的描边被三角化 → Triangles 非空）；
//	③ 文字仍是「目标」+ 目标正文。
func TestDrawsOfficialGoalGlyph(t *testing.T) {
	canvas := renderGoal(t, &Plugin{}, pluginapi.Actions{
		Goal: func() string { return "把 DSH 重写成 Go 版本" },
	})
	if rects := canvas.Rectangles(); len(rects) != 0 {
		t.Fatalf("目标条已改用官方 sprite 图标，不该再有圆点矩形：%+v", rects)
	}
	if len(canvas.Triangles()) == 0 {
		t.Fatal("官方 goal-outline 图标没画出来（没有任何描边三角形）")
	}
	var label, objective bool
	for _, text := range canvas.Texts() {
		if text.Value == "目标" {
			label = true
		}
		if text.Value == "把 DSH 重写成 Go 版本" {
			objective = true
		}
	}
	if !label || !objective {
		t.Fatalf("目标条文字不全：label=%v objective=%v texts=%+v", label, objective, canvas.Texts())
	}
}

// TestGoalIsReadEveryFrame 钉住"目标变了会跟着变"。
//
// Goal 是个闭包而不是快照字段，很容易被谁改成"注册时读一次"——
// 那样改完目标界面不刷新，而现象是"设了新目标但上面还是旧的"。
func TestGoalIsReadEveryFrame(t *testing.T) {
	goal := ""
	act := pluginapi.Actions{Goal: func() string { return goal }}
	p := &Plugin{}

	if canvas := renderGoal(t, p, act); len(canvas.Triangles()) != 0 {
		t.Fatalf("一开始没有目标，不该画图标")
	}
	goal = "先出方案"
	if canvas := renderGoal(t, p, act); len(canvas.Triangles()) == 0 {
		t.Fatalf("设了目标之后应当画出图标")
	}
	goal = ""
	if canvas := renderGoal(t, p, act); len(canvas.Triangles()) != 0 {
		t.Fatalf("清掉目标之后应当整块消失，仍有 %d 个三角形", len(canvas.Triangles()))
	}
}
