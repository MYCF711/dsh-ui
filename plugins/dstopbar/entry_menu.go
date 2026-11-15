package dstopbar

// entry_menu.go —— 会话头 view tab 行的 **More 菜单宿主**（官方 `ViewsMoreMenu`）。
//
// # 官方事实（rc.2 随包 src/**）
//
//	OFF:dsh-client-ui-conversation/src/client/skeleton/ConversationSession.tsx:27-45
//	  ViewsMoreMenu：`Menu` 的锚点是一枚 sm 按钮，里面是
//	  `<IconEllipsisOutlineRegular size={13}/>`，`aria-haspopup=menu` + `aria-expanded`
//	  label = `t('session.moreViews')` = 「更多会话视图」（locales.ts:84）
//	  菜单行 = `renderSlot('conversation.session.header.views.actions',
//	           {entries, onOpen, onPinnedChange, onDismiss})`
//	OFF:dsh-client-ui-conversation/src/client/skeleton/ConversationSession.tsx:119-166
//	  `showTabs = !hideChrome && tabs.length > 1`（只有一个 view 时连 tab 行都没有）
//	  `preferred` = 固定 view 在前，其余**未固定**的按注册序
//	  `visibleViewLimit = max(1, floor((row.clientWidth - 48) / 120))`
//	  `visibleTabs = hasEntryActions ? preferred.slice(0, limit) : preferred`
//	  `moreTabs = tabs.filter(t => !visibleIds.has(t.id))`
//	OFF:dsh-client-ui-conversation/src/client/skeleton/ConversationRoot.module.css:300-323
//	  `.viewsMore{width:22px;height:22px;margin-top:-3px;padding:0;color:label-secondary}`
//	  `.viewsMoreActive{color:state-business-primary}`（当前 view 落在 More 里时）
//	  `.viewsMore[aria-expanded=true]{background:interactive-bg-hover}`
//	  `.viewsMoreMenu{width:288px;max-width:calc(100vw - 24px)}`
//
// # 本仓的形状（与 dssidebar 的 More 同一套两半）
//
//	Draw     drawViewsMoreButton  22×22 按钮（13px ellipsis）+ 命中区
//	Children entryMenuChildren    菜单面（`uikernel.DrawEntryMoreMenu`）+ 派发座位
//	AfterDraw afterEntryMenu      本帧矩形留给下一帧（锚点 / 翻面判据）
//
// 边界（都不留死按钮）：
//
//	座位没有占用方（`hasEntryActions` 假） → 不画按钮
//	名录只有一个 view                       → 不画按钮（官方 showTabs 同一判据）
//	固定态服务没接线                        → 照画按钮，只是没有"取消固定"的动作
//
// 一处登记在案的差异：More 按钮与最后一个 tab 的间距用本包既有的 `tabsGap`（20）。
// 官方 `.viewsMore` 与 `.viewTabs` 是两个兄弟容器（间距由 `.tabs` 的 flex 排布决定，
// 没有可引用的单一数值），本仓一行 Row 里只能给它一个 gap。

import (
	"math"

	"github.com/MYCF711/dsh-ui/internal/entrypins"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方文案（`ui-conversation/src/client/locales.ts` zh 字典，逐字）。
const (
	// zhSessionMoreViews 是 'session.moreViews'（locales.ts:84）。
	zhSessionMoreViews = "更多会话视图"
)

// 官方 `.viewsMore` 几何（ConversationRoot.module.css:304-312）。
const (
	viewsMoreWidth     float32 = 22
	viewsMoreHeight    float32 = 22
	viewsMoreMarginTop float32 = -3
	viewsMoreIconSize  float32 = 13
)

// 官方 `visibleViewLimit` 的两个数（ConversationSession.tsx:127-128）。
const (
	// viewsMoreRowReserve 是 `row.clientWidth - 48` 的 48。
	viewsMoreRowReserve float32 = 48
	// viewsMoreCellBudget 是每个 tab 的预算 120。
	viewsMoreCellBudget float32 = 120
)

// viewEntryMenuState 是 view tab 行 More 菜单的本地状态（只由绘制线程读写）。
type viewEntryMenuState struct {
	open        bool
	button      ui.Box
	buttonDrawn bool
	anchor      ui.Rect
	panel       ui.Box
	panelDrawn  bool
	panelRect   ui.Rect
}

// entryActionsLive 报告本帧 `conversation.session.header.views.actions` 有没有占用方。
func (p *Plugin) entryActionsLive(env *pluginapi.Env) bool {
	return env != nil && env.Has != nil && env.Has(uikernel.OffSessionHeaderViewAction)
}

// viewEntryRoster 是官方 `visibleTabs` / `moreTabs` 的投影。
//
// 返回（全部、可见、当前激活、座位是否有占用方）。没有占用方时**可见 = 全部**
// （官方列表过滤只在 `hasEntryActions` 下生效，SidebarRoot 同一条判据）；
// 有占用方时按官方 preferential order + 宽度预算切，剩下的进 More。
func (p *Plugin) viewEntryRoster(env *pluginapi.Env) (all, visible []pluginapi.Tab, active string, actionsLive bool) {
	all, active = p.viewRoster(env)
	actionsLive = p.entryActionsLive(env)
	if !actionsLive {
		return all, all, active, false
	}
	preferred := make([]pluginapi.Tab, 0, len(all))
	for _, tab := range all {
		if tab.Key == entrypins.FixedViewID {
			preferred = append(preferred, tab)
		}
	}
	for _, tab := range all {
		if tab.Key == entrypins.FixedViewID {
			continue
		}
		if p.pins != nil && p.pins.Pinned(pluginapi.EntryRegionConversation, tab.Key) {
			preferred = append(preferred, tab)
		}
	}
	limit := p.visibleViewLimit(env)
	if limit > len(preferred) {
		limit = len(preferred)
	}
	return all, preferred[:limit], active, true
}

// visibleViewLimit 是官方 `max(1, floor((row.clientWidth - 48) / 120))`。
//
// 行宽取壳给的 `env.MainWidth` 减掉官方 header 的左右内边距（20 / 28，
// ConversationRoot.module.css 的 `padding:10px 28px 0 20px`）。拿不到宽度时
// 返回一个足够大的值：宁可都把 tab 画出来，也不因测不到宽度把它们藏进菜单。
func (p *Plugin) visibleViewLimit(env *pluginapi.Env) int {
	if env == nil || env.MainWidth <= 0 {
		return len(env.Views) + 1
	}
	row := env.MainWidth - headerPadLeft - headerPadRight
	if row <= 0 {
		return 1
	}
	limit := int(math.Floor(float64((row - viewsMoreRowReserve) / viewsMoreCellBudget)))
	if limit < 1 {
		return 1
	}
	return limit
}

// viewEntryEntries 是官方 `moreTabs` 的 entries：可见 tab 之外的**全部** view，
// 每条带 `{id, label, pinned, active}`。
//
// `active` 是官方那一条投影的第四格（`active: viewTab.id === active?.id`，
// OFF:dsh-client-ui-conversation/src/client/skeleton/ConversationSession.tsx:244-247）：
// 当前 view 落在 More 里时，菜单那一行要画行尾对勾（#17 的菜单行读它）。
func (p *Plugin) viewEntryEntries(all, visible []pluginapi.Tab, active string) []pluginapi.EntryMenuEntry {
	shown := make(map[string]bool, len(visible))
	for _, tab := range visible {
		shown[tab.Key] = true
	}
	out := make([]pluginapi.EntryMenuEntry, 0, len(all))
	for _, tab := range all {
		if shown[tab.Key] {
			continue
		}
		pinned := true
		if p.pins != nil {
			pinned = p.pins.Pinned(pluginapi.EntryRegionConversation, tab.Key)
		}
		out = append(out, pluginapi.EntryMenuEntry{
			ID: tab.Key, Label: tab.Label, Pinned: pinned,
			Active: tab.Key == active && active != "",
		})
	}
	return out
}

// drawViewsMoreButton 画官方 `.viewsMore`（22×22 的 ellipsis 按钮）。
func (p *Plugin) drawViewsMoreButton(f ui.C, env *pluginapi.Env, active string, hidden []pluginapi.Tab) {
	t := p.theme
	inMore := false
	for _, tab := range hidden {
		if tab.Key == active {
			inMore = true
			break
		}
	}
	button := ui.Row(f).Width(viewsMoreWidth).Height(viewsMoreHeight).Shrink(0).
		MarginSides(viewsMoreMarginTop, 0, 0, 0).Radius(t.Radii.SM).Items(ui.Center).Justify(ui.Center)
	switch {
	case p.entryMenu.open:
		// `.viewsMore[aria-expanded=true]{background:interactive-bg-hover}`。
		button = button.Background(t.Hover)
	case inMore:
		// `.viewsMoreActive{color:state-business-primary}`：当前 view 藏在 More 里。
		button = button.Hover(ui.Look{}.Background(t.Hover))
	default:
		button = button.Hover(ui.Look{}.Background(t.Hover))
	}
	color := t.LabelSecondary
	if inMore {
		// `--dsw-alias-state-business-primary` 在本主题里的映射就是 `Theme.Info`
		// （见 plugins/dssidebar/drag.go 的同一条映射说明）。
		color = t.Info
	}
	button = button.Hit(env.Tree.ID(env.Feat, "views-more"), zhSessionMoreViews)
	button.Children(func() {
		if icon, ok := ellipsisIcon(); ok {
			ui.Icon(f, icon).Size(viewsMoreIconSize, viewsMoreIconSize).Color(color)
		}
	})
	p.entryMenu.button, p.entryMenu.buttonDrawn = button, true
	if button.Clicked() {
		p.entryMenu.open = !p.entryMenu.open
	}
}

// afterEntryMenu 把本帧的按钮/菜单面矩形留给下一帧。
func (p *Plugin) afterEntryMenu(canvas *gfx.Canvas, env *pluginapi.Env) {
	if p.entryMenu.buttonDrawn {
		p.entryMenu.buttonDrawn = false
		if rect := p.entryMenu.button.Rect(); rect.Width > 0 && rect.Height > 0 {
			p.entryMenu.anchor = rect
		}
	}
	if p.entryMenu.panelDrawn {
		p.entryMenu.panelDrawn = false
		if rect := p.entryMenu.panel.Rect(); rect.Width > 0 && rect.Height > 0 {
			p.entryMenu.panelRect = rect
		} else {
			p.entryMenu.panelRect = ui.Rect{}
		}
	}
}

// entryMenuChildren 画菜单面并把 `conversation.session.header.views.actions` 派发进去。
func (p *Plugin) entryMenuChildren(f ui.C, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	if !p.entryMenu.open || drawSlot == nil {
		return
	}
	all, visible, active, actionsLive := p.viewEntryRoster(env)
	if !actionsLive {
		// 座位被撤回（插件卸载）而菜单还开着：这一帧直接收起来，不留空菜单面。
		p.entryMenu.open = false
		return
	}
	uikernel.DrawEntryMoreMenu(f, env, uikernel.EntryMenuOptions{
		Theme:       p.theme,
		Anchor:      p.entryMenu.anchor,
		Bounds:      env.Bounds,
		Open:        true,
		Slot:        uikernel.OffSessionHeaderViewAction,
		Region:      pluginapi.EntryRegionConversation,
		Entries:     p.viewEntryEntries(all, visible, active),
		OpenEntry:   func(id string) { p.Select(env.Data.Cur, id) },
		SetPinned:   p.setViewPinned,
		Dismiss:     func() { p.entryMenu.open = false },
		Dispatch:    drawSlot,
		PanelHeight: p.entryMenu.panelRect.Height,
		OnPanel: func(box ui.Box) {
			p.entryMenu.panel, p.entryMenu.panelDrawn = box, true
		},
	})
}

// setViewPinned 是官方 onPinnedChange：改一个 view 的固定态。
func (p *Plugin) setViewPinned(id string, pinned bool) {
	if p.pins == nil {
		return
	}
	p.pins.SetPinned(pluginapi.EntryRegionConversation, id, pinned)
}
