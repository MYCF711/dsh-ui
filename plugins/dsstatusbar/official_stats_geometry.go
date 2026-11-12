// official_stats_geometry.go —— 官方 Activity 详情弹窗（StatsPills DialogPill）的
// **唯一几何常量表**（rc.2 口径）。
//
// 规矩与首屏骨架（internal/uikernel/official_geometry.go）和设置面
// （plugins/dssettings/official_geometry.go）同源：
//
//	① 结构树里每个 gap / padding / min-max 宽高只有一个出处：本文件的常量。
//	   绘制代码（official_stats_dialog.go）只准引用常量，不许再写魔法数。
//	② 「官方值」与「实现值」写在**同一张表**里（OfficialStatsGeometry 的每一行），
//	   测试逐行核对：aligned 行实现值必须等于官方值；derived 行必须写清官方原文
//	   公式；deviation 行必须写清原因。
//	③ 官方值全部从 rc.2 随包 `src/**` 的 CSS Module / TSX 读出来，锚点一律
//	   `<pkg>/src/….module.css:line`（或 `.tsx:line`），不是从截图猜的。
//
// 证据文件（rc.2 解包树 `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`）：
//
//	dsh-client-ui-chat/src/client/chat/stat-dialog.module.css  弹窗面（StatDialog）
//	dsh-client-ui-chat/src/client/chat/StatsPills.module.css   胶囊（pill）
//	dsh-client-ui-chat/src/client/chat/StatsPills.tsx          结构与行为
//	dsh-client-ui-chat/src/client/chat/stat-dialog.ts          展开态/点外关闭/Esc
//	dsh-client-ui-chat/src/client/locale.ts                    zh 文案（逐字）
package dsstatusbar

// StatsGeoUnit 是值的量纲。像素之外只有比例、权重、层序三种——混着写会让
// 「1100 是 1100px 还是 1100 层」变成口头约定。
type StatsGeoUnit string

const (
	StatsUnitPx     StatsGeoUnit = "px"
	StatsUnitWeight StatsGeoUnit = "font-weight"
	StatsUnitLayer  StatsGeoUnit = "z-index"
)

// StatsGeoStatus 说明一行「官方值 ↔ 实现值」的关系（三态，与 GR/GQ 报告同口径）。
type StatsGeoStatus string

const (
	// StatsGeoAligned 实现值与官方值一致。
	StatsGeoAligned StatsGeoStatus = "aligned"
	// StatsGeoDerived 官方是 calc()/clamp()/内容自适应，没有单一数值；实现值是
	// 实现侧的等价取值，必须同时写 Formula。
	StatsGeoDerived StatsGeoStatus = "derived"
	// StatsGeoDeviation 实现值与官方值**不同**或官方那层能力本仓没有，必须写 Reason。
	StatsGeoDeviation StatsGeoStatus = "deviation"
)

// StatsGeoRow 是一行几何事实。
type StatsGeoRow struct {
	Node     string         // 官方节点（StatsDialog.panel / StatsPills.pill …）
	Prop     string         // 属性（padding / gap / min-width …）
	Official float32        // 官方值；derived 行填实现侧等价取值
	Impl     float32        // 本仓实现值（同一行，不另表）
	Unit     StatsGeoUnit   // 量纲
	Status   StatsGeoStatus // aligned / derived / deviation
	Anchor   string         // 官方 rc.2 源码锚点
	Formula  string         // derived 行的官方原文（calc/clamp/内容自适应）
	Reason   string         // deviation 行必须写原因
}

// ── 弹窗面（官方 `.panel`，stat-dialog.module.css:3-40）────────────────
//
// 官方是 portal 出去的一层：`position: fixed; z-index: 1100`，`width: max-content`
// 再夹到 `min-width: min(300px, 100vw - 24px)` / `max-width: min(440px, 100vw - 24px)`，
// 锚定在触发胶囊**上方 8px**（`bottom: max(12px, calc(anchor(top) + 8px))`），
// 左右各留 12px 窗口内边距。
const (
	// StatsDialogPad 是 `.panel { padding: 16px }`（:13）。
	StatsDialogPad float32 = 16
	// StatsDialogMinWidth 是 `min-width: min(300px, calc(100vw - 24px))`（:11）的
	// 300 那一档；窗口更窄时按窗口夹（见 statsDialogWidth）。
	StatsDialogMinWidth float32 = 300
	// StatsDialogMaxWidth 是 `max-width: min(440px, calc(100vw - 24px))`（:12）的
	// 440 那一档。
	StatsDialogMaxWidth float32 = 440
	// StatsDialogWindowInset 是左右各 12px 的窗口内边距（:32-33 的 `max(12px, …)`
	// 与 `right: 12px`）。
	StatsDialogWindowInset float32 = 12
	// StatsDialogAnchorGap 是弹窗与触发胶囊之间的 8px（:31 的 `calc(anchor(top) + 8px)`）。
	StatsDialogAnchorGap float32 = 8
	// StatsDialogRadius 是 `border-radius: var(--dsw-radius-lg)`（:15）＝ lg 16。
	StatsDialogRadius float32 = 16
	// StatsDialogFontSize / StatsDialogLineHeight 是 `.panel { font-size:12px;
	// line-height:18px }`（:20-21）。**不是** theme 的 body 派生值：官方这里是
	// 写死的 12/18，用户调正文档不改变它。
	StatsDialogFontSize   float32 = 12
	StatsDialogLineHeight float32 = 18
	// StatsDialogZIndex 是 `.panel { z-index: 1100 }`（:5）。本仓没有全局 z-index
	// 阶梯，等价手段是「popup 层最后绘制」（flex 的 floats 在所有节点之后画）。
	StatsDialogZIndex float32 = 1100
	// StatsDialogBorderWidth 是 `.panel { border: 0 }`（:14）——面靠阴影浮起来，
	// 不描边（与 `.menu` 的 1px 描边不同）。
	StatsDialogBorderWidth float32 = 0
)

// ── 标题行（官方 `.title` / `.titleRule` / `.titleLabel`，stat-dialog.module.css:44-75）──
const (
	// StatsDialogTitleMarginBottom 是 `.title { margin-bottom: 8px }`（:48）。
	StatsDialogTitleMarginBottom float32 = 8
	// StatsDialogTitleGap 是 `.title { gap: 16px }`（:47）：章节名与右侧数值之间。
	StatsDialogTitleGap float32 = 16
	// StatsDialogTitleWeight 是 `.title { font-weight: 500 }`（:50）。
	StatsDialogTitleWeight float32 = 500
	// StatsDialogTitleLabelGap 是 `.titleLabel { gap: 6px }`（:68）：图标与章节名之间。
	StatsDialogTitleLabelGap float32 = 6
	// StatsDialogTitleIconSize 是 `.titleLabel svg { width:14px; height:14px }`（:72-73）。
	StatsDialogTitleIconSize float32 = 14
	// StatsDialogTitleRuleMarginBottom 是 `.titleRule { margin-bottom: 10px }`（:55）。
	StatsDialogTitleRuleMarginBottom float32 = 10
	// StatsDialogTitleRuleHeight 是 `.titleRule { border-top: 0.5px solid }`（:56）。
	StatsDialogTitleRuleHeight float32 = 0.5
)

// ── 明细网格（官方 `.details`，stat-dialog.module.css:77-95）────────────
//
// `display: grid; grid-template-columns: minmax(76px, auto) minmax(0, 1fr);
//
//	gap: 6px 16px`：左列是 dt（label-tertiary），右列是 dd（label-secondary、
//	等宽数字、右对齐）。dt 列的下界 76 是官方写死的，上界由最长标签撑开。
const (
	// StatsDialogDetailsLabelMinWidth 是 dt 列下界 76px（:79）。
	StatsDialogDetailsLabelMinWidth float32 = 76
	// StatsDialogDetailsColumnGap 是 `gap` 的列间距 16px（:80）。
	StatsDialogDetailsColumnGap float32 = 16
	// StatsDialogDetailsRowGap 是 `gap` 的行间距 6px（:80）。
	StatsDialogDetailsRowGap float32 = 6
)

// ── 胶囊（官方 `.pill`，StatsPills.module.css:6-58）──────────────────────
//
// 胶囊几何在 statusbar.go 已有唯一出处（pillHeight / pillPadX …），这里只把
// 官方弹窗相关的两条（图标、字阶）补进同一张表，避免第二套常量。
const (
	// StatsPillIconSize 是 `.pill svg { width:14px; height:14px }`（StatsPills.module.css:30-34）。
	StatsPillIconSize float32 = 14
	// StatsPillFontSize 是 `.anchor { font-size: calc(13px - 1px) }`（:8-9）的 12。
	StatsPillFontSize float32 = 12
	// StatsPillSepMargin 是 `.sep { margin: 0 6px }`（:62）——两段读数之间的分隔符留白。
	StatsPillSepMargin float32 = 6
)

// OfficialStatsGeometry 是 Activity 详情弹窗几何的**逐行登记**。
//
// 三态计数由 StatsGeoCount 给出（测试逐行核对，见 official_stats_geometry_test.go）。
var OfficialStatsGeometry = []StatsGeoRow{
	// ── 弹窗面 ──
	{Node: "StatsDialog.panel", Prop: "padding", Official: 16, Impl: StatsDialogPad, Unit: StatsUnitPx, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:13"},
	{Node: "StatsDialog.panel", Prop: "min-width", Official: 300, Impl: StatsDialogMinWidth, Unit: StatsUnitPx, Status: StatsGeoDerived, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:11", Formula: "min(300px, calc(100vw - 24px))"},
	{Node: "StatsDialog.panel", Prop: "max-width", Official: 440, Impl: StatsDialogMaxWidth, Unit: StatsUnitPx, Status: StatsGeoDerived, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:12", Formula: "min(440px, calc(100vw - 24px))"},
	{Node: "StatsDialog.panel", Prop: "window-inset", Official: 12, Impl: StatsDialogWindowInset, Unit: StatsUnitPx, Status: StatsGeoDerived, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:32-33", Formula: "left: max(12px, anchor(left)); right: 12px"},
	{Node: "StatsDialog.panel", Prop: "anchor-gap", Official: 8, Impl: StatsDialogAnchorGap, Unit: StatsUnitPx, Status: StatsGeoDerived, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:31", Formula: "bottom: max(12px, calc(anchor(top) + 8px))"},
	{Node: "StatsDialog.panel", Prop: "border-radius", Official: 16, Impl: StatsDialogRadius, Unit: StatsUnitPx, Status: StatsGeoDerived, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:15", Formula: "var(--dsw-radius-lg) = 16"},
	{Node: "StatsDialog.panel", Prop: "font-size", Official: 12, Impl: StatsDialogFontSize, Unit: StatsUnitPx, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:20"},
	{Node: "StatsDialog.panel", Prop: "line-height", Official: 18, Impl: StatsDialogLineHeight, Unit: StatsUnitPx, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:21"},
	{Node: "StatsDialog.panel", Prop: "border-width", Official: 0, Impl: StatsDialogBorderWidth, Unit: StatsUnitPx, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:14"},
	{Node: "StatsDialog.panel", Prop: "z-index", Official: 1100, Impl: StatsDialogZIndex, Unit: StatsUnitLayer, Status: StatsGeoDerived, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:5", Formula: "position:fixed + z-index:1100；本仓等价物 = flex popup 层（在所有节点之后绘制）"},
	// ── 标题行 ──
	{Node: "StatsDialog.title", Prop: "margin-bottom", Official: 8, Impl: StatsDialogTitleMarginBottom, Unit: StatsUnitPx, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:48"},
	{Node: "StatsDialog.title", Prop: "gap", Official: 16, Impl: StatsDialogTitleGap, Unit: StatsUnitPx, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:47"},
	{Node: "StatsDialog.title", Prop: "font-weight", Official: 500, Impl: StatsDialogTitleWeight, Unit: StatsUnitWeight, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:50"},
	{Node: "StatsDialog.titleLabel", Prop: "gap", Official: 6, Impl: StatsDialogTitleLabelGap, Unit: StatsUnitPx, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:68"},
	{Node: "StatsDialog.titleLabel.icon", Prop: "size", Official: 14, Impl: StatsDialogTitleIconSize, Unit: StatsUnitPx, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:72-73（图标本体：ActivityPill 的 IconGaugeOutlineRegular，StatsPills.tsx:255；rc.2 sprite gauge-outline，viewBox 0 0 16 16）"},
	{Node: "StatsDialog.titleRule", Prop: "margin-bottom", Official: 10, Impl: StatsDialogTitleRuleMarginBottom, Unit: StatsUnitPx, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:55"},
	{Node: "StatsDialog.titleRule", Prop: "border-top", Official: 0.5, Impl: StatsDialogTitleRuleHeight, Unit: StatsUnitPx, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:56"},
	// ── 明细网格 ──
	{Node: "StatsDialog.details", Prop: "column-gap", Official: 16, Impl: StatsDialogDetailsColumnGap, Unit: StatsUnitPx, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:80"},
	{Node: "StatsDialog.details", Prop: "row-gap", Official: 6, Impl: StatsDialogDetailsRowGap, Unit: StatsUnitPx, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:80"},
	{Node: "StatsDialog.details.dt", Prop: "min-width", Official: 76, Impl: StatsDialogDetailsLabelMinWidth, Unit: StatsUnitPx, Status: StatsGeoDerived, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:79", Formula: "grid-template-columns: minmax(76px, auto) minmax(0, 1fr)"},
	{Node: "StatsDialog.details.dd", Prop: "text-align", Official: 0, Impl: 0, Unit: StatsUnitPx, Status: StatsGeoDeviation, Anchor: "dsh-client-ui-chat/src/client/chat/stat-dialog.module.css:94", Reason: "官方 dd 是 text-align:right 的文本对齐（不是可量的像素值）；本仓用 TextEnd 表达同一语义，几何值记 0（不可量化），行为断言见 official_stats_dialog_test.go 的右对齐用例"},
	// ── 胶囊（弹窗入口）──
	{Node: "StatsPills.pill", Prop: "padding-block", Official: 1, Impl: pillPadY, Unit: StatsUnitPx, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/StatsPills.module.css:19"},
	{Node: "StatsPills.pill", Prop: "padding-inline", Official: 8, Impl: pillPadX, Unit: StatsUnitPx, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/StatsPills.module.css:19"},
	{Node: "StatsPills.pill", Prop: "gap", Official: 6, Impl: pillGap, Unit: StatsUnitPx, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/StatsPills.module.css:17"},
	{Node: "StatsPills.pill", Prop: "border-radius", Official: 8, Impl: 8, Unit: StatsUnitPx, Status: StatsGeoDerived, Anchor: "dsh-client-ui-chat/src/client/chat/StatsPills.module.css:21", Formula: "var(--dsw-radius-sm) = 8（实现取 theme.Radii.SM）"},
	{Node: "StatsPills.pill", Prop: "line-height", Official: 20, Impl: pillLineHeight, Unit: StatsUnitPx, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/StatsPills.module.css:10"},
	{Node: "StatsPills.pill", Prop: "height", Official: 22, Impl: pillHeight, Unit: StatsUnitPx, Status: StatsGeoDerived, Anchor: "dsh-client-ui-chat/src/client/chat/StatsPills.module.css:13-28", Formula: "line-height 20 + 2×padding 1（无固定 height，由内容撑开）"},
	{Node: "StatsPills.pill.icon", Prop: "size", Official: 14, Impl: StatsPillIconSize, Unit: StatsUnitPx, Status: StatsGeoAligned, Anchor: "dsh-client-ui-chat/src/client/chat/StatsPills.module.css:30-34（Activity = rc.2 sprite gauge-outline / Usage = database-outline，见 StatsPills.tsx:255/305）"},
	{Node: "StatsPills.anchor", Prop: "font-size", Official: 12, Impl: StatsPillFontSize, Unit: StatsUnitPx, Status: StatsGeoDerived, Anchor: "dsh-client-ui-chat/src/client/chat/StatsPills.module.css:9", Formula: "calc(var(--dsh-content-font-size-secondary, 13px) - 1px) = 12"},
}

// StatsGeoCount 数三态各有几行（测试用来核对状态计数与总行数一致）。
func StatsGeoCount() (aligned, derived, deviation int) {
	for _, row := range OfficialStatsGeometry {
		switch row.Status {
		case StatsGeoAligned:
			aligned++
		case StatsGeoDerived:
			derived++
		case StatsGeoDeviation:
			deviation++
		}
	}
	return aligned, derived, deviation
}

// statsGeoRow 按 node+prop 取一行（测试用）。
func statsGeoRow(node, prop string) (StatsGeoRow, bool) {
	for _, row := range OfficialStatsGeometry {
		if row.Node == node && row.Prop == prop {
			return row, true
		}
	}
	return StatsGeoRow{}, false
}
