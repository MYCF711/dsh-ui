//go:build darwin || windows || linux

package uikernel

// view_guide_more_test.go —— 右栏向导页 More 卡的机器断言（#17 ②）。
//
// 判据分四类，全部走真 Viva 无头窗口（真布局 / 真命中区 / 真点击 / 真像素）：
//
//	A. 出现条件：官方 `hasEntryActions`——`sidebar.right.tab.guide.actions` 没有
//	   占用方时，向导页里**不许**出现这张卡（GuideBody.tsx:160）
//	B. 几何与墨色：卡片就是入口卡那枚胶囊（380×≥56），图标格 26 里画 **22×22** 的
//	   `GuideMoreIcon`，且素材自带色 `#A6B2C7`（GuideMoreIcon.tsx:9）
//	C. 菜单面：点卡片 → 展开态翻成真、`sidebar.right.tab.guide.actions` **派发一次**、
//	   owner props 带 rightbar 区域与 `kind:id` 条目；菜单面宽度按官方
//	   `minWidthFromAnchor` 抬到锚点宽（GuideBody.tsx:104、module.css:128）
//	D. 关闭：点菜单面外 → 关（官方 `onClose`）；点条目 → 打开它声明的类型
//	   （GuideBody.tsx:165-168）

import (
	"fmt"
	"image"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// openGuideMoreFrame 开一帧 headless 画面并**按壳的真实时机**调一次 AfterDraw。
//
// 壳里那句是 main 的 Draw 在 `m.view.Draw(...)` 返回之后调 `GuideMore.AfterDraw()`
// （布局已定，`Box.Rect()` 才有值）。这里把 after 放在同一条位置，否则锚点永远
// 是零矩形——那会让"菜单贴在卡上"这条断言失去意义。
func openGuideMoreFrame(t *testing.T, width, height int, draw func(env *pluginapi.Env) func(ui.C), after func()) *headless.Session {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("guide-more-frame")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{
		Title: "guide-more-frame", Width: width, Height: height, Tree: tree, WindowMode: app.WindowFrameless,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			Has: func(pluginapi.Slot) bool { return false },
		}
		canvas.Fill(bounds, theme.BgBase)
		body := draw(env)
		if body == nil {
			return
		}
		view.Draw(canvas, bounds, body)
		if after != nil {
			after() // 壳的 `m.guideMore.AfterDraw()`
		}
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	session.Resize(width, height)
	for i := 0; i < 8; i++ {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	return session
}

// guideMoreEntriesForTest 是两张向导卡（终端有实现、浏览器没有）。
func guideMoreEntriesForTest() []GuideEntry {
	return []GuideEntry{
		{TabGuide: pluginapi.TabGuide{ID: "new", Title: "新建终端"}, ProviderID: "terminal", Kind: "terminal"},
		{TabGuide: pluginapi.TabGuide{ID: "new", Title: "浏览器"}, ProviderID: "browser", Kind: "browser"},
	}
}

// hitByLabel 在整帧的命中区里按可读文案找一块（不过 reachable 这关：遮罩中心
// 可能被菜单面盖住，但它的四角仍可点，那正是"点外面"这个动作）。
func hitByLabel(t *testing.T, session *headless.Session, label string) (gfx.Hit, bool) {
	t.Helper()
	for _, candidate := range session.Snapshot().Hits {
		if candidate.Label == label {
			return candidate, true
		}
	}
	return gfx.Hit{}, false
}

// countInk 数一块矩形里落在 want（±tolerance）上的像素数，并回它们的包围盒。
func countInk(picture *image.RGBA, area image.Rectangle, want gfx.Color, tolerance uint8) (int, image.Rectangle) {
	red := uint8(want.Red*255 + 0.5)
	green := uint8(want.Green*255 + 0.5)
	blue := uint8(want.Blue*255 + 0.5)
	near := func(got, expected uint8) bool {
		delta := int(got) - int(expected)
		if delta < 0 {
			delta = -delta
		}
		return delta <= int(tolerance)
	}
	count := 0
	box := image.Rectangle{}
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			pixel := picture.RGBAAt(x, y)
			if !near(pixel.R, red) || !near(pixel.G, green) || !near(pixel.B, blue) {
				continue
			}
			count++
			if box.Empty() {
				box = image.Rect(x, y, x+1, y+1)
				continue
			}
			box = box.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return count, box
}

// ── A. 出现条件 ─────────────────────────────────────────────────────

// TestGuideMoreCardOnlyWithEntryActions：官方这张卡只在座位有占用方时出现。
//
// 两个方向都要成立：没占用方 → 一个字都不许多画；有占用方 → 必须画出来
// （否则这条座位就是"注册了也看不见"，与 #17 的 HUD 前身同一个病）。
func TestGuideMoreCardOnlyWithEntryActions(t *testing.T) {
	for _, hasActions := range []bool{false, true} {
		hasActions := hasActions
		t.Run(fmt.Sprintf("hasActions=%v", hasActions), func(t *testing.T) {
			var state GuideMoreState
			dispatched := 0
			session := openGuideMoreFrame(t, 520, 360, func(env *pluginapi.Env) func(ui.C) {
				env.Has = func(slot pluginapi.Slot) bool {
					return hasActions && slot == OffRightGuideActions
				}
				env.HasKey = func(pluginapi.Slot, string) bool { return false }
				env.DrawKeyed = func(ui.C, pluginapi.Slot, string) {}
				env.Act.OpenRightTab = func(string) {}
				return func(f ui.C) {
					DrawRightGuideMore(f, env, guideMoreEntriesForTest(), GuideMoreOptions{
						State:      &state,
						Dispatch:   func(pluginapi.Slot) { dispatched++ },
						HasActions: hasActions,
					})
				}
			}, state.AfterDraw)
			if got := session.HasText(GuideMoreTitle); got != hasActions {
				t.Fatalf("画出了 More 卡 = %v，want %v（官方 `hasEntryActions` 时才有这张卡）：%v",
					got, hasActions, session.Texts())
			}
			if !hasActions && dispatched != 0 {
				t.Fatalf("没有占用方却派发了 %d 次 guide.actions 座位", dispatched)
			}
		})
	}
}

// TestGuideMoreCardWithoutStateIsNotDrawn：没有跨帧状态（最小装配/单测环境）时
// 整张卡不画——一个点了永远不展开的按钮比不画更糟。
func TestGuideMoreCardWithoutStateIsNotDrawn(t *testing.T) {
	session := openGuideMoreFrame(t, 520, 360, func(env *pluginapi.Env) func(ui.C) {
		env.Has = func(slot pluginapi.Slot) bool { return slot == OffRightGuideActions }
		env.HasKey = func(pluginapi.Slot, string) bool { return false }
		env.DrawKeyed = func(ui.C, pluginapi.Slot, string) {}
		env.Act.OpenRightTab = func(string) {}
		return func(f ui.C) {
			DrawRightGuideMore(f, env, guideMoreEntriesForTest(), GuideMoreOptions{
				State: nil, Dispatch: func(pluginapi.Slot) {}, HasActions: true,
			})
		}
	}, nil)
	if session.HasText(GuideMoreTitle) {
		t.Fatalf("没有跨帧状态却画了 More 卡（点了不会展开）：%v", session.Texts())
	}
}

// ── B. 几何与墨色 ───────────────────────────────────────────────────

// TestGuideMoreCardIsTheEntryCapsule 钉住官方那套几何与素材色：
//
//	`.entryCell{width:380px;max-width:100%}`（module.css:121）
//	`.entry{min-height:56px;padding:14px 20px;gap:14}`（:43-59）
//	`.entryIcon{width:26px;height:26px}`（:70-78）里画 **22×22** 的素材（GuideMoreIcon.tsx:8）
//	素材自带 `stroke="#A6B2C7"`（:9），不跟主题令牌
func TestGuideMoreCardIsTheEntryCapsule(t *testing.T) {
	var state GuideMoreState
	session := openGuideMoreFrame(t, 520, 360, func(env *pluginapi.Env) func(ui.C) {
		env.Has = func(slot pluginapi.Slot) bool { return slot == OffRightGuideActions }
		env.HasKey = func(pluginapi.Slot, string) bool { return false }
		env.DrawKeyed = func(ui.C, pluginapi.Slot, string) {}
		env.Act.OpenRightTab = func(string) {}
		return func(f ui.C) {
			DrawRightGuideMore(f, env, guideMoreEntriesForTest(), GuideMoreOptions{
				State: &state, Dispatch: func(pluginapi.Slot) {}, HasActions: true,
			})
		}
	}, state.AfterDraw)
	hit, ok := session.Hit(GuideMoreTitle)
	if !ok {
		t.Fatalf("More 卡没有可点到的命中区（画没画出来都说不清）：%v", session.Texts())
	}
	if got := hit.Bounds.Width; got < 379.5 || got > 380.5 {
		t.Fatalf("卡片宽 = %v，want 380（`.entryCell{width:380px}`）", got)
	}
	if hit.Bounds.Height < GuideEntryMinHeight {
		t.Fatalf("卡片高 = %v，want ≥ %v（`.entry{min-height:56px}`）", hit.Bounds.Height, GuideEntryMinHeight)
	}
	picture, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	// 图标格：卡片左缘 + padding 20，竖向居中，26×26。
	iconBox := image.Rect(
		int(hit.Bounds.X+GuideEntryPadX)-1, int(hit.Bounds.Y+hit.Bounds.Height/2-GuideEntryIconBox/2)-1,
		int(hit.Bounds.X+GuideEntryPadX+GuideEntryIconBox)+1, int(hit.Bounds.Y+hit.Bounds.Height/2+GuideEntryIconBox/2)+1,
	)
	// 字面色取自官方素材本身（`GuideMoreIcon.tsx:9` 的 `stroke="#A6B2C7"`）：
	// **不引用本包的常量**，否则把常量改掉时两边一起动，判据就永远绿。
	want := gfx.RGB24(0xA6B2C7)
	count, ink := countInk(picture, iconBox, want, 10)
	if count < 12 {
		t.Fatalf("图标格里 `#A6B2C7` 的墨迹只有 %d 像素：GuideMoreIcon 没画出来或换了颜色", count)
	}
	// 22 而不是 26：素材的图形在 22 单位 viewBox 里只占 3.55..18.81（宽 15.26 单位），
	// 22px 档实测墨迹 14×14，26px 档会到 ~17×17。区间卡在两者之间。
	if ink.Dx() < 13 || ink.Dx() > 15 || ink.Dy() < 13 || ink.Dy() > 15 {
		t.Fatalf("图标墨迹包围盒 = %v×%v，want 14×14（22px 档；26px 档约 17×17）", ink.Dx(), ink.Dy())
	}
	// 墨色只许出现在图标格：卡片右半边（文字区）不许有同色像素。
	rightHalf := image.Rect(int(hit.Bounds.X+hit.Bounds.Width*0.6), int(hit.Bounds.Y), int(hit.Bounds.X+hit.Bounds.Width), int(hit.Bounds.Y+hit.Bounds.Height))
	if leaked, _ := countInk(picture, rightHalf, want, 10); leaked != 0 {
		t.Fatalf("卡片文字区出现了 %d 枚图标色像素：图标跑出 26px 格了", leaked)
	}
}

// ── C. 菜单面 ───────────────────────────────────────────────────────

// TestGuideMoreCardOpensMenuAndDispatchesOnce 钉住"点了就展开"这整条链：
// 展开态翻真、座位派发一次、owner props 是 rightbar + `kind:id`、菜单面至少与卡同宽。
func TestGuideMoreCardOpensMenuAndDispatchesOnce(t *testing.T) {
	var state GuideMoreState
	var props *pluginapi.EntryMenuProps
	dispatches := 0
	fixedEntry := pluginapi.EntryMenuEntry{ID: "files:new", Label: "文件", Pinned: true}
	entries := append(guideMoreEntriesForTest(),
		GuideEntry{TabGuide: pluginapi.TabGuide{ID: "new", Title: "文件"}, ProviderID: "files", Kind: "files"})
	session := openGuideMoreFrame(t, 520, 360, func(env *pluginapi.Env) func(ui.C) {
		env.Has = func(slot pluginapi.Slot) bool { return slot == OffRightGuideActions }
		env.HasKey = func(pluginapi.Slot, string) bool { return false }
		env.DrawKeyed = func(ui.C, pluginapi.Slot, string) {}
		env.Act.OpenRightTab = func(string) {}
		return func(f ui.C) {
			dispatches, props = 0, nil // 每帧重记：断言的是"这一帧派发了几次"
			DrawRightGuideMore(f, env, entries, GuideMoreOptions{
				State: &state,
				Dispatch: func(slot pluginapi.Slot) {
					if slot != OffRightGuideActions {
						t.Errorf("派发了别的座位 %q", slot)
					}
					dispatches++
					props = env.EntryMenu
				},
				HasActions: true,
			})
		}
	}, state.AfterDraw)
	hit, ok := session.Hit(GuideMoreTitle)
	if !ok {
		t.Fatal("More 卡没有可点到的命中区")
	}
	if err := session.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		t.Fatalf("点 More 卡失败：%v", err)
	}
	for i := 0; i < 8 && !state.Open; i++ {
		if err := session.Frame(); err != nil {
			t.Fatal(err)
		}
	}
	if !state.Open {
		t.Fatalf("点了 More 卡展开态没翻真（官方 `onClick={() => setOpen(v => !v)}`）")
	}
	if err := session.Frame(); err != nil {
		t.Fatal(err)
	}
	if dispatches != 1 {
		t.Fatalf("菜单展开那一帧派发了 %d 次 guide.actions，want 1", dispatches)
	}
	if props == nil {
		t.Fatal("派发时 `env.EntryMenu` 是 nil：占用方拿不到 owner props")
	}
	if props.Region != pluginapi.EntryRegionRightbar {
		t.Fatalf("owner props 区域 = %q，want rightbar", props.Region)
	}
	if len(props.Entries) != 2 {
		t.Fatalf("owner props 条目 = %d 条，want 2（固定 kind 不进条目表）：%+v", len(props.Entries), props.Entries)
	}
	want := map[string]bool{"terminal:new": true, "browser:new": true}
	for _, entry := range props.Entries {
		if !want[entry.ID] {
			t.Fatalf("条目 id = %q，want `kind:id`（guide-pins.ts:15-17）；固定 kind 的 %q 不该进来",
				entry.ID, fixedEntry.ID)
		}
		if !entry.Pinned {
			t.Fatalf("条目 %q 的 pinned = false：官方默认全部固定", entry.ID)
		}
	}
	// 菜单面宽度：官方 `minWidthFromAnchor`（GuideBody.tsx:104）⇒ 至少与锚点同宽。
	if state.PanelHeight <= 0 {
		t.Fatal("菜单面高度没量到（AfterDraw 没接上 OnPanel 的盒）")
	}
	panelWidth := state.panel.Rect().Width
	if delta := panelWidth - hit.Bounds.Width; delta < -0.5 || delta > 0.5 {
		t.Fatalf("菜单面宽 = %v，want 正好锚点宽 %v（`minWidthFromAnchor` + 两行内容的 max-content 比锚点窄）",
			panelWidth, hit.Bounds.Width)
	}
}

// ── D. 关闭与打开 ───────────────────────────────────────────────────

// TestGuideMoreMenuDismissesOnOutsideClick：点菜单面外 = 官方 `onClose`。
func TestGuideMoreMenuDismissesOnOutsideClick(t *testing.T) {
	var state GuideMoreState
	state.Open = true
	session := openGuideMoreFrame(t, 520, 360, func(env *pluginapi.Env) func(ui.C) {
		env.Has = func(slot pluginapi.Slot) bool { return slot == OffRightGuideActions }
		env.HasKey = func(pluginapi.Slot, string) bool { return false }
		env.DrawKeyed = func(ui.C, pluginapi.Slot, string) {}
		env.Act.OpenRightTab = func(string) {}
		return func(f ui.C) {
			DrawRightGuideMore(f, env, guideMoreEntriesForTest(), GuideMoreOptions{
				State: &state, Dispatch: func(pluginapi.Slot) {}, HasActions: true,
			})
		}
	}, state.AfterDraw)
	// 等锚点量到（第一帧 Draw 后 AfterDraw 才写），再看遮罩在不在。
	if state.Anchor.Width <= 0 {
		t.Fatalf("锚点没量到：%+v", state.Anchor)
	}
	scrim, ok := hitByLabel(t, session, "关闭菜单")
	if !ok {
		t.Fatalf("菜单开着却没有整窗遮罩（点外面关不掉）：%v", session.Texts())
	}
	if err := session.Click(scrim.Bounds.X+4, scrim.Bounds.Y+4); err != nil {
		t.Fatalf("点菜单外失败：%v", err)
	}
	for i := 0; i < 8 && state.Open; i++ {
		if err := session.Frame(); err != nil {
			t.Fatal(err)
		}
	}
	if state.Open {
		t.Fatal("点菜单面外没有关掉菜单（官方 onClose）")
	}
}

// TestGuideMoreMenuOpensTheDeclaredKind：菜单里选中一条 = 打开它声明的类型
// （官方 `onOpen` → `open(kind)`，GuideBody.tsx:165-168）。
func TestGuideMoreMenuOpensTheDeclaredKind(t *testing.T) {
	var state GuideMoreState
	state.Open = true
	opened := ""
	var props *pluginapi.EntryMenuProps
	session := openGuideMoreFrame(t, 520, 360, func(env *pluginapi.Env) func(ui.C) {
		env.Has = func(slot pluginapi.Slot) bool { return slot == OffRightGuideActions }
		env.HasKey = func(pluginapi.Slot, string) bool { return false }
		env.DrawKeyed = func(ui.C, pluginapi.Slot, string) {}
		env.Act.OpenRightTab = func(kind string) { opened = kind }
		return func(f ui.C) {
			DrawRightGuideMore(f, env, guideMoreEntriesForTest(), GuideMoreOptions{
				State: &state,
				Dispatch: func(pluginapi.Slot) {
					props = env.EntryMenu
				},
				HasActions: true,
			})
		}
	}, state.AfterDraw)
	for i := 0; i < 8 && props == nil; i++ {
		if err := session.Frame(); err != nil {
			t.Fatal(err)
		}
	}
	if props == nil || props.Open == nil {
		t.Fatal("菜单派发时没给 onOpen：占用方的行点了不会有任何动作")
	}
	props.Open("browser:new")
	if opened != "browser" {
		t.Fatalf("选中 browser:new 打开的类型 = %q，want browser（`kind:id` 的 kind 那一半）", opened)
	}
}

// ── 宽度规则的单元判据 ──────────────────────────────────────────────

// TestEntryMenuWidthFollowsOfficialRules 把三处的 `max-width` 与 guide 的
// `minWidthFromAnchor` 逐条钉住（`SidebarRoot.module.css:726-729`、
// `GuideBody.module.css:128`、`GuideBody.tsx:104`）。
func TestEntryMenuWidthFollowsOfficialRules(t *testing.T) {
	cap := func(bounds float32) float32 { return bounds - 2*EntryMoreMenuEdgeMargin }
	// 左栏/会话头：`width:288px;max-width:calc(100vw-24px)`。
	if got := EntryMoreMenuWidthFor(1000); got != EntryMoreMenuWidth {
		t.Fatalf("宽窗口下 = %v，want 288", got)
	}
	if got := EntryMoreMenuWidthFor(240); got != cap(240) {
		t.Fatalf("窄窗口下 = %v，want %v（窗口优先）", got, cap(240))
	}
	// 右栏 guide：`width:max-content;max-width:min(480px,calc(100vw-24px))` + minWidthFromAnchor。
	cases := []struct {
		name     string
		anchor   float32
		bounds   float32
		expected float32
	}{
		{name: "桌面档按锚点宽（380）", anchor: 380, bounds: 1000, expected: 380},
		{name: "窄栏按锚点宽收窄", anchor: 252, bounds: 1000, expected: 252},
		{name: "锚点比 480 还宽时被 480 夹住", anchor: 600, bounds: 1000, expected: GuideMoreMenuMax},
		{name: "窗口比锚点还窄时窗口优先", anchor: 600, bounds: 400, expected: cap(400)},
		{name: "锚点还没量到（首帧）时按 480 上限走", anchor: 0, bounds: 1000, expected: GuideMoreMenuMax},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := entryMenuAnchorWidth(GuideMoreMenuMax, tc.anchor, tc.bounds); got != tc.expected {
				t.Fatalf("宽度 = %v，want %v", got, tc.expected)
			}
		})
	}
}

// ── AfterDraw：锚点回填与"卡片离场就归零" ────────────────────────────

// TestGuideMoreAfterDrawMeasuresAndResets 钉住壳那句 `guideMore.AfterDraw()`：
//
//	① 画过卡片 → 锚点被量到（否则菜单永远贴在 (0,0)）
//	② 卡片这一帧没画（座位没人 / 向导页不在）→ 展开态归零、锚点清零
//	   （官方 unmount 就是丢 `useState`，不留一个"下次出现就已经展开"的幽灵菜单）
func TestGuideMoreAfterDrawMeasuresAndResets(t *testing.T) {
	var state GuideMoreState
	drawCard := true
	session := openGuideMoreFrame(t, 520, 360, func(env *pluginapi.Env) func(ui.C) {
		env.Has = func(slot pluginapi.Slot) bool { return slot == OffRightGuideActions }
		env.HasKey = func(pluginapi.Slot, string) bool { return false }
		env.DrawKeyed = func(ui.C, pluginapi.Slot, string) {}
		env.Act.OpenRightTab = func(string) {}
		return func(f ui.C) {
			DrawRightGuideMore(f, env, guideMoreEntriesForTest(), GuideMoreOptions{
				State: &state, Dispatch: func(pluginapi.Slot) {}, HasActions: drawCard,
			})
		}
	}, state.AfterDraw)
	if state.Anchor.Width < 379 {
		t.Fatalf("锚点宽 = %v，want 380（AfterDraw 没把卡片矩形量回来）", state.Anchor.Width)
	}
	state.Open = true
	drawCard = false
	for i := 0; i < 4; i++ {
		if err := session.Frame(); err != nil {
			t.Fatal(err)
		}
	}
	if state.Open {
		t.Fatal("卡片离场后展开态还留着：下次向导页一出现，菜单就会自己开着")
	}
	if state.Anchor != (gfx.Rect{}) {
		t.Fatalf("卡片离场后锚点没清零：%+v", state.Anchor)
	}
}

// ── 骨架接线：向导页体里真的出现这张卡 ──────────────────────────────

// TestShellGuideBodyDrawsTheMoreCard 走**骨架自己那条路**（`drawRightbarSkeleton` 的
// 向导页体分支）：右栏没有任何 tab 类型时向导页体就是 fallback，More 卡必须真的画出来；
// 而 `sidebar.right.tab.guide.actions` 在菜单没开时**一次都不许派发**——旧实现是一行
// 裸座位，每帧都派发一次（那几行会直接摊在向导页上，而不是在菜单里）。
func TestShellGuideBodyDrawsTheMoreCard(t *testing.T) {
	var state GuideMoreState
	dispatches := 0
	session := openGuideMoreFrame(t, 1180, 800, func(env *pluginapi.Env) func(ui.C) {
		env.Has = func(slot pluginapi.Slot) bool { return slot == OffRightGuideActions }
		env.HasKey = func(pluginapi.Slot, string) bool { return false }
		env.DrawKeyed = func(ui.C, pluginapi.Slot, string) {}
		env.Act.OpenRightTab = func(string) {}
		env.Act.SelectRightTab = func(string) {}
		options := ShellOptions{
			RightbarOn:        true,
			RightbarPreferred: 420,
			SidebarPreferred:  280,
			GuideEntries:      guideMoreEntriesForTest(),
			GuideMore:         &state,
		}
		return func(f ui.C) {
			dispatch := func(slot pluginapi.Slot) {
				if slot == OffRightGuideActions {
					dispatches++
				}
			}
			FirstScreenWithMeasure(f, env, options, dispatch, nil)
		}
	}, state.AfterDraw)
	if !session.HasText(GuideMoreTitle) {
		t.Fatalf("骨架的向导页体没画出 More 卡：%v", session.Texts())
	}
	if dispatches != 0 {
		t.Fatalf("菜单没开却派发了 %d 次 guide.actions：那几行会直接画在向导页上", dispatches)
	}
}
