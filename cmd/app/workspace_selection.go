// workspace_selection.go —— WorkspacePicker 的 onPick(workspaceId) 选择语义。
//
// 官方不是“注册目录后刷新列表”：WorkspacePickFlow 等 createWorkspace 返回 WorkspaceView，
// 再把 workspaceId 交给 owner（WorkspacePicker.tsx:152-161）。ConversationContent 与
// WorkspaceBrowser 都经 UiWorkspaceService.openWorkspace/startSession 复用该工作区的
// 空白会话（navigation.ts:215-256），而不是随手打开一个非空历史会话。
//
// # 选择语义逐条照官方 `navigation.ts:235-256`（createInWorkspace）
//
//  1. 先找**可复用的空白成员会话**：blank + cwd === workspace.path + 是工作区成员
//     + 未归档（`navigation.ts:237-243`）。本仓坚持用 `session.load` 证实
//     `Total == 0` 再复用——列表里的 blank 是缓存提示，可能落后于日志。
//  2. 没有可复用的就发官方 `session.create { workspaceId }`（`navigation.ts:245-250`
//     的 `createNamed`），拿到**宿主返回的** sessionId（可能与请求的 sessionId 不同）。
//
// # 为什么建完还要 load 一次
//
// 官方契约（`types.ts:268-277`）在**已发布的宿主**上才生效：DSH-GO 的 `origin/main`
// 只解 `mode`（`HOST:internal/transport/server.go` 的 `handleSessionCreate`），
// 官方四字段要等 PR #63 合入。老宿主收到 `{workspaceId}` 会**静默忽略**它、
// 把会话建在进程 cwd —— 那是"收下 workspaceId 却建到别处"，比报错糟得多。
// 所以新建之后必须用 `session.load` 核对会话 cwd 真的等于工作区目录；
// 不等就如实报错（这不是猜测，是读回来的宿主真值）。
package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

const workspaceSelectionTimeout = 8 * time.Second

var errWorkspaceSessionCwdMismatch = errors.New(
	"宿主 session.create 没有按 workspaceId 建会话")

// adoptPickedWorkspace 是两个 directory-flow 后端共用的 onPick 入口。
// 先用宿主刚返回的真 WorkspaceView 更新 registry 缓存，再按 workspaceId 选择；
// 刷新只负责重核投影，不再冒充选择动作本身。
func (m *model) adoptPickedWorkspace(workspace apiclient.WorkspaceView) {
	if m == nil || workspace.WorkspaceID == "" {
		return
	}
	pluginWorkspace := pluginapi.Workspace{
		WorkspaceID: workspace.WorkspaceID,
		Path:        workspace.Path,
		Title:       workspace.Title,
		SessionIDs:  append([]string(nil), workspace.SessionIDs...),
		CreatedAt:   workspace.CreatedAt,
		UpdatedAt:   workspace.UpdatedAt,
	}
	m.mu.Lock()
	replaced := false
	for i := range m.workspaces {
		if m.workspaces[i].WorkspaceID == workspace.WorkspaceID {
			m.workspaces[i] = pluginWorkspace
			replaced = true
			break
		}
	}
	if !replaced {
		m.workspaces = append(m.workspaces, pluginWorkspace)
	}
	m.workspacesKnown = true
	m.mu.Unlock()
	m.inv.Invalidate()
	m.refreshList()
	m.selectWorkspace(workspace.WorkspaceID)
}

// selectWorkspace 是 Actions.SelectWorkspace。重复选择同一个 pending id 不重复连接；
// 新选择取代旧选择，旧请求不得在新选择之后落地（对应官方 navigation lifetime）。
func (m *model) selectWorkspace(workspaceID string) {
	if m == nil || m.cli == nil || workspaceID == "" {
		return
	}
	m.mu.Lock()
	if m.workspaceSelectionPending == workspaceID {
		m.mu.Unlock()
		return
	}
	m.workspaceSelectionSeq++
	seq := m.workspaceSelectionSeq
	m.workspaceSelectionPending = workspaceID
	archived := make(map[string]bool, len(m.archivedSessions))
	for id, yes := range m.archivedSessions {
		archived[id] = yes
	}
	m.mu.Unlock()
	m.inv.Invalidate()

	go func() {
		workspace, ok := m.workspaceSelectionTarget(workspaceID)
		if !ok {
			m.finishWorkspaceSelection(seq, workspaceID, fmt.Errorf("workspace registry 中没有工作区 %q", workspaceID))
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), workspaceSelectionTimeout)
		defer cancel()
		sessionID, reused, err := m.blankSessionInWorkspace(ctx, workspace, archived)
		if err != nil {
			m.finishWorkspaceSelection(seq, workspaceID, err)
			return
		}
		if !reused {
			// 官方在没有可复用空白时**就发一次 create**（navigation.ts:245-250），
			// 而不是把"这个工作区还没会话"当成失败。
			sessionID, err = m.createSessionInWorkspace(ctx, workspace)
			if err != nil {
				m.finishWorkspaceSelection(seq, workspaceID, err)
				return
			}
		}
		m.mu.Lock()
		if seq != m.workspaceSelectionSeq {
			m.mu.Unlock()
			return
		}
		m.workspaceSelectionPending = ""
		m.mu.Unlock()
		m.openSession(sessionID)
	}()
}

func (m *model) workspaceSelectionTarget(workspaceID string) (pluginapi.Workspace, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, workspace := range m.workspaces {
		if workspace.WorkspaceID == workspaceID {
			workspace.SessionIDs = append([]string(nil), workspace.SessionIDs...)
			return workspace, true
		}
	}
	return pluginapi.Workspace{}, false
}

func (m *model) finishWorkspaceSelection(seq uint64, workspaceID string, err error) {
	m.mu.Lock()
	if seq != m.workspaceSelectionSeq {
		m.mu.Unlock()
		return
	}
	m.workspaceSelectionPending = ""
	m.mu.Unlock()
	m.setStatus("新建会话失败：" + err.Error())
}

// blankSessionInWorkspace 只返回经宿主 load 证实为空白的成员会话。
// archived 与会话 cwd 都必须匹配；任意一条读失败都如实返回，不跳过错误后猜一个。
//
// 没有可复用空白会话是**正常情况**（第二个返回值 false），不是错误：
// 调用方接着走官方 create（navigation.ts:245-250）。
func (m *model) blankSessionInWorkspace(ctx context.Context, workspace pluginapi.Workspace, archived map[string]bool) (string, bool, error) {
	for _, sessionID := range workspace.SessionIDs {
		if archived[sessionID] {
			continue
		}
		result, err := m.cli.Load(ctx, sessionID, 1)
		if err != nil {
			return "", false, fmt.Errorf("读取工作区空白会话 %q 失败: %w", sessionID, err)
		}
		if result.Total == 0 && sameWorkspaceSelectionPath(result.Cwd, workspace.Path) {
			return sessionID, true, nil
		}
	}
	return "", false, nil
}

// createSessionInWorkspace 发官方 `session.create { workspaceId }`（navigation.ts:245-250）。
//
// 建完必须核对 cwd：老宿主（`origin/main`）只解 mode，收到 workspaceId 会当成
// 不认识而忽略，把会话建在进程 cwd —— 那是"收下却没照做"，客户端必须自己看穿。
func (m *model) createSessionInWorkspace(ctx context.Context, workspace pluginapi.Workspace) (string, error) {
	created, err := m.cli.SessionCreate(ctx, apiclient.SessionCreateRequest{
		WorkspaceID: workspace.WorkspaceID,
	})
	if err != nil {
		// 官方 manager.ts:505-523：发布先于挂载，`session/workspace-attach-failed`
		// 的 details.sessionId 是一份**真实存在**的会话，要当 Ungrouped 暴露，
		// 不能因为"没挂上"就让用户看不到它。
		if published, ok := apiclient.AttachFailedSessionID(err); ok {
			m.refreshList()
			return "", fmt.Errorf("会话 %q 已创建但没挂到工作区「%s」（它已在 Ungrouped 里）：%w",
				published, workspace.Title, err)
		}
		return "", err
	}
	if err := m.verifySessionCwd(ctx, created.SessionID, workspace.Path); err != nil {
		return "", fmt.Errorf("%w（会话 %q）：%w", errWorkspaceSessionCwdMismatch, created.SessionID, err)
	}
	return created.SessionID, nil
}

// createSessionInDirectory 发官方 `session.create { cwd }`：会话自己拥有这个目录。
//
// 官方字段语义（`types.ts:271-273`）：`cwd` 是"没有点名工作区时会话自己拥有的目录"。
// 目录流只在宿主**没有**工作区注册表（`workspace.create` 回 501 not_implemented）
// 时才走这里——那时没有 workspaceId 可发，`{cwd}` 是官方唯一的表示法。
func (m *model) createSessionInDirectory(ctx context.Context, dir string) (string, error) {
	created, err := m.cli.SessionCreate(ctx, apiclient.SessionCreateRequest{Cwd: dir})
	if err != nil {
		if published, ok := apiclient.AttachFailedSessionID(err); ok {
			m.refreshList()
			return "", fmt.Errorf("会话 %q 已创建但没有挂上目录 %q：%w", published, dir, err)
		}
		return "", err
	}
	if err := m.verifySessionCwd(ctx, created.SessionID, dir); err != nil {
		return "", fmt.Errorf("%w（会话 %q，目录 %q）：%w",
			errWorkspaceSessionCwdMismatch, created.SessionID, dir, err)
	}
	return created.SessionID, nil
}

// verifySessionCwd 读回刚建的会话，确认它的 cwd 就是期望目录。
//
// 只读 `session.load`（与空白判定同一个读取口），不猜路径、不做归一化以外的修饰。
func (m *model) verifySessionCwd(ctx context.Context, sessionID, want string) error {
	loaded, err := m.cli.Load(ctx, sessionID, 1)
	if err != nil {
		return fmt.Errorf("读取刚建的会话失败: %w", err)
	}
	if !sameWorkspaceSelectionPath(loaded.Cwd, want) {
		return fmt.Errorf("宿主把会话建在了 %q，期望 %q；需要宿主支持官方 workspaceId/cwd 字段（DSH-GO PR #63）",
			loaded.Cwd, want)
	}
	return nil
}

// adoptPickedDirectory 是"目录选定、但没能变成工作区"时的官方回落：会话自己拥有该目录。
//
// 只在宿主不提供 workspace registry 时走（见 dirpicker_wiring 的判据）。
// 走官方 `{cwd}`，绝不把请求路径冒充成 workspaceId。
func (m *model) adoptPickedDirectory(dir string) {
	if m == nil || m.cli == nil || strings.TrimSpace(dir) == "" {
		return
	}
	m.setStatus("正在该目录下新建会话…")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), workspaceSelectionTimeout)
		defer cancel()
		sessionID, err := m.createSessionInDirectory(ctx, dir)
		if err != nil {
			m.setStatus("新建会话失败：" + err.Error())
			return
		}
		m.refreshList()
		m.openSession(sessionID)
	}()
}

func sameWorkspaceSelectionPath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
