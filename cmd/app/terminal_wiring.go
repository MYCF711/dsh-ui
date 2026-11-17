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
)

type terminalViewState struct {
	public     pluginapi.TerminalViewState
	started    bool
	generation uint64
	cancel     context.CancelFunc
}

// 官方 `TerminalViewIssue` 的五档里本仓真会遇到的两种
// （OFF:dsh-api-terminal-controller/src/client/model.ts:19）。
// 它们是**分类**而不是文案：界面按官方 locale 渲染（`terminal.tsx:36-46`），
// 所以这里只放 key，不往 Error 里塞人类的句子，也不让界面把 key 当错误原文打印。
const (
	terminalIssueMissingTerminal = "missingTerminal"
	terminalIssueAttachmentEnded = "attachmentEnded"
)

type terminalWatchState struct {
	cancel context.CancelFunc
	failed bool
	err    string
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
		if roster[i].ID == tabID {
			copy := roster[i]
			info = &copy
			break
		}
	}
	if info == nil {
		cols, rows := 80, 24
		if env.MaxCols > 0 && cols > env.MaxCols {
			cols = env.MaxCols
		}
		if env.MaxRows > 0 && rows > env.MaxRows {
			rows = env.MaxRows
		}
		request := apiclient.TerminalCreateRequest{ID: tabID, Cols: cols, Rows: rows}
		for _, shell := range shells {
			if shell.Supported == nil || *shell.Supported {
				request.ShellPath = shell.Path
				break
			}
		}
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
	m.mu.Lock()
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
		m.inv.Invalidate()
	}
}

func (m *model) failTerminalWatch(sessionID string, err error) {
	m.mu.Lock()
	if watch := m.terminalWatchers[sessionID]; watch != nil {
		watch.failed = true
		watch.err = err.Error()
	}
	m.mu.Unlock()
}
