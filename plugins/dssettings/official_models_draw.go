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
	if write.Kind == WriteSaving && strings.HasPrefix(row.ID, "retry-") {
		row.Disabled = true
		row.Reason = "保存中…"
	}
	switch {
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
	if strings.HasPrefix(row.ID, "provider-edit-") {
		route, err := modelRouteFromRow(row)
		if err != nil {
			p.setWriteState(ClassifyWriteError(row.NS, row.Path, err))
			return true
		}
		p.toggleModelsEditor(route)
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
		p.setWriteState(WriteState{Kind: WriteIdle})
		return true
	case "retry-" + route + "-apply":
		p.setWriteState(WriteState{Kind: WriteSaving, NS: apiclient.SettingsNSLLM, Path: row.Path})
		go func() { _ = p.applyModelsRetry() }()
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

func (p *Plugin) toggleModelsEditor(route string) {
	o := p.ensureOfficial()
	o.mu.Lock()
	if o.modelsEdit != nil && o.modelsEdit.route == route {
		o.modelsEdit = nil
		o.mu.Unlock()
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
		o.modelsEdit = edit
	}
	o.mu.Unlock()
	if err != nil {
		p.setWriteState(ClassifyWriteError(apiclient.SettingsNSLLM, []string{"providers", route, "retryPolicy"}, err))
		return
	}
	p.setWriteState(WriteState{Kind: WriteIdle})
}

// applyModelsRetry 是唯一的 retryPolicy 写出口：一次 Apply、一次原子 mutate、
// expectedRevision 取开卡时的 namespace.revision。失败不关卡，草稿仍在。
func (p *Plugin) applyModelsRetry() WriteState {
	o := p.ensureOfficial()
	o.mu.Lock()
	edit := o.modelsEdit.clone()
	writer := o.writer
	o.mu.Unlock()
	if edit == nil {
		state := WriteState{Kind: WriteInvalid, Code: apiclient.SettingsCodeBadRequest, Message: "重试策略编辑卡已关闭"}
		p.setWriteState(state)
		return state
	}
	if err := edit.validate(); err != nil {
		state := WriteState{Kind: WriteInvalid, NS: apiclient.SettingsNSLLM,
			Path: []string{"providers", edit.route, "retryPolicy"}, Code: apiclient.SettingsCodeBadRequest, Message: err.Error()}
		p.setWriteState(state)
		return state
	}
	ops := edit.ops()
	if len(ops) == 0 {
		o.mu.Lock()
		if o.modelsEdit != nil && o.modelsEdit.generation == edit.generation {
			o.modelsEdit = nil
		}
		o.mu.Unlock()
		state := WriteState{Kind: WriteIdle}
		p.setWriteState(state)
		return state
	}
	if writer == nil {
		state := ClassifyWriteError(apiclient.SettingsNSLLM, ops[0].Path, errNoSettingsWriter)
		p.setWriteState(state)
		return state
	}
	ctx, cancel := context.WithTimeout(context.Background(), panelWriteTimeout)
	defer cancel()
	state := ApplyMutateRevision(ctx, writer, apiclient.SettingsNSLLM, ops, &edit.revision)
	if state.Success() {
		o.mu.Lock()
		if o.modelsEdit != nil && o.modelsEdit.generation == edit.generation {
			o.modelsEdit = nil
		}
		o.mu.Unlock()
	}
	p.setWriteState(state)
	return state
}
