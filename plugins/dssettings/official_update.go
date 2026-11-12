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
// 也就是行里的第 **3** 个子项（本仓在 HH 单之前只画了 1、2 两格）。它**不是
// slot**：官方把它硬写在 SettingsRoot 里，所以本插件（设置面 owner）画它，
// 与设置触发器同一个座位（`sidebar.settings`），Order 排在连接指示（61）之后。
//
// # 可见性（官方逐条，这是"什么时候不画"的唯一出处）
//
// `DesktopUpdateIndicator.tsx:66`：
//
//	if (!wide || hidden || (!failed && (state === undefined || state.phase === 'idle')))
//	  return null
//
// 三条判据本仓逐条对应：
//
//	!wide                    本仓只有宽态（没有侧栏收起态），恒为 false
//	hidden                   `connectionIndicator !== undefined && phase !== 'installing'`
//	                         （`SettingsRoot.tsx:243-244`）
//	state undefined / idle   `Snapshot.DesktopUpdate == nil` 或 `Phase == idle`
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
// # 几何（`DesktopUpdateIndicator.module.css:1-21`，逐条）
//
//	.indicator{display:inline-grid;grid-template-columns:14px max-content;align-items:center;
//	           column-gap:4px;flex:none;height:28px;padding:0 8px;box-sizing:border-box;
//	           border:1px solid color-mix(in srgb, brand 20%, transparent);
//	           border-radius:var(--dsw-radius-sm);
//	           color: brand; background: color-mix(in srgb, brand 8%, transparent);
//	           font: 12px/18px 500; text-align:left; white-space:nowrap; cursor:pointer}
//	.icon{display:grid;place-items:center;width:14px;height:14px}
//	.spinner{animation:spin 1s linear infinite}   ← busy 态的 loading-outline
//	@keyframes spin{to{transform:rotate(360deg)}}
//	@media (prefers-reduced-motion:reduce){ .spinner{animation:none} }
//
// 图标（`DesktopUpdateIndicator.tsx:76`）：
//
//	busy ? <IconLoadingOutlineRegular className={css.spinner} size={14} />
//	     : <IconDownloadOutlineRegular size={14} />
//
// # 与官方的两处显式差异（都登记在报告里，不假装对齐）
//
//  1. **只读呈现，不画按钮**：官方是 `<button onClick={onOpen}>`（点开更新面板）。
//     本仓没有桌面更新面板、也没有 updater 通道，画一个点了没反应的按钮比不画更坏
//     ——这与本仓胶囊的同一判据一致（没有真 action 就不画按钮）。有真 action 时
//     这里要补命中区与 `aria-disabled`（官方的 busy 分支）。
//  2. **本仓产品常态不出现**：官方宿主自带 Electron 更新器；本仓宿主没有 updater
//     方法，`Snapshot.DesktopUpdate` 恒为 nil ⇒ 按官方 idle 规则整枚不渲染。
//     这不是漏实现，是官方在"没有更新这件事"时的形状；通道缺失登记为差距。
package dssettings

import (
	"math"
	"time"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// desktopUpdateOrder 是 `sidebar.settings` 行内第 3 格的位置：
// 设置触发器 55 → 账号启动器 60 → 连接指示 61 → **更新指示 62**（官方行内顺序）。
const desktopUpdateOrder = 62

// 官方 zh 文案（`ui-settings-general/src/client/locales.ts` 的 zh 段，逐字）。
const (
	zhUpdateChecking   = "正在检查更新…"
	zhUpdateAvailable  = "新版本"
	zhUpdateVerifying  = "正在校验更新文件…"
	zhUpdateInstalling = "正在准备重启…"
	zhUpdateReady      = "安装并重启"
	zhUpdateRetry      = "重试更新"
)

// zhUpdateFailure 把官方 `DesktopUpdateFailureKind` 翻成 zh 文案（逐字，locales.ts:16-25）。
func zhUpdateFailure(kind pluginapi.DesktopUpdateFailureKind) string {
	switch kind {
	case pluginapi.DesktopUpdateFailureCheck:
		return "检查更新失败，请稍后重试。"
	case pluginapi.DesktopUpdateFailureCheckNetwork:
		return "检查更新失败，请检查网络连接后重试。"
	case pluginapi.DesktopUpdateFailureDownload:
		return "下载更新失败，请重试。"
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
	default:
		return "安装更新失败，请稍后重试。"
	}
}

// updatePillLabel 复刻官方 `updateCopy()` 的 label 分支（`DesktopUpdateIndicator.tsx:17-56`）。
//
// 官方那张表里 `checking`/`verifying`/`installing` 的 label 就是上面三句；
// `downloading` 的 label 是进度百分比；`available`/`ready`/`error` 各一句。
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
		return itoaPercent(state.Percent)
	case pluginapi.DesktopUpdateVerifying:
		return zhUpdateVerifying
	case pluginapi.DesktopUpdateInstalling:
		return zhUpdateInstalling
	case pluginapi.DesktopUpdateReady:
		return zhUpdateReady
	case pluginapi.DesktopUpdateError:
		return zhUpdateRetry
	default:
		return zhUpdateRetry
	}
}

// itoaPercent 造官方 `desktop.update.progress` = `{percent}%` 的文本。
func itoaPercent(percent int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	digits := []byte{}
	if percent == 0 {
		digits = append(digits, '0')
	}
	for percent > 0 {
		digits = append([]byte{byte('0' + percent%10)}, digits...)
		percent /= 10
	}
	return string(digits) + "%"
}

// desktopUpdateVisible 是官方那三行判据的实现（`DesktopUpdateIndicator.tsx:66`）。
func desktopUpdateVisible(d pluginapi.Snapshot) bool {
	if d.DesktopUpdate.Idle() && !d.DesktopUpdate.Busy() {
		return false // state undefined / phase idle ⇒ 整枚不画
	}
	// hidden = 连接胶囊在画（官方：connectionIndicator !== undefined）且不是 installing
	// （安装阶段更新指示必须能看见进度）。本仓 Link 的三态映射见文件头。
	if d.Link == pluginapi.LinkConnecting || d.Link == pluginapi.LinkDown {
		if d.DesktopUpdate.Phase != pluginapi.DesktopUpdateInstalling {
			return false
		}
	}
	return true
}

// drawDesktopUpdateIndicator 画官方 `DesktopUpdateIndicator`（28 高、brand 描边/底、
// 12/18 500、14px glyph + gap4）。
func (p *Plugin) drawDesktopUpdateIndicator(f *flex.Frame, env *pluginapi.Env) {
	if !desktopUpdateVisible(env.Data) {
		return
	}
	t := p.theme
	if env.Theme != nil {
		t = env.Theme
	}
	state := env.Data.DesktopUpdate
	label := updatePillLabel(state)
	if state.Failed && state.Phase == pluginapi.DesktopUpdateError {
		// 官方 `detail` = 具体错因（tooltip 内容）；本仓没有 tooltip 层，
		// 错因留在 aria/命中说明里（见报告「未实现」）。
		_ = zhUpdateFailure(state.Failure)
	}
	brand := t.BrandPrimary
	pill := f.Row().Height(UpdateIndicatorHeight).Radius(t.Radii.SM).
		Background(withAlphaTint(brand, UpdateIndicatorBgTint)).
		Border(UpdateIndicatorBorder, withAlphaTint(brand, UpdateIndicatorBorderTint)).
		Items(flex.Center).Gap(0).
		// 官方 `.indicator[aria-disabled=true]{cursor:default}`：busy 时不可点。
		// 本仓只读（无更新面板），所以整格不接命中区。
		Children(func() {
			// `padding:0 8px` 用定宽占位盒（与 capsule 同一理由：避免 pref 往返截字）。
			f.Row().Width(UpdateIndicatorPadX).Height(UpdateIndicatorHeight)
			p.drawUpdateGlyph(f, env, state, brand)
			f.Row().Width(UpdateIndicatorGap).Height(UpdateIndicatorHeight)
			f.Text(label).FontSize(UpdateIndicatorFontSize).LineHeight(UpdateIndicatorLineHeight).
				Weight(ui.WeightMedium).Color(brand).Ellipsis()
			f.Row().Width(UpdateIndicatorPadX).Height(UpdateIndicatorHeight)
		})
	// 官方 busy 分支的 spinner 是 1s 线性旋转：自己排下一帧（本仓没有渲染循环）。
	if state.Busy() && !reducedMotion(env) {
		p.scheduleUpdateFrame(env.Act.Invalidate)
	}
	_ = pill
}

// drawUpdateGlyph 画官方那两枚 14px glyph（busy = loading-outline 旋转 1s，否则
// download-outline）。颜色随 `.indicator{color:brand}`。
func (p *Plugin) drawUpdateGlyph(f *flex.Frame, env *pluginapi.Env, state *pluginapi.DesktopUpdatePresentation, tint ui.Color) {
	if state.Busy() {
		rotate := float32(0)
		if !reducedMotion(env) {
			// 官方 `.spinner{animation:spin 1s linear infinite}`。
			rotate = updateSpinRadians(time.Now())
		}
		f.Icon(loadingOutlineIcon()).Size(UpdateIndicatorIconSize, UpdateIndicatorIconSize).
			Color(tint).IconTransform(ui.IconTransform{Rotate: rotate})
		return
	}
	f.Icon(downloadOutlineIcon()).Size(UpdateIndicatorIconSize, UpdateIndicatorIconSize).Color(tint)
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
func withAlphaTint(c ui.Color, tint float32) ui.Color {
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
func loadingOutlineIcon() ui.Icon {
	return ui.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []ui.PathCommand{
			ui.MoveTo(12.596, 12.596),
			ui.CubicTo(11.687, 13.5049, 10.5288, 14.1239, 9.26798, 14.3747),
			ui.CubicTo(8.00716, 14.6255, 6.70028, 14.4968, 5.51261, 14.0048),
			ui.CubicTo(4.32494, 13.5129, 3.30981, 12.6798, 2.59557, 11.611),
			ui.CubicTo(1.88134, 10.5421, 1.50008, 9.2855, 1.5, 7.99998),
			ui.CubicTo(1.50008, 6.71446, 1.88134, 5.45783, 2.59557, 4.38898),
			ui.CubicTo(3.30981, 3.32013, 4.32494, 2.48707, 5.51261, 1.99513),
			ui.CubicTo(6.70028, 1.50319, 8.00716, 1.37447, 9.26798, 1.62524),
			ui.CubicTo(10.5288, 1.87602, 11.687, 2.49502, 12.596, 3.40398),
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
func downloadOutlineIcon() ui.Icon {
	return ui.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []ui.PathCommand{
			ui.MoveTo(8, 1.95317),
			ui.LineTo(8, 10.0469),
			ui.MoveTo(4.25, 6.29688),
			ui.LineTo(8, 10.0469),
			ui.LineTo(11.75, 6.29688),
			ui.MoveTo(1.5, 10.0469),
			ui.LineTo(1.5, 13.158),
			ui.CubicTo(1.5, 13.3937, 1.60536, 13.6198, 1.79289, 13.7865),
			ui.CubicTo(1.98043, 13.9532, 2.23478, 14.0469, 2.5, 14.0469),
			ui.LineTo(13.5, 14.0469),
			ui.CubicTo(13.7652, 14.0469, 14.0196, 13.9532, 14.2071, 13.7865),
			ui.CubicTo(14.3946, 13.6198, 14.5, 13.3937, 14.5, 13.158),
			ui.LineTo(14.5, 10.0469),
		},
	}
}
