// Package dsterminal 在 shell 浮层里显示**长连接 shell 的输出**。
//
// # 它不是终端模拟器
//
// 不处理光标定位、不清屏、不画彩色。`internal/ansi` 只剥转义序列，
// 剩下的当纯文本画——**画一个半对半错的彩色终端，不如老老实实给可读文字**。
//
// # 一次会话一个
//
// 面板是"看一眼命令在跑什么"，不是"开一堆标签页"。多开会让
// "哪个是哪个"变成一个新问题，而那不是当前要解决的问题。
package dsterminal

import (
	"strings"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// maxLines 面板里最多显示几行（取**最后**这几行）。
//
// 为什么取最后：终端是往下滚的，用户关心的是"刚刚发生了什么"。
// 从头显示会让他每次都往下翻。
const maxLines = 18

type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives

	// open 由 F5 切换（右栏收起时用的浮层面板）。
	open bool
	// act / showRightbar 由 draw 每帧刷新（和 dscommands 同款）：HandleKey 没有 env，
	// 而 F5 要按"右栏开没开"分流，所以每帧把这两个事实抓下来。
	act          pluginapi.Actions
	showRightbar bool

	// field 是终端命令输入框。**值类型**：写成 *flex.Field 又忘了 new，
	// Fields() 会返回一个 nil 元素，flex.Fields(...) 里直接 panic（dscomposer 踩过）。
	field ui.Field
}

func (*Plugin) ID() string { return "ds-terminal" }

func (*Plugin) Provides() []string { return nil }

func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	return nil
}

// RightTabTerminal 是本包注册的右栏 tab **类型** key。
//
// 官方的终端是右栏的一个 tab 类型（OFF:dsh-client-ui-sidebar-terminal/src/client/
// index.ts:72-75 注册 `{kind:'terminal', priority:'builtin', title:()=>t('title')}`，
// tab 体注册在 `sidebar.right.pane.tab` 的 key = 类型 `id` 上，同文件 :95-99）。
//
// 本仓只用**一个** key（= 官方的 `kind`）：官方把 `id`（包名，逐实现唯一）与
// `kind`（类型判别）分开，是为了让一个 extension 顶掉同名 builtin；那条
// `priority` 覆盖机制本仓还没建（登记在 docs/official-parity.md 的缺口里），
// 所以现在没有第二种实现来抢同一个 kind。
const RightTabTerminal = "terminal"

// Panels 实现 pluginapi.Panels。
//
// 两块画，两个座位：
//
//	sidebar.right.pane.tab（keyed）  官方座位：终端是右栏的一个 tab 类型
//	shell.overlay                    右栏收起时的 F5 浮层（本包已有的产品行为）
//
// 两块共用同一个终端与同一份读取逻辑，**不会同屏出现**：浮层在右栏展开时整块不画
// （见 draw 的第一条判断）。两个入口画两套内容是这个仓踩过的坑（审批条那次）。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "terminal-tab", Slot: pluginapi.SlotRightPaneTab, Key: RightTabTerminal,
			Label: "终端", Order: 20, Draw: p.drawPanelBody},
		{
			ID:    "terminal-panel",
			Slot:  pluginapi.SlotShellOverlay,
			Order: 98, // 和 files-preview(97) 同一层，靠右错开
			Draw:  p.draw,
		},
	}
}

// Fields 把输入框登记给壳（viva 只把登记过的 Field 当输入焦点候选）。
func (p *Plugin) Fields() []*ui.Field { return []*ui.Field{&p.field} }

// Toggle 打开 / 关掉面板。
func (p *Plugin) Toggle() { p.open = !p.open }

// HandleKey 绑 F5。
//
// 为什么是 F5：F1（快捷键）/ F2（命令面板）/ F12（demo）已经占了，
// 而 viva 的 OnKey **不送字母键**（字母走文本输入框），所以只能用功能键。
// F5 在剩下的里面最靠前、也最不容易误触。
func (p *Plugin) HandleKey(ev gfx.KeyEvent) bool {
	// F6：把输入框里那行发给终端。
	//
	// **用 F6 而不是 Enter**：Enter 是输入框自己的提交键，这层拿不到
	// （viva 的 OnKey 只送功能键/方向/Enter 的**空文本**）。
	// 让用户在框里打字、按 F6 执行，比抢 Enter 更不容易和 composer 打架。
	if ev.Key == gfx.KeyF6 {
		if !p.open {
			return false
		}
		line := strings.TrimSpace(p.field.Text())
		if line == "" {
			return true // 空行按了也不做事，但算消费掉（否则会冒泡成别的键）
		}
		p.field.SetText("")
		if p.act.TerminalOpen != nil && p.act.TerminalInput != nil {
			if id := p.act.TerminalOpen(); id != "" {
				// 补换行：终端是行缓冲的，不带换行它不会执行。
				p.act.TerminalInput(id, line+"\n")
			}
		}
		if p.act.Notify != nil {
			p.act.Notify("已发送: " + line)
		}
		return true
	}
	if ev.Key != gfx.KeyF5 {
		return false
	}
	// F5 按"右栏开没开"分流：
	//
	//	右栏开着 → 把右栏切到终端页签（官方形状：终端就是右栏的一个 tab）
	//	右栏收起 → 用本包自己的浮层面板（行为不变）
	//
	// 为什么要分流而不是只留一条：右栏收起时插件无法"打开"它（Actions 只有
	// ToggleRightbar，且状态归壳），把终端藏进一个看不见的列等于 F5 失灵。
	if p.showRightbar && p.act.SelectRightTab != nil {
		p.act.SelectRightTab(RightTabTerminal)
		if p.act.Notify != nil {
			p.act.Notify("终端已打开 · 右栏面板")
		}
		return true
	}
	p.Toggle()
	// 借 Notify 的副作用请求重绘（壳的实现是 setStatus：写状态栏 + Invalidate）。
	// pluginapi 没有显式的 repaint 动作——dsshortcuts/dscommands 都这么做。
	if p.act.Notify != nil {
		if p.open {
			p.act.Notify("终端已打开 · F5 关闭")
		} else {
			p.act.Notify("终端已关闭")
		}
	}
	return true
}

// draw 画终端面板。
//
// **没有终端就不画**：`TerminalRead` 返回空串时整块不出现——
// 画一个空白黑框会让人以为"终端开了但没输出"，实际是我们没开。
func (p *Plugin) draw(f ui.C, env *pluginapi.Env) {
	p.capture(env)
	// 右栏展开时终端在**右栏页签**里（同一块内容、另一个座位），浮层整块不画——
	// 两个入口同屏画两套是这一仓踩过的坑（审批条那次同屏两条）。
	if !p.open || env.Data.ShowRightbar {
		return
	}
	lines, ok := p.readLines(env)
	if !ok {
		return
	}
	t := p.theme
	ui.Column(f).Absolute().Top(56).Right(24).Width(720).Children(func() {
		ui.Column(f).Radius(t.Radii.Panel).Background(t.InputMajor).Border(1, t.BorderL2).
			PadSides(14, 14, 12, 14).Gap(4).Children(func() {
			p.drawLines(f, env, lines)
		})
	})
}

// drawPanelBody 是右栏页签里的终端面板（官方 `sidebar.right.pane.tab`，key = terminal）。
//
// 与浮层共用 `readLines` / `drawLines`：**同一份终端、同一套判据**，差别只有外壳
// （浮层是绝对定位的 720 宽卡片；页签里铺满槽位，内边距沿用浮层的 14/14/12/14）。
func (p *Plugin) drawPanelBody(f ui.C, env *pluginapi.Env) {
	p.capture(env)
	lines, ok := p.readLines(env)
	if !ok {
		return
	}
	ui.Column(f).FillWidth().FillHeight().PadSides(14, 14, 12, 14).Gap(4).Children(func() {
		p.drawLines(f, env, lines)
	})
}

// capture 抓这一帧的壳事实给 HandleKey 用（它没有 env）。
func (p *Plugin) capture(env *pluginapi.Env) {
	p.act = env.Act
	p.showRightbar = env.Data.ShowRightbar
}

// readLines 取终端输出，并判"到底有没有终端"。
//
// **"没有终端"和"终端没输出"是两件事**：
//   - 没有终端 → 不画（画个黑框会让人以为开了但坏了）
//   - 有终端但还没输出 → **要画**，写一句"还没有输出"
//     （`bash -s` 是非交互的，不会打提示符；第一次敲命令之前就是空的）
func (p *Plugin) readLines(env *pluginapi.Env) ([]string, bool) {
	if env.Act.TerminalRead == nil || env.Act.TerminalOpen == nil {
		return nil, false
	}
	id := env.Act.TerminalOpen()
	if id == "" {
		return nil, false
	}
	text := env.Act.TerminalRead(id)
	if text == "" {
		// 有终端但还没输出：写一句话而不是留白。
		// `bash -s` 是非交互的，不打提示符——空面板会让人以为"开了但坏了"。
		return []string{"（还没有输出 —— 终端已就绪）"}, true
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, true
}

// drawLines 画输出行 + 命令输入框（浮层与页签共用）。
func (p *Plugin) drawLines(f ui.C, env *pluginapi.Env, lines []string) {
	t := p.theme
	ui.Text(f, "终端").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
	for _, ln := range lines {
		if ln == "" {
			ln = " " // 空行也要占位，否则段落会挤在一起
		}
		ui.Text(f, ln).FontSize(12).Color(t.LabelSecondary)
	}
	// 输入框：F6 执行。**放在输出下面**——终端本来就是"上面输出、下面敲命令"。
	ui.TextFieldID(f, env.Tree.ID(env.Feat, "terminal"), &p.field, "命令", "输入后按 F6 执行")
}
