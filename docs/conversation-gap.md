# 消息区差距审计：官方 DSH vs 原生 UI

> 审计冻结基线：UI `f007fadd18964a1f2dad0aefa9a939855ee23bcc`（短名 `f007fad`）。  
> 取证时间：2026-11-04。只读源码审计；没有依据截图推断行为，没有改产品代码、没有 commit/push、没有启动或杀进程。  
> 审计开始时工作树已有其他 owner 的修改；本文“当前”指上述冻结对照，行号对应取证时工作树。本文件是本 owner 唯一新增文件。  
> 并发说明：复核时其他 owner 已把 HEAD 推进到 `9a155c7`，并在 `plugins/dsconversation` 加入流式 Markdown 改动与测试；这些后续变化**未纳入本矩阵**，需另行验收，不回写本 base 的结论。

## 0. 结论先行

“GUI 离 DSH 很远”不是单纯的色号、圆角或插件数量差距，而是三层问题叠加：

1. **内容正确性仍有 P0 风险**：`session.load` 的 v4 `seq` 与 WS 的 Transport journal `seq` 被 UI 当成同一种消息身份；工具结果也没有按 `callId` 配对。首先要防止旧消息被新帧覆盖、跨轮工具串结果。
2. **官方是“逐 Turn 的可操作工作台”，当前主要是“扁平文本 + 通用卡片”**：流式 Markdown、完整工具生命周期、Turn 折叠/页脚/导航、错误终局、代码块工具栏、消息时间和反馈都不完整。
3. **有些差距纯 UI 可关闭，有些确实缺契约真值**：宿主已经提供 `time`、原始 `turn/error`、原始工具事件，UI 适配层却丢掉了；附件、消息反馈、未加载 Turn 大纲、向前分页和完整工具树投影则需要扩当前 Transport 契约。不能把所有缺口都归为“皮肤没做”，也不能都推给宿主。

### 状态口径

- **缺失**：当前没有对应的实际行为。
- **部分**：已有真实实现，但范围、交互或生命周期明显小于官方；每项写出已覆盖部分。
- **已覆盖**：在本行限定范围内，当前源码已有对应行为；不表示整个官方包完成。
- **契约缺真值**：当前 UI 快照/Transport 投影缺少忠实实现所需的身份、状态或内容；另注明是 **UI 适配丢字段**，还是 **需要宿主新契约**。

### 责任边界缩写

- **纯 UI**：布局、Markdown、折叠、滚动、按钮和本地状态，不要求宿主新增能力。
- **UI 适配**：宿主原始帧/投影已有事实，需改 `internal/apiclient`、事件归约或 `pluginapi` 消费，不要求宿主新增方法。
- **宿主契约**：当前 Transport 没有所需字段或方法，必须 snapshot-first 跨仓冻结后实现。

## 1. 按用户影响排序的差距矩阵

> Markdown 各子项共同受两条当前限制影响：`plugins/dsconversation/conversation.go:30-37` 仅最近 30 条走 Markdown；`:260-272` 的流式正文始终是 `f.Wrap` 纯文本。下文不再把这两条重复写成独立缺口。

| # | 项目与用户影响 | 官方源码位置 | 当前源码位置 | 状态 | 具体差距与责任边界 |
|---:|---|---|---|---|---|
| 1 | **消息身份、流式落条与历史归并**：错了会覆盖旧内容、重复或错排，是内容正确性门 | `tmp/official/dsh-client-ui-conversation/client.js:2575-2595` `replaceWindow`；`:2602-2621` `append` 以同一事件域 `seq` 去重；`:2631-2664` `settleAssistant` 保留稳定 key | `internal/apiclient/mux.go:492-504` 把 WS `ev.Seq` 赋给 `Stream.seq`；`:546-556` 用它生成 `Message`；`:588-619` `upsertMsg` 按 `(Seq,Role)` 覆盖。宿主 `../DSH-GO/cmd/dshd/transport_source.go:223-255` 的 load 投影却使用 v4 `ev.Seq`，`../DSH-GO/internal/transport/journal.go:62-67` 的 WS 使用另一套 journal seq | **契约缺真值** | 两套 seq 在 `../DSH-GO/docs/transport-contract.md:81` 明确不同，当前 UI 却共用一个 `Message.Seq`。旧会话加载后，新 journal 小序号可能与旧 v4 消息同号。需要宿主提供稳定 `sourceSeq/eventId`（journalSeq 仅用于连续性），UI 再分离“水位身份”和“内容身份”；不能继续靠同号猜测。 |
| 2 | **流式正文与流式 Markdown**：生成过程是用户最常看的界面 | `tmp/official/dsh-client-ui-chat/client.js:6290-6311` `AssistantMarkdown` 对 settled/streaming 正文统一调用 `MarkdownText`；`tmp/official-primitives-bundle.js:173` `vU.render` 缓存 frozen blocks、只处理变化尾部 | `plugins/dsconversation/conversation.go:260-272` `drawStreaming` 用 `f.Wrap`；`:333-345` 仅定稿 assistant 调 `layoutMarkdown`；`internal/apiclient/mux.go:351-355,509-514` 确有 text delta 累加 | **部分** | delta 接收和逐字显示已覆盖；但生成中标题/列表/表格/代码围栏都不是 Markdown，结束后还会换排版。流式统一 Markdown、稳定未完成围栏和增量排版属于**纯 UI**。 |
| 3 | **后台连续性与本地 echo**：切会话、发送失败时直接影响是否敢连续工作 | `tmp/official/dsh-client-ui-conversation/client.js:3962-3988` `ConversationBinding.accept` 持续消费 append/settle，与 active target 分离；`tmp/official/dsh-client-ui-chat/client.js:2722-2735` `observedInputs` 以 `rpcId` 抑制重复 echo；`:5428-5452` pending echo | `cmd/app/main.go:851-869` 后台帧只记水位、正文不归约进缓存；`:948-960` 发送先追加 `Seq:-1`，失败只改状态且草稿已清；`plugins/dsconversation/conversation.go:69-78` 思考/工具展开状态只按消息 `seq`，没有 session/callId 命名空间 | **部分** | 每会话 WS 与 seq 水位已覆盖；但后台内容要切回后重放/refresh，展开状态还可能跨会话复用。失败发送缺可靠草稿恢复与 rpcId 对账。后台缓存、折叠命名空间是纯 UI/UI 适配；可靠 echo 身份需与第 1 项一起定契约。 |
| 4 | **工具调用状态与结果配对**：运行中、成功、失败、停止应一眼可分，结果必须属于正确调用 | `tmp/official/dsh-client-ui-tool/client.js:311-329` `rowModel` 映射 `preparing/running/error/stopped/ok`；`:1579-1585` `stateStatus`；`:1594-1624` `ToolRow` | `plugins/dsconversation/conversation.go:461-522` `drawTool` 只有单行工具卡、展开和 error/ok badge；`internal/apiclient/mux.go:515-525` 新结果总是写到 `s.tools[len-1]`；`:484-489` `Reset` 未清 `tools` | **部分** | 基本调用行与结果摘要已覆盖；没有 preparing、running shimmer、stopped、退出码/信号等完整状态。原始结果含 `toolCallId`（`../DSH-GO/cmd/dshd/serve_tools.go:125-146`），UI `packet.Message` 却没接，属于 **UI 适配 + 纯 UI**；跨轮清理也是 UI 自身问题。 |
| 5 | **工具树、子调用与 Inspect**：复杂任务无法追溯根调用、PTC 子调用和原始输入输出 | `tmp/official/dsh-client-ui-tool/client.js:1980-2004` `ToolCallBranch` 按 `child.callId` 递归；`:2013-2025` `ToolCallTree`；`:1797-1805` Inspect；`tmp/asar-css/141__ToolCallTree.module.css.mjs.css:1` `.pDft-q_subCalls` | `internal/apiclient/client.go:27-40` `Message` 无 `callId/parentCallId/subCalls/phase`；`plugins/dsrightbar/rightbar.go:147-155` 只收集扁平 `role=tool`；`plugins/dsconversation/conversation.go:524-589` 只有通用 ViewSpec 卡 | **契约缺真值** | 实时适配可保留原始 `callId/parentCallId`，但历史 `session.load` 投影没有完整树、阶段、Inspect 载荷；完整历史等价需要**宿主契约**，不是把右栏改成缩进就能解决。 |
| 6 | **专用工具结果：terminal/diff/read/search/web/todo/question/image** | `tmp/official/dsh-client-ui-tool/client.js:1594-1612` `ToolRow` 组合专用 card；`:1750-1775` terminal/diff/read/image/search/web/details；`:2723` `ReadRow`、`:2944` `SearchRow`、`:4505` `TodoRow`、`:4557` `WebRow` | `plugins/dsconversation/conversation.go:524-589` `drawToolView` 支持 `card/table/plain` 和默认纯文本；`:461-522` 通用折叠卡 | **部分** | 当前 H3 `ViewSpec` 三种声明与无声明回退已覆盖；官方按工具语义展示终端对齐、退出码、diff 统计、文件行号、搜索结果、todo 历史、问题回答和图片。可先从现有 Args/Summary 做一部分纯 UI；完整结构化结果需要契约。 |
| 7 | **错误终局与自动重试行**：模型失败、欠费、鉴权、重试倒计时不能只消失成“就绪” | `tmp/official/dsh-client-ui-chat/client.js:5278-5283` `failureMessage`；`:5285-5330` `ModelRetryItem`；`:5361-5415` `TurnErrorItem`；`tmp/asar-css/015__MessageItem.module.css.mjs.css:1` `.LOmM4q_turnErrorRow/.retryRow` | `internal/apiclient/mux.go:426-434` `Event.End` 把 `turn/error` 只当结束；`cmd/app/main.go:882-885` 结束后清流、清 busy；`plugins/dsconversation/conversation.go:776-787` 没有 error/retry 消息类型 | **缺失** | 宿主 `../DSH-GO/cmd/dshd/serve_journal.go:40-53` 已发送 `{turnId,error}`，属于 **UI 适配未消费 + 消息区未画**。自动重试次数、deadline、上次错误需保留原始 retry 事件或补投影。 |
| 8 | **取消与 stopped/interrupted 终态** | `tmp/official/dsh-client-ui-chat/client.js:6332-6335` 部分回答后的 `message.stopped`；`:6658-6663` `TurnProcessNodeView` 对 aborted/interrupted 显示已停止；`:5280` 中断错误映射 | `cmd/app/main.go:688-709` 真调 `session.cancel`，成功后只显示状态“已取消”并清 busy；`plugins/dscomposer/composer.go:281-305,362-388` busy/stream 时发送键变停止键并调用 Cancel | **部分** | 真取消入口和失败提示已覆盖；没有消息内 stopped 标记、部分回答终态和 Turn 摘要。审批 pending cancel 的宿主整链在 `../DSH-GO/docs/transport-contract.md:102` 仍明确未闭环，不能仅凭本地“已取消”签完成。 |
| 9 | **手动重试、失败发送恢复、分支重试** | `tmp/official/dsh-client-ui-chat/client.js:5285-5330` 自动重试状态；`tmp/official/dsh-client-ui-conversation/client.js:15593-15606` `settleDetachedFailure` 恢复失败草稿；`tmp/official/dsh-client-ui-chat/client.js:6996-7039` Turn 尾部按 seq `forkAt` | `cmd/app/main.go:917-935` `retryLast` 只是再次发送最后一条 user 文本；`:948-960` 清空草稿且失败不恢复；`plugins/dsconversation/conversation.go:369-372` 仅最后一条 assistant 有重试按钮 | **部分** | 基本“重发最后提示”已覆盖，但不是官方 Turn/分支语义，也没有自动重试状态。草稿恢复与普通重试主要是纯 UI/UI 适配；指定 seq fork 需要契约，当前快照 `testdata/api.snapshot.json:172-179` 明确 `fromSeq` 未实现。 |
| 10 | **用户/助手角色、气泡与消息 chrome** | `tmp/official/dsh-client-ui-chat/client.js:5392-5438` `UserStyleBubble`；`:6290-6418` `AssistantMarkdown` 无额外“助手”标题；`tmp/asar-css/015__MessageItem.module.css.mjs.css:1` `.LOmM4q_userStack/.bubble` | `plugins/dsconversation/conversation.go:318-345` 用户右气泡、助手正文；`:320-323` 额外“用户”标签、`:336` 额外“助手”标签；`plugins/dscore/primitives.go:71-76` `Bubble` 四边 `Pad(10)` | **部分** | 右对齐、XL 圆角和气泡底色已覆盖。官方 userStack 上限 `min(748*.702,82%)`、padding `10px 16px`；当前 `:286-292` 只按 82% 上限且四边 10，气泡更宽、水平 padding 少 12px。文字角色标签与按钮式动作也使观感不同，属于**纯 UI**。 |
| 11 | **system/context/compaction/trigger 等角色**：不能把内部上下文当助手回答 | `tmp/official/dsh-client-ui-chat/client.js:9064-9085` `shouldHideChatNode` 隐藏 system/context-injection/permission；`:6514-6529` compaction 卡；`:6996-7043` Turn tail；`:7128-7166` 非人工 trigger 通知 | `plugins/dsconversation/conversation.go:776-787` `roleKind` 把未知角色（含 system）都降级为 assistant；`internal/apiclient/client.go:27-40` 无 context 类型、压缩和 trigger 结构 | **契约缺真值** | 基本 user/assistant/tool/thinking 已有；其他语义不能靠文本猜。当前 host load 投影也只保留 user/assistant/tool（`../DSH-GO/cmd/dshd/transport_source.go:216-256`），需保留原始角色/块类型与 compaction/trigger 数据。 |
| 12 | **思考折叠、摘要与流式预览** | `tmp/official/dsh-client-ui-chat/client.js:6153-6176` 取首行/最近完成段落；`:6187-6236` `ReasoningRow` 的 preview、running、Thought for Ns；`tmp/asar-css/020__ReasoningRow.module.css.mjs.css:1` `.VJlF5q_summary/.thinkBody` | `plugins/dsconversation/conversation.go:376-434` `drawThinking` 有默认折叠、展开全文、运行 spinner；`:424-430` 摘要取原始文本前 120 rune，标题显示“N 字” | **部分** | 折叠与流式思考已覆盖；摘要不是最近完成段落，未清理 `<signature>`，没有“想了 N 秒”、sticky header 和官方 preview mask。大部分为纯 UI，准确时长需第 26 项时间真值。 |
| 13 | **长消息与虚拟化**：长会话不能因全量排版卡死，也不能截断 | `tmp/official/dsh-client-ui-chat/client.js:4144-4171` `ChatGroupSeat` 使用 virtualizer 和稳定 group key；`tmp/official/dsh-client-ui-trajectory/client.js:4912-4933` Trajectory 可见记录窗口 | `plugins/dsconversation/conversation.go:219-249` 视口外只放固定高度 Slot；`:670-701` `estimateMsgHeight` 估算；`:37` 仅 30 条 Markdown | **部分** | 虚拟化和 LOD 已覆盖，且有明确性能动机；但估高不是官方按 group 测量/锚点模型，旧消息退成原始 Markdown 文本。可见时精确测量、保持内容身份和滚动补偿是纯 UI。 |
| 14 | **滚动锚点、阅读所有权与回到底部** | `tmp/official/dsh-client-ui-chat/client.js:2415-2436` `ChatViewport.write` 记录 anchorKey/top；`:2497-2550` `useChatScroll` 区分 own input、跟随和导航；`:2657-2675` `returnToBottom`；`tmp/asar-css/013__ChatView.module.css.mjs.css:1` `.V4vHZW_toBottom` | `plugins/dsconversation/conversation.go:185-188` `ScrollArea(...).Follow(true)`；`cmd/app/main.go:451-490` 每会话保存/恢复 Offset | **部分** | Viva `Follow(true)` 本身支持上滚暂停跟随、回底恢复，且当前有每会话 Offset，不能误报“总是强制拉底”。缺显式回底按钮、prepend 锚点补偿、流式/折叠时语义锚点和未读状态；这些主要是纯 UI。 |
| 15 | **向前翻历史、Turn 序号轨道与未加载 Turn 跳转** | `tmp/official/dsh-client-ui-chat/client.js:231-258` `TurnMark`；`:260-484` `TurnNavigatorRail`；`:619-697` `HistoryNavigator` 以 preserve/loadThrough 保持位置；`tmp/asar-css/012__TurnNavigator.module.css.mjs.css:1` `.SNx2jG_*` | `internal/apiclient/client.go:303-313` 只有 load tail；`internal/apiclient/history.go:35-55` history 是断号后的 `sinceSeq`，不是 `beforeSeq/loadOlder`；`pluginapi/plugin.go:199-204` `Truncated` 已传递但消息区没有分页入口 | **契约缺真值** | 当前无法打开 tail 之前的历史，也没有 Turn rail。已加载 Turn 的分组可从原始事件做 UI 适配；未加载大纲、`loadOlder/loadThrough` 和 prepend 锚点需要**宿主契约**。 |
| 16 | **Markdown 标题** | `tmp/official/dsh-client-ui-chat/client.js:6305-6311` `MarkdownText`；`tmp/official-primitives-bundle.js:168` `kn` 的 heading 分支 | `plugins/dsconversation/conversation.go:333-345,635-655` Viva `markdown.Document/MarkdownView`；`github.com/vivaui/viva@v0.2.3/markdown/markdown.go:28-29` 支持 1–6 级 Heading | **部分** | 定稿最近 30 条的基础标题已覆盖；流式和更早消息不覆盖。内容字号自适应、与官方段距一致仍需视觉令牌对拍，属于纯 UI。 |
| 17 | **Markdown 列表（有序/无序/嵌套/任务项）** | `tmp/official-primitives-bundle.js:168-172` `kn` 的 list/listItem、`sU`、`fw`，任务项使用 `.task-list-item` | `plugins/dsconversation/conversation.go:333-345`；`github.com/vivaui/viva@v0.2.3/markdown/markdown.go:30-31,81-89` Item/Level/Number | **部分** | 基础有序/无序和嵌套列表解析已覆盖于定稿最近 30 条；流式、旧消息和官方任务列表样式不完整，属于纯 UI。 |
| 18 | **Markdown 表格与宽表溢出** | `tmp/official-primitives-bundle.js:172` `aU` 输出 table/thead/tbody，≥4 列使用 `.md-table-wide`；`tmp/asar-css/021__AssistantMarkdown.module.css.mjs.css:1` `.xMeOpW_body .md-table-wide` 扩展可滚动区域 | `plugins/dsconversation/conversation.go:333-345`；`github.com/vivaui/viva@v0.2.3/components/markdown_test.go:564` `TestMarkdownTableFitsItsWidthAndWrapsItsCells` | **部分** | 基础表格、对齐和单元格折行已覆盖于定稿最近 30 条；宽表不是官方横向扩展/滚动模式，流式与旧消息无表格，属于纯 UI。 |
| 19 | **代码块：高亮、语言 banner、复制、折行/横滚** | `tmp/official/dsh-client-ui-chat/client.js:4256-4265` `markdownLabels` 提供 copy/wrap/unwrap；`tmp/official-primitives-bundle.js:140` `d4` 的 `[data-code-block-banner]`；`:165` `iw` 的 `.md-code-block`、`[data-code-block-content]` 和高亮 | `plugins/dsconversation/conversation.go:589-626` 配置 code fill、`CodeInset` 与 `mdlite.Highlight`；`internal/mdlite/highlight.go:14-75` 仅有限语言关键词；`internal/mdlite/overflow.go:22-65` 有横滚容器但生产消息区未调用 | **部分** | 围栏块、等宽、有限高亮和不丢内容的折行已覆盖；没有语言/title banner、块级复制、wrap toggle，长行折行会破坏列对齐。`docs/u1-codeblock.md:47-65` 也确认当前无横滚。需要自有代码块绘制路径，纯 UI 可做。 |
| 20 | **行内码与行内文件 mention** | `tmp/official-primitives-bundle.js:168` `inlineCode` 分支；文件 mention 生成带图标的可点按钮；`tmp/asar-css/032__ReferenceChip.module.css.mjs.css:1` `.NOA9Ua_chip` | `plugins/dsconversation/conversation.go:592-596` `InlineCode` 底色；`github.com/vivaui/viva@v0.2.3/components/markdown.go:1433-1437,1500-1501` 行内码背景和 mono | **部分** | 最近定稿消息的行内码底色/等宽已覆盖；没有行内文件解析与点击，流式/旧消息无样式。视觉与文本解析是纯 UI，文件预览动作见第 21/25 项。 |
| 21 | **HTTP(S)/文件链接与 `#L24` 定位** | `tmp/official-primitives-bundle.js:165` `ow` 解析 `#L…`；`:172` `mw/cU/lU` 分流文件与外链；`tmp/official/dsh-client-ui-chat/client.js:2794-2809,2909-2911` `requestOpenFile` 与 `MarkdownDelegateProvider` | `plugins/dsconversation/conversation.go:678-684` `MarkdownPaint` 只给 Look/X/Y/Clip，没有 `Links` scope；`pluginapi/plugin.go:339-415` 没有 OpenExternal/OpenFile action | **部分** | Viva 会把链接画成颜色，但 `github.com/vivaui/viva@v0.2.3/components/markdown.go:135-138,1583-1590` 明确 `Links` 为零就没有 hit region。因此当前没有可点链接。HTTP 链接是纯 UI + OS action；文件内容预览可复用宿主 `fs.read`，会话引用准备另需契约。 |
| 22 | **复制与消息动作行** | `tmp/official/dsh-client-ui-chat/client.js:5175-5235` `MessageIconActions` 的复制状态、时间、分支和扩展动作；`:6996-7039` `TurnTailNodeView`；`tmp/asar-css/018__MessageIconActions.module.css.mjs.css:1` `._1KfHza_*` | `plugins/dsconversation/conversation.go:358-373` 每条消息文字“复制”按钮、末条 assistant“重试”；`:790-824` Ctrl+Shift+C 复制最后 assistant；`cmd/app/main.go:1499-1505` 真写剪贴板并提示 | **部分** | 原文复制和成功/失败状态已覆盖；没有选中区复制、官方 28px 图标动作行、悬停/焦点显示、复制后图标反馈、Turn 尾部动作和分支。代码块复制见第 19 项。 |
| 23 | **Like/Dislike 反馈与消息分支** | `tmp/official/dsh-client-ui-message-feedback/client.js:168-262` `MessageFeedbackActions`；`:318-540` `MessageFeedbackController` 按 messageId/version 对账；`tmp/official/dsh-client-ui-chat/client.js:7020-7035` `messageId` 与 `forkAt(data.seq)` | `internal/apiclient/client.go:27-40` 无 messageId/rating/version；`pluginapi/plugin.go:339-415` 无反馈/分支 Action；`cmd/app/main.go:1495-1522` 动作表也无对应项 | **契约缺真值** | 不能只画两个本地点赞图标冒充官方持久反馈。需要 messageId、list/put/retract、version-conflict、失败保留草稿，以及指定 seq 的 fork。当前 Transport 没有这些方法/字段。 |
| 24 | **附件、图片、文件卡与灯箱** | `tmp/official/dsh-client-ui-chat/client.js:7330-7380` `UserContent` 保持附件顺序并渲染文件卡/图片；`tmp/official/dsh-client-ui-attachment/client.js:754-854` `MessageImage/ImageGallery` 加载、重试、单图/多图尺寸和灯箱；`tmp/asar-css/011__MessageImage.module.css.mjs.css:1` | `internal/apiclient/client.go:27-40` 只有扁平 Text；`plugins/dsconversation/conversation.go:318-345` 用户内容只画 Text；`internal/apiclient/mux.go:286-312` packet 块不保留 attachment | **契约缺真值** | 当前消息区没有附件结构可画。`testdata/api.snapshot.json:139-147` 明确 prompt attachments 未实现；需要内容块、附件 id/名称/大小/尺寸、授权读取和失败重试，不能用本地假图补齐。 |
| 25 | **`@file/@folder/@session` 引用 chip、摘要与点击** | `tmp/official/dsh-client-ui-reference/client.js:17-64` `FileReference/FolderReference`；`:68-87` 原文偏移解析；`tmp/official/dsh-client-ui-chat/client.js:7377-7380` `referenceSummary`；`tmp/asar-css/032__ReferenceChip.module.css.mjs.css:1` | `plugins/dsconversation/conversation.go:318-329` 用户文本原样 `Wrap`；`internal/apiclient/client.go:27-40` 无引用节点/标签；`pluginapi/plugin.go:339-415` 无 OpenFile | **契约缺真值** | 原始 `@...` 文本若能进入 Text 会保留，但没有原子 chip、引用摘要、文件点击或 session 身份。文件 chip 外观/解析可纯 UI；可靠身份、会话快照准备与授权打开需要宿主契约。 |
| 26 | **消息时间与时长** | `tmp/official/dsh-client-ui-chat/client.js:5105-5125` `formatMessageClock`；`:5175-5214` `MessageIconActions`；`tmp/official/dsh-client-ui-trajectory/client.js:4552-4565` `RecordTiming` | `internal/apiclient/client.go:27-40` `Message` 无 `Time`；`internal/apiclient/mux.go:30-42` `Event` 无 `Time`，尽管 `serverFrame` 在 `:97-102` 已解出；`plugins/dsconversation/conversation.go:358-373` 动作行无时间 | **契约缺真值** | 这是 **UI 适配丢字段**，不是宿主没给：`../DSH-GO/internal/transport/api.go:49-55` load 有 Time，`frame.go:80-84` WS 有 Time。补齐后未知时间应隐藏，不能编 0；TTFT/吞吐另需真实 token 时间戳。 |
| 27 | **Turn 序号、Turn 时长与逐轮 usage** | `tmp/official/dsh-client-ui-chat/client.js:6652-6691` `TurnProcessNodeView`；`:6880-6905` `TurnUsagePanel`；`:6996-7039` `TurnTailNodeView`；`tmp/official/dsh-client-ui-trajectory/client.js:4956-4965` 请求编号索引 | `cmd/app/main.go:957-965` Prompt 返回 TurnID 只写状态，没有归入消息；`internal/apiclient/client.go:68-79` Usage 只有基础 token/rate；`plugins/dsconversation/conversation.go` 无 Turn 分组/页脚 | **契约缺真值** | 已加载 Turn 可从原始 turn/start/end 做 UI 适配；完整未加载大纲、请求编号、TTFT/decode、route 和精确分桶需要宿主投影。不得用消息数组下标冒充官方 Turn 序号。 |
| 28 | **Trajectory：账本、搜索、检查器与时间概览** | `tmp/official/dsh-client-ui-trajectory/client.js:4912-5044` `TrajectoryTable` 选择/检查器/历史锚点；`:6439-6580` `TrajectoryTimeline` 时间域、缩放/拖选；`:7785-7871` `TrajectoryView` 搜索、折叠、分页、尾部跟随 | `plugins/dsrightbar/rightbar.go:21-26` 最近 80 次工具；`:59-110` 单列可滚时间线；`:113-145` 每项只有名称/一行摘要/成败点 | **部分** | “最近工具时间线”已覆盖，空态、成功/失败点和截断提示真实存在；但不是官方按 Turn/Step 的事件账本，没有用户/助手/压缩记录、搜索、检查器、输入输出、时间条、TTFT 和选择联动。当前窗口内分组可纯 UI；完整历史与计时依赖第 5/15/26/27 项契约。 |

## 2. 为什么用户会觉得“GUI 离 DSH 很远”：前 5 个差异

| 排名 | 用户直接感受 | 纯 UI 现在能做 | UI 适配可做（宿主不增方法） | 必须宿主契约 |
|---:|---|---|---|---|
| 1 | **内容可信度不像正式产品**：新流可能按错误 seq 覆盖旧消息；工具结果写最近一条而非按 callId；错误终局被吞 | 修复 `Stream.Reset`、折叠状态命名空间、不要再用文本/最近项猜身份 | 保留 Time、turn/error、toolCallId，分离 journal 水位与内容归约 | **稳定 sourceSeq/eventId/messageId/callId 的跨 load+WS 同一身份**。这是下一批最优先项，先于继续美化。 |
| 2 | **生成中与生成后不是同一种阅读体验**：流式纯文本，旧消息 30 条以后又变纯文本；代码块无 banner/复制/wrap，链接不可点；思考摘要与官方不同 | **大部分可做**：流式 Markdown、可见消息 LOD、代码块工具栏、链接 hit、思考摘要、气泡几何和动作图标 | 只需把真实文本/时间送入视图，无新方法 | 图片/附件型 Markdown、会话引用准备和授权文件预览的完整能力需要契约。 |
| 3 | **工具与 Trajectory 只是“摘要”，不是工作过程**：无 preparing/running/stopped 全态、专用 terminal/diff/read/search 卡、子调用树、Inspect 和时间概览 | 通用行状态、折叠、基础 terminal/diff/read 展示、右栏当前窗口分组与搜索 | 从原始事件保留 callId/parentCallId/phase/error/result，即可显著提升实时工具卡 | 完整历史工具树、结构化专用结果、Inspect 原始载荷、请求编号和完整计时投影。 |
| 4 | **长会话不像可导航的 DSH**：只能看 tail，不能 loadOlder；无 Turn rail、Turn 页脚/时长、显式回底和可靠 prepend 锚点；切走时不积累后台正文缓存 | 回底按钮、可见测量、滚动补偿、后台缓存、已加载 Turn 折叠和页脚 | 从原始 turn/start/end 保留 TurnID/time/usage，恢复每会话阅读位置 | **beforeSeq/loadOlder/loadThrough、turnOutline、未加载 Turn 预览**，以及完整 TTFT/decode/requestNumber。 |
| 5 | **消息生命周期与动作面太薄**：错误没有持久行，取消只改状态，重试只是重发最后文本，复制是文字按钮，无时间/反馈/分支/附件/引用 | 错误行、stopped 标记、失败草稿恢复、复制动作行、选中区复制、真实取消后的第二轮 | 消费 turn/error/time/retry 原始状态，保留 rpcId/turnId | **反馈 list/put/retract、指定 seq fork、附件内容块与授权读取、session reference**。这些不能用本地假状态宣称完成。 |

## 3. 契约真值清单：哪些能直接接，哪些必须新增

### 3.1 宿主已有，UI 适配层应保留

| 真值 | 宿主证据 | UI 丢失位置 | 建议消费 |
|---|---|---|---|
| 消息/帧 `time` | `../DSH-GO/internal/transport/api.go:49-55`；`frame.go:80-84` | `internal/apiclient/client.go:27-40` 无 Message.Time；`mux.go:30-42` Event 无 Time | 消息时钟、Turn 时长；缺值隐藏，非正间隔不算速率 |
| `turn/error{turnId,error}` | `../DSH-GO/cmd/dshd/serve_journal.go:40-53` | `internal/apiclient/mux.go:426-434` 仅当 End | 持久错误行、重试入口、部分回答 stopped 状态 |
| `toolCallId` 与原始工具结果 | `../DSH-GO/cmd/dshd/serve_tools.go:125-146`；`transport_source.go:238-247` | `internal/apiclient/mux.go:286-312` packet 未读；`:523-525` 写最近一条 | 按 callId 配对，处理交错/乱序结果；工具折叠状态按 session+callId |
| 原始 Turn/Assistant 事件 | `../DSH-GO/docs/transport-contract.md:78-84` | `cmd/app/main.go:874-888` 只归约为 thinking/text/tool，后台帧还在 `:860-869` 丢弃内容 | 已加载 Turn 分组、后台缓存、终态和逐轮 usage；不要整页刷新代替增量 |

### 3.2 当前 Transport 确实缺，需 snapshot-first 新契约

| 能力 | 最小真值建议 | 不能用什么冒充 |
|---|---|---|
| 跨 load+WS 内容身份 | `journalSeq` 只负责连续性；另给 `sourceSeq` 或稳定 `eventId/messageId/attemptId`，并在 load/history/WS 同义 | 不用 journalSeq 覆盖 v4 Seq，不用数组下标或相同文本猜身份 |
| 完整工具树与检查 | `callId,parentCallId,phase,argsRaw,result,isError,error,startedAt,endedAt,subCalls`；Inspect 原始载荷 | 不用 `Tool+Args` 当唯一身份，不把最近结果塞给最近调用 |
| 向前历史与 Turn 大纲 | `loadOlder(beforeSeq,limit)` 返回 prepend/hasMore；`loadThrough(turn/start seq)`；`turnOutline{turn,seq,promptPreview,responsePreview}` | 不用断号 `session.history(sinceSeq)` 冒充向前分页，不编造未加载回答预览 |
| 本地提交对账 | prompt 接收并回传 `rpcId`，durable user event 携带同一 rpcId；失败返回可恢复草稿/附件状态 | 不用 `Seq:-1` 永久占位，不在失败时同时丢草稿和留下假成功气泡 |
| 附件与引用 | 有序 content blocks；attachment id/name/bytes/dimensions；session 授权 load；file/session reference 的稳定身份 | 不用假缩略图、0 bytes、可见标题反推 session id |
| 消息反馈与分支 | messageId；`messageFeedback.list/put/retract` 与 version；`session.fork` 收 `fromSeq` | 不做只存在内存的 👍/👎 冒充已记录；不fork整个会话冒充指定 Turn 分支 |
| Trajectory 计时/统计 | start/end/token timestamps、TTFT/decode、requestNumber、usage routes 与错误详情；运行中未知保持未知 | 不用当前墙钟虚构耗时，不把 message 数当 Turn 数，不编 0 tok/s |

## 4. 下一批可独立实施的工作包建议

> 这只是 owner/路径/验收建议，**没有修改任务表**。共享 `pluginapi`、`cmd/app`、apiclient、快照按仓内规则串行；下面用依赖顺序避免同文件并行写。

### WP-A · P0：统一事件身份与真值适配（最先，契约单 owner）

- **Owner 类型**：跨仓 Transport 契约 owner + Go apiclient owner；契约冻结前不改两侧实现。
- **Owned paths**：`../DSH-GO/docs/transport-contract.md`、`../DSH-GO/internal/transport/**`、`../DSH-GO/internal/transport/testdata/api.snapshot.json`、`../DSH-GO/cmd/dshd/{serve_journal.go,serve_tools.go,transport_source.go}`、`internal/apiclient/**`、`testdata/api.snapshot.json` 及对应 golden/tests。
- **范围**：分离 journalSeq 与 sourceSeq/eventId；接 Time、turn/error、callId/parentCallId；工具结果按 id 配对；Reset 清轮次状态；负载保留 Turn/attempt 信息。附件/反馈等后续方法可在同一契约冻结中登记，但不要求本包全部实现。
- **验收**：
  1. 夹具先 load v4 seq `101/102`，再收 WS journal seq `1/2`，必须追加正确新消息，**不得覆盖 101/102**；重放同一 source event 不重复。
  2. 两个 Turn 有相同文本、两个工具同名同参，仍各自独立；交错 result 按 callId 落到正确调用；Reset 后无前一轮工具。
  3. `turn/error` 保留 turnId/error/time；Time 缺失时 UI 数据为 unknown 而非 0。
  4. 两仓 snapshot/golden 同步，`go test ./internal/apiclient/...` 与宿主 transport/dshd 相关测试通过；至少一条真实 apiclient 对契约夹具重放。

### WP-B · 原生 Markdown/代码块渲染组件（可与 WP-A 并行）

- **Owner 类型**：原生 Go 渲染 owner。
- **Owned paths**：`internal/mdlite/**`（可新增独立组件与测试）；不改 `cmd/app`、pluginapi 或 dsconversation。
- **范围**：可复用的流式 Markdown view、定稿/增量文档缓存、标题/列表/表格/行内码、宽表策略、代码块 banner/复制/wrap 或横滚、链接 hit 与选择复制；通过显式 callbacks 暴露 open/copy，组件自己不猜宿主。
- **验收**：
  1. 同一组件对 settled 与 streaming 输入均输出 Markdown；未闭合 fence/bold 不丢原文，完成后无整棵文档闪烁。
  2. 覆盖 H1–H6、有序/无序/嵌套/任务列表、4+ 列宽表、行内码、HTTP/file link；链接有点击命中区。
  3. 代码块复制逐字节等于原文；wrap toggle 或横滚真实可操作，长行不硬切；复制有成功/失败反馈。
  4. 基准记录流式追加只重排变化尾部；30 条以上消息不再以“超预算即纯文本”作为组件限制。
  5. `go test ./internal/mdlite/...`，附组件级无头几何/交互断言；不以截图代替复制和点击断言。

### WP-C · 消息区整合：Turn shell、工具状态、异常与滚动（依赖 WP-A/B）

- **Owner 类型**：原生消息区 Go UI owner；共享壳/公共契约串行。
- **Owned paths**：`plugins/dsconversation/**`、`plugins/dscore/**`、`pluginapi/plugin.go`、`cmd/app/main.go` 及对应测试。
- **范围**：接入 WP-A 身份和 WP-B 组件；气泡几何、思考摘要、工具全状态与专用卡、错误/stopped/retry、失败草稿恢复、消息动作行、回底/锚点、后台缓存、已加载 Turn 折叠/页脚。
- **验收**：
  1. 同一真帧流在“停留观看”和“A→B→A”后正文、工具、错误、顺序完全一致；后台内容已缓存，不靠整页 load 才出现。
  2. 流式到定稿保持同 message/event id；思考/工具展开状态按 session+真实 id，不切会话串状态。
  3. 注入 preparing/running/ok/error/stopped、两个交错工具、turn/error、取消后第二轮；各状态可见且结果归属正确。
  4. 发送失败恢复草稿且无假 durable 气泡；重试语义明确；HTTP link、代码复制、消息复制、回底按钮实际可点。
  5. 1182×832 与 1920×1032 对拍 user bubble `min(.702*contentWidth,82%)`、`10px 16px`；1000 条虚拟滚动中 prepend/流式/折叠保持语义锚点。
  6. `go test ./plugins/dsconversation/... ./plugins/dscore/... ./cmd/app/...`，另跑真实 apiclient 契约夹具；截图只作对应视觉证据。

### WP-D · Trajectory 当前窗口升级（可与 WP-C 并行，依赖 WP-A 数据形状）

- **Owner 类型**：原生 Trajectory/右栏 Go UI owner。
- **Owned paths**：`plugins/dsrightbar/**` 及测试；不改 dsconversation、cmd/app 或 apiclient。
- **范围**：在 WP-A 提供的数据上按 Turn/Step 分组用户/助手/工具/错误；搜索、展开记录、输入输出与错误详情、选择联动、未知时间隐藏；保留当前“最近 80 + 明示截断”的降级。
- **验收**：
  1. 不再只收 `role=tool`；当前窗口内各角色按真实 seq/turn 分组，嵌套调用按 parentCallId 缩进。
  2. 选择记录可查看原始 input/output/args/error；运行中 Time 为未知，不虚构耗时。
  3. 搜索、折叠、选择、向前 prepend 后 record key 稳定；1000 条只布可见行。
  4. 无 turnOutline/完整历史时明确显示“仅当前已加载窗口”，不冒充全量 Trajectory。
  5. `go test ./plugins/dsrightbar/...`，含交互与窗口缩放断言。

### WP-E · 附件/引用与反馈（契约冻结后独立实施）

- **Owner 类型**：原生附件/引用 UI owner + 对应宿主能力 owner；UI 与宿主按冻结契约并行。
- **Owned paths**：新增 `plugins/dsattachment/**`、`plugins/dsreference/**`、必要时 `plugins/dscomposer/**`；宿主侧 `../DSH-GO/internal/transport/{api.go,server.go,handlers_extra.go}`、`../DSH-GO/cmd/dshd/{serve.go,transport_source.go}` 及对应 snapshot/golden/tests（与 WP-A 的契约文件串行交接）。不得与 WP-C 同写 `pluginapi`/`cmd/app`，公共 Action 由 WP-C 先留稳定扩展点或串行交接。
- **范围**：有序图片/文件卡、授权加载、失败重试、灯箱；file/folder/session chip 与 referenceSummary；Like/Dislike 弹窗、版本冲突和指定 seq fork。
- **验收**：
  1. 单图长边 240、多图 64、Trajectory thumbnail 48；加载失败可重试，Escape/遮罩/关闭按钮关灯箱并还焦点。
  2. 文件链接支持 `#L24/#L24-L30`；带空格路径不丢；session 引用使用宿主稳定 id，不从标题反推。
  3. 反馈提交后 durable event 可回读；相同评分 retract，version-conflict 对账，失败保留弹窗草稿；无 messageId 的中断部分不显示反馈。
  4. fork 从指定 seq 创建并打开真实新会话，不用本地复制消息列表冒充。
  5. 真宿主端到端各一条成功与失败用例；未知字段隐藏，无假附件、假评分或假新会话。

## 5. 建议实施顺序与验收门

1. **先 WP-A 内容身份门**：旧消息不被覆盖、工具结果不串，才值得继续做表现层。
2. **WP-B 与 WP-A 并行**：纯渲染不依赖新宿主能力；WP-C 在两者接口稳定后整合。
3. **WP-D 与 WP-C 并行**：右栏只消费冻结数据，避免和消息区抢文件。
4. **WP-E 在对应契约真值就绪后做**：优先附件/文件链接，再反馈/fork；不要先画死按钮。
5. 每包验收都要区分：纯函数/组件测试、真实 apiclient 夹具、真宿主链路、视觉证据。源码存在、装配成功、单张截图、假审批或本地气泡消失均不等于官方行为完成。
