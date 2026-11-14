// reference_wiring_test.go —— 壳侧候选/反馈缓存的**真实请求路径**。
//
// 这里用的是 httptest 真 HTTP（不是假 client）：插件看到的是"同步读缓存、
// 后台拉一次"的行为，而这一层最容易出的错是**每帧重发一次请求**的重试风暴——
// 它不会崩，只会把宿主打满。所以断言里既有"拉到了什么"，也有"打了几次"。
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func waitUntil(t *testing.T, what string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("%s：2s 内没有发生", what)
}

func TestFileReferencesCachesAndSuppressesFailedRetry(t *testing.T) {
	var calls, failedCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		query, _ := body["query"].(string)
		switch query {
		case "ok":
			calls.Add(1)
			_, _ = w.Write([]byte(`[{"path":"plugins/dsopen/open.go","kind":"file"}]`))
		default:
			failedCalls.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"code":"internal","message":"boom"}}`))
		}
	}))
	defer srv.Close()
	m := &model{cli: apiclient.New(srv.URL)}

	// 第一帧：缓存没有、启动后台拉取；这一帧返回 nil（插件不能把它当"空目录"）。
	if got := m.fileReferences("s-1", "ok"); got != nil {
		t.Fatalf("首帧不该有同步结果，得到 %v", got)
	}
	waitUntil(t, "fileReferences 拉到结果", func() bool {
		return len(m.fileReferences("s-1", "ok")) == 1
	})
	if got := m.fileReferences("s-1", "ok"); got[0].Path != "plugins/dsopen/open.go" || got[0].IsDir {
		t.Fatalf("候选解错：%+v", got)
	}
	// 多次问同一个 query 只发一次请求。
	for i := 0; i < 5; i++ {
		m.fileReferences("s-1", "ok")
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("同一 query 发了 %d 次请求，want 1", n)
	}

	// 失败也只在"新 query"上试一次；同一个失败 query 不再重发（插件每帧都问）。
	if got := m.fileReferences("s-1", "bad"); got != nil {
		t.Fatalf("失败 query 首帧不该有结果：%v", got)
	}
	waitUntil(t, "fileReferences 失败计数", func() bool { return failedCalls.Load() >= 1 })
	for i := 0; i < 5; i++ {
		m.fileReferences("s-1", "bad")
	}
	if n := failedCalls.Load(); n != 1 {
		t.Fatalf("失败 query 被重试了 %d 次（每帧重试会打满宿主）", n)
	}
}

func TestMessageFeedbackCacheAndPut(t *testing.T) {
	var listCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/messageFeedback.list":
			listCalls.Add(1)
			_, _ = w.Write([]byte(`{"ok":true,"value":{"items":[]}}`))
		case "/api/messageFeedback.put":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			req, _ := body["request"].(map[string]any)
			id, _ := req["messageId"].(string)
			out, _ := json.Marshal(map[string]any{
				"ok": true,
				"value": map[string]any{
					"messageId": id, "rating": req["rating"], "version": "v1",
					"createdAt": 1, "updatedAt": 2,
				},
			})
			_, _ = w.Write(out)
		default:
			t.Errorf("意外路径：%s", r.URL.Path)
		}
	}))
	defer srv.Close()
	m := &model{cli: apiclient.New(srv.URL)}

	items, loaded := m.messageFeedback("s-1")
	if loaded || len(items) != 0 {
		t.Fatalf("首帧应当是未加载：loaded=%v items=%v", loaded, items)
	}
	waitUntil(t, "messageFeedback 加载", func() bool {
		_, loaded := m.messageFeedback("s-1")
		return loaded
	})
	if n := listCalls.Load(); n != 1 {
		t.Fatalf("list 调了 %d 次，want 1", n)
	}

	m.messageFeedbackPut("s-1", "v4:7", apiclient.FeedbackRatingPositive, "", apiclient.FeedbackCategoryTaskResult)
	waitUntil(t, "put 终态", func() bool {
		_, ok := m.feedbackTakeResult("s-1")
		return ok
	})
	items, _ = m.messageFeedback("s-1")
	item, ok := items["v4:7"]
	if !ok || item.Rating != apiclient.FeedbackRatingPositive || item.Version != "v1" {
		t.Fatalf("put 之后缓存没有 authoritative item：%+v", items)
	}
	// take 是取走即清空：同一句 toast 不能被重放。
	if _, ok := m.feedbackTakeResult("s-1"); ok {
		t.Fatal("take 之后又取到了同一份结果")
	}
}
