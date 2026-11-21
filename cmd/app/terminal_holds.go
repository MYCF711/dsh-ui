// terminal_holds.go —— 官方 `window-holds`（`retainTabs` 全量保命面）在壳里的等价物。
//
// # 官方原件（两段，缺一不可）
//
// 订阅面（返回 null 的组件都没有，就是一个 cordis effect）：
//
//	ctx.effect(() => {
//	  const sync = () => ctx.webTerminals.retainTabs(
//	    ctx.sidebarRight.openTabs.getSnapshot().filter((tab) => tab.kind === "terminal"));
//	  const unsubscribe = ctx.sidebarRight.openTabs.subscribe(sync);
//	  sync();
//	  return () => { unsubscribe(); ctx.webTerminals.retainTabs([]); };
//	}, "ui-sidebar-terminal.window-holds");
//	（OFF:dsh-client-ui-sidebar-terminal/src/client/index.ts:34-39，
//	  实物复核：…/tmp/official/dsh-client-ui-sidebar-terminal/client.js:363-374）
//
// 对账面（控制器侧）：
//
//	retainTabs(tabs) { this.openTabs = tabs; this.reconcileHolds(); }
//	reconcileHolds() {
//	  wanted = Map<sessionId, Set<hostTerminalId>>
//	  for tab of openTabs:
//	    id = bindings.get(tab.sessionId, tab.contentId)       // 没有宿主身份 -> 跳过
//	    if (id === undefined || closed.has(id)) continue
//	    wanted[tab.sessionId].add(id)
//	    view = views.get(tab.sessionId)?.get(tab.tabId)
//	    if (view === undefined || view.state.info !== undefined)
//	      if (!holds.get(...)?.has(id)) this.hold(...)       // 开一条 retain 寿命流
//	  for (sessionId, holds) of this.holds:                  // ★ 释放就是这一次 diff
//	    for (id, hold) of holds: if (!wanted[sessionId].has(id)) { holds.delete(id); release(hold); }
//	}
//	hold(sessionId, id) = gateway.$stream({ open: (signal) => remote.retain(sessionId, id, signal) })
//	release(hold)       = hold.dispose()                      // 关流 = 释放保留窗口
//	（OFF:dsh-api-terminal-controller/src/client/index.ts:1076-1127 的 retainTabs/
//	  reconcileHolds/hold/release，实物复核 lib/client.js 同名段）
//
// 三件事因此是**官方的**，不是本仓的发挥：
//
//  1. `retainTabs` 吃的是**全量成员表**（`openTabs`，跨会话，含没有被展示的
//     dormant Session），不是"当前会话 + 当前 attach"；
//  2. 视图的 retain 与窗口的 hold 是**同一份**（`view(..., (signal) =>
//     this.hold(sessionId, id).ready(signal))`，OFF:…/client/index.ts:1007-1010），
//     所以同一条终端只有一条 retain 流，attach 等它 ack；
//  3. 释放语义是**全量对账的差集**：每次成员表变化都把"不在 wanted 里"的 hold
//     dispose 掉；effect 拆除时 `retainTabs([])` 全部释放。本仓没有 React 组件
//     卸载，所以拆除点落在 `main()` 的 `app.RunCanvas` 之后。
package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// terminalHold 是官方 `TerminalWindowHold`（OFF:dsh-api-terminal-controller
// /src/client/index.ts 的 retention 段）在壳里的持有凭证：一条 retain 寿命流。
//
// `TerminalStream` 的 retain 流是**寿命流**——宿主回一帧 `{type:'retained'}`
// 之后保持打开直到取消（DSH-GO `cmd/dshd/serve_terminal_api.go:815-860`），
// 所以 hold 的寿命就是保留窗口的寿命。
type terminalHold struct {
	// ctx 是这条 hold 的寿命；cancel 关掉这条流 = 释放保留窗口（官方 `hold.dispose()`）。
	ctx    context.Context
	cancel context.CancelFunc
	// acked 在第一帧 `retained` 之后关闭：attach 必须等它（官方 `hold.ready()`）。
	acked chan struct{}
	// done 在持有 goroutine 返回后关闭；`failed`/`err` 在它之前写好，读端
	// 只在这条通道关闭后读（channel close 提供 happens-before）。
	done chan struct{}
	// failed/err 是这条 hold 起不来时的终态（官方 `hold.failed`）：视图那条路
	// 见到它会重建（官方 `if (hold === void 0 || hold.failed)`），全量对账**不**重建
	// （官方 reconcileHolds 只判"有没有"，不判失败）。
	failed bool
	err    error
}

func terminalHoldKey(sessionID, id string) string { return sessionID + "\x00" + id }

// terminalHoldTarget 是 wanted 集合里一项：会话 + 宿主终端 id。
type terminalHoldTarget struct {
	sessionID string
	id        string
}

// terminalHoldTab 是全量成员表里的一条 terminal tab。
//
// restored=true 表示它来自**持久化布局**（`dsh.sidebar-right.v1.<session>`）——
// 也就是官方 `openTabs` 里"dormant saved Session 的 tab"。官方靠持久化在
// localStorage 的 `dsh.terminal.binding.v1.<session>.<contentId>` 解出它的宿主身份
// （OFF:dsh-api-terminal-controller/src/client/index.js 的 `TerminalBindings`）；
// 本仓的持久身份就是"occurrence id 即终端 id"的既有约定（`terminalHostID`），
// 所以恢复出来的 tab 自带身份，不需要另一张表。
type terminalHoldTab struct {
	sessionID string
	tabID     string
	kind      string
	restored  bool
}

// terminalHoldMembersLocked 是官方 `openTabs` 在壳里的投影：**本窗口认识的**
// 全部右栏 occurrence。
//
// 两个来源（顺序即优先级）：
//
//  1. `rightTabSurfaces`：本窗口已经装载过的会话布局（用户这一轮真看过/改过的）；
//  2. `uiConfig` 里 `dsh.sidebar-right.v1.<session>` 的持久化布局：**没被展示过的**
//     会话。官方 `openTabs` 从窗口启动就含这些 dormant Session 的 tab
//     （`retainTabs` 的注释原话："Reconcile this window's open terminal
//     occurrences, including dormant saved Sessions"）。
//
// 已装载的会话以内存布局为准，不再回头读持久化：内存里那份才含用户刚改的增删。
//
// 调用方必须持有 m.mu（`uiConfig` 与 `rightTab*` 都在这把锁下读）。
func (m *model) terminalHoldMembersLocked() []terminalHoldTab {
	var members []terminalHoldTab
	for sessionID, surface := range m.rightTabSurfaces {
		if surface == nil {
			continue
		}
		for _, tab := range surface.Tabs {
			members = append(members, terminalHoldTab{sessionID: sessionID, tabID: tab.ID, kind: tab.Kind})
		}
	}
	prefix := rightSidebarPersistence + "."
	for key, raw := range uiConfig {
		sessionID, ok := strings.CutPrefix(key, prefix)
		if !ok || sessionID == "" || m.rightTabLoaded[sessionID] {
			continue
		}
		// 坏数据只作废这一条成员（不刷状态栏）：这里每会话每帧都可能被读。
		surface, err := decodeRightTabSurface(sessionID, raw)
		if err != nil || surface == nil {
			continue
		}
		for _, tab := range surface.Tabs {
			members = append(members, terminalHoldTab{sessionID: sessionID, tabID: tab.ID, kind: tab.Kind, restored: true})
		}
	}
	return members
}

// terminalHoldWantedLocked 算出该持有 retain 的 (会话, 宿主终端 id) 全量集合。
//
// 判据逐条照官方 `reconcileHolds`（见文件头注释）：
//
//	视图存在但还没有 info  → 跳过（视图那条 attach 前的 retain 正在负责它）
//	视图存在且已有 info    → 持有（这条终端在本窗口真建/真接上了）
//	没有视图但带 terminalId → 持有（接管/Agent/恢复型 tab，身份是显式给的）
//	没有视图且来自持久化布局 → 持有（dormant Session 的 tab，身份=occurrence id）
//	上述都不成立          → 跳过（从没打开过的 seed tab：宿主上根本还没有这条终端，
//	                        先 hold 只会拿到一条 missingTerminal）
func (m *model) terminalHoldWantedLocked() map[string]terminalHoldTarget {
	wanted := map[string]terminalHoldTarget{}
	for _, tab := range m.terminalHoldMembersLocked() {
		if tab.kind != rightTabKindTerminal || tab.sessionID == "" || tab.tabID == "" {
			continue
		}
		key := userTerminalKey(tab.sessionID, tab.tabID)
		view := m.terminalViews[key]
		if view != nil && view.public.Info == nil {
			continue
		}
		id := tab.tabID
		explicit := false
		if params, ok := m.rightTabParams[rightTabParamsKey(tab.sessionID, tab.tabID)]; ok && params.TerminalID != "" {
			id = params.TerminalID
			explicit = true
		}
		if view == nil && !explicit && !tab.restored {
			continue
		}
		wanted[terminalHoldKey(tab.sessionID, id)] = terminalHoldTarget{sessionID: tab.sessionID, id: id}
	}
	return wanted
}

// syncTerminalHolds 是官方 `retainTabs(...)` + `reconcileHolds()` 的合体：
// 全量成员表进，差集出——缺的补 hold，多的**立刻释放**。
//
// 释放语义（最容易漏的那条）就在这里：tab 一关，它从成员表里消失，
// 下一次对账把它对应的 hold `cancel()` 掉，寿命流关闭；tab 若又开回来，
// 下次对账再起一条。不存在"关掉还留着 retain"的泄漏。
//
// 触发点与官方 `openTabs.subscribe(sync)` 同构：任何改变成员表的动作之后调一次
// （开 tab / 关 tab / Agent 自动开关 / 会话布局首次装载 / 启动后第一次画）。
// **不**每帧调：失败过的 hold 在官方也是等视图那条路重建，不在 reconcile 里重试，
// 每帧调会把一次失败放大成持续请求。
func (m *model) syncTerminalHolds() {
	if m.cli == nil {
		// 没有 Transport 客户端就没有终端控制面（旧壳 / 裸 model 的单测）：
		// 不起流，也不在内存里伪报"已保留"。
		return
	}
	m.mu.Lock()
	wanted := m.terminalHoldWantedLocked()
	var releases []context.CancelFunc
	for key, hold := range m.terminalHolds {
		if _, keep := wanted[key]; keep {
			continue
		}
		delete(m.terminalHolds, key)
		if hold.cancel != nil {
			releases = append(releases, hold.cancel)
		}
	}
	var starts []*terminalHold
	var targets []terminalHoldTarget
	if m.terminalHolds == nil {
		m.terminalHolds = map[string]*terminalHold{}
	}
	for key, target := range wanted {
		if m.terminalHolds[key] != nil {
			continue // 官方：`if (!holds.get(sessionId)?.has(id))` —— 已持有就不动（失败过的也留着）
		}
		hold := newTerminalHold()
		m.terminalHolds[key] = hold
		starts = append(starts, hold)
		targets = append(targets, target)
	}
	m.mu.Unlock()
	for _, cancel := range releases {
		cancel()
	}
	for i, hold := range starts {
		go m.runTerminalHold(hold.ctx, targets[i].sessionID, targets[i].id, hold)
	}
}

func newTerminalHold() *terminalHold {
	ctx, cancel := context.WithCancel(context.Background())
	return &terminalHold{ctx: ctx, cancel: cancel, acked: make(chan struct{}), done: make(chan struct{})}
}

// terminalHoldLocked 返回/新建一条 hold，给**视图**这条路用（调用方持有 m.mu）。
//
// 与全量对账的差别只在失败处理：视图等不到 hold 就接不上终端，所以见到
// `failed` 的旧条目要重建（官方 `hold()` 的 `if (hold === void 0 || hold.failed)`）。
func (m *model) terminalHoldLocked(sessionID, id string) *terminalHold {
	key := terminalHoldKey(sessionID, id)
	if m.terminalHolds == nil {
		m.terminalHolds = map[string]*terminalHold{}
	}
	if hold := m.terminalHolds[key]; hold != nil && !hold.failed {
		return hold
	}
	hold := newTerminalHold()
	m.terminalHolds[key] = hold
	go m.runTerminalHold(hold.ctx, sessionID, id, hold)
	return hold
}

// awaitTerminalHold 是官方 `this.hold(sessionId, id).ready(signal)`：确保这条
// (会话, 终端) 有一枚持有中的 retain，并等它的 `retained` ack。
// attach 必须在这之后才开始（官方 `connect()` 里 `await this.retain?.(signal)`
// 就在 `remote.attach` 之前）。
func (m *model) awaitTerminalHold(ctx context.Context, sessionID, id string) error {
	if m.cli == nil || sessionID == "" || id == "" {
		return errors.New("没有终端控制面，无法保留终端")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	hold := m.terminalHoldLocked(sessionID, id)
	m.mu.Unlock()
	select {
	case <-hold.acked:
		return nil
	case <-hold.done:
		if hold.failed && hold.err != nil {
			return hold.err
		}
		return errors.New("终端保留窗口在确认前就结束了")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// runTerminalHold 跑一条 retain 寿命流：首帧 retained -> 关 acked -> 一直持有到
// cancel（释放）为止。宿主那边取消即释放保留窗口（DSH-GO
// `serve_terminal_api.go:815-835`），所以这里 `defer stream.Close()` 就是官方的
// `hold.dispose()`。
func (m *model) runTerminalHold(ctx context.Context, sessionID, id string, hold *terminalHold) {
	defer close(hold.done)
	stream, err := m.cli.TerminalRetain(ctx, sessionID, id)
	if err != nil {
		m.failTerminalHold(sessionID, id, hold, err, false)
		return
	}
	defer stream.Close()
	if err := stream.NextRetained(); err != nil {
		m.failTerminalHold(sessionID, id, hold, err, false)
		return
	}
	close(hold.acked)
	// 寿命流只会**断**（宿主重启、断网），不会主动推第二帧。断了必须让下一次
	// 全量对账把它重开——否则界面上 tab 还开着、宿主那头已经不保活了。
	// （官方那条 hold 由 gateway 的 `$stream`「reconnecting stream owner」自动重连，
	// 见 retention 段；本仓没有那层，等价做法是在这里发现 EOF。）
	ended := make(chan struct{})
	go func() {
		defer close(ended)
		if _, nextErr := stream.Next(); nextErr != nil && ctx.Err() == nil {
			m.failTerminalHold(sessionID, id, hold, nextErr, true)
		}
	}()
	select {
	case <-ctx.Done():
	case <-ended:
	}
}

// failTerminalHold 记下终态。写 `failed/err` 与 `terminalHoldLocked` 的读同一把锁；
// `done` 在返回之后才关，读端不会看到半成品。
//
// revive 区分两种失败：
//
//	false：流**起来之前**就失败（终端不存在/宿主拒绝）。凭证留在表里，官方
//	       `reconcileHolds` 同样不重建失败凭证（只由视图那条路 `hold()` 重建）——
//	       否则每次开/关 tab 都会重试一条不存在的终端。
//	true ：流**起来之后**断了（宿主重启/断网）。官方由 gateway 自动重连，本仓
//	       把凭证摘掉，让下一次对账重开一条。
func (m *model) failTerminalHold(sessionID, id string, hold *terminalHold, err error, revive bool) {
	m.mu.Lock()
	hold.failed = true
	hold.err = fmt.Errorf("terminal.retain：%w", err)
	if revive {
		key := terminalHoldKey(sessionID, id)
		// 只摘自己那一枚：这条凭证可能已经被释放（表里换成新的或已删除），
		// 那是释放路径干的，不是失败。
		if m.terminalHolds[key] == hold {
			delete(m.terminalHolds, key)
		}
	}
	m.mu.Unlock()
}

// releaseAllTerminalHolds 是官方 effect 拆除时的 `retainTabs([])`：全部释放。
// 落点是 `main()` 的 `app.RunCanvas` 之后（本仓没有组件卸载这一层）。
func (m *model) releaseAllTerminalHolds() {
	m.mu.Lock()
	holds := m.terminalHolds
	m.terminalHolds = nil
	m.mu.Unlock()
	for _, hold := range holds {
		if hold.cancel != nil {
			hold.cancel()
		}
	}
}
