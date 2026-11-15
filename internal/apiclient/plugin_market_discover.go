// plugin_market_discover.go —— 官方 `dsh-client-ui-plugin-market`（`#30`）Discover tab
// 消费的三个宿主方法的客户端：`listIndexes` / `search` / `setIndexes`。
//
// # 官方契约（唯一权威）
//
//	OFF:../dsh-client-ui-plugin-market/src/client/market-store.ts:295
//	     listIndexes() → MarketIndexSummary[]（无参数）
//	OFF:../dsh-client-ui-plugin-market/src/client/market-store.ts:381
//	     search(url, query{text?,category?,labels?,sort,offset,limit,refresh?,fetchedAt?})
//	       → MarketSearchResult
//	OFF:../dsh-client-ui-plugin-market/src/client/market-store.ts:405
//	     setIndexes(urls: string[]) → void
//	OFF:../dsh-client-ui-plugin-market/src/client/market-store.ts:140-152
//	     三个失败投影：sourceFailure / listingFailure / detailFailure
//	OFF:../dsh-plugin-market/lib/typert.host.js:9-18,62-170
//	     上面每个结构的逐字段 schema
//
// # 宿主证据（2026-11-15 起是真实现，不再是 501）
//
//	HOST:internal/transport/handlers_plugin_market_official.go:18-30
//	     结果结构体与官方同集合（search 结果见 :217-242：url/title/description/
//	     metadata/fetchedAt/entries/total/nextOffset，**没有**旧的自造 index/plugins/offset）
//	HOST:internal/transport/handlers_plugin_market_official.go:337-392
//	     listIndexes 无参数、search 读平铺的 `url` + `query`、setIndexes 读 `urls`
//	HOST:internal/transport/handlers_plugin_market_official.go:722-731
//	     参数不合契约是 400 `bad_request`；索引取不到是 502 `index_unavailable`
//	HOST:internal/market/store.go:1-30
//	     索引文档只认官方 format 1（`metadata.categories/labels/sortKeys` 三张声明表），
//	     这是筛选工具栏（分类菜单/标签过滤/排序键）的数据来源
//
// # 为什么这里不做"猜默认值"
//
// 官方 query 的 `sort`/`offset`/`limit` 是**必填**，`limit` 只能在 1..100。宿主对
// 未知字段/越界值一律 400，所以本客户端只发它真会发的键：`text`/`category`/`labels`
// 为空就整键不发（与官方 `market-store.ts:395-403` 的展开写法逐条对应）。
package apiclient

import (
	"context"
	"errors"
	"strings"
)

// ── 官方数据结构（@deepseek-ai/dsh-plugin-market 的 typert schema）──────────

// MarketDeclaration 是索引声明的一类 id（分类或标签）：id → 显示标题
// （OFF `MarketDeclaration`，typert.host.js:20-26）。
//
// 标题是官方 `string | localized` union，复用本仓的 PluginLocalizedText。
type MarketDeclaration struct {
	Title PluginLocalizedText `json:"title"`
}

// MarketSortKeyDeclaration 是索引声明的排序键：显示标题 + 初始方向
// （OFF `MarketSortKeyDeclaration`）。
type MarketSortKeyDeclaration struct {
	Title PluginLocalizedText `json:"title"`
	Order string              `json:"order"`
}

// MarketIndexMetadata 是索引的**三张声明表**（OFF `MarketIndexMetadata`）。
//
// 三组总是下发（缺省为空表）；界面靠它画分类菜单 / 标签过滤 / 排序键，
// 没有它就只能画搜索框（`#30` B 块此前卡的就是这一条）。
type MarketIndexMetadata struct {
	Categories map[string]MarketDeclaration        `json:"categories"`
	Labels     map[string]MarketDeclaration        `json:"labels"`
	SortKeys   map[string]MarketSortKeyDeclaration `json:"sortKeys"`
}

// MarketIndexSummary 是 `listIndexes` 的一项（OFF `MarketIndexSummary`）。
//
// title/description 是 union；error 非空 = 最近一次读这个索引失败（官方客户端
// 把失败原因回写进索引行，`market-store.ts:slide` 的 settle）。
type MarketIndexSummary struct {
	URL         string              `json:"url"`
	Title       PluginLocalizedText `json:"title"`
	Description PluginLocalizedText `json:"description,omitempty"`
	Error       string              `json:"error,omitempty"`
}

// MarketRepository 是插件源码仓库的展示信息（OFF `MarketRepository`）。
type MarketRepository struct {
	URL       string `json:"url"`
	Directory string `json:"directory,omitempty"`
}

// MarketPluginMetadata 是插件自己的市场元数据（OFF `MarketPluginMetadata`）。
type MarketPluginMetadata struct {
	Category string             `json:"category,omitempty"`
	Keywords []string           `json:"keywords"`
	Labels   []string           `json:"labels"`
	SortKeys map[string]float64 `json:"sortKeys"`
}

// MarketInstall 是"装这一确切版本"的参数（OFF `MarketInstallTarget`）。
type MarketInstall struct {
	Spec     string `json:"spec"`
	Registry string `json:"registry,omitempty"`
}

// MarketEngines 是这一版的运行时要求（OFF `MarketEngines`）。
type MarketEngines struct {
	DSH string `json:"dsh,omitempty"`
}

// 官方 `MarketListingVersion.source.type` 的两个分支。
const (
	MarketSourceNPM = "npm"
	MarketSourceGit = "git"
)

// MarketSource 是版本来源的官方 union（npm | git）。
//
// 判别键是 Type：npm 分支只有 Registry，git 分支只有 URL/Commit/Path
// （宿主按分支只下发对应字段，见 HOST:handlers_plugin_market_official.go 的
// OfficialMarketSource.MarshalJSON）。
type MarketSource struct {
	Type     string `json:"type"`
	Registry string `json:"registry,omitempty"`
	URL      string `json:"url,omitempty"`
	Commit   string `json:"commit,omitempty"`
	Path     string `json:"path,omitempty"`
}

// MarketCompatibility 是官方兼容性 union（OFF `MarketCompatibility`）。
//
// 兼容时只有 Compatible=true；不兼容时带运行时版本、未满足的要求与豁免标记。
type MarketCompatibility struct {
	Compatible     bool              `json:"compatible"`
	RuntimeVersion string            `json:"runtimeVersion,omitempty"`
	Requirements   map[string]string `json:"requirements,omitempty"`
	Exempted       bool              `json:"exempted,omitempty"`
}

// MarketListingVersion 是一行版本（OFF `MarketListingVersion`）。
//
// PublishTimestamp 是 Unix **秒**（官方 dateStyle 格式化就按秒 ×1000 用，
// OFF:DiscoverSection.tsx:90-92）。
type MarketListingVersion struct {
	Version          string              `json:"version"`
	PublishTimestamp int64               `json:"publishTimestamp"`
	Source           MarketSource        `json:"source"`
	Install          MarketInstall       `json:"install"`
	Engines          MarketEngines       `json:"engines"`
	Compatibility    MarketCompatibility `json:"compatibility"`
	Update           bool                `json:"update"`
}

// 官方 `MarketInstallation.status` 的五个取值（`provided` 宿主不下发）。
const (
	MarketInstallStatusNotInstalled     = "not-installed"
	MarketInstallStatusInstalled        = "installed"
	MarketInstallStatusDifferentVersion = "different-version"
	MarketInstallStatusUpdateAvailable  = "update-available"
	MarketInstallStatusProvided         = "provided"
)

// MarketInstallation 是"这一包在当前 profile 里的状态"（OFF `MarketInstallation`）。
type MarketInstallation struct {
	Status           string `json:"status"`
	InstalledVersion string `json:"installedVersion,omitempty"`
}

// MarketEntry 是搜索结果的一条（OFF `MarketListingEntry`）。
//
// 所有字段都按官方同名下发；界面据此画卡片与详情页，**不另算一遍**。
type MarketEntry struct {
	Name           string                 `json:"name"`
	Title          PluginLocalizedText    `json:"title,omitempty"`
	Description    PluginLocalizedText    `json:"description,omitempty"`
	Icon           string                 `json:"icon,omitempty"`
	Author         string                 `json:"author,omitempty"`
	Homepage       string                 `json:"homepage,omitempty"`
	Repository     *MarketRepository      `json:"repository,omitempty"`
	Metadata       MarketPluginMetadata   `json:"metadata"`
	Versions       []MarketListingVersion `json:"versions"`
	DefaultVersion string                 `json:"defaultVersion,omitempty"`
	Installation   MarketInstallation     `json:"installation"`
}

// MarketSearchResult 是 `pluginMarket.search` 的结果（OFF `MarketSearchResult`）。
//
// NextOffset 用指针：**最后一页是显式 null**，缺席与 null 在客户端语义不同
// （HOST:handlers_plugin_market_official.go:230-236 的注释写着同一句话）。
type MarketSearchResult struct {
	URL         string              `json:"url"`
	Title       PluginLocalizedText `json:"title"`
	Description PluginLocalizedText `json:"description,omitempty"`
	Metadata    MarketIndexMetadata `json:"metadata"`
	FetchedAt   int64               `json:"fetchedAt"`
	Entries     []MarketEntry       `json:"entries"`
	Total       int                 `json:"total"`
	NextOffset  *int                `json:"nextOffset"`
}

// 官方 `MarketSort.kind` 的四个取值（OFF `MarketSort` union）。
const (
	MarketSortIndex     = "index"
	MarketSortRelevance = "relevance"
	MarketSortPublished = "published"
	MarketSortKey       = "key"
)

// MarketSort 是官方 `MarketSort` union 的 Go 侧形状。
//
// `index`/`relevance` 只有 Kind；`published`/`key` 还必须有 Direction（`asc`|`desc`），
// `key` 另加 ID。宿主按这张表校验，缺字段直接 400
// （HOST:handlers_plugin_market_official.go 的 marketSort）。
type MarketSort struct {
	Kind      string
	ID        string
	Direction string
}

// MarketQuery 是 `pluginMarket.search` 的 query（官方 `MarketQuery`）。
//
// Limit 必须是 1..100（宿主 400 越界）；Offset 非负。
type MarketQuery struct {
	Text      string
	Category  string
	Labels    []string
	Sort      MarketSort
	Offset    int
	Limit     int
	Refresh   bool
	FetchedAt int64
}

// wire 把 query 编成宿主认的平铺对象。
//
// 与官方 `market-store.ts:395-403` 逐键对应：空 text / 空 category / 空 labels
// **整键不发**（官方是对象展开，空对象/空数组根本不出现）。
func (q MarketQuery) wire() map[string]any {
	query := map[string]any{
		"sort":   q.Sort.wire(),
		"offset": q.Offset,
		"limit":  q.Limit,
	}
	if q.Text != "" {
		query["text"] = q.Text
	}
	if q.Category != "" {
		query["category"] = q.Category
	}
	if len(q.Labels) > 0 {
		query["labels"] = q.Labels
	}
	if q.Refresh {
		query["refresh"] = true
	}
	if q.FetchedAt != 0 {
		query["fetchedAt"] = q.FetchedAt
	}
	return query
}

// wire 把 sort union 编成宿主认的对象。
func (s MarketSort) wire() map[string]any {
	out := map[string]any{"kind": s.Kind}
	if s.ID != "" {
		out["id"] = s.ID
	}
	if s.Direction != "" {
		out["direction"] = s.Direction
	}
	return out
}

// ── 三个宿主方法 ────────────────────────────────────────────────────────

// PluginMarketListIndexes 列出已配置的索引源（官方 `pluginMarket.listIndexes`，**无参数**）。
//
// 空数组是**合法回答**（"一个索引源都没配"），官方客户端据此画 `emptyTitle` +
// 「添加索引源」；不是错误。
func (c *Client) PluginMarketListIndexes(ctx context.Context) ([]MarketIndexSummary, error) {
	var out []MarketIndexSummary
	if err := c.Call(ctx, "pluginMarket.listIndexes", map[string]any{}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PluginMarketSearch 搜一个索引源的一页（官方 `pluginMarket.search(url, query)`）。
func (c *Client) PluginMarketSearch(ctx context.Context, url string, query MarketQuery) (MarketSearchResult, error) {
	params := map[string]any{"url": url, "query": query.wire()}
	var out MarketSearchResult
	if err := c.Call(ctx, "pluginMarket.search", params, &out); err != nil {
		return MarketSearchResult{}, err
	}
	return out, nil
}

// PluginMarketSetIndexes 写回索引源列表（官方 `pluginMarket.setIndexes(urls)`）。
//
// 官方结果类型是 void，宿主用 `{}` 表达"这次调用没有载荷"
// （HOST:handlers_plugin_market_official.go:366-369），所以这里不取结果。
func (c *Client) PluginMarketSetIndexes(ctx context.Context, urls []string) error {
	if urls == nil {
		urls = []string{}
	}
	return c.Call(ctx, "pluginMarket.setIndexes", map[string]any{"urls": urls}, nil)
}

// ── 失败投影（判据与官方客户端逐条一致）─────────────────────────────────

// MarketFailureKind 是列表读失败的官方三档（OFF `ListingFailure`）。
//
// 本仓宿主**产不出** `invalid`：索引文档非法时 store 回 `index_unavailable`
// （HOST:cmd/dshd/serve_plugin_market.go:146-149），与"取不到"同一码，所以界面
// 按 `fetch` 画「无法读取此索引」。这条差值登记在证据文档里，不在这里假装能分。
type MarketFailureKind string

const (
	// MarketFailureFetch 是"这个索引读不到"（网络/404/文档非法都落这里）。
	MarketFailureFetch MarketFailureKind = "fetch"
	// MarketFailureChanged 是"索引在多页加载过程中换了一份文档"（fetchedAt 变了两次）。
	MarketFailureChanged MarketFailureKind = "changed"
)

// MarketListingFailureOf 把一次搜索结果读的错误投影成官方档。
func MarketListingFailureOf(err error) MarketFailureKind {
	if err == nil {
		return ""
	}
	return MarketFailureFetch
}

// MarketSourceFailureKind 是写回索引源失败的官方四档（OFF `SourceFailure`）。
type MarketSourceFailureKind string

const (
	// MarketSourceInvalid：URL 不是 http(s) 绝对地址。
	MarketSourceInvalid MarketSourceFailureKind = "invalid"
	// MarketSourceDuplicate：这个源已经加过了（宿主会静默去重，**到不了这里**）。
	MarketSourceDuplicate MarketSourceFailureKind = "duplicate"
	// MarketSourceUnavailable：当前环境无法保存索引源（宿主没有这个码，到不了这里）。
	MarketSourceUnavailable MarketSourceFailureKind = "unavailable"
	// MarketSourceOther：其余一切（宿主原文进 reason）。
	MarketSourceOther MarketSourceFailureKind = "other"
)

// MarketSourceFailureOf 把一次 setIndexes 的错误投影成官方四档。
//
// 判据对齐官方 `market-store.ts:140-145`：只有 `plugin-market/invalid-index-url`
// 才细分 invalid/duplicate；本仓宿主用 `bad_request` 表达非法 URL
// （HOST:cmd/dshd/serve_plugin_market.go:112-121 的注释写明这是"参数问题"），
// 所以 `bad_request` → invalid，其余一律 other 并带上宿主原文。
func MarketSourceFailureOf(err error) (MarketSourceFailureKind, string) {
	if err == nil {
		return "", ""
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr == nil {
		return MarketSourceOther, err.Error()
	}
	if apiErr.Code == "bad_request" {
		return MarketSourceInvalid, apiErr.Message
	}
	if strings.TrimSpace(apiErr.Message) != "" {
		return MarketSourceOther, apiErr.Message
	}
	return MarketSourceOther, apiErr.Code
}

// PluginMarketNoIndexes 报告一次 listIndexes 失败是不是"一个索引源都没配置"。
//
// 官方 `pluginMarket.listIndexes` 在"没有配置"时回**空数组**（界面据此画
// `emptyTitle` + 「添加索引源」）。本仓宿主走的是另一条路：`Store.List` 回
// `ErrNoIndexes`，serve 层翻成 503 `registry_unavailable`
// （HOST:cmd/dshd/serve_plugin_market.go:94-99）。
//
// 与"宿主根本没接线"区分开：未接线时 transport 回的是同一个码、但消息里写着
// `未接线`（HOST:internal/transport/handlers_plugin_market_official.go:329-334）。
// 所以这里两者都看，缺一不可：
//
//	registry_unavailable + 消息不含「未接线」= 没有配置索引源（空清单，能添加）
//	registry_unavailable + 消息含「未接线」  = 没有这个能力（画 unavailable）
//
// 这是登记过的**宿主形状差值**（GUI 证据文档 §缺口，要宿主线改成回空数组）。
func PluginMarketNoIndexes(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr == nil {
		return false
	}
	if apiErr.Code != "registry_unavailable" {
		return false
	}
	return !strings.Contains(apiErr.Message, "未接线")
}

// PluginMarketUnavailable 报告一次市场调用是不是"宿主没有这个能力"。
//
// 两种形态都要认：
//
//	not_implemented                  —— 旧宿主/未接线的结构化 501
//	registry_unavailable + 未接线     —— transport 在没 Attach 时回的 503
//	                                   （HOST:handlers_plugin_market_official.go:329-334）
func PluginMarketUnavailable(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr == nil {
		return false
	}
	if apiErr.Code == "not_implemented" {
		return true
	}
	return apiErr.Code == "registry_unavailable" && strings.Contains(apiErr.Message, "未接线")
}

// Pick 取索引入口要显示的标题：市场标题 → 浏览器能解析时的 host → 原样 URL
// （官方 `indexLabel`，OFF:presentation.ts:106-115）。
func (i MarketIndexSummary) PickLabel() string {
	if title := i.Title.Pick(); title != "" {
		return title
	}
	if host := urlHost(i.URL); host != "" {
		return host
	}
	return i.URL
}

// urlHost 抽出一个绝对 URL 的 host（官方用 `URL.canParse` + `new URL(url).host`）。
//
// 不用 net/url 的 Parse 判据：官方 `URL.canParse` 对带用户名/缺 scheme 的串有
// 自己的规则，而这里只是**显示兜底**——解析不了就原样显示 URL，不影响任何请求。
func urlHost(raw string) string {
	scheme := strings.Index(raw, "://")
	if scheme <= 0 {
		return ""
	}
	rest := raw[scheme+3:]
	if end := strings.IndexAny(rest, "/?#"); end >= 0 {
		rest = rest[:end]
	}
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		rest = rest[at+1:]
	}
	return rest
}
