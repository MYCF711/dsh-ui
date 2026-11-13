// official_skill.go —— 官方 `@deepseek-ai/dsh-client-ui-skill` 面的**唯一事实表**（rc.2 口径）。
//
// # 官方包干什么（逐条读源码得到，不是猜的）
//
// 官方包 `dsh-client-ui-skill@0.2.2-rc.2` 的 README 只有一句
// *"Web skill references and the dedicated skill tool row"*，但它其实有**五个**
// 互不相同的面（`src/client/index.ts` 的 `apply` 里逐个注册）：
//
//	① `ctx.locale.register(NS, {zh,en,ja,es,fr,de,ru,pt})`  —— 8 语言字典（NS = 'skill'）
//	② `ctx.uiConversation.userText.register(skillReferenceRule)` —— 用户文本里的 `/name` 规则
//	③ `ctx.slots.inject('tool.call.toolview', …{key:'skill'}…)` —— **专用工具行** SkillRow
//	④ `ctx.slots.inject('conversation.user.text.node', …{key:'skill',priority:100}…)`
//	                                                          —— 用户文本里的 skill 胶囊
//	⑤ `inputTriggers.registerSource(source)`               —— `/` 斜杠候选源（order 2）
//
// 本文件只描述 ③ 的**几何 / 令牌 / 结构**，以及 ⑤ 的候选排序契约；
// ② 的规则在 `user_text_rules.go`，④ 的胶囊在 `skillreference.go`，
// ① 的字典在 `locales.go`。
//
// # 官方源码锚点（全部相对解包树 `tmp/GO8-RC2/official-rc2/dsh/node_modules/`）
//
//	@deepseek-ai/dsh-client-ui-skill/src/client/SkillRow.tsx           173 行（主体）
//	@deepseek-ai/dsh-client-ui-skill/src/client/SkillRow.module.css    215 行（几何）
//	@deepseek-ai/dsh-client-ui-skill/src/client/SkillReference.tsx      28 行（胶囊）
//	@deepseek-ai/dsh-client-ui-skill/src/client/user-text-rules.ts      36 行（规则）
//	@deepseek-ai/dsh-client-ui-skill/src/client/locales.ts             103 行（8 语言）
//	@deepseek-ai/dsh-client-ui-skill/src/client/index.ts               264 行（装配）
//
// # 一个必须说清的语义（否则实现会画错）
//
// 官方**不**从活的 skill 目录推状态：`skillRowModel` 的注释是
// *"Derive display state without consulting the live skill catalog"*。
// 整行（名称 / 输出 / 错误摘要 / 四态）**只**来自这一条 durable 的
// call/result slice。所以本包的渲染器**没有**任何宿主 RPC 依赖，
// 也因此 `skills/list` 缺失（见报告「缺口」§1）**不影响**工具行，
// 只影响斜杠候选源与胶囊的 `openSkill`。
//
// # 规矩（与 `internal/uikernel/official_geometry.go`、`plugins/dsstatusbar/official_stats_geometry.go` 同源）
//
//	① 每个数值只有一个出处：本文件的常量。
//	② 「官方值」与「实现值」写在**同一张表**里：aligned 行必须相等，
//	   derived 行必须写官方公式，deviation 行必须写原因。
//	③ 官方值全部从 rc.2 随包 `src/**` 的 CSS Module / TSX 读出来，
//	   **不是从截图猜的**，也不是沿用 rc.1 的口径。
package dsskill

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// ── 官方四态（`SkillRow.tsx:10`）────────────────────────────────────

// SkillRowState 是官方 `SkillRowState`：**只由 durable 的 call slice 推出**。
type SkillRowState string

const (
	// SkillStateRunning 是未落定的调用（`!('kind' in block)`）。
	SkillStateRunning SkillRowState = "running"
	// SkillStateOK 是已落定且没有错误。
	SkillStateOK SkillRowState = "ok"
	// SkillStateError 是已落定且 `isError`。
	SkillStateError SkillRowState = "error"
	// SkillStateStopped 是已落定且 `error.code === 'interrupted'`。
	// 注意判据是 **code**，不是 `isError`：中止优先于错误。
	SkillStateStopped SkillRowState = "stopped"
)

// SkillPhase 是官方 `ToolCallViewProps['phase']`（`SkillRow.tsx:103`）。
//
// 两态而不是三态：`preparing` 走**另一棵**组件树（只有 leading + 隐藏文案 +
// 闪烁标题），`started` 才走完整行。把它折进 `SkillRowState` 会让
// 「preparing 时没有 data-state 四态」这条官方事实消失。
type SkillPhase string

const (
	// SkillPhasePreparing 是参数还没到（`data-state="preparing"` 写死在 JSX 里）。
	SkillPhasePreparing SkillPhase = "preparing"
	// SkillPhaseStarted 是调用已经落盘，走完整行。
	SkillPhaseStarted SkillPhase = "started"
)

// ── 几何表的三个量纲 ─────────────────────────────────────────────────

// SkillUnit 是值的量纲。像素之外只有比例、字重、em 相对、毫秒四种——
// 混着写会让「100 是 100px 还是 100ms」变成口头约定。
type SkillUnit string

const (
	// SkillUnitPx 是逻辑像素。
	SkillUnitPx SkillUnit = "px"
	// SkillUnitRatio 是 0–1 的不透明度 / 比例。
	SkillUnitRatio SkillUnit = "ratio"
	// SkillUnitWeight 是 `font-weight` 的数值档。
	SkillUnitWeight SkillUnit = "font-weight"
	// SkillUnitEm 是相对当前字号的长度（官方 `letter-spacing: 0.04em`）。
	SkillUnitEm SkillUnit = "em"
	// SkillUnitMs 是过渡时长。
	SkillUnitMs SkillUnit = "ms"
)

// SkillStatus 说明一行「官方值 ↔ 实现值」的关系（三态，与 GR/GQ/H2 报告同口径）。
type SkillStatus string

const (
	// SkillAligned 实现值与官方值一致。
	SkillAligned SkillStatus = "aligned"
	// SkillDerived 官方是 calc()/clamp()/内容自适应，没有单一数值；实现值是
	// 实现侧的等价取值，必须同时写 Formula。
	SkillDerived SkillStatus = "derived"
	// SkillDeviation 实现值与官方值**不同**，必须写 Reason。
	SkillDeviation SkillStatus = "deviation"
)

// SkillGeoRow 是一行几何事实。
type SkillGeoRow struct {
	Node     string      // 官方节点（SkillRow.row / SkillRow.leading …）
	Prop     string      // 属性（height / margin-right / font-size …）
	Official float32     // 官方值；derived 行填实现侧等价取值
	Impl     float32     // 本仓实现值（同一行，不另表）
	Unit     SkillUnit   // 量纲
	Status   SkillStatus // aligned / derived / deviation
	Anchor   string      // 官方 rc.2 源码锚点 `SkillRow.module.css:行号`
	Formula  string      // derived 行的官方原文（calc/clamp/内容自适应）
	Reason   string      // deviation 行必须写原因
}

// ── 官方 CSS 的每一个数值（`SkillRow.module.css` 逐行）────────────────

const (
	// `.row{height:calc(24px + var(--dsh-content-font-delta, 0px))}`（:13）。
	SkillRowHeight float32 = 24
	// `.row{min-width:0}`（:14）。
	SkillRowMinWidth float32 = 0
	// `.row{transition:color 100ms ease}`（:16）。
	SkillRowColorTransitionMs float32 = 100

	// `.leading{width:calc(16px + var(--dsh-content-font-delta, 0px))}`（:32）。
	SkillLeadingWidth float32 = 16
	// `.leading{height:calc(16px + var(--dsh-content-font-delta, 0px))}`（:33）。
	SkillLeadingHeight float32 = 16
	// `.leading{margin-right:6px}`（:37）。
	SkillLeadingMarginRight float32 = 6
	// `.leading svg{width:calc(14px + var(--dsh-content-font-delta, 0px))}`（:42）。
	SkillLeadingIconSize float32 = 14

	// `.iconIdle{opacity:1}`（:52）。
	SkillIconIdleOpacity float32 = 1
	// `.iconIdle{transition:opacity 100ms ease}`（:53）。
	SkillIconIdleTransitionMs float32 = 100
	// `.chevronHover{opacity:0}`（:60）。
	SkillChevronHoverOpacity float32 = 0
	// `.chevronHover{transition:opacity 100ms ease}`（:61）。
	SkillChevronHoverTransitionMs float32 = 100
	// `.row:hover .iconIdle{opacity:0}`（:66）。
	SkillRowHoverIconIdleOpacity float32 = 0
	// `.row:hover .chevronHover{opacity:1}`（:70）。
	SkillRowHoverChevronOpacity float32 = 1

	// `.title{font-size:var(--dsh-content-font-size-secondary, 13px)}`（:76）。
	SkillTitleFontSize float32 = 13
	// `.title{line-height:calc(24px + var(--dsh-content-font-delta, 0px))}`（:77）。
	SkillTitleLineHeight float32 = 24

	// `.separator{width:2px}`（:82）。
	SkillSeparatorWidth float32 = 2
	// `.separator{height:2px}`（:83）。
	SkillSeparatorHeight float32 = 2
	// `.separator{border-radius:1px}`（:84）。
	SkillSeparatorRadius float32 = 1
	// `.separator{margin:0 8px}`（:85）。
	SkillSeparatorMarginBlock  float32 = 0
	SkillSeparatorMarginInline float32 = 8

	// `.summary{font-size:var(--dsh-content-font-size-secondary, 13px)}`（:95）。
	SkillSummaryFontSize float32 = 13
	// `.summary{line-height:calc(24px + var(--dsh-content-font-delta, 0px))}`（:96）。
	SkillSummaryLineHeight float32 = 24
	// `.summary{min-width:0}`（:92）。
	SkillSummaryMinWidth float32 = 0

	// `.instructionsCard{max-height:260px}`（:115）。
	SkillInstructionsCardMaxHeight float32 = 260
	// `.instructionsCard{margin:4px 0 4px 4px}`（:116）。
	SkillInstructionsCardMarginTop    float32 = 4
	SkillInstructionsCardMarginRight  float32 = 0
	SkillInstructionsCardMarginBottom float32 = 4
	SkillInstructionsCardMarginLeft   float32 = 4
	// `.instructionsCard{border:0.5px solid …}`（:118）。
	SkillInstructionsCardBorderWidth float32 = 0.5
	// `.instructionsCard{border-radius:var(--dsw-radius-lg)}`（:119）→ 16。
	SkillInstructionsCardRadius float32 = 16

	// `.instructionsHeader{padding:8px 12px}`（:125）。
	SkillInstructionsHeaderPadTop    float32 = 8
	SkillInstructionsHeaderPadRight  float32 = 12
	SkillInstructionsHeaderPadBottom float32 = 8
	SkillInstructionsHeaderPadLeft   float32 = 12
	// `.instructionsHeader{border-bottom:0.5px solid …}`（:126）。
	SkillInstructionsHeaderBorderBottom float32 = 0.5
	// `.instructionsHeader{font-size:11px}`（:128）。
	SkillInstructionsHeaderFontSize float32 = 11
	// `.instructionsHeader{font-weight:500}`（:129）。
	SkillInstructionsHeaderWeight float32 = 500
	// `.instructionsHeader{line-height:16px}`（:130）。
	SkillInstructionsHeaderLineHeight float32 = 16
	// `.instructionsHeader{letter-spacing:0.04em}`（:133）。
	SkillInstructionsHeaderLetterSpacing float32 = 0.04

	// `.instructions{min-height:0}`（:137）。
	SkillInstructionsMinHeight float32 = 0
	// `.instructions{margin:0}`（:138）。
	SkillInstructionsMargin float32 = 0
	// `.instructions{padding:10px 12px 12px}`（:139）。
	SkillInstructionsPadTop    float32 = 10
	SkillInstructionsPadRight  float32 = 12
	SkillInstructionsPadBottom float32 = 12
	SkillInstructionsPadLeft   float32 = 12
	// `.instructions{font:var(--dsw-font-markdown-code-block-small)}`（:143）。
	// 官方令牌 = `var(--dsh-code-font-size,11px) / calc(19px + var(--dsh-code-font-delta))`。
	SkillInstructionsFontSize float32 = 11
	// `.instructions{line-height:calc(19px + var(--dsh-code-font-delta))}`。
	SkillInstructionsLineHeight float32 = 19

	// `.instructions::-webkit-scrollbar-thumb{border:2px solid transparent}`（:152）。
	SkillScrollbarThumbBorder float32 = 2
	// `.instructions::-webkit-scrollbar-thumb{border-radius:var(--dsw-radius-sm)}`（:154）→ 8。
	SkillScrollbarThumbRadius float32 = 8
	// `.instructions::-webkit-scrollbar-track{margin:6px 0}`（:158）。
	SkillScrollbarTrackMargin float32 = 6

	// `.inspectButton{gap:4px}`（:165）。
	SkillInspectGap float32 = 4
	// `.inspectButton{margin:4px 0 2px 4px}`（:166）。
	SkillInspectMarginTop    float32 = 4
	SkillInspectMarginRight  float32 = 0
	SkillInspectMarginBottom float32 = 2
	SkillInspectMarginLeft   float32 = 4
	// `.inspectButton{padding:2px 8px}`（:167）。
	SkillInspectPadTop    float32 = 2
	SkillInspectPadRight  float32 = 8
	SkillInspectPadBottom float32 = 2
	SkillInspectPadLeft   float32 = 8
	// `.inspectButton{border:0.5px solid …}`（:168）。
	SkillInspectBorderWidth float32 = 0.5
	// `.inspectButton{border-radius:999px}`（:169）。
	SkillInspectRadius float32 = 999
	// `.inspectButton{font-size:11px}`（:173）。
	SkillInspectFontSize float32 = 11
	// `.inspectButton{line-height:16px}`（:174）。
	SkillInspectLineHeight float32 = 16
	// `.inspectButton{opacity:0}`（:176）。
	SkillInspectOpacity float32 = 0
	// `.inspectButton{transition:opacity 100ms ease}`（:177）。
	SkillInspectTransitionMs float32 = 100
	// `.inspectButton:focus-visible{opacity:1}`（:181）。
	SkillInspectFocusOpacity float32 = 1
	// `.card:hover .inspectButton{opacity:1}`（:186）。
	SkillCardHoverInspectOpacity float32 = 1
	// `@media (hover:none){.inspectButton{opacity:1}}`（:214）。
	SkillInspectNoHoverOpacity float32 = 1

	// `.visuallyHidden{width:1px}`（:197）。
	SkillVisuallyHiddenWidth float32 = 1
	// `.visuallyHidden{height:1px}`（:198）。
	SkillVisuallyHiddenHeight float32 = 1
)

// officialSkillGeometry 是官方 `.module.css` 的**全量数值**，一条不落。
//
// 排序 = 官方 CSS 文件自上而下的顺序，方便逐行对照。
var officialSkillGeometry = []SkillGeoRow{
	// ── .row（:8-17）────────────────────────────────────────────────
	{Node: "SkillRow.row", Prop: "height", Official: 24, Impl: SkillRowHeight, Unit: SkillUnitPx, Status: SkillDerived, Anchor: "SkillRow.module.css:13", Formula: "calc(24px + var(--dsh-content-font-delta, 0px))；实现取 delta=0 基准值"},
	{Node: "SkillRow.row", Prop: "min-width", Official: 0, Impl: SkillRowMinWidth, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:14"},
	{Node: "SkillRow.row", Prop: "transition-duration", Official: 100, Impl: SkillRowColorTransitionMs, Unit: SkillUnitMs, Status: SkillAligned, Anchor: "SkillRow.module.css:16"},

	// ── .leading（:29-44）──────────────────────────────────────────
	{Node: "SkillRow.leading", Prop: "width", Official: 16, Impl: SkillLeadingWidth, Unit: SkillUnitPx, Status: SkillDerived, Anchor: "SkillRow.module.css:32", Formula: "calc(16px + var(--dsh-content-font-delta, 0px))；实现取 delta=0"},
	{Node: "SkillRow.leading", Prop: "height", Official: 16, Impl: SkillLeadingHeight, Unit: SkillUnitPx, Status: SkillDerived, Anchor: "SkillRow.module.css:33", Formula: "calc(16px + var(--dsh-content-font-delta, 0px))；实现取 delta=0"},
	{Node: "SkillRow.leading", Prop: "margin-right", Official: 6, Impl: SkillLeadingMarginRight, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:37"},
	{Node: "SkillRow.leading svg", Prop: "width", Official: 14, Impl: SkillLeadingIconSize, Unit: SkillUnitPx, Status: SkillDerived, Anchor: "SkillRow.module.css:42", Formula: "calc(14px + var(--dsh-content-font-delta, 0px))；实现取 delta=0"},
	{Node: "SkillRow.leading svg", Prop: "height", Official: 14, Impl: SkillLeadingIconSize, Unit: SkillUnitPx, Status: SkillDerived, Anchor: "SkillRow.module.css:43", Formula: "calc(14px + var(--dsh-content-font-delta, 0px))；实现取 delta=0"},

	// ── 悬停换图（:50-72）──────────────────────────────────────────
	{Node: "SkillRow.iconIdle", Prop: "opacity", Official: 1, Impl: SkillIconIdleOpacity, Unit: SkillUnitRatio, Status: SkillAligned, Anchor: "SkillRow.module.css:52"},
	{Node: "SkillRow.iconIdle", Prop: "transition-duration", Official: 100, Impl: SkillIconIdleTransitionMs, Unit: SkillUnitMs, Status: SkillAligned, Anchor: "SkillRow.module.css:53"},
	{Node: "SkillRow.chevronHover", Prop: "opacity", Official: 0, Impl: SkillChevronHoverOpacity, Unit: SkillUnitRatio, Status: SkillAligned, Anchor: "SkillRow.module.css:60"},
	{Node: "SkillRow.chevronHover", Prop: "transition-duration", Official: 100, Impl: SkillChevronHoverTransitionMs, Unit: SkillUnitMs, Status: SkillAligned, Anchor: "SkillRow.module.css:61"},
	{Node: "SkillRow.row:hover .iconIdle", Prop: "opacity", Official: 0, Impl: SkillRowHoverIconIdleOpacity, Unit: SkillUnitRatio, Status: SkillAligned, Anchor: "SkillRow.module.css:66"},
	{Node: "SkillRow.row:hover .chevronHover", Prop: "opacity", Official: 1, Impl: SkillRowHoverChevronOpacity, Unit: SkillUnitRatio, Status: SkillAligned, Anchor: "SkillRow.module.css:70"},

	// ── .title（:74-78）────────────────────────────────────────────
	{Node: "SkillRow.title", Prop: "font-size", Official: 13, Impl: SkillTitleFontSize, Unit: SkillUnitPx, Status: SkillDerived, Anchor: "SkillRow.module.css:76", Formula: "var(--dsh-content-font-size-secondary, 13px)；官方默认档 13px（theme: min(14-1, max(13, 14-2)) = 13）"},
	{Node: "SkillRow.title", Prop: "line-height", Official: 24, Impl: SkillTitleLineHeight, Unit: SkillUnitPx, Status: SkillDerived, Anchor: "SkillRow.module.css:77", Formula: "calc(24px + var(--dsh-content-font-delta, 0px))；实现取 delta=0"},

	// ── .separator（:80-87）────────────────────────────────────────
	{Node: "SkillRow.separator", Prop: "width", Official: 2, Impl: SkillSeparatorWidth, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:82"},
	{Node: "SkillRow.separator", Prop: "height", Official: 2, Impl: SkillSeparatorHeight, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:83"},
	{Node: "SkillRow.separator", Prop: "border-radius", Official: 1, Impl: SkillSeparatorRadius, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:84"},
	{Node: "SkillRow.separator", Prop: "margin-block", Official: 0, Impl: SkillSeparatorMarginBlock, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:85"},
	{Node: "SkillRow.separator", Prop: "margin-inline", Official: 8, Impl: SkillSeparatorMarginInline, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:85"},

	// ── .summary（:89-97）──────────────────────────────────────────
	{Node: "SkillRow.summary", Prop: "min-width", Official: 0, Impl: SkillSummaryMinWidth, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:92"},
	{Node: "SkillRow.summary", Prop: "font-size", Official: 13, Impl: SkillSummaryFontSize, Unit: SkillUnitPx, Status: SkillDerived, Anchor: "SkillRow.module.css:95", Formula: "var(--dsh-content-font-size-secondary, 13px)"},
	{Node: "SkillRow.summary", Prop: "line-height", Official: 24, Impl: SkillSummaryLineHeight, Unit: SkillUnitPx, Status: SkillDerived, Anchor: "SkillRow.module.css:96", Formula: "calc(24px + var(--dsh-content-font-delta, 0px))；实现取 delta=0"},

	// ── .instructionsCard（:112-121）───────────────────────────────
	{Node: "SkillRow.instructionsCard", Prop: "max-height", Official: 260, Impl: SkillInstructionsCardMaxHeight, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:115"},
	{Node: "SkillRow.instructionsCard", Prop: "margin-top", Official: 4, Impl: SkillInstructionsCardMarginTop, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:116"},
	{Node: "SkillRow.instructionsCard", Prop: "margin-right", Official: 0, Impl: SkillInstructionsCardMarginRight, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:116"},
	{Node: "SkillRow.instructionsCard", Prop: "margin-bottom", Official: 4, Impl: SkillInstructionsCardMarginBottom, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:116"},
	{Node: "SkillRow.instructionsCard", Prop: "margin-left", Official: 4, Impl: SkillInstructionsCardMarginLeft, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:116"},
	{Node: "SkillRow.instructionsCard", Prop: "border-width", Official: 0.5, Impl: SkillInstructionsCardBorderWidth, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:118"},
	{Node: "SkillRow.instructionsCard", Prop: "border-radius", Official: 16, Impl: SkillInstructionsCardRadius, Unit: SkillUnitPx, Status: SkillDerived, Anchor: "SkillRow.module.css:119", Formula: "var(--dsw-radius-lg) = 16（pluginapi.Theme.Radii.LG）"},

	// ── .instructionsHeader（:123-134）─────────────────────────────
	{Node: "SkillRow.instructionsHeader", Prop: "padding-top", Official: 8, Impl: SkillInstructionsHeaderPadTop, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:125"},
	{Node: "SkillRow.instructionsHeader", Prop: "padding-right", Official: 12, Impl: SkillInstructionsHeaderPadRight, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:125"},
	{Node: "SkillRow.instructionsHeader", Prop: "padding-bottom", Official: 8, Impl: SkillInstructionsHeaderPadBottom, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:125"},
	{Node: "SkillRow.instructionsHeader", Prop: "padding-left", Official: 12, Impl: SkillInstructionsHeaderPadLeft, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:125"},
	{Node: "SkillRow.instructionsHeader", Prop: "border-bottom-width", Official: 0.5, Impl: SkillInstructionsHeaderBorderBottom, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:126"},
	{Node: "SkillRow.instructionsHeader", Prop: "font-size", Official: 11, Impl: SkillInstructionsHeaderFontSize, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:128"},
	{Node: "SkillRow.instructionsHeader", Prop: "font-weight", Official: 500, Impl: SkillInstructionsHeaderWeight, Unit: SkillUnitWeight, Status: SkillAligned, Anchor: "SkillRow.module.css:129"},
	{Node: "SkillRow.instructionsHeader", Prop: "line-height", Official: 16, Impl: SkillInstructionsHeaderLineHeight, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:130"},
	{Node: "SkillRow.instructionsHeader", Prop: "letter-spacing", Official: 0.04, Impl: SkillInstructionsHeaderLetterSpacing, Unit: SkillUnitEm, Status: SkillAligned, Anchor: "SkillRow.module.css:133"},

	// ── .instructions（:136-149）───────────────────────────────────
	{Node: "SkillRow.instructions", Prop: "min-height", Official: 0, Impl: SkillInstructionsMinHeight, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:137"},
	{Node: "SkillRow.instructions", Prop: "margin", Official: 0, Impl: SkillInstructionsMargin, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:138"},
	{Node: "SkillRow.instructions", Prop: "padding-top", Official: 10, Impl: SkillInstructionsPadTop, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:139"},
	{Node: "SkillRow.instructions", Prop: "padding-right", Official: 12, Impl: SkillInstructionsPadRight, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:139"},
	{Node: "SkillRow.instructions", Prop: "padding-bottom", Official: 12, Impl: SkillInstructionsPadBottom, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:139"},
	{Node: "SkillRow.instructions", Prop: "padding-left", Official: 12, Impl: SkillInstructionsPadLeft, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:139"},
	{Node: "SkillRow.instructions", Prop: "font-size", Official: 11, Impl: SkillInstructionsFontSize, Unit: SkillUnitPx, Status: SkillDerived, Anchor: "SkillRow.module.css:143", Formula: "font:var(--dsw-font-markdown-code-block-small) → var(--dsh-code-font-size,11px) / calc(19px + var(--dsh-code-font-delta)) = 11px/19px"},
	{Node: "SkillRow.instructions", Prop: "line-height", Official: 19, Impl: SkillInstructionsLineHeight, Unit: SkillUnitPx, Status: SkillDerived, Anchor: "SkillRow.module.css:143", Formula: "calc(19px + var(--dsh-code-font-delta, 0px))；实现取 delta=0"},

	// ── .instructions 滚动条（:151-159）────────────────────────────
	{Node: "SkillRow.instructions::-webkit-scrollbar-thumb", Prop: "border-width", Official: 2, Impl: SkillScrollbarThumbBorder, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:152"},
	{Node: "SkillRow.instructions::-webkit-scrollbar-thumb", Prop: "border-radius", Official: 8, Impl: SkillScrollbarThumbRadius, Unit: SkillUnitPx, Status: SkillDerived, Anchor: "SkillRow.module.css:154", Formula: "var(--dsw-radius-sm) = 8（pluginapi.Theme.Radii.SM）"},
	{Node: "SkillRow.instructions::-webkit-scrollbar-track", Prop: "margin-block", Official: 6, Impl: SkillScrollbarTrackMargin, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:158"},

	// ── .inspectButton（:161-178）──────────────────────────────────
	{Node: "SkillRow.inspectButton", Prop: "gap", Official: 4, Impl: SkillInspectGap, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:165"},
	{Node: "SkillRow.inspectButton", Prop: "margin-top", Official: 4, Impl: SkillInspectMarginTop, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:166"},
	{Node: "SkillRow.inspectButton", Prop: "margin-right", Official: 0, Impl: SkillInspectMarginRight, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:166"},
	{Node: "SkillRow.inspectButton", Prop: "margin-bottom", Official: 2, Impl: SkillInspectMarginBottom, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:166"},
	{Node: "SkillRow.inspectButton", Prop: "margin-left", Official: 4, Impl: SkillInspectMarginLeft, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:166"},
	{Node: "SkillRow.inspectButton", Prop: "padding-top", Official: 2, Impl: SkillInspectPadTop, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:167"},
	{Node: "SkillRow.inspectButton", Prop: "padding-right", Official: 8, Impl: SkillInspectPadRight, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:167"},
	{Node: "SkillRow.inspectButton", Prop: "padding-bottom", Official: 2, Impl: SkillInspectPadBottom, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:167"},
	{Node: "SkillRow.inspectButton", Prop: "padding-left", Official: 8, Impl: SkillInspectPadLeft, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:167"},
	{Node: "SkillRow.inspectButton", Prop: "border-width", Official: 0.5, Impl: SkillInspectBorderWidth, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:168"},
	{Node: "SkillRow.inspectButton", Prop: "border-radius", Official: 999, Impl: SkillInspectRadius, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:169"},
	{Node: "SkillRow.inspectButton", Prop: "font-size", Official: 11, Impl: SkillInspectFontSize, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:173"},
	{Node: "SkillRow.inspectButton", Prop: "line-height", Official: 16, Impl: SkillInspectLineHeight, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:174"},
	{Node: "SkillRow.inspectButton", Prop: "opacity", Official: 0, Impl: SkillInspectOpacity, Unit: SkillUnitRatio, Status: SkillAligned, Anchor: "SkillRow.module.css:176"},
	{Node: "SkillRow.inspectButton", Prop: "transition-duration", Official: 100, Impl: SkillInspectTransitionMs, Unit: SkillUnitMs, Status: SkillAligned, Anchor: "SkillRow.module.css:177"},
	{Node: "SkillRow.inspectButton:focus-visible", Prop: "opacity", Official: 1, Impl: SkillInspectFocusOpacity, Unit: SkillUnitRatio, Status: SkillAligned, Anchor: "SkillRow.module.css:181"},
	{Node: "SkillRow.card:hover .inspectButton", Prop: "opacity", Official: 1, Impl: SkillCardHoverInspectOpacity, Unit: SkillUnitRatio, Status: SkillAligned, Anchor: "SkillRow.module.css:186"},
	{Node: "SkillRow.inspectButton (hover:none)", Prop: "opacity", Official: 1, Impl: SkillInspectNoHoverOpacity, Unit: SkillUnitRatio, Status: SkillAligned, Anchor: "SkillRow.module.css:214"},

	// ── .visuallyHidden（:195-202）─────────────────────────────────
	{Node: "SkillRow.visuallyHidden", Prop: "width", Official: 1, Impl: SkillVisuallyHiddenWidth, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:197"},
	{Node: "SkillRow.visuallyHidden", Prop: "height", Official: 1, Impl: SkillVisuallyHiddenHeight, Unit: SkillUnitPx, Status: SkillAligned, Anchor: "SkillRow.module.css:198"},
}

// ── 非数值声明（display / position / flex / white-space …）───────────
//
// 这些没有「数值」，但**它们是几何的一部分**（`position:absolute` 决定
// chevronHover 压在同一格里、`white-space:nowrap` 决定 summary 不换行）。
// 不写进表里，实现就可以悄悄换一种布局而没人发现。

// SkillDeclRow 是一行非数值 CSS 声明。
//
// 与 `SkillGeoRow` 用**同一套**三态：非数值声明照样会"实现得不一样"
// （`inline-flex` 在 viva 里没有对应原语、`clip` 没有对应属性），
// 靠"官方值 == 实现值"一条判据会把合理的等价物逼成假值。
type SkillDeclRow struct {
	Node     string      // 官方节点
	Prop     string      // 属性
	Official string      // 官方值
	Impl     string      // 本仓实现值
	Status   SkillStatus // aligned / derived / deviation
	Anchor   string      // 官方 rc.2 源码锚点
	Note     string      // derived 写等价公式，deviation 写原因（aligned 可空）
}

// officialSkillDecls 是 `.module.css` 里**所有非数值声明**的逐条对照。
var officialSkillDecls = []SkillDeclRow{
	{Node: "SkillRow.card", Prop: "display", Official: "flex", Impl: "flex", Anchor: "SkillRow.module.css:4", Status: SkillAligned},
	{Node: "SkillRow.card", Prop: "flex-direction", Official: "column", Impl: "column", Anchor: "SkillRow.module.css:5", Status: SkillAligned},

	{Node: "SkillRow.row", Prop: "position", Official: "relative", Impl: "relative", Anchor: "SkillRow.module.css:9", Note: ".chevronHover 的 inset:0 以它为定位上下文", Status: SkillAligned},
	{Node: "SkillRow.row", Prop: "overflow", Official: "hidden", Impl: "hidden", Anchor: "SkillRow.module.css:10", Status: SkillAligned},
	{Node: "SkillRow.row", Prop: "display", Official: "flex", Impl: "flex", Anchor: "SkillRow.module.css:11", Status: SkillAligned},
	{Node: "SkillRow.row", Prop: "align-items", Official: "center", Impl: "center", Anchor: "SkillRow.module.css:12", Status: SkillAligned},
	{Node: "SkillRow.row", Prop: "transition-property", Official: "color", Impl: "color", Anchor: "SkillRow.module.css:16", Status: SkillAligned},
	{Node: "SkillRow.row[data-expandable]", Prop: "cursor", Official: "pointer", Impl: "pointer", Anchor: "SkillRow.module.css:26", Note: "只有 expandable（output !== null）才有这个属性", Status: SkillAligned},

	{Node: "SkillRow.leading", Prop: "position", Official: "relative", Impl: "relative", Anchor: "SkillRow.module.css:30", Status: SkillAligned},
	{Node: "SkillRow.leading", Prop: "flex", Official: "none", Impl: "none", Anchor: "SkillRow.module.css:31", Status: SkillAligned},
	{Node: "SkillRow.leading", Prop: "display", Official: "inline-flex", Impl: "flex", Status: SkillDerived, Anchor: "SkillRow.module.css:34", Note: "viva 没有 inline-flex 原语；ui.Row 在 flex 父里作为 flex item 排布，与 inline-flex 同形"},
	{Node: "SkillRow.leading", Prop: "align-items", Official: "center", Impl: "center", Anchor: "SkillRow.module.css:35", Status: SkillAligned},
	{Node: "SkillRow.leading", Prop: "justify-content", Official: "center", Impl: "center", Anchor: "SkillRow.module.css:36", Status: SkillAligned},
	{Node: "SkillRow.leading", Prop: "color", Official: "inherit", Impl: "inherit", Anchor: "SkillRow.module.css:38", Status: SkillAligned},

	{Node: "SkillRow.iconIdle", Prop: "display", Official: "inline-flex", Impl: "flex", Status: SkillDerived, Anchor: "SkillRow.module.css:51", Note: "同上：inline-flex → ui.Row(flex)"},
	{Node: "SkillRow.iconIdle", Prop: "transition-property", Official: "opacity", Impl: "opacity", Anchor: "SkillRow.module.css:53", Status: SkillAligned},
	{Node: "SkillRow.chevronHover", Prop: "position", Official: "absolute", Impl: "absolute", Anchor: "SkillRow.module.css:57", Note: "压在同一格上：图标与 chevron 不是并排，是叠放", Status: SkillAligned},
	{Node: "SkillRow.chevronHover", Prop: "inset", Official: "0", Impl: "0", Anchor: "SkillRow.module.css:58", Status: SkillAligned},
	{Node: "SkillRow.chevronHover", Prop: "margin", Official: "auto", Impl: "auto", Anchor: "SkillRow.module.css:59", Note: "inset:0 + margin:auto = 在 leading 盒里居中", Status: SkillAligned},
	{Node: "SkillRow.chevronHover", Prop: "transition-property", Official: "opacity", Impl: "opacity", Anchor: "SkillRow.module.css:61", Status: SkillAligned},

	{Node: "SkillRow.title", Prop: "flex", Official: "none", Impl: "none", Anchor: "SkillRow.module.css:75", Status: SkillAligned},
	{Node: "SkillRow.separator", Prop: "flex", Official: "none", Impl: "none", Anchor: "SkillRow.module.css:81", Status: SkillAligned},
	{Node: "SkillRow.summary", Prop: "flex", Official: "1 1 auto", Impl: "1 1 auto", Anchor: "SkillRow.module.css:90", Status: SkillAligned},
	{Node: "SkillRow.summary", Prop: "overflow", Official: "hidden", Impl: "hidden", Anchor: "SkillRow.module.css:93", Status: SkillAligned},
	{Node: "SkillRow.summary", Prop: "text-overflow", Official: "ellipsis", Impl: "ellipsis", Anchor: "SkillRow.module.css:94", Status: SkillAligned},
	{Node: "SkillRow.summary", Prop: "white-space", Official: "nowrap", Impl: "nowrap", Anchor: "SkillRow.module.css:94", Note: "摘要恒一行；换行由 MaxLines(1) 等价表达", Status: SkillAligned},

	{Node: "SkillRow.bodyWrap", Prop: "display", Official: "flex", Impl: "flex", Anchor: "SkillRow.module.css:108", Status: SkillAligned},
	{Node: "SkillRow.bodyWrap", Prop: "flex-direction", Official: "column", Impl: "column", Anchor: "SkillRow.module.css:109", Status: SkillAligned},

	{Node: "SkillRow.instructionsCard", Prop: "display", Official: "flex", Impl: "flex", Anchor: "SkillRow.module.css:113", Status: SkillAligned},
	{Node: "SkillRow.instructionsCard", Prop: "flex-direction", Official: "column", Impl: "column", Anchor: "SkillRow.module.css:114", Status: SkillAligned},
	{Node: "SkillRow.instructionsCard", Prop: "overflow", Official: "hidden", Impl: "hidden", Anchor: "SkillRow.module.css:117", Status: SkillAligned},

	{Node: "SkillRow.instructionsHeader", Prop: "flex", Official: "none", Impl: "none", Anchor: "SkillRow.module.css:124", Status: SkillAligned},
	{Node: "SkillRow.instructionsHeader", Prop: "text-transform", Official: "uppercase", Impl: "uppercase", Anchor: "SkillRow.module.css:132", Note: "官方把「说明 / Instructions」转成全大写——这是可见文案变换，实现必须做", Status: SkillAligned},

	{Node: "SkillRow.instructions", Prop: "overflow", Official: "auto", Impl: "auto", Anchor: "SkillRow.module.css:140", Status: SkillAligned},
	{Node: "SkillRow.instructions", Prop: "white-space", Official: "pre-wrap", Impl: "pre-wrap", Anchor: "SkillRow.module.css:141", Status: SkillAligned},
	{Node: "SkillRow.instructions", Prop: "overflow-wrap", Official: "anywhere", Impl: "anywhere", Anchor: "SkillRow.module.css:142", Status: SkillAligned},
	{Node: "SkillRow.instructions", Prop: "font-family", Official: "var(--ds-font-family-code)", Impl: "mono", Status: SkillDerived, Anchor: "SkillRow.module.css:143", Note: "font 简写的第三段 = 等宽字族；viva 对应 Box.Mono()"},
	{Node: "SkillRow.instructions", Prop: "font-weight", Official: "400", Impl: "400", Anchor: "SkillRow.module.css:143", Status: SkillAligned},

	{Node: "SkillRow.inspectButton", Prop: "display", Official: "inline-flex", Impl: "flex", Status: SkillDerived, Anchor: "SkillRow.module.css:162", Note: "同上：inline-flex → ui.Row(flex)"},
	{Node: "SkillRow.inspectButton", Prop: "align-self", Official: "flex-start", Impl: "flex-start", Anchor: "SkillRow.module.css:163", Status: SkillAligned},
	{Node: "SkillRow.inspectButton", Prop: "align-items", Official: "center", Impl: "center", Anchor: "SkillRow.module.css:164", Status: SkillAligned},
	{Node: "SkillRow.inspectButton", Prop: "corner-shape", Official: "round", Impl: "round", Anchor: "SkillRow.module.css:170", Note: "CSS corner-shape（圆角形状）；viva 的 Radius 本身就是圆角，等价", Status: SkillAligned},
	{Node: "SkillRow.inspectButton", Prop: "cursor", Official: "pointer", Impl: "pointer", Anchor: "SkillRow.module.css:175", Status: SkillAligned},
	{Node: "SkillRow.inspectButton", Prop: "transition-property", Official: "opacity", Impl: "opacity", Anchor: "SkillRow.module.css:177", Status: SkillAligned},

	{Node: "SkillRow.visuallyHidden", Prop: "position", Official: "absolute", Impl: "absolute", Anchor: "SkillRow.module.css:196", Status: SkillAligned},
	{Node: "SkillRow.visuallyHidden", Prop: "overflow", Official: "hidden", Impl: "hidden", Anchor: "SkillRow.module.css:199", Status: SkillAligned},
	{Node: "SkillRow.visuallyHidden", Prop: "clip", Official: "rect(0 0 0 0)", Impl: "absolute+1x1+opacity(0)", Status: SkillDerived, Anchor: "SkillRow.module.css:200", Note: "viva 没有 clip 属性；等价做法是 Absolute() + 1×1 + Opacity(0)（绝对定位不占位、1px 盒+全透明 = 不可见）"},

	{Node: "SkillRow (prefers-reduced-motion)", Prop: "transition", Official: "none", Impl: "none", Anchor: "SkillRow.module.css:204-211", Note: "四个过渡全关", Status: SkillAligned},
}

// ── 颜色 / 圆角令牌映射 ──────────────────────────────────────────────

// SkillTokenRow 是一行「官方 CSS 自定义属性 → 本仓 pluginapi.Theme 字段」。
//
// 为什么不合并进 `officialSkillGeometry`：几何断言比的是**数值**（16px 就是 16），
// 令牌比的是**名字**（哪个字段），两者断言方式不同，混在一起会两边都断不准。
type SkillTokenRow struct {
	// Var 是官方 CSS 自定义属性名（`--dsw-alias-…`）。
	Var string
	// Token 是本仓 `pluginapi.Theme` 的字段路径（`LabelTertiary` / `Radii.LG` …）。
	Token string
	// DarkLight 是官方暗/亮两套主题里该属性的原始值（`--dsw-static-*` 档位）。
	DarkLight string
	// Anchor 是官方源码锚点。
	Anchor string
	// Note 记差异（空 = 一一对应）。
	Note string
}

// officialSkillTokens 是 `.module.css` 里**每一个** `var(--…)` 的映射。
var officialSkillTokens = []SkillTokenRow{
	{Var: "--dsh-content-font-delta", Token: "(基准 0)", DarkLight: "calc(--dsh-content-font-size - 14px) = 0", Anchor: "dsh-client-ui-theme/lib/client.js", Note: "用户可调的字号增量；本仓没有该可调项，取默认 0"},
	{Var: "--dsh-content-font-size-secondary", Token: "Fonts.Secondary", DarkLight: "min(14-1, max(13, 14-2)) = 13", Anchor: "dsh-client-ui-theme/lib/client.js"},
	{Var: "--dsh-code-font-size", Token: "Fonts.Tiny", DarkLight: "11（默认档）", Anchor: "dsh-client-ui-theme/lib/client.js"},
	{Var: "--ds-font-family-code", Token: "(等宽字族)", DarkLight: "var(--ds-font-family-code)", Anchor: "dsh-client-ui-theme/lib/client.js"},

	{Var: "--dsw-alias-label-tertiary", Token: "LabelTertiary", DarkLight: "dark bluish-400 / light bluish-600", Anchor: "SkillRow.module.css:15"},
	{Var: "--dsw-alias-label-secondary", Token: "LabelSecondary", DarkLight: "dark bluish-300 / light bluish-700", Anchor: "SkillRow.module.css:21"},
	{Var: "--dsw-alias-label-caption", Token: "LabelCaption", DarkLight: "dark bluish-600 / light bluish-400", Anchor: "SkillRow.module.css:86"},
	{Var: "--dsw-alias-label-primary", Token: "LabelPrimary", DarkLight: "dark bluish-50 / light #000000", Anchor: "SkillRow.module.css:191"},
	{Var: "--dsw-alias-state-error-primary", Token: "Error", DarkLight: "dark red-600 / light red-400", Anchor: "SkillRow.module.css:100", Note: "本仓亮色 Error = red-600（官方亮色是 red-400）——这是**既有主题**的差异，不在本包内改；登记为 deviation"},
	{Var: "--dsw-alias-state-warn-label", Token: "WarnLabel", DarkLight: "dark amber-600 / light amber-600", Anchor: "SkillRow.module.css:104"},
	{Var: "--dsw-alias-border-l1", Token: "BorderL1", DarkLight: "dark #ffffff0f / light #0000000a", Anchor: "SkillRow.module.css:118"},
	{Var: "--dsw-alias-border-l2", Token: "BorderL2", DarkLight: "dark #ffffff1f / light #0000001a", Anchor: "SkillRow.module.css:126"},
	{Var: "--dsw-alias-border-l4", Token: "BorderL4", DarkLight: "dark #ffffff33 / light #00000029", Anchor: "SkillRow.module.css:168"},
	{Var: "--dsw-alias-bg-base", Token: "BgBase", DarkLight: "dark bluish-950 / light bluish-00", Anchor: "SkillRow.module.css:171"},
	{Var: "--dsw-alias-markdown-code-block", Token: "CodeBlock", DarkLight: "dark bluish-900 / light bluish-50", Anchor: "SkillRow.module.css:120"},
	{Var: "--dsw-alias-markdown-code-block-banner", Token: "CodeBanner", DarkLight: "dark bluish-850 / light bluish-50", Anchor: "SkillRow.module.css:127"},
	{Var: "--dsw-alias-interactive-bg-hover-solid", Token: "hoverSolidToken(t)", DarkLight: "dark bluish-800 / light bluish-75", Anchor: "SkillRow.module.css:190", Note: "本仓没有独立的 `-solid` 字段；两套主题各有一个同值字段（暗 BgLayer3 / 亮 NavHover），按主题取，见 hoverSolidToken"},
	{Var: "--dsw-radius-lg", Token: "Radii.LG", DarkLight: "16", Anchor: "SkillRow.module.css:119"},
	{Var: "--dsw-radius-sm", Token: "Radii.SM", DarkLight: "8", Anchor: "SkillRow.module.css:154"},
}

// ── 组件树（谁是谁的子节点）───────────────────────────────────────────

// SkillNode 是官方组件树里的一个节点。
type SkillNode struct {
	// Name 是节点路径（父在前）。根是 `SkillRow`。
	Name string
	// Parent 是父节点路径；根为空串。
	Parent string
	// Element 是官方 JSX 元素 / 组件名（`div.card`、`TextShimmer` …）。
	Element string
	// Anchor 是官方 rc.2 源码锚点 `SkillRow.tsx:行号`。
	Anchor string
	// When 是该节点出现的条件；空 = 恒在。
	When string
	// Note 记语义（空 = 无）。
	Note string
}

// officialSkillTree 是**官方 started 态的组件树**（`SkillRow.tsx:137-171`）。
//
// 树的形状是断言项：`leading` 是 `row` 的子节点、`title/separator/summary`
// 都在同一个 `TextShimmer` 里三个兄弟、`bodyWrap` 是 `card` 的**兄弟级**子节点
// 而不是 `row` 的子节点（展开体在整个 row 下面，不是嵌在行里）。
var officialSkillTree = []SkillNode{
	{Name: "SkillRow", Element: "div.card", Anchor: "SkillRow.tsx:138", Note: "data-tool=\"skill\" data-state={model.state}"},

	{Name: "SkillRow.row", Parent: "SkillRow", Element: "div.row", Anchor: "SkillRow.tsx:139-143", Note: "data-expandable={expandable || undefined}；expandable 时 role=button tabIndex=0 aria-expanded onClick onKeyDown"},

	{Name: "SkillRow.row.leading", Parent: "SkillRow.row", Element: "span.leading", Anchor: "SkillRow.tsx:144"},

	{Name: "SkillRow.row.leading.chevron", Parent: "SkillRow.row.leading", Element: "IconChevronDownOutlineRegular.chevron", Anchor: "SkillRow.tsx:77", When: "open", Note: "展开时 leading 里**只有** chevron"},
	{Name: "SkillRow.row.leading.iconIdle", Parent: "SkillRow.row.leading", Element: "span.iconIdle > IconSkillOutlineRegular(size=14)", Anchor: "SkillRow.tsx:78-82", When: "!open && expandable", Note: "静止态：skill 图标；悬停淡出换 chevron"},
	{Name: "SkillRow.row.leading.chevronHover", Parent: "SkillRow.row.leading", Element: "IconChevronDownOutlineRegular.chevron.chevronHover", Anchor: "SkillRow.tsx:83", When: "!open && expandable", Note: "与 iconIdle 叠放（absolute inset:0 margin:auto）"},
	{Name: "SkillRow.row.leading.icon", Parent: "SkillRow.row.leading", Element: "IconSkillOutlineRegular(size=14)", Anchor: "SkillRow.tsx:79", When: "!expandable", Note: "不可展开时只有图标，没有 chevron"},

	{Name: "SkillRow.row.status", Parent: "SkillRow.row", Element: "span.visuallyHidden", Anchor: "SkillRow.tsx:145", When: "state ∈ {running, error}", Note: "running → t('row.running')；error → t('row.failed')"},

	{Name: "SkillRow.row.shimmer", Parent: "SkillRow.row", Element: "TextShimmer", Anchor: "SkillRow.tsx:146", Note: "active={running}：只有 running 才闪"},

	{Name: "SkillRow.row.shimmer.title", Parent: "SkillRow.row.shimmer", Element: "span.title > TextShimmer", Anchor: "SkillRow.tsx:147", Note: "t('row.title') 外面又套一层 TextShimmer（官方原样，嵌套闪烁）"},
	{Name: "SkillRow.row.shimmer.separator", Parent: "SkillRow.row.shimmer", Element: "span.separator", Anchor: "SkillRow.tsx:148", Note: "data-shimmer-decoration aria-hidden"},
	{Name: "SkillRow.row.shimmer.summary", Parent: "SkillRow.row.shimmer", Element: "span.summary > TextShimmer", Anchor: "SkillRow.tsx:149-154", Note: "error 时加 .errorSummary、stopped 时加 .stoppedSummary"},

	{Name: "SkillRow.bodyWrap", Parent: "SkillRow", Element: "div.bodyWrap", Anchor: "SkillRow.tsx:158", When: "open", Note: "open = expanded && expandable"},

	{Name: "SkillRow.bodyWrap.instructionsCard", Parent: "SkillRow.bodyWrap", Element: "section.instructionsCard", Anchor: "SkillRow.tsx:159-162", Note: "aria-label={t('row.instructions')}"},
	{Name: "SkillRow.bodyWrap.instructionsCard.header", Parent: "SkillRow.bodyWrap.instructionsCard", Element: "div.instructionsHeader", Anchor: "SkillRow.tsx:160", Note: "t('row.instructions')；CSS text-transform:uppercase"},
	{Name: "SkillRow.bodyWrap.instructionsCard.instructions", Parent: "SkillRow.bodyWrap.instructionsCard", Element: "pre.instructions", Anchor: "SkillRow.tsx:161", Note: "data-error={state === 'error' || undefined}；正文 = model.output"},

	{Name: "SkillRow.bodyWrap.inspectButton", Parent: "SkillRow.bodyWrap", Element: "button.inspectButton", Anchor: "SkillRow.tsx:163-168", When: "inspect !== undefined", Note: "IconInspectOutlineRegular + t('row.inspect')"},
}

// officialSkillPreparingTree 是官方 **preparing 态**的组件树（`SkillRow.tsx:103-109`）。
//
// 它比 started 态少得多：没有 separator、没有 summary、没有展开体。
// 单独一张表是因为「preparing 时 summary 不出现」是一条会被实现忘掉的官方事实。
var officialSkillPreparingTree = []SkillNode{
	{Name: "SkillRow", Element: "div.card", Anchor: "SkillRow.tsx:103", Note: "data-tool=\"skill\" data-state=\"preparing\"（**写死**，不取四态）"},

	{Name: "SkillRow.row", Parent: "SkillRow", Element: "div.row", Anchor: "SkillRow.tsx:104", Note: "preparing 态没有 data-expandable，也没有 role/onClick"},
	{Name: "SkillRow.row.leading", Parent: "SkillRow.row", Element: "span.leading > IconSkillOutlineRegular(size=14)", Anchor: "SkillRow.tsx:105", Note: "只有图标，没有 chevron"},
	{Name: "SkillRow.row.preparing", Parent: "SkillRow.row", Element: "span.visuallyHidden", Anchor: "SkillRow.tsx:106", Note: "t('row.preparing')"},
	{Name: "SkillRow.row.shimmer", Parent: "SkillRow.row", Element: "TextShimmer.active > span.title", Anchor: "SkillRow.tsx:107", Note: "active 恒真；只有 title，没有 separator/summary"},
}

// ── 斜杠候选源的排序契约（`index.ts` §source）────────────────────────

// SkillTrigger 是官方斜杠源的触发表征（`index.ts` 的 `source` 字面量）。
const (
	// SkillTriggerChar 是官方 `trigger: '/'`。
	SkillTriggerChar = "/"
	// SkillTriggerName 是官方 `name: 'skill'`。
	SkillTriggerName = "skill"
	// SkillTriggerOrder 是官方 `order: 2`（与 commands 组同一套排序）。
	SkillTriggerOrder = 2
	// SkillUserTextRuleID 是官方 `skillReferenceRule.id`。
	SkillUserTextRuleID = "skill"
	// SkillUserTextRuleOrder 是官方 `skillReferenceRule.order`。
	SkillUserTextRuleOrder = 20
	// SkillUserTextNodePriority 是官方 `conversation.user.text.node` 的 priority。
	SkillUserTextNodePriority = 100
	// SkillLocaleNS 是官方 `NS = 'skill'`。
	SkillLocaleNS = "skill"
)

// hoverSolidToken 返回官方 `--dsw-alias-interactive-bg-hover-solid` 在本仓
// 主题里的等价色。
//
// 本仓 `pluginapi.Theme` 没有独立的 `-solid` 字段，但两套主题里各有一个
// **同值**的字段：
//
//	暗色  staticNeutralBluish800 = BgLayer3（暗色「抬高第三层」底）
//	亮色  staticNeutralBluish75  = NavHover（亮色侧栏悬停底）
//
// 直接写死 BgLayer3 会在亮色下偏成 bluish-00（比官方浅一档），
// 所以按 `t.Dark` 取。这是**等价代换**，不是另编一个颜色。
func hoverSolidToken(t *pluginapi.Theme) ui.Color {
	if t.Dark {
		return t.BgLayer3
	}
	return t.NavHover
}
