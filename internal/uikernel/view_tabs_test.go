//go:build darwin || windows || linux

package uikernel

// view_tabs_test.go —— 两个「多视图座位」机制的机器断言。
//
// 断言分五类：
//
//	A. 名录投影：同 key 合并成一条、按 (Order, 装配顺序) 排、空 key 不进名录
//	B. 激活判据：官方 `resolveActiveView` / `defaultSeed` 的逐条分支
//	C. keyed 派发：只画 key 命中的那几份；key 空 = 什么都不画（**不是**全画一遍）
//	D. 几何锚定：芯片数值 = 官方 dockkit CSS 里那几个数
//	E. 真画一遍：骨架按 ActiveView / ActiveRightTab 派发；点芯片会走选择动作

import (
	"fmt"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// ── 测试替身 ────────────────────────────────────────────────────────

// tabPlugin 是一个只挂画的最小插件。
type tabPlugin struct {
	id       string
	panels   []pluginapi.Painter
	provides []string
	requires []string
}

func (p *tabPlugin) ID() string                       { return p.id }
func (p *tabPlugin) Provides() []string               { return p.provides }
func (p *tabPlugin) Requires() []string               { return p.requires }
func (p *tabPlugin) Register(pluginapi.Context) error { return nil }
func (p *tabPlugin) Panels() []pluginapi.Painter      { return p.panels }

// noopDraw 是一块什么都不画的占位（名录投影只看 Key/Label/Order）。
func noopDraw(ui.C, *pluginapi.Env) {}

// loadTabs 装一组只挂画的最小插件。
func loadTabs(t *testing.T, panels ...pluginapi.Painter) *Kernel {
	t.Helper()
	k := New("view-tabs-test")
	for i, p := range panels {
		one := &tabPlugin{id: fmt.Sprintf("tab-%d", i), panels: []pluginapi.Painter{p}}
		if err := k.Load(one); err != nil {
			t.Fatalf("装第 %d 块画失败：%v", i, err)
		}
	}
	return k
}

// ── A. 名录投影 ─────────────────────────────────────────────────────

func TestViewTabsMergesSameKeyAndSortsByOrder(t *testing.T) {
	k := loadTabs(t,
		// 同一 key 的两块画（本仓 chat 由 ds-conversation 与 ds-agent-team 各画一段）。
		pluginapi.Painter{ID: "a", Slot: pluginapi.SlotConvView, Key: pluginapi.ViewChat,
			Label: "对话", Order: 0, Draw: noopDraw},
		pluginapi.Painter{ID: "b", Slot: pluginapi.SlotConvView, Key: pluginapi.ViewChat,
			Order: 80, Draw: noopDraw},
		pluginapi.Painter{ID: "c", Slot: pluginapi.SlotConvView, Key: pluginapi.ViewTrajectory,
			Label: "轨迹", Order: 10, Draw: noopDraw},
	)
	got := ViewTabs(k)
	if len(got) != 2 {
		t.Fatalf("chat 的两块画应当合成一条名录项，实际 %d 条：%+v", len(got), got)
	}
	if got[0].Key != pluginapi.ViewChat || got[1].Key != pluginapi.ViewTrajectory {
		t.Fatalf("名录顺序 = %q/%q，官方是 chat(0) 在 trajectory(10) 前", got[0].Key, got[1].Key)
	}
	if got[0].Label != "对话" {
		t.Fatalf("同 key 的 label 应当取第一条非空的，实际 %q", got[0].Label)
	}
}

func TestRightTabsOrderIsRegistrationOrderAtSameOrder(t *testing.T) {
	// 同 Order 时按装配顺序（官方 `defaultSeed`/名录也是"同序按注册顺序"）。
	k := loadTabs(t,
		pluginapi.Painter{ID: "first", Slot: pluginapi.SlotRightPaneTab, Key: "one",
			Label: "一", Order: 5, Draw: noopDraw},
		pluginapi.Painter{ID: "second", Slot: pluginapi.SlotRightPaneTab, Key: "two",
			Label: "二", Order: 5, Draw: noopDraw},
	)
	got := RightTabs(k)
	if len(got) != 2 || got[0].Key != "one" || got[1].Key != "two" {
		t.Fatalf("同 Order 应当保持装配顺序，实际 %+v", got)
	}
}

func TestTabRosterSkipsEmptyKey(t *testing.T) {
	k := loadTabs(t,
		pluginapi.Painter{ID: "nokey", Slot: pluginapi.SlotRightPaneTab, Label: "没 key", Draw: noopDraw},
		pluginapi.Painter{ID: "ok", Slot: pluginapi.SlotRightPaneTab, Key: "k", Label: "有 key", Draw: noopDraw},
	)
	if got := RightTabs(k); len(got) != 1 || got[0].Key != "k" {
		t.Fatalf("没 key 的画不该进名录，实际 %+v", got)
	}
}

func TestNilKernelHasNoRoster(t *testing.T) {
	if got := ViewTabs(nil); got != nil {
		t.Fatalf("nil 内核应当返回空名录，实际 %+v", got)
	}
	if got := RightTabs(nil); got != nil {
		t.Fatalf("nil 内核应当返回空名录，实际 %+v", got)
	}
}

// ── B. 激活判据 ─────────────────────────────────────────────────────

func TestResolveActiveViewFollowsOfficialRule(t *testing.T) {
	roster := []pluginapi.Tab{
		{Key: pluginapi.ViewChat, Label: "对话"},
		{Key: pluginapi.ViewTrajectory, Label: "轨迹"},
	}
	cases := []struct {
		name     string
		selected string
		want     string
		wantOK   bool
	}{
		{"存着的偏好命中就用它", pluginapi.ViewTrajectory, pluginapi.ViewTrajectory, true},
		{"偏好失效落到 chat", "unloaded-plugin-view", pluginapi.ViewChat, true},
		{"没有偏好落到 chat", "", pluginapi.ViewChat, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ResolveActiveView(roster, tc.selected)
			if ok != tc.wantOK || got.Key != tc.want {
				t.Fatalf("ResolveActiveView(%q) = %q/%v，want %q/%v", tc.selected, got.Key, ok, tc.want, tc.wantOK)
			}
		})
	}
	// 官方明文："without choosing another View"——没有 chat 时**不许**随手挑一个。
	if got, ok := ResolveActiveView([]pluginapi.Tab{{Key: "stats"}}, ""); ok {
		t.Fatalf("名录里没有 chat 时不该选 %q", got.Key)
	}
}

func TestResolveActiveRightTabFollowsDefaultSeed(t *testing.T) {
	one := []pluginapi.Tab{{Key: "timeline", Label: "轨迹"}}
	if got, ok := ResolveActiveRightTab(one, ""); !ok || got.Key != "timeline" {
		t.Fatalf("恰好一个条目时应当直接上它，实际 %q/%v", got.Key, ok)
	}
	tabs := []pluginapi.Tab{{Key: "timeline", Order: 10}, {Key: "terminal", Order: 20}}
	if got, ok := ResolveActiveRightTab(tabs, "terminal"); !ok || got.Key != "terminal" {
		t.Fatalf("命中的偏好应当优先，实际 %q", got.Key)
	}
	if got, ok := ResolveActiveRightTab(tabs, "gone"); !ok || got.Key != "timeline" {
		t.Fatalf("偏好失效应当落到名录第一条，实际 %q", got.Key)
	}
	if _, ok := ResolveActiveRightTab(nil, ""); ok {
		t.Fatal("空名录应当报「没有可画的 tab」")
	}
}

// ── C. keyed 派发 ───────────────────────────────────────────────────

func TestDrawSlotKeyedOnlyDrawsMatchingKey(t *testing.T) {
	var drawn []string
	k := loadTabs(t,
		pluginapi.Painter{ID: "chat", Slot: pluginapi.SlotConvView, Key: pluginapi.ViewChat,
			Draw: func(ui.C, *pluginapi.Env) { drawn = append(drawn, "chat") }},
		pluginapi.Painter{ID: "traj", Slot: pluginapi.SlotConvView, Key: pluginapi.ViewTrajectory,
			Draw: func(ui.C, *pluginapi.Env) { drawn = append(drawn, "traj") }},
	)
	drawKeyed(t, k, pluginapi.SlotConvView, pluginapi.ViewTrajectory)
	if strings.Join(drawn, ",") != "traj" {
		t.Fatalf("keyed 派发只该画命中的那一份，实际 %v", drawn)
	}
	drawn = nil
	// key 空 = 什么都没命中。**不是**"全画一遍"：回退会让两个 view 画进同一块矩形。
	drawKeyed(t, k, pluginapi.SlotConvView, "")
	if len(drawn) != 0 {
		t.Fatalf("空 key 不该画任何一块，实际 %v", drawn)
	}
	if !k.HasKey(pluginapi.SlotConvView, pluginapi.ViewChat) || k.HasKey(pluginapi.SlotConvView, "gone") {
		t.Fatal("HasKey 判据不对")
	}
}

func TestKeyedOrderIsPaintOrder(t *testing.T) {
	var drawn []string
	k := loadTabs(t,
		pluginapi.Painter{ID: "late", Slot: pluginapi.SlotConvView, Key: "k", Order: 80,
			Draw: func(ui.C, *pluginapi.Env) { drawn = append(drawn, "late") }},
		pluginapi.Painter{ID: "early", Slot: pluginapi.SlotConvView, Key: "k", Order: 0,
			Draw: func(ui.C, *pluginapi.Env) { drawn = append(drawn, "early") }},
	)
	drawKeyed(t, k, pluginapi.SlotConvView, "k")
	if strings.Join(drawn, ",") != "early,late" {
		t.Fatalf("同 key 的多块画按 Order 升序，实际 %v", drawn)
	}
}

// drawKeyed 在**真的布局游标**上调一次 keyed 派发。
//
// 不手造 `ui.C`（它有未导出字段，构造不出来），也不传 nil：viva 的 `ui.C` 是
// 布局游标的句柄，脚本里要拿到它只有一条路——在 `view.Draw` 的回调里。
func drawKeyed(t *testing.T, k *Kernel, slot pluginapi.Slot, key string) {
	t.Helper()
	env, canvas := newSkeletonEnv(640, 200, func(pluginapi.Slot) bool { return false })
	pageTheme := dscore.VivaTheme(dscore.Dark())
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		k.DrawSlotKeyed(f, slot, key, env)
	})
}

// ── D. 几何锚定 ─────────────────────────────────────────────────────

func TestRightTabStripGeometryMatchesOfficialDockkitCSS(t *testing.T) {
	// 全部来自 dsh-web-frontend/dist/assets/index-CSAdOBx6.css 的 `_123xe_` 模块
	// （dockkit 的源码包不在随包 src/** 里，只有这份编译产物）。
	// 会话侧 CSS 的注释独立佐证了 38 这个数
	// （dsh-client-ui-conversation/src/client/skeleton/ConversationRoot.module.css:17-23）。
	cases := []struct {
		name string
		got  float32
		want float32
	}{
		{".tabStrip{height:28px}", RightTabStripRowHeight, 28},
		{".tabStrip{padding:10px … 0 …}", RightTabStripPadTop, 10},
		{".tabStrip{gap:4px}", RightTabStripGap, 4},
		{".tabStrip{padding:… 6px 0 …}", RightTabStripPadRight, 6},
		{"--dsh-dockkit-strip-inline-start 缺省 10px", RightTabStripPadLeft, 10},
		{".tab{min-width:80px}", RightTabChipMinWidth, 80},
		{".tab{max-width:170px}", RightTabChipMaxWidth, 170},
		{".tab{height:28px}", RightTabChipHeight, 28},
		{".tab{padding:0 10px}", RightTabChipPadX, 10},
		{".tab{font-size:13px}", RightTabChipFontSize, 13},
		{".tabTitle{gap:5px}", RightTabChipGap, 5},
		{".slot{width:10px}", RightTabSepWidth, 10},
		{".slot:before{height:14px}", RightTabSepRuleH, 14},
		{".slot:before{width:.5px}", RightTabSepRuleW, 0.5},
		{".viewTabs{min-height:25px}", ViewTabRowHeight, 25},
		{".viewTabs{gap:32px}", ViewTabGap, 32},
		{".tab::after{height:2px}", ViewTabActiveBarHeight, 2},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Fatalf("%s：实现值 %v，官方 %v", tc.name, tc.got, tc.want)
		}
	}
	// `.tabStrip` 的外高 = 28 + 10 = 38，与会话侧注释里的 "ui-dockkit .tabStrip 38px" 对齐。
	if outer := RightTabStripRowHeight + RightTabStripPadTop; outer != 38 {
		t.Fatalf(".tabStrip 外高 = %v，官方注释写的是 38px", outer)
	}
}

// ── E. 骨架按注册派发 ───────────────────────────────────────────────

func TestSkeletonDispatchesOnlyActiveView(t *testing.T) {
	drawn := map[string]int{}
	env, canvas := newSkeletonEnv(1920, 1032, func(pluginapi.Slot) bool { return false })
	env.Theme = themePtr()
	env.DrawKeyed = func(_ ui.C, slot pluginapi.Slot, key string) {
		drawn[string(slot)+":"+key]++
	}
	opts := ShellOptions{
		Windows: true, Tabs: true, SidebarPreferred: 280, RightbarOn: true, PanellistRows: 3,
		Views: []pluginapi.Tab{
			{Key: pluginapi.ViewChat, Label: "对话"},
			{Key: pluginapi.ViewTrajectory, Label: "轨迹", Order: 10},
		},
		ActiveView: pluginapi.ViewTrajectory,
	}
	seen := map[pluginapi.Slot]bool{}
	pageTheme := dscore.VivaTheme(env.Theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		FirstScreen(f, env, opts, func(slot pluginapi.Slot) { seen[slot] = true })
	})
	if drawn[string(OffConvView)+":"+pluginapi.ViewTrajectory] != 1 {
		t.Fatalf("选中的 view 应当派发一次，实际 %v", drawn)
	}
	if _, ok := drawn[string(OffConvView)+":"+pluginapi.ViewChat]; ok {
		t.Fatal("没选中的 view 也派发了 = 两个 view 会画进同一块矩形")
	}
	if seen[OffConvView] {
		t.Fatal("接了 keyed 派发时不该再走旧的「全画一遍」那条路")
	}
}

func TestSkeletonRightbarDispatchesOnlyActiveTab(t *testing.T) {
	drawn := map[string]int{}
	env, canvas := newSkeletonEnv(1920, 1032, func(pluginapi.Slot) bool { return false })
	env.Theme = themePtr()
	env.DrawKeyed = func(_ ui.C, slot pluginapi.Slot, key string) {
		drawn[string(slot)+":"+key]++
	}
	opts := ShellOptions{
		Windows: true, SidebarPreferred: 280, RightbarOn: true, PanellistRows: 3,
		RightTabs: []pluginapi.Tab{
			{Key: "timeline", Label: "轨迹", Order: 10},
			{Key: "terminal", Label: "终端", Order: 20},
		},
		ActiveRightTab: "terminal",
	}
	seen := map[pluginapi.Slot]bool{}
	pageTheme := dscore.VivaTheme(env.Theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		FirstScreen(f, env, opts, func(slot pluginapi.Slot) { seen[slot] = true })
	})
	if drawn[string(OffRightPaneTab)+":terminal"] != 1 {
		t.Fatalf("选中的右栏 tab 应当派发一次，实际 %v", drawn)
	}
	if _, ok := drawn[string(OffRightPaneTab)+":timeline"]; ok {
		t.Fatal("没选中的右栏 tab 也派发了")
	}
	if seen[OffRightPaneTab] {
		t.Fatal("按名录派发时不该再走旧的「全画一遍」那条路")
	}
}

// ── F. 真画 + 真点 ──────────────────────────────────────────────────

func TestDrawRightTabsDrawsStripAndSelects(t *testing.T) {
	tabs := []pluginapi.Tab{
		{Key: "timeline", Label: "轨迹", Order: 10},
		{Key: "terminal", Label: "终端", Order: 20},
	}
	selected := ""
	runHeadlessChips(t, 640, 200, func(f ui.C, env *pluginapi.Env) {
		DrawRightTabs(f, env, tabs, "timeline", func(key string) { selected = key })
	})
	if selected != "terminal" {
		t.Fatalf("点「终端」芯片没走到选择动作，实际 %q", selected)
	}
}

func TestDrawViewTabsDrawsChipsAndSelects(t *testing.T) {
	tabs := []pluginapi.Tab{
		{Key: pluginapi.ViewChat, Label: "对话"},
		{Key: pluginapi.ViewTrajectory, Label: "轨迹", Order: 10},
	}
	selected := ""
	runHeadlessChips(t, 640, 120, func(f ui.C, env *pluginapi.Env) {
		DrawViewTabs(f, env, tabs, pluginapi.ViewChat, func(key string) { selected = key })
	})
	if selected != pluginapi.ViewTrajectory {
		t.Fatalf("点「轨迹」页签没走到选择动作，实际 %q", selected)
	}
}

// runHeadlessChips 开一个无头窗口画一次芯片行，然后点标签 clickLabel。
func runHeadlessChips(t *testing.T, width, height int, draw func(ui.C, *pluginapi.Env)) {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("view-tabs-headless")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{
		Title: "view-tabs", Width: width, Height: height, Tree: tree, WindowMode: app.WindowFrameless,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			Has: func(pluginapi.Slot) bool { return false },
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) { draw(f, env) })
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	session.Resize(width, height)
	// 两个芯片都真的画出来了吗（文字层）。
	texts := map[string]bool{}
	for _, one := range session.Snapshot().Texts {
		texts[one.Value] = true
	}
	wantSecond := "终端"
	if width == 640 && height == 120 {
		wantSecond = "轨迹"
	}
	if !texts[wantSecond] {
		t.Fatalf("芯片文案 %q 没画出来：%v", wantSecond, session.Texts())
	}
	if err := session.ClickLabel(wantSecond); err != nil {
		t.Fatalf("点芯片 %q 失败：%v", wantSecond, err)
	}
}

func themePtr() *pluginapi.Theme {
	th := dscore.Dark()
	return th
}
