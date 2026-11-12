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
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
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
	HostID ui.ID
	SelfID ui.ID
}

// RowBox 是渲染出来的**一行**（对应官方一个 `.flowItem`）。
type RowBox struct {
	// Key 是稳定渲染键（官方 `data-chat-node-key`）。
	Key string
	// Kind 是官方 kind（官方 `data-chat-flow-kind`）。
	Kind ChatNodeKind
	// Box 是这一行的盒子（`min-width:0` 的 flowItem）。
	Box flex.Box
}

// TranscriptBoxes 是上一帧的容器几何（AfterDraw / 测试用）。
//
// 为什么留着：viva 的绘制顺序不受调用方控制，Markdown 这类"必须先量后画"
// 的东西要等布局定下来；而测试要断言"双层滚动口真的存在"也只能读几何。
type TranscriptBoxes struct {
	// ScrollBody 是宿主滚动口（`HostScroll` 为 false 时为零值）。
	ScrollBody flex.Box
	// Frame / Root / Scroll / Column / Flow 是 ChatView 的五层容器。
	Frame  flex.Box
	Root   flex.Box
	Scroll flex.Box
	Column flex.Box
	Flow   flex.Box
	// Rows 是逐条消息行（顺序 = 节点顺序）。
	Rows []RowBox
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
}

// NewOfficialTranscriptView 造一个渲染器。
func NewOfficialTranscriptView(theme *pluginapi.Theme, prims pluginapi.Primitives) *OfficialTranscriptView {
	return &OfficialTranscriptView{Theme: theme, Prims: prims}
}

// Draw 画完整 transcript：容器链 + 逐条节点。
//
// 入参 entries 必须是**已经投影好的**节点（projection 层负责帧 → 节点）；
// 本函数不解析 journal、不发请求、不猜数据。
func (v *OfficialTranscriptView) Draw(f *flex.Frame, env *pluginapi.Env, vp TranscriptViewport, entries []TranscriptEntry) {
	t := v.Theme
	if t == nil && env != nil {
		t = env.Theme
	}
	v.Boxes = TranscriptBoxes{HostScroll: vp.HostScroll, SelfScrolls: !vp.HostScroll}

	// ── ChatView 五层容器（frame → root → scroll → column → flow）──
	//
	// 挂载点二选一（官方 `use-chat-viewport.ts:118-119` 的判据）：
	//
	//	外层有宿主滚动口 → 自滚动口降级 overflow-y:visible（纵向滚动归宿主）
	//	否则             → 自滚动口自己滚（独立挂载 Chat）
	drawChatView := func() {
		// ── ChatView.frame（container-type:inline-size，给浮动控件做断点）──
		// 宿主滚动口存在时官方降级 flex:0 0 auto;min-height:auto;height:auto
		v.Boxes.Frame = f.Column().FillWidth().Grow(1).MinHeight(0)
		v.Boxes.Frame.Children(func() {
			// ── ChatView.root[data-chat-following-tail]（overflow-y:clip）──
			v.Boxes.Root = f.Column().FillWidth().Grow(1).MinHeight(0).Clip()
			v.Boxes.Root.Children(func() {
				// ── ChatView.scroll（自滚动口；宿主口内降级 overflow-y:visible）──
				if !vp.HostScroll && !vp.SelfID.IsZero() {
					v.Boxes.Scroll = f.ScrollArea(vp.SelfID).FillWidth().Grow(1).MinHeight(0).
						PadSides(ChatScrollPad, ChatScrollPad+ComposerSideClearance, ChatScrollPad, ChatScrollPad+ComposerSideClearance)
				} else {
					v.Boxes.Scroll = f.Column().FillWidth().Grow(1).MinHeight(0).
						PadSides(ChatScrollPad, ChatScrollPad+ComposerSideClearance, ChatScrollPad, ChatScrollPad+ComposerSideClearance)
				}
				v.Boxes.Scroll.Children(func() {
					// ── ChatView.column（唯一行序容器，max-width: chat-content-width）──
					v.Boxes.Column = f.Column().FillWidth().MinHeight(0)
					v.Boxes.Column.Children(func() {
						// ── flow 槽（display:contents：保留布局但不保留盒模型）──
						v.Boxes.Flow = f.Column().FillWidth()
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
		v.Boxes.ScrollBody = f.ScrollArea(vp.HostID).FillWidth().Grow(1).MinHeight(0).
			MarginSides(0, ScrollBodyMarginRight, 0, 0)
		v.Boxes.ScrollBody.Children(drawChatView)
		return
	}
	drawChatView()
}

// drawRow 画一条节点（官方 `.flowItem` 外壳 + kind 分派）。
func (v *OfficialTranscriptView) drawRow(f *flex.Frame, env *pluginapi.Env, e TranscriptEntry) RowBox {
	row := f.Column().FillWidth().MinWidth(0)
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
func (v *OfficialTranscriptView) drawUser(f *flex.Frame, env *pluginapi.Env, e TranscriptEntry) {
	t := v.Theme
	f.Row().FillWidth().Justify(flex.End).Gap(UserRowGap).Children(func() {
		stack := f.Column().Items(flex.End).Gap(UserStackGap).MinWidth(0).
			MaxWidthPercent(UserStackWidthCap * 100)
		stack.Children(func() {
			if e.Data.Text != "" {
				f.Text(e.Data.Text).FontSize(t.Fonts.Body).LineHeight(t.Fonts.BodyLine).Color(t.LabelPrimary)
			}
			// 引用摘要（.referenceSummary：13px / 18px + delta-secondary，label-tertiary）
			if len(e.Data.References) > 0 {
				f.Text(strings.Join(e.Data.References, " · ")).
					FontSize(ReferenceSummaryFontSize).LineHeight(ReferenceSummaryLineHeight).
					Color(t.LabelTertiary).Ellipsis()
			}
			// 附件行（.attachmentRow：右对齐、wrap、gap 8）
			if len(e.Data.Attachments) > 0 {
				f.Row().Justify(flex.End).Gap(AttachmentRowGap).Children(func() {
					for _, attachment := range e.Data.Attachments {
						v.drawAttachment(f, attachment)
					}
				})
			}
		})
	})
}

// drawAttachment 画一个附件：图片走 64×64 瓦片，文件走 240×64 卡片。
func (v *OfficialTranscriptView) drawAttachment(f *flex.Frame, attachment TranscriptAttachment) {
	t := v.Theme
	if attachment.Image {
		// 图片瓦片（MessageImage.tile）：64×64，多图时降级。
		f.Slot().Size(ImageTileSize, ImageTileSize).Radius(t.Radii.SM).Border(0.5, t.BorderL2)
		return
	}
	card := f.Row().Size(FileCardWidth, FileCardMinHeight).Gap(FileCardGap).Radius(t.Radii.LG).
		Border(0.5, t.BorderL2).PadSides(FileCardPadY, FileCardPadX, FileCardPadY, FileCardPadX)
	card.Children(func() {
		f.Slot().Size(FileIconSize, FileIconSize)
		f.Column().MinWidth(0).Children(func() {
			name := attachment.Name
			if name == "" {
				name = UnknownText
			}
			f.Text(name).FontSize(FileNameFontSize).LineHeight(FileNameLineHeight).
				Weight(ui.WeightMedium).Color(t.LabelPrimary).Ellipsis()
			if attachment.Meta != "" {
				f.Text(attachment.Meta).FontSize(FileMetaFontSize).LineHeight(FileMetaLineHeight).
					Color(t.LabelTertiary).Ellipsis()
			}
		})
	})
}

// drawSteering 画 §5.2 活动 Turn 内被接收的用户消息。
//
// 与 user 的差别：行距是 `calc(44px + delta)`（保住 Turn 尾槽），
// 且行上带 `data-pending-steering`（Host 权威待入）。
func (v *OfficialTranscriptView) drawSteering(f *flex.Frame, env *pluginapi.Env, e TranscriptEntry) {
	t := v.Theme
	f.Row().FillWidth().Justify(flex.End).Gap(UserRowGap).
		MarginSides(ChatFlowGapSteeringBatch-ChatFlowGapDefault, 0, 0, 0).Children(func() {
		f.Column().Items(flex.End).Gap(UserStackGap).MinWidth(0).Children(func() {
			f.Text(e.Data.Text).FontSize(t.Fonts.Body).LineHeight(t.Fonts.BodyLine).Color(t.LabelSecondary)
		})
	})
}

// drawDisclosureRow 画 §5.3 context / §5.16 system-prompt 的共享 24px 行壳。
//
// 官方 `DisclosureRow` 的精确几何**未取证**（`ui-primitives` 没有 rc.2 src），
// 见几何表 derived 行——按包自身 CSS 注释的"leading 16 + gap 6 + 13/24 标题"
// 二手证据摆位，**不得反推像素**。
func (v *OfficialTranscriptView) drawDisclosureRow(f *flex.Frame, e TranscriptEntry, title, subtitle string) {
	t := v.Theme
	row := f.Row().FillWidth().Items(flex.Center).Gap(CompactionLeadingGap).PadY(2)
	row.Children(func() {
		f.Slot().Size(CompactionLeading, CompactionLeading)
		f.Text(title).FontSize(t.Fonts.Secondary).LineHeight(t.Fonts.SecondaryLine).Color(t.LabelTertiary)
		if subtitle != "" {
			f.Slot().Size(ContextSepSize, ContextSepSize)
			f.Text(subtitle).FontSize(t.Fonts.Caption).Color(t.LabelCaption).Ellipsis()
		}
		if e.Data.Text != "" {
			f.Text(e.Data.Text).FontSize(t.Fonts.Secondary).Color(t.LabelSecondary).Ellipsis()
		}
	})
}

// drawTurnTrigger 画 §5.4 记录型唤醒 / 续写回执。
func (v *OfficialTranscriptView) drawTurnTrigger(f *flex.Frame, e TranscriptEntry) {
	t := v.Theme
	card := f.Column().FillWidth().Border(0.5, t.BorderL1).Radius(t.Radii.XL)
	card.Children(func() {
		header := f.Row().FillWidth().Items(flex.Center).Gap(TurnTriggerHeaderGap).
			PadSides(TurnTriggerHeaderPadY, TurnTriggerHeaderPadX, TurnTriggerHeaderPadY, TurnTriggerHeaderPadX)
		header.Children(func() {
			f.Text(triggerTitle(e)).FontSize(t.Fonts.Body).Color(t.LabelPrimary)
			f.Text(e.TimeText()).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		})
		if e.Data.Text != "" {
			f.Row().FillWidth().PadSides(0, TurnTriggerHeaderPadX, TurnTriggerBodyPadBottom, TurnTriggerBodyPadLeft).Children(func() {
				f.Text(e.Data.Text).FontSize(t.Fonts.Caption).Color(t.LabelSecondary).
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
func (v *OfficialTranscriptView) drawCommandInput(f *flex.Frame, e TranscriptEntry) {
	t := v.Theme
	f.Row().FillWidth().Items(flex.Center).Gap(6).Children(func() {
		f.Text(e.Data.Text).FontSize(t.Fonts.Body).Mono().Color(t.LabelSecondary)
	})
}

// drawAssistant 画 §5.6 助手正文：markdown 块分派 + reasoning + 中断标记。
func (v *OfficialTranscriptView) drawAssistant(f *flex.Frame, env *pluginapi.Env, e TranscriptEntry) {
	t := v.Theme
	root := f.Column().FillWidth().Gap(AssistantBlockGap)
	root.Children(func() {
		if e.Data.Reasoning != "" {
			// reasoning 分区块间距 4px（AssistantMarkdown.module.css:24-29）
			f.Column().FillWidth().Gap(AssistantReasoningBlockGap).Children(func() {
				f.Row().FillWidth().Items(flex.Center).Gap(CompactionLeadingGap).Children(func() {
					f.Slot().Size(TurnProcessChevron, TurnProcessChevron)
					f.Text("思考").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
				})
				f.Text(e.Data.Reasoning).FontSize(t.Fonts.Secondary).Color(t.LabelSecondary)
			})
		}
		if e.Data.Text != "" {
			v.drawMarkdown(f, env, e.Data.Text)
		}
		if e.Data.Status == "stopped" {
			// 中断标记 .stopped：11px/18px，左对齐
			f.Row().Justify(flex.Start).Children(func() {
				f.Text("已停止").FontSize(StoppedFontSize).LineHeight(StoppedLineHeight).
					PadX(StoppedPadX).Radius(t.Radii.SM).Color(t.LabelTertiary)
			})
		}
	})
}

// drawMarkdown 画 markdown 块：段落 / 标题 / 列表 → 文本；围栏 → 代码卡；表格 → 等宽文本。
//
// **行内代码**（Mono span）单独成段时走等宽字体——精确的行内背景色
// （`--dsw-alias-markdown-inline-code`）在 primitives 里，未取证（§9 第 1/2 条）。
func (v *OfficialTranscriptView) drawMarkdown(f *flex.Frame, env *pluginapi.Env, text string) {
	t := v.Theme
	var doc markdown.Document
	doc.Parse(text)
	for i := range doc.Blocks {
		block := doc.Blocks[i]
		switch block.Kind {
		case markdown.Code:
			v.drawCodeBlock(f, block)
		case markdown.Rule:
			f.Divider()
		case markdown.Table:
			// 宽表格突破（md-table-wide 的 100cqw）在 Viva 里没有容器查询，
			// 见几何表 deviation 行：这里按等宽文本原样渲染。
			for _, cell := range doc.Cells(&block) {
				f.Text(spansText(doc.CellSpans(cell))).FontSize(t.Fonts.Code).Mono().Color(t.LabelSecondary)
			}
		default:
			body := spansText(doc.Spans(&block))
			if body == "" {
				continue
			}
			box := f.Text(body).FontSize(t.Fonts.Body).LineHeight(t.Fonts.BodyLine).Color(t.LabelPrimary)
			if allMono(doc.Spans(&block)) {
				box.Mono().FontSize(t.Fonts.Code)
			}
		}
	}
}

// drawCodeBlock 画围栏代码块（banner + 等宽正文）。
//
// 官方走 `FenceCodeBlock` / `CodeBlock`（primitives），**不得手写高亮**；
// 这里只做骨架：语言名 banner + 等宽正文，精确高亮与 copy 按钮归代码块线。
func (v *OfficialTranscriptView) drawCodeBlock(f *flex.Frame, block markdown.Block) {
	t := v.Theme
	card := f.Column().FillWidth().Radius(t.Radii.SM).Border(0.5, t.BorderL1)
	card.Children(func() {
		banner := f.Row().FillWidth().PadSides(4, 8, 4, 8)
		banner.Children(func() {
			lang := strings.TrimSpace(block.Info)
			if lang == "" {
				lang = "text"
			}
			f.Text(lang).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		})
		f.Text(block.Text).FontSize(t.Fonts.Code).LineHeight(t.Fonts.CodeLine).Mono().Color(t.LabelPrimary)
	})
}

// drawToolCall 画 §5.7 工具调用行：callRow（前导 + 标题 + 分隔点 + 摘要）+ 展开体。
//
// 四态真值（结构树 §5.7「真假值边界」）：
//   - 有退出码时保留命令描述，不显示错误首行；
//   - error / stopped 不显示 diff 统计与后缀；
//   - per-file 链接只在 ok 出现（骨架不画链接）；
//   - preparing 不画子调用。
func (v *OfficialTranscriptView) drawToolCall(f *flex.Frame, e TranscriptEntry) {
	t := v.Theme
	call := f.Column().FillWidth().Radius(t.Radii.SM)
	call.Children(func() {
		row := f.Row().FillWidth().Items(flex.Center).Gap(ContextSepMargin)
		row.Children(func() {
			f.Slot().Size(CompactionLeading, CompactionLeading)
			title := e.Data.Tool
			if title == "" {
				title = "工具"
			}
			f.Text(title).FontSize(t.Fonts.Secondary).LineHeight(t.Fonts.SecondaryLine).Color(t.LabelPrimary)
			f.Slot().Size(ContextSepSize, ContextSepSize)
			f.Text(toolSummary(e)).FontSize(t.Fonts.Secondary).Color(toolSummaryColor(t, e)).Ellipsis()
		})
		// 展开体：有输出或参数时给 IN/OUT 卡。
		if e.Data.ResultText != "" || e.Data.Args != "" {
			card := f.Column().FillWidth().MarginSides(ToolIOCardMargin, 0, ToolIOCardMargin, ToolIOCardMargin).
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
func (v *OfficialTranscriptView) drawIOSection(f *flex.Frame, label, text string, isError bool) {
	t := v.Theme
	f.Row().FillWidth().PadSides(ToolIOSectionPadY, ToolIOSectionPadX, ToolIOSectionPadY, ToolIOSectionPadX).
		Gap(ToolIOSectionColumnGap).Children(func() {
		f.Text(label).FontSize(t.Fonts.Caption).Color(t.LabelCaption)
		color := t.LabelSecondary
		if isError {
			color = t.Error
		}
		f.Text(text).FontSize(t.Fonts.Secondary).Color(color)
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
func (v *OfficialTranscriptView) drawCommand(f *flex.Frame, e TranscriptEntry) {
	t := v.Theme
	f.Row().FillWidth().Items(flex.Center).Gap(ContextSepMargin).Radius(t.Radii.SM).Children(func() {
		f.Slot().Size(CompactionLeading, CompactionLeading)
		name := e.Data.CommandName
		if name == "" {
			name = "命令"
		}
		f.Text("/" + name).FontSize(t.Fonts.Secondary).Color(t.LabelPrimary)
		f.Slot().Size(ContextSepSize, ContextSepSize)
		outcome := e.Data.CommandOutcome
		if outcome == "" {
			outcome = "—"
		}
		f.Text(outcome).FontSize(t.Fonts.Secondary).Color(t.LabelSecondary).Ellipsis()
	})
}

// drawManualCompaction 画 §5.9 `/compact` 与其压缩事务合一。
func (v *OfficialTranscriptView) drawManualCompaction(f *flex.Frame, e TranscriptEntry) {
	t := v.Theme
	f.Row().FillWidth().Items(flex.Center).Gap(ContextSepMargin).Children(func() {
		name := e.Data.CommandName
		if name == "" {
			name = "compact"
		}
		f.Text("/" + name).FontSize(t.Fonts.Secondary).Color(t.LabelPrimary)
		if e.Data.Text != "" {
			f.Text(e.Data.Text).FontSize(t.Fonts.Secondary).Color(t.LabelSecondary).Ellipsis()
		}
	})
}

// drawCompaction 画 §5.10 自动压缩检查点（24px 行 + 前导 + 分隔 + 摘要）。
func (v *OfficialTranscriptView) drawCompaction(f *flex.Frame, e TranscriptEntry) {
	t := v.Theme
	f.Row().FillWidth().Items(flex.Center).Height(CompactionRowHeight).PadY(CompactionRowPadY).
		Children(func() {
			f.Slot().Size(CompactionLeading, CompactionLeading).MarginSides(0, CompactionLeadingGap, 0, 0)
			f.Text("压缩").FontSize(t.Fonts.Secondary).Color(t.LabelPrimary)
			f.Slot().Size(CompactionSepSize, CompactionSepSize).MarginSides(0, CompactionSepMargin, 0, CompactionSepMargin)
			summary := e.Data.CompactionSummary
			if summary == "" {
				summary = "—"
			}
			f.Text(summary).FontSize(t.Fonts.Secondary).Color(t.LabelTertiary).Ellipsis()
		})
}

// drawModelRetry 画 §5.11 模型重试链（一行 + 可展开明细）。
func (v *OfficialTranscriptView) drawModelRetry(f *flex.Frame, e TranscriptEntry) {
	t := v.Theme
	f.Column().FillWidth().Children(func() {
		f.Row().FillWidth().Items(flex.Center).Gap(RetrySummaryGap).PadY(RetrySummaryPadY).Children(func() {
			f.Text(e.Data.RetryText()).FontSize(RetryRowFontSize).LineHeight(RetryRowLineHeight).Color(t.LabelTertiary)
		})
		if e.Data.RetryDelayMs != nil && *e.Data.RetryDelayMs > 0 {
			f.Row().FillWidth().PadSides(RetryDetailsMarginTop, 0, 0, RetryDetailsPadLeft).Children(func() {
				f.Text("延迟 " + itoa64(*e.Data.RetryDelayMs) + " ms").FontSize(t.Fonts.Caption).Color(t.LabelSecondary)
			})
		}
	})
}

// drawTurnError 画 §5.12 turn-error / §5.13 turn-max-tokens 的共享栅格。
//
// 栅格 `10px minmax(0,1fr) auto`、gap 8、padding 2px 0（MessageItem.module.css:268-276）。
func (v *OfficialTranscriptView) drawTurnError(f *flex.Frame, e TranscriptEntry, maxTokens bool) {
	t := v.Theme
	title := "Turn 错误"
	color := t.Error
	if maxTokens {
		title = "输出达到 token 上限"
		color = t.Warn
	} else if e.Data.ErrorCode == "ACCOUNT_SIGNED_OUT" || e.Data.ErrorCode == "INTERRUPTED" {
		title = "任务已停止"
	}
	f.Row().FillWidth().Items(flex.Start).Gap(TurnErrorGap).PadY(TurnErrorPadY).Children(func() {
		f.Slot().Size(TurnErrorDotColumn, TurnErrorDotColumn)
		f.Column().FillWidth().MinWidth(0).Children(func() {
			f.Row().Children(func() {
				f.Text(title).FontSize(t.Fonts.Secondary).Weight(ui.WeightSemibold).Color(color).
					MarginSides(0, TurnErrorTitleMarginRight, 0, 0)
				if e.Data.ErrorMessage != "" {
					f.Text(e.Data.ErrorMessage).FontSize(t.Fonts.Secondary).Color(t.LabelSecondary)
				}
			})
			// INTERRUPTED 不显示 code 胶囊（§5.12 真值）。
			if e.Data.ErrorCode != "" && e.Data.ErrorCode != "INTERRUPTED" {
				f.Text(e.Data.ErrorCode).FontSize(t.Fonts.Caption).Mono().Color(t.LabelTertiary)
			}
			if maxTokens && e.Data.MaxTokensLimit != nil {
				f.Text("limit " + itoa64(*e.Data.MaxTokensLimit)).FontSize(t.Fonts.Caption).Mono().Color(t.LabelTertiary)
			}
		})
	})
}

// drawTurnProcess 画 §5.14 Turn 级过程披露 + 耗时。
//
// **Turn 未闭合时官方返回 null**（不是 CSS 隐藏）——投影层只会对已闭合
// Turn 产出这一条，所以这里直接画。
func (v *OfficialTranscriptView) drawTurnProcess(f *flex.Frame, e TranscriptEntry) {
	t := v.Theme
	f.Row().FillWidth().Items(flex.Center).Height(TurnProcessHeight).
		PadSides(0, 0, TurnProcessPadBottom, 0).Children(func() {
		label := "已工作"
		if e.Data.DurationMs != nil {
			label = "耗时 " + itoa64(*e.Data.DurationMs/1000) + " 秒"
		}
		f.Text(label).FontSize(t.Fonts.Body).Color(t.LabelSecondary).Ellipsis()
		f.Slot().Size(TurnProcessChevron, TurnProcessChevron)
	})
}

// drawTurnTail 画 §5.15 Turn 尾：动作行 + 用量 + 扩展位。
func (v *OfficialTranscriptView) drawTurnTail(f *flex.Frame, e TranscriptEntry) {
	t := v.Theme
	f.Column().FillWidth().Gap(TurnTailGap).Children(func() {
		actions := f.Row().FillWidth().Items(flex.Center).Gap(ActionsRowGap).Height(ActionsRowHeight).
			MarginSides(TurnTailActionsMarginTop, 0, 0, AssistantActionsMarginLeft)
		actions.Children(func() {
			if e.TimeKnown() {
				f.Text(e.TimeText()).FontSize(ActionsTimeEndFontSize).Color(t.LabelTertiary)
			}
			if usage := e.Data.Usage.UsageText(); usage != "" {
				f.Text(usage).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
			}
		})
	})
}

// drawQuestionReply 画 §5.18 已答问题批次。
func (v *OfficialTranscriptView) drawQuestionReply(f *flex.Frame, e TranscriptEntry) {
	t := v.Theme
	f.Column().FillWidth().Gap(4).Children(func() {
		f.Text(e.Data.Text).FontSize(t.Fonts.Body).LineHeight(t.Fonts.BodyLine).Color(t.LabelPrimary)
		if e.Data.Outcome != "" {
			f.Text(e.Data.Outcome).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		}
	})
}

// drawWorkflowRun 画 §5.20 工作流运行（外部包节点：只摆位，不定尺寸）。
func (v *OfficialTranscriptView) drawWorkflowRun(f *flex.Frame, e TranscriptEntry) {
	t := v.Theme
	card := f.Column().FillWidth().Border(0.5, t.BorderL1).Radius(t.Radii.LG)
	card.Children(func() {
		f.Row().FillWidth().Items(flex.Center).Gap(6).PadSides(8, 12, 8, 12).Children(func() {
			name := e.Data.RunName
			if name == "" {
				name = "工作流"
			}
			f.Text(name).FontSize(t.Fonts.Secondary).Color(t.LabelPrimary)
			if e.Data.StopReason != "" {
				f.Text(e.Data.StopReason).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
			}
		})
	})
}

// drawUnknown 画 §5.17 未知 surface 兜底：原样 JSON，**不编摘要**。
func (v *OfficialTranscriptView) drawUnknown(f *flex.Frame, e TranscriptEntry) {
	t := v.Theme
	raw := e.Data.Raw
	if raw == "" {
		raw = "{}"
	}
	f.Column().FillWidth().PadY(TurnErrorPadY).Children(func() {
		f.Text(raw).FontSize(t.Fonts.Code).LineHeight(t.Fonts.CodeLine).Mono().Color(t.LabelSecondary)
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
	Card flex.Box
	// Header 是头部（`padding:10px 16px`）。
	Header flex.Box
	// Body 是体（`padding:12px 16px 0;gap:6px`）。
	Body flex.Box
	// Options 是选项按钮（**顺序 = 宿主给的顺序**，不重排、不造 id）。
	Options []ApprovalOptionBox
	// Rendered 报告这一帧是否真的画了（零值 Approval 不画）。
	Rendered bool
}

// ApprovalOptionBox 是一个选项按钮的渲染结果。
type ApprovalOptionBox struct {
	// Box 是按钮盒子。
	Box flex.Box
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
func (v *OfficialTranscriptView) DrawApprovalCard(f *flex.Frame, env *pluginapi.Env, approval pluginapi.Approval) ApprovalBoxes {
	t := v.Theme
	var boxes ApprovalBoxes
	card := f.Column().FillWidth().Radius(t.Radii.XL).Border(0.5, t.BorderL1)
	boxes.Card = card
	boxes.Rendered = true
	card.Children(func() {
		header := f.Row().FillWidth().Items(flex.Center).Gap(ApprovalBodyGap).
			PadSides(ApprovalHeaderPadY, ApprovalHeaderPadX, ApprovalHeaderPadY, ApprovalHeaderPadX)
		boxes.Header = header
		header.Children(func() {
			title := approval.Title
			if title == "" {
				title = "需要审批"
			}
			f.Text(title).FontSize(t.Fonts.Body).Color(t.LabelPrimary).Ellipsis()
		})
		body := f.Column().FillWidth().Gap(ApprovalBodyGap).
			PadSides(ApprovalBodyPadTop, ApprovalBodyPadX, 0, ApprovalBodyPadX)
		boxes.Body = body
		body.Children(func() {
			if approval.Args != "" || approval.Cmd != "" {
				text := approval.Args
				if text == "" {
					text = approval.Cmd
				}
				f.Text(text).FontSize(t.Fonts.Code).LineHeight(t.Fonts.CodeLine).Mono().Color(t.LabelSecondary)
			}
			if approval.Detail != "" {
				f.Text(approval.Detail).FontSize(t.Fonts.Secondary).Color(t.LabelSecondary)
			}
			row := f.Row().Gap(ActionsRowGap).PadSides(0, 0, ApprovalBodyPadTop, 0)
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
					button := f.Button(v.optionID(env, label), label).Small()
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
func (v *OfficialTranscriptView) optionID(env *pluginapi.Env, label string) ui.ID {
	if env == nil || env.Tree == nil {
		return ui.ID{}
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
func (p *Plugin) OfficialTranscript() *OfficialTranscriptView {
	p.officialTranscript = NewOfficialTranscriptView(p.theme, p.prims)
	return p.officialTranscript
}
