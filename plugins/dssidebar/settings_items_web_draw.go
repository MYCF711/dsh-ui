// settings_items_web_draw.go —— 官方 #47 `ui-settings-web-search`（「网页搜索与获取」）的**绘制**。
//
// # 几何 / 文案的权威源（每一项都能核对到官方原文）
//
//	页面结构     OFF:src/client/WebSearchCard.tsx:68-129（两个 section + 灰面板）
//	样式         OFF:src/client/WebSearchCard.module.css:1-150（本文件的常量逐行对应）
//	字段样式     primitives `dsh-web-frontend/dist/assets/index-CSAdOBx6.css`
//	             `._field_181ew_3` / `._head_181ew_14` / `._input_181ew_115` 等（与
//	             settings_items_draw.go 共用常量，不重复定义）
//	下拉/面板    WebSearchCard 自己的 `.selector`（34px）与 `.panel`（bg-module-platform）
//	密钥控件     primitives `SettingsSecretField`（编译产物里的 `VZ`：head{label + Tag} +
//	             `input[type=password]` + hint；**没有** overridden/reset）
//	计费控件     WebSearchCard.tsx:208-239（panelField{panelHead{label + Tag} + 下拉 + hint}）
//
// # 已登记的差异（缺真实素材/框架能力，不造假）
//
//	① **菜单选中项的对勾字形不是官方那一枚**（位置与语义已一致，**不是** radio 圆点）：
//	   官方 `Menu` 的 `selection="check"` 在行尾画 sprite `check-outline`
//	   （`index-ghOpC9uV.js:129:100052` 的 `d&&f==="check"&&m.jsx(Cs,...)`；`Cs` 见
//	   `129:76101`；sprite 原文 `viewBox="0 0 16 16"` + `M2.25 8.5L5.49732 11.7473C…L13.75 4`，
//	   CSS `._check_70kcw_265` 尺寸 14×14、色 `label-primary`，见
//	   `index-CSAdOBx6.css:1:16475`、`:1:17024`）。
//	   viva 0.6.4 的 `.Chosen(true)` 走 `DropdownMenuRadio`+`Selected`
//	   （`ui/menu.go:465-471`、`:314-331`），`paintItem` 用 `paintCheck` 在**行尾**
//	   画内置 `checkIcon`（`components/dropdown_menu.go:1066-1071`、
//	   `components/checkbox.go:144-153`，viewBox 24 / stroke 2 / `M20 6.5 L9 17.5 L4 12.5`，
//	   16×16，右边缘同样距行右沿 8px）。所以：**位置一致、形状同为对勾、字形与尺寸不同**。
//	   `ui.MenuItems` 没有行尾插槽（`paintItem` 里的字形是私有 `checkIcon`，无覆盖入口），
//	   官方那枚 path 不能塞进这一行；本仓已备 `dsassets.CheckOutline()`，等上游开口。
//	   不手绘冒充，也不为一行对勾换掉整个菜单（会丢焦点陷阱/方向键/Esc/点外关闭）。
//	② **帮助/链接的可点击**：官方 `searchGuide` 是 `<a href target=_blank>`；本仓用
//	   主题的 link 色文字 + 真命中区，点击经壳的 `OpenExternal`（系统默认浏览器）。
//	   `OpenExternal == nil` 时不挂命中区（画一个点了没反应的链接比不画更糟）。
//	③ **灰面板** `background: --dsw-alias-bg-module-platform`：本仓主题里这一层是
//	   `Theme.BgLayer3`（`pluginapi/theme.go:27` 注明"抬高三层（选择器、模块台）"）。
//	④ 官方 `.chevron` 只给颜色不给尺寸（组件默认 = viewBox 16），本仓按 16 画。
package dssidebar

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 几何常量（WebSearchCard.module.css 逐行）──────────────────────────

const (
	// 两个 section 之间（`.section + .section{margin-top:32px}`）。
	webSectionGap float32 = 32
	// `.sectionTitle{margin:0;font-size:14px;font-weight:500;line-height:20px}`。
	webSectionTitleSize float32 = 14
	webSectionTitleLine float32 = 20

	// `.field{display:flex;flex-direction:column;gap:6px;padding:8px 0 12px}`（**本页自有**的
	// field 样式，与 primitives 的 `._field_181ew_3{padding:12px 0}` 不同：它是下拉那一格）。
	webFieldPadTop    float32 = 8
	webFieldPadBottom float32 = 12
	webFieldGap       float32 = 6

	// `.selector{height:34px;padding:0 12px;border:.5px solid border-l4;border-radius:md;
	//            background:bg-layer-3;font-size:13px;line-height:1.5;gap:12px}`。
	webSelectorHeight float32 = 34
	webSelectorPadX   float32 = 12
	webSelectorGap    float32 = 12
	webSelectorFont   float32 = 13
	webSelectorLine   float32 = 19.5
	webSelectorStroke float32 = 0.5
	// `.chevron{flex:none;color:label-tertiary}`（尺寸取 viewBox 16，见文件头 ④）。
	webChevronSize float32 = 16

	// `.panel{display:flex;flex-direction:column;padding:0 16px;border-radius:md;
	//         background:bg-module-platform}`。
	webPanelPadX float32 = 16
	// `.panelField{gap:6px;padding:12px 0}`（官方自己搭的计费那一格）。
	webPanelFieldPadY float32 = 12
	webPanelFieldGap  float32 = 6
	// `.panelHead{gap:8px}` / `.panelLabel{font-size:13px;font-weight:500;line-height:1.5}`。
	webPanelHeadGap   float32 = 8
	webPanelLabelSize float32 = 13
	webPanelLabelLine float32 = 19.5

	// `.hint{margin:0;font-size:12px;line-height:1.5}`。
	webHintSize float32 = 12
	webHintLine float32 = 18
	// `.notice{gap:12px;font-size:12px;line-height:1.5;color:state-error-primary}`。
	webNoticeGap  float32 = 12
	webNoticeSize float32 = 12
	webNoticeLine float32 = 18
)

// webGeoRow 是一行几何事实（与 settings_items_draw.go 的 itemGeoRow 同构）。
type webGeoRow struct {
	Node     string
	Prop     string
	Official float32
	Impl     float32
	Anchor   string
}

// settingsWebGeometry 是这一页的几何台账：每行 Official 必须等于 Impl。
func settingsWebGeometry() []webGeoRow {
	const css = "ui-settings-web-search/src/client/WebSearchCard.module.css"
	return []webGeoRow{
		{"section+section", "margin-top", webSectionGap, webSectionGap, css + ":16-18"},
		{"sectionTitle", "font-size", webSectionTitleSize, webSectionTitleSize, css + ":20-26"},
		{"sectionTitle", "line-height", webSectionTitleLine, webSectionTitleLine, css + ":20-26"},
		{"field.provider", "padding-block", webFieldPadTop, webFieldPadTop, css + ":28-33"},
		{"field.provider", "gap", webFieldGap, webFieldGap, css + ":28-33"},
		{"selector", "height", webSelectorHeight, webSelectorHeight, css + ":35-52"},
		{"selector", "padding-inline", webSelectorPadX, webSelectorPadX, css + ":35-52"},
		{"selector", "gap", webSelectorGap, webSelectorGap, css + ":35-52"},
		{"selector", "font-size", webSelectorFont, webSelectorFont, css + ":35-52"},
		{"selector", "border-width", webSelectorStroke, webSelectorStroke, css + ":35-52"},
		{"panel", "padding-inline", webPanelPadX, webPanelPadX, css + ":83-89"},
		{"panelField", "padding-block", webPanelFieldPadY, webPanelFieldPadY, css + ":92-97"},
		{"panelField", "gap", webPanelFieldGap, webPanelFieldGap, css + ":92-97"},
		{"panelHead", "gap", webPanelHeadGap, webPanelHeadGap, css + ":105-109"},
		{"panelLabel", "font-size", webPanelLabelSize, webPanelLabelSize, css + ":117-122"},
		{"panelLabel", "line-height", webPanelLabelLine, webPanelLabelLine, css + ":117-122"},
		{"hint", "font-size", webHintSize, webHintSize, css + ":124-129"},
		{"hint", "line-height", webHintLine, webHintLine, css + ":124-129"},
		{"notice", "gap", webNoticeGap, webNoticeGap, css + ":131-139"},
		{"notice", "font-size", webNoticeSize, webNoticeSize, css + ":131-139"},
	}
}

// ── 表单主体 ──────────────────────────────────────────────────────

// drawSettingsWebForm 画 #47 那一页的正文：两个 section（标题 + 下拉 + 说明 + 面板）。
func (p *Plugin) drawSettingsWebForm(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, card settingsItemCardState) {
	if card.Web == nil {
		return
	}
	sections := []settingsWebSectionState{card.Web.Search, card.Web.Fetch}
	for index, section := range sections {
		section := section
		ui.Column(f).FillWidth().Children(func() {
			if index > 0 {
				// `.section + .section{margin-top:32px}`
				ui.Spacer(f).Height(webSectionGap)
			}
			p.drawSettingsWebSection(f, env, t, card, section)
		})
	}
}

// drawSettingsWebSection 画一个能力那一节。
func (p *Plugin) drawSettingsWebSection(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	card settingsItemCardState, section settingsWebSectionState) {
	ui.Column(f).FillWidth().Children(func() {
		// `.sectionTitle`
		ui.Text(f, section.Title).FillWidth().FontSize(webSectionTitleSize).
			LineHeight(webSectionTitleLine).Weight(gfx.WeightMedium).Color(t.LabelPrimary)
		// `.field{padding:8px 0 12px;gap:6px}`：下拉 + 说明行。
		ui.Column(f).FillWidth().Gap(webFieldGap).
			PadSides(webFieldPadTop, 0, webFieldPadBottom, 0).Children(func() {
			p.drawSettingsWebProviderSelect(f, env, t, card, section)
			if section.Description != "" {
				p.drawSettingsWebHint(f, t, settingsWebHint{Text: section.Description})
			}
			for _, hint := range section.Hints {
				p.drawSettingsWebHint(f, t, hint)
			}
			if section.ProvidersFailed {
				p.drawSettingsWebProvidersFailed(f, env, t)
			}
		})
		if section.Panel != nil {
			p.drawSettingsWebPanel(f, env, t, card, section)
		}
	})
}

// drawSettingsWebProviderSelect 画 provider 下拉（官方 `.selector`）。
func (p *Plugin) drawSettingsWebProviderSelect(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	card settingsItemCardState, section settingsWebSectionState) {
	web := card.Web
	disabled := !card.Writable || card.Saving || web.selectionBlocked
	if !web.providerWired {
		// 壳没接 `web.listProviders`：清单列不出来，只画当前值 + 一句**本仓自己的**
		// 说明（不冒充官方文案），不给命中区——点了只会展开一个空菜单。
		ui.Column(f).FillWidth().Gap(webFieldGap).Children(func() {
			_, _ = p.drawSettingsWebSelectorBox(f, env, t, section.ProviderDOMID, section.ValueLabel, true)
			p.drawSettingsWebHint(f, t, settingsWebHint{Text: zhWebProviderNone})
		})
		return
	}
	menuKey := "web-provider-" + section.Capability
	options := section.Options
	if section.Auto {
		// 官方把「自动选择」（id = 空串）拼在菜单最前（WebSearchCard.tsx:280-283）。
		options = append([]settingsWebProviderOption{{ID: ""}}, options...)
	}
	composed, composedSet := web.searchComposed, web.searchComposedSet
	if section.Capability == "fetch" {
		composed, composedSet = web.fetchComposed, web.fetchComposedSet
	}
	revision := web.webRevision
	p.drawSettingsWebMenu(f, env, t, menuKey, section.ProviderDOMID, section.ValueLabel, disabled,
		options, section.Selected, settingsWebProviderOptionLabel, func(id string) {
			p.ensureSettingsItemForm().settingsWebStageSelection(
				settingsWebSelectionField(section.Capability), id, composed, composedSet, revision)
		})
}

// drawSettingsWebMenu 是 provider / 计费**共用**的那个下拉（`.selector` + viva 菜单）。
func (p *Plugin) drawSettingsWebMenu(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, menuKey, domID string,
	valueLabel string, disabled bool, options []settingsWebProviderOption, selected string,
	label func(settingsWebProviderOption) string, onSelect func(string)) {
	if p.itemWebMenus == nil {
		p.itemWebMenus = map[string]bool{}
	}
	open := p.itemWebMenus[menuKey] && !disabled
	box, clicked := p.drawSettingsWebSelectorBox(f, env, t, domID, valueLabel, disabled)
	if !disabled && clicked {
		open = !open
	}
	ui.Menu(f, box, &open, func(items *ui.MenuItems) {
		for _, option := range options {
			option := option
			// viva 的菜单行由 Scope 身份驱动（`MenuItems.Item` 内部分配），不是 `Box.Hit`；
			// 与 plugins/dscontextmenu 的 `declare` 同一写法。
			if items.Item(label(option)).Chosen(option.ID == selected).Clicked() {
				onSelect(option.ID)
			}
		}
	})
	// 菜单自己会把 open 置回 false（选中/点外面）；写回映射表，下一帧才是真的关着。
	p.itemWebMenus[menuKey] = open
}

// drawSettingsWebSelectorBox 画 `.selector`（34px 圆角框 + 右侧 chevron），并报告它被点没被点。
//
// 命中区**在本函数里**挂（调用方只给 id）：所以"点没点"也在这里读，调用方拿 bool，
// 不在自己体内再 `.Clicked()`——`cmd/app` 的静态守卫（V-06-4 判据②）要求
// `.Clicked()` 与 `.Hit()` 在同一个函数里出现，否则那个接收者被判成"永远点不着"。
func (p *Plugin) drawSettingsWebSelectorBox(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	domID, valueLabel string, disabled bool) (ui.Box, bool) {
	color := t.LabelPrimary
	if disabled {
		// `.selector:disabled{color:label-tertiary}`
		color = t.LabelTertiary
	}
	box := ui.Row(f).FillWidth().Height(webSelectorHeight).Radius(t.Radii.MD).
		Border(webSelectorStroke, t.BorderL4).Background(t.BgLayer3).
		Items(ui.Center).Gap(webSelectorGap).
		PadSides(0, webSelectorPadX, 0, webSelectorPadX).
		Hit(env.Tree.ID(env.Feat, domID), valueLabel).Children(func() {
		ui.Text(f, valueLabel).FontSize(webSelectorFont).LineHeight(webSelectorLine).
			Color(color).Grow(1).MinWidth(0).Ellipsis()
		if commands, ok := dsassets.ChevronDownOutline(); ok {
			ui.Icon(f, gfx.Icon{
				ViewBox: webChevronSize, Stroke: 1, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: commands,
			}).Size(webChevronSize, webChevronSize).Color(t.LabelTertiary)
		}
	})
	clicked := box.Clicked()
	return box.Disabled(disabled), clicked
}

// drawSettingsWebHint 画一行 `.hint`（可带官方链接）。
func (p *Plugin) drawSettingsWebHint(f ui.C, t *pluginapi.Theme, hint settingsWebHint) {
	if hint.URL == "" {
		ui.Text(f, hint.Text).FillWidth().FontSize(webHintSize).LineHeight(webHintLine).Color(t.LabelTertiary)
		return
	}
	ui.Text(f, hint.Text).FillWidth().FontSize(webHintSize).LineHeight(webHintLine).Color(t.Link)
}

// drawSettingsWebProvidersFailed 画官方 `ReadFailure`：错误行 + 「重试」（WebSearchCard.tsx:246-253）。
//
// 官方是 `.notice{gap:12px}`：左一句 state-error-primary 的文字，右一个
// `Button variant=outline size=sm`。本仓用 `ui.Button(...).Outline().Small()`（同一支控件）。
func (p *Plugin) drawSettingsWebProvidersFailed(f ui.C, env *pluginapi.Env, t *pluginapi.Theme) {
	ui.Row(f).FillWidth().Items(ui.Center).Gap(webNoticeGap).Children(func() {
		ui.Text(f, zhWebProvidersFailed).Grow(1).MinWidth(0).FontSize(webNoticeSize).
			LineHeight(webNoticeLine).Color(t.Error)
		retry := ui.Button(f, zhWebRetry).Outline().Small().
			Hit(env.Tree.ID(env.Feat, "settings-web-providers-retry"), zhWebRetry)
		if retry.Clicked() && env.Act.WebProviderRefresh != nil {
			env.Act.WebProviderRefresh()
		}
	})
}

// drawSettingsWebPanel 画灰面板（`.panel`）。
func (p *Plugin) drawSettingsWebPanel(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	card settingsItemCardState, section settingsWebSectionState) {
	ui.Column(f).FillWidth().Radius(t.Radii.MD).Background(t.BgLayer3).
		PadSides(0, webPanelPadX, 0, webPanelPadX).Children(func() {
		for index, entry := range section.Panel {
			entry := entry
			first := index == 0
			switch entry.Kind {
			case settingsWebEntryBilling:
				p.drawSettingsWebBilling(f, env, t, card, section, entry.Billing, first)
			case settingsWebEntrySecret:
				p.drawSettingsWebSecret(f, env, t, card, entry.Secret, first)
			default:
				p.drawSettingsWebPanelField(f, env, t, card, entry.Field, first)
			}
		}
	})
}

// drawSettingsWebPanelField 在面板里画一个 `SettingsValueField`。
//
// 复用 #39/#45/#46 那套绘制（label / 「已覆盖」+「恢复默认」/ 输入框 / hint），
// 只把所在命名空间的写状态换成本页对应的那一条。
func (p *Plugin) drawSettingsWebPanelField(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	card settingsItemCardState, field settingsItemFieldState, first bool) {
	ns, writable, revision := settingsWebFieldScope(card, field.Spec.Name)
	section := settingsItemSectionState{NS: ns, Writable: writable, Revision: revision, Available: true}
	p.drawSettingsItemField(f, env, t, card, section, field, first)
}

// settingsWebFieldScope 返回一个字段属于哪条命名空间，以及它那条的写状态。
func settingsWebFieldScope(card settingsItemCardState, name string) (string, bool, int) {
	web := card.Web
	if web == nil {
		return "", false, 0
	}
	switch name {
	case settingsFieldWebCount:
		return settingsNSWebAPI, web.apiWritable, web.apiRevision
	case settingsFieldLegacyBaseURL, settingsFieldLegacyMaxUses:
		return settingsNSWebSearch, web.legacyWritable, web.legacyRevision
	default:
		return settingsNSWeb, web.webWritable, web.webRevision
	}
}

// drawSettingsWebBilling 画计费那一格（官方 `.panelField`）。
func (p *Plugin) drawSettingsWebBilling(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	card settingsItemCardState, section settingsWebSectionState, billing settingsWebBillingState,
	first bool) {
	web := card.Web
	domID := "plugin-config-web-" + section.Capability + "-billing"
	if !first {
		// `.panel > * + .panelField{border-top:.5px solid border-l2}`
		ui.Column(f).FillWidth().Children(func() { ui.Divider(f) })
	}
	ui.Column(f).FillWidth().Gap(webPanelFieldGap).
		PadSides(webPanelFieldPadY, 0, webPanelFieldPadY, 0).Children(func() {
		ui.Row(f).FillWidth().Items(ui.Center).Gap(webPanelHeadGap).Children(func() {
			ui.Column(f).Grow(1).MinWidth(0).Children(func() {
				ui.Text(f, zhWebBilling).FontSize(webPanelLabelSize).
					LineHeight(webPanelLabelLine).Weight(gfx.WeightMedium).Color(t.LabelPrimary)
			})
			if len(billing.AdvancedOverride) > 0 {
				// 官方 `<Tag tone=neutral>`：文案「已被 {fields} 覆盖」，字段名以 zh 的
				// 「、」连接（WebSearchCard.tsx:198-199 的 FIELD_SEPARATOR）。
				label := strings.ReplaceAll(zhWebBillingOverride, "{fields}",
					strings.Join(billing.AdvancedOverride, zhWebBillingSeparator))
				p.drawSettingsItemTag(f, t, label, true)
			}
		})
		locked := len(billing.AdvancedOverride) > 0
		disabled := !card.Writable || card.Saving || locked
		options := make([]settingsWebProviderOption, 0, len(settingsWebBillings))
		for _, mode := range settingsWebBillings {
			options = append(options, settingsWebProviderOption{ID: mode, Title: settingsWebBillingName(mode)})
		}
		label := func(option settingsWebProviderOption) string { return option.Title }
		composed := web.searchBillingComposed
		if section.Capability == "fetch" {
			composed = web.fetchBillingComposed
		}
		revision := web.apiRevision
		p.drawSettingsWebMenu(f, env, t, "web-billing-"+section.Capability, domID,
			settingsWebBillingName(billing.Value), disabled, options, billing.Value, label,
			func(mode string) {
				p.ensureSettingsItemForm().settingsWebStageBilling(section.Capability, mode, composed, revision)
			})
		ui.Text(f, settingsWebBillingHint(billing.Value)).FillWidth().FontSize(webHintSize).
			LineHeight(webHintLine).Color(t.LabelTertiary)
	})
}

// drawSettingsWebSecret 画密钥控件（primitives `SettingsSecretField`）。
//
// DOM：`.field` → `.head{label, .badges → Tag}` → `input[type=password]` → `.hint`。
// **没有** overridden 角标与「恢复默认」（官方这一支根本没有它们）。
func (p *Plugin) drawSettingsWebSecret(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	card settingsItemCardState, key settingsWebKeyState, first bool) {
	if !first {
		ui.Column(f).FillWidth().Children(func() { ui.Divider(f) })
	}
	ui.Column(f).FillWidth().Gap(webFieldGap).
		PadSides(itemFieldPadY, 0, itemFieldPadY, 0).Children(func() {
		ui.Row(f).FillWidth().Items(ui.Center).Gap(webPanelHeadGap).Children(func() {
			ui.Column(f).Grow(1).MinWidth(0).Children(func() {
				ui.Text(f, zhWebCredential).FontSize(itemFieldLabelSize).
					LineHeight(itemFieldLabelLine).Weight(gfx.WeightMedium).Color(t.LabelPrimary)
			})
			// 官方 `<Tag tone={configured ? 'neutral' : 'quiet'}>`：本仓「已配置」用与
			// 「已覆盖」同一枚 neutral 胶囊，「未配置 / 未知」用次要色（差异已登记）。
			p.drawSettingsItemTag(f, t, key.StateLabel, key.Configured)
		})
		ui.Row(f).FillWidth().Height(itemFieldInputH).Radius(t.Radii.MD).
			Border(itemFieldStrokeW, t.BorderL4).Background(t.BgLayer3).
			Items(ui.Center).PadSides(0, itemFieldInputPadX, 0, itemFieldInputPadX).Children(func() {
			// `input[type=password]` 的等价物：`Field.Secure`（见 settingsItemFields.all 的注释）。
			ui.TextFieldID(f, env.Tree.ID(env.Feat, settingsWebKeyDOMID), &p.itemFields.webKey,
				zhWebCredential, "").Grow(1).Height(itemFieldInputH).
				FontSize(itemFieldInputFont).Disabled(key.Disabled)
			if key.Disabled {
				return
			}
			if p.itemFields.syncField(settingsNSWeb+"."+settingsFieldLegacySecret, &p.itemFields.webKey, key.Text) {
				// 用户敲了：stage 一份**记引用**的草稿（官方 `editKey` → `draftRef`）。
				p.ensureSettingsItemForm().settingsWebStageKey(
					key.Ref, p.itemFields.webKey.Text(), card.Web.webRevision)
			}
		})
		ui.Text(f, settingsWebKeyHint(key.Ref)).FillWidth().FontSize(webHintSize).
			LineHeight(webHintLine).Color(t.LabelTertiary)
		if key.Note != "" {
			// 本仓自己那句（宿主还没答"设没设"时），不冒充官方文案。
			ui.Text(f, key.Note).FillWidth().FontSize(webHintSize).
				LineHeight(webHintLine).Color(t.LabelTertiary)
		}
	})
}

// ── 计费文案（官方 locales.ts:104-111）──────────────────────────────

// settingsWebBillingName 是一处计费在菜单/按钮上的名字。
func settingsWebBillingName(billing string) string {
	switch billing {
	case "account":
		return zhWebBillingAccount
	case "api-key":
		return zhWebBillingAPIKey
	default:
		return zhWebBillingAuto
	}
}

// settingsWebBillingHint 是选中计费下面那行说明。
func settingsWebBillingHint(billing string) string {
	switch billing {
	case "account":
		return zhWebBillingAcctHint
	case "api-key":
		return zhWebBillingKeyHint
	default:
		return zhWebBillingAutoHint
	}
}

// settingsWebProviderOptionLabel 是 provider 菜单项的文字。
func settingsWebProviderOptionLabel(option settingsWebProviderOption) string {
	if option.ID == "" {
		// 官方菜单里的「自动选择」项 id 是空串（WebSearchCard.tsx:134 的 AUTO_ID）。
		return zhWebProviderAuto
	}
	return settingsWebProviderLabel(option)
}
