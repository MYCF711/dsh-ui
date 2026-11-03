//go:build darwin || windows || linux

package dsrightbar

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// TestV4AHeadlessResizeAndScreenshots 在同一个无头窗口里驱动真实 shell+rightbar。
// 设置 V4A_CAPTURE_DIR 时保留 PNG；否则也在临时目录完成相同的渲染和尺寸断言。
func TestV4AHeadlessResizeAndScreenshots(t *testing.T) {
	dir := os.Getenv("V4A_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	kernel := uikernel.New("v4a-headless")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dsshell.Plugin{}, &Plugin{}); err != nil {
		t.Fatal(err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &ui.Tree{}
	feat := tree.Feature("v4a-headless")
	var view flex.View
	config := app.Config{Title: "v4a-shell-rightbar", Width: 1920, Height: 1032, Tree: tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		rightPref := size.Width * 0.45
		env := &pluginapi.Env{
			Theme: theme,
			Data: pluginapi.Snapshot{
				Cur: "v4a-visual", ShowRightbar: true,
				Msgs: []apiclient.Message{
					{Role: "tool", Tool: "bash", Summary: "CGO_ENABLED=0 go test ./plugins/dsshell ./plugins/dsrightbar", View: &apiclient.ViewSpec{Badge: "ok"}},
					{Role: "tool", Tool: "read", Summary: "plugins/dsrightbar/rightbar.go", View: &apiclient.ViewSpec{Badge: "error"}},
				},
			},
			Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			MainWidth: size.Width - 280 - rightPref,
			Geometry: pluginapi.Geometry{
				Width: size.Width, Height: size.Height, SidebarWidth: 280,
				RightWidth: rightPref, MainWidth: size.Width - 280 - rightPref, BottomHeight: 24,
			},
			Has: func(slot pluginapi.Slot) bool {
				if slot == pluginapi.SlotShellOverlay {
					return false
				}
				if slot == pluginapi.SlotRightbar {
					return kernel.Has(slot)
				}
				return true
			},
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f *flex.Frame) {
			kernel.DrawSlot(f, pluginapi.SlotShellRoot, env)
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	capture := func(width, height int, name string, wantRightbar bool) {
		t.Helper()
		if err := session.Resize(width, height); err != nil {
			t.Fatal(err)
		}
		snapshot := session.Snapshot()
		if snapshot.Width != width || snapshot.Height != height {
			t.Fatalf("resize 后场景 = %dx%d，期望 %dx%d", snapshot.Width, snapshot.Height, width, height)
		}
		if got := session.HasText("轨迹"); got != wantRightbar {
			t.Fatalf("%dx%d 右栏文本存在=%v，期望 %v；texts=%q", width, height, got, wantRightbar, session.Texts())
		}
		path := filepath.Join(dir, name)
		if err := session.Screenshot(path); err != nil {
			t.Fatal(err)
		}
		picture, err := session.Image()
		if err != nil {
			t.Fatal(err)
		}
		if picture.Bounds().Dx() != width || picture.Bounds().Dy() != height {
			t.Fatalf("PNG 像素尺寸 = %v，期望 %dx%d", picture.Bounds(), width, height)
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
		}
		t.Logf("V4A_CAPTURE path=%s size=%dx%d bytes=%d rightbar=%v", path, width, height, info.Size(), wantRightbar)
	}

	capture(1920, 1032, "headless-1920x1032.png", true)
	capture(1182, 832, "headless-1182x832.png", true)
	// available = 960-280-400 = 280 < 300：官方要求右栏轨道整列消失。
	capture(960, 832, "headless-resize-960x832.png", false)
	fmt.Fprintln(t.Output(), "V4A resize sequence 1920x1032 -> 1182x832 -> 960x832 completed")
}
