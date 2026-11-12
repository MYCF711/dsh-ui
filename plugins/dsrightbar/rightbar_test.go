//go:build darwin || windows || linux

package dsrightbar

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// TestEmptyStateUsesOfficialGuideGeometry 对应官方 SidebarRight guide：
// 空态在可用 body 里水平/垂直居中，横向 padding 24，正文为 secondary 13px/tertiary。
func TestEmptyStateUsesOfficialGuideGeometry(t *testing.T) {
	session, _ := openRightbar(t, 500, 600, nil)
	text := findText(t, session.Snapshot().Texts, "本会话还没有工具调用")
	theme := dscore.Dark()
	if text.Style.Size != theme.Fonts.Secondary {
		t.Fatalf("空态字号 = %v，官方 secondary = %v", text.Style.Size, theme.Fonts.Secondary)
	}
	if text.Style.Color != theme.LabelTertiary {
		t.Fatalf("空态颜色 = %+v，官方 label-tertiary = %+v", text.Style.Color, theme.LabelTertiary)
	}
	cx := text.Bounds.X + text.Bounds.Width/2
	cy := text.Bounds.Y + text.Bounds.Height/2
	if cx < 242 || cx > 258 {
		t.Fatalf("空态没有水平居中：centerX=%v text=%+v", cx, text.Bounds)
	}
	// 顶部标题/分隔线之后，可用 body 的中心应接近整窗中心；旧实现贴在 y≈50。
	if cy < 270 || cy > 340 {
		t.Fatalf("空态没有在 body 内垂直居中：centerY=%v text=%+v", cy, text.Bounds)
	}
}

// TestToolCallsReallyPaint 使用 Viva 无头驱动跑真实 Draw + flex 布局，
// 同时断言文本、成功/失败圆点和非零高度连接线，而不是只测 describe 纯函数。
func TestToolCallsReallyPaint(t *testing.T) {
	msgs := []apiclient.Message{
		{Role: "tool", Tool: "bash", Summary: "echo v4a", View: &apiclient.ViewSpec{Badge: "ok"}},
		{Role: "tool", Tool: "read", Summary: "read failed", View: &apiclient.ViewSpec{Badge: "error"}},
	}
	session, _ := openRightbar(t, 500, 600, msgs)
	for _, want := range []string{"轨迹", "2 次", "bash", "read", "echo v4a", "read failed"} {
		if !session.HasText(want) {
			t.Fatalf("真实绘制缺少文本 %q；texts=%q", want, session.Texts())
		}
	}
	theme := dscore.Dark()
	var okDot, errDot, connector bool
	for _, r := range session.Snapshot().Primitives {
		if r.Bounds.Width == 6 && r.Bounds.Height == 6 && sameColor(r.Color, theme.Success) {
			okDot = true
		}
		if r.Bounds.Width == 6 && r.Bounds.Height == 6 && sameColor(r.Color, theme.Error) {
			errDot = true
		}
		if r.Bounds.Width == 2 && r.Bounds.Height > 0 && sameColor(r.Color, theme.BorderL2) {
			connector = true
		}
	}
	if !okDot || !errDot || !connector {
		t.Fatalf("状态图元实画不全：okDot=%v errDot=%v connector=%v", okDot, errDot, connector)
	}
}

// TestRightbarResizeKeepsRealContentInside 在同一无头窗口里缩放，
// 验证不是只对一张静态截图有效。
func TestRightbarResizeKeepsRealContentInside(t *testing.T) {
	msgs := []apiclient.Message{{Role: "tool", Tool: "a-very-long-tool-name", Args: "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz"}}
	session, _ := openRightbar(t, 500, 600, msgs)
	if err := session.Resize(300, 480); err != nil {
		t.Fatal(err)
	}
	for _, text := range session.Snapshot().Texts {
		if text.Bounds.X < 0 || text.Bounds.X+text.Bounds.Width > 300.01 {
			t.Fatalf("缩放后文本越出右栏：%+v", text)
		}
	}
	if !session.HasText("a-very-long-tool-name") {
		t.Fatalf("缩放后工具条目没有实画：%q", session.Texts())
	}
}

func openRightbar(t *testing.T, width, height int, msgs []apiclient.Message) (*headless.Session, *Plugin) {
	t.Helper()
	theme := dscore.Dark()
	p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
	tree := &gfx.Tree{}
	feat := tree.Feature("rightbar-test")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{Title: "rightbar-v4a-test", Width: width, Height: height, Tree: tree}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: theme, Data: pluginapi.Snapshot{Cur: "session-v4a", Msgs: msgs},
			Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			Has: func(pluginapi.Slot) bool { return true },
		}
		view.Draw(canvas, bounds, func(f ui.C) { p.draw(f, env) })
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session, p
}

func sameColor(got, want ui.Color) bool { return got == want }

func findText(t *testing.T, texts []gfx.Text, value string) gfx.Text {
	t.Helper()
	for _, text := range texts {
		if text.Value == value {
			return text
		}
	}
	t.Fatalf("找不到文本 %q；texts=%+v", value, texts)
	return gfx.Text{}
}
