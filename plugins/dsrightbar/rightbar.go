// Package dsrightbar 是「右栏工具调用轨迹」的移植。
//
// 官方对应物是 `dsh-client-ui-plan` / `dsh-client-ui-goal` 那一列（任务板 + 树状图）。
// 我们的宿主侧 P0 还没做 plan/goal，所以这一版只画**真的有的数据**：
// 当前会话里发生过的工具调用，按发生顺序排成一条竖向时间线——点、线、点，
// 一眼看出「跑到第几步、哪一步炸了」。
//
// 克制的地方：没有工具调用时只画一句提示，**不画一条空线**；
// 「耗时」「token」这类宿主还没给的数字一律不显示。
package dsrightbar

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// maxEntries 是时间线最多画多少条。
//
// 真实会话里有 3000+ 次工具调用（实测一个 31 MB 的会话 3020 次）。
// 全画出来既没人看得过来，也会把这一列变成几千个节点。只画最近的，
// 并在顶上如实写出「更早的还有多少条」——截断必须让人看得见。
const maxEntries = 80

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

// draw 画整条右栏：标题 + 分隔线 + 时间线（可滚）。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	calls := toolCalls(env.Data.Msgs)
	hidden := 0
	if len(calls) > maxEntries {
		hidden = len(calls) - maxEntries
		calls = calls[len(calls)-maxEntries:]
	}

	// 壳的右栏轨道已有 12px inset；本层只负责内容轨道。
	// 空态再补 12px，合计官方 guide 的 24px；标题/时间线直接使用壳的 12px。
	f.Column().WidthPercent(100).FillHeight().Children(func() {
		// ── 标题：轨迹 + 条数 ──
		f.Row().WidthPercent(100).Items(flex.Center).Gap(6).Children(func() {
			f.Text("轨迹").FontSize(t.Fonts.Secondary).LineHeight(t.Fonts.SecondaryLine).Color(t.LabelTertiary)
			f.Spacer()
			if n := hidden + len(calls); n > 0 {
				f.Text(itoa(n) + " 次").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
			}
		})
		p.prims.Divider(f)

		// ── 空态：官方 guide 在可用 body 内居中，padding 0 24px，secondary/tertiary ──
		if len(calls) == 0 {
			msg := "本会话还没有工具调用"
			if env.Data.Cur == "" {
				msg = "先选一个会话"
			}
			f.Column().WidthPercent(100).Grow(1).Justify(flex.Center).Items(flex.Center).PadX(12).Children(func() {
				f.Text(msg).FontSize(t.Fonts.Secondary).LineHeight(t.Fonts.SecondaryLine).
					Color(t.LabelTertiary).TextCenter()
			})
			return
		}

		// ── 时间线本体 ──
		// 宽度一律用百分比：槽位里 f.Bounds() 拿到的是整窗宽，
		// 按它算会把内容画到窗口外面去。
		scrID := env.Tree.ID(env.Feat, "rightbar-scroll")
		f.ScrollArea(scrID).WidthPercent(100).Grow(1).MinHeight(80).Gap(2).Clip().Children(func() {
			f.Column().WidthPercent(100).Gap(2).Children(func() {
				if hidden > 0 {
					f.Text("… 更早的 " + itoa(hidden) + " 次未显示").
						FontSize(t.Fonts.Tiny).Color(t.LabelCaption).PadY(2)
				}
				for i := range calls {
					p.drawEntry(f, calls[i], i == len(calls)-1)
				}
			})
		})
	})
}

// drawEntry 画时间线上的一条：左边「点 + 竖线」，右边两行文字。
//
// 竖线的画法：gutter 是一个竖着的 Column，里面先放一个点，再放一个
// `Width(2) + Grow(1)` 的方块——Grow(1) 在 Column 里就是「吃掉剩下的高度」，
// 于是它正好从点下面连到下一条的入口。最后一条不画线（线不能凭空延长）。
func (p *Plugin) drawEntry(f *flex.Frame, msg apiclient.Message, last bool) {
	t := p.theme
	name, detail, failed := describe(msg)

	// 行高用 MinHeight + PadY 撑，不要写 Height(...).Shrink(0)：
	// 那组合会让整行塌成 0 高、一根线都不画（踩过，见 docs/plugin-api.md §7）。
	f.Row().WidthPercent(100).Gap(8).PadY(4).Children(func() {
		f.Column().Width(14).Items(flex.Center).Children(func() {
			p.prims.Dot(f, dotColor(t, failed, msg.View), 6)
			if !last {
				f.Column().Width(2).Grow(1).Background(t.BorderL2)
			}
		})

		f.Column().Grow(1).Gap(2).Children(func() {
			f.Row().WidthPercent(100).Gap(6).Items(flex.Center).Children(func() {
				f.Text(name).Mono().FontSize(t.Fonts.Caption).
					Color(t.LabelSecondary).Ellipsis().Grow(1)
				if failed {
					p.prims.Badge(f, "error", t.Error)
				}
			})
			if s := oneLine(detail, 72); s != "" {
				f.Text(s).FontSize(t.Fonts.Tiny).Color(t.LabelCaption).Ellipsis()
			}
		})
	})
}

// toolCalls 按顺序取出工具调用。
func toolCalls(msgs []apiclient.Message) []apiclient.Message {
	out := make([]apiclient.Message, 0, len(msgs)/3)
	for i := range msgs {
		if msgs[i].Role == "tool" {
			out = append(out, msgs[i])
		}
	}
	return out
}

// describe 决定一条的标题、摘要与「是不是失败」。
//
// 摘要优先级按显示契约来：View 的声明 > Summary > Args。
// 但 View.Title 通常就等于工具名，原样拼上去只会得到「bash / bash」这种废话，
// 所以**标题与工具名相同时视为没提供新信息**，继续往下取。
func describe(msg apiclient.Message) (name, detail string, failed bool) {
	name = msg.Tool
	if name == "" {
		name = "工具"
	}

	var viewTitle, viewBadge string
	if v := msg.View; v != nil {
		viewTitle, viewBadge = v.Title, v.Badge
	}
	failed = viewBadge == "error"

	switch {
	case viewTitle != "" && viewTitle != msg.Tool:
		detail = viewTitle
	case msg.Summary != "":
		detail = msg.Summary
	default:
		detail = msg.Args
	}
	return name, detail, failed
}

// dotColor 决定圆点颜色：失败红、声明 ok 绿、没声明灰。
func dotColor(t *pluginapi.Theme, failed bool, view *apiclient.ViewSpec) ui.Color {
	if failed {
		return t.Error
	}
	if view != nil && view.Badge == "ok" {
		return t.Success
	}
	return t.LabelDimmed
}

// oneLine 压成一行并截断（这一列只有两百多像素，长命令不能铺开）。
func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// itoa 让这个包不依赖 strconv（与 ds-sidebar / ds-statusbar 保持一致）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 && i > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

var (
	_ pluginapi.Plugin = (*Plugin)(nil)
	_ pluginapi.Panels = (*Plugin)(nil)
)
