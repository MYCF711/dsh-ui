// official_plugins_draw.go —— 「内置插件」页的绘制（官方两段组件的等价物）。
//
// # 官方出处（逐条）
//
//	外壳   OFF:ui-settings-plugins/src/client/PluginsSettingsSection.tsx:53-118
//	       OFF:ui-settings-plugins/src/client/PluginsSettingsSection.module.css:1-113
//	清单页 OFF:ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.tsx:427-578
//	       OFF:ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:1-512
//
// # 画的是哪一层
//
// 官方 `PluginsSettingsSection` 在 `settings.plugins.tab` 只有**一个**贡献时
// 直接渲染那一页、**不画 tab 条**（同文件 :40,57-58）。本仓当前正好是这种情况
// （只做了只读清单这一页），所以这里画的是：标题 + 简介 + 清单页本体。
// 多 tab 的 tab 条没有实现（第二条 tab 属 #29 的可配置管理页，见报告）。
//
// # 交互（官方都是真动作，这里都接上）
//
//	· 点段标题 -> 展开/收起那一段（:481-490,552-562）
//	· 点卡片   -> 展开/收起详情（手风琴：同时只有一张展开，:140,325-327）
//	· 搜索框   -> 实时过滤（:464-474）
//	· 失败时的「重试」-> 让壳重读一次清单（:454-461）
//
// # 图标
//
// 官方用的是 sprite 图标（`IconChevronDownOutlineRegular` / `IconSearchOutlineRegular`）。
// 本仓没有那张 sprite 的导出，按仓规**不许手绘冒充官方图标**；这里用文字字形
// （`▾`/`▸`/`⌕`），与 `dssidebar` 的搜索框、`dssettings` 的 `›`/`×` 同一套做法。
// 官方 chevron 的 `rotate(-90deg)`（收起态）在这里等价成换一个字形。
package dssettings

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// 文字字形：官方 sprite 图标的等价物（见文件头「图标」一段）。
const (
	glyphChevronDown      = "▾"
	glyphChevronCollapsed = "▸"
	glyphSearch           = "⌕"
)

// drawPluginsPage 画「内置插件」右页。
//
// 入参的 a 是整个右页区域（与 drawPanelOptions 同一个矩形），padding 也照它
// 那一套来（OptionsPadX / OptionsPadBottom），这样两个页面的内容左缘对齐。
func (p *Plugin) drawPluginsPage(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, a AreaRect, section PanelSection) {
	plugins := section.Plugins
	if plugins == nil {
		return
	}
	ui.Column(f).Absolute().Left(a.X).Top(a.Y).Width(a.Width).Height(a.Height).
		PadSides(OptionsPadTop, OptionsPadRight, OptionsPadBottom, OptionsPadLeft).
		Gap(PluginsSectionOuterGap).Children(func() {
		// 外壳：`h2` 标题 + 简介（官方 `.heading` 18/600、`.intro` 13/tertiary）。
		ui.Text(f, section.Title).FontSize(PluginsHeadingFontSize).
			Weight(PluginsHeadingWeight).Color(t.LabelPrimary).Ellipsis()
		ui.Text(f, zhPluginsIntro).FontSize(PluginsIntroFontSize).Color(t.LabelTertiary).Ellipsis()

		contentWidth := a.Width - OptionsPadLeft - OptionsPadRight
		if contentWidth > PluginsSectionMaxWidth {
			contentWidth = PluginsSectionMaxWidth
		}

		// 外壳的 tab 条：**只有多 tab 时才画**（官方 :57-58）。
		if plugins.ShowTabBar {
			p.drawPluginsTabs(f, env, t, plugins, contentWidth)
		}

		// 清单本体进滚动容器：宿主真有几十条插件时，其余条目必须够得着
		// （官方整页是窗口滚动；本仓面板是固定高浮层，所以滚的是内容区）。
		scroll := ui.Scroll(f).FillWidth().Grow(1).MinHeight(0)
		scroll.Children(func() {
			ui.Column(f).Width(contentWidth).Gap(PluginsCatalogGap).Children(func() {
				p.drawPluginsStatus(f, env, t, plugins, contentWidth)
				if plugins.TabMissing != "" {
					// 没有真实现的 tab：明说缺什么，不画一页空壳。
					ui.Text(f, plugins.TabMissing).FontSize(PluginsStatusFontSize).
						LineHeight(PluginsStatusLn).Color(t.Warn).FillWidth()
					return
				}
				if plugins.State != PluginsReady {
					return
				}
				p.drawPluginsSearch(f, env, t, contentWidth)
				for _, group := range plugins.Groups {
					p.drawPluginsGroup(f, env, t, group, contentWidth)
				}
				if plugins.PresetGap != "" {
					// 自检行：宿主给了 agentPresets 而本页没画那一段时明说，
					// 不静默少一段（当前宿主不产出，正常路径永远看不到它）。
					ui.Text(f, plugins.PresetGap).FontSize(PluginsBrokenFontSize).
						LineHeight(PluginsBrokenLn).Color(t.Warn)
				}
			})
		})
	})
}

// drawPluginsTabs 画 tab 条（官方 `.tabs`：底部分隔线 0.5px、gap 22、选中态 2px 下划线）。
func (p *Plugin) drawPluginsTabs(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, plugins *PluginsPanel, width float32) {
	o := p.ensureOfficial()
	bar := ui.Row(f).Width(width).Gap(PluginsTabsGap).Items(ui.End).
		MarginSides(PluginsTabsMarginT, 0, 0, 0)
	bar.Children(func() {
		for _, tab := range plugins.Tabs {
			tab := tab
			active := tab.ID == plugins.ActiveTab
			cell := ui.Column(f).Shrink(0).
				PadSides(PluginsTabPadTop, PluginsTabPadX, PluginsTabPadBt, PluginsTabPadX).
				Hit(env.Tree.ID(env.Feat, "settings-plugins-tab-"+tab.ID), tab.Label).
				Hover(ui.Look{}.Background(t.Hover))
			cell.Children(func() {
				color := t.LabelTertiary
				if active {
					color = t.LabelPrimary
				}
				ui.Text(f, tab.Label).FontSize(PluginsTabFontSize).LineHeight(PluginsTabLn).Color(color)
				if active {
					// 官方选中态 `::after`（2px，贴底）。
					ui.Row(f).FillWidth().Height(PluginsTabUnderline).Radius(PluginsTabUnderline).
						Background(t.LabelPrimary)
				}
			})
			if cell.Clicked() {
				o.mu.Lock()
				o.pluginsActiveTab = tab.ID
				o.mu.Unlock()
			}
		}
	})
	// 官方 `.tabs{border-bottom:0.5px solid border-l2}`。
	ui.Row(f).Width(width).Height(PluginsRetryBorderWidth).Background(t.BorderL2)
}

// drawPluginsStatus 画状态区：未知/失败原因、重试按钮、空清单/无命中文案。
func (p *Plugin) drawPluginsStatus(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, plugins *PluginsPanel, width float32) {
	if plugins.Note != "" {
		color := t.LabelTertiary
		if plugins.State == PluginsError || plugins.State == PluginsUnknown {
			color = t.Error
		}
		ui.Text(f, plugins.Note).FontSize(PluginsStatusFontSize).LineHeight(PluginsStatusLn).
			Color(color).FillWidth()
	}
	if !plugins.ShowRetry {
		return
	}
	// 官方 `.failure button`（:37-45）：0.5px 描边、radius-sm、padding 4 10。
	row := ui.Row(f).Height(PluginsStatusLn+2*PluginsRetryPadY).
		PadSides(PluginsRetryPadY, PluginsRetryPadX, PluginsRetryPadY, PluginsRetryPadX).
		Radius(t.Radii.SM).Border(PluginsRetryBorderWidth, t.BorderL3).Items(ui.Center).
		Hit(env.Tree.ID(env.Feat, "settings-plugins-retry"), zhPluginsRetry).
		Hover(ui.Look{}.Background(t.Hover))
	row.Children(func() {
		ui.Text(f, zhPluginsRetry).FontSize(PluginsStatusFontSize).Color(t.LabelPrimary)
	})
	if row.Clicked() && env.Act.PluginInventoryRefresh != nil {
		env.Act.PluginInventoryRefresh()
	}
}

// drawPluginsSearch 画搜索框（官方 `.search`：height 36、圆角 md、0.5px 描边、
// 左内缩 36 / 右内缩 34、放大镜在左 12）。
func (p *Plugin) drawPluginsSearch(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, width float32) {
	o := p.ensureOfficial()
	ui.Row(f).Width(width).Height(PluginsSearchHeight).Radius(t.Radii.MD).
		Border(PluginsRetryBorderWidth, t.BorderL4).Background(t.BgLayer1).
		Items(ui.Center).Children(func() {
		ui.Row(f).Width(PluginsSearchIconLeft + PluginsSearchFontSize).Height(PluginsSearchHeight).
			Shrink(0).Items(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(f, glyphSearch).FontSize(PluginsSearchFontSize).Color(t.LabelTertiary)
		})
		ui.TextFieldID(f, env.Tree.ID(env.Feat, "settings-plugins-search"), &o.pluginsSearchField,
			zhPluginsSearch, zhPluginsSearch).
			Grow(1).Height(PluginsSearchHeight).FontSize(PluginsSearchFontSize)
	})
}

// drawPluginsGroup 画一段（官方 `.group`：标题行 36 高、副标题 -6/20 缩进、段体 10 间距）。
func (p *Plugin) drawPluginsGroup(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, group PluginGroup, width float32) {
	o := p.ensureOfficial()
	// 段与段之间有 0.5px 分隔 + 14 上边距（官方 `.group + .group`）。
	if group.Key != "global" {
		ui.Divider(f)
	}
	ui.Column(f).Width(width).Gap(PluginsGroupGap).Children(func() {
		head := ui.Row(f).FillWidth().MinHeight(PluginsGroupTitleRowMin).Gap(PluginsGroupTitleRowGap).
			Items(ui.Center).Hit(env.Tree.ID(env.Feat, "settings-plugins-group-"+group.Key), group.Title).
			Hover(ui.Look{}.Background(t.Hover))
		head.Children(func() {
			glyph := glyphChevronCollapsed
			if group.Open {
				glyph = glyphChevronDown
			}
			ui.Text(f, glyph).FontSize(PluginsChevronSize).Color(t.LabelTertiary)
			ui.Text(f, group.Title).FontSize(PluginsGroupTitle).LineHeight(PluginsGroupTitleLn).
				Weight(400).Color(t.LabelPrimary)
		})
		if head.Clicked() {
			o.mu.Lock()
			next := !group.Open
			o.pluginsGlobalOpen = &next
			o.mu.Unlock()
		}
		ui.Text(f, group.Subtitle).FontSize(PluginsGroupSubFontSize).LineHeight(PluginsGroupSubLn).
			Color(t.LabelTertiary).MarginSides(PluginsGroupSubMarginT, 0, 0, PluginsGroupSubMarginL)

		if !group.Open || group.Count == 0 {
			return
		}
		ui.Column(f).Width(width).Gap(PluginsGroupBodyGap).Children(func() {
			p.drawPluginCards(f, env, t, group.Cards, width)
		})
	})
}

// drawPluginCards 按官方网格摆卡片：两列 `minmax(0,1fr)`，容器 ≤520 时单列。
func (p *Plugin) drawPluginCards(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, cards []PluginCard, width float32) {
	if len(cards) == 0 {
		return
	}
	columns := int(PluginsCardColumns)
	if width <= PluginsCardSingleCol || columns < 1 {
		columns = 1
	}
	cardWidth := (width - PluginsCardsGap*float32(columns-1)) / float32(columns)
	for i := 0; i < len(cards); i += columns {
		row := ui.Row(f).FillWidth().Gap(PluginsCardsGap).Items(ui.Start)
		row.Children(func() {
			for j := 0; j < columns; j++ {
				index := i + j
				if index >= len(cards) {
					// 官方网格最后一格就是这么留空的；补一个占位保持列宽。
					ui.Row(f).Width(cardWidth)
					continue
				}
				p.drawPluginCard(f, env, t, cards[index], cardWidth)
			}
		})
	}
}

// drawPluginCard 画一张卡（官方 `PluginCard`，:123-178）。
func (p *Plugin) drawPluginCard(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, card PluginCard, width float32) {
	o := p.ensureOfficial()
	stroke := t.BorderL2
	if card.Failed {
		// 官方 `.card[data-failed]{border-color: color-mix(error 45%, transparent)}`；
		// 本仓没有混色 API，用纯 error 描边（色板差异已登记在报告 §缺口）。
		stroke = t.Error
	}
	if card.Expanded {
		// 官方 `.card[data-open]{border-color: var(--dsw-alias-border-l3)}`。
		stroke = t.BorderL3
	}
	if p.pluginCardFocused(card.RowKey) {
		// 官方 `.cardContent:focus-visible{outline: focus-ring}` 的等价物：
		// 本仓没有 outline 层，用同一枚描边换色表达键盘焦点。
		stroke = t.BrandPrimary
	}
	root := ui.Column(f).Width(width).Radius(t.Radii.XL).Clip().
		Border(PluginsRetryBorderWidth, stroke).Background(t.BgLayer1)
	root.Children(func() {
		content := ui.Column(f).FillWidth().MinHeight(PluginsCardMinHeight).
			PadSides(PluginsCardPad, PluginsCardPadX, PluginsCardPad, PluginsCardPadX).
			Gap(PluginsCardContentGap).
			Hit(env.Tree.ID(env.Feat, "settings-plugins-card-"+card.RowKey), pluginCardLabel(card)).
			Hover(ui.Look{}.Background(t.Hover))
		content.Children(func() {
			main := ui.Row(f).FillWidth().Gap(PluginsCardMainRowGap).Items(ui.Center)
			main.Children(func() {
				ui.Text(f, card.Title).FontSize(PluginsCardTitleFontSize).
					LineHeight(PluginsCardTitleLn).Weight(500).Color(t.LabelPrimary).
					Grow(1).Ellipsis()
				trailing := ui.Row(f).Shrink(0).Gap(PluginsCardTrailingGap).Items(ui.Center)
				trailing.Children(func() {
					if card.PhaseDot {
						ui.Row(f).Width(PluginsDotSize).Height(PluginsDotSize).
							Radius(PluginsDotSize / 2).Background(t.BrandPrimary)
					}
					if card.Tag {
						p.drawPluginTag(f, t, card)
					}
					glyph := glyphChevronCollapsed
					if card.Expanded {
						glyph = glyphChevronDown
					}
					ui.Text(f, glyph).FontSize(PluginsChevronSize).Color(t.LabelTertiary)
				})
			})
			if card.Desc != "" {
				ui.Text(f, card.Desc).FontSize(PluginsCardDescFontSize).
					LineHeight(PluginsCardDescLn).Color(t.LabelTertiary).FillWidth()
			}
			if card.EntryChip != "" {
				ui.Row(f).FillWidth().MarginSides(PluginsCardMetaPadTop, 0, 0, 0).Children(func() {
					ui.Row(f).Height(PluginsCardChipLn+2*PluginsCardChipPadY).
						PadSides(PluginsCardChipPadY, PluginsCardChipPadX, PluginsCardChipPadY, PluginsCardChipPadX).
						Radius(t.Radii.XS).Background(t.BgLayer3).Items(ui.Center).Children(func() {
						ui.Text(f, card.EntryChip).Mono().FontSize(PluginsCardChipFontSize).
							LineHeight(PluginsCardChipLn).Color(t.LabelSecondary).Ellipsis()
					})
				})
			}
		})
		if content.Clicked() {
			o.mu.Lock()
			if o.pluginsExpanded == card.RowKey {
				o.pluginsExpanded = ""
			} else {
				o.pluginsExpanded = card.RowKey
			}
			o.mu.Unlock()
		}
		if card.MetaError != "" {
			p.drawPluginBrokenNote(f, t, card.MetaError)
		}
		if card.Expanded {
			p.drawPluginDetails(f, t, card)
		}
	})
}

// drawPluginTag 画行尾状态标签（官方 `Tag`；三档色调见 TAG_TONES）。
func (p *Plugin) drawPluginTag(f ui.C, t *pluginapi.Theme, card PluginCard) {
	bg, fg := t.BgLayer3, t.LabelSecondary
	if card.Failed {
		// 官方 danger 档是 error 容器色（本仓色板没有 error tertiary，见报告缺口）。
		fg = t.Error
	}
	ui.Row(f).Height(PluginsTagLn+2*PluginsTagPadY).
		PadSides(PluginsTagPadY, PluginsTagPadX, PluginsTagPadY, PluginsTagPadX).
		Radius(t.Radii.SM).Background(bg).Items(ui.Center).Children(func() {
		ui.Text(f, card.State).FontSize(PluginsTagFontSize).LineHeight(PluginsTagLn).Color(fg)
	})
}

// drawPluginBrokenNote 画错误注记（官方 `.brokenNote`：radius-lg、8/10 内缩、error 系）。
func (p *Plugin) drawPluginBrokenNote(f ui.C, t *pluginapi.Theme, text string) {
	ui.Row(f).FillWidth().PadSides(PluginsBrokenPadY, PluginsBrokenPadX, PluginsBrokenPadY, PluginsBrokenPadX).
		Radius(t.Radii.LG).Background(t.BgLayer3).Children(func() {
		ui.Text(f, text).FontSize(PluginsBrokenFontSize).LineHeight(PluginsBrokenLn).
			Color(t.Error).FillWidth()
	})
}

// drawPluginDetails 画展开详情（官方 `.cardDetails` + `CardFacts`，:180-204,437-473）。
func (p *Plugin) drawPluginDetails(f ui.C, t *pluginapi.Theme, card PluginCard) {
	ui.Column(f).FillWidth().
		PadSides(PluginsDetailsPadTop, PluginsDetailsPadX, PluginsDetailsPadBottom, PluginsDetailsPadX).
		Background(t.BgLayer3).Border(PluginsRetryBorderWidth, t.BorderL2).Children(func() {
		// Loader entry id 一行（官方 `<code className={css.entryValue}>`）。
		if card.EntryID != "" {
			ui.Text(f, card.EntryID).Mono().FontSize(PluginsCardChipFontSize).
				LineHeight(PluginsCardChipLn).Color(t.LabelPrimary).FillWidth()
		}
		ui.Column(f).FillWidth().MarginTop(PluginsDetailsMarginTop).
			Gap(PluginsDetailsRowGap).Children(func() {
			// 第一行永远是「完整名称」（官方 `CardFacts` 固定加的那一行）。
			p.drawPluginFact(f, t, zhPluginsModuleLabel, card.ModuleName)
			for _, fact := range card.Facts {
				p.drawPluginFact(f, t, fact.Label, fact.Value)
			}
		})
		if card.Diagnostic != "" {
			p.drawPluginBrokenNote(f, t, card.Diagnostic)
		}
	})
}

// drawPluginFact 画一行 `dt`/`dd`（官方 `.details{grid-template-columns:76px 1fr}`）。
func (p *Plugin) drawPluginFact(f ui.C, t *pluginapi.Theme, label, value string) {
	ui.Row(f).FillWidth().Gap(PluginsDetailsColGap).Items(ui.Start).Children(func() {
		ui.Row(f).Width(PluginsDetailsLabelWidth).Shrink(0).Children(func() {
			ui.Text(f, label).FontSize(PluginsDetailsDtFontSize).LineHeight(PluginsDetailsDtLn).
				Color(t.LabelTertiary).Ellipsis()
		})
		ui.Text(f, value).FontSize(PluginsDetailsDdFontSize).LineHeight(PluginsDetailsDdLn).
			Color(t.LabelSecondary).Grow(1)
	})
}

// pluginCardLabel 是卡片按钮的可访问名（官方 `cardLabel`，:88-91）：
// 标题、可见的完整 entry id、状态，三段用逗号连。
func pluginCardLabel(card PluginCard) string {
	if card.EntryID != "" && card.EntryChip != "" {
		return card.Title + ", " + card.EntryID + ", " + card.State
	}
	return card.Title + ", " + card.State
}
