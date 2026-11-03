// Package dscomposer 是「输入区」这个官方部件的移植。
//
// 官方对应物是 `dsh-client-ui-chat` 里的 composer（类名 `.LOmM4q_panel`）：
// 一个圆角大输入框 + 右下角发送键，按 Enter 直接发。
//
// 它自己持有 flex.Field，并实现 pluginapi.Fields —— 不实现的话壳不会把这个
// Field 交给 viva 的 TextInput，键盘就一个字都收不到。
package dscomposer

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
)

// Plugin 实现 pluginapi.Plugin / Panels / Fields。
type Plugin struct {
	theme *pluginapi.Theme
	// field 是值类型：用 &p.field 取地址。写成 *flex.Field 又忘了 new，
	// Fields() 就会返回一个 nil 元素，flex.Fields(...) 里直接 panic（踩过）。
	prims pluginapi.Primitives
	field flex.Field
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

// draw 画一行：输入框（占满）+ 发送键。
//
// 几何照官方：外层圆角 LG(16)，底色 InputMajor，描边 BorderL2；
// 高度靠内边距撑（不要写死 Height，见 docs/plugin-api.md §7 坑 1）。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	// 一个大圆角面板把输入框和发送键包在一起（官方 composer 就是这样一整块）。
	f.Column().WidthPercent(100).Radius(t.Radii.LG).
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

var (
	_ pluginapi.Plugin = (*Plugin)(nil)
	_ pluginapi.Panels = (*Plugin)(nil)
	_ pluginapi.Fields = (*Plugin)(nil)
)
