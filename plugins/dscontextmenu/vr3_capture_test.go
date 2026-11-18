//go:build darwin || windows || linux

// vr3_capture_test.go —— 第三轮真实渲染审计：上下文菜单的**空态 / 项多时越界 /
// 窄窗边缘 / disabled / 键盘高亮 / 分隔线**。
//
// 这一轮盯的官方值全部取自 rc.2 asar（原件未压缩，字节偏移写在各条断言里）：
//
//	primitives Menu（`.mOxcpG`/`_70kcw_` 那一族，asar 偏移 **123324111**）：
//	  `._list_70kcw_7{box-sizing:border-box;padding:4px;…box-shadow:elevation-prominent}`
//	  `._list_70kcw_7{min-width:min(144px,calc(100vw - 24px));max-width:min(360px,calc(100vw - 24px))}`
//	  `._list_70kcw_7[data-context-menu]:not(:has([role=menuitem])){display:none}`
//	  `._scrollable_70kcw_19{max-height:calc(100dvh - 12px - max(12px,var(--dsh-frame-overlay-top,12px)))}`
//	  `._viewport_70kcw_19{display:flex;flex-direction:column;min-height:0}` +
//	  `._scrollable_70kcw_19 ._viewport_70kcw_19{overflow-y:auto}`
//	  `._item_70kcw_173{…min-height:34px;padding:6px 8px;gap:6px;border-radius:radius-md;
//	                    background:transparent;font-size:13px;line-height:20px}`
//	  `._item_70kcw_173:hover:not(:disabled):not([aria-disabled=true]){background:interactive-bg-hover}`
//	  `._item_70kcw_173:focus-visible:not(:disabled){background:interactive-bg-hover;outline:none}`
//	  `._item_70kcw_173:disabled,._item_70kcw_173[aria-disabled=true]{opacity:.4;cursor:not-allowed}`
//
// 截图落 `$VR3_CAPTURE_DIR`（未设置时用临时目录）。
package dscontextmenu

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func vr3CaptureDir(t *testing.T) string {
	t.Helper()
	base := os.Getenv("VR3_CAPTURE_DIR")
	if base == "" {
		return t.TempDir()
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	return base
}

// vr3Menu 是一台装配好的真窗口：dscore + 本插件，页面上放一行可右键的目标。
type vr3Menu struct {
	t       *testing.T
	session *headless.Session
	plugin  *Plugin
	dir     string
	width   int
	height  int
	runs    []string
}

func vr3OpenMenu(t *testing.T, dir string, width, height int, actions []pluginapi.ContextMenuAction) *vr3Menu {
	t.Helper()
	m := &vr3Menu{t: t, dir: dir, width: width, height: height}
	p := newTestPlugin()
	if _, err := p.RegisterContribution(pluginapi.ContextMenuContribution{
		ID: "vr3.target", Target: "workspace", Order: 100,
		Resolve: func(pluginapi.ContextMenuTarget) []pluginapi.ContextMenuAction {
			// 每次 resolve 都拷一份，把 Run 记到测试上（动作真的被点了吗）。
			out := make([]pluginapi.ContextMenuAction, 0, len(actions))
			for _, action := range actions {
				copied := action
				id := action.ID
				copied.Run = func() { m.runs = append(m.runs, id) }
				out = append(out, copied)
			}
			return out
		},
	}); err != nil {
		t.Fatal(err)
	}
	m.plugin = p
	tree := &gfx.Tree{}
	feature := tree.Feature("context-menu-vr3")
	rowID := tree.ID(feature, "vr3-row")
	target := pluginapi.ContextMenuTarget{Kind: "workspace", ID: "w-vr3"}
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{
		Title: "context-menu-vr3", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, p.theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			// 目标行铺满窗口：右键落点决定菜单位置（官方 `data-context-menu` 也是
			// 任意落点），所以"贴底右键"必须点得中这个目标。
			row := ui.Row(f).FillWidth().FillHeight().Hit(rowID, "工作区")
			p.Attach(f, rowID, row, target)
		})
	}
	config.OnScroll = view.Wheel
	// 按键接线与壳（`cmd/app/main.go` 的 `cfg.OnKey`）同序：**先页面，再插件**。
	// 壳是 `Page.Draw` 形态，viva 要求这种形态由宿主自己用 `Page.Key` 路由输入
	// （`viva@v0.6.4/ui/page.go:716-721`），并且排在宿主自己的按键处理之前
	// （`ui/page.go:1088-1093`）；`ui.Box.Shortcut` 只由那条路由派发
	// （`ui/shortcut.go:94-135`）。页面接掉的键不再交给插件（一个键只走一条路）。
	config.OnKey = func(ev gfx.KeyEvent) gfx.EventResult {
		if ev.Release {
			return gfx.EventResult{}
		}
		if result := view.Key(ev); result.Handled || !result.Activate.IsZero() {
			return result
		}
		if p.HandleKey(ev) {
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		return gfx.EventResult{}
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

func (m *vr3Menu) pump() {
	m.t.Helper()
	if err := m.session.Driver().Pump(); err != nil {
		m.t.Fatal(err)
	}
}

func (m *vr3Menu) shot(name string) *image.RGBA {
	m.t.Helper()
	path := filepath.Join(m.dir, name)
	if err := m.session.Screenshot(path); err != nil {
		m.t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		m.t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	img, err := m.session.Image()
	if err != nil {
		m.t.Fatal(err)
	}
	m.t.Logf("VR3_CAPTURE path=%s size=%v", path, img.Bounds())
	return img
}

func vr3PixelDiff(a, b *image.RGBA) int {
	if a.Bounds() != b.Bounds() {
		return -1
	}
	n := 0
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				n++
			}
		}
	}
	return n
}

// vr3Surface 找菜单面：颜色 = 菜单材质、圆角 16 的那块矩形（`_surface_1w4el_1` 的 radius-lg）。
func vr3Surface(snapshot app.Snapshot, theme *pluginapi.Theme) (app.Primitive, bool) {
	best := app.Primitive{}
	found := false
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind != app.RectanglePrimitive || primitive.Radius != menuRadius {
			continue
		}
		if primitive.Color != theme.Menu {
			continue
		}
		if !found || primitive.Bounds.Width*primitive.Bounds.Height > best.Bounds.Width*best.Bounds.Height {
			best, found = primitive, true
		}
	}
	return best, found
}

// vr3MenuItems 收集这一帧的菜单行命中区（Role = menuitem）。
func vr3MenuItems(snapshot app.Snapshot) []gfx.Hit {
	out := make([]gfx.Hit, 0, 8)
	for _, hit := range snapshot.Hits {
		if hit.Role == gfx.RoleMenuItem {
			out = append(out, hit)
		}
	}
	return out
}

// vr3Actions 造 n 条普通菜单项。
func vr3Actions(n int) []pluginapi.ContextMenuAction {
	out := make([]pluginapi.ContextMenuAction, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, pluginapi.ContextMenuAction{
			ID:    "action-" + string(rune('a'+i-1)),
			Label: "动作" + string(rune('A'+i-1)),
		})
	}
	return out
}

// ── ① 菜单几何（落点 + padding 4 + 项高 34 + 宽 min144/max360）─────────────────

// TestVR3MenuGeometryAtPointer 右键落点 + 官方 padding/项高/宽度夹取。
func TestVR3MenuGeometryAtPointer(t *testing.T) {
	dir := vr3CaptureDir(t)
	m := vr3OpenMenu(t, dir, 900, 700, vr3Actions(3))
	const px, py = 40, 20
	if err := m.session.RightClick(px, py); err != nil {
		t.Fatal(err)
	}
	m.pump()
	m.shot("context-menu-geometry.png")

	theme := m.plugin.theme
	surfacePrim, ok := vr3Surface(m.session.Snapshot(), theme)
	if !ok {
		t.Fatalf("帧里没有菜单面（radius=%v、color=Menu）：primitives=%d",
			menuRadius, len(m.session.Snapshot().Primitives))
	}
	surface := surfacePrim.Bounds
	// 官方 `._list_70kcw_7{position:absolute;padding:4px}` 贴着落点摆（padding 是**内**边距），
	// 所以菜单面左上 = 落点，菜单行左上 = 落点 + 4。
	if want := float32(px); surface.X != want {
		t.Errorf("菜单面 x=%v，官方彻底贴落点 = %v（`.mOxcpG`/`_70kcw_` 的 padding 是内边距）", surface.X, want)
	}
	if want := float32(py); surface.Y != want {
		t.Errorf("菜单面 y=%v，官方贴落点 = %v", surface.Y, want)
	}
	if surfacePrim.Radius != menuRadius {
		t.Errorf("菜单面圆角=%v，官方 `_surface_1w4el_1{border-radius:radius-lg}`=%v", surfacePrim.Radius, menuRadius)
	}
	// 宽度：`min-width:min(144px,100vw-24px);max-width:min(360px,100vw-24px)`。
	if surface.Width < menuMinWidth-1 || surface.Width > menuMaxWidth+1 {
		t.Errorf("菜单宽=%v，官方夹在 [min(144,100vw-24), min(360,100vw-24)] = [%v,%v]",
			surface.Width, menuMinWidth, menuMaxWidth)
	}
	items := vr3MenuItems(m.session.Snapshot())
	if len(items) != 3 {
		t.Fatalf("菜单行=%d，want 3", len(items))
	}
	for i, item := range items {
		if item.Bounds.Height != menuItemHeight {
			t.Errorf("第 %d 行高=%v，官方 `._item_70kcw_173{min-height:34px}`", i, item.Bounds.Height)
		}
		wantX := surface.X + menuPadding
		if d := item.Bounds.X - wantX; d > 1 || d < -1 {
			t.Errorf("第 %d 行 x=%v，官方 padding:4px 之后应为 %v", i, item.Bounds.X, wantX)
		}
		wantY := surface.Y + menuPadding + float32(i)*menuItemHeight
		if d := item.Bounds.Y - wantY; d > 1 || d < -1 {
			t.Errorf("第 %d 行 y=%v，want %v（上一行 34 高紧排）", i, item.Bounds.Y, wantY)
		}
		if item.Bounds.Width != surface.Width-2*menuPadding {
			t.Errorf("第 %d 行宽=%v，want 菜单宽-%v=%v（左右各 4px padding）",
				i, item.Bounds.Width, 2*menuPadding, surface.Width-2*menuPadding)
		}
	}
}

// ── ② 项多 + 贴底右键：不许越界，且菜单自己滚 ────────────────────────────────

// TestVR3MenuManyItemsStayInsideWindow 20 项（20×34+8=688 高）在 900×700 窗口里贴底右键：
// 官方两条约束——`._scrollable_70kcw_19{max-height:calc(100dvh - 12px - max(12px,overlayTop))}`
// 与 `._viewport_70kcw_19{overflow-y:auto}`——菜单必须整块留在窗口内并自己滚。
func TestVR3MenuManyItemsStayInsideWindow(t *testing.T) {
	dir := vr3CaptureDir(t)
	const w, h = 900, 700
	// 40 项 × 34 + 8 = 1368px，远超 `calc(100dvh - 24px)` = 676 → 必须靠内部滚动。
	m := vr3OpenMenu(t, dir, w, h, vr3Actions(40))
	if err := m.session.RightClick(430, 650); err != nil {
		t.Fatal(err)
	}
	m.pump()
	first := m.shot("context-menu-many-items.png")

	surfacePrim, ok := vr3Surface(m.session.Snapshot(), m.plugin.theme)
	if !ok {
		t.Fatal("帧里没有菜单面")
	}
	surface := surfacePrim.Bounds
	if maxH := float32(h - 2*menuViewportMargin); surface.Height > maxH+1 {
		t.Errorf("菜单高=%v，官方 `._scrollable_70kcw_19{max-height:calc(100dvh - 12px - 12px)}` 上界 %v",
			surface.Height, maxH)
	}
	if surface.Y < menuViewportMargin-1 || surface.Y+surface.Height > float32(h)-menuViewportMargin+1 {
		t.Errorf("菜单越出窗口：%+v（窗口 %dx%d，官方浮层留 %v 边距）",
			surface, w, h, menuViewportMargin)
	}
	if surface.X < menuViewportMargin-1 || surface.X+surface.Width > float32(w)-menuViewportMargin+1 {
		t.Errorf("菜单横向越界：%+v", surface)
	}
	// 每一行的命中区都不许出菜单面。
	for _, item := range vr3MenuItems(m.session.Snapshot()) {
		b := item.Bounds
		if b.Y < surface.Y-1 || b.Y+b.Height > surface.Y+surface.Height+1 {
			t.Errorf("菜单行越出菜单面：label=%q bounds=%+v surface=%+v", item.Label, b, surface)
			break
		}
	}
	// 菜单自己要能滚：滚轮往下，末尾那条要能进视口。
	lastDrawn := func() string {
		value := ""
		for _, label := range m.session.Snapshot().Labels {
			if len(label.Value) > 6 && label.Value[:6] == "动作" {
				value = label.Value
			}
		}
		return value
	}
	before := lastDrawn()
	for range 20 {
		if err := m.session.Scroll(gfx.ScrollEvent{
			X: surface.X + surface.Width/2, Y: surface.Y + surface.Height/2,
			DeltaY: 200, Precise: true,
		}); err != nil {
			t.Fatalf("菜单内滚轮失败：%v", err)
		}
		m.pump()
	}
	after := m.shot("context-menu-many-items-bottom.png")
	if lastDrawn() == before {
		t.Errorf("菜单内滚了 20×200 仍停在 %q——`._viewport_70kcw_19{overflow-y:auto}` 没生效",
			before)
	}
	if diff := vr3PixelDiff(first, after); diff == 0 {
		t.Errorf("菜单内滚动后画面零像素变化")
	}
	t.Logf("菜单面=%+v，首帧末尾行=%q，滚到底末尾行=%q", surface, before, lastDrawn())
}

// ── ③ 空菜单：官方 `:not(:has([role=menuitem])){display:none}` ──────────────

// TestVR3EmptyResolutionDrawsNothing 贡献解析出 0 条动作时：
// 官方用 `._list_70kcw_7[data-context-menu]:not(:has([role=menuitem])){display:none}`
// 把整个菜单面藏掉（rc.2 asar 偏移 123324111），本仓也不许画一个空壳。
func TestVR3EmptyResolutionDrawsNothing(t *testing.T) {
	dir := vr3CaptureDir(t)
	m := vr3OpenMenu(t, dir, 900, 700, nil) // 0 条动作
	baseline := m.shot("context-menu-empty-before.png")
	if err := m.session.RightClick(40, 20); err != nil {
		t.Fatal(err)
	}
	m.pump()
	after := m.shot("context-menu-empty.png")

	if _, ok := vr3Surface(m.session.Snapshot(), m.plugin.theme); ok {
		t.Errorf("0 条动作时仍画出了菜单面（官方 :not(:has([role=menuitem])){display:none}）")
	}
	if items := vr3MenuItems(m.session.Snapshot()); len(items) != 0 {
		t.Errorf("0 条动作时出现 %d 个菜单行命中区", len(items))
	}
	for _, text := range m.session.Texts() {
		if text != "工作区" {
			t.Errorf("空菜单画面里出现了文字 %q（不该有任何菜单文案）", text)
		}
	}
	if diff := vr3PixelDiff(baseline, after); diff != 0 {
		t.Errorf("空菜单右键前后画面差 %d 像素（应该什么都不弹）", diff)
	}
}

// ── ④ disabled：opacity .4 + 不响应 hover/点击 ──────────────────────────────

// TestVR3DisabledItemDimsAndIgnoresPointer 官方
// `._item_70kcw_173:disabled,._item_70kcw_173[aria-disabled=true]{opacity:.4;cursor:not-allowed}`
// 与 `:hover:not(:disabled)`。
func TestVR3DisabledItemDimsAndIgnoresPointer(t *testing.T) {
	dir := vr3CaptureDir(t)
	actions := []pluginapi.ContextMenuAction{
		{ID: "enabled", Label: "可用动作"},
		{ID: "disabled", Label: "不可用动作", Disabled: true},
	}
	m := vr3OpenMenu(t, dir, 900, 700, actions)
	theme := m.plugin.theme
	if err := m.session.RightClick(40, 20); err != nil {
		t.Fatal(err)
	}
	m.pump()

	items := vr3MenuItems(m.session.Snapshot())
	if len(items) != 2 {
		t.Fatalf("菜单行=%d，want 2", len(items))
	}
	var disabled gfx.Hit
	for _, item := range items {
		if item.Label == "不可用动作" {
			disabled = item
		}
	}
	if disabled.ID.IsZero() {
		t.Fatalf("找不到 disabled 行：%+v", items)
	}
	if !disabled.Disabled {
		t.Errorf("禁止行的命中区没有标 Disabled")
	}
	// 文字 alpha = label-primary × 0.4（官方 opacity:.4）。
	var text *gfx.Text
	for _, one := range m.session.Snapshot().Texts {
		if one.Value == "不可用动作" {
			copied := one
			text = &copied
			break
		}
	}
	if text == nil {
		t.Fatalf("禁止行的文字没画出来：texts=%q", m.session.Texts())
	}
	want := theme.LabelPrimary.Alpha * menuDisabledOpacity
	if d := text.Style.Color.Alpha - want; d > 0.02 || d < -0.02 {
		t.Errorf("禁止行文字 alpha=%v，官方 opacity:.4 × label-primary(%v) = %v",
			text.Style.Color.Alpha, theme.LabelPrimary.Alpha, want)
	}

	// hover 禁止行：官方 `:hover:not(:disabled)` → 不许出现 hover 底。
	before := m.shot("context-menu-disabled-idle.png")
	if err := m.session.Move(disabled.Bounds.X+disabled.Bounds.Width/2,
		disabled.Bounds.Y+disabled.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	m.pump()
	after := m.shot("context-menu-disabled-hover.png")
	hoverRects := 0
	for _, primitive := range m.session.Snapshot().Primitives {
		if primitive.Kind != app.RectanglePrimitive || primitive.Color != theme.Hover {
			continue
		}
		b := primitive.Bounds
		if b.X >= disabled.Bounds.X-1 && b.Y >= disabled.Bounds.Y-1 &&
			b.Y+b.Height <= disabled.Bounds.Y+disabled.Bounds.Height+1 {
			hoverRects++
		}
	}
	if hoverRects != 0 {
		t.Errorf("hover 禁止行出现了 %d 块 hover 底色（官方 :hover:not(:disabled)）", hoverRects)
	}
	if diff := vr3PixelDiff(before, after); diff != 0 {
		t.Errorf("指针移到禁止行上画面差 %d 像素（官方禁止行不响应 hover）", diff)
	}

	// 点禁止行：动作不许执行、菜单也不许关。
	if err := m.session.Click(disabled.Bounds.X+disabled.Bounds.Width/2,
		disabled.Bounds.Y+disabled.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	m.pump()
	if len(m.runs) != 0 {
		t.Errorf("点禁止行执行了动作 %v（官方 disabled 行不跑 Run）", m.runs)
	}
	if len(vr3MenuItems(m.session.Snapshot())) == 0 {
		t.Errorf("点禁止行把菜单关掉了（官方不关）")
	}
	// 再点可用行：这次要真的执行并关菜单。
	if err := m.session.Click(items[0].Bounds.X+items[0].Bounds.Width/2,
		items[0].Bounds.Y+items[0].Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	m.pump()
	if len(m.runs) != 1 || m.runs[0] != "enabled" {
		t.Errorf("点可用行动作执行=%v，want [enabled]", m.runs)
	}
}

// ── ⑤ 键盘高亮：官方 focus-visible 也给 hover 底 ─────────────────────────────

// TestVR3KeyboardFocusHighlightsItem 菜单打开时插件自己 `RequestFocus` 到第一行；
// 官方 `._item_70kcw_173:focus-visible:not(:disabled){background:interactive-bg-hover;outline:none}`
// 要求**键盘焦点那一行有 hover 底色**（这是菜单唯一的键盘位置反馈），并且 ↓ 要换行。
func TestVR3KeyboardFocusHighlightsItem(t *testing.T) {
	dir := vr3CaptureDir(t)
	m := vr3OpenMenu(t, dir, 900, 700, vr3Actions(3))
	theme := m.plugin.theme
	if err := m.session.RightClick(40, 20); err != nil {
		t.Fatal(err)
	}
	m.pump()
	items := vr3MenuItems(m.session.Snapshot())
	if len(items) != 3 {
		t.Fatalf("菜单行=%d，want 3", len(items))
	}
	focused := m.session.Snapshot().Focused
	idx := -1
	for i, item := range items {
		if item.ID == focused {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatalf("菜单打开后焦点没有落到任何菜单行（focused=%v）", focused)
	}
	m.shot("context-menu-keyboard-first.png")
	background := func(rect gfx.Rect) int {
		count := 0
		for _, primitive := range m.session.Snapshot().Primitives {
			if primitive.Kind != app.RectanglePrimitive || primitive.Color != theme.Hover {
				continue
			}
			b := primitive.Bounds
			if b.X >= rect.X-1 && b.X+b.Width <= rect.X+rect.Width+1 &&
				b.Y >= rect.Y-1 && b.Y+b.Height <= rect.Y+rect.Height+1 {
				count++
			}
		}
		return count
	}
	if background(items[idx].Bounds) == 0 {
		t.Errorf("键盘焦点在第 %d 行（%q）却没有 hover 底色："+
			"官方 `._item_70kcw_173:focus-visible:not(:disabled){background:var(--dsw-alias-interactive-bg-hover)}`"+
			"（rc.2 asar 偏移 123324111）没有落到画面上", idx, items[idx].Label)
	}

	// ↓ 换行：焦点必须真的移动，而且新行要有底色。
	if err := m.session.Keys("down"); err != nil {
		t.Fatalf("↓ 失败：%v", err)
	}
	m.pump()
	m.shot("context-menu-keyboard-second.png")
	nextFocused := m.session.Snapshot().Focused
	if nextFocused == focused {
		t.Fatalf("按 ↓ 焦点没动（还是 %v）", focused)
	}
	nextItems := vr3MenuItems(m.session.Snapshot())
	nextIdx := -1
	for i, item := range nextItems {
		if item.ID == nextFocused {
			nextIdx = i
		}
	}
	if nextIdx < 0 {
		t.Fatalf("↓ 之后焦点落到了菜单行之外：%v", nextFocused)
	}
	if background(nextItems[nextIdx].Bounds) == 0 {
		t.Errorf("↓ 之后焦点在第 %d 行（%q）仍无 hover 底色", nextIdx, nextItems[nextIdx].Label)
	}

	// Esc 必须关菜单（官方菜单的 Escape 收起）。插件用 `menu.Shortcut(0,"escape")` 判，
	// 而壳只把按键交给 `kernel.HandleKey`（`cmd/app/main.go:2237-2265`），全仓没有把按键
	// 送进 `ui.Page.Key`——所以这一条会连带失效。
	if err := m.session.Keys("escape"); err != nil {
		t.Fatalf("Esc 失败：%v", err)
	}
	m.pump()
	if len(vr3MenuItems(m.session.Snapshot())) != 0 {
		t.Errorf("按 Esc 菜单没关：`menu.Shortcut(0,\"escape\")`（context_menu.go:292）在真装配下不触发")
	}
}

// TestVR3KeyboardOpenShortcut 插件声明了 Shift+F10 打开菜单
// （`context_menu.go:165` 的 `target.Shortcut(gfx.ModifierShift, "f10")`）。
// 官方在 Windows 上就是这条键盘入口；它和 ↑↓/Esc 是同一个判据族（`ui.Box.Shortcut`），
// 所以同一根因失效。指针停在目标行上、不按右键，只按 Shift+F10。
func TestVR3KeyboardOpenShortcut(t *testing.T) {
	dir := vr3CaptureDir(t)
	m := vr3OpenMenu(t, dir, 900, 700, vr3Actions(2))
	// 先把焦点落到目标行上（官方 `Box.Shortcut` 要求"焦点在它或它里面"；
	// 真机上的形状就是先点中那一行、再按 Shift+F10）。
	if err := m.session.Click(60, 20); err != nil {
		t.Fatal(err)
	}
	m.pump()
	if err := m.session.Move(60, 20); err != nil {
		t.Fatal(err)
	}
	m.pump()
	if err := m.session.Keys("shift+f10"); err != nil {
		t.Fatalf("Shift+F10 失败：%v", err)
	}
	m.pump()
	m.shot("context-menu-shift-f10.png")
	if items := vr3MenuItems(m.session.Snapshot()); len(items) == 0 {
		t.Errorf("按 Shift+F10 没打开菜单：`target.Shortcut(gfx.ModifierShift,\"f10\")`" +
			"（context_menu.go:165）在真装配下不触发——壳只把按键交给 `kernel.HandleKey`，" +
			"全仓没有任何地方调用 `ui.Page.Key`（`cmd/app/main.go:2237-2265`）")
	}
}

// ── ⑥ 分隔线：0.5px + 3/8 外边距 ────────────────────────────────────────────

// TestVR3SeparatorGeometry 官方 `_separator_70kcw_157`：0.5px 高、margin 3px 8px
// （插件常量已登记在 context_menu.go:44-46，这里验**画出来的高度与外距**）。
func TestVR3SeparatorGeometry(t *testing.T) {
	dir := vr3CaptureDir(t)
	actions := []pluginapi.ContextMenuAction{
		{ID: "one", Label: "第一条"},
		{ID: "two", Label: "第二条", SeparatorBefore: true},
	}
	m := vr3OpenMenu(t, dir, 900, 700, actions)
	theme := m.plugin.theme
	if err := m.session.RightClick(40, 20); err != nil {
		t.Fatal(err)
	}
	m.pump()
	m.shot("context-menu-separator.png")

	items := vr3MenuItems(m.session.Snapshot())
	if len(items) != 2 {
		t.Fatalf("菜单行=%d，want 2", len(items))
	}
	surfacePrim, ok := vr3Surface(m.session.Snapshot(), theme)
	if !ok {
		t.Fatal("帧里没有菜单面")
	}
	surface := surfacePrim.Bounds
	var line *app.Primitive
	for _, primitive := range m.session.Snapshot().Primitives {
		if primitive.Kind != app.RectanglePrimitive || primitive.Color != theme.BorderL2 {
			continue
		}
		if primitive.Bounds.Height > 1 {
			continue
		}
		copied := primitive
		line = &copied
		break
	}
	if line == nil {
		t.Fatalf("没有画出分隔线（0.5px、border-l2 的那一条）")
	}
	// 0.5px 在 scale=1 的画布上会被像素栅格吸附成 1（`gfx/rounded_outline.go` 同族
	// 的 `snapBox`/`borderWidth`），两种值都算"画对了"；布局仍按 0.5 算。
	if line.Bounds.Height != menuSeparatorHeight && line.Bounds.Height != 1 {
		t.Errorf("分隔线高=%v，官方 `_separator_70kcw_157` 是 0.5px（scale=1 下允许吸附成 1）",
			line.Bounds.Height)
	}
	wantX := surface.X + menuPadding + menuSeparatorMarginX
	wantW := surface.Width - 2*(menuPadding+menuSeparatorMarginX)
	if d := line.Bounds.X - wantX; d > 1 || d < -1 {
		t.Errorf("分隔线 x=%v，官方 margin 8px（含菜单 4px padding）应为 %v", line.Bounds.X, wantX)
	}
	if d := line.Bounds.Width - wantW; d > 1 || d < -1 {
		t.Errorf("分隔线宽=%v，应为 %v", line.Bounds.Width, wantW)
	}
	// 两条行之间必须空出 0.5 + 2×3 = 6.5 的距离。
	gap := items[1].Bounds.Y - (items[0].Bounds.Y + items[0].Bounds.Height)
	want := menuSeparatorHeight + 2*menuSeparatorMarginY
	if d := gap - want; d > 1 || d < -1 {
		t.Errorf("两条菜单行之间空 %v，官方分隔线 0.5 + 上下各 3 = %v", gap, want)
	}
}
