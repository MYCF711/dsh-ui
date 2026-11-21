// terminal_shortcut.go —— 官方 `terminal.new` 快捷键（`Ctrl+Backquote`，即反引号）。
//
// # 官方事实
//
//	OFF:dsh-client-ui-sidebar-terminal/src/client/index.ts:55-70
//	  ctx.shortcuts.register({
//	    id: 'terminal.new', label: () => t('new'), aliases: ['new terminal','shell'],
//	    defaults: { 'desktop:windows': { code:'Backquote', modifiers:['control'] }, … },
//	    regions: ['page','editable','terminal'], modals: [],
//	    resolve: ({ target: element }) => {
//	      const target = ctx.sidebarRight.commandTarget(element)
//	      if (target === undefined) return { status: 'blocked', reason: t('shortcut.noSession') }
//	      return { status: 'handled', run: () => { ctx.sidebarRight.openTabFromTarget('terminal', target) } }
//	    },
//	  })
//	OFF:…/locales.ts:9  'shortcut.noSession': '请先选择会话'
//
// 官方在 desktop 的 macos/windows/linux 与 web 的 macos/windows 五个键位表里
// 登记了同一个 chord：`Ctrl+Backquote`（web 没有 linux 档）。
//
// # 为什么落在壳的 OnShortcut 而不是插件的 HandleKey
//
// 与 `Ctrl+O`（`workspace.add`）同一个理由：viva 的 `OnKey` 只送**具名键**
// （`gfx/input.go:12-47` 的枚举里没有反引号），带 Control 的可打印键在 `OnKey` 上
// 是 `Key == KeyUnknown && Text == "`"`。`app.Config.OnShortcut` 在原生文本处理
// **之前**接收这类和弦（`viva@app/app.go:130-142`），这才是官方
// `primary+Backquote` 的等价入口。走插件的 HandleKey 既收不到这个键，
// 也会和输入框抢字符。
//
// # 两个差异（写清楚，不假装对齐）
//
//	① 官方 `contexts`/`regions` 的三档（page / editable / terminal）本仓没有全局
//	   "当前焦点区域"概念；这里对任何焦点都生效。功能上等价于官方的**并集**，
//	   不会漏、只可能多（在设置浮层里按也会开一枚 tab，那枚 tab 在浮层背后）。
//	② 官方 `modals: []` = 模态打开时不响应。本仓壳没有模态登记表
//	   （设置浮层是 `plugins/dssettings` 自己的绘制状态，壳读不到），所以这条限制
//	   暂时落不了地。它不是"漏做"，缺的是一个壳级模态登记口。
package main

import (
	"strings"

	"github.com/MYCF711/dsh-ui/plugins/dsbrowser"
	"github.com/vivaui/viva/gfx"
)

// terminalNewShortcutNoSession 是官方 `shortcut.noSession` 的中文文案
// （`OFF:dsh-client-ui-sidebar-terminal/src/client/locales.ts:9`）。
const terminalNewShortcutNoSession = "请先选择会话"

// handleTerminalNewShortcut 处理官方 `terminal.new`（`Ctrl+Backquote`）。
//
// 返回 true = 这次按键被吃了（壳必须自己 Invalidate：这条路径绕过了 OnKey 里那句
// 显式重绘，与 `Ctrl+O` 的注释同一个坑）。
func (m *model) handleTerminalNewShortcut(ev gfx.KeyEvent) bool {
	if m == nil || ev.Release {
		return false
	}
	if ev.Key != gfx.KeyUnknown {
		return false
	}
	// 官方默认值是 `{code:'Backquote', modifiers:['control']}`：只有 Control 这一档。
	// Shift/Alt/Meta 一起按下时 Text 也不是纯反引号，但它们仍会到这里，
	// 所以显式挡掉——否则 Ctrl+Shift+` 这类别的 chord 会被我们先吃掉。
	if ev.Modifiers != gfx.ModifierControl {
		return false
	}
	if strings.TrimSpace(ev.Text) != "`" {
		return false
	}
	m.mu.Lock()
	sessionID := m.cur
	m.mu.Unlock()
	if sessionID == "" {
		// 官方 `{ status:'blocked', reason }`：把理由显示出来，**不静默**。
		m.setStatus(terminalNewShortcutNoSession)
		m.inv.Invalidate()
		return true
	}
	// 官方 `openTabFromTarget('terminal', target)`：target 就是被展示的会话，
	// 于是落到 openTab('terminal')——terminal 是 multiple:true，每次新建一枚。
	// 直接调壳的 openRightTab（`Actions.OpenRightTab` 就是它），不再多做一层判空：
	// 这一层本来就在壳里，不存在"插件没接"的那种 nil。
	m.openRightTab(rightTabKindTerminal)
	m.inv.Invalidate()
	return true
}

// shortcutHandler 是 `app.Config.OnShortcut` 的**唯一**实现：窗口级 chord 都在这里。
//
// 现在三条，都是官方注册在**窗口**上的绑定（不是某个插件的 HandleKey）：
//
//	Ctrl+O         `workspace.add` OFF:dsh-client-ui-workspace/src/client/shortcuts.ts:85-89
//	Ctrl+Backquote `terminal.new`  OFF:dsh-client-ui-sidebar-terminal/src/client/index.ts:55-70
//	Ctrl+T         `browser.new`   OFF:dsh-client-ui-sidebar-browser/src/client/index.ts:46-66
//
// 抽成一个函数是为了让"接线"本身有测试：只测 `handleTerminalNewShortcut` 不测
// `cfg.OnShortcut`，把这一行注释掉也照样全绿——那等于没测（2026-11-18 变异实测过）。
func (m *model) shortcutHandler(ev gfx.KeyEvent) gfx.EventResult {
	if m.handleDirectoryAddShortcut(ev) {
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	if m.handleTerminalNewShortcut(ev) {
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	if m.handleBrowserNewShortcut(ev) {
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}

// terminalNewCommandID 是官方快捷键的命令 id（`index.ts:56` 的 `id: 'terminal.new'`）。
const terminalNewCommandID = "terminal.new"

// browserNewCommandID 是官方快捷键的命令 id（`sidebar-browser/src/client/index.ts:48`
// 的 `id: 'browser.new'`）。
const browserNewCommandID = "browser.new"

// handleBrowserNewShortcut 处理官方 `browser.new`（桌面三平台都是 `Primary+T`）。
//
// # 官方事实
//
//	OFF:dsh-client-ui-sidebar-browser/src/client/index.ts:46-66
//	  ctx.inject(['shortcuts'], (ctx) => {
//	    ctx.effect(() => ctx.shortcuts.register({
//	      id: 'browser.new', label: () => t('guide.title'),
//	      aliases: ['browser','new browser tab'],
//	      defaults: {
//	        'desktop:macos':   { code:'KeyT', modifiers:['primary'] },
//	        'desktop:windows': { code:'KeyT', modifiers:['primary'] },
//	        'desktop:linux':   { code:'KeyT', modifiers:['primary'] },
//	        'web:macos':       { code:'KeyT', modifiers:['primary','alt'] },
//	        'web:windows':     { code:'KeyT', modifiers:['primary','alt'] },
//	      },
//	      regions: ['page','editable','terminal'], modals: [],
//	      resolve: ({ target: element }) => {
//	        const target = ctx.sidebarRight.commandTarget(element)
//	        if (target === undefined) return { status:'blocked', reason: t('shortcut.noSession') }
//	        return { status:'handled', run: () => { ctx.sidebarRight.openTabFromTarget('browser', target) } }
//	      },
//	    }), 'ui-sidebar-browser: shortcut')
//	  })
//
// 桌面档（macos/windows/linux）**只有 primary**，web 档才多一个 alt —— 本仓是桌面壳，
// 所以取 `Ctrl+T` 这一支。`Primary` 在 Windows 上就是 Control。
//
// 与 `terminal.new`（`Ctrl+Backquote`）不冲突：官方两张表里没有第三个键位撞上
// `KeyT`（`grep "code: 'KeyT'"` 在 rc.5 的 338 个包里只命中这一段），本仓也没有
// 别处吃 `Ctrl+T`（`grep 'KeyT'` 零命中、没有别的 `Text == "t"` 分支）。
//
// # 两个差异与 `terminal.new` 同源（不假装对齐）
//
//	① 官方 `regions:['page','editable','terminal']` 是"当前焦点区域"三档，本仓没有
//	   这个概念，这里对任何焦点都生效 —— 等价于官方的**并集**，只多不漏。
//	② 官方 `modals: []` = 模态开着时不响应；本仓壳没有模态登记表，落不了地。
//
// 判据与理由见同目录 `terminal_shortcut.go` 文件头：`OnKey` 只送具名键，
// 带 Control 的 `t` 在 `OnKey` 上是 `Key==KeyUnknown && Text=="t"`，必须走
// `OnShortcut` 才能在原生文本处理之前拿到它。
func (m *model) handleBrowserNewShortcut(ev gfx.KeyEvent) bool {
	if m == nil || ev.Release {
		return false
	}
	if ev.Key != gfx.KeyUnknown {
		return false
	}
	// 官方桌面档是 `modifiers:['primary']`：只有 Control 一档。Shift/Alt/Meta 一起
	// 按下时 Text 仍可能是纯 "t"，显式挡掉，免得 Ctrl+Shift+T 这类别的 chord 被先吃。
	if ev.Modifiers != gfx.ModifierControl {
		return false
	}
	if !strings.EqualFold(ev.Text, "t") {
		return false
	}
	m.mu.Lock()
	sessionID := m.cur
	m.mu.Unlock()
	if sessionID == "" {
		// 官方 `{ status:'blocked', reason }`：把理由显示出来，**不静默**。
		m.setStatus(terminalNewShortcutNoSession)
		m.inv.Invalidate()
		return true
	}
	// 官方 `openTabFromTarget('browser', target)`：浏览器是 multiple:true，每次新建
	// 一枚。这条路径与「向导页点浏览器胶囊」完全同一条
	// （`internal/uikernel/view_guide.go:196-197` 的 `OpenRightTab(entry.Kind)`），
	// 所以不会多出一种"空参数的浏览器 tab"形态。
	m.openRightTab(dsbrowser.RightTabBrowser)
	m.inv.Invalidate()
	return true
}

// shortcutKeys 返回命令在当前平台的按键序列（官方 `ctx.shortcuts.catalog` 的投影）。
//
// `terminal.new` 的官方默认值是 `{code:'Backquote', modifiers:['control']}`
// （`index.ts:57-63`，desktop 三平台 + web 的 macos/windows 同值）。官方
// `ShortcutKeys` 的元素表里 `"+"` 单独成一项、走 separator 样式（bundle `Sp`），
// 所以这里也把 `"+"` 作为独立元素交出去，由界面按官方 `._keys_38b9q_1` 的
// gap:3px 排成 `Ctrl + \“。
//
// `browser.new` 同理：桌面档 `{code:'KeyT', modifiers:['primary']}`
// （`sidebar-browser/src/client/index.ts:49-55`），排成 `Ctrl + T`。
func (m *model) shortcutKeys(commandID string) []string {
	switch commandID {
	case terminalNewCommandID:
		return []string{"Ctrl", "+", "`"}
	case browserNewCommandID:
		return []string{"Ctrl", "+", "T"}
	}
	return nil
}
