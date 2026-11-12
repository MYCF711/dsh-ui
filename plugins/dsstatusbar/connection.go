// connection.go —— 连接指示归**官方座位**：`sidebar.settings` 的 `SettingsRoot.triggerRow`。
//
// # 官方挂在哪里（逐字）
//
// `ui-settings-general/src/client/SettingsRoot.tsx` 的 `SettingsRoot` 返回的第一段：
//
//	<div className={clsx(SettingsRoot_module_css_default.triggerRow, !wide && …railRow)}>
//	  {renderSlot('settings.launcher', {…})}                     ← 账号启动器（dsnotifications）
//	  <ConnectionIndicator
//	     state={wide && … ? connectionIndicator : undefined}
//	     disconnectedLabel={t('connection.error')}
//	     connectingLabel={t('connection.connecting')}
//	     recoveredLabel={t('connection.connected')}
//	     reconnectActionLabel={t('connection.reconnect')}
//	     restartActionLabel={t('connection.restart')} onReconnect={reconnect} />
//	  <DesktopUpdateIndicator … />
//	</div>
//
// 也就是：**侧栏 footArea 最后一行（设置行）的右侧**，和设置启动器同一行。
// 结构树 §4 S8 的判定同一句：「连接指示由 settings 的连接状态驱动，不能移到底部
// 自创 statusbar」。旧实现把它画在 `shell.bottom` 的整行里，是错层。
//
// # 几何（官方 `SettingsRoot.module.css` + primitives 的 CSS，逐条）
//
//	.triggerRow{position:relative;flex:none;display:flex;align-items:center;gap:8px;width:100%;margin:4px 0}
//	.trigger{height:42px;…}                       ← 同一行里的设置按钮，决定行高
//	._indicator{flex:none;display:inline-grid;grid-template-columns:14px max-content;
//	            align-items:center;column-gap:4px;height:28px;padding:0 8px;box-sizing:border-box;
//	            border:1px solid transparent;border-radius:var(--dsw-radius-sm);
//	            font-size:12px;font-weight:500;line-height:18px;white-space:nowrap}
//	._warning{background:state-warn-tertiary;color:state-warn-label;
//	          border-color:color-mix(in srgb,state-warn-label 20%,transparent)}
//	._success{background:state-success-tertiary;color:state-success-primary;
//	          border-color:color-mix(in srgb,state-success-primary 20%,transparent)}
//	._icon{display:grid;place-items:center;width:14px;height:14px}
//
// # 状态取值：真值只有一处，缺事实就写「未知」
//
// 官方那五个 props 由 `useConnectionState` + `showRecovery`/`holdConnecting` 算出来；
// 本仓没有那条框架钩子，等价事实由壳在**真实 apiclient 回调**里写进
// `Snapshot.Link`（`pluginapi/link.go` 有完整说明：订阅成功 = LinkConnected，
// 订阅失败 = LinkDown，还没订阅过 = LinkUnknown）。
//
// 于是三态映射（前两条是官方文案逐字，第三条是**本仓的诚实扩展**）：
//
//	LinkConnected → success 色 + `check-outline`  + 「已连接」
//	LinkDown      → warning 色 + `refresh-outline` + 「连接异常，刷新重试」  ← connection.error 原句
//	LinkUnknown   → 中性描边 + 无图标             + 「未知」                ← derived（官方无 unknown 态）
//
// **两处与官方的显式差异（都登记在报告里，不假装对齐）：**
//
//  1. 官方在**连接正常时不渲染**（`connectionIndicator` 为 undefined →
//     `ConnectionIndicator` 返回 null），只在 connecting/disconnected/recovered
//     三种异常或恢复瞬间出现。本任务书要求「连接指示在 official 档渲染 + 连/断两态」，
//     所以 connected 常驻显示，复用官方 success 色调与 `connection.connected` 文案。
//  2. 官方的 warning 态是 `<button onClick={onReconnect}>`（aria-label
//     `connection.reconnect`/`connection.restart`）。本仓**没有重连动作**
//     （壳的 mux 订阅自带 1s→30s 退避重试），所以 warning 态画成**状态**而不是死按钮——
//     "画一个点了没反应的按钮"比不画更坏。等壳接上真 reconnect 后再升级成按钮。
package dsstatusbar

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// 官方几何（逐条对应上面的 CSS；值与官方一致，不另取整）。
const (
	// `SettingsRoot.module.css` 的 `.trigger{height:42px}` —— triggerRow 的行高。
	connRowHeight float32 = 42
	// `.triggerRow{margin:4px 0}`。
	connRowMarginY float32 = 4
	// `.triggerRow{gap:8px}`。
	connRowGap float32 = 8
	// `._indicator{height:28px}`。
	connIndicatorHeight float32 = 28
	// `._indicator{padding:0 8px}`。
	connIndicatorPadX float32 = 8
	// `._indicator{column-gap:4px}`。
	connIndicatorGap float32 = 4
	// `._indicator{font-size:12px;font-weight:500;line-height:18px}`。
	connIndicatorFont float32 = 12
	connIndicatorLine float32 = 18
	// `._indicator{border:1px solid transparent}` + `color-mix(… 20%, transparent)`。
	connIndicatorBorder     float32 = 1
	connIndicatorBorderTint float32 = 0.2
)

// 座位顺序。
//
// `sidebar.settings`（single/root）上，dsnotifications 的账号启动器占 Order 60
// （官方它就是 `settings.launcher` 那一级）；指示器排在它之后 61——官方这两者
// 在**同一行**（triggerRow 的 flex 子项），本仓只能另画一行，所以位置取"紧邻其下"，
// 报告里登记为剩余差距（等 settings 触发行/同一行容器接上后合并）。
const (
	connSettingsOrder = 61
	// legacy 桥：`sidebar.footer` 里 dssidebar 的设置行(30)、文件(40)、
	// schedule(50)、通知(55)、deliverables(60)、jobs(70) 之后，取 80 让它成为
	// footer 区域**最后一行**——官方 footArea 的顺序就是「footerActions 在上、settings 在下」。
	connFooterBridgeOrder = 80
)

// 官方文案（`ui-settings-general/src/client/locales.ts` 的 zh 段，逐字）。
const (
	// zhConnectionError 是 `connection.error`：**可见文案**（不是 aria）。
	zhConnectionError = "连接异常，刷新重试"
	// zhConnectionConnected 是 `connection.connected`（官方 recovered 态的可见文案）。
	zhConnectionConnected = "连接成功"
	// zhConnectionUnknown 是 derived：官方没有 unknown 态，壳还没拿到事实时如实说。
	zhConnectionUnknown = "未知"
)

// 可见文案的**导出别名**：真机验收（`cmd/app`）与插件测试读同一份，
// 免得两边各写一份中文字符串断言（写重了就会漂移）。
const (
	// OfficialConnectedLabel 是连着时的可见文案（官方 `connection.connected`）。
	OfficialConnectedLabel = zhConnectionConnected
	// OfficialDisconnectedLabel 是断开时的可见文案（官方 `connection.error`）。
	OfficialDisconnectedLabel = zhConnectionError
	// OfficialUnknownLabel 是缺事实时的可见文案（derived）。
	OfficialUnknownLabel = zhConnectionUnknown
)

// connectionTone 是官方 `._success` / `._warning` 之外的第三档（中性，derived）。
type connectionTone uint8

const (
	connToneNeutral connectionTone = iota
	connToneSuccess
	connToneWarning
)

// connectionView 是这一帧要画的那一段（色调 + 文案 + 图标）。
type connectionView struct {
	tone  connectionTone
	label string
	icon  func() ui.Icon // nil = 这一档没有官方 glyph
}

// connectionViewOf 把壳的三态事实翻成要画的东西。
//
// 这里是**唯一**把「连接状态」变成「文案/颜色」的地方：别处不许再拿
// `Disconnected` 或 `Status` 字符串猜状态（旧实现按 `!Disconnected` 写「就绪」，
// 那是"没有断线证据"被当成"连着"，正是任务书点名要拆掉的那句"编已连接"）。
func connectionViewOf(link pluginapi.LinkState) connectionView {
	switch link {
	case pluginapi.LinkConnected:
		return connectionView{tone: connToneSuccess, label: zhConnectionConnected, icon: checkOutlineIcon}
	case pluginapi.LinkDown:
		return connectionView{tone: connToneWarning, label: zhConnectionError, icon: refreshOutlineIcon}
	default:
		return connectionView{tone: connToneNeutral, label: zhConnectionUnknown}
	}
}

// officialSlotsLive 报告官方子槽这一帧会不会被 dispatch（判据唯一出处是壳）。
func (p *Plugin) officialSlotsLive() bool { return p.tier.OfficialSlotsLive() }

// drawConnectionSettingsSeat 是官方座位（`sidebar.settings`）上的那一行。
func (p *Plugin) drawConnectionSettingsSeat(f *flex.Frame, env *pluginapi.Env) {
	if !p.officialSlotsLive() {
		return // 整列还在兼容桥上：官方槽不会被 dispatch，画在这里等于画在空气里
	}
	p.drawConnectionRow(f, env)
}

// drawConnectionFooterBridge 是兼容桥：整列还是旧三栏时只有 `sidebar.footer` 会被 dispatch。
//
// 与 dsnotifications 的 `account-launcher.bridge` 同一条判据、同一条理由：
// 官方骨架在旧部件占槽时会整段换成旧三栏排版（`ShellTier.LegacyBridge`），
// 那时 `sidebar.settings` 根本不会 dispatch，只按档位判就会把画挂在永远不画的座上。
func (p *Plugin) drawConnectionFooterBridge(f *flex.Frame, env *pluginapi.Env) {
	if p.officialSlotsLive() {
		return // 官方座位活着：桥这一帧不画（同帧只有一个活，不双画）
	}
	p.drawConnectionRow(f, env)
}

// drawConnectionRow 画官方 `.triggerRow` 里属于连接指示的那一段。
//
// `Justify(flex.End)`：官方这一行是 `[settings 按钮(flex:1)] [连接指示] [更新指示]`，
// 指示器因此贴在**行的右缘**。本仓的设置按钮是 dssettings 的 `settings.trigger`
// 座位（今天它只画模态、不画触发行），所以这里用一个"右侧对齐"的等价位置：
// 横向位置与官方一致（右缘），左半个 flex:1 的位置留给将来的设置按钮。
func (p *Plugin) drawConnectionRow(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	if env.Theme != nil {
		t = env.Theme
	}
	view := connectionViewOf(env.Data.Link)
	f.Row().FillWidth().Height(connRowHeight).
		MarginSides(connRowMarginY, 0, connRowMarginY, 0).
		Items(flex.Center).Gap(connRowGap).Justify(flex.End).Children(func() {
		p.drawConnectionIndicator(f, t, view)
	})
}

// drawConnectionIndicator 画官方 `ConnectionIndicator`（28 高胶囊：图标 14 + 文案 12/18）。
//
// 三档色调都按官方那两条 CSS：背景 = 该色系的 tertiary、文字 = label/primary、
// 描边 = 同色 20%（`color-mix(… 20%, transparent)` 的等价写法是给颜色乘 0.2 alpha）。
// 中性档（unknown）官方没有，取透明的底 + `border-l2` 描边 + `label-tertiary` 文字，
// 这样它读起来是"一个状态"，不会被误认成成功/失败的彩色结论。
func (p *Plugin) drawConnectionIndicator(f *flex.Frame, t *pluginapi.Theme, view connectionView) {
	fill := ui.Color{}
	text := t.LabelTertiary
	border := t.BorderL2
	switch view.tone {
	case connToneSuccess:
		fill = t.SuccessTertiary
		text = t.SuccessPrimary
		border = withAlpha(t.SuccessPrimary, connIndicatorBorderTint)
	case connToneWarning:
		fill = t.WarnTertiary
		text = t.WarnLabel
		border = withAlpha(t.WarnLabel, connIndicatorBorderTint)
	}
	f.Row().Height(connIndicatorHeight).Radius(t.Radii.SM).
		Background(fill).Border(connIndicatorBorder, border).
		Items(flex.Center).Gap(connIndicatorGap).Children(func() {
		// `padding:0 8px` 用定宽占位盒表达（不是 PadSides）：
		// 胶囊宽 = 内容 pref + 两侧 padding，用 PadSides 时文字拿到的是"容器宽 - pad"，
		// float32 往返会让刚好放得下的文案被判断成放不下（HC 真机报告里那条
		// 「571 tok/s 被截成 571 tok…」的同一个坑）。
		f.Row().Width(connIndicatorPadX).Height(connIndicatorHeight)
		if view.icon != nil {
			f.Icon(view.icon()).Size(connectionIconSize, connectionIconSize).Color(text)
		}
		// 官方 `font-weight:500`；viva 的 Weight 是**相对常规字重的增量**，
		// `ui.WeightMedium` 就是这一档（别处也都用它，别写 500）。
		f.Text(view.label).FontSize(connIndicatorFont).LineHeight(connIndicatorLine).
			Weight(ui.WeightMedium).Color(text).Ellipsis()
		f.Row().Width(connIndicatorPadX).Height(connIndicatorHeight)
	})
}

// withAlpha 是官方 `color-mix(in srgb, C 20%, transparent)` 的等价写法。
func withAlpha(c ui.Color, alpha float32) ui.Color {
	c.Alpha *= alpha
	return c
}
