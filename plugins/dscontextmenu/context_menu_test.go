package dscontextmenu

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func newTestPlugin() *Plugin {
	return &Plugin{
		theme:   dscore.Dark(),
		byKind:  map[string][]pluginapi.ContextMenuContribution{},
		buttons: map[string]*bool{},
	}
}

func TestRegisterContributionSortsAndRejectsDuplicate(t *testing.T) {
	p := newTestPlugin()
	mk := func(id string, order int) pluginapi.ContextMenuContribution {
		return pluginapi.ContextMenuContribution{
			ID: id, Target: "workspace", Order: order,
			Resolve: func(pluginapi.ContextMenuTarget) []pluginapi.ContextMenuAction {
				return []pluginapi.ContextMenuAction{{ID: id, Label: id}}
			},
		}
	}
	if _, err := p.RegisterContribution(mk("b", 300)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.RegisterContribution(mk("a", 100)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.RegisterContribution(mk("a", 100)); err == nil {
		t.Fatal("同 target 内重复 id 必须报错（官方 registry.ts:32-33）")
	}
	if !p.HasTarget("workspace") {
		t.Fatal("登记后 HasTarget(workspace) 应为 true")
	}
	if p.HasTarget("session") {
		t.Fatal("没有贡献的 kind 必须为 false——contextMenu.button 靠它决定不渲染")
	}
	actions := p.resolve(pluginapi.ContextMenuTarget{Kind: "workspace", ID: "w"})
	if len(actions) != 2 || actions[0].ID != "a" || actions[1].ID != "b" {
		t.Fatalf("动作顺序 = %+v，want order 100(a) → 300(b)", actions)
	}
}

func TestUnregisterRemovesContribution(t *testing.T) {
	p := newTestPlugin()
	un, err := p.RegisterContribution(pluginapi.ContextMenuContribution{
		ID: "rename", Target: "workspace", Order: 100,
		Resolve: func(pluginapi.ContextMenuTarget) []pluginapi.ContextMenuAction {
			return []pluginapi.ContextMenuAction{{ID: "rename", Label: "重命名"}}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	un()
	if p.HasTarget("workspace") {
		t.Fatal("反注册后 HasTarget 必须为 false（官方卸载即撤回）")
	}
	un() // 幂等：重复反注册不 panic，也不再改动
}

func TestOfficialSlotsAreRegisteredAndIdempotent(t *testing.T) {
	if err := RegisterOfficialSlots(); err != nil {
		t.Fatal(err)
	}
	before := len(uikernel.OfficialSlots())
	if err := RegisterOfficialSlots(); err != nil {
		t.Fatal(err)
	}
	if after := len(uikernel.OfficialSlots()); after != before {
		t.Fatalf("重复调用 RegisterOfficialSlots 改变了槽数：%d → %d", before, after)
	}
	for _, name := range []pluginapi.Slot{SlotContextMenuButton, SlotContextMenuIcon, SlotContextMenuContent} {
		spec, ok := uikernel.LookupOfficialSlot(name)
		if !ok {
			t.Fatalf("官方槽 %q 没有登记", name)
		}
		if spec.Anchor == "" || spec.Parent != uikernel.OffRoot {
			t.Fatalf("槽 %q 的锚点/父槽不完整：%+v", name, spec)
		}
	}
}

// TestAttachAndButtonRenderThroughViva 走一遍真实 Page 绘制：Attach 注册右键目标、
// Button 画省略号并挂 ui.Menu。这不是像素断言，而是证明 viva v0.6.0 的
// ContextMenu/Menu API 用法在这个宿主里能真正执行（不 panic、不吞错误）。
func TestAttachAndButtonRenderThroughViva(t *testing.T) {
	p := newTestPlugin()
	target := pluginapi.ContextMenuTarget{Kind: "workspace", ID: "w1", DisplayTitle: "A"}
	if _, err := p.RegisterContribution(pluginapi.ContextMenuContribution{
		ID: "rename", Target: "workspace", Order: 100,
		Resolve: func(pluginapi.ContextMenuTarget) []pluginapi.ContextMenuAction {
			return []pluginapi.ContextMenuAction{{ID: "rename", Label: "重命名"}}
		},
	}); err != nil {
		t.Fatal(err)
	}
	tree := &gfx.Tree{}
	feature := tree.Feature("context-menu-render")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	pageTheme := dscore.VivaTheme(p.theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	page.Draw(canvas, gfx.Rect{Width: 320, Height: 200}, func(f ui.C) {
		row := ui.Row(f).FillWidth().Height(32).Hit(tree.ID(feature, "row"), "row")
		row.Children(func() { ui.Text(f, "A").FontSize(14) })
		p.Attach(f, row, target)
		if !p.Button(f, tree.ID(feature, "menu"), target, "工作区“A”的操作") {
			t.Fatal("有贡献的 target kind 必须画 contextMenu.button")
		}
	})
}

// TestButtonHiddenWithoutContribution 钉住官方「空 kind 不渲染按钮」。
func TestButtonHiddenWithoutContribution(t *testing.T) {
	p := newTestPlugin()
	tree := &gfx.Tree{}
	feature := tree.Feature("context-menu-empty")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	pageTheme := dscore.VivaTheme(p.theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	drawn := false
	page.Draw(canvas, gfx.Rect{Width: 320, Height: 200}, func(f ui.C) {
		drawn = p.Button(f, tree.ID(feature, "menu"), pluginapi.ContextMenuTarget{Kind: "workspace", ID: "w1"}, "x")
	})
	if drawn {
		t.Fatal("没有贡献者的 target kind 不能画按钮（官方 index.ts:37-40）")
	}
}
