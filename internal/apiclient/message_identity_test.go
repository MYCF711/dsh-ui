package apiclient

import "testing"

// TestLoadThenTransportSeqDoesNotOverwriteV4History 复现两个序号域碰撞：
// load 投影里是 v4 seq，WS Event.Seq 是每进程 Transport journal seq。
// 新帧自己的 v4 seq=101，但 Transport seq=1；绝不能拿后者覆盖 load 的 v4 seq=1。
func TestLoadThenTransportSeqDoesNotOverwriteV4History(t *testing.T) {
	loaded := []Message{
		{Seq: 1, Role: "assistant", Text: "最早一条"},
		{Seq: 100, Role: "assistant", Text: "load 的最后一条"},
	}
	var s Stream
	s.Apply(Event{Kind: "assistant/message", Seq: 1, Data: raw(`{
		"seq":101,"messageId":"message-101","turnId":"turn-new",
		"content":[{"type":"text","text":"新 WS 消息"}]
	}`)})

	got := MergeMessages(loaded, nil)
	got = Merge(got, &s)
	if len(got) != 3 {
		t.Fatalf("v4 1/100 加新 v4 101 应为 3 条，实际 %d：%+v", len(got), got)
	}
	if got[0].Seq != 1 || got[0].Text != "最早一条" {
		t.Fatalf("Transport seq=1 覆盖了 load 的 v4 seq=1：%+v", got[0])
	}
	if got[1].Seq != 100 || got[1].Text != "load 的最后一条" {
		t.Fatalf("load 的 v4 seq=100 被改动：%+v", got[1])
	}
	if got[2].Seq != 101 || got[2].Text != "新 WS 消息" {
		t.Fatalf("新消息必须保留 data 内 v4 seq=101：%+v", got[2])
	}
}

func TestSameV4MessageReplayDeduplicatesAcrossTransportSeq(t *testing.T) {
	loaded := []Message{{Seq: 100, Role: "assistant", Text: "同一条"}}
	var s Stream
	s.Apply(Event{Kind: "assistant/message", Seq: 1, Data: raw(`{
		"seq":100,"messageId":"message-100",
		"content":[{"type":"text","text":"同一条（重放）"}]
	}`)})

	got := Merge(loaded, &s)
	if len(got) != 1 {
		t.Fatalf("同 v4 seq=100 经 Transport seq=1 重放仍应只有 1 条，实际 %d：%+v", len(got), got)
	}
	if got[0].Seq != 100 || got[0].Text != "同一条（重放）" {
		t.Fatalf("重放应更新原 v4 消息且保留原序号：%+v", got[0])
	}
}

func TestSameMessageIDReplayDeduplicatesWithoutV4Seq(t *testing.T) {
	ev := Event{Kind: "assistant/message", Seq: 1, Data: raw(`{
		"messageId":"stable-message","turnId":"turn-1",
		"content":[{"type":"text","text":"按 messageId 去重"}]
	}`)}
	var first Stream
	first.Apply(ev)
	got := Merge(nil, &first)

	ev.Seq = 2 // 另一个 Transport 序号，内容身份不变。
	var replay Stream
	replay.Apply(ev)
	got = Merge(got, &replay)
	if len(got) != 1 {
		t.Fatalf("同 messageId 的重放不应追加，实际 %d：%+v", len(got), got)
	}
}

func TestMissingV4IdentityDoesNotAliasTransportSeq(t *testing.T) {
	loaded := []Message{{Seq: 1, Role: "assistant", Text: "旧 v4 消息"}}
	var s Stream
	s.Apply(Event{Kind: "assistant/message", Seq: 1, Data: raw(`{
		"turnId":"new-turn",
		"content":[{"type":"text","text":"没有 v4 seq/messageId 的新消息"}]
	}`)})
	got := Merge(loaded, &s)
	if len(got) != 2 || got[0].Text != "旧 v4 消息" {
		t.Fatalf("缺 v4 身份时必须有 scoped fallback，不能按 Transport seq 覆盖：%+v", got)
	}
	if got[1].Seq == 1 {
		t.Fatalf("缺 v4 seq 时不能把 Transport seq=1 写成内容 Seq：%+v", got[1])
	}
}

func TestStreamToolResultsPairByCallID(t *testing.T) {
	var s Stream
	for _, ev := range []Event{
		{Kind: "tool/call", Seq: 10, Data: raw(`{"turnId":"turn-1","callId":"call-1","tool":"bash","args":"same"}`)},
		{Kind: "tool/call", Seq: 11, Data: raw(`{"turnId":"turn-1","callId":"call-2","tool":"bash","args":"same"}`)},
		{Kind: "tool/result", Seq: 12, Data: raw(`{"turnId":"turn-1","message":{"role":"tool","toolCallId":"call-2","content":[{"type":"text","text":"result-2"}]}}`)},
		{Kind: "tool/result", Seq: 13, Data: raw(`{"turnId":"turn-1","message":{"role":"tool","toolCallId":"call-1","content":[{"type":"text","text":"result-1"}]}}`)},
	} {
		s.Apply(ev)
	}

	got := Merge(nil, &s)
	if len(got) != 2 {
		t.Fatalf("同 Tool+Args 但不同 callId 必须是两次调用，实际 %d：%+v", len(got), got)
	}
	if got[0].Summary != "result-1" || got[1].Summary != "result-2" {
		t.Fatalf("工具结果必须按 callId 配对而不是最近一条：%+v", got)
	}
}

func TestToolResultBeforeCallReconcilesByCallID(t *testing.T) {
	var s Stream
	s.Apply(Event{Kind: "tool/result", Seq: 2, Data: raw(`{
		"turnId":"turn-1","message":{"role":"tool","toolCallId":"call-early",
		"content":[{"type":"text","text":"先到的结果"}]}
	}`)})
	s.Apply(Event{Kind: "tool/call", Seq: 1, Data: raw(`{
		"turnId":"turn-1","callId":"call-early","tool":"bash","args":"echo early"
	}`)})

	got := Merge(nil, &s)
	if len(got) != 1 || got[0].Tool != "bash" || got[0].Args != "echo early" || got[0].Summary != "先到的结果" {
		t.Fatalf("result 先于 call 时应按 callId 补全同一张工具卡：%+v", got)
	}
}

func TestToolResultWithoutCallIDDoesNotCrossTurn(t *testing.T) {
	var s Stream
	s.Apply(Event{Kind: "tool/call", Seq: 1, Data: raw(`{"turnId":"turn-1","tool":"bash","args":"same"}`)})
	s.Apply(Event{Kind: "tool/result", Seq: 2, Data: raw(`{
		"turnId":"turn-2","message":{"role":"tool","content":[{"type":"text","text":"另一轮的结果"}]}
	}`)})

	got := Merge(nil, &s)
	if len(got) != 2 {
		t.Fatalf("无 callId 且 turn 不同的结果不能并入 turn-1 的调用：%+v", got)
	}
	if got[0].Summary != "" {
		t.Fatalf("turn-2 的结果串进了 turn-1：%+v", got[0])
	}
	if got[1].Summary != "另一轮的结果" {
		t.Fatalf("无法安全配对的结果仍应保留为独立结果：%+v", got[1])
	}
}

func TestStreamResetClearsToolsAndSameArgsTurnsStaySeparate(t *testing.T) {
	var s Stream
	var msgs []Message
	apply := func(ev Event) {
		s.Apply(ev)
		msgs = Merge(msgs, &s)
	}
	apply(Event{Kind: "tool/call", Seq: 1, Data: raw(`{"turnId":"turn-1","tool":"bash","args":"same"}`)})
	apply(Event{Kind: "tool/result", Seq: 2, Data: raw(`{"turnId":"turn-1","message":{"role":"tool","content":[{"type":"text","text":"第一轮结果"}]}}`)})
	apply(Event{Kind: "turn/done", Seq: 3, Data: raw(`{"turnId":"turn-1"}`)})
	s.Reset()

	if tools := s.ToolMessages(); len(tools) != 0 {
		t.Fatalf("Reset 必须清空 tools，实际残留 %+v", tools)
	}
	apply(Event{Kind: "tool/call", Seq: 4, Data: raw(`{"turnId":"turn-2","tool":"bash","args":"same"}`)})
	if tools := s.ToolMessages(); len(tools) != 1 || tools[0].Summary != "" {
		t.Fatalf("第二轮同 Tool+Args 不得复用第一轮结果：%+v", tools)
	}
	apply(Event{Kind: "tool/result", Seq: 5, Data: raw(`{"turnId":"turn-2","message":{"role":"tool","content":[{"type":"text","text":"第二轮结果"}]}}`)})

	if len(msgs) != 2 {
		t.Fatalf("两轮同 Tool+Args 应保留两条独立工具消息，实际 %d：%+v", len(msgs), msgs)
	}
	if msgs[0].Summary != "第一轮结果" || msgs[1].Summary != "第二轮结果" {
		t.Fatalf("两轮工具结果串配：%+v", msgs)
	}
}

func TestContentIdentityPreservesOriginalZeroValues(t *testing.T) {
	var s Stream
	s.Apply(Event{Kind: "assistant/message", Seq: 99, Data: raw(`{
		"seq":0,"messageId":"raw-message-id","turnId":"raw-turn-id","turn":0,"step":0,
		"content":[{"type":"text","text":"零值也是真值"}]
	}`)})
	got := Merge(nil, &s)
	if len(got) != 1 {
		t.Fatalf("应只有一条消息：%+v", got)
	}
	m := got[0]
	if m.Seq != 0 || m.V4Seq == nil || *m.V4Seq != 0 {
		t.Fatalf("data.seq=0 必须与字段缺失区分，且不能取 Transport seq=99：%+v", m)
	}
	if m.MessageID != "raw-message-id" || m.TurnID != "raw-turn-id" {
		t.Fatalf("字符串身份必须原样保留：%+v", m)
	}
	if m.Turn == nil || *m.Turn != 0 || m.Step == nil || *m.Step != 0 {
		t.Fatalf("turn=0/step=0 必须保留为已知值：%+v", m)
	}
}

func TestSameV4UserMessageReplayKeepsRoleAndDeduplicates(t *testing.T) {
	loaded := []Message{{Seq: 100, Role: "user", Text: "用户原文"}}
	var s Stream
	s.Apply(Event{Kind: "user/message", Seq: 1, Data: raw(`{
		"seq":100,"id":"user-message-100","role":"user",
		"content":[{"type":"text","text":"用户原文"}]
	}`)})
	got := Merge(loaded, &s)
	if len(got) != 1 || got[0].Role != "user" || got[0].Text != "用户原文" {
		t.Fatalf("user/message 重放必须按 v4 身份去重且保留 user 角色：%+v", got)
	}
}

func TestSameV4EventReplayInsideOneStreamIsIdempotent(t *testing.T) {
	var s Stream
	ev := Event{Kind: "assistant/message", Seq: 1, Data: raw(`{
		"seq":100,"content":[{"type":"text","text":"只出现一次"}]
	}`)}
	if !s.Apply(ev) {
		t.Fatal("首次应用应改变状态")
	}
	ev.Seq = 2
	if s.Apply(ev) {
		t.Fatal("同 v4 事件的逐字重放不应再次改变内容")
	}
	got := Merge(nil, &s)
	if len(got) != 1 || got[0].Text != "只出现一次" {
		t.Fatalf("同一 Stream 内重放不应把正文拼两遍：%+v", got)
	}
}

func TestV4ToolNamesAndSourceCallIDAreConsumed(t *testing.T) {
	var s Stream
	s.Apply(Event{Kind: "tool/call", Seq: 1, Data: raw(`{
		"seq":20,"turn":0,"step":0,"callId":"raw-call-id","name":"bash","arguments":"{}"
	}`)})
	s.Apply(Event{Kind: "tool/result", Seq: 2, Data: raw(`{
		"seq":21,"turn":0,"step":0,"message":{"role":"tool","source":{"callId":"raw-call-id"},
		"content":[{"type":"text","text":"v4 结果"}]}
	}`)})
	got := Merge(nil, &s)
	if len(got) != 1 || got[0].Tool != "bash" || got[0].Args != "{}" || got[0].Summary != "v4 结果" {
		t.Fatalf("应消费 v4 name/arguments 与 message.source.callId：%+v", got)
	}
	if got[0].CallID != "raw-call-id" || got[0].V4Seq == nil || *got[0].V4Seq != 21 {
		t.Fatalf("callId 与 result 的 v4 seq 必须原样保留：%+v", got[0])
	}
}
