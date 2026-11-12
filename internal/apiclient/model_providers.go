// model_providers.go —— 官方 rc.2 `llm.listConfigurableProviders` 的客户端。
//
// 为什么界面要这个方法：composer 的模型菜单（`conversation.input.model` 的父容器）
// 要列"这台宿主配了哪些 provider"。官方由 `ui-model-selection` 走 session 的
// model directory；本仓宿主当前能给出的**真值**是 provider 目录
// （`DSH-GO/internal/transport/handlers_llm_provider_official.go`），model 目录
// 还没接（`session.selectModel` 的 catalog 面未实现到界面侧）。
//
// 契约（官方 handler 逐字段）：
//
//	参数：**没有参数**（官方 `parameters: []`；传任何字段 → 400 bad_request）
//	结果：数组，每项
//	      {provider, displayName, settingsNs, settingsPath,
//	       declared?, defaultBaseURL?, baseURLEnv?, nativeAuth?, error?}
//	错误：未接线 → 501 not_implemented（**不是空数组**）
//
// # 为什么"空数组"与"未接线"必须分开
//
// 空数组 = 宿主真的没有可配置 provider；501 = 宿主没有这个能力。
// 界面对这两者的显示都是"整段隐藏"，但**日志与报告里必须能区分**：
// 把 501 当空数组吞掉，就永远查不出"菜单为什么不显示"。
package apiclient

import "context"

// ProviderNativeAuth 是官方 `ProviderNativeAuth`（`{env, oauth}`）。
//
// 只描述"凭据从哪来"，**不含任何值**：secret 只在宿主侧。
type ProviderNativeAuth struct {
	Env   []string `json:"env"`
	OAuth bool     `json:"oauth"`
}

// ConfigurableProvider 是 `llm.listConfigurableProviders` 的一行。
//
// 字段名逐字对齐官方 wire（`provider` / `displayName` / `settingsNs` /
// `settingsPath` / `declared` / `defaultBaseURL` / `baseURLEnv` / `nativeAuth` /
// `error`）。`Declared` 是"当前 catalog 里有模型声明它"，官方用 omitempty，
// 所以缺字段与 false 同义。
type ConfigurableProvider struct {
	Provider          string              `json:"provider"`
	DisplayName       string              `json:"displayName"`
	SettingsNamespace string              `json:"settingsNs"`
	SettingsPath      []string            `json:"settingsPath"`
	Declared          bool                `json:"declared,omitempty"`
	DefaultBaseURL    string              `json:"defaultBaseURL,omitempty"`
	BaseURLEnv        []string            `json:"baseURLEnv,omitempty"`
	NativeAuth        *ProviderNativeAuth `json:"nativeAuth,omitempty"`
	// Error 是这个 provider 自己的失败说明（宿主算不出来时给）。
	// 非空 = 这一项不可用；界面必须照实显示，不能拿它假装"有 provider"。
	Error string `json:"error,omitempty"`
}

// ListConfigurableProviders 是 `llm.listConfigurableProviders`。
//
// 返回 nil 切片 + nil error 表示宿主回了空数组（**真的没有**可配置 provider）；
// 宿主没有这个方法时返回 *APIError（code=not_implemented / HTTP 501）。
func (c *Client) ListConfigurableProviders(ctx context.Context) ([]ConfigurableProvider, error) {
	var out []ConfigurableProvider
	// 官方 parameters: []：传 nil 时 Call 发 `{}`，正好是"无参数"。
	if err := c.Call(ctx, "llm.listConfigurableProviders", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
