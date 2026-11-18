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

// fontStepperStep 是一次点击的方向（0 = 没点）。
type fontStepperStep int

const (
	fontStepperNone fontStepperStep = 0
	// fontStepperUp 是上方 chevron（rotate 180 的那枚，官方先画）。
	fontStepperUp fontStepperStep = 1
	// fontStepperDown 是下方 chevron。
	fontStepperDown fontStepperStep = -1
)

// fontStepperIDs 是字号 stepper 的三个命中区。
//
// 三个 id 必须**互不相同**：viva 的 `Box.Clicked()` 按 hit id 判定，两枚箭头
// 共用一个 id 时"点 ↑"会同时点亮两枚按钮、后判的那枚把 step 覆盖成 −1——
// 实测点增大反而把 14 写成了 13。官方两枚 button 是各自独立的节点
// （FontSizeRow.tsx:69-93），各自 onClick 只带自己的 ±1。
type fontStepperIDs struct {
	pill, up, down gfx.ID
}

// drawFontStepper 画 72×36 胶囊、居中数值、右侧 17×12 双 chevron 与外部 px 单位
// （FontSizeRow.module.css 的 ._stepper / ._value / ._arrows / ._arrow / ._unit）。
//
// 显隐照官方两条规则：`._arrows{opacity:0}` 默认不出现，
// `._stepper:hover ._arrows{opacity:1}` 指针进入胶囊才出现。
//
// viva 没有「父级 hover 驱动子级样式」的通道，只能按本帧 hover 判定：
// `Canvas.Hovered()` 是 hover 目标的读口（viva components/accordion.go:62 的
// `hovered := accordion.canvas.Hovered() == id`）。判定键是**胶囊自己的命中区**，
// 不是整行——官方是 `._stepper:hover`，指针停在标题文字上不该让箭头冒出来。
// hover 目标变化本身就会触发重绘（app/painter.go:325-328），所以不需要额外 invalidate。
//
// `readable` 为 false（值未知 / 纯绘制）时不挂箭头命中区，也绝不把点击咽下去。
func drawFontStepper(f ui.C, t *pluginapi.Theme, ids fontStepperIDs, title, value string, readable bool) fontStepperStep {
	step := fontStepperNone
	hovered := f.Canvas().Hovered()
	// 指针落在箭头上时（上一帧已显形，命中目标换成箭头 id）也要继续显形，
	// 否则箭头会在胶囊与箭头之间来回闪。
	showArrows := ids.pill != (gfx.ID{}) &&
		(hovered == ids.pill || hovered == ids.up || hovered == ids.down)
	ui.Row(f).Height(SelectorHeight).Gap(StepperControlGap).Items(ui.Center).Children(func() {
		ui.Row(f).Width(StepperMinWidth).Height(SelectorHeight).Radius(t.Radii.MD).
			Background(t.BgLayer3).Hit(ids.pill, title).Children(func() {
			ui.Row(f).Absolute().Inset(0).Items(ui.Center).Justify(ui.Center).Children(func() {
				ui.Text(f, value).MinWidth(StepperValueMinWidth).FontSize(RowTitleFontSize).
					LineHeight(RowTitleLineHeight).TextCenter().Color(t.LabelPrimary)
			})
			if showArrows {
				ui.Column(f).Absolute().Right(StepperArrowRight).Top(StepperArrowTop).
					Width(StepperArrowWidth).Gap(StepperArrowGap).Children(func() {
					for _, entry := range []struct {
						rotate float32
						step   fontStepperStep
						id     gfx.ID
						label  string
					}{
						{180, fontStepperUp, ids.up, "增大"},
						{0, fontStepperDown, ids.down, "减小"},
					} {
						arrow := ui.Row(f).Width(StepperArrowWidth).Height(StepperArrowHeight).Radius(t.Radii.XS).
							Items(ui.Center).Justify(ui.Center)
						if readable {
							arrow = arrow.Hit(entry.id, title+" "+entry.label)
						}
						arrow.Children(func() {
							ui.Icon(f, chevronOutlineIcon()).Size(StepperArrowIcon, StepperArrowIcon).
								IconTransform(gfx.IconTransform{Rotate: entry.rotate}).Color(t.LabelPrimary)
						})
						if readable && arrow.Clicked() {
							step = entry.step
						}
					}
				})
			}
		})
		ui.Text(f, "px").FontSize(StepperUnitFontSize).LineHeight(StepperUnitLine).Color(t.LabelSecondary)
	})
	return step
}

// settingsModuleFill 是官方 bg-module-platform：浅色 neutral-bluish-60
// rgb(245,246,247)（design-platform.css:68,189），暗色 neutral-bluish-800（:341）。
func settingsModuleFill(t *pluginapi.Theme) ui.Color {
	if t.Dark {
		return t.BgLayer3
	}
	return gfx.RGB24(0xf5f6f7)
}
