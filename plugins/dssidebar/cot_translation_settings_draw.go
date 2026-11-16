package dssidebar

import (
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/ui"
)

const (
	cotPrivacyNotice   = "展开的思考内容会发送给所选翻译服务，可能包含私有代码或对话细节。"
	cotPaidNotice      = "每个未缓存片段都会通过所选账号或官方 API 凭据单独发送一次付费的 DeepSeek Flash 请求；重新展开会复用已保存的译文，更改服务、语言、原文或请求设置可能发送新的付费请求"
	cotAnonymousNotice = "Bing 和 Google 翻译无需额外登录或 API key，但匿名接口可能拒绝或限制请求"
	cotProviderUnavail = "此服务暂不可用，请配置 DeepSeek 账号或官方 API 服务，或选择可用的翻译服务"
)

// drawCotTranslationSettings 落在官方 PackageDetail 的 data-plugin-config 段。
// 只有完整 bundle 名与宿主 namespace 同时匹配才出现，不在其它插件详情里借画。
func (p *Plugin) drawCotTranslationSettings(f ui.C, env *pluginapi.Env, bundle apiclient.PluginBundle) {
	if bundle.Name != cotTranslationBundleName || env == nil || env.Act.SettingsDescribe == nil {
		return
	}
	limits, loaded := apiclient.CotTranslationLimits{}, false
	if env.Act.CotTranslationLimits != nil {
		limits, loaded = env.Act.CotTranslationLimits()
	}
	card, ok := cotTranslationSettingsCard(env.Act.SettingsDescribe(), p.itemFormSnapshot(), limits, loaded)
	if !ok {
		p.cotSettingsMounted = false
		return
	}
	if !p.cotSettingsMounted {
		p.cotSettingsMounted = true
		// 官方 TranslationSettings useEffect(() => refreshProviders(), [refreshProviders])。
		if env.Act.CotTranslationRefresh != nil {
			env.Act.CotTranslationRefresh()
		}
	}
	theme := p.theme
	if env.Theme != nil {
		theme = env.Theme
	}
	p.drawSettingsItemForm(f, env, theme, card)
}

func (p *Plugin) drawCotTranslationNotices(f ui.C, t *pluginapi.Theme, card settingsItemCardState) {
	provider := ""
	for _, section := range card.Sections {
		for _, field := range section.Fields {
			if field.Spec.Name == settingsFieldCotProvider {
				provider = field.Text
			}
		}
	}
	notice := cotAnonymousNotice
	if provider == "deepseek-account" || provider == "deepseek-official" {
		notice = cotPaidNotice
	}
	ui.Column(f).FillWidth().Gap(itemFieldGap).MarginSides(0, 0, itemFieldPadY, 0).Children(func() {
		ui.Wrap(f, cotPrivacyNotice).FillWidth().FontSize(itemFieldMsgSize).
			LineHeight(itemFieldMsgLine).Color(t.LabelTertiary)
		ui.Wrap(f, notice).FillWidth().FontSize(itemFieldMsgSize).
			LineHeight(itemFieldMsgLine).Color(t.LabelTertiary)
	})
}

// drawCotProviderInput 是官方 select：availableProviders 只决定选项，不改写当前草稿；
// 已选但当前不可用的值保留为 disabled option，并显示官方 providerUnavailable。
func (p *Plugin) drawCotProviderInput(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	card settingsItemCardState, section settingsItemSectionState, field settingsItemFieldState) {
	cot := card.Cot
	selected := field.Text
	missing := cot != nil && cot.Loaded && selected != "" && !cotProviderAvailable(cot, selected)
	options := make([]settingsWebProviderOption, 0, 4)
	if missing {
		options = append(options, settingsWebProviderOption{ID: selected, Disabled: true})
	}
	if cot != nil {
		for _, provider := range cot.AvailableProviders {
			options = append(options, settingsWebProviderOption{ID: provider})
		}
	}
	label := cotProviderLabel(selected)
	if missing {
		label += " — 不可用"
	}
	disabled := field.Disabled || card.Saving || cot == nil || !cot.Loaded
	p.drawSettingsWebMenu(f, env, t, "cot-translation-provider", field.Spec.DOMID,
		label, disabled, options, selected, func(option settingsWebProviderOption) string {
			return cotProviderLabel(option.ID)
		}, func(provider string) {
			p.ensureSettingsItemForm().stage(section.NS, field.Spec.Name,
				settingsItemDraft{Text: provider}, section.Revision)
		})
	if missing {
		ui.Wrap(f, cotProviderUnavail).FillWidth().FontSize(itemFieldMsgSize).
			LineHeight(itemFieldMsgLine).Color(t.LabelTertiary)
	}
}
