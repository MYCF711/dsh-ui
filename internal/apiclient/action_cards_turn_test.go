package apiclient

// action_cards_turn_test.go —— 按轮取卡（官方 `proposalOf`）的两条规则。
//
// 官方（OFF:dsh-client-ui-action-cards/src/client/Card.tsx:69-76）：
//
//	只认 proposal.turn === turn
//	同轮多张取**最小的 callId**（投影理论上不会出现，但规则是明写的）
//
// turnTail 座位按轮派发之后，"取哪一张卡"必须由这一轮决定；`LatestProposal`
// 只剩"没有轮次基线"时的那一档回退。

import "testing"

func turnFixtureView() *ActionCardView {
	return &ActionCardView{Proposals: map[string]ActionCardProposal{
		"call-old":   {CallID: "call-old", Turn: 1, Action: "kind-a", Title: "旧"},
		"call-new-b": {CallID: "call-new-b", Turn: 3, Action: "kind-a", Title: "新乙"},
		"call-new-a": {CallID: "call-new-a", Turn: 3, Action: "kind-a", Title: "新甲"},
	}}
}

// TestProposalForTurnPicksThisTurn 钉住"只认这一轮"。
func TestProposalForTurnPicksThisTurn(t *testing.T) {
	view := turnFixtureView()
	got, ok := view.ProposalForTurn(1)
	if !ok || got.CallID != "call-old" {
		t.Fatalf("第 1 轮 = %+v（ok=%v），want call-old", got, ok)
	}
	if _, ok := view.ProposalForTurn(2); ok {
		t.Fatal("第 2 轮没有任何提案，不该返回一张卡（空槽不占位）")
	}
}

// TestProposalForTurnPicksLowestCallID 钉住"同轮多张取最小 callId"。
func TestProposalForTurnPicksLowestCallID(t *testing.T) {
	view := turnFixtureView()
	got, ok := view.ProposalForTurn(3)
	if !ok || got.CallID != "call-new-a" {
		t.Fatalf("第 3 轮 = %+v（ok=%v），want call-new-a（同轮最小 callId）", got, ok)
	}
}

// TestLatestProposalStaysSessionFallback 钉住回退那一档没被这次改造改坏：
// 没有轮次基线时仍按"最新一轮 + 最小 callId"取。
func TestLatestProposalStaysSessionFallback(t *testing.T) {
	view := turnFixtureView()
	got, ok := view.LatestProposal()
	if !ok || got.CallID != "call-new-a" {
		t.Fatalf("回退档 = %+v（ok=%v），want call-new-a", got, ok)
	}
	if _, ok := (*ActionCardView)(nil).ProposalForTurn(1); ok {
		t.Fatal("nil 投影不该返回卡片")
	}
}
