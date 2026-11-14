// plugin_inventory_wiring.go —— 官方 `pluginInventory.list` 的数据面接线。
//
// # 为什么单独一个文件
//
// 「内置插件」设置页要的是**一份清单**，与 `settings.describe` 是两条独立的
// 宿主能力（官方也不在 describe 里给这份数据：它是 Cordis Loader 的只读投影，
// `dsh-host-plugin-inventory/src/index.ts:82-121`）。所以它有自己的一份缓存、
// 自己的拉取与自己的失败语义，不塞进 settings_panel_wiring.go。
//
// # 缓存策略（与 SettingsDescribe / ModelsList / JobsList 同一套）
//
//   - `PluginInventory` **同步返回缓存**：插件在 draw 里调，等 HTTP 会卡首帧；
//   - 首次问触发一次后台拉取；`PluginInventoryRefresh` 再显式触发（页内「重试」）；
//   - 拉失败**保留上一次**（别把已看到的清单清空），原因写状态栏；
//   - 拉成功后 `Invalidate`，插件下一帧就读到新值。
//
// # 为什么区分"没接线"与"清单是空的"
//
// 宿主没挂 `OfficialPluginManagerAPI.PluginInventoryList` 时回 501
// `not_implemented`；那是**没有这条能力**，界面必须显示"未知·宿主未接"，
// 而不是"暂无插件"——后者是在说"宿主查过了，一个都没有"，那是另一句话。
package main

import (
	"context"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// pluginInventoryTimeout 是一次清单读的上限（与设置面同一档）。
const pluginInventoryTimeout = 8 * time.Second

// refreshPluginInventory 重新拉一次插件清单并发布到缓存。
func (m *model) refreshPluginInventory() {
	ctx, cancel := context.WithTimeout(context.Background(), pluginInventoryTimeout)
	defer cancel()
	inventory, err := m.cli.PluginInventoryList(ctx)

	m.mu.Lock()
	m.pluginInventoryPending = false
	if err != nil {
		// 保留上一次（可能是 nil = 还没成功过），只报状态栏。
		m.pluginInventoryErr = err
		m.mu.Unlock()
		m.setStatus("读插件清单失败: " + err.Error())
		return
	}
	m.pluginInventory = &inventory
	m.pluginInventoryErr = nil
	m.mu.Unlock()
	m.inv.Invalidate()
}

// pluginInventorySnapshot 是给插件用的同步快照（首次问触发后台拉取）。
//
// 与 settingsDescribeSnapshot 同一个防重入形状：每帧都问，但只发一次请求。
func (m *model) pluginInventorySnapshot() *apiclient.PluginInventoryResult {
	m.mu.Lock()
	inventory := m.pluginInventory
	pending := m.pluginInventoryPending
	m.mu.Unlock()
	if !pending && inventory == nil {
		m.mu.Lock()
		m.pluginInventoryPending = true
		m.mu.Unlock()
		go m.refreshPluginInventory()
	}
	return inventory
}

// pluginInventoryRefresh 是插件点「重试」时走的路径（也用于"清单变了，重读"）。
func (m *model) pluginInventoryRefresh() {
	m.mu.Lock()
	if m.pluginInventoryPending {
		m.mu.Unlock()
		return // 已经有一次在飞，别叠请求
	}
	m.pluginInventoryPending = true
	m.mu.Unlock()
	go m.refreshPluginInventory()
}
