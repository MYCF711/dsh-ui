package dsstatusbar

// official_stats_dialog_test.go —— 官方 Activity 详情弹窗的结构 / 真值 / 降级断言。
//
// 判据（task-53 §任务 2/3/4）：
//
//	① 入口：活动胶囊按官方 DialogPill 画成 button（命中区 + 展开态 + hover 字色）；
//	   用量胶囊在没有官方 `tokenUsage` 投影时保持 PlainPill（无命中区）；
//	② 结构：role 区域（label = 会话统计）+ 标题（章节名 + 头条计数）+ 细线 +
//	   四条 dt/dd，顺序与官方 JSX 一致；
//	③ 八字段逐个渲染：只改一个字段，弹窗里的读数必须跟着变；
//	④ 空投影降级：`SessionStats == nil` / `steps == 0` / 计时全 0 → 静态读数、
//	   点不开、不画任何编造数字（尤其**不得出现** `0 轮 0 步`）；
//	⑤ 行为：点弹窗外面关闭、点弹窗内部不关、Esc 关闭、切会话关闭、每帧只画一次。

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// statsDialogHarness 是一个"只画官方 composer 座位"的最小装配。
type statsDialogHarness struct {
	plugin *Plugin
	kernel *uikernel.Kernel
	tree   *gfx.Tree
	feat   gfx.Scope
	theme  *pluginapi.Theme
}

func newStatsDialogHarness(t *testing.T) *statsDialogHarness {
	t.Helper()
	tier := uikernel.NewShellTier()
	tier.Set(true)
	kernel := uikernel.New("stats-dialog")
	kernel.Root().Provide(uikernel.KeyShellTier, tier)
	plugin := &Plugin{}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, plugin); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	tree := &gfx.Tree{}
	return &statsDialogHarness{
		plugin: plugin,
		kernel: kernel,
		tree:   tree,
		feat:   tree.Feature("stats-dialog"),
		theme:  kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme),
	}
}

// frame 画一帧：click / hover 非空时按该交互名造一次事件 / 悬停态。
//
// 与 `uikernel.Kernel.DrawSlot` 同义（Draw → Children 转发），只是**顺手**把
// AfterDraw 也跑一遍——弹窗的锚点矩形就是在那里缓存的，不跑 AfterDraw 的帧
// 只能走过渡几何。
func (h *statsDialogHarness) frame(t *testing.T, bounds ui.Rect, data pluginapi.Snapshot, click, hover string) *gfx.Canvas {
	t.Helper()
	canvas := &gfx.Canvas{}
	canvas.SetTree(h.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size * 0.55
	})
	if hover != "" {
		canvas.SetInteraction(h.tree.ID(h.feat, hover), gfx.ID{}, gfx.ID{}, false)
	}
	env := &pluginapi.Env{
		Theme: h.theme, Data: data, Tree: h.tree, Feat: h.feat, Canvas: canvas,
		Bounds: bounds, MainWidth: bounds.Width,
		Geometry: pluginapi.Geometry{Width: bounds.Width, Height: bounds.Height, MainWidth: bounds.Width},
		Has:      h.kernel.Has,
	}
	if click != "" {
		canvas.BeginEventPass(h.tree.ID(h.feat, click))
	}
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(h.theme), bounds, func(f *flex.Frame) {
		// 产品形状：dock 在输入卡片下方、贴着主区底部。用 Spacer 把它压到底，
		// 这样弹窗的"向上展开"路径（官方首选方向）才是被验收的那一条。
		f.Column().FillWidth().FillHeight().Children(func() {
			f.Spacer()
			h.drawSlot(f, env, uikernel.OffConvComposer)
		})
	})
	if click != "" {
		canvas.EndEventPass()
	}
	for _, painter := range h.kernel.AllPainters() {
		if painter.AfterDraw != nil {
			painter.AfterDraw(canvas, env)
		}
	}
	return canvas
}

func (h *statsDialogHarness) drawSlot(f *flex.Frame, env *pluginapi.Env, slot pluginapi.Slot) {
	for _, painter := range h.kernel.Painters(slot) {
		if painter.Draw != nil {
			painter.Draw(f, env)
		}
		if painter.Children != nil {
			painter.Children(f, env, func(next pluginapi.Slot) {
				h.drawSlot(f, env, next)
			})
		}
	}
}

// openDialog 画两帧把弹窗点开：第一帧只画胶囊（AfterDraw 缓存真实矩形），
// 第二帧在胶囊的命中区上造一次点击。返回第二帧的画布。
func (h *statsDialogHarness) openDialog(t *testing.T, data pluginapi.Snapshot) *gfx.Canvas {
	t.Helper()
	bounds := ui.Rect{Width: 1182, Height: 832}
	h.frame(t, bounds, data, "", "")
	return h.frame(t, bounds, data, statsPillHitName, "")
}

func statsDialogTexts(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

func statsDialogHasText(canvas *gfx.Canvas, want string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == want {
			return true
		}
	}
	return false
}

func statsDialogJoined(canvas *gfx.Canvas) string {
	return strings.Join(statsDialogTexts(canvas), "\n")
}

// statsDialogHit 按 aria 名找命中区（官方支持的名字：胶囊读数 / 会话统计）。
func statsDialogHit(canvas *gfx.Canvas, label string) (gfx.Hit, bool) {
	for _, hit := range canvas.Hits() {
		if hit.Label == label {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// statsDialogHitByName 按交互名找命中区（Tree 名字比 aria 名稳定）。
func statsDialogHitByName(t *testing.T, h *statsDialogHarness, canvas *gfx.Canvas, name string) (gfx.Hit, bool) {
	t.Helper()
	id := h.tree.ID(h.feat, name)
	for _, hit := range canvas.Hits() {
		if hit.ID == id {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// statsFullStats 是真机跑出来的那份八字段形状（HC 报告 §4.1）。
func statsFullStats() *apiclient.SessionStats {
	return &apiclient.SessionStats{
		Turns: 2, Steps: 3, LLMMs: 3267, ToolMs: 61, TTFTMs: 3055,
		TTFTSteps: 2, DecodeMs: 212, DecodeTokens: 87,
	}
}

func statsSnapshot(stats *apiclient.SessionStats) pluginapi.Snapshot {
	return pluginapi.Snapshot{
		Cur:          "s-stats",
		SessionStats: stats,
		UsageTotal:   pluginapi.Usage{InputTokens: 12000, OutputTokens: 345, TotalTokens: 12345},
	}
}

// ① 结构：打开后画的是官方那一棵树（标题 / 计数 / 四条 dt-dd），顺序一致。
func TestStatsDialogStructureFollowsOfficialTree(t *testing.T) {
	h := newStatsDialogHarness(t)
	data := statsSnapshot(statsFullStats())

	// 第一帧只有胶囊：没有弹窗、但有命中区（官方 DialogPill）。
	closed := h.frame(t, ui.Rect{Width: 1182, Height: 832}, data, "", "")
	if statsDialogHasText(closed, zhStatsDialogTitle) {
		t.Fatalf("没点开的帧里出现了弹窗标题：%v", statsDialogTexts(closed))
	}
	pill, ok := statsDialogHitByName(t, h, closed, statsPillHitName)
	if !ok {
		t.Fatal("活动胶囊没有命中区（官方 DialogPill 是 button）")
	}
	if pill.Role != gfx.RoleButton || !pill.Expandable || pill.Expanded {
		t.Fatalf("胶囊命中区 = %+v，want button/expandable/未展开", pill)
	}

	open := h.openDialog(t, data)
	texts := statsDialogTexts(open)
	// 结构存在的判据：标题行（章节名 + 计数）、四条明细，按官方 JSX 顺序。
	wantOrder := []string{
		zhStatsDialogTitle, "2 轮 3 步",
		zhStatsDialogLLMTime, "3.3秒",
		zhStatsDialogToolTime, "0.1秒",
		zhStatsDialogTTFT, "1.5秒",
		zhStatsDialogSpeed, "410 tok/s",
	}
	for _, want := range wantOrder {
		if !statsDialogHasText(open, want) {
			t.Fatalf("弹窗缺 %q；texts=%v", want, texts)
		}
	}
	last := -1
	for _, want := range wantOrder {
		index := -1
		for i, text := range texts {
			if text == want {
				index = i
				break
			}
		}
		if index <= last {
			t.Fatalf("%q 的顺序不对（index=%d，上一项=%d）：%v", want, index, last, texts)
		}
		last = index
	}
	// 弹窗本体的角色区：label = 会话统计（官方 aria-label={title}）。
	panel, ok := statsDialogHit(open, zhStatsDialogTitle)
	if !ok {
		t.Fatalf("弹窗没有角色命中区（官方 role=dialog aria-label=会话统计）；hits=%v", open.Hits())
	}
	if panel.Role != gfx.RoleGroup {
		t.Fatalf("弹窗角色 = %v，want RoleGroup（viva v0.2.3 没有 RoleDialog）", panel.Role)
	}
	// 展开后胶囊的 aria-expanded 要变 true（官方 trigger 的语义）。
	opened, ok := statsDialogHitByName(t, h, open, statsPillHitName)
	if !ok || !opened.Expanded {
		t.Fatalf("展开后胶囊 aria-expanded 未置位：%+v (ok=%v)", opened, ok)
	}
	if h.plugin.statsDialogOpen(data) != true {
		t.Fatal("点开之后插件状态里没有展开")
	}
}

// ③ 八字段逐个渲染：只改一个字段，弹窗读数必须跟着变（其余字段保持为 0）。
func TestStatsDialogRendersEachOfTheEightFields(t *testing.T) {
	cases := []struct {
		name  string
		stats *apiclient.SessionStats
		want  string
	}{
		// turns / steps：弹窗标题右侧那一格（官方 titleValue={counts}）。
		{"turns", &apiclient.SessionStats{Turns: 7, Steps: 3}, "7 轮 3 步"},
		{"steps", &apiclient.SessionStats{Turns: 2, Steps: 9}, "2 轮 9 步"},
		// llmMs：≥60s 走 official compactMinutes（1分30秒）。
		{"llmMs", &apiclient.SessionStats{Turns: 1, Steps: 1, LLMMs: 90000}, "1分30秒"},
		// toolMs：61s → 1分1秒（整秒四舍五入后拆成分秒）。
		{"toolMs", &apiclient.SessionStats{Turns: 1, Steps: 1, ToolMs: 61000}, "1分1秒"},
		// ttftMs / ttftSteps：4000ms 摊到 2 步 = 2秒。
		{"ttftMs+ttftSteps", &apiclient.SessionStats{Turns: 1, Steps: 1, TTFTMs: 4000, TTFTSteps: 2}, "2秒"},
		// ttftSteps 单独变：同一份 ttftMs 摊到 4 步 = 1秒。
		{"ttftSteps", &apiclient.SessionStats{Turns: 1, Steps: 1, TTFTMs: 4000, TTFTSteps: 4}, "1秒"},
		// decodeMs / decodeTokens：500 tok / 1s = 500 tok/s。
		{"decodeMs+decodeTokens", &apiclient.SessionStats{Turns: 1, Steps: 1, DecodeMs: 1000, DecodeTokens: 500}, "500 tok/s"},
		// decodeTokens 单独变：同一 1s 窗口里 250 tok。
		{"decodeTokens", &apiclient.SessionStats{Turns: 1, Steps: 1, DecodeMs: 2000, DecodeTokens: 500}, "250 tok/s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newStatsDialogHarness(t)
			canvas := h.openDialog(t, statsSnapshot(tc.stats))
			if !statsDialogHasText(canvas, tc.want) {
				t.Fatalf("弹窗没渲染 %s 的真值 %q；texts=%v", tc.name, tc.want, statsDialogTexts(canvas))
			}
		})
	}
}

// ④ 空投影降级：没有 sessionStats 时胶囊写「未知」、**不可点**、点不出弹窗。
func TestStatsDialogHiddenWithoutProjection(t *testing.T) {
	h := newStatsDialogHarness(t)
	data := pluginapi.Snapshot{Cur: "s-empty"} // SessionStats == nil（老宿主 / 未拉到）
	first := h.frame(t, ui.Rect{Width: 1182, Height: 832}, data, "", "")
	if !statsDialogHasText(first, zhActivityUnknown) {
		t.Fatalf("没有投影时活动胶囊 = %v，want「活动 未知」", statsDialogTexts(first))
	}
	if _, ok := statsDialogHitByName(t, h, first, statsPillHitName); ok {
		t.Fatal("没有投影时活动胶囊仍有命中区（点了也开不出弹窗 = 假入口）")
	}
	// 就算硬点这个名字，也不许出现弹窗。
	clicked := h.frame(t, ui.Rect{Width: 1182, Height: 832}, data, statsPillHitName, "")
	if statsDialogHasText(clicked, zhStatsDialogTitle) {
		t.Fatalf("没有投影却点出了弹窗：%v", statsDialogTexts(clicked))
	}
	for _, text := range statsDialogTexts(clicked) {
		for _, banned := range []string{"0 轮", "0 步", "0 tok/s", "会话统计"} {
			if strings.Contains(text, banned) {
				t.Fatalf("没有投影时编造了 %q：%v", banned, statsDialogTexts(clicked))
			}
		}
	}
}

// ④' 真值 0 步（空会话）：官方 `steps === 0 → null`，不显示 `0 轮 0 步`。
func TestStatsDialogDoesNotFabricateZeroCounts(t *testing.T) {
	h := newStatsDialogHarness(t)
	data := pluginapi.Snapshot{Cur: "s-zero", SessionStats: &apiclient.SessionStats{}}
	first := h.frame(t, ui.Rect{Width: 1182, Height: 832}, data, "", "")
	if _, ok := statsDialogHitByName(t, h, first, statsPillHitName); ok {
		t.Fatal("steps=0 时胶囊仍可点（官方整枚不渲染 + 本仓写「未知」）")
	}
	clicked := h.frame(t, ui.Rect{Width: 1182, Height: 832}, data, statsPillHitName, "")
	joined := statsDialogJoined(clicked)
	for _, banned := range []string{"0 轮 0 步", "0 轮", "0 步"} {
		if strings.Contains(joined, banned) {
			t.Fatalf("空会话编造了 %q：%v", banned, statsDialogTexts(clicked))
		}
	}
	if strings.Contains(joined, zhStatsDialogTitle) {
		t.Fatalf("空会话点出了弹窗：%v", statsDialogTexts(clicked))
	}
}

// ④” 有 step 但四个计时全 0：官方退回 PlainPill（没有行就不开空弹窗）。
func TestStatsDialogStaysPlainWithoutTimedRows(t *testing.T) {
	h := newStatsDialogHarness(t)
	stats := &apiclient.SessionStats{Turns: 1, Steps: 2}
	data := statsSnapshot(stats)
	first := h.frame(t, ui.Rect{Width: 1182, Height: 832}, data, "", "")
	if !statsDialogHasText(first, "1 轮 2 步") {
		t.Fatalf("胶囊读数 = %v，want「1 轮 2 步」", statsDialogTexts(first))
	}
	if _, ok := statsDialogHitByName(t, h, first, statsPillHitName); ok {
		t.Fatal("没有计时行时胶囊仍有命中区（官方是 PlainPill，不可点）")
	}
	clicked := h.frame(t, ui.Rect{Width: 1182, Height: 832}, data, statsPillHitName, "")
	if strings.Contains(statsDialogJoined(clicked), zhStatsDialogTitle) {
		t.Fatalf("没有计时行却点出了空弹窗：%v", statsDialogTexts(clicked))
	}
}

// ②' hover：官方 `button.pill:hover` 字色升到 label-secondary；静止态是 label-tertiary。
func TestStatsDialogPillHoverState(t *testing.T) {
	h := newStatsDialogHarness(t)
	data := statsSnapshot(statsFullStats())
	bounds := ui.Rect{Width: 1182, Height: 832}

	rest := h.frame(t, bounds, data, "", "")
	if color := statsDialogTextColor(rest, "2 轮 3 步 · 410 tok/s"); color != h.theme.LabelTertiary {
		t.Fatalf("静止态胶囊字色 = %v，want LabelTertiary", color)
	}
	hover := h.frame(t, bounds, data, "", statsPillHitName)
	if color := statsDialogTextColor(hover, "2 轮 3 步 · 410 tok/s"); color != h.theme.LabelSecondary {
		t.Fatalf("悬停态胶囊字色 = %v，want LabelSecondary（官方 :47-52）", color)
	}
}

// statsDialogTextColor 取某段文本的颜色。
func statsDialogTextColor(canvas *gfx.Canvas, want string) ui.Color {
	for _, text := range canvas.Texts() {
		if text.Value == want {
			return text.Style.Color
		}
	}
	return ui.Color{}
}

// ⑤ 点弹窗外面关闭（官方 useDismissOnOutsidePointer）。
func TestStatsDialogClosesOnOutsideClick(t *testing.T) {
	h := newStatsDialogHarness(t)
	data := statsSnapshot(statsFullStats())
	bounds := ui.Rect{Width: 1182, Height: 832}
	h.frame(t, bounds, data, "", "")
	open := h.frame(t, bounds, data, statsPillHitName, "")
	if !statsDialogHasText(open, zhStatsDialogTitle) {
		t.Fatalf("弹窗没打开：%v", statsDialogTexts(open))
	}
	// 全窗遮罩命中区必须存在且覆盖整窗（官方 document pointerdown 的等价物）。
	backdrop, ok := statsDialogHitByName(t, h, open, statsDialogBackdrop)
	if !ok {
		t.Fatal("展开时没有遮罩命中层（点外面关不掉）")
	}
	if backdrop.Bounds.Width != bounds.Width || backdrop.Bounds.Height != bounds.Height {
		t.Fatalf("遮罩没有覆盖整窗：%+v，want %+v", backdrop.Bounds, bounds)
	}
	closed := h.frame(t, bounds, data, statsDialogBackdrop, "")
	if statsDialogHasText(closed, zhStatsDialogTitle) {
		t.Fatalf("点外面之后弹窗还在：%v", statsDialogTexts(closed))
	}
	if h.plugin.dialog.open {
		t.Fatal("点外面之后插件状态仍是展开")
	}
}

// ⑤' 点弹窗内部不关闭（官方只认"外面"）。
func TestStatsDialogStaysOpenOnPanelClick(t *testing.T) {
	h := newStatsDialogHarness(t)
	data := statsSnapshot(statsFullStats())
	bounds := ui.Rect{Width: 1182, Height: 832}
	h.frame(t, bounds, data, "", "")
	h.frame(t, bounds, data, statsPillHitName, "")
	inside := h.frame(t, bounds, data, statsDialogPanelHit, "")
	if !statsDialogHasText(inside, zhStatsDialogTitle) {
		t.Fatalf("点弹窗内部把弹窗关掉了：%v", statsDialogTexts(inside))
	}
}

// ⑤” Esc 关闭（官方 stat-dialog.ts 的 keydown → Escape）。
func TestStatsDialogClosesOnEscape(t *testing.T) {
	h := newStatsDialogHarness(t)
	data := statsSnapshot(statsFullStats())
	bounds := ui.Rect{Width: 1182, Height: 832}
	h.frame(t, bounds, data, "", "")
	h.frame(t, bounds, data, statsPillHitName, "")
	if !h.plugin.dialog.open {
		t.Fatal("前置：弹窗应已展开")
	}
	if h.plugin.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) != true {
		t.Fatal("展开时 Esc 没被接住")
	}
	if h.plugin.dialog.open {
		t.Fatal("Esc 之后弹窗仍展开")
	}
	// 没展开时 Esc 必须放行（不能抢 composer 的"停止 turn"）。
	if h.plugin.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) {
		t.Fatal("没展开时 Esc 被吞掉了")
	}
}

// ⑤”' 切会话关闭（官方 dock 是 session 作用域，换会话 ≠ 沿用上一个会话的弹窗）。
func TestStatsDialogClosesOnSessionSwitch(t *testing.T) {
	h := newStatsDialogHarness(t)
	first := statsSnapshot(statsFullStats())
	bounds := ui.Rect{Width: 1182, Height: 832}
	h.frame(t, bounds, first, "", "")
	h.frame(t, bounds, first, statsPillHitName, "")
	if !h.plugin.dialog.open {
		t.Fatal("前置：弹窗应已展开")
	}
	other := first
	other.Cur = "s-other"
	switched := h.frame(t, bounds, other, "", "")
	if h.plugin.dialog.open || statsDialogHasText(switched, zhStatsDialogTitle) {
		t.Fatalf("切会话后弹窗没关：%v", statsDialogTexts(switched))
	}
}

// ⑤”” 每帧只画一次（重画不累积、不重复）。
func TestStatsDialogRendersOncePerFrame(t *testing.T) {
	h := newStatsDialogHarness(t)
	data := statsSnapshot(statsFullStats())
	bounds := ui.Rect{Width: 1182, Height: 832}
	h.frame(t, bounds, data, "", "")
	h.frame(t, bounds, data, statsPillHitName, "")
	canvas := h.frame(t, bounds, data, "", "")
	for _, want := range []string{zhStatsDialogTitle, zhStatsDialogLLMTime, zhStatsDialogSpeed, "2 轮 3 步"} {
		count := 0
		for _, text := range canvas.Texts() {
			if text.Value == want {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("%q 一帧画了 %d 次，want 1；texts=%v", want, count, statsDialogTexts(canvas))
		}
	}
}

// ①' 用量胶囊今天保持 PlainPill：宿主没有官方 `tokenUsage` 投影，不画假入口。
func TestUsagePillStaysPlainWithoutTokenUsage(t *testing.T) {
	h := newStatsDialogHarness(t)
	data := statsSnapshot(statsFullStats())
	canvas := h.frame(t, ui.Rect{Width: 1182, Height: 832}, data, "", "")
	pills := 0
	for _, hit := range canvas.Hits() {
		if hit.Role == gfx.RoleButton {
			pills++
		}
	}
	if pills != 1 {
		t.Fatalf("dock 上可点胶囊 = %d 颗，want 1（只有活动；用量缺 tokenUsage 投影）", pills)
	}
	if !statsDialogHasText(canvas, "总量 12.3k") {
		t.Fatalf("用量读数 = %v，want「总量 12.3k」", statsDialogTexts(canvas))
	}
}
