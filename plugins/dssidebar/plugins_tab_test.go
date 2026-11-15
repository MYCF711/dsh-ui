// plugins_tab_test.go —— 官方 Plugins 页多 tab 骨架（`#30` A 块）的判据。
//
// 官方对照：
//
//	OFF:../dsh-client-ui-plugin-manager/src/client/PluginManagerPage.tsx:1577-1580
//	     选中的贡献方离开台账就落回 `installed`
//	OFF:PluginManagerPage.tsx:1732-1749
//	     台账为空时**不画 tab 条**；否则 `installed` 在前、贡献方在后
//	OFF:PluginManagerPage.tsx:1750-1760
//	     贡献方的内容画在页头与 tab 条下面
//	OFF:../dsh-client-ui-plugin-manager/src/client/tab-ledger.ts:7-10
//	     一项贡献方 = id + label
//
// 这里钉住：台账随贡献变化、选中项回落、tab 条与页面内容在同一帧上的真实绘制。
package dssidebar

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// fakeMarketSeat 是「发现」tab 的假座位（记下它被画了几次）。
type fakeMarketSeat struct {
	label   string
	drawn   int
	details bool
}

func (f *fakeMarketSeat) Label() string { return f.label }

func (f *fakeMarketSeat) Draw(c ui.C, env *pluginapi.Env) {
	f.drawn++
	ui.Text(c, "市场座位正文").FontSize(13).LineHeight(20).Color(env.Theme.LabelPrimary)
}

func (f *fakeMarketSeat) DetailOpen() bool { return f.details }

// TestPluginsPageTabLedgerFollowsContributions 钉住台账：没有贡献方 = 没有 tab；
// 有贡献方 = installed 在前、贡献方在后。
func TestPluginsPageTabLedgerFollowsContributions(t *testing.T) {
	p := &Plugin{}
	if ids := p.pluginTabIDs(); len(ids) != 0 {
		t.Fatalf("没有贡献方时不该有 tab：%v", ids)
	}
	if got := p.activePluginTab(); got != pluginsTabInstalled {
		t.Fatalf("没有贡献方时活动 tab = %q，应为 installed", got)
	}

	seat := &fakeMarketSeat{label: "发现"}
	p.RegisterPluginsTab(seat)
	if ids := p.pluginTabIDs(); len(ids) != 2 || ids[0] != pluginsTabInstalled || ids[1] != pluginsTabDiscover {
		t.Fatalf("有贡献方时 tab 名单 = %v，应为 [installed discover]", ids)
	}
	if label := p.pluginTabs()[0].Label; label != "发现" {
		t.Fatalf("贡献方的 label = %q，应为「发现」", label)
	}
}

// TestPluginsPageTabRegisterDedupesAndValidates 钉住登记的边界：nil / 空 label 丢掉，
// 同一 id 后登记的覆盖前一次（官方槽位按 id 去重）。
func TestPluginsPageTabRegisterDedupesAndValidates(t *testing.T) {
	p := &Plugin{}
	p.RegisterPluginsTab(nil)
	p.RegisterPluginsTab(&fakeMarketSeat{label: ""})
	if len(p.pluginTabIDs()) != 0 {
		t.Fatal("nil 与空 label 的贡献方都不该进台账")
	}
	first := &fakeMarketSeat{label: "发现"}
	p.RegisterPluginsTab(first)
	second := &fakeMarketSeat{label: "发现（新）"}
	p.RegisterPluginsTab(second)
	tabs := p.pluginTabs()
	if len(tabs) != 1 || tabs[0].Label != "发现（新）" {
		t.Fatalf("同一 id 应当覆盖而不是追加：%+v", tabs)
	}
	if p.marketSeat != second {
		t.Fatal("座位也要换成后登记的那个")
	}
}

// TestPluginsPageTabFallsBackToInstalled 钉住选中项回落（官方 :1577-1580）。
func TestPluginsPageTabFallsBackToInstalled(t *testing.T) {
	p := &Plugin{}
	seat := &fakeMarketSeat{label: "发现"}
	p.RegisterPluginsTab(seat)
	p.pluginTab = pluginsTabDiscover
	if got := p.activePluginTab(); got != pluginsTabDiscover {
		t.Fatalf("贡献方在册时活动 tab = %q，应为 discover", got)
	}
	// 贡献方不在了（老壳 / 插件卸载）→ 落回 installed，并把状态写回去。
	p.marketSeat = nil
	p.pluginsTabLedger.tabs = nil
	if got := p.activePluginTab(); got != pluginsTabInstalled {
		t.Fatalf("贡献方离开后活动 tab = %q，应为 installed", got)
	}
	if p.pluginTab != pluginsTabInstalled {
		t.Fatalf("活动 tab 状态没写回：%q", p.pluginTab)
	}
}

// TestPluginsPageDrawsTabBarOnlyWithContributions 钉住"没有贡献方就不画 tab 条"
// （官方 `tabs.length === 0` 那条规则）。
func TestPluginsPageDrawsTabBarOnlyWithContributions(t *testing.T) {
	p, _, env, _, canvas := v2Fixture()
	env.Act.PluginBundles = nil
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: 900, Height: 700}, func(f ui.C) {
		p.drawPluginManagerPage(f, env)
	})
	for _, unwanted := range []string{zhPluginInstalledTab, "发现"} {
		if fg2HasText(canvas, unwanted) {
			t.Fatalf("没有贡献方时不该画 tab 条（出现了 %q）", unwanted)
		}
	}
}

// TestPluginsPageDrawsTabBarAndDiscoverBody 有贡献方时：tab 条上出现「已安装」与
// 「发现」；选中「发现」那一格时页头还在、tab 体换成贡献方、已安装那页不画。
func TestPluginsPageDrawsTabBarAndDiscoverBody(t *testing.T) {
	p, _, env, _, canvas := v2Fixture()
	env.Act.PluginBundles = func() ([]apiclient.PluginBundle, bool, bool, string) {
		return []apiclient.PluginBundle{pluginBundleFixture()}, true, false, ""
	}
	seat := &fakeMarketSeat{label: "发现"}
	p.RegisterPluginsTab(seat)

	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: 900, Height: 700}, func(f ui.C) {
		p.drawPluginManagerPage(f, env)
	})
	// 默认在已安装页：tab 条两格都在，已安装的内容照画。
	for _, want := range []string{zhPluginTitle, zhPluginInstalledTab, "发现", "鲸鱼宠物"} {
		if !fg2HasText(canvas, want) {
			t.Fatalf("已安装页缺 %q", want)
		}
	}
	if seat.drawn != 0 {
		t.Fatalf("没切到发现页时不该画贡献方（画了 %d 次）", seat.drawn)
	}

	// 切到发现页：页头仍在，tab 体换成贡献方，已安装的卡片不再画。
	p.pluginTab = pluginsTabDiscover
	canvas2 := v2Canvas(env.Tree)
	view2 := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view2.Draw(canvas2, gfx.Rect{Width: 900, Height: 700}, func(f ui.C) {
		p.drawPluginManagerPage(f, env)
	})
	for _, want := range []string{zhPluginTitle, zhPluginInstalledTab, "发现", "市场座位正文"} {
		if !fg2HasText(canvas2, want) {
			t.Fatalf("发现页缺 %q", want)
		}
	}
	if fg2HasText(canvas2, "鲸鱼宠物") {
		t.Fatal("发现页不该还画着已安装的卡片")
	}
	if seat.drawn != 1 {
		t.Fatalf("发现页应当把贡献方画一次，实际 %d 次", seat.drawn)
	}
}
