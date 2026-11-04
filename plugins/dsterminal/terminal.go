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
	"github.com/vivaui/viva/flex"
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

	// open 由 F5 切换。
	open bool
	// act 由 draw 每帧刷新（和 dscommands 同款）：HandleKey 里要借 Notify 的重绘副作用。
	act pluginapi.Actions
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

func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{
		ID:    "terminal-panel",
		Slot:  pluginapi.SlotShellOverlay,
		Order: 98, // 和 files-preview(97) 同一层，靠右错开
		Draw:  p.draw,
	}}
}

// Toggle 打开 / 关掉面板。
func (p *Plugin) Toggle() { p.open = !p.open }

// HandleKey 绑 F5。
//
// 为什么是 F5：F1（快捷键）/ F2（命令面板）/ F12（demo）已经占了，
// 而 viva 的 OnKey **不送字母键**（字母走文本输入框），所以只能用功能键。
// F5 在剩下的里面最靠前、也最不容易误触。
func (p *Plugin) HandleKey(ev ui.KeyEvent) bool {
	if ev.Key != ui.KeyF5 {
		return false
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
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	p.act = env.Act // 给 HandleKey 用（它没有 env）
	if !p.open || env.Act.TerminalRead == nil {
		return
	}
	text := ""
	hasTerm := false
	if env.Act.TerminalOpen != nil {
		if id := env.Act.TerminalOpen(); id != "" {
			hasTerm = true
			text = env.Act.TerminalRead(id)
		}
	}
	// **"没有终端"和"终端没输出"是两件事**：
	//   - 没有终端 → 不画（画个黑框会让人以为开了但坏了）
	//   - 有终端但还没输出 → **要画**，写一句"还没有输出"
	//     （`bash -s` 是非交互的，不会打提示符；第一次敲命令之前就是空的）
	if !hasTerm {
		return
	}
	t := p.theme
	var lines []string
	if text == "" {
		// 有终端但还没输出：写一句话而不是留白。
		// `bash -s` 是非交互的，不打提示符——空面板会让人以为"开了但坏了"。
		lines = []string{"（还没有输出 —— 终端已就绪）"}
	} else {
		lines = strings.Split(strings.TrimRight(text, "\n"), "\n")
		if len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:]
		}
	}

	f.Column().Absolute().Top(56).Right(24).Width(720).Children(func() {
		f.Column().Radius(t.Radii.Panel).Background(t.InputMajor).Border(1, t.BorderL2).
			PadSides(14, 14, 12, 14).Gap(4).Children(func() {
			f.Text("终端").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
			for _, ln := range lines {
				if ln == "" {
					ln = " " // 空行也要占位，否则段落会挤在一起
				}
				f.Text(ln).FontSize(12).Color(t.LabelSecondary)
			}
		})
	})
}
