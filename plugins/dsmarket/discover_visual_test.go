//go:build darwin || windows || linux

// discover_visual_test.go —— 「发现」tab 的**真实像素帧**验收（`#30` A/B 块）。
//
// 为什么必须有这一条：单测证明"投影算对了"，证明不了它**画在屏幕上**、也证明不了
// 点得到。这个仓里"算了但没画"踩过好几次，所以每块新界面都要一帧真实像素 + 一次真
// 命中区点击。
//
// 判据取自官方：
//
//	工具栏五件套（索引菜单 / 搜索框 / 标签过滤 / 排序 / 刷新）  DiscoverSection.tsx:492-560
//	分类胶囊行（「全部」+ 每个声明的分类）                      :565-583
//	卡片的标题 / 说明 / meta / 标签                            :150-165
//	空态「还没有插件索引」+「添加索引源」                      :585-605
//	索引源对话框（标题 / 说明 / 列表 / 输入框 / 添加 / 信任提示）:318-350
//
// 两条**否定**判据同样重要（画了不该画的东西也是错）：
//   - 安装 / 更新按钮一个都不画（宿主 installBundle/updateBundle 是 501）；
//   - 没有索引源时不画工具栏（官方那时整页就是空态）。
package dsmarket

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// discoverIndexes 是两条索引源（工具栏的索引菜单与对话框都要用）。
func discoverIndexes() []apiclient.MarketIndexSummary {
	return []apiclient.MarketIndexSummary{
		{URL: "https://m.example/index.json", Title: apiclient.PluginLocalizedText{Text: "演示索引"}},
		{URL: "https://n.example/index.json", Title: apiclient.PluginLocalizedText{Text: "第二个索引"}},
	}
}

// discoverListing 是一次落定的搜索结果（两条条目 + 完整声明表）。
func discoverListing() pluginapi.MarketListing {
	query := apiclient.MarketQuery{Sort: apiclient.MarketSort{Kind: apiclient.MarketSortIndex}}
	metadata := marketMetadataFixture()
	result := apiclient.MarketSearchResult{
		URL:       "https://m.example/index.json",
		Title:     apiclient.PluginLocalizedText{Text: "演示索引"},
		Metadata:  metadata,
		FetchedAt: 1767268800000,
		Total:     2,
		Entries:   []apiclient.MarketEntry{marketEntryFixture(), secondMarketEntry()},
	}
	return pluginapi.MarketListing{
		URL:      result.URL,
		Query:    query,
		Answered: &query,
		Status:   pluginapi.MarketListingReady,
		Value:    &result,
	}
}

// secondMarketEntry 是第二条（无作者、无标签，验"缺席就不画"）。
func secondMarketEntry() apiclient.MarketEntry {
	return apiclient.MarketEntry{
		Name:  "another-plugin",
		Title: apiclient.PluginLocalizedText{Text: "另一个插件"},
		Versions: []apiclient.MarketListingVersion{{
			Version:          "0.1.0",
			PublishTimestamp: 1767355200,
			Source:           apiclient.MarketSource{Type: apiclient.MarketSourceGit, URL: "https://github.com/x/z", Commit: "abc"},
			Compatibility:    apiclient.MarketCompatibility{Compatible: true},
		}},
		DefaultVersion: "0.1.0",
	}
}

// discoverActs 是一组记录型 Actions：把插件真正调了哪一下记下来。
type discoverActs struct {
	indexes      []apiclient.MarketIndexSummary
	indexesReady bool
	listing      pluginapi.MarketListing
	hasListing   bool

	searchQueries []apiclient.MarketQuery
	selects       []string
	mores         int
	refreshes     int
	retries       int
	setIndexes    [][]string
	reloads       int
}

func (a *discoverActs) actions() pluginapi.Actions {
	return pluginapi.Actions{
		MarketIndexes: func() ([]apiclient.MarketIndexSummary, bool, bool, string) {
			return a.indexes, a.indexesReady, false, ""
		},
		MarketIndexesReload: func() { a.reloads++ },
		MarketSelectIndex:   func(url string) { a.selects = append(a.selects, url) },
		MarketListing: func() (pluginapi.MarketListing, bool) {
			return a.listing, a.hasListing
		},
		MarketSearch: func(query apiclient.MarketQuery, _ string) {
			a.searchQueries = append(a.searchQueries, query)
		},
		MarketMore:         func() { a.mores++ },
		MarketRefresh:      func() { a.refreshes++ },
		MarketRetry:        func() { a.retries++ },
		MarketSetIndexes:   func(urls []string) { a.setIndexes = append(a.setIndexes, urls) },
		MarketSourcesState: func() (bool, apiclient.MarketSourceFailureKind, string) { return false, "", "" },
		MarketEntryDetail: func(url, name, version, language string) (apiclient.PluginMarketEntryDetail, bool, apiclient.PluginMarketDetailFailure) {
			return apiclient.PluginMarketEntryDetail{
				Status:   apiclient.PluginDetailFound,
				Origin:   apiclient.PluginDetailOriginReadme,
				Markdown: "## 用法\n\n先跑 `demo --help`。",
				Document: name + "@" + version,
			}, true, ""
		},
	}
}

// discoverRig 是一帧真绘制的装置。
type discoverRig struct {
	discover   *Discover
	env        *pluginapi.Env
	session    *headless.Session
	invalidato *app.Invalidator
}

// openDiscoverRig 起一个真 Viva 无头窗口，把「发现」tab 画在主区（902px 宽）。
func openDiscoverRig(t *testing.T, acts *discoverActs) *discoverRig {
	t.Helper()
	theme := dscore.Dark()
	page := NewPage(theme)
	discover := NewDiscover(theme, page)

	tree := &gfx.Tree{}
	feat := tree.Feature("market-discover-headless")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{
		Title: "market-discover", Width: 1182, Height: 832, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
		// 真输入框必须交给平台：壳在 main.go 里是 `ui.Fields(fields...)`
		// （cmd/app/main.go:1793），这里同一句——否则点了能聚焦、但打不进字。
		TextInput: ui.Fields(discover.Fields()...),
	}
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: feat,
		Bounds:    gfx.Rect{Width: 1182, Height: 832},
		MainWidth: 902,
		Geometry:  pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902},
		Act:       acts.actions(),
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env.Canvas = canvas
		view.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).Width(902).FillHeight().Children(func() {
				discover.Draw(f, env)
			})
		})
		page.afterDraw(canvas, env)
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	invalidator.Invalidate()
	return &discoverRig{discover: discover, env: env, session: session, invalidato: &invalidator}
}

// pumpUntil 一直泵到条件成立（viva 的字体测量第一帧之后才稳定，一帧就断言会时红时绿）。
func (r *discoverRig) pumpUntil(t *testing.T, what string, cond func(text string) bool) app.Snapshot {
	t.Helper()
	var text string
	for i := 0; i < 30; i++ {
		if err := r.session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
		text = discoverDrawnText(r.session.Snapshot())
		if cond(text) {
			return r.session.Snapshot()
		}
	}
	t.Fatalf("30 帧内没等到「%s」：\n%s", what, text)
	return app.Snapshot{}
}

// discoverDrawnText 把这一帧真正画出来的文字拼成一段。
func discoverDrawnText(snapshot app.Snapshot) string {
	values := make([]string, 0, len(snapshot.Labels))
	for _, label := range snapshot.Labels {
		values = append(values, label.Value)
	}
	return strings.Join(values, "\n")
}

// TestDiscoverDrawsToolbarAndCards 是主证据：工具栏五件套 + 分类行 + 两张卡片真的画出来。
func TestDiscoverDrawsToolbarAndCards(t *testing.T) {
	acts := &discoverActs{indexes: discoverIndexes(), indexesReady: true, listing: discoverListing(), hasListing: true}
	rig := openDiscoverRig(t, acts)
	text := discoverDrawnText(rig.pumpUntil(t, "工具栏与卡片", func(text string) bool {
		return strings.Contains(text, "演示插件") && strings.Contains(text, "刷新")
	}))

	for _, want := range []string{
		"演示索引",    // 索引菜单触发器（官方 indexLabel）
		"刷新",      // 刷新按钮（官方是图标，本仓文字化）
		"标签",      // 标签过滤按钮（0 个时就是 `filter`）
		"排序：默认顺序", // 排序按钮（官方 `sortBy`）
		"全部",      // 分类行第一个胶囊（官方 `allCategories`）
		"工具",      // 声明的分类
		"界面",      // 声明的分类
		"演示插件",    // 卡片标题
		"一句话说明",   // 卡片说明
		"官方",      // 条目的声明标签（Tag）
		"另一个插件",   // 第二张卡
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("画面上缺 %q：\n%s", want, text)
		}
	}
	// 卡片第二行：版本 · 作者 · 日期（官方 entryMetaAuthor）。
	if !strings.Contains(text, "1.2.3 · Alice · ") {
		t.Fatalf("卡片 meta 行没画（应当带版本与作者）：\n%s", text)
	}
	// 安装/更新按钮一个都不许画（宿主 501）。判据是整条文字相等——
	// "当前已安装"这类标签本身带"安装"，用子串判会自伤（C 块踩过）。
	for _, label := range rig.session.Snapshot().Labels {
		if label.Value == "安装" || label.Value == "更新" {
			t.Fatalf("%q 按钮不该出现（宿主 installBundle/updateBundle 是 501）：\n%s", label.Value, text)
		}
	}
}

// TestDiscoverSearchFieldIsRealInput 搜索框是真输入框：点了能打字，打字真的发出 query。
func TestDiscoverSearchFieldIsRealInput(t *testing.T) {
	acts := &discoverActs{indexes: discoverIndexes(), indexesReady: true, listing: discoverListing(), hasListing: true}
	rig := openDiscoverRig(t, acts)
	rig.pumpUntil(t, "搜索框", func(text string) bool { return strings.Contains(text, "刷新") })

	if err := rig.session.ClickLabel("搜索插件"); err != nil {
		t.Fatalf("点搜索框：%v", err)
	}
	if err := rig.session.Type("demo"); err != nil {
		t.Fatalf("往搜索框打字：%v", err)
	}
	rig.pumpUntil(t, "打字发出 query", func(string) bool { return len(acts.searchQueries) > 0 })
	last := acts.searchQueries[len(acts.searchQueries)-1]
	if last.Text != "demo" {
		t.Fatalf("打出来的 query.Text = %q，想要 demo", last.Text)
	}
	// 官方 `withText`：从空文本变有字，默认顺序切成最佳匹配。
	if last.Sort.Kind != apiclient.MarketSortRelevance {
		t.Fatalf("从空文本开始打字后排序应是 relevance，实得 %q", last.Sort.Kind)
	}
}

// TestDiscoverCategoryPillFiltersAndSortMenuOpens 点分类胶囊真的发 query；
// 索引菜单点开真的列出全部索引源。
func TestDiscoverCategoryPillFiltersAndSortMenuOpens(t *testing.T) {
	acts := &discoverActs{indexes: discoverIndexes(), indexesReady: true, listing: discoverListing(), hasListing: true}
	rig := openDiscoverRig(t, acts)
	rig.pumpUntil(t, "分类行", func(text string) bool { return strings.Contains(text, "工具") })

	if err := rig.session.ClickLabel("工具"); err != nil {
		t.Fatalf("点分类胶囊：%v", err)
	}
	rig.pumpUntil(t, "分类过滤发出 query", func(string) bool { return len(acts.searchQueries) > 0 })
	if got := acts.searchQueries[len(acts.searchQueries)-1].Category; got != "tools" {
		t.Fatalf("点「工具」应当按 tools 过滤，实得 %q", got)
	}

	// 索引菜单：点开之后两个索引源都要在画面上（官方 Menu 的 items）。
	if err := rig.session.ClickLabel("演示索引"); err != nil {
		t.Fatalf("点索引菜单：%v", err)
	}
	rig.pumpUntil(t, "索引菜单列出两个源", func(text string) bool {
		return strings.Contains(text, "第二个索引") && strings.Contains(text, "管理索引源…")
	})
}

// TestDiscoverEntryOpensDetailAndBackReturns 点卡片标题真的打开详情页；
// 面包屑「发现」真的回到列表。
func TestDiscoverEntryOpensDetailAndBackReturns(t *testing.T) {
	acts := &discoverActs{indexes: discoverIndexes(), indexesReady: true, listing: discoverListing(), hasListing: true}
	rig := openDiscoverRig(t, acts)
	rig.pumpUntil(t, "卡片", func(text string) bool { return strings.Contains(text, "演示插件") })

	if err := rig.session.ClickLabel("演示插件"); err != nil {
		t.Fatalf("点卡片标题：%v", err)
	}
	text := discoverDrawnText(rig.pumpUntil(t, "详情页", func(text string) bool {
		return strings.Contains(text, "用法") && strings.Contains(text, "demo --help")
	}))
	// 详情页有返回目标 → 面包屑「发现」（C 块判据的另一半）。
	if !strings.Contains(text, "发现") {
		t.Fatalf("详情页应当画面包屑：\n%s", text)
	}
	if !rig.discover.DetailOpen() {
		t.Fatal("打开详情后 DetailOpen 应当为 true（页面据此让出内距）")
	}

	if err := rig.session.ClickLabel("发现"); err != nil {
		t.Fatalf("点面包屑返回：%v", err)
	}
	rig.pumpUntil(t, "回到列表", func(text string) bool {
		return strings.Contains(text, "演示插件") && !strings.Contains(text, "demo --help")
	})
	if rig.discover.DetailOpen() {
		t.Fatal("返回列表之后 DetailOpen 应当为 false")
	}
}

// TestDiscoverEmptyIndexDrawsAddSource 没有任何索引源时画官方空态；点「添加索引源」
// 真的打开索引源对话框（标题/说明/输入框/添加/信任提示都在）。
func TestDiscoverEmptyIndexDrawsAddSource(t *testing.T) {
	acts := &discoverActs{indexesReady: true}
	rig := openDiscoverRig(t, acts)
	text := discoverDrawnText(rig.pumpUntil(t, "空态", func(text string) bool {
		return strings.Contains(text, "还没有插件索引")
	}))
	if !strings.Contains(text, "添加一个插件索引") || !strings.Contains(text, "添加索引源") {
		t.Fatalf("空态缺说明或按钮：\n%s", text)
	}
	// 没有索引源时不该画工具栏（官方那时整页就是空态）。
	if strings.Contains(text, "刷新") || strings.Contains(text, "排序：") {
		t.Fatalf("没有索引源时不该画工具栏：\n%s", text)
	}

	if err := rig.session.ClickLabel("添加索引源"); err != nil {
		t.Fatalf("点「添加索引源」：%v", err)
	}
	text = discoverDrawnText(rig.pumpUntil(t, "索引源对话框", func(text string) bool {
		return strings.Contains(text, "索引由第三方提供")
	}))
	// 输入框在没有内容时画的是官方占位（`sourcePlaceholder`），它的可访问名是
	// `sourceUrl`——本仓没有无障碍树，所以按画出来的字断言。
	for _, want := range []string{"索引源", "插件索引以一个索引文件的 URL 提供", "添加", "https://example.com/index.json"} {
		if !strings.Contains(text, want) {
			t.Fatalf("对话框缺 %q：\n%s", want, text)
		}
	}
}

// TestDiscoverSourcesDialogAddsAndRemoves 对话框里打字 + 点「添加」真的写回索引源；
// 点「移除」真的把那个源从列表里去掉。
func TestDiscoverSourcesDialogAddsAndRemoves(t *testing.T) {
	acts := &discoverActs{indexes: discoverIndexes(), indexesReady: true, listing: discoverListing(), hasListing: true}
	rig := openDiscoverRig(t, acts)
	rig.pumpUntil(t, "工具栏", func(text string) bool { return strings.Contains(text, "刷新") })

	// 工具栏的索引菜单 → 管理索引源…
	if err := rig.session.ClickLabel("演示索引"); err != nil {
		t.Fatalf("点索引菜单：%v", err)
	}
	if err := rig.session.ClickLabel("管理索引源…"); err != nil {
		t.Fatalf("点「管理索引源…」：%v", err)
	}
	rig.pumpUntil(t, "对话框打开", func(text string) bool { return strings.Contains(text, "索引由第三方提供") })

	// 列表里两个源都在（标题 + URL）。
	text := discoverDrawnText(rig.session.Snapshot())
	for _, want := range []string{"演示索引", "第二个索引", "https://m.example/index.json"} {
		if !strings.Contains(text, want) {
			t.Fatalf("对话框缺索引源 %q：\n%s", want, text)
		}
	}

	// 往 URL 输入框打字，然后点「添加」。
	if err := rig.session.ClickLabel("索引 URL"); err != nil {
		t.Fatalf("点 URL 输入框：%v", err)
	}
	if err := rig.session.Type("https://new.example/index.json"); err != nil {
		t.Fatalf("输入新索引源：%v", err)
	}
	rig.session.Driver().Pump()
	if err := rig.session.ClickLabel("添加"); err != nil {
		t.Fatalf("点「添加」：%v", err)
	}
	rig.pumpUntil(t, "写回索引源", func(string) bool { return len(acts.setIndexes) > 0 })
	got := acts.setIndexes[len(acts.setIndexes)-1]
	if len(got) != 3 || got[2] != "https://new.example/index.json" {
		t.Fatalf("添加应把新源追加到列表末尾（官方同序）：%v", got)
	}

	// 移除第一个源：只剩另一个 + 新加的那个。
	if err := rig.session.ClickLabel("移除 https://m.example/index.json"); err != nil {
		t.Fatalf("点「移除」：%v", err)
	}
	rig.pumpUntil(t, "写回移除", func(string) bool { return len(acts.setIndexes) > 1 })
	got = acts.setIndexes[len(acts.setIndexes)-1]
	// 夹具的 listIndexes 不随写回变化（真宿主会重读），所以这里剩下的是第二个源。
	if len(got) != 1 || got[0] != "https://n.example/index.json" {
		t.Fatalf("移除第一个源之后应只剩第二个：%v", got)
	}
}

// TestDiscoverListingFailureDrawsRetry 列表读失败时画「无法读取此索引」+ 重试，
// 点重试真的走到壳的重试动作；**旧结果仍然画着**（官方同一条：失败不清旧值）。
func TestDiscoverListingFailureDrawsRetry(t *testing.T) {
	listing := discoverListing()
	listing.Status = pluginapi.MarketListingError
	listing.Failure = apiclient.MarketFailureFetch
	acts := &discoverActs{indexes: discoverIndexes(), indexesReady: true, listing: listing, hasListing: true}
	rig := openDiscoverRig(t, acts)
	text := discoverDrawnText(rig.pumpUntil(t, "失败行", func(text string) bool {
		return strings.Contains(text, "无法读取此索引")
	}))
	if !strings.Contains(text, "重试") {
		t.Fatalf("失败行要带重试：\n%s", text)
	}
	if !strings.Contains(text, "演示插件") {
		t.Fatalf("失败时已读到的结果必须保留：\n%s", text)
	}
	if err := rig.session.ClickLabel("重试"); err != nil {
		t.Fatalf("点重试：%v", err)
	}
	rig.pumpUntil(t, "重试动作", func(string) bool { return acts.retries > 0 })
}

// TestDiscoverSourcesFailureDrawsRetry 索引源清单读不到时画「无法读取索引源」+ 重试。
func TestDiscoverSourcesFailureDrawsRetry(t *testing.T) {
	acts := &discoverActs{}
	rig := openDiscoverRig(t, acts)
	rig.env.Act.MarketIndexes = func() ([]apiclient.MarketIndexSummary, bool, bool, string) {
		return nil, false, true, "not_implemented: 宿主没接 pluginMarket"
	}
	text := discoverDrawnText(rig.pumpUntil(t, "索引源失败", func(text string) bool {
		return strings.Contains(text, "无法读取索引源")
	}))
	if !strings.Contains(text, "重试") {
		t.Fatalf("索引源失败也要能重试：\n%s", text)
	}
	if err := rig.session.ClickLabel("重试"); err != nil {
		t.Fatalf("点重试：%v", err)
	}
	rig.pumpUntil(t, "重载索引源", func(string) bool { return acts.reloads > 0 })
}
