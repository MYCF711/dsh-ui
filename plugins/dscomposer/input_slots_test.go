package dscomposer

// input_slots_test.go —— `conversation.input.*` 子槽派发 + 模型标签唯一性（本任务核心断言）。
//
// 四条判据：
//
//	① 官方座位（`conversation.composer`）每帧把 10 个 `conversation.input.*` 子槽
//	   各派发**恰好 1 次**；旧槽 `main.composer` 同帧 0 次（不双画）。
//	② 旧三栏座位（`main.composer`）派发的是**同一批**子槽（同一份父容器实现）。
//	③ 模型标签**恰好一个**：有占用方（dsagentteam）时由它画 trigger，composer 不画
//	   回退 chip；没有占用方时才画回退 chip。两种形态各自只产生一个模型文本。
//	④ 降级两态：目录未知 → 只读 chip（宿主真值 / 「未知」）且**没有**命中区；
//	   目录可用 → 官方 trigger（可点）+ 菜单展开时出现官方 Team 开关。

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dsagentteam"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// zhModelMenuAria 是 dsagentteam 的模型 trigger 无障碍名（官方 ModelSelect
// `aria-label={t('trigger')}`，`zhModelMenuAria` 同值）。用字面量而不是 import：
// 那是插件内部常量，跨包断言只认**官方文案本身**。
const zhModelMenuAria = "模型与推理等级"

// zhTeamModeLabel 是 dsagentteam 的 Team 开关文案（`locales.ts` 的 `mode.label`）。
const zhTeamModeLabel = "启用智能体团队协作"

// probePlugin 在每一个 `conversation.input.*` 子槽上挂一块"只记账"的画。
//
// 为什么需要它：计数 dispatch 的壳级回调只包住**座位**那一层；子槽是 composer
// 在 `Children` 里用内核的 `drawSlot` 直接派发的（这正是父容器的定义），壳级
// 回调看不到。探针占住这些槽，内核每派发一次就调它的 Draw 一次，于是画布上的
// 绘制次数 == dispatch 次数。它不画任何像素，不影响几何。
type probePlugin struct {
	counts map[pluginapi.Slot]int
	// skipModel 让探针不占模型槽：测"回退 chip 仍在"时必须留空，
	// 否则占用方存在 → composer 按规则不画 chip（那时该由占用方画）。
	skipModel bool
}

func (p *probePlugin) ID() string                     { return "test-input-slot-probe" }
func (*probePlugin) Provides() []string               { return nil }
func (*probePlugin) Requires() []string               { return []string{pluginapi.KeyTheme} }
func (*probePlugin) Register(pluginapi.Context) error { return nil }

func (p *probePlugin) Panels() []pluginapi.Painter {
	out := make([]pluginapi.Painter, 0, len(composerAllInputSubSlots()))
	for _, slot := range composerAllInputSubSlots() {
		if p.skipModel && slot == uikernel.OffInputModel {
			continue
		}
		slot := slot
		out = append(out, pluginapi.Painter{
			ID:   "probe:" + string(slot),
			Slot: slot,
			Draw: func(ui.C, *pluginapi.Env) { p.counts[slot]++ },
		})
	}
	return out
}

// slotHarness 是"真内核 + 真插件 + 真 dispatch"的测试台。
type slotHarness struct {
	kernel     *uikernel.Kernel
	team       *dsagentteam.Store
	dispatched map[pluginapi.Slot]int
	tree       *gfx.Tree
	feat       gfx.Scope
	theme      *pluginapi.Theme
}

// newSlotHarness 装配 dscore + dscomposer（可选 dsagentteam），
// 并把内核的 DrawSlot 当 dispatch——与壳/产品走的是同一条链路。
func newSlotHarness(t *testing.T, withTeam bool) *slotHarness {
	t.Helper()
	return newSlotHarnessWithProbe(t, withTeam, false)
}

// newSlotHarnessNoModelProbe 的探针**不占模型槽**（见 probePlugin.skipModel）。
func newSlotHarnessNoModelProbe(t *testing.T, withTeam bool) *slotHarness {
	t.Helper()
	return newSlotHarnessProbe(t, withTeam, true)
}

// newSlotHarnessWithProbe 的第三参打开子槽探针（见 probePlugin）。
func newSlotHarnessWithProbe(t *testing.T, withTeam, probe bool) *slotHarness {
	return newSlotHarnessProbe(t, withTeam, probe)
}

func newSlotHarnessProbe(t *testing.T, withTeam, probe bool) *slotHarness {
	t.Helper()
	h := &slotHarness{
		kernel:     uikernel.New("composer-input-slots"),
		dispatched: map[pluginapi.Slot]int{},
		tree:       &gfx.Tree{},
	}
	plugins := []pluginapi.Plugin{&dscore.Plugin{Dark: true}, &Plugin{}}
	if withTeam {
		plugins = append(plugins, &dsagentteam.Plugin{})
	}
	if probe {
		plugins = append(plugins, &probePlugin{counts: h.dispatched})
	}
	if err := h.kernel.Load(plugins...); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	h.theme = h.kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	h.feat = h.tree.Feature("composer-input-slots")
	if withTeam {
		store, ok := h.kernel.Get(dsagentteam.ServiceKey).(*dsagentteam.Store)
		if !ok {
			t.Fatalf("dsagentteam.ServiceKey 不是 *Store：%T", h.kernel.Get(dsagentteam.ServiceKey))
		}
		h.team = store
	}
	return h
}

// paint 画一帧官方骨架（真 flex + 真画布），返回画布。
func (h *slotHarness) paint(t *testing.T, opts uikernel.ShellOptions, snap pluginapi.Snapshot) *gfx.Canvas {
	t.Helper()
	canvas := &gfx.Canvas{}
	canvas.SetTree(h.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: h.theme, Data: snap, Tree: h.tree, Feat: h.feat, Canvas: canvas,
		Bounds:    gfx.Rect{Width: 1182, Height: 832},
		MainWidth: 902,
		Geometry:  pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902},
		Has:       func(slot pluginapi.Slot) bool { return h.kernel.Has(slot) },
	}
	// 原地清空（不是换新 map）：子槽探针持有的是同一个 map 引用。
	for slot := range h.dispatched {
		delete(h.dispatched, slot)
	}
	pageTheme := dscore.VivaTheme(h.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		uikernel.FirstScreen(f, env, opts, func(slot pluginapi.Slot) {
			h.dispatched[slot]++
			h.kernel.DrawSlot(f, slot, env)
		})
	})
	// 布局已定：跑插件的 AfterDraw（dsagentteam 在那里缓存 trigger 矩形）。
	for _, painter := range h.kernel.AllPainters() {
		if painter.AfterDraw != nil {
			painter.AfterDraw(canvas, env)
		}
	}
	return canvas
}

// officialActiveOpts 是「官方骨架 + 有会话」的一帧输入（产品 `-shell official` 的形状：
// 旧部件不在，整列走官方骨架；composer 座位有占用方 → 走官方座位不派发旧槽）。
func officialActiveOpts() uikernel.ShellOptions {
	return uikernel.ShellOptions{
		SidebarPreferred: 280,
		Phase:            uikernel.PhaseActive,
	}
}

// legacySeatOpts 是旧三栏座位的一帧输入：整列兼容桥开着，composer 退 `main.composer`
// （纯旧插件集没有官方座位占用方时壳算出来的形状，见 cmd/app/shell.go）。
func legacySeatOpts() uikernel.ShellOptions {
	return uikernel.ShellOptions{
		SidebarPreferred:     280,
		Phase:                uikernel.PhaseActive,
		BridgeLegacy:         true,
		BridgeLegacyComposer: true,
	}
}

// countTexts 数一个精确文本出现次数（模型标签唯一性判据）。
func countTexts(canvas *gfx.Canvas, value string) int {
	n := 0
	for _, text := range canvas.Texts() {
		if text.Value == value {
			n++
		}
	}
	return n
}

// hasHitLabel 报告画布上有没有这个标签的命中区。
func hasHitLabel(canvas *gfx.Canvas, label string) bool {
	for _, hit := range canvas.Hits() {
		if hit.Label == label {
			return true
		}
	}
	return false
}

// canvasTexts 是失败诊断用的文本列表。
func canvasTexts(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

// TestOfficialSeatDispatchesInputSubSlots ①：官方座位每帧派发全部子槽恰好一次。
func TestOfficialSeatDispatchesInputSubSlots(t *testing.T) {
	h := newSlotHarnessWithProbe(t, false, true)
	h.paint(t, officialActiveOpts(), pluginapi.Snapshot{
		Cur: "s-1", Model: "deepseek-v4-pro", Permission: "ask",
		ContextTokens: 12345, ContextWindow: 200000,
	})

	if got := h.dispatched[uikernel.OffConvComposer]; got != 1 {
		t.Fatalf("官方 composer 座位 dispatch = %d，want 1（全部=%v）", got, h.dispatched)
	}
	if got := h.dispatched[pluginapi.SlotMainComposer]; got != 0 {
		t.Fatalf("官方座位活着时旧槽 main.composer 也被 dispatch（双画）：%d", got)
	}
	for _, slot := range composerAllInputSubSlots() {
		if got := h.dispatched[slot]; got != 1 {
			t.Fatalf("子槽 %q 一帧 dispatch = %d，want 1（全部=%v）", slot, got, h.dispatched)
		}
	}
	// 表里的模型槽必须就是官方字面量（防止有人把常量抄错）。
	if composerInputSlotModel != uikernel.OffInputModel {
		t.Fatalf("composerInputSlotModel = %q，官方 %q", composerInputSlotModel, uikernel.OffInputModel)
	}
}

// TestLegacySeatKeepsReadOnlyChip ②：旧三栏座位**不派发**官方子槽，仍画只读 chip。
//
// 这是**登记在案的过渡差距**（不是遗漏）：旧三栏座位由 dsshell 的兼容桥直调
// `painter.Draw` 渲染，迁移它要先改掉那些直调点（`plugins/dsstatusbar/visual_test.go`
// 等），属另一条线。官方座位（`-shell official` 终态）已完整迁移。
//
// 这条测试钉住"现状 + 只有一个模型标签"：旧座位派发 0 个官方子槽，模型文本
// 仍由本插件的只读 chip 画一次，没有模型菜单命中区（旧座位里 dsagentteam 的
// 兼容命中区在自己的 painter 上，不在本包的渲染里）。
func TestLegacySeatKeepsReadOnlyChip(t *testing.T) {
	h := newSlotHarnessNoModelProbe(t, true)
	h.team.SetProviders(true, "", []dsagentteam.ModelProviderOption{
		{Provider: "deepseek", DisplayName: "DeepSeek", Seed: "llm"},
	})
	canvas := h.paint(t, legacySeatOpts(), pluginapi.Snapshot{Cur: "s-1", Model: "deepseek-v4-pro"})

	if got := h.dispatched[pluginapi.SlotMainComposer]; got != 1 {
		t.Fatalf("旧座位 main.composer dispatch = %d，want 1（全部=%v）", got, h.dispatched)
	}
	if got := h.dispatched[uikernel.OffConvComposer]; got != 0 {
		t.Fatalf("旧座位活着时官方座位也被 dispatch（双画）：%d", got)
	}
	for _, slot := range composerAllInputSubSlots() {
		if got := h.dispatched[slot]; got != 0 {
			t.Fatalf("旧三栏座位不该派发官方子槽 %q（过渡差距已登记），实际 %d 次", slot, got)
		}
	}
	if n := countTexts(canvas, "deepseek-v4-pro"); n != 1 {
		t.Fatalf("旧座位的只读 chip 出现 %d 次，want 1；文本=%v", n, canvasTexts(canvas))
	}
	// 旧三栏档的菜单命中区由 dsagentteam 的**兼容 painter**（`main.composer` 上的
	// 透明绝对盒）提供——它和本插件的只读 chip 叠在一起，只有一份文案。
	if !hasHitLabel(canvas, zhModelMenuAria) {
		t.Fatal("旧三栏档缺模型菜单命中区（dsagentteam 的兼容 painter），菜单点不开")
	}
}

// TestModelLabelExactlyOneWithOccupant ③：官方占用方在 → 画布上只有一个模型标签。
func TestModelLabelExactlyOneWithOccupant(t *testing.T) {
	h := newSlotHarness(t, true)
	h.team.SetProviders(true, "", []dsagentteam.ModelProviderOption{
		{Provider: "deepseek", DisplayName: "DeepSeek", Seed: "llm"},
	})
	canvas := h.paint(t, officialActiveOpts(), pluginapi.Snapshot{
		Cur: "s-1", Model: "deepseek-v4-pro", Permission: "ask",
		ContextTokens: 12345, ContextWindow: 200000,
	})

	if n := countTexts(canvas, "deepseek-v4-pro"); n != 1 {
		t.Fatalf("模型标签出现 %d 次，want 恰好 1（双标签？）；文本=%v", n, canvasTexts(canvas))
	}
	if !hasHitLabel(canvas, zhModelMenuAria) {
		t.Fatalf("目录可用时官方 trigger 应有命中区；hits=%v", canvas.Hits())
	}
}

// TestModelLabelExactlyOneWithoutOccupant ③：没有占用方 → composer 回退 chip，仍只有一个。
func TestModelLabelExactlyOneWithoutOccupant(t *testing.T) {
	h := newSlotHarness(t, false)
	canvas := h.paint(t, officialActiveOpts(), pluginapi.Snapshot{
		Cur: "s-1", Model: "deepseek-v4-pro", Permission: "ask",
		ContextTokens: 12345, ContextWindow: 200000,
	})
	if n := countTexts(canvas, "deepseek-v4-pro"); n != 1 {
		t.Fatalf("回退 chip 的模型标签出现 %d 次，want 恰好 1；文本=%v", n, canvasTexts(canvas))
	}
	if hasHitLabel(canvas, zhModelMenuAria) {
		t.Fatal("没有 dsagentteam 时不该有模型菜单命中区（假入口）")
	}
}

// TestModelSlotDegradedStates ④：降级两态 + 不编 0。
func TestModelSlotDegradedStates(t *testing.T) {
	t.Run("目录未知：只读 chip 显示宿主真值，无命中区", func(t *testing.T) {
		h := newSlotHarness(t, true)
		canvas := h.paint(t, officialActiveOpts(), pluginapi.Snapshot{
			Cur: "s-1", Model: "deepseek-v4-pro",
			ContextTokens: 12345, ContextWindow: 200000,
		})
		if n := countTexts(canvas, "deepseek-v4-pro"); n != 1 {
			t.Fatalf("降级 chip 的模型标签出现 %d 次，want 1；文本=%v", n, canvasTexts(canvas))
		}
		if hasHitLabel(canvas, zhModelMenuAria) {
			t.Fatal("目录未知时不该给可点 trigger（点了只会开空菜单 = 假入口）")
		}
	})

	t.Run("目录未知且宿主没给模型：「未知」，不编 0", func(t *testing.T) {
		h := newSlotHarness(t, true)
		canvas := h.paint(t, officialActiveOpts(), pluginapi.Snapshot{
			Cur: "s-1", Permission: "ask",
			ContextTokens: 12345, ContextWindow: 200000,
		})
		if n := countTexts(canvas, "未知"); n != 1 {
			t.Fatalf("宿主没给模型时应有一个「未知」，实际 %d；文本=%v", n, canvasTexts(canvas))
		}
		for _, banned := range []string{"0", "默认模型"} {
			if countTexts(canvas, banned) > 0 {
				t.Fatalf("降级态编造了 %q；文本=%v", banned, canvasTexts(canvas))
			}
		}
	})
}

// TestOfficialToggleAppearsInModelMenu 「官方开关出现」：菜单里真的有 Team 开关。
//
// 数据形状照官方投影：`agentTeam`（投影已知）+ `modelSelection`；这里只给
// `ProjectionKnown`，开关的可见性判据就只有这一条（`TeamModeToggle.tsx:43`）。
func TestOfficialToggleAppearsInModelMenu(t *testing.T) {
	h := newSlotHarness(t, true)
	h.team.SetProviders(true, "", []dsagentteam.ModelProviderOption{
		{Provider: "deepseek", DisplayName: "DeepSeek", Seed: "llm"},
	})
	h.team.SetView(dsagentteam.View{
		SessionID:       "s-1",
		ProjectionKnown: true,
		Projection:      dsagentteam.TeamProjection{Enabled: true},
	})
	h.team.ToggleModelMenu()

	canvas := h.paint(t, officialActiveOpts(), pluginapi.Snapshot{
		Cur: "s-1", Model: "deepseek-v4-pro",
		ContextTokens: 12345, ContextWindow: 200000,
	})
	if !strings.Contains(strings.Join(canvasTexts(canvas), "\n"), zhTeamModeLabel) {
		t.Fatalf("模型菜单里没有官方 Team 开关 %q；文本=%v", zhTeamModeLabel, canvasTexts(canvas))
	}
	if !strings.Contains(strings.Join(canvasTexts(canvas), "\n"), "DeepSeek") {
		t.Fatalf("模型菜单里没有 provider 行；文本=%v", canvasTexts(canvas))
	}
	// 菜单展开不改变"只有一份模型标签"这条。
	if n := countTexts(canvas, "deepseek-v4-pro"); n != 1 {
		t.Fatalf("菜单展开后模型标签出现 %d 次，want 1；文本=%v", n, canvasTexts(canvas))
	}
}
