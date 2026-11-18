// Package dsactioncards 是官方 `dsh-client-ui-action-cards` 的 Go 版：
// 「答案下面的动作卡」+「`propose_action` 工具调用的过程行」。
//
// # 结构证据（唯一权威，几何/文案/结构全部照它）
//
//	OFF = DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/dsh-client-ui-action-cards/src/client
//
//	OFF:Card.tsx            150 行：卡片框（标题 + 折叠 brief + kind body/兜底 body）
//	OFF:Card.module.css       7 行：卡片框全部几何值
//	OFF:ProposeRow.tsx       43 行：`propose_action` 调用的一行过程行（五态）
//	OFF:ProposeRow.module.css 6 行：过程行几何
//	OFF:locales.ts           22 行：zh 字典 14 条（本包**只许**用这 14 条原文）
//	OFF:mount.ts             63 行：两个座位（conversation.chat.turnTail / tool.call.toolview）
//	OFF:registry.ts          72 行：kind body 注册表（本包**搬了**：见 §「正文注册表」）
//
// # 数据面证据（宿主是真的，不是注释）
//
//	HOST = DSH-GO/tmp/GO8-RC2/_audit-tree
//	HOST:internal/actioncards/service.go:216-235   View / ViewMap（投影真源）
//	HOST:internal/actioncards/service.go:237-331   Act（四变体：accepted/settled/refused/unconfirmed）
//	HOST:internal/transport/handlers_account_official.go:1447-1502  actionCards.act handler
//	HOST:internal/transport/handlers_session_official.go:947         values["actionCards"]
//	HOST:cmd/dshd/serve_action_cards.go:55-57 / serve.go:941       产品启动路径上已挂真实现
//
// # 正文注册表（2026-11-15 补齐）
//
// 官方 `ctx.actionCards`（OFF:mount.ts:47-49 的 `reflect.provide`）在本仓的对等物是
// `pluginapi.KeyActionCardBodies`：本包在 `Register` 里把它 `Provide` 出去，任何
// action kind 的插件用 `Requires()` 声明依赖后，在自己的 `Register` 里
// `RegisterBody(…)` 一次。派发在 `drawCardTail`：注册了正文的 kind 由正文**接管**
// 卡片内部（官方 OFF:Card.tsx:99-118 同一形状），没注册的画 FallbackBody。
// 第一个消费方是 `plugins/dspluginreco`（官方 `dsh-client-ui-plugin-recommendations`）。
//
// # 它要壳注入什么（**接线时照这个来**）
//
//  1. 数据面：`SetSource(CardSource)`。source 是 `*apiclient.Client` 的薄包装，
//     实现 `View`（= `ActionCardsView`）与 `Act`（= `ActionCardAct`）。
//     **必须在 `Load` 之后注入**——那之前 apiclient 还没拿到宿主地址。
//     为什么走包级注入而不是 `Register` 里 `ctx.Inject`：插件拿不到
//     `*apiclient.Client`（界面线 R2/R7：插件不建 Transport、不直连宿主），
//     与 `plugins/dsdirpicker` 的 `SetSource`、`plugins/dsagentteam` 的
//     `AttachShellTier` 同一时序与理由。
//  2. 刷新：`Refresh(ctx, sessionID)`。壳在 `session.load` / `session.history`
//     补完基线后、以及收到新的 journal 帧后各调一次即可（同一会话重复调幂等）。
//     **不要在 Draw 里发请求**：绘制是同步的，一次慢请求会把整窗卡住。
//  3. 插件登记：`cmd/app/main.go` 加一行 `_ ".../plugins/dsactioncards"`。
//     槽位名写死在本文件的两个常量里（官方字面量），壳不需要额外声明。
//
// **没有 source 时整块不画**（一个节点都不画）；投影为空时同样整块不画——
// 这与官方「空槽不占位」同义。画一个点了没反应的动作卡比不画更坏。
//
// # 交互（照 OFF:Card.tsx / OFF:ProposeRow.tsx）
//
//   - 卡片：标题 + brief（超过 260 拉丁字宽折叠，展开/收起切换）+ 决定状态行
//     （`dismissed`→已忽略 / `replaced`→已替换）+ 「暂不需要」按钮（未决定时才有）
//   - 点「暂不需要」= `act('dismiss')`（OFF:Card.tsx:134）；成功后按宿主真实 status 渲染
//   - `refused` / `unconfirmed`：显示官方 `unavailable` 文案，并在下面显示**宿主 reason 原文**
//   - `Act` 报错：显示错误原文，卡片**不消失**
//   - 过程行：`row.title` + 状态文案（preparing/running/ok/stopped/error）+ 卡片标题；
//     失败时点行展开看宿主结果原文
//
// # 没搬的官方细节（照实登记，不假装搬了）
//
//   - **kind body 注册表的 TypeScript 形状**：官方 body 是 React 组件，还额外收
//     `locale`（运行时字典）与 `Folded`（按字素折叠的组件）两个参数。本仓的正文是
//     `pluginapi.ActionCardBody`（`Action() + Draw()`），没有运行时字典、也没有
//     Folded——理由逐条写在 `pluginapi/action_card_bodies.go` 文件头的「有意差异」。
//     没有注册正文的 kind 仍画官方 `FallbackBody`（OFF:Card.tsx:118-150），其中
//     kind 自有动词（非 dismiss）**没有入口可点**：不画假按钮（官方在没有 body 时
//     也只给 dismiss）。
//   - ~~**per-turn 座位**~~（2026-11-18 已补）：官方 turnTail 按轮取
//     （`proposalOf(view, turn)`，OFF:Card.tsx:69-76）；壳现在按轮派发
//     （`Env.Turn`，见 `pluginapi/turn_tail.go`），本包按那一轮的 turn 选卡。
//     只有壳没有轮次基线（老壳 / mockd）时才退回「最新一轮 + 最小 callId」。
//   - **per-call 过程行**：官方 `tool.call.toolview` 是 keyed 槽（key=`propose_action`），
//     由 tool-call 节点在**每个调用处**渲染；本骨架的座位是会话级的，所以这里只画
//     **最近一次** `propose_action` 调用的一行。
//   - **DisclosureRow 的运行 shimmer / 悬停 chevron 置换**：本仓复用官方
//     `apps-add-outline` sprite 与 16px leading / 14px glyph 几何；运行 shimmer 和
//     悬停时把 apps-add 换成 chevron 的动效未搬（ProposeRow.tsx:35 + DisclosureRow.tsx:77-91）。
//   - **「查看调用」按钮**（OFF:ProposeRow.tsx:40，`inspect`）：需要壳提供"在轨迹视图
//     里定位这次调用"的能力，`pluginapi.Actions` 没有这一格，所以不画（不画假按钮）。
//   - **grapheme 切分**：官方用 `Intl.Segmenter` 按字素算宽度（OFF:Card.tsx:52-61）；
//     Go 侧按 rune 算。组合字符（emoji ZWJ 序列）可能提前一个字符折叠——
//     只影响"展开/收起"的出现位置，不改写原文。
//   - **`transport` 文案**：官方在"响应丢失"时显示 `transport`。本仓的 apiclient 把
//     丢包报成 error，所以走"错误原文"那条分支；`transport` 常量保留但当前无入口
//     （不为了让常量被用到而编一个状态）。
package dsactioncards

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 座位：官方字面量（OFF:mount.ts:48-62 的两个 register 名）────────────────
const (
	// SlotTurnTail 是官方 `conversation.chat.turnTail`（list/session，
	// ui-chat/src/client/contract/slots.ts:492）；答案下面的动作卡挂这里。
	SlotTurnTail = pluginapi.Slot("conversation.chat.turnTail")
	// SlotToolView 是官方 `tool.call.toolview`（keyed/session，
	// ui-tool/src/client/contract/slots.ts:23-27）；key=`propose_action` 的过程行挂这里。
	SlotToolView = pluginapi.Slot("tool.call.toolview")
)

// ── 文案：只有这 14 条，逐字来自 OFF:locales.ts:15-22 的 zh 字典 ──────────────
//
// **不许新写中文**：界面上一旦出现这 14 条以外的字样，就不是官方那个卡片了。
const (
	zhExpandBrief   = "展开"                        // expandBrief
	zhCollapseBrief = "收起"                        // collapseBrief
	zhDismiss       = "暂不需要"                      // dismiss
	zhDismissed     = "已忽略"                       // dismissed
	zhReplaced      = "已替换"                       // replaced
	zhUnavailable   = "暂时无法执行，请到插件页面查看"           // unavailable
	zhTransport     = "暂时无法确认结果，请重新连接并查看卡片状态后再重试" // transport
	zhRowTitle      = "操作卡片"                      // row.title
	zhRowPreparing  = "正在准备卡片"                    // row.preparing
	zhRowRunning    = "正在提议"                      // row.running
	zhRowOK         = "已提议"                       // row.ok
	zhRowError      = "未显示"                       // row.error
	zhRowStopped    = "已中断"                       // row.stopped
	zhRowInspect    = "查看调用"                      // row.inspect
)

// verbDismiss 是官方共享动词（OFF:Card.tsx:134 的 `act('dismiss')`；
// 宿主侧常量 HOST:internal/actioncards/service.go:45 `VerbDismiss`）。
const verbDismiss = "dismiss"

// proposeActionName 是官方工具名（OFF:../dsh-action-cards/src/projection.ts:8；
// 宿主 HOST:internal/actioncards/service.go:34 `ToolName`）。
const proposeActionName = "propose_action"

// ── 几何：全部来自 OFF:Card.module.css / OFF:ProposeRow.module.css ───────────
const (
	// 卡片框 `.card{ margin:16px 0; border:0.5px solid var(--dsw-alias-border-l2);
	// border-radius: var(--dsw-radius-lg); overflow:hidden;
	// background: var(--dsw-alias-bg-layer-1) }`（Card.module.css:1）。
	cardMarginY float32 = 16
	cardBorder  float32 = 0.5
	// `.body{ padding: 12px 20px 20px }`（Card.module.css:2）。
	cardBodyPadTop    float32 = 12
	cardBodyPadRight  float32 = 20
	cardBodyPadBottom float32 = 20
	cardBodyPadLeft   float32 = 20

	// `.title{ display:block; font-size:16px; font-weight:500; line-height:1.5 }`
	// （Card.module.css:3）→ 行高 16×1.5=24。
	cardTitleFontSize float32 = 16
	cardTitleLineH    float32 = 24
	cardTitleWeight   float32 = 500

	// `.text{ margin:8px 0 0; font-size:13px; line-height:1.7 }`（Card.module.css:4）
	// → 行高 13×1.7=22.1。
	cardTextMarginTop float32 = 8
	cardTextFontSize  float32 = 13
	cardTextLineH     float32 = 22.1

	// 共享 small Button：28 高、0×10、12/18、radius-sm、disabled opacity .4
	// （ui-primitives/src/Button.module.css:18-34）。两个按钮都是 ghost
	// （Card.tsx:64,147），hover/active 色在同文件 :41-53。
	smallButtonHeight          float32 = 28
	smallButtonPadX            float32 = 10
	smallButtonFontSize        float32 = 12
	smallButtonLineH           float32 = 18
	smallButtonDisabledOpacity float32 = 0.4

	// `.toggle{ margin:8px 0 0; padding-inline:0; height:auto; min-height:24px;
	// font-size:12px }`（Card.module.css:5）；small Button 的行高仍是 18。
	toggleMarginTop float32 = 8
	toggleMinHeight float32 = 24

	// `.status{ margin:12px 0 0; font-size:12px; line-height:1.6 }`（Card.module.css:6）
	// → 行高 12×1.6=19.2。
	statusMarginTop float32 = 12
	statusFontSize  float32 = 12
	statusLineH     float32 = 19.2

	// `.footer{ display:flex; align-items:center; justify-content:flex-end;
	// gap:12px; margin-top:12px }`（Card.module.css:7）。
	footerGap       float32 = 12
	footerMarginTop float32 = 12

	// DisclosureRow 默认密度（delta=0）：行高 24、leading 16、右间距 6、
	// 标题 13/24（ui-primitives/src/DisclosureRow.module.css:30-39,52-74,104-109）。
	rowHeight             float32 = 24
	rowLeadingSize        float32 = 16
	rowLeadingMarginRight float32 = 6
	rowTitleFontSize      float32 = 13
	rowTitleLineH         float32 = 24
	// 过程行 `.summary{ margin-left:8px; gap:8px; font-size:12px }`
	// （ProposeRow.module.css:2）。
	rowSummaryMarginLeft float32 = 8
	rowSummaryGap        float32 = 8
	rowSummaryFontSize   float32 = 12
	// leading 内的官方 `IconAppsAddOutlineRegular size={14}`（ProposeRow.tsx:35；
	// DisclosureRow.module.css:71-74）。素材复用 dsassets.SidebarAppsAdd。
	rowIconSize float32 = 14

	// `.reason{ margin:8px 0; padding:12px; border-radius:var(--dsw-radius-lg);
	// background:var(--dsw-alias-bg-layer-1); color:var(--dsw-alias-label-secondary);
	// white-space:pre-wrap; font-size:12px }`（ProposeRow.module.css:5）。
	reasonMarginY  float32 = 8
	reasonPad      float32 = 12
	reasonFontSize float32 = 12

	// `.inspect{ align-self:flex-start; padding:4px 0; font-size:12px;
	// color:var(--dsw-alias-link) }`（ProposeRow.module.css:6）。
	inspectPadY     float32 = 4
	inspectFontSize float32 = 12

	// foldWidth 是 brief 的折叠预算，单位是"拉丁字母宽"（官方 `FOLD_WIDTH = 260`，
	// OFF:Card.tsx:41：约两行卡片正文）。
	foldWidth = 260
)

// actTimeout 是一次用户动作调用的上限。
//
// 为什么要有：点完之后按钮进 disabled（官方 `pending`），一次挂住的请求不设上限
// 会让按钮永远回不来——那比报错更糟（用户以为点了没反应）。
const actTimeout = 15 * time.Second

// CardSource 由壳注入；nil = 没有数据能力，整块不画。
//
// 两条方法一一对应官方那条 Remote（OFF:src/client/mount.ts:54-57 的
// `ctx.remote.actionCards.act` 与 `useProjection('actionCards')`），
// 参数/返回值形状照搬 apiclient：
//
//	View = (*apiclient.Client).ActionCardsView
//	Act  = (*apiclient.Client).ActionCardAct
type CardSource interface {
	View(ctx context.Context, sessionID string) (*apiclient.ActionCardView, error)
	Act(ctx context.Context, sessionID, callID, verb string, payload map[string]json.RawMessage) (json.RawMessage, error)
}

var (
	injectMu sync.RWMutex
	// injected 是壳塞进来的数据源（nil = 没有）。
	injected CardSource
)

// SetSource 由壳注入数据面（**必须在 Load 之后**）。
//
// 传 nil = 收回能力（动作卡从此整块不画）——老壳 / 没有 apiclient 时就是这样。
func SetSource(src CardSource) {
	injectMu.Lock()
	injected = src
	injectMu.Unlock()
}

// currentSource 取当前注入的数据源（每帧都重新问，壳中途注入/收回都立刻生效）。
func currentSource() CardSource {
	injectMu.RLock()
	defer injectMu.RUnlock()
	return injected
}

// Plugin 是动作卡本体。
type Plugin struct {
	theme *pluginapi.Theme

	// run 执行一次宿主调用。
	//
	// 默认 nil = `go fn()`：Act 背后是 HTTP，在绘制线程里同步等会把整个窗口卡住。
	// 测试把它换成同步执行，点击 → 状态落定在同一线程，断言才是确定的
	// （与 `plugins/dsdirpicker` 的 `run` 同一个思路）。
	run func(fn func())

	mu sync.Mutex
	// act 是上一帧的 Actions：异步回调要靠它的 Invalidate 请壳重绘一帧。
	act pluginapi.Actions

	// view 是最近一次刷新到的官方投影；nil = 没有数据 → 整块不画。
	view        *apiclient.ActionCardView
	viewSession string
	// session 是这一帧正在画的会话（点按钮时用它，避免 viewSession 为空时发不出请求）。
	session string

	// expandedBrief / expandedRow 是两个折叠位的展开态。
	//
	// expandedBrief **按 callId 记**：turnTail 座位按轮派发后，同一帧里会画多张卡
	// （一轮一张），共一个布尔量会让「展开」互相串（点开第 1 轮的 brief，第 3 轮
	// 那份也跟着展开）。官方每张卡各自持有折叠态（`FoldedText` 的 `useState`，
	// OFF:Card.tsx:51；每张卡各渲染一个），这里用同一张表的 per-callId 项等价承载。
	expandedBrief map[string]bool
	expandedRow   bool

	// acting 表示有一次动作在飞（按钮 disabled，官方 `pending`）。
	acting bool
	// actResult 是宿主真实返回的四变体；actError 是失败时的**宿主错误原文**。
	actResult apiclient.ActionCardResult
	actError  string
	// actFor 记录这份反馈属于哪个 callId：换卡之后旧反馈不许串到新卡上。
	actFor string

	// cardBoxes / rowBoxes 是最近一帧画出来的几何（无头测试量它；产品路径只写不读）。
	cardBoxes cardBoxes
	rowBoxes  rowBoxes

	// bodies 是注册进来的 kind 正文（官方 ActionCardRegistry 的 bodies map）。
	//
	// 键是 action 名；空 = 一种都没注册，此时所有卡片都走官方 FallbackBody
	// （与官方"没有 body 就画 fallback"同义）。受 p.mu 保护。
	bodies map[string]pluginapi.ActionCardBody
}

// errEmptyCardBodyAction 是"注册了一个说不出 action 名的正文"。
//
// 不用 action 名去索引的注册项在本仓无处存放：官方那边注册项自带 action，
// 空串会在注册时就被类型系统挡住；Go 这边只能显式拒绝。
var errEmptyCardBodyAction = errors.New("ds-actioncards: 卡片正文没有 action 名")

// cardBoxes 是卡片这一帧的关键矩形与状态行。
type cardBoxes struct {
	Card    ui.Box
	Body    ui.Box
	Title   ui.Box
	Brief   ui.Box
	Toggle  ui.Box
	Footer  ui.Box
	Dismiss ui.Box
	Statuss []ui.Box
}

// rowBoxes 是过程行这一帧的关键矩形。
type rowBoxes struct {
	Row       ui.Box
	Icon      ui.Box
	Title     ui.Box
	State     ui.Box
	CardTitle ui.Box
	Reason    ui.Box
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-actioncards" }

// Provides 实现 pluginapi.Plugin：动作卡对外提供**正文注册表**
// （官方 `ctx.reflect.provide('actionCards', registry)`，OFF:mount.ts:47-49）。
//
// 不提供它，`enable_official_plugin` 那类卡片就只能画官方 FallbackBody——
// 也就是"标题 + 暂不需要"，kind 自己的动词没有入口可点。
func (*Plugin) Provides() []string { return []string{pluginapi.KeyActionCardBodies} }

// Requires 实现 pluginapi.Plugin：没有设计令牌就画不出官方那套颜色。
func (p *Plugin) Requires() []string { return []string{pluginapi.KeyTheme} }

// Register 实现 pluginapi.Plugin：注入主题 + 提供正文注册表，不画东西。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme, _ = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	ctx.Provide(pluginapi.KeyActionCardBodies, pluginapi.ActionCardBodyRegistry(p))
	return nil
}

// RegisterBody 实现 pluginapi.ActionCardBodyRegistry：登记一种 kind 的正文。
//
// 重复登记**报错不覆盖**（OFF:registry.ts:104-106 抛 `a body for "…" is already
// registered`）：两个人抢一种卡片的绘制权，静默覆盖会让被覆盖那一方永远看不见
// 自己的改动。action 为空同样拒绝——那是注册方写错，不是"画默认"。
func (p *Plugin) RegisterBody(body pluginapi.ActionCardBody) error {
	if body == nil || body.Action() == "" {
		return errEmptyCardBodyAction
	}
	action := body.Action()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.bodies == nil {
		p.bodies = map[string]pluginapi.ActionCardBody{}
	}
	if _, exists := p.bodies[action]; exists {
		return fmt.Errorf("ds-actioncards: action %q 的正文已经被注册过", action)
	}
	p.bodies[action] = body
	return nil
}

// BodyFor 实现 pluginapi.ActionCardBodyRegistry：取一种 kind 的正文（没有返回 nil）。
func (p *Plugin) BodyFor(action string) pluginapi.ActionCardBody {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.bodies[action]
}

// Panels 实现 pluginapi.Panels：官方两个座位各一个画法。
//
// Order 取 60：与 transcript 的节点（60 档）同层，画在正文列里；
// 空数据时两个 Draw 都直接 return，不占位。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "action-card-tail", Slot: SlotTurnTail, Order: 60, Draw: p.drawCardTail},
		{ID: "action-propose-row", Slot: SlotToolView, Order: 60, Draw: p.drawProposeRow},
	}
}

// Refresh 拉一次当前会话的动作卡投影并缓存。
//
// 壳在 load/history 之后以及收到新 journal 帧后调即可；同一会话重复调幂等。
// 返回 error 时**保留上一帧的显示**（一次查询失败不等于"卡片没了"）。
func (p *Plugin) Refresh(ctx context.Context, sessionID string) error {
	src := currentSource()
	if src == nil {
		return nil
	}
	view, err := src.View(ctx, sessionID)
	p.mu.Lock()
	if err != nil {
		p.mu.Unlock()
		return err
	}
	if p.viewSession != sessionID {
		// 换会话：两个折叠位与反馈都重来（旧会话的展开态不该跟到新会话）。
		p.expandedBrief, p.expandedRow = nil, false
		p.actResult, p.actError, p.actFor = apiclient.ActionCardResult{}, "", ""
	}
	p.view, p.viewSession = view, sessionID
	p.mu.Unlock()
	p.invalidate()
	return nil
}

// dispatch 执行一次宿主调用（默认丢给 goroutine，见 Plugin.run 的注释）。
func (p *Plugin) dispatch(fn func()) {
	if p.run != nil {
		p.run(fn)
		return
	}
	go fn()
}

// invalidate 请壳重绘一帧（异步结果落地之后必须调，否则画面停在旧状态）。
func (p *Plugin) invalidate() {
	p.mu.Lock()
	fn := p.act.Invalidate
	p.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// failureText 把一次失败变成界面上那一行**原文**。
//
// 官方 failureText 优先读 `rpcError.message`；本仓的对等物是
// `*apiclient.APIError.Message`。其它错误原样 Error()——**不新写中文**，
// 也不把宿主的话改写成人话。
func failureText(err error) string {
	if err == nil {
		return ""
	}
	var api *apiclient.APIError
	if errors.As(err, &api) && api.Message != "" {
		return api.Message
	}
	return err.Error()
}

// ── 动作：点按钮 ────────────────────────────────────────────────────────────

// proposalFor 取这一帧该画的那**一张**卡。
//
// 官方座位按轮（owner props 带 turn，OFF:Card.tsx:89 的 `proposalOf(view, turn.turn)`），
// 本仓壳按轮派发时把那一轮的 turn 放在 `env.Turn` 上，于是这里走
// `ProposalForTurn`（同轮多张取最小 callId，逐条照 OFF:Card.tsx:69-76）。
//
// `env.Turn == nil` = 没有轮次基线（老壳 / mockd / 还没有 turn/end 帧）：退回
// `LatestProposal` 的会话级近似——改造前就是这个行为。
func (p *Plugin) proposalFor(env *pluginapi.Env, view *apiclient.ActionCardView) (apiclient.ActionCardProposal, bool) {
	if view == nil {
		return apiclient.ActionCardProposal{}, false
	}
	if env != nil && env.Turn != nil {
		return view.ProposalForTurn(env.Turn.Turn)
	}
	return view.LatestProposal()
}

// actVerb 对**指定那张卡**执行一次动作（官方 `act(callId, verb, payload)`）。
func (p *Plugin) actVerb(callID, verb string, payload map[string]json.RawMessage) {
	src := currentSource()
	if src == nil || callID == "" || verb == "" {
		return
	}
	p.mu.Lock()
	if p.acting || p.view == nil {
		p.mu.Unlock()
		return
	}
	session := p.session
	if session == "" {
		session = p.viewSession
	}
	if session == "" {
		p.mu.Unlock()
		return
	}
	p.acting = true
	p.actError = ""
	p.actResult = apiclient.ActionCardResult{}
	p.actFor = callID
	p.mu.Unlock()
	p.invalidate() // 先把 disabled 那一帧放出来

	p.dispatch(func() {
		ctx, cancel := context.WithTimeout(context.Background(), actTimeout)
		defer cancel()
		raw, err := src.Act(ctx, session, callID, verb, payload)
		p.mu.Lock()
		p.acting = false
		switch {
		case err != nil:
			// 失败：卡片留着，宿主错误原文挂在状态行上（官方 `transport` 那一档的等价物）。
			p.actError = failureText(err)
		default:
			result, decodeErr := apiclient.DecodeActionCardResult(raw)
			if decodeErr != nil {
				// 响应形状不合约：如实报出来，不猜它是什么状态。
				p.actError = decodeErr.Error()
			} else {
				p.actResult = result
				if result.Decision != nil {
					p.applyDecisionLocked(*result.Decision)
				}
			}
		}
		p.mu.Unlock()
		p.invalidate()
	})
}

// applyDecisionLocked 把宿主回的决定并进投影（在锁内调用）。
//
// 官方在动作之后**不自己编状态**：`settled` 带回来的 decision 就是真值，
// 重新读投影是同一份数据；这里直接并入，省一次 round-trip。
func (p *Plugin) applyDecisionLocked(decision apiclient.ActionCardDecision) {
	if p.view == nil || decision.CallID == "" {
		return
	}
	if p.view.Decisions == nil {
		p.view.Decisions = map[string]apiclient.ActionCardDecision{}
	}
	p.view.Decisions[decision.CallID] = decision
}

// ── 绘制：卡片（官方 turnTail 座位）──────────────────────────────────────────

// drawCardTail 画动作卡。**没有 source / 投影为空 / 没有提案就一个节点都不画**。
func (p *Plugin) drawCardTail(f ui.C, env *pluginapi.Env) {
	t := p.theme
	if t == nil {
		return
	}
	// 上一帧的几何作废：这一帧一个节点都不画时，boxes 必须是空的
	// （测试据此断言"没有数据就什么都没画"）。
	p.cardBoxes = cardBoxes{}

	p.mu.Lock()
	p.act = env.Act
	view := p.view
	session := p.viewSession
	expandedBrief := p.expandedBrief
	acting := p.acting
	actError, actResult, actFor := p.actError, p.actResult, p.actFor
	p.mu.Unlock()

	if currentSource() == nil || view == nil {
		return
	}
	if session == "" && env != nil {
		session = env.Data.Cur
	}
	p.mu.Lock()
	p.session = session
	p.mu.Unlock()

	proposal, ok := p.proposalFor(env, view)
	if !ok {
		return
	}
	expanded := expandedBrief[proposal.CallID]
	decision, hasDecision := view.Decision(proposal.CallID)

	// 官方 ActionCard（OFF:Card.tsx:99-118）：这个 kind 注册了正文就**整段**交给它，
	// 框架只画那层 `<section class=card>`；没注册才画 FallbackBody。
	//
	// 先查注册表再画框：body 的存在性属于"这一帧画不画"，和卡片有没有数据同一层。
	if body := p.BodyFor(proposal.Action); body != nil {
		p.drawKindCard(f, env, t, body, proposal, decision, hasDecision, session,
			acting, actError, actResult, actFor == proposal.CallID)
		return
	}

	card := ui.Column(f).FillWidth().
		MarginSides(cardMarginY, 0, cardMarginY, 0).
		Background(t.BgLayer1).Radius(t.Radii.LG).
		Border(cardBorder, t.BorderL2).Clip()
	p.cardBoxes.Card = card
	card.Children(func() {
		body := ui.Column(f).FillWidth().
			PadSides(cardBodyPadTop, cardBodyPadRight, cardBodyPadBottom, cardBodyPadLeft)
		p.cardBoxes.Body = body
		body.Children(func() {
			// 标题（`.title`）。
			p.cardBoxes.Title = ui.Text(f, proposal.Title).
				FontSize(cardTitleFontSize).LineHeight(cardTitleLineH).Weight(cardTitleWeight).
				Color(t.LabelPrimary)
			p.drawBrief(f, env, t, proposal.CallID, proposal.Brief, expanded)
			p.drawStatuses(f, t, decision, hasDecision, actError, actResult, actFor == proposal.CallID)
			if !hasDecision {
				p.drawFooter(f, env, t, proposal.CallID, acting)
			}
		})
	})
}

// drawKindCard 画"注册了正文的 kind 卡片"：框架只给卡片框，正文由 body 画。
//
// 官方把 `<section class=card>` 留在 ActionCard 里、把 `.header` 与 `.body` 留给
// body（OFF:Card.tsx:99-118 + OFF:plugin-recommendations/Card.tsx:130-159），所以
// 这里**不加内边距**：header 在官方是通栏的（含左右 20 的内边距），由 body 自己给。
func (p *Plugin) drawKindCard(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, body pluginapi.ActionCardBody,
	proposal apiclient.ActionCardProposal, decision apiclient.ActionCardDecision, hasDecision bool,
	session string, acting bool, actError string, actResult apiclient.ActionCardResult, feedbackForThisCard bool) {
	card := ui.Column(f).FillWidth().
		MarginSides(cardMarginY, 0, cardMarginY, 0).
		Background(t.BgLayer1).Radius(t.Radii.LG).
		Border(cardBorder, t.BorderL2).Clip()
	p.cardBoxes.Card = card

	props := pluginapi.ActionCardBodyProps{
		SessionID: session,
		CallID:    proposal.CallID,
		Action:    proposal.Action,
		Title:     proposal.Title,
		Brief:     proposal.Brief,
		Details:   proposal.Details,
		Acting:    acting,
		ActError:  actError,
		// 动词落在**这张卡**的 callId 上：kind 的 enable 不许顺手落到别的卡。
		Act: func(verb string, payload map[string]json.RawMessage) {
			p.actVerb(proposal.CallID, verb, payload)
		},
	}
	if hasDecision {
		props.HasDecision = true
		props.Phase = decision.Phase
		props.Data = decision.Data
	}
	if feedbackForThisCard {
		info := &pluginapi.ActionCardResultInfo{Status: actResult.Status, Reason: actResult.Reason}
		if actResult.Decision != nil {
			info.Phase = actResult.Decision.Phase
		}
		props.LastResult = info
	}
	card.Children(func() { body.Draw(f, env, props) })
}

// smallGhostButton 画官方 Button variant="ghost" size="sm"。ButtonBase 让我们
// 使用官方的 12/18 常规字重、透明底、hover/active 令牌和 disabled opacity .4，
// 不继承 Viva 默认 primary/14px/500/dim .5 的另一套外观。
func (p *Plugin) smallGhostButton(f ui.C, t *pluginapi.Theme,
	label string, height, padX float32, disabled bool) ui.Box {
	active := ui.RGBA(38, 49, 72, 0.10) // design-platform.css:238（light）
	if t.Dark {
		active = ui.RGBA(255, 255, 255, 0.14) // design-platform.css:390（dark）
	}
	button := ui.ButtonBase(f, label).Height(height).PadX(padX).Radius(t.Radii.SM).
		Items(ui.Center).Justify(ui.Center).Self(ui.Start).
		Hover(ui.Look{}.Background(t.Hover)).Pressed(ui.Look{}.Background(active)).Disabled(disabled)
	if disabled {
		button = button.Opacity(smallButtonDisabledOpacity)
	}
	button.Children(func() {
		ui.Text(f, label).FontSize(smallButtonFontSize).LineHeight(smallButtonLineH).Color(t.LabelPrimary)
	})
	return button
}

// drawBrief 画 brief 与它的折叠开关（官方 FoldedText，OFF:Card.tsx:43-67）。
func (p *Plugin) drawBrief(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, callID, brief string, expanded bool) {
	preview, omitted := foldBrief(brief)
	shown := brief
	if omitted && !expanded {
		shown = preview
	}
	text := ui.Text(f, shown).MarginTop(cardTextMarginTop).
		FontSize(cardTextFontSize).LineHeight(cardTextLineH).Color(t.LabelSecondary)
	p.cardBoxes.Brief = text
	if !omitted {
		return
	}
	label := zhExpandBrief
	if expanded {
		label = zhCollapseBrief
	}
	// `.toggle` 覆盖 small Button 的 padding-inline=0、height:auto/min-height=24；
	// 两个中文标签都只有一行，因此实际高度就是 24，字号/行高仍为 12/18。
	toggle := p.smallGhostButton(f, t, label, toggleMinHeight, 0, false).
		MarginTop(toggleMarginTop).HitWith(p.hitID(env, "brief-toggle"), label, gfx.HitOptions{
		Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand,
	})
	p.cardBoxes.Toggle = toggle
	if toggle.Clicked() {
		p.mu.Lock()
		if p.expandedBrief == nil {
			p.expandedBrief = map[string]bool{}
		}
		p.expandedBrief[callID] = !p.expandedBrief[callID]
		p.mu.Unlock()
	}
}

// drawStatuses 画决定状态行与动作反馈行。
//
// 官方 `FallbackBody`（OFF:Card.tsx:118-150）的三种状态：
//
//	decision.phase ∈ {dismissed, replaced} → 对应 locale 文案
//	refused / unconfirmed                 → `unavailable`
//	回复丢失                              → `transport`
//
// 本仓在此基础上**按任务要求**把宿主给的 reason 原文也显示出来（官方不显示；
// 理由见包注释「没搬」与任务书 "Act 返回 refused → 显示宿主给的 reason 原文"）。
func (p *Plugin) drawStatuses(f ui.C, t *pluginapi.Theme, decision apiclient.ActionCardDecision, hasDecision bool,
	actError string, actResult apiclient.ActionCardResult, feedbackForThisCard bool) {
	lines := make([]string, 0, 3)
	if hasDecision {
		switch decision.Phase {
		case "dismissed":
			lines = append(lines, zhDismissed)
		case "replaced":
			lines = append(lines, zhReplaced)
		}
	}
	if feedbackForThisCard {
		if actError != "" {
			lines = append(lines, actError)
		} else {
			switch actResult.Status {
			case apiclient.ActionCardRefused, apiclient.ActionCardUnconfirmed:
				lines = append(lines, zhUnavailable)
				if actResult.Reason != "" {
					lines = append(lines, actResult.Reason)
				}
			}
		}
	}
	for _, line := range lines {
		box := ui.Text(f, line).MarginTop(statusMarginTop).
			FontSize(statusFontSize).LineHeight(statusLineH).Color(t.LabelSecondary)
		p.cardBoxes.Statuss = append(p.cardBoxes.Statuss, box)
	}
}

// drawFooter 画 footer（只有未决定时才有「暂不需要」按钮，官方 :146-148）。
//
// 按钮带**自己那张卡的 callId**：turnTail 按轮派发后同一帧可能有多张卡，
// "这一屏上那张卡"这种取法会把点击落到别的轮上。官方也是
// `act(proposal.callId, 'dismiss')`（OFF:Card.tsx:104 的 `send = useCallback((verb, payload) =>
// act(proposal.callId, verb, payload))`，`FallbackBody` 的按钮走 `send('dismiss')`）。
func (p *Plugin) drawFooter(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, callID string, acting bool) {
	footer := ui.Row(f).FillWidth().Items(ui.Center).Justify(ui.End).
		Gap(footerGap).MarginTop(footerMarginTop)
	p.cardBoxes.Footer = footer
	footer.Children(func() {
		button := p.smallGhostButton(f, t, zhDismiss, smallButtonHeight, smallButtonPadX, acting).
			HitWith(p.hitID(env, "dismiss"), zhDismiss, gfx.HitOptions{
				Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand, Disabled: acting,
			})
		p.cardBoxes.Dismiss = button
		if !acting && button.Clicked() {
			p.actVerb(callID, verbDismiss, nil)
		}
	})
}

// ── 绘制：过程行（官方 tool.call.toolview 座位）─────────────────────────────

// drawProposeRow 画 `propose_action` 调用的一行。**没有这次调用就一个节点都不画**。
func (p *Plugin) drawProposeRow(f ui.C, env *pluginapi.Env) {
	t := p.theme
	if t == nil {
		return
	}
	p.rowBoxes = rowBoxes{}
	if currentSource() == nil {
		return
	}
	p.mu.Lock()
	p.act = env.Act
	expanded := p.expandedRow
	p.mu.Unlock()

	call, ok := proposeCallOf(env.Data.Frames)
	if !ok {
		return
	}

	wrapper := ui.Column(f).FillWidth()
	wrapper.Children(func() {
		expandable := call.Reason != ""
		row := ui.Row(f).FillWidth().Items(ui.Center).Height(rowHeight).Gap(0).Color(t.LabelTertiary)
		if expandable {
			row = row.HitWith(p.hitID(env, "propose-row"), zhRowTitle, gfx.HitOptions{
				Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand, Expanded: expanded, Expandable: true,
			})
		} else {
			// 普通 div 也有 hover 色；RoleGroup 只提供悬停事实，不新增按钮/Tab 停靠。
			row = row.HitWith(p.hitID(env, "propose-row"), zhRowTitle,
				gfx.HitOptions{Role: gfx.RoleGroup, SkipTab: true})
		}
		row = row.Hover(ui.Look{}.Color(t.LabelSecondary))
		p.rowBoxes.Row = row
		row.Children(func() {
			// `.leading` 是 16×16、margin-right:6；内部 sprite 是 14×14。
			leading := ui.Row(f).Size(rowLeadingSize, rowLeadingSize).
				MarginRight(rowLeadingMarginRight).Items(ui.Center).Justify(ui.Center)
			p.rowBoxes.Icon = leading
			leading.Children(func() { drawProposeIcon(f, row, t) })
			// 行首标题：DisclosureRow `.title` 是 13/24、tertiary，hover 变 secondary。
			p.rowBoxes.Title = ui.Text(f, zhRowTitle).
				FontSize(rowTitleFontSize).LineHeight(rowTitleLineH).Color(t.LabelTertiary).
				StateFrom(row).Hover(ui.Look{}.Color(t.LabelSecondary))
			summary := ui.Row(f).Items(ui.Center).Gap(rowSummaryGap).MarginLeft(rowSummaryMarginLeft).Grow(1)
			summary.Children(func() {
				// `.summary{color:inherit}`：状态与卡片标题都继承行的 tertiary/secondary，
				// 不是一个 secondary、一个 primary（ProposeRow.module.css:2）。
				p.rowBoxes.State = ui.Text(f, proposeStateText(call.State)).
					FontSize(rowSummaryFontSize).Color(t.LabelTertiary).
					StateFrom(row).Hover(ui.Look{}.Color(t.LabelSecondary))
				p.rowBoxes.CardTitle = ui.Text(f, call.Title).
					FontSize(rowSummaryFontSize).Color(t.LabelTertiary).Grow(1).Ellipsis().
					StateFrom(row).Hover(ui.Look{}.Color(t.LabelSecondary))
			})
		})
		if expandable && row.Clicked() {
			p.mu.Lock()
			p.expandedRow = !p.expandedRow
			// 同一帧生效（React 里 setState 之后组件立刻重渲染，展开的那一帧就该有正文）。
			expanded = p.expandedRow
			p.mu.Unlock()
		}
		if expandable && expanded {
			// `.reason`：宿主结果原文，pre-wrap 语义（多行原样显示）。
			p.rowBoxes.Reason = ui.Text(f, call.Reason).
				MarginSides(reasonMarginY, 0, reasonMarginY, 0).
				Pad(reasonPad).Radius(t.Radii.LG).Background(t.BgLayer1).
				FontSize(reasonFontSize).Color(t.LabelSecondary)
		}
		// 「查看调用」（`row.inspect`）需要壳提供轨迹定位能力，`pluginapi.Actions`
		// 没有这一格 → 不画假按钮（`zhRowInspect` / `inspectPadY` / `inspectFontSize`
		// 三个官方值保留为证据，见包注释「没搬」）。
	})
}

// drawProposeIcon 复用 rc.2 官方 apps-add-outline sprite：一条 path + 三枚
// 圆角矩形，原始几何由 dsassets.SidebarAppsAdd 提供；这里只做 16→14 的等比缩放。
func drawProposeIcon(f ui.C, row ui.Box, t *pluginapi.Theme) {
	icon, rects, ok := dsassets.SidebarAppsAdd(t.LabelTertiary)
	if !ok {
		return
	}
	ui.Icon(f, icon).Size(rowIconSize, rowIconSize).Color(t.LabelTertiary).
		StateFrom(row).Hover(ui.Look{}.Color(t.LabelSecondary))
	scale := rowIconSize / 16
	inset := (rowLeadingSize - rowIconSize) / 2
	for _, rect := range rects {
		ui.Row(f).Absolute().
			Left(inset+rect.X*scale).Top(inset+rect.Y*scale).
			Width(rect.Width*scale).Height(rect.Height*scale).
			Radius(rect.Radius*scale).Border(rect.Stroke*scale, t.LabelTertiary).
			StateFrom(row).Hover(ui.Look{}.Border(t.LabelSecondary))
	}
}

// ── 过程行的数据：从 journal 原始帧折叠 ─────────────────────────────────────

// 官方的五个行状态（OFF:ProposeRow.tsx:11 的 `RowState`）。
const (
	rowPreparing = "preparing"
	rowRunning   = "running"
	rowOK        = "ok"
	rowStopped   = "stopped"
	rowError     = "error"
)

// proposeStateText 是 `t(\`row.${state}\`)` 的等价物（OFF:ProposeRow.tsx:38）。
//
// 未知状态回落到 `row.error`（官方 stateOf 的兜底也是 error 分支）。
func proposeStateText(state string) string {
	switch state {
	case rowPreparing:
		return zhRowPreparing
	case rowRunning:
		return zhRowRunning
	case rowOK:
		return zhRowOK
	case rowStopped:
		return zhRowStopped
	default:
		return zhRowError
	}
}

// proposeCall 是一行过程行需要的全部数据（官方 ToolCallViewProps 里本行真用到的那几个）。
type proposeCall struct {
	CallID string
	// Title 来自调用参数里的 `title`（官方 `block.args.text('title') ?? ''`，:30）。
	Title string
	State string
	// Reason 是失败结果的正文（官方 `block.content` 里的 text 拼起来，:31-33）。
	Reason string
}

// proposeCallOf 从 journal 原始帧里折叠"最近一次 `propose_action` 调用"。
//
// 数据来源是 Transport 的原始帧（`Env.Data.Frames`），不是宿主私有日志——
// 与官方从 `ToolCallViewProps.phase/block` 取数是同一份事实：
//
//	tool/call{name=propose_action, callId, arguments}   → preparing（参数还没到齐）| running
//	tool/result{message:{toolCallId,isError,content}}   → ok | error | stopped
//
// `stopped` 的判据照官方 `props.block.error?.code === 'interrupted'`（:16）；
// 本仓宿主目前不在 tool/result 上发小写 `interrupted` 码（见包注释），
// 所以这条分支暂时只能由"码恰好是 interrupted"的帧命中——不猜别的码。
func proposeCallOf(frames []apiclient.Frame) (proposeCall, bool) {
	var (
		call    proposeCall
		found   bool
		callIdx = -1
	)
	for i := range frames {
		frame := frames[i]
		switch frame.Type {
		case "tool/call":
			var data struct {
				CallID    string `json:"callId"`
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}
			if json.Unmarshal(frame.Data, &data) != nil || data.Name != proposeActionName || data.CallID == "" {
				continue
			}
			call = proposeCall{CallID: data.CallID, Title: toolArgumentTitle(data.Arguments), State: rowPreparing}
			if strings.TrimSpace(data.Arguments) != "" {
				call.State = rowRunning
			}
			found, callIdx = true, i
		case "tool/result":
			if !found || i <= callIdx {
				continue
			}
			var data struct {
				Message *struct {
					ToolCallID string `json:"toolCallId"`
					IsError    bool   `json:"isError"`
					Content    []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				} `json:"message"`
				Error *struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if json.Unmarshal(frame.Data, &data) != nil || data.Message == nil ||
				data.Message.ToolCallID != call.CallID {
				continue
			}
			if !data.Message.IsError {
				call.State, call.Reason = rowOK, ""
				continue
			}
			call.Reason = resultText(data.Message.Content)
			call.State = rowError
			if data.Error != nil && strings.EqualFold(data.Error.Code, "interrupted") {
				call.State = rowStopped
			}
		}
	}
	return call, found
}

// toolArgumentTitle 取调用参数里的 `title`；参数不是合法 JSON 时返回空串
// （官方 `block.args.text('title') ?? ”` 同样不抛）。
func toolArgumentTitle(arguments string) string {
	var args struct {
		Title string `json:"title"`
	}
	if json.Unmarshal([]byte(arguments), &args) != nil {
		return ""
	}
	return args.Title
}

// resultText 把结果内容里的 text 块拼起来（官方 `content.flatMap(...).join('\n')`，:31-33）。
func resultText(content []struct {
	Type string `json:"type"`
	Text string `json:"text"`
}) string {
	parts := make([]string, 0, len(content))
	for _, item := range content {
		if item.Type == "text" {
			parts = append(parts, item.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// ── brief 折叠（官方 FoldedText，OFF:Card.tsx:43-67）────────────────────────

// foldBrief 把 brief 折成预览：先压平空白，再按"拉丁字母宽"累计到 260 就截断加 `…`。
//
// 宽度规则照官方：CJK（Han/Hiragana/Katakana/Hangul）、CJK 标点与全角形式算 2，
// 其余算 1（OFF:Card.tsx:38-41 的 WIDE_GRAPHEME）。**原文一个字都不改写**，
// 只有"要不要折叠"和预览的截断位置。
func foldBrief(text string) (preview string, omitted bool) {
	compact := strings.Join(strings.Fields(strings.ReplaceAll(text, "\u00a0", " ")), " ")
	if compact == "" {
		return "", false
	}
	width := 0
	for i, r := range compact {
		if wideRune(r) {
			width += 2
		} else {
			width++
		}
		if width > foldWidth {
			return strings.TrimRight(compact[:i], " ") + "…", true
		}
	}
	return compact, false
}

// wideRune 报告一个 rune 是不是"画出来约两倍宽"的字（官方 WIDE_GRAPHEME 的 rune 等价物）。
func wideRune(r rune) bool {
	switch {
	case r >= 0x3000 && r <= 0x303F: // CJK 标点
		return true
	case r >= 0xFF00 && r <= 0xFFEF: // 全角形式
		return true
	case r >= 0x3040 && r <= 0x30FF: // 平假名 / 片假名
		return true
	case r >= 0x4E00 && r <= 0x9FFF: // Han（基本区）
		return true
	case r >= 0x3400 && r <= 0x4DBF: // Han 扩展 A
		return true
	case r >= 0xAC00 && r <= 0xD7AF: // Hangul 音节
		return true
	case r >= 0x1100 && r <= 0x11FF: // Hangul Jamo
		return true
	case r == 0xA0:
		return false
	}
	return false
}

// hitID 造交互 id（不许写死数字：环境里有 Tree 时用 Tree.ID）。
//
// 没有 Tree（单测直接画一眼）时返回零值 ID：零值 = "没有目标"，
// `Clicked()` 恒 false —— 那是**正确的降级**，不是伪造一个会响的 id。
func (p *Plugin) hitID(env *pluginapi.Env, name string) gfx.ID {
	if env == nil || env.Tree == nil {
		return gfx.ID{}
	}
	// 按轮座位同一帧会画多张卡：交互 id 必须把轮次带上，否则第 1 轮与第 3 轮的
	// 「展开」「暂不需要」共用一个 id——Viva 的交互态按 id 存，两张卡会互相顶掉。
	if env.Turn != nil {
		name = name + "-turn" + strconv.FormatInt(env.Turn.Turn, 10)
	}
	return env.Tree.ID(env.Feat, name)
}
