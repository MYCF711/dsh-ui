//go:build darwin || windows || linux

package dscomposer

import (
	"fmt"
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

// TestControlsHeadlessGeometryAndCapture 用真实 Viva 文本测量、矢量光栅化和 flex
// 布局验收两档窗口。COMPOSER_CAPTURE_DIR 非空时 PNG 留在指定证据目录；
// 默认仍会在临时目录实画，不把“测试环境没设截图目录”当作跳过几何断言的理由。
func TestControlsHeadlessGeometryAndCapture(t *testing.T) {
	dir := os.Getenv("COMPOSER_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		width      int
		height     int
		mainWidth  float32
		active     bool
		busy       bool
		wantCardW  float32
		wantCardH  float32
		wantLocalX float32
	}{
		{name: "1920-hero", width: 1920, height: 1032, mainWidth: 1640, wantCardW: 952, wantCardH: 114, wantLocalX: 332},
		{name: "1920-active", width: 1920, height: 1032, mainWidth: 1640, active: true, wantCardW: 952, wantCardH: 98, wantLocalX: 332},
		{name: "1182-hero", width: 1182, height: 832, mainWidth: 902, wantCardW: 712, wantCardH: 114, wantLocalX: 83},
		{name: "1182-active", width: 1182, height: 832, mainWidth: 902, active: true, wantCardW: 712, wantCardH: 98, wantLocalX: 83},
		{name: "1182-stop", width: 1182, height: 832, mainWidth: 902, active: true, busy: true, wantCardW: 712, wantCardH: 98, wantLocalX: 83},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			theme := dscore.Dark()
			p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
			data := pluginapi.Snapshot{}
			if tc.active {
				data = pluginapi.Snapshot{Cur: "s-001", Model: "deepseek-v4-pro", Permission: "ask", Busy: tc.busy}
				p.field.SetText("继续")
			}
			act := pluginapi.Actions{Send: func(string) {}, Cancel: func() {}}
			tree := &ui.Tree{}
			feat := tree.Feature("composer-headless-" + tc.name)
			var view flex.View
			var invalidator app.Invalidator
			config := app.Config{
				Title: "composer-" + tc.name, Width: tc.width, Height: tc.height, Tree: tree,
				WindowMode: app.WindowFrameless, Invalidator: &invalidator,
			}
			config.Paint = func(canvas *ui.Canvas, size ui.Size) {
				bounds := ui.Rect{Width: size.Width, Height: size.Height}
				canvas.Fill(bounds, theme.BgBase)
				env := &pluginapi.Env{
					Theme: theme, Data: data, Act: act, Tree: tree, Feat: feat, Canvas: canvas,
					Bounds: bounds, MainWidth: tc.mainWidth,
					Geometry: pluginapi.Geometry{Width: size.Width, Height: size.Height, SidebarWidth: 280, MainWidth: tc.mainWidth},
				}
				view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f *flex.Frame) {
					// 与 composer_test 一样模拟 dsshell 的中栏内容框和底部 12px。
					f.Column().Width(tc.mainWidth-24).FillHeight().PadSides(0, 0, 12, 0).Children(func() {
						f.Spacer()
						p.draw(f, env)
					})
				})
				p.afterDraw(canvas, env)
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

			var card app.Primitive
			found := false
			for _, primitive := range session.Snapshot().Primitives {
				if primitive.Color != theme.InputMajor || primitive.Radius != 28 {
					continue
				}
				area := primitive.Bounds.Width * primitive.Bounds.Height
				if !found || area > card.Bounds.Width*card.Bounds.Height {
					card, found = primitive, true
				}
			}
			if !found {
				t.Fatal("真实像素帧里找不到 R28 composer 卡片")
			}
			if card.Bounds.Width != tc.wantCardW || card.Bounds.Height != tc.wantCardH || card.Bounds.X != tc.wantLocalX {
				t.Errorf("卡片几何=%+v，want x=%v w=%v h=%v", card.Bounds, tc.wantLocalX, tc.wantCardW, tc.wantCardH)
			}
			primary := p.controls.Primary.Rect()
			if primary.Width != 34 || primary.Height != 34 {
				t.Errorf("主按钮真实布局=%+v，官方 34×34", primary)
			}
			if gap := card.Bounds.X + card.Bounds.Width - (primary.X + primary.Width); gap != 8 {
				t.Errorf("主按钮右距=%v，官方 8", gap)
			}
			if gap := card.Bounds.Y + card.Bounds.Height - (primary.Y + primary.Height); gap != 8 {
				t.Errorf("主按钮底距=%v，官方 8（含 translateY(-2) 等效位移）", gap)
			}
			if tc.active {
				if got := p.controls.Permission.Rect(); got.Height != 28 {
					t.Errorf("权限 trigger 真实布局高=%v，官方 28", got.Height)
				}
				if got := p.controls.Model.Rect(); got.Height != 28 {
					t.Errorf("模型 trigger 真实布局高=%v，官方 28", got.Height)
				}
			}

			path := filepath.Join(dir, fmt.Sprintf("visual-v4-%s.png", tc.name))
			if err := session.Screenshot(path); err != nil {
				t.Fatal(err)
			}
			picture, err := session.Image()
			if err != nil {
				t.Fatal(err)
			}
			if picture.Bounds().Dx() != tc.width || picture.Bounds().Dy() != tc.height {
				t.Fatalf("实图尺寸=%v，want %dx%d", picture.Bounds(), tc.width, tc.height)
			}
			info, err := os.Stat(path)
			if err != nil || info.Size() == 0 {
				t.Fatalf("截图文件无效 path=%s info=%v err=%v", path, info, err)
			}
			t.Logf("COMPOSER_VISUAL path=%s bytes=%d card={x:%v y:%v w:%v h:%v r:28} primary={w:%v h:%v right:8 bottom:8}",
				path, info.Size(), card.Bounds.X, card.Bounds.Y, card.Bounds.Width, card.Bounds.Height, primary.Width, primary.Height)
		})
	}
}
