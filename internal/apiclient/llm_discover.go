// llm_discover.go —— 官方 `llm.discoverModels` 的客户端（发现端点上的模型）。
//
// 官方形状（rc.2 随包 src）：
//
//	dsh-client-ui-settings-models/src/client/operations.ts:92-96
//	  `ctx.remote.llm.discoverModels(settingsNs, request)`
//	dsh-client-ui-settings-models/src/client/ModelListEditor.tsx:226-268
//	  按钮 → `discoverModels` → 候选列表 → 勾选后 `adoptPicked` 并回写模型列表
//
// 参数：`settingsNs`（必填，空串宿主拒绝）+ 可选 `request{provider?,baseURL?,api?,apiKey?}`。
// 结果：`ModelDiscovery[]`，每项 `{id, name?, contextWindow?, maxTokens?,
// inputModalities?}`，**缺字段 = 未知**（不是 0，也不是空数组）。
//
// 宿主实现：`../DSH-GO/internal/transport/handlers_account_official.go:1326-1391`
// （`officialLLMDiscoverModels`）→ `cmd/dshd/serve_account_api.go:492-538`
// （真实 `GET <baseURL>/models`）→ `internal/llm/discover.go:113`。
//
// 它**不是** `models.catalog.test`：后者是一次真实推理，返回 `{id,ok,model,latencyMs}`，
// 不列模型（`model_catalog.go:884-886`；见 model_catalog_editor.go 的同名注释）。
package apiclient

import (
	"context"
)

// ModelDiscoveryRequest 是 `llm.discoverModels` 的可选 request 四字段。
//
// 四个都是可选：只给 provider 时由宿主用目录里该路由声明的 baseURL 探；
// 只给 baseURL 时直接探这个端点。两者都没有时宿主会 fail-loud（
// `provider_registry.go:156-164`），客户端不替它猜一个端点。
type ModelDiscoveryRequest struct {
	Provider string
	BaseURL  string
	API      string
	APIKey   string
}

// DiscoveredModel 是一条发现结果（官方 `ModelDiscovery`）。
//
// 指针 = absent 与零值的区别：官方 zod 里 name/contextWindow/maxTokens/
// inputModalities 都是 `.optional()`，"端点没报"与"报了 0/空"是两件事。
type DiscoveredModel struct {
	ID              string   `json:"id"`
	Name            *string  `json:"name,omitempty"`
	ContextWindow   *int64   `json:"contextWindow,omitempty"`
	MaxTokens       *int64   `json:"maxTokens,omitempty"`
	InputModalities []string `json:"inputModalities,omitempty"`
}

// LLMDiscoverModels 调 `llm.discoverModels`：问端点上有哪些模型。
//
// settingsNs 必须是宿主真实存在且可路由的命名空间（本仓就是 `llm`）；
// 空串由宿主当 `bad_request` 拒（`handlers_account_official.go:1371-1373`），
// 客户端不替它编一个，原样发出去让宿主的四字段回来。
func (c *Client) LLMDiscoverModels(ctx context.Context, settingsNS string, req ModelDiscoveryRequest) ([]DiscoveredModel, error) {
	params := map[string]any{"settingsNs": settingsNS}
	// request 整体可选：官方 `z.object({...}).optional()`。只有真的带了字段才发，
	// 免得把"没有东西可问"投成一次空请求。
	request := map[string]any{}
	if req.Provider != "" {
		request["provider"] = req.Provider
	}
	if req.BaseURL != "" {
		request["baseURL"] = req.BaseURL
	}
	if req.API != "" {
		request["api"] = req.API
	}
	if req.APIKey != "" {
		request["apiKey"] = req.APIKey
	}
	if len(request) > 0 {
		params["request"] = request
	}
	var out []DiscoveredModel
	if err := c.Call(ctx, "llm.discoverModels", params, &out); err != nil {
		return nil, err
	}
	return out, nil
}
