// cot_translation_wiring_test.go —— #63 壳侧两条数据面的钉子（cmd/app/cot_translation_wiring.go）。
//
// 对着**真 HTTP**（httptest）验四件事，全是最容易写错又最难在真机上看出来的：
//
//	limits     首问触发一次后台拉取；拉成功置 loaded；拉失败保留上一次（不清缓存）
//	translate  只发起、不阻塞；终态按 jobID 留给插件取；**取走即清空**（同一结果不重放）
//	失败映射   四字段错误压成"码 + 一句原因"（界面只有一个失败文案位）
//	参数形状   text/targetLanguage/sessionId/provider 四个字段真的发出去了（宿主严格解平铺）
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// cotWiringHost 是一个只认 cotTranslation.* 的假宿主。
type cotWiringHost struct {
	limitsCalls    *int32
	translateCalls *int32
	lastTranslate  map[string]any
	// failLimits / failTranslate 让对应路径回四字段错误。
	failLimits    bool
	failTranslate bool
	// translated 是 translate 成功时回的**裸字符串**（官方 result 就是 z.string()）。
	translated string
}

func newCotWiringHost(t *testing.T) (*cotWiringHost, *httptest.Server) {
	t.Helper()
	var limits, translate int32
	host := &cotWiringHost{limitsCalls: &limits, translateCalls: &translate, translated: "译文"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/cotTranslation.limits":
			atomic.AddInt32(host.limitsCalls, 1)
			if host.failLimits {
				w.WriteHeader(http.StatusNotImplemented)
				_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"宿主没有 cotTranslation.limits","hint":"","retryable":false}}`))
				return
			}
			_, _ = w.Write([]byte(`{"maxTextChars":100000,` +
				`"preferences":{"provider":"deepseek-official","targetLanguage":"auto"},` +
				`"availableProviders":["deepseek-official","deepseek-account"]}`))
		case "/api/cotTranslation.translate":
			atomic.AddInt32(host.translateCalls, 1)
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			host.lastTranslate = body
			if host.failTranslate {
				w.WriteHeader(http.StatusNotImplemented)
				_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"provider=\"bing\" 是第三方匿名翻译端点","hint":"","retryable":false}}`))
				return
			}
			// 裸字符串（不是 {result:…}）：客户端 Call 直接把 body 当 result 解。
			raw, _ := json.Marshal(host.translated)
			_, _ = w.Write(raw)
		default:
			t.Errorf("意外路径：%s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return host, srv
}

// TestCotTranslationLimitsCached 钉住"首问一次、之后读缓存、失败不清缓存"。
func TestCotTranslationLimitsCached(t *testing.T) {
	host, srv := newCotWiringHost(t)
	m := &model{cli: apiclient.New(srv.URL)}

	if _, loaded := m.cotTranslationLimits(); loaded {
		t.Fatal("首帧不该是已加载")
	}
	waitUntil(t, "limits 加载", func() bool {
		_, loaded := m.cotTranslationLimits()
		return loaded
	})
	if got := atomic.LoadInt32(host.limitsCalls); got != 1 {
		t.Fatalf("limits 调了 %d 次，want 1（缓存命中后不再发）", got)
	}
	limits, _ := m.cotTranslationLimits()
	if limits.MaxTextChars != 100000 {
		t.Fatalf("maxTextChars=%d，want 100000", limits.MaxTextChars)
	}
	if limits.Preferences.Provider != apiclient.CotTranslationProviderOfficial {
		t.Fatalf("provider=%q", limits.Preferences.Provider)
	}
	if len(limits.AvailableProviders) != 2 {
		t.Fatalf("availableProviders=%v", limits.AvailableProviders)
	}

	// 拉失败：保留上一次的值（别把已经能用的清单清空）。
	host.failLimits = true
	m.mu.Lock()
	m.cotLimitsPending = false
	m.mu.Unlock()
	m.refreshCotTranslationLimits()
	if limits, loaded := m.cotTranslationLimits(); !loaded || limits.MaxTextChars != 100000 {
		t.Fatalf("拉失败后缓存被清掉了：loaded=%v limits=%+v", loaded, limits)
	}
	if m.cotLimitsErr == nil {
		t.Fatal("拉失败必须留下原因（状态栏要用）")
	}
}

// TestCotTranslationTranslateJobFlow 钉住"发起 → 取终态 → 取走即清空"。
func TestCotTranslationTranslateJobFlow(t *testing.T) {
	host, srv := newCotWiringHost(t)
	m := &model{cli: apiclient.New(srv.URL)}

	job := m.cotTranslationTranslate("sess-1", "一段思考", "zh-CN", apiclient.CotTranslationProviderOfficial)
	if job == "" {
		t.Fatal("没有拿到 jobID")
	}
	waitUntil(t, "translate 终态", func() bool {
		_, ok := m.cotTranslationTakeResult(job)
		return ok
	})
	// 已经取走了：再取不许重复返回（同一段译文不能被"重放"成第二次付费结果）。
	if _, ok := m.cotTranslationTakeResult(job); ok {
		t.Fatal("同一 jobID 的终态被取走了两次")
	}
	if got := atomic.LoadInt32(host.translateCalls); got != 1 {
		t.Fatalf("translate 调了 %d 次，want 1", got)
	}
	// 参数形状（宿主严格解平铺 + DisallowUnknownFields）。
	if host.lastTranslate["text"] != "一段思考" {
		t.Fatalf("text=%v", host.lastTranslate["text"])
	}
	if host.lastTranslate["targetLanguage"] != "zh-CN" {
		t.Fatalf("targetLanguage=%v", host.lastTranslate["targetLanguage"])
	}
	if host.lastTranslate["sessionId"] != "sess-1" {
		t.Fatalf("sessionId=%v", host.lastTranslate["sessionId"])
	}
	if host.lastTranslate["provider"] != apiclient.CotTranslationProviderOfficial {
		t.Fatalf("provider=%v", host.lastTranslate["provider"])
	}
}

// TestCotTranslationTranslateSuccessCarriesText 钉住成功时 Text 真的是那段译文。
func TestCotTranslationTranslateSuccessCarriesText(t *testing.T) {
	host, srv := newCotWiringHost(t)
	host.translated = "真实译文"
	m := &model{cli: apiclient.New(srv.URL)}

	job := m.cotTranslationTranslate("sess-1", "text", "zh-CN", "")
	var result struct {
		Text string
		OK   bool
	}
	waitUntil(t, "translate 终态", func() bool {
		got, ok := m.cotTranslationTakeResult(job)
		if ok {
			result.Text, result.OK = got.Text, got.OK
		}
		return ok
	})
	if !result.OK || result.Text != "真实译文" {
		t.Fatalf("终态=%+v，want OK + 真实译文", result)
	}
}

// TestCotTranslationTranslateFailureIsStructured 钉住失败映射（码 + 一句原因）。
func TestCotTranslationTranslateFailureIsStructured(t *testing.T) {
	host, srv := newCotWiringHost(t)
	host.failTranslate = true
	m := &model{cli: apiclient.New(srv.URL)}

	job := m.cotTranslationTranslate("sess-1", "text", "zh-CN", apiclient.CotTranslationProviderBing)
	var (
		code    string
		message string
		isOK    bool
	)
	waitUntil(t, "translate 失败终态", func() bool {
		got, ok := m.cotTranslationTakeResult(job)
		if ok {
			code, message, isOK = got.Code, got.Message, got.OK
		}
		return ok
	})
	if isOK {
		t.Fatal("失败时不许 OK")
	}
	if code != "not_implemented" {
		t.Fatalf("code=%q，want not_implemented（宿主四字段原样传下来）", code)
	}
	if message == "" {
		t.Fatal("失败必须带一句原因（界面只有一个失败文案位）")
	}
}

// TestCotTranslationTranslateRejectsEmptyInput 钉住"明显不合法的请求不外发"。
func TestCotTranslationTranslateRejectsEmptyInput(t *testing.T) {
	host, srv := newCotWiringHost(t)
	m := &model{cli: apiclient.New(srv.URL)}
	if job := m.cotTranslationTranslate("sess-1", "", "zh-CN", ""); job != "" {
		t.Fatalf("空文本也发了请求：jobID=%s", job)
	}
	// 壳没有客户端（未连接）：不许假装发出了请求。
	empty := &model{}
	if job := empty.cotTranslationTranslate("sess-1", "text", "zh-CN", ""); job != "" {
		t.Fatalf("没有 cli 也发了请求：jobID=%s", job)
	}
	if got := atomic.LoadInt32(host.translateCalls); got != 0 {
		t.Fatalf("不该有 translate 请求，实际 %d", got)
	}
}
