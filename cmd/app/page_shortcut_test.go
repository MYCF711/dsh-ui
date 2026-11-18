//go:build darwin || windows || linux

// page_shortcut_test.go —— 壳把按键送进 `ui.Page.Key` 的那条驱动链。
//
// 背景：本壳是 `Page.Draw` 把整页画进内容区的形态（`newConfig` 的 cfg.Paint 里那句
// `m.view.Draw(canvas, contentBounds, …)`）。viva 对这种形态的规定是**宿主自己**用
// `Page.Key` 路由输入——"A host that draws a page this way routes its input with
// Key, Pointer, Wheel and TextInput; Config does that for a whole window"
// （`viva@v0.6.4/ui/page.go:716-721`）——而 `ui.Box.Shortcut` / `ui.Shortcut` 只由
// 那条路由派发（`ui/page.go:502-514`、`ui/shortcut.go:94-135`）。壳以前从不调它，
// 于是插件里每条 Shortcut 都没有驱动源，只能靠 `pluginapi.KeyHandler` 绕。
//
// 下面这些测试盯的是**壳的接线**（`newConfig` 的 cfg.OnKey）而不是插件内部。
// 判据里都带一条「探针插件收没收到这个键」：页面接掉的键必须一个都不进
// `kernel.HandleKey`（否则就是同一个键被处理两次），页面不认的键必须照样进去
// （既有 KeyHandler 路径不能被破坏）。
package main

import (
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscontextmenu"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// pageKeySpy 是「想接键盘的插件」的最小替身（`pluginapi.KeyHandler`）：
// 只记录看见了哪些键、从不处理。装配顺序排最前——`kernel.HandleKey` 按装配顺序
// 转发，它必须第一个看到键，才能证明「页面接掉的键没有再进插件链」。
type pageKeySpy struct {
	mu   sync.Mutex
	seen []gfx.KeyEvent
}

func (*pageKeySpy) ID() string                       { return "test-page-key-spy" }
func (*pageKeySpy) Provides() []string               { return nil }
func (*pageKeySpy) Requires() []string               { return nil }
func (*pageKeySpy) Register(pluginapi.Context) error { return nil }

// HandleKey 实现 pluginapi.KeyHandler：只记录，不处理（false 让链继续往下走）。
func (s *pageKeySpy) HandleKey(ev gfx.KeyEvent) bool {
	s.mu.Lock()
	s.seen = append(s.seen, ev)
	s.mu.Unlock()
	return false
}

func (s *pageKeySpy) keys() []gfx.KeyEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]gfx.KeyEvent(nil), s.seen...)
}

// shellPageKeyHarness 是一台真装配的壳：真 kernel（ds-core + ds-contextmenu + 探针）
// + 真 `newConfig`（被测的 cfg.OnKey）+ 真 headless 窗口过程（`EventResult.Activate`
// 由 painter 兑现成一次事件重画，声明的 Shortcut 那一句才返回 true）。
type shellPageKeyHarness struct {
	t       *testing.T
	model   *model
	session *headless.Session
	spy     *pageKeySpy
	menu    *dscontextmenu.Plugin
	theme   ui.Theme
	rowID   gfx.ID
	runs    []string // 菜单动作真的被执行了几次、哪几条
	wide    int      // 页面级 ui.Shortcut(c, 0, "f9") 被兑现的次数
}

func newShellPageKeyHarness(t *testing.T) *shellPageKeyHarness {
	t.Helper()
	h := &shellPageKeyHarness{t: t, spy: &pageKeySpy{}, menu: &dscontextmenu.Plugin{}}
	h.theme = dscore.VivaTheme(dscore.Dark())

	kernel := uikernel.New("shell-page-key")
	if err := kernel.Load(h.spy, &dscore.Plugin{Dark: true}, h.menu); err != nil {
		t.Fatalf("装配内核：%v", err)
	}
	// 动作只记录下来：「跑一次」还是「跑两次」是这群测试的判据之一。
	if _, err := h.menu.RegisterContribution(pluginapi.ContextMenuContribution{
		ID: "page-key.target", Target: "workspace", Order: 100,
		Resolve: func(pluginapi.ContextMenuTarget) []pluginapi.ContextMenuAction {
			return []pluginapi.ContextMenuAction{
				{ID: "one", Label: "第一条", Run: func() { h.runs = append(h.runs, "one") }},
				{ID: "two", Label: "第二条", Run: func() { h.runs = append(h.runs, "two") }},
			}
		},
	}); err != nil {
		t.Fatalf("登记菜单贡献：%v", err)
	}

	m := &model{kernel: kernel}
	h.model = m
	// 目标行命名在 `home`（插件给交互 id 用的那个作用域，见 ids.go 与 env.Feat）。
	h.rowID = tree.ID(home, "page-key-target")
	pageTheme := h.theme
	m.view = newWindowPage(&pageTheme, &tree)

	cfg := newConfig(m)
	cfg.Width, cfg.Height = 900, 700
	cfg.Paint = h.paint
	session, err := headless.Open(cfg, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless 打不开：%v", err)
	}
	t.Cleanup(session.Close)
	h.session = session
	h.pump()
	return h
}

// paint 是这一台壳的帧：一个铺满窗口的目标控件 + 一条页面级 Shortcut。
//
// 目标故意做成**叶子**控件（`ui.Slot`，没有子元素）：`ui.Box.Shortcut` 对叶子按
// 「焦点正在这个元素上」精确匹配（`viva@v0.6.4/ui/shortcut.go:72-73`），对容器按
// 「焦点在它的作用域里」覆盖（`:69-70` + `:135-140` 的 shortcutCovers）。本仓的交互
// id 全是 `env.Tree.ID(env.Feat, …)` 扁平命名，不落在任何容器作用域里。
//
// 于是「元素级声明落在容器上」这种形状（dscontextmenu 的两个真调用点都是——
// `dscore/primitives.go:101-110` 的 SectionHeader、`dsfiles/files.go:207-215` 的
// fileRow）页面路由**匹配不上**，只能继续靠插件自己的 `HandleKey` 兜：实测把这里
// 换成带子元素的 `ui.Row` 时，本测试的「探针收不到键」那条立刻红。这是插件命名
// 形状的问题，不是壳的驱动问题，所以这里用叶子形态钉壳的驱动链。
func (h *shellPageKeyHarness) paint(canvas *gfx.Canvas, size ui.Size) {
	bounds := gfx.Rect{Width: size.Width, Height: size.Height}
	canvas.Fill(bounds, h.theme.Background)
	h.model.view.Draw(canvas, bounds, func(f ui.C) {
		if ui.Shortcut(f, 0, "f9") {
			h.wide++
		}
		target := ui.Slot(f).FillWidth().FillHeight().
			HitWith(h.rowID, "工作区", gfx.HitOptions{Role: gfx.RoleButton})
		h.menu.Attach(f, h.rowID, target, pluginapi.ContextMenuTarget{Kind: "workspace", ID: "w-page-key"})
	})
}

func (h *shellPageKeyHarness) pump() {
	h.t.Helper()
	if err := h.session.Driver().Pump(); err != nil {
		h.t.Fatalf("pump 失败：%v", err)
	}
}

func (h *shellPageKeyHarness) keys(spec string) {
	h.t.Helper()
	if err := h.session.Keys(spec); err != nil {
		h.t.Fatalf("按键 %q 失败：%v", spec, err)
	}
	h.pump()
}

func (h *shellPageKeyHarness) click(x, y float32) {
	h.t.Helper()
	if err := h.session.Click(x, y); err != nil {
		h.t.Fatalf("点击 (%v,%v) 失败：%v", x, y, err)
	}
	h.pump()
}

func (h *shellPageKeyHarness) rightClick(x, y float32) {
	h.t.Helper()
	if err := h.session.RightClick(x, y); err != nil {
		h.t.Fatalf("右键 (%v,%v) 失败：%v", x, y, err)
	}
	h.pump()
}

func (h *shellPageKeyHarness) focused() gfx.ID { return h.session.Snapshot().Focused }

// menuItems 取这一帧画出来的菜单行（与 vr3 审计同一判据：RoleMenuItem）。
func (h *shellPageKeyHarness) menuItems() []gfx.Hit {
	out := make([]gfx.Hit, 0, 8)
	for _, hit := range h.session.Snapshot().Hits {
		if hit.Role == gfx.RoleMenuItem {
			out = append(out, hit)
		}
	}
	return out
}

// focusTarget 点中目标控件，把键盘焦点放到它身上：官方 Shift+F10 的形状就是
// 「先点中那一行，再按 Shift+F10」（`Box.Shortcut` 要求焦点在元素上或它里面）。
func (h *shellPageKeyHarness) focusTarget() {
	h.t.Helper()
	h.click(60, 20)
	if got := h.focused(); got != h.rowID {
		h.t.Fatalf("点击后焦点 = %v，want 目标控件 %v（测试支架失效：目标必须能拿焦点）", got, h.rowID)
	}
}

// openMenuByPointer 用右键打开菜单：先把按键从"打开菜单"里摘出去，后面那个键
// 的效果才只可能来自页面路由（或插件 HandleKey，而探针就是用来区分这两者的）。
func (h *shellPageKeyHarness) openMenuByPointer() {
	h.t.Helper()
	h.rightClick(60, 20)
	if items := h.menuItems(); len(items) == 0 {
		h.t.Fatal("右键没打开菜单，测试支架失效")
	}
}

// TestShellPageKeyDrivesElementShortcut 钉住：`Box.Shortcut(Shift,"f10")` 声明在
// 目标控件上时，按键真的被页面派发（菜单打开），而且这个键**没有再进插件链**。
//
// 变异（拿掉 `m.view.Key(ev)`）：dscontextmenu 自己的 `HandleKey` 兜底还能把菜单
// 打开（那是 vc-render3 的绕行路径），但探针会收到 shift+f10，第二条断言红。
func TestShellPageKeyDrivesElementShortcut(t *testing.T) {
	h := newShellPageKeyHarness(t)
	h.focusTarget()
	h.keys("shift+f10")
	if items := h.menuItems(); len(items) == 0 {
		t.Error("Shift+F10 没打开右键菜单：`target.Shortcut(gfx.ModifierShift,\"f10\")`" +
			"（plugins/dscontextmenu/context_menu.go:180）没被触发——壳的 cfg.OnKey 没把按键送进 ui.Page.Key")
	}
	if seen := h.spy.keys(); len(seen) != 0 {
		t.Errorf("Shift+F10 已由页面（Shortcut）接掉，却又进了 kernel.HandleKey：%v。"+
			"同一个键被处理两次（或页面路由根本没接上）", seen)
	}
}

// TestShellPageKeyDrivesContainerShortcut 钉住容器级 `menu.Shortcut(0,"down")`
// （`context_menu.go:321` 那一族）：菜单用指针打开，↓ 必须由页面派发——
// 焦点从第一行移到第二行，且这个键不进程探针。
func TestShellPageKeyDrivesContainerShortcut(t *testing.T) {
	h := newShellPageKeyHarness(t)
	h.openMenuByPointer()
	before := h.focused()
	h.keys("down")
	after := h.focused()
	if after == before || after.IsZero() {
		t.Errorf("↓ 之后焦点没动（%v → %v）：`menu.Shortcut(0,\"down\")`"+
			"（plugins/dscontextmenu/context_menu.go:321）没被触发", before, after)
	}
	if seen := h.spy.keys(); len(seen) != 0 {
		t.Errorf("↓ 已由页面（Shortcut）接掉，却又进了 kernel.HandleKey：%v", seen)
	}
}

// TestShellPageKeyDrivesEscapeShortcut 钉住容器级 `menu.Shortcut(0,"escape")`
// （`context_menu.go:317`）：菜单用指针打开，Esc 由页面派发关菜单，不进程探针。
func TestShellPageKeyDrivesEscapeShortcut(t *testing.T) {
	h := newShellPageKeyHarness(t)
	h.openMenuByPointer()
	h.keys("escape")
	if items := h.menuItems(); len(items) != 0 {
		t.Errorf("按 Esc 菜单没关：`menu.Shortcut(0,\"escape\")`" +
			"（plugins/dscontextmenu/context_menu.go:317）没被触发")
	}
	if seen := h.spy.keys(); len(seen) != 0 {
		t.Errorf("Esc 已由页面（Shortcut）接掉，却又进了 kernel.HandleKey：%v", seen)
	}
}

// TestShellKeyHandlerStillGetsKeysThePageDoesNotTake 钉住既有路径没被破坏：
// Enter 不是任何 Shortcut，必须原样落到 `pluginapi.KeyHandler`，动作只跑一次。
func TestShellKeyHandlerStillGetsKeysThePageDoesNotTake(t *testing.T) {
	h := newShellPageKeyHarness(t)
	h.openMenuByPointer()
	if len(h.runs) != 0 {
		t.Fatalf("还没按 Enter 就已经跑了动作：%v", h.runs)
	}
	h.keys("enter")
	if len(h.runs) != 1 {
		t.Errorf("Enter 之后菜单动作跑了 %d 次（%v），want 1："+
			"页面不认的键必须落到 pluginapi.KeyHandler，且只跑一次", len(h.runs), h.runs)
	}
	if seen := h.spy.keys(); len(seen) != 1 || seen[0].Key != gfx.KeyEnter {
		t.Errorf("Enter 应该原样进 kernel.HandleKey 一次，实际看到 %v", seen)
	}
}

// TestShellPageWideShortcut 钉住页面级 `ui.Shortcut(c,0,"f9")` 也生效。
//
// 这条钉的不是 Shortcut 派发（与元素级同一条路），而是 `PageConfig.Root`：
// 页面用 Root 判「这个 id 是不是我页面上的」（`ui/page.go:182-183,399-402`），
// 焦点落在插件命名的作用域（`home`）里；Root 一旦退回默认的 "page" 就永远不算，
// 页面级 Shortcut 会变成静默死代码。
func TestShellPageWideShortcut(t *testing.T) {
	h := newShellPageKeyHarness(t)
	h.focusTarget()
	h.keys("f9")
	if h.wide != 1 {
		t.Errorf("页面级 ui.Shortcut(c,0,\"f9\") 被兑现 %d 次，want 1："+
			"`PageConfig.Root` 必须指向插件命名交互 id 的作用域（pageRoot = home）", h.wide)
	}
	if seen := h.spy.keys(); len(seen) != 0 {
		t.Errorf("F9 已由页面接掉，却又进了 kernel.HandleKey：%v", seen)
	}
}
