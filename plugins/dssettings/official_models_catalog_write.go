// official_models_catalog_write.go —— 模型目录编辑卡的开关、行动作与唯一写出口。
//
// # 写入形状（与官方同语义、不同面）
//
// 官方一次 `settings.mutate(ns, ops, revision)` 把整份 `models` 数组换掉
// （`OFF:operations.ts:86-91` + ProviderEditor 的 applyOnce）。本仓宿主没有"整数组
// 重写"，只有**逐条 upsert** 与**全排列 reorder**（`../DSH-GO/cmd/dshd/serve_models.go:28-34`），
// 所以保存 = 对每一条草稿 upsert 一次，再（仅当顺序变了）reorder 一次：
//
//	① 校验（#178 五条 + 推理等级两条）——不合法一次请求都不发（官方 `submitDisabled` 的
//	   等价门，DeepSeekModelsEditor.tsx:181-182 的"保存期点名"）；
//	② 逐条 upsert，expectedRevision 跟着宿主回报往前走（乐观锁，冲突**不重试覆盖**，
//	   官方 settings/conflict 同一条：`OFF:operations.ts:89-90`）；
//	③ 顺序变了才 reorder；宿主要求 ids 是目录全排列，数量对不上时**不猜**，直接报失败。
//
// 每一步失败都保留草稿（官方：冲突时保留编辑上下文），只有成功才关卡。
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
	}
	// 其余（catalog-row-* / catalog-cap-* / catalog-input-* / catalog-reason-* /
	// catalog-eff-*）：行内各自有命中区，这里只认领、不做事。
	return true
}

// applyCatalogEditor 是这张卡唯一的写出口：逐条 upsert，必要时 reorder。
//
// 必须在 goroutine 里调（会发 HTTP）。返回的 WriteState 已 `setWriteState`。
func (p *Plugin) applyCatalogEditor() WriteState {
	o := p.ensureOfficial()
	o.mu.Lock()
	editor := o.modelsCatalog
	upsert := o.catalogUpsert
	reorder := o.catalogReorder
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
	if upsert == nil {
		state := WriteState{Kind: WriteFailed, NS: modelsCatalogNS,
			Message: "壳未接 models.catalog.upsert：模型目录不能在界面上保存"}
		p.setWriteState(state)
		return state
	}

	o.mu.Lock()
	count := len(editor.drafts)
	revision := editor.revision
	o.mu.Unlock()
	var last apiclient.ModelCatalogDocument
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
		document, err := upsert(payload, nil, revision)
		if err != nil {
			state := catalogWriteState(err)
			state.Path = []string{"models", strconv.Itoa(index)}
			p.setWriteState(state)
			p.reloadCatalogAfterFailure()
			return state
		}
		next := document.Revision
		revision = &next
		last = document
	}

	o.mu.Lock()
	orderChanged := false
	ids := []string(nil)
	if current := o.modelsCatalog; current != nil {
		ids = current.idsForWrite()
		orderChanged = current.orderChanged()
	}
	o.mu.Unlock()
	if orderChanged {
		if reorder == nil {
			state := WriteState{Kind: WriteFailed, NS: modelsCatalogNS,
				Message: "壳未接 models.catalog.reorder：模型顺序不能在界面上保存"}
			p.setWriteState(state)
			return state
		}
		// 宿主 Reorder 要的是**全排列**：数量对不上就别发（发了必然 bad_request，
		// 而且会让用户以为保存成功了一半）。
		if len(last.Models) != len(ids) {
			state := WriteState{Kind: WriteInvalid, NS: modelsCatalogNS, Path: []string{"models", "ids"},
				Code: apiclient.SettingsCodeBadRequest,
				Message: "目录里有 " + strconv.Itoa(len(last.Models)) + " 个模型，草稿有 " +
					strconv.Itoa(len(ids)) + " 条；宿主 reorder 要求完整排列，未提交顺序"}
			p.setWriteState(state)
			return state
		}
		document, err := reorder(ids, revision)
		if err != nil {
			state := catalogWriteState(err)
			state.Path = []string{"models", "ids"}
			p.setWriteState(state)
			p.reloadCatalogAfterFailure()
			return state
		}
		last = document
	}

	state := WriteState{Kind: WriteSaved, NS: modelsCatalogNS, Path: []string{"models"}}
	o.mu.Lock()
	if current := o.modelsCatalog; current != nil && current.generation == editor.generation {
		o.modelsCatalog = nil
		o.resetCatalogFields()
	}
	o.mu.Unlock()
	_ = last
	p.setWriteState(state)
	catalogInvalidateStored(o)
	return state
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
