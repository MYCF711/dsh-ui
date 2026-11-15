// market_discover.go —— 官方「发现」tab（`dsh-client-ui-plugin-market`，`#30` A/B 块）
// 的壳侧数据面：索引源（`pluginMarket.listIndexes` / `setIndexes`）与列表
// （`pluginMarket.search`）。
//
// # 为什么壳要有这一份
//
// 官方把这条状态机放在客户端控制器里（`OFF:../dsh-client-ui-plugin-market/src/client/
// market-store.ts` 的 `DiscoverController`）：索引源清单、选中项、分页拼接、
// `fetchedAt` 钉住（同一份索引文档才算同一批结果）、声明表裁剪（索引不再声明的分类/
// 标签/排序键从 query 里剔掉）、写回失败档。本仓 Transport 没有
// `plugin-market/changed` 推送，且插件不许持有数据源（R2），所以整台状态机在壳里，
// 插件只读缓存、只发意图。
//
// # 与官方逐条对应的判据
//
//	OFF:market-store.ts:295-311  loadIndexes：选中项 = 新加的最后一个 → 原来那个（还在时）
//	                             → 第一个；一个都没有就是"没有来源"
//	OFF:market-store.ts:318-360  load：一页 50，最多 100/页；拼页时 fetchedAt 必须相同，
//	                             变了就重来，再一次变就报 `changed`
//	OFF:market-store.ts:326-330  declaredQuery：索引不再声明的 id 从 query 里剔掉，
//	                             然后按剔过的 query 重读一次
//	OFF:market-store.ts:363-380  loadMore：下一页追加；文档变了就从头重读
//	OFF:market-store.ts:405-415  saveSources：写回成功后重读索引源清单
//
// # 三条边界
//
//  1. **失败不清旧值**：已经读到的条目保持可见（官方同样只改 status/failure），
//     否则用户会看到列表闪成空白。
//  2. **不做乐观更新**：写索引源成功后一律重读清单，不在本地拼。
//  3. **`registry_unavailable` 对 listIndexes 的含义是"没有配置索引源"**：
//     宿主 `ListIndexes` 在 `ErrNoIndexes` 时回 503 `registry_unavailable`
//     （HOST:cmd/dshd/serve_plugin_market.go:94-99），而官方这个方法是回空数组
//     （`MarketIndexSummary[]`，`OFF:market-store.ts:295-299` 拿空数组画"还没有插件
//     索引"）。这是一处**宿主形状差值**：不认它，全新 profile 上永远只能画
//     "无法读取索引源"，而"添加索引源"按钮只在空态出现 —— 用户就没法添加第一个源。
//     所以这里按"空清单"处理，并在证据文档 §缺口 里登记成宿主待修。
package main

import (
	"context"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

const (
	// marketPageSize 是官方 `PAGE_SIZE`（`market-store.ts:18`）。
	marketPageSize = 50
	// marketMaxPage 是宿主接受的最大 limit（官方 `MAX_PAGE`，`market-store.ts:129`）。
	marketMaxPage = 100
	// marketCallTimeout 是一次市场调用的上限：宿主那边是真 HTTP 抓索引文档，
	// 比本地 RPC 慢得多（与 `pluginCallTimeout` 同一个量级，取宽一点）。
	marketCallTimeout = 20 * time.Second
)

// marketSearchDebounce 是官方那 250ms 输入去抖（`OFF:DiscoverSection.tsx:44,455-459`）。
//
// 做成变量而不是常量只有一个原因：壳的测试要把它调成 0，否则每条用例都要真等
// 250ms 再等 HTTP。生产路径上没有任何地方改它。
var marketSearchDebounce = 250 * time.Millisecond

// marketState 是「发现」tab 的全部壳侧状态（字段见 main.go 的 model）。
type marketState struct {
	// indexes 是索引源清单的缓存。
	indexes marketIndexesCache
	// listing 是当前索引的列表状态（官方 `DiscoverState.listing`）。
	listing *pluginapi.MarketListing
	// pending 表示有一次列表读在飞（防止同一请求叠加）。
	pending bool
	// seq 是列表读的代号：每一次新的 load 自增，迟到的回答按 seq 丢弃。
	seq uint64
	// retry 是"重跑上一次失败的那一次读"（官方 `controller.failed` 闭包）。
	retry func()
	// searchGen 是去抖代号：连续输入只让最后一次真的发出去。
	searchGen uint64
	// selectAdded 表示下一次读到清单时选**最后一条**（刚添加的那个源），
	// 官方 `DiscoverController.selectAdded` 同一语义（market-store.ts:181-201）。
	selectAdded bool
	// sourcesSaving / sourceFailure / sourceReason 是写回索引源的状态。
	sourcesSaving bool
	sourceFailure apiclient.MarketSourceFailureKind
	sourceReason  string
}

// marketIndexesCache 是索引源清单 + 它的读取状态。
type marketIndexesCache struct {
	items       []apiclient.MarketIndexSummary
	loaded      bool
	unavailable bool
	errText     string
	pending     bool
}

// ── Actions 实现 ────────────────────────────────────────────────────────

// marketIndexes 是 `Actions.MarketIndexes`：同步返回缓存，首次问时触发一次拉取
// （与 pluginBundles 同一模式，插件每帧都会问）。
func (m *model) marketIndexes() ([]apiclient.MarketIndexSummary, bool, bool, string) {
	m.mu.Lock()
	cache := m.market.indexes
	start := !cache.loaded && !cache.pending && !cache.unavailable
	if start {
		m.market.indexes.pending = true
	}
	m.mu.Unlock()

	if start {
		go m.refreshMarketIndexes()
	}
	// 返回副本：缓存整体替换，不原地改元素。
	out := make([]apiclient.MarketIndexSummary, len(cache.items))
	copy(out, cache.items)
	return out, cache.loaded, cache.unavailable, cache.errText
}

// marketIndexesReload 是 `Actions.MarketIndexesReload`（失败后的重试）。
func (m *model) marketIndexesReload() {
	m.mu.Lock()
	if m.market.indexes.pending {
		m.mu.Unlock()
		return // 已经有一次在飞，别叠加
	}
	m.market.indexes.pending = true
	m.market.indexes.errText = ""
	m.mu.Unlock()
	go m.refreshMarketIndexes()
}

// refreshMarketIndexes 真的读一次索引源清单，并按官方的规则决定选中项。
func (m *model) refreshMarketIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), marketCallTimeout)
	defer cancel()
	items, err := m.cli.PluginMarketListIndexes(ctx)

	m.mu.Lock()
	m.market.indexes.pending = false
	if err != nil {
		// 宿主把"没有配置索引源"也回成 503 registry_unavailable（见文件头第 3 条），
		// 那是**空清单**，不是故障：官方在这个位置画"还没有插件索引"。
		if apiclient.PluginMarketNoIndexes(err) {
			m.market.indexes.items = nil
			m.market.indexes.loaded = true
			m.market.indexes.errText = ""
			m.market.indexes.unavailable = false
		} else {
			m.market.indexes.unavailable = apiclient.PluginMarketUnavailable(err)
			m.market.indexes.errText = err.Error()
		}
		m.mu.Unlock()
		m.inv.Invalidate()
		return
	}
	m.market.indexes.items = items
	m.market.indexes.loaded = true
	m.market.indexes.errText = ""
	m.market.indexes.unavailable = false
	previous := ""
	if m.market.listing != nil {
		previous = m.market.listing.URL
	}
	// 官方选中项规则（market-store.ts:295-311）：刚添加的源选中最后一个；
	// 否则原来的还在就保持；否则第一条；一个都没有 = 没有选中。
	selected := previous
	selectAdded := m.market.selectAdded
	m.market.selectAdded = false
	if selectAdded && len(items) > 0 {
		selected = items[len(items)-1].URL
	} else if !marketHasIndex(items, previous) {
		selected = ""
		if len(items) > 0 {
			selected = items[0].URL
		}
	}
	needsLoad := selected != previous && selected != ""
	if selected == "" {
		m.market.listing = nil
		m.market.seq++ // 丢掉在飞的读
	}
	m.mu.Unlock()
	m.inv.Invalidate()

	if needsLoad {
		m.marketLoad(selected, marketInitialQuery(), marketPageSize, false)
	}
}

// marketHasIndex 报告清单里有没有这个 URL。
func marketHasIndex(items []apiclient.MarketIndexSummary, url string) bool {
	if url == "" {
		return false
	}
	for _, item := range items {
		if item.URL == url {
			return true
		}
	}
	return false
}

// marketInitialQuery 是官方 `INITIAL_QUERY`（`OFF:presentation.ts:15`）。
func marketInitialQuery() apiclient.MarketQuery {
	return apiclient.MarketQuery{Sort: apiclient.MarketSort{Kind: apiclient.MarketSortIndex}}
}

// marketSelectIndex 是 `Actions.MarketSelectIndex`（官方 `select`）。
func (m *model) marketSelectIndex(url string) {
	m.mu.Lock()
	if m.market.listing != nil && m.market.listing.URL == url {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	m.marketLoad(url, marketInitialQuery(), marketPageSize, false)
}

// marketListingNow 是 `Actions.MarketListing`：同步返回壳里的列表状态。
func (m *model) marketListingNow() (pluginapi.MarketListing, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.market.listing == nil {
		return pluginapi.MarketListing{}, false
	}
	listing := *m.market.listing
	return listing, true
}

// marketSearch 是 `Actions.MarketSearch`：去抖后按新 query 重读第一页。
//
// text 只用来比较"这一下是不是又改文本了"：同一段文本连打两帧不会叠加请求，
// 换了文本就把上一次的定时器作废（官方 250ms 定时器语义）。
func (m *model) marketSearch(query apiclient.MarketQuery, _ string) {
	m.mu.Lock()
	m.market.searchGen++
	gen := m.market.searchGen
	url := ""
	if m.market.listing != nil {
		url = m.market.listing.URL
	}
	m.mu.Unlock()
	if url == "" {
		return
	}
	time.AfterFunc(marketSearchDebounce, func() {
		m.mu.Lock()
		stale := m.market.searchGen != gen
		m.mu.Unlock()
		if stale {
			return
		}
		m.marketLoad(url, query, marketPageSize, false)
	})
}

// marketMore 是 `Actions.MarketMore`（官方 `more`）。
func (m *model) marketMore() {
	m.mu.Lock()
	listing := m.market.listing
	if listing == nil || listing.Value == nil || listing.Answered == nil {
		m.mu.Unlock()
		return
	}
	// 只有"结果已经落定且回答的就是当前 query"才允许读下一页
	// （官方 `loadMore` 的前置判据，market-store.ts:365-368）。
	if listing.Status != pluginapi.MarketListingReady && listing.Status != pluginapi.MarketListingError {
		m.mu.Unlock()
		return
	}
	if !marketSameQuery(*listing.Answered, listing.Query) {
		m.mu.Unlock()
		return
	}
	if listing.Value.NextOffset == nil {
		m.mu.Unlock()
		return
	}
	url := listing.URL
	query := listing.Query
	value := listing.Value
	m.market.seq++
	seq := m.market.seq
	m.market.pending = true
	m.market.listing = &pluginapi.MarketListing{
		URL: url, Query: query, Answered: listing.Answered,
		Status: pluginapi.MarketListingMore, Value: value,
	}
	m.mu.Unlock()
	m.inv.Invalidate()

	go m.runMarketMore(seq, url, query, value)
}

// runMarketMore 读下一页并追加（官方 `loadMore`）。
func (m *model) runMarketMore(seq uint64, url string, query apiclient.MarketQuery, value *apiclient.MarketSearchResult) {
	page := query
	page.Offset = len(value.Entries)
	page.Limit = marketPageSize
	page.FetchedAt = value.FetchedAt

	ctx, cancel := context.WithTimeout(context.Background(), marketCallTimeout)
	defer cancel()
	answer, err := m.cli.PluginMarketSearch(ctx, url, page)
	if !m.marketSeqIs(seq) {
		return
	}
	if err != nil {
		m.settleMarketFailure(seq, url, query, value, m.marketAnswered(), apiclient.MarketFailureFetch, err.Error(), m.marketMore)
		return
	}
	// 文档换了 → 从头重读（官方 `load(url, query, 已加载+一页)`）。
	if answer.FetchedAt != value.FetchedAt {
		m.marketLoad(url, query, len(value.Entries)+marketPageSize, false)
		return
	}
	merged := answer
	merged.Entries = append(append([]apiclient.MarketEntry{}, value.Entries...), answer.Entries...)
	m.settleMarketReady(seq, url, query, &merged)
}

// marketRefresh 是 `Actions.MarketRefresh`（官方 `refresh`，带 refresh:true）。
func (m *model) marketRefresh() {
	m.mu.Lock()
	listing := m.market.listing
	if listing == nil {
		m.mu.Unlock()
		return
	}
	url, query := listing.URL, listing.Query
	count := marketPageSize
	if listing.Value != nil && len(listing.Value.Entries) > count {
		count = len(listing.Value.Entries)
	}
	m.mu.Unlock()
	m.marketLoad(url, query, count, true)
}

// marketRetry 是 `Actions.MarketRetry`：重跑上一次失败的那一次读。
func (m *model) marketRetry() {
	m.mu.Lock()
	retry := m.market.retry
	m.mu.Unlock()
	if retry != nil {
		retry()
	}
}

// marketSetIndexes 是 `Actions.MarketSetIndexes`（官方 `saveSources`）。
func (m *model) marketSetIndexes(urls []string) {
	adding := false
	m.mu.Lock()
	adding = len(urls) > len(m.market.indexes.items)
	m.market.selectAdded = adding
	m.market.sourcesSaving = true
	m.market.sourceFailure = ""
	m.market.sourceReason = ""
	m.mu.Unlock()
	m.inv.Invalidate()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), marketCallTimeout)
		defer cancel()
		err := m.cli.PluginMarketSetIndexes(ctx, urls)

		m.mu.Lock()
		m.market.sourcesSaving = false
		if err != nil {
			kind, reason := apiclient.MarketSourceFailureOf(err)
			m.market.sourceFailure = kind
			m.market.sourceReason = reason
			m.mu.Unlock()
			m.inv.Invalidate()
			return
		}
		m.market.sourceFailure = ""
		m.market.sourceReason = ""
		m.mu.Unlock()
		m.inv.Invalidate()
		// 官方写完一律重读索引源清单（`await this.loadIndexes()`）：选中项按
		// selectAdded 规则落在新加的那个源上（见 marketState.selectAdded）。
		m.refreshMarketIndexes()
	}()
}

// marketSourcesState 是 `Actions.MarketSourcesState`。
func (m *model) marketSourcesState() (bool, apiclient.MarketSourceFailureKind, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.market.sourcesSaving, m.market.sourceFailure, m.market.sourceReason
}

// ── 列表读的核心（官方 `load` 的移植）────────────────────────────────────

// marketLoad 开始一次第一页读（新 query / 换索引 / 刷新都走它）。
func (m *model) marketLoad(url string, query apiclient.MarketQuery, count int, refresh bool) {
	if url == "" {
		return
	}
	m.mu.Lock()
	m.market.seq++
	seq := m.market.seq
	m.market.pending = true
	kept := (*apiclient.MarketSearchResult)(nil)
	var answered *apiclient.MarketQuery
	if current := m.market.listing; current != nil && current.URL == url {
		kept = current.Value
		answered = current.Answered
	}
	m.market.listing = &pluginapi.MarketListing{
		URL: url, Query: query, Answered: answered,
		Status: pluginapi.MarketListingLoading, Value: kept,
	}
	m.mu.Unlock()
	m.inv.Invalidate()

	go m.runMarketLoad(seq, url, query, count, refresh, kept, answered)
}

// runMarketLoad 是官方 `load` 的循环：按宿主接受的上限分页，拼到 count 条为止；
// 拼页时 `fetchedAt` 必须相同，变了就整批重来，**再一次变**才报 `changed`。
func (m *model) runMarketLoad(seq uint64, url string, query apiclient.MarketQuery, count int, refresh bool,
	kept *apiclient.MarketSearchResult, answered *apiclient.MarketQuery) {
	if count < 1 {
		count = marketPageSize
	}
	var value *apiclient.MarketSearchResult
	restarted := false
	for {
		if !m.marketSeqIs(seq) {
			return
		}
		if value != nil && (len(value.Entries) >= count || value.NextOffset == nil) {
			break
		}
		offset := 0
		if value != nil {
			offset = len(value.Entries)
		}
		limit := count - offset
		if limit > marketMaxPage {
			limit = marketMaxPage
		}
		page := query
		page.Offset = offset
		page.Limit = limit
		if value != nil {
			// 后续页钉住第一页的文档（官方同一判据：fetchedAt 参与 query）。
			page.FetchedAt = value.FetchedAt
		}
		// 只有整批重来的第一页才带 refresh（官方 `refresh && value === null && !restarted`）。
		page.Refresh = refresh && value == nil && !restarted

		ctx, cancel := context.WithTimeout(context.Background(), marketCallTimeout)
		answer, err := m.cli.PluginMarketSearch(ctx, url, page)
		cancel()
		if !m.marketSeqIs(seq) {
			return
		}
		if err != nil {
			m.settleMarketFailure(seq, url, query, kept, answered, apiclient.MarketListingFailureOf(err), err.Error(), func() {
				m.marketLoad(url, query, count, refresh)
			})
			return
		}
		switch {
		case value == nil:
			value = &answer
		case answer.FetchedAt == value.FetchedAt:
			merged := answer
			merged.Entries = append(append([]apiclient.MarketEntry{}, value.Entries...), answer.Entries...)
			value = &merged
		case restarted:
			// 拼页期间文档换了两次：官方报 `readChanged`，不再自己重来（无限循环更糟）。
			m.settleMarketFailure(seq, url, query, kept, answered, apiclient.MarketFailureChanged, "", func() {
				m.marketLoad(url, query, count, refresh)
			})
			return
		default:
			restarted = true
			value = nil
		}
	}

	// 索引不再声明的 id 从 query 里剔掉，然后按剔过的 query 重读一次
	// （官方 `declaredQuery`，market-store.ts:326-330）。
	if value != nil {
		if declared, changed := marketDeclaredQuery(query, value.Metadata); changed {
			m.marketLoad(url, declared, count, false)
			return
		}
	}
	m.settleMarketReady(seq, url, query, value)
}

// settleMarketReady 发布一次成功的列表读，并把索引行的事实（标题/说明）回写。
func (m *model) settleMarketReady(seq uint64, url string, query apiclient.MarketQuery, value *apiclient.MarketSearchResult) {
	m.mu.Lock()
	if m.market.seq != seq {
		m.mu.Unlock()
		return
	}
	answered := query
	m.market.pending = false
	m.market.retry = nil
	m.market.listing = &pluginapi.MarketListing{
		URL: url, Query: query, Answered: &answered,
		Status: pluginapi.MarketListingReady, Value: value,
	}
	if value != nil {
		for i, item := range m.market.indexes.items {
			if item.URL != url {
				continue
			}
			m.market.indexes.items[i].Title = value.Title
			m.market.indexes.items[i].Description = value.Description
			m.market.indexes.items[i].Error = ""
		}
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}

// settleMarketFailure 发布一次失败的列表读：旧结果**保留**，只记失败档与重试入口。
func (m *model) settleMarketFailure(seq uint64, url string, query apiclient.MarketQuery, kept *apiclient.MarketSearchResult,
	answeredIn *apiclient.MarketQuery, kind apiclient.MarketFailureKind, errText string, retry func()) {
	m.mu.Lock()
	if m.market.seq != seq {
		m.mu.Unlock()
		return
	}
	answered := answeredIn
	m.market.pending = false
	m.market.retry = retry
	m.market.listing = &pluginapi.MarketListing{
		URL: url, Query: query, Answered: answered,
		Status: pluginapi.MarketListingError, Value: kept, Failure: kind,
	}
	for i, item := range m.market.indexes.items {
		if item.URL == url {
			m.market.indexes.items[i].Error = errText
		}
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}

// marketSeqIs 报告 seq 是否仍是当前代号（迟到的回答按它丢弃）。
func (m *model) marketSeqIs(seq uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.market.seq == seq
}

// marketAnswered 返回当前列表已落定结果所回答的那个 query（没有结果就是 nil）。
func (m *model) marketAnswered() *apiclient.MarketQuery {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.market.listing == nil {
		return nil
	}
	return m.market.listing.Answered
}

// marketSameQuery 比较两个 query 是否逐字段相同（Labels 是切片，不能直接用 ==）。
func marketSameQuery(a, b apiclient.MarketQuery) bool {
	if a.Text != b.Text || a.Category != b.Category || a.Sort != b.Sort ||
		a.Offset != b.Offset || a.Limit != b.Limit || a.Refresh != b.Refresh || a.FetchedAt != b.FetchedAt {
		return false
	}
	if len(a.Labels) != len(b.Labels) {
		return false
	}
	for i := range a.Labels {
		if a.Labels[i] != b.Labels[i] {
			return false
		}
	}
	return true
}

// marketDeclaredQuery 是官方 `declaredQuery`（`OFF:presentation.ts:36-46`）：
// 索引不再声明的分类/标签/排序键从 query 里剔掉，让工具栏上每一个生效的选项
// 都能真的在结果里看见。
func marketDeclaredQuery(query apiclient.MarketQuery, metadata apiclient.MarketIndexMetadata) (apiclient.MarketQuery, bool) {
	out := query
	changed := false
	if out.Category != "" {
		if _, ok := metadata.Categories[out.Category]; !ok {
			out.Category = ""
			changed = true
		}
	}
	if len(out.Labels) > 0 {
		kept := make([]string, 0, len(out.Labels))
		for _, label := range out.Labels {
			if _, ok := metadata.Labels[label]; ok {
				kept = append(kept, label)
			}
		}
		if len(kept) != len(out.Labels) {
			out.Labels = kept
			changed = true
		}
	}
	if out.Sort.Kind == apiclient.MarketSortKey {
		if _, ok := metadata.SortKeys[out.Sort.ID]; !ok {
			out.Sort = apiclient.MarketSort{Kind: apiclient.MarketSortIndex}
			changed = true
		}
	}
	return out, changed
}
