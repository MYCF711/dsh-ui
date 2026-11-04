package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

// WP-C2 先红：旧实现后台只推进水位，正文既不进缓存，也不能在切回时继续显示。
func TestWPC2BackgroundFramesReduceIntoTheirOwnSessionCache(t *testing.T) {
	m := &model{
		cur: "session-b",
		cache: map[string]*cachedSession{
			"session-a": {msgs: []apiclient.Message{{Seq: 1, Role: "user", Text: "A 原消息"}}},
			"session-b": {msgs: []apiclient.Message{{Seq: 1, Role: "user", Text: "B 原消息"}}},
		},
		msgs: []apiclient.Message{{Seq: 1, Role: "user", Text: "B 原消息"}},
		seqs: map[string]int{},
	}
	m.applyEventAt("session-a", apiclient.Event{Kind: "assistant/message", Seq: 2, Data: json.RawMessage(
		`{"seq":2,"turnId":"turn-a","content":[{"type":"text","text":"A 后台完整回答"}]}`)}, 1762222445000)

	if got := m.cache["session-a"].msgs; len(got) != 2 || got[1].Text != "A 后台完整回答" || got[1].TurnID != "turn-a" {
		t.Fatalf("后台帧没有归约进 A 自己的缓存：%+v", got)
	}
	if len(m.msgs) != 1 || m.msgs[0].Text != "B 原消息" {
		t.Fatalf("A 的后台正文污染了 B 的前台：%+v", m.msgs)
	}
}

// WP-C2 先红：没有 rpcId 时，两个尚未对账的同文本发送是歧义，不能一次 durable
// user event 就把两条本地 echo 全清掉。只有唯一候选才允许做文本兜底。
func TestWPC2DuplicateOutgoingTextIsNotReconciledByTextAlone(t *testing.T) {
	m := &model{outgoing: map[string][]pluginapi.OutgoingMessage{
		"session-a": {
			{Key: "send-1", Text: "相同文本", State: pluginapi.OutgoingSent, TurnID: "turn-1"},
			{Key: "send-2", Text: "相同文本", State: pluginapi.OutgoingSending},
		},
	}}
	m.reconcileOutgoingLocked("session-a", "相同文本")
	if got := m.outgoing["session-a"]; len(got) != 2 {
		t.Fatalf("一条无身份 durable event 误清了 %d 条同文本 echo：%+v", 2-len(got), got)
	}
}

func TestWPC2OutgoingExactTurnReconcilesOnlyOneMatchingEcho(t *testing.T) {
	m := &model{outgoing: map[string][]pluginapi.OutgoingMessage{
		"session-a": {
			{Key: "send-1", Text: "相同文本", State: pluginapi.OutgoingSent, TurnID: "turn-1"},
			{Key: "send-2", Text: "相同文本", State: pluginapi.OutgoingSent, TurnID: "turn-2"},
		},
	}}
	m.reconcileOutgoingLocked("session-a", "相同文本", "turn-1")
	got := m.outgoing["session-a"]
	if len(got) != 1 || got[0].Key != "send-2" {
		t.Fatalf("TurnID 精确对账结果错误：%+v", got)
	}
}

// 真实 model.openSession + 增量帧走 A→B→A；两个会话都已有订阅和温缓存，
// 因此任何 load/refresh/取消重订都会被计数并视为回归。
func TestWPC2ABAStreamingSwitchKeepsCanonicalStateWithoutReload(t *testing.T) {
	var loads atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loads.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"messages":[]}`))
	}))
	defer ts.Close()

	cancelled := 0
	m := &model{
		cli: apiclient.New(ts.URL), mux: apiclient.NewMux(ts.URL), cur: "session-a",
		cache: map[string]*cachedSession{
			"session-a": {msgs: []apiclient.Message{{Seq: 1, Role: "user", Text: "A 问题"}}, total: 1},
			"session-b": {msgs: []apiclient.Message{{Seq: 1, Role: "user", Text: "B 问题"}}, total: 1},
		},
		msgs:   []apiclient.Message{{Seq: 1, Role: "user", Text: "A 问题"}},
		seqs:   map[string]int{"session-a": 10, "session-b": 20},
		epochs: map[string]int64{},
		subs: map[string]*muxSub{
			"session-a": {lastSeq: 10, cancel: func() { cancelled++ }},
			"session-b": {lastSeq: 20, cancel: func() { cancelled++ }},
		},
		streamStates: map[string]*sessionStreamState{},
		scrollPos:    map[string]float32{},
	}
	m.applyEventAt("session-a", apiclient.Event{Kind: "assistant/text", Seq: 11, Data: json.RawMessage(
		`{"turnId":"turn-a","delta":"前半"}`)}, 1762222445000)
	if m.streamText != "前半" {
		t.Fatalf("A 初始流正文 = %q", m.streamText)
	}

	m.applyEventAt("session-a", apiclient.Event{Kind: "approval/request", Seq: 12, Data: json.RawMessage(
		`{"requestId":"perm-A-original","toolCall":{"title":"bash: pwd","kind":"execute","rawInput":{"command":"pwd"}},"options":[{"optionId":"allow-original","name":"允许","kind":"allow_once"},{"optionId":"deny-original","name":"拒绝","kind":"reject_once"}]}`)}, 1762222445500)
	if m.approval == nil || m.approval.RequestID != "perm-A-original" {
		t.Fatalf("A 原审批未接上：%+v", m.approval)
	}

	m.openSession("session-b")
	if m.cur != "session-b" || len(m.msgs) != 1 || m.msgs[0].Text != "B 问题" {
		t.Fatalf("切到 B 后前台错误：cur=%q msgs=%+v", m.cur, m.msgs)
	}
	if m.approval != nil {
		t.Fatalf("A 的审批泄漏到 B：%+v", m.approval)
	}
	m.applyEventAt("session-a", apiclient.Event{Kind: "assistant/text", Seq: 13, Data: json.RawMessage(
		`{"turnId":"turn-a","delta":"后台后半"}`)}, 1762222446000)
	if m.streamText != "" || len(m.msgs) != 1 || m.msgs[0].Text != "B 问题" {
		t.Fatalf("A 后台流污染 B：stream=%q msgs=%+v", m.streamText, m.msgs)
	}

	m.openSession("session-a")
	if got := m.streamText; got != "前半后台后半" {
		t.Fatalf("流式切回正文不连续：%q", got)
	}
	if m.approval == nil || m.approval.RequestID != "perm-A-original" || len(m.approval.Options) != 2 ||
		m.approval.Options[0].OptionID != "allow-original" || m.approval.Options[1].OptionID != "deny-original" {
		t.Fatalf("切回后审批原 ID/options 未恢复：%+v", m.approval)
	}
	if cancelled != 0 || loads.Load() != 0 {
		t.Fatalf("温切发生重订/整页加载：cancel=%d HTTP=%d", cancelled, loads.Load())
	}
	m.applyEventAt("session-a", apiclient.Event{Kind: "turn/end", Seq: 14, Data: json.RawMessage(
		`{"turnId":"turn-a"}`)}, 1762222447000)
	if m.streamText != "" || len(m.msgs) != 2 || m.msgs[1].Text != "前半后台后半" {
		t.Fatalf("切回后的最终落条不一致：stream=%q msgs=%+v", m.streamText, m.msgs)
	}
	if got := m.cache["session-b"].msgs; len(got) != 1 || got[0].Text != "B 问题" {
		t.Fatalf("B canonical 缓存被 A 改写：%+v", got)
	}
	if cancelled != 0 || loads.Load() != 0 {
		t.Fatalf("完成帧后又发生重订/加载：cancel=%d HTTP=%d", cancelled, loads.Load())
	}
	events := m.turnEvents["session-a"]
	if len(events) != 1 || events[0].Kind != "done" || events[0].TurnID != "turn-a" || events[0].Time != 1762222447000 {
		t.Fatalf("真实 turn/end 未进入页脚状态：%+v", events)
	}
}

func TestWPC2TruncatedLoadPublishesMissingPrependTruth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"session-a","messages":[{"seq":8,"role":"assistant","text":"tail"}],"truncated":true,"total":9}`))
	}))
	defer ts.Close()
	m := &model{cli: apiclient.New(ts.URL), cur: "session-a", cache: map[string]*cachedSession{}, seqs: map[string]int{}}
	r, metadata, err := m.loadSession(context.Background(), "session-a", 1)
	if err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.integrateLoadLocked("session-a", r, metadata)
	m.mu.Unlock()
	cache := m.cache["session-a"]
	if !cache.truncated || cache.total != 9 || len(cache.msgs) != 1 {
		t.Fatalf("loadOlder 缺真值/总数未保留：%+v", cache)
	}
}
