// projections.go —— 会话投影（官方 `session/projections` 的客户端，解界面真消费的四个键）。
//
// # 为什么需要它
//
// 官方 rc.2 的子代理目录（`ui-subagent`）、Team 开关（`ui-agent-team`）与状态栏
// 活动读数（`ui-stats` 的 ActivityPill）的数据都来自 **Session projection**，
// 不是某个 get 方法：
//
//	subagentCatalog   直接子代理清单（ui-subagent 的行数据）
//	teamMode          Team 模式开关的持久值（ui-agent-team 的 enabled）
//	sessionStats      轮/步计数与墙钟（ActivityPill 的整份读数，八个字段）
//	modelSelection    最近一次模型选择（Team 开关的 `modelSelection !== undefined`
//	                  可用性判据 + 关闭时的 `lastUsed !== null` 确认判据）
//
// 本仓宿主**已经实现**这个面（DSH-GO `handlers_session_official.go:875`
// `handleSessionProjections`，值集合见 `officialProjectionValues`），但界面侧
// 此前没有任何客户端去取——于是子代理目录容器没有数据可画（GB 报告的
// "4 个父容器不存在"里，这一条的数据面同样缺）。
//
// # 契约（宿主侧逐字核对）
//
//	方法   session.projections
//	入参   {sessionId}（officialSessionRequest.SessionID，json `sessionId`）
//	结果   {asOfSeq, values}；values 是 map[string]any，**键缺失 = 该能力不存在**
//	       （官方 values 里这些键都是 optional；宿主注释明写"三个键仍然不出现"）
//
// # 两条"不编"
//
//  1. 键缺失 → `Known=false`。界面据此隐藏，**不是**画成"0 个子代理"。
//  2. `values` 里键存在但值为 null → 同样 `Known=false`（null 是"没有"，不是空数组）。
package apiclient

import (
	"context"
	"encoding/json"
)

// SessionProjections 是官方 `session.projections` 的结果（非 null 分支）。
//
// Values 保留原始 JSON：官方 projections 是开放集合（十几个键、各有自己的
// schema），全量建模等于把宿主 schema 抄一份到界面里，改一个字段就要两边同步。
// 只解界面真用到的键（`SubagentCatalog` / `TeamMode`），其余原样不动。
type SessionProjections struct {
	// AsOfSeq 是这份投影对应的 journal 水位；-1 = 空会话（宿主约定）。
	AsOfSeq int64 `json:"asOfSeq"`
	// Values 是投影键 → 原始值。
	Values map[string]json.RawMessage `json:"values"`
}

// raw 取一个键的原始值；键缺失或字面 null 都返回 (nil, false)。
func (p *SessionProjections) raw(key string) ([]byte, bool) {
	if p == nil || p.Values == nil {
		return nil, false
	}
	raw, ok := p.Values[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	return raw, true
}

// SubagentCatalogEntry 是官方 `subagentCatalog` 的一项
// （宿主 `OfficialSubagentCatalogEntry`，handlers_session_official.go:356）。
//
// Mode 是官方闭集四值：`continuable` | `one-shot` | `external` | `unknown`。
// 本宿主真实创建的子代理只可能是 continuable（durable `subagent/spawn`）。
type SubagentCatalogEntry struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"createdAt"`
	Mode      string `json:"mode"`
	// Label 官方对 continuable 必填；本宿主没有 durable 创建标签 → 空串。
	// 界面不许拿 id 冒充 label（那是另一种"编"）；空 label 时按官方做法只显示兜底行。
	Label string `json:"label"`
}

// SubagentCatalog 解 `subagentCatalog`。返回 (nil, false) = 该键不存在/为 null
// → 目录**未知**（隐藏），不是"空目录"。
func (p *SessionProjections) SubagentCatalog() ([]SubagentCatalogEntry, bool) {
	raw, ok := p.raw("subagentCatalog")
	if !ok {
		return nil, false
	}
	var entries []SubagentCatalogEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, false
	}
	if entries == nil {
		entries = []SubagentCatalogEntry{}
	}
	return entries, true
}

// TeamModeProjection 是宿主的 `teamMode` 投影（rc.2 新增折叠，
// `officialTeamModeFold`：只认最后一次 `team/mode` 事件）。
type TeamModeProjection struct {
	Enabled bool `json:"enabled"`
}

// TeamMode 解 `teamMode`。返回 (nil, false) = 从没出现过 `team/mode` 事件
// → Team 模式**未知**（界面不改本地值、不画成"已关闭"）。
func (p *SessionProjections) TeamMode() (*TeamModeProjection, bool) {
	raw, ok := p.raw("teamMode")
	if !ok {
		return nil, false
	}
	var mode TeamModeProjection
	if err := json.Unmarshal(raw, &mode); err != nil {
		return nil, false
	}
	return &mode, true
}

// SessionStats 是宿主 `sessionStats` 投影的 wire 视图（八个字段，全部非负）。
//
// 官方形状（`@deepseek-ai/dsh-session-stats/lib/types/projection.js:154-167`；
// 逐字段语义 `src/types.ts:24-41`）：
//
//	turns        至少闭合过一个 step 的轮次数（step/end 去重）
//	steps        已闭合的 step 数（完成/失败/取消/max-tokens 各算一步）
//	llmMs        step/start → assistant/message 的墙钟之和（毫秒）
//	toolMs       tool/call → tool/result 配对墙钟之和（毫秒，按 callId）
//	ttftMs       首 token 延迟之和（毫秒）
//	ttftSteps    记录了首 token 的 step 数
//	decodeMs     first token → 组装消息的墙钟之和（毫秒）
//	decodeTokens 同批 step 上 provider 上报的 output tokens 之和
//
// 为什么整个结构体不带 `Known`：**只有"有没有这个投影"这一种未知**，那是
// `SessionStats()` 的第二个返回值；进入结构体之后每个字段都是宿主的真值
// （包括真实的 0）。把两者混在一起就是下一次"未知被写成 0"的来源。
type SessionStats struct {
	Turns        int64 `json:"turns"`
	Steps        int64 `json:"steps"`
	LLMMs        int64 `json:"llmMs"`
	ToolMs       int64 `json:"toolMs"`
	TTFTMs       int64 `json:"ttftMs"`
	TTFTSteps    int64 `json:"ttftSteps"`
	DecodeMs     int64 `json:"decodeMs"`
	DecodeTokens int64 `json:"decodeTokens"`
}

// SessionStats 解 `sessionStats`。返回 (nil, false) = 宿主没给这个投影
// （老宿主 / 查询失败）→ 状态栏「活动」写「未知」，**不许拿消息窗口凑轮/步**。
//
// 键存在就返回真值，哪怕八个数全是 0：那是"空会话/没有 step"的真话，
// 由界面决定写什么（官方 ActivityPill 在 `steps === 0` 时整枚不渲染）。
func (p *SessionProjections) SessionStats() (*SessionStats, bool) {
	raw, ok := p.raw("sessionStats")
	if !ok {
		return nil, false
	}
	var stats SessionStats
	if err := json.Unmarshal(raw, &stats); err != nil {
		return nil, false
	}
	return &stats, true
}

// ModelSelectionEntry 是一次模型选择（宿主 `OfficialModelSelection`，
// `handlers_session_official.go:302`）。
type ModelSelectionEntry struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoningEffort,omitempty"`
}

// ModelSelectionProjection 是 `modelSelection` 投影（`{lastUsed,next}`）。
//
// 两侧都允许 null：宿主没给真值的那一侧就是 null（**键本身仍然出现**，
// 见宿主 `officialProjectionValues` 的注释）。所以"键在但两侧都空"与
// "键不在"是两件事：前者 Team 开关可用（只是没有历史选择），后者未知。
type ModelSelectionProjection struct {
	LastUsed *ModelSelectionEntry `json:"lastUsed"`
	Next     *ModelSelectionEntry `json:"next"`
}

// ModelSelection 解 `modelSelection`。返回 (nil, false) = 键缺失/为 null
// → Team 开关的可用性未知（官方 `modelSelection === undefined` → 禁用）。
func (p *SessionProjections) ModelSelection() (*ModelSelectionProjection, bool) {
	raw, ok := p.raw("modelSelection")
	if !ok {
		return nil, false
	}
	var selection ModelSelectionProjection
	if err := json.Unmarshal(raw, &selection); err != nil {
		return nil, false
	}
	return &selection, true
}

// SessionProjections 取一条会话的当前投影（官方 session/projections）。
func (c *Client) SessionProjections(ctx context.Context, sessionID string) (*SessionProjections, error) {
	var out SessionProjections
	if err := c.Call(ctx, "session.projections", map[string]any{"sessionId": sessionID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
