// account_menu.go —— AccountMenu 的登录/登出动作、官方两个确认对话框，以及
// 「意见反馈」（`contactUs`，见 contact.go）。通知卡片仍在 notifications.go。
package dsnotifications

import (
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/ui"
)

const (
	zhAccountSettings = "设置"
	zhAccountSignIn   = "登录"
	zhAccountSignOut  = "退出登录"
	zhAccountCancel   = "取消"
	zhAccountFailed   = "操作未完成，请重试。"
)

type accountMenuEntry struct {
	id       string
	label    string
	disabled bool
}

// accountMenuEntries 只声明真实接通的项目。
//
// 官方顺序（AccountMenu.tsx:74-78）是 settings → contact → (signout | signin)：
// 「意见反馈」**不看登录态**（未登录也能提反馈）。它只在壳接了 `OpenExternal` 时
// 才画——画一行点了没反应的菜单，比不画更坏。addApiKey 仍未接线，继续不画。
func accountMenuEntries(state AccountState, busy bool, contact bool) []accountMenuEntry {
	out := []accountMenuEntry{{id: "settings", label: zhAccountSettings}}
	if contact {
		out = append(out, accountMenuEntry{id: "contact", label: zhAccountContact})
	}
	if state.SignedIn {
		out = append(out, accountMenuEntry{id: "signout", label: zhAccountSignOut, disabled: busy})
	} else {
		out = append(out, accountMenuEntry{id: "signin", label: zhAccountSignIn, disabled: busy})
	}
	return out
}

func (p *Plugin) accountSnapshot(env *pluginapi.Env) apiclient.AccountSnapshot {
	if env != nil && env.Act.AccountSnapshot != nil {
		return env.Act.AccountSnapshot()
	}
	return apiclient.AccountSnapshot{}
}

func (p *Plugin) drawAccountMenu(f ui.C, anchor ui.Box, env *pluginapi.Env, state AccountState) {
	p.accountUIMu.Lock()
	if p.accountMenuOpen == nil {
		open := false
		p.accountMenuOpen = &open
	}
	open := p.accountMenuOpen
	busy := p.accountSignInBusy || p.accountSignOutBusy
	p.accountUIMu.Unlock()
	// 「意见反馈」要有地方可开才画（官方那行背后是 `window.open`，永远可开；
	// 本仓的等价物是壳的 `OpenExternal`，没接就退回不画）。
	contact := env != nil && env.Act.OpenExternal != nil
	ui.Menu(f, anchor, open, func(items *ui.MenuItems) {
		if items == nil {
			return
		}
		for _, entry := range accountMenuEntries(state, busy, contact) {
			entry := entry
			if items.Item(entry.label).Disabled(entry.disabled).Clicked() && !entry.disabled {
				p.chooseAccountMenu(entry.id, env)
			}
		}
	})
}

func (p *Plugin) toggleAccountMenu() {
	p.accountUIMu.Lock()
	defer p.accountUIMu.Unlock()
	if p.accountMenuOpen == nil {
		open := false
		p.accountMenuOpen = &open
	}
	*p.accountMenuOpen = !*p.accountMenuOpen
}

func (p *Plugin) closeAccountMenu() {
	p.accountUIMu.Lock()
	if p.accountMenuOpen != nil {
		*p.accountMenuOpen = false
	}
	p.accountUIMu.Unlock()
}

func (p *Plugin) chooseAccountMenu(id string, env *pluginapi.Env) {
	p.closeAccountMenu()
	switch id {
	case "settings":
		p.LauncherOpenSettings(env)
	case "contact":
		// 官方 `onSelect` 的顺序是「先关菜单、再打开问卷」（AccountMenu.tsx:83）：
		// closeAccountMenu 已经在上面做了。
		p.openContactUs(env)
	case "signin":
		p.beginAccountSignIn(env)
	case "signout":
		p.requestAccountSignOut(env)
	}
}

func (p *Plugin) beginAccountSignIn(env *pluginapi.Env) {
	if env == nil || env.Act.AccountStartSignIn == nil {
		return
	}
	p.accountUIMu.Lock()
	if p.accountSignInBusy {
		p.accountUIMu.Unlock()
		return
	}
	p.accountSignInBusy = true
	p.accountUIMu.Unlock()
	go func() {
		err := env.Act.AccountStartSignIn()
		p.accountUIMu.Lock()
		p.accountSignInBusy = false
		p.accountUIMu.Unlock()
		p.notifyAccountResult(env, err)
	}()
}

func (p *Plugin) notifyAccountResult(env *pluginapi.Env, err error) {
	if env == nil || env.Act.Notify == nil {
		return
	}
	if err != nil {
		env.Act.Notify(zhAccountFailed + " " + err.Error())
	} else {
		env.Act.Notify("账号状态已更新")
	}
}

// requestAccountSignOut 先读运行任务，再打开官方三态确认框。
func (p *Plugin) requestAccountSignOut(env *pluginapi.Env) {
	if env == nil || env.Act.AccountHasRunningTasks == nil {
		return
	}
	p.accountUIMu.Lock()
	if p.accountSignOutBusy {
		p.accountUIMu.Unlock()
		return
	}
	p.accountSignOutBusy = true
	p.accountUIMu.Unlock()
	go func() {
		running, err := env.Act.AccountHasRunningTasks()
		impact := "unknown"
		if err == nil && running {
			impact = "running"
		} else if err == nil {
			impact = "idle"
		}
		p.accountUIMu.Lock()
		p.accountSignOutBusy = false
		p.accountSignOutImpact = impact
		p.accountSignOutFailed = false
		p.accountUIMu.Unlock()
		if env.Act.Notify != nil {
			env.Act.Notify("请确认退出登录")
		}
	}()
}

func (p *Plugin) confirmAccountSignOut(env *pluginapi.Env) {
	if env == nil || env.Act.AccountSignOut == nil {
		return
	}
	p.accountUIMu.Lock()
	if p.accountSignOutBusy {
		p.accountUIMu.Unlock()
		return
	}
	p.accountSignOutBusy = true
	p.accountSignOutFailed = false
	p.accountUIMu.Unlock()
	go func() {
		err := env.Act.AccountSignOut()
		p.accountUIMu.Lock()
		p.accountSignOutBusy = false
		p.accountSignOutFailed = err != nil
		if err == nil {
			p.accountSignOutImpact = ""
		}
		p.accountUIMu.Unlock()
		p.notifyAccountResult(env, err)
	}()
}

func (p *Plugin) dismissAccountSignIn(env *pluginapi.Env, snapshot apiclient.AccountSnapshot) {
	if env == nil || env.Act.AccountShowLogin == nil {
		return
	}
	p.accountUIMu.Lock()
	busy := p.accountSignInBusy
	p.accountUIMu.Unlock()
	if busy || snapshot.View != nil && snapshot.View.Attempt != nil && snapshot.View.Attempt.Phase == "committing" {
		return
	}
	if snapshot.View != nil && snapshot.View.ActiveSignIn() && snapshot.View.Attempt != nil && env.Act.AccountCancelSignIn != nil {
		id := snapshot.View.Attempt.ID
		p.accountUIMu.Lock()
		p.accountSignInBusy = true
		p.accountUIMu.Unlock()
		go func() {
			err := env.Act.AccountCancelSignIn(id)
			if err == nil {
				env.Act.AccountShowLogin(false)
			}
			p.accountUIMu.Lock()
			p.accountSignInBusy = false
			p.accountUIMu.Unlock()
			p.notifyAccountResult(env, err)
		}()
		return
	}
	env.Act.AccountShowLogin(false)
}

func (p *Plugin) drawAccountDialogs(f ui.C, env *pluginapi.Env) {
	snapshot := p.accountSnapshot(env)
	if snapshot.LoginVisible {
		p.drawAccountSignInDialog(f, env, snapshot)
	}
	p.accountUIMu.Lock()
	impact := p.accountSignOutImpact
	p.accountUIMu.Unlock()
	if impact != "" {
		p.drawAccountSignOutDialog(f, env, impact)
	}
}

func (p *Plugin) drawAccountSignInDialog(f ui.C, env *pluginapi.Env, snapshot apiclient.AccountSnapshot) {
	open := true
	active := snapshot.View != nil && snapshot.View.ActiveSignIn()
	committing := snapshot.View != nil && snapshot.View.Attempt != nil && snapshot.View.Attempt.Phase == "committing"
	failed := snapshot.LoginFailed || snapshot.Failed || snapshot.View != nil && snapshot.View.Attempt != nil && snapshot.View.Attempt.Phase == "failed"
	expired := snapshot.View != nil && snapshot.View.Attempt != nil && snapshot.View.Attempt.Phase == "expired"
	title, description := "开始使用", "登录 DeepSeek 账号，或添加 API Key，即可开始使用。你的项目和文件保存在本地。"
	if failed {
		title, description = "登录失败", zhAccountFailed
	} else if active {
		title, description = "等待登录", "没有自动打开新页面？请复制登录链接，在浏览器中完成登录。"
	} else if expired {
		title, description = "登录已超时", "请重新登录后继续操作。"
	}
	ui.ModalWith(f, &open, title, ui.ModalSpec{Description: description, Width: 420}, func() {
		if snapshot.View != nil && snapshot.View.Attempt != nil && snapshot.View.Attempt.AuthorizeURL != "" && env.Act.CopyResult != nil {
			if ui.Button(f, "复制登录链接").Outline().Small().Clicked() {
				if err := env.Act.CopyResult(snapshot.View.Attempt.AuthorizeURL); err != nil && env.Act.Notify != nil {
					env.Act.Notify("复制失败: " + err.Error())
				} else if env.Act.Notify != nil {
					env.Act.Notify("链接已复制")
				}
			}
		}
	}, func() {
		ui.Row(f).FillWidth().Justify(ui.End).Gap(10).Children(func() {
			p.accountUIMu.Lock()
			busy := p.accountSignInBusy
			p.accountUIMu.Unlock()
			if active {
				secondary := ui.Button(f, zhAccountCancel).Outline().Disabled(busy || committing)
				if secondary.Clicked() {
					p.dismissAccountSignIn(env, snapshot)
				}
			}
			label := zhAccountSignIn
			if active {
				label = "请在浏览器中继续"
			} else if expired || failed {
				label = "重新登录"
			}
			primary := ui.Button(f, label).Disabled(busy || committing || active || env.Act.AccountStartSignIn == nil)
			if primary.Clicked() {
				p.beginAccountSignIn(env)
			}
		})
	})
	if !open {
		p.dismissAccountSignIn(env, snapshot)
	}
}

func (p *Plugin) drawAccountSignOutDialog(f ui.C, env *pluginapi.Env, impact string) {
	open := true
	description := "退出登录不会丢失任何数据，你仍可以登录此账号。"
	switch impact {
	case "running":
		description = "当前有进程正在执行中，点击“退出登录”，将会打断进程，是否即刻退出登录？"
	case "unknown":
		description = "暂时无法确认任务状态。退出登录可能会中断使用此账号的任务，是否继续？"
	}
	ui.ModalWith(f, &open, zhAccountSignOut, ui.ModalSpec{Description: description, Width: 420}, func() {
		p.accountUIMu.Lock()
		failed := p.accountSignOutFailed
		p.accountUIMu.Unlock()
		if failed {
			ui.Text(f, zhAccountFailed).Color(p.theme.Error)
		}
	}, func() {
		ui.Row(f).FillWidth().Justify(ui.End).Gap(10).Children(func() {
			p.accountUIMu.Lock()
			busy := p.accountSignOutBusy
			p.accountUIMu.Unlock()
			if ui.Button(f, zhAccountCancel).Outline().Disabled(busy).Clicked() && !busy {
				p.accountUIMu.Lock()
				p.accountSignOutImpact = ""
				p.accountUIMu.Unlock()
			}
			if ui.Button(f, zhAccountSignOut).Disabled(busy).Clicked() && !busy {
				p.confirmAccountSignOut(env)
			}
		})
	})
	if !open {
		p.accountUIMu.Lock()
		if !p.accountSignOutBusy {
			p.accountSignOutImpact = ""
		}
		p.accountUIMu.Unlock()
	}
}

// accountUIStateSnapshot 是测试读取异步状态的统一入口。
func (p *Plugin) accountUIStateSnapshot() (signInBusy, signOutBusy bool, impact string, failed bool) {
	p.accountUIMu.Lock()
	defer p.accountUIMu.Unlock()
	return p.accountSignInBusy, p.accountSignOutBusy, p.accountSignOutImpact, p.accountSignOutFailed
}
