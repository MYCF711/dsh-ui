// discover_projection_test.go —— 「发现」tab 的**纯投影**判据（`#30` A/B 块）。
//
// 全部对着官方两份原件：`presentation.ts`（query / 声明排序 / 索引名）与
// `DiscoverSection.tsx`（排序菜单、卡片 meta、过滤开关）。这一层不碰 ui、不发请求，
// 所以能一条一条钉住语义；画的那一层只负责摆位置（由 discover_visual_test.go 验）。
package dsmarket

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// marketMetadataFixture 是索引声明的三张表（与官方 format 1 同形）。
func marketMetadataFixture() apiclient.MarketIndexMetadata {
	return apiclient.MarketIndexMetadata{
		Categories: map[string]apiclient.MarketDeclaration{
			"tools": {Title: apiclient.PluginLocalizedText{Locales: map[string]string{"zh": "工具"}}},
			"ui":    {Title: apiclient.PluginLocalizedText{Locales: map[string]string{"zh": "界面"}}},
		},
		Labels: map[string]apiclient.MarketDeclaration{
			"official": {Title: apiclient.PluginLocalizedText{Locales: map[string]string{"zh": "官方"}}},
			"beta":     {Title: apiclient.PluginLocalizedText{Locales: map[string]string{"zh": "测试版"}}},
		},
		SortKeys: map[string]apiclient.MarketSortKeyDeclaration{
			"stars": {Title: apiclient.PluginLocalizedText{Locales: map[string]string{"zh": "星标"}}, Order: "desc"},
			"name":  {Title: apiclient.PluginLocalizedText{Locales: map[string]string{"zh": "名称"}}, Order: "asc"},
		},
	}
}

// marketEntryFixture 是一条官方字段齐全的条目（labels 故意给反序，验声明顺序）。
func marketEntryFixture() apiclient.MarketEntry {
	return apiclient.MarketEntry{
		Name:        "demo-plugin",
		Title:       apiclient.PluginLocalizedText{Locales: map[string]string{"zh": "演示插件"}},
		Description: apiclient.PluginLocalizedText{Text: "一句话说明"},
		Author:      "Alice",
		Homepage:    "https://example.com/demo",
		Repository:  &apiclient.MarketRepository{URL: "https://github.com/x/y", Directory: "packages/demo"},
		Metadata: apiclient.MarketPluginMetadata{
			Category: "tools",
			Keywords: []string{"dsh"},
			Labels:   []string{"beta", "official"},
			SortKeys: map[string]float64{"stars": 12},
		},
		Versions: []apiclient.MarketListingVersion{{
			Version:          "1.2.3",
			PublishTimestamp: 1767268800,
			Source:           apiclient.MarketSource{Type: apiclient.MarketSourceNPM, Registry: "https://registry.example"},
			Install:          apiclient.MarketInstall{Spec: "demo-plugin@1.2.3"},
			Compatibility:    apiclient.MarketCompatibility{Compatible: true},
		}},
		DefaultVersion: "1.2.3",
		Installation:   apiclient.MarketInstallation{Status: apiclient.MarketInstallStatusNotInstalled},
	}
}

// TestEntryFromAPILooksUpDeclaredTitles 钉住"条目的分类/标签只有 id，标题从声明表查"。
func TestEntryFromAPILooksUpDeclaredTitles(t *testing.T) {
	entry := EntryFromAPI(marketEntryFixture(), marketMetadataFixture(), "演示索引")
	if entry.Title != "演示插件" || entry.Description != "一句话说明" || entry.Author != "Alice" {
		t.Fatalf("基本字段翻错了：%+v", entry)
	}
	if entry.MarketTitle != "演示索引" {
		t.Fatalf("索引名要带上（详情页 facts 的「插件索引」那一行）：%q", entry.MarketTitle)
	}
	if entry.CategoryID != "tools" || entry.CategoryTitle != "工具" {
		t.Fatalf("分类标题没从声明表查出来：id=%q title=%q", entry.CategoryID, entry.CategoryTitle)
	}
	// 标签按**声明顺序**（官方 `displayDeclarations` 的排序：标题排序后 official 在 beta 前）。
	wantLabels := []string{"官方", "测试版"}
	if len(entry.LabelTitles) != len(wantLabels) {
		t.Fatalf("标签标题数 = %d，想要 %d（%v）", len(entry.LabelTitles), len(wantLabels), entry.LabelTitles)
	}
	for i, want := range wantLabels {
		if entry.LabelTitles[i] != want {
			t.Fatalf("标签顺序错了：%v，想要 %v", entry.LabelTitles, wantLabels)
		}
	}
	if len(entry.Versions) != 1 || entry.Versions[0].Version != "1.2.3" ||
		entry.Versions[0].Source.Kind != apiclient.MarketSourceNPM ||
		!entry.Versions[0].Compatibility.Compatible {
		t.Fatalf("版本行翻错了：%+v", entry.Versions)
	}
	if got, ok := PresentedVersion(entry); !ok || got.Version != "1.2.3" {
		t.Fatalf("默认版本应当是 1.2.3：%+v ok=%v", got, ok)
	}
}

// TestEntryFromAPIDoesNotInventUndeclaredTitles 钉住"没声明就不画标题"：
// 拿 id 冒充标题是把索引的问题画成插件的问题。
func TestEntryFromAPIDoesNotInventUndeclaredTitles(t *testing.T) {
	raw := marketEntryFixture()
	raw.Metadata.Category = "gone"
	raw.Metadata.Labels = []string{"gone"}
	entry := EntryFromAPI(raw, marketMetadataFixture(), "演示索引")
	if entry.CategoryTitle != "" {
		t.Fatalf("索引没声明的分类不该有标题，实得 %q", entry.CategoryTitle)
	}
	if len(entry.LabelTitles) != 0 || len(entry.LabelIDs) != 0 {
		t.Fatalf("索引没声明的标签不该出现：%v / %v", entry.LabelTitles, entry.LabelIDs)
	}
}

// TestWithTextSwitchesSortLikeOfficial 钉住 `withText`：空→有字把默认顺序换成最佳匹配，
// 清空时换回来；用户自己选过的顺序不动。
func TestWithTextSwitchesSortLikeOfficial(t *testing.T) {
	query := InitialQuery()
	query = WithText(query, "demo")
	if query.Sort.Kind != apiclient.MarketSortRelevance {
		t.Fatalf("有字之后应切到最佳匹配，实得 %q", query.Sort.Kind)
	}
	query = WithText(query, "")
	if query.Sort.Kind != apiclient.MarketSortIndex {
		t.Fatalf("清空之后应切回默认顺序，实得 %q", query.Sort.Kind)
	}
	// 用户选过"发布时间"之后，打字不该把它改掉。
	query.Sort = apiclient.MarketSort{Kind: apiclient.MarketSortPublished, Direction: "desc"}
	query = WithText(query, "demo")
	if query.Sort.Kind != apiclient.MarketSortPublished || query.Sort.Direction != "desc" {
		t.Fatalf("用户选定的排序不许被打字改掉：%+v", query.Sort)
	}
}

// TestUnfilteredDistinguishesEmptyIndexFromNoMatches 钉住两种空列表的判据。
func TestUnfilteredDistinguishesEmptyIndexFromNoMatches(t *testing.T) {
	if !Unfiltered(InitialQuery()) {
		t.Fatal("默认 query 就该是「没有过滤」")
	}
	// 只有空白的文本等于没有文本（官方 `query.text.trim() === ''`）。
	if !Unfiltered(Query{Text: "   "}) {
		t.Fatal("只有空白的文本应当算「没有过滤」")
	}
	for _, query := range []Query{
		{Text: "demo"},
		{Category: "tools"},
		{Labels: []string{"official"}},
	} {
		if Unfiltered(query) {
			t.Fatalf("%+v 应当算有过滤", query)
		}
	}
}

// TestDisplayDeclarationsSortByTitleThenID 钉住 `displayDeclarations` 的排序。
func TestDisplayDeclarationsSortByTitleThenID(t *testing.T) {
	declarations := map[string]apiclient.MarketDeclaration{
		"b": {Title: apiclient.PluginLocalizedText{Text: "甲"}},
		"a": {Title: apiclient.PluginLocalizedText{Text: "甲"}},
		"c": {Title: apiclient.PluginLocalizedText{Text: "乙"}},
	}
	got := DisplayDeclarations(declarations, func(d apiclient.MarketDeclaration) string { return d.Title.Pick() })
	// 先按标题（"乙" U+4E59 在 "甲" U+7532 之前），标题相同的再按 id 码点（a < b）。
	want := []string{"c", "a", "b"}
	if len(got) != len(want) {
		t.Fatalf("声明数 = %d，想要 %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("排序错了：%v，想要 %v", idsOf(got), want)
		}
	}
}

func idsOf(declarations []Declaration) []string {
	out := make([]string, len(declarations))
	for i, declaration := range declarations {
		out[i] = declaration.ID
	}
	return out
}

// TestSortMenuOptionsFollowText 钉住排序菜单：有文本才有"最佳匹配"，声明过的排序键都在。
func TestSortMenuOptionsFollowText(t *testing.T) {
	sortKeys := SortKeys(marketMetadataFixture())
	options := SortMenuOptions(InitialQuery(), sortKeys)
	if len(options) != 4 {
		t.Fatalf("无文本时应有 默认顺序/发布时间/两个排序键 共 4 项，实得 %d（%+v）", len(options), options)
	}
	if options[0].ID != apiclient.MarketSortIndex || options[1].ID != apiclient.MarketSortPublished {
		t.Fatalf("前三项顺序不对：%+v", options)
	}
	query := InitialQuery()
	query.Text = "demo"
	options = SortMenuOptions(query, sortKeys)
	if options[0].ID != apiclient.MarketSortRelevance || options[0].Label != zhSortRelevance {
		t.Fatalf("有文本时第一项必须是「最佳匹配」：%+v", options)
	}
	// 排序按钮上那一句取的是当前项的名字（官方 `sortBy`）。
	query.Sort = apiclient.MarketSort{Kind: apiclient.MarketSortKey, ID: "stars", Direction: "desc"}
	if got := SortMenuLabel(query, sortKeys); got != "排序：星标" {
		t.Fatalf("排序按钮文案 = %q，想要「排序：星标」", got)
	}
	// 壳还没裁掉的未声明排序键：显示默认顺序，而不是空着。
	query.Sort = apiclient.MarketSort{Kind: apiclient.MarketSortKey, ID: "gone"}
	if got := SortMenuLabel(query, sortKeys); got != "排序：默认顺序" {
		t.Fatalf("未声明的排序键应显示默认顺序，实得 %q", got)
	}
}

// TestSortMenuIDAndDirected 钉住官方两个小判据。
func TestSortMenuIDAndDirected(t *testing.T) {
	if got := SortMenuID(apiclient.MarketSort{Kind: apiclient.MarketSortIndex}); got != "index" {
		t.Fatalf("index 的菜单 id = %q", got)
	}
	if got := SortMenuID(apiclient.MarketSort{Kind: apiclient.MarketSortKey, ID: "stars"}); got != "key:stars" {
		t.Fatalf("key 的菜单 id = %q", got)
	}
	if !Directed(apiclient.MarketSort{Kind: apiclient.MarketSortPublished}) ||
		!Directed(apiclient.MarketSort{Kind: apiclient.MarketSortKey}) ||
		Directed(apiclient.MarketSort{Kind: apiclient.MarketSortIndex}) ||
		Directed(apiclient.MarketSort{Kind: apiclient.MarketSortRelevance}) {
		t.Fatal("只有 published / key 有方向（官方 `directed`）")
	}
}

// TestFilterLabelAndToggle 钉住标签过滤按钮两态与开关语义。
func TestFilterLabelAndToggle(t *testing.T) {
	if FilterMenuLabel(0) != zhFilter {
		t.Fatalf("没选标签时文案 = %q", FilterMenuLabel(0))
	}
	if FilterMenuLabel(2) != "标签 · 2" {
		t.Fatalf("选了 2 个标签时文案 = %q", FilterMenuLabel(2))
	}
	labels := ToggleLabel(nil, "official")
	if len(labels) != 1 || labels[0] != "official" {
		t.Fatalf("加标签失败：%v", labels)
	}
	labels = ToggleLabel(labels, "beta")
	if len(labels) != 2 {
		t.Fatalf("再加一个失败：%v", labels)
	}
	labels = ToggleLabel(labels, "official")
	if len(labels) != 1 || labels[0] != "beta" {
		t.Fatalf("再点一次要去掉：%v", labels)
	}
}

// TestEntryMetaJoinsAuthorAndDate 钉住卡片第二行的两档（官方 `entryMeta` / `entryMetaAuthor`）。
func TestEntryMetaJoinsAuthorAndDate(t *testing.T) {
	entry := EntryFromAPI(marketEntryFixture(), marketMetadataFixture(), "演示索引")
	if got := EntryMeta(entry, "zh-CN"); got != "1.2.3 · Alice · 2026年1月1日" {
		t.Fatalf("有作者时 meta = %q", got)
	}
	entry.Author = ""
	if got := EntryMeta(entry, "zh-CN"); got != "1.2.3 · 2026年1月1日" {
		t.Fatalf("没有作者时 meta = %q", got)
	}
	// 没有版本可展示时整行不画（不许画成 " · date"）。
	entry.Versions = nil
	if got := EntryMeta(entry, "zh-CN"); got != "" {
		t.Fatalf("没有版本时 meta 应当为空，实得 %q", got)
	}
}

// TestQueryWireRoundTrip 钉住 Query ↔ 宿主 query 的双向转换（含 Labels 拷贝）。
func TestQueryWireRoundTrip(t *testing.T) {
	query := Query{Text: "demo", Category: "tools", Labels: []string{"official"},
		Sort: apiclient.MarketSort{Kind: apiclient.MarketSortKey, ID: "stars", Direction: "asc"}}
	wire := query.Wire()
	if wire.Text != "demo" || wire.Category != "tools" || len(wire.Labels) != 1 || wire.Sort.ID != "stars" {
		t.Fatalf("Wire 丢字段：%+v", wire)
	}
	// 改 wire 的 labels 不许影响 query（切片必须拷贝）。
	wire.Labels[0] = "changed"
	if query.Labels[0] != "official" {
		t.Fatal("Wire 必须拷贝 Labels，不能与界面共享底层数组")
	}
	back := QueryFromWire(query.Wire())
	if !back.Equal(query) {
		t.Fatalf("往返不一致：%+v vs %+v", back, query)
	}
	if back.Equal(Query{}) {
		t.Fatal("不同 query 不该判等")
	}
}
