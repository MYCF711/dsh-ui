// settings_items_web.go —— 官方 #47 `ui-settings-web-search`（「网页搜索与获取」设置页）
// 在本仓的**数据模型**。
//
// # 官方结构（为什么它和另外三条插件设置页不同形）
//
// 官方这一页也挂在「插件」页的 `plugins.item` 槽（`OFF:src/client/index.ts:72-74`
// 的 `whileServed([WEB_NS]) → {id:'web-search', order:20}`），但它的正文不是一串
// `SettingsValueField`：
//
//	OFF:WebSearchCard.tsx:68-129   两个 section，各一个 `.selector` 下拉 + 说明，
//	                               下面是 `.panel` 灰面板（选中 provider 自己的设置）
//	OFF:WebSearchCard.tsx:79-90    DeepSeek Search API：count + billing
//	OFF:WebSearchCard.tsx:91-114   DeepSeek（旧版）：密钥 + Base URL + 最大搜索次数
//	OFF:WebSearchCard.tsx:122-127  DeepSeek Fetch API：只有 billing
//
// 所以它在本仓有自己的状态类型（`settingsItemWebState`），与 `settingsItemSectionState`
// 并列而不是硬套。
//
// # 三条命名空间（宿主 `internal/settingsform/web_namespaces_schema.go` 逐字对齐）
//
//	web                    searchProvider / fetchProvider（官方**无默认值**）
//	web-deepseek-api       count(1..50) / searchBilling / fetchBilling /
//	                       searchBaseURL / searchApiKeyEnv / fetchBaseURL / fetchApiKeyEnv …
//	web-search-deepseek    apiKey(**secret**) / apiKeyEnv / baseURL / model / apiVersion /
//	                       maxTokens / maxUses
//
// 本页只编辑官方那 9 个：`count` / 两个 billing / 两个 provider 选择 / legacy 的
// `baseURL` / `maxUses` / `apiKey`。`model` / `apiVersion` / `maxTokens` /
// `searchRequestsPerSecond` / `sendOriginalQuestion` / 两个 API 端点字段在官方
// JSX 里**没有控件**（端点与凭据引用是 YAML-only，只读，用来报告"它覆盖了计费"）。
// 少画它们不是缺块，是官方形状本身如此。
//
// # 状态机（与官方 `SettingsFormModel` 同一套，逐条对照编译产物）
//
// 编译产物（`dsh-web-frontend/dist/assets/index-ghOpC9uV.js` 的 `BZ` 类）里
// `plan()` / `save()` / `field()` 的判据被本文件与 settings_items.go 一起实现：
//
//	field(name)  : secretSpec → {text: 草稿 ?? '', overridden:false, invalid:false}
//	               无草稿 → {text: format(sectionValue), overridden: stored}
//	               有草稿 → {text: 草稿, overridden: parse.kind==='set', invalid: parse===undefined}
//	plan()       : secret 草稿文本 trim 非空 → {run: 写凭据}
//	               clear → stored 才发 unset；文本 === format(current) → 跳过
//	               parse 失败 → 记 invalid（不发）；空白 → unset；否则 set
//	save()       : 官方**先 api 再 legacy，最后 selection**（保证选择生效时它依赖的设置
//	               已经落库），任一节被拒就停；密钥写属于 selection 那一节，在其
//	               mutate 之后跑（`savingRef = draftRef`）。
//
// 一个官方怪处照搬并登记：`save()` 的前置判据含 `!scope.writable`，而密钥草稿挂在
// **selection（web）** 这个 form 上——所以 `web` 只读时密钥也存不下去，哪怕凭据域
// 本身可写。界面注释（OFF:WebSearchCard.tsx:95-101）说的"设置只读不影响密钥"只对
// **禁用态**成立，不对保存路径成立。本仓照官方行为，不擅自"修好"。
package dssidebar

import (
	"encoding/json"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// ── 官方命名空间与字段（逐字，锚点是 rc.2 随包 src）────────────────────

const (
	// settingsNSWeb 是 provider 选择的命名空间（OFF:web-search-card-controller.ts:48）。
	settingsNSWeb = "web"
	// settingsNSWebAPI 是 Search/Fetch API 的命名空间（同文件 :46）。
	settingsNSWebAPI = "web-deepseek-api"
	// settingsNSWebSearch 是旧 Messages 搜索的命名空间（同文件 :44）。
	settingsNSWebSearch = "web-search-deepseek"

	// settingsFieldSearchProvider / FetchProvider 是两个 provider 选择（:290）。
	settingsFieldSearchProvider = "searchProvider"
	settingsFieldFetchProvider  = "fetchProvider"

	// settingsFieldWebCount 是每次搜索结果数（:294）。
	settingsFieldWebCount = "count"
	// settingsFieldSearchBilling / FetchBilling 是两处能力的计费方式（:302）。
	settingsFieldSearchBilling = "searchBilling"
	settingsFieldFetchBilling  = "fetchBilling"
	// settingsFieldSearchBaseURL / SearchKeyEnv 与 Fetch 版是**只读**的 advanced override
	// （:147-148 的 `advancedOverride` 判据；官方页面从不写它们）。
	settingsFieldSearchBaseURL = "searchBaseURL"
	settingsFieldSearchKeyEnv  = "searchApiKeyEnv"
	settingsFieldFetchBaseURL  = "fetchBaseURL"
	settingsFieldFetchKeyEnv   = "fetchApiKeyEnv"

	// settingsFieldLegacyBaseURL / MaxUses 是旧版搜索页的两个可编辑字段（:305-310）。
	settingsFieldLegacyBaseURL = "baseURL"
	settingsFieldLegacyMaxUses = "maxUses"
	// settingsFieldLegacyKeyEnv 是密钥引用名（默认 DEEPSEEK_API_KEY）。
	settingsFieldLegacyKeyEnv = "apiKeyEnv"
	// settingsFieldLegacySecret 是官方**表单内部的**密钥字段名：它既不是
	// `web-search-deepseek.apiKey` 的写路径，也不是任何一个 settings 字段——
	// 它是 selection 表单里的一个 `secretSpec`（`API_KEY_FIELD='legacyApiKey'`，:71）。
	settingsFieldLegacySecret = "legacyApiKey"
)

// ── provider id 与菜单顺序（逐字，OFF:web-search-card-controller.ts:51-65）──

const (
	settingsWebProviderAPISearch    = "deepseek-search-api"
	settingsWebProviderLegacySearch = "deepseek-official"
	settingsWebProviderLocalFetch   = "http"
	settingsWebProviderAPIFetch     = "deepseek-fetch-api"
)

// settingsWebSearchMenuOrder / FetchMenuOrder 是官方菜单里**排在最前**的四个 id
// （`MENU_ORDER`，OFF:web-search-card-controller.ts:62-65）：官方 provider 先按这个
// 固定顺序，其余按宿主注册顺序跟在后面。
var (
	settingsWebSearchMenuOrder = []string{settingsWebProviderAPISearch, settingsWebProviderLegacySearch}
	settingsWebFetchMenuOrder  = []string{settingsWebProviderLocalFetch, settingsWebProviderAPIFetch}
)

// settingsWebTitles 是官方四种 provider 的显示名（locales.ts:73/74/79/80）。
var settingsWebTitles = map[string]string{
	settingsWebProviderAPISearch:    zhWebProviderDeepSeek,
	settingsWebProviderLegacySearch: zhWebProviderLegacy,
	settingsWebProviderLocalFetch:   zhWebProviderLocal,
	settingsWebProviderAPIFetch:     zhWebProviderDeepSeekFetch,
}

// 官方默认与界（web_namespaces_schema.go 逐值对应）。
const (
	// settingsWebDefaultAPIKeyRef 是密钥控件的默认引用（controller:68）。
	settingsWebDefaultAPIKeyRef = "DEEPSEEK_API_KEY"
	// settingsWebDefaultBilling 是计费的官方默认（index.ts:102-103 的 .default('auto')）。
	settingsWebDefaultBilling = "auto"
	// settingsWebCountMin / Max 是官方 count 的 parse 界（controller:296-299）。
	settingsWebCountMin = 1
	settingsWebCountMax = 50
	// settingsWebMaxUsesMin 是官方 maxUses 的下界（index.ts:78 的 .min(1)）。
	settingsWebMaxUsesMin = 1
	// settingsWebMessagesPath 是官方 legacy Base URL 说明里的 `{path}` 实参
	// （OFF:WebSearchCard.tsx:105 的 `baseUrlHint({path:'/messages'})`）。
	settingsWebMessagesPath = "/messages"
)

// settingsWebBillings 是官方计费的三个取值，**按菜单顺序**（WebSearchCard.tsx:196）。
var settingsWebBillings = []string{"auto", "account", "api-key"}

// 页内 section 的"选中 provider 显示什么"（官方判别联合，controller:147-149）。
const (
	settingsWebKindNone     = "none"
	settingsWebKindDeepSeek = "deepseek"
	settingsWebKindLegacy   = "legacy"
	settingsWebKindPlugin   = "plugin"
	settingsWebKindLocal    = "local"
)

// 官方写死的 DOM id（`plugin-config-…`）；本仓当稳定交互 id 用。
const (
	settingsWebSearchSectionDOMID  = "plugin-config-web-search-section"
	settingsWebSearchProviderDOMID = "plugin-config-web-search-provider"
	settingsWebCountDOMID          = "plugin-config-web-search-count"
	settingsWebKeyDOMID            = "plugin-config-web-search-key"
	settingsWebEndpointDOMID       = "plugin-config-web-search-endpoint"
	settingsWebMaxUsesDOMID        = "plugin-config-web-search-max-uses"
	settingsWebFetchSectionDOMID   = "plugin-config-web-fetch-section"
	settingsWebFetchProviderDOMID  = "plugin-config-web-fetch-provider"
)

// ── 官方 zh 文案（逐字，勿改；OFF:ui-settings-web-search/src/client/locales.ts）──

const (
	zhWebTitle       = "网页搜索与获取"            // 'title' :69
	zhWebSummary     = "选择网页搜索与网页获取使用的提供方。" // 'description' :70
	zhWebSearchTitle = "网页搜索提供方"            // 'searchSection' :71
	zhWebFetchTitle  = "网页获取提供方"            // 'fetchSection' :72

	zhWebProviderDeepSeek      = "DeepSeek Search API" // 'providerDeepSeek' :73
	zhWebProviderLegacy        = "DeepSeek（旧版）"        // 'providerLegacy' :74
	zhWebProviderLocal         = "本机直接获取"              // 'providerLocal' :79
	zhWebProviderDeepSeekFetch = "DeepSeek Fetch API"  // 'providerDeepSeekFetch' :80
	zhWebProviderAuto          = "自动选择"                // 'providerAuto' :82
	zhWebProviderMissing       = "{id}（未加载）"           // 'providerMissing' :81

	zhWebProvidersFailed = "无法读取提供方列表，此菜单可能不完整" // 'providersFailed' :83
	zhWebRetry           = "重试"                 // 'retry' :84
	zhWebPluginHint      = "请在该插件自己的设置中配置"      // 'pluginProviderHint' :85

	zhWebSearchGuide   = "网页搜索与获取指南"                                         // 'searchGuide' :90
	zhWebSearchGuideZH = "https://api-docs.deepseek.com/zh-cn/guides/search" // 'searchGuideUrl' :91
	zhWebSearchGuideEN = "https://api-docs.deepseek.com/guides/search"       // 同键 en 版（locales.ts:44）

	zhWebCredential  = "API Key"                               // 'credential' :86
	zhWebBaseURL     = "Base URL"                              // 'baseUrl' :87
	zhWebBaseURLHint = "填写含 API 前缀的地址；留空时使用官方地址；程序追加 {path}"   // 'baseUrlHint' :88（{path} 由调用处替换）
	zhWebMaxUses     = "单次请求最多搜索次数"                            // 'maxUses' :76
	zhWebMaxUsesHint = "每次 Messages 请求允许的服务端搜索次数上限，须为正整数，默认 5" // 'maxUsesHint' :77
	zhWebMaxUsesBad  = "请填正整数；留空表示使用默认值"                       // 'invalidMaxUses' :78

	// legacyKeyHint 带 {ref} 占位（:75）。
	zhWebLegacyKeyHint = "账号模型发起的请求使用已登录账号；其他请求使用此 API Key（{ref}），默认与「模型」页共用"

	zhWebAPIKeySet   = "已配置密钥。"                  // 'apiKeySet' :92
	zhWebAPIKeyUnset = "未配置密钥。"                  // 'apiKeyUnsetApi' :89
	zhWebCount       = "每次搜索结果数"                 // 'count' :93
	zhWebCountHint   = "每次搜索返回的结果条数，1–50，默认 10。" // 'countHint' :94
	zhWebCountBad    = "请填 1–50 的整数；留空表示使用默认值。"  // 'invalidCount' :103
	zhWebUnavailable = "该插件当前未加载，暂时无法配置。"        // 'unavailable' :98

	zhWebBilling         = "计费方式"                                                         // 'billing' :104
	zhWebBillingAuto     = "自动"                                                           // 'billingAuto' :105
	zhWebBillingAccount  = "已登录的 DeepSeek 账号"                                             // 'billingAccount' :107
	zhWebBillingAPIKey   = "DeepSeek API Key"                                             // 'billingApiKey' :109
	zhWebBillingAutoHint = "跟随当前会话的 DeepSeek 账号或 API Key；会话用其他模型时，优先用 API Key，没有则用已登录账号。" // 'billingAutoHint' :106
	zhWebBillingAcctHint = "需要先登录 DeepSeek 账号。"                                           // 'billingAccountHint' :108
	zhWebBillingKeyHint  = "使用「模型」页配置的 API Key，需要先配置。"                                    // 'billingApiKeyHint' :110
	zhWebBillingOverride = "已被 {fields} 覆盖"                                               // 'billingAdvancedOverride' :111
	// zhWebBillingSeparator 是官方 `FIELD_SEPARATOR` 的 zh 值（WebSearchCard.tsx:199）。
	zhWebBillingSeparator = "、"

	// 本仓自己的三句话（官方没有对应键，只在**宿主没给数据**时说，不冒充官方文案）。
	zhWebKeyUnknown      = "未知：还没有拿到凭据状态"
	zhWebProviderUnknown = "未知：宿主未接这项能力的提供方清单"
	zhWebProviderNone    = "宿主未接 web.listProviders，无法列出可选提供方"
)

// ── 状态类型 ──────────────────────────────────────────────────────

// settingsWebProviderOption 是一个 provider 菜单项（官方 `WebProviderOption`，:135-144）。
type settingsWebProviderOption struct {
	// ID 是写进选择的值。
	ID string
	// Title 是 provider 注册的显示名（空 = 没声明）。
	Title string
	// Description 是 provider 注册的说明：`string` 或 `{en,zh,…}` 的 LocalizedText。
	Description any
	// Missing 表示这个 id 是"选中了但宿主当前没注册"（官方 `missing`）。
	Missing bool
}

// settingsWebHint 是 provider 菜单下的一行说明（官方 `<p class=hint>`，:289-290）。
type settingsWebHint struct {
	// Text 是这一行的文字。
	Text string
	// URL 非空表示官方这里是 `<a href>`（整行是链接，:77）。
	URL string
}

// settingsWebEntryKind 是灰面板里一格的种类。
type settingsWebEntryKind uint8

const (
	// settingsWebEntryField 是 `SettingsValueField`（count / Base URL / maxUses）。
	settingsWebEntryField settingsWebEntryKind = iota
	// settingsWebEntryBilling 是官方自己搭的计费菜单（`.panelField`，:221-238）。
	settingsWebEntryBilling
	// settingsWebEntrySecret 是 `SettingsSecretField`（密钥，:93-104）。
	settingsWebEntrySecret
)

// settingsWebPanelEntry 是灰面板里的一格（顺序即官方 JSX 顺序）。
type settingsWebPanelEntry struct {
	Kind    settingsWebEntryKind
	Field   settingsItemFieldState
	Billing settingsWebBillingState
	Secret  settingsWebKeyState
}

// settingsWebBillingState 是一处能力的计费菜单（官方 `WebBillingState`，:122-132）。
type settingsWebBillingState struct {
	// Value 是当前计费；settings 里没有值时读作 'auto'（官方 `?? 'auto'`）。
	Value string
	// Overridden 表示保存后会留下一条 user 层记录。
	Overridden bool
	// AdvancedOverride 是"已用自己端点覆盖计费"的字段名（baseURL 在前）。
	// 非空时菜单**锁死**并挂上官方那句「已被 … 覆盖」标签。
	AdvancedOverride []string
}

// settingsWebKeyState 是密钥控件（官方 `WebApiKeyState`，:152-159）。
type settingsWebKeyState struct {
	// Ref 是控件写的凭据引用：`web-search-deepseek.apiKeyEnv` 或默认。
	Ref string
	// Known 表示凭据域答过这个 ref。
	Known bool
	// Configured 表示宿主报告这个 ref 当前能取到值。
	Configured bool
	// Writable 表示凭据域接受写入；false 时控件禁用。
	Writable bool
	// Text 是草稿文本（官方 secret 字段没有 sectionValue，永远是草稿）。
	Text string
	// StateLabel 是 head 里那枚 Tag 的文案。
	StateLabel string
	// Disabled 是官方 `disabled={!writable || saving}`。
	Disabled bool
	// Note 是本仓自己的补充说明（宿主没接/还没答时用）。
	Note string
}

// settingsWebSectionState 是一个能力那一节（`WebProviderSectionState`，:161-171）。
type settingsWebSectionState struct {
	// Capability 是 search / fetch。
	Capability string
	// Title 是官方 section 标题。
	Title string
	// SectionDOMID / ProviderDOMID 是官方写死的 id。
	SectionDOMID  string
	ProviderDOMID string
	// Selected 是选中的 provider id（空 = 自动选择）。
	Selected string
	// ValueLabel 是下拉按钮上显示的文字。
	ValueLabel string
	// Options 是菜单项（**含**自动选择那一项，官方把 auto 拼在最前）。
	Options []settingsWebProviderOption
	// Auto 表示菜单里有没有"自动选择"那一项（= 部署没钉死 provider）。
	Auto bool
	// Kind 是选中 provider 的形态（none/deepseek/legacy/plugin/local）。
	Kind string
	// Description 是选中 provider 注册的说明，已解析成当前语言（'' = 不画）。
	Description string
	// Hints 是官方页面自己的那几行说明。
	Hints []settingsWebHint
	// ProvidersFailed 表示最近一次 provider 清单读失败（官方 `providersFailed`，
	// 两个菜单下都会画同一句提示 + 「重试」）。
	ProvidersFailed bool
	// Panel 是灰面板里的格子（空 = 官方这一节没有面板）。
	Panel []settingsWebPanelEntry
	// Available / Writable / Revision 是 provider 选择那条命名空间的状态。
	Available bool
	Writable  bool
	Revision  int
}

// settingsItemWebPlan 是官方三个 form 的 `plan()` 合起来的结果。
type settingsItemWebPlan struct {
	// API / Legacy / Selection 是按官方保存顺序排好的三段路径操作。
	API       []apiclient.SettingsOp
	Legacy    []apiclient.SettingsOp
	Selection []apiclient.SettingsOp
	// KeyRef / KeyValue 非空表示要写一次凭据（官方 secret spec 的 `run`）。
	KeyRef   string
	KeyValue string
	// Invalid 表示至少一个草稿解析不了（官方那时 save 不动）。
	Invalid bool
}

// Dirty 报告有没有需要写的东西（官方 `shell().dirty`：plan 非空）。
func (p settingsItemWebPlan) Dirty() bool {
	return len(p.API)+len(p.Legacy)+len(p.Selection) > 0 || p.KeyRef != ""
}

// TotalOps 是这次保存要发的路径操作条数（测试与状态栏用）。
func (p settingsItemWebPlan) TotalOps() int {
	return len(p.API) + len(p.Legacy) + len(p.Selection)
}

// settingsItemWebState 是 #47 那一页这一帧的呈现状态。
type settingsItemWebState struct {
	Search settingsWebSectionState
	Fetch  settingsWebSectionState
	// Count / BaseURL / MaxUses 是三个 `SettingsValueField`（**始终**算，
	// 画不画由所在面板决定：官方 plan 也包含当前没显示的面板里的草稿）。
	Count   settingsItemFieldState
	BaseURL settingsItemFieldState
	MaxUses settingsItemFieldState
	// SearchBilling / FetchBilling 只有对应能力选中 DeepSeek API 时才非 nil。
	SearchBilling *settingsWebBillingState
	FetchBilling  *settingsWebBillingState
	// Key 是密钥控件；nil = 这一帧不画它（未选旧版 provider，或壳没接凭据域）。
	Key *settingsWebKeyState

	plan settingsItemWebPlan
	// selectionBlocked 是官方 `state.selectionBlocked`：invalid/failed 时菜单锁死。
	selectionBlocked bool
	// providersFailed 是官方 `state.providersFailed`（画提示 + 「重试」）。
	providersFailed bool
	// providerWired 表示壳接了 `web.listProviders`；false = 菜单列不出别项（见绘制层）。
	providerWired bool

	// 部署组合值（官方 `scope.getSnapshot().base` 的那几个字段）：菜单交互要靠它判
	// "选中值等于组合值 → 清覆盖发 unset"（controller:409-426）。
	searchComposed        string
	searchComposedSet     bool
	fetchComposed         string
	fetchComposedSet      bool
	searchBillingComposed string
	fetchBillingComposed  string

	webAvailable    bool
	webWritable     bool
	webRevision     int
	apiAvailable    bool
	apiWritable     bool
	apiRevision     int
	legacyAvailable bool
	legacyWritable  bool
	legacyRevision  int
}

// ── 构造 ─────────────────────────────────────────────────────────

// settingsWebCountMaxValue 是 count 上限的指针形式（官方 `.max(50)`）。
var settingsWebCountMaxValue = int64(settingsWebCountMax)

// settingsWebCountSpec 是 `count` 字段声明（官方 controller:294-299 的自定义 parse）。
func settingsWebCountSpec() settingsItemField {
	return settingsItemField{
		Name:    settingsFieldWebCount,
		Kind:    settingsItemFieldNumber,
		Label:   zhWebCount,
		Hint:    zhWebCountHint,
		Invalid: zhWebCountBad,
		DOMID:   settingsWebCountDOMID,
		Limit:   true,
		Min:     settingsWebCountMin,
		Max:     &settingsWebCountMaxValue,
	}
}

// settingsWebBaseURLSpec 是 legacy Base URL（官方 `settingsTextField`，controller:305）。
func settingsWebBaseURLSpec() settingsItemField {
	return settingsItemField{
		Name:    settingsFieldLegacyBaseURL,
		Kind:    settingsItemFieldText,
		Label:   zhWebBaseURL,
		Hint:    strings.ReplaceAll(zhWebBaseURLHint, "{path}", settingsWebMessagesPath),
		Invalid: zhFormInvalidNum,
		DOMID:   settingsWebEndpointDOMID,
	}
}

// settingsWebMaxUsesSpec 是 legacy 最大搜索次数（官方自定义 parse：整数 ≥ 1）。
func settingsWebMaxUsesSpec() settingsItemField {
	return settingsItemField{
		Name:    settingsFieldLegacyMaxUses,
		Kind:    settingsItemFieldNumber,
		Label:   zhWebMaxUses,
		Hint:    zhWebMaxUsesHint,
		Invalid: zhWebMaxUsesBad,
		DOMID:   settingsWebMaxUsesDOMID,
		Limit:   true,
		Min:     settingsWebMaxUsesMin,
	}
}

// buildSettingsItemWebCard 把 describe + provider 清单 + 凭据答案 + 草稿投影成这一页的状态。
func buildSettingsItemWebCard(item settingsItem, describe *apiclient.SettingsDescribeResult,
	snap settingsItemFormSnapshot) settingsItemCardState {
	card := settingsItemCardState{Item: item, Saving: snap.write.Saving, Failed: snap.write.Failed, FailNote: snap.write.Detail}
	state := &settingsItemWebState{}
	card.Web = state
	if describe == nil {
		return card
	}
	card.Known = true
	served := settingsItemServed(describe)
	state.webAvailable = served[settingsNSWeb]
	state.apiAvailable = served[settingsNSWebAPI]
	state.legacyAvailable = served[settingsNSWebSearch]

	webView, webOK := describe.Section(settingsNSWeb)
	if webOK {
		state.webWritable, state.webRevision = webView.Writable, webView.Revision
	}
	apiView, apiOK := describe.Section(settingsNSWebAPI)
	if apiOK {
		state.apiWritable, state.apiRevision = apiView.Writable, apiView.Revision
	}
	legacyView, legacyOK := describe.Section(settingsNSWebSearch)
	if legacyOK {
		state.legacyWritable, state.legacyRevision = legacyView.Writable, legacyView.Revision
	}

	// 选择项：有草稿看草稿，否则看 section 的当前值（官方 `selected()`）。
	searchSelected := settingsWebStagedText(snap.drafts, settingsNSWeb, settingsFieldSearchProvider,
		settingsWebFieldText(webView, webOK, settingsFieldSearchProvider))
	fetchSelected := settingsWebStagedText(snap.drafts, settingsNSWeb, settingsFieldFetchProvider,
		settingsWebFieldText(webView, webOK, settingsFieldFetchProvider))

	// 部署是否钉死 provider（官方 `auto = base?.[field] === undefined`）。
	webBase := settingsItemNamespaceBase(describe, settingsNSWeb)
	searchComposed, searchComposedSet := settingsWebMapText(webBase, settingsFieldSearchProvider)
	fetchComposed, fetchComposedSet := settingsWebMapText(webBase, settingsFieldFetchProvider)
	state.searchComposed, state.searchComposedSet = searchComposed, searchComposedSet
	state.fetchComposed, state.fetchComposedSet = fetchComposed, fetchComposedSet
	// 计费的组合值（官方 `base?.[field] ?? 'auto'`，controller:423）。
	apiBase := settingsItemNamespaceBase(describe, settingsNSWebAPI)
	state.searchBillingComposed = settingsWebComposedBilling(apiBase, settingsFieldSearchBilling)
	state.fetchBillingComposed = settingsWebComposedBilling(apiBase, settingsFieldFetchBilling)

	state.Search = settingsWebBuildSection("search", zhWebSearchTitle, settingsWebSearchSectionDOMID,
		settingsWebSearchProviderDOMID, searchSelected, !searchComposedSet, snap.providers,
		settingsWebSearchMenuOrder, state.apiAvailable, state.legacyAvailable, webOK, state.webWritable, state.webRevision)
	state.Fetch = settingsWebBuildSection("fetch", zhWebFetchTitle, settingsWebFetchSectionDOMID,
		settingsWebFetchProviderDOMID, fetchSelected, !fetchComposedSet, snap.providers,
		settingsWebFetchMenuOrder, state.apiAvailable, state.legacyAvailable, webOK, state.webWritable, state.webRevision)

	// 三个可编辑字段（始终投影：草稿即使当前面板没显示也要进 plan）。
	countSpec := settingsWebCountSpec()
	countState, countOp, countInvalid := buildSettingsItemField(settingsNSWebAPI, countSpec, describe, apiView, apiOK, snap.drafts)
	baseURLSpec := settingsWebBaseURLSpec()
	baseURLState, baseURLOp, baseURLInvalid := buildSettingsItemField(settingsNSWebSearch, baseURLSpec, describe, legacyView, legacyOK, snap.drafts)
	maxUsesSpec := settingsWebMaxUsesSpec()
	maxUsesState, maxUsesOp, maxUsesInvalid := buildSettingsItemField(settingsNSWebSearch, maxUsesSpec, describe, legacyView, legacyOK, snap.drafts)
	state.Count, state.BaseURL, state.MaxUses = countState, baseURLState, maxUsesState

	// 计费（只有对应能力选中 DeepSeek API 时才存在）。
	searchBillingState, searchBillingOp := settingsWebBuildBilling(settingsNSWebAPI, "search", describe, apiView, apiOK, snap.drafts)
	fetchBillingState, fetchBillingOp := settingsWebBuildBilling(settingsNSWebAPI, "fetch", describe, apiView, apiOK, snap.drafts)

	// 密钥控件（只有选中旧版搜索、且壳接了凭据域时才画）。
	keyRef := settingsWebKeyRef(legacyView, legacyOK)
	state.Key = settingsWebBuildKey(snap, keyRef, state.Search.Kind == settingsWebKindLegacy)

	// 面板（官方 JSX 顺序，逐行对照 WebSearchCard.tsx:79-127）。
	if state.Search.Kind == settingsWebKindDeepSeek {
		state.SearchBilling = searchBillingState
		state.Search.Panel = append(state.Search.Panel, settingsWebPanelEntry{Kind: settingsWebEntryField, Field: countState})
		if searchBillingState != nil {
			state.Search.Panel = append(state.Search.Panel, settingsWebPanelEntry{Kind: settingsWebEntryBilling, Billing: *searchBillingState})
		}
	}
	if state.Search.Kind == settingsWebKindLegacy && state.Key != nil {
		state.Search.Panel = append(state.Search.Panel, settingsWebPanelEntry{Kind: settingsWebEntrySecret, Secret: *state.Key})
		state.Search.Panel = append(state.Search.Panel,
			settingsWebPanelEntry{Kind: settingsWebEntryField, Field: baseURLState},
			settingsWebPanelEntry{Kind: settingsWebEntryField, Field: maxUsesState})
	}
	if state.Fetch.Kind == settingsWebKindDeepSeek && fetchBillingState != nil {
		state.FetchBilling = fetchBillingState
		state.Fetch.Panel = append(state.Fetch.Panel, settingsWebPanelEntry{Kind: settingsWebEntryBilling, Billing: *fetchBillingState})
	}

	// 选择项的下拉禁用判据：官方用 `!state.writable || state.saving`，加上
	// `selectionBlocked`（invalid/failed 时锁死，避免"改着改着把错的存下去"）。
	state.providerWired = snap.providerWired
	state.providersFailed = snap.providerWired && snap.providers == nil
	state.Search.ProvidersFailed = state.providersFailed
	state.Fetch.ProvidersFailed = state.providersFailed

	// plan（官方保存顺序：api → legacy → selection，密钥在 selection 之后）。
	plan := settingsItemWebPlan{}
	if countOp != nil {
		plan.API = append(plan.API, *countOp)
	}
	if searchBillingOp != nil {
		plan.API = append(plan.API, *searchBillingOp)
	}
	if fetchBillingOp != nil {
		plan.API = append(plan.API, *fetchBillingOp)
	}
	if baseURLOp != nil {
		plan.Legacy = append(plan.Legacy, *baseURLOp)
	}
	if maxUsesOp != nil {
		plan.Legacy = append(plan.Legacy, *maxUsesOp)
	}
	searchSpec := settingsItemField{Name: settingsFieldSearchProvider, Kind: settingsItemFieldText}
	_, searchProviderOp, searchProviderInvalid := buildSettingsItemField(settingsNSWeb, searchSpec, describe, webView, webOK, snap.drafts)
	fetchSpec := settingsItemField{Name: settingsFieldFetchProvider, Kind: settingsItemFieldText}
	_, fetchProviderOp, fetchProviderInvalid := buildSettingsItemField(settingsNSWeb, fetchSpec, describe, webView, webOK, snap.drafts)
	if searchProviderOp != nil {
		plan.Selection = append(plan.Selection, *searchProviderOp)
	}
	if fetchProviderOp != nil {
		plan.Selection = append(plan.Selection, *fetchProviderOp)
	}
	// 密钥草稿只在"控件还画着 + 引用没变"时才写（官方 `dropStaleKeyDraft` 的等价结果）：
	// 草稿记录它被输入时的 ref，引用变了就整份丢，绝不跨引用写。
	if state.Key != nil && state.Search.Kind == settingsWebKindLegacy {
		if draft, ok := snap.drafts[settingsItemDraftKey(settingsNSWeb, settingsFieldLegacySecret)]; ok &&
			draft.Ref == keyRef && strings.TrimSpace(draft.Text) != "" {
			plan.KeyRef, plan.KeyValue = keyRef, strings.TrimSpace(draft.Text)
		}
	}
	plan.Invalid = countInvalid || baseURLInvalid || maxUsesInvalid || searchProviderInvalid || fetchProviderInvalid ||
		settingsWebBillingInvalid(snap.drafts)
	state.plan = plan
	// 官方 `selectionBlocked = shell.invalid || shell.failed`（controller:74）。
	state.selectionBlocked = plan.Invalid || card.Failed

	// 写状态与可用性（官方 `projection()` 的 shell 部分）。
	if state.webAvailable {
		card.Writable = state.webWritable
		for _, servedWritable := range []struct {
			available bool
			writable  bool
		}{
			{state.apiAvailable, state.apiWritable},
			{state.legacyAvailable, state.legacyWritable},
		} {
			if servedWritable.available && !servedWritable.writable {
				card.Writable = false
			}
		}
	}
	return card
}

// settingsWebBuildSection 造一个能力那一节的状态。
func settingsWebBuildSection(capability, title, sectionDOMID, providerDOMID, selected string, auto bool,
	providers *apiclient.WebProviderList, order []string, apiAvailable, legacyAvailable,
	webOK, webWritable bool, webRevision int) settingsWebSectionState {
	section := settingsWebSectionState{
		Capability: capability, Title: title, SectionDOMID: sectionDOMID, ProviderDOMID: providerDOMID,
		Selected: selected, Auto: auto, Available: webOK, Writable: webWritable, Revision: webRevision,
	}
	section.Options = settingsWebProviderOptions(providers, capability, selected)
	section.Kind = settingsWebSectionKind(capability, selected, providers, apiAvailable, legacyAvailable)
	section.ValueLabel = settingsWebProviderAuto(settingsWebProviderOptions(providers, capability, selected), selected)
	section.Description = settingsWebSelectedDescription(section.Options, selected)
	section.Hints = settingsWebSectionHints(capability, section.Kind)
	return section
}

// settingsWebProviderOptions 是官方 `options()`（controller:328-344）：
// 官方四个 id 按 `MENU_ORDER` 在前，其余按宿主顺序；选中的 id 宿主没注册时补一条
// `missing`；清单还没到就只列选中的那个 id。
func settingsWebProviderOptions(providers *apiclient.WebProviderList, capability, selected string) []settingsWebProviderOption {
	var registered []apiclient.WebProviderInfo
	if providers != nil {
		if capability == "search" {
			registered = providers.Search
		} else {
			registered = providers.Fetch
		}
	}
	if providers == nil {
		if selected == "" {
			return nil
		}
		return []settingsWebProviderOption{{ID: selected}}
	}
	order := settingsWebSearchMenuOrder
	if capability == "fetch" {
		order = settingsWebFetchMenuOrder
	}
	options := make([]settingsWebProviderOption, 0, len(registered)+1)
	for _, id := range order {
		for _, info := range registered {
			if info.ID == id {
				options = append(options, settingsWebProviderOption{ID: info.ID, Title: info.Title, Description: info.Description})
			}
		}
	}
	for _, info := range registered {
		if settingsWebContains(order, info.ID) {
			continue
		}
		options = append(options, settingsWebProviderOption{ID: info.ID, Title: info.Title, Description: info.Description})
	}
	if selected != "" && !settingsWebContainsProvider(registered, selected) {
		options = append(options, settingsWebProviderOption{ID: selected, Missing: true})
	}
	return options
}

// settingsWebSectionKind 是官方 `searchKind()` / `fetchKind()`（controller:380-394）。
func settingsWebSectionKind(capability, selected string, providers *apiclient.WebProviderList,
	apiAvailable, legacyAvailable bool) string {
	registered := func(id string) bool {
		if providers == nil {
			return true // 清单还没到：不替宿主判"没注册"
		}
		if capability == "search" {
			return settingsWebContainsProvider(providers.Search, id)
		}
		return settingsWebContainsProvider(providers.Fetch, id)
	}
	if selected == "" || !registered(selected) {
		return settingsWebKindNone
	}
	if capability == "search" {
		switch selected {
		case settingsWebProviderAPISearch:
			if apiAvailable {
				return settingsWebKindDeepSeek
			}
			return settingsWebKindNone
		case settingsWebProviderLegacySearch:
			if legacyAvailable {
				return settingsWebKindLegacy
			}
			return settingsWebKindNone
		}
	} else {
		switch selected {
		case settingsWebProviderLocalFetch:
			return settingsWebKindLocal
		case settingsWebProviderAPIFetch:
			if apiAvailable {
				return settingsWebKindDeepSeek
			}
			return settingsWebKindNone
		}
	}
	if providers == nil {
		return settingsWebKindNone
	}
	return settingsWebKindPlugin
}

// settingsWebSectionHints 是官方 ProviderSelect 的 `hint`（WebSearchCard.tsx:76-78 / :121）。
func settingsWebSectionHints(capability, kind string) []settingsWebHint {
	if capability == "search" && kind == settingsWebKindDeepSeek {
		return []settingsWebHint{{Text: zhWebSearchGuide, URL: zhWebSearchGuideZH}}
	}
	if kind == settingsWebKindPlugin {
		return []settingsWebHint{{Text: zhWebPluginHint}}
	}
	return nil
}

// settingsWebProviderAuto 取下拉按钮上的文字（官方 `valueLabel`，WebSearchCard.tsx:287）。
func settingsWebProviderAuto(options []settingsWebProviderOption, selected string) string {
	for _, option := range options {
		if option.ID == selected {
			return settingsWebProviderLabel(option)
		}
	}
	return zhWebProviderAuto
}

// settingsWebProviderLabel 是官方 `providerLabel()`（WebSearchCard.tsx:45-49）。
func settingsWebProviderLabel(option settingsWebProviderOption) string {
	name := ""
	if title, ok := settingsWebTitles[option.ID]; ok {
		name = title
	} else if option.Title != "" {
		name = option.Title
	} else {
		name = option.ID
	}
	if option.Missing {
		return strings.ReplaceAll(zhWebProviderMissing, "{id}", name)
	}
	return name
}

// settingsWebSelectedDescription 取选中 provider 的注册说明（已解析成当前语言）。
//
// 官方 `resolveText(description)`（controller:264-266 → WebSearchCard.tsx:64-67）：
// 空翻译 = 不画那一行。
func settingsWebSelectedDescription(options []settingsWebProviderOption, selected string) string {
	for _, option := range options {
		if option.ID == selected {
			return settingsWebResolveText(option.Description)
		}
	}
	return ""
}

// settingsWebResolveText 把 LocalizedText（`string` 或 `{en,zh,…}`）解析成当前语言。
//
// 本仓界面全程官方 zh 文案（与另外三条插件设置页一致），所以优先 `zh` / `zh-cn` /
// `zh-CN`，再退 `en`；两者都没有就返回空串（官方空翻译也是返回空串，不是编一个）。
func settingsWebResolveText(description any) string {
	switch typed := description.(type) {
	case nil:
		return ""
	case string:
		return typed
	case map[string]any:
		for _, key := range []string{"zh", "zh-cn", "zh-CN", "zh-Hans"} {
			if text, ok := typed[key].(string); ok && text != "" {
				return text
			}
		}
		if text, ok := typed["en"].(string); ok {
			return text
		}
	}
	return ""
}

// settingsWebBuildBilling 造一处能力的计费菜单（官方 `billing()`，controller:143-150）。
func settingsWebBuildBilling(ns, capability string, describe *apiclient.SettingsDescribeResult,
	apiView apiclient.SettingsSectionView, apiOK bool, drafts map[string]settingsItemDraft) (*settingsWebBillingState, *apiclient.SettingsOp) {
	fieldName := capability + "Billing"
	spec := settingsItemField{Name: fieldName, Kind: settingsItemFieldText}
	fieldState, op, _ := buildSettingsItemField(ns, spec, describe, apiView, apiOK, drafts)
	if !apiOK {
		return nil, op
	}
	value := fieldState.Text
	if !settingsWebContains(settingsWebBillings, value) {
		value = settingsWebDefaultBilling
	}
	override := make([]string, 0, 2)
	for _, name := range []string{capability + "BaseURL", capability + "ApiKeyEnv"} {
		if text := settingsWebFieldText(apiView, apiOK, name); text != "" {
			override = append(override, name)
		}
	}
	return &settingsWebBillingState{Value: value, Overridden: fieldState.Overridden, AdvancedOverride: override}, op
}

// settingsWebBillingInvalid 报告两个计费草稿里有没有解析不了的。
//
// 计费草稿只由菜单产生（值恒为三选一），所以正常路径不会 invalid；这里保留判据是
// 为了与官方 plan 的 `invalid` 语义一致（万一有坏草稿，保存必须不动）。
func settingsWebBillingInvalid(drafts map[string]settingsItemDraft) bool {
	for _, name := range []string{settingsFieldSearchBilling, settingsFieldFetchBilling} {
		draft, ok := drafts[settingsItemDraftKey(settingsNSWebAPI, name)]
		if !ok || draft.Clear {
			continue
		}
		if !settingsWebContains(settingsWebBillings, draft.Text) {
			return true
		}
	}
	return false
}

// settingsWebKeyRef 是密钥控件写的引用（官方 `ref()`，controller:397-399）。
func settingsWebKeyRef(legacyView apiclient.SettingsSectionView, legacyOK bool) string {
	ref := settingsWebFieldText(legacyView, legacyOK, settingsFieldLegacyKeyEnv)
	if ref == "" {
		return settingsWebDefaultAPIKeyRef
	}
	return ref
}

// settingsWebBuildKey 造密钥控件；返回 nil = 这一帧不画它。
//
// 不画的三条真实原因：没选旧版 provider（官方 `keyShown()`）、壳没接凭据域
// （`CredentialStatus == nil`，画了也没人答"设没设"）、凭据域没接线。
func settingsWebBuildKey(snap settingsItemFormSnapshot, ref string, shown bool) *settingsWebKeyState {
	if !shown || !snap.credentialWired {
		return nil
	}
	state := &settingsWebKeyState{Ref: ref, Known: false, Writable: true}
	if info, ok := snap.credentials[ref]; ok {
		state.Known = true
		state.Configured = info.Configured
		state.Writable = info.Writable
	}
	// 草稿：official 的 secret 字段没有 sectionValue，文本永远来自草稿；
	// 引用变了就不认这份草稿（官方 `dropStaleKeyDraft` 的等价结果）。
	if draft, ok := snap.drafts[settingsItemDraftKey(settingsNSWeb, settingsFieldLegacySecret)]; ok &&
		draft.Ref == ref {
		state.Text = draft.Text
	}
	switch {
	case !state.Known:
		state.StateLabel = zhWebKeyUnknown
		state.Note = zhWebKeyUnknown
	case state.Configured:
		state.StateLabel = zhWebAPIKeySet
	default:
		state.StateLabel = zhWebAPIKeyUnset
	}
	state.Disabled = !state.Writable || snap.write.Saving
	return state
}

// settingsWebKeyHint 是密钥控件下面那行 hint（官方 `legacyKeyHint({ref})`）。
func settingsWebKeyHint(ref string) string {
	return strings.ReplaceAll(zhWebLegacyKeyHint, "{ref}", ref)
}

// ── 菜单交互（官方 select / selectBilling 的等价物）─────────────────────

// settingsWebStageSelection 把一个 provider 选择写进草稿（官方 `select()`，controller:409-413）。
//
// 选中值等于部署组合值、或选了"自动选择"时**清覆盖**（发 unset），而不是存一份
// 与组合值逐字相同的副本——否则用户永远回不到"跟随部署默认"。
func (f *settingsItemForm) settingsWebStageSelection(field, id, composed string, composedSet bool,
	revision int) {
	if id == "" || composedSet && composed == id {
		f.stage(settingsNSWeb, field, settingsItemDraft{Text: "", Clear: true}, revision)
		return
	}
	f.stage(settingsNSWeb, field, settingsItemDraft{Text: id}, revision)
}

// settingsWebStageBilling 把一处计费写进草稿（官方 `selectBilling()`，controller:421-426）。
func (f *settingsItemForm) settingsWebStageBilling(capability, billing, composed string, revision int) {
	field := capability + "Billing"
	if composed == billing {
		f.stage(settingsNSWebAPI, field, settingsItemDraft{Text: "", Clear: true}, revision)
		return
	}
	f.stage(settingsNSWebAPI, field, settingsItemDraft{Text: billing}, revision)
}

// settingsWebStageKey 把密钥草稿记账：官方 `editKey` 会记下当时的引用（controller:427-432）。
//
// `Ref` 是**唯一**决定"写到哪个凭据"的字段：草稿被输入后引用变了，plan 就不认它
// （见 buildSettingsItemWebCard 的密钥分支），所以不存在"跨引用写密钥"的路径。
func (f *settingsItemForm) settingsWebStageKey(ref, text string, revision int) {
	f.stage(settingsNSWeb, settingsFieldLegacySecret, settingsItemDraft{Text: text, Ref: ref}, revision)
}

// ── 小工具 ────────────────────────────────────────────────────────

// settingsWebContains 报告切片里有没有这个字符串。
func settingsWebContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// settingsWebContainsProvider 报告清单里有没有这个 provider id。
func settingsWebContainsProvider(list []apiclient.WebProviderInfo, id string) bool {
	for _, info := range list {
		if info.ID == id {
			return true
		}
	}
	return false
}

// settingsWebStagedText 是官方 `field(name).text`：有草稿看草稿，否则看当前值。
func settingsWebStagedText(drafts map[string]settingsItemDraft, ns, field, current string) string {
	if draft, ok := drafts[settingsItemDraftKey(ns, field)]; ok {
		return draft.Text
	}
	return current
}

// settingsWebFieldText 取 section 里某个字段的当前字符串值（” = 没有/不是字符串）。
func settingsWebFieldText(view apiclient.SettingsSectionView, sectionOK bool, name string) string {
	if !sectionOK {
		return ""
	}
	for _, field := range view.Fields {
		if field.Name == name {
			return formatItemText(field.Current)
		}
	}
	return ""
}

// settingsItemNamespaceBase 读命名空间的 base 层（官方 `scope.getSnapshot().base`）。
func settingsItemNamespaceBase(describe *apiclient.SettingsDescribeResult, ns string) map[string]any {
	return settingsItemNamespaceLayer(describe, ns, true)
}

// settingsItemNamespaceValue 读命名空间的有效值层（官方 `getSnapshot().value`）。
func settingsItemNamespaceValue(describe *apiclient.SettingsDescribeResult, ns string) map[string]any {
	return settingsItemNamespaceLayer(describe, ns, false)
}

// settingsItemNamespaceLayer 解析 describe 里某个命名空间的 base / value 层。
func settingsItemNamespaceLayer(describe *apiclient.SettingsDescribeResult, ns string, base bool) map[string]any {
	if describe == nil {
		return nil
	}
	for _, view := range describe.Namespaces {
		if view.NS != ns {
			continue
		}
		raw := view.Value
		if base {
			raw = view.Base
		}
		if len(raw) == 0 {
			return nil
		}
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return nil
		}
		return decoded
	}
	return nil
}

// settingsWebMapText 取一个 map 里的字符串值，并报告这个键在不在。
//
// "在不在"是官方 `base?.[field] === undefined` 的判据，**不能**用"值是不是空串"
// 代替：base 里显式写了空串表示"钉死成空"，与没写是两件事。
func settingsWebMapText(values map[string]any, key string) (string, bool) {
	if values == nil {
		return "", false
	}
	raw, ok := values[key]
	if !ok {
		return "", false
	}
	return formatItemText(raw), true
}

// settingsWebComposedBilling 取 base 里的计费组合值（官方 `?? 'auto'`）。
func settingsWebComposedBilling(base map[string]any, field string) string {
	if value, ok := settingsWebMapText(base, field); ok && settingsWebContains(settingsWebBillings, value) {
		return value
	}
	return settingsWebDefaultBilling
}

// settingsWebSelectionField 是能力 → 选择字段名。
func settingsWebSelectionField(capability string) string {
	if capability == "fetch" {
		return settingsFieldFetchProvider
	}
	return settingsFieldSearchProvider
}

// ── 保存（官方三个 form 的 save() + 密钥写入）─────────────────────────

// applySettingsItemWebSave 是 #47 那一页的保存路径，逐条照官方 `save()`：**先 api、
// 再 legacy、最后 selection**（官方注释写明原因：选择生效时它依赖的设置必须已经落库），
// 任一节被拒就停下、草稿全留；密钥写属于 selection 那一节，在其 mutate 之后跑。
//
// **在调用方的 goroutine 里跑**（绘制线程不能等 HTTP）。
func (p *Plugin) applySettingsItemWebSave(act pluginapi.Actions, plan settingsItemWebPlan,
	baseline map[string]int, form *settingsItemForm, drafts map[string]settingsItemDraft) {
	if act.SettingsMutate == nil {
		form.setWrite(settingsItemWrite{Failed: true, Detail: "壳未接 settings.mutate"})
		if act.Invalidate != nil {
			act.Invalidate()
		}
		return
	}
	stages := []struct {
		ns  string
		ops []apiclient.SettingsOp
	}{
		{settingsNSWebAPI, plan.API},
		{settingsNSWebSearch, plan.Legacy},
		{settingsNSWeb, plan.Selection},
	}
	for _, stage := range stages {
		if len(stage.ops) == 0 {
			// 官方 `r = !ops.length || await scope.mutate(...)`：没有操作就不发请求。
			continue
		}
		view, err := act.SettingsMutate(stage.ns, stage.ops, baseline[stage.ns])
		if err != nil {
			form.setWrite(settingsItemWrite{Failed: true, Detail: sectionWriteDetail(stage.ns, err)})
			if act.Invalidate != nil {
				act.Invalidate()
			}
			return
		}
		if view.Revision > 0 {
			form.setBaseline(stage.ns, view.Revision)
		}
	}
	// 密钥：官方 `writeKey()` = `credentials.set(ref, value)` 然后**重新读一次**状态
	// （结果是 void，不读就不知道现在配没配）。trim 是官方的：`l = text.trim()`。
	if plan.KeyRef != "" {
		if act.CredentialSet == nil {
			form.setWrite(settingsItemWrite{Failed: true, Detail: "壳未接 credentials.set"})
			if act.Invalidate != nil {
				act.Invalidate()
			}
			return
		}
		if err := act.CredentialSet(plan.KeyRef, plan.KeyValue); err != nil {
			form.setWrite(settingsItemWrite{Failed: true, Detail: sectionWriteDetail(plan.KeyRef, err)})
			if act.Invalidate != nil {
				act.Invalidate()
			}
			return
		}
	}
	keys := make([]string, 0, len(drafts))
	for key := range drafts {
		keys = append(keys, key)
	}
	form.clearDrafts(keys)
	form.setWrite(settingsItemWrite{})
	if act.Invalidate != nil {
		act.Invalidate()
	}
}
