//go:build darwin || windows || linux

package dsconversation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func TestWPC2TurnNavigationReadingPositionAndPrependEvidence(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{"1182x832", 1182, 832}, {"1920x1032", 1920, 1032}} {
		t.Run(size.name, func(t *testing.T) {
			runWPC2ConversationEvidence(t, size.width, size.height)
		})
	}
}

func runWPC2ConversationEvidence(t *testing.T, width, height int) {
	t.Helper()
	p := newStreamingMarkdownTestPlugin()
	theme := p.theme
	msgs := make([]apiclient.Message, 0, 20)
	for _, turn := range []string{"turn-A", "turn-B"} {
		msgs = append(msgs, apiclient.Message{Role: "user", TurnID: turn, Text: "问题 " + turn})
		for i := 0; i < 8; i++ {
			msgs = append(msgs, apiclient.Message{
				Role: "assistant", TurnID: turn,
				Text:  fmt.Sprintf("%s 的已加载回答 %02d：%s", turn, i+1, strings.Repeat("真实正文 ", 12)),
				Usage: &apiclient.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
			})
		}
	}
	data := pluginapi.Snapshot{Cur: "s-wpc2", Msgs: msgs, Truncated: true}
	tree := &gfx.Tree{}
	feat := tree.Feature("wpc2-headless")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{Title: "wpc2-headless", Width: width, Height: height, Tree: tree, WindowMode: app.WindowFrameless, Invalidator: &invalidator}
	// v0.6.0 没有 ID→ScrollState 查表：消息流滚动区由插件交给壳（env.ScrollBox），
	// 这里把它记下来。区域每帧都声明，所以拿到的是同一个 ScrollState 对象。
	var bodyState *ui.ScrollState
	config.Paint = func(canvas *gfx.Canvas, current ui.Size) {
		bounds := ui.Rect{Width: current.Width, Height: current.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{Theme: theme, Data: data, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds, MainWidth: current.Width,
			ScrollBox: func(box ui.Box) { bodyState = box.ScrollState() }}
		view.Draw(canvas, bounds, func(f ui.C) { p.draw(f, env) })
		p.afterDraw(canvas, env)
	}
	config.OnScroll = func(event gfx.ScrollEvent) gfx.EventResult {
		if result := p.HandleScroll(event); result.Handled {
			return result
		}
		return view.Wheel(event)
	}
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

	state := bodyState
	if state == nil || state.Max() <= 0 || !state.AtEnd() {
		t.Fatalf("首载没有真实回底：%+v", state)
	}
	if session.HasText("回到底部") {
		t.Fatal("已经明确在底部时仍显示回底按钮")
	}

	state.ScrollTo(0)
	pump()
	pump()
	for _, want := range []string{"loadOlder 未提供", "回到底部", "Turn · turn-A"} {
		if !session.HasText(want) {
			t.Fatalf("回看首屏缺 %q：%q", want, session.Texts())
		}
	}
	before := state.Offset
	data.Msgs = append(data.Msgs, apiclient.Message{Role: "assistant", TurnID: "turn-B", Text: "回看期间新到的正文"})
	pump()
	pump()
	if state.Offset != before {
		t.Fatalf("新消息打断了用户回看：offset %v → %v", before, state.Offset)
	}
	captureWPC2(t, session, width, height, "reading")

	if err := session.ClickLabel("回到底部"); err != nil {
		t.Fatal(err)
	}
	pump()
	pump()
	if !state.AtEnd() || session.HasText("回到底部") || !session.HasText("回看期间新到的正文") {
		t.Fatalf("显式回底失败：offset=%v max=%v texts=%q", state.Offset, state.Max(), session.Texts())
	}
	if err := session.ClickLabel("← Turn · turn-A"); err != nil {
		t.Fatalf("已加载 Turn 导航不可点击：%v；texts=%q", err, session.Texts())
	}
	pump()
	pump()
	if state.AtEnd() || !session.HasText("Turn · turn-A") {
		t.Fatalf("Turn 导航没有定位已加载内容：offset=%v max=%v texts=%q", state.Offset, state.Max(), session.Texts())
	}
	captureWPC2(t, session, width, height, "turn-navigation")
}

func captureWPC2(t *testing.T, session *headless.Session, width, height int, phase string) {
	t.Helper()
	dir := os.Getenv("WPC2_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("conversation-%s-%dx%d.png", phase, width, height))
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
	t.Logf("WPC2_CAPTURE phase=%s path=%s size=%dx%d bytes=%d", phase, path, width, height, info.Size())
}
