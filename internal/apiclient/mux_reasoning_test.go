// mux_reasoning_test.go —— H6：流式 reasoning 的消费链。
//
// 分两层测，故意不混：
//  1. **纯归约**（Chunks / Stream / Merge）：给一串帧，断言落出来的消息；
//  2. **真 WS 回路**：起一个说 H14 帧形状（{seq,type,data}）的假宿主，
//     证明"线上字节 → Event → 消息"这条链真的通，而不是只有纯函数对。
//
// 为什么要第 2 层：只测纯函数的话，"服务端帧里 reasoning 到底有没有被解析出来"
// 仍然是没有证据的——而 H6 卡住的恰恰就是这一段。
package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// raw 是 json.RawMessage 的简写（测试里到处都是，写全了看不清帧序列）。
func raw(s string) json.RawMessage { return json.RawMessage(s) }

// TestChunksV4ContentBlocks 钉住 v4 原始事件的拆块规则。
//
// 真宿主 follow 推的就是这种帧：`assistant/message` 带 content 数组，
// **一个 delta 字段都没有**。只认 delta 的实现在这里会拿到 0 段。
func TestChunksV4ContentBlocks(t *testing.T) {
	ev := Event{Kind: "assistant/message", Seq: 7, Data: raw(`{
		"content":[
			{"type":"reasoning","text":"先看看有没有现成的"},
			{"type":"text","text":"有，直接用"}
		]}`)}
	ch := ev.Chunks()
	if len(ch) != 2 {
		t.Fatalf("应当拆成 2 段（思考 + 正文），得到 %d 段：%+v", len(ch), ch)
	}
	if ch[0].Block != BlockReasoning || ch[0].Text != "先看看有没有现成的" || !ch[0].Full {
		t.Fatalf("第一段应当是完整思考块，得到 %+v", ch[0])
	}
	if ch[1].Block != BlockText || ch[1].Text != "有，直接用" || !ch[1].Full {
		t.Fatalf("第二段应当是完整正文块，得到 %+v", ch[1])
	}
}

// TestChunksMessageContent 覆盖"块放在 data.message.content"的另一种落法。
// 真实语料两种都见过；只认一种，那些会话就是一条消息都没有。
func TestChunksMessageContent(t *testing.T) {
	ev := Event{Kind: "assistant/message", Seq: 3, Data: raw(`{
		"message":{"content":[{"type":"reasoning","text":"想一想"}]}}`)}
	ch := ev.Chunks()
	if len(ch) != 1 || ch[0].Block != BlockReasoning || ch[0].Text != "想一想" {
		t.Fatalf("应当只拆出一段思考，得到 %+v", ch)
	}
}

// TestChunksReasoningOnlyFrame 钉住"只有 reasoning 没有 text 的帧"。
// 这种帧以前会被 `text == ""` 的判据整条丢掉——而真实语料里它占八成。
func TestChunksReasoningOnlyFrame(t *testing.T) {
	for _, kind := range []string{"assistant/reasoning", "assistant/thought"} {
		ev := Event{Kind: kind, Seq: 4, Data: raw(`{"delta":"只有思考"}`)}
		ch := ev.Chunks()
		if len(ch) != 1 || ch[0].Block != BlockReasoning || ch[0].Text != "只有思考" {
			t.Fatalf("kind=%s 应当拆出一段思考，得到 %+v", kind, ch)
		}
	}
	// turn/done 没有内容，不该凭空造出一段正文。
	if ch := (Event{Kind: "turn/done", Seq: 5, Data: raw(`{}`)}).Chunks(); len(ch) != 0 {
		t.Fatalf("turn/done 不该产出内容段，得到 %+v", ch)
	}
}

// TestStreamMergeInterleaved 是主用例：text / reasoning **交错**来，
// 合并后 Msgs 里两种角色都在、顺序对（先想后说）、同一段的增量不重复追加。
func TestStreamMergeInterleaved(t *testing.T) {
	frames := []Event{
		{Kind: "assistant/reasoning", Seq: 10, Data: raw(`{"delta":"先看"}`)},
		{Kind: "assistant/message", Seq: 11, Data: raw(`{"delta":"结论"}`)},
		{Kind: "assistant/reasoning", Seq: 12, Data: raw(`{"delta":"一眼目录"}`)},
		{Kind: "assistant/message", Seq: 13, Data: raw(`{"delta":"：可以用"}`)},
		{Kind: "assistant/message", Seq: 14, Data: raw(`{"done":true}`)},
	}
	var (
		msgs []Message
		s    Stream
	)
	for _, ev := range frames {
		if !s.Apply(ev) {
			t.Fatalf("seq=%d kind=%s 应当改变流状态", ev.Seq, ev.Kind)
		}
		msgs = Merge(msgs, &s)
		if s.Done() {
			s.Reset()
		}
	}

	if len(msgs) != 2 {
		t.Fatalf("交错增量应当只落成 2 条（思考 + 正文），得到 %d 条：%+v", len(msgs), msgs)
	}
	if msgs[0].Role != "thinking" {
		t.Fatalf("第一条应当是思考（先想后说），得到角色 %q", msgs[0].Role)
	}
	if msgs[0].Text != "先看一眼目录" {
		t.Fatalf("思考正文拼错了：%q", msgs[0].Text)
	}
	if msgs[1].Role != "assistant" || msgs[1].Text != "结论：可以用" {
		t.Fatalf("第二条应当是拼好的正文，得到 %+v", msgs[1])
	}
	if msgs[0].Seq != msgs[1].Seq {
		t.Fatalf("同一段的两条消息必须共用同一个 seq（否则每次增量都会新增一条）：%d vs %d",
			msgs[0].Seq, msgs[1].Seq)
	}
}

// TestStreamReasoningOnly 钉住"只有思考、没有正文"的整段。
// 不能因为正文为空就把整条丢掉——那正是 H6 里最容易被忽略的一半。
func TestStreamReasoningOnly(t *testing.T) {
	var (
		msgs []Message
		s    Stream
	)
	for _, ev := range []Event{
		{Kind: "assistant/reasoning", Seq: 1, Data: raw(`{"delta":"只想不说"}`)},
		{Kind: "turn/done", Seq: 2, Data: raw(`{}`)},
	} {
		s.Apply(ev)
		msgs = Merge(msgs, &s)
		if s.Done() {
			s.Reset()
		}
	}
	if len(msgs) != 1 {
		t.Fatalf("只有思考的一段应当落 1 条消息，得到 %d 条：%+v", len(msgs), msgs)
	}
	if msgs[0].Role != "thinking" || msgs[0].Text != "只想不说" {
		t.Fatalf("应当是一条思考消息，得到 %+v", msgs[0])
	}
}

// TestStreamSameSeqNoDuplicate 钉住"同 seq 的增量更新同一条，不重复追加"。
//
// 三条路都会让同一个 seq 再来一次：断线重连重放、同一帧被重复投递、
// 以及"先流式落了一条、随后 session.load 又给了同一条"。
func TestStreamSameSeqNoDuplicate(t *testing.T) {
	msgs := []Message{{Seq: 9, Role: "assistant", Text: "旧的正文"}}

	var s Stream
	// 思考比正文晚到：也必须排在正文**前面**（先想后说），不能追加到末尾。
	if !s.Apply(Event{Kind: "assistant/reasoning", Seq: 9, Data: raw(`{"delta":"想了一半"}`)}) {
		t.Fatal("思考增量应当改变状态")
	}
	msgs = Merge(msgs, &s)
	msgs = Merge(msgs, &s) // 同一帧再来一次（重放）
	if len(msgs) != 2 {
		t.Fatalf("同 seq 重放不该新增消息，得到 %d 条：%+v", len(msgs), msgs)
	}
	if msgs[0].Role != "thinking" || msgs[1].Role != "assistant" {
		t.Fatalf("思考必须排在正文前，得到 %q / %q", msgs[0].Role, msgs[1].Role)
	}

	// 同一段再来一条增量：更新同一条，而不是插一条新的。
	s.Apply(Event{Kind: "assistant/reasoning", Seq: 9, Data: raw(`{"delta":"，想完了"}`)})
	msgs = Merge(msgs, &s)
	if len(msgs) != 2 {
		t.Fatalf("同 seq 的增量应当更新同一条，得到 %d 条：%+v", len(msgs), msgs)
	}
	if msgs[0].Text != "想了一半，想完了" {
		t.Fatalf("同一条思考应当累积，得到 %q", msgs[0].Text)
	}
}

// TestStreamToolFrameMakesNoMessage 钉住"tool/call 不落空卡"。
// 一张没有结果的工具卡比不画更糟。
func TestStreamToolFrameMakesNoMessage(t *testing.T) {
	var s Stream
	if s.Apply(Event{Kind: "tool/call", Seq: 5, Data: raw(`{"tool":"bash","args":"git status"}`)}) {
		t.Fatal("只有 tool/call 时不该改变消息状态")
	}
	if msgs := Merge(nil, &s); len(msgs) != 0 {
		t.Fatalf("tool/call 不该落消息，得到 %+v", msgs)
	}
}

// TestMuxReadsReasoningOverWS 是线上回路：假宿主按 H14 的形状推帧
// （`{seq,type,data,time}`，没有 topic 信封），客户端解析出来之后
// 走与生产环境完全相同的 Chunks/Stream/Merge。
func TestMuxReadsReasoningOverWS(t *testing.T) {
	frames := []string{
		// ① v4 原始事件：一条完整的助手消息，思考 + 正文在同一帧里
		`{"seq":1,"type":"assistant/message","time":1762000000000,"data":{"content":[{"type":"reasoning","text":"先数一下有多少条"},{"type":"text","text":"一共 42 条"}]}}`,
		// ② 心跳回包：不是事件，必须被跳过（不能当成一条空消息）
		`{"type":"pong"}`,
		// ③ 只有思考、没有正文的另一条
		`{"seq":2,"type":"assistant/message","data":{"content":[{"type":"reasoning","text":"再想想边界"}]}}`,
		// ④ 一轮结束
		`{"seq":3,"type":"turn/done","data":{}}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws" {
			http.NotFound(w, r)
			return
		}
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // 先收 sub 帧
			return
		}
		for _, f := range frames {
			if err := c.Write(ctx, websocket.MessageText, []byte(f)); err != nil {
				return
			}
		}
		<-ctx.Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	m := NewMux(srv.URL)
	ch, err := m.Subscribe(ctx, "session:s-1", 0)
	if err != nil {
		t.Fatalf("Subscribe 失败：%v", err)
	}

	var (
		msgs []Message
		s    Stream
		got  []string
	)
	for ev := range ch {
		got = append(got, ev.Kind)
		if ev.Kind == "gap" {
			t.Fatalf("帧序号是连续的，不该报 gap：%+v", ev.Gap)
		}
		s.Apply(ev)
		msgs = Merge(msgs, &s)
		if s.Done() {
			s.Reset()
		}
		if len(msgs) == 3 {
			cancel() // 该到的都到了，收工
			break
		}
	}

	if len(msgs) != 3 {
		t.Fatalf("应当落 3 条消息（思考+正文 / 只有思考 / 无），得到 %d 条：%+v（收到的事件 %v）",
			len(msgs), msgs, got)
	}
	if msgs[0].Role != "thinking" || msgs[0].Text != "先数一下有多少条" {
		t.Fatalf("第一条应当是思考块，得到 %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Text != "一共 42 条" {
		t.Fatalf("第二条应当是正文，得到 %+v", msgs[1])
	}
	if msgs[2].Role != "thinking" || msgs[2].Text != "再想想边界" {
		t.Fatalf("第三条应当只有思考、没有正文，得到 %+v", msgs[2])
	}
}
