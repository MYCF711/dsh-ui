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
//	            font-size:12px;font-weight:500;line-height:18px;white-space:nowrap;
//	            transition:background-color .16s ease-out,color .16s ease-out,
//	                       border-color .16s ease-out,opacity .15s ease-out;
//	            animation:_indicator-enter .15s ease-out}
//	._leaving{opacity:0}
//	._warning{background:state-warn-tertiary;color:state-warn-label;
//	          border-color:color-mix(in srgb,state-warn-label 20%,transparent);cursor:pointer}
//	._warning:hover{background:color-mix(in srgb,state-warn-tertiary,state-warn-primary 6%)}
//	._warning:active{background:color-mix(in srgb,state-warn-tertiary,state-warn-primary 10%)}
//	._success{background:state-success-tertiary;color:state-success-primary;
//	          border-color:color-mix(in srgb,state-success-primary 20%,transparent)}
//	._icon{display:grid;place-items:center;width:14px;height:14px}
//
// **行容器由壳画，本文件只画行里的一个子项**（官方 `._indicator{flex:none}`）：
// `internal/uikernel/shell_skeleton.go` 的 `SettingsRoot.triggerRow` 那个 Row 把
// `sidebar.settings` 槽上的所有占用方摆进同一行（设置按钮 flex:1 + 连接指示）。
//
// # 状态取值：真值只有一处，缺事实就写「未知」
//
// 官方那五个 props 由 `useConnectionState` + `showRecovery`/`holdConnecting` 算出来；
// 本仓没有那条框架钩子，等价事实由壳在**真实 apiclient 回调**里写进
// `Snapshot.Link`（`pluginapi/link.go` 有完整说明）：
//
//	LinkConnecting → 一次订阅尝试正在飞行中（首连 / 退避到点 / 用户点了立即重连）
//	LinkConnected  → 订阅成功
//	LinkDown       → 订阅失败（壳按 1s→30s 退避重试）
//	LinkUnknown    → 还没拿到任何事实
//
// 于是四态映射（前两条是官方文案逐字，后两条见下面的「显式差异」）：
//
//	LinkConnected  → success 色 + `check-outline`   + 「连接成功」
//	LinkDown       → warning 色 + `refresh-outline` + 「连接异常，刷新重试」  ← connection.error 原句
//	LinkConnecting → warning 色 + loading 弧       + 「重新连接中…」        ← connection.connecting 原句
//	LinkUnknown    → 中性描边 + 无图标             + 「未知」              ← derived（官方无 unknown 态）
//
// **warning 两态是按钮**（官方 `<button onClick={onReconnect}>`）：点击走
// `Actions.Reconnect`——壳实现的真实重连（取消当前 sub + 立刻重订），不是假动作。
// `Actions.Reconnect == nil` 时**不画成按钮**（画一个点了没反应的东西比不画更坏），
// 保留 warning 胶囊的静态外观。
//
// # 150ms 淡出（官方 `._leaving{opacity:0}` + `transition:opacity .15s`）
//
// 官方的离场是 React 状态：`state` 变 undefined 后组件再画 150ms（`_leaving`
// 把 opacity 过渡到 0）才卸载。本仓没有渲染循环，等价实现是一个**显式的
// 150ms 离场窗**（`connectionAnim`）：目标态一到，先把**旧态**再画 150ms 并
// 线性淡出（`1 → 0`），150ms 到才换成新态；淡出期间用
// `Actions.Invalidate` 申请下一帧（否则要等下一个外部事件，淡出会卡住）。
//
// # 两处与官方的显式差异（都登记在报告里，不假装对齐）
//
//  1. 官方在**连接正常时不渲染**（`connectionIndicator` 为 undefined →
//     `ConnectionIndicator` 返回 null），只在 connecting / disconnected / recovered
//     三种异常或恢复瞬间出现。本任务书要求「连接指示在 official 档渲染 + 连/断两态」，
//     所以 connected 常驻显示，复用官方 success 色调与 `connection.connected` 文案。
//  2. 官方的 recovered 态有 2s 恢复确认窗（`RECOVERY_CONFIRMATION_MS`），且
//     connecting 态至少驻留 800ms（`CONNECTING_MIN_VISIBLE_MS`）。本仓的
//     `LinkConnecting` 是**真实拨号时长**：拨号比 800ms 短时不会硬拉长
//     （多画 800ms 的"正在连接"就是编一个已经不存在的状态）；恢复确认窗
//     也没做——success 是常驻态，没有再消失的时机。
//     见报告 §剩余差距。
package dsstatusbar

import (
	"time"

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
	// `._warning:hover/:active` 的混色比例（`color-mix(… 6%/10%)`）。
	connWarningHoverTint  float32 = 0.06
	connWarningActiveTint float32 = 0.10
)

// 官方时间常量（逐条对齐 `ui-primitives/src/ConnectionIndicator.module.css` 与
// `ui-settings-general/src/client/SettingsRoot.tsx`）。
const (
	// connLeaveWindow 是官方 `._indicator` 的离场时长（`transition:…opacity .15s`
	// + `._leaving{opacity:0}`）。React 靠 150ms 的 setTimeout 撑住这一帧；本仓靠
	// `Actions.Invalidate` 在 150ms 后排下一帧。
	connLeaveWindow = 150 * time.Millisecond

	// connEnterWindow 是官方入场动画：`._indicator{animation:_indicator-enter .15s ease-out}`
	// + `@keyframes _indicator-enter{0%{opacity:0}}`（opacity 0→1，`.15s ease-out`）。
	// 官方在 `prefers-reduced-motion:reduce` 下把这条 animation 关掉（本仓
	// `ui.Canvas.ReduceMotion()` 为真时跳过，见 `advance`）。
	connEnterWindow = 150 * time.Millisecond

	// connRecoveryWindow 是官方 `RECOVERY_CONFIRMATION_MS = 2_000`：
	// 恢复（disconnected/connecting → connected）后 recovered 胶囊的确认窗。
	// 官方计时从"recovered 胶囊真正可见"开始（连接态最小可见时长会推迟它）。
	connRecoveryWindow = 2 * time.Second

	// connConnectingMinVisible 是官方 `CONNECTING_MIN_VISIBLE_MS = 800`：
	// connecting 胶囊的最小可见时长，避免一次极短的重连尝试闪一下。
	connConnectingMinVisible = 800 * time.Millisecond

	// connFrameInterval 是 spinner 真动画的重绘间隔。官方是 CSS 合成动画
	// （1.5s 线性旋转，由合成器按显示刷新率推进）；Go 侧没有合成器，只能按
	// 计时器申请整帧重绘，30fps（33ms）在 14px 的指示器上肉眼看不出台阶。
	connFrameInterval = 33 * time.Millisecond
)

// 座位顺序。
//
// `sidebar.settings`（single/root）上，dsnotifications 的账号启动器占 Order 60
// （官方它就是 `settings.launcher` 那一级）；指示器排在它之后 61——官方这两者
// 在**同一行**（triggerRow 的 flex 子项）。行容器由壳画
// （`uikernel.drawSidebarSkeleton` 的 `SettingsRoot.triggerRow`）。
const (
	connSettingsOrder = 61
	// legacy 桥：`sidebar.footer` 里 dssidebar 的设置行(30)、文件(40)、
	// schedule(50)、通知(55)、deliverables(60)、jobs(70) 之后。桥现在挂在
	// **`sidebar.settings` 槽**上（Order 80 保序），因为壳在两种档位下都会 dispatch
	// 这一行；`sidebar.footer` 上的其它行照旧。
	connFooterBridgeOrder = 80
)

// 官方文案（`ui-settings-general/src/client/locales.ts` 的 zh 段，逐字）。
const (
	// zhConnectionError 是 `connection.error`：**可见文案**（不是 aria）。
	zhConnectionError = "连接异常，刷新重试"
	// zhConnectionConnected 是 `connection.connected`（官方 recovered 态的可见文案）。
	zhConnectionConnected = "连接成功"
	// zhConnectionConnecting 是 `connection.connecting`：官方可见文案是
	// 「重新连接中」+ 三个逐点出现的省略号（CSS 动画）。本仓渲染静态的「…」，
	// 省略号动画是唯一差异（无头截图拍不到动画，且 viva 没有 per-dot 动画）。
	zhConnectionConnecting = "重新连接中"
	// zhConnectionDots 是官方那三个 dot `<span>` 的静态等价物。
	zhConnectionDots = "…"
	// zhConnectionUnknown 是 derived：官方没有 unknown 态，壳还没拿到事实时如实说。
	zhConnectionUnknown = "未知"
	// zhConnectionReconnect 是 `connection.reconnect`（disconnected 态按钮的 aria-label）。
	zhConnectionReconnect = "连接异常，点击立即重连"
	// zhConnectionRestart 是 `connection.restart`（connecting 态按钮的 aria-label）。
	zhConnectionRestart = "连接中断，正在重试，点击立即重连"
)

// 可见文案的**导出别名**：真机验收（`cmd/app`）与插件测试读同一份，
// 免得两边各写一份中文字符串断言（写重了就会漂移）。
const (
	// OfficialConnectedLabel 是连着时的可见文案（官方 `connection.connected`）。
	OfficialConnectedLabel = zhConnectionConnected
	// OfficialDisconnectedLabel 是断开时的可见文案（官方 `connection.error`）。
	OfficialDisconnectedLabel = zhConnectionError
	// OfficialConnectingLabel 是正在重连时的可见文案（官方 `connection.connecting`）。
	OfficialConnectingLabel = zhConnectionConnecting
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

// connPhase 是这一帧要画的**态**（官方 `ConnectionIndicator` 的 state 枚举 + derived）。
type connPhase uint8

const (
	// connPhaseUnknown 是 derived 的「未知」（官方枚举里没有）。
	connPhaseUnknown connPhase = iota
	// connPhaseConnected 是官方 `recovered` 的常驻化（见文件头「显式差异 1」）。
	connPhaseConnected
	// connPhaseConnecting 是官方 `connecting`（`data-phase=connecting`）。
	connPhaseConnecting
	// connPhaseDown 是官方 `disconnected`。
	connPhaseDown
)

// String 只服务测试失败信息。
func (p connPhase) String() string {
	switch p {
	case connPhaseConnected:
		return "connected"
	case connPhaseConnecting:
		return "connecting"
	case connPhaseDown:
		return "disconnected"
	default:
		return "unknown"
	}
}

// connPhaseOf 把壳的事实翻成要画的态（**唯一**一处做这个映射）。
func connPhaseOf(link pluginapi.LinkState) connPhase {
	switch link {
	case pluginapi.LinkConnected:
		return connPhaseConnected
	case pluginapi.LinkConnecting:
		return connPhaseConnecting
	case pluginapi.LinkDown:
		return connPhaseDown
	default:
		return connPhaseUnknown
	}
}

// officialGlyph 是这一帧要画的官方 glyph（spinner 要按时间算帧，所以不能是
// `func() ui.Icon`；胶囊的两枚图标也用同一套枚举）。全部来自 rc.2 sprite 表：
//
//	glyphCheck    check-outline      ConnectionIndicator（recovered）
//	glyphRefresh  refresh-outline    ConnectionIndicator（disconnected/connecting）
//	glyphSpinner  StateDot ongoing   ConnectionIndicator（connecting）= 1.5s 旋转
//	glyphGauge    gauge-outline      ActivityPill / Activity 弹窗标题
//	glyphDatabase database-outline   UsagePill
type officialGlyph uint8

const (
	glyphNone officialGlyph = iota
	glyphCheck
	glyphRefresh
	glyphSpinner
	glyphGauge
	glyphDatabase
)

// connectionView 是这一帧要画的那一段（色调 + 文案 + 图标 + 可点性）。
type connectionView struct {
	phase  connPhase
	tone   connectionTone
	label  string
	icon   officialGlyph // glyphNone = 这一档没有官方 glyph
	button bool          // 官方 warning 态是 `<button onClick={onReconnect}>`
	aria   string        // 官方 aria-label（`connection.reconnect` / `connection.restart`）
}

// connectionViewOf 把态翻成要画的东西。
//
// 这里是**唯一**把「连接状态」变成「文案/颜色」的地方：别处不许再拿
// `Disconnected` 或 `Status` 字符串猜状态（旧实现按 `!Disconnected` 写「就绪」，
// 那是"没有断线证据"被当成"连着"，正是任务书点名要拆掉的那句"编已连接"）。
func connectionViewOf(phase connPhase) connectionView {
	switch phase {
	case connPhaseConnected:
		return connectionView{phase: phase, tone: connToneSuccess, label: zhConnectionConnected, icon: glyphCheck}
	case connPhaseDown:
		return connectionView{
			phase: phase, tone: connToneWarning, label: zhConnectionError,
			icon: glyphRefresh, button: true, aria: zhConnectionReconnect,
		}
	case connPhaseConnecting:
		return connectionView{
			phase: phase, tone: connToneWarning, label: zhConnectionConnecting + zhConnectionDots,
			icon: glyphSpinner, button: true, aria: zhConnectionRestart,
		}
	default:
		return connectionView{phase: phase, tone: connToneNeutral, label: zhConnectionUnknown}
	}
}

// connectionAnim 是官方 `ConnectionIndicator` + `SettingsRoot` 连接状态机的等价物
// （**唯一**状态源）。它把官方三处 `useLayoutEffect`/`useState` 合成一台：
//
//	官方 ConnectionIndicator           本仓字段
//	shown / target / leaving / EXIT_MS  ready/shown/target/leaving/since
//	._indicator-enter .15s ease-out     entering/enterSince（本单新增）
//	RECOVERY_CONFIRMATION_MS = 2_000    showRecovery/recoverySince（本单新增）
//	CONNECTING_MIN_VISIBLE_MS = 800     connectingShownAt/holdConnecting（本单新增）
//	previousConnectionState ref         prev（恢复窗口的边沿判据）
//
// 时间从 `now` 来（测试注入固定时钟，真机是 `time.Now()`）。
type connectionAnim struct {
	ready   bool
	shown   connPhase
	target  connPhase
	leaving bool
	since   time.Time

	// entering/enterSince 是官方 `_indicator-enter`（opacity 0→1，150ms ease-out）。
	entering   bool
	enterSince time.Time

	// reduceMotion 等价 `@media (prefers-reduced-motion:reduce)`：
	// 官方在那里把 `_indicator` 的 animation 与 transition 都关掉。
	reduceMotion bool

	// prev 是官方 `previousConnectionState` ref：恢复窗口只看边沿。
	prev connPhase

	// connectingShownAt 是官方 `connectingShownAt` ref（零值 = 不在 connecting 态）。
	connectingShownAt time.Time
	// holdConnecting/holdSince/holdFor 是官方 `holdConnecting` + 那条 setTimeout。
	holdConnecting bool
	holdSince      time.Time
	holdFor        time.Duration

	// showRecovery/recoverySince 是官方 `showRecovery` + 2s 确认窗计时。
	// recoverySince 为零 = 窗口还没开始计时（官方：等 holdConnecting 走完才开始）。
	showRecovery  bool
	recoverySince time.Time
}

// advance 推进一步，返回这一帧要画的 (态, 不透明度)。
//
// 不透明度的语义：
//
//	入场（官方 `_indicator-enter .15s ease-out`）  0 → 1，ease-out
//	离场（官方 `._leaving{opacity:0}` + opacity .15s ease-out）  1 → 0，ease-out
//	其余                                           1
//
// 减动效档（`reduceMotion`）下两条动画都跳过（官方 media query 逐字），
// 但**状态机的窗口**照旧推进——官方那里 `showRecovery`/`holdConnecting` 与 CSS 无关。
func (a *connectionAnim) advance(link pluginapi.LinkState, now time.Time) (connPhase, float32) {
	state := connPhaseOf(link)
	if !a.ready {
		a.ready, a.shown, a.target, a.prev = true, state, state, state
		a.startEnter(now)
		// 首帧也要记 connecting 的起点：官方 `connectingShownAt` 是在同一个
		// useLayoutEffect 里写的，漏了它，首帧就 connecting → 下一帧恢复时
		// 800ms 最小可见会被整体跳过（真机首连正好是这条路径）。
		if state == connPhaseConnecting {
			a.connectingShownAt = now
		}
		return a.shown, a.opacity(now)
	}

	// ① 官方 SettingsRoot 的第一个 useLayoutEffect：恢复窗口的开关（只看边沿）。
	if state != a.prev {
		switch {
		case state != connPhaseConnected:
			// 一旦离开 connected，确认窗立刻关（官方 `if (connectionState !== 'connected')`）。
			a.showRecovery, a.recoverySince = false, time.Time{}
		case a.prev == connPhaseDown || a.prev == connPhaseConnecting:
			// 从断开/正在连接恢复 → 开确认窗；计时从胶囊真正可见开始（②③）。
			a.showRecovery, a.recoverySince = true, time.Time{}
		}
		a.prev = state
	}

	// ② 官方第三个 useLayoutEffect：connecting 最小可见时长
	//    （`CONNECTING_MIN_VISIBLE_MS - (Date.now() - connectingShownAt)`）。
	if state == connPhaseConnecting {
		a.connectingShownAt = now
	} else if !a.connectingShownAt.IsZero() {
		elapsed := now.Sub(a.connectingShownAt)
		a.connectingShownAt = time.Time{}
		if remaining := connConnectingMinVisible - elapsed; remaining > 0 {
			a.holdConnecting, a.holdSince, a.holdFor = true, now, remaining
		}
	}
	if a.holdConnecting && now.Sub(a.holdSince) >= a.holdFor {
		a.holdConnecting = false
		if a.showRecovery {
			// 官方在 `setHoldConnecting(false)` 的那一次渲染里开始 2s 计时；
			// 两次绘制之间可能跨过 hold 的截止点，所以按**截止时刻**起算，
			// 而不是按这一帧的 now（否则离散采样会把确认窗推后）。
			a.recoverySince = a.holdSince.Add(a.holdFor)
		}
	}

	// ③ 官方第二个 useLayoutEffect：2s 恢复确认窗（`showRecovery && !holdConnecting`）。
	if a.showRecovery && !a.holdConnecting {
		if a.recoverySince.IsZero() {
			a.recoverySince = now
		} else if now.Sub(a.recoverySince) >= connRecoveryWindow {
			a.showRecovery, a.recoverySince = false, time.Time{}
		}
	} else {
		a.recoverySince = time.Time{}
	}

	// ④ 官方 `connectionIndicator` 的三选一（+ 本仓 connected 常驻这条显式差异）。
	want := a.visiblePhase(state)

	// ⑤ 离场 → 换态 → 入场（官方 React 的 unmount/mount 时序）：
	//    态变了先把旧态按官方 `_leaving` 淡出 150ms，再挂新态（新态带 `_indicator-enter`）。
	if a.leaving {
		if elapsed := now.Sub(a.since); elapsed >= connLeaveWindow {
			// 离场走完 = 官方 React 的 unmount → 挂新组件（新组件带入场动画）。
			a.shown = a.target
			a.leaving = false
			a.startEnter(now)
		}
		return a.shown, a.opacity(now)
	}
	if want != a.shown {
		a.leaving, a.target, a.since = true, want, now
		a.entering = false // 离场期间不跑入场（官方 `_leaving` 与 mount 动画不会同时出现）
		return a.shown, a.opacity(now)
	}
	if a.entering && now.Sub(a.enterSince) >= connEnterWindow {
		a.entering = false
	}
	return a.shown, a.opacity(now)
}

// visiblePhase 是官方 `connectionIndicator` 的取值（connecting 占先、其次 disconnected、
// 再次 recovered），再叠加本仓那条**显式差异**：connected 常驻显示（官方此处返回
// undefined，胶囊短暂出现 2s 后消失；本仓不让"连着"这件事没有画面）。
func (a *connectionAnim) visiblePhase(state connPhase) connPhase {
	switch {
	case state == connPhaseConnecting || a.holdConnecting:
		return connPhaseConnecting
	case state == connPhaseDown:
		return connPhaseDown
	case a.showRecovery:
		return connPhaseConnected
	default:
		return state
	}
}

// startEnter 开始一次官方入场动画（减动效档直接跳过）。
func (a *connectionAnim) startEnter(now time.Time) {
	if a.reduceMotion {
		a.entering, a.enterSince = false, time.Time{}
		return
	}
	a.entering, a.enterSince = true, now
}

// opacity 是这一帧的不透明度（入场/离场两条官方 150ms ease-out 曲线）。
func (a *connectionAnim) opacity(now time.Time) float32 {
	if a.reduceMotion {
		return 1
	}
	if a.leaving {
		progress := float32(now.Sub(a.since)) / float32(connLeaveWindow)
		return 1 - easeOut(progress)
	}
	if a.entering {
		progress := float32(now.Sub(a.enterSince)) / float32(connEnterWindow)
		return easeOut(progress)
	}
	return 1
}

// leavingNow 报告状态机是否正处在离场窗里（兼容旧判据）。
func (a *connectionAnim) leavingNow() bool { return a.ready && a.leaving }

// nextFrameAt 报告壳**下一次**必须重绘的时刻（零值 = 无事可做，不需要排帧）。
//
// 四类事各有截止点：离场窗、入场窗、connecting 最小可见窗、2s 恢复确认窗；另外
// connecting 态的 spinner 是持续动画，按 `connFrameInterval` 连排。
// 测试用固定时钟直接断言这个时刻，避免"动画靠 sleep 才对"。
func (a *connectionAnim) nextFrameAt(now time.Time) (time.Time, bool) {
	if !a.ready {
		return time.Time{}, false
	}
	earliest := time.Time{}
	take := func(at time.Time) {
		if at.IsZero() || at.Before(now) {
			at = now
		}
		if earliest.IsZero() || at.Before(earliest) {
			earliest = at
		}
	}
	if a.leaving {
		take(a.since.Add(connLeaveWindow))
	}
	if a.entering {
		take(a.enterSince.Add(connEnterWindow))
	}
	if a.holdConnecting {
		take(a.holdSince.Add(a.holdFor))
	}
	if a.showRecovery && !a.holdConnecting && !a.recoverySince.IsZero() {
		take(a.recoverySince.Add(connRecoveryWindow))
	}
	// spinner 动画：只要这一帧画的是 connecting 态就继续排。
	if a.shown == connPhaseConnecting && !a.leaving {
		take(now.Add(connFrameInterval))
	}
	if earliest.IsZero() {
		return time.Time{}, false
	}
	return earliest, true
}

// drawConnectionIndicator 画官方 `ConnectionIndicator`（28 高胶囊：图标 14 + 文案 12/18）。
//
// 它只画**一个子项**：行容器（`SettingsRoot.triggerRow` 的 Row）由壳提供，
// 这一格靠 `flex:none` 贴在行右缘（设置按钮 `flex:1` 把它推过去）。
//
// 三档色调都按官方那两条 CSS：背景 = 该色系的 tertiary、文字 = label/primary、
// 描边 = 同色 20%（`color-mix(… 20%, transparent)` 的等价写法是给颜色乘 0.2 alpha）。
// 中性档（unknown）官方没有，取透明的底 + `border-l2` 描边 + `label-tertiary` 文字，
// 这样它读起来是"一个状态"，不会被误认成成功/失败的彩色结论。
func (p *Plugin) drawConnectionIndicator(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	if env.Theme != nil {
		t = env.Theme
	}
	now := time.Now()
	if p.now != nil {
		now = p.now()
	}
	// 官方 `@media (prefers-reduced-motion:reduce)` 由 viva 的 canvas 承担
	// （`ui.Canvas.ReduceMotion()`，桌面壳按操作系统设置注入）。
	p.conn.reduceMotion = env.Canvas != nil && env.Canvas.ReduceMotion()
	phase, opacity := p.conn.advance(env.Data.Link, now)
	// 动画（入场/离场/连接态最小可见/恢复确认窗/spinner 旋转）都要自己排下一帧：
	// 这个仓没有渲染循环，不排就会卡在半透明或停在一帧的角度上
	// （见 pluginapi.Actions.Invalidate 的注释）。
	if next, ok := p.conn.nextFrameAt(now); ok {
		delay := next.Sub(now)
		if delay < 0 {
			delay = 0
		}
		p.scheduleFrameAfter(env.Act.Invalidate, delay)
	}
	view := connectionViewOf(phase)

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

	pill := f.Row().Height(connIndicatorHeight).Radius(t.Radii.SM).
		Background(fill).Border(connIndicatorBorder, border).
		Items(flex.Center).Gap(connIndicatorGap).Opacity(opacity)

	// 官方 warning 态是**可点按钮**（`<button onClick={onReconnect}>`）：
	// 命中区 + hover/active 底色都只在真的接了重连动作时才画。
	interactive := view.button && env.Act.Reconnect != nil
	if interactive {
		pill = pill.Hit(env.Tree.ID(env.Feat, "connection-reconnect"), view.aria).
			Hover(flex.Look{}.Background(mixColor(fill, t.Warn, connWarningHoverTint))).
			Pressed(flex.Look{}.Background(mixColor(fill, t.Warn, connWarningActiveTint)))
	}

	pill.Children(func() {
		// `padding:0 8px` 用定宽占位盒表达（不是 PadSides）：
		// 胶囊宽 = 内容 pref + 两侧 padding，用 PadSides 时文字拿到的是"容器宽 - pad"，
		// float32 往返会让刚好放得下的文案被判断成放不下（HC 真机报告里那条
		// 「571 tok/s 被截成 571 tok…」的同一个坑）。
		f.Row().Width(connIndicatorPadX).Height(connIndicatorHeight)
		p.drawConnectionIcon(f, view.icon, text, t.LabelTertiary, now)
		// 官方 `font-weight:500`；viva 的 Weight 是**相对常规字重的增量**，
		// `ui.WeightMedium` 就是这一档（别处也都用它，别写 500）。
		f.Text(view.label).FontSize(connIndicatorFont).LineHeight(connIndicatorLine).
			Weight(ui.WeightMedium).Color(text).Ellipsis()
		f.Row().Width(connIndicatorPadX).Height(connIndicatorHeight)
	})

	if interactive && pill.Clicked() {
		// 真动作：壳取消当前 sub 并立刻重订（pluginapi.Actions.Reconnect）。
		env.Act.Reconnect()
	}
}

// drawConnectionIcon 画连接指示的官方 glyph（check / refresh / spinner）。
//
// spinner 的颜色按官方 `.spinner{color:var(--dsw-alias-label-tertiary)}` 取
// **label-tertiary**——即使它长在 warning 胶囊里（官方 warning 只染文字与描边，
// `.spinner` 自己那条 color 覆盖了继承色）。
func (p *Plugin) drawConnectionIcon(f *flex.Frame, kind officialGlyph, text, tertiary ui.Color, now time.Time) {
	switch kind {
	case glyphCheck:
		f.Icon(checkOutlineIcon()).Size(connectionIconSize, connectionIconSize).Color(text)
	case glyphRefresh:
		f.Icon(refreshOutlineIcon()).Size(connectionIconSize, connectionIconSize).Color(text)
	case glyphSpinner:
		frame := spinnerFrameAt(now, p.conn.reduceMotion)
		f.Icon(spinnerGlyph(frame, tertiary)).Size(spinnerSize, spinnerSize).Color(tertiary)
	}
}

// scheduleFrameAfter 在 delay 之后申请一帧重绘（幂等：同时只排一个，取最早的那个）。
//
// 计时器回调在另一条 goroutine 上跑，所以只捕获**不可变**的局部值
// （invalidate 函数本身与 delay），不读插件字段——那会与绘制线程竞争（-race 门禁）。
// 已经排过一帧时：新截止点更早才重排（spinner 每帧都会来排，不能无限顺延）。
func (p *Plugin) scheduleFrameAfter(invalidate func(), delay time.Duration) {
	if invalidate == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.frameTimer != nil && p.frameAt.Sub(time.Now()) <= delay {
		return // 已排的那一帧不晚于这次请求：不用重排
	}
	if p.frameTimer != nil {
		p.frameTimer.Stop()
	}
	p.frameAt = time.Now().Add(delay)
	p.frameTimer = time.AfterFunc(delay, func() {
		p.mu.Lock()
		p.frameTimer = nil
		p.mu.Unlock()
		invalidate()
	})
}

// scheduleFrame 是旧的一步版封装（离场窗 150ms），保留给既有调用点。
func (p *Plugin) scheduleFrame(invalidate func()) {
	p.scheduleFrameAfter(invalidate, connLeaveWindow)
}

// withAlpha 是官方 `color-mix(in srgb, C 20%, transparent)` 的等价写法。
func withAlpha(c ui.Color, alpha float32) ui.Color {
	c.Alpha *= alpha
	return c
}

// mixColor 是官方 `color-mix(in srgb, a, b t)` 的等价写法（线性插值各通道）。
func mixColor(a, b ui.Color, t float32) ui.Color {
	return ui.Color{
		Red:   a.Red + (b.Red-a.Red)*t,
		Green: a.Green + (b.Green-a.Green)*t,
		Blue:  a.Blue + (b.Blue-a.Blue)*t,
		Alpha: a.Alpha + (b.Alpha-a.Alpha)*t,
	}
}
