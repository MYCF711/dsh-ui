// Package pluginapi（内部路径）是公开契约包 github.com/MYCF711/dsh-ui/pluginapi 的别名层。
//
// 为什么要有这一层：第三方插件必须能 import 一个**不是 internal/** 的路径，
// 但仓库里的内核与内置插件已经在 import 这个 internal 路径了。两份定义会漂移，
// 所以这里一个类型都不定义，全部 = 公开包。装配时它们是同一组类型，
// 第三方插件对象可以原样交给 Load。
package pluginapi

import pub "github.com/MYCF711/dsh-ui/pluginapi"

type (
	Source       = pub.Source
	SourceFunc   = pub.SourceFunc
	Plugin       = pub.Plugin
	Fields       = pub.Fields
	KeyHandler   = pub.KeyHandler
	Panels       = pub.Panels
	Context      = pub.Context
	Slot         = pub.Slot
	FSEntry      = pub.FSEntry
	ScheduleItem = pub.ScheduleItem
	Job          = pub.Job
	Model        = pub.Model
	Painter      = pub.Painter
	Primitives   = pub.Primitives
	Env          = pub.Env
	Geometry     = pub.Geometry
	Snapshot     = pub.Snapshot
	Usage        = pub.Usage
	Approval     = pub.Approval
	// ApprovalOption 是审批帧里 data.options 的一项（H13）。
	// 纯别名，不在这里加任何东西——两份定义会漂移。
	ApprovalOption = pub.ApprovalOption
	Actions        = pub.Actions
	// SettingsHostFacts 是设置面板的宿主事实（决定动态注册哪些 section/item）。
	SettingsHostFacts = pub.SettingsHostFacts
	Theme             = pub.Theme
	Radii             = pub.Radii
	Fonts             = pub.Fonts
)

const (
	KeyTheme      = pub.KeyTheme
	KeyPrimitives = pub.KeyPrimitives

	SlotShellRoot     = pub.SlotShellRoot
	SlotShellLeading  = pub.SlotShellLeading
	SlotShellBottom   = pub.SlotShellBottom
	SlotShellOverlay  = pub.SlotShellOverlay
	SlotSidebarHeader = pub.SlotSidebarHeader
	SlotSidebarList   = pub.SlotSidebarList
	SlotSidebarFooter = pub.SlotSidebarFooter
	SlotMainHeader    = pub.SlotMainHeader
	SlotMainBody      = pub.SlotMainBody
	SlotMainComposer  = pub.SlotMainComposer
	SlotRightbar      = pub.SlotRightbar
	SlotStatusbar     = pub.SlotStatusbar
)

// Builtin / Register 是注册表函数，别名层透传。
var (
	Builtin    = pub.Builtin
	Register   = pub.Register
	Registered = pub.Registered
)
