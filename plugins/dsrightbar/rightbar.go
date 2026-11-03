// Package dsrightbar 是「右栏工具调用轨迹」的移植。
//
// 官方对应物是 `dsh-client-ui-plan` / `dsh-client-ui-goal` 那一列（任务板 + 树状图）。
// 我们的宿主侧 P0 还没做 plan/goal，所以这一版只画**真的有的数据**：
// 当前会话里发生过的工具调用，按顺序排成一条竖向时间线。
//
// 克制的地方：没有工具调用时只画一句提示，不画一条空线；
// 不显示「耗时」「token」这类宿主还没给的数字。
package dsrightbar

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
)

// Plugin 实现 pluginapi.Plugin / Panels。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-rightbar" }

// Provides 实现 pluginapi.Plugin。
func (*Plugin) Provides() []string { return nil }

// Requires 实现 pluginapi.Plugin。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

// Register 实现 pluginapi.Plugin。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	return nil
}

// Panels 实现 pluginapi.Panels。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "timeline", Slot: pluginapi.SlotRightbar, Order: 10, Draw: p.draw},
	}
}

// draw 画标题 + 时间线。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme

	f.Row().WidthPercent(100).Items(flex.Center).Children(func() {
		f.Text("轨迹").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
	})
	p.prims.Divider(f)

	tools := 0
	for i := range env.Data.Msgs {
		if env.Data.Msgs[i].Role == "tool" {
			tools++
		}
	}
	if tools == 0 {
		f.Column().Pad(8).Children(func() {
			f.Text("本会话还没有工具调用").FontSize(t.Fonts.Caption).Color(t.LabelCaption)
		})
		return
	}

	// 内容是动态的，必须能滚。宽度用百分比——但注意槽位里百分比可能按整窗解析，
	// 所以上一级的 Column 已经由壳钉死了 300px 宽，这里的 100% 才有意义。
	scrID := env.Tree.ID(env.Feat, "rightbar-scroll")
	f.ScrollArea(scrID).WidthPercent(100).Grow(1).MinHeight(120).Gap(2).Clip().Children(func() {
		f.Column().WidthPercent(100).Gap(2).Children(func() {
			for i := range env.Data.Msgs {
				if env.Data.Msgs[i].Role == "tool" {
					p.drawEntry(f, env.Data.Msgs[i])
				}
			}
		})
	})
}

// drawEntry 画一条：左边状态点，右边两行（工具名 + 摘要），错误时带徽标。
func (p *Plugin) drawEntry(f *flex.Frame, msg apiclient.Message) {
	t := p.theme

	name := msg.Tool
	if name == "" {
		name = "工具"
	}
	badge := ""
	sum := msg.Summary
	if msg.View != nil {
		if msg.View.Badge != "" {
			badge = msg.View.Badge
		}
		if msg.View.Title != "" && name == "" {
			name = msg.View.Title
		}
	}
	if sum == "" {
		sum = msg.Args
	}

	f.Row().WidthPercent(100).PadY(4).Gap(8).Items(flex.Start).Children(func() {
		// 左：状态点（错误是红的，其余是链接色）
		color := t.Link
		if badge == "error" {
			color = t.Error
		}
		f.Column().Width(8).Children(func() {
			p.prims.Dot(f, color, 6)
		})
		// 右：两行
		f.Column().Grow(1).Gap(1).Children(func() {
			f.Text(name).Mono().FontSize(t.Fonts.Caption).
				Color(t.LabelSecondary).Ellipsis()
			if s := summarize(sum, 60); s != "" {
				f.Text(s).FontSize(t.Fonts.Tiny).Color(t.LabelCaption).Ellipsis()
			}
		})
	})
}

// summarize 压成一行并截断（时间线只有 300px，长命令不能铺开）。
func summarize(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

var (
	_ pluginapi.Plugin = (*Plugin)(nil)
	_ pluginapi.Panels = (*Plugin)(nil)
)
