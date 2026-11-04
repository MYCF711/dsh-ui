package dsgoal

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// dsgoal 以前一个测试都没有。它的全部逻辑就是**一条守卫**：
// 没有目标就整块不画。守卫生效时看起来"什么都没有"，失效时看起来
// "本来就该有东西"——所以两条路都得断言，否则这个插件等于没被测。

// renderGoal 画一帧目标条，返回这一帧画出来的矩形。
func renderGoal(t *testing.T, p *Plugin, act pluginapi.Actions) []ui.Rectangle {
	t.Helper()
	if p.theme == nil {
		p.theme = dscore.Dark()
	}
	tree := &ui.Tree{}
	feat := tree.Feature("goal-test")
	canvas := &ui.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: p.theme, Data: pluginapi.Snapshot{Cur: "s"}, Act: act,
		Tree: tree, Feat: feat, Canvas: canvas,
		Bounds: ui.Rect{Width: 1182, Height: 832}, MainWidth: 902,
	}
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(p.theme), env.Bounds, func(f *flex.Frame) {
		p.draw(f, env)
	})
	return canvas.Rectangles()
}

// TestDrawsNothingWithoutGoalCapability 钉住老宿主那条路。
func TestDrawsNothingWithoutGoalCapability(t *testing.T) {
	rects := renderGoal(t, &Plugin{}, pluginapi.Actions{})
	if len(rects) != 0 {
		t.Fatalf("壳没接 Goal 时不该画任何东西，画了 %+v", rects)
	}
}

// TestDrawsNothingForEmptyGoal 是这条插件最要紧的一条。
//
// "没有目标"不是一种状态，是"还没有这回事"——画一条空白的横条会占掉一行高度
// 却什么都不说。它和 dsterminal 的「有终端但没输出要画」是**相反**的决定：
// 那边"空着"本身是个信息，这边不是。
func TestDrawsNothingForEmptyGoal(t *testing.T) {
	rects := renderGoal(t, &Plugin{}, pluginapi.Actions{Goal: func() string { return "" }})
	if len(rects) != 0 {
		t.Fatalf("目标为空时不该画任何东西，画了 %+v", rects)
	}
}

// TestDrawsOneDotWithAGoal 是有目标那一侧。
//
// 目标条上唯一被填色的东西就是那个 4×4 的圆点（文字不算矩形）。
// 所以"正好一个 4×4 的矩形"既是"画了"，也是"没有多余的框"。
func TestDrawsOneDotWithAGoal(t *testing.T) {
	rects := renderGoal(t, &Plugin{}, pluginapi.Actions{
		Goal: func() string { return "把 DSH 重写成 Go 版本" },
	})
	if len(rects) != 1 {
		t.Fatalf("有目标时应当正好一个圆点矩形，得到 %d 个：%+v", len(rects), rects)
	}
	got := rects[0].Bounds
	if got.Width != 4 || got.Height != 4 {
		t.Errorf("圆点尺寸 %vx%v，want 4x4", got.Width, got.Height)
	}
	// 左边 16px 是 PadSides(16,16,…) 的左边距：圆点贴左，文字在它后面。
	if got.X != 16 {
		t.Errorf("圆点左边距 %v，want 16（PadSides 的左内边距）", got.X)
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

	if rects := renderGoal(t, p, act); len(rects) != 0 {
		t.Fatalf("一开始没有目标，不该画")
	}
	goal = "先出方案"
	if rects := renderGoal(t, p, act); len(rects) != 1 {
		t.Fatalf("设了目标之后应当画出来，得到 %+v", rects)
	}
	goal = ""
	if rects := renderGoal(t, p, act); len(rects) != 0 {
		t.Fatalf("清掉目标之后应当整块消失，得到 %+v", rects)
	}
}
