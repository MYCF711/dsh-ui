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
//     的 path/viewBox/stroke 逐字取自官方前端 bundle，卡片按官方 26px 画
//     （`TerminalGuide.tsx:58` 的 `size={description === undefined ? 22 : 26}`：
//     没有描述时官方只画 22，本仓按同一个判据取 `guideIconSize`）。
//   - 官方的 shell 菜单是 primitives `Menu`（`TerminalGuide.tsx:62-77`，
//     `open={open} portal autoFocus align="end"`）。本仓用 Viva 的 `ui.Menu`：
//     同样是不占布局的浮层，点外部/Esc 关闭、键盘可走（`ui/menu.go:369-376` 原文
//     「Hits, focus, the keyboard, dismissal and submenu opening stay the menu's」）。
//     面与行由 `plugins/dscore` 画（`Unpainted` + `DrawPanel` + `DrawItem`）：官方菜单面
//     是 `--dsw-elevation-prominent` + `border-l1`，而 Viva 的默认面把阴影写死
//     `gfx.ShadowMD`。仍缺的两处读数登记在 `docs/official-sidebar-terminal-close-rc2.md`
//     §5.1：`align="end"` 的对齐（Viva 无此出口）与每行行高（原语 CSS 不在解包树内）。

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

	// `.icon` 里那枚 `PluginArtworkTerminal size={description === undefined ? 22 : 26}`
	// （TerminalGuide.tsx:58）：有描述画 26，没有描述画 22。
	guideIconBoxCompact float32 = 22

	// 卡片右下的 `ShortcutKeys`（TerminalGuide.tsx:81）plain 档：
	// `._keys_38b9q_1{display:inline-flex;gap:3px;color:label-tertiary;
	// font-size:12px;line-height:16px;white-space:nowrap}`
	guideShortcutGap  float32 = 3  // `.keys{gap:3px}`
	guideShortcutFont float32 = 12 // `.keys{font-size:12px}`
	guideShortcutLine float32 = 16 // `.keys{line-height:16px}`

	// shell 菜单的行几何：官方 `.menu` 只有 `display:flex;flex:none`
	// （TerminalGuide.module.css:11），面与行都是 primitives `Menu` 的，
	// 读数与画笔在 `plugins/dscore/menu.go`（forge 范围菜单同一套）。
	// 行高**不设**：读数取得到（dense 30 / base 34），但 Viva 的
	// `DropdownMenuOptions` 没有行高出口（`ui/menu_paint.go` 的 `rowExtent`
	// 只在右键菜单分支读 `GetRowSize()`），改它要动上游共享契约。
	// 账本 #54 ② 与 #64 ⑧-a 是同一条判定。
	guideMenuRowPadX = dscore.MenuRowPadX
	guideMenuFont    = dscore.MenuRowFont
	guideMenuLine    = dscore.MenuRowLine
	guideCheckSize   = dscore.MenuCheckSize
	guideMenuGap     = dscore.MenuRowGap
)

// terminalNewCommandID 是官方终端快捷键的命令 id（`index.ts:56` 的
// `id: 'terminal.new'`）：向导卡用它向壳要 `ShortcutKeys` 的 keys。
const terminalNewCommandID = "terminal.new"

// ── shell 菜单 ───────────────────────────────────────────────────────────
//
// 官方 `.menu` 只有 `display:flex;flex:none`（TerminalGuide.module.css:11），面本身是
// primitives 的 `Menu`：`open={open} portal autoFocus align="end"`，锚点是 `.trigger`
// 那个 ghost 按钮，`items` 由三态投影而来、`selectedId` 给当前 shell 打勾
// （TerminalGuide.tsx:62-77）。本仓用 `ui.Menu` 复刻同一形状，面与行由
// `plugins/dscore` 的官方画笔接管（见该文件头）。

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
	// chosen 对应官方 `Menu` 的 `selectedId`：当前记住的那个 shell（按 path 比）。
	chosen bool
	pick   func()
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
			// `.icon`（:4）：26×26；官方 `size={description === undefined ? 22 : 26}`
			// （TerminalGuide.tsx:58），没有描述时画 22。官方 `PluginArtworkTerminal`
			// 是固定色 #679EFE 的 36-box 描边字形（见 dsassets.PluginArtworkTerminal 的出处）。
			iconBox := guideIconBox
			if props.Description == "" {
				iconBox = guideIconBoxCompact
			}
			ui.Row(f).Width(guideIconBox).Height(guideIconBox).Shrink(0).
				Items(ui.Center).Justify(ui.Center).Children(func() {
				ui.Icon(f, p.guideIcon()).Size(iconBox, iconBox).
					Color(dsassets.TerminalArtworkStroke())
			})
			// `.text`（:5）：flex:1 的竖列。
			ui.Column(f).Grow(1).Shrink(1).Gap(guideTextGap).Children(func() {
				// `.titleRow`（:6）：标题 + 展开菜单的小箭头。官方 `Menu` 的 `anchor`
				// 就是这个 `.trigger` 按钮（TerminalGuide.tsx:73-76），所以浮层必须在
				// 同一次声明里锚在它身上。
				ui.Row(f).FillWidth().Items(ui.Center).Gap(guideTitleRowGap).Children(func() {
					ui.Text(f, props.Title).FontSize(guideTitleFont).LineHeight(guideTitleLine).
						MaxLines(1).Ellipsis().Grow(1).Color(t.LabelPrimary)
					trigger := p.drawGuideTrigger(f, env, t)
					p.drawGuideMenu(f, env, t, trigger)
				})
				if props.Description != "" {
					ui.Text(f, props.Description).FontSize(guideDescFont).
						LineHeight(guideDescLine).MaxLines(1).Ellipsis().Color(t.LabelTertiary)
				}
			})
			// `.shortcut{position:relative;pointer-events:none}`（:12）：卡片最右。
			p.drawGuideShortcut(f, env, t)
		})
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

// drawGuideTrigger 画 `.trigger`（TerminalGuide.module.css:9-10）：20×20、圆角 4、
// 14px 的 chevron、三级字色。
//
// 返回它自己的 `ui.Box`：官方 `Menu` 的 `anchor` 就是这个按钮（TerminalGuide.tsx:73-76），
// 调用方拿它当浮层锚点。
//
// # 悬停态是原语 ghost Button 的，不是自造的
//
// 官方那个 anchor 是 primitives 的 `<Button variant="ghost" className={css.trigger}>`
// （TerminalGuide.tsx:73），而 `.trigger` 自己**没有**任何 hover/active 规则
// （`TerminalGuide.module.css` 全文 12 行，只有 `flex/height/width/padding/border-radius`
// 与 `.trigger svg` 的静态 `color: label-tertiary`）。悬停底来自 primitives：
//
//	._ghost_1ulci_46:hover:not(:disabled)  { background: var(--dsw-alias-interactive-bg-hover) }   ← byte 8644
//	._ghost_1ulci_46:active:not(:disabled) { background: var(--dsw-alias-interactive-bg-active) }  ← byte 8732
//
// 两项本仓都有等价令牌（`t.Hover` 就是 `--dsw-alias-interactive-bg-hover`、
// `t.InteractiveActive` 就是 `--dsw-alias-interactive-bg-active`），所以复用，
// 不新造。原先这里还会把 chevron 从 label-tertiary 换成 label-primary —— 官方
// `.trigger svg` 的字色是**静态**的，没有这条，已回正。
func (p *Plugin) drawGuideTrigger(f ui.C, env *pluginapi.Env, t *pluginapi.Theme) ui.Box {
	var id gfx.ID
	if env.Tree != nil {
		id = env.Tree.ID(env.Feat, "terminal-guide-shell")
	}
	button := ui.Row(f).Width(guideTriggerSize).Height(guideTriggerSize).Shrink(0).
		Radius(guideTriggerRadius).Items(ui.Center).Justify(ui.Center).
		Hover(ui.Look{}.Background(t.Hover)).
		Pressed(ui.Look{}.Background(t.InteractiveActive)).
		Hit(id, zhGuideShell)
	button.Children(func() {
		// `.trigger svg{width:14px;height:14px;color:var(--dsw-alias-label-tertiary)}`
		// （TerminalGuide.module.css:10）：字色恒定，不随悬停换。
		ui.Icon(f, terminalChevronDownIcon()).Size(guideChevronSize, guideChevronSize).Color(t.LabelTertiary)
	})
	if button.Clicked() {
		p.guideMenuOpen = !p.guideMenuOpen
	}
	return button
}

// drawGuideMenu 画官方的 shell 菜单：**锚在 `.trigger` 上的浮层**。
//
// 官方形状（TerminalGuide.tsx:62-77）：
//
//	<Menu open portal autoFocus align="end" className={css.menu}
//	      items={…} selectedId={state.choices.selectedShell} onClose onSelect
//	      anchor={<Button variant="ghost" className={css.trigger}>…</Button>} />
//
// 三态投影与官方 `items` 逐条一致（`TerminalGuide.tsx:64` 的 `items.length === 0` 也算）：
//
//	ready   → 每枚 shell 一行（`{id: shell.path, label: shell.name}`）；空表 = `shellEmpty` 禁用行
//	failed  → 错误行（禁用）+ `retry` 行
//	其余    → `shellLoading` 禁用行
//
// `selectedId` 比的是 **shell.path**（官方 `items` 的 id 就是 `shell.path`），
// 所以打勾判据是 `choices.Selected == shell.Path`，不是标签相等。
//
// 面与行交给 `plugins/dscore`（`Unpainted` + `DrawPanel` + `DrawItem`）：官方菜单面是
// `--dsw-elevation-prominent` + `border-l1`，Viva 的默认面把阴影写死 `gfx.ShadowMD`。
// 行为（命中、焦点、键盘、Esc、点外关闭）仍归菜单自己（`ui/menu.go:369-376` 原文）。
func (p *Plugin) drawGuideMenu(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, anchor ui.Box) {
	if env.Act.TerminalLaunchShells == nil {
		return
	}
	choices := env.Act.TerminalLaunchShells()
	rows := guideMenuRows(choices, env, p)
	ui.Menu(f, anchor, &p.guideMenuOpen, func(items *ui.MenuItems) {
		if items == nil {
			return
		}
		items.Unpainted().
			DrawPanel(func(canvas *gfx.Canvas, bounds gfx.Rect, _ gfx.Theme) {
				dscore.PaintMenuPanel(canvas, bounds, t)
			}).
			DrawItem(func(parts ui.MenuItemParts) { dscore.PaintMenuItem(parts, t) })
		for _, row := range rows {
			row := row
			item := items.Item(row.label).Chosen(row.chosen).Disabled(row.disabled)
			if !row.disabled && row.pick != nil && item.Clicked() {
				row.pick()
			}
		}
	})
}

// guideMenuRows 把三态投影成官方 Menu 的 items[]（TerminalGuide.tsx:50-54,64）：
//
//	ready   → 每枚 shell 一行；空表 = `shellEmpty` 禁用行
//	failed  → 错误行（禁用）+ `retry` 行
//	其余    → `shellLoading` 禁用行
//
// `chosen` 是官方 `selectedId` 的投影：官方 `items` 的 id 是 `shell.path`、
// `selectedId` 也是 path（`dsh-api-terminal-controller/src/client/index.ts:169-191`
// 的 `selectedShell: …find(s => s.path === previous)?.path`），所以按 path 比。
func guideMenuRows(choices pluginapi.TerminalShellChoices, env *pluginapi.Env, p *Plugin) []guideMenuRow {
	switch choices.Phase {
	case pluginapi.TerminalShellsReady:
		rows := make([]guideMenuRow, 0, len(choices.Shells))
		for _, shell := range choices.Shells {
			shell := shell
			label := shell.Name
			if label == "" {
				// 官方直接用 `shell.name`；宿主没给名字时本仓退回 path，好过一行空白。
				label = shell.Path
			}
			rows = append(rows, guideMenuRow{
				label:  label,
				chosen: choices.Selected != "" && choices.Selected == shell.Path,
				pick: func() {
					// 官方先 `selectShell(path)`（记住选择），再带 `shellPath` 打开新标签
					// （TerminalGuide.tsx:69-71）。
					if env.Act.TerminalSelectShell != nil {
						env.Act.TerminalSelectShell(shell.Path)
					}
					p.guideOpenTerminal(env, shell.Path)
				},
			})
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
