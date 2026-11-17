//go:build darwin || windows || linux

// panel_capture_test.go —— 命令面板的**真实渲染截图**（给截图比对用）。
//
// 与 `panel_visual_test.go` 的分工：那边在裸 canvas 上量官方几何（面板底边
// y=396、宽 680、圆角 MD…）；这边只多一件事——把同一帧经真 Viva headless
// 渲染成 PNG，让人能一眼看出"画出来是不是那个东西"（阴影有没有被裁掉、
// 圆角有没有切到文字、行高有没有跳档）。
//
// 截图落 `$VR_COMMAND_CAPTURE_DIR`（默认 t.TempDir()）。
package dscommands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// commandCaptureActions 给面板一份**全都有绑定**的动作表，这样列表里每一档
// 命令都真的出现（Action 为 nil 的命令不画，见 commands()）。
func commandCaptureActions() pluginapi.Actions {
	cfg := map[string]string{"ui.sidebarWidth": "280", "ui.rightbarWidth": "420"}
	return pluginapi.Actions{
		NewSession:     func() {},
		CompactSession: func() {},
		ForkSession:    func() {},
		SetPermission:  func(string) {},
		SetTheme:       func(string) {},
		ConfigGet:      func() map[string]string { return cfg },
		ConfigSet:      func(map[string]string) {},
		Notify:         func(string) {},
	}
}

// TestCommandPanelRenderCapture 画一帧真实命令面板并落 PNG。
func TestCommandPanelRenderCapture(t *testing.T) {
	dir := os.Getenv("VR_COMMAND_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const width, height = 920, 832
	p := &Plugin{theme: dscore.Dark(), shown: true}
	act := commandCaptureActions()
	tree := &gfx.Tree{}
	feat := tree.Feature("command-capture")
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{
		Title: "command-capture", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: p.theme, Act: act, Tree: tree, Feat: feat, Canvas: canvas,
			Data:   pluginapi.Snapshot{Cur: "session-1"},
			Bounds: bounds, MainWidth: size.Width,
		}
		canvas.Fill(bounds, p.theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			// 与 panel_visual_test.go 同一个锚点：composer 卡片宽 680、底边 400。
			ui.Column(f).Absolute().Left(20).Top(400).Width(680).Height(36).Children(func() {
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
	for range 2 {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}

	// 官方几何：面板与 composer 同宽（680），底边在锚点上方 4px（y=396）。
	snapshot := session.Snapshot()
	var panel *app.Primitive
	for i := range snapshot.Primitives {
		primitive := snapshot.Primitives[i]
		if primitive.Kind != app.RectanglePrimitive || primitive.Color != p.theme.BgLayer3 {
			continue
		}
		if primitive.Radius != p.theme.Radii.MD || primitive.Bounds.Width != 680 {
			continue
		}
		panel = &primitive
	}
	if panel == nil {
		t.Fatalf("没有找到与 composer 同宽（680）的面板底：%+v", snapshot.Primitives)
	}
	if panel.Bounds.Y+panel.Bounds.Height != 396 {
		t.Fatalf("面板底边 y=%.1f，官方 bottom:100%%+4 应为 396", panel.Bounds.Y+panel.Bounds.Height)
	}
	joined := strings.Join(session.Texts(), "\u0000")
	for _, want := range []string{"侧栏宽一点", "权限：每次都问我", "主题：深色", "压缩上下文"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("命令面板缺 %q；文本=%q", want, session.Texts())
		}
	}

	t.Logf("VR_COMMAND_GEOM panel=%+v rows=%d", panel.Bounds, len(session.Texts()))
	path := filepath.Join(dir, "command-panel.png")
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	t.Logf("VR_COMMAND_CAPTURE path=%s", path)
}
