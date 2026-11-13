package uikernel

// official_command_node.go —— 官方 chat 的**命令节点渲染器**（本任务的"容器 D"）。
//
// # 官方事实（rc.2 随包 src/**）
//
//	ui-chat/src/client/chat/register-node-renderers.ts:74   command → CommandNodeView
//	ui-chat/src/client/chat/CommandNodeView.tsx:18
//	  <div className={css.callRow}>
//	    {renderSlot('conversation.chat.commandview', owner,
//	                { entryKey: command.name ?? '', fallback: <GenericCommandCard …/> })}
//	  </div>
//	ui-chat/src/client/chat/ChatView.module.css:205  .callRow{border-radius: var(--dsw-radius-sm)}
//	槽声明：contract/slots.ts:480  `conversation.chat.commandview` = **keyed**/session，
//	        owner 是 `{ node }`；"Reusing a key replaces that command renderer;
//	        an unoccupied key uses the generic card."
//
// 也就是说这个容器只有两件事：
//
//	① 画节点外壳 `.callRow`（圆角 sm），
//	② 按 **命令名**（entryKey）把 keyed 子槽 dispatch 出去；
//	   没人认领这个 key 时画官方 `GenericCommandCard` 兜底。
//
// # 为什么需要"认领"这一步
//
// 官方 renderSlot 带 fallback：key 没人占就画通用卡。本仓 `pluginapi.Painter`
// 没有 key 字段（槽位是扁平字符串），`Kernel.DrawSlot` 也不带 key，所以容器
// 必须自己问一次"这个命令名有人认领吗"（`Claims`）。答案是"有"才 dispatch，
// 否则画兜底卡——否则就会出现官方绝不会有的空白行。
//
// # 几何（GenericCommandCard.module.css，逐条照抄）
//
//	.root      flex-column
//	.leading   图标 14（DisclosureRow 的 leading）
//	.title     font-weight:400
//	.separator 2×2 / margin 0 8px / radius 1 / label-caption
//	.summary   13px / line-height 24 / nowrap+ellipsis；data-error → error 色
//	.body      <pre> max-height 260 / margin 4px 0 4px 4px / padding 12·16 /
//	           radius lg / .5px border-l1 / markdown-code-block 底 / pre-wrap

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// OffChatCommandView 是 `conversation.chat.commandview`（keyed/session）。
//
// 官方 slot 表里由 dsagentteam 用 RegisterOfficialSlot 登记（锚点
// `ui-chat/src/client/chat/register-node-renderers.ts:74`）；这里给出常量，
// 免得容器与登记处两处各写一遍字面量。
const OffChatCommandView pluginapi.Slot = "conversation.chat.commandview"

// 官方几何（`ui-chat/src/client/chat/GenericCommandCard.module.css`）。
const (
	// CommandRowRadius 是 `.callRow{border-radius: var(--dsw-radius-sm)}`；圆角值
	// 从主题 Radii.SM 取，这里只留一个语义常量方便断言。
	CommandNodeSeparatorSize float32 = 2
	CommandNodeSeparatorGap  float32 = 8
	CommandNodeSummaryFont   float32 = 13
	CommandNodeSummaryLine   float32 = 24
	CommandNodeBodyMaxHeight float32 = 260
	CommandNodeBodyMargin    float32 = 4
	CommandNodeBodyPadV      float32 = 12
	CommandNodeBodyPadH      float32 = 16
	CommandNodeIconSize      float32 = 14
	CommandNodeRowGap        float32 = 6
)

// 官方文案（`ui-chat/src/client/locale.ts` zh 字典，逐字）。
const (
	zhChatCommandRunning = "执行中…"
	zhChatCommandFailed  = "指令失败"
	zhChatCommandDone    = "已完成"
	zhChatCommandTitle   = "指令"
)

// CommandOutcome 是官方命令节点的 `outcome`：`null | {kind:'success'|'error', text}`。
//
// 用指针区分"没有 outcome（还在跑）"与"有 outcome 但文本为空"——前者画
// `command.running`，后者画 `command.done`/`command.failed`。
type CommandOutcome struct {
	Kind string // "success" | "error"
	Text string
}

// CommandNode 是官方 `conversation.chat.commandview` 的 `node` 入参（`{ name, outcome }`）。
type CommandNode struct {
	// Command 是命令名（官方 `node.name`，同时是 keyed 槽的 entryKey）。
	Command string
	// Outcome 为 nil 表示命令还没结算（running）。
	Outcome *CommandOutcome
	// Expanded 是兜底卡片的展开态（官方 useState(false)）。由调用方持有：
	// 同一条命令节点只允许一个展开态，容器本身无状态。
	Expanded bool
}

// CommandNodeOptions 是画一次命令节点容器的全部输入。
type CommandNodeOptions struct {
	Theme *pluginapi.Theme
	// Width 是流列宽（官方 `.callRow` 在 `--dsh-chat-content-width` 列里）。
	Width float32
	// Bounds 是这一帧的窗口矩形（夹 popup 用）。
	Bounds ui.Rect
	// Nodes 是这一帧要画的命令节点（顺序 = 流内顺序）。
	Nodes []CommandNode
	// Claims 判断 keyed 子槽有没有人认领这个命令名（官方 entryKey 语义）。
	Claims func(name string) bool
	// Toggle 是兜底卡片展开态的回写（点行时调用）。
	Toggle func(name string)
	// Dispatch 把子槽交给内核画（内核的 DrawSlot）。
	Dispatch func(pluginapi.Slot)
}

// DrawCommandNodeCards 画**没人认领**的命令节点（官方 `GenericCommandCard` 兜底卡）。
//
// 这是父容器被拆成的两半之一（另一半是 `DrawCommandNodeSlots`）：viva 的
// `Painter.Draw` 拿不到 drawSlot，只有 `Children` 有，所以：
//
//	Draw     ← 本函数：兜底卡的像素与命中区（不需要 dispatch）
//	Children ← DrawCommandNodeSlots：认领节点的 `.callRow` + 子槽 dispatch
//
// 返回画出来的兜底卡数量。
func DrawCommandNodeCards(f ui.C, env *pluginapi.Env, opts CommandNodeOptions) int {
	drawn, _ := drawCommandNodes(f, env, opts, false)
	return drawn
}

// DrawCommandNodeSlots 画**被认领**的命令节点：只画 `.callRow` 外壳，
// 再把 keyed 子槽 dispatch 出去（官方 `CommandNodeView.tsx:18`）。
//
// 返回 dispatch 出去的节点数量。
func DrawCommandNodeSlots(f ui.C, env *pluginapi.Env, opts CommandNodeOptions) int {
	drawn, _ := drawCommandNodes(f, env, opts, true)
	return drawn
}

// DrawCommandNodes 是两半的合成入口（测试与"dispatch 已经可用"的调用点用）。
//
// 生产路径上 `Painter.Draw` 用 `DrawCommandNodeCards`、`Painter.Children` 用
// `DrawCommandNodeSlots`；本函数只是把两者按同一判据跑一遍，方便单点断言。
func DrawCommandNodes(f ui.C, env *pluginapi.Env, opts CommandNodeOptions) int {
	if f == nil || len(opts.Nodes) == 0 {
		return 0
	}
	cards := DrawCommandNodeCards(f, env, opts)
	slots := DrawCommandNodeSlots(f, env, opts)
	return cards + slots
}

// drawCommandNodes 是共用实现；claimed 决定这一次只画认领的还是只画兜底的。
func drawCommandNodes(f ui.C, env *pluginapi.Env, opts CommandNodeOptions, claimed bool) (int, int) {
	if f == nil || len(opts.Nodes) == 0 {
		return 0, 0
	}
	t := opts.Theme
	if t == nil && env != nil {
		t = env.Theme
	}
	if t == nil {
		return 0, 0
	}
	width := opts.Width
	if width <= 0 && env != nil {
		width = env.MainWidth
	}
	drawn, dispatched := 0, 0
	for _, node := range opts.Nodes {
		if node.Command == "" {
			// 官方 entryKey 是 `command.name ?? ''`：没有名字的命令节点没有 key，
			// 认领不了也兜底不出来——不当成一条命令画（宁可少画，不画错行）。
			continue
		}
		hasClaim := opts.Claims != nil && opts.Claims(node.Command)
		if hasClaim != claimed {
			continue
		}
		// `.callRow`：只有圆角 sm，自己不占额外内边距/边框。
		ui.Column(f).FillWidth().Radius(t.Radii.SM).Children(func() {
			if hasClaim {
				if opts.Dispatch != nil {
					// keyed 子槽：认领方（ds-agent-team 的 TeamCommandRow）画整行。
					opts.Dispatch(OffChatCommandView)
					dispatched++
				}
				return
			}
			drawGenericCommandCard(f, env, t, node, width, opts.Toggle)
		})
		drawn++
	}
	return drawn, dispatched
}

// drawGenericCommandCard 是官方 `GenericCommandCard` 的 Go 等价物。
//
// 官方 summary 三分支（GenericCommandCard.tsx:30-35）：
//
//	outcome === null              → t('command.running')
//	有 text                       → text
//	否则 error → t('command.failed') / success → t('command.done')
//
// title = `node.name ?? t('command.title')`（这里是裸命令名，见官方注释）。
// 可展开条件：`text` 存在且含换行（body !== null）。
func drawGenericCommandCard(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, node CommandNode, width float32, toggle func(string)) {
	state := commandState(node.Outcome)
	title := node.Command
	if title == "" {
		title = zhChatCommandTitle
	}
	text := ""
	if node.Outcome != nil {
		text = node.Outcome.Text
	}
	summaryLabel := ""
	switch state {
	case "running":
		summaryLabel = zhChatCommandRunning
	case "error":
		if node.Outcome.Text != "" {
			summaryLabel = node.Outcome.Text
		} else {
			summaryLabel = zhChatCommandFailed
		}
	default:
		if node.Outcome.Text != "" {
			summaryLabel = node.Outcome.Text
		} else {
			summaryLabel = zhChatCommandDone
		}
	}
	expandable := text != "" && strings.Contains(text, "\n")
	open := node.Expanded && expandable

	summaryColor := t.LabelSecondary
	if state == "error" {
		summaryColor = t.Error
	}
	row := ui.Row(f).FillWidth().Items(ui.Center).Gap(CommandNodeRowGap)
	if expandable && toggle != nil {
		// 可展开行才是命中区（官方 DisclosureRow 的同款语义）。
		id := commandRowID(env, node.Command)
		clicked := row.HitWith(id, title, gfx.HitOptions{
			Role: gfx.RoleButton, Expandable: true, Expanded: open,
		}).Clicked()
		if clicked {
			toggle(node.Command)
		}
	}
	row.Children(func() {
		// leading 图标 14×14：官方是 IconApiOutlineRegular；本仓统一用命令图标。
		ui.Row(f).Width(CommandNodeIconSize).Height(CommandNodeIconSize).
			Items(ui.Center).Justify(ui.Center).Children(func() {
			ui.Icon(f, commandNodeIcon()).Size(CommandNodeIconSize, CommandNodeIconSize).
				Color(t.LabelTertiary)
		})
		ui.Text(f, title).FontSize(ModelMenuCellFont).LineHeight(ModelMenuCellLine).Color(t.LabelPrimary)
		// separator 2×2 + 两侧 8px
		ui.Row(f).Width(CommandNodeSeparatorSize).Height(CommandNodeSeparatorSize).
			Radius(CommandNodeSeparatorSize/2).Background(t.LabelCaption).
			MarginSides(0, CommandNodeSeparatorGap, 0, CommandNodeSeparatorGap)
		ui.Text(f, summaryLabel).FontSize(CommandNodeSummaryFont).LineHeight(CommandNodeSummaryLine).
			Color(summaryColor).Grow(1).Ellipsis()
	})
	if !open {
		return
	}
	// 官方 `.body` <pre>：max-height 260 / margin 4px 0 4px 4px / padding 12·16 /
	// radius lg / .5px border-l1 / 代码块底 / pre-wrap。
	ui.Column(f).FillWidth().MaxWidth(width).Background(t.CodeBlock).Radius(t.Radii.LG).
		Border(0.5, t.BorderL1).MaxHeight(CommandNodeBodyMaxHeight).
		PadSides(CommandNodeBodyPadV, CommandNodeBodyPadH, CommandNodeBodyPadV, CommandNodeBodyPadH).
		MarginSides(CommandNodeBodyMargin, CommandNodeBodyMargin, CommandNodeBodyMargin, CommandNodeBodyMargin).
		Children(func() {
			ui.Text(f, text).FontSize(t.Fonts.Code).LineHeight(t.Fonts.CodeLine).
				Color(t.LabelPrimary).Mono()
		})
}

// commandState 把 outcome 压成官方三态 `running | ok | error`（GenericCommandCard.tsx:20-23）。
func commandState(outcome *CommandOutcome) string {
	if outcome == nil {
		return "running"
	}
	if outcome.Kind == "error" {
		return "error"
	}
	return "ok"
}

// commandNodeIcon 取命令行的 leading 图标。
//
// 官方是 `IconApiOutlineRegular`（尖括号，16×16 viewBox）。本仓官方素材库里
// 没有这一枚（`$DSH_HOME/dsh-ui-official/assets/icons/` 下没有 api-outline），
// 这里按官方 viewBox=16 / stroke=1 画一枚**斜杠**（`/` —— 命令行的通用记号），
// 并在报告 §8 的差异表里登记"官方 api 图标缺失，用斜杠占位"。
func commandNodeIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Cap:     gfx.CapRound,
		Join:    gfx.JoinRound,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(12.05, 3.95),
			gfx.LineTo(3.95, 12.05),
		},
	}
}

// commandRowID 造兜底行交互 id（官方 DisclosureRow 的 row 是 button）。
func commandRowID(env *pluginapi.Env, name string) gfx.ID {
	if env == nil || env.Tree == nil {
		return gfx.ID{}
	}
	return env.Tree.ID(env.Feat, "command:"+name)
}
