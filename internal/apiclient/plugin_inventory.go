// plugin_inventory.go — 官方 `pluginInventory.list` 的客户端。
//
// # 契约的唯一来源
//
// 官方是一份 typert Remote（`@deepseek-ai/dsh-host-plugin-inventory` 0.2.2-rc.2，
// `src/index.ts:50-74` 的 `PluginInventoryGateway` + `:82-121` 的
// `readPluginInventory`；类型在 `src/types.ts:16-77`）：
//
//	pluginInventory.list() -> {
//	  managementAvailable?: boolean,
//	  entries: [{
//	    entryId,                                        // Loader 条目 id
//	    moduleName,                                     // 条目 import 的模块 specifier
//	    meta?: { title?, description?, icon?, error? },  // 本地包显示元数据
//	    enabled,                                        // 生效的启用状态（含祖先 group）
//	    fiberPhase: null|pending|loading|active|failed|unloading,
//	    diagnostic?                                     // 启动失败信息
//	  }],
//	  agentPresets?: [{ id, name?, isDefault, broken?, rows[] }]
//	}
//
// 零参数、direct 调用（宿主 `POST /api/pluginInventory.list`，见 DSH-GO
// `internal/transport/handlers_plugin_inventory_official.go:26-69`）。
//
// # 三个"缺席"的语义，客户端不许自己补
//
//   - `agentPresets` **整键缺席**表示这个部署没有组合 agent-preset roster；
//     它不是"空数组"，也不等于"有 preset 但没有行"。本仓宿主明确不产出
//     （`handlers_plugin_inventory_official.go:20-23`），所以界面必须能区分
//     "没有这个面" 与 "有这个面但为空"。
//   - `meta` 缺席 = 包没有本地显示元数据；官方此时回退到 `moduleName`
//     （`ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.tsx:94-100`）。
//     本仓宿主不产出 meta，所以这条回退是**唯一真实路径**。
//   - `fiberPhase: null` 是官方类型里的一个**合法取值**（"没有活着的 root Fiber"），
//     与"字段缺失"不同；所以这里用 `*string` 而不是 `string`。
package apiclient

import (
	"context"
	"encoding/json"
)

// PluginFiberPhase 是官方 `PluginFiberPhase` 的 wire 值
// （`dsh-host-plugin-inventory/src/types.ts:8-14`）。
//
// 官方类型是 `'pending' | 'loading' | 'active' | 'failed' | 'unloading' | null`，
// 而且 `fiberPhase` 在官方接口里是**必填**（`types.ts:25` 没有 `?`）。所以这里
// 保留"字段在不在"这一位：`Present=false` = 宿主没给（违约），
// `Present=true && Phase==""` = 官方 `null`（没有活着的 root Fiber）。
//
// 两者在界面上都不画阶段点，但**不许被解成同一件事**——那正是
// `settings.go` 里「字段缺失与字段是 null 不能混」的同一条规矩。
type PluginFiberPhase struct {
	// Present 表示宿主的 JSON 里真的出现了 `fiberPhase` 这个键。
	Present bool
	// Phase 是归一后的阶段；空串表示官方 `null`。
	Phase string
}

// UnmarshalJSON 只在键存在时被调用，所以它就是 `Present` 的来源。
func (p *PluginFiberPhase) UnmarshalJSON(raw []byte) error {
	p.Present = true
	p.Phase = ""
	if string(raw) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil
	}
	p.Phase = s
	return nil
}

// 官方五个阶段字面量（`types.ts:8-14`，逐个照抄，不增减）。
const (
	PluginPhasePending   = "pending"
	PluginPhaseLoading   = "loading"
	PluginPhaseActive    = "active"
	PluginPhaseFailed    = "failed"
	PluginPhaseUnloading = "unloading"
)

// PluginLocalizedText 是官方 `LocalizedText`（`dsh-package-manifest/src/types.ts:44`）：
//
//	string | { en: string; [locale: string]: string }
//
// 两种 wire 形状都认，**不猜**：解析失败就留空并由调用方回退到模块名，
// 而不是把 JSON 文本当标题显示出来。
type PluginLocalizedText struct {
	// Text 是纯字符串形态的值（对象形态为空）。
	Text string
	// Locales 是对象形态的字典（纯字符串形态为空）。
	Locales map[string]string
}

// UnmarshalJSON 同时接受字符串与对象两种官方形态。
func (t *PluginLocalizedText) UnmarshalJSON(raw []byte) error {
	*t = PluginLocalizedText{}
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		t.Text = s
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		// 两种官方形态都不是：**不报错、也不编值**，留空让调用方回退。
		return nil
	}
	t.Locales = m
	return nil
}

// Pick 按官方回退顺序取一种语言的文本：`zh` → `zh-CN` → `en` → 字典里字典序最小的一项。
//
// 官方客户端用的是 `ctx.locale.resolveText`（跟随当前界面语言）；本仓界面只有中文，
// 所以固定按中文优先，而**不是**随便挑一个键——挑错了会把日文当标题显示。
func (t PluginLocalizedText) Pick() string {
	if t.Text != "" {
		return t.Text
	}
	if len(t.Locales) == 0 {
		return ""
	}
	for _, key := range []string{"zh", "zh-CN", "zh-Hans", "en"} {
		if value, ok := t.Locales[key]; ok && value != "" {
			return value
		}
	}
	best, bestKey := "", ""
	for key, value := range t.Locales {
		if value == "" {
			continue
		}
		if bestKey == "" || key < bestKey {
			best, bestKey = value, key
		}
	}
	return best
}

// PluginInventoryMeta 是官方 `PluginLocalizedMeta`（`types.ts:46-56`）。
//
// 官方字段全部**可选**：缺席就是没有，本仓不往空串上凑。
type PluginInventoryMeta struct {
	// Title 是显示标题；缺席时官方回退到 `moduleName`（本仓宿主不产出）。
	Title PluginLocalizedText `json:"title,omitempty"`
	// Description 是显示简介；缺席时官方不画简介行。
	Description PluginLocalizedText `json:"description,omitempty"`
	// Icon 是包根 manifest 图标导出的 base64 data URL（官方要求按图片渲染，不当内联标记）。
	Icon string `json:"icon,omitempty"`
	// Error 是本地元数据诊断；**插件仍然可管理**，所以它不改变条目的启用状态。
	Error string `json:"error,omitempty"`
}

// PluginInventoryEntry 是官方 `PluginInventoryEntry`（`types.ts:16-28`）。
type PluginInventoryEntry struct {
	// EntryID 是 Loader 树里这一条的稳定身份。
	EntryID string `json:"entryId"`
	// ModuleName 是 Loader 条目实际 import 的模块 specifier。
	ModuleName string `json:"moduleName"`
	// Meta 是本地包显示元数据；nil = 包没带（官方回退到 ModuleName）。
	Meta *PluginInventoryMeta `json:"meta,omitempty"`
	// Enabled 是生效的启用状态（含被祖先 group 禁掉的情况）。
	Enabled bool `json:"enabled"`
	// FiberPhase 是 root Fiber 生命周期（官方必填；`null` 是合法取值）。
	FiberPhase PluginFiberPhase `json:"fiberPhase"`
	// Diagnostic 是当前启动失败信息（没有失败 Fiber 时缺席）。
	Diagnostic string `json:"diagnostic,omitempty"`
}

// Phase 返回归一后的阶段：空串 = 官方 `null`（没有活着的 Fiber）。
//
// 空串**不是**"未知"：调用方不许再补一个界面值。
func (e PluginInventoryEntry) Phase() string { return e.FiberPhase.Phase }

// PluginPresetEnablement 是官方 `PresetPluginEnablement`（`types.ts:30-31`）：
// `boolean | 'conditional'`。
type PluginPresetEnablement struct {
	// Value 是 `true` / `false`；Conditional 为真时它没有意义。
	Value bool
	// Conditional 对应官方字面量 `'conditional'`（disable 表达式要 Loader 上下文才能求值）。
	Conditional bool
}

// UnmarshalJSON 同时接受布尔与 `'conditional'` 两种官方形态。
func (p *PluginPresetEnablement) UnmarshalJSON(raw []byte) error {
	*p = PluginPresetEnablement{}
	var cond string
	if err := json.Unmarshal(raw, &cond); err == nil {
		p.Conditional = cond == "conditional"
		return nil
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	p.Value = value
	return nil
}

// AgentPresetPluginRow 是官方 `AgentPresetPluginRow`（`types.ts:33-51`）。
type AgentPresetPluginRow struct {
	// EntryID 是组合行 id；官方允许 null（行自己没声明）。
	EntryID *string `json:"entryId"`
	// ModuleName 是这一行点名的模块 specifier。
	ModuleName string `json:"moduleName"`
	// Meta 是本地显示元数据。
	Meta *PluginInventoryMeta `json:"meta,omitempty"`
	// Enabled 是生效启用状态（含祖先 group 与被预设挂载的语义）。
	Enabled PluginPresetEnablement `json:"enabled"`
	// Condition 是这一行自己的 `!!js` 禁用表达式（有才出现）。
	Condition string `json:"condition,omitempty"`
	// FiberPhase 是组合活着时的 root Fiber 阶段，否则官方 `null`。
	FiberPhase PluginFiberPhase `json:"fiberPhase"`
}

// AgentPresetPluginGroup 是官方 `AgentPresetPluginGroup`（`types.ts:53-65`）。
type AgentPresetPluginGroup struct {
	// ID 是稳定的 preset id。
	ID string `json:"id"`
	// Name 是 preset 自己发布的显示名；缺席时读者回退到 ID。
	Name string `json:"name,omitempty"`
	// IsDefault 表示没有点名 preset 的会话组合的就是它。
	IsDefault bool `json:"isDefault"`
	// Broken 说明这份组合读不出来（有值则 Rows 为空）。
	Broken string `json:"broken,omitempty"`
	// Rows 是组合顺序里的插件行。
	Rows []AgentPresetPluginRow `json:"rows"`
}

// PluginInventoryResult 是官方 `PluginInventorySnapshot`（`types.ts:67-77`）。
type PluginInventoryResult struct {
	// ManagementAvailable 表示这个宿主暴露了持久化的 profile 管理。
	ManagementAvailable *bool `json:"managementAvailable,omitempty"`
	// Entries 是当前 Loader 的非 group 条目（**按 Loader 顺序**，官方不做排序）。
	Entries []PluginInventoryEntry `json:"entries"`
	// AgentPresets 只有部署里组合了 agent-preset roster 时才出现；
	// nil 与"空数组"不同：前者=没有这个面，后者=有面但没有 preset。
	AgentPresets []AgentPresetPluginGroup `json:"agentPresets,omitempty"`
}

// PluginInventoryList 读当前插件清单（官方 `pluginInventory.list`）。
//
// 零参数、direct 调用。宿主没接线时回四字段 501 `not_implemented`（*APIError），
// **不是**空 entries——调用方必须区分"没这个能力"与"清单是空的"。
func (c *Client) PluginInventoryList(ctx context.Context) (PluginInventoryResult, error) {
	var out PluginInventoryResult
	if err := c.Call(ctx, "pluginInventory.list", nil, &out); err != nil {
		return PluginInventoryResult{}, err
	}
	return out, nil
}
