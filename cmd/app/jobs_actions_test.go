package main

// jobs_actions_test.go —— 壳侧后台任务动作的**真 HTTP**测试。
//
// 判据跟着宿主走（`HOST:internal/transport/handlers_jobs.go`）：
//
//   - `jobs.output` 必须带 sessionId + id（生产路径拒绝跨会话猜任务）；
//   - 分页是 rune 游标：cursor=0 第一页，下一页用上一页的 nextCursor；
//   - 读失败是错误，不是"没有输出"；
//   - `jobs.stop` 只在 stopped=true 时才认为停了（失败要如实报状态栏）。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

// waitFor 轮询一个条件（后台拉取是异步的；测试用短超时，失败即红）。
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等不到 %s", what)
}

func TestJobOutputLoadsTwoPagesThroughCache(t *testing.T) {
	var mu sync.Mutex
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/jobs.output":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			bodies = append(bodies, body)
			mu.Unlock()
			cursor, _ := body["cursor"].(float64)
			if cursor == 0 {
				_, _ = w.Write([]byte(`{"result":{"id":"job-1","sessionId":"s-1","output":"abc",
					"cursor":0,"nextCursor":3,"hasMore":true,"total":6}}`))
				return
			}
			// 宿主每页只回**这一页**的文本（`PageJobOutput`），累计由壳拼。
			_, _ = w.Write([]byte(`{"result":{"id":"job-1","sessionId":"s-1","output":"def",
				"cursor":3,"nextCursor":6,"hasMore":false,"total":6}}`))
		case "/api/jobs.list":
			_, _ = w.Write([]byte(`{"result":{"jobs":[]}}`))
		default:
			t.Errorf("意外路径 %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	m := &model{cli: apiclient.New(srv.URL)}

	// 第一帧：缓存没有 → ok=false，壳在后台拉。
	if _, ok := m.jobOutput("s-1", "job-1", 0); ok {
		t.Fatal("首帧不该直接给出数据（缓存是空的）")
	}
	waitFor(t, "第一页到位", func() bool {
		view, ok := m.jobOutput("s-1", "job-1", 0)
		return ok && view.Output == "abc"
	})
	view, _ := m.jobOutput("s-1", "job-1", 0)
	if !view.HasMore || view.NextCursor != 3 {
		t.Fatalf("第一页分页字段 = %+v", view)
	}

	// 要下一页：用 nextCursor=3。
	if _, ok := m.jobOutput("s-1", "job-1", 3); ok {
		t.Fatal("第二页还没拉到，不该立刻 ok")
	}
	waitFor(t, "第二页到位", func() bool {
		got, ok := m.jobOutput("s-1", "job-1", 3)
		return ok && got.Output == "abcdef" && !got.HasMore
	})

	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 2 {
		t.Fatalf("应只发两次 output 请求，实际 %d：%v", len(bodies), bodies)
	}
	for _, body := range bodies {
		if body["sessionId"] != "s-1" || body["id"] != "job-1" {
			t.Fatalf("入参缺归属：%v", body)
		}
	}
	if bodies[1]["cursor"] != float64(3) {
		t.Fatalf("第二页 cursor = %v，应为 3", bodies[1]["cursor"])
	}
}

func TestJobOutputMissingOwnershipRefusesWithoutRequest(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	defer srv.Close()

	m := &model{cli: apiclient.New(srv.URL)}
	view, ok := m.jobOutput("", "job-1", 0)
	if !ok || view.Err == "" {
		t.Fatalf("缺会话归属应直接报错（ok=%v err=%q），而不是猜一个会话", ok, view.Err)
	}
	if hits != 0 {
		t.Fatalf("缺归属时不该发请求，实际发了 %d 次", hits)
	}
}

func TestJobOutputErrorSurfacesAndRetries(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/jobs.output" {
			return
		}
		mu.Lock()
		calls++
		first := calls == 1
		mu.Unlock()
		if first {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"no such job","retryable":false}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":{"id":"job-1","sessionId":"s-1","output":"ok","nextCursor":2,"total":2}}`))
	}))
	defer srv.Close()

	m := &model{cli: apiclient.New(srv.URL)}
	if _, ok := m.jobOutput("s-1", "job-1", 0); ok {
		t.Fatal("首帧不该 ok")
	}
	// 失败要作为 Err 交出去（不是空输出），同时自动重排一次拉取。
	waitFor(t, "错误上报", func() bool {
		view, ok := m.jobOutput("s-1", "job-1", 0)
		return ok && view.Err != ""
	})
	waitFor(t, "重试成功", func() bool {
		view, ok := m.jobOutput("s-1", "job-1", 0)
		return ok && view.Err == "" && view.Output == "ok"
	})
}

func TestJobStopSendsScopedRequest(t *testing.T) {
	var mu sync.Mutex
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/jobs.stop":
			mu.Lock()
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Unlock()
			_, _ = w.Write([]byte(`{"result":{"id":"job-1","sessionId":"s-1","stopped":true,"done":true,"exit_code":-1}}`))
		case "/api/jobs.list":
			_, _ = w.Write([]byte(`{"result":{"jobs":[]}}`))
		default:
			t.Errorf("意外路径 %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	m := &model{cli: apiclient.New(srv.URL)}
	m.jobStop("s-1", "job-1")
	waitFor(t, "停止请求到达宿主", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return body != nil
	})
	mu.Lock()
	defer mu.Unlock()
	if body["sessionId"] != "s-1" || body["id"] != "job-1" {
		t.Fatalf("stop 入参 = %v，应同时带 sessionId 与 id", body)
	}
}

func TestJobStopWithoutOwnershipDoesNotPanicOrRequest(t *testing.T) {
	m := &model{}
	m.jobStop("", "job-1") // 只该写状态栏
	if m.status == "" {
		t.Fatal("缺会话归属时应说明原因")
	}
}

func TestPruneJobOutputsDropsStaleKeys(t *testing.T) {
	m := &model{
		jobOutputs: map[string]*jobOutputCache{
			jobOutputKey("s-1", "job-1"): {},
			jobOutputKey("s-1", "job-2"): {},
		},
	}
	m.pruneJobOutputs([]pluginapi.Job{{SessionID: "s-1", ID: "job-1"}})
	if _, ok := m.jobOutputs[jobOutputKey("s-1", "job-1")]; !ok {
		t.Fatal("还在清单里的任务被清掉了")
	}
	if _, ok := m.jobOutputs[jobOutputKey("s-1", "job-2")]; ok {
		t.Fatal("已离开清单的任务缓存没清掉")
	}
}
