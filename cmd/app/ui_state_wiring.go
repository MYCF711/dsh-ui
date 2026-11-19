// ui_state_wiring.go —— 界面状态（`uiState.get/set/delete`）的壳侧实现。
//
// # 为什么要有这一层
//
// 官方把一批纯前端记忆放在 Electron `Local Storage` 里；本仓的宿主管道是
// `uiState.*`（落 `<home>/state/ui-state/<key>.json`）。插件不碰 apiclient，
// 只经 `pluginapi.Actions.UIState` 说话，所以壳要在这里做两件事：
//
//	读 —— 同步读缓存；第一次问就回 found=false 并**后台**拉一次，拉完 Invalidate。
//	写 —— 先更新缓存再丢 goroutine；失败只报状态栏。
//
// 同步读缓存不是优化，是**正确性**：插件的 Draw 在绘制线程上，任何一次同步 HTTP
// 都会把首帧卡住（forge_wiring.go 头注同一条理由）。
//
// # 失败怎么记
//
// 一次读失败就按"没有这份状态"缓存住，直到进程重启：界面记忆丢了不致命，而每帧
// 重试会把宿主打满。写失败同样只报一次状态栏，不重试（last-write-wins，下一次
// 点击会自然再写一次）。
package main

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

const (
	// uiStateReadTimeout 是一次后台读的上限。本机读一个小 JSON 是毫秒级，
	// 5 秒足够；超时按"没有"处理（与 fsDirs 同一条口径：不假装有数据）。
	uiStateReadTimeout = 5 * time.Second
	// uiStateWriteTimeout 是一次写入的上限：落盘 + rename，同样是毫秒级。
	uiStateWriteTimeout = 5 * time.Second
)

// uiStateEntry 是一份已读到的界面状态。found=false 表示"宿主说没有"。
type uiStateEntry struct {
	value json.RawMessage
	found bool
}

// uiStateCache 是本进程的界面状态缓存。零值可用。
//
// 自己的锁（不复用 m.mu）：插件每帧都可能来问，而 m.mu 被会话/forge 的读写
// 占着，混用会把绘制线程和网络回包串在一起。
type uiStateCache struct {
	mu      sync.Mutex
	entries map[string]uiStateEntry
	loading map[string]bool
}

// uiStateGet 是 `Actions.UIState.Get`：同步读缓存，首次问触发后台拉。
func (m *model) uiStateGet(key string) (json.RawMessage, bool) {
	if key == "" {
		return nil, false
	}
	cache := &m.uiState
	cache.mu.Lock()
	if entry, ok := cache.entries[key]; ok {
		value, found := entry.value, entry.found
		cache.mu.Unlock()
		return value, found
	}
	if cache.loading[key] || m.cli == nil {
		cache.mu.Unlock()
		return nil, false
	}
	if cache.loading == nil {
		cache.loading = map[string]bool{}
	}
	cache.loading[key] = true
	cache.mu.Unlock()
	go m.uiStateLoad(key)
	return nil, false
}

// uiStateLoad 是一次后台读。回包后放进缓存并 Invalidate，下一帧插件就能读到。
func (m *model) uiStateLoad(key string) {
	ctx, cancel := context.WithTimeout(context.Background(), uiStateReadTimeout)
	defer cancel()
	value, found, err := m.cli.UIStateGet(ctx, key)
	cache := &m.uiState
	cache.mu.Lock()
	delete(cache.loading, key)
	if cache.entries == nil {
		cache.entries = map[string]uiStateEntry{}
	}
	// 失败也记一笔 found=false（见文件头"失败怎么记"），避免每帧重试。
	cache.entries[key] = uiStateEntry{value: value, found: err == nil && found}
	cache.mu.Unlock()
	if err != nil {
		m.setStatus("读界面状态失败: " + err.Error())
	}
	// 零值 Invalidator 是安全的空操作：viva 的 `Waker.Wake` 在没绑 post 原语时
	// 直接 return（`internal/platform/contract.go:754-757`），所以无窗口的测试
	// 与还没建窗口的首帧都不会在这里炸。
	m.inv.Invalidate()
}

// uiStateSet 是 `Actions.UIState.Set`：先改缓存（下一帧读到的就是新值），再异步落盘。
func (m *model) uiStateSet(key string, value json.RawMessage) {
	if key == "" || len(value) == 0 {
		return
	}
	// 拷一份再进 goroutine：插件下一帧可能复用/改写自己的缓冲区。
	payload := append(json.RawMessage(nil), value...)
	cache := &m.uiState
	cache.mu.Lock()
	if cache.entries == nil {
		cache.entries = map[string]uiStateEntry{}
	}
	cache.entries[key] = uiStateEntry{value: payload, found: true}
	delete(cache.loading, key)
	cache.mu.Unlock()
	if m.cli == nil {
		return
	}
	go m.uiStateWrite(key, payload)
}

// uiStateWrite 是一次后台写。失败只报状态栏：界面记忆不是数据，丢了不致命。
func (m *model) uiStateWrite(key string, value json.RawMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), uiStateWriteTimeout)
	defer cancel()
	if err := m.cli.UIStateSet(ctx, key, value); err != nil {
		m.setStatus("保存界面状态失败: " + err.Error())
	}
}

// uiStateDelete 是 `Actions.UIState.Delete`：缓存立刻变"没有"，再异步删盘。
func (m *model) uiStateDelete(key string) {
	if key == "" {
		return
	}
	cache := &m.uiState
	cache.mu.Lock()
	if cache.entries == nil {
		cache.entries = map[string]uiStateEntry{}
	}
	cache.entries[key] = uiStateEntry{found: false}
	delete(cache.loading, key)
	cache.mu.Unlock()
	if m.cli == nil {
		return
	}
	go m.uiStateErase(key)
}

// uiStateErase 是一次后台删。
func (m *model) uiStateErase(key string) {
	ctx, cancel := context.WithTimeout(context.Background(), uiStateWriteTimeout)
	defer cancel()
	if _, err := m.cli.UIStateDelete(ctx, key); err != nil {
		m.setStatus("删除界面状态失败: " + err.Error())
	}
}
