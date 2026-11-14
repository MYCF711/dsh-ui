// official_geometry.go —— 设置面的**唯一几何常量表**（rc.2 口径）。
//
// 规矩与首屏骨架（internal/uikernel/official_geometry.go）同源：
//
//	① 结构树里每个 gap / padding / min-max 宽高只有一个出处：本文件的常量。
//	   绘制代码（official_panel.go）只准引用常量，不许再写魔法数。
//	② 「官方值」与「实现值」写在**同一张表**里（OfficialSettingsGeometry 的每一
//	   行），测试逐行核对：aligned 行实现值必须等于官方值；derived 行必须写清
//	   官方原文公式；deviation 行必须写清原因。
//	③ 官方值全部从 rc.2 随包 `src/**` 的 CSS Module 读出来，锚点一律
//	   `<pkg>/src/….module.css:line`，不是从截图猜的、也不是 rc.1 口径。
//
// 证据文件（rc.2 解包树）：
//
//	ui-settings-general/src/client/SettingsRoot.module.css（342 行，类名明文）
//	ui-chat/src/client/settings/PreferenceRow.module.css
//	ui-conversation/src/client/settings/EnterBehaviorRow.module.css
//	ui-permission-presets/src/client/PermissionRow.module.css
//	ui-settings-general/src/client/DeveloperToolsRow.module.css
//	ui-settings-session-log/src/client/UploadRow.module.css
//	ui-shortcuts/src/client/Reference.module.css
//	ui-settings-general/src/client/CurrentVersionRow.module.css
//	ui-theme/src/client/FontSizeRow.module.css / FontFamilyRow.module.css
//	ui-settings-models/src/client/ModelsSection.module.css（1003 行）
//	ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css
package dssettings

// GeoUnit 是值的量纲。像素之外只有比例/百分比/派生公式三种，
// 混着写会让「800 是 800px 还是 800%」这种问题变成口头约定。
type GeoUnit string

const (
	UnitPx     GeoUnit = "px"
	UnitRatio  GeoUnit = "ratio"
	UnitZIndex GeoUnit = "z-index"
)

// GeoStatus 说明一行「官方值 ↔ 实现值」的关系。
type GeoStatus string

const (
	// GeoAligned 实现值与官方值一致（本任务要求：结构容器全部 aligned）。
	GeoAligned GeoStatus = "aligned"
	// GeoDerived 官方是 calc/clamp/内容自适应，没有单一数值；实现值是实现侧的
	// 等价取值，必须同时写 Formula。
	GeoDerived GeoStatus = "derived"
	// GeoDeviation 实现值与官方值**不同**，必须写 Reason，且只能是为了兼容
	// 尚未迁移的旧部件（遗留桥）。
	GeoDeviation GeoStatus = "deviation"
)

// GeoRow 是一行几何事实。
type GeoRow struct {
	Node     string    // 官方节点（SettingsPanel.panel / SettingsRoot.navCell …）
	Prop     string    // 属性（gap / padding-inline / min-height …）
	Official float32   // 官方值；derived 行填实现侧等价取值
	Impl     float32   // 本仓实现值（同一行，不另表）
	Unit     GeoUnit   // 量纲
	Status   GeoStatus // aligned / derived / deviation
	Anchor   string    // 官方 rc.2 源码锚点
	Formula  string    // derived 行的官方原文（calc/clamp/内容自适应）
	Reason   string    // deviation 行必须写原因
}

// ── 设置卡片（.panel）─────────────────────────────────────────────────
const (
	// PanelWidth 是卡片宽度（`.panel { width: 800px }`，
	// SettingsRoot.module.css:104）。
	PanelWidth float32 = 800
	// PanelMaxHeight 是卡片高度上限（`height: min(800px, …)`，:105）。
	PanelMaxHeight float32 = 800
	// PanelViewportMargin 是上下各留的空气（`max(24px, …)`，:105）。
	PanelViewportMargin float32 = 24
	// PanelMaxWidthInset 是窗口变窄时左右各扣的边距
	// （`max-width: calc(100vw - 48px)`，:106）。
	PanelMaxWidthInset float32 = 48
	// PanelOverlayTop 是 frame 发布的浮层上沿（`--dsh-frame-overlay-top` 的
	// 默认值 24，:105）。
	PanelOverlayTop float32 = 24
)

// ── 四区网格（rc.2 D1：CSS Grid 2×2）────────────────────────────────
const (
	// GridNavWidth 是左导航列宽（`grid-template-columns: 188px minmax(0, 1fr)`，
	// SettingsRoot.module.css:99）。
	GridNavWidth float32 = 188
	// GridHeaderRowHeight 是第一行高度（`auto`，由 navTitle 与 header 共同
	// 撑到 54；两者都是 54，见 :141 / :204）。
	GridHeaderRowHeight float32 = 54
)

// ── 左导航 ────────────────────────────────────────────────────────────
const (
	// NavPadTop / NavPadX 是 `.nav` 的 padding（rc.2 D2：22px → **10px** 上边，
	// SettingsRoot.module.css:132）。
	NavPadTop float32 = 10
	NavPadX   float32 = 12

	// NavTitlePadTop/Right/Bottom/Left 是 `.navTitle` 的 padding
	// （22px 12px 8px 24px，:141）。
	NavTitlePadTop    float32 = 22
	NavTitlePadRight  float32 = 12
	NavTitlePadBottom float32 = 8
	NavTitlePadLeft   float32 = 24
	// NavTitleFontSize / NavTitleLineHeight / NavTitleWeight 是标题排版（16/24、500，:142-144）。
	NavTitleFontSize   float32 = 16
	NavTitleLineHeight float32 = 24
	NavTitleWeight     float32 = 500

	// NavListGap 是导航列表纵向间距（`gap: 4px`，:151）。
	NavListGap float32 = 4

	// NavCellHeight 是导航行高（`height: 40px`，:161）。
	NavCellHeight       float32 = 40
	NavCellPadTop       float32 = 9
	NavCellPadRight     float32 = 16
	NavCellPadBottom    float32 = 9
	NavCellPadLeft      float32 = 12
	NavCellGap          float32 = 8
	NavCellFontSize     float32 = 14
	NavCellLineHeight   float32 = 22
	NavCellFontWeight   float32 = 400
	NavCellIconSize     float32 = 16
	NavCellIconFallback float32 = 16
)

// ── 右列头 ────────────────────────────────────────────────────────────
const (
	// HeaderHeight 是右列头高（`height: 54px`，:204）。
	HeaderHeight     float32 = 54
	HeaderPadTop     float32 = 20
	HeaderPadRight   float32 = 14
	HeaderPadBottom  float32 = 8
	HeaderPadLeft    float32 = 10
	HeaderGap        float32 = 8
	HeaderActionsGap float32 = 8

	// CloseSize 是关闭按钮边长（`width/height: 28px`，:225-226）。
	CloseSize     float32 = 28
	CloseIconSize float32 = 14
)

// ── section 滚动体（.options）────────────────────────────────────────
const (
	OptionsPadTop    float32 = 0
	OptionsPadRight  float32 = 24
	OptionsPadBottom float32 = 24
	OptionsPadLeft   float32 = 24
)

// ── overlay / mask ───────────────────────────────────────────────────
const (
	// OverlayZIndex 是整层 z-index（`.overlay { z-index: 1000 }`，:70）。
	OverlayZIndex float32 = 1000
	// MaskInsetTop 是遮罩上沿（`inset: var(--dsh-frame-chrome-top, 0px) 0 0`，:79）。
	MaskInsetTop float32 = 0
)

// ── 启动器触发器（sidebar.settings）──────────────────────────────────
const (
	TriggerHeight        float32 = 42
	TriggerRowMarginY    float32 = 4
	TriggerRailSize      float32 = 36
	TriggerRailMarginTop float32 = 8
	TriggerRailMarginBot float32 = 10
	TriggerFontSize      float32 = 14
	TriggerLineHeight    float32 = 22
	// `.trigger{padding:0 10px 0 8px}`（:25）：左 8 右 10，**不对称**。
	TriggerPadLeft  float32 = 8
	TriggerPadRight float32 = 10
	// `.trigger{gap:8px}`（:21）。
	TriggerGap float32 = 8
	// 宽态图标 16（`IconSettingsOutlineMedium` 的 `wide ? 16 : 18`，
	// `chrome.tsx:22-29`）。
	TriggerIconSize float32 = 16
)

// ── general 行（§4.1 共享行样式）─────────────────────────────────────
const (
	// RowPadY 是选择器行的上下内边距（`padding: 16px 0`，PreferenceRow.module.css:5）。
	RowPadY float32 = 16
	// RowGap 是行内主副文案间距（`gap: 8`，:6）。
	RowGap float32 = 8
	// RowBorderWidth 是行底边（`.5px --dsw-alias-border-l2`，:7）。
	RowBorderWidth float32 = 0.5

	// RowTextGap 是标题与说明之间（`gap: 4`，:13）。
	RowTextGap float32 = 4
	// RowTextPadRight 是文本区右内边距（`padding-right: 48px`，:14）。
	RowTextPadRight float32 = 48

	RowTitleFontSize   float32 = 14
	RowTitleLineHeight float32 = 22
	RowDescFontSize    float32 = 12
	RowDescLineHeight  float32 = 18

	// SelectorHeight 是选择器 pill 高（`height: 36px`，:41）。
	SelectorHeight  float32 = 36
	SelectorPadX    float32 = 14
	SelectorGap     float32 = 12
	SelectorChevron float32 = 14

	// DevToolsRowGap 是开发者工具 / 上传 / 桌面通道行的间距（`gap: 24`，
	// DeveloperToolsRow.module.css:5）。
	DevToolsRowGap          float32 = 24
	DevToolsTitleLineHeight float32 = 20

	// ShortcutRowMinHeight 是快捷键行最小高（`min-height: 60px`，Reference.module.css:9）。
	ShortcutRowMinHeight   float32 = 60
	ShortcutButtonHeight   float32 = 36
	ShortcutButtonMinWidth float32 = 90
	ShortcutButtonMaxWidth float32 = 280
	ShortcutButtonPadX     float32 = 14
	ShortcutDescFontSize   float32 = 12
	ShortcutDescLineHeight float32 = 18

	// VersionRowPadY 是版本行内边距（CurrentVersionRow.module.css:2）。
	VersionRowPadY       float32 = 16
	VersionRowFontSize   float32 = 14
	VersionRowLineHeight float32 = 22
)

// ── font-size 展开 / 字体族 ──────────────────────────────────────────
const (
	// FontSizeMin/Max/Default 是正文字号范围（theme-settings.ts:24-30）。
	FontSizeMin     float32 = 10
	FontSizeMax     float32 = 22
	FontSizeDefault float32 = 14
	// CodeFontSizeMin/Max/Default 是代码字号（:57-61 的 FONT_SIZE_SPECS）。
	CodeFontSizeMin     float32 = 10
	CodeFontSizeMax     float32 = 16
	CodeFontSizeDefault float32 = 11
	// TerminalFontSizeMin/Max/Default 是终端字号。
	TerminalFontSizeMin     float32 = 10
	TerminalFontSizeMax     float32 = 20
	TerminalFontSizeDefault float32 = 13
	// FontFamilyFlexShrink/Basis 是字体输入框（`flex: 0 1 240px`，rc.2 D7，
	// FontFamilyRow.module.css:3-6）。
	FontFamilyFlexShrink float32 = 1
	FontFamilyFlexBasis  float32 = 240
	// FontSizeStackedBreakpoint 是 `.stacked` 的窄容器断点
	// （`@media (width <= 767px)`，FontSizeRow.module.css:28-37）。
	FontSizeStackedBreakpoint float32 = 767
)

// ── 右页（models / plugins / 删除对话框）─────────────────────────────
const (
	// ModelsSectionMaxWidth 是 models 页最大内容宽（`max-width: 720px`，
	// ModelsSection.module.css:6）。
	ModelsSectionMaxWidth float32 = 720
	// ModelsSectionGap 是页内纵向间距（`gap: 12px`，同处）。
	ModelsSectionGap     float32 = 12
	ModelsTitleFontSize  float32 = 16
	ModelsTitleLine      float32 = 24
	ModelsIntroFontSize  float32 = 14
	ModelsIntroLine      float32 = 22
	ModelsRowsGap        float32 = 8
	ModelsRowsMarginTop  float32 = 12
	ModelsRowCardPadY    float32 = 12
	ModelsRowCardPadX    float32 = 14
	ModelsRowCardGap     float32 = 12
	ModelsRowHeadGap     float32 = 10
	ModelsRowNameSize    float32 = 14
	ModelsRowNameLine    float32 = 22
	ModelsRowTagSize     float32 = 11
	ModelsRowTagLine     float32 = 16
	ModelsRowTagPadX     float32 = 6
	ModelsRowTagPadY     float32 = 1
	ModelsCredentialDot  float32 = 8
	ModelsEditorPadY     float32 = 14
	ModelsEditorPadX     float32 = 16
	ModelsEditorGap      float32 = 14
	ModelsFieldGap       float32 = 6
	ModelsInputMinHeight float32 = 36
	ModelsButtonHeight   float32 = 36
	ModelsButtonPadX     float32 = 14
	ModelsRowButtonH     float32 = 28
	ModelsRowButtonPadX  float32 = 10

	// DeleteDialogMaxWidth 是删除确认框宽（rc.2 D9：300 → `min(480px, 100%)`，
	// ModelsSection.module.css:908-910）。
	DeleteDialogMaxWidth float32 = 480

	// ── 内置插件页（rc.2 两段：ui-settings-plugins 外壳 + ui-settings-plugin-inventory 清单页）──
	//
	// 2026-11-14 修了 4 行**错的**：原先 groupTitle 写 13/20（官方 `.groupTitle` 是
	// 14px/22px）、group gap 写 8（`.group{gap:10px}`）、单列断点写 640（官方是
	// `@container (max-width: 520px)`）、card padding 的锚点指到 `.card`（`padding`
	// 其实在 `.cardContent{12px 14px}`）。四处都按 rc.2 原文重锚了。
	//
	// PluginsSectionMaxWidth 是清单页最大内容宽
	// （`max-width: 760px`，PluginInventorySettingsTab.module.css:2-10）。
	PluginsSectionMaxWidth float32 = 760
	// PluginsSectionGap 是清单页纵向间距（`gap: 14px`，同处）。
	PluginsSectionGap float32 = 14
	// PluginsSectionOuterGap 是**外壳**页的纵向间距（`.section{gap:12px}`，
	// PluginsSettingsSection.module.css:3-9）。
	PluginsSectionOuterGap float32 = 12
	// PluginsHeadingFontSize / PluginsHeadingWeight 是外壳标题（18px / 600，
	// `.heading`，同文件 :11-15）。
	PluginsHeadingFontSize float32 = 18
	PluginsHeadingWeight   float32 = 600
	// PluginsIntroFontSize 是外壳简介（`font-size: 13px`，`.intro`，:17-21）。
	PluginsIntroFontSize float32 = 13
	// ── 外壳的 tab 条（只有多 tab 时才画，官方 PluginsSettingsSection.module.css:23-70）──
	//
	// PluginsTabsGap / MarginTop 是 `.tabs{gap:22px;margin-top:2px}`（:23-29）；
	// PluginsTabPadTop/Bottom/X 是 `.tab{padding:7px 1px 9px}`（:31-41）；
	// PluginsTabFontSize/Ln 是 `.tab{font-size:13px;line-height:20px}`（同处）；
	// PluginsTabUnderline 是选中态 `::after{height:2px}`（:53-63）。
	PluginsTabsGap      float32 = 22
	PluginsTabsMarginT  float32 = 2
	PluginsTabPadTop    float32 = 7
	PluginsTabPadBt     float32 = 9
	PluginsTabPadX      float32 = 1
	PluginsTabFontSize  float32 = 13
	PluginsTabLn        float32 = 20
	PluginsTabUnderline float32 = 2

	// PluginsCatalogGap 是搜索框与分段之间的间距（`.catalog{gap:12px}`，清单页 :47-51）。
	PluginsCatalogGap float32 = 12

	// PluginsSearchHeight / PadL / PadR / FontSize 是搜索框（`.search input`，
	// 清单页 :67-78：height 36、padding 0 34 0 36、font-size 13）。
	PluginsSearchHeight   float32 = 36
	PluginsSearchPadL     float32 = 36
	PluginsSearchPadR     float32 = 34
	PluginsSearchFontSize float32 = 13

	// PluginsCardsGap 是卡片网格间距（`.cards{gap:10px}`，:89-96）。
	PluginsCardsGap float32 = 10
	// PluginsCardColumns 是网格列数（`grid-template-columns: repeat(2, minmax(0,1fr))`，同处）。
	PluginsCardColumns float32 = 2
	// PluginsCardSingleCol 是**容器**查询的单列断点（`@container plugin-inventory
	// (max-width: 520px)`，:501-505）——不是视口断点。
	PluginsCardSingleCol float32 = 520
	// PluginsCardPad / PluginsCardPadX 是卡片内容内边距（`.cardContent{padding:12px 14px}`，:124-140）。
	PluginsCardPad  float32 = 12
	PluginsCardPadX float32 = 14
	// PluginsCardMinHeight 是卡内容最小高（`.cardContent{min-height:52px}`，同处）。
	PluginsCardMinHeight float32 = 52
	// PluginsCardContentGap 是卡内三行之间的间距（`.cardContent{gap:2px}`，同处）。
	PluginsCardContentGap float32 = 2
	// PluginsCardMainRowGap 是标题行左右两块的间距（`.cardMainRow{gap:12px}`，:163-169）。
	PluginsCardMainRowGap float32 = 12
	// PluginsCardTitleFontSize / Ln 是卡片标题（`.cardTitle` 14px/20px，:178-184）。
	PluginsCardTitleFontSize float32 = 14
	PluginsCardTitleLn       float32 = 20
	// PluginsCardDescFontSize / Ln 是卡片简介（`.cardDescription` 12px/18px，:186-196）。
	PluginsCardDescFontSize float32 = 12
	PluginsCardDescLn       float32 = 18
	// PluginsCardMetaPadTop 是角标行上边距（`.cardMeta{padding-top:6px}`，:204-208）。
	PluginsCardMetaPadTop float32 = 6
	// PluginsCardChipFontSize / Ln / PadX / PadY 是 entry id 角标
	// （`.cardIdentity` 12px/18px、`padding:1px 6px`，:213-228）。
	PluginsCardChipFontSize float32 = 12
	PluginsCardChipLn       float32 = 18
	PluginsCardChipPadX     float32 = 6
	PluginsCardChipPadY     float32 = 1
	// PluginsCardTrailingGap 是行尾状态组内部间距（`.cardTrailing{gap:8px}`，:230-236）。
	PluginsCardTrailingGap float32 = 8
	// PluginsChevronSize 是 chevron 图标尺寸（TSX `size={12}`，:164）。
	PluginsChevronSize float32 = 12
	// PluginsTagFontSize / Ln / PadX / PadY 是状态标签（官方 `Tag` 组件；
	// 本仓没有该 primitive，按同一档排版画：11px/16px、左右 6、上下 1）。
	PluginsTagFontSize float32 = 11
	PluginsTagLn       float32 = 16
	PluginsTagPadX     float32 = 6
	PluginsTagPadY     float32 = 1
	// PluginsDotSize 是状态点直径（官方 `StateDot`；本仓按 8px 的既有档画）。
	PluginsDotSize float32 = 8

	// PluginsGroupGap 是分段内纵向间距（`.group{gap:10px}`，:245-249）。
	PluginsGroupGap float32 = 10
	// PluginsGroupTitleRowGap / MinHeight 是段标题行（`.groupTitleRow{gap:8px;
	// min-height:36px}`，:253-258）。
	PluginsGroupTitleRowGap float32 = 8
	PluginsGroupTitleRowMin float32 = 36
	// PluginsGroupTitle / Ln 是段标题排版（`.groupTitle{font-size:14px;line-height:22px}`，:296-301）。
	PluginsGroupTitle   float32 = 14
	PluginsGroupTitleLn float32 = 22
	// PluginsGroupSubFontSize / Ln / MarginTop / MarginLeft 是段副标题
	// （`.groupSub{margin:-6px 0 0 20px;font-size:12px;line-height:18px}`，:303-312）。
	PluginsGroupSubFontSize float32 = 12
	PluginsGroupSubLn       float32 = 18
	PluginsGroupSubMarginL  float32 = 20
	PluginsGroupSubMarginT  float32 = -6
	// PluginsGroupDividerPadTop 是段与段之间的分隔（`.group + .group{border-top:.5px;
	// padding-top:14px}`，:260-263）。
	PluginsGroupDividerPadTop float32 = 14
	// PluginsGroupBodyGap 是段体纵向间距（`.groupBody{gap:10px}`，:359-363）。
	PluginsGroupBodyGap float32 = 10

	// PluginsDetailsPadTop / PadBottom / PadX 是展开详情盒（`.cardDetails{padding:10px 14px 12px}`，:437-441）。
	PluginsDetailsPadTop    float32 = 10
	PluginsDetailsPadBottom float32 = 12
	PluginsDetailsPadX      float32 = 14
	// PluginsDetailsLabelWidth / ColGap / RowGap / MarginTop 是 `dt`/`dd` 网格
	// （`.details{grid-template-columns:76px minmax(0,1fr);gap:6px 10px;margin:8px 0 0}`，:449-454）。
	PluginsDetailsLabelWidth float32 = 76
	PluginsDetailsColGap     float32 = 10
	PluginsDetailsRowGap     float32 = 6
	PluginsDetailsMarginTop  float32 = 8
	// PluginsDetailsDtFontSize / Ln 与 PluginsDetailsDdFontSize / Ln 是详情两列排版
	// （`.details dt{font-size:11px;line-height:17px}` / `dd{font-size:12px;line-height:17px}`，:460-473）。
	PluginsDetailsDtFontSize float32 = 11
	PluginsDetailsDtLn       float32 = 17
	PluginsDetailsDdFontSize float32 = 12
	PluginsDetailsDdLn       float32 = 17

	// PluginsSearchIconLeft 是搜索图标左内缩（`.search > svg{left:12px}`，:61-65）。
	PluginsSearchIconLeft float32 = 12
	// PluginsRetryBorderWidth 是搜索框与「重试」按钮的 0.5px 描边
	// （`.search input{border:0.5px}` :67-78；`.failure button{border:0.5px}` :37-45）。
	PluginsRetryBorderWidth float32 = 0.5
	// PluginsRetryPadX / PadY 是「重试」按钮内边距（`.failure button{padding:4px 10px}`，:37-45）。
	PluginsRetryPadX float32 = 10
	PluginsRetryPadY float32 = 4
	// PluginsStatusFontSize / Ln 是状态/空态文案（`.status{font-size:13px;line-height:20px}`，:17-22）。
	PluginsStatusFontSize float32 = 13
	PluginsStatusLn       float32 = 20
	// PluginsBrokenPadX / PadY 与 PluginsBrokenFontSize 是错误条与元信息错误注记
	// （`.brokenNote{padding:8px 10px;font-size:12px;line-height:18px}`，:366-376）。
	PluginsBrokenPadX     float32 = 10
	PluginsBrokenPadY     float32 = 8
	PluginsBrokenFontSize float32 = 12
	PluginsBrokenLn       float32 = 18
)

// ── 设置行的第 3 格：桌面更新指示（`DesktopUpdateIndicator.module.css:1-34`）──
//
// 逐条：`.indicator{height:28px;padding:0 8px;column-gap:4px;
// border:1px solid color-mix(brand 20%,transparent); border-radius:radius-sm;
// background:color-mix(brand 8%,transparent); font:12px/18px 500}` +
// `.icon{width:14px;height:14px}`。
const (
	// UpdateIndicatorHeight 是 `.indicator{height:28px}`。
	UpdateIndicatorHeight float32 = 28
	// UpdateIndicatorPadX 是 `.indicator{padding:0 8px}`（用定宽占位盒表达，见绘制）。
	UpdateIndicatorPadX float32 = 8
	// UpdateIndicatorGap 是 `.indicator{column-gap:4px}`（图标与文字之间）。
	UpdateIndicatorGap float32 = 4
	// UpdateIndicatorIconSize 是 `.icon{width:14px;height:14px}`（svg 也是 14）。
	UpdateIndicatorIconSize float32 = 14
	// UpdateIndicatorBorder 是 `.indicator{border:1px solid …}`。
	UpdateIndicatorBorder float32 = 1
	// UpdateIndicatorBorderTint 是描边的 `color-mix(… 20%, transparent)`。
	UpdateIndicatorBorderTint float32 = 0.2
	// UpdateIndicatorBgTint 是底的 `color-mix(… 8%, transparent)`。
	UpdateIndicatorBgTint float32 = 0.08
	// UpdateIndicatorFontSize 是 `.indicator{font-size:12px}`。
	UpdateIndicatorFontSize float32 = 12
	// UpdateIndicatorLineHeight 是 `.indicator{line-height:18px}`。
	UpdateIndicatorLineHeight float32 = 18
	// UpdateIndicatorSpinPeriodMS 是 `.spinner{animation:spin 1s linear infinite}`
	// 的周期（毫秒；与连接指示的 spinner 1.5s 不是同一条，别混用）。
	UpdateIndicatorSpinPeriodMS float32 = 1000
	// UpdateErrorDotSize 是 `.errorDot{width:6px;height:6px;border-radius:50%}`。
	//
	// 它是官方 error 相里那枚 glyph：长在 14×14 的 `.icon`（grid + place-items:center）
	// 里，颜色是 `background:currentColor`（= `.indicator` 的 brand 色）。
	// **不是叉、不是感叹号**——照官方原样画一个实心圆点。
	UpdateErrorDotSize float32 = 6
)

// ── 窄窗口（官方 <768px 全屏 + 横向 rail）───────────────────────────
const (
	// NarrowViewportBreakpoint 是官方窄窗口断点（`@media (width <= 767px)`）。
	//
	// **未实现**：官方在窄窗口把 rail 改成横向条、panel 铺满视口；本仓骨架只做
	// 桌面布局（见报告「未实现项」）。常量留着是为了让「知道有这条规则」可被测试
	// 断言，而不是散落在注释里。
	NarrowViewportBreakpoint float32 = 767
)

// OfficialSettingsGeometry 是设置面全部几何事实（官方值 ↔ 实现值）。
var OfficialSettingsGeometry = []GeoRow{
	// ── overlay / mask ──
	{Node: "SettingsPanel.overlay", Prop: "inset", Official: 0, Impl: 0, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:67-74"},
	{Node: "SettingsPanel.overlay", Prop: "z-index", Official: 1000, Impl: OverlayZIndex, Unit: UnitZIndex, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:70"},
	{Node: "SettingsPanel.mask", Prop: "inset-top", Official: 0, Impl: MaskInsetTop, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:77-82"},

	// ── 卡片 ──
	{Node: "SettingsPanel.panel", Prop: "width", Official: 800, Impl: PanelWidth, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:104"},
	{Node: "SettingsPanel.panel", Prop: "height", Official: 800, Impl: PanelMaxHeight, Unit: UnitPx, Status: GeoDerived, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:105", Formula: "min(800px, calc(100vh - 2 * max(24px, var(--dsh-frame-overlay-top, 24px))))"},
	{Node: "SettingsPanel.panel", Prop: "max-width", Official: 48, Impl: PanelMaxWidthInset, Unit: UnitPx, Status: GeoDerived, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:106", Formula: "calc(100vw - 48px)"},

	// ── 四区网格（D1）──
	{Node: "SettingsPanel.panel", Prop: "grid-template-columns[0]", Official: 188, Impl: GridNavWidth, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:99"},
	{Node: "SettingsPanel.panel", Prop: "grid-template-columns[1]", Official: 0, Impl: 0, Unit: UnitPx, Status: GeoDerived, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:99", Formula: "minmax(0, 1fr)"},
	{Node: "SettingsPanel.panel", Prop: "grid-template-rows[0]", Official: 54, Impl: GridHeaderRowHeight, Unit: UnitPx, Status: GeoDerived, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:100-103", Formula: "auto（navTitle 与 header 都是 54，取内容高）"},
	{Node: "SettingsPanel.panel", Prop: "grid-template-rows[1]", Official: 0, Impl: 0, Unit: UnitPx, Status: GeoDerived, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:100-103", Formula: "minmax(0, 1fr)"},

	// ── 左导航（D2）──
	{Node: "SettingsRoot.nav", Prop: "padding-top", Official: 10, Impl: NavPadTop, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:132"},
	{Node: "SettingsRoot.nav", Prop: "padding-inline", Official: 12, Impl: NavPadX, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:132"},
	{Node: "SettingsRoot.nav", Prop: "gap", Official: 0, Impl: 0, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:126-137"},
	{Node: "SettingsRoot.navTitle", Prop: "padding-top", Official: 22, Impl: NavTitlePadTop, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:141"},
	{Node: "SettingsRoot.navTitle", Prop: "padding-right", Official: 12, Impl: NavTitlePadRight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:141"},
	{Node: "SettingsRoot.navTitle", Prop: "padding-bottom", Official: 8, Impl: NavTitlePadBottom, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:141"},
	{Node: "SettingsRoot.navTitle", Prop: "padding-left", Official: 24, Impl: NavTitlePadLeft, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:141"},
	{Node: "SettingsRoot.navTitle", Prop: "font-size", Official: 16, Impl: NavTitleFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:142"},
	{Node: "SettingsRoot.navTitle", Prop: "line-height", Official: 24, Impl: NavTitleLineHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:143"},
	{Node: "SettingsRoot.navTitle", Prop: "font-weight", Official: 500, Impl: NavTitleWeight, Unit: UnitRatio, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:144"},
	{Node: "SettingsRoot.navList", Prop: "gap", Official: 4, Impl: NavListGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:151"},
	{Node: "SettingsRoot.navCell", Prop: "height", Official: 40, Impl: NavCellHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:161"},
	{Node: "SettingsRoot.navCell", Prop: "padding-top", Official: 9, Impl: NavCellPadTop, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:162"},
	{Node: "SettingsRoot.navCell", Prop: "padding-right", Official: 16, Impl: NavCellPadRight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:162"},
	{Node: "SettingsRoot.navCell", Prop: "padding-bottom", Official: 9, Impl: NavCellPadBottom, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:162"},
	{Node: "SettingsRoot.navCell", Prop: "padding-left", Official: 12, Impl: NavCellPadLeft, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:162"},
	{Node: "SettingsRoot.navCell", Prop: "gap", Official: 8, Impl: NavCellGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:164"},
	{Node: "SettingsRoot.navCell", Prop: "font-size", Official: 14, Impl: NavCellFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:171"},
	{Node: "SettingsRoot.navCell", Prop: "line-height", Official: 22, Impl: NavCellLineHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:172"},
	{Node: "SettingsRoot.navIcon", Prop: "size", Official: 16, Impl: NavCellIconSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.tsx:35-42"},

	// ── 右列头 ──
	{Node: "SettingsRoot.header", Prop: "height", Official: 54, Impl: HeaderHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:204"},
	{Node: "SettingsRoot.header", Prop: "padding-top", Official: 20, Impl: HeaderPadTop, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:204"},
	{Node: "SettingsRoot.header", Prop: "padding-right", Official: 14, Impl: HeaderPadRight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:204"},
	{Node: "SettingsRoot.header", Prop: "padding-bottom", Official: 8, Impl: HeaderPadBottom, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:204"},
	{Node: "SettingsRoot.header", Prop: "padding-left", Official: 10, Impl: HeaderPadLeft, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:204"},
	{Node: "SettingsRoot.actions", Prop: "gap", Official: 8, Impl: HeaderActionsGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:213"},
	{Node: "SettingsRoot.close", Prop: "width", Official: 28, Impl: CloseSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:225-226"},
	{Node: "SettingsRoot.close", Prop: "icon-size", Official: 14, Impl: CloseIconSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.tsx:103-106"},

	// ── options ──
	{Node: "SettingsRoot.options", Prop: "padding-inline", Official: 24, Impl: OptionsPadLeft, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:245-252"},
	{Node: "SettingsRoot.options", Prop: "padding-bottom", Official: 24, Impl: OptionsPadBottom, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:245-252"},

	// ── 启动器 ──
	{Node: "SettingsRoot.trigger", Prop: "height", Official: 42, Impl: TriggerHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:23"},
	{Node: "SettingsRoot.triggerRow", Prop: "margin-block", Official: 4, Impl: TriggerRowMarginY, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:1-9"},
	{Node: "SettingsRoot.triggerRow", Prop: "gap", Official: 8, Impl: TriggerGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:1-9"},
	{Node: "SettingsRoot.trigger", Prop: "padding-left", Official: 8, Impl: TriggerPadLeft, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:25"},
	{Node: "SettingsRoot.trigger", Prop: "padding-right", Official: 10, Impl: TriggerPadRight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:25"},
	{Node: "SettingsRoot.trigger", Prop: "font-size", Official: 14, Impl: TriggerFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:23-27"},
	{Node: "SettingsRoot.trigger", Prop: "line-height", Official: 22, Impl: TriggerLineHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:23-27"},
	{Node: "SettingsRoot.trigger", Prop: "icon-size", Official: 16, Impl: TriggerIconSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/chrome.tsx:22-29"},
	{Node: "SettingsRoot.trigger.rail", Prop: "width", Official: 36, Impl: TriggerRailSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:11-14"},
	{Node: "SettingsRoot.trigger.rail", Prop: "height", Official: 36, Impl: TriggerRailSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/SettingsRoot.module.css:48-58"},

	// ── general 行 ──
	{Node: "PreferenceRow.row", Prop: "padding-block", Official: 16, Impl: RowPadY, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-chat/src/client/settings/PreferenceRow.module.css:5"},
	{Node: "PreferenceRow.row", Prop: "gap", Official: 8, Impl: RowGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-chat/src/client/settings/PreferenceRow.module.css:6"},
	{Node: "PreferenceRow.row", Prop: "border-bottom-width", Official: 0.5, Impl: RowBorderWidth, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-chat/src/client/settings/PreferenceRow.module.css:7"},
	{Node: "PreferenceRow.rowText", Prop: "gap", Official: 4, Impl: RowTextGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-chat/src/client/settings/PreferenceRow.module.css:13"},
	{Node: "PreferenceRow.rowText", Prop: "padding-right", Official: 48, Impl: RowTextPadRight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-chat/src/client/settings/PreferenceRow.module.css:14"},
	{Node: "PreferenceRow.title", Prop: "font-size", Official: 14, Impl: RowTitleFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-chat/src/client/settings/PreferenceRow.module.css:18-22"},
	{Node: "PreferenceRow.title", Prop: "line-height", Official: 22, Impl: RowTitleLineHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-chat/src/client/settings/PreferenceRow.module.css:18-22"},
	{Node: "PreferenceRow.desc", Prop: "font-size", Official: 12, Impl: RowDescFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-chat/src/client/settings/PreferenceRow.module.css:24-29"},
	{Node: "PreferenceRow.desc", Prop: "line-height", Official: 18, Impl: RowDescLineHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-chat/src/client/settings/PreferenceRow.module.css:24-29"},
	{Node: "PreferenceRow.selector", Prop: "height", Official: 36, Impl: SelectorHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-chat/src/client/settings/PreferenceRow.module.css:41"},
	{Node: "PreferenceRow.selector", Prop: "padding-inline", Official: 14, Impl: SelectorPadX, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-chat/src/client/settings/PreferenceRow.module.css:44"},
	{Node: "PreferenceRow.selector", Prop: "gap", Official: 12, Impl: SelectorGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-chat/src/client/settings/PreferenceRow.module.css:45"},
	{Node: "DeveloperToolsRow.row", Prop: "gap", Official: 24, Impl: DevToolsRowGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/DeveloperToolsRow.module.css:1-20"},
	{Node: "DeveloperToolsRow.title", Prop: "line-height", Official: 20, Impl: DevToolsTitleLineHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/DeveloperToolsRow.module.css:1-20"},
	{Node: "Reference.row", Prop: "min-height", Official: 60, Impl: ShortcutRowMinHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-shortcuts/src/client/Reference.module.css:9"},
	{Node: "Reference.button", Prop: "height", Official: 36, Impl: ShortcutButtonHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-shortcuts/src/client/Reference.module.css:37-39"},
	{Node: "Reference.button", Prop: "min-width", Official: 90, Impl: ShortcutButtonMinWidth, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-shortcuts/src/client/Reference.module.css:37-39"},
	{Node: "Reference.button", Prop: "max-width", Official: 280, Impl: ShortcutButtonMaxWidth, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-shortcuts/src/client/Reference.module.css:37-39"},
	{Node: "Reference.button", Prop: "padding-inline", Official: 14, Impl: ShortcutButtonPadX, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-shortcuts/src/client/Reference.module.css:37-39"},
	{Node: "CurrentVersionRow.row", Prop: "padding-block", Official: 16, Impl: VersionRowPadY, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/CurrentVersionRow.module.css:1-7"},
	{Node: "CurrentVersionRow.row", Prop: "font-size", Official: 14, Impl: VersionRowFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/CurrentVersionRow.module.css:1-7"},
	{Node: "CurrentVersionRow.row", Prop: "line-height", Official: 22, Impl: VersionRowLineHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/CurrentVersionRow.module.css:1-7"},

	// ── 字体 ──
	{Node: "FontSizeRow.range", Prop: "min", Official: 10, Impl: FontSizeMin, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-theme/src/theme-settings.ts:24-30"},
	{Node: "FontSizeRow.range", Prop: "max", Official: 22, Impl: FontSizeMax, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-theme/src/theme-settings.ts:24-30"},
	{Node: "FontSizeRow.range", Prop: "default", Official: 14, Impl: FontSizeDefault, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-theme/src/theme-settings.ts:24-30"},
	{Node: "FontSizeRow.stack", Prop: "breakpoint", Official: 767, Impl: FontSizeStackedBreakpoint, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-theme/src/client/FontSizeRow.module.css:28-37"},
	{Node: "FontFamilyRow.field", Prop: "flex-shrink", Official: 1, Impl: FontFamilyFlexShrink, Unit: UnitRatio, Status: GeoAligned, Anchor: "ui-theme/src/client/FontFamilyRow.module.css:3-6"},
	{Node: "FontFamilyRow.field", Prop: "flex-basis", Official: 240, Impl: FontFamilyFlexBasis, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-theme/src/client/FontFamilyRow.module.css:3-6"},

	// ── models 右页 ──
	{Node: "ModelsSection.section", Prop: "max-width", Official: 720, Impl: ModelsSectionMaxWidth, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:6"},
	{Node: "ModelsSection.section", Prop: "gap", Official: 12, Impl: ModelsSectionGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:6"},
	{Node: "ModelsSection.title", Prop: "font-size", Official: 16, Impl: ModelsTitleFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:14"},
	{Node: "ModelsSection.title", Prop: "line-height", Official: 24, Impl: ModelsTitleLine, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:14"},
	{Node: "ModelsSection.intro", Prop: "font-size", Official: 14, Impl: ModelsIntroFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:22"},
	{Node: "ModelsSection.intro", Prop: "line-height", Official: 22, Impl: ModelsIntroLine, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:22"},
	{Node: "ModelsSection.rows", Prop: "gap", Official: 8, Impl: ModelsRowsGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:43"},
	{Node: "ModelsSection.rows", Prop: "margin-top", Official: 12, Impl: ModelsRowsMarginTop, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:43"},
	{Node: "ModelsSection.rowCard", Prop: "padding-block", Official: 12, Impl: ModelsRowCardPadY, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:54"},
	{Node: "ModelsSection.rowCard", Prop: "padding-inline", Official: 14, Impl: ModelsRowCardPadX, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:54"},
	{Node: "ModelsSection.rowCard", Prop: "gap", Official: 12, Impl: ModelsRowCardGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:54"},
	{Node: "ModelsSection.rowHead", Prop: "gap", Official: 10, Impl: ModelsRowHeadGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:64"},
	{Node: "ModelsSection.rowName", Prop: "font-size", Official: 14, Impl: ModelsRowNameSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:77"},
	{Node: "ModelsSection.credentialDot", Prop: "size", Official: 8, Impl: ModelsCredentialDot, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:98"},
	{Node: "ModelsSection.editor", Prop: "padding-block", Official: 14, Impl: ModelsEditorPadY, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:240"},
	{Node: "ModelsSection.editor", Prop: "padding-inline", Official: 16, Impl: ModelsEditorPadX, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:240"},
	{Node: "ModelsSection.editor", Prop: "gap", Official: 14, Impl: ModelsEditorGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:240"},
	{Node: "ModelsSection.primaryButton", Prop: "height", Official: 36, Impl: ModelsButtonHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:136-165"},
	{Node: "ModelsSection.primaryButton", Prop: "padding-inline", Official: 14, Impl: ModelsButtonPadX, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:136-165"},
	{Node: "ModelsSection.deleteDialog", Prop: "width", Official: 480, Impl: DeleteDialogMaxWidth, Unit: UnitPx, Status: GeoDerived, Anchor: "ui-settings-models/src/client/ModelsSection.module.css:908-910", Formula: "min(480px, 100%)"},

	// ── plugins 右页 ──
	{Node: "PluginsSettingsSection.section", Prop: "gap", Official: 12, Impl: PluginsSectionOuterGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugins/src/client/PluginsSettingsSection.module.css:3-9"},
	{Node: "PluginsSettingsSection.heading", Prop: "font-size", Official: 18, Impl: PluginsHeadingFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugins/src/client/PluginsSettingsSection.module.css:11-15"},
	{Node: "PluginsSettingsSection.heading", Prop: "font-weight", Official: 600, Impl: PluginsHeadingWeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugins/src/client/PluginsSettingsSection.module.css:11-15"},
	{Node: "PluginsSettingsSection.intro", Prop: "font-size", Official: 13, Impl: PluginsIntroFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugins/src/client/PluginsSettingsSection.module.css:17-21"},
	{Node: "PluginsSettingsSection.tabs", Prop: "gap", Official: 22, Impl: PluginsTabsGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugins/src/client/PluginsSettingsSection.module.css:23-29"},
	{Node: "PluginsSettingsSection.tabs", Prop: "margin-top", Official: 2, Impl: PluginsTabsMarginT, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugins/src/client/PluginsSettingsSection.module.css:23-29"},
	{Node: "PluginsSettingsSection.tab", Prop: "padding-top", Official: 7, Impl: PluginsTabPadTop, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugins/src/client/PluginsSettingsSection.module.css:31-41"},
	{Node: "PluginsSettingsSection.tab", Prop: "padding-bottom", Official: 9, Impl: PluginsTabPadBt, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugins/src/client/PluginsSettingsSection.module.css:31-41"},
	{Node: "PluginsSettingsSection.tab", Prop: "padding-inline", Official: 1, Impl: PluginsTabPadX, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugins/src/client/PluginsSettingsSection.module.css:31-41"},
	{Node: "PluginsSettingsSection.tab", Prop: "font-size", Official: 13, Impl: PluginsTabFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugins/src/client/PluginsSettingsSection.module.css:31-41"},
	{Node: "PluginsSettingsSection.tab", Prop: "line-height", Official: 20, Impl: PluginsTabLn, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugins/src/client/PluginsSettingsSection.module.css:31-41"},
	{Node: "PluginsSettingsSection.tab[data-active]", Prop: "underline-height", Official: 2, Impl: PluginsTabUnderline, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugins/src/client/PluginsSettingsSection.module.css:53-63"},
	{Node: "PluginInventorySettingsTab.section", Prop: "max-width", Official: 760, Impl: PluginsSectionMaxWidth, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:2-10"},
	{Node: "PluginInventorySettingsTab.section", Prop: "gap", Official: 14, Impl: PluginsSectionGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:2-10"},
	{Node: "PluginInventorySettingsTab.catalog", Prop: "gap", Official: 12, Impl: PluginsCatalogGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:47-51"},
	{Node: "PluginInventorySettingsTab.search", Prop: "height", Official: 36, Impl: PluginsSearchHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:67-78"},
	{Node: "PluginInventorySettingsTab.searchInput", Prop: "padding-left", Official: 36, Impl: PluginsSearchPadL, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:67-78"},
	{Node: "PluginInventorySettingsTab.searchInput", Prop: "padding-right", Official: 34, Impl: PluginsSearchPadR, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:67-78"},
	{Node: "PluginInventorySettingsTab.searchInput", Prop: "font-size", Official: 13, Impl: PluginsSearchFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:67-78"},
	{Node: "PluginInventorySettingsTab.cards", Prop: "gap", Official: 10, Impl: PluginsCardsGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:89-96"},
	{Node: "PluginInventorySettingsTab.cards", Prop: "grid-template-columns", Official: 2, Impl: PluginsCardColumns, Unit: UnitRatio, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:89-96"},
	{Node: "PluginInventorySettingsTab.cards", Prop: "single-column-breakpoint", Official: 520, Impl: PluginsCardSingleCol, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:501-505"},
	{Node: "PluginInventorySettingsTab.cardContent", Prop: "padding-block", Official: 12, Impl: PluginsCardPad, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:124-140"},
	{Node: "PluginInventorySettingsTab.cardContent", Prop: "padding-inline", Official: 14, Impl: PluginsCardPadX, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:124-140"},
	{Node: "PluginInventorySettingsTab.cardContent", Prop: "min-height", Official: 52, Impl: PluginsCardMinHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:124-140"},
	{Node: "PluginInventorySettingsTab.cardContent", Prop: "gap", Official: 2, Impl: PluginsCardContentGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:124-140"},
	{Node: "PluginInventorySettingsTab.cardMainRow", Prop: "gap", Official: 12, Impl: PluginsCardMainRowGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:163-169"},
	{Node: "PluginInventorySettingsTab.cardTitle", Prop: "font-size", Official: 14, Impl: PluginsCardTitleFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:178-184"},
	{Node: "PluginInventorySettingsTab.cardTitle", Prop: "line-height", Official: 20, Impl: PluginsCardTitleLn, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:178-184"},
	{Node: "PluginInventorySettingsTab.cardDescription", Prop: "font-size", Official: 12, Impl: PluginsCardDescFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:186-196"},
	{Node: "PluginInventorySettingsTab.cardDescription", Prop: "line-height", Official: 18, Impl: PluginsCardDescLn, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:186-196"},
	{Node: "PluginInventorySettingsTab.cardMeta", Prop: "padding-top", Official: 6, Impl: PluginsCardMetaPadTop, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:204-208"},
	{Node: "PluginInventorySettingsTab.cardIdentity", Prop: "font-size", Official: 12, Impl: PluginsCardChipFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:213-228"},
	{Node: "PluginInventorySettingsTab.cardIdentity", Prop: "line-height", Official: 18, Impl: PluginsCardChipLn, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:213-228"},
	{Node: "PluginInventorySettingsTab.cardIdentity", Prop: "padding-inline", Official: 6, Impl: PluginsCardChipPadX, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:213-228"},
	{Node: "PluginInventorySettingsTab.cardIdentity", Prop: "padding-block", Official: 1, Impl: PluginsCardChipPadY, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:213-228"},
	{Node: "PluginInventorySettingsTab.cardTrailing", Prop: "gap", Official: 8, Impl: PluginsCardTrailingGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:230-236"},
	{Node: "PluginInventorySettingsTab.chevron", Prop: "size", Official: 12, Impl: PluginsChevronSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:428-431"},
	{Node: "PluginInventorySettingsTab.group", Prop: "gap", Official: 10, Impl: PluginsGroupGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:245-249"},
	{Node: "PluginInventorySettingsTab.groupTitleRow", Prop: "gap", Official: 8, Impl: PluginsGroupTitleRowGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:253-258"},
	{Node: "PluginInventorySettingsTab.groupTitleRow", Prop: "min-height", Official: 36, Impl: PluginsGroupTitleRowMin, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:253-258"},
	{Node: "PluginInventorySettingsTab.groupTitle", Prop: "font-size", Official: 14, Impl: PluginsGroupTitle, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:296-301"},
	{Node: "PluginInventorySettingsTab.groupTitle", Prop: "line-height", Official: 22, Impl: PluginsGroupTitleLn, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:296-301"},
	{Node: "PluginInventorySettingsTab.groupSub", Prop: "font-size", Official: 12, Impl: PluginsGroupSubFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:303-312"},
	{Node: "PluginInventorySettingsTab.groupSub", Prop: "line-height", Official: 18, Impl: PluginsGroupSubLn, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:303-312"},
	{Node: "PluginInventorySettingsTab.groupSub", Prop: "margin-left", Official: 20, Impl: PluginsGroupSubMarginL, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:303-312"},
	{Node: "PluginInventorySettingsTab.groupSub", Prop: "margin-top", Official: -6, Impl: PluginsGroupSubMarginT, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:303-312"},
	{Node: "PluginInventorySettingsTab.group", Prop: "border-top+padding-top", Official: 14, Impl: PluginsGroupDividerPadTop, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:260-263"},
	{Node: "PluginInventorySettingsTab.groupBody", Prop: "gap", Official: 10, Impl: PluginsGroupBodyGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:359-363"},
	{Node: "PluginInventorySettingsTab.cardDetails", Prop: "padding-top", Official: 10, Impl: PluginsDetailsPadTop, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:437-441"},
	{Node: "PluginInventorySettingsTab.cardDetails", Prop: "padding-bottom", Official: 12, Impl: PluginsDetailsPadBottom, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:437-441"},
	{Node: "PluginInventorySettingsTab.cardDetails", Prop: "padding-inline", Official: 14, Impl: PluginsDetailsPadX, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:437-441"},
	{Node: "PluginInventorySettingsTab.details", Prop: "grid-template-columns", Official: 76, Impl: PluginsDetailsLabelWidth, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:449-454"},
	{Node: "PluginInventorySettingsTab.details", Prop: "column-gap", Official: 10, Impl: PluginsDetailsColGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:449-454"},
	{Node: "PluginInventorySettingsTab.details", Prop: "row-gap", Official: 6, Impl: PluginsDetailsRowGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:449-454"},
	{Node: "PluginInventorySettingsTab.details", Prop: "margin-top", Official: 8, Impl: PluginsDetailsMarginTop, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:449-454"},
	{Node: "PluginInventorySettingsTab.detailsDt", Prop: "font-size", Official: 11, Impl: PluginsDetailsDtFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:460-464"},
	{Node: "PluginInventorySettingsTab.detailsDt", Prop: "line-height", Official: 17, Impl: PluginsDetailsDtLn, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:460-464"},
	{Node: "PluginInventorySettingsTab.detailsDd", Prop: "font-size", Official: 12, Impl: PluginsDetailsDdFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:466-473"},
	{Node: "PluginInventorySettingsTab.detailsDd", Prop: "line-height", Official: 17, Impl: PluginsDetailsDdLn, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:466-473"},
	{Node: "PluginInventorySettingsTab.searchIcon", Prop: "left", Official: 12, Impl: PluginsSearchIconLeft, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:61-65"},
	{Node: "PluginInventorySettingsTab.retryButton", Prop: "border-width", Official: 0.5, Impl: PluginsRetryBorderWidth, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:37-45"},
	{Node: "PluginInventorySettingsTab.retryButton", Prop: "padding-inline", Official: 10, Impl: PluginsRetryPadX, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:37-45"},
	{Node: "PluginInventorySettingsTab.retryButton", Prop: "padding-block", Official: 4, Impl: PluginsRetryPadY, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:37-45"},
	{Node: "PluginInventorySettingsTab.status", Prop: "font-size", Official: 13, Impl: PluginsStatusFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:17-22"},
	{Node: "PluginInventorySettingsTab.status", Prop: "line-height", Official: 20, Impl: PluginsStatusLn, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:17-22"},
	{Node: "PluginInventorySettingsTab.brokenNote", Prop: "padding-inline", Official: 10, Impl: PluginsBrokenPadX, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:366-376"},
	{Node: "PluginInventorySettingsTab.brokenNote", Prop: "padding-block", Official: 8, Impl: PluginsBrokenPadY, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:366-376"},
	{Node: "PluginInventorySettingsTab.brokenNote", Prop: "font-size", Official: 12, Impl: PluginsBrokenFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:366-376"},
	{Node: "PluginInventorySettingsTab.brokenNote", Prop: "line-height", Official: 18, Impl: PluginsBrokenLn, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:366-376"},
	// ── 设置行第 3 格：桌面更新指示（DesktopUpdateIndicator.module.css）──
	{Node: "DesktopUpdateIndicator.indicator", Prop: "height", Official: 28, Impl: UpdateIndicatorHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/DesktopUpdateIndicator.module.css:7"},
	{Node: "DesktopUpdateIndicator.indicator", Prop: "padding-inline", Official: 8, Impl: UpdateIndicatorPadX, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/DesktopUpdateIndicator.module.css:8"},
	{Node: "DesktopUpdateIndicator.indicator", Prop: "column-gap", Official: 4, Impl: UpdateIndicatorGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/DesktopUpdateIndicator.module.css:5"},
	{Node: "DesktopUpdateIndicator.icon", Prop: "size", Official: 14, Impl: UpdateIndicatorIconSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/DesktopUpdateIndicator.module.css:29-34"},
	{Node: "DesktopUpdateIndicator.indicator", Prop: "border-width", Official: 1, Impl: UpdateIndicatorBorder, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/DesktopUpdateIndicator.module.css:10"},
	{Node: "DesktopUpdateIndicator.indicator", Prop: "border-tint", Official: 0.2, Impl: UpdateIndicatorBorderTint, Unit: UnitRatio, Status: GeoAligned, Anchor: "ui-settings-general/src/client/DesktopUpdateIndicator.module.css:10", Formula: "color-mix(in srgb, var(--dsw-alias-brand-primary-new-colorprimary-new-color) 20%, transparent)"},
	{Node: "DesktopUpdateIndicator.indicator", Prop: "background-tint", Official: 0.08, Impl: UpdateIndicatorBgTint, Unit: UnitRatio, Status: GeoAligned, Anchor: "ui-settings-general/src/client/DesktopUpdateIndicator.module.css:13", Formula: "color-mix(in srgb, var(--dsw-alias-brand-primary-new-colorprimary-new-color) 8%, transparent)"},
	{Node: "DesktopUpdateIndicator.indicator", Prop: "font-size", Official: 12, Impl: UpdateIndicatorFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/DesktopUpdateIndicator.module.css:15"},
	{Node: "DesktopUpdateIndicator.indicator", Prop: "line-height", Official: 18, Impl: UpdateIndicatorLineHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/DesktopUpdateIndicator.module.css:17"},
	{Node: "DesktopUpdateIndicator.spinner", Prop: "cycle-ms", Official: 1000, Impl: UpdateIndicatorSpinPeriodMS, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/DesktopUpdateIndicator.module.css:48"},
	{Node: "DesktopUpdateIndicator.errorDot", Prop: "size", Official: 6, Impl: UpdateErrorDotSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-general/src/client/DesktopUpdateIndicator.module.css:47"},

	// ── 「Agent 预设」页（#3；`ui-agent-preset/src/client/AgentPresetSection.module.css`）──
	//
	// 官方 `.section` 的 `gap:12px` / `max-width:720px`（:1-7）、`.cards` 的
	// `repeat(auto-fill,minmax(268px,1fr))` + `gap:12px`（:44-51）、卡片
	// `.cardMain{padding:14px 16px 12px;gap:12px}`（:179-192）、页脚
	// `.cardFoot{padding:6px 10px;gap:2px}`（:271-279）逐条对齐。
	//
	// 两条 line-height 是**无单位**值（1.4 / 1.55），本仓按像素实现，所以标 derived
	// 并把换算写进 Formula —— 别把它写成"官方就是 21px"，那不是原文。
	{Node: "AgentPresetSection.section", Prop: "gap", Official: 12, Impl: AgentPresetSectionGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:1-7"},
	{Node: "AgentPresetSection.section", Prop: "max-width", Official: 720, Impl: AgentPresetSectionMaxWidth, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:1-7"},
	{Node: "AgentPresetSection.title", Prop: "font-size", Official: 18, Impl: AgentPresetTitleFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:9-13"},
	{Node: "AgentPresetSection.title", Prop: "font-weight", Official: 600, Impl: AgentPresetTitleWeight, Unit: UnitRatio, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:9-13"},
	{Node: "AgentPresetSection.intro", Prop: "font-size", Official: 13, Impl: AgentPresetIntroFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:15-18"},
	{Node: "AgentPresetSection.cards", Prop: "grid-minmax", Official: 268, Impl: AgentPresetCardMinW, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:44-51"},
	{Node: "AgentPresetSection.cards", Prop: "gap", Official: 12, Impl: AgentPresetCardsGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:44-51"},
	{Node: "AgentPresetSection.card", Prop: "border-width", Official: 0.5, Impl: AgentPresetCardBorderWidth, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:54-56"},
	{Node: "AgentPresetSection.cardMain", Prop: "padding-block-start", Official: 14, Impl: AgentPresetCardMainPadTop, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:179-192"},
	{Node: "AgentPresetSection.cardMain", Prop: "padding-block-end", Official: 12, Impl: AgentPresetCardMainPadBottom, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:179-192"},
	{Node: "AgentPresetSection.cardMain", Prop: "padding-inline", Official: 16, Impl: AgentPresetCardMainPadX, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:179-192"},
	{Node: "AgentPresetSection.cardMain", Prop: "gap", Official: 12, Impl: AgentPresetCardMainGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:179-192"},
	{Node: "AgentPresetSection.cardHead", Prop: "gap", Official: 12, Impl: AgentPresetCardHeadGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:204-207"},
	{Node: "AgentPresetSection.cardIdentity", Prop: "gap", Official: 6, Impl: AgentPresetCardIdentityGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:215-220"},
	{Node: "AgentPresetSection.cardName", Prop: "font-size", Official: 15, Impl: AgentPresetCardNameFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:223-227"},
	{Node: "AgentPresetSection.cardName", Prop: "line-height", Official: 1.4, Impl: AgentPresetCardNameLn, Unit: UnitPx, Status: GeoDerived, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:223-227", Formula: "line-height:1.4（无单位）× font-size 15px = 21px"},
	{Node: "AgentPresetSection.cardDesc", Prop: "font-size", Official: 13, Impl: AgentPresetCardDescFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:240-244"},
	{Node: "AgentPresetSection.cardDesc", Prop: "line-height", Official: 1.55, Impl: AgentPresetCardDescLn, Unit: UnitPx, Status: GeoDerived, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:240-244", Formula: "line-height:1.55（无单位）× font-size 13px = 20.15px → 取整 20px"},
	{Node: "AgentPresetSection.cardId", Prop: "font-size", Official: 11, Impl: AgentPresetCardIDFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:259-265"},
	{Node: "AgentPresetSection.cardId", Prop: "line-height", Official: 21, Impl: AgentPresetCardIDLn, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:259-265"},
	{Node: "AgentPresetSection.cardFoot", Prop: "padding-block", Official: 6, Impl: AgentPresetCardFootPadY, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:271-279"},
	{Node: "AgentPresetSection.cardFoot", Prop: "padding-inline", Official: 10, Impl: AgentPresetCardFootPadX, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:271-279"},
	{Node: "AgentPresetSection.cardFoot", Prop: "gap", Official: 2, Impl: AgentPresetCardFootGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:271-279"},
	{Node: "AgentPresetSection.error", Prop: "font-size", Official: 12, Impl: AgentPresetErrorFontSize, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-agent-preset/src/client/AgentPresetSection.module.css:384-388"},
}

// GeoCount 返回表里 aligned / derived / deviation 三种状态的行数。
func GeoCount() (aligned, derived, deviation int) {
	for _, r := range OfficialSettingsGeometry {
		switch r.Status {
		case GeoAligned:
			aligned++
		case GeoDerived:
			derived++
		case GeoDeviation:
			deviation++
		}
	}
	return aligned, derived, deviation
}

// ── 派生几何：卡片尺寸与位置 ─────────────────────────────────────────

// PanelSize 求卡片尺寸（官方 `.panel` 的 width + height + max-width 三条一起解）。
//
// 官方原文：
//
//	width: 800px;
//	height: min(800px, calc(100vh - 2 * max(24px, var(--dsh-frame-overlay-top, 24px))));
//	max-width: calc(100vw - 48px);
//
// 三条都要参与：只取 width 会在窄窗口溢出（官方就是靠 max-width 收窄的）。
func PanelSize(viewportW, viewportH, overlayTop float32) (width, height float32) {
	if overlayTop < PanelOverlayTop {
		overlayTop = PanelOverlayTop
	}
	width = PanelWidth
	if maxW := viewportW - PanelMaxWidthInset; maxW > 0 && width > maxW {
		width = maxW
	}
	height = viewportH - 2*overlayTop
	if height > PanelMaxHeight {
		height = PanelMaxHeight
	}
	if height < 0 {
		height = 0
	}
	return width, height
}

// PanelLeft 求卡片左沿（官方 `.overlay` 是 flex 水平居中）。
func PanelLeft(viewportW, panelW float32) float32 {
	left := (viewportW - panelW) / 2
	if left < 0 {
		return 0
	}
	return left
}

// PanelTop 求卡片上沿（官方 `.overlay` 是 flex 垂直居中）。
func PanelTop(viewportH, panelH float32) float32 {
	top := (viewportH - panelH) / 2
	if top < 0 {
		return 0
	}
	return top
}

// NarrowViewport 报告是否落在官方窄窗口断点内（`<768px`）。
//
// 窄窗口下官方的行为是「panel 铺满视口 + rail 变横向条」；本仓骨架**未实现**
// 该重排（报告「未实现项」已登记），但判据本身要可测：否则下一个人会拿
// 767/768 这两个数各自猜一个。
func NarrowViewport(viewportW float32) bool {
	return viewportW < NarrowViewportBreakpoint+1
}
