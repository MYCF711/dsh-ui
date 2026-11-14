// agent_preset_wiring.go —— 官方 `agentPresets.list` 的数据面接线（#3）。
//
// # 为什么单独一个文件
//
// 「Agent 预设」设置页要的是**一份名册**，它不是 Host 设置：
// `settings.describe` 里没有它，官方由 `agent-preset-registry` 服务发布
// （`OFF:dsh-client-ui-agent-preset/src/client/settings-store.ts:36-46`）。
// 所以它有自己的一份缓存、自己的拉取与自己的失败语义——与
// plugin_inventory_wiring.go 同一个形状，不塞进 settings_panel_wiring.go。
//
// # 缓存策略（与 SettingsDescribe / PluginInventory 同一套）
//
//   - `AgentPresetRoster` **同步返回缓存**：插件在 draw 里调，等 HTTP 会卡首帧；
//   - 首次问触发一次后台拉取；写路径（设为默认）成功后壳重拉一次；
//   - 拉失败**保留上一次**（别把已看到的名册清空），原因写状态栏；
//   - 拉成功后 `Invalidate`，插件下一帧读到新值。
//
// # 为什么区分"没接线"与"名册是空的"
//
// 宿主没挂 `OfficialAgentAPI.AgentPresetsList` 时回 501 `not_implemented`：
// 那是**没有这条能力**，那一页必须显示"未知"，而不是"暂无预设"——后者是在说
// "宿主查过了，一个都没有"，那是另一句话（官方 `readRoster` 对
// `gateway/invocation-unavailable` 也是这么区分的）。
package main

import (
	"context"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// agentPresetTimeout 是一次名册读的上限（与设置面同一档）。
const agentPresetTimeout = 8 * time.Second

// refreshAgentPresets 重新拉一次 preset 名册并发布到缓存。
func (m *model) refreshAgentPresets() {
	if m.cli == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), agentPresetTimeout)
	defer cancel()
	roster, err := m.cli.AgentPresetsList(ctx)

	m.mu.Lock()
	m.agentPresetPending = false
	if err != nil {
		// 保留上一次（可能是 nil = 还没成功过），只报状态栏。
		m.agentPresetErr = err
		m.mu.Unlock()
		m.setStatus("读 Agent 预设失败: " + err.Error())
		return
	}
	m.agentPresetRoster = roster
	m.agentPresetErr = nil
	m.mu.Unlock()
	m.inv.Invalidate()
}

// agentPresetRosterSnapshot 是给插件用的同步快照（首次问触发后台拉取）。
//
// 与 settingsDescribeSnapshot 同一个防重入形状：每帧都问，但只发一次请求。
func (m *model) agentPresetRosterSnapshot() *apiclient.AgentPresetRoster {
	m.mu.Lock()
	roster := m.agentPresetRoster
	pending := m.agentPresetPending
	m.mu.Unlock()
	if !pending && roster == nil && m.cli != nil {
		m.mu.Lock()
		m.agentPresetPending = true
		m.mu.Unlock()
		go m.refreshAgentPresets()
	}
	return roster
}

// agentPresetRefresh 是「名字册变了，重读一次」的显式入口（与
// pluginInventoryRefresh 同一个形状）。
//
// 「设为默认」写完后壳会自己重拉一次；这个入口是给"宿主在别处改了名册
// （例如新装了 `<home>/modes/x.toml`）"时用的——页面上的显式重读。
func (m *model) agentPresetRefresh() {
	m.mu.Lock()
	if m.agentPresetPending {
		m.mu.Unlock()
		return // 已经有一次在飞，别叠请求
	}
	m.agentPresetPending = true
	m.mu.Unlock()
	go m.refreshAgentPresets()
}
