package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// 这组测试钉住"入参只有一种真相，却有两种形状进来"：
//
//	平铺   {"id":"s-002","tailMessages":3}      ← 契约就是它（真宿主 handler 直接 decode 到平铺结构）
//	包裹   {"params":{"id":"s-002",…}}          ← mockd 早期形状，只为历史脚本继续收
//
// 两边不一致的后果不是报错，而是**参数静默变零值**：session.list 照常返回
// （它不要参数）、session.load 报 not_found、session.prompt 拿到空 id/text。
// 所以这里断言的是"两种写法解出来的参数完全相等"，不是"都没报错"。

// decodeProto 走一遍真实解包路径（decodeParams + json.Unmarshal），
// 用 proto 的类型决定解到哪个入参结构。
func decodeProto(t *testing.T, label, body string, proto any) any {
	t.Helper()
	raw, ok := decodeParams([]byte(body))
	if !ok {
		t.Fatalf("%s：decodeParams 判成非法 body：%s", label, body)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		// 与 handler 一致：没参数就是零值（json.Unmarshal(nil) 的错被 handler 忽略）
		return reflect.Zero(reflect.TypeOf(proto)).Interface()
	}
	v := reflect.New(reflect.TypeOf(proto))
	if err := json.Unmarshal(raw, v.Interface()); err != nil {
		t.Fatalf("%s：解参数失败：%v（body=%s）", label, err, body)
	}
	return v.Elem().Interface()
}

func TestDecodeParamsFlatAndWrapped(t *testing.T) {
	cases := []struct {
		method  string
		proto   any
		flat    string
		wrapped string
		want    any
	}{
		{
			// 契约快照里的名字
			method:  "session.load",
			proto:   loadParams{},
			flat:    `{"id":"s-002","tailMessages":3}`,
			wrapped: `{"params":{"id":"s-002","tailMessages":3}}`,
			want:    loadParams{ID: "s-002", TailMessages: 3},
		},
		{
			// 宿主 handler 里的名字（sessionId/limit）：别的脚本可能只发这一套
			method:  "session.load",
			proto:   loadParams{},
			flat:    `{"sessionId":"s-003","limit":7}`,
			wrapped: `{"params":{"sessionId":"s-003","limit":7}}`,
			want:    loadParams{SessionID: "s-003", Limit: 7},
		},
		{
			method:  "session.history",
			proto:   historyParams{},
			flat:    `{"id":"s-002","sinceSeq":10,"limit":5}`,
			wrapped: `{"params":{"id":"s-002","sinceSeq":10,"limit":5}}`,
			want:    historyParams{ID: "s-002", SinceSeq: 10, Limit: 5},
		},
		{
			method:  "session.prompt",
			proto:   promptParams{},
			flat:    `{"id":"s-002","text":"流式自测"}`,
			wrapped: `{"params":{"id":"s-002","text":"流式自测"}}`,
			want:    promptParams{ID: "s-002", Text: "流式自测"},
		},
		{
			method:  "session.cancel",
			proto:   cancelParams{},
			flat:    `{"sessionId":"s-001"}`,
			wrapped: `{"params":{"sessionId":"s-001"}}`,
			want:    cancelParams{SessionID: "s-001"},
		},
		{
			method:  "session.create",
			proto:   createParams{},
			flat:    `{"mode":"plan"}`,
			wrapped: `{"params":{"mode":"plan"}}`,
			want:    createParams{Mode: "plan"},
		},
	}

	for _, c := range cases {
		flat := decodeProto(t, c.method+" 平铺", c.flat, c.proto)
		wrapped := decodeProto(t, c.method+" 包裹", c.wrapped, c.proto)
		if !reflect.DeepEqual(flat, c.want) {
			t.Fatalf("%s 平铺解错：got %+v, want %+v", c.method, flat, c.want)
		}
		if !reflect.DeepEqual(wrapped, c.want) {
			t.Fatalf("%s 包裹解错：got %+v, want %+v", c.method, wrapped, c.want)
		}
		if !reflect.DeepEqual(flat, wrapped) {
			t.Fatalf("%s 两种形状解出的参数不一致：flat=%+v wrapped=%+v", c.method, flat, wrapped)
		}
	}
}

// TestLoadParamsAliasPrecedence 钉住别名冲突时的取法，逐字对齐宿主：
// sessionId 优先于 id（server.go:395 fillID）、limit 优先于 tailMessages（server.go:417 firstNonZero）。
func TestLoadParamsAliasPrecedence(t *testing.T) {
	body := `{"id":"a","sessionId":"b","tailMessages":1,"limit":2}`
	for _, shape := range []struct {
		label string
		body  string
	}{
		{"平铺", body},
		{"包裹", `{"params":` + body + `}`},
	} {
		got := decodeProto(t, shape.label, shape.body, loadParams{}).(loadParams)
		if got.sessionID() != "b" {
			t.Fatalf("%s：sessionId 应优先于 id，got %q", shape.label, got.sessionID())
		}
		if got.tail() != 2 {
			t.Fatalf("%s：limit 应优先于 tailMessages，got %d", shape.label, got.tail())
		}
	}
}

// TestDecodeParamsMissingBothIsZeroAndNoPanic 是"两者都缺"那一条：
// 没有平铺参数、也没有 params 包裹 → 参数全零值，且不 panic。
func TestDecodeParamsMissingBothIsZeroAndNoPanic(t *testing.T) {
	bodies := []struct{ label, body string }{
		{"空 body", ""},
		{"只有空白", " \n\t "},
		{"空对象", `{}`},
		{"空包裹", `{"params":{}}`},
		{"params 不是对象", `{"params":3}`},
	}
	protos := []struct {
		name  string
		proto any
	}{
		{"session.load", loadParams{}},
		{"session.history", historyParams{}},
		{"session.prompt", promptParams{}},
		{"session.cancel", cancelParams{}},
		{"session.create", createParams{}},
	}

	for _, b := range bodies {
		for _, p := range protos {
			got := decodeProto(t, p.name+" / "+b.label, b.body, p.proto)
			zero := reflect.Zero(reflect.TypeOf(p.proto)).Interface()
			if !reflect.DeepEqual(got, zero) {
				t.Fatalf("%s / %s：应是零值，got %+v", p.name, b.label, got)
			}
		}
	}
}

// TestDecodeParamsRejectsNonObjectBody 保证"发错形状"仍有 400，不会静默零值。
func TestDecodeParamsRejectsNonObjectBody(t *testing.T) {
	for _, body := range []string{"[]", `"x"`, "123", "null", "not-json", "{oops"} {
		if _, ok := decodeParams([]byte(body)); ok {
			t.Fatalf("body %q 应判成非法（要么不是 JSON，要么不是对象）", body)
		}
	}
}

// TestRouteFlatAndWrappedSameResult 是端到端那一层：同一个 method，两种 body，
// route() 必须回同一份响应（status + body 逐字相同）。
func TestRouteFlatAndWrappedSameResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(route))
	defer srv.Close()

	post := func(body string) (int, string) {
		resp, err := http.Post(srv.URL+"/api/session.load", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST %q: %v", body, err)
		}
		defer resp.Body.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(resp.Body)
		return resp.StatusCode, strings.TrimSpace(buf.String())
	}

	flatStatus, flatBody := post(`{"id":"s-002","tailMessages":3}`)
	wrapStatus, wrapBody := post(`{"params":{"id":"s-002","tailMessages":3}}`)
	if flatStatus != http.StatusOK || wrapStatus != http.StatusOK {
		t.Fatalf("两种形状都要 200，拿到 flat=%d wrapped=%d", flatStatus, wrapStatus)
	}
	if flatBody != wrapBody {
		t.Fatalf("两种入参形状的响应不一致：\nflat  =%.200s\nwrapped=%.200s", flatBody, wrapBody)
	}
	if !strings.Contains(flatBody, `"messages"`) {
		t.Fatalf("响应里没有 messages：%.200s", flatBody)
	}
	t.Logf("两种形状同一份响应（%d 字节）：%.120s", len(flatBody), flatBody)

	// 两者都缺：空 id → 会话不存在（404），不是 400 也不是 panic
	if st, _ := post(`{}`); st != http.StatusNotFound {
		t.Fatalf("空 body 应走 404（空 id），拿到 %d", st)
	}
	if st, _ := post(``); st != http.StatusNotFound {
		t.Fatalf("无 body 应走 404（空 id），拿到 %d", st)
	}
	// 形状发错：400 invalid_arg
	if st, _ := post(`[]`); st != http.StatusBadRequest {
		t.Fatalf("数组 body 应 400，拿到 %d", st)
	}
}
