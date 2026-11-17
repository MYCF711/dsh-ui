// official_general_draw.go —— general 中不能由普通行尾控件表达的官方布局。
package dssettings

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// drawAppearanceRow 按 AppearanceRow.module.css:4-46 画“标题在上、三个 cube 在下”。
// 每个 cube 是 20px 纵 padding + 16px 图标 + 4px gap + 22px 标签 = 82px。
func (p *Plugin) drawAppearanceRow(f ui.C, t *pluginapi.Theme, row PanelRow, id gfx.ID) {
	box := ui.Column(f).FillWidth().Height(AppearanceRowHeight).
		PadSides(RowPadY, 0, RowPadY, 0).Gap(AppearanceGroupGap).
		Hit(id, row.Title).Children(func() {
		ui.Text(f, row.Title).FontSize(RowTitleFontSize).LineHeight(RowTitleLineHeight).
			Color(t.LabelPrimary)
		ui.Row(f).FillWidth().Height(AppearanceCubeHeight).Gap(AppearanceCubeGap).Children(func() {
			for _, choice := range row.Choices {
				selected := row.Known && row.Selected == choice.Value
				bg := ui.Color{}
				border := t.BorderL4
				if selected {
					bg = settingsModuleFill(t)
					border = t.BrandPrimary
				}
				ui.Column(f).Grow(1).Basis(AppearanceCubeMinWidth).Height(AppearanceCubeHeight).
					PadSides(AppearanceCubePadY, AppearanceCubePadX, AppearanceCubePadY, AppearanceCubePadX).
					Gap(AppearanceCubeTextGap).Items(ui.Center).Justify(ui.Center).
					Border(AppearanceCubeBorder, border).Radius(t.Radii.XL).Background(bg).Children(func() {
					ui.Icon(f, appearanceOutlineIcon(choice.Value)).Size(AppearanceCubeIconSize, AppearanceCubeIconSize).
						Color(t.LabelPrimary)
					ui.Text(f, choice.Label).FontSize(AppearanceCubeFontSize).LineHeight(AppearanceCubeLineHeight).
						Color(t.LabelPrimary)
				})
			}
		})
	})
	if row.Writable() && box.Clicked() {
		p.activateRow(row)
	}
}

// drawFontStepper 按 FontSizeRow.module.css:54-128 画 72×36 stepper、
// 居中数值、右侧 17×12 双 chevron 与外部 px 单位。
func drawFontStepper(f ui.C, t *pluginapi.Theme, value string) {
	ui.Row(f).Height(SelectorHeight).Gap(StepperControlGap).Items(ui.Center).Children(func() {
		ui.Row(f).Width(StepperMinWidth).Height(SelectorHeight).Radius(t.Radii.MD).
			Background(t.BgLayer3).Children(func() {
			ui.Row(f).Absolute().Inset(0).Items(ui.Center).Justify(ui.Center).Children(func() {
				ui.Text(f, value).MinWidth(StepperValueMinWidth).FontSize(RowTitleFontSize).
					LineHeight(RowTitleLineHeight).TextCenter().Color(t.LabelPrimary)
			})
			ui.Column(f).Absolute().Right(StepperArrowRight).Top(StepperArrowTop).
				Width(StepperArrowWidth).Gap(StepperArrowGap).Children(func() {
				for _, rotate := range []float32{180, 0} {
					ui.Row(f).Width(StepperArrowWidth).Height(StepperArrowHeight).Radius(t.Radii.XS).
						Items(ui.Center).Justify(ui.Center).Children(func() {
						ui.Icon(f, chevronOutlineIcon()).Size(StepperArrowIcon, StepperArrowIcon).
							IconTransform(gfx.IconTransform{Rotate: rotate}).Color(t.LabelPrimary)
					})
				}
			})
		})
		ui.Text(f, "px").FontSize(StepperUnitFontSize).LineHeight(StepperUnitLine).Color(t.LabelSecondary)
	})
}

// settingsModuleFill 是官方 bg-module-platform：浅色 neutral-bluish-60
// rgb(245,246,247)（design-platform.css:68,189），暗色 neutral-bluish-800（:341）。
func settingsModuleFill(t *pluginapi.Theme) ui.Color {
	if t.Dark {
		return t.BgLayer3
	}
	return gfx.RGB24(0xf5f6f7)
}
