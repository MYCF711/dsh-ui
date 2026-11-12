//go:build darwin || windows || linux

package dsconversation

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

// TestStreamingMarkdownHeadlessFrames 用同一无头会话依次推进合成增量帧，
// 在未闭合围栏的“进行中”与闭合后的“完成”各抓一张真实像素图。
// MDSTREAM_CAPTURE_DIR 非空时把 PNG 留在指定证据目录，否则仍会在临时目录实画。
func TestStreamingMarkdownHeadlessFrames(t *testing.T) {
	dir := os.Getenv("MDSTREAM_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	p := newStreamingMarkdownTestPlugin()
	theme := p.theme
	tree := &gfx.Tree{}
	feat := tree.Feature("mdstream-headless")
	var (
		view        ui.Page
		invalidator app.Invalidator
		data        = streamingMarkdownData("")
	)
	config := app.Config{
		Title: "mdstream-headless", Width: 900, Height: 700, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: data, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width,
		}
		view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f ui.C) {
			p.draw(f, env)
		})
		p.afterDraw(canvas, env)
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	advance := func(stream string) {
		t.Helper()
		data.Busy = true
		data.Stream = stream
		invalidator.Invalidate()
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	inProgress := strings.TrimSuffix(streamingMarkdownSample, "```\n\n完成。")
	frames := []string{
		"# 流式标题\n\n- 第一项",
		"# 流式标题\n\n- 第一项\n- 第二项\n\n正文 `inline`。",
		inProgress,
	}
	for i, frame := range frames {
		if i > 0 && !strings.HasPrefix(frame, frames[i-1]) {
			t.Fatalf("第 %d 帧不是前一帧的增量追加", i+1)
		}
		advance(frame)
	}

	assertFrame := func(phase string) {
		t.Helper()
		snapshot := session.Snapshot()
		var heading, marker, inline, codeMono, rawFence bool
		for _, text := range snapshot.Texts {
			switch text.Value {
			case "流式标题":
				heading = text.Style.Weight >= gfx.WeightSemibold
			case "•":
				marker = true
			case "inline":
				inline = text.Style.Mono
			case "func main() {":
				codeMono = text.Style.Mono
			}
			if strings.Contains(text.Value, "```") || text.Value == "- 第一项" {
				rawFence = true
			}
		}
		const messageIndex = 1
		if len(p.views) <= messageIndex || len(p.views[messageIndex].Codes()) != 1 {
			t.Fatalf("%s：fenced code block 未进入真实 MarkdownView", phase)
		}
		look := p.markdownLook()
		codeFill := false
		for _, primitive := range snapshot.Primitives {
			if primitive.Color == look.CodeFill && primitive.Bounds.Width > 400 && primitive.Bounds.Height > 30 {
				codeFill = true
				break
			}
		}
		if !heading || !marker || !inline || !codeMono || !codeFill || rawFence {
			t.Fatalf("%s：真实绘制断言失败 heading=%v marker=%v inline=%v codeMono=%v codeFill=%v raw=%v；texts=%q",
				phase, heading, marker, inline, codeMono, codeFill, rawFence, session.Texts())
		}
		name := "streaming-in-progress.png"
		if phase == "done" {
			name = "streaming-done.png"
		}
		path := filepath.Join(dir, name)
		if err := session.Screenshot(path); err != nil {
			t.Fatal(err)
		}
		picture, err := session.Image()
		if err != nil {
			t.Fatal(err)
		}
		if picture.Bounds().Dx() != 900 || picture.Bounds().Dy() != 700 {
			t.Fatalf("%s：PNG 尺寸=%v，期望 900x700", phase, picture.Bounds())
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("%s：截图为空 path=%s info=%v err=%v", phase, path, info, err)
		}
		t.Logf("MDSTREAM_CAPTURE phase=%s path=%s size=900x700 bytes=%d code_blocks=1 list_marker=true inline_mono=true code_mono=true code_fill=true raw_markdown=false",
			phase, path, info.Size())
	}
	assertFrame("in_progress")

	data.Busy = false
	data.Stream = ""
	data.Msgs = append(data.Msgs, apiclient.Message{Seq: 2, Role: "assistant", Text: streamingMarkdownSample})
	invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	assertFrame("done")
}
