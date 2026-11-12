package dsconversation

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const streamingMarkdownSample = "# 流式标题\n\n- 第一项\n- 第二项\n\n正文 `inline`。\n\n```go\nfunc main() {\n\tprintln(\"进行\")\n}\n```\n\n完成。"

func streamingMarkdownData(stream string) pluginapi.Snapshot {
	return pluginapi.Snapshot{
		Cur: "mdstream", Busy: stream != "", Stream: stream,
		Msgs: []apiclient.Message{{Seq: 1, Role: "user", Text: "演示"}},
	}
}

func drawStreamingMarkdownTest(t *testing.T, p *Plugin, data pluginapi.Snapshot) *gfx.Canvas {
	t.Helper()
	theme := p.theme
	const width = float32(900)
	tree := &gfx.Tree{}
	canvas := newHeroTestCanvas(tree)
	// v0.6.0 没有 Begin/End：Draw 内部就完成声明与解算，主题构造期给。
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	env := &pluginapi.Env{
		Theme: theme, Data: data, Tree: tree, Feat: tree.Feature("mdstream-test"), Canvas: canvas,
		Bounds: ui.Rect{Width: width, Height: 700}, MainWidth: width,
	}
	view.Draw(canvas, ui.Rect{Width: width, Height: 700}, func(f ui.C) { p.draw(f, env) })
	p.afterDraw(canvas, env)
	return canvas
}

func newStreamingMarkdownTestPlugin() *Plugin {
	theme := dscore.Dark()
	return &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
}

func TestStreamingMarkdownReallyDrawsHeadingListAndInlineCode(t *testing.T) {
	p := newStreamingMarkdownTestPlugin()
	canvas := drawStreamingMarkdownTest(t, p, streamingMarkdownData(streamingMarkdownSample))

	var heading, marker, item, inline bool
	for _, text := range canvas.Texts() {
		if text.Value == streamingMarkdownSample {
			t.Fatal("流式正文仍是一整段 f.Wrap 纯文本")
		}
		switch text.Value {
		case "流式标题":
			heading = text.Style.Weight >= gfx.WeightSemibold && !text.Style.Mono
		case "•":
			marker = true
		case "第一项":
			item = true
		case "inline":
			inline = text.Style.Mono
		}
	}
	if !heading || !marker || !item || !inline {
		t.Fatalf("流式 Markdown 实画不全：heading=%v marker=%v item=%v inlineMono=%v；texts=%+v",
			heading, marker, item, inline, canvas.Texts())
	}
}

func TestStreamingMarkdownOpenFenceStaysCodeAndKeepsText(t *testing.T) {
	p := newStreamingMarkdownTestPlugin()
	frames := []struct {
		name     string
		stream   string
		wantCode string
	}{
		{name: "空代码块", stream: "# 流式标题\n\n```go\n", wantCode: ""},
		{name: "追加半个标识符", stream: "# 流式标题\n\n```go\nfmt.Print", wantCode: "fmt.Print"},
		{name: "未闭合围栏", stream: "# 流式标题\n\n```go\nfmt.Println(\"未闭合\")\n", wantCode: "fmt.Println(\"未闭合\")"},
		{name: "闭合围栏", stream: "# 流式标题\n\n```go\nfmt.Println(\"未闭合\")\n```\n\n完成。", wantCode: "fmt.Println(\"未闭合\")"},
	}
	for _, frame := range frames {
		t.Run(frame.name, func(t *testing.T) {
			canvas := drawStreamingMarkdownTest(t, p, streamingMarkdownData(frame.stream))
			const streamIndex = 1 // 一条 user 消息之后，流式与定稿共用同一个缓存槽。
			if len(p.views) <= streamIndex {
				t.Fatal("流式正文没有自己的 MarkdownView，未闭合围栏只能走纯文本")
			}
			view := &p.views[streamIndex]
			codes := view.Codes()
			if len(codes) != 1 {
				t.Fatalf("围栏应始终绘制成 1 个代码块，实际 %d；texts=%+v", len(codes), canvas.Texts())
			}
			if codes[0].Text != frame.wantCode {
				t.Fatalf("代码正文被吞或改变：got %q want %q", codes[0].Text, frame.wantCode)
			}
			plain := view.SelectedText(components.MarkdownSelection{Anchor: 0, Focus: view.TextLen()})
			if !strings.Contains(plain, frame.wantCode) {
				t.Fatalf("可选择正文缺少代码 %q：%q", frame.wantCode, plain)
			}
			for _, text := range canvas.Texts() {
				if strings.Contains(text.Value, "```") {
					t.Fatalf("完整开围栏后仍闪烁为原始反引号：%q", text.Value)
				}
			}
		})
	}
}

func TestStreamingMarkdownReusesFinalMessageCacheSlot(t *testing.T) {
	p := newStreamingMarkdownTestPlugin()
	data := streamingMarkdownData(streamingMarkdownSample)
	drawStreamingMarkdownTest(t, p, data)
	const messageIndex = 1
	if len(p.views) <= messageIndex {
		t.Fatal("流式正文未进入定稿共用的 Markdown 缓存")
	}
	streamView := &p.views[messageIndex]
	streamHeight := streamView.Height()

	data.Busy = false
	data.Stream = ""
	data.Msgs = append(data.Msgs, apiclient.Message{Seq: 2, Role: "assistant", Text: streamingMarkdownSample})
	drawStreamingMarkdownTest(t, p, data)
	if len(p.views) <= messageIndex || &p.views[messageIndex] != streamView {
		t.Fatal("流式转定稿时替换了 MarkdownView，缓存没有连续复用")
	}
	if got := p.views[messageIndex].Height(); got != streamHeight {
		t.Fatalf("同正文同宽度在流式/定稿之间高度跳变：stream=%v final=%v", streamHeight, got)
	}
}

func TestStreamingMarkdownLayoutCacheBoundsRepeatedFrames(t *testing.T) {
	p := newStreamingMarkdownTestPlugin()
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	measurements := 0
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		measurements++
		return float32(len([]rune(value))) * size
	})
	const width = float32(860)
	p.layoutMarkdown(canvas, 1, streamingMarkdownSample, width)
	first := measurements
	if first == 0 {
		t.Fatal("首次 Markdown 排版没有测量文本，测试支架无效")
	}
	for range 8 {
		p.layoutMarkdown(canvas, 1, streamingMarkdownSample, width)
	}
	if measurements != first {
		t.Fatalf("正文与宽度未变仍重复排版：首次测量 %d 次，重复 8 帧后 %d 次", first, measurements)
	}
	p.layoutMarkdown(canvas, 1, streamingMarkdownSample+"\n新增增量", width)
	if measurements <= first {
		t.Fatal("正文追加后没有按需重排")
	}
}
