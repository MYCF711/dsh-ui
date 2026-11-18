//go:build darwin || windows || linux

// capture2_test.go —— 第二轮真实渲染审计：顶栏在 **Windows caption** 下的样子。
//
// 官方 Windows 规则（AppFrame.module.css.mjs.css）：
//
//	[data-windows-titlebar] .frame{padding-top:var(--dsh-windows-titlebar-height);
//	                               background:var(--dsw-specific-sidebar-fill)}
//	[data-windows-titlebar] .centerCol{background:var(--dsw-alias-bg-base);
//	                                   border-radius:16px 0 0 0}
//
// 也就是：caption 是 40 高、铺 sidebar-fill、可拖窗口；内容列整体下移 40，
// 顶栏自己**不**再为窗口按钮让位（按钮在 caption 行里）。本仓的等价物是
// `cmd/app/frame_chrome.go` 的 Layout.Caption / Layout.Content（40 + 下移）与
// `configureFrameWindow` 的 WindowCustomTitlebar。
//
// 这里把两者的相对关系画成一张图：capture 带上一条 40 的 caption 带，
// 顶栏画在它下面——图上能直接看出顶栏有没有压进 caption、右侧有没有被切。
//
// 截图落 `$TOPBAR_CAPTURE_DIR`（未设置时用临时目录）。
package dstopbar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dstopbar/viewapi"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func TestTopbarWindowsCaptionCapture(t *testing.T) {
	dir := os.Getenv("TOPBAR_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const width, height = 1182, 160
	theme := dscore.Dark()
	p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
	tree := &gfx.Tree{}
	feat := tree.Feature("topbar-caption")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	caption := float32(uikernel.CaptionHeight)
	config := app.Config{
		Title: "topbar-caption", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowCustomTitlebar, TitlebarHeight: uikernel.CaptionHeight,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		// caption 带：官方 `[data-windows-titlebar] .frame` 的 sidebar-fill。
		canvas.Fill(gfx.Rect{Width: size.Width, Height: caption}, theme.SidebarFill)
		body := gfx.Rect{Y: caption, Width: size.Width, Height: size.Height - caption}
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: body,
			MainWidth: size.Width,
			Data: pluginapi.Snapshot{
				Cur: "s1",
				Sessions: []apiclient.Session{{
					ID: "s1", Title: "渲染验收：主窗口骨架", Cwd: "D:/work/dsh-ui",
				}},
				Msgs:       []apiclient.Message{{Role: "user", Text: "hi"}},
				Total:      1,
				Model:      "group/deepseek-v4.1",
				Permission: "ask",
			},
			Views: []pluginapi.Tab{
				{Key: "chat", Kind: "chat", Label: "对话"},
				{Key: "trajectory", Kind: "trajectory", Label: "轨迹"},
			},
			ActiveView: viewapi.Chat,
			Act:        pluginapi.Actions{ToggleRightbar: func() {}},
			Has:        func(slot pluginapi.Slot) bool { return slot == pluginapi.SlotRightbar },
		}
		canvas.Fill(body, theme.BgBase)
		// 布局父矩形必须是 body（Y=caption）：官方是 frame 的 padding-top 把内容
		// 整块压下去，不是顶栏自己再让一次位。
		view.Draw(canvas, body, func(f ui.C) {
			p.draw(f, env)
		})
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

	// 顶栏内容必须整块落在 caption 之下（官方 frame 的 padding-top）：任何一条
	// 文本压进上面 40px 就是被 caption 盖住/被窗口按钮遮住。
	captionTop := caption
	for _, text := range session.Snapshot().Texts {
		if text.Bounds.Y < captionTop {
			t.Fatalf("%q 画进了 caption 带：y=%v < %v", text.Value, text.Bounds.Y, captionTop)
		}
		if right := text.Bounds.X + text.Bounds.Width; right > width {
			t.Fatalf("%q 右缘 %v 超出窗口宽 %d", text.Value, right, width)
		}
	}
	// 官方 active Header：crumbs 的第一段是工作区名、第二段是当前会话标题（14/20/500）。
	joined := strings.Join(session.Texts(), "\u0000")
	for _, want := range []string{"渲染验收：主窗口骨架", "对话", "轨迹"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("顶栏没画出 %q：%q", want, session.Texts())
		}
	}
	path := filepath.Join(dir, "topbar-windows-caption.png")
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	t.Logf("TOPBAR_CAPTURE path=%s", path)
}
