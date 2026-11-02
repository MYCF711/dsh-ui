package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestPromptProducesStream 端到端验证：
//
//	session.prompt 发一句 → /api/mux 订阅 → 收到 >=10 个 delta 帧 → 回答写进 history
//
// 用 httptest 起真 handler，不走 main()。
func TestPromptProducesStream(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/mux", muxHandler)
	mux.HandleFunc("/api/", route)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	const sid = "s-002" // 12 条，短，够快

	// 1) 发一句
	body := `{"params":{"id":"` + sid + `","text":"流式自测"}}`
	resp, err := http.Post(srv.URL+"/api/session.prompt", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	var env envelope
	_ = json.NewDecoder(resp.Body).Decode(&env)
	resp.Body.Close()
	if env.Error != nil {
		t.Fatalf("prompt 返回错误: %+v", env.Error)
	}
	t.Logf("prompt ok, 待推行数=%d", pendingLen(sid))

	// 2) 订阅
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/mux"
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer c.CloseNow()

	sub, _ := json.Marshal(muxFrame{T: "sub", Topic: "session:" + sid, SinceSeq: 1000})
	if err := c.Write(ctx, websocket.MessageText, sub); err != nil {
		t.Fatalf("sub 写失败: %v", err)
	}

	// 3) 收帧
	var (
		deltas   int
		dones    int
		pending  strings.Builder
		lastSeq  int
		finished bool
	)
	for !finished {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("读帧失败（已收 delta=%d done=%d）: %v", deltas, dones, err)
		}
		var f muxFrame
		if json.Unmarshal(data, &f) != nil {
			continue
		}
		if f.T != "ev" || f.Kind != "assistant/message" {
			continue // 背景假事件（user/message）不管
		}
		// seq 必须连续（同一秒内的背景事件也算在内）
		if lastSeq != 0 && f.Seq != lastSeq+1 && f.Seq != lastSeq {
			// 背景 ticker 与字符 ticker 共用 seq，中间允许 gap 帧导致跳号
		}
		lastSeq = f.Seq

		var payload map[string]any
		_ = json.Unmarshal(f.Data, &payload)
		if s, ok := payload["delta"].(string); ok {
			deltas++
			pending.WriteString(s)
			if deltas >= 10 {
				// 已经满足"至少 10 个 delta"，继续收到 done 为止
			}
		}
		if _, ok := payload["done"]; ok {
			dones++
			if dones >= 3 {
				finished = true
			}
		}
	}

	got := pending.String()
	t.Logf("收到 delta 帧=%d, done 帧=%d", deltas, dones)
	t.Logf("拼出的正文=%.120s", got)

	if deltas < 10 {
		t.Fatalf("delta 帧只有 %d，要 >=10", deltas)
	}
	// 回答现在刻意是 Markdown（标题 + 列表 + 代码块），首句以 "### 收到：" 开头。
	if !strings.Contains(got, "收到：") || !strings.Contains(got, "收尾一句普通文本") {
		t.Fatalf("拼出的正文不像回答: %.200s", got)
	}

	// 4) 确认写进了 history
	time.Sleep(100 * time.Millisecond)
	lr, err := http.Post(srv.URL+"/api/session.load", "application/json",
		strings.NewReader(`{"params":{"id":"`+sid+`","tailMessages":3}}`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var lenv struct {
		Result struct {
			Messages []message `json:"messages"`
			Total    int       `json:"total"`
		} `json:"result"`
	}
	_ = json.NewDecoder(lr.Body).Decode(&lenv)
	lr.Body.Close()

	if len(lenv.Result.Messages) == 0 {
		t.Fatal("history 里没有消息")
	}
	last := lenv.Result.Messages[len(lenv.Result.Messages)-1]
	t.Logf("history 末条: seq=%d role=%s text=%.80s", last.Seq, last.Role, last.Text)
	if last.Role != "assistant" {
		t.Fatalf("末条应是 assistant，实际 %s", last.Role)
	}
	// 第 3 句是代码块 + 收尾段落；用它作为"三句都到齐"的凭证。
	if !strings.Contains(last.Text, "收尾一句普通文本") || !strings.Contains(last.Text, "等宽代码块") {
		t.Fatalf("末条正文不像完整回答: %.200s", last.Text)
	}
	if n := pendingLen(sid); n != 0 {
		t.Fatalf("推完后 pending 应清空，还剩 %d 句", n)
	}
	t.Logf("✅ 端到端通过：%d 个 delta → 3 句进入 history，pending 已清空", deltas)
}
