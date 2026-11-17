//go:build darwin || windows || linux

package dssidebar

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dsbrand"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// TestFG2HeadlessTwoWindowSizes 在同一真实 Viva 无头窗口中缩放并渲染 PNG。
// 默认写临时目录；FG2_CAPTURE_DIR 指向 testdata 时保留交付实图。
func TestFG2HeadlessTwoWindowSizes(t *testing.T) {
	dir := os.Getenv("FG2_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	kernel := uikernel.New("fg2-headless")
	sidebar := &Plugin{}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dsshell.Plugin{}, sidebar); err != nil {
		t.Fatal(err)
	}
	defer kernel.Dispose()
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("fg2-headless")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{Title: "fg2-sidebar", Width: 1920, Height: 1032, Tree: tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		mainWidth := size.Width - 280
		env := &pluginapi.Env{
			Theme: theme,
			Data: pluginapi.Snapshot{
				Sessions: mockdSessions(), Cur: "s-002", Busy: true,
			},
			Act:  pluginapi.Actions{NewSession: func() {}, OpenSession: func(string) {}},
			Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds, MainWidth: mainWidth,
			Geometry: pluginapi.Geometry{
				Width: size.Width, Height: size.Height, SidebarWidth: 280, MainWidth: mainWidth,
			},
			Has: kernel.Has,
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			kernel.DrawSlot(f, pluginapi.SlotShellRoot, env)
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	capture := func(width, height int, name string) {
		t.Helper()
		if err := session.Resize(width, height); err != nil {
			t.Fatal(err)
		}
		snapshot := session.Snapshot()
		if snapshot.Width != width || snapshot.Height != height {
			t.Fatalf("场景尺寸=%dx%d，want %dx%d", snapshot.Width, snapshot.Height, width, height)
		}
		// 品牌名（deepseek / HARNESS）已按官方做成矢量，不是文字
		// （brand-official/src/client/Brand.tsx:17-19 的 `<BrandWordmark includeMark={false} />`），
		// 所以它们从「必须有文字」的清单里移出，改由下面两条**反向**断言守住：
		// 它们一旦重新变成文字，说明有人把矢量实现回退了。
		for _, text := range []string{"新会话", "插件", "自动化任务", "工作区"} {
			if !session.HasText(text) {
				t.Errorf("%dx%d 缺少真实绘制文字 %q；texts=%q", width, height, text, session.Texts())
			}
		}
		for _, text := range []string{"deepseek", "HARNESS"} {
			if session.HasText(text) {
				t.Errorf("%dx%d 品牌名不该是文字 %q（官方是 BrandWordmark 矢量）；texts=%q", width, height, text, session.Texts())
			}
		}
		// 品牌行确实画了矢量：鱼标 + wordmark 字形 + 徽标字形都是纯填充路径。
		if len(dsbrand.BrandNameIcon().Commands) == 0 || len(dsbrand.BrandBadgeInkIcon().Commands) == 0 {
			t.Fatalf("%dx%d 品牌 wordmark 矢量是空的", width, height)
		}
		// 矢量在 app.Snapshot 里是 TrianglePrimitive（driver.go:988-989），不是 canvas。
		// 数一数以字面的确被光栅化成三角形——空的话就是品牌行整块没画出来。
		triangles := 0
		for _, prim := range session.Snapshot().Primitives {
			if prim.Kind == app.TrianglePrimitive {
				triangles++
			}
		}
		if triangles == 0 {
			t.Errorf("%dx%d 品牌行没有画出任何矢量（鱼标 + wordmark 应产生三角形）", width, height)
		}
		for _, text := range []string{"主题鱼", "上下文压缩", "更多"} {
			if session.HasText(text) {
				t.Errorf("%dx%d 没有扩展/动作却画出 %q", width, height, text)
			}
		}

		boxes := sidebar.frameBoxes
		assertRect := func(name string, got gfx.Rect, want gfx.Rect) {
			t.Helper()
			values := [][4]float32{{got.X, want.X, 0, 0}, {got.Y, want.Y, 0, 0}, {got.Width, want.Width, 0, 0}, {got.Height, want.Height, 0, 0}}
			for _, v := range values {
				if math.Abs(float64(v[0]-v[1])) > 8 {
					t.Errorf("%s 几何=%+v，官方推导=%+v（偏差>8px）", name, got, want)
					return
				}
			}
		}
		assertRect("logoRow", boxes.Brand.Rect(), gfx.Rect{X: 8, Y: 6, Width: 264, Height: 40})
		assertRect("newSession", boxes.NewSession.Rect(), gfx.Rect{X: 12, Y: 52, Width: 256, Height: 38})
		if len(boxes.PanelRows) != 2 {
			t.Fatalf("导航行=%d，want 2", len(boxes.PanelRows))
		} else {
			assertRect("plugins panelRow", boxes.PanelRows[0].Rect(), gfx.Rect{X: 11, Y: 102, Width: 258, Height: 32})
			assertRect("schedules panelRow", boxes.PanelRows[1].Rect(), gfx.Rect{X: 11, Y: 136, Width: 258, Height: 32})
		}
		assertRect("workspace sectionHeader", boxes.WorkspaceHeader.Rect(), gfx.Rect{X: 8, Y: 178, Width: 268, Height: 36})
		searchButton := boxes.SearchButton.Rect()
		assertRect("workspace search button", searchButton, gfx.Rect{X: searchButton.X, Y: searchButton.Y, Width: 28, Height: 28})

		path := filepath.Join(dir, name)
		if err := session.Screenshot(path); err != nil {
			t.Fatal(err)
		}
		picture, err := session.Image()
		if err != nil {
			t.Fatal(err)
		}
		if picture.Bounds().Dx() != width || picture.Bounds().Dy() != height {
			t.Fatalf("PNG 尺寸=%v，want %dx%d", picture.Bounds(), width, height)
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("实图无效：path=%s info=%v err=%v", path, info, err)
		}
		t.Logf("FG2_CAPTURE path=%s size=%dx%d bytes=%d brand=%+v new=%+v panels=%+v workspace=%+v",
			path, width, height, info.Size(), boxes.Brand.Rect(), boxes.NewSession.Rect(),
			[]gfx.Rect{boxes.PanelRows[0].Rect(), boxes.PanelRows[1].Rect()}, boxes.WorkspaceHeader.Rect())
	}

	capture(1920, 1032, "frame-1920x1032.png")
	capture(1182, 832, "frame-1182x832.png")
}
