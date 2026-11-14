//go:build darwin || windows || linux

// official_plugins_visual_test.go —— 「内置插件」页的**真实绘制**证据。
//
// 走真 Viva：真 Paint（经真 dsshell 的 shell.overlay 槽）→ 真 HitTest → 真 Click → 再 Paint。
// 判据不是「模型里算对了」（那是 official_plugins_test.go 的事），而是：
//
//   - 宿主给了清单之后，**画面上真的出现**条目的模块名与状态（不是空壳/未知）；
//   - 全局段默认收起时卡片不画，点段标题之后**真的**画出来；
//   - 点卡片**真的**展开出「完整名称/配置状态/运行状态」；
//   - 搜索框是**真输入框**（有命中区、能聚焦），不是画了一个框；
//   - 宿主没接清单时画面上是「未知 + 重试」，点重试**真的**触发刷新动作。
//
// 截图只在断言通过之后落盘（`SETTINGS_CAPTURE_DIR` 非空时保留），与官方四区面板
// 的视觉测试同一个开关。
package dssettings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/vivaui/viva/gfx"
)

// inventoryFixtureForDraw 造一份真宿主同形的清单（entryId=bundle name、
// moduleName=manifest entry、fiberPhase 只有 active/null）。
func inventoryFixtureForDraw() *apiclient.PluginInventoryResult {
	management := true
	active := apiclient.PluginFiberPhase{Present: true, Phase: apiclient.PluginPhaseActive}
	null := apiclient.PluginFiberPhase{Present: true}
	return &apiclient.PluginInventoryResult{
		ManagementAvailable: &management,
		Entries: []apiclient.PluginInventoryEntry{
			{EntryID: "hello", ModuleName: "plugin.wasm", Enabled: true, FiberPhase: active},
			{EntryID: "off", ModuleName: "other.wasm", Enabled: false, FiberPhase: null},
		},
	}
}

// openPluginsPage 打开设置面板并切到「内置插件」页。
func openPluginsPage(t *testing.T, h *officialHarness) {
	t.Helper()
	if err := h.session.Driver().Key(gfx.KeyF2, gfx.ModifierShift); err != nil {
		t.Fatalf("Shift+F2：%v", err)
	}
	h.pump()
	if !h.p.OfficialShown() {
		t.Fatal("Shift+F2 没打开设置面板")
	}
	h.click("settings-nav-plugins")
	if got := h.p.ActiveSection(); got != "plugins" {
		t.Fatalf("点导航后活动 section = %q，应为 plugins", got)
	}
}

// capturePlugins 在 SETTINGS_CAPTURE_DIR 设置时落一张截图（只做事后证据）。
func capturePlugins(t *testing.T, h *officialHarness, name string) {
	t.Helper()
	dir := os.Getenv("SETTINGS_CAPTURE_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建截图目录：%v", err)
	}
	if err := h.session.Screenshot(filepath.Join(dir, name)); err != nil {
		t.Fatalf("截图：%v", err)
	}
}

// TestPluginsPageDrawsRealInventory 是主证据：宿主清单里的模块名与状态真的画在屏幕上。
func TestPluginsPageDrawsRealInventory(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	h.mu.Lock()
	h.inventory = inventoryFixtureForDraw()
	h.mu.Unlock()

	openPluginsPage(t, h)

	// 页头（官方外壳）：标题 + 简介。
	if !h.hasText("内置插件") {
		t.Fatal("页面上没有「内置插件」标题")
	}
	if !h.hasText("查看内置部署的插件列表") {
		t.Fatal("页面上没有官方简介「查看内置部署的插件列表」")
	}
	// 全局段标题与副标题。
	if !h.hasText("全局插件") || !h.hasText("系统与所有会话共用") {
		t.Fatal("页面上没有全局段标题/副标题")
	}
	// **默认收起**：卡片此时不该在画面上（官方同默认态）。
	if h.hasText("plugin.wasm") {
		t.Fatal("全局段默认收起时不该已经画出卡片")
	}

	h.click("settings-plugins-group-global")

	// 段展开后：两条真实条目（模块名）与各自的状态标签都要出现。
	for _, want := range []string{"plugin.wasm", "other.wasm", "已停用"} {
		if !h.hasText(want) {
			t.Fatalf("段展开后画面上缺 %q；文本=%v", want, h.session.Texts())
		}
	}
	// entryId 角标（与标题不同才画）。
	if !h.hasText("hello") {
		t.Fatalf("卡片缺 entry id 角标；文本=%v", h.session.Texts())
	}
	capturePlugins(t, h, "plugins-page-expanded.png")
}

// TestPluginsCardExpandsToFacts 点卡片真的展开官方那三行事实。
func TestPluginsCardExpandsToFacts(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	h.mu.Lock()
	h.inventory = inventoryFixtureForDraw()
	h.mu.Unlock()

	openPluginsPage(t, h)
	h.click("settings-plugins-group-global")

	// 展开前没有详情标签。
	if h.hasText("完整名称") {
		t.Fatal("未点卡片时不该有详情")
	}
	h.click("settings-plugins-card-global:hello")

	for _, want := range []string{"完整名称", "配置状态", "已启用", "运行状态", "运行中"} {
		if !h.hasText(want) {
			t.Fatalf("展开后缺 %q；文本=%v", want, h.session.Texts())
		}
	}
	capturePlugins(t, h, "plugins-card-expanded.png")

	// 手风琴：再点一次收起。
	h.click("settings-plugins-card-global:hello")
	if h.hasText("完整名称") {
		t.Fatal("再点一次应收起详情（官方手风琴）")
	}
}

// TestPluginsPageUnknownDrawsRetryAndTriggersRefresh 宿主没接清单时画「未知 + 重试」，
// 点重试真的走到壳的刷新动作上。
func TestPluginsPageUnknownDrawsRetryAndTriggersRefresh(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()

	openPluginsPage(t, h)

	texts := strings.Join(h.session.Texts(), "|")
	if !strings.Contains(texts, "pluginInventory.list") {
		t.Fatalf("未知态要点名缺哪个方法；文本=%v", texts)
	}
	if !h.hasText("重试") {
		t.Fatalf("未知态要有「重试」")
	}
	if h.hasText("plugin.wasm") {
		t.Fatal("没数据时一张卡都不许画")
	}

	h.click("settings-plugins-retry")
	h.mu.Lock()
	refresh := h.inventoryRefresh
	h.mu.Unlock()
	if refresh != 1 {
		t.Fatalf("点重试后刷新动作被调 %d 次，应为 1", refresh)
	}
}

// TestPluginsSearchFieldIsRealInput 搜索框是真输入框：有命中区、打字能过滤。
func TestPluginsSearchFieldIsRealInput(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	h.mu.Lock()
	h.inventory = inventoryFixtureForDraw()
	h.mu.Unlock()

	openPluginsPage(t, h)
	// 输入框必须有真实命中区（不是画了一个框）。
	h.hitFor("settings-plugins-search")

	// 点**可访问名**聚焦（官方那个框的 aria-label 就是 `t('search')`）。
	if err := h.session.ClickLabel("搜索插件"); err != nil {
		t.Fatalf("点搜索框：%v", err)
	}
	if err := h.session.Type("other"); err != nil {
		t.Fatalf("往搜索框打字：%v", err)
	}
	h.pump()

	// 搜索会强制展开全局段，并且只留命中那一条。
	if !h.hasText("other.wasm") {
		t.Fatalf("搜索 other 之后应命中 other.wasm；文本=%v", h.session.Texts())
	}
	if h.hasText("plugin.wasm") {
		t.Fatalf("搜索 other 之后不该还画着 plugin.wasm；文本=%v", h.session.Texts())
	}
	capturePlugins(t, h, "plugins-search.png")
}

// TestPluginsPageScrollsThroughManyEntries 真有几十条插件时，清单进滚动容器：
// 首屏条目照画、全量条目都在（不是被截断成前 N 条）。
func TestPluginsPageScrollsThroughManyEntries(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	active := apiclient.PluginFiberPhase{Present: true, Phase: apiclient.PluginPhaseActive}
	var entries []apiclient.PluginInventoryEntry
	for i := 0; i < 40; i++ {
		name := "mod" + string(rune('a'+i%26)) + string(rune('0'+i/26)) + ".wasm"
		entries = append(entries, apiclient.PluginInventoryEntry{
			EntryID: "entry" + name, ModuleName: name, Enabled: true, FiberPhase: active,
		})
	}
	h.mu.Lock()
	h.inventory = &apiclient.PluginInventoryResult{Entries: entries}
	h.mu.Unlock()

	openPluginsPage(t, h)
	h.click("settings-plugins-group-global")

	// 第一张卡必须在画面上（滚动区首屏）。
	if !h.hasText("moda0.wasm") {
		t.Fatalf("首屏缺第一条；文本=%v", h.session.Texts())
	}
	// 40 条都进了渲染（滚动容器负责露出后面的，不是只画前几条）。
	model := h.p.PanelModel()
	section, ok := model.Section("plugins")
	if !ok || section.Plugins == nil {
		t.Fatal("模型里没有插件页")
	}
	if got := len(section.Plugins.Groups[0].Cards); got != 40 {
		t.Fatalf("卡片 %d 张，应把 40 条全投影出来", got)
	}
	capturePlugins(t, h, "plugins-many.png")
}
