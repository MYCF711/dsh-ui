// confirm.go —— TeamDisableConfirmation：会话定向的停用确认
// （官方 `TeamDisableConfirmation.tsx` + `.module.css`）。
//
// # 为什么挂在 shell.overlay 而不是菜单里
//
// 官方：关闭开关时先 `dismiss()` 关菜单，再 `requestDisableConfirmation()`。
// 菜单已经关了，确认框必须活过它——所以落点是 `shell.overlay`
// （mount.ts:40-56 的 `scope.slots.register({name:'shell.overlay', id:'agent-team.disable'})`）。
//
// # 状态机（官方 §2.3）
//
//	{kind:'ready'} → {kind:'saving'} → 成功：cancelDisable(sessionId) 关框
//	                                  → 失败：{kind:'failed', message}（错误**一直可见**
//	                                    到重试或取消；saving 期间两个按钮都禁用）
//
// # 几何（§4.2）
//
//	.dialog  width:min(440px, 100%); max-height:100%
//	.content min-height:0; overflow-y:auto; overscroll-behavior:contain
//	.copy    14px/22px label-secondary; 段间距 12px
package dsagentteam

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const (
	dialogWidth  = 440 // .dialog width:min(440px,100%)
	dialogMargin = 32  // 与官方的 max(视口-32) 留白一致
	dialogPad    = 20
	dialogGap    = 12 // .copy 段间距 12px
	copySize     = 14
	copyLine     = 22
	modalTitleZ  = 16
)

// drawConfirm 画停用确认。slot ③（shell.overlay）的 painter。
//
// 守卫只有一条：`request === null → null`（官方 TeamDisableConfirmation.tsx:33）。
func (p *Plugin) drawConfirm(f ui.C, env *pluginapi.Env) {
	req := p.team.DisableRequest()
	if req == nil {
		return
	}
	t := p.themeFor(env)
	state := p.team.Confirm()
	saving := state.Kind == ConfirmSaving

	width := float32(dialogWidth)
	if env.Bounds.Width > 0 && width > env.Bounds.Width-dialogMargin {
		width = env.Bounds.Width - dialogMargin
	}
	left := (env.Bounds.Width - width) / 2
	if left < 0 {
		left = 0
	}
	top := env.Bounds.Height * 0.22
	if top < 0 {
		top = 0
	}

	// 遮罩：官方 Modal 的 overlay。点遮罩 = onClose = cancel（与官方一致）。
	mask := t.BgBase
	mask.Alpha = 0.55
	backdropID := env.Tree.ID(env.Feat, "team-disable-backdrop")
	backdrop := ui.Row(f).Absolute().Inset(0).Background(mask).Hit(backdropID, zhDisableClose)

	ui.Column(f).Absolute().Left(left).Top(top).Width(width).
		// 对话本体也吃掉点击：官方点框内不关闭，只有遮罩/ESC 才关。
		Hit(env.Tree.ID(env.Feat, "team-disable-dialog"), zhDisableTitle).
		Radius(t.Radii.XL).Background(t.InputMajor).Border(1, t.BorderL2).
		Shadow(gfx.ShadowMD).Pad(dialogPad).Gap(dialogGap).Children(func() {
		// 标题（Modal 的 title；tone=warning 由黄色圆点表达）。
		ui.Row(f).FillWidth().Items(ui.Center).Gap(errGap).Children(func() {
			ui.Row(f).Width(dotSize).Height(dotSize).Radius(dotSize / 2).Background(t.Warn)
			ui.Text(f, zhDisableTitle).FontSize(modalTitleZ).LineHeight(24).Weight(500).
				Color(t.LabelPrimary).Grow(1)
		})
		// .copy：两段说明（14/22，label-secondary）。
		ui.Column(f).FillWidth().Gap(dialogGap).Children(func() {
			ui.Text(f, zhDisableDesc).FillWidth().FontSize(copySize).LineHeight(copyLine).
				Color(t.LabelSecondary)
			ui.Text(f, zhDisableWarning).FillWidth().FontSize(copySize).LineHeight(copyLine).
				Color(t.LabelSecondary)
		})
		// 失败：停在 failed，错误**一直可见**到重试或取消。
		if state.Kind == ConfirmFailed {
			ui.Text(f, state.Message).FillWidth().FontSize(t.Fonts.Caption).Color(t.Error)
		}
		// footer：取消（outline）+ 确认停用（primary），saving 期间两个都禁用。
		ui.Row(f).FillWidth().Items(ui.Center).Justify(ui.End).Gap(8).Children(func() {
			cancel := ui.Button(f, zhDisableCancel).
				Outline().Small()
			if saving {
				cancel = cancel.Disabled(true)
			}
			if cancel.Clicked() {
				p.team.CancelDisable(req.SessionID)
			}
			confirm := ui.Button(f, zhDisableConfirm).Small()
			if saving {
				confirm = confirm.Disabled(true)
			}
			if confirm.Clicked() {
				p.team.ConfirmDisable()
			}
		})
	})

	// 点遮罩关闭（放在最后画，命中优先级低于对话框本体）。
	if backdrop.Clicked() {
		p.team.CancelDisable(req.SessionID)
	}
}
