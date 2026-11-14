// official_plugins_test.go —— 「内置插件」页的模型测试。
//
// 判据全部来自官方 rc.2 随包 `src/**`（`OFF:` 锚点见 official_plugins.go 的文件头），
// 不是从截图猜的：
//
//	① 顺序：`failed` 条先、其余保持 Loader 顺序（PluginInventorySettingsTab.tsx:295-301）；
//	② 分组：`entries.length > 0` 才画全局段，段体只在有命中时画（:550,565）；
//	③ 默认态：全局段**收起**、搜索时强制展开（:315-317）；
//	④ 文案：一律官方 zh 字典逐字（两份 locales.ts）；
//	⑤ 缺席语义：meta 缺席回退短模块名、fiberPhase null = 未运行、agentPresets 缺席 = 没这个面；
//	⑥ 宿主没接（inventory==nil）= 未知 + 重试，**不画假卡片**。
package dssettings

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// pluginsFixture 造一份与真宿主同形的清单（entryId=bundle name、moduleName=manifest entry、
// fiberPhase 只有 active/null 两种——见 DSH-GO serve_plugins_official.go:188-225）。
func pluginsFixture() *apiclient.PluginInventoryResult {
	management := true
	active := apiclient.PluginFiberPhase{Present: true, Phase: apiclient.PluginPhaseActive}
	null := apiclient.PluginFiberPhase{Present: true}
	failed := apiclient.PluginFiberPhase{Present: true, Phase: apiclient.PluginPhaseFailed}
	return &apiclient.PluginInventoryResult{
		ManagementAvailable: &management,
		Entries: []apiclient.PluginInventoryEntry{
			{EntryID: "alpha", ModuleName: "plugin.wasm", Enabled: true, FiberPhase: active},
			{EntryID: "beta", ModuleName: "other.wasm", Enabled: false, FiberPhase: null},
			{EntryID: "gamma", ModuleName: "bad.wasm", Enabled: true, FiberPhase: failed,
				Diagnostic: "运行时装载的是 1.0.0，state.json 记录 0.9.9"},
		},
	}
}

// pluginsSection 取模型里的插件页。
func pluginsSection(t *testing.T, model PanelModel) (PanelSection, *PluginsPanel) {
	t.Helper()
	section, ok := model.Section("plugins")
	if !ok {
		t.Fatal("模型里没有 plugins 这一页")
	}
	if section.Plugins == nil {
		t.Fatal("plugins 页没有 Plugins 投影（还是老的空壳）")
	}
	return section, section.Plugins
}

func TestPluginsPageRendersRealEntries(t *testing.T) {
	model := BuildPanelWithPlugins(describeFixture(), fullDesktopFacts(), pluginsFixture(), PluginsView{})
	section, plugins := pluginsSection(t, model)

	if section.State != PanelOK {
		t.Fatalf("宿主给了清单，页状态应为 ok，实得 %q", section.State)
	}
	if plugins.State != PluginsReady {
		t.Fatalf("状态应为 ready，实得 %q", plugins.State)
	}
	if len(section.Rows) != 0 {
		t.Fatalf("插件页走自己的渲染面，Rows 应为空，实得 %d 行", len(section.Rows))
	}
	if len(plugins.Groups) != 1 || plugins.Groups[0].Key != "global" {
		t.Fatalf("只有全局段（宿主不产出 agentPresets）：%+v", plugins.Groups)
	}
	group := plugins.Groups[0]
	if group.Title != "全局插件" || group.Subtitle != "系统与所有会话共用" {
		t.Fatalf("段标题未逐字对齐官方 zh：%q / %q", group.Title, group.Subtitle)
	}
	// 官方：failed 先，其余保持 Loader 顺序。
	wantOrder := []string{"gamma", "alpha", "beta"}
	if len(group.Cards) != len(wantOrder) {
		t.Fatalf("卡片 %d 张，应为 %d", len(group.Cards), len(wantOrder))
	}
	for i, want := range wantOrder {
		if group.Cards[i].RowKey != "global:"+want {
			t.Fatalf("第 %d 张是 %q，官方顺序要求 %q（failed 先）", i, group.Cards[i].RowKey, want)
		}
	}

	// 三张卡的状态与详情行，逐个对官方语义。
	gamma, alpha, beta := group.Cards[0], group.Cards[1], group.Cards[2]
	if gamma.State != "启动失败" || !gamma.Tag || !gamma.Failed {
		t.Fatalf("failed 条 = %+v", gamma)
	}
	if alpha.State != "已启用" || alpha.Tag {
		t.Fatalf("普通启用条不该有标签（官方 enabled 不画 tag）：%+v", alpha)
	}
	if beta.State != "已停用" || !beta.Tag {
		t.Fatalf("停用条应带 neutral 标签：%+v", beta)
	}
	// 官方 facts：配置状态 + （启用时）运行状态。
	if len(alpha.Facts) != 2 || alpha.Facts[0].Label != "配置状态" || alpha.Facts[0].Value != "已启用" ||
		alpha.Facts[1].Label != "运行状态" || alpha.Facts[1].Value != "运行中" {
		t.Fatalf("启用条的详情行 = %+v", alpha.Facts)
	}
	if len(gamma.Facts) != 2 || gamma.Facts[1].Value != "启动失败" {
		t.Fatalf("failed 条启用时也要运行状态行 = %+v", gamma.Facts)
	}
	if len(beta.Facts) != 1 {
		t.Fatalf("停用条只有配置状态一行（官方 enabled 才加运行状态）= %+v", beta.Facts)
	}
	if beta.Facts[0].Value != "已停用" {
		t.Fatalf("停用条配置状态 = %q", beta.Facts[0].Value)
	}
	// 标题回退：宿主不产出 meta -> 短模块名（这里没有可剥的前缀，原样）。
	if alpha.Title != "plugin.wasm" {
		t.Fatalf("meta 缺席时标题应回退到模块名，实得 %q", alpha.Title)
	}
	// 角标：entryId 与标题不同才画。
	if alpha.EntryChip != "alpha" {
		t.Fatalf("角标 = %q，应为 entryId", alpha.EntryChip)
	}
	if gamma.Diagnostic == "" {
		t.Fatal("宿主给的 diagnostic 必须带进模型（详情里要画）")
	}
	if !plugins.ManagementAvailable {
		t.Fatal("宿主 managementAvailable=true 应如实带进来")
	}
}

func TestPluginsGlobalGroupFoldedByDefaultAndSearchForcesOpen(t *testing.T) {
	// 默认：全局段**收起**（官方 `globalOpen ?? false`）。
	model := BuildPanelWithPlugins(describeFixture(), fullDesktopFacts(), pluginsFixture(), PluginsView{})
	_, plugins := pluginsSection(t, model)
	if plugins.Groups[0].Open {
		t.Fatal("全局段默认必须收起（官方 :317）")
	}
	// 用户点开过：按用户的态。
	open := true
	model = BuildPanelWithPlugins(describeFixture(), fullDesktopFacts(), pluginsFixture(),
		PluginsView{GlobalOpen: &open})
	_, plugins = pluginsSection(t, model)
	if !plugins.Groups[0].Open {
		t.Fatal("用户展开过就应保持展开")
	}
	// 搜索：即使没点过也必须展开（官方 `searching ||`）。
	model = BuildPanelWithPlugins(describeFixture(), fullDesktopFacts(), pluginsFixture(),
		PluginsView{Query: "plugin"})
	_, plugins = pluginsSection(t, model)
	if !plugins.Groups[0].Open {
		t.Fatal("搜索时必须强制展开（官方 :315-317）")
	}
	if !plugins.Searching {
		t.Fatal("Searching 应为真")
	}
}

func TestPluginsSearchFiltersAndReportsNoMatch(t *testing.T) {
	// 命中模块名。
	model := BuildPanelWithPlugins(describeFixture(), fullDesktopFacts(), pluginsFixture(),
		PluginsView{Query: "other"})
	_, plugins := pluginsSection(t, model)
	if n := len(plugins.Groups[0].Cards); n != 1 {
		t.Fatalf("命中 1 张，实得 %d", n)
	}
	if plugins.Groups[0].Cards[0].RowKey != "global:beta" {
		t.Fatalf("命中的是 %q", plugins.Groups[0].Cards[0].RowKey)
	}
	if plugins.EmptySearch {
		t.Fatal("有命中时不该报 emptySearch")
	}

	// 没命中：官方 `emptySearch`，且**不**说"暂无插件"。
	model = BuildPanelWithPlugins(describeFixture(), fullDesktopFacts(), pluginsFixture(),
		PluginsView{Query: "zzz-not-there"})
	_, plugins = pluginsSection(t, model)
	if !plugins.EmptySearch {
		t.Fatal("无命中应报 emptySearch")
	}
	if !strings.Contains(plugins.Note, "没有匹配的插件。") {
		t.Fatalf("空搜索文案 = %q", plugins.Note)
	}
	if plugins.Empty {
		t.Fatal("有清单时不该报 Empty")
	}
	// 段还在（官方按 entries.length 判断画不画段），但段体为空。
	if len(plugins.Groups) != 1 || plugins.Groups[0].Count != 0 {
		t.Fatalf("无命中时全局段应仍在、计数为 0：%+v", plugins.Groups)
	}
}

func TestPluginsEmptyInventoryIsNotUnknown(t *testing.T) {
	empty := &apiclient.PluginInventoryResult{Entries: []apiclient.PluginInventoryEntry{}}
	model := BuildPanelWithPlugins(describeFixture(), fullDesktopFacts(), empty, PluginsView{})
	_, plugins := pluginsSection(t, model)
	if plugins.State != PluginsReady {
		t.Fatalf("空清单也是 ready（宿主答过了），实得 %q", plugins.State)
	}
	if !plugins.Empty || !strings.Contains(plugins.Note, "暂无插件。") {
		t.Fatalf("空清单文案 = %q（Empty=%v）", plugins.Note, plugins.Empty)
	}
	if len(plugins.Groups) != 0 {
		t.Fatalf("空清单不画全局段（官方 entries.length>0 才画）：%+v", plugins.Groups)
	}
	if plugins.ShowRetry {
		t.Fatal("ready 态没有重试按钮（官方只有 error 态有）")
	}
}

func TestPluginsHostNotWiredIsUnknownNotEmpty(t *testing.T) {
	model := BuildPanelWithPlugins(describeFixture(), fullDesktopFacts(), nil, PluginsView{})
	section, plugins := pluginsSection(t, model)
	if section.State != PanelUnknown {
		t.Fatalf("宿主没接清单时页状态应为 unknown，实得 %q", section.State)
	}
	if plugins.State != PluginsUnknown {
		t.Fatalf("插件页状态 = %q", plugins.State)
	}
	if len(plugins.Groups) != 0 {
		t.Fatal("没数据时一张卡都不许画")
	}
	if !plugins.ShowRetry {
		t.Fatal("unknown 态要给重试（对「没接」和「还没读到」都是正确下一步）")
	}
	if !strings.Contains(plugins.Note, "pluginInventory.list") {
		t.Fatalf("说明里要点名缺的是哪个方法：%q", plugins.Note)
	}
	if plugins.Empty {
		t.Fatal("没读到 ≠ 一个都没有")
	}
}

func TestPluginsExpandedCardCarriesDetails(t *testing.T) {
	model := BuildPanelWithPlugins(describeFixture(), fullDesktopFacts(), pluginsFixture(),
		PluginsView{GlobalOpen: boolPtr(true), Expanded: "global:alpha"})
	_, plugins := pluginsSection(t, model)
	var alpha PluginCard
	for _, card := range plugins.Groups[0].Cards {
		if card.RowKey == "global:alpha" {
			alpha = card
		}
	}
	if !alpha.Expanded {
		t.Fatal("Expanded 指定的卡应标记展开")
	}
	for _, card := range plugins.Groups[0].Cards {
		if card.RowKey != "global:alpha" && card.Expanded {
			t.Fatalf("手风琴只开一张，%q 却也展开了", card.RowKey)
		}
	}
}

func TestPluginsMetaFallbackAndLocalizedTitle(t *testing.T) {
	title := apiclient.PluginLocalizedText{Locales: map[string]string{"en": "English", "zh": "中文标题"}}
	desc := apiclient.PluginLocalizedText{Text: "一句简介"}
	entry := apiclient.PluginInventoryEntry{
		EntryID: "meta-entry", ModuleName: "@acme/dsh-host-thing",
		Meta:       &apiclient.PluginInventoryMeta{Title: title, Description: desc},
		Enabled:    true,
		FiberPhase: apiclient.PluginFiberPhase{Present: true, Phase: apiclient.PluginPhaseActive},
	}
	empty := &apiclient.PluginInventoryResult{Entries: []apiclient.PluginInventoryEntry{entry}}
	model := BuildPanelWithPlugins(describeFixture(), fullDesktopFacts(), empty, PluginsView{GlobalOpen: boolPtr(true)})
	_, plugins := pluginsSection(t, model)
	card := plugins.Groups[0].Cards[0]
	// 有 meta：标题按中文解析（官方 resolveText 的等价物），简介照给。
	if card.Title != "中文标题" || card.Desc != "一句简介" {
		t.Fatalf("带 meta 的卡 = %q / %q", card.Title, card.Desc)
	}
	// 角标：entryId 与标题不同 -> 画。
	if card.EntryChip != "meta-entry" {
		t.Fatalf("角标 = %q", card.EntryChip)
	}
}

func TestPluginShortNameStripsOfficialPrefixes(t *testing.T) {
	cases := map[string]string{
		"@deepseek-ai/dsh-client-ui-sidebar": "ui-sidebar",
		"cordis-plugin-timer":                "timer",
		"cordis:loader":                      "loader",
		"dsh-host-plugin-inventory":          "plugin-inventory",
		"dsh-client-ui-chat":                 "ui-chat",
		"plugin.wasm":                        "plugin.wasm",
		"":                                   "",
	}
	for in, want := range cases {
		if got := pluginShortName(in); got != want {
			t.Fatalf("pluginShortName(%q) = %q，应为 %q", in, got, want)
		}
	}
}

func TestPluginEntryChipLaws(t *testing.T) {
	// `include:` 前缀在 display 时剥掉。
	if got := pluginEntryChip("include:chat", "chat"); got != "" {
		t.Fatalf("剥掉 include: 后与标题相同就不画角标，实得 %q", got)
	}
	if got := pluginEntryChip("include:chat", "别的标题"); got != "chat" {
		t.Fatalf("剥掉前缀后不同 -> 画 %q", got)
	}
	// 空 entryId 不画（官方类型里它必填，这里只是防御）。
	if got := pluginEntryChip("", "x"); got != "" {
		t.Fatalf("空 entryId 不该画角标，实得 %q", got)
	}
}

func TestPluginPhaseTextMatchesOfficialZh(t *testing.T) {
	cases := map[string]string{
		"":                             "未运行",
		apiclient.PluginPhasePending:   "等待依赖",
		apiclient.PluginPhaseLoading:   "加载中",
		apiclient.PluginPhaseActive:    "运行中",
		apiclient.PluginPhaseFailed:    "启动失败",
		apiclient.PluginPhaseUnloading: "卸载中",
	}
	for phase, want := range cases {
		if got := pluginPhaseText(phase); got != want {
			t.Fatalf("pluginPhaseText(%q) = %q，官方 zh 是 %q", phase, got, want)
		}
	}
	// 只有"正在动"的三态有点（官方 showsPhaseDot）。
	for _, phase := range []string{apiclient.PluginPhasePending, apiclient.PluginPhaseLoading, apiclient.PluginPhaseUnloading} {
		if !pluginPhaseShowsDot(phase) {
			t.Fatalf("%q 应有阶段点", phase)
		}
	}
	for _, phase := range []string{"", apiclient.PluginPhaseActive, apiclient.PluginPhaseFailed} {
		if pluginPhaseShowsDot(phase) {
			t.Fatalf("%q 不该有阶段点（官方只在 pending/loading/unloading 画）", phase)
		}
	}
}

func TestPluginsAgentPresetsGapIsVisible(t *testing.T) {
	inv := pluginsFixture()
	inv.Entries = nil
	inv.AgentPresets = []apiclient.AgentPresetPluginGroup{{ID: "default", IsDefault: true}}
	model := BuildPanelWithPlugins(describeFixture(), fullDesktopFacts(), inv, PluginsView{})
	_, plugins := pluginsSection(t, model)
	if !plugins.HasAgentPresets {
		t.Fatal("宿主给了 agentPresets 时 HasAgentPresets 应为真")
	}
	// 本页没做会话插件段：必须**自己说出来**，不许静默少一段。
	if plugins.PresetGap == "" {
		t.Fatal("没实现的段必须登记成可见说明，不许静默丢")
	}
	if plugins.Empty {
		t.Fatal("有 agentPresets 面就不是「一个插件都没有」")
	}
}

func TestPluginsSectionTitleAndIntroAreOfficial(t *testing.T) {
	model := BuildPanelWithPlugins(describeFixture(), fullDesktopFacts(), pluginsFixture(), PluginsView{})
	section, _ := pluginsSection(t, model)
	// nav 标题来自 official_tree.go（ui-settings-plugins/index.ts:80 `t('nav')`）。
	if section.Title != "内置插件" {
		t.Fatalf("section 标题 = %q，官方 zh nav 是「内置插件」", section.Title)
	}
	if zhPluginsIntro != "查看内置部署的插件列表" {
		t.Fatalf("简介文案 = %q，官方 zh intro 是「查看内置部署的插件列表」", zhPluginsIntro)
	}
	if zhPluginsTabLabel != "插件列表" {
		t.Fatalf("tab 标签 = %q，官方 zh tab 是「插件列表」", zhPluginsTabLabel)
	}
}

func boolPtr(v bool) *bool { return &v }

func TestPluginsTabLedgerSingleTabHasNoBar(t *testing.T) {
	model := BuildPanelWithPlugins(describeFixture(), fullDesktopFacts(), pluginsFixture(), PluginsView{})
	_, plugins := pluginsSection(t, model)
	// 产品默认台账只有「插件列表」这一条（官方 inventory 注册的 id=all / order=10）。
	if len(plugins.Tabs) != 1 {
		t.Fatalf("默认台账 = %d 条，应只有只读清单一条：%+v", len(plugins.Tabs), plugins.Tabs)
	}
	if plugins.Tabs[0].ID != "all" || plugins.Tabs[0].Order != 10 ||
		plugins.Tabs[0].Label != "插件列表" || plugins.Tabs[0].Kind != PluginsTabInventory {
		t.Fatalf("默认 tab = %+v，官方是 id=all/order=10/label=插件列表/kind=inventory", plugins.Tabs[0])
	}
	// 只有一个 tab 时**不画 tab 条**（官方 :57-58）。
	if plugins.ShowTabBar {
		t.Fatal("单 tab 时不该出现 tab 条")
	}
	if plugins.ActiveTab != "all" {
		t.Fatalf("活动 tab = %q，应为第一项", plugins.ActiveTab)
	}
	if plugins.TabMissing != "" {
		t.Fatalf("单 tab 且真实现时不该报缺失：%q", plugins.TabMissing)
	}
}

func TestPluginsTabLedgerMultiTabShowsBarAndSorts(t *testing.T) {
	tabs := []PluginsTab{
		{ID: "manage", Order: 20, Label: "插件配置", Kind: PluginsTabManagable},
		{ID: "all", Order: 10, Label: "插件列表", Kind: PluginsTabInventory},
	}
	model := BuildPanelWithPlugins(describeFixture(), fullDesktopFacts(), pluginsFixture(),
		PluginsView{Tabs: tabs})
	_, plugins := pluginsSection(t, model)
	if !plugins.ShowTabBar {
		t.Fatal("多 tab 时必须画 tab 条（官方 :57-60）")
	}
	// 排序按 order（官方 `.sort((a,b) => a.order - b.order)`）。
	if plugins.Tabs[0].ID != "all" || plugins.Tabs[1].ID != "manage" {
		t.Fatalf("台账顺序 = %+v，应按 order 升序", plugins.Tabs)
	}
	if plugins.ActiveTab != "all" {
		t.Fatalf("未指定时活动 tab = %q，应为第一项", plugins.ActiveTab)
	}
	// 选中没有真实现的 tab：页面必须**明说**缺什么，不画空壳。
	model = BuildPanelWithPlugins(describeFixture(), fullDesktopFacts(), pluginsFixture(),
		PluginsView{Tabs: tabs, ActiveTab: "manage"})
	_, plugins = pluginsSection(t, model)
	if plugins.TabMissing == "" {
		t.Fatal("选中未实现的 tab 时应给出明确说明")
	}
	if !strings.Contains(plugins.TabMissing, "未实现") {
		t.Fatalf("说明文案 = %q", plugins.TabMissing)
	}
}

func TestPluginsTabLedgerFallsBackToFirstOnUnknownID(t *testing.T) {
	model := BuildPanelWithPlugins(describeFixture(), fullDesktopFacts(), pluginsFixture(),
		PluginsView{ActiveTab: "no-such-tab"})
	_, plugins := pluginsSection(t, model)
	// 官方：`rows.find(...)?.id ?? rows[0]?.id`。
	if plugins.ActiveTab != "all" {
		t.Fatalf("未知 tab id 应回退到第一项，实得 %q", plugins.ActiveTab)
	}
	if plugins.TabMissing != "" {
		t.Fatalf("回退到真实现的 tab 后不该报缺失：%q", plugins.TabMissing)
	}
}
