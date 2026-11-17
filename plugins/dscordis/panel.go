package dscordis

// panel.go —— 官方面板的两块画：触发器（侧栏底部动作带）与面板本体（整窗浮层）。
//
// 官方是一个 React 组件里的两段（`CordisPanel.tsx`：`.badge` 在 :485-504，
// `open` 决定的 `<section className={css.panel}>` 在 :459-484）。Viva 没有 portal、
// 一个槽只能挂一块画，所以拆成两块：
//
//	drawTrigger   `uikernel.OffSidebarFooterActions`（官方 sidebar.footer.action）
//	drawBackdrop  `pluginapi.SlotShellOverlay` 的 Draw   = 全窗命中区（点外面就收起）
//	drawPanel     `pluginapi.SlotShellOverlay` 的 Children = 面板本体
//
// 遮罩 + 本体的两半形状照本仓既有先例 `plugins/dsagentteam/catalog.go:206-437`
// （`drawCatalogBackdrop` / `drawCatalogPanel`）：Draw 画全窗命中区，Children 画盒子。

import (
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 交互 id 的名字部分（一律经 `env.Tree.ID(env.Feat, …)` 生成，不写死数字）。
//
// 命名对齐官方 DOM 上的 data 属性（`data-cordis-badge` / `data-cordis-row` /
// `data-cordis-switch="run|stop"` / `data-cordis-remove`），方便按官方源码检索。
const (
	// hitTrigger 是 `.badge`（官方 `data-cordis-badge`，CordisPanel.tsx:489）。
	hitTrigger = "cordis-trigger"
	// hitBackdrop 是遮罩层（官方 `useDismissOnOutsidePointer`，CordisPanel.tsx:153）。
	hitBackdrop = "cordis-backdrop"
	// hitPanel 是面板本体（官方 `<section data-cordis-panel>`，:460）。
	hitPanel = "cordis-panel"

	// hitRunPrefix / hitStopPrefix / hitRemovePrefix 是三个动作按钮
	// （官方 `data-cordis-switch="run"` / `="stop"` / `data-cordis-remove`，:330-395）。
	// 后面接 pluginId：同一个面板里多行都有同名按钮，身份必须按行区分。
	hitRunPrefix    = "cordis-run-"
	hitStopPrefix   = "cordis-stop-"
	hitRemovePrefix = "cordis-remove-"
)

// visibleRows 返回本帧清单里**应当显示**的行。
//
// 官方 `inventory.rows` 已经把本页 `retire()` 掉的插件删掉了（OFF:inventory.ts:102-106），
// 本仓的清单位于快照里，移除动作之后宿主还没重读回来之前它仍然带着那一行，
// 所以这里按 `CordisView.Removed` 过滤一次——语义与官方 `retire()` 相同：
// **本页显式移除过的插件不再显示**（`pluginapi/cordis.go` 的 `Removed` 注释）。
func visibleRows(env *pluginapi.Env) []apiclient.CordisRow {
	if env == nil || env.Data.Cordis == nil {
		return nil
	}
	rows := env.Data.Cordis.CordisRows()
	out := make([]apiclient.CordisRow, 0, len(rows))
	for _, row := range rows {
		if env.Data.Cordis.CordisRemoved(row.PluginID) {
			continue
		}
		out = append(out, row)
	}
	return out
}

// splitRows 按官方两组分批（CordisPanel.tsx:183-184）：
//
//	mine   = 当前主视图持有的那个会话（官方 `current`）的行
//	theirs = 其余全部（**包括 agentId 为空串的行**——官方判据是"不等于 current"，
//	         空串不等于任何会话 id，自然落到「其他会话」；这里不改判据、不猜归属）
//
// 组内顺序不变（官方不重排：分组不能改变行顺序，否则每次刷新行都在跳）。
//
// 官方还有一步 `blockingFirst`（CordisPanel.tsx:88-93）：把 `activity.phase ===
// 'awaiting-approval'` 的行提到组首。本仓没有 client-runner 的 activity
// （HIDE 第 3 条），那一步在这里是**恒等变换**，所以不实现（差异表第 7 条）。
func splitRows(rows []apiclient.CordisRow, current string) (mine, theirs []apiclient.CordisRow) {
	for _, row := range rows {
		if current != "" && row.AgentID == current {
			mine = append(mine, row)
			continue
		}
		theirs = append(theirs, row)
	}
	return mine, theirs
}

// runningCount 是触发器上的计数（官方 `running`，CordisPanel.tsx:186-190）：
// 按 `visiblePanelStatus(...) === 'running'` 数——**不是**数 activeRun 有几条。
func runningCount(rows []apiclient.CordisRow) int {
	loaded := loadedClientHalves()
	n := 0
	for _, row := range rows {
		status := visiblePanelStatus(row, selectedPackageIDOf(row), loaded)
		if status == panelStatusRunning {
			n++
		}
	}
	return n
}

// drawTrigger 画官方 `.badge`（触发器）。
//
// 官方整块不画的条件是 `if (all.length === 0) return null`（CordisPanel.tsx:192）；
// 本仓在此之前还有一条：`Snapshot.Cordis == nil` = 壳没接这条能力
// （`pluginapi/plugin.go` 的 `Cordis` 字段注释）——那时连触发器都不画，
// 不画成"还没有定义任何插件"（那是没有证据的断言）。
func (p *Plugin) drawTrigger(f ui.C, env *pluginapi.Env) {
	rows := visibleRows(env)
	if len(rows) == 0 {
		return
	}
	t := p.themeFor(env)
	open := p.isOpen()
	id := env.Tree.ID(env.Feat, hitTrigger)
	// 官方 `.badge`：图标 + `panel.trigger` + `panel.runningCount`（宽态，:496-502）。
	count := formatCount(zhPanelRunningCount, runningCount(rows))
	row := ui.Row(f).FillWidth().Height(cordisTriggerHeight).Items(ui.Center).
		Gap(cordisTriggerGap).MarginTop(cordisLayerMarginTop).
		PadSides(0, cordisTriggerPadRight, 0, cordisTriggerPadLeft).
		Radius(cordisTriggerRadius).
		Hover(ui.Look{}.Background(t.Hover)).
		// 可访问名用官方 `panel.plugins.aria`（:492），展开态照官方 `aria-expanded`（:493）。
		HitWith(id, zhPanelPluginsAria, gfx.HitOptions{
			Role: gfx.RoleButton, Expandable: true, Expanded: open,
		}).
		Children(func() {
			// 图标槽 16×16：本仓没有 `cordis-plugin-outline` 这枚 sprite，
			// 只留槽宽、不画图形（见 geometry.go 的 cordisTriggerIcon 与差异表第 2 条）。
			ui.Row(f).Size(cordisTriggerIcon, cordisTriggerIcon).Shrink(0)
			ui.Text(f, zhPanelTrigger).FontSize(cordisTriggerFont).
				Color(t.LabelPrimary).Ellipsis().MinWidth(0)
			// `.badgeCount{flex:none;margin-left:auto}`（CSS:55-56）：viva 用等价的撑开盒。
			ui.Row(f).Grow(1)
			ui.Text(f, count).FontSize(cordisTriggerCountFont).
				LineHeight(cordisTriggerCountLine).Color(t.LabelTertiary).Shrink(0)
		})
	p.rememberTrigger(row)
	if row.Clicked() {
		p.toggleOpen(env.Act)
	}
}

// afterTrigger 把本帧触发器的**真实矩形**留给下一帧的面板锚点。
//
// 为什么不能在 Draw 里读：viva 的 `Box.Rect()` 要等布局解算之后才有效，
// 同帧的 Draw 读到的还是零值（`plugins/dsagentteam/model_slot.go:196-215` 同一做法与理由）。
func (p *Plugin) afterTrigger(_ *gfx.Canvas, _ *pluginapi.Env) {
	box, ok := p.takeTriggerBox()
	if !ok || !box.Valid() {
		return
	}
	if rect := box.Rect(); rect.Width > 0 && rect.Height > 0 {
		p.setTriggerRect(rect)
	}
}

// drawBackdrop 是全窗命中区：官方 `useDismissOnOutsidePointer(rootRef, open, setOpen)`
// （CordisPanel.tsx:153）—— 菜单外任何一次 pointerdown 都收起面板。
//
// 官方监听的是 document 级 pointerdown；viva 没有 document，等价物是**全窗透明命中区**
// （`plugins/dsagentteam/catalog.go:280-299` 的 `drawCatalogBackdrop` 同形）：
// 它盖在所有内容之上，所以面板外任何一次点击都先落到它。
func (p *Plugin) drawBackdrop(f ui.C, env *pluginapi.Env) {
	if env == nil || env.Data.Cordis == nil {
		return
	}
	if !p.isOpen() {
		return
	}
	id := env.Tree.ID(env.Feat, hitBackdrop)
	if ui.Row(f).Absolute().Inset(0).Hit(id, zhPanelTitle).Clicked() {
		p.setOpen(false)
	}
}

// drawPanel 画面板本体（官方 `<section className={css.panel}>`，CordisPanel.tsx:459-484）。
func (p *Plugin) drawPanel(f ui.C, env *pluginapi.Env, _ func(pluginapi.Slot)) {
	if env == nil || env.Data.Cordis == nil {
		return
	}
	if !p.isOpen() {
		return
	}
	t := p.themeFor(env)
	cordis := env.Data.Cordis
	rows := visibleRows(env)

	width := float32(cordisPanelWidth)
	if env.Bounds.Width > 0 && width > env.Bounds.Width-cordisPanelWindowInset {
		width = env.Bounds.Width - cordisPanelWindowInset
	}
	if width < 0 {
		width = 0
	}
	left, bottom := p.panelAnchor(env)
	// 官方靠 `max-width: calc(100vw - 24px)` + 固定 420 保证不出横向滚动；
	// viva 没有 clamp()，这里显式把左缘夹回窗口里（触发器的 left 可能在很靠右的窄侧栏）。
	if env.Bounds.Width > 0 {
		if maxLeft := env.Bounds.Width - width - cordisPanelEdgeMin; left > maxLeft {
			left = maxLeft
		}
	}
	if left < cordisPanelEdgeMin {
		left = cordisPanelEdgeMin
	}
	panelMaxHeight := env.Bounds.Height * cordisPanelMaxHeightRatio
	// 正文能用的高度上限：面板最大高减掉钉在上面的头部（`.header{min-height:44px}`）。
	bodyMaxHeight := panelMaxHeight - cordisHeaderMinHeight
	if bodyMaxHeight < 0 {
		bodyMaxHeight = 0
	}
	panel := ui.Column(f).Absolute().Left(left).Bottom(bottom).Width(width).
		// `.panel{max-height:60vh}`（CSS:94）。
		MaxHeight(panelMaxHeight).
		Radius(cordisPanelRadius).Background(t.BgLayer1).
		// 阴影 = 官方 `--dsw-elevation-prominent` + `--dsw-alias-border-l1` 描边：
		// `.Fu0XRq_panel{border:0;border-radius:12px;
		//  --dsw-elevation-stroke-color:var(--dsw-alias-border-l1);
		//  box-shadow:var(--dsw-elevation-prominent)}`（CordisPanel.module.css:96-98，
		//  取值核对见 tmp/asar-css/043__CordisPanel）。
		// 原来那条 `Border(cordisPanelStroke=1, BorderL1)` 是"viva 预设没有描边层"的
		// 手工代偿——1px 占布局描边 + 零描边阴影；官方是 0.5px **不占布局**的环。
		Draw(func(d *ui.DrawContext) {
			dscore.PaintElevation(d, cordisPanelRadius, t.Shadows.Prominent, t.BorderL1)
		})
	// 面板本体吃掉点击：官方点面板里面不关（只有面板外的 pointerdown 才关）。
	panel.Hit(env.Tree.ID(env.Feat, hitPanel), zhPanelTitle)
	panel.Children(func() {
		// `.header{min-height:44px;padding:10px 12px}`（CSS:116-124）+ `.title`（:133-138）。
		ui.Row(f).FillWidth().MinHeight(cordisHeaderMinHeight).
			PadSides(cordisHeaderPadV, cordisHeaderPadH, cordisHeaderPadV, cordisHeaderPadH).
			Items(ui.Center).Children(func() {
			ui.Text(f, zhPanelTitle).FontSize(cordisTitleFont).
				LineHeight(cordisTitleLine).Weight(500).Color(t.LabelPrimary)
		})
		// `.body{flex:1;min-height:0;overflow-y:auto;padding:0 12px 12px}`（CSS:126-131）：
		// 只有正文滚动，标题钉在上面。
		//
		// **MaxHeight 必须写在 Scroll 自己身上**：viva 的滚动区"高度由外面告诉它，
		// 不是由内容决定"（`ui/layout.go` 的 `if n.size[axis].kind == dimAuto &&
		// n.maxS[axis] == 0 { n.pref[axis] = is + ie }`）——不给自己封顶，它就只按
		// 内边距占高（实测 12px），正文被自己的裁剪框切掉，画了等于没画。
		// 封顶后它按内容长高、到顶才滚，正是官方 `.body` 的行为。
		ui.Scroll(f).FillWidth().MinHeight(0).MaxHeight(bodyMaxHeight).
			PadSides(0, cordisBodyPadH, cordisBodyPadBottom, cordisBodyPadH).
			Children(func() {
				p.drawPanelBody(f, env, t, cordis, rows)
			})
	})
}

// drawPanelBody 是面板正文档三段状态 + 两个分组（CordisPanel.tsx:464-482）。
func (p *Plugin) drawPanelBody(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	cordis *pluginapi.CordisView, rows []apiclient.CordisRow) {
	// ① 读失败：带宿主原文，**并保留旧行**（`Error` 非空时不清空 Rows，
	//    见 pluginapi/cordis.go 的 Error 注释）。官方 :465-467。
	if cordis.Error != "" {
		ui.Text(f, formatReadFailed(zhPanelReadFailed, cordis.Error)).
			FillWidth().FontSize(cordisNoteFont).LineHeight(cordisNoteLine).
			Color(t.Error).MarginSides(cordisNoteMargin, 0, cordisNoteMargin, 0)
	}
	// ② 还没读到过一次：说「读取中…」，**不许**说「还没有定义任何插件」
	//    —— 在知道之前宣称"没有"是编的（官方 :468）。
	if !cordis.Known && cordis.Error == "" {
		ui.Text(f, zhPanelLoading).FillWidth().
			FontSize(cordisNoteFont).LineHeight(cordisNoteLine).
			Color(t.LabelTertiary).MarginSides(cordisNoteMargin, 0, cordisNoteMargin, 0)
	}
	// ③ 读到了，但一条都没有（官方 :469）。
	if cordis.Known && len(rows) == 0 {
		ui.Text(f, zhPanelEmpty).FillWidth().
			FontSize(cordisNoteFont).LineHeight(cordisNoteLine).
			Color(t.LabelTertiary).MarginSides(cordisNoteMargin, 0, cordisNoteMargin, 0)
	}
	// ④ 两组（官方 :470-481）：空组整块不画（连组标题都不画）。
	current := ""
	if env != nil {
		current = env.Data.Cur
	}
	mine, theirs := splitRows(rows, current)
	p.drawGroup(f, env, t, zhPanelGroupCurrent, mine)
	p.drawGroup(f, env, t, zhPanelGroupOthers, theirs)
}

// drawGroup 画一组（官方 `<section><h3 className={css.group}>…</h3><ul>…</ul></section>`）。
func (p *Plugin) drawGroup(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, title string, rows []apiclient.CordisRow) {
	if len(rows) == 0 {
		return
	}
	ui.Column(f).FillWidth().Children(func() {
		// `.group{margin:8px 0 8px;font-size:11px;line-height:16px;font-weight:500}`
		// （CSS:153-158）。官方那条 text-transform/letter-spacing 见 geometry.go 的说明。
		ui.Text(f, title).FillWidth().FontSize(cordisGroupFont).
			LineHeight(cordisGroupLine).Weight(500).Color(t.LabelCaption).
			MarginSides(cordisGroupMargin, 0, cordisGroupMargin, 0)
		// `.rows{gap:8px}`（CSS:163-170）。
		ui.Column(f).FillWidth().Gap(cordisRowsGap).Children(func() {
			for _, row := range rows {
				p.drawRow(f, env, t, row)
			}
		})
	})
}

// drawRow 画一行（官方 `.row` 的 DOM 顺序，CordisPanel.tsx:251-453）。
//
// 落地顺序与官方一致：
//
//	rowHead    rowId（等宽）+ rowName（选中 Package 的 name）+ rowStatus（点 + 文案）
//	[版本选择器] 条件照写、恒不出现（本宿主恒 1 个 Package，差异表第 6 条）
//	rowDetail  rowPurpose + rowActions（三个动作按钮的条件见 drawRowActions）
//	[transition] 条件照写、恒不出现（nextPackageId 本宿主不产生，差异表第 6 条）
//	rowError   latestRun.error 的 `message (phase)` / 最近一次动作的宿主原文
//	activeVersion  只在这一行跑的不是选中版本时出现（恒不出现，差异表第 6 条）
func (p *Plugin) drawRow(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row apiclient.CordisRow) {
	selectedID := selectedPackageIDOf(row)
	selected, hasSelected := packageOf(row, selectedID)
	active, hasActive := activePackageOf(row)
	// 官方 :231-232：`name = selectedPackage?.name ?? pluginId`（没有 fallback，
	// 官方那里空 name 就是空 —— 这里不编一个"未命名"）。
	name := row.PluginID
	purpose := ""
	if hasSelected {
		name = selected.Name
		purpose = selected.Purpose
	}
	status := visiblePanelStatus(row, selectedID, loadedClientHalves())
	busy := p.isPending(row.PluginID)

	ui.Column(f).FillWidth().Gap(cordisRowGap).
		// `.row{padding:14px 12px 10px;border:0.5px solid border-l4;border-radius:12px}`
		// （CSS:172-179）。
		PadSides(cordisRowPadTop, cordisRowPadH, cordisRowPadBottom, cordisRowPadH).
		Border(cordisRowBorder, t.BorderL4).Radius(cordisRowRadius).
		Children(func() {
			// ── rowHead（官方 :259-266）──
			ui.Row(f).FillWidth().Items(ui.Center).Gap(cordisRowHeadGap).Children(func() {
				// `.rowId`：等宽 11/20，label-tertiary（CSS:191-197）。
				ui.Text(f, row.PluginID).Mono().FontSize(cordisRowIDFont).
					LineHeight(cordisRowIDLine).Color(t.LabelTertiary).Shrink(0)
				// `.rowName`：13/20、500、ellipsis、flex:1（CSS:199-209）。
				ui.Text(f, name).FontSize(cordisRowNameFont).LineHeight(cordisRowNameLine).
					Weight(500).Color(t.LabelPrimary).Ellipsis().MinWidth(0).Grow(1)
				// `.rowStatus`：状态点 + 文案，gap 4 / height 20 / 11px（CSS:211-220）。
				ui.Row(f).Height(cordisRowStatusHeight).Items(ui.Center).
					Gap(cordisRowStatusGap).Shrink(0).Children(func() {
					p.drawDot(f, panelDotColor(t, status, busy))
					ui.Text(f, panelStatusLabel(status)).FontSize(cordisRowStatusFont).
						LineHeight(cordisRowStatusLine).Color(t.LabelCaption)
				})
			})
			// 版本选择器（官方 :267-285）不会出现：条件是该行 `packages.length > 1`，
			// 本宿主恒 1 个 Package（pluginapi/cordis.go 的 Rows 注释）。
			// **条件照官方写在这里，不画一个没有真实选择动作的下拉框**（差异表第 6 条）。
			//
			// ── rowDetail（官方 :286-398）──
			ui.Row(f).FillWidth().MinHeight(cordisRowDetailMinHeight).
				Items(ui.Center).Gap(cordisRowDetailGap).Children(func() {
				// `.rowPurpose`：`purpose` 空串就空着——官方 `selectedPackage?.purpose ?? ''`
				// （:233-234），宿主没给来源时宿主本来就给空串（apiclient/cordis.go 的注释）。
				if purpose != "" {
					ui.Text(f, purpose).FontSize(cordisRowPurposeFont).
						LineHeight(cordisRowPurposeLine).Color(t.LabelTertiary).
						Ellipsis().MinWidth(0).Grow(1)
				} else {
					ui.Row(f).Grow(1)
				}
				// `.rowActions{gap:10px}`（CSS:267-272）。
				ui.Row(f).Items(ui.Center).Gap(cordisRowActionsGap).Shrink(0).Children(func() {
					p.drawRowActions(f, env, t, row, selected, hasSelected, status, busy)
				})
			})
			// transition 块（官方 :399-430）不会出现：条件是
			// `nextPackageId !== undefined && nextPackageId !== currentPackageId`，
			// 而本宿主不产生 nextPackageId（apiclient/cordis.go 的 Rows 注释）。
			// **条件照官方写在这里，不给假状态**（差异表第 6 条）。
			//
			// ── rowError（官方 :431-437）──
			p.drawRowError(f, t, row)
			// `.activeVersion`（官方 :450-452）：只在这一行跑的不是选中版本时出现。
			// 本宿主恒 1 个 Package → 选中的 Package 与在跑的 Package 必然是同一个
			// → 恒不出现；照官方条件写（差异表第 6 条）。
			if hasActive && selectedID != active.PackageID {
				ui.Text(f, zhStatusRunning+": "+active.Name+" · "+active.PackageID).
					FontSize(cordisActiveVersionFont).LineHeight(cordisActiveVersionLine).
					Color(t.LabelCaption)
			}
		})
}

// drawRowError 画行下方那几行红字（官方 `.rowError`，CordisPanel.tsx:431-449）。
//
// 官方三条来源的顺序是 `runErrors` → `latestRun.error` → `actionErrors` → `renderFailures`；
// 本仓只有中间两条：第一条与第四条的数据面（client-runner 的 run errors / render failures）
// 属于没有 JS 宿主的 HIDE（第 3、6 条）。
func (p *Plugin) drawRowError(f ui.C, t *pluginapi.Theme, row apiclient.CordisRow) {
	// `hostFailure = latestRun.status === 'failed' ? latestRun.error : undefined`
	// （官方 :242），文案是 `` `${message} (${phase})` ``（:434-436）。
	if latest := row.LatestRun; latest != nil &&
		latest.Status == cordisRunStatusFailed && latest.Error != nil {
		ui.Text(f, latest.Error.Message+" ("+latest.Error.Phase+")").
			FillWidth().FontSize(cordisRowErrorFont).LineHeight(cordisRowErrorLine).
			Color(t.Error)
	}
	// 最近一次动作的**宿主原文**（官方 :437 的 `actionError`）：宿主/传输把
	// `ok:false` 的理由折成 error 交回来，那串字就在这里显示，不加工、不改写。
	if msg := p.actionErrorOf(row.PluginID); msg != "" {
		ui.Text(f, msg).FillWidth().FontSize(cordisRowErrorFont).
			LineHeight(cordisRowErrorLine).Color(t.Error)
	}
}

// drawRowActions 画右侧的动作按钮，**显示条件逐条照官方**（CordisPanel.tsx:326-396）：
//
//	(a) :326-342  没有 activeRun 且选中 Package 存在            → 运行
//	(b) :343-359  有 activeRun 且选中 ≠ activeRun.packageId     → 运行（切版本）
//	(c) :360-376  有 activeRun 且 client-pending 且同包         → 运行（促 Client 加载回执）
//	(d) :377-386  有 activeRun                                   → 停止
//	(e) :387-396  这一行存在                                     → 移除
//
// 审批三按钮（:289-325）不画：本宿主不发 `cordis/request-run`，
// `approvalRequestId` / `requiresApproval` 不会出现（HIDE 第 4 条）。
//
// # HIDE 第 5 条：运行按钮只对**没有 Client 半**的包画
//
// 官方 (a)(b)(c) 三条的 onRun 都会连带加载 Client 半并结算（`startUserRun`
// → `getClientCode` → 页面加载后 `settleUserRun`）。本仓只能起 Host 半，
// 起了就永远停在 `client-pending` —— 那是**点了没结果**的按钮，比不画更坏。
// 所以三条都再与 `canRun`（选中包存在且 `!HasClientHalf`）取交集。
//
// (c) 因此恒不成立：`client-pending` 的定义就要求该包 `HasClientHalf == true`
// （status.ts:38-44），与 `canRun` 互斥。判据写在注释里而不是删掉，
// 是为了让"这里为什么没有运行按钮"在代码里可读。
func (p *Plugin) drawRowActions(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	row apiclient.CordisRow, selected apiclient.CordisPackage, hasSelected bool,
	status panelStatus, busy bool) {
	canRun := hasSelected && !selected.HasClientHalf
	activeRun := row.ActiveRun
	// 官方 (c) 还要 `activePackage !== undefined`（:360-361）：在跑的包必须还在清单里。
	_, hasActive := activePackageOf(row)

	// (a) + (b)：官方两处同一个 `data-cordis-switch="run"`，条件互斥（有无 activeRun）。
	// `env.Act.CordisRun == nil` = 壳没接这条动作 → 按钮不画（死按钮比没有更坏，
	// `pluginapi/plugin.go` 的 `CordisRun` 字段注释）。
	if env.Act.CordisRun != nil && canRun &&
		(activeRun == nil || selected.PackageID != activeRun.PackageID) {
		packageID := selected.PackageID
		if p.drawActionButton(f, env, t, hitRunPrefix+row.PluginID, zhActionRun, busy) {
			p.fireAction(env, row, func(actions pluginapi.Actions) error {
				return actions.CordisRun(row.AgentID, row.PluginID, packageID, runModeOf(row, packageID))
			})
		}
	}
	// (c) 官方 :360-376：client-pending 且选中的就是正在跑的那个包 → 再点一次 run
	//     （官方语义是"催一次 Client 加载回执"）。判据照写；它**恒不成立**：
	//     `panelStatusClientPending` 的定义要求该包 `HasClientHalf == true`
	//     （status.ts:38-44），而 canRun 要求 `!HasClientHalf`，两者互斥。
	if env.Act.CordisRun != nil && status == panelStatusClientPending && activeRun != nil &&
		hasActive && selected.PackageID == activeRun.PackageID && canRun {
		packageID := selected.PackageID
		if p.drawActionButton(f, env, t, hitRunPrefix+row.PluginID, zhActionRun, busy) {
			p.fireAction(env, row, func(actions pluginapi.Actions) error {
				return actions.CordisRun(row.AgentID, row.PluginID, packageID, apiclient.CordisRunModeRun)
			})
		}
	}

	// (d) 停止：有 activeRun 就画（官方不额外要求状态）。动作没接就不画。
	if env.Act.CordisStop != nil && activeRun != nil {
		if p.drawActionButton(f, env, t, hitStopPrefix+row.PluginID, zhActionStop, busy) {
			p.fireAction(env, row, func(actions pluginapi.Actions) error {
				return actions.CordisStop(row.AgentID, row.PluginID)
			})
		}
	}
	// (e) 移除：这一行存在就画（官方一次点击就发，没有二次确认，:387-396）。
	// 动作没接就不画——它能**删宿主上的定义**，画一个点了没反应的删除键最坏。
	if env.Act.CordisRemove != nil &&
		p.drawActionButton(f, env, t, hitRemovePrefix+row.PluginID, zhActionRemove, busy) {
		p.fireAction(env, row, func(actions pluginapi.Actions) error {
			return actions.CordisRemove(row.AgentID, row.PluginID)
		})
	}
}

// drawActionButton 画 `RowAction` 的按钮壳（官方 `.actionButton`，CSS:320-345）。
//
// 28×28 圆形、透明底、悬停叠色；可读文案在 `aria-label` 里（官方就是这么做的，
// CSS:318-320 的注释写着"compact circular icons whose visible copy lives in a tooltip"）。
// 忙碌时官方 `disabled` → `opacity:0.4`（CSS:342-345），这里同样降透明度。
func (p *Plugin) drawActionButton(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	name, label string, busy bool) bool {
	btn := ui.Row(f).Size(cordisActionButton, cordisActionButton).
		Radius(cordisActionButtonRadius).Items(ui.Center).Justify(ui.Center).
		Hover(ui.Look{}.Background(t.Hover)).
		Hit(env.Tree.ID(env.Feat, name), label)
	if busy {
		btn = btn.Opacity(0.4)
	}
	btn = btn.Children(func() {
		// 图标槽 14×14（官方四处 `<Icon… size={14}/>`）。本仓没有 play-stop-trash
		// 三枚官方 sprite，按任务书只保留槽宽、不手画 SVG（差异表第 3 条）。
		ui.Row(f).Size(cordisActionIcon, cordisActionIcon).Shrink(0)
	})
	return btn.Clicked()
}

// drawDot 画官方 `StateDot` 的本仓等价物（10px 实心圆，见 geometry.go 的 cordisDotSize）。
func (p *Plugin) drawDot(f ui.C, color ui.Color) {
	ui.Row(f).Size(cordisDotSize, cordisDotSize).Radius(gfx.RadiusFull).
		Background(color).Shrink(0)
}

// panelAnchor 返回面板的绝对定位（左缘、与窗口底的距离），单位 px。
//
// 官方 `useLayoutEffect` 里量触发器矩形后算 `left = rect.left`、
// `bottom = window.innerHeight - rect.top + 8`（CordisPanel.tsx:140-151）。
// viva 同帧拿不到刚布的矩形（`Box.Rect()` 要等布局解算），所以用**上一帧**触发器
// 的真实矩形（`afterTrigger` 缓存，`plugins/dsagentteam/model_slot.go` 同一做法）；
// 还没画过才落 geometry.go 的 derived 兜底值。
func (p *Plugin) panelAnchor(env *pluginapi.Env) (left, bottom float32) {
	rect, ok := p.lastTriggerRect()
	if ok && rect.Width > 0 && rect.Height > 0 {
		height := env.Bounds.Height
		return rect.X, height - rect.Y + cordisPanelAnchorGap
	}
	return cordisPanelFallbackLeft, cordisPanelFallbackBottom
}
