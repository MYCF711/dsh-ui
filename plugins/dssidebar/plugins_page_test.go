// plugins_page_test.go —— 官方插件管理页（#29）里**能脱离真机验证的判断**。
//
// 绘制本身由无头截图看（本包既有权变测试管外观）；这里钉住三类东西：
//
//	① 数据 → 行的投影：官方 `packageStatus` / `packageText` / `versionTag` 的等价物；
//	② 状态机：开关"在飞"标记只在宿主收敛（或失败）后清除，不做乐观更新；
//	③ 一帧真画：卡片、空态、unavailable 态画出来的字与官方文案逐字一致。
package dssidebar

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func pluginBundleFixture() apiclient.PluginBundle {
	return apiclient.PluginBundle{
		Name: "whale-pet", Availability: "profile", Version: "1.2.3",
		Enabled: true, Installed: true, Source: "/tmp/whale-pet", Removable: true,
		Rows:      []apiclient.PluginBundleRow{},
		Overrides: []string{},
		Meta: &apiclient.PluginBundleMeta{
			Title:       apiclient.PluginLocalizedText{Locales: map[string]string{"zh": "鲸鱼宠物"}},
			Description: apiclient.PluginLocalizedText{Text: "包简介"},
		},
	}
}

// TestPluginStatusMirrorsOfficialPackageStatus 钉住官方 `packageStatus`
// （OFF:PluginManagerPage.tsx:434-438）在本仓 bundle 形状（rows 恒空）下的取值。
func TestPluginStatusMirrorsOfficialPackageStatus(t *testing.T) {
	b := pluginBundleFixture()

	if got := pluginStatusOf(b); got != "running" {
		t.Fatalf("启用的正常 bundle = %q，want running", got)
	}
	b.Enabled = false
	if got := pluginStatusOf(b); got != "disabled" {
		t.Fatalf("停用的 bundle = %q，want disabled", got)
	}
	// 宿主报错 + 启用 + 没有任何带 entryId 的行（本仓恒如此）→ 官方判 not-running。
	b.Enabled = true
	b.Error = &apiclient.PluginBundleError{Code: "unsupported-platform"}
	if got := pluginStatusOf(b); got != "not-running" {
		t.Fatalf("启用但宿主报错 = %q，want not-running", got)
	}
	b.Enabled = false
	if got := pluginStatusOf(b); got != "problem" {
		t.Fatalf("停用且宿主报错 = %q，want problem", got)
	}

	if pluginStatusTag("running") != "" || pluginStatusTag("disabled") != "" {
		t.Fatal("running/disabled 不带标签（官方 STATUS_TAG_KEYS 只有另两态）")
	}
	if pluginStatusTag("not-running") != zhPluginStatusOff || pluginStatusTag("problem") != zhPluginStatusProb {
		t.Fatal("状态标签文案必须是官方 zh 字典原文")
	}
}

// TestPluginReasonTextFollowsOfficialCodes 钉住官方 `managementText` 的映射。
func TestPluginReasonTextFollowsOfficialCodes(t *testing.T) {
	cases := map[string]string{
		"management-required":  "插件管理所需，不能停用或卸载",
		"unknown-plugin":       "找不到该插件",
		"not-removable":        "这个包不属于当前 profile，或者是插件管理所需的组件",
		"unsupported-platform": "此插件不支持运行 DSH 的主机操作系统",
		"incompatible-version": "这个插件与当前 DSH 版本不兼容，运行它可能导致崩溃或数据丢失。",
		"unexpected-package":   "插件来源安装了与所请求不同的包或版本，已撤销这次安装。请向插件发布者或插件索引维护者报告此问题。",
	}
	for code, want := range cases {
		if got := pluginReasonText(code, ""); got != want {
			t.Fatalf("pluginReasonText(%q) = %q，want %q", code, got, want)
		}
	}
	// operation-error 按官方规则优先回宿主诊断原文；没有诊断才用兜底句。
	if got := pluginReasonText("operation-error", "磁盘满了"); got != "磁盘满了" {
		t.Fatalf("operation-error 应回宿主原文，实得 %q", got)
	}
	if got := pluginReasonText("operation-error", ""); got != zhPluginReasonOp {
		t.Fatalf("operation-error 无诊断应回兜底句，实得 %q", got)
	}
	// 未知 code（含宿主 `unsupported_plugin` 落成的 operation-error）走兜底。
	if got := pluginReasonText("something-new", "x"); got != zhPluginReasonOp {
		t.Fatalf("未知 code 应回兜底句，实得 %q", got)
	}
}

// TestPluginVersionTagSkipsEmptyVersion 钉住官方 `versionTag` 的"没有就不画"。
func TestPluginVersionTagSkipsEmptyVersion(t *testing.T) {
	b := pluginBundleFixture()
	if got := pluginVersionTag(b); got != "v1.2.3" {
		t.Fatalf("版本标签 = %q，want v1.2.3", got)
	}
	b.Version = ""
	if got := pluginVersionTag(b); got != "" {
		t.Fatalf("没有版本时应整枚不画，实得 %q", got)
	}
}

// TestPluginBusyClearsOnlyAfterHostConverges 钉住"不乐观更新"。
func TestPluginBusyClearsOnlyAfterHostConverges(t *testing.T) {
	p := &Plugin{pluginWant: map[string]bool{"whale-pet": true}}
	// 宿主还是旧值（false）→ 仍在飞，开关禁用。
	if !p.pluginBusy("whale-pet", false) {
		t.Fatal("宿主还没收敛时开关必须保持禁用")
	}
	// 宿主收敛到期望值 → 标记清除，且下一次直接放行。
	if p.pluginBusy("whale-pet", true) {
		t.Fatal("宿主已收敛，开关不该继续禁用")
	}
	if _, still := p.pluginWant["whale-pet"]; still {
		t.Fatal("收敛后必须把在飞标记清掉")
	}
	if p.pluginBusy("whale-pet", true) {
		t.Fatal("标记清掉后不该再报在飞")
	}
	// 失败路径：壳带回的 name 让页面清掉标记（不靠计时器）。
	p.pluginWant["whale-pet"] = true
	delete(p.pluginWant, "whale-pet")
	if p.pluginBusy("whale-pet", false) {
		t.Fatal("失败后必须立刻解除禁用（否则开关会永远卡住）")
	}
}

// TestPluginPageDrawsRealBundleCard 走一遍真绘制：卡片上的字必须是官方文案 +
// 宿主真值，且不能出现空态那句。
func TestPluginPageDrawsRealBundleCard(t *testing.T) {
	p, _, env, _, canvas, _ := searchFixture(t)
	env.Act.PluginBundles = func() ([]apiclient.PluginBundle, bool, bool, string) {
		return []apiclient.PluginBundle{pluginBundleFixture()}, true, false, ""
	}
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: 900, Height: 600}, func(f ui.C) {
		p.drawPluginManagerPage(f, env)
	})

	for _, want := range []string{zhPluginTitle, zhPluginIntro, zhPluginBundlesTtl, "鲸鱼宠物", "v1.2.3", "包简介"} {
		if !fg2HasText(canvas, want) {
			t.Fatalf("卡片缺官方文案/宿主真值 %q", want)
		}
	}
	if fg2HasText(canvas, zhPluginEmpty) {
		t.Fatal("有插件时不该画空态")
	}
	if fg2HasText(canvas, zhPluginUnavail) {
		t.Fatal("宿主有能力时不该画 unavailable")
	}
}

// TestPluginPageUnavailableHidesRetry 钉住 501 的处置：画官方 unavailable，
// **不画重试**（重试一个不存在的方法永远同一个 501）。
func TestPluginPageUnavailableHidesRetry(t *testing.T) {
	p, _, env, _, canvas, _ := searchFixture(t)
	env.Act.PluginBundles = func() ([]apiclient.PluginBundle, bool, bool, string) {
		return nil, false, true, "not_implemented: 官方 pluginManager API 未接线"
	}
	env.Act.PluginBundlesRefresh = func() { t.Fatal("unavailable 态不该画重试入口") }
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: 900, Height: 600}, func(f ui.C) {
		p.drawPluginManagerPage(f, env)
	})
	if !fg2HasText(canvas, zhPluginUnavail) {
		t.Fatal("501 必须画官方 unavailable 文案")
	}
	if fg2HasText(canvas, zhPluginRetry) {
		t.Fatal("unavailable 态不该画重试")
	}
}

// TestPluginDetailRoundTrip 钉住详情页的本地状态迁移与"清单里消失即退出"。
func TestPluginDetailRoundTrip(t *testing.T) {
	p := &Plugin{}
	p.openPluginDetail("whale-pet")
	if p.pluginSelected != "whale-pet" {
		t.Fatalf("打开详情后 selected=%q", p.pluginSelected)
	}
	p.pluginRemoveArm = "whale-pet"
	p.closePluginDetail()
	if p.pluginSelected != "" || p.pluginRemoveArm != "" {
		t.Fatalf("退出详情必须同时清掉上膛态：%q/%q", p.pluginSelected, p.pluginRemoveArm)
	}
	if _, ok := findPluginBundle([]apiclient.PluginBundle{pluginBundleFixture()}, "whale-pet"); !ok {
		t.Fatal("清单里存在的 bundle 必须能找到")
	}
	if _, ok := findPluginBundle([]apiclient.PluginBundle{pluginBundleFixture()}, "gone"); ok {
		t.Fatal("清单里没有的 bundle 不该被找到")
	}
}

// TestPluginBundlesKeepHostOrder 钉住分组顺序：官方对 `mine` 不做排序，
// 宿主给什么画什么（排序会让"刚启用的插件跳位置"）。
func TestPluginBundlesKeepHostOrder(t *testing.T) {
	in := []apiclient.PluginBundle{{Name: "b"}, {Name: "a"}, {Name: "c"}}
	out := pluginSortedBundles(in)
	if len(out) != 3 || out[0].Name != "b" || out[1].Name != "a" || out[2].Name != "c" {
		t.Fatalf("顺序被改了：%+v", out)
	}
	out[0].Name = "mutated"
	if in[0].Name != "b" {
		t.Fatal("必须返回副本，不能原地改调用方的切片")
	}
}
