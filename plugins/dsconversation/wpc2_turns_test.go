package dsconversation

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

// WP-C2 先红：Turn 是宿主真值，不是 assistant 消息数；同 Turn 的两条 assistant
// 只能有一个 shell，页脚只消费已有 usage/time/terminal 真值。
func TestWPC2LoadedTurnShellFooterAndMissingPrependTruth(t *testing.T) {
	turnA1 := apiclient.Message{Role: "assistant", TurnID: "turn-A", Text: "A 第一段", Usage: &apiclient.Usage{InputTokens: 10, OutputTokens: 4, TotalTokens: 14}}
	turnA2 := apiclient.Message{Role: "assistant", TurnID: "turn-A", Text: "A 第二段", Usage: &apiclient.Usage{InputTokens: 5, OutputTokens: 3, TotalTokens: 8}}
	turnB := apiclient.Message{Role: "assistant", TurnID: "turn-B", Text: "B 回答"}
	msgs := []apiclient.Message{
		{Role: "user", TurnID: "turn-A", Text: "A 问题"}, turnA1, turnA2,
		{Role: "user", TurnID: "turn-B", Text: "B 问题"}, turnB,
	}
	meta := map[string]pluginapi.MessageMetadata{}
	for i, msg := range msgs {
		meta[pluginapi.MessageKey("s-turn", msg, i)] = pluginapi.MessageMetadata{Time: 1762222444000 + int64(i)*500}
	}
	data := pluginapi.Snapshot{
		Cur: "s-turn", Msgs: msgs, MessageMeta: meta, Truncated: true,
		TurnEvents: []pluginapi.TurnEvent{
			{Key: "done-a", Kind: "done", TurnID: "turn-A", Time: 1762222445500},
			{Key: "error-b", Kind: "error", TurnID: "turn-B", Error: "model failed", Time: 1762222446000},
		},
	}
	turns := buildLoadedTurns(data, []int{0, 1, 2, 3, 4})
	if len(turns) != 2 {
		t.Fatalf("真实 Turn 分组 = %+v", turns)
	}
	if start, end := turnTimes(turns[0], data); start != 1762222444000 || end != 1762222445500 {
		t.Fatalf("Turn A 起止真值 = %d → %d（%dms），meta=%+v", start, end, end-start, meta)
	}
	p := newStreamingMarkdownTestPlugin()
	canvas := drawStreamingMarkdownTest(t, p, data)
	text := sessionTextValues(canvas.Texts())
	for _, want := range []string{
		"已加载 2 个 Turn", "Turn · turn-A", "Turn · turn-B",
		"↑ 15", "↓ 7", "1.5s", "已完成", "失败", "loadOlder 未提供",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Turn shell/footer/ prepend 真值缺 %q：\n%s", want, text)
		}
	}
	if got := strings.Count(text, "Turn · turn-A"); got != 2 {
		// 一次 shell 标题，一次导航当前项；不能按两条 assistant 再造两个 Turn。
		t.Fatalf("同一真实 Turn 被按 assistant 数重复建壳：出现 %d 次\n%s", got, text)
	}
}
