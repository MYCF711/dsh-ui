package uikernel

// turns.go —— 从 journal 原始帧里取出**每一个按轮座位**（官方
// `conversation.chat.turnTail`）的 owner props。
//
// # 为什么需要它
//
// 那个座位在官方是**一轮一次**的（`turn-tail` 节点的 children：
// OFF:dsh-client-ui-chat/src/client/chat/register-node-renderers.ts:88-97，
// `TurnTailNodeView.tsx:43-49` 每画一个 turn-tail 节点 renderSlot 一次）。
// 本骨架没有 `conversation.chat.node` 工厂节点，座位只能在正文列尾部由骨架
// 自己派发（`drawConversationSkeleton` 的 `dispatchTurnTails`），所以骨架必须
// **自己知道这一帧有几轮**——这就是本文件。
//
// # 判据与 transcript 投影必须一致（否则出现"画了尾却没座位"的假缺口）
//
// 官方每一轮以一条 `turn/end` 收尾，投影把它的 `reason.kind` 分派成四种节点：
//
//	error       → turn-error        （没有 turn-tail 节点 ⇒ 没有座位）
//	max-tokens  → turn-max-tokens   （同上）
//	interrupted → turn-error        （同上）
//	其余        → turn-tail        （有节点 ⇒ 有一个座位）
//
// 本仓同一张分派表在 `plugins/dsconversation/official_transcript_projection.go`
// 的 `case "turn/end"`（:805-837）。**这里跟着它走**：改那边必须改这里，
// 两处不一致的表现是"transcript 少了/多了尾卡"或"座位派发了却没有那一轮"。
//
// # 轮次号缺失的不派发
//
// 官方 `TurnTailNodeView.tsx:43-46`：`turn === undefined` 直接 `return null`
// ——那一轮尾连 DOM 都不产生。所以这里同样跳过 `data.turn` 缺失的 `turn/end`
// （老日志 / 畸形帧），不编一个"第 0 轮"出来。

import (
	"encoding/json"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// turnEndData 是 `turn/end` 帧里本函数要的两个字段。
//
// 只解这两个键：其余载荷（usage / durationMs）属于 transcript 渲染，
// 这里多解一个字段就多一处会漂移的形状。
type turnEndData struct {
	Turn   *int64 `json:"turn"`
	Reason *struct {
		Kind string `json:"kind"`
	} `json:"reason"`
}

// TurnTails 返回这一帧该派发的按轮座位（按 journal 顺序 = 轮次顺序）。
//
// 每一轮一条 `TurnProps{Turn, Seq}`；没有 `turn/end` 帧（首帧 / 老宿主 / mockd）
// 时返回 nil，调用方据此退回"会话级一次"的旧行为。
func TurnTails(frames []apiclient.Frame) []pluginapi.TurnProps {
	var out []pluginapi.TurnProps
	seen := map[int64]bool{}
	for _, frame := range frames {
		if frame.Type != "turn/end" {
			continue
		}
		var data turnEndData
		if len(frame.Data) > 0 {
			if err := json.Unmarshal(frame.Data, &data); err != nil {
				// 解不开 = 不认识的载荷：投影那边落 unknown 兜底（不是 turn-tail），
				// 所以这里也不产生座位。
				continue
			}
		}
		if data.Reason != nil {
			switch data.Reason.Kind {
			case "error", "max-tokens", "interrupted":
				// 这三种收尾没有 turn-tail 节点（见文件头那张分派表）。
				continue
			}
		}
		if data.Turn == nil || *data.Turn < 0 {
			// 拿不到轮次号：官方那一轮尾自己不渲染（TurnTailNodeView.tsx:46）。
			continue
		}
		// 同一个轮次只派一次：官方按轮建节点，重复的 `turn/end` 是日志异常，
		// 画两遍同一张卡比少画更糟。
		if seen[*data.Turn] {
			continue
		}
		seen[*data.Turn] = true
		out = append(out, pluginapi.TurnProps{Turn: *data.Turn, Seq: frame.Seq})
	}
	return out
}
