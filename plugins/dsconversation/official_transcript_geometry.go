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

// ── markdown 块级与代码块（**整节 derived**，结构树 §9 第 1/2 条）─────────
//
// 为什么整节都是 derived、一行 aligned 都没有：
//
//	官方 `MarkdownText` / `FenceCodeBlock` / `CodeBlock` 全部在
//	`@deepseek-ai/dsh-client-ui-primitives` 里，而**该包没有 rc.2 `src/`**
//	（结构树 §9 第 1 条：对 asar 全路径检索 `ui-primitives` = 0 命中，
//	`dsh/node_modules/@deepseek-ai/**` 里也没有包目录）。
//
// 三条纪律（不是"以后再补"，是现在就必须守住）：
//
//	① 下面每个值都标 `derived` 且写 `Formula`（推导依据），
//	   `official_transcript_geometry_test.go` 逐行核对；没有一个值伪装成 aligned。
//	② **不得反推像素**：这些数不是从截图里量出来的，也**不许**拿它们反推官方几何、
//	   不许拿去改 `internal/uikernel` 的壳几何、不许写进结构树当"官方值"。
//	   它们是**本实现的选择**，只为让人看到"这里为什么是 20 而不是 18"。
//	③ 取到 primitives `src/` 后逐行换锚（derived → aligned + 官方行号），
//	   并把本段注释改掉。在此之前任何"官方代码块 banner 是 32px"的说法都是假的。
//
// 唯一能锚的两处（已取证，作推导基准）：正文 14/24（`AssistantMarkdown.module.css:9-15`
// 的 `--dsh-content-font-size` + `calc(24px + delta)`）与块间距 16px
// （`AssistantMarkdown.module.css:17-22` 的 `.body { gap: 16px }`）。
const (
	// MarkdownListIndentPerLevel 是无序/有序列表**每级**的缩进（derived）。
	MarkdownListIndentPerLevel float32 = 20
	// MarkdownListMarkerGap 是列表标记与正文之间的间距（derived）。
	MarkdownListMarkerGap float32 = 6
	// MarkdownListMarkerWidth 是列表标记槽宽度（derived；"99." 要放得下）。
	MarkdownListMarkerWidth float32 = 18
	// MarkdownListBullet 是无序列表的标记（与官方 MarkdownText 同字符，derived）。
	MarkdownListBullet = "•"

	// MarkdownQuoteBorderWidth 是引用块左边框宽度（derived）。
	MarkdownQuoteBorderWidth float32 = 2
	// MarkdownQuotePadLeft 是引用块左内边距（derived）。
	MarkdownQuotePadLeft float32 = 12

	// MarkdownRuleHeight 是分隔线粗细（derived；1 物理像素）。
	MarkdownRuleHeight float32 = 1

	// CodeBlockBannerHeight 是代码块 banner 高度（derived；mdlite 默认同值）。
	CodeBlockBannerHeight float32 = 32
	// CodeBlockBannerFontSize 是 banner 语言标签字号（derived；对齐 --dsw-font-xxs-12 档）。
	CodeBlockBannerFontSize float32 = 12
	// CodeBlockPadX 是代码块左右内边距（derived）。
	CodeBlockPadX float32 = 10
	// CodeBlockPadY 是代码块上下内边距（derived）。
	CodeBlockPadY float32 = 8
	// CodeBlockRadius 是代码块圆角（derived）。
	CodeBlockRadius float32 = 6
	// CodeBlockMeasureMinWidth 是**测量宽度**的下限（derived）。
	//
	// 为什么要有下限：布局阶段拿不到列宽（viva 的 `f.Bounds()` 是整窗），
	// 只能按 `env.MainWidth` 减滚动口内边距估算；窗口极窄时估算值会掉到
	// 几十像素，横滚条算出来的高度没有意义。120 是"还能看两个字符"的下限。
	CodeBlockMeasureMinWidth float32 = 120
)

// markdownAnchor 是 markdown 块级/代码块那一整段的**证据锚点**。
//
// 它指向的不是官方 CSS 行号，而是结构树的「未取证」条目本身：
// `structure/official-conversation-transcript-tree.md:647-654`（§9 第 1/2 条，
// 明确写 primitives 没有 rc.2 `src/`、代码块与 markdown 块级几何未取证）。
//
// 为什么几何表里要允许这种锚：几何表的规矩是"每行必须有锚点、锚点带行号"。
// 这几行的**唯一**官方事实就是"官方没有可锚的行号"——把这件事写进 Anchor，
// 比编一个 `primitives/CodeBlock.module.css:12` 诚实。测试只校验"有锚点且带冒号"，
// 不校验它是不是 CSS（结构树自己也不是 CSS）。
const markdownAnchor = "official-conversation-transcript-tree.md:647-654（§9 第 1/2 条：primitives 无 rc.2 src，块级几何未取证）"

// MarkdownHeading 是一级标题的字号/行高（h1–h6 的阶梯）。
type MarkdownHeading struct {
	// Level 是标题级别（1–6）。
	Level int
	// Size 是字号（px，derived）。
	Size float32
	// Line 是行高（px，derived）。
	Line float32
}

// markdownHeadings 是标题阶梯（索引 0 = h1）。
//
// derived 依据：官方正文 14/24 已取证，标题阶梯在 primitives 里未取证。
// 这里按「正文的 1.43 / 1.29 / 1.14 / 1.07 / 1 / 1 倍」推出（20 / 18 / 16 / 15 / 14 / 14），
// 行高取 `size + 8`（与正文 14/24 的同一比例）。
//
// **不得反推像素**：这六组数不是官方值，取到 primitives `src/` 后整表换锚。
var markdownHeadings = [6]MarkdownHeading{
	{Level: 1, Size: 20, Line: 28},
	{Level: 2, Size: 18, Line: 26},
	{Level: 3, Size: 16, Line: 24},
	{Level: 4, Size: 15, Line: 23},
	{Level: 5, Size: 14, Line: 22},
	{Level: 6, Size: 14, Line: 22},
}

// MarkdownHeadingFor 返回某一级标题的样式。
//
// 越界按最近一级夹紧（官方 clamp(h1..h6) 语义）：0 和负数按 h1，>6 按 h6。
// 这样调用方不必在分派处再判一次边界。
func MarkdownHeadingFor(level int) MarkdownHeading {
	if level < 1 {
		level = 1
	}
	if level > len(markdownHeadings) {
		level = len(markdownHeadings)
	}
	return markdownHeadings[level-1]
}

// MarkdownHeadings 返回标题阶梯的只读副本（测试与报告用同一个出处）。
func MarkdownHeadings() []MarkdownHeading {
	return append([]MarkdownHeading(nil), markdownHeadings[:]...)
}

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

// ── 工作流运行面板（结构树 §5.20，官方 `WorkflowRunPanel.module.css`）──
//
// 两个来源，全部实读、带行号：
//
//	① `dsh-client-ui-workflow-run/src/client/WorkflowRunPanel.module.css`
//	   sha256 `fa04126bf3cd35ed9ba3bc24d8255dc9e602dbd586647ce53753d0542ef67316`
//	② primitives 的 CSS（`DisclosureRow` / `StateDot`）——
//	   `dsh-web-frontend/dist/assets/index-CSAdOBx6.css` L1（单行压缩）
//	   sha256 `d487cca8d8a97f1e0cc5eca02343528c873e544eb7ec3a5b323a0f035553551d`
//
// 为什么 primitives 的几何是 **aligned** 而不是 derived：本仓既有注释
// （`official_transcript_view.go:383`）写的是"`ui-primitives` 没有 rc.2 src"，
// 那是对的——**包目录确实不存在**；但预构建 Web bundle 里有完整的
// CSS-module 产物。既然能实读，就不该继续标 derived。
//
// 合并规则（重要）：面板把自己的类名当 `rowClassName` / `leadingClassName` /
// `titleClassName` 传进 `DisclosureRow`，两层类名**同时命中**。下面每一行的
// 取值都是**合并后的生效值**，注释里写清哪一层覆盖哪一层。
const (
	// WorkflowRunRootMinWidth 是 `.root { min-width:0 }`（WorkflowRunPanel.module.css:1-4）。
	WorkflowRunRootMinWidth float32 = 0

	// WorkflowRunHeaderGap 是 `.runHeader { gap:6px }` / `.phaseHeader { gap:6px }`
	// （:6-17 / :72-80）。**覆盖** primitives `.row` 的默认间距——DisclosureRow
	// 自己不给 gap，靠 leading 的 margin-right 6 撑；面板改成行 gap 6 并把
	// leading 的 margin-right 归零（见下）。
	WorkflowRunHeaderGap float32 = 6
	// WorkflowRunHeaderHeight 是 `.runHeader` / `.phaseHeader` 的 `height:32px`
	// （:6-17 / :72-80）。**覆盖** primitives `.row` 的 24px。
	WorkflowRunHeaderHeight float32 = 32
	// WorkflowRunHeaderPadX 是 `.runHeader { padding:0 8px }`（:6-17）。
	WorkflowRunHeaderPadX float32 = 8
	// WorkflowRunFocusOutlineOffset 是 `:focus-visible { outline-offset:-2px }`
	// （:19-22 / :82-86）。负偏移 = 描边画在盒内，行高不变。
	WorkflowRunFocusOutlineOffset float32 = -2

	// WorkflowRunLeadingSize 是 `.runLeading` / `.phaseLeading` 的 16×16
	// （`calc(16px + var(--dsh-content-font-delta, 0px))`，:24-33 / :88-97）。
	WorkflowRunLeadingSize float32 = 16
	// WorkflowRunLeadingMarginRight 是 `.runLeading { margin-right:0 }`
	// （:24-33）。**覆盖** primitives `.leading` 的 6px——间距改由行 gap 给。
	WorkflowRunLeadingMarginRight float32 = 0
	// WorkflowRunChevronSize 是 primitives `.leading svg:not([data-state])` 的
	// 14×14（`calc(14px + delta)`，index-CSAdOBx6.css L1:6996）。**不是** viewBox 16。
	WorkflowRunChevronSize float32 = 14

	// WorkflowRunTitleMaxWidth 是 `.runTitle` / `.phaseTitle` 的 `max-width:42%`
	// （:35-45 / :99-109）。
	WorkflowRunTitleMaxWidth float32 = 42
	// WorkflowRunTitleWeight 是 `.runTitle` 的 `font-weight:510`（:35-45）。
	// 510 是可变字重的中间档（不是 500/600），Viva 按 float 透传。
	WorkflowRunTitleWeight float32 = 510
	// WorkflowRunRowLineHeight 是 `.runTitle` / `.phaseTitle` 的行高
	// `calc(24px + var(--dsh-content-font-delta, 0px))` = 24（delta 默认 0）。
	WorkflowRunRowLineHeight float32 = 24
	// WorkflowRunRowMinHeight 是 primitives `.row` 的
	// `height:calc(24px + var(--dsh-content-font-delta, 0px))` = 24
	// （index-CSAdOBx6.css L1:6355）。被面板的 32 覆盖，只对**非面板**行生效。
	WorkflowRunRowMinHeight float32 = 24

	// WorkflowRunSummaryLineHeight 是 `.runSummary` 的行高
	// `calc(18px + var(--dsh-content-font-delta-secondary, 0px))` = 18（:47-56）。
	WorkflowRunSummaryLineHeight float32 = 18

	// WorkflowRunStatusTailHeight 是 `.statusTail { height:20px }`（:58-70）。
	WorkflowRunStatusTailHeight float32 = 20
	// WorkflowRunStatusTailFontSize 是 `.statusTail { font-size:11px }`（:58-70）。
	WorkflowRunStatusTailFontSize float32 = 11
	// WorkflowRunStatusTailLineHeight 是 `.statusTail { line-height:16px }`（:58-70）。
	WorkflowRunStatusTailLineHeight float32 = 16
	// WorkflowRunStatusTailGap 是 `.statusTail { gap:4px }`（:58-70）。
	WorkflowRunStatusTailGap float32 = 4

	// WorkflowRunSeparatorSize 是 `.separator` 的 2×2（:136-143）。
	WorkflowRunSeparatorSize float32 = 2

	// WorkflowRunPhaseCountLineHeight 是 `.phaseCount` / `.phaseStatus` 的行高
	// `calc(20px + var(--dsh-content-font-delta-secondary, 0px))` = 20（:111-134）。
	WorkflowRunPhaseCountLineHeight float32 = 20
	// WorkflowRunPhaseStatusWidth 是 `.phaseStatus { width:calc(132px + delta*10) }`
	// = 132（delta 默认 0）（:122-134）。
	WorkflowRunPhaseStatusWidth float32 = 132

	// WorkflowRunPhaseListGap 是 `.phaseList { gap:4px }`（:145-151）。
	WorkflowRunPhaseListGap float32 = 4
	// WorkflowRunPhaseListPadTop 是 `.phaseList { padding:4px 0 0 16px }` 的上边。
	WorkflowRunPhaseListPadTop float32 = 4
	// WorkflowRunPhaseListPadLeft 是 `.phaseList` 的左内边距 16。
	WorkflowRunPhaseListPadLeft float32 = 16
	// WorkflowRunPhaseListPadLeftNarrow 是 `@media (max-width:560px)` 下的左内边距
	// （:267-272）。Viva 没有媒体查询 → 见 deviation 行。
	WorkflowRunPhaseListPadLeftNarrow float32 = 12
	// WorkflowRunNarrowMaxWidth 是上面那条媒体查询的断点 560px。
	WorkflowRunNarrowMaxWidth float32 = 560

	// WorkflowRunMembersGap 是 `.members { gap:2px }`（:157-163）。
	WorkflowRunMembersGap float32 = 2
	// WorkflowRunMembersPadLeft 是 `.members { padding:0 0 0 16px }`（:157-163）。
	WorkflowRunMembersPadLeft float32 = 16

	// WorkflowRunMemberGap 是 `.memberRow, .memberButton { gap:12px }`（:165-180）。
	WorkflowRunMemberGap float32 = 12
	// WorkflowRunMemberMinHeight 是 `.memberRow { min-height:calc(24px + delta) }` = 24
	// （:165-180）。
	WorkflowRunMemberMinHeight float32 = 24
	// WorkflowRunMemberLabelWeight 是 `.memberButton .memberLabel { font-weight:500 }`
	// （:186-190）。**只有可导航行**加粗。
	WorkflowRunMemberLabelWeight float32 = 500
	// WorkflowRunMemberUnderlineOffset 是 `.memberButton:hover .memberLabel` 的
	// `text-underline-offset:3px`（:192-202）。
	WorkflowRunMemberUnderlineOffset float32 = 3

	// WorkflowRunDotSlotWidth 是 `.dotSlot { width:16px }`（:204-212）。
	WorkflowRunDotSlotWidth float32 = 16
	// WorkflowRunDotSlotHeight 是 `.dotSlot { height:calc(24px + delta) }` = 24（:204-212）。
	WorkflowRunDotSlotHeight float32 = 24

	// WorkflowRunMemberLabelWrapHeight 是 `.memberLabelWrap { height:calc(24px + delta) }`
	// = 24（:223-232）。
	WorkflowRunMemberLabelWrapHeight float32 = 24
	// WorkflowRunMemberLabelWrapPadX 是 `.memberLabelWrap { padding:0 2px }`（:223-232）。
	WorkflowRunMemberLabelWrapPadX float32 = 2
	// WorkflowRunMemberLabelFontSize 是 `.memberLabel { font-size:14px }`
	// （`var(--dsh-content-font-size, 14px)`，:234-246）——**正文档，不是 secondary**。
	WorkflowRunMemberLabelFontSize float32 = 14
	// WorkflowRunMemberStatusWidth 是 `.memberStatus { width:64px }`（:248-258）。
	WorkflowRunMemberStatusWidth float32 = 64

	// WorkflowRunEmptyLineHeight 是 `.empty { line-height:calc(20px + delta-secondary) }` = 20
	// （:260-265）。
	WorkflowRunEmptyLineHeight float32 = 20

	// WorkflowRunHeaderContentGap 是 `.headerContentLayout { gap:6px }`（:278-280）。
	WorkflowRunHeaderContentGap float32 = 6

	// WorkflowRunDotSize 是 `StateDot` 非 ongoing 分支的边长 10
	// （`index-ghOpC9uV.js` L129:82467 的 `t ?? (e === "ongoing" ? 14 : 10)`）。
	WorkflowRunDotSize float32 = 10
	// WorkflowRunDotSizeOngoing 是 ongoing 分支的边长 14（同上）。
	WorkflowRunDotSizeOngoing float32 = 14
	// WorkflowRunDotCoreSize 是实心点直径：10 × (1 − 2×20%) = 6
	// （`._dot_:after { inset:20%; border-radius:50% }`，index-CSAdOBx6.css L1:2851）。
	WorkflowRunDotCoreSize float32 = 6
	// WorkflowRunDotInsetPercent 是上面那条 `inset:20%`（百分比量纲）。
	WorkflowRunDotInsetPercent float32 = 20
	// WorkflowRunSpinnerViewBox 是 ongoing spinner 的 viewBox 边长 24（L129:82598）。
	WorkflowRunSpinnerViewBox float32 = 24
	// WorkflowRunSpinnerRadius 是 spinner 两圆的 r=9.5（L129:82598）。
	WorkflowRunSpinnerRadius float32 = 9.5
	// WorkflowRunSpinnerStroke 是 spinner 的 `stroke-width:2`（L1:3446）。
	WorkflowRunSpinnerStroke float32 = 2
	// WorkflowRunSpinnerDurationMs 是 `animation: … 1.5s linear infinite` 的 1500ms
	// （L1:3338）。**Viva 画布静态 → 见 deviation 行**。
	WorkflowRunSpinnerDurationMs float32 = 1500
)

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

	// ── markdown 块级（§5.6；**全部 derived**：primitives 无 rc.2 src）───
	//
	// 锚点统一指结构树的「未取证」条目本身——这是**证据的缺席**的锚，
	// 不是官方行号。任何人看到这些行都该知道：值是本实现的选择。
	{Node: "MarkdownText.h1", Prop: "font-size", Official: 20, Impl: 20, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "官方无 rc.2 src 可锚（primitives 缺包）；按正文 14px × 1.43 推出，**不得反推像素**"},
	{Node: "MarkdownText.h1", Prop: "line-height", Official: 28, Impl: 28, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "font-size + 8（与正文 14/24 同比例），**不得反推像素**"},
	{Node: "MarkdownText.h2", Prop: "font-size", Official: 18, Impl: 18, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "正文 14px × 1.29，**不得反推像素**"},
	{Node: "MarkdownText.h2", Prop: "line-height", Official: 26, Impl: 26, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "font-size + 8，**不得反推像素**"},
	{Node: "MarkdownText.h3", Prop: "font-size", Official: 16, Impl: 16, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "正文 14px × 1.14，**不得反推像素**"},
	{Node: "MarkdownText.h3", Prop: "line-height", Official: 24, Impl: 24, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "font-size + 8，**不得反推像素**"},
	{Node: "MarkdownText.h4", Prop: "font-size", Official: 15, Impl: 15, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "正文 14px × 1.07，**不得反推像素**"},
	{Node: "MarkdownText.h4", Prop: "line-height", Official: 23, Impl: 23, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "font-size + 8，**不得反推像素**"},
	{Node: "MarkdownText.h5", Prop: "font-size", Official: 14, Impl: 14, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "与正文同档（14px），只靠字重区分，**不得反推像素**"},
	{Node: "MarkdownText.h5", Prop: "line-height", Official: 22, Impl: 22, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "与正文同档（22px），**不得反推像素**"},
	{Node: "MarkdownText.h6", Prop: "font-size", Official: 14, Impl: 14, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "与 h5 同档（14px），**不得反推像素**"},
	{Node: "MarkdownText.h6", Prop: "line-height", Official: 22, Impl: 22, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "与 h5 同档（22px），**不得反推像素**"},
	{Node: "MarkdownText.ul/ol", Prop: "padding-inline-start", Official: MarkdownListIndentPerLevel, Impl: MarkdownListIndentPerLevel, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "每级 20px 累加（level × 20）；官方列表缩进未取证，**不得反推像素**"},
	{Node: "MarkdownText.li", Prop: "marker-gap", Official: MarkdownListMarkerGap, Impl: MarkdownListMarkerGap, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "标记与正文间距 6px（与 DisclosureRow 的 leading-gap 6 同档），**不得反推像素**"},
	{Node: "MarkdownText.li", Prop: "marker-width", Official: MarkdownListMarkerWidth, Impl: MarkdownListMarkerWidth, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "标记槽 18px（容得下 \"99.\"），**不得反推像素**"},
	{Node: "MarkdownText.blockquote", Prop: "border-inline-start-width", Official: MarkdownQuoteBorderWidth, Impl: MarkdownQuoteBorderWidth, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "左边框 2px（Viva 无 border-inline-start，用 Row 里的竖线盒表达），**不得反推像素**"},
	{Node: "MarkdownText.blockquote", Prop: "padding-inline-start", Official: MarkdownQuotePadLeft, Impl: MarkdownQuotePadLeft, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "左内边距 12px，**不得反推像素**"},
	{Node: "MarkdownText.hr", Prop: "height", Official: MarkdownRuleHeight, Impl: MarkdownRuleHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "1 物理像素（与 flex.Frame.Divider 同值），**不得反推像素**"},

	// ── 代码块（§5.6；**全部 derived**：FenceCodeBlock 无 rc.2 src）──────
	{Node: "FenceCodeBlock.banner", Prop: "height", Official: CodeBlockBannerHeight, Impl: CodeBlockBannerHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "banner 32px（mdlite.DefaultCodeBlockStyle 默认同值）；官方 banner 高度未取证，**不得反推像素**"},
	{Node: "FenceCodeBlock.banner", Prop: "font-size", Official: CodeBlockBannerFontSize, Impl: CodeBlockBannerFontSize, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "语言标签 12px（对齐 --dsw-font-xxs-12 档），**不得反推像素**"},
	{Node: "FenceCodeBlock.body", Prop: "padding-inline", Official: CodeBlockPadX, Impl: CodeBlockPadX, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "左右 10px（mdlite 默认同值），**不得反推像素**"},
	{Node: "FenceCodeBlock.body", Prop: "padding-block", Official: CodeBlockPadY, Impl: CodeBlockPadY, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "上下 8px（mdlite 默认同值），**不得反推像素**"},
	{Node: "FenceCodeBlock.card", Prop: "border-radius", Official: CodeBlockRadius, Impl: CodeBlockRadius, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "圆角 6px（Viva 无 --dsw-radius 令牌解析，取 SM 档），**不得反推像素**"},
	{Node: "FenceCodeBlock.scroll", Prop: "min-measure-width", Official: CodeBlockMeasureMinWidth, Impl: CodeBlockMeasureMinWidth, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: markdownAnchor, Formula: "布局期测量宽度下限 120px（列宽在 Draw 阶段拿不到，按 env.MainWidth 减滚动口内边距估算），**不得反推像素**"},

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

	// ── 工作流运行面板（§5.20）─────────────────────────────────────
	//
	// 锚点两族：
	//
	//	`WorkflowRunPanel.module.css`  → 官方包 src（sha256 fa04126b…）
	//	`index-CSAdOBx6.css`           → primitives 预构建 CSS（sha256 d487cca8…）
	//
	// 面板的类名与 primitives 的类名**同时命中**（前者当 rowClassName 等传入），
	// 所以每一行都是合并后的生效值；注释写清谁覆盖谁。
	{Node: "WorkflowRunPanel.root", Prop: "min-width", Official: 0, Impl: WorkflowRunRootMinWidth, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:1-4"},
	{Node: "WorkflowRunPanel.runHeader", Prop: "gap", Official: 6, Impl: WorkflowRunHeaderGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:6-17"},
	{Node: "WorkflowRunPanel.runHeader", Prop: "height", Official: 32, Impl: WorkflowRunHeaderHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:6-17"},
	{Node: "WorkflowRunPanel.runHeader", Prop: "padding-inline", Official: 8, Impl: WorkflowRunHeaderPadX, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:6-17"},
	{Node: "WorkflowRunPanel.runHeader", Prop: "border-radius", Official: 12, Impl: 12, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:6-17 → var(--dsw-radius-md)"},
	{Node: "WorkflowRunPanel.runHeader:focus-visible", Prop: "outline-offset", Official: -2, Impl: WorkflowRunFocusOutlineOffset, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:19-22"},
	{Node: "WorkflowRunPanel.runLeading", Prop: "width", Official: 16, Impl: WorkflowRunLeadingSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:24-33 → calc(16px + delta)"},
	{Node: "WorkflowRunPanel.runLeading", Prop: "height", Official: 16, Impl: WorkflowRunLeadingSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:24-33 → calc(16px + delta)"},
	{Node: "WorkflowRunPanel.runLeading", Prop: "margin-right", Official: 0, Impl: WorkflowRunLeadingMarginRight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:24-33（覆盖 primitives .leading 的 6px）"},
	{Node: "DisclosureRow.leading>svg", Prop: "size", Official: 14, Impl: WorkflowRunChevronSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "index-CSAdOBx6.css:1 → ._leading_l4nv8_52 svg:not([data-state]) calc(14px + delta)"},
	{Node: "WorkflowRunPanel.runTitle", Prop: "max-width", Official: 42, Impl: WorkflowRunTitleMaxWidth, Unit: uikernel.UnitPct, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:35-45"},
	{Node: "WorkflowRunPanel.runTitle", Prop: "font-weight", Official: 510, Impl: WorkflowRunTitleWeight, Unit: uikernel.UnitRatio, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:35-45"},
	{Node: "WorkflowRunPanel.runTitle", Prop: "line-height", Official: 24, Impl: WorkflowRunRowLineHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:35-45 → calc(24px + delta)"},
	{Node: "WorkflowRunPanel.runSummary", Prop: "line-height", Official: 18, Impl: WorkflowRunSummaryLineHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:47-56 → calc(18px + delta-secondary)"},
	{Node: "WorkflowRunPanel.statusTail", Prop: "height", Official: 20, Impl: WorkflowRunStatusTailHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:58-70"},
	{Node: "WorkflowRunPanel.statusTail", Prop: "font-size", Official: 11, Impl: WorkflowRunStatusTailFontSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:58-70"},
	{Node: "WorkflowRunPanel.statusTail", Prop: "line-height", Official: 16, Impl: WorkflowRunStatusTailLineHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:58-70"},
	{Node: "WorkflowRunPanel.statusTail", Prop: "gap", Official: 4, Impl: WorkflowRunStatusTailGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:58-70"},
	{Node: "WorkflowRunPanel.separator", Prop: "width", Official: 2, Impl: WorkflowRunSeparatorSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:136-143"},
	{Node: "WorkflowRunPanel.separator", Prop: "height", Official: 2, Impl: WorkflowRunSeparatorSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:136-143"},
	{Node: "WorkflowRunPanel.phaseHeader", Prop: "gap", Official: 6, Impl: WorkflowRunHeaderGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:72-80"},
	{Node: "WorkflowRunPanel.phaseHeader", Prop: "height", Official: 32, Impl: WorkflowRunHeaderHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:72-80"},
	{Node: "WorkflowRunPanel.phaseHeader", Prop: "border-radius", Official: 12, Impl: 12, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:82-86 → var(--dsw-radius-md)"},
	{Node: "WorkflowRunPanel.phaseLeading", Prop: "width", Official: 16, Impl: WorkflowRunLeadingSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:88-97 → calc(16px + delta)"},
	{Node: "WorkflowRunPanel.phaseLeading", Prop: "margin-right", Official: 0, Impl: WorkflowRunLeadingMarginRight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:88-97"},
	{Node: "WorkflowRunPanel.phaseTitle", Prop: "max-width", Official: 42, Impl: WorkflowRunTitleMaxWidth, Unit: uikernel.UnitPct, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:99-109"},
	{Node: "WorkflowRunPanel.phaseTitle", Prop: "line-height", Official: 24, Impl: WorkflowRunRowLineHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:99-109 → calc(24px + delta)"},
	{Node: "WorkflowRunPanel.phaseCount", Prop: "line-height", Official: 20, Impl: WorkflowRunPhaseCountLineHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:111-120 → calc(20px + delta-secondary)"},
	{Node: "WorkflowRunPanel.phaseStatus", Prop: "width", Official: 132, Impl: WorkflowRunPhaseStatusWidth, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:122-134 → calc(132px + delta-secondary*10)"},
	{Node: "WorkflowRunPanel.phaseStatus", Prop: "line-height", Official: 20, Impl: WorkflowRunPhaseCountLineHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:122-134 → calc(20px + delta-secondary)"},
	{Node: "WorkflowRunPanel.phaseList", Prop: "gap", Official: 4, Impl: WorkflowRunPhaseListGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:145-151"},
	{Node: "WorkflowRunPanel.phaseList", Prop: "padding-top", Official: 4, Impl: WorkflowRunPhaseListPadTop, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:145-151 → padding:4px 0 0 16px"},
	{Node: "WorkflowRunPanel.phaseList", Prop: "padding-left", Official: 16, Impl: WorkflowRunPhaseListPadLeft, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:145-151"},
	{Node: "WorkflowRunPanel.phaseList", Prop: "padding-left@max-width:560px", Official: 12, Impl: WorkflowRunPhaseListPadLeftNarrow, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:267-272"},
	{Node: "WorkflowRunPanel.phaseList", Prop: "media-max-width", Official: 560, Impl: WorkflowRunNarrowMaxWidth, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:267-272"},
	{Node: "WorkflowRunPanel.members", Prop: "gap", Official: 2, Impl: WorkflowRunMembersGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:157-163"},
	{Node: "WorkflowRunPanel.members", Prop: "padding-left", Official: 16, Impl: WorkflowRunMembersPadLeft, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:157-163 → padding:0 0 0 16px"},
	{Node: "WorkflowRunPanel.members", Prop: "padding-left@max-width:560px", Official: 12, Impl: WorkflowRunPhaseListPadLeftNarrow, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:267-272"},
	{Node: "WorkflowRunPanel.memberRow", Prop: "gap", Official: 12, Impl: WorkflowRunMemberGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:165-180"},
	{Node: "WorkflowRunPanel.memberRow", Prop: "min-height", Official: 24, Impl: WorkflowRunMemberMinHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:165-180 → calc(24px + delta)"},
	{Node: "WorkflowRunPanel.memberRow", Prop: "border-width", Official: 0, Impl: 0, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:165-180 → border:0"},
	{Node: "WorkflowRunPanel.memberRow", Prop: "border-radius", Official: 8, Impl: 8, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:165-180 → var(--dsw-radius-sm)"},
	{Node: "WorkflowRunPanel.memberButton", Prop: "font-weight", Official: 500, Impl: WorkflowRunMemberLabelWeight, Unit: uikernel.UnitRatio, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:186-190 → .memberButton .memberLabel"},
	{Node: "WorkflowRunPanel.memberButton", Prop: "text-underline-offset", Official: 3, Impl: WorkflowRunMemberUnderlineOffset, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:192-202"},
	{Node: "WorkflowRunPanel.dotSlot", Prop: "width", Official: 16, Impl: WorkflowRunDotSlotWidth, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:204-212"},
	{Node: "WorkflowRunPanel.dotSlot", Prop: "height", Official: 24, Impl: WorkflowRunDotSlotHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:204-212 → calc(24px + delta)"},
	{Node: "WorkflowRunPanel.memberLabelWrap", Prop: "height", Official: 24, Impl: WorkflowRunMemberLabelWrapHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:223-232 → calc(24px + delta)"},
	{Node: "WorkflowRunPanel.memberLabelWrap", Prop: "padding-inline", Official: 2, Impl: WorkflowRunMemberLabelWrapPadX, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:223-232 → padding:0 2px"},
	{Node: "WorkflowRunPanel.memberLabel", Prop: "font-size", Official: 14, Impl: WorkflowRunMemberLabelFontSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:234-246 → var(--dsh-content-font-size, 14px)"},
	{Node: "WorkflowRunPanel.memberLabel", Prop: "line-height", Official: 24, Impl: WorkflowRunMemberLabelWrapHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:234-246 → calc(24px + delta)"},
	{Node: "WorkflowRunPanel.memberStatus", Prop: "width", Official: 64, Impl: WorkflowRunMemberStatusWidth, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:248-258"},
	{Node: "WorkflowRunPanel.memberStatus", Prop: "line-height", Official: 20, Impl: WorkflowRunPhaseCountLineHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:248-258 → calc(20px + delta-secondary)"},
	{Node: "WorkflowRunPanel.empty", Prop: "line-height", Official: 20, Impl: WorkflowRunEmptyLineHeight, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:260-265 → calc(20px + delta-secondary)"},
	{Node: "WorkflowRunPanel.headerContentLayout", Prop: "gap", Official: 6, Impl: WorkflowRunHeaderContentGap, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "WorkflowRunPanel.module.css:278-280"},

	// StateDot（primitives 预构建产物；三档尺寸 + 两个几何事实）。
	{Node: "StateDot.dot", Prop: "size", Official: 10, Impl: WorkflowRunDotSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "index-ghOpC9uV.js:129:82467 → t ?? (e === 'ongoing' ? 14 : 10)"},
	{Node: "StateDot.spinner", Prop: "size", Official: 14, Impl: WorkflowRunDotSizeOngoing, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "index-ghOpC9uV.js:129:82467"},
	{Node: "StateDot.dot::after", Prop: "inset", Official: 20, Impl: WorkflowRunDotInsetPercent, Unit: uikernel.UnitPct, Status: uikernel.GeoAligned, Anchor: "index-CSAdOBx6.css:1 → ._dot_1i3xo_2:after { inset:20% }"},
	{Node: "StateDot.dot::after", Prop: "width", Official: 6, Impl: WorkflowRunDotCoreSize, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "index-CSAdOBx6.css:1 → 10 × (1 − 2×20%) = 6"},
	{Node: "StateDot.spinner", Prop: "viewBox", Official: 24, Impl: WorkflowRunSpinnerViewBox, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "index-ghOpC9uV.js:129:82598 → viewBox='0 0 24 24'"},
	{Node: "StateDot.spinner", Prop: "r", Official: 9.5, Impl: WorkflowRunSpinnerRadius, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "index-ghOpC9uV.js:129:82598 → <circle cx=12 cy=12 r=9.5>"},
	{Node: "StateDot.spinner", Prop: "stroke-width", Official: 2, Impl: WorkflowRunSpinnerStroke, Unit: uikernel.UnitPx, Status: uikernel.GeoAligned, Anchor: "index-CSAdOBx6.css:1 → ._spinnerTrack_1i3xo_47,_spinnerArc_1i3xo_48 { stroke-width:2 }"},

	// 动画：Viva 画布是静态的，旋转不表达。这不是几何差，是**时间轴**差——
	// 登记在这里是为了让"官方有动画、我们没有"这件事可检索（见报告 §缺口）。
	{Node: "StateDot.spinnerMotion", Prop: "animation-duration", Official: 1500, Impl: 0, Unit: uikernel.UnitPx, Status: uikernel.GeoDerived, Anchor: "index-CSAdOBx6.css:1 → _dsh-state-dot-spin 1.5s linear infinite", Formula: "animation:_dsh-state-dot-spin 1.5s linear infinite；Viva 静态画布无时间轴，实现只画 spinner 的静态帧（track+arc），旋转不表达"},
	{Node: "DisclosureRow.row", Prop: "transition-duration", Official: 0.1, Impl: 0, Unit: uikernel.UnitRatio, Status: uikernel.GeoDerived, Anchor: "index-CSAdOBx6.css:1 → ._row_l4nv8_26 { transition:color .1s ease }", Formula: "transition:color .1s ease；Viva 静态画布无过渡，颜色直接取目标值（终态一致）"},
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
