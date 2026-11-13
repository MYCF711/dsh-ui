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

	// PluginsSectionMaxWidth 是内置插件页最大内容宽
	// （`max-width: 760px`，PluginInventorySettingsTab.module.css:2-10）。
	PluginsSectionMaxWidth float32 = 760
	// PluginsSectionGap 是页内纵向间距（`gap: 14px`，同处）。
	PluginsSectionGap    float32 = 14
	PluginsSearchHeight  float32 = 36
	PluginsSearchPadL    float32 = 36
	PluginsSearchPadR    float32 = 34
	PluginsCardsGap      float32 = 10
	PluginsCardPad       float32 = 12
	PluginsCardSingleCol float32 = 640
	PluginsGroupGap      float32 = 8
	PluginsGroupTitle    float32 = 13
	PluginsGroupTitleLn  float32 = 20
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
	{Node: "PluginInventorySettingsTab.section", Prop: "max-width", Official: 760, Impl: PluginsSectionMaxWidth, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:2-10"},
	{Node: "PluginInventorySettingsTab.section", Prop: "gap", Official: 14, Impl: PluginsSectionGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:2-10"},
	{Node: "PluginInventorySettingsTab.search", Prop: "height", Official: 36, Impl: PluginsSearchHeight, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:53-88"},
	{Node: "PluginInventorySettingsTab.cards", Prop: "gap", Official: 10, Impl: PluginsCardsGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:89-96"},
	{Node: "PluginInventorySettingsTab.card", Prop: "padding", Official: 12, Impl: PluginsCardPad, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:101"},
	{Node: "PluginInventorySettingsTab.cards", Prop: "single-column-breakpoint", Official: 640, Impl: PluginsCardSingleCol, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:89-96"},
	{Node: "PluginInventorySettingsTab.group", Prop: "gap", Official: 8, Impl: PluginsGroupGap, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:101-120"},
	{Node: "PluginInventorySettingsTab.groupTitle", Prop: "font-size", Official: 13, Impl: PluginsGroupTitle, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:101-120"},
	{Node: "PluginInventorySettingsTab.groupTitle", Prop: "line-height", Official: 20, Impl: PluginsGroupTitleLn, Unit: UnitPx, Status: GeoAligned, Anchor: "ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css:101-120"},
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
