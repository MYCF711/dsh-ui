// Package entrypins 是三个区域「哪些条目被固定」的**纯内存**表。
//
// # 官方事实（rc.2 随包 src/**，逐行核对）
//
//	OFF:dsh-client-ui-sidebar/src/client/panel-pins.ts:1-33
//	  `createPanelPins()`：`unpinned: readonly string[]` + `setPinned(id, pinned)`
//	  `FIXED_PANEL_ID = 'plugins'`（launcher 恒显示、永不进自定义列表）
//	OFF:dsh-client-ui-conversation/src/client/view-pins.ts:1-38
//	  `createViewPins()`：同一个形状，`FIXED_VIEW_ID = 'chat'`
//	OFF:dsh-client-ui-sidebar-right/src/client/tabs/guide/guide-pins.ts:1-52
//	  `createGuidePins()`：同一个形状，但固定判据是**kind**：`FIXED_GUIDE_KIND =
//	  'files'`，id 是 `kind:id`（`guideEntryId`），`id.startsWith('files:')` 即固定
//
// 三份的 `setPinned` 是**逐字相同**的六行逻辑：
//
//	const current = store.getSnapshot().unpinned
//	if (id === FIXED || pinned !== current.includes(id)) return
//	store.set({ unpinned: pinned ? current.filter(e => e !== id) : [...current, id] })
//
// 逐条含义（每条下面都有用例）：
//
//	① `pinned === current.includes(id)` 时**直接返回**：已经是这个态，不重发快照
//	② 固定 = 从表里删掉（保持其余 id 的相对顺序）
//	③ 取消固定 = 追加到表尾（表序 = 取消固定的先后，不是字母序、也不是注册序）
//	④ 固定条目不受理（左栏 `plugins` / 会话头 `chat` / 右栏 `files:*`）
//	⑤ 表里没有的 id 就是固定（默认全部固定）
//	⑥ 已注销的 id **留在表里**不清理（"ids of panels that are not registered stay
//	   listed"，panel-pins.ts:18）
//
// # 内存语义
//
// 三份都**没有任何持久化通道**：官方 spec 用例显式断言 `localStorage` 里不会出现
// 那条老键（`panel-pins.client.spec.ts:19-30` 的 `KEY = 'dsh.slot.manual-order.
// sidebar.panellist.v1'`），新建实例一律从空表开始。本包同样只活在进程里：
// 没有文件、没有配置、没有 Transport 方法；重启/重载即回到"全部固定"。
package entrypins

import (
	"sync"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// 固定条目（官方三个 `FIXED_*` 常量，见包注释）。
const (
	// FixedPanelID 是左栏面板列表里恒显示、永不进自定义列表的那一条
	// （OFF:dsh-client-ui-sidebar/src/client/panel-pins.ts:5 `FIXED_PANEL_ID`）。
	FixedPanelID = "plugins"
	// FixedViewID 是会话头 view tab 行里恒显示的那一个
	// （OFF:dsh-client-ui-conversation/src/client/view-pins.ts:5 `FIXED_VIEW_ID`）。
	FixedViewID = "chat"
	// FixedGuideKind 是右栏 guide 里恒显示的那一类
	// （OFF:dsh-client-ui-sidebar-right/src/client/tabs/guide/guide-pins.ts:5）。
	FixedGuideKind = "files"
)

// Store 是三个区域共用的纯内存固定态表。
//
// 零值可用（内部表按需创建），但生产装配走 New()：壳建一份、按
// `pluginapi.KeyEntryPins` 提供，区域宿主读、菜单内容写。
type Store struct {
	mu sync.Mutex
	// unpinned 按 region 分组存"未固定"的 id；表序 = 取消固定的先后。
	unpinned map[string][]string
}

// New 造一个空表（= 全部固定，官方三个 `create*Pins()` 的初值）。
func New() *Store { return &Store{unpinned: map[string][]string{}} }

var _ pluginapi.EntryPins = (*Store)(nil)

// Pinned 报告某个条目是否固定。不在未固定表里 = 固定（官方语义⑤）。
//
// 未登记的区域按"全部固定"回答：那是没有固定态时的正确默认值，
// 而不是"未知"，所以不 panic、也不写表。
func (s *Store) Pinned(region, id string) bool {
	if s == nil || id == "" {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range s.unpinned[region] {
		if entry == id {
			return false
		}
	}
	return true
}

// Unpinned 返回某个区域未固定的 id（拷贝，调用方随便改）。
//
// 顺序是**取消固定的先后**（官方 `[...current, id]` 的追加序），不是注册序；
// 区域宿主据此决定"哪些条目从区域里拿掉、改由 More 菜单列出"。
func (s *Store) Unpinned(region string) []string {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.unpinned[region]
	if len(list) == 0 {
		return nil
	}
	out := make([]string, len(list))
	copy(out, list)
	return out
}

// SetPinned 设置某条目的固定态，返回是否真的改了。
//
// 与官方 `setPinned` 逐行等价（包注释的六条）：固定条目不受理、已是目标态不重发、
// 固定 = 删除表项、取消固定 = 追加表尾。
func (s *Store) SetPinned(region, id string, pinned bool) bool {
	if s == nil || id == "" || fixedInRegion(region, id) {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unpinned == nil {
		s.unpinned = map[string][]string{}
	}
	current := s.unpinned[region]
	index := -1
	for i, entry := range current {
		if entry == id {
			index = i
			break
		}
	}
	// 官方判据：`pinned !== current.includes(id)` 时**直接返回**（已是目标态）；
	// 只有"当前未固定且要求固定" / "当前已固定且要求取消" 才真的改表。
	if (index >= 0) != pinned {
		return false
	}
	if pinned {
		next := make([]string, 0, len(current)-1)
		next = append(next, current[:index]...)
		next = append(next, current[index+1:]...)
		if len(next) == 0 {
			delete(s.unpinned, region)
			return true
		}
		s.unpinned[region] = next
		return true
	}
	// 取消固定：**追加到表尾**（官方 `[...current, id]`，不排序、不去重后的重排）。
	s.unpinned[region] = append(current, id)
	return true
}

// fixedInRegion 是官方三处 `FIXED_*` 判据的合并形态。
func fixedInRegion(region, id string) bool {
	switch region {
	case pluginapi.EntryRegionSidebar:
		return id == FixedPanelID
	case pluginapi.EntryRegionConversation:
		return id == FixedViewID
	case pluginapi.EntryRegionRightbar:
		// 右栏 guide 的固定判据是 kind 前缀（`kind:id`，guide-pins.ts:46）。
		return len(id) > len(FixedGuideKind) && id[:len(FixedGuideKind)+1] == FixedGuideKind+":"
	default:
		return false
	}
}
