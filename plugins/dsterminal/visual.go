package dsterminal

import (
	"strconv"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 终端几何逐项来自 rc.2 官方 CSS；注释中的行号对应
// dsh-client-ui-sidebar-terminal/src/client/*.module.css。
const (
	// terminal.module.css:1 —— .root。
	terminalRootPadTop   float32 = 8
	terminalRootFontSize float32 = 13
	// terminal.module.css:2 —— .screen。
	terminalScreenPad float32 = 8
	// terminal.module.css:5-6 —— .status / .error。
	terminalStatusGap  float32 = 8
	terminalStatusPadY float32 = 5
	terminalStatusPadX float32 = 12
	terminalErrorPadY  float32 = 10
	terminalErrorPadX  float32 = 12
	// terminal.tsx:95-103,185；theme-settings.ts:57-68：默认终端 cell 字号 13。
	terminalFontSize float32 = 13
	// TerminalIcon.tsx:9：viewBox/width/height 都是 16。
	terminalTabIconSize float32 = 16
	// terminal.module.css:7-8 —— `.empty` / `.emptyMessage`：
	//   .empty{flex:1;column;center/center;gap:16px;min-height:0;padding:24px;overflow:auto;text-align:center}
	//   .emptyMessage{max-width:28ch;margin:0;line-height:1.6;overflow-wrap:anywhere}
	// 只取 gap / padding / line-height 三个像素值；`28ch` 不搬——
	// `ch` 是"当前字体 0 的进位宽"，Viva 没有这个单位的度量口，编一个 px 值
	// 就是自造数值（面板宽 360 减去 24×2 内边距后本来就窄于 28ch）。
	terminalEmptyGap  float32 = 16
	terminalEmptyPad  float32 = 24
	terminalEmptyLine float32 = 1.6
	// TerminalTitle.module.css:2 —— .name。
	terminalRenameWidth    float32 = 120
	terminalRenameMinWidth float32 = 48
	terminalRenamePadX     float32 = 4
	terminalRenameBorder   float32 = 0.5
	// ui-primitives/Button.module.css:2-16,28-34,56-60,78-84 —— small Button。
	terminalButtonHeight float32 = 28
	terminalButtonPadX   float32 = 10
	terminalButtonGap    float32 = 4
	terminalButtonFont   float32 = 12
	terminalButtonLine   float32 = 18
	terminalButtonBorder float32 = 0.5
	terminalButtonIcon   float32 = 16
)

type terminalButtonKind uint8

const (
	terminalButtonPrimary terminalButtonKind = iota
	terminalButtonOutline
	terminalButtonGhost
)

// terminalStatus 对应 terminal.tsx:46-65 的 status/ended/retry/readOnly 判定。
func terminalStatus(state pluginapi.TerminalViewState) (status string, ended, retry, readOnly bool) {
	if state.Phase != "" && state.Phase != "connected" {
		// 官方 phase==='failed' 且 info===undefined 时**不输出状态文字**，
		// 只留 Retry 按钮（terminal.tsx:46-52 的 status 判定链里没有 failed 这一支）。
		if state.Phase != "failed" {
			status = terminalPhaseText(state.Phase)
		}
	}
	ended = state.Phase == "closed"
	if state.Info != nil {
		switch state.Info.State {
		case "exited":
			code := "—"
			if state.Info.ExitCode != nil {
				code = strconv.Itoa(*state.Info.ExitCode)
			}
			status = "进程已退出（" + code + "）"
			ended = true
		case "failed":
			status = "不可用"
		}
		readOnly = state.Phase == "connected" && state.Info.State == "running" && !state.Writable
	}
	retry = !ended && (state.Phase == "failed" || state.Phase == "disconnected")
	if readOnly && status == "" {
		status = "此页面当前只读。"
	}
	return status, ended, retry, readOnly
}

// drawTerminalButton 画官方 Button size="sm"：28 高、0×10、gap 4、12/18、radius-sm。
func drawTerminalButton(f ui.C, env *pluginapi.Env, name, label string, kind terminalButtonKind, withPlus bool) bool {
	t := env.Theme
	box := ui.Row(f).Shrink(0).Height(terminalButtonHeight).Items(ui.Center).Justify(ui.Center).
		Gap(terminalButtonGap).PadSides(0, terminalButtonPadX, 0, terminalButtonPadX).Radius(t.Radii.SM)
	foreground := t.LabelPrimary
	switch kind {
	case terminalButtonPrimary:
		box = box.Background(t.ButtonPrimaryFill).Hover(ui.Look{}.Background(t.ButtonPrimaryHover))
		foreground = t.ButtonPrimaryFg
	case terminalButtonOutline:
		box = box.Border(terminalButtonBorder, t.BorderL3).Hover(ui.Look{}.Background(t.Hover))
	default:
		box = box.Hover(ui.Look{}.Background(t.Hover))
	}
	var id gfx.ID
	if env.Tree != nil {
		id = env.Tree.ID(env.Feat, name)
	}
	box.Hit(id, label).Children(func() {
		if withPlus {
			ui.Row(f).Width(terminalButtonIcon).Height(terminalButtonIcon).Shrink(0).
				Items(ui.Center).Justify(ui.Center).Children(func() {
				if icon, ok := terminalPlusIcon(); ok {
					ui.Icon(f, icon).Size(terminalButtonIcon, terminalButtonIcon).Color(foreground)
				}
			})
		}
		ui.Text(f, label).FontSize(terminalButtonFont).LineHeight(terminalButtonLine).Color(foreground)
	})
	return box.Clicked()
}

// 官方 `TerminalViewIssue` 里本插件会单独画分支的一档
// （OFF:dsh-api-terminal-controller/src/client/model.ts:19）。
// 其余四档共用同一段错误行渲染。
const terminalIssueMissingTerminal = "missingTerminal"

// terminalIssueText 是官方 `sidebarTerminal` 词表里的 issue 文案
// （OFF:dsh-client-ui-sidebar-terminal/src/client/locales.ts:21-25，中文逐字）。
func terminalIssueText(issue string) string {
	switch issue {
	case "missingTerminal":
		return "此终端已不存在，请新建终端。"
	case "inputFull":
		return "输入缓冲区已满，请重新连接后重试。"
	case "attachmentEnded":
		return "终端连接已结束，请重新连接。"
	case "invalidOutput":
		return "终端画面传输异常，请重新连接。"
	case "terminalLimit":
		return "终端数量已达上限，请关闭不用的终端后重试。已退出的终端也计入数量。"
	}
	return ""
}

// drawMissingTerminal 画官方 `.empty` 空态（terminal.module.css:7-8、
// terminal.tsx:36-45）：提示文案 + 主要按钮「新建终端」。官方在文案之上还有
// 一枚 36×36 的终端插画（`newTerminal` 之外另画），本仓不画那枚插画，
// 因为它只存在于官方产物里、没有可回查的素材登记（差异登记在账本 #54 行）。
func (p *Plugin) drawMissingTerminal(f ui.C, env *pluginapi.Env) {
	t := p.theme
	ui.Column(f).FillWidth().FillHeight().Grow(1).MinHeight(0).Pad(terminalEmptyPad).
		Gap(terminalEmptyGap).Items(ui.Center).Justify(ui.Center).Children(func() {
		ui.Text(f, terminalIssueText(terminalIssueMissingTerminal)).
			FontSize(terminalRootFontSize).LineHeight(terminalRootFontSize * terminalEmptyLine).
			Color(t.LabelPrimary).TextCenter()
		if env.Act.OpenRightTab != nil &&
			drawTerminalButton(f, env, "terminal-new-missing", "新建终端", terminalButtonPrimary, true) {
			env.Act.OpenRightTab(RightTabTerminal)
		}
	})
}
