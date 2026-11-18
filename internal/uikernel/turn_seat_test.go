package uikernel

// turn_seat_test.go —— transcript 按轮座位（官方 `conversation.chat.turnTail`）的两层断言。
//
// # 为什么这一单必须有它
//
// 官方那个座位一轮一次（`turn-tail` 节点的 children：
// OFF:dsh-client-ui-chat/src/client/chat/register-node-renderers.ts:88-97），
// owner props 带这一轮的 turn（OFF:.../contract/slots.ts:60-64）。改造前本骨架
// 每帧只 dispatch 一次、不带轮次，于是占用方（dsplan / dsactioncards）只能画
// "最新一轮"——两轮都有计划时前一伦的卡永远不出现。
//
// 两条断言分别钉住两半：
//
//	① `TurnTails` 从 journal 帧里取出的轮次表与 transcript 投影同一张分派表；
//	② 骨架真的**按轮各派一次**，每次 `env.Turn` 是那一轮的 owner props，
//	   派发结束恢复 nil（否则后面的座位会看见上一格的轮次）。

import (
	"encoding/json"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/ui"
)

// turnFrame 造一帧 canonical journal 帧（`Snapshot.Frames` 的元素）。
func turnFrame(t *testing.T, seq int, eventType string, data any) apiclient.Frame {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("构造帧失败：%v", err)
	}
	return apiclient.Frame{Seq: seq, Type: eventType, Data: raw}
}

// turnEndFrame 造一条 `turn/end`；reasonKind 为空 = 帧里没有 reason（官方 default 分支）。
func turnEndFrame(t *testing.T, seq int, turn int64, reasonKind string) apiclient.Frame {
	t.Helper()
	data := map[string]any{"turn": turn}
	if reasonKind != "" {
		data["reason"] = map[string]any{"kind": reasonKind}
	}
	return turnFrame(t, seq, "turn/end", data)
}

// TestTurnTailsMatchesTranscriptTailRule ①：只有会变成 `turn-tail` 节点的
// `turn/end` 才有座位，且轮次/seq 与帧一致。
//
// 四种收尾里三种**没有** turn-tail 节点（官方 turn-error.ts / turn-max-tokens.ts），
// 所以它们不许出现在座位表里——多派一次就是"那一轮尾凭空多出一块"。
func TestTurnTailsMatchesTranscriptTailRule(t *testing.T) {
	frames := []apiclient.Frame{
		turnFrame(t, 1, "turn/start", map[string]any{"turn": 1}),
		turnFrame(t, 2, "user/message", map[string]any{"text": "第一轮"}),
		turnEndFrame(t, 3, 1, "completed"),
		turnFrame(t, 4, "turn/start", map[string]any{"turn": 2}),
		turnEndFrame(t, 5, 2, "error"),
		turnFrame(t, 6, "turn/start", map[string]any{"turn": 3}),
		turnEndFrame(t, 7, 3, "max-tokens"),
		turnFrame(t, 8, "turn/start", map[string]any{"turn": 4}),
		turnEndFrame(t, 9, 4, "interrupted"),
		turnFrame(t, 10, "turn/start", map[string]any{"turn": 5}),
		// reason 缺失：官方 default 分支 → turn-tail（有座位）。
		turnEndFrame(t, 11, 5, ""),
	}
	got := TurnTails(frames)
	want := []pluginapi.TurnProps{{Turn: 1, Seq: 3}, {Turn: 5, Seq: 11}}
	if len(got) != len(want) {
		t.Fatalf("座位数 = %d（%+v），want %d（%+v）", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 个座位 = %+v，want %+v", i, got[i], want[i])
		}
	}
}

// TestTurnTailsSkipsWithoutTurnOrDuplicate ①：没有轮次号的不派发、同一轮只派一次。
//
// 前者照官方 `TurnTailNodeView.tsx:43-46`（turn === undefined → return null）；
// 后者是日志异常的防线：同一轮画两遍同一张卡比少画更糟。
func TestTurnTailsSkipsWithoutTurnOrDuplicate(t *testing.T) {
	frames := []apiclient.Frame{
		// turn 缺失：官方那一轮尾自己不渲染。
		turnFrame(t, 1, "turn/end", map[string]any{"reason": map[string]any{"kind": "completed"}}),
		turnEndFrame(t, 2, 7, "completed"),
		// 同一轮的第二条 turn/end（异常日志）：不许变成两个座位。
		turnEndFrame(t, 3, 7, "completed"),
	}
	got := TurnTails(frames)
	if len(got) != 1 || got[0].Turn != 7 || got[0].Seq != 2 {
		t.Fatalf("座位 = %+v，want 只有一个 {7 2}", got)
	}
	if tails := TurnTails(nil); tails != nil {
		t.Fatalf("没有帧基线时座位表必须为空（调用方据此退回一次派发），得到 %+v", tails)
	}
}

// TestFirstScreenDispatchesTurnTailPerTurn ②：骨架按轮各派一次，且带着那一轮的
// owner props；派发结束后 `env.Turn` 恢复 nil。
//
// 断言直接读 dispatch 回调里看到的 `env.Turn`——那是插件在同一调用栈里读到的
// 同一个值，不是另算一遍。
func TestFirstScreenDispatchesTurnTailPerTurn(t *testing.T) {
	frames := []apiclient.Frame{
		turnFrame(t, 1, "turn/start", map[string]any{"turn": 1}),
		turnEndFrame(t, 2, 1, "completed"),
		turnFrame(t, 3, "turn/start", map[string]any{"turn": 3}),
		turnEndFrame(t, 4, 3, "completed"),
		turnFrame(t, 5, "turn/start", map[string]any{"turn": 4}),
		turnEndFrame(t, 6, 4, "error"), // 没有 turn-tail 节点：不许派座位
	}
	env, canvas := newSkeletonEnv(1182, 832, func(pluginapi.Slot) bool { return false })
	env.Data.Frames = frames

	var seen []pluginapi.TurnProps
	var nullSeen int
	var turnDuringToolView *pluginapi.TurnProps
	pageTheme := dscore.VivaTheme(dscore.Dark())
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		FirstScreen(f, env, officialSeatOpts(), func(slot pluginapi.Slot) {
			switch slot {
			case OffChatTurnTail:
				if env.Turn == nil {
					nullSeen++
					return
				}
				seen = append(seen, *env.Turn)
			case OffToolCallView:
				// 同一个 Children 闭包里紧挨着的下一个座位：它不该看见上一格的轮次。
				turnDuringToolView = env.Turn
			}
		})
	})

	if len(seen) != 2 {
		t.Fatalf("turnTail 按轮派发次数 = %d（%+v），want 2（第 1 轮、第 3 轮各一次）", len(seen), seen)
	}
	if seen[0] != (pluginapi.TurnProps{Turn: 1, Seq: 2}) || seen[1] != (pluginapi.TurnProps{Turn: 3, Seq: 4}) {
		t.Fatalf("派发的 owner props = %+v，want [{1 2} {3 4}]（按 journal 顺序）", seen)
	}
	if nullSeen != 0 {
		t.Fatalf("有轮次基线时出现了 %d 次 env.Turn == nil 的派发（占用方会退回会话级，双画）", nullSeen)
	}
	if turnDuringToolView != nil {
		t.Fatalf("turnTail 之后 env.Turn 没有恢复：tool.call.toolview 座位看见 %+v", *turnDuringToolView)
	}
}

// TestFirstScreenDispatchTurnTailWithoutBaseline ②：没有 `turn/end` 帧时退回
// 改造前的行为——派发一次、`env.Turn == nil`，占用方不至于整块不画。
func TestFirstScreenDispatchTurnTailWithoutBaseline(t *testing.T) {
	env, canvas := newSkeletonEnv(1182, 832, func(pluginapi.Slot) bool { return false })
	env.Data.Frames = []apiclient.Frame{
		turnFrame(t, 1, "user/message", map[string]any{"text": "还在跑"}),
	}
	count := 0
	nullSeen := 0
	pageTheme := dscore.VivaTheme(dscore.Dark())
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		FirstScreen(f, env, officialSeatOpts(), func(slot pluginapi.Slot) {
			if slot != OffChatTurnTail {
				return
			}
			count++
			if env.Turn == nil {
				nullSeen++
			}
		})
	})
	if count != 1 || nullSeen != 1 {
		t.Fatalf("没有轮次基线时应派发 1 次且 env.Turn == nil；实际 count=%d null=%d", count, nullSeen)
	}
}
