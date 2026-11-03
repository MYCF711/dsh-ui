// Package dsshortcuts 是一份**可查的键位说明**。
//
// 为什么需要它：键位散落在各插件里（侧栏吃 ↑↓/Home/End、composer 吃 Enter、
// 审批吃 Esc、ds-demo 吃 F12），而界面上**没有任何地方能查到"我还能按什么键"**。
// 用户只能靠读源码或者别人告诉。
//
// 这里做的事很窄：按 F1 弹一块面板列出**真实存在的绑定**，Esc 或再按 F1 收起。
// 不编：表里每一条都在本仓有对应的 `HandleKey`，注释里写了它在哪个文件。
//
// 它同时是「第三方插件只靠 ID/Requires/Register/Panels/HandleKey 就能接入」的第二个样例
// ——第一个是 dsdemo，那个更小，只有 Panels 没有 HandleKey。
package dsshortcuts

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// Binding 是一条键位说明。Where 写的是**这条绑定真实存在于哪个包**，
// 不写"理论支持"——写了就有人会去试，试不通就是骗人。
type Binding struct {
	Key   string
	Does  string
	Where string
}

// bindings 是全部真实键位。改动这里之前先确认对应插件里那行 HandleKey 还在。
var bindings = []Binding{
	{"↑ / ↓", "上下选会话（焦点在侧栏时）", "plugins/dssidebar"},
	{"Home / End", "跳到第一个 / 最后一个会话", "plugins/dssidebar"},
	{"Enter", "发送输入框里的内容", "plugins/dscomposer"},
	{"Esc", "拒绝当前审批请求（reject_once）", "plugins/dscomposer/approval.go"},
	{"Esc", "关闭搜索框（搜索打开时）", "plugins/dsconversation"},
	{"Esc / 停止键", "打断当前正在跑的 turn", "cmd/app（Cancel 接线）"},
	{"F12", "收起 / 展开右上角的插件提示条", "plugins/dsdemo"},
	{"F1", "打开 / 关闭这份键位表", "plugins/dsshortcuts"},
}

// Plugin 只有「显示 / 收起」一个状态。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives
	shown bool
}

func (*Plugin) ID() string { return "ds-shortcuts" }

func (*Plugin) Provides() []string { return nil }

// Requires 与 dsdemo 一致：颜色令牌 + 基础图元。装配器据此保证 ds-core 先就位。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	return nil
}

// Panels 挂在 shell.overlay：它盖住整个框架，正好是弹层该在的地方。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{
		ID:    "shortcuts-sheet",
		Slot:  pluginapi.SlotShellOverlay,
		Order: 95, // 比 dsdemo 的提示条（90）高一档，两者可同时出现时不互相盖住
		Draw:  p.draw,
	}}
}

// HandleKey 实现 pluginapi.KeyHandler（**返回 bool**，不是 EventResult——
// 我第一次写成 EventResult，类型断言过不了，插件装上了却永远收不到键）。
//
// **返回 true 是有代价的**：事件不会再传给后面的插件。所以只在
// ①按 F1（是我的键）②面板开着时按 Esc（先关面板）两种情况下吃掉。
// 面板没开时按 Esc 必须放行——那是审批拒绝 / 取消 turn 的键，被我吞掉就出事了。
func (p *Plugin) HandleKey(ev ui.KeyEvent) bool {
	switch ev.Key {
	case ui.KeyF1:
		p.shown = !p.shown
		return true
	case ui.KeyEscape:
		if p.shown {
			p.shown = false
			return true
		}
	}
	return false
}

// draw 画一张居中卡片：标题 + 逐条键位。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	if !p.shown {
		return
	}
	t := p.theme
	// **不依赖 env.Bounds**：overlay 槽里它不一定是整窗尺寸，算出来是 0 的话
	// 整块面板就退化成 0 宽、等于没画（我第一次就是这么失败又找不到原因的）。
	// 用固定尺寸 + 固定左边距，和 dsdemo 的 Absolute().Top/Right 是同一个思路。
	w := float32(520)
	rowH := float32(26)

	// **必须 Absolute**：overlay 槽给的是一个"自由画"的区域，`FillWidth/FillHeight`
	// 在里面不一定撑满（dsdemo 也是 Absolute + Top/Right）。
	f.Column().Absolute().
		Left(200).Top(96).
		Children(func() {
			f.Column().Width(w).Radius(t.Radii.Panel).
				Background(t.InputMajor).
				Shadow(ui.ShadowXS).
				PadSides(20, 20, 16, 20).Gap(10).Children(func() {
				f.Text("键盘快捷键").FontSize(18).Color(t.LabelPrimary)
				f.Text("按 F1 或 Esc 关闭").FontSize(12).Color(t.LabelSecondary)
				for _, bd := range bindings {
					f.Row().FillWidth().Height(rowH).Items(flex.Center).Gap(12).Children(func() {
						// 键名列定宽，右对齐——键名长短不一时才看得成一列
						// 键名列定宽即可，不折腾右对齐（flex.Box 没有 AlignH）
						f.Text(bd.Key).FontSize(13).Color(t.LabelPrimary).Width(120)
						f.Text(bd.Does).FontSize(13).Color(t.LabelSecondary).Grow(1)
					})
				}
			})
		})
}
