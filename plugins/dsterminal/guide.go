package dsterminal

// guide.go —— 右栏向导页上的**终端入口卡**（官方 `TerminalGuide`）与终端芯片徽标。
//
// # 官方事实
//
// 终端这个 tab 类型在自己的注册里带一张向导卡：
//
//	OFF:dsh-client-ui-sidebar-terminal/src/client/index.ts:72-75
//	  ctx.sidebarRightTabs.register({ id, kind: 'terminal', multiple: true, priority: 'builtin',
//	    title: () => t('title'),
//	    guide: [{ id: 'new', order: 20, title: () => t('new'), description: () => t('description'),
//	              icon: PluginArtworkTerminal }] })
//
// 卡片本身不是普通胶囊：它注册进 **keyed** 座位 `sidebar.right.tab.guide.entry`
// （key = 提供方 id），自己画图标 / 标题 / 描述 / 快捷键，外加一枚**展开 shell 菜单**的
// 小箭头：
//
//	OFF:…/src/client/index.ts:87-94
//	  ctx.slots.inject('sidebar.right.tab.guide.entry', () => ctx.slots.register({
//	    name: 'sidebar.right.tab.guide.entry', key: id, locale: namespace,
//	    inject: sessionId => ({ hooks: { shortcuts: ctx.shortcuts.catalog },
//	      loadShells: signal => ctx.webTerminals.launchShells(sessionId, signal),
//	      selectShell: path => { ctx.webTerminals.selectShell(path) } }),
//	  }, TerminalGuide))
//	OFF:…/src/client/TerminalGuide.tsx:32-83（卡片本体）
//
// # 几何（逐条来自 TerminalGuide.module.css:1-12）
//
//	.entry      position:relative; display:flex; align-items:center; gap:14px; width:100%;
//	            min-height:56px; padding:14px 20px; overflow:hidden;
//	            border:.5px solid border-l3; border-radius:var(--dsl-guide-entry-radius);
//	            background:bg-layer-1
//	.main       position:absolute; inset:0（整张卡片的点击面）
//	.icon       width:26px; height:26px; color:label-secondary
//	.text       flex:1; column; gap:3px; min-width:0
//	.titleRow   flex; align-items:center; min-width:0; gap:4px
//	.title      font-size:14px; line-height:1.4; label-primary; 单行省略
//	.description font-size:11px; line-height:1.4; label-tertiary; 单行省略
//	.trigger    flex:none; height:20px; width:20px; padding:0; border-radius:4px
//	.trigger svg width:14px; height:14px; color:label-tertiary
//	.menu       display:flex; flex:none（面本身是 primitives 的 MenuSurface）
//
// `--dsl-guide-entry-radius` 由向导页给：`var(--dsw-radius-xl)`（GuideBody.module.css:8），
// 在 pluginapi.Theme 里就是 `Radii.XL`。
//
// # 本仓的边界（如实登记）
//
//   - 官方那枚 `PluginArtworkTerminal` **没有登记素材**：卡片图标复用本包已登记的
//     16-box 终端字形（`TerminalIcon.tsx`），尺寸仍按官方 26px 画。这条差异写在
//     docs/official-parity.md 的 #54 行。
//   - 官方的 shell 菜单是 primitives `Menu`（portal 到窗口层）；Viva 没有门户层，
//     本仓把它画成贴住 trigger 的一块绝对定位面板，面的材质/圆角/内边距/行高逐条取
//     仓库已冻结的官方菜单度量（见下面常量表的出处）。

import (
	"strings"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 官方几何（TerminalGuide.module.css:1-12）──────────────────────────────
const (
	guideEntryGap       float32 = 14 // `.entry{gap:14px}`
	guideEntryMinHeight float32 = 56 // `.entry{min-height:56px}`
	guideEntryPadY      float32 = 14 // `.entry{padding:14px 20px}`
	guideEntryPadX      float32 = 20
	guideEntryBorder    float32 = 0.5 // `.entry{border:.5px solid …}`
	guideIconBox        float32 = 26  // `.icon{width:26px;height:26px}`
	guideTextGap        float32 = 3   // `.text{gap:3px}`
	guideTitleRowGap    float32 = 4   // `.titleRow{gap:4px}`
	guideTitleFont      float32 = 14  // `.title{font-size:14px;line-height:1.4}`
	guideTitleLine      float32 = 14 * 1.4
	guideDescFont       float32 = 11 // `.description{font-size:11px;line-height:1.4}`
	guideDescLine       float32 = 11 * 1.4
	guideTriggerSize    float32 = 20 // `.trigger{height:20px;width:20px}`
	guideTriggerRadius  float32 = 4  // `.trigger{border-radius:4px}`
	guideChevronSize    float32 = 14 // `.trigger svg{width:14px;height:14px}`
)

// ── shell 菜单面的度量 ───────────────────────────────────────────────────
//
// 官方 `.menu` 只有 `display:flex;flex:none`（TerminalGuide.module.css:11），面本身是
// primitives 的 `MenuSurface`。本仓取**仓库已冻结的同一份官方菜单度量**，不另编数值：
//
//	面   padding 4 / radius md / 材质 --dsw-specific-menu / 阴影 elevation
//	     （OFF:ui-model-selection/ModelSelect.module.css:132 的 `.menu{padding:4px}`，
//	      已落在 internal/uikernel/official_model_menu.go:64-66、:68）
//	行   height 34 / padding-inline 8 / gap 6 / 13px-20
//	     （同文件 `.cell`，`official_model_menu.go:74-80`）
//	锚点间距 4（internal/uikernel/entry_menu.go 的 EntryMoreMenuAnchorGap）
const (
	guideMenuPad     float32 = 4
	guideMenuGap     float32 = 4
	guideMenuRowH    float32 = 34
	guideMenuRowPadX float32 = 8
	guideMenuRowGap  float32 = 6
	guideMenuFont    float32 = 13
	guideMenuLine    float32 = 20
	// 菜单面与卡片之间的间距：官方 Menu 贴锚点下方 4（`anchor(bottom) + 4px`）。
	guideMenuAnchorGap float32 = 4
)

// 官方文案（OFF:…/sidebar-terminal/src/client/locales.ts，中文逐字）。
const (
	zhGuideTitle       = "新建终端"       // 'new'（:15）
	zhGuideDescription = "在会话工作区运行命令" // 'description'（:14）
	zhGuideShell       = "选择 Shell"   // 'shell'（:14）
	zhGuideShellLoad   = "正在读取 Shell…"
	zhGuideShellEmpty  = "没有可用的 Shell"
	zhGuideRetry       = "重试"    // 'retry'（:19）
	zhGuideFailedLead  = "终端错误：" // 'failed'（:18）的模板前缀
	// 徽标（locales.ts:26-29）。
	zhBadgeAgent        = "Agent"
	zhBadgeAgentHint    = "由 Agent 打开。你一输入就由你接管，Agent 之后只能读取。"
	zhBadgeSandbox      = "沙箱"
	zhBadgeReadOnlyHint = "命令在 Agent 的只读沙箱里运行，包括你在这里输入的命令。"
	zhBadgeWriteHint    = "命令在 Agent 的工作区沙箱里运行，包括你在这里输入的命令。"
)

// guideEntryPainterKey 是向导卡在 keyed 座位里的 key = 类型注册的 `Painter.Key`。
//
// 官方那里是 `entry.providerId`（`SidebarRightTabDefinition.id`，允许 extension 顶掉
// builtin）；本仓还没有 priority 覆盖机制（见 docs/official-view-tabs-evidence-rc2.md
// 的 G2），类型的实现身份就是它的 kind，所以两者取同一个值。
const guideEntryPainterKey = RightTabTerminal

// guideMenuRow 是菜单里的一行（官方 Menu 的 `items[]` 一项）。
type guideMenuRow struct {
	label    string
	disabled bool
	pick     func()
}

// RightTabBadge 实现官方 TerminalTitle 里那枚 `Tag`。
//
//	OFF:…/TerminalTitle.tsx:17-20,44-50
//	  agentControl = info.origin==='agent' && info.control==='agent'
//	  sandboxMode  = info.origin==='agent' ? info.sandboxMode : undefined
//	  agentControl → <Tag tone="info">Agent</Tag>（title = badge.agentHint[+ 沙箱提示]）
//	  sandboxMode  → <Tag tone="neutral">沙箱</Tag>（title = badge.readOnlyHint / badge.workspaceWriteHint）
//	  一次只有一枚：Agent 优先，沙箱是「被 Agent 打开、还没被接管」那一档
//
// 数据源是本宿主 `terminal.list|attach` 的真实回包（`internal/apiclient.TerminalInfo` 的
// origin / control / sandboxMode，与 rc.2 `types.ts:53-70` 同形）。
func (p *Plugin) RightTabBadge(sessionID, tabID string) (pluginapi.TabBadge, bool) {
	if p.act.TerminalSnapshot == nil {
		return pluginapi.TabBadge{}, false
	}
	info := p.act.TerminalSnapshot(tabID).Info
	if info == nil || info.Origin != "agent" {
		return pluginapi.TabBadge{}, false
	}
	sandboxHint := ""
	switch info.SandboxMode {
	case "read-only":
		sandboxHint = zhBadgeReadOnlyHint
	case "workspace-write":
		sandboxHint = zhBadgeWriteHint
	}
	if info.Control == "agent" {
		hint := zhBadgeAgentHint
		if sandboxHint != "" {
			hint = zhBadgeAgentHint + " " + sandboxHint
		}
		return pluginapi.TabBadge{Text: zhBadgeAgent, Tone: pluginapi.TabBadgeInfo, Hint: hint}, true
	}
	if sandboxHint == "" {
		// origin==='agent' 但既没控制权、沙箱模式也不认识：官方两枚徽标都不画。
		return pluginapi.TabBadge{}, false
	}
	return pluginapi.TabBadge{Text: zhBadgeSandbox, Tone: pluginapi.TabBadgeNeutral, Hint: sandboxHint}, true
}

// drawGuideEntry 画官方 `TerminalGuide` 卡片（`sidebar.right.tab.guide.entry` 的占用方）。
func (p *Plugin) drawGuideEntry(f ui.C, env *pluginapi.Env) {
	if env == nil || env.Theme == nil {
		return
	}
	props := env.GuideEntry
	if props == nil {
		// 官方这个组件只从老板 props 取 entryId / kind / title / description
		// （`TerminalGuideProps`，TerminalGuide.tsx:22,32-34）；壳没给就不该画。
		return
	}
	t := env.Theme
	cardID := gfx.ID{}
	if env.Tree != nil {
		cardID = env.Tree.ID(env.Feat, "terminal-guide-entry")
	}
	// `.entry`（:1）：整张卡片就是那个 flex 行，`overflow:hidden` 让圆角生效。
	ui.Column(f).FillWidth().Children(func() {
		card := ui.Row(f).FillWidth().MinHeight(guideEntryMinHeight).Shrink(0).
			PadSides(guideEntryPadY, guideEntryPadX, guideEntryPadY, guideEntryPadX).
			Gap(guideEntryGap).Items(ui.Center).
			Background(t.BgLayer1).Border(guideEntryBorder, t.BorderL3).
			Radius(t.Radii.XL).Clip()
		card.Children(func() {
			// `.main{position:absolute;inset:0}`：整张卡片的点击面（官方是覆盖在卡片上的
			// ghost 按钮，所以点标题/空白都算「打开终端」）。**必须先声明**：viva 同层内
			// 后声明的赢，先声明它才能让后面的 trigger 拿到自己的点击。
			if !p.guideMenuOpen {
				if hit := (ui.Row(f).Absolute().Inset(0).Hit(cardID, props.Title)); hit.Clicked() {
					p.guideOpenTerminal(env, "")
				}
			}
			// `.icon`（:4）：26×26，次要色。官方那枚 `PluginArtworkTerminal` 没有登记素材，
			// 这里用本包已登记的终端字形（TerminalIcon.tsx）按 26 画，差异见包注释。
			ui.Row(f).Width(guideIconBox).Height(guideIconBox).Shrink(0).
				Items(ui.Center).Justify(ui.Center).Children(func() {
				ui.Icon(f, terminalTitleIcon()).Size(guideIconBox, guideIconBox).Color(t.LabelSecondary)
			})
			// `.text`（:5）：flex:1 的竖列。
			ui.Column(f).Grow(1).Shrink(1).Gap(guideTextGap).Children(func() {
				// `.titleRow`（:6）：标题 + 展开菜单的小箭头。
				ui.Row(f).FillWidth().Items(ui.Center).Gap(guideTitleRowGap).Children(func() {
					ui.Text(f, props.Title).FontSize(guideTitleFont).LineHeight(guideTitleLine).
						MaxLines(1).Ellipsis().Grow(1).Color(t.LabelPrimary)
					p.drawGuideTrigger(f, env, t)
				})
				if props.Description != "" {
					ui.Text(f, props.Description).FontSize(guideDescFont).
						LineHeight(guideDescLine).MaxLines(1).Ellipsis().Color(t.LabelTertiary)
				}
			})
		})
		p.drawGuideMenu(f, env, t)
	})
}

// guideOpenTerminal 打开一个终端 tab。
//
// 官方 `TerminalGuide.tsx:57`（整卡）与 `:71`（选中 shell 后）都是
// `tab.actions.openTab('terminal', { replaceTab: true, params?: { shellPath } })`：
// 卡片自己就在 guide tab 里，`replaceTab` 让新终端占掉 guide 的位置。
//
// 本仓的向导只在右栏没有 tab 时才出现，所以这里只需要「打开」；`shellPath` 走
// `Actions.OpenRightTabParams`（官方 `SidebarRightTabParamsMap` 的 terminal 那一支，
// face.ts:20-24），壳在 ensure 终端时消费它。
func (p *Plugin) guideOpenTerminal(env *pluginapi.Env, shellPath string) bool {
	if env == nil {
		return false
	}
	if shellPath != "" && env.Act.OpenRightTabParams != nil {
		env.Act.OpenRightTabParams(RightTabTerminal, pluginapi.RightTabParams{ShellPath: shellPath})
		p.guideMenuOpen = false
		return true
	}
	if env.Act.OpenRightTab != nil {
		env.Act.OpenRightTab(RightTabTerminal)
		p.guideMenuOpen = false
		return true
	}
	return false
}

// drawGuideTrigger 画 `.trigger`（:9-10）：20×20、圆角 4、14px 的 chevron、三级字色。
func (p *Plugin) drawGuideTrigger(f ui.C, env *pluginapi.Env, t *pluginapi.Theme) {
	var id gfx.ID
	if env.Tree != nil {
		id = env.Tree.ID(env.Feat, "terminal-guide-shell")
	}
	color := t.LabelTertiary
	if env.Canvas != nil && (env.Canvas.Hovered() == id || env.Canvas.Pressed() == id) {
		color = t.LabelPrimary
	}
	button := ui.Row(f).Width(guideTriggerSize).Height(guideTriggerSize).Shrink(0).
		Radius(guideTriggerRadius).Items(ui.Center).Justify(ui.Center).
		Hover(ui.Look{}.Background(t.Hover)).Hit(id, zhGuideShell)
	button.Children(func() {
		ui.Icon(f, terminalChevronDownIcon()).Size(guideChevronSize, guideChevronSize).Color(color)
	})
	if button.Clicked() {
		p.guideMenuOpen = !p.guideMenuOpen
	}
}

// drawGuideMenu 画官方的 shell 菜单面。
//
// 官方是 primitives 的 `Menu`（portal、贴锚点、选中项打勾、四态：
// loading / ready / failed+retry / empty，TerminalGuide.tsx:50-54,62-77）。
func (p *Plugin) drawGuideMenu(f ui.C, env *pluginapi.Env, t *pluginapi.Theme) {
	if !p.guideMenuOpen || env.Act.TerminalLaunchShells == nil {
		return
	}
	choices := env.Act.TerminalLaunchShells()
	rows := guideMenuRows(choices, env, p)
	// 官方菜单是 primitives `Menu`（portal 到窗口层，宽度 = max-content）。Viva 没有
	// 门户层，这里把菜单面排在卡片正下方（同一列、贴住卡片下沿）；宽度仍按官方
	// max-content 语义量出来，量不到（没有 canvas）才铺满卡片那一列。
	// 阴影 = 官方菜单原语的 `--dsw-elevation-prominent` + `--dsw-alias-border-l1`
	// 描边：官方 `TerminalGuide.module.css` 的 `.-4GESq_menu` 只有
	// `pointer-events:auto;flex:none;display:flex`（**没有** box-shadow），菜单面本身
	// 是 primitives `Menu`——`._list_70kcw_7,._submenu_70kcw_8
	// {--dsw-elevation-stroke-color:var(--dsw-alias-border-l1);
	//  box-shadow:var(--dsw-elevation-prominent)}`
	// （`dsh-web-frontend/dist/assets/index-CSAdOBx6.css`）；同档还有 MenuView 的
	// `.pc3rQa_menu{border:0;…box-shadow:var(--dsw-elevation-prominent)}`。
	// 原来的 `Shadow(gfx.ShadowLG)`（Tailwind `0 10px 15px -3px .10` 两层）不是这一档。
	panel := ui.Column(f).Shrink(0).Pad(guideMenuPad).Gap(guideMenuGap).
		MarginSides(guideMenuAnchorGap, 0, 0, 0).
		Background(t.Menu).Radius(t.Radii.MD).
		Draw(func(d *ui.DrawContext) {
			dscore.PaintElevation(d, t.Radii.MD, t.Shadows.Prominent, t.BorderL1)
		})
	if width, ok := guideMenuWidth(env, rows, choices.Selected); ok {
		panel = panel.Width(width)
	} else {
		panel = panel.FillWidth()
	}
	panel.Children(func() {
		for _, row := range rows {
			row := row
			color := t.LabelPrimary
			if row.disabled {
				color = t.LabelTertiary
			}
			var id gfx.ID
			if env.Tree != nil && !row.disabled {
				id = env.Tree.ID(env.Feat, "terminal-guide-shell-"+row.label)
			}
			item := ui.Row(f).FillWidth().Height(guideMenuRowH).Shrink(0).
				PadSides(0, guideMenuRowPadX, 0, guideMenuRowPadX).
				Gap(guideMenuRowGap).Items(ui.Center)
			if !row.disabled {
				item = item.Hover(ui.Look{}.Background(t.Hover)).Hit(id, row.label)
			}
			item.Children(func() {
				// 官方 Menu 用 `selectedId` 给选中项打勾（MenuSurface 的选中标记）。
				if choices.Phase == pluginapi.TerminalShellsReady && choices.Selected != "" &&
					choices.Selected == row.label {
					ui.Icon(f, terminalCheckIcon()).Size(guideChevronSize, guideChevronSize).
						Color(t.LabelPrimary)
				}
				ui.Text(f, row.label).FontSize(guideMenuFont).LineHeight(guideMenuLine).
					MaxLines(1).Ellipsis().Color(color)
			})
			if !row.disabled && item.Clicked() && row.pick != nil {
				row.pick()
			}
		}
	})
}

// guideMenuRows 把三态投影成官方 Menu 的 items[]（TerminalGuide.tsx:50-54）：
//
//	ready   → 每枚 shell 一行；空表 = `shellEmpty` 禁用行
//	failed  → 错误行（禁用）+ `retry` 行
//	其余    → `shellLoading` 禁用行
func guideMenuRows(choices pluginapi.TerminalShellChoices, env *pluginapi.Env, p *Plugin) []guideMenuRow {
	switch choices.Phase {
	case pluginapi.TerminalShellsReady:
		rows := make([]guideMenuRow, 0, len(choices.Shells))
		for _, shell := range choices.Shells {
			shell := shell
			label := shell.Name
			if label == "" {
				label = shell.Path
			}
			rows = append(rows, guideMenuRow{label: label, pick: func() {
				// 官方先 `selectShell(path)`（记住选择），再带 `shellPath` 打开新标签
				// （TerminalGuide.tsx:69-71）。
				if env.Act.TerminalSelectShell != nil {
					env.Act.TerminalSelectShell(shell.Path)
				}
				p.guideOpenTerminal(env, shell.Path)
			}})
		}
		if len(rows) == 0 {
			return []guideMenuRow{{label: zhGuideShellEmpty, disabled: true}}
		}
		return rows
	case pluginapi.TerminalShellsFailed:
		message := strings.TrimSpace(choices.Message)
		if message == "" {
			message = zhGuideShellLoad
		}
		return []guideMenuRow{
			{label: zhGuideFailedLead + message, disabled: true},
			{label: zhGuideRetry, pick: func() {
				// 官方 retry：`setState({phase:'loading'}); setAttempt(v => v+1)`
				// （TerminalGuide.tsx:68）→ 再问一次，壳对 failed 档会重新拉。
				if env.Act.TerminalLaunchShells != nil {
					_ = env.Act.TerminalLaunchShells()
				}
			}},
		}
	}
	return []guideMenuRow{{label: zhGuideShellLoad, disabled: true}}
}

// guideMenuWidth 按官方 `width:max-content` 的语义量出菜单面宽度
// （primitives MenuSurface：宽度随内容，没有固定值）：
//
//	最长一行的文字宽 + 行左右内边距 + 面左右内边距
//
// ok=false 表示这一帧量不到（没有 canvas），调用方退回铺满那一列。
func guideMenuWidth(env *pluginapi.Env, rows []guideMenuRow, selected string) (float32, bool) {
	if env == nil || env.Canvas == nil || len(rows) == 0 {
		return 0, false
	}
	widest := float32(0)
	for _, row := range rows {
		width := env.Canvas.TextWidth(row.label, guideMenuFont)
		// 选中的那一行前面多一枚 14px 的勾（官方 Menu 的 selectedId 标记），
		// 它和后面的 gap 也要算进 max-content。
		if selected != "" && selected == row.label {
			width += guideChevronSize + guideMenuRowGap
		}
		if width > widest {
			widest = width
		}
	}
	if widest <= 0 {
		return 0, false
	}
	return widest + 2*guideMenuRowPadX + 2*guideMenuPad, true
}
