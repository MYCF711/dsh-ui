//go:build darwin || windows || linux

// terminal_capture_test.go —— 终端面板的**真实渲染截图**（给截图比对用）。
//
// 与 `visual_test.go` 的分工：那边在裸 canvas 上核对官方像素几何（root 8px
// 上内距、13px 等宽、status 8/5/12、按钮 28 高…）；这边把同一帧经真 Viva
// headless 渲染成 PNG，专看只有像素才暴露的问题：等宽字体有没有掉回比例字体、
// 行盒有没有跳档、状态条与按钮有没有被父级裁掉。
//
// 截图落 `$VR_TERMINAL_CAPTURE_DIR`（默认 t.TempDir()）。
package dsterminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// terminalCaptureState 是一份"连着、可写、有历史输出"的终端态。
func terminalCaptureState() pluginapi.TerminalViewState {
	return pluginapi.TerminalViewState{
		Phase:    "connected",
		Writable: true,
		Info:     &apiclient.TerminalInfo{Title: "终端 1", State: "running"},
		Screen: "$ CGO_ENABLED=0 go test ./plugins/dsterminal\n" +
			"ok  \tgithub.com/MYCF711/dsh-ui/plugins/dsterminal\t0.41s\n" +
			"$ gofmt -l cmd internal plugins pluginapi\n" +
			"$ ",
	}
}

// TestTerminalPanelRenderCapture 画右侧终端面板并落 PNG。
func TestTerminalPanelRenderCapture(t *testing.T) {
	dir := os.Getenv("VR_TERMINAL_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	theme := dscore.Dark()
	state := terminalCaptureState()
	const width, height = 720, 480
	tree := &gfx.Tree{}
	feat := tree.Feature("terminal-capture")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	p := &Plugin{theme: theme}
	config := app.Config{
		Title: "terminal-capture", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width,
			Data:           pluginapi.Snapshot{ShowRightbar: true},
			ActiveRightTab: RightTabTerminal,
			Act: pluginapi.Actions{
				TerminalEnsure:        func(string) {},
				TerminalSnapshot:      func(string) pluginapi.TerminalViewState { return state },
				TerminalRetry:         func(string) {},
				OpenRightTab:          func(string) {},
				CloseRightTab:         func(string) {},
				TerminalWriteOfficial: func(string, string, bool) {},
			},
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			p.drawPanelBody(f, env)
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

	// 官方几何：首行从 y=16（root 8 + screen 8）开始，13px 行盒、等宽。
	snapshot := session.Snapshot()
	// 官方几何：首行从 y=16（root 8 + screen 8）开始，13px 行盒、等宽。
	var first *gfx.Text
	for i := range snapshot.Texts {
		if strings.Contains(snapshot.Texts[i].Value, "CGO_ENABLED=0 go test") {
			first = &snapshot.Texts[i]
			break
		}
	}
	if first == nil {
		t.Fatalf("终端屏上没有第一行命令；文本=%q", session.Texts())
	}
	if !first.Style.Mono || first.Style.Size != terminalFontSize {
		t.Fatalf("终端输出必须是 %vpx 等宽：%+v", terminalFontSize, first.Style)
	}
	if first.Bounds.X != terminalScreenPad || first.Bounds.Y != terminalRootPadTop+terminalScreenPad {
		t.Fatalf("首行应落在 (8,16)（screen 8px 内距 + root 8px 上内距），得到 %+v", first.Bounds)
	}

	// 派生控件（本仓保留的 F6 输入 + 重命名，不是官方工具栏）也必须按 inline 按钮排：
	// 官方 Button 宽度由 label 决定（ButtonSizeSmall），不是拉满整行的隐形按钮。
	// 直接把它放进 Column 会被块级拉伸成整行（实测命中区 704×28、标签居中），
	// 这就是渲染截图才看得出来的错位。
	var renameHit *gfx.Hit
	for i := range snapshot.Hits {
		if snapshot.Hits[i].Label == "重命名" {
			renameHit = &snapshot.Hits[i]
		}
	}
	if renameHit == nil {
		t.Fatalf("找不到重命名按钮命中区；hits=%+v", snapshot.Hits)
	}
	if renameHit.Bounds.Width > 120 || renameHit.Bounds.X != terminalScreenPad {
		t.Fatalf("重命名按钮命中区 = %+v，应贴左（x=%v）且按内容宽（≤120px），不是拉满整行",
			renameHit.Bounds, terminalScreenPad)
	}

	t.Logf("VR_TERMINAL_GEOM firstLine=%+v renameHit=%+v", first.Bounds, renameHit.Bounds)
	path := filepath.Join(dir, "terminal-panel.png")
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	t.Logf("VR_TERMINAL_CAPTURE path=%s", path)
}
