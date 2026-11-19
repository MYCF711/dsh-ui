// official_models_catalog_write.go —— 模型目录编辑卡的开关、行动作与唯一写出口。
//
// # 写入形状：一次 `models.catalog.replace`（与官方同语义）
//
// 官方一次 `settings.mutate(ns, ops, revision)` 把整份 `models` 数组换掉
// （`OFF:operations.ts:86-91` + ProviderEditor 的 applyOnce；删行就是 splice 之后
// 整数组重写，DeepSeekModelsEditor.tsx:157-160）。宿主侧的对等方法是
// `models.catalog.replace`（`../DSH-GO/cmd/dshd/serve_models.go:33`、
// `internal/llm/model_catalog.go` 的 Replace：逐行规范化 + 重复 id 拒绝 + 乐观锁
// + 整数组替换），所以保存是**一次请求**：
//
//	① 校验（#178 五条 + 推理等级两条）——不合法一次请求都不发（官方 `submitDisabled` 的
//	   等价门，DeepSeekModelsEditor.tsx:181-182 的"保存期点名"）；
//	② 把所有草稿序列化成一份数组，带开卡 revision 一次 replace；冲突**不重试覆盖**
//	   （官方 settings/conflict 同一条：`OFF:operations.ts:89-90`），失败保留草稿。
//
// 为什么不是"逐条 upsert + 全排列 reorder"：那两段之间失败会留下半份目录，而且
// reorder 要求 ids 是目录的**全排列**（`model_catalog.go` 的 Reorder），少一条
// （= 删掉一行）就被 `bad_request` 拒——删除在那条契约下根本无法表达。
package dssettings

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// modelsCatalogNS 是模型目录写失败时的归属标签（不是设置命名空间）。
//
// 用它的理由：WriteState.Render 只渲 message，但状态栏/测试要靠 NS 分清"这是
// 设置写失败还是目录写失败"——两者的恢复动作不同（一个重新载入设置，一个重读目录）。
const modelsCatalogNS = "models.catalog"

// catalogWriteTimeout 是一次目录写的超时。写是本地文件落盘（宿主 Update），
// 给 8 秒足够；test 是真实推理，走它自己的超时（Actions.ModelsCatalogTest 内部）。
const catalogWriteTimeout = 8 * time.Second

// catalogFailureMessage 把 #178 的失败键翻成官方中文（locales.ts:238-242、:224-225）。
//
// 行号是**人看的行号**（官方 `index + 1` 的行标签，ModelRow.tsx:227 的 position），
// 所以报文里用第几条而不是下标。
func catalogFailureMessage(failure *ModelFailure) string {
	if failure == nil {
		return ""
	}
	text := ModelEditorCopy(failure.Key)
	if text == "" {
		text = failure.Key
	}
	return "第 " + strconv.Itoa(failure.Index+1) + " 行：" + text
}

// catalogWriteState 按宿主四字段把目录写的失败翻成 WriteState。
//
// 目录的错误码比设置面多（`invalid_id` / `invalid_field` / `duplicate_id` /
// `empty_order` / `empty_catalog` / `not_found` / `revision_conflict`），
// 分类只按 code，不匹配文本；认不出的按内部失败报，**不吞**。
func catalogWriteState(err error) WriteState {
	state := ClassifyWriteError(modelsCatalogNS, []string{"models"}, err)
	if state.Kind != WriteFailed {
		return state
	}
	switch state.Code {
	case "invalid_id", "invalid_field", "duplicate_id", "empty_order", "empty_catalog", "not_found":
		state.Kind = WriteInvalid
	}
	return state
}

// toggleModelsCatalogEditor 开/关模型目录编辑卡。
func (p *Plugin) toggleModelsCatalogEditor() {
	o := p.ensureOfficial()
	o.mu.Lock()
	if o.modelsCatalog != nil {
		o.modelsCatalog = nil
		o.resetCatalogFields()
		o.mu.Unlock()
		p.setWriteState(WriteState{Kind: WriteIdle})
		catalogInvalidateStored(o)
		return
	}
	if o.catalogSnapshot == nil {
		o.mu.Unlock()
		p.setWriteState(WriteState{Kind: WriteFailed, NS: modelsCatalogNS,
			Message: "宿主未接 models.catalog.*：模型目录没有写面"})
		return
	}
	editor := newCatalogEditor(o.catalogSnapshot, o.catalogWritable)
	if o.modelsPage != nil {
		editor.applyRouteFacts(o.modelsPage.Configurable)
	}
	o.nextCatalogGeneration++
	editor.generation = o.nextCatalogGeneration
	o.modelsCatalog = editor
	o.resetCatalogFields()
	o.mu.Unlock()
	p.setWriteState(WriteState{Kind: WriteIdle})
	catalogInvalidateStored(o)
}

// catalogInvalidateStored 用同步时记下的 Invalidate 要一帧（非绘制路径用）。
func catalogInvalidateStored(o *officialPanel) {
	o.mu.Lock()
	invalidate := o.catalogInvalidate
	o.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

// catalogAddModel 追加一行空 id 草稿（官方 `addModel`，DeepSeekModelsEditor.tsx:245-253）。
func (p *Plugin) catalogAddModel() {
	o := p.ensureOfficial()
	o.mu.Lock()
	if editor := o.modelsCatalog; editor != nil {
		editor.addModel()
	}
	o.mu.Unlock()
	catalogInvalidateStored(o)
}

// catalogAddEffort 追加一个档位（官方 `addLevel`，ModelReasoningEfforts.tsx:131-138）。
func (p *Plugin) catalogAddEffort(key string) {
	o := p.ensureOfficial()
	o.mu.Lock()
	if editor := o.modelsCatalog; editor != nil {
		editor.addEffort(key)
	}
	o.mu.Unlock()
	catalogInvalidateStored(o)
}

// activateCatalogRow 处理"整行点击"的目录卡动作（行内有自己命中区的行不走这里）。
//
// 只有两枚小按钮走整行命中区：添加模型、添加档位（官方 `.addModelButton` 是
// 一整颗按钮，ModelReasoningEfforts.tsx:234-242）。
func (p *Plugin) activateCatalogRow(row PanelRow) {
	switch {
	case row.ID == catalogAddRowID:
		p.catalogAddModel()
	case strings.HasPrefix(row.ID, catalogEffortAddPrefix):
		p.catalogAddEffort(catalogRowKey(row.ID, catalogEffortAddPrefix))
	case row.ID == catalogFetchRowID:
		// 询问会发 HTTP：必须离开绘制帧（官方 `void fetchModels()` 是 async）。
		o := p.ensureOfficial()
		o.mu.Lock()
		busy := o.modelsCatalog != nil && o.modelsCatalog.discovering
		o.mu.Unlock()
		if !busy {
			go p.catalogFetchModels()
		}
	case row.ID == catalogCandidateAllID:
		p.catalogToggleAllCandidates()
	case row.ID == catalogAdoptRowID:
		p.catalogAdoptCandidates()
	case row.ID == catalogCandCloseID:
		p.catalogCloseCandidates()
	}
}

// activateModelsCatalogRow 是 activateModelsRow 里的目录分支：返回 true = 认领了这行。
//
// 认领四类：卡片头（开/关）、添加模型、添加档位、EditorFooter 的两枚。
// 其余 catalog-* 行（含每行内部的复合行）也一律认领，**绝不落到 applyRowWrite**——
// 那会把一个不存在的设置路径当成设置字段去写。
func (p *Plugin) activateModelsCatalogRow(row PanelRow) bool {
	if !strings.HasPrefix(row.ID, "catalog-") {
		return false
	}
	o := p.ensureOfficial()
	o.mu.Lock()
	busy := o.write.Kind == WriteSaving
	o.mu.Unlock()
	switch {
	case row.ID == catalogCardRowID:
		p.toggleModelsCatalogEditor()
	case row.ID == catalogAddRowID:
		p.catalogAddModel()
	case row.ID == catalogCancelRowID:
		if busy {
			return true
		}
		// 关卡 = 丢弃草稿（官方 `onClose(false)`），并清掉输入框登记，
		// 下次开卡重新从宿主快照灌初值。
		o.mu.Lock()
		o.modelsCatalog = nil
		o.resetCatalogFields()
		o.mu.Unlock()
		p.setWriteState(WriteState{Kind: WriteIdle})
		catalogInvalidateStored(o)
	case row.ID == catalogApplyRowID:
		if busy {
			return true
		}
		p.setWriteState(WriteState{Kind: WriteSaving, NS: modelsCatalogNS, Path: []string{"models"}})
		go func() { _ = p.applyCatalogEditor() }()
	case strings.HasPrefix(row.ID, catalogEffortAddPrefix):
		p.catalogAddEffort(catalogRowKey(row.ID, catalogEffortAddPrefix))
	case row.ID == catalogFetchRowID, row.ID == catalogCandidateAllID,
		row.ID == catalogAdoptRowID, row.ID == catalogCandCloseID:
		p.activateCatalogRow(row)
	}
	// 其余（catalog-row-* / catalog-base-* / catalog-api-* / catalog-cap-* /
	// catalog-input-* / catalog-reason-* / catalog-eff-* / catalog-cand-*）：
	// 行内各自有命中区，这里只认领、不做事。
	return true
}

// applyCatalogEditor 是这张卡唯一的写出口：一次 `models.catalog.replace`。
//
// 必须在 goroutine 里调（会发 HTTP）。返回的 WriteState 已 `setWriteState`。
func (p *Plugin) applyCatalogEditor() WriteState {
	o := p.ensureOfficial()
	o.mu.Lock()
	editor := o.modelsCatalog
	replace := o.catalogReplace
	o.mu.Unlock()
	if editor == nil {
		state := WriteState{Kind: WriteInvalid, NS: modelsCatalogNS,
			Code: apiclient.SettingsCodeBadRequest, Message: "模型目录编辑卡已关闭"}
		p.setWriteState(state)
		return state
	}
	if failure := editor.validate(); failure != nil {
		state := WriteState{Kind: WriteInvalid, NS: modelsCatalogNS, Path: []string{"models", strconv.Itoa(failure.Index)},
			Code: apiclient.SettingsCodeBadRequest, Message: catalogFailureMessage(failure)}
		p.setWriteState(state)
		return state
	}
	if replace == nil {
		state := WriteState{Kind: WriteFailed, NS: modelsCatalogNS,
			Message: "壳未接 models.catalog.replace：模型目录不能在界面上保存"}
		p.setWriteState(state)
		return state
	}

	o.mu.Lock()
	count := len(editor.drafts)
	revision := editor.revision
	o.mu.Unlock()
	// 一次请求写完：删除/新增/排序都在这一份数组里，不再有"写到一半"的中间态。
	models := make([]json.RawMessage, 0, count)
	for index := 0; index < count; index++ {
		o.mu.Lock()
		current := o.modelsCatalog
		payload := json.RawMessage(nil)
		var payloadErr error
		if current != nil {
			payload, payloadErr = current.modelPayload(index)
		} else {
			payloadErr = fmt.Errorf("模型目录编辑卡已关闭")
		}
		o.mu.Unlock()
		if payloadErr != nil {
			state := WriteState{Kind: WriteInvalid, NS: modelsCatalogNS, Path: []string{"models", strconv.Itoa(index)},
				Code: apiclient.SettingsCodeBadRequest, Message: payloadErr.Error()}
			p.setWriteState(state)
			return state
		}
		models = append(models, payload)
	}
	document, err := replace(models, revision)
	if err != nil {
		state := catalogWriteState(err)
		state.Path = []string{"models"}
		p.setWriteState(state)
		p.reloadCatalogAfterFailure()
		return state
	}
	_ = document

	state := WriteState{Kind: WriteSaved, NS: modelsCatalogNS, Path: []string{"models"}}
	o.mu.Lock()
	if current := o.modelsCatalog; current != nil && current.generation == editor.generation {
		o.modelsCatalog = nil
		o.resetCatalogFields()
	}
	o.mu.Unlock()
	p.setWriteState(state)
	catalogInvalidateStored(o)
	return state
}

// catalogFetchModels 问端点上有哪些模型（官方 `fetchModels`，
// ModelListEditor.tsx:212-245），结果落成候选并打开挑模型的面。
//
// 必须在 goroutine 里调（会发 HTTP）。三条照官方：
//   - 上游拒绝 → 失败原文当说明，**不打开**挑模型的面（:220-223）；
//   - 一个模型都没列 → `fetchEmpty`，同样不开面（:230-233）；
//   - 非空 → 清掉上一次的翻转，按 id 去重后开面（:234-245）。
//
// 端点取自草稿（`askable`），不替用户猜一个地址：猜错就是把别人的模型列表画出来。
func (p *Plugin) catalogFetchModels() {
	o := p.ensureOfficial()
	o.mu.Lock()
	editor := o.modelsCatalog
	discover := o.catalogDiscover
	askBase, askAPI := "", ""
	if editor != nil {
		askBase, askAPI, _ = editor.askable()
		editor.discovering = true
		editor.discoverError = ""
		editor.discoverTarget = askBase
	}
	o.mu.Unlock()
	catalogInvalidateStored(o)
	if editor == nil {
		return
	}
	if discover == nil {
		o.mu.Lock()
		if current := o.modelsCatalog; current != nil {
			current.discovering = false
			current.discoverError = "壳未接 llm.discoverModels：问不到端点上有哪些模型"
		}
		o.mu.Unlock()
		catalogInvalidateStored(o)
		return
	}
	if askBase == "" {
		// 官方这时按钮本来就是禁用的（`askable` 判据，ModelListEditor.tsx:310-311），
		// 走到这里说明状态在飞行中变了：按 fetchNeedsBaseUrl 说完就收。
		o.mu.Lock()
		if current := o.modelsCatalog; current != nil {
			current.discovering = false
			current.discoverError = catalogCopy("fetchNeedsBaseUrl")
		}
		o.mu.Unlock()
		catalogInvalidateStored(o)
		return
	}
	found, err := discover(apiclient.SettingsNSLLM, apiclient.ModelDiscoveryRequest{BaseURL: askBase, API: askAPI})
	o.mu.Lock()
	current := o.modelsCatalog
	if current != nil && current.generation == editor.generation {
		current.discovering = false
		if err != nil {
			current.discoverError = err.Error()
		} else {
			current.openCandidates(found)
		}
	}
	o.mu.Unlock()
	catalogInvalidateStored(o)
}

// catalogToggleCandidate 翻转一条候选的勾选（官方 `toggle`，ModelListEditor.tsx:276-282）。
func (p *Plugin) catalogToggleCandidate(id string) {
	o := p.ensureOfficial()
	o.mu.Lock()
	if editor := o.modelsCatalog; editor != nil {
		editor.toggleCandidate(id)
	}
	o.mu.Unlock()
	catalogInvalidateStored(o)
}

// catalogToggleAllCandidates 全选/取消全选当前可见候选（官方 `toggleVisibleCandidates`）。
func (p *Plugin) catalogToggleAllCandidates() {
	o := p.ensureOfficial()
	o.mu.Lock()
	if editor := o.modelsCatalog; editor != nil {
		editor.toggleAllCandidates()
	}
	o.mu.Unlock()
	catalogInvalidateStored(o)
}

// catalogAdoptCandidates 结算"更新列表"（官方 `adoptPicked`，ModelListEditor.tsx:270-274）。
//
// 被 `fetchKeepOne` 挡住时什么都不动，只把官方那句说明放到面上。
func (p *Plugin) catalogAdoptCandidates() {
	o := p.ensureOfficial()
	o.mu.Lock()
	adopted := false
	if editor := o.modelsCatalog; editor != nil {
		_, adopted = editor.adoptPicked()
	}
	o.mu.Unlock()
	if adopted {
		// 换了行身份/顺序：输入框登记按 key 记，留着会把上一份文本灌到别的行上。
		o.mu.Lock()
		o.resetCatalogFields()
		o.mu.Unlock()
	}
	catalogInvalidateStored(o)
}

// catalogCloseCandidates 关掉挑模型的面（官方 `closePicker`，ModelListEditor.tsx:246-250）。
func (p *Plugin) catalogCloseCandidates() {
	o := p.ensureOfficial()
	o.mu.Lock()
	if editor := o.modelsCatalog; editor != nil {
		editor.closeCandidates()
	}
	o.mu.Unlock()
	catalogInvalidateStored(o)
}

// reloadCatalogAfterFailure 在写失败后请求重读目录（官方"失败先 recovery reload"，
// `ui-settings/src/client/config-form.ts:129-155`）：本地的 revision 可能已过期。
func (p *Plugin) reloadCatalogAfterFailure() {
	o := p.ensureOfficial()
	o.mu.Lock()
	refresh := o.catalogRefresh
	o.mu.Unlock()
	if refresh != nil {
		refresh()
	}
}
