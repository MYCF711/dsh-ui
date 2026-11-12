// official_transcript_nodes.go —— 主区 transcript 的**节点类型表**（rc.2 官方口径）。
//
// # 为什么单独一张表
//
// 官方主区的消息流不是"一列文本"，而是 `ChatNodeDataMap` 里 **20 个 kind**
// 各自一个渲染器（`dsh-client-ui-chat/src/client/chat/register-node-renderers.ts`
// 注册 17 个，goal / user-questions / workflow-run 三个包各注册 1 个）。
// 以前界面线只有 `msgs []apiclient.Message` 一条扁平的 role 流，于是：
//
//   - 没有任何测试能回答"官方这 20 种我们到底覆盖了几种"；
//   - 新增一种节点时只能靠人记，漏了看不出来（`submitted-plan` 就被误当成漏实现）。
//
// 把「kind → 数据声明锚点 / 渲染器注册锚点 / DOM 根 / 可见性」写死成数据，
// 绘制代码只消费这张表，测试逐条核对——"漏了第 7 种"于是变成可断言的事实。
//
// # 证据口径
//
// 锚点全部来自 rc.2 解包树（`tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`）
// 与主库 `home/dsh-ui-official/structure/official-conversation-transcript-tree.md`
// §5 的总表（20 行）。裸文件名（`conversation-nodes/*.ts`、`ChatNodeSeat.tsx` …）
// 相对 `dsh-client-ui-chat/src/client/chat/`，其余包写全路径——与结构树 §1.1 同一条规则。
//
// # 一条最容易错读的官方真值
//
// `submitted-plan` **没有渲染器**，`buildViewNode` 恒返回 `visibility:'hidden'`
// （`dsh-client-ui-plan/src/client/plan-definition.ts:28-34`）。它只作 Turn 尾卡
// （`conversation.chat.turnTail`）的数据源，**不产生 transcript 行**。
// 为它补一张行渲染器 = 做了个官方不存在的界面，所以本文件把它的
// `RendererAnchor` 留空、`DOMRoot` 留空、可见性写死 `hidden`，并有专门断言。
package dsconversation

// ChatNodeKind 是官方 `ChatNodeDataMap` 的键（= 渲染器分派键）。
//
// **不要**把它翻译成另一套名字：官方写 `turn-tail` 就是 `turn-tail`，
// 换个词就多一处会漂移的映射，而且没人知道它对应 v4 的哪一块。
type ChatNodeKind string

// 20 种节点类型（顺序 = 结构树 §5 总表的行号，方便对照）。
const (
	// ChatNodeUser 是回合开头的普通用户消息（`conversation-nodes/message.ts:36`）。
	ChatNodeUser ChatNodeKind = "user"
	// ChatNodeSteering 是活动 Turn 内被接收的用户消息（`message.ts:38`）。
	ChatNodeSteering ChatNodeKind = "steering"
	// ChatNodeContext 是非用户上下文注入（`message.ts:40`）。
	ChatNodeContext ChatNodeKind = "context"
	// ChatNodeTurnTrigger 是记录型唤醒 / 手动续写回执（`message.ts:42`）。
	ChatNodeTurnTrigger ChatNodeKind = "turn-trigger"
	// ChatNodeCommandInput 是 `/goal` 命令输入（goal 包注册）。
	ChatNodeCommandInput ChatNodeKind = "command-input"
	// ChatNodeAssistantStep 是助手正文（`conversation-nodes/assistant.ts:16-19`）。
	ChatNodeAssistantStep ChatNodeKind = "assistant-step"
	// ChatNodeToolCall 是工具调用卡 / 输出卡 / 失败态（`conversation-nodes/tool.ts:13-16`）。
	ChatNodeToolCall ChatNodeKind = "tool-call"
	// ChatNodeCommand 是斜杠命令生命周期（`conversation-nodes/command.ts:13-17`）。
	ChatNodeCommand ChatNodeKind = "command"
	// ChatNodeManualCompaction 是 `/compact` 与其压缩事务合一（`command.ts:17`）。
	ChatNodeManualCompaction ChatNodeKind = "manual-compaction"
	// ChatNodeCompaction 是自动压缩检查点（`conversation-nodes/compaction.ts:10-13`）。
	ChatNodeCompaction ChatNodeKind = "compaction"
	// ChatNodeModelRetry 是模型重试链（`conversation-nodes/retry.ts:10-13`）。
	ChatNodeModelRetry ChatNodeKind = "model-retry"
	// ChatNodeTurnError 是 Turn 终态失败（`conversation-nodes/turn-error.ts:10-13`）。
	ChatNodeTurnError ChatNodeKind = "turn-error"
	// ChatNodeTurnMaxTokens 是输出 token 上限结束（`turn-max-tokens.ts:8-11`）。
	ChatNodeTurnMaxTokens ChatNodeKind = "turn-max-tokens"
	// ChatNodeTurnProcess 是 Turn 级过程披露 + 耗时（`turn-process.ts:17-20`）。
	ChatNodeTurnProcess ChatNodeKind = "turn-process"
	// ChatNodeTurnTail 是 Turn 尾：动作行 + 用量 + 扩展位（`turn-tail.ts:14-17`）。
	ChatNodeTurnTail ChatNodeKind = "turn-tail"
	// ChatNodeSystemPrompt 是完整系统提示披露（`request-prompt.ts:10-13`）。
	ChatNodeSystemPrompt ChatNodeKind = "system-prompt"
	// ChatNodeUnknown 是未知 surface 兜底（`conversation-nodes/fallback.ts:9-12`）。
	ChatNodeUnknown ChatNodeKind = "unknown"
	// ChatNodeQuestionReply 是已答问题批次（user-questions 包注册）。
	ChatNodeQuestionReply ChatNodeKind = "question-reply"
	// ChatNodeSubmittedPlan 是计划提交——**恒 hidden，无渲染器**（plan 包）。
	ChatNodeSubmittedPlan ChatNodeKind = "submitted-plan"
	// ChatNodeWorkflowRun 是工作流运行（workflow-run 包注册）。
	ChatNodeWorkflowRun ChatNodeKind = "workflow-run"
)

// ChatNodeVisibility 是官方节点可见性。
//
// 三态而不是布尔：`hidden` 是**结构性**的（官方渲染器根本不会画），
// `conditional` 是**数据**驱动的（同一 kind 两种状态，例如 tool-call 的
// preparing+中断、turn-process 的未闭合 Turn）。把两者混成一个 bool
// 会让"这条为什么不见了"变成只能读代码的问题。
type ChatNodeVisibility string

const (
	// ChatVisibilityVisible 恒可见（正常渲染）。
	ChatVisibilityVisible ChatNodeVisibility = "visible"
	// ChatVisibilityHidden 恒隐藏：官方没有行渲染器，永不产生 DOM。
	ChatVisibilityHidden ChatNodeVisibility = "hidden"
	// ChatVisibilityConditional 条件可见：由数据决定，条件写在 Condition 里。
	ChatVisibilityConditional ChatNodeVisibility = "conditional"
)

// ChatNodeSpec 是官方一个节点类型的**全部结构事实**。
type ChatNodeSpec struct {
	// Kind 是渲染器分派键（`ChatNodeDataMap` 的键）。
	Kind ChatNodeKind
	// DataAnchor 是数据声明（`ChatNodeDataMap` 的 interface 成员）的 rc.2 锚点。
	DataAnchor string
	// RendererAnchor 是渲染器注册锚点；**空串 = 官方没有注册渲染器**
	// （只有 `submitted-plan` 这一种，是官方真值不是漏实现）。
	RendererAnchor string
	// DOMRoot 是渲染器产出的 DOM 根（类名 / 元素）；空串 = 不产生 DOM。
	DOMRoot string
	// Visibility 是官方可见性三态。
	Visibility ChatNodeVisibility
	// Condition 只在 `conditional` 时非空，写官方判据原文。
	Condition string
	// Anchor 是结构树 §5 的条目锚点（`official-conversation-transcript-tree.md §5.N`）。
	Anchor string
	// Note 记容易错读的语义（空 = 无）。
	Note string
}

// officialChatNodes 是**官方全量 20 种**，顺序 = 结构树 §5 总表行号。
//
// 这一列必须与 `structure/official-conversation-transcript-tree.md §5` 的
// 20 行逐条对应：少一种就是"主区漏画一种节点"，多一种就是"自造界面"。
var officialChatNodes = []ChatNodeSpec{
	{
		Kind:           ChatNodeUser,
		DataAnchor:     "conversation-nodes/message.ts:36",
		RendererAnchor: "register-node-renderers.ts:41-44",
		DOMRoot:        "div.userRow",
		Visibility:     ChatVisibilityVisible,
		Anchor:         "§5.1",
		Note:           "渲染器 UserMessageNodeView → UserStyleBubble；rc.1 起无 .bubble 类，宽度由 .userStack 承担",
	},
	{
		Kind:           ChatNodeSteering,
		DataAnchor:     "conversation-nodes/message.ts:38",
		RendererAnchor: "register-node-renderers.ts:46-49",
		DOMRoot:        "div.userRow[data-pending-steering][data-chat-steering-batch]",
		Visibility:     ChatVisibilityVisible,
		Anchor:         "§5.2",
		Note:           "同一渲染器但不拥有 message-source 槽（inject 返回 null）；首条 batch 行距 calc(44px + delta)",
	},
	{
		Kind:           ChatNodeContext,
		DataAnchor:     "conversation-nodes/message.ts:40",
		RendererAnchor: "register-node-renderers.ts:50-51",
		DOMRoot:        "DisclosureRow(.root)",
		Visibility:     ChatVisibilityVisible,
		Anchor:         "§5.3",
		Note:           "context/steering 行不得重复挂 conversation.chat.user.before（user 独占）",
	},
	{
		Kind:           ChatNodeTurnTrigger,
		DataAnchor:     "conversation-nodes/message.ts:42",
		RendererAnchor: "register-node-renderers.ts:52-53",
		DOMRoot:        "section.root | div.continuation",
		Visibility:     ChatVisibilityVisible,
		Anchor:         "§5.4",
		Note:           "两种形态：compact（details.compact）→ div.continuation；完整 → section.root + button.header + 可展开体",
	},
	{
		Kind:           ChatNodeCommandInput,
		DataAnchor:     "dsh-client-ui-goal/src/client/goal-command-input.ts:19-21",
		RendererAnchor: "dsh-client-ui-goal/src/client/index.ts:81-85",
		DOMRoot:        "GoalCommandInputView",
		Visibility:     ChatVisibilityVisible,
		Anchor:         "§5.5",
		Note:           "外部包节点：kind/注册位置/数据形状已取证，组件内部几何未取证（§9 第 4 条）→ 几何 derived",
	},
	{
		Kind:           ChatNodeAssistantStep,
		DataAnchor:     "conversation-nodes/assistant.ts:16-19",
		RendererAnchor: "register-node-renderers.ts:56-69",
		DOMRoot:        "div.root（.root > .body）",
		Visibility:     ChatVisibilityVisible,
		Anchor:         "§5.6",
		Note:           "block 分派：text→MarkdownText / reasoning→ReasoningRow / image→gallery / tool-call→交给 tool 行 / 其他→JsonBlock",
	},
	{
		Kind:           ChatNodeToolCall,
		DataAnchor:     "conversation-nodes/tool.ts:13-16",
		RendererAnchor: "dsh-client-ui-tool/src/client/apply.ts:37-44",
		DOMRoot:        "div.callRow → div.root",
		Visibility:     ChatVisibilityConditional,
		Condition:      "preparing && interruptedAt !== undefined → 'hidden'（conversation-nodes/tool.ts:345-351）",
		Anchor:         "§5.7",
		Note:           "三态 preparing|start|result；有退出码时保留命令描述，不显示错误首行",
	},
	{
		Kind:           ChatNodeCommand,
		DataAnchor:     "conversation-nodes/command.ts:13-17",
		RendererAnchor: "register-node-renderers.ts:70-75",
		DOMRoot:        "div.callRow",
		Visibility:     ChatVisibilityVisible,
		Anchor:         "§5.8",
		Note:           "按 command.name 走 keyed 子槽 conversation.chat.commandview，无注册落 GenericCommandCard",
	},
	{
		Kind:           ChatNodeManualCompaction,
		DataAnchor:     "conversation-nodes/command.ts:17",
		RendererAnchor: "register-node-renderers.ts:76-77",
		DOMRoot:        "div.callRow",
		Visibility:     ChatVisibilityVisible,
		Anchor:         "§5.9",
		Note:           "ManualCompactionChatData{command, compaction|null}；compaction === null 时不传压缩块",
	},
	{
		Kind:           ChatNodeCompaction,
		DataAnchor:     "conversation-nodes/compaction.ts:10-13",
		RendererAnchor: "register-node-renderers.ts:78-79",
		DOMRoot:        "div.compactionRow",
		Visibility:     ChatVisibilityVisible,
		Anchor:         "§5.10",
		Note:           "expandable = node.summary !== null；钉住态 z-index:7 压过代码块 banner 的 6",
	},
	{
		Kind:           ChatNodeModelRetry,
		DataAnchor:     "conversation-nodes/retry.ts:10-13",
		RendererAnchor: "register-node-renderers.ts:80-81",
		DOMRoot:        "details.retryRow",
		Visibility:     ChatVisibilityVisible,
		Anchor:         "§5.11",
		Note:           "倒计时锚在首次渲染该节点的浏览器时钟（deadline = Date.now() + delayMs），不是 Host 事件时间",
	},
	{
		Kind:           ChatNodeTurnError,
		DataAnchor:     "conversation-nodes/turn-error.ts:10-13",
		RendererAnchor: "register-node-renderers.ts:82-83",
		DOMRoot:        "div.turnErrorRow",
		Visibility:     ChatVisibilityVisible,
		Anchor:         "§5.12",
		Note:           "ACCOUNT_SIGNED_OUT / INTERRUPTED 用「任务已停止」标题；INTERRUPTED 不显示 code 胶囊",
	},
	{
		Kind:           ChatNodeTurnMaxTokens,
		DataAnchor:     "conversation-nodes/turn-max-tokens.ts:8-11",
		RendererAnchor: "register-node-renderers.ts:84-85",
		DOMRoot:        "div.turnErrorRow",
		Visibility:     ChatVisibilityVisible,
		Anchor:         "§5.13",
		Note:           "复用 .turnErrorRow 栅格，标题色改 --dsw-alias-state-warn-primary，状态点 state=warning",
	},
	{
		Kind:           ChatNodeTurnProcess,
		DataAnchor:     "conversation-nodes/turn-process.ts:17-20",
		RendererAnchor: "register-node-renderers.ts:86-87",
		DOMRoot:        "button.root",
		Visibility:     ChatVisibilityConditional,
		Condition:      "Turn 未闭合时渲染器返回 null（TurnProcessNodeView.tsx:18）——不是 CSS 隐藏",
		Anchor:         "§5.14",
		Note:           "elapsedMs 下限 1000ms；停止与失败不显示耗时；z-index:7 与 turn rail 同档",
	},
	{
		Kind:           ChatNodeTurnTail,
		DataAnchor:     "conversation-nodes/turn-tail.ts:14-17",
		RendererAnchor: "register-node-renderers.ts:88-97",
		DOMRoot:        "div.root",
		Visibility:     ChatVisibilityVisible,
		Anchor:         "§5.15",
		Note:           "closing === null 时只渲染扩展位 tail，不渲染动作行（没有耐久消息就不给 per-message 动作）",
	},
	{
		Kind:           ChatNodeSystemPrompt,
		DataAnchor:     "conversation-nodes/request-prompt.ts:10-13",
		RendererAnchor: "register-node-renderers.ts:54-55",
		DOMRoot:        "DisclosureRow(.root)",
		Visibility:     ChatVisibilityVisible,
		Anchor:         "§5.16",
		Note:           "体内容 = OpaqueBody（11px/16px code 字体、141px 上限）；标题按 update 二选一",
	},
	{
		Kind:           ChatNodeUnknown,
		DataAnchor:     "conversation-nodes/fallback.ts:9-12",
		RendererAnchor: "register-node-renderers.ts:98-99",
		DOMRoot:        "div.contextRow",
		Visibility:     ChatVisibilityVisible,
		Anchor:         "§5.17",
		Note:           "兜底只做 JsonBlock 原样渲染，**不得编造摘要**；ChatNodeSeat 自己的 fallback 也是 JsonBlock",
	},
	{
		Kind:           ChatNodeQuestionReply,
		DataAnchor:     "dsh-client-ui-user-questions/src/client/question-reply.ts:37-40",
		RendererAnchor: "dsh-client-ui-user-questions/src/client/index.ts:396-400",
		DOMRoot:        "QuestionReplyView",
		Visibility:     ChatVisibilityVisible,
		Anchor:         "§5.18",
		Note:           "挂在 conversation.chat.node keyed/session 槽下；审批走 composer 接管，问题答复走 transcript 行",
	},
	{
		Kind:           ChatNodeSubmittedPlan,
		DataAnchor:     "dsh-client-ui-plan/src/client/plan-definition.ts:8-11",
		RendererAnchor: "",
		DOMRoot:        "",
		Visibility:     ChatVisibilityHidden,
		Condition:      "buildViewNode 恒 visibility:'hidden'（plan-definition.ts:28-34）",
		Anchor:         "§5.19",
		Note:           "官方真值：只作 Turn 尾卡（conversation.chat.turnTail）的数据源，**不要**补行渲染器",
	},
	{
		Kind:           ChatNodeWorkflowRun,
		DataAnchor:     "dsh-client-ui-workflow-run/src/client/workflow-definition.ts:38-41",
		RendererAnchor: "dsh-client-ui-workflow-run/src/client/index.ts:29-36",
		DOMRoot:        "WorkflowRunPanel",
		Visibility:     ChatVisibilityVisible,
		Anchor:         "§5.20",
		Note:           "外部包节点：内部几何未逐类取证（§9 第 4 条）→ 几何 derived",
	},
}

// OfficialChatNodes 返回官方全量节点表的只读副本（顺序稳定，20 行）。
func OfficialChatNodes() []ChatNodeSpec {
	return append([]ChatNodeSpec(nil), officialChatNodes...)
}

// ChatNodeSpecFor 按 kind 取结构事实；ok=false 表示**不存在这个 kind**
// （调用方必须当未知处理，不能自己编一个默认值）。
func ChatNodeSpecFor(kind ChatNodeKind) (ChatNodeSpec, bool) {
	for _, spec := range officialChatNodes {
		if spec.Kind == kind {
			return spec, true
		}
	}
	return ChatNodeSpec{}, false
}

// ChatNodeRenders 报告这个 kind 在官方有没有行渲染器。
//
// 判据只有一条：`RendererAnchor` 非空。**不要**用 switch 写死 kind 列表——
// 那样新增一种节点时两处都要改，而漏改的表现是"静默少画一行"。
func ChatNodeRenders(kind ChatNodeKind) bool {
	spec, ok := ChatNodeSpecFor(kind)
	return ok && spec.RendererAnchor != ""
}

// ChatNodeKinds 返回官方 20 个 kind（按结构树顺序）。
func ChatNodeKinds() []ChatNodeKind {
	out := make([]ChatNodeKind, 0, len(officialChatNodes))
	for _, spec := range officialChatNodes {
		out = append(out, spec.Kind)
	}
	return out
}

// ── 官方槽位（transcript 这一层的三处挂载点）──────────────────────────────
//
// 结构树 §2 的层级总树把消息流拆成三层槽：conversation.content（工厂槽）→
// conversation.session（会话级 View）→ conversation.view（Chat tab）。
// 这三个名字是官方 `ui-conversation/src/client/contract/slots.ts` 的字面量，
// **不许改写成 main.body 之类的旧槽名**（旧槽只有兼容桥会 dispatch）。
const (
	// SlotConversationContent 是 `conversation.content`（工厂槽，子槽由会话容器承载）。
	SlotConversationContent = "conversation.content"
	// SlotConversationSession 是 `conversation.session`（会话级 View 容器）。
	SlotConversationSession = "conversation.session"
	// SlotConversationView 是 `conversation.view`（Chat 视图 tab，order=0）。
	SlotConversationView = "conversation.view"
	// SlotChatFlow 是 `conversation.chat.flow`——**display:contents** 的流槽。
	//
	// 它保留布局但不保留选择器祖先：官方必须用
	// `:global([data-slot="conversation.chat.flow"])` 显式穿透。
	// 给它加 display:flex 或任何盒模型都会让行距选择器失效（结构树 §2 语义 1）。
	SlotChatFlow = "conversation.chat.flow"
	// SlotChatNode 是 `conversation.chat.node`——按 entryKey 分派的节点槽。
	SlotChatNode = "conversation.chat.node"
)

// ChatFlowSlotDisplay 是 flow 槽的 display 值（官方 `display:contents`）。
//
// 单独一个常量：测试要断言"flow 槽没有盒模型"，绘制代码要拿它做唯一出处。
const ChatFlowSlotDisplay = "contents"
