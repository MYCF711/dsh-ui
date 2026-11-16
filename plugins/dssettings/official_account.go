// official_account.go —— 官方 AccountSection（账号资料、余额与登录尝试操作）。
// 数据源是 account.*，不是 settings.describe；几何/文案对照 AccountSection.tsx、
// AccountSection.module.css 与 locales.ts 的 zh 表。
package dssettings

import (
	"math/big"
	"net/url"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

const (
	accountPageGap   float32 = 16
	accountCardPadY  float32 = 12
	accountCardPadX  float32 = 16
	accountAvatar    float32 = 32
	accountRowHeight float32 = 40
	accountRowGap    float32 = 8
	accountNameSize  float32 = 14
	accountTextSize  float32 = 13
	accountTextLine  float32 = 22
	accountAmount    float32 = 18
	accountAmountLn  float32 = 28
)

func accountSignedIn(snapshot apiclient.AccountSnapshot) bool {
	return snapshot.View != nil && snapshot.View.SignedIn()
}

func accountAttemptActive(snapshot apiclient.AccountSnapshot) bool {
	return snapshot.View != nil && snapshot.View.ActiveSignIn()
}

func accountStatus(snapshot apiclient.AccountSnapshot) string {
	if snapshot.Failed || snapshot.LoginFailed || snapshot.View != nil && snapshot.View.Attempt != nil && snapshot.View.Attempt.Phase == "failed" {
		return "操作未完成，请重试。"
	}
	if snapshot.View != nil && snapshot.View.Attempt != nil {
		switch snapshot.View.Attempt.Phase {
		case "expired":
			return "登录已过期，请重试。"
		case "initializing":
			return "正在发起登录…"
		case "waiting-browser":
			return "请在浏览器中继续"
		case "exchanging", "committing":
			return "正在完成登录…"
		}
	}
	if accountSignedIn(snapshot) {
		if snapshot.Profile != nil && snapshot.Profile.Status == "ready" && snapshot.Profile.Value != nil && snapshot.Profile.Value.Contact != nil {
			return *snapshot.Profile.Value.Contact
		}
		if snapshot.Profile == nil && !snapshot.ProfileFailed {
			return "加载中…"
		}
		return "账号资料暂不可用"
	}
	return "登录 DeepSeek 账号以开始使用"
}

func accountName(snapshot apiclient.AccountSnapshot) string {
	if !accountSignedIn(snapshot) {
		return "尚未登录"
	}
	if snapshot.Profile != nil && snapshot.Profile.Status == "ready" && snapshot.Profile.Value != nil && snapshot.Profile.Value.Name != nil {
		return *snapshot.Profile.Value.Name
	}
	return "已登录 DeepSeek"
}

// formatAccountBalance 逐条实现官方 formatBalance.ts：两位小数、sub-cent、分组与精度保持。
func formatAccountBalance(amount, symbol string) (string, bool) {
	if symbol != "¥" && symbol != "$" {
		return "", false
	}
	amount = strings.TrimSpace(amount)
	if amount == "" || strings.Contains(amount, "/") {
		return "", false
	}
	value, ok := new(big.Rat).SetString(amount)
	if !ok {
		return "", false
	}
	if value.Sign() == 0 {
		return symbol + "0.00", true
	}
	hundredth := big.NewRat(1, 100)
	if value.Sign() < 0 {
		absolute := new(big.Rat).Abs(value)
		if absolute.Cmp(hundredth) < 0 {
			return "-" + symbol + "0.01", true
		}
		return "-" + symbol + formatAccountCents(absolute, true), true
	}
	if value.Cmp(hundredth) < 0 {
		return "<" + symbol + "0.01", true
	}
	return symbol + formatAccountCents(value, false), true
}

// formatAccountCents 正数 roundDown；负数路径传绝对值并 roundHalfUp（Big.toFixed 默认）。
func formatAccountCents(value *big.Rat, halfUp bool) string {
	scaled := new(big.Rat).Mul(value, big.NewRat(100, 1))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(scaled.Num(), scaled.Denom(), remainder)
	if halfUp && new(big.Int).Lsh(new(big.Int).Abs(remainder), 1).Cmp(scaled.Denom()) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	digits := quotient.String()
	if len(digits) < 3 {
		digits = strings.Repeat("0", 3-len(digits)) + digits
	}
	integer, fraction := digits[:len(digits)-2], digits[len(digits)-2:]
	for i := len(integer) - 3; i > 0; i -= 3 {
		integer = integer[:i] + "," + integer[i:]
	}
	return integer + "." + fraction
}

func formatAccountWallets(wallets []apiclient.AccountWallet) (string, bool) {
	parts := make([]string, 0, len(wallets))
	for _, wallet := range wallets {
		symbol := "$"
		if wallet.Currency == "CNY" {
			symbol = "¥"
		} else if wallet.Currency != "USD" {
			return "", false
		}
		formatted, ok := formatAccountBalance(wallet.Balance, symbol)
		if !ok {
			return "", false
		}
		parts = append(parts, formatted)
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, " + "), true
}

func accountBonusVisible(snapshot apiclient.AccountSnapshot) bool {
	if snapshot.Balance == nil {
		return true
	}
	for _, wallet := range snapshot.Balance.BonusWallets {
		value, ok := new(big.Rat).SetString(wallet.Balance)
		if !ok {
			// 非法金额要落入“前往开放平台查看”，不能静默隐藏整行。
			return true
		}
		if value.Sign() > 0 {
			return true
		}
	}
	return false
}

func (p *Plugin) accountBusySnapshot() bool {
	o := p.ensureOfficial()
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.accountBusy
}

// runAccountAction 在后台执行真实宿主动作； busy 防重入，失败不伪造成功。
func (p *Plugin) runAccountAction(env *pluginapi.Env, action func() error) {
	if action == nil {
		return
	}
	o := p.ensureOfficial()
	o.mu.Lock()
	if o.accountBusy {
		o.mu.Unlock()
		return
	}
	o.accountBusy = true
	o.mu.Unlock()
	go func() {
		err := action()
		o.mu.Lock()
		o.accountBusy = false
		notify := o.notify
		o.mu.Unlock()
		if notify == nil {
			notify = env.Act.Notify
		}
		if notify != nil {
			if err != nil {
				notify("操作未完成，请重试: " + err.Error())
			} else {
				notify("账号状态已更新")
			}
		}
	}()
}

func accountExternal(env *pluginapi.Env, destination string) {
	if destination == "" || env.Act.OpenExternal == nil {
		return
	}
	if err := env.Act.OpenExternal(destination); err != nil && env.Act.Notify != nil {
		env.Act.Notify("打开链接失败: " + err.Error())
	}
}

func accountButton(f ui.C, env *pluginapi.Env, id, label string, primary, disabled bool, action func()) {
	button := ui.Button(f, label).Height(36).Hit(env.Tree.ID(env.Feat, id), label).Disabled(disabled)
	if !primary {
		button = button.Outline()
	}
	if button.Clicked() && !disabled && action != nil {
		action()
	}
}

func (p *Plugin) drawAccountPage(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, a AreaRect, snapshot apiclient.AccountSnapshot) {
	ui.Column(f).Absolute().Left(a.X).Top(a.Y).Width(a.Width).Height(a.Height).
		PadSides(OptionsPadTop, OptionsPadRight, OptionsPadBottom, OptionsPadLeft).
		Gap(accountPageGap).Children(func() {
		if !accountSignedIn(snapshot) && !accountAttemptActive(snapshot) {
			ui.Column(f).FillWidth().Height(120).Justify(ui.Center).Items(ui.Center).Gap(20).Children(func() {
				ui.Text(f, "当前未登录 DeepSeek Harness 账号").FontSize(accountNameSize).LineHeight(accountTextLine).Color(t.LabelPrimary)
				description := "登录 DeepSeek Harness 账号获取专属 API Key"
				if snapshot.Failed || snapshot.LoginFailed {
					description = "操作未完成，请重试。"
				} else if snapshot.View == nil {
					description = "加载中…"
				}
				ui.Text(f, description).FontSize(12).LineHeight(18).Color(t.LabelTertiary)
				accountButton(f, env, "account-sign-in", "登录", true, snapshot.View == nil || p.accountBusySnapshot() || env.Act.AccountStartSignIn == nil, func() {
					p.runAccountAction(env, env.Act.AccountStartSignIn)
				})
			})
			return
		}

		// identity card：官方 avatar/name/status/accountInfo。
		ui.Row(f).FillWidth().PadSides(accountCardPadY, accountCardPadX, accountCardPadY, accountCardPadX).
			Radius(t.Radii.XL).Background(t.BgLayer3).Border(1, t.BorderL2).Items(ui.Center).Gap(16).Children(func() {
			ui.Row(f).Items(ui.Center).Gap(10).Grow(1).MinWidth(0).Children(func() {
				// 头像图片加载器尚未接入：保留官方 32px skeleton，不画姓名首字冒充头像。
				ui.Row(f).Size(accountAvatar, accountAvatar).Radius(accountAvatar / 2).Background(t.BgLayer2)
				ui.Column(f).Grow(1).MinWidth(0).Children(func() {
					ui.Text(f, accountName(snapshot)).FontSize(accountNameSize).Weight(500).Color(t.LabelPrimary).Ellipsis()
					ui.Text(f, accountStatus(snapshot)).FontSize(accountTextSize).LineHeight(accountTextLine).Color(t.LabelTertiary).Ellipsis()
				})
			})
			if accountSignedIn(snapshot) && snapshot.View.Links.UsageURL != "" && env.Act.OpenExternal != nil {
				if parsed, err := url.Parse(snapshot.View.Links.UsageURL); err == nil && parsed.Scheme != "" && parsed.Host != "" {
					accountButton(f, env, "account-info", "更多账号信息", false, false, func() {
						accountExternal(env, parsed.Scheme+"://"+parsed.Host+"/")
					})
				}
			}
		})

		if accountAttemptActive(snapshot) {
			attempt := snapshot.View.Attempt
			ui.Row(f).FillWidth().Justify(ui.End).Gap(10).Children(func() {
				if attempt.AuthorizeURL != "" && env.Act.OpenExternal != nil {
					accountButton(f, env, "account-open-authorize", "打开浏览器", false, false, func() {
						accountExternal(env, attempt.AuthorizeURL)
					})
				}
				accountButton(f, env, "account-cancel-sign-in", "取消", false, p.accountBusySnapshot() || attempt.Phase == "committing" || env.Act.AccountCancelSignIn == nil, func() {
					p.runAccountAction(env, func() error { return env.Act.AccountCancelSignIn(attempt.ID) })
				})
			})
		}

		// balanceCard：充值余额、可选赠金余额、usage/top-up。
		ui.Column(f).FillWidth().PadSides(accountCardPadY, accountCardPadX, accountCardPadY, accountCardPadX).
			Radius(t.Radii.XL).Background(t.BgLayer3).Border(1, t.BorderL2).Gap(accountRowGap).Children(func() {
			drawAccountBalanceRow(f, t, "充值余额", snapshot.Balance == nil, snapshot.BalanceFailed, snapshot.Balance)
			if accountSignedIn(snapshot) && accountBonusVisible(snapshot) {
				if p.prims != nil {
					p.prims.Divider(f)
				}
				drawAccountBonusRow(f, t, snapshot)
			}
			if p.prims != nil {
				p.prims.Divider(f)
			}
			ui.Row(f).FillWidth().MinHeight(accountRowHeight).PadSides(6, 0, 6, 0).Items(ui.Center).Children(func() {
				ui.Text(f, "更多").FontSize(accountTextSize).Color(t.LabelTertiary).Grow(1)
				if snapshot.View != nil && snapshot.View.Links.UsageURL != "" && env.Act.OpenExternal != nil {
					accountButton(f, env, "account-usage", "查询用量", false, false, func() { accountExternal(env, snapshot.View.Links.UsageURL) })
				}
				if snapshot.View != nil && snapshot.View.Links.TopUpURL != "" && env.Act.OpenExternal != nil {
					accountButton(f, env, "account-top-up", "充值", true, false, func() { accountExternal(env, snapshot.View.Links.TopUpURL) })
				}
			})
		})
	})
}

func drawAccountBalanceRow(f ui.C, t *pluginapi.Theme, title string, loading, failed bool, balance *apiclient.AccountBalance) {
	ui.Row(f).FillWidth().MinHeight(accountRowHeight).PadSides(6, 0, 6, 0).Items(ui.Center).Children(func() {
		ui.Text(f, title).FontSize(accountTextSize).Color(t.LabelPrimary).Grow(1)
		text, amount := "", false
		if loading && !failed {
			text = "加载中…"
		} else if balance != nil && balance.Status == "ready" {
			text, amount = formatAccountWallets(balance.Value)
		}
		if text == "" {
			text = "前往开放平台查看"
		}
		if amount {
			ui.Text(f, text).FontSize(accountAmount).LineHeight(accountAmountLn).Weight(510).Color(t.LabelPrimary)
		} else {
			ui.Text(f, text).FontSize(accountTextSize).Color(t.LabelTertiary)
		}
	})
}

func drawAccountBonusRow(f ui.C, t *pluginapi.Theme, snapshot apiclient.AccountSnapshot) {
	ui.Row(f).FillWidth().MinHeight(accountRowHeight).PadSides(6, 0, 6, 0).Items(ui.Center).Children(func() {
		ui.Text(f, "赠金余额").FontSize(accountTextSize).Color(t.LabelPrimary).Grow(1)
		if snapshot.Balance == nil && !snapshot.BalanceFailed {
			ui.Text(f, "加载中…").FontSize(accountTextSize).Color(t.LabelTertiary)
			return
		}
		var positive []apiclient.AccountWallet
		if snapshot.Balance != nil {
			for _, wallet := range snapshot.Balance.BonusWallets {
				if value, ok := new(big.Rat).SetString(wallet.Balance); ok && value.Sign() > 0 {
					positive = append(positive, wallet)
				}
			}
		}
		text, amount := formatAccountWallets(positive)
		if text == "" {
			text = "前往开放平台查看"
		}
		if amount {
			ui.Text(f, text).FontSize(accountAmount).LineHeight(accountAmountLn).Weight(510).Color(t.LabelPrimary)
		} else {
			ui.Text(f, text).FontSize(accountTextSize).Color(t.LabelTertiary)
		}
	})
}
