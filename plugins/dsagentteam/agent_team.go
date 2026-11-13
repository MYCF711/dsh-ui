// Package dsagentteam 复刻官方 rc.2 新增包 `@deepseek-ai/dsh-client-ui-agent-team`
// （**4 个槽位**，不是 rc.1 实验包那个 500px 锚定弹层）。
//
// 官方事实（`structure/official-agent-team-tree.md`，rc.2 src 原件行号）：
//
//	① conversation.session.header.actions.subagent.footer  id=agent-team  TeamBoard
//	② conversation.input.model.options                      id=agent-team  TeamModeToggle
//	③ shell.overlay                                         id=agent-team.disable  TeamDisableConfirmation
//	④ conversation.chat.commandview                         key=team   TeamCommandRow
//
// 服务依赖 `['slots','locale']`；②③ 还要 `remote.agentTeams`（唯一远端动作
// `agentTeams.setEnabled(sessionId, enabled)` → `ok ? null : error.message`）。
//
// # rc.1 → rc.2 是**重做**，不是改名（结构树 §7）
//
// rc.1 的实验包自己画面板（`width:min(500px,100vw-32px)` + 锚定定位 + 名册网格）；
// rc.2 把富面板拆成四个官方扩展点，**面板容器 / 滚动 / 键盘交给 ui-subagent**。
// 所以本包**禁止**自画弹层：任务板只是子代理目录 footer 里的一段（`section.board`）。
//
// # 哪些"可见"由父级决定（照抄官方，不自己判）
//
//	① 子代理目录 footer 是否渲染（官方 ui-subagent 的 footerAvailable）
//	② 模型菜单是否打开（父级 slot 环境）
//	③ shell.overlay 一直存在，靠 request===null 自己守
//	④ chat 的命令节点渲染器决定"这条命令有没有 commandview"
//
// 本包只负责"塞进去之后画什么"。
//
// # 本仓落地方式（先落槽位，再落内容）
//
// 四个 painter 全部注册在官方槽位名上（`pluginapi.Slot` 是写死常量的闭集，
// 新槽位用字符串转换先落——报告里列了"需要 GA 补哪个类型"）。
// 数据面是本包的 `Store`：壳/父级把投影（`agentTeam` project、mode、
// 是否子会话、modelSelection.lastUsed）与命令节点喂进来；远端写走
// `agentTeams.setEnabled`（`*apiclient.Client` 直接满足 Remote）。
package dsagentteam

import (
	"sync"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/ui"
)

// 官方槽位名（逐字）。①②④ 是 rc.2 新造的契约面；③ 是既有 `shell.overlay`，
// 官方用 `id: 'agent-team.disable'` 往里加一块画，所以这里复用 `pluginapi.SlotShellOverlay`。
const (
	// SlotSubagentFooter 官方 `conversation.session.header.actions.subagent.footer`
	// （声明方 ui-subagent：`ui-subagent/src/client/index.ts:105`；消费方本包 mount.ts:68-70）。
	SlotSubagentFooter = pluginapi.Slot("conversation.session.header.actions.subagent.footer")
	// SlotInputModelOptions 官方 `conversation.input.model.options`
	// （声明方 ui-model-selection：`index.ts:177`；消费方本包 mount.ts:57-65）。
	SlotInputModelOptions = pluginapi.Slot("conversation.input.model.options")
	// SlotChatCommandView 官方 `conversation.chat.commandview`（key `team`）。
	SlotChatCommandView = pluginapi.Slot("conversation.chat.commandview")
	// SlotDisableOverlay 是停用确认的落点：官方挂 `shell.overlay`，**不是**菜单里。
	SlotDisableOverlay = pluginapi.SlotShellOverlay
)

// 官方 slot 的 id / key（写进日志与命中区 id 前缀，便于对照官方）。
const (
	// SlotIDBoard 是 ① 的官方 `id: 'agent-team'`。
	SlotIDBoard = "agent-team"
	// SlotIDToggle 是 ② 的官方 `id: 'agent-team'`。
	SlotIDToggle = "agent-team"
	// SlotIDDisable 是 ③ 的官方 `id: 'agent-team.disable'`。
	SlotIDDisable = "agent-team.disable"
	// CommandKey 是 ④ 的官方 `key: 'team'`。
	CommandKey = "team"
)

// ServiceKey 是本包提供的运行时（值是 *Store）。
//
// 壳/父级用它喂数据（SetView / SetCommand）与驱动动作（Select / ConfirmDisable）。
const ServiceKey = "ui.agent-team"

// RemoteKey 是可选注入：远端实现（*apiclient.Client 直接满足 Remote）。
const RemoteKey = "ui.apiclient"

// Plugin 是 `ds-agent-team` 插件。
type Plugin struct {
	theme *pluginapi.Theme
	team  *Store

	// mu 保护运行期才接上的可选远端（provider 目录）。
	mu sync.Mutex
	// providers 是 `llm.listConfigurableProviders` 的远端（容器 C 的数据面）。
	providers ProvidersRemote
	// catalog 是子代理目录父容器（GC 的容器 B）的界面态。
	catalog catalogUI
	// model 是官方 `conversation.input.model` 座位（本包占用的模型 trigger）的
	// 跨帧几何：上一帧 trigger 矩形，供菜单锚点（见 model_slot.go）。
	model modelSlotState
	// toggleValue 是 Team 开关这一帧的绑定值：v0.6.0 的 `ui.Switch` 绑 `*bool`，
	// 点击由控件写回；真值仍然来自投影（非乐观更新，见 toggle.go 文件头），每帧先重置。
	toggleValue bool
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-agent-team" }

// Provides 提供 Team 运行时。
func (*Plugin) Provides() []string { return []string{ServiceKey} }

// Requires 只要主题令牌；数据面可选（缺了就是"投影未知"→ 四块都不画）。
func (*Plugin) Requires() []string { return []string{pluginapi.KeyTheme} }

// Register 注入主题与可选远端，并把运行时提供给下游。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	// 三个 rc.2 新槽位登记进官方槽表（shell.overlay 是既有槽，不重复登记）。
	if err := RegisterOfficialSlots(); err != nil {
		return err
	}
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	if v := ctx.Inject(ServiceKey); v != nil {
		if store, ok := v.(*Store); ok {
			p.team = store
		}
	}
	if p.team == nil {
		p.team = NewStore(nil)
	}
	if v := ctx.Inject(RemoteKey); v != nil {
		if remote, ok := v.(Remote); ok {
			p.team.SetRemote(remote)
		}
		// 同一个注入值（*apiclient.Client）也满足容器 C 的 provider 目录远端。
		// 缺它时菜单整段隐藏（空态），不影响装配。
		if providers, ok := v.(ProvidersRemote); ok {
			p.SetProvidersRemote(providers)
			// 起手拉一次目录：官方 trigger 在首屏就该知道有没有模型可配。
			p.team.RefreshProviders()
		}
	}
	ctx.Provide(ServiceKey, p.team)
	return nil
}

// Panels 实现 pluginapi.Panels：**四个官方槽位各一块画**。
//
// Order 都是 0：官方每个槽里本包都是唯一一块（`id` 不同），顺序无意义，
// 但必须稳定（同一份插件集合每次画出来一样）。
func (p *Plugin) Panels() []pluginapi.Painter {
	// 四个官方槽位各自的贡献（panel ①-④）。
	base := []pluginapi.Painter{
		{ID: "agent-team.board", Slot: SlotSubagentFooter, Order: 0, Draw: p.drawBoard},
		{ID: "agent-team.toggle", Slot: SlotInputModelOptions, Order: 0, Draw: p.drawToggle},
		{ID: "agent-team.disable", Slot: SlotDisableOverlay, Order: 0, Draw: p.drawConfirm},
		{ID: "agent-team.command", Slot: SlotChatCommandView, Order: 0, Draw: p.drawCommand},
	}
	// **父容器**（本单新增）：模型菜单（②的父级）与 chat 命令节点渲染器（④的父级）。
	// 它们不是"槽位贡献"，是"槽位父级里那一次 drawSlot"——没有它们，②④ 永远画不出来。
	base = append(base, p.containerPanels()...)
	// 子代理目录（GC 的父容器 B）：触发器 / 面板 / 旧三栏过渡桥 3 条。
	base = append(base, p.catalogPanels()...)
	return base
}

// Store 返回本插件的运行时（测试与父级用）。
func (p *Plugin) Store() *Store { return p.team }

// themeFor 按这一帧的主题取令牌（壳换主题时插件不用重新注册）。
func (p *Plugin) themeFor(env *pluginapi.Env) *pluginapi.Theme {
	if env != nil && env.Theme != nil {
		return env.Theme
	}
	return p.theme
}

// drawAll 只是给测试/调试用的便捷入口：在同一个 frame 上按四个槽位各画一次。
//
// 生产路径永远由壳按槽位分派（`uikernel.DrawSlot`），这里不参与装配。
func (p *Plugin) drawAll(f ui.C, env *pluginapi.Env) {
	p.drawBoard(f, env)
}
