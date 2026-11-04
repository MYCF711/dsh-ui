package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

func TestWPC1LoadPreservesOriginalTimeAndStoppedWithoutInventingThem(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"s-meta","messages":[{"seq":8,"role":"assistant","text":"partial","time":1762222445000,"stopped":true},{"seq":9,"role":"assistant","text":"unknown"}],"total":2}`)
	}))
	defer ts.Close()
	m := &model{cli: apiclient.New(ts.URL)}
	result, metadata, err := m.loadSession(context.Background(), "s-meta", 40)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Messages) != 2 {
		t.Fatalf("消息数 = %d", len(result.Messages))
	}
	key := pluginapi.MessageKey("s-meta", result.Messages[0], 0)
	if got := metadata[key]; got.Time != 1762222445000 || !got.Stopped {
		t.Fatalf("原始 time/stopped 丢失：%+v", got)
	}
	unknown := pluginapi.MessageKey("s-meta", result.Messages[1], 1)
	if got := metadata[unknown]; got.Time != 0 || got.Stopped {
		t.Fatalf("缺真值的消息被补了元信息：%+v", got)
	}
}

func TestWPC1TurnErrorAndExplicitStoppedSurviveStreamReduction(t *testing.T) {
	m := &model{cur: "s-terminal", seqs: map[string]int{}}
	m.applyEventAt("s-terminal", apiclient.Event{Kind: "assistant/message", Data: json.RawMessage(
		`{"turnId":"turn-real","content":[{"type":"text","text":"部分回答"}]}`)}, 1762222444000)
	m.applyEventAt("s-terminal", apiclient.Event{Kind: "turn/error", Data: json.RawMessage(
		`{"turnId":"turn-real","error":"model failed"}`)}, 1762222445000)
	m.applyEventAt("s-terminal", apiclient.Event{Kind: "turn/stopped", Data: json.RawMessage(
		`{"turnId":"turn-real","stopped":true}`)}, 1762222446000)

	events := m.turnEvents["s-terminal"]
	if len(events) != 2 {
		t.Fatalf("终局事件 = %+v，期望 error+stopped", events)
	}
	if events[0].Kind != "error" || events[0].TurnID != "turn-real" || events[0].Error != "model failed" || events[0].Time != 1762222445000 {
		t.Fatalf("turn/error 真值未保留：%+v", events[0])
	}
	if events[1].Kind != "stopped" || events[1].Time != 1762222446000 {
		t.Fatalf("stopped 真值未保留：%+v", events[1])
	}
	if len(m.msgs) == 0 || m.msgs[len(m.msgs)-1].Text != "部分回答" {
		t.Fatalf("终局前的部分正文丢失：%+v", m.msgs)
	}
}
