// right_tab_rename.go —— 右栏芯片的**就地改名**（官方 TerminalTitle 的 write 那一半）。
//
// 官方那一段是纯客户端行为，没有独立的 wire 方法：
//
//	OFF:dsh-client-ui-sidebar-terminal/src/client/TerminalTitle.tsx:25-36
//	  「Dockkit captures the pointer on its drag handle, which receives the
//	   double-click」→ chip.addEventListener('dblclick', () => setEditing(true))
//	OFF:…/TerminalTitle.tsx:51-59
//	  editing ? <input className={css.name} maxLength={120} …/> : <span className={css.title}>{title}</span>
//	OFF:…/TerminalTitle.tsx:55-59
//	  onBlur: next = value.trim(); !cancelled && next !== '' && next !== title && void view(tab.id).rename(next)
//	OFF:…/TerminalTitle.tsx:64-66
//	  Escape: cancelled.current = true; blur()（所以 Esc 这一支**不**提交）
//
// 改名最终落到宿主的 `terminal.rename`（`webTerminals.view(...).rename` →
// `remote.terminal.rename`，客户端只按 occurrence 找 view）。本仓的对应物是
// `Actions.TerminalRenameOfficial`（`terminal_wiring.go` 的 terminalRenameOfficial）。
// 所以这里只做两件事：①把 occurrence 的 kind 翻成"这个 kind 归谁"；②交给插件，
// 由插件发宿主调用——壳不替终端写标题，也不伪造成功。
package main

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// renameRightTab 是 `Actions.RenameRightTab` 的实现（fire-and-forget，与官方 `void` 同形）。
//
// 失败只进状态栏：官方也不在输入框里回填错误（错误进的是视图形状）。
func (m *model) renameRightTab(tabID, title string) {
	if tabID == "" || m.kernel == nil {
		return
	}
	m.mu.Lock()
	sessionID := m.cur
	kind := ""
	if surface := m.rightTabSurfaces[sessionID]; surface != nil {
		for _, tab := range surface.Tabs {
			if tab.ID == tabID {
				kind = tab.Kind
				break
			}
		}
	}
	m.mu.Unlock()
	if sessionID == "" || kind == "" {
		// occurrence 已经不在了：官方那时 input 也已卸载，什么都不做。
		return
	}
	for _, plugin := range m.kernel.Plugins() {
		provider, ok := plugin.(pluginapi.RightTabRenameProvider)
		if !ok {
			continue
		}
		owned, err := provider.RightTabRename(sessionID, tabID, kind, title)
		if !owned {
			continue
		}
		if err != nil {
			m.setStatus("重命名标签失败: " + err.Error())
		}
		return
	}
	// 没有任何实现者认领这个 kind：那是"这个类型不支持改名"，不是静默成功。
	m.setStatus("这个标签的类型不支持重命名")
}

// cancelRightTabRename 处理编辑态下按下的 Esc（官方 `cancelled.current = true` 那一支）。
//
// 返回 true = 这次 Esc 被编辑框吃掉了：壳据此**不再**把 Esc 往下发给插件，
// 否则一次取消会顺带拒掉一条审批请求（dscomposer 的 Esc 就是 reject_once）。
func (m *model) cancelRightTabRename() bool {
	if m.rightTabRename.TabID == "" {
		return false
	}
	m.rightTabRename.End()
	m.inv.Invalidate()
	return true
}
