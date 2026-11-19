// official_models_catalog_editor_test.go —— 模型目录编辑器的**语义与写形状**测试。
//
// 断言的是三件事：
//
//	① 快照 → 草稿的灌入（行身份、顺序、推理档位顺序、raw revision）；
//	② 行内改动的语义逐条来自 #178 的语义层（容量三态、清空即删键、五条校验、
//	   推理等级两条），**这里不另造一套判断**；
//	③ 写出去的是宿主 `models.catalog.upsert` 认的 model JSON：id 已 trim、
//	   未识别字段与 retryPolicy 原样存活、推理档位按编辑器顺序。
package dssettings

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// catalogFixture 造一份宿主目录快照（revision 7，两条模型）。
func catalogFixture(t *testing.T) *apiclient.ModelCatalogSnapshot {
	t.Helper()
	document := apiclient.ModelCatalogDocument{
		Revision: 7,
		Models: []json.RawMessage{
			json.RawMessage(`{"id":"alpha","name":"阿尔法","contextWindow":131072,"input":["text"],"retryPolicy":{"mode":"normal"},"futureField":{"keep":true},"reasoningEfforts":{"off":null,"low":"low"}}`),
			json.RawMessage(`{"id":"beta"}`),
		},
		Active: apiclient.ModelCatalogActive{ID: "alpha", Revision: 3},
	}
	return &apiclient.ModelCatalogSnapshot{Document: &document}
}

func newCatalogEditorForTest(t *testing.T) *catalogEditor {
	t.Helper()
	return newCatalogEditor(catalogFixture(t), true)
}

// ① 灌入：行身份、顺序、档位顺序、revision。
func TestCatalogEditorSeedsDraftsKeysEffortsAndRevision(t *testing.T) {
	editor := newCatalogEditorForTest(t)
	if editor.revision == nil || *editor.revision != 7 {
		t.Fatalf("revision 应灌成 7，得到 %v", editor.revision)
	}
	if len(editor.drafts) != 2 || len(editor.keys) != 2 {
		t.Fatalf("应灌入两行：drafts=%d keys=%d", len(editor.drafts), len(editor.keys))
	}
	if ModelText(editor.drafts[0], "id") != "alpha" || ModelText(editor.drafts[1], "id") != "beta" {
		t.Fatalf("顺序或 id 不对：%+v", editor.drafts)
	}
	if len(editor.serverIDs) != 2 || editor.serverIDs[1] != "beta" {
		t.Fatalf("serverIDs 不对：%v", editor.serverIDs)
	}
	// 推理档位必须按 JSON 原文顺序（官方 Object.entries，ModelReasoningEfforts.tsx:43）。
	efforts := editor.efforts[editor.keys[0]]
	if len(efforts) != 2 || efforts[0].Level != "off" || efforts[0].Value != nil {
		t.Fatalf("档位顺序/off=null 不对：%+v", efforts)
	}
	if efforts[1].Level != "low" || efforts[1].Value != "low" {
		t.Fatalf("档位值不对：%+v", efforts[1])
	}
	if editor.reasoningOn(editor.keys[1]) {
		t.Fatalf("beta 没有 reasoningEfforts，不该算打开")
	}
}

// ② 容量三态（#178 ParseModelCapacity，官方 DeepSeekModelsEditor.tsx:35-47）。
func TestCatalogCapacityThreeStatesReachTheDraft(t *testing.T) {
	editor := newCatalogEditorForTest(t)
	p := &Plugin{}
	o := p.ensureOfficial()
	o.modelsCatalog = editor
	key := editor.keys[1]

	p.catalogSetCapacity(key, "contextWindow", "256K")
	if value, ok := modelCapacityNumber(editor.drafts[1]["contextWindow"]); !ok || value != 256000 {
		t.Fatalf("256K 应写成十进制 256000（1M=1000K），得到 %v", editor.drafts[1]["contextWindow"])
	}

	// 空串 = 继承：字段必须**离开草稿**，不是写 0 或写空串。
	p.catalogSetCapacity(key, "contextWindow", "   ")
	if _, present := editor.drafts[1]["contextWindow"]; present {
		t.Fatalf("空串必须删键：%+v", editor.drafts[1])
	}

	// 读不出 = 屏上文本保留、保存期由校验点名（官方把 NaN 放进草稿）。
	p.catalogSetCapacity(key, "maxTokens", "很多")
	count, ok := modelCapacityNumber(editor.drafts[1]["maxTokens"])
	if ok || !math.IsNaN(editor.drafts[1]["maxTokens"].(float64)) {
		t.Fatalf("读不出应写成 NaN：%v / %v", count, editor.drafts[1]["maxTokens"])
	}
	failure := editor.validate()
	if failure == nil || failure.Key != ModelFailureMaxTokensInvalid || failure.Index != 1 {
		t.Fatalf("NaN 容量应报 modelMaxTokensInvalid@1：%+v", failure)
	}
}

// ② 清空名称 = 删键（官方 ModelRow.tsx:74）。
func TestCatalogEmptyNameLeavesTheDraft(t *testing.T) {
	editor := newCatalogEditorForTest(t)
	p := &Plugin{}
	o := p.ensureOfficial()
	o.modelsCatalog = editor
	p.catalogSetText(editor.keys[0], "name", "")
	if _, present := editor.drafts[0]["name"]; present {
		t.Fatalf("清空名称必须删键：%+v", editor.drafts[0])
	}
	p.catalogSetText(editor.keys[0], "name", "甲")
	if ModelText(editor.drafts[0], "name") != "甲" {
		t.Fatalf("名称没写进去：%+v", editor.drafts[0])
	}
}

// ② 五条行校验 + 推理等级两条（#178 ValidateModelDrafts / 官方 validateReasoningEfforts）。
func TestCatalogValidationNamesTheOfficialFailures(t *testing.T) {
	cases := []struct {
		name    string
		drafts  []ModelDraft
		efforts map[string][]catalogEffort
		want    string
		index   int
	}{
		{"id 必填", []ModelDraft{{"id": "  "}, {"id": "b"}}, nil, ModelFailureIDRequired, 0},
		// 比的是 trim 后的值：`model ` 不能绕过与自己的重复检查（:91-93）。
		{"id 重复", []ModelDraft{{"id": "m"}, {"id": " m "}}, nil, ModelFailureIDDuplicate, 1},
		{"名称为空串", []ModelDraft{{"id": "m", "name": ""}}, nil, ModelFailureNameInvalid, 0},
		{"容量非整数", []ModelDraft{{"id": "m", "contextWindow": 1.5}}, nil, ModelFailureContextInvalid, 0},
		{"容量非正", []ModelDraft{{"id": "m", "maxTokens": 0.0}}, nil, ModelFailureMaxTokensInvalid, 0},
		{"档位缺值", []ModelDraft{{"id": "m"}}, map[string][]catalogEffort{"m1": {{Level: "off", Value: nil}}}, ModelFailureReasoningNeedsLevel, 0},
		{"档位值为空", []ModelDraft{{"id": "m"}},
			map[string][]catalogEffort{"m1": {{Level: "off", Value: nil}, {Level: "low", Value: ""}}}, ModelFailureReasoningValueRequired, 0},
	}
	for _, testCase := range cases {
		editor := &catalogEditor{drafts: testCase.drafts, keys: make([]string, len(testCase.drafts)), efforts: testCase.efforts}
		for index := range editor.keys {
			editor.keys[index] = "m" + string(rune('1'+index))
		}
		failure := editor.validate()
		if failure == nil || failure.Key != testCase.want || failure.Index != testCase.index {
			t.Fatalf("%s：期望 %s@%d，得到 %+v", testCase.name, testCase.want, testCase.index, failure)
		}
	}
}

// ③ 写出去的 model JSON：id trim、未知字段与 retryPolicy 存活、档位按编辑器顺序。
func TestCatalogModelPayloadKeepsUnknownFieldsAndOrder(t *testing.T) {
	editor := newCatalogEditorForTest(t)
	p := &Plugin{}
	o := p.ensureOfficial()
	o.modelsCatalog = editor
	// 用户把 id 打成了带空格的样子（官方 onIdBlur 也 trim）。
	p.catalogSetText(editor.keys[0], "id", "  alpha  ")
	payload, err := editor.modelPayload(0)
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("不是合法 JSON：%v", err)
	}
	var id string
	if err := json.Unmarshal(decoded["id"], &id); err != nil || id != "alpha" {
		t.Fatalf("id 必须 trim 后写出去（宿主 validateCatalogID 要求首尾无空白），得到 %q", id)
	}
	if _, ok := decoded["futureField"]; !ok {
		t.Fatalf("未识别字段必须原样存活（宿主 Upsert 会整条写回）：%s", payload)
	}
	if _, ok := decoded["retryPolicy"]; !ok {
		t.Fatalf("本卡不编辑的 retryPolicy 必须存活：%s", payload)
	}
	// 档位顺序：off 在前（JSON 原文顺序），不是 Go map 的键序。
	efforts := string(decoded["reasoningEfforts"])
	if efforts != `{"off":null,"low":"low"}` {
		t.Fatalf("推理档位顺序/形状不对：%s", efforts)
	}
	// 容量 131072 是 json.Number，回写不能变成科学计数法。
	if string(decoded["contextWindow"]) != "131072" {
		t.Fatalf("容量回写形状不对：%s", decoded["contextWindow"])
	}
}

// ⑨ 移动一行：行身份与档位表跟着一起挪（官方 useModelRows 的 rowKey 语义）。
func TestCatalogMoveRowKeepsRowIdentity(t *testing.T) {
	editor := newCatalogEditorForTest(t)
	first := editor.keys[0]
	if !editor.moveRow(0, 1) {
		t.Fatalf("0→1 应当可移动")
	}
	if editor.keys[1] != first {
		t.Fatalf("行身份应跟着模型挪：keys=%v", editor.keys)
	}
	if ModelText(editor.drafts[1], "id") != "alpha" {
		t.Fatalf("草稿顺序不对：%+v", editor.drafts)
	}
	if len(editor.efforts[first]) != 2 {
		t.Fatalf("档位表必须跟行身份走：%+v", editor.efforts)
	}
	// 越界与原地都是空操作（官方四个早退条件，#178 ModelRowMove）。
	if editor.moveRow(1, 5) || editor.moveRow(0, 0) {
		t.Fatalf("越界/原地不该移动")
	}
}

// ③ 顺序判断与 reorder 前提：数量对不上时**不发** reorder。
func TestCatalogOrderChangedAndSameIDSet(t *testing.T) {
	editor := newCatalogEditorForTest(t)
	if editor.orderChanged() {
		t.Fatalf("刚灌入时顺序未变")
	}
	editor.addModel()
	if !editor.orderChanged() {
		t.Fatalf("加了新行就是顺序变了")
	}
	// 新加的一行还不算"丢模型"：宿主那两条都还在草稿里。
	if !editor.consumesSameIDs() {
		t.Fatalf("宿主两条都还在草稿里，应算没丢模型")
	}
	editor.drafts[len(editor.drafts)-1]["id"] = "gamma"
	if !editor.consumesSameIDs() {
		t.Fatalf("加了一条新 id 之后，宿主两条仍在，应算没丢模型")
	}
	// 真的丢掉一条（将来宿主有 delete 才会出现的草稿）：不许发 reorder。
	editor.drafts = editor.drafts[:1]
	if editor.consumesSameIDs() {
		t.Fatalf("少了一条宿主模型就不能算同集合（reorder 会 bad_request）")
	}
}

// 推理等级：打开写官方那组初始档位，关闭删字段（ModelReasoningEfforts.tsx:157,171-176）。
func TestCatalogReasoningToggleAndEfforts(t *testing.T) {
	editor := newCatalogEditorForTest(t)
	key := editor.keys[1]
	editor.setReasoning(key, true)
	entries := editor.efforts[key]
	if len(entries) != len(catalogReasoningLevels) || entries[0].Level != "off" || entries[0].Value != nil {
		t.Fatalf("打开应按官方初始档位写：%+v", entries)
	}
	// 换档位：值还是档位名 → 重置成新档位的默认值。
	editor.setEffortValue(key, 1, "自定义")
	editor.cycleEffortLevel(key, 1)
	if editor.efforts[key][1].Value != "自定义" {
		t.Fatalf("自定义值跨档位应保留（官方 carried 语义）：%+v", editor.efforts[key])
	}
	// 删档位后，off 之外一个都不剩 → 校验点名 needsLevel。
	for index := len(editor.efforts[key]) - 1; index >= 1; index-- {
		editor.removeEffort(key, index)
	}
	if failure := editor.validate(); failure == nil || failure.Key != ModelFailureReasoningNeedsLevel {
		t.Fatalf("只剩 off 应报 modelReasoningNeedsLevel：%+v", failure)
	}
	editor.setReasoning(key, false)
	if _, ok := editor.efforts[key]; ok {
		t.Fatalf("关闭必须删掉档位表（官方 withEfforts(undefined)）")
	}
}

// 输入类型：至少留一项、固定次序、切到只有图片时也合法
// （#178 ToggleModelInputType / 官方 ModelInputTypes.tsx:39-58）。
func TestCatalogInputTypesFollowTheSemanticLayer(t *testing.T) {
	draft := ModelDraft{"id": "m"}
	selected := catalogSelectedInputs(draft)
	if len(selected) != 1 || selected[0] != ModelInputText {
		t.Fatalf("没有声明时回退 ['text']：%v", selected)
	}
	next, ok := toggleCatalogInput(draft, ModelInputImage, true)
	if !ok || len(catalogSelectedInputs(next)) != 2 {
		t.Fatalf("加图片应得到两项：%v", catalogSelectedInputs(next))
	}
	// 剩一项且已选中时官方忽略这次点击（ModelInputText locked）。
	next, ok = toggleCatalogInput(next, ModelInputText, false)
	if !ok || len(catalogSelectedInputs(next)) != 1 || catalogSelectedInputs(next)[0] != ModelInputImage {
		t.Fatalf("去掉文本应只剩图片：%v", catalogSelectedInputs(next))
	}
	if _, ok := toggleCatalogInput(next, ModelInputImage, false); ok {
		t.Fatalf("只剩一项且已选中时不能取消")
	}
}
