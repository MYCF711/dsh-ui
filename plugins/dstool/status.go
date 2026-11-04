package dstool

import (
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// Status 是一张工具卡可以证明的生命周期状态。空串是 neutral：不是“未知成功”，
// 也不是“尚未开始”，只表示当前快照没有足够真值，标题行不画状态徽标。
type Status string

const (
	StatusNeutral   Status = ""
	StatusPreparing Status = "preparing"
	StatusRunning   Status = "running"
	StatusOK        Status = "ok"
	StatusError     Status = "error"
	StatusStopped   Status = "stopped"
)

// ResultStatus 只从工具消息自身携带的显式结果与 View.Badge 推导状态。
// running 的活动 Turn 判断、stopped/interrupted 的原始终局都属于消息流快照，
// 由 ds-conversation 补充；这里绝不因“有 Args”就编造 preparing/running。
func ResultStatus(tool, summary string, view *apiclient.ViewSpec) Status {
	if view != nil {
		switch view.Badge {
		case "preparing":
			return StatusPreparing
		case "running":
			return StatusRunning
		case "ok":
			return StatusOK
		case "error":
			return StatusError
		case "stopped":
			return StatusStopped
		}
	}
	if summary == "" && view == nil {
		return StatusNeutral
	}
	if tool == "bash" {
		if code, _ := splitExit(summary); code != "" {
			if badExit(code) {
				return StatusError
			}
			return StatusOK
		}
	}
	// Summary 或 View 的存在本身就是显式结果；没有 error/非零退出码真值时，
	// 结果状态是 ok。View.Badge 的其它值（例如耗时）仍由 View 原样展示。
	return StatusOK
}

func statusLabel(status Status) string {
	switch status {
	case StatusPreparing:
		return "准备中"
	case StatusRunning:
		return "运行中"
	case StatusOK:
		return "完成"
	case StatusError:
		return "失败"
	case StatusStopped:
		return "已停止"
	default:
		return ""
	}
}

func statusColor(theme *pluginapi.Theme, status Status) ui.Color {
	switch status {
	case StatusOK:
		return theme.Success
	case StatusError:
		return theme.Error
	case StatusStopped:
		return theme.LabelCaption
	default:
		return theme.LabelTertiary
	}
}
