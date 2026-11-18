package dsplan

// turn_cards_test.go —— 逐轮计划卡（官方 `PlanCards` 一轮一次）的机器断言。
//
// # 这一单修的是什么
//
// 官方座位 `conversation.chat.turnTail` 一轮一个（`turn-tail` 节点的 children：
// OFF:dsh-client-ui-chat/src/client/chat/register-node-renderers.ts:88-97），
// `PlanCards` 拿 owner props 里的 turn 只取**那一轮**的 `submitted-plan`
// （OFF:PlanCard.tsx:43-45 + OFF:ui-plan/src/client/index.ts:88-92）。
//
// 改造前本仓座位每帧只派发一次、不带轮次，于是本包只能 `latestTurn()`——两轮都有
// 计划时，第 1 轮那张卡永远不出现。现在壳按轮派发（`Env.Turn`，
// `internal/uikernel/shell_skeleton.go` 的 `dispatchTurnTails`），本包按那一轮过滤。
//
// 断言走**真实路径**：轮次表从 `uikernel.TurnTails`（壳用的同一个函数）取，
// 每轮设 `env.Turn` 后画真的一帧。变异（改回 `latestTurn`）会让第 1 轮那帧画出
// 第 3 轮的卡——下面两条断言同时红。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

// turnEnd 造一条 `turn/end`（`turn` 缺失在真实日志里意味着没有 turn-tail 节点）。
func turnEnd(t *testing.T, seq int, turn int64, reasonKind string) apiclient.Frame {
	t.Helper()
	data := map[string]any{"turn": turn}
	if reasonKind != "" {
		data["reason"] = map[string]any{"kind": reasonKind}
	}
	return frame(t, seq, "turn/end", data)
}

// perTurnFrames 造"两轮各一份计划"的会话。
func perTurnFrames(t *testing.T) []apiclient.Frame {
	t.Helper()
	return []apiclient.Frame{
		callFrame(t, 1, 1, "call-old", "exit_plan_mode", planArgs(t, "# 旧方案\n\n旧的正文。")),
		turnEnd(t, 2, 1, "completed"),
		callFrame(t, 3, 3, "call-new", "exit_plan_mode", planArgs(t, "# 新方案\n\n新的正文。")),
		turnEnd(t, 4, 3, "completed"),
	}
}

// TestCardsRenderEachTurnSeparately 钉住「两轮的计划卡都会画出来」，而且
// 每一帧只画**这一轮**的：两半缺一不可。
func TestCardsRenderEachTurnSeparately(t *testing.T) {
	p, env, _, canvas := fixture(t, perTurnFrames(t), 360)
	tails := uikernel.TurnTails(env.Data.Frames)
	if len(tails) != 2 {
		t.Fatalf("壳应派发 2 个按轮座位（第 1 轮、第 3 轮），得到 %d：%+v", len(tails), tails)
	}

	perTurn := map[int64]string{}
	for _, tail := range tails {
		turn := tail
		env.Turn = &turn // 壳在 dispatch 期间就是这么给的
		drawCardsOnce(t, p, env, canvas, 520, 240)
		perTurn[turn.Turn] = joinedTexts(canvas)
	}
	env.Turn = nil // 派发结束恢复（壳的约定）

	old, okOld := perTurn[1]
	newer, okNew := perTurn[3]
	if !okOld || !okNew {
		t.Fatalf("座位轮次 = %v，want 1 与 3", perTurn)
	}
	// 半 1：两轮各自的卡都画出来了（改造前第 1 轮什么都没有）。
	if !contains(old, "旧方案") {
		t.Errorf("第 1 轮的 Turn 尾必须画出这一轮的计划「旧方案」；实际文字=%q", old)
	}
	if !contains(newer, "新方案") {
		t.Errorf("第 3 轮的 Turn 尾必须画出这一轮的计划「新方案」；实际文字=%q", newer)
	}
	// 半 2：每帧只画这一轮（官方"Turn 尾只显示该轮的计划"）。
	if contains(old, "新方案") {
		t.Errorf("第 1 轮的 Turn 尾画进了第 3 轮的计划；实际文字=%q", old)
	}
	if contains(newer, "旧方案") {
		t.Errorf("第 3 轮的 Turn 尾画进了第 1 轮的计划；实际文字=%q", newer)
	}
}

// TestCardsSkipTurnWithoutPlan 钉住"这一轮没有提交就一个像素都不画"：
// 即使别的轮有计划，也不能把手边最近的那份摊到这一轮尾上。
func TestCardsSkipTurnWithoutPlan(t *testing.T) {
	frames := append(perTurnFrames(t),
		turnEnd(t, 5, 4, "completed"), // 第 4 轮：没有任何 exit_plan_mode
	)
	p, env, _, canvas := fixture(t, frames, 360)
	if got := uikernel.TurnTails(env.Data.Frames); len(got) != 3 {
		t.Fatalf("第 4 轮也有一条 turn/end，应有 3 个座位，得到 %d：%+v", len(got), got)
	}

	env.Turn = &pluginapi.TurnProps{Turn: 4}
	drawCardsOnce(t, p, env, canvas, 520, 240)
	if texts := textValues(canvas); len(texts) != 0 {
		t.Errorf("第 4 轮没有提交计划，不该画任何东西；实际画了 %v", texts)
	}
}
