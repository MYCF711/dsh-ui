// Package dscomposer 是「输入区」这个官方部件的移植。
//
// 官方对应物是 `dsh-client-ui-chat` 里的 composer（类名 `.LOmM4q_panel`）：
// 一个圆角大输入框 + 右下角发送键，按 Enter 直接发。
//
// 它自己持有 flex.Field，并实现 pluginapi.Fields —— 不实现的话壳不会把这个
// Field 交给 viva 的 TextInput，键盘就一个字都收不到。
//
// 它还画审批条（见 approval.go）：待审批的工具调用画在输入框正上方，
// 因为这里就是"用户该做决定的地方"。
package dscomposer

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// Plugin 实现 pluginapi.Plugin / Panels / Fields / KeyHandler。
type Plugin struct {
	theme *pluginapi.Theme
	// field 是值类型：用 &p.field 取地址。写成 *flex.Field 又忘了 new，
	// Fields() 就会返回一个 nil 元素，flex.Fields(...) 里直接 panic（踩过）。
	prims pluginapi.Primitives
	field flex.Field

	// lastData / lastAct 是绘制期快照，按键时要用。
	// HandleKey 没有 env 参数，只能记住上一帧（与 ds-conversation 同一做法）。
	// 只读、不跨帧持有可变状态：壳每帧换一份新快照。
	lastData pluginapi.Snapshot
	lastAct  pluginapi.Actions

	// logf 是插件的日志出口（注册期拿到的 Context.Logf）。
	// 只用来记"用户点了哪一项"，一行一次，不进绘制热路径。
	logf func(format string, args ...any)
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-composer" }

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
	p.logf = ctx.Logf
	return nil
}

// Fields 实现 pluginapi.Fields：壳靠它把输入框登记给 viva。
func (p *Plugin) Fields() []*flex.Field { return []*flex.Field{&p.field} }

// Panels 实现 pluginapi.Panels。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "composer", Slot: pluginapi.SlotMainComposer, Order: 10, Draw: p.draw},
	}
}

// draw 画一列：审批条（有才画）+ 输入框那一行。
//
// 几何照官方：外层圆角 LG(16)，底色 InputMajor，描边 BorderL2；
// 高度靠内边距撑（不要写死 Height，见 docs/plugin-api.md §7 坑 1）。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	p.lastData = env.Data
	p.lastAct = env.Act
	f.Column().WidthPercent(100).Gap(8).Children(func() {
		// 审批条：Approval == nil 时这里什么都不画（不留空框）。
		p.drawApproval(f, env)
		// 一个大圆角面板把输入框和发送键包在一起（官方 composer 就是这样一整块）。
		f.Column().FillWidth().Radius(t.Radii.LG).
			Border(1, t.BorderL2).Background(t.InputMajor).
			PadX(12).PadY(10).Gap(6).Children(func() {
			field := f.Field(&p.field, env.Tree.ID(env.Feat, "composer"), "输入", "说点什么…（Enter 发送）").
				FillWidth().FontSize(t.Fonts.Body)
			if field.Submitted() {
				p.submit(env)
			}
			// 底部一行：左边留给以后的模型/权限 chip（宿主还没这几个字段），右边发送。
			f.Row().WidthPercent(100).Items(flex.Center).Children(func() {
				f.Spacer()
				if p.prims.CircleButton(f, env.Tree.ID(env.Feat, "send"), "↑", true) {
					p.submit(env)
				}
			})
		})
	})
}

// submit 把非空文本交给壳，然后清空自己。
//
// 空文本不提交：让壳去判空等于把"没说话也发了一条"的风险留给上游。
func (p *Plugin) submit(env *pluginapi.Env) {
	text := strings.TrimSpace(p.field.Text())
	if text == "" {
		return
	}
	if env.Act.Send != nil {
		env.Act.Send(text)
	}
	p.field.SetText("")
}

// HandleKey 实现 pluginapi.KeyHandler：**只在审批条可见时**接管两个键。
//
//	Enter  允许（回传宿主给的 allow 项）
//	Esc    拒绝（同理）
//
// 审批条不在（Approval == nil）就直接返回 false —— 输入框的 Enter 发送
// 不能被抢掉：viva 的分发顺序是「先 OnKey → 没被处理才交给聚焦控件」，
// 这里返回 true 就等于把 Enter 从 TextInput 手里拿走，所以门槛必须卡在
// approvalVisible 上（见 approval_test.go 的 TestHandleKeyTakesEnterOnlyWhenVisible）。
func (p *Plugin) HandleKey(ev ui.KeyEvent) bool {
	if ev.Release || ev.Modifiers != 0 {
		return false
	}
	if !approvalVisible(p.lastData) {
		return false
	}
	plan := planApproval(p.lastData, p.lastAct.Approve != nil)
	switch ev.Key {
	case ui.KeyEnter:
		if !plan.Allow.Usable {
			return false
		}
		p.decideKey(true)
		return true
	case ui.KeyEscape:
		if !plan.Deny.Usable {
			return false
		}
		p.decideKey(false)
		return true
	}
	return false
}

// decideKey 是按键路径的出入口：与按钮走同一个 decide()，只是没有 env。
func (p *Plugin) decideKey(allow bool) {
	ap := p.lastData.Approval
	if !decide(ap, p.lastAct, allow) {
		return
	}
	if p.logf != nil {
		opt, _ := chooseOption(ap, allow)
		verdict := "拒绝"
		if allow {
			verdict = "允许"
		}
		p.logf("审批（键盘）：%s → %s", verdict, opt.OptionID)
	}
}

var (
	_ pluginapi.Plugin     = (*Plugin)(nil)
	_ pluginapi.Panels     = (*Plugin)(nil)
	_ pluginapi.Fields     = (*Plugin)(nil)
	_ pluginapi.KeyHandler = (*Plugin)(nil)
)
