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
	"sync"

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
//
// **这张表必须覆盖代码里所有 HandleKey 吃掉的键**——不是"记得的键"。
// 这条规矩由 shortcuts_guard_test.go 卡住：从 plugins/ 下扫 `ui.KeyX`，
// 表里少一个就红。写这条守卫的原因见那个文件的开头。
var bindings = []Binding{
	{"↑ / ↓", "上下选会话（焦点在侧栏时）", "plugins/dssidebar"},
	{"Home / End", "跳到第一个 / 最后一个会话", "plugins/dssidebar"},
	{"Enter", "发送输入框里的内容", "plugins/dscomposer"},
	{"Esc", "拒绝当前审批请求（reject_once）", "plugins/dscomposer/approval.go"},
	{"Esc", "关闭搜索框（搜索打开时）", "plugins/dsconversation"},
	{"Esc / 停止键", "打断当前正在跑的 turn", "plugins/dscomposer（Cancel 接线）"},
	{"F1", "打开 / 关闭这份键位表", "plugins/dsshortcuts"},
	{"F2", "命令面板：模型 / 权限 / 反馈记录 / 栏宽", "plugins/dscommands"},
	{"F5", "打开 / 关闭终端面板", "plugins/dsterminal"},
	{"F6", "把终端输入框里的命令发出去", "plugins/dsterminal"},
	{"F7 / F8", "选第 1 / 第 2 个选项（提问条出现时）", "plugins/dsquestions"},
	{"F12", "收起 / 展开右上角的插件提示条", "plugins/dsdemo"},
	{"Tab / Shift+Tab", "设置面板打开时在面板内循环焦点（模态层陷阱）", "plugins/dssettings/official_modal.go"},
}

// Plugin 只有「显示 / 收起」一个状态。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives

	// mu 保护 shown / notify：HandleKey 会从两个线程进来（窗口线程的 OnKey，
	// 以及 cmd/app `-keys` 的注入 goroutine），而 notify 是在绘制线程的 draw 里写的。
	mu     sync.Mutex
	shown  bool
	notify func(string)
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
		Order: 95, // 比 dsdemo 的提示条（Order 10）后画：两者同时出现时面板在上面
		Draw:  p.draw,
	}}
}

// HandleKey 实现 pluginapi.KeyHandler（**返回 bool**，不是 EventResult——
// 我第一次写成 EventResult，类型断言过不了，插件装上了却永远收不到键）。
//
// **返回 true 是有代价的**：事件不会再传给后面的插件。所以只在
// ①按 F1（是我的键）②面板开着时按 Esc（先关面板）两种情况下吃掉。
// 面板没开时按 Esc 必须放行——那是审批拒绝 / 取消 turn 的键，被我吞掉就出事了。
//
// 状态切换之后**必须请求一次重绘**：HandleKey 跑在绘制之外，改完 shown 之后
// 没有任何一帧会自动发生。pluginapi 没有「请重绘」这个动作，插件手里唯一的
// 公开入口是 Actions.Notify——它的宿主实现写状态栏 + Invalidate（cmd/app 的
// setStatus）。少了这一步，`-keys f1` 的日志是 `key 16 handled=true`、
// 屏幕上却什么都没有：上一轮「面板画不出来」卡的就是这里，不是 Absolute 布局
// （布局在 overlay_test.go 里用真 kernel + 真 dsshell 验过）。
func (p *Plugin) HandleKey(ev ui.KeyEvent) bool {
	switch ev.Key {
	case ui.KeyF1:
		p.mu.Lock()
		p.shown = !p.shown
		shown, notify := p.shown, p.notify
		p.mu.Unlock()
		if notify != nil {
			if shown {
				notify("键位表已打开 · F1 或 Esc 关闭")
			} else {
				notify("键位表已关闭")
			}
		}
		return true
	case ui.KeyEscape:
		p.mu.Lock()
		shown, notify := p.shown, p.notify
		p.shown = false
		p.mu.Unlock()
		if !shown {
			return false
		}
		if notify != nil {
			notify("键位表已关闭")
		}
		return true
	}
	return false
}

// 面板的几何。宽度是设计值，但要能在窄窗口里收窄；顶距是固定值——
// 面板高度要等布局完才知道，而 viva 的 Absolute 只在「没有 edge 的轴」上
// 按 Self/Items 对齐，给不了"垂直居中"。
const (
	sheetWidth    = 520
	sheetMinWidth = 320
	sheetMargin   = 80
	sheetTop      = 96
)

// panelWidth 返回面板在给定窗口宽度下的宽度。
func panelWidth(window float32) float32 {
	w := float32(sheetWidth)
	if window > 0 && w > window-2*sheetMargin {
		w = window - 2*sheetMargin
	}
	if w < sheetMinWidth {
		w = sheetMinWidth
	}
	return w
}

// draw 画一张居中卡片：标题 + 逐条键位。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	// 记住「请求重绘」的入口，给 HandleKey 用（理由见 HandleKey 的注释）。
	// 每帧都写：Actions 是壳每帧新建的，缓存一份旧的不如跟着帧走。
	if env.Act.Notify != nil {
		p.mu.Lock()
		p.notify = env.Act.Notify
		p.mu.Unlock()
	}
	if !p.shown {
		return
	}
	t := p.theme
	// **必须 Absolute，并且必须显式 Width**：
	//   - overlay 槽是"自由画"的区域，普通流式布局里它只是浮层行的一个孩子；
	//   - 只给 Left/Top 的绝对盒子，宽度取自内容的 pref；内容若是 FillWidth，
	//     Fill 又相对父盒子的宽度解析，于是父 0 → 子 0，整块退化成 0 宽（实测）。
	//   - 给两条边（Left+Right）会拉伸到容器宽；这里要的是设计宽度，所以给 Width。
	// 居中用的是 env.Bounds：overlay 槽的边界就是整窗（dsshell 用同一对数给
	// 浮层行定宽高，dsdemo 的提示条也靠它贴在右上角）。
	w := panelWidth(env.Bounds.Width)
	left := (env.Bounds.Width - w) / 2
	if left < 0 {
		left = 0
	}
	rowH := float32(26)
	f.Row().Absolute().
		Left(left).Top(sheetTop).
		Width(w).
		Children(func() {
			f.Column().FillWidth().Radius(t.Radii.Panel).
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
