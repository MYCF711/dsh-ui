package dstool

import (
	"fmt"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
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

// drawView 按 ViewSpec 的 Kind 选渲染器。
func (s *Cards) drawView(f *flex.Frame, env *pluginapi.Env, v *apiclient.ViewSpec) {
	t := env.Theme

	// 头：工具名 + 徽标
	if v.Title != "" || v.Badge != "" {
		f.Row().Gap(8).Items(flex.Center).Children(func() {
			f.Text(v.Title).Mono().Bold().FontSize(t.Fonts.Caption).Color(t.LabelSecondary)
			f.Spacer()
			if v.Badge != "" {
				color := t.Success
				if v.Badge == "error" {
					color = t.Error
				}
				s.badge(f, v.Badge, color)
			}
		})
	}

	switch v.Kind {
	case "table":
		s.drawTable(f, env, v)
	case "plain":
		// 纯文本工具结果只给一眼：12 行会把一条工具卡撑满半屏（实测过），
		// 想看全文应该点开或去 artifacts，而不是在消息流里翻。
		f.Text(v.Body).Mono().FontSize(t.Fonts.Tiny).Color(t.LabelTertiary).MaxLines(6)
	case "card", "":
		s.drawCard(f, env, v)
	default:
		// 不认识的 Kind：退化成正文，别丢内容也别崩。
		f.Text(v.Body).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
	}
}

// drawCard 是 Kind=card：一组键值。
func (s *Cards) drawCard(f *flex.Frame, env *pluginapi.Env, v *apiclient.ViewSpec) {
	t := env.Theme
	cw := contentWidth(env)
	for _, fv := range v.Fields {
		f.Row().WidthPercent(100).Gap(8).Children(func() {
			f.Text(fv.Key).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
			// Value 的 MaxWidth 必须给：不设边界的话 Ellipsis 没有可省略的地方，
			// 长命令会直接画到窗口外（实测：bash 的 command 被切在右边缘）。
			f.Text(fv.Value).Mono().FontSize(t.Fonts.Tiny).Color(t.LabelSecondary).
				MaxWidth(cw - 120).Ellipsis()
		})
	}
}

// drawTable 是 Kind=table：列头 + 行。
//
// 行宽必须等于列数；不等就截断到列数（**不许 panic**：这是外部数据）。
func (s *Cards) drawTable(f *flex.Frame, env *pluginapi.Env, v *apiclient.ViewSpec) {
	t := env.Theme
	cw := contentWidth(env)
	if len(v.Columns) == 0 {
		return
	}
	f.Row().FillWidth().Gap(8).Children(func() {
		for _, c := range v.Columns {
			f.Text(c).FontSize(t.Fonts.Tiny).Color(t.LabelCaption).
				MaxWidth(cw / float32(len(v.Columns))).Ellipsis()
		}
	})
	const maxRows = 8
	for r := 0; r < len(v.Rows) && r < maxRows; r++ {
		row := v.Rows[r]
		f.Row().FillWidth().Gap(8).Children(func() {
			for c := range v.Columns {
				cell := ""
				if c < len(row) {
					cell = row[c]
				}
				f.Text(cell).Mono().FontSize(t.Fonts.Tiny).Color(t.LabelTertiary).
					MaxWidth(cw / float32(len(v.Columns))).Ellipsis()
			}
		})
	}
	if len(v.Rows) > maxRows {
		f.Text(fmt.Sprintf("… 另有 %d 行", len(v.Rows)-maxRows)).
			FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
	}
}
