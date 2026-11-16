// official_models.go —— rc.2 Models 独立页的只读目录投影。
//
// 官方事实源不是静态 provider 表：ModelsSettingsStore 同时读取
// llm/listProviders、llm/listConfigurableProviders 与 session/modelCatalog
// （official store.ts:230-273）。这里做同一目录 join；模型目录保留宿主分组顺序。
package dssettings

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

const modelsIntro = "填入各提供商的 API 密钥即可使用其模型。"

// ModelsProvider 是目录 join 后的一行。Active 与 Configured 是两个事实，不能合并。
type ModelsProvider struct {
	Provider        string
	DisplayName     string
	SettingsNS      string
	SettingsPath    []string
	Active          bool
	ActiveKnown     bool
	Configured      bool
	ConfiguredKnown bool
	Error           string
}

// ModelsPage 是 Models 页的非设置文档数据面。
type ModelsPage struct {
	Providers []ModelsProvider
	Errors    []string
	Loading   bool
}

// JoinProviderDirectory 对齐官方 joinProviderDirectory：先 configurable 声明顺序，
// 再补只注册的 route，最后稳定地把 account/official 提前（store.ts:45-76）。
func JoinProviderDirectory(registered []apiclient.ProviderInfo, configurable []apiclient.ConfigurableProvider, describe *apiclient.SettingsDescribeResult) []ModelsProvider {
	active := make(map[string]bool, len(registered))
	for _, provider := range registered {
		active[provider.ID] = true
	}
	declared := make(map[string]bool, len(configurable))
	rows := make([]ModelsProvider, 0, len(registered)+len(configurable))
	for _, entry := range configurable {
		declared[entry.Provider] = true
		rows = append(rows, ModelsProvider{
			Provider: entry.Provider, DisplayName: entry.DisplayName,
			SettingsNS: entry.SettingsNamespace, SettingsPath: append([]string(nil), entry.SettingsPath...),
			Active: active[entry.Provider], Configured: providerConfigured(describe, entry.SettingsNamespace, entry.SettingsPath),
			Error: entry.Error,
		})
	}
	for _, provider := range registered {
		if declared[provider.ID] {
			continue
		}
		rows = append(rows, ModelsProvider{Provider: provider.ID, DisplayName: provider.Name, Active: true})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return providerRank(rows[i].Provider) < providerRank(rows[j].Provider)
	})
	return rows
}

func providerRank(provider string) int {
	switch provider {
	case "deepseek-account":
		return 0
	case "deepseek-official":
		return 1
	default:
		return 2
	}
}

func providerConfigured(describe *apiclient.SettingsDescribeResult, ns string, path []string) bool {
	if describe == nil || ns == "" {
		return false
	}
	namespace, ok := describe.Namespace(ns)
	if !ok {
		return false
	}
	if len(path) == 0 {
		return true
	}
	_, found := valueAtPath(namespace.Value, path)
	return found
}

// valueAtPath 只读宿主原始 JSON；json.Number 保留数值文本，不把缺失误判成 0。
func valueAtPath(raw json.RawMessage, path []string) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var current any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&current); err != nil {
		return nil, false
	}
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func rawObject(raw json.RawMessage) map[string]any {
	value, ok := valueAtPath(raw, nil)
	if !ok {
		return nil
	}
	object, ok := value.(map[string]any)
	return object
}

func cloneObject(object map[string]any) map[string]any {
	if object == nil {
		return nil
	}
	out := make(map[string]any, len(object))
	for key, value := range object {
		if nested, ok := value.(map[string]any); ok {
			out[key] = cloneObject(nested)
		} else if list, ok := value.([]any); ok {
			out[key] = append([]any(nil), list...)
		} else {
			out[key] = value
		}
	}
	return out
}

// fallbackProviders 只在老壳没有目录快照时使用 llm 的真实 settings 路由；
// 它不是一份硬编码 provider 名录。
func fallbackProviders(describe *apiclient.SettingsDescribeResult) []ModelsProvider {
	routes, _, ok := llmRetryPolicies(describe)
	if !ok {
		return nil
	}
	out := make([]ModelsProvider, 0, len(routes))
	for _, route := range routes {
		out = append(out, ModelsProvider{
			Provider: route, DisplayName: route, SettingsNS: apiclient.SettingsNSLLM,
			SettingsPath: []string{"providers", route}, Configured: true, ConfiguredKnown: true,
			// settings 只能证明配置存在，不能证明 route 已注册。
			ActiveKnown: false,
		})
	}
	return out
}

// buildModelsPage 重建 Models 页的拍平行与视觉分组。edit 非空时只展开该 provider
// 的 retryPolicy 草稿；其它 provider 保持一张关闭的卡（ModelsSection.tsx:458-563）。
func buildModelsPage(panel PanelSection, describe *apiclient.SettingsDescribeResult, m *PanelModel,
	snapshot *apiclient.ModelsPageSnapshot, edit *retryEditorState) PanelSection {
	panel.Rows = nil
	panel.Groups = nil
	page := &ModelsPage{}
	panel.Models = page
	if describe == nil {
		panel.State = PanelUnknown
		panel.Note = "未知：宿主未接 settings.describe"
		return panel
	}
	panel.Writable = describe.Writable

	var defaultRows []PanelRow
	if row, ok := defaultModelRow(describe, snapshot, m); ok {
		defaultRows = append(defaultRows, row)
		panel.Rows = append(panel.Rows, row)
	}
	if len(defaultRows) > 0 {
		panel.Groups = append(panel.Groups, PanelGroup{Title: "默认模型", Rows: defaultRows})
	}

	if snapshot == nil {
		page.Providers = fallbackProviders(describe)
		if page.Providers == nil {
			panel.Rows = append(panel.Rows, PanelRow{ID: "retry-policy-unknown", Title: "重试策略", Kind: ItemSelector,
				Disabled: true, Reason: "宿主未提供 llm 命名空间（providers.<route>.retryPolicy）"})
			if m != nil {
				m.Unknown = append(m.Unknown, "llm.providers.*.retryPolicy")
			}
		}
	} else {
		page.Loading = snapshot.Loading
		page.Providers = JoinProviderDirectory(snapshot.Registered, snapshot.Configurable, describe)
		// 空错误表示本次成功；非 nil 数据表示刷新失败后仍保有上一次成功值。
		// 首次失败且没有旧值时是未知，绝不能投影成确定的未注册/未配置。
		registeredKnown := snapshot.RegisteredError == "" || snapshot.Registered != nil
		configurableKnown := snapshot.ConfigurableError == "" || snapshot.Configurable != nil
		for index := range page.Providers {
			page.Providers[index].ActiveKnown = registeredKnown
			page.Providers[index].ConfiguredKnown = configurableKnown
		}
		for _, err := range []string{snapshot.RegisteredError, snapshot.ConfigurableError, snapshot.CatalogError} {
			if err != "" {
				page.Errors = append(page.Errors, err)
			}
		}
		if snapshot.Loading {
			panel.Note = "正在加载提供商目录…"
		} else if len(page.Errors) > 0 {
			panel.Note = "部分模型数据加载失败；已保留宿主返回的其余真值"
		} else {
			panel.Note = ""
		}
	}

	for _, provider := range page.Providers {
		rows := []PanelRow{providerEditRow(provider, edit != nil && edit.route == provider.Provider)}
		if edit != nil && edit.route == provider.Provider {
			rows = append(rows, edit.rows()...)
			rows = append(rows, retryFooterRows(provider.Provider, edit.writable)...)
		}
		panel.Rows = append(panel.Rows, rows...)
		panel.Groups = append(panel.Groups, PanelGroup{Title: provider.DisplayName, Rows: rows})
	}

	if snapshot != nil && snapshot.Catalog != nil {
		for _, group := range snapshot.Catalog.Groups {
			rows := make([]PanelRow, 0, len(group.Models))
			for index, model := range group.Models {
				name := model.Name
				if name == "" {
					name = model.ID
				}
				rows = append(rows, PanelRow{ID: "catalog-model-" + group.ID + "-" + strconv.Itoa(index),
					Title: name, Desc: model.ID, Kind: ItemSelector, Known: true, Disabled: true})
			}
			panel.Rows = append(panel.Rows, rows...)
			panel.Groups = append(panel.Groups, PanelGroup{Title: "模型目录 · " + group.Name, Rows: rows})
		}
		for index, failure := range snapshot.Catalog.Failures {
			row := PanelRow{ID: "catalog-failure-" + strconv.Itoa(index), Title: failure.Name,
				Desc: failure.ID, Reason: failure.Message, Kind: ItemSelector, Disabled: true}
			panel.Rows = append(panel.Rows, row)
			panel.Groups = append(panel.Groups, PanelGroup{Title: "模型目录加载失败", Rows: []PanelRow{row}})
		}
	}
	return panel
}

func providerEditRow(provider ModelsProvider, open bool) PanelRow {
	status := []string{provider.Provider}
	if !provider.ActiveKnown {
		status = append(status, "注册未知")
	} else if provider.Active {
		status = append(status, "已注册")
	} else {
		status = append(status, "未注册")
	}
	if !provider.ConfiguredKnown {
		status = append(status, "配置未知")
	} else if provider.Configured {
		status = append(status, "已配置")
	} else {
		status = append(status, "未配置")
	}
	title := "编辑"
	if open {
		title = "收起"
	}
	row := PanelRow{ID: "provider-edit-" + provider.Provider, Title: title,
		Desc: strings.Join(status, " · "), Reason: provider.Error,
		Kind: ItemButton, NS: provider.SettingsNS}
	// 目前真实写契约是 llm.providers.<route>.retryPolicy；whole-section 或
	// 未配置 route 不画一个点了才报错的假编辑按钮。
	if provider.ConfiguredKnown && provider.Configured && provider.SettingsNS == apiclient.SettingsNSLLM {
		row.Path = append([]string{"providers"}, provider.Provider)
	} else {
		row.Title = "不可编辑"
		row.Disabled = true
		if row.Reason == "" {
			switch {
			case !provider.ConfiguredKnown:
				row.Reason = "配置目录读取失败，配置状态未知"
			case !provider.Configured:
				row.Reason = "该 route 没有可写的 llm.providers.<route> 配置"
			default:
				row.Reason = "该 route 属于 " + provider.SettingsNS + "；当前仅接 llm.providers.<route>.retryPolicy"
			}
		}
	}
	return row
}

func defaultModelRow(describe *apiclient.SettingsDescribeResult, snapshot *apiclient.ModelsPageSnapshot, m *PanelModel) (PanelRow, bool) {
	section, ok := describe.Section(apiclient.SettingsNSModel)
	if !ok {
		if m != nil {
			m.Unknown = append(m.Unknown, "model.*")
		}
		return PanelRow{}, false
	}
	row := PanelRow{ID: "model-default", Title: "默认模型", Kind: ItemSelector,
		NS: section.NS, Field: "defaultModel", Path: []string{"defaultModel"}, Revision: section.Revision,
		Disabled: !section.Writable,
		Choices:  []PanelChoice{{Value: "", Label: "跟随启动参数", Desc: "清除覆盖，回到宿主的 --model"}}}
	if field, found := section.Field("defaultModel"); found {
		if value, known := apiclient.SettingValueString(field.Current); known {
			row.Value, row.Known, row.Selected = value, true, value
		}
		for _, value := range field.Enum {
			row.Choices = appendPanelChoice(row.Choices, PanelChoice{Value: value, Label: value})
		}
	} else {
		row.Disabled = true
		row.Reason = "宿主未提供 model.defaultModel 字段"
	}
	if snapshot != nil && snapshot.Catalog != nil {
		for _, group := range snapshot.Catalog.Groups {
			for _, model := range group.Models {
				label := model.Name
				if label == "" {
					label = model.ID
				}
				row.Choices = appendPanelChoice(row.Choices, PanelChoice{Value: model.ID,
					Label: label, Desc: group.Name + " · " + model.ID})
			}
		}
		if row.Value == "" && snapshot.Catalog.Default.Model != "" {
			row.Value = snapshot.Catalog.Default.Model
			row.Selected = snapshot.Catalog.Default.Model
			row.Known = true
			row.Reason = "宿主当前生效的默认模型"
		}
	}
	if row.Selected != "" {
		row.Choices = appendPanelChoice(row.Choices, PanelChoice{Value: row.Selected, Label: row.Selected, Desc: "当前设置"})
	}
	if !row.Known {
		row.Disabled = true
		if row.Reason == "" {
			row.Reason = "未知：宿主未给 defaultModel 当前值"
		}
		if m != nil {
			m.Unknown = append(m.Unknown, "model.defaultModel")
		}
	}
	return row, true
}

func appendPanelChoice(choices []PanelChoice, choice PanelChoice) []PanelChoice {
	for _, existing := range choices {
		if existing.Value == choice.Value {
			return choices
		}
	}
	return append(choices, choice)
}

func retryFooterRows(route string, writable bool) []PanelRow {
	reason := ""
	if !writable {
		reason = "只读：llm 命名空间未开放写入"
	}
	return []PanelRow{
		{ID: "retry-" + route + "-cancel", Title: "取消", Kind: ItemButton,
			NS: apiclient.SettingsNSLLM, Path: []string{"providers", route, "retryPolicy"}},
		{ID: "retry-" + route + "-apply", Title: "保存", Kind: ItemButton,
			NS: apiclient.SettingsNSLLM, Path: []string{"providers", route, "retryPolicy"}, Disabled: !writable, Reason: reason},
	}
}

func modelRouteFromRow(row PanelRow) (string, error) {
	if len(row.Path) < 2 || row.Path[0] != "providers" || row.Path[1] == "" {
		return "", fmt.Errorf("Models 行 %q 没有 provider 路径", row.ID)
	}
	return row.Path[1], nil
}
