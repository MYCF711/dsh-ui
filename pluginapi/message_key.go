package pluginapi

import (
	"strconv"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// MessageKey 返回消息区本地视图状态的稳定键。Transport journal seq 不在输入中；
// 身份优先级与 MSGID 一致，旧 load 的 Seq 单独标为 v4-load，未知身份只退到
// session+index 的会话内局部键。key 不跨进程持久化，也不冒充宿主 messageId。
func MessageKey(session string, msg apiclient.Message, index int) string {
	identity := ""
	switch {
	case msg.V4Seq != nil:
		identity = "v4:" + strconv.FormatInt(*msg.V4Seq, 10)
	case msg.MessageID != "":
		identity = "message:" + msg.MessageID
	case msg.TurnID != "":
		identity = "turn-id:" + strconv.Quote(msg.TurnID) + ":step:" + messageStepKey(msg.Step)
		if msg.Step == nil && msg.CallID == "" {
			// 同一 Turn 可有多条同角色消息；Step 缺失时只能明确降级为
			// 当前窗口局部键，不能让它们共享时间/展开/复制状态。
			identity += ":local:" + strconv.Itoa(index)
		}
	case msg.Turn != nil:
		identity = "turn:" + strconv.FormatInt(*msg.Turn, 10) + ":step:" + messageStepKey(msg.Step)
		if msg.Step == nil && msg.CallID == "" {
			identity += ":local:" + strconv.Itoa(index)
		}
	case msg.Seq != 0:
		// 旧 session.load 投影的 Seq 本来就是 v4 域；本地 echo 不再写这个字段。
		identity = "v4-load:" + strconv.Itoa(msg.Seq)
	default:
		identity = "local:" + strconv.Itoa(index)
	}
	if msg.CallID != "" {
		identity += ":call:" + strconv.Quote(msg.CallID)
	}
	return session + "\x00" + identity + "\x00" + msg.Role
}

func messageStepKey(step *int64) string {
	if step == nil {
		return "unknown"
	}
	return strconv.FormatInt(*step, 10)
}
