//go:build darwin || windows || linux

package dsentrycustom

// entry_menu_test.go —— #17 内容行的**真绘制 + 真点击**证据。
//
// 判据不是"画面里有文字"，而是逐条对官方那一份 `EntryMenuContent` 的结构事实：
//
//	① `Env.EntryMenu == nil`（不在菜单里）→ 一行都不画
//	② 左栏只画**未固定**条目（`entries.filter(e => !e.pinned)`），且行首带面板图标、
//	   行尾带固定按钮；点了行 = onDismiss + onOpen，点了固定按钮 = onDismiss + onPinnedChange(id,true)
//	③ 会话头画**全部**折起来的 view，没有固定按钮；`active` 那条行尾多一枚对勾
//	④ 「浏览更多插件」在面板导航在册时画（官方 `browsable`），点了 = onDismiss + openList
//	⑤ 「创建你的插件」**不画**（HIDE：宿主 `session.create` 不收 agentPreset）
//
// 点击走真 Viva 无头窗口（`headless.Session.Click` → 真命中区 → 真事件 pass），
// 与 `plugins/dssidebar/entry_menu_test.go` 同一套夹具写法；读状态一律"泵到条件成立"。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar/panelapi"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// fakeNavigation 是官方 `pluginNavigation` 的替身：只记"列表被打开了"。
type fakeNavigation struct{ opened int }

func (f *fakeNavigation) OpenList() { f.opened++ }

// entryRowCalls 记录本 fixture 收到的 owner props 回调。
type entryRowCalls struct {
	opened    []string
	pinned    []string
	dismissed int
}

type entryFixture struct {
	t           *testing.T
	p           *Plugin
	env         *pluginapi.Env
	tree        *gfx.Tree
	feat        gfx.Scope
	theme       *pluginapi.Theme
	session     *headless.Session
	invalidator *app.Invalidator
	props       pluginapi.EntryMenuProps
	calls       entryRowCalls
	region      string
	draws       int
}

func newEntryFixture(t *testing.T, region string, entries []pluginapi.EntryMenuEntry, nav panelapi.Navigation) *entryFixture {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	f := &entryFixture{t: t, tree: tree, feat: tree.Feature("entry-customization-test"), theme: theme, region: region}
	f.p = &Plugin{theme: theme, nav: nav}
	f.props = pluginapi.EntryMenuProps{
		Region:  region,
		Entries: entries,
		Open:    func(id string) { f.calls.opened = append(f.calls.opened, id) },
		SetPinned: func(id string, pinned bool) {
			if pinned {
				f.calls.pinned = append(f.calls.pinned, id)
			}
		},
		Dismiss: func() { f.calls.dismissed++ },
	}
	f.env = &pluginapi.Env{Theme: theme, Tree: tree, Feat: f.feat, Act: pluginapi.Actions{}}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{Title: "entry-customization", Width: 288, Height: 400, Tree: tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		f.draws++
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		f.env.Canvas, f.env.Bounds = canvas, bounds
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(cc ui.C) {
			// 宿主派发座位时的形状：菜单面里挂上 owner props，派发完恢复。
			f.env.EntryMenu = &f.props
			ui.Column(cc).Width(288).FillHeight().Pad(4).Gap(0).Children(func() {
				if region == pluginapi.EntryRegionSidebar {
					f.p.drawSidebarRows(cc, f.env)
				} else {
					f.p.drawConversationRows(cc, f.env)
				}
			})
			f.env.EntryMenu = nil
		})
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

func (f *entryFixture) close() { f.session.Close() }

func (f *entryFixture) pump() {
	f.t.Helper()
	f.invalidator.Invalidate()
	if err := f.session.Driver().Pump(); err != nil {
		f.t.Fatal(err)
	}
}

func (f *entryFixture) hit(name string) (gfx.Hit, bool) {
	id := f.tree.ID(f.feat, name)
	for _, candidate := range f.session.Snapshot().Hits {
		if candidate.ID == id {
			return candidate, true
		}
	}
	return gfx.Hit{}, false
}

func (f *entryFixture) mustHit(name string) gfx.Hit {
	f.t.Helper()
	h, ok := f.hit(name)
	if !ok {
		f.t.Fatalf("没有命中区 %q（行没画出来或没挂 Hit）", name)
	}
	return h
}

func (f *entryFixture) click(name string) {
	f.t.Helper()
	h := f.mustHit(name)
	before := f.draws
	if err := f.session.Click(h.Bounds.X+h.Bounds.Width/2, h.Bounds.Y+h.Bounds.Height/2); err != nil {
		f.t.Fatal(err)
	}
	for i := 0; i < 8 && f.draws <= before; i++ {
		f.pump()
	}
	f.pump()
}

// TestEntryRowsHiddenOutsideMenu：不在菜单里（宿主没挂 props）就一行都不画。
func TestEntryRowsHiddenOutsideMenu(t *testing.T) {
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("entry-customization-off")
	p := &Plugin{theme: theme, nav: &fakeNavigation{}}
	env := &pluginapi.Env{Theme: theme, Tree: tree, Feat: feat}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{Title: "entry-off", Width: 288, Height: 200, Tree: tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env.Canvas, env.Bounds = canvas, bounds
		view.Draw(canvas, bounds, func(cc ui.C) {
			p.drawSidebarRows(cc, env)
			p.drawConversationRows(cc, env)
		})
	}
	config.OnKey = func(ev gfx.KeyEvent) gfx.EventResult { return gfx.EventResult{} }
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{zhInstall, zhCreate} {
		if session.HasText(text) {
			t.Fatalf("Env.EntryMenu 是 nil（本帧不在菜单里）却画了 %q", text)
		}
	}
	if len(session.Snapshot().Hits) != 0 {
		t.Fatalf("不在菜单里却挂了 %d 个命中区", len(session.Snapshot().Hits))
	}
}

func sidebarEntries() []pluginapi.EntryMenuEntry {
	return []pluginapi.EntryMenuEntry{
		{ID: "plugins", Label: "面板甲", Glyph: "▦", Pinned: true, Active: false},
		{ID: "schedules", Label: "自动化任务", Glyph: "◷", Pinned: false, Active: true},
		{ID: "timeline", Label: "轨迹", Glyph: "≡", Pinned: false, Active: false},
	}
}

// TestSidebarRowsListOnlyUnpinnedEntries：左栏只列未固定条目，行首图标 + 行尾固定按钮。
func TestSidebarRowsListOnlyUnpinnedEntries(t *testing.T) {
	nav := &fakeNavigation{}
	f := newEntryFixture(t, pluginapi.EntryRegionSidebar, sidebarEntries(), nav)
	defer f.close()

	if !f.session.HasText("自动化任务") || !f.session.HasText("轨迹") {
		t.Fatal("未固定的条目没有进 More 菜单（官方 moreEntries = entries.filter(!pinned)）")
	}
	if f.session.HasText("面板甲") {
		t.Fatal("已固定的条目也进了 More 菜单：官方把它留在列表里，不进菜单")
	}
	// 行首面板图标：官方是 `renderFactorySlot('sidebar.panel.icon', {size:16})`。
	if !f.session.HasText("◷") {
		t.Fatal("左栏菜单行的面板图标没画（官方每行行首一枚 16px 面板图标）")
	}
	// 行尾固定按钮：只有未固定条目有。
	f.mustHit("entry-pin-schedules")
	if _, ok := f.hit("entry-pin-plugins"); ok {
		t.Fatal("已固定（进不了菜单）的条目居然挂了固定按钮")
	}
}

// TestSidebarRowClickOpensEntry：点行 = onDismiss + onOpen（官方顺序）。
func TestSidebarRowClickOpensEntry(t *testing.T) {
	f := newEntryFixture(t, pluginapi.EntryRegionSidebar, sidebarEntries(), &fakeNavigation{})
	defer f.close()

	f.click("entry-open-timeline")
	if len(f.calls.opened) != 1 || f.calls.opened[0] != "timeline" {
		t.Fatalf("点行没有走 onOpen：opened = %v", f.calls.opened)
	}
	if f.calls.dismissed != 1 {
		t.Fatalf("点行应先 onDismiss 再 onOpen（官方 :47），dismissed = %d", f.calls.dismissed)
	}
}

// TestPinButtonPinsUnpinnedEntry：点行尾固定按钮 = onDismiss + onPinnedChange(id,true)。
func TestPinButtonPinsUnpinnedEntry(t *testing.T) {
	f := newEntryFixture(t, pluginapi.EntryRegionSidebar, sidebarEntries(), &fakeNavigation{})
	defer f.close()

	f.click("entry-pin-timeline")
	if len(f.calls.pinned) != 1 || f.calls.pinned[0] != "timeline" {
		t.Fatalf("点固定按钮没有走 onPinnedChange(id,true)：pinned = %v", f.calls.pinned)
	}
	if f.calls.dismissed != 1 {
		t.Fatalf("点固定按钮应先 onDismiss（官方 :56），dismissed = %d", f.calls.dismissed)
	}
}

// TestBrowsePluginsRowOpensList：面板导航在册 → 画「浏览更多插件」，点了 openList。
func TestBrowsePluginsRowOpensList(t *testing.T) {
	nav := &fakeNavigation{}
	f := newEntryFixture(t, pluginapi.EntryRegionSidebar, sidebarEntries(), nav)
	defer f.close()

	if !f.session.HasText(zhInstall) {
		t.Fatalf("面板导航在册却没画官方 `install`（%q）", zhInstall)
	}
	f.click("entry-browse-plugins")
	if nav.opened != 1 {
		t.Fatalf("点「浏览更多插件」没有调 openList，opened = %d", nav.opened)
	}
	if f.calls.dismissed != 1 {
		t.Fatalf("点「浏览更多插件」应先 onDismiss（官方 :63），dismissed = %d", f.calls.dismissed)
	}
}

// TestBrowsePluginsRowHiddenWithoutNavigation：服务不在册 → 整行不画（官方 browsable）。
func TestBrowsePluginsRowHiddenWithoutNavigation(t *testing.T) {
	f := newEntryFixture(t, pluginapi.EntryRegionSidebar, sidebarEntries(), nil)
	defer f.close()

	if f.session.HasText(zhInstall) {
		t.Fatal("面板导航不在册却画了「浏览更多插件」：点了不会开任何东西（官方 browsable 判据）")
	}
	if _, ok := f.hit("entry-browse-plugins"); ok {
		t.Fatal("面板导航不在册却挂了「浏览更多插件」的命中区")
	}
}

// TestCreatePluginRowIsHIDEEntilHostAcceptsAgentPreset：「创建你的插件」整行不画。
//
// 宿主 DSH-GO 的 `session.create` 只收 `mode`（`internal/transport/server.go` 的
// `handleSessionCreate` + `opt.Create func(mode string)`），官方那一行要的是
// `agentPreset:'cordis'`——画出来点了也只会建出一个没有预设的会话。HIDE 到宿主补上。
func TestCreatePluginRowIsHIDEEntilHostAcceptsAgentPreset(t *testing.T) {
	nav := &fakeNavigation{}
	f := newEntryFixture(t, pluginapi.EntryRegionSidebar, sidebarEntries(), nav)
	defer f.close()

	if f.session.HasText(zhCreate) {
		t.Fatalf("宿主还收不了 agentPreset，「创建你的插件」（%q）必须 HIDE", zhCreate)
	}
	if _, ok := f.hit("entry-create-plugin"); ok {
		t.Fatal("「创建你的插件」不画，却还挂着一个命中区")
	}
}

// TestConversationRowsShowEveryFoldedViewWithActiveMark：会话头画全部，active 带对勾。
func TestConversationRowsShowEveryFoldedViewWithActiveMark(t *testing.T) {
	entries := []pluginapi.EntryMenuEntry{
		{ID: "chat", Label: "对话", Pinned: true, Active: false},
		{ID: "trajectory", Label: "轨迹", Pinned: true, Active: true},
	}
	on := newEntryFixture(t, pluginapi.EntryRegionConversation, entries, &fakeNavigation{})
	defer on.close()

	// 会话头区域 `moreEntries = entries`：宿主已经把折起来的 tab 筛好了，这里**不**再
	// 按 pinned 过滤（官方 :38 的两个分支）。
	if !on.session.HasText("对话") || !on.session.HasText("轨迹") {
		t.Fatal("会话头 More 菜单漏画了折起来的 view（官方 moreEntries = entries）")
	}
	// 会话头这一侧没有行尾固定按钮。
	if _, ok := on.hit("entry-pin-chat"); ok {
		t.Fatal("会话头区域画了左栏才有的固定按钮")
	}
	// active 那一条多一枚对勾：图标是矢量，按"本帧多出至少一条矢量图元"判。
	inactive := []pluginapi.EntryMenuEntry{
		{ID: "chat", Label: "对话", Pinned: true, Active: false},
		{ID: "trajectory", Label: "轨迹", Pinned: true, Active: false},
	}
	off := newEntryFixture(t, pluginapi.EntryRegionConversation, inactive, &fakeNavigation{})
	defer off.close()
	activePrims := len(on.session.Snapshot().Primitives)
	idlePrims := len(off.session.Snapshot().Primitives)
	if activePrims <= idlePrims {
		t.Fatalf("active 条目的行尾对勾没画出来（矢量图元 %d 条，未激活档 %d 条）", activePrims, idlePrims)
	}

	on.click("entry-open-chat")
	if len(on.calls.opened) != 1 || on.calls.opened[0] != "chat" {
		t.Fatalf("点会话头的行没有走 onOpen：%v", on.calls.opened)
	}
}

// TestIconSymbolsMatchRc2Sprite：三枚图标按 sprite 原文转写（不是手画近似形）。
func TestIconSymbolsMatchRc2Sprite(t *testing.T) {
	pin, ok := pinOutlineIcon(gfx.RGB24(0xffffff))
	if !ok || pin.ViewBox != 16 || pin.Stroke != 1 || pin.Join != gfx.JoinRound {
		t.Fatalf("pin-outline 转写不对：ok=%v viewBox=%v stroke=%v join=%v", ok, pin.ViewBox, pin.Stroke, pin.Join)
	}
	if len(pin.Commands) == 0 || len(pin.Layers) != 1 {
		t.Fatalf("pin-outline 应是「描边一条 + 一个填充层」：commands=%d layers=%d",
			len(pin.Commands), len(pin.Layers))
	}
	pinwheel, ok := pluginPinwheelIcon()
	if !ok || pinwheel.ViewBox != 16 || pinwheel.Stroke != 1 || len(pinwheel.Layers) != 0 {
		t.Fatalf("plugin-pinwheel-outline 转写不对：ok=%v viewBox=%v stroke=%v layers=%d",
			ok, pinwheel.ViewBox, pinwheel.Stroke, len(pinwheel.Layers))
	}
	if len(pinwheel.Commands) < 4 {
		t.Fatalf("plugin-pinwheel-outline 是 4 条曲线，只解出 %d 条命令", len(pinwheel.Commands))
	}
	check, ok := checkOutlineIcon()
	if !ok || check.ViewBox != 16 || check.Stroke != 1 {
		t.Fatalf("check-outline 转写不对：ok=%v viewBox=%v stroke=%v", ok, check.ViewBox, check.Stroke)
	}
}

// TestSidebarRowsGeometry：行/固定按钮/分离线的几何按官方数值落位。
//
// 只断言**能引用的官方数**（`_item_70kcw_173` 的 min-height 34、`.pinButton` 的
// 22×22、两行上下不相叠），不猜像素级边距：viva 的命中区矩形就是这一行真正占的盒。
func TestSidebarRowsGeometry(t *testing.T) {
	f := newEntryFixture(t, pluginapi.EntryRegionSidebar, sidebarEntries(), &fakeNavigation{})
	defer f.close()

	row := f.mustHit("entry-open-schedules")
	if row.Bounds.Height < entryRowMinHeight-0.5 || row.Bounds.Height > entryRowMinHeight+8 {
		t.Fatalf("条目行高 %v，want ≈%v（官方 `_item_70kcw_173{min-height:34px}`）",
			row.Bounds.Height, entryRowMinHeight)
	}
	pin := f.mustHit("entry-pin-schedules")
	if pin.Bounds.Width != sidebarPinBox || pin.Bounds.Height != sidebarPinBox {
		t.Fatalf("固定按钮 %v×%v，want %v×%v（官方 `.pinButton{width:22px;height:22px}`）",
			pin.Bounds.Width, pin.Bounds.Height, sidebarPinBox, sidebarPinBox)
	}
	if pin.Bounds.X < row.Bounds.X+row.Bounds.Width {
		t.Fatalf("固定按钮压在条目行里（pin.x=%v row.right=%v）：官方它是行尾独立一格",
			pin.Bounds.X, row.Bounds.X+row.Bounds.Width)
	}
	browse := f.mustHit("entry-browse-plugins")
	if browse.Bounds.Y < pin.Bounds.Y+pin.Bounds.Height {
		t.Fatalf("「浏览更多插件」没有排条目行下面（browse.y=%v pin.bottom=%v）",
			browse.Bounds.Y, pin.Bounds.Y+pin.Bounds.Height)
	}
}
