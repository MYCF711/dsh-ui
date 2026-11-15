package dscordis

import (
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// status.go —— 官方 `status.ts`（45 行）的逐行等价物 + 面板状态派生。
//
// 官方两份来源：
//
//	OFF:dsh-client-ui-cordis/src/client/status.ts:17-22   packageOf
//	OFF:dsh-client-ui-cordis/src/client/status.ts:31-45   cordisVisibleStatus
//	OFF:dsh-client-ui-cordis/src/client/CordisPanel.tsx:49-60   selectedPackageIdOf
//	OFF:dsh-client-ui-cordis/src/client/CordisPanel.tsx:62-86   visiblePanelStatus / panelDotState
//
// # 为什么"有 Client 半的包在跑"必须显示 client-pending
//
// `cordisVisibleStatus` 的第三个参数是**本页已加载的 Client 半**（官方
// `DynamicCordisLivePackage[]`：页面里真的把那个 React 组件挂起来了）。
// 本仓没有 JS 组件宿主（HIDE 第 1 条），所以那份集合**恒为空集**——
// 于是"有 Client 半的包跑起来了"永远是 `client-pending`，而不是 `running`。
//
// 这不是显示 bug，是官方语义的直接结果：`running` 的意思是"Host 半与 Client 半
// 都在跑"，本仓只有 Host 半。为了让这件事在代码里**看得见**（而不是靠一个
// `return "client-pending"` 硬编码），这里把那份空集写成 `loadedClientHalves()`
// 并让派生逐行照官方的成员判断走。

// cordisVisibleStatus 是官方 `CordisVisibleStatus`（status.ts:9）的三值。
type cordisVisibleStatus uint8

const (
	// cordisStatusIdle 是 `'idle'`（status.ts:9）——这个 Package 没在跑。
	cordisStatusIdle cordisVisibleStatus = iota
	// cordisStatusClientPending 是 `'client-pending'`（status.ts:9）——
	// Host 半在跑，Client 半还没在本页激活。
	cordisStatusClientPending
	// cordisStatusRunning 是 `'running'`（status.ts:9）——Host 半与 Client 半都在跑。
	cordisStatusRunning
)

// 官方 `CordisRunStatus` 九值里本文件用得到的两个（types.ts:109-118）。
const (
	// cordisRunStatusAwaitingApproval 是 `'awaiting-approval'`。
	cordisRunStatusAwaitingApproval = "awaiting-approval"
	// cordisRunStatusFailed 是 `'failed'`。
	cordisRunStatusFailed = "failed"
)

// loadedClientHalf 是官方 `DynamicCordisLivePackage`（client-runner）在派生里
// 被用到的三个字段（status.ts:40-42 拿它们做成员判断）。
type loadedClientHalf struct {
	// PluginID 是 `live.pluginId`。
	PluginID string
	// PackageID 是 `live.packageId`。
	PackageID string
	// PluginRunID 是 `live.pluginRunId`。
	PluginRunID string
}

// loadedClientHalves 返回"本页已加载的 Client 半"——**本仓恒为空集**。
//
// 空集不是省略，是事实：本仓没有能在页面里加载并运行 `client.js` 的 JS 组件宿主
// （HIDE 第 1、3 条）。返回 nil 而不是一个默认包，是为了让
// `cordisVisibleStatusOf` 的 `loaded.some(...)` 真的去判断"有没有"，
// 而不是把结论写死在调用点。
func loadedClientHalves() []loadedClientHalf { return nil }

// packageOf 是官方 `packageOf`（status.ts:17-22）：在一个 Plugin 行里找一个 Package。
func packageOf(row apiclient.CordisRow, packageID string) (apiclient.CordisPackage, bool) {
	if packageID == "" {
		return apiclient.CordisPackage{}, false
	}
	for _, pkg := range row.Packages {
		if pkg.PackageID == packageID {
			return pkg, true
		}
	}
	return apiclient.CordisPackage{}, false
}

// activePackageOf 是官方 `packageOf(listed, listed.activeRun.packageId)`
// （CordisPanel.tsx:228-230）：正在跑的那个 Package 的元数据。
func activePackageOf(row apiclient.CordisRow) (apiclient.CordisPackage, bool) {
	if row.ActiveRun == nil {
		return apiclient.CordisPackage{}, false
	}
	return packageOf(row, row.ActiveRun.PackageID)
}

// cordisVisibleStatusOf 是官方 `cordisVisibleStatus`（status.ts:31-45）逐行等价物。
func cordisVisibleStatusOf(row apiclient.CordisRow, packageID string, loaded []loadedClientHalf) cordisVisibleStatus {
	run := row.ActiveRun
	// status.ts:36-37：没有 activeRun，或者跑的不是这个 Package → idle。
	if run == nil || run.PackageID != packageID {
		return cordisStatusIdle
	}
	pkg, ok := packageOf(row, packageID)
	// status.ts:38-39：`if (pkg?.hasClientHalf !== true) return 'running'`——
	// 没有 Client 半的包，Host 半起来就是完整运行。
	if !ok || !pkg.HasClientHalf {
		return cordisStatusRunning
	}
	// status.ts:40-44：有 Client 半 → 只有"本页真的加载了它"才算 running，
	// 否则是 client-pending。本仓 loaded 恒空（见 loadedClientHalves），所以这里
	// 恒走 else 分支——但判断本身是真的在判断。
	for _, live := range loaded {
		if live.PluginID == row.PluginID && live.PackageID == packageID && live.PluginRunID == run.PluginRunID {
			return cordisStatusRunning
		}
	}
	return cordisStatusClientPending
}

// selectedPackageIDOf 是官方 `selectedPackageIdOf`（CordisPanel.tsx:49-60）
// 的**没有选择**分支。
//
// 官方首个分支读的是用户在版本选择器里点过的那一项（`selected[pluginId]`）；
// 本仓没有那个选择器——版本选择器的出现条件是该行 `packages.length > 1`
// （CordisPanel.tsx:267），而本宿主恒 1 个 Package（`pluginapi/cordis.go` 的
// Rows 注释），所以那张 `selected` 表**永远为空**，首分支不可达。
// 这里保留官方剩下的兜底顺序：nextPackageId → currentPackageId → 最后一个 Package
// → 在跑的那个 Package。条件照官方写，但不给假状态（差异登记见 plugin.go 第 6 条）。
func selectedPackageIDOf(row apiclient.CordisRow) string {
	if row.NextPackageID != "" {
		return row.NextPackageID
	}
	if row.CurrentPackageID != "" {
		return row.CurrentPackageID
	}
	if n := len(row.Packages); n > 0 {
		return row.Packages[n-1].PackageID
	}
	if row.ActiveRun != nil {
		return row.ActiveRun.PackageID
	}
	return ""
}

// panelStatus 是官方 `PanelStatus = CordisVisibleStatus | 'awaiting-approval' | 'failed'`
// （CordisPanel.tsx:27）。
type panelStatus uint8

const (
	// panelStatusIdle 对应 `'idle'`。
	panelStatusIdle panelStatus = iota
	// panelStatusAwaitingApproval 对应 `'awaiting-approval'`（本宿主不出现，见 locale.go）。
	panelStatusAwaitingApproval
	// panelStatusClientPending 对应 `'client-pending'`。
	panelStatusClientPending
	// panelStatusRunning 对应 `'running'`。
	panelStatusRunning
	// panelStatusFailed 对应 `'failed'`。
	panelStatusFailed
)

// visiblePanelStatus 是官方 `visiblePanelStatus`（CordisPanel.tsx:62-75）逐行等价物。
//
// 判据顺序就是官方的顺序，**不能重排**：
//
//	① awaiting-approval（CordisPanel.tsx:69-71）——本宿主不发这条帧，保留判据
//	② failed（latestRun.status == 'failed' 且 latestRun.packageId == 选中包，:72）
//	③ idle（没有 activeRun，:73）
//	④ 否则 cordisVisibleStatus(row, activeRun.packageId, loaded)（:74）
func visiblePanelStatus(row apiclient.CordisRow, selectedPackageID string, loaded []loadedClientHalf) panelStatus {
	latest := row.LatestRun
	if latest != nil && latest.Status == cordisRunStatusAwaitingApproval {
		return panelStatusAwaitingApproval
	}
	if latest != nil && latest.Status == cordisRunStatusFailed && latest.PackageID == selectedPackageID {
		return panelStatusFailed
	}
	if row.ActiveRun == nil {
		return panelStatusIdle
	}
	switch cordisVisibleStatusOf(row, row.ActiveRun.PackageID, loaded) {
	case cordisStatusRunning:
		return panelStatusRunning
	case cordisStatusClientPending:
		return panelStatusClientPending
	default:
		return panelStatusIdle
	}
}

// panelStatusLabel 是官方 `STATUS_LABELS[status]`（CordisPanel.tsx:29-35）的取值。
func panelStatusLabel(status panelStatus) string {
	switch status {
	case panelStatusAwaitingApproval:
		return zhStatusAwaitingApproval
	case panelStatusClientPending:
		return zhStatusClientPending
	case panelStatusRunning:
		return zhStatusRunning
	case panelStatusFailed:
		return zhStatusFailed
	default:
		return zhStatusIdle
	}
}

// panelDotColor 是官方 `panelDotState`（CordisPanel.tsx:77-86）+ StateDot 的令牌映射。
//
// 官方 StateDot 的五态（idle / ongoing / done / warning / error）在本仓的令牌：
//
//	idle    → LabelTertiary（中性灰；官方 idle 就是中性）
//	ongoing → BrandPrimary（"在动"的品牌色）
//	done    → Success（跑起来了）
//	warning → Warn（待审批）
//	error   → Error（失败）
//
// 与 `plugins/dsagentteam/board.go:61-75` 的 StateDot 同义映射一致（同色的语义，
// 不写十六进制色号）。`busy` 覆盖状态本身（官方 `if (busy) return 'ongoing'`）。
func panelDotColor(t *pluginapi.Theme, status panelStatus, busy bool) ui.Color {
	if busy {
		return t.BrandPrimary
	}
	switch status {
	case panelStatusClientPending:
		return t.BrandPrimary
	case panelStatusRunning:
		return t.Success
	case panelStatusAwaitingApproval:
		return t.Warn
	case panelStatusFailed:
		return t.Error
	default:
		return t.LabelTertiary
	}
}

// runModeOf 是官方 `runMode`（CordisPanel.tsx:248-249）：
// `currentPackageId !== undefined && selected !== currentPackageId ? 'update' : 'run'`。
//
// 调用方负责把它原样交给 `Actions.CordisRun` 的 mode —— 客户端层不猜
// （见 `internal/apiclient/cordis.go` 的 `CordisRunHostHalf` 注释）。
func runModeOf(row apiclient.CordisRow, selectedPackageID string) string {
	if row.CurrentPackageID != "" && selectedPackageID != row.CurrentPackageID {
		return apiclient.CordisRunModeUpdate
	}
	return apiclient.CordisRunModeRun
}
