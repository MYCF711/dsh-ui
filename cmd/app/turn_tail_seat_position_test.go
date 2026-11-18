//go:build darwin || windows || linux

package main

// turn_tail_seat_position_test.go —— 端到端证明「两轮的计划卡分别出现在各自的
// 轮次之后」，而不是都堆在正文列尾（账本 #28 最后一条差异）。
//
// 走的是产品真实链路：
//
//	uikernel.FirstScreen  →  conversation.view  →  dsconversation 的 transcript
//	                        →  每条 turn-tail 行里派发 conversation.chat.turnTail
//	                        →  dsplan 画那一轮的计划卡
//
// 官方位置语义：座位是 `turn-tail` 节点的 children
// （OFF:dsh-client-ui-chat/src/client/chat/register-node-renderers.ts:88-97），
// 节点按 anchorSeq 排在 flow 里（OFF:.../conversation-nodes/turn-tail.ts:135-139，
// 排序 OFF:.../chat-snapshot-builder.ts:522-535）——每一轮尾后面就是这一轮的卡。
//
// 断言用**几何**：第 1 轮的卡要在第二轮提问之上。列尾派发时它一定在第二轮
// 内容**之下**，那一条就是这一单的红线。

import (
	"encoding/json"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsplan"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// turnSeatFrame 造一条 canonical journal 帧。
func turnSeatFrame(t *testing.T, seq int, eventType string, data any) apiclient.Frame {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("构造帧失败：%v", err)
	}
	return apiclient.Frame{Seq: seq, Type: eventType, Data: raw}
}

// turnSeatPlanFrames 造「两轮各提交一份计划」的会话（第 1 轮、第 3 轮）。
func turnSeatPlanFrames(t *testing.T) []apiclient.Frame {
	t.Helper()
	planArgs := func(markdown string) string {
		raw, err := json.Marshal(map[string]string{"plan": markdown})
		if err != nil {
			t.Fatalf("构造计划参数失败：%v", err)
		}
		return string(raw)
	}
	return []apiclient.Frame{
		turnSeatFrame(t, 1, "turn/start", map[string]any{"turn": 1}),
		turnSeatFrame(t, 2, "user/message", map[string]any{
			"surfaceOp": "append", "source": map[string]any{"kind": "user"},
			"content": []map[string]any{{"type": "text", "text": "第一轮提问"}},
		}),
		turnSeatFrame(t, 3, "assistant/message", map[string]any{
			"turn": 1, "step": 1,
			"content": []map[string]any{{"type": "text", "text": "第一轮回答"}},
		}),
		turnSeatFrame(t, 4, "tool/call", map[string]any{
			"turn": 1, "step": 1, "callId": "call-old", "name": "exit_plan_mode",
			"arguments": planArgs("# 旧方案\n\n旧的正文。"),
		}),
		turnSeatFrame(t, 5, "turn/end", map[string]any{
			"turn": 1, "reason": map[string]any{"kind": "completed"},
		}),
		turnSeatFrame(t, 6, "turn/start", map[string]any{"turn": 3}),
		turnSeatFrame(t, 7, "user/message", map[string]any{
			"surfaceOp": "append", "source": map[string]any{"kind": "user"},
			"content": []map[string]any{{"type": "text", "text": "第二轮提问"}},
		}),
		turnSeatFrame(t, 8, "assistant/message", map[string]any{
			"turn": 3, "step": 1,
			"content": []map[string]any{{"type": "text", "text": "第二轮回答"}},
		}),
		turnSeatFrame(t, 9, "tool/call", map[string]any{
			"turn": 3, "step": 1, "callId": "call-new", "name": "exit_plan_mode",
			"arguments": planArgs("# 新方案\n\n新的正文。"),
		}),
		turnSeatFrame(t, 10, "turn/end", map[string]any{
			"turn": 3, "reason": map[string]any{"kind": "completed"},
		}),
	}
}

// drawTurnSeatProductFrame 用真实内核 + 官方骨架画一帧产品路径。
func drawTurnSeatProductFrame(t *testing.T) *gfx.Canvas {
	t.Helper()
	kernel := uikernel.New("turn-tail-seat-position")
	if err := kernel.Load(
		&dscore.Plugin{Dark: true},
		&dsconversation.Plugin{},
		&dsplan.Plugin{},
	); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("turn-tail-seat-position"),
		Canvas: canvas,
		Bounds: gfx.Rect{Width: 1182, Height: 832}, MainWidth: 902,
		Geometry: pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902},
		Data:     pluginapi.Snapshot{Cur: "session-1", Frames: turnSeatPlanFrames(t)},
		Has:      func(slot pluginapi.Slot) bool { return kernel.Has(slot) },
	}
	m := &model{kernel: kernel, cur: "session-1"}
	opts := uikernel.ShellOptions{
		Tabs: true, SidebarPreferred: 280, RightbarOn: true,
		Views:      []pluginapi.Tab{{Key: pluginapi.ViewChat, Label: "对话"}},
		ActiveView: pluginapi.ViewChat,
	}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		env.DrawSlot = func(target ui.C, slot pluginapi.Slot) {
			m.kernel.DrawSlot(target, slot, env)
		}
		env.DrawKeyed = func(target ui.C, slot pluginapi.Slot, key string) {
			m.kernel.DrawSlotKeyed(target, slot, key, env)
		}
		uikernel.FirstScreen(f, env, opts, func(slot pluginapi.Slot) {
			drawSlot(f, m.kernel, slot, env)
		})
	})
	return canvas
}

// turnSeatTextRect 找一条文字（含前缀匹配：卡片标题会被 Ellipsis 截断）。
func turnSeatTextRect(canvas *gfx.Canvas, value string) (gfx.Rect, bool) {
	for _, text := range canvas.Texts() {
		if text.Value == value || (len(text.Value) >= len(value) && text.Value[:len(value)] == value) {
			return text.Bounds, true
		}
	}
	return gfx.Rect{}, false
}

// TestPlanCardsSitAfterTheirOwnTurn 是 #28 位置差异的端到端断言。
//
// 三条判据（少一条就不能叫"在各自轮次之后"）：
//
//	① 两轮的卡都画出来了（内容层早已正确，这里只是前提）；
//	② 第 1 轮的卡在**第二轮提问之上** —— 列尾派发时它一定在下面（红）；
//	③ 第 2 轮的卡在**第二轮内容之下** —— 属于它自己那一轮。
func TestPlanCardsSitAfterTheirOwnTurn(t *testing.T) {
	canvas := drawTurnSeatProductFrame(t)

	oldCard, okOld := turnSeatTextRect(canvas, "旧方案")
	newCard, okNew := turnSeatTextRect(canvas, "新方案")
	if !okOld || !okNew {
		t.Fatalf("两轮的计划卡没同时画出来：旧方案=%v 新方案=%v（画布文字=%v）",
			okOld, okNew, turnSeatCanvasTexts(canvas))
	}
	q1, okQ1 := turnSeatTextRect(canvas, "第一轮提问")
	a1, okA1 := turnSeatTextRect(canvas, "第一轮回答")
	q3, okQ3 := turnSeatTextRect(canvas, "第二轮提问")
	a3, okA3 := turnSeatTextRect(canvas, "第二轮回答")
	if !okQ1 || !okA1 || !okQ3 || !okA3 {
		t.Fatalf("transcript 行没画全：q1=%v a1=%v q3=%v a3=%v（画布文字=%v）",
			okQ1, okA1, okQ3, okA3, turnSeatCanvasTexts(canvas))
	}

	// ② 第 1 轮的卡必须在第二轮提问**上面**（列尾派发 ⇒ 在下面 ⇒ 这条红）。
	if oldCard.Y >= q3.Y {
		t.Fatalf("第 1 轮的计划卡被画到了第 2 轮之后（列尾）：卡 y=%v，第 2 轮提问 y=%v（a1 y=%v）",
			oldCard.Y, q3.Y, a1.Y)
	}
	// 并且在本轮内容之下（不是跑到上一轮头上）。
	if oldCard.Y < q1.Y {
		t.Fatalf("第 1 轮的计划卡跑到了本轮提问之前：卡 y=%v，提问 y=%v", oldCard.Y, q1.Y)
	}
	// ③ 第 2 轮的卡在第二轮回答之下。
	if newCard.Y < a3.Y {
		t.Fatalf("第 2 轮的计划卡不在本轮内容之后：卡 y=%v，回答 y=%v", newCard.Y, a3.Y)
	}
	// 顺序：第 1 轮的卡在前、第 2 轮的卡在后；且第 1 轮的卡底不越过第 2 轮起点。
	if !(oldCard.Y < newCard.Y) {
		t.Fatalf("两轮计划卡顺序错乱：旧 y=%v，新 y=%v", oldCard.Y, newCard.Y)
	}
	if oldCard.Y+oldCard.Height > q3.Y {
		t.Fatalf("第 1 轮的计划卡越过了第 2 轮起点：卡底 %v > 第 2 轮提问 %v",
			oldCard.Y+oldCard.Height, q3.Y)
	}
}

// turnSeatCanvasTexts 收画布文字（失败信息用）。
func turnSeatCanvasTexts(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}
