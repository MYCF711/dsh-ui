// official_plugins.go —— 设置面板「内置插件」页的**模型**。
//
// # 官方结构（两段来源，逐条照做）
//
// 设置里这一页由两个官方包拼成：
//
//	① ui-settings-plugins（外壳）：nav「内置插件」、`h2` 标题、简介、
//	   `settings.plugins.tab` 的 tab 条；**只有一个 tab 时直接画那一页、不画 tab 条**
//	   （OFF:ui-settings-plugins/src/client/PluginsSettingsSection.tsx:40,57-58）。
//	② ui-settings-plugin-inventory（只读清单页）：搜索框 + 两段分组
//	   （「会话插件」← agentPresets，「全局插件」← entries）+ 卡片网格 + 卡片展开详情
//	   （OFF:ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.tsx:427-578）。
//
// 本仓宿主**不产出 agentPresets**（DSH-GO internal/transport/
// handlers_plugin_inventory_official.go:20-23 写明理由），所以这一页在当前宿主上
// 只有「全局插件」一段——**这不是省略，是那一段真的不存在**：官方也只在部署组合了
// agent-preset roster 时才画它（同文件 types.ts:72-76 的 optional 语义）。
//
// # 文案来源
//
// 一律取官方 `locale/zh` 字典原文：
//
//	OFF:ui-settings-plugins/src/client/locales.ts:16-22   （nav/title/intro/tabs/empty）
//	OFF:ui-settings-plugin-inventory/src/client/locales.ts:4-42（清单页全部键）
//
// **不翻译、不改写、不合并**：两处 `empty` 是两个不同的键（一个是"本部署没有开放
// 任何插件视图"，一个是"暂无插件"），合并只会把判断条件弄丢。
//
// # 规矩：只画真值
//
//   - 宿主没接 `pluginInventory.list`（inventory==nil）= **未知**，画说明、不画卡片；
//   - 清单是空数组 = 宿主真的查过了、一个都没有 -> 画官方 `empty` 文案；
//   - `meta` 缺席 = 标题回退到模块名（官方 `moduleShortName`），不编标题；
//   - `fiberPhase: null` = 官方 `unobserved`（未运行），不等于"未知"。
package dssettings

import (
	"sort"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// ── 官方 zh 文案（逐字，勿改）────────────────────────────────────────

const (
	// zhPluginsIntro 是 `settings.plugins` 的简介
	// （ui-settings-plugins/src/client/locales.ts:19）。
	zhPluginsIntro = "查看内置部署的插件列表"
	// zhPluginsTabLabel 是清单 tab 的标签（ui-settings-plugin-inventory/src/client/locales.ts:5）。
	zhPluginsTabLabel = "插件列表"

	// zhPluginsLoading 是读取中的状态文案（同处 :6）。
	zhPluginsLoading = "正在读取插件…"
	// zhPluginsError 是读取失败文案（同处 :10）。
	zhPluginsError = "暂时无法读取插件。"
	// zhPluginsRetry 是重试按钮（同处 :12）。
	zhPluginsRetry = "重试"
	// zhPluginsSearch 是搜索框的占位与可访问名（同处 :13）。
	zhPluginsSearch = "搜索插件"
	// zhPluginsEmpty 是**清单为空**的文案（同处 :14）。
	zhPluginsEmpty = "暂无插件。"
	// zhPluginsEmptySearch 是搜索无命中的文案（同处 :15）。
	zhPluginsEmptySearch = "没有匹配的插件。"
	// zhPluginsMetadataError 是包元信息错误（同处 :11，`{error}` 是占位符）。
	zhPluginsMetadataError = "包元信息错误：{error}"

	// zhPluginsGlobalTitle / zhPluginsGlobalSubtitle 是全局段标题与副标题（同处 :21-22）。
	zhPluginsGlobalTitle    = "全局插件"
	zhPluginsGlobalSubtitle = "系统与所有会话共用"
	// zhPluginsPresetTitle / zhPluginsPresetSubtitle 是会話段（同处 :16-17）。
	//
	// 本仓宿主不产出 agentPresets，所以这两个键**当前不会出现在界面上**；
	// 保留它们是为了在宿主将来产出 roster 时不必重新找证据（台账在
	// docs/official-ui-structure-evidence-rc2.md 的 #42 条）。
	zhPluginsPresetTitle    = "会话插件"
	zhPluginsPresetSubtitle = "由 Agent 预设按会话组成"

	// zhPluginsEnabledTag / zhPluginsDisabledTag / zhPluginsFailedTag 是行尾状态标签（同处 :26,27,30）。
	zhPluginsEnabledTag  = "已启用"
	zhPluginsDisabledTag = "已停用"
	zhPluginsFailedTag   = "启动失败"

	// zhPluginsModuleLabel 是详情里"完整名称"那行的标签（同处 :31）。
	zhPluginsModuleLabel = "完整名称"
	// zhPluginsConfiguration 是详情里"配置状态"那行的标签（同处 :34）。
	zhPluginsConfiguration = "配置状态"
	// zhPluginsRuntime 是详情里"运行状态"那行的标签（同处 :35）。
	zhPluginsRuntime = "运行状态"

	// 五个 fiber 阶段各自的 zh 文案（同处 :36-41，顺序与官方键序一致）。
	zhPluginsPhaseUnobserved = "未运行"
	zhPluginsPhasePending    = "等待依赖"
	zhPluginsPhaseLoading    = "加载中"
	zhPluginsPhaseActive     = "运行中"
	zhPluginsPhaseFailed     = "启动失败"
	zhPluginsPhaseUnloading  = "卸载中"
)

// PluginsTab 是 `settings.plugins.tab` 上的一项（官方 `PluginsSettingsTabEntry`，
// OFF:ui-settings-plugins/src/client/PluginsSettingsSection.tsx:11-15）。
//
// # 为什么本仓也要有这张台账
//
// 官方那一页是**外壳 + 别人贡献的 tab**：外壳只管导航项与 tab 条，页面内容由
// `settings.plugins.tab` 的贡献方渲染（同文件 :1-8,:76-84）。本仓只有一个贡献方
// （只读清单），但台账必须真实存在——否则"只有一个 tab 所以不画 tab 条"这句话
// 就没有依据，只是"我们恰好只做了一个页"。
//
// 判据照官方：`len(tabs) == 1` 直接把那一页当整页渲染、**不画 tab 条**；
// `len(tabs) > 1` 才画（:40,57-58）。
type PluginsTab struct {
	// ID 是注册 id（官方 `entry.options.id`）。
	ID string
	// Order 是排序键（官方 `entry.options.order`，升序）。
	Order int
	// Label 是 tab 文案（官方 `resolveSlotLabel` 的结果）。
	Label string
	// Kind 决定这一页怎么渲染。现在只有 `PluginsTabInventory` 一种真实现；
	// 不认识的 kind **不静默画空**，界面上会明说"这一页没实现"。
	Kind string
}

const (
	// PluginsTabInventory 是只读清单页（官方
	// `ui-settings-plugin-inventory` 注册的那个 `id: 'all', order: 10`，:56-63）。
	PluginsTabInventory = "inventory"
	// PluginsTabManagable 是**可配置管理页**（官方 `ui-plugin-manager` 贡献的那一页，
	// 含安装/更新/启停/详情）。本仓没有实现，**不注册**——注册了就成了死 tab。
	PluginsTabManagable = "manage"
)

// DefaultPluginsTabs 是产品路径上的 tab 台账。
//
// 只登记一个：只读清单。第二条（可配置管理页）归 #29 `ui-plugin-manager`，
// 需要 install/update/setBundleEnabled 等一整套写动作，本轮未做，已登记在报告里。
func DefaultPluginsTabs() []PluginsTab {
	return []PluginsTab{{ID: "all", Order: 10, Label: zhPluginsTabLabel, Kind: PluginsTabInventory}}
}

// PluginsState 是清单页的读取状态（官方 `ViewState`，
// OFF:ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.tsx:48-51）。
type PluginsState string

const (
	// PluginsLoading 表示还在读（官方 `{ status: 'loading' }`）。
	PluginsLoading PluginsState = "loading"
	// PluginsError 表示读失败（官方 `{ status: 'error' }`）。
	PluginsError PluginsState = "error"
	// PluginsReady 表示拿到了一份快照（官方 `{ status: 'ready' }`）。
	PluginsReady PluginsState = "ready"
	// PluginsUnknown 表示**宿主没接这条能力**（官方没有这个态：它一定拿得到
	// 一个 Promise；本仓的壳在老宿主上可能根本没有 `pluginInventory.list`）。
	//
	// 它与 `PluginsReady` + 空清单**必须分开**：「宿主没这能力」和「宿主管过、
	// 一个插件都没有」是两句话。
	PluginsUnknown PluginsState = "unknown"
)

// PluginFact 是卡片展开后的一行 `dt`/`dd`（官方 `CardFacts` 的 facts 数组，
// OFF:ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.tsx:180-204）。
type PluginFact struct {
	// Label 是 `dt`（官方 `完整名称` / `配置状态` / `运行状态` …）。
	Label string
	// Value 是 `dd`。
	Value string
}

// PluginCard 是一张插件卡（官方 `PluginCard` 的投影，
// OFF:ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.tsx:123-178）。
type PluginCard struct {
	// RowKey 是官方 `rowKey`（全局段 `<entryId>`），展开态按它记。
	RowKey string
	// EntryID 是官方 `entryId`；空串 = 宿主没给（官方类型里它必填，这里只是防御）。
	EntryID string
	// EntryChip 是卡片左下角的 entry id 角标文本；空串 = **不画角标**
	// （官方 `idAddsToTitle`：剥掉 `include:` 后与标题相同就不画，
	// OFF:...PluginInventorySettingsTab.tsx:83-86）。
	EntryChip string
	// ModuleName 是官方 `moduleName`（详情里的「完整名称」）。
	ModuleName string
	// Title 是卡片标题：`meta.title` 按语言解析，缺席回退到短模块名
	// （官方 `pluginText`，:93-100）。
	Title string
	// Desc 是简介；空串 = 不画简介行（官方 `description === undefined` 时不画）。
	Desc string
	// MetaError 是包元信息错误的正文（已按官方模板填好）；空串 = 不画。
	MetaError string
	// Enabled 是官方 `enabled`。
	Enabled bool
	// Phase 是官方 `fiberPhase` 的归一值；空串 = 官方 `null`。
	Phase string
	// Failed 是官方 `fiberPhase === 'failed'`。
	Failed bool
	// State 是行尾状态标签的文案；Tag=false 时它只用于可访问名。
	State string
	// Tag 表示行尾**画不画**状态标签（官方：只在偏离默认态时画，
	// OFF:...PluginInventorySettingsTab.tsx:246-249）。
	Tag bool
	// PhaseDot 表示行尾画不画阶段点（官方只给 pending/loading/unloading 画，
	// :206-234）。
	PhaseDot bool
	// Diagnostic 是宿主报的启动失败/版本不一致说明；空串 = 不画。
	Diagnostic string
	// Facts 是展开后的 `dt`/`dd` 行（官方 `CardFacts` 的 facts）。
	Facts []PluginFact
	// Expanded 是这一帧是否展开。
	Expanded bool
}

// PluginGroup 是清单页的一段（官方两段之一，:478-575）。
type PluginGroup struct {
	// Key 是段的稳定标识：`preset` / `global`（官方 `data-plugin-scope`）。
	Key string
	// Title 是段标题（官方 `presetTitle` / `globalTitle`）。
	Title string
	// Subtitle 是段副标题（官方 `presetSubtitle` / `globalSubtitle`）。
	Subtitle string
	// Open 是这一帧是否展开。
	Open bool
	// Count 是这一段的条目数（官方 `globalCount`；搜索后是命中数）。
	Count int
	// Cards 是卡片（**按官方顺序**：先 failed 再其余，各自保持 Loader 顺序，:295-301）。
	Cards []PluginCard
}

// PluginsPanel 是「内置插件」页这一帧的全部内容。
type PluginsPanel struct {
	// State 是读取状态。
	State PluginsState
	// Note 是状态说明（未知原因 / 失败原因），画在页头下方。
	Note string
	// ShowRetry 表示画「重试」按钮（官方 error 态才有，:454-461）。
	ShowRetry bool
	// Query 是当前搜索串（官方受控输入；空串 = 没在搜索）。
	Query string
	// Searching 表示 Query 非空（官方 `searching`）。
	Searching bool
	// Groups 是分段（官方顺序：会话插件在前、全局插件在后）。
	Groups []PluginGroup
	// Empty 表示清单真的为空（官方 `empty` 文案，:475）。
	Empty bool
	// EmptySearch 表示有清单但搜索无命中（官方 `emptySearch`，:476）。
	EmptySearch bool
	// ManagementAvailable 是官方 `managementAvailable`（本仓宿主给 true）。
	//
	// 它决定的是**可配置管理页**（另一个包 #29 的面），本页只做只读清单，
	// 所以这里只登记事实、不据此画开关。
	ManagementAvailable bool
	// Tabs 是按 order 排好的 tab 台账（官方 `useTabs` 的投影，:34-36）。
	Tabs []PluginsTab
	// ShowTabBar 表示要画 tab 条：**只有多 tab 时才画**（官方 :57-58）。
	ShowTabBar bool
	// ActiveTab 是当前页的 tab id。
	ActiveTab string
	// TabMissing 非空表示选中的 tab 没有真实现（不静默画空）。
	TabMissing string
	// HasAgentPresets 表示宿主真的给了 `agentPresets` 这个键
	// （false = 这个部署没有这个面，官方也不画那一段）。
	HasAgentPresets bool
	// PresetGap 非空表示**宿主给了 agentPresets 而这一页没画那一段**。
	//
	// 本仓宿主不产出 agentPresets（所以正常路径永远为空）；这里保留一条会自己
	// 说话的自检：哪天宿主开始产出，界面会明说"这一段没做"，而不是静默少一段。
	PresetGap string
}

// PluginsView 是页面上的**交互态**（官方那几个 useState 的等价物）。
//
// 为什么把它作为入参传进模型：官方这些状态在组件里，本仓面板状态在
// `officialPanel`；模型层拿到它们才能算出「这一帧画哪些卡、哪张展开」，
// 于是整条分支都能被普通单测钉住（不需要开窗口）。
type PluginsView struct {
	// Query 是搜索框里当前的文本（**原始**文本，模型负责 trim + 小写）。
	Query string
	// GlobalOpen 是全局段的展开态；nil = 用户没点过（官方默认**收起**，:317）。
	GlobalOpen *bool
	// Expanded 是当前展开的卡片 rowKey；空串 = 都收起（官方是手风琴，:259）。
	Expanded string
	// Tabs 是 `settings.plugins.tab` 的台账；nil = 用产品默认台账。
	Tabs []PluginsTab
	// ActiveTab 是当前选中的 tab id；空串 = 第一项（官方 `rows[0]?.id`，:39）。
	ActiveTab string
}

// buildPluginsSection 把 `pluginInventory.list` 的结果投影成「内置插件」页。
//
// inventory == nil 表示宿主没接这条能力 -> `PluginsUnknown`，**不画任何卡片**。
func buildPluginsSection(panel *PanelSection, inventory *apiclient.PluginInventoryResult, view PluginsView) {
	if panel == nil {
		return
	}
	out := &PluginsPanel{State: PluginsUnknown}
	out.Tabs = sortedPluginsTabs(view.Tabs)
	out.ActiveTab = activePluginsTab(out.Tabs, view.ActiveTab)
	out.ShowTabBar = len(out.Tabs) > 1
	panel.Plugins = out
	panel.State = PanelUnknown

	if inventory == nil {
		// nil 有两种来源：宿主没接这条能力，或者壳还没读到（首次拉取在飞 /
		// 上一次失败后保留 nil）。壳的缓存形状区分不了这两者，所以文案**两者都说**，
		// 并给「重试」——那对两种情况都是正确的下一步。
		out.Note = "未知：还没读到 pluginInventory.list（宿主未接这条能力，或首次读取还没回来；" +
			"失败原因见状态栏）"
		out.ShowRetry = true
		panel.Note = out.Note
		return
	}

	panel.State = PanelOK
	out.State = PluginsReady
	out.ManagementAvailable = inventory.ManagementAvailable != nil && *inventory.ManagementAvailable
	// 选中的 tab 没有真实现时**不静默画空**：页面上明说这一页没做。
	if tab, ok := pluginsTabByID(out.Tabs, out.ActiveTab); ok && tab.Kind != PluginsTabInventory {
		out.TabMissing = "未实现：tab「" + tab.Label + "」这一页（官方由 ui-plugin-manager 贡献，" +
			"含安装/更新/启停/详情），本仓只做了只读清单页。"
	}

	out.HasAgentPresets = inventory.AgentPresets != nil
	if out.HasAgentPresets {
		out.PresetGap = "未实现：会话插件段（宿主给了 agentPresets，本页只做了全局段；" +
			"缺的是 preset 名称解析与 switcher，已登记在报告里）"
	}
	out.Query = view.Query
	query := strings.ToLower(strings.TrimSpace(view.Query))
	out.Searching = query != ""

	// ── 全局段（官方把 failed 条先拎出来，其余按 Loader 顺序，:295-301）──
	var failed, regular []apiclient.PluginInventoryEntry
	for _, entry := range inventory.Entries {
		if entry.Phase() == apiclient.PluginPhaseFailed {
			failed = append(failed, entry)
		} else {
			regular = append(regular, entry)
		}
	}

	// 官方：会话段默认**展开**，全局段默认**收起**；搜索时两段都强制展开
	// （:315-317）。
	globalOpen := false
	if view.GlobalOpen != nil {
		globalOpen = *view.GlobalOpen
	}
	if out.Searching {
		globalOpen = true
	}

	global := PluginGroup{
		Key:      "global",
		Title:    zhPluginsGlobalTitle,
		Subtitle: zhPluginsGlobalSubtitle,
		Open:     globalOpen,
	}
	// 官方 globalRows：failed 段在前（不合组，直接接在 regular 前面），
	// 并且两者都按同一套搜索规则过滤（:305-307）。
	for _, entry := range failed {
		if !pluginMatches(entry, query) {
			continue
		}
		global.Cards = append(global.Cards, pluginCardOf(entry, view.Expanded))
	}
	for _, entry := range regular {
		if !pluginMatches(entry, query) {
			continue
		}
		global.Cards = append(global.Cards, pluginCardOf(entry, view.Expanded))
	}
	global.Count = len(global.Cards)

	// 官方：`entries.length > 0` 才画全局段（**不是** 过滤后的条数，:550）；
	// 但段体只在 `globalCount > 0` 时才画（:565）。
	if len(inventory.Entries) > 0 {
		out.Groups = append(out.Groups, global)
	}

	// 官方：一个条目都没有、也没有 preset 面 -> `empty`（:475）。
	out.Empty = len(inventory.Entries) == 0 && len(inventory.AgentPresets) == 0
	// 官方：在搜索、且全局命中为 0、且没有别的段命中 -> `emptySearch`（:318）。
	out.EmptySearch = out.Searching && global.Count == 0

	switch {
	case out.Empty:
		out.Note = zhPluginsEmpty
	case out.EmptySearch:
		out.Note = zhPluginsEmptySearch
	case global.Count == 0:
		// 有清单但这一段一条都没有：说明条目全被过滤掉了（只有 preset 面时也会
		// 走到这里）。**不画**"暂无插件"——那不是事实。
		out.Note = ""
	}
	panel.Note = out.Note
}

// sortedPluginsTabs 按官方 order 升序排（官方 `.sort((a,b) => a.order - b.order)`，
// OFF:ui-settings-plugins/src/client/PluginsSettingsSection.tsx:58）。
//
// 空台账 = 用产品默认台账（产品里永远至少有一条；空只可能来自测试夹具）。
func sortedPluginsTabs(tabs []PluginsTab) []PluginsTab {
	if len(tabs) == 0 {
		tabs = DefaultPluginsTabs()
	}
	out := append([]PluginsTab(nil), tabs...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// pluginsTabByID 按 id 找台账里的一项。
func pluginsTabByID(tabs []PluginsTab, id string) (PluginsTab, bool) {
	for _, tab := range tabs {
		if tab.ID == id {
			return tab, true
		}
	}
	return PluginsTab{}, false
}

// activePluginsTab 取选中的 tab id：未指定 / 指定了不存在的 id -> 第一项
// （官方 `rows.find(...)?.id ?? rows[0]?.id`，同文件 :39）。
func activePluginsTab(tabs []PluginsTab, want string) string {
	if len(tabs) == 0 {
		return ""
	}
	if want != "" {
		for _, tab := range tabs {
			if tab.ID == want {
				return want
			}
		}
	}
	return tabs[0].ID
}

// pluginCardOf 把一条 inventory entry 投影成一张卡（官方 `globalRowCard`，
// OFF:ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.tsx:376-425）。
func pluginCardOf(entry apiclient.PluginInventoryEntry, expanded string) PluginCard {
	phase := entry.Phase()
	failed := phase == apiclient.PluginPhaseFailed
	title, desc := pluginTexts(entry.ModuleName, entry.Meta)

	card := PluginCard{
		RowKey:     "global:" + entry.EntryID,
		EntryID:    entry.EntryID,
		EntryChip:  pluginEntryChip(entry.EntryID, title),
		ModuleName: entry.ModuleName,
		Title:      title,
		Desc:       desc,
		Enabled:    entry.Enabled,
		Phase:      phase,
		Failed:     failed,
		Diagnostic: entry.Diagnostic,
		Expanded:   expanded != "" && expanded == "global:"+entry.EntryID,
	}
	if entry.Meta != nil && entry.Meta.Error != "" {
		card.MetaError = strings.Replace(zhPluginsMetadataError, "{error}", entry.Meta.Error, 1)
	}

	// 官方行尾：状态标签只在**偏离默认态**时画；阶段点只给"正在动"的三态画。
	switch {
	case failed:
		card.State = zhPluginsFailedTag
		card.Tag = true
	case entry.Enabled:
		card.State = zhPluginsEnabledTag
		card.Tag = false
	default:
		card.State = zhPluginsDisabledTag
		card.Tag = true
	}
	card.PhaseDot = entry.Enabled && pluginPhaseShowsDot(phase)

	// 官方详情行（`CardFacts` 的 facts）：
	//   配置状态 = 已启用/已停用；启用时再加一行 运行状态 = 阶段文案。
	card.Facts = []PluginFact{{Label: zhPluginsConfiguration, Value: pluginConfigurationText(entry.Enabled)}}
	if entry.Enabled {
		card.Facts = append(card.Facts, PluginFact{Label: zhPluginsRuntime, Value: pluginPhaseText(phase)})
	}
	return card
}

// pluginTexts 是官方 `pluginText`（:93-100）：
//
//	title       = meta.title 解析值，缺席回退到**短模块名**
//	description = meta.description 解析值，空则不画
//
// 注意"缺席"与"解析出空串"要分开：前者回退，后者官方也会回退到 undefined
// （`resolveText(...) || undefined`）。
func pluginTexts(moduleName string, meta *apiclient.PluginInventoryMeta) (title, desc string) {
	if meta != nil {
		if text := meta.Title.Pick(); text != "" {
			title = text
		}
		desc = meta.Description.Pick()
	}
	if title == "" {
		title = pluginShortName(moduleName)
	}
	return title, desc
}

// pluginShortName 是官方 `moduleShortName`（:69-76）：去 scope、再去三类前缀。
//
// 逐条照抄官方三步：
//
//	① `@scope/name` -> `name`
//	② 去 `cordis:` / `cordis-plugin-`
//	③ 去 `dsh-host-` / `dsh-client-` / `dsh-`
func pluginShortName(moduleName string) string {
	unscoped := moduleName
	if strings.HasPrefix(moduleName, "@") {
		if i := strings.Index(moduleName, "/"); i >= 0 {
			unscoped = moduleName[i+1:]
		}
	}
	unscoped = strings.TrimPrefix(unscoped, "cordis:")
	unscoped = strings.TrimPrefix(unscoped, "cordis-plugin-")
	for _, prefix := range []string{"dsh-host-", "dsh-client-", "dsh-"} {
		unscoped = strings.TrimPrefix(unscoped, prefix)
	}
	return unscoped
}

// pluginEntrySubtitle 是官方 `entrySubtitle`（:78-81）：display 时剥掉 `include:`。
func pluginEntrySubtitle(entryID string) string {
	return strings.TrimPrefix(entryID, "include:")
}

// pluginEntryChip 是官方 `idAddsToTitle`（:83-86）的落点：返回空串 = 不画角标。
func pluginEntryChip(entryID, title string) string {
	if entryID == "" {
		return ""
	}
	chip := pluginEntrySubtitle(entryID)
	if chip == title {
		return ""
	}
	return chip
}

// pluginPhaseText 是官方 `phaseLabel`（:64-67）：空串（官方 null）= `未运行`。
func pluginPhaseText(phase string) string {
	switch phase {
	case apiclient.PluginPhasePending:
		return zhPluginsPhasePending
	case apiclient.PluginPhaseLoading:
		return zhPluginsPhaseLoading
	case apiclient.PluginPhaseActive:
		return zhPluginsPhaseActive
	case apiclient.PluginPhaseFailed:
		return zhPluginsPhaseFailed
	case apiclient.PluginPhaseUnloading:
		return zhPluginsPhaseUnloading
	}
	return zhPluginsPhaseUnobserved
}

// pluginPhaseShowsDot 是官方 `showsPhaseDot`（:220-223）：只有"正在动"的三态有点。
func pluginPhaseShowsDot(phase string) bool {
	switch phase {
	case apiclient.PluginPhasePending, apiclient.PluginPhaseLoading, apiclient.PluginPhaseUnloading:
		return true
	}
	return false
}

// pluginConfigurationText 是官方配置状态文案：启用 -> 已启用，否则 已停用。
func pluginConfigurationText(enabled bool) string {
	if enabled {
		return zhPluginsEnabledTag
	}
	return zhPluginsDisabledTag
}

// pluginMatches 是官方 `matches`（:102-108）：搜索串对
// **模块名 / entry id / 标题 / 简介** 四个字段做小写包含匹配。
//
// 注意官方匹配的是**解析后的标题与简介**（`pluginText` 的输出），不是原始 meta。
func pluginMatches(entry apiclient.PluginInventoryEntry, query string) bool {
	if query == "" {
		return true
	}
	title, desc := pluginTexts(entry.ModuleName, entry.Meta)
	for _, value := range []string{entry.ModuleName, entry.EntryID, title, desc} {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}
