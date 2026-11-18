package pluginapi

// turn_tail.go —— transcript 的**按轮座位**（官方 `conversation.chat.turnTail`，rc.2）。
//
// # 官方语义：这个座位一轮一次，不是会话一次
//
//	OFF:dsh-client-ui-chat/src/client/chat/register-node-renderers.ts:88-97
//	  `key: 'turn-tail'` 节点的 children 里声明 `conversation.chat.turnTail`
//	  （`{ kind: 'list'; scope: 'session' }`）
//	OFF:dsh-client-ui-chat/src/client/chat/TurnTailNodeView.tsx:43-49
//	  **每画一个 turn-tail 节点**就 `renderSlot('conversation.chat.turnTail', owner)` 一次
//	OFF:dsh-client-ui-chat/src/client/contract/slots.ts:60-64
//	  owner props = `TurnTailOwnerProps { turn: TurnLocation; seq: number; openFile }`
//
// 所以「几轮就有几个座位」，owner props 里那一轮的 `turn.turn` 就是占用方该画的
// 那一轮。两个已知消费方都**只认这一轮**：
//
//	OFF:dsh-client-ui-plan/src/client/PlanCard.tsx:43-45
//	  `const plans = usePlans(String(turn.turn))`（数据源同样按轮取：
//	  OFF:.../ui-plan/src/client/index.ts:88-92 的 `turnDataSource(Number(turn), 'submitted-plan')`）
//	OFF:dsh-client-ui-action-cards/src/client/Card.tsx:69-76
//	  `proposalOf(view, turn.turn)`（同轮多张取最小 callId）
//
// # 本仓的形状：为什么在 Env 上，而不是改成每帧入参
//
// 本仓的插件拿不到"owner props"这种每帧入参：`Painter.Draw` 只有 `*Env`。
// 所以壳在派发这个座位之前把 owner props 挂到 `Env.Turn` 上，**派发完必须恢复
// 原值**——与 `Env.EntryMenu` / `Env.GuideEntry` 同一约定：写者负责恢复，读方不缓存。
//
// 壳按轮各派一次（谁写、写在哪见 `internal/uikernel/shell_skeleton.go` 的
// `dispatchTurnTails`）。`Env.Turn == nil` 表示**没有轮次基线**（老壳 / mockd /
// 这一帧还没有 `turn/end` 帧）：占用方这时只能退回会话级的近似选择，
// 不许把 nil 当成"第 0 轮"。
type TurnProps struct {
	// Turn 是这一轮的轮次号（官方 `TurnTailOwnerProps.turn.turn`）。
	//
	// 来自 `turn/end` 帧的 `data.turn`（`Snapshot.Frames` 是 journal 原文）。
	// 拿不到轮次的尾**不产生座位**：官方 TurnTailNodeView.tsx:46 在
	// `turn === undefined` 时同样直接 `return null`（不画、不留空壳）。
	Turn int64
	// Seq 是这一轮尾的序号，官方 `TurnTailOwnerProps.seq`
	// （取 `closing?.finalNode.seq ?? data.seq`）。本仓取产生这一轮尾的
	// `turn/end` 帧的 seq，与 transcript 投影里那条 `turn-tail` 节点的 seq 同一个值。
	Seq int
}
