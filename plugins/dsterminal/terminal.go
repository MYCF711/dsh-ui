// Package dsterminal 实现官方 sidebar-terminal 的用户终端控制面。
//
// 每个右栏 occurrence 对应一条真实 Host PTY：create/list/environment/shells →
// retain → attach/watch，输入带官方 typed 语义，rename/close 都等宿主确认。
//
// 画面走 `internal/vterm` 的**单元格网格**（见 grid.go）：颜色、反显、下划线、
// 光标形状与位置、全角字双宽都按官方 xterm 的样子画。`internal/ansi` 的可读文本
// 投影只留作**退路**——壳还没给出网格（老壳 / snapshot 帧未到）时按行画文本，
// 不假装有颜色与光标。
package dsterminal

import (
	"fmt"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/vterm"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/app"
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

	// resizeRequested 记每个 occurrence **上一次发给宿主的**尺寸：官方同尺寸去重
	// 的「当前尺寸」。绘制是逐帧的，而宿主的 state 回填要等 resize 往返，
	// 这中间只读 Info 会对同一个尺寸连发（详见 resize.go 的 requestTerminalResize）。
	resizeRequested map[string]terminalSize

	// guideMenuOpen 是向导卡片上那枚 shell 菜单的展开态（官方 TerminalGuide 的
	// `open` 状态，TerminalGuide.tsx:37）。菜单面画在卡片下方，见 guide.go。
	guideMenuOpen bool

	// scroll 是每个 occurrence 的滚动位置（官方那边由 xterm 的 viewport 持有，
	// 本仓的屏幕在壳里，所以由界面记）。见 scrollback.go。
	scroll map[string]*terminalScrollView

	// selection 是每个 occurrence 的选区（官方那边由 xterm 的 SelectionService
	// 持有）。见 selection.go。
	selection map[string]*terminalSelection

	// mouse 是每个 occurrence 的鼠标上报簿记（按住的键、报过按下的键、上一帧
	// 发出去的事件）。官方那份在 CoreMouseService 里。见 mouse.go。
	mouse map[string]*terminalMouseState

	// clipboard 是「把一段文本写进系统剪贴板」。默认是 Viva 的
	// `app.SetClipboardText`（`viva@v0.6.4/app/system.go:139`；Windows 实现在
	// `internal/platform/windows/system_windows.go:317`，无窗口时用 0 作为
	// owner）。测试换成假的：单测不许往真剪贴板里写字。
	clipboard func(value string) error

	// 下面这几项是**最近一帧**留下的手势几何：滚轮事件只带指针坐标与命中 id，
	// 要知道「落在哪个终端、一格多高、还能往上滚几行」得由绘制时记下来。
	wheelHit        gfx.ID
	wheelTab        string
	wheelCellH      float32
	wheelCellW      float32
	wheelCols       int
	wheelRows       int
	wheelScrollback int
	wheelYdisp      int
	wheelRect       ui.Rect
	// wheelMouse 是这一帧的鼠标上报状态：滚轮要不要转成序列写回 PTY 由它决定
	// （见 mouse.go 的 reportTerminalMouseWheel）。
	wheelMouse vterm.MouseState
}

func (*Plugin) ID() string         { return "ds-terminal" }
func (*Plugin) Provides() []string { return nil }
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	// 复制走 Viva 自己的剪贴板 API，不自己写平台代码。
	p.clipboard = app.SetClipboardText
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
				IconImage: p.guideIcon(), IconSize: guideIconBox,
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
	// 这条 occurrence 没了：滚动位置与选区都不能留给下一次重开
	//（新终端没有那段历史，也没有那段文本）。
	p.forgetScroll(tabID)
	p.forgetSelection(tabID)
	p.forgetMouse(tabID)
	if actions.TerminalCloseOfficial == nil {
		return fmt.Errorf("用户终端 close 未接线")
	}
	return actions.TerminalCloseOfficial(tabID)
}

// HandleScroll 接住落在终端上的滚轮：鼠标上报开着时它归 PTY（见 mouse.go），
// 否则官方那边是 xterm viewport 的滚动条与滚轮（`scrollSensitivity:1`），
// 本仓按同一套换算自己滚（见 scrollback.go）。
//
// 只在指针命中终端时接手；返回 Handled 之后壳不再把这次手势交给页面滚动
// ——官方 xterm 同样是 `preventDefault`，滚到头也不让页面动。
func (p *Plugin) HandleScroll(ev gfx.ScrollEvent) gfx.EventResult {
	if p.wheelHit.IsZero() || ev.Target != p.wheelHit {
		return gfx.EventResult{}
	}
	if ev.DeltaY == 0 {
		return gfx.EventResult{}
	}
	// 滚轮写在本地滚回之前判：鼠标上报开着时这一下是给程序的，官方那条
	// `wheel` 监听器会把事件吃掉（byte 26995），本地滚回不动。
	if p.reportTerminalMouseWheel(ev) {
		return gfx.EventResult{Handled: true}
	}
	return gfx.EventResult{Handled: true, Repaint: p.scrollTerminal(ev)}
}

// HandleKey 是 F5/F6：F5 开关浮层、F6 把输入行写进 PTY。
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
			// 用户一输入就清空选区：官方
			// `_coreService.onUserInput(() => hasSelection && clearSelection())`
			// （client.terminal.js byte 116835）。xterm 里这条挂在**真键盘输入**上，
			// 本仓唯一的人员输入口就是这行命令，所以挂这里。
			p.clearTerminalSelection(p.currentTab)
			// F6 是真实人员输入，不是 xterm 的协议回复，因此 typed=true。
			p.act.TerminalWriteOfficial(p.currentTab, line+"\n", true)
			// 官方 `scrollOnUserInput: true`（DEFAULT_OPTIONS，官方没有覆盖它）：
			// 用户一输入，viewport 回到贴底——正在看的永远是最新那几行。
			p.scrollViewFor(p.currentTab).follow = true
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
		// terminal.module.css:5：`.status{display:flex;flex-wrap:wrap;align-items:center;
		// gap:8px;padding:5px 12px;background:var(--dsw-alias-bg-l2)}`。
		//
		// `flex-wrap:wrap` 用 viva 的 `Flow()`：它是"会换行的 Row"，语义与 CSS 的
		// flex-wrap 一致，每一行各自分配宽度，`gap` 同时是行内与行间距
		// （`viva@v0.6.4/ui/frame.go:533-544`、`website/content/guides/zh/declarative-layout.md:36`）。
		// 旧账写这条"缺 Viva 容器 wrap"是**过期结论**——v0.6.4 有这个容器。
		// 换成 Flow 后窄右栏（面板 360 左右）里的状态文字 + 两个按钮会像官方那样折行，
		// 而不是被压成一行截断。
		ui.Flow(f).FillWidth().Gap(terminalStatusGap).Items(ui.Center).
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
			if state.Grid != nil {
				// 网格路径：`state.Grid` 是壳里那块 VT 屏幕的快照（官方画的是同一块
				// xterm 屏幕缓冲），颜色/光标定位/全角宽度都在里面。见 grid.go。
				//
				// 高度：右栏面板由容器分配剩余高度；F5 浮层的高度由内容决定，
				// 所以按「屏幕行数（封顶 maxLines）× 实测行盒」给。
				height := float32(0)
				if !p.showRightbar {
					height = float32(min(state.Grid.Height, maxLines)) * p.gridRowHeight(env)
				}
				// 宿主 environment 的列/行上限：官方 fitScreen 就是拿它夹住
				// FitAddon 量出来的尺寸（terminal.tsx:195-196）。
				maxCols, maxRows := 0, 0
				if state.Environment != nil {
					maxCols, maxRows = state.Environment.MaxCols, state.Environment.MaxRows
				}
				p.drawTerminalGrid(f, env, tabID, state, height, maxCols, maxRows)
			} else {
				for _, line := range terminalLines(state.Screen) {
					if line == "" {
						line = " "
					}
					// terminal.tsx:95-103,185 + client.terminal.js:8：Consolas 档、13px、lineHeight=1。
					// Viva 的 MonoSystem 在 Windows 同样落 Consolas；Mono 不读 LineHeight，故固定 13px 行盒。
					ui.Text(f, line).Mono().FontSize(terminalFontSize).Height(terminalFontSize).Color(t.LabelPrimary)
				}
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
