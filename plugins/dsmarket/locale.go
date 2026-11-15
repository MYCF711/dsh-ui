package dsmarket

// locale.go —— 详情页/版本列表用到的官方 zh 文案，**逐字**来自
// `OFF:../dsh-client-ui-plugin-market/src/client/locales.ts` 的 zh 字典。
//
// 常量名就是官方字典键（`zhVersionDefault` ← `versionDefault`），注释里的行号是
// 那份字典里的行号。官方那份 zh 字典是这个包的**键来源**（`locales.ts:3`：
// "Simplified Chinese dictionary and key source of truth"），所以一个字都不许改。
//
// 为什么本包自带而不是走 locale 服务：本仓的 locale 服务由 dssettings 提供、覆盖
// 全局语言；官方这里是插件私有命名空间（`index.ts:29` 的 `NS = 'pluginMarket'`，
// 八种语言各一份）。本仓今天只有 zh 一套界面语言（见 docs/official-parity.md 的
// `settings-general` 行：语言项还没有入口），所以按仓里既有做法（dssidebar /
// dsgoal / dsplan 都内置官方 zh 常量）把 zh 逐字搬进来；等语言切换落地时，
// 这些常量就是那个命名空间的初始字典。

const (
	// 'backToList'（locales.ts:62）——面包屑上的返回文案。
	//
	// 官方用的字就是「发现」（不是"返回"）：面包屑回到 Discover 列表。
	zhBackToList = "发现"

	// 'factSource'（locales.ts:63）——"来源"分区标题。
	zhFactSource = "来源"
	// 'factPackage'（:64）'factRegistry'（:65）'factRepository'（:66）
	// 'factCommit'（:67）'factDirectory'（:68）——五种来源事实的标签。
	zhFactPackage    = "软件包"
	zhFactRegistry   = "注册表"
	zhFactRepository = "仓库"
	zhFactCommit     = "提交"
	zhFactDirectory  = "目录"
	// 'factPublished'（:69）'factAuthor'（:70）'factHomepage'（:71）
	// 'factSourceCode'（:72）'factCategory'（:73）'factLabels'（:74）'factMarket'（:75）。
	zhFactPublished  = "发布时间"
	zhFactAuthor     = "作者"
	zhFactHomepage   = "主页"
	zhFactSourceCode = "源代码"
	zhFactCategory   = "分类"
	zhFactLabels     = "标签"
	zhFactMarket     = "插件索引"

	// 'versionsTitle'（:76）——版本分区标题；'versionsLabel'（:77）是那个列表的
	// 无障碍名（"{name} 的版本"）。本仓没有无障碍树，只留标题。
	zhVersionsTitle = "版本"

	// 'versionDefault'（:78）'versionInstalled'（:79）'versionIncompatible'（:80）
	// 'versionExempted'（:81）'versionPrerelease'（:82）——版本行的五枚标签。
	zhVersionDefault      = "推荐"
	zhVersionInstalled    = "当前已安装"
	zhVersionIncompatible = "不兼容"
	zhVersionExempted     = "已豁免"
	zhVersionPrerelease   = "预发布"

	// 'incompatible'（:59）与 'exempted'（:61）——版本行说明行与详情页提示条
	// 共用的两句话。前者带两个占位：{runtime} 与 {range}。
	zhIncompatible = "此版本与当前的 DSH {runtime} 不兼容，它要求 DSH {range}"
	zhExempted     = "此版本与当前 DSH 不兼容，但你已为它设置了兼容性豁免"

	// 'detailLoading'（:86）——详情文档读取中的那一行。
	zhDetailLoading = "正在读取详细介绍"
	// 'retry'（:8）——失败后的重试。
	zhRetry = "重试"

	// 六个详情失败档（locales.ts:87-92，顺序与官方 FAILURE_KEYS 一致）。
	zhDetailTooLarge       = "详细介绍或它所在的仓库超过大小限制，未加载"
	zhDetailNetwork        = "无法连接到详细介绍的来源"
	zhDetailDenied         = "无法访问详细介绍所在的仓库，请检查 Git 凭据"
	zhDetailGitUnavailable = "详细介绍所在的仓库需要用 Git 读取，但无法运行 Git。请安装 Git 后重试"
	zhDetailInvalid        = "插件声明的详细介绍文件无效"
	zhDetailFailed         = "无法读取详细介绍"
)

// ── Discover tab（`#30` A/B 块）的官方 zh 文案 ──────────────────────────────
//
// 全部逐字来自同一份 zh 字典（`locales.ts:5-46`），键名写在注释里。这一批是
// 工具栏 / 空态 / 索引源对话框 / 卡片与列表状态用到的那些。
const (
	// 'title'（locales.ts:5）——建设 tab 的名字，也是 `plugins.tab` 的 label
	// （`client/index.ts:55-57` 的 `label: () => t('title')`）。
	zhTabTitle = "发现"
	// 'refresh'（:6）——刷新当前索引。
	zhRefresh = "刷新"
	// 'loading'（:7）——索引读取中。
	zhLoading = "正在读取索引"

	// 'emptyTitle'（:11）'emptyDescription'（:12）'addFirstSource'（:13）——
	// 一个索引源都没有时的空态三行。
	zhEmptyTitle       = "还没有插件索引"
	zhEmptyDescription = "添加一个插件索引，即可在这里浏览和安装其中的插件"
	zhAddFirstSource   = "添加索引源"

	// 'sourcesFailed'（:14）——索引源清单读取失败的提示。
	zhSourcesFailed = "无法读取索引源"
	// 'manageSources'（:15）——索引菜单底部那一项。
	zhManageSources = "管理索引源…"
	// 'marketMenu'（:16）——索引菜单触发器的无障碍名（含 {market}）。
	zhMarketMenu = "切换插件索引，当前为 {market}"

	// 'sourcesTitle'（:17）'sourcesDescription'（:18）'sourcesLabel'（:19）——
	// 索引源对话框的标题、说明、列表名。
	zhSourcesTitle       = "索引源"
	zhSourcesDescription = "插件索引以一个索引文件的 URL 提供，「发现」从这里列出的索引浏览插件"
	zhSourcesLabel       = "已添加的索引源"
	// 'sourceUrl'（:20）'sourcePlaceholder'（:21）'addSource'（:22）——输入框与添加按钮。
	zhSourceURL         = "索引 URL"
	zhSourcePlaceholder = "https://example.com/index.json"
	zhAddSource         = "添加"
	// 'removeSource'（:23）——移除按钮（官方是一枚垃圾桶图标，本仓没有那枚 sprite，
	// 用同一句官方文案当文字按钮；已在证据文档登记为 derived）。
	zhRemoveSource = "移除"
	// 'removeSourceNamed'（:24）——同一枚按钮的可访问名（含 {url}），命中区标签用它。
	zhRemoveSourceNamed = "移除 {url}"
	// 'sourcesTrust'（:25）——对话框底部的信任提示。
	zhSourcesTrust = "索引由第三方提供。只添加你信任的来源：其中的插件安装后以你的权限运行"
	// 三档写回失败（:26-29）。'sourceDuplicate' 与 'sourceUnavailable' 宿主产不出
	// （见 internal/apiclient 的 MarketSourceFailureOf 注释），所以本包不引那两个键。
	zhSourceInvalid = "请使用 http 或 https 地址，且不要包含用户名、密码或 # 片段"
	zhSourceFailed  = "无法保存索引源"

	// 'readFailed'（:30）——索引读不到。`readInvalid`/`readChanged` 两档本仓宿主
	// 产不出（索引非法也回 index_unavailable），所以不引。
	zhReadFailed = "无法读取此索引"

	// 'search'（:33）——搜索框的占位与可访问名。
	zhSearch = "搜索插件"
	// 'filter'（:34）'filterCount'（:35）——标签过滤按钮的两态。
	zhFilter      = "标签"
	zhFilterCount = "标签 · {count}"
	// 'sortBy'（:36）——排序按钮（含 {sort}）。
	zhSortBy = "排序：{sort}"
	// 四个排序项（:37-39）。
	zhSortRelevance = "最佳匹配"
	zhSortDefault   = "默认顺序"
	zhSortPublished = "发布时间"
	// 'switchToAscending'（:40）'switchToDescending'（:41）——方向切换项。
	zhSwitchToAscending  = "改为升序"
	zhSwitchToDescending = "改为降序"

	// 'categoriesLabel'（:42）——分类胶囊行的可访问名；'allCategories'（:43）是第一个胶囊。
	zhCategoriesLabel = "分类"
	zhAllCategories   = "全部"

	// 'emptyIndex'（:44）'noMatches'（:45）——两种空列表（索引本来就空 / 过滤没命中）。
	zhEmptyIndex = "此索引没有插件"
	zhNoMatches  = "没有符合条件的插件"
	// 'showMore'（:46）——下一页。
	zhShowMore = "显示更多"

	// 'entryMeta'（:47）'entryMetaAuthor'（:48）——卡片第二行（版本 · 作者 · 日期）。
	zhEntryMeta       = "{version} · {date}"
	zhEntryMetaAuthor = "{version} · {author} · {date}"
	// 'openEntry'（:49）——卡片标题按钮的无障碍名（含 {name}）。
	zhOpenEntry = "查看 {name}"
)
