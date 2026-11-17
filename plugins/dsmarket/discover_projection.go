// discover_projection.go —— Learn tab 的**纯投影**：官方 `MarketSearchResult` /
// `MarketIndexMetadata` → 本包画得出来的形状，以及工具栏那些判据。
//
// 官方对应：
//
//	OFF:../dsh-client-ui-plugin-market/src/client/presentation.ts:6-12   DiscoverQuery
//	OFF:presentation.ts:15                                              INITIAL_QUERY
//	OFF:presentation.ts:24-34                                           withText
//	OFF:presentation.ts:51-67                                           displayDeclarations
//	OFF:presentation.ts:69-77                                           presentedVersion
//	OFF:presentation.ts:106-115                                         indexLabel
//	OFF:DiscoverSection.tsx:63-65,84-88,196-204,470-481                 entryTitle / installable /
//	                                                                    entryMeta / sortMenuId / directed
//
// 这些函数都是**纯的**（不碰 ui、不发请求），所以单测能直接钉住官方语义；
// 画的那一层只负责摆位置。
package dsmarket

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// Query 是官方 `DiscoverQuery`：文本 + 过滤（分类、标签）+ 排序，不含分页。
type Query struct {
	Text     string
	Category string
	Labels   []string
	Sort     apiclient.MarketSort
}

// InitialQuery 是官方 `INITIAL_QUERY`（`OFF:presentation.ts:15`）：
// 空文本、无过滤、默认顺序。
func InitialQuery() Query {
	return Query{Sort: apiclient.MarketSort{Kind: apiclient.MarketSortIndex}}
}

// Wire 把界面 query 翻成宿主 query（分页参数由壳补）。
func (q Query) Wire() apiclient.MarketQuery {
	labels := make([]string, len(q.Labels))
	copy(labels, q.Labels)
	return apiclient.MarketQuery{Text: q.Text, Category: q.Category, Labels: labels, Sort: q.Sort}
}

// QueryFromWire 反向：壳回来声明裁剪过的 query 时，界面跟着它走
// （官方是控制器直接持有 query，本仓壳与插件分居两处，所以要这一趟）。
func QueryFromWire(q apiclient.MarketQuery) Query {
	labels := make([]string, len(q.Labels))
	copy(labels, q.Labels)
	return Query{Text: q.Text, Category: q.Category, Labels: labels, Sort: q.Sort}
}

// Equal 报告两个 query 是否逐字段相同（Labels 是切片，不能直接用 ==）。
func (q Query) Equal(other Query) bool {
	if q.Text != other.Text || q.Category != other.Category || q.Sort != other.Sort ||
		len(q.Labels) != len(other.Labels) {
		return false
	}
	for i := range q.Labels {
		if q.Labels[i] != other.Labels[i] {
			return false
		}
	}
	return true
}

// WithText 是官方 `withText`（`OFF:presentation.ts:24-34`）：
// 空文本变成有字时"默认顺序"切到"最佳匹配"，再清空时切回来；用户自己选过的顺序不动。
func WithText(query Query, text string) Query {
	wasBlank := strings.TrimSpace(query.Text) == ""
	blank := strings.TrimSpace(text) == ""
	out := query
	out.Text = text
	switch {
	case wasBlank && !blank && query.Sort.Kind == apiclient.MarketSortIndex:
		out.Sort = apiclient.MarketSort{Kind: apiclient.MarketSortRelevance}
	case blank && query.Sort.Kind == apiclient.MarketSortRelevance:
		out.Sort = apiclient.MarketSort{Kind: apiclient.MarketSortIndex}
	}
	return out
}

// Unfiltered 是官方 `unfiltered`（`OFF:DiscoverSection.tsx:441-443`）：
// 没有任何过滤条件 = "这个索引本来就空"而不是"没命中"。
func Unfiltered(query Query) bool {
	return strings.TrimSpace(query.Text) == "" && query.Category == "" && len(query.Labels) == 0
}

// Declaration 是一条市场声明（分类 / 标签 / 排序键）在当前语言下的显示形状。
type Declaration struct {
	// ID 是声明 id（query 里引用的就是它）。
	ID string
	// Title 是当前语言的显示标题。
	Title string
	// Order 只有排序键声明有值（官方 `MarketSortKeyDeclaration.order`）。
	Order string
}

// DisplayDeclarations 是官方 `displayDeclarations`（`OFF:presentation.ts:51-67`）：
// 按显示标题排序，标题相同的按 id 的码点排序——工具栏的顺序因此是稳定的。
func DisplayDeclarations[V any](declarations map[string]V, title func(V) string) []Declaration {
	out := make([]Declaration, 0, len(declarations))
	for id, declaration := range declarations {
		out = append(out, Declaration{ID: id, Title: title(declaration)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Title != out[j].Title {
			return out[i].Title < out[j].Title
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Categories 把索引声明的分类排成官方显示顺序。
func Categories(metadata apiclient.MarketIndexMetadata) []Declaration {
	return DisplayDeclarations(metadata.Categories, func(d apiclient.MarketDeclaration) string { return d.Title.Pick() })
}

// Labels 把索引声明的标签排成官方显示顺序。
func Labels(metadata apiclient.MarketIndexMetadata) []Declaration {
	return DisplayDeclarations(metadata.Labels, func(d apiclient.MarketDeclaration) string { return d.Title.Pick() })
}

// SortKeys 把索引声明的排序键排成官方显示顺序（带上各自的初始方向）。
func SortKeys(metadata apiclient.MarketIndexMetadata) []Declaration {
	out := DisplayDeclarations(metadata.SortKeys, func(d apiclient.MarketSortKeyDeclaration) string { return d.Title.Pick() })
	for i, key := range out {
		out[i].Order = metadata.SortKeys[key.ID].Order
	}
	return out
}

// EntryFromAPI 把宿主的官方 `MarketListingEntry` 翻成详情页/卡片共用的 Entry。
//
// metadata / marketTitle 来自同一份搜索结果的**索引级**声明：条目的分类与标签只有 id，
// 标题得从声明表里查（官方 `EntryDetail` 与卡片标签都是这么来的）。
func EntryFromAPI(entry apiclient.MarketEntry, metadata apiclient.MarketIndexMetadata, marketTitle string) Entry {
	out := Entry{
		Name:           entry.Name,
		Title:          entry.Title.Pick(),
		Icon:           entry.Icon,
		Description:    entry.Description.Pick(),
		Author:         entry.Author,
		Homepage:       entry.Homepage,
		MarketTitle:    marketTitle,
		DefaultVersion: entry.DefaultVersion,
		Installation: Installation{
			Status:           entry.Installation.Status,
			InstalledVersion: entry.Installation.InstalledVersion,
		},
	}
	if entry.Repository != nil && entry.Repository.URL != "" {
		out.Repository = &Repository{URL: entry.Repository.URL, Directory: entry.Repository.Directory}
	}
	// 分类标题只在索引真的声明了它时才有（官方同判据：声明表里查不到就不画那一行）。
	if entry.Metadata.Category != "" {
		out.CategoryID = entry.Metadata.Category
		if declared, ok := metadata.Categories[entry.Metadata.Category]; ok {
			out.CategoryTitle = declared.Title.Pick()
		}
	}
	// 标签按**声明顺序**排（官方卡片：`labels.filter(label => entry.metadata.labels.includes(label.id))`）。
	declaredLabels := Labels(metadata)
	for _, declaration := range declaredLabels {
		if !containsString(entry.Metadata.Labels, declaration.ID) {
			continue
		}
		out.LabelIDs = append(out.LabelIDs, declaration.ID)
		out.LabelTitles = append(out.LabelTitles, declaration.Title)
	}
	out.Versions = make([]Version, 0, len(entry.Versions))
	for _, version := range entry.Versions {
		out.Versions = append(out.Versions, VersionFromAPI(version))
	}
	return out
}

// VersionFromAPI 把官方版本行翻成详情页用的 Version。
func VersionFromAPI(version apiclient.MarketListingVersion) Version {
	out := Version{
		Version:          version.Version,
		PublishTimestamp: version.PublishTimestamp,
		Update:           version.Update,
		Source: Source{
			Kind:     version.Source.Type,
			URL:      version.Source.URL,
			Commit:   version.Source.Commit,
			Path:     version.Source.Path,
			Registry: version.Source.Registry,
		},
		Compatibility: Compatibility{
			Compatible:     version.Compatibility.Compatible,
			Exempted:       version.Compatibility.Exempted,
			RuntimeVersion: version.Compatibility.RuntimeVersion,
			Requirements:   version.Compatibility.Requirements,
		},
	}
	return out
}

// EntriesFromResult 把一页搜索结果整批翻成 Entry（卡片的顺序就是宿主的顺序）。
func EntriesFromResult(result apiclient.MarketSearchResult) []Entry {
	marketTitle := result.Title.Pick()
	out := make([]Entry, 0, len(result.Entries))
	for _, entry := range result.Entries {
		out = append(out, EntryFromAPI(entry, result.Metadata, marketTitle))
	}
	return out
}

// EntryMeta 是官方卡片第二行（`OFF:DiscoverSection.tsx:196-204`）：
// 有作者就三段、没有就两段；日期按当前语言（官方 `dateStyle:'medium'`）。
func EntryMeta(entry Entry, language string) string {
	version, ok := PresentedVersion(entry)
	if !ok {
		return ""
	}
	date := FormatDate(version.PublishTimestamp, language)
	params := map[string]string{"version": version.Version, "date": date}
	if entry.Author == "" {
		return fillTemplate(zhEntryMeta, params)
	}
	params["author"] = entry.Author
	return fillTemplate(zhEntryMetaAuthor, params)
}

// containsString 报告切片里有没有这个值。
func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// SortMenuOption 是排序菜单的一项（官方 `sortOptions`，`OFF:DiscoverSection.tsx:470-478`）。
type SortMenuOption struct {
	// ID 是菜单项的 id：`relevance` / `index` / `published` / `key:<声明 id>`。
	ID string
	// Label 是菜单项文案。
	Label string
}

// SortMenuID 是官方 `sortMenuId`（`OFF:DiscoverSection.tsx:434-436`）。
func SortMenuID(sort apiclient.MarketSort) string {
	if sort.Kind == apiclient.MarketSortKey {
		return "key:" + sort.ID
	}
	return sort.Kind
}

// Directed 是官方 `directed`（`OFF:DiscoverSection.tsx:482`）：
// published / key 两种排序有方向，另外两种没有。
func Directed(sort apiclient.MarketSort) bool {
	return sort.Kind == apiclient.MarketSortPublished || sort.Kind == apiclient.MarketSortKey
}

// SortMenuOptions 是官方排序菜单的选项表（`OFF:DiscoverSection.tsx:470-478`）：
// 文本非空才有"最佳匹配"，然后是默认顺序、发布时间，最后是索引声明的每一个排序键。
func SortMenuOptions(query Query, sortKeys []Declaration) []SortMenuOption {
	options := make([]SortMenuOption, 0, len(sortKeys)+3)
	if strings.TrimSpace(query.Text) != "" {
		options = append(options, SortMenuOption{ID: apiclient.MarketSortRelevance, Label: zhSortRelevance})
	}
	options = append(options,
		SortMenuOption{ID: apiclient.MarketSortIndex, Label: zhSortDefault},
		SortMenuOption{ID: apiclient.MarketSortPublished, Label: zhSortPublished},
	)
	for _, key := range sortKeys {
		options = append(options, SortMenuOption{ID: "key:" + key.ID, Label: key.Title})
	}
	return options
}

// SortMenuLabel 是排序按钮上那一句（官方 `sortBy`）。
func SortMenuLabel(query Query, sortKeys []Declaration) string {
	id := SortMenuID(query.Sort)
	for _, option := range SortMenuOptions(query, sortKeys) {
		if option.ID == id {
			return fillTemplate(zhSortBy, map[string]string{"sort": option.Label})
		}
	}
	// 走到这里说明壳还没把未声明的排序键裁掉（一帧的窗口）：显示默认顺序，
	// 而不是一句「排序：」后面空着。
	return fillTemplate(zhSortBy, map[string]string{"sort": zhSortDefault})
}

// FilterMenuLabel 是标签过滤按钮的两态（官方 `filter` / `filterCount`）。
func FilterMenuLabel(count int) string {
	if count == 0 {
		return zhFilter
	}
	return fillTemplate(zhFilterCount, map[string]string{"count": fmt.Sprintf("%d", count)})
}

// ToggleLabel 在标签集合里加上或去掉一个 id（官方过滤菜单的 onSelect）。
func ToggleLabel(labels []string, id string) []string {
	out := make([]string, 0, len(labels)+1)
	removed := false
	for _, label := range labels {
		if label == id {
			removed = true
			continue
		}
		out = append(out, label)
	}
	if !removed {
		out = append(out, id)
	}
	return out
}
