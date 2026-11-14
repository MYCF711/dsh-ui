// view_tabs_wiring.go —— 壳侧把两个「多视图座位」的选择接到插件面上。
//
// 机制本体在 `internal/uikernel/view_tabs.go`（注册表投影 + 选择器绘制），
// 这里只做三件事：**读当前选中项**、**写选中项**、**把名录交给 Env**。
//
// 两个座位的选择存在两处，各有理由：
//
//	会话 view   存在 dstopbar 提供的 `viewapi.Store` 里。那是本仓插件间的既有
//	            唯一出处（dsrightbar 的轨迹视图靠它决定要不要接管正文），
//	            壳另起一份就变成两个真相。
//	右栏 tab    存在壳的模型里（`rightTabSel`）。它没有别的所有者，官方那边
//	            也是 dockkit 的 pane 状态而不是某个业务插件的。
//
// 两处都按**会话**分开存：官方 view / tab 的选择也是 per-session 的
// （OFF:dsh-client-ui-conversation/src/client/contract/views.ts:18-25 的
// ConversationStoreState.view 就是 per-session）。
package main

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dstopbar/viewapi"
)

// viewStore 取会话视图选择表；没装配 dstopbar（或它没 Provide）时返回 nil。
//
// 不把它做成启动期断言：测试与最小装配都会只装一部分插件，那时"没有视图表"
// 是**正常状态**，不是错误——调用点按 nil 处理即可。
func (m *model) viewStore() viewapi.Store {
	if m.kernel == nil {
		return nil
	}
	store, _ := m.kernel.Get(viewapi.ServiceKey).(viewapi.Store)
	return store
}

// activeViewKey 是这一帧主区该画哪个 view。
//
// **一定回名录里的 key，或者空串**：存的偏好可能早就失效了（选了某个扩展的
// view，然后那个插件被卸载），直接把原值交出去会让"壳画的是 chat、页签上没有
// 一个是选中态"——那是界面上一眼可见的自相矛盾。所以这里过一遍官方那三条判据
// （`uikernel.ResolveActiveView`）：偏好命中 → 用它；否则 chat；再没有 → 空串。
//
// 空串 = 主区不画 view（官方 `DefaultConversationViews` 的 `viewId === undefined`）。
func (m *model) activeViewKey() string {
	store := m.viewStore()
	m.mu.Lock()
	cur := m.cur
	m.mu.Unlock()
	if store == nil || cur == "" {
		return ""
	}
	tab, ok := uikernel.ResolveActiveView(uikernel.ViewTabs(m.kernel), store.Active(cur))
	if !ok {
		return ""
	}
	return tab.Key
}

// selectViewKey 实现 `Actions.SelectView`：只切"看哪一个"，不发请求、不 reload。
//
// 名录之外的 key **不写**：官方 `resolveActiveView` 也不会因为存了一个不存在的
// id 就创造出视图（view-selection.ts:19-21），这里保持一致——否则界面会出现
// 「点了一个不存在的页签，主区整块空白」。
func (m *model) selectViewKey(view string) {
	if view == "" {
		return
	}
	known := false
	for _, tab := range uikernel.ViewTabs(m.kernel) {
		if tab.Key == view {
			known = true
			break
		}
	}
	if !known {
		return
	}
	store := m.viewStore()
	if store == nil {
		return
	}
	m.mu.Lock()
	cur := m.cur
	m.mu.Unlock()
	if cur == "" {
		return
	}
	store.Select(cur, view)
	m.inv.Invalidate()
}

// activeRightTabKey 是这一帧右栏面板该画哪个 tab 类型。
//
// 解析规则在 `uikernel.ResolveActiveRightTab`（官方 `defaultSeed` 的移植，
// 差异写在那个函数的注释里）。名录为空时返回空串 = 右栏没有可画的面板。
func (m *model) activeRightTabKey(tabs []pluginapi.Tab) string {
	m.mu.Lock()
	cur := m.cur
	m.mu.Unlock()
	selected := ""
	if cur != "" && m.rightTabSel != nil {
		selected = m.rightTabSel[cur]
	}
	tab, ok := uikernel.ResolveActiveRightTab(tabs, selected)
	if !ok {
		return ""
	}
	return tab.Key
}

// selectRightTabKey 实现 `Actions.SelectRightTab`：把右栏切到名录里的某个 tab。
//
// 与 selectViewKey 同一条规矩：名录之外的 key 不写。空会话也不写——
// 「切了个 tab 但下次开新会话又回到默认」比"什么都不做"更难解释。
func (m *model) selectRightTabKey(key string) {
	if key == "" {
		return
	}
	known := false
	for _, tab := range uikernel.RightTabs(m.kernel) {
		if tab.Key == key {
			known = true
			break
		}
	}
	if !known {
		return
	}
	m.mu.Lock()
	if m.cur != "" {
		if m.rightTabSel == nil {
			m.rightTabSel = map[string]string{}
		}
		m.rightTabSel[m.cur] = key
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}
