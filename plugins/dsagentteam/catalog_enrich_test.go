// catalog_enrich_test.go —— 目录行**指标/展开/当前会话**的证据（#56 补齐面）。
//
// 判据全部来自官方 `SubagentHeaderLineage.tsx` 的纯函数与 `locales.ts` 的 zh 段：
//
//	formatTokens      :69-77    （<1000 原样；<1e6 → K；否则 M）
//	formatDuration    :130-160  （多档；判定顺序不许重排）
//	formatExactDuration :162-178
//	canExpandCatalog  :209-213  （mode ≠ external ∧ 子目录有行）
//	currentCheck      :403-426  （当前会话画 ✓）
package dsagentteam

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func ptrInt64(v int64) *int64 { return &v }
func ptrBool(v bool) *bool    { return &v }

func TestCatalogFormatTokensFollowsOfficialRounds(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 tok"},
		{999, "999 tok"},
		{1000, "1K tok"},
		{1500, "1.5K tok"},
		{99_500, "99.5K tok"},
		{100_000, "100K tok"},
		{1_500_000, "1.5M tok"},
		{12_000_000, "12M tok"},
	}
	for _, tc := range cases {
		if got := catalogTokenMetric(tc.in); got != tc.want {
			t.Fatalf("catalogTokenMetric(%d) = %q，want %q", tc.in, got, tc.want)
		}
	}
}

func TestCatalogDurationCompactFollowsOfficialLadder(t *testing.T) {
	cases := []struct {
		ms   int64
		want string
	}{
		{500, "0秒"},
		{5_000, "5秒"},
		{65_000, "1分05秒"},
		{3_600_000, "1小时00分00秒"},
		{3_661_000, "1小时01分01秒"},
		{90_000_000, "1天1小时"},
		{86_400_000, "1天"},
		// ≥30 天走"约 N 个月"，余天非零时带"天"。
		{45 * 86_400_000, "约1个月15天"},
		{30 * 86_400_000, "约1个月"},
		// ≥365 天走"约 N 年"，余月非零时带"个月"。
		{400 * 86_400_000, "约1年1个月"},
		{365 * 86_400_000, "约1年"},
	}
	for _, tc := range cases {
		if got := catalogDurationCompact(tc.ms); got != tc.want {
			t.Fatalf("catalogDurationCompact(%d) = %q，want %q", tc.ms, got, tc.want)
		}
	}
}

func TestCatalogDurationExactUsesFullDays(t *testing.T) {
	// 不足一天：复用 compact（官方 formatExactDuration 的 days===0 分支）。
	if got := catalogDurationExact(3_661_000); got != "1小时01分01秒" {
		t.Fatalf("不足一天应复用 compact，实得 %q", got)
	}
	if got := catalogDurationExact(90_000_000); got != "1天01小时00分00秒" {
		t.Fatalf("≥1 天应走 exactDays，实得 %q", got)
	}
	if got := catalogDurationExact(0); got != "0秒" {
		t.Fatalf("0 应画 0秒（settledMs=0 由调用方过滤），实得 %q", got)
	}
}

func TestCatalogCanExpandMirrorsOfficialGate(t *testing.T) {
	entry := CatalogEntry{ID: "child-1", Mode: "continuable"}
	withRows := ChildProjection{CatalogKnown: true, Catalog: []CatalogEntry{{ID: "grand-1"}}}
	if !catalogCanExpand(entry, withRows) {
		t.Fatal("可继续 + 子目录有行 → 必须可展开")
	}
	if catalogCanExpand(entry, ChildProjection{CatalogKnown: true}) {
		t.Fatal("子目录是空 → 不可展开")
	}
	if catalogCanExpand(entry, ChildProjection{}) {
		t.Fatal("子目录未知 → 不可展开（未知不是空）")
	}
	external := CatalogEntry{ID: "ext", Mode: "external"}
	if catalogCanExpand(external, withRows) {
		t.Fatal("external 行按官方第一条判据不可展开")
	}
}

func TestCatalogReserveDisclosureReservesForSiblings(t *testing.T) {
	entries := []CatalogEntry{{ID: "a"}, {ID: "b"}}
	children := map[string]ChildProjection{
		"b": {CatalogKnown: true, Catalog: []CatalogEntry{{ID: "grand"}}},
	}
	if !catalogReserveDisclosure(entries, children) {
		t.Fatal("层内有可展开行 → 兄弟叶子行必须让出展开列")
	}
	if catalogReserveDisclosure([]CatalogEntry{{ID: "a"}}, nil) {
		t.Fatal("没有可展开行时不该留展开列")
	}
}

func TestSubagentTimingDurationFollowsOfficialActivityDuration(t *testing.T) {
	// 没有投影：不画。
	var nilTiming *SubagentTiming
	if _, ok := nilTiming.DurationMs(0); ok {
		t.Fatal("nil 投影必须返回 false（不画时长）")
	}
	// 只有 settledMs。
	settled := &SubagentTiming{SettledMs: 65_000}
	if ms, ok := settled.DurationMs(0); !ok || ms != 65_000 {
		t.Fatalf("settledMs 分支 = (%d,%v)，want (65000,true)", ms, ok)
	}
	// 有未闭合轮：settled + (min(now,through) − since)。
	active := &SubagentTiming{SettledMs: 1_000, Active: true, ActiveSince: 5_000, ActiveThrough: 9_000}
	if ms, ok := active.DurationMs(20_000); !ok || ms != 5_000 {
		t.Fatalf("active 已闭合分支 = (%d,%v)，want (5000,true)（用 through 而不是 now）", ms, ok)
	}
	active.ActiveThrough = 0
	if ms, ok := active.DurationMs(20_000); !ok || ms != 16_000 {
		t.Fatalf("active 进行中分支 = (%d,%v)，want (16000,true)", ms, ok)
	}
	// 全零：不算"0 秒"，返回 false。
	if _, ok := (&SubagentTiming{}).DurationMs(0); ok {
		t.Fatal("没有任何时长真值时必须返回 false")
	}
}

func TestCatalogRowMetricsSkipUnknown(t *testing.T) {
	child := ChildProjection{
		TokenTotal: ptrInt64(1500),
		Timing:     &SubagentTiming{SettledMs: 65_000},
	}
	token, compact, exact := catalogRowMetrics(child, 0)
	if token != "1.5K tok" || compact != "1分05秒" {
		t.Fatalf("指标 = %q / %q", token, compact)
	}
	if exact != "总活跃耗时：1分05秒" {
		t.Fatalf("exact 标题 = %q，官方 `duration.exactTitle`", exact)
	}
	// 未知就是空串：不画 0 tok、不画 0秒。
	if token, compact, _ := catalogRowMetrics(ChildProjection{}, 0); token != "" || compact != "" {
		t.Fatalf("未知子投影不该有指标：%q / %q", token, compact)
	}
}

func TestCatalogBranchExpandedStateFollowsStore(t *testing.T) {
	s := NewStore(nil)
	view := catalogReadyView()
	view.Children = map[string]ChildProjection{
		"child-1": {CatalogKnown: true, Catalog: []CatalogEntry{{ID: "grand-1"}}},
		"grand-1": {CatalogKnown: true, Catalog: []CatalogEntry{{ID: "leaf-1"}}},
	}
	s.SetView(view)

	s.ToggleCatalogBranch("child-1")
	if !s.CatalogExpanded("child-1") {
		t.Fatal("toggle 后必须记录展开")
	}
	// 展开下级再收起上级：整支都要清掉（官方 closeBranch 递归）。
	s.ToggleCatalogBranch("grand-1")
	s.ToggleCatalogBranch("child-1")
	if s.CatalogExpanded("child-1") || s.CatalogExpanded("grand-1") {
		t.Fatal("收起上级必须递归清掉下级展开态")
	}
	// 关闭目录清空全部（官方 changeOpen(false) 的 setExpanded(new Set())）。
	s.ToggleCatalogBranch("child-1")
	s.ResetCatalogExpanded()
	if s.CatalogExpanded("child-1") {
		t.Fatal("ResetCatalogExpanded 必须清空")
	}
}

// renderCatalogPanelWith 用自定 env 渲染一帧 shell.overlay（为了喂 Cur/Sessions）。
func renderCatalogPanelWith(t *testing.T, k *uikernel.Kernel, mutate func(*pluginapi.Env)) *gfx.Canvas {
	return renderCatalogPanelClick(t, k, mutate, "")
}

// renderCatalogPanelClick 同上，并在这一帧里按 `clickName` 造一次点击
// （交互 id 由插件同一条 `env.Tree.ID(env.Feat, name)` 公式算出）。
func renderCatalogPanelClick(t *testing.T, k *uikernel.Kernel, mutate func(*pluginapi.Env), clickName string) *gfx.Canvas {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("catalog-enrich")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	bounds := gfx.Rect{Width: 1180, Height: 800}
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds, MainWidth: 900,
		Has:      k.Has,
		Geometry: pluginapi.Geometry{Width: bounds.Width, Height: bounds.Height, SidebarWidth: 280},
	}
	if mutate != nil {
		mutate(env)
	}
	if clickName != "" {
		canvas.BeginEventPass(tree.ID(feat, clickName), gfx.ClickInfo{})
	}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, bounds, func(f ui.C) {
		k.DrawSlot(f, pluginapi.SlotShellOverlay, env)
	})
	if clickName != "" {
		canvas.EndEventPass()
	}
	return canvas
}

func canvasHasText(canvas *gfx.Canvas, want string) bool {
	for _, text := range canvas.Texts() {
		if strings.Contains(text.Value, want) {
			return true
		}
	}
	return false
}

// TestCatalogRowDrawsMetricsAndCurrentCheck 是这一批的主规格：
// 子会话自己的投影（token/时长）与当前会话对勾必须真画到面板上，
// 而"下一层目录"要有 disclosure，展开后递归画出孙行。
func TestCatalogRowDrawsMetricsAndCurrentCheck(t *testing.T) {
	p := newTestPlugin(nil)
	k, _ := catalogChain(t, p)

	view := catalogReadyView()
	view.Children = map[string]ChildProjection{
		"child-1": {
			CatalogKnown: true,
			Catalog:      []CatalogEntry{{ID: "grand-1", Label: "孙任务", Mode: "continuable"}},
			Timing:       &SubagentTiming{SettledMs: 65_000},
			TokenTotal:   ptrInt64(1500),
		},
	}
	p.team.SetView(view)
	p.OpenCatalog()

	canvas := renderCatalogPanelWith(t, k, func(env *pluginapi.Env) {
		env.Data.Cur = "child-1"
		env.Data.Sessions = []apiclient.Session{{ID: "child-2", Title: "第二个子代理的标题"}}
	})
	for _, want := range []string{"1.5K tok", "1分05秒", "✓", "第二个子代理的标题"} {
		if !canvasHasText(canvas, want) {
			t.Fatalf("面板上找不到 %q（指标/当前会话对勾/子会话真实标题）", want)
		}
	}
	// 未展开时不该画孙行。
	if canvasHasText(canvas, "孙任务") {
		t.Fatal("未展开就在画下一层")
	}
	// 展开后递归画出下一层（官方 CatalogRows level+1）。
	p.team.ToggleCatalogBranch("child-1")
	expanded := renderCatalogPanelWith(t, k, nil)
	if !canvasHasText(expanded, "孙任务") {
		t.Fatal("展开后必须画出下一层子会话")
	}
	// 外部任务行不画 disclosure（官方第一条判据；与 TestCatalogCanExpandMirrorsOfficialGate 同源）。
	if catalogCanExpand(CatalogEntry{ID: "x", Mode: "external"}, ChildProjection{CatalogKnown: true, Catalog: []CatalogEntry{{ID: "y"}}}) {
		t.Fatal("external 行不该可展开")
	}
}

// TestCatalogEscClosesPanel 钉住 Esc：目录开着时消费该键并关闭（官方
// document keydown 的 Escape，OFF:…tsx:649-653）。
func TestCatalogEscClosesPanel(t *testing.T) {
	p := newTestPlugin(nil)
	p.team.SetView(catalogReadyView())
	p.OpenCatalog()
	if !p.catalog.isOpen() {
		t.Fatal("前置：目录应已打开")
	}
	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) {
		t.Fatal("Esc 必须被消费（否则会漏给别的插件）")
	}
	if p.catalog.isOpen() {
		t.Fatal("Esc 后目录必须关闭")
	}
	// 目录没开时 Esc 不消费（不抢别的插件的键）。
	if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) {
		t.Fatal("目录没开时不该消费 Esc")
	}
	// 带修饰键、以及非 Esc 的键一律不消费。
	p.OpenCatalog()
	if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape, Modifiers: gfx.ModifierShift}) {
		t.Fatal("带修饰键时不该消费")
	}
	if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyDown}) {
		t.Fatal("方向键不在本轮范围（登记为差异），不该消费")
	}
	if !p.catalog.isOpen() {
		t.Fatal("不该被误关")
	}
}

// TestCatalogDisclosureClickExpandsWithoutOpening 钉住命中区层级：
// 点 disclosure 只展开/收起，**不能**顺带打开子会话（官方的 disclosure 是行内
// 独立 button，点击 stopPropagation）。
func TestCatalogDisclosureClickExpandsWithoutOpening(t *testing.T) {
	p := newTestPlugin(nil)
	k, _ := catalogChain(t, p)
	view := catalogReadyView()
	view.Children = map[string]ChildProjection{
		"child-1": {CatalogKnown: true, Catalog: []CatalogEntry{{ID: "grand-1", Label: "孙任务"}}},
	}
	p.team.SetView(view)
	p.OpenCatalog()

	opened := ""
	renderCatalogPanelClick(t, k, func(env *pluginapi.Env) {
		env.Act.OpenSession = func(id string) { opened = id }
	}, "subagent-catalog-branch-child-1")

	if !p.team.CatalogExpanded("child-1") {
		t.Fatal("点 disclosure 后必须记录展开")
	}
	if opened != "" {
		t.Fatalf("点 disclosure 不该打开子会话，实得 %q", opened)
	}
	// 再点一次是收起。
	renderCatalogPanelClick(t, k, nil, "subagent-catalog-branch-child-1")
	if p.team.CatalogExpanded("child-1") {
		t.Fatal("再点 disclosure 应收起")
	}
}
