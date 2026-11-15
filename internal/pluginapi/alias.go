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
	JobOutput    = pub.JobOutput
	// PluginsTab / PluginsTabs 是 Plugins 页的 tab 贡献口（官方 `plugins.tab`）。
	PluginsTab  = pub.PluginsTab
	PluginsTabs = pub.PluginsTabs
	Model       = pub.Model
	// 引用 / 技能候选 / 消息反馈（pluginapi/references.go）。
	FileReferenceCandidate    = pub.FileReferenceCandidate
	SessionReferenceCandidate = pub.SessionReferenceCandidate
	SkillCandidate            = pub.SkillCandidate
	FeedbackItem              = pub.FeedbackItem
	FeedbackResult            = pub.FeedbackResult
	// 思考译文（pluginapi/cot_translation.go，#63）。
	CotTranslationResult = pub.CotTranslationResult
	// 官方 workspace registry 的一条工作区（#62）。
	Workspace = pub.Workspace
	// context menu 服务与贡献（#10）。
	ContextMenus            = pub.ContextMenus
	ContextMenuTarget       = pub.ContextMenuTarget
	ContextMenuAction       = pub.ContextMenuAction
	ContextMenuContribution = pub.ContextMenuContribution
	// 三个区域 More 菜单的 owner props 与固定态服务（#17 前置，见 entry_menu.go）。
	EntryPins      = pub.EntryPins
	EntryMenuProps = pub.EntryMenuProps
	EntryMenuEntry = pub.EntryMenuEntry
	Painter        = pub.Painter
	// Tab 是选择器条目（会话 view / 右栏 tab 的共同形状）。
	Tab = pub.Tab
	// WebHost / WebPage / WebOpenOptions 是窗口里的网页宿主（官方右栏浏览器那一层）。
	// `Env.Web` 已经带上了它；这里补别名是为了让 import internal 路径的插件也能
	// 在自己的测试里造假载具，而不必去 import 公开包。
	WebHost        = pub.WebHost
	WebPage        = pub.WebPage
	WebOpenOptions = pub.WebOpenOptions
	WebLoad        = pub.WebLoad
	WebLoadPhase   = pub.WebLoadPhase
	Primitives     = pub.Primitives
	Env            = pub.Env
	Geometry       = pub.Geometry
	Snapshot       = pub.Snapshot
	Usage          = pub.Usage
	Approval       = pub.Approval
	// HistoryPaging 是主区「加载更早」单按钮的五态输入（additive）。
	HistoryPaging = pub.HistoryPaging
	// LinkState 是壳与宿主流连接的三态事实（`Snapshot.Link`）。
	// 纯别名：取值常量见 pluginapi/link.go 的公开包。
	LinkState = pub.LinkState
	// ApprovalOption 是审批帧里 data.options 的一项（H13）。
	// 纯别名，不在这里加任何东西——两份定义会漂移。
	ApprovalOption = pub.ApprovalOption
	Actions        = pub.Actions
	// SettingsHostFacts 是设置面板的宿主事实（决定动态注册哪些 section/item）。
	SettingsHostFacts = pub.SettingsHostFacts
	// DesktopUpdatePresentation 是壳对"桌面更新"的事实呈现
	// （官方 `DesktopUpdateIndicator` 的 `view.presentation`）。纯别名。
	DesktopUpdatePresentation = pub.DesktopUpdatePresentation
	DesktopUpdatePhase        = pub.DesktopUpdatePhase
	DesktopUpdateFailureKind  = pub.DesktopUpdateFailureKind
	Theme                     = pub.Theme
	Radii                     = pub.Radii
	Fonts                     = pub.Fonts
)

const (
	KeyTheme      = pub.KeyTheme
	KeyPrimitives = pub.KeyPrimitives
	// KeyMarketDetail / KeyMarketDiscover 是 ds-market 交出的两个官方市场能力
	// （详情页本体 / 「发现」tab 本体，`#30` A/B/C 三块）。
	KeyMarketDetail   = pub.KeyMarketDetail
	KeyMarketDiscover = pub.KeyMarketDiscover
	// KeyPluginsTabs 是 Plugins 页接收 `plugins.tab` 贡献的服务键。
	KeyPluginsTabs = pub.KeyPluginsTabs
	// KeyContextMenus 是 ds-contextmenu 提供的菜单服务键。
	KeyContextMenus = pub.KeyContextMenus
	// KeyEntryPins 是壳提供的面板/view/guide 固定态服务键（纯内存）。
	KeyEntryPins = pub.KeyEntryPins
	// 三个区域 More 菜单的 region 身份（官方 EntryRegion）。
	EntryRegionSidebar      = pub.EntryRegionSidebar
	EntryRegionConversation = pub.EntryRegionConversation
	EntryRegionRightbar     = pub.EntryRegionRightbar

	// context menu 的 target kind（官方 `ContextMenuTargetMap` 的判别键，
	// OFF:ui-context-menu/src/client/types.ts:29-40 + open-in-app 的 workspace 扩展）。
	TargetSession   = pub.TargetSession
	TargetWorkspace = pub.TargetWorkspace
	TargetFile      = pub.TargetFile
	TargetImage     = pub.TargetImage

	// 原始字节保存的两种模式（官方 `'download' | 'saveAs'`）。
	FileSaveAs   = pub.FileSaveAs
	FileDownload = pub.FileDownload

	// 连接四态（`Snapshot.Link`）的取值：纯转发公开包的常量。
	LinkUnknown    = pub.LinkUnknown
	LinkConnected  = pub.LinkConnected
	LinkDown       = pub.LinkDown
	LinkConnecting = pub.LinkConnecting

	// 桌面更新的阶段与错因（官方 `DesktopUpdatePresentation['phase']` /
	// `DesktopUpdateFailureKind` 逐值）：纯转发公开包的常量。
	DesktopUpdateIdle                      = pub.DesktopUpdateIdle
	DesktopUpdateChecking                  = pub.DesktopUpdateChecking
	DesktopUpdateAvailable                 = pub.DesktopUpdateAvailable
	DesktopUpdateDownloading               = pub.DesktopUpdateDownloading
	DesktopUpdateVerifying                 = pub.DesktopUpdateVerifying
	DesktopUpdateInstalling                = pub.DesktopUpdateInstalling
	DesktopUpdateReady                     = pub.DesktopUpdateReady
	DesktopUpdateError                     = pub.DesktopUpdateError
	DesktopUpdateFailureConfiguration      = pub.DesktopUpdateFailureConfiguration
	DesktopUpdateFailureCheck              = pub.DesktopUpdateFailureCheck
	DesktopUpdateFailureCheckNetwork       = pub.DesktopUpdateFailureCheckNetwork
	DesktopUpdateFailureDownload           = pub.DesktopUpdateFailureDownload
	DesktopUpdateFailureDownloadSpace      = pub.DesktopUpdateFailureDownloadSpace
	DesktopUpdateFailureDownloadPermission = pub.DesktopUpdateFailureDownloadPermission
	DesktopUpdateFailureDownloadBusy       = pub.DesktopUpdateFailureDownloadBusy
	DesktopUpdateFailureDownloadNet        = pub.DesktopUpdateFailureDownloadNet
	DesktopUpdateFailureInstall            = pub.DesktopUpdateFailureInstall
	DesktopUpdateFailureInstallNetwork     = pub.DesktopUpdateFailureInstallNetwork
	DesktopUpdateFailureStopFailed         = pub.DesktopUpdateFailureStopFailed
	DesktopUpdateFailureTasksChanged       = pub.DesktopUpdateFailureTasksChanged
	DesktopUpdateFailureTasksUnavail       = pub.DesktopUpdateFailureTasksUnavail
	DesktopUpdateFailureFullscreen         = pub.DesktopUpdateFailureFullscreen

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
	// 选择器座位（keyed / list），见公开包里的注释与官方锚点。
	SlotRightbarSession   = pub.SlotRightbarSession
	SlotRightPaneTab      = pub.SlotRightPaneTab
	SlotRightPaneTabTitle = pub.SlotRightPaneTabTitle
	SlotConvView          = pub.SlotConvView

	// 网页加载相位（与公开包同三个值）。
	WebLoadStart     = pub.WebLoadStart
	WebLoadCommitted = pub.WebLoadCommitted
	WebLoadDone      = pub.WebLoadDone

	ViewChat       = pub.ViewChat
	ViewTrajectory = pub.ViewTrajectory
	ViewStats      = pub.ViewStats
)

// ErrWebUnsupported 是"这个部署放不了网页"的哨兵错误（公开包定义，这里透传）。
var ErrWebUnsupported = pub.ErrWebUnsupported

// Builtin / Register 是注册表函数，别名层透传。
var (
	Builtin    = pub.Builtin
	Register   = pub.Register
	Registered = pub.Registered
)
