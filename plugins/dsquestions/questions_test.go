package dsquestions

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
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

// TestF7F8PickOptions 验 F7/F8 分别选第 1/第 2 个选项，并且发的是**标签**。
//
// 发标签而不是"选项 id"：用户回答就是下一条普通 prompt，
// agent 那边只认自然语言——发个 `opt-1` 它会当成一个新问题。
func TestF7F8PickOptions(t *testing.T) {
	var sent []string
	p := &Plugin{}
	p.act = pluginapi.Actions{Send: func(s string) { sent = append(sent, s) }}
	p.cur = Question{Options: []Option{{Label: "A"}, {Label: "B"}}}

	if !p.HandleKey(ui.KeyEvent{Key: ui.KeyF7}) {
		t.Fatal("F7 应当被消费")
	}
	if !p.HandleKey(ui.KeyEvent{Key: ui.KeyF8}) {
		t.Fatal("F8 应当被消费")
	}
	if len(sent) != 2 || sent[0] != "A" || sent[1] != "B" {
		t.Fatalf("发的应当是 A 然后 B，实际 %v", sent)
	}
}

// TestF8WithoutSecondOptionNotConsumed 验只有 1 个选项时 F8 **不被消费**。
//
// 不消费的话，这个键会在这里"消失"，别的插件再也收不到。
func TestF8WithoutSecondOptionNotConsumed(t *testing.T) {
	p := &Plugin{}
	p.act = pluginapi.Actions{Send: func(string) {}}
	p.cur = Question{Options: []Option{{Label: "唯一一个"}}}
	if p.HandleKey(ui.KeyEvent{Key: ui.KeyF8}) {
		t.Fatal("没有第二个选项时不该消费 F8")
	}
}

// TestNoOptionsNotConsumed 验没有待答问题时 F7/F8 都不消费。
func TestNoOptionsNotConsumed(t *testing.T) {
	p := &Plugin{}
	p.act = pluginapi.Actions{Send: func(string) {}}
	if p.HandleKey(ui.KeyEvent{Key: ui.KeyF7}) {
		t.Fatal("没有问题时不该消费 F7")
	}
}
