// right_tab_occurrences.go —— 右栏 tab occurrence 与 Session 级恢复。
//
// 官方把「类型」与「打开记录」分开：SidebarRightTabDefinition.multiple 决定
// openTab(kind) 是否每次新建；TabRecord.id 才是页面状态、标题与关闭动作的归属。
// 本文件保存本仓单 pane 布局所需的子集，不伪装成 dockkit 的分屏/浮动布局。
package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
)

const rightSidebarPersistence = "dsh.sidebar-right.v1"

// rightTabRecord 是本仓单 pane 中的 TabRecord 子集。
// ID 是 occurrence 身份；Kind 是 keyed 槽派发与插件归属。
type rightTabRecord struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

// rightTabSurface 是每会话的持久化布局。空 Tabs 是合法状态（全部关闭），
// 与「尚未读到配置」严格区分，不能在恢复后偷偷重新播种浏览器。
type rightTabSurface struct {
	Tabs        []rightTabRecord `json:"tabs"`
	ActiveTabID string           `json:"activeTabId,omitempty"`
	Minted      int              `json:"minted"`
}

type rightSidebarEnvelope struct {
	BySession map[string]*rightTabSurface `json:"bySession"`
}

func rightSidebarConfigKey(sessionID string) string {
	return rightSidebarPersistence + "." + sessionID
}

// defaultRightTabSurface 保留旧版「每个注册类型先有一个 tab」的产品入口。
// 第一批 ID 继续等于 kind，避免旧调用方与旧选择状态失效；后续 multiple
// occurrence 使用官方同形的 tab<N>。
func defaultRightTabSurface(types []pluginapi.Tab) *rightTabSurface {
	surface := &rightTabSurface{Minted: len(types)}
	for _, tab := range types {
		kind := tab.Kind
		if kind == "" {
			kind = tab.Key
		}
		surface.Tabs = append(surface.Tabs, rightTabRecord{ID: kind, Kind: kind})
	}
	if len(surface.Tabs) > 0 {
		surface.ActiveTabID = surface.Tabs[0].ID
	}
	return surface
}

func decodeRightTabSurface(sessionID, raw string) (*rightTabSurface, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var envelope rightSidebarEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return nil, err
	}
	surface := envelope.BySession[sessionID]
	if surface == nil {
		return nil, nil
	}
	seen := map[string]bool{}
	maxMint := 0
	for _, tab := range surface.Tabs {
		if tab.ID == "" || tab.Kind == "" || seen[tab.ID] {
			return nil, fmt.Errorf("无效或重复的 tab occurrence")
		}
		seen[tab.ID] = true
		if strings.HasPrefix(tab.ID, "tab") {
			if n, err := strconv.Atoi(strings.TrimPrefix(tab.ID, "tab")); err == nil && n > maxMint {
				maxMint = n
			}
		}
	}
	if surface.Minted < maxMint {
		return nil, fmt.Errorf("minted=%d 小于已分配 tab id %d", surface.Minted, maxMint)
	}
	if surface.ActiveTabID != "" && !seen[surface.ActiveTabID] {
		return nil, fmt.Errorf("activeTabId=%q 不在 tabs 中", surface.ActiveTabID)
	}
	return surface, nil
}

func encodeRightTabSurface(sessionID string, surface *rightTabSurface) (string, error) {
	copySurface := &rightTabSurface{ActiveTabID: surface.ActiveTabID, Minted: surface.Minted,
		Tabs: append([]rightTabRecord(nil), surface.Tabs...)}
	raw, err := json.Marshal(rightSidebarEnvelope{BySession: map[string]*rightTabSurface{sessionID: copySurface}})
	return string(raw), err
}

// rightTabsFor 返回当前会话的 occurrence 名录，并叠加插件的实时标题。
// 配置未到之前先用默认内存布局；配置到达后，只要用户尚未改动，就用保存布局替换。
func (m *model) rightTabsFor() []pluginapi.Tab {
	if m.kernel == nil {
		return nil
	}
	types := uikernel.RightTabs(m.kernel)
	m.mu.Lock()
	sessionID := m.cur
	if sessionID == "" {
		m.mu.Unlock()
		return types
	}
	if m.rightTabSurfaces == nil {
		m.rightTabSurfaces = map[string]*rightTabSurface{}
	}
	if m.rightTabLoaded == nil {
		m.rightTabLoaded = map[string]bool{}
	}
	if m.rightTabDirty == nil {
		m.rightTabDirty = map[string]bool{}
	}
	surface := m.rightTabSurfaces[sessionID]
	var restoreErr error
	merged := false
	if !m.rightTabLoaded[sessionID] && uiConfig != nil {
		m.rightTabLoaded[sessionID] = true
		saved, err := decodeRightTabSurface(sessionID, uiConfig[rightSidebarConfigKey(sessionID)])
		if err != nil {
			restoreErr = err
		} else if saved != nil {
			if m.rightTabDirty[sessionID] {
				mergeRightTabSurfaces(surface, saved)
				merged = true
			} else {
				surface = saved
				m.rightTabSurfaces[sessionID] = saved
				if m.rightTabSel == nil {
					m.rightTabSel = map[string]string{}
				}
				m.rightTabSel[sessionID] = saved.ActiveTabID
			}
		}
	}
	if surface == nil {
		surface = defaultRightTabSurface(types)
		m.rightTabSurfaces[sessionID] = surface
	}
	tabs := projectRightTabs(types, surface)
	m.mu.Unlock()
	if merged {
		m.persistRightTabSurface(sessionID)
	}
	if restoreErr != nil {
		m.setStatus("右栏恢复数据无效，已忽略: " + restoreErr.Error())
	}
	for _, plugin := range m.kernel.Plugins() {
		provider, ok := plugin.(pluginapi.RightTabTitleProvider)
		if !ok {
			continue
		}
		for i := range tabs {
			if title := provider.RightTabTitle(sessionID, tabs[i].Key); title != "" {
				tabs[i].Label = title
			}
		}
	}
	// 芯片徽标（官方 `sidebar.right.pane.tab.title` 的 TerminalTitle 会在标题前画一枚
	// `Tag`）：与标题同一套「问插件一遍」的模式，每帧取值，因为控制权与沙箱是
	// 跟着宿主终端状态变的实时事实（TerminalTitle.tsx:18-20）。
	for _, plugin := range m.kernel.Plugins() {
		provider, ok := plugin.(pluginapi.RightTabBadgeProvider)
		if !ok {
			continue
		}
		for i := range tabs {
			if badge, ok := provider.RightTabBadge(sessionID, tabs[i].Key); ok {
				badge := badge
				tabs[i].Badge = &badge
			}
		}
	}
	return tabs
}

func projectRightTabs(types []pluginapi.Tab, surface *rightTabSurface) []pluginapi.Tab {
	if surface == nil {
		return nil
	}
	byKind := map[string]pluginapi.Tab{}
	for _, tab := range types {
		kind := tab.Kind
		if kind == "" {
			kind = tab.Key
		}
		tab.Kind = kind
		byKind[kind] = tab
	}
	seenSingle := map[string]bool{}
	out := make([]pluginapi.Tab, 0, len(surface.Tabs))
	for _, record := range surface.Tabs {
		tab, ok := byKind[record.Kind]
		if !ok || seenSingle[record.Kind] && !tab.Multiple {
			continue
		}
		seenSingle[record.Kind] = true
		tab.Key = record.ID
		tab.Order = len(out)
		out = append(out, tab)
	}
	return out
}

// mergeRightTabSurfaces 合并「配置到达前已做的选择/新建」与旧布局。
// 关闭在配置就绪前会被拒绝，所以这里不会产生关闭记录复活。
func mergeRightTabSurfaces(current, saved *rightTabSurface) {
	if current == nil || saved == nil {
		return
	}
	seen := map[string]bool{}
	for _, tab := range current.Tabs {
		seen[tab.ID] = true
	}
	for _, tab := range saved.Tabs {
		if !seen[tab.ID] {
			current.Tabs = append(current.Tabs, tab)
			seen[tab.ID] = true
		}
	}
	if saved.Minted > current.Minted {
		current.Minted = saved.Minted
	}
}

func (m *model) markRightTabSelectionLocked(sessionID, key string) {
	if m.rightTabSel == nil {
		m.rightTabSel = map[string]string{}
	}
	m.rightTabSel[sessionID] = key
	if surface := m.rightTabSurfaces[sessionID]; surface != nil {
		surface.ActiveTabID = key
		m.rightTabDirty[sessionID] = true
	}
}

// openRightTab 实现官方 openTab(kind)（不带 params 的那一支）：multiple 每次都新建，
// 其他类型复用。
func (m *model) openRightTab(kind string) {
	m.openRightTabParams(kind, pluginapi.RightTabParams{})
}

// rightTabParamsKey 是右栏导航参数的内存键（会话 + occurrence）。
func rightTabParamsKey(sessionID, tabID string) string { return sessionID + "\x00" + tabID }

// rightTabParamsFor 读某个 occurrence 的官方 `tab.navigation.params`
// （contract/slots.ts:109-121）。零值 = 这次打开没带参数（官方 `undefined`）。
func (m *model) rightTabParamsFor(sessionID, tabID string) pluginapi.RightTabParams {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rightTabParams == nil {
		return pluginapi.RightTabParams{}
	}
	return m.rightTabParams[rightTabParamsKey(sessionID, tabID)]
}

// openRightTabParams 实现官方 `openTab(kind, { params })`
// （OFF:dsh-client-ui-sidebar-right/src/client/contract/slots.ts:174-179）：
// multiple 类型每次新建 occurrence，其他类型复用并重新导航——**两种情况下 params
// 都挂到最终选中的那个 occurrence 上**（官方 navigation.revision 每次 open 都 +1，
// `slots.ts:115-120`）。
//
// 为什么 params 只活在内存：见 model.rightTabParams 的注释。
func (m *model) openRightTabParams(kind string, params pluginapi.RightTabParams) {
	if kind == "" {
		return
	}
	m.mu.Lock()
	sessionID := m.cur
	ready := m.rightTabLoaded[sessionID] || uiConfig != nil
	m.mu.Unlock()
	if sessionID != "" && !ready {
		m.setStatus("右栏配置尚未加载，请稍后重试新建")
		return
	}
	tabs := m.rightTabsFor()
	var definition *pluginapi.Tab
	for i := range tabs {
		tabKind := tabs[i].Kind
		if tabKind == "" {
			tabKind = tabs[i].Key
		}
		if tabKind == kind {
			definition = &tabs[i]
			break
		}
	}
	if definition == nil {
		// 全部关闭后名录为空，也要能从注册类型重新打开。
		for _, tab := range uikernel.RightTabs(m.kernel) {
			tabKind := tab.Kind
			if tabKind == "" {
				tabKind = tab.Key
			}
			if tabKind == kind {
				copy := tab
				definition = &copy
				break
			}
		}
	}
	if definition == nil {
		return
	}
	m.mu.Lock()
	sessionID = m.cur
	if sessionID == "" {
		m.mu.Unlock()
		return
	}
	surface := m.rightTabSurfaces[sessionID]
	if surface == nil {
		surface = defaultRightTabSurface(nil)
		m.rightTabSurfaces[sessionID] = surface
	}
	selected := ""
	if !definition.Multiple {
		for _, tab := range surface.Tabs {
			if tab.Kind == kind {
				selected = tab.ID
				break
			}
		}
	}
	if selected == "" {
		surface.Minted++
		selected = fmt.Sprintf("tab%d", surface.Minted)
		surface.Tabs = append(surface.Tabs, rightTabRecord{ID: selected, Kind: kind})
	}
	m.markRightTabSelectionLocked(sessionID, selected)
	if m.rightTabParams == nil {
		m.rightTabParams = map[string]pluginapi.RightTabParams{}
	}
	// opener 没给参数 = 官方 `navigation.params === undefined`；这时**清掉**旧值，
	// 否则复用同一 occurrence 时会读到上一次打开的参数（官方每次 open 都换导航记录）。
	m.rightTabParams[rightTabParamsKey(sessionID, selected)] = params
	m.rightTabDirty[sessionID] = true
	m.showRightbar = true
	if m.rightbarSessions == nil {
		m.rightbarSessions = map[string]rightbarSessionState{}
	}
	m.rightbarSessions[sessionID] = rightbarSessionState{open: true, explicit: true}
	m.mu.Unlock()
	m.persistRightTabSurface(sessionID)
	m.inv.Invalidate()
}

// closeRightTab 先跑插件清理，再提交布局删除；清理失败时 tab 保留。
func (m *model) closeRightTab(key string) {
	if key == "" {
		return
	}
	m.mu.Lock()
	sessionID := m.cur
	surface := m.rightTabSurfaces[sessionID]
	ready := m.rightTabLoaded[sessionID] || uiConfig != nil
	m.mu.Unlock()
	if sessionID != "" && !ready {
		m.setStatus("右栏配置尚未加载，请稍后重试关闭")
		return
	}
	if sessionID == "" || surface == nil {
		return
	}
	found := false
	kind := ""
	for _, tab := range surface.Tabs {
		if tab.ID == key {
			found = true
			kind = tab.Kind
			break
		}
	}
	if !found {
		return
	}
	for _, plugin := range m.kernel.Plugins() {
		if handler, ok := plugin.(pluginapi.RightTabCloseHandler); ok {
			if err := handler.CloseRightTab(sessionID, key, kind, m.actions); err != nil {
				m.setStatus("关闭右栏标签失败: " + err.Error())
				return
			}
		}
	}
	m.mu.Lock()
	surface = m.rightTabSurfaces[sessionID]
	if surface == nil {
		m.mu.Unlock()
		return
	}
	index := -1
	for i, tab := range surface.Tabs {
		if tab.ID == key {
			index = i
			break
		}
	}
	if index < 0 {
		m.mu.Unlock()
		return
	}
	surface.Tabs = append(surface.Tabs[:index], surface.Tabs[index+1:]...)
	// occurrence 消失 = 官方那次导航记录也没了：参数不跨关闭保留。
	delete(m.rightTabParams, rightTabParamsKey(sessionID, key))
	if surface.ActiveTabID == key {
		surface.ActiveTabID = ""
		if len(surface.Tabs) > 0 {
			if index >= len(surface.Tabs) {
				index = len(surface.Tabs) - 1
			}
			surface.ActiveTabID = surface.Tabs[index].ID
		}
	}
	m.markRightTabSelectionLocked(sessionID, surface.ActiveTabID)
	m.rightTabDirty[sessionID] = true
	if len(surface.Tabs) == 0 {
		m.showRightbar = false
		if m.rightbarSessions == nil {
			m.rightbarSessions = map[string]rightbarSessionState{}
		}
		m.rightbarSessions[sessionID] = rightbarSessionState{open: false, explicit: true}
	}
	m.mu.Unlock()
	m.persistRightTabSurface(sessionID)
	m.inv.Invalidate()
}

// persistRightTabSurface 使用官方持久化 namespace，把本仓单 pane 子集写进
// 宿主 ui-config。ConfigSet 的失败由壳统一显示，不在内存里伪报落盘成功。
func (m *model) persistRightTabSurface(sessionID string) {
	m.mu.Lock()
	if uiConfig == nil && !m.rightTabLoaded[sessionID] {
		m.mu.Unlock()
		return
	}
	surface := m.rightTabSurfaces[sessionID]
	if surface == nil {
		m.mu.Unlock()
		return
	}
	copy := &rightTabSurface{ActiveTabID: surface.ActiveTabID, Minted: surface.Minted,
		Tabs: append([]rightTabRecord(nil), surface.Tabs...)}
	m.mu.Unlock()
	raw, err := encodeRightTabSurface(sessionID, copy)
	if err != nil {
		m.setStatus("保存右栏布局失败: " + err.Error())
		return
	}
	key := rightSidebarConfigKey(sessionID)
	m.mu.Lock()
	if uiConfig == nil {
		uiConfig = map[string]string{}
	}
	uiConfig[key] = raw
	m.mu.Unlock()
	if m.actions.ConfigSet != nil {
		m.actions.ConfigSet(map[string]string{key: raw})
	}
}
