//go:build darwin || windows || linux

package dssidebar

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func openCotSettingsDetail(t *testing.T, h *settingsItemHarness) {
	t.Helper()
	bundle := pluginBundleFixture()
	bundle.Name = cotTranslationBundleName
	bundle.Meta.Title.Locales["zh"] = "思考翻译"
	h.mu.Lock()
	h.bundles = []apiclient.PluginBundle{bundle}
	h.mu.Unlock()
	if !h.p.OpenPluginDetail(cotTranslationBundleName) {
		t.Fatal("无法打开 cot bundle 详情")
	}
	h.pump()
	h.pump()
}

func TestCotTranslationBundleConfigDrawsRefreshesAndSaves(t *testing.T) {
	h := openSettingsItemHarness(t, 960, 1100)
	defer h.close()
	h.mu.Lock()
	h.describe = cotTranslationDescribeFixture()
	h.mu.Unlock()
	openCotSettingsDetail(t, h)

	for _, want := range []string{cotPrivacyNotice, cotPaidNotice, "翻译服务", "目标语言", zhFormSave} {
		if !strings.Contains(strings.Join(h.session.Texts(), ""), want) {
			t.Fatalf("plugins.bundle.config 缺官方文案 %q；texts=%q", want, h.session.Texts())
		}
	}
	h.mu.Lock()
	refreshes := h.cotRefreshes
	h.mu.Unlock()
	if refreshes != 1 {
		h.t.Fatalf("配置段挂载应 refreshProviders 一次，实际 %d", refreshes)
	}

	h.click(cotTranslationProviderDOMID)
	h.clickText("DeepSeek Flash · 账号（付费）")
	h.p.itemFields.cotTargetLanguage.SetText("ja")
	h.pump()
	h.pump()
	drafts, _, _ := h.p.ensureSettingsItemForm().snapshot()
	if got := drafts[settingsItemDraftKey(settingsNSCotTranslation, settingsFieldCotProvider)].Text; got != "deepseek-account" {
		h.t.Fatalf("provider 草稿=%q", got)
	}
	if got := drafts[settingsItemDraftKey(settingsNSCotTranslation, settingsFieldCotTargetLanguage)].Text; got != "ja" {
		h.t.Fatalf("targetLanguage 草稿=%q", got)
	}
	before := h.spy.count()
	h.click("settings-item-save-cot-translation")
	if !h.waitMutate(before + 1) {
		t.Fatal("保存没有发 settings.mutate")
	}
	call, _ := h.spy.last()
	if call.ns != settingsNSCotTranslation || call.expectedRe != 11 || len(call.ops) != 2 {
		h.t.Fatalf("cot 写路径/revision/字段数不对：%+v", call)
	}
	if call.ops[0].Value != "deepseek-account" || call.ops[1].Value != "ja" {
		h.t.Fatalf("cot 写入值不对：%+v", call.ops)
	}
	h.capture("cot-translation-bundle-config.png")
}

func TestCotTranslationUnavailableProviderRemainsVisible(t *testing.T) {
	h := openSettingsItemHarness(t, 960, 1100)
	defer h.close()
	describe := cotTranslationDescribeFixture()
	describe.Sections[len(describe.Sections)-1].Fields[0].Current = "bing"
	h.mu.Lock()
	h.describe = describe
	h.cotLimits.Preferences.Provider = "bing"
	h.mu.Unlock()
	openCotSettingsDetail(t, h)
	if !strings.Contains(strings.Join(h.session.Texts(), ""), "Bing — 不可用") ||
		!strings.Contains(strings.Join(h.session.Texts(), ""), cotProviderUnavail) {
		h.t.Fatalf("失效 provider 必须保留 disabled option + 官方说明；texts=%q", h.session.Texts())
	}
	if !h.hasText(cotAnonymousNotice) {
		h.t.Fatalf("匿名 provider 的说明不对；texts=%q", h.session.Texts())
	}
}
