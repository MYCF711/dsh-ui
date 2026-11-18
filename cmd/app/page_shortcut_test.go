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
	// container 让目标按**真实调用点的形状**摆：`ui.Row` + 子元素 + `.Hit(id)`
	// （`dscore/primitives.go:99-111` 的 SectionHeader、`dsfiles/files.go:161-172`
	// 的 fileRow 都是这个形状），而不是叶子 `ui.Slot`。
	container bool
}

func newShellPageKeyHarness(t *testing.T) *shellPageKeyHarness {
	t.Helper()
	return newShellPageKeyHarnessShaped(t, false)
}

// newShellPageKeyHarnessShaped 同一台壳，只换目标的形状（叶子 / 真实容器）。
func newShellPageKeyHarnessShaped(t *testing.T, container bool) *shellPageKeyHarness {
	t.Helper()
	h := &shellPageKeyHarness{t: t, spy: &pageKeySpy{}, menu: &dscontextmenu.Plugin{}, container: container}
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
		var target ui.Box
		if h.container {
			// 真实调用点的形状：容器 + 子元素 + `.Hit(id)`。子元素故意不带交互 id
			// （`ui.Row`/`ui.Text` 由 Viva 按位置命名），这正是"容器的作用域覆盖不到
			// 扁平命名 id"的那种形状。
			target = ui.Row(f).FillWidth().FillHeight().HitWith(h.rowID, "工作区", gfx.HitOptions{Role: gfx.RoleButton}).
				Children(func() {
					ui.Row(f).Width(16).Height(20).Children(func() {
						ui.Text(f, "▸").FontSize(13)
					})
					ui.Text(f, "工作区").FontSize(13).Grow(1).Ellipsis()
				})
		} else {
			target = ui.Slot(f).FillWidth().FillHeight().
				HitWith(h.rowID, "工作区", gfx.HitOptions{Role: gfx.RoleButton})
		}
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

// ── dscontextmenu 的真形状：容器目标 + 官方键位 ───────────────────────────────
//
// 上面那台 harness 用叶子 `ui.Slot`（`Box.Shortcut` 的元素级精确匹配）。插件的两个
// 真实调用点却是容器（`dscore/primitives.go:99-111`、`dsfiles/files.go:161-172`），
// 而 `Box.Shortcut` 对容器按作用域覆盖匹配，本仓的扁平 id 永远覆盖不到。下面这台
// harness 就按真实形状摆，钉住**插件改走页面路由之后**每个键只处理一次。

// takeKeys 取走探针到目前为止看到的键（清空，便于按阶段判）。
func (h *shellPageKeyHarness) takeKeys() []gfx.KeyEvent {
	seen := h.spy.keys()
	h.spy.mu.Lock()
	h.spy.seen = nil
	h.spy.mu.Unlock()
	return seen
}

// itemIDs 取这一帧菜单行的 id（按画面顺序）。
func (h *shellPageKeyHarness) itemIDs() []gfx.ID {
	items := h.menuItems()
	out := make([]gfx.ID, len(items))
	for i, item := range items {
		out[i] = item.ID
	}
	return out
}

// TestShellContextMenuShiftF10OnContainerTarget 钉住：真实形状（容器 + 扁平 id）下，
// Shift+F10 仍然由页面路由打开菜单。
//
// 官方依据：`trigger.ts:29-37` 把 Shift+F10 挂在**有焦点的触发器元素**上；
// viva 的 `ui.ContextMenu` 是同一条规则（`ui/menu.go:57-59,631-640`，判据是
// `target.id == event.Target`），容器只要 `Hit` 过就算数（`ui/menu.go:75`）。
//
// 变异：把 `Attach` 里的 `ui.Shortcut(c, ui.Shift, "f10")` 换回
// `target.Shortcut(gfx.ModifierShift, "f10")` → 本测试红（容器分支匹配不到），
// 而插件已经不再有 `HandleKey` 兜底。
func TestShellContextMenuShiftF10OnContainerTarget(t *testing.T) {
	h := newShellPageKeyHarnessShaped(t, true)
	h.focusTarget()
	h.keys("shift+f10")
	if items := h.menuItems(); len(items) == 0 {
		t.Error("真实形状（容器目标）下 Shift+F10 没打开菜单：页面对容器级 `Box.Shortcut` " +
			"匹配不上（`ui/shortcut.go:69-71,120-123,162-165`），必须由 `Attach` 里的页面级 " +
			"`ui.Shortcut(c, ui.Shift, \"f10\")` 派发")
	}
	if seen := h.spy.keys(); len(seen) != 0 {
		t.Errorf("Shift+F10 由页面接掉后又进了 kernel.HandleKey：%v（同一个键走两条路）", seen)
	}
}

// TestShellContextMenuKeysHandledOnce 钉住**一个键只处理一次**：每个键按一次，
// 只发生一次状态转移（↓ 只走一格、Enter/Space 只跑一次动作、Esc 只关一次）。
//
// 为什么这样判：双处理在菜单上表现为"一次按键走两格 / 动作跑两次"。所以 ↓ 一次的
// 判据是「焦点正好落在第 2 行」，而不是"焦点变了"——后者两份处理都满足。
//
// 按键归属（每条都带探针判据，防的正是同一个键被两条链各处理一次）：
//
//	Shift+F10 / Esc / ↑↓ Home End → 页面路由（插件声明的 `Shortcut`），
//	                               **一个都不进程探针**；
//	Enter / Space                → 页面不认，落在 Viva 的输入路由器上
//	                               （`internal/input/input.go:308-315`）变成一次点击，
//	                               由 painter 兑现（`app/painter.go:554`）——
//	                               插件不再自己跑一遍，`Run` 只被调一次。
func TestShellContextMenuKeysHandledOnce(t *testing.T) {
	h := newShellPageKeyHarnessShaped(t, true)
	h.focusTarget()
	h.keys("shift+f10")
	h.takeKeys()
	ids := h.itemIDs()
	if len(ids) != 2 {
		t.Fatalf("菜单行数 = %d，want 2（测试支架失效）", len(ids))
	}
	if got := h.focused(); got != ids[0] {
		t.Fatalf("菜单打开后焦点 = %v，want 第一行 %v", got, ids[0])
	}

	// ↓ 一次 = 走一格。双处理会一次走两格（或原地来回）。
	h.keys("down")
	if got := h.focused(); got != ids[1] {
		t.Errorf("↓ 按一次后焦点 = %v，want 第 2 行 %v：一个键被处理了不止一次", got, ids[1])
	}
	h.keys("up")
	if got := h.focused(); got != ids[0] {
		t.Errorf("↑ 按一次后焦点 = %v，want 第 1 行 %v：一个键被处理了不止一次", got, ids[0])
	}
	h.keys("end")
	if got := h.focused(); got != ids[1] {
		t.Errorf("End 按一次后焦点 = %v，want 末行 %v", got, ids[1])
	}
	h.keys("home")
	if got := h.focused(); got != ids[0] {
		t.Errorf("Home 按一次后焦点 = %v，want 首行 %v", got, ids[0])
	}
	if seen := h.takeKeys(); len(seen) != 0 {
		t.Errorf("↑↓/Home/End 由页面接掉后仍进了 kernel.HandleKey：%v（同一个键走两条路）", seen)
	}

	// Enter：页面不认 → 插件链看到一次（没人处理）→ Viva 兑现一次点击。
	h.keys("enter")
	if seen := h.takeKeys(); len(seen) != 1 || seen[0].Key != gfx.KeyEnter {
		t.Errorf("Enter 应该原样进 kernel.HandleKey 一次（Viva 的原生激活靠这条），实际 %v", seen)
	}
	if len(h.runs) != 1 || h.runs[0] != "one" {
		t.Errorf("Enter 之后动作跑了 %d 次（%v），want 1：`RoleMenuItem` 上的 Enter 由 Viva 兑现一次点击", len(h.runs), h.runs)
	}
	if items := h.menuItems(); len(items) != 0 {
		t.Error("Enter 选了菜单项之后菜单没关（官方 `onSelect` 之后关闭）")
	}

	// Space 同理（同一段路由器把 Space 也算作激活）。
	h.runs = nil
	h.keys("shift+f10")
	h.takeKeys()
	h.keys("space")
	if seen := h.takeKeys(); len(seen) != 1 || seen[0].Key != gfx.KeySpace {
		t.Errorf("Space 应该原样进 kernel.HandleKey 一次，实际 %v", seen)
	}
	if len(h.runs) != 1 {
		t.Errorf("Space 之后动作跑了 %d 次（%v），want 1", len(h.runs), h.runs)
	}

	// Esc 由页面接掉；菜单关掉之后同一个键回到插件链（菜单关着就没有这条快捷键）。
	h.keys("shift+f10")
	h.takeKeys()
	h.keys("escape")
	if items := h.menuItems(); len(items) != 0 {
		t.Error("Esc 之后菜单没关")
	}
	if seen := h.takeKeys(); len(seen) != 0 {
		t.Errorf("Esc 由页面接掉后仍进了 kernel.HandleKey：%v", seen)
	}
	h.keys("escape")
	if seen := h.takeKeys(); len(seen) != 1 || seen[0].Key != gfx.KeyEscape {
		t.Errorf("菜单已经关掉，第二个 Esc 必须原样进 kernel.HandleKey 一次，实际 %v", seen)
	}
}

// TestShellContextMenuClosesLeavesKeysToPlugins 钉住：菜单没开时，这些键**不属于**
// 插件——容器级 `menu.Shortcut(...)` 只在 `drawMenu` 里声明，菜单关着就不存在，
// 所以 Esc / ↓ / Enter 必须原样进 `kernel.HandleKey`（否则菜单会偷走别的插件的键）。
func TestShellContextMenuClosesLeavesKeysToPlugins(t *testing.T) {
	h := newShellPageKeyHarnessShaped(t, true)
	h.focusTarget()
	for _, key := range []string{"escape", "down", "up", "home", "end"} {
		h.keys(key)
	}
	seen := h.spy.keys()
	if len(seen) != 5 {
		t.Fatalf("菜单没开时按 5 个键，插件链看到 %d 个（%v）：菜单关着就不该声明这些快捷键", len(seen), seen)
	}
	if items := h.menuItems(); len(items) != 0 {
		t.Error("菜单没开，按键却把菜单打开了")
	}
}
