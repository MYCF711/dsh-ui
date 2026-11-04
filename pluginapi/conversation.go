package pluginapi

// MessageMetadata 是 apiclient 当前尚未暴露、但 session.load 原始消息已经携带的
// 真值。Time 为 Transport 的 Unix 毫秒时间戳；0 = 未知。Stopped 只来自原始
// stopped=true，不由取消按钮或本地数组下标推断。
type MessageMetadata struct {
	Time    int64
	Stopped bool
}

// TurnEvent 是已经到达的 turn/error 或显式 stopped/interrupted 终局。
// Key 是会话内局部视图键；TurnID/Time 缺真值时保持零值并由界面隐藏。
type TurnEvent struct {
	Key    string
	Kind   string // done / error / stopped
	TurnID string
	Turn   *int64
	Error  string
	Time   int64
}

const (
	OutgoingSending = "sending"
	OutgoingSent    = "sent"
	OutgoingFailed  = "failed"
)

// OutgoingMessage 是尚未与 durable user event 对账的本地发送。它独立于
// apiclient.Message，因此没有 Seq:-1，也不会借 Transport journal seq 编号。
// 失败时 Text 就是可重试草稿；TurnID 仅在 prompt 真返回后填写。
type OutgoingMessage struct {
	Key    string
	Text   string
	State  string
	TurnID string
	Error  string
}
