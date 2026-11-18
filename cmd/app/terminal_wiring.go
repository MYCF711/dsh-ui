// terminal_wiring.go —— 官方用户终端控制面（rc.2）在壳里的真实状态机。
//
// 每个右栏 occurrence 绑定一个调用方生成的 Host terminal id（本仓直接使用
// occurrence id，作用域仍是 Session）。创建/恢复后先 retain，再 attach；watch
// 每 Session 一条，只接受 whole-set roster。输入走宿主官方 HTTP 上行投影，
// typed 只来自人员手势。所有缓存都只由真响应更新，失败不伪造空名册或成功。
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/ansi"
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/vterm"
)

type terminalViewState struct {
	public     pluginapi.TerminalViewState
	started    bool
	generation uint64
	cancel     context.CancelFunc

	// emu 是这条终端的 VT 屏幕，snapshot 帧建屏、output 帧喂**原始字节**
	// （不是剥过序列的 Screen）。官方在这里做的是同一件事：snapshot 时
	// `xterm.reset(); xterm.resize(cols, rows)` 再 write，之后逐帧 write
	// （terminal.tsx:141-151）。
	emu *vterm.Emulator
	// grid 是 emu 的不可变快照，供插件逐格绘制。只在有新字节之后重建一次，
	// 因为绘制一帧只读一次快照，而读取循环可能一帧来好几批。
	grid      *vterm.Grid
	gridDirty bool
}

// 官方 `TerminalViewIssue` 的五档里本仓真会遇到的两种
// （OFF:dsh-api-terminal-controller/src/client/model.ts:19）。
// 它们是**分类**而不是文案：界面按官方 locale 渲染（`terminal.tsx:36-46`），
// 所以这里只放 key，不往 Error 里塞人类的句子，也不让界面把 key 当错误原文打印。
const (
	terminalIssueMissingTerminal = "missingTerminal"
	terminalIssueAttachmentEnded = "attachmentEnded"
)

// rightTabKindTerminal 是终端右栏 tab 的 kind，与官方
// `sidebarRightTabs.register({ kind: 'terminal' })`（OFF:dsh-client-ui-sidebar-terminal
// /src/client/index.ts:72-75）和本仓 `dsterminal.RightTabTerminal` 同值。
// 只有壳自动开 tab（Agent 终端接管）时用得到它——其余路径都是插件传 kind 进来。
const rightTabKindTerminal = "terminal"

type terminalWatchState struct {
	cancel context.CancelFunc
	failed bool
	err    string
	// agentControlled 是上一轮 roster 里 `origin=agent && control=agent` 的身份集合。
	// 官方用它判「受控终端离开名单 = Agent 或它的会话把它结束了」
	// （OFF:dsh-client-ui-sidebar-terminal/src/client/index.ts:116-126）。
	agentControlled map[string]bool
}

func userTerminalKey(sessionID, tabID string) string { return sessionID + "\x00" + tabID }

func (m *model) terminalEnsure(tabID string) {
	m.mu.Lock()
	sessionID := m.cur
	m.mu.Unlock()
	if sessionID == "" || tabID == "" {
		return
	}
	m.ensureTerminalWatch(sessionID)
	m.ensureTerminalView(sessionID, tabID, false)
}

func (m *model) terminalRetry(tabID string) {
	m.mu.Lock()
	sessionID := m.cur
	m.mu.Unlock()
	if sessionID == "" || tabID == "" {
		return
	}
	m.ensureTerminalWatch(sessionID)
	m.ensureTerminalView(sessionID, tabID, true)
}

// ensureTerminalView 的 retry 会先取消旧 attach；generation 防止旧 goroutine
// 的终态覆盖新一轮。重复 Ensure 在 failed 状态不自动重连，避免错误时每帧刷请求。
func (m *model) ensureTerminalView(sessionID, tabID string, retry bool) {
	key := userTerminalKey(sessionID, tabID)
	m.mu.Lock()
	if m.terminalViews == nil {
		m.terminalViews = map[string]*terminalViewState{}
	}
	state := m.terminalViews[key]
	if state == nil {
		state = &terminalViewState{public: pluginapi.TerminalViewState{Phase: "idle"}}
		m.terminalViews[key] = state
	}
	if state.started && !retry {
		m.mu.Unlock()
		return
	}
	if state.cancel != nil {
		state.cancel()
	}
	state.generation++
	generation := state.generation
	ctx, cancel := context.WithCancel(context.Background())
	state.cancel = cancel
	state.started = true
	state.public.Phase = "loading"
	state.public.Error = ""
	// issue 是**这一轮**的分类：重开一轮时旧的 attachmentEnded/missingTerminal
	// 不能继续贴在新状态上，否则界面会把已经接上的终端画成空态。
	state.public.Issue = ""
	state.public.Writable = false
	// 上一轮的屏幕不能带进新一轮：官方重新 attach 时也是 `xterm.reset()`。
	state.emu = nil
	state.grid = nil
	state.gridDirty = false
	m.mu.Unlock()
	m.inv.Invalidate()
	go m.runTerminalView(ctx, sessionID, tabID, generation)
}

func (m *model) terminalSnapshot(tabID string) pluginapi.TerminalViewState {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.terminalViews[userTerminalKey(m.cur, tabID)]
	if state == nil {
		return pluginapi.TerminalViewState{Phase: "idle"}
	}
	out := state.public
	out.Shells = append([]apiclient.TerminalShell(nil), state.public.Shells...)
	if state.emu != nil {
		if state.grid == nil || state.gridDirty {
			state.grid = state.emu.Grid()
			state.gridDirty = false
		}
		out.Grid = state.grid
	}
	if state.public.Info != nil {
		info := *state.public.Info
		out.Info = &info
	}
	if state.public.Environment != nil {
		env := *state.public.Environment
		out.Environment = &env
	}
	return out
}

func (m *model) terminalWriteOfficial(tabID, data string, typed bool) {
	m.mu.Lock()
	sessionID := m.cur
	state := m.terminalViews[userTerminalKey(sessionID, tabID)]
	var id string
	var writable bool
	var maxBytes int
	if state != nil && state.public.Info != nil {
		id = state.public.Info.ID
		writable = state.public.Writable && state.public.Info.State == "running"
		if state.public.Environment != nil {
			maxBytes = state.public.Environment.MaxInputBytes
		}
	}
	m.mu.Unlock()
	if sessionID == "" || id == "" || !writable {
		m.setStatus("终端当前不可写")
		return
	}
	if data == "" {
		return
	}
	if maxBytes > 0 && len(data) > maxBytes {
		m.setStatus(fmt.Sprintf("终端输入超过上限 %d 字节", maxBytes))
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.cli.TerminalWrite(ctx, sessionID, id, data, typed); err != nil {
			m.setStatus("写终端失败: " + err.Error())
		}
	}()
}

func (m *model) terminalRenameOfficial(tabID, title string) error {
	m.mu.Lock()
	sessionID := m.cur
	state := m.terminalViews[userTerminalKey(sessionID, tabID)]
	var id string
	if state != nil && state.public.Info != nil {
		id = state.public.Info.ID
	}
	m.mu.Unlock()
	if sessionID == "" || id == "" {
		return errors.New("终端尚未创建，不能重命名")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.cli.TerminalRename(ctx, sessionID, id, title); err != nil {
		return err
	}
	m.mu.Lock()
	if current := m.terminalViews[userTerminalKey(sessionID, tabID)]; current != nil && current.public.Info != nil {
		info := *current.public.Info
		info.Title = strings.TrimSpace(title)
		current.public.Info = &info
	}
	m.mu.Unlock()
	m.inv.Invalidate()
	return nil
}

func (m *model) terminalCloseOfficial(tabID string) error {
	m.mu.Lock()
	sessionID := m.cur
	key := userTerminalKey(sessionID, tabID)
	state := m.terminalViews[key]
	id := tabID
	if state != nil && state.public.Info != nil {
		id = state.public.Info.ID
	}
	m.mu.Unlock()
	if sessionID == "" || id == "" {
		return errors.New("没有可关闭的终端")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// 即使 create 的响应丢失也按确定身份 close，与官方 model.close 一致。
	if err := m.cli.TerminalClose(ctx, sessionID, id); err != nil {
		return err
	}
	m.mu.Lock()
	if current := m.terminalViews[key]; current != nil {
		if current.cancel != nil {
			current.cancel()
		}
		delete(m.terminalViews, key)
	}
	m.mu.Unlock()
	m.inv.Invalidate()
	return nil
}

// terminalHostID 返回这条 occurrence 在宿主上的**真实终端身份**。
//
//	官方 `ClientTerminals.view(sessionId, key, contentId, terminalId, shellPath)`：
//	  saved  = terminalId ?? bindings.get(sessionId, contentId)
//	  id     = saved ?? randomUUID()
//	  create = saved === undefined
//	（OFF:dsh-api-terminal-controller/src/client/index.ts:104-119）
//
// 也就是说：**带 `terminalId` 打开的 tab 是接管一条已经在跑的终端**（Agent 开的
// 那种，见 `AgentTerminalOpener`/`openAgentTerminals`，OFF:dsh-client-ui-sidebar-terminal
// /src/client/index.ts:103-112），必须按那个身份 attach，而不是照 occurrence id 新开一条。
// 没带参数时 occurrence id 就是终端身份（本仓的既有做法，等价于官方新铸一个 UUID）。
func (m *model) terminalHostID(sessionID, tabID string) string {
	if id := m.rightTabParamsFor(sessionID, tabID).TerminalID; id != "" {
		return id
	}
	return tabID
}

func (m *model) runTerminalView(ctx context.Context, sessionID, tabID string, generation uint64) {
	fail := func(err error) {
		if err == nil || ctx.Err() != nil {
			return
		}
		m.mu.Lock()
		if state := m.terminalViews[userTerminalKey(sessionID, tabID)]; state != nil && state.generation == generation {
			state.public.Phase = "failed"
			state.public.Error = err.Error()
			state.public.Writable = false
		}
		m.mu.Unlock()
		m.inv.Invalidate()
	}
	hostID := m.terminalHostID(sessionID, tabID)
	adopted := hostID != tabID

	env, err := m.terminalEnvironmentCall(ctx, sessionID)
	if err != nil {
		fail(err)
		return
	}
	shells, err := m.terminalShellsCall(ctx, sessionID)
	if err != nil {
		fail(err)
		return
	}
	roster, err := m.terminalListCall(ctx, sessionID)
	if err != nil {
		fail(err)
		return
	}
	var info *apiclient.TerminalInfo
	for i := range roster {
		if roster[i].ID == hostID {
			copy := roster[i]
			info = &copy
			break
		}
	}
	if info == nil {
		if adopted {
			// 官方语义：带 `terminalId` 打开的 tab 是**接管**一条已经在宿主上跑着的
			// 终端。那条终端不在了，视图进入终态（issue=missingTerminal，界面上是
			// 官方那块空态 + 「新建终端」），**绝不**在这里照着 occurrence id 新开
			// 一条：那会变成"用户终端冒充 agent 终端"，徽标与归属都成了假的。
			m.mu.Lock()
			if state := m.terminalViews[userTerminalKey(sessionID, tabID)]; state != nil && state.generation == generation {
				state.public.Phase = "failed"
				state.public.Issue = terminalIssueMissingTerminal
				state.public.Error = ""
				state.public.Writable = false
			}
			m.mu.Unlock()
			m.inv.Invalidate()
			return
		}
		cols, rows := 80, 24
		if env.MaxCols > 0 && cols > env.MaxCols {
			cols = env.MaxCols
		}
		if env.MaxRows > 0 && rows > env.MaxRows {
			rows = env.MaxRows
		}
		request := apiclient.TerminalCreateRequest{ID: hostID, Cols: cols, Rows: rows}
		// 官方 `openTab('terminal', { params: { shellPath } })`（face.ts:20-24）优先；
		// 没有就退回向导页记住的那个选择（`webTerminals.selectShell`），
		// 再没有才是第一枚可用的 shell（原行为）。
		preferred := m.rightTabParamsFor(sessionID, tabID).ShellPath
		if preferred == "" {
			m.mu.Lock()
			preferred = m.terminalShellPreferenceLocked(sessionID)
			m.mu.Unlock()
		}
		request.ShellPath = pickTerminalShell(shells, preferred)
		created, createErr := m.terminalCreateCall(ctx, sessionID, request)
		if createErr != nil {
			fail(createErr)
			return
		}
		info = &created
	}
	m.mu.Lock()
	state := m.terminalViews[userTerminalKey(sessionID, tabID)]
	if state == nil || state.generation != generation {
		m.mu.Unlock()
		return
	}
	state.public.Environment = &env
	state.public.Shells = shells
	state.public.Info = info
	state.public.Phase = "connecting"
	m.mu.Unlock()
	m.inv.Invalidate()

	retain, err := m.cli.TerminalRetain(ctx, sessionID, info.ID)
	if err != nil {
		fail(err)
		return
	}
	defer retain.Close()
	if err := retain.NextRetained(); err != nil {
		fail(err)
		return
	}
	m.mu.Lock()
	if state := m.terminalViews[userTerminalKey(sessionID, tabID)]; state != nil && state.generation == generation {
		state.public.Retained = true
	}
	m.mu.Unlock()

	attach, err := m.cli.TerminalAttach(ctx, sessionID, info.ID)
	if err != nil {
		fail(err)
		return
	}
	defer attach.Close()
	var sequence int64
	opened := false
	for {
		frame, nextErr := attach.Next()
		if nextErr != nil {
			if errors.Is(nextErr, io.EOF) {
				m.mu.Lock()
				if state := m.terminalViews[userTerminalKey(sessionID, tabID)]; state != nil && state.generation == generation && ctx.Err() == nil {
					state.public.Writable = false
					if state.public.Info != nil && state.public.Info.State == "exited" {
						state.public.Phase = "closed"
					} else {
						state.public.Phase = "disconnected"
						// 官方把这一类叫 issue 而不是自由文本
						// （`model.ts:19` 的 `attachmentEnded`），界面按官方 locale 渲染。
						state.public.Issue = terminalIssueAttachmentEnded
					}
				}
				m.mu.Unlock()
				m.inv.Invalidate()
				return
			}
			fail(nextErr)
			return
		}
		m.mu.Lock()
		state := m.terminalViews[userTerminalKey(sessionID, tabID)]
		if state == nil || state.generation != generation {
			m.mu.Unlock()
			return
		}
		switch frame.Type {
		case "snapshot":
			if opened {
				m.mu.Unlock()
				fail(errors.New("Unexpected terminal screen snapshot"))
				return
			}
			opened = true
			sequence = frame.Sequence
			state.public.Screen = ansi.Strip(frame.Screen)
			// 建 VT 屏幕并喂**原始**快照：官方就是 `xterm.reset();
			// xterm.resize(info.cols, info.rows); xterm.write(snapshot.screen)`
			// （terminal.tsx:146-150）。Screen 那条剥序列投影照旧保留，
			// 给没有网格的读者（agent 侧文本语义）用。
			state.emu = vterm.New(frame.Info.Cols, frame.Info.Rows)
			if _, err := state.emu.Write([]byte(frame.Screen)); err != nil {
				m.mu.Unlock()
				fail(err)
				return
			}
			state.gridDirty = true
			state.public.Info = terminalInfoPtr(frame.Info)
			state.public.Writable = frame.Writable && frame.Info.State == "running"
			state.public.Phase = "connected"
			state.public.Error = ""
		case "output":
			if !opened || frame.Sequence != sequence+1 {
				m.mu.Unlock()
				fail(fmt.Errorf("Terminal output sequence has a gap（上一帧 %d，收到 %d）", sequence, frame.Sequence))
				return
			}
			sequence = frame.Sequence
			state.public.Screen = boundedTerminalScreen(state.public.Screen+ansi.Strip(frame.Data), env.MaxBufferedBytes)
			if state.emu != nil {
				// 喂原始字节：颜色/光标定位只在这条路上存在。
				if _, err := state.emu.Write([]byte(frame.Data)); err != nil {
					m.mu.Unlock()
					fail(err)
					return
				}
				state.gridDirty = true
			}
		case "state":
			state.public.Info = terminalInfoPtr(frame.Info)
			state.public.Writable = frame.Writable && frame.Info.State == "running"
			state.public.Phase = "connected"
		case "ack":
			if frame.Refusal != "" {
				state.public.Writable = false
				state.public.Error = frame.Refusal
			}
		default:
			m.mu.Unlock()
			fail(fmt.Errorf("未知终端帧 type=%q", frame.Type))
			return
		}
		m.mu.Unlock()
		m.inv.Invalidate()
	}
}

func terminalInfoPtr(info apiclient.TerminalInfo) *apiclient.TerminalInfo {
	copy := info
	return &copy
}

func boundedTerminalScreen(screen string, maxBytes int) string {
	if maxBytes <= 0 || len(screen) <= maxBytes {
		return screen
	}
	screen = screen[len(screen)-maxBytes:]
	for screen != "" && !utf8.ValidString(screen) {
		screen = screen[1:]
	}
	return screen
}

func (m *model) terminalEnvironmentCall(ctx context.Context, sessionID string) (apiclient.TerminalEnvironment, error) {
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return m.cli.TerminalEnvironment(callCtx, sessionID)
}

func (m *model) terminalShellsCall(ctx context.Context, sessionID string) ([]apiclient.TerminalShell, error) {
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return m.cli.TerminalShells(callCtx, sessionID)
}

func (m *model) terminalListCall(ctx context.Context, sessionID string) ([]apiclient.TerminalInfo, error) {
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return m.cli.TerminalList(callCtx, sessionID)
}

func (m *model) terminalCreateCall(ctx context.Context, sessionID string, request apiclient.TerminalCreateRequest) (apiclient.TerminalInfo, error) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return m.cli.TerminalCreate(callCtx, sessionID, request)
}

func (m *model) ensureTerminalWatch(sessionID string) {
	// 没有 Transport 客户端就没有终端控制面（旧壳 / 单测里的裸 model）：
	// 这时**不开** roster 流，而不是让 goroutine 拿 nil 客户端崩。
	m.mu.Lock()
	if m.cli == nil || sessionID == "" {
		m.mu.Unlock()
		return
	}
	if m.terminalWatchers == nil {
		m.terminalWatchers = map[string]*terminalWatchState{}
	}
	if existing := m.terminalWatchers[sessionID]; existing != nil {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.terminalWatchers[sessionID] = &terminalWatchState{cancel: cancel}
	m.mu.Unlock()
	go m.runTerminalWatch(ctx, sessionID)
}

func (m *model) runTerminalWatch(ctx context.Context, sessionID string) {
	stream, err := m.cli.TerminalWatch(ctx, sessionID)
	if err != nil {
		m.failTerminalWatch(sessionID, err)
		return
	}
	defer stream.Close()
	for {
		roster, nextErr := stream.NextRoster()
		if nextErr != nil {
			if ctx.Err() == nil {
				m.failTerminalWatch(sessionID, nextErr)
			}
			return
		}
		m.mu.Lock()
		byID := map[string]apiclient.TerminalInfo{}
		for _, info := range roster {
			byID[info.ID] = info
		}
		var cancelMissing []context.CancelFunc
		for key, state := range m.terminalViews {
			if !strings.HasPrefix(key, sessionID+"\x00") || state.public.Info == nil {
				continue
			}
			if info, ok := byID[state.public.Info.ID]; ok {
				copy := info
				state.public.Info = &copy
			} else {
				state.public.Phase = "closed"
				state.public.Issue = terminalIssueMissingTerminal
				state.public.Writable = false
				if state.cancel != nil {
					cancelMissing = append(cancelMissing, state.cancel)
				}
			}
		}
		m.mu.Unlock()
		for _, cancel := range cancelMissing {
			cancel()
		}
		// 官方 `AgentTerminalOpener` 的两支（index.ts:103-126）：Agent 新开的终端
		// 自动变成一枚右栏 tab，Agent 结束掉的受控终端把它的 tab 关掉。
		m.syncAgentTerminals(sessionID, roster)
		m.inv.Invalidate()
	}
}

// syncAgentTerminals 是官方 `openAgentTerminals` + `closeEndedAgentTerminals`
// （OFF:dsh-client-ui-sidebar-terminal/src/client/index.ts:103-126）在壳里的等价物。
//
// 为什么是壳的活：官方那两段由一个**只返回 null 的组件**（AgentTerminalOpener，
// `conversation.session.header.actions`）在会话头显示期间挂着，把 `webTerminals.watch`
// 的 roster 翻成 sidebar 动作。本仓没有 React 组件这一层，roster 本来就在壳里
// （runTerminalWatch），所以翻在这里——不多加一个空画板插件。
//
// # 判据逐条照抄官方
//
//	开 tab：`info.origin === 'agent' && info.state === 'running' && !isRepresented(info.id)`
//	        → openTab('terminal', { params: { terminalId: info.id } })
//	关 tab：受控集合（`origin==='agent' && control==='agent'`）里**上一轮有、本轮没了**
//	        的身份 → 关掉绑定到它的 tab（那是 Agent 或它的会话结束的，不是用户关的）
//
// `isRepresented` 在官方是"已经有一个 view 指着这条终端"。本仓的等价判据是
// "已经有一枚 occurrence 绑定了它"——params 里的 terminalId，或视图里已经在用的
// Info.ID（老 tab 在 params 还没建立前就绑上的情况）。少了这一半，每一帧 roster
// 都会再开一枚同名 tab。
func (m *model) syncAgentTerminals(sessionID string, roster []apiclient.TerminalInfo) {
	if sessionID == "" {
		return
	}
	m.mu.Lock()
	watch := m.terminalWatchers[sessionID]
	previous := map[string]bool{}
	if watch != nil {
		previous = watch.agentControlled
	}
	controlled := map[string]bool{}
	for _, info := range roster {
		if info.Origin == "agent" && info.Control == "agent" {
			controlled[info.ID] = true
		}
	}
	if watch != nil {
		watch.agentControlled = controlled
	}
	var closeTabs []string
	for id := range previous {
		if controlled[id] {
			continue
		}
		if tabID := m.rightTabBoundToLocked(sessionID, id); tabID != "" {
			closeTabs = append(closeTabs, tabID)
		}
	}
	current := sessionID == m.cur
	// 右栏布局没到之前不铸 occurrence：那时 openRightTabAt 会拒绝并往状态栏写一句，
	// 而 roster 每几秒就来一帧——会刷屏。等布局到了下一帧自动接上。
	ready := current && (m.rightTabLoaded[sessionID] || uiConfig != nil)
	var openFor []string
	if ready {
		for _, info := range roster {
			if info.Origin != "agent" || info.State != "running" {
				continue
			}
			if m.rightTabBoundToLocked(sessionID, info.ID) != "" {
				continue
			}
			openFor = append(openFor, info.ID)
		}
	}
	m.mu.Unlock()
	// 关在开之前：受控终端已经不在宿主上了，这时不能走「先请宿主关、成功再删 tab」
	// 那条路（宿主会回 not-found，tab 就永远留着）。官方这里也是直接 closeIn。
	for _, tabID := range closeTabs {
		m.forgetRightTab(sessionID, tabID)
	}
	for _, id := range openFor {
		// 官方每次 openTab 都新铸一枚 occurrence（terminal 是 multiple:true）。
		m.openRightTabParams(rightTabKindTerminal, pluginapi.RightTabParams{TerminalID: id})
	}
}

// rightTabBoundToLocked 返回绑定到某条宿主终端的 occurrence id；没有则空串。
// 调用方必须持有 m.mu。
func (m *model) rightTabBoundToLocked(sessionID, hostID string) string {
	if sessionID == "" || hostID == "" {
		return ""
	}
	prefix := sessionID + "\x00"
	for key, params := range m.rightTabParams {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if params.TerminalID == hostID {
			return strings.TrimPrefix(key, prefix)
		}
	}
	for key, state := range m.terminalViews {
		if !strings.HasPrefix(key, prefix) || state.public.Info == nil {
			continue
		}
		if state.public.Info.ID == hostID {
			return strings.TrimPrefix(key, prefix)
		}
	}
	return ""
}

func (m *model) failTerminalWatch(sessionID string, err error) {
	m.mu.Lock()
	if watch := m.terminalWatchers[sessionID]; watch != nil {
		watch.failed = true
		watch.err = err.Error()
	}
	m.mu.Unlock()
}

// releaseTerminalWatchesExcept 停掉除 keep 之外、且**没有任何终端视图**的会话 roster 流。
//
// 官方 `AgentTerminalOpener` 只挂在**被展示**的会话头上，切走即释放那条 watch
// （OFF:dsh-client-ui-sidebar-terminal/src/client/index.ts:128-143）。本仓的壳没有
// 组件卸载这一层，所以在这里显式释放；但**有 attach 在跑的会话不收**——那条流还在
// 给视图做 roster 对账（"终端被别处关掉了"就是从这里发现的）。
func (m *model) releaseTerminalWatchesExcept(keep string) {
	m.mu.Lock()
	var releases []context.CancelFunc
	for sessionID, watch := range m.terminalWatchers {
		if sessionID == keep || watch == nil {
			continue
		}
		active := false
		prefix := sessionID + "\x00"
		for key := range m.terminalViews {
			if strings.HasPrefix(key, prefix) {
				active = true
				break
			}
		}
		if active {
			continue
		}
		if watch.cancel != nil {
			releases = append(releases, watch.cancel)
		}
		delete(m.terminalWatchers, sessionID)
	}
	m.mu.Unlock()
	for _, cancel := range releases {
		cancel()
	}
}
