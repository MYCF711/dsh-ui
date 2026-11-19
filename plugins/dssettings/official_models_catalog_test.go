// official_models_catalog_test.go —— 模型目录编辑器语义层的判据测试。
//
// 每条断言都对着官方原件的一个 file:line（注释里写明）。这里刻意**不测绘制**：
// 本片只交语义，绘制与写路径缺 GUI 仓的 models.catalog.* 接线（见报告）。
package dssettings

import (
	"encoding/json"
	"math"
	"testing"
)

// TestParseModelCapacityThreeStates 钉住官方 parseCapacity 的三态
// （DeepSeekModelsEditor.tsx:36-46）：
//
//	空串   → 继承，不写值
//	不可读 → 保留文本，保存期点名
//	可读   → 数值
func TestParseModelCapacityThreeStates(t *testing.T) {
	cases := []struct {
		text   string
		want   float64
		blank  bool
		broken bool
	}{
		{text: "", blank: true},
		{text: "   ", blank: true},
		{text: "131072", want: 131072},
		{text: "256K", want: 256_000},
		{text: "1M", want: 1_000_000},
		{text: " 1M ", want: 1_000_000},
		{text: "1k", want: 1_000},
		{text: "1m", want: 1_000_000},
		{text: "1.5K", want: 1_500},
		{text: "2.3M", want: 2_300_000},
		{text: "131072.5", want: 131072.5},
		// 十进制进制：若把 K 当 1024，下面这条立刻红（官方 :25-26）。
		{text: "1K", want: 1000},
		// 官方正则不接受空格分段、负号、指数、复合后缀。
		{text: "1 M", broken: true},
		{text: "-5", broken: true},
		{text: "1e6", broken: true},
		{text: "1MB", broken: true},
		{text: "K", broken: true},
	}
	for _, testCase := range cases {
		got := ParseModelCapacity(testCase.text)
		if got.Blank != testCase.blank || got.Unreadable != testCase.broken {
			t.Fatalf("ParseModelCapacity(%q) 三态错：blank=%v unreadable=%v，期望 blank=%v unreadable=%v",
				testCase.text, got.Blank, got.Unreadable, testCase.blank, testCase.broken)
		}
		if testCase.blank || testCase.broken {
			continue
		}
		if math.Abs(got.Value-testCase.want) > modelCapacitySnap {
			t.Fatalf("ParseModelCapacity(%q) = %v，期望 %v", testCase.text, got.Value, testCase.want)
		}
	}
}

// TestParseModelCapacitySnapsIntegralIntent 钉住官方"整数意图吸附"
// （DeepSeekModelsEditor.tsx:42-46）：乘完后离整数只差几个 ULP 时取整数，
// 而真的差得多时一律保留原值。两条必须同时成立，否则"吸附"就退化成四舍五入。
func TestParseModelCapacitySnapsIntegralIntent(t *testing.T) {
	// 8.000000000000001 * 1e6 在 float64 里是 8000000.0000000019（差 1.9e-9 < 1e-6）。
	if got := ParseModelCapacity("8.000000000000001M").Value; got != 8_000_000 {
		t.Fatalf("8.000000000000001M 应吸附成 8000000，得到 %v", got)
	}
	// 反例：差 1e-5 已经超出吸附窗，必须保留原值（不能被四舍五入成整数）。
	if got := ParseModelCapacity("1234.56700000001M").Value; got == math.Trunc(got) {
		t.Fatalf("1234.56700000001M 超出吸附窗，不该变成整数，得到 %v", got)
	}
	// 反例：1.0001K 是 1000.1，也不能被吸附。
	if got := ParseModelCapacity("1.0001K").Value; got == math.Trunc(got) {
		t.Fatalf("1.0001K 不该被吸附成整数，得到 %v", got)
	}
}

// TestFormatModelCapacity 钉住官方回显（DeepSeekModelsEditor.tsx:56-61）：
// 整除 1e6 → M、整除 1000 → K、其余原样、非正数或非整数一律原样。
func TestFormatModelCapacity(t *testing.T) {
	cases := map[float64]string{
		1_000_000: "1M",
		2_500_000: "2500K", // 不整除 1e6，但整除 1000
		256_000:   "256K",
		1_000:     "1K",
		131_072:   "131072",
		1_500:     "1500",
		0:         "0",
		-5:        "-5",
		1.5:       "1.5",
	}
	for value, want := range cases {
		if got := FormatModelCapacity(value); got != want {
			t.Fatalf("FormatModelCapacity(%v) = %q，期望 %q", value, got, want)
		}
	}
}

// TestCapacityRoundTrip 钉住"回显能被原样读回"这条不变量：官方两个函数是一对
// （ModelListEditor.tsx:127-136 注释说明两个界面共用同一套 K/M 词汇）。
func TestCapacityRoundTrip(t *testing.T) {
	for _, value := range []float64{1_000, 256_000, 1_000_000, 2_500_000, 131_072, 8_192, 1_500} {
		text := FormatModelCapacity(value)
		back := ParseModelCapacity(text)
		if back.Blank || back.Unreadable || back.Value != value {
			t.Fatalf("%v → %q → %+v，不能原样读回", value, text, back)
		}
	}
}

// TestValidateModelDrafts 覆盖官方五条约束（DeepSeekModelsEditor.tsx:86-115）。
func TestValidateModelDrafts(t *testing.T) {
	cases := []struct {
		name   string
		models []ModelDraft
		want   *ModelFailure
	}{
		{name: "空列表合法", models: nil},
		{name: "只有 ID", models: []ModelDraft{{"id": "a"}}},
		{
			name:   "ID 缺失",
			models: []ModelDraft{{"id": "a"}, {"name": "无名"}},
			want:   &ModelFailure{Index: 1, Key: ModelFailureIDRequired},
		},
		{
			name:   "ID 只有空白",
			models: []ModelDraft{{"id": "   "}},
			want:   &ModelFailure{Index: 0, Key: ModelFailureIDRequired},
		},
		{
			name:   "ID 不是字符串",
			models: []ModelDraft{{"id": 42}},
			want:   &ModelFailure{Index: 0, Key: ModelFailureIDRequired},
		},
		{
			// 官方 :91-93：比的是 trim 后的值，否则 "model " 能绕过与自己的重复检查。
			name:   "trim 后重复也算重复",
			models: []ModelDraft{{"id": "model"}, {"id": " model "}},
			want:   &ModelFailure{Index: 1, Key: ModelFailureIDDuplicate},
		},
		{
			name:   "显示名称空串非法",
			models: []ModelDraft{{"id": "a", "name": ""}},
			want:   &ModelFailure{Index: 0, Key: ModelFailureNameInvalid},
		},
		{
			name:   "显示名称非字符串非法",
			models: []ModelDraft{{"id": "a", "name": 7}},
			want:   &ModelFailure{Index: 0, Key: ModelFailureNameInvalid},
		},
		{
			name:   "上下文窗口为 0 非法",
			models: []ModelDraft{{"id": "a", "contextWindow": 0}},
			want:   &ModelFailure{Index: 0, Key: ModelFailureContextInvalid},
		},
		{
			name:   "上下文窗口为负非法",
			models: []ModelDraft{{"id": "a", "contextWindow": -1}},
			want:   &ModelFailure{Index: 0, Key: ModelFailureContextInvalid},
		},
		{
			// 官方 typeof number：字符串 "131072" 不是数字。
			name:   "上下文窗口字符串非法",
			models: []ModelDraft{{"id": "a", "contextWindow": "131072"}},
			want:   &ModelFailure{Index: 0, Key: ModelFailureContextInvalid},
		},
		{
			name:   "上下文窗口非整数非法",
			models: []ModelDraft{{"id": "a", "contextWindow": 1.5}},
			want:   &ModelFailure{Index: 0, Key: ModelFailureContextInvalid},
		},
		{
			name:   "最大输出 token 为 0 非法",
			models: []ModelDraft{{"id": "a", "maxTokens": 0}},
			want:   &ModelFailure{Index: 0, Key: ModelFailureMaxTokensInvalid},
		},
		{
			name:   "JSON 数字形态的整数合法",
			models: []ModelDraft{{"id": "a", "contextWindow": json.Number("131072"), "maxTokens": json.Number("8192")}},
		},
		{
			name:   "JSON 数字形态的小数非法",
			models: []ModelDraft{{"id": "a", "maxTokens": json.Number("8.5")}},
			want:   &ModelFailure{Index: 0, Key: ModelFailureMaxTokensInvalid},
		},
	}
	for _, testCase := range cases {
		got := ValidateModelDrafts(testCase.models)
		if testCase.want == nil {
			if got != nil {
				t.Fatalf("%s：期望合法，得到 %+v", testCase.name, *got)
			}
			continue
		}
		if got == nil {
			t.Fatalf("%s：期望 %+v，得到合法", testCase.name, *testCase.want)
		}
		if got.Index != testCase.want.Index || got.Key != testCase.want.Key {
			t.Fatalf("%s：得到 %+v，期望 %+v", testCase.name, *got, *testCase.want)
		}
	}
}

// TestPatchModelDraftClearsOptionalFields 钉住官方 patch 的"清空即删键"
// （ModelListEditor.tsx:192-207）：清空的可选字段必须离开 profile，
// 而本卡不编辑的字段要存活。
func TestPatchModelDraftClearsOptionalFields(t *testing.T) {
	model := ModelDraft{"id": "a", "name": "A", "baseURL": "https://x", "contextWindow": 1000}
	got := PatchModelDraft(model, map[string]any{"name": "", "contextWindow": nil})
	if _, present := got["name"]; present {
		t.Fatalf("空串必须删键，得到 %v", got)
	}
	if _, present := got["contextWindow"]; present {
		t.Fatalf("nil 必须删键，得到 %v", got)
	}
	if got["baseURL"] != "https://x" {
		t.Fatalf("本卡不编辑的字段必须存活，得到 %v", got)
	}
	if got["id"] != "a" {
		t.Fatalf("未提到的字段必须保留，得到 %v", got)
	}
	// 原草稿不能被就地改掉（官方 map 出新对象）。
	if _, present := model["name"]; !present {
		t.Fatalf("不能修改传入的草稿，得到 %v", model)
	}
}

// TestModelDropTarget 钉住官方 onDrop 的插入位换算（useModelRows.ts:83-88）。
func TestModelDropTarget(t *testing.T) {
	cases := []struct {
		from, target int
		after        bool
		want         int
		ok           bool
	}{
		{from: 0, target: 2, after: false, want: 1, ok: true},
		{from: 0, target: 2, after: true, want: 2, ok: true},
		{from: 3, target: 1, after: false, want: 1, ok: true},
		// 落回原位：不动。
		{from: 1, target: 1, after: false, want: 1, ok: false},
		{from: 1, target: 0, after: true, want: 1, ok: false},
	}
	for _, testCase := range cases {
		got, ok := ModelDropTarget(testCase.from, testCase.target, testCase.after)
		if ok != testCase.ok || got != testCase.want {
			t.Fatalf("ModelDropTarget(from=%d,target=%d,after=%v) = (%d,%v)，期望 (%d,%v)",
				testCase.from, testCase.target, testCase.after, got, ok, testCase.want, testCase.ok)
		}
	}
}

// TestModelRowAfterHalf 钉住官方 rowHalf（ModelRow.tsx:46-49）。
func TestModelRowAfterHalf(t *testing.T) {
	if ModelRowAfterHalf(99, 100, 40) {
		t.Fatalf("中线以上应判 before")
	}
	if !ModelRowAfterHalf(120, 100, 40) {
		t.Fatalf("中线以下应判 after")
	}
	if !ModelRowAfterHalf(120, 100, 40) || ModelRowAfterHalf(119.99, 100, 40) {
		t.Fatalf("中线 jitter 判定错")
	}
}

// TestMoveModelDraft 钉住官方 move 的四个早退与 splice 语义（useModelRows.ts:60-67）。
func TestMoveModelDraft(t *testing.T) {
	models := []ModelDraft{{"id": "a"}, {"id": "b"}, {"id": "c"}}
	moved := MoveModelDraft(models, 0, 2)
	if ids := draftIDs(moved); ids != "b,c,a" {
		t.Fatalf("移动 0→2 得到 %s，期望 b,c,a", ids)
	}
	if ids := draftIDs(MoveModelDraft(models, 2, 0)); ids != "c,a,b" {
		t.Fatalf("移动 2→0 得到 %s，期望 c,a,b", ids)
	}
	if ids := draftIDs(MoveModelDraft(models, 1, 1)); ids != "a,b,c" {
		t.Fatalf("原地移动不该改变次序，得到 %s", ids)
	}
	if ids := draftIDs(MoveModelDraft(models, 0, 3)); ids != "a,b,c" {
		t.Fatalf("越界目标必须早退，得到 %s", ids)
	}
	if ids := draftIDs(models); ids != "a,b,c" {
		t.Fatalf("不能就地改传入切片，得到 %s", ids)
	}
	if ModelRowMove(true, 0, 2, 3) {
		t.Fatalf("不可编辑时必须早退")
	}
}

// draftIDs 把草稿列表拼成 "a,b,c" 便于断言次序。
func draftIDs(models []ModelDraft) string {
	out := ""
	for index, model := range models {
		if index > 0 {
			out += ","
		}
		out += ModelText(model, "id")
	}
	return out
}

// TestModelPickerDefaults 钉住官方"默认勾选"（ModelListEditor.tsx:244-249）：
// 列表里一个已识别模型都没有时全勾，否则只勾已配置的。
func TestModelPickerDefaults(t *testing.T) {
	empty := NewModelPicker(nil)
	if !empty.PickedByDefault("anything") || !empty.Picked("anything") {
		t.Fatalf("空列表必须默认全勾")
	}
	configured := NewModelPicker([]ModelDraft{{"id": "a"}, {"id": ""}})
	if !configured.PickedByDefault("a") || configured.PickedByDefault("b") {
		t.Fatalf("已配置列表只勾 a")
	}
	// 半截行（id 为空）不参与"已识别"。
	if len(configured.configured) != 1 {
		t.Fatalf("空 id 不能算已配置，得到 %v", configured.configured)
	}
}

// TestModelPickerToggleAndVisible 钉住官方只翻转可见项（ModelListEditor.tsx:292-304）。
func TestModelPickerToggleAndVisible(t *testing.T) {
	picker := NewModelPicker([]ModelDraft{{"id": "a"}, {"id": "b"}, {"id": "hidden"}})
	// 只可见 a、b：全勾后取消它们，隐藏的 hidden 必须仍保留勾选。
	if !picker.Picked("hidden") {
		t.Fatalf("已配置项默认勾选")
	}
	picker.ToggleVisible([]string{"a", "b"})
	if picker.Picked("a") || picker.Picked("b") {
		t.Fatalf("可见项应被取消勾选")
	}
	if !picker.Picked("hidden") {
		t.Fatalf("隐藏项不能被连带取消")
	}
	// 再点回来。
	picker.ToggleVisible([]string{"a", "b"})
	if !picker.Picked("a") || !picker.Picked("b") {
		t.Fatalf("第二次应全勾")
	}
	picker.Toggle("a")
	if picker.Picked("a") {
		t.Fatalf("单点翻转失效")
	}
}

// ToggleVisibleAndCount 只在测试里用来数翻转集大小。
func (p *ModelPicker) ToggleVisibleAndCount(ids []string) map[string]bool {
	copied := make(map[string]bool, len(p.flipped))
	for key, value := range p.flipped {
		copied[key] = value
	}
	return copied
}

// TestModelPickerAdopt 钉住官方结算（ModelListEditor.tsx:251-274）。
func TestModelPickerAdopt(t *testing.T) {
	models := []ModelDraft{{"id": "a"}, {"id": "b"}, {"id": ""}, {"id": "outside"}}
	picker := NewModelPicker(models)
	// 取消 b（候选名单里有它），勾上新候选 c；a 保留，outside 不在名单里必须留下。
	picker.Toggle("b")
	picker.Toggle("c")
	adoption := picker.Adopt(models, []string{"a", "b", "c"})
	if ids := draftIDs(adoption.Kept); ids != "a,,outside" {
		t.Fatalf("留下的行错：%s", ids)
	}
	if len(adoption.Removed) != 1 || adoption.Removed[0] != 1 {
		t.Fatalf("应删掉 b（下标 1），得到 %v", adoption.Removed)
	}
	if len(adoption.AddedIDs) != 1 || adoption.AddedIDs[0] != "c" {
		t.Fatalf("应新增 c，得到 %v", adoption.AddedIDs)
	}
	if adoption.EmptiesList {
		t.Fatalf("还有行在，不该判成清空")
	}
}

// TestModelPickerAdoptRefusesEmptying 钉住官方"不许清空"（ModelListEditor.tsx:266-268）。
func TestModelPickerAdoptRefusesEmptying(t *testing.T) {
	models := []ModelDraft{{"id": "a"}}
	picker := NewModelPicker(models)
	picker.Toggle("a")
	adoption := picker.Adopt(models, []string{"a"})
	if !adoption.EmptiesList {
		t.Fatalf("取消唯一一行并无可加项时必须判成清空，得到 %+v", adoption)
	}
	// 有新增项时不算清空。
	picker = NewModelPicker(models)
	picker.Toggle("a")
	picker.Toggle("b")
	if picker.Adopt(models, []string{"a", "b"}).EmptiesList {
		t.Fatalf("有新增项时不该判成清空")
	}
}

// TestModelSelectedInputs 钉住官方输入类型回退（ModelInputTypes.tsx:33-34）。
func TestModelSelectedInputs(t *testing.T) {
	if got := ModelSelectedInputs(ModelDraft{}, "input", nil); len(got) != 1 || got[0] != ModelInputText {
		t.Fatalf("无声明无回退时必须是 text，得到 %v", got)
	}
	if got := ModelSelectedInputs(ModelDraft{"input": []any{"text", "image"}}, "input", []string{"image"}); len(got) != 2 {
		t.Fatalf("行内声明优先，得到 %v", got)
	}
	if got := ModelSelectedInputs(ModelDraft{"input": []any{}}, "input", []string{"image"}); len(got) != 1 || got[0] != "image" {
		t.Fatalf("空数组走回退，得到 %v", got)
	}
	if got := ModelSelectedInputs(ModelDraft{"input": []string{"image"}}, "input", nil); len(got) != 1 || got[0] != "image" {
		t.Fatalf("[]string 声明也要认，得到 %v", got)
	}
}

// TestToggleModelInputType 钉住官方两个语义（ModelInputTypes.tsx:44-55）：
// 至少留一项；DeepSeek 去图时连带删两个图片预算字段。
func TestToggleModelInputType(t *testing.T) {
	// 只剩 text 时不能取消。
	model := ModelDraft{"id": "a", "inputModalities": []any{"text"}}
	if _, ok := ToggleModelInputType(model, "inputModalities", ModelInputText, false, nil); ok {
		t.Fatalf("只剩一项时必须忽略取消点击")
	}
	if !ModelInputTypeLocked([]string{ModelInputText}, ModelInputText) {
		t.Fatalf("单项应为锁定态")
	}
	// 勾上 image：顺序固定为 text,image，两个预算字段保留。
	withImage := ModelDraft{"id": "a", "inputModalities": []any{"text"},
		"imagePixelBudget": 1000, "imageMaxBytes": 2000}
	toggled, ok := ToggleModelInputType(withImage, "inputModalities", ModelInputImage, true, nil)
	if !ok {
		t.Fatalf("加 image 不该被拒")
	}
	if selected := ModelSelectedInputs(toggled, "inputModalities", nil); len(selected) != 2 || selected[0] != "text" || selected[1] != "image" {
		t.Fatalf("次序必须是 text,image，得到 %v", selected)
	}
	if toggled["imagePixelBudget"] != 1000 || toggled["imageMaxBytes"] != 2000 {
		t.Fatalf("还有 image 时预算字段必须保留，得到 %v", toggled)
	}
	// 再去掉 image：两个预算字段必须一起消失（DeepSeek 拒绝文本模型的图片限额）。
	back, ok := ToggleModelInputType(toggled, "inputModalities", ModelInputImage, false, nil)
	if !ok {
		t.Fatalf("去 image 不该被拒")
	}
	if _, present := back["imagePixelBudget"]; present {
		t.Fatalf("去图必须删 imagePixelBudget，得到 %v", back)
	}
	if _, present := back["imageMaxBytes"]; present {
		t.Fatalf("去图必须删 imageMaxBytes，得到 %v", back)
	}
	// pi-ai 一侧（字段 input）不做这条连带删除。
	piModel := ModelDraft{"id": "a", "input": []any{"text", "image"}}
	piBack, ok := ToggleModelInputType(piModel, "input", ModelInputImage, false, nil)
	if !ok {
		t.Fatalf("pi-ai 去图不该被拒")
	}
	if len(ModelSelectedInputs(piBack, "input", nil)) != 1 {
		t.Fatalf("pi-ai 去图后只剩 text，得到 %v", piBack)
	}
	// 原草稿不被就地改。
	if len(ModelSelectedInputs(withImage, "inputModalities", nil)) != 1 {
		t.Fatalf("不能修改传入草稿，得到 %v", withImage)
	}
}

// TestModelEditorCopy 钉住文案逐字取官方 locales.ts 的中文段（:203-257）。
func TestModelEditorCopy(t *testing.T) {
	cases := map[string]string{
		"models":                "模型目录",
		"modelsInherited":       "正在使用适配器默认模型",
		"modelsCustomized":      "已自定义模型目录",
		"resetModels":           "恢复默认模型",
		"addModel":              "添加模型",
		"removeModel":           "删除模型",
		"reorderModelHelp":      "拖动排序，或使用上下方向键调整顺序。",
		"modelsEmpty":           "模型选择器中将不显示任何模型；目录外 ID 仍可直接发送。",
		"modelIdDuplicate":      "模型 ID 不能重复。",
		"modelMaxTokensInvalid": "最大输出 token 数必须是正数，例如 8192、64K 或 1M。",
		"fetchAdopt":            "更新列表",
		"fetchKeepOne":          "请至少保留一个勾选的模型。",
	}
	for key, want := range cases {
		if got := ModelEditorCopy(key); got != want {
			t.Fatalf("ModelEditorCopy(%q) = %q，期望 %q", key, got, want)
		}
	}
	if got := ModelEditorCopy("no-such-key"); got != "" {
		t.Fatalf("未登记的键必须返回空串（不画键名），得到 %q", got)
	}
}

// TestModelCapacityHint 钉住官方 pi-ai 的占位文案（ModelListEditor.tsx:122-125）。
func TestModelCapacityHint(t *testing.T) {
	if got := ModelCapacityHint("contextWindow"); got != "1M" {
		t.Fatalf("contextWindow 占位应为 1M，得到 %q", got)
	}
	if got := ModelCapacityHint("maxTokens"); got != "128K" {
		t.Fatalf("maxTokens 占位应为 128K，得到 %q", got)
	}
}
