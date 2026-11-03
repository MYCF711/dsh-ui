// Package dsdemo 是一个**最小第三方插件样例**：不碰壳、不碰内核，
// 只声明「我要什么服务」和「我挂哪个槽位」，接进装配就出现在界面上。
//
// 它挂在 shell.overlay（覆盖整个框架的浮层槽，之前一直没人用），
// 在右上角画一个小提示条，F12 或点「×」收起/展开。
//
// 读这个文件的正确顺序：ID → Requires → Register → Panels → draw。
// 这五段就是第三方插件的全部接口面，其余一律不需要知道。
package dsdemo

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// Plugin 是本样例的全部状态：两个注入来的服务 + 一个「显示还是收起」的开关。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives
	shown bool
}

// ID 必须全局唯一。第三方建议带自己的前缀（这里用 ds-demo）。
func (*Plugin) ID() string { return "ds-demo" }

// Provides 本插件不对别人提供任何能力。
func (*Plugin) Provides() []string { return nil }

// Requires 只说它要什么：颜色令牌 + 基础图元。
//
// 装配器按这张表排顺序——所以 ds-core 一定先就位，Register 里 Inject 不会拿到 nil。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

// Register 在窗口起来之前调用一次，只做注入，不画东西。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	p.shown = true
	return nil
}

// Panels 声明「我挂在哪里、什么时候画」。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "overlay", Slot: pluginapi.SlotShellOverlay, Order: 10, Draw: p.draw},
	}
}

// HandleKey 实现 pluginapi.KeyHandler：F12 切换显隐。
//
// 键盘按装配顺序转发，第一个返回 true 的插件赢；返回 false 表示「这个键我不要」。
func (p *Plugin) HandleKey(ev ui.KeyEvent) bool {
	if ev.Key != ui.KeyF12 || ev.Modifiers != 0 {
		return false
	}
	p.shown = !p.shown
	return true
}

// draw 画右上角那个小浮层。
//
// 三件事是「外部插件最容易写错」的三处，这里都示范了正确写法：
//   - 颜色一律问 theme，不写十六进制；
//   - 不拿 f.Bounds().Width 当列宽（槽位里那是整窗宽），要像素宽度用 env.MainWidth；
//   - 交互 id 用 env.Tree.ID(env.Feat, "名字")，不写死数字。
//
// 注意 Absolute() 必须紧跟在一个新盒子上调用（viva 把它标记成脱离文档流），
// 所以这里先从 f 起一个 Row，再对那个 Row 调 Absolute().Top().Right()。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	if !p.shown {
		return
	}
	t := p.theme

	f.Row().Absolute().Top(16).Right(16).MaxWidth(360).
		Radius(t.Radii.MD).Background(t.BgLayer2).Border(1, t.BorderL2).
		PadX(12).PadY(8).Items(flex.Center).Gap(8).Children(func() {
		f.Text("ds-demo 插件已接入 · 按 F12 收起").
			FontSize(t.Fonts.Caption).Color(t.LabelSecondary).Ellipsis()
		// 关闭按钮：28px 圆钮交给 prims，不自己拼尺寸/颜色。
		if p.prims.CircleButton(f, env.Tree.ID(env.Feat, "demo-close"), "×", false) {
			p.shown = false
		}
	})
}

var (
	_ pluginapi.Plugin     = (*Plugin)(nil)
	_ pluginapi.Panels     = (*Plugin)(nil)
	_ pluginapi.KeyHandler = (*Plugin)(nil)
)
