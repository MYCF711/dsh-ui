// jobs_output_retry_test.go —— `jobs.output` 失败**不许跟着重绘无限重发**。
//
// 缺陷形状（不报错、也没人看见，只看见宿主被越问越快）：失败分支要先
// `Invalidate`，下一帧插件又来问一次，于是「失败 → 清 err → 重发 → 失败」跟着
// 重绘一直跑。稳定失败的场景（任务 owner 已不在、子进程退出后宿主回 404）就是
// **每帧一次 HTTP**，界面上还错误/加载一闪一闪。
//
// 两条同时要钉：请求数有界；**错误照报**（不能为了止住重发把 err 吞掉）。
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func TestJobsOutputFailureDoesNotRetryEveryFrame(t *testing.T) {
	oldEvery := jobOutputRetryEvery
	jobOutputRetryEvery = time.Hour // 窗口内不许有第二次自动重试
	defer func() { jobOutputRetryEvery = oldEvery }()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/jobs.output") {
			calls.Add(1)
			http.Error(w, `{"error":{"code":"not_found","message":"任务已不在"}}`, http.StatusNotFound)
			return
		}
		_, _ = fmt.Fprint(w, `{"result":{}}`)
	}))
	defer srv.Close()

	m := &model{cli: apiclient.New(srv.URL)}

	// 第一帧：缓存空 → 后台发一次请求 → 本帧按"加载中"。
	if _, ok := m.jobOutput("s-1", "job-1", 0); ok {
		t.Fatal("首帧就返回了 ok=true（缓存里本来什么都没有）")
	}
	waitUntil(t, "第一次 jobs.output 发出去", func() bool { return calls.Load() >= 1 })

	// 之后几十帧（真实绘制每帧都会问一次）：错误必须照报，请求数不许涨。
	//
	// 每帧之间要隔一点时间：旧写法下"清 err → 重发 → 失败 → 再问"是跟着
	// 异步结果落地的，一帧内连问 50 次会被 fetching 位挡住，测不出风暴。
	const frames = 40
	seenErr := false
	for i := 0; i < frames; i++ {
		out, ok := m.jobOutput("s-1", "job-1", 0)
		if ok && out.Err != "" {
			seenErr = true
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !seenErr {
		t.Fatalf("%d 帧里一次错误都没报出来（失败被折叠成'没有输出'了）", frames)
	}
	time.Sleep(30 * time.Millisecond)
	// 上界是 2：首次尝试 + **一次**自动重试（限流窗口内不再发）。
	// 旧写法这里是 51（每帧一次），正是这条用例要挡住的形状。
	if got := calls.Load(); got > 2 {
		t.Fatalf("%d 帧重绘触发了 %d 次 jobs.output，上限 2（首次 + 一次自动重试；按时间限流，不是每帧一次）", frames, got)
	}
}

func TestJobsOutputFailureRetriesAfterWindow(t *testing.T) {
	oldEvery := jobOutputRetryEvery
	jobOutputRetryEvery = 20 * time.Millisecond
	defer func() { jobOutputRetryEvery = oldEvery }()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/jobs.output") {
			calls.Add(1)
			http.Error(w, `{"error":{"code":"internal","message":"test host"}}`, http.StatusInternalServerError)
			return
		}
		_, _ = fmt.Fprint(w, `{"result":{}}`)
	}))
	defer srv.Close()

	m := &model{cli: apiclient.New(srv.URL)}
	if _, ok := m.jobOutput("s-1", "job-1", 0); ok {
		t.Fatal("首帧就返回了 ok=true")
	}
	waitUntil(t, "第一次请求", func() bool { return calls.Load() >= 1 })

	// 窗口过后再问一次（用户点插件上那个「重试」就是这个入口）：
	// 必须真的再发一次，否则按钮只会在错误态里打转。
	time.Sleep(40 * time.Millisecond)
	for i := 0; i < 10; i++ {
		if out, ok := m.jobOutput("s-1", "job-1", 0); ok && out.Err != "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	waitUntil(t, "窗口过后重试发出第二次请求", func() bool { return calls.Load() >= 2 })
}
