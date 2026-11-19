// model_catalog_editor_test.go —— `models.catalog.*` 五条 wire 契约测试。
//
// 断言的都是**宿主真实读法**（`../DSH-GO/cmd/dshd/serve_models.go`）：
// 裸参数对象、可选键省略而不是发 null、reorder 要全排列、结果里的 raw model 原样保留。
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// catalogServer 收一份请求体，回一份固定 result。
func catalogServer(t *testing.T, path string, result string, capture *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != path {
			t.Errorf("路径应为 %s，得到 %q", path, got)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if capture != nil {
			*capture = body
		}
		_, _ = w.Write([]byte(result))
	}))
}

const catalogDocumentResult = `{"result":{
	"revision":7,
	"models":[
		{"id":"m-1","name":"一号","contextWindow":131072,"input":["text"],"futureField":{"keep":true}},
		{"id":"m-2"}
	],
	"credentials":{"KEY_ONE":{"configured":true,"writable":true}},
	"active":{"id":"m-1","revision":3}
}}`

func TestModelCatalogListSendsNoParamsAndKeepsRawModels(t *testing.T) {
	var body map[string]any
	srv := catalogServer(t, "/api/models.catalog.list", catalogDocumentResult, &body)
	defer srv.Close()

	doc, err := New(srv.URL).ModelCatalogList(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if len(body) != 0 {
		t.Fatalf("models.catalog.list 是零参数方法，实际发了 %v", body)
	}
	if doc.Revision != 7 {
		t.Fatalf("revision 解码不对：%d", doc.Revision)
	}
	if len(doc.Models) != 2 {
		t.Fatalf("models 数量不对：%d", len(doc.Models))
	}
	// 未知字段必须原样存活：宿主 Upsert 会把没认出的键一起写回，
	// 客户端一旦解成结构体就会把它吃掉。
	var first map[string]any
	if err := json.Unmarshal(doc.Models[0], &first); err != nil {
		t.Fatalf("models[0] 不是原样 JSON：%v", err)
	}
	if _, ok := first["futureField"].(map[string]any); !ok {
		t.Fatalf("未识别字段被丢了：%v", first)
	}
	if doc.Credentials["KEY_ONE"].Configured != true || doc.Credentials["KEY_ONE"].Writable != true {
		t.Fatalf("credentials 解码不对：%+v", doc.Credentials)
	}
	if doc.Active.ID != "m-1" || doc.Active.Revision != 3 {
		t.Fatalf("active 解码不对：%+v", doc.Active)
	}
}

func TestModelCatalogUpsertOmitsAbsentKeysAndKeepsNulls(t *testing.T) {
	var body map[string]any
	srv := catalogServer(t, "/api/models.catalog.upsert", catalogDocumentResult, &body)
	defer srv.Close()

	// model 里显式 null = 删掉那个可选字段（宿主 model_catalog.go:466-471）。
	model := json.RawMessage(`{"id":"m-1","name":null}`)
	if _, err := New(srv.URL).ModelCatalogUpsert(context.Background(), model, nil, nil); err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if _, ok := body["model"]; !ok {
		t.Fatalf("必须带 model object：%v", body)
	}
	if _, ok := body["credential"]; ok {
		t.Fatalf("credential=nil 必须整键省略（宿主拒绝 null）：%v", body)
	}
	if _, ok := body["expectedRevision"]; ok {
		t.Fatalf("expectedRevision=nil 必须整键省略（宿主拒绝 null）：%v", body)
	}
	sent := body["model"].(map[string]any)
	if value, present := sent["name"]; !present || value != nil {
		t.Fatalf("显式 null 是删除语义，必须原样发出去：%v", sent)
	}
}

func TestModelCatalogUpsertSendsExplicitOptionalValues(t *testing.T) {
	var body map[string]any
	srv := catalogServer(t, "/api/models.catalog.upsert", catalogDocumentResult, &body)
	defer srv.Close()

	// 空串 credential 是"删掉这个引用"（model_catalog.go:519-521），不是"没给"。
	credential := ""
	revision := int64(4)
	if _, err := New(srv.URL).ModelCatalogUpsert(context.Background(), json.RawMessage(`{"id":"m-1"}`), &credential, &revision); err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if got, ok := body["credential"].(string); !ok || got != "" {
		t.Fatalf("credential 空串必须真的发出去（删除语义）：%v", body)
	}
	if got, ok := body["expectedRevision"].(float64); !ok || got != 4 {
		t.Fatalf("expectedRevision 必须是数字 4：%v", body)
	}
}

func TestModelCatalogReorderSendsFullPermutation(t *testing.T) {
	var body map[string]any
	srv := catalogServer(t, "/api/models.catalog.reorder", catalogDocumentResult, &body)
	defer srv.Close()

	if _, err := New(srv.URL).ModelCatalogReorder(context.Background(), []string{"m-2", "m-1"}, nil); err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	ids, ok := body["ids"].([]any)
	if !ok || len(ids) != 2 || ids[0] != "m-2" || ids[1] != "m-1" {
		t.Fatalf("ids 必须按调用方给的顺序原样发出：%v", body)
	}
	if _, ok := body["expectedRevision"]; ok {
		t.Fatalf("expectedRevision=nil 必须整键省略：%v", body)
	}
}

func TestModelCatalogReorderNilIDsBecomeEmptyArray(t *testing.T) {
	var body map[string]any
	srv := catalogServer(t, "/api/models.catalog.reorder", `{"error":{"code":"empty_order","message":"reorder ids 不能为空","hint":"","retryable":false}}`, &body)
	defer srv.Close()

	_, err := New(srv.URL).ModelCatalogReorder(context.Background(), nil, nil)
	if err == nil {
		t.Fatalf("宿主拒绝空 ids，客户端必须把错误交出去")
	}
	// nil 与 [] 在 wire 上只能是一种：JSON 数组。发 null 会被宿主判成"缺 ids 数组"。
	if _, ok := body["ids"].([]any); !ok {
		t.Fatalf("ids 必须是 JSON 数组而不是 null：%v", body)
	}
}

func TestModelCatalogTestOmitsAbsentOverrides(t *testing.T) {
	var body map[string]any
	srv := catalogServer(t, "/api/models.catalog.test", `{"result":{"id":"m-1","ok":true,"model":"m-1","latencyMs":42}}`, &body)
	defer srv.Close()

	result, err := New(srv.URL).ModelCatalogTest(context.Background(), ModelCatalogTestRequest{ID: "m-1"})
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if len(body) != 1 || body["id"] != "m-1" {
		t.Fatalf("只有 id 应当出现（三个 override 是省略而不是空串）：%v", body)
	}
	if !result.OK || result.LatencyMs != 42 || result.Model != "m-1" {
		t.Fatalf("结果解码不对：%+v", result)
	}
}

func TestModelCatalogTestSendsOverridesWhenGiven(t *testing.T) {
	var body map[string]any
	srv := catalogServer(t, "/api/models.catalog.test", `{"result":{"id":"m-1","ok":true}}`, &body)
	defer srv.Close()

	baseURL := "https://example.invalid/v1"
	api := "openai-completions"
	credential := "secret"
	if _, err := New(srv.URL).ModelCatalogTest(context.Background(), ModelCatalogTestRequest{
		ID: "m-1", BaseURL: &baseURL, API: &api, Credential: &credential,
	}); err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if body["baseURL"] != baseURL || body["api"] != api || body["credential"] != credential {
		t.Fatalf("三个 override 必须原样发出：%v", body)
	}
}

func TestModelCatalogSetActiveSendsIDAndRevision(t *testing.T) {
	var body map[string]any
	srv := catalogServer(t, "/api/models.catalog.set-active",
		`{"result":{"activeModel":"m-2","activeRevision":5,"revision":7}}`, &body)
	defer srv.Close()

	revision := int64(6)
	out, err := New(srv.URL).ModelCatalogSetActive(context.Background(), "m-2", &revision)
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if body["id"] != "m-2" {
		t.Fatalf("id 必须原样发出：%v", body)
	}
	if got, ok := body["expectedRevision"].(float64); !ok || got != 6 {
		t.Fatalf("expectedRevision 必须是数字 6：%v", body)
	}
	if out.ActiveModel != "m-2" || out.ActiveRevision != 5 || out.Revision != 7 {
		t.Fatalf("结果解码不对：%+v", out)
	}
}

// revision_conflict 必须作为四字段错误交给调用方，客户端不得自己重试或吞掉。
func TestModelCatalogRevisionConflictIsSurfacedVerbatim(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":{"code":"revision_conflict","message":"模型目录 revision 已变化：expected=3 actual=4，未覆盖新值","hint":"","retryable":false}}`))
	}))
	defer srv.Close()

	revision := int64(3)
	doc, err := New(srv.URL).ModelCatalogUpsert(context.Background(), json.RawMessage(`{"id":"m-1"}`), nil, &revision)
	if err == nil {
		t.Fatalf("409 必须报错，不能当成写成功")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("应该是 *APIError，得到 %T: %v", err, err)
	}
	if apiErr.Code != "revision_conflict" {
		t.Fatalf("错误码没保留：%+v", apiErr)
	}
	if len(doc.Models) != 0 || doc.Revision != 0 {
		t.Fatalf("失败时不应返回半个文档：%+v", doc)
	}
}

// 501 = 宿主没有模型目录能力（老宿主），必须报错而不是折叠成空目录。
func TestModelCatalogListNotImplementedIsNotAnEmptyCatalog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"未知模型目录 Transport 方法","hint":"","retryable":false}}`))
	}))
	defer srv.Close()

	doc, err := New(srv.URL).ModelCatalogList(context.Background())
	if err == nil {
		t.Fatalf("501 必须报错，不能当成空目录：%+v", doc)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "not_implemented" {
		t.Fatalf("错误码没保留：%v", err)
	}
	if doc.Models != nil {
		t.Fatalf("失败时 Models 必须保持 nil（与真实空目录区分）：%+v", doc)
	}
}
