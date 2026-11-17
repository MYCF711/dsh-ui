package dstool

import (
	"fmt"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// 这个文件画宿主声明的结构化结果（apiclient.ViewSpec）。
//
// 它和"按工具类型分派"是两条互补的路：
//
//	宿主声明了 View  → 照宿主说的画（card / table / plain），界面不猜
//	宿主没声明       → 按 Card.Tool 走 renderers 里的定制画法
//
// 顺序不能反：View 是宿主对自己结果的**直接声明**，比界面从参数里推出来的
// 摘要准。不认识 Kind 时退化成正文——"不认识 == 没声明"，不是"猜一个"。
//
// 几何全部走官方两套卡（出处见 official_tool.go 表头）：
//
//	card          → `ToolDetails.root` + `.item` 10/14 + `.field` 5.5em/12px
//	plain         → `ioCard` + `ioSection` 12/16，正文是 `.ioText` 11/16
//	table         → 官方没有这个 Kind（宿主自造形状），只借外壳，不自造新数值
//
// 面板里的字号是官方 `--dsw-font-xs-13`（13px/20px，无衬线）：标签 `.field dt`
// 是 label-caption，数值 `.field dd` 是 label-secondary（ToolDetails.module.css:7,143-151）。

// drawView 按 ViewSpec 的 Kind 选渲染器。
func (s *Cards) drawView(f ui.C, env *pluginapi.Env, v *apiclient.ViewSpec) {
	t := env.Theme

	// 头：工具名 + 徽标。它不是官方卡的一部分（官方卡头在卡内），所以只做
	// 一件事：左边缘对齐到卡的 4px 缩进，别让头浮在卡外面。
	if v.Title != "" || v.Badge != "" {
		ui.Row(f).WidthPercent(100).Gap(8).Items(ui.Center).
			MarginSides(0, 0, 0, toolCardMarginH).Children(func() {
			ui.Text(f, v.Title).Mono().Bold().FontSize(t.Fonts.Secondary).
				LineHeight(toolFieldLine).Color(t.LabelSecondary)
			ui.Spacer(f)
			if v.Badge != "" {
				// Badge 原文照画，但只有明确的 ok 是绿色、error 是红色；
				// preparing/running/stopped/耗时等不能被统一涂成“成功”。
				s.badge(f, v.Badge, statusColor(t, Status(v.Badge)))
			}
		})
	}

	switch v.Kind {
	case "table":
		s.drawTable(f, env, v)
	case "plain":
		// 纯文本工具结果只给一眼：12 行会把一条工具卡撑满半屏（实测过），
		// 想看全文应该点开或去 artifacts，而不是在消息流里翻。
		drawToolCard(f, t, func(f ui.C) {
			drawIOSection(f, t, func(f ui.C) {
				ui.Text(f, v.Body).Mono().FontSize(t.Fonts.Tiny).
					LineHeight(toolCodeLine).Color(t.LabelSecondary).MaxLines(6)
			})
		})
	case "card", "":
		s.drawCard(f, env, v)
	default:
		// 不认识的 Kind：退化成正文，别丢内容也别崩。
		ui.Text(f, v.Body).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
	}
}

// drawCard 是 Kind=card：一组键值，几何 = 官方参数面板。
func (s *Cards) drawCard(f ui.C, env *pluginapi.Env, v *apiclient.ViewSpec) {
	if len(v.Fields) == 0 {
		return
	}
	t := env.Theme
	drawToolCard(f, t, func(f ui.C) {
		drawDetailsItem(f, t, func(f ui.C) {
			ui.Column(f).FillWidth().Gap(toolFieldRowGap).Children(func() {
				for _, fv := range v.Fields {
					drawField(f, env, fv.Key, fv.Value)
				}
			})
		})
	})
}

// drawTable 是 Kind=table：列头 + 行。
//
// 行宽必须等于列数；不等就截断到列数（**不许 panic**：这是外部数据）。
//
// 官方 `ToolDetails` 没有表格式，所以这里**只借外壳**（`ToolDetails.root`
// 的外框与 `.item` 的 10/14 内边距），列内字号沿用面板的 13/20；
// 列宽平分可用宽，不引入任何"看起来像官方"的新数值。
func (s *Cards) drawTable(f ui.C, env *pluginapi.Env, v *apiclient.ViewSpec) {
	t := env.Theme
	if len(v.Columns) == 0 {
		return
	}
	inner := contentWidth(env) - toolCardMarginH - 2*toolFieldPadH
	cellWidth := inner / float32(len(v.Columns))
	const maxRows = 8
	drawToolCard(f, t, func(f ui.C) {
		drawDetailsItem(f, t, func(f ui.C) {
			ui.Row(f).FillWidth().Gap(toolFieldGap).Children(func() {
				for _, c := range v.Columns {
					ui.Text(f, c).FontSize(t.Fonts.Secondary).LineHeight(toolFieldLine).
						Color(t.LabelCaption).MaxWidth(cellWidth).Ellipsis()
				}
			})
			for r := 0; r < len(v.Rows) && r < maxRows; r++ {
				row := v.Rows[r]
				ui.Row(f).FillWidth().Gap(toolFieldGap).Children(func() {
					for c := range v.Columns {
						cell := ""
						if c < len(row) {
							cell = row[c]
						}
						ui.Text(f, cell).Mono().FontSize(t.Fonts.Secondary).
							LineHeight(toolFieldLine).Color(t.LabelTertiary).
							MaxWidth(cellWidth).Ellipsis()
					}
				})
			}
			if len(v.Rows) > maxRows {
				ui.Text(f, fmt.Sprintf("… 另有 %d 行", len(v.Rows)-maxRows)).
					FontSize(toolSubtitleSize).LineHeight(toolFieldLine).Color(t.LabelCaption)
			}
		})
	})
}
