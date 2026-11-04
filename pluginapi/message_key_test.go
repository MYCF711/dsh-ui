package pluginapi

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func TestMessageKeyUsesMSGIDAndSessionLocalFallback(t *testing.T) {
	v4 := int64(41)
	turn := int64(7)
	step := int64(2)
	cases := []struct {
		name string
		msg  apiclient.Message
		want string
	}{
		{name: "v4", msg: apiclient.Message{V4Seq: &v4, Seq: 0, Role: "assistant"}, want: "v4:41"},
		{name: "message id", msg: apiclient.Message{MessageID: "message-real", Role: "assistant"}, want: "message:message-real"},
		{name: "turn step", msg: apiclient.Message{Turn: &turn, Step: &step, Role: "assistant"}, want: "turn:7"},
		{name: "legacy load seq is namespaced", msg: apiclient.Message{Seq: 41, Role: "assistant"}, want: "v4-load:41"},
		{name: "unknown seq never bare", msg: apiclient.Message{Seq: 0, Role: "assistant"}, want: "local:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := MessageKey("session-a", tc.msg, 3)
			if !strings.Contains(key, tc.want) || !strings.HasPrefix(key, "session-a\x00") {
				t.Fatalf("key = %q，缺稳定身份命名空间 %q", key, tc.want)
			}
			if key == "0" || key == "41" {
				t.Fatalf("仍使用裸 msg.Seq 作为渲染 key：%q", key)
			}
		})
	}

	unknownA := MessageKey("session-a", apiclient.Message{Role: "thinking"}, 1)
	unknownB := MessageKey("session-a", apiclient.Message{Role: "thinking"}, 2)
	if unknownA == unknownB {
		t.Fatal("两个未知 v4 seq 的思考块共享了渲染 key")
	}
	if other := MessageKey("session-b", apiclient.Message{Role: "thinking"}, 1); other == unknownA {
		t.Fatal("局部渲染 key 跨会话串用")
	}
	if MessageKey("session-a", apiclient.Message{Seq: 9, Role: "tool", CallID: "call-a"}, 0) ==
		MessageKey("session-a", apiclient.Message{Seq: 9, Role: "tool", CallID: "call-b"}, 0) {
		t.Fatal("同 seq 的不同工具调用共享了展开 key")
	}
	withoutStepA := MessageKey("session-a", apiclient.Message{TurnID: "turn-a", Role: "assistant"}, 1)
	withoutStepB := MessageKey("session-a", apiclient.Message{TurnID: "turn-a", Role: "assistant"}, 2)
	if withoutStepA == withoutStepB {
		t.Fatal("同 Turn 缺 Step 的两条 assistant 共享了局部视图键")
	}
	withStep := apiclient.Message{TurnID: "turn-a", Step: &step, Role: "assistant"}
	if MessageKey("session-a", withStep, 1) != MessageKey("session-a", withStep, 2) {
		t.Fatal("真实 TurnID+Step 的身份不应随窗口下标改变")
	}
}
