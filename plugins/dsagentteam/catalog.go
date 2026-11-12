// catalog.go —— 父容器 B：子代理目录（`conversation.session.header.actions.subagent.footer`
// 的**父级**）。
//
// # 官方归属先说清楚（免得下一个人以为这是 agent-team 的东西）
//
// 官方这个容器属于 **`ui-subagent`**，不属于 `ui-agent-team`：
//
//	conversation.session.header.actions            ← ui-subagent 注册点（index.ts:100-111）
//	└─ SubagentCatalogAction（id: subagent-catalog, order 10）
//	   │   children 声明：`conversation.session.header.actions.subagent.footer`
//	   │                 （index.ts:105、slots.ts:6-9，list/session）
//	   ├─ 触发器 button（role=dialog）：users 图标 14 + `{count} 个子智能体`
//	   └─ createPortal(菜单, document.body)        SubagentHeaderLineage.tsx:785
//	      └─ MenuSurface.menu (360px)               .menu C§152-178
//	         ├─ menuBody
//	         │  ├─ sectionHeader（有共享任务时：图标 + "子智能体" + 进度）
//	         │  ├─ sectionEmpty（有共享任务但目录为空："暂无子智能体"）
//	         │  ├─ div[role=tree] → CatalogRows（子会话树）
//	         │  └─ **renderFooter() → conversation.session.header.actions.subagent.footer**
//	         │      （SubagentHeaderLineage.tsx:868；任务板 TeamBoard 就画在这里）
//
// 按写作用域这个文件先落在 `plugins/dsagentteam`（同一波只有这个包归我写）；
// 等独立的 `ds-subagent` 插件落地后整文件搬过去，导出 API 不用改——这里刻意
// 只依赖 `Store.View()` 里的数据快照与 `pluginapi`，不碰 dsagentteam 的其它内部。
//
// # 可见性（官方写死，逐条照抄）
//
//	子会话（address / origin=subagent）         → 不渲染（SubagentCatalogAction:845-849）
//	presentedCatalog === undefined               → 不渲染（:650-652）
//	visible = catalogKnown && (hasBoardContent || error || entries>0)   (:648-656)
//	hasSharedTasks = footerAvailable && team.enabled && 有非 deleted 任务  (RootCatalogAction:857-858)
//	hasBoardContent = footerAvailable && (hasSharedTasks || failure)      (:859)
//
// `footerAvailable` 官方是"该槽有没有注册者"，本仓等价物是 `env.Has(SlotSubagentFooter)`。
//
// # 几何（`SubagentHeaderLineage.module.css`，逐条）
//
//	.trigger       min-height:28px; padding:3px 2px; gap:4px; 12px/18px; radius sm
//	.activitySlot  14×14；.count 同 .trigger 字号
//	.menu          width:min(360px,100vw-32px); padding:6px; max-height:min(560px,100dvh-140px)
//	               top:calc(anchor(bottom)+5px)；left:clamp(16px,anchor(left),100vw-宽-16)
//	.sectionHeader flex; gap:7px; padding:4px 7px 8px；icon 14×24；title 13px/24px/500
//	.sectionProgress margin-inline-start:4px; padding-top:2px；13px/20px
//	.sectionEmpty  padding:0 7px 8px 28px; 12px
//	.row           min-height:40px; gap:7px; padding:4px 6px 4px 9px; radius md; 12px/17px
//	.rowActivitySlot 14×17
package dsagentteam

import (
	"sync"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// SlotSessionHeaderActions 是官方 `conversation.session.header.actions`
// （uikernel 的 `OffSessionHeaderActions`；ui-subagent 就往这里注册目录动作）。
const SlotSessionHeaderActions = pluginapi.Slot("conversation.session.header.actions")

// 官方几何（SubagentHeaderLineage.module.css）。
const (
	catalogTriggerMinHeight = 28 // .trigger{min-height:28px}
	catalogTriggerPadV      = 3  // .trigger{padding:3px 2px}
	catalogTriggerPadH      = 2
	catalogTriggerGap       = 4 // .trigger{gap:4px}
	catalogTriggerFont      = 12
	catalogTriggerLine      = 18
	catalogActivitySize     = 14 // .activitySlot{14×14}

	catalogMenuWidth       = 360 // min(360px, 100vw-32px)
	catalogMenuPad         = 6   // .menu{padding:6px}
	catalogMenuMaxHeight   = 560 // min(560px, 100dvh-140px)
	catalogMenuTopOffset   = 5   // top: calc(anchor(bottom) + 5px)
	catalogMenuWindowInset = 16  // left/right clamp 的 16px
	catalogMenuRadius      = 12  // MenuSurface 的圆角档（radius md）

	catalogSectionGap        = 7 // .sectionHeader{gap:7px}
	catalogSectionPadTop     = 4 // padding:4px 7px 8px
	catalogSectionPadH       = 7
	catalogSectionPadBottom  = 8
	catalogSectionIconW      = 14 // .sectionIcon 14×24
	catalogSectionIconH      = 24
	catalogSectionTitleLine  = 24 // .sectionTitle 13px/24px
	catalogSectionTitleSize  = 13
	catalogSectionProgLead   = 4 // .sectionProgress margin-inline-start:4px
	catalogSectionProgTop    = 2 // padding-top:2px
	catalogSectionProgLine   = 20
	catalogSectionEmptyFont  = 12
	catalogSectionEmptyLeft  = 28 // padding:0 7px 8px 28px
	catalogSectionEmptyRight = 7

	catalogRowMinHeight = 40 // .row{min-height:40px}
	catalogRowGap       = 7  // gap:7px
	catalogRowPadV      = 4  // padding:4px 6px 4px 9px
	catalogRowPadLeft   = 9
	catalogRowPadRight  = 6
	catalogRowFont      = 12
	catalogRowLine      = 17
	catalogRowActW      = 14 // .rowActivitySlot 14×17
	catalogRowActH      = 17
)

// 官方文案（`subagent` 语言域 zh 字典，逐字）。
const (
	zhCatalogSectionTitle  = "子智能体"
	zhCatalogSectionEmpty  = "暂无子智能体"
	zhCatalogLoadError     = "无法加载子智能体"
	zhCatalogRetry         = "重试"
	zhCatalogCountTemplate = "{count} 个子智能体"
	zhCatalogTreeAria      = "子智能体会话"
	// zhCatalogFooterMissing 是"目录打开了、但 footer 槽没人注册"时的空态说明。
	//
	// 官方这一格就是 `renderFooter()`，没人注册时**什么都不画**；本仓多一句话
	// 是因为我们同时是父容器与消费方的宿主，留一个可诊断的痕迹比纯空白好查。
	zhCatalogFooterMissing = "没有可画的内容（footer 槽没有注册者）"
)

// catalogUI 是本容器的界面态（开合）。
//
// 放在独立结构里而不是 `Plugin` 上：`agent_team.go` 由并行线改（Panels 追加），
// 我这边只加**一个字段**，减少同文件冲突面。
type catalogUI struct {
	mu   sync.Mutex
	open bool
	// tier 是壳档位（official / legacy）：官方动作行 vs 旧三栏过渡桥。
	tier *uikernel.ShellTier
}

// attachTier 记下壳档位对象（Register 时注入；nil = 按 official）。
func (c *catalogUI) attachTier(tier *uikernel.ShellTier) {
	c.mu.Lock()
	c.tier = tier
	c.mu.Unlock()
}

// officialSlotsLive 报告官方子槽这一帧会不会被 dispatch（见 uikernel.ShellTier）。
//
// 只看"是不是 official 档位"不够：旧部件还在占槽时官方骨架会换成旧三栏排版，
// `conversation.session.header.actions` 不被 dispatch，`main.header` 才会。
func (c *catalogUI) officialSlotsLive() bool {
	c.mu.Lock()
	tier := c.tier
	c.mu.Unlock()
	return tier.OfficialSlotsLive()
}

// toggle 切目录开合（官方点触发器 = 开/关；点外面 = 关）。
func (c *catalogUI) toggle() {
	c.mu.Lock()
	c.open = !c.open
	c.mu.Unlock()
}

// close 收起目录（点遮罩 / 点某一行跳转后）。
func (c *catalogUI) close() {
	c.mu.Lock()
	c.open = false
	c.mu.Unlock()
}

// isOpen 报告目录是否展开。
func (c *catalogUI) isOpen() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.open
}

// CatalogOpen 返回子代理目录是否展开（壳/测试用）。
func (p *Plugin) CatalogOpen() bool { return p.catalog.isOpen() }

// OpenCatalog / CloseCatalog / ToggleCatalog 供壳与测试驱动（与点击等价）。
func (p *Plugin) OpenCatalog()   { p.catalog.toggleIf(true) }
func (p *Plugin) ToggleCatalog() { p.catalog.toggle() }
func (p *Plugin) CloseCatalog()  { p.catalog.close() }

// toggleIf 把开合设成指定值（测试与壳的无鼠标入口用）。
func (c *catalogUI) toggleIf(open bool) {
	c.mu.Lock()
	c.open = open
	c.mu.Unlock()
}

// catalogPanels 返回本容器的 painter（由 `Panels()` 追加）。
//
// 三条各管一段，因为 viva 没有 portal：
//
//	触发器      官方槽 conversation.session.header.actions（官方骨架的 header 带）
//	触发器桥    旧槽 main.header（旧三栏的顶栏；official 档自动让位）
//	面板        官方 createPortal(...) 的等价物 = 全窗覆盖层 shell.overlay
//	            （与 TeamDisableConfirmation 同层；**footer 槽在面板里被真 drawSlot**）
func (p *Plugin) catalogPanels() []pluginapi.Painter {
	return []pluginapi.Painter{
		// 触发器：可见像素（图标 + 计数 + 命中区）都在 Draw 里，没有子槽。
		{ID: "agent-team.catalog-trigger", Slot: SlotSessionHeaderActions, Order: 10,
			Draw: p.drawCatalogTrigger},
		// 旧三栏桥：同一个触发器，绝对定位到顶栏右端（official 档自动让位）。
		// Order 93：ds-open 的 open-in-app 占着 90（同槽同 Order 谁在上由装配顺序决定，
		// 那是隐式设计）；本桥排它之后，命中区压在最上面一层里靠后的位置。
		{ID: "agent-team.catalog-trigger-legacy", Slot: pluginapi.SlotMainHeader, Order: 93,
			Draw: p.drawCatalogTriggerLegacy},
		// 面板：Draw 画**遮罩层**（全窗命中区 = 官方菜单外的 pointerdown 关闭），
		// Children 画面板本体并 **dispatch footer 槽**（官方 renderFooter 的位置）。
		// 为什么必须拆两半：`drawSlot` 只存在于 Children 的回调里，而面板内容
		// 必须画在面板自己的盒子里——两件事只能分两段做。
		{ID: "agent-team.catalog-panel", Slot: pluginapi.SlotShellOverlay, Order: 40,
			Draw: p.drawCatalogBackdrop, Children: p.drawCatalogPanel},
	}
}

// catalogFacts 是本帧可见性判据的打包结果（官方 RootCatalogAction 的三元组）。
type catalogFacts struct {
	// child 是"这是子会话"：官方两条判据任一为真就不渲染。
	child bool
	// known 是目录投影已知（官方 presentedCatalog !== undefined）。
	known bool
	// entries 是直接子代理清单（服务端顺序）。
	entries []CatalogEntry
	// err 是目录读取失败原因（官方 state === 'error'）。
	err string
	// hasSharedTasks 是"有共享任务且 footer 有注册者"。
	hasSharedTasks bool
	// hasBoardContent 是 hasSharedTasks 或投影自带 failure。
	hasBoardContent bool
	// visible 是官方 visible 的等价结果。
	visible bool
}

// catalogFactsFor 按官方判定顺序算出本帧事实。
func (p *Plugin) catalogFactsFor(env *pluginapi.Env) catalogFacts {
	view := p.team.View()
	f := catalogFacts{
		child:   view.SubagentChild || view.ListedAsSubagent,
		known:   view.CatalogKnown,
		entries: view.Catalog,
		err:     view.CatalogError,
	}
	if f.child {
		return f
	}
	if !f.known {
		// 目录未知：官方 `presentedCatalog === undefined` → 不渲染；
		// 但**错误态**例外（`state === 'error'` 要能看见并重试）。
		f.visible = f.err != ""
		return f
	}
	// footerAvailable：官方是"该槽有没有注册者"，本仓是"内核里这个槽有没有 painter"。
	footerAvailable := env != nil && env.Has != nil && env.Has(SlotSubagentFooter)
	tasks := 0
	failure := false
	if view.ProjectionKnown && view.Projection.Enabled {
		tasks = len(visibleTasks(view.Projection.Tasks))
		failure = view.Projection.Failure != ""
	}
	f.hasSharedTasks = footerAvailable && tasks > 0
	f.hasBoardContent = footerAvailable && (f.hasSharedTasks || failure)
	f.visible = f.hasBoardContent || f.err != "" || len(f.entries) > 0
	return f
}

// catalogCountLabel 是官方 `t(totalCountKey, {count: directCount})`。
func catalogCountLabel(n int) string { return formatCount(zhCatalogCountTemplate, n) }

// catalogTriggerAria 是触发器命中区的可访问名：目录已知时是计数文案（官方），
// 未知时退回树的可访问名——**不编计数**。
func catalogTriggerAria(f catalogFacts) string {
	if !f.known {
		return zhCatalogTreeAria
	}
	return catalogCountLabel(len(f.entries))
}

// drawCatalogTrigger 画官方动作行（触发器）——官方骨架的 header 带。
func (p *Plugin) drawCatalogTrigger(f *flex.Frame, env *pluginapi.Env) {
	if !p.catalog.officialSlotsLive() {
		return // 这一帧不 dispatch header.actions；让位给 drawCatalogTriggerLegacy
	}
	p.drawCatalogTriggerBody(f, env)
}

// drawCatalogTriggerLegacy 是旧三栏的过渡桥：顶栏右端（corner 按钮左侧）放同一个触发器。
//
// 不是新造挂载点：官方骨架把目录动作放在会话 header 的 actions 带里，
// 旧壳的对应位置就是 `main.header` 顶栏。几何取旧顶栏已知常量
// （dstopbar：padding 28 + corner 28 + 8 间距），**不是**官方几何——
// 这一档本来就登记为过渡债（GA 报告 §3.1 同口径）。
func (p *Plugin) drawCatalogTriggerLegacy(f *flex.Frame, env *pluginapi.Env) {
	if p.catalog.officialSlotsLive() {
		return
	}
	facts := p.catalogFactsFor(env)
	if !facts.visible {
		return
	}
	f.Row().Absolute().Right(legacyTriggerRight).Top(legacyTriggerTop).Children(func() {
		p.drawCatalogTriggerBody(f, env)
	})
}

// drawCatalogBackdrop 画面板的遮罩层（官方菜单外的 pointerdown 关闭）。
//
// 官方是 document 级 `pointerdown` 监听；viva 没有 document，等价物是
// **全窗透明命中区**：它盖在所有内容之上，所以菜单外任何一次点击都先落到它，
// 于是"点外面 = 关菜单"与官方一致。透明点击区也是真实产出的像素对象
// （有命中矩形、有 aria label），不是空函数。
func (p *Plugin) drawCatalogBackdrop(f *flex.Frame, env *pluginapi.Env) {
	if !p.catalog.isOpen() {
		return
	}
	facts := p.catalogFactsFor(env)
	if !facts.visible {
		return
	}
	id := env.Tree.ID(env.Feat, "subagent-catalog-backdrop")
	if f.Row().Absolute().Inset(0).Hit(id, zhCatalogTreeAria).Clicked() {
		p.catalog.close()
	}
}

// 旧三栏顶栏上的落点（右端 = corner 按钮左侧；见 dstopbar 的 headerPadRight=28、
// corner 28、间距 8；titleRow 高 30、padding-top 10）。
const (
	legacyTriggerRight float32 = 28 + 28 + 8
	legacyTriggerTop   float32 = 11
)

// drawCatalogTriggerBody 是触发器本体（官方 `.trigger`）。
func (p *Plugin) drawCatalogTriggerBody(f *flex.Frame, env *pluginapi.Env) {
	facts := p.catalogFactsFor(env)
	if !facts.visible {
		return
	}
	t := p.themeFor(env)
	id := env.Tree.ID(env.Feat, "subagent-catalog")
	count := len(facts.entries)

	row := f.Row().MinHeight(catalogTriggerMinHeight).Items(ui.Center).
		Gap(catalogTriggerGap).
		PadSides(catalogTriggerPadV, catalogTriggerPadH, catalogTriggerPadV, catalogTriggerPadH).
		Radius(t.Radii.SM).
		// 官方 `aria-label={t(totalCountKey,{count: directCount})}`：命中区 label 就用
		// 那句计数文案（不是另编一个"目录"）；目录未知时退回树的可访问名。
		Hit(id, catalogTriggerAria(facts)).Children(func() {
		// `.activitySlot{width:14px;height:14px}`：users 图标（官方 IconUsersOutlineRegular 14）。
		f.Row().Size(catalogActivitySize, catalogActivitySize).Items(ui.Center).
			Justify(ui.Center).Children(func() {
			f.Icon(usersIcon()).Size(catalogActivitySize-2, catalogActivitySize-2).
				Color(t.LabelTertiary)
		})
		// `.count`：官方 `{count} 个子智能体`（窄容器 ≤460px 时整段被 CSS 隐藏；
		// viva 没有容器查询，这里恒显示计数——登记为差异）。
		//
		// **目录未知时不画计数**：那时 count 只是零值，画成"0 个子智能体"就是
		// 把"不知道"说成"没有"（官方 error 态也没有可信计数）。
		if facts.known {
			f.Text(catalogCountLabel(count)).FontSize(catalogTriggerFont).
				LineHeight(catalogTriggerLine).Color(t.LabelTertiary)
		}
	})
	if row.Clicked() {
		p.catalog.toggle()
	}
}

// drawCatalogPanel 画展开后的面板（官方 `createPortal(...)` 的等价层）。
//
// **footer 槽在这里被真的 dispatch**（官方 `renderFooter()` 的位置：
// menuBody 的最后一行，SubagentHeaderLineage.tsx:868）。
func (p *Plugin) drawCatalogPanel(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	if !p.catalog.isOpen() {
		return
	}
	facts := p.catalogFactsFor(env)
	if !facts.visible {
		// 面板开着但已经不该可见（目录被清空等）：官方会关掉菜单，这里同步收起。
		p.catalog.close()
		return
	}
	t := p.themeFor(env)

	width := float32(catalogMenuWidth)
	if env.Bounds.Width > 0 && width > env.Bounds.Width-2*catalogMenuWindowInset {
		width = env.Bounds.Width - 2*catalogMenuWindowInset
	}
	if width < 0 {
		width = 0
	}
	// 官方 `top: calc(anchor(bottom) + 5px)` + `left: clamp(16px, anchor(left), 100vw-宽-16)`。
	// viva 没有 CSS anchor 查询：触发器在会话 header 的动作带（titleRow 内垂直居中），
	// 所以 bottom ≈ header padding-top 10 + titleRow 30 / 2 + 触发器 14 → 取
	// 官方几何表里的 header 顶部与行高推导，右缘按 clamp 的结果贴 16px。
	top := catalogMenuTopFromHeader()
	right := float32(catalogMenuWindowInset)

	panel := f.Column().Absolute().Top(top).Right(right).Width(width).
		MaxHeight(catalogMenuMaxHeight).Radius(catalogMenuRadius).
		Background(t.BgLayer1).Border(1, t.BorderL2).Shadow(gfx.ShadowMD).
		Pad(catalogMenuPad)
	// 面板本体吃掉点击：官方点菜单里面不会关（只有菜单外的 pointerdown 才关）。
	panel.Hit(env.Tree.ID(env.Feat, "subagent-catalog-menu"), zhCatalogTreeAria)
	panel.Children(func() {
		// `.menuBody{display:flex;flex-direction:column;overflow:auto}`：内容一列。
		f.Column().FillWidth().Gap(0).Children(func() {
			if facts.hasSharedTasks {
				p.drawCatalogSectionHeader(f, t)
			}
			if facts.hasSharedTasks && len(facts.entries) == 0 && facts.err == "" {
				f.Text(zhCatalogSectionEmpty).FillWidth().FontSize(catalogSectionEmptyFont).
					LineHeight(catalogRowLine).Color(t.LabelTertiary).
					PadSides(0, catalogSectionEmptyRight, catalogSectionPadBottom, catalogSectionEmptyLeft)
			}
			if facts.err != "" {
				p.drawCatalogError(f, env, t, facts.err)
			}
			// `div[role=tree]`：子会话行。
			for _, entry := range facts.entries {
				p.drawCatalogRow(f, env, t, entry)
			}
			// ── footer：官方 SubagentHeaderLineage.tsx:868 `props.renderFooter?.()` ──
			// 无论有没有注册者都调一次（与官方 `renderSlot` 一致），
			// 这样"槽写错了"表现为空而不是"父级忘了画"。
			f.Column().FillWidth().Children(func() {
				drawSlot(SlotSubagentFooter)
			})
		})
	})

}

// catalogMenuTopFromHeader 是面板顶边（官方 `anchor(bottom) + 5px` 的等价推导）。
//
// 依据是官方几何表里 header 的两行（`header padding-top 10`、`titleRow 30`）与
// 触发器自身高度（min-height 28，垂直居中）：10 + 30/2 + 28/2 = 39，再加 5px 间距。
const catalogMenuTopBase float32 = 10 + 30/2 + 28/2 // 39

func catalogMenuTopFromHeader() float32 { return catalogMenuTopBase + catalogMenuTopOffset }

// drawCatalogSectionHeader 画 `.sectionHeader`（图标 + "子智能体" + 进度）。
func (p *Plugin) drawCatalogSectionHeader(f *flex.Frame, t *pluginapi.Theme) {
	f.Row().FillWidth().Items(ui.Start).Gap(catalogSectionGap).
		PadSides(catalogSectionPadTop, catalogSectionPadH, catalogSectionPadBottom, catalogSectionPadH).
		Children(func() {
			f.Row().Size(catalogSectionIconW, catalogSectionIconH).Items(ui.Center).
				Justify(ui.Center).Children(func() {
				f.Icon(usersIcon()).Size(catalogSectionIconW-2, catalogSectionIconW-2).
					Color(t.LabelTertiary)
			})
			f.Text(zhCatalogSectionTitle).FontSize(catalogSectionTitleSize).
				LineHeight(catalogSectionTitleLine).Weight(500).Color(t.LabelPrimary)
			if progress := p.catalogProgressLabel(); progress != "" {
				f.Text(progress).FontSize(catalogSectionTitleSize).
					LineHeight(catalogSectionProgLine).Color(t.LabelTertiary).
					MarginSides(catalogSectionProgTop, 0, 0, catalogSectionProgLead).
					Grow(1)
			}
		})
}

// catalogProgressLabel 是这一格的进度文案。
//
// 官方 `.sectionProgress` 画的是**子会话**的完成/进行/未运行段（`progress.*`），
// 需要每个子会话的运行态；本仓宿主的 `subagentCatalog` 只有 id/createdAt/mode/label，
// **没有运行态**，所以这里不编一个"0 进行中"——返回空串，这一格就不画。
func (p *Plugin) catalogProgressLabel() string { return "" }

// drawCatalogError 画官方错误态（`.error` + 重试按钮）。
func (p *Plugin) drawCatalogError(f *flex.Frame, env *pluginapi.Env, t *pluginapi.Theme, reason string) {
	f.Row().FillWidth().Items(ui.Start).Gap(catalogSectionGap).
		PadSides(catalogSectionPadTop, catalogSectionPadH, catalogSectionPadBottom, catalogSectionPadH).
		Children(func() {
			f.Text(zhCatalogLoadError + "：" + reason).FillWidth().Grow(1).
				FontSize(catalogSectionEmptyFont).LineHeight(catalogRowLine).
				Color(t.Error)
			if f.Button(env.Tree.ID(env.Feat, "subagent-catalog-retry"), zhCatalogRetry).
				Outline().Small().Clicked() {
				// 重试 = 让壳重新拉一次投影（壳自己排程；插件不直接发请求）。
				if env.Act.Notify != nil {
					env.Act.Notify(zhCatalogLoadError)
				}
			}
		})
}

// drawCatalogRow 画一行子会话（官方 `.row`）。
//
// 本仓宿主只给 id/createdAt/mode/label：activity 槽保留**结构宽度**但**不画状态点**
// —— 没有运行态数据就画一个中性点等于编数据（官方在这个位置有 running/completed/
// inactive 三态）。点行 = 打开那个子会话（官方 openChild）。
func (p *Plugin) drawCatalogRow(f *flex.Frame, env *pluginapi.Env, t *pluginapi.Theme, entry CatalogEntry) {
	id := env.Tree.ID(env.Feat, "subagent-catalog-row-"+entry.ID)
	row := f.Row().FillWidth().MinHeight(catalogRowMinHeight).Items(ui.Start).
		Gap(catalogRowGap).
		PadSides(catalogRowPadV, catalogRowPadRight, catalogRowPadV, catalogRowPadLeft).
		Radius(t.Radii.MD).Hit(id, entry.catalogLabel()).Children(func() {
		// `.rowActivitySlot{14×17}`：保留槽宽，不画状态（见函数注释）。
		f.Row().Size(catalogRowActW, catalogRowActH)
		f.Text(entry.catalogLabel()).Grow(1).FontSize(catalogRowFont).
			LineHeight(catalogRowLine).Color(t.LabelPrimary).Ellipsis()
	})
	if row.Clicked() {
		p.catalog.close()
		if env.Act.OpenSession != nil {
			env.Act.OpenSession(entry.ID)
		}
	}
}

// AttachShellTier 由壳在 `Load` 之后调用一次，把壳档位告诉本容器。
//
// 为什么走"壳注入"而不是 `Register` 里 `ctx.Inject`：容器 painter 归
// `agent_team.go` 的 `Panels()` 登记（并行线在改那个文件），而档位要等
// `Load` 完成、`shell.root` 占用方确定后才能解析——两者时序对不上。
// 壳遍历 `kernel.Plugins()` 做一次类型断言调用，零契约改动。
func (p *Plugin) AttachShellTier(tier *uikernel.ShellTier) { p.catalog.attachTier(tier) }
