// desktop_update.go —— 官方 `DesktopUpdateIndicator`（`SettingsRoot.triggerRow` 的
// **第三个子项**）要画的那一帧事实。
//
// # 官方出处（逐字）
//
// `ui-settings-general/src/client/DesktopUpdateIndicator.tsx`：
//
//	export function DesktopUpdateIndicator({ wide, hidden, t, view, onOpen }) {
//	  const { presentation: state, failed, opening } = view
//	  if (!wide || hidden || (!failed && (state === undefined || state.phase === 'idle'))) return null
//	  …
//	}
//
// 也就是：**没有呈现（presentation）为 undefined 或 phase==='idle' 时整枚不画**。
// 这条判据正是本仓"无真值不画"的同一句，所以产品没有桌面更新通道时它天然不出现——
// 不画是官方的形状，不是缺实现。
//
// `SettingsRoot.tsx:243-244` 的可见性输入：
//
//	<DesktopUpdateIndicator
//	  wide={wide}
//	  hidden={connectionIndicator !== undefined && desktopUpdate.presentation?.phase !== 'installing'}
//	  … />
//
// 阶段取值（`ui-settings-general/src/types.ts` 的 `DesktopUpdatePresentation['phase']`，
// 与 `DesktopUpdateIndicator.tsx` 的 `labels` 表逐条对应）：
//
//	idle / checking / available / downloading / verifying / installing / ready / error
//
// 文案（`ui-settings-general/src/client/locales.ts` 的 zh 段，逐字）：
//
//	desktop.update.checking         正在检查更新…
//	desktop.update.available        新版本
//	desktop.update.progress         {percent}%
//	desktop.update.verifying        正在校验更新文件…
//	desktop.update.installing       正在准备重启…
//	desktop.update.ready            安装并重启
//	desktop.update.retry            重试更新
//	desktop.update.versionDetail    {label}：{version}
//	desktop.update.downloadDetail   正在下载更新：{percent}%\n目标版本：{version}
//
// # 为什么类型放在 pluginapi 而不是某个插件里
//
// 它和 `LinkState` 同一层：是**壳告诉界面的事实**（官方那条 `useDesktopUpdate`
// 注入 face 的等价物）。后端的 updater 一旦存在，只有壳知道，插件不该自己猜。
package pluginapi

// DesktopUpdatePhase 是官方 `DesktopUpdatePresentation['phase']`（逐值转写）。
type DesktopUpdatePhase string

const (
	// DesktopUpdateIdle 是"什么都没有"——官方在此整枚不渲染。
	DesktopUpdateIdle DesktopUpdatePhase = "idle"
	// DesktopUpdateChecking 对应 `desktop.update.checking`（busy）。
	DesktopUpdateChecking DesktopUpdatePhase = "checking"
	// DesktopUpdateAvailable 对应 `desktop.update.available`（非 busy，可点开）。
	DesktopUpdateAvailable DesktopUpdatePhase = "available"
	// DesktopUpdateDownloading 对应 `desktop.update.progress`（busy）。
	DesktopUpdateDownloading DesktopUpdatePhase = "downloading"
	// DesktopUpdateVerifying 对应 `desktop.update.verifying`（busy）。
	DesktopUpdateVerifying DesktopUpdatePhase = "verifying"
	// DesktopUpdateInstalling 对应 `desktop.update.installing`（busy）。
	DesktopUpdateInstalling DesktopUpdatePhase = "installing"
	// DesktopUpdateReady 对应 `desktop.update.ready`（可点开=安装并重启）。
	DesktopUpdateReady DesktopUpdatePhase = "ready"
	// DesktopUpdateError 对应 `desktop.update.retry`（错因见 Failure）。
	DesktopUpdateError DesktopUpdatePhase = "error"
)

// DesktopUpdateFailureKind 是官方 `DesktopUpdateFailureKind`（逐值转写）。
type DesktopUpdateFailureKind string

const (
	// DesktopUpdateFailureConfiguration 是壳级配置坏掉（官方 `'configuration'`）。
	DesktopUpdateFailureConfiguration DesktopUpdateFailureKind = "configuration"
	DesktopUpdateFailureCheck         DesktopUpdateFailureKind = "check"
	DesktopUpdateFailureCheckNetwork  DesktopUpdateFailureKind = "check-network"
	DesktopUpdateFailureDownload      DesktopUpdateFailureKind = "download"
	// DesktopUpdateFailureDownloadSpace 是磁盘满（官方 `'download-space'`）。
	DesktopUpdateFailureDownloadSpace DesktopUpdateFailureKind = "download-space"
	// DesktopUpdateFailureDownloadPermission 是写不进更新目录（官方 `'download-permission'`）。
	DesktopUpdateFailureDownloadPermission DesktopUpdateFailureKind = "download-permission"
	// DesktopUpdateFailureDownloadBusy 是更新文件被占用（官方 `'download-busy'`）。
	DesktopUpdateFailureDownloadBusy   DesktopUpdateFailureKind = "download-busy"
	DesktopUpdateFailureDownloadNet    DesktopUpdateFailureKind = "download-network"
	DesktopUpdateFailureInstall        DesktopUpdateFailureKind = "install"
	DesktopUpdateFailureInstallNetwork DesktopUpdateFailureKind = "install-network"
	DesktopUpdateFailureStopFailed     DesktopUpdateFailureKind = "stop-failed"
	DesktopUpdateFailureTasksChanged   DesktopUpdateFailureKind = "tasks-changed"
	DesktopUpdateFailureTasksUnavail   DesktopUpdateFailureKind = "tasks-unavailable"
	DesktopUpdateFailureFullscreen     DesktopUpdateFailureKind = "fullscreen-timeout"
)

// DesktopUpdatePresentation 是官方 `DesktopUpdatePresentation`（壳侧事实的投影）。
//
// 零值 Phase 视为 `idle`（官方 `state === undefined || state.phase === 'idle'` 同一句）。
type DesktopUpdatePresentation struct {
	// Phase 是当前阶段（官方 `state.phase`）。
	Phase DesktopUpdatePhase
	// Version 是目标版本号（官方 `state.version`；没有就空串）。
	Version string
	// Percent 是下载进度（官方 `state.percent`；0..100，仅下载阶段有意义）。
	Percent int
	// Failure 是错误阶段的具体错因（官方 `state.failure`）。
	Failure DesktopUpdateFailureKind
	// ConfigurationError 是壳渠道的故障码（官方 `state.configurationError`：
	// read / format / channel / cleanup）。
	//
	// 官方渲染层**先判它**（`DesktopUpdateIndicator.tsx:15`）：一旦非空，
	// phase / version / percent / failure 全部被忽略，label 固定成
	// `desktop.channel.unavailable`（zh「配置不可用」）。所以它不是"另一个错因"，
	// 而是盖过其余一切的那一条。
	ConfigurationError string
	// Failed 是官方 `view.failed`：桥接/更新器本身失败，界面转成「重试更新」。
	Failed bool
	// Opening 是官方 `view.opening`：用户点了更新入口、更新面板正在打开（busy）。
	Opening bool
}

// Idle 报告这枚呈现是否等于官方的"什么都不画"。
func (p *DesktopUpdatePresentation) Idle() bool {
	return p == nil || p.Phase == "" || p.Phase == DesktopUpdateIdle
}

// Busy 报告官方 `BUSY_PHASES`（checking/downloading/verifying/installing）+ opening：
// 官方在 busy 时把按钮设成 `aria-disabled` 并画 spinner，点它不打开面板。
func (p *DesktopUpdatePresentation) Busy() bool {
	if p == nil {
		return false
	}
	if p.Opening {
		return true
	}
	switch p.Phase {
	case DesktopUpdateChecking, DesktopUpdateDownloading, DesktopUpdateVerifying, DesktopUpdateInstalling:
		return true
	default:
		return false
	}
}
