package dsdemo

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// dsdemo 是**第三方插件的样例**——它示范的五段接口（ID / Requires / Register /
// Panels / draw）是别人照着抄的模板。样例错了，抄的人一起错。
//
// 所以这里测的不是"演示条好看不好看"，是那几条**会被抄走的行为**：
//   - F12 切换显隐，且别的键不抢；
//   - 收起之后**真的什么都不画**（不是画个透明的）；
//   - 「×」点了会收起。

func TestF12TogglesAndNothingElse(t *testing.T) {
	p := &Plugin{}
	p.shown = true

	// 别的键一律不认，必须让回去（抢了别人的键，别的插件就收不到）。
	for _, k := range []ui.Key{ui.KeyF1, ui.KeyEnter, ui.KeyEscape, ui.KeyDown} {
		if p.HandleKey(ui.KeyEvent{Key: k}) {
			t.Errorf("%v 被 dsdemo 吃掉了——它只该认 F12", k)
		}
	}
	if !p.shown {
		t.Fatal("按了别的键却把显隐切了")
	}

	// 带修饰键的 F12 不算：Ctrl+F12 是别的意思（而且 viva 的组合键路由不同）。
	if p.HandleKey(ui.KeyEvent{Key: ui.KeyF12, Modifiers: ui.ModifierControl}) {
		t.Error("Ctrl+F12 不该被当成 F12")
	}
	if !p.shown {
		t.Fatal("带修饰键的 F12 不该切换显隐")
	}

	if !p.HandleKey(ui.KeyEvent{Key: ui.KeyF12}) {
		t.Fatal("F12 应当被处理（返回 true），否则键盘路由会继续往下传")
	}
	if p.shown {
		t.Fatal("F12 之後应当收起")
	}
	if !p.HandleKey(ui.KeyEvent{Key: ui.KeyF12}) || !p.shown {
		t.Fatal("再按一次 F12 应当展开")
	}
}

// drawOnce 画一帧，返回这一帧画出来的矩形数（浮层有背景，所以"画了"= 至少一个矩形）。
func drawOnce(t *testing.T, p *Plugin, click string) int {
	t.Helper()
	theme := dscore.Dark()
	p.theme = theme
	if p.prims == nil {
		p.prims = &dscore.Primitives{Theme: theme}
	}
	tree := &ui.Tree{}
	feat := tree.Feature("demo-test")
	canvas := &ui.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	if click != "" {
		canvas.BeginEventPass(tree.ID(feat, click))
		defer canvas.EndEventPass()
	}
	env := &pluginapi.Env{
		Theme: theme, Data: pluginapi.Snapshot{Cur: "s"}, Act: pluginapi.Actions{},
		Tree: tree, Feat: feat, Canvas: canvas,
		Bounds: ui.Rect{Width: 1182, Height: 832}, MainWidth: 902,
	}
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(theme), env.Bounds, func(f *flex.Frame) {
		p.draw(f, env)
	})
	return len(canvas.Rectangles())
}

// TestHiddenDrawsNothing 钉住"收起 = 真的不画"。
//
// 画一个透明的/零高的浮层，看着一样，但它仍然占着 overlay 槽位、仍然参与命中——
// 用户会点到一片看不见的区域。
func TestHiddenDrawsNothing(t *testing.T) {
	p := &Plugin{}
	p.shown = false
	if n := drawOnce(t, p, ""); n != 0 {
		t.Fatalf("收起状态下画出了 %d 个矩形", n)
	}
}

// TestCloseButtonHides：点「×」收起。
func TestCloseButtonHides(t *testing.T) {
	p := &Plugin{}
	p.shown = true
	if n := drawOnce(t, p, ""); n == 0 {
		t.Fatal("展开状态下应当画出来")
	}
	drawOnce(t, p, "demo-close")
	if p.shown {
		t.Fatal("点了「×」之后应当收起")
	}
}
