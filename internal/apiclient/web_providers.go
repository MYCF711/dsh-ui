// web_providers.go —— 官方 `web.listProviders` 的客户端（包 `dsh-web`）。
//
// 契约唯一来源是**官方 rc.2 源码**（GUI 侧没有别的权威）：
//
//	OFF:dsh-web/src/index.ts:146-157   listProviders(): WebProviderList
//	OFF:dsh-web/src/types.ts:196-217   WebProviderInfo / WebProviderList
//	OFF:dsh-web/src/index.ts:246-252   providerInfo()：没声明 title/description 时**缺席**
//	OFF:dsh-web/src/index.ts:274-297   选择规则与 WEB_PROVIDER_* 错误码
//
// 宿主侧实现与附加字段（不冒充官方）：`../DSH-GO/internal/transport/
// handlers_web_providers_rc2.go` 的 `OfficialWebProviderList`。附加字段一律
// 用指针 + `omitempty`：**缺席**表示"宿主没给"，不是"值是 false"。
//
// # 为什么它有自己的一路而不是并进 settings.describe
//
// provider 清单不是设置文档的一部分：官方它由 `web` 服务在运行期发布
// （插件注册/注销时发 `web/providers-changed`），而 `settings.describe` 只投影
// 设置命名空间。设置页里"菜单里有哪几项"必须来自这份清单，**不能**按 id 硬编码
// ——官方菜单里除了四个官方 id 之外还会有插件注册的 provider。
package apiclient

import "context"

// WebProviderInfo 是一条 provider（官方 `WebProviderInfo` + 宿主附加运行态）。
type WebProviderInfo struct {
	// ID 是注册键，也是 `searchProvider` / `fetchProvider` 选的值。
	ID string `json:"id"`
	// Title 是显示名；provider 没声明时**缺席**（不是空串）。
	Title string `json:"title,omitempty"`
	// Description 是 picker 说明：`string` 或 `{en: string, [locale]: string}`。
	// 官方 LocalizedText，所以客户端用 any 收、再按语言解析（见 ResolveLocalizedText）。
	Description any `json:"description,omitempty"`

	// Available 是 provider 自己 `Available()` 的回答（本地检查，不联网）。
	// nil = 宿主没给出（官方清单本身不含这个字段）。
	Available *bool `json:"available,omitempty"`
	// Default 表示它就是当前配置钉死（或基线默认生效）的 provider。
	Default bool `json:"default,omitempty"`
	// Selected 表示执行期解析当前会选中的那个 provider。
	Selected bool `json:"selected,omitempty"`
}

// WebProviderSelection 是一个能力的"当前选择"结果（宿主附加）。
type WebProviderSelection struct {
	// Configured 是配置钉死的 id；空 = 未钉死（按官方规则自动选择）。
	Configured string `json:"configured,omitempty"`
	// Source 说明 Configured 从哪来：config / env / default / auto。
	Source string `json:"source,omitempty"`
	// Selected 是执行期真正会选中的 provider id；选择失败时为空。
	Selected string `json:"selected,omitempty"`
	// ErrorCode 是选择失败时的官方 web 码（WEB_PROVIDER_*）。
	ErrorCode string `json:"errorCode,omitempty"`
	// Error 是选择失败的可读说明。
	Error string `json:"error,omitempty"`
}

// WebProviderResolution 是两个能力各自的当前选择结果（宿主附加）。
type WebProviderResolution struct {
	Search WebProviderSelection `json:"search"`
	Fetch  WebProviderSelection `json:"fetch"`
}

// WebProviderBaseline 报告 rc.2 基础组合的默认 provider 与本宿主是否已注册它。
type WebProviderBaseline struct {
	Search           string `json:"search"`
	SearchRegistered bool   `json:"searchRegistered"`
	Fetch            string `json:"fetch"`
	FetchRegistered  bool   `json:"fetchRegistered"`
}

// WebSearchLimits 是 `web_search` 的生效上限（宿主附加）。
type WebSearchLimits struct {
	MaxQueries      int `json:"maxQueries"`
	MaxResults      int `json:"maxResults"`
	SummaryMaxChars int `json:"summaryMaxChars"`
	SearchTimeoutMS int `json:"searchTimeoutMs"`
}

// WebCredentialState 是一个 provider 相关环境变量的真实配置状态（宿主附加）。
//
// 宿主把它直接列出来，是因为官方把凭据解析藏在 provider 内部、失败时只给
// `WEB_PROVIDER_*` 错误码；有了这一节界面才能说清"缺哪一项"。
type WebCredentialState struct {
	// Name 是环境变量名。
	Name string `json:"name"`
	// Configured 是"当前非空"（真读环境）。
	Configured bool `json:"configured"`
	// Role 是它管什么：selection / endpoint / credential / model / billing / limit。
	Role string `json:"role,omitempty"`
	// Capability 是 search / fetch / both。
	Capability string `json:"capability,omitempty"`
}

// WebProviderList 是 `web.listProviders` 的结果。
//
// `Search` / `Fetch` 是官方字段；其余为宿主附加（见文件头）。
type WebProviderList struct {
	Search []WebProviderInfo `json:"search"`
	Fetch  []WebProviderInfo `json:"fetch"`

	Resolution  *WebProviderResolution `json:"resolution,omitempty"`
	Baseline    *WebProviderBaseline   `json:"baseline,omitempty"`
	Limits      *WebSearchLimits       `json:"limits,omitempty"`
	Credentials []WebCredentialState   `json:"credentials,omitempty"`
}

// WebListProviders 读宿主的 provider 清单（官方 `web/listProviders`，**无参数**）。
func (c *Client) WebListProviders(ctx context.Context) (WebProviderList, error) {
	var out WebProviderList
	// 官方 `parameters: []`：宿主严格无参校验，所以发空对象。
	if err := c.Call(ctx, "web.listProviders", struct{}{}, &out); err != nil {
		return WebProviderList{}, err
	}
	return out, nil
}
