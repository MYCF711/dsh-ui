// plugins_tab_wiring_test.go —— `plugins.tab` 贡献链路**端到端**（`#30` A 块）。
//
// 为什么要有这条：单测能证明"有座位时 tab 条画得对"，证明不了**座位真的能被登记进来**。
// 这条故障是"tab 永远不出现"——不报错、界面少一块，最难查。所以这里用真内核装配
// 真插件（ds-core / ds-sidebar / ds-market），只验一件事：
//
//	ds-market 的 Register 真的把自己登记进了 ds-sidebar 的台账，
//	并且侧栏选中 discover 时画出来的是市场的「发现」页。
//
// 依赖方向也一并钉住：侧栏**不**要求市场插件（`Requires` 里没有市场键），
// 所以最小装配（只有 ds-core + 侧栏）起得来、只是没有 tab 条。
package dssidebar

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsmarket"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// corePlugin 是最小装配里的 ds-core 替身：只交主题与基础组件。
type corePlugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives
}

func (c *corePlugin) ID() string { return "ds-core-test" }
func (c *corePlugin) Provides() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}
func (c *corePlugin) Requires() []string { return nil }
func (c *corePlugin) Register(ctx pluginapi.Context) error {
	ctx.Provide(pluginapi.KeyTheme, c.theme)
	ctx.Provide(pluginapi.KeyPrimitives, c.prims)
	return nil
}

// TestPluginsTabContributionArrivesThroughKernel 钉住真装配下的贡献链路。
func TestPluginsTabContributionArrivesThroughKernel(t *testing.T) {
	theme := dscore.Dark()
	core := &corePlugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
	sidebar := &Plugin{width: 280, collapsed: map[string]bool{}}
	market := &dsmarket.Plugin{}

	kernel := uikernel.New("plugins-tab-wiring")
	// 故意把市场插件排在侧栏**前面**：内核按 Requires 拓扑排序，顺序不该靠运气。
	if err := kernel.Load(core, market, sidebar); err != nil {
		t.Fatalf("装配失败：%v", err)
	}

	ids := sidebar.pluginTabIDs()
	if len(ids) != 2 || ids[0] != pluginsTabInstalled || ids[1] != pluginsTabDiscover {
		t.Fatalf("市场插件的 Register 没把 tab 登记进侧栏台账：%v", ids)
	}
	if label := sidebar.pluginTabs()[0].Label; label != "发现" {
		t.Fatalf("贡献方的 tab 名 = %q，应为官方 `title`（发现）", label)
	}

	// 选中 discover 时画的是市场那一页（这里只验"座位被派发了"，
	// 内容几何由 dsmarket 自己的视觉测试管）。
	env := &pluginapi.Env{Theme: theme, Tree: &gfx.Tree{}, Feat: gfx.Scope(0)}
	env.Feat = env.Tree.Feature("plugins-tab-wiring")
	canvas := v2Canvas(env.Tree)
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	sidebar.pluginTab = pluginsTabDiscover
	view.Draw(canvas, gfx.Rect{Width: 900, Height: 700}, func(f ui.C) {
		sidebar.drawPluginManagerPage(f, env)
	})
	if !fg2HasText(canvas, "发现") {
		t.Fatalf("选中 discover 时 tab 条上应有「发现」；文本=%v", canvas.Texts())
	}
}

// TestSidebarDoesNotRequireMarketPlugin 钉住依赖方向：侧栏不要求市场插件，
// 最小装配（只有 ds-core + 侧栏）必须起得来，只是没有 tab 条。
func TestSidebarDoesNotRequireMarketPlugin(t *testing.T) {
	for _, key := range (&Plugin{}).Requires() {
		if key == pluginapi.KeyMarketDiscover || key == pluginapi.KeyPluginsTabs {
			t.Fatalf("侧栏不该Requires %q（否则最小装配起不来，且依赖方向反了）", key)
		}
	}
	theme := dscore.Dark()
	core := &corePlugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
	sidebar := &Plugin{width: 280, collapsed: map[string]bool{}}
	kernel := uikernel.New("plugins-tab-minimal")
	if err := kernel.Load(core, sidebar); err != nil {
		t.Fatalf("最小装配（无市场插件）应当起得来：%v", err)
	}
	if ids := sidebar.pluginTabIDs(); len(ids) != 0 {
		t.Fatalf("没有贡献方时不该有 tab：%v", ids)
	}
}
