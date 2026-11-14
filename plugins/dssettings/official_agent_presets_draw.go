// official_agent_presets_draw.go —— 「Agent 预设」页的绘制（#3）。
//
// # 官方出处（逐条）
//
//	结构   OFF:dsh-client-ui-agent-preset/src/client/AgentPresetSection.tsx:96-176
//	几何   OFF:dsh-client-ui-agent-preset/src/client/AgentPresetSection.module.css:1-286
//	文案   OFF:dsh-client-ui-agent-preset/src/client/locales.ts:83-121（zh 逐字）
//
// # 与官方的**已登记差异**（不是随手简化，逐条写清为什么）
//
//  1. **不画「内置 / 自定义」分组头**。官方分组的判据是
//     `isBuiltInPreset = name === undefined && id ∈ {standard,ptc,minimal,cordis}`
//     （`OFF:dsh-agent-preset-registry/src/display.ts:52-58`）。本仓宿主发布的每条
//     roster 都带 `name`（`modes.Builtin` 的显示名），且 id 是
//     `default/read-only/workspace-write/danger-full-access` —— 按官方那条 fold，
//     **全部**条目都会被判成「自定义」，那是错的标签。宿主 wire 形状
//     （`OfficialAgentPreset`）没有 `builtin` 字段，要加属于跨线契约。
//     所以这里只画一个卡片网格，不画分组头：**宁可不分组，不贴错标签**。
//  2. 卡片页脚只画「设为新任务默认」这一枚动作。官方页脚还有「模式说明」
//     （走 `PresetGuideDialog` 的 guide 文案）与「查看配置」（走 `agentPresets.read`
//     的模态）。guide 文案是按官方四个 preset id 写的，本仓 id 不同，套上去就是
//     编内容；查看配置是另一条动作面，本轮未接。两者都不画 —— **不画死按钮**。
//
// # 颜色
//
// 官方选中卡用 `--dsw-alias-border-selected`、卡片底用 `--dsw-alias-settings-card-fill`。
// 本仓 Theme 没有这两枚 token，分别取 `BgLayer1`（卡片底，与内置插件页同一枚）
// 与 `BrandPrimary`（选中描边，与内置插件页的键盘焦点同一枚）。色板差异已登记。
package dssettings

import (
	"context"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// ── 「Agent 预设」页几何（锚点见 official_agent_presets.go 的文件头）──
const (
	// AgentPresetSectionGap / MaxWidth 是 `.section{gap:12px;max-width:720px}`（:1-7）。
	AgentPresetSectionGap      float32 = 12
	AgentPresetSectionMaxWidth float32 = 720
	// AgentPresetTitleFontSize / Weight 是 `.title{font-size:18px;font-weight:600}`（:9-13）。
	AgentPresetTitleFontSize float32 = 18
	AgentPresetTitleWeight   float32 = 600
	// AgentPresetIntroFontSize 是 `.intro{font-size:13px}`（:15-18）。
	AgentPresetIntroFontSize float32 = 13
	// AgentPresetCardsGap / CardMinWidth 是
	// `.cards{grid-template-columns:repeat(auto-fill,minmax(268px,1fr));gap:12px}`（:44-51）。
	AgentPresetCardsGap float32 = 12
	AgentPresetCardMinW float32 = 268
	// AgentPresetCardBorderWidth 是 `.card{border:0.5px}`（:54-56）。
	AgentPresetCardBorderWidth float32 = 0.5
	// AgentPresetCardMainPadTop/Bottom/X 是 `.cardMain{padding:14px 16px 12px}`（:179-192）。
	AgentPresetCardMainPadTop    float32 = 14
	AgentPresetCardMainPadBottom float32 = 12
	AgentPresetCardMainPadX      float32 = 16
	// AgentPresetCardMainGap 是 `.cardMain{gap:12px}`（同处）。
	AgentPresetCardMainGap float32 = 12
	// AgentPresetCardHeadGap 是 `.cardHead{gap:12px}`（:204-207）。
	AgentPresetCardHeadGap float32 = 12
	// AgentPresetCardIdentityGap 是 `.cardIdentity{gap:6px}`（:215-220）。
	AgentPresetCardIdentityGap float32 = 6
	// AgentPresetCardNameFontSize 是 `.cardName{font-size:15px}`（:223-227）。
	AgentPresetCardNameFontSize float32 = 15
	// AgentPresetCardNameLn 是 `.cardName{line-height:1.4}`（同处）在 15px 上的实际行高。
	AgentPresetCardNameLn float32 = 21
	// AgentPresetCardDescFontSize 是 `.cardDesc{font-size:13px}`（:240-244）。
	AgentPresetCardDescFontSize float32 = 13
	// AgentPresetCardDescLn 是 `.cardDesc{line-height:1.55}`（同处）在 13px 上的实际行高。
	AgentPresetCardDescLn float32 = 20
	// AgentPresetCardIDFontSize / Ln 是 `.cardId{font-size:11px;line-height:21px}`（:259-265）。
	AgentPresetCardIDFontSize float32 = 11
	AgentPresetCardIDLn       float32 = 21
	// AgentPresetCardFootPadY/X 与 FootGap 是 `.cardFoot{padding:6px 10px;gap:2px}`（:271-279）。
	AgentPresetCardFootPadY float32 = 6
	AgentPresetCardFootPadX float32 = 10
	AgentPresetCardFootGap  float32 = 2
	// AgentPresetTagFontSize / Ln / PadY / PadX 是 Tag 的等价物（官方 Tag 是
	// primitives 组件；本仓行的 tag 几何取内置插件页那一档，见 PluginsTag*）。
	AgentPresetTagFontSize float32 = 12
	AgentPresetTagLn       float32 = 18
	AgentPresetTagPadY     float32 = 1
	AgentPresetTagPadX     float32 = 8
	// AgentPresetErrorFontSize 是 `.error{font-size:12px}`（:384-388）。
	AgentPresetErrorFontSize float32 = 12
)

// drawAgentPresetsPage 画「Agent 预设」右页。
//
// 入参 a 是整个右页区域（与 drawPanelOptions 同一个矩形），padding 照它那一套来，
// 这样与「内置插件」页的内容左缘对齐。
func (p *Plugin) drawAgentPresetsPage(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, a AreaRect, section PanelSection) {
	panel := section.AgentPresets
	if panel == nil {
		// 没有数据面（壳没接 agentPresets.list）：那一页走 drawPanelOptions 的
		// 「未知」说明，不在这里画空网格。
		return
	}
	ui.Column(f).Absolute().Left(a.X).Top(a.Y).Width(a.Width).Height(a.Height).
		PadSides(OptionsPadTop, OptionsPadRight, OptionsPadBottom, OptionsPadLeft).
		Gap(AgentPresetSectionGap).Children(func() {
		// 外壳：`h2` 标题 + 简介（官方 `.title` 18/600、`.intro` 13/tertiary）。
		ui.Text(f, section.Title).FontSize(AgentPresetTitleFontSize).
			Weight(AgentPresetTitleWeight).Color(t.LabelPrimary).Ellipsis()
		ui.Text(f, panel.Intro).FontSize(AgentPresetIntroFontSize).
			LineHeight(RowDescLineHeight).Color(t.LabelTertiary).FillWidth()

		if panel.State != PanelOK {
			if panel.Note != "" {
				ui.Text(f, panel.Note).FontSize(AgentPresetErrorFontSize).
					LineHeight(RowDescLineHeight).Color(t.Warn).FillWidth()
			}
			return
		}
		if panel.DenyReason != "" {
			// 写路径不成立时**明说**：卡片仍然列出名册（那是真数据），
			// 但不可点，也不假装点得动。
			ui.Text(f, panel.DenyReason).FontSize(AgentPresetErrorFontSize).
				LineHeight(RowDescLineHeight).Color(t.Warn).FillWidth()
		}

		contentWidth := a.Width - OptionsPadLeft - OptionsPadRight
		if contentWidth > AgentPresetSectionMaxWidth {
			contentWidth = AgentPresetSectionMaxWidth
		}
		scroll := ui.Scroll(f).FillWidth().Grow(1).MinHeight(0)
		scroll.Children(func() {
			ui.Column(f).Width(contentWidth).Gap(AgentPresetCardsGap).Children(func() {
				p.drawAgentPresetCards(f, env, t, panel, contentWidth)
			})
		})
	})
}

// drawAgentPresetCards 按官方网格摆卡片：`repeat(auto-fill, minmax(268px, 1fr))` + gap 12。
func (p *Plugin) drawAgentPresetCards(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	panel *AgentPresetsPanel, width float32) {
	cards := panel.Cards
	if len(cards) == 0 {
		return
	}
	columns := 1
	if width >= AgentPresetCardMinW {
		columns = int((width + AgentPresetCardsGap) / (AgentPresetCardMinW + AgentPresetCardsGap))
	}
	if columns < 1 {
		columns = 1
	}
	cardWidth := (width - AgentPresetCardsGap*float32(columns-1)) / float32(columns)
	for i := 0; i < len(cards); i += columns {
		row := ui.Row(f).FillWidth().Gap(AgentPresetCardsGap).Items(ui.Start)
		row.Children(func() {
			for j := 0; j < columns; j++ {
				index := i + j
				if index >= len(cards) {
					ui.Row(f).Width(cardWidth)
					continue
				}
				p.drawAgentPresetCard(f, env, t, panel, cards[index], cardWidth)
			}
		})
	}
}

// drawAgentPresetCard 画一张 preset 卡（官方 `AgentPresetSection.tsx:130-165`）。
func (p *Plugin) drawAgentPresetCard(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	panel *AgentPresetsPanel, card AgentPresetCard, width float32) {
	// 官方 `.cardActive` = 平台底 + 选中描边；`.cardBroken` = error 描边。
	bg, stroke := t.BgLayer1, t.BorderL2
	if card.InUse {
		bg, stroke = t.BgLayer3, t.BrandPrimary
	}
	if card.Broken != "" {
		stroke = t.Error
	}
	selectable := panel.CanSetDefault && card.Broken == "" && !card.InUse

	root := ui.Column(f).Width(width).Radius(t.Radii.XL).Clip().
		Border(AgentPresetCardBorderWidth, stroke).Background(bg)
	root.Children(func() {
		main := ui.Column(f).FillWidth().
			PadSides(AgentPresetCardMainPadTop, AgentPresetCardMainPadX,
				AgentPresetCardMainPadBottom, AgentPresetCardMainPadX).
			Gap(AgentPresetCardMainGap).
			Hit(env.Tree.ID(env.Feat, "settings-agent-preset-"+card.ID), agentPresetCardLabel(card))
		if selectable {
			main = main.Hover(ui.Look{}.Background(t.Hover))
		}
		main.Children(func() {
			head := ui.Row(f).FillWidth().Gap(AgentPresetCardHeadGap).Items(ui.Start)
			head.Children(func() {
				identity := ui.Row(f).Grow(1).Gap(AgentPresetCardIdentityGap).Items(ui.Center)
				identity.Children(func() {
					ui.Text(f, card.Name).FontSize(AgentPresetCardNameFontSize).
						LineHeight(AgentPresetCardNameLn).Weight(400).Color(t.LabelPrimary).Ellipsis()
					if card.Broken != "" {
						p.drawAgentPresetTag(f, t, zhAgentPresetBroken, t.Error)
					}
					if card.InUse {
						p.drawAgentPresetTag(f, t, zhAgentPresetInUse, t.LabelSecondary)
					}
				})
				ui.Text(f, card.ID).Mono().FontSize(AgentPresetCardIDFontSize).
					LineHeight(AgentPresetCardIDLn).Color(t.LabelTertiary).Shrink(0).Ellipsis()
			})
			desc := card.Description
			if desc == "" {
				desc = zhAgentPresetNoDesc
			}
			ui.Text(f, desc).FontSize(AgentPresetCardDescFontSize).
				LineHeight(AgentPresetCardDescLn).Color(t.LabelSecondary).FillWidth()
			if card.Broken != "" {
				ui.Text(f, card.Broken).FontSize(AgentPresetErrorFontSize).
					LineHeight(RowDescLineHeight).Color(t.Error).FillWidth()
			}
		})
		if selectable && main.Clicked() {
			go func() { _ = p.applyAgentPresetDefault(panel, card) }()
		}
		// 页脚只在**有动作**时画：官方页脚的其余两枚（模式说明 / 查看配置）
		// 本轮不接，见文件头「已登记差异」第 2 条。
		if selectable {
			ui.Row(f).FillWidth().Gap(AgentPresetCardFootGap).Items(ui.Center).
				Justify(ui.End).
				PadSides(AgentPresetCardFootPadY, AgentPresetCardFootPadX,
					AgentPresetCardFootPadY, AgentPresetCardFootPadX).
				Border(AgentPresetCardBorderWidth, t.BorderL2).Children(func() {
				ui.Text(f, zhAgentPresetSetDefault).FontSize(AgentPresetTagFontSize).
					LineHeight(AgentPresetTagLn).Color(t.LabelSecondary)
			})
		}
	})
}

// drawAgentPresetTag 画卡片头里的小标签（官方 `Tag`；本仓是纯文字胶囊）。
func (p *Plugin) drawAgentPresetTag(f ui.C, t *pluginapi.Theme, text string, fg ui.Color) {
	ui.Row(f).Shrink(0).Height(AgentPresetTagLn+2*AgentPresetTagPadY).
		PadSides(AgentPresetTagPadY, AgentPresetTagPadX, AgentPresetTagPadY, AgentPresetTagPadX).
		Radius(t.Radii.SM).Background(t.BgLayer2).Items(ui.Center).Children(func() {
		ui.Text(f, text).FontSize(AgentPresetTagFontSize).
			LineHeight(AgentPresetTagLn).Color(fg)
	})
}

// agentPresetCardLabel 是卡片的可访问名（官方 `aria-label` 是
// `${selectionAction}: ${display.name}`，`AgentPresetSection.tsx:139-140`）。
//
// 已选中的卡在官方是 `t('inUse')`，可选的是 `t('setDefault')`；
// 两者都按官方逐字，不另编文案。
func agentPresetCardLabel(card AgentPresetCard) string {
	action := zhAgentPresetSetDefault
	if card.InUse {
		action = zhAgentPresetInUse
	}
	return action + ": " + card.Name
}

// applyAgentPresetDefault 是「设为新任务默认」的写路径（#3 的那半条）。
//
// 写的是**设置命名空间的一个字段**（官方 `settings.update('agent-preset-registry',
// {selectedDefault: id}, rev)`，`OFF:.../settings-store.ts:19-26`），不是 preset
// 自身的属性：宿主在写路径上校验 id 真实存在，未知 id 当场 400 —— 这里把宿主的
// 四字段错误原样交给面板的 WriteState 渲染，**不做本地乐观更新**。
func (p *Plugin) applyAgentPresetDefault(panel *AgentPresetsPanel, card AgentPresetCard) WriteState {
	o := p.ensureOfficial()
	o.mu.Lock()
	writer := o.writer
	refresh := o.agentPresetRefresh
	o.mu.Unlock()
	row := PanelRow{
		ID:       "agent-preset-" + card.ID,
		NS:       apiclient.SettingsNSAgentPreset,
		Path:     []string{apiclient.SettingsFieldSelectedDefault},
		Revision: panel.Revision,
	}
	if writer == nil {
		state := ClassifyWriteError(row.NS, row.Path, errNoSettingsWriter)
		p.setWriteState(state)
		return state
	}
	ctx, cancel := context.WithTimeout(context.Background(), panelWriteTimeout)
	defer cancel()
	state := ApplySet(ctx, writer, row.NS, row.Path, card.ID, row.Revision)
	p.setWriteState(state)
	if state.Success() && refresh != nil {
		// 写成功后名册的 `isDefault` 换行了：让壳重拉一次。
		// 不刷新的话角标会在旧的那张卡上，而后缀会话已经用新 preset 了。
		refresh()
	}
	return state
}
