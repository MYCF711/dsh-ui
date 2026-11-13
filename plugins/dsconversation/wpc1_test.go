package dsconversation

import (
	"strings"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
)

func TestWPC1MessageAndTerminalTruthIsVisible(t *testing.T) {
	at := time.Date(2026, 11, 4, 3, 4, 5, 0, time.Local).UnixMilli()
	msg := apiclient.Message{Role: "assistant", Text: "部分回答", TurnID: "turn-message"}
	key := pluginapi.MessageKey("s-turn", msg, 0)
	data := pluginapi.Snapshot{
		Cur:  "s-turn",
		Msgs: []apiclient.Message{msg},
		MessageMeta: map[string]pluginapi.MessageMetadata{
			key: {Time: at, Stopped: true},
		},
		TurnEvents: []pluginapi.TurnEvent{
			{Key: "error-local", Kind: "error", TurnID: "turn-real", Error: "model failed", Time: at},
			{Key: "stopped-local", Kind: "stopped", Time: at},
		},
		Outgoing: []pluginapi.OutgoingMessage{
			{Key: "send-local", State: pluginapi.OutgoingFailed, Text: "保留这句", Error: "offline"},
		},
	}
	p := newStreamingMarkdownTestPlugin()
	canvas := drawStreamingMarkdownTest(t, p, data)
	for _, want := range []string{
		time.UnixMilli(at).Format("15:04"), "turn-message", "已停止",
		"model failed", "turn-real", "发送失败", "offline", "保留这句",
	} {
		if !strings.Contains(sessionTextValues(canvas.Texts()), want) {
			t.Fatalf("画面缺少真值 %q：%q", want, sessionTextValues(canvas.Texts()))
		}
	}
	for _, text := range canvas.Texts() {
		if strings.Contains(text.Value, "重试发送") {
			t.Fatal("没有 RetrySend Action 时不得画重试按钮")
		}
		if strings.Contains(text.Value, "turn-0") {
			t.Fatal("缺 TurnID 时虚构了 turn")
		}
	}
}

func TestWPC1ToolRenderIDIsStableAndSessionLocal(t *testing.T) {
	p := &Plugin{}
	envA := &pluginapi.Env{Data: pluginapi.Snapshot{Cur: "session-a"}}
	first := messageRenderKey(envA, apiclient.Message{Role: "tool", Tool: "bash"}, 0)
	second := messageRenderKey(envA, apiclient.Message{Role: "tool", Tool: "read"}, 1)
	firstID, secondID := p.toolRenderID(first), p.toolRenderID(second)
	if firstID == secondID {
		t.Fatal("两个未知 Seq 的工具消息共享了 dstool 展开键")
	}
	if got := p.toolRenderID(first); got != firstID {
		t.Fatalf("同一 MessageKey 的 dstool 局部键不稳定：%d → %d", firstID, got)
	}
	envB := &pluginapi.Env{Data: pluginapi.Snapshot{Cur: "session-b"}}
	otherSession := messageRenderKey(envB, apiclient.Message{Role: "tool", Tool: "bash"}, 0)
	if otherSession == first || p.toolRenderID(otherSession) == firstID {
		t.Fatal("工具展开键跨会话串用")
	}
}

func TestWPC1ExplicitFileLinkRewriteDoesNotGuessPlainTextOrCode(t *testing.T) {
	source := "普通 file:///tmp/not-a-link.txt 与 `file:///tmp/inline.txt`\n\n" +
		"[HTTP](https://example.com/a) [文件](file:///tmp/a%20b.txt#L24-L30)\n\n" +
		"```text\n[不解析](file:///tmp/code.txt)\n```\n"
	rewritten, files := rewriteExplicitFileLinks(source)
	if len(files) != 1 {
		t.Fatalf("只应改写一个显式 Markdown file link，得到 %d：%q", len(files), rewritten)
	}
	if !strings.Contains(rewritten, "普通 file:///tmp/not-a-link.txt") ||
		!strings.Contains(rewritten, "`file:///tmp/inline.txt`") ||
		!strings.Contains(rewritten, "[不解析](file:///tmp/code.txt)") {
		t.Fatalf("普通文本、行内码或代码围栏被猜成文件：%q", rewritten)
	}
	for fake, original := range files {
		if !strings.Contains(rewritten, fake) || original != "file:///tmp/a%20b.txt#L24-L30" {
			t.Fatalf("文件映射被改写：fake=%q original=%q", fake, original)
		}
	}
}

func sessionTextValues(texts []gfx.Text) string {
	var b strings.Builder
	for _, text := range texts {
		b.WriteString(text.Value)
		b.WriteByte('\n')
	}
	return b.String()
}
