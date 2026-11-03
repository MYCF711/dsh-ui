// merge_test.go —— 「刷新不能吞历史」。
//
// session.load 给的是**尾巴**（-tail 默认 40 条）。界面手里可能已经有更完整的一份，
// 直接 msgs = r.Messages 会把尾巴之前的内容整段吃掉。实测：切走后给 A 发新消息、
// 切回来时界面「共 2 条（总 2）」而宿主是 4 条——第一轮问答消失了。
package apiclient

import "testing"

func TestMergeMessagesKeepsHistory(t *testing.T) {
	// 手里有 4 条（1..4），fresh 只给最后 2 条（3..4）
	keep := []Message{
		{Seq: 1, Role: "user", Text: "q1"},
		{Seq: 2, Role: "assistant", Text: "a1"},
		{Seq: 3, Role: "user", Text: "q2"},
		{Seq: 4, Role: "assistant", Text: "a2"},
	}
	fresh := []Message{
		{Seq: 3, Role: "user", Text: "q2"},
		{Seq: 4, Role: "assistant", Text: "a2"},
	}
	got := MergeMessages(keep, fresh)
	if len(got) != 4 {
		t.Fatalf("合并后应仍是 4 条（不能因为 fresh 只有尾巴就丢 1/2），得到 %d：%+v", len(got), got)
	}
	if got[0].Text != "q1" || got[1].Text != "a1" {
		t.Fatalf("历史被吃了：%+v", got)
	}
}

func TestMergeMessagesUpdatesAndAppends(t *testing.T) {
	keep := []Message{
		{Seq: 1, Role: "user", Text: "q1"},
		{Seq: 2, Role: "assistant", Text: "a1-旧"},
	}
	fresh := []Message{
		{Seq: 2, Role: "assistant", Text: "a1-新"}, // 同 seq 同 role → 更新
		{Seq: 3, Role: "user", Text: "q2"},        // 新的 → 追加
	}
	got := MergeMessages(keep, fresh)
	if len(got) != 3 {
		t.Fatalf("应是 3 条（更新 1 + 追加 1），得到 %d：%+v", len(got), got)
	}
	if got[1].Text != "a1-新" {
		t.Fatalf("同 seq 应更新而不是保留旧的，得到 %q", got[1].Text)
	}
	if got[2].Text != "q2" {
		t.Fatalf("新消息应追加，得到 %+v", got)
	}
}

func TestMergeMessagesDoesNotMutateKeep(t *testing.T) {
	keep := []Message{{Seq: 1, Role: "user", Text: "q1"}}
	_ = MergeMessages(keep, []Message{{Seq: 2, Role: "assistant", Text: "a1"}})
	if len(keep) != 1 {
		t.Fatalf("不该改动入参，keep 变成 %d 条", len(keep))
	}
}

func TestMergeMessagesToolIdentity(t *testing.T) {
	// 旧投影没有 callId/turn 时，以 (Seq,Tool,Args) 认身份；这里的零 Seq
	// 是既有调用方的未指定值，不是 Transport 水位。
	keep := []Message{{Role: "tool", Tool: "bash", Args: "ls", Summary: ""}}
	fresh := []Message{{Role: "tool", Tool: "bash", Args: "ls", Summary: "[exit 0] a.go"}}
	got := MergeMessages(keep, fresh)
	if len(got) != 1 {
		t.Fatalf("同一工具应更新而不是新增，得到 %d 条", len(got))
	}
	if got[0].Summary != "[exit 0] a.go" {
		t.Fatalf("结果应补上，得到 %q", got[0].Summary)
	}
}

func TestMergeMessagesLegacyToolsDoNotCrossV4Seq(t *testing.T) {
	keep := []Message{{Seq: 10, Role: "tool", Tool: "bash", Args: "same", Summary: "第一轮"}}
	fresh := []Message{{Seq: 20, Role: "tool", Tool: "bash", Args: "same", Summary: "第二轮"}}
	got := MergeMessages(keep, fresh)
	if len(got) != 2 || got[0].Summary != "第一轮" || got[1].Summary != "第二轮" {
		t.Fatalf("不同 v4 seq 的同参数工具不得跨轮覆盖：%+v", got)
	}
}
