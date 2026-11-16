package dssidebar

// #63 `dsh-experimental-client-ui-cot-translation` 的官方第二处座位：
// `plugins.bundle.config`（不是 `plugins.item` 的内置配置卡）。
//
// 权威源：
//   - mount.ts:81,105-111：configForms.get('cot-translation')，entryKey 是完整 bundle 名。
//   - PluginManagerPage.tsx:817-825：配置段位于 detail 之后、组件清单之前。
//   - TranslationSettings.tsx:35-66：隐私/付费说明、provider select、失效当前值、
//     targetLanguage 文本字段，以及同一 SettingsFormModel 的保存/恢复默认。
//   - preferences-form.ts:25-36：两个字段各自的 parse，不把 targetLanguage 空白当 clear。

import (
	"regexp"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

const (
	settingsNSCotTranslation          = apiclient.SettingsNSCotTranslation
	settingsFieldCotProvider          = "provider"
	settingsFieldCotTargetLanguage    = "targetLanguage"
	cotTranslationBundleName          = "@deepseek-ai/dsh-experimental-cot-translation-bundle"
	cotTranslationProviderDOMID       = "plugin-config-cot-translation-provider"
	cotTranslationTargetLanguageDOMID = "plugin-config-cot-translation-language"
)

var cotTargetLanguagePattern = regexp.MustCompile(`^(?:auto|[a-zA-Z]{2,8}(?:-[a-zA-Z0-9]{1,8})*)$`)

var cotProviderPreferences = []string{"bing", "google", "deepseek-account", "deepseek-official"}

// settingsCotState 是 limits 发布的可用 provider 快照。Loaded=false 时只画当前值，
// 不把"还没读到"误报成"当前 provider 不可用"。
type settingsCotState struct {
	Loaded             bool
	AvailableProviders []string
}

func cotTranslationSettingsItem() settingsItem {
	return settingsItem{
		ID:     "cot-translation",
		Title:  "思考翻译",
		Kind:   "cot-translation",
		Anchor: "dsh-experimental-client-ui-cot-translation/src/client/mount.ts:105-111",
		Sections: []settingsItemSection{{
			NS: settingsNSCotTranslation,
			Fields: []settingsItemField{
				{
					Name: settingsFieldCotProvider, Kind: settingsItemFieldProvider,
					Label: "翻译服务", Invalid: "请选择翻译服务", DOMID: cotTranslationProviderDOMID,
				},
				{
					Name: settingsFieldCotTargetLanguage, Kind: settingsItemFieldTargetLanguage,
					Label: "目标语言", Hint: "auto 使用界面语言；也可填写语言代码，例如 zh、en、ja",
					Invalid: "请填写 auto 或语言代码，例如 zh、en、ja", DOMID: cotTranslationTargetLanguageDOMID,
				},
			},
		}},
	}
}

func cotTranslationSettingsCard(describe *apiclient.SettingsDescribeResult,
	snapshot settingsItemFormSnapshot, limits apiclient.CotTranslationLimits, loaded bool) (settingsItemCardState, bool) {
	card := buildSettingsItemCard(cotTranslationSettingsItem(), describe, snapshot.drafts, snapshot.write)
	if !card.Available() {
		return settingsItemCardState{}, false
	}
	card.Cot = &settingsCotState{Loaded: loaded, AvailableProviders: append([]string(nil), limits.AvailableProviders...)}
	return card, true
}

func parseCotProviderDraft(text string) (any, bool, bool) {
	provider := strings.TrimSpace(text)
	if provider == "" {
		return nil, true, true
	}
	for _, candidate := range cotProviderPreferences {
		if provider == candidate {
			return provider, false, true
		}
	}
	return nil, false, false
}

func parseCotTargetLanguageDraft(text string) (any, bool, bool) {
	language := strings.TrimSpace(text)
	if !cotTargetLanguagePattern.MatchString(language) {
		return nil, false, false
	}
	return language, false, true
}

func cotProviderLabel(provider string) string {
	switch provider {
	case "bing":
		return "Bing"
	case "google":
		return "Google"
	case "deepseek-account":
		return "DeepSeek Flash · 账号（付费）"
	case "deepseek-official":
		return "DeepSeek Flash · 官方 API（付费）"
	default:
		return provider
	}
}

func cotProviderAvailable(cot *settingsCotState, provider string) bool {
	if cot == nil || !cot.Loaded {
		return true
	}
	for _, candidate := range cot.AvailableProviders {
		if candidate == provider {
			return true
		}
	}
	return false
}
