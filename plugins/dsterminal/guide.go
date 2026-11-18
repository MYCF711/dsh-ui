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
//   - 官方那枚 `PluginArtworkTerminal` 已登记：`plugins/dsassets.PluginArtworkTerminal`
//     的 path/viewBox/stroke 逐字取自 rc.2 前端 bundle，卡片按官方 26px 画。
//   - 官方的 shell 菜单是 primitives `Menu`（portal 到窗口层）。Viva v0.6.4 有两个
//     入口都不合用：`frame.Popup()` 不导出到 `ui.C`（插件无法声明 popup），
//     `ui.Menu` 又把菜单面阴影硬编码成 `gfx.ShadowMD`（会丢掉官方
//     `--dsw-elevation-prominent` + `border-l1`）。所以仍手绘面板并用官方令牌画阴影；
//     代价（占布局、无方向键/外部点击关闭）与 Viva issue 草稿见
//     `docs/official-sidebar-terminal-close-rc2.md` §5。

import (
	"fmt"
	"strings"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
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

	// 卡片右下的 `ShortcutKeys`（TerminalGuide.tsx:81）plain 档：
	// `._keys_38b9q_1{display:inline-flex;gap:3px;color:label-tertiary;
	// font-size:12px;line-height:16px;white-space:nowrap}`
	guideShortcutGap  float32 = 3  // `.keys{gap:3px}`
	guideShortcutFont float32 = 12 // `.keys{font-size:12px}`
	guideShortcutLine float32 = 16 // `.keys{line-height:16px}`

	// shell 菜单面的度量：官方 `.menu` 只有 `display:flex;flex:none`
	// （TerminalGuide.module.css:11），面本身是 primitives 的 `Menu`——
	// 面 padding 4 / 行高 34 / 行 padding-inline 8 / 13px-20 取仓库已冻结的官方菜单档
	// （internal/uikernel/official_model_menu.go:64-80），锚点间距 4（官方 anchor+4）。
	guideMenuPad       float32 = 4
	guideMenuGap       float32 = 4
	guideMenuRowH      float32 = 34
	guideMenuRowPadX   float32 = 8
	guideMenuRowGap    float32 = 6
	guideMenuFont      float32 = 13
	guideMenuLine      float32 = 20
	guideMenuAnchorGap float32 = 4
)

// terminalNewCommandID 是官方终端快捷键的命令 id（`index.ts:56` 的
// `id: 'terminal.new'`）：向导卡用它向壳要 `ShortcutKeys` 的 keys。
const terminalNewCommandID = "terminal.new"

// ── shell 菜单 ───────────────────────────────────────────────────────────
//
// 官方 `.menu` 只有 `display:flex;flex:none`（TerminalGuide.module.css:11），面本身是
// primitives 的 `Menu`（portal 到窗口层）。Viva 的内部 popup 与 `ui.Menu` 都不可直接
// 用于"插件 + 官方阴影令牌"：前者不导出，后者阴影写死 `gfx.ShadowMD`。本仓保留手绘
// 面板，度量取仓库已冻结的官方菜单档，阴影用 `dscore.PaintElevation` 的官方令牌。

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
			// `.icon`（:4）：26×26。官方 `PluginArtworkTerminal` 是固定色 #679EFE 的
			// 36-box 描边字形（见 dsassets.PluginArtworkTerminal 的出处）。
			ui.Row(f).Width(guideIconBox).Height(guideIconBox).Shrink(0).
				Items(ui.Center).Justify(ui.Center).Children(func() {
				ui.Icon(f, p.guideIcon()).Size(guideIconBox, guideIconBox).
					Color(dsassets.TerminalArtworkStroke())
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
			// `.shortcut{position:relative;pointer-events:none}`（:12）：卡片最右。
			p.drawGuideShortcut(f, env, t)
		})
		// 官方的 Menu 是 portal（TerminalGuide.tsx:62-77）；本仓的手绘面板排在卡片
		// 正下方（占布局），原因与 Viva 限制见 drawGuideMenu 的注释。
		p.drawGuideMenu(f, env, t)
	})
}

// drawGuideShortcut 画官方 `ShortcutKeys`（TerminalGuide.tsx:81）的 plain 档。
//
// 官方结构（bundle 的 `Sp`）：外层 `<span class=keys>`，每个元素一枚 `<kbd>`，
// `"+"` 元素走 separator 样式。plain 档的 `_key_` 没有自己的尺寸/底色，
// 所以这里就是一段 12px/16 的文字行，间距 3。
func (p *Plugin) drawGuideShortcut(f ui.C, env *pluginapi.Env, t *pluginapi.Theme) {
	if env.Act.ShortcutKeys == nil {
		return
	}
	keys := env.Act.ShortcutKeys(terminalNewCommandID)
	if len(keys) == 0 {
		return
	}
	ui.Row(f).Shrink(0).Items(ui.Center).Gap(guideShortcutGap).Children(func() {
		for _, key := range keys {
			ui.Text(f, key).FontSize(guideShortcutFont).LineHeight(guideShortcutLine).
				MaxLines(1).Color(t.LabelTertiary)
		}
	})
}

// guideIcon 返回官方 `PluginArtworkTerminal`；素材缺失时退回本包已登记的终端字形。
//
// 官方 path/viewBox/stroke 在 dsassets 里登记（`dsh-web-frontend/dist/assets/
// index-ghOpC9uV.js` 的 xK 组件）；退回那一条只在素材解析失败时走，不改变几何：
// 仍按 26×26 画。
func (p *Plugin) guideIcon() gfx.Icon {
	if icon, ok := dsassets.PluginArtworkTerminal(); ok {
		return icon
	}
	return terminalTitleIcon()
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
// 官方是 primitives 的 `Menu`：portal 到窗口层、`align="end"`、选中项打勾、
// 四态 loading / ready / failed+retry / empty（TerminalGuide.tsx:50-54,62-77）。
//
// 本仓为什么仍然手绘：Viva v0.6.4 有两个现成入口，都不合用——
//
//   - `frame.Popup()` 能把菜单变成不占布局的浮层，但它是 frame 内部方法，
//     插件拿到的 `ui.C` 上**没有**对应导出（`ui/page.go` 的 Ctx 只导出
//     Key/KeyInt/Part/After/Theme/Bounds/Canvas/ID/PopupLayer），插件无法声明。
//   - `ui.Menu` 能用，但它的菜单面把阴影**硬编码**成 `gfx.ShadowMD`
//     （`viva@v0.6.4/components/dropdown_menu.go:772-774`，Tailwind 阶梯），
//     官方这一档是 `--dsw-elevation-prominent` + `--dsw-alias-border-l1`；
//     用它就等于把官方阴影令牌静默退回 Viva 预设（`plugins/dscore` 的阴影守卫
//     会红，红得对）。
//
// 所以保持手绘面板：材质/圆角/行高逐条取仓库已冻结的官方菜单档，阴影用
// `dscore.PaintElevation(…, Shadows.Prominent, BorderL1)`（官方菜单原语的
// `--dsw-elevation-prominent` + `--dsw-alias-border-l1`）。代价是它作为卡片列的
// 子元素会占布局（打开时把下面内容往下推），且没有 `ui.Menu` 自带的方向键/
// 外部点击关闭——这两点登记在 #54 的"仍未闭合"里。
func (p *Plugin) drawGuideMenu(f ui.C, env *pluginapi.Env, t *pluginapi.Theme) {
	if !p.guideMenuOpen || env.Act.TerminalLaunchShells == nil {
		return
	}
	choices := env.Act.TerminalLaunchShells()
	rows := guideMenuRows(choices, env, p)
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
				// 官方 Menu 的 `selectedId` 给选中项打勾（MenuSurface 的选中标记）。
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

// RightTabRename 实现官方 TerminalTitle 的 write 那一半：双击芯片改名。
//
//	OFF:dsh-client-ui-sidebar-terminal/src/client/TerminalTitle.tsx:55-59
//	  `onBlur` → `void view(tab.id).rename(next)`；`view(...).rename` 是
//	  `webTerminals` 的视图动作，最终调宿主 `terminal.rename`
//	  （`dsh-api-terminal-controller/src/client/model.ts` 的 `rename`）。
//
// kind 不是 terminal 时返回 owned=false（壳继续问下一个实现者），与
// `CloseRightTab` 的约定一致。宿主失败原样回错，界面显示在状态栏——
// 不本地改标题、不假装成功。
func (p *Plugin) RightTabRename(sessionID, tabID, kind, title string) (bool, error) {
	if kind != RightTabTerminal {
		return false, nil
	}
	if p.act.TerminalRenameOfficial == nil {
		return true, fmt.Errorf("用户终端 rename 未接线")
	}
	return true, p.act.TerminalRenameOfficial(tabID, title)
}
