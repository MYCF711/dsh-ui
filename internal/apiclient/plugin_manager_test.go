// plugin_manager_test.go —— 官方 `pluginManager.*` 客户端的契约测试。
//
// 判据对着宿主 `internal/transport/handlers_pluginmanager_official.go` 的结构体
// 与 `cmd/dshd/serve_plugins_official.go` 的真实载荷，不是猜的：
//
//	① 零参数方法走裸对象（`{}`），路径是 `/api/pluginManager.listBundles`；
//	② `rows`/`overrides` 在官方 zod 里是必填数组：空也必须是 `[]`，不是 nil；
//	③ `update` 缺省 = 第三态（换不了），界面据此不画更新入口；
//	④ `meta` 缺席时标题回退到 bundle 名（官方 `packageText`），不编标题；
//	⑤ 写方法的 `application === 'failed'` 是**管理性失败**（要看 error.code），
//	   不是 HTTP 错误；
//	⑥ 宿主没接线时回 501 `not_implemented`（*APIError），界面据此画
//	   `unavailable`，**不吞成空清单**。
package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPluginBundlesSendsBareObjectAndDecodesHostShape(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":[
			{"name":"whale-pet","official":false,"availability":"profile","version":"1.2.3",
			 "enabled":true,"installed":true,"source":"/tmp/whale-pet","optional":false,
			 "removable":true,"updatable":false,"rows":[],"overrides":[]}
		]}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).PluginBundles(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotPath != "/api/pluginManager.listBundles" {
		t.Fatalf("路径 = %q，应为 /api/pluginManager.listBundles", gotPath)
	}
	if len(body) != 0 {
		t.Fatalf("零参数方法应发空对象，实得 %v", body)
	}
	if len(got) != 1 {
		t.Fatalf("bundles = %d 条，应为 1", len(got))
	}
	b := got[0]
	if b.Name != "whale-pet" || b.Version != "1.2.3" || b.Source != "/tmp/whale-pet" {
		t.Fatalf("解出来的一行不对：%+v", b)
	}
	if !b.Enabled || !b.Installed || b.Official || b.Optional {
		t.Fatalf("启用/安装/官方/可选 四位不对：%+v", b)
	}
	if b.Update != "" {
		t.Fatalf("本仓 listBundles 不产出 update（第三态），实得 %q", b.Update)
	}
	// 官方 zod 里 rows/overrides 是必填数组：空也必须是 []，不是 nil 的"未知"。
	if b.Rows == nil || len(b.Rows) != 0 {
		t.Fatalf("rows 必须解成空数组，实得 %#v", b.Rows)
	}
	if b.Overrides == nil || len(b.Overrides) != 0 {
		t.Fatalf("overrides 必须解成空数组，实得 %#v", b.Overrides)
	}
	// meta 缺席 → 标题回退到包名（官方 packageText），不编。
	if b.Title() != "whale-pet" {
		t.Fatalf("Title() = %q，meta 缺席应回退包名", b.Title())
	}
	if b.DescriptionText() != "" {
		t.Fatalf("DescriptionText() = %q，没有简介就应是空串", b.DescriptionText())
	}
}

func TestPluginBundleMetaFallbackChain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":[
			{"name":"a","enabled":true,"installed":true,"removable":true,"rows":[],"overrides":[],
			 "meta":{"title":{"zh":"鲸鱼宠物","en":"Whale Pet"}},"description":"包清单里的简介"},
			{"name":"b","enabled":false,"installed":true,"removable":true,"rows":[],"overrides":[],
			 "meta":{"title":"字面标题","description":"字面简介"}}
		]}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).PluginBundles(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if got[0].Title() != "鲸鱼宠物" {
		t.Fatalf("字典形态标题应取 zh：%q", got[0].Title())
	}
	// meta 里没有 description 时回退到包清单的 description（官方 packageText 同链）。
	if got[0].DescriptionText() != "包清单里的简介" {
		t.Fatalf("简介回退 = %q", got[0].DescriptionText())
	}
	if got[1].Title() != "字面标题" || got[1].DescriptionText() != "字面简介" {
		t.Fatalf("字面形态没解对：%q / %q", got[1].Title(), got[1].DescriptionText())
	}
}

func TestPluginBundlesMissingCapabilityIsNotImplemented(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"官方 pluginManager API 未接线","hint":"","retryable":false}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).PluginBundles(context.Background())
	if err == nil {
		t.Fatal("501 必须是错误，不能被吞成空清单")
	}
	if got != nil {
		t.Fatalf("失败时不应有清单：%#v", got)
	}
	if !PluginManagerUnavailable(err) {
		t.Fatalf("not_implemented 必须被识别成'宿主没这个能力'：%v", err)
	}
}

func TestPluginReadBundleDetailSendsNameAndLanguage(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":{"status":"found","origin":"readme","path":"README.md","markdown":"# 鲸鱼宠物\n"}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).PluginReadBundleDetail(context.Background(), "whale-pet", "zh-CN")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotPath != "/api/pluginManager.readBundleDetail" {
		t.Fatalf("路径 = %q", gotPath)
	}
	if body["name"] != "whale-pet" || body["language"] != "zh-CN" {
		t.Fatalf("入参 = %v，应带 name/language", body)
	}
	if !got.Found() || got.Path != "README.md" {
		t.Fatalf("文档没解对：%+v", got)
	}
}

func TestPluginReadBundleDetailNoneIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"status":"none"}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).PluginReadBundleDetail(context.Background(), "bare", "zh-CN")
	if err != nil {
		t.Fatalf("官方 none 分支不是错误：%v", err)
	}
	if got.Found() {
		t.Fatalf("none 不应被当成读到文档：%+v", got)
	}
}

func TestPluginChangeResultFailedIsManagementFailureNotHTTPError(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":{"changed":false,"application":"failed","stage":"enable",
			"target":"bad","error":{"code":"unsupported-platform","diagnostic":"只支持 Go/WASM 包"}}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).PluginSetBundleEnabled(context.Background(), "bad", true)
	if err != nil {
		t.Fatalf("管理性失败走 result，不是 HTTP 错误：%v", err)
	}
	if !got.Failed() {
		t.Fatalf("application=failed 必须被判成失败：%+v", got)
	}
	if got.Error == nil || got.Error.Code != "unsupported-platform" {
		t.Fatalf("失败码没解出来：%+v", got.Error)
	}
	if body["name"] != "bad" || body["enabled"] != true {
		t.Fatalf("写方法入参 = %v", body)
	}
}

func TestPluginRemoveBundleSendsName(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":{"changed":true,"application":"applied","stage":"remove","target":"whale-pet"}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).PluginRemoveBundle(context.Background(), "whale-pet")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotPath != "/api/pluginManager.removeBundle" {
		t.Fatalf("路径 = %q", gotPath)
	}
	if body["name"] != "whale-pet" {
		t.Fatalf("入参 = %v", body)
	}
	if !got.Changed || got.Stage != "remove" {
		t.Fatalf("结果没解对：%+v", got)
	}
}
