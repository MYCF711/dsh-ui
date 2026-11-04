//go:build darwin || windows || linux

package dsgoal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
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

	tree := &ui.Tree{}
	feat := tree.Feature("goal-headless")
	var view flex.View
	var invalidator app.Invalidator
	config := app.Config{
		Title: "goal-bar", Width: 1182, Height: 832, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: data, Act: act, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: 902,
			Geometry: pluginapi.Geometry{Width: size.Width, Height: size.Height, SidebarWidth: 280, MainWidth: 902},
		}
		view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f *flex.Frame) {
			f.Column().FillHeight().Pad(24).Gap(6).Children(func() {
				f.Spacer()
				p.draw(f, env)
				// 占位：输入卡片。目标条该在它上方。
				f.Row().FillWidth().Height(98).Radius(28).Background(theme.InputMajor)
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

	var dot, card ui.Rect
	haveDot, haveCard := false, false
	for _, primitive := range session.Snapshot().Primitives {
		b := primitive.Bounds
		if b.Width == 4 && b.Height == 4 {
			dot, haveDot = b, true
		}
		if primitive.Color == theme.InputMajor && primitive.Radius == 28 && b.Height == 98 {
			card, haveCard = b, true
		}
	}
	if !haveDot {
		t.Fatal("真实像素帧里找不到 4×4 的目标圆点")
	}
	if !haveCard {
		t.Fatal("真实像素帧里找不到输入卡片占位（98 高 R28）")
	}
	// 钩住的判据：圆点（目标条）整行都在卡片顶边之上。
	if dot.Y >= card.Y {
		t.Errorf("目标条圆点 y=%v 没在输入卡片 y=%v 上方——\"docked above the composer\" 没做到", dot.Y, card.Y)
	}
	// 左边距 16（PadSides(6,16,6,16) 的 left）：这条以前是 6，写串过。
	if dot.X != 24+16 {
		t.Errorf("圆点 x=%v，want 40（外层 Pad 24 + 本行左内边距 16）", dot.X)
	}

	path := filepath.Join(dir, "goal-bar.png")
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("截图无效 path=%s info=%v err=%v", path, info, err)
	}
	t.Logf("GOAL_VISUAL path=%s bytes=%d dot=%+v card=%+v", path, info.Size(), dot, card)
}
