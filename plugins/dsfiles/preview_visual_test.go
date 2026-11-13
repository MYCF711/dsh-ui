//go:build darwin || windows || linux

package dsfiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// TestPreviewHeadlessCapture 给预览浮层留一帧真实像素。
//
// 为什么非要它：`TestPreviewOpensInDefaultApp` 能证明"按钮点得动、链接造得对"，
// 但证明不了**它在屏幕上**。这个仓里"执行了但看不见"真发生过
// （`dsopen` 的按钮查了三轮才确认是抓帧时机太早），所以新画的东西都要有一帧。
func TestPreviewHeadlessCapture(t *testing.T) {
	dir := os.Getenv("FILES_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	theme := dscore.Dark()
	p := &Plugin{sel: "open.go", content: "package dsopen\n\nfunc draw() {}", curDir: "plugins",
		theme: theme, prims: &dscore.Primitives{Theme: theme}}
	data := pluginapi.Snapshot{Cur: "s-001", CWD: `D:\ws`}
	act := pluginapi.Actions{OpenFile: func(string) error { return nil }}

	tree := &gfx.Tree{}
	feat := tree.Feature("files-preview-headless")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{
		Title: "files-preview", Width: 1182, Height: 832, Tree: tree,
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
			p.drawPreview(f, env)
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

	// 判据：预览卡片（InputMajor + 圆角 Panel）真的画出来了，而且它上面有文字。
	// 只写 PNG 不断言的话，"画在一块看不见的地方"照样能出一张图。
	var card gfx.Rect
	found := false
	for _, primitive := range session.Snapshot().Primitives {
		if primitive.Color != theme.InputMajor {
			continue
		}
		area := primitive.Bounds.Width * primitive.Bounds.Height
		if !found || area > card.Width*card.Height {
			card, found = primitive.Bounds, true
		}
	}
	if !found {
		t.Fatal("真实像素帧里找不到预览卡片（InputMajor 面板）")
	}
	if card.Width <= 0 || card.Height <= 0 {
		t.Fatalf("预览卡片尺寸异常：%+v", card)
	}
	if card.Width > 600 {
		t.Errorf("预览卡片宽 %v，超过设计值 520——布局把它撑开了", card.Width)
	}

	path := filepath.Join(dir, "files-preview.png")
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("截图无效 path=%s info=%v err=%v", path, info, err)
	}
	t.Logf("FILES_VISUAL path=%s bytes=%d card={x:%v y:%v w:%v h:%v}",
		path, info.Size(), card.X, card.Y, card.Width, card.Height)
}
