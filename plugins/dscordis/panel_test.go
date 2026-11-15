// panel_test.go —— 本包的结构、数据面闸门与**画到屏幕上**的证据。
//
// 判据（五条，全部"真内核装配 + 真 Draw + 真事件 pass"，不是只对注册表断言）：
//
//	① 两个 painter 挂在正确的官方槽位 / Order 上，都有非空 Draw
//	   （`uikernel.Load` 对"没有 Draw 的画"是 fail-loud，空 Draw 就是骗校验）；
//	② `Snapshot.Cordis == nil`（壳没接）= 触发器和面板**一个字都不画**；
//	③ `Known == false`（还没读到）= 画「读取中…」，**不许**画「还没有定义任何插件」；
//	④ 正常两行（一行归属当前会话、一行 agentId 空串）→ 两个分组标题、两个 pluginID、
//	   与派生一致的状态文案都在；再点遮罩收起；
//	⑤ 动作按钮条件：`activeRun != nil` + 包有 Client 半 → 停止 / 移除在，运行不在（HIDE 第 5 条）。
//
// 交互一律**pump 到条件成立再断言**（先点开 → 下一帧再看），不假设"点完立刻已开"。
package dscordis

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const (
	// testWindowW / H 是本仓真机默认窗口（三栏齐全）。
	testWindowW float32 = 1180
	testWindowH float32 = 800
	// testTriggerTop 把触发器摆到窗口下半部（侧栏底部动作带的位置）：
	// 顶部贴边的话"面板在触发器上方"会被窗口边界夹住，锚点就测不出来了。
	testTriggerTop  float32 = 720
	testTriggerLeft float32 = 8
	// testCurrentSession 是"当前主视图持有的会话"（官方 `current`）。
	testCurrentSession = "s1"
)

// cordisHarness 是"真内核 + 真插件 + 两槽真 dispatch + 真 AfterDraw"的测试台。
type cordisHarness struct {
	kernel  *uikernel.Kernel
	plugin  *Plugin
	tree    *gfx.Tree
	feat    gfx.Scope
	trigger pluginapi.Painter
	panel   pluginapi.Painter
	// act 是这一帧的动作面（零值 = 壳没接任何 Cordis 动作 → 三个按钮都不画）。
	act pluginapi.Actions
	// rendered 记录面板 Children 里真的 dispatch 过的子槽。
	rendered []pluginapi.Slot
}

func newCordisHarness(t *testing.T) *cordisHarness {
	t.Helper()
	kernel := uikernel.New("cordis-panel-test")
	plugin := &Plugin{}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, plugin); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	tree := &gfx.Tree{}
	h := &cordisHarness{
		kernel: kernel, plugin: plugin, tree: tree,
		feat: tree.Feature("cordis-panel-test"),
	}
	for _, painter := range plugin.Panels() {
		switch painter.ID {
		case "cordis-trigger":
			h.trigger = painter
		case "cordis-panel":
			h.panel = painter
		}
	}
	if h.trigger.ID == "" || h.panel.ID == "" {
		t.Fatalf("Panels() 缺少 painter：trigger=%q panel=%q", h.trigger.ID, h.panel.ID)
	}
	return h
}

// paint 画一帧：触发器（侧栏底部位置）+ 整窗浮层（壳的 AppFrame.overlayLayer 同形）。
//
// click 非空时按该交互 id 造一次事件（等价于用户点了它）。
func (h *cordisHarness) paint(t *testing.T, snap pluginapi.Snapshot, click string) *gfx.Canvas {
	t.Helper()
	theme := dscore.Dark()
	canvas := &gfx.Canvas{}
	canvas.SetTree(h.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: theme, Data: snap, Act: h.act, Tree: h.tree, Feat: h.feat, Canvas: canvas,
		Bounds:    gfx.Rect{Width: testWindowW, Height: testWindowH},
		MainWidth: 900,
		Geometry: pluginapi.Geometry{
			Width: testWindowW, Height: testWindowH, SidebarWidth: 280, MainWidth: 900,
		},
		Has: func(slot pluginapi.Slot) bool { return h.kernel.Has(slot) },
	}
	h.rendered = nil
	if click != "" {
		canvas.BeginEventPass(h.tree.ID(h.feat, click), gfx.ClickInfo{})
	}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		// 触发器：窗口左下（侧栏底部动作带）。
		ui.Column(f).FillWidth().FillHeight().PadSides(testTriggerTop, 0, 0, testTriggerLeft).
			Children(func() {
				ui.Column(f).Grow(1) // 占位，把触发器推到下面
				h.trigger.Draw(f, env)
			})
		// 整窗浮层：Draw（遮罩）→ Children（面板本体）。
		ui.Row(f).Absolute().Left(0).Top(0).Width(testWindowW).Height(testWindowH).
			Children(func() {
				h.panel.Draw(f, env)
				h.panel.Children(f, env, func(slot pluginapi.Slot) {
					h.rendered = append(h.rendered, slot)
					h.kernel.DrawSlot(f, slot, env)
				})
			})
	})
	if click != "" {
		canvas.EndEventPass()
	}
	// 布局已定：AfterDraw 缓存触发器的真实矩形（下一帧面板锚点）。
	if h.trigger.AfterDraw != nil {
		h.trigger.AfterDraw(canvas, env)
	}
	return canvas
}

// pumpClick 先按 click 点一次，再逐帧重画直到 want 成立（最多 4 帧）。
//
// 为什么不能"点完立刻断言"：点击在 Draw 里写的是插件状态，面板本体由**下一个**
// 布局帧或同一帧的 Children 画出来；把"点开"和"已经画出来"当成同一件事，
// 测试就会在真的能用的实现上假红、在画歪了的实现上假绿。
func (h *cordisHarness) pumpClick(t *testing.T, snap pluginapi.Snapshot, click string,
	want func(*gfx.Canvas) bool) *gfx.Canvas {
	t.Helper()
	var canvas *gfx.Canvas
	for i := 0; i < 4; i++ {
		id := click
		if i > 0 {
			id = "" // 只点一次：每帧都点会把开合来回翻
		}
		canvas = h.paint(t, snap, id)
		if want(canvas) {
			return canvas
		}
	}
	return canvas
}

// textValues 取这一帧所有文本。
func textValues(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

// hasText 报告这一帧画没画某段文本（子串匹配，官方文案带占位符时也能用）。
func hasText(canvas *gfx.Canvas, want string) bool {
	for _, text := range canvas.Texts() {
		if strings.Contains(text.Value, want) {
			return true
		}
	}
	return false
}

// hitLabels 取这一帧所有命中区的可访问名（动作按钮的可读文案就在这里面）。
func hitLabels(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Hits()))
	for _, hit := range canvas.Hits() {
		out = append(out, hit.Label)
	}
	return out
}

// hasHitLabel 报告这一帧有没有这个可访问名的命中区。
func hasHitLabel(canvas *gfx.Canvas, want string) bool {
	for _, label := range hitLabels(canvas) {
		if label == want {
			return true
		}
	}
	return false
}

// cordisRow 造一条清单行（字段名与 `apiclient.CordisRow` 一致）。
func cordisRow(pluginID, agentID string, packages []apiclient.CordisPackage) apiclient.CordisRow {
	return apiclient.CordisRow{PluginID: pluginID, AgentID: agentID, Packages: packages}
}

// emptySnapshot 是"壳没接 Cordis"的基线快照。
func emptySnapshot() pluginapi.Snapshot {
	return pluginapi.Snapshot{Cur: testCurrentSession}
}

// cordisSnapshot 造一份"读到过清单"的快照。
func cordisSnapshot(rows ...apiclient.CordisRow) pluginapi.Snapshot {
	snap := emptySnapshot()
	snap.Cordis = &pluginapi.CordisView{Known: true, Rows: rows}
	return snap
}

// TestPanelsSeatOfficialSlots 判据①：两个 painter 各就各位，且每个都有真实 Draw。
//
// 同时钉住 `init()` 的登记还在（`panels_order`/装配门禁都靠 `pluginapi.Builtin()`）。
func TestPanelsSeatOfficialSlots(t *testing.T) {
	h := newCordisHarness(t)

	seats := []struct {
		painter pluginapi.Painter
		id      string
		slot    pluginapi.Slot
		order   int
		child   bool
	}{
		// 官方 `sidebar.footer.action`（OFF:index.ts:86-114 注册，:485-504 画触发器）。
		{h.trigger, "cordis-trigger", uikernel.OffSidebarFooterActions, 60, false},
		// 官方 `createPortal(<section className={css.panel}/>)` 的等价层（:459-484）。
		{h.panel, "cordis-panel", pluginapi.SlotShellOverlay, 46, true},
	}
	for _, want := range seats {
		if want.painter.ID != want.id {
			t.Fatalf("painter id = %q，want %q", want.painter.ID, want.id)
		}
		if want.painter.Slot != want.slot {
			t.Fatalf("%q 应挂在 %q，实际 %q", want.id, want.slot, want.painter.Slot)
		}
		if want.painter.Order != want.order {
			t.Fatalf("%q 的 Order = %d，want %d", want.id, want.painter.Order, want.order)
		}
		if want.painter.Draw == nil {
			t.Fatalf("%q 没有 Draw：内核装配会 fail-loud", want.id)
		}
		if want.child && want.painter.Children == nil {
			t.Fatalf("%q 必须有 Children（面板本体靠它画）", want.id)
		}
	}

	// 官方槽本身必须是**已经登记过的官方槽**（不是本包自己造的名字）。
	for _, slot := range []pluginapi.Slot{uikernel.OffSidebarFooterActions, pluginapi.SlotShellOverlay} {
		if _, ok := uikernel.LookupOfficialSlot(slot); !ok {
			t.Fatalf("官方槽表里没有 %q", slot)
		}
	}

	// `init()` 的登记还在：`pluginapi.Builtin()` 里必须有 ds-cordis，且带这两块画。
	plugins, err := pluginapi.Builtin().Plugins()
	if err != nil {
		t.Fatalf("枚举内置插件失败：%v", err)
	}
	found := false
	for _, p := range plugins {
		if p.ID() != "ds-cordis" {
			continue
		}
		found = true
		withPanels, ok := p.(pluginapi.Panels)
		if !ok {
			t.Fatal("ds-cordis 没实现 Panels")
		}
		if len(withPanels.Panels()) != 2 {
			t.Fatalf("ds-cordis 应有 2 块画，实际 %d", len(withPanels.Panels()))
		}
	}
	if !found {
		t.Fatalf("内置插件里没有 ds-cordis（register.go 的 init 没生效？）：%v", pluginapi.Registered())
	}
}

// TestNoCordisDataDrawsNothing 判据②：`Snapshot.Cordis == nil` = 壳没接 → 整块不画。
//
// 这里连**面板被壳直接打开**的情况一起钉：没有数据面时，面板不该画成
// 「还没有定义任何插件」——那是没有证据的断言（`pluginapi/plugin.go` 的 Cordis 注释）。
func TestNoCordisDataDrawsNothing(t *testing.T) {
	h := newCordisHarness(t)
	h.plugin.Open()

	canvas := h.paint(t, emptySnapshot(), "")
	if texts := textValues(canvas); len(texts) != 0 {
		t.Fatalf("没有 Cordis 数据面时一个文本都不该画，实际画了 %v", texts)
	}
	// 连命中区也不该有：触发器不在，就没有可点的东西。
	if labels := hitLabels(canvas); len(labels) != 0 {
		t.Fatalf("没有 Cordis 数据面时不该有命中区，实际 %v", labels)
	}
}

// TestLoadingIsNotClaimedAsEmpty 判据③：`Known == false` 画「读取中…」，**不许**画「还没有定义任何插件」。
func TestLoadingIsNotClaimedAsEmpty(t *testing.T) {
	h := newCordisHarness(t)
	// 读取中的清单里没有行 → 触发器按官方 `if (all.length === 0) return null` 不画，
	// 所以这里用壳的无鼠标入口直接打开面板（`Plugin.Open`，与点一次触发器等价）。
	h.plugin.Open()

	snap := emptySnapshot()
	snap.Cordis = &pluginapi.CordisView{Known: false}
	canvas := h.pumpClick(t, snap, "", func(c *gfx.Canvas) bool {
		return hasText(c, zhPanelLoading)
	})

	if !hasText(canvas, zhPanelTitle) {
		t.Fatalf("面板标题没画出来：%v", textValues(canvas))
	}
	if !hasText(canvas, zhPanelLoading) {
		t.Fatalf("读取中必须画 %q，实际画了 %v", zhPanelLoading, textValues(canvas))
	}
	if hasText(canvas, zhPanelEmpty) {
		t.Fatalf("读到之前不许说 %q（把不知道说成没有）：%v", zhPanelEmpty, textValues(canvas))
	}
	// 触发器此时**不该**出现（官方 :192，0 行整块不画）。
	if hasText(canvas, zhPanelTrigger) {
		t.Fatalf("清单还没有行时不该画触发器：%v", textValues(canvas))
	}
}

// TestTwoGroupsAndStatusMatchDerivation 判据④：两个分组 + 状态文案与派生一致 + 点遮罩收起。
func TestTwoGroupsAndStatusMatchDerivation(t *testing.T) {
	mine := cordisRow("cordis-alpha", testCurrentSession, []apiclient.CordisPackage{
		{PackageID: "pkg-a1", Name: "Alpha 包", Purpose: "给 alpha 用的", HasHostHalf: true},
	})
	theirs := cordisRow("cordis-beta", "", []apiclient.CordisPackage{
		{PackageID: "pkg-b1", Name: "Beta 包", HasHostHalf: true},
	})
	// beta 在跑：它的 Host 半已经起来了（`activeRun` 有值、`currentPackageId` 已结算）。
	theirs.CurrentPackageID = "pkg-b1"
	theirs.ActiveRun = &apiclient.CordisRunRef{PluginRunID: "run-b1", PackageID: "pkg-b1"}
	snap := cordisSnapshot(mine, theirs)

	// 派生先自己算一遍：界面上的文案必须与它逐条一致（不是各画各的）。
	loaded := loadedClientHalves()
	if got, want := panelStatusLabel(visiblePanelStatus(mine, selectedPackageIDOf(mine), loaded)), zhStatusIdle; got != want {
		t.Fatalf("alpha 的派生状态 = %q，want %q", got, want)
	}
	if got, want := panelStatusLabel(visiblePanelStatus(theirs, selectedPackageIDOf(theirs), loaded)), zhStatusRunning; got != want {
		t.Fatalf("beta 的派生状态 = %q，want %q", got, want)
	}

	h := newCordisHarness(t)
	// 先点触发器打开面板（pump 到面板真的画出来）。
	opened := h.pumpClick(t, snap, hitTrigger, func(c *gfx.Canvas) bool {
		return hasText(c, zhPanelTitle)
	})
	for _, want := range []string{
		zhPanelTitle,
		zhPanelGroupCurrent, zhPanelGroupOthers, // 两个分组标题
		"cordis-alpha", "cordis-beta", // 两个 pluginID
		"Alpha 包", "Beta 包", // 选中 Package 的 name
		"给 alpha 用的",    // rowPurpose
		zhStatusIdle,    // alpha：没有 activeRun → 待激活
		zhStatusRunning, // beta：Host 半在跑且包没有 Client 半 → 运行中
	} {
		if !hasText(opened, want) {
			t.Fatalf("打开面板后应画 %q，实际 %v", want, textValues(opened))
		}
	}
	// 空串 agentId 归「其他会话」（官方判据：`agentId !== current`），不是"当前会话"。
	if hasText(opened, zhStatusClientPending) {
		t.Fatalf("这一帧没有任何 Client 半在跑的包，不该出现 %q", zhStatusClientPending)
	}
	// 触发器这一帧同时还在（面板不取代触发器，官方 :485-504 与 :459-484 同组件）。
	if !hasText(opened, zhPanelTrigger) {
		t.Fatalf("触发器应与面板同帧存在，实际 %v", textValues(opened))
	}

	// 再点一次遮罩收起（官方 `useDismissOnOutsidePointer`）。
	closed := h.pumpClick(t, snap, hitBackdrop, func(c *gfx.Canvas) bool {
		return !hasText(c, zhPanelTitle)
	})
	if hasText(closed, zhPanelTitle) {
		t.Fatalf("点遮罩后应收起面板，仍然画着 %q", zhPanelTitle)
	}
	if !hasText(closed, zhPanelTrigger) {
		t.Fatalf("收起面板后触发器必须还在，实际 %v", textValues(closed))
	}
	if h.plugin.isOpen() {
		// 状态也必须真的翻了（不是只在画布上消失）。
		t.Fatal("点遮罩后插件状态仍是展开")
	}
}

// TestActionButtonsForClientHalfPackage 判据⑤：HIDE 第 5 条 ——
// `activeRun != nil` + 包有 Client 半 → 停止 / 移除在，运行**不在**。
func TestActionButtonsForClientHalfPackage(t *testing.T) {
	row := cordisRow("cordis-client", testCurrentSession, []apiclient.CordisPackage{
		{PackageID: "pkg-c1", Name: "Client 包", HasClientHalf: true},
	})
	row.CurrentPackageID = "pkg-c1"
	row.ActiveRun = &apiclient.CordisRunRef{PluginRunID: "run-c1", PackageID: "pkg-c1"}
	snap := cordisSnapshot(row)

	// 动作面全接（nil 时按钮按契约本来就不画，那就测不出条件了）。
	h := newCordisHarness(t)
	h.act = pluginapi.Actions{
		CordisRun:    func(string, string, string, string) error { return nil },
		CordisStop:   func(string, string) error { return nil },
		CordisRemove: func(string, string) error { return nil },
	}
	opened := h.pumpClick(t, snap, hitTrigger, func(c *gfx.Canvas) bool {
		return hasText(c, zhPanelTitle)
	})

	// 有 Client 半 → Host 半起来了也只是 client-pending（不许写成 running）。
	if !hasText(opened, zhStatusClientPending) {
		t.Fatalf("有 Client 半的包在跑必须显示 %q，实际 %v", zhStatusClientPending, textValues(opened))
	}
	if hasText(opened, zhStatusRunning) {
		t.Fatalf("有 Client 半的包在跑不许显示 %q（把 client-pending 写成 running）", zhStatusRunning)
	}
	if !hasHitLabel(opened, zhActionStop) {
		t.Fatalf("有 activeRun 就该有停止按钮，实际 %v", hitLabels(opened))
	}
	if !hasHitLabel(opened, zhActionRemove) {
		t.Fatalf("行存在就该有移除按钮，实际 %v", hitLabels(opened))
	}
	if hasHitLabel(opened, zhActionRun) {
		t.Fatalf("有 Client 半的包不该有运行按钮（HIDE 第 5 条：起了就永远停在 client-pending），实际 %v",
			hitLabels(opened))
	}

	// 对照组：没有 Client 半、也没有 activeRun → 运行按钮在，停止不在。
	hostOnly := cordisSnapshot(cordisRow("cordis-host", testCurrentSession, []apiclient.CordisPackage{
		{PackageID: "pkg-h1", Name: "Host 包", HasHostHalf: true},
	}))
	other := newCordisHarness(t)
	other.act = h.act
	hostOpened := other.pumpClick(t, hostOnly, hitTrigger, func(c *gfx.Canvas) bool {
		return hasText(c, zhPanelTitle)
	})
	if !hasHitLabel(hostOpened, zhActionRun) {
		t.Fatalf("没有 Client 半的包应该给运行按钮，实际 %v", hitLabels(hostOpened))
	}
	if hasHitLabel(hostOpened, zhActionStop) {
		t.Fatalf("没有 activeRun 时不该有停止按钮，实际 %v", hitLabels(hostOpened))
	}
	if !hasHitLabel(hostOpened, zhActionRemove) {
		t.Fatalf("行存在就该有移除按钮，实际 %v", hitLabels(hostOpened))
	}
}

// TestNoActionButtonsWhenActionsMissing 是判据⑤的反面：壳没接动作面 = 三个按钮都不画
// （死按钮比没有更坏，`pluginapi/plugin.go` 的 `CordisRun` 字段注释）。
func TestNoActionButtonsWhenActionsMissing(t *testing.T) {
	row := cordisRow("cordis-host", testCurrentSession, []apiclient.CordisPackage{
		{PackageID: "pkg-h1", Name: "Host 包"},
	})
	row.CurrentPackageID = "pkg-h1"
	row.ActiveRun = &apiclient.CordisRunRef{PluginRunID: "run-h1", PackageID: "pkg-h1"}
	h := newCordisHarness(t) // act 是零值：一条动作都没接
	opened := h.pumpClick(t, cordisSnapshot(row), hitTrigger, func(c *gfx.Canvas) bool {
		return hasText(c, zhPanelTitle)
	})
	for _, label := range []string{zhActionRun, zhActionStop, zhActionRemove} {
		if hasHitLabel(opened, label) {
			t.Fatalf("动作面缺席时不该画 %q 按钮，实际 %v", label, hitLabels(opened))
		}
	}
}

// TestRowErrorShowsHostWords 钉住「失败要说宿主原文」这条：
// `latestRun.status == "failed"` → `message (phase)`，与官方 “ `${message} (${phase})` “ 同形。
func TestRowErrorShowsHostWords(t *testing.T) {
	row := cordisRow("cordis-broken", testCurrentSession, []apiclient.CordisPackage{
		{PackageID: "pkg-x", Name: "坏包"},
	})
	row.LatestRun = &apiclient.CordisRunAttempt{
		PluginRunID: "run-x", PackageID: "pkg-x", Mode: apiclient.CordisRunModeRun,
		Status: cordisRunStatusFailed,
		Error: &apiclient.CordisDiagnostic{
			Phase: "host-apply", Message: "插件抛了异常", PluginID: "cordis-broken", PackageID: "pkg-x",
		},
	}
	h := newCordisHarness(t)
	opened := h.pumpClick(t, cordisSnapshot(row), hitTrigger, func(c *gfx.Canvas) bool {
		return hasText(c, zhPanelTitle)
	})
	if !hasText(opened, "插件抛了异常 (host-apply)") {
		t.Fatalf("行错误应画宿主原文 + (phase)，实际 %v", textValues(opened))
	}
	// 同一条失败也会让状态落到 failed（官方 :72 的判据：failed 且 packageId == 选中包）。
	if !hasText(opened, zhStatusFailed) {
		t.Fatalf("该行状态应为 %q，实际 %v", zhStatusFailed, textValues(opened))
	}
}

// TestReadErrorKeepsRowsAndShowsHostWords 钉住官方那句最容易被"优化掉"的语义：
// **读失败时保留旧行并说明原因**，不把一次传输抖动说成"什么都没有"。
func TestReadErrorKeepsRowsAndShowsHostWords(t *testing.T) {
	rows := []apiclient.CordisRow{cordisRow("cordis-alpha", testCurrentSession, []apiclient.CordisPackage{
		{PackageID: "pkg-a1", Name: "Alpha 包"},
	})}
	snap := emptySnapshot()
	snap.Cordis = &pluginapi.CordisView{
		Known: true, Rows: rows, Error: "dynamicCordisRunner.inventory: 连接被重置",
	}
	h := newCordisHarness(t)
	opened := h.pumpClick(t, snap, hitTrigger, func(c *gfx.Canvas) bool {
		return hasText(c, zhPanelTitle)
	})
	if !hasText(opened, "读取插件清单失败：dynamicCordisRunner.inventory: 连接被重置") {
		t.Fatalf("读失败应把宿主原文带出来，实际 %v", textValues(opened))
	}
	// 旧行必须还在（`Error` 非空时不清空 Rows）。
	if !hasText(opened, "cordis-alpha") {
		t.Fatalf("读失败时旧行不许被清掉，实际 %v", textValues(opened))
	}
	if hasText(opened, zhPanelEmpty) {
		t.Fatalf("读失败不许画 %q", zhPanelEmpty)
	}
}
