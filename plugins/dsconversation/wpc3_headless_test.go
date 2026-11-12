//go:build darwin || windows || linux

package dsconversation

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func wpc3EvidenceData() (pluginapi.Snapshot, map[string]pluginapi.MessageMetadata) {
	args := `{"command":"printf same"}`
	msgs := []apiclient.Message{
		{Role: "tool", Tool: "bash", CallID: "prepare", Args: "go test ./...", View: &apiclient.ViewSpec{Badge: "preparing"}},
		{Role: "tool", Tool: "bash", TurnID: "turn-live", CallID: "running", Args: "go vet ./..."},
		{Role: "tool", Tool: "bash", TurnID: "turn-live", CallID: "call-A", Args: args, Summary: "exit 0 · RESULT-A"},
		{Role: "tool", Tool: "bash", TurnID: "turn-live", CallID: "call-B", Args: args, Summary: "[exit 1] RESULT-B"},
		{Role: "tool", Tool: "read", CallID: "stopped", Args: `{"file_path":"stopped.go","offset":8,"limit":2}`},
		{Role: "tool", Tool: "edit", CallID: "unknown", Args: `{"file_path":"neutral.go","offset":3,"limit":4}`},
	}
	data := pluginapi.Snapshot{Cur: "s-wpc3", Msgs: msgs, Busy: true, ActiveTurnID: "turn-live"}
	stoppedKey := pluginapi.MessageKey(data.Cur, msgs[4], 4)
	data.MessageMeta = map[string]pluginapi.MessageMetadata{stoppedKey: {Stopped: true}}
	return data, data.MessageMeta
}

func TestWPC3RealStatusExpansionAndInterleavedEvidence(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{"1182x832", 1182, 832}, {"1920x1032", 1920, 1032}} {
		t.Run(size.name, func(t *testing.T) { runWPC3Evidence(t, size.width, size.height) })
	}
}

func runWPC3Evidence(t *testing.T, width, height int) {
	t.Helper()
	p := newStreamingMarkdownTestPlugin()
	theme := p.theme
	data, _ := wpc3EvidenceData()
	tree := &gfx.Tree{}
	feat := tree.Feature("wpc3-headless")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{Title: "wpc3-headless", Width: width, Height: height, Tree: tree, WindowMode: app.WindowFrameless, Invalidator: &invalidator}
	config.Paint = func(canvas *gfx.Canvas, current ui.Size) {
		bounds := ui.Rect{Width: current.Width, Height: current.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{Theme: theme, Data: data, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds, MainWidth: current.Width}
		view.Draw(canvas, bounds, func(f ui.C) { p.draw(f, env) })
		p.afterDraw(canvas, env)
	}
	config.OnScroll = func(event gfx.ScrollEvent) gfx.EventResult { return view.Wheel(event) }
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	pump := func() {
		t.Helper()
		invalidator.Invalidate()
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	pump()
	pump()

	renderIDs := make([]int, 0, 2)
	for _, index := range []int{2, 3} {
		key := messageRenderKey(&pluginapi.Env{Data: data}, data.Msgs[index], index)
		renderID := p.toolRenderID(key)
		renderIDs = append(renderIDs, renderID)
		hit, ok := findToolHit(session.Snapshot(), tree.ID(feat, "tool-"+itoa(renderID)))
		if !ok {
			t.Fatalf("交错调用 index=%d 没有独立命中区：%+v", index, session.Snapshot().Hits)
		}
		x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
		if err := session.Press(x, y); err != nil {
			t.Fatal(err)
		}
		if err := session.Release(x, y); err != nil {
			t.Fatal(err)
		}
		pump()
		pump()
		if !p.toolCards().Open(renderID) {
			t.Fatalf("交错调用 index=%d 没有独立展开", index)
		}
	}
	if len(renderIDs) != 2 || renderIDs[0] == renderIDs[1] || !p.toolCards().Open(renderIDs[0]) || !p.toolCards().Open(renderIDs[1]) {
		t.Fatalf("展开第二个同参调用后，第一个被覆盖或收起：ids=%v openA=%v openB=%v", renderIDs, p.toolCards().Open(renderIDs[0]), p.toolCards().Open(renderIDs[1]))
	}
	for _, value := range []string{"准备中", "运行中", "完成", "失败", "已停止", "RESULT-A", "RESULT-B", "printf same"} {
		if !session.HasText(value) {
			t.Fatalf("真实状态/展开/交错证据缺 %q：%q", value, session.Texts())
		}
	}
	for _, value := range []string{"旧内容", "新内容", "diff"} {
		if session.HasText(value) {
			t.Fatalf("无 old/new 真值却画 %q：%q", value, session.Texts())
		}
	}

	dir := os.Getenv("WPC3_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("conversation-wpc3-%dx%d.png", width, height))
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	picture, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	if picture.Bounds().Dx() != width || picture.Bounds().Dy() != height {
		t.Fatalf("截图尺寸=%v，期望 %dx%d", picture.Bounds(), width, height)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	t.Logf("WPC3_CAPTURE path=%s size=%dx%d bytes=%d", path, width, height, info.Size())
}
