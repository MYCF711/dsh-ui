// settings_items_subagent_draw.go —— 官方 #46「模型选择」section 的绘制。
// 结构：SubagentModelSelectionFields.tsx:61-126；几何：同名 module.css:1-138。
// 差异登记：Viva 没有原生 fieldset/input[type=checkbox]，本文件用同位置滚动框、
// 14px 方框与 dsassets 的官方 CheckOutline 路径表达选中；不新增图标或文案。
package dssidebar

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const (
	subagentPermissionPadY float32 = 12
	subagentPermissionGap  float32 = 6
	subagentToggleGap      float32 = 16
	subagentToggleFont     float32 = 13
	subagentToggleLine     float32 = 19.5
	subagentSelectionGap   float32 = 10
	subagentNoticeFont     float32 = 12
	subagentNoticeLine     float32 = 18
	subagentCatalogGap     float32 = 12
	subagentModelsGap      float32 = 6
	subagentModelsMaxH     float32 = 280
	subagentModelsPad      float32 = 10
	subagentModelsStroke   float32 = 0.5
	subagentLegendFont     float32 = 12
	subagentGroupGap       float32 = 6
	subagentGroupMargin    float32 = 4
	subagentGroupPadTop    float32 = 10
	subagentProviderPadX   float32 = 6
	subagentProviderFont   float32 = 11
	subagentModelGap       float32 = 8
	subagentModelPad       float32 = 6
	subagentModelNameFont  float32 = 13
	subagentModelRouteFont float32 = 11
	subagentModelRouteGap  float32 = 2
	subagentCheckboxSize   float32 = 14 // 平台适配：官方浏览器 checkbox 的默认控件尺寸。
)

// settingsSubagentModelGeometry 把本节 CSS 数字逐行钉住；测试不许只验 UI 能编译。
func settingsSubagentModelGeometry() []itemGeoRow {
	const css = "ui-settings-subagent/src/client/SubagentModelSelectionFields.module.css"
	return []itemGeoRow{
		{"permission", "padding-block", subagentPermissionPadY, subagentPermissionPadY, css + ":1-5"},
		{"permission", "gap", subagentPermissionGap, subagentPermissionGap, css + ":1-5"},
		{"toggleRow", "gap", subagentToggleGap, subagentToggleGap, css + ":7-15"},
		{"toggleRow", "font-size", subagentToggleFont, subagentToggleFont, css + ":7-15"},
		{"selection", "gap", subagentSelectionGap, subagentSelectionGap, css + ":22-25"},
		{"notice", "font-size", subagentNoticeFont, subagentNoticeFont, css + ":27-34"},
		{"notice", "line-height", subagentNoticeLine, subagentNoticeLine, css + ":27-34"},
		{"catalogError", "gap", subagentCatalogGap, subagentCatalogGap, css + ":46-53"},
		{"models", "gap", subagentModelsGap, subagentModelsGap, css + ":63-73"},
		{"models", "max-height", subagentModelsMaxH, subagentModelsMaxH, css + ":63-73"},
		{"models", "padding", subagentModelsPad, subagentModelsPad, css + ":63-73"},
		{"models", "border-width", subagentModelsStroke, subagentModelsStroke, css + ":63-73"},
		{"models.legend", "font-size", subagentLegendFont, subagentLegendFont, css + ":75-79"},
		{"modelGroup", "gap", subagentGroupGap, subagentGroupGap, css + ":81-84"},
		{"modelGroup+modelGroup", "margin-top", subagentGroupMargin, subagentGroupMargin, css + ":86-90"},
		{"modelGroup+modelGroup", "padding-top", subagentGroupPadTop, subagentGroupPadTop, css + ":86-90"},
		{"providerName", "padding-inline", subagentProviderPadX, subagentProviderPadX, css + ":92-97"},
		{"providerName", "font-size", subagentProviderFont, subagentProviderFont, css + ":92-97"},
		{"model", "gap", subagentModelGap, subagentModelGap, css + ":99-108"},
		{"model", "padding", subagentModelPad, subagentModelPad, css + ":99-108"},
		{"modelName", "font-size", subagentModelNameFont, subagentModelNameFont, css + ":124-127"},
		{"route", "margin-top", subagentModelRouteGap, subagentModelRouteGap, css + ":129-133"},
		{"route", "font-size", subagentModelRouteFont, subagentModelRouteFont, css + ":129-133"},
	}
}

type settingsSubagentModelGroup struct {
	Provider   string
	Name       string
	Candidates []settingsSubagentModelCandidate
}

// groupSettingsSubagentCandidates 按目录出现顺序分 provider，失效路由单独放官方最后一组。
func groupSettingsSubagentCandidates(candidates []settingsSubagentModelCandidate) (available []settingsSubagentModelGroup, unavailable settingsSubagentModelGroup) {
	indices := map[string]int{}
	for _, candidate := range candidates {
		if !candidate.Available {
			unavailable.Name = zhSubagentModelStaleGroup
			unavailable.Candidates = append(unavailable.Candidates, candidate)
			continue
		}
		index, ok := indices[candidate.Provider]
		if !ok {
			index = len(available)
			indices[candidate.Provider] = index
			available = append(available, settingsSubagentModelGroup{Provider: candidate.Provider, Name: candidate.ProviderName})
		}
		available[index].Candidates = append(available[index].Candidates, candidate)
	}
	return available, unavailable
}

func (p *Plugin) drawSettingsSubagentModelSection(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, card settingsItemCardState) {
	model := card.Model
	if model == nil {
		return
	}
	ui.Column(f).FillWidth().PadSides(itemSectionPadY, 0, itemSectionPadY, 0).Children(func() {
		ui.Text(f, zhSubagentModels).FontSize(itemSectionHeadSze).LineHeight(itemSectionHeadLn).
			Weight(gfx.WeightSemibold).Color(t.LabelPrimary)
		if !model.Known {
			ui.Text(f, zhItemUnknownValue).FillWidth().FontSize(subagentNoticeFont).
				LineHeight(subagentNoticeLine).Color(t.Error)
			return
		}
		// .permission > .toggleRow + .hint。
		ui.Column(f).FillWidth().Gap(subagentPermissionGap).
			PadSides(subagentPermissionPadY, 0, subagentPermissionPadY, 0).Children(func() {
			ui.Row(f).FillWidth().Items(ui.Center).Gap(subagentToggleGap).Children(func() {
				ui.Text(f, zhSubagentModelToggle).Grow(1).MinWidth(0).FontSize(subagentToggleFont).
					LineHeight(subagentToggleLine).Color(t.LabelPrimary)
				disabled := !model.Known || !card.Writable || card.Saving
				if drawSettingsSubagentSwitch(f, env, t, model.Enabled, disabled) {
					p.ensureSettingsItemForm().stageSubagentModel(model, !model.Enabled, model.DesiredRoutes)
				}
			})
			hint := zhSubagentModelOff
			if model.Enabled {
				hint = zhSubagentModelChoose
			}
			ui.Wrap(f, hint).FillWidth().FontSize(subagentNoticeFont).LineHeight(subagentNoticeLine).Color(t.LabelTertiary)
		})
		if model.Enabled {
			p.drawSettingsSubagentModelSelection(f, env, t, card)
		}
		if model.Conflicted {
			ui.Wrap(f, zhSubagentModelConflict).FillWidth().FontSize(subagentNoticeFont).
				LineHeight(subagentNoticeLine).Color(t.Error)
		}
	})
}

// drawSettingsSubagentSwitch 使用官方 36×20/旋钮 16×16；off 必须靠左，不能复用
// plugins 页那个始终 Justify(End) 的 bundle 开关外观。
func drawSettingsSubagentSwitch(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, on, disabled bool) bool {
	bg := t.BorderL3
	justify := ui.Start
	if on {
		bg = t.BrandPrimary
		justify = ui.End
	}
	row := ui.Row(f).Width(pluginSwitchWidth).Height(pluginSwitchHeight).Shrink(0).
		Radius(pluginSwitchHeight/2).Background(bg).
		PadSides(pluginSwitchPad, pluginSwitchPad, pluginSwitchPad, pluginSwitchPad).
		Justify(justify).Hit(env.Tree.ID(env.Feat, "settings-subagent-model-enabled"), zhSubagentModelToggle).Children(func() {
		ui.Row(f).Size(pluginSwitchKnob, pluginSwitchKnob).Radius(pluginSwitchKnob / 2).Background(t.BgLayer2)
	})
	if disabled {
		return false
	}
	return row.Clicked()
}

func (p *Plugin) drawSettingsSubagentModelSelection(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, card settingsItemCardState) {
	model := card.Model
	ui.Column(f).FillWidth().Gap(subagentSelectionGap).Children(func() {
		if model.CatalogStatus == settingsSubagentCatalogLoading {
			ui.Text(f, zhSubagentModelLoading).FillWidth().FontSize(subagentNoticeFont).
				LineHeight(subagentNoticeLine).Color(t.LabelTertiary)
		}
		if model.CatalogStatus == settingsSubagentCatalogError {
			ui.Row(f).FillWidth().Items(ui.Center).Gap(subagentCatalogGap).Children(func() {
				ui.Text(f, zhSubagentModelLoadFailed).Grow(1).FontSize(subagentNoticeFont).
					LineHeight(subagentNoticeLine).Color(t.Error)
				if model.CatalogRetry && env.Act.ModelsPageRefresh != nil {
					retry := ui.Row(f).Shrink(0).Hit(env.Tree.ID(env.Feat, "settings-subagent-model-retry"), zhSubagentModelRetry).
						Children(func() {
							ui.Text(f, zhSubagentModelRetry).FontSize(subagentNoticeFont).LineHeight(subagentNoticeLine).Color(t.Link)
						})
					if !card.Saving && retry.Clicked() {
						env.Act.ModelsPageRefresh()
					}
				}
			})
		}
		if model.CatalogPartial {
			ui.Wrap(f, zhSubagentModelPartial).FillWidth().FontSize(subagentNoticeFont).
				LineHeight(subagentNoticeLine).Color(t.LabelTertiary)
		}
		if len(model.Candidates) > 0 {
			p.drawSettingsSubagentModelList(f, env, t, card)
		} else if model.CatalogStatus == settingsSubagentCatalogReady {
			ui.Text(f, zhSubagentModelEmpty).FillWidth().FontSize(subagentNoticeFont).
				LineHeight(subagentNoticeLine).Color(t.LabelTertiary)
		}
		if model.Invalid {
			ui.Text(f, zhSubagentModelRequired).FillWidth().FontSize(subagentNoticeFont).
				LineHeight(subagentNoticeLine).Color(t.Error)
		}
	})
}

func (p *Plugin) drawSettingsSubagentModelList(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, card settingsItemCardState) {
	available, unavailable := groupSettingsSubagentCandidates(card.Model.Candidates)
	groups := available
	if len(unavailable.Candidates) > 0 {
		groups = append(append([]settingsSubagentModelGroup(nil), groups...), unavailable)
	}
	ui.Scroll(f).FillWidth().MaxHeight(subagentModelsMaxH).Clip().
		Border(subagentModelsStroke, t.BorderL4).Radius(t.Radii.LG).
		PadSides(subagentModelsPad, subagentModelsPad, subagentModelsPad, subagentModelsPad).Children(func() {
		ui.Column(f).FillWidth().Gap(subagentModelsGap).Children(func() {
			ui.Text(f, zhSubagentModelAllowed).FontSize(subagentLegendFont).LineHeight(subagentNoticeLine).Color(t.LabelSecondary)
			for index, group := range groups {
				group := group
				ui.Column(f).FillWidth().Gap(subagentGroupGap).Children(func() {
					if index > 0 {
						ui.Column(f).FillWidth().MarginSides(subagentGroupMargin, 0, 0, 0).
							PadSides(subagentGroupPadTop, 0, 0, 0).Children(func() { ui.Divider(f) })
					}
					ui.Text(f, group.Name).FillWidth().PadSides(0, subagentProviderPadX, 0, subagentProviderPadX).
						FontSize(subagentProviderFont).LineHeight(subagentNoticeLine).Weight(gfx.WeightMedium).Color(t.LabelTertiary)
					for _, candidate := range group.Candidates {
						p.drawSettingsSubagentModelCandidate(f, env, t, card, candidate)
					}
				})
			}
		})
	})
}

func (p *Plugin) drawSettingsSubagentModelCandidate(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	card settingsItemCardState, candidate settingsSubagentModelCandidate) {
	disabled := !card.Writable || card.Saving
	row := ui.Row(f).FillWidth().Items(ui.Center).Gap(subagentModelGap).
		PadSides(subagentModelPad, subagentModelPad, subagentModelPad, subagentModelPad).
		Radius(t.Radii.MD).Hover(ui.Look{}.Background(t.Hover)).
		Hit(env.Tree.ID(env.Feat, "settings-subagent-model-"+candidate.Key), candidate.ModelName)
	row.Children(func() {
		drawSettingsSubagentCheckbox(f, t, candidate.Selected, disabled)
		ui.Column(f).Grow(1).MinWidth(0).Children(func() {
			ui.Text(f, candidate.ModelName).FillWidth().FontSize(subagentModelNameFont).
				LineHeight(subagentToggleLine).Color(t.LabelPrimary).Ellipsis()
			route := candidate.ProviderName + " · " + candidate.Provider + "/" + candidate.Model
			ui.Text(f, route).FillWidth().MarginSides(subagentModelRouteGap, 0, 0, 0).
				FontSize(subagentModelRouteFont).LineHeight(subagentNoticeLine).Color(t.LabelTertiary).Ellipsis()
		})
		if !candidate.Available {
			ui.Text(f, zhSubagentModelUnavailable).Shrink(0).FontSize(subagentModelRouteFont).
				LineHeight(subagentNoticeLine).Color(t.LabelTertiary)
		}
	})
	if !disabled && row.Clicked() {
		model := card.Model
		if routes, ok := toggleSubagentModel(model.DesiredRoutes, model.Candidates, candidate.Key); ok {
			p.ensureSettingsItemForm().stageSubagentModel(model, model.Enabled, routes)
		}
	}
}

func drawSettingsSubagentCheckbox(f ui.C, t *pluginapi.Theme, checked, disabled bool) {
	box := ui.Row(f).Size(subagentCheckboxSize, subagentCheckboxSize).Shrink(0).
		Items(ui.Center).Justify(ui.Center).Radius(t.Radii.SM)
	if checked {
		box = box.Background(t.BrandPrimary)
	} else {
		border := t.BorderL3
		if disabled {
			border = t.BorderL1
		}
		box = box.Border(1, border)
	}
	box.Children(func() {
		if !checked {
			return
		}
		if commands, ok := dsassets.CheckOutline(); ok {
			ui.Icon(f, gfx.Icon{ViewBox: 16, Stroke: 1.5, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: commands}).
				Size(subagentCheckboxSize-2, subagentCheckboxSize-2).Color(t.ButtonPrimaryFg)
		}
	})
}
