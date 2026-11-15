// cordis.go —— 官方 `dynamicCordisRunner.*` 的客户端（GUI 账本 #12）。
//
// # 官方契约（唯一权威）
//
//	OFF = DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai
//	OFF:dsh-cordis-host-runner/src/types.ts:95      CordisDynamicRunMode = 'run' | 'update'
//	OFF:dsh-cordis-host-runner/src/types.ts:109-118  CordisRunStatus 九值
//	OFF:dsh-cordis-host-runner/src/types.ts:121-128  CordisHalfState
//	OFF:dsh-cordis-host-runner/src/types.ts:131-145  CordisRunDiagnostic（phase 六值）
//	OFF:dsh-cordis-host-runner/src/types.ts:147-166  DynamicCordisRunAttempt
//	OFF:dsh-cordis-host-runner/src/types.ts:219-230  DynamicCordisInventoryPackage
//	OFF:dsh-cordis-host-runner/src/types.ts:233-251  DynamicCordisInventoryRow
//	OFF:dsh-cordis-host-runner/src/types.ts:254-256  DynamicCordisUndefineReceipt
//	OFF:dsh-cordis-host-runner/src/types.ts:308-310  DynamicCordisStopResponse
//	OFF:dsh-cordis-host-runner/src/types.ts:313-323  DynamicCordisHostHalfResult
//	OFF:dsh-cordis-host-runner/lib/typert.host.js:13-19 / :146-170 / :64-81
//	    12 个方法的 zod 入参形状（runHostHalf 是 6 个位置参数，见 :146-157）
//
// # 本宿主的 wire（不是位置参数，是命名参数）
//
// 官方是进程内调用（`ctx.remote.dynamicCordisRunner.inventory()`），没有可抄的
// RPC wire。本仓宿主（`HOST:cmd/dshd/serve_cordis_api.go:640-`）用**命名参数**
// 承载官方的第一个位置参数 `agent`：面板/事件路径用 `sessionId`，另 5 个方法用
// `agentId`（宿主两个都收，见同一处 `session := str("sessionId")` 的归一）。
// 这里逐方法照那个约定发，不发明第三种名字。
//
// # 业务失败是 200 里的值，不是四字段错误
//
// stop / undefine / runHostHalf / invoke 的官方结果都是**判别联合**，`ok:false`
// 带着真实理由（`plugin-missing` / `not-running` / 加载失败原文）在 200 体里回来。
// 所以这里保留 union 形状：`Call` 只报传输层失败，业务失败由调用方读 `OK`。
// 把它们折成 error 会让"插件不存在"看起来像"宿主连不上"。
package apiclient

import (
	"context"
	"time"
)

// officialCordisRunner 是官方 remote 命名空间（12 个方法共用这一个前缀）。
const officialCordisRunner = "dynamicCordisRunner."

// 官方 12 方法里的取用面（本仓界面真的在调的那 4 个）。
//
// 另外 8 个（getClientCode / invoke / settleUserRun / resolveRequestRun /
// reportClientGuardFailure / reportRenderFailure / resolveInspectQuery /
// syncInspectManifest）本仓**不调**，理由不是"来不及"：
//   - 前 4 个要求一个"能在本页加载并运行 Client 半 JS"的宿主，Go+Viva 没有 JS
//     组件宿主（账本 #12 的 `tool.view.cordis` 就是因为它整块 HIDE 的）；
//   - 两个 report 是那个宿主失败时的回报口，没有客户端宿主就没有要回报的失败；
//   - inspect 两个属于 `dsh-tool-cordis`（只读 inspect），是另一个包的事。
const (
	MethodCordisInventory         = officialCordisRunner + "inventory"
	MethodCordisStopFromPanel     = officialCordisRunner + "stopFromPanel"
	MethodCordisUndefineFromPanel = officialCordisRunner + "undefineFromPanel"
	MethodCordisRunHostHalf       = officialCordisRunner + "runHostHalf"
)

// CordisRunMode 是官方 `CordisDynamicRunMode`（types.ts:95）。
const (
	CordisRunModeRun    = "run"
	CordisRunModeUpdate = "update"
)

// CordisPackage 是官方 `DynamicCordisInventoryPackage`（types.ts:219-230）。
type CordisPackage struct {
	PackageID     string `json:"packageId"`
	Name          string `json:"name"`
	Purpose       string `json:"purpose"`
	HasHostHalf   bool   `json:"hasHostHalf"`
	HasClientHalf bool   `json:"hasClientHalf"`
}

// CordisHalf 是官方 `CordisHalfState`（types.ts:121-128）。
//
// `status` 六值：absent / pending / stopped / running / waiting / failed。
// `waitingFor` 在本宿主恒为空数组（没有 Cordis 服务依赖图，无缺失服务即空，
// 与官方 `missingFor()` 在"不缺"时返回 `[]` 同义）。
type CordisHalf struct {
	Status     string   `json:"status"`
	WaitingFor []string `json:"waitingFor"`
	Error      string   `json:"error,omitempty"`
}

// CordisDiagnostic 是官方 `CordisRunDiagnostic`（types.ts:131-145）。
//
// `phase` 六值：approval / host-load / host-apply / client-load / client-apply /
// client-render。本宿主只可能给 `host-load` / `host-apply` / `approval`。
type CordisDiagnostic struct {
	Phase       string `json:"phase"`
	Message     string `json:"message"`
	PluginID    string `json:"pluginId"`
	PackageID   string `json:"packageId"`
	PluginRunID string `json:"pluginRunId"`
}

// CordisRunAttempt 是官方 `DynamicCordisRunAttempt`（types.ts:147-166）。
//
// `ApprovalRequestID` / `RequiresApproval` 本宿主**不出现**（没有 Cordis 审批队列），
// 因此按官方可选字段处理：空串 / false = 没有待审批请求。
type CordisRunAttempt struct {
	PluginRunID       string            `json:"pluginRunId"`
	PackageID         string            `json:"packageId"`
	Mode              string            `json:"mode"`
	Status            string            `json:"status"`
	ApprovalRequestID string            `json:"approvalRequestId,omitempty"`
	RequiresApproval  bool              `json:"requiresApproval,omitempty"`
	Host              CordisHalf        `json:"host"`
	Client            CordisHalf        `json:"client"`
	Error             *CordisDiagnostic `json:"error,omitempty"`
}

// CordisRunRef 是官方 `activeRun` 的内联形状（types.ts:245-248）。
type CordisRunRef struct {
	PluginRunID string `json:"pluginRunId"`
	PackageID   string `json:"packageId"`
}

// CordisRow 是官方 `DynamicCordisInventoryRow`（types.ts:233-251）。
//
// # 本宿主如实缺席的字段（界面必须能处理空值，不许补默认值）
//
//	agentId          可能为空串。官方在 define 时把 Plugin 绑到一个会话；本宿主
//	                 没有 define 通道，归属只来自"激活/面板调用时带的 sessionId"。
//	                 空 = 官方 UI 的「其他会话」那一组（不是"当前会话"）。
//	packages[].purpose  恒空串（盘上没有来源）。官方定义卡用 `purpose.missing`
//	                 文案兜底，面板那一行就空着。
//	nextPackageId    不出现（本宿主不做失败的版本切换）。
//	currentPackageId 只在**结算成功**（或没有 Client 半的原位 commit）后出现。
type CordisRow struct {
	PluginID         string            `json:"pluginId"`
	AgentID          string            `json:"agentId"`
	Packages         []CordisPackage   `json:"packages"`
	CurrentPackageID string            `json:"currentPackageId,omitempty"`
	NextPackageID    string            `json:"nextPackageId,omitempty"`
	ActiveRun        *CordisRunRef     `json:"activeRun,omitempty"`
	LatestRun        *CordisRunAttempt `json:"latestRun,omitempty"`
}

// CordisStopResponse 是官方 `DynamicCordisStopResponse`（types.ts:308-310）。
type CordisStopResponse struct {
	OK      bool   `json:"ok"`
	Reason  string `json:"reason,omitempty"` // plugin-missing | not-running
	Message string `json:"message,omitempty"`
}

// CordisUndefineReceipt 是官方 `DynamicCordisUndefineReceipt`（types.ts:254-256）。
type CordisUndefineReceipt struct {
	OK         bool   `json:"ok"`
	WasRunning bool   `json:"wasRunning,omitempty"`
	Reason     string `json:"reason,omitempty"` // plugin-missing
	Message    string `json:"message,omitempty"`
}

// CordisHostHalfResult 是官方 `DynamicCordisHostHalfResult`（types.ts:313-323）。
type CordisHostHalfResult struct {
	OK          bool     `json:"ok"`
	PluginID    string   `json:"pluginId,omitempty"`
	PackageID   string   `json:"packageId,omitempty"`
	PluginRunID string   `json:"pluginRunId,omitempty"`
	WaitingFor  []string `json:"waitingFor,omitempty"`
	StartedHere bool     `json:"startedHere,omitempty"`
	Message     string   `json:"message,omitempty"`
	Stack       string   `json:"stack,omitempty"`
}

// cordisReadTimeout 是一次 inventory 读的上限。
//
// 宿主这一路只读内存表 + 盘上的 package.json（不启动 node），所以比 forge 的
// git 读快得多；给 10s 是为了容住"几十个插件目录首次补元数据"的冷启动。
const cordisReadTimeout = 10 * time.Second

// cordisRunTimeout 是一次 runHostHalf 的上限。
//
// 它会**真的拉起外部 node 并加载插件包**（`internal/cordis/extern`），比读慢一个
// 数量级。超时不是"失败"而是"还在跑"：面板按 official 语义继续显示上一态，
// 由 `cordis/dynamic-package` 事件或下一次刷新收敛。
const cordisRunTimeout = 60 * time.Second

// CordisInventory 读宿主当前的 Cordis 定义清单（官方 `dynamicCordisRunner.inventory`）。
//
// 官方语义：**一次全局读**，结果不带会话过滤（面板要列"其他会话"的行）。
// 空切片 = 宿主真的一个定义都没有；它不是错误，也不等于"还没读到"。
func (c *Client) CordisInventory(ctx context.Context) ([]CordisRow, error) {
	readCtx, cancel := context.WithTimeout(ctx, cordisReadTimeout)
	defer cancel()
	var rows []CordisRow
	if err := c.Call(readCtx, MethodCordisInventory, struct{}{}, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// CordisStopFromPanel 停掉一个插件的当前运行，保留它的 Package
// （官方 `stopFromPanel(agent, pluginId)`）。
//
// 官方面板的语义（OFF:dsh-client-ui-cordis/src/client/index.ts:46-51）：
// `ok:true` 与 `reason:'not-running'` **都算成功**（"本来就没在跑"= 想要的结果
// 已经成立），只有 `plugin-missing` 与传输失败才是失败。这个折算属于界面策略，
// 所以不放在客户端层——这里如实把宿主回执交回去。
func (c *Client) CordisStopFromPanel(ctx context.Context, sessionID, pluginID string) (CordisStopResponse, error) {
	var out CordisStopResponse
	err := c.Call(ctx, MethodCordisStopFromPanel, map[string]string{
		"sessionId": sessionID,
		"pluginId":  pluginID,
	}, &out)
	return out, err
}

// CordisUndefineFromPanel 停掉并移除一个插件及其全部 Package
// （官方 `undefineFromPanel(agent, pluginId)`）。
func (c *Client) CordisUndefineFromPanel(ctx context.Context, sessionID, pluginID string) (CordisUndefineReceipt, error) {
	var out CordisUndefineReceipt
	err := c.Call(ctx, MethodCordisUndefineFromPanel, map[string]string{
		"sessionId": sessionID,
		"pluginId":  pluginID,
	}, &out)
	return out, err
}

// CordisRunHostHalf 起一个插件的 **Host 半**（官方 `runHostHalf(agent, pluginId,
// packageId, mode, requestId, approveFutureVersions)`）。
//
// # 为什么本仓只发前 4 个参数
//
// `requestId` 是"模型侧发起的激活请求"的关联号，`approveFutureVersions` 是那次
// 审批的答复——本宿主没有 `cordis/request-run` 审批队列（`accepted` 恒 false，
// 见 HOST:cmd/dshd/serve_cordis_api.go 的文件头），面板路径上官方自己也传
// `null` / `false`（OFF:…/orchestrator.ts 的 startUserRun）。所以这里不发这两个
// 字段：宿主对缺省值按 null / false 处理，与官方面板路径逐字等价。
//
// # `mode`
//
// 官方面板算 mode 的规则（OFF:CordisPanel.tsx:248-249）是
// `currentPackageId !== undefined && selected !== currentPackageId ? 'update' : 'run'`。
// 调用方按那条规则算好再传，客户端不猜。
func (c *Client) CordisRunHostHalf(ctx context.Context, agentID, pluginID, packageID, mode string) (CordisHostHalfResult, error) {
	runCtx, cancel := context.WithTimeout(ctx, cordisRunTimeout)
	defer cancel()
	var out CordisHostHalfResult
	err := c.Call(runCtx, MethodCordisRunHostHalf, map[string]string{
		"agentId":   agentID,
		"pluginId":  pluginID,
		"packageId": packageID,
		"mode":      mode,
	}, &out)
	return out, err
}
