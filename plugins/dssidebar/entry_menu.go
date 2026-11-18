package dssidebar

// entry_menu.go —— 左栏面板列表的 **More 菜单宿主**（官方 `SidebarRoot` 的 `PanelMoreMenu`）。
//
// # 官方事实（rc.2 随包 src/**）
//
//	OFF:dsh-client-ui-sidebar/src/client/SidebarRoot.tsx:107-133  PanelMoreMenu
//	  anchor = `button.panelRow`（与面板行同几何）+ `aria-haspopup=menu` + `aria-expanded`
//	  glyph  = `<IconAppsAddOutlineRegular size={wide?16:18}/>`（:124）
//	  wide   = 同一行里再画 `<span.panelTitle>{label}</span>`
//	  label  = `t('panels.more')` = 「更多」（locales.ts:13）
//	  menu   = `renderSlot('sidebar.panellist.actions', {entries, onOpen, onPinnedChange, onDismiss})`
//	OFF:dsh-client-ui-sidebar/src/client/SidebarRoot.tsx:404-431  列表与入口的判据
//	  可见行 = `id===FIXED || !hasEntryActions || !unpinned.includes(id)`
//	  More    = 只在 `hasEntryActions`（座位有占用方）时出现
//	  entries = 除固定行以外的**全部**面板 `{id,label,pinned,active}`
//
// # 本仓的形状
//
// 两半（viva 的 `Painter.Draw` 拿不到 drawSlot，只有 `Children` 有）：
//
//	Draw     drawPanelMoreRow   行本体 + 命中区（点击切展开态）
//	Children entryMenuChildren  菜单面（`uikernel.DrawEntryMoreMenu`）+ 派发座位
//	AfterDraw afterEntryMenu    把本帧的行/菜单面矩形留给下一帧（锚点与翻面判据）
//
// 三个"不画"的边界（每一条都不会留下死按钮）：
//
//	座位没有占用方        → 整枚 More 行不画（官方 `hasEntryActions` 同一判据）
//	固定态服务没接线      → 行照画，但不画行尾取消固定按钮（点了没反应的按钮更糟）
//	菜单面无 drawSlot     → 只画行、不展开（最小装配/单测环境）
//
// # 与官方的两处登记在案的差异
//
//  1. **More 行用官方复合素材**：3 个圆角矩形由 Viva Border/Radius 实画，
//     加号路径由 dsassets 逐字解析，不用字体 glyph 或贝塞尔近似。
//  2. **行尾取消固定按钮恒显示**：官方 `.panelUnpin{opacity:0}` 只在
//     `.panelEntry:hover` 时淡入（SidebarRoot.module.css:660-671）。viva 的
//     `Box` 没有"本帧是否悬停"的读口，因此按 caption 色常显（不冒充 hover 态）。
//
// 行高与面板行共用官方 `.panelRow{min-height:32}`（frame.go 的 panelRowHeight）。

import (
	"github.com/MYCF711/dsh-ui/internal/entrypins"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方文案（`ui-sidebar/src/client/locales.ts` zh 字典，逐字）。
const (
	// zhPanelsMore 是 'panels.more'（locales.ts:13）。
	zhPanelsMore = "更多"
	// zhPanelUnpinTooltip 是 'panels.unpinTooltip'（locales.ts:5）。
	zhPanelUnpinTooltip = "从左侧栏取消固定"
	// zhPanelUnpin 是 'panels.unpin'（locales.ts:6，"从侧边栏取消固定{name}"）：
	// viva 的 Hit 文案里带上条目名，与官方 aria-label 的插值一致。
	zhPanelUnpin = "从侧边栏取消固定"
)

// entryMenuState 是 More 菜单的本地状态（只由绘制线程读写）。
type entryMenuState struct {
	// open 是官方 `PanelMoreMenu` 的 `open`（展开态）。
	open bool
	// row / rowDrawn 是**本帧**的 More 行；afterEntryMenu 量它的矩形给下一帧当锚点。
	row      ui.Box
	rowDrawn bool
	anchor   ui.Rect
	// panel / panelDrawn / panelRect 是菜单面：同一套"上一帧量到的"几何。
	panel      ui.Box
	panelDrawn bool
	panelRect  ui.Rect
}

// entryActionsLive 报告本帧 `sidebar.panellist.actions` 有没有占用方
// （官方 `hasEntryActions`：`slots.entriesOfSlot(...).length > 0`）。
func (p *Plugin) entryActionsLive(env *pluginapi.Env) bool {
	return env != nil && env.Has != nil && env.Has(uikernel.OffSidebarPanelActions)
}

// visiblePanelRoutes 是官方列表过滤（SidebarRoot.tsx:404-405）：
//
//	id === FIXED_PANEL_ID || !hasEntryActions || !unpinned.includes(id)
//
// 没有固定态服务时按"全部固定"处理（`p.pins == nil`）——那正是默认值。
func (p *Plugin) visiblePanelRoutes(env *pluginapi.Env) []panelRoute {
	routes := p.panelRoutes()
	if !p.entryActionsLive(env) {
		return routes
	}
	out := make([]panelRoute, 0, len(routes))
	for _, route := range routes {
		if route.ID != entrypins.FixedPanelID && !p.pinned(route.ID) {
			continue
		}
		out = append(out, route)
	}
	return out
}

// pinned 报告一个面板 id 是否固定（没有固定态服务时恒 true）。
func (p *Plugin) pinned(id string) bool {
	if p.pins == nil {
		return true
	}
	return p.pins.Pinned(pluginapi.EntryRegionSidebar, id)
}

// entryMenuEntries 是官方传给菜单的 entries：除固定行以外的**全部**面板
// （SidebarRoot.tsx:419-421 的 `panels.filter(({id}) => id !== FIXED_PANEL_ID)`），
// 每条带 `{id, label, pinned, active}`。图标（`Glyph`）一并给：#17 的菜单行要用
// 面板行**同一个**图标（官方那一格是 `sidebar.panel.icon` 渲染工厂槽，
// OFF:dsh-client-ui-entry-customization/src/client/EntryMenuContent.tsx:43-45）。
func (p *Plugin) entryMenuEntries() []pluginapi.EntryMenuEntry {
	routes := p.panelRoutes()
	out := make([]pluginapi.EntryMenuEntry, 0, len(routes))
	for _, route := range routes {
		if route.ID == entrypins.FixedPanelID {
			continue
		}
		out = append(out, pluginapi.EntryMenuEntry{
			ID:     route.ID,
			Label:  route.Label,
			Glyph:  route.Glyph,
			Pinned: p.pinned(route.ID),
			Active: p.activePanel == route.ID,
		})
	}
	return out
}

// drawPanelMoreRow 画官方 `PanelMoreMenu` 的锚点行（不含菜单面）。
//
// `wide` 在本仓是常真（Windows 档侧栏收起即整列消失，没有 56px 图标轨），
// 所以图标与文案都按展开档 16px 实画。
func (p *Plugin) drawPanelMoreRow(f ui.C, env *pluginapi.Env) {
	t := p.theme
	var row ui.Box
	ui.Row(f).FillWidth().Children(func() {
		row = p.prims.ListRow(f, env.Tree.ID(env.Feat, "panel-more"), p.entryMenu.open, panelRowHeight, func() {
			ui.Row(f).Width(16).Height(22).Shrink(0).Items(ui.Center).Justify(ui.Center).Children(func() {
				drawAppsAddIcon(f, t.LabelSecondary, 16)
			})
			ui.Text(f, zhPanelsMore).FontSize(t.Fonts.Body).LineHeight(22).
				Color(t.LabelPrimary).MarginSides(0, 0, 0, 6).Ellipsis()
		})
	})
	p.entryMenu.row, p.entryMenu.rowDrawn = row, true
	if row.Clicked() {
		// 官方 `onClick={() => setOpen(value => !value)}`：同一枚按钮开/关。
		p.entryMenu.open = !p.entryMenu.open
	}
}

// drawPanelUnpin 画面板行尾的「取消固定」按钮（官方 `.panelUnpin`）。
//
// 只在有固定态服务时画：没有服务就没有"取消固定"这个动作，按钮只会点了没反应。
//
// 显隐照官方 `@media (hover:hover)` 的两条：
//
//	SidebarRoot.module.css  `.panelUnpin{opacity:0}`
//	SidebarRoot.module.css  `.panelEntry:hover .panelUnpin,
//	                         .panelEntry:has(:focus-visible) .panelUnpin{opacity:1}`
//
// 即：指针没进这一行时 pin 图标**不可见**，进这一行才淡入。命中区始终在
// （官方 opacity 0 的 button 仍在 DOM 里、仍可点），所以这里只控制图标画不画。
//
// 判定用 `Canvas.Hovered()`（viva components/accordion.go:62 的同一读口）；
// 行 id 与按钮自己的 id 都算"在这一行里"，否则指针一移到 pin 上 hover 目标
// 就换成按钮、图标反而消失。
func (p *Plugin) drawPanelUnpin(f ui.C, env *pluginapi.Env, route panelRoute, rowID gfx.ID) {
	t := p.theme
	unpinID := env.Tree.ID(env.Feat, "panel-unpin-"+route.ID)
	hovered := f.Canvas().Hovered()
	show := hovered == rowID || hovered == unpinID
	unpin := ui.Row(f).Width(28).Height(28).Shrink(0).Radius(t.Radii.SM).
		Items(ui.Center).Justify(ui.Center).Hover(ui.Look{}.Background(t.Hover)).
		Hit(unpinID, zhPanelUnpinTooltip+route.Label)
	unpin.Children(func() {
		if !show {
			return
		}
		if icon, ok := pinFillIcon(t.LabelCaption); ok {
			ui.Icon(f, icon).Size(14, 14).Color(t.LabelCaption)
		}
	})
	if unpin.Clicked() {
		p.pins.SetPinned(pluginapi.EntryRegionSidebar, route.ID, false)
	}
}

// afterEntryMenu 把本帧的行/菜单面矩形留给下一帧（v0.6.0 声明期拿不到盒子矩形）。
func (p *Plugin) afterEntryMenu(canvas *gfx.Canvas, env *pluginapi.Env) {
	if p.entryMenu.rowDrawn {
		p.entryMenu.rowDrawn = false
		if rect := p.entryMenu.row.Rect(); rect.Width > 0 && rect.Height > 0 {
			p.entryMenu.anchor = rect
		}
	}
	if p.entryMenu.panelDrawn {
		p.entryMenu.panelDrawn = false
		if rect := p.entryMenu.panel.Rect(); rect.Width > 0 && rect.Height > 0 {
			p.entryMenu.panelRect = rect
		} else {
			// 菜单这一帧没画出来：高度清零，下次打开按"还没量过"处理。
			p.entryMenu.panelRect = ui.Rect{}
		}
	}
}

// entryMenuChildren 画菜单面并把 `sidebar.panellist.actions` 派发进去。
func (p *Plugin) entryMenuChildren(f ui.C, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	if !p.entryMenu.open || drawSlot == nil {
		return
	}
	uikernel.DrawEntryMoreMenu(f, env, uikernel.EntryMenuOptions{
		Theme:       p.theme,
		Anchor:      p.entryMenu.anchor,
		Bounds:      env.Bounds,
		Open:        true,
		Slot:        uikernel.OffSidebarPanelActions,
		Region:      pluginapi.EntryRegionSidebar,
		Entries:     p.entryMenuEntries(),
		OpenEntry:   p.openPanel,
		SetPinned:   p.setPanelPinned,
		Dismiss:     func() { p.entryMenu.open = false },
		Dispatch:    drawSlot,
		PanelHeight: p.entryMenu.panelRect.Height,
		OnPanel: func(box ui.Box) {
			p.entryMenu.panel, p.entryMenu.panelDrawn = box, true
		},
	})
}

// openPanel 是官方 onOpen：选中一个面板（官方 `selectPanel(id)`）。
//
// 与行点击走同一条路（任务页还要顺手拉一次目录，见 drawHeader 的注释）。
func (p *Plugin) openPanel(id string) {
	p.activePanel = id
	if id == "schedules" {
		if p.act.ScheduleRefresh != nil {
			p.act.ScheduleRefresh()
		}
	}
}

// setPanelPinned 是官方 onPinnedChange：改一条面板的固定态。
func (p *Plugin) setPanelPinned(id string, pinned bool) {
	if p.pins == nil {
		return
	}
	p.pins.SetPinned(pluginapi.EntryRegionSidebar, id, pinned)
}
