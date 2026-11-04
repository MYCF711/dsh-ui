package dsconversation

import (
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dstool"
)

// toolStatus 把消息流里的终局/活动 Turn 真值与工具自身结果合并。
// 优先级防止两种误报：已完成/失败的调用不因所属 Turn 后来停止而改成 stopped；
// 没有结果的调用也不因“数组里有 Busy”就被猜成 running，必须精确匹配活动 Turn。
func toolStatus(msg apiclient.Message, key string, data pluginapi.Snapshot) dstool.Status {
	status := dstool.ResultStatus(msg.Tool, msg.Summary, msg.View)
	if data.MessageMeta[key].Stopped {
		// MessageMetadata.Stopped 只来自 load 原始 stopped=true，直接附着于这条消息。
		return dstool.StatusStopped
	}
	turn := loadedTurn{id: msg.TurnID, number: msg.Turn}
	if status != dstool.StatusOK && status != dstool.StatusError {
		for _, event := range data.TurnEvents {
			// TurnEvent.Kind=stopped 已由壳统一收纳显式 stopped/interrupted；
			// turn/error 不在这里冒充某一次工具调用的 error。
			if event.Kind == "stopped" && sameLoadedTurn(turn, event.TurnID, event.Turn) {
				return dstool.StatusStopped
			}
		}
	}
	if status == dstool.StatusNeutral && data.Busy && sameLoadedTurn(turn, data.ActiveTurnID, data.ActiveTurn) {
		return dstool.StatusRunning
	}
	return status
}
