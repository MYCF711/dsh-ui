// icons_globe_test.go —— 地球赤道 `stroke-linecap="square"` 的两处落点判据（账本 #50 ⑦）。
//
// 官方 `IconGlobeOutlineRegular` 是**一枚** svg、四条 path，其中只有赤道那一笔写
// `stroke-linecap="square"`（另三条没写 = butt）：
//
//	OFF:dsh-client-ui-sidebar-browser/src/client/view/BrowserTitle.tsx:3
//	  <IconGlobeOutlineRegular size={14} />
//
// 本仓有两个落点，各用各的办法：
//
//	tab 芯片  `pluginapi.Painter.IconImage` 是**单枚** gfx.Icon（内核用 ui.Icon 画，
//	          internal/uikernel/view_tabs.go 的芯片分支），叠不了两枚 → 把赤道的
//	          方帽换成**等价几何**：两端各外推 strokeWidth/2（SVG 1.1 F.11），
//	          整枚图标仍是 butt ⇒ 墨迹与 square 端头逐像素相同。
//	标题行    ui.Stack 叠两枚同尺寸图标：外圆+经线（butt）+ 只有赤道那一笔
//	          （`Cap = gfx.CapSquare`，viva 真画方帽）。
//
// 本文件钉四件事：
//
//	① 外推量逐端 == StrokeWidth/2（几何，viewBox 单位，不吃像素容差）；
//	② 两处的赤道墨迹都落在官方算出来的 [1.84619, 14.1538]（headless 像素）；
//	③ 芯片那一枚与标题行叠出来的墨迹一致（"两处视觉一致"）；
//	④ 标题行为什么只把**远端**那半截写进坐标（viva 当前的 square 帽行为）。
package dsbrowser

import (
	"math"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// ── 几何：外推量与路径原文 ──────────────────────────────────────────────

// globePathPoints 解析一条官方 `d` 原文，返回它的全部端点与控制点。
func globePathPoints(t *testing.T, d string) []gfx.Point {
	t.Helper()
	commands, ok := dsassets.ParsePath(d)
	if !ok {
		t.Fatalf("官方路径解析失败：%q", d)
	}
	points := make([]gfx.Point, 0, len(commands))
	for _, command := range commands {
		if command.Kind == gfx.PathCommandClose {
			continue
		}
		points = append(points, command.Point)
		if command.Kind == gfx.PathCommandCubic || command.Kind == gfx.PathCommandQuad {
			points = append(points, command.Control1)
		}
		if command.Kind == gfx.PathCommandCubic {
			points = append(points, command.Control2)
		}
	}
	return points
}

// globeCommands 解析一条官方 `d` 原文成命令表。
func globeCommands(t *testing.T, d string) []gfx.PathCommand {
	t.Helper()
	commands, ok := dsassets.ParsePath(d)
	if !ok {
		t.Fatalf("官方路径解析失败：%q", d)
	}
	return commands
}

// globeViewBoxBounds 是若干条 `d` 的几何包围盒（viewBox 单位）。
func globeViewBoxBounds(t *testing.T, paths ...string) (minX, minY, maxX, maxY float32) {
	t.Helper()
	minX, minY = math.MaxFloat32, math.MaxFloat32
	maxX, maxY = -math.MaxFloat32, -math.MaxFloat32
	for _, path := range paths {
		for _, point := range globePathPoints(t, path) {
			minX = min(minX, point.X)
			minY = min(minY, point.Y)
			maxX = max(maxX, point.X)
			maxY = max(maxY, point.Y)
		}
	}
	return minX, minY, maxX, maxY
}

// sameGlobeCommands 逐命令比较两条命令表（同一个解析器出来的，可以逐位比）。
func sameGlobeCommands(a, b []gfx.PathCommand) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// concatGlobeCommands 按官方 JSX 的 path 顺序拼出期望的命令表。
func concatGlobeCommands(t *testing.T, paths ...string) []gfx.PathCommand {
	t.Helper()
	out := []gfx.PathCommand{}
	for _, path := range paths {
		out = append(out, globeCommands(t, path)...)
	}
	return out
}

// globeEquatorEnds 返回赤道那一笔的两个端点（左边在前，与官方 `M…H…` 同序）。
func globeEquatorEnds(t *testing.T, d string) (left, right gfx.Point) {
	t.Helper()
	commands := globeCommands(t, d)
	if len(commands) != 2 || commands[0].Kind != gfx.PathCommandMove || commands[1].Kind != gfx.PathCommandLine {
		t.Fatalf("赤道应当只解析出 M + H 两条命令：%q → %+v", d, commands)
	}
	return commands[0].Point, commands[1].Point
}

// TestGlobeEquatorSquareCapExtendsByHalfTheStroke 是 ①（芯片那一枚）：
// 方帽的等价几何**逐端**正好外推 StrokeWidth/2，不是"大概挪一点"。
//
// 依据 SVG 1.1 F.11：square 端头把描边沿端点处的路径方向正向延长半个描边宽。
// 本图 strokeWidth = 1（`globeStrokeWidth`，官方 Regular 档），所以两端各 0.5。
func TestGlobeEquatorSquareCapExtendsByHalfTheStroke(t *testing.T) {
	// 官方 `d` 原文**逐字**没被改（改了就没有"官方来源"了）。
	if pathGlobeEquator != "M2.34619 8H13.6538" {
		t.Fatalf("赤道路径原文 = %q，官方是 M2.34619 8H13.6538", pathGlobeEquator)
	}
	if globeStrokeWidth != 1 {
		t.Fatalf("描边宽 = %v，官方 ICON_REGULAR_STROKE = 1", globeStrokeWidth)
	}
	if globeSquareCapExtend != globeStrokeWidth/2 {
		t.Fatalf("半描边常量 = %v，应当是 StrokeWidth/2 = %v", globeSquareCapExtend, globeStrokeWidth/2)
	}

	officialLeft, officialRight := globeEquatorEnds(t, pathGlobeEquator)
	bakedLeft, bakedRight := globeEquatorEnds(t, pathGlobeEquatorSquareCap)
	const epsilon = 1e-5
	if got := float64(officialLeft.X) - float64(bakedLeft.X); math.Abs(got-float64(globeSquareCapExtend)) > epsilon {
		t.Fatalf("左端外推量 = %.6f，应当是 StrokeWidth/2 = %.6f", got, globeSquareCapExtend)
	}
	if got := float64(bakedRight.X) - float64(officialRight.X); math.Abs(got-float64(globeSquareCapExtend)) > epsilon {
		t.Fatalf("右端外推量 = %.6f，应当是 StrokeWidth/2 = %.6f", got, globeSquareCapExtend)
	}
	// 方帽只沿路径方向延长：y 一点不动。
	if bakedLeft.Y != officialLeft.Y || bakedRight.Y != officialRight.Y {
		t.Fatalf("外推改了 y：%+v / %+v", bakedLeft, bakedRight)
	}
}

// TestGlobeChipBakesTheEquatorAndStaysButt 是芯片那一枚的结构判据：
//
//   - 赤道那一笔 = 方帽等价几何（逐命令等于 pathGlobeEquatorSquareCap）；
//   - 外圆与两条经线 = 官方原文，**没跟着外扩**；
//   - 整枚 Cap 仍是 butt（一枚 icon 只有一个 Cap，赤道靠几何等价）。
func TestGlobeChipBakesTheEquatorAndStaysButt(t *testing.T) {
	icon := iconGlobe(gfx.Color{})
	if icon.ViewBox != 16 || icon.Stroke != globeStrokeWidth {
		t.Fatalf("芯片图标 viewBox=%v stroke=%v，官方 16 / %v", icon.ViewBox, icon.Stroke, globeStrokeWidth)
	}
	if len(icon.Layers) != 0 {
		t.Fatalf("芯片图标不该有填充层：%d 层", len(icon.Layers))
	}
	if icon.Cap != gfx.CapButt {
		t.Fatalf("芯片图标的 Cap = %v，必须是零值 butt（整枚只有一个 Cap，赤道靠几何等价）", icon.Cap)
	}
	wanted := concatGlobeCommands(t, pathGlobeRing, pathGlobeEquatorSquareCap,
		pathGlobeMeridianWest, pathGlobeMeridianEast)
	if !sameGlobeCommands(icon.Commands, wanted) {
		t.Fatalf("芯片图标不是「官方原文 + 赤道等价几何」：%d 条命令，期望 %d 条（经线被外扩了？）",
			len(icon.Commands), len(wanted))
	}
}

// TestGlobeTitleIconsAreButtPlusASquareCappedEquator 是标题行两枚的结构判据：
// 底座（外圆+经线）Cap 为零值 butt、不含赤道；赤道那一枚 Cap = square、
// 几何 = 官方起点 + 远端那半截。
func TestGlobeTitleIconsAreButtPlusASquareCappedEquator(t *testing.T) {
	base := iconGlobeOutline(gfx.Color{})
	if base.Cap != gfx.CapButt {
		t.Fatalf("底座图标的 Cap = %v，必须是零值 butt（官方那三条没写 linecap）", base.Cap)
	}
	wantedBase := concatGlobeCommands(t, pathGlobeRing, pathGlobeMeridianWest, pathGlobeMeridianEast)
	if !sameGlobeCommands(base.Commands, wantedBase) {
		t.Fatalf("底座图标应当是外圆 + 两条经线的官方原文：%d 条命令，期望 %d 条",
			len(base.Commands), len(wantedBase))
	}

	equator := iconGlobeEquator(gfx.Color{})
	if equator.Cap != gfx.CapSquare {
		t.Fatalf("赤道图标 Cap = %v，官方 stroke-linecap=\"square\"", equator.Cap)
	}
	if !sameGlobeCommands(equator.Commands, globeCommands(t, pathGlobeEquator)) {
		t.Fatalf("赤道图标应当就是官方原文的 pathGlobeEquator（上游修好 square cap 后无需绕法）：%+v", equator.Commands)
	}
	if equator.ViewBox != 16 || equator.Stroke != globeStrokeWidth {
		t.Fatalf("赤道图标 viewBox=%v stroke=%v，官方 16 / %v", equator.ViewBox, equator.Stroke, globeStrokeWidth)
	}
	if len(equator.Layers) != 0 {
		t.Fatalf("赤道图标不该有填充层：%d 层", len(equator.Layers))
	}
}

// ── 像素判据（headless）──────────────────────────────────────────────

// globeTriangleTolerance 是描边三角化允许的差（px）：viva 把 1 单位描边"不画得比
// 1 物理像素细"，14px 图标上实际描边是 1.0px 而不是 0.875px（gfx/icon.go 的
// `snapToHalfDeviceUp`），端点因此最多偏 1/16 px；这个容差正好挡住那点舍入。
const globeTriangleTolerance = 0.25

// globeInk 收集一帧里颜色等于 color、且三个顶点都落在 within 内的三角形的包围盒。
func globeInk(snapshot app.Snapshot, color ui.Color, within gfx.Rect) (gfx.Rect, int) {
	box := gfx.Rect{X: math.MaxFloat32, Y: math.MaxFloat32}
	maxX, maxY := float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	count := 0
	for _, prim := range snapshot.Primitives {
		if prim.Kind != app.TrianglePrimitive || prim.Color != color {
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
	box.Width = maxX - box.X
	box.Height = maxY - box.Y
	return box, count
}

// globeCovers 报告点 point 是否落在某个颜色为 color 的三角形里。
//
// 为什么用探针而不是外接盒：赤道是一条**长条**，它的两个三角形顶点横跨整条线；
// "所有顶点都在窗口内"的筛法会把它们整条排除，只剩经线，量出来的就不是赤道。
func globeCovers(snapshot app.Snapshot, color ui.Color, point gfx.Point) bool {
	for _, prim := range snapshot.Primitives {
		if prim.Kind != app.TrianglePrimitive || prim.Color != color {
			continue
		}
		if triangleCovers(prim.Points[0], prim.Points[1], prim.Points[2], point) {
			return true
		}
	}
	return false
}

// triangleCovers 是重心坐标判据：三个叉积同号（含 0）即点在三角形内。
func triangleCovers(a, b, c, point gfx.Point) bool {
	cross := func(p, q gfx.Point) float32 {
		return (q.X-p.X)*(point.Y-p.Y) - (q.Y-p.Y)*(point.X-p.X)
	}
	d1, d2, d3 := cross(a, b), cross(b, c), cross(c, a)
	negative := d1 < 0 || d2 < 0 || d3 < 0
	positive := d1 > 0 || d2 > 0 || d3 > 0
	return !(negative && positive)
}

// assertGlobeAt 断言 point 处的墨迹覆盖情况。
func assertGlobeAt(t *testing.T, what string, snapshot app.Snapshot, color ui.Color, point gfx.Point, want bool) {
	t.Helper()
	if got := globeCovers(snapshot, color, point); got != want {
		if want {
			t.Fatalf("%s：点 (%.3f,%.3f) 上没有墨迹（应有）", what, point.X, point.Y)
		}
		t.Fatalf("%s：点 (%.3f,%.3f) 上有墨迹（应无）", what, point.X, point.Y)
	}
}

// assertGlobeEdges 逐边比较墨迹（不是只看宽高）：错尺寸/少一枚/没有方帽
// 都会让某一条边先偏出去，四条边同时对上才算对上。
func assertGlobeEdges(t *testing.T, what string, got, want gfx.Rect) {
	t.Helper()
	for _, edge := range []struct {
		name string
		got  float32
		want float32
	}{
		{"左边", got.X, want.X},
		{"上边", got.Y, want.Y},
		{"右边", got.X + got.Width, want.X + want.Width},
		{"下边", got.Y + got.Height, want.Y + want.Height},
	} {
		if diff := math.Abs(float64(edge.got - edge.want)); diff > globeTriangleTolerance {
			t.Fatalf("%s 的墨迹%s = %.4f，应当是 %.4f（差 %.4f > %.2f）\n墨迹=%+v 期望=%+v",
				what, edge.name, edge.got, edge.want, diff, globeTriangleTolerance, got, want)
		}
	}
}

// globeFrame 固定一帧 headless 画面的尺寸与图标盒原点。
const (
	globeFrameWidth  = 240
	globeFrameHeight = 120
	// globeBox 是图标盒的左上角（Column padding 20/20）。
	globeBox = float32(20)
)

// openGlobeFrame 开一帧 headless 画面：在 (20,20) 处画 draw 交进来的东西。
func openGlobeFrame(t *testing.T, draw func(c ui.C)) app.Snapshot {
	t.Helper()
	theme := dscore.Dark()
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{
		Title: "globe-frame", Width: globeFrameWidth, Height: globeFrameHeight, Tree: &gfx.Tree{},
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		page.Draw(canvas, bounds, func(c ui.C) {
			ui.Column(c).PadSides(20, 0, 0, 20).Children(func() { draw(c) })
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless 开窗失败：%v", err)
	}
	t.Cleanup(func() { session.Close() })
	invalidator.Invalidate()
	for i := 0; i < 3; i++ {
		if err := session.Driver().Pump(); err != nil {
			t.Fatalf("headless 泵帧失败：%v", err)
		}
	}
	return session.Snapshot()
}

// globeWholeFrame 是"整帧"那个测量窗口。
func globeWholeFrame() gfx.Rect {
	return gfx.Rect{Width: globeFrameWidth, Height: globeFrameHeight}
}

// globePointAt 把图标盒 (boxX,boxY) 里的 viewBox 坐标换成本帧的像素坐标。
func globePointAt(boxX, boxY, x, y float32) gfx.Point {
	scale := iconSizeTitle / 16
	return gfx.Point{X: boxX + x*scale, Y: boxY + y*scale}
}

// globeGlyphInkAt 是"外圆 + 两条经线"在 (boxX,boxY) 处 14px 的墨迹包围盒。
//
// 三条 path 的端点/控制点就是几何极值（外圆的极值落在四条三次贝塞尔的端点上，
// 经线的极值也在端点），所以几何盒 ± 半个描边就是墨迹盒。
func globeGlyphInkAt(t *testing.T, boxX, boxY float32) gfx.Rect {
	t.Helper()
	scale := iconSizeTitle / 16
	minX, minY, maxX, maxY := globeViewBoxBounds(t, pathGlobeRing,
		pathGlobeMeridianWest, pathGlobeMeridianEast)
	return gfx.Rect{
		X:      boxX + (minX-globeStrokeWidth/2)*scale,
		Y:      boxY + (minY-globeStrokeWidth/2)*scale,
		Width:  (maxX - minX + globeStrokeWidth) * scale,
		Height: (maxY - minY + globeStrokeWidth) * scale,
	}
}

// TestGlobeEquatorInkMatchesTheOfficialEndpoints 是 ② 的第一条，也是 ⑦ 的核心
// 可证伪断言：**只画赤道那一枚**（Cap=square），墨迹在 viewBox 上必须正好是
// [1.84619, 14.1538] × [7.5, 8.5]。
//
// 期望值不是抄来的数字：左边 = 官方端点 2.34619 − StrokeWidth/2、右边 = 13.6538 +
// StrokeWidth/2、上下 = 8 ∓ StrokeWidth/2，全部由常量与官方原文现场算。
func TestGlobeEquatorInkMatchesTheOfficialEndpoints(t *testing.T) {
	theme := dscore.Dark()
	color := theme.LabelSecondary
	icon := iconGlobeEquator(color)
	snapshot := openGlobeFrame(t, func(c ui.C) {
		ui.Icon(c, icon).Size(iconSizeTitle, iconSizeTitle).Color(color)
	})

	scale := iconSizeTitle / 16
	left, right := globeEquatorEnds(t, pathGlobeEquator)
	want := gfx.Rect{
		X:      globeBox + (left.X-globeStrokeWidth/2)*scale,
		Y:      globeBox + (left.Y-globeStrokeWidth/2)*scale,
		Width:  (right.X - left.X + globeStrokeWidth) * scale,
		Height: globeStrokeWidth * scale,
	}
	got, count := globeInk(snapshot, color, globeWholeFrame())
	if count == 0 {
		t.Fatal("赤道图标一个三角形都没画出来（图标没画或颜色没传下去）")
	}
	assertGlobeEdges(t, "赤道图标", got, want)

	// 官方端点的**绝对**读数：左边 1.84619、右边 14.1538（viewBox）。
	for _, corner := range []struct {
		name string
		got  float32
		want float32
	}{
		{"左端", got.X, globeBox + 1.84619*scale},
		{"右端", got.X + got.Width, globeBox + 14.1538*scale},
	} {
		if diff := math.Abs(float64(corner.got - corner.want)); diff > globeTriangleTolerance {
			t.Fatalf("赤道%s = %.4f，官方等价几何是 %.4f（差 %.4f）", corner.name, corner.got, corner.want, diff)
		}
	}
	// 端头是真墨迹：端点内侧有、外侧没有。
	assertGlobeAt(t, "赤道左端内侧", snapshot, color, globePointAt(globeBox, globeBox, 1.9, 8), true)
	assertGlobeAt(t, "赤道左端外侧", snapshot, color, globePointAt(globeBox, globeBox, 1.7, 8), false)
	assertGlobeAt(t, "赤道右端内侧", snapshot, color, globePointAt(globeBox, globeBox, 14.1, 8), true)
	assertGlobeAt(t, "赤道右端外侧", snapshot, color, globePointAt(globeBox, globeBox, 14.3, 8), false)
}

// TestGlobeChipAndTitleStackPaintTheSameInk 是 ③「两处必须视觉一致」：
//
//	芯片：iconGlobe（单枚，赤道用两端等价几何）               ← Painter.IconImage
//	标题：drawTitle 的 ui.Stack（外圆+经线 / 赤道 square 帽）  ← 真的走 drawTitle
//
// 两边都必须 (a) 外圆+经线的墨迹到位，(b) 赤道横贯中线（叠少了一枚就断）。
func TestGlobeChipAndTitleStackPaintTheSameInk(t *testing.T) {
	theme := dscore.Dark()
	color := theme.LabelSecondary

	chip := openGlobeFrame(t, func(c ui.C) {
		ui.Icon(c, iconGlobe(color)).Size(iconSizeTitle, iconSizeTitle).Color(color)
	})
	chipInk, chipCount := globeInk(chip, color, globeWholeFrame())
	if chipCount == 0 {
		t.Fatal("芯片地球一个三角形都没画出来")
	}
	chipBox := gfx.Point{X: globeBox, Y: globeBox}
	assertGlobeEdges(t, "芯片地球", chipInk, globeGlyphInkAt(t, chipBox.X, chipBox.Y))

	// 标题行：真的走 drawTitle（不是复刻它的树）。图标盒位置由标题文字反推：
	// 图标在 Row 里垂直居中（官方 `.tabTitle{align-items:center}`），左边是
	// padding + 图标 + gap（browserTitleGap = 5 + margin-right 4）。
	p := &Plugin{}
	env := &pluginapi.Env{Theme: theme, Data: pluginapi.Snapshot{Cur: "s1"}, RightTabID: "tab1"}
	p.theme = theme
	p.activate(env)
	p.nav.navigate(browserTarget{Kind: kindHTTPS, URL: "https://other.example/landing", Title: "other.example"})
	title := openGlobeFrame(t, func(c ui.C) { p.drawTitle(c, env) })
	titleInk, titleCount := globeInk(title, color, globeWholeFrame())
	if titleCount == 0 {
		t.Fatal("标题行地球一个三角形都没画出来（ui.Stack 里两枚都没画？）")
	}
	var label gfx.Rect
	found := false
	for _, candidate := range title.Labels {
		if candidate.Value == "other.example" {
			label, found = candidate.Bounds, true
		}
	}
	if !found {
		t.Fatal("标题行没有画出 tab 标题")
	}
	titleBox := gfx.Point{
		X: label.X - browserTitleGap - iconSizeTitle,
		Y: label.Y + (label.Height-iconSizeTitle)/2,
	}
	if math.Abs(float64(titleBox.X-globeBox)) > globeTriangleTolerance {
		t.Fatalf("标题图标盒左沿 = %.2f，应当是 padding %.2f", titleBox.X, globeBox)
	}
	assertGlobeEdges(t, "标题行地球", titleInk, globeGlyphInkAt(t, titleBox.X, titleBox.Y))
	// 两处的墨迹在"图标盒坐标"里必须**同一份**（盒子位置不同只是布局差异）：
	// 芯片用两端等价几何、标题行用 CapSquare + 远端半截，落下来的墨迹一样。
	shifted := chipInk
	shifted.X += titleBox.X - chipBox.X
	shifted.Y += titleBox.Y - chipBox.Y
	assertGlobeEdges(t, "芯片 vs 标题行", titleInk, shifted)

	// 赤道横贯中线：这三个点落在**只有赤道**才会画到的位置（外圆在 x≈1.9/14.1，
	// 两条经线在 x≈5.8/10.2，都不碰 4 / 8 / 12）。叠少一枚或漏了等价几何就断。
	for _, x := range []float32{4, 8, 12} {
		assertGlobeAt(t, "芯片地球的赤道", chip, color, globePointAt(chipBox.X, chipBox.Y, x, 8), true)
		assertGlobeAt(t, "标题行的赤道", title, color, globePointAt(titleBox.X, titleBox.Y, x, 8), true)
		// 中线上下各约 1px 是空的（描边只有 1 单位 ≈ 0.875px 带）：探针不是糊的。
		assertGlobeAt(t, "赤道带上方", title, color, globePointAt(titleBox.X, titleBox.Y, x, 6.8), false)
		assertGlobeAt(t, "赤道带下方", chip, color, globePointAt(chipBox.X, chipBox.Y, x, 9.2), false)
	}
}
