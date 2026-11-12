// store.go —— 本包的运行时（官方 mount.ts 里那几个 store 的 Go 等价物）。
//
// 三块状态各自独立，对应官方的三个 store/hooks：
//
//	projection + session flags   ← Session projection `agentTeam` / `subagent` / `modelSelection`
//	disableRequest               ← `createSnapshotStore<TeamDisableRequest|null>(null)`（mount.ts:41）
//	command node                 ← `PropsRuntime<'conversation.chat.commandview'>['node']`
//
// # 两条"不做"（官方明确，结构树 §8）
//
//  1. **不做乐观更新**：`Select` 期间只是 `pending=true`，`enabled` 仍是宿主值；
//     成功才把本地的 `enabled` 改掉（宿主随后也会推同值投影）。
//  2. **取消只在 sessionId 匹配时清空**（mount.ts:51-53）：防止跨会话误清别人的确认框。
package dsagentteam

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Remote 是唯一的远端写动作：官方 `remote.agentTeams.setEnabled(sessionId, enabled)`。
//
// 返回值语义照抄官方适配代码 `selectTeamMode`：`ok ? null : error.message`。
// 所以这里 `error` 就是"要显示给用户的那句失败原因"，不是 HTTP 细节。
// `*apiclient.Client` 直接满足本接口。
type Remote interface {
	SetAgentTeamEnabled(ctx context.Context, sessionID string, enabled bool) error
}

// DisableRequest 是官方 `TeamDisableRequest`（只有 sessionId，TeamDisableConfirmation.tsx:11-13）。
type DisableRequest struct {
	SessionID string
}

// ConfirmKind 是官方确认框状态机的三个状态。
type ConfirmKind string

const (
	ConfirmReady  ConfirmKind = "ready"
	ConfirmSaving ConfirmKind = "saving"
	ConfirmFailed ConfirmKind = "failed"
)

// ConfirmState 是官方 `{kind:'ready'} | {kind:'saving'} | {kind:'failed',message}`。
type ConfirmState struct {
	Kind    ConfirmKind
	Message string
}

// Store 是本包的运行时。所有方法都可并发调；内部只用一把锁。
type Store struct {
	mu sync.Mutex

	remote Remote
	view   View

	// pending = 官方 useState(false) 的 pending（开关请求在途）。
	pending bool
	// failure = 官方 reportFailure 收到的 message（画在开关下方 / 状态栏）。
	failure string

	// expandedTaskID 是官方 expandedTaskId：**单值**，开一个就关上一个（TeamBoard.tsx:85）。
	expandedTaskID string
	// infoOpen 是 Tooltip(openOnClick) 的展开态（信息按钮点开）。
	infoOpen bool

	disableRequest *DisableRequest
	confirm        ConfirmState

	command *CommandNode
	// commandExpanded 是官方 TeamCommandRow 内部的 useState(false)。
	commandExpanded bool

	log *slog.Logger
}

// NewStore 造一个运行时。remote 可以为 nil（= 未接线：开关点了会原地报错，不静默）。
func NewStore(remote Remote) *Store {
	return &Store{
		remote:  remote,
		confirm: ConfirmState{Kind: ConfirmReady},
		log:     slog.Default(),
	}
}

// SetRemote 接上/摘掉远端实现。
func (s *Store) SetRemote(remote Remote) {
	s.mu.Lock()
	s.remote = remote
	s.mu.Unlock()
}

// SetView 用这一帧的投影/会话态覆盖运行时快照。
func (s *Store) SetView(v View) {
	s.mu.Lock()
	s.view = v
	// 任务没了就把展开态收掉（否则会记住一个不存在的 id）。
	if v.ProjectionKnown {
		found := false
		for _, task := range v.Projection.Tasks {
			if task.ID == s.expandedTaskID {
				found = true
				break
			}
		}
		if !found {
			s.expandedTaskID = ""
		}
	}
	s.mu.Unlock()
}

// View 返回当前快照（值拷贝；切片/指针字段只读使用）。
func (s *Store) View() View {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.view
}

// Pending 是开关请求在途。
func (s *Store) Pending() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pending
}

// Failure 是最近一次开关失败的文案（官方 mode.failed）。
func (s *Store) Failure() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failure
}

// ExpandedTaskID 是当前展开的任务（同时最多一个）。
func (s *Store) ExpandedTaskID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.expandedTaskID
}

// ToggleExpanded 展开/收起源任务；开一个就关上一个（官方 expandedTaskId 单值）。
func (s *Store) ToggleExpanded(taskID string) {
	s.mu.Lock()
	if s.expandedTaskID == taskID {
		s.expandedTaskID = ""
	} else {
		s.expandedTaskID = taskID
	}
	s.mu.Unlock()
}

// InfoOpen 是信息按钮的展开态。
func (s *Store) InfoOpen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.infoOpen
}

// ToggleInfo 切信息按钮的展开态。
func (s *Store) ToggleInfo() {
	s.mu.Lock()
	s.infoOpen = !s.infoOpen
	s.mu.Unlock()
}

// Select 是官方 `select(enabled)`（TeamModeToggle.tsx:45-56）。
//
// **关闭**且会话已经有模型请求（`modelSelection.lastUsed !== null`）时：
// 不直接关，改成"请求停用确认"（确认框因此必须挂 shell.overlay，不在菜单里）。
func (s *Store) Select(enabled bool) {
	s.mu.Lock()
	view := s.view
	s.mu.Unlock()

	if !enabled && view.ModelSelectionKnown && view.ModelLastUsed {
		s.RequestDisable(view.SessionID)
		return
	}
	s.applySelection(view.SessionID, enabled)
}

// applySelection 走真实的远端写：pending 期间开关禁用，成功才改本地值。
func (s *Store) applySelection(sessionID string, enabled bool) {
	s.mu.Lock()
	if s.pending {
		s.mu.Unlock()
		return
	}
	remote := s.remote
	s.pending = true
	s.failure = ""
	s.mu.Unlock()

	if remote == nil {
		// 没接线：**不假装成功**，也不把 enabled 改掉。
		s.mu.Lock()
		s.pending = false
		s.failure = formatMessage(zhModeFailed, "宿主没有 agentTeams.setEnabled（rc.2 Team 面未接线）")
		s.mu.Unlock()
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := remote.SetAgentTeamEnabled(ctx, sessionID, enabled)

		s.mu.Lock()
		defer s.mu.Unlock()
		s.pending = false
		if err != nil {
			// 官方：失败把 `t('mode.failed',{message})` 交给 reportFailure，本地值不变。
			s.failure = formatMessage(zhModeFailed, err.Error())
			if s.log != nil {
				s.log.Warn("agentTeams.setEnabled 失败", "session", sessionID, "enabled", enabled, "err", err)
			}
			return
		}
		// 成功：选择在宿主侧已生效（官方"选择在 await 持久化之前生效"），
		// 本地投影跟着改，免得下一帧又画回旧值。
		if s.view.ProjectionKnown {
			s.view.Projection.Enabled = enabled
		}
		s.failure = ""
	}()
}

// RequestDisable 弹停用确认（官方 `requestDisableConfirmation()`）。
func (s *Store) RequestDisable(sessionID string) {
	if sessionID == "" {
		return
	}
	s.mu.Lock()
	s.disableRequest = &DisableRequest{SessionID: sessionID}
	s.confirm = ConfirmState{Kind: ConfirmReady}
	s.mu.Unlock()
}

// CancelDisable 关掉确认框。**只在 sessionId 匹配时清空**（防跨会话误清）。
func (s *Store) CancelDisable(sessionID string) {
	s.mu.Lock()
	if s.disableRequest != nil && s.disableRequest.SessionID == sessionID {
		s.disableRequest = nil
		s.confirm = ConfirmState{Kind: ConfirmReady}
	}
	s.mu.Unlock()
}

// DisableRequest 返回待确认请求（nil = 没有，整块不渲染）。
func (s *Store) DisableRequest() *DisableRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.disableRequest
}

// Confirm 返回确认框状态。
func (s *Store) Confirm() ConfirmState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.confirm
}

// ConfirmDisable 是官方 `confirm()`：saving 期间两个按钮都禁用；
// 成功关框，失败停在 failed 且错误**一直可见到重试或取消**。
func (s *Store) ConfirmDisable() {
	s.mu.Lock()
	if s.disableRequest == nil || s.confirm.Kind == ConfirmSaving {
		s.mu.Unlock()
		return
	}
	if s.remote == nil {
		s.confirm = ConfirmState{
			Kind:    ConfirmFailed,
			Message: formatMessage(zhModeFailed, "宿主没有 agentTeams.setEnabled（rc.2 Team 面未接线）"),
		}
		s.mu.Unlock()
		return
	}
	sessionID := s.disableRequest.SessionID
	remote := s.remote
	s.confirm = ConfirmState{Kind: ConfirmSaving}
	s.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := remote.SetAgentTeamEnabled(ctx, sessionID, false)

		s.mu.Lock()
		if err != nil {
			s.confirm = ConfirmState{
				Kind:    ConfirmFailed,
				Message: formatMessage(zhModeFailed, err.Error()),
			}
			if s.log != nil {
				s.log.Warn("停用 Team 失败（确认框保留错误）", "session", sessionID, "err", err)
			}
			s.mu.Unlock()
			return
		}
		if s.view.ProjectionKnown {
			s.view.Projection.Enabled = false
		}
		s.disableRequest = nil
		s.confirm = ConfirmState{Kind: ConfirmReady}
		s.mu.Unlock()
	}()
}

// SetCommand 喂入当前命令节点（父级在画 commandview 时给）。
func (s *Store) SetCommand(node *CommandNode) {
	s.mu.Lock()
	s.command = node
	s.mu.Unlock()
}

// CommandExpanded 是可展开回执的展开态（官方 useState(false)）。
func (s *Store) CommandExpanded() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.commandExpanded
}

// ToggleCommandExpanded 切回执的展开态。
func (s *Store) ToggleCommandExpanded() {
	s.mu.Lock()
	s.commandExpanded = !s.commandExpanded
	s.mu.Unlock()
}

// Command 返回当前命令节点（nil = 没有，本槽不画）。
func (s *Store) Command() *CommandNode {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.command
}
