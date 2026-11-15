// market_detail.go —— 官方 `dsh-client-ui-plugin-market`（`#30` C 块）详情页的
// 壳侧数据面：`pluginMarket.readEntryDetail`。
//
// # 为什么壳要有这一份
//
// 官方详情页（`EntryDetail` 里的 `PluginDetailSection`）消费的是
// `pluginMarket.readEntryDetail(url, name, version, language)`；本仓 Transport 没有
// `plugin-market/changed` 推送，所以刷新边界与插件管理页、任务页同一条先例：
// **详情页第一次问时拉一次、失败后给重试、连接重置后作废重来**。
//
// 插件侧只读 `Actions.MarketEntryDetail`（同步、走缓存），因此不会在 Draw 里发请求
// （`pluginapi.Env` 的注释：插件不该知道数据怎么来的）。
//
// # 三条边界
//
//  1. **失败不清旧值**：已经读到的文档保持可见，只记失败档让页面画重试。
//  2. **失败档是官方六档，不是自由文本**：`apiclient.PluginMarketDetailFailureOf`
//     逐个映射（只有 `plugin-market/detail-unavailable` 带结构化 reason，其余一律
//     other，判据与官方客户端 `market-store.ts:142` 一致）。
//  3. **不缓存 501/网络抖动这类"还没读到"**：`loaded=false` + `failure=""` 就是
//     官方 `loading` 态；下次问会自动再试（不会永远卡在 loading，因为失败会记档）。
package main

import (
	"context"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// marketDetailKey 是一次详情读的键：官方四个位置参数里的前三个 + 语言。
//
// 语言进键是必须的：宿主按语言挑本地化兜底文案（`HOST:cmd/dshd/serve_plugin_market.go`
// 的 localizedMarketText），同一个版本换语言就是另一次读。
type marketDetailKey struct {
	url      string
	name     string
	version  string
	language string
}

// marketDetailRead 是一次详情读的缓存格子。
type marketDetailRead struct {
	loaded  bool
	pending bool
	detail  apiclient.PluginMarketEntryDetail
	failure apiclient.PluginMarketDetailFailure
}

// marketEntryDetail 是 `Actions.MarketEntryDetail` 的实现：同步返回缓存，
// 首次问时触发一次后台拉取（插件每帧都会问，与 pluginBundleDetail 同一模式）。
func (m *model) marketEntryDetail(url, name, version, language string) (apiclient.PluginMarketEntryDetail, bool, apiclient.PluginMarketDetailFailure) {
	key := marketDetailKey{url: url, name: name, version: version, language: language}
	m.mu.Lock()
	if m.marketDetails == nil {
		m.marketDetails = map[marketDetailKey]*marketDetailRead{}
	}
	entry := m.marketDetails[key]
	if entry == nil {
		entry = &marketDetailRead{}
		m.marketDetails[key] = entry
	}
	start := !entry.loaded && !entry.pending && entry.failure == ""
	if start {
		entry.pending = true
	}
	detail, loaded, failure := entry.detail, entry.loaded, entry.failure
	m.mu.Unlock()

	if start {
		go m.refreshMarketEntryDetail(key)
	}
	return detail, loaded, failure
}

// marketEntryDetailRetry 是 `Actions.MarketEntryDetailRetry`：绕过缓存重读一次。
//
// 官方 `retryDetail` 就是带 `refresh: true` 再读（`market-store.ts:229`）；本仓宿主
// 不读 `refresh`（它没有缓存那一层，每次都读真索引），所以这里等价于"清掉失败档再读"。
func (m *model) marketEntryDetailRetry(url, name, version, language string) {
	key := marketDetailKey{url: url, name: name, version: version, language: language}
	m.mu.Lock()
	if m.marketDetails == nil {
		m.marketDetails = map[marketDetailKey]*marketDetailRead{}
	}
	entry := m.marketDetails[key]
	if entry == nil {
		entry = &marketDetailRead{}
		m.marketDetails[key] = entry
	}
	if entry.pending {
		m.mu.Unlock()
		return // 已经有一次在飞，别叠加
	}
	entry.pending = true
	entry.failure = ""
	entry.loaded = false
	m.mu.Unlock()
	go m.refreshMarketEntryDetail(key)
}

// refreshMarketEntryDetail 真的读一次。
func (m *model) refreshMarketEntryDetail(key marketDetailKey) {
	ctx, cancel := context.WithTimeout(context.Background(), pluginCallTimeout)
	defer cancel()
	detail, err := m.cli.PluginMarketReadEntryDetail(ctx, key.url, key.name, key.version, key.language)

	m.mu.Lock()
	entry := m.marketDetails[key]
	if entry != nil {
		entry.pending = false
		if err != nil {
			// 失败档按官方六档投影；旧值**不清**（已经读到的文档保持可见）。
			entry.failure = apiclient.PluginMarketDetailFailureOf(err)
		} else {
			entry.detail = detail
			entry.loaded = true
			entry.failure = ""
		}
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}

// # 为什么**没有**"连接重置就作废详情缓存"
//
// 官方在 `connection/reset` 上把详情读全清（`OFF:…/client/index.ts:46-49` 的
// `controller.detailsReset()`），理由是重启后的宿主不再认得旧的 `document` id。
// 本仓那个 id **只用于**拼 `GET /api/plugin-market/detail-file` 的图片地址，而那条
// 路由本仓没有、图片也不解析（见证据文档 §4 缺口 2）。所以这里不加一个永远没人调的
// `resetXxx`：等 detail-file 路由落地、document id 真被用起来时，再在重连路径上接一次清空。
