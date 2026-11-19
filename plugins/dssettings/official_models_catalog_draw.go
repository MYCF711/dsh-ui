// official_models_catalog_draw.go —— 模型目录编辑卡的**行投影与绘制**。
//
// 行身份与几何都照官方（ModelsSection.module.css 的锚点写在每个常量上）：
//
//	.modelCatalog       列、gap 10、上方 0.5px 分隔线、上内缩 12   :466-473
//	.modelCatalogTitle  12/18、500、label-secondary               :481-486
//	.modelCatalogMeta   12/18、label-tertiary                     :488-494
//	.modelList          列、gap 8                                 :496-500
//	.modelEntry         0.5px border-l4、radius-lg、内缩 6         :509-515
//	.modelRow           行、gap 6                                 :517-522
//	.iconButton         28×28、radius-sm、透明、label-tertiary      :620-632
//	.iconButton:disabled 透明度 0.4                               :641-644
//	.modelAdvanced      两列、gap 8、内缩 2/4                      :654-660
//	.modelField         列、gap 4                                 :662-667
//	.modelInputChoices  行、gap 16、最小高 32                      :697-702
//	.modelEmpty         内缩 12、0.5px 虚线 border-l3、居中         :729-735
//	.addModelButton     28 高、0.5px border-l3、radius-sm、12/18    :737-755
//
// # 每个可交互控件的命中区（测试逐个断言，见 official_models_catalog_visual_test.go）
//
//	"调整模型顺序 N"（Control，键盘 ↑/↓ 各挪一格）       官方 ModelRow.tsx:104-130（含 :121-126 的方向键）
//	settings-catalog-remove-<key>                        删一行，ModelRow.tsx:142-151
//	settings-catalog-field-<key>-id                      官方 ModelRow.tsx:61-80
//	settings-catalog-field-<key>-name                    同上（名称列）
//	settings-catalog-toggle-<key>                        展开/收起，ModelRow.tsx:132-141
//	settings-catalog-field-<key>-baseURL                 API 地址，ProviderEditor.tsx:552-570
//	settings-catalog-apicycle-<key>                      API 协议（官方 `<select>`，:575-595）
//	settings-catalog-field-<key>-contextWindow / -maxTokens  两组容量，:154-169
//	settings-catalog-input-text-<key> / -image-<key>     输入类型两个复选，ModelInputTypes.tsx:36-58
//	settings-catalog-reason-<key>                        推理等级开关，ModelReasoningEfforts.tsx:171-176
//	settings-catalog-efflevel-<key>-<i>                  档位选择，:196-206
//	settings-catalog-field-<key>-effvalue-<i>            档位 API 值，:207-218
//	settings-catalog-effdel-<key>-<i>                    删档位，:219-228
//	settings-row-catalog-effadd-<key>                    添加档位，:234-242
//	settings-row-catalog-add                             添加模型，DeepSeekModelsEditor.tsx:245-253
//	settings-row-catalog-fetch                           获取可用模型，ModelListEditor.tsx:326-346
//	settings-catalog-cand-<i> / -candall                 候选勾选 / 全选，ModelListEditor.tsx:292-304
//	settings-row-catalog-adopt / -candclose              更新列表 / 关闭，ModelListEditor.tsx:270-290
//	settings-row-catalog-cancel / -catalog-apply         EditorFooter 两枚，EditorFooter.tsx:45-67
//
// 删除现在是**真按钮**：宿主 `dsh-go#82` 的 `models.catalog.replace` 给了整数组
// 写入（官方删行就是 splice + 整数组重写），所以它能落地，不再是禁用态。
package dssettings

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 编辑器几何（逐条锚在官方 ModelsSection.module.css，见文件头）。
const (
	catalogListGap        float32 = 8   // :496-500
	catalogEntryPad       float32 = 6   // :509-515
	catalogEntryBorder    float32 = 0.5 // :509-515
	catalogRowGap         float32 = 6   // :517-522
	catalogTitleSize      float32 = 12  // :481-486
	catalogTitleLine      float32 = 18  // :481-486
	catalogMetaSize       float32 = 12  // :488-494
	catalogMetaLine       float32 = 18  // :488-494
	catalogIconButtonSize float32 = 28  // :620-632
	catalogIconSize       float32 = 14  // ModelRow.tsx:126,148 的 size={14}
	catalogFieldGap       float32 = 4   // :662-667
	catalogChoicesGap     float32 = 16  // :697-702
	catalogChoicesMinH    float32 = 32  // :697-702
	catalogEmptyPad       float32 = 12  // :729-735
	catalogAddHeight      float32 = 28  // :737-755
	catalogAddPadX        float32 = 10  // :737-755
	catalogAddSize        float32 = 12  // :737-755
	catalogAddLine        float32 = 18  // :737-755
	catalogDisabledFade   float32 = 0.4 // :641-644
	catalogCheckboxSide   float32 = 16  // primitives Checkbox 的方盒
	catalogNoteSize       float32 = 12
	catalogNoteLine       float32 = 18
)

// catalogAPIDeepSeek 是官方 DeepSeek 适配器那一族的 api 名。
//
// 官方 baseURL 的 placeholder 先按**家族**分支（`family === 'deepseek'` 走
// `deepSeekBaseUrl`，ProviderEditor.tsx:558-560）；本卡是逐行编辑，家族由这一行
// 声明的 api 决定——`deepseek-completions` 就是宿主的 DeepSeek 适配器
// （`../DSH-GO/internal/llm/provider.go:37`）。
const catalogAPIDeepSeek = "deepseek-completions"

// 发现面（官方那个 Modal）在本卡里落成行：标题/说明/状态/候选/两枚动作。
const (
	catalogFetchRowID       = "catalog-fetch"        // 获取可用模型（官方 `.linkButton`）
	catalogDiscoverTitleID  = "catalog-fetch-title"  // fetchTitle
	catalogDiscoverDescID   = "catalog-fetch-desc"   // fetchDescription
	catalogDiscoverStatusID = "catalog-fetch-status" // 失败/空列表说明
	catalogCandidatePrefix  = "catalog-cand-"        // 一条候选，后缀 = 下标
	catalogCandidateAllID   = "catalog-candall"      // 全选/取消全选
	catalogAdoptRowID       = "catalog-adopt"        // 更新列表
	catalogCandCloseID      = "catalog-candclose"    // 关闭
)

// catalogBasePrefix / catalogAPIPrefix 是行内两格新字段（API 地址 / API 协议）。
const (
	catalogBasePrefix = "catalog-base-"
	catalogAPIPrefix  = "catalog-api-"
)

// catalogRowPath 是目录卡可点行的 Path 标记。它**不是设置路径**：目录行落到
// `activateModelsCatalogRow` 就返回了，不会进 applyRowWrite；只是 PanelRow.Writable
// 用 `len(Path) > 0` 判"这行能不能点"（official_describe.go:147）。
const catalogRowPath = "models.catalog"

// catalogCardRow 造卡片头行（编辑 / 收起）。
//
// 为什么头行是折叠钮而不是一进来就摊开：官方这一格在 provider 卡里、卡本身是折叠的
// （`ModelsSection.tsx` 的展开卡）；本页把 provider 卡做成行头折叠，目录编辑器复用
// 同一形状，免得一进 Models 页就是几十行长列表。
func catalogCardRow(edit *catalogEditor, snapshot *apiclient.ModelCatalogSnapshot) PanelRow {
	title := "编辑"
	if edit != nil {
		title = "收起"
	}
	desc := "宿主模型目录"
	switch {
	case snapshot.Loading:
		desc = "正在读取宿主模型目录…"
	case snapshot.Document == nil && snapshot.Error != "":
		desc = "宿主模型目录读取失败"
	case snapshot.Document != nil:
		desc = "宿主模型目录 · " + strconv.Itoa(len(snapshot.Document.Models)) + " 个模型"
	}
	// Path 非空 = 行可点（PanelRow.Writable 的判据，official_describe.go:147）。
	// 目录行不写设置文档，所以 Path 只是"这是可点行"的标记，落不到 applyRowWrite
	// （activateModelsCatalogRow 会先认领掉）。
	return PanelRow{ID: catalogCardRowID, Title: title, Desc: desc, Kind: ItemButton,
		Path: []string{catalogRowPath}}
}

// catalogRows 把一张开着的编辑卡投影成行。
func catalogRows(edit *catalogEditor, writable bool) []PanelRow {
	rows := []PanelRow{{ID: catalogTitleRowID, Title: catalogCopy("models"), Kind: ItemVersion}}
	meta := catalogCopy("modelsCustomized")
	if len(edit.drafts) == 0 {
		meta = catalogCopy("modelsInherited")
	}
	rows = append(rows, PanelRow{ID: catalogMetaRowID, Title: meta, Kind: ItemVersion})
	if len(edit.drafts) == 0 {
		rows = append(rows, PanelRow{ID: catalogEmptyRowID, Title: catalogCopy("modelsEmpty"), Kind: ItemVersion})
	}
	for index, key := range edit.keys {
		draft := edit.drafts[index]
		rows = append(rows, PanelRow{
			ID: catalogRowPrefix + key, Title: ModelText(draft, "id"), Desc: ModelText(draft, "name"),
			Kind: ItemButton, Disabled: !writable, Revision: index + 1,
		})
		if !edit.expanded[key] {
			continue
		}
		for _, field := range []string{"contextWindow", "maxTokens"} {
			rows = append(rows, PanelRow{
				ID: catalogCapacityPrefix + key + "-" + field, Title: catalogCopy(field),
				// Kind 必须不是 ItemSecret：drawModelRow 先把 ItemSecret 分流给 API
				// 密钥行（那才是 secret 语义），容量只是普通文本输入。
				Kind: ItemButton, Disabled: !writable, Revision: index + 1, Path: []string{catalogRowPath},
			})
		}
		// API 地址 / API 协议：官方这两格在 provider 卡的"自定义设置"里
		// （ProviderEditor.tsx:552-595），本仓宿主的 provider profile 只发布
		// retryPolicy（settingsform/provider_schema.go:164-176），baseURL/api 的
		// 真实消费点在**目录条目**上（internal/llm/model_catalog.go:1271-1290 +
		// ResolveProvider），所以画在行里。
		rows = append(rows, PanelRow{
			ID: catalogBasePrefix + key, Title: catalogCopy("baseUrl"),
			Kind: ItemButton, Disabled: !writable, Revision: index + 1, Path: []string{catalogRowPath},
		})
		rows = append(rows, PanelRow{
			ID: catalogAPIPrefix + key, Title: catalogCopy("customApi"),
			Kind: ItemButton, Disabled: !writable, Revision: index + 1, Path: []string{catalogRowPath},
		})
		rows = append(rows, PanelRow{
			ID: catalogInputsPrefix + key, Title: catalogCopy("modelInputTypes"),
			Kind: ItemSwitch, Disabled: !writable, Revision: index + 1,
		})
		rows = append(rows, PanelRow{
			ID: catalogReasonPrefix + key, Title: catalogCopy("modelReasoning"),
			Kind: ItemSwitch, Disabled: !writable, Revision: index + 1,
			Selected: strconv.FormatBool(edit.reasoningOn(key)),
		})
		for at := range edit.efforts[key] {
			rows = append(rows, PanelRow{
				ID: catalogEffortPrefix + key + "-" + strconv.Itoa(at), Title: catalogCopy("modelReasoningLevel"),
				Kind: ItemButton, Disabled: !writable, Revision: index + 1,
			})
		}
		if edit.reasoningOn(key) {
			rows = append(rows, PanelRow{
				ID: catalogEffortAddPrefix + key, Title: catalogCopy("modelReasoningAdd"),
				Kind: ItemButton, Disabled: !writable, Revision: index + 1,
			})
		}
	}
	rows = append(rows, PanelRow{ID: catalogAddRowID, Title: catalogCopy("addModel"), Kind: ItemButton,
		Disabled: !writable, Path: []string{catalogRowPath}})
	rows = append(rows, catalogFetchRows(edit, writable)...)
	rows = append(rows,
		PanelRow{ID: catalogCancelRowID, Title: zhEditorCancel, Kind: ItemButton, Path: []string{catalogRowPath}},
		PanelRow{ID: catalogApplyRowID, Title: zhEditorApply, Kind: ItemButton, Disabled: !writable,
			Path: []string{catalogRowPath}, Reason: catalogApplyReason(writable)},
	)
	return rows
}

// catalogFetchRows 是"获取可用模型"这一面（官方 `.linkButton` + Modal，
// ModelListEditor.tsx:326-346、:391-457）落在行投影上的形状。
//
// 三档可见性照官方：
//   - 常态只有那颗按钮（文案 fetchModels；在飞时 fetching）；
//   - 有失败原文 → 多一行说明（上游拒绝/`fetchEmpty` 都走这里）；
//   - 有候选 → 标题、说明、候选逐条、全选、更新列表、关闭。
func catalogFetchRows(edit *catalogEditor, writable bool) []PanelRow {
	_, _, askable := edit.askable()
	title := catalogCopy("fetchModels")
	if edit.discovering {
		title = catalogCopy("fetching")
	}
	reason := ""
	if !writable {
		reason = "只读：壳未接 models.catalog.replace"
	} else if !askable {
		reason = catalogCopy("fetchNeedsBaseUrl")
	}
	rows := []PanelRow{{
		ID: catalogFetchRowID, Title: title, Kind: ItemButton,
		Disabled: !writable || !askable || edit.discovering, Path: []string{catalogRowPath}, Reason: reason,
	}}
	if edit.discoverError != "" {
		rows = append(rows, PanelRow{ID: catalogDiscoverStatusID, Title: edit.discoverError, Kind: ItemVersion})
	}
	if edit.picker == nil || edit.candidates == nil {
		return rows
	}
	rows = append(rows,
		PanelRow{ID: catalogDiscoverTitleID, Title: catalogCopy("fetchTitle"), Kind: ItemVersion},
		PanelRow{ID: catalogDiscoverDescID, Title: catalogCopy("fetchDescription"), Kind: ItemVersion},
	)
	for index, candidate := range edit.candidates {
		id := ModelText(candidate, "id")
		desc := ""
		if name := ModelText(candidate, "name"); name != "" {
			desc = name
		}
		rows = append(rows, PanelRow{
			ID: catalogCandidatePrefix + strconv.Itoa(index), Title: id, Desc: desc,
			Kind: ItemSwitch, Selected: strconv.FormatBool(edit.picker.Picked(id)), Revision: index + 1,
		})
	}
	rows = append(rows, PanelRow{ID: catalogCandidateAllID, Title: catalogCopy("fetchSelectAll"), Kind: ItemButton})
	rows = append(rows,
		PanelRow{ID: catalogAdoptRowID, Title: catalogCopy("fetchAdopt"), Kind: ItemButton,
			Path: []string{catalogRowPath}},
		PanelRow{ID: catalogCandCloseID, Title: catalogCopy("close"), Kind: ItemButton,
			Path: []string{catalogRowPath}},
	)
	return rows
}

// catalogApplyReason 说明保存为什么被禁用。
func catalogApplyReason(writable bool) string {
	if writable {
		return ""
	}
	return "只读：壳未接 models.catalog.upsert / reorder"
}

// appendModelsCatalogEditor 把模型目录编辑卡追加到 Models 页，作为独立分组。
//
// snapshot 为 nil（壳没接 `models.catalog.*`）时整卡不出现：老壳上画一张"读取失败"
// 的卡只会让人以为宿主坏了，而真实情况是这条能力不存在。
func appendModelsCatalogEditor(panel PanelSection, snapshot *apiclient.ModelCatalogSnapshot, edit *catalogEditor, writable bool) PanelSection {
	if snapshot == nil {
		return panel
	}
	rows := []PanelRow{catalogCardRow(edit, snapshot)}
	if edit != nil {
		rows = append(rows, catalogRows(edit, writable)...)
	}
	panel.Rows = append(panel.Rows, rows...)
	panel.Groups = append(panel.Groups, PanelGroup{Title: catalogCopy("models"), Rows: rows})
	return panel
}

// catalogRowKey 从行 ID 里取行身份（`catalog-row-m3` → `m3`）。
func catalogRowKey(rowID, prefix string) string { return strings.TrimPrefix(rowID, prefix) }

// catalogSplitTail 把 `<key>-<tail>` 拆开（key 里不会有 `-`：它是 m1/m2… 这种身份）。
func catalogSplitTail(text string) (head, tail string, ok bool) {
	split := strings.LastIndex(text, "-")
	if split <= 0 || split == len(text)-1 {
		return "", "", false
	}
	return text[:split], text[split+1:], true
}

// drawCatalogRow 画编辑卡里的一行。
func (p *Plugin) drawCatalogRow(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow) {
	o := p.ensureOfficial()
	o.mu.Lock()
	edit := o.modelsCatalog
	o.mu.Unlock()
	if edit == nil {
		return
	}
	writable := edit.writable
	switch {
	case row.ID == catalogTitleRowID:
		ui.Text(f, row.Title).FontSize(catalogTitleSize).LineHeight(catalogTitleLine).
			Weight(500).Color(t.LabelSecondary).FillWidth().Ellipsis()
	case row.ID == catalogMetaRowID:
		ui.Text(f, row.Title).FontSize(catalogMetaSize).LineHeight(catalogMetaLine).
			Color(t.LabelTertiary).FillWidth().Ellipsis()
	case row.ID == catalogEmptyRowID:
		// `.modelEmpty`：内缩 12、0.5px 虚线 border-l3、居中（:729-735）。
		ui.Row(f).FillWidth().Pad(catalogEmptyPad).Border(catalogEntryBorder, t.BorderL3).Dashed().
			Radius(t.Radii.LG).Justify(ui.Center).Children(func() {
			ui.Text(f, row.Title).FontSize(catalogMetaSize).LineHeight(catalogMetaLine).Color(t.LabelTertiary)
		})
	case strings.HasPrefix(row.ID, catalogRowPrefix):
		p.drawCatalogEditorModelRow(f, env, t, row, edit, writable)
	case strings.HasPrefix(row.ID, catalogCapacityPrefix):
		p.drawCatalogCapacityRow(f, env, t, row, edit, writable)
	case strings.HasPrefix(row.ID, catalogBasePrefix):
		p.drawCatalogBaseURLRow(f, env, t, row, edit, writable)
	case strings.HasPrefix(row.ID, catalogAPIPrefix):
		p.drawCatalogAPIRow(f, env, t, row, edit, writable)
	case row.ID == catalogFetchRowID:
		p.drawCatalogLinkButton(f, env, t, row, !row.Disabled)
	case row.ID == catalogDiscoverStatusID:
		ui.Text(f, row.Title).FontSize(catalogMetaSize).LineHeight(catalogMetaLine).
			Color(t.LabelTertiary).FillWidth()
	case row.ID == catalogDiscoverTitleID:
		ui.Text(f, row.Title).FontSize(catalogTitleSize).LineHeight(catalogTitleLine).
			Weight(500).Color(t.LabelSecondary).FillWidth().Ellipsis()
	case row.ID == catalogDiscoverDescID:
		ui.Text(f, row.Title).FontSize(catalogMetaSize).LineHeight(catalogMetaLine).
			Color(t.LabelTertiary).FillWidth()
	case strings.HasPrefix(row.ID, catalogCandidatePrefix):
		p.drawCatalogCandidateRow(f, env, t, row, edit)
	case row.ID == catalogCandidateAllID:
		p.drawCatalogLinkButton(f, env, t, row, true)
	case row.ID == catalogAdoptRowID, row.ID == catalogCandCloseID:
		p.drawCatalogSmallButton(f, env, t, row, true)
	case strings.HasPrefix(row.ID, catalogInputsPrefix):
		p.drawCatalogInputTypesRow(f, env, t, row, edit, writable)
	case strings.HasPrefix(row.ID, catalogReasonPrefix):
		p.drawCatalogReasoningRow(f, env, t, row, edit, writable)
	case strings.HasPrefix(row.ID, catalogEffortAddPrefix):
		p.drawCatalogSmallButton(f, env, t, row, writable)
	case strings.HasPrefix(row.ID, catalogEffortPrefix):
		p.drawCatalogEffortRow(f, env, t, row, edit, writable)
	case row.ID == catalogAddRowID:
		p.drawCatalogSmallButton(f, env, t, row, writable)
	default:
		p.drawOfficialRow(f, env, t, row)
	}
}

// catalogIconButton 画一枚 28×28 的无字按钮（`.iconButton`，:620-632）。
//
// disabled 时**不挂命中区**并压到 0.4（:641-644）——一枚点得着却什么都不做的钮是
// 最坏的降级（本仓 hits_static_guard 的同一条精神）。
func catalogIconButton(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, name, label string, disabled bool, paint func()) bool {
	box := ui.Row(f).Shrink(0).Width(catalogIconButtonSize).Height(catalogIconButtonSize).
		Radius(t.Radii.SM).Items(ui.Center).Justify(ui.Center)
	clicked := false
	if disabled {
		box = box.Opacity(catalogDisabledFade)
	} else {
		box = box.Hit(env.Tree.ID(env.Feat, name), label).Hover(ui.Look{}.Background(t.Hover))
	}
	box.Children(paint)
	if !disabled {
		clicked = box.Clicked()
	}
	return clicked
}

// drawCatalogEditorModelRow 画一条模型：拖拽柄 / ID / 名称 / 展开 / 删除（ModelRow.tsx:102-183）。
//
// 官方网格是 handle | id(1.4fr) | name(1fr) | chevron | trash（`.modelRow` 的五列；
// 名字被显式放到第 3 列，:517-527），这里按同一顺序横排。
func (p *Plugin) drawCatalogEditorModelRow(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow, edit *catalogEditor, writable bool) {
	key := catalogRowKey(row.ID, catalogRowPrefix)
	index := edit.indexOfKey(key)
	if index < 0 {
		return
	}
	position := strconv.Itoa(index + 1)
	ui.Row(f).FillWidth().Pad(catalogEntryPad).Gap(catalogRowGap).Items(ui.Center).
		Border(catalogEntryBorder, t.BorderL4).Radius(t.Radii.LG).Children(func() {
		p.drawCatalogDragHandle(f, env, t, key, position, writable)
		p.catalogTextField(f, env, t, "settings-catalog-field-"+key+"-id", catalogCopy("modelId"),
			ModelText(edit.drafts[index], "id"), catalogCopy("modelId"), writable,
			func(text string) { p.catalogSetText(key, "id", text) })
		p.catalogTextField(f, env, t, "settings-catalog-field-"+key+"-name", catalogCopy("modelName"),
			ModelText(edit.drafts[index], "name"), catalogCopy("modelName"), writable,
			func(text string) { p.catalogSetText(key, "name", text) })
		// 展开/收起：官方那枚 chevron 钮（`aria-expanded`，ModelRow.tsx:130-139）。
		if catalogIconButton(f, env, t, "settings-catalog-toggle-"+key,
			catalogCopy("modelAdvanced")+" "+position, !writable, func() {
				ui.Icon(f, chevronOutlineIcon()).Size(12, 12).Color(t.LabelTertiary)
			}) {
			p.catalogToggleExpanded(env, key)
		}
		// 删除：真按钮（官方 `onRemove` → splice，ModelRow.tsx:142-151）。落地靠
		// 宿主 `models.catalog.replace` 的整数组写入，见 editor 文件顶部。
		if catalogIconButton(f, env, t, "settings-catalog-remove-"+key,
			catalogCopy("removeModel")+" "+position, !writable, func() {
				if icon, ok := dsassets.MarketTrashOutline(t.LabelTertiary); ok {
					ui.Icon(f, icon).Size(catalogIconSize, catalogIconSize).Color(t.LabelTertiary)
				}
			}) {
			p.catalogRemoveModel(env, key)
		}
	})
}

// drawCatalogDragHandle 画拖拽柄。
//
// 官方是 HTML5 拖放 + `onKeyDown` 的 ↑/↓（ModelRow.tsx:103-127，`aria-keyshortcuts=
// "ArrowUp ArrowDown"`），帮助文案也是"拖动排序，或使用上下方向键调整顺序"
// （locales.ts:233）。本栈没有 HTML5 拖放，但有等价的两件东西：`ui.Control` 是
// "可聚焦 + 收键盘"的元素（`ui/control.go:56-78`），而壳把按键**先**交给页面
// （`cmd/app/main.go:2291-2298` 的 `m.view.Key(ev)`，对应 `ui/page.go:716-721`
// 的"host routes its input with Key..."），页面再按焦点把键送给控件的
// `Handlers.Key`（`ui/page.go:445-450`）。所以官方那半条入口（柄上按 ↑/↓）如实保留，
// 由 TestModelsCatalogReorderHandleMovesDraftWithArrowKeys 用真按键钉住。
func (p *Plugin) drawCatalogDragHandle(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, key, position string, writable bool) {
	label := catalogCopy("reorderModel") + " " + position
	handlers := &ui.Handlers{Key: func(event gfx.KeyEvent) gfx.EventResult {
		if !writable {
			return gfx.EventResult{}
		}
		offset := 0
		switch event.Key {
		case gfx.KeyUp:
			offset = -1
		case gfx.KeyDown:
			offset = 1
		default:
			return gfx.EventResult{}
		}
		p.catalogMoveRow(env, key, offset)
		return gfx.EventResult{Handled: true, Repaint: true}
	}}
	handle := ui.Control(f, label, handlers).Shrink(0).
		Width(catalogIconButtonSize).Height(catalogIconButtonSize).Radius(t.Radii.SM)
	if writable {
		handle = handle.Hover(ui.Look{}.Background(t.Hover))
	} else {
		handle = handle.Opacity(catalogDisabledFade)
	}
	icon, _ := catalogDragHandleIcon()
	tint := t.LabelTertiary
	handle.Draw(func(ctx *ui.DrawContext) {
		inset := (ctx.Rect.Width - catalogIconSize) / 2
		ctx.Canvas.DrawIcon(icon, gfx.Rect{
			X: ctx.Rect.X + inset, Y: ctx.Rect.Y + inset, Width: catalogIconSize, Height: catalogIconSize,
		}, tint)
	})
}

// catalogBaseURLPlaceholder 是 API 地址那格的占位文案（官方三个分支，
// ProviderEditor.tsx:558-561 + locales.ts:236-241）：
//
//	DeepSeek 家族            → deepSeekBaseUrl（含默认端点与 DEEPSEEK_BASE_URL 说明）
//	路由声明了默认端点       → baseUrlPlaceholder，`{url}` 换成那条端点
//	路由只有环境变量         → baseUrlConfigured
//	两者都没有               → baseUrlDefault（"没有默认地址"）
//
// 家族与路由事实都取宿主报的值（listConfigurableProviders 的 defaultBaseURL /
// baseURLEnv，`../DSH-GO/internal/llm/provider_registry.go:35-40`），不猜。
func catalogBaseURLPlaceholder(edit *catalogEditor, draft ModelDraft) string {
	api := strings.TrimSpace(ModelText(draft, "api"))
	if api == catalogAPIDeepSeek {
		return catalogCopy("deepSeekBaseUrl")
	}
	if edit != nil {
		if value := edit.providerDefault[api]; value != "" {
			return strings.ReplaceAll(catalogCopy("baseUrlPlaceholder"), "{url}", value)
		}
		if len(edit.providerEnv[api]) > 0 {
			return catalogCopy("baseUrlConfigured")
		}
	}
	return catalogCopy("baseUrlDefault")
}

// drawCatalogBaseURLRow 画 API 地址（官方 `.field` + `.input`，ProviderEditor.tsx:552-570；
// 几何同本卡其它字段：label 12/18 tertiary + 32 高输入框）。
func (p *Plugin) drawCatalogBaseURLRow(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow,
	edit *catalogEditor, writable bool) {
	key := catalogRowKey(row.ID, catalogBasePrefix)
	index := edit.indexOfKey(key)
	if index < 0 {
		return
	}
	ui.Column(f).FillWidth().Gap(catalogFieldGap).Children(func() {
		ui.Text(f, row.Title).FontSize(catalogMetaSize).LineHeight(catalogMetaLine).Color(t.LabelTertiary)
		// 官方 `aria-label={t('baseUrl')}`（ProviderEditor.tsx:563）。
		p.catalogTextField(f, env, t, "settings-catalog-field-"+key+"-baseURL", row.Title,
			ModelText(edit.drafts[index], "baseURL"), catalogBaseURLPlaceholder(edit, edit.drafts[index]),
			writable, func(text string) { p.catalogSetBaseURL(key, text) })
	})
}

// drawCatalogAPIRow 画 API 协议。官方是 `<select>`（ProviderEditor.tsx:575-595），
// 候选 = 宿主注册的 wire 协议；未选择时显示官方 `customApiUnset`（"未选择"）。
//
// 本仓行词汇里"点一下换下一个候选"是同一种语义（与档位行 `settings-catalog-efflevel-*`
// 同一手法，见 drawCatalogEffortRow）。
func (p *Plugin) drawCatalogAPIRow(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow,
	edit *catalogEditor, writable bool) {
	key := catalogRowKey(row.ID, catalogAPIPrefix)
	index := edit.indexOfKey(key)
	if index < 0 {
		return
	}
	if len(edit.protocols) == 0 {
		return
	}
	current := strings.TrimSpace(ModelText(edit.drafts[index], "api"))
	label := current
	if label == "" {
		label = catalogCopy("customApiUnset")
	}
	ui.Column(f).FillWidth().Gap(catalogFieldGap).Children(func() {
		ui.Text(f, row.Title).FontSize(catalogMetaSize).LineHeight(catalogMetaLine).Color(t.LabelTertiary)
		pill := ui.Row(f).Shrink(0).Height(catalogChoicesMinH).Radius(t.Radii.MD).
			PadX(ModelsKeyPadX).Gap(SelectorGap).Items(ui.Center).Background(settingsModuleFill(t))
		if writable {
			pill = pill.Hit(env.Tree.ID(env.Feat, "settings-catalog-apicycle-"+key), row.Title)
		} else {
			pill = pill.Opacity(catalogDisabledFade)
		}
		pill.Children(func() {
			ui.Text(f, protocolLabelText(label)).FontSize(ModelsRowNameSize).
				LineHeight(ModelsRowNameLine).Color(t.LabelPrimary)
			ui.Icon(f, chevronOutlineIcon()).Size(SelectorChevron, SelectorChevron).Color(t.LabelTertiary)
		})
		if writable && pill.Clicked() {
			p.catalogCycleAPI(env, key)
		}
	})
}

// protocolLabelText 是协议名的产品名（官方 `protocolLabel`，protocol-label.ts:1-37）。
//
// 已命名四个之外的值原样返回 api 名——官方也是这条规则（"适配器先加的协议显示
// 标识符"），因为 settings 里存的就是那个标识符。
func protocolLabelText(api string) string {
	switch api {
	case "openai-completions":
		return catalogCopy("protocolOpenAiCompletions")
	case "openai-responses":
		return catalogCopy("protocolOpenAiResponses")
	case "anthropic-messages":
		return catalogCopy("protocolAnthropicMessages")
	case "google-generative-ai":
		return catalogCopy("protocolGemini")
	default:
		return api
	}
}

// drawCatalogLinkButton 画官方 `.linkButton`（ModelListEditor.tsx:326-346 的"获取可用模型"
// 与 :288-302 的"全选"）。几何沿用本卡小按钮的 28 高/12/18，不加新数值。
func (p *Plugin) drawCatalogLinkButton(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow, enabled bool) {
	box := ui.Row(f).Shrink(0).Height(catalogAddHeight).Gap(catalogFieldGap).
		PadSides(0, catalogAddPadX, 0, catalogAddPadX).Radius(t.Radii.SM).Items(ui.Center)
	if enabled {
		box = box.Hit(env.Tree.ID(env.Feat, "settings-row-"+row.ID), row.Title).Hover(ui.Look{}.Background(t.Hover))
	} else {
		box = box.Opacity(catalogDisabledFade)
	}
	box.Children(func() {
		ui.Text(f, row.Title).FontSize(catalogAddSize).LineHeight(catalogAddLine).Color(t.BrandPrimary)
	})
	if enabled && box.Clicked() {
		p.activateCatalogRow(row)
	}
}

// drawCatalogCandidateRow 画一条候选（官方 Modal 里的复选框行，
// ModelListEditor.tsx:391-440）。勾选态来自 ModelPicker（默认值随列表实时推）。
func (p *Plugin) drawCatalogCandidateRow(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow, edit *catalogEditor) {
	if edit.picker == nil {
		return
	}
	index, err := strconv.Atoi(catalogRowKey(row.ID, catalogCandidatePrefix))
	if err != nil || index < 0 || index >= len(edit.candidates) {
		return
	}
	picked := edit.picker.Picked(ModelText(edit.candidates[index], "id"))
	body := ui.Row(f).FillWidth().Gap(catalogRowGap).Items(ui.Center)
	body = body.Hit(env.Tree.ID(env.Feat, "settings-catalog-cand-"+strconv.Itoa(index)), row.Title).
		Hover(ui.Look{}.Background(t.Hover))
	body.Children(func() {
		mark := ui.Row(f).Shrink(0).Size(catalogCheckboxSide, catalogCheckboxSide).
			Radius(t.Radii.SM).Items(ui.Center).Justify(ui.Center)
		if picked {
			mark = mark.Background(t.BrandPrimary)
		} else {
			mark = mark.Border(catalogEntryBorder, t.BorderL3)
		}
		mark.Children(func() {
			if !picked {
				return
			}
			if commands, ok := dsassets.CheckOutline(); ok {
				ui.Icon(f, gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}).
					Size(12, 12).Color(t.ButtonPrimaryFg)
			}
		})
		ui.Text(f, row.Title).FontSize(ModelsRowNameSize).LineHeight(ModelsRowNameLine).
			Color(t.LabelPrimary).Grow(1).Ellipsis()
		if row.Desc != "" {
			ui.Text(f, row.Desc).FontSize(catalogMetaSize).LineHeight(catalogMetaLine).
				Color(t.LabelTertiary).Ellipsis()
		}
	})
	if body.Clicked() {
		p.catalogToggleCandidate(ModelText(edit.candidates[index], "id"))
	}
}

// catalogRemoveModel 删一行草稿（真按钮的落点）。
func (p *Plugin) catalogRemoveModel(env *pluginapi.Env, key string) {
	o := p.ensureOfficial()
	o.mu.Lock()
	removed := false
	if edit := o.modelsCatalog; edit != nil {
		removed = edit.removeModel(key)
	}
	if removed {
		// 行没了，它的输入框登记也必须忘掉：留着会把上一份文本灌到占用同一
		// 前缀的另一行上（`resetCatalogFields` 是全清，这里只清该行的键）。
		o.forgetCatalogRowFields(key)
	}
	o.mu.Unlock()
	if removed {
		catalogInvalidate(env)
	}
}

// forgetCatalogRowFields 丢掉某一行的输入框登记（删行/换行身份时调）。
func (o *officialPanel) forgetCatalogRowFields(key string) {
	if o.catalogFields == nil || o.catalogSeen == nil {
		return
	}
	suffix := "-" + key
	for name := range o.catalogFields {
		if strings.HasSuffix(name, suffix) || strings.Contains(name, suffix+"-") {
			delete(o.catalogFields, name)
			delete(o.catalogSeen, name)
		}
	}
}

// catalogCycleAPI 换一行的协议（官方 `<select>` 的等价动作）。
func (p *Plugin) catalogCycleAPI(env *pluginapi.Env, key string) {
	o := p.ensureOfficial()
	o.mu.Lock()
	changed := false
	if edit := o.modelsCatalog; edit != nil {
		if index := edit.indexOfKey(key); index >= 0 {
			current := strings.TrimSpace(ModelText(edit.drafts[index], "api"))
			if next, ok := edit.nextAPI(current); ok {
				changed = edit.setAPI(key, next)
			}
		}
	}
	o.mu.Unlock()
	if changed {
		catalogInvalidate(env)
	}
}

// catalogSetBaseURL 改一行的 API 地址（空白 = 删键，官方 ProviderEditor.tsx:565-567）。
func (p *Plugin) catalogSetBaseURL(key, text string) {
	o := p.ensureOfficial()
	o.mu.Lock()
	if edit := o.modelsCatalog; edit != nil {
		edit.setBaseURL(key, text)
	}
	o.mu.Unlock()
}

// drawCatalogCapacityRow 画一组容量输入（label 12/18 + 32 高输入框，:274-281/:808-820）。
func (p *Plugin) drawCatalogCapacityRow(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow, edit *catalogEditor, writable bool) {
	key, field, ok := catalogSplitTail(catalogRowKey(row.ID, catalogCapacityPrefix))
	if !ok {
		return
	}
	index := edit.indexOfKey(key)
	if index < 0 {
		return
	}
	// 官方 placeholder：行里没写容量时显示宿主内置默认（`CAPACITY_HINT`，
	// ModelListEditor.tsx:122-125 → #178 的 ModelCapacityHint）。
	placeholder := ModelCapacityHint(field)
	current := ""
	if count, known := modelCapacityNumber(edit.drafts[index][field]); known {
		current = FormatModelCapacity(count)
	}
	ui.Column(f).FillWidth().Gap(catalogFieldGap).Children(func() {
		ui.Text(f, row.Title).FontSize(catalogMetaSize).LineHeight(catalogMetaLine).Color(t.LabelTertiary)
		p.catalogTextField(f, env, t, "settings-catalog-field-"+key+"-"+field, row.Title, current, placeholder, writable,
			func(text string) { p.catalogSetCapacity(key, field, text) })
	})
}

// drawCatalogInputTypesRow 画输入类型两个复选（ModelInputTypes.tsx:36-59）。
func (p *Plugin) drawCatalogInputTypesRow(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow, edit *catalogEditor, writable bool) {
	key := catalogRowKey(row.ID, catalogInputsPrefix)
	index := edit.indexOfKey(key)
	if index < 0 {
		return
	}
	selected := catalogSelectedInputs(edit.drafts[index])
	ui.Column(f).FillWidth().Gap(catalogFieldGap).Children(func() {
		ui.Text(f, row.Title).FontSize(catalogMetaSize).LineHeight(catalogMetaLine).Color(t.LabelTertiary)
		ui.Row(f).FillWidth().Gap(catalogChoicesGap).Items(ui.Center).MinHeight(catalogChoicesMinH).Children(func() {
			for _, modality := range []string{ModelInputText, ModelInputImage} {
				label := catalogCopy("modelInputText")
				if modality == ModelInputImage {
					label = catalogCopy("modelInputImage")
				}
				checked := containsString(selected, modality)
				// 官方：只剩这一项且已选中时不能取消（ModelInputTypes.tsx:44），并画 0.4。
				locked := ModelInputTypeLocked(selected, modality)
				enabled := writable && !locked
				box := ui.Row(f).Shrink(0).Gap(catalogRowGap).Items(ui.Center)
				if enabled {
					box = box.Hit(env.Tree.ID(env.Feat, "settings-catalog-input-"+modality+"-"+key), label)
				} else {
					box = box.Opacity(catalogDisabledFade)
				}
				clicked := false
				box.Children(func() {
					mark := ui.Row(f).Shrink(0).Size(catalogCheckboxSide, catalogCheckboxSide).
						Radius(t.Radii.SM).Items(ui.Center).Justify(ui.Center)
					if checked {
						mark = mark.Background(t.BrandPrimary)
					} else {
						mark = mark.Border(catalogEntryBorder, t.BorderL3)
					}
					mark.Children(func() {
						if !checked {
							return
						}
						if commands, ok := dsassets.CheckOutline(); ok {
							ui.Icon(f, gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}).
								Size(12, 12).Color(t.ButtonPrimaryFg)
						}
					})
					ui.Text(f, label).FontSize(ModelsRowNameSize).LineHeight(ModelsRowNameLine).Color(t.LabelPrimary)
				})
				clicked = enabled && box.Clicked()
				if clicked {
					p.catalogToggleInput(env, key, modality, !checked)
				}
			}
		})
	})
}

// drawCatalogReasoningRow 画推理等级开关（ModelReasoningEfforts.tsx:163-177 的头）。
func (p *Plugin) drawCatalogReasoningRow(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow, edit *catalogEditor, writable bool) {
	key := catalogRowKey(row.ID, catalogReasonPrefix)
	on := edit.reasoningOn(key)
	ui.Row(f).FillWidth().Gap(catalogRowGap).Items(ui.Center).Children(func() {
		ui.Text(f, row.Title).FontSize(catalogMetaSize).LineHeight(catalogMetaLine).
			Color(t.LabelTertiary).Grow(1)
		bg := t.BorderL3
		justify := ui.Start
		thumb := t.BgLayer2
		if on {
			bg = t.BrandPrimary
			justify = ui.End
			thumb = t.ButtonPrimaryFg
		}
		track := ui.Row(f).Shrink(0).Width(SwitchWidth).Height(SwitchHeight).Radius(SwitchHeight/2).
			Background(bg).PadSides(SwitchPad, SwitchPad, SwitchPad, SwitchPad).Justify(justify)
		if writable {
			track = track.Hit(env.Tree.ID(env.Feat, "settings-catalog-reason-"+key), row.Title)
		} else {
			track = track.Opacity(catalogDisabledFade)
		}
		track.Children(func() {
			ui.Row(f).Width(SwitchThumbSize).Height(SwitchThumbSize).
				Radius(SwitchThumbSize / 2).Background(thumb)
		})
		if writable && track.Clicked() {
			p.catalogSetReasoning(env, key, !on)
		}
	})
}

// drawCatalogEffortRow 画一个档位行：档位选择 / API 参数值 / 删除
// （官方 `.modelReasoningRow` 三列，ModelReasoningEfforts.tsx:190-229）。
func (p *Plugin) drawCatalogEffortRow(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow, edit *catalogEditor, writable bool) {
	key, rawIndex, ok := catalogSplitTail(catalogRowKey(row.ID, catalogEffortPrefix))
	if !ok {
		return
	}
	index, err := strconv.Atoi(rawIndex)
	if err != nil || index < 0 || index >= len(edit.efforts[key]) {
		return
	}
	entry := edit.efforts[key][index]
	valueText := ""
	if text, isString := entry.Value.(string); isString {
		valueText = text
	}
	placeholder := entry.Level
	if entry.Level == "off" {
		placeholder = catalogCopy("modelReasoningOffValuePlaceholder")
	}
	ui.Row(f).FillWidth().Gap(catalogRowGap).Items(ui.Center).Children(func() {
		// 档位选择：官方是 `<select>`（:196-206）；本仓行词汇里"点一下换下一个候选"
		// 是同一种语义，候选 = 还没被别的行用掉的档位。
		pill := ui.Row(f).Shrink(0).Height(catalogChoicesMinH).Radius(t.Radii.MD).
			PadX(ModelsKeyPadX).Gap(SelectorGap).Items(ui.Center).Background(settingsModuleFill(t))
		if writable {
			pill = pill.Hit(env.Tree.ID(env.Feat, "settings-catalog-efflevel-"+key+"-"+rawIndex), row.Title)
		} else {
			pill = pill.Opacity(catalogDisabledFade)
		}
		pill.Children(func() {
			ui.Text(f, entry.Level).FontSize(ModelsRowNameSize).LineHeight(ModelsRowNameLine).Color(t.LabelPrimary)
			ui.Icon(f, chevronOutlineIcon()).Size(SelectorChevron, SelectorChevron).Color(t.LabelTertiary)
		})
		if writable && pill.Clicked() {
			p.catalogCycleEffortLevel(env, key, index)
		}
		// API 参数值：off 那一档不可编辑（官方 `disabled={disabled || level === 'off'}`，
		// :215）。
		p.catalogTextField(f, env, t, "settings-catalog-field-"+key+"-effvalue-"+rawIndex,
			row.Title, valueText, placeholder, writable && entry.Level != "off",
			func(text string) { p.catalogSetEffortValue(env, key, index, text) })
		if catalogIconButton(f, env, t, "settings-catalog-effdel-"+key+"-"+rawIndex,
			catalogCopy("modelReasoningRemove")+" "+entry.Level, !writable, func() {
				if icon, ok := dsassets.MarketTrashOutline(t.LabelTertiary); ok {
					ui.Icon(f, icon).Size(catalogIconSize, catalogIconSize).Color(t.LabelTertiary)
				}
			}) {
			p.catalogRemoveEffort(env, key, index)
		}
	})
}

// drawCatalogSmallButton 画官方 `.addModelButton`（28 高、0.5px border-l3、radius-sm、
// 12/18）那一族小按钮：添加模型 / 添加档位。
func (p *Plugin) drawCatalogSmallButton(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow, enabled bool) {
	body := ui.Row(f).Shrink(0).Height(catalogAddHeight).Gap(catalogFieldGap).
		PadSides(0, catalogAddPadX, 0, catalogAddPadX).Radius(t.Radii.SM).
		Border(catalogEntryBorder, t.BorderL3).Items(ui.Center)
	if enabled {
		body = body.Hit(env.Tree.ID(env.Feat, "settings-row-"+row.ID), row.Title).Hover(ui.Look{}.Background(t.Hover))
	} else {
		body = body.Opacity(catalogDisabledFade)
	}
	body.Children(func() {
		if icon, ok := dsassets.MarketPlusOutline(t.LabelPrimary); ok {
			ui.Icon(f, icon).Size(catalogIconSize, catalogIconSize).Color(t.LabelPrimary)
		}
		ui.Text(f, row.Title).FontSize(catalogAddSize).LineHeight(catalogAddLine).Color(t.LabelPrimary)
	})
	if enabled && body.Clicked() {
		p.activateCatalogRow(row)
	}
}

// catalogTextField 登记/复用一枚输入框，并把它的文本折回草稿。
//
// 为什么 Field 必须常驻（存在 panel 上而不是每帧新建）：Viva 的 Field 是输入法、
// 光标与选区的宿主，只能有一份（`ui/field.go` 的 Field 注释；本仓 `modelsKeyField`
// 同一约束）。
//
// 折回规则：**Field 是草稿文本的真值**，只有它变了才写回 editor（与
// `modelsKeyLastDraft` 同一手法），所以用户打字的半截内容不会被每帧重算覆盖。
//
// label 是这枚输入框的无障碍名（官方每个 `<input>` 都带 `aria-label`，
// 例如 `t('baseUrl')`，ProviderEditor.tsx:563）；占位文案会随宿主事实变化，
// 不能拿来当名字。
func (p *Plugin) catalogTextField(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, name, label, current, placeholder string,
	writable bool, apply func(text string)) {
	o := p.ensureOfficial()
	o.mu.Lock()
	field := o.catalogField(name)
	seen, hasSeen := o.catalogSeen[name]
	o.mu.Unlock()
	if !hasSeen {
		// 首次出现：把草稿值灌进 Field（只能在窗口线程做，本函数就是）。
		field.SetText(current)
		o.mu.Lock()
		o.catalogSeen[name] = current
		o.mu.Unlock()
		return
	}
	if text := field.Text(); text != seen {
		o.mu.Lock()
		o.catalogSeen[name] = text
		o.mu.Unlock()
		apply(text)
	}
	box := ui.TextFieldID(f, env.Tree.ID(env.Feat, name), field, placeholder, label).
		FillWidth().Height(ModelsInputMinHeight).FontSize(ModelsRowNameSize).LineHeight(ModelsRowNameLine).
		PadX(ModelsKeyPadX).Border(catalogEntryBorder, t.BorderL4).Radius(t.Radii.MD).
		Background(t.BgLayer1)
	if !writable {
		box.Disabled(true).Opacity(catalogDisabledFade)
	}
}

// catalogField 取（必要时创建）一枚输入框。只在窗口线程调用。
func (o *officialPanel) catalogField(name string) *ui.Field {
	if o.catalogFields == nil {
		o.catalogFields = map[string]*ui.Field{}
	}
	if o.catalogSeen == nil {
		o.catalogSeen = map[string]string{}
	}
	field, ok := o.catalogFields[name]
	if !ok {
		field = &ui.Field{}
		o.catalogFields[name] = field
	}
	return field
}

// resetCatalogFields 丢弃所有输入框的登记（关卡/换卡时调，窗口线程）。
//
// 不丢的话，下一次开卡会用上一份草稿的文本做初值——那是"看着对、其实写错卡片"的
// 最短路径（与 `discardModelsKeyDraft` 同一类风险）。
func (o *officialPanel) resetCatalogFields() {
	o.catalogFields = nil
	o.catalogSeen = nil
}

// catalogSetText 写一行草稿的字符串字段。空串 = **删键**（官方
// `field === 'name' && value === ” ? undefined`，ModelRow.tsx:74）。
func (p *Plugin) catalogSetText(key, field, text string) {
	o := p.ensureOfficial()
	o.mu.Lock()
	defer o.mu.Unlock()
	edit := o.modelsCatalog
	if edit == nil {
		return
	}
	if index := edit.indexOfKey(key); index >= 0 {
		edit.drafts[index] = catalogPatch(edit.drafts[index], map[string]any{field: text})
	}
}

// catalogSetCapacity 把一段容量文本折回草稿（#178 的三态，官方
// DeepSeekModelsEditor.tsx:35-47、:190-195）。
//
//	空串    → 删键（继承，不写值）
//	读不出  → 写 NaN，保存期校验点名（官方也是把 NaN 放进草稿再校验）
//	读得出  → 写十进制计数
func (p *Plugin) catalogSetCapacity(key, field, text string) {
	capacity := ParseModelCapacity(text)
	o := p.ensureOfficial()
	o.mu.Lock()
	defer o.mu.Unlock()
	edit := o.modelsCatalog
	if edit == nil {
		return
	}
	index := edit.indexOfKey(key)
	if index < 0 {
		return
	}
	switch {
	case capacity.Blank:
		next := make(ModelDraft, len(edit.drafts[index]))
		for name, value := range edit.drafts[index] {
			next[name] = value
		}
		delete(next, field)
		edit.drafts[index] = next
	case capacity.Unreadable:
		edit.drafts[index] = catalogPatch(edit.drafts[index], map[string]any{field: math.NaN()})
	default:
		edit.drafts[index] = catalogPatch(edit.drafts[index], map[string]any{field: capacity.Value})
	}
}

// catalogToggleExpanded 翻转一行的展开态（官方 `rowState.toggle`，useModelRows.ts:92-94）。
func (p *Plugin) catalogToggleExpanded(env *pluginapi.Env, key string) {
	o := p.ensureOfficial()
	o.mu.Lock()
	if o.modelsCatalog != nil {
		o.modelsCatalog.toggleExpanded(key)
	}
	o.mu.Unlock()
	catalogInvalidate(env)
}

// catalogToggleInput 应用一次输入类型勾选。
func (p *Plugin) catalogToggleInput(env *pluginapi.Env, key, modality string, checked bool) {
	o := p.ensureOfficial()
	o.mu.Lock()
	edit := o.modelsCatalog
	if edit != nil {
		if index := edit.indexOfKey(key); index >= 0 {
			if next, ok := toggleCatalogInput(edit.drafts[index], modality, checked); ok {
				edit.drafts[index] = next
			}
		}
	}
	o.mu.Unlock()
	catalogInvalidate(env)
}

// catalogSetReasoning 打开/关闭一行的推理等级。
func (p *Plugin) catalogSetReasoning(env *pluginapi.Env, key string, on bool) {
	o := p.ensureOfficial()
	o.mu.Lock()
	if o.modelsCatalog != nil {
		o.modelsCatalog.setReasoning(key, on)
	}
	o.mu.Unlock()
	catalogInvalidate(env)
}

// catalogCycleEffortLevel 换一个档位行的档位。
func (p *Plugin) catalogCycleEffortLevel(env *pluginapi.Env, key string, index int) {
	o := p.ensureOfficial()
	o.mu.Lock()
	if o.modelsCatalog != nil {
		o.modelsCatalog.cycleEffortLevel(key, index)
	}
	o.mu.Unlock()
	catalogInvalidate(env)
}

// catalogRemoveEffort 删掉一个档位行。
func (p *Plugin) catalogRemoveEffort(env *pluginapi.Env, key string, index int) {
	o := p.ensureOfficial()
	o.mu.Lock()
	if o.modelsCatalog != nil {
		o.modelsCatalog.removeEffort(key, index)
	}
	o.mu.Unlock()
	catalogInvalidate(env)
}

// catalogSetEffortValue 改一个档位的 API 参数值。
func (p *Plugin) catalogSetEffortValue(env *pluginapi.Env, key string, index int, text string) {
	o := p.ensureOfficial()
	o.mu.Lock()
	if o.modelsCatalog != nil {
		o.modelsCatalog.setEffortValue(key, index, text)
	}
	o.mu.Unlock()
	catalogInvalidate(env)
}

// catalogMoveRow 把一行挪 offset 格（官方 `onMove(±1)`，ModelRow.tsx:123）。
func (p *Plugin) catalogMoveRow(env *pluginapi.Env, key string, offset int) {
	o := p.ensureOfficial()
	o.mu.Lock()
	moved := false
	if edit := o.modelsCatalog; edit != nil {
		if index := edit.indexOfKey(key); index >= 0 {
			moved = edit.moveRow(index, index+offset)
		}
	}
	o.mu.Unlock()
	if moved {
		catalogInvalidate(env)
	}
}

// catalogInvalidate 请求下一帧重画（绘制帧里的状态变化必须显式要一帧）。
func catalogInvalidate(env *pluginapi.Env) {
	if env != nil && env.Act.Invalidate != nil {
		env.Act.Invalidate()
	}
}

// modelCapacityNumber 把草稿里的容量读成数值（字符串不算，交给校验点名）。
func modelCapacityNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return 0, false
		}
		return typed, true
	case json.Number:
		parsed, err := strconv.ParseFloat(typed.String(), 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

// catalogDragHandleIcon 是官方 `IconDragHandleOutlineRegular`（三条横线）。
//
// 素材出处：`ui-primitives/src/icons/artwork.ts` 的 `'drag-handle-outline'`
// （viewBox 0 0 16 16、`stroke-linecap: round`、stroke-width = regular 1）：
//
//	<path d="M4 5h8M4 8h8M4 11h8">
//
// 逐段转写（与 `icons.go` 的三段转写同一规则，无近似）。
func catalogDragHandleIcon() (gfx.Icon, bool) {
	return officialVectorIcon(officialRegularStroke, gfx.Color{},
		officialIconPart{d: "M4 5h8"},
		officialIconPart{d: "M4 8h8"},
		officialIconPart{d: "M4 11h8"},
	), true
}
