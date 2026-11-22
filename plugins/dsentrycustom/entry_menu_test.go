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
//	⑤ 「创建你的插件」点了 = onDismiss + startSession({agentPreset:'cordis',
//	   区域 prompt, whenDraftExists:'append'})，不自动发送
//
// 点击走真 Viva 无头窗口（`headless.Session.Click` → 真命中区 → 真事件 pass），
// 与 `plugins/dssidebar/entry_menu_test.go` 同一套夹具写法；读状态一律"泵到条件成立"。

import (
	"strings"
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
	started     []pluginapi.StartSessionOptions
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
	f.env = &pluginapi.Env{Theme: theme, Tree: tree, Feat: f.feat, Act: pluginapi.Actions{
		StartSession: func(options pluginapi.StartSessionOptions) {
			f.started = append(f.started, options)
		},
	}}
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
				switch region {
				case pluginapi.EntryRegionSidebar:
					f.p.drawSidebarRows(cc, f.env)
				case pluginapi.EntryRegionRightbar:
					f.p.drawRightGuideRows(cc, f.env)
				default:
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
	// 浏览服务缺席不牵连 Creator；官方此时由 Creator 自己承担 separatorBefore。
	f.mustHit("entry-create-plugin")
}

// TestCreatePluginRowStartsCordisDraftByRegion：两个已接区域都画官方 Creator；
// 点击只发 StartSession 动作，preset / 区域 prompt / append 三格逐字核对。
func TestCreatePluginRowStartsCordisDraftByRegion(t *testing.T) {
	for _, tc := range []struct {
		region string
		prompt string
	}{
		{region: pluginapi.EntryRegionSidebar, prompt: zhCreatePromptSidebar},
		{region: pluginapi.EntryRegionConversation, prompt: zhCreatePromptConversation},
		{region: pluginapi.EntryRegionRightbar, prompt: zhCreatePromptRightbar},
	} {
		t.Run(tc.region, func(t *testing.T) {
			f := newEntryFixture(t, tc.region, sidebarEntries(), &fakeNavigation{})
			defer f.close()
			f.mustHit("entry-create-plugin")
			f.click("entry-create-plugin")
			if f.calls.dismissed != 1 {
				t.Fatalf("Creator 应先 onDismiss，dismissed=%d", f.calls.dismissed)
			}
			if len(f.started) != 1 {
				t.Fatalf("Creator 应调一次 StartSession，实际 %d", len(f.started))
			}
			got := f.started[0]
			if got.AgentPreset != "cordis" || got.Prompt != tc.prompt || got.WhenDraftExists != pluginapi.DraftAppend {
				t.Fatalf("StartSession = %+v，want cordis / %q / append", got, tc.prompt)
			}
		})
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

// TestIconSymbolsMatchRc2Sprite：四枚图标按 sprite 原文转写（不是手画近似形）。
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
	plus, ok := plusOutlineIcon()
	if !ok || plus.ViewBox != 16 || plus.Stroke != 1 || len(plus.Commands) != 4 {
		t.Fatalf("plus-outline 转写不对：ok=%v viewBox=%v stroke=%v commands=%d",
			ok, plus.ViewBox, plus.Stroke, len(plus.Commands))
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
	// 官方 `.pinAnchor{position:absolute;right:6px;top:0;bottom:0}`（OFF:EntryMenu.module.css:15）：
	// 固定按钮**绝对定位盖在行内**、右边缘距行右 6px——不是"行外独立一格"。
	// （旧断言 `pin.x < row.right` 判的是相反的 DOM，已按官方改写；见 docs 的差异登记。）
	if inset := row.Bounds.X + row.Bounds.Width - (pin.Bounds.X + pin.Bounds.Width); inset < sidebarPinRight-0.5 || inset > sidebarPinRight+0.5 {
		t.Fatalf("固定按钮右边缘距行右 %v，want %v（官方 `.pinAnchor{right:6px}`）", inset, sidebarPinRight)
	}
	if pin.Bounds.Y < row.Bounds.Y-0.5 || pin.Bounds.Y+pin.Bounds.Height > row.Bounds.Y+row.Bounds.Height+0.5 {
		t.Fatalf("固定按钮没竖直落在行内（pin %v..%v，row %v..%v）：官方 `.pinAnchor{top:0;bottom:0}` 拉满行高再居中",
			pin.Bounds.Y, pin.Bounds.Y+pin.Bounds.Height, row.Bounds.Y, row.Bounds.Y+row.Bounds.Height)
	}
	browse := f.mustHit("entry-browse-plugins")
	if browse.Bounds.Y < pin.Bounds.Y+pin.Bounds.Height {
		t.Fatalf("「浏览更多插件」没有排条目行下面（browse.y=%v pin.bottom=%v）",
			browse.Bounds.Y, pin.Bounds.Y+pin.Bounds.Height)
	}
	create := f.mustHit("entry-create-plugin")
	if create.Bounds.Y < browse.Bounds.Y+browse.Bounds.Height {
		t.Fatalf("Creator 没有排在浏览行下面（create.y=%v browse.bottom=%v）",
			create.Bounds.Y, browse.Bounds.Y+browse.Bounds.Height)
	}
}

// TestSidebarLongLabelWrapsClearOfPin：长条目名换行，且右边缘停在固定按钮左侧。
//
// 两条官方规则一起验：`.entryTitle{overflow-wrap:anywhere;white-space:normal}`
// （OFF:EntryMenu.module.css:2，条目名可以换行、不是 ellipsis）与
// `.sidebarEntry .entryName{padding-right:28px}`（同文件 :6，给绝对定位的
// `.pinAnchor` 让出 28 = 22 + 6 的宽度）。让位没生效时，行会被 Grow(1) 拉到
// 行右内边（pin 右侧），文字就会压到固定按钮上。
func TestSidebarLongLabelWrapsClearOfPin(t *testing.T) {
	const long = "一个名字很长很长很长很长很长很长很长的面板条目"
	entries := []pluginapi.EntryMenuEntry{{ID: "long", Label: long, Glyph: "▦"}}
	f := newEntryFixture(t, pluginapi.EntryRegionSidebar, entries, &fakeNavigation{})
	defer f.close()

	row := f.mustHit("entry-open-long")
	pin := f.mustHit("entry-pin-long")
	// 换行后每一行是一个独立文字图元：取所有"是这条条目名的真子串"的行，
	// 它们才是这条标签画出来的内容（字形 "▦" 与浏览/创建两行都不在 long 里）。
	var lines []gfx.Text
	for _, text := range f.session.Snapshot().Texts {
		if text.Value != "" && text.Value != long && strings.Contains(long, text.Value) {
			lines = append(lines, text)
		}
	}
	if len(lines) < 2 {
		t.Fatalf("长条目名只画出 %d 行：官方 `.entryTitle{white-space:normal}` 应换行，不是单行溢出", len(lines))
	}
	for _, line := range lines {
		if right := line.Bounds.X + line.Bounds.Width; right > pin.Bounds.X {
			t.Fatalf("条目名某一行右边缘 %v 压到固定按钮左边缘 %v：`.entryName{padding-right:28px}` 的让位没生效",
				right, pin.Bounds.X)
		}
	}
	if row.Bounds.Height <= entryRowMinHeight {
		t.Fatalf("长条目名换行了但行高没长（%v ≤ min-height %v）", row.Bounds.Height, entryRowMinHeight)
	}
}

// TestRightbarMenuListsOnlyTheTwoFooterRows：右栏 guide 那层 More 菜单**不列条目**，
// 只有末尾两行（官方 `region === 'rightbar'` 时 `moreEntries` 恒 `[]`，
// EntryMenuContent.tsx:38 + :62-69）。
//
// 这一条同时是「不要照抄左栏」的负向判据：向导卡本身就是入口，菜单里再列一遍就是
// 同一扇门画两次；官方那一侧明确是空表。
func TestRightbarMenuListsOnlyTheTwoFooterRows(t *testing.T) {
	entries := []pluginapi.EntryMenuEntry{
		// 一条未固定、一条已固定：官方 rightbar 区域**两条都不列**（moreEntries 恒空）。
		// 照抄左栏那一支的话，未固定的那条会被画出来，下面那条断言就会红。
		{ID: "terminal:new", Label: "新建终端", Pinned: false},
		{ID: "browser:new", Label: "浏览器", Pinned: true},
	}
	f := newEntryFixture(t, pluginapi.EntryRegionRightbar, entries, &fakeNavigation{})
	defer f.close()

	for _, label := range []string{"新建终端", "浏览器"} {
		if f.session.HasText(label) {
			t.Fatalf("右栏 More 菜单列了条目 %q：官方 rightbar 区域 moreEntries 恒空，向导卡不在菜单里重列", label)
		}
	}
	// 末尾两行都在：浏览服务在册（browsable）+ Creator 恒在。
	f.mustHit("entry-browse-plugins")
	f.mustHit("entry-create-plugin")
	if !f.session.HasText(zhInstall) || !f.session.HasText(zhCreate) {
		t.Fatalf("右栏 More 菜单漏了末尾两行：%v", f.session.Texts())
	}
}

// TestRightbarRowsFollowTheNavigationService：浏览那行仍归 `browsable` 管，
// Creator 不受牵连（与左栏/会话头同一判据，官方三段共用一份组件）。
func TestRightbarRowsFollowTheNavigationService(t *testing.T) {
	f := newEntryFixture(t, pluginapi.EntryRegionRightbar, nil, nil)
	defer f.close()

	if _, ok := f.hit("entry-browse-plugins"); ok {
		t.Fatal("面板导航不在册却给右栏挂了「浏览更多插件」的命中区")
	}
	f.mustHit("entry-create-plugin")
}

// TestRightbarRowsAreInertOutsideTheirMenu：这第三条座位也必须只在菜单里画行——
// 壳外（`Env.EntryMenu == nil`）画出来就等于把菜单行直接摊在向导页上。
func TestRightbarRowsAreInertOutsideTheirMenu(t *testing.T) {
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("entry-customization-rightbar-off")
	p := &Plugin{theme: theme, nav: &fakeNavigation{}}
	env := &pluginapi.Env{Theme: theme, Tree: tree, Feat: feat}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{Title: "entry-rightbar-off", Width: 288, Height: 200, Tree: tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env.Canvas, env.Bounds = canvas, bounds
		view.Draw(canvas, bounds, func(cc ui.C) { p.drawRightGuideRows(cc, env) })
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
			t.Fatalf("不在菜单里（Env.EntryMenu 为 nil）却画了 %q", text)
		}
	}
	if len(session.Snapshot().Hits) != 0 {
		t.Fatalf("不在菜单里却挂了 %d 个命中区", len(session.Snapshot().Hits))
	}
}
