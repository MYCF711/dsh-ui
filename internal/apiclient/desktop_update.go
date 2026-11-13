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
