//go:build darwin || windows || linux

package dscontextmenu

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func TestRightClickOpensAtPointer(t *testing.T) {
	p := newTestPlugin()
	if _, err := p.RegisterContribution(pluginapi.ContextMenuContribution{
		ID: "workspace.rename", Target: "workspace", Order: 100,
		Resolve: func(pluginapi.ContextMenuTarget) []pluginapi.ContextMenuAction {
			return []pluginapi.ContextMenuAction{{ID: "rename", Label: "重命名", Icon: "workspace.rename"}}
		},
	}); err != nil {
		t.Fatal(err)
	}
	tree := &gfx.Tree{}
	feature := tree.Feature("context-menu-right-click")
	rowID := tree.ID(feature, "row")
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	target := pluginapi.ContextMenuTarget{Kind: "workspace", ID: "w1"}
	config := app.Config{Title: "context-menu-right-click", Width: 320, Height: 200, Tree: tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		view.Draw(canvas, gfx.Rect{Width: size.Width, Height: size.Height}, func(f ui.C) {
			row := ui.Row(f).Width(100).Height(40).Hit(rowID, "工作区")
			p.Attach(f, rowID, row, target)
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.RightClick(40, 20); err != nil {
		t.Fatal(err)
	}
	if !session.HasText("重命名") {
		t.Fatalf("右键没有打开菜单：texts=%q", session.Texts())
	}
	found := false
	for _, hit := range session.Snapshot().Hits {
		if hit.Role == gfx.RoleMenuItem {
			found = true
			if hit.Bounds.X != 44 || hit.Bounds.Y != 24 {
				t.Fatalf("右键菜单行原点=(%v,%v)，want (44,24)（指针点 + 4px 菜单 padding）", hit.Bounds.X, hit.Bounds.Y)
			}
		}
	}
	if !found {
		t.Fatal("右键后没有菜单行命中区")
	}
}

func TestButtonOpensAnchoredMenuAndRunsAction(t *testing.T) {
	p := newTestPlugin()
	runs := 0
	if _, err := p.RegisterContribution(pluginapi.ContextMenuContribution{
		ID: "workspace.rename", Target: "workspace", Order: 100,
		Resolve: func(pluginapi.ContextMenuTarget) []pluginapi.ContextMenuAction {
			return []pluginapi.ContextMenuAction{{
				ID: "rename", Label: "重命名", Icon: "workspace.rename", Run: func() { runs++ },
			}}
		},
	}); err != nil {
		t.Fatal(err)
	}
	tree := &gfx.Tree{}
	feature := tree.Feature("context-menu-headless")
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	target := pluginapi.ContextMenuTarget{Kind: "workspace", ID: "w1"}
	config := app.Config{Title: "context-menu", Width: 320, Height: 200, Tree: tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		view.Draw(canvas, gfx.Rect{Width: size.Width, Height: size.Height}, func(f ui.C) {
			p.Button(f, tree.ID(feature, "menu"), target, "工作区操作")
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Click(10, 10); err != nil {
		t.Fatal(err)
	}
	if !session.HasText("重命名") {
		t.Fatalf("省略号按钮没有展开官方锚定菜单：texts=%q", session.Texts())
	}
	var item gfx.Hit
	found := false
	for _, hit := range session.Snapshot().Hits {
		if hit.Role == gfx.RoleMenuItem && hit.Label == "重命名" {
			item, found = hit, true
			break
		}
	}
	if !found {
		t.Fatalf("展开后找不到菜单行命中区：hits=%+v", session.Snapshot().Hits)
	}
	if item.Bounds.Y != 28 {
		t.Fatalf("菜单首行 y=%v，want 28（20px 锚点 + 官方 4px gap + 4px padding）", item.Bounds.Y)
	}
	if err := session.Click(item.Bounds.X+item.Bounds.Width/2, item.Bounds.Y+item.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if runs != 1 {
		t.Fatalf("点击菜单行动作执行 %d 次，want 1", runs)
	}
	if session.HasText("重命名") {
		t.Fatal("动作完成后菜单仍展开")
	}
}
