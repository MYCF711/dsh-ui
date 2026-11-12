// model_slot_test.go —— 官方 `conversation.input.model` 槽的**占用方**证据。
//
// 与 parent_containers_test.go 的分工：那边测"容器把子槽交回内核"，这边测
// "占用方自己画出来的 trigger / 降级 chip / 菜单锚点"。
//
// 三条判据：
//
//	① trigger 两态：目录可用 → 官方 trigger（可点、有 chevron、展开态）；
//	   目录未知 → 只读 chip（宿主真值 / 「未知」），**没有命中区**（假入口）。
//	② 点击开菜单：命中区就是 trigger 自己的矩形；菜单锚点用**上一帧 trigger 的
//	   真实矩形**（AfterDraw 缓存），不再依赖过渡几何。
//	③ 只有一个模型标签：trigger 文字在画布上恰好出现一次，菜单展开也不变。
package dsagentteam

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const (
	// triggerTop / triggerLeft 把 trigger 摆到窗口下半部偏右（composer 控件行的大致位置）。
	triggerTop  float32 = 600
	triggerLeft float32 = 200
)

// modelSlotHarness 是"真内核 + 真插件 + 真 dispatch + 真 AfterDraw"的测试台。
type modelSlotHarness struct {
	kernel   *uikernel.Kernel
	plugin   *Plugin
	store    *Store
	tree     *gfx.Tree
	feat     gfx.Scope
	rendered []pluginapi.Slot
	// painter 是官方模型槽上的 painter（Panels() 返回的切片副本，这里只读）。
	painter pluginapi.Painter
}

func newModelSlotHarness(t *testing.T) *modelSlotHarness {
	t.Helper()
	kernel := uikernel.New("agent-team-model-slot-test")
	plugin := &Plugin{}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, plugin); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	store, ok := kernel.Get(ServiceKey).(*Store)
	if !ok {
		t.Fatalf("ServiceKey 不是 *Store：%T", kernel.Get(ServiceKey))
	}
	tree := &gfx.Tree{}
	var painter pluginapi.Painter
	for _, candidate := range plugin.Panels() {
		if candidate.ID == "agent-team.model-menu" {
			painter = candidate
		}
	}
	if painter.ID == "" {
		t.Fatal("官方模型槽上的 painter 缺失")
	}
	return &modelSlotHarness{
		kernel: kernel, plugin: plugin, store: store, tree: tree,
		feat: tree.Feature("agent-team-model-slot"), painter: painter,
	}
}

// paint 画一帧：Draw → Children（真内核 DrawSlot）→ 全部 AfterDraw。
//
// click 非空时按该交互 id 造一次事件（等价于用户点了它）。
func (h *modelSlotHarness) paint(t *testing.T, snap pluginapi.Snapshot, click string) *gfx.Canvas {
	t.Helper()
	theme := dscore.Dark()
	canvas := &gfx.Canvas{}
	canvas.SetTree(h.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: theme, Data: snap, Tree: h.tree, Feat: h.feat, Canvas: canvas,
		Bounds:    ui.Rect{Width: 1180, Height: 800},
		MainWidth: 900,
		Geometry:  pluginapi.Geometry{Width: 1180, Height: 800, MainWidth: 900, SidebarWidth: 280},
		Has:       func(slot pluginapi.Slot) bool { return h.kernel.Has(slot) },
	}
	h.rendered = nil
	if click != "" {
		canvas.BeginEventPass(h.tree.ID(h.feat, click))
	}
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(theme), env.Bounds, func(f *flex.Frame) {
		// 把 trigger 放在窗口下半部（模拟它真实的位置：composer 在底部）。
		// 顶部贴边的话 `Popup.Above` 会被窗口边界夹住，"菜单在 trigger 上方"
		// 这条就测不出来了。
		f.Column().FillWidth().FillHeight().PadSides(triggerTop, 0, 0, triggerLeft).Children(func() {
			// 与内核同一个顺序：Draw（trigger）→ Children（菜单 + 子槽派发）。
			h.painter.Draw(f, env)
			h.painter.Children(f, env, func(slot pluginapi.Slot) {
				h.rendered = append(h.rendered, slot)
				h.kernel.DrawSlot(f, slot, env)
			})
		})
	})
	if click != "" {
		canvas.EndEventPass()
	}
	// 布局已定：AfterDraw 缓存 trigger 的真实矩形（下一帧菜单锚点）。
	if h.painter.AfterDraw != nil {
		h.painter.AfterDraw(canvas, env)
	}
	return canvas
}

func (h *modelSlotHarness) renderedSlot(slot pluginapi.Slot) int {
	n := 0
	for _, got := range h.rendered {
		if got == slot {
			n++
		}
	}
	return n
}

// textCount 数一个精确文本出现次数。
func textCount(canvas *gfx.Canvas, value string) int {
	n := 0
	for _, text := range canvas.Texts() {
		if text.Value == value {
			n++
		}
	}
	return n
}

// hitFor 取某个标签的命中区。
func hitFor(canvas *gfx.Canvas, label string) (gfx.Hit, bool) {
	for _, hit := range canvas.Hits() {
		if hit.Label == label {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// TestModelSlotTriggerStates ①：trigger 的两态（可用 / 降级）。
func TestModelSlotTriggerStates(t *testing.T) {
	t.Run("目录未知：宿主真值 → 只读 chip，无命中区", func(t *testing.T) {
		h := newModelSlotHarness(t)
		canvas := h.paint(t, pluginapi.Snapshot{Cur: "s-1", Model: "deepseek-v4-pro"}, "")
		if got := textCount(canvas, "deepseek-v4-pro"); got != 1 {
			t.Fatalf("降级 chip 的模型文本出现 %d 次，want 1；texts=%v", got, canvas.Texts())
		}
		if _, ok := hitFor(canvas, zhModelMenuAria); ok {
			t.Fatal("目录未知时不该有命中区（点了只会开空菜单 = 假入口）")
		}
		if got := h.renderedSlot(SlotInputModelOptions); got != 0 {
			t.Fatalf("降级态不该派发菜单子槽；rendered=%v", h.rendered)
		}
	})

	t.Run("目录未知且宿主没给模型：「未知」，不编默认型号", func(t *testing.T) {
		h := newModelSlotHarness(t)
		canvas := h.paint(t, pluginapi.Snapshot{Cur: "s-1"}, "")
		if got := textCount(canvas, zhModelUnknown); got != 1 {
			t.Fatalf("应显示一个 %q 的只读 chip，实际 %d；texts=%v", zhModelUnknown, got, canvas.Texts())
		}
		for _, banned := range []string{"0", "默认模型"} {
			if textCount(canvas, banned) > 0 {
				t.Fatalf("降级态编造了 %q；texts=%v", banned, canvas.Texts())
			}
		}
	})

	t.Run("目录可用：官方 trigger 带命中区（未展开）", func(t *testing.T) {
		h := newModelSlotHarness(t)
		h.store.SetProviders(true, "", []ModelProviderOption{
			{Provider: "deepseek", DisplayName: "DeepSeek", Seed: "llm"},
		})
		canvas := h.paint(t, pluginapi.Snapshot{Cur: "s-1", Model: "deepseek-v4-pro"}, "")
		if got := textCount(canvas, "deepseek-v4-pro"); got != 1 {
			t.Fatalf("trigger 文本出现 %d 次，want 1；texts=%v", got, canvas.Texts())
		}
		hit, ok := hitFor(canvas, zhModelMenuAria)
		if !ok {
			t.Fatalf("目录可用时 trigger 必须有命中区；hits=%v", canvas.Hits())
		}
		if !hit.Expandable || hit.Expanded {
			t.Fatalf("trigger 应是可展开且当前未展开：expandable=%v expanded=%v", hit.Expandable, hit.Expanded)
		}
		if hit.Role != gfx.RoleButton {
			t.Fatalf("trigger 的 role = %v，官方是 button", hit.Role)
		}
		if hit.Bounds.Height != modelChipHeight {
			t.Fatalf("trigger 高 = %v，官方 %v", hit.Bounds.Height, modelChipHeight)
		}
	})
}

// TestModelSlotClickOpensMenu ②：点 trigger 真开菜单，锚点是 trigger 自己的矩形。
func TestModelSlotClickOpensMenu(t *testing.T) {
	h := newModelSlotHarness(t)
	h.store.SetProviders(true, "", []ModelProviderOption{
		{Provider: "deepseek", DisplayName: "DeepSeek", Seed: "llm"},
	})
	snap := pluginapi.Snapshot{Cur: "s-1", Model: "deepseek-v4-pro"}

	// 第一帧：只画 trigger。布局结束后 AfterDraw 把它的矩形缓存下来。
	first := h.paint(t, snap, "")
	hit, ok := hitFor(first, zhModelMenuAria)
	if !ok {
		t.Fatalf("第一帧没有 trigger 命中区；hits=%v", first.Hits())
	}
	if rect := h.plugin.model.rect; rect != hit.Bounds {
		t.Fatalf("AfterDraw 缓存的 trigger 矩形 = %+v，画布上是 %+v", rect, hit.Bounds)
	}
	if h.store.ModelMenuOpen() {
		t.Fatal("还没点就展开了")
	}

	// 第二帧：点它 → 开菜单 + 派发 `conversation.input.model.options`。
	canvas := h.paint(t, snap, "model-menu-trigger")
	if !h.store.ModelMenuOpen() {
		t.Fatal("点了 trigger 但菜单没开")
	}
	if got := h.renderedSlot(SlotInputModelOptions); got != 1 {
		t.Fatalf("菜单展开必须派发一次 %q；rendered=%v", SlotInputModelOptions, h.rendered)
	}
	if !anyTextContains(canvas, "DeepSeek") {
		t.Fatalf("菜单里没有 provider 行；texts=%v", canvas.Texts())
	}
	// 展开态写进无障碍属性（官方 `.trigger[aria-expanded=true]`）。
	//
	// 点击这一帧的 trigger 是**点击前**画的（hit 的 expanded 取自那一刻的
	// Store 状态），官方 React 也是下一次 render 才把新状态写进属性——所以这里
	// 再画一帧，断言稳定态。
	next := h.paint(t, snap, "")
	openHit, ok := hitFor(next, zhModelMenuAria)
	if !ok || !openHit.Expanded {
		t.Fatalf("展开后 trigger 应报 expanded=true：%+v ok=%v", openHit, ok)
	}
	if got := h.renderedSlot(SlotInputModelOptions); got != 1 {
		t.Fatalf("稳定态每帧都要派发一次 %q；rendered=%v", SlotInputModelOptions, h.rendered)
	}
	// 菜单在 trigger **上方**（官方 `bottom:calc(anchor(top)+8px)`）。
	anchor := h.plugin.model.rect
	for _, text := range canvas.Texts() {
		if text.Value != "DeepSeek" {
			continue
		}
		if text.Bounds.Y+text.Bounds.Height > anchor.Y {
			t.Fatalf("菜单不在 trigger 上方：provider 行底 = %v，trigger 顶 = %v",
				text.Bounds.Y+text.Bounds.Height, anchor.Y)
		}
	}
	// 只有一个模型标签：菜单展开不复制 trigger 文本。
	if got := textCount(canvas, "deepseek-v4-pro"); got != 1 {
		t.Fatalf("菜单展开后模型文本出现 %d 次，want 1；texts=%v", got, canvas.Texts())
	}
}
