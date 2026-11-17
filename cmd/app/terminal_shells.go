// terminal_shells.go —— 官方向导卡那张 **shell 菜单**的数据面（`webTerminals.launchShells`）。
//
// # 官方事实
//
// 向导卡展开菜单时向宿主问一次"这台机器能起哪些 shell"，并记住浏览器选过的那一个：
//
//	OFF:dsh-api-terminal-controller/src/client/index.ts:169-191
//	  async launchShells(sessionId, signal): Promise<TerminalLaunchShells> {
//	    const result = await this.remote.shells(sessionId, signal)
//	    if (!result.ok) throw new Error(result.error.message)
//	    const previous = preferredShell()
//	    return { shells: result.value,
//	             selectedShell: result.value.find(s => s.path === previous)?.path
//	                            ?? result.value[0]?.path }
//	  }
//	  selectShell(path) { rememberShell(path) }
//	OFF:dsh-client-ui-sidebar-terminal/src/client/TerminalGuide.tsx:24,50-54
//	  type MenuState = {phase:'loading'} | {phase:'ready', choices} | {phase:'failed', message}
//
// # 本仓的形态（如实说明代价）
//
// 绘制帧里没有事件循环（Viva 的 Draw 是同步的），所以这里把那个 Promise 投影成
// **缓存 + 三态**：调用方每帧同步读缓存，壳在后台补一轮请求。这不是"假实现"——
// 同一条 `terminal.shells` 宿主方法、同一份 `TerminalShell` 表、同一个选中回退规则。
package main

import (
	"context"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

const (
	// terminalShellsTTL 是 ready 缓存的新鲜期；过期后下一次读取再补一轮。
	terminalShellsTTL = 60 * time.Second
	// terminalShellsRetryTTL 是 failed 之后允许重试的最短间隔（避免每帧重打）。
	terminalShellsRetryTTL = 2 * time.Second
)

// terminalShellCache 是一个会话的 shell 目录缓存。
type terminalShellCache struct {
	phase     string
	shells    []apiclient.TerminalShell
	selected  string
	message   string
	fetchedAt time.Time
	inflight  bool
}

// terminalShellChoices 是官方 `launchShells` 的同步投影：返回当前缓存，必要时补一轮。
func (m *model) terminalShellChoices() pluginapi.TerminalShellChoices {
	m.mu.Lock()
	defer m.mu.Unlock()
	sessionID := m.cur
	if m.terminalShells == nil {
		m.terminalShells = map[string]*terminalShellCache{}
	}
	cache := m.terminalShells[sessionID]
	if cache == nil {
		cache = &terminalShellCache{phase: pluginapi.TerminalShellsLoading}
		m.terminalShells[sessionID] = cache
	}
	need := !cache.inflight
	switch cache.phase {
	case pluginapi.TerminalShellsReady:
		need = need && time.Since(cache.fetchedAt) > terminalShellsTTL
	case pluginapi.TerminalShellsFailed:
		need = need && time.Since(cache.fetchedAt) > terminalShellsRetryTTL
	default:
		// 还没成功过（loading / 空）：允许补一轮。
		need = need && (cache.fetchedAt.IsZero() || time.Since(cache.fetchedAt) > terminalShellsRetryTTL)
	}
	if need && sessionID != "" {
		cache.inflight = true
		cache.fetchedAt = time.Now()
		if cache.phase == "" {
			cache.phase = pluginapi.TerminalShellsLoading
		}
		go m.fetchTerminalShells(sessionID)
	}
	return pluginapi.TerminalShellChoices{
		Phase:    cache.phase,
		Shells:   append([]apiclient.TerminalShell(nil), cache.shells...),
		Selected: cache.selected,
		Message:  cache.message,
	}
}

// fetchTerminalShells 跑一轮真实 `terminal.shells`，把结果写回缓存。
func (m *model) fetchTerminalShells(sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	shells, err := m.terminalShellsCall(ctx, sessionID)
	m.mu.Lock()
	cache := m.terminalShells[sessionID]
	if cache == nil {
		m.mu.Unlock()
		return
	}
	cache.inflight = false
	cache.fetchedAt = time.Now()
	if err != nil {
		cache.phase = pluginapi.TerminalShellsFailed
		cache.message = err.Error()
	} else {
		cache.phase = pluginapi.TerminalShellsReady
		cache.shells = shells
		cache.selected = pickTerminalShell(shells, m.terminalShellPreferenceLocked(sessionID))
		cache.message = ""
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}

// terminalSelectShell 是官方 `webTerminals.selectShell(path)`：记住下一次新建终端的默认 shell。
func (m *model) terminalSelectShell(path string) {
	if path == "" {
		return
	}
	m.mu.Lock()
	if m.terminalShellPref == nil {
		m.terminalShellPref = map[string]string{}
	}
	m.terminalShellPref[m.cur] = path
	if cache := m.terminalShells[m.cur]; cache != nil {
		cache.selected = path
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}

// terminalShellPreferenceLocked 读会话记住的 shell（调用方持锁）。
func (m *model) terminalShellPreferenceLocked(sessionID string) string {
	if m.terminalShellPref == nil {
		return ""
	}
	return m.terminalShellPref[sessionID]
}

// pickTerminalShell 按官方 `launchShells` 的规则选一个 shell：
//
//	偏好命中表里那一项 → 用它
//	否则               → 第一枚**可创建**的（`TerminalShell.Supported`；nil 按官方
//	                      契约理解为"由 create 最终判定"，所以也算可用）
//	表为空             → 空串（调用方按宿主默认处理）
func pickTerminalShell(shells []apiclient.TerminalShell, preferred string) string {
	if preferred != "" {
		for _, shell := range shells {
			if shell.Path == preferred {
				return shell.Path
			}
		}
	}
	for _, shell := range shells {
		if shell.Supported == nil || *shell.Supported {
			return shell.Path
		}
	}
	return ""
}
