// official_models_catalog_editor.go —— 模型目录编辑器的**草稿状态与写出口**。
//
// # 官方形状（rc.2 随包 src）
//
//	dsh-client-ui-settings-models/src/client/ModelRow.tsx:56-187        一行（id/name/展开/删除/拖拽柄）
//	dsh-client-ui-settings-models/src/client/DeepSeekModelsEditor.tsx:143-256  目录编辑器本体
//	dsh-client-ui-settings-models/src/client/useModelRows.ts:48-110     行身份/展开态/草稿顺序
//	dsh-client-ui-settings-models/src/client/ModelInputTypes.tsx:32-61  输入类型复选
//	dsh-client-ui-settings-models/src/client/ModelReasoningEfforts.tsx:110-247 推理等级
//
// # 与官方唯一的实质差异：**写到哪**
//
// 官方两个编辑器写 provider profile 的 `models` 数组（DeepSeekModelsEditor.tsx:2-5
// 的注释"settings layer replaces `models` as one array"）。本仓宿主不发布这个字段
// （`../DSH-GO/internal/settingsform/provider_schema.go:167` 原文"baseURL/apiKeyEnv/
// models 等在宿主由模型目录承载"），写了就是**落盘无人消费**。所以写出口是宿主的
// 模型目录 `models.catalog.*`（`../DSH-GO/cmd/dshd/serve_models.go:28-34`），
// 语义层仍然逐条复用 #178 的 `official_models_catalog.go`（容量三态、五条行校验、
// patch 清空即删键、输入类型至少留一项、拖放落点）。
//
// # 删除没有面（不发明契约）
//
// 官方行上的删除按钮（ModelRow.tsx:140-149）落到 `onRemove` → `remove(index)`
// （DeepSeekModelsEditor.tsx:157-160）：**本地数组 splice + 整数组重写**。宿主目录
// 没有"整数组重写"，也没有 delete：`../DSH-GO/internal/llm/model_catalog.go:513-546`
// 的 Reorder 要求 ids 是目录的**全排列**（少一个即 `bad_request`）。所以这一格在
// 本单**不画成可点按钮**（点不着的按钮比没有更坏），只画禁用态并写明原因，
// 宿主侧缺口另开单（见 docs/official-parity.md #41）。
package dssettings

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// 行 ID。**catalog-model- / catalog-failure- 已被只读目录分组占用**
// （official_models.go:296-306），所以编辑器的行一律用 catalog-* 的另一组前缀。
const (
	catalogCardRowID   = "catalog-edit" // 卡片头行（编辑/收起）
	catalogTitleRowID  = "catalog-title"
	catalogMetaRowID   = "catalog-meta"
	catalogEmptyRowID  = "catalog-empty"
	catalogAddRowID    = "catalog-add"
	catalogCancelRowID = "catalog-cancel" // EditorFooter dismiss
	catalogApplyRowID  = "catalog-apply"  // EditorFooter commit

	catalogRowPrefix       = "catalog-row-"    // 一条模型（复合行：柄/id/展开/删除/名称）
	catalogCapacityPrefix  = "catalog-cap-"    // 容量输入行，后缀 = key + "-" + field
	catalogInputsPrefix    = "catalog-input-"  // 输入类型行
	catalogReasonPrefix    = "catalog-reason-" // 推理等级开关行
	catalogEffortPrefix    = "catalog-eff-"    // 一个档位行，后缀 = key + "-" + 下标
	catalogEffortAddPrefix = "catalog-effadd-" // 添加档位
)

// catalogReasoningLevels 是这一格能声明的档位，按**升级顺序**。
//
// 出处：官方 `INITIAL_LEVELS = ['off','low','medium','high']`
// （ModelReasoningEfforts.tsx:15 的注释"Off plus the common OpenAI-compatible
// trio"）。用它的理由：宿主对 `reasoningEfforts` 的 level 名**只要求非空**
// （`../DSH-GO/internal/llm/model_catalog.go:1299-1327`），并不发布一份档位清单；
// 官方在没有 schema 清单时画不出这一格（ModelRow.tsx:174 的
// `reasoningLevels.length > 0`），而有了这份官方常量就能画且写下去宿主一定收。
var catalogReasoningLevels = []string{"off", "low", "medium", "high"}

// catalogCatalogFieldInput 是输入类型写在宿主目录上的字段名。
//
// 官方 DeepSeek 适配器用 `inputModalities`（ModelRow.tsx:228），pi-ai 手写路由用
// `input`（ModelRow.tsx:29 的 `inputField` 二选一）。本仓宿主只认 `input`
// （`../DSH-GO/internal/llm/model_catalog.go:1218` 的字段白名单），
// 所以固定写 `input`——写 `inputModalities` 会被宿主当成未知字段存下来却永不生效。
const catalogCatalogFieldInput = "input"

// catalogEffort 是一个推理档位。Value 用 any：宿主允许 `off` 的值为 null
// （`../DSH-GO/internal/llm/model_catalog.go:1315-1317`），其它档位必须非空字符串。
type catalogEffort struct {
	Level string
	Value any
}

// catalogEffortsJSON 按**编辑器里的顺序**序列化 reasoningEfforts。
//
// 为什么不用 map：官方 `Object.entries(model['reasoningEfforts'])`
// （ModelReasoningEfforts.tsx:43）读的就是存储顺序，Go 的 map 序列化会按键名重排，
// 一存一读就把用户写的顺序洗掉了。
type catalogEffortsJSON []catalogEffort

func (o catalogEffortsJSON) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for index, entry := range o {
		if index > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(entry.Level)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		value, err := json.Marshal(entry.Value)
		if err != nil {
			return nil, err
		}
		buf.Write(value)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// catalogEditor 是"模型目录"编辑卡的本地草稿。
//
// 生命周期与 `retryEditorState` 同一套：点卡片头开卡时从宿主快照建一份，Cancel
// 丢弃，Apply 成功后清掉。**只改草稿不发请求**——唯一的写出口是 `applyCatalogEditor`。
type catalogEditor struct {
	// generation 让"旧卡的收尾"不能清掉新卡（与 retryEditorState 同一理由）。
	generation uint64
	// writable 表示壳接上了写口（Actions.ModelsCatalogUpsert/Reorder 非 nil）。
	writable bool
	// revision 是宿主的目录 revision；每次写成功后换成宿主回的新值。
	// nil = 宿主没给（不 fence，绝不自己编一个 0 当 revision）。
	revision *int64
	// drafts/keys 一一对齐。keys 是官方 `rowKey` 的等价物（useModelRows.ts:9-13）：
	// 行身份跟着模型走，移动时一起挪，所以展开态不会串到别的行上。
	drafts []ModelDraft
	keys   []string
	// nextKey 只增不减（官方 `useRef(0)` 的 nextKey，useModelRows.ts:51）。
	nextKey int
	// expanded 是展开态，键 = 行 key。
	expanded map[string]bool
	// efforts 是每行的推理档位表，键 = 行 key；顺序 = 写入顺序。
	efforts map[string][]catalogEffort
	// serverIDs 是最近一次从宿主读到的 id 顺序，用来判断保存时要不要 reorder。
	serverIDs []string
}

// newCatalogEditor 按宿主快照建一份草稿。
func newCatalogEditor(document *apiclient.ModelCatalogSnapshot, writable bool) *catalogEditor {
	editor := &catalogEditor{
		writable: writable, expanded: map[string]bool{}, efforts: map[string][]catalogEffort{},
	}
	if document == nil {
		return editor
	}
	if document.Document != nil {
		revision := document.Document.Revision
		editor.revision = &revision
		for _, raw := range document.Document.Models {
			editor.serverIDs = append(editor.serverIDs, catalogModelID(raw))
			key := editor.freshKey()
			editor.keys = append(editor.keys, key)
			editor.drafts = append(editor.drafts, catalogDecodeDraft(raw))
			editor.efforts[key] = catalogReasoningEntries(raw)
		}
	}
	return editor
}

// freshKey 造一个新行身份。
func (e *catalogEditor) freshKey() string {
	e.nextKey++
	return "m" + strconv.Itoa(e.nextKey)
}

// catalogDecodeDraft 把一条目录模型解成开放草稿（未知字段原样存活）。
//
// 用 UseNumber：容量这类整数必须保持"整数"身份，解成 float64 再写回去会把
// 131072 写成 131072 但把超大值写成科学计数法；json.Number 原样回写。
func catalogDecodeDraft(raw json.RawMessage) ModelDraft {
	draft := ModelDraft{}
	if len(raw) == 0 {
		return draft
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&draft); err != nil {
		return ModelDraft{}
	}
	return draft
}

// catalogModelID 读一条目录模型的 id（读不出来就空串，交给校验点名）。
func catalogModelID(raw json.RawMessage) string {
	var probe struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return ""
	}
	return probe.ID
}

// catalogReasoningEntries 按**JSON 原文顺序**读出一行的 reasoningEfforts 条目。
//
// 官方 `Object.entries` 读的就是存储顺序；Go 的 `map[string]any` 读不出顺序，
// 所以这里按 token 流扫一遍。`reasoningEfforts: false`（宿主允许，
// `../DSH-GO/internal/llm/model_catalog.go:1300-1302`）= 关闭，返回空表。
func catalogReasoningEntries(raw json.RawMessage) []catalogEffort {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil
	}
	value, ok := fields["reasoningEfforts"]
	if !ok || len(bytes.TrimSpace(value)) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	token, err := decoder.Token()
	if err != nil {
		return nil
	}
	if delim, isDelim := token.(json.Delim); !isDelim || delim != '{' {
		return nil
	}
	entries := make([]catalogEffort, 0, 4)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return entries
		}
		level, _ := keyToken.(string)
		var entryValue any
		if err := decoder.Decode(&entryValue); err != nil {
			return entries
		}
		if text, isString := entryValue.(string); isString {
			entries = append(entries, catalogEffort{Level: level, Value: text})
			continue
		}
		if entryValue == nil {
			entries = append(entries, catalogEffort{Level: level, Value: nil})
			continue
		}
		// 手改过的值不是 string 也不是 null：原样留着，让校验点名（官方
		// `acceptedValue` 也是"不认就报错"，ModelReasoningEfforts.tsx:28-30）。
		entries = append(entries, catalogEffort{Level: level, Value: entryValue})
	}
	return entries
}

// indexOfKey 返回行身份对应的下标；找不到 = -1。
func (e *catalogEditor) indexOfKey(key string) int {
	for index, candidate := range e.keys {
		if candidate == key {
			return index
		}
	}
	return -1
}

// toggleExpanded 翻转一行的展开态（官方 `rowState.toggle`，useModelRows.ts:92-94）。
func (e *catalogEditor) toggleExpanded(key string) {
	if e.expanded == nil {
		e.expanded = map[string]bool{}
	}
	e.expanded[key] = !e.expanded[key]
}

// addModel 追加一行空 id 的草稿（官方 `onChange([...models, {id:”}])`，
// DeepSeekModelsEditor.tsx:249-250）。
func (e *catalogEditor) addModel() {
	key := e.freshKey()
	e.keys = append(e.keys, key)
	e.drafts = append(e.drafts, ModelDraft{"id": ""})
	e.expanded[key] = true
}

// moveRow 按官方 splice 语义移动一行，行身份与档位表跟着一起挪
// （official `move`，useModelRows.ts:60-67 → #178 的 MoveModelDraft）。
func (e *catalogEditor) moveRow(from, to int) bool {
	if !ModelRowMove(false, from, to, len(e.drafts)) {
		return false
	}
	order := MoveModelDraft(e.drafts, from, to)
	keys := make([]string, len(e.keys))
	copy(keys, e.keys)
	key := keys[from]
	keys = append(keys[:from], keys[from+1:]...)
	keys = append(keys[:to], append([]string{key}, keys[to:]...)...)
	e.drafts, e.keys = order, keys
	return true
}

// reasoningOn 报告一行的推理等级是否打开（字段存在且不是 false）。
func (e *catalogEditor) reasoningOn(key string) bool {
	return len(e.efforts[key]) > 0
}

// setReasoning 打开/关闭一行的推理等级（官方 Switch 的 onChange，
// ModelReasoningEfforts.tsx:171-176）：打开写官方那组初始档位，关闭**删字段**。
func (e *catalogEditor) setReasoning(key string, on bool) {
	index := e.indexOfKey(key)
	if index < 0 {
		return
	}
	if !on {
		delete(e.efforts, key)
		return
	}
	entries := make([]catalogEffort, 0, len(catalogReasoningLevels))
	for _, level := range catalogReasoningLevels {
		// 官方 `defaultValue`：off 发 null，其它档位默认发档位名
		// （ModelReasoningEfforts.tsx:18-20）。
		if level == "off" {
			entries = append(entries, catalogEffort{Level: level, Value: nil})
			continue
		}
		entries = append(entries, catalogEffort{Level: level, Value: level})
	}
	e.efforts[key] = entries
}

// addEffort 追加一个还没用过的档位（官方 `addLevel`，ModelReasoningEfforts.tsx:131-138）。
func (e *catalogEditor) addEffort(key string) bool {
	used := map[string]bool{}
	for _, entry := range e.efforts[key] {
		used[entry.Level] = true
	}
	for _, level := range catalogReasoningLevels {
		if used[level] {
			continue
		}
		e.efforts[key] = append(e.efforts[key], catalogEffort{Level: level, Value: catalogEffortDefault(level)})
		return true
	}
	return false
}

// catalogEffortDefault 是某一档位的默认值（off = null，其它 = 档位名）。
func catalogEffortDefault(level string) any {
	if level == "off" {
		return nil
	}
	return level
}

// cycleEffortLevel 把一个档位行换到下一个候选（官方那个 `<select>` 的等价动作：
// 候选 = 没被别的行用掉的档位，ModelReasoningEfforts.tsx:152-156）。
func (e *catalogEditor) cycleEffortLevel(key string, index int) bool {
	entries := e.efforts[key]
	if index < 0 || index >= len(entries) {
		return false
	}
	used := map[string]bool{}
	for at, entry := range entries {
		if at != index {
			used[entry.Level] = true
		}
	}
	choices := make([]string, 0, len(catalogReasoningLevels))
	for _, level := range catalogReasoningLevels {
		if !used[level] {
			choices = append(choices, level)
		}
	}
	if len(choices) == 0 {
		return false
	}
	current := entries[index].Level
	next := choices[0]
	for at, choice := range choices {
		if choice == current {
			next = choices[(at+1)%len(choices)]
			break
		}
	}
	if next == current {
		return false
	}
	// 官方 `changeLevel`：值还是旧档位名（或跨越 off）就重置，否则保留自定义值
	// （ModelReasoningEfforts.tsx:124-129）。
	value := entries[index].Value
	carried := false
	if text, isString := value.(string); isString && text != "" && text != current && current != "off" && next != "off" {
		carried = true
	}
	if carried {
		entries[index] = catalogEffort{Level: next, Value: value}
	} else {
		entries[index] = catalogEffort{Level: next, Value: catalogEffortDefault(next)}
	}
	e.efforts[key] = entries
	return true
}

// removeEffort 删掉一个档位行（官方行尾那枚 trash，ModelReasoningEfforts.tsx:219-228）。
func (e *catalogEditor) removeEffort(key string, index int) bool {
	entries := e.efforts[key]
	if index < 0 || index >= len(entries) {
		return false
	}
	e.efforts[key] = append(entries[:index:index], entries[index+1:]...)
	return true
}

// setEffortValue 改一个档位的 API 参数值。空白 = 回到默认值（官方 `editValue` +
// `leaveValue` 的合并：本仓没有 blur，所以"清空即默认"这条在输入时就结算，
// 语义与官方 `leaveValue` 一致，ModelReasoningEfforts.tsx:140-150）。
func (e *catalogEditor) setEffortValue(key string, index int, text string) bool {
	entries := e.efforts[key]
	if index < 0 || index >= len(entries) {
		return false
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		entries[index].Value = catalogEffortDefault(entries[index].Level)
	} else {
		entries[index].Value = trimmed
	}
	e.efforts[key] = entries
	return true
}

// selectedInputs 读一行当前生效的输入类型（#178 的 ModelSelectedInputs，
// 官方 ModelInputTypes.tsx:33-34；没有任何声明时回退 `['text']`）。
func catalogSelectedInputs(draft ModelDraft) []string {
	return ModelSelectedInputs(draft, catalogCatalogFieldInput, nil)
}

// toggleInput 应用一次输入类型勾选（#178 的 ToggleModelInputType，
// 官方 ModelInputTypes.tsx:45-55）。
func toggleCatalogInput(draft ModelDraft, modality string, checked bool) (ModelDraft, bool) {
	return ToggleModelInputType(draft, catalogCatalogFieldInput, modality, checked, nil)
}

// catalogPatch 应用一次行内字段改动（#178 的 PatchModelDraft，官方
// ModelListEditor.tsx:192-207：**清空的可选字段离开 profile，不存空串**）。
func catalogPatch(draft ModelDraft, next map[string]any) ModelDraft {
	return PatchModelDraft(draft, next)
}

// 行校验的文案键复用 #178 的 modelEditorCopy（逐字取官方 locales.ts:238-242）。
// 推理等级两条来自同一份 locales（:224-225）。
const (
	ModelFailureReasoningValueRequired = "modelReasoningValueRequired"
	ModelFailureReasoningNeedsLevel    = "modelReasoningNeedsLevel"
)

// validateCatalogDrafts 先跑 #178 的五条行校验，再跑推理等级的两条
// （官方 validateReasoningEfforts，ModelReasoningEfforts.tsx:77-85）。
func (e *catalogEditor) validate() *ModelFailure {
	if failure := ValidateModelDrafts(e.drafts); failure != nil {
		return failure
	}
	for index, key := range e.keys {
		entries := e.efforts[key]
		if len(entries) == 0 {
			continue
		}
		nonOff := false
		for _, entry := range entries {
			if entry.Level != "off" {
				nonOff = true
			}
			if entry.Level == "off" && entry.Value == nil {
				continue
			}
			text, isString := entry.Value.(string)
			if !isString || text == "" {
				return &ModelFailure{Index: index, Key: ModelFailureReasoningValueRequired}
			}
		}
		if !nonOff {
			return &ModelFailure{Index: index, Key: ModelFailureReasoningNeedsLevel}
		}
	}
	return nil
}

// modelPayload 把一行草稿序列化成宿主认的 model JSON。
//
// 三条：
//   - id 先 trim（官方 `onIdBlur` 就 trim，DeepSeekModelsEditor.tsx:235-238；
//     宿主 `validateCatalogID` 要求首尾无空白，`../DSH-GO/internal/llm/
//     model_catalog.go:1230-1236`）；
//   - 推理档位按编辑器顺序写，空表 = **删字段**（官方 `withEfforts`，
//     ModelReasoningEfforts.tsx:52-61）；
//   - 其它字段原样（未知字段与 retryPolicy 必须活着）。
func (e *catalogEditor) modelPayload(index int) (json.RawMessage, error) {
	draft := e.drafts[index]
	out := make(map[string]any, len(draft)+1)
	for key, value := range draft {
		out[key] = value
	}
	if id, ok := out["id"].(string); ok {
		out["id"] = strings.TrimSpace(id)
	}
	key := e.keys[index]
	if entries := e.efforts[key]; len(entries) > 0 {
		out["reasoningEfforts"] = catalogEffortsJSON(entries)
	} else {
		delete(out, "reasoningEfforts")
	}
	return json.Marshal(out)
}

// idsForWrite 是草稿的 id 顺序（已 trim）。宿主 reorder 要的是**全排列**。
func (e *catalogEditor) idsForWrite() []string {
	out := make([]string, 0, len(e.drafts))
	for _, draft := range e.drafts {
		id, _ := draft["id"].(string)
		out = append(out, strings.TrimSpace(id))
	}
	return out
}

// orderChanged 报告草稿顺序是否与宿主不同——不同才发 reorder，不白抬 revision。
func (e *catalogEditor) orderChanged() bool {
	ids := e.idsForWrite()
	if len(ids) != len(e.serverIDs) {
		return true
	}
	for index := range ids {
		if ids[index] != e.serverIDs[index] {
			return true
		}
	}
	return false
}

// consumesSameIDs 报告草稿是否**一条宿主模型都没丢**（reorder 的前提）。
//
// 判据只看"宿主那 N 条都还在草稿里"：新加的行还没进目录，不参与这个判断；
// 一旦少一条（将来宿主有了 delete 就会有这种草稿）就**不能**发 reorder——
// 宿主会以 `bad_request` 拒绝（"必须逐项给出完整排列"，model_catalog.go:530-533），
// 而且那种拒绝不该被用户理解成"删除成功了"。
func (e *catalogEditor) consumesSameIDs() bool {
	present := map[string]bool{}
	for _, id := range e.idsForWrite() {
		present[id] = true
	}
	for _, id := range e.serverIDs {
		if !present[id] {
			return false
		}
	}
	return true
}

// hasServerID 报告某个 id 是否是宿主目录里已有的模型（新加的行没有）。
func (e *catalogEditor) hasServerID(id string) bool {
	for _, candidate := range e.serverIDs {
		if candidate == id {
			return true
		}
	}
	return false
}

// catalogCopy 按官方键取中文文案；未登记的键返回空串（绘制层据此不画）。
func catalogCopy(key string) string { return ModelEditorCopy(key) }
