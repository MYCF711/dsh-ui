//go:build darwin || windows || linux

// guide_menu_test.go —— 终端 shell 菜单**是官方菜单浮层**的判据（Viva headless）。
//
// # 这一条闭合的是什么
//
// 账本 #54 ⑤ 原来记的是「菜单载体仍手绘（Viva 限制）」：面被画成卡片列的**子元素**，
// 打开时把下面的内容整体下推、没有方向键、点外部不关闭。v0.6.5 起 `ui.Menu` 给了
// `Unpainted`/`DrawPanel`/`DrawItem`（`viva@v0.6.5/ui/menu.go:377/382/389`，原文本就
// 写明「Hits, focus, the keyboard, dismissal and submenu opening stay the menu's」），
// 所以这里改成官方形状：`<Menu open portal autoFocus align="end" …>`
// （`OFF:dsh-client-ui-sidebar-terminal/src/client/TerminalGuide.tsx:62-77`）。
//
// # 判据全部落在官方读数上
//
//	① **不占布局**：菜单打开前后，卡片标题与**下一张卡片**的 Y 一格不动；
//	   菜单里的字**不在**卡片矩形内（它是浮层，不是卡片的一行）。
//	② **面**：材料 `--dsw-specific-menu`（`pluginapi.Theme.Menu`）、圆角 md、
//	   官方 `--dsw-elevation-prominent` 的两层柔光、外圈 `--dsw-elevation-stroke-color:
//	   border-l1`（`ModelSelect.module.css:109-117` 原文注释写明照 `Menu` 原语复刻）；
//	   **否定判据**：出现负 spread 就是退回了 viva 的 `gfx.ShadowMD`。
//	③ **行**：官方 `.cell` 的 13px、label-primary、左缘 8（`ModelSelect.module.css:347-354`）。
//	④ **勾**：只有 `selectedId` 那一行有勾，且比的是 **shell.path**（官方 `items` 的 id
//	   就是 path，`dsh-api-terminal-controller/src/client/index.ts:169-191`）。
package dsterminal

import (
	"math"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// guideMenuShells 是两枚真 shell：name 与 path **不同**，这样"勾打在谁身上"才可证伪。
var guideMenuShells = []apiclient.TerminalShell{
	{Path: "/bin/bash", Name: "bash"},
	{Path: "/bin/zsh", Name: "zsh"},
}

// guideMenuFrame 真画一帧向导页（两张卡片），返回 headless 快照。
//
// 两张卡片是**布局判据的夹具**：第二张卡片的标题 Y 就是"菜单有没有占布局"的量尺。
func guideMenuFrame(t *testing.T, open bool, choices pluginapi.TerminalShellChoices) app.Snapshot {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("terminal-guide-menu")
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	p := &Plugin{guideMenuOpen: open}

	var invalidator app.Invalidator
	config := app.Config{
		Title: "terminal-guide", Width: 420, Height: 360, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: bounds.Width,
			Data: pluginapi.Snapshot{Cur: "s1"},
		}
		env.Act.OpenRightTab = func(string) {}
		env.Act.OpenRightTabParams = func(string, pluginapi.RightTabParams) {}
		env.Act.TerminalLaunchShells = func() pluginapi.TerminalShellChoices { return choices }
		env.Act.TerminalSelectShell = func(string) {}
		page.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).FillWidth().Gap(8).Pad(16).Children(func() {
				entry := env
				entry.GuideEntry = &pluginapi.GuideEntryProps{
					EntryID: "new", Kind: RightTabTerminal,
					Title: zhGuideTitle, Description: zhGuideDescription,
				}
				p.drawGuideEntry(f, entry)
				second := env
				second.GuideEntry = &pluginapi.GuideEntryProps{
					EntryID: "second", Kind: RightTabTerminal,
					Title: "第二个类目", Description: "量布局用",
				}
				// 另一个座位占用者：各自持有自己的菜单状态（真实装配里一个 guide
				// 座位一个占用者，共用一份状态会让两张卡的菜单同时打开）。
				other := &Plugin{}
				other.drawGuideEntry(f, second)
			})
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	invalidator.Invalidate()
	for i := 0; i < 5; i++ {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	return session.Snapshot()
}

// sameRGB 只比红绿蓝：浮层有入场动画，驱动会把不透明度乘进颜色（alpha 会变）。
func sameRGB(a, b ui.Color) bool { return a.Red == b.Red && a.Green == b.Green && a.Blue == b.Blue }

// rectPrimsByRGB 收集颜色（只比 RGB）等于 color 的矩形图元。
func rectPrimsByRGB(snapshot app.Snapshot, color ui.Color) []app.Primitive {
	out := make([]app.Primitive, 0, 4)
	for _, prim := range snapshot.Primitives {
		if prim.Kind == app.RectanglePrimitive && sameRGB(prim.Color, color) {
			out = append(out, prim)
		}
	}
	return out
}

// triInkRGB 收集颜色（只比 RGB）等于 color、且三个顶点都在 within 内的三角，返回包围盒与条数。
func triInkRGB(snapshot app.Snapshot, color ui.Color, within gfx.Rect) (gfx.Rect, int) {
	box := gfx.Rect{X: math.MaxFloat32, Y: math.MaxFloat32}
	maxX, maxY := float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	count := 0
	for _, prim := range snapshot.Primitives {
		if prim.Kind != app.TrianglePrimitive || !sameRGB(prim.Color, color) {
			continue
		}
		inside := true
		for _, point := range prim.Points {
			if point.X < within.X || point.X > within.X+within.Width ||
				point.Y < within.Y || point.Y > within.Y+within.Height {
				inside = false
			}
		}
		if !inside {
			continue
		}
		count++
		for _, point := range prim.Points {
			box.X = min(box.X, point.X)
			box.Y = min(box.Y, point.Y)
			maxX = max(maxX, point.X)
			maxY = max(maxY, point.Y)
		}
	}
	if count == 0 {
		return gfx.Rect{}, 0
	}
	box.Width = maxX - box.X
	box.Height = maxY - box.Y
	return box, count
}

// labelAt 找一条文字（按值 + 官方行字号筛，免得撞上标题里同名的字）。
func labelAt(snapshot app.Snapshot, value string, size float32) (app.Label, bool) {
	for _, label := range snapshot.Labels {
		if label.Value == value && (size == 0 || label.FontSize == size) {
			return label, true
		}
	}
	return app.Label{}, false
}

// menuSurface 是这一帧里菜单面那块矩形（按 `--dsw-specific-menu` 材质找）。
func menuSurface(t *testing.T, snapshot app.Snapshot, theme *pluginapi.Theme) gfx.Rect {
	t.Helper()
	surface := rectPrimsByRGB(snapshot, theme.Menu)
	if len(surface) == 0 {
		t.Fatalf("打开的菜单面没铺 `--dsw-specific-menu`（DrawPanel 没接上）")
	}
	return surface[0].Bounds
}

// TestGuideMenuIsAnOverlayNotAnExtraRow 钉住「菜单不占布局」：打开前后，卡片标题与
// **下一张卡片**的 Y 一格不动，而且菜单里的字不在卡片矩形内。
func TestGuideMenuIsAnOverlayNotAnExtraRow(t *testing.T) {
	ready := pluginapi.TerminalShellChoices{
		Phase: pluginapi.TerminalShellsReady, Shells: guideMenuShells, Selected: "/bin/zsh",
	}
	closed := guideMenuFrame(t, false, ready)
	opened := guideMenuFrame(t, true, ready)

	for _, want := range []string{zhGuideTitle, "第二个类目"} {
		before, ok := labelAt(closed, want, guideTitleFont)
		if !ok {
			t.Fatalf("关闭态没画卡片标题 %q", want)
		}
		after, ok := labelAt(opened, want, guideTitleFont)
		if !ok {
			t.Fatalf("打开态没画卡片标题 %q", want)
		}
		if before.Bounds.Y != after.Bounds.Y {
			t.Fatalf("菜单占了布局：%q 的 Y 从 %v 变成 %v（旧手绘面板就是这个症状）",
				want, before.Bounds.Y, after.Bounds.Y)
		}
	}

	// 菜单行是浮层：它的字不在任何一张卡片矩形里。
	theme := dscore.Dark()
	card := rectPrimsByRGB(opened, theme.BgLayer1)
	if len(card) == 0 {
		t.Fatal("没找到卡片底（`--dsw-alias-bg-layer-1`）")
	}
	row, ok := labelAt(opened, "zsh", guideMenuFont)
	if !ok {
		t.Fatalf("打开态没画 shell 行：%v", opened.Labels)
	}
	for _, one := range card {
		if row.Bounds.X >= one.Bounds.X && row.Bounds.X <= one.Bounds.X+one.Bounds.Width &&
			row.Bounds.Y >= one.Bounds.Y && row.Bounds.Y <= one.Bounds.Y+one.Bounds.Height {
			t.Fatalf("菜单行落在卡片矩形 %+v 内（%+v）——说明菜单还是卡片里的一行",
				one.Bounds, row.Bounds)
		}
	}
}

// TestGuideMenuPaintsOfficialMenuSurface 钉住面：菜单材质 + 圆角 md + prominent 两层柔光 +
// border-l1 那圈 0.5px 描边；否定判据是"不许退回 viva ShadowMD"。
func TestGuideMenuPaintsOfficialMenuSurface(t *testing.T) {
	theme := dscore.Dark()
	snapshot := guideMenuFrame(t, true, pluginapi.TerminalShellChoices{
		Phase: pluginapi.TerminalShellsReady, Shells: guideMenuShells, Selected: "/bin/zsh",
	})
	surface := menuSurface(t, snapshot, theme)

	// ① 圆角 md（`.cell` 与面都用 `--dsw-radius-md`）。
	for _, prim := range rectPrimsByRGB(snapshot, theme.Menu) {
		if prim.Radius != theme.Radii.MD {
			t.Fatalf("菜单面圆角 = %v，官方 --dsw-radius-md = %v", prim.Radius, theme.Radii.MD)
		}
	}
	// ② 官方 prominent 的两层柔光：比 (offsetY, blur) —— 浮层进场时驱动会把
	// 不透明度乘进颜色，alpha 不能当判据（`sameRGB` 同一条理由）。
	for _, layer := range theme.Shadows.Prominent.Glows {
		found := false
		for _, prim := range snapshot.Primitives {
			if prim.Kind != app.ShadowPrimitive {
				continue
			}
			if prim.Shadow.OffsetY == layer.OffsetY && prim.Shadow.Blur == layer.Blur {
				found = true
			}
		}
		if !found {
			t.Fatalf("菜单面缺官方 prominent 柔光层（offsetY=%v blur=%v）",
				layer.OffsetY, layer.Blur)
		}
	}
	// ③ 否定判据：负 spread = 退回 viva 的 `gfx.ShadowMD`（`0 4px 6px -1px`）。
	for _, prim := range snapshot.Primitives {
		if prim.Kind == app.ShadowPrimitive && prim.Shadow.Spread < 0 {
			t.Fatalf("菜单面用的是 viva ShadowMD（spread=%v），不是官方 prominent", prim.Shadow.Spread)
		}
	}
	// ④ 外圈那圈 0.5px 描边的**颜色档**不在这里判：浮层入场时驱动把不透明度乘成 0，
	// 而 border-l1/l2/l4 的 RGB 都是纯白、只差 alpha，比 RGB 比不出档位。它由共享画笔
	// 自己的守卫钉住——`plugins/dscore` 的 `TestElevationDecisionTablePerSite` 里
	// `dscore/menu.go` 那一行把 `PaintMenuPanel` 的档位锁在 Prominent + BorderL1，
	// 改这里改不到它（两处菜单都走同一支画笔）。
	// 这里只判"面不是 viva 的默认面板"：默认面板没有描边层、且阴影是 ShadowMD
	// （③ 已经把它挡掉）。
	// 面要装得下它的行：zsh 那行的字必须在面里。
	if row, ok := labelAt(snapshot, "zsh", guideMenuFont); !ok ||
		row.Bounds.X < surface.X || row.Bounds.X+row.Bounds.Width > surface.X+surface.Width {
		t.Fatalf("菜单行没落在面里：面 %+v", surface)
	}
}

// TestGuideMenuRowsUseOfficialCell 钉住行：官方 `.cell` 的 13px / label-primary / 左缘 8。
func TestGuideMenuRowsUseOfficialCell(t *testing.T) {
	theme := dscore.Dark()
	snapshot := guideMenuFrame(t, true, pluginapi.TerminalShellChoices{
		Phase: pluginapi.TerminalShellsReady, Shells: guideMenuShells, Selected: "/bin/bash",
	})
	surface := menuSurface(t, snapshot, theme)
	for _, want := range []string{"bash", "zsh"} {
		row, ok := labelAt(snapshot, want, guideMenuFont)
		if !ok {
			t.Fatalf("%q 行没按官方 13px 画出来：%v", want, snapshot.Labels)
		}
		if !sameRGB(row.Color, theme.LabelPrimary) {
			t.Fatalf("%q 行的字色 = %v，官方 .cell 是 label-primary", want, row.Color)
		}
		// 两段内边距叠出来的左缘：面 `padding:4px`（`ModelSelect.module.css:117`
		// 照 `Menu` 原语复刻的那一张面）+ 行 `.cell{padding:0 8px}`（`:348`）。
		// 8 与 4 都写字面值，不拿常量比常量（常量本身在 guide_test.go 的几何表里钉）。
		if row.Bounds.X != surface.X+4+8 {
			t.Fatalf("%q 行左缘 = %v，官方 = 面 4 + .cell 8 → %v",
				want, row.Bounds.X, surface.X+4+8)
		}
	}
}

// TestGuideMenuChecksOnlyTheSelectedShellByPath 钉住 `selectedId`：勾只打在当前 shell 那
// 一行，而且比的是 path —— 宿主回的 `Selected` 是 path（`launchShells` 的
// `find(s => s.path === previous)?.path`），拿标签比会在 name≠path 时永远不打勾。
func TestGuideMenuChecksOnlyTheSelectedShellByPath(t *testing.T) {
	theme := dscore.Dark()
	snapshot := guideMenuFrame(t, true, pluginapi.TerminalShellChoices{
		Phase: pluginapi.TerminalShellsReady, Shells: guideMenuShells, Selected: "/bin/zsh",
	})
	surface := menuSurface(t, snapshot, theme)
	zsh, ok := labelAt(snapshot, "zsh", guideMenuFont)
	if !ok {
		t.Fatal("zsh 行没画出来")
	}
	bash, ok := labelAt(snapshot, "bash", guideMenuFont)
	if !ok {
		t.Fatal("bash 行没画出来")
	}
	band := func(label app.Label) gfx.Rect {
		return gfx.Rect{X: surface.X, Y: label.Bounds.Y, Width: surface.Width, Height: guideMenuLine}
	}
	if ink, count := triInkRGB(snapshot, theme.LabelPrimary, band(zsh)); count == 0 || ink.Width == 0 {
		t.Fatalf("当前 shell（/bin/zsh）那一行没有勾：selectedId 没接上")
	}
	if ink, _ := triInkRGB(snapshot, theme.LabelPrimary, band(bash)); ink.Width > 0 {
		t.Fatalf("没选中那一行也画了勾：%+v", ink)
	}

	// 反证：`Selected` 恰好等于某个 **name**（不是 path）时，不许打勾。
	p := &Plugin{}
	env := &pluginapi.Env{}
	rows := guideMenuRows(pluginapi.TerminalShellChoices{
		Phase: pluginapi.TerminalShellsReady, Shells: guideMenuShells, Selected: "zsh",
	}, env, p)
	for _, row := range rows {
		if row.chosen {
			t.Fatalf("Selected 是标签不是 path 时不该有勾：%+v", row)
		}
	}
	rows = guideMenuRows(pluginapi.TerminalShellChoices{
		Phase: pluginapi.TerminalShellsReady, Shells: guideMenuShells, Selected: "/bin/zsh",
	}, env, p)
	chosen := ""
	for _, row := range rows {
		if row.chosen {
			chosen = row.label
		}
	}
	if chosen != "zsh" {
		t.Fatalf("按 path 比之后打勾的行 = %q，want zsh", chosen)
	}
}

// guideCardFrame 真画一帧**单张**卡片（图标尺寸判据用），description 为空即官方
// `description === undefined` 那一支。
func guideCardFrame(t *testing.T, description string) app.Snapshot {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("terminal-guide-icon")
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	p := &Plugin{}
	var invalidator app.Invalidator
	config := app.Config{
		Title: "terminal-guide-icon", Width: 420, Height: 160, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: bounds.Width,
			Data: pluginapi.Snapshot{Cur: "s1"},
		}
		env.Act.OpenRightTab = func(string) {}
		env.Act.TerminalLaunchShells = func() pluginapi.TerminalShellChoices {
			return pluginapi.TerminalShellChoices{Phase: pluginapi.TerminalShellsReady}
		}
		env.GuideEntry = &pluginapi.GuideEntryProps{
			EntryID: "new", Kind: RightTabTerminal,
			Title: zhGuideTitle, Description: description,
		}
		page.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).FillWidth().Pad(16).Children(func() { p.drawGuideEntry(f, env) })
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	invalidator.Invalidate()
	for i := 0; i < 5; i++ {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	return session.Snapshot()
}

// TestGuideIconSizeFollowsDescription 钉住 `.icon` 里那枚
// `PluginArtworkTerminal size={description === undefined ? 22 : 26}`（TerminalGuide.tsx:58）。
//
// 判据用**比例**而不是绝对像素：素材是 36-box 的描边字形，墨迹不会恰好等于外框，
// 但两个尺寸的比值就是 26:22——把那个三元表达式写成常量 26 时两帧一样大，这条就红。
func TestGuideIconSizeFollowsDescription(t *testing.T) {
	stroke := dsassets.TerminalArtworkStroke()
	whole := gfx.Rect{X: -1e4, Y: -1e4, Width: 2e4, Height: 2e4}
	ink := func(description string) gfx.Rect {
		t.Helper()
		snapshot := guideCardFrame(t, description)
		box, count := triInkRGB(snapshot, stroke, whole)
		if count == 0 || box.Width == 0 {
			t.Fatalf("描述=%q 那一帧没画官方终端素材（%v）", description, snapshot.Primitives)
		}
		return box
	}
	withDescription := ink(zhGuideDescription)
	without := ink("")
	if withDescription.Width <= without.Width {
		t.Fatalf("没有描述时图标没变小：有描述 %v，无描述 %v（官方 26 → 22）",
			withDescription.Width, without.Width)
	}
	ratio := withDescription.Width / without.Width
	if want := guideIconBox / guideIconBoxCompact; math.Abs(float64(ratio-want)) > 0.05 {
		t.Fatalf("图标尺寸比 = %v，官方 %v（26:22）", ratio, want)
	}
}
