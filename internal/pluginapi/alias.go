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
	// forge 面板 A 档的数据面与动作（#64a，pluginapi/forge.go）。
	ForgeView             = pub.ForgeView
	ForgeCheckout         = pub.ForgeCheckout
	ForgeWorktree         = pub.ForgeWorktree
	ForgeWorktreeCreation = pub.ForgeWorktreeCreation
	ForgeLocalSummary     = pub.ForgeLocalSummary
	ForgeDiff             = pub.ForgeDiff
	ForgeCommit           = pub.ForgeCommit
	ForgeActions          = pub.ForgeActions
	// Cordis 面板的只读数据面（#12，pluginapi/cordis.go）。
	//
	// 为什么必须在这里转发：壳（`cmd/app/cordis_wiring.go`）与插件
	// （`plugins/dscordis`）分别 import 两个路径，而这是**同一个类型**的两个名字。
	// 漏一行就是"插件编译得动、壳编译不动"（2026-11-15 实测踩到）。
	CordisView = pub.CordisView
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
	// TabBadge / TabBadgeTone / TabGuide 是芯片徽标与向导入口卡（公开包定义，纯别名）。
	TabBadge     = pub.TabBadge
	TabBadgeTone = pub.TabBadgeTone
	TabGuide     = pub.TabGuide
	// GuideEntryProps 是官方 `SidebarRightGuideEntryOwnerProps`（向导卡片的 owner props）。
	GuideEntryProps = pub.GuideEntryProps
	// TurnProps 是 transcript 按轮座位（官方 `conversation.chat.turnTail`）的
	// owner props（`TurnTailOwnerProps`，见公开包 pluginapi/turn_tail.go）。
	// 漏这一行就是"插件编译得动、壳编译不动"——与 CordisView 那次的踩法同一类。
	TurnProps = pub.TurnProps
	// RightTabParams 是官方 `SidebarRightTabParamsMap` 的投影（openTab 的导航参数）。
	RightTabParams = pub.RightTabParams
	// 右栏 occurrence 的实时标题 / 徽标与关闭清理（官方 sidebar-right 的三个扩展点）。
	RightTabTitleProvider = pub.RightTabTitleProvider
	// RightTabAddressTitleProvider 是资源型右栏 tab 的**带地址**实时标题
	// （官方 `tab.navigation.address`；见公开包 pluginapi/plugin.go）。
	RightTabAddressTitleProvider = pub.RightTabAddressTitleProvider
	RightTabBadgeProvider        = pub.RightTabBadgeProvider
	// RightTabRenameProvider 是芯片就地改名的写入面（官方 TerminalTitle 的
	// `view(tab.id).rename(next)`，见公开包 pluginapi/plugin.go）。
	RightTabRenameProvider = pub.RightTabRenameProvider
	RightTabCloseHandler   = pub.RightTabCloseHandler
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
	ApprovalOption       = pub.ApprovalOption
	TerminalViewState    = pub.TerminalViewState
	TerminalShellChoices = pub.TerminalShellChoices
	Actions              = pub.Actions
	StartSessionOptions  = pub.StartSessionOptions
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
	// ShadowLayer / Elevation / Shadows 是官方阴影令牌（pluginapi/theme.go）。
	// 漏一行就是「插件编译得动、壳编译不动」，同 CordisView 那次的踩法。
	ShadowLayer = pub.ShadowLayer
	Elevation   = pub.Elevation
	Shadows     = pub.Shadows
	// 界面状态（官方 Local Storage 的等价物）：Actions.UIState 的动作组。纯别名。
	UIStateActions = pub.UIStateActions
	// 麦克风听写（#66）：Actions.Voice 的动作组与三份只读快照。纯别名。
	VoiceActions    = pub.VoiceActions
	VoiceCapability = pub.VoiceCapability
	VoiceProvider   = pub.VoiceProvider
	VoiceDevice     = pub.VoiceDevice
	VoiceState      = pub.VoiceState
	// 草稿插入的快照类型（`InsertionSpan`）与提供方接口（`DraftInsertion`）。
	// 键 `KeyDraftInsertion` 在下面的 const 块里（与其余服务键同一处）。
	InsertionSpan  = pub.InsertionSpan
	DraftInsertion = pub.DraftInsertion
	// 消息区图片面的取数结果（Actions.AttachmentGet 的返回值，#18 ②③）。
	AttachmentBlob = pub.AttachmentBlob
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
	DraftReplace            = pub.DraftReplace
	DraftAppend             = pub.DraftAppend
	DraftKeep               = pub.DraftKeep
	DraftInsert             = pub.DraftInsert
	// 草稿插入能力（官方 `inputActions.captureInsertion/insertText`，#66 ①）。
	// 漏转发的后果与 CordisView 那次同类：插件（dscomposer）编译得动、壳编译不动。
	KeyDraftInsertion = pub.KeyDraftInsertion

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
	// 右栏向导页（guide）的三个官方座位，见公开包里的注释与官方锚点。
	SlotRightGuide        = pub.SlotRightGuide
	SlotRightGuideActions = pub.SlotRightGuideActions
	SlotRightGuideEntry   = pub.SlotRightGuideEntry

	// 芯片徽标的两个官方 tone（`Tag tone="info"` / `tone="neutral"`）。
	TabBadgeInfo    = pub.TabBadgeInfo
	TabBadgeNeutral = pub.TabBadgeNeutral

	// 向导卡片 shell 菜单的三态（官方 TerminalGuide.tsx 的 MenuState）。
	TerminalShellsLoading = pub.TerminalShellsLoading
	TerminalShellsReady   = pub.TerminalShellsReady
	TerminalShellsFailed  = pub.TerminalShellsFailed

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

// 官方 checkout 四态的取值（公开包里的常量在这一层不重复定义，直接转发常量值）。
const (
	ForgeCheckoutPending = pub.ForgeCheckoutPending
	ForgeCheckoutNone    = pub.ForgeCheckoutNone
	ForgeCheckoutFailed  = pub.ForgeCheckoutFailed
	ForgeCheckoutReady   = pub.ForgeCheckoutReady

	// 官方 `ForgeWorktreeBlocker` 三值 + `ForgeWorktreeCreation.phase` 两值
	// （pluginapi/forge.go 的转发，壳与插件各 import 一个路径）。
	ForgeWorktreeBlockerNotGit              = pub.ForgeWorktreeBlockerNotGit
	ForgeWorktreeBlockerOutsideWritableRoot = pub.ForgeWorktreeBlockerOutsideWritableRoot
	ForgeWorktreeBlockerReadOnly            = pub.ForgeWorktreeBlockerReadOnly
	ForgeCreationPrepare                    = pub.ForgeCreationPrepare
	ForgeCreationCheckout                   = pub.ForgeCreationCheckout
)
