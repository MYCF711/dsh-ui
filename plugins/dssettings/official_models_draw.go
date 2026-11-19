// official_models_draw.go —— Models 独立页的卡片绘制与草稿交互。
package dssettings

import (
	"context"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

func (p *Plugin) drawModelsPage(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, a AreaRect, section PanelSection, write WriteState) {
	page := section.Models
	if page == nil {
		return
	}
	// 绘制帧开头清掉"上一笔 Apply 成功"留下的密钥草稿：SetText 只能在窗口线程做，
	// 这一帧就在这里。
	p.flushModelsKeyField()
	contentWidth := a.Width - OptionsPadLeft - OptionsPadRight
	if contentWidth > ModelsSectionMaxWidth {
		contentWidth = ModelsSectionMaxWidth
	}
	ui.Column(f).Absolute().Left(a.X).Top(a.Y).Width(a.Width).Height(a.Height).
		PadSides(OptionsPadTop, OptionsPadRight, OptionsPadBottom, OptionsPadLeft).
		Gap(ModelsSectionGap).Children(func() {
		ui.Text(f, section.Title).FontSize(ModelsTitleFontSize).LineHeight(ModelsTitleLine).
			Weight(NavTitleWeight).Color(t.LabelPrimary).Width(contentWidth).Ellipsis()
		ui.Text(f, modelsIntro).FontSize(ModelsIntroFontSize).LineHeight(ModelsIntroLine).
			Color(t.LabelTertiary).Width(contentWidth)
		if section.Note != "" {
			ui.Text(f, section.Note).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).
				Color(t.Warn).FillWidth()
		}
		for _, message := range page.Errors {
			ui.Text(f, message).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).
				Color(t.Error).FillWidth()
		}
		if write.Render() != "" {
			color := t.LabelSecondary
			if write.Failure() {
				color = t.Error
			} else if write.Success() {
				color = t.Success
			}
			ui.Text(f, write.Render()).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).
				Color(color).FillWidth()
		}

		scroll := ui.Scroll(f).Width(contentWidth).Grow(1).MinHeight(0).MarginTop(ModelsRowsMarginTop)
		scroll.Children(func() {
			ui.Column(f).Width(contentWidth).Gap(ModelsRowsGap).Children(func() {
				for _, group := range section.Groups {
					p.drawModelsGroup(f, env, t, group, write)
				}
			})
		})
	})
}

func (p *Plugin) drawModelsGroup(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, group PanelGroup, write WriteState) {
	provider := len(group.Rows) > 0 && strings.HasPrefix(group.Rows[0].ID, "provider-edit-")
	if group.Title != "" && !provider {
		ui.Text(f, group.Title).FontSize(ModelsRowNameSize).LineHeight(ModelsRowNameLine).
			Weight(NavTitleWeight).Color(t.LabelPrimary).Ellipsis()
	}
	ui.Column(f).FillWidth().PadSides(ModelsRowCardPadY, ModelsRowCardPadX, ModelsRowCardPadY, ModelsRowCardPadX).
		Gap(ModelsRowCardGap).Radius(t.Radii.XL).Border(ModelsCardBorder, t.BorderL4).
		Background(t.BgLayer2).Children(func() {
		if provider {
			headRow := group.Rows[0]
			ui.Row(f).FillWidth().Gap(ModelsRowHeadGap).Items(ui.Center).Children(func() {
				ui.Text(f, group.Title).FontSize(ModelsRowNameSize).LineHeight(ModelsRowNameLine).
					Weight(NavTitleWeight).Color(t.LabelPrimary).Ellipsis()
				// 官方 `.credentialDot`：8px 实心圆，紧跟名字（ModelsSection.tsx:481-500
				// 把它放在 `.rowIdentity` 里、`rowActions` 之前）。
				if headRow.Badge != PanelBadgeNone {
					color := t.Success
					if headRow.Badge == PanelBadgeCredentialMissing {
						color = t.Error
					}
					ui.Row(f).Shrink(0).Width(ModelsCredentialDot).Height(ModelsCredentialDot).
						Radius(ModelsCredentialDot / 2).Background(color)
				}
				if headRow.Desc != "" {
					ui.Text(f, headRow.Desc).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).
						Color(t.LabelTertiary).Grow(1).Ellipsis()
				} else {
					ui.Row(f).Grow(1)
				}
				p.drawModelsActionRow(f, env, t, headRow)
			})
			if len(group.Rows) > 1 {
				ui.Column(f).FillWidth().PadSides(ModelsEditorPadY, ModelsEditorPadX, ModelsEditorPadY, ModelsEditorPadX).
					Gap(ModelsEditorGap).Radius(t.Radii.LG).Background(settingsModuleFill(t)).Children(func() {
					for i := 1; i < len(group.Rows); i++ {
						row := group.Rows[i]
						if strings.HasSuffix(row.ID, "-cancel") && i+1 < len(group.Rows) {
							ui.Row(f).FillWidth().Gap(ModelsRowActionsGap).Justify(ui.End).Children(func() {
								p.drawModelRow(f, env, t, row, write)
								p.drawModelRow(f, env, t, group.Rows[i+1], write)
							})
							i++
							continue
						}
						p.drawModelRow(f, env, t, row, write)
					}
				})
			}
			return
		}
		for _, row := range group.Rows {
			p.drawModelRow(f, env, t, row, write)
		}
	})
}

func (p *Plugin) drawModelRow(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow, write WriteState) {
	// 提交中：官方 EditorFooter 把 Cancel 禁用、把 commit 的文案换成 `applying`
	// （EditorFooter.tsx:52,63；ProviderEditor.tsx:681-683 传的是 'applying'）。
	// 卡片里其它可写行同时停手，理由写在行上。
	footer := strings.HasSuffix(row.ID, "-cancel") || strings.HasSuffix(row.ID, "-apply")
	if write.Kind == WriteSaving {
		switch {
		case footer:
			row.Disabled = true
			if strings.HasSuffix(row.ID, "-apply") {
				row.Title = zhEditorApplying
			}
		case strings.HasPrefix(row.ID, "retry-") || strings.HasPrefix(row.ID, modelsKeyRowPrefix):
			row.Disabled = true
			row.Reason = zhEditorApplying
		}
	}
	switch {
	case row.Kind == ItemSecret:
		p.drawModelSecretField(f, env, t, row)
	case strings.HasPrefix(row.ID, "provider-edit-"),
		strings.HasSuffix(row.ID, "-cancel"), strings.HasSuffix(row.ID, "-apply"):
		p.drawModelsActionRow(f, env, t, row)
	case strings.HasPrefix(row.ID, "catalog-model-"):
		p.drawCatalogModelRow(f, t, row)
	case row.NS == apiclient.SettingsNSLLM && (row.Kind == ItemStepper || row.Kind == ItemSelector):
		p.drawModelEditorField(f, env, t, row)
	default:
		p.drawOfficialRow(f, env, t, row)
	}
}

// drawModelSecretField 画官方 ProviderEditor 的 API 密钥字段。
//
// 几何逐条取官方 CSS：
//
//	.field      列、gap 6            ModelsSection.module.css:268-272
//	.fieldLabel 12/18、500、label-secondary（可带 16px 帮助钮，本仓没有帮助文案来源，不画）
//	                                ModelsSection.module.css:274-281
//	.input      高 32、左右内缩 10、0.5px border-l4、radius-md、14/22、bg-layer-1
//	                                ModelsSection.module.css:808-820
//	.keyToggle  右侧 32px 按钮、绝对定位在输入框内 1px（`:865-875`）
//	说明行语气：.notice/.savedNotice/.advancedHint（`:29-39`/`:335-339`）
//
// 输入本身是 `officialPanel.modelsKeyField`（Viva 的 ui.Field 必须常驻窗口线程，
// 不能每帧新建），这里只把 label / placeholder / 显隐状态挂上去。
//
// # 已存密钥那一态（官方 `saved`，ApiKeyInput.tsx:15-19,34-49）
//
// 官方只在"有已存密钥 **且** 没有草稿"时传 `saved`（ProviderEditor.tsx:492-497），
// 那一刻：
//
//	:34      根节点带 `.savedKeyInput`，所以占位色是 label-primary 而不是 weak；
//	:41      placeholder = saved.masked（服务端掩码，明文永远不上线）；
//	:42-43   onCopy / onCut 被 preventDefault；
//	:44-49   Backspace / Delete 被 preventDefault，然后 `saved.clear()`。
//
// 本仓对应：`.savedKeyInput` 的占位色由我们自己画（见下面的 `ui.Text`，理由在
// 那一段注释里）；Backspace/Delete 与 onCopy/onCut 在 Viva 里收不到（平台文本
// 系统先消费，见下面的注释），所以"删已存密钥"接的是官方**另一半**判据
// `onChange === ”`（ProviderEditor.tsx:507-509）。
func (p *Plugin) drawModelSecretField(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow) {
	o := p.ensureOfficial()
	// 草稿必须在本函数（窗口线程）读，见 official_panel.go 的 modelsKeyField 注释。
	draft := o.modelsKeyField.Text()
	// 官方 `saved` 属性只在"有已存密钥 **且** 没有草稿"时传（ProviderEditor.tsx:492-497）：
	// 一旦有草稿或删除待办，`saved` 整块不传，掩码占位与那两条拦截同时失效。
	saved := row.Saved && draft == ""
	// 「有草稿 → 清空」这一帧 = 官方 `onChange` 的 `''` 分支
	// （ProviderEditor.tsx:507-509）：`event.target.value === '' && savedCredential`
	// → `setKeyDraft({ ref })`，也就是"请求清除已存密钥"。
	//
	// 为什么走这条路而不是按键：Viva 里聚焦输入框后 Backspace/Delete 由**平台
	// 文本系统**消费（app/driver.go:679-685 → textKey → `TextDeleteBackward`），
	// 应用层的 `Box.OnKey` 根本收不到（ui/field.go:236-262 的处理器只在非编辑
	// 路由下才被调用）。官方那两条 `onKeyDown`（ApiKeyInput.tsx:44-49）因此在
	// 本仓无法照抄；官方自己的另一个入口——把已存密钥那一格的草稿删空——可以，
	// 而且判据完全一样（`saved.clear()`）。
	o.mu.Lock()
	previousDraft := o.modelsKeyLastDraft
	o.modelsKeyLastDraft = draft
	o.mu.Unlock()
	if draft == "" && previousDraft != "" && row.Saved && !row.Disabled {
		// 只读（或已删）时官方连 `saved.clear` 都不给（ProviderEditor.tsx:495-496
		// 的展开是按 `writable`），所以这里同样要求可写。
		p.requestModelsKeyRemoval(env, row)
	}
	ui.Column(f).FillWidth().Gap(ModelsFieldGap).Children(func() {
		// `.fieldLabel`（12/18/500 + gap 10），后面跟着官方那枚 `?` 帮助钮
		// （`.helpButton`：16×16、0.5px border-l3、圆、11/15、label-tertiary）。
		ui.Row(f).Gap(ModelsRowHeadGap).Items(ui.Center).Children(func() {
			ui.Text(f, row.Title).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).
				Weight(500).Color(t.LabelSecondary)
			if row.HelpText != "" {
				help := ui.Row(f).Shrink(0).Width(ModelsKeyHelpButton).Height(ModelsKeyHelpButton).
					Radius(ModelsKeyHelpButton/2).Border(ModelsCardBorder, t.BorderL3).
					Items(ui.Center).Justify(ui.Center).
					// Tooltip 要求元素**先有身份**（viva `Box.Tooltip`：control 或给了 Hit 的
					// 容器），所以 Hit 必须在 Tooltip 之前调用。
					Hit(env.Tree.ID(env.Feat, "settings-models-key-help"), "API 密钥帮助").
					Tooltip(row.HelpText)
				help.Children(func() {
					ui.Text(f, "?").FontSize(ModelsRowTagSize).LineHeight(ModelsKeyHelpLine).Color(t.LabelTertiary)
				})
			}
		})
		placeholder := row.Placeholder
		if placeholder == "" {
			placeholder = zhModelsKeyPlaceholder
		}
		// 掩码占位由我们自己画（官方 `.savedKeyInput .input::placeholder`
		// = `var(--dsw-alias-label-primary)`，ModelsSection.module.css:847-850）。
		// 为什么不能交给 Field：Viva 的占位色是写死的 `theme.MutedForeground`
		// （components/input.go:224-231），`InputSnapshot` 没有给调用方的旋钮
		// （:16-67），而 ui 层的 TextField 也没有占位色链（ui/field.go:96-129）。
		// 所以这里把 Field 的 placeholder 留空、用一枚**没有 Hit 的** Text 画在
		// 同一位置：不给 `Hit()` 的 Text 不注册命中区（ui/paint.go:102-115），
		// 点击仍然落到下面的输入框上，不会把光标挡掉。
		maskedPlaceholder := ""
		if saved && row.SavedMask != "" {
			// 官方 `placeholder={saved.masked}`：宿主没给预览时是空串，不画。
			maskedPlaceholder = row.SavedMask
			placeholder = ""
		}
		// 显隐钮是 `Absolute().Right/Top`（官方 `.keyToggle{position:absolute}`），
		// 它按**容器的 padding box** 定位，所以必须挂在一个「只包住输入框」的容器里，
		// 否则会跑到整个 field 块（label + 输入 + 说明）的右上角。
		ui.Row(f).FillWidth().Height(ModelsInputMinHeight).Children(func() {
			fieldBox := ui.TextFieldID(f, env.Tree.ID(env.Feat, "settings-models-key"), &o.modelsKeyField,
				row.Title, placeholder).
				FillWidth().Height(ModelsInputMinHeight).FontSize(ModelsRowNameSize).
				LineHeight(ModelsRowNameLine).PadX(ModelsKeyPadX).
				Border(ModelsCardBorder, t.BorderL4).Radius(t.Radii.MD).Background(t.BgLayer1)
			if row.Disabled {
				fieldBox.Opacity(0.6)
			}
			if maskedPlaceholder != "" {
				mask := ui.Text(f, maskedPlaceholder).Absolute().Inset(ModelsCardBorder).
					PadX(ModelsKeyPadX).FontSize(ModelsRowNameSize).LineHeight(ModelsRowNameLine).
					Color(t.LabelPrimary)
				if row.Disabled {
					// 输入框自己被压到 0.6（见上面 fieldBox.Opacity），掩码同一档。
					mask.Opacity(0.6)
				}
			}
			// 显隐钮：官方 `keyToggle` 只在**草稿非空**时可点（ApiKeyInput.tsx:31 的
			// `canReveal = saved === undefined && value.length > 0 && !disabled`）。
			// 这里的草稿就是 ui.Field 自己的文本；读它必须发生在窗口线程（本函数就是）。
			if draft != "" && !row.Disabled {
				label := zhModelsKeyHide
				if o.modelsKeyField.Secure {
					label = zhModelsKeyReveal
				}
				toggle := ui.Row(f).Absolute().Right(ModelsKeyToggleInset).Top(ModelsKeyToggleInset).
					Width(ModelsKeyToggleWidth).Height(ModelsInputMinHeight-2*ModelsKeyToggleInset).
					Radius(t.Radii.MD).Items(ui.Center).Justify(ui.Center).
					Hit(env.Tree.ID(env.Feat, "settings-models-key-toggle"), label).
					Hover(ui.Look{}.Background(t.Hover)).Children(func() {
					ui.Text(f, label).FontSize(ModelsRowTagSize).LineHeight(ModelsRowTagLine).Color(t.LabelSecondary)
				})
				if toggle.Clicked() {
					o.modelsKeyField.Secure = !o.modelsKeyField.Secure
				}
			}
		})
		// 说明行：官方三档（ProviderEditor.tsx:446-459）里有一档要看**草稿**，
		// 而草稿只在窗口线程读得到，所以投影层给不出它——在这里补：
		// `keyText.length > 0 && savedCredential` → 「保存以替换已保存的 API key。」，
		// 语气取 `.advancedHint`（tertiary；官方那一行的 className 三元里
		// `keyText.length > 0` 落在 advancedHint 分支，:513）。
		desc, tone := row.Desc, row.Tone
		if draft != "" && row.Saved && !row.Disabled {
			desc, tone = zhModelsKeyReplacePending, panelToneOf(modelsKeyHintMuted)
		}
		if desc != "" {
			color := t.LabelTertiary
			switch tone {
			case PanelToneWarn:
				color = t.Warn
			case PanelToneSuccess:
				color = t.Success
			}
			ui.Text(f, desc).FontSize(ModelsRowTagSize).LineHeight(ModelsRowTagLine).Color(color)
		}
		if row.Reason != "" {
			// Reason 这里只有「只读」与「保存中…」两种（校验失败走页级 WriteState 那行），
			// 所以用 warn 色；红色留给页面顶部那条真正的写失败说明。
			ui.Text(f, row.Reason).FontSize(ModelsRowTagSize).LineHeight(ModelsRowTagLine).Color(t.Warn)
		}
	})
}

// drawModelEditorField 按 ModelsSection.module.css:268-278,808-827 画
// 12/18/500 fieldLabel + 32px 高输入控件，而不是 general 的 76px 左右行。
func (p *Plugin) drawModelEditorField(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow) {
	ui.Column(f).FillWidth().Gap(ModelsFieldGap).Children(func() {
		ui.Text(f, row.Title).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).
			Weight(500).Color(t.LabelSecondary)
		control := ui.Row(f).FillWidth().Height(ModelsInputMinHeight).Radius(t.Radii.MD).
			PadX(10).Border(ModelsCardBorder, t.BorderL4).Background(t.BgLayer1).
			Items(ui.Center).Gap(8).Hit(env.Tree.ID(env.Feat, "settings-row-"+row.ID), row.Title).
			Children(func() {
				ui.Text(f, row.Display()).FontSize(RowTitleFontSize).LineHeight(RowTitleLineHeight).
					Color(t.LabelPrimary).Grow(1).Ellipsis()
				if row.Kind == ItemSelector {
					ui.Icon(f, chevronOutlineIcon()).Size(12, 12).Color(t.LabelTertiary)
				}
			})
		if row.Disabled {
			control.Opacity(0.5)
		}
		if row.Writable() && control.Clicked() {
			p.activateRow(row)
		}
		if row.Reason != "" {
			ui.Text(f, row.Reason).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).Color(t.Warn)
		}
	})
}

func (p *Plugin) drawModelsActionRow(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow) {
	color := t.LabelPrimary
	if strings.HasSuffix(row.ID, "-apply") {
		color = t.ButtonPrimaryFg
	}
	height := ModelsRowButtonH
	padX := ModelsRowButtonPadX
	fontSize := ModelsRowButtonFontSize
	lineHeight := ModelsRowButtonLine
	radius := t.Radii.SM
	footer := strings.HasSuffix(row.ID, "-cancel") || strings.HasSuffix(row.ID, "-apply")
	if footer {
		height = ModelsButtonHeight
		padX = ModelsButtonPadX
		fontSize = ModelsRowNameSize
		lineHeight = ModelsRowNameLine
		radius = t.Radii.MD
	}
	body := ui.Row(f).Shrink(0).Height(height).Gap(ModelsRowHeadGap).Items(ui.Center).
		PadSides(0, padX, 0, padX).Radius(radius).
		Hit(env.Tree.ID(env.Feat, "settings-row-"+row.ID), row.Title)
	if strings.HasPrefix(row.ID, "provider-edit-") || footer && !strings.HasSuffix(row.ID, "-apply") {
		body = body.Border(ModelsCardBorder, t.BorderL3)
	}
	if strings.HasSuffix(row.ID, "-apply") {
		body = body.Background(t.ButtonPrimaryFill)
	}
	if row.Writable() {
		body = body.Hover(ui.Look{}.Background(t.Hover))
	}
	body.Children(func() {
		ui.Text(f, row.Title).FontSize(fontSize).LineHeight(lineHeight).Color(color)
		if row.Desc != "" && !strings.HasPrefix(row.ID, "provider-edit-") {
			ui.Text(f, row.Desc).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).
				Color(t.LabelTertiary).Grow(1).Ellipsis()
		}
		if row.Reason != "" && !strings.HasPrefix(row.ID, "provider-edit-") {
			ui.Text(f, row.Reason).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).
				Color(t.Warn).Grow(1).Ellipsis()
		}
	})
	if row.Writable() && body.Clicked() {
		p.activateRow(row)
	}
}

func (p *Plugin) drawCatalogModelRow(f ui.C, t *pluginapi.Theme, row PanelRow) {
	ui.Row(f).FillWidth().Height(ModelsRowButtonH).Gap(ModelsRowHeadGap).Items(ui.Center).Children(func() {
		ui.Text(f, row.Title).FontSize(ModelsRowNameSize).LineHeight(ModelsRowNameLine).
			Color(t.LabelPrimary).Ellipsis()
		ui.Text(f, row.Desc).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).
			Color(t.LabelTertiary).Grow(1).Ellipsis()
	})
}

// activateModelsRow 分流 Models 的所有行。返回 false 表示不是 Models 行，交给
// general 的既有写路径；retry 字段在这里只改草稿，绝不会落入 applyRowWrite。
func (p *Plugin) activateModelsRow(row PanelRow) bool {
	if strings.HasPrefix(row.ID, modelsKeyRowPrefix) {
		// 密钥行不是按钮：输入由登记的 ui.Field 接键盘（Field 自己拿焦点），
		// 点它不写任何东西。这里认领掉，免得落到 applyRowWrite 去。
		return true
	}
	if strings.HasPrefix(row.ID, "provider-edit-") {
		route, err := modelRouteFromRow(row)
		if err != nil {
			p.setWriteState(ClassifyWriteError(row.NS, row.Path, err))
			return true
		}
		p.toggleModelsEditor(route, row.NS, row.Path)
		return true
	}
	if !strings.HasPrefix(row.ID, "retry-") {
		return false
	}
	route, err := modelRouteFromRow(row)
	if err != nil {
		return false
	}
	o := p.ensureOfficial()
	o.mu.Lock()
	busy := o.write.Kind == WriteSaving
	o.mu.Unlock()
	if busy {
		return true
	}
	switch row.ID {
	case "retry-" + route + "-cancel":
		o.mu.Lock()
		o.modelsEdit = nil
		o.mu.Unlock()
		// 关卡的语义就是丢弃草稿（官方 `onClose(false)`）：输入框里的密钥也跟着丢，
		// 免得下一次开卡把上一个 provider 的密钥写过去。
		p.discardModelsKeyDraft()
		p.setWriteState(WriteState{Kind: WriteIdle})
		return true
	case "retry-" + route + "-apply":
		// 草稿必须在**窗口线程**读（Viva ui.Field 的约束），activateRow 就是窗口线程；
		// 校验也在这里做完，非法值一次请求都不发（官方 submitDisabled 的等价门）。
		keyDraft := o.modelsKeyField.Text()
		if failure := modelsAPIKeyFailure(keyDraft); failure != "" {
			p.setWriteState(WriteState{Kind: WriteInvalid, NS: apiclient.SettingsNSLLM, Path: row.Path,
				Code: apiclient.SettingsCodeBadRequest, Message: failure})
			return true
		}
		p.setWriteState(WriteState{Kind: WriteSaving, NS: apiclient.SettingsNSLLM, Path: row.Path})
		go func() { _ = p.applyModelsEditor(keyDraft) }()
		return true
	default:
		o.mu.Lock()
		if o.modelsEdit == nil || o.modelsEdit.route != route {
			o.mu.Unlock()
			return true
		}
		err = o.modelsEdit.change(row)
		o.mu.Unlock()
		if err != nil {
			p.setWriteState(WriteState{Kind: WriteInvalid, NS: apiclient.SettingsNSLLM, Path: row.Path,
				Code: apiclient.SettingsCodeBadRequest, Message: err.Error()})
		} else {
			p.setWriteState(WriteState{Kind: WriteIdle})
		}
		return true
	}
}

// toggleModelsEditor 开/关一张 provider 卡。ns/path 是这张卡对应的 profile 地址：
// 凭据引用 `keyRef` 就是从它推出来的（官方 `refFor`，ProviderEditor.tsx:167-181），
// 开卡时钉一次，Apply 时不再重算。
func (p *Plugin) toggleModelsEditor(route, ns string, path []string) {
	o := p.ensureOfficial()
	o.mu.Lock()
	if o.modelsEdit != nil && o.modelsEdit.route == route {
		o.modelsEdit = nil
		o.mu.Unlock()
		p.discardModelsKeyDraft()
		p.setWriteState(WriteState{Kind: WriteIdle})
		return
	}
	describe := o.describe
	o.mu.Unlock()
	edit, err := newRetryEditorState(describe, route)
	o.mu.Lock()
	if err == nil {
		o.nextModelsGeneration++
		edit.generation = o.nextModelsGeneration
		edit.credentialRef = credentialRefFor(describe, route, ns, path)
		o.modelsEdit = edit
	}
	o.mu.Unlock()
	if err != nil {
		p.setWriteState(ClassifyWriteError(apiclient.SettingsNSLLM, []string{"providers", route, "retryPolicy"}, err))
		return
	}
	// 换卡 = 换草稿：上一个 provider 的密钥绝不能留在这个框里。
	p.discardModelsKeyDraft()
	p.setWriteState(WriteState{Kind: WriteIdle})
}

// discardModelsKeyDraft 请求清空密钥输入框。
//
// 为什么只是"请求"：清空必须落在窗口线程上（Viva ui.Field 只能在窗口线程读写），
// 而本函数还会从 goroutine 的收尾路径调进来。真正的 SetText 在下一个绘制帧的
// flushModelsKeyField 里做（那一帧一定在窗口线程上）。
func (p *Plugin) discardModelsKeyDraft() {
	o := p.ensureOfficial()
	o.mu.Lock()
	o.modelsKeyClearPending = true
	// 换卡/关卡/写完都把"上一帧草稿"归零：否则清空动作会被下一次开卡的那一帧
	// 误判成"用户刚刚删空了草稿"（见 drawModelSecretField 的注释）。
	o.modelsKeyLastDraft = ""
	o.mu.Unlock()
}

// requestModelsKeyRemoval 记下"清除已存密钥"的待办。
//
// 官方入口是 ApiKeyInput.tsx:44-49：已存密钥那一态里按 Backspace/Delete，
// `preventDefault()` 之后调 `saved.clear()`，而 clear 是
// `setKeyDraft({ ref: detected[0] })`（ProviderEditor.tsx:496）——**只翻状态**，
// 不立刻发请求。真正删除在 Apply 里（:350-356），因为删掉的密钥撤不回来。
//
// row.ID 是 `key-<route>`（modelsKeyRowPrefix），route 用来确认还是那张开着的卡：
// 用户可能在按键与重绘之间切了卡，这时不能把删除记到新卡上。
func (p *Plugin) requestModelsKeyRemoval(env *pluginapi.Env, row PanelRow) {
	provider := strings.TrimPrefix(row.ID, modelsKeyRowPrefix)
	o := p.ensureOfficial()
	o.mu.Lock()
	edit := o.modelsEdit
	changed := false
	if edit != nil && edit.route == provider && !edit.keyRemovalApplied && edit.credentialRef != "" {
		edit.requestKeyRemoval()
		changed = true
	}
	o.mu.Unlock()
	if !changed {
		return
	}
	if env != nil && env.Act.Invalidate != nil {
		env.Act.Invalidate()
	}
}

// flushModelsKeyField 在绘制帧开头把待清空的草稿清掉（窗口线程）。
func (p *Plugin) flushModelsKeyField() {
	o := p.ensureOfficial()
	o.mu.Lock()
	pending := o.modelsKeyClearPending
	o.modelsKeyClearPending = false
	o.mu.Unlock()
	if pending {
		o.modelsKeyField.SetText("")
	}
}

// applyModelsRetry 保留给"只写设置"的既有调用方（测试与无密钥路径）：等价于
// 带一个空草稿走同一条 Apply。空草稿在官方语义里是"这次没给密钥"（apiKey.ts:52）。
func (p *Plugin) applyModelsRetry() WriteState { return p.applyModelsEditor("") }

// applyModelsEditor 是这张 provider 卡唯一的 Apply 出口。
//
// 官方顺序（ProviderEditor.tsx:326-382）照抄，三段不是同一个次序：
//
//	:350-356  先 removeCredential（删已存密钥）
//	:358-365  再 settings.mutate（写设置文档）
//	:366-369  最后 storeCredential（写新密钥）
//
// 为什么删排在最前：删掉的密钥**撤不回来**。设置写失败还能重试，密钥不会自己回来，
// 所以官方先删、删成功就地记成 `removed:true`（:354），之后设置写失败时卡片不关、
// Cancel 变 close、说明改成「密钥已清除，请重试保存以完成配置更新；此删除无法撤销」。
// 写则相反：设置写失败时新密钥**根本不该落库**，所以它排在最后。
func (p *Plugin) applyModelsEditor(keyText string) WriteState {
	o := p.ensureOfficial()
	o.mu.Lock()
	edit := o.modelsEdit.clone()
	writer := o.writer
	setKey := o.credentialSet
	unsetKey := o.credentialUnset
	o.mu.Unlock()
	if edit == nil {
		state := WriteState{Kind: WriteInvalid, Code: apiclient.SettingsCodeBadRequest, Message: "重试策略编辑卡已关闭"}
		p.setWriteState(state)
		return state
	}
	keyValue := modelsKeyDraftValue(keyText)
	// 官方在 card 与 apply 两处都判（ProviderEditor.tsx:282 的 `shownKeyFailure` 进
	// `submitDisabled`，`:326-382` 的 applyOnce 又不会自己重算）——这里同样两处都判：
	// 按钮那侧给即时反馈，这里是权威门（任何绕过按钮的调用方也过不去）。
	if failure := modelsAPIKeyFailure(keyText); failure != "" {
		state := WriteState{Kind: WriteInvalid, NS: credentialNS, Path: []string{edit.credentialRef},
			Code: apiclient.SettingsCodeBadRequest, Message: failure}
		p.setWriteState(state)
		return state
	}
	if keyValue != "" && setKey == nil {
		state := WriteState{Kind: WriteFailed, NS: credentialNS, Path: []string{edit.credentialRef},
			Message: "壳未接 credentials.set：密钥不能在界面上保存（设置写不受影响）"}
		p.setWriteState(state)
		return state
	}
	if err := edit.validate(); err != nil {
		state := WriteState{Kind: WriteInvalid, NS: apiclient.SettingsNSLLM,
			Path: []string{"providers", edit.route, "retryPolicy"}, Code: apiclient.SettingsCodeBadRequest, Message: err.Error()}
		p.setWriteState(state)
		return state
	}
	// ① 删除已存密钥（官方 :350-356）。nil = 壳没接这条能力：如实说，不许静默成功。
	if edit.keyRemovalRef != "" && !edit.keyRemovalApplied {
		if unsetKey == nil {
			state := WriteState{Kind: WriteFailed, NS: credentialNS, Path: []string{edit.keyRemovalRef},
				Message: "壳未接 credentials.unset：已保存的密钥不能在界面上删除"}
			p.setWriteState(state)
			return state
		}
		if err := unsetKey(edit.keyRemovalRef); err != nil {
			state := ClassifyWriteError(credentialNS, []string{edit.keyRemovalRef}, err)
			p.setWriteState(state)
			return state
		}
		// 删除已落库：就地记成"已删"（官方 `setKeyDraft({...removed:true})`）。
		// 这一步之后卡片即使留着也不会再发第二次 unset，且 footer 的 dismiss
		// 文案变成「关闭」（ProviderEditor.tsx:683）。
		edit.keyRemovalApplied = true
		o.mu.Lock()
		if o.modelsEdit != nil && o.modelsEdit.generation == edit.generation {
			o.modelsEdit.keyRemovalRef = edit.keyRemovalRef
			o.modelsEdit.keyRemovalApplied = true
		}
		o.mu.Unlock()
	}
	ops := edit.ops()
	written := WriteState{Kind: WriteIdle}
	if len(ops) == 0 && keyValue == "" {
		o.mu.Lock()
		if o.modelsEdit != nil && o.modelsEdit.generation == edit.generation {
			o.modelsEdit = nil
		}
		o.mu.Unlock()
		// 什么都没改是 idle；只删了密钥则是"真的写成功了一笔"（官方 :367 走
		// `onClose(true)`，卡片按已应用关闭），结果挂在凭据域上。
		state := WriteState{Kind: WriteIdle}
		if edit.keyRemovalApplied {
			state = WriteState{Kind: WriteSaved, NS: credentialNS, Path: []string{edit.keyRemovalRef}}
		}
		p.setWriteState(state)
		return state
	}
	if len(ops) > 0 {
		if writer == nil {
			state := ClassifyWriteError(apiclient.SettingsNSLLM, ops[0].Path, errNoSettingsWriter)
			p.setWriteState(state)
			return state
		}
		ctx, cancel := context.WithTimeout(context.Background(), panelWriteTimeout)
		defer cancel()
		state := ApplyMutateRevision(ctx, writer, apiclient.SettingsNSLLM, ops, &edit.revision)
		if !state.Success() {
			p.setWriteState(state)
			return state
		}
		written = state
	}
	if keyValue != "" {
		if err := setKey(edit.credentialRef, keyValue); err != nil {
			// 设置已落库、密钥没落：如实报凭据域的四字段，卡片保持打开让用户重试。
			state := ClassifyWriteError(credentialNS, []string{edit.credentialRef}, err)
			p.setWriteState(state)
			return state
		}
		if !written.Success() {
			// 只打密钥、没动设置：凭据写成功了就该给"已保存"（官方 `savedProvider`
			// 那句），只是没有 revision 可回——`credentials.set` 的结果本来就是 void。
			written = WriteState{Kind: WriteSaved, NS: credentialNS, Path: []string{edit.credentialRef}}
		}
	}
	o.mu.Lock()
	if o.modelsEdit != nil && o.modelsEdit.generation == edit.generation {
		o.modelsEdit = nil
	}
	o.mu.Unlock()
	if keyValue != "" {
		p.discardModelsKeyDraft()
	}
	// 只用密钥、没动设置时没有 mutate 结果可回：state 保持 idle（官方
	// `credentials.set` 的结果是 void，本来就没有 revision）。
	p.setWriteState(written)
	return written
}
