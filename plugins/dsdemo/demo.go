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
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 这条提示条的每个数字都抄自官方浮层底板，不估、不凑：
//
//	FontNotice.module.css:32  width: min(340px, calc(100vw - 24px))
//	FontNotice.module.css:34  padding: 16px                 （面板整体内边距，单行条不用）
//	FontNotice.module.css:37  background: var(--dsw-alias-bg-layer-2)
//	FontNotice.module.css:39  border-radius: var(--dsw-radius-lg)   → base.css:21 = 16px
//	FontNotice.module.css:45  top: max(12px, …)
//	FontNotice.module.css:48  right: 12px
//	FontNotice.module.css:72  .description 的 label-secondary（正文色）
//
// 边上那道 0.5px 描边来自官方横幅（浮层底板在官方是"边框 0 或 0.5px hairline"二选一，
// 我们有边框就得是 0.5px，1px 会把浮层画成卡片边框）：
//
//	viewer-status-banner.module.css:5  border: 0.5px solid var(--dsw-alias-border-l2)
//
// 单行文字条本身的内边距与字号，官方紧凑条（padding 8/12、12/18）：
//
//	CordisRunRow.module.css:106-112  padding: 8px 12px; font-size: 12px; line-height: 18px
//	InputBar.module.css:130          gap: 8px（行内图标间距）
const (
	demoInset    float32 = 12  // 距窗口右上角
	demoMaxWidth float32 = 340 // 官方 min(340px, …)
	demoGutter   float32 = 24  // 官方 calc(100vw - 24px) 的两侧余量
	demoBorder   float32 = 0.5 // 官方 hairline
	demoPadX     float32 = 12
	demoPadY     float32 = 8
	demoGap      float32 = 8
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
func (p *Plugin) HandleKey(ev gfx.KeyEvent) bool {
	if ev.Key != gfx.KeyF12 || ev.Modifiers != 0 {
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
func (p *Plugin) draw(f ui.C, env *pluginapi.Env) {
	if !p.shown {
		return
	}
	t := p.theme

	// 官方这里是 `width` 而不是 `max-width`：浮层是定宽底板，
	// 所以用 Width 拿满 min(340px, 100vw-24px)，不靠内容 pref 撑。
	ui.Row(f).Absolute().Top(demoInset).Right(demoInset).Width(panelWidth(env.Bounds.Width)).
		Radius(t.Radii.LG).Background(t.BgLayer2).Border(demoBorder, t.BorderL2).
		PadX(demoPadX).PadY(demoPadY).Items(ui.Center).Gap(demoGap).Children(func() {
		ui.Text(f, "ds-demo 插件已接入 · 按 F12 收起").
			FontSize(t.Fonts.Caption).Color(t.LabelSecondary).Ellipsis()
		// 关闭键的几何交给 prims（28px 圆钮），**图标用官方素材**：
		// 官方界面里的图标键一律画 <svg>，从不写字形（MessageFeedbackActions.module.css:20-23
		// 的 `.action svg { width: 15px }`），所以这里取 dsassets 的
		// IconCloseFillRegular（sidebar.go:95 `WorkspaceCloseFill`）而不是「×」。
		// 素材解析失败只在官方路径出现未知图元时发生，退回字形保住可点击性。
		id := env.Tree.ID(env.Feat, "demo-close")
		if icon, ok := dsassets.WorkspaceCloseFill(t.LabelSecondary); ok {
			if p.prims.CircleIconButton(f, id, icon, false) {
				p.shown = false
			}
		} else if p.prims.CircleButton(f, id, "×", false) {
			p.shown = false
		}
	})
}

// panelWidth 是官方 `width: min(340px, calc(100vw - 24px))` 的翻译：
// 窗口窄到 364px 以下时按两侧各 12px 让路，否则封顶 340。
func panelWidth(windowWidth float32) float32 {
	if available := windowWidth - demoGutter; available < demoMaxWidth {
		return available
	}
	return demoMaxWidth
}

var (
	_ pluginapi.Plugin     = (*Plugin)(nil)
	_ pluginapi.Panels     = (*Plugin)(nil)
	_ pluginapi.KeyHandler = (*Plugin)(nil)
)
