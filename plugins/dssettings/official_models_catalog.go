// official_models_catalog.go —— 官方模型目录编辑器的**语义层**（纯函数）。
//
// 官方原件的两个编辑器（rc.2 随 asar 发布 TS）：
//
//	dsh-client-ui-settings-models/src/client/ModelListEditor.tsx:1-454   pi-ai 手写路由
//	dsh-client-ui-settings-models/src/client/DeepSeekModelsEditor.tsx:1-256  DeepSeek 适配器
//
// 二者互不包含，是同一张 provider 卡里的二叉择一（ProviderEditor.tsx:48-68），但共用
// 同一份行基础件（ModelRow.tsx:56）与同一套容量词汇（formatCapacity/parseCapacity 由
// DeepSeek 文件导出、列表编辑器 import，ModelListEditor.tsx:25）。
//
// 本文件把这两个编辑器里**与绘制无关的判据**逐条搬成纯函数：容量三态解析/回显、
// 五条行校验、行字段 patch 的"清空即删键"语义、发现弹窗的勾选与"一个都不剩"判据、
// 拖放的落点与移动、输入类型"至少留一项"与 DeepSeek 去图连带删预算字段。
//
// 为什么先搬语义、不先画：这些判据是编辑器唯一的正确性来源，画的时候一条都不许自造
// （例如容量进制是十进制 1M=1000K、trim 后比重复、清空的可选字段必须**离开** profile
// 而不是写空串）。
//
// **写路径说明**：官方这两个编辑器写 provider profile 的 `models` 数组；本仓宿主
// 不发布这个字段（internal/settingsform/provider_schema.go:167 原文"baseURL/apiKeyEnv/
// models 等在宿主由模型目录承载"），等价真值是模型目录 `models.catalog.*`
// （cmd/dshd/serve_models.go）。GUI 仓目前没有该 wire 的客户端与 Actions 钩子，
// 所以本文件只有语义、没有写出口——不许拿一个落盘无人消费的 path 冒充编辑器。
package dssettings

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ModelDraft 是一行模型草稿。结构**保持开放**：本卡不编辑的字段（baseURL/api/
// apiKeyEnv/retryPolicy…）必须随行存活，不能被重建掉。
// 官方 `export type DeepSeekModelDraft = Record<string, unknown>`（DeepSeekModelsEditor.tsx:17）。
type ModelDraft map[string]any

// modelCapacityPattern 是官方接受的容量写法：十进制计数 + 可选 K/M 后缀，大小写不敏感
// （DeepSeekModelsEditor.tsx:23 的 `/([km])/i`——`1m` 与 `1M` 同义，所以要 `(?i)`）。
var modelCapacityPattern = regexp.MustCompile(`(?i)^(\d+(?:\.\d+)?)([km])?$`)

// 十进制后缀：官方注释写明 `1M` 就是 1000K，与模型容量的通行写法一致
// （DeepSeekModelsEditor.tsx:25-26）。不是 1024 进制。
const (
	modelCapacityK = 1_000.0
	modelCapacityM = 1_000_000.0
)

// modelCapacitySnap 是官方"整数意图吸附"的阈值：`2.3 * 1e6` 在二进制浮点里会高出几个
// ULP，所以离整数不到 1e-6 就取整数（DeepSeekModelsEditor.tsx:42-46）。
const modelCapacitySnap = 1e-6

// ModelCapacity 是官方 parseCapacity 的**三态**（DeepSeekModelsEditor.tsx:35-47）。
//
// 为什么必须有三种、而不是"数值 + 错误"：官方用 `undefined` 与 `NaN` 表达两件不同的事，
// 而这两件事在界面上后果相反——
//
//	Blank      空串 = 继承，**不写值**（:36-37）
//	Unreadable 不可读 = 屏上文本**保留**，靠保存期校验点名（:38-39，:178-181）
type ModelCapacity struct {
	Value      float64
	Blank      bool
	Unreadable bool
}

// ParseModelCapacity 读一个用户输入的容量（DeepSeekModelsEditor.tsx:35-47）。
//
// 顺序照官方：先 trim（:36），空 → 继承；不匹配 → 不可读；否则乘后缀比例并按需吸附整数。
func ParseModelCapacity(text string) ModelCapacity {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ModelCapacity{Blank: true}
	}
	match := modelCapacityPattern.FindStringSubmatch(trimmed)
	if match == nil {
		return ModelCapacity{Unreadable: true}
	}
	scale := 1.0
	switch strings.ToLower(match[2]) {
	case "k":
		scale = modelCapacityK
	case "m":
		scale = modelCapacityM
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		// 正则已经把形状钉死，这里只可能是超出 float64 范围（官方此时得到 Infinity）。
		return ModelCapacity{Value: math.Inf(1)}
	}
	scaled := value * scale
	rounded := math.Round(scaled)
	if math.Abs(scaled-rounded) < modelCapacitySnap {
		scaled = rounded
	}
	return ModelCapacity{Value: scaled}
}

// FormatModelCapacity 把一个存量计数回显成最短的、能被 ParseModelCapacity 原样读回的写法
// （DeepSeekModelsEditor.tsx:56-61）。
//
// 非整数或 ≤0 一律原样（官方 `String(value)`），不把 0 或负数写成 "0K"。
func FormatModelCapacity(value float64) string {
	if value != math.Trunc(value) || value <= 0 || math.IsInf(value, 0) || math.IsNaN(value) {
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	if math.Mod(value, modelCapacityM) == 0 {
		return strconv.FormatFloat(value/modelCapacityM, 'f', -1, 64) + "M"
	}
	if math.Mod(value, modelCapacityK) == 0 {
		return strconv.FormatFloat(value/modelCapacityK, 'f', -1, 64) + "K"
	}
	// 已经确认是整数值，用整数形态输出，免得 131072 变成 131072.000000。
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// 五条行校验的消息键（官方 DeepSeekModelsValidationFailure.key，
// DeepSeekModelsEditor.tsx:68-69）。中文文案见 modelEditorCopy。
const (
	ModelFailureIDRequired       = "modelIdRequired"
	ModelFailureIDDuplicate      = "modelIdDuplicate"
	ModelFailureNameInvalid      = "modelNameInvalid"
	ModelFailureContextInvalid   = "modelContextInvalid"
	ModelFailureMaxTokensInvalid = "modelMaxTokensInvalid"
)

// ModelFailure 是第一条不合法的行（官方 DeepSeekModelsValidationFailure，
// DeepSeekModelsEditor.tsx:64-70）。只报**第一条**：官方也是 `return` 第一条。
type ModelFailure struct {
	Index int
	Key   string
}

// modelInteger 把草稿里的一个数值读成整数。//
// 官方的判据是 `typeof number && Number.isInteger(value) && value > 0`
// （DeepSeekModelsEditor.tsx:103-112）——字符串 "123" 在官方是**不合法**的，
// 所以这里也不接受字符串涨落；json.Number 只有整数字面量才算。
func modelInteger(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		if math.IsInf(typed, 0) || math.IsNaN(typed) || typed != math.Trunc(typed) {
			return 0, false
		}
		return typed, true
	case json.Number:
		parsed, err := strconv.ParseFloat(typed.String(), 64)
		if err != nil || parsed != math.Trunc(parsed) {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

// ValidateModelDrafts 校验序列化 schema 表达不了的那几条适配器约束
// （DeepSeekModelsEditor.tsx:86-115）。
//
// 顺序与判据逐条照官方：id trim 后非空 → 唯一（**比的是 trim 后的值**：官方注释写明
// 不 trim 会让 `model ` 绕过与自己的重复检查，:91-93）→ name 给了就不能是空串 →
// contextWindow/maxTokens 给了就必须是正整数。
func ValidateModelDrafts(models []ModelDraft) *ModelFailure {
	seen := make(map[string]bool, len(models))
	for index, model := range models {
		id, isString := model["id"].(string)
		trimmed := strings.TrimSpace(id)
		if !isString || trimmed == "" {
			return &ModelFailure{Index: index, Key: ModelFailureIDRequired}
		}
		if seen[trimmed] {
			return &ModelFailure{Index: index, Key: ModelFailureIDDuplicate}
		}
		seen[trimmed] = true
		if name, present := model["name"]; present {
			text, ok := name.(string)
			if !ok || text == "" {
				return &ModelFailure{Index: index, Key: ModelFailureNameInvalid}
			}
		}
		for _, checked := range []struct{ field, key string }{
			{"contextWindow", ModelFailureContextInvalid},
			{"maxTokens", ModelFailureMaxTokensInvalid},
		} {
			value, present := model[checked.field]
			if !present {
				continue
			}
			count, ok := modelInteger(value)
			if !ok || count <= 0 {
				return &ModelFailure{Index: index, Key: checked.key}
			}
		}
	}
	return nil
}

// PatchModelDraft 应用一次行内字段改动（官方 `patch`，ModelListEditor.tsx:192-207）。
//
// 语义只有一条但很关键：**被清空的可选字段要离开 profile，不能存成 schema 会拒绝的空串**。
// 所以先合并、再把"值为 nil 或空串"的键整体删掉；本卡不编辑的键因为先合并而存活。
func PatchModelDraft(model ModelDraft, next map[string]any) ModelDraft {
	out := make(ModelDraft, len(model)+len(next))
	for key, value := range model {
		out[key] = value
	}
	cleared := make([]string, 0, len(next))
	for key, value := range next {
		out[key] = value
		if value == nil {
			cleared = append(cleared, key)
			continue
		}
		if text, ok := value.(string); ok && text == "" {
			cleared = append(cleared, key)
		}
	}
	for _, key := range cleared {
		delete(out, key)
	}
	return out
}

// ModelText 读一行里的文本字段；未设置或不是字符串都得空串
// （官方 `textOf`，ModelListEditor.tsx:41-44）。
func ModelText(model ModelDraft, key string) string {
	text, _ := model[key].(string)
	return text
}

// ModelRowMove 校验并计算一次行移动（官方 `move`，useModelRows.ts:60-67）。
//
// 官方四个早退条件逐条同形：不可编辑 / 原地 / 越界 / 目标越界。
// 返回 false 表示这次移动什么都不做。
func ModelRowMove(disabled bool, from, to, length int) bool {
	if disabled || from == to || from < 0 || to < 0 || to >= length {
		return false
	}
	return true
}

// MoveModelDraft 按官方 splice 语义移动一行：取出一项后插到目标位置
// （useModelRows.ts:62-66）。调用方先用 ModelRowMove 判过。
func MoveModelDraft(models []ModelDraft, from, to int) []ModelDraft {
	if !ModelRowMove(false, from, to, len(models)) {
		return models
	}
	order := make([]ModelDraft, len(models))
	copy(order, models)
	moved := order[from]
	order = append(order[:from], order[from+1:]...)
	order = append(order[:to], append([]ModelDraft{moved}, order[to:]...)...)
	return order
}

// ModelDropTarget 把一次拖放折算成"插到第几项之前"（官方 `onDrop`，
// useModelRows.ts:83-88）。
//
// 官方算的是**插入位**（目标行 + half），再因为"移走一项后后面的下标会前移一位"而
// 在插入位大于起点时减 1。这两个换算少一个就会差一位，所以单独成函数并单测。
func ModelDropTarget(from, targetIndex int, after bool) (to int, ok bool) {
	insertion := targetIndex
	if after {
		insertion = targetIndex + 1
	}
	to = insertion
	if insertion > from {
		to = insertion - 1
	}
	return to, to != from
}

// ModelRowAfterHalf 把落点 y 折算成"上半/下半"（官方 `rowHalf`，ModelRow.tsx:46-49）：
// 鼠标在行中线以上插到它前面，以下插到它后面。
func ModelRowAfterHalf(pointerY, rowTop, rowHeight float64) bool {
	return pointerY >= rowTop+rowHeight/2
}

// 输入类型（官方 ModelInputTypes.tsx:39-39 的 `['text','image']`）。
const (
	ModelInputText  = "text"
	ModelInputImage = "image"
)

// modalityStrings 读一个输入类型数组。官方是 JS 数组（ModelInputTypes.tsx:33-34），
// 本仓草稿既可能来自 JSON 解码（[]any）也可能由 Go 直接构造（[]string），两种都认。
func modalityStrings(value any) ([]string, bool) {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...), true
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out, true
	default:
		return nil, false
	}
}

// ModelSelectedInputs 读一行当前生效的输入类型（ModelInputTypes.tsx:33-34）：
// 行里声明了非空数组就用它，否则用回退（装了目录模型的类型 / provider 默认），
// 都没有就 `['text']`——**不会是空集**。
func ModelSelectedInputs(model ModelDraft, field string, fallback []string) []string {
	if out, ok := modalityStrings(model[field]); ok && len(out) > 0 {
		return out
	}
	if len(fallback) > 0 {
		return append([]string(nil), fallback...)
	}
	return []string{ModelInputText}
}

// ModelInputTypeLocked 报告某个勾选是否被禁用（官方 ModelInputTypes.tsx:44）：
// **只剩这一项且它已选中**时不能取消，界面要给 0.4 透明度并忽略点击。
func ModelInputTypeLocked(selected []string, modality string) bool {
	return len(selected) == 1 && selected[0] == modality
}

// ToggleModelInputType 应用一次勾选（官方 ModelInputTypes.tsx:45-55）。
//
// 两个语义都在这里：
//   - 结果按 `['text','image']` 的**固定次序**重建，不受点击顺序影响；
//   - DeepSeek 的 `inputModalities` 在去掉 image 时**连带删掉** imagePixelBudget /
//     imageMaxBytes（:49-54）：文本模型带图片限额会被适配器拒绝。
//
// 返回 ok=false 表示这次点击官方会忽略（只剩一项且已选中，见 ModelInputTypeLocked）。
func ToggleModelInputType(model ModelDraft, field, modality string, checked bool, fallback []string) (ModelDraft, bool) {
	selected := ModelSelectedInputs(model, field, fallback)
	if ModelInputTypeLocked(selected, modality) {
		return model, false
	}
	next := make([]any, 0, 2)
	for _, candidate := range []string{ModelInputText, ModelInputImage} {
		on := false
		if candidate == modality {
			on = checked
		} else {
			for _, current := range selected {
				if current == candidate {
					on = true
					break
				}
			}
		}
		if on {
			next = append(next, candidate)
		}
	}
	out := PatchModelDraft(model, map[string]any{field: next})
	if field == "inputModalities" && !containsModality(next, ModelInputImage) {
		delete(out, "imagePixelBudget")
		delete(out, "imageMaxBytes")
	}
	return out, true
}

func containsModality(values []any, modality string) bool {
	for _, value := range values {
		if text, ok := value.(string); ok && text == modality {
			return true
		}
	}
	return false
}

// ModelPicker 是发现弹窗里的勾选状态（官方 `flipped` + 由列表实时推默认，
// ModelListEditor.tsx:176-180、:244-249）。
//
// 为什么默认值不存快照：官方注释写明"默认值在渲染时从列表读"，于是**探测在飞的时候
// 补进来的行**开弹窗就是已勾选，而不是被一份过期选择删掉（:177-180）。
type ModelPicker struct {
	configured map[string]bool
	flipped    map[string]bool
}

// NewModelPicker 按当前草稿建一个选择器。
func NewModelPicker(models []ModelDraft) *ModelPicker {
	picker := &ModelPicker{configured: make(map[string]bool, len(models)), flipped: map[string]bool{}}
	for _, model := range models {
		if id := ModelText(model, "id"); id != "" {
			picker.configured[id] = true
		}
	}
	return picker
}

// PickedByDefault 报告一个候选的**默认**勾选态：列表里一个已识别模型都没有时全勾
// （"空列表 = 服务整份目录"），否则只勾已配置的那些（ModelListEditor.tsx:247-249）。
func (p *ModelPicker) PickedByDefault(id string) bool {
	return len(p.configured) == 0 || p.configured[id]
}

// Picked 报告最终勾选态 = 默认 XOR 用户翻转（ModelListEditor.tsx:249）。
func (p *ModelPicker) Picked(id string) bool {
	return p.PickedByDefault(id) != p.flipped[id]
}

// Toggle 翻转一个候选（ModelListEditor.tsx:276-282）。
func (p *ModelPicker) Toggle(id string) {
	if p.flipped[id] {
		delete(p.flipped, id)
		return
	}
	p.flipped[id] = true
}

// ToggleVisible 只翻转**当前可见**的候选（官方 `toggleVisibleCandidates`，
// ModelListEditor.tsx:292-304）。
//
// 为什么不能顺手把隐藏的清掉：隐藏的已勾选项可能就是已配置模型，清掉等于悄悄删模型
// （官方注释 :293-294）。
func (p *ModelPicker) ToggleVisible(visibleIDs []string) {
	if len(visibleIDs) == 0 {
		return
	}
	target := false
	all := true
	for _, id := range visibleIDs {
		if !p.Picked(id) {
			all = false
			break
		}
	}
	if !all {
		target = true
	}
	for _, id := range visibleIDs {
		if p.PickedByDefault(id) == target {
			delete(p.flipped, id)
			continue
		}
		p.flipped[id] = true
	}
}

// ModelAdoption 是一次"更新列表"的结算结果（官方 `adoptPicked`，ModelListEditor.tsx:270-274）。
type ModelAdoption struct {
	// Kept 是留下的行（保持原次序）。
	Kept []ModelDraft
	// Removed 是被取消勾选的已配置行下标（行状态要跟着忘掉，:272）。
	Removed []int
	// AddedIDs 是新增的候选 id（按候选顺序）。
	AddedIDs []string
	// EmptiesList 为真时官方**拒绝**确认：把非空列表清空会回到"服务整份目录"
	// （ModelListEditor.tsx:266-268）。
	EmptiesList bool
}

// Adopt 结算一次"更新列表"：留下未取消的行 + 追加新勾选的候选
// （ModelListEditor.tsx:251-274）。
//
// 三条边界逐条照官方：名单外的行（含半截没写完 id 的行）留在原地；已勾选且已存在的
// 不重复加；结果一个不剩时置 EmptiesList 让按钮禁用。
func (p *ModelPicker) Adopt(models []ModelDraft, candidateIDs []string) ModelAdoption {
	listed := make(map[string]bool, len(candidateIDs))
	for _, id := range candidateIDs {
		listed[id] = true
	}
	adoption := ModelAdoption{}
	keptIDs := make(map[string]bool, len(models))
	for index, model := range models {
		id := ModelText(model, "id")
		if listed[id] && !p.Picked(id) {
			adoption.Removed = append(adoption.Removed, index)
			continue
		}
		adoption.Kept = append(adoption.Kept, model)
		keptIDs[id] = true
	}
	for _, id := range candidateIDs {
		if p.Picked(id) && !keptIDs[id] {
			adoption.AddedIDs = append(adoption.AddedIDs, id)
		}
	}
	adoption.EmptiesList = len(models) > 0 && len(adoption.Kept) == 0 && len(adoption.AddedIDs) == 0
	return adoption
}

// modelEditorCopy 是这两个编辑器用到的中文文案，逐字取自官方 locales.ts 的 zh 段
// （dsh-client-ui-settings-models/src/client/locales.ts:203-257）。
// 键名与官方一致，绘制层只按键取，不自己拼中文。
var modelEditorCopy = map[string]string{
	"models":                   "模型目录",         // locales.ts:203
	"modelsInherited":          "正在使用适配器默认模型",  // :204
	"modelsCustomized":         "已自定义模型目录",     // :205
	"resetModels":              "恢复默认模型",       // :206
	"modelId":                  "模型 ID",        // :208
	"modelName":                "显示名称",         // :209
	"contextWindow":            "上下文窗口",        // :211
	"contextWindowPlaceholder": "使用提供商默认值",     // :212
	"maxTokens":                "最大输出 token 数", // :213
	"maxTokensPlaceholder":     "使用提供商默认值",     // :214
	"modelAdvanced":            "模型选项",         // :215
	"modelInputTypes":          "输入类型",         // :216
	"modelInputText":           "文本",           // :217
	"modelInputImage":          "图片",           // :218
	// 推理等级那七条（ModelReasoningEfforts.tsx 用；中文逐字取 locales.ts:219-228）。
	"modelReasoning":                    "推理等级",                                // :219
	"modelReasoningLevel":               "档位",                                  // :220
	"modelReasoningNeedsLevel":          "至少添加一个 Off 以外的档位；不需要思考请关闭推理等级。",      // :224
	"modelReasoningValueRequired":       "Off 以外的档位都需要填写 API 参数值，且参数值不能为空字符串。", // :225
	"modelReasoningAdd":                 "添加档位",                                // :226
	"modelReasoningRemove":              "删除档位",                                // :227
	"modelReasoningOffValuePlaceholder": "关闭思考",                                // :228
	"addModel":                          "添加模型",                                // :230
	"removeModel":                       "删除模型",                                // :231
	"reorderModel":                      "调整模型顺序",                              // :232
	"reorderModelHelp":                  "拖动排序，或使用上下方向键调整顺序。",                  // :233
	"modelsEmpty":                       "模型选择器中将不显示任何模型；目录外 ID 仍可直接发送。",       // :234
	"modelIdRequired":                   "模型 ID 不能为空。",                         // :238
	"modelIdDuplicate":                  "模型 ID 不能重复。",                         // :239
	"modelNameInvalid":                  "显示名称不能为空。",                           // :240
	"modelContextInvalid":               "上下文窗口必须是正数，例如 131072、256K 或 1M。",     // :241
	"modelMaxTokensInvalid":             "最大输出 token 数必须是正数，例如 8192、64K 或 1M。", // :242
	"fetchModels":                       "获取可用模型",                              // :246
	"fetching":                          "正在询问提供商…",                            // :247
	"fetchNeedsBaseUrl":                 "请先填写 API 地址，再获取。",                    // :248
	"fetchEmpty":                        "该提供商没有列出任何模型，请手动添加。",                 // :249
	"fetchTitle":                        "选择模型",                                // :250
	"fetchDescription":                  "以下是模型提供商的可用模型，勾选的模型即为模型列表。",          // :251
	"fetchSearch":                       "搜索模型",                                // :252
	"fetchNoMatches":                    "没有匹配的模型。",                            // :253
	"fetchSelectAll":                    "全选",                                  // :254
	"fetchDeselectAll":                  "取消全选",                                // :255
	"fetchAdopt":                        "更新列表",                                // :256
	"fetchKeepOne":                      "请至少保留一个勾选的模型。",                       // :257
	"close":                             "关闭",                                  // :171
	"cancel":                            "取消",                                  // :172
}

// ModelEditorCopy 按官方键取中文文案；未登记的键返回空串（绘制层据此不画，而不是画键名）。
func ModelEditorCopy(key string) string { return modelEditorCopy[key] }

// ModelCapacityHint 是 pi-ai 一侧空容量格的占位文案：官方把"留空"的语义写成宿主内置
// 默认值（`CAPACITY_HINT`，ModelListEditor.tsx:122-125）——contextWindow `1M`、
// maxTokens `128K`，与 llm-pi-ai 的 defaultContextWindow/defaultMaxTokens 对齐。
func ModelCapacityHint(field string) string {
	if field == "contextWindow" {
		return "1M"
	}
	return "128K"
}
