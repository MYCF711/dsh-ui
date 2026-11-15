//go:build darwin || windows || linux

package dstopbar

// entry_menu_test.go —— 会话头 view tab 行 More 菜单宿主的真绘制 + 真点击证据。
//
// 四条结构事实（与官方 ConversationSession.tsx:119-166 的一一对应）：
//
//	① 座位没占用方 → tabs 行上**没有** More 按钮（官方 `hasEntryActions`）
//	② 名录只有一个 view → 也没有（官方 `showTabs = tabs.length > 1`）
//	③ 有占用方 + 多于一个 view → 点 More **真的**展开菜单面，并派发
//	   `conversation.session.header.views.actions`，entries = 没画成 tab 的那些
//	④ 取消固定一个 view → 它从 tab 行消失、进入菜单 entries（官方
//	   `preferred` 只收"固定 + fixed"的那些）
//
// 点击走真 Viva 无头窗口，展开态 pump 到落地再断言。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/entrypins"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

type topbarMenuFixture struct {
	t           *testing.T
	p           *Plugin
	env         *pluginapi.Env
	tree        *gfx.Tree
	feat        gfx.Scope
	session     *headless.Session
	invalidator *app.Invalidator
	dispatched  []pluginapi.Slot
	props       []pluginapi.EntryMenuProps
	draws       int
}

func newTopbarMenuFixture(t *testing.T, width float32, views []pluginapi.Tab, actionsLive bool) *topbarMenuFixture {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	f := &topbarMenuFixture{t: t, tree: tree, feat: tree.Feature("topbar-entry-menu"), p: &Plugin{}}
	f.p.theme = theme
	f.p.prims = &dscore.Primitives{Theme: theme}
	f.p.views = map[string]string{}
	if actionsLive {
		f.p.pins = entrypins.New()
	}
	f.env = &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: f.feat, Act: pluginapi.Actions{},
		Views: views, ActiveView: views[0].Key,
		Data: pluginapi.Snapshot{
			Cur:  "session-a",
			Busy: true,
			Sessions: []apiclient.Session{
				{ID: "session-a", Title: "会话 A", Cwd: `D:\work`},
			},
		},
		Has: func(slot pluginapi.Slot) bool {
			return actionsLive && slot == uikernel.OffSessionHeaderViewAction
		},
	}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{Title: "topbar-menu", Width: int(width), Height: 300, Tree: tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		f.draws++
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		f.env.Canvas, f.env.Bounds = canvas, bounds
		f.env.MainWidth = bounds.Width - 280
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(cc ui.C) {
			ui.Column(cc).WidthPercent(100).Children(func() {
				f.p.draw(cc, f.env)
				f.p.entryMenuChildren(cc, f.env, func(slot pluginapi.Slot) {
					f.dispatched = append(f.dispatched, slot)
					if f.env.EntryMenu == nil {
						return
					}
					f.props = append(f.props, *f.env.EntryMenu)
					for _, entry := range f.env.EntryMenu.Entries {
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

func (f *topbarMenuFixture) close() { f.session.Close() }

func (f *topbarMenuFixture) pump() {
	f.t.Helper()
	f.invalidator.Invalidate()
	if err := f.session.Driver().Pump(); err != nil {
		f.t.Fatal(err)
	}
}

func (f *topbarMenuFixture) hasHit(name string) bool {
	id := f.tree.ID(f.feat, name)
	for _, hit := range f.session.Snapshot().Hits {
		if hit.ID == id {
			return true
		}
	}
	return false
}

func (f *topbarMenuFixture) click(name string) {
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
		f.t.Fatalf("没有命中区 %q（More 按钮没画出来或没挂 Hit）", name)
	}
	before := f.draws
	if err := f.session.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		f.t.Fatal(err)
	}
	for i := 0; i < 8 && f.draws <= before; i++ {
		f.pump()
	}
	f.pump()
}

func (f *topbarMenuFixture) hasText(want string) bool { return f.session.HasText(want) }

// threeViews 是官方那三个 view（chat 固定 / trajectory / stats）。
func threeViews() []pluginapi.Tab {
	return []pluginapi.Tab{
		{Key: "chat", Label: "对话"},
		{Key: "trajectory", Label: "轨迹", Order: 10},
		{Key: "stats", Label: "统计", Order: 40},
	}
}

// ① 没有占用方：没有 More 按钮。
func TestTopbarEntryMenuNoContributorNoButton(t *testing.T) {
	f := newTopbarMenuFixture(t, 1420, threeViews(), false)
	defer f.close()
	if f.hasHit("views-more") {
		t.Fatal("views.actions 没有占用方却画了 More 按钮（官方 hasEntryActions 判据）")
	}
}

// ② 只有一个 view：官方连 tab 行都不出，更没有 More。
func TestTopbarEntryMenuSingleViewNoButton(t *testing.T) {
	f := newTopbarMenuFixture(t, 1420, threeViews()[:1], true)
	defer f.close()
	if f.hasHit("views-more") {
		t.Fatal("只有一个 view 却画了 More 按钮（官方 showTabs = tabs.length > 1）")
	}
}

// ③ 有占用方 + 多个 view：点 More 展开并派发座位。
func TestTopbarEntryMenuOpensAndDispatches(t *testing.T) {
	// 窄窗：可见预算 1 个，其余三个进 More 的 entries（官方宽度预算同一判据）。
	f := newTopbarMenuFixture(t, 700, threeViews(), true)
	defer f.close()
	if !f.hasHit("views-more") {
		t.Fatal("有占用方 + 多个 view 时应画出 More 按钮")
	}
	f.click("views-more")
	if !f.p.entryMenu.open {
		t.Fatal("点了 More 却没展开")
	}
	if len(f.dispatched) == 0 {
		t.Fatal("展开后没有任何座位派发：菜单面里一行都画不出来")
	}
	for _, slot := range f.dispatched {
		if slot != uikernel.OffSessionHeaderViewAction {
			t.Fatalf("派发了别的座位 %q（want conversation.session.header.views.actions）", slot)
		}
	}
	props := f.props[len(f.props)-1]
	if props.Region != pluginapi.EntryRegionConversation {
		t.Fatalf("region = %q，want conversation", props.Region)
	}
	// entries = 没画成 tab 的那些（官方 `moreTabs`）：至少包含 stats。
	found := false
	for _, entry := range props.Entries {
		if entry.ID == "stats" {
			found = true
		}
		if entry.ID == "chat" {
			t.Fatal("固定 view（chat）不该出现在 More 的 entries 里")
		}
	}
	if !found {
		t.Fatalf("没画成 tab 的 view 没进 More entries：%+v", props.Entries)
	}
	if !f.hasText("菜单:统计") {
		t.Fatal("菜单行没画出来（占用方没拿到 props）")
	}
}

// ④ 取消固定一个 view → 它从 tab 行消失、进菜单 entries。
func TestTopbarEntryMenuUnpinnedViewLeavesTabRow(t *testing.T) {
	f := newTopbarMenuFixture(t, 1420, threeViews(), true)
	defer f.close()
	// 宽窗下三个 tab 都画得下（可见预算不限），轨迹在 tab 行里。
	if !f.hasText("轨迹") {
		t.Fatal("宽窗下轨迹应在 tab 行里")
	}
	if !f.p.pins.SetPinned(pluginapi.EntryRegionConversation, "trajectory", false) {
		t.Fatal("取消固定 trajectory 应改表")
	}
	f.pump()
	f.pump()
	// 取消固定后它不再占 tab 行，但仍在名录里（只是挪进 More）。
	if !f.hasHit("views-more") {
		t.Fatal("取消固定之后 More 按钮应仍在（其余 view 还画着）")
	}
	f.click("views-more")
	entered := false
	for _, props := range f.props {
		for _, entry := range props.Entries {
			if entry.ID == "trajectory" && !entry.Pinned {
				entered = true
			}
		}
	}
	if !entered {
		t.Fatalf("取消固定的 view 没进 More entries：%+v", f.props)
	}
}

// TestViewEntryEntriesCarryActiveProjection：#17 菜单行读的 `active` 真的填了。
//
// 官方 ConversationSession.tsx:244-247 传给 More 的是
// `{id, label, pinned, active: viewTab.id === active?.id}`——`active` 是那一组四个字段
// 里的第四格，菜单据此在行尾画对勾（`EntryMenuContent.tsx:50`）。少了它，界面上
// 永远不会有"当前 view 在 More 里"的那一格提示，且不报错。
func TestViewEntryEntriesCarryActiveProjection(t *testing.T) {
	p := &Plugin{}
	all := []pluginapi.Tab{
		{Key: "chat", Label: "对话"},
		{Key: "trajectory", Label: "轨迹"},
		{Key: "stats", Label: "统计"},
	}
	visible := all[:1] // 只有 chat 画成 tab，其余两个进 More
	entries := p.viewEntryEntries(all, visible, "stats")
	if len(entries) != 2 {
		t.Fatalf("entries 应为两个折起来的 view，got %+v", entries)
	}
	for _, entry := range entries {
		if entry.ID == "stats" && !entry.Active {
			t.Fatalf("当前 view（stats）在 entries 里却没标 active：%+v", entries)
		}
		if entry.ID == "trajectory" && entry.Active {
			t.Fatalf("非当前 view（trajectory）被标成了 active：%+v", entries)
		}
	}
	// 没有当前 view（空串）时一条都不该标 active。
	for _, entry := range p.viewEntryEntries(all, visible, "") {
		if entry.Active {
			t.Fatalf("active 空串时不该有条目被标 active：%+v", entry)
		}
	}
}
