//go:build darwin || windows || linux

// keyboard_routing_test.go —— 上下文菜单的**按键归属**：一条路，不是两条。
//
// 背景：`ui.Box.Shortcut` / `ui.Shortcut` 在 `ui.Page.Key` 接上之前是死代码，本插件
// 因此用 `pluginapi.KeyHandler` 自己处理 Shift+F10 / ↑↓ / Home / End / Enter / Space /
// Esc。页面上接线之后两套机制会同时存在，这是这一轮要收掉的东西。
//
// 归宿按官方 `dsh-client-ui-context-menu`：
//   - 触发器键（Shift+F10）归**有焦点的那个触发器元素**：`trigger.ts:29-37` 的
//     `onKeyDown` 读 `event.currentTarget`，`show()` 里 `preventDefault` +
//     `stopPropagation`；
//   - 菜单键（Esc / ↑↓ / Home / End）归**菜单自己**：`View.tsx:50-57` 的 `Menu`
//     原语带 `autoFocus` / `onClose` / `onSelect`，插件不声明任何全局键；
//   - 选中键（Enter / Space）是**菜单项自己的激活**：Viva 的输入路由器
//     （`viva@v0.6.4/internal/input/input.go:308-315`，`RoleMenuItem` 两键都算）
//     把它变成一次点击，由 painter 兑现（`app/painter.go:554`）。
//
// 于是本插件**没有** `pluginapi.KeyHandler`，两套机制不存在。
package dscontextmenu

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// TestPluginHasNoGlobalKeyHandler 钉住「不要两套并存」：本插件不得再实现
// `pluginapi.KeyHandler`。
//
// 官方 `dsh-client-ui-context-menu` 里没有任何插件级全局按键路由（`index.ts` 只注册
// 一个**开菜单过渡期**的 window keydown escape，并在处理时 `stopImmediatePropagation`）；
// 一旦这里重新长出一个 KeyHandler，同一条键就会同时落在页面路由和插件链上，
// 行为取决于哪条先命中。
func TestPluginHasNoGlobalKeyHandler(t *testing.T) {
	p := newTestPlugin()
	if h, ok := any(p).(pluginapi.KeyHandler); ok {
		t.Errorf("dscontextmenu 又实现了 pluginapi.KeyHandler（%T）：官方没有插件级全局按键路由，"+
			"菜单键归菜单原语、触发器键归有焦点的触发器（trigger.ts:29-37、View.tsx:50-57），"+
			"两套机制处理同一个键就是双重处理", h)
	}
}

// keyboardMenu 是一台最小窗口：两个都有菜单的目标，用来验「Shift+F10 只开有焦点的那个」。
type keyboardMenu struct {
	t       *testing.T
	session *headless.Session
	plugin  *Plugin
	runs    []string
}

func newKeyboardMenu(t *testing.T) *keyboardMenu {
	t.Helper()
	m := &keyboardMenu{t: t}
	p := newTestPlugin()
	m.plugin = p
	for kind, label := range map[string]string{"workspace": "工作区动作", "session": "会话动作"} {
		kind, label := kind, label
		if _, err := p.RegisterContribution(pluginapi.ContextMenuContribution{
			ID: kind + ".one", Target: kind, Order: 100,
			Resolve: func(pluginapi.ContextMenuTarget) []pluginapi.ContextMenuAction {
				return []pluginapi.ContextMenuAction{{ID: label, Label: label, Run: func() { m.runs = append(m.runs, label) }}}
			},
		}); err != nil {
			t.Fatalf("登记 %s 贡献：%v", kind, err)
		}
	}
	tree := &gfx.Tree{}
	feature := tree.Feature("context-menu-keyboard")
	rowA := tree.ID(feature, "row-a")
	rowB := tree.ID(feature, "row-b")
	pageTheme := dscore.VivaTheme(p.theme)
	// Root 与壳的 `pageRoot` 同位：页面级 `ui.Shortcut` 的 pageWide 判据要它
	// （`viva@v0.6.4/ui/shortcut.go:106-108`）。
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme, Root: "context-menu-keyboard"})
	config := app.Config{Title: "context-menu-keyboard", Width: 400, Height: 300, Tree: tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		view.Draw(canvas, gfx.Rect{Width: size.Width, Height: size.Height}, func(f ui.C) {
			// 真实调用点的形状：容器 + 子元素 + `.Hit(id)`。
			a := ui.Row(f).FillWidth().Height(40).Hit(rowA, "工作区").
				Children(func() { ui.Text(f, "工作区").FontSize(13) })
			p.Attach(f, rowA, a, pluginapi.ContextMenuTarget{Kind: "workspace", ID: "a"})
			b := ui.Row(f).FillWidth().Height(40).Hit(rowB, "会话").
				Children(func() { ui.Text(f, "会话").FontSize(13) })
			p.Attach(f, rowB, b, pluginapi.ContextMenuTarget{Kind: "session", ID: "b"})
		})
	}
	config.OnKey = func(ev gfx.KeyEvent) gfx.EventResult {
		if ev.Release {
			return gfx.EventResult{}
		}
		return view.Key(ev)
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless.Open 失败：%v", err)
	}
	t.Cleanup(session.Close)
	m.session = session
	m.pump()
	return m
}

func (m *keyboardMenu) pump() {
	m.t.Helper()
	if err := m.session.Driver().Pump(); err != nil {
		m.t.Fatalf("pump 失败：%v", err)
	}
}

func (m *keyboardMenu) text(label string) bool { return m.session.HasText(label) }

// TestShiftF10OpensOnlyTheFocusedTrigger 钉住官方触发器规则：Shift+F10 开的是
// **焦点所在**那个目标的菜单，不是"随便第一个登记过的目标"。
//
// 旧实现（`pluginapi.KeyHandler` 的 Shift+F10 分支）遍历 `p.contexts` 取第一个，
// 焦点在哪都一样；官方 `trigger.ts:31` 读的是 `event.currentTarget`，
// `ui/menu.go:633` 判的是 `target.id == event.Target`。
func TestShiftF10OpensOnlyTheFocusedTrigger(t *testing.T) {
	m := newKeyboardMenu(t)
	// 先点第二行：焦点落在会话目标上。
	if err := m.session.Click(200, 60); err != nil {
		t.Fatalf("点击第二行失败：%v", err)
	}
	m.pump()
	if err := m.session.Keys("shift+f10"); err != nil {
		t.Fatalf("Shift+F10 失败：%v", err)
	}
	m.pump()
	if !m.text("会话动作") {
		t.Fatalf("焦点在第二行（会话）时 Shift+F10 没开它的菜单：texts=%q", m.session.Texts())
	}
	if m.text("工作区动作") {
		t.Errorf("Shift+F10 开了**另一个**目标的菜单（工作区动作）：焦点不在它身上，官方不会开它")
	}

	// 先收起菜单（裸点击落在遮罩上只会消失，不会把焦点换到别的行——官方
	// `pointerdown` 的 outside 处理也是这个行为），再点第一行。
	if err := m.session.Keys("escape"); err != nil {
		t.Fatalf("Esc 失败：%v", err)
	}
	m.pump()
	if m.text("会话动作") {
		t.Fatalf("Esc 没关掉菜单：texts=%q", m.session.Texts())
	}
	if err := m.session.Click(200, 20); err != nil {
		t.Fatalf("点击第一行失败：%v", err)
	}
	m.pump()
	if err := m.session.Keys("shift+f10"); err != nil {
		t.Fatalf("Shift+F10 失败：%v", err)
	}
	m.pump()
	if !m.text("工作区动作") {
		t.Fatalf("焦点在第一行（工作区）时 Shift+F10 没开它的菜单：texts=%q", m.session.Texts())
	}
}

// TestShiftF10WithoutFocusOpensNothing 钉住另一半：焦点不在任何有菜单的目标上时，
// Shift+F10 什么都不开（官方只在触发器元素自己收到键时才开）。
func TestShiftF10WithoutFocusOpensNothing(t *testing.T) {
	m := newKeyboardMenu(t)
	// 页面刚起来，焦点为空。
	if err := m.session.Keys("shift+f10"); err != nil {
		t.Fatalf("Shift+F10 失败：%v", err)
	}
	m.pump()
	if m.text("工作区动作") || m.text("会话动作") {
		t.Errorf("焦点不在任何目标上，Shift+F10 却开了菜单：texts=%q", m.session.Texts())
	}
}
