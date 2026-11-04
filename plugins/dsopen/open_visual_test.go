//go:build darwin || windows || linux

package dsopen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// TestOpenButtonHeadlessCapture 给「打开」按钮留一帧真实像素。
//
// 这个部件的全部历史都是"看得见吗"：做出来之后连续三轮在截图里找不到
// （先怀疑槽位没画、再怀疑高度塌陷，最后确认是抓帧时机太早——
// CWD 要等 session.load 回来才填）。布局断言能证明"画了、56×24、在哪"，
// 但证明不了它**被真的光栅化出来、颜色和字对**。所以这一帧不能省。
func TestOpenButtonHeadlessCapture(t *testing.T) {
	dir := os.Getenv("OPEN_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	theme := dscore.Dark()
	p := &Plugin{theme: theme}
	data := pluginapi.Snapshot{Cur: "s-001", CWD: `D:\ws`}
	act := pluginapi.Actions{OpenExternal: func(string) error { return nil }}

	tree := &ui.Tree{}
	feat := tree.Feature("open-headless")
	var view flex.View
	var invalidator app.Invalidator
	config := app.Config{
		Title: "open-in-app", Width: 1182, Height: 832, Tree: tree,
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
			// 模拟会话头部那一行：靠右。
			f.Column().Pad(24).Children(func() {
				f.Row().FillWidth().Justify(flex.End).Children(func() { p.draw(f, env) })
				f.Spacer()
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

	var button ui.Rect
	found := false
	for _, primitive := range session.Snapshot().Primitives {
		if primitive.Bounds.Width == 56 && primitive.Bounds.Height == 24 {
			button, found = primitive.Bounds, true
			break
		}
	}
	if !found {
		t.Fatal("真实像素帧里找不到 56×24 的「打开」按钮")
	}
	// 右边距 24 是外层 Pad(24)：按钮贴右，不该跑出可视区。
	if right := float32(1182) - (button.X + button.Width); right != 24 {
		t.Errorf("按钮右边距 %v，want 24（外层 Pad）", right)
	}

	path := filepath.Join(dir, "open-in-app.png")
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("截图无效 path=%s info=%v err=%v", path, info, err)
	}
	t.Logf("OPEN_VISUAL path=%s bytes=%d button=%+v", path, info.Size(), button)
}
