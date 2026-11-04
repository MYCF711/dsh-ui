package apiclient

import "strings"

// reasoning.go —— "只有思考、没有正文"那条回答怎么落成消息。
//
// # 为什么必须有这条规则
//
// v4 里它就是一条 `assistant/message`：`content` 是空的、`reasoning` 非空。
// 宿主在 `session.load` 里按同一形状给（`transport_source.go`：有正文时**不**带
// reasoning，因为"有正文时思考是草稿，不是已经给用户的回答"）。
//
// 界面的**实时路径**把它落成 `role=thinking`（`Stream.Messages()`）。
// 如果重载路径不按同一条规则翻，同一个回答就会有两种样子：
//
//	正在跑的时候：看到一个思考块
//	切回来之后：  那条消息正文是空的 —— 空气泡，或者干脆不见了
//
// 这正是宿主那边刚修过的"静默失败"（`session.load` 丢思考）。

// ReasoningOnlyThinking 把"只有思考、没有正文"的 assistant 消息翻成一条 thinking 消息。
//
// 返回 (翻好的消息, true)；不是这种情况返回 (零值, false)，调用方原样用原消息。
//
// 刻意**只认这一种形状**：正文非空的 assistant 消息即使带了 reasoning 也不翻——
// 宿主根本不会那样发，而多认一种形状就等于自己发明一条渲染规则。
func ReasoningOnlyThinking(msg Message) (Message, bool) {
	if msg.Role != "assistant" {
		return Message{}, false
	}
	if strings.TrimSpace(msg.Text) != "" {
		return Message{}, false
	}
	if strings.TrimSpace(msg.Reasoning) == "" {
		return Message{}, false
	}
	out := msg
	out.Role = "thinking"
	out.Text = msg.Reasoning // thinking 那条路的正文就是思考本身
	out.Reasoning = ""
	return out, true
}
