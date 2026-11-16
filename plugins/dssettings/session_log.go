// session_log.go —— 官方 ui-settings-session-log 的 UploadToast。
//
// general Switch 的注册台账在 official_tree.go；这里只负责它写入后的
// shell.overlay/session-log-upload-toast：成功/失败文案逐字取官方 zh locale，
// 通知在设置面板关闭后仍然有效，并在时间窗后自动 dismiss。
package dssettings

import (
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

const (
	zhSessionLogSaved  = "设置已保存"
	zhSessionLogFailed = "无法保存设置"

	// 与本仓既有官方反馈 Toast 的成功/失败停留窗口一致；dismiss 语义与官方
	// UploadToast 的 Toast(onDone=dismiss) 相同。
	sessionLogSavedToastWindow  = 3 * time.Second
	sessionLogFailedToastWindow = 5 * time.Second
)

// updateSessionLogWrite 把这一行的 mutation 终态投影成 UploadToast。
// saving 只负责清掉上一条通知；busy 的真值仍是 officialPanel.write。
func (p *Plugin) updateSessionLogWrite(state WriteState) {
	if state.NS != apiclient.SettingsNSSessionLog {
		return
	}
	o := p.ensureOfficial()
	o.mu.Lock()
	o.sessionLogGeneration++
	generation := o.sessionLogGeneration
	if o.sessionLogToastTimer != nil {
		o.sessionLogToastTimer.Stop()
		o.sessionLogToastTimer = nil
	}
	o.sessionLogToast = ""
	o.sessionLogToastSuccess = false
	var window time.Duration
	switch {
	case state.Kind == WriteSaving:
		o.sessionLogBusy = true
		// 新一次 mutation 开始时撤下旧结果，不让旧成功覆盖当前 busy。
	case state.Success():
		o.sessionLogBusy = false
		o.sessionLogToast = zhSessionLogSaved
		o.sessionLogToastSuccess = true
		window = sessionLogSavedToastWindow
	case state.Failure():
		o.sessionLogBusy = false
		o.sessionLogToast = zhSessionLogFailed
		window = sessionLogFailedToastWindow
	default:
		o.mu.Unlock()
		return
	}
	if o.sessionLogToast != "" {
		o.sessionLogToastTimer = time.AfterFunc(window, func() {
			p.expireSessionLogToast(generation)
		})
	}
	o.mu.Unlock()
}

// expireSessionLogToast 实现 Toast(onDone=dismiss)；过期回调只使通知失效，
// 不清 WriteState（面板里仍能看到本次失败原因）。
func (p *Plugin) expireSessionLogToast(generation uint64) {
	o := p.ensureOfficial()
	o.mu.Lock()
	if generation != o.sessionLogGeneration {
		o.mu.Unlock()
		return
	}
	o.sessionLogToast = ""
	o.sessionLogToastSuccess = false
	o.sessionLogToastTimer = nil
	notify := o.notify
	if notify == nil {
		notify = p.notify
	}
	o.mu.Unlock()
	// Actions.Notify 在壳里同时承担 Invalidate；空文本不编造一条新状态。
	if notify != nil {
		notify("")
	}
}

// sessionLogBusySnapshot 独立于面板的共享 WriteState：其它 namespace 的写入
// 不会把 Session Log 的 busy 意外盖掉。
func (p *Plugin) sessionLogBusySnapshot() bool {
	o := p.ensureOfficial()
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.sessionLogBusy
}

// sessionLogToastSnapshot 是绘制/测试共享的一次原子读。
func (p *Plugin) sessionLogToastSnapshot() (string, bool) {
	o := p.ensureOfficial()
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.sessionLogToast, o.sessionLogToastSuccess
}

// drawSessionLogToast 是 shell.overlay/session-log-upload-toast 的 Viva 绘制。
func (p *Plugin) drawSessionLogToast(f ui.C, env *pluginapi.Env) {
	if env == nil {
		return
	}
	text, success := p.sessionLogToastSnapshot()
	if text == "" {
		return
	}
	t := p.theme
	if env.Theme != nil {
		t = env.Theme
	}
	width := env.Canvas.TextWidth(text, t.Fonts.Secondary) + 32
	if width < 160 {
		width = 160
	}
	left := (env.Bounds.Width - width) / 2
	top := env.Bounds.Height - 120
	if top < 0 {
		top = 0
	}
	accent := t.Error
	if success {
		accent = t.Success
	}
	ui.Row(f).Absolute().Left(left).Top(top).Width(width).Height(40).Radius(t.Radii.MD).
		Background(t.BgLayer3).Border(1, accent).Items(ui.Center).Justify(ui.Center).
		Hit(env.Tree.ID(env.Feat, "session-log-upload-toast"), text).
		Children(func() {
			ui.Text(f, text).FontSize(t.Fonts.Secondary).Color(t.LabelPrimary)
		})
}
