// official_transcript_view.go —— 主区 transcript 的**绘制层**（rc.2 官方结构）。
//
// # 挂载策略：逐条真实挂载，不是虚拟列表
//
// 官方 `ChatFlow` 把 `entries` 全量 `map` 成 seat（`ChatFlow.tsx:52-63`），
// 每个 seat 一个 `div.flowItem[data-chat-node-key]`（`ChatNodeSeat.tsx:152-165`），
// **没有任何窗口化切片**。设计意图写在 CSS 注释里：
//
//	.flowItem——"It is neutral until it becomes the natural measurement/mount unit
//	for a virtualizer without changing the column gap."（ChatView.module.css:100-104）
//
// 即"虚拟化是未来选项，当前未启用"。**不要**为了"性能"先上虚拟列表：
// 那会让行距语义、Turn 查回（`hidden="until-found"`）、滚动定位三处同时失真。
//
// 历史窗口化靠**分页**（`hasMore` / `loadingOlder` 五态单按钮），不是虚拟化。
//
// # 双层滚动口
//
//	ConversationRoot .scrollBody[data-conversation-scroll]   ← 宿主滚动口（overflow-y:auto）
//	└─ ChatView .frame (container-type:inline-size)
//	   └─ .root[data-chat-following-tail]  overflow-y:clip
//	      └─ .scroll                      ← 自滚动口；在宿主滚动口内降级 overflow-y:visible
//	         └─ .column[data-chat-flow]   ← 唯一行序容器
//	            └─ [data-slot=conversation.chat.flow]  ← display:contents 的槽锚点
//	               └─ .flowItem           ← 逐条消息（真实挂载）
//
// 判据（`use-chat-viewport.ts:118-119`）：`list.closest('[data-conversation-scroll]') ?? list`
// ——「外层有宿主滚动口就用它，没有就自己滚」。
package dsconversation

import (
	"strconv"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

// TranscriptViewport 是这一帧的两个滚动口配置。
type TranscriptViewport struct {
	// HostScroll 表示外层**已经有**宿主滚动口（ConversationRoot .scrollBody）。
	//
	// true  → 自滚动口降级 `overflow-y:visible`（官方 ChatView.module.css:57-64），
	//         纵向滚动归宿主口；
	// false → 自滚动口自己滚（独立挂载 Chat 的情形）。
	HostScroll bool
	// HostID / SelfID 是两个滚动口的交互 id。
	//
	// **必须分开**：viva 的 ScrollState 按 id 存，共用一个 id 会让两层拿到
	// 同一个 Offset（滚动位置互相打架）。
	HostID gfx.ID
	SelfID gfx.ID
}

// RowBox 是渲染出来的**一行**（对应官方一个 `.flowItem`）。
type RowBox struct {
	// Key 是稳定渲染键（官方 `data-chat-node-key`）。
	Key string
	// Kind 是官方 kind（官方 `data-chat-flow-kind`）。
	Kind ChatNodeKind
	// Box 是这一行的盒子（`min-width:0` 的 flowItem）。
	Box ui.Box
}

// TranscriptBoxes 是上一帧的容器几何（AfterDraw / 测试用）。
//
// 为什么留着：viva 的绘制顺序不受调用方控制，Markdown 这类"必须先量后画"
// 的东西要等布局定下来；而测试要断言"双层滚动口真的存在"也只能读几何。
type TranscriptBoxes struct {
	// ScrollBody 是宿主滚动口（`HostScroll` 为 false 时为零值）。
	ScrollBody ui.Box
	// Frame / Root / Scroll / Column / Flow 是 ChatView 的五层容器。
	Frame  ui.Box
	Root   ui.Box
	Scroll ui.Box
	Column ui.Box
	Flow   ui.Box
	// Rows 是逐条消息行（顺序 = 节点顺序）。
	Rows []RowBox
	// Older 是「加载更早」那一行（官方 `.older` / `.hint`）。
	//
	// **它不在 Rows 里**：官方它画在 flow 槽**之前**、`.column` 的直接子级，
	// 不是一个消息节点（`ChatView.tsx:265-279`）。零值 = 这一帧不画。
	Older ui.Box
	// HistoryState 是这一帧「加载更早」的五态（诊断/测试用；HistoryHidden = 没画）。
	HistoryState HistoryLoadState
	// HostScroll 记录这一帧外层有没有宿主滚动口。
	HostScroll bool
	// SelfScrolls 记录自滚动口这一帧是否真的在滚（= !HostScroll）。
	SelfScrolls bool
}

// RowCount 是这一帧挂载的行数（= 可见节点数）。
func (b TranscriptBoxes) RowCount() int { return len(b.Rows) }

// HostScrollRect 返回宿主滚动口的矩形；这一帧没有宿主滚动口时 ok=false。
//
// 为什么要有这个包装：**零值 flex.Box 调 Rect() 会空指针**（b.f 为 nil）。
// 没有宿主口的模式（独立挂载 Chat）下 `.scrollBody` 本来就不该存在，
// 调用方必须拿到 ok=false 而不是崩。
func (b TranscriptBoxes) HostScrollRect() (ui.Rect, bool) {
	if !b.HostScroll {
		return ui.Rect{}, false
	}
	return b.ScrollBody.Rect(), true
}

// RowOf 按 key 找一行；ok=false 表示这一帧没挂载它。
//
// **submitted-plan 恒 ok=false**：官方没有行渲染器（§5.19），
// 这不是缺陷而是真值——有专门断言钉住。
func (b TranscriptBoxes) RowOf(key string) (RowBox, bool) {
	for _, row := range b.Rows {
		if row.Key == key {
			return row, true
		}
	}
	return RowBox{}, false
}

// OfficialTranscriptView 是主区 transcript 的渲染器。
//
// 无状态（几何每帧覆盖写）；一个 Plugin 持有一个实例即可。
type OfficialTranscriptView struct {
	Theme *pluginapi.Theme
	Prims pluginapi.Primitives
	// Boxes 是上一帧的几何（每帧 Draw 开头重置）。
	Boxes TranscriptBoxes

	// codeBlocks 是本帧**布局阶段占位、覆画阶段才真画**的代码块。
	//
	// 为什么要两段：`mdlite.CodeBlock` 要的是**真实矩形**，而布局阶段
	// viva 还没解出列宽（`f.Bounds()` 给的是整窗）。详见 `placeCodeBlock`
	// 与 `PaintCodeBlocks` 的注释。
	codeBlocks []pendingCodeBlock
	// prevCodeBlocks 是上一帧的代码块：按 (key, 源码) 复用，保住横滚位移、
	// 折行开关与复制反馈——每帧重建等于"滚一下弹回原位"。
	prevCodeBlocks []pendingCodeBlock
	// codeClipBox 是代码块的裁剪视口（自滚动口；布局之后才有矩形）。
	codeClipBox ui.Box
	// codeClipOK 报告这一帧有没有可用的裁剪视口（Draw 没跑 / 零值 Box 时不能取矩形）。
	codeClipOK bool
	// frameSeq / paintedSeq 是"一帧只覆画一次"的帧号。
	//
	// 为什么需要去重：两个座位（`conversation.view` 与 `main.body`）的 painter
	// 都挂了 AfterDraw，壳会把**所有** painter 的 AfterDraw 跑一遍，不管这个
	// 座位这一帧有没有被 dispatch。不去重就会同一块画两遍（叠字 + 双 hit 区）。
	frameSeq   uint64
	paintedSeq uint64
}

// pendingCodeBlock 是一条「布局阶段占位、覆画阶段真画」的代码块。
type pendingCodeBlock struct {
	// key 是稳定渲染键（entry key + 块序号），四个交互 id 由它派生。
	key string
	// block 是代码块视图（跨帧复用，自带横滚/折行/复制反馈状态）。
	block mdlite.CodeBlock
	// box 是布局阶段占的位（覆画阶段从这里取真实矩形）。
	box ui.Box
	// geometry 是上一次覆画的真实几何（banner / 视口 / 横滚上限），
	// 测试与 Turn 导航读它——**不跨帧外推**（每帧覆画时覆盖写）。
	geometry mdlite.CodeBlockGeometry
}

// NewOfficialTranscriptView 造一个渲染器。
func NewOfficialTranscriptView(theme *pluginapi.Theme, prims pluginapi.Primitives) *OfficialTranscriptView {
	return &OfficialTranscriptView{Theme: theme, Prims: prims}
}

// Draw 画完整 transcript：容器链 + 逐条节点。
//
// 入参 entries 必须是**已经投影好的**节点（projection 层负责帧 → 节点）；
// 本函数不解析 journal、不发请求、不猜数据。
func (v *OfficialTranscriptView) Draw(f ui.C, env *pluginapi.Env, vp TranscriptViewport, entries []TranscriptEntry) {
	t := v.Theme
	if t == nil && env != nil {
		t = env.Theme
	}
	v.Boxes = TranscriptBoxes{HostScroll: vp.HostScroll, SelfScrolls: !vp.HostScroll}

	// ── 代码块：本帧重新占位 ──────────────────────────────────────
	//
	// 上一帧的代码块先挪到 prevCodeBlocks（覆画阶段按 key+源码复用它的
	// 横滚位移与复制反馈），本帧只重新占位、不重新画。
	v.prevCodeBlocks = append(v.prevCodeBlocks[:0], v.codeBlocks...)
	v.codeBlocks = v.codeBlocks[:0]
	v.frameSeq++
	v.codeClipOK = false

	// ── ChatView 五层容器（frame → root → scroll → column → flow）──
	//
	// 挂载点二选一（官方 `use-chat-viewport.ts:118-119` 的判据）：
	//
	//	外层有宿主滚动口 → 自滚动口降级 overflow-y:visible（纵向滚动归宿主）
	//	否则             → 自滚动口自己滚（独立挂载 Chat）
	drawChatView := func() {
		// ── ChatView.frame（container-type:inline-size，给浮动控件做断点）──
		// 宿主滚动口存在时官方降级 flex:0 0 auto;min-height:auto;height:auto
		v.Boxes.Frame = ui.Column(f).FillWidth().Grow(1).MinHeight(0)
		v.Boxes.Frame.Children(func() {
			// ── ChatView.root[data-chat-following-tail]（overflow-y:clip）──
			v.Boxes.Root = ui.Column(f).FillWidth().Grow(1).MinHeight(0).Clip()
			v.Boxes.Root.Children(func() {
				// ── ChatView.scroll（自滚动口；宿主口内降级 overflow-y:visible）──
				if !vp.HostScroll && !vp.SelfID.IsZero() {
					v.Boxes.Scroll = ui.Scroll(f).FillWidth().Grow(1).MinHeight(0).
						PadSides(ChatScrollPad, ChatScrollPad+ComposerSideClearance, ChatScrollPad, ChatScrollPad+ComposerSideClearance)
				} else {
					v.Boxes.Scroll = ui.Column(f).FillWidth().Grow(1).MinHeight(0).
						PadSides(ChatScrollPad, ChatScrollPad+ComposerSideClearance, ChatScrollPad, ChatScrollPad+ComposerSideClearance)
				}
				// 代码块的裁剪视口就是自滚动口：横滚内容只能裁在列内，
				// 不能画到顶栏/输入区上（旧渲染路径踩过同样的坑）。
				v.codeClipBox, v.codeClipOK = v.Boxes.Scroll, true
				v.Boxes.Scroll.Children(func() {
					// ── ChatView.column（唯一行序容器，max-width: chat-content-width）──
					v.Boxes.Column = ui.Column(f).FillWidth().MinHeight(0)
					v.Boxes.Column.Children(func() {
						// ── 「加载更早」那一行（官方 `.hint` / `.older`）──
						// 它画在 flow 槽**之前**：官方 `.column` 的直接子级里它是
						// 第一条（`ChatView.tsx:265-279`），不属于任何消息节点。
						// 五态的判据全部来自 `Snapshot.History`（壳算好的），这里不猜。
						v.Boxes.Older, v.Boxes.HistoryState = v.drawHistoryRow(f, env, env.Data.History)
						// ── flow 槽（display:contents：保留布局但不保留盒模型）──
						v.Boxes.Flow = ui.Column(f).FillWidth()
						v.Boxes.Flow.Children(func() {
							v.Boxes.Rows = v.Boxes.Rows[:0]
							for _, entry := range entries {
								if !ChatNodeRenders(entry.Kind) {
									// 官方没有行渲染器（只有 submitted-plan）：
									// 不产生任何 DOM，**不是**画一个空壳。
									continue
								}
								row := v.drawRow(f, env, entry)
								v.Boxes.Rows = append(v.Boxes.Rows, row)
							}
						})
					})
				})
			})
		})
	}

	// ── 宿主滚动口（ConversationRoot .scrollBody[data-conversation-scroll]）──
	// 官方：flex:1;flex-direction:column;min-height:0;margin-right:2px;overflow-y:auto
	//      scrollbar-gutter:stable（Viva 没有该能力 → 见几何表 deviation 行）
	if vp.HostScroll && !vp.HostID.IsZero() {
		v.Boxes.ScrollBody = ui.Scroll(f).FillWidth().Grow(1).MinHeight(0).
			MarginSides(0, ScrollBodyMarginRight, 0, 0)
		v.Boxes.ScrollBody.Children(drawChatView)
		return
	}
	drawChatView()
}

// drawRow 画一条节点（官方 `.flowItem` 外壳 + kind 分派）。
func (v *OfficialTranscriptView) drawRow(f ui.C, env *pluginapi.Env, e TranscriptEntry) RowBox {
	row := ui.Column(f).FillWidth().MinWidth(0)
	row.Children(func() {
		switch e.Kind {
		case ChatNodeUser:
			v.drawUser(f, env, e)
		case ChatNodeSteering:
			v.drawSteering(f, env, e)
		case ChatNodeContext:
			v.drawDisclosureRow(f, e, "上下文注入", e.Data.SourceKind)
		case ChatNodeTurnTrigger:
			v.drawTurnTrigger(f, e)
		case ChatNodeCommandInput:
			v.drawCommandInput(f, e)
		case ChatNodeAssistantStep:
			v.drawAssistant(f, env, e)
		case ChatNodeToolCall:
			v.drawToolCall(f, e)
		case ChatNodeCommand:
			v.drawCommand(f, e)
		case ChatNodeManualCompaction:
			v.drawManualCompaction(f, e)
		case ChatNodeCompaction:
			v.drawCompaction(f, e)
		case ChatNodeModelRetry:
			v.drawModelRetry(f, e)
		case ChatNodeTurnError:
			v.drawTurnError(f, e, false)
		case ChatNodeTurnMaxTokens:
			v.drawTurnError(f, e, true)
		case ChatNodeTurnProcess:
			v.drawTurnProcess(f, e)
		case ChatNodeTurnTail:
			v.drawTurnTail(f, e)
		case ChatNodeSystemPrompt:
			v.drawDisclosureRow(f, e, "系统提示", "")
		case ChatNodeQuestionReply:
			v.drawQuestionReply(f, e)
		case ChatNodeWorkflowRun:
			v.drawWorkflowRun(f, e)
		default:
			// 表外 kind：原样 JSON 兜底（**不编摘要**）。
			v.drawUnknown(f, e)
		}
	})
	return RowBox{Key: e.Key, Kind: e.Kind, Box: row}
}

// ── 逐种节点 ───────────────────────────────────────────────────────────

// drawUser 画 §5.1 用户消息：右对齐行 + 内容栈 + 气泡 + 附件行 + 引用摘要。
func (v *OfficialTranscriptView) drawUser(f ui.C, env *pluginapi.Env, e TranscriptEntry) {
	t := v.Theme
	ui.Row(f).FillWidth().Justify(ui.End).Gap(UserRowGap).Children(func() {
		stack := ui.Column(f).Items(ui.End).Gap(UserStackGap).MinWidth(0).
			MaxWidthPercent(UserStackWidthCap * 100)
		stack.Children(func() {
			if e.Data.Text != "" {
				ui.Text(f, e.Data.Text).FontSize(t.Fonts.Body).LineHeight(t.Fonts.BodyLine).Color(t.LabelPrimary)
			}
			// 引用摘要（.referenceSummary：13px / 18px + delta-secondary，label-tertiary）
			if len(e.Data.References) > 0 {
				ui.Text(f, strings.Join(e.Data.References, " · ")).
					FontSize(ReferenceSummaryFontSize).LineHeight(ReferenceSummaryLineHeight).
					Color(t.LabelTertiary).Ellipsis()
			}
			// 附件行（.attachmentRow：右对齐、wrap、gap 8）
			if len(e.Data.Attachments) > 0 {
				ui.Row(f).Justify(ui.End).Gap(AttachmentRowGap).Children(func() {
					for _, attachment := range e.Data.Attachments {
						v.drawAttachment(f, attachment)
					}
				})
			}
		})
	})
}

// drawAttachment 画一个附件：图片走 64×64 瓦片，文件走 240×64 卡片。
func (v *OfficialTranscriptView) drawAttachment(f ui.C, attachment TranscriptAttachment) {
	t := v.Theme
	if attachment.Image {
		// 图片瓦片（MessageImage.tile）：64×64，多图时降级。
		ui.Slot(f).Size(ImageTileSize, ImageTileSize).Radius(t.Radii.SM).Border(0.5, t.BorderL2)
		return
	}
	card := ui.Row(f).Size(FileCardWidth, FileCardMinHeight).Gap(FileCardGap).Radius(t.Radii.LG).
		Border(0.5, t.BorderL2).PadSides(FileCardPadY, FileCardPadX, FileCardPadY, FileCardPadX)
	card.Children(func() {
		ui.Slot(f).Size(FileIconSize, FileIconSize)
		ui.Column(f).MinWidth(0).Children(func() {
			name := attachment.Name
			if name == "" {
				name = UnknownText
			}
			ui.Text(f, name).FontSize(FileNameFontSize).LineHeight(FileNameLineHeight).
				Weight(gfx.WeightMedium).Color(t.LabelPrimary).Ellipsis()
			if attachment.Meta != "" {
				ui.Text(f, attachment.Meta).FontSize(FileMetaFontSize).LineHeight(FileMetaLineHeight).
					Color(t.LabelTertiary).Ellipsis()
			}
		})
	})
}

// drawSteering 画 §5.2 活动 Turn 内被接收的用户消息。
//
// 与 user 的差别：行距是 `calc(44px + delta)`（保住 Turn 尾槽），
// 且行上带 `data-pending-steering`（Host 权威待入）。
func (v *OfficialTranscriptView) drawSteering(f ui.C, env *pluginapi.Env, e TranscriptEntry) {
	t := v.Theme
	ui.Row(f).FillWidth().Justify(ui.End).Gap(UserRowGap).
		MarginSides(ChatFlowGapSteeringBatch-ChatFlowGapDefault, 0, 0, 0).Children(func() {
		ui.Column(f).Items(ui.End).Gap(UserStackGap).MinWidth(0).Children(func() {
			ui.Text(f, e.Data.Text).FontSize(t.Fonts.Body).LineHeight(t.Fonts.BodyLine).Color(t.LabelSecondary)
		})
	})
}

// drawDisclosureRow 画 §5.3 context / §5.16 system-prompt 的共享 24px 行壳。
//
// 官方 `DisclosureRow` 的精确几何**未取证**（`ui-primitives` 没有 rc.2 src），
// 见几何表 derived 行——按包自身 CSS 注释的"leading 16 + gap 6 + 13/24 标题"
// 二手证据摆位，**不得反推像素**。
func (v *OfficialTranscriptView) drawDisclosureRow(f ui.C, e TranscriptEntry, title, subtitle string) {
	t := v.Theme
	row := ui.Row(f).FillWidth().Items(ui.Center).Gap(CompactionLeadingGap).PadY(2)
	row.Children(func() {
		ui.Slot(f).Size(CompactionLeading, CompactionLeading)
		ui.Text(f, title).FontSize(t.Fonts.Secondary).LineHeight(t.Fonts.SecondaryLine).Color(t.LabelTertiary)
		if subtitle != "" {
			ui.Slot(f).Size(ContextSepSize, ContextSepSize)
			ui.Text(f, subtitle).FontSize(t.Fonts.Caption).Color(t.LabelCaption).Ellipsis()
		}
		if e.Data.Text != "" {
			ui.Text(f, e.Data.Text).FontSize(t.Fonts.Secondary).Color(t.LabelSecondary).Ellipsis()
		}
	})
}

// drawTurnTrigger 画 §5.4 记录型唤醒 / 续写回执。
func (v *OfficialTranscriptView) drawTurnTrigger(f ui.C, e TranscriptEntry) {
	t := v.Theme
	card := ui.Column(f).FillWidth().Border(0.5, t.BorderL1).Radius(t.Radii.XL)
	card.Children(func() {
		header := ui.Row(f).FillWidth().Items(ui.Center).Gap(TurnTriggerHeaderGap).
			PadSides(TurnTriggerHeaderPadY, TurnTriggerHeaderPadX, TurnTriggerHeaderPadY, TurnTriggerHeaderPadX)
		header.Children(func() {
			ui.Text(f, triggerTitle(e)).FontSize(t.Fonts.Body).Color(t.LabelPrimary)
			ui.Text(f, e.TimeText()).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		})
		if e.Data.Text != "" {
			ui.Row(f).FillWidth().PadSides(0, TurnTriggerHeaderPadX, TurnTriggerBodyPadBottom, TurnTriggerBodyPadLeft).Children(func() {
				ui.Text(f, e.Data.Text).FontSize(t.Fonts.Caption).Color(t.LabelSecondary).
					MaxHeight(TurnTriggerContentMaxHeight)
			})
		}
	})
}

// triggerTitle 取 Turn 触发卡标题（没有 source.kind 就说"记录"——不猜具体类型）。
func triggerTitle(e TranscriptEntry) string {
	if e.Data.SourceKind != "" {
		return e.Data.SourceKind
	}
	return "记录"
}

// drawCommandInput 画 §5.5 `/goal` 命令输入（goal 包节点）。
func (v *OfficialTranscriptView) drawCommandInput(f ui.C, e TranscriptEntry) {
	t := v.Theme
	ui.Row(f).FillWidth().Items(ui.Center).Gap(6).Children(func() {
		ui.Text(f, e.Data.Text).FontSize(t.Fonts.Body).Mono().Color(t.LabelSecondary)
	})
}

// drawAssistant 画 §5.6 助手正文：markdown 块分派 + reasoning + 中断标记。
func (v *OfficialTranscriptView) drawAssistant(f ui.C, env *pluginapi.Env, e TranscriptEntry) {
	t := v.Theme
	root := ui.Column(f).FillWidth().Gap(AssistantBlockGap)
	root.Children(func() {
		if e.Data.Reasoning != "" {
			// reasoning 分区块间距 4px（AssistantMarkdown.module.css:24-29）
			ui.Column(f).FillWidth().Gap(AssistantReasoningBlockGap).Children(func() {
				ui.Row(f).FillWidth().Items(ui.Center).Gap(CompactionLeadingGap).Children(func() {
					ui.Slot(f).Size(TurnProcessChevron, TurnProcessChevron)
					ui.Text(f, "思考").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
				})
				ui.Text(f, e.Data.Reasoning).FontSize(t.Fonts.Secondary).Color(t.LabelSecondary)
			})
		}
		if e.Data.Text != "" {
			v.drawMarkdown(f, env, e.Data.Text, e.Key)
		}
		if e.Data.Status == "stopped" {
			// 中断标记 .stopped：11px/18px，左对齐
			ui.Row(f).Justify(ui.Start).Children(func() {
				ui.Text(f, "已停止").FontSize(StoppedFontSize).LineHeight(StoppedLineHeight).
					PadX(StoppedPadX).Radius(t.Radii.SM).Color(t.LabelTertiary)
			})
		}
	})
}

// drawMarkdown 画 markdown 块：标题阶梯（h1–h6）/ 段落 / 列表 / 引用块 /
// 分隔线 / 表格 / 围栏代码块。
//
// # 块间距
//
// 官方 `AssistantMarkdown.body` 是 `display:flex;flex-direction:column;gap:16px`
// （`AssistantMarkdown.module.css:17-22`，几何表 aligned 行）。**块与块之间靠
// 容器的 gap，不靠每个块的 margin**——所以这里建一个带 gap 的列，各块自己
// 不带上下边距。这样"两个块挨在一起"和"三个块挨在一起"的行距是同一个数。
//
// # 几何来源（重要）
//
// 标题阶梯、列表缩进、引用块边框、代码块 banner 高度**全部**来自
// `official_transcript_geometry.go` 的 derived 段。官方 `MarkdownText` /
// `FenceCodeBlock` 在 `@deepseek-ai/dsh-client-ui-primitives` 里，而该包
// **没有 rc.2 `src/`**（结构树 §9 第 1/2 条），所以这些值**不得反推像素**，
// 也不许拿去改壳几何。取到 primitives `src/` 后逐行换锚。
//
// # 行内代码
//
// 整段都是 Mono span 时走等宽字体（官方 `MarkdownText` 的 Mono span）；
// 精确的行内背景色（`--dsw-alias-markdown-inline-code`）同样在 primitives 里，
// 未取证——这里只换字体，不画背景。
func (v *OfficialTranscriptView) drawMarkdown(f ui.C, env *pluginapi.Env, text, key string) {
	body := ui.Column(f).FillWidth().Gap(AssistantBlockGap)
	body.Children(func() {
		var doc markdown.Document
		doc.Parse(text)
		for i := range doc.Blocks {
			block := doc.Blocks[i]
			switch block.Kind {
			case markdown.Code:
				// 围栏：**不在布局阶段画**，只占位（见 placeCodeBlock）。
				v.placeCodeBlock(f, env, block, key+":"+strconv.Itoa(i))
			case markdown.Rule:
				// 分隔线 1 物理像素（derived；官方 hr 几何未取证）。
				ui.Divider(f).Height(MarkdownRuleHeight).Background(v.Theme.BorderL2)
			case markdown.Table:
				v.drawTableBlock(f, &doc, block)
			case markdown.Heading:
				v.drawHeading(f, &doc, block)
			case markdown.Item:
				v.drawListItem(f, &doc, block)
			default: // markdown.Paragraph
				v.drawParagraph(f, &doc, block)
			}
		}
	})
}

// drawParagraph 画段落；引用块内的段落再套一层左边框。
func (v *OfficialTranscriptView) drawParagraph(f ui.C, doc *markdown.Document, block markdown.Block) {
	t := v.Theme
	spans := doc.Spans(&block)
	body := spansText(spans)
	if body == "" {
		return
	}
	v.drawQuoted(f, block.Quote, func() {
		// Wrap 而不是 Text：官方正文会按列宽折行（`white-space` 默认 normal）。
		// 用 Text 的话长段落会被右边缘硬切——真机上表现成"回答只显示一行"。
		box := ui.Wrap(f, body).FontSize(t.Fonts.Body).LineHeight(t.Fonts.BodyLine).Color(t.LabelPrimary)
		if allMono(spans) {
			box.Mono().FontSize(t.Fonts.Code)
		}
	})
}

// drawHeading 画标题阶梯（h1–h6）。
//
// 字号/行高来自 `MarkdownHeadingFor`（derived 阶梯表）：级别越界按最近一级
// 夹紧，调用方不必再判边界。字重取 semibold——h5/h6 与正文同字号，只靠字重
// 区分（官方 primitives 未取证，这是实现选择）。
func (v *OfficialTranscriptView) drawHeading(f ui.C, doc *markdown.Document, block markdown.Block) {
	body := spansText(doc.Spans(&block))
	if body == "" {
		return
	}
	heading := MarkdownHeadingFor(block.Level)
	v.drawQuoted(f, block.Quote, func() {
		ui.Wrap(f, body).FontSize(heading.Size).LineHeight(heading.Line).
			Weight(gfx.WeightSemibold).Color(v.Theme.LabelPrimary)
	})
}

// drawListItem 画列表项：按 level 缩进 + 固定宽标记槽（无序 `•`、有序 `N.`）+ 正文。
//
// 缩进用 `level × MarkdownListIndentPerLevel`（derived）：Viva 没有
// `padding-inline-start: 20px` 这种列表缩进，只能按嵌套深度自己算左内边距。
func (v *OfficialTranscriptView) drawListItem(f ui.C, doc *markdown.Document, block markdown.Block) {
	t := v.Theme
	body := spansText(doc.Spans(&block))
	if body == "" {
		return
	}
	marker := MarkdownListBullet
	if block.Number > 0 {
		// 有序列表：官方 `Number` 是标记的**实际值**（不是序号），照抄不重排。
		marker = strconv.Itoa(block.Number) + "."
	}
	indent := float32(block.Level) * MarkdownListIndentPerLevel
	v.drawQuoted(f, block.Quote, func() {
		row := ui.Row(f).FillWidth().PadSides(0, 0, 0, indent).Gap(MarkdownListMarkerGap)
		row.Children(func() {
			// 标记槽固定 18px 右对齐（derived；容得下 "99."），正文吃掉剩余宽度。
			ui.Text(f, marker).FontSize(t.Fonts.Body).LineHeight(t.Fonts.BodyLine).
				Color(t.LabelSecondary).Width(MarkdownListMarkerWidth).TextEnd()
			ui.Wrap(f, body).FillWidth().FontSize(t.Fonts.Body).LineHeight(t.Fonts.BodyLine).Color(t.LabelPrimary)
		})
	})
}

// drawTableBlock 画表格：按等宽文本原样渲染。
//
// 宽表格突破（`md-table-wide` 的 `100cqw`）在 Viva 里没有容器查询，
// 见几何表 deviation 行——这是渲染器能力边界，不是"没做"。
func (v *OfficialTranscriptView) drawTableBlock(f ui.C, doc *markdown.Document, block markdown.Block) {
	t := v.Theme
	card := ui.Column(f).FillWidth().Gap(AssistantReasoningBlockGap)
	card.Children(func() {
		for _, cell := range doc.Cells(&block) {
			ui.Text(f, spansText(doc.CellSpans(cell))).FontSize(t.Fonts.Code).Mono().Color(t.LabelSecondary)
		}
	})
}

// drawQuoted 按引用深度套左边框（每级 2px 竖线 + 12px 左内边距）。
//
// Viva 没有 `border-inline-start`，左边框用 Row 里的竖线盒（`ui.Divider(f)`
// 在 Row 里是竖线）表达；竖线随行高拉伸，所以多行引用块也是一条通栏线。
// 两个值都是 derived（官方 blockquote 几何未取证）。
func (v *OfficialTranscriptView) drawQuoted(f ui.C, depth int, body func()) {
	if depth <= 0 {
		body()
		return
	}
	row := ui.Row(f).FillWidth().Gap(MarkdownQuotePadLeft)
	row.Children(func() {
		ui.Divider(f).Width(MarkdownQuoteBorderWidth).Background(v.Theme.BorderL2)
		ui.Column(f).FillWidth().Children(func() {
			v.drawQuoted(f, depth-1, body)
		})
	})
}

// placeCodeBlock 是**布局阶段**：按估算列宽算好高度，占一个固定高的盒子。
//
// 真正的绘制在 `PaintCodeBlocks`（布局之后矩形才有效）。为什么不能在这里画：
//
//	viva 的布局是「先声明整棵树、再一次性 End() 解算」，Draw 回调里所有
//	Box.Rect() 都还是零值；`f.Bounds()` 给的是**整窗**，不是列宽。
//	而 mdlite.CodeBlock 必须拿到真实矩形（banner 宽度、横滚视口、复制按钮
//	命中区全都由它算）。`pluginapi.Painter.AfterDraw` 就是为这件事存在的。
//
// 高度用估算宽度算：估算公式与真实列宽**同一条算式**（主区宽 - 滚动口
// 左右内边距），所以 `wrap=false` 时行数完全一致，只有"要不要横滚条"那 10px
// 可能差 1–2px（不会让内容错位）。
func (v *OfficialTranscriptView) placeCodeBlock(f ui.C, env *pluginapi.Env, block markdown.Block, key string) {
	style := v.codeBlockStyle(env)
	width := v.codeMeasureWidth(env)
	code := mdlite.NewCodeBlock(block, nil)
	// 跨帧复用视图状态（横滚位移 / 折行 / 复制反馈）。
	if prev, ok := v.previousCodeBlock(key, code.Code); ok {
		code = prev
	}
	height := code.Height(v.measureCanvas(env), width, style)
	// MinHeight(0) 是必须的：默认 min-height:auto 会被内容顶开，
	// 而我们只要一个**固定高**的占位盒子。
	box := ui.Column(f).FillWidth().Height(height).MinHeight(0)
	v.codeBlocks = append(v.codeBlocks, pendingCodeBlock{key: key, block: code, box: box})
}

// measureCanvas 取测量用的画布（拉伸不了就退一张零值画布）。
//
// `mdlite` 的 `MonoCellSize` 对零值画布是安全的（拿不到字形宽度时按
// `fontSize × 0.6` 兜底），所以这里不需要伪造 measurer。
func (v *OfficialTranscriptView) measureCanvas(env *pluginapi.Env) *gfx.Canvas {
	if env != nil && env.Canvas != nil {
		return env.Canvas
	}
	return &gfx.Canvas{}
}

// PaintCodeBlocks 是**覆画阶段**：布局定下来之后，把 Draw 阶段占位的代码块
// 按真实矩形画出来（banner 语言标签 / 语法高亮正文 / 横滚视口 / 复制按钮）。
//
// 一帧只画一次：两个座位（`conversation.view` 与 `main.body`）的 painter 都挂了
// AfterDraw，壳会把**所有** painter 的 AfterDraw 跑一遍（不管这个座位这一帧
// 有没有被 dispatch），所以用帧号去重——否则同一块画两遍，表现为叠字 + 双命中区。
func (v *OfficialTranscriptView) PaintCodeBlocks(canvas *gfx.Canvas, env *pluginapi.Env) {
	if v == nil || canvas == nil || len(v.codeBlocks) == 0 {
		return
	}
	if v.frameSeq == 0 || v.frameSeq == v.paintedSeq {
		return
	}
	v.paintedSeq = v.frameSeq

	style := v.codeBlockStyle(env)
	// 裁剪到自滚动口：横滚内容只能出现在列内，不许画到顶栏/输入区上。
	if v.codeClipOK {
		if clip := v.codeClipBox.Rect(); clip.Width > 0 && clip.Height > 0 {
			canvas.Clip(clip)
			defer canvas.Unclip()
		}
	}
	for i := range v.codeBlocks {
		pending := &v.codeBlocks[i]
		bounds := pending.box.Rect()
		if bounds.Width <= 0 || bounds.Height <= 0 {
			// 这一行被滚动口裁掉了（或布局没跑到它）：**不画**，不是画到 (0,0)。
			continue
		}
		pending.block.Copy = transcriptCopyFunc(env)
		pending.geometry = pending.block.Draw(canvas, bounds, v.codeBlockIDs(env, pending.key), style)
	}
}

// CodeBlockGeometryOf 返回某个渲染键的代码块上一帧真实几何；ok=false 表示
// 这一帧没有这块（被裁掉 / 不是代码块）。测试与 Turn 导航用。
func (v *OfficialTranscriptView) CodeBlockGeometryOf(key string) (mdlite.CodeBlockGeometry, bool) {
	if v == nil {
		return mdlite.CodeBlockGeometry{}, false
	}
	for i := range v.codeBlocks {
		if v.codeBlocks[i].key == key {
			return v.codeBlocks[i].geometry, true
		}
	}
	return mdlite.CodeBlockGeometry{}, false
}

// CodeBlockBox 是一条代码块的渲染结果（渲染键 + 真实几何）。
type CodeBlockBox struct {
	// Key 是稳定渲染键（entry key + 块序号）。
	Key string
	// Geometry 是上一次覆画的真实几何。
	Geometry mdlite.CodeBlockGeometry
}

// CodeBlocks 返回本帧代码块的只读几何快照（顺序 = 文档块顺序）。
//
// 为什么不在 Draw 里返回：代码块的矩形要等布局结束才有，所以几何是**覆画
// 阶段**的产物。测试断言"banner 32px / 视口非零 / 横滚上限"全读这一份。
func (v *OfficialTranscriptView) CodeBlocks() []CodeBlockBox {
	if v == nil {
		return nil
	}
	boxes := make([]CodeBlockBox, 0, len(v.codeBlocks))
	for i := range v.codeBlocks {
		boxes = append(boxes, CodeBlockBox{Key: v.codeBlocks[i].key, Geometry: v.codeBlocks[i].geometry})
	}
	return boxes
}

// HandleScroll 把真实滚轮/触控板事件转发给代码块。
//
// 判据在 mdlite：只认**本代码块视口**上的横向手势（或 Shift+纵向滚轮），
// 绝不抢外层消息流的纵向滚动——抢了就会表现为"列表滚不动"。
func (v *OfficialTranscriptView) HandleScroll(event gfx.ScrollEvent) gfx.EventResult {
	if v == nil {
		return gfx.EventResult{}
	}
	for i := range v.codeBlocks {
		if result := v.codeBlocks[i].block.HandleScroll(event); result.Handled {
			return result
		}
	}
	return gfx.EventResult{}
}

// codeStyle 之外的绘制令牌：主题 + 代码块几何（全部 derived）。
func (v *OfficialTranscriptView) codeBlockStyle(env *pluginapi.Env) mdlite.CodeBlockStyle {
	t := v.Theme
	if t == nil && env != nil {
		t = env.Theme
	}
	vivaTheme := dscore.VivaTheme(t)
	// banner 语言标签字号走 viva 的 FontSizeS 档：钉成几何表的 12px，
	// 不跟平台默认值漂移（否则不同机器上 banner 高度对不上）。
	vivaTheme.Metrics.FontSizeS = CodeBlockBannerFontSize
	style := mdlite.DefaultCodeBlockStyle(vivaTheme)
	// 语法色：与旧渲染路径（`Plugin.markdownLook`）同一个出处，避免两套配色。
	style.Look.Code[components.CodeComment-1] = t.CodeComment
	style.Look.Code[components.CodeString-1] = t.CodeString
	style.Look.Code[components.CodeNumber-1] = t.CodeNumber
	style.Look.Code[components.CodeKeyword-1] = t.CodeKeyword
	style.Look.Code[components.CodeFunction-1] = t.CodeFunction
	style.Look.Code[components.CodeVariable-1] = t.CodeVariable
	style.BannerFill = t.BgLayer1
	style.ButtonFill = t.Hover
	style.Success = t.Success
	style.Failure = t.Error
	style.FontSize = t.Fonts.Code
	style.BannerHeight = CodeBlockBannerHeight
	style.PadX = CodeBlockPadX
	style.PadY = CodeBlockPadY
	return style
}

// codeMeasureWidth 估算代码块的**真实列宽**（布局阶段拿不到列宽时的唯一依据）。
//
// 算式与真实列宽同源：主区宽 - 滚动口左右内边距
// （`ChatView.scroll` 的 `padding-inline` = 两侧净空 + 16，几何表 aligned 行）。
func (v *OfficialTranscriptView) codeMeasureWidth(env *pluginapi.Env) float32 {
	if env == nil {
		return CodeBlockMeasureMinWidth
	}
	width := env.MainWidth - 2*(ChatScrollPad+ComposerSideClearance)
	if width < CodeBlockMeasureMinWidth {
		width = CodeBlockMeasureMinWidth
	}
	return width
}

// previousCodeBlock 在上一帧里找同 key 同源码的代码块（复用视图状态）。
func (v *OfficialTranscriptView) previousCodeBlock(key, code string) (mdlite.CodeBlock, bool) {
	for i := range v.prevCodeBlocks {
		if v.prevCodeBlocks[i].key == key && v.prevCodeBlocks[i].block.Code == code {
			return v.prevCodeBlocks[i].block, true
		}
	}
	return mdlite.CodeBlock{}, false
}

// codeBlockIDs 造四个真实交互 id（复制 / 折行 / 横滚视口 / 横滚条）。
//
// 没有交互树时返回零值 id：mdlite 仍画预览按钮，但不声明死命中区
// （`CodeBlockIDs` 的注释：留空仍画预览，只是不可点）。
func (v *OfficialTranscriptView) codeBlockIDs(env *pluginapi.Env, key string) mdlite.CodeBlockIDs {
	if env == nil || env.Tree == nil {
		return mdlite.CodeBlockIDs{}
	}
	return mdlite.CodeBlockIDs{
		Copy:      env.Tree.ID(env.Feat, "transcript-code-copy-"+key),
		Wrap:      env.Tree.ID(env.Feat, "transcript-code-wrap-"+key),
		Scroll:    env.Tree.ID(env.Feat, "transcript-code-scroll-"+key),
		Scrollbar: env.Tree.ID(env.Feat, "transcript-code-scrollbar-"+key),
	}
}

// transcriptCopyFunc 取宿主的真实复制动作（`CopyResult` 优先：要原位反馈）。
func transcriptCopyFunc(env *pluginapi.Env) mdlite.CopyFunc {
	if env == nil {
		return nil
	}
	if env.Act.CopyResult != nil {
		return env.Act.CopyResult
	}
	if env.Act.Copy != nil {
		return func(text string) error {
			env.Act.Copy(text)
			return nil
		}
	}
	return nil
}

// drawToolCall 画 §5.7 工具调用行：callRow（前导 + 标题 + 分隔点 + 摘要）+ 展开体。
//
// 四态真值（结构树 §5.7「真假值边界」）：
//   - 有退出码时保留命令描述，不显示错误首行；
//   - error / stopped 不显示 diff 统计与后缀；
//   - per-file 链接只在 ok 出现（骨架不画链接）；
//   - preparing 不画子调用。
func (v *OfficialTranscriptView) drawToolCall(f ui.C, e TranscriptEntry) {
	t := v.Theme
	call := ui.Column(f).FillWidth().Radius(t.Radii.SM)
	call.Children(func() {
		row := ui.Row(f).FillWidth().Items(ui.Center).Gap(ContextSepMargin)
		row.Children(func() {
			ui.Slot(f).Size(CompactionLeading, CompactionLeading)
			title := e.Data.Tool
			if title == "" {
				title = "工具"
			}
			ui.Text(f, title).FontSize(t.Fonts.Secondary).LineHeight(t.Fonts.SecondaryLine).Color(t.LabelPrimary)
			ui.Slot(f).Size(ContextSepSize, ContextSepSize)
			ui.Text(f, toolSummary(e)).FontSize(t.Fonts.Secondary).Color(toolSummaryColor(t, e)).Ellipsis()
		})
		// 展开体：有输出或参数时给 IN/OUT 卡。
		if e.Data.ResultText != "" || e.Data.Args != "" {
			card := ui.Column(f).FillWidth().MarginSides(ToolIOCardMargin, 0, ToolIOCardMargin, ToolIOCardMargin).
				Radius(t.Radii.LG).Border(0.5, t.BorderL1)
			card.Children(func() {
				if e.Data.Args != "" {
					v.drawIOSection(f, "IN", e.Data.Args, false)
				}
				if e.Data.ResultText != "" {
					v.drawIOSection(f, "OUT", e.Data.ResultText, e.Data.Status == "error")
				}
			})
		}
	})
}

// drawIOSection 画 IN/OUT 段（12px 16px 内边距、150px 高度上限、14px 列间距）。
func (v *OfficialTranscriptView) drawIOSection(f ui.C, label, text string, isError bool) {
	t := v.Theme
	ui.Row(f).FillWidth().PadSides(ToolIOSectionPadY, ToolIOSectionPadX, ToolIOSectionPadY, ToolIOSectionPadX).
		Gap(ToolIOSectionColumnGap).Children(func() {
		ui.Text(f, label).FontSize(t.Fonts.Caption).Color(t.LabelCaption)
		color := t.LabelSecondary
		if isError {
			color = t.Error
		}
		ui.Text(f, text).FontSize(t.Fonts.Secondary).Color(color)
	})
}

// toolSummary 取工具行摘要：错误首行只在**没有退出码**时显示（§5.7 真值 2）。
func toolSummary(e TranscriptEntry) string {
	if e.Data.Summary != "" {
		return e.Data.Summary
	}
	if e.Data.ExitCode != nil {
		// 有退出码：保留命令描述，不显示错误首行。
		return e.Data.ExitCodeText()
	}
	if e.Data.Status == "error" && e.Data.ResultText != "" {
		return e.Data.ResultText
	}
	if e.Data.Status == "preparing" {
		return "准备中…"
	}
	if e.Data.ResultText != "" {
		return e.Data.ResultText
	}
	return "—"
}

// toolSummaryColor 是工具摘要色：错误 → error，中断 → warn，其余次级。
func toolSummaryColor(t *pluginapi.Theme, e TranscriptEntry) ui.Color {
	switch e.Data.Status {
	case "error":
		return t.Error
	case "stopped":
		return t.Warn
	default:
		return t.LabelSecondary
	}
}

// drawCommand 画 §5.8 斜杠命令生命周期。
func (v *OfficialTranscriptView) drawCommand(f ui.C, e TranscriptEntry) {
	t := v.Theme
	ui.Row(f).FillWidth().Items(ui.Center).Gap(ContextSepMargin).Radius(t.Radii.SM).Children(func() {
		ui.Slot(f).Size(CompactionLeading, CompactionLeading)
		name := e.Data.CommandName
		if name == "" {
			name = "命令"
		}
		ui.Text(f, "/"+name).FontSize(t.Fonts.Secondary).Color(t.LabelPrimary)
		ui.Slot(f).Size(ContextSepSize, ContextSepSize)
		outcome := e.Data.CommandOutcome
		if outcome == "" {
			outcome = "—"
		}
		ui.Text(f, outcome).FontSize(t.Fonts.Secondary).Color(t.LabelSecondary).Ellipsis()
	})
}

// drawManualCompaction 画 §5.9 `/compact` 与其压缩事务合一。
func (v *OfficialTranscriptView) drawManualCompaction(f ui.C, e TranscriptEntry) {
	t := v.Theme
	ui.Row(f).FillWidth().Items(ui.Center).Gap(ContextSepMargin).Children(func() {
		name := e.Data.CommandName
		if name == "" {
			name = "compact"
		}
		ui.Text(f, "/"+name).FontSize(t.Fonts.Secondary).Color(t.LabelPrimary)
		if e.Data.Text != "" {
			ui.Text(f, e.Data.Text).FontSize(t.Fonts.Secondary).Color(t.LabelSecondary).Ellipsis()
		}
	})
}

// drawCompaction 画 §5.10 自动压缩检查点（24px 行 + 前导 + 分隔 + 摘要）。
func (v *OfficialTranscriptView) drawCompaction(f ui.C, e TranscriptEntry) {
	t := v.Theme
	ui.Row(f).FillWidth().Items(ui.Center).Height(CompactionRowHeight).PadY(CompactionRowPadY).
		Children(func() {
			ui.Slot(f).Size(CompactionLeading, CompactionLeading).MarginSides(0, CompactionLeadingGap, 0, 0)
			ui.Text(f, "压缩").FontSize(t.Fonts.Secondary).Color(t.LabelPrimary)
			ui.Slot(f).Size(CompactionSepSize, CompactionSepSize).MarginSides(0, CompactionSepMargin, 0, CompactionSepMargin)
			summary := e.Data.CompactionSummary
			if summary == "" {
				summary = "—"
			}
			ui.Text(f, summary).FontSize(t.Fonts.Secondary).Color(t.LabelTertiary).Ellipsis()
		})
}

// drawModelRetry 画 §5.11 模型重试链（一行 + 可展开明细）。
func (v *OfficialTranscriptView) drawModelRetry(f ui.C, e TranscriptEntry) {
	t := v.Theme
	ui.Column(f).FillWidth().Children(func() {
		ui.Row(f).FillWidth().Items(ui.Center).Gap(RetrySummaryGap).PadY(RetrySummaryPadY).Children(func() {
			ui.Text(f, e.Data.RetryText()).FontSize(RetryRowFontSize).LineHeight(RetryRowLineHeight).Color(t.LabelTertiary)
		})
		if e.Data.RetryDelayMs != nil && *e.Data.RetryDelayMs > 0 {
			ui.Row(f).FillWidth().PadSides(RetryDetailsMarginTop, 0, 0, RetryDetailsPadLeft).Children(func() {
				ui.Text(f, "延迟 "+itoa64(*e.Data.RetryDelayMs)+" ms").FontSize(t.Fonts.Caption).Color(t.LabelSecondary)
			})
		}
	})
}

// drawTurnError 画 §5.12 turn-error / §5.13 turn-max-tokens 的共享栅格。
//
// 栅格 `10px minmax(0,1fr) auto`、gap 8、padding 2px 0（MessageItem.module.css:268-276）。
func (v *OfficialTranscriptView) drawTurnError(f ui.C, e TranscriptEntry, maxTokens bool) {
	t := v.Theme
	title := "Turn 错误"
	color := t.Error
	if maxTokens {
		title = "输出达到 token 上限"
		color = t.Warn
	} else if e.Data.ErrorCode == "ACCOUNT_SIGNED_OUT" || e.Data.ErrorCode == "INTERRUPTED" {
		title = "任务已停止"
	}
	ui.Row(f).FillWidth().Items(ui.Start).Gap(TurnErrorGap).PadY(TurnErrorPadY).Children(func() {
		ui.Slot(f).Size(TurnErrorDotColumn, TurnErrorDotColumn)
		ui.Column(f).FillWidth().MinWidth(0).Children(func() {
			ui.Row(f).Children(func() {
				ui.Text(f, title).FontSize(t.Fonts.Secondary).Weight(gfx.WeightSemibold).Color(color).
					MarginSides(0, TurnErrorTitleMarginRight, 0, 0)
				if e.Data.ErrorMessage != "" {
					ui.Text(f, e.Data.ErrorMessage).FontSize(t.Fonts.Secondary).Color(t.LabelSecondary)
				}
			})
			// INTERRUPTED 不显示 code 胶囊（§5.12 真值）。
			if e.Data.ErrorCode != "" && e.Data.ErrorCode != "INTERRUPTED" {
				ui.Text(f, e.Data.ErrorCode).FontSize(t.Fonts.Caption).Mono().Color(t.LabelTertiary)
			}
			if maxTokens && e.Data.MaxTokensLimit != nil {
				ui.Text(f, "limit "+itoa64(*e.Data.MaxTokensLimit)).FontSize(t.Fonts.Caption).Mono().Color(t.LabelTertiary)
			}
		})
	})
}

// drawTurnProcess 画 §5.14 Turn 级过程披露 + 耗时。
//
// **Turn 未闭合时官方返回 null**（不是 CSS 隐藏）——投影层只会对已闭合
// Turn 产出这一条，所以这里直接画。
func (v *OfficialTranscriptView) drawTurnProcess(f ui.C, e TranscriptEntry) {
	t := v.Theme
	ui.Row(f).FillWidth().Items(ui.Center).Height(TurnProcessHeight).
		PadSides(0, 0, TurnProcessPadBottom, 0).Children(func() {
		label := "已工作"
		if e.Data.DurationMs != nil {
			label = "耗时 " + itoa64(*e.Data.DurationMs/1000) + " 秒"
		}
		ui.Text(f, label).FontSize(t.Fonts.Body).Color(t.LabelSecondary).Ellipsis()
		ui.Slot(f).Size(TurnProcessChevron, TurnProcessChevron)
	})
}

// drawTurnTail 画 §5.15 Turn 尾：动作行 + 用量 + 扩展位。
func (v *OfficialTranscriptView) drawTurnTail(f ui.C, e TranscriptEntry) {
	t := v.Theme
	ui.Column(f).FillWidth().Gap(TurnTailGap).Children(func() {
		actions := ui.Row(f).FillWidth().Items(ui.Center).Gap(ActionsRowGap).Height(ActionsRowHeight).
			MarginSides(TurnTailActionsMarginTop, 0, 0, AssistantActionsMarginLeft)
		actions.Children(func() {
			if e.TimeKnown() {
				ui.Text(f, e.TimeText()).FontSize(ActionsTimeEndFontSize).Color(t.LabelTertiary)
			}
			if usage := e.Data.Usage.UsageText(); usage != "" {
				ui.Text(f, usage).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
			}
		})
	})
}

// drawQuestionReply 画 §5.18 已答问题批次。
func (v *OfficialTranscriptView) drawQuestionReply(f ui.C, e TranscriptEntry) {
	t := v.Theme
	ui.Column(f).FillWidth().Gap(4).Children(func() {
		ui.Text(f, e.Data.Text).FontSize(t.Fonts.Body).LineHeight(t.Fonts.BodyLine).Color(t.LabelPrimary)
		if e.Data.Outcome != "" {
			ui.Text(f, e.Data.Outcome).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		}
	})
}

// drawWorkflowRun 画 §5.20 工作流运行（外部包节点：只摆位，不定尺寸）。
func (v *OfficialTranscriptView) drawWorkflowRun(f ui.C, e TranscriptEntry) {
	t := v.Theme
	card := ui.Column(f).FillWidth().Border(0.5, t.BorderL1).Radius(t.Radii.LG)
	card.Children(func() {
		ui.Row(f).FillWidth().Items(ui.Center).Gap(6).PadSides(8, 12, 8, 12).Children(func() {
			name := e.Data.RunName
			if name == "" {
				name = "工作流"
			}
			ui.Text(f, name).FontSize(t.Fonts.Secondary).Color(t.LabelPrimary)
			if e.Data.StopReason != "" {
				ui.Text(f, e.Data.StopReason).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
			}
		})
	})
}

// drawUnknown 画 §5.17 未知 surface 兜底：原样 JSON，**不编摘要**。
func (v *OfficialTranscriptView) drawUnknown(f ui.C, e TranscriptEntry) {
	t := v.Theme
	raw := e.Data.Raw
	if raw == "" {
		raw = "{}"
	}
	ui.Column(f).FillWidth().PadY(TurnErrorPadY).Children(func() {
		ui.Text(f, raw).FontSize(t.Fonts.Code).LineHeight(t.Fonts.CodeLine).Mono().Color(t.LabelSecondary)
	})
}

// spansText 把一组 inline span 拼成纯文本（保持原顺序与换行）。
func spansText(spans []markdown.Span) string {
	var b strings.Builder
	for _, span := range spans {
		b.WriteString(span.Text)
	}
	return b.String()
}

// allMono 报告一段文本是不是**整段行内代码**（官方 MarkdownText 的 Mono span）。
func allMono(spans []markdown.Span) bool {
	if len(spans) == 0 {
		return false
	}
	mono := false
	for _, span := range spans {
		if span.Text == "" || span.Text == markdown.Newline {
			continue
		}
		if span.Style&markdown.Mono == 0 {
			return false
		}
		mono = true
	}
	return mono
}

// ── 审批卡（官方 ApprovalPanel，不是 transcript 节点）────────────────────

// ApprovalBoxes 是审批卡的上一帧几何（测试与 AfterDraw 用）。
type ApprovalBoxes struct {
	// Card 是卡片根（`radius-xl` + `max-width: chat-content-width`）。
	Card ui.Box
	// Header 是头部（`padding:10px 16px`）。
	Header ui.Box
	// Body 是体（`padding:12px 16px 0;gap:6px`）。
	Body ui.Box
	// Options 是选项按钮（**顺序 = 宿主给的顺序**，不重排、不造 id）。
	Options []ApprovalOptionBox
	// Rendered 报告这一帧是否真的画了（零值 Approval 不画）。
	Rendered bool
}

// ApprovalOptionBox 是一个选项按钮的渲染结果。
type ApprovalOptionBox struct {
	// Box 是按钮盒子。
	Box ui.Box
	// Label 是按钮文案（宿主没给 name 时用 kind，两者都空用「未知」）。
	Label string
	// Disabled 表示这一项当前不可用（缺 requestId / optionId）。
	Disabled bool
}

// DrawApprovalCard 画一次待审批的工具调用。
//
// 为什么单独一个入口而不是一个 ChatNode kind：官方审批走 **composer 接管**
// （`ChatView.tsx:293-295` 注释：审批是 composer takeover，问题答复才是
// transcript 行）。把它做成 transcript 节点会与 composer 上那条重复
// （历史上同屏出现两条「需要审批 bash」）。
//
// 三条真值（跨线 §12 H13）：
//   - `RequestID` 为空 → 按钮必须禁用（回传不出去，不编 id）；
//   - `Options` 为空 → 这次审批点不动（不自己造 optionId）；
//   - 风险等级**不显示**（宿主没有这个字段，不许从命令里算一个出来）。
func (v *OfficialTranscriptView) DrawApprovalCard(f ui.C, env *pluginapi.Env, approval pluginapi.Approval) ApprovalBoxes {
	t := v.Theme
	var boxes ApprovalBoxes
	card := ui.Column(f).FillWidth().Radius(t.Radii.XL).Border(0.5, t.BorderL1)
	boxes.Card = card
	boxes.Rendered = true
	card.Children(func() {
		header := ui.Row(f).FillWidth().Items(ui.Center).Gap(ApprovalBodyGap).
			PadSides(ApprovalHeaderPadY, ApprovalHeaderPadX, ApprovalHeaderPadY, ApprovalHeaderPadX)
		boxes.Header = header
		header.Children(func() {
			title := approval.Title
			if title == "" {
				title = "需要审批"
			}
			ui.Text(f, title).FontSize(t.Fonts.Body).Color(t.LabelPrimary).Ellipsis()
		})
		body := ui.Column(f).FillWidth().Gap(ApprovalBodyGap).
			PadSides(ApprovalBodyPadTop, ApprovalBodyPadX, 0, ApprovalBodyPadX)
		boxes.Body = body
		body.Children(func() {
			if approval.Args != "" || approval.Cmd != "" {
				text := approval.Args
				if text == "" {
					text = approval.Cmd
				}
				ui.Text(f, text).FontSize(t.Fonts.Code).LineHeight(t.Fonts.CodeLine).Mono().Color(t.LabelSecondary)
			}
			if approval.Detail != "" {
				ui.Text(f, approval.Detail).FontSize(t.Fonts.Secondary).Color(t.LabelSecondary)
			}
			row := ui.Row(f).Gap(ActionsRowGap).PadSides(0, 0, ApprovalBodyPadTop, 0)
			boxes.Options = boxes.Options[:0]
			row.Children(func() {
				for _, option := range approval.Options {
					label := option.Name
					if label == "" {
						label = option.Kind
					}
					if label == "" {
						label = UnknownText
					}
					disabled := approval.RequestID == "" || option.OptionID == ""
					button := ui.Button(f, label).Small()
					if disabled {
						// 没有 requestId / optionId = 回传不出去：**禁用**，不假装成功。
						button = button.Disabled(true)
					}
					boxes.Options = append(boxes.Options, ApprovalOptionBox{
						Box: button, Label: label, Disabled: disabled,
					})
				}
			})
		})
	})
	return boxes
}

// optionID 造一个稳定的选项交互 id。
//
// 同一 label 在两次渲染里必须给**同一个** id，否则 viva 会把按钮当成新节点
// 重挂（焦点/悬停状态丢失）。所以按 label 起名，不用指针/自增。
func (v *OfficialTranscriptView) optionID(env *pluginapi.Env, label string) gfx.ID {
	if env == nil || env.Tree == nil {
		return gfx.ID{}
	}
	return env.Tree.ID(env.Feat, "approval-option-"+label)
}

// OfficialTranscript 返回主区 transcript 渲染器（惰性初始化）。
//
// 这是本包对外的**唯一**入口：主题/图元来自 ds-core（`Register` 注入），
// 投影与数据由调用方提供（壳持有 apiclient 的帧，插件不自己建 Transport——
// 界面线 R2/R7）。
//
// 为什么放在插件上而不是包级单例：主题是**每进程一份**的共享指针，
// 包级单例会在多内核（测试）之间串颜色。
// OfficialTranscript 返回（惰性创建）官方 transcript 渲染器。
//
// **必须是惰性而不是每次新建**：渲染器持有**跨两阶段**的状态——
// Draw 阶段占位的代码块（`codeBlocks`）、上一帧的视图状态（横滚位移）、
// 帧号去重（`frameSeq`/`paintedSeq`）。每次调用都 new 一个的话，
// 覆画阶段（AfterDraw）拿到的是空实例，代码块一个都画不出来
// （GT3 的真机断言当场抓到这个：标题/列表在，代码块整块消失）。
func (p *Plugin) OfficialTranscript() *OfficialTranscriptView {
	if p.officialTranscript == nil {
		p.officialTranscript = NewOfficialTranscriptView(p.theme, p.prims)
	}
	return p.officialTranscript
}
