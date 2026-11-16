// settings_items_subagent.go —— 官方 #46「模型选择」的数据模型与草稿状态机。
//
// 权威源：ui-settings-subagent/src/client/subagent-model-selection-card-controller.ts。
// allowedModels 不走 sections[].fields（那里只有标量），而走 SettingsNamespaceView.Value；
// 宿主已在 host_settings_pages_test.go:257-271 证明同一次 mutate 写两字段并可从 Value 读回。
package dssidebar

import (
	"encoding/json"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// settingsSubagentModelRoute 是 Host 保存的一条精确 provider/model 授权路由。
// JSON 字段与官方 AllowedSubagentModel 逐字一致（controller:11-23）。
type settingsSubagentModelRoute struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// settingsSubagentModelDraft 是整个模型选择 section 的一份草稿。
// Routes 用切片而不是 map：官方 Map 保留插入顺序，新增候选追加、取消仅删除该项。
type settingsSubagentModelDraft struct {
	Enabled bool
	Routes  []settingsSubagentModelRoute
}

func cloneSettingsSubagentModelDraft(in *settingsSubagentModelDraft) *settingsSubagentModelDraft {
	if in == nil {
		return nil
	}
	out := *in
	out.Routes = cloneSettingsSubagentModelRoutes(in.Routes)
	return &out
}

func cloneSettingsSubagentModelRoutes(in []settingsSubagentModelRoute) []settingsSubagentModelRoute {
	if in == nil {
		return nil
	}
	out := make([]settingsSubagentModelRoute, len(in))
	copy(out, in)
	return out
}

// settingsSubagentModelCandidate 是目录行与已保存路由的 join 结果（controller:25-37）。
type settingsSubagentModelCandidate struct {
	settingsSubagentModelRoute
	Key          string
	ProviderName string
	ModelName    string
	Available    bool
	Selected     bool
}

// settingsSubagentCatalogStatus 与官方 catalogStatus 四态同名。
type settingsSubagentCatalogStatus string

const (
	settingsSubagentCatalogIdle    settingsSubagentCatalogStatus = "idle"
	settingsSubagentCatalogLoading settingsSubagentCatalogStatus = "loading"
	settingsSubagentCatalogReady   settingsSubagentCatalogStatus = "ready"
	settingsSubagentCatalogError   settingsSubagentCatalogStatus = "error"
)

// settingsSubagentModelState 是模型选择 section 的完整呈现状态。
type settingsSubagentModelState struct {
	Available      bool
	Known          bool
	Writable       bool
	Revision       int
	Enabled        bool
	CurrentEnabled bool
	CurrentRoutes  []settingsSubagentModelRoute
	DesiredRoutes  []settingsSubagentModelRoute
	Candidates     []settingsSubagentModelCandidate
	CatalogStatus  settingsSubagentCatalogStatus
	CatalogPartial bool
	CatalogRetry   bool
	Dirty          bool
	Invalid        bool
	Conflicted     bool
}

// settingsSubagentModelValue 只投影官方页面读取的两个字段；其它原始字段不 invented。
type settingsSubagentModelValue struct {
	Enabled       bool                         `json:"enabled"`
	AllowedModels []settingsSubagentModelRoute `json:"allowedModels"`
}

func decodeSettingsSubagentModelValue(raw json.RawMessage) (settingsSubagentModelValue, error) {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	var value settingsSubagentModelValue
	if err := json.Unmarshal(raw, &value); err != nil {
		return settingsSubagentModelValue{}, err
	}
	if value.AllowedModels == nil {
		value.AllowedModels = []settingsSubagentModelRoute{}
	}
	return value, nil
}

// settingsSubagentModelKey 是官方 subagentModelKey 的同一不透明身份（controller:76-78）。
func settingsSubagentModelKey(route settingsSubagentModelRoute) string {
	return route.Provider + "\x00" + route.Model
}

// sameSettingsSubagentModelRoutes 与官方 sameRoutes 一样忽略顺序、比较完整集合。
func sameSettingsSubagentModelRoutes(left, right []settingsSubagentModelRoute) bool {
	if len(left) != len(right) {
		return false
	}
	keys := make(map[string]bool, len(right))
	for _, route := range right {
		keys[settingsSubagentModelKey(route)] = true
	}
	for _, route := range left {
		if !keys[settingsSubagentModelKey(route)] {
			return false
		}
	}
	return true
}

// settingsSubagentModelWantsCatalog 决定绘制期是否允许触发目录请求。
// 官方只在 enabled 时 loadCatalog；默认关闭时不能为了画一个隐藏列表就发三路 HTTP。
func settingsSubagentModelWantsCatalog(describe *apiclient.SettingsDescribeResult, drafts map[string]settingsItemDraft) bool {
	if describe == nil {
		return false
	}
	view, ok := describe.Namespace(settingsNSSubagentModelSelection)
	if !ok {
		return false
	}
	value, err := decodeSettingsSubagentModelValue(view.Value)
	if err != nil {
		return false
	}
	enabled := value.Enabled
	if draft, ok := drafts[settingsItemDraftKey(settingsNSSubagentModelSelection, settingsFieldSubagentModelEnabled)]; ok && draft.Subagent != nil {
		enabled = draft.Subagent.Enabled
	}
	return enabled
}

// reconcileSubagentModel 对应官方 scope.subscribe 的收敛分支：外部 revision 已前进，
// 但新 Host 值与草稿完全一致时清草稿，而不是把旧 baseline 留给下一次编辑制造假冲突。
func (f *settingsItemForm) reconcileSubagentModel(describe *apiclient.SettingsDescribeResult) {
	if describe == nil {
		return
	}
	view, ok := describe.Namespace(settingsNSSubagentModelSelection)
	if !ok {
		return
	}
	value, err := decodeSettingsSubagentModelValue(view.Value)
	if err != nil {
		return
	}
	revision := view.Revision
	if section, ok := describe.Section(settingsNSSubagentModelSelection); ok {
		revision = section.Revision
	}
	key := settingsItemDraftKey(settingsNSSubagentModelSelection, settingsFieldSubagentModelEnabled)
	f.mu.Lock()
	defer f.mu.Unlock()
	draft, staged := f.drafts[key]
	if !staged || draft.Subagent == nil || f.baseline[settingsNSSubagentModelSelection] == revision {
		return
	}
	if value.Enabled == draft.Subagent.Enabled && sameSettingsSubagentModelRoutes(value.AllowedModels, draft.Subagent.Routes) {
		delete(f.drafts, key)
		delete(f.baseline, settingsNSSubagentModelSelection)
		f.write.Failed = false
		f.write.Detail = ""
	}
}

// attachSettingsSubagentModel 把对象数组 section 并入同一张卡和同一个 Save footer。
// 官方保存顺序是 models → limits（subagent-card-controller.ts:62-69），所以模型 ops 放前面。
func attachSettingsSubagentModel(card *settingsItemCardState, describe *apiclient.SettingsDescribeResult, fallback settingsItemFormSnapshot) {
	model := buildSettingsSubagentModelState(describe, fallback)
	if model == nil {
		return
	}
	card.Model = model
	if model.Dirty {
		routes := cloneSettingsSubagentModelRoutes(model.DesiredRoutes)
		if routes == nil {
			routes = []settingsSubagentModelRoute{}
		}
		op := settingsItemOp{NS: settingsNSSubagentModelSelection, Ops: []apiclient.SettingsOp{
			{Op: "set", Path: []string{settingsFieldSubagentModelEnabled}, Value: model.Enabled},
			{Op: "set", Path: []string{settingsFieldAllowedModels}, Value: routes},
		}}
		card.Plan.Sections = append([]settingsItemOp{op}, card.Plan.Sections...)
	}
	if model.Invalid || (model.Dirty && model.Conflicted) {
		card.Plan.Invalid = true
	}
	card.Writable = card.Available()
	for _, section := range card.Sections {
		if !section.Writable {
			card.Writable = false
		}
	}
	if model.Available && !model.Writable {
		card.Writable = false
	}
}

func buildSettingsSubagentModelState(describe *apiclient.SettingsDescribeResult, fallback settingsItemFormSnapshot) *settingsSubagentModelState {
	if describe == nil {
		return nil
	}
	view, served := describe.Namespace(settingsNSSubagentModelSelection)
	if !served {
		return nil
	}
	state := &settingsSubagentModelState{Available: true, Revision: view.Revision, CatalogStatus: settingsSubagentCatalogIdle}
	section, sectionOK := describe.Section(settingsNSSubagentModelSelection)
	if sectionOK {
		state.Writable = section.Writable
		state.Revision = section.Revision
	}
	value, err := decodeSettingsSubagentModelValue(view.Value)
	if err != nil {
		// 原始 Value 坏掉时不把 false/空数组冒充真值；section 保留但整节只读并显示未知。
		return state
	}
	state.Known = true
	state.CurrentEnabled = value.Enabled
	state.CurrentRoutes = cloneSettingsSubagentModelRoutes(value.AllowedModels)
	state.Enabled = value.Enabled
	state.DesiredRoutes = cloneSettingsSubagentModelRoutes(value.AllowedModels)

	draftKey := settingsItemDraftKey(settingsNSSubagentModelSelection, settingsFieldSubagentModelEnabled)
	draft, staged := fallback.drafts[draftKey]
	if staged && draft.Subagent != nil {
		state.Enabled = draft.Subagent.Enabled
		state.DesiredRoutes = cloneSettingsSubagentModelRoutes(draft.Subagent.Routes)
	}
	state.Dirty = state.CurrentEnabled != state.Enabled || !sameSettingsSubagentModelRoutes(state.CurrentRoutes, state.DesiredRoutes)
	state.Invalid = state.Enabled && len(state.DesiredRoutes) == 0
	if staged && state.Dirty {
		if baseline, ok := fallback.baseline[settingsNSSubagentModelSelection]; ok && baseline != state.Revision {
			state.Conflicted = true
		}
	}

	if state.Enabled {
		state.CatalogRetry = fallback.catalogRetryWired
		switch {
		case !fallback.catalogWired:
			state.CatalogStatus = settingsSubagentCatalogError
		case fallback.catalog == nil:
			state.CatalogStatus = settingsSubagentCatalogIdle
		case fallback.catalog.Loading:
			state.CatalogStatus = settingsSubagentCatalogLoading
		case fallback.catalog.CatalogError != "":
			state.CatalogStatus = settingsSubagentCatalogError
		case fallback.catalog.Catalog != nil:
			state.CatalogStatus = settingsSubagentCatalogReady
		}
		if state.CatalogStatus == settingsSubagentCatalogReady && fallback.catalog != nil && fallback.catalog.Catalog != nil {
			state.CatalogPartial = len(fallback.catalog.Catalog.Failures) > 0
		}
	}
	var groups []apiclient.ModelProvider
	if fallback.catalog != nil && fallback.catalog.Catalog != nil {
		groups = fallback.catalog.Catalog.Groups
	}
	state.Candidates = settingsSubagentModelCandidates(groups, state.CurrentRoutes, state.DesiredRoutes)
	return state
}

// settingsSubagentModelCandidates 按官方 controller:87-118 join：目录顺序优先，
// 已保存/草稿中已消失的路由追加到末尾，仍可取消并保存移除。
func settingsSubagentModelCandidates(groups []apiclient.ModelProvider, stored, selected []settingsSubagentModelRoute) []settingsSubagentModelCandidate {
	selectedKeys := make(map[string]bool, len(selected))
	for _, route := range selected {
		selectedKeys[settingsSubagentModelKey(route)] = true
	}
	retained := make(map[string]settingsSubagentModelRoute, len(stored)+len(selected))
	retainedOrder := make([]string, 0, len(stored)+len(selected))
	for _, routes := range [][]settingsSubagentModelRoute{stored, selected} {
		for _, route := range routes {
			key := settingsSubagentModelKey(route)
			if _, ok := retained[key]; ok {
				continue
			}
			retained[key] = route
			retainedOrder = append(retainedOrder, key)
		}
	}
	var out []settingsSubagentModelCandidate
	for _, group := range groups {
		for _, model := range group.Models {
			route := settingsSubagentModelRoute{Provider: group.ID, Model: model.ID}
			key := settingsSubagentModelKey(route)
			delete(retained, key)
			out = append(out, settingsSubagentModelCandidate{
				settingsSubagentModelRoute: route,
				Key:                        key, ProviderName: group.Name, ModelName: model.Name,
				Available: true, Selected: selectedKeys[key],
			})
		}
	}
	for _, key := range retainedOrder {
		route, ok := retained[key]
		if !ok {
			continue
		}
		out = append(out, settingsSubagentModelCandidate{
			settingsSubagentModelRoute: route,
			Key:                        key, ProviderName: route.Provider, ModelName: route.Model,
			Available: false, Selected: selectedKeys[key],
		})
	}
	return out
}

// stageSubagentModel 记录整节草稿并锁下首次编辑的 revision。
func (f *settingsItemForm) stageSubagentModel(state *settingsSubagentModelState, enabled bool, routes []settingsSubagentModelRoute) {
	if state == nil || !state.Known {
		return
	}
	f.stage(settingsNSSubagentModelSelection, settingsFieldSubagentModelEnabled, settingsItemDraft{
		Subagent: &settingsSubagentModelDraft{Enabled: enabled, Routes: cloneSettingsSubagentModelRoutes(routes)},
	}, state.Revision)
}

// toggleSubagentModel 切换一条精确路由；未知 key 不 invented 一条授权。
func toggleSubagentModel(routes []settingsSubagentModelRoute, candidates []settingsSubagentModelCandidate, key string) ([]settingsSubagentModelRoute, bool) {
	var candidate *settingsSubagentModelCandidate
	for index := range candidates {
		if candidates[index].Key == key {
			candidate = &candidates[index]
			break
		}
	}
	if candidate == nil {
		return routes, false
	}
	out := cloneSettingsSubagentModelRoutes(routes)
	for index, route := range out {
		if settingsSubagentModelKey(route) == key {
			out = append(out[:index], out[index+1:]...)
			return out, true
		}
	}
	out = append(out, settingsSubagentModelRoute{Provider: candidate.Provider, Model: candidate.Model})
	return out, true
}
