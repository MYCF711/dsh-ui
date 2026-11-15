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
