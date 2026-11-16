// model_catalog_test.go —— `session.modelCatalog` 的真实 wire 契约测试。
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionModelCatalogSendsNoParamsAndDecodesOfficialFields(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":{
			"default":{"provider":"deepseek","model":"deepseek-reasoner","reasoningEffort":"high"},
			"routableProviders":["deepseek","openai"],
			"groups":[{
				"id":"deepseek","name":"DeepSeek","models":[{
					"id":"deepseek-reasoner","name":"DeepSeek Reasoner","description":"推理模型",
					"reasoning":{"efforts":[
						{"id":"low","name":"低","description":"快速"},
						{"id":"high","name":"高","description":"深入"}
					],"defaultEffort":"high"}
				},{
					"id":"deepseek-chat","name":"DeepSeek Chat"
				}]
			}],
			"failures":[{"id":"broken","name":"Broken","message":"目录读取失败"}]
		}}`))
	}))
	defer srv.Close()

	catalog, err := New(srv.URL).SessionModelCatalog(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotPath != "/api/session.modelCatalog" {
		t.Fatalf("路径应为 /api/session.modelCatalog，得到 %q", gotPath)
	}
	if len(body) != 0 {
		t.Fatalf("官方 parameters: []，实际发了 %v", body)
	}
	if catalog.Default.Provider != "deepseek" || catalog.Default.Model != "deepseek-reasoner" || catalog.Default.ReasoningEffort != "high" {
		t.Fatalf("default 解码不对：%+v", catalog.Default)
	}
	if len(catalog.RoutableProviders) != 2 || catalog.RoutableProviders[1] != "openai" {
		t.Fatalf("routableProviders 解码不对：%v", catalog.RoutableProviders)
	}
	if len(catalog.Groups) != 1 || catalog.Groups[0].ID != "deepseek" || catalog.Groups[0].Name != "DeepSeek" {
		t.Fatalf("groups 解码不对：%+v", catalog.Groups)
	}
	models := catalog.Groups[0].Models
	if len(models) != 2 || models[0].ID != "deepseek-reasoner" || models[0].Name != "DeepSeek Reasoner" || models[0].Description != "推理模型" {
		t.Fatalf("models 解码不对：%+v", models)
	}
	if models[0].Reasoning == nil || models[0].Reasoning.DefaultEffort != "high" {
		t.Fatalf("reasoning/defaultEffort 解码不对：%+v", models[0].Reasoning)
	}
	efforts := models[0].Reasoning.Efforts
	if len(efforts) != 2 || efforts[0].ID != "low" || efforts[0].Name != "低" || efforts[0].Description != "快速" || efforts[1].ID != "high" {
		t.Fatalf("efforts 解码不对：%+v", efforts)
	}
	if models[1].Reasoning != nil {
		t.Fatalf("缺 reasoning 必须保持 nil：%+v", models[1].Reasoning)
	}
	if len(catalog.Failures) != 1 || catalog.Failures[0].ID != "broken" || catalog.Failures[0].Name != "Broken" || catalog.Failures[0].Message != "目录读取失败" {
		t.Fatalf("failures 解码不对：%+v", catalog.Failures)
	}
}

func TestSessionModelCatalogNotImplementedIsNotAnEmptyCatalog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"session.modelCatalog 未接线","hint":"宿主没有 OfficialModelCatalogHost","retryable":false}}`))
	}))
	defer srv.Close()

	catalog, err := New(srv.URL).SessionModelCatalog(context.Background())
	if err == nil {
		t.Fatalf("501 必须报错，不能当成空目录：catalog=%+v", catalog)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("应该是 *APIError，得到 %T: %v", err, err)
	}
	if apiErr.Code != "not_implemented" || apiErr.Message != "session.modelCatalog 未接线" {
		t.Fatalf("501 原文没有保留：%+v", apiErr)
	}
	if len(catalog.Groups) != 0 || len(catalog.Failures) != 0 || catalog.Default != (ModelSelection{}) {
		t.Fatalf("失败时不应返回半条目录：%+v", catalog)
	}
}
