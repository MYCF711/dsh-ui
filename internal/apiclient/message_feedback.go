// message_feedback.go —— 官方 rc.2 `messageFeedback.list|put|delete` 的客户端。
//
// 官方语义（`dsh-client-ui-message-feedback/src/client/controller.ts:1-9`）：
// 宿主持有每条消息反馈的 sidecar，**每条写入带一个观察到的 version**；
// 版本冲突回 `{ok:false,error:{code:"version-conflict",current:item|null}}`，
// 客户端用回包里的 authoritative item 就地收敛，而不是重拉整个会话。
//
// 本仓宿主已实现在 `HOST:cmd/dshd/serve_agent_api.go:1478-1665`，
// 业务失败是 **HTTP 200 + ok:false**（不是四字段错误），所以这里必须保留
// union 形状；`Call` 只负责传输层失败。
package apiclient

import (
	"context"
	"encoding/json"
)

// MessageFeedbackItem 是宿主 `OfficialMessageFeedbackItem` 的投影。
type MessageFeedbackItem struct {
	MessageID string `json:"messageId"`
	Rating    string `json:"rating"`
	Note      string `json:"note,omitempty"`
	Category  string `json:"category,omitempty"`
	Version   string `json:"version"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// MessageFeedbackFailure 是业务失败体。
//
// Current 是 **json 原文**：`version-conflict` 要求它存在且可为 `null`，
// 其它码不该带这个键——用 RawMessage 才能区分"没有这个键"与"显式的 null"。
type MessageFeedbackFailure struct {
	Code      string          `json:"code"`
	SessionID string          `json:"sessionId,omitempty"`
	MessageID string          `json:"messageId,omitempty"`
	Current   json.RawMessage `json:"current,omitempty"`
	MaxBytes  *int64          `json:"maxBytes,omitempty"`
	Actual    *int64          `json:"actualBytes,omitempty"`
}

// FeedbackRating 是官方枚举（宿主只收这两个值）。
const (
	FeedbackRatingPositive = "positive"
	FeedbackRatingNegative = "negative"
)

// FeedbackCategory 是官方七值枚举（`HOST:internal/transport/handlers_agent_official.go:814-820`）。
const (
	FeedbackCategoryOther                     = "other"
	FeedbackCategoryTaskResult                = "task-result"
	FeedbackCategoryInstructionFollowing      = "instruction-following"
	FeedbackCategoryProductInteraction        = "product-interaction"
	FeedbackCategoryServiceStability          = "service-stability"
	FeedbackCategoryResourceCost              = "resource-cost"
	FeedbackCategorySecurityPrivacyPermission = "security-privacy-permission"
)

// MessageFeedbackListResult 是 `list` 的 union。
type MessageFeedbackListResult struct {
	OK    bool                      `json:"ok"`
	Value *MessageFeedbackListValue `json:"value,omitempty"`
	Error *MessageFeedbackFailure   `json:"error,omitempty"`
}

// MessageFeedbackListValue 是列表结果的 value 段。
type MessageFeedbackListValue struct {
	Items []MessageFeedbackItem `json:"items"`
}

// MessageFeedbackPutResult 是 `put` 的 union。
type MessageFeedbackPutResult struct {
	OK    bool                    `json:"ok"`
	Value *MessageFeedbackItem    `json:"value,omitempty"`
	Error *MessageFeedbackFailure `json:"error,omitempty"`
}

// MessageFeedbackDeleteResult 是 `delete` 的 union。
type MessageFeedbackDeleteResult struct {
	OK    bool                        `json:"ok"`
	Value *MessageFeedbackDeleteValue `json:"value,omitempty"`
	Error *MessageFeedbackFailure     `json:"error,omitempty"`
}

// MessageFeedbackDeleteValue 是删除成功的稳定后置条件。
type MessageFeedbackDeleteValue struct {
	Absent bool `json:"absent"`
}

// MessageFeedbackList 读某会话的全部消息反馈。
func (c *Client) MessageFeedbackList(ctx context.Context, sessionID string) (MessageFeedbackListResult, error) {
	var out MessageFeedbackListResult
	raw, err := c.callRaw(ctx, "messageFeedback.list", map[string]any{"agentId": sessionID})
	if err != nil {
		return MessageFeedbackListResult{}, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return MessageFeedbackListResult{}, err
	}
	return out, nil
}

// MessageFeedbackPut 写一条消息反馈。
//
// ifVersion 为 nil 表示官方 wire 的 `null`（要求"当前不存在"）；
// 非 nil 是客户端上次观察到的 version（乐观并发）。
func (c *Client) MessageFeedbackPut(ctx context.Context, sessionID, messageID, rating, note, category string, ifVersion *string) (MessageFeedbackPutResult, error) {
	req := map[string]any{
		"sessionId": sessionID,
		"messageId": messageID,
		"rating":    rating,
		"ifVersion": ifVersion,
	}
	if note != "" {
		req["note"] = note
	}
	if category != "" {
		req["category"] = category
	}
	var out MessageFeedbackPutResult
	raw, err := c.callRaw(ctx, "messageFeedback.put", map[string]any{"request": req})
	if err != nil {
		return MessageFeedbackPutResult{}, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return MessageFeedbackPutResult{}, err
	}
	return out, nil
}

// MessageFeedbackDelete 撤回一条消息反馈。
func (c *Client) MessageFeedbackDelete(ctx context.Context, sessionID, messageID, ifVersion string) (MessageFeedbackDeleteResult, error) {
	req := map[string]any{
		"sessionId": sessionID,
		"messageId": messageID,
		"ifVersion": ifVersion,
	}
	var out MessageFeedbackDeleteResult
	raw, err := c.callRaw(ctx, "messageFeedback.delete", map[string]any{"request": req})
	if err != nil {
		return MessageFeedbackDeleteResult{}, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return MessageFeedbackDeleteResult{}, err
	}
	return out, nil
}

// FeedbackCurrent 解出 `error.current` 里的 authoritative item。
//
// 返回值：item（可能是 nil 表示"当前不存在"）、是否有这个键。
// 键不在 = 不是版本冲突；键在且是 null = 冲突且宿主说"现在没有"。
func (f *MessageFeedbackFailure) FeedbackCurrent() (*MessageFeedbackItem, bool) {
	if f == nil || len(f.Current) == 0 {
		return nil, false
	}
	if string(f.Current) == "null" {
		return nil, true
	}
	var item MessageFeedbackItem
	if err := json.Unmarshal(f.Current, &item); err != nil {
		return nil, false
	}
	return &item, true
}

// FeedbackCategoryValues 是官方枚举的展示顺序（与 `FeedbackDialog.tsx:30-38` 一致）。
//
// 客户端只提供"顺序 + 值"；**中文文案在插件里**（官方 zh 字典），这一层不写用户文案。
func FeedbackCategoryValues() []string {
	return []string{
		FeedbackCategoryTaskResult,
		FeedbackCategoryInstructionFollowing,
		FeedbackCategoryProductInteraction,
		FeedbackCategoryServiceStability,
		FeedbackCategoryResourceCost,
		FeedbackCategorySecurityPrivacyPermission,
		FeedbackCategoryOther,
	}
}
