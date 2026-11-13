// desktop_update.go —— 官方「更新渠道」壳桥的客户端。
//
// 契约来源：官方 `dsh-client-ui-settings-general/src/types.ts:29-74`。
//
// 官方这条**不是** Host 设置（`types.ts:60`：「UI choices persist in the shell
// configuration, independently of Host settings」），数据由 Electron 主进程经隔离
// preload 桥暴露。本仓宿主提供同形的壳服务（DSH-GO `desktopUpdate.channel.*`），
// 所以这里按同一套形状消费：
//
//	get  -> { channel: "stable"|"nightly"|null, packagedChannel, changing, error? }
//	set  -> { kind: "applied"|"unchanged"|"failed", state, reason? }
//
// **`channel` 为 null 是合法状态**：官方语义是"用户还没选过"，界面据此显示
// 「配置不可用」。不要把它替成 packaged —— 那是自造事实。
package apiclient

import "context"

// DesktopUpdateChannel 是官方 `DesktopUpdateChannel`（types.ts:45）。
type DesktopUpdateChannel string

const (
	// DesktopUpdateChannelStable 是正式版。
	DesktopUpdateChannelStable DesktopUpdateChannel = "stable"
	// DesktopUpdateChannelNightly 是开发版。
	DesktopUpdateChannelNightly DesktopUpdateChannel = "nightly"
)

// DesktopChannelState 是官方 `DesktopChannelState`（types.ts:48-53）。
type DesktopChannelState struct {
	// Channel 是当前用户选择；nil = 还没选过（界面显示「配置不可用」）。
	Channel *DesktopUpdateChannel `json:"channel"`
	// PackagedChannel 是本构建自带的渠道（安装包决定）。
	PackagedChannel DesktopUpdateChannel `json:"packagedChannel"`
	// Changing 报告是否有一次切换正在进行。
	Changing bool `json:"changing"`
	// Error 是壳级故障（read / format / channel / cleanup）；空 = 正常。
	Error string `json:"error,omitempty"`
}

// DesktopChannelChangeResult 是官方 `DesktopChannelChangeResult`（types.ts:56-58）。
//
// `Kind` 三态：applied / unchanged / failed。**failed 不是请求失败**——
// 它是"用户选择被拒"，所以宿主回 200，界面看 kind 渲染。
type DesktopChannelChangeResult struct {
	Kind   string              `json:"kind"`
	State  DesktopChannelState `json:"state"`
	Reason string              `json:"reason,omitempty"`
}

// DesktopUpdateChannelGet 读当前更新渠道状态（官方壳桥的 `get()`）。
//
// 宿主没接线时会拿到 501（*APIError），调用方据此判定"这条能力不存在"，
// 不要把 501 当成"渠道是空"。
func (c *Client) DesktopUpdateChannelGet(ctx context.Context) (DesktopChannelState, error) {
	var out DesktopChannelState
	if err := c.Call(ctx, "desktopUpdate.channel.get", map[string]any{}, &out); err != nil {
		return DesktopChannelState{}, err
	}
	return out, nil
}

// DesktopUpdateChannelSet 持久化并应用一个渠道选择（官方壳桥的 `set()`）。
func (c *Client) DesktopUpdateChannelSet(ctx context.Context, channel DesktopUpdateChannel) (DesktopChannelChangeResult, error) {
	var out DesktopChannelChangeResult
	params := map[string]any{"channel": string(channel)}
	if err := c.Call(ctx, "desktopUpdate.channel.set", params, &out); err != nil {
		return DesktopChannelChangeResult{}, err
	}
	return out, nil
}

// ---------- 更新状态（官方 `DesktopUpdateBridge` 的 status() / open()）----------
//
// 与上面的渠道族是**兄弟**：官方 `types.ts:30-35` 的 `DesktopUpdateBridge`
// 有 status / open / subscribe 三个成员，`types.ts:60-74` 的 `DesktopChannelBridge`
// 有 get / set / subscribe 三个。本仓宿主两族都服务（DSH-GO
// `internal/transport/handlers_desktop_update.go`），本文件两族都消费。
//
// **本仓不消费 `subscribe`**（NDJSON 长连接）：GUI 壳没有 NDJSON 流客户端，
// 而宿主 `status()` 是**不阻塞 + TTL 缓存 + 单飞**的（宿主
// `internal/updater/status.go` 的 `Status()`），轮询它的代价与一条长连接相同量级
// 且更好收拾（无连接生命周期、无重连状态机）。选轮询的理由见
// `cmd/app/desktop_update_wiring.go` 的文件头。

// DesktopUpdatePresentation 是官方 `DesktopUpdatePresentation`（`types.ts:21-27`）
// 在宿主线上的形状（宿主 `internal/updater/status.go` 的同名结构，字段逐字）。
//
// 四态之外**不会出现**：本仓没有下载器/校验器/安装器，所以
// `downloading` / `verifying` / `installing` / `ready` 一个都不会来，
// `percent` 永远是 nil（宿主 `ProduciblePhases()` 是这条的可执行判据）。
type DesktopUpdatePresentation struct {
	// Phase 是 `idle` / `checking` / `available` / `error` 之一（宿主只产这四个）。
	Phase string `json:"phase"`
	// Version 只在 available 时有值（规范化写法，不带前导 v）。
	Version string `json:"version,omitempty"`
	// Percent 官方用于 downloading 的进度；本仓宿主永远不填。
	Percent *int `json:"percent,omitempty"`
	// Failure 只在 error 时有值；本仓只会是 configuration / check / check-network。
	Failure string `json:"failure,omitempty"`
	// ConfigurationError 是壳渠道的故障码（read/format/channel/cleanup）。
	//
	// 官方渲染层**先判它**（`DesktopUpdateIndicator.tsx:15`）：一旦有值，
	// phase/version/failure 全部被忽略。所以它不是"另一个 error 字段"。
	ConfigurationError string `json:"configurationError,omitempty"`
}

// DesktopUpdateOpenResult 是 `desktopUpdate.open` 的结果（宿主 `updater.OpenResult`）。
//
// 官方 `open(): Promise<void>` 不回值——它自己弹窗口。本仓宿主没有窗口，
// 所以**把入口交出来**：`URL` 一定非空（最差是 releases 列表页）。
// 界面负责拿它去唤起系统默认浏览器，宿主自己不开浏览器（那是不可逆的对外动作）。
type DesktopUpdateOpenResult struct {
	// URL 是更新入口：有更新时是那条 release 的页面，否则是 releases 列表页。
	URL string `json:"url"`
	// Channel 是本次检查实际用的渠道（用户选择，没选过则是打包渠道）。
	Channel string `json:"channel"`
	// Presentation 是这次检查的结论（界面不用再问一次 status）。
	Presentation DesktopUpdatePresentation `json:"presentation"`
}

// DesktopUpdateStatus 读当前更新状态（官方壳桥的 `status()`）。
//
// **不阻塞**：宿主拿到的是"此刻已知的状态"，检查在后台跑。所以这一次可能回
// `checking`，要**再问一次**（或等下一次轮询）才拿到结论——调用方不许把
// `checking` 当成答案。宿主没接线时回 501（*APIError），调用方据此判定
// "这条能力不存在"（= 官方没有 `dshDesktop.updates` 桥），**不要**当成 `error` 相。
func (c *Client) DesktopUpdateStatus(ctx context.Context) (DesktopUpdatePresentation, error) {
	var out DesktopUpdatePresentation
	if err := c.Call(ctx, "desktopUpdate.status", struct{}{}, &out); err != nil {
		return DesktopUpdatePresentation{}, err
	}
	return out, nil
}

// DesktopUpdateOpen 是官方壳桥的 `open()`：用户点了，交出更新入口。
//
// 宿主这一次**绕过 TTL 真查一遍**（那是"用户主动点"的语义），所以它比
// `status()` 慢；并发点击在宿主侧仍然只发一次请求（单飞）。
func (c *Client) DesktopUpdateOpen(ctx context.Context) (DesktopUpdateOpenResult, error) {
	var out DesktopUpdateOpenResult
	if err := c.Call(ctx, "desktopUpdate.open", struct{}{}, &out); err != nil {
		return DesktopUpdateOpenResult{}, err
	}
	return out, nil
}
