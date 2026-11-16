package dssidebar

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func cotTranslationDescribeFixture() *apiclient.SettingsDescribeResult {
	describe := settingsItemDescribeFixture()
	describe.Namespaces = append(describe.Namespaces, apiclient.SettingsNamespaceView{
		NS: settingsNSCotTranslation, Revision: 11,
	})
	describe.Sections = append(describe.Sections, apiclient.SettingsSectionView{
		NS: settingsNSCotTranslation, Writable: true, Revision: 11,
		Fields: []apiclient.SettingsFieldView{
			{Name: settingsFieldCotProvider, Path: []string{settingsFieldCotProvider}, Type: "string",
				Current: "deepseek-official", Default: "deepseek-official", Writable: true},
			{Name: settingsFieldCotTargetLanguage, Path: []string{settingsFieldCotTargetLanguage}, Type: "string",
				Current: "auto", Default: "auto", Writable: true},
		},
	})
	return describe
}

func TestCotTranslationSettingsCardPlansOfficialFields(t *testing.T) {
	describe := cotTranslationDescribeFixture()
	drafts := map[string]settingsItemDraft{
		settingsItemDraftKey(settingsNSCotTranslation, settingsFieldCotProvider):       {Text: "deepseek-account"},
		settingsItemDraftKey(settingsNSCotTranslation, settingsFieldCotTargetLanguage): {Text: "ja"},
	}
	card, ok := cotTranslationSettingsCard(describe, settingsItemFormSnapshot{drafts: drafts},
		apiclient.CotTranslationLimits{AvailableProviders: []string{"deepseek-official", "deepseek-account"}}, true)
	if !ok || card.Cot == nil || len(card.Sections) != 1 {
		t.Fatalf("cot 配置段未按 namespace 出现：%+v", card)
	}
	if len(card.Plan.Sections) != 1 || len(card.Plan.Sections[0].Ops) != 2 {
		t.Fatalf("provider + targetLanguage 应同事务保存：%+v", card.Plan)
	}
	ops := card.Plan.Sections[0].Ops
	if ops[0].Path[0] != settingsFieldCotProvider || ops[0].Value != "deepseek-account" ||
		ops[1].Path[0] != settingsFieldCotTargetLanguage || ops[1].Value != "ja" {
		t.Fatalf("保存操作不对：%+v", ops)
	}
	if card.Plan.Sections[0].NS != settingsNSCotTranslation {
		t.Fatalf("写入 ns=%q", card.Plan.Sections[0].NS)
	}

	// 没有官方 namespace 时配置段整张不存在，不能靠 bundle 名假画。
	if _, ok := cotTranslationSettingsCard(settingsItemDescribeFixture(), settingsItemFormSnapshot{},
		apiclient.CotTranslationLimits{}, false); ok {
		t.Fatal("宿主未服务 cot-translation 时不得画配置段")
	}
}

func TestCotTranslationFieldParsingMatchesOfficial(t *testing.T) {
	if value, clear, ok := parseCotProviderDraft("  "); !ok || !clear || value != nil {
		t.Fatalf("provider 空白应是 clear：value=%v clear=%v ok=%v", value, clear, ok)
	}
	if value, clear, ok := parseCotProviderDraft(" deepseek-account "); !ok || clear || value != "deepseek-account" {
		t.Fatalf("合法 provider 解析失败：value=%v clear=%v ok=%v", value, clear, ok)
	}
	if _, _, ok := parseCotProviderDraft("bogus"); ok {
		t.Fatal("provider 非官方枚举必须 invalid")
	}
	for _, valid := range []string{"auto", "zh", "zh-Hans", "en-US", "ja"} {
		if value, clear, ok := parseCotTargetLanguageDraft(" " + valid + " "); !ok || clear || value != valid {
			t.Fatalf("targetLanguage=%q 应合法：value=%v clear=%v ok=%v", valid, value, clear, ok)
		}
	}
	for _, invalid := range []string{"", "zh_CN", "x", "auto-"} {
		if _, _, ok := parseCotTargetLanguageDraft(invalid); ok {
			t.Fatalf("targetLanguage=%q 必须 invalid", invalid)
		}
	}
}
