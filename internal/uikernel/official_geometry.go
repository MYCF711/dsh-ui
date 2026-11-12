package uikernel

// official_geometry.go —— 首屏 shell 的**唯一几何常量表**（rc.2 口径）。
//
// 规矩（本任务的核心交付之一）：
//
//	① 结构树里的每个 gap / padding / min-max 宽高只有一个出处：本文件的常量。
//	   绘制代码（shell_skeleton.go）只准引用常量，不许再写魔法数。
//	② 「官方值」与「实现值」写在**同一张表**里（officialGeometry 的每一行），
//	   测试逐行核对：aligned 行实现值必须等于官方值；deviation 行必须写清原因。
//	③ 官方值是**从 rc.2 随包 `src/**` 的 CSS Module / columns.ts 里读出来的**，
//	   不是从截图猜的，也不是沿用 rc.1 的审计口径（结构树 §9.2 明确写：除
//	   headerActions.gap 外的数值仍是 rc.1 口径）。锚点一律 `pkg/src/…:line`。
//
// 本文件顺手核到、但 H2 报告没列的 rc.2 数值变化（报告 GA-shell-skeleton.md §3）：
//
//	.ui-sidebar `.root` 内联 padding    12px → **8px**（SidebarRoot.module.css:10,17）
//	`.panelRow` min-height              36px → **32px**（SidebarRoot.module.css:690）
//	`.headerActions` gap                8px  → **10px**（ConversationRoot.module.css:175）
//
// 表里出现 `deviation` 的唯一原因是「遗留桥」：本任务不动 plugins/**，
// 旧内容插件（dssidebar / dstopbar / dsconversation / dscomposer）仍按 dsshell
// 时代那套 12px 栏内边距排版。把官方裸列模型（center inset 0）直接套到它们身上
// 会让 composer 净边距从 16 掉到 4（plugins/dsshell/shell.go 的 columnPad 注释
// 已写明要四个插件同一次改），所以桥模式下保留 12 并在这里登记为 deviation，
// 等后续任务把内容插件迁到官方槽位后删掉。

import "github.com/MYCF711/dsh-ui/internal/pluginapi"

// GeoUnit 是值的量纲。像素之外只有比例/百分比/派生公式三种，
// 混着写会让「10 是 10px 还是 10%」这种问题变成口头约定。
type GeoUnit string

const (
	UnitPx     GeoUnit = "px"
	UnitRatio  GeoUnit = "ratio"
	UnitPct    GeoUnit = "percent"
	UnitZIndex GeoUnit = "z-index"
)

// GeoStatus 说明一行「官方值 ↔ 实现值」的关系。
type GeoStatus string

const (
	// GeoAligned 实现值与官方值一致（本任务要求：首屏结构容器全部 aligned）。
	GeoAligned GeoStatus = "aligned"
	// GeoDerived 官方是 calc/clamp/内容自适应，没有单一数值；实现值是实现侧的
	// 等价取值，必须同时写 Formula。
	GeoDerived GeoStatus = "derived"
	// GeoDeviation 实现值与官方值**不同**，必须写 Reason，且只能是为了兼容
	// 尚未迁移的旧插件（遗留桥）。
	GeoDeviation GeoStatus = "deviation"
)

// GeoRow 是一行几何事实。
type GeoRow struct {
	Node     string    // 官方节点（AppFrame.frame / ConversationRoot.header …）
	Prop     string    // 属性（gap / padding-inline / min-height …）
	Official float32   // 官方值；derived 行填实现侧等价取值
	Impl     float32   // 本仓实现值（同一行，不另表）
	Unit     GeoUnit   // 量纲
	Status   GeoStatus // aligned / derived / deviation
	Anchor   string    // 官方 rc.2 源码锚点
	Formula  string    // derived 行的官方原文（calc/clamp/内容自适应）
	Reason   string    // deviation 行必须写原因
}

// ── AppFrame：窗口框架与三列 ─────────────────────────────────────────
const (
	// CaptionHeight 是 Windows 标题栏高度（`--dsh-windows-titlebar-height`）。
	// 只由 L0 扣一次：frame 的 padding-top，不是每列各扣一次。
	CaptionHeight float32 = 40
	// ContentRadius 是 Windows 内容区左上圆角（`--dsh-windows-content-radius`）。
	ContentRadius float32 = 16

	// 三列求解（columns.ts 逐行移植）。官方值：
	//	SIDEBAR_MIN 264 / SIDEBAR_MAX 420 / SIDEBAR_DEFAULT 280
	//	SIDEBAR_COLLAPSED 56 / CENTER_MIN 400
	//	RIGHTBAR_MIN 300 / RIGHTBAR_MAX_RATIO .7 / RIGHTBAR_DEFAULT_RATIO .45
	SidebarMin           float32 = 264
	SidebarMax           float32 = 420
	SidebarDefault       float32 = 280
	SidebarCollapsed     float32 = 56
	SidebarDrawerBreak   float32 = 768
	SidebarAutoCollapse  float32 = 1024
	CenterMin            float32 = 400
	RightbarMin          float32 = 300
	RightbarMaxRatio     float32 = 0.7
	RightbarDefaultRatio float32 = 0.45

	// HandleWidth / HandleOffset：拖拽命中带 8px，骑在列边界上（margin-left -4px）。
	HandleWidth  float32 = 8
	HandleOffset float32 = 4

	// 浮层与座位层级（AppFrame.module.css）。
	OverlayZ         float32 = 20
	DrawerZ          float32 = 71
	DrawerMaskZ      float32 = 70
	LeadingSeatTop   float32 = 11
	LeadingSeatLeft  float32 = 88
	LeadingSeatLeftD float32 = 12 // drawer 模式（.frame[data-sidebar-drawer] .leadingSeat）

	// BottomDefaultHeight：官方 bottomRow 高度由占用方决定，空内容不预留。
	BottomDefaultHeight float32 = 0
)

// ── SidebarRoot：左栏 ────────────────────────────────────────────────
const (
	// rc.2：padding 6px var(--dsh-sidebar-inline-padding)，inline=8px。
	SidebarRootPadY   float32 = 6
	SidebarRootPadX   float32 = 8
	SidebarRootRowPad float32 = 12 // --dsh-sidebar-row-padding
	SidebarRootLabel  float32 = 6  // --dsh-sidebar-label-gap
	SidebarRootInset  float32 = 3  // --dsh-sidebar-paint-inset

	SidebarCollapsedPadTop float32 = 18
	SidebarCollapsedPadX   float32 = 10

	// logoRow：Web 60 / Windows 40；padding 8px 0 8px 8px；gap 8。
	SidebarLogoRowHeight      float32 = 60
	SidebarLogoRowHeightWin   float32 = 40
	SidebarLogoRowPadTop      float32 = 8
	SidebarLogoRowPadLeft     float32 = 8
	SidebarLogoRowGap         float32 = 8
	SidebarBrandIdentityGap   float32 = 5
	SidebarBrandIdentityH     float32 = 24
	SidebarBrandFontSize      float32 = 18
	SidebarBrandLineHeight    float32 = 24
	SidebarBrandLetterSpacing float32 = 0.04
	SidebarTopStripHeight     float32 = 52

	// newSession：高 38、radius-md、border .5px l3、padding 8px 12px、
	// margin 0 calc(paint-inset + 1px)、font 14/22 500、内容行 gap 6。
	SidebarNewSessionHeight float32 = 38
	SidebarNewSessionPadX   float32 = 12
	SidebarNewSessionPadY   float32 = 8
	SidebarNewSessionGap    float32 = 6
	SidebarNewSessionMargin float32 = 4 // paint-inset 3 + 1
	SidebarNewSessionRadius float32 = 8 // --dsw-radius-md

	// 设置行（`SettingsRoot.triggerRow`，官方 `sidebar.settings` 的唯一一行）：
	//
	//	.triggerRow{flex:none;align-items:center;gap:8px;width:calc(100% + 4px);
	//	            margin:4px -2px;display:flex;position:relative}
	//	.trigger{height:42px;border-radius:var(--dsw-radius-md);flex:1;
	//	         padding:0 10px 0 8px;font:14px/22px;gap:8px;display:flex}
	//
	// 结构树 §4 S8 的判定同一句：「官方是 [设置按钮(flex:1)] [连接指示] 同一行」。
	// 横向 -2px 是官方的 paint-inset（`--dsh-sidebar-paint-inset` 的一半）：
	// 行比侧栏内容宽 4px，左右各溢出 2px，让选中/悬停底色的边缘贴到栏边。
	SettingsTriggerRowMarginY float32 = 4
	SettingsTriggerRowInset   float32 = 2 // margin-inline: -2px
	SettingsTriggerRowGap     float32 = 8

	// panelList：list gap 2、bottom margin 8；panelRow min-height 32、
	// padding 5px calc(12px - 3px)、margin 0。
	SidebarPanelListGap    float32 = 2
	SidebarPanelListBottom float32 = 8
	SidebarPanelRowMinH    float32 = 32
	SidebarPanelRowPadY    float32 = 5
	SidebarPanelRowPadX    float32 = 9 // 12 - 3（paint inset 由 border-inline 承担）

	SidebarScrollRegionTop float32 = 4 // .scrollRegion{margin-top:4px}
	SidebarScrollFadeH     float32 = 24
)

// ── ConversationRoot：header / tabs / body / composer seat ────────────
const (
	// header：有 tabs 时 min-height 76、padding 10px 28px 0 20px、下边框 .5px l3；
	// 无 tabs 时 min-height 0 + padding-bottom 10；blank 去底 padding 与边框。
	HeaderMinHeightTabs float32 = 76
	HeaderPadTop        float32 = 10
	HeaderPadRight      float32 = 28
	HeaderPadLeft       float32 = 20
	HeaderPadBottomNone float32 = 10
	HeaderHairline      float32 = 1  // 0.5px border-bottom，布局上占 1
	HeaderBlankHeight   float32 = 40 // blank：10 top + 30 title row（官方注释明写 40px 行）

	TitleRowMinHeight  float32 = 30
	TitleClusterGap    float32 = 10
	TitleClusterFade   float32 = 20
	TitleClusterFadeSm float32 = 8 // utilities 为空时
	TitleClusterMargin float32 = 4

	CrumbsGap      float32 = 4
	CrumbSegGap    float32 = 4
	CrumbMaxWidth  float32 = 220
	CrumbPadX      float32 = 8
	CrumbPadY      float32 = 4
	CrumbFontSize  float32 = 14
	CrumbLineH     float32 = 20
	CrumbSepFontSz float32 = 14
	CrumbSepLineH  float32 = 20

	// headerActions 的 gap 是 rc.1→rc.2 的**真变**：8px → 10px。
	// 窄容器（@container max-width 460px）另有 8px 覆盖，两个值都要有行。
	HeaderActionsGap       float32 = 10
	HeaderActionsGapNarrow float32 = 8
	HeaderActionsNarrowMax float32 = 460

	HeaderUtilitiesGap        float32 = 8
	HeaderUtilitiesMarginLeft float32 = 20
	HeaderCornerMarginLeft    float32 = 8
	HeaderCornerMarginRight   float32 = 16 // 官方 -16px

	// tabs / viewTabs / tab（官方写死值）。
	TabsGapRow       float32 = 16
	TabsGapCol       float32 = 20
	TabsMarginTop    float32 = 10
	TabsPaddingLeft  float32 = 8
	ViewTabsGap      float32 = 32
	ViewTabsMinH     float32 = 25
	TabHitInline     float32 = 10 // --dsh-tab-hit-inline
	TabMinHeight     float32 = 35
	TabPadTop        float32 = 10
	TabPadBottom     float32 = 9
	TabFontSize      float32 = 13
	TabLineHeight    float32 = 16
	TabBarHeight     float32 = 2
	TabLabelMaxWidth float32 = 88
	TabMoreSize      float32 = 22

	// body / scrollBody / composerSeat。
	BodyScrollMarginRight float32 = 2
	ComposerSeatZ         float32 = 9
	ComposerSeatFade      float32 = 36
	ComposerStackGap      float32 = 6

	// 正文宽轴：clamp(680px, 主列宽×.64, 920px)，composer 卡片上限 = 正文宽 + 32。
	ContentWidthMin   float32 = 680
	ContentWidthRatio float32 = 0.64
	ContentWidthMax   float32 = 920
	CardMaxExtra      float32 = 32

	// composer 卡片：radius-panel 28、padding-top 8、gap 12；
	// 输入区 min-height 36（hero 52）、padding 4px 8px 0 14px；
	// 控件行 gap 12、padding 2px 8px 6px；主按钮 34×34；
	// dock gap 12、padding-top 4；seat 内含三段 → 静止高度 128px。
	//
	// 控件行高度**不是** 28（trigger 高）+ padding：官方 `.row` 没有 height，
	// 它被子元素撑开，而最矮的 trigger 是 28、主按钮是 **34**（`.primary` 的
	// `transform:translateY(-2px)` 不参与布局，只挪视觉位置）。所以内容高 = 34，
	// 行高 = 2+34+6 = 42，卡片 = 8+36+12+42 = 98，座位 = 98+26+4 = **128**。
	// 早先按 28 算出的 36/92/122 与真渲染差 6px（dscomposer 的卡片一直是 98）——
	// 两个数字对不上，就是这一条。
	ComposerCardRadius  float32 = 28
	ComposerCardPadTop  float32 = 8
	ComposerCardGap     float32 = 12
	ComposerInputMinH   float32 = 36
	ComposerInputHeroH  float32 = 52
	ComposerInputPadTop float32 = 4
	ComposerInputPadX   float32 = 8
	ComposerInputPadL   float32 = 14
	ComposerRowGap      float32 = 12
	ComposerRowPadTop   float32 = 2
	ComposerRowPadBot   float32 = 6
	ComposerRowPadX     float32 = 8
	ComposerRowHeight   float32 = ComposerPrimarySize + ComposerRowPadTop + ComposerRowPadBot
	ComposerPrimarySize float32 = 34
	ComposerDockGap     float32 = 12
	ComposerDockPadTop  float32 = 4
	ComposerDockMinH    float32 = 22 // 非 hero 的 min-height calc(22px + font-delta-secondary)
	ComposerPadBottom   float32 = 4
	ComposerSeatHeight  float32 = ComposerCardPadTop + ComposerInputMinH + ComposerCardGap +
		ComposerRowHeight + ComposerDockPadTop + ComposerDockMinH + ComposerPadBottom

	ComposerSideClearance float32 = 16
	ComposerDockInset     float32 = 8
	ComposerTextMaxHeight float32 = 336

	// 控件行左端的「加号」命令/附件入口（官方 `.add`，InputBar.module.css:336-348）：
	// 28×28 圆形、背景 --dsw-specific-selector（主题里是 BgLayer3）、图标
	// IconPlusOutlineMedium size=14、disabled 时 opacity .5。
	ComposerAddSize   float32 = 28
	ComposerAddIcon   float32 = 14
	ComposerAddRadius float32 = 999
	ComposerAddDim    float32 = 0.5

	// 卡片下方 dock 里的读数胶囊（官方 StatsPills `.pill` + ContextMeter
	// `.trigger`，ui-chat/src/client/chat/StatsPills.module.css:13-60、
	// ui-conversation/src/client/skeleton/ContextMeter.module.css:9-38）：
	// padding 1px 8px、gap 6、radius-sm、字号 calc(13px-1px)、行高 20、图标 14。
	ComposerPillPadY   float32 = 1
	ComposerPillPadX   float32 = 8
	ComposerPillGap    float32 = 6
	ComposerPillFont   float32 = 12
	ComposerPillLineH  float32 = 20
	ComposerPillIconWH float32 = 14
)

// ── ChatView / HeroShell ─────────────────────────────────────────────
const (
	ChatScrollPad   float32 = 16 // scroll padding 16px calc(clearance + 16px)
	ChatScrollSideX float32 = 16
	ChatFlowGap     float32 = 4
	ChatResponseGap float32 = 12
	ChatTurnProcGap float32 = 16
	ChatColumnMaxH  float32 = 0 // column 宽度走 ContentWidth*（无定高）

	HeroPadX             float32 = 24
	HeroStackGap         float32 = 12
	HeroHeadlineFontSize float32 = 26
	HeroHeadlineLineH    float32 = 32
	HeroHeadlineGapCol   float32 = 10
	HeroHeadlineGapRow   float32 = 12
	HeroTitleGapCol      float32 = 7
	HeroTitleGapRow      float32 = 4
	HeroFishSize         float32 = 34
	HeroBadgeFontSize    float32 = 12
	HeroBadgeLineH       float32 = 18
	HeroBadgePadX        float32 = 7
	HeroBadgePadTop      float32 = 1
	HeroWorkspaceMinH    float32 = 28
	HeroWorkspaceMaxW    float32 = 360
	HeroWorkspacePadX    float32 = 8
	HeroWorkspaceGap     float32 = 4
)

// ── 右栏 DockKit ────────────────────────────────────────────────────
const (
	RightbarPaneZ       float32 = 10
	RightbarFloatZ      float32 = 60
	RightbarFullscreenZ float32 = 40
	RightbarIconSize    float32 = 28
	RightbarIconPad     float32 = 6
	RightbarIconSvg     float32 = 15
	RightbarUnavailPad  float32 = 12

	// rc.2 pane 闪烁改成属性选择器（[data-sidebar-right-flash='odd'/'even']），
	// 三个 ring 百分比没变：48% / 16% / 10%。
	FlashRing float32 = 48
	FlashNear float32 = 16
	FlashFar  float32 = 10
	FlashZ    float32 = 7
)

// ── 遗留桥模式（只用于旧插件迁移前，见文件头说明）───────────────────
const (
	// LegacyColumnInset 是 dsshell 时代三列的内边距（官方中栏/右栏是裸列 0）。
	LegacyColumnInset float32 = 12
	// LegacySidebarPadX 是 dsshell 时代左栏内联 padding（rc.2 官方 8）。
	LegacySidebarPadX float32 = 12
)

// officialGeometry 是官方值 ↔ 实现值的**同一张表**。
// 新增几何一律加行，不许在绘制代码里写字面量。
var officialGeometry = []GeoRow{
	// AppFrame.frame
	{Node: "AppFrame.frame", Prop: "grid-template-rows", Official: 0, Impl: 0, Unit: UnitPx, Status: GeoDerived,
		Anchor: "ui-layout/src/client/AppFrame.module.css:4", Formula: "minmax(0,1fr) auto（主体自适应 + 底部整行自动）"},
	{Node: "AppFrame.frame", Prop: "padding-top (Windows caption)", Official: CaptionHeight, Impl: CaptionHeight, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-layout/src/client/AppFrame.module.css:30"},
	{Node: "AppFrame.frame", Prop: "background", Official: 0, Impl: 0, Unit: UnitPx, Status: GeoDerived,
		Anchor: "ui-layout/src/client/AppFrame.module.css:8", Formula: "var(--dsw-alias-bg-base)"},
	{Node: "AppFrame.frame", Prop: "content corner radius (Windows)", Official: ContentRadius, Impl: ContentRadius, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-layout/src/client/AppFrame.module.css:28,36"},
	{Node: "AppFrame.handle", Prop: "width", Official: HandleWidth, Impl: HandleWidth, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-layout/src/client/AppFrame.module.css:225"},
	{Node: "AppFrame.handle", Prop: "margin-left", Official: -HandleOffset, Impl: -HandleOffset, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-layout/src/client/AppFrame.module.css:226"},
	{Node: "AppFrame.overlayLayer", Prop: "z-index", Official: OverlayZ, Impl: OverlayZ, Unit: UnitZIndex, Status: GeoAligned,
		Anchor: "ui-layout/src/client/AppFrame.module.css:366"},
	{Node: "AppFrame.leadingSeat", Prop: "top", Official: LeadingSeatTop, Impl: LeadingSeatTop, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-layout/src/client/AppFrame.module.css:206"},
	{Node: "AppFrame.leadingSeat", Prop: "left", Official: LeadingSeatLeft, Impl: LeadingSeatLeft, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-layout/src/client/AppFrame.module.css:207"},
	{Node: "AppFrame.leadingSeat", Prop: "left (drawer)", Official: LeadingSeatLeftD, Impl: LeadingSeatLeftD, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-layout/src/client/AppFrame.module.css:328"},
	{Node: "AppFrame.sidebarCol", Prop: "border-right", Official: 1, Impl: 1, Unit: UnitPx, Status: GeoDerived,
		Anchor: "ui-layout/src/client/AppFrame.module.css:65", Formula: "0.5px solid var(--dsw-alias-border-l3)（Windows 无边框）"},
	{Node: "AppFrame.bottomRow", Prop: "height (empty)", Official: BottomDefaultHeight, Impl: BottomDefaultHeight, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-layout/src/client/AppFrame.module.css:355-361"},

	// 三列求解
	{Node: "columns", Prop: "sidebar min", Official: SidebarMin, Impl: SidebarMin, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-layout/src/client/columns.ts:17"},
	{Node: "columns", Prop: "sidebar max", Official: SidebarMax, Impl: SidebarMax, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-layout/src/client/columns.ts:19"},
	{Node: "columns", Prop: "sidebar default", Official: SidebarDefault, Impl: SidebarDefault, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-layout/src/client/columns.ts:21"},
	{Node: "columns", Prop: "sidebar collapsed rail", Official: SidebarCollapsed, Impl: SidebarCollapsed, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-layout/src/client/columns.ts:23"},
	{Node: "columns", Prop: "sidebar drawer breakpoint", Official: SidebarDrawerBreak, Impl: SidebarDrawerBreak, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-layout/src/client/columns.ts:25"},
	{Node: "columns", Prop: "sidebar auto collapse", Official: SidebarAutoCollapse, Impl: SidebarAutoCollapse, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-layout/src/client/columns.ts:28"},
	{Node: "columns", Prop: "center min", Official: CenterMin, Impl: CenterMin, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-layout/src/client/columns.ts:14"},
	{Node: "columns", Prop: "rightbar min", Official: RightbarMin, Impl: RightbarMin, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-layout/src/client/columns.ts:30"},
	{Node: "columns", Prop: "rightbar max ratio", Official: RightbarMaxRatio, Impl: RightbarMaxRatio, Unit: UnitRatio, Status: GeoAligned,
		Anchor: "ui-layout/src/client/columns.ts:29"},
	{Node: "columns", Prop: "rightbar first-open ratio", Official: RightbarDefaultRatio, Impl: RightbarDefaultRatio, Unit: UnitRatio, Status: GeoAligned,
		Anchor: "ui-layout/src/client/columns.ts:34"},

	// SidebarRoot
	{Node: "SidebarRoot.root", Prop: "padding-block", Official: SidebarRootPadY, Impl: SidebarRootPadY, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:17"},
	// 设置行（`SettingsRoot.triggerRow`）：官方是**一行 flex**，本仓的
	// `sidebar.settings` 槽由壳包成 Row 派发，三个子项（设置按钮/连接指示/更新指示）
	// 同 Y。三个值逐条来自 SettingsRoot.module.css:1-9。
	{Node: "SettingsRoot.triggerRow", Prop: "margin-block", Official: SettingsTriggerRowMarginY, Impl: SettingsTriggerRowMarginY, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:1-9"},
	{Node: "SettingsRoot.triggerRow", Prop: "margin-inline", Official: -SettingsTriggerRowInset, Impl: -SettingsTriggerRowInset, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:1-9"},
	{Node: "SettingsRoot.triggerRow", Prop: "gap", Official: SettingsTriggerRowGap, Impl: SettingsTriggerRowGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:1-9"},
	{Node: "SidebarRoot.root", Prop: "padding-inline", Official: SidebarRootPadX, Impl: SidebarRootPadX, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:10,17"},
	{Node: "SidebarRoot.root", Prop: "font-size", Official: 14, Impl: 14, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:21"},
	{Node: "SidebarRoot.root", Prop: "padding-block (collapsed)", Official: SidebarCollapsedPadTop, Impl: SidebarCollapsedPadTop, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:42"},
	{Node: "SidebarRoot.root", Prop: "padding-inline (collapsed)", Official: SidebarCollapsedPadX, Impl: SidebarCollapsedPadX, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:42"},
	{Node: "SidebarRoot.logoRow", Prop: "height (web)", Official: SidebarLogoRowHeight, Impl: SidebarLogoRowHeight, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:118"},
	{Node: "SidebarRoot.logoRow", Prop: "height (windows)", Official: SidebarLogoRowHeightWin, Impl: SidebarLogoRowHeightWin, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:47"},
	{Node: "SidebarRoot.logoRow", Prop: "padding", Official: SidebarLogoRowPadTop, Impl: SidebarLogoRowPadTop, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:119"},
	{Node: "SidebarRoot.logoRow", Prop: "gap", Official: SidebarLogoRowGap, Impl: SidebarLogoRowGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:117"},
	{Node: "SidebarRoot.brandIdentity", Prop: "gap", Official: SidebarBrandIdentityGap, Impl: SidebarBrandIdentityGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:163"},
	{Node: "SidebarRoot.brandIdentity", Prop: "height", Official: SidebarBrandIdentityH, Impl: SidebarBrandIdentityH, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:164"},
	{Node: "SidebarRoot.brandName", Prop: "font-size/line-height", Official: SidebarBrandFontSize, Impl: SidebarBrandFontSize, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:300"},
	{Node: "SidebarRoot.brandName", Prop: "line-height", Official: SidebarBrandLineHeight, Impl: SidebarBrandLineHeight, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:302"},
	{Node: "SidebarRoot.topStrip", Prop: "height", Official: SidebarTopStripHeight, Impl: SidebarTopStripHeight, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:73"},
	{Node: "SidebarRoot.newSession", Prop: "height", Official: SidebarNewSessionHeight, Impl: SidebarNewSessionHeight, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:429"},
	{Node: "SidebarRoot.newSession", Prop: "padding", Official: SidebarNewSessionPadX, Impl: SidebarNewSessionPadX, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:429"},
	{Node: "SidebarRoot.newSession", Prop: "divider content gap", Official: SidebarNewSessionGap, Impl: SidebarNewSessionGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:512"},
	{Node: "SidebarRoot.panelList", Prop: "gap", Official: SidebarPanelListGap, Impl: SidebarPanelListGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:679"},
	{Node: "SidebarRoot.panelList", Prop: "margin-bottom", Official: SidebarPanelListBottom, Impl: SidebarPanelListBottom, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:680"},
	{Node: "SidebarRoot.panelRow", Prop: "min-height", Official: SidebarPanelRowMinH, Impl: SidebarPanelRowMinH, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:690"},
	{Node: "SidebarRoot.panelRow", Prop: "padding-block", Official: SidebarPanelRowPadY, Impl: SidebarPanelRowPadY, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:691"},
	{Node: "SidebarRoot.scrollRegion", Prop: "margin-top", Official: SidebarScrollRegionTop, Impl: SidebarScrollRegionTop, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:533"},
	{Node: "SidebarRoot.footArea", Prop: "height", Official: 0, Impl: 0, Unit: UnitPx, Status: GeoDerived,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:781-785", Formula: "内容决定（flex:none），默认空"},

	// ConversationRoot.header
	{Node: "ConversationRoot.header", Prop: "min-height (tabs)", Official: HeaderMinHeightTabs, Impl: HeaderMinHeightTabs, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:29"},
	{Node: "ConversationRoot.header", Prop: "padding-top", Official: HeaderPadTop, Impl: HeaderPadTop, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:30"},
	{Node: "ConversationRoot.header", Prop: "padding-right", Official: HeaderPadRight, Impl: HeaderPadRight, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:30"},
	{Node: "ConversationRoot.header", Prop: "padding-left", Official: HeaderPadLeft, Impl: HeaderPadLeft, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:30"},
	{Node: "ConversationRoot.header", Prop: "padding-bottom (no tabs)", Official: HeaderPadBottomNone, Impl: HeaderPadBottomNone, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:36"},
	{Node: "ConversationRoot.header", Prop: "border-bottom", Official: HeaderHairline, Impl: HeaderHairline, Unit: UnitPx, Status: GeoDerived,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:31", Formula: "0.5px solid var(--dsw-alias-border-l3)"},
	{Node: "ConversationRoot.headerBlank", Prop: "height", Official: HeaderBlankHeight, Impl: HeaderBlankHeight, Unit: UnitPx, Status: GeoDerived,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:42-45", Formula: "10px inset + 30px title row（官方注释明写 40px）"},

	// titleRow / crumbs
	{Node: "ConversationRoot.titleRow", Prop: "min-height", Official: TitleRowMinHeight, Impl: TitleRowMinHeight, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:76"},
	{Node: "ConversationRoot.titleCluster", Prop: "gap", Official: TitleClusterGap, Impl: TitleClusterGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:95"},
	{Node: "ConversationRoot.titleCluster", Prop: "fade width", Official: TitleClusterFade, Impl: TitleClusterFade, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:97"},
	{Node: "ConversationRoot.crumbs", Prop: "gap", Official: CrumbsGap, Impl: CrumbsGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:123"},
	{Node: "ConversationRoot.crumb", Prop: "max-width", Official: CrumbMaxWidth, Impl: CrumbMaxWidth, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:145"},
	{Node: "ConversationRoot.crumb", Prop: "padding-inline", Official: CrumbPadX, Impl: CrumbPadX, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:147"},
	{Node: "ConversationRoot.crumb", Prop: "padding-block", Official: CrumbPadY, Impl: CrumbPadY, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:147"},
	{Node: "ConversationRoot.crumb", Prop: "font-size", Official: CrumbFontSize, Impl: CrumbFontSize, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:151"},
	{Node: "ConversationRoot.crumb", Prop: "line-height", Official: CrumbLineH, Impl: CrumbLineH, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:152"},
	{Node: "ConversationRoot.crumbSep", Prop: "font-size", Official: CrumbSepFontSz, Impl: CrumbSepFontSz, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:139"},
	{Node: "ConversationRoot.crumbSep", Prop: "line-height", Official: CrumbSepLineH, Impl: CrumbSepLineH, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:140"},

	// header actions —— rc.2 真变（8 → 10）
	{Node: "ConversationRoot.headerActions", Prop: "gap", Official: HeaderActionsGap, Impl: HeaderActionsGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:175"},
	{Node: "ConversationRoot.headerActions", Prop: "gap (@container <=460px)", Official: HeaderActionsGapNarrow, Impl: HeaderActionsGapNarrow, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:178-181"},
	{Node: "ConversationRoot.headerActions", Prop: "narrow container max-width", Official: HeaderActionsNarrowMax, Impl: HeaderActionsNarrowMax, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:178"},
	{Node: "ConversationRoot.headerUtilities", Prop: "gap", Official: HeaderUtilitiesGap, Impl: HeaderUtilitiesGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:190"},
	{Node: "ConversationRoot.headerUtilities", Prop: "margin-left", Official: HeaderUtilitiesMarginLeft, Impl: HeaderUtilitiesMarginLeft, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:189"},
	{Node: "ConversationRoot.headerCorner", Prop: "margin-left", Official: HeaderCornerMarginLeft, Impl: HeaderCornerMarginLeft, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:216"},
	{Node: "ConversationRoot.headerCorner", Prop: "margin-right", Official: -HeaderCornerMarginRight, Impl: -HeaderCornerMarginRight, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:217"},

	// tabs
	{Node: "ConversationRoot.tabs", Prop: "row-gap", Official: TabsGapRow, Impl: TabsGapRow, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:230"},
	{Node: "ConversationRoot.tabs", Prop: "column-gap", Official: TabsGapCol, Impl: TabsGapCol, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:230"},
	{Node: "ConversationRoot.tabs", Prop: "margin-top", Official: TabsMarginTop, Impl: TabsMarginTop, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:236"},
	{Node: "ConversationRoot.tabs", Prop: "padding-left", Official: TabsPaddingLeft, Impl: TabsPaddingLeft, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:237"},
	{Node: "ConversationRoot.viewTabs", Prop: "gap", Official: ViewTabsGap, Impl: ViewTabsGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:249"},
	{Node: "ConversationRoot.viewTabs", Prop: "min-height", Official: ViewTabsMinH, Impl: ViewTabsMinH, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:251"},
	{Node: "ConversationRoot.tab", Prop: "min-height", Official: TabMinHeight, Impl: TabMinHeight, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:265"},
	{Node: "ConversationRoot.tab", Prop: "padding-top", Official: TabPadTop, Impl: TabPadTop, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:267"},
	{Node: "ConversationRoot.tab", Prop: "padding-bottom", Official: TabPadBottom, Impl: TabPadBottom, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:267"},
	{Node: "ConversationRoot.tab", Prop: "font-size", Official: TabFontSize, Impl: TabFontSize, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:270"},
	{Node: "ConversationRoot.tab", Prop: "line-height", Official: TabLineHeight, Impl: TabLineHeight, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:271"},
	{Node: "ConversationRoot.tab", Prop: "active bar height", Official: TabBarHeight, Impl: TabBarHeight, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:277-289"},
	{Node: "ConversationRoot.tab", Prop: "hit inline padding", Official: TabHitInline, Impl: TabHitInline, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:261"},
	{Node: "ConversationRoot.tabLabel", Prop: "max-width", Official: TabLabelMaxWidth, Impl: TabLabelMaxWidth, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:314"},

	// body / scrollBody / composerSeat
	{Node: "ConversationRoot.scrollBody", Prop: "margin-right", Official: BodyScrollMarginRight, Impl: BodyScrollMarginRight, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:493"},
	{Node: "ConversationRoot.composerSeat", Prop: "z-index (active)", Official: ComposerSeatZ, Impl: ComposerSeatZ, Unit: UnitZIndex, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:531"},
	{Node: "ConversationRoot.composerSeat", Prop: "top fade band", Official: ComposerSeatFade, Impl: ComposerSeatFade, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:534-544"},
	{Node: "ConversationRoot.composerStack", Prop: "gap", Official: ComposerStackGap, Impl: ComposerStackGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:422-427"},
	{Node: "ConversationRoot.composerSeat", Prop: "height (resting)", Official: ComposerSeatHeight, Impl: ComposerSeatHeight, Unit: UnitPx, Status: GeoDerived,
		Anchor:  "ui-conversation/src/client/skeleton/ConversationRoot.module.css:430-442",
		Formula: "内容自适应（官方 trajectory 的 composer-height 默认 152 只是 clearance 兜底）"},
	{Node: "ConversationRoot.body", Prop: "content width", Official: ContentWidthMin, Impl: ContentWidthMin, Unit: UnitPx, Status: GeoDerived,
		Anchor:  "ui-conversation/src/client/skeleton/ConversationRoot.module.css:468-471",
		Formula: "clamp(680px, 主列宽 × .64, 920px)"},
	{Node: "ConversationRoot.body", Prop: "content width ratio", Official: ContentWidthRatio, Impl: ContentWidthRatio, Unit: UnitRatio, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:470"},
	{Node: "ConversationRoot.body", Prop: "content width max", Official: ContentWidthMax, Impl: ContentWidthMax, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:470"},
	{Node: "ConversationRoot.body", Prop: "composer card max extra", Official: CardMaxExtra, Impl: CardMaxExtra, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ConversationRoot.module.css:472"},

	// InputBar
	{Node: "InputBar.root", Prop: "padding-bottom", Official: ComposerPadBottom, Impl: ComposerPadBottom, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:10"},
	{Node: "InputBar.root", Prop: "side clearance", Official: ComposerSideClearance, Impl: ComposerSideClearance, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:10"},
	{Node: "InputBar.card", Prop: "border-radius (radius-panel)", Official: ComposerCardRadius, Impl: ComposerCardRadius, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:54-66"},
	{Node: "InputBar.card", Prop: "padding-top", Official: ComposerCardPadTop, Impl: ComposerCardPadTop, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:64"},
	{Node: "InputBar.card", Prop: "gap", Official: ComposerCardGap, Impl: ComposerCardGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:61"},
	{Node: "InputBar.input", Prop: "min-height (docked)", Official: ComposerInputMinH, Impl: ComposerInputMinH, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:190"},
	{Node: "InputBar.input", Prop: "min-height (hero)", Official: ComposerInputHeroH, Impl: ComposerInputHeroH, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:186-191"},
	{Node: "InputBar.input", Prop: "padding-top", Official: ComposerInputPadTop, Impl: ComposerInputPadTop, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:191"},
	{Node: "InputBar.input", Prop: "padding-right", Official: ComposerInputPadX, Impl: ComposerInputPadX, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:191"},
	{Node: "InputBar.input", Prop: "padding-left", Official: ComposerInputPadL, Impl: ComposerInputPadL, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:191"},
	{Node: "InputBar.scroll", Prop: "max-height", Official: ComposerTextMaxHeight, Impl: ComposerTextMaxHeight, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:146"},
	{Node: "InputBar.row", Prop: "gap", Official: ComposerRowGap, Impl: ComposerRowGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:278"},
	{Node: "InputBar.row", Prop: "padding-top", Official: ComposerRowPadTop, Impl: ComposerRowPadTop, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:282"},
	{Node: "InputBar.row", Prop: "padding-bottom", Official: ComposerRowPadBot, Impl: ComposerRowPadBot, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:282"},
	{Node: "InputBar.row", Prop: "padding-inline", Official: ComposerRowPadX, Impl: ComposerRowPadX, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:282"},
	{Node: "InputBar.primary", Prop: "size", Official: ComposerPrimarySize, Impl: ComposerPrimarySize, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:402-403"},
	{Node: "InputBar.dock", Prop: "gap", Official: ComposerDockGap, Impl: ComposerDockGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:17"},
	{Node: "InputBar.dock", Prop: "padding-top", Official: ComposerDockPadTop, Impl: ComposerDockPadTop, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:19"},
	{Node: "InputBar.dock", Prop: "min-height (non-hero)", Official: ComposerDockMinH, Impl: ComposerDockMinH, Unit: UnitPx, Status: GeoDerived,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:22-52", Formula: "calc(22px + font-delta-secondary)"},
	// 控件行左端「加号」入口（官方 `.add`，命令/附件入口）。
	{Node: "InputBar.row.add", Prop: "width/height", Official: ComposerAddSize, Impl: ComposerAddSize, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:340-341"},
	{Node: "InputBar.row.add", Prop: "border-radius", Official: ComposerAddRadius, Impl: ComposerAddRadius, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:343"},
	{Node: "InputBar.row.add", Prop: "icon size", Official: ComposerAddIcon, Impl: ComposerAddIcon, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.tsx:517"},
	{Node: "InputBar.row.add", Prop: "disabled opacity", Official: ComposerAddDim, Impl: ComposerAddDim, Unit: UnitRatio, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/InputBar.module.css:356-358"},
	// dock 里的读数胶囊（StatsPills `.pill` / ContextMeter `.trigger`）。
	{Node: "InputBar.dock.pill", Prop: "padding-block", Official: ComposerPillPadY, Impl: ComposerPillPadY, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-chat/src/client/chat/StatsPills.module.css:13-60"},
	{Node: "InputBar.dock.pill", Prop: "padding-inline", Official: ComposerPillPadX, Impl: ComposerPillPadX, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-chat/src/client/chat/StatsPills.module.css:13-60"},
	{Node: "InputBar.dock.pill", Prop: "gap", Official: ComposerPillGap, Impl: ComposerPillGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-chat/src/client/chat/StatsPills.module.css:13-60"},
	{Node: "InputBar.dock.pill", Prop: "font-size", Official: ComposerPillFont, Impl: ComposerPillFont, Unit: UnitPx, Status: GeoDerived,
		Anchor: "ui-chat/src/client/chat/StatsPills.module.css:13-60", Formula: "calc(13px - 1px)"},
	{Node: "InputBar.dock.pill", Prop: "line-height", Official: ComposerPillLineH, Impl: ComposerPillLineH, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-chat/src/client/chat/StatsPills.module.css:13-60"},
	{Node: "InputBar.dock.pill", Prop: "icon size", Official: ComposerPillIconWH, Impl: ComposerPillIconWH, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/ContextMeter.module.css:9-38"},

	// ChatView
	{Node: "ChatView.scroll", Prop: "padding-block", Official: ChatScrollPad, Impl: ChatScrollPad, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-chat/src/client/chat/ChatView.module.css:32"},
	{Node: "ChatView.scroll", Prop: "side padding extra", Official: ChatScrollSideX, Impl: ChatScrollSideX, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-chat/src/client/chat/ChatView.module.css:32"},
	{Node: "ChatView.flow", Prop: "gap", Official: ChatFlowGap, Impl: ChatFlowGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-chat/src/client/chat/ChatView.module.css:102-110"},
	{Node: "ChatView.flow", Prop: "response group gap", Official: ChatResponseGap, Impl: ChatResponseGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-chat/src/client/chat/ChatView.module.css:102-110"},
	{Node: "ChatView.flow", Prop: "turn-process gap", Official: ChatTurnProcGap, Impl: ChatTurnProcGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-chat/src/client/chat/ChatView.module.css:102-110"},

	// HeroShell
	{Node: "HeroShell.root", Prop: "padding-inline", Official: HeroPadX, Impl: HeroPadX, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/HeroShell.module.css:11"},
	{Node: "HeroShell.stack", Prop: "gap", Official: HeroStackGap, Impl: HeroStackGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/HeroShell.module.css:20"},
	{Node: "HeroShell.headline", Prop: "font-size", Official: HeroHeadlineFontSize, Impl: HeroHeadlineFontSize, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/HeroShell.module.css:41"},
	{Node: "HeroShell.headline", Prop: "line-height", Official: HeroHeadlineLineH, Impl: HeroHeadlineLineH, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/HeroShell.module.css:42"},
	{Node: "HeroShell.headline", Prop: "column-gap", Official: HeroHeadlineGapCol, Impl: HeroHeadlineGapCol, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/HeroShell.module.css:35"},
	{Node: "HeroShell.headline", Prop: "row-gap", Official: HeroHeadlineGapRow, Impl: HeroHeadlineGapRow, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/HeroShell.module.css:38"},
	{Node: "HeroShell.titleGroup", Prop: "column-gap", Official: HeroTitleGapCol, Impl: HeroTitleGapCol, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/HeroShell.module.css:57"},
	{Node: "HeroShell.titleGroup", Prop: "row-gap", Official: HeroTitleGapRow, Impl: HeroTitleGapRow, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/HeroShell.module.css:58"},
	{Node: "HeroShell.fish", Prop: "width", Official: HeroFishSize, Impl: HeroFishSize, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/HeroShell.module.css:90"},
	{Node: "HeroShell.previewBadge", Prop: "font-size", Official: HeroBadgeFontSize, Impl: HeroBadgeFontSize, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/HeroShell.module.css:72"},
	{Node: "HeroShell.previewBadge", Prop: "line-height", Official: HeroBadgeLineH, Impl: HeroBadgeLineH, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/HeroShell.module.css:73"},
	{Node: "HeroShell.previewBadge", Prop: "padding-inline", Official: HeroBadgePadX, Impl: HeroBadgePadX, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/HeroShell.module.css:65"},
	{Node: "HeroShell.workspace", Prop: "min-height", Official: HeroWorkspaceMinH, Impl: HeroWorkspaceMinH, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/HeroShell.module.css:157"},
	{Node: "HeroShell.workspace", Prop: "max-width", Official: HeroWorkspaceMaxW, Impl: HeroWorkspaceMaxW, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/HeroShell.module.css:156"},
	{Node: "HeroShell.workspace", Prop: "gap", Official: HeroWorkspaceGap, Impl: HeroWorkspaceGap, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-conversation/src/client/skeleton/HeroShell.module.css:155"},

	// SidebarRight（右栏）
	{Node: "SidebarRight.panel", Prop: "dock layer", Official: RightbarPaneZ, Impl: RightbarPaneZ, Unit: UnitZIndex, Status: GeoAligned,
		Anchor: "ui-sidebar-right/src/client/shell/SidebarRight.module.css:32"},
	{Node: "SidebarRight.panel", Prop: "float layer", Official: RightbarFloatZ, Impl: RightbarFloatZ, Unit: UnitZIndex, Status: GeoAligned,
		Anchor: "ui-sidebar-right/src/client/shell/SidebarRight.module.css:33"},
	{Node: "SidebarRight.panel", Prop: "fullscreen dock layer", Official: RightbarFullscreenZ, Impl: RightbarFullscreenZ, Unit: UnitZIndex, Status: GeoAligned,
		Anchor: "ui-sidebar-right/src/client/shell/SidebarRight.module.css:82"},
	{Node: "SidebarRight.iconButton", Prop: "size", Official: RightbarIconSize, Impl: RightbarIconSize, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar-right/src/client/shell/SidebarRight.module.css:139-140"},
	{Node: "SidebarRight.iconButton", Prop: "padding", Official: RightbarIconPad, Impl: RightbarIconPad, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar-right/src/client/shell/SidebarRight.module.css:141"},
	{Node: "SidebarRight.iconButton", Prop: "svg size", Official: RightbarIconSvg, Impl: RightbarIconSvg, Unit: UnitPx, Status: GeoAligned,
		Anchor: "ui-sidebar-right/src/client/shell/SidebarRight.module.css:134-163"},
	{Node: "SidebarRight.flash", Prop: "ring mix", Official: FlashRing, Impl: FlashRing, Unit: UnitPct, Status: GeoAligned,
		Anchor: "ui-sidebar-right/src/client/shell/SidebarRight.module.css:220"},
	{Node: "SidebarRight.flash", Prop: "near mix", Official: FlashNear, Impl: FlashNear, Unit: UnitPct, Status: GeoAligned,
		Anchor: "ui-sidebar-right/src/client/shell/SidebarRight.module.css:221"},
	{Node: "SidebarRight.flash", Prop: "far mix", Official: FlashFar, Impl: FlashFar, Unit: UnitPct, Status: GeoAligned,
		Anchor: "ui-sidebar-right/src/client/shell/SidebarRight.module.css:222"},
	{Node: "SidebarRight.flash", Prop: "overlay z-index", Official: FlashZ, Impl: FlashZ, Unit: UnitZIndex, Status: GeoAligned,
		Anchor: "ui-sidebar-right/src/client/shell/SidebarRight.module.css:236"},

	// 遗留桥（deviation 行：只允许因旧插件未迁移）
	{Node: "AppFrame.centerCol", Prop: "content inset (legacy bridge)", Official: 0, Impl: LegacyColumnInset, Unit: UnitPx, Status: GeoDeviation,
		Anchor: "ui-layout/src/client/AppFrame.module.css:68-75",
		Reason: "官方中栏是裸列（inset 归占用方）；旧 dstopbar/dsconversation/dscomposer 仍按 dsshell 的 12px 栏内边距排版，直接改 0 会让 composer 净边距 16→4。等四个插件同一次迁到官方槽位后删掉这一行。"},
	{Node: "AppFrame.rightbarCol", Prop: "content inset (legacy bridge)", Official: 0, Impl: LegacyColumnInset, Unit: UnitPx, Status: GeoDeviation,
		Anchor: "ui-layout/src/client/AppFrame.module.css:264-270",
		Reason: "同上：旧 dsrightbar 没有自己的面板框，12px 是它当前的排版基准。"},
	{Node: "SidebarRoot.root", Prop: "padding-inline (legacy bridge)", Official: SidebarRootPadX, Impl: LegacySidebarPadX, Unit: UnitPx, Status: GeoDeviation,
		Anchor: "ui-sidebar/src/client/SidebarRoot.module.css:10,17",
		Reason: "rc.2 官方是 8px；旧 dssidebar 的子节点按 12px 排版。桥模式沿用 12，官方子槽模式用 8。"},
}

// OfficialGeometry 返回几何表的只读副本（测试与报告都从这一份取数据）。
func OfficialGeometry() []GeoRow {
	return append([]GeoRow(nil), officialGeometry...)
}

// contentWidth 按官方 clamp(680px, 主列宽 × .64, 920px) 求正文宽。
func contentWidth(column float32) float32 {
	w := column * ContentWidthRatio
	if w < ContentWidthMin {
		w = ContentWidthMin
	}
	if w > ContentWidthMax {
		w = ContentWidthMax
	}
	return w
}

// composerCardWidth 是官方 `--dsh-composer-card-max-width` = 正文宽 + 32。
func composerCardWidth(column float32) float32 {
	return contentWidth(column) + CardMaxExtra
}

// 复用官方同名槽常量是「不新造平行体系」的一半；这里保留一个显式引用，
// 让编译失败先于「两套槽名悄悄漂移」发生。
var _ pluginapi.Slot = pluginapi.SlotRightbar
