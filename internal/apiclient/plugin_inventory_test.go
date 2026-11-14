// plugin_inventory_test.go —— 官方 `pluginInventory.list` 的客户端契约测试。
//
// 判据对着宿主 `internal/transport/handlers_plugin_inventory_official.go`
// 与官方 `dsh-host-plugin-inventory/src/types.ts`，不是猜的：
//
//	① 零参数走裸对象（`{}`），路径是 `/api/pluginInventory.list`；
//	② `fiberPhase: null` 与"字段缺失"**不同**：前者是合法取值（没有活 Fiber）；
//	③ `agentPresets` 缺席 ≠ 空数组（前者=部署没有这个面）；
//	④ `meta` 缺席时标题回退到 `moduleName`，界面不编标题；
//	⑤ `meta.title` 的两种官方形态（字符串 / 语言字典）都认；
//	⑥ 宿主没接线时回 *APIError 四字段，**不吞成空清单**。
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPluginInventoryListSendsBareObject(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":{"managementAvailable":true,"entries":[
			{"entryId":"hello","moduleName":"plugin.wasm","enabled":true,"fiberPhase":"active"},
			{"entryId":"off","moduleName":"other.wasm","enabled":false,"fiberPhase":null},
			{"entryId":"broken","moduleName":"bad.wasm","enabled":true,"fiberPhase":"failed","diagnostic":"装载失败：签名不对"}
		]}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).PluginInventoryList(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotPath != "/api/pluginInventory.list" {
		t.Fatalf("路径 = %q，应为 /api/pluginInventory.list", gotPath)
	}
	if len(body) != 0 {
		t.Fatalf("零参数方法应发空对象，实得 %v", body)
	}
	if got.ManagementAvailable == nil || !*got.ManagementAvailable {
		t.Fatalf("managementAvailable = %v，宿主给了 true", got.ManagementAvailable)
	}
	if len(got.Entries) != 3 {
		t.Fatalf("entries = %d 条，应为 3", len(got.Entries))
	}
	if got.Entries[0].Phase() != PluginPhaseActive {
		t.Fatalf("第 1 条 phase = %q，应为 active", got.Entries[0].Phase())
	}
	// `fiberPhase: null` 必须解成"字段在、值是 null"，而不是漏解成"没这个字段"。
	if !got.Entries[1].FiberPhase.Present {
		t.Fatal("第 2 条 fiberPhase 显式给了 null，不能解成字段缺失")
	}
	if got.Entries[1].Phase() != "" {
		t.Fatalf("第 2 条 phase = %q，null 应归一成空串", got.Entries[1].Phase())
	}
	if got.Entries[2].Phase() != PluginPhaseFailed {
		t.Fatalf("第 3 条 phase = %q，应为 failed", got.Entries[2].Phase())
	}
	if got.Entries[2].Diagnostic != "装载失败：签名不对" {
		t.Fatalf("第 3 条 diagnostic = %q", got.Entries[2].Diagnostic)
	}
	// 官方把 agentPresets 标为 optional：宿主没给 = 没有这个面。
	if got.AgentPresets != nil {
		t.Fatalf("agentPresets 缺席时必须是 nil，实得 %#v", got.AgentPresets)
	}
}

func TestPluginInventoryMetaAbsentMeansFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"entries":[
			{"entryId":"bare","moduleName":"plugin.wasm","enabled":true,"fiberPhase":null},
			{"entryId":"meta","moduleName":"m.wasm","enabled":true,"fiberPhase":null,
			 "meta":{"title":"字面标题","description":"字面简介"}},
			{"entryId":"localized","moduleName":"l.wasm","enabled":true,"fiberPhase":null,
			 "meta":{"title":{"en":"English","zh":"中文标题"},"icon":"data:image/png;base64,AA"}}
		]}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).PluginInventoryList(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if got.Entries[0].Meta != nil {
		t.Fatalf("第 1 条没有 meta，不该编一个：%+v", got.Entries[0].Meta)
	}
	if got.Entries[1].Meta == nil || got.Entries[1].Meta.Title.Pick() != "字面标题" {
		t.Fatalf("字符串形态标题解错：%+v", got.Entries[1].Meta)
	}
	meta := got.Entries[2].Meta
	if meta == nil {
		t.Fatal("第 3 条应带 meta")
	}
	// 对象形态按中文优先（官方 resolveText 的等价物），**不是**取任意键。
	if meta.Title.Pick() != "中文标题" {
		t.Fatalf("语言字典标题 = %q，应取 zh", meta.Title.Pick())
	}
	if meta.Icon != "data:image/png;base64,AA" {
		t.Fatalf("icon = %q", meta.Icon)
	}
}

func TestPluginInventoryBadMetaDoesNotBecomeTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"entries":[
			{"entryId":"weird","moduleName":"w.wasm","enabled":true,"fiberPhase":null,
			 "meta":{"title":42}}
		]}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).PluginInventoryList(context.Background())
	if err != nil {
		t.Fatalf("坏 meta 不该让整次调用失败：%v", err)
	}
	if title := got.Entries[0].Meta.Title.Pick(); title != "" {
		t.Fatalf("解不出的 title = %q，必须留空让调用方回退到 moduleName，不许把 42 当标题", title)
	}
}

func TestPluginInventoryAgentPresetsShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"entries":[],"agentPresets":[
			{"id":"default","name":"默认","isDefault":true,"rows":[
				{"entryId":null,"moduleName":"t.wasm","enabled":true,"fiberPhase":"active"},
				{"entryId":"c","moduleName":"c.wasm","enabled":"conditional","condition":"ctx.get('x')","fiberPhase":null}
			]}
		]}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).PluginInventoryList(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	// 空 entries 与"有 agentPresets 面"是两件事，都要如实解出来。
	if got.Entries == nil || len(got.Entries) != 0 {
		t.Fatalf("entries 应为空数组（不是 nil）：%#v", got.Entries)
	}
	if len(got.AgentPresets) != 1 {
		t.Fatalf("agentPresets = %d，应为 1", len(got.AgentPresets))
	}
	preset := got.AgentPresets[0]
	if preset.ID != "default" || !preset.IsDefault || preset.Name != "默认" {
		t.Fatalf("preset = %+v", preset)
	}
	if preset.Rows[0].EntryID != nil {
		t.Fatalf("官方允许 entryId: null，不该被补成空串：%v", *preset.Rows[0].EntryID)
	}
	if preset.Rows[1].Enabled.Conditional != true || preset.Rows[1].Enabled.Value {
		t.Fatalf("`'conditional'` 必须解成 Conditional：%+v", preset.Rows[1].Enabled)
	}
	if preset.Rows[1].Condition != "ctx.get('x')" {
		t.Fatalf("condition = %q", preset.Rows[1].Condition)
	}
}

func TestPluginInventoryNotImplementedIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"pluginInventory.list 未接线",
			"hint":"宿主没有挂接已安装插件清单","retryable":false}}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL).PluginInventoryList(context.Background())
	if err == nil {
		t.Fatal("宿主回 501 时必须返回错误，不许当空清单")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("应为 *APIError，实得 %T: %v", err, err)
	}
	if apiErr.Code != "not_implemented" {
		t.Fatalf("code = %q，应原样保留宿主的 not_implemented", apiErr.Code)
	}
}
