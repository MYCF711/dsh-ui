// official_transcript_geometry.go —— 主区 transcript 的**几何常量表**（rc.2 口径）。
//
// # 三条规矩（与 `internal/uikernel/official_geometry.go` 同一套）
//
//	① 结构树里的每个 gap / padding / min-max / 行高只有一个出处：本文件的常量。
//	   绘制代码（official_transcript_view.go）只准引用常量，不许再写魔法数。
//	② 「官方值」与「实现值」写在**同一张表**里（officialTranscriptGeometry 每一行），
//	   测试逐行核对：aligned 行实现值必须等于官方值；derived 行必须写 Formula
//	   （官方 calc/clamp 原文）；deviation 行必须写 Reason。
//	③ 官方值一律取自 rc.2 随包 `src/**` 的 CSS Module 实读原文，锚点写
//	   `pkg/src/…:line`。**不许从截图猜，不许沿用 rc.1 口径。**
//
// # 本次唯一的三类 deviation（都不是"差一点"，而是渲染器能力边界）
//
//	container-type:inline-size   Viva 没有容器查询 → 宽表格的 100cqw 突破用列宽代替
//	scrollbar-gutter: stable     Viva 没有滚动条槽位 → 滚动时列宽不预留
//	overflow-anchor: none        Viva 没有浏览器滚动锚定 → 该声明是 no-op（结构上等价）
//
// 除这三条外，主区 transcript 的纵向节奏与横向宽度轴全部 aligned/derived。
package dsconversation

import "github.com/MYCF711/dsh-ui/internal/uikernel"

// ── 纵向节奏（结构树 §6.1）────────────────────────────────────────────
const (
	// ChatFlowGapDefault 是行距默认值（`--dsh-chat-flow-gap`，ChatView.module.css:86）。
	ChatFlowGapDefault float32 = 4
	// ChatFlowGapResponse 是助手回复与邻居的行距（ChatView.module.css:91-98）。
	ChatFlowGapResponse float32 = 12
	// ChatFlowGapTurnHeader 是 Turn 头的行距（ChatView.module.css:140-144）。
	ChatFlowGapTurnHeader float32 = 16
	// ChatFlowGapSteeringBatch 是 steering batch 首行的行距基数
	// （`calc(44px + var(--dsh-content-font-delta, 0px))`，ChatView.module.css:155-157）。
	// 实现取 delta=0 的基准值；运行时按主题 delta 加偏。
	ChatFlowGapSteeringBatch float32 = 44
	// ChatFlowGapRunningSteering 是 `.running + [data-pending-steering]` 的强制行距
	// （ChatView.module.css:159-161）。
	ChatFlowGapRunningSteering float32 = 0

	// AssistantBlockGap 是助手正文块间距（AssistantMarkdown.module.css:17-22）。
	AssistantBlockGap float32 = 16
	// AssistantReasoningBlockGap 是 reasoning 分组内的块间距
	// （AssistantMarkdown.module.css:24-29）。
	AssistantReasoningBlockGap float32 = 4
	// TurnTailGap 是 Turn 尾根间距（TurnTailNodeView.module.css:1-5）。
	TurnTailGap float32 = 16
	// TurnTailActionsMarginTop 是 Turn 尾动作行上偏移（TurnTailNodeView.module.css:7-12）。
	TurnTailActionsMarginTop float32 = 4
	// AssistantActionsMarginTop 是助手动作行上偏移（AssistantMarkdown.module.css:84-88）。
	AssistantActionsMarginTop float32 = 16
	// AssistantActionsMarginLeft 是助手动作行左偏移（同上，-6px）。
	AssistantActionsMarginLeft float32 = -6
	// ProcessGroupMemberGap 是 process 组内成员间距（ChatGroupSeat.module.css:103/149）。
	ProcessGroupMemberGap float32 = 4
	// SubCallsGap 是子调用列间距（ToolCallTree.module.css:8-10）。
	SubCallsGap float32 = 4
	// SubCallsMarginLeft 是子调用列左缩进（ToolCallTree.module.css:8-10）。
	SubCallsMarginLeft float32 = 22
	// SubCallsPaddingLeft 是子调用列内缩进（ToolCallTree.module.css:8-10）。
	SubCallsPaddingLeft float32 = 8

	// ActionsRowHeight 是 footer 动作行高（`calc(28px + delta)`，MessageIconActions.module.css:9）。
	ActionsRowHeight float32 = 28
	// ActionsRowGap 是 footer 动作行内间距（MessageIconActions.module.css:5-10）。
	ActionsRowGap float32 = 8
	// ActionsButtonSize 是动作按钮边长（`28×28 + delta`，MessageIconActions.module.css:67-71）。
	ActionsButtonSize float32 = 28
	// ActionsButtonPad 是动作按钮内边距（MessageIconActions.module.css:67-71）。
	ActionsButtonPad float32 = 6
	// ActionsIconSize 是动作图标边长（`15×15 + delta`，MessageIconActions.module.css:79-87）。
	ActionsIconSize float32 = 15
	// ActionsClockEndIconSize 是收尾时钟图标边长（`17×17 + delta`，MessageIconActions.module.css:79-87）。
	ActionsClockEndIconSize float32 = 17
	// ActionsTimeStartPadRight 是用户时钟右内边距（MessageIconActions.module.css:13-19）。
	ActionsTimeStartPadRight float32 = 12
	// ActionsTimeEndFontSize 是助手时钟字号（`calc(13px - 1px)`，MessageIconActions.module.css:23-28）。
	ActionsTimeEndFontSize float32 = 12

	// StoppedFontSize 是中断标记字号（AssistantMarkdown.module.css:73-81）。
	StoppedFontSize float32 = 11
	// StoppedLineHeight 是中断标记行高（AssistantMarkdown.module.css:73-81）。
	StoppedLineHeight float32 = 18
	// StoppedPadX 是中断标记左右内边距（AssistantMarkdown.module.css:73-81）。
	StoppedPadX float32 = 6

	// TurnProcessHeight 是 Turn 过程头高度（`calc(33px + delta)`，TurnProcessNodeView.module.css:7-8）。
	TurnProcessHeight float32 = 33
	// TurnProcessPadBottom 是 Turn 过程头下内边距（TurnProcessNodeView.module.css:7-8）。
	TurnProcessPadBottom float32 = 8
	// TurnProcessChevron 是 Turn 过程头 chevron 边长（TurnProcessNodeView.module.css:30-41）。
	TurnProcessChevron float32 = 14

	// CompactionRowHeight 是压缩行高（`calc(24px + delta)`，MessageItem.module.css:33-39）。
	CompactionRowHeight float32 = 24
	// CompactionRowPadY 是压缩行上下内边距（MessageItem.module.css:33-39）。
	CompactionRowPadY float32 = 2
	// CompactionLeading 是压缩行前导盒边长（`16×16 + delta`，MessageItem.module.css:108-116）。
	CompactionLeading float32 = 16
	// CompactionLeadingGap 是压缩行前导右边距（MessageItem.module.css:108-116）。
	CompactionLeadingGap float32 = 6
	// CompactionSepSize 是压缩行分隔点边长（MessageItem.module.css:164-171）。
	CompactionSepSize float32 = 2
	// CompactionSepMargin 是压缩行分隔点左右边距（MessageItem.module.css:164-171）。
	CompactionSepMargin float32 = 8
	// CompactionBodyPadLeft 是压缩体左内边距基数（`calc(22px + delta)`，MessageItem.module.css:184-190）。
	CompactionBodyPadLeft float32 = 22

	// RetryRowFontSize 是重试行字号（MessageItem.module.css:192-196）。
	RetryRowFontSize float32 = 13
	// RetryRowLineHeight 是重试行行高（`calc(20px + delta-secondary)`，MessageItem.module.css:192-196）。
	RetryRowLineHeight float32 = 20
	// RetrySummaryPadY 是重试摘要上下内边距（MessageItem.module.css:198-210）。
	RetrySummaryPadY float32 = 2
	// RetrySummaryGap 是重试摘要内间距（MessageItem.module.css:198-210）。
	RetrySummaryGap float32 = 7
	// RetryDetailsGap 是重试明细行间距（MessageItem.module.css:254-262）。
	RetryDetailsGap float32 = 2
	// RetryDetailsMarginTop 是重试明细上边距（MessageItem.module.css:254-262）。
	RetryDetailsMarginTop float32 = 3
	// RetryDetailsPadLeft 是重试明细左内边距（MessageItem.module.css:254-262）。
	RetryDetailsPadLeft float32 = 14

	// TurnErrorDotColumn 是 Turn 错误行状态点列宽（MessageItem.module.css:268-276）。
	TurnErrorDotColumn float32 = 10
	// TurnErrorGap 是 Turn 错误行栅格间距（MessageItem.module.css:268-276）。
	TurnErrorGap float32 = 8
	// TurnErrorPadY 是 Turn 错误行上下内边距（MessageItem.module.css:268-276）。
	TurnErrorPadY float32 = 2
	// TurnErrorDotMarginTop 是 Turn 错误状态点上边距（MessageItem.module.css:278-280）。
	TurnErrorDotMarginTop float32 = 5
	// TurnErrorTitleMarginRight 是 Turn 错误标题右边距（MessageItem.module.css:287-291）。
	TurnErrorTitleMarginRight float32 = 6
)

// ── 横向（结构树 §6.2）────────────────────────────────────────────────
const (
	// UserRowGap 是用户行内间距（MessageItem.module.css:3-8）。
	UserRowGap float32 = 6
	// UserStackGap 是用户内容栈间距（MessageItem.module.css:10-21）。
	UserStackGap float32 = 8
	// UserStackWidthRatio 是用户气泡宽度系数（`chat-width * 0.702`，MessageItem.module.css:10-21）。
	UserStackWidthRatio float32 = 0.702
	// UserStackWidthCap 是用户气泡宽度上限（`82%`，MessageItem.module.css:10-21）。
	UserStackWidthCap float32 = 0.82
	// UserStackFallbackWidth 是 `--dsh-chat-content-width` 的兜底值（官方 748px）。
	UserStackFallbackWidth float32 = 748

	// AttachmentRowGap 是附件行间距（MessageItem.module.css:308-314）。
	AttachmentRowGap float32 = 8
	// FileCardWidth 是文件卡宽（MessageItem.module.css:316-328）。
	FileCardWidth float32 = 240
	// FileCardMinHeight 是文件卡最小高（MessageItem.module.css:316-328）。
	FileCardMinHeight float32 = 64
	// FileCardPadX 是文件卡左右内边距（MessageItem.module.css:316-328）。
	FileCardPadX float32 = 12
	// FileCardPadY 是文件卡上下内边距（MessageItem.module.css:316-328）。
	FileCardPadY float32 = 8
	// FileCardGap 是文件卡内间距（MessageItem.module.css:316-328）。
	FileCardGap float32 = 10
	// FileIconSize 是文件图标边长（MessageItem.module.css:330-334）。
	FileIconSize float32 = 28
	// FileNameFontSize 是文件名号（MessageItem.module.css:343-351）。
	FileNameFontSize float32 = 14
	// FileNameLineHeight 是文件名行高（MessageItem.module.css:343-351）。
	FileNameLineHeight float32 = 22
	// FileMetaFontSize 是文件元信息字号（MessageItem.module.css:353-360）。
	FileMetaFontSize float32 = 12
	// FileMetaLineHeight 是文件元信息行高（MessageItem.module.css:353-360）。
	FileMetaLineHeight float32 = 15
	// ReferenceSummaryFontSize 是引用摘要字号（MessageItem.module.css:22-26）。
	ReferenceSummaryFontSize float32 = 13
	// ReferenceSummaryLineHeight 是引用摘要行高（`18px + delta-secondary`，MessageItem.module.css:22-26）。
	ReferenceSummaryLineHeight float32 = 18

	// ImageTileSize 是图片降级瓦片边长（MessageImage.module.css:32-37）。
	ImageTileSize float32 = 64
	// ImageThumbnailSize 是图片缩略图边长（MessageImage.module.css:76-79）。
	ImageThumbnailSize float32 = 48
	// ImageSingleMaxEdge 是单图变体长边（MessageImage.tsx:36-48）。
	ImageSingleMaxEdge float32 = 240

	// ContextBodyWidthInset 是上下文展开体宽度扣减基数
	// （`calc(100% - 22px - delta)`，ContextInjectionRow.module.css:50-64）。
	ContextBodyWidthInset float32 = 22
	// ContextBodyMaxHeight 是上下文展开体高度上限（ContextInjectionRow.module.css:50-64）。
	ContextBodyMaxHeight float32 = 141
	// ContextBodyMarginTop 是上下文展开体上边距（ContextInjectionRow.module.css:50-64）。
	ContextBodyMarginTop float32 = 4
	// ContextBodyPaddingTop 是上下文展开体上内边距（ContextInjectionRow.module.css:50-64）。
	ContextBodyPaddingTop float32 = 10
	// ContextBodyPaddingRight 是上下文展开体右内边距（ContextInjectionRow.module.css:50-64）。
	ContextBodyPaddingRight float32 = 16
	// ContextBodyPaddingBottom 是上下文展开体下内边距（ContextInjectionRow.module.css:50-64）。
	ContextBodyPaddingBottom float32 = 12
	// ContextBodyPaddingLeft 是上下文展开体左内边距（ContextInjectionRow.module.css:50-64）。
	ContextBodyPaddingLeft float32 = 12
	// ContextOpenPadBottom 是上下文行展开时下内边距（ContextInjectionRow.module.css:8-10）。
	ContextOpenPadBottom float32 = 4
	// ContextSepSize 是上下文分隔点边长（ContextInjectionRow.module.css:18-25）。
	ContextSepSize float32 = 2
	// ContextSepMargin 是上下文分隔点左右边距（ContextInjectionRow.module.css:18-25）。
	ContextSepMargin float32 = 8

	// TurnTriggerHeaderPadX 是 Turn 触发头左右内边距（TurnTriggerNodeView.module.css:11-24）。
	TurnTriggerHeaderPadX float32 = 16
	// TurnTriggerHeaderPadY 是 Turn 触发头上下内边距（TurnTriggerNodeView.module.css:11-24）。
	TurnTriggerHeaderPadY float32 = 12
	// TurnTriggerHeaderGap 是 Turn 触发头内间距（TurnTriggerNodeView.module.css:11-24）。
	TurnTriggerHeaderGap float32 = 10
	// TurnTriggerBodyPadLeft 是 Turn 触发体左内边距（TurnTriggerNodeView.module.css:44）。
	TurnTriggerBodyPadLeft float32 = 40
	// TurnTriggerBodyPadBottom 是 Turn 触发体下内边距（TurnTriggerNodeView.module.css:44）。
	TurnTriggerBodyPadBottom float32 = 12
	// TurnTriggerContentMaxHeight 是 Turn 触发内容高度上限（TurnTriggerNodeView.module.css:46）。
	TurnTriggerContentMaxHeight float32 = 240

	// ToolBodyScrollMaxHeight 是工具展开体高度上限（ToolRow.module.css:238-241）。
	ToolBodyScrollMaxHeight float32 = 260
	// ToolIOCardMargin 是 IN/OUT 卡外边距（ToolRow.module.css:247-255）。
	ToolIOCardMargin float32 = 4
	// ToolIOSectionPadX 是 IN/OUT 段左右内边距（ToolRow.module.css:259-267）。
	ToolIOSectionPadX float32 = 16
	// ToolIOSectionPadY 是 IN/OUT 段上下内边距（ToolRow.module.css:259-267）。
	ToolIOSectionPadY float32 = 12
	// ToolIOSectionMaxHeight 是 IN/OUT 段高度上限（ToolRow.module.css:259-267）。
	ToolIOSectionMaxHeight float32 = 150
	// ToolIOSectionColumnGap 是 IN/OUT 段列间距（ToolRow.module.css:259-267）。
	ToolIOSectionColumnGap float32 = 14
	// ToolIOInputWidthRatio 是 IN/OUT 段输入列宽（官方 `max-content`，
	// 实现按内容的 max-content 近似取 30%）。
	ToolIOInputWidthRatio float32 = 0.3
	// ToolInspectPadX 是 inspect 胶囊左右内边距（ToolRow.module.css:185-204）。
	ToolInspectPadX float32 = 8
	// ToolInspectPadY 是 inspect 胶囊上下内边距（ToolRow.module.css:185-204）。
	ToolInspectPadY float32 = 2
	// ToolInspectOffset 是悬浮 inspect 胶囊的右上偏移（ToolRow.module.css:216-225）。
	ToolInspectOffset float32 = 12
	// ToolTerminalMaxHeight 是终端卡高度上限（ToolRow.module.css:370-376）。
	ToolTerminalMaxHeight float32 = 224
	// ToolTerminalLineHeight 是终端卡行高（ToolRow.module.css:370-376）。
	ToolTerminalLineHeight float32 = 18
	// ToolExitBadgeGap 是退出码徽标内间距（ExitCodeBadge.module.css:1-5）。
	ToolExitBadgeGap float32 = 5
	// ToolExitDotSize 是退出码状态点边长（ExitCodeBadge.module.css:7-14）。
	ToolExitDotSize float32 = 4

	// ApprovalMaxWidthFallback 是审批卡宽度上限的兜底（chat 宽未知时）。
	ApprovalMaxWidthFallback float32 = 748
	// ApprovalHeaderPadY 是审批卡头上内边距（ApprovalPanel.module.css:24）。
	ApprovalHeaderPadY float32 = 10
	// ApprovalHeaderPadX 是审批卡头左右内边距（ApprovalPanel.module.css:24）。
	ApprovalHeaderPadX float32 = 16
	// ApprovalBodyPadX 是审批卡体左右内边距（ApprovalPanel.module.css:34-38）。
	ApprovalBodyPadX float32 = 16
	// ApprovalBodyPadTop 是审批卡体上内边距（ApprovalPanel.module.css:34-38）。
	ApprovalBodyPadTop float32 = 12
	// ApprovalBodyGap 是审批卡体间距（ApprovalPanel.module.css:34-38）。
	ApprovalBodyGap float32 = 6

	// ToBottomSize 是回到底部按钮边长（ChatView.module.css:391-398）。
	ToBottomSize float32 = 34
	// ToBottomBottom 是回到底部按钮下偏移（ChatView.module.css:359-405）。
	ToBottomBottom float32 = 16

	// TurnRailWidth 是 Turn 导航轨宽（TurnNavigator.module.css:34）。
	TurnRailWidth float32 = 28
	// TurnRailRight 是 Turn 导航轨右偏移（TurnNavigator.module.css:34）。
	TurnRailRight float32 = 12
	// TurnMarkHeight 是 Turn 刻度高（TurnNavigator.module.css:84-95）。
	TurnMarkHeight float32 = 10
	// TurnMarkBarWidth 是 Turn 刻度条宽（TurnNavigator.module.css:97-129）。
	TurnMarkBarWidth float32 = 20
	// TurnMarkBarHeight 是 Turn 刻度条高（TurnNavigator.module.css:97-129）。
	TurnMarkBarHeight float32 = 2
	// TurnPreviewMaxWidth 是 Turn 预览卡宽度上限（TurnNavigator.module.css:154-175）。
	TurnPreviewMaxWidth float32 = 300
	// TurnPreviewMaxHeight 是 Turn 预览卡高度上限（TurnNavigator.module.css:154-175）。
	TurnPreviewMaxHeight float32 = 100
	// TurnPreviewPadX 是 Turn 预览卡左右内边距（TurnNavigator.module.css:154-175）。
	TurnPreviewPadX float32 = 12
	// TurnPreviewPadY 是 Turn 预览卡上下内边距（TurnNavigator.module.css:154-175）。
	TurnPreviewPadY float32 = 10

	// ChatScrollPad 是滚动口纵向内边距（ChatView.module.css:22-38）。
	ChatScrollPad float32 = 16
	// ChatScrollSideExtra 是滚动口侧向净空之外的额外 16px（ChatView.module.css:22-38）。
	ChatScrollSideExtra float32 = 16
	// ScrollBodyMarginRight 是宿主滚动口右外边距（ConversationRoot.module.css:485-500）。
	ScrollBodyMarginRight float32 = 2
	// ComposerSideClearance 是 composer 侧向净空（ConversationRoot.module.css:473）。
	ComposerSideClearance float32 = 16
)

// ── z-index 固定阶梯（结构树 §5.22，官方注释固化，实现不得改）───────────
const (
	// ZChatCodeBanner 是代码块 banner 的层级（第 1 档）。
	ZChatCodeBanner float32 = 6
	// ZChatStickyHeader 是钉住的压缩头层级（第 2 档，压过 banner）。
	ZChatStickyHeader float32 = 7
	// ZChatTurnRail 是 Turn 导航轨层级（第 2 档，与钉住头同层）。
	ZChatTurnRail float32 = 7
	// ZChatToBottom 是回到底部的层级（第 3 档）。
	ZChatToBottom float32 = 8
	// ZChatComposerSeat 是 composer seat 的层级（第 4 档，最高）。
	ZChatComposerSeat float32 = 9
)

// ZIndexRow 是 z-index 阶梯的一档。
type ZIndexRow struct {
	// Z 是官方层级值（6/7/8/9 四档，**固定**）。
	Z float32
	// Node 是这一档的节点（官方类名）。
	Node string
	// Anchor 是 rc.2 源码锚点。
	Anchor string
	// Note 记这一档与相邻档的关系（空 = 无）。
	Note string
}

// officialZIndexLadder 是官方 z-index 阶梯（**顺序即层级**，从低到高）。
//
// 为什么必须是一张表而不是常量堆：官方把它写成一条注释固化在
// `ConversationRoot.module.css:522-527`，四档是**一对一的先后关系**
// （banner 6 < 钉住头/rail 7 < 回到底部 8 < composer 9）。
// 拆成散装常量之后，"谁该压过谁"就只能靠读注释，改一个数字没人拦得住。
var officialZIndexLadder = []ZIndexRow{
	{Z: ZChatCodeBanner, Node: "代码块 banner（.md-code-banner / [data-code-block-banner]）", Anchor: "ConversationRoot.module.css:522-527", Note: "最低档：钉住的压缩头要压过它"},
	{Z: ZChatStickyHeader, Node: "钉住的压缩头（.compactionRow:has(.compactionBody) .compactionButton）", Anchor: "MessageItem.module.css:67-73", Note: "sticky top:0"},
	{Z: ZChatTurnRail, Node: "Turn 导航轨（TurnNavigator.slot）", Anchor: "TurnNavigator.module.css:2-15", Note: "与钉住头同档：两者不在同一层叠上下文里冲突"},
	{Z: ZChatToBottom, Node: "回到底部（.toBottomSlot）", Anchor: "ChatView.module.css:359-405", Note: "浮在 transcript 之上"},
	{Z: ZChatComposerSeat, Node: "composer seat", Anchor: "ConversationRoot.module.css:522-527", Note: "最高档：输入区永远压在消息流上"},
}

// OfficialZIndexLadder 返回官方 z-index 阶梯的只读副本（从低到高）。
func OfficialZIndexLadder() []ZIndexRow {
	return append([]ZIndexRow(nil), officialZIndexLadder...)
}

// ── 双层滚动口（结构树 §3.1）────────────────────────────────────────
// TranscriptScrollPort 是一层滚动口的声明。
type TranscriptScrollPort struct {
	// Name 是这一层的名字（host = 宿主滚动口，self = 自滚动口）。
	Name string
	// Selector 是官方选择器原文。
	Selector string
	// Anchor 是 rc.2 源码锚点。
	Anchor string
	// OverflowY 是官方 `overflow-y` 值。
	OverflowY string
	// Flex 是官方 `flex` 值。
	Flex string
	// MinHeight 是官方 `min-height` 值。
	MinHeight string
	// Note 记这一层的语义（空 = 无）。
	Note string
}

// officialScrollPorts 是**双层滚动口**的官方声明（顺序：宿主 → 自滚）。
//
// 判据（`use-chat-viewport.ts:118-119`）：
//
//	list.closest('[data-conversation-scroll]') ?? list
//
// 即「**外层有宿主滚动口就用它，没有就自己滚**」。
// 这不是可选优化：`ConversationRoot` 恒存在，所以线上恒走宿主滚动口；
// `.scroll` 的 `overflow-y:auto` 只在**独立挂载 Chat**（没有宿主滚动口）时生效。
var officialScrollPorts = []TranscriptScrollPort{
	{
		Name:      "host",
		Selector:  ".scrollBody[data-conversation-scroll]",
		Anchor:    "dsh-client-ui-conversation/src/client/skeleton/ConversationRoot.module.css:485-500；属性由 ConversationContent.tsx:202 打",
		OverflowY: "auto",
		Flex:      "1",
		MinHeight: "0",
		Note:      "宿主滚动口：flex:1;flex-direction:column;min-height:0;margin-right:2px;scrollbar-gutter:stable",
	},
	{
		Name:      "self",
		Selector:  ".scroll",
		Anchor:    "ChatView.module.css:22-38",
		OverflowY: "auto",
		Flex:      "1 1 auto",
		MinHeight: "0",
		Note:      "自滚动口：overflow-x:clip；padding:16px calc(clearance + 16px)",
	},
	{
		Name:      "self-downgraded",
		Selector:  ":global([data-conversation-scroll]) .scroll",
		Anchor:    "ChatView.module.css:57-64",
		OverflowY: "visible",
		Flex:      "0 0 auto",
		MinHeight: "auto",
		Note:      "降级态：宿主滚动口存在时自滚动口让出纵向滚动",
	},
}

// OfficialScrollPorts 返回双层滚动口的只读副本（host → self → self-downgraded）。
func OfficialScrollPorts() []TranscriptScrollPort {
	return append([]TranscriptScrollPort(nil), officialScrollPorts...)
}

// ── 官方值 ↔ 实现值总表 ─────────────────────────────────────────────
//
// 每行四件事：官方节点 / 属性 / 官方值 / 实现值；再加状态、锚点、公式（derived）、
// 原因（deviation）。测试逐行核对，**新增一行必须同时给出锚点**。
var officialTranscriptGeometry = []uikernel.GeoRow{
	// ── ChatView 容器与滚动口 ──────────────────────────────────────
	{Node: "ChatView.column", Prop: "width", Official: 100, Impl: 100, Unit: uikernel.UnitPct, Status: uikernel.GeoAligned, Anchor: "ChatView.module.css:69-76"},
	{Node: "ChatView.column", Prop: "max-width", Official: 748, Impl: 748, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: "ChatView.module.css:69-76", Formula: "var(--dsh-chat-content-width) = var(--dsh-chat-user-width, clamp(680px, var(--dsh-conversation-column-width, 0px) * 0.64, 920px))；实现取 748 基准档"},
	{Node: "ChatView.scroll", Prop: "padding-block", Official: 16, Impl: ChatScrollPad, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "ChatView.module.css:22-38"},
	{Node: "ChatView.scroll", Prop: "padding-inline", Official: 32, Impl: ComposerSideClearance + ChatScrollSideExtra, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: "ChatView.module.css:22-38", Formula: "calc(var(--dsh-composer-side-clearance) + 16px)"},
	{Node: "ChatView.flowItem", Prop: "min-width", Official: 0, Impl: 0, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "ChatView.module.css:102-104"},
	{Node: "ChatView.turnSpacer", Prop: "height", Official: 0, Impl: 0, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "ChatView.module.css:108-110"},
	{Node: "ChatView.flow", Prop: "gap-default", Official: ChatFlowGapDefault, Impl: ChatFlowGapDefault, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "ChatView.module.css:86"},
	{Node: "ChatView.flow", Prop: "gap-response", Official: ChatFlowGapResponse, Impl: ChatFlowGapResponse, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "ChatView.module.css:91-98"},
	{Node: "ChatView.flow", Prop: "gap-turn-header", Official: ChatFlowGapTurnHeader, Impl: ChatFlowGapTurnHeader, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "ChatView.module.css:140-144"},
	{Node: "ChatView.flow", Prop: "gap-steering-batch", Official: ChatFlowGapSteeringBatch, Impl: ChatFlowGapSteeringBatch, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: "ChatView.module.css:155-157", Formula: "calc(44px + var(--dsh-content-font-delta, 0px))；实现取 delta=0 基准值"},
	{Node: "ChatView.flow", Prop: "gap-running-steering", Official: ChatFlowGapRunningSteering, Impl: ChatFlowGapRunningSteering, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "ChatView.module.css:159-161"},
	{Node: "ConversationRoot.scrollBody", Prop: "margin-right", Official: ScrollBodyMarginRight, Impl: ScrollBodyMarginRight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-conversation/src/client/skeleton/ConversationRoot.module.css:485-500"},
	{Node: "ConversationRoot.scrollBody", Prop: "scrollbar-gutter", Official: 0, Impl: 0, Unit: uikernel.UnitPx, Status: uikernel.GeoDeviation, Anchor: "ConversationRoot.module.css:485-500", Reason: "Viva 没有滚动条槽位（scrollbar-gutter）；滚动时列宽不预留，属渲染器能力边界"},
	{Node: "ChatView.scroll", Prop: "container-type", Official: 0, Impl: 0, Unit: uikernel.UnitPx, Status: uikernel.GeoDeviation, Anchor: "ChatView.module.css:37", Reason: "Viva 没有 CSS 容器查询；md-table-wide 的 100cqw 突破用列宽代替（见 AssistantMarkdown 宽表格行）"},
	{Node: "ChatView.scroll", Prop: "overflow-anchor", Official: 0, Impl: 0, Unit: uikernel.UnitPx, Status: uikernel.GeoDeviation, Anchor: "ChatView.module.css:45-48", Reason: "Viva 没有浏览器滚动锚定（overflow-anchor）；声明是 no-op，结构上等价"},

	// ── 用户消息与附件（§5.1）─────────────────────────────────────
	{Node: "UserMessageNodeView.userRow", Prop: "gap", Official: UserRowGap, Impl: UserRowGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:3-8"},
	{Node: "UserStyleBubble.userStack", Prop: "gap", Official: UserStackGap, Impl: UserStackGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:10-21"},
	{Node: "UserStyleBubble.userStack", Prop: "max-width-ratio", Official: UserStackWidthRatio, Impl: UserStackWidthRatio, Unit: uikernel.UnitRatio, Status: uikernel.GeoDerived, Anchor: "MessageItem.module.css:10-21", Formula: "min(calc(var(--dsh-chat-content-width, 748px) * 0.702), 82%)"},
	{Node: "UserStyleBubble.userStack", Prop: "max-width-cap", Official: UserStackWidthCap, Impl: UserStackWidthCap, Unit: uikernel.UnitRatio, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:10-21"},
	{Node: "UserContent.attachmentRow", Prop: "gap", Official: AttachmentRowGap, Impl: AttachmentRowGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:308-314"},
	{Node: "UserContent.fileCard", Prop: "width", Official: FileCardWidth, Impl: FileCardWidth, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:316-328"},
	{Node: "UserContent.fileCard", Prop: "min-height", Official: FileCardMinHeight, Impl: FileCardMinHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:316-328"},
	{Node: "UserContent.fileCard", Prop: "padding-inline", Official: FileCardPadX, Impl: FileCardPadX, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:316-328"},
	{Node: "UserContent.fileCard", Prop: "padding-block", Official: FileCardPadY, Impl: FileCardPadY, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:316-328"},
	{Node: "UserContent.fileCard", Prop: "gap", Official: FileCardGap, Impl: FileCardGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:316-328"},
	{Node: "UserContent.fileIcon", Prop: "size", Official: FileIconSize, Impl: FileIconSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:330-334"},
	{Node: "UserContent.fileName", Prop: "font-size", Official: FileNameFontSize, Impl: FileNameFontSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:343-351"},
	{Node: "UserContent.fileName", Prop: "line-height", Official: FileNameLineHeight, Impl: FileNameLineHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:343-351"},
	{Node: "UserContent.fileMeta", Prop: "font-size", Official: FileMetaFontSize, Impl: FileMetaFontSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:353-360"},
	{Node: "UserContent.fileMeta", Prop: "line-height", Official: FileMetaLineHeight, Impl: FileMetaLineHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:353-360"},
	{Node: "UserContent.referenceSummary", Prop: "font-size", Official: ReferenceSummaryFontSize, Impl: ReferenceSummaryFontSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:22-26"},
	{Node: "MessageImage.tile", Prop: "size", Official: ImageTileSize, Impl: ImageTileSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-attachment/src/MessageImage.module.css:32-37"},
	{Node: "MessageImage.thumbnail", Prop: "size", Official: ImageThumbnailSize, Impl: ImageThumbnailSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-attachment/src/MessageImage.module.css:76-79"},

	// ── 助手正文（§5.6）───────────────────────────────────────────
	{Node: "AssistantNodeView.root", Prop: "font-size", Official: 14, Impl: 14, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "AssistantMarkdown.module.css:9-15"},
	{Node: "AssistantNodeView.root", Prop: "line-height", Official: 24, Impl: 24, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: "AssistantMarkdown.module.css:9-15", Formula: "calc(24px + var(--dsh-content-font-delta, 0px))；实现取 delta=0"},
	{Node: "AssistantMarkdown.body", Prop: "gap", Official: AssistantBlockGap, Impl: AssistantBlockGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "AssistantMarkdown.module.css:17-22"},
	{Node: "AssistantMarkdown.body[reasoning]", Prop: "gap", Official: AssistantReasoningBlockGap, Impl: AssistantReasoningBlockGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "AssistantMarkdown.module.css:24-29"},
	{Node: "AssistantMarkdown.stopped", Prop: "font-size", Official: StoppedFontSize, Impl: StoppedFontSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "AssistantMarkdown.module.css:73-81"},
	{Node: "AssistantMarkdown.stopped", Prop: "line-height", Official: StoppedLineHeight, Impl: StoppedLineHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "AssistantMarkdown.module.css:73-81"},
	{Node: "AssistantMarkdown.stopped", Prop: "padding-inline", Official: StoppedPadX, Impl: StoppedPadX, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "AssistantMarkdown.module.css:73-81"},
	{Node: "AssistantMarkdown.actions", Prop: "margin-top", Official: AssistantActionsMarginTop, Impl: AssistantActionsMarginTop, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "AssistantMarkdown.module.css:84-88"},
	{Node: "AssistantMarkdown.actions", Prop: "margin-left", Official: AssistantActionsMarginLeft, Impl: AssistantActionsMarginLeft, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "AssistantMarkdown.module.css:84-88"},

	// ── footer 元信息（§5.21，user / assistant / turn-tail 三处共用）──
	{Node: "MessageIconActions.actions", Prop: "height", Official: ActionsRowHeight, Impl: ActionsRowHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: "MessageIconActions.module.css:5-10", Formula: "calc(28px + var(--dsh-content-font-delta, 0px))；实现取 delta=0"},
	{Node: "MessageIconActions.action", Prop: "size", Official: ActionsButtonSize, Impl: ActionsButtonSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageIconActions.module.css:63-75"},
	{Node: "MessageIconActions.action", Prop: "padding", Official: ActionsButtonPad, Impl: ActionsButtonPad, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageIconActions.module.css:63-75"},
	{Node: "MessageIconActions.action svg", Prop: "size", Official: ActionsIconSize, Impl: ActionsIconSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageIconActions.module.css:79-87"},
	{Node: "MessageIconActions.action[data-clock=end] svg", Prop: "size", Official: ActionsClockEndIconSize, Impl: ActionsClockEndIconSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageIconActions.module.css:79-87"},
	{Node: "MessageIconActions.timeStart", Prop: "padding-right", Official: ActionsTimeStartPadRight, Impl: ActionsTimeStartPadRight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageIconActions.module.css:13-19"},
	{Node: "MessageIconActions.timeEnd", Prop: "font-size", Official: ActionsTimeEndFontSize, Impl: ActionsTimeEndFontSize, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: "MessageIconActions.module.css:23-28", Formula: "calc(13px - 1px)"},

	// ── 工具卡（§5.7）─────────────────────────────────────────────
	{Node: "ToolCallTree.callRow", Prop: "border-radius", Official: 8, Impl: 8, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/ToolCallTree.module.css:1-3"},
	{Node: "ToolCallTree.subCalls", Prop: "gap", Official: SubCallsGap, Impl: SubCallsGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/ToolCallTree.module.css:5-12"},
	{Node: "ToolCallTree.subCalls", Prop: "margin-left", Official: SubCallsMarginLeft, Impl: SubCallsMarginLeft, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/ToolCallTree.module.css:5-12"},
	{Node: "ToolCallTree.subCalls", Prop: "padding-left", Official: SubCallsPaddingLeft, Impl: SubCallsPaddingLeft, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/ToolCallTree.module.css:5-12"},
	{Node: "ToolRow.summary", Prop: "font-size", Official: 13, Impl: 13, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/components/ToolRow.module.css:35-43"},
	{Node: "ToolRow.bodyScroll", Prop: "max-height", Official: ToolBodyScrollMaxHeight, Impl: ToolBodyScrollMaxHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/components/ToolRow.module.css:238-241"},
	{Node: "ToolRow.ioCard", Prop: "margin", Official: ToolIOCardMargin, Impl: ToolIOCardMargin, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/components/ToolRow.module.css:247-255"},
	{Node: "ToolRow.ioSection", Prop: "padding-inline", Official: ToolIOSectionPadX, Impl: ToolIOSectionPadX, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/components/ToolRow.module.css:259-267"},
	{Node: "ToolRow.ioSection", Prop: "padding-block", Official: ToolIOSectionPadY, Impl: ToolIOSectionPadY, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/components/ToolRow.module.css:259-267"},
	{Node: "ToolRow.ioSection", Prop: "max-height", Official: ToolIOSectionMaxHeight, Impl: ToolIOSectionMaxHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/components/ToolRow.module.css:259-267"},
	{Node: "ToolRow.ioSection", Prop: "column-gap", Official: ToolIOSectionColumnGap, Impl: ToolIOSectionColumnGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/components/ToolRow.module.css:259-267"},
	{Node: "ToolRow.ioSection", Prop: "grid-template-columns", Official: ToolIOInputWidthRatio, Impl: ToolIOInputWidthRatio, Unit: uikernel.UnitRatio, Status: uikernel.GeoDerived, Anchor: "dsh-client-ui-tool/src/client/tool/components/ToolRow.module.css:259-267", Formula: "max-content 1fr；实现按列宽 30% 近似 max-content"},
	{Node: "ToolRow.inspectButton", Prop: "padding-inline", Official: ToolInspectPadX, Impl: ToolInspectPadX, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/components/ToolRow.module.css:185-204"},
	{Node: "ToolRow.inspectButton", Prop: "padding-block", Official: ToolInspectPadY, Impl: ToolInspectPadY, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/components/ToolRow.module.css:185-204"},
	{Node: "ToolRow.detailsBodyWrap .inspectButton", Prop: "top", Official: ToolInspectOffset, Impl: ToolInspectOffset, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/components/ToolRow.module.css:216-225"},
	{Node: "ToolRow.terminalBody", Prop: "max-height", Official: ToolTerminalMaxHeight, Impl: ToolTerminalMaxHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/components/ToolRow.module.css:370-376"},
	{Node: "ToolRow.terminalBody", Prop: "line-height", Official: ToolTerminalLineHeight, Impl: ToolTerminalLineHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/components/ToolRow.module.css:370-376"},
	{Node: "ExitCodeBadge.badge", Prop: "gap", Official: ToolExitBadgeGap, Impl: ToolExitBadgeGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/components/ExitCodeBadge.module.css:1-5"},
	{Node: "ExitCodeBadge.dot", Prop: "size", Official: ToolExitDotSize, Impl: ToolExitDotSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-tool/src/client/tool/components/ExitCodeBadge.module.css:7-14"},

	// ── 审批卡（§5.22 几何摘要）───────────────────────────────────
	{Node: "ApprovalPanel.panel", Prop: "max-width", Official: ApprovalMaxWidthFallback, Impl: ApprovalMaxWidthFallback, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: "dsh-client-ui-approval/src/client/ApprovalPanel.module.css:11", Formula: "var(--dsh-chat-content-width)；实现取 748 兜底"},
	{Node: "ApprovalPanel.header", Prop: "padding-block", Official: ApprovalHeaderPadY, Impl: ApprovalHeaderPadY, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-approval/src/client/ApprovalPanel.module.css:24"},
	{Node: "ApprovalPanel.header", Prop: "padding-inline", Official: ApprovalHeaderPadX, Impl: ApprovalHeaderPadX, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-approval/src/client/ApprovalPanel.module.css:24"},
	{Node: "ApprovalPanel.body", Prop: "padding-inline", Official: ApprovalBodyPadX, Impl: ApprovalBodyPadX, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-approval/src/client/ApprovalPanel.module.css:34-38"},
	{Node: "ApprovalPanel.body", Prop: "padding-top", Official: ApprovalBodyPadTop, Impl: ApprovalBodyPadTop, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-approval/src/client/ApprovalPanel.module.css:34-38"},
	{Node: "ApprovalPanel.body", Prop: "gap", Official: ApprovalBodyGap, Impl: ApprovalBodyGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "dsh-client-ui-approval/src/client/ApprovalPanel.module.css:34-38"},

	// ── 上下文注入 / 系统提示（§5.3 / §5.16）────────────────────────
	{Node: "ContextInjectionRow.root[data-open]", Prop: "padding-bottom", Official: ContextOpenPadBottom, Impl: ContextOpenPadBottom, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "ContextInjectionRow.module.css:8-10"},
	{Node: "ContextInjectionRow.sep", Prop: "size", Official: ContextSepSize, Impl: ContextSepSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "ContextInjectionRow.module.css:18-25"},
	{Node: "ContextInjectionRow.sep", Prop: "margin-inline", Official: ContextSepMargin, Impl: ContextSepMargin, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "ContextInjectionRow.module.css:18-25"},
	{Node: "ContextInjectionRow.body", Prop: "max-height", Official: ContextBodyMaxHeight, Impl: ContextBodyMaxHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "ContextInjectionRow.module.css:50-64"},
	{Node: "ContextInjectionRow.body", Prop: "width-inset", Official: ContextBodyWidthInset, Impl: ContextBodyWidthInset, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: "ContextInjectionRow.module.css:50-64", Formula: "calc(100% - 22px - var(--dsh-content-font-delta, 0px))；实现取 delta=0"},
	{Node: "ContextInjectionRow.body", Prop: "padding-block", Official: ContextBodyPaddingTop, Impl: ContextBodyPaddingTop, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "ContextInjectionRow.module.css:50-64"},
	{Node: "ContextInjectionRow.body", Prop: "font-size", Official: 11, Impl: 11, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "ContextInjectionRow.module.css:63"},
	{Node: "ContextInjectionRow.body", Prop: "line-height", Official: 16, Impl: 16, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "ContextInjectionRow.module.css:63"},

	// ── Turn 触发 / 过程 / 尾（§5.4 / §5.14 / §5.15）────────────────
	{Node: "TurnTriggerNodeView.header", Prop: "padding-inline", Official: TurnTriggerHeaderPadX, Impl: TurnTriggerHeaderPadX, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnTriggerNodeView.module.css:11-24"},
	{Node: "TurnTriggerNodeView.header", Prop: "padding-block", Official: TurnTriggerHeaderPadY, Impl: TurnTriggerHeaderPadY, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnTriggerNodeView.module.css:11-24"},
	{Node: "TurnTriggerNodeView.header", Prop: "gap", Official: TurnTriggerHeaderGap, Impl: TurnTriggerHeaderGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnTriggerNodeView.module.css:11-24"},
	{Node: "TurnTriggerNodeView.body", Prop: "padding-left", Official: TurnTriggerBodyPadLeft, Impl: TurnTriggerBodyPadLeft, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnTriggerNodeView.module.css:44"},
	{Node: "TurnTriggerNodeView.body", Prop: "padding-bottom", Official: TurnTriggerBodyPadBottom, Impl: TurnTriggerBodyPadBottom, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnTriggerNodeView.module.css:44"},
	{Node: "TurnTriggerNodeView.content", Prop: "max-height", Official: TurnTriggerContentMaxHeight, Impl: TurnTriggerContentMaxHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnTriggerNodeView.module.css:46"},
	{Node: "TurnProcessNodeView.root", Prop: "height", Official: TurnProcessHeight, Impl: TurnProcessHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: "TurnProcessNodeView.module.css:7-8", Formula: "calc(33px + var(--dsh-content-font-delta, 0px))；实现取 delta=0"},
	{Node: "TurnProcessNodeView.root", Prop: "padding-bottom", Official: TurnProcessPadBottom, Impl: TurnProcessPadBottom, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnProcessNodeView.module.css:7-8"},
	{Node: "TurnProcessNodeView.chevron", Prop: "size", Official: TurnProcessChevron, Impl: TurnProcessChevron, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnProcessNodeView.module.css:30-41"},
	{Node: "TurnTailNodeView.root", Prop: "gap", Official: TurnTailGap, Impl: TurnTailGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnTailNodeView.module.css:1-5"},
	{Node: "TurnTailNodeView.actions", Prop: "margin-top", Official: TurnTailActionsMarginTop, Impl: TurnTailActionsMarginTop, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnTailNodeView.module.css:7-12"},

	// ── 压缩 / 重试 / Turn 错误（§5.10 / §5.11 / §5.12）─────────────
	{Node: "CompactionItem.compactionRow", Prop: "height", Official: CompactionRowHeight, Impl: CompactionRowHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: "MessageItem.module.css:33-39", Formula: "calc(24px + var(--dsh-content-font-delta, 0px))；实现取 delta=0"},
	{Node: "CompactionItem.compactionRow", Prop: "padding-block", Official: CompactionRowPadY, Impl: CompactionRowPadY, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:33-39"},
	{Node: "CompactionItem.compactionLeading", Prop: "size", Official: CompactionLeading, Impl: CompactionLeading, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:108-116"},
	{Node: "CompactionItem.compactionLeading", Prop: "margin-right", Official: CompactionLeadingGap, Impl: CompactionLeadingGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:108-116"},
	{Node: "CompactionItem.compactionSep", Prop: "size", Official: CompactionSepSize, Impl: CompactionSepSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:164-171"},
	{Node: "CompactionItem.compactionSep", Prop: "margin-inline", Official: CompactionSepMargin, Impl: CompactionSepMargin, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:164-171"},
	{Node: "CompactionItem.compactionBody", Prop: "padding-left", Official: CompactionBodyPadLeft, Impl: CompactionBodyPadLeft, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: "MessageItem.module.css:184-190", Formula: "calc(22px + var(--dsh-content-font-delta, 0px))；实现取 delta=0"},
	{Node: "ModelRetryItem.retryRow", Prop: "font-size", Official: RetryRowFontSize, Impl: RetryRowFontSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:192-196"},
	{Node: "ModelRetryItem.retryRow", Prop: "line-height", Official: RetryRowLineHeight, Impl: RetryRowLineHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: "MessageItem.module.css:192-196", Formula: "calc(20px + var(--dsh-content-font-delta-secondary, 0px))；实现取 delta=0"},
	{Node: "ModelRetryItem.retrySummary", Prop: "padding-block", Official: RetrySummaryPadY, Impl: RetrySummaryPadY, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:198-210"},
	{Node: "ModelRetryItem.retrySummary", Prop: "gap", Official: RetrySummaryGap, Impl: RetrySummaryGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:198-210"},
	{Node: "ModelRetryItem.retryDetails", Prop: "gap", Official: RetryDetailsGap, Impl: RetryDetailsGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:254-262"},
	{Node: "ModelRetryItem.retryDetails", Prop: "margin-top", Official: RetryDetailsMarginTop, Impl: RetryDetailsMarginTop, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:254-262"},
	{Node: "ModelRetryItem.retryDetails", Prop: "padding-left", Official: RetryDetailsPadLeft, Impl: RetryDetailsPadLeft, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:254-262"},
	{Node: "TurnErrorItem.turnErrorRow", Prop: "grid-column-1", Official: TurnErrorDotColumn, Impl: TurnErrorDotColumn, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:268-276"},
	{Node: "TurnErrorItem.turnErrorRow", Prop: "gap", Official: TurnErrorGap, Impl: TurnErrorGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:268-276"},
	{Node: "TurnErrorItem.turnErrorRow", Prop: "padding-block", Official: TurnErrorPadY, Impl: TurnErrorPadY, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:268-276"},
	{Node: "TurnErrorItem.turnErrorDot", Prop: "margin-top", Official: TurnErrorDotMarginTop, Impl: TurnErrorDotMarginTop, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:278-280"},
	{Node: "TurnErrorItem.turnErrorTitle", Prop: "margin-right", Official: TurnErrorTitleMarginRight, Impl: TurnErrorTitleMarginRight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:287-291"},

	// ── 浮动控件（§5.22 / §5.23）───────────────────────────────────
	{Node: "ChatView.toBottomSlot", Prop: "bottom", Official: ToBottomBottom, Impl: ToBottomBottom, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "ChatView.module.css:359-405"},
	{Node: "ChatView.toBottom", Prop: "size", Official: ToBottomSize, Impl: ToBottomSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "ChatView.module.css:391-398"},
	{Node: "TurnNavigator.frame", Prop: "width", Official: TurnRailWidth, Impl: TurnRailWidth, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnNavigator.module.css:34"},
	{Node: "TurnNavigator.frame", Prop: "right", Official: TurnRailRight, Impl: TurnRailRight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnNavigator.module.css:21-46"},
	{Node: "TurnNavigator.mark", Prop: "height", Official: TurnMarkHeight, Impl: TurnMarkHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnNavigator.module.css:84-95"},
	{Node: "TurnNavigator.mark::before", Prop: "width", Official: TurnMarkBarWidth, Impl: TurnMarkBarWidth, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnNavigator.module.css:97-129"},
	{Node: "TurnNavigator.mark::before", Prop: "height", Official: TurnMarkBarHeight, Impl: TurnMarkBarHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnNavigator.module.css:97-129"},
	{Node: "TurnNavigator.preview", Prop: "max-width", Official: TurnPreviewMaxWidth, Impl: TurnPreviewMaxWidth, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: "TurnNavigator.module.css:154-175", Formula: "min(300px, calc(100cqw - 120px))；Viva 无容器查询，取 300px 上限"},
	{Node: "TurnNavigator.preview", Prop: "max-height", Official: TurnPreviewMaxHeight, Impl: TurnPreviewMaxHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnNavigator.module.css:154-175"},
	{Node: "TurnNavigator.preview", Prop: "padding-inline", Official: TurnPreviewPadX, Impl: TurnPreviewPadX, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnNavigator.module.css:154-175"},
	{Node: "TurnNavigator.preview", Prop: "padding-block", Official: TurnPreviewPadY, Impl: TurnPreviewPadY, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "TurnNavigator.module.css:154-175"},

	// ── z-index 阶梯（四档，见 officialZIndexLadder）─────────────────
	{Node: "ChatView.codeBanner", Prop: "z-index", Official: ZChatCodeBanner, Impl: ZChatCodeBanner, Unit: uikernel.UnitZIndex, Status: uikernel.GeoAligned, Anchor: "ConversationRoot.module.css:522-527"},
	{Node: "CompactionItem.stickyHeader", Prop: "z-index", Official: ZChatStickyHeader, Impl: ZChatStickyHeader, Unit: uikernel.UnitZIndex, Status: uikernel.GeoAligned, Anchor: "MessageItem.module.css:67-73"},
	{Node: "TurnNavigator.slot", Prop: "z-index", Official: ZChatTurnRail, Impl: ZChatTurnRail, Unit: uikernel.UnitZIndex, Status: uikernel.GeoAligned, Anchor: "TurnNavigator.module.css:2-15"},
	{Node: "ChatView.toBottomSlot", Prop: "z-index", Official: ZChatToBottom, Impl: ZChatToBottom, Unit: uikernel.UnitZIndex, Status: uikernel.GeoAligned, Anchor: "ChatView.module.css:359-405"},
	{Node: "ConversationRoot.composerSeat", Prop: "z-index", Official: ZChatComposerSeat, Impl: ZChatComposerSeat, Unit: uikernel.UnitZIndex, Status: uikernel.GeoAligned, Anchor: "ConversationRoot.module.css:522-527"},
	{Node: "ChatView.zIndexLayers", Prop: "stacking", Official: 0, Impl: 0, Unit: uikernel.UnitPx, Status: uikernel.GeoDeviation, Anchor: "ConversationRoot.module.css:522-527", Reason: "Viva 没有 CSS z-index；四档层叠顺序由画布绘制顺序（PopupLayer）表达，顺序本身由 officialZIndexLadder 断言守住"},
}

// OfficialTranscriptGeometry 返回官方几何总表的只读副本。
//
// **只读**：绘制代码不得修改它；新增/调整行必须同时补锚点，
// 否则 `official_transcript_geometry_test.go` 的"每行必须有锚点"会红。
func OfficialTranscriptGeometry() []uikernel.GeoRow {
	return append([]uikernel.GeoRow(nil), officialTranscriptGeometry...)
}

// TranscriptGeometryCount 是几何表行数（测试与报告引用同一个数，避免各数一遍）。
func TranscriptGeometryCount() int { return len(officialTranscriptGeometry) }
