// Package dsterminal 实现官方 sidebar-terminal 的用户终端控制面。
//
// 每个右栏 occurrence 对应一条真实 Host PTY：create/list/environment/shells →
// retain → attach/watch，输入带官方 typed 语义，rename/close 都等宿主确认。
// 本仓仍使用 internal/ansi 的可读文本投影，不冒充 xterm 的完整 VT 模拟器。
package dsterminal

import (
	"fmt"
	"strings"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const maxLines = 18

type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives

	open         bool
	act          pluginapi.Actions
	showRightbar bool
	rightActive  bool
	currentTab   string

	field       ui.Field
	renameField ui.Field
	renaming    bool
}

func (*Plugin) ID() string         { return "ds-terminal" }
func (*Plugin) Provides() []string { return nil }
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	return nil
}

// RightTabTerminal 与官方 tab-registry 的 kind:'terminal' 一致。
const RightTabTerminal = "terminal"

func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "terminal-tab", Slot: pluginapi.SlotRightPaneTab, Key: RightTabTerminal,
			Label: "终端", Order: 20, Multiple: true, Draw: p.drawPanelBody},
		{ID: "terminal-panel", Slot: pluginapi.SlotShellOverlay, Order: 98, Draw: p.draw},
	}
}

func (p *Plugin) Fields() []*ui.Field { return []*ui.Field{&p.field, &p.renameField} }
func (p *Plugin) Toggle()             { p.open = !p.open }

// RightTabTitle 对应官方 TerminalTitle：标题来自 attach/watch 的真实 info.title。
func (p *Plugin) RightTabTitle(sessionID, tabID string) string {
	if p.act.TerminalSnapshot == nil {
		return ""
	}
	state := p.act.TerminalSnapshot(tabID)
	if state.Info == nil {
		return ""
	}
	return state.Info.Title
}

// CloseRightTab 对应官方 registerCloseHandler：Host close 成功后才允许壳删 tab。
func (p *Plugin) CloseRightTab(sessionID, tabID, kind string, actions pluginapi.Actions) error {
	if kind != RightTabTerminal {
		return nil
	}
	if actions.TerminalCloseOfficial == nil {
		return fmt.Errorf("用户终端 close 未接线")
	}
	return actions.TerminalCloseOfficial(tabID)
}

func (p *Plugin) HandleKey(ev gfx.KeyEvent) bool {
	if ev.Key == gfx.KeyF6 {
		if !p.open && !p.rightActive {
			return false
		}
		line := strings.TrimSpace(p.field.Text())
		if line == "" {
			return true
		}
		p.field.SetText("")
		if p.currentTab != "" && p.act.TerminalWriteOfficial != nil {
			// F6 是真实人员输入，不是 xterm 的协议回复，因此 typed=true。
			p.act.TerminalWriteOfficial(p.currentTab, line+"\n", true)
		}
		if p.act.Notify != nil {
			p.act.Notify("输入已提交: " + line)
		}
		return true
	}
	if ev.Key != gfx.KeyF5 {
		return false
	}
	if p.showRightbar && p.act.OpenRightTab != nil {
		// 官方 multiple:true：每次新建都产生独立 occurrence/Host identity。
		p.act.OpenRightTab(RightTabTerminal)
		if p.act.Notify != nil {
			p.act.Notify("新终端已打开 · 右栏面板")
		}
		return true
	}
	p.Toggle()
	if p.act.Notify != nil {
		if p.open {
			p.act.Notify("终端已打开 · F5 关闭")
		} else {
			p.act.Notify("终端已关闭")
		}
	}
	return true
}

func (p *Plugin) capture(env *pluginapi.Env) {
	p.act = env.Act
	p.showRightbar = env.Data.ShowRightbar
	p.rightActive = env.Data.ShowRightbar && env.ActiveRightTabKind == RightTabTerminal
	if env.RightTabID != "" {
		p.currentTab = env.RightTabID
	} else if env.ActiveRightTab != "" {
		p.currentTab = env.ActiveRightTab
	} else if p.currentTab == "" {
		p.currentTab = RightTabTerminal
	}
}

func (p *Plugin) draw(f ui.C, env *pluginapi.Env) {
	p.capture(env)
	if !p.open || env.Data.ShowRightbar {
		return
	}
	ui.Column(f).Absolute().Top(56).Right(24).Width(720).Children(func() {
		ui.Column(f).Radius(p.theme.Radii.Panel).Background(p.theme.InputMajor).Border(1, p.theme.BorderL2).
			PadSides(14, 14, 12, 14).Gap(4).Children(func() {
			p.drawTerminal(f, env, RightTabTerminal)
		})
	})
}

func (p *Plugin) drawPanelBody(f ui.C, env *pluginapi.Env) {
	p.capture(env)
	tabID := env.RightTabID
	if tabID == "" {
		tabID = env.ActiveRightTab
	}
	if tabID == "" {
		tabID = RightTabTerminal
	}
	ui.Column(f).FillWidth().FillHeight().PadSides(14, 14, 12, 14).Gap(4).Children(func() {
		p.drawTerminal(f, env, tabID)
	})
}

func (p *Plugin) drawTerminal(f ui.C, env *pluginapi.Env, tabID string) {
	if env.Act.TerminalEnsure == nil || env.Act.TerminalSnapshot == nil {
		return // 老壳没有官方控制面时整块 HIDE，不回退到旧 terminal.open 冒充。
	}
	env.Act.TerminalEnsure(tabID)
	state := env.Act.TerminalSnapshot(tabID)
	t := p.theme
	ui.Row(f).Gap(8).Children(func() {
		ui.Text(f, "终端").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		if state.Info != nil {
			ui.Text(f, state.Info.Title).FontSize(t.Fonts.Caption).Color(t.LabelSecondary)
			if state.Info.Origin == "agent" {
				ui.Text(f, "Agent").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
				if state.Info.SandboxMode != "" {
					ui.Text(f, state.Info.SandboxMode).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
				}
			}
		}
		if env.Act.OpenRightTab != nil && ui.Button(f, "新建").Ghost().Small().Clicked() {
			env.Act.OpenRightTab(RightTabTerminal)
		}
	})
	if state.Phase != "" && state.Phase != "connected" {
		ui.Text(f, terminalPhaseText(state.Phase)).FontSize(12).Color(t.LabelTertiary)
	}
	if state.Error != "" {
		ui.Text(f, state.Error).FontSize(12).Color(t.LabelSecondary)
		if env.Act.TerminalRetry != nil && ui.Button(f, "重试 / 重新取得控制权").Outline().Small().Clicked() {
			env.Act.TerminalRetry(tabID)
		}
	}
	if state.Info == nil {
		return
	}
	ui.Text(f, state.Info.Shell.Name+" · "+state.Info.State).FontSize(11).Color(t.LabelTertiary)
	if state.Info.State == "running" && !state.Writable {
		ui.Text(f, "只读（另一个连接持有输入控制权）").FontSize(12).Color(t.LabelTertiary)
		if env.Act.TerminalRetry != nil && ui.Button(f, "取得控制权").Outline().Small().Clicked() {
			env.Act.TerminalRetry(tabID)
		}
	}
	lines := terminalLines(state.Screen)
	for _, line := range lines {
		if line == "" {
			line = " "
		}
		ui.Text(f, line).FontSize(12).Color(t.LabelSecondary)
	}
	if p.renaming {
		ui.TextFieldID(f, env.Tree.ID(env.Feat, "terminal-rename"), &p.renameField, "终端名称", "1–120 个字符")
		ui.Row(f).Gap(6).Children(func() {
			if ui.Button(f, "保存名称").Small().Clicked() {
				title := strings.TrimSpace(p.renameField.Text())
				p.renaming = false
				if title != "" && env.Act.TerminalRenameOfficial != nil {
					go func() {
						if err := env.Act.TerminalRenameOfficial(tabID, title); err != nil && env.Act.Notify != nil {
							env.Act.Notify("重命名失败: " + err.Error())
						}
					}()
				}
			}
			if ui.Button(f, "取消").Ghost().Small().Clicked() {
				p.renaming = false
			}
		})
	} else if ui.Button(f, "重命名").Ghost().Small().Clicked() {
		p.renameField.SetText(state.Info.Title)
		p.renaming = true
	}
	if env.Act.CloseRightTab != nil && ui.Button(f, "关闭终端").Ghost().Small().Clicked() {
		env.Act.CloseRightTab(tabID)
	}
	if state.Writable {
		ui.TextFieldID(f, env.Tree.ID(env.Feat, "terminal-input"), &p.field, "命令", "输入后按 F6 执行")
	}
}

func terminalPhaseText(phase string) string {
	switch phase {
	case "idle", "loading":
		return "正在读取终端环境…"
	case "creating":
		return "正在创建终端…"
	case "connecting":
		return "正在连接终端…"
	case "disconnected":
		return "终端连接已断开"
	case "closed":
		return "终端已关闭"
	case "failed":
		return "终端不可用"
	default:
		return phase
	}
}

func terminalLines(text string) []string {
	if text == "" {
		return []string{"（还没有输出 —— 终端已就绪）"}
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines
}
