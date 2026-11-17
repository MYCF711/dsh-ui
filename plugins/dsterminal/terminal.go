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

	// guideMenuOpen 是向导卡片上那枚 shell 菜单的展开态（官方 TerminalGuide 的
	// `open` 状态，TerminalGuide.tsx:37）。菜单面画在卡片下方，见 guide.go。
	guideMenuOpen bool
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
			Label: "终端", Order: 20, Multiple: true,
			// TerminalIcon.tsx:9 的官方 16×16 prompt 图标；没有额外右 margin。
			IconImage: terminalTitleIcon(), IconSize: terminalTabIconSize,
			// 官方向导页的入口卡：id 'new' / order 20 / title '新建终端' / description
			// '在会话工作区运行命令'（index.ts:74、locales.ts:14-15）。
			Guide: []pluginapi.TabGuide{{
				ID: "new", Order: 20, Title: zhGuideTitle, Description: zhGuideDescription,
				IconImage: terminalTitleIcon(), IconSize: guideIconBox,
			}},
			Draw: p.drawPanelBody},
		{ID: "terminal-panel", Slot: pluginapi.SlotShellOverlay, Order: 98, Draw: p.draw},
		// 官方向导卡（`SidebarRightTabDefinition.guide`，index.ts:72-75）。
		{ID: "terminal-guide", Slot: pluginapi.SlotRightGuideEntry, Key: guideEntryPainterKey,
			Draw: p.drawGuideEntry},
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
		// F5 浮层是既有 derived 入口；终端内容自身仍只吃官方 root 的 8px 上内边距。
		ui.Column(f).Radius(p.theme.Radii.Panel).Background(p.theme.InputMajor).Border(1, p.theme.BorderL2).
			PadTop(terminalRootPadTop).Children(func() {
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
	// terminal.module.css:1：root  height:100%; min-height:0; padding-top:8px; font-size:13px。
	ui.Column(f).FillWidth().FillHeight().MinHeight(0).PadTop(terminalRootPadTop).Children(func() {
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
	// 官方 `state.issue === 'missingTerminal'` 是一块**空态**而不是错误行：
	// `.empty{flex:1;column;center;gap:16px;min-height:0;padding:24px;text-align:center}`
	// + `.emptyMessage{max-width:28ch;line-height:1.6}` + 主要按钮「新建终端」
	// （terminal.module.css:7-8、terminal.tsx:36-45）。
	if state.Issue == terminalIssueMissingTerminal {
		p.drawMissingTerminal(f, env)
		return
	}
	status, ended, retry, readOnly := terminalStatus(state)
	// terminal.module.css:5：状态条只在一项状态/动作存在时出现，gap 8、padding 5×12、bg-l2。
	if status != "" || retry || readOnly || ended {
		ui.Row(f).FillWidth().Gap(terminalStatusGap).Items(ui.Center).
			PadSides(terminalStatusPadY, terminalStatusPadX, terminalStatusPadY, terminalStatusPadX).
			Background(t.BgLayer2).Children(func() {
			if status != "" {
				ui.Text(f, status).FontSize(terminalRootFontSize).Color(t.LabelPrimary)
			}
			if readOnly && env.Act.TerminalRetry != nil &&
				drawTerminalButton(f, env, "terminal-control", "接管输入", terminalButtonOutline, false) {
				env.Act.TerminalRetry(tabID)
			}
			if retry && env.Act.TerminalRetry != nil {
				label := "重新连接"
				if state.Phase == "failed" && state.Info == nil {
					label = "重试"
				}
				if drawTerminalButton(f, env, "terminal-retry", label, terminalButtonOutline, false) {
					env.Act.TerminalRetry(tabID)
				}
			}
			if ended && env.Act.OpenRightTab != nil &&
				drawTerminalButton(f, env, "terminal-new-ended", "新建终端", terminalButtonPrimary, true) {
				env.Act.OpenRightTab(RightTabTerminal)
			}
		})
	}
	if state.Info != nil {
		// terminal.module.css:2-4：screen 吃掉剩余高度，四边 8px、bg-base、label-primary。
		screen := ui.Column(f).FillWidth().Pad(terminalScreenPad).Background(t.BgBase).Clip()
		if p.showRightbar {
			screen = screen.Grow(1).MinHeight(0)
		}
		screen.Children(func() {
			for _, line := range terminalLines(state.Screen) {
				if line == "" {
					line = " "
				}
				// terminal.tsx:95-103,185 + client.terminal.js:8：Consolas 档、13px、lineHeight=1。
				// Viva 的 MonoSystem 在 Windows 同样落 Consolas；Mono 不读 LineHeight，故固定 13px 行盒。
				ui.Text(f, line).Mono().FontSize(terminalFontSize).Height(terminalFontSize).Color(t.LabelPrimary)
			}
			p.drawDerivedControls(f, env, tabID, state)
		})
	}
	// 官方 `error = phase==='disconnected' ? undefined
	//   : issue === undefined ? state.error ?? state.info?.error : t(issue)`
	// （terminal.tsx:46）：**issue 优先**，它有自己的官方文案，不能把 key 当错误原文。
	errorText := state.Error
	if state.Phase == "disconnected" {
		errorText = ""
	} else if state.Issue != "" {
		errorText = terminalIssueText(state.Issue)
	} else if errorText == "" && state.Info != nil {
		errorText = state.Info.Error
	}
	if errorText != "" {
		// terminal.module.css:6：错误段 margin 0、padding 10×12、bg-l2、label-primary。
		ui.Text(f, "终端错误："+errorText).FillWidth().FontSize(terminalRootFontSize).
			Color(t.LabelPrimary).PadSides(terminalErrorPadY, terminalErrorPadX, terminalErrorPadY, terminalErrorPadX).
			Background(t.BgLayer2)
	}
}

// drawDerivedControls 保留本仓已有的 F6 输入与重命名能力；它们不是官方 xterm 工具栏。
// 几何只复用官方 .name 与 Button sm，不另造一套按钮尺寸。
func (p *Plugin) drawDerivedControls(f ui.C, env *pluginapi.Env, tabID string, state pluginapi.TerminalViewState) {
	t := p.theme
	if p.renaming {
		// TerminalTitle.module.css:2：name 120px、min-width 48px、**max-width 100%**、
		// 0×4 padding、0.5px border、radius-sm。
		ui.TextFieldID(f, env.Tree.ID(env.Feat, "terminal-rename"), &p.renameField, "终端名称", "1–120 个字符").
			Width(terminalRenameWidth).MinWidth(terminalRenameMinWidth).MaxWidthPercent(100).
			PadSides(0, terminalRenamePadX, 0, terminalRenamePadX).
			FontSize(terminalRootFontSize).Background(t.BgLayer1).
			Border(terminalRenameBorder, t.BorderL3).Radius(t.Radii.SM)
		ui.Row(f).Gap(terminalStatusGap).Children(func() {
			if drawTerminalButton(f, env, "terminal-rename-save", "保存名称", terminalButtonPrimary, false) {
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
			if drawTerminalButton(f, env, "terminal-rename-cancel", "取消", terminalButtonGhost, false) {
				p.renaming = false
			}
		})
	} else {
		// 按钮是**块级 Row**：直接放进这个 Column 会被拉满整行（viva 的块级容器占满
		// 父宽，`Shrink(0)` 只管主轴、不管交叉轴的宽度），渲染出来是「704px 宽、
		// 标签居中」的隐形按钮——命中区也跟着变成整行（截图比对时量到的
		// HIT 重命名 = x8/w704）。官方 Button 是 inline 尺寸（按钮宽度由 label 决定，
		// ButtonSizeSmall），所以这里包一层 Row，让它按内容宽排。
		var renameClicked bool
		ui.Row(f).FillWidth().Children(func() {
			renameClicked = drawTerminalButton(f, env, "terminal-rename", "重命名", terminalButtonGhost, false)
		})
		if renameClicked {
			p.renameField.SetText(state.Info.Title)
			p.renaming = true
		}
	}
	if state.Writable {
		ui.TextFieldID(f, env.Tree.ID(env.Feat, "terminal-input"), &p.field, "命令", "输入后按 F6 执行").
			FontSize(terminalRootFontSize)
	}
}

func terminalPhaseText(phase string) string {
	switch phase {
	case "idle", "loading":
		return "正在读取终端环境…"
	case "creating":
		return "正在启动…"
	case "connecting":
		return "正在连接…"
	case "disconnected":
		return "连接已断开。"
	case "closed":
		return "终端已关闭。"
	case "failed":
		return "不可用"
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
