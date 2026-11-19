//go:build darwin || windows || linux

// changes_menu_test.go —— 范围菜单"面与行"的判据（Viva headless）。
//
// 账本 #64 ⑧ 原来剩的那半是「菜单面阴影」：官方 `--dsw-elevation-prominent`
// （0 0 0 .5px 描边 + 0 3px 8px .04 + 0 0 20px .05，
// `gradient-shadow-text.css:36-37`），Viva 的默认面写死 `gfx.ShadowMD`
// （`viva@v0.6.5/components/dropdown_menu.go:813`）。v0.6.5 的
// `Unpainted`/`DrawPanel`/`DrawItem`（`ui/menu.go:377/382/389`）让调用方接管面与行
// 之后这一条可以闭合，所以这里的判据全部落在**官方读数**上：
//
//	① 柔光两层 = prominent 的 (offsetY 3, blur 8, α .04) 与 (blur 20, α .05)；
//	   任何带**负 spread** 的阴影都算 viva 那一档（ShadowMD 是 `-1px`/`-2px` 扩展）
//	   ——这条同时是"没退回 ShadowMD"的否定判据；
//	② 面材料 = `--dsw-specific-menu`（`design-platform.css:290,414`），圆角 md；
//	③ 面外那圈 0.5px 描边 = `--dsw-elevation-stroke-color: border-l1`
//	   （`ModelSelect.module.css:114-116` 明写 `border: 0` + 这行重绑）；
//	④ 行 = 官方 `.cell`：13px/20px、`padding-inline: 8px`、悬停底
//	   `--dsw-alias-interactive-bg-hover`、选中项打官方 `IconCheckOutlineRegular` 的勾。
package dsforge

import (
	"math"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// paintDirect 开一张 headless 画布，把 paint 里画的东西原样交回来。
//
// 为什么不复用 drawOncePrepared：那一个把"整页三个座位"画一遍再返回快照，
// 而面/行的判据要能**单独**驱动一个函数（面只画一次、行按 Active 画一次），
// 否则"这一帧里哪个矩形是菜单面"只能靠猜。
func paintDirect(t *testing.T, size gfx.Rect, paint func(canvas *gfx.Canvas)) app.Snapshot {
	t.Helper()
	var invalidator app.Invalidator
	config := app.Config{
		Title: "forge-menu", Width: int(size.Width), Height: int(size.Height),
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
		Paint: func(canvas *gfx.Canvas, _ ui.Size) {
			canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, gfx.RGBA(0, 0, 0, 0))
			paint(canvas)
		},
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	invalidator.Invalidate()
	for i := 0; i < 3; i++ {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	return session.Snapshot()
}

// sameRGB 只比红绿蓝，不比 alpha。
//
// 为什么要单列：菜单是**带入场动画**的浮层（`ui/menu.go:806-809` 的
// `canvas.BeginPresence`），驱动把这一档不透明度乘进了文字图元的颜色里，
// 所以同一帧里早期文本的 alpha 会是 0。判据只关心"用的哪个令牌"，那看 RGB。
func sameRGB(a, b ui.Color) bool { return a.Red == b.Red && a.Green == b.Green && a.Blue == b.Blue }

// rectPrimsByColor 收集颜色等于 color 的矩形图元。
func rectPrimsByColor(snapshot app.Snapshot, color ui.Color) []app.Primitive {
	out := make([]app.Primitive, 0, 4)
	for _, prim := range snapshot.Primitives {
		if prim.Kind == app.RectanglePrimitive && prim.Color == color {
			out = append(out, prim)
		}
	}
	return out
}

// rectPrimsByRGB 同 rectPrimsByColor，但只比 RGB（浮层入场不透明度会乘进 alpha）。
func rectPrimsByRGB(snapshot app.Snapshot, color ui.Color) []app.Primitive {
	out := make([]app.Primitive, 0, 4)
	for _, prim := range snapshot.Primitives {
		if prim.Kind == app.RectanglePrimitive && sameRGB(prim.Color, color) {
			out = append(out, prim)
		}
	}
	return out
}

// rgbTriangleInk 收集颜色（只比 RGB）等于 color、且三个顶点都落在 within 内的三角。
//
// 与 `triangleInk`（`icons_branch_test.go`）同一思路，区别只在"不看 alpha"：
// 浮层里的图元颜色被入场不透明度乘过（见 sameRGB）。
func rgbTriangleInk(snapshot app.Snapshot, color ui.Color, within gfx.Rect) (gfx.Rect, int) {
	box := gfx.Rect{X: math.MaxFloat32, Y: math.MaxFloat32}
	maxX, maxY := float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	count := 0
	for _, prim := range snapshot.Primitives {
		if prim.Kind != app.TrianglePrimitive || !sameRGB(prim.Color, color) {
			continue
		}
		inside := true
		for _, p := range prim.Points {
			if p.X < within.X || p.X > within.X+within.Width || p.Y < within.Y || p.Y > within.Y+within.Height {
				inside = false
			}
		}
		if !inside {
			continue
		}
		count++
		box.X = min(box.X, prim.Points[0].X)
		box.Y = min(box.Y, prim.Points[0].Y)
		for _, p := range prim.Points[1:] {
			box.X = min(box.X, p.X)
			box.Y = min(box.Y, p.Y)
			maxX = max(maxX, p.X)
			maxY = max(maxY, p.Y)
		}
	}
	if count == 0 {
		return gfx.Rect{}, 0
	}
	box.Width = maxX - box.X
	box.Height = maxY - box.Y
	return box, count
}

// exactTriangleInk 同 rgbTriangleInk，但颜色**连 alpha 一起**比。
//
// 用在没有入场不透明度的画布上（paintDirect 直接画，无浮层动画）：
// `border-l1` 与 `border-l2` 的 RGB 都是纯白、只差 alpha（.06 / .12），
// 所以"描边用哪一档"这条判据必须比 alpha，否则两档互换它也绿。
func exactTriangleInk(snapshot app.Snapshot, color ui.Color, within gfx.Rect) int {
	count := 0
	for _, prim := range snapshot.Primitives {
		if prim.Kind != app.TrianglePrimitive || prim.Color != color {
			continue
		}
		inside := true
		for _, p := range prim.Points {
			if p.X < within.X || p.X > within.X+within.Width || p.Y < within.Y || p.Y > within.Y+within.Height {
				inside = false
			}
		}
		if inside {
			count++
		}
	}
	return count
}

// TestScopeMenuPanelPaintsOfficialElevation 钉住菜单面：prominent 两层柔光 +
// border-l1 那圈 0.5px 描边 + `--dsw-specific-menu` 的面材料。
func TestScopeMenuPanelPaintsOfficialElevation(t *testing.T) {
	theme := dscore.Dark()
	panel := gfx.Rect{X: 100, Y: 80, Width: 180, Height: 140}
	snapshot := paintDirect(t, gfx.Rect{Width: 480, Height: 360}, func(canvas *gfx.Canvas) {
		drawScopeMenuPanel(canvas, panel, theme)
	})

	// ① prominent 的两层柔光（官方 `--dsw-elevation-prominent` 原值）。
	for _, layer := range theme.Shadows.Prominent.Glows {
		found := false
		for _, prim := range snapshot.Primitives {
			if prim.Kind != app.ShadowPrimitive {
				continue
			}
			if prim.Shadow.OffsetY == layer.OffsetY && prim.Shadow.Blur == layer.Blur &&
				math.Abs(float64(prim.Color.Alpha-layer.Alpha)) < 1e-6 {
				found = true
			}
		}
		if !found {
			t.Fatalf("菜单面缺官方 prominent 柔光层（offsetY=%v blur=%v α=%v）",
				layer.OffsetY, layer.Blur, layer.Alpha)
		}
	}
	// ② 否定判据：出现负 spread 就说明退回 viva 的 `gfx.ShadowMD`
	// （`0 4px 6px -1px` / `0 2px 4px -2px`，`viva@v0.6.5/gfx/canvas.go:123`）。
	for _, prim := range snapshot.Primitives {
		if prim.Kind == app.ShadowPrimitive && prim.Shadow.Spread < 0 {
			t.Fatalf("菜单面用的是 viva ShadowMD（spread=%v），不是官方 prominent", prim.Shadow.Spread)
		}
	}
	// ③ 面材料 = `--dsw-specific-menu`，圆角 md。
	surface := rectPrimsByColor(snapshot, theme.Menu)
	if len(surface) == 0 {
		t.Fatalf("菜单面没有铺 `--dsw-specific-menu`（t.Menu=%v）", theme.Menu)
	}
	if surface[0].Radius != theme.Radii.MD {
		t.Fatalf("菜单面圆角 = %v，官方是 --dsw-radius-md = %v", surface[0].Radius, theme.Radii.MD)
	}
	if surface[0].Bounds.Width < panel.Width || surface[0].Bounds.Height < panel.Height {
		t.Fatalf("菜单面没盖住 bounds：面 %+v，bounds %+v", surface[0].Bounds, panel)
	}
	// ④ 那圈 0.5px 描边用 `border-l1`（官方在菜单面上重绑了
	// `--dsw-elevation-stroke-color`），且落在面**外**（外扩 0.5）。
	ring := gfx.Rect{
		X: panel.X - 1.5, Y: panel.Y - 1.5,
		Width: panel.Width + 3, Height: panel.Height + 3,
	}
	if exactTriangleInk(snapshot, theme.BorderL1, ring) == 0 {
		t.Fatalf("菜单面外没有 border-l1 的 0.5px 描边（--dsw-elevation-stroke-color 在菜单面重绑成 border-l1）")
	}
	// 否定判据：默认档 `border-l4` 与相邻的 `border-l2` 都不许出现在那圈上
	// （`paintElevation` 的 stroke 是调用方传的，传错档这条就红）。
	if n := exactTriangleInk(snapshot, theme.BorderL2, ring); n != 0 {
		t.Fatalf("菜单面描边用了 border-l2（%d 条），官方重绑的是 border-l1", n)
	}
}

// TestScopeMenuRowPaintsOfficialCell 钉住一行：悬停底、13px/20px 的字、打了勾的行
// 才画官方 `IconCheckOutlineRegular`。
func TestScopeMenuRowPaintsOfficialCell(t *testing.T) {
	theme := dscore.Dark()
	row := gfx.Rect{X: 40, Y: 60, Width: 180, Height: 34}
	frame := gfx.Rect{Width: 320, Height: 200}

	active := paintDirect(t, frame, func(canvas *gfx.Canvas) {
		drawScopeMenuItem(ui.MenuItemParts{
			Canvas: canvas, Theme: dscore.VivaTheme(theme), Rect: row,
			Label: "未暂存", Active: true, Foreground: theme.LabelPrimary,
		}, theme)
	})
	// ① 悬停底 = `--dsw-alias-interactive-bg-hover`（`.cell:hover`），圆角 md。
	hover := rectPrimsByColor(active, theme.Hover)
	if len(hover) == 0 {
		t.Fatalf("悬停行没铺 `--dsw-alias-interactive-bg-hover`")
	}
	if hover[0].Radius != theme.Radii.MD {
		t.Fatalf("悬停底圆角 = %v，官方 .cell 是 --dsw-radius-md = %v", hover[0].Radius, theme.Radii.MD)
	}
	// ② 字 = 官方 `.cell` 的 13px/20px，颜色 label-primary，左缘 8。
	label, ok := firstLabel(active, "未暂存")
	if !ok {
		t.Fatalf("行里的标签没画出来")
	}
	if scopeMenuRowFont != 13 || label.FontSize != scopeMenuRowFont {
		t.Fatalf("行字号 = %v，官方 .cell 是 13px", label.FontSize)
	}
	// 8 是官方 `.cell{padding:0 8px}` 的字面值：这里**不用常量比常量**，
	// 否则改常量时两边一起动、判据永远绿（变异实测过）。
	if scopeMenuRowPadX != 8 {
		t.Fatalf("行内边距常量 = %v，官方 .cell 是 8px", scopeMenuRowPadX)
	}
	if label.Bounds.X != row.X+8 {
		t.Fatalf("标签左缘 = %v，官方 .cell 的 padding-inline 8 → %v", label.Bounds.X, row.X+8)
	}
	if !sameRGB(label.Color, theme.LabelPrimary) {
		t.Fatalf("标签字色 = %v，官方 .cell 是 label-primary", label.Color)
	}
	// ③ 没打勾的行不许有勾的墨迹。
	if ink, _ := rgbTriangleInk(active, theme.LabelPrimary, row); ink.Width > 0 {
		t.Fatalf("没选中的行画了勾：墨迹 %+v", ink)
	}

	chosen := paintDirect(t, frame, func(canvas *gfx.Canvas) {
		drawScopeMenuItem(ui.MenuItemParts{
			Canvas: canvas, Theme: dscore.VivaTheme(theme), Rect: row,
			Label: "分支", Selected: true, Foreground: theme.LabelPrimary,
		}, theme)
	})
	// ④ 打了勾的行：墨迹落在行右缘的 16px 勾框里（`.cell` 的 padding-inline 8）。
	if scopeMenuCheckSize != 16 {
		t.Fatalf("勾的尺寸常量 = %v，官方 sprite 是 16", scopeMenuCheckSize)
	}
	checkBox := gfx.Rect{
		X: row.X + row.Width - scopeMenuRowPadX - scopeMenuCheckSize, Y: row.Y,
		Width: scopeMenuCheckSize, Height: row.Height,
	}
	ink, count := rgbTriangleInk(chosen, theme.LabelPrimary, checkBox)
	if count == 0 {
		t.Fatalf("选中行右缘没有勾的墨迹（官方 Menu 的 selectedId 打勾）")
	}
	// 勾的墨迹只能是那一枚 16px 的官方素材：横向几何 4..20 缩到 16 → 约 10.5px，
	// 加上描边 1 的两侧各 0.5；这里钉"不是别的形状、且没越框"。
	if ink.Width < 8 || ink.Width > 13 || ink.Height < 8 || ink.Height > 13 {
		t.Fatalf("勾的墨迹尺寸不像官方 16px 的 IconCheckOutlineRegular：%+v", ink)
	}
	if ink.X < checkBox.X-0.5 || ink.X+ink.Width > checkBox.X+checkBox.Width+0.5 {
		t.Fatalf("勾画出勾框了：墨迹 %+v，框 %+v", ink, checkBox)
	}
	// ⑤ 勾就是官方素材的产物：三角化条数与 `dsassets.CheckOutline()` 单独画一致。
	if want := checkIconTriangles(t, theme, frame); count != want {
		t.Fatalf("勾的三角化条数 = %d，官方素材单独画是 %d（说明不是同一枚素材）", count, want)
	}
}

// TestScopeMenuSeparatorKeepsMenuOwnGeometry 钉住分隔线：菜单自己的那一档
// （`viva@v0.6.5/components/dropdown_menu.go:974-978`：`Fill(y+4, h=1, theme.Border)`），
// 颜色取菜单给的 Foreground（`addSeparator` 传的就是 `theme.Border`）。
func TestScopeMenuSeparatorKeepsMenuOwnGeometry(t *testing.T) {
	theme := dscore.Dark()
	row := gfx.Rect{X: 40, Y: 60, Width: 180, Height: 9}
	snapshot := paintDirect(t, gfx.Rect{Width: 320, Height: 200}, func(canvas *gfx.Canvas) {
		drawScopeMenuItem(ui.MenuItemParts{
			Canvas: canvas, Theme: dscore.VivaTheme(theme), Rect: row,
			Separator: true, Foreground: theme.BorderL2,
		}, theme)
	})
	lines := rectPrimsByColor(snapshot, theme.BorderL2)
	if len(lines) != 1 {
		t.Fatalf("分隔线应该正好一条，画了 %d 条", len(lines))
	}
	line := lines[0]
	if scopeMenuSeparatorHeight != 1 || line.Bounds.Height != scopeMenuSeparatorHeight {
		t.Fatalf("分隔线高 = %v，菜单自己那一档是 1px", line.Bounds.Height)
	}
	if scopeMenuSeparatorTop != 4 || line.Bounds.Y != row.Y+scopeMenuSeparatorTop {
		t.Fatalf("分隔线 Y = %v，菜单自己那一档是行顶 +4 → %v", line.Bounds.Y, row.Y+scopeMenuSeparatorTop)
	}
}

// TestOpenScopeMenuDrawsRowsAndOfficialSurface 是**接线**判据：真菜单打开那一帧里，
// 面与行确实由 `DrawPanel`/`DrawItem` 画出来了（不是 Unpainted 之后什么都没画——
// 那正是"菜单条目全消失"那个坑）。
func TestOpenScopeMenuDrawsRowsAndOfficialSurface(t *testing.T) {
	theme := dscore.Dark()
	opened := drawOncePrepared(t, sampleView(), "s-1", func(p *Plugin) {
		p.open.menu["s-1"] = true
	})

	// ① 四个条目都在，字是官方 .cell 的 13px、label-primary。
	for _, want := range []string{"未提交", "未暂存", "已暂存", "分支"} {
		at := -1
		for i := len(opened.Labels) - 1; i >= 0; i-- {
			if opened.Labels[i].Value == want && opened.Labels[i].FontSize == scopeMenuRowFont {
				at = i
				break
			}
		}
		if at < 0 {
			t.Fatalf("菜单条目 %q 没按官方 13px 画出来：\n%s", want, drawnText(opened))
		}
		if !sameRGB(opened.Labels[at].Color, theme.LabelPrimary) {
			t.Fatalf("菜单条目 %q 的字色 = %v，官方 .cell 是 label-primary", want, opened.Labels[at].Color)
		}
	}

	// ② 面是 `--dsw-specific-menu`：整帧里只有菜单面该铺这个色。
	surface := rectPrimsByRGB(opened, theme.Menu)
	if len(surface) == 0 {
		t.Fatalf("打开的菜单面没铺 `--dsw-specific-menu`（DrawPanel 没接上）")
	}
	if surface[0].Radius != theme.Radii.MD {
		t.Fatalf("菜单面圆角 = %v，官方 --dsw-radius-md = %v", surface[0].Radius, theme.Radii.MD)
	}

	// ③ 官方 prominent 柔光在真菜单这一帧也在；且没有退回 ShadowMD。
	found := false
	for _, prim := range opened.Primitives {
		if prim.Kind != app.ShadowPrimitive {
			continue
		}
		if prim.Shadow.Spread < 0 {
			t.Fatalf("真菜单退回 viva ShadowMD 了（spread=%v）", prim.Shadow.Spread)
		}
		if prim.Shadow.OffsetY == theme.Shadows.Prominent.Glows[0].OffsetY &&
			prim.Shadow.Blur == theme.Shadows.Prominent.Glows[0].Blur {
			found = true
		}
	}
	if !found {
		t.Fatalf("真菜单面没有官方 prominent 柔光")
	}

	// ④ 当前范围是 branch ⇒ 勾只出现在「分支」那一行。
	branchY, ok := menuLabelY(opened, "分支")
	if !ok {
		t.Fatalf("找不到菜单里的「分支」行")
	}
	otherY, ok := menuLabelY(opened, "未暂存")
	if !ok {
		t.Fatalf("找不到菜单里的「未暂存」行")
	}
	band := gfx.Rect{X: surface[0].Bounds.X, Y: branchY, Width: surface[0].Bounds.Width, Height: scopeMenuRowLine}
	if _, count := rgbTriangleInk(opened, theme.LabelPrimary, band); count == 0 {
		t.Fatalf("选中行（分支）没有勾：band=%+v", band)
	}
	other := gfx.Rect{X: surface[0].Bounds.X, Y: otherY, Width: surface[0].Bounds.Width, Height: scopeMenuRowLine}
	if ink, _ := rgbTriangleInk(opened, theme.LabelPrimary, other); ink.Width > 0 {
		t.Fatalf("没选中的行（未暂存）也画了勾：%+v", ink)
	}
}

// menuLabelY 取菜单里那一行文字的 Y（按官方行字号筛，免得撞上芯片同名的字）。
func menuLabelY(snapshot app.Snapshot, value string) (float32, bool) {
	for i := len(snapshot.Labels) - 1; i >= 0; i-- {
		label := snapshot.Labels[i]
		if label.Value == value && label.FontSize == scopeMenuRowFont {
			return label.Bounds.Y, true
		}
	}
	return 0, false
}

// firstLabel 找一帧里第一条文字。
func firstLabel(snapshot app.Snapshot, value string) (app.Label, bool) {
	for _, label := range snapshot.Labels {
		if label.Value == value {
			return label, true
		}
	}
	return app.Label{}, false
}

// checkIconTriangles 单独画一次官方勾素材，数它的三角化条数（对照判据用）。
func checkIconTriangles(t *testing.T, theme *pluginapi.Theme, frame gfx.Rect) int {
	t.Helper()
	snapshot := paintDirect(t, frame, func(canvas *gfx.Canvas) {
		icon, ok := scopeMenuCheckIcon()
		if !ok {
			t.Fatalf("官方勾素材取不到（dsassets.CheckOutline）")
		}
		canvas.DrawIcon(icon, gfx.Rect{X: 8, Y: 8, Width: scopeMenuCheckSize, Height: scopeMenuCheckSize}, theme.LabelPrimary)
	})
	_, count := rgbTriangleInk(snapshot, theme.LabelPrimary, frame)
	return count
}
