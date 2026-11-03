// thinking_test.go —— 流式 reasoning 落成消息之后，真的会走到「思考块」那条绘制分支。
//
// 这是 H6 在插件侧的钉子：绘制函数本身要真实布局才能测（靠无头截图），
// 但"这条消息会不会被当成思考块画"是一个纯判定，必须能裸跑。
// 只测 roleKind("thinking") 不够——那只能证明映射表写对了，证明不了
// **流式那条路产出的角色就是 thinking**。所以这里从 apiclient 的流归约开始走。
package dsconversation

import (
	"encoding/json"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func TestRoleKindMapsRoles(t *testing.T) {
	// 未知角色必须降级成助手正文：宁可画得朴素，也不能让内容消失。
	cases := []struct {
		role string
		want msgKind
	}{
		{"thinking", kindThinking},
		{"tool", kindTool},
		{"user", kindUser},
		{"assistant", kindAssistant},
		{"", kindAssistant},       // 空角色（老数据）
		{"system", kindAssistant}, // 不认识的角色
	}
	for _, tc := range cases {
		if got := roleKind(tc.role); got != tc.want {
			t.Fatalf("roleKind(%q) = %v，want %v", tc.role, got, tc.want)
		}
	}
}

// TestStreamedReasoningUsesThinkingBranch 是这条链的端到端断言（不带窗口）：
//
//	流帧（assistant/message 带 reasoning 块）
//	  → apiclient.Stream.Apply / Merge
//	  → Msgs 里出现 Role=="thinking" 的一条
//	  → roleKind 判成 kindThinking（即 drawThinking 那一支）
//
// 缺任何一环，界面上都不会有思考块——而三者分别在不同的包里，
// 这就是当初"收得到却画不出来"能长期存在的原因。
func TestStreamedReasoningUsesThinkingBranch(t *testing.T) {
	var (
		msgs []apiclient.Message
		s    apiclient.Stream
	)
	frames := []apiclient.Event{
		{Kind: "assistant/message", Seq: 21, Data: json.RawMessage(
			`{"content":[{"type":"reasoning","text":"先看一眼目录结构"},{"type":"text","text":"看完了"}]}`)},
	}
	for _, ev := range frames {
		s.Apply(ev)
		msgs = apiclient.Merge(msgs, &s)
		if s.Done() {
			s.Reset()
		}
	}
	if len(msgs) != 2 {
		t.Fatalf("应当得到 2 条消息（思考 + 正文），得到 %d 条：%+v", len(msgs), msgs)
	}

	if got := roleKind(msgs[0].Role); got != kindThinking {
		t.Fatalf("流式 reasoning 落出来的第一条必须走思考分支，实际角色 %q → kind %v",
			msgs[0].Role, got)
	}
	if msgs[0].Text != "先看一眼目录结构" {
		t.Fatalf("思考正文丢了：%q", msgs[0].Text)
	}
	if got := roleKind(msgs[1].Role); got != kindAssistant {
		t.Fatalf("第二条应当是正文，实际角色 %q → kind %v", msgs[1].Role, got)
	}
}
