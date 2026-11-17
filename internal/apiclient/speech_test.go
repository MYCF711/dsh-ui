// speech_test.go —— 官方 `speech.*` 六方法的客户端契约测试。
//
// 判据对着**宿主 wire 形状**（`HOST:internal/transport/handlers_speech_official.go:124-246`），
// 不是对着 JS 端看起来的样子：
//
//	① `catalog` 直接返回**数组**（不套 result 信封）；
//	② `transcribe` 从 `request` 子对象取参数，`audioBase64` 是裸 base64；
//	③ `prepare` / `cancelPreparation` 只要 `providerId`，`downloadSource` 可选地嵌在 `options`；
//	④ `configure` 从 `patch` 子对象取；
//	⑤ `follow` 是 NDJSON，一帧一行。
//
// 另外两条是**不许静默**的：空音频不发请求；识别出的时长原样留住（宿主算过一次，
// 界面再丢掉就等于白算）。
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSpeechCatalogReadsAFlatArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/speech.catalog" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		// 宿主的真形状：顶层就是数组，没有 result 信封。
		_, _ = w.Write([]byte(`[{"id":"MS-2052-80-DESK","name":"MS-2052-80-DESK","language":"zh-CN"}]`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).SpeechCatalog(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if len(got) != 1 || got[0].ID != "MS-2052-80-DESK" || got[0].Language != "zh-CN" {
		t.Fatalf("catalog = %+v", got)
	}
}

func TestSpeechCatalogEmptyArrayIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	got, err := New(srv.URL).SpeechCatalog(context.Background())
	if err != nil {
		t.Fatalf("空数组不该报错：%v", err)
	}
	if len(got) != 0 {
		t.Fatalf("catalog = %+v，应为空", got)
	}
}

func TestSpeechTranscribeSendsRequestSubObject(t *testing.T) {
	var body map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/speech.transcribe" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("解请求体：%v", err)
		}
		_, _ = w.Write([]byte(`{"text":"你好","audioSeconds":1.25,"inferenceSeconds":0.4}`))
	}))
	defer srv.Close()

	tr, err := New(srv.URL).SpeechTranscribe(context.Background(), "MS-2052-80-DESK", "zh-CN", "UklGRg==")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	// 宿主从 `request` 子对象取；顶层的键会被忽略（它在 ServeOfficialSpeechAPI 里
	// 先试 request，再退回顶层 p）。发错层就等于什么都没传。
	if _, ok := body["request"]; !ok {
		t.Fatalf("请求体缺 request 子对象：%v", keysOf(body))
	}
	var req map[string]string
	if err := json.Unmarshal(body["request"], &req); err != nil {
		t.Fatalf("request 不是对象：%v", err)
	}
	if req["providerId"] != "MS-2052-80-DESK" || req["language"] != "zh-CN" {
		t.Fatalf("request = %+v", req)
	}
	// 裸 base64：带 data: 前缀会被宿主 base64 解码失败当成 invalid_audio。
	if req["audioBase64"] != "UklGRg==" {
		t.Fatalf("audioBase64 = %q", req["audioBase64"])
	}
	if tr.Text != "你好" {
		t.Fatalf("text = %q", tr.Text)
	}
	// 时长由宿主算过，界面丢掉就等于白算。
	if tr.AudioSeconds != 1.25 || tr.InferenceSeconds != 0.4 {
		t.Fatalf("时长丢了：%+v", tr)
	}
}

func TestSpeechTranscribeRefusesEmptyAudioWithoutARequest(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()
	if _, err := New(srv.URL).SpeechTranscribe(context.Background(), "p", "", ""); err == nil {
		t.Fatalf("空音频必须报错")
	}
	if called {
		t.Fatalf("空音频不该发请求")
	}
}

func TestSpeechPrepareAndCancelSendProviderID(t *testing.T) {
	var got []map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&body)
		body["__path"] = json.RawMessage(`"` + r.URL.Path + `"`)
		got = append(got, body)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.SpeechPrepare(context.Background(), "MS-2052-80-DESK", ""); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := c.SpeechCancelPreparation(context.Background(), "MS-2052-80-DESK"); err != nil {
		t.Fatalf("cancelPreparation: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("请求 %d 次，应为 2", len(got))
	}
	for i, want := range []string{"/api/speech.prepare", "/api/speech.cancelPreparation"} {
		var path string
		_ = json.Unmarshal(got[i]["__path"], &path)
		if path != want {
			t.Fatalf("第 %d 条路径 = %q，期望 %q", i, path, want)
		}
		var id string
		_ = json.Unmarshal(got[i]["providerId"], &id)
		if id != "MS-2052-80-DESK" {
			t.Fatalf("第 %d 条 providerId = %q", i, id)
		}
	}
	// downloadSource 为空时不该出现 options：宿主只在非空时读它。
	if _, ok := got[0]["options"]; ok {
		t.Fatalf("空 downloadSource 不该带 options：%v", keysOf(got[0]))
	}
	// 空 providerId 必须本地拦住（宿主会回 400，但界面不该发这种请求）。
	if err := c.SpeechPrepare(context.Background(), "  ", ""); err == nil {
		t.Fatalf("空 providerId 必须报错")
	}
}

func TestSpeechConfigureSendsPatch(t *testing.T) {
	var body map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/speech.configure" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"providerId":"MS-2052-80-DESK","language":"zh-CN"}`))
	}))
	defer srv.Close()

	cfg, err := New(srv.URL).SpeechConfigure(context.Background(), SpeechConfig{ProviderID: "MS-2052-80-DESK", Language: "zh-CN"})
	if err != nil {
		t.Fatalf("configure: %v", err)
	}
	if _, ok := body["patch"]; !ok {
		t.Fatalf("请求体缺 patch 子对象：%v", keysOf(body))
	}
	if cfg.ProviderID != "MS-2052-80-DESK" || cfg.Language != "zh-CN" {
		t.Fatalf("回参 = %+v", cfg)
	}
}

func TestSpeechFollowReadsNDJSONFrames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/speech.follow" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"type":"status","providerId":"p","state":"idle","prepared":false}`+"\n")
		_, _ = io.WriteString(w, `{"type":"status","providerId":"p","state":"prepared","prepared":true}`+"\n")
		_, _ = io.WriteString(w, "\n") // 空行要跳过，不是帧
	}))
	defer srv.Close()

	var states []string
	err := New(srv.URL).SpeechFollow(context.Background(), func(st SpeechStatus) error {
		states = append(states, st.State)
		return nil
	})
	if err != nil {
		t.Fatalf("follow: %v", err)
	}
	if strings.Join(states, ",") != "idle,prepared" {
		t.Fatalf("帧 = %v", states)
	}
}

func TestSpeechFollowStopsWhenTheCallbackFails(t *testing.T) {
	stop := errors.New("stop here")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, `{"type":"status","providerId":"p","state":"idle"}`+"\n")
		_, _ = io.WriteString(w, `{"type":"status","providerId":"p","state":"error","error":"boom"}`+"\n")
	}))
	defer srv.Close()

	seen := 0
	err := New(srv.URL).SpeechFollow(context.Background(), func(SpeechStatus) error {
		seen++
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("err = %v，应当把回调错误原样冒上来", err)
	}
	if seen != 1 {
		t.Fatalf("回调 %d 次，应当在第 1 次就停", seen)
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
