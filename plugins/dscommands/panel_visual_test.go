package dscommands

import (
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// renderCommandPanel 在真实的 680px composer overlay 锚点里画一次。
// 父盒 y=400,h=36，官方 bottom:calc(100%+4px) 因而要求面板底边 y=396。
func renderCommandPanel(t *testing.T, p *Plugin, act pluginapi.Actions) *gfx.Canvas {
	t.Helper()
	if p.theme == nil {
		p.theme = dscore.Dark()
	}
	p.shown = true
	tree := &gfx.Tree{}
	feat := tree.Feature("commands-visual")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: p.theme, Act: act, Tree: tree, Feat: feat, Canvas: canvas,
		Data:   pluginapi.Snapshot{Cur: "session-1"},
		Bounds: gfx.Rect{Width: 920, Height: 832},
	}
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	// 两帧：第一帧量 ScrollArea Content 与搜索框高，第二帧验证稳定后的官方几何。
	for range 2 {
		view.Draw(canvas, env.Bounds, func(f ui.C) {
			ui.Column(f).Absolute().Left(20).Top(400).Width(680).Height(36).Children(func() {
				p.draw(f, env)
			})
		})
		p.afterDraw(canvas, env)
	}
	return canvas
}

func commandPanelSurface(canvas *gfx.Canvas, theme *pluginapi.Theme) (gfx.Rectangle, bool) {
	var panel gfx.Rectangle
	found := false
	for _, r := range canvas.Rectangles() {
		if r.Color == theme.BgLayer3 && r.Radius == theme.Radii.MD && r.Bounds.Width == 680 {
			panel, found = r, true // 同一 canvas 有两帧；取最后稳定帧
		}
	}
	return panel, found
}

func commandText(canvas *gfx.Canvas, value string) (gfx.Text, bool) {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text, true
		}
	}
	return gfx.Text{}, false
}

func TestCommandPanelOfficialGeometryAndFonts(t *testing.T) {
	p := &Plugin{}
	act := pluginapi.Actions{
		NewSession: func() {}, CompactSession: func() {}, SetPermission: func(string) {},
	}
	canvas := renderCommandPanel(t, p, act)
	panel, ok := commandPanelSurface(canvas, p.theme)
	if !ok {
		t.Fatal("没有找到 composer 同宽的 MenuSurface")
	}
	if panel.Bounds.X != 20 || panel.Bounds.Y+panel.Bounds.Height != 396 {
		t.Fatalf("面板位置 = %+v；官方 left:0/right:0/bottom:100%%+4 应为 x=20,bottom=396", panel.Bounds)
	}
	if panel.Bounds.Height > commandPanelMaxHeight {
		t.Fatalf("面板高 %v > 官方 320", panel.Bounds.Height)
	}
	name, ok := commandText(canvas, "新建会话")
	if !ok || name.Style.Size != commandRowFont {
		t.Fatalf("命令名 %q 字号 = %+v，官方 12px", "新建会话", name.Style)
	}
	detail, ok := commandText(canvas, "开一个新会话")
	if !ok || detail.Style.Size != commandDetailFont {
		t.Fatalf("命令 detail 字号 = %+v，官方 11px", detail.Style)
	}
	placeholder, ok := commandText(canvas, commandSearchPlaceholder)
	if !ok || placeholder.Style.Size != commandSearchFont {
		t.Fatalf("搜索框 placeholder 字号 = %+v，官方 12px", placeholder.Style)
	}
	var selected bool
	for _, r := range canvas.Rectangles() {
		if r.Color == p.theme.Hover && r.Radius == p.theme.Radii.MD && r.Bounds.Width == 672 {
			selected = true
		}
	}
	if !selected {
		t.Fatal("首条选中态没有 interactive-bg-hover + radius-md 的整行背景")
	}
}

func TestCommandPanelSearchUsesRealFieldAndHeightCap(t *testing.T) {
	p := &Plugin{}
	p.search.SetText("压缩")
	canvas := renderCommandPanel(t, p, scriptedActions(func() {}))
	if _, ok := commandText(canvas, "压缩上下文"); !ok {
		t.Fatal("真实搜索框输入「压缩」后没有留下匹配命令")
	}
	if _, ok := commandText(canvas, "新建会话"); ok {
		t.Fatal("搜索「压缩」后仍画出了不匹配的新建会话")
	}

	p.search.SetText("")
	canvas = renderCommandPanel(t, p, scriptedActions(func() {}))
	panel, ok := commandPanelSurface(canvas, p.theme)
	if !ok {
		t.Fatal("完整命令表没有画出面板")
	}
	if panel.Bounds.Height != commandPanelMaxHeight {
		for _, r := range canvas.Rectangles() {
			t.Logf("rect color=%v radius=%v bounds=%+v clip=%+v/%v", r.Color, r.Radius, r.Bounds, r.ClipOf, r.HasClip)
		}
		t.Fatalf("长命令表面板高 = %v，官方 cap 应为 320（commands=%d content=%v search=%v query=%q）",
			panel.Bounds.Height, len(p.filtered()), p.viewportContent, p.searchHeight, p.search.Text())
	}
}
