// Package dsnotifications 复刻官方 rc.2 新增包
// `@deepseek-ai/dsh-client-ui-account-notifications`（1 个槽位 + NoticeCard 叠层）。
//
// 官方事实（`structure/official-account-notifications-tree.md`，rc.2 src 原件行号）：
//
//	槽位      settings.launcher.notifications（single / scope root，**由
//	          ui-settings-account 的账号菜单声明并渲染**，本包只往里塞卡片）
//	组件      AccountNotifications → (active &&) AccountStack → NoticeCard[]
//	数据      宿主 account/getNotifications、account/getNotificationContent、
//	          account/reportNotificationEvent（本仓宿主已实现，见 DSH-GO
//	          internal/account/notifications.go 与 handlers_account_official.go）
//	语言域    account.notifications，**只有 close 一个键**；标题/正文/按钮文案
//	          全部来自服务端（§6 数据形状 AccountNotification）
//
// # 三条"不许自造"（结构树 §8 原文）
//
//  1. 卡片宽度上限 280px、圆角 16、padding 20、叠层比例 252/260、每层上移 4px、
//     250ms 过渡——按 §4 取值，不自选。
//  2. 只有前景卡（depth=0）可交互/可关闭；非前景卡 `pointer-events:none`，
//     **不许**做成可点击列表。
//  3. 动作只有"打开账号设置"与"关闭"两种；不得新增"稍后提醒 / 全部已读"。
//
// # 本仓的落地方式（先落槽位，再落内容）
//
// 官方把"何时可见"交给父级（账号菜单的 `active`）与控制器（列表是否为空）；
// 这里同样：本包只注册 `settings.launcher.notifications` 一块画，
// 可见性 = 控制器 Active（父级给的运行时 props）**且** 至少有一条未过期通知。
//
// 数据面由 `Controller` 持有：壳（或账号菜单插件）调 `Update(AccountState{...})`
// 驱动 Start/End，调 `Shown/CTAClicked/Dismiss` 上报交互。取数与上报全部经
// `internal/apiclient` 的三个官方方法（R7：界面不直连平台）。
//
// # 当前缺口（如实登记，不静默）
//
// 父槽 `settings.launcher.notifications` 的**声明方**（官方 `ui-settings-account`
// 的账号菜单）在本仓 GUI 里还没有对应实现——本仓的设置面是 `plugins/dssettings`
// 的浮层，没有账号菜单。因此本包目前只完成"槽位注册 + 内容渲染 + 可见性"，
// 真正挂到屏幕上的接线有两条待办（报告 §未实现项）：
//
//  1. 父级在账号菜单里 `drawSlot(dsnotifications.SlotSettingsLauncherNotifications)`
//  2. 壳/账号插件把 *apiclient.Client 提供为 "ui.apiclient" 并每帧喂 Controller
package dsnotifications

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
)

// SlotSettingsLauncherNotifications 是官方 rc.2 的槽位名（逐字）。
//
// 官方声明方是 `ui-settings-account/src/client/index.ts:316`（children 声明）、
// 渲染方是 `AccountMenu.tsx:60`（包在 `hidden={!wide}` 里，侧栏收起即整块隐藏）。
//
// 本仓 `pluginapi.Slot` 是写死常量的闭集（`pluginapi/plugin.go`），而官方这两个
// 新槽位登记在"需要 GA 补的类型"清单里（见报告）。这里用既有 API——字符串转换成
// `pluginapi.Slot`——先落注册点，等 `pluginapi` 补上常量后换成常量即可，行为不变。
const SlotSettingsLauncherNotifications = pluginapi.Slot("settings.launcher.notifications")

// ServiceKey 是本插件提供的账号通知运行时（值是 *Controller）。
//
// 壳/账号菜单插件 Inject 它来驱动生命周期（Update）与交互上报
// （Shown / CTAClicked / Dismiss）。本包不自己定时轮询"当前会话"以外的状态：
// 官方把 begin/end 交给账号态变化驱动（结构树 §5.5）。
const ServiceKey = "ui.account.notifications"

// RemoteKey 是可选注入：远程实现（*apiclient.Client 直接满足 Remote）。
//
// 没注入 = 未授权/未接线：控制器保持空态，界面**整块不渲染**（不是画一个空卡片）。
const RemoteKey = "ui.apiclient"

// Plugin 是 `ds-account-notifications` 插件。
type Plugin struct {
	theme *pluginapi.Theme
	ctrl  *Controller
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-account-notifications" }

// Provides 提供通知运行时（父级/壳据此驱动生命周期）。
func (*Plugin) Provides() []string { return []string{ServiceKey} }

// Requires 只要求主题令牌：数据面是**可选**注入，缺了就是空态，不是装配失败。
func (*Plugin) Requires() []string { return []string{pluginapi.KeyTheme} }

// Register 注入主题与可选的远程实现，并把运行时提供给下游。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	// 先把槽位登记进 uikernel 的 rc.2 官方槽表（fail-loud：登记失败就不装配）。
	if err := RegisterOfficialSlot(); err != nil {
		return err
	}
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)

	// 壳可能先提供一份现成控制器（测试/热接），否则自己造一份。
	if v := ctx.Inject(ServiceKey); v != nil {
		if ctrl, ok := v.(*Controller); ok {
			p.ctrl = ctrl
		}
	}
	if p.ctrl == nil {
		p.ctrl = NewController(nil)
	}
	if v := ctx.Inject(RemoteKey); v != nil {
		if remote, ok := v.(Remote); ok {
			p.ctrl.SetRemote(remote)
		}
	}
	p.ctrl.SetTheme(p.theme)

	ctx.Provide(ServiceKey, p.ctrl)
	return nil
}

// Panels 实现 pluginapi.Panels：只挂官方那一个槽。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{
		ID:   "account-notices",
		Slot: SlotSettingsLauncherNotifications,
		// 单槽 single：同槽再有别人挂也是各自画，Order 0 保持稳定顺序。
		Order: 0,
		Draw:  p.draw,
	}}
}

// Controller 返回本插件持有的通知运行时（测试与父级用）。
func (p *Plugin) Controller() *Controller { return p.ctrl }

// draw 画叠层。可见性三连（官方 §5.1）：
//
//	① 父级没给 active（账号菜单没打开）→ 整块不渲染；
//	② 没有未过期通知 → 整块不渲染（不是画空壳）；
//	③ 未授权（控制器没跑起来）→ 整块不渲染。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	if p.ctrl == nil || !p.ctrl.Visible() {
		return
	}
	t := p.theme
	if env.Theme != nil {
		t = env.Theme
	}
	p.drawStack(f, env, t, p.ctrl.Notices())
}
