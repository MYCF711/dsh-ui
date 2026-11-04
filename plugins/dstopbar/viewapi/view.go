// Package viewapi 是 ds-topbar 与 ds-rightbar 之间的最小会话视图契约。
//
// 它故意只覆盖官方已经注册的 chat / trajectory 两个真实视图，不描述“主题鱼”
// 或任何没有 painter 的扩展页签。正式公共 Action/Snapshot 若由 FG-1/Lead 接入，
// 可用同一个 service key 适配，而不需要让两个 UI 插件互相 import 实现。
package viewapi

const (
	// ServiceKey 是会话视图 store 的插件能力键。
	ServiceKey = "ui.conversation-view"
	// Chat 是官方 ui-chat 注册的固定主视图。
	Chat = "chat"
	// Trajectory 是官方 ui-trajectory 注册的工具轨迹主视图。
	Trajectory = "trajectory"
)

// Store 保存每个会话独立的主区视图选择。空 session id 不持久化。
type Store interface {
	Active(sessionID string) string
	Select(sessionID, view string)
}

// Known 报告视图是否有真实实现；未知扩展不进入当前两个页签。
func Known(view string) bool { return view == Chat || view == Trajectory }
