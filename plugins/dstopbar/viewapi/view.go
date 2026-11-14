// Package viewapi 是 ds-topbar 与 ds-rightbar 之间的最小会话视图契约。
//
// 它只描述「哪个会话选了哪个 view」这一条事实，不描述名单：**名录归壳**
// （`uikernel.ViewTabs` 从内核的 `conversation.view` 占用方投影，
// 壳通过 `pluginapi.Env.Views` / `ActiveView` 交给界面）。
//
// 为什么把名单从本包拿掉（2026-11-15）：这里原先写死 `Known = chat|trajectory`，
// 于是「插件注册了一个新 view」在界面上永远不出现——闸门在错误的地方。
// 现在唯一闸门是注册表本身：注册了就进名录，没注册就进不去；
// 壳写选择项前也先查名录（`cmd/app/view_tabs_wiring.go`）。
package viewapi

import "github.com/MYCF711/dsh-ui/pluginapi"

const (
	// ServiceKey 是会话视图 store 的插件能力键。
	ServiceKey = "ui.conversation-view"
	// Chat 是官方 ui-chat 注册的固定主视图（= pluginapi.ViewChat）。
	Chat = pluginapi.ViewChat
	// Trajectory 是官方 ui-trajectory 注册的工具轨迹主视图（= pluginapi.ViewTrajectory）。
	Trajectory = pluginapi.ViewTrajectory
)

// Store 保存每个会话独立的主区视图选择。空 session id 不持久化。
type Store interface {
	Active(sessionID string) string
	Select(sessionID, view string)
}

// Known 报告 view 是不是官方已注册的那几个之一。
//
// **它不再是闸门**：闸门是注册表（见包注释）。保留它只为一处判断——
// ds-rightbar 的轨迹视图要问"当前选中的是不是 trajectory"，那是按 id 比大小，
// 不是准入。未知扩展照样可以注册、照样可以被选中。
func Known(view string) bool {
	return view == Chat || view == Trajectory || view == pluginapi.ViewStats
}
