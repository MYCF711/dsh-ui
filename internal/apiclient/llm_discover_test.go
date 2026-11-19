// llm_discover_test.go —— `llm.discoverModels` 的 wire 契约测试。
//
// 断言的是**宿主真实读法**（`../DSH-GO/internal/transport/handlers_account_official.go`
// 的 officialLLMDiscoverModels）：裸参数 `{settingsNs, request?}`，request 四字段全可选、
// 缺就不发；结果每项 id 必填，name/contextWindow/maxTokens/inputModalities 缺 = 未知
// （宿主用 `omitempty` 发，不会发 0 也不是发空数组）。
package apiclient

import (
	"context"
	"testing"
)

func TestLLMDiscoverModelsSendsSettingsNSAndEndpoint(t *testing.T) {
	var body map[string]any
	srv := catalogServer(t, "/api/llm.discoverModels", `{"result":[
		{"id":"m-1","name":"One","contextWindow":200000,"maxTokens":8192,"inputModalities":["text","image"]},
		{"id":"m-2"}
	]}`, &body)
	defer srv.Close()

	models, err := New(srv.URL).LLMDiscoverModels(context.Background(), "llm", ModelDiscoveryRequest{
		BaseURL: "https://api.example.com/v1", API: "openai-completions", APIKey: "one-shot",
	})
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if body["settingsNs"] != "llm" {
		t.Fatalf("settingsNs 必须原样发出：%v", body)
	}
	request, ok := body["request"].(map[string]any)
	if !ok {
		t.Fatalf("有字段时必须发 request 对象：%v", body)
	}
	if request["baseURL"] != "https://api.example.com/v1" || request["api"] != "openai-completions" ||
		request["apiKey"] != "one-shot" {
		t.Fatalf("request 四字段没发对：%v", request)
	}
	if _, present := request["provider"]; present {
		t.Fatalf("没给的 provider 必须整键省略：%v", request)
	}
	if len(models) != 2 || models[0].ID != "m-1" {
		t.Fatalf("结果没解出来：%+v", models)
	}
	// 报了的值是真值。
	if models[0].ContextWindow == nil || *models[0].ContextWindow != 200000 {
		t.Fatalf("contextWindow 应解成 200000：%+v", models[0].ContextWindow)
	}
	if len(models[0].InputModalities) != 2 {
		t.Fatalf("inputModalities 应两项：%v", models[0].InputModalities)
	}
	// 没报的字段保持 nil（未知 ≠ 0）。
	if models[1].Name != nil || models[1].ContextWindow != nil || models[1].MaxTokens != nil {
		t.Fatalf("宿主没发的字段必须留 nil：%+v", models[1])
	}
}

func TestLLMDiscoverModelsOmitsEmptyRequest(t *testing.T) {
	var body map[string]any
	srv := catalogServer(t, "/api/llm.discoverModels", `{"result":[]}`, &body)
	defer srv.Close()

	models, err := New(srv.URL).LLMDiscoverModels(context.Background(), "llm", ModelDiscoveryRequest{})
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	// 没有任何可问的字段时官方 zod 的 request 是可选的：不发明一个空对象。
	if _, present := body["request"]; present {
		t.Fatalf("没有字段时不该发 request：%v", body)
	}
	if models == nil || len(models) != 0 {
		t.Fatalf("空结果是确定的真值 []，不是 nil：%v", models)
	}
}
