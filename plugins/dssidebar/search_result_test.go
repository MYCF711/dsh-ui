// search_result_test.go —— 官方 `searchResultRow`（48px 搜索结果卡）的规格测试。
//
// 官方原文（`tmp/official/dsh-client-ui-workspace/client.js`，模块 hash `wnFoaW`）：
//
//	.wnFoaW_searchResultRow{box-sizing:border-box;border-radius:var(--dsw-radius-lg);
//	  cursor:pointer;text-align:left;width:100%;min-height:48px;
//	  color:var(--dsw-alias-label-primary);background:0 0;border:none;
//	  flex-direction:column;align-items:stretch;padding:4px 8px;display:flex}
//	.wnFoaW_searchResultRow:hover,.wnFoaW_searchResultRow.wnFoaW_selected{
//	  background:var(--dsw-alias-interactive-bg-hover)}
//	.wnFoaW_searchResultMeta{align-items:center;gap:6px;min-width:0;
//	  margin-left:20px;display:flex}
//	.wnFoaW_searchResultTitle{…;margin-left:4px;font-size:14px;line-height:20px;…}
//	.wnFoaW_searchResultWorkspace,.wnFoaW_searchResultSnippet{…;
//	  font-size:12px;line-height:17px;…}
//	.wnFoaW_searchResultWorkspace{max-width:40%;color:var(--dsw-alias-label-tertiary);flex:none}
//	.wnFoaW_searchResultSnippet{min-width:0;color:var(--dsw-alias-label-secondary);flex:1}
//	.GAkraW_searchTree>[role=treeitem]+[role=treeitem]{margin-top:2px}
//
// 官方 JSX（同一个包）：
//
//	SearchResultItem: button.searchResultRow
//	  div.searchResultHeading → span.searchResultTitle{result.title || t("session.untitled")}
//	  div.searchResultMeta    → span.searchResultWorkspace{result.workspace || t("group.ungrouped")}
//	                            result.snippet !== void 0 && span.searchResultSnippet{result.snippet}
//
// 其中 `snippet` **只有宿主全文检索结果页（content.items）才给**：
// `deriveSearchResults` 里本地元信息命中（标题/工作区名）的每一项都**没有** snippet 字段。
// 我们的 Snapshot/Transport 也没有这个字段（见 docs/visual-search-result.md §2），
// 所以生产路径永远是"没有真值 → 整段不画"，绝不拿标题或 cwd 拼一个假摘要。
package dssidebar

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// measuredText 是一帧里 flex 真正量过的文字。文本节点必须量宽才能布局，
// 所以这个序列等价于"这一帧画了哪些字、各是什么字号"。
type measuredText struct {
	value string
	size  float32
}

// searchFixture 在 v2Fixture 之上换一个记录型 measurer。
func searchFixture(t *testing.T) (*Plugin, *v2RecordingPrimitives, *pluginapi.Env, *gfx.Tree, *gfx.Canvas, *[]measuredText) {
	t.Helper()
	p, rec, env, tree, canvas := v2Fixture()
	texts := &[]measuredText{}
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		*texts = append(*texts, measuredText{value: value, size: size})
		return float32(len([]rune(value))) * size / 2
	})
	return p, rec, env, tree, canvas, texts
}

// mockdSessions 是 mockd 的三条真实夹具（同一个 cwd、三个标题），
// 用来量"搜索态到底画成了什么"。récency 顺序：s-003 → s-001 → s-002。
func mockdSessions() []apiclient.Session {
	return []apiclient.Session{
		{ID: "s-001", Title: "界面线 spike 讨论", Cwd: "D:/DeepSeek Harness/default-workspace", UpdatedAt: "2026-11-03T05:40:00+08:00", Mode: "default"},
		{ID: "s-002", Title: "UI 插件化架构方案讨论", Cwd: "D:/DeepSeek Harness/default-workspace", UpdatedAt: "2026-11-03T04:10:00+08:00", Mode: "default"},
		{ID: "s-003", Title: "1000 条压力会话", Cwd: "D:/DeepSeek Harness/default-workspace", UpdatedAt: "2026-11-03T06:00:00+08:00", Mode: "default"},
	}
}

// drawSearchList 真实走一遍 `搜索框 → drawList`，返回实际绘制的行与文字。
func drawSearchList(t *testing.T) (*Plugin, *v2RecordingPrimitives, *pluginapi.Env, *gfx.Tree, *gfx.Canvas, *[]measuredText) {
	t.Helper()
	p, rec, env, tree, canvas, texts := searchFixture(t)
	env.Data.Sessions = mockdSessions()
	env.Data.Cur = "s-002"
	env.Data.Busy = true
	env.Act.OpenSession = func(string) {}
	// "e" 命中三条会话 cwd 里的 DeepSeek：走的是与用户打字完全相同的 Field 路径。
	p.search.SetText("e")
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: 268, Height: 400}, func(f ui.C) {
		ui.Column(f).FillHeight().Children(func() { p.drawList(f, env) })
	})
	return p, rec, env, tree, canvas, texts
}

// TestSearchModeDraws48pxResultCards 是这次的主规格：搜索态画出来的必须是
// 官方 `searchResultRow` 的 48px 结果卡，平铺、无分组头、行距 2。
func TestSearchModeDraws48pxResultCards(t *testing.T) {
	p, rec, _, _, _, _ := drawSearchList(t)

	if len(rec.rows) != 3 {
		t.Fatalf("搜索命中三条，实际画了 %d 行", len(rec.rows))
	}
	for i, row := range rec.rows {
		rect := row.box.Rect()
		if row.height != 48 || rect.Height != 48 {
			t.Fatalf("第 %d 张结果卡高度 = %v/%v，官方 .wnFoaW_searchResultRow{min-height:48px}",
				i, row.height, rect.Height)
		}
		if rect.Width != 268 {
			t.Fatalf("第 %d 张结果卡宽度 = %v，want 268（侧栏内宽 = 280-12）", i, rect.Width)
		}
	}
	if len(rec.headers) != 0 {
		t.Fatalf("搜索态是平铺的结果树，不该画工作区分组头，实际画了 %d 个", len(rec.headers))
	}
	if len(rec.rows) > 1 {
		first, second := rec.rows[0].box.Rect(), rec.rows[1].box.Rect()
		if got := second.Y - (first.Y + first.Height); got != 2 {
			t.Fatalf("结果卡间距 = %v，官方 .GAkraW_searchTree>[role=treeitem]+[role=treeitem]{margin-top:2px}", got)
		}
	}

	// 搜索过滤后的可见顺序 = 键盘顺序 = 按最近更新降序
	want := []string{"s-003", "s-001", "s-002"}
	if len(p.lastOrder) != len(want) {
		t.Fatalf("可见顺序 = %v，want %v", p.lastOrder, want)
	}
	for i := range want {
		if p.lastOrder[i] != want[i] {
			t.Fatalf("可见顺序 = %v，want %v", p.lastOrder, want)
		}
	}

	// 选中仍只认真实 Cur；活动点仍只画在当前卡片（Busy 真值）上。
	for i, row := range rec.rows {
		wantActive := p.lastOrder[i] == "s-002"
		if row.active != wantActive {
			t.Fatalf("第 %d 张卡 active = %v，want %v（Cur=s-002）", i, row.active, wantActive)
		}
	}
	if rec.dots != 1 {
		t.Fatalf("结果卡上的活动点 = %d 个，want 1（只有当前会话有 Busy/Stream 真值）", rec.dots)
	}
}

// TestSearchCardsShowRealWorkspaceWithoutInventedSnippet 钉住卡片上写出来的每一个字：
// 只有三条**真标题**和由真实 cwd 推导的 workspace；没有 snippet 真值时一个字都不许补。
func TestSearchCardsShowRealWorkspaceWithoutInventedSnippet(t *testing.T) {
	_, _, _, _, _, texts := drawSearchList(t)

	got := map[string]float32{}
	for _, m := range *texts {
		got[m.value] = m.size
	}
	want := map[string]float32{
		"界面线 spike 讨论":      14, // 官方 searchResultTitle{font-size:14px}
		"UI 插件化架构方案讨论":      14,
		"1000 条压力会话":        14,
		"default-workspace": 12, // 官方 searchResultWorkspace{font-size:12px}，来自真实 cwd
	}
	for value, size := range want {
		s, ok := got[value]
		if !ok {
			t.Fatalf("卡片上没有画 %q；实际画了 %v", value, keysOf(got))
		}
		if s != size {
			t.Fatalf("%q 的字号 = %v，官方是 %v", value, s, size)
		}
	}
	// 多出来的只能是省略号截断的中间候选（文字引擎会试若干前缀），
	// 任何**不是**真值前缀的东西都说明卡片自己补了内容（假 snippet / 假时间）。
	for value := range got {
		if !prefixOfAny(strings.TrimSuffix(value, "…"), want) {
			t.Fatalf("卡片上多画了内容（没有真值就不许补）：%q；全部实测 %v", value, keysOf(got))
		}
	}
}

// prefixOfAny 判断 s 是不是 want 里某个真值字符串的前缀：省略号截断只会产生前缀。
func prefixOfAny(s string, want map[string]float32) bool {
	if s == "" {
		return true // 省略号本身（"…"）会被量一次
	}
	for value := range want {
		if strings.HasPrefix(value, s) {
			return true
		}
	}
	return false
}

// TestSearchNoMatchesUsesOfficialEmptyText：官方空态是 `.GAkraW_empty{font-size:13px}`
// 配 `search.noMatches`「无匹配会话」。
func TestSearchNoMatchesUsesOfficialEmptyText(t *testing.T) {
	p, rec, env, _, canvas, texts := searchFixture(t)
	env.Data.Sessions = mockdSessions()
	p.search.SetText("zzz-不存在的词")
	pageTheme2 := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme2})
	view.Draw(canvas, gfx.Rect{Width: 268, Height: 400}, func(f ui.C) {
		ui.Column(f).FillHeight().Children(func() { p.drawList(f, env) })
	})
	if len(rec.rows) != 0 || len(p.lastOrder) != 0 {
		t.Fatalf("没有命中时不该画结果卡：rows=%d order=%v", len(rec.rows), p.lastOrder)
	}
	found := false
	for _, m := range *texts {
		if m.value == "无匹配会话" && m.size == 13 {
			found = true
		}
	}
	if !found {
		t.Fatalf("没有命中时应显示官方 search.noMatches「无匹配会话」/13px，实际画了 %v", *texts)
	}
}

// TestSearchCardClickUsesRealSessionID：点击必须经真实命中区域落到原始会话 ID，
// 图元收到的 ID 与分组态完全一致（`s-<sessionID>`），键盘与点击共用一条路。
func TestSearchCardClickUsesRealSessionID(t *testing.T) {
	p, _, env, tree, canvas, _ := searchFixture(t)
	var opened string
	env.Data.Sessions = mockdSessions()
	env.Data.Cur = "s-001"
	env.Act.OpenSession = func(id string) { opened = id }
	p.search.SetText("e")
	pageTheme3 := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme3})
	draw := func() {
		view.Draw(canvas, gfx.Rect{Width: 268, Height: 400}, func(f ui.C) {
			ui.Column(f).FillHeight().Children(func() { p.drawList(f, env) })
		})
	}
	draw()
	canvas.Reset()
	canvas.BeginEventPass(tree.ID(env.Feat, "s-s-003"), gfx.ClickInfo{})
	draw()
	canvas.EndEventPass()
	if opened != "s-003" {
		t.Fatalf("点击第一张结果卡后 OpenSession = %q，want s-003", opened)
	}
}

// TestSearchResultsKeepTruthOnly 钉住数据面：
//   - 标题 / 工作区名只来自 `Session.Title` 与真实 `Cwd`，空值用官方兜底词；
//   - snippet **永远没有真值**：`apiclient.Session` 里没有这个字段，
//     `pluginapi.Snapshot` 也没有逐会话片段（官方只有 Host 全文检索结果页才给 snippet），
//     所以 HasSnippet 一直是 false —— 这是契约缺口，不是"暂时没数据"。
func TestSearchResultsKeepTruthOnly(t *testing.T) {
	sessions := []apiclient.Session{
		{ID: "a", Title: "", Cwd: `D:\work\alpha`, UpdatedAt: "2026-11-03T10:00:00Z"},
		{ID: "b", Title: "Beta", Cwd: "", UpdatedAt: "2026-11-03T11:00:00Z"},
		{ID: "c", Title: "Gamma", Cwd: `D:\work\alpha`, UpdatedAt: "2026-11-03T12:00:00Z"},
	}

	all := searchResults(sessions, "")
	if len(all) != 3 || all[0].ID != "c" || all[1].ID != "b" || all[2].ID != "a" {
		t.Fatalf("结果顺序 = %v，want 按最近更新降序 c/b/a", idsOfResults(all))
	}
	if all[0].Title != "Gamma" || all[0].Workspace != "alpha" {
		t.Fatalf("c 的标题/工作区 = %q/%q，want Gamma/alpha（真实 cwd 末段）", all[0].Title, all[0].Workspace)
	}
	if all[1].Workspace != "未分组" { // 官方 group.ungrouped
		t.Fatalf("没有 cwd 的会话工作区 = %q，want 官方 group.ungrouped「未分组」", all[1].Workspace)
	}
	if all[2].Title != "未命名" { // 官方 session.untitled
		t.Fatalf("空标题 = %q，want 官方 session.untitled「未命名」", all[2].Title)
	}
	for _, r := range all {
		if r.HasSnippet || r.Snippet != "" {
			t.Fatalf("%s 竟然有 snippet 真值 %q：宿主的 session.list/snapshot 里没有这个字段", r.ID, r.Snippet)
		}
	}

	for _, q := range []string{"alpha", "ALPHA", "  alpha  "} {
		got := searchResults(sessions, normalizeQuery(q))
		if len(got) != 2 || got[0].ID != "c" || got[1].ID != "a" {
			t.Fatalf("过滤 %q 的命中 = %v，want 仅 cwd 命中的 c/a（顺序按最近更新）", q, idsOfResults(got))
		}
	}
	if got := searchResults(sessions, "zzz"); len(got) != 0 {
		t.Fatalf("无命中时结果 = %v，want 空", idsOfResults(got))
	}
}

// TestSearchResultCardInnerGeometry 用真实 flex 矩形量卡片内部：
// 标题行 20、元信息行 17、元信息左缩进 20（对齐标题文字）、
// workspace 上限 = 只有它自己时 100%（官方 `:only-child`）、与片段并排时 40%。
func TestSearchResultCardInnerGeometry(t *testing.T) {
	draw := func(hasSnippet bool) searchCardBoxes {
		p, _, env, _, canvas, _ := searchFixture(t)
		env.Data.Cur = "x"
		env.Act.OpenSession = func(string) {}
		results := searchResults([]apiclient.Session{
			sess("x", "一条真标题", `D:\work\a-very-long-workspace-label-beyond-forty-percent`, "2026-11-03T10:00:00Z"),
		}, "")
		if len(results) != 1 {
			t.Fatalf("结果数 = %d，want 1", len(results))
		}
		results[0].Snippet, results[0].HasSnippet = "一段真实片段", hasSnippet
		var boxes searchCardBoxes
		pageTheme4 := dscore.VivaTheme(p.theme)
		view := ui.NewPage(ui.PageConfig{Theme: &pageTheme4})
		view.Draw(canvas, gfx.Rect{Width: 268, Height: 200}, func(f ui.C) {
			ui.Column(f).Width(268).Children(func() { boxes = p.drawSearchCard(f, env, results[0]) })
		})
		return boxes
	}

	boxes := draw(false)
	card := boxes.Card.Rect()
	// 这几个数字是 docs/visual-search-result.md 几何表里的"设计值"（flex 真实矩形），
	// 与无头截图的"实测值"分开记，别互相冒充。
	t.Logf("design card=%+v heading=%+v meta.y=%v meta.h=%v title.x=%v meta.x=%v ws.w=%v 底留白=%v",
		card, boxes.Heading.Rect(), boxes.Meta.Rect().Y, boxes.Meta.Rect().Height,
		boxes.Title.Rect().X-card.X, boxes.Meta.Rect().X-card.X, boxes.Workspace.Rect().Width,
		card.Y+card.Height-(boxes.Meta.Rect().Y+boxes.Meta.Rect().Height))
	if card.Height != 48 {
		t.Fatalf("卡片高 %v，官方 min-height:48px", card.Height)
	}
	if got := boxes.Heading.Rect().Height; got != 20 {
		t.Fatalf("标题行高 %v，官方 searchResultTitle{line-height:20px}", got)
	}
	if got := boxes.Meta.Rect().Height; got != 17 {
		t.Fatalf("元信息行高 %v，官方 searchResultWorkspace/Snippet{line-height:17px}", got)
	}
	if got := boxes.Meta.Rect().Y - boxes.Heading.Rect().Y; got != 20 {
		t.Fatalf("标题行到元信息行 = %v，want 20（两行紧挨）", got)
	}
	// 标题左边 = 卡片内边距 8 + 图标槽 16 + 官方 .searchResultTitle{margin-left:4px}
	if got := boxes.Title.Rect().X - card.X; got != 28 {
		t.Fatalf("标题左偏移 = %v，want 28（8+16+4）", got)
	}
	// 元信息左边 = 卡片内边距 8 + 官方 .searchResultMeta{margin-left:20px}
	if got := boxes.Meta.Rect().X - card.X; got != 28 {
		t.Fatalf("元信息左偏移 = %v，want 28（8+20）", got)
	}
	if got := card.Y + card.Height - (boxes.Meta.Rect().Y + boxes.Meta.Rect().Height); got < 4 {
		t.Fatalf("卡片底部留白 = %v，官方 padding-bottom 4px", got)
	}
	// 没有片段真值时工作区名是这一行的独子：官方 :only-child → max-width:100%
	if got, metaW := boxes.Workspace.Rect().Width, boxes.Meta.Rect().Width; got > metaW+1 {
		t.Fatalf("独子工作区名宽 %v 超过元信息行宽 %v（官方 :only-child{max-width:100%%}）", got, metaW)
	}
	if boxes.HasSnippet {
		t.Fatalf("没有 snippet 真值时不该画片段行")
	}

	// 有片段时：工作区名缩到 40%（`max-width:40%`），片段吃掉剩下的宽度。
	withSnippet := draw(true)
	metaW := withSnippet.Meta.Rect().Width
	if got := withSnippet.Workspace.Rect().Width; got > 0.4*metaW+1 {
		t.Fatalf("与片段并排的工作区名宽 = %v（元信息行宽 %v），官方 max-width:40%%", got, metaW)
	}
	if !withSnippet.HasSnippet {
		t.Fatalf("HasSnippet=true 时应有片段盒子")
	}
	if got := withSnippet.Snippet.Rect().X; got <= withSnippet.Workspace.Rect().X {
		t.Fatalf("片段应排在工作区名右边：workspace.x=%v snippet.x=%v",
			withSnippet.Workspace.Rect().X, got)
	}
}

// TestSearchCardSnippetRendersOnlyWithTruth：渲染路径本身是通的——
// 一旦哪天宿主给了真值（HasSnippet=true），片段就画在元信息行上；
// 没有真值时整段不画、不给占位。
func TestSearchCardSnippetRendersOnlyWithTruth(t *testing.T) {
	draw := func(has bool) (searchCardBoxes, map[string]float32) {
		p, _, env, _, canvas, texts := searchFixture(t)
		env.Act.OpenSession = func(string) {}
		r := searchResult{ID: "x", Title: "标题", Workspace: "alpha", Snippet: "真实片段", HasSnippet: has}
		var boxes searchCardBoxes
		pageTheme5 := dscore.VivaTheme(p.theme)
		view := ui.NewPage(ui.PageConfig{Theme: &pageTheme5})
		view.Draw(canvas, gfx.Rect{Width: 268, Height: 200}, func(f ui.C) {
			ui.Column(f).Width(268).Children(func() { boxes = p.drawSearchCard(f, env, r) })
		})
		got := map[string]float32{}
		for _, m := range *texts {
			got[m.value] = m.size
		}
		return boxes, got
	}

	boxes, texts := draw(true)
	if !boxes.HasSnippet {
		t.Fatalf("HasSnippet=true（宿主给了真值）时卡片应画出片段行")
	}
	if size, ok := texts["真实片段"]; !ok || size != 12 {
		t.Fatalf("片段没有按官方 12px 画出来：%v", texts)
	}
	if boxes.Snippet.Rect().Height != 17 {
		t.Fatalf("片段行高 = %v，官方 line-height:17px", boxes.Snippet.Rect().Height)
	}

	boxes, texts = draw(false)
	if boxes.HasSnippet {
		t.Fatalf("HasSnippet=false 时不许画片段行")
	}
	if _, ok := texts["真实片段"]; ok {
		t.Fatalf("没有真值时片段被画出来了：%v", texts)
	}
}

func idsOfResults(rs []searchResult) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.ID)
	}
	return out
}

// keysOf 只用来在失败信息里把 map 排好序，避免每次跑出来的顺序不一样。
func keysOf(m map[string]float32) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
