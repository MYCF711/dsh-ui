package dsopen

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// dsopen 以前**一个测试都没有**，而它的历史全是"看得见吗"：
// 按钮做出来之后连续三轮在截图里找不到（先怀疑槽位没画、再怀疑高度塌陷，
// 最后确认是抓帧时机太早——CWD 要等 session.load 回来才填）。
//
// 所以这里的重点不是"代码跑过了"，是**它到底画在了哪里、有多大、点得动吗**。

// renderOpen 画一帧会话头部，返回这一帧画出来的矩形。
// click 非空时走 canvas 的事件 pass，测到的是 row.Clicked() 的真实分支。
func renderOpen(t *testing.T, p *Plugin, data pluginapi.Snapshot, act pluginapi.Actions, click string) []ui.Rectangle {
	t.Helper()
	if p.theme == nil {
		p.theme = dscore.Dark()
	}
	tree := &ui.Tree{}
	feat := tree.Feature("open-test")
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
		Theme: p.theme, Data: data, Act: act, Tree: tree, Feat: feat, Canvas: canvas,
		Bounds: ui.Rect{Width: 1182, Height: 832}, MainWidth: 902,
	}
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(p.theme), env.Bounds, func(f *flex.Frame) {
		// 模拟会话头部那一行会怎么放它：靠右。
		f.Row().FillWidth().Justify(flex.End).Children(func() { p.draw(f, env) })
	})
	return canvas.Rectangles()
}

func TestDrawsNothingWithoutCWD(t *testing.T) {
	// 没有工作目录就没有"打开哪"这个问题。这条不只是省一次绘制：
	// 画出来一个点了没反应的按钮，比不画它更让人困惑。
	p := &Plugin{}
	rects := renderOpen(t, p, pluginapi.Snapshot{Cur: "s"}, pluginapi.Actions{
		OpenExternal: func(string) error { return nil },
	}, "")
	if len(rects) != 0 {
		t.Fatalf("没有 CWD 时不该画按钮，画了 %d 个矩形：%+v", len(rects), rects)
	}
}

func TestDrawsNothingWithoutOpenExternal(t *testing.T) {
	// 老宿主没接 OpenExternal。同理：手段都没有，就别摆一个按钮。
	p := &Plugin{}
	rects := renderOpen(t, p, pluginapi.Snapshot{Cur: "s", CWD: `D:\ws`}, pluginapi.Actions{}, "")
	if len(rects) != 0 {
		t.Fatalf("壳没接 OpenExternal 时不该画按钮，画了 %d 个矩形：%+v", len(rects), rects)
	}
}

// TestButtonIsAReal56x24Box 是"看得见"这一层的最小证据。
//
// 56×24 不是随手写的：宽度必须显式给。第一版没给宽度，Row 默认撑满，
// 按钮变成一条横贯整行的色带——这个坑在注释里记着，这里把它钉住。
func TestButtonIsAReal56x24Box(t *testing.T) {
	p := &Plugin{}
	rects := renderOpen(t, p, pluginapi.Snapshot{Cur: "s", CWD: `D:\ws`}, pluginapi.Actions{
		OpenExternal: func(string) error { return nil },
	}, "")
	if len(rects) != 1 {
		t.Fatalf("应当正好画一个按钮矩形，得到 %d 个：%+v", len(rects), rects)
	}
	got := rects[0].Bounds
	if got.Width != 56 || got.Height != 24 {
		t.Errorf("按钮尺寸 %vx%v，want 56x24——宽度不给就会撑满整行", got.Width, got.Height)
	}
}

// TestClickOpensTheWorkspaceDirectory 验动作本身：点一下 → OpenExternal(CWD)。
func TestClickOpensTheWorkspaceDirectory(t *testing.T) {
	var opened []string
	p := &Plugin{}
	data := pluginapi.Snapshot{Cur: "s", CWD: `D:\ws`}
	act := pluginapi.Actions{OpenExternal: func(dst string) error {
		opened = append(opened, dst)
		return nil
	}}

	// 先画一帧（真实运行时也是先画再点）。
	renderOpen(t, p, data, act, "")
	if len(opened) != 0 {
		t.Fatalf("没点就调了 OpenExternal：%v", opened)
	}
	renderOpen(t, p, data, act, "open-in-app")
	if len(opened) != 1 {
		t.Fatalf("点了按钮，OpenExternal 被调 %d 次，want 1", len(opened))
	}
	if opened[0] != `D:\ws` {
		t.Errorf("打开的是 %q，want %q（必须是工作目录本身，不是某个文件）", opened[0], `D:\ws`)
	}
}
