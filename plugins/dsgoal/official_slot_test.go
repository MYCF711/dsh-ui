package dsgoal

// official_slot_test.go —— 目标条**官方座位**的 dispatch / 不双画 / 可见性两态。
//
// 四条判据（对应 task-47 §任务 1-4）：
//
//	① 注册在官方 chain 座位 `conversation.composer`（官方 ui-goal 的目标槽是
//	   `conversation.input.dock`：id 'goal'、order 10；该子槽今天由 composer
//	   占用方转发，本仓占用方还没转发 → 挂同一 chain 座位，见 official_seat.go）；
//	② 同一帧 legacy 座位（`main.composer`）一次都不画（不双画）；
//	③ 可见性两态：没有活跃 goal → 整块不画；有 → 画且只画一次；
//	④ 相对顺序：问题条(3) < 目标条(5) < 输入卡片(10)，即"dock 在卡片上方"。

import (
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// drawSeat 模拟官方骨架 dispatch 一个槽（含 Children 转发），语义与
// `uikernel.Kernel.drawSlot` 一致（本测试只需要"座位被派发"这件事）。
func drawSeat(f *flex.Frame, k *uikernel.Kernel, env *pluginapi.Env, slot pluginapi.Slot) {
	for _, painter := range k.Painters(slot) {
		if painter.Draw != nil {
			painter.Draw(f, env)
		}
		if painter.Children != nil {
			painter.Children(f, env, func(next pluginapi.Slot) {
				drawSeat(f, k, env, next)
			})
		}
	}
}

// officialFrame 画一帧「官方 composer 座位」，返回画布。
func officialFrame(t *testing.T, tierOfficial bool, goal func() string) *gfx.Canvas {
	t.Helper()
	tier := uikernel.NewShellTier()
	tier.Set(tierOfficial)
	kernel := uikernel.New("goal-official-seat")
	kernel.Root().Provide(uikernel.KeyShellTier, tier)
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &Plugin{}); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: theme, Data: pluginapi.Snapshot{Cur: "s"}, Act: pluginapi.Actions{Goal: goal},
		Tree: tree, Feat: tree.Feature("goal-official"), Canvas: canvas,
		Bounds: ui.Rect{Width: 1182, Height: 832}, MainWidth: 902,
		Geometry: pluginapi.Geometry{Width: 1182, Height: 832, MainWidth: 902},
		Has:      kernel.Has,
	}
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(theme), env.Bounds, func(f *flex.Frame) {
		drawSeat(f, kernel, env, uikernel.OffConvComposer)
	})
	return canvas
}

// ① 官方座位登记：chain 座位 + 相对顺序（3 问题 < 5 目标 < 10 输入卡片）。
func TestGoalRegistersOfficialComposerSeat(t *testing.T) {
	p := &Plugin{}
	byID := map[string]pluginapi.Painter{}
	for _, painter := range p.Panels() {
		byID[painter.ID] = painter
	}
	official, ok := byID["goal-bar-official"]
	if !ok {
		t.Fatal("缺官方座位 painter（goal-bar-official）")
	}
	if official.Slot != uikernel.OffConvComposer {
		t.Fatalf("官方座位 = %q，want conversation.composer（官方 conversation.input.dock 今天不可达）", official.Slot)
	}
	if official.Order != goalDockOrder || goalDockOrder != 5 {
		t.Fatalf("官方 order = %d（want 5：3 问题 < 5 目标 < 10 输入卡片）", official.Order)
	}
	if _, ok := byID["goal-bar"]; !ok {
		t.Fatal("legacy 座位必须还在（main.composer）")
	}
}

// ② 官方档位下 legacy 座位一声不响（不双画）。
func TestGoalLegacySeatStaysSilentOnOfficialTier(t *testing.T) {
	p := &Plugin{}
	tier := uikernel.NewShellTier()
	tier.Set(true)
	kernel := uikernel.New("goal-legacy-silent")
	kernel.Root().Provide(uikernel.KeyShellTier, tier)
	if err := kernel.Load(&dscore.Plugin{Dark: true}, p); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return 8 })
	env := &pluginapi.Env{
		Theme: theme, Data: pluginapi.Snapshot{Cur: "s"},
		Act:  pluginapi.Actions{Goal: func() string { return "有目标" }},
		Tree: tree, Feat: tree.Feature("goal-legacy-silent"), Canvas: canvas,
		Bounds: ui.Rect{Width: 1182, Height: 832}, MainWidth: 902,
		Geometry: pluginapi.Geometry{MainWidth: 902}, Has: kernel.Has,
	}
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(theme), env.Bounds, func(f *flex.Frame) {
		drawSeat(f, kernel, env, pluginapi.SlotMainComposer)
	})
	if rects := canvas.Rectangles(); len(rects) != 0 {
		t.Fatalf("官方档位下 legacy 座位还画了 %d 个矩形（双画）：%+v", len(rects), rects)
	}
}

// ③ 可见性两态：没有活跃 goal → 不画；有 → 只画一次；清掉 → 消失。
func TestGoalVisibilityBothStates(t *testing.T) {
	goal := ""
	canvas := officialFrame(t, true, func() string { return goal })
	if rects := canvas.Rectangles(); len(rects) != 0 {
		t.Fatalf("没有活跃 goal 时不该画任何东西，画了 %+v", rects)
	}
	goal = "先出方案"
	canvas = officialFrame(t, true, func() string { return goal })
	rects := canvas.Rectangles()
	// 官方条 = 条背景(36 高)；glyph 是 sprite 的描边（三角化后不进矩形），
	// 所以"条 1 / 文字标签 1 / 有描边三角形"三条一起才算"正好一份、没双画"。
	var bars, labels int
	for _, rect := range rects {
		if rect.Bounds.Height == goalBarHeight {
			bars++
		}
	}
	for _, text := range canvas.Texts() {
		if text.Value == goalLabelText {
			labels++
		}
	}
	if bars != 1 || labels != 1 {
		t.Fatalf("官方目标条应当正好一份（条 %d / 标签 %d），rects=%+v texts=%+v",
			bars, labels, rects, canvas.Texts())
	}
	if len(canvas.Triangles()) == 0 {
		t.Fatal("官方目标条没画出 goal-outline 图标（没有任何描边三角形）")
	}
	goal = ""
	canvas = officialFrame(t, true, func() string { return goal })
	if rects := canvas.Rectangles(); len(rects) != 0 {
		t.Fatalf("清掉目标之后应当整块消失，得到 %+v", rects)
	}
}

// ④ 每帧 dispatch 一次、每帧只画一次：连画两帧，每帧各出现一份。
func TestGoalDrawsOncePerFrame(t *testing.T) {
	officialFrame(t, true, func() string { return "同一目标" })
	second := officialFrame(t, true, func() string { return "同一目标" })
	var labels int
	for _, text := range second.Texts() {
		if text.Value == goalLabelText {
			labels++
		}
	}
	if labels != 1 {
		t.Fatalf("第二帧目标标签画了 %d 次，want 1（每帧重画一次）", labels)
	}
	if len(second.Triangles()) == 0 {
		t.Fatal("第二帧没画出 goal-outline 图标")
	}
}
