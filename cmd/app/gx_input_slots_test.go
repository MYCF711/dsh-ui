// gx_input_slots_test.go —— 产品装配级证据：官方 composer 的子槽派发 + 模型标签唯一性。
//
// # 本单解决了什么
//
// 改造前：模型名是 `plugins/dscomposer` 在控件行里**硬编码画的只读 chip**，而
// `plugins/dsagentteam` 的官方模型菜单只挂了一块透明命中区——因为一旦真的
// dispatch `conversation.input.model`，画布上就会同时出现旧 chip 与官方 trigger
// 两个模型标签（GR 报告 §8 第 1 条）。
//
// 现在：`conversation.composer` 座位是**父容器**，每帧把 `conversation.input.*`
// 子槽各派发一次；`conversation.input.model` 的占用方（dsagentteam 的官方
// trigger）画模型标签，composer 只在**没有占用方**时才画回退 chip。判据是内核的
// `env.Has(OffInputModel)`——与壳判 `BridgeLegacyComposer` 同一个事实。
//
// # 四条判据
//
//	① 官方座位每帧 dispatch 全 10 个 `conversation.input.*` 子槽各 1 次；
//	   `main.composer` 同帧 0 次（不双画）。
//	② 画布上模型标签**恰好 1 个**（占用方在 → composer 不画；反之亦然）。
//	③ 点 trigger 真开官方菜单：provider 行与官方 Team 开关都出现，且标签仍只有 1 个。
//	④ 旧三栏档（产品默认 auto→dsshell）仍只有一个标签、菜单仍点得开——它走的是
//	   dsagentteam 的兼容命中区（过渡路径，登记在报告里），不派发官方子槽。
package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dsagentteam"
	"github.com/MYCF711/dsh-ui/plugins/dscomposer"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const (
	// gxModelLabel 是这一帧的宿主模型真值。
	gxModelLabel = "deepseek-v4-pro"
	// gxModelMenuAria 是官方模型 trigger 的无障碍名（dsagentteam 的 `zhModelMenuAria`）。
	gxModelMenuAria = "模型与推理等级"
	// gxTeamModeLabel 是官方 Team 开关的文案（`locales.ts` 的 `mode.label`）。
	gxTeamModeLabel = "启用智能体团队协作"
)

// gxComposerInputSubSlots 是 composer 作为父容器每帧派发的官方子槽全集。
//
// 必须与 `plugins/dscomposer/input_slots.go` 的两张表逐字一致（那边有编译期
// 自检；这里再写一遍是为了让"产品装配下的派发次数"能独立于实现被钉住）。
var gxComposerInputSubSlots = []pluginapi.Slot{
	uikernel.OffConvComposerDock,
	uikernel.OffInputAttachments,
	uikernel.OffInputOverlay,
	uikernel.OffInputPermission,
	uikernel.OffInputPlan,
	uikernel.OffInputLeft,
	uikernel.OffInputRight,
	uikernel.OffInputModel,
	uikernel.OffInputActivity,
	uikernel.OffInputDock,
}

// gxProbe 在每一个官方子槽上挂一块"只记账"的画（见 dscomposer 的同名探针：
// 子槽由 composer 在 `Children` 里直派，壳级 dispatch 回调看不到）。
type gxProbe struct {
	counts    map[pluginapi.Slot]int
	skipModel bool
}

func (p *gxProbe) ID() string                     { return "gx-input-slot-probe" }
func (*gxProbe) Provides() []string               { return nil }
func (*gxProbe) Requires() []string               { return []string{pluginapi.KeyTheme} }
func (*gxProbe) Register(pluginapi.Context) error { return nil }

func (p *gxProbe) Panels() []pluginapi.Painter {
	out := make([]pluginapi.Painter, 0, len(gxComposerInputSubSlots))
	for _, slot := range gxComposerInputSubSlots {
		if p.skipModel && slot == uikernel.OffInputModel {
			continue
		}
		slot := slot
		out = append(out, pluginapi.Painter{
			ID:   "gx-probe:" + string(slot),
			Slot: slot,
			Draw: func(ui.C, *pluginapi.Env) { p.counts[slot]++ },
		})
	}
	return out
}

// gxProduct 是产品插件集（旧部件 + 官方 composer + 官方模型菜单）的一帧装配。
type gxProduct struct {
	kernel     *uikernel.Kernel
	composer   *dscomposer.Plugin
	team       *dsagentteam.Store
	theme      *pluginapi.Theme
	tree       *gfx.Tree
	feat       gfx.Scope
	opts       uikernel.ShellOptions
	dispatched map[pluginapi.Slot]int
	probe      *gxProbe
	// mode 是这一帧的结构档位：官方档走官方骨架，旧三栏档走 dsshell 形状。
	mode shellMode
}

// newGXProduct 装配产品插件集。
//
// shellMode 决定这一帧走哪个 composer 座位：
//
//	shellOfficial → 官方座位（`conversation.composer`）
//	shellLegacy   → 旧三栏座位（`main.composer`，产品默认 auto 时 dsshell 占根）
func newGXProduct(t *testing.T, mode shellMode) *gxProduct {
	t.Helper()
	p := &gxProduct{
		kernel:     uikernel.New(fmt.Sprintf("gx-input-slots-%d", mode)),
		composer:   &dscomposer.Plugin{},
		tree:       &gfx.Tree{},
		dispatched: map[pluginapi.Slot]int{},
		mode:       mode,
	}
	p.probe = &gxProbe{counts: p.dispatched, skipModel: mode == shellLegacy}
	if err := p.kernel.Load(
		&dscore.Plugin{Dark: true},
		&dssidebar.Plugin{},
		&dsconversation.Plugin{},
		p.composer,
		&dsagentteam.Plugin{},
		p.probe,
	); err != nil {
		t.Fatalf("装配产品插件集失败：%v", err)
	}
	p.theme = p.kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.feat = p.tree.Feature(fmt.Sprintf("gx-input-slots-%d", mode))
	store, ok := p.kernel.Get(dsagentteam.ServiceKey).(*dsagentteam.Store)
	if !ok {
		t.Fatalf("dsagentteam.ServiceKey 不是 *Store：%T", p.kernel.Get(dsagentteam.ServiceKey))
	}
	p.team = store
	// 官方 provider 目录：真宿主 `llm.listConfigurableProviders` 的形状。
	store.SetProviders(true, "", []dsagentteam.ModelProviderOption{
		{Provider: "deepseek", DisplayName: "DeepSeek", Seed: "llm"},
	})
	m := &model{kernel: p.kernel, cur: "s-gx", shellMode: mode}
	p.opts = shellOptionsFor(m, p.kernel, 1182, 832)
	return p
}

// paint 画一帧产品路径；click 非空时按该交互 id 造一次事件。
func (p *gxProduct) paint(t *testing.T, snap pluginapi.Snapshot, click string) *gfx.Canvas {
	t.Helper()
	canvas := &gfx.Canvas{}
	canvas.SetTree(p.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		n := 0
		for range value {
			n++
		}
		return float32(n) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: p.theme, Data: snap, Tree: p.tree, Feat: p.feat, Canvas: canvas,
		Bounds:    ui.Rect{Width: 1182, Height: 832},
		MainWidth: 902,
		Geometry:  pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902},
		Has:       func(slot pluginapi.Slot) bool { return p.kernel.Has(slot) },
	}
	for slot := range p.dispatched {
		delete(p.dispatched, slot)
	}
	if click != "" {
		canvas.BeginEventPass(p.tree.ID(p.feat, click))
	}
	var view ui.Page
	view.Draw(canvas, dscore.VivaTheme(p.theme), env.Bounds, func(f ui.C) {
		uikernel.FirstScreen(f, env, p.opts, func(slot pluginapi.Slot) {
			p.dispatched[slot]++
			p.kernel.DrawSlot(f, slot, env)
		})
	})
	if click != "" {
		canvas.EndEventPass()
	}
	// 布局已定：跑插件的 AfterDraw（dsagentteam 在那里缓存 trigger 矩形）。
	for _, painter := range p.kernel.AllPainters() {
		if painter.AfterDraw != nil {
			painter.AfterDraw(canvas, env)
		}
	}
	return canvas
}

// paintLegacySeat 画**旧三栏档**的 composer 座位：dsshell 那条路只 dispatch
// `main.composer`（壳里没有官方骨架），所以这里也只画那个座位。
//
// 为什么不复用 `paint`：官方骨架的 `BridgeLegacyComposer` 判据是"官方 composer
// 座位有没有占用方"——dscomposer 注册了官方座位，所以官方骨架永远走官方座位；
// 旧三栏档根本不经过官方骨架（dsshell 自己 dispatch `main.composer`）。
func (p *gxProduct) paintLegacySeat(t *testing.T, snap pluginapi.Snapshot) *gfx.Canvas {
	t.Helper()
	canvas := &gfx.Canvas{}
	canvas.SetTree(p.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		n := 0
		for range value {
			n++
		}
		return float32(n) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: p.theme, Data: snap, Tree: p.tree, Feat: p.feat, Canvas: canvas,
		Bounds:    ui.Rect{Width: 1182, Height: 832},
		MainWidth: 902,
		Geometry:  pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902},
		Has:       func(slot pluginapi.Slot) bool { return p.kernel.Has(slot) },
	}
	for slot := range p.dispatched {
		delete(p.dispatched, slot)
	}
	var view ui.Page
	view.Draw(canvas, dscore.VivaTheme(p.theme), env.Bounds, func(f ui.C) {
		// dsshell drawMain 的形状：中栏 Pad(12) 里 dispatch `main.composer`。
		ui.Column(f).FillWidth().FillHeight().Pad(12).Children(func() {
			p.dispatched[pluginapi.SlotMainComposer]++
			p.kernel.DrawSlot(f, pluginapi.SlotMainComposer, env)
		})
	})
	for _, painter := range p.kernel.AllPainters() {
		if painter.AfterDraw != nil {
			painter.AfterDraw(canvas, env)
		}
	}
	return canvas
}

// gxSnap 是这一帧的宿主快照（模型真值 + 权限 + 上下文）。
func gxSnap() pluginapi.Snapshot {
	return pluginapi.Snapshot{
		Cur: "s-gx", Model: gxModelLabel, Permission: "ask",
		ContextTokens: 12345, ContextWindow: 200000,
	}
}

// gxTextCount 数一个精确文本出现次数。
func gxTextCount(canvas *gfx.Canvas, value string) int {
	n := 0
	for _, text := range canvas.Texts() {
		if text.Value == value {
			n++
		}
	}
	return n
}

// gxTexts 列画布文本（失败诊断用）。
func gxTexts(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

// gxHasText 报告画布上有没有这段文本。
func gxHasText(canvas *gfx.Canvas, want string) bool {
	return strings.Contains(strings.Join(gxTexts(canvas), "\n"), want)
}

// gxHit 取某个标签的命中区。
func gxHit(canvas *gfx.Canvas, label string) (gfx.Hit, bool) {
	for _, hit := range canvas.Hits() {
		if hit.Label == label {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// TestOfficialComposerDispatchesInputSubSlots ①：官方座位每帧派发全部子槽各一次。
func TestOfficialComposerDispatchesInputSubSlots(t *testing.T) {
	p := newGXProduct(t, shellOfficial)
	if p.opts.BridgeLegacyComposer {
		t.Fatal("官方 composer 座位有占用方时不该走兼容桥")
	}
	p.paint(t, gxSnap(), "")

	if got := p.dispatched[uikernel.OffConvComposer]; got != 1 {
		t.Fatalf("官方 composer 座位 dispatch = %d，want 1（全部=%v）", got, p.dispatched)
	}
	if got := p.dispatched[pluginapi.SlotMainComposer]; got != 0 {
		t.Fatalf("官方座位活着时 main.composer 也被 dispatch（双画）：%d", got)
	}
	for _, slot := range gxComposerInputSubSlots {
		if got := p.dispatched[slot]; got != 1 {
			t.Fatalf("子槽 %q 一帧 dispatch = %d，want 1（全部=%v）", slot, got, p.dispatched)
		}
	}
}

// TestOfficialComposerSingleModelLabel ②：占用方在，模型标签只有一个。
func TestOfficialComposerSingleModelLabel(t *testing.T) {
	p := newGXProduct(t, shellOfficial)
	canvas := p.paint(t, gxSnap(), "")
	if got := gxTextCount(canvas, gxModelLabel); got != 1 {
		t.Fatalf("模型标签出现 %d 次，want 恰好 1（双标签？）；文本=%v", got, gxTexts(canvas))
	}
	hit, ok := gxHit(canvas, gxModelMenuAria)
	if !ok {
		t.Fatalf("官方 trigger 没有命中区（模型菜单点不开）；hits=%v", canvas.Hits())
	}
	if hit.Bounds.Height != 28 {
		t.Fatalf("trigger 高 = %v，官方 28", hit.Bounds.Height)
	}
	// 官方 trigger 在 composer 控件行里：底部区域内，不在窗口顶端。
	if hit.Bounds.Y < 700 {
		t.Fatalf("trigger 不在 composer 控件行（y=%v），像是画到别处了", hit.Bounds.Y)
	}
	if _, ok := gxHit(canvas, "发送"); !ok {
		t.Fatalf("控件行没有发送键；hits=%v", canvas.Hits())
	}
}

// TestOfficialModelMenuOpensWithTeamToggle ③：点 trigger 真开官方菜单。
func TestOfficialModelMenuOpensWithTeamToggle(t *testing.T) {
	p := newGXProduct(t, shellOfficial)
	snap := gxSnap()

	// 官方 Team 开关的可见性判据：`agentTeam` 投影已知（TeamModeToggle.tsx:43）。
	// 数据形状照官方投影（`View.ProjectionKnown` + `TeamProjection.Enabled`）。
	p.team.SetView(dsagentteam.View{
		SessionID:       "s-gx",
		ProjectionKnown: true,
		Projection:      dsagentteam.TeamProjection{Enabled: true},
	})

	// 第一帧只画 trigger（AfterDraw 缓存它的真实矩形）。
	first := p.paint(t, snap, "")
	if _, ok := gxHit(first, gxModelMenuAria); !ok {
		t.Fatalf("第一帧没有 trigger 命中区；hits=%v", first.Hits())
	}
	// 第二帧点它。
	canvas := p.paint(t, snap, "model-menu-trigger")
	if !p.team.ModelMenuOpen() {
		t.Fatal("点了 trigger 但菜单没开")
	}
	if !gxHasText(canvas, "DeepSeek") {
		t.Fatalf("菜单里没有 provider 行；文本=%v", gxTexts(canvas))
	}
	if !gxHasText(canvas, gxTeamModeLabel) {
		t.Fatalf("模型菜单里没有官方 Team 开关 %q；文本=%v", gxTeamModeLabel, gxTexts(canvas))
	}
	if got := gxTextCount(canvas, gxModelLabel); got != 1 {
		t.Fatalf("菜单展开后模型标签出现 %d 次，want 1；文本=%v", got, gxTexts(canvas))
	}
}

// TestLegacyComposerKeepsSingleModelLabel ④：旧三栏档（产品默认）仍只有一个标签。
//
// 旧座位不派发官方子槽（登记的过渡差距）：模型 chip 由 composer 只读画，
// dsagentteam 的兼容命中区盖在它上面提供菜单——两个座位每帧只活一个。
func TestLegacyComposerKeepsSingleModelLabel(t *testing.T) {
	p := newGXProduct(t, shellLegacy)
	canvas := p.paintLegacySeat(t, gxSnap())

	if got := p.dispatched[pluginapi.SlotMainComposer]; got != 1 {
		t.Fatalf("旧座位 main.composer dispatch = %d，want 1（全部=%v）", got, p.dispatched)
	}
	if got := p.dispatched[uikernel.OffConvComposer]; got != 0 {
		t.Fatalf("旧座位活着时官方座位也被 dispatch（双画）：%d", got)
	}
	for _, slot := range gxComposerInputSubSlots {
		if got := p.dispatched[slot]; got != 0 {
			t.Fatalf("旧三栏档不该派发官方子槽 %q（过渡差距已登记），实际 %d 次", slot, got)
		}
	}
	if got := gxTextCount(canvas, gxModelLabel); got != 1 {
		t.Fatalf("旧三栏档模型标签出现 %d 次，want 1；文本=%v", got, gxTexts(canvas))
	}
	if _, ok := gxHit(canvas, gxModelMenuAria); !ok {
		t.Fatalf("旧三栏档缺模型菜单命中区（dsagentteam 兼容 painter），菜单点不开；hits=%v",
			canvas.Hits())
	}
}

// gxSwitchHit 取官方 Team 开关本体的命中区（viva 的 Switch 命中标签固定是 "switch"）。
func gxSwitchHit(canvas *gfx.Canvas) (gfx.Hit, bool) {
	for _, hit := range canvas.Hits() {
		if hit.Label == "switch" {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}
