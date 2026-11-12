// settings_test.go —— HOST-SETTINGS-FORM 六个方法的客户端契约测试。
//
// 判据对着宿主 handler 与 `../DSH-GO/docs/settings-form.md`，不是猜的：
//
//	① 六个方法都走 `POST /api/<method>`、裸参数对象（无 params 包装）；
//	② `settings.describe` 的 result 有 writable/hasDocument/namespaces/sections/errors；
//	③ 未知 namespace / field 用 `(v, false)` 表达，**不给零值兜底**；
//	④ 409 `revision_conflict` 保持四字段（不吞成 generic error）；
//	⑤ `settings.form.reset` 的 path 省略 ≠ `path: null`（后者宿主拒绝）；
//	⑥ `SettingValueString` 对 nil 返回 false（界面显示未知，不补 0）。
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSettingsDescribeSendsBareObject(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":{"writable":true,"hasDocument":false,
			"namespaces":[{"ns":"permission","revision":2,"value":{"defaultPreset":"ask"}}],
			"sections":[{"ns":"permission","title":"权限","writable":true,"revision":2,
				"fields":[{"name":"defaultPreset","path":["defaultPreset"],"type":"string",
					"current":"ask","default":"allow","overridden":true,"writable":true,
					"enum":["deny","ask","allow","auto"]}],"errors":[]}],"errors":[]}}`))
	}))
	defer srv.Close()

	describe, err := New(srv.URL).SettingsDescribe(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotPath != "/api/settings.describe" {
		t.Fatalf("路径 = %q，应为 /api/settings.describe", gotPath)
	}
	if len(body) != 0 {
		t.Fatalf("裸参数应为空对象，实得 %v", body)
	}
	if !describe.Writable || describe.HasDocument {
		t.Fatalf("顶层字段 = %+v", describe)
	}
	ns, ok := describe.Namespace(SettingsNSPermission)
	if !ok {
		t.Fatal("permission 命名空间应在结果里")
	}
	if ns.Revision != 2 || string(ns.Value) != `{"defaultPreset":"ask"}` {
		t.Fatalf("namespace = %+v（value 必须原样保留宿主 JSON）", ns)
	}
	section, ok := describe.Section(SettingsNSPermission)
	if !ok {
		t.Fatal("permission section 应在结果里")
	}
	field, ok := section.Field("defaultPreset")
	if !ok {
		t.Fatal("defaultPreset 字段应在结果里")
	}
	if field.Current != "ask" || field.Default != "allow" || !field.Overridden || !field.Writable {
		t.Fatalf("字段投影 = %+v", field)
	}
	if len(field.Enum) != 4 || field.Enum[0] != "deny" {
		t.Fatalf("枚举 = %v", field.Enum)
	}
	// 未知 namespace / field 必须返回 false（不给零值）。
	if _, ok := describe.Namespace("nope"); ok {
		t.Fatal("未知 namespace 应返回 false")
	}
	if _, ok := section.Field("nope"); ok {
		t.Fatal("未知 field 应返回 false")
	}
}

func TestSettingsFormSetSendsPathValueAndRevision(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":{"ns":"permission","title":"权限","writable":true,
			"revision":3,"fields":[],"errors":[]}}`))
	}))
	defer srv.Close()

	rev := 2
	section, err := New(srv.URL).SettingsFormSet(context.Background(), "permission",
		[]string{"defaultPreset"}, "deny", &rev)
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotPath != "/api/settings.form.set" {
		t.Fatalf("路径 = %q", gotPath)
	}
	if body["ns"] != "permission" {
		t.Fatalf("ns = %v", body["ns"])
	}
	if body["value"] != "deny" {
		t.Fatalf("value = %v", body["value"])
	}
	if body["expectedRevision"] != float64(2) {
		t.Fatalf("expectedRevision = %v", body["expectedRevision"])
	}
	path, ok := body["path"].([]any)
	if !ok || len(path) != 1 || path[0] != "defaultPreset" {
		t.Fatalf("path = %v", body["path"])
	}
	if section.Revision != 3 {
		t.Fatalf("写后 revision = %d（应来自 result，不是本地 +1）", section.Revision)
	}
}

func TestSettingsFormResetOmitsPathWhenNil(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":{"ns":"permission","title":"权限","writable":true,"revision":4,"fields":[],"errors":[]}}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL).SettingsFormReset(context.Background(), "permission", nil, nil); err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if _, ok := body["path"]; ok {
		t.Fatalf("清整个 namespace 时必须**省略** path（显式 null 会被宿主拒绝）：%v", body)
	}
	if _, ok := body["expectedRevision"]; ok {
		t.Fatalf("nil revision 必须省略：%v", body)
	}

	// 带 path 时原样发。
	if _, err := New(srv.URL).SettingsFormReset(context.Background(), "permission", []string{"defaultPreset"}, nil); err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if body["path"] == nil {
		t.Fatal("带 path 时应发送 path")
	}
}

func TestSettingsMutateSendsOps(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":{"ns":"llm","revision":6,"value":{}}}`))
	}))
	defer srv.Close()

	rev := 5
	view, err := New(srv.URL).SettingsMutate(context.Background(), SettingsNSLLM,
		retryPolicyWriteOpsForTest("deepseek", "maxRetries", 9), &rev)
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotPath != "/api/settings.mutate" {
		t.Fatalf("路径 = %q", gotPath)
	}
	ops, ok := body["ops"].([]any)
	if !ok || len(ops) != 1 {
		t.Fatalf("ops = %v", body["ops"])
	}
	op := ops[0].(map[string]any)
	if op["op"] != "set" {
		t.Fatalf("op = %v", op["op"])
	}
	p, ok := op["path"].([]any)
	if !ok || len(p) != 4 || p[0] != "providers" || p[3] != "maxRetries" {
		t.Fatalf("path = %v（官方形状 providers.<route>.retryPolicy.<field>）", op["path"])
	}
	if view.Revision != 6 {
		t.Fatalf("revision = %d", view.Revision)
	}
}

func TestSettingsRevisionConflictKeepsFourFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":{"code":"revision_conflict",
			"message":"permission revision 已变化：expected=2 actual=3，未覆盖新值",
			"hint":"重新 settings.describe/get，基于最新 revision 再提交","retryable":false}}`))
	}))
	defer srv.Close()

	rev := 2
	_, err := New(srv.URL).SettingsFormSet(context.Background(), "permission",
		[]string{"defaultPreset"}, "deny", &rev)
	if err == nil {
		t.Fatal("409 必须报错")
	}
	if code := SettingsErrorCode(err); code != SettingsCodeRevisionConflict {
		t.Fatalf("code = %q，应为 revision_conflict", code)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("错误应能取到 *APIError：%T", err)
	}
	if apiErr.Hint == "" || apiErr.Message == "" {
		t.Fatalf("四字段被吞了：%+v", apiErr)
	}
	if SettingsRetryable(err) {
		t.Fatal("revision_conflict 的 retryable 是 false")
	}
}

func TestSettingsFormGetAndDescribeShareShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/settings.form.get":
			_, _ = w.Write([]byte(`{"result":{"ns":"model","title":"模型","writable":true,"revision":1,
				"fields":[{"name":"defaultModel","path":["defaultModel"],"type":"string",
					"current":"m","default":"","overridden":true,"writable":true,"minLength":1,"maxLength":256}],
				"errors":[]}}`))
		case "/api/settings.form.describe":
			_, _ = w.Write([]byte(`{"result":{"writable":true,"hasDocument":false,"namespaces":[],"sections":[],"errors":[]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	section, err := New(srv.URL).SettingsFormGet(context.Background(), "model")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	field, _ := section.Field("defaultModel")
	if field.MinLength == nil || *field.MinLength != 1 || field.MaxLength == nil || *field.MaxLength != 256 {
		t.Fatalf("长度约束 = %+v", field)
	}
	if _, err := New(srv.URL).SettingsFormDescribe(context.Background()); err != nil {
		t.Fatalf("form.describe 应当成功：%v", err)
	}
}

// TestSettingValueStringNeverFabricates 钉住「未知就是未知」。
func TestSettingValueStringNeverFabricates(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  string
		known bool
	}{
		{"nil 是未知", nil, "", false},
		{"字符串", "ask", "ask", true},
		{"true", true, "true", true},
		{"false", false, "false", true},
		{"整数", float64(14), "14", true},
		{"5e-324 不能渲染成 0", 5e-324, "5e-324", true},
		{"指数去前导零", 1e-5, "1e-5", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, known := SettingValueString(tc.value)
			if known != tc.known {
				t.Fatalf("known = %v，期望 %v", known, tc.known)
			}
			if got != tc.want {
				t.Fatalf("文本 = %q，期望 %q", got, tc.want)
			}
		})
	}
	// 数组（retryableCodes）交给专用渲染器，但文本不能丢。
	if got, known := SettingValueString([]string{"RATE_LIMIT", "SERVER"}); !known || got != `["RATE_LIMIT","SERVER"]` {
		t.Fatalf("数组文本 = %q known=%v", got, known)
	}
}

// TestRetryPolicyPathHelpers 钉住路径拼法。
func TestRetryPolicyPathHelpers(t *testing.T) {
	got := RetryPolicyPath("deepseek", "maxRetries")
	want := []string{"providers", "deepseek", "retryPolicy", "maxRetries"}
	if len(got) != len(want) {
		t.Fatalf("路径 = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("路径 = %v，期望 %v", got, want)
		}
	}
	backoff := RetryPolicyBackoffPath("deepseek", "jitterRatio")
	if len(backoff) != 5 || backoff[3] != "backoff" || backoff[4] != "jitterRatio" {
		t.Fatalf("backoff 路径 = %v", backoff)
	}
}

// retryPolicyWriteOpsForTest 是测试侧的路径构造（apiclient 不 import 插件包，
// 所以这里重复一份**官方形状**的拼法；两边由各自测试钉住）。
func retryPolicyWriteOpsForTest(route, field string, value any) []SettingsOp {
	return []SettingsOp{SettingsSetOp(RetryPolicyPath(route, field), value)}
}
