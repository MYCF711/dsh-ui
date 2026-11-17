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
		status = terminalPhaseText(state.Phase)
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
