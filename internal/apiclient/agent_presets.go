// agent_presets.go —— 官方 `agentPresets.*` 的读客户端（#3 的数据面）。
//
// # 为什么单独一路、不放进 settings.go
//
// 官方 roster **不是 Host 设置**：`settings.describe` 里没有它。它是
// `agent-preset-registry` 服务发布的清单（
// `OFF:dsh-client-ui-agent-preset/src/client/settings-store.ts:36-46` 的
// `readRoster` 走 `ctx.remote.agentPresets.list()`）。本仓宿主同形：
// `HOST:internal/transport/handlers_agent_official.go:1652-1684`。
//
// # 契约（逐条对宿主，不是凭印象）
//
//	agentPresets.list  零参数；结果 {presets:[{id, isDefault, name?, description?, broken?}]}
//	agentPresets.read  {agentPreset}；结果 {agentPreset, content, name?, description?}
//	                   （参数名逐字取自 `HOST:.../handlers_agent_official.go:1676-1690`
//	                   的 `p.str("agentPreset")`；**不是** presetId）
//
// 三条都是 POST /api/<method> + 平铺参数 + {result}/四字段错误。
//
// **`isDefault` 是宿主算出来的**（`selectedDefault ?? default`，
// `HOST:cmd/dshd/agent_preset_default.go:29-46`）：客户端不许自己拿
// `agent-preset-registry.selectedDefault` 的值去猜哪一行是默认——那是另一条
// 读取路径，两边一旦不一致就会出现"角标说 A、实际建会话用 B"。
package apiclient

import "context"

// AgentPreset 是官方 roster 里的一条 preset（逐字对齐宿主 `OfficialAgentPreset`）。
//
// `Broken` 非空 = 声明存在但装不起来（官方用它画 broken 角标并禁掉"设为默认"）。
// 本仓宿主当前不产生它（mode 解析失败是 fail-loud 的），但字段先按官方留着：
// 将来宿主真给出来时，界面必须已经知道它意味着"这一行不能选"。
type AgentPreset struct {
	ID          string `json:"id"`
	IsDefault   bool   `json:"isDefault"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Broken      string `json:"broken,omitempty"`
}

// AgentPresetRoster 是 `agentPresets.list` 的结果。
type AgentPresetRoster struct {
	Presets []AgentPreset `json:"presets"`
}

// AgentPresetDocument 是 `agentPresets.read` 的结果（声明正文 + 已发布元数据）。
type AgentPresetDocument struct {
	AgentPreset string `json:"agentPreset"`
	Content     string `json:"content"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// SettingsNSAgentPreset 是官方 agent-preset registry 的设置命名空间（逐字）。
//
// 「设为默认」写的是**这个命名空间里的一个字段**，不是 preset 自身的属性
// （`OFF:dsh-agent-preset-registry/src/index.ts:52-59` 的 `Config`）。
const SettingsNSAgentPreset = "agent-preset-registry"

// SettingsFieldSelectedDefault 是"后续会话的默认 preset"字段（逐字，官方
// `Config.selectedDefault`）。宿主在写路径上校验 id 真实存在，未知 id 回 400。
const SettingsFieldSelectedDefault = "selectedDefault"

// AgentPresetsList 读 preset 名册（宿主 `agentPresets.list`）。
//
// 宿主没接这条能力时回四字段 `not_implemented`——调用方必须把这一路当成
// 「没有名册」而不是「名册是空的」：前者整页显示未知，后者才是空状态。
func (c *Client) AgentPresetsList(ctx context.Context) (*AgentPresetRoster, error) {
	var out AgentPresetRoster
	if err := c.Call(ctx, "agentPresets.list", struct{}{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AgentPresetRead 读一个 preset 的声明正文（宿主 `agentPresets.read`）。
func (c *Client) AgentPresetRead(ctx context.Context, presetID string) (*AgentPresetDocument, error) {
	var out AgentPresetDocument
	if err := c.Call(ctx, "agentPresets.read", map[string]any{"agentPreset": presetID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
