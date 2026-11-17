// Package dsentrycustom 是官方 `dsh-client-ui-entry-customization` 的本仓等价物（rc.2 的 #17）：
// **三个区域 More 菜单里的那几行**。
//
// # 官方事实（rc.2 随包 src/**，逐行核对）
//
//	OFF:dsh-client-ui-entry-customization/src/client/index.ts:52-76
//	  同一份内容注册进两个（本仓可用的）list 座位：
//	    sidebar.panellist.actions                 左栏面板列表的 More
//	    conversation.session.header.views.actions 会话头 view tab 行的 More
//	  （第三处 `sidebar.right.tab.guide.actions` 本仓没有宿主，见文件尾 HIDE 段）
//	OFF:dsh-client-ui-entry-customization/src/client/EntryMenuContent.tsx:35-70
//	  moreEntries = region === 'sidebar' ? entries.filter(!pinned)
//	              : region === 'conversation' ? entries : []
//	  每行 = primitives 的 `MenuItemButton`（current = active），左栏行首还有一枚
//	  16px 面板图标（`renderFactorySlot('sidebar.panel.icon', …)`），行尾：
//	    左栏  → `.pinAnchor` 里的 22×22「固定」按钮（`IconPinOutlineRegular` 14）
//	    两侧  → active 时行尾对勾（`IconCheckOutlineRegular`，会话头那一侧恒可见）
//	  末尾两行：`browsable && 「浏览更多插件」`（`IconPluginPinwheelOutlineRegular` 16）、
//	          「创建你的插件」（`IconPlusOutlineRegular` 16）
//	OFF:.../src/client/locales.ts:13-21  zh 文案逐字：
//	  pinTooltip 固定到左侧栏 · pin 将{name}固定到侧边栏 · install 浏览更多插件 · create 创建你的插件
//
// # 行几何（都能在 rc.2 dist CSS 里查到数）
//
//	primitives MenuItemButton  `_item_70kcw_173`：
//	  display:flex / align-items:center / gap:6px / width:100% / min-height:34px /
//	  padding:6px 8px / border-radius:--dsw-radius-md / font-size:13px /
//	  line-height:20px / color:label-primary；`:hover` = interactive-bg-hover
//	图标框 `_itemIcon_70kcw_233{width:14px;height:14px}` + `_itemIcon svg{14×14}`
//	分离线 `_separator_70kcw_157{height:.5px;margin:3px 8px;background:border-l2}`
//	  （`separatorBefore` 只在**行前**画；官方 CSS 另有一条"是第一子元素就隐藏"的规则，
//	   本包靠 `moreEntries > 0` 这一个判据等价地满足它）
//	左栏行 .sidebarEntry / .pinAnchor / .pinButton（OFF:EntryMenu.module.css:5-16）：
//	  `.entryName{padding-right:28px}`、`.pinAnchor{right:6px;top:0;bottom:0}`、
//	  `.pinButton{width:22px;height:22px;padding:0;color:label-tertiary}`
//
// # 本仓的形状
//
// 宿主（`dssidebar` 左栏 / `dstopbar` 会话头）在派发座位时把 owner props 挂到
// `pluginapi.Env.EntryMenu` 上（`internal/uikernel.DrawEntryMoreMenu` 负责挂/恢复），
// 本包两个 painter 读它画行；`EntryMenu == nil` 时**一行都不画**（不在菜单里）。
//
// 「浏览更多插件」那行的开关是**服务存在性**：官方 `ctx.inject(['pluginNavigation'], …)`
// 只在 plugin-manager 在册时把 `browsable` 置真。本仓等价物是 ds-sidebar 提供的
// `plugins/dssidebar/panelapi`（`ui.pluginNavigation`），注入不到就整行不画。
//
// # HIDE（登记在案，不是"以后再说"）
//
//  1. **右栏 guide 的 More 卡片不注册**：`sidebar.right.tab.guide.actions` 本仓还没有
//     宿主（guide 面未落地，`docs/official-parity.md` 的 #53），注册了也没有派发方。
//  2. **键盘模态那半边（`html[data-input-modality='keyboard'] … :focus-within`）不画**：
//     它挂在同一串 CSS 里（OFF:EntryMenu.module.css:29,31），但本仓没有"全局输入模态"
//     这一位，而且它是 `:focus-within`（行内任一后代有焦点），与 viva 的
//     `FocusVisible()==id` 不是同一个判据，不冒充。
//
// # 悬停三态（2026-11-17 本轮补上，值全取官方）
//
// `@media (hover:hover)` 那一块（OFF:EntryMenu.module.css:20-32）定义了**行内三件
// 东西随行悬停一起变**：
//
//	行     `.sidebarEntry:hover > div:first-child > button`
//	       → `background:var(--dsw-alias-interactive-bg-hover)`（:21）
//	pin    `.pinAnchor{opacity:0}`（:26）→ `:hover .pinAnchor{opacity:1}`（:28）
//	       外加 `.pinButton:hover:not(:disabled){color:label-primary;background:transparent}`（:22-25）
//	对勾   `.activeMark{opacity:0}`（:12）→ `.sidebarEntry .activeMark{opacity:1}`（:27）
//	       → `.sidebarEntry:hover .activeMark{opacity:0}`（:30）
//
// 注意对勾是**反的**：静止时画（rest 1 由 :27 给，覆盖 :12 的 0），行一悬停就隐去。
// 所以左栏行尾**任何时刻都只有一枚** 22×22 的按钮位（pin 或对勾），两枚不并排。
//
// 实现用 viva 的声明式跟随：`Box.Opacity(...)` 给静止值，`Hover(ui.Look{}.Opacity(...))`
// 给悬停值，`StateFrom(row)` 让它们跟**这一行**的悬停（`ui/style.go:29-40` 的文档例
// 就是"铅笔在它所在行悬停时出现"）。行自己的悬停背景沿用同一机制（原本就有）。
package dsentrycustom

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar/panelapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方 zh 文案（locales.ts:13-21，逐字）。
const (
	// zhPinTooltip 是 'pinTooltip'（locales.ts:14）。
	zhPinTooltip = "固定到左侧栏"
	// zhPinPrefix / zhPinSuffix 拼出 'pin' 的 zh 模板「将{name}固定到侧边栏」
	// （locales.ts:15）。viva 的 Hit 只收一整条可读文案，所以在这里插值。
	zhPinPrefix = "将"
	zhPinSuffix = "固定到侧边栏"
	// zhInstall 是 'install'（locales.ts:16）。
	zhInstall = "浏览更多插件"
	// zhCreate 是 'create'（locales.ts:17）。
	zhCreate = "创建你的插件"
	// 两个 Creator 提示词逐字来自 locales.ts:18-19；rightbar 没有区域宿主，仍按 HIDE。
	zhCreatePromptSidebar      = "创建一个带左侧栏入口的 DSH 插件。"
	zhCreatePromptConversation = "创建一个 DSH 会话视图插件。"
)

// 官方 primitives 菜单行的几何（出处见文件头；dist CSS 的 `_70kcw_` 模块）。
const (
	// entryRowMinHeight 是 `_item_70kcw_173{min-height:34px}`。
	entryRowMinHeight float32 = 34
	// entryRowPadX / entryRowPadY 是 `.item{padding:6px 8px}`。
	entryRowPadX float32 = 8
	entryRowPadY float32 = 6
	// entryRowGap 是 `.item{gap:6px}`。
	entryRowGap float32 = 6
	// entryFontSize / entryLineHeight 是 `.item{font-size:13px;line-height:20px}`。
	entryFontSize   float32 = 13
	entryLineHeight float32 = 20
	// entryIconBox 是 `.itemIcon{width:14px;height:14px}`（图标 svg 同样夹到 14）。
	entryIconBox float32 = 14
	// entrySeparatorHeight 是 `.separator{height:.5px}`（dist CSS `_separator_70kcw_157`）。
	//
	// 0.5 就是官方原值，不做取整：同一份 primitives 的分离线在本仓已有 0.5 的落点先例
	// （`plugins/dscontextmenu/context_menu.go:43` 的 `menuSeparatorHeight`，同一条 `_70kcw_` 规则）。
	entrySeparatorHeight float32 = 0.5
	// entrySeparatorMarginV / H 是 `.separator{margin:3px 8px}`。
	entrySeparatorMarginV float32 = 3
	entrySeparatorMarginH float32 = 8

	// sidebarPinBox 是 `.pinButton{width:22px;height:22px}`。
	sidebarPinBox float32 = 22
	// sidebarPinRight 是 `.pinAnchor{right:6px}`（OFF:EntryMenu.module.css:15）：按钮的
	// 右边缘距行右边 6px。官方 `.pinAnchor` 是**绝对定位**（`position:absolute;
	// top:0;bottom:0`），不受 MenuItemButton 自身 `padding:6px 8px` 影响——所以这里
	// 也用 `Absolute().Right(6).Top(0).Bottom(0)`，而不是把它当流内的最后一格。
	sidebarPinRight float32 = 6
	// sidebarNamePadRight 是 `.sidebarEntry .entryName{padding-right:28px}`
	// （OFF:EntryMenu.module.css:6）：28 = 22（`.pinButton` 宽）+ 6（`.pinAnchor` right），
	// 条目名在这个位置收住，绝对定位的按钮盖在让出的这一条上。
	sidebarNamePadRight float32 = 28
	// sidebarActiveMarkRight / sidebarActiveMarkBox 是 `.sidebarEntry .activeMark`
	// 的绝对定位与尺寸（OFF:EntryMenu.module.css:7-14、:3）。
	//
	//	.sidebarEntry .activeMark{position:absolute;right:9px;top:50%;
	//	  transform:translateY(-50%);opacity:0;pointer-events:none}
	//	.activeMark{flex:none;margin-left:auto}
	//
	// 9px 与 6px 是**两条不同的规则**：`.pinAnchor{right:6px}` 是 22 宽按钮的左缘
	// 落在 right:6 处（左缘 = 6+22 = 28），`.activeMark{right:9}` 是 14 宽对勾的右缘
	// 落在 right:9 处（左缘 = 9+14 = 23）。两条都从行右边量，不共用 28。
	sidebarActiveMarkRight float32 = 9
	// sidebarActiveMarkBox 是对勾本身边长：官方传 `size={16}`（EntryMenuContent.tsx:49），
	// 但 primitives 那条 `._itemIcon svg,._check{width:14px;height:14px}` 把它夹到 14，
	// 与行首面板图标同一个落点（见 entryGlyphSize）。
	sidebarActiveMarkBox float32 = 14
	// sidebarEntryHoverOpacity 是官方 `@media (hover:hover)` 里两处 opacity 的原值
	// （OFF:EntryMenu.module.css:27,29 的显形与 :12,31 的隐去），一侧恒为 1、另一侧恒为 0。
	sidebarEntryHoverOpacity float32 = 1
	// sidebarEntryRestOpacity 是非悬停态的原值（`.activeMark{opacity:0}` :12）。
	sidebarEntryRestOpacity float32 = 0

	// entryGlyphSize / entryGlyphHeight 是面板图标在菜单行里**画出来的**尺寸。
	//
	// 官方那一格是 `renderFactorySlot('sidebar.panel.icon', { size: 16 })`
	// （OFF:EntryMenuContent.tsx:44），包在 `.entryIcon{width:16px;height:16px}` 里
	// （OFF:EntryMenu.module.css:4），而这枚 svg 会被 primitives 的
	// `._itemIcon_70kcw_233 svg{width:14px;height:14px}` 夹到 **14×14**——
	// 所以菜单行里真正落到像素上的图标是 14（16 的 `.entryIcon` 在 14 的槽里居中溢出），
	// 字形的字号/行高取同一个 14，不再另取 13 这种近似值。
	entryGlyphSize   float32 = 14
	entryGlyphHeight float32 = 14
)

// Plugin 是 #17 的内容插件。
type Plugin struct {
	theme *pluginapi.Theme
	// nav 是官方 `pluginNavigation` 的等价物；nil = 面板导航不在册
	// → 「浏览更多插件」整行不画（官方 `browsable` 同一判据）。
	nav panelapi.Navigation
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-entry-customization" }

// Provides 实现 pluginapi.Plugin：本包只画行，不对外提供能力。
func (*Plugin) Provides() []string { return nil }

// Requires 实现 pluginapi.Plugin。
//
// 面板导航写进 Requires（不是可选注入）：官方那行是在服务**存在时**才画，
// 而本仓这条服务由 ds-sidebar 提供——ds-sidebar 不在，这个插件就只剩"会话头那几行"，
// 那正是官方 plugin-manager 缺席时的行为。写成 Requires 让装配顺序确定（服务先就位），
// 画不画仍按注入结果决定。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, panelapi.ServiceKey}
}

// Register 实现 pluginapi.Plugin。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.nav = nil
	if service := ctx.Inject(panelapi.ServiceKey); service != nil {
		if nav, ok := service.(panelapi.Navigation); ok {
			p.nav = nav
		}
	}
	return nil
}

// Panels 实现 pluginapi.Panels：两个座位各一份（右栏 guide 没有宿主，见包注释 HIDE 2）。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		// 左栏：未固定条目行（带面板图标 + 行尾固定按钮）+ 「浏览更多插件」。
		{ID: "entry-menu-sidebar", Slot: uikernel.OffSidebarPanelActions, Draw: p.drawSidebarRows},
		// 会话头：More 里折起来的 view 行（无图标、无固定按钮、active 带对勾）
		// + 「浏览更多插件」。
		{ID: "entry-menu-conversation", Slot: uikernel.OffSessionHeaderViewAction, Draw: p.drawConversationRows},
	}
}

// drawSidebarRows 画左栏 More 菜单里的行。
//
// 官方那一行（EntryMenuContent.tsx:38）：`entries.filter(entry => !entry.pinned)`——
// 已固定的条目还在列表里，不进菜单。
func (p *Plugin) drawSidebarRows(f ui.C, env *pluginapi.Env) {
	props := env.EntryMenu
	if props == nil || p.theme == nil {
		return
	}
	more := make([]pluginapi.EntryMenuEntry, 0, len(props.Entries))
	for _, entry := range props.Entries {
		if entry.Pinned {
			continue
		}
		more = append(more, entry)
	}
	for _, entry := range more {
		p.drawSidebarEntryRow(f, env, props, entry)
	}
	p.drawFooterRows(f, env, props, len(more))
}

// drawConversationRows 画会话头 More 菜单里的行。
//
// 官方（EntryMenuContent.tsx:38）：会话头区域 `moreEntries = entries`（宿主已经
// 把"折起来的 tab"筛好了），行里**没有**面板图标、也**没有**固定按钮。
func (p *Plugin) drawConversationRows(f ui.C, env *pluginapi.Env) {
	props := env.EntryMenu
	if props == nil || p.theme == nil {
		return
	}
	for _, entry := range props.Entries {
		p.drawConversationEntryRow(f, env, props, entry)
	}
	p.drawFooterRows(f, env, props, len(props.Entries))
}

// drawSidebarEntryRow 画左栏的一条未固定条目：`[图标][标题][固定按钮|对勾]`。
//
// 行尾那一格是**二选一**（官方同一块 CSS 里对勾与固定按钮互斥）：
//
//	静止    固定按钮 opacity 0（`.pinAnchor{opacity:0}` :26）、对勾 opacity 1（:27）
//	行悬停  固定按钮 opacity 1（`:hover .pinAnchor` :28）、对勾 opacity 0（:30）
func (p *Plugin) drawSidebarEntryRow(f ui.C, env *pluginapi.Env, props *pluginapi.EntryMenuProps, entry pluginapi.EntryMenuEntry) {
	t := p.theme
	// 这一行的两个命中身份：行本身与行尾固定按钮。官方的 `.sidebarEntry:hover` 是
	// **祖先级**判据（指针落在固定按钮上时外层同样 `:hover`），所以两个都要比。
	rowID := env.Tree.ID(env.Feat, "entry-open-"+entry.ID)
	pinID := env.Tree.ID(env.Feat, "entry-pin-"+entry.ID)
	rowHovered, pinHovered := sidebarEntryHovered(env, rowID, pinID)
	hovered := rowHovered || pinHovered
	// 外层这一圈就是官方 `div.sidebarEntry{position:relative}`（OFF:EntryMenuContent.tsx:41）：
	// 它不参与额外几何，只给绝对定位的 `.pinAnchor` / `.activeMark` 当参照物。
	ui.Row(f).FillWidth().Items(ui.Center).Children(func() {
		row := ui.Row(f).Grow(1).MinHeight(entryRowMinHeight).
			PadSides(entryRowPadY, entryRowPadX, entryRowPadY, entryRowPadX).
			Gap(entryRowGap).Radius(t.Radii.MD).Items(ui.Center)
		if pinHovered {
			// 官方那条祖先级规则（`.sidebarEntry:hover>div:first-child>button`，:21）：
			// 指针落在固定按钮上时**整行底色也亮**。声明式 Hover 只看自己，
			// 所以这一档写成静止值；行自己悬停仍由下面的 `Hover` 负责。
			row = row.Background(t.Hover)
		}
		row = row.
			// `._item_70kcw_173:hover:not(:disabled):not([aria-disabled=true]){background:interactive-bg-hover}`
			// （同样覆盖 `:focus-visible`）——行背景跟的是 `.item` **自己的** hover。
			Hover(ui.Look{}.Background(t.Hover)).
			Hit(rowID, entry.Label)
		row.Children(func() {
			// 官方 `icon` 槽在 sidebar 区域**恒存在**：`icon={region === 'sidebar' ? () : undefined}`
			// （OFF:EntryMenuContent.tsx:42-46）。所以即使这一条没有字形，那一格也占位
			// （primitives 的 `.itemIcon` 14×14 与行内 gap 6 都照常算），标签不会左移。
			ui.Row(f).Size(entryIconBox, entryIconBox).Shrink(0).
				Items(ui.Center).Justify(ui.Center).Children(func() {
				if entry.Glyph == "" {
					return
				}
				// 图标色是 primitives 的 `._itemIcon_70kcw_233{color:var(--dsw-alias-menu-icon)}`；
				// 本仓对应令牌是 `pluginapi.Theme.MenuIcon`（`pluginapi/theme.go:65`，
				// `plugins/dscore` 明暗两套主题都给了值），与 dscontextmenu 同一个令牌。
				ui.Text(f, entry.Glyph).FontSize(entryGlyphSize).LineHeight(entryGlyphHeight).
					Color(t.MenuIcon)
			})
			// 官方 `.itemLabel` 是 nowrap+ellipsis，但 `.entryTitle`
			// （OFF:EntryMenu.module.css:2）把它改回 `white-space:normal;overflow-wrap:anywhere`
			// ——条目名可以换行，所以这里**不挂 Ellipsis**，改用 `.MaxLines(0)`
			// （viva `ui/text_wrap.go:72-78`：0 = 按宽度换任意多行；单行 Text 不会自己换行）。
			//
			// `.sidebarEntry .entryName{padding-right:28px}`（OFF:EntryMenu.module.css:6）：
			// 条目名右侧让出 28px（= 22 的固定按钮 + 6 的 `.pinAnchor{right:6px}`），
			// 这样绝对定位的固定按钮不会压到字上。
			ui.Text(f, entry.Label).FontSize(entryFontSize).LineHeight(entryLineHeight).
				Grow(1).MaxLines(0).PadSides(0, sidebarNamePadRight, 0, 0).Color(t.LabelPrimary)
			// 官方 `.activeMark` 在**两个区域都画**（EntryMenuContent.tsx:48-50：它在
			// `.entryName` 里，与 region 无关），只是规则不同：
			//   会话头 → `.activeMark{flex:none;margin-left:auto}`（:3），行内流位、恒可见
			//   左栏   → `.sidebarEntry .activeMark{position:absolute;right:9px;top:50%;
			//            transform:translateY(-50%)}`（:7-14）+ 悬停才隐（:27,:30）
			if entry.Active {
				if props.Region == pluginapi.EntryRegionSidebar {
					drawSidebarActiveMark(f, t, hovered)
				} else {
					drawInlineActiveMark(f, t)
				}
			}
		})
		if row.Clicked() {
			p.dismiss(props)
			if props.Open != nil {
				props.Open(entry.ID)
			}
		}
		// 固定按钮画在行之后、绝对定位：viva 的绝对元素"painted after the in-flow
		// children … wins pointer hits over them"（`viva/ui/absolute.go:11`），
		// 与官方 DOM 里 `.pinAnchor` 盖在 MenuItemButton 上完全同序。
		p.drawPinButton(f, env, props, entry, pinID, hovered, pinHovered)
	})
}

// drawInlineActiveMark 画 `.activeMark{flex:none;margin-left:auto}`（OFF:EntryMenu.module.css:3）：
// 流内行尾一格、`margin-left:auto` 把它推到最右——在 viva 里就是"前面那格 Grow(1)"。
//
// 会话头那一侧没有 `.sidebarEntry` 祖先，所以 :12 的 `opacity:0` 与 :30 的悬停隐去
// 都**不适用**：恒可见（这是官方会话头的真实样子，不是我们简化）。
func drawInlineActiveMark(f ui.C, t *pluginapi.Theme) {
	if icon, ok := checkOutlineIcon(); ok {
		ui.Icon(f, icon).Size(entryIconBox, entryIconBox).Shrink(0).Color(t.LabelPrimary)
	}
}

// drawSidebarActiveMark 画左栏行尾的对勾（官方 `.sidebarEntry .activeMark`，
// OFF:EntryMenu.module.css:7-14）。
//
// 几何：`right:9px` 从**行右边**量、宽 14（官方传 `size={16}`，被
// `._itemIcon svg,._check_70kcw_265{width:14px;height:14px}` 夹到 14）；
// 竖直靠 `top:50%` + `translateY(-50%)` 居中——viva 用 `Top(0).Bottom(0)` 拉伸再
// `Items(ui.Center)` 居中，落点相同。
//
// 显隐：静止 1（:27 覆盖 :12 的 0）→ 行悬停 0（:30）。`pointer-events:none`（:13）
// 在 viva 里天然成立——这枚盒子没有挂 Hit，不参与命中。
//
// 9 与 6 是两条独立规则：pin 右缘在 6（宽 22，左缘 28），对勾右缘在 9（宽 14，
// 左缘 23）；两者靠 opacity 互斥，同一时刻只有一枚可见。
func drawSidebarActiveMark(f ui.C, t *pluginapi.Theme, hovered bool) {
	opacity := sidebarEntryHoverOpacity // 静止 = 1
	if hovered {
		opacity = sidebarEntryRestOpacity // 行悬停 = 0
	}
	mark := ui.Row(f).Absolute().Right(sidebarActiveMarkRight).Top(0).Bottom(0).
		Items(ui.Center).Opacity(opacity)
	mark.Children(func() {
		if icon, ok := checkOutlineIcon(); ok {
			ui.Icon(f, icon).Size(sidebarActiveMarkBox, sidebarActiveMarkBox).Color(t.LabelPrimary)
		}
	})
}

// sidebarEntryHovered 报告这一行里**谁**被悬停：行本身、行尾固定按钮，或都没有。
//
// CSS 的 `:hover` 沿祖先链传播：指针落在固定按钮上时外层 `.sidebarEntry` 也是
// `:hover`（所以 `.pinAnchor` 悬停时不会自己消失，整行底色也照样亮）。viva 的
// `Canvas.Hovered()` 只给**最上面那一个**命中身份（`gfx/canvas.go:1962`），
// 所以两个 id 分开比、由调用方合并。
//
// 与 `dsattachment` 的 `removeVisualActive`（`plugins/dsattachment/rail.go:1035-1041`）
// 同一条写法：声明式 `StateFrom` 只能跟一个来源，这里需要"任一即真"。
// `env.Canvas` 为 nil（无头/最小装配）时按未悬停处理——宁可少画一次显形，
// 也不让绘制路径 panic。
func sidebarEntryHovered(env *pluginapi.Env, rowID, pinID gfx.ID) (row, pin bool) {
	if env == nil || env.Canvas == nil {
		return false, false
	}
	hovered := env.Canvas.Hovered()
	if hovered.IsZero() {
		return false, false
	}
	return hovered == rowID, hovered == pinID
}

// drawConversationEntryRow 画会话头的一条折起来的 view：`[标题][对勾]`。
func (p *Plugin) drawConversationEntryRow(f ui.C, env *pluginapi.Env, props *pluginapi.EntryMenuProps, entry pluginapi.EntryMenuEntry) {
	t := p.theme
	row := ui.Row(f).FillWidth().MinHeight(entryRowMinHeight).
		PadSides(entryRowPadY, entryRowPadX, entryRowPadY, entryRowPadX).
		Gap(entryRowGap).Radius(t.Radii.MD).Items(ui.Center).
		Hover(ui.Look{}.Background(t.Hover)).
		Hit(env.Tree.ID(env.Feat, "entry-open-"+entry.ID), entry.Label)
	row.Children(func() {
		ui.Text(f, entry.Label).FontSize(entryFontSize).LineHeight(entryLineHeight).
			Grow(1).MaxLines(0).Color(t.LabelPrimary)
		if entry.Active {
			drawInlineActiveMark(f, t)
		}
	})
	if row.Clicked() {
		p.dismiss(props)
		if props.Open != nil {
			props.Open(entry.ID)
		}
	}
}

// drawPinButton 画左栏行尾的「固定」按钮（官方 `.pinAnchor` + `.pinButton`）。
//
// 不画的判据与宿主同一条：`SetPinned == nil`（没接线）时不画——画一个点了不改任何
// 状态的按钮比不画更坏（`dssidebar` 对行尾取消固定按钮用的是同一条判据）。
func (p *Plugin) drawPinButton(f ui.C, env *pluginapi.Env, props *pluginapi.EntryMenuProps,
	entry pluginapi.EntryMenuEntry, pinID gfx.ID, hovered, pinHovered bool) {
	t := p.theme
	if props.SetPinned == nil {
		return
	}
	// `.pinAnchor{position:absolute;right:6px;top:0;bottom:0;display:flex;align-items:center}`（:15）
	// + `@media (hover:hover){.pinAnchor{opacity:0}}`（:26）与
	// `.sidebarEntry:hover .pinAnchor{opacity:1}`（:28）：
	// 不占流内位置，竖直拉满行高再居中；**静止完全透明、行不悬停就不显形**。
	// viva 的 Opacity 语义与 CSS 一致：透明元素仍然可点（`ui/style.go:207-219`），
	// 所以 x 轴上这一格在静止态也接得住点击——与官方 `.pinAnchor` 同一条行为。
	opacity := sidebarEntryRestOpacity // :26
	if hovered {
		opacity = sidebarEntryHoverOpacity // :28
	}
	anchor := ui.Row(f).Absolute().Right(sidebarPinRight).Top(0).Bottom(0).
		Items(ui.Center).Opacity(opacity)
	// `.pinButton:hover:not(:disabled){color:label-primary;background:0 0}`（:22-25）：
	// 只有**按钮自己**被悬停才换色（行悬停不换）。这一格的颜色要同时喂给描边和
	// 填充层——官方那枚图标的尾 path 是 `fill="currentColor"`（sprite 原文），
	// 所以两处必须同一个值，否则悬停后描边变亮、填充层还留在旧色上。
	pinColor := t.LabelTertiary
	if pinHovered {
		pinColor = t.LabelPrimary
	}
	anchor.Children(func() {
		// 几何/配色全取官方 `.pinButton`（OFF:EntryMenu.module.css:16）+ primitives Button 基类：
		//   width/height 22、padding 0、color --dsw-alias-label-tertiary（本仓 LabelTertiary）；
		//   `.pinButton` 不覆盖圆角 → 吃 Button 基类 `._button_1ulci_2{border-radius:var(--dsw-radius-md)}`，
		//   所以是 Radii.MD，不是 SM。
		button := ui.Row(f).Size(sidebarPinBox, sidebarPinBox).Shrink(0).
			Radius(t.Radii.MD).Items(ui.Center).Justify(ui.Center).
			Hit(pinID, zhPinPrefix+entry.Label+zhPinSuffix).
			// 官方两层文案是**分开**的（OFF:EntryMenuContent.tsx:54-55）：
			//   Tooltip  label={t('pinTooltip')}       → 可见的 "固定到左侧栏"
			//   Button   aria-label={t('pin',{name})}  → 无障碍名 "将{name}固定到侧边栏"
			// viva 的 `ui.Tooltip` 同时充当可见浮条与无障碍描述（`viva/ui/tooltip.go:12-26`），
			// 但两件事本仓各走各的：Hit 的 label 给无障碍名，Tooltip 给可见文案。
			// `side="bottom"` → `TooltipOn(ui.TooltipBottom)`；官方 `delayMs={500}` 与
			// `align="end"`/`portal` 是时序与呈现位置，viva 的工具条延迟由主题给、
			// 不按元素配，这一处不匹配不构成"少了部件"。
			Tooltip(zhPinTooltip).TooltipOn(ui.TooltipBottom)
		button.Children(func() {
			// `pinOutlineIcon` 收的就是"这一帧的颜色"：描边由 `.Color()` 画、填充层
			// （sprite 里的 `fill="currentColor"`）由工厂参数画，两者是**同一枚令牌**。
			if icon, ok := pinOutlineIcon(pinColor); ok {
				ui.Icon(f, icon).Size(entryIconBox, entryIconBox).Color(pinColor)
			}
		})
		if button.Clicked() {
			// 官方 `onClick={() => { onDismiss(); onPinnedChange(entry.id, true) }}`（:56）：
			// 先关菜单，再把这一条固定回去。
			p.dismiss(props)
			props.SetPinned(entry.ID, true)
		}
	})
}

// drawFooterRows 按官方顺序画浏览行与 Creator。浏览服务缺席时，Creator
// 自己承担 separatorBefore = !browsable && moreEntries.length > 0（官方 :66）。
func (p *Plugin) drawFooterRows(f ui.C, env *pluginapi.Env, props *pluginapi.EntryMenuProps, moreEntries int) {
	p.drawBrowseRow(f, env, props, moreEntries > 0)
	p.drawCreateRow(f, env, props, p.nav == nil && moreEntries > 0)
}

// drawCreateRow 画官方「创建你的插件」（EntryMenuContent.tsx:66-69）。点击只创建
// cordis 草稿，不自动发送；区域提示词与 whenDraftExists:'append' 逐字来自 index.ts:47-52。
func (p *Plugin) drawCreateRow(f ui.C, env *pluginapi.Env, props *pluginapi.EntryMenuProps, separatorBefore bool) {
	t := p.theme
	if env.Act.StartSession == nil || props == nil {
		return
	}
	prompt := ""
	switch props.Region {
	case pluginapi.EntryRegionSidebar:
		prompt = zhCreatePromptSidebar
	case pluginapi.EntryRegionConversation:
		prompt = zhCreatePromptConversation
	default:
		return
	}
	if separatorBefore {
		p.drawSeparator(f)
	}
	row := ui.Row(f).FillWidth().MinHeight(entryRowMinHeight).
		PadSides(entryRowPadY, entryRowPadX, entryRowPadY, entryRowPadX).
		Gap(entryRowGap).Radius(t.Radii.MD).Items(ui.Center).
		Hover(ui.Look{}.Background(t.Hover)).
		Hit(env.Tree.ID(env.Feat, "entry-create-plugin"), zhCreate)
	row.Children(func() {
		ui.Row(f).Size(entryIconBox, entryIconBox).Shrink(0).
			Items(ui.Center).Justify(ui.Center).Children(func() {
			if icon, ok := plusOutlineIcon(); ok {
				ui.Icon(f, icon).Size(entryIconBox, entryIconBox).Color(t.MenuIcon)
			}
		})
		ui.Text(f, zhCreate).FontSize(entryFontSize).LineHeight(entryLineHeight).
			Grow(1).Ellipsis().Color(t.LabelPrimary)
	})
	if row.Clicked() {
		p.dismiss(props)
		env.Act.StartSession(pluginapi.StartSessionOptions{
			AgentPreset: "cordis", Prompt: prompt, WhenDraftExists: pluginapi.DraftAppend,
		})
	}
}

// drawBrowseRow 画「浏览更多插件」（官方 `browsable && <MenuItemButton …>`，:62-65）。
//
// 三个不画：服务不在册（官方 `browsable`）、主题没就位、props 来自别的区域。
func (p *Plugin) drawBrowseRow(f ui.C, env *pluginapi.Env, props *pluginapi.EntryMenuProps, separatorBefore bool) {
	t := p.theme
	if p.nav == nil {
		return
	}
	if separatorBefore {
		p.drawSeparator(f)
	}
	row := ui.Row(f).FillWidth().MinHeight(entryRowMinHeight).
		PadSides(entryRowPadY, entryRowPadX, entryRowPadY, entryRowPadX).
		Gap(entryRowGap).Radius(t.Radii.MD).Items(ui.Center).
		Hover(ui.Look{}.Background(t.Hover)).
		Hit(env.Tree.ID(env.Feat, "entry-browse-plugins"), zhInstall)
	row.Children(func() {
		ui.Row(f).Size(entryIconBox, entryIconBox).Shrink(0).
			Items(ui.Center).Justify(ui.Center).Children(func() {
			if icon, ok := pluginPinwheelIcon(); ok {
				ui.Icon(f, icon).Size(entryIconBox, entryIconBox).Color(t.MenuIcon)
			}
		})
		ui.Text(f, zhInstall).FontSize(entryFontSize).LineHeight(entryLineHeight).
			Grow(1).Ellipsis().Color(t.LabelPrimary)
	})
	if row.Clicked() {
		// 官方 `onSelect={() => { onDismiss(); browsePlugins() }}`（:63）：先关，再开插件列表。
		p.dismiss(props)
		p.nav.OpenList()
	}
}

// drawSeparator 画官方 `separatorBefore` 那条分离线（`.separator{height:.5px;margin:3px 8px}`）。
func (p *Plugin) drawSeparator(f ui.C) {
	ui.Row(f).FillWidth().PadSides(entrySeparatorMarginV, entrySeparatorMarginH,
		entrySeparatorMarginV, entrySeparatorMarginH).Children(func() {
		ui.Row(f).FillWidth().Height(entrySeparatorHeight).Background(p.theme.BorderL2)
	})
}

// dismiss 调官方 onDismiss（nil = 宿主没接，直接跳过）。
func (*Plugin) dismiss(props *pluginapi.EntryMenuProps) {
	if props == nil || props.Dismiss == nil {
		return
	}
	props.Dismiss()
}
