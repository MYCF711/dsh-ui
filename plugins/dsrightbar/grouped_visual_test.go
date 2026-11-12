//go:build darwin || windows || linux

package dsrightbar

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// groupedSession 造一份「三轮、共 12 次工具」的会话：分组要解决的就是这种形状。
//
// 这里**不编宿主的 turn 字段**——真实 session.load 的消息里没有 turnId/turn，
// 轮次只能靠用户消息切；这份语料和真机同形（只多了可读的工具名）。
func groupedSession() []apiclient.Message {
	ok := func() *apiclient.ViewSpec { return &apiclient.ViewSpec{Badge: "ok"} }
	return []apiclient.Message{
		{Role: "user", Text: "1. 先跑测试，再把失败的那个文件读出来"},
		{Role: "tool", Tool: "bash", Summary: "CGO_ENABLED=0 go test ./...", View: ok()},
		{Role: "tool", Tool: "read", Summary: "plugins/dsrightbar/rightbar.go", View: ok()},
		{Role: "tool", Tool: "grep", Summary: "toolCalls(msgs)", View: ok()},
		{Role: "tool", Tool: "edit", Summary: "rightbar.go · +128 −0", View: ok()},
		{Role: "assistant", Text: "改完了"},

		{Role: "user", Text: "2. 再跑一次，确认全绿"},
		{Role: "tool", Tool: "bash", Summary: "go build ./... && go test ./plugins/dsrightbar", View: ok()},
		{Role: "tool", Tool: "bash", Summary: "go vet ./plugins/dsrightbar", View: ok()},
		{Role: "tool", Tool: "read", Summary: "turns_test.go", View: &apiclient.ViewSpec{Badge: "error"}},
		{Role: "assistant", Text: "有一个测试红了"},

		{Role: "user", Text: "3. 修掉它，然后按 1920 和 1182 各截一张"},
		{Role: "tool", Tool: "edit", Summary: "turns_test.go · +6 −2", View: ok()},
		{Role: "tool", Tool: "bash", Summary: "go test ./plugins/dsrightbar -run Turn", View: ok()},
		{Role: "tool", Tool: "bash", Summary: "app.exe -size 1920x1032 -shot tmp/a.png", View: ok()},
		{Role: "tool", Tool: "bash", Summary: "app.exe -size 1182x832 -shot tmp/b.png", View: ok()},
		{Role: "tool", Tool: "glob", Summary: "tmp/traj-*.png", View: ok()},
		{Role: "assistant", Text: "两张都在"},
	}
}

// openRightbarShot 起一个只画右栏的真 Viva 无头窗口（真布局、真主题、真点击）。
func openRightbarShot(t *testing.T, width, height int, msgs []apiclient.Message) *headless.Session {
	t.Helper()
	theme := dscore.Dark()
	p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
	tree := &gfx.Tree{}
	feat := tree.Feature("traj-shot")
	var view flex.View
	config := app.Config{Title: "traj-shot", Width: width, Height: height, Tree: tree}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: theme, Data: pluginapi.Snapshot{Cur: "traj-shot", Msgs: msgs},
			Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			Has: func(pluginapi.Slot) bool { return true },
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f *flex.Frame) { p.draw(f, env) })
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session
}

// TestGroupedTrajectoryScreenshots 出三张图：默认（分组 + 只展开当前轮）、全展开、全收起。
//
// 三张都在**同一个**无头窗口里、用真点击切过去，所以"折叠/展开"是点出来的，
// 不是三个不同状态各画一遍摆拍。
func TestGroupedTrajectoryScreenshots(t *testing.T) {
	dir := os.Getenv("TRAJ_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	msgs := groupedSession()
	session := openRightbarShot(t, 300, 900, msgs)

	save := func(name string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := session.Screenshot(path); err != nil {
			t.Fatal(err)
		}
		picture, err := session.Image()
		if err != nil {
			t.Fatal(err)
		}
		if picture.Bounds().Dx() != 300 || picture.Bounds().Dy() != 900 {
			t.Fatalf("PNG 像素尺寸 = %v，期望 300x900", picture.Bounds())
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
		}
		t.Logf("TRAJ_CAPTURE path=%s bytes=%d", path, info.Size())
	}

	heads := []string{"第 1 轮 · 4 次工具", "第 2 轮 · 3 次工具", "第 3 轮 · 5 次工具"}

	// ── 1. 默认态：三行组头，只有当前轮（第 3 轮）展开 ──
	for _, head := range heads {
		if !session.HasText(head) {
			t.Fatalf("缺少组头 %q；texts=%q", head, session.Texts())
		}
	}
	if !session.HasText("go test ./plugins/dsrightbar -run Turn") {
		t.Fatalf("当前轮没有展开：%q", session.Texts())
	}
	if session.HasText("plugins/dsrightbar/rightbar.go") {
		t.Fatalf("历史轮默认不该展开：%q", session.Texts())
	}
	save("traj-headless-default.png")

	// ── 2. 全展开：逐个点开前两轮，第 3 轮本来就是开的 ──
	for _, head := range heads[:2] {
		if err := session.ClickLabel(head); err != nil {
			t.Fatalf("点开 %q：%v", head, err)
		}
	}
	for _, want := range []string{"plugins/dsrightbar/rightbar.go", "go vet ./plugins/dsrightbar", "turns_test.go", "go test ./plugins/dsrightbar -run Turn"} {
		if !session.HasText(want) {
			t.Fatalf("全展开后缺少 %q；texts=%q", want, session.Texts())
		}
	}
	save("traj-expanded.png")

	// ── 3. 全收起：三组都点一下 ──
	for _, head := range heads {
		if err := session.ClickLabel(head); err != nil {
			t.Fatalf("收起 %q：%v", head, err)
		}
	}
	for _, gone := range []string{"plugins/dsrightbar/rightbar.go", "go vet ./plugins/dsrightbar", "go test ./plugins/dsrightbar -run Turn"} {
		if session.HasText(gone) {
			t.Fatalf("全收起后仍在画 %q；texts=%q", gone, session.Texts())
		}
	}
	for _, head := range heads {
		if !session.HasText(head) {
			t.Fatalf("收起后组头没了 %q；texts=%q", head, session.Texts())
		}
	}
	save("traj-collapsed.png")
}
