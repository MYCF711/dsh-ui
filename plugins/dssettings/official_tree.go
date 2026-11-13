// official_tree.go —— rc.2 官方设置面的**结构表**（5 个 section + 13 个 general item
// + 子 slot 种类 + 动态注册条件）。
//
// 为什么单独一张表：设置面是「注册 ledger 决定有无/顺序/key」的动态面，不是固定
// 页面。把「官方有哪些节点、什么条件下出现、挂在哪个 slot 上」写死成数据，绘制代码
// 只消费这张表——否则每加一行都要在两处改，且没有任何测试能证明「官方 13 项我漏了
// 第 7 项」。
//
// 证据口径（rc.2 解包树 `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`）：
//
//	<pkg>/src/client/index.ts:N   → slot 注册块
//	<pkg>/src/client/*.tsx:N      → 行组件
//
// 行号与结论逐条来自 `home/dsh-ui-official/structure/official-settings-tree.md`
// 的 rc.2 重锚版（§0 计数、§3 五个 section、§4.2 十三行、§13/§14 真变）。
//
// # rc.2 的四处真变（本文件承载其中两处）
//
//	D3  panel `aria-label` → **`aria-labelledby`**（titleId 指向 navTitle）→ official_modal.go
//	D5  12 个 general item → **13 个**（新增 `desktop-update-channel`，order 95）→ 本文件
//
// 另外两处（D1 CSS Grid、D4 createPortal + useModalLayer）在 official_grid.go /
// official_modal.go，因为它们是布局与模态层的实现形状，不是节点清单。
package dssettings

import "sort"

// ── 五个 settings.section ────────────────────────────────────────────────

// SectionCondition 是一个 section 的**动态注册条件**。
//
// 为什么不是布尔：官方五个注册点里只有 account 带条件（
// `snapshot.view.status === "credential-stored"`），其余四个固定注册。
// 把条件写成枚举，测试才能逐条断言「条件不满足时确实不注册」，而不是靠读代码。
type SectionCondition string

const (
	// SectionAlways 固定注册（general / models / plugins / agent-presets）。
	SectionAlways SectionCondition = "always"
	// SectionCredentialStored 仅 `snapshot.view.status === "credential-stored"`
	// 时注册；状态离开即卸载（ui-settings-account/src/client/index.ts:318-334，判据 :321）。
	SectionCredentialStored SectionCondition = "credential-stored"
)

// OfficialSection 是官方 5 个 `settings.section` 注册项之一。
type OfficialSection struct {
	// ID 是 section id（也是 `settings.section.icon` 这个 inline list 的 only 键）。
	ID string
	// Title 是左导航文案（rc.2 中文 locale）。
	Title string
	// Order 是 `settings.section` list 的排序值（官方 -10/0/10/15/20）。
	Order int
	// RightPage 是右列一级组件名（结构树 §3 的「右页一级节点」）。
	RightPage string
	// Condition 决定这一页是否注册。
	Condition SectionCondition
	// Anchor 是 rc.2 注册点，`<pkg>/src/...:line`。
	Anchor string
}

// officialSections 是**官方注册点的全量**（含条件页），按 order 升序。
//
// 注意 `archived-sessions` 不在这里：官方 rc.2 源码里**没有任何包注册该 section**，
// 它只是根组件 `navIcon` 的一个图标分支（ui-settings-general/src/client/
// SettingsRoot.tsx:40）。把它当第六页是 v0 台账的错误，已由 §0 第 1 条纠正。
var officialSections = []OfficialSection{
	{
		ID: "account", Title: "账号与余额", Order: -10, RightPage: "AccountSection",
		Condition: SectionCredentialStored,
		Anchor:    "ui-settings-account/src/client/index.ts:318-334",
	},
	{
		ID: "general", Title: "通用", Order: 0, RightPage: "GeneralSection",
		Condition: SectionAlways,
		Anchor:    "ui-settings-general/src/client/index.ts:266-273",
	},
	{
		ID: "models", Title: "模型", Order: 10, RightPage: "ModelsSection",
		Condition: SectionAlways,
		Anchor:    "ui-settings-models/src/client/index.ts:136-146",
	},
	{
		ID: "plugins", Title: "内置插件", Order: 15, RightPage: "PluginsSettingsSection",
		Condition: SectionAlways,
		Anchor:    "ui-settings-plugins/src/client/index.ts:76-84",
	},
	{
		ID: "agent-presets", Title: "Agent 预设", Order: 20, RightPage: "AgentPresetSection",
		Condition: SectionAlways,
		Anchor:    "ui-agent-preset/src/client/index.ts:283-290",
	},
}

// ── 十三个 settings.general.item ────────────────────────────────────────

// ItemKind 是一行 general item 的**控件形状**（官方行组件不同，几何也不同）。
type ItemKind string

const (
	// ItemSelector 标题 + 选择器 pill（permission / language / link-opening /
	// composer-enter / performance-usage / transcript-view 三行 / desktop-update-channel）。
	ItemSelector ItemKind = "selector"
	// ItemSwitch 右侧 Switch（developer-tools / session-log-upload-toast）。
	ItemSwitch ItemKind = "switch"
	// ItemStepper 字号 stepper（font-size）。
	ItemStepper ItemKind = "stepper"
	// ItemCubes 三张主题 cube（appearance）。
	ItemCubes ItemKind = "cubes"
	// ItemButton 右侧按钮（shortcuts 的「编辑快捷键」）。
	ItemButton ItemKind = "button"
	// ItemVersion 只读版本行（current-version）。
	ItemVersion ItemKind = "version"
)

// ItemCondition 是一行 general item 的**动态注册条件**。
//
// 官方每个 item 都是由某个包在自己的 `ctx.inject(...)` 里注册的；条件不满足时
// **整行不出现**（不是画成灰的）。这也正是「宿主没给字段就整段隐藏」的落点。
type ItemCondition string

const (
	// ItemAlways 无条件（permission / appearance / font-size / transcript-view 前两行 /
	// developer-tools / shortcuts / composer-enter / performance-usage）。
	ItemAlways ItemCondition = "always"
	// ItemPermissionCatalog 依赖 `permissionPresets.catalog`（权限目录）。
	ItemPermissionCatalog ItemCondition = "permission-catalog"
	// ItemSidebarRightTabs 依赖 `sidebarRightTabs` 服务（link-opening）。
	ItemSidebarRightTabs ItemCondition = "sidebar-right-tabs"
	// ItemSessionLogServed 依赖 `session-log-deepseek` namespace 被宿主服务。
	ItemSessionLogServed ItemCondition = "session-log-served"
	// ItemVersionInjected 依赖 build 注入的 `DSH_CLIENT_VERSION`（current-version；
	// 缺失时**整行不渲染**，官方 CurrentVersionRow.tsx:10-14）。
	ItemVersionInjected ItemCondition = "client-version"
	// ItemDesktopChannel 依赖官方 Electron 桌面桥：
	// `dshDesktop.protocolVersion === 1` **且** 桥里带 `updates.channels`
	// （ui-settings-general/src/client/index.ts:109-122）。
	//
	// Go GUI 没有该桥，所以本行在 Go GUI 上**不出现**——不要自造。
	ItemDesktopChannel ItemCondition = "desktop-update-channel"
)

// Choice 是选择器的一个候选（value 是写给宿主的原值，label 是界面文案）。
type Choice struct {
	Value string
	Label string
	Desc  string
}

// Bounds 是数值行的官方边界（font-size 这类）。
type Bounds struct {
	HasMin bool
	HasMax bool
	Min    float64
	Max    float64
	Step   float64
}

// OfficialGeneralItem 是官方 13 个 `settings.general.item` 之一。
//
// Rows 是这个 item 贡献的**可见行数**：官方 13 个 slot 里 `transcript-view` 一个
// slot 渲染三行（工作步骤展示 / 工作步骤收起时机 / 收起后显示的回复），其余各一行。
// 所以「默认 14 行」= 12 个单行 slot + transcript-view 的 3 行 − Go GUI 没有的
// desktop-update-channel 1 行（见 HostFacts.VisibleRowCount）。
type OfficialGeneralItem struct {
	ID        string
	Order     int
	Title     string
	Desc      string
	Kind      ItemKind
	HostField string
	Rows      int
	Condition ItemCondition
	Anchor    string
	RowAnchor string
	Choices   []Choice
	Default   string
	Bounds    Bounds
	// ToggleLabel 是 Switch 行「开」时的显示文案（官方 Switch 不显示文字，
	// 这里给报告/测试用，绘制仍只画开关）。
	ToggleLabel string
}

// officialGeneralItems 是**注册点全量**（13 条），按官方 order 升序。
//
// 排序值来自各包注册块；`transcript-view` 的三行共享 order 12（一个 slot 三个
// PreferenceRow），这里用一行 + Rows=3 表达。
var officialGeneralItems = []OfficialGeneralItem{
	{
		ID: "permission", Order: -20, Title: "权限",
		Desc: "选择新会话的默认权限模式",
		Kind: ItemSelector, HostField: "permission.defaultPreset", Rows: 1,
		Condition: ItemPermissionCatalog,
		Anchor:    "ui-permission-presets/src/client/index.ts:169-175",
		RowAnchor: "ui-permission-presets/src/client/PermissionRow.tsx:1-135",
		Choices: []Choice{
			{Value: "deny", Label: "只读", Desc: "写类工具一律拒绝"},
			{Value: "ask", Label: "每次都问我", Desc: "写类工具每次都要点允许"},
			{Value: "allow", Label: "自动允许", Desc: "写类工具不问直接跑"},
			{Value: "auto", Label: "按模式自动", Desc: "由当前会话模式决定"},
		},
		// 空 id = unset（恢复宿主默认）；默认值由 catalog.defaultPreset 给，
		// 界面不编。
		Default: "",
	},
	{
		ID: "language", Order: 0, Title: "语言",
		Desc: "界面语言",
		// 官方字段是 `locale.preference`（`locale-settings.ts:9` LOCALE_PREFERENCE_FIELD），
		// 不是 `locale.active`——写错的话宿主发了也读不到，永远显示未知。
		Kind: ItemSelector, HostField: "locale.preference", Rows: 1,
		Condition: ItemAlways,
		Anchor:    "dsh-client-locale/src/client/index.ts:622-629",
		RowAnchor: "dsh-client-locale/src/client/LanguageRow.tsx:30-67",
		Choices: []Choice{
			{Value: "en", Label: "English"},
			{Value: "zh", Label: "简体中文"},
			{Value: "ja", Label: "日本語"},
			{Value: "es", Label: "Español"},
			{Value: "fr", Label: "Français"},
			{Value: "de", Label: "Deutsch"},
			{Value: "ru", Label: "Русский"},
			{Value: "pt", Label: "Português"},
		},
	},
	{
		ID: "appearance", Order: 10, Title: "外观",
		Desc: "",
		Kind: ItemCubes, HostField: "ui-theme.preference", Rows: 1,
		Condition: ItemAlways,
		Anchor:    "ui-theme/src/client/index.ts:630-637",
		RowAnchor: "ui-theme/src/client/AppearanceRow.tsx:45-63",
		Choices: []Choice{
			{Value: "light", Label: "浅色"},
			{Value: "dark", Label: "深色"},
			{Value: "system", Label: "跟随系统"},
		},
		Default: "system",
	},
	{
		ID: "font-size", Order: 11, Title: "字号大小",
		Desc: "",
		Kind: ItemStepper, HostField: "ui-theme.fontSize", Rows: 1,
		Condition: ItemAlways,
		Anchor:    "ui-theme/src/client/index.ts:639-653",
		RowAnchor: "ui-theme/src/client/FontSizeRow.tsx:49-99",
		Default:   "14",
		Bounds:    Bounds{HasMin: true, HasMax: true, Min: 10, Max: 22, Step: 1},
	},
	{
		ID: "transcript-view", Order: 12, Title: "工作步骤展示",
		Desc: "",
		Kind: ItemSelector, HostField: "ui-chat.transcriptView", Rows: 3,
		Condition: ItemAlways,
		Anchor:    "ui-chat/src/client/apply.ts:194-209",
		RowAnchor: "ui-chat/src/client/settings/TranscriptViewRow.tsx:57-93",
		Choices: []Choice{
			{Value: "compact", Label: "紧凑"},
			{Value: "standard", Label: "标准"},
			{Value: "detailed", Label: "详细"},
			{Value: "verbose", Label: "完整"},
		},
		// 桌面默认 standard，Web 默认 detailed（ui-chat/src/client/apply.ts:172）。
		Default: "standard",
	},
	{
		ID: "developer-tools", Order: 15, Title: "开发者模式",
		Desc: "",
		Kind: ItemSwitch, HostField: "ui-settings.enabled", Rows: 1,
		Condition: ItemAlways,
		Anchor:    "ui-settings-general/src/client/index.ts:85-99",
		RowAnchor: "ui-settings-general/src/client/DeveloperToolsRow.tsx:19-37",
		Default:   "true",
	},
	{
		ID: "shortcuts", Order: 16, Title: "快捷键",
		// 官方 zh 字典（ui-shortcuts/src/client/locales.ts:30）：
		// settings='快捷键' / view='编辑快捷键' / description='查看和编辑当前可用的快捷键和输入操作'。
		Desc: "查看和编辑当前可用的快捷键和输入操作",
		Kind: ItemButton, HostField: "shortcuts.catalog", Rows: 1,
		Condition: ItemAlways,
		Anchor:    "ui-shortcuts/src/client/index.ts:42-45",
		RowAnchor: "ui-shortcuts/src/client/Reference.tsx:54-71",
	},
	{
		ID: "link-opening", Order: 17, Title: "网页链接默认打开方式",
		Desc: "",
		Kind: ItemSelector, HostField: "ui-chat.linkOpening", Rows: 1,
		Condition: ItemSidebarRightTabs,
		Anchor:    "ui-chat/src/client/apply.ts:150-171",
		RowAnchor: "ui-chat/src/client/settings/LinkOpeningRow.tsx:30-47",
		Choices: []Choice{
			{Value: "sidebar", Label: "应用内侧边栏"},
			{Value: "new-tab", Label: "默认浏览器"},
		},
		Default: "sidebar",
	},
	{
		ID: "composer-enter", Order: 20, Title: "繁忙时的发送行为",
		Desc: "",
		Kind: ItemSelector, HostField: "ui-conversation.busyEnter", Rows: 1,
		Condition: ItemAlways,
		Anchor:    "ui-conversation/src/client/apply.ts:199-208",
		RowAnchor: "ui-conversation/src/client/settings/EnterBehaviorRow.tsx:39-76",
		Choices: []Choice{
			{Value: "queue", Label: "排队"},
			{Value: "steer", Label: "插入"},
		},
		// rc.2 实测默认 queue（rc.1 台账写 steer 是错的，见结构树 §14.2 V1）。
		Default: "queue",
	},
	{
		ID: "performance-usage", Order: 30, Title: "性能与用量",
		Desc: "",
		Kind: ItemSelector, HostField: "ui-chat.performanceUsage", Rows: 1,
		Condition: ItemAlways,
		Anchor:    "ui-chat/src/client/apply.ts:183-192",
		RowAnchor: "ui-chat/src/client/settings/PerformanceUsageRow.tsx:31-45",
		Choices: []Choice{
			{Value: "compact", Label: "紧凑"},
			{Value: "detailed", Label: "详细"},
		},
		Default: "detailed",
	},
	{
		ID: "session-log-upload-toast", Order: 90, Title: "在使用官方模型 API 时上传 Session Log",
		Desc: "",
		Kind: ItemSwitch, HostField: "session-log-deepseek.enabled", Rows: 1,
		Condition: ItemSessionLogServed,
		Anchor:    "ui-settings-session-log/src/client/index.ts:25-37",
		RowAnchor: "ui-settings-session-log/src/client/UploadRow.tsx:26-38",
	},
	{
		// rc.2 新增（D5）。
		//
		// **两个官方细节原先是错的**（2026-11-13 对着 DesktopChannelRow.tsx 复核）：
		//   1. 选项**顺序**：官方 `items={[{id:'stable'},{id:'nightly'}]}`（:34）——
		//      正式版在前。本行原先是 nightly 在前。
		//   2. 文案：官方 zh 字典是「正式版 / 开发版」（locales.ts:7-8），
		//      不是 "Stable" / "Nightly"。
		//
		// 条件也不再写死"Go GUI 没有桌面桥"：宿主现在提供**同形**的壳服务
		// （DSH-GO 的 `desktopUpdate.channel.*`），`ShellChannelServed` 为真时本行
		// 应当出现。宿主没接线时它仍不出现（条件的另一半）。
		ID: "desktop-update-channel", Order: 95, Title: "更新渠道",
		Desc: "正式版适合日常使用；开发版可提前体验新功能，但可能不稳定。",
		Kind: ItemSelector, HostField: "dshDesktop.updates.channels", Rows: 1,
		Condition: ItemDesktopChannel,
		Anchor:    "ui-settings-general/src/client/index.ts:109-122",
		RowAnchor: "ui-settings-general/src/client/DesktopChannelRow.tsx:26-45",
		Choices: []Choice{
			{Value: "stable", Label: "正式版"},
			{Value: "nightly", Label: "开发版"},
		},
	},
	{
		ID: "current-version", Order: 100, Title: "当前版本",
		Desc: "",
		Kind: ItemVersion, HostField: "modules.entries.currentVersion", Rows: 1,
		Condition: ItemVersionInjected,
		Anchor:    "ui-settings-general/src/client/index.ts:101-103",
		RowAnchor: "ui-settings-general/src/client/CurrentVersionRow.tsx:10-14",
	},
}

// transcriptViewRowTitles 是 transcript-view 这个 slot 内部三行的标题。
//
// 官方一个 slot 三个 `PreferenceRow`（TranscriptViewRow.tsx:57-93），第三行
// 「收起后显示的回复」只在 `mode !== "verbose"` 时渲染（:81-90）。
var transcriptViewRowTitles = [3]string{
	"工作步骤展示",
	"工作步骤收起时机",
	"收起后显示的回复",
}

// transcriptViewRowFields 是这三行各自的 host 字段。
var transcriptViewRowFields = [3]string{
	"ui-chat.transcriptView",
	"ui-chat.collapseTiming",
	"ui-chat.foldedReplies",
}

// transcriptViewRowChoices 是这三行的候选。
var transcriptViewRowChoices = [3][]Choice{
	{
		{Value: "compact", Label: "紧凑"},
		{Value: "standard", Label: "标准"},
		{Value: "detailed", Label: "详细"},
		{Value: "verbose", Label: "完整"},
	},
	{
		{Value: "completion", Label: "回答结束后"},
		{Value: "next-input", Label: "下次有新消息时"},
	},
	{
		{Value: "all", Label: "全部回复"},
		{Value: "final", Label: "仅最终回答"},
	},
}

// transcriptViewRowDefaults 是三行的官方默认值。
//
//   - transcriptView：桌面 standard（apply.ts:172 按桌面覆盖），Web detailed；
//   - collapseTiming：completion（chat-settings.ts:12）；
//   - foldedReplies：all（chat-settings.ts 同段）。
var transcriptViewRowDefaults = [3]string{"standard", "completion", "all"}

// ── 宿主事实（决定「注册与否」）────────────────────────────────────────

// HostFacts 是这一帧宿主能提供的事实，决定动态注册。
//
// 零值 = 什么都提供不了（老宿主 / 未接线）：那时所有条件项都不注册，界面只画
// 固定注册那几行。**不许把零值当"默认开"**——那会画出宿主根本没提供的项。
type HostFacts struct {
	// DesktopProtocolVersion 是 `dshDesktop.protocolVersion`；0 = 没有桌面桥。
	DesktopProtocolVersion int
	// UpdatesChannels 是 `dshDesktop.updates.channels`；空 = 桥没给通道。
	UpdatesChannels []string
	// ShellChannelServed 报告**宿主**的壳更新渠道服务能不能应答
	// （`desktopUpdate.channel.get` 返回 200）。
	//
	// 它与 DesktopProtocolVersion 是**两个来源**：前者是 Electron preload 桥，
	// 后者是本仓宿主提供的同形服务。都不为真时本行不出现。
	ShellChannelServed bool
	// ShellChannel 是宿主报的当前渠道；nil = 用户还没选过（官方 `channel: null`）。
	ShellChannel *string
	// ShellChannelFailure 是宿主报的壳级故障（read/format/channel/cleanup），空 = 正常。
	ShellChannelFailure string
	// ShortcutCount 是**本窗口**真实生效的键位数（GUI 自己的键位表条目数）。
	//
	// 官方 `shortcuts` 是**窗口本地**服务（`dsh-client-shortcuts/src/client/index.ts:31`
	// 的 `ctx.shortcuts.catalog`），键位来自 `DesktopShortcutsApi` 适配器，
	// **不是 Host 设置**——所以它没有对应的 settings 命名空间，也不该去问宿主。
	// 这一行的真值来源就是 GUI 自己的键位表（那张表有守卫卡着覆盖所有 HandleKey）。
	ShortcutCount int
	// SidebarRightTabs 表示 `sidebarRightTabs` 服务存在（link-opening 的前置）。
	SidebarRightTabs bool
	// SessionLogServed 表示 `session-log-deepseek` namespace 被服务。
	SessionLogServed bool
	// ClientVersion 是 build 注入的 `DSH_CLIENT_VERSION`；空 = 没注入（整行隐藏）。
	ClientVersion string
	// PermissionCatalog 表示 `permissionPresets.catalog` 已就绪。
	PermissionCatalog bool
	// TranscriptView 是 `ui-chat.transcriptView` 的当前值；空 = 未知。
	// 只有 `verbose` 会让第三行消失。
	TranscriptView string
	// CredentialStored 表示 `snapshot.view.status === "credential-stored"`。
	CredentialStored bool
}

// DesktopChannel 报告官方 Electron 桌面通道桥是否可用（rc.2 条件项的唯一判据）。
//
// 官方两个条件**同时**成立才注册：`protocolVersion === 1` 且 `updates.channels`
// 非空（`ui-settings-general/src/client/index.ts:109-122`）。
func (f HostFacts) DesktopChannel() bool {
	// 官方条件：Electron 桥在（`dshDesktop.protocolVersion === 1`）且桥给了通道。
	if f.DesktopProtocolVersion == 1 && len(f.UpdatesChannels) > 0 {
		return true
	}
	// 本仓等价物：宿主提供了**同形**的壳服务（`desktopUpdate.channel.get` 能应答）。
	//
	// 为什么不给 `DesktopProtocolVersion` 直接赋 1：那个字段名专指 Electron 的
	// preload 协议号，本仓没有 Electron 主进程。塞 1 进去等于**冒名**。
	// 所以另起一个语义明确的事实位，两个来源任一为真都能让本行出现。
	return f.ShellChannelServed
}

// SectionVisible 报告某个 section 条件是否满足。
func (f HostFacts) SectionVisible(c SectionCondition) bool {
	switch c {
	case SectionAlways:
		return true
	case SectionCredentialStored:
		return f.CredentialStored
	default:
		// 未知条件按「不注册」处理：宁可少画一页，也不画一个宿主没注册的页面。
		return false
	}
}

// ItemVisible 报告某个 general item 条件是否满足。
func (f HostFacts) ItemVisible(c ItemCondition) bool {
	switch c {
	case ItemAlways:
		return true
	case ItemPermissionCatalog:
		return f.PermissionCatalog
	case ItemSidebarRightTabs:
		return f.SidebarRightTabs
	case ItemSessionLogServed:
		return f.SessionLogServed
	case ItemVersionInjected:
		return f.ClientVersion != ""
	case ItemDesktopChannel:
		return f.DesktopChannel()
	default:
		return false
	}
}

// Sections 返回**这一帧注册的** section，按 order 升序。
func (f HostFacts) Sections() []OfficialSection {
	out := make([]OfficialSection, 0, len(officialSections))
	for _, s := range officialSections {
		if f.SectionVisible(s.Condition) {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}

// GeneralItems 返回**这一帧注册的** general item，按 order 升序。
func (f HostFacts) GeneralItems() []OfficialGeneralItem {
	out := make([]OfficialGeneralItem, 0, len(officialGeneralItems))
	for _, it := range officialGeneralItems {
		if f.ItemVisible(it.Condition) {
			out = append(out, it)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}

// ItemRows 返回一个 item 在这一帧贡献的可见行数。
//
// 只有 transcript-view 会变：`verbose` 时第三行「收起后显示的回复」不渲染
// （官方 TranscriptViewRow.tsx:81-90 的 `mode !== 'verbose'` 条件）。
func (f HostFacts) ItemRows(it OfficialGeneralItem) int {
	if it.ID != "transcript-view" {
		return it.Rows
	}
	if f.TranscriptView == "verbose" {
		return 2
	}
	return 3
}

// VisibleRowCount 是 general 页这一帧的**可见行数**（官方台账的判据）。
//
// 官方口径（结构树 §4 开头）：
//
//	默认桌面 / 浏览器（无 Electron 通道桥、字体项未展开、transcriptView != verbose）→ 14 行
//	transcriptView == verbose                                                   → 13 行
//	官方 Electron 桌面且 protocolVersion=1 且有 updates.channels                 → 15 行
func (f HostFacts) VisibleRowCount() int {
	n := 0
	for _, it := range f.GeneralItems() {
		n += f.ItemRows(it)
	}
	return n
}

// ── section 图标（动态 slot + fallback）────────────────────────────────

// 图标名是官方 `*OutlineMedium` 组件名的短名（不带 `Icon` 前缀），绘制层按短名
// 取字形。这里只给**名字**，不编素材路径（结构树 §1.1：不得硬编码图标路径）。
const (
	IconUser            = "user"
	IconData            = "data"
	IconAgentPreset     = "agent-preset"
	IconPersonalization = "personalization"
	IconArchive         = "archive"
	IconSettings        = "settings"
	IconClose           = "close"
)

// navIcon 是官方的 fallback 图标表（SettingsRoot.tsx:35-42 的 `navIcon(id)`）。
//
// 未列出的 id（含 `general` 与任何第三方 section）一律回退到 settings 齿轮。
func navIcon(id string) string {
	switch id {
	case "account":
		return IconUser
	case "models":
		return IconData
	case "agent-presets":
		return IconAgentPreset
	case "plugins":
		return IconPersonalization
	case "archived-sessions":
		return IconArchive
	default:
		return IconSettings
	}
}

// SectionIcon 解析一个 section 的图标。
//
// 官方语义（结构树 §1.1）：section 图标是**动态 slot**——`settings.section.icon`
// 是 `kind:"list", scope:"root", inline:true` 的 inline list，渲染时按
// `only: row.id` 取注册视图、以 `navIcon(row.id)` 作 fallback。第三方注册同名
// slot（id = section id）即可替换；未注册时必须回退，**不能画死一个固定图标**。
func SectionIcon(sectionID string, registered map[string]string) string {
	if name, ok := registered[sectionID]; ok && name != "" {
		return name
	}
	return navIcon(sectionID)
}

// iconGlyphs 把官方图标短名映射到**可绘制的字形**。
//
// 为什么是字形不是矢量图标：与 dscore 的 primitives 同一处理（那边用 `+ ↑ ×`
// 画 28px 圆形按钮）——为几个 16px 图标引一套路径构造不划算，这个尺寸下字形与
// 图标看不出差别。官方组件名（`IconUserOutlineMedium` 等）仍是 slot 的键，
// 这里只是**渲染器**把它翻成一个能画的字符。
var iconGlyphs = map[string]string{
	IconUser:            "◉",
	IconData:            "▤",
	IconAgentPreset:     "◈",
	IconPersonalization: "◆",
	IconArchive:         "▣",
	IconSettings:        "⚙",
	IconClose:           "×",
}

// IconGlyph 把图标短名翻成字形；未知名字回退到设置齿轮（与 navIcon 同一 fallback）。
func IconGlyph(name string) string {
	if glyph, ok := iconGlyphs[name]; ok {
		return glyph
	}
	return iconGlyphs[IconSettings]
}

// TriggerIcon 是设置启动器（`settings.trigger`）的图标。
func TriggerIcon() string { return IconSettings }

// ── 子 slot 种类（官方 ledger 的 kind）─────────────────────────────────

// SlotKind 是官方 slot 的注册种类。
type SlotKind string

const (
	SlotSingle     SlotKind = "single"
	SlotList       SlotKind = "list"
	SlotKeyed      SlotKind = "keyed"
	SlotInlineList SlotKind = "inline-list"
)

// OfficialSubSlot 是设置面声明/消费的一个子 slot。
type OfficialSubSlot struct {
	// Name 是 slot 名（官方字符串）。
	Name string
	// Kind 是注册种类。
	Kind SlotKind
	// Parent 是父槽（`settings.launcher.notifications` 这类两级名）。
	Parent string
	// DeclaredBy 是声明方（提供 slot 的包）。
	DeclaredBy string
	// ConsumedBy 是消费方（渲染该 slot 的组件）。
	ConsumedBy string
	// Anchor 是 rc.2 锚点。
	Anchor string
}

// officialSubSlots 是设置面全部子 slot（结构树 §2.1 的 ledger 清单）。
var officialSubSlots = []OfficialSubSlot{
	{Name: "settings.launcher", Kind: SlotSingle, DeclaredBy: "ui-settings-general", ConsumedBy: "SettingsRoot.triggerRow", Anchor: "ui-settings-general/src/client/index.ts:236-245"},
	{Name: "settings.trigger", Kind: SlotSingle, DeclaredBy: "ui-settings-general", ConsumedBy: "SettingsRoot.trigger", Anchor: "ui-settings-general/src/client/index.ts:236-245"},
	{Name: "settings.header", Kind: SlotSingle, DeclaredBy: "ui-settings-general", ConsumedBy: "SettingsRoot.header", Anchor: "ui-settings-general/src/client/index.ts:236-245"},
	{Name: "settings.close", Kind: SlotSingle, DeclaredBy: "ui-settings-general", ConsumedBy: "SettingsRoot.close", Anchor: "ui-settings-general/src/client/index.ts:236-245"},
	{Name: "settings.action", Kind: SlotList, DeclaredBy: "ui-settings-general", ConsumedBy: "SettingsRoot.header.actions", Anchor: "ui-settings-general/src/client/index.ts:255-263"},
	{Name: "settings.section", Kind: SlotList, DeclaredBy: "ui-settings-general", ConsumedBy: "SettingsRoot.options", Anchor: "ui-settings-general/src/client/index.ts:161-187"},
	{Name: "settings.section.icon", Kind: SlotInlineList, Parent: "settings.section", DeclaredBy: "ui-settings-general", ConsumedBy: "SettingsRoot.navIcon", Anchor: "ui-settings-general/src/client/index.ts:243"},
	{Name: "settings.onboarding", Kind: SlotList, DeclaredBy: "ui-settings-models", ConsumedBy: "SettingsRoot.onboarding", Anchor: "ui-settings-models/src/client/index.ts:147-159"},
	{Name: "settings.general.item", Kind: SlotList, Parent: "settings.section", DeclaredBy: "ui-settings-general", ConsumedBy: "GeneralSection", Anchor: "ui-settings-general/src/client/index.ts:85-103"},
	{Name: "settings.plugins.tab", Kind: SlotList, Parent: "settings.section", DeclaredBy: "ui-settings-plugins", ConsumedBy: "PluginsSettingsSection", Anchor: "ui-settings-plugins/src/client/index.ts:76-84"},
	{Name: "settings.models.provider-card", Kind: SlotKeyed, Parent: "settings.section", DeclaredBy: "ui-settings-models", ConsumedBy: "ModelsSection.rowCard", Anchor: "ui-settings-models/src/client/index.ts:142-145"},
	{Name: "settings.models.footer", Kind: SlotList, Parent: "settings.section", DeclaredBy: "ui-settings-models", ConsumedBy: "ModelsSection.footer", Anchor: "ui-settings-models/src/client/index.ts:142-145"},
	{Name: "settings.models.sign-in", Kind: SlotSingle, Parent: "settings.onboarding", DeclaredBy: "ui-settings-models", ConsumedBy: "AccountOnboarding", Anchor: "ui-settings-models/src/client/index.ts:147-159"},
	{Name: "settings.launcher.notifications", Kind: SlotList, Parent: "settings.launcher", DeclaredBy: "ui-settings-account", ConsumedBy: "SettingsRoot.launcher", Anchor: "ui-settings-account/src/client/index.ts:294-296"},
}

// SubSlots 返回设置面子 slot 清单的副本（测试与报告用）。
func SubSlots() []OfficialSubSlot {
	out := make([]OfficialSubSlot, len(officialSubSlots))
	copy(out, officialSubSlots)
	return out
}

// LookupSubSlot 按名字查一个子 slot。
func LookupSubSlot(name string) (OfficialSubSlot, bool) {
	for _, s := range officialSubSlots {
		if s.Name == name {
			return s, true
		}
	}
	return OfficialSubSlot{}, false
}

// ActiveSectionDefault 是「打开时默认选哪一页」：官方取 ledger 第一项
// （SettingsRoot.tsx:60 `rows[0]?.id`）。
func ActiveSectionDefault(sections []OfficialSection) string {
	if len(sections) == 0 {
		return ""
	}
	return sections[0].ID
}
