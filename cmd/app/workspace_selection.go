// workspace_selection.go —— WorkspacePicker 的 onPick(workspaceId) 选择语义。
//
// 官方不是“注册目录后刷新列表”：WorkspacePickFlow 等 createWorkspace 返回 WorkspaceView，
// 再把 workspaceId 交给 owner（WorkspacePicker.tsx:152-161）。ConversationContent 与
// WorkspaceBrowser 都经 UiWorkspaceService.openWorkspace/startSession 复用该工作区的
// 空白会话（navigation.ts:215-249），而不是随手打开一个非空历史会话。
//
// 当前宿主有一个明确边界：session.create 只解 mode（transport/server.go:1211-1227），
// 客户端虽会发 cwd，宿主不读（testdata/api.snapshot.json 同文）。所以本文件只复用
// 经 session.load 证实 Total==0 的空白会话；没有时可创建新会话的能力缺失必须报错，
// 绝不能创建一个落在进程 cwd 的会话再声称它属于刚选的工作区。
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

var errWorkspaceSessionCreateUnsupported = errors.New(
	"宿主 session.create 只解 mode，不支持 workspaceId/cwd；此工作区没有可复用的空白会话")

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
		sessionID, err := m.blankSessionInWorkspace(ctx, workspace, archived)
		if err != nil {
			m.finishWorkspaceSelection(seq, workspaceID, err)
			return
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
func (m *model) blankSessionInWorkspace(ctx context.Context, workspace pluginapi.Workspace, archived map[string]bool) (string, error) {
	for _, sessionID := range workspace.SessionIDs {
		if archived[sessionID] {
			continue
		}
		result, err := m.cli.Load(ctx, sessionID, 1)
		if err != nil {
			return "", fmt.Errorf("读取工作区空白会话 %q 失败: %w", sessionID, err)
		}
		if result.Total == 0 && sameWorkspaceSelectionPath(result.Cwd, workspace.Path) {
			return sessionID, nil
		}
	}
	return "", errWorkspaceSessionCreateUnsupported
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
