package dssettings

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
)

// 宿主提供壳渠道服务时，「更新渠道」行必须出现，且**选项顺序与文案照官方**。
//
// 官方 DesktopChannelRow.tsx:34：`items={[{id:'stable'},{id:'nightly'}]}` —— 正式版在前。
// 官方 locales.ts:7-8：zh 是「正式版 / 开发版」，不是 Stable / Nightly。
func TestDesktopChannelRowAppearsWithOfficialOrderAndLabels(t *testing.T) {
	facts := fullDesktopFacts()
	facts.ShellChannelServed = true
	channel := "nightly"
	facts.ShellChannel = &channel

	model := BuildPanel(describeFixture(), facts)
	panel, ok := model.Section("general")
	if !ok {
		t.Fatal("BuildPanel 没有 general 页")
	}
	row, ok := panel.Row("desktop-update-channel")
	if !ok {
		t.Fatal("宿主提供了壳渠道服务时，「更新渠道」行必须出现")
	}
	if len(row.Choices) != 2 {
		t.Fatalf("应当有两个渠道选项，实得 %d", len(row.Choices))
	}
	if row.Choices[0].Value != "stable" || row.Choices[1].Value != "nightly" {
		t.Fatalf("选项顺序必须是官方顺序（stable 在前）：%+v", row.Choices)
	}
	if row.Choices[0].Label != "正式版" || row.Choices[1].Label != "开发版" {
		t.Fatalf("文案必须是官方 zh 字典「正式版 / 开发版」：%+v", row.Choices)
	}
	if !row.Known || row.Value != "nightly" {
		t.Fatalf("当前渠道应显示宿主的真值：known=%v value=%q", row.Known, row.Value)
	}
}

// **没选过 ≠ stable**：官方 `channel: null` 时界面显示「配置不可用」，
// 而不是把 packaged 当成用户选择。
func TestDesktopChannelRowShowsUnavailableWhenUnselected(t *testing.T) {
	facts := fullDesktopFacts()
	facts.ShellChannelServed = true
	facts.ShellChannel = nil

	model := BuildPanel(describeFixture(), facts)
	panel, _ := model.Section("general")
	row, ok := panel.Row("desktop-update-channel")
	if !ok {
		t.Fatal("行必须出现")
	}
	if row.Known {
		t.Fatalf("没选过时不该是「已知值」，实得 %q", row.Value)
	}
	if row.Value != "配置不可用" {
		t.Fatalf("没选过时应当显示官方文案「配置不可用」，实得 %q", row.Value)
	}
	if strings.Contains(row.Value, "stable") {
		t.Fatal("不许把没选过显示成 stable —— 那是自造事实")
	}
}

// 宿主**没有**这条能力时整行不出现（而不是出现一个「未知」）。
func TestDesktopChannelRowAbsentWhenHostLacksService(t *testing.T) {
	facts := fullDesktopFacts()
	facts.ShellChannelServed = false
	facts.DesktopProtocolVersion = 0

	model := BuildPanel(describeFixture(), facts)
	panel, _ := model.Section("general")
	if _, ok := panel.Row("desktop-update-channel"); ok {
		t.Fatal("宿主没这条能力时整行不该出现")
	}
}

// 壳级故障要在行内说出来（官方 DesktopChannelRow.tsx:30 的 role="alert"）。
func TestDesktopChannelRowReportsShellFailure(t *testing.T) {
	facts := fullDesktopFacts()
	facts.ShellChannelServed = true
	facts.ShellChannelFailure = "format"

	model := BuildPanel(describeFixture(), facts)
	panel, _ := model.Section("general")
	row, ok := panel.Row("desktop-update-channel")
	if !ok {
		t.Fatal("行必须出现（故障也要让用户看到）")
	}
	if !row.Disabled {
		t.Fatal("壳级故障时该行应当不可操作")
	}
	if !strings.Contains(row.Reason, "format") {
		t.Fatalf("行内要说出故障码，实得 %q", row.Reason)
	}
}

// TestFromAPIHostFactsCarriesShellChannel 钉住**转换函数**不丢字段。
//
// 为什么单独钉这一条：`pluginapi.SettingsHostFacts` 与 `dssettings.HostFacts`
// 是两个类型，中间靠 `fromAPIHostFacts` 搬。**漏搬一个字段的后果是静默的**：
// 探针照样成功、事实位照样为真、`settingsHostFacts()` 返回的东西也对，
// 但面板永远看不到 -> 「更新渠道」行永远不出现。
//
// 2026-11-13 真机上就是这么发生的：截图看不到那一行（order 95 在面板很下面），
// 端到端测试才把它抓出来。
func TestFromAPIHostFactsCarriesShellChannel(t *testing.T) {
	channel := "nightly"
	from := pluginapi.SettingsHostFacts{
		ShellChannelServed:  true,
		ShellChannel:        &channel,
		ShellChannelFailure: "format",
	}
	got := fromAPIHostFacts(from)
	if !got.ShellChannelServed {
		t.Fatal("ShellChannelServed 被转换函数丢掉了")
	}
	if got.ShellChannel == nil || *got.ShellChannel != "nightly" {
		t.Fatalf("ShellChannel 被转换函数丢掉了：%v", got.ShellChannel)
	}
	if got.ShellChannelFailure != "format" {
		t.Fatalf("ShellChannelFailure 被转换函数丢掉了：%q", got.ShellChannelFailure)
	}
}

// TestShortcutsRowUsesWindowCatalogNotHostNamespace 钉住「快捷键」行的真值来源。
//
// 官方 `shortcuts` 是**窗口本地**服务（`dsh-client-shortcuts/src/client/index.ts:31`
// 的 `ctx.shortcuts.catalog`），键位来自 `DesktopShortcutsApi` 适配器 ——
// **不是 Host 设置**。本仓原先把它的 HostField 写成 `shortcuts.catalog` 并走
// settings 查询，真机上就一直显示橙色告警「宿主未提供 shortcuts 命名空间」——
// 那是一句**假**结论（宿主本来就不该提供它）。
func TestShortcutsRowUsesWindowCatalogNotHostNamespace(t *testing.T) {
	facts := fullDesktopFacts()
	model := BuildPanel(describeFixture(), facts)
	panel, ok := model.Section("general")
	if !ok {
		t.Fatal("没有 general 页")
	}
	row, ok := panel.Row("shortcuts")
	if !ok {
		t.Fatal("缺 shortcuts 行")
	}
	if !row.Known || row.Value != "13 条键位" {
		t.Fatalf("应显示本窗口键位数，实得 known=%v value=%q", row.Known, row.Value)
	}
	if strings.Contains(row.Reason, "命名空间") {
		t.Fatalf("不该再说「宿主未提供 … 命名空间」——它不是 Host 设置：%q", row.Reason)
	}
	if row.Disabled {
		t.Fatal("这一行不该被标成只读/禁用")
	}
}

// 键位表没装（0 条）时保持未知，**不**谎报「0 条键位」。
func TestShortcutsRowUnknownWhenCatalogEmpty(t *testing.T) {
	facts := fullDesktopFacts()
	facts.ShortcutCount = 0
	model := BuildPanel(describeFixture(), facts)
	panel, _ := model.Section("general")
	row, ok := panel.Row("shortcuts")
	if !ok {
		t.Fatal("缺 shortcuts 行")
	}
	if row.Known {
		t.Fatalf("键位表为空时不该有真值，实得 %q", row.Value)
	}
}
