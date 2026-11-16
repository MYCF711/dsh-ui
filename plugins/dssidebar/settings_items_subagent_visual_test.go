//go:build darwin || windows || linux

package dssidebar

import (
	"encoding/json"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// TestSettingsSubagentModelClickSaveWritesExactRoutes 走真 Viva：开卡 → Switch →
// 勾选真实目录候选 → Save，并核对同一次 mutate 的两个字段与 baseline revision。
func TestSettingsSubagentModelClickSaveWritesExactRoutes(t *testing.T) {
	h := openSettingsItemHarness(t, 1100, 1100)
	defer h.close()
	h.click("settings-item-card-subagent")
	for _, want := range []string{zhSubagentModels, zhSubagentModelToggle, zhSubagentModelOff} {
		if !h.hasText(want) {
			t.Fatalf("模型选择初始态缺 %q；texts=%q", want, h.session.Texts())
		}
	}
	h.click("settings-subagent-model-enabled")
	for _, want := range []string{zhSubagentModelChoose, zhSubagentModelAllowed, "提供方 A", "模型 A1", zhSubagentModelStaleGroup, "saved-model"} {
		if !h.hasText(want) {
			t.Fatalf("开启后缺官方内容 %q；texts=%q", want, h.session.Texts())
		}
	}
	route := settingsSubagentModelRoute{Provider: "provider-a", Model: "model-a1"}
	h.click("settings-subagent-model-" + settingsSubagentModelKey(route))
	h.capture("settings-subagent-model-selection.png")
	before := h.spy.count()
	h.click("settings-item-save-subagent")
	if !h.waitMutate(before + 1) {
		t.Fatal("点击保存没有发出 settings.mutate")
	}
	call, ok := h.spy.last()
	if !ok || call.ns != settingsNSSubagentModelSelection || call.expectedRe != 11 {
		t.Fatalf("写目标/revision 错误：%+v", call)
	}
	if len(call.ops) != 2 || call.ops[0].Path[0] != settingsFieldSubagentModelEnabled || call.ops[1].Path[0] != settingsFieldAllowedModels {
		t.Fatalf("两字段必须同批写入：%+v", call.ops)
	}
	if enabled, ok := call.ops[0].Value.(bool); !ok || !enabled {
		t.Fatalf("enabled 写入值=%#v", call.ops[0].Value)
	}
	routes, ok := call.ops[1].Value.([]settingsSubagentModelRoute)
	if !ok || len(routes) != 2 {
		t.Fatalf("allowedModels 写入值=%#v", call.ops[1].Value)
	}
	want := map[string]bool{"saved-provider\x00saved-model": true, "provider-a\x00model-a1": true}
	for _, saved := range routes {
		delete(want, settingsSubagentModelKey(saved))
	}
	if len(want) != 0 {
		t.Fatalf("写入路由缺少 %+v；实际 %+v", want, routes)
	}
	h.pump()
	drafts, _, _ := h.p.ensureSettingsItemForm().snapshot()
	if len(drafts) != 0 {
		t.Fatalf("成功后草稿必须清空：%v", drafts)
	}
}

func TestSettingsSubagentModelEmptySelectionDrawsRequired(t *testing.T) {
	h := openSettingsItemHarness(t, 1100, 1000)
	defer h.close()
	h.mu.Lock()
	h.catalog = &apiclient.ModelsPageSnapshot{Catalog: &apiclient.ModelCatalog{}}
	h.mu.Unlock()
	h.click("settings-item-card-subagent")
	h.click("settings-subagent-model-enabled")
	stale := settingsSubagentModelRoute{Provider: "saved-provider", Model: "saved-model"}
	h.click("settings-subagent-model-" + settingsSubagentModelKey(stale))
	if !h.hasText(zhSubagentModelRequired) {
		t.Fatalf("enabled 且空选择必须画官方 required；texts=%q", h.session.Texts())
	}
	if h.spy.count() != 0 {
		t.Fatal("仅编辑草稿不该提前发写请求")
	}
}

func TestSettingsSubagentModelCatalogErrorRetryReallyRefreshes(t *testing.T) {
	h := openSettingsItemHarness(t, 1100, 1000)
	defer h.close()
	h.mu.Lock()
	h.catalog = &apiclient.ModelsPageSnapshot{CatalogError: "not_implemented"}
	h.mu.Unlock()
	h.click("settings-item-card-subagent")
	h.click("settings-subagent-model-enabled")
	if !h.hasText(zhSubagentModelLoadFailed) {
		t.Fatalf("目录失败必须画官方错误：%q", h.session.Texts())
	}
	h.click("settings-subagent-model-retry")
	h.mu.Lock()
	retries := h.catalogRetries
	h.mu.Unlock()
	if retries != 1 {
		t.Fatalf("重试次数=%d，want 1", retries)
	}
	if !h.hasText("模型 A1") {
		t.Fatal("重试成功后必须画真实目录，而不是保留空错误页")
	}
}

func TestSettingsSubagentModelCatalogLoadingPartialAndEmptyDraw(t *testing.T) {
	t.Run("loading", func(t *testing.T) {
		h := openSettingsItemHarness(t, 1100, 1000)
		defer h.close()
		h.mu.Lock()
		h.catalog = &apiclient.ModelsPageSnapshot{Loading: true}
		h.mu.Unlock()
		h.click("settings-item-card-subagent")
		h.click("settings-subagent-model-enabled")
		if !h.hasText(zhSubagentModelLoading) {
			t.Fatalf("缺 loading 文案：%q", h.session.Texts())
		}
	})
	t.Run("partial", func(t *testing.T) {
		h := openSettingsItemHarness(t, 1100, 1000)
		defer h.close()
		h.mu.Lock()
		h.catalog.Catalog.Failures = []apiclient.ModelFailure{{ID: "provider-b"}}
		h.mu.Unlock()
		h.click("settings-item-card-subagent")
		h.click("settings-subagent-model-enabled")
		if !h.hasText(zhSubagentModelPartial) {
			t.Fatalf("缺 partial 文案：%q", h.session.Texts())
		}
	})
	t.Run("empty", func(t *testing.T) {
		h := openSettingsItemHarness(t, 1100, 1000)
		defer h.close()
		h.mu.Lock()
		h.catalog = &apiclient.ModelsPageSnapshot{Catalog: &apiclient.ModelCatalog{}}
		for index := range h.describe.Namespaces {
			if h.describe.Namespaces[index].NS == settingsNSSubagentModelSelection {
				h.describe.Namespaces[index].Value = json.RawMessage(`{"enabled":false,"allowedModels":[]}`)
			}
		}
		h.mu.Unlock()
		h.click("settings-item-card-subagent")
		h.click("settings-subagent-model-enabled")
		if !h.hasText(zhSubagentModelEmpty) {
			t.Fatalf("真实空目录必须画官方 empty：%q", h.session.Texts())
		}
	})
}
