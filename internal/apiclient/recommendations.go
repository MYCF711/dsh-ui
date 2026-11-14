// recommendations.go —— 官方插件推荐卡的**目录文案**通道
// （`pluginRecommendations.describe`，`dsh-client-ui-plugin-recommendations`）。
//
// # 官方契约（唯一权威）
//
//	OFF = DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/dsh-client-ui-plugin-recommendations
//	OFF:src/client/Card.tsx:44-74      正文按 `proposal.details.name` 去查目录文案
//	OFF:src/client/Card.tsx:87-95      describe 的结果分三档：undefined / null(丢包) / available|refused
//	OFF:src/client/mount.ts:40-44      `describe: async (sessionId, callId) => result.ok ? result.value : undefined`
//
// # 宿主证据（本仓真的实现了，不是注释抄的）
//
//	HOST:internal/transport/handlers_plugin_recommendations_official.go:53-84
//	      handler：参数 `sessionId|agentId|id`（别名）+ `callId`，两个都非空才往下走。
//	HOST:internal/recommendations/service.go:120-181
//	      真实现：目录来自 pluginMarket 索引（真实 HTTP），启用状态来自 pluginManager
//	      state；`available` 回 `{status,title,description,enabled}`，其余回 `{status:"refused",reason}`。
//	HOST:internal/recommendations/service.go:183-185   refused 的五个 reason
//
// # 为什么结果是一个判别联合而不是一个 struct
//
// `available` 有 title/description/enabled，`refused` 只有 reason——把两者压成一个
// struct 会让"没查到的插件"看起来像"标题为空串的插件"，界面就会画一张没有标题的
// 卡片。所以这里先解 `status`，再按分支解各自字段（与 `DecodeActionCardResult`
// 同一个理由）。
package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
)

// 官方 `RecommendationDetails` 的两个 status（OFF:../dsh-plugin-recommendations/src/types.ts）。
const (
	// PluginRecommendationAvailable 是"目录里有这个插件，且它可以被启用"。
	PluginRecommendationAvailable = "available"
	// PluginRecommendationRefused 是"这次查询被拒"，理由见 Reason。
	PluginRecommendationRefused = "refused"
)

// 官方五个拒绝理由（HOST:internal/recommendations/service.go:183-185 与 OFF:Card.tsx:57-62
// 的 REASON_KEYS 一一对应）。界面按这五条取本地化文案，不自己编第六种。
const (
	PluginRecommendationReasonUnknown      = "unknown"
	PluginRecommendationReasonUnavailable  = "unavailable"
	PluginRecommendationReasonAccessDenied = "access-denied"
	PluginRecommendationReasonNotLiveRoot  = "not-live-root"
	PluginRecommendationReasonClosing      = "closing"
)

// PluginRecommendationDetails 是一次 describe 的结果（判别联合）。
//
// Available() 是唯一的判据：不要看 Title 是否为空来猜状态。
type PluginRecommendationDetails struct {
	// Status 是 PluginRecommendationAvailable 或 PluginRecommendationRefused。
	Status string
	// Title / Description 只在 available 时有意义：宿主已按 en 本地化
	// （HOST:service.go:172-177 的 `localized(plugin.Title, "en", …)`）。
	Title       string
	Description string
	// Enabled 是**插件管理器**里的真实启用状态，不是推荐卡的记录状态。
	Enabled bool
	// Reason 只在 refused 时有意义。
	Reason string
}

// Available 报告这份详情是否是"目录里有、可启用"的那一支。
func (d PluginRecommendationDetails) Available() bool {
	return d.Status == PluginRecommendationAvailable
}

// PluginRecommendationDescribe 读一张推荐卡的目录文案。
//
// sessionID 必须是**这张卡所在的那个会话**（宿主按它找 v4 日志再校验卡片归属：
// 非 live root 直接 refused{not-live-root}），callID 是 `propose_action` 的调用 id。
// 两个都是空串时**在本地就拒绝**：宿主对空 sessionId 会回 bad_request，但更重要的是
// 界面永远不该发出一个不知道自己问的是哪张卡的请求。
func (c *Client) PluginRecommendationDescribe(ctx context.Context, sessionID, callID string) (*PluginRecommendationDetails, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("PluginRecommendationDescribe 需要非空 sessionId")
	}
	if callID == "" {
		return nil, fmt.Errorf("PluginRecommendationDescribe 需要非空 callId")
	}
	var raw json.RawMessage
	if err := c.Call(ctx, "pluginRecommendations.describe",
		map[string]any{"sessionId": sessionID, "callId": callID}, &raw); err != nil {
		return nil, err
	}
	return DecodePluginRecommendationDetails(raw)
}

// DecodePluginRecommendationDetails 把宿主原始响应解成判别联合。
//
// 独立成导出函数是为了让**已经拿到 raw 的调用方**（壳的缓存、测试夹具）复用同一条
// 解码规则：两处各解一次，迟早有一处把 refused 当成 available。
func DecodePluginRecommendationDetails(raw json.RawMessage) (*PluginRecommendationDetails, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("pluginRecommendations.describe 返回空载荷")
	}
	var head struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, fmt.Errorf("pluginRecommendations.describe 结果不是 JSON: %w", err)
	}
	switch head.Status {
	case PluginRecommendationAvailable:
		var out struct {
			Status      string `json:"status"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Enabled     bool   `json:"enabled"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("available 详情形状不合约: %w", err)
		}
		return &PluginRecommendationDetails{
			Status: out.Status, Title: out.Title, Description: out.Description, Enabled: out.Enabled,
		}, nil
	case PluginRecommendationRefused:
		var out struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("refused 详情形状不合约: %w", err)
		}
		return &PluginRecommendationDetails{Status: out.Status, Reason: out.Reason}, nil
	default:
		// 未知 status：**不猜**。官方 Card.tsx 对未知分支同样不画可用态。
		return nil, fmt.Errorf("pluginRecommendations.describe 返回未知 status %q", head.Status)
	}
}
