// launcher.go —— 父容器 A：设置启动器里的账号/通知入口
// （官方 `ui-settings-general` 的 `SettingsRoot.triggerRow` +
// `ui-settings-account` 的 `AccountMenu`，两棵树合起来才是
// `settings.launcher.notifications` 的**父级**）。
//
// # 官方层级（逐层对得上，不许跳级挂载）
//
//	sidebar.settings                    ui-settings-general index.ts:237（children 声明）
//	└─ triggerRow                       SettingsRoot.tsx:222（display:flex）
//	   ├─ renderSlot('settings.launcher')   SettingsRoot.tsx:226
//	   │  └─ AccountMenu                 ui-settings-account index.ts:316 → AccountMenu.tsx:24
//	   │     ├─ div.notifications        AccountMenu.tsx:60（**hidden={!wide}**）
//	   │     │  └─ renderSlot('settings.launcher.notifications',
//	   │     │       {active: signedIn && !onboarding, openAccount})   ← 本插件的槽
//	   │     └─ Menu 触发器（avatar 24 + label）  AccountMenu.tsx:62-72
//	   ├─ ConnectionIndicator            （未实现，登记为差距）
//	   └─ DesktopUpdateIndicator         （未实现，登记为差距）
//
// 官方把"何时可见"交给父级：`active` 由账号菜单给，`wide` 决定整段 `hidden`。
// 本文件就是那两级父容器的 Go 复刻——**第三层（通知卡片）仍是本插件已有的
// `notifications.go` painter**，本文件只负责给它一个真实的座位与可见性输入。
//
// # 几何（`AccountMenu.module.css` 全量，逐条对应）
//
//	.root{position:relative; flex:1; min-width:0}
//	.root[data-inset='true']{margin-inline:2px}
//	.notifications{position:absolute; inset-inline:0; bottom:calc(100% + 4px); z-index:1}
//	.trigger{height:44px; gap:8px; padding:6px; radius:md; font-size:14px; transparent}
//	.trigger[data-collapsed='true']{36×36; justify-content:center; gap:0; padding:0}
//	.trigger[data-signed-out='true']{height:32px; line-height:20px}
//	.avatar{width:24px; height:24px; border-radius:50%; background:bg-skeleton}
//
// # 两条"不编"
//
//  1. **未授权**（宿主没有账号状态机 / 查询失败）→ 通知座位整段不激活，
//     控制器不轮询，界面不画空卡片；启动器仍按官方**登出态**（32px「更多」行）画——
//     这是官方自己就有的降级形状，不是我们编的。
//  2. **数据缺失**（账号状态未知）不当成"未登录"存档：`AccountState.SignedIn`
//     由壳按"确定已登录"才置真，未知一律假 → 座位隐藏，**不写"未登录"文案**。
package dsnotifications

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/vivaui/viva/flex"
)

// 父级槽位名（官方逐字）。
const (
	// SlotSidebarSettings 是官方 `sidebar.settings`：`ui-settings-general` 把
	// SettingsRoot 注册在这里（`index.ts:237` 的 children 声明），侧栏底部那一行。
	SlotSidebarSettings = pluginapi.Slot("sidebar.settings")
	// SlotSettingsLauncher 是官方 `settings.launcher`：账号菜单（AccountMenu）的挂点。
	// 官方由 `ui-settings-account/index.ts:316` 往这里注册，`SettingsRoot.tsx:226` 渲染。
	SlotSettingsLauncher = pluginapi.Slot("settings.launcher")
)

// 官方几何（`AccountMenu.module.css` 逐条，单位和取值都不自选）。
const (
	// LauncherRowHeight 是 `.trigger{height:44px}`（已登录、宽态那一行）。
	LauncherRowHeight float32 = 44
	// LauncherRowSignedOut 是 `.trigger[data-signed-out='true']{height:32px}`。
	LauncherRowSignedOut float32 = 32
	// LauncherCollapsed 是 `.trigger[data-collapsed='true']{36×36}`。
	LauncherCollapsed float32 = 36
	// LauncherAvatar 是 `.avatar{width:24px;height:24px}`。
	LauncherAvatar float32 = 24
	// LauncherGap 是 `.trigger{gap:8px}`。
	LauncherGap float32 = 8
	// LauncherRowPad 是 `.trigger{padding:6px}`。
	LauncherRowPad float32 = 6
	// LauncherFontSize 是 `.trigger{font-size:14px}`。
	LauncherFontSize float32 = 14
	// LauncherInset 是 `.root[data-inset='true']{margin-inline:2px}`。
	LauncherInset float32 = 2
	// SeatGap 是 `.notifications{bottom:calc(100% + 4px)}` 的 4px。
	SeatGap float32 = 4
	// LauncherWideMin 是"侧栏是宽态还是收起轨"的判据下限。
	//
	// 官方这个值是设置外壳给的运行时 props `wide`；本仓没有那个外壳，
	// 所以由**壳**按侧栏真实宽度折算（`cmd/app` 的 `launcherWide`）后经
	// `AccountState.Wide` 传进来。这个常量只做兜底：没有壳注入时，
	// 侧栏宽度 ≥ 该值即宽态（官方 SIDEBAR_MIN=264，收起轨 56/0 都远小于它）。
	LauncherWideMin float32 = 200
)

// 官方文案（`settings.account` 语言域里本容器用到的三个键，逐字）。
const (
	zhAccountSignedIn = "已登录"
	zhAccountMore     = "更多"
	zhAccountMenu     = "账号"
	// zhAccountNoSettings 是"点开了但设置面板还没接线"时如实说的话。
	zhAccountNoSettings = "设置面板还没有就绪（ds-settings 未装配）"
)

// settingsKey 是 `plugins/dssettings` 提供的"打开设置浮层"能力键。
//
// 不 import dssettings（插件之间不互相 import）：只按契约键取一个 `func() bool`。
const settingsKey = "ui.settings.open"

// officialSlotsLive 报告**官方父链这一帧会不会被 dispatch**。
//
// 为什么不是只判档位：官方骨架在旧部件还占槽时（`BridgeLegacy`）会整段换成
// 旧三栏排版，`sidebar.settings` **不会被 dispatch**，`sidebar.footer` 才会。
// 只看档位就会把画挂在永远不 dispatch 的槽上——界面空白且不报错（真机踩过）。
// 判据唯一出处是 `uikernel.ShellTier.OfficialSlotsLive`。
func (p *Plugin) officialSlotsLive() bool { return p.tier.OfficialSlotsLive() }

// AccountStateSnapshot 取当前账号态（父级/测试用）。
func (c *Controller) AccountStateSnapshot() AccountState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// LauncherOpenSettings 执行"打开账号设置"（官方 `openSettings('account')`）。
//
// 返回 false = 设置面板没接线；调用方据此**如实提示**，不假装跳过去了。
func (p *Plugin) LauncherOpenSettings(env *pluginapi.Env) bool {
	if p.settings == nil {
		if env != nil && env.Act.Notify != nil {
			env.Act.Notify(zhAccountNoSettings)
		}
		return false
	}
	ok := p.settings()
	if !ok && env != nil && env.Act.Notify != nil {
		env.Act.Notify(zhAccountNoSettings)
	}
	return ok
}

// launcherRowHeight 是这一帧启动器行的高度（官方三档，来自 AccountState）。
//
// 一处定义、两处用（父级触发器行 + 本行）：分成两处算就会出现"座位贴着一半高度"。
func launcherRowHeight(st AccountState) float32 {
	switch {
	case !st.Wide:
		return LauncherCollapsed // [data-collapsed] 36×36
	case !st.SignedIn:
		return LauncherRowSignedOut // [data-signed-out] 32px「更多」行
	default:
		return LauncherRowHeight // 已登录 44px
	}
}

// drawSettingsLauncherRow 是**第一级父容器**：官方 SettingsRoot 的 `triggerRow`。
//
// 它只做一件事：把 `settings.launcher` 槽真的 dispatch 出来（`renderSlot`）。
// 官方这一行里还有 ConnectionIndicator / DesktopUpdateIndicator：连接指示由
// dsstatusbar 画在同一行（`sidebar.settings` 槽的第二个子项），更新指示本仓没有
// （登记为差距），所以这里不画它，也不留占位空白。
//
// **不再按档位让位**：壳现在在**所有**绘制路径下都画「设置行」那一行
// （uikernel 官方骨架 / uikernel 兼容桥 / dsshell 旧三栏），本容器是那一行的
// 第一个子项——`flex:1` 把它撑开、把连接指示推到行右缘（官方
// `[设置按钮(flex:1)] [连接指示]` 的排法）。
func (p *Plugin) drawSettingsLauncherRow(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	f.Column().FillWidth().Children(func() {
		drawSlot(SlotSettingsLauncher)
	})
}

// drawLauncher 是**第二级父容器**：官方 `AccountMenu`。
//
// 两件事：
//
//  1. 通知座位 `.notifications`：`position:absolute; inset-inline:0;
//     bottom:calc(100% + 4px)`，`hidden={!wide}`。座位里 **无条件** dispatch
//     `settings.launcher.notifications`（官方也是无条件 `renderSlot`，
//     可见性由被挂组件自己按 `active` 判）；这保证了"槽真被画到"而不是
//     "只在表里登记过"。
//  2. 启动器触发行：avatar 24 + label；`data-collapsed` 36×36；登出 32px。
//     点击 = 官方 `openSettings('account')`（本仓打开设置浮层，见差距登记）。
func (p *Plugin) drawLauncher(f *flex.Frame, env *pluginapi.Env) {
	st := p.ctrl.AccountStateSnapshot()
	t := p.theme
	if env != nil && env.Theme != nil {
		t = env.Theme
	}
	h := launcherRowHeight(st)

	root := f.Column().FillWidth().Height(h)
	if st.SignedIn && st.Wide {
		// `.root[data-inset='true']{margin-inline:2px}`（官方只在已登录 + 宽态时加）
		root = root.MarginSides(0, LauncherInset, 0, LauncherInset)
	}
	root.Children(func() {
		p.drawLauncherTrigger(f, env, t, st, h)
	})
}

// drawLauncherTrigger 画触发行本体（`.trigger`）。
func (p *Plugin) drawLauncherTrigger(f *flex.Frame, env *pluginapi.Env, t *pluginapi.Theme,
	st AccountState, h float32) {
	id := env.Tree.ID(env.Feat, "account-launcher")

	// 官方三档几何只在**这一处**决定：宽态 6px 内边距 + 8px 间距；收起轨
	// 36×36 居中、无内边距无间距（`[data-collapsed='true']`）。
	pad, gap, justify := LauncherRowPad, LauncherGap, flex.Start
	if !st.Wide {
		pad, gap, justify = 0, 0, flex.Center
	}
	row := f.Row().FillWidth().Height(h).Items(flex.Center).Justify(justify).
		Gap(gap).PadSides(pad, pad, pad, pad).
		Radius(t.Radii.MD).Hit(id, zhAccountMenu)

	// 子盒子必须**挂在 row 里**（`row.Children`）：直接 `f.Row()` 会变成 row 的
	// 兄弟，命中区就空了、文字会掉到行下面一行（真机截图踩过：行是空的、
	// 文案跑到状态栏外）。
	row.Children(func() {
		if st.SignedIn {
			// `.avatar` 24×24 圆形；官方 AccountAvatar 在 profile 未就绪时也是一枚
			// 骨架色圆（不编造姓名/头像 URL）。
			f.Row().Size(LauncherAvatar, LauncherAvatar).Radius(LauncherAvatar / 2).
				Background(t.BgLayer2)
		} else {
			// 登出态官方用 `IconEllipsisOutlineMedium size=14`：一个居中的省略号。
			f.Row().Size(LauncherAvatar, LauncherAvatar).Items(flex.Center).
				Justify(flex.Center).Children(func() {
				f.Text("⋯").FontSize(LauncherFontSize).LineHeight(LauncherFontSize).
					Color(t.LabelTertiary)
			})
		}
		if !st.Wide {
			// 收起轨：官方 36×36 只留图标，不给 label（`{wide && <span …>}`）。
			return
		}
		label := zhAccountMore
		line := LauncherFontSize + 6 // 登出态 `.trigger[data-signed-out] line-height:20px`
		if st.SignedIn {
			// 官方：profile 未 ready 时 label = t('signedIn')（不编名字）。
			label = zhAccountSignedIn
			line = LauncherFontSize + 4 // 已登录行 14px 字 → 18px 行高
		}
		f.Text(label).FontSize(LauncherFontSize).LineHeight(line).
			Color(t.LabelPrimary).Ellipsis().Grow(1)
	})

	if row.Clicked() {
		p.LauncherOpenSettings(env)
	}
}

// drawLauncherSeat 是 `.notifications` 座位本体：脱离文档流、贴着启动器行上沿。
//
// 官方 `z-index:1` 的语义是"盖在同一行的其它元素之上"，在 viva 里由
// "absolute 子节点在本容器 in-flow 之后绘制"天然满足。
func (p *Plugin) drawLauncherSeat(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	st := p.ctrl.AccountStateSnapshot()
	if !st.Wide {
		// 官方 `hidden={!wide}`：侧栏收起成轨时整段不渲染（不是"画了看不见"）。
		return
	}
	h := launcherRowHeight(st)
	f.Column().Absolute().Left(0).Right(0).Bottom(h + SeatGap).Children(func() {
		drawSlot(SlotSettingsLauncherNotifications)
	})
}

// launcherWide 是壳侧折算 `wide` 的唯一出处（`cmd/app` 也调它）。
//
// 判据只看侧栏真实宽度：官方 `wide` 是设置外壳的运行时 props，本仓没有那个外壳，
// 所以等价物是"侧栏不是收起轨"。宽度未知（0）时**不当成收起**——那会把整块
// 通知静默藏掉，是"不知道"被当成"没有"。
func launcherWide(env *pluginapi.Env) bool {
	if env == nil {
		return true
	}
	return LauncherWide(env.Geometry.SidebarWidth)
}

// registerLauncherSlots 把两级的槽位补进官方槽表（fail-loud、幂等）。
//
// `sidebar.settings` 已在 uikernel 的内置槽表里（`OffSidebarSettings`），
// 所以这里只登记官方新增的 `settings.launcher`，并把
// `settings.launcher.notifications` 的父槽从 `root` 纠正为真实的 `settings.launcher`。
func registerLauncherSlots() error {
	if _, ok := uikernel.LookupOfficialSlot(SlotSettingsLauncher); !ok {
		if err := uikernel.RegisterOfficialSlot(uikernel.SlotSpec{
			Name:   SlotSettingsLauncher,
			Kind:   uikernel.KindSingle,
			Scope:  uikernel.ScopeRoot,
			Parent: uikernel.OffSidebarSettings,
			Host:   "SettingsRoot.triggerRow",
			Anchor: "ui-settings-general/src/client/SettingsRoot.tsx:226",
			Note: "single/root；官方 sidebar.settings 的 children（ui-settings-general/index.ts:237），" +
				"ui-settings-account 把 AccountMenu 注册在这里（index.ts:316）。" +
				"本仓由 dsnotifications 的父容器 A 实现（launcher.go）",
		}); err != nil {
			return err
		}
	}
	return nil
}

// containerNoop 是"纯容器" painter 的 Draw：不画像素，只让内核的
//
//	"没有槽位或没有 Draw 的画" → fail-loud
//
// 校验通过。真正的活儿在同一个 painter 的 `Children` 里（dispatch 子槽）。
func containerNoop(*flex.Frame, *pluginapi.Env) {}

// LauncherWide 是壳侧折算"侧栏宽态"的唯一出处（与 launcherWide 同源）。
//
// 壳没有 `Env`（那是绘制期对象），但 `AccountState.Wide` 必须由它给：
// 所以把判据导出成纯函数，两边都调它，别各写一份阈值。
func LauncherWide(sidebarWidth float32) bool {
	if sidebarWidth <= 0 {
		// 宽度未知（0）**不当作收起**：那会把整块通知静默藏掉，
		// 是"不知道"被当成"没有"。
		return true
	}
	return sidebarWidth >= LauncherWideMin
}
