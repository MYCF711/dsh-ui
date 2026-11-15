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
//  1. **「创建你的插件」整行不画**：官方点了走
//     `uiWorkspace.startSession(undefined, {agentPreset:'cordis', prompt, whenDraftExists:'append'})`
//     （OFF:src/client/index.ts:47-52），落点是宿主 `session.create` 要认 `agentPreset`。
//     DSH-GO 宿主今天**不认**：`internal/transport/server.go` 的 `handleSessionCreate`
//     只解 `Mode`，`opt.Create` 签名是 `func(mode string) (string, error)`。
//     画一行点了建不出正确预设的按钮，比不画更坏——所以整行 HIDE，
//     等宿主能收 `agentPreset` 再补（那时「浏览更多插件」那行的 separatorBefore
//     还要按官方补一条 `!browsable && moreEntries.length > 0`）。
//  2. **右栏 guide 的 More 卡片不注册**：`sidebar.right.tab.guide.actions` 本仓还没有
//     宿主（guide 面未落地，`docs/official-parity.md` 的 #53），注册了也没有派发方。
//  3. **左栏行尾的对勾不画**（两处差异里的一条，另一条见 `drawSidebarRows`）：
//     官方 `.sidebarEntry .activeMark{opacity:0}`，只在行 hover 时对勾出现、固定按钮
//     隐去（OFF:EntryMenu.module.css:20-31）。viva 没有"本帧是否悬停"的读口，
//     本包按**非 hover 态**画（固定按钮恒显、对勾不画），不冒充 hover。
package dsentrycustom

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar/panelapi"
	"github.com/vivaui/viva/ui"
)

// 官方 zh 文案（locales.ts:13-21，逐字；`create` 只用于 HIDE 登记与测试断言）。
const (
	// zhPinTooltip 是 'pinTooltip'（locales.ts:14）。
	zhPinTooltip = "固定到左侧栏"
	// zhPinPrefix / zhPinSuffix 拼出 'pin' 的 zh 模板「将{name}固定到侧边栏」
	// （locales.ts:15）。viva 的 Hit 只收一整条可读文案，所以在这里插值。
	zhPinPrefix = "将"
	zhPinSuffix = "固定到侧边栏"
	// zhInstall 是 'install'（locales.ts:16）。
	zhInstall = "浏览更多插件"
	// zhCreate 是 'create'（locales.ts:17）——本包**不画**这一行（见包注释 HIDE 段），
	// 常量留着给测试钉住"它确实不在画面里"。
	zhCreate = "创建你的插件"
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
	// entrySeparatorHeight 是 `.separator{height:.5px}`（0.5 取整到 1：viva 的
	// 边框/线条落点按整像素对齐，半像素在这台机器上会被抹掉或双写）。
	entrySeparatorHeight float32 = 1
	// entrySeparatorMarginV / H 是 `.separator{margin:3px 8px}`。
	entrySeparatorMarginV float32 = 3
	entrySeparatorMarginH float32 = 8

	// sidebarPinBox 是 `.pinButton{width:22px;height:22px}`。
	sidebarPinBox float32 = 22
	// sidebarPinRight 是 `.pinAnchor{right:6px}`：本包把按钮放在行内（不绝对定位），
	// 右侧留同样 6px，因此与官方 `.entryName{padding-right:28px}` 的 28 = 22+6 对齐。
	sidebarPinRight float32 = 6
	// entryGlyphSize 是面板行字形在菜单行里的字号（面板行本身用 16，菜单图标框
	// 只有 14——字形按框高取 13/14，与官方"图标被夹到 14"同一效果）。
	entryGlyphSize   float32 = 13
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
	p.drawBrowseRow(f, env, props, len(more) > 0)
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
	p.drawBrowseRow(f, env, props, len(props.Entries) > 0)
}

// drawSidebarEntryRow 画左栏的一条未固定条目：`[图标][标题][固定按钮]`。
func (p *Plugin) drawSidebarEntryRow(f ui.C, env *pluginapi.Env, props *pluginapi.EntryMenuProps, entry pluginapi.EntryMenuEntry) {
	t := p.theme
	ui.Row(f).FillWidth().Items(ui.Center).Children(func() {
		row := ui.Row(f).Grow(1).MinHeight(entryRowMinHeight).
			PadSides(entryRowPadY, entryRowPadX, entryRowPadY, entryRowPadX).
			Gap(entryRowGap).Radius(t.Radii.MD).Items(ui.Center).
			Hover(ui.Look{}.Background(t.Hover)).
			Hit(env.Tree.ID(env.Feat, "entry-open-"+entry.ID), entry.Label)
		row.Children(func() {
			if entry.Glyph != "" {
				ui.Row(f).Size(entryIconBox, entryIconBox).Shrink(0).
					Items(ui.Center).Justify(ui.Center).Children(func() {
					// 官方图标色是 `--dsw-alias-menu-icon`：本仓没有这个令牌，
					// 按"最接近的那一档"用 LabelDimmed（见 docs 的证据文档差异登记）。
					ui.Text(f, entry.Glyph).FontSize(entryGlyphSize).LineHeight(entryGlyphHeight).
						Color(t.LabelDimmed)
				})
			}
			// 官方 `.itemLabel` 是 nowrap+ellipsis，但 `.entryTitle` 把它改回
			// `white-space:normal;overflow-wrap:anywhere`——条目名可以换行，所以这里
			// 不挂 Ellipsis（长名换行，而不是被截断）。
			ui.Text(f, entry.Label).FontSize(entryFontSize).LineHeight(entryLineHeight).
				Grow(1).Color(t.LabelPrimary)
		})
		if row.Clicked() {
			p.dismiss(props)
			if props.Open != nil {
				props.Open(entry.ID)
			}
		}
		p.drawPinButton(f, env, props, entry)
	})
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
			Grow(1).Color(t.LabelPrimary)
		if entry.Active {
			// 官方 `.activeMark{flex:none;margin-left:auto}`：当前 view 落在 More 里时
			// 行尾一枚对勾（会话头这一侧没有 hover 规则，恒可见）。
			if icon, ok := checkOutlineIcon(); ok {
				ui.Icon(f, icon).Size(entryIconBox, entryIconBox).Shrink(0).Color(t.LabelPrimary)
			}
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
func (p *Plugin) drawPinButton(f ui.C, env *pluginapi.Env, props *pluginapi.EntryMenuProps, entry pluginapi.EntryMenuEntry) {
	t := p.theme
	if props.SetPinned == nil {
		return
	}
	button := ui.Row(f).Size(sidebarPinBox, sidebarPinBox).Shrink(0).
		MarginSides(0, sidebarPinRight, 0, 0).
		Radius(t.Radii.SM).Items(ui.Center).Justify(ui.Center).
		Hover(ui.Look{}.Background(t.Hover)).
		Hit(env.Tree.ID(env.Feat, "entry-pin-"+entry.ID), zhPinPrefix+entry.Label+zhPinSuffix)
	button.Children(func() {
		if icon, ok := pinOutlineIcon(t.LabelTertiary); ok {
			ui.Icon(f, icon).Size(entryIconBox, entryIconBox).Color(t.LabelTertiary)
		}
	})
	if button.Clicked() {
		// 官方 `onClick={() => { onDismiss(); onPinnedChange(entry.id, true) }}`（:56）：
		// 先关菜单，再把这一条固定回去。
		p.dismiss(props)
		props.SetPinned(entry.ID, true)
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
				ui.Icon(f, icon).Size(entryIconBox, entryIconBox).Color(t.LabelDimmed)
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
