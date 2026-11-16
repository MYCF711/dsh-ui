// model_catalog.go —— 官方 `session.modelCatalog` 的客户端与 Models 页壳内投影。
//
// 目录是严格零参数方法；501 not_implemented 表示宿主没有真实目录能力，
// 与宿主成功返回空 groups/failures 是两种事实，调用方必须分别保留。
package apiclient

import "context"

// ModelCatalog 是 `session.modelCatalog` 的完整结果。
type ModelCatalog struct {
	Default           ModelSelection  `json:"default"`
	RoutableProviders []string        `json:"routableProviders"`
	Groups            []ModelProvider `json:"groups"`
	Failures          []ModelFailure  `json:"failures"`
}

// ModelSelection 是目录给出的默认模型选择。
type ModelSelection struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoningEffort,omitempty"`
}

// ModelProvider 是一个 provider 及其模型分组。
type ModelProvider struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Models []CatalogModel `json:"models"`
}

// CatalogModel 是模型目录中的一行。
type CatalogModel struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Reasoning   *ModelReasoning `json:"reasoning,omitempty"`
}

// ModelReasoning 声明模型支持的推理档。
type ModelReasoning struct {
	Efforts       []ModelReasoningEffort `json:"efforts"`
	DefaultEffort string                 `json:"defaultEffort,omitempty"`
}

// ModelReasoningEffort 是一个可选推理档。
type ModelReasoningEffort struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ModelFailure 是单个 provider 的目录加载失败；它不阻断其它分组。
type ModelFailure struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

// SessionModelCatalog 是 `session.modelCatalog`。
// 官方 parameters: []；未接线返回 501，不能折叠成空目录。
func (c *Client) SessionModelCatalog(ctx context.Context) (ModelCatalog, error) {
	var out ModelCatalog
	if err := c.Call(ctx, "session.modelCatalog", nil, &out); err != nil {
		return ModelCatalog{}, err
	}
	return out, nil
}

// ModelsPageSnapshot 是壳同步发布给 Models 页的三路数据快照。
// 字段只用于壳内投影，不参与 Transport JSON。
//
// Registered/Configurable/Catalog 仅在对应请求成功时赋值；空切片只表示宿主
// 真实返回空目录。任一路失败只写自己的 Error 原文，不影响另外两路的数据。
type ModelsPageSnapshot struct {
	Registered        []ProviderInfo
	Configurable      []ConfigurableProvider
	Catalog           *ModelCatalog
	RegisteredError   string
	ConfigurableError string
	CatalogError      string
	Loading           bool
}
