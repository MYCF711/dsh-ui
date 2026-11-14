// action_cards.go —— 官方动作卡（`dsh-client-ui-action-cards`）的两个数据面。
//
// # 官方结构（唯一权威）
//
//	OFF = DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/dsh-client-ui-action-cards
//	OFF:src/client/Card.tsx:69-92     卡片只从 `useProjection('actionCards')` 取数据
//	OFF:src/client/mount.ts:54-57     用户动作 = `ctx.remote.actionCards.act(sessionId, callId, verb, payload)`
//	OFF:src/client/registry.ts:34     结果的四变体（accepted / settled / refused / unconfirmed）
//
// # 宿主证据（逐条核过，不是注释抄的）
//
//	HOST = DSH-GO/tmp/GO8-RC2/_audit-tree
//	HOST:cmd/dshd/serve_action_cards.go:55-57   attachActionCardsView(srv, home) → srv.AttachActionCardsView(actioncards.Open(home).ViewMap)
//	HOST:cmd/dshd/serve.go:926                  accountAPI.ActionCardAct = actionCardAct(*home)
//	HOST:cmd/dshd/serve.go:941                  attachActionCardsView(srv, *home)（真在产品启动路径上）
//	HOST:internal/transport/handlers_action_cards_view.go:38-52  投影值来源（session.projections 的 actionCards 键）
//	HOST:internal/transport/handlers_session_official.go:947      values["actionCards"] = view
//	HOST:internal/transport/handlers_account_official.go:1447-1502  actionCards.act handler（参数/四变体校验）
//	HOST:internal/actioncards/service.go:216-235  View / ViewMap
//	HOST:internal/actioncards/service.go:237-331  Act（含 accepted/settled/refused/unconfirmed 出口）
//	HOST:internal/actioncards/service.go:36-42    四个 status 常量
//	HOST:internal/actioncards/service.go:111-133  Proposal / Decision / View 的 JSON 形状
//
// # 两个"不编"
//
//  1. **`actionCards.view` 不是宿主方法**。官方视图是 Session projection
//     （`session.projections` 的 `actionCards` 键），宿主侧挂在
//     `AttachActionCardsView` 上；宿主方法表里只有 `actionCards.act`
//     （HOST:internal/transport/handlers_account_official_test.go:174-191 的方法集合，
//     其中 `"actionCards.view"` 明确列在"不许被当成方法"的名单里）。
//     所以这里不伪造一条 `actionCards.view` 的 HTTP 方法，`ActionCardsView` 走真实的
//     `session.projections`。
//  2. 投影键缺失 = **宿主没有这个投影** → 返回 `(nil, nil)`，调用方整块不画；
//     不是"空卡片列表"，也不是错误。
package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
)

// 官方 `ActionCardResult.status` 的四个可判别值（HOST:internal/actioncards/service.go:36-42）。
const (
	ActionCardAccepted    = "accepted"
	ActionCardSettled     = "settled"
	ActionCardRefused     = "refused"
	ActionCardUnconfirmed = "unconfirmed"
)

// ActionCardProposal 是被采纳的一张卡（HOST:internal/actioncards/service.go:111-119；
// 官方 `ActionCardProposal`，OFF:../dsh-action-cards/src/types.ts:12-24）。
//
// 字段与 json tag 逐字对齐宿主 View 的 wire 形状（不是界面自己起的名字）。
type ActionCardProposal struct {
	// CallID 是 `propose_action` 工具调用 id（官方 callId）。
	CallID string `json:"callId"`
	// Turn 是这张卡跟随的那一轮（官方 turn）。
	Turn int64 `json:"turn"`
	// Action 是注册的 kind 名（官方 action）。
	Action string `json:"action"`
	// Title 是卡片标题（官方 title）。
	Title string `json:"title"`
	// Brief 是"这个动作做什么"，用户语言（官方 brief）。
	Brief string `json:"brief"`
	// Details 是 kind 自己校验的字段（官方 details，JSON 对象）。
	Details map[string]json.RawMessage `json:"details"`
}

// ActionCardDecision 是已记录的卡片决定（HOST:internal/actioncards/service.go:121-127）。
//
// Phase 是开放串（kind 自定义），只有 `dismissed` / `removed` / `replaced` 是共享语义；
// `replaced` 由日志派生，永远不会被 kind 直接记录。
type ActionCardDecision struct {
	CallID string                     `json:"callId"`
	Action string                     `json:"action"`
	Phase  string                     `json:"phase"`
	Data   map[string]json.RawMessage `json:"data,omitempty"`
}

// ActionCardView 是官方 `ActionCardView`（OFF:../dsh-action-cards/src/types.ts:39-43）：
// 打开中的会话里，按 callId 索引的提案与决定。
type ActionCardView struct {
	Proposals map[string]ActionCardProposal `json:"proposals"`
	Decisions map[string]ActionCardDecision `json:"decisions"`
}

// Empty 报告这份投影里有没有任何卡片。
//
// 官方"每轮最多一张卡"；`proposals` 与 `decisions` 都空 = 这个会话没有动作卡，
// 界面据此整块不画（空槽不占位）。
func (v *ActionCardView) Empty() bool {
	return v == nil || len(v.Proposals) == 0
}

// Decision 取一张卡的决定；没有记录返回 (nil, false)。
//
// 用 `(值, bool)` 而不是"零值即没有"：Phase 是开放串，零值 `""` 不是一个合法的
// 决定，两者混在一起会让"没记录"看起来像"记录了一个空 phase"。
func (v *ActionCardView) Decision(callID string) (ActionCardDecision, bool) {
	if v == nil || callID == "" {
		return ActionCardDecision{}, false
	}
	decision, ok := v.Decisions[callID]
	return decision, ok
}

// LatestProposal 取"这一屏该画的那一张卡"。
//
// 官方判据（OFF:src/client/Card.tsx:69-76）：`turnTail` 座位**按轮**取；
// 若同一轮里出现多张（宿主投影理论上不会），取**最小的 callId**。本仓的
// turnTail 座位是会话级（见 plugins/dsactioncards 的包注释"没搬的细节"），
// 所以按"最新一轮 + 最小 callId"选一张：这与官方在同一轮里的择一规则一致，
// 只是把"哪一轮"从座位参数换成了投影里的最新 turn。
func (v *ActionCardView) LatestProposal() (ActionCardProposal, bool) {
	if v == nil || len(v.Proposals) == 0 {
		return ActionCardProposal{}, false
	}
	var best ActionCardProposal
	var found bool
	for _, proposal := range v.Proposals {
		switch {
		case !found:
			best, found = proposal, true
		case proposal.Turn > best.Turn:
			best = proposal
		case proposal.Turn == best.Turn && proposal.CallID < best.CallID:
			best = proposal
		}
	}
	return best, found
}

// ActionCardResult 是 `actionCards.act` 的响应（官方四变体联合，
// HOST:internal/transport/handlers_account_official.go:1447-1502）。
//
// 判别方式：`Status` 四值；`settled` 必带 Decision；`refused` / `unconfirmed`
// 必带 Reason（宿主在出口处**自己校验**这三条，不合约回 500，所以客户端拿到
// 的响应一定满足它们）。
type ActionCardResult struct {
	Status   string              `json:"status"`
	Decision *ActionCardDecision `json:"decision,omitempty"`
	Reason   string              `json:"reason,omitempty"`
}

// DecodeActionCardResult 把 `ActionCardAct` 的原始响应解成四变体。
//
// 为什么让 `ActionCardAct` 返回 `json.RawMessage`：状态是**联合**，一条固定
// struct 会把"accepted 没有 decision"和"宿主漏了字段"混成同一个零值。解一次、
// 在这里判一次，调用方拿到的就是"宿主真的回了什么"。
func DecodeActionCardResult(raw json.RawMessage) (ActionCardResult, error) {
	var result ActionCardResult
	if len(raw) == 0 {
		return result, fmt.Errorf("actionCards.act 的响应是空 body（官方必回四变体之一）")
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return ActionCardResult{}, fmt.Errorf("actionCards.act 的响应不是合法 JSON: %w", err)
	}
	switch result.Status {
	case ActionCardAccepted:
	case ActionCardSettled:
		if result.Decision == nil {
			return ActionCardResult{}, fmt.Errorf("actionCards.act 回 settled 但缺 decision（官方必填）")
		}
	case ActionCardRefused, ActionCardUnconfirmed:
		if result.Reason == "" {
			return ActionCardResult{}, fmt.Errorf("actionCards.act 回 %s 但缺 reason（官方必填）", result.Status)
		}
	default:
		return ActionCardResult{}, fmt.Errorf("actionCards.act 回的 status %q 不在官方四值里", result.Status)
	}
	return result, nil
}

// ActionCardsView 取一条会话的官方 actionCards 投影。
//
// 数据路径（不是自造的 get 方法）：
//
//	session.projections {sessionId} → values.actionCards
//	HOST:internal/transport/handlers_session_official.go:947  values["actionCards"] = view
//	HOST:internal/transport/handlers_action_cards_view.go:38-52  该值来自 AttachActionCardsView
//
// 三种真实回答：
//
//	(view, nil)      宿主挂了投影源：view 是**真值**
//	(nil, nil)       宿主没挂投影源（values 里没有这个键）→ 界面整块不画
//	(nil, err)       查询失败 → 保留上一帧的显示，不要把卡片清成"没有"
func (c *Client) ActionCardsView(ctx context.Context, sessionID string) (*ActionCardView, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("ActionCardsView 需要非空 sessionId")
	}
	var projections SessionProjections
	if err := c.Call(ctx, "session.projections", map[string]any{"sessionId": sessionID}, &projections); err != nil {
		return nil, err
	}
	raw, ok := projections.raw("actionCards")
	if !ok {
		return nil, nil
	}
	var view ActionCardView
	if err := json.Unmarshal(raw, &view); err != nil {
		return nil, fmt.Errorf("actionCards 投影不是合法 JSON: %w", err)
	}
	if view.Proposals == nil {
		view.Proposals = map[string]ActionCardProposal{}
	}
	if view.Decisions == nil {
		view.Decisions = map[string]ActionCardDecision{}
	}
	return &view, nil
}

// ActionCardAct 对一个已记录的卡片执行一次用户动作（官方 `actionCards.act`）。
//
// 契约（HOST:internal/transport/handlers_account_official.go:1447-1502）：
//
//	入参  {sessionId, callId, verb, payload?}（平铺；宿主 strAlias 也认官方 wire 名
//	      `agentId`，见 HOST:internal/transport/handlers_agent_official.go:511-520）
//	结果  accepted | settled{decision} | refused{reason} | unconfirmed{reason}
//	错误  400（callId/verb 空、payload 不是对象、未知字段）/
//	      501（宿主没有 action card 运行时）/
//	      500（宿主给出非法 status、settled 缺 decision、refused 缺 reason）
//
// payload 为 nil 时不发这个键：宿主对"缺 payload"和"payload 是空对象"是同一语义
// （HOST:internal/transport/handlers_account_official.go:1472-1480）。
//
// 返回**原始 JSON**：四变体的判别留给 DecodeActionCardResult，避免这一层把
// "accepted 没有 decision"悄悄补成零值。
func (c *Client) ActionCardAct(ctx context.Context, sessionID, callID, verb string, payload map[string]json.RawMessage) (json.RawMessage, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("ActionCardAct 需要非空 sessionId")
	}
	if callID == "" {
		return nil, fmt.Errorf("ActionCardAct 需要非空 callId")
	}
	if verb == "" {
		return nil, fmt.Errorf("ActionCardAct 需要非空 verb（官方只允许 dismiss 或 kind 自己的动词）")
	}
	params := map[string]any{
		"sessionId": sessionID,
		"callId":    callID,
		"verb":      verb,
	}
	if payload != nil {
		params["payload"] = payload
	}
	var raw json.RawMessage
	if err := c.Call(ctx, "actionCards.act", params, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
