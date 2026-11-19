// official_models_catalog_rest_test.go —— #41 收尾四条的语义判据（不测绘制）。
//
// 每条断言对着官方原件的一个 file:line，与绘制/写路径分开：
//
//	行删除      ModelRow.tsx:142-151 → DeepSeekModelsEditor.tsx:157-160 splice
//	baseURL     ProviderEditor.tsx:552-570（placeholder 三分支 :558-561）
//	api         ProviderEditor.tsx:575-595（customApiUnset 空选项 :590）
//	发现测试    ModelListEditor.tsx:212-245 + :270-274（adoptPicked）
package dssettings

import (
	"encoding/json"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// newRestEditor 造一份两条模型的草稿（alpha 带容量与名称，beta 空）。
func newRestEditor(t *testing.T, writable bool) *catalogEditor {
	t.Helper()
	document := apiclient.ModelCatalogDocument{
		Revision: 7,
		Models: []json.RawMessage{
			json.RawMessage(`{"id":"alpha","name":"阿尔法","contextWindow":131072}`),
			json.RawMessage(`{"id":"beta"}`),
		},
	}
	return newCatalogEditor(&apiclient.ModelCatalogSnapshot{Document: &document}, writable)
}

func TestCatalogRemoveModelSplicesAndForgetsRowState(t *testing.T) {
	editor := newRestEditor(t, true)
	first, second := editor.keys[0], editor.keys[1]
	editor.expanded[first] = true
	editor.expanded[second] = true
	editor.setReasoning(first, true)

	if !editor.removeModel(first) {
		t.Fatalf("删已有行必须成功")
	}
	if len(editor.drafts) != 1 || len(editor.keys) != 1 || editor.keys[0] != second {
		t.Fatalf("删一行应 splice 掉它并保住另一行的身份：%d/%v", len(editor.drafts), editor.keys)
	}
	// 行状态必须一起忘掉（官方 `rowState.forget`，ModelListEditor.tsx:272）。
	if _, present := editor.expanded[first]; present {
		t.Fatalf("删掉的行不该留展开态")
	}
	if _, present := editor.efforts[first]; present {
		t.Fatalf("删掉的行不该留档位表")
	}
	if editor.removeModel(first) {
		t.Fatalf("删不存在的行必须返回 false（不许静默成功）")
	}
	// 删到空是合法目标（官方 `.modelEmpty` 文案就是那个状态）。
	if !editor.removeModel(second) {
		t.Fatalf("删最后一行也必须成功（宿主接受 models: []）")
	}
	if len(editor.drafts) != 0 {
		t.Fatalf("应删到空：%d", len(editor.drafts))
	}
}

func TestCatalogSetBaseURLTrimsAndEmptyDeletesTheKey(t *testing.T) {
	editor := newRestEditor(t, true)
	key := editor.keys[0]

	if !editor.setBaseURL(key, "  https://api.example.com/v1  ") {
		t.Fatalf("写 baseURL 应成功")
	}
	if got := ModelText(editor.drafts[0], "baseURL"); got != "https://api.example.com/v1" {
		t.Fatalf("baseURL 应 trim 后落草稿：%q", got)
	}
	// 清空 = 删键（官方 `setField('baseURL', '' → undefined)`，ProviderEditor.tsx:566）。
	editor.setBaseURL(key, "   ")
	if _, present := editor.drafts[0]["baseURL"]; present {
		t.Fatalf("清空 baseURL 必须删键，不许存空串（宿主会 invalid_field）")
	}
	// 宿主真正读的就是这个字段（internal/llm/model_catalog.go 的字段白名单）。
	payload, err := editor.modelPayload(0)
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	if contains := len(payload) > 0 && json.Valid(payload); !contains {
		t.Fatalf("payload 应是合法 JSON：%s", payload)
	}
}

func TestCatalogNextAPICyclesRegisteredProtocols(t *testing.T) {
	editor := newRestEditor(t, true)
	editor.applyRouteFacts([]apiclient.ConfigurableProvider{
		{Provider: "openai-completions"},
		{Provider: "deepseek-completions", DefaultBaseURL: "https://api.deepseek.com/v1"},
		{Provider: "openai-completions"}, // 重复项不许进候选
	})
	if len(editor.protocols) != 2 {
		t.Fatalf("协议候选应按宿主上报去重：%v", editor.protocols)
	}
	// 未选择 → 第一个候选（官方空选项 `customApiUnset` 只在这个状态出现，:590）。
	if next, ok := editor.nextAPI(""); !ok || next != "openai-completions" {
		t.Fatalf("未选择应给第一个候选：%q/%v", next, ok)
	}
	// 已选 → 下一个；走到尾回第一个。
	if next, _ := editor.nextAPI("openai-completions"); next != "deepseek-completions" {
		t.Fatalf("应轮换到下一个候选：%q", next)
	}
	if next, _ := editor.nextAPI("deepseek-completions"); next != "openai-completions" {
		t.Fatalf("到尾应回第一个候选：%q", next)
	}
	// 写进去的 api 是真值（宿主 ResolveProvider 读它，model_catalog.go:1271-1279）。
	key := editor.keys[1]
	if !editor.setAPI(key, "deepseek-completions") {
		t.Fatalf("写 api 应成功")
	}
	if got := ModelText(editor.drafts[1], "api"); got != "deepseek-completions" {
		t.Fatalf("api 没落草稿：%q", got)
	}
	editor.setAPI(key, "")
	if _, present := editor.drafts[1]["api"]; present {
		t.Fatalf("清空 api 必须删键，不许存空串")
	}
}

func TestCatalogBaseURLPlaceholderFollowsOfficialBranches(t *testing.T) {
	editor := newRestEditor(t, true)
	editor.applyRouteFacts([]apiclient.ConfigurableProvider{
		{Provider: "openai-completions", DefaultBaseURL: "https://route.example.com/v1"},
		{Provider: "env-only", BaseURLEnv: []string{"ENV_ONLY_BASE_URL"}},
	})
	// DeepSeek 家族（官方 `family === 'deepseek'`，ProviderEditor.tsx:558-559）。
	if got := catalogBaseURLPlaceholder(editor, ModelDraft{"api": "deepseek-completions"}); got != catalogCopy("deepSeekBaseUrl") {
		t.Fatalf("DeepSeek 家族应取 deepSeekBaseUrl：%q", got)
	}
	// 路由声明了默认端点 → baseUrlPlaceholder 的 `{url}` 换成那条端点。
	if got := catalogBaseURLPlaceholder(editor, ModelDraft{"api": "openai-completions"}); got != "https://route.example.com/v1" {
		t.Fatalf("有默认端点应把 {url} 换掉：%q", got)
	}
	// 只有环境变量 → baseUrlConfigured。
	if got := catalogBaseURLPlaceholder(editor, ModelDraft{"api": "env-only"}); got != catalogCopy("baseUrlConfigured") {
		t.Fatalf("只有 env 应取 baseUrlConfigured：%q", got)
	}
	// 都没有 → baseUrlDefault。
	if got := catalogBaseURLPlaceholder(editor, ModelDraft{}); got != catalogCopy("baseUrlDefault") {
		t.Fatalf("都没有应取 baseUrlDefault：%q", got)
	}
}

func TestCatalogAskableNeedsADeclaredEndpoint(t *testing.T) {
	editor := newRestEditor(t, true)
	if _, _, ok := editor.askable(); ok {
		t.Fatalf("没有 baseURL 时不可问（官方 askable 判据，ModelListEditor.tsx:310-311）")
	}
	editor.setBaseURL(editor.keys[1], "https://api.example.com/v1")
	editor.setAPI(editor.keys[1], "openai-completions")
	base, api, ok := editor.askable()
	if !ok || base != "https://api.example.com/v1" || api != "openai-completions" {
		t.Fatalf("应取第一条声明了 baseURL 的行：%q/%q/%v", base, api, ok)
	}
}

func TestCatalogOpenCandidatesDedupesAndReportsEmpty(t *testing.T) {
	editor := newRestEditor(t, true)
	name := "One"
	window := int64(200000)
	maxOut := int64(8192)
	if !editor.openCandidates([]apiclient.DiscoveredModel{
		{ID: "m-1", Name: &name, ContextWindow: &window, MaxTokens: &maxOut, InputModalities: []string{"text", "image"}},
		{ID: "m-1"}, // 同一 id 只留一条（官方 Map 去重，ModelListEditor.tsx:243-245）
		{ID: "m-2"},
	}) {
		t.Fatalf("有候选时应打开挑模型的面")
	}
	if len(editor.candidates) != 2 {
		t.Fatalf("候选应按 id 去重：%d", len(editor.candidates))
	}
	// 已配置的行按默认勾选（列表里已有 alpha/beta，候选是 m-1/m-2 → 都不勾）。
	if editor.picker == nil || editor.picker.Picked("m-1") {
		t.Fatalf("列表非空时默认只勾已配置的 id")
	}
	// 端点在没报的字段上必须留空（未知 ≠ 0）。
	if _, present := editor.candidates[1]["contextWindow"]; present {
		t.Fatalf("候选没报 contextWindow 就不该写这个键")
	}
	if got := editor.candidates[0]["contextWindow"]; got != float64(window) {
		t.Fatalf("报了的容量应原样带进候选：%v", got)
	}

	// 空列表 → 官方 `fetchEmpty`，且**不打开**面（ModelListEditor.tsx:230-233）。
	fresh := newRestEditor(t, true)
	if fresh.openCandidates(nil) {
		t.Fatalf("上游一个模型都没列时不该开面")
	}
	if fresh.discoverError != catalogCopy("fetchEmpty") {
		t.Fatalf("应显示官方 fetchEmpty：%q", fresh.discoverError)
	}
	if fresh.picker != nil || fresh.candidates != nil {
		t.Fatalf("空列表不该留下面或候选")
	}
}

func TestCatalogAdoptPickedAddsCheckedAndRefusesToEmptyTheList(t *testing.T) {
	editor := newRestEditor(t, true)
	editor.openCandidates([]apiclient.DiscoveredModel{{ID: "alpha"}, {ID: "m-9"}})
	editor.picker.Toggle("alpha") // 取消勾选 alpha → 它要被删掉
	editor.picker.Toggle("m-9")   // 勾上 m-9 → 它要被加进来
	adoption, ok := editor.adoptPicked()
	if !ok {
		t.Fatalf("至少留一条时应允许更新列表：%+v", adoption)
	}
	ids := editor.idsForWrite()
	if len(ids) != 2 || ids[0] != "beta" || ids[1] != "m-9" {
		t.Fatalf("更新列表应删掉取消勾选的 alpha 并追加 m-9，实际 %v", ids)
	}
	if editor.picker != nil || editor.candidates != nil {
		t.Fatalf("结算后应关掉挑模型的面")
	}

	// 把一个非空列表全部取消勾选 → 官方拒绝（那会回到"服务整份目录"，
	// `fetchKeepOne`，ModelListEditor.tsx:266-268/:284-290），草稿一个字节都不动。
	blocked := newRestEditor(t, true)
	blocked.openCandidates([]apiclient.DiscoveredModel{{ID: "alpha"}, {ID: "beta"}})
	// 列表里两条都"已配置"，默认全勾 → 全部翻转。
	blocked.picker.Toggle("alpha")
	blocked.picker.Toggle("beta")
	before := blocked.idsForWrite()
	if _, ok := blocked.adoptPicked(); ok {
		t.Fatalf("清空非空列表必须被拒")
	}
	if blocked.discoverError != catalogCopy("fetchKeepOne") {
		t.Fatalf("应显示官方 fetchKeepOne：%q", blocked.discoverError)
	}
	if got := blocked.idsForWrite(); len(got) != len(before) {
		t.Fatalf("被拒时草稿一个字节都不许动：%v → %v", before, got)
	}
}
