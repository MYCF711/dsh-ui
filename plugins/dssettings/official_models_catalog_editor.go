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
// # 删除/排序/新增：一次 `models.catalog.replace`
//
// 官方行上的删除按钮（ModelRow.tsx:142-151）落到 `onRemove` → `remove(index)`
// （DeepSeekModelsEditor.tsx:157-160）：**本地数组 splice + 整数组重写**。宿主
// 早先只有逐条 upsert 与全排列 reorder（`../DSH-GO/internal/llm/model_catalog.go`
// 的 Reorder 少一个 id 即 `bad_request`），表达不了删除；`dsh-go#82` 补的
// `models.catalog.replace` 就是这条整数组语义（逐行规范化 + 重复 id 拒绝 +
// 乐观锁 + 整数组替换），所以现在**删除、排序、新增走同一个写口**（见
// official_models_catalog_write.go）。
//
// # 本卡编辑的字段（每一条都对得上宿主真实消费点）
//
//	id/name/contextWindow/maxTokens/input/reasoningEfforts  官方 ModelRow 的字段面
//	baseURL  `../DSH-GO/internal/llm/model_catalog.go:1280-1287` 的白名单字段，
//	         由 ResolveProvider（同文件 :871-905）真正拿去发请求
//	api      同上（:1271-1290），缺省按 openai-completions 解析（:878-880）
//
// 官方把 baseURL/api 放在**provider profile**（ProviderEditor.tsx:552-595），本仓
// 宿主的 provider profile 只发布并消费 `retryPolicy`（`../DSH-GO/internal/
// settingsform/provider_schema.go:164-176` + `internal/llm/provider_settings.go:41-90`），
// 所以写 `providers.<route>.baseURL` 只会落盘无人消费。等价真值就在目录条目上
// （同一份 model_catalog.go 的字段白名单），因此这两格画在**行**上而不是 provider 卡上：
// 文案/占位/协议候选仍逐条取官方（locales.ts:236-241、:333-338）。
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
	// serverIDs 是最近一次从宿主读到的 id 顺序（保存时用它标出"哪几行是新加的"）。
	serverIDs []string
	// protocols 是"API 协议"这一格的候选：宿主真正注册的 wire 协议。
	//
	// 官方候选来自 llm 命名空间 schema 里 `providers.<route>.api` 的 union
	// （`OFF:store.ts:138-147` 的 protocolChoices）；本仓宿主不发布那份 union
	// （`../DSH-GO/internal/settingsform/provider_schema.go:164-176` 只发布
	// retryPolicy），等价真值是 `llm.listConfigurableProviders` 报的 provider 名
	// ——就是 adapter 的 api（`../DSH-GO/internal/llm/provider_registry.go:113-125`）。
	// 空表 = 宿主一个适配器都没报，这一格不画（没有候选的下拉等于没有信息）。
	protocols []string
	// providerDefault/providerEnv 是路由级的端点事实，只用来算 baseURL 那格的
	// **占位文案**（官方 placeholder 三个分支读的就是这些，ProviderEditor.tsx:558-561）：
	// `defaultBaseURL` 与 `baseURLEnv` 由 `llm.listConfigurableProviders` 报出
	// （`../DSH-GO/internal/llm/provider_registry.go:35-40`、:113-125）。
	providerDefault map[string]string
	providerEnv     map[string][]string
	// candidates 是一次"获取可用模型"的候选（官方 `candidates`，
	// ModelListEditor.tsx:236-251），picker 是勾选状态（ModelPicker）。
	// nil = 弹窗没开；空表 + picker 非空 = 上游一个模型都没列（fetchEmpty）。
	candidates []ModelDraft
	picker     *ModelPicker
	// discovering 是询问在飞（官方 `busy`），discoverError 是失败原文
	// （上游拒绝时官方把 message 直接显示，:220-222）。
	discovering   bool
	discoverError string
	// discoverTarget 记下这次询问用的端点，回填"问了哪个地址"的失败上下文。
	discoverTarget string
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

// applyRouteFacts 记下宿主报的路由级事实（协议候选 + 端点占位依据）。
//
// 数据来自 `llm.listConfigurableProviders`（`../DSH-GO/internal/llm/
// provider_registry.go:113-125` 组装）：`Provider` 就是 adapter 的 api 名，
// `DefaultBaseURL`/`BaseURLEnv` 是它声明的端点事实。它们只影响**画什么**：
// 协议候选与 baseURL 那格的占位文案，不参与写入。
func (e *catalogEditor) applyRouteFacts(providers []apiclient.ConfigurableProvider) {
	e.protocols = e.protocols[:0]
	e.providerDefault = map[string]string{}
	e.providerEnv = map[string][]string{}
	for _, provider := range providers {
		name := strings.TrimSpace(provider.Provider)
		if name == "" {
			continue
		}
		if !containsString(e.protocols, name) {
			e.protocols = append(e.protocols, name)
		}
		if provider.DefaultBaseURL != "" {
			if _, exists := e.providerDefault[name]; !exists {
				e.providerDefault[name] = provider.DefaultBaseURL
			}
		}
		if len(provider.BaseURLEnv) > 0 {
			e.providerEnv[name] = append([]string(nil), provider.BaseURLEnv...)
		}
	}
}

// removeModel 删掉一行（官方 `onRemove` → `remove(index)`，
// ModelRow.tsx:145-148 的删除钮 → DeepSeekModelsEditor.tsx:157-160 的 splice）。
//
// 行身份、展开态与档位表一起忘掉：留在 map 里会串到下一条占用同一 key 的行上
// （官方 `rowState.forget(removedRows)`，ModelListEditor.tsx:273 是同一件事）。
// 删到空是合法目标——官方 `<p className="modelEmpty">` 就是那个状态的文案，
// 宿主也接受 `models: []`（serve_models.go 的 replace 分支注释）。
func (e *catalogEditor) removeModel(key string) bool {
	index := e.indexOfKey(key)
	if index < 0 {
		return false
	}
	e.drafts = append(e.drafts[:index:index], e.drafts[index+1:]...)
	e.keys = append(e.keys[:index:index], e.keys[index+1:]...)
	if e.expanded != nil {
		delete(e.expanded, key)
	}
	if e.efforts != nil {
		delete(e.efforts, key)
	}
	return true
}

// setBaseURL 改一行的 API 地址。空白 = **删键**（官方
// `setField('baseURL', event.target.value === ” ? undefined : value)`，
// ProviderEditor.tsx:565-567）：存空串会被宿主 `invalid_field` 拒
// （`../DSH-GO/internal/llm/model_catalog.go:1281-1287` 要 http(s) URL）。
func (e *catalogEditor) setBaseURL(key, text string) bool {
	index := e.indexOfKey(key)
	if index < 0 {
		return false
	}
	e.drafts[index] = catalogPatch(e.drafts[index], map[string]any{"baseURL": strings.TrimSpace(text)})
	return true
}

// setAPI 改一行的 wire 协议。空串 = 删键（回到"未选择"，官方
// `customApiUnset` 那个空选项，ProviderEditor.tsx:584-590）。
func (e *catalogEditor) setAPI(key, api string) bool {
	index := e.indexOfKey(key)
	if index < 0 {
		return false
	}
	e.drafts[index] = catalogPatch(e.drafts[index], map[string]any{"api": strings.TrimSpace(api)})
	return true
}

// nextAPI 是一个协议行的"下一个候选"（官方那个 `<select>` 的等价动作）：
// 未选择 → 第一个候选；已选 → 列表里的下一个（走到尾回第一个）。
//
// 为什么循环里**不含**"未选择"：官方那个空选项只在 `probeApi === undefined`
// 时出现（ProviderEditor.tsx:590），选中任何一个协议之后它就不再出现，
// 所以这里也不提供"再点一下清空"的路径——清空只发生在尚未选择时。
func (e *catalogEditor) nextAPI(current string) (string, bool) {
	if len(e.protocols) == 0 {
		return "", false
	}
	for at, candidate := range e.protocols {
		if candidate == current {
			return e.protocols[(at+1)%len(e.protocols)], true
		}
	}
	return e.protocols[0], true
}

// askable 报告这一卡有没有可问的端点：官方判据是
// `probe.provider !== undefined || (probe.baseURL !== undefined && baseURL.length > 0)`
// （ModelListEditor.tsx:310-311）。本卡的端点来自**行**：取第一条声明了 baseURL
// 的草稿。一条都没有时不可问，按钮的 title 用的是官方 `fetchNeedsBaseUrl`
// （locales.ts:248）。
func (e *catalogEditor) askable() (baseURL, api string, ok bool) {
	for _, draft := range e.drafts {
		value := strings.TrimSpace(ModelText(draft, "baseURL"))
		if value == "" {
			continue
		}
		return value, strings.TrimSpace(ModelText(draft, "api")), true
	}
	return "", "", false
}

// catalogAdoptedDraft 把一条候选折成一行草稿（官方 `adopt`，
// ModelListEditor.tsx:142-150）：**只写端点报了的字段**，缺的就是未知，
// 不拿 0 或空数组顶上（那是"报了 0"，与"没报"是两件事）。
func catalogAdoptedDraft(candidate apiclient.DiscoveredModel) ModelDraft {
	draft := ModelDraft{"id": candidate.ID}
	if candidate.Name != nil {
		draft["name"] = *candidate.Name
	}
	if candidate.ContextWindow != nil {
		draft["contextWindow"] = float64(*candidate.ContextWindow)
	}
	if candidate.MaxTokens != nil {
		draft["maxTokens"] = float64(*candidate.MaxTokens)
	}
	if len(candidate.InputModalities) > 0 {
		inputs := make([]any, 0, len(candidate.InputModalities))
		for _, modality := range candidate.InputModalities {
			inputs = append(inputs, modality)
		}
		draft["input"] = inputs
	}
	return draft
}

// openCandidates 打开挑模型的面（官方：拿到非空结果后清空搜索与翻转、按 id 去重，
// ModelListEditor.tsx:236-245）。
//
// 返回 false = 上游一个模型都没列：官方这时**不开面**，直接把
// `fetchEmpty`（"该提供商没有列出任何模型，请手动添加。"）当失败文案显示。
func (e *catalogEditor) openCandidates(found []apiclient.DiscoveredModel) bool {
	seen := make(map[string]bool, len(found))
	candidates := make([]ModelDraft, 0, len(found))
	for _, candidate := range found {
		if candidate.ID == "" || seen[candidate.ID] {
			continue
		}
		seen[candidate.ID] = true
		candidates = append(candidates, catalogAdoptedDraft(candidate))
	}
	e.discovering = false
	e.discoverError = ""
	if len(candidates) == 0 {
		e.discoverError = catalogCopy("fetchEmpty")
		e.candidates = nil
		e.picker = nil
		return false
	}
	e.candidates = candidates
	// picker 建在**当前草稿**上：空列表时全勾（"服务整份目录"），否则只勾已配置的
	// （官方 `pickedByDefault`，ModelListEditor.tsx:247-249 用的是渲染时的列表）。
	e.picker = NewModelPicker(e.drafts)
	return true
}

// closeCandidates 关掉挑模型的面（官方 `closePicker`，:246-250）。
func (e *catalogEditor) closeCandidates() {
	e.candidates = nil
	e.picker = nil
	e.discoverError = ""
}

// candidateIDs 是候选的 id 顺序（Adopt 要它）。
func (e *catalogEditor) candidateIDs() []string {
	ids := make([]string, 0, len(e.candidates))
	for _, candidate := range e.candidates {
		ids = append(ids, ModelText(candidate, "id"))
	}
	return ids
}

// toggleCandidate 翻转一条候选的勾选。
func (e *catalogEditor) toggleCandidate(id string) bool {
	if e.picker == nil {
		return false
	}
	e.picker.Toggle(id)
	return true
}

// toggleAllCandidates 翻转**当前可见**的全部候选（官方 `toggleVisibleCandidates`，
// ModelListEditor.tsx:292-304；本卡没有搜索框，可见 = 全部）。
func (e *catalogEditor) toggleAllCandidates() bool {
	if e.picker == nil {
		return false
	}
	e.picker.ToggleVisible(e.candidateIDs())
	return true
}

// adoptPicked 结算"更新列表"（官方 `adoptPicked`，ModelListEditor.tsx:270-274）。
//
// 返回 false = 这一按被官方判据拒（把非空列表清空会回到"服务整份目录"，
// `emptiesList` → `fetchKeepOne`，:266-268/:284-290），草稿一个字节都不动。
func (e *catalogEditor) adoptPicked() (ModelAdoption, bool) {
	if e.picker == nil {
		return ModelAdoption{}, false
	}
	adoption := e.picker.Adopt(e.drafts, e.candidateIDs())
	if adoption.EmptiesList {
		e.discoverError = catalogCopy("fetchKeepOne")
		return adoption, false
	}
	// 留下的行**保留行身份**（展开态/档位表跟着走），新增的行用新身份并默认展开
	// （官方新加的行也从不带状态开始）。
	removed := make(map[int]bool, len(adoption.Removed))
	for _, index := range adoption.Removed {
		removed[index] = true
	}
	keptDrafts := make([]ModelDraft, 0, len(e.drafts))
	keptKeys := make([]string, 0, len(e.keys))
	efforts := make(map[string][]catalogEffort, len(e.efforts))
	expanded := make(map[string]bool, len(e.expanded))
	for index, draft := range e.drafts {
		if removed[index] {
			continue
		}
		keptDrafts = append(keptDrafts, draft)
		key := e.keys[index]
		keptKeys = append(keptKeys, key)
		if entries, ok := e.efforts[key]; ok {
			efforts[key] = entries
		}
		expanded[key] = e.expanded[key]
	}
	for _, id := range adoption.AddedIDs {
		key := e.freshKey()
		keptDrafts = append(keptDrafts, e.candidateDraft(id))
		keptKeys = append(keptKeys, key)
		expanded[key] = true
	}
	e.drafts, e.keys, e.efforts, e.expanded = keptDrafts, keptKeys, efforts, expanded
	e.closeCandidates()
	return adoption, true
}

// candidateDraft 按 id 取回候选那条草稿（Adopt 只回 id）。
func (e *catalogEditor) candidateDraft(id string) ModelDraft {
	for _, candidate := range e.candidates {
		if ModelText(candidate, "id") == id {
			out := make(ModelDraft, len(candidate))
			for field, value := range candidate {
				out[field] = value
			}
			return out
		}
	}
	return ModelDraft{"id": id}
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
