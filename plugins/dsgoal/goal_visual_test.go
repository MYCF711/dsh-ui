//go:build darwin || windows || linux

package dsgoal

import (
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

// TestGoalBarHeadlessCapture 给目标条留一帧真实像素。
//
// 判据是**相对位置**，不是"图里有东西"：目标条必须落在输入卡片**上方**——
// 官方那句是 "GoalBar docked above the composer"，而位置是靠 SlotMainComposer
// 里的 Order 5 < 10 实现的。这条断言把它从"顺序对了"落成"像素上真在上方"。
func TestGoalBarHeadlessCapture(t *testing.T) {
	dir := os.Getenv("GOAL_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	theme := dscore.Dark()
	p := &Plugin{theme: theme}
	data := pluginapi.Snapshot{Cur: "s-001"}
	act := pluginapi.Actions{Goal: func() string { return "把 DSH 重写成 Go 版本，界面用 dsh-ui" }}

	tree := &gfx.Tree{}
	feat := tree.Feature("goal-headless")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{
		Title: "goal-bar", Width: 1182, Height: 832, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: data, Act: act, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: 902,
			Geometry: pluginapi.Geometry{Width: size.Width, Height: size.Height, SidebarWidth: 280, MainWidth: 902},
		}
		view.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).FillHeight().Pad(24).Gap(6).Children(func() {
				ui.Spacer(f)
				p.draw(f, env)
				// 占位：输入卡片。目标条该在它上方。
				ui.Row(f).FillWidth().Height(98).Radius(28).Background(theme.InputMajor)
			})
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}

	// 官方 sprite `goal-outline` 与 elevation 描边都会三角化；按 label-tertiary
	// 只聚合 glyph，避免把 36px 卡片的 BorderL1 轮廓误算进图标包围盒。
	var glyph gfx.Rect
	haveGlyph := false
	var card gfx.Rect
	haveCard := false
	var bar gfx.Rect
	haveBar := false
	for _, primitive := range session.Snapshot().Primitives {
		if primitive.Kind == app.TrianglePrimitive && primitive.Color == theme.LabelTertiary {
			for _, point := range primitive.Points {
				if !haveGlyph {
					glyph = gfx.Rect{X: point.X, Y: point.Y}
					haveGlyph = true
					continue
				}
				if point.X < glyph.X {
					glyph.Width += glyph.X - point.X
					glyph.X = point.X
				}
				if point.Y < glyph.Y {
					glyph.Height += glyph.Y - point.Y
					glyph.Y = point.Y
				}
				if point.X > glyph.X+glyph.Width {
					glyph.Width = point.X - glyph.X
				}
				if point.Y > glyph.Y+glyph.Height {
					glyph.Height = point.Y - glyph.Y
				}
			}
		}
		b := primitive.Bounds
		if primitive.Color == theme.InputMajor && primitive.Radius == 28 && b.Height == 98 {
			card, haveCard = b, true
		}
		if primitive.Color == theme.Menu && primitive.Radius == theme.Radii.MD && b.Height == goalBarHeight {
			bar, haveBar = b, true
		}
	}
	if !haveGlyph {
		t.Fatal("真实像素帧里找不到官方 goal-outline 图标（没有任何描边三角形）")
	}
	if !haveCard {
		t.Fatal("真实像素帧里找不到输入卡片占位（98 高 R28）")
	}
	// 图标框（14×14）的左上角 = 墨迹左上角 - 墨迹相对框的偏移（sprite 不贴框边）。
	inkDX, inkDY, inkW, inkH := iconInk(goalOutlineIcon(), goalGlyphSize)
	// 钩住的判据：图标（目标条）整行都在卡片顶边之上。
	if glyph.Y-inkDY+goalGlyphSize >= card.Y {
		t.Errorf("目标条图标底边 y=%v 没在输入卡片 y=%v 上方——\"docked above the composer\" 没做到",
			glyph.Y-inkDY+goalGlyphSize, card.Y)
	}
	if !haveBar {
		t.Fatal("真实像素帧里找不到 36px specific-menu 目标条")
	}
	// `.bar{padding-left:12px}`：以真实目标条左边为锚，不再沿用旧 16px 假设。
	if !nearPx(glyph.X-inkDX, bar.X+goalBarPadLeft, 0.5) {
		t.Errorf("图标框 x=%v，want %v（目标条 x + 官方左内边距 12）", glyph.X-inkDX, bar.X+goalBarPadLeft)
	}
	// 图标是 14×14 的框，但**着墨范围**比框小：官方 sprite 的墨迹落在 viewBox
	// 的 1.5..14.5（13 个单位）里，缩到 14px 后约 11.4px，再加一个描边宽度。
	// 期望值从 sprite 自己的命令里算，不写死像素（写死就成了"按截图调测试"）。
	if !nearInk(glyph.Width, inkW) || !nearInk(glyph.Height, inkH) {
		t.Errorf("图标包围盒 %vx%v，want ≈%vx%v（官方 sprite 墨迹按 size=%v 缩放 + 描边）",
			glyph.Width, glyph.Height, inkW, inkH, goalGlyphSize)
	}

	path := filepath.Join(dir, "goal-bar.png")
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("截图无效 path=%s info=%v err=%v", path, info, err)
	}
	t.Logf("GOAL_VISUAL path=%s bytes=%d glyph=%+v card=%+v", path, info.Size(), glyph, card)
}

// nearPx 是像素级比较：描边在 viva 里按设备像素对齐，坐标会有亚像素级偏移。
func nearPx(got, want, tolerance float32) bool {
	d := got - want
	if d < 0 {
		d = -d
	}
	return d <= tolerance
}

// nearInk 是"着墨尺寸"的比较：曲线描边的外沿与凸包包围盒之间会有 1px 内外的
// 差异（曲线可以在凸包里收缩），所以容差取 1px。
func nearInk(got, want float32) bool {
	d := got - want
	if d < 0 {
		d = -d
	}
	return d <= 1
}
