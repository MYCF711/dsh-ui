// plugin_inventory_e2e_test.go —— 钉住「宿主 pluginInventory.list -> 设置面板内置插件页」这条真链路。
//
// 为什么需要它：这一页的价值全在"它接的是真数据"。单测里塞一份假清单只能证明
// 投影逻辑对，**证明不了壳真的会去问宿主、也证明不了宿主回的形状能被解**。
// 所以这里用 httptest 起一个按**真宿主 wire 形状**应答的宿主，把三段跑通：
//
//	POST /api/pluginInventory.list  ->  m.pluginInventorySnapshot()  ->  BuildPanelWithPlugins（卡片）
//
// 三个方向都钉：有清单（画卡）/ 501 没这条能力（未知 + 重试）/ 空清单（暂无插件）。
//
// 真宿主的 wire 形状来源：DSH-GO `internal/transport/handlers_plugin_inventory_official.go:26-69`
// 的结果类型 + `cmd/dshd/serve_plugins_official.go:54-74`（entryId=bundle name、
// moduleName=manifest entry、fiberPhase 只有 active/null）。
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/plugins/dssettings"
)

// pluginInventoryProbeServer 造一个只回答插件清单（与 settings.describe）的宿主。
func pluginInventoryProbeServer(t *testing.T, inventoryResponse func() (int, string)) (*httptest.Server, *string) {
	t.Helper()
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[len("/api/"):]
		switch method {
		case "pluginInventory.list":
			gotPath = r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			status, body := inventoryResponse()
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		case "settings.describe":
			_, _ = w.Write([]byte(`{"result":{"namespaces":[],"sections":[],"errors":[],"hasDocument":false}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"x","hint":"y","retryable":false}}`))
		}
	}))
	return srv, &gotPath
}

// pluginPageTexts 把插件页上"用户真能看到"的文本摊平（断卡片/标签/详情用）。
func pluginPageTexts(t *testing.T, inventory *apiclient.PluginInventoryResult) (dssettings.PanelModel, []string) {
	t.Helper()
	model := dssettings.BuildPanelWithPlugins(nil, dssettings.HostFacts{}, inventory,
		dssettings.PluginsView{GlobalOpen: boolPointer(true)})
	section, ok := model.Section("plugins")
	if !ok || section.Plugins == nil {
		t.Fatalf("模型里没有插件页投影：%+v", section)
	}
	var texts []string
	for _, group := range section.Plugins.Groups {
		texts = append(texts, group.Title, group.Subtitle)
		for _, card := range group.Cards {
			texts = append(texts, card.Title, card.State)
			if card.EntryChip != "" {
				texts = append(texts, card.EntryChip)
			}
			for _, fact := range card.Facts {
				texts = append(texts, fact.Label, fact.Value)
			}
		}
	}
	return model, texts
}

func TestPluginInventoryChainRealHost(t *testing.T) {
	srv, gotPath := pluginInventoryProbeServer(t, func() (int, string) {
		// 逐字照真宿主的输出形状（字段名与可选性都对齐）。
		return http.StatusOK, `{"result":{"managementAvailable":true,"entries":[
			{"entryId":"hello","moduleName":"plugin.wasm","enabled":true,"fiberPhase":"active"},
			{"entryId":"off","moduleName":"other.wasm","enabled":false,"fiberPhase":null}
		]}}`
	})
	defer srv.Close()

	m := &model{cli: apiclient.New(srv.URL)}
	m.refreshPluginInventory()

	inventory := m.pluginInventorySnapshot()
	if inventory == nil {
		t.Fatal("宿主 200 之后快照不能还是 nil")
	}
	if len(inventory.Entries) != 2 {
		t.Fatalf("解出 %d 条，宿主给了 2 条：%+v", len(inventory.Entries), inventory.Entries)
	}
	if *gotPath != "/api/pluginInventory.list" {
		t.Fatalf("请求路径 = %q", *gotPath)
	}

	model, texts := pluginPageTexts(t, inventory)
	section, _ := model.Section("plugins")
	if section.Plugins.State != dssettings.PluginsReady {
		t.Fatalf("页状态 = %q，应为 ready", section.Plugins.State)
	}
	// 用户看得见的文本里，必须有真实条目名与状态——这就是"不是空壳"的判据。
	joined := ""
	for _, text := range texts {
		joined += text + "|"
	}
	for _, want := range []string{"全局插件", "plugin.wasm", "other.wasm", "已启用", "已停用", "运行中"} {
		if !containsText(texts, want) {
			t.Fatalf("页面上找不到 %q（实际：%s）", want, joined)
		}
	}
}

func TestPluginInventoryChainHostNotImplemented(t *testing.T) {
	srv, _ := pluginInventoryProbeServer(t, func() (int, string) {
		return http.StatusNotImplemented, `{"error":{"code":"not_implemented",
			"message":"pluginInventory.list 未接线","hint":"宿主没有挂接已安装插件清单","retryable":false}}`
	})
	defer srv.Close()

	m := &model{cli: apiclient.New(srv.URL)}
	m.refreshPluginInventory()

	if inventory := m.pluginInventorySnapshot(); inventory != nil {
		t.Fatalf("501 之后快照必须是 nil（未知），实得 %+v", inventory)
	}
	model := dssettings.BuildPanelWithPlugins(nil, dssettings.HostFacts{}, m.pluginInventorySnapshot(),
		dssettings.PluginsView{})
	section, _ := model.Section("plugins")
	if section.Plugins.State != dssettings.PluginsUnknown {
		t.Fatalf("没这条能力时状态 = %q，应为 unknown", section.Plugins.State)
	}
	if !section.Plugins.ShowRetry {
		t.Fatal("unknown 态要给「重试」")
	}
	// 失败要说人话（状态栏），而不是静默。
	if m.status == "" {
		t.Fatal("拉失败必须报状态栏")
	}
}

func TestPluginInventoryChainEmptyList(t *testing.T) {
	srv, _ := pluginInventoryProbeServer(t, func() (int, string) {
		return http.StatusOK, `{"result":{"managementAvailable":true,"entries":[]}}`
	})
	defer srv.Close()

	m := &model{cli: apiclient.New(srv.URL)}
	m.refreshPluginInventory()
	inventory := m.pluginInventorySnapshot()
	if inventory == nil {
		t.Fatal("空清单也是成功应答，快照不该是 nil")
	}
	if len(inventory.Entries) != 0 {
		t.Fatalf("空清单解出 %d 条", len(inventory.Entries))
	}
	model := dssettings.BuildPanelWithPlugins(nil, dssettings.HostFacts{}, inventory, dssettings.PluginsView{})
	section, _ := model.Section("plugins")
	if section.Plugins.State != dssettings.PluginsReady || !section.Plugins.Empty {
		t.Fatalf("空清单应是 ready+Empty：%+v", section.Plugins)
	}
}

func containsText(texts []string, want string) bool {
	for _, text := range texts {
		if text == want {
			return true
		}
	}
	return false
}

func boolPointer(v bool) *bool { return &v }
