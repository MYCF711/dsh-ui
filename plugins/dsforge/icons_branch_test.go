//go:build darwin || windows || linux

// icons_branch_test.go —— 分支图标"真的被画出来"的**像素判据**（Viva headless）。
//
// 账本 #64 ④ 原来的差异是"素材有、画不出来"；`plugins/dsassets` 把 SVG 椭圆弧换算成
// 三次贝塞尔之后（`plugins/dsassets/arc.go`），这枚图标可以精确画了。这里钉住三件事：
//
//	① 卡片/芯片的头部**有墨迹**，且墨迹不是文字（图标所在矩形里一个 Label 都没有）；
//	② 墨迹的**尺寸与位置**逐项等于官方：14px 盒（WorktreeProgress.tsx:156,221 /
//	   WorktreeToggle.tsx:146）、盒与文字的间距 8px / 6px（WorktreeProgress.module.css:54-60 /
//	   WorktreeToggle.module.css:11-29）；
//	③ 墨迹就是 `dsassets.Build(dsassets.Branch, …)` 的产物（三角化条数逐点相同），
//	   不是另画的近似形状——官方素材见 `assets/official/icons/messages/branch.jsx`。
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

// 官方 `IconBranchOutlineRegular`（`assets/official/icons/messages/branch.jsx`）的几何范围
// （viewBox 16 单位）：
//
//	x 左端 = 第二条支线的起点 1.01221（第一条是 1.01503，取更左的那条）
//	x 右端 = 节点圆外沿 cx+r = 12.4502 + 1.56962 = 14.01982
//	y 上端 = cy-r = 3.3079 - 1.56962 = 1.73828
//	y 下端 = cy+r = 12.6921 + 1.56962 = 14.26172
//
// 描边 1（官方 `ICON_REGULAR_STROKE`，`dsassets.regularStroke`）在两侧各外扩 0.5；
// 左端是水平的 butt 端头，x 不外扩。
const (
	branchInkLeft   = float64(1.01221)
	branchInkRight  = float64(12.4502 + 1.56962)
	branchInkTop    = float64(3.3079 - 1.56962)
	branchInkBottom = float64(12.6921 + 1.56962)
	branchInkStroke = float64(1)
)

// expectedBranchInk 给出"一枚 size 大小的官方分支图标"应当落下的墨迹包围盒。
//
// 图标把 viewBox 16 等比缩到 size；墨迹 = 几何范围 ± 半个描边（左端除外，butt 端头）。
func expectedBranchInk(boxX, boxY, size float32) gfx.Rect {
	scale := float64(size) / 16
	x0 := float64(boxX) + branchInkLeft*scale
	y0 := float64(boxY) + (branchInkTop-branchInkStroke/2)*scale
	x1 := float64(boxX) + (branchInkRight+branchInkStroke/2)*scale
	y1 := float64(boxY) + (branchInkBottom+branchInkStroke/2)*scale
	return gfx.Rect{X: float32(x0), Y: float32(y0), Width: float32(x1 - x0), Height: float32(y1 - y0)}
}

// triangleInk 收集一帧里颜色等于 color、且三个顶点都落在 within 内的三角形的包围盒。
//
// 为什么按颜色+区域筛：官方这两个位置的分支图标都用 `.glyph`/芯片的 ink（label-tertiary），
// 同色的其它图元（running 步骤的 StateDot）落在别的 Y 带里，不会被算进来。
func triangleInk(snapshot app.Snapshot, color ui.Color, within gfx.Rect) (gfx.Rect, int) {
	box := gfx.Rect{X: math.MaxFloat32, Y: math.MaxFloat32}
	maxX, maxY := float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	count := 0
	for _, prim := range snapshot.Primitives {
		if prim.Kind != app.TrianglePrimitive || prim.Color != color {
			continue
		}
		inside := true
		for _, p := range prim.Points {
			if p.X < within.X || p.X > within.X+within.Width ||
				p.Y < within.Y || p.Y > within.Y+within.Height {
				inside = false
			}
		}
		if !inside {
			continue
		}
		count++
		for _, p := range prim.Points {
			if p.X < box.X {
				box.X = p.X
			}
			if p.Y < box.Y {
				box.Y = p.Y
			}
			if p.X > maxX {
				maxX = p.X
			}
			if p.Y > maxY {
				maxY = p.Y
			}
		}
	}
	box.Width = maxX - box.X
	box.Height = maxY - box.Y
	return box, count
}

// labelsInside 返回落在 rect 里的文字（用来证明图标那块矩形"没有文字"）。
// labelInk 找一段本帧画出来的文字的包围盒（精确匹配整段文字）。
func labelInk(snapshot app.Snapshot, value string) (gfx.Rect, bool) {
	for _, label := range snapshot.Labels {
		if label.Value == value {
			return label.Bounds, true
		}
	}
	return gfx.Rect{}, false
}

func labelsInside(snapshot app.Snapshot, rect gfx.Rect) []string {
	var values []string
	for _, label := range snapshot.Labels {
		if label.Bounds.X >= rect.X && label.Bounds.X <= rect.X+rect.Width &&
			label.Bounds.Y >= rect.Y && label.Bounds.Y <= rect.Y+rect.Height {
			values = append(values, label.Value)
		}
	}
	return values
}

// assertBranchInk 断言一组墨迹等于 expected，且允许 viva 描边三角化带来的半像素级误差。
//
// 判据是**逐边**的（不是只看宽高）：错尺寸（12px 或 16px）会差 1.2–1.7px，
// 手画圆点会连包围盒形状都对不上，只有真的按官方素材画才会四条边同时落在这里。
func assertBranchInk(t *testing.T, what string, got gfx.Rect, want gfx.Rect) {
	t.Helper()
	const tolerance = 0.25
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
		if diff := math.Abs(float64(edge.got - edge.want)); diff > tolerance {
			t.Fatalf("%s 的墨迹%s = %.4f，官方算出来是 %.4f（差 %.4f > %.2f）\n墨迹框=%+v 期望=%+v",
				what, edge.name, edge.got, edge.want, diff, tolerance, got, want)
		}
	}
}

// TestBranchIconIsCompiledByDsassets 钉住"不许手画"：图标只能来自 dsassets.Build。
//
// 官方 `IconBranchOutlineRegular` 是两条 `<path>` + 两枚 `<circle r=1.56962>`；
// `dsassets` 换算后是 24 条描边命令（12 条支线 + 每枚圆 6 条：M + 4C + Z）、viewBox 16、
// 描边 1（`plugins/dsassets/registry.go` 的 `Branch`）。
func TestBranchIconIsCompiledByDsassets(t *testing.T) {
	icon, ok := branchIcon()
	if !ok {
		t.Fatal("branchIcon 编译失败：分支图标又回到「画不出来」了")
	}
	if icon.ViewBox != 16 || icon.Stroke != 1 {
		t.Fatalf("分支图标 viewBox=%v stroke=%v，官方是 16 / 1", icon.ViewBox, icon.Stroke)
	}
	if len(icon.Commands) != 24 {
		t.Fatalf("分支图标命令数 = %d，官方素材换算后是 24 条（账本 #64 ④ 的实测值）", len(icon.Commands))
	}
	if len(icon.Layers) != 0 {
		t.Fatalf("分支图标不该有填充层（纯描边），实得 %d 层", len(icon.Layers))
	}
}

// TestProgressHeaderDrawsTheOfficialBranchGlyph 是创建进度卡的一帧像素判据。
//
// 官方 `WorktreeProgress.tsx:156`：`<IconBranchOutlineRegular size={14} className={css.glyph} />`
// 是 `.header` 的第一个孩子，后面才是标题与分支名；`.header{display:flex;align-items:center;
// gap:8px;padding-right:4px}`（WorktreeProgress.module.css:54-60）、`.glyph{flex:none;
// color:label-tertiary}`（同文件 :62-65）。
func TestProgressHeaderDrawsTheOfficialBranchGlyph(t *testing.T) {
	view := creationView(runningCreation(pluginapi.ForgeCreationPrepare, true))
	f := newForgeCardsFixture(t, view, "s-1", drawProgressOnly)
	snapshot := f.session.Snapshot()

	title, ok := labelInk(snapshot, copyCreationTitle)
	if !ok {
		t.Fatalf("进度卡没有画出标题 %q", copyCreationTitle)
	}
	branch, ok := labelInk(snapshot, "dsh/feat-base")
	if !ok {
		t.Fatal("进度卡没有画出分支名")
	}

	// 官方顺序：图标 → 标题 → 分支名，图标盒左沿 = 标题左沿 - gap(8) - 14。
	boxX := title.X - progressHeaderGap - progressGlyphSize
	boxY := title.Y + (title.Height-progressGlyphSize)/2
	want := expectedBranchInk(boxX, boxY, progressGlyphSize)

	got, count := triangleInk(snapshot, f.theme.LabelTertiary, gfx.Rect{
		X: boxX - 2, Y: boxY - 2, Width: progressGlyphSize + 4, Height: progressGlyphSize + 4,
	})
	if count == 0 {
		t.Fatalf("创建进度卡头部没有分支图标的墨迹（%d 个三角形）：图标没画出来", count)
	}
	assertBranchInk(t, "创建进度卡头像标", got, want)

	// 不是文字：图标那块 14×14 的矩形里一个 Label 都没有；标题与分支名都在它右边。
	if texts := labelsInside(snapshot, gfx.Rect{X: boxX, Y: boxY, Width: progressGlyphSize, Height: progressGlyphSize}); len(texts) != 0 {
		t.Fatalf("图标所在矩形里出现了文字 %v：那不是画出来的图标", texts)
	}
	if boxX+progressGlyphSize+progressHeaderGap != title.X {
		t.Fatalf("图标与标题的间距不是官方 %.0fpx：图标右沿 %.2f，标题左沿 %.2f",
			progressHeaderGap, boxX+progressGlyphSize, title.X)
	}
	if branch.X <= title.X {
		t.Fatalf("分支名没有排在标题后面：branch.X=%.2f title.X=%.2f", branch.X, title.X)
	}

	// 同一张卡的另一处官方用法（`.glyph`，WorktreeProgress.tsx:221 恢复卡）共用
	// 同一个 helper，所以这里只钉创建卡；恢复卡在本仓还没画（进度卡只画 running 一支）。
}

// TestWorktreeChipDrawsTheOfficialBranchGlyph 是 hero 分支芯片的像素判据。
//
// 官方 `WorktreeToggle.tsx:146-147`：`.current` 按钮里先画 `<IconBranchOutlineRegular
// size={14} />`、再画分支名；`.toggle,.base,.current{gap:6px}`（WorktreeToggle.module.css:11-29）、
// `.current svg{flex:none}`（同文件 :55-57）。
func TestWorktreeChipDrawsTheOfficialBranchGlyph(t *testing.T) {
	view := creationView(pluginapi.ForgeWorktree{PanelKnown: true, Editable: true})
	f := newForgeCardsFixture(t, view, "s-1", drawToggleOnly)
	snapshot := f.session.Snapshot()

	chip, ok := labelInk(snapshot, "当前分支：feat/base")
	if !ok {
		t.Fatal("分支芯片没有画出分支名")
	}
	boxX := chip.X - worktreeChipGap - worktreeChipGlyphSize
	boxY := chip.Y + (chip.Height-worktreeChipGlyphSize)/2
	want := expectedBranchInk(boxX, boxY, worktreeChipGlyphSize)

	got, count := triangleInk(snapshot, f.theme.LabelTertiary, gfx.Rect{
		X: boxX - 2, Y: boxY - 2, Width: worktreeChipGlyphSize + 4, Height: worktreeChipGlyphSize + 4,
	})
	if count == 0 {
		t.Fatalf("分支芯片里没有分支图标的墨迹（%d 个三角形）", count)
	}
	assertBranchInk(t, "分支芯片图标", got, want)
	if texts := labelsInside(snapshot, gfx.Rect{X: boxX, Y: boxY, Width: worktreeChipGlyphSize, Height: worktreeChipGlyphSize}); len(texts) != 0 {
		t.Fatalf("芯片图标所在矩形里出现了文字 %v：那不是画出来的图标", texts)
	}
}

// TestBranchGlyphEqualsTheCompiledAsset 证明画出来的墨迹**就是** dsassets.Build 的产物：
// 把它单独画在同一尺寸的空页面上，三角化条数与相对包围盒必须与卡片里那枚逐项相同。
//
// 这是"不是手画"的第二道判据：手画的近似圆/圆点不会复现官方素材的 164 个三角形，
// 也不会在四条边上同时与 `cx±r / cy±r` 的换算值重合。
func TestBranchGlyphEqualsTheCompiledAsset(t *testing.T) {
	view := creationView(runningCreation(pluginapi.ForgeCreationPrepare, true))
	f := newForgeCardsFixture(t, view, "s-1", drawProgressOnly)
	snapshot := f.session.Snapshot()
	title, ok := labelInk(snapshot, copyCreationTitle)
	if !ok {
		t.Fatal("进度卡没有画出标题")
	}
	boxX := title.X - progressHeaderGap - progressGlyphSize
	boxY := title.Y + (title.Height-progressGlyphSize)/2
	cardInk, cardCount := triangleInk(snapshot, f.theme.LabelTertiary, gfx.Rect{
		X: boxX - 2, Y: boxY - 2, Width: progressGlyphSize + 4, Height: progressGlyphSize + 4,
	})
	if cardCount == 0 {
		t.Fatal("卡片里没有可比的图标墨迹")
	}

	standaloneInk, standaloneCount := renderBranchIconAlone(t, progressGlyphSize)
	if standaloneCount != cardCount {
		t.Fatalf("卡片里的图标三角化 %d 条，dsassets 编译产物单独画是 %d 条：画的不是同一枚图标",
			cardCount, standaloneCount)
	}
	if diff := math.Abs(float64(standaloneInk.Width - cardInk.Width)); diff > 0.01 {
		t.Fatalf("卡片图标宽 %.4f，编译产物单独画宽 %.4f（差 %.4f）", cardInk.Width, standaloneInk.Width, diff)
	}
	if diff := math.Abs(float64(standaloneInk.Height - cardInk.Height)); diff > 0.01 {
		t.Fatalf("卡片图标高 %.4f，编译产物单独画高 %.4f（差 %.4f）", cardInk.Height, standaloneInk.Height, diff)
	}
}

// renderBranchIconAlone 把 `branchIcon()` 单独画在一张空页面的 (20,20) 处，返回墨迹框。
func renderBranchIconAlone(t *testing.T, size float32) (gfx.Rect, int) {
	t.Helper()
	icon, ok := branchIcon()
	if !ok {
		t.Fatal("branchIcon 编译失败")
	}
	theme := dscore.Dark()
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{
		Title: "branch-icon", Width: 200, Height: 200, Tree: &gfx.Tree{},
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, sz ui.Size) {
		bounds := gfx.Rect{Width: sz.Width, Height: sz.Height}
		canvas.Fill(bounds, theme.BgBase)
		page.Draw(canvas, bounds, func(c ui.C) {
			ui.Column(c).PadSides(20, 0, 0, 20).Children(func() {
				ui.Icon(c, icon).Size(size, size).Color(theme.LabelTertiary)
			})
		})
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
	return triangleInk(session.Snapshot(), theme.LabelTertiary, gfx.Rect{X: 0, Y: 0, Width: 200, Height: 200})
}
