// official_update.go —— 官方 `SettingsRoot.triggerRow` 的**第三个子项**
// `DesktopUpdateIndicator`（桌面更新指示器）。
//
// # 官方出处（逐字）
//
// `ui-settings-general/src/client/SettingsRoot.tsx:222-245`：
//
//	<div className={clsx(SettingsRoot_module_css_default.triggerRow, !wide && …railRow)}>
//	  {renderSlot('settings.launcher', {…})}            ← 账号启动器（dsnotifications）
//	  <ConnectionIndicator state={…} … />              ← 连接指示（dsstatusbar）
//	  <DesktopUpdateIndicator wide={wide} hidden={…} t={t} view={desktopUpdate}
//	                          onOpen={openDesktopUpdate} />
//	</div>
//
// 也就是行里的第 **3** 个子项。它**不是 slot**：官方把它硬写在 SettingsRoot 里，
// 所以本插件（设置面 owner）画它，与设置触发器同一个座位（`sidebar.settings`），
// Order 排在连接指示（61）之后。
//
// # 可见性（官方逐条，这是"什么时候不画"的唯一出处）
//
// `DesktopUpdateIndicator.tsx:70`：
//
//	if (!wide || hidden
//	    || (!failed && (state === undefined
//	                    || (state.phase === 'idle' && state.configurationError === undefined))))
//	  return null
//
// 四条判据本仓逐条对应：
//
//	!wide                     本仓只有宽态（没有侧栏收起态），恒为 false
//	hidden                    `connectionIndicator !== undefined && phase !== 'installing'`
//	                          （`SettingsRoot.tsx:243-244`）
//	failed                    壳报"桥存在但调用失败" → 要画「重试更新」
//	state undefined / idle    `Snapshot.DesktopUpdate == nil`，或
//	                          `Phase == idle` 且 `ConfigurationError == ""`
//
// 注意最后一条里的 `&& configurationError === undefined`：**配置坏了的 idle 是要画的**
// （label 固定「配置不可用」）。这是官方原文里的一个 `&&`，漏了它就会在
// "渠道配置读不出来"时整枚消失——那正是用户最需要看见提示的时候。
//
// `hidden` 的映射口径：官方 `connectionIndicator` 只在 connecting / disconnected /
// recovered 三种态下非 undefined（其余 undefined）。本仓 `Snapshot.Link` 与之对应：
//
//	LinkConnecting / LinkDown        → 连接胶囊在画 → 更新指示让位（hidden）
//	LinkConnected                    → 官方此时 connectionIndicator 已消失（2s 恢复窗
//	                                   走完），更新指示可以画
//	LinkUnknown                      → 官方的"还不知道"在 React 里也是 undefined 一侧，
//	                                   本仓同样不让位
//
// 注意本仓那条**既有显式差异**：connected 常驻显示连接胶囊（官方 2s 后隐藏）。
// 若照官方字面用"连接胶囊可见"当 hidden 判据，那这枚更新指示就**永远**不出现了；
// 所以这里按官方 `connectionIndicator` 的**语义取值**（只有三种异常/恢复态）判，
// 不按"胶囊这一帧画没画"判。
//
// # 文案（官方 `locales.ts` 的 zh 段，逐字；`updateCopy()` 的分支逐条复刻）
//
// 可见 label 与 tooltip detail 是**两套**，官方 `updateCopy()`
// （`DesktopUpdateIndicator.tsx:14-55`）就是那个分派器：
//
//	configurationError 非空   label = desktop.channel.unavailable（配置不可用）
//	                          detail = desktop.update.configuration.<read|format|channel|cleanup>
//	checking/verifying/installing   detail = version ?? label
//	error                     detail = desktop.update.<failure 分类文案>（failure 空则按 install）
//	downloading 且带 version   detail = desktop.update.downloadDetail
//	其余（available/ready/idle）detail = desktop.update.versionDetail（"{label}：{version}"），
//	                          没有 version 时 detail = label
//
// 而 `failed === true` 时**整套 copy 被绕过**：label 与 detail 都是 `desktop.update.retry`
// （「重试更新」）。
//
// # 几何（`DesktopUpdateIndicator.module.css:1-34`，逐条，全部登记在几何表里）
//
//	.indicator{display:inline-grid;grid-template-columns:14px max-content;align-items:center;
//	           column-gap:4px;flex:none;height:28px;padding:0 8px;box-sizing:border-box;
//	           border:1px solid color-mix(in srgb, brand 20%, transparent);
//	           border-radius:var(--dsw-radius-sm);
//	           color: brand; background: color-mix(in srgb, brand 8%, transparent);
//	           font: 12px/18px 500; text-align:left; white-space:nowrap; cursor:pointer}
//	.indicator[aria-disabled='true']{cursor:default}
//	.icon{display:grid;place-items:center;width:14px;height:14px}
//	.errorDot{background:currentColor;width:6px;height:6px;border-radius:50%}
//	.spinner{animation:spin 1s linear infinite}
//	@keyframes spin{to{transform:rotate(360deg)}}
//	@media (prefers-reduced-motion:reduce){ .spinner{animation:none} }
//
// 图标（`DesktopUpdateIndicator.tsx:81-83`）三选一：
//
//	error ? <span class=errorDot/>                       ← 错因点（不是叉、不是感叹号）
//	      : busy ? <IconLoadingOutlineRegular class=spinner size=14/>
//	             : <IconDownloadOutlineRegular size=14/>
//
// # 点击（官方 `DesktopUpdateIndicator.tsx:75-79`）
//
//	busy     = opening || (configurationError === undefined && BUSY_PHASES.has(phase))
//	disabled = opening || (busy && phase !== 'downloading')
//	<button aria-label={label} aria-disabled={disabled}
//	        onClick={() => { if (!disabled) onOpen() }}>
//
// 本仓照抄这条禁用判据：busy 期间不可点（`downloading` 除外，本仓宿主不产它）。
// 命中区只在壳真的接了 `DesktopUpdateOpen` 时才挂——壳没接时**画同一枚像素但不给
// 命中区**（画一个点了没反应的东西比不画更坏，这是本仓胶囊那条既有判据）。
//
// # 与官方的两处显式差异（都登记在报告里，不假装对齐）
//
//  1. **tooltip 没有 side**：官方 `<Tooltip side="top">`；viva 的 `ui.Tooltip`
//     由运行时决定位置，插件侧没有 side 入参。detail 内容本身是逐字的。
//  2. **`downloading` 的 `.progress{min-inline-size:calc(3ch + 1em)}` 没生效**：
//     宿主不产 `downloading`（没有下载器），这条分支在本仓不可达；3ch 依赖字体
//     度量，viva 侧没有对应的已登记节点。文案分支本身照官方实现了。
package dssettings

import (
	"math"
	"strconv"
	"time"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// desktopUpdateOrder 是 `sidebar.settings` 行内第 3 格的位置：
// 设置触发器 55 → 账号启动器 60 → 连接指示 61 → **更新指示 62**（官方行内顺序）。
const desktopUpdateOrder = 62

// 官方 zh 文案（`ui-settings-general/src/client/locales.ts` 的 zh 段，逐字）。
const (
	// zhUpdateChecking 是 `desktop.update.checking`。
	zhUpdateChecking = "正在检查更新…"
	// zhUpdateAvailable 是 `desktop.update.available`。
	//
	// 只有两个字，所以**版本号不在可见文字里**——官方的版本号在
	// `desktop.update.versionDetail`（"{label}：{version}"）里，那是 tooltip。
	zhUpdateAvailable = "新版本"
	// zhUpdateVerifying 是 `desktop.update.verifying`。
	zhUpdateVerifying = "正在校验更新文件…"
	// zhUpdateInstalling 是 `desktop.update.installing`。
	zhUpdateInstalling = "正在准备重启…"
	// zhUpdateReady 是 `desktop.update.ready`。
	zhUpdateReady = "安装并重启"
	// zhUpdateRetry 是 `desktop.update.retry`（也是 `failed` 时唯一的 label+detail）。
	zhUpdateRetry = "重试更新"
	// zhChannelUnavailable 是 `desktop.channel.unavailable`：
	// `configurationError` 非空时盖过一切的 label。
	zhChannelUnavailable = "配置不可用"
)

// zhUpdateConfiguration 是 `desktop.update.configuration.<code>` 四句（逐字）。
//
// 官方取值就是壳渠道的四个故障码（`DesktopChannelState['error']`）。
// 未知码返回空串：官方那张表查不到键时拿到 `undefined`，**不**回退到某一句
// ——回退会让我们编出一句官方没说的话。
func zhUpdateConfiguration(code string) string {
	switch code {
	case "read":
		return "更新已暂停：无法读取配置文件"
	case "format":
		return "更新已暂停：配置文件格式有误"
	case "channel":
		return "更新已暂停：channel 只能是 nightly 或 stable"
	case "cleanup":
		return "更新已暂停：无法清理旧更新，请重启应用"
	default:
		return ""
	}
}

// zhUpdateFailure 把官方 `DesktopUpdateFailureKind` 翻成 zh 文案（逐字，locales.ts:36-48）。
//
// 14 个取值**全都在册**（官方 union 就是 14 个），这样"官方哪天加一档我们没跟"
// 会表现为一条测试红，而不是悄悄少一句文案。
//
// `install` 是 `kind == ""` 的兜底——对应官方 `failures[state.failure ?? 'install']`
// 里的 `?? 'install'`。**只有"没给 failure"才走兜底**；给了但不在词表里时返回空串，
// 与官方 `undefined` 同形（那时官方 tooltip 什么都不显示）。
func zhUpdateFailure(kind pluginapi.DesktopUpdateFailureKind) string {
	switch kind {
	case pluginapi.DesktopUpdateFailureConfiguration:
		return "未能应用更新配置。请点击桌面更新入口查看原因。"
	case pluginapi.DesktopUpdateFailureCheck:
		return "检查更新失败，请稍后重试。"
	case pluginapi.DesktopUpdateFailureCheckNetwork:
		return "检查更新失败，请检查网络连接后重试。"
	case pluginapi.DesktopUpdateFailureDownload:
		return "下载更新失败，请重试。"
	case pluginapi.DesktopUpdateFailureDownloadSpace:
		return "磁盘空间不足，无法下载更新。请释放空间后重试。"
	case pluginapi.DesktopUpdateFailureDownloadPermission:
		return "无法写入更新文件，请检查目录权限后重试。"
	case pluginapi.DesktopUpdateFailureDownloadBusy:
		return "更新文件正被占用，请关闭占用它的程序后重试。"
	case pluginapi.DesktopUpdateFailureDownloadNet:
		return "下载更新失败，请检查网络连接后重试。"
	case pluginapi.DesktopUpdateFailureInstall:
		return "安装更新失败，请稍后重试。"
	case pluginapi.DesktopUpdateFailureInstallNetwork:
		return "安装更新失败，请检查网络连接后重试。"
	case pluginapi.DesktopUpdateFailureStopFailed:
		return "未能安全停止任务，更新尚未安装，请稍后重试。"
	case pluginapi.DesktopUpdateFailureTasksChanged:
		return "有新任务开始运行，请重新确认是否停止任务并更新。"
	case pluginapi.DesktopUpdateFailureTasksUnavail:
		return "无法确认任务状态，请在工作区就绪后重试更新。"
	case pluginapi.DesktopUpdateFailureFullscreen:
		return "无法退出全屏，请退出全屏后重试更新。"
	case "":
		return "安装更新失败，请稍后重试。" // 官方 `?? 'install'`
	default:
		return "" // 官方表里没有这个键 -> undefined
	}
}

// updateCopyView 是官方 `updateCopy()` 的返回值（label 与 detail 一对）。
type updateCopyView struct {
	// Label 是可见文字（官方 `<span>{label}</span>`）。
	Label string
	// Detail 是 tooltip 文字（官方 `<Tooltip label={copy.detail}>`）。
	Detail string
}

// updatePillLabel 复刻官方 `updateCopy()` 的 label 分支（`DesktopUpdateIndicator.tsx:16-26`）。
//
// 官方那张 `labels` 表逐条在这里；`idle` 的表项是空串（官方在 idle 时整枚不画，
// 所以这个值实际不可达，但表里有它，照抄）。
func updatePillLabel(state *pluginapi.DesktopUpdatePresentation) string {
	if state == nil {
		return zhUpdateRetry
	}
	switch state.Phase {
	case pluginapi.DesktopUpdateChecking:
		return zhUpdateChecking
	case pluginapi.DesktopUpdateAvailable:
		return zhUpdateAvailable
	case pluginapi.DesktopUpdateDownloading:
		return strconv.Itoa(clampPercent(state.Percent)) + "%"
	case pluginapi.DesktopUpdateVerifying:
		return zhUpdateVerifying
	case pluginapi.DesktopUpdateInstalling:
		return zhUpdateInstalling
	case pluginapi.DesktopUpdateReady:
		return zhUpdateReady
	case pluginapi.DesktopUpdateError:
		return zhUpdateRetry
	default:
		return ""
	}
}

// updateCopy 复刻官方 `updateCopy()`（`DesktopUpdateIndicator.tsx:14-55`）整条分派。
//
// 顺序**不许调**：`configurationError` 排在最前（官方 :15 的第一次 return），
// 它一旦非空，phase / version / percent / failure 全部被忽略。
// 把 `error` 判在它前面，配置坏掉时会显示成"检查更新失败，请重试"——那是另一件事。
func updateCopy(state *pluginapi.DesktopUpdatePresentation) updateCopyView {
	if state == nil {
		return updateCopyView{Label: zhUpdateRetry, Detail: zhUpdateRetry}
	}
	if state.ConfigurationError != "" {
		return updateCopyView{
			Label:  zhChannelUnavailable,
			Detail: zhUpdateConfiguration(state.ConfigurationError),
		}
	}
	label := updatePillLabel(state)
	switch state.Phase {
	case pluginapi.DesktopUpdateChecking, pluginapi.DesktopUpdateVerifying,
		pluginapi.DesktopUpdateInstalling:
		// 官方 :27-29：`detail = state.version ?? label`。
		if state.Version == "" {
			return updateCopyView{Label: label, Detail: label}
		}
		return updateCopyView{Label: label, Detail: state.Version}
	case pluginapi.DesktopUpdateError:
		return updateCopyView{Label: label, Detail: zhUpdateFailure(state.Failure)}
	case pluginapi.DesktopUpdateDownloading:
		if state.Version != "" {
			// 官方 :49-51：`desktop.update.downloadDetail` =
			// "正在下载更新：{percent}%\n目标版本：{version}"。
			return updateCopyView{Label: label, Detail: "正在下载更新：" +
				strconv.Itoa(clampPercent(state.Percent)) + "%\n目标版本：" + state.Version}
		}
	}
	// 官方 :52-54：`detail = version === undefined ? label : versionDetail`。
	if state.Version == "" {
		return updateCopyView{Label: label, Detail: label}
	}
	// 官方 `desktop.update.versionDetail` = "{label}：{version}"（**全角冒号**）。
	return updateCopyView{Label: label, Detail: label + "：" + state.Version}
}

// clampPercent 把进度夹到官方的 0..100（官方主进程侧 `Math.floor(percent)`，
// 越界值不该被画成 "137%" 这种不存在的进度）。
func clampPercent(percent int) int {
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

// desktopUpdateFailed 报告官方 `view.failed`（桥存在但调用失败）。
func desktopUpdateFailed(d pluginapi.Snapshot) bool {
	return d.DesktopUpdate != nil && d.DesktopUpdate.Failed
}

// desktopUpdateVisible 是官方那四条判据的实现（`DesktopUpdateIndicator.tsx:70`）。
func desktopUpdateVisible(d pluginapi.Snapshot) bool {
	failed := desktopUpdateFailed(d)
	state := d.DesktopUpdate
	// `!failed && (state === undefined || (phase === 'idle' && configurationError === undefined))`
	if !failed {
		if state == nil {
			return false
		}
		if state.Phase == pluginapi.DesktopUpdateIdle || state.Phase == "" {
			if state.ConfigurationError == "" {
				return false
			}
		}
	}
	// hidden = 连接胶囊在画（官方：connectionIndicator !== undefined）且不是 installing
	// （安装阶段更新指示必须能看见进度）。本仓 Link 的三态映射见文件头。
	if d.Link == pluginapi.LinkConnecting || d.Link == pluginapi.LinkDown {
		if state == nil || state.Phase != pluginapi.DesktopUpdateInstalling {
			return false
		}
	}
	return true
}

// desktopUpdateBusy 是官方 `BUSY_PHASES`（`DesktopUpdateIndicator.tsx:10-12`）+ opening。
//
// 注意官方那两句里 `configurationError === undefined` 是**并列条件**：
// 配置坏了的时候指示器是能点的（点它去看配置错在哪），不该被 busy 吞掉。
func desktopUpdateBusy(state *pluginapi.DesktopUpdatePresentation) bool {
	if state == nil {
		return false
	}
	if state.Opening {
		return true
	}
	if state.ConfigurationError != "" {
		return false
	}
	return state.Busy()
}

// desktopUpdateDisabled 是官方 `disabled`（`DesktopUpdateIndicator.tsx:75-76`）：
//
//	opening || (busy && phase !== 'downloading')
func desktopUpdateDisabled(state *pluginapi.DesktopUpdatePresentation) bool {
	if state == nil {
		return false
	}
	if state.Opening {
		return true
	}
	if !desktopUpdateBusy(state) {
		return false
	}
	// 下载中**不禁用**（官方要点开面板看进度）。本仓宿主不产 downloading，
	// 这条分支照官方留着，不因为"到不了"就删掉判据。
	return state.Phase != pluginapi.DesktopUpdateDownloading
}

// drawDesktopUpdateIndicator 画官方 `DesktopUpdateIndicator`（28 高、brand 描边/底、
// 12/18 500、14px glyph + gap4）。
func (p *Plugin) drawDesktopUpdateIndicator(f ui.C, env *pluginapi.Env) {
	if !desktopUpdateVisible(env.Data) {
		return
	}
	t := p.theme
	if env.Theme != nil {
		t = env.Theme
	}
	state := env.Data.DesktopUpdate
	failed := desktopUpdateFailed(env.Data)

	// 官方 :71-73：failed 时整条 copy 被换掉（label 与 detail 都是「重试更新」）。
	copy := updateCopy(state)
	if failed {
		copy = updateCopyView{Label: zhUpdateRetry, Detail: zhUpdateRetry}
	}
	// 官方 :74 的 `error` 标志：决定画错因点还是下载图标，以及文字要不要用 .progress。
	errorState := failed || (state != nil &&
		(state.Phase == pluginapi.DesktopUpdateError || state.ConfigurationError != ""))
	busy := desktopUpdateBusy(state)
	disabled := desktopUpdateDisabled(state)

	brand := t.BrandPrimary
	pill := ui.Row(f).Height(UpdateIndicatorHeight).Radius(t.Radii.SM).
		Background(withAlphaTint(brand, UpdateIndicatorBgTint)).
		Border(UpdateIndicatorBorder, withAlphaTint(brand, UpdateIndicatorBorderTint)).
		Items(ui.Center).Gap(0)

	// 命中区只在壳真的接了 open 时才挂；壳没接时像素照画、不画假按钮。
	//
	// 官方 `.indicator{cursor:pointer}` / `[aria-disabled='true']{cursor:default}`
	// 由 HitOptions.Cursor 表达；`aria-disabled` 由 HitOptions.Disabled 表达
	// （置位后 viva 不再派发点击，与官方 `onClick` 里的 `if (!disabled)` 等效）。
	if env.Act.DesktopUpdateOpen != nil {
		cursor := gfx.PointerCursorHand
		if disabled {
			cursor = gfx.PointerCursorDefault
		}
		pill = pill.HitWith(env.Tree.ID(env.Feat, "settings-desktop-update"), copy.Label,
			gfx.HitOptions{Role: gfx.RoleButton, Disabled: disabled, Cursor: cursor})
		if copy.Detail != "" {
			// 官方 `<Tooltip label={failed ? label : copy.detail} side="top">`。
			pill = pill.Tooltip(copy.Detail)
		}
	}

	pill.Children(func() {
		// `padding:0 8px` 用定宽占位盒（与 capsule 同一理由：避免 pref 往返截字）。
		ui.Row(f).Width(UpdateIndicatorPadX).Height(UpdateIndicatorHeight)
		p.drawUpdateGlyph(f, env, state, brand, errorState, busy)
		ui.Row(f).Width(UpdateIndicatorGap).Height(UpdateIndicatorHeight)
		ui.Text(f, copy.Label).FontSize(UpdateIndicatorFontSize).LineHeight(UpdateIndicatorLineHeight).
			Weight(gfx.WeightMedium).Color(brand).Ellipsis()
		ui.Row(f).Width(UpdateIndicatorPadX).Height(UpdateIndicatorHeight)
	})

	// 官方 busy 分支的 spinner 是 1s 线性旋转：自己排下一帧（本仓没有渲染循环）。
	if busy && !reducedMotion(env) {
		p.scheduleUpdateFrame(env.Act.Invalidate)
	}
	// 官方 `onClick={() => { if (!disabled) onOpen() }}`。
	if !disabled && env.Act.DesktopUpdateOpen != nil && pill.Clicked() {
		env.Act.DesktopUpdateOpen()
	}
}

// drawUpdateGlyph 画官方那三枚 14px glyph（error = 错因点；busy = loading 旋转 1s；
// 否则 download）。颜色随 `.indicator{color:brand}`。
func (p *Plugin) drawUpdateGlyph(f ui.C, env *pluginapi.Env, state *pluginapi.DesktopUpdatePresentation,
	tint gfx.Color, errorState, busy bool) {
	// 官方 :81-83 的分支顺序就是 error → busy → download，**不许调**：
	// 一个 error 相同时 busy 为假，但配置错 + busy 是能同时为真的
	// （official :15 的 configurationError 优先），那时画的仍是错因点。
	if errorState {
		// `.errorDot{background:currentColor;width:6px;height:6px;border-radius:50%}`，
		// 长在 14×14 的 `.icon`（grid + place-items:center）里，所以是居中的 6px 圆。
		ui.Row(f).Size(UpdateIndicatorIconSize, UpdateIndicatorIconSize).
			Items(ui.Center).Justify(ui.Center).Children(func() {
			ui.Row(f).Size(UpdateErrorDotSize, UpdateErrorDotSize).
				Radius(UpdateErrorDotSize / 2).Background(tint)
		})
		return
	}
	if busy {
		rotate := float32(0)
		if !reducedMotion(env) {
			// 官方 `.spinner{animation:spin 1s linear infinite}`。
			rotate = updateSpinRadians(time.Now())
		}
		ui.Icon(f, loadingOutlineIcon()).Size(UpdateIndicatorIconSize, UpdateIndicatorIconSize).
			Color(tint).IconTransform(gfx.IconTransform{Rotate: rotate})
		return
	}
	ui.Icon(f, downloadOutlineIcon()).Size(UpdateIndicatorIconSize, UpdateIndicatorIconSize).Color(tint)
}

// updateSpinRadians 是官方 `@keyframes spin{to{transform:rotate(360deg)}}` + 1s 周期。
func updateSpinRadians(now time.Time) float32 {
	periodNanos := int64(UpdateIndicatorSpinPeriodMS) * int64(time.Millisecond)
	nanos := now.UnixNano() % periodNanos
	if nanos < 0 {
		nanos += periodNanos
	}
	return 2 * math.Pi * float32(nanos) / float32(periodNanos)
}

// reducedMotion 取 viva canvas 的减动效开关（桌面壳按操作系统设置注入）。
func reducedMotion(env *pluginapi.Env) bool {
	return env != nil && env.Canvas != nil && env.Canvas.ReduceMotion()
}

// scheduleUpdateFrame 在 busy spinner 需要下一帧时申请重绘（幂等，只排一个）。
//
// 与 dsstatusbar 的同名逻辑同构：计时器在另一条 goroutine 上跑，函数只捕获不可变值。
func (p *Plugin) scheduleUpdateFrame(invalidate func()) {
	if invalidate == nil {
		return
	}
	const delay = 33 * time.Millisecond
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.updateFrameTimer != nil {
		return
	}
	p.updateFrameTimer = time.AfterFunc(delay, func() {
		p.mu.Lock()
		p.updateFrameTimer = nil
		p.mu.Unlock()
		invalidate()
	})
}

// withAlphaTint 是官方 `color-mix(in srgb, C t, transparent)` 的等价写法。
func withAlphaTint(c gfx.Color, tint float32) gfx.Color {
	c.Alpha *= tint
	return c
}

// loadingOutlineIcon 是官方 sprite `loading-outline`（busy 态，14px 旋转）。
//
// 官方原文（dist 偏移 693206，viewBox `0 0 16 16`、`stroke-width=1`）：
//
//	<path d="M12.596 12.596C11.687 13.5049 10.5288 14.1239 9.26798 14.3747C8.00716
//	         14.6255 6.70028 14.4968 5.51261 14.0048C4.32494 13.5129 3.30981 12.6798
//	         2.59557 11.611C1.88134 10.5421 1.50008 9.2855 1.5 7.99998C1.50008 6.71446
//	         1.88134 5.45783 2.59557 4.38898C3.30981 3.32013 4.32494 2.48707 5.51261
//	         1.99513C6.70028 1.50319 8.00716 1.37447 9.26798 1.62524C10.5288 1.87602
//	         11.687 2.49502 12.596 3.40398" stroke="currentColor"/>
func loadingOutlineIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(12.596, 12.596),
			gfx.CubicTo(11.687, 13.5049, 10.5288, 14.1239, 9.26798, 14.3747),
			gfx.CubicTo(8.00716, 14.6255, 6.70028, 14.4968, 5.51261, 14.0048),
			gfx.CubicTo(4.32494, 13.5129, 3.30981, 12.6798, 2.59557, 11.611),
			gfx.CubicTo(1.88134, 10.5421, 1.50008, 9.2855, 1.5, 7.99998),
			gfx.CubicTo(1.50008, 6.71446, 1.88134, 5.45783, 2.59557, 4.38898),
			gfx.CubicTo(3.30981, 3.32013, 4.32494, 2.48707, 5.51261, 1.99513),
			gfx.CubicTo(6.70028, 1.50319, 8.00716, 1.37447, 9.26798, 1.62524),
			gfx.CubicTo(10.5288, 1.87602, 11.687, 2.49502, 12.596, 3.40398),
		},
	}
}

// downloadOutlineIcon 是官方 sprite `download-outline`（非 busy 态，14px）。
//
// 官方原文（dist 偏移 693771，viewBox `0 0 16 16`、`stroke-width=1`，三条 path）：
//
//	<path d="M8 1.95317V10.0469" stroke="currentColor"/>
//	<path d="M4.25 6.29688L8 10.0469L11.75 6.29688" stroke="currentColor"/>
//	<path d="M1.5 10.0469V13.158C1.5 13.3937 1.60536 13.6198 1.79289 13.7865C1.98043
//	         13.9532 2.23478 14.0469 2.5 14.0469H13.5C13.7652 14.0469 14.0196 13.9532
//	         14.2071 13.7865C14.3946 13.6198 14.5 13.3937 14.5 13.158V10.0469"
//	     stroke="currentColor"/>
func downloadOutlineIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(8, 1.95317),
			gfx.LineTo(8, 10.0469),
			gfx.MoveTo(4.25, 6.29688),
			gfx.LineTo(8, 10.0469),
			gfx.LineTo(11.75, 6.29688),
			gfx.MoveTo(1.5, 10.0469),
			gfx.LineTo(1.5, 13.158),
			gfx.CubicTo(1.5, 13.3937, 1.60536, 13.6198, 1.79289, 13.7865),
			gfx.CubicTo(1.98043, 13.9532, 2.23478, 14.0469, 2.5, 14.0469),
			gfx.LineTo(13.5, 14.0469),
			gfx.CubicTo(13.7652, 14.0469, 14.0196, 13.9532, 14.2071, 13.7865),
			gfx.CubicTo(14.3946, 13.6198, 14.5, 13.3937, 14.5, 13.158),
			gfx.LineTo(14.5, 10.0469),
		},
	}
}
