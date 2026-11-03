// Package dsstatusbar 是「底部状态栏」的移植。
//
// 官方对应物是 `dsh-client-ui-layout` 里那一行状态条（右侧显示会话名 / 轮次 /
// token 速率）。我们先把**能真算出来的**放上去：轮数、步数、消息条数、壳给的提示、
// 以及一个就绪/待审批的状态点。
//
// 明确的克制：token 速率和「199M tok」这类数字宿主侧还没给，**这里就不编**。
// 界面上出现一个假的 tok/s 比不显示它糟糕得多。
package dsstatusbar

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
)

// Plugin 实现 pluginapi.Plugin / Panels。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-statusbar" }

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
		{ID: "bar", Slot: pluginapi.SlotStatusbar, Order: 10, Draw: p.draw},
	}
}

// draw 画一整条状态栏。数据全部从快照现算，没有第二份真相。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme

	turns, steps := 0, 0
	for _, msg := range env.Data.Msgs {
		switch msg.Role {
		case "assistant":
			turns++
		case "tool":
			steps++
		}
	}

	// 状态点：待审批时是警告色，否则就绪。
	dotColor, dotLabel := t.Success, "就绪"
	if env.Data.Approval != nil {
		dotColor, dotLabel = t.Warn, "待审批"
	}

	// 上边一条最轻的分隔线：状态栏要和上面的内容分开，但不该用重描边。
	// 用 prims 而不是自己 f.Row().Height(1).Background(...)：形状一律问根本插件。
	p.prims.Divider(f)

	f.Row().FillWidth().FillHeight().Background(t.SidebarFill).
		PadX(12).Items(flex.Center).Gap(10).Children(func() {
		if turns+steps > 0 {
			f.Text(itoa(turns) + " 轮 · " + itoa(steps) + " 步").
				FontSize(t.Fonts.Caption).Color(t.LabelCaption)
			f.Text("·").FontSize(t.Fonts.Caption).Color(t.LabelDimmed)
			f.Text("共 " + itoa(env.Data.Total) + " 条").
				FontSize(t.Fonts.Caption).Color(t.LabelCaption)
		}
		f.Spacer()
		if env.Data.Status != "" {
			// 壳给的提示（连接中 / 已拒绝 / 渲染 N 条…）。太长就省略，
			// 状态栏不该抢主区的注意力。
			f.Text(env.Data.Status).FontSize(t.Fonts.Caption).
				Color(t.LabelTertiary).Ellipsis().MaxWidth(520)
		}
		f.Row().Gap(6).Items(flex.Center).Children(func() {
			p.prims.Dot(f, dotColor, 6)
			f.Text(dotLabel).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		})
	})
}

// itoa 避免为一个小数字 import strconv，让这个包只剩 flex + 契约两个依赖。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 && i > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg && i > 0 {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

var (
	_ pluginapi.Plugin = (*Plugin)(nil)
	_ pluginapi.Panels = (*Plugin)(nil)
)
