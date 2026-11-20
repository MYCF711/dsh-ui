// uistate_test.go —— `uiState.get/set/delete` 的 wire 形状。
//
// 断言的都是**宿主真实读法**（`../DSH-GO/internal/transport/handlers_ui_state.go`）：
// 裸参数对象、键名逐字、value 原样 JSON、没存过是 found:false 而不是错误。
package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUIStateGetSendsBareKeyAndReadsRawValue(t *testing.T) {
	var seen map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/uiState.get" {
			t.Errorf("路径应为 /api/uiState.get，得到 %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("请求体不是 JSON：%v", err)
		}
		seen = body
		_, _ = w.Write([]byte(`{"result":{"found":true,"value":{"byTab":{"browser":{"url":"https://example.com"}}}}}`))
	}))
	defer srv.Close()

	value, found, err := New(srv.URL).UIStateGet(context.Background(), "dsh.sidebar-browser.v1.session-5f0d")
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if len(seen) != 1 || seen["key"] != "dsh.sidebar-browser.v1.session-5f0d" {
		t.Fatalf("裸参数不对（多发了键？）：%v", seen)
	}
	if _, ok := seen["params"]; ok {
		t.Fatalf("不该套 params：%v", seen)
	}
	var decoded map[string]any
	if err := json.Unmarshal(value, &decoded); err != nil {
		t.Fatalf("value 不是原样 JSON：%v（%s）", err, value)
	}
	if _, ok := decoded["byTab"]; !ok {
		t.Fatalf("value 丢了 byTab：%s", value)
	}
}

func TestUIStateGetMissingIsFoundFalseNotError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"found":false}}`))
	}))
	defer srv.Close()

	value, found, err := New(srv.URL).UIStateGet(context.Background(), "dsh.user-questions.drafts.v1")
	if err != nil {
		t.Fatalf("没存过不是错误：%v", err)
	}
	if found || value != nil {
		t.Fatalf("found=%v value=%s", found, value)
	}
}

func TestUIStateSetSendsRawValueAndRejectsEmpty(t *testing.T) {
	var seen map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/uiState.set" {
			t.Errorf("路径应为 /api/uiState.set，得到 %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("请求体不是 JSON：%v", err)
		}
		seen = body
		_, _ = w.Write([]byte(`{"result":{"key":"dsh.x.v1","bytes":9}}`))
	}))
	defer srv.Close()

	client := New(srv.URL)
	key := "dsh.sidebar-browser.v1.session-5f0d"
	if err := client.UIStateSet(context.Background(), key, json.RawMessage(`{"byTab":{}}`)); err != nil {
		t.Fatalf("set: %v", err)
	}
	if seen["key"] != key {
		t.Fatalf("key 不对：%v", seen)
	}
	if _, ok := seen["value"].(map[string]any); !ok {
		t.Fatalf("value 必须作为**JSON 值**发出去，不是字符串：%v", seen["value"])
	}
	// 空 value 在客户端就挡掉：宿主会 400（清状态用 delete），不必发这一趟。
	if err := client.UIStateSet(context.Background(), key, nil); err == nil {
		t.Fatal("空 value 应在本地被拒")
	}
	if err := client.UIStateSet(context.Background(), "", json.RawMessage(`{}`)); err == nil {
		t.Fatal("空 key 应在本地被拒")
	}
}

func TestUIStateDeleteReportsWhetherItExisted(t *testing.T) {
	bodies := []string{`{"result":{"deleted":true}}`, `{"result":{"deleted":false}}`}
	index := 0
	var seen map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/uiState.delete" {
			t.Errorf("路径应为 /api/uiState.delete，得到 %q", got)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		seen = body
		_, _ = w.Write([]byte(bodies[index]))
		index++
	}))
	defer srv.Close()

	client := New(srv.URL)
	deleted, err := client.UIStateDelete(context.Background(), "dsh.x.v1")
	if err != nil || !deleted {
		t.Fatalf("deleted=%v err=%v", deleted, err)
	}
	if len(seen) != 1 || seen["key"] != "dsh.x.v1" {
		t.Fatalf("delete 只该发 key：%v", seen)
	}
	deleted, err = client.UIStateDelete(context.Background(), "dsh.x.v1")
	if err != nil || deleted {
		t.Fatalf("本来就没有应 deleted=false：%v %v", deleted, err)
	}
}

func TestUIStateUnwiredIs501NotImplemented(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"uiState.get 未接线","hint":"宿主这一侧还没接线，换一个 method 或等它实现","retryable":false}}`))
	}))
	defer srv.Close()

	_, _, err := New(srv.URL).UIStateGet(context.Background(), "dsh.x.v1")
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("四字段错误应原样上抛，得到 %T %v", err, err)
	}
	if apiErr.Code != "not_implemented" || apiErr.Retryable {
		t.Fatalf("code=%q retryable=%v", apiErr.Code, apiErr.Retryable)
	}
}
