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
	"github.com/MYCF711/dsh-ui/plugins/dstopbar"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
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
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dsshell.Plugin{}, &dstopbar.Plugin{}, &Plugin{}); err != nil {
		t.Fatal(err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("v4a-headless")
	var view flex.View
	showRightbar := true
	config := app.Config{Title: "v4a-shell-rightbar", Width: 1920, Height: 1032, Tree: tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		rightPref := size.Width * 0.45
		env := &pluginapi.Env{
			Theme: theme,
			Data: pluginapi.Snapshot{
				Cur: "v4a-visual", ShowRightbar: showRightbar,
				Sessions: []apiclient.Session{{ID: "v4a-visual", Title: "右栏会话"}},
				Msgs: []apiclient.Message{
					{Role: "tool", Tool: "bash", Summary: "CGO_ENABLED=0 go test ./plugins/dsshell ./plugins/dsrightbar", View: &apiclient.ViewSpec{Badge: "ok"}},
					{Role: "tool", Tool: "read", Summary: "plugins/dsrightbar/rightbar.go", View: &apiclient.ViewSpec{Badge: "error"}},
				},
			},
			Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			Act:       pluginapi.Actions{ToggleRightbar: func() { showRightbar = !showRightbar }},
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
		// “轨迹”同时是主区真实页签；右栏可见性只认自己的计数，避免把页签误判成右栏。
		if got := session.HasText("2 次"); got != wantRightbar {
			t.Fatalf("%dx%d 右栏计数存在=%v，期望 %v；texts=%q", width, height, got, wantRightbar, session.Texts())
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

	// PanelChrome：真实点击全屏/恢复；轨迹条目必须真的从右侧移动到全画面板。
	normalX := findText(t, session.Snapshot().Texts, "bash").Bounds.X
	if err := session.ClickLabel("右栏全屏"); err != nil {
		t.Fatalf("点击右栏全屏：%v", err)
	}
	if got := findText(t, session.Snapshot().Texts, "bash").Bounds.X; got >= normalX || got > 80 {
		t.Fatalf("全屏后轨迹没有进入全画面板：beforeX=%v afterX=%v", normalX, got)
	}
	if err := session.ClickLabel("退出全屏"); err != nil {
		t.Fatalf("点击退出全屏：%v", err)
	}
	if got := findText(t, session.Snapshot().Texts, "bash").Bounds.X; got < normalX-1 {
		t.Fatalf("退出全屏没有恢复右侧位置：normalX=%v restoredX=%v", normalX, got)
	}
	// 展开时 corner 消失，只能从右栏收起；收起后 corner 才重新出现。
	if err := session.ClickLabel("收起右侧边栏"); err != nil {
		t.Fatalf("右栏自己的收起按钮不可用：%v", err)
	}
	if session.HasText("2 次") {
		t.Fatalf("收起后右栏仍实画：%q", session.Texts())
	}
	if err := session.ClickLabel("打开右侧边栏"); err != nil {
		t.Fatalf("收起态 corner 无法重新展开：%v", err)
	}
	if !session.HasText("2 次") {
		t.Fatalf("corner 展开后右栏未恢复：%q", session.Texts())
	}

	capture(1182, 832, "headless-1182x832.png", true)
	// available = 960-280-400 = 280 < 300：官方要求右栏轨道整列消失。
	capture(960, 832, "headless-resize-960x832.png", false)
	fmt.Fprintln(t.Output(), "V4A resize sequence 1920x1032 -> 1182x832 -> 960x832 completed")
}
