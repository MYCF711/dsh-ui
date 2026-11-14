// jobs_test.go —— 后台任务 `jobs.list` / `jobs.output` / `jobs.stop` 的客户端契约测试。
//
// 判据对着宿主 `internal/transport/handlers_jobs.go`（`:41-66` 的返回类型、
// `:87-160` 的 output 分页、`:258-292` 的 stop），不是猜的：
//
//	① 入参**平铺**（不套 params）：sessionId + id 必须一起给，宿主拒绝跨会话猜任务；
//	② output 的游标是 **rune** 单位：`hasMore/nextCursor/cursor` 原样透传；
//	③ `truncated` 与"没有输出"是两件事，不许折叠成空串；
//	④ stop 返回 stopped=true 才代表进程真的停了；失败要作为错误冒上来，
//	   不能吞成"已停止"。
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJobsListCarriesSessionOwnership(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/jobs.list" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"result":{"jobs":[
			{"id":"job-1","sessionId":"s-1","command":"npm test","done":false,"exit_code":0,"bytes":120},
			{"id":"job-2","sessionId":"s-2","command":"go build","done":true,"exit_code":1,"bytes":9}
		]}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).JobsList(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if len(got) != 2 {
		t.Fatalf("jobs = %d 条，应为 2", len(got))
	}
	// 归属必须在客户端这一层就留住：output/stop 都要它，界面不能重新猜。
	if got[0].SessionID != "s-1" || got[1].SessionID != "s-2" {
		t.Fatalf("sessionId 丢了：%+v", got)
	}
	if got[1].Done != true || got[1].ExitCode != 1 {
		t.Fatalf("第 2 条状态 = %+v", got[1])
	}
}

func TestJobsOutputSendsFlatScopedParamsAndKeepsCursorSemantics(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/jobs.output" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":{
			"id":"job-1","sessionId":"s-1","output":"第二页输出","done":false,"exit_code":0,
			"cwd":"D:/work","cursor":10,"nextCursor":15,"hasMore":true,"total":40,"truncated":true
		}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).JobsOutput(context.Background(), "s-1", "job-1", 10, 5)
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	// 平铺：外面再套一层 params 会让真宿主每个参数都解成零值（历史踩点）。
	if body["sessionId"] != "s-1" || body["id"] != "job-1" {
		t.Fatalf("入参不是平铺的 sessionId/id：%v", body)
	}
	if body["cursor"] != float64(10) || body["limit"] != float64(5) {
		t.Fatalf("cursor/limit 没按数字发：%v", body)
	}
	if got.Output != "第二页输出" || got.Cursor != 10 || got.NextCursor != 15 {
		t.Fatalf("分页字段解错：%+v", got)
	}
	if !got.HasMore || got.Total != 40 {
		t.Fatalf("hasMore/total 解错：%+v", got)
	}
	// 保留窗口淘汰是**独立事实**：不能因为拿到的是一页就当成"没有更早的输出"。
	if !got.Truncated {
		t.Fatal("truncated 丢了：更早的输出被淘汰这件事必须能显示出来")
	}
}

func TestJobsStopSurfacesHostRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/jobs.stop" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["sessionId"] != "s-1" || body["id"] != "job-1" {
			t.Errorf("stop 入参 = %v", body)
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":{"code":"job_finished","message":"job already finished","retryable":false}}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL).JobsStop(context.Background(), "s-1", "job-1")
	if err == nil {
		t.Fatal("宿主 409 必须作为错误返回，不能当成已停止")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "job_finished" {
		t.Fatalf("错误 = %v，应为 job_finished 四字段", err)
	}
}

func TestJobsStopDecodesConfirmedTermination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"id":"job-1","sessionId":"s-1","stopped":true,"done":true,"exit_code":-1}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).JobsStop(context.Background(), "s-1", "job-1")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if !got.Stopped || !got.Done {
		t.Fatalf("stopped/done = %+v，应都为 true", got)
	}
	if got.ExitCode != -1 {
		t.Fatalf("exitCode = %d，应为 -1（被信号结束）", got.ExitCode)
	}
}
