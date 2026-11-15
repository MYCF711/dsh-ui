// Package dsmarket 是官方 `@deepseek-ai/dsh-client-ui-plugin-market` 的**条目详情页**
// （rc.2 的 `#30` C 块：详情 markdown + 版本列表）。
//
// # 官方挂在哪里（逐字）
//
//	OFF:../dsh-client-ui-plugin-market/src/client/index.ts:55-57
//	     ctx.slots.register({name:'plugins.tab', id:'discover', …}, DiscoverSection)
//	OFF:../dsh-client-ui-plugin-market/src/client/DiscoverSection.tsx:265-355
//	     `EntryDetail`：面包屑 / 头（标题 + 版本 Tag）/ 说明 / 兼容性提示 /
//	     详情文档（`PluginDetailSection`）/ 版本列表 / 来源 facts
//	OFF:../dsh-client-ui-plugin-market/src/client/PluginDetailSection.tsx:25-47
//	     详情文档本体：读 `pluginMarket.readEntryDetail`，把 markdown 交给官方
//	     `DetailDocument` 原语渲染
//
// # 数据 → 容器 → 渲染
//
//	pluginMarket.readEntryDetail（本仓真实现，形状 ✅）
//	  → internal/apiclient.PluginMarketReadEntryDetail（唯一新增的宿主方法）
//	  → 本包 Page.Draw：面包屑 / 头 / 说明 / 提示 / 文档 / 版本 / facts
//
// 版本列表**不是**另一个宿主方法：官方 `EntryDetail` 的 `entry.versions` 来自
// `pluginMarket.search` 的 `entries[]`（`OFF:…/market-store.ts:318,381`），
// 由打开详情页的那一层把它交进来（本仓那块是 A/B 两块的活，见文件尾的座位说明）。
//
// # 为什么这里只做 C 块（详情 + 版本列表）
//
// `#30` 整条拆成三块，A（tab 骨架 / 索引源增删）与 B（搜索 + 分类/标签/排序工具栏）
// 都卡在宿主 `pluginMarket.listIndexes` / `pluginMarket.search` 的形状上；
// 只有详情这一条宿主形状**完全对得上**（`HOST:cmd/dshd/serve_plugin_market.go:234-275`），
// 所以先把它做实，A/B 落地时只需要把 entry 交进来。
//
// # 缺真实能力的三处，按仓规 HIDE（不画假入口、不给假图）
//
//  1. **安装 / 更新按钮不画**：官方 `EntryAction`（`DiscoverSection.tsx:102-135`）走
//     `pluginManager.installBundle` / `updateBundle`，本仓这两个方法**结构化 501**
//     （`HOST:internal/transport/handlers_pluginmanager_official.go:167-168,279-281`：
//     宿主没有 bundle 下载源，也没有安装事务）。画一个注定失败的按钮比不画更糟。
//  2. **相对图片不给假图**：官方文档里的相对图片走
//     `GET /api/plugin-market/detail-file?document=&path=`（`OFF:…/market-store.ts:147,230-234`），
//     本仓宿主没有这条路由（`grep -rn 'detail-file' cmd/ internal/` = 0 命中）。
//  3. **分类 / 标签 / 索引名三行按数据缺失不画**：它们要官方
//     `listing.metadata.categories|labels` 的**声明标题**，而本仓索引文档不解析
//     `metadata.*`（B 块的宿主缺口）。Entry 里留了字段，数据一到就画。
package dsmarket

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// InstallStatus 是官方 `MarketListingEntry.installation.status`
// （`OFF:../dsh-plugin-market/lib/typert.host.js` 的 search 结果 schema；
// 宿主侧对应 `HOST:cmd/dshd/serve_plugin_market.go:121,131-137`）。
const (
	InstallStatusNotInstalled     = "not-installed"
	InstallStatusInstalled        = "installed"
	InstallStatusDifferentVersion = "different-version"
	InstallStatusUpdateAvailable  = "update-available"
	InstallStatusProvided         = "provided"
)

// SourceKind 是官方 `MarketListingVersion.source.type`（'npm' | 'git'）。
const (
	SourceNPM = "npm"
	SourceGit = "git"
)

// Source 是官方 `MarketListingVersion.source`。
//
// npm 用 Registry（可空：没声明就用默认 registry）；git 用 URL + Commit（完整 commit）
// 与可空的 Path（仓库内的包目录）。这与 `OFF:…/presentation.ts:91-104` 的
// sourceFacts 一一对应。
type Source struct {
	Kind     string
	URL      string
	Commit   string
	Path     string
	Registry string
}

// Compatibility 是官方 `MarketCompatibility`。
//
// Compatible=false 时 Requirements 是"这一版要求的 DSH 版本范围"，界面按官方
// `incompatible` 文案把它拼进一行提示。
type Compatibility struct {
	Compatible     bool
	Exempted       bool
	RuntimeVersion string
	Requirements   map[string]string
}

// Version 是官方 `MarketListingVersion`。
//
// PublishTimestamp 是**秒**（官方 `publishTimestamp`，`OFF:…/DiscoverSection.tsx:90-92`
// 的 formatDate 就按秒 × 1000 用）。本仓宿主当前把同一事实叫 `publishedAt`
// （字符串、`HOST:internal/transport/handlers_plugin_market_official.go:42-54`）；
// B 块把 search 形状改成官方 `entries[]` 时由那一层换算，这里只认官方字段。
type Version struct {
	Version          string
	PublishTimestamp int64
	Source           Source
	Compatibility    Compatibility
	// Update 是官方 0.2.2-rc.2 新增的 `update: boolean`（已装副本可从这一版升级）。
	//
	// 本包的详情页**不画**升级按钮（安装/更新 501，见包注释），所以它只用来
	// 让版本行的状态与官方同源；将来 A/B 打开这条路径时不必再算一遍。
	Update bool
}

// Installation 是官方 `MarketListingEntry.installation`。
type Installation struct {
	Status           string
	InstalledVersion string
}

// Repository 是官方 `MarketListingEntry.repository`。
type Repository struct {
	URL       string
	Directory string
}

// Entry 是详情页要的一条市场条目，字段名与官方 `MarketListingEntry` 对齐。
//
// 三个"声明标题"（MarketTitle / CategoryTitle / LabelTitles）在官方来自
// `MarketSearchResult` 的 `title` 与 `metadata.categories|labels`；本仓宿主今天
// 不解析 `metadata.*`，所以它们通常为空——空就不画（见包注释第 3 条）。
type Entry struct {
	Name        string
	Title       string
	Description string
	Author      string
	Homepage    string
	Repository  *Repository

	MarketTitle   string
	CategoryTitle string
	LabelTitles   []string

	Versions       []Version
	DefaultVersion string
	Installation   Installation
}

// TitleText 是官方 `entryTitle`（`OFF:…/DiscoverSection.tsx:63-65`）：有标题用标题，
// 否则用包名。**不许**在别处再判一次。
func (e Entry) TitleText() string {
	if e.Title != "" {
		return e.Title
	}
	return e.Name
}

// PresentedVersion 是官方 `presentedVersion`（`OFF:…/presentation.ts:69-77`）：
// 默认版本；默认版本不在列里时退到第一条（宿主保证至少有一条）。
func PresentedVersion(e Entry) (Version, bool) {
	for _, version := range e.Versions {
		if version.Version == e.DefaultVersion {
			return version, true
		}
	}
	if len(e.Versions) == 0 {
		return Version{}, false
	}
	return e.Versions[0], true
}

// Prerelease 是官方 `prerelease`（`OFF:…/DiscoverSection.tsx:78-81`）：SemVer 的
// 预发布后缀（`+` 元数据之前有 `-`）。
func Prerelease(version string) bool {
	build := version
	if index := strings.IndexByte(build, '+'); index >= 0 {
		build = build[:index]
	}
	return strings.Contains(build, "-")
}

// SourceFactKind 是来源 facts 的行种类（决定取哪一条官方 zh 文案）。
type SourceFactKind string

// 官方 `SourceFact['kind']` 的五个值（`OFF:…/presentation.ts:79-83`）。
const (
	FactPackage    SourceFactKind = "package"
	FactRegistry   SourceFactKind = "registry"
	FactRepository SourceFactKind = "repository"
	FactCommit     SourceFactKind = "commit"
	FactDirectory  SourceFactKind = "directory"
)

// SourceFact 是一条"这一版从哪装"的事实（`OFF:…/presentation.ts:79-83`）。
type SourceFact struct {
	Kind  SourceFactKind
	Value string
}

// SourceFacts 是官方 `sourceFacts`（`OFF:…/presentation.ts:85-104`）：
// npm → 包名@版本（+ registry）；git → 仓库 + 完整 commit（+ 包目录）。
//
// 为什么不合并成一行：官方就是按"能否核对来源"逐行摆的，用户装前要看的就是这些。
func SourceFacts(name string, version Version) []SourceFact {
	if version.Source.Kind == SourceNPM {
		facts := []SourceFact{{Kind: FactPackage, Value: name + "@" + version.Version}}
		if version.Source.Registry != "" {
			facts = append(facts, SourceFact{Kind: FactRegistry, Value: version.Source.Registry})
		}
		return facts
	}
	facts := []SourceFact{
		{Kind: FactRepository, Value: version.Source.URL},
		{Kind: FactCommit, Value: version.Source.Commit},
	}
	if version.Source.Path != "" {
		facts = append(facts, SourceFact{Kind: FactDirectory, Value: version.Source.Path})
	}
	return facts
}

// TagTone 是官方 `Tag` 的 tone（`OFF:…/DiscoverSection.tsx:180-190` 的版本行
// 用到 info / neutral / success / warning 四档）。
type TagTone uint8

// 四个 tone。顺序即官方在那个组件里的出现顺序。
const (
	TagInfo TagTone = iota
	TagNeutral
	TagSuccess
	TagWarning
)

// Tag 是一枚只读小标签（官方 `Tag`）。
type Tag struct {
	Text string
	Tone TagTone
}

// VersionRow 是版本列表的一行：版本号 + 状态标签 + 一行说明。
//
// 官方 `VersionRow`（`OFF:…/DiscoverSection.tsx:166-215`）在行尾还有「安装」与
// 「更新」两个按钮——本仓**不画**（安装/更新 501，见包注释），所以这里没有动作。
type VersionRow struct {
	Version string
	Tags    []Tag
	Meta    string
}

// VersionRows 把条目的版本列成官方版本行（`OFF:…/DiscoverSection.tsx:169-215`）。
//
// 标签顺序与官方逐条一致：推荐 → 预发布 → 当前已安装 → 不兼容/已豁免；
// 说明行三档：兼容（发布时间）/ 已豁免 / 不兼容（运行时 + 要求范围）。
// `language` 只影响日期格式。
func VersionRows(entry Entry, language string) []VersionRow {
	rows := make([]VersionRow, 0, len(entry.Versions))
	for _, version := range entry.Versions {
		var tags []Tag
		if version.Version == entry.DefaultVersion {
			tags = append(tags, Tag{Text: zhVersionDefault, Tone: TagInfo})
		}
		if Prerelease(version.Version) {
			tags = append(tags, Tag{Text: zhVersionPrerelease, Tone: TagNeutral})
		}
		if entry.Installation.InstalledVersion == version.Version && version.Version != "" {
			tags = append(tags, Tag{Text: zhVersionInstalled, Tone: TagSuccess})
		}
		compatibility := version.Compatibility
		if !compatibility.Compatible {
			if compatibility.Exempted {
				tags = append(tags, Tag{Text: zhVersionExempted, Tone: TagWarning})
			} else {
				tags = append(tags, Tag{Text: zhVersionIncompatible, Tone: TagWarning})
			}
		}
		rows = append(rows, VersionRow{
			Version: version.Version,
			Tags:    tags,
			Meta:    compatibilityMeta(compatibility, language, version.PublishTimestamp),
		})
	}
	return rows
}

// compatibilityMeta 是官方版本行 `.meta` 那一行的三档（`DiscoverSection.tsx:193-203`）。
func compatibilityMeta(compatibility Compatibility, language string, published int64) string {
	if compatibility.Compatible {
		return FormatDate(published, language)
	}
	if compatibility.Exempted {
		return zhExempted
	}
	return fillTemplate(zhIncompatible, map[string]string{
		"runtime": compatibility.RuntimeVersion,
		"range":   requirementsRange(compatibility.Requirements),
	})
}

// requirementsRange 复刻官方 `Object.values(compatibility.requirements).join(' ')`：
// 只拼值、不排序（官方也没排，它按对象自身的键序）。
func requirementsRange(requirements map[string]string) string {
	if len(requirements) == 0 {
		return ""
	}
	// map 的遍历序在 Go 里是随机的，而这里的键通常只有一个（dsh）。多于一个时按
	// 键排序，保证同一个输入永远画同一行——官方 JS 对象的键序是插入序，我们
	// 拿不到插入序，选一个稳定的序比随机好。
	keys := make([]string, 0, len(requirements))
	for key := range requirements {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, requirements[key])
	}
	return strings.Join(values, " ")
}

// FormatDate 把官方 `publishTimestamp`（秒）按官方 `dateStyle:'medium'` 画成一行。
//
// 官方用 `Intl.DateTimeFormat(language, {dateStyle:'medium'})`
// （`OFF:…/DiscoverSection.tsx:90-92`）。Go 标准库没有 CLDR 本地化数据，所以这里
// 只实现**我们真正会跑到的两种语言**：
//
//	zh*  → `2026年1月5日`（CLDR zh 的 medium）
//	其余 → `Jan 5, 2026`（CLDR en 的 medium）
//
// 这是登记过的差异（见 docs/official-plugin-market-detail-evidence-rc2.md）：除
// 中英文以外的语言会退回英文格式，而不是消失或画成 `0001-01-01`。
func FormatDate(timestamp int64, language string) string {
	if timestamp <= 0 {
		return ""
	}
	at := time.Unix(timestamp, 0)
	if strings.HasPrefix(strings.ToLower(language), "zh") {
		return fmt.Sprintf("%d年%d月%d日", at.Year(), int(at.Month()), at.Day())
	}
	return at.Format("Jan 2, 2006")
}

// fillTemplate 按官方的 `{name}` 占位替换（官方 `t(key, params)` 的同一形状）。
func fillTemplate(text string, params map[string]string) string {
	out := text
	for key, value := range params {
		out = strings.ReplaceAll(out, "{"+key+"}", value)
	}
	return out
}
