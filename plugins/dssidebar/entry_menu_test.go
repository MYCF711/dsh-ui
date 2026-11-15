//go:build darwin || windows || linux

package dssidebar

// entry_menu_test.go —— 左栏 More 菜单宿主的**真绘制 + 真点击**证据（#17 前置 A/B）。
//
// 判据不是"画面里有东西"，而是四条结构事实：
//
//	① 座位没占用方 → 列表里**没有**「更多」行（官方 `hasEntryActions` 同一判据，
//	   不自造一个点了没反应的入口）
//	② 座位有占用方 → 「更多」行真的排在面板行之后，点它**真的**展开菜单面，
//	   并把 `sidebar.panellist.actions` 派发进去（占用方拿到 owner props 并画行）
//	③ 固定态：取消固定一个面板 → 它从列表里消失、出现在菜单的 entries 里
//	   （官方 `panels.filter(... !unpinned.includes(id))`）
//	④ 行尾取消固定按钮**真的**能点：点了之后该面板回到"未固定"
//
// 点击走真 Viva 无头窗口（`headless.Session.Click` → 真命中区 → 真事件 pass），
// 展开态与可见性一律 **pump 到条件成立再断言**，不在点完的单帧上读异步结果。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/entrypins"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

type entryMenuFixture struct {
	t           *testing.T
	p           *Plugin
	env         *pluginapi.Env
	tree        *gfx.Tree
	feat        gfx.Scope
	theme       *pluginapi.Theme
	session     *headless.Session
	invalidator *app.Invalidator
	actionsLive bool
	// dispatched / props 记录菜单面里派发过的槽与 owner props。
	dispatched []pluginapi.Slot
	props      []pluginapi.EntryMenuProps
	// draws 是本帧序号（给"泵到条件成立"用）。
	draws int
}

func newEntryMenuFixture(t *testing.T, actionsLive bool, pins pluginapi.EntryPins) *entryMenuFixture {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	f := &entryMenuFixture{
		t: t, tree: tree, feat: tree.Feature("entry-menu-test"), theme: theme,
		actionsLive: actionsLive,
	}
	f.p = &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}, width: 280,
		collapsed: map[string]bool{}, pins: pins}
	f.env = &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: f.feat,
		Act: pluginapi.Actions{},
		Has: func(slot pluginapi.Slot) bool {
			return actionsLive && slot == uikernel.OffSidebarPanelActions
		},
	}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{Title: "entry-menu", Width: 268, Height: 600, Tree: tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		f.draws++
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		f.env.Canvas, f.env.Bounds = canvas, bounds
		f.env.MainWidth = bounds.Width
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(cc ui.C) {
			ui.Column(cc).Width(268).FillHeight().Children(func() {
				f.p.drawHeader(cc, f.env)
				f.p.entryMenuChildren(cc, f.env, func(slot pluginapi.Slot) {
					f.dispatched = append(f.dispatched, slot)
					if f.env.EntryMenu == nil {
						return
					}
					f.props = append(f.props, *f.env.EntryMenu)
					// 占用方（#17 的等价物）在这里画菜单行：本测试只看"行到没到"。
					for _, entry := range f.env.EntryMenu.Entries {
						if entry.Pinned {
							continue
						}
						ui.Text(cc, "菜单:"+entry.Label).FontSize(13).LineHeight(20)
					}
				})
			})
		})
		f.p.afterEntryMenu(canvas, f.env)
	}
	config.OnKey = func(ev gfx.KeyEvent) gfx.EventResult { return gfx.EventResult{} }
	f.invalidator = &app.Invalidator{}
	config.Invalidator = f.invalidator
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	f.session = session
	f.pump()
	f.pump()
	return f
}

func (f *entryMenuFixture) close() { f.session.Close() }

// pump 重绘一帧（与 settings 视觉测试同一先例）。
func (f *entryMenuFixture) pump() {
	f.t.Helper()
	f.invalidator.Invalidate()
	if err := f.session.Driver().Pump(); err != nil {
		f.t.Fatal(err)
	}
}

// click 用真无头窗口点一个命中区，然后 **泵到这一击落地**再返回。
func (f *entryMenuFixture) click(name string) {
	f.t.Helper()
	id := f.tree.ID(f.feat, name)
	var hit gfx.Hit
	found := false
	for _, candidate := range f.session.Snapshot().Hits {
		if candidate.ID == id {
			hit, found = candidate, true
			break
		}
	}
	if !found {
		f.t.Fatalf("没有命中区 %q（入口没画出来或没挂 Hit）", name)
	}
	before := f.draws
	if err := f.session.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		f.t.Fatal(err)
	}
	// 点击本身会触发一次重绘；再泵到"画过至少一帧"为止（不在事件当帧读状态）。
	for i := 0; i < 8 && f.draws <= before; i++ {
		f.pump()
	}
	f.pump()
}

func (f *entryMenuFixture) hasText(want string) bool { return f.session.HasText(want) }

// TestEntryMenuMoreRowOnlyWithContributor：没有占用方就没有「更多」行。
func TestEntryMenuMoreRowOnlyWithContributor(t *testing.T) {
	off := newEntryMenuFixture(t, false, entrypins.New())
	defer off.close()
	if off.hasText(zhPanelsMore) {
		t.Fatal("sidebar.panellist.actions 没有占用方却画了「更多」行（官方 hasEntryActions 判据）")
	}
	unpinID := off.tree.ID(off.feat, "panel-unpin-schedules")
	for _, candidate := range off.session.Snapshot().Hits {
		if candidate.ID == unpinID {
			t.Fatal("没有菜单座位却画了取消固定按钮：点了没有任何动作")
		}
	}

	on := newEntryMenuFixture(t, true, entrypins.New())
	defer on.close()
	if !on.hasText(zhPanelsMore) {
		t.Fatalf("有占用方时应画出官方 `panels.more`（「更多」）")
	}
}

// TestEntryMenuMoreRowOpensMenuAndDispatches：点「更多」→ 菜单面 + 座位派发。
func TestEntryMenuMoreRowOpensMenuAndDispatches(t *testing.T) {
	pins := entrypins.New()
	f := newEntryMenuFixture(t, true, pins)
	defer f.close()
	if len(f.dispatched) != 0 {
		t.Fatalf("菜单还没展开就派发了座位：%v", f.dispatched)
	}
	// 先取消固定一条：菜单内容只画**未固定**的条目（官方
	// `moreEntries = entries.filter(entry => !entry.pinned)`，EntryMenuContent.tsx:36）。
	if !pins.SetPinned(pluginapi.EntryRegionSidebar, "schedules", false) {
		t.Fatal("取消固定 schedules 应改表")
	}
	f.pump()
	f.pump()
	f.click("panel-more")
	if !f.p.entryMenu.open {
		t.Fatal("点了「更多」行却没展开")
	}
	// 菜单开着 → 每帧都会派发一次（菜单面是每帧重画的），但**只派发这一个座位**。
	if len(f.dispatched) == 0 {
		t.Fatal("展开后没有任何座位派发：菜单面里一行都画不出来")
	}
	for _, slot := range f.dispatched {
		if slot != uikernel.OffSidebarPanelActions {
			t.Fatalf("展开后派发了别的座位 %q（want 只派发 sidebar.panellist.actions）", slot)
		}
	}
	if len(f.props) == 0 {
		t.Fatal("owner props 一次都没送到占用方")
	}
	props := f.props[0]
	if props.Region != pluginapi.EntryRegionSidebar {
		t.Fatalf("region = %q，want sidebar", props.Region)
	}
	// entries 是**除固定行以外**的全部面板，带 pinned 标记（官方
	// SidebarRoot.tsx:419-421；固定行 `plugins` 不进这条列表）。
	if len(props.Entries) != 1 || props.Entries[0].ID != "schedules" || props.Entries[0].Pinned {
		t.Fatalf("entries 不是除固定行以外的全部面板：%+v", props.Entries)
	}
	// 行首图标：官方那一格是 `renderFactorySlot('sidebar.panel.icon')`，本仓把面板行
	// 同一个字形投影进 props（#17 的菜单行要用它，`EntryMenuEntry.Glyph`）。
	if props.Entries[0].Glyph == "" {
		t.Fatalf("entries 没带面板字形：菜单行会没有图标（官方 EntryMenuContent.tsx:43-45）")
	}
	if !f.hasText("菜单:自动化任务") {
		t.Fatal("菜单行没画出来（占用方没拿到 props）")
	}
}

// TestEntryMenuUnpinnedPanelLeavesListAndReappearsInMenu：固定态驱动可见性。
func TestEntryMenuUnpinnedPanelLeavesListAndReappearsInMenu(t *testing.T) {
	pins := entrypins.New()
	f := newEntryMenuFixture(t, true, pins)
	defer f.close()
	if !f.hasText("自动化任务") {
		t.Fatal("默认态下面板行应在列表里")
	}
	// 取消固定（等价于菜单里那条「固定」按钮被点）。
	if !pins.SetPinned(pluginapi.EntryRegionSidebar, "schedules", false) {
		t.Fatal("取消固定 schedules 应改表")
	}
	f.pump()
	f.pump()
	if f.hasText("自动化任务") {
		t.Fatal("未固定的面板还留在列表里：官方 `!unpinned.includes(id)` 判据没生效")
	}
	// 菜单里必须还能看到它（否则用户没有任何路径把它固定回来）。
	f.click("panel-more")
	if !f.hasText("菜单:自动化任务") {
		t.Fatal("未固定的面板既不在列表里也不在菜单里（丢了）")
	}
}

// TestEntryMenuUnpinButtonRestoresPinnedPanel：行尾取消固定按钮真的能点。
func TestEntryMenuUnpinButtonRestoresPinnedPanel(t *testing.T) {
	pins := entrypins.New()
	f := newEntryMenuFixture(t, true, pins)
	defer f.close()
	f.click("panel-unpin-schedules")
	if pins.Pinned(pluginapi.EntryRegionSidebar, "schedules") {
		t.Fatal("点了行尾取消固定按钮，schedules 仍是固定态")
	}
	if f.hasText("自动化任务") {
		t.Fatal("取消固定之后列表里还画着那一行（可见性没跟着固定态走）")
	}
}

// TestEntryMenuDefaultPinServiceKeepsEveryRow：没有固定态服务时按"全部固定"。
func TestEntryMenuDefaultPinServiceKeepsEveryRow(t *testing.T) {
	f := newEntryMenuFixture(t, true, nil)
	defer f.close()
	if !f.hasText("自动化任务") || !f.hasText("插件") {
		t.Fatal("没有固定态服务时面板行应全部可见")
	}
	unpinID := f.tree.ID(f.feat, "panel-unpin-schedules")
	for _, candidate := range f.session.Snapshot().Hits {
		if candidate.ID == unpinID {
			t.Fatal("没有固定态服务却画了取消固定按钮：点了不会改任何状态")
		}
	}
}
