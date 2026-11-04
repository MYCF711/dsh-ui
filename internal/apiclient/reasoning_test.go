// reasoning_test.go —— "只有思考、没有正文"的消息怎么翻。
//
// 这条规则的价值在于**同一个回答在两条路径上长得一样**：
// 实时流（Stream.Messages 落 role=thinking）和重载（session.load 给正文为空 +
// reasoning 的 assistant 消息）。不翻的话，切回会话时思考块就没了——
// 宿主那边刚修过同一个"静默失败"，界面这边不能还留着。
package apiclient

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReasoningOnlyBecomesThinking(t *testing.T) {
	msg := Message{Seq: 42, Role: "assistant", Text: "", Reasoning: "先看有没有 fs.read"}
	got, ok := ReasoningOnlyThinking(msg)
	if !ok {
		t.Fatal("正文为空、reasoning 非空，应当翻成 thinking")
	}
	if got.Role != "thinking" {
		t.Errorf("role = %q，want thinking", got.Role)
	}
	if got.Text != "先看有没有 fs.read" {
		t.Errorf("thinking 的正文应当是思考本身，实际 %q", got.Text)
	}
	if got.Reasoning != "" {
		t.Errorf("翻过之后不该再带 Reasoning（否则渲染层会画两遍），实际 %q", got.Reasoning)
	}
	if got.Seq != 42 {
		t.Errorf("seq 要保住（合并/去重靠它），实际 %d", got.Seq)
	}
}

// TestReasoningOnlyDoesNotTouchOtherShapes 是**反方向**：别把正常消息翻了。
//
// 没有这条的话，把函数写成"只要 role==assistant 就翻"也能过上面那条。
func TestReasoningOnlyDoesNotTouchOtherShapes(t *testing.T) {
	cases := []struct {
		name string
		msg  Message
	}{
		{"有正文就不翻（正文才是回答）", Message{Role: "assistant", Text: "结论如下", Reasoning: "我先想了很久"}},
		{"纯空白正文也不翻？——不，空白算空，这条是纯空格正文", Message{Role: "assistant", Text: "   ", Reasoning: "想了"}},
		{"没有 reasoning", Message{Role: "assistant", Text: ""}},
		{"user 消息带 reasoning？（不该出现，但也不能翻）", Message{Role: "user", Text: "", Reasoning: "x"}},
		{"tool 消息", Message{Role: "tool", Text: "", Reasoning: "x"}},
		{"已经是 thinking", Message{Role: "thinking", Text: "想了", Reasoning: ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 第三条（纯空格正文）**应当**翻：宿主用 strings.TrimSpace 判断空。
			wantConvert := tc.name == "纯空白正文也不翻？——不，空白算空，这条是纯空格正文"
			got, ok := ReasoningOnlyThinking(tc.msg)
			if ok != wantConvert {
				t.Fatalf("翻=%v，want %v（msg=%+v）", ok, wantConvert, tc.msg)
			}
			if !ok && !strings.EqualFold(got.Role, "") {
				t.Errorf("没翻的时候不该返回任何东西，实际 %+v", got)
			}
		})
	}
}

// TestReasoningFrameParses：wire 上那个键叫 reasoning，别拼错。
func TestReasoningFrameParses(t *testing.T) {
	var m Message
	if err := json.Unmarshal([]byte(`{"seq":7,"role":"assistant","text":"","reasoning":"想了三步"}`), &m); err != nil {
		t.Fatal(err)
	}
	if m.Reasoning != "想了三步" {
		t.Fatalf("Reasoning 没解出来：%+v", m)
	}
	if _, ok := ReasoningOnlyThinking(m); !ok {
		t.Fatal("解出来的形状应当能翻成 thinking")
	}
}
