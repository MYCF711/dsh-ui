// desktop_update_wiring.go —— 「桌面更新指示器」的数据面接线（壳 ↔ 宿主）。
//
// # 它接的是哪条官方链
//
// 官方 `ui-settings-general` 的 `DesktopUpdateIndicator`（设置行第 3 格）读的是
// `DesktopUpdateView`（`types.ts:38-42`），后者的唯一来源是 Electron 主进程经
// 隔离 preload 桥暴露的 `DesktopUpdateBridge`（`types.ts:30-35`）：
//
//	status()    -> DesktopUpdatePresentation
//	open()      -> void（官方自己弹原生面板）
//	subscribe() -> 状态变化的推送
//
// 本仓是 Go 宿主 + Go GUI，没有 Electron 主进程，所以宿主提供了**同形**的壳服务
// （DSH-GO `desktopUpdate.status` / `.open` / `.subscribe`）。本文件就是壳侧那一层：
// 把宿主的方法变成 `pluginapi.Snapshot.DesktopUpdate`。
//
// # 为什么选**轮询** status，不选 subscribe 流（任务书要求说明取舍）
//
// 三条，权重从高到低：
//
//  1. **宿主 `status()` 本来就是为"被反复问"设计的**：它不阻塞（
//     `internal/updater/status.go` 的 `Status()`），冷缓存时回 `checking` 并在
//     后台起一次检查，**同一个检查只会有一个在飞**（单飞），并且**有 TTL 缓存**
//     （默认 5 分钟）+ ETag 条件请求。也就是说轮询它**不会**打爆 GitHub：
//     TTL 内问一万次也只出一次网。subscribe 相对轮询省下的那点调用次数，
//     换不到任何真实收益。
//
//  2. **GUI 壳没有 NDJSON 流客户端**。`internal/apiclient` 只有
//     `POST /api/<method>`（`Client.Call`）和 WS 会话订阅（`mux.go`）；
//     `desktopUpdate.subscribe` 是 `Content-Type: application/x-ndjson` 的
//     HTTP 长连接。为它新写一套流客户端 = 新增长连接生命周期、重连退避、
//     与既有连接指示器的两套"连接态"互相打架（官方那条 subscribe 本身也不带
//     重连，靠 preload 的 IPC 天然不会断）。
//
//  3. **指示器要画的四个相里，只有一个需要"尽快"**：`checking -> available|idle|error`。
//     这个用"回问"解决最直接——`status()` 回 `checking` 时**再问一次**，
//     而不是等下一次轮询周期（见下方 `desktopUpdateAskGap`）。
//
// 反过来说：如果将来宿主把 `status()` 改成阻塞、或去掉 TTL，
// 这个取舍要重新算——那时轮询会变成"每次心跳一次真出网"。
//
// # 四个相里我们只会看到两个的"下一步"
//
// 宿主只产 `idle` / `checking` / `available` / `error` 四态（没有下载器，
// `downloading` / `verifying` / `installing` / `ready` 永不出现）。
// 所以本文件的轮询循环只在 `checking` 上重问；其余三相都是终态，拿到就收手。
package main

import (
	"context"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

const (
	// desktopUpdateTimeout 是一次 `status` / `open` 调用的上限。
	//
	// 比 settingWriteTimeout（8s）宽：`open()` 绕过 TTL **真查一次** GitHub
	// （宿主注释：「那是用户点了的语义」），一次 CONNECT + TLS + API 往返
	// 在上面一圈实测里接近 1s，慢网络下 8s 会误报失败。
	desktopUpdateTimeout = 20 * time.Second

	// desktopUpdateAskGap 是 `checking` 之后**回问**的间隔。
	//
	// 宿主那一次后台检查是"一次 GitHub API 往返"，实测 0.3~1.2s；400ms 让
	// 典型情况下第 2 次就拿到结论，又不至于把宿主问成忙等。
	desktopUpdateAskGap = 400 * time.Millisecond

	// desktopUpdateAskAttempts 是回问次数上限（含第一次）。
	//
	// 12 × 400ms ≈ 4.4s。超了就把 `checking` 原样交出去（界面画 spinner），
	// 下一次轮询周期再问——**不编结论**。
	desktopUpdateAskAttempts = 12

	// desktopUpdatePollInterval 是"观察者还在，但结论在 TTL 内"时的重问周期。
	//
	// 取 5 分钟，与宿主默认 TTL 同量级：比它短是白问（宿主回缓存），
	// 比它长会让"上游半小时前发了新版"迟迟不出现。真出网的次数仍然由宿主
	// 的 TTL 决定，本常量只决定"我们多勤快地去读一次缓存"。
	desktopUpdatePollInterval = 5 * time.Minute
)

// desktopUpdatePhaseChecking 是宿主"还在查"的那一相（`internal/updater/status.go`）。
//
// 只在本文件里用一次：判断"要不要回问"。它不是一份 phase 词表——
// 词表在 `pluginapi.DesktopUpdatePhase`。
const desktopUpdatePhaseChecking = "checking"

// desktopUpdateSnapshot 是给插件用的**同步**快照（每帧都会被问）。
//
// 三条语义，与 `settingsDescribeSnapshot` 同一套：
//
//	① 返回缓存（可能是 nil = 还不知道），**绝不在这里发请求**——插件在 draw 里调，
//	   等 HTTP 会卡首帧；
//	② 首次问 / 距上次结论超过 `desktopUpdatePollInterval` → 触发一次后台拉取（防重入）；
//	③ `Opening` 是**动作态**不是宿主事实，每帧现取（用户点了到回调回来之间为真）。
//
// 为什么"没有观察者"就不出网这件事在这里自动成立：本函数只在设置行那一格被画到
// 时才被调用（`Snapshot` 装配处），没人画就没人问；宿主侧也只在被问时才出网。
func (m *model) desktopUpdateSnapshot() *pluginapi.DesktopUpdatePresentation {
	m.mu.Lock()
	cached := m.desktopUpdate
	failed := m.desktopUpdateFailed
	opening := m.desktopUpdateOpening
	pending := m.desktopUpdatePending
	lastAsk := m.desktopUpdateAt
	m.mu.Unlock()

	if !pending && (cached == nil || time.Since(lastAsk) >= desktopUpdatePollInterval) {
		m.mu.Lock()
		m.desktopUpdatePending = true
		m.mu.Unlock()
		go m.refreshDesktopUpdate()
	}

	if cached == nil && !failed {
		// 官方 `state === undefined && !failed` → 整枚不渲染。
		return nil
	}
	out := pluginapi.DesktopUpdatePresentation{Opening: opening, Failed: failed}
	if cached != nil {
		out.Phase = pluginapi.DesktopUpdatePhase(cached.Phase)
		out.Version = cached.Version
		if cached.Percent != nil {
			out.Percent = *cached.Percent
		}
		out.Failure = pluginapi.DesktopUpdateFailureKind(cached.Failure)
		out.ConfigurationError = cached.ConfigurationError
	}
	return &out
}

// refreshDesktopUpdate 后台拉一次更新状态，`checking` 时回问到有结论为止。
func (m *model) refreshDesktopUpdate() {
	var (
		pres apiclient.DesktopUpdatePresentation
		err  error
	)
	for attempt := 0; attempt < desktopUpdateAskAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), desktopUpdateTimeout)
		pres, err = m.cli.DesktopUpdateStatus(ctx)
		cancel()
		if err != nil {
			break
		}
		if pres.Phase != desktopUpdatePhaseChecking {
			break
		}
		if attempt == desktopUpdateAskAttempts-1 {
			break
		}
		time.Sleep(desktopUpdateAskGap)
	}

	m.mu.Lock()
	m.desktopUpdatePending = false
	m.desktopUpdateAt = time.Now()
	switch {
	case err != nil && isCapabilityMissing(err):
		// 宿主没这条能力 = 官方**根本没有 `dshDesktop.updates` 桥**。
		//
		// 官方那种情况下 `DesktopUpdateSource` 的 `bridge` 是 undefined：
		// 既不订阅也不调 status，`failed` 永远 false、`presentation` 永远 undefined
		// ⇒ 整枚指示器不渲染。所以这里要回到"什么都没有"，**不是** failed——
		// 把"宿主没这个能力"画成"更新出错了，点重试"是编了一条不存在的事实。
		m.desktopUpdate = nil
		m.desktopUpdateFailed = false
	case err != nil:
		// 桥**存在**但这次调用失败了：官方 `status()` 被 reject →
		// `store.set({failed: true})`（`desktop-update-source.ts:26-28`）。
		// 界面据此把 label 换成「重试更新」，所以这里不能把失败吞掉。
		m.desktopUpdate = nil
		m.desktopUpdateFailed = true
	default:
		m.desktopUpdate = &pres
		m.desktopUpdateFailed = false
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}

// desktopUpdateOpen 是官方 `DesktopUpdateBridge.open()` 的壳侧实现。
//
// 完整语义见 `pluginapi.Actions.DesktopUpdateOpen` 的注释。这里只写实现要点：
//
//   - `Opening` 期间**直接返回**（官方 `DesktopUpdateSource.open()`：
//     `if (state.opening || …) return`），并发点击不会发出两次请求；
//   - 宿主返回的 `presentation` **直接收下**（宿主那一次绕过了 TTL，比缓存新），
//     省掉一次"打开完还要再问一次"；
//   - URL 交给**系统默认浏览器**（`m.openExternal` → `openURL` → viva `app.OpenURL`）。
//     壳里**不嵌浏览器**：官方那一下弹的是原生更新面板，"在应用里开一个网页"
//     既不是官方的形状，也把外链变成了一条应用内可伪造的表面；
//   - 任何一步失败都**写状态栏**（`openExternal` 自己也写，所以只补前面两步），
//     并且按官方记 `failed = true`——用户点了必须能看到"没成"。
func (m *model) desktopUpdateOpen() {
	m.mu.Lock()
	if m.desktopUpdateOpening {
		m.mu.Unlock()
		return
	}
	m.desktopUpdateOpening = true
	m.mu.Unlock()
	m.inv.Invalidate()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), desktopUpdateTimeout)
		result, err := m.cli.DesktopUpdateOpen(ctx)
		cancel()

		if err != nil {
			missing := isCapabilityMissing(err)
			m.mu.Lock()
			m.desktopUpdateOpening = false
			if !missing {
				// 桥存在但这次动作失败：官方 `open()` 的 catch 也是
				// `store.set({failed: true})`（`desktop-update-source.ts:40-42`）。
				m.desktopUpdateFailed = true
			}
			m.mu.Unlock()
			if missing {
				// 宿主没接线：不是"更新失败"，是这条能力不存在——别报一条假错。
				m.setStatus("桌面更新：宿主未提供更新入口")
			} else {
				m.setStatus("打开更新入口失败: " + err.Error())
			}
			return
		}

		pres := result.Presentation
		m.mu.Lock()
		m.desktopUpdateOpening = false
		m.desktopUpdateFailed = false
		m.desktopUpdate = &pres
		m.desktopUpdateAt = time.Now()
		m.mu.Unlock()
		m.inv.Invalidate()

		if result.URL == "" {
			// 宿主契约说 `url` 一定非空；真拿到空的就如实说，不假装打开了。
			m.setStatus("更新入口为空（宿主没有给 URL）")
			return
		}
		// 唤起系统默认浏览器。失败时 `openExternal` 自己写状态栏。
		_ = m.openExternal(result.URL)
	}()
}
