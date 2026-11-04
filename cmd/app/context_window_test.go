package main

import (
	"encoding/json"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// STATS-WIRE：容量是跟会话缓存走的真值，不是全局最后一次 load 的值。
func TestContextWindowLoadCompactionAndABAIsolation(t *testing.T) {
	m := &model{
		cur: "session-a",
		cache: map[string]*cachedSession{
			"session-a": {contextTokens: 2000, contextWindow: 10000},
			"session-b": {contextTokens: 4000, contextWindow: 32000},
		},
		seqs:   map[string]int{},
		epochs: map[string]int64{},
		subs: map[string]*muxSub{
			"session-a": {cancel: func() {}},
			"session-b": {cancel: func() {}},
		},
		scrollPos: map[string]float32{},
	}
	m.publishSessionLocked("session-a")
	m.publishContextWindowLocked("session-a")
	if m.contextTokens != 2000 || m.contextWindow != 10000 {
		t.Fatalf("A 初始发布 = %d/%d", m.contextTokens, m.contextWindow)
	}

	m.integrateLoadWithContextWindowLocked("session-a", &apiclient.LoadResult{
		ID: "session-a", ContextTokens: 500, ContextWindow: 10000,
	}, nil)
	if got := m.cache["session-a"]; got.contextTokens != 500 || got.contextWindow != 10000 {
		t.Fatalf("压缩后的 load 未进入 A 缓存：%+v", got)
	}

	m.openSession("session-b")
	if m.contextTokens != 4000 || m.contextWindow != 32000 {
		t.Fatalf("切到 B 套用了 A 的容量：used/window=%d/%d", m.contextTokens, m.contextWindow)
	}
	// 后台 A 的 history/live 归约只能改 A 的内容，不得改写 B 的前台容量。
	m.applyEventAt("session-a", apiclient.Event{Kind: "assistant/message", Seq: 1, Data: json.RawMessage(
		`{"seq":1,"content":[{"type":"text","text":"A live"}]}`)}, 1762222445000)
	m.applyHistoryFrame("session-a", apiclient.Frame{Seq: 2, Type: "assistant/message", Data: json.RawMessage(
		`{"seq":2,"content":[{"type":"text","text":"A history"}]}`)})
	if m.contextTokens != 4000 || m.contextWindow != 32000 {
		t.Fatalf("A 后台帧污染 B 容量：used/window=%d/%d", m.contextTokens, m.contextWindow)
	}
	m.openSession("session-a")
	if m.contextTokens != 500 || m.contextWindow != 10000 {
		t.Fatalf("切回 A 未恢复自己的压缩后真值：used/window=%d/%d", m.contextTokens, m.contextWindow)
	}
}

func TestContextWindowUnknownSessionAndJournalReset(t *testing.T) {
	m := &model{
		cur: "session-a",
		cache: map[string]*cachedSession{
			"session-a": {contextTokens: 2000, contextWindow: 10000},
			"session-b": {contextTokens: 100, contextWindow: 0},
		},
		seqs:      map[string]int{"session-a": 8},
		epochs:    map[string]int64{"session-a": 1},
		subs:      map[string]*muxSub{"session-a": {lastSeq: 8, cancel: func() {}}, "session-b": {cancel: func() {}}},
		scrollPos: map[string]float32{},
	}
	m.openSession("session-b")
	if m.contextWindow != 0 || m.contextTokens != 100 {
		t.Fatalf("未知容量的 B 继承了 A：used/window=%d/%d", m.contextTokens, m.contextWindow)
	}
	m.openSession("session-a")
	m.mu.Lock()
	m.onJournalResetLocked("session-a")
	m.mu.Unlock()
	if m.contextTokens != 2000 || m.contextWindow != 10000 {
		t.Fatalf("reset 丢掉了 A 仍然正确的容量：used/window=%d/%d", m.contextTokens, m.contextWindow)
	}
	if got := m.cache["session-b"]; got.contextTokens != 100 || got.contextWindow != 0 {
		t.Fatalf("reset A 改写了 B：%+v", got)
	}
}
