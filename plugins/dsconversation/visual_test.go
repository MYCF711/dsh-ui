package dsconversation

import (
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// newHeroTestCanvas 用真实 flex 布局跑 draw，但把字宽测量换成确定性的测试测量器。
// 这样不需要窗口，也能让旧实现的空态缺口在 CGO_ENABLED=0 下稳定复现。
func newHeroTestCanvas(tree *gfx.Tree) *gfx.Canvas {
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size
	})
	return canvas
}

func drawHeroTest(t *testing.T, data pluginapi.Snapshot, mainWidth float32) (*gfx.Canvas, *Plugin) {
	t.Helper()
	theme := dscore.Dark()
	p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
	tree := &gfx.Tree{}
	canvas := newHeroTestCanvas(tree)
	var view flex.View
	f := view.Begin(canvas, dscore.VivaTheme(theme), ui.Rect{Width: mainWidth, Height: 700})
	p.draw(f, &pluginapi.Env{
		Theme: theme, Data: data, Tree: tree, Feat: tree.Feature("hero-test"), Canvas: canvas,
		Bounds: ui.Rect{Width: mainWidth, Height: 700}, MainWidth: mainWidth,
	})
	f.End()
	// 鲸鱼与 Markdown 一样走真实的第二阶段覆画；漏掉这一步会把测试支架
	// 自己的问题误报成实现没有品牌标记。
	p.afterDraw(canvas, &pluginapi.Env{Theme: theme, Data: data, Bounds: ui.Rect{Width: mainWidth, Height: 700}})
	return canvas, p
}

func heroTestText(t *testing.T, canvas *gfx.Canvas, value string) gfx.Text {
	t.Helper()
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text
		}
	}
	t.Fatalf("画面缺少文本 %q；实际文本：%+v", value, canvas.Texts())
	return gfx.Text{}
}

func TestEmptyStateUsesOfficialHero(t *testing.T) {
	canvas, p := drawHeroTest(t, pluginapi.Snapshot{}, 900)

	headline := heroTestText(t, canvas, "探索未至之境")
	if headline.Style.Size != 26 || headline.Bounds.Height != 32 || headline.Style.Weight != gfx.WeightMedium {
		t.Errorf("官方 headline 应为 26/32/500，实际 size=%v line=%v weight=%v",
			headline.Style.Size, headline.Bounds.Height, headline.Style.Weight)
	}
	preview := heroTestText(t, canvas, "预览版")
	if preview.Style.Size != 12 || preview.Style.Weight != gfx.WeightMedium {
		t.Errorf("官方预览胶囊应为 12/500，实际 size=%v weight=%v", preview.Style.Size, preview.Style.Weight)
	}
	if !preview.Style.Mono {
		t.Error("官方预览胶囊使用 ds-font-family-code，应走等宽字体")
	}
	// Viva 的 mono glyph 框不使用 LineHeight；CSS 的 18px 行高由胶囊内容区表达：
	// 20（外高）- 2×0.5（描边）- 1（上 padding）= 18。
	badge := p.heroBoxes.Badge.Rect()
	if badge.Height != 20 || badge.Height-2*heroBadgeBorder-heroBadgePadTop != 18 {
		t.Errorf("预览胶囊几何 = %+v，不能表达官方 12/18 与 20px 外高", badge)
	}
	if len(canvas.Triangles()) == 0 {
		t.Error("空态没有矢量品牌鱼标，仍只是文字提示")
	}
	for _, text := range canvas.Texts() {
		if text.Value == "← 左边选一个会话" {
			t.Error("旧的简陋空态提示仍在")
		}
	}
}

func TestHeroWorkspaceUsesOnlyCurrentSessionCWD(t *testing.T) {
	sessions := []apiclient.Session{{ID: "s1", Title: "blank", Cwd: `D:\\work\\dsh-ui`}}
	canvas, _ := drawHeroTest(t, pluginapi.Snapshot{Cur: "s1", Sessions: sessions}, 900)
	workspace := heroTestText(t, canvas, "dsh-ui")
	if workspace.Style.Size != 13 || workspace.Bounds.Height != 20 || workspace.Style.Weight != gfx.WeightMedium {
		t.Errorf("工作区应为 13/20/500，实际 size=%v line=%v weight=%v",
			workspace.Style.Size, workspace.Bounds.Height, workspace.Style.Weight)
	}

	canvas, _ = drawHeroTest(t, pluginapi.Snapshot{Sessions: sessions}, 900)
	for _, text := range canvas.Texts() {
		if text.Value == "dsh-ui" {
			t.Fatal("没有当前会话时不得借用别的会话 cwd 冒充当前工作区")
		}
	}
}

func TestHeroGeometryAtOfficialWindowWidths(t *testing.T) {
	data := pluginapi.Snapshot{
		Cur: "s1", Sessions: []apiclient.Session{{ID: "s1", Title: "blank", Cwd: `D:\\work\\dsh-ui`}},
	}
	cases := []struct {
		name       string
		mainWidth  float32
		stackWidth float32
	}{
		{name: "1182 外框对应主列", mainWidth: 886, stackWidth: 712},
		{name: "1920 外框对应主列", mainWidth: 1624, stackWidth: 952},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, p := drawHeroTest(t, data, tc.mainWidth)
			root := p.heroBoxes.Root.Rect()
			stack := p.heroBoxes.Stack.Rect()
			if stack.Width != tc.stackWidth {
				t.Errorf("stack 宽 = %v，官方 %v", stack.Width, tc.stackWidth)
			}
			if got, want := stack.X+stack.Width/2, root.X+root.Width/2; got != want {
				t.Errorf("stack 未水平居中：center=%v want %v", got, want)
			}
			if got, want := stack.Y+stack.Height/2, root.Y+root.Height/2; got != want {
				t.Errorf("stack 未垂直居中：center=%v want %v", got, want)
			}
			mark := p.heroBoxes.Mark.Rect()
			if mark.Width != 34 || mark.Height < 25.01 || mark.Height > 25.03 {
				t.Errorf("鱼标几何 = %+v，官方 34×25.02", mark)
			}
			if got := p.heroBoxes.Workspace.Rect().X - stack.X; got != 20 {
				t.Errorf("workspace 相对 stack 左边 = %v，官方 20", got)
			}
			if got := p.heroBoxes.Workspace.Rect().Height; got != 28 {
				t.Errorf("workspace 高 = %v，官方 28", got)
			}
			t.Logf("V1_GEOMETRY %s root=%+v stack=%+v headline=%+v mark=%+v workspaceRow=%+v workspace=%+v",
				tc.name, root, stack, p.heroBoxes.Headline.Rect(), mark,
				p.heroBoxes.WorkspaceRow.Rect(), p.heroBoxes.Workspace.Rect())
		})
	}
}

func TestHeroHiddenForRealConversationActivity(t *testing.T) {
	cases := []struct {
		name string
		data pluginapi.Snapshot
	}{
		{name: "历史消息", data: pluginapi.Snapshot{Msgs: []apiclient.Message{{Role: "user", Text: "hi"}}}},
		{name: "流式正文", data: pluginapi.Snapshot{Stream: " answering"}},
		{name: "已发送但正文尚未到达", data: pluginapi.Snapshot{Busy: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			canvas, _ := drawHeroTest(t, tc.data, 900)
			for _, text := range canvas.Texts() {
				if text.Value == "探索未至之境" || text.Value == "预览版" {
					t.Fatalf("真实会话活动期间不得覆盖欢迎区：%q", text.Value)
				}
			}
		})
	}
}
