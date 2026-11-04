package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Event.Time 必须与既有 Frame.Time 一样使用 int64（epoch 毫秒）。
var _ int64 = Event{}.Time

func TestCanonicalEventTime(t *testing.T) {
	const (
		envelopeTime = int64(1762000000000)
		dataTime     = int64(1762000000999)
	)
	tests := []struct {
		name  string
		frame string
		want  int64
	}{
		{
			name:  "envelope 优先于同时存在的 data.time",
			frame: `{"seq":1,"type":"turn/done","data":{"time":1762000000999},"time":1762000000000}`,
			want:  envelopeTime,
		},
		{
			name:  "仅在 envelope 缺失时读取合法 data.time",
			frame: `{"seq":1,"type":"turn/done","data":{"time":1762000000999}}`,
			want:  dataTime,
		},
		{
			name:  "两处都缺失就是未知",
			frame: `{"seq":1,"type":"turn/done","data":{}}`,
			want:  0,
		},
		{
			name:  "零 envelope 不回退 data",
			frame: `{"seq":1,"type":"turn/done","data":{"time":1762000000999},"time":0}`,
			want:  0,
		},
		{
			name:  "负 envelope 不回退 data",
			frame: `{"seq":1,"type":"turn/done","data":{"time":1762000000999},"time":-1}`,
			want:  0,
		},
		{
			name:  "字符串 envelope 不回退 data",
			frame: `{"seq":1,"type":"turn/done","data":{"time":1762000000999},"time":"1762000000000"}`,
			want:  0,
		},
		{
			name:  "null envelope 不回退 data",
			frame: `{"seq":1,"type":"turn/done","data":{"time":1762000000999},"time":null}`,
			want:  0,
		},
		{
			name:  "小数 envelope 不回退 data",
			frame: `{"seq":1,"type":"turn/done","data":{"time":1762000000999},"time":1.5}`,
			want:  0,
		},
		{
			name:  "零 data.time 是未知",
			frame: `{"seq":1,"type":"turn/done","data":{"time":0}}`,
			want:  0,
		},
		{
			name:  "负 data.time 是未知",
			frame: `{"seq":1,"type":"turn/done","data":{"time":-1}}`,
			want:  0,
		},
		{
			name:  "字符串 data.time 不猜测",
			frame: `{"seq":1,"type":"turn/done","data":{"time":"1762000000999"}}`,
			want:  0,
		},
		{
			name:  "null data.time 不猜测",
			frame: `{"seq":1,"type":"turn/done","data":{"time":null}}`,
			want:  0,
		},
		{
			name:  "小数 data.time 不猜测",
			frame: `{"seq":1,"type":"turn/done","data":{"time":1.5}}`,
			want:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Frame 与 WS 必须走同一套 canonical time 规则；非法 time 不应吞掉整帧。
			var decoded Frame
			if err := json.Unmarshal([]byte(tc.frame), &decoded); err != nil {
				t.Fatalf("canonical Frame 解码失败：%v", err)
			}
			if decoded.Time != tc.want {
				t.Fatalf("Frame.Time=%d，期望 %d", decoded.Time, tc.want)
			}

			h := newFakeHost(t)
			m := NewMux(h.URL())
			m.PingEvery = time.Hour
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ch, err := m.Subscribe(ctx, "session:time", 0)
			if err != nil {
				t.Fatalf("订阅失败：%v", err)
			}
			h.waitSub(t, "session:time", 1)
			h.conn(t, "session:time", 1).push(t, tc.frame)

			ev := msNextEvent(t, ch, 3*time.Second)
			if ev.Time != tc.want {
				t.Fatalf("WS Event.Time=%d，期望 %d；事件=%+v", ev.Time, tc.want, ev)
			}
			if ev.Kind != "turn/done" || ev.Seq != 1 || ev.Topic != "session:time" {
				t.Fatalf("time 解码破坏了既有事件字段：%+v", ev)
			}
		})
	}
}

func TestHistoryLiveAndReplayPreserveSameEventTime(t *testing.T) {
	const wireFrame = `{"seq":7,"type":"turn/done","data":{"time":1762000000999},"time":1762000000000}`
	const wantTime = int64(1762000000000)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"frames":[` + wireFrame + `],"lastSeq":7,"journalEpoch":9}}`))
	}))
	defer srv.Close()
	history, err := New(srv.URL).History(context.Background(), "s-time", 6, 1, 9)
	if err != nil {
		t.Fatalf("history 失败：%v", err)
	}
	if len(history.Frames) != 1 || history.Frames[0].Time != wantTime {
		t.Fatalf("history 没保留 envelope time：%+v", history.Frames)
	}
	if history.LastSeq != 7 || history.JournalEpoch != 9 {
		t.Fatalf("history 水位语义被改变：%+v", history)
	}

	h := newFakeHost(t)
	m := NewMux(h.URL())
	m.PingEvery = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := m.Subscribe(ctx, "session:s-time", 6)
	if err != nil {
		t.Fatalf("订阅失败：%v", err)
	}
	h.waitSub(t, "session:s-time", 1)
	h.conn(t, "session:s-time", 1).push(t, wireFrame)
	live := msNextEvent(t, ch, 3*time.Second)
	if live.Time != wantTime || live.Time != history.Frames[0].Time {
		t.Fatalf("live/history 时刻不同：live=%d history=%d", live.Time, history.Frames[0].Time)
	}

	replayed := history.Frames[0].Event("session:s-time")
	if replayed.Time != live.Time || replayed.Kind != live.Kind || replayed.Seq != live.Seq ||
		replayed.Topic != live.Topic || string(replayed.Data) != string(live.Data) {
		t.Fatalf("重放没有完整保留 live 帧：replay=%+v live=%+v", replayed, live)
	}
}
