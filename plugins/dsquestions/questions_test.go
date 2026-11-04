package dsquestions

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func toolMsg(args string) apiclient.Message {
	return apiclient.Message{Role: "tool", Tool: askTool, Args: args}
}

const twoOpts = `{"questions":[{"id":"q1","question":"选哪个？","header":"确认",` +
	`"options":[{"label":"A","description":"第一个"},{"label":"B"}]}]}`

// TestPendingFindsUnanswered 验：最后一条是提问、之后没人说话 → 在等。
func TestPendingFindsUnanswered(t *testing.T) {
	msgs := []apiclient.Message{
		{Role: "user"},
		{Role: "assistant"},
		toolMsg(twoOpts),
	}
	got := pending(msgs)
	if len(got) != 1 {
		t.Fatalf("应当找到 1 条待答，实际 %d", len(got))
	}
	if got[0].ID != "q1" || got[0].Question != "选哪个？" {
		t.Fatalf("解析错：%+v", got[0])
	}
	if len(got[0].Options) != 2 || got[0].Options[1].Label != "B" {
		t.Fatalf("选项错：%+v", got[0].Options)
	}
}

// TestNoPendingAfterUserReply 验：提问之后有 user 消息 → **已答过，不显示**。
//
// 不这么判断的话，一个答完的问题会永远挂在输入框上方。
func TestNoPendingAfterUserReply(t *testing.T) {
	msgs := []apiclient.Message{
		toolMsg(twoOpts),
		{Role: "user"}, // 答过了
		{Role: "assistant"},
	}
	if got := pending(msgs); got != nil {
		t.Fatalf("答过的不该再显示，实际 %+v", got)
	}
}

// TestNoPendingWhenNoAsk 验消息流里根本没有提问时不显示。
func TestNoPendingWhenNoAsk(t *testing.T) {
	msgs := []apiclient.Message{
		{Role: "user"},
		{Role: "assistant"},
		{Role: "tool", Tool: "bash", Args: `{"command":"ls"}`},
	}
	if got := pending(msgs); got != nil {
		t.Fatalf("没有提问却显示 %+v", got)
	}
}

// TestNoPendingOnBadArgs 验参数不是合法 JSON 时**宁可不显示**。
//
// 画半截问题（只有标题没有选项）比不画更让人困惑。
func TestNoPendingOnBadArgs(t *testing.T) {
	msgs := []apiclient.Message{toolMsg(`{"questions":`)} // 截断的 JSON
	if got := pending(msgs); got != nil {
		t.Fatalf("坏参数不该显示，实际 %+v", got)
	}
}

// TestNoPendingOnEmpty 验空消息流不崩。
func TestNoPendingOnEmpty(t *testing.T) {
	if got := pending(nil); got != nil {
		t.Fatalf("空消息流不该有东西，实际 %+v", got)
	}
}
