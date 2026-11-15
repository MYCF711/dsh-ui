// Package dscordis 把官方 `@deepseek-ai/dsh-client-ui-cordis` 的**面板**落进本仓界面
// （GUI 账本 #12）。这是本仓第一次接这条包。
//
// # 座位表（官方注册行号 → 本仓 painter）
//
//	官方座位                     官方注册 / 画法                                 本仓 painter
//	──────────────────────────  ─────────────────────────────────────────────  ────────────────────────────────
//	sidebar.footer.action        OFF:index.ts:86-114（id 'cordis-panel'，list/root）  cordis-trigger
//	  （list 槽）= 触发器        组件 CordisPanel.tsx:485-504 的 `.badge`          Slot=uikernel.OffSidebarFooterActions
//	                             Order 无（官方只有一个注册者）                    Order=60（本仓槽内互斥要唯一）
//	shell.overlay（面板本体）    官方是 `position:fixed` 的 `<section>`             cordis-panel
//	                            CordisPanel.tsx:459-484                           Slot=pluginapi.SlotShellOverlay
//	                            + useDismissOnOutsidePointer（:153）              Order=46
//	                                                                              Draw=全窗遮罩 / Children=面板
//
// 官方这两个面是**同一个** React 组件里的两段（`open` 决定的 `<section>` 与 `.badge`）。
// Viva 一个槽只能挂一块画，所以拆成两块，形状照本仓既有先例
// `plugins/dsagentteam/catalog.go:206-437`（`drawCatalogBackdrop` / `drawCatalogPanel`）：
// `shell.overlay` 的 Draw 画全窗命中区（官方 document 级 pointerdown 的等价物），
// Children 画面板盒子。viva 没有 portal，`shell.overlay` 就是本仓
// `createPortal(…)` 的等价物——子代理目录（dsagentteam）与后台任务弹层（dsjobs）
// 都是这么落的。
//
// # 数据来源
//
// 面板是**帧内全局**的：它不按会话过滤，读的是一次全局 inventory
// （OFF:index.ts:5-10 的注释写明 panel 是 frame-wide surface）。本仓把它挂在
// `Snapshot.Cordis`（`pluginapi.CordisView`，字段与语义见 `pluginapi/cordis.go` 文件头）：
// 壳在每帧组装快照时**同步给出缓存**，插件在 Draw 里不发请求、不等 HTTP。
//
// 三个生命周期动作走 `pluginapi.Actions`：`CordisRun` / `CordisStop` / `CordisRemove`
// （官方面板上的三个 `RowAction`，OFF:CordisPanel.tsx:326-396）。仓规：这三条
// **同步阻塞、只能在该插件起的 goroutine 里调**（见本文件 `fireAction`）；
// 成功后壳自己重读清单并重绘，插件**不改本地状态、不重试**。
//
// # HIDE 清单（每一条都是"没有真实能力"，不是"还没做"）
//
// HIDE-1 `tool.view.cordis` 业务视图（Package 自己的 React 组件）。
//
//	官方把它塞进 `cordis_run` 卡片的 keyed 子槽（OFF:CordisRunRow.tsx:119-130，
//	`children: { 'tool.view.cordis': { kind: 'keyed', scope: 'session' } }`
//	见 OFF:index.ts:128），画的是那枚 Package 里 `client.js` 导出的 React 组件。
//	Go+Viva 没有 JS 组件宿主 → 整块 HIDE。
//
// HIDE-2 四张 `cordis_*` 历史工具卡（cordis_define / cordis_run / cordis_stop /
// cordis_undefine；官方 OFF:index.ts:117-145 的四个 `tool.call.toolview` keyed 座位）。
//
//	它们的**唯一数据源是已记录的 tool/call + tool/result 帧**
//	（OFF:card-model.ts:95-160 全部从 `block.argsRaw` / `block.result` 派生），
//	而本宿主注册 **0 个 `cordis_*` 工具**（HOST:docs/host-stable-gap-register.md:412
//	「4 个 cordis_* 工具：注册 0 个」），官方 rc.2 自己也不注册
//	（`dsh-cordis-host-runner/README.md:119`「This package registers no tool or prompt」）。
//	没有生产者 → 那些帧永远不会出现 → 画出来是一块**永远空着的座位**，所以不注册 painter。
//
// HIDE-3 Client 半的加载与结算（官方 `getClientCode` + 页面加载后的 `settleUserRun`）。
//
//	没有 JS 宿主就没有"页面加载回执"；调 settle 就是**编造一次成功**。
//	连带后果（如实保留在派生里）：`loadedClientHalves()` 恒为空集，所以有 Client 半的包
//	在跑时状态是 `client-pending`（`Client 待激活`）而不是 `running`（`运行中`）——
//	那是官方语义的直接结果，不是显示 bug，**不许为了好看改成 running**。
//
// HIDE-4 审批（`cordis/request-run` 的 approve / decline 两个按钮 + `awaiting-approval` 态）。
//
//	宿主没有 Cordis 审批队列，这两条事件不发（`pluginapi/plugin.go` 的
//	`CordisRun` 注释：`requestId` / `approveFutureVersions` 在本仓恒 null / false）。
//	官方面板那三个审批按钮（OFF:CordisPanel.tsx:289-325）不画。
//	状态派生里保留 `awaiting-approval` 那一档（文案用官方原文），因为官方判据顺序
//	里它在最前面——哪天宿主真发了这条帧，界面要说"待审批"而不是"待激活"。
//
// HIDE-5 面板「运行」按钮对**有 Client 半的包**。
//
//	官方 onRun 会连带加载 Client 半并结算（见 HIDE-3）；本仓只能起 Host 半，
//	起了就永远停在 client-pending。所以运行按钮**只对 `HasClientHalf == false`
//	的包画**（见 `drawRowActions` 的 `canRun`），有 Client 半的行只给停止 / 移除。
//	理由：点了没结果的按钮比不画更坏。
//
// HIDE-6 `renderFailures` 那一行（官方 `.rowError` 的渲染失败分支，OFF:CordisPanel.tsx:438-449）。
//
//	本页不加载动态组件（HIDE-1），不会有渲染失败，那条数据的生产者（client-runner 的
//	`reportRenderFailure`）本仓根本不调（`internal/apiclient/cordis.go:43-51` 的注释）。
//
// # 照实登记的差异（不是 HIDE，是"能画但不是逐像素同形"）
//
//  1. **0 行时官方连面板一起不画**（`if (all.length === 0) return null`，
//     OFF:CordisPanel.tsx:192）。本仓把这条判据只用在**触发器**上；面板在 `open`
//     时仍然画。理由：`panel.loading`（读取中）/ `panel.empty`（还没有定义任何插件）
//     是官方自己的文案，落在 `<section>` 里；若连面板一起返回，这两句永远画不出来。
//     官方靠"触发器不出现 → 面板打不开"来保证不矛盾，本仓多留了这一条出口，
//     让"读取中"能如实显示成读取中（测试 3 钉住这个判据）。
//  2. 触发器图标（官方 `IconCordisPluginOutlineRegular size={16}`，OFF:CordisPanel.tsx:496）：
//     `plugins/dsassets` 的素材登记表里没有这枚 sprite，按 `plugins/dsagentteam` 的已知做法
//     **只留 16×16 槽宽、不手画 SVG**（geometry.go 的 cordisTriggerIcon）。
//  3. 三个动作按钮的图标（官方 `IconPlayOutlineRegular` / `IconStopFillRegular` /
//     `IconTrashOutlineRegular`，size=14）：同上，**只留 14×14 槽宽**。
//     按钮本体（28×28 圆 + 悬停底色 + 官方 `action.*` 可访问名）照官方画。
//  4. 状态点：官方 `StateDot` 的 ongoing 分支是 14px 的旋转 spinner
//     （`plugins/dsconversation/official_transcript_geometry.go:925-931`），
//     静态画布没有时间轴 → 统一画 10px 实心圆、ongoing 用品牌色
//     （与 `plugins/dsagentteam/board.go:186-190` 的 StateDot 等价物同一做法）。
//  5. `.group` 的 `text-transform:uppercase;letter-spacing:0.04em`（CSS:159-160）不实现：
//     本仓唯一语言是中文，uppercase 对它无效果，letter-spacing viva 没有对应能力。
//  6. **版本选择器**（`packages.length > 1`，OFF:CordisPanel.tsx:267-285）、
//     **transition 块**（`nextPackageId != currentPackageId`，:399-430）、
//     **`.activeVersion`**（:450-452）三条：判据条件照官方写在代码里，
//     但本宿主恒 1 个 Package、不产生 nextPackageId（`internal/apiclient/cordis.go:121-129`），
//     所以**永不出现**。这里不画一个没有真实选择动作的下拉框，也不给假状态。
//  7. 官方 `blockingFirst`（OFF:CordisPanel.tsx:88-93，把待审批行提到组首）不实现：
//     它读的是 client-runner 的 `activity.phase`（HIDE-3），本仓那份数据不存在，
//     这一步在这是恒等变换——写了就是死代码。
//  8. 面板锚点用**上一帧**触发器的真实矩形（`afterTrigger` → `panelAnchor`）：
//     viva 的 `Box.Rect()` 要等布局解算之后才有效，同帧 Draw 读不到
//     （`plugins/dsagentteam/model_slot.go:196-215` 同一做法）。首帧兜底值见 geometry.go。
//  9. `.badge{width:calc(100% + 4px);margin:0 -2px}`（CSS:23-25）那 2px 外溢不实现：
//     它是为了让 42px 的行在侧栏里贴边，viva 的负 margin 会改变父容器的可用宽度，
//     在这里收益为 0、风险是撑破侧栏。
//  10. 官方的 `data-cordis-*` 属性（`data-cordis-badge` / `data-cordis-row` /
//     `data-cordis-switch` / `data-cordis-remove`）无法在 viva 上表达（没有 data attribute），
//     等价的身份改由命中区 id 承载（`panel.go` 顶部的 `hit*` 常量，命名与官方属性对齐）。
//  11. 官方 `.rail` 窄态（侧栏折叠成 36×36 圆钮，CSS:63-82）不画：本仓侧栏底部是宽态
//     （`uikernel` 的 `footArea.actions` 给的是整行宽），没有"折叠成 rail"这个档。
//  12. 官方在发现新的待审批请求时会自动展开面板（OFF:CordisPanel.tsx:155-163）：
//     本宿主不发 `cordis/request-run`（HIDE-4），这段没有输入，不实现。
//  13. 官方 `useEffect(() => { if (open) onRefresh() })`（:165-166）——面板每次打开都重读清单：
//     本仓在 `toggleOpen` 打开成功那一刻调一次 `Actions.CordisRefresh`（fire-and-forget，
//     `pluginapi/plugin.go` 的 `CordisRefresh` 注释），之后交给事件帧与动作后的壳重读，
//     不在每帧 Draw 里重复触发。
package dscordis

import (
	"sync"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// Plugin 是 `ds-cordis` 插件（官方 `dsh-client-ui-cordis` 的面板）。
//
// # 它自己持有什么（**只有界面态，没有数据**）
//
// 官方把这些放在 React 的 `useState` 里（OFF:CordisPanel.tsx:130-136）：
//
//	open     面板是否展开                     官方 `const [open, setOpen] = useState(false)`
//	pending  每个插件"动作在飞"               官方 `const [pending, setPending] = useState(new Set())`
//	actionErrs 每个插件最近一次动作的错误原文   官方 `const [actionErrors, setActionErrors] = useState(new Map())`
//
// 三样都是**界面态**：清单（rows / known / error）来自 `Snapshot.Cordis`，
// 一次都不在插件里缓存——官方也是这个分工（inventory 属于 apply 闭包，见 OFF:inventory.ts:1-18）。
//
// 另外两个字段是 viva 特有的过渡量，不是数据：
//
//	triggerBox / triggerRect  上一帧触发器盒子与它的真实矩形，给面板做锚点（差异表第 8 条）。
type Plugin struct {
	// theme 是 Register 时注入的主题令牌（`Requires` 只有 KeyTheme）。
	theme *pluginapi.Theme

	// mu 保护下面这些**既被 Draw 读、又被 goroutine 写**的状态。
	// 绘制与动作 goroutine 可能来自不同线程（`plugins/dsjobs/jobs.go` 的同款注释）。
	mu sync.Mutex
	// open 是面板是否展开（官方 `open`，OFF:CordisPanel.tsx:130）。
	open bool
	// pending 是"这些插件的动作正在进行中"（官方 `pending`，:132）。
	pending map[string]bool
	// actionErrs 是每个插件最近一次动作的失败原文（官方 `actionErrors`，:133）。
	actionErrs map[string]string
	// triggerBox 是本帧 Draw 阶段量到的触发器等盒；`Rect()` 要等布局解算之后才有效，
	// 所以由 `AfterDraw`（`afterTrigger`）读走（`plugins/dsagentteam/model_slot.go`:92-95 同款）。
	triggerBox ui.Box
	// triggerRect 是上一帧触发器的真实矩形（面板锚点）。
	triggerRect ui.Rect
}

// ID 实现 pluginapi.Plugin。名字对齐官方包名（`dsh-client-ui-cordis` → `ds-cordis`）。
func (*Plugin) ID() string { return "ds-cordis" }

// Provides 不提供任何服务：面板只读快照 + 调 Actions，没有下游要用的运行时。
func (*Plugin) Provides() []string { return nil }

// Requires 只要主题令牌。
//
// 数据面（`Snapshot.Cordis`）与三个动作（`Actions.Cordis*`）都**不可 Require**：
// 壳没接这条能力时它们就是 nil，面板整块不画（`Data.Cordis == nil`）或对应按钮不画
// （`CordisRun/Stop/Remove == nil`）——那是本仓对"能力缺席"的既有表达方式，
// 而不是让整棵装配树失败。
func (*Plugin) Requires() []string { return []string{pluginapi.KeyTheme} }

// Register 注入主题。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	return nil
}

// Panels 返回本包的两块画。
//
// Order 的两个数字：侧栏底部动作带 60、整窗浮层 46。两个都是本仓确认空闲的档位
// （`cmd/app/panels_order_test.go` 会对"同槽同 Order"报红；overlay 已用
// 10/40/45/50/95-104/900，footer action 此前无人占）。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		// 触发器：官方 `sidebar.footer.action`（OFF:index.ts:86-114）。
		// AfterDraw 只做一件事：把这一帧的盒子矩形缓存给下一帧的面板锚点。
		{
			ID:        "cordis-trigger",
			Slot:      uikernel.OffSidebarFooterActions,
			Order:     60,
			Draw:      p.drawTrigger,
			AfterDraw: p.afterTrigger,
		},
		// 面板：官方 `createPortal(<section className={css.panel}/>)` 的等价物
		// = 整窗浮层。Draw 画遮罩（点外面收起），Children 画面板本体。
		{
			ID:       "cordis-panel",
			Slot:     pluginapi.SlotShellOverlay,
			Order:    46,
			Draw:     p.drawBackdrop,
			Children: p.drawPanel,
		},
	}
}

// themeFor 取本帧主题：`env.Theme` 优先（壳每帧给的），否则用 Register 注入的那份。
// 单测里可以不装配壳，两条路都通（`plugins/dsagentteam/agent_team.go:163-168` 同款）。
func (p *Plugin) themeFor(env *pluginapi.Env) *pluginapi.Theme {
	if env != nil && env.Theme != nil {
		return env.Theme
	}
	return p.theme
}

// ── 面板开合 ─────────────────────────────────────────────────────────────

// isOpen 报告面板是否展开。
func (p *Plugin) isOpen() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.open
}

// setOpen 直接把开合设成指定值（壳与测试的无鼠标入口用，与点击等价）。
func (p *Plugin) setOpen(open bool) {
	p.mu.Lock()
	p.open = open
	p.mu.Unlock()
}

// Open 打开面板。给壳/测试的无鼠标入口（`plugins/dsagentteam/catalog.go:181-188`
// 的 `OpenCatalog` 同款语义：与点一次触发器等价，只是不经鼠标）。
//
// 它**不发** `CordisRefresh`：那条动作需要 `env.Act`，只有 Draw 里才有；
// 真实路径（点触发器）一定经过 `drawTrigger`，刷新在那里调。
func (p *Plugin) Open() { p.setOpen(true) }

// Close 收起面板（与点一次遮罩等价）。
func (p *Plugin) Close() { p.setOpen(false) }

// toggleOpen 翻转开合，并在"变成展开"时触发一次后台清单重读。
//
// 官方 `useEffect(() => { if (open) onRefresh() }, [onRefresh, open])`
// （OFF:CordisPanel.tsx:165-166）：面板一打开就重读一次，避免看到点开前的旧值。
// 本仓这一步只调 `Actions.CordisRefresh`（fire-and-forget：壳拉完自己 Invalidate，
// 插件不等结果、也不改本地状态）。
func (p *Plugin) toggleOpen(act ...pluginapi.Actions) {
	p.mu.Lock()
	p.open = !p.open
	open := p.open
	p.mu.Unlock()
	if !open {
		return
	}
	if len(act) > 0 && act[0].CordisRefresh != nil {
		act[0].CordisRefresh()
	}
}

// ── 动作在飞 / 动作错误 ───────────────────────────────────────────────────

// isPending 报告这个插件的动作是不是还在飞（官方 `pending.has(pluginId)`，:240）。
func (p *Plugin) isPending(pluginID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pending[pluginID]
}

// setPending 记下/清掉"动作在飞"。
func (p *Plugin) setPending(pluginID string, pending bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pending == nil {
		p.pending = map[string]bool{}
	}
	if pending {
		p.pending[pluginID] = true
		return
	}
	delete(p.pending, pluginID)
}

// actionErrorOf 取这个插件最近一次动作的失败原文（没有就是空串）。
func (p *Plugin) actionErrorOf(pluginID string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.actionErrs[pluginID]
}

// setActionError 记下一次动作失败（官方 `actionErrors.set(pluginId, message)`，:205-211）。
func (p *Plugin) setActionError(pluginID, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.actionErrs == nil {
		p.actionErrs = map[string]string{}
	}
	p.actionErrs[pluginID] = message
}

// clearActionError 抹掉上一次动作的错误（官方在发起新动作前 `delete`，:197-201）。
func (p *Plugin) clearActionError(pluginID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.actionErrs, pluginID)
}

// fireAction 发一次生命周期动作。
//
// **这是本包唯一调用 `Actions.Cordis*` 的地方，也是唯一起 goroutine 的地方**：
// 三条动作都会真的动宿主的插件进程（run 会拉起外部 node，stop/undefine 会关掉它），
// 而它们是同步阻塞的 HTTP 调用——在 Draw 里等就是卡整窗（`pluginapi/plugin.go` 的
// `CordisRun` 注释）。所以：记 pending → 起 goroutine → 收回结果只写界面态。
//
// 官方 `runAction`（OFF:CordisPanel.tsx:194-220）的其余部分逐条对上：
//
//	pending 中再点?  → 直接 return（官方 :195 `if (pending.has(pluginId)) return`）
//	发起前清错误      → clearActionError（官方 :197-201）
//	失败记原文        → setActionError（官方 :204-211）
//	成功后 onRefresh  → **不学**：本仓壳在动作成功后自己重读清单并 Invalidate
//	                    （`pluginapi/plugin.go` 的 `CordisStop` 注释），插件不重复触发。
func (p *Plugin) fireAction(env *pluginapi.Env, row apiclient.CordisRow,
	call func(pluginapi.Actions) error) {
	if p.isPending(row.PluginID) {
		return
	}
	p.setPending(row.PluginID, true)
	p.clearActionError(row.PluginID)
	// env.Act 是值拷贝（一组函数值），goroutine 里用它是安全的；但 env 本身不许跨线程用。
	actions := env.Act
	go func() {
		err := call(actions)
		if err != nil {
			// 宿主原文：`ok:false` 的理由与传输失败都已经由壳折成 error
			// （`pluginapi/plugin.go` 的 `CordisRun` 注释），这里不加工、不改写。
			p.setActionError(row.PluginID, err.Error())
		}
		p.setPending(row.PluginID, false)
	}()
}

// ── 触发器几何缓存 ───────────────────────────────────────────────────────

// rememberTrigger 记下本帧量到的触发器盒子，等 `AfterDraw` 读矩形。
func (p *Plugin) rememberTrigger(box ui.Box) {
	p.mu.Lock()
	p.triggerBox = box
	p.mu.Unlock()
}

// takeTriggerBox 取走并清空上一帧记下的盒子（读一次就够，不留跨帧引用）。
func (p *Plugin) takeTriggerBox() (ui.Box, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	box := p.triggerBox
	p.triggerBox = ui.Box{}
	return box, box.Valid()
}

// setTriggerRect 存下触发器的真实矩形（面板锚点用）。
func (p *Plugin) setTriggerRect(rect ui.Rect) {
	p.mu.Lock()
	p.triggerRect = rect
	p.mu.Unlock()
}

// lastTriggerRect 取上一帧的触发器矩形（还没有就是零值 + false）。
func (p *Plugin) lastTriggerRect() (ui.Rect, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.triggerRect, p.triggerRect.Width > 0 && p.triggerRect.Height > 0
}
