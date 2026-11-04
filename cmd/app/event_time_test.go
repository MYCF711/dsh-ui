package main

import (
	"encoding/json"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func TestShellLiveApplyEventUsesCanonicalEventTime(t *testing.T) {
	for _, tc := range []struct {
		name       string
		eventTime  int64
		dataTime   int64
		wantFooter int64
	}{
		{name: "envelope wins", eventTime: 1762222447000, dataTime: 1762222444000, wantFooter: 1762222447000},
		{name: "zero remains unknown", eventTime: 0, dataTime: 1762222444000, wantFooter: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const session = "s-live-time"
			m := &model{cur: session, seqs: map[string]int{}}
			data, err := json.Marshal(map[string]any{"turnId": "turn-live", "time": tc.dataTime})
			if err != nil {
				t.Fatal(err)
			}
			m.applyEvent(session, apiclient.Event{Kind: "turn/end", Seq: 1, Topic: "session:" + session, Data: data, Time: tc.eventTime})
			events := m.turnEvents[session]
			if len(events) != 1 || events[0].Kind != "done" || events[0].Time != tc.wantFooter {
				t.Fatalf("live canonical time 未进入 Turn footer：%+v", events)
			}
		})
	}
}

func TestShellHistoryReplayUsesFrameEventWithoutSecondTimeGuess(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want int64
	}{
		{name: "canonical envelope", raw: `{"seq":7,"type":"turn/end","time":1762222447000,"data":{"turnId":"turn-history","time":1762222444000}}`, want: 1762222447000},
		{name: "zero envelope stays unknown", raw: `{"seq":7,"type":"turn/end","time":0,"data":{"turnId":"turn-history","time":1762222444000}}`, want: 0},
		{name: "invalid envelope stays unknown", raw: `{"seq":7,"type":"turn/end","time":"invalid","data":{"turnId":"turn-history","time":1762222444000}}`, want: 0},
		{name: "missing envelope uses apiclient fallback", raw: `{"seq":7,"type":"turn/end","data":{"turnId":"turn-history","time":1762222444000}}`, want: 1762222444000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var frame apiclient.Frame
			if err := json.Unmarshal([]byte(tc.raw), &frame); err != nil {
				t.Fatal(err)
			}
			event := frame.Event("session:s-history-time")
			if event.Kind != "turn/end" || event.Seq != 7 || event.Topic != "session:s-history-time" || event.Time != tc.want || string(event.Data) == "" {
				t.Fatalf("Frame.Event 未保真：%+v", event)
			}

			m := &model{cur: "s-history-time", seqs: map[string]int{}}
			m.applyHistoryFrame("s-history-time", frame)
			events := m.turnEvents["s-history-time"]
			if len(events) != 1 || events[0].Kind != "done" || events[0].TurnID != "turn-history" || events[0].Time != tc.want {
				t.Fatalf("history 重放时间被二次猜测：%+v", events)
			}
			if m.lastSeq != 7 || m.seqs["s-history-time"] != 7 {
				t.Fatalf("history 重放改变身份/水位语义：lastSeq=%d seqs=%+v", m.lastSeq, m.seqs)
			}
		})
	}
}
