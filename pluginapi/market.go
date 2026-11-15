// market.go —— 官方插件市场「发现」tab 的**壳 → 插件**数据形状（`#30` A/B 块）。
//
// 它只描述"壳缓存里现在有什么"，不是宿主契约：宿主那一侧的类型是
// `internal/apiclient` 的 `MarketSearchResult`（官方 `MarketSearchResult` 的 Go 形状）。
// 之所以另起一个结构：插件要的是"这次结果回答的是哪个 query、还在加载还是已经失败"，
// 这些是官方客户端控制器（`market-store.ts` 的 `ListingState`）的状态，不是宿主下发的。
//
// 官方对照：
//
//	OFF:../dsh-client-ui-plugin-market/src/client/market-store.ts:20-33
//	     ListingState{url, query, status:'loading'|'more'|'ready'|'error', value, answered, failure}
//	OFF:../dsh-client-ui-plugin-market/src/client/market-store.ts:36-49
//	     DiscoverState{status:'idle'|'loading'|'ready'|'error', indexes, selected, listing, …}
package pluginapi

import (
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/vivaui/viva/ui"
)

// 官方 `ListingState.status` 的四个取值（逐字，`market-store.ts:24`）。
const (
	// MarketListingLoading 是"第一页/新 query 正在读"。
	MarketListingLoading = "loading"
	// MarketListingMore 是"下一页正在读"（旧结果仍然显示）。
	MarketListingMore = "more"
	// MarketListingReady 是"这批结果已经落定"。
	MarketListingReady = "ready"
	// MarketListingError 是"最近一次读失败了"（旧结果仍然显示，官方同样保留）。
	MarketListingError = "error"
)

// PluginsTab 是一个 `plugins.tab` 贡献方（官方 `slot-contract.ts:135-141` 的 list 槽一项）。
//
// 官方那边贡献方是 React 组件，本仓是一个能画的对象；三个方法就是 Plugins 页真正
// 用到的那三件事：叫什么（tab 名）、画什么（tab 体）、详情页开着没有（详情页那一帧
// 页面要不要让出自己那层页内距）。
type PluginsTab interface {
	// Label 是 tab 文案（官方注册里的 `label`）。
	Label() string
	// Draw 画这一页；env 与页面同源（同一帧、同一棵树）。
	Draw(f ui.C, env *Env)
	// DetailOpen 报告这一页当前是不是在"详情/子页"里（页面据此收内距）。
	DetailOpen() bool
}

// PluginsTabs 是 Plugins 页收 tab 贡献的服务（官方 `plugins.tab` 的注册面）。
type PluginsTabs interface {
	// RegisterPluginsTab 登记一个 tab。同一 id 后登记的那次覆盖前一次
	// （官方槽位同样按 id 去重）。
	RegisterPluginsTab(tab PluginsTab)
}

// MarketListing 是壳缓存的"当前索引的列表状态"（官方 `ListingState` 的投影）。
//
// 不变式（与官方控制器逐条一致）：
//
//	· Value 是**已经落定**的结果；新 query 在飞时它仍是上一批（官方"结果与它回答的
//	  query 一起发布"，`market-store.ts:26-29` 的注释）；
//	· Answered 是 Value 回答的那个 query；nil = 还没有任何结果；
//	· Answered 与 Query 不同 = 新 query 在飞或失败了 —— 插件据此决定画骨架还是
//	  "没有符合条件的插件"（官方 `unfiltered(listing.answered)` 同判据）；
//	· Failure 只在 Status==error 时有值。
type MarketListing struct {
	// URL 是这个列表所属的索引源。
	URL string
	// Query 是**当前**（用户最后选定的）query。
	Query apiclient.MarketQuery
	// Answered 是 Value 回答的 query；nil = 还没有结果。
	Answered *apiclient.MarketQuery
	// Status 是 MarketListingLoading / More / Ready / Error 之一。
	Status string
	// Value 是已落定的结果；nil = 还没有结果（首屏）。
	Value *apiclient.MarketSearchResult
	// Failure 是列表读的失败档（只有 error 时有值）。
	Failure apiclient.MarketFailureKind
}
