package dsconversation

// official_transcript_turn_tail_seat_test.go —— 按轮座位（官方
// `conversation.chat.turnTail`）的**位置**断言。
//
// # 这一单修的是位置（内容早已逐轮正确）
//
// 官方那个座位是 `turn-tail` 节点的 children
// （OFF:dsh-client-ui-chat/src/client/chat/register-node-renderers.ts:88-97），
// `TurnTailNodeView.tsx:43-51` 每画一个节点 renderSlot 一次；而尾节点本身按
// `anchorSeq = data.seq + finalizedFollowup` 排在 flow 里
// （OFF:.../conversation-nodes/turn-tail.ts:135-139，排序
// OFF:.../chat-snapshot-builder.ts:522-535），所以**每一轮的卡紧跟那一轮尾**。
//
// 改造前本仓在正文列尾按轮各派一次：卡的内容对、位置堆在列尾。现在的判据是
// 两条，缺一不可：
//
//	① 派发发生在**那条尾节点的行内**（派发时已经画完的行数 == 该尾节点的行号，
//	   而且它后面还有行）——「不在列尾」；
//	② 几何上第一轮的座位在**第一轮尾之下、第二轮提问之上**，第二轮的座位在
//	   第二轮尾之下——「各自的轮次之后」。
//
// 变异（把派发移回正文列尾：两条尾节点都不派、循环结束后统一派一次/两次）：
// ① 的行号判定与 ② 的几何同时红。

import (
	"strconv"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// turnTailSeatCall 是一次座位派发的现场记录。
type turnTailSeatCall struct {
	// Slot 是被派发的槽（必须是官方那个字面量）。
	Slot pluginapi.Slot
	// Owner 是派发期间 `env.Turn` 的 owner props（官方 `TurnTailOwnerProps`）。
	Owner pluginapi.TurnProps
	// RowsBefore 是派发时已经画完的行数（= 该 `turn-tail` 节点的行号）。
	RowsBefore int
}

// renderTurnTailSeatFrame 在真实画布上画一帧 transcript，并按节点派发座位。
//
// 座位占用方（这里用假的）画一条 `计划卡@<turn>` 文字：**几何断言要有真矩形**，
// 不能只看调用序列——"派发了但画在列尾"正是这一单要排除的那种错。
func renderTurnTailSeatFrame(t *testing.T, entries []TranscriptEntry) (*OfficialTranscriptView, *gfx.Canvas, *pluginapi.Env, *[]turnTailSeatCall) {
	t.Helper()
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	canvas, tree := newTranscriptCanvas()
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("turn-tail-seat-test"),
		Canvas: canvas, Bounds: gfx.Rect{Width: 900, Height: 1200}, MainWidth: 900,
	}
	calls := &[]turnTailSeatCall{}
	env.DrawSlot = func(f ui.C, slot pluginapi.Slot) {
		if slot != uikernel.OffChatTurnTail {
			return
		}
		if env.Turn == nil {
			t.Fatalf("座位 %q 派发时没有 owner props（env.Turn == nil）", slot)
			return
		}
		*calls = append(*calls, turnTailSeatCall{
			Slot:       slot,
			Owner:      *env.Turn,
			RowsBefore: len(view.Boxes.Rows),
		})
		ui.Text(f, "计划卡@"+strconv.FormatInt(env.Turn.Turn, 10)).FontSize(14).LineHeight(22)
	}
	pageTheme := dscore.VivaTheme(theme)
	fv := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	fv.Draw(canvas, env.Bounds, func(f ui.C) {
		view.Draw(f, env, hostViewport(tree), entries)
	})
	return view, canvas, env, calls
}

// 两条 `turn-tail` 节点各在自己那一轮里：第 1 轮（Seq 3）与第 3 轮（Seq 6）。
func twoTurnTailEntries() []TranscriptEntry {
	return []TranscriptEntry{
		{Key: "u1", Kind: ChatNodeUser, Seq: 1, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Text: "第一轮提问"}},
		{Key: "a1", Kind: ChatNodeAssistantStep, Seq: 2, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Text: "第一轮回答"}},
		{Key: "t1", Kind: ChatNodeTurnTail, Seq: 3, Turn: 1, Visibility: ChatVisibilityVisible},
		{Key: "u3", Kind: ChatNodeUser, Seq: 4, Turn: 3, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Text: "第二轮提问"}},
		{Key: "a3", Kind: ChatNodeAssistantStep, Seq: 5, Turn: 3, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Text: "第二轮回答"}},
		{Key: "t3", Kind: ChatNodeTurnTail, Seq: 6, Turn: 3, Visibility: ChatVisibilityVisible},
	}
}

// findTextRect 找一条文字的矩形（找不到直接失败：几何断言必须有真矩形）。
func findTextRect(t *testing.T, canvas *gfx.Canvas, value string) gfx.Rect {
	t.Helper()
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text.Bounds
		}
	}
	t.Fatalf("画布上找不到文字 %q（画出来的有 %v）", value, textValuesOf(canvas))
	return gfx.Rect{}
}

// textValuesOf 收画布文字（失败信息用）。
func textValuesOf(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

// TestTurnTailSeatDispatchedInsideEachTurnNode ①：座位在**每条尾节点的行内**派发，
// owner props 就是那一轮的 turn/seq，派发结束 `env.Turn` 恢复 nil。
func TestTurnTailSeatDispatchedInsideEachTurnNode(t *testing.T) {
	view, _, env, calls := renderTurnTailSeatFrame(t, twoTurnTailEntries())

	if len(*calls) != 2 {
		t.Fatalf("座位派发次数 = %d（%+v），want 2（每条 turn-tail 节点一次）", len(*calls), *calls)
	}
	if (*calls)[0].Slot != uikernel.OffChatTurnTail {
		t.Fatalf("派发的槽 = %q，want 官方字面量 %q", (*calls)[0].Slot, uikernel.OffChatTurnTail)
	}
	// owner props：官方 `TurnTailOwnerProps{turn, seq}`（contract/slots.ts:60-64）。
	if got, want := (*calls)[0].Owner, (pluginapi.TurnProps{Turn: 1, Seq: 3}); got != want {
		t.Fatalf("第 1 次派发的 owner props = %+v，want %+v", got, want)
	}
	if got, want := (*calls)[1].Owner, (pluginapi.TurnProps{Turn: 3, Seq: 6}); got != want {
		t.Fatalf("第 2 次派发的 owner props = %+v，want %+v", got, want)
	}
	// ① 行内：第 1 轮尾是第 3 行（下标 2）、第 3 轮尾是第 6 行（下标 5）。
	// 派发时"已经画完的行数"就是它的行号——**不是**总行数（那是列尾）。
	if got := (*calls)[0].RowsBefore; got != 2 {
		t.Fatalf("第 1 轮座位派发时已画 %d 行，want 2（= 它自己那条 turn-tail 的行号，列尾会是 6）", got)
	}
	if got := (*calls)[1].RowsBefore; got != 5 {
		t.Fatalf("第 3 轮座位派发时已画 %d 行，want 5（= 它自己那条 turn-tail 的行号，列尾会是 6）", got)
	}
	// 关键一条：第 1 轮那次派发**后面还有行**（第 2 轮的三条），所以它绝不是
	// "整列画完后统一派一次"（那种实现下两次派发都会落在总行数 6 上）。
	// 最后一条尾节点当然在行尾——那是"最后一轮"的尾，不是列尾派发。
	if got, total := (*calls)[0].RowsBefore, len(view.Boxes.Rows); got >= total-1 {
		t.Fatalf("第 1 轮的座位派发后面没有行了（已画 %d 行 / 共 %d 行）：位置退回列尾", got, total)
	}
	// 写者负责恢复：派发结束后 env.Turn 必须是 nil。
	if env.Turn != nil {
		t.Fatalf("座位派发结束 env.Turn 没恢复原值：%+v", *env.Turn)
	}
	if env.TurnTailSeatsDrawn != 2 {
		t.Fatalf("Env.TurnTailSeatsDrawn = %d，want 2（壳据此不在列尾补派）", env.TurnTailSeatsDrawn)
	}
}

// TestTurnTailSeatSitsBetweenItsOwnTurnAndTheNext ②：**几何**证明两轮的卡分别落在
// 各自的轮次之后——第一轮的卡在第一轮尾之下、第二轮提问之上；第二轮的卡在
// 第二轮回答之下。两条同时成立，才排除"都堆在列尾"。
func TestTurnTailSeatSitsBetweenItsOwnTurnAndTheNext(t *testing.T) {
	_, canvas, _, calls := renderTurnTailSeatFrame(t, twoTurnTailEntries())
	if len(*calls) != 2 {
		t.Fatalf("座位派发次数 = %d，want 2", len(*calls))
	}

	card1 := findTextRect(t, canvas, "计划卡@1")
	card3 := findTextRect(t, canvas, "计划卡@3")
	q1 := findTextRect(t, canvas, "第一轮提问")
	a1 := findTextRect(t, canvas, "第一轮回答")
	q3 := findTextRect(t, canvas, "第二轮提问")
	a3 := findTextRect(t, canvas, "第二轮回答")

	// 第一轮的卡：在本轮回答**下面**，在下一轮提问**上面**。
	if !(card1.Y >= a1.Y) {
		t.Fatalf("第 1 轮的座位卡不在本轮内容之后：卡 y=%v，回答 y=%v", card1.Y, a1.Y)
	}
	if !(card1.Y < q3.Y) {
		t.Fatalf("第 1 轮的座位卡被画到了第 2 轮之后（列尾）：卡 y=%v，第 2 轮提问 y=%v", card1.Y, q3.Y)
	}
	// 第二轮的卡：在本轮回答下面。
	if !(card3.Y >= a3.Y) {
		t.Fatalf("第 2 轮的座位卡不在本轮内容之后：卡 y=%v，回答 y=%v", card3.Y, a3.Y)
	}
	// 两轮各自的卡先后有序，且都夹在各自的轮次里（同一列、非重叠）。
	if !(card1.Y < card3.Y) {
		t.Fatalf("两轮座位卡顺序错乱：第 1 轮 y=%v，第 2 轮 y=%v", card1.Y, card3.Y)
	}
	if card1.Y+card1.Height > q3.Y {
		t.Fatalf("第 1 轮的卡越过了第 2 轮的起点：卡底 %v > 第 2 轮提问 %v", card1.Y+card1.Height, q3.Y)
	}
	// 上面几条一起等价于：卡 1 在 [第一轮回答, 第二轮提问) 区间内、卡 3 在其轮次内。
	if !(q1.Y < card1.Y) {
		t.Fatalf("第 1 轮的卡跑到本轮提问之前：卡 y=%v，提问 y=%v", card1.Y, q1.Y)
	}
}

// TestTurnTailSeatSkippedWithoutTurnOrChannel 边界：没有轮次号（官方
// `TurnTailNodeView.tsx:43-46` 直接 return null）或没有派发通道时不派发。
func TestTurnTailSeatSkippedWithoutTurnOrChannel(t *testing.T) {
	entries := []TranscriptEntry{
		{Key: "t-unknown", Kind: ChatNodeTurnTail, Seq: 1, Turn: -1, Visibility: ChatVisibilityVisible},
	}
	_, _, env, calls := renderTurnTailSeatFrame(t, entries)
	if len(*calls) != 0 {
		t.Fatalf("turn 未知的尾节点不该派发座位，实际 %+v", *calls)
	}
	if env.TurnTailSeatsDrawn != 0 {
		t.Fatalf("没派发却记了 %d 次（壳会误判'已经画过'）", env.TurnTailSeatsDrawn)
	}

	// 没有 `env.DrawSlot`（独立装配 / 单测环境）：不派发、不记数、不 panic。
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	canvas, tree := newTranscriptCanvas()
	noChannel := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("turn-tail-seat-nochannel"),
		Canvas: canvas, Bounds: gfx.Rect{Width: 900, Height: 600}, MainWidth: 900,
	}
	pageTheme := dscore.VivaTheme(theme)
	fv := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	fv.Draw(canvas, noChannel.Bounds, func(f ui.C) {
		view.Draw(f, noChannel, hostViewport(tree), twoTurnTailEntries())
	})
	if noChannel.TurnTailSeatsDrawn != 0 || noChannel.Turn != nil {
		t.Fatalf("没有派发通道时不该写计数/owner props：drawn=%d turn=%+v",
			noChannel.TurnTailSeatsDrawn, noChannel.Turn)
	}
}
