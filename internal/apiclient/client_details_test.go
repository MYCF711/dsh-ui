// client_details_test.go —— 409 的结构化附加信息（`details.reason`）。
//
// 背景（GP 报告 §5 第 3 条）：409 的两种来源共用 code `stale_epoch`，
// 界面原来只能匹配 message 文本（"滑出内存"）——文案一改就失效，而且判错的
// 两个方向都难看（当成换代 → 一直转圈；当成到头了 → 少一页）。
//
// 宿主现在把语义放在信封**顶层**的 `details.reason`（`error` 的兄弟，
// `error` 仍然恰好四字段，见 `DSH-GO/docs/transport-contract.md` §3D）。
// 客户端必须把它原样解出来，否则界面还是只能猜文本。
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serveRaw 起一台"原样回一段 JSON + 状态码"的假宿主。
func serveRaw(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestCallCarriesTopLevelDetails 钉住"结构优先"的那一半：details.reason 必须解出来。
func TestCallCarriesTopLevelDetails(t *testing.T) {
	srv := serveRaw(t, http.StatusConflict, `{
		"error":{"code":"stale_epoch","message":"sinceSeq=0 之前的历史已经滑出内存 journal",
		         "hint":"丢掉旧水位重新 load","retryable":true},
		"journalEpoch":7,"lastSeq":1024,
		"details":{"reason":"history_evicted"}
	}`)

	_, err := New(srv.URL).History(context.Background(), "s-1", 0, 10, 0)
	if err == nil {
		t.Fatal("409 必须作为错误冒泡（不能当成成功）")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("错误应当是可断言的 *APIError，实际 %T：%v", err, err)
	}
	// 四字段一个都不许变（details 是 additive）。
	if apiErr.Code != "stale_epoch" || apiErr.Message == "" || apiErr.Hint == "" || !apiErr.Retryable {
		t.Fatalf("四字段被破坏了：%+v", apiErr)
	}
	if apiErr.Details == nil {
		t.Fatal("details 没解出来：界面只能匹配 message 文本")
	}
	if reason, _ := apiErr.Details["reason"].(string); reason != "history_evicted" {
		t.Fatalf("details.reason = %v，want history_evicted（%+v）", apiErr.Details["reason"], apiErr.Details)
	}
}

// TestCallWithoutDetailsIsNil 钉住"老宿主兼容"的那一半：没有 details 是**正常**的。
//
// 不能报成"响应不是合法 JSON"——那样老宿主上所有 409 都会变成一条看不懂的解析错，
// 界面连兜底的 message 文本都拿不到。
func TestCallWithoutDetailsIsNil(t *testing.T) {
	srv := serveRaw(t, http.StatusConflict, `{
		"error":{"code":"stale_epoch","message":"journal 世代或水位对不上，需要重新 load 基线",
		         "hint":"丢弃旧水位","retryable":true}
	}`)

	_, err := New(srv.URL).History(context.Background(), "s-1", 0, 10, 0)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("老宿主（无 details）也必须解成 *APIError，实际 %T：%v", err, err)
	}
	if apiErr.Details != nil {
		t.Fatalf("没有 details 时应当是 nil（调用方据此走文本兜底），实际 %+v", apiErr.Details)
	}
	if strings.Contains(apiErr.Message, "不是合法 JSON") {
		t.Fatalf("details 缺席被误报成解析错：%v", apiErr.Message)
	}
}

// TestCallIgnoresMalformedDetails 形状不对的 details 只丢弃这一截，不许污染错误本身。
//
// 为什么钉：`details` 是**可选**字段。把它解析成硬依赖，等于让一个附加信息
// 把 401/404 也一起吞掉——客户端拿到的会是"响应不是合法 JSON"而不是真正的原因。
func TestCallIgnoresMalformedDetails(t *testing.T) {
	for _, body := range []string{
		`{"error":{"code":"busy","message":"上一轮还没结束","hint":"等 turn/done","retryable":true},"details":"oops"}`,
		`{"error":{"code":"busy","message":"上一轮还没结束","hint":"等 turn/done","retryable":true},"details":[1,2,3]}`,
	} {
		srv := serveRaw(t, http.StatusConflict, body)
		_, err := New(srv.URL).History(context.Background(), "s-1", 0, 10, 0)
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Code != "busy" {
			t.Fatalf("坏 details 把错误本身吞了：%T %v（body=%s）", err, err, body)
		}
		if apiErr.Details != nil {
			t.Fatalf("坏 details 应当丢弃，实际 %+v（body=%s）", apiErr.Details, body)
		}
	}
}

// TestDetailsNotSerialized 钉住 Details 的 json tag 是 `-`。
//
// 它承载的是信封附加信息，不是 error 四字段的一部分；序列化出去会把
// "error 恰好四字段"的契约悄悄改掉（日志、快照、跨进程转发都会跟着变）。
func TestDetailsNotSerialized(t *testing.T) {
	raw, err := json.Marshal(&APIError{
		Code: "stale_epoch", Message: "m", Hint: "h", Retryable: true,
		Details: map[string]any{"reason": "history_evicted"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	if len(obj) != 4 {
		t.Fatalf("APIError 序列化出 %d 个键，契约要求恰好四字段：%s", len(obj), raw)
	}
	if _, ok := obj["Details"]; ok {
		t.Fatalf("Details 漏进了序列化：%s", raw)
	}
}
