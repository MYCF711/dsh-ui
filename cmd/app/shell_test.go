//go:build darwin || windows || linux

package main

// shell_test.go —— L0 接线断言：档位解析、几何同源、官方骨架真的 dispatch 官方槽。
//
// 这里**不重复** uikernel 的结构断言（槽覆盖/几何表/rc.2 三处真变都在
// internal/uikernel/official_shell_test.go）。本文件只验壳这一层的三件事：
//
//	① -shell 三档解析与 auto 的判定（有 shell.root 占用方 → 旧路径）；
//	② officialGeometry 与骨架同一份三列求解（不再有第二套算术）；
//	③ 强制 official 时：官方槽与旧部件兼容桥都真的被 dispatch。

import (
	"reflect"
	"sort"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// slotProbe 是只做一件事的插件：登记一个槽，画的时候把槽名记下来。
type slotProbe struct {
	id     string
	slots  []pluginapi.Slot
	drawn  *[]pluginapi.Slot
	panels []pluginapi.Painter
}

func (p *slotProbe) ID() string                       { return p.id }
func (p *slotProbe) Provides() []string               { return nil }
func (p *slotProbe) Requires() []string               { return nil }
func (p *slotProbe) Register(pluginapi.Context) error { return nil }
func (p *slotProbe) Panels() []pluginapi.Painter {
	if p.panels != nil {
		return p.panels
	}
	p.panels = make([]pluginapi.Painter, 0, len(p.slots))
	for _, slot := range p.slots {
		slot := slot
		p.panels = append(p.panels, pluginapi.Painter{
			ID: p.id + "-" + string(slot), Slot: slot,
			Draw: func(ui.C, *pluginapi.Env) { *p.drawn = append(*p.drawn, slot) },
		})
	}
	return p.panels
}

func newProbeKernel(t *testing.T, id string, slots ...pluginapi.Slot) (*uikernel.Kernel, *[]pluginapi.Slot) {
	t.Helper()
	k := uikernel.New("shell-test-" + id)
	drawn := &[]pluginapi.Slot{}
	if err := k.Load(&dscore.Plugin{Dark: true}, &slotProbe{id: id, slots: slots, drawn: drawn}); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	return k, drawn
}

func TestParseShellMode(t *testing.T) {
	cases := map[string]shellMode{
		"":         shellAuto,
		"auto":     shellAuto,
		"legacy":   shellLegacy,
		"official": shellOfficial,
	}
	for raw, want := range cases {
		got, err := parseShellMode(raw)
		if err != nil {
			t.Fatalf("parseShellMode(%q) 报错：%v", raw, err)
		}
		if got != want {
			t.Fatalf("parseShellMode(%q) = %v，期望 %v", raw, got, want)
		}
	}
	for _, bad := range []string{"offical", "OFFICIAL", "auto ", "1"} {
		if _, err := parseShellMode(bad); err == nil {
			t.Fatalf("parseShellMode(%q) 应该报错（拼错不许静默回落 auto）", bad)
		}
	}
	if shellAuto.String() != "auto" || shellLegacy.String() != "legacy" || shellOfficial.String() != "official" {
		t.Fatalf("三档 String() 不一致：%q/%q/%q", shellAuto, shellLegacy, shellOfficial)
	}
}

func TestShellModeAutoResolvesByRootOccupant(t *testing.T) {
	withRoot, _ := newProbeKernel(t, "root-owner", pluginapi.SlotShellRoot)
	if got := shellAuto.resolve(withRoot); got != shellLegacy {
		t.Fatalf("有 shell.root 占用方时 auto 应走 legacy，得到 %v", got)
	}
	without, _ := newProbeKernel(t, "no-root", pluginapi.SlotMainBody)
	if got := shellAuto.resolve(without); got != shellOfficial {
		t.Fatalf("没有 shell.root 占用方时 auto 应走 official 骨架，得到 %v", got)
	}
	if got := shellLegacy.resolve(withRoot); got != shellLegacy {
		t.Fatalf("强制 legacy 被改写成 %v", got)
	}
	if got := shellOfficial.resolve(withRoot); got != shellOfficial {
		t.Fatalf("强制 official 被改写成 %v", got)
	}
	// 内核还没装配（nil）时不许 panic：按没有占用方处理。
	if got := shellAuto.resolve(nil); got != shellOfficial {
		t.Fatalf("nil 内核时 auto 应为 official，得到 %v", got)
	}
}

func TestOfficialGeometryMatchesOfficialColumns(t *testing.T) {
	cases := []struct {
		name                             string
		width, height                    float32
		sidebarPref                      float32
		rightbar                         bool
		wantSidebar, wantMain, wantRight float32
	}{
		{name: "1920x1032 右栏开", width: 1920, height: 1032, sidebarPref: 280, rightbar: true, wantSidebar: 280, wantMain: 776, wantRight: 864},
		{name: "1182x832 右栏开（中栏恰好 400）", width: 1182, height: 832, sidebarPref: 280, rightbar: true, wantSidebar: 280, wantMain: 400, wantRight: 502},
		{name: "960x832 右栏放不下 → 整列消失", width: 960, height: 832, sidebarPref: 280, rightbar: true, wantSidebar: 280, wantMain: 680, wantRight: 0},
		{name: "1920x1032 右栏关", width: 1920, height: 1032, sidebarPref: 280, wantSidebar: 280, wantMain: 1640, wantRight: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := uikernel.ShellOptions{SidebarPreferred: tc.sidebarPref, RightbarOn: tc.rightbar}
			geom := officialGeometry(tc.width, tc.height, opts)
			if geom.SidebarWidth != tc.wantSidebar || geom.MainWidth != tc.wantMain || geom.RightWidth != tc.wantRight {
				t.Fatalf("几何 = sidebar %v / main %v / right %v，期望 %v/%v/%v",
					geom.SidebarWidth, geom.MainWidth, geom.RightWidth, tc.wantSidebar, tc.wantMain, tc.wantRight)
			}
			if geom.Width != tc.width || geom.Height != tc.height {
				t.Fatalf("视口尺寸 = %v×%v，期望 %v×%v", geom.Width, geom.Height, tc.width, tc.height)
			}
			// 三列之和必须严格等于视口（官方 solveColumns 的契约）。
			if got := geom.SidebarWidth + geom.MainWidth + geom.RightWidth; got != tc.width {
				t.Fatalf("三列之和 %v ≠ 视口 %v（列宽算出空隙/溢出）", got, tc.width)
			}
			if geom.BottomHeight != 0 {
				t.Fatalf("shell.bottom 空内容必须不预留高度，得到 %v", geom.BottomHeight)
			}
		})
	}
}

func TestShellOptionsForRealState(t *testing.T) {
	opts := shellOptionsFor(&model{cur: "s1"}, nil, 1182, 832)
	if opts.Phase != uikernel.PhaseActive {
		t.Fatalf("有会话时 phase 应为 active，得到 %v", opts.Phase)
	}
	if opts.Blank {
		t.Fatal("有会话时不该是 blank header")
	}
	if opts.Windows {
		t.Fatal("bounds 已经是内容区（caption 由 L0 扣过），Options.Windows 必须 false，否则二次扣 40px")
	}
	// Windows 上侧栏收起后整列消失（rail 0）；这是一条产品口径，不该被 cfg 覆盖。
	if opts.CollapsedWidth != 0 {
		t.Fatalf("Windows 上 CollapsedWidth 应为 0（官方 rail=0），得到 %v", opts.CollapsedWidth)
	}

	hero := shellOptionsFor(&model{cur: ""}, nil, 1182, 832)
	if hero.Phase != uikernel.PhaseHero {
		t.Fatalf("没有会话时 phase 应为 hero，得到 %v", hero.Phase)
	}
	if !hero.Blank {
		t.Fatal("没有会话时 header 是 blank 态（官方 40px 行）")
	}

	// 侧栏宽度偏好来自 dssidebar 的真实状态（Register 后是 280）；宽度未知时落官方
	// 默认 280 而不是 0（0 在 official 档位 = 整列消失，不能拿它当「未知」）。
	side := &dssidebar.Plugin{}
	optKernel := uikernel.New("shell-opts-test")
	if err := optKernel.Load(&dscore.Plugin{Dark: true}, side); err != nil {
		t.Fatalf("装配 dssidebar 失败：%v", err)
	}
	withSide := shellOptionsFor(&model{cur: "s1", side: side}, optKernel, 1182, 832)
	if withSide.SidebarPreferred != side.Width() || withSide.SidebarPreferred != 280 {
		t.Fatalf("侧栏偏好 = %v，dssidebar 实际是 %v（应为 280）", withSide.SidebarPreferred, side.Width())
	}
	unknown := shellOptionsFor(&model{cur: "s1"}, optKernel, 1182, 832)
	if unknown.SidebarPreferred != uikernel.SidebarDefault {
		t.Fatalf("宽度未知时应落官方默认 %v，得到 %v", uikernel.SidebarDefault, unknown.SidebarPreferred)
	}
}

// TestOfficialShellWiringDispatchesOfficialSlots 用真实内核 + 无头 flex 画一帧，
// 断言强制 official 档位下：
//   - 官方槽被 dispatch（不是画了一个空壳）；
//   - 旧部件还在时兼容桥把旧槽也 dispatch（否则切过去内容全没了）。
func TestOfficialShellWiringDispatchesOfficialSlots(t *testing.T) {
	legacySlots := []pluginapi.Slot{
		pluginapi.SlotSidebarHeader, pluginapi.SlotSidebarList, pluginapi.SlotSidebarFooter,
		pluginapi.SlotMainHeader, pluginapi.SlotMainBody, pluginapi.SlotMainComposer,
		pluginapi.SlotRightbar, pluginapi.SlotShellOverlay,
	}
	kernel, drawn := newProbeKernel(t, "legacy", legacySlots...)
	// 只装旧部件：official 档位下它们必须靠兼容桥显示出来。
	// （official 槽本身的 dispatch 由下面第二个用例用另一副内核断言：
	//   一旦有插件占了官方槽，桥就必须让位——官方语义是「注册在这里 = 整列替换」。）

	m := &model{kernel: kernel, showRightbar: true, cur: "s1"}
	opts := shellOptionsFor(m, kernel, 1182, 832)
	if !opts.BridgeLegacy {
		t.Fatal("内核里有 sidebar.list / main.body 占用方，兼容桥必须打开")
	}

	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len([]rune(value))) * size / 2 })
	theme := dscore.Dark()
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("shell-official-test"), Canvas: canvas,
		Bounds: ui.Rect{Width: 1182, Height: 832},
		Has:    func(slot pluginapi.Slot) bool { return kernel.Has(slot) },
	}
	var view ui.Page
	view.Draw(canvas, dscore.VivaTheme(theme), env.Bounds, func(f ui.C) {
		drawOfficialFirstScreen(f, m, env, opts)
	})

	got := map[pluginapi.Slot]bool{}
	for _, slot := range *drawn {
		got[slot] = true
	}
	for _, slot := range []pluginapi.Slot{
		pluginapi.SlotRightbar,
		// 兼容桥：旧部件仍要显示
		pluginapi.SlotSidebarHeader, pluginapi.SlotSidebarList, pluginapi.SlotSidebarFooter,
		pluginapi.SlotMainHeader, pluginapi.SlotMainBody, pluginapi.SlotMainComposer, pluginapi.SlotShellOverlay,
	} {
		if !got[slot] {
			t.Fatalf("official 档位没有 dispatch 槽 %q（drawn=%v）", slot, sortedSlots(*drawn))
		}
	}
	// 反过来：没有旧部件时不许画兼容桥（否则会有两套结构叠着）。
	clean, cleanDrawn := newProbeKernel(t, "clean", uikernel.OffSidebar, uikernel.OffMain)
	m2 := &model{kernel: clean, cur: "s1"}
	opts2 := shellOptionsFor(m2, clean, 1182, 832)
	if opts2.BridgeLegacy {
		t.Fatal("没有任何旧槽占用方时不该打开兼容桥")
	}
	env2 := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("shell-official-test-2"), Canvas: canvas,
		Bounds: ui.Rect{Width: 1182, Height: 832},
		Has:    func(slot pluginapi.Slot) bool { return clean.Has(slot) },
	}
	var view2 ui.Page
	view2.Draw(canvas, dscore.VivaTheme(theme), env2.Bounds, func(f ui.C) {
		drawOfficialFirstScreen(f, m2, env2, opts2)
	})
	for _, slot := range *cleanDrawn {
		if slot == pluginapi.SlotSidebarList || slot == pluginapi.SlotMainBody {
			t.Fatalf("兼容桥关着却 dispatch 了旧槽 %q", slot)
		}
	}
}

func sortedSlots(slots []pluginapi.Slot) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(slots))
	for _, s := range slots {
		if seen[string(s)] {
			continue
		}
		seen[string(s)] = true
		out = append(out, string(s))
	}
	sort.Strings(out)
	return out
}

// TestFrameCaptionHeightIsSingleSourced 断言标题栏高度没有第二个魔法数：
// L0 的 frameCaptionHeight 必须就是 uikernel 几何表里的 CaptionHeight。
func TestFrameCaptionHeightIsSingleSourced(t *testing.T) {
	if frameCaptionHeight != uikernel.CaptionHeight {
		t.Fatalf("frame_caption 高度 %v ≠ 官方几何表 %v（两处魔法数会分叉）", frameCaptionHeight, uikernel.CaptionHeight)
	}
}

// TestDrawnOfficialSlotsAreRegistered 断言壳真的把内核里挂的官方槽画了出来，
// 且没有「插件登记了、壳根本不调」的槽（这一类 bug 表现为界面空白但不报错）。
func TestDrawnOfficialSlotsAreRegistered(t *testing.T) {
	kernel, drawn := newProbeKernel(t, "official-all",
		uikernel.OffSidebar, uikernel.OffConversationSession,
		uikernel.OffConvView, uikernel.OffConvComposerBar, uikernel.OffInputModel,
		uikernel.OffSessionHeaderLineage, uikernel.OffRightbarSession)
	m := &model{kernel: kernel, showRightbar: true, cur: "s1"}
	opts := shellOptionsFor(m, kernel, 1920, 1032)
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len([]rune(value))) * size / 2 })
	theme := dscore.Dark()
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("shell-official-all"), Canvas: canvas,
		Bounds: ui.Rect{Width: 1920, Height: 1032},
		Has:    func(slot pluginapi.Slot) bool { return kernel.Has(slot) },
	}
	var view ui.Page
	view.Draw(canvas, dscore.VivaTheme(theme), env.Bounds, func(f ui.C) {
		drawOfficialFirstScreen(f, m, env, opts)
	})
	got := map[pluginapi.Slot]bool{}
	for _, slot := range *drawn {
		got[slot] = true
	}
	for _, slot := range []pluginapi.Slot{
		uikernel.OffSidebar, uikernel.OffConversationSession,
		uikernel.OffConvView, uikernel.OffConvComposerBar, uikernel.OffInputModel,
		uikernel.OffSessionHeaderLineage, uikernel.OffRightbarSession,
	} {
		if !got[slot] {
			t.Fatalf("官方槽 %q 登记了却没被画出来（drawn=%v）", slot, sortedSlots(*drawn))
		}
	}
	if !reflect.DeepEqual(sortedSlots(*drawn), sortedSlots([]pluginapi.Slot{
		uikernel.OffSidebar, uikernel.OffConversationSession,
		uikernel.OffConvView, uikernel.OffConvComposerBar, uikernel.OffInputModel,
		uikernel.OffSessionHeaderLineage, uikernel.OffRightbarSession,
	})) {
		t.Logf("额外 dispatch 的槽（官方默认组合的空槽）：%v", sortedSlots(*drawn))
	}
}

// TestComposerBridgeFollowsOfficialSeat 壳的 composer 兼容桥判据：
// **官方座位有占用方 → 走官方座位**（旧槽这一帧不 dispatch）。
//
// 为什么这一条必须单独测：`BridgeLegacy` 是整列一个开关（sidebar/header/body
// 都还有旧部件），如果 composer 也跟着它，`-shell official` 下官方座位永远
// dispatch 不到——GP 报告 §5 第 1 条就是这个。
func TestComposerBridgeFollowsOfficialSeat(t *testing.T) {
	// 产品形状：dscomposer 同时注册 main.composer 与 conversation.composer。
	product, _ := newProbeKernel(t, "product",
		pluginapi.SlotSidebarList, pluginapi.SlotMainBody,
		pluginapi.SlotMainComposer, uikernel.OffConvComposer)
	opts := shellOptionsFor(&model{cur: "s1"}, product, 1182, 832)
	if !opts.BridgeLegacy {
		t.Fatal("sidebar.list / main.body 还占着槽，整列兼容桥必须开着（header/正文仍走旧槽）")
	}
	if opts.BridgeLegacyComposer {
		t.Fatal("官方 composer 座位有占用方时，composer 不该走兼容桥（官方座位就永远 dispatch 不到）")
	}

	// 纯旧插件集：官方座位没人 → composer 退回 main.composer（不黑屏）。
	legacy, _ := newProbeKernel(t, "legacy-only",
		pluginapi.SlotMainBody, pluginapi.SlotMainComposer)
	legacyOpts := shellOptionsFor(&model{cur: "s1"}, legacy, 1182, 832)
	if !legacyOpts.BridgeLegacyComposer {
		t.Fatal("官方座位没人、旧槽有人时 composer 必须走兼容桥")
	}

	// 官方座位有人、旧槽没人：同样走官方座位。
	officialOnly, _ := newProbeKernel(t, "official-only", uikernel.OffConvComposer)
	officialOpts := shellOptionsFor(&model{cur: "s1"}, officialOnly, 1182, 832)
	if officialOpts.BridgeLegacyComposer {
		t.Fatal("只有官方座位有占用方时 composer 不该走兼容桥")
	}
}

// TestOfficialTierReallyDispatchesComposerSeat `-shell official` 的产品形状：
// 中栏 header/正文走兼容桥（旧槽），**composer 走官方座位**。
//
// 断言的是「真的 dispatch 了」：座位有人时官方槽被调用一次、旧槽零次；
// 反过来（官方座位没人）旧槽一次。任何一边为 0 都是 GP 报告 §5 的那条缺口
// 又回来了。
func TestOfficialTierReallyDispatchesComposerSeat(t *testing.T) {
	counts := func(t *testing.T, slots ...pluginapi.Slot) map[pluginapi.Slot]int {
		t.Helper()
		kernel, drawn := newProbeKernel(t, "composer-tier", slots...)
		m := &model{kernel: kernel, cur: "s1"}
		opts := shellOptionsFor(m, kernel, 1182, 832)
		tree := &gfx.Tree{}
		canvas := &gfx.Canvas{}
		canvas.SetTree(tree)
		canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len([]rune(value))) * size / 2 })
		theme := dscore.Dark()
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: tree.Feature("composer-tier-test"), Canvas: canvas,
			Bounds: ui.Rect{Width: 1182, Height: 832},
			Has:    func(slot pluginapi.Slot) bool { return kernel.Has(slot) },
		}
		var view ui.Page
		view.Draw(canvas, dscore.VivaTheme(theme), env.Bounds, func(f ui.C) {
			drawOfficialFirstScreen(f, m, env, opts)
		})
		out := map[pluginapi.Slot]int{}
		for _, slot := range *drawn {
			out[slot]++
		}
		return out
	}

	// 产品形状：旧 header/正文 + 官方 composer 座位。
	product := counts(t, pluginapi.SlotSidebarList, pluginapi.SlotMainBody,
		pluginapi.SlotMainHeader, pluginapi.SlotMainComposer, uikernel.OffConvComposer)
	if product[uikernel.OffConvComposer] != 1 {
		t.Fatalf("官方 composer 座位 dispatch 次数 = %d，want 1（counts=%v）",
			product[uikernel.OffConvComposer], product)
	}
	if product[pluginapi.SlotMainComposer] != 0 {
		t.Fatalf("官方座位活着时 main.composer 也被 dispatch（双画）：%v", product)
	}
	// 兼容桥的其余部分不动：header / 正文仍走旧槽。
	for _, slot := range []pluginapi.Slot{pluginapi.SlotMainHeader, pluginapi.SlotMainBody, pluginapi.SlotSidebarList} {
		if product[slot] != 1 {
			t.Fatalf("兼容桥旧槽 %q dispatch 次数 = %d，want 1（counts=%v）", slot, product[slot], product)
		}
	}

	// 纯旧插件集：官方座位没人 → 旧槽一次。
	legacy := counts(t, pluginapi.SlotMainBody, pluginapi.SlotMainComposer)
	if legacy[pluginapi.SlotMainComposer] != 1 {
		t.Fatalf("纯旧插件集 main.composer dispatch 次数 = %d，want 1（counts=%v）",
			legacy[pluginapi.SlotMainComposer], legacy)
	}
	if legacy[uikernel.OffConvComposer] != 0 {
		t.Fatalf("官方座位没人却 dispatch 了它：%v", legacy)
	}
}
