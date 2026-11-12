// model_providers_test.go —— 官方 rc.2 `llm.listConfigurableProviders` 的客户端契约测试。
//
// 四条判据（都对着宿主 handler 的源码，不是猜的）：
//
//	① 参数为空（官方 `parameters: []`）：宿主对任何字段都回 400；
//	② 路径是 `/api/llm.listConfigurableProviders`；
//	③ 结果解码官方九个字段，缺字段用零值（omitempty 的字段缺失 == false/空）；
//	④ 未接线是 501 + 四字段错误，**不是空数组**——界面必须能区分"没有"与"查不了"。
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListConfigurableProvidersSendsNoParams(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":[]}`))
	}))
	defer srv.Close()

	list, err := New(srv.URL).ListConfigurableProviders(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotPath != "/api/llm.listConfigurableProviders" {
		t.Fatalf("路径应为 /api/llm.listConfigurableProviders，得到 %q", gotPath)
	}
	if len(body) != 0 {
		t.Fatalf("官方 parameters: []，宿主对任何参数都回 400；实际发了 %v", body)
	}
	if len(list) != 0 {
		t.Fatalf("空目录应解成空切片，得到 %d 项", len(list))
	}
}

func TestListConfigurableProvidersDecodesOfficialFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":[
			{"provider":"deepseek","displayName":"DeepSeek","settingsNs":"llm",
			 "settingsPath":["providers","deepseek"],"declared":true,
			 "defaultBaseURL":"https://api.deepseek.com","baseURLEnv":["DSHD_LLM_BASE"],
			 "nativeAuth":{"env":["DEEPSEEK_API_KEY"],"oauth":false}},
			{"provider":"openai","displayName":"OpenAI-compatible","settingsNs":"llm",
			 "settingsPath":["providers","openai"],"error":"没有可用端点"}
		]}`))
	}))
	defer srv.Close()

	list, err := New(srv.URL).ListConfigurableProviders(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if len(list) != 2 {
		t.Fatalf("应解出 2 个 provider，得到 %d", len(list))
	}
	first := list[0]
	if first.Provider != "deepseek" || first.DisplayName != "DeepSeek" || first.SettingsNamespace != "llm" {
		t.Fatalf("前三个字段解错了：%+v", first)
	}
	if !first.Declared || first.DefaultBaseURL != "https://api.deepseek.com" {
		t.Fatalf("declared/defaultBaseURL 解错了：%+v", first)
	}
	if len(first.SettingsPath) != 2 || len(first.BaseURLEnv) != 1 {
		t.Fatalf("settingsPath/baseURLEnv 解错了：%+v", first)
	}
	if first.NativeAuth == nil || len(first.NativeAuth.Env) != 1 || first.NativeAuth.OAuth {
		t.Fatalf("nativeAuth 解错了：%+v", first.NativeAuth)
	}
	second := list[1]
	if second.Declared {
		t.Fatalf("缺 declared 字段应解成 false（官方 omitempty），得到 %+v", second)
	}
	if second.Error != "没有可用端点" {
		t.Fatalf("error 字段丢了：%+v", second)
	}
}

func TestListConfigurableProvidersNotImplementedIsNotAnEmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"llm.listConfigurableProviders 未接线","hint":"宿主没有 AttachOfficialLLMProviderAPI","retryable":false}}`))
	}))
	defer srv.Close()

	list, err := New(srv.URL).ListConfigurableProviders(context.Background())
	if err == nil {
		t.Fatalf("501 必须报错，不能当成空目录（会静默隐藏菜单）：list=%v", list)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("应该是 *APIError，得到 %T: %v", err, err)
	}
	if apiErr.Code != "not_implemented" {
		t.Fatalf("code 应为 not_implemented，得到 %q", apiErr.Code)
	}
	if list != nil {
		t.Fatalf("失败时不应返回半条目录：%v", list)
	}
}

func TestListConfigurableProvidersRawArrayWithoutEnvelope(t *testing.T) {
	// 官方 handler 走 writeJSON → 会套 {"result": …}；但真宿主历史上也直接回过
	// 裸数组（Call 的兜底分支）。两种形状都必须能解，否则界面会"看不到 provider"。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"provider":"deepseek","displayName":"DeepSeek"}]`))
	}))
	defer srv.Close()

	list, err := New(srv.URL).ListConfigurableProviders(context.Background())
	if err != nil {
		t.Fatalf("裸数组也应能解：%v", err)
	}
	if len(list) != 1 || list[0].Provider != "deepseek" {
		t.Fatalf("裸数组解码结果不对：%+v", list)
	}
}
