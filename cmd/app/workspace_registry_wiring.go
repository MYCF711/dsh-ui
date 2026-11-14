// workspace_registry_wiring.go —— 官方 workspace registry 的壳侧缓存与动作（#62）。
//
// # 为什么要有这一层
//
// 插件只拿 `pluginapi.Snapshot` / `Actions`（红线 R2），不碰 `*apiclient.Client`。
// 而 #62 的工作区改名/删除要写**真实 registry**（宿主 `workspace.rename` /
// `workspace.delete`），所以读取缓存与写动作都由壳转一手：
//
//	Snapshot.Workspaces          ← 本文件缓存（workspace.registry 的投影）
//	Actions.WorkspaceRename      ← m.renameWorkspace
//	Actions.WorkspaceDelete      ← m.deleteWorkspace
//
// 三条语义：
//
//   - **空 ≠ 没有**：workspacesKnown=false 表示"还没拉到"，插件必须退回按 cwd
//     分组；拉到空数组才是"registry 真的是空"。
//   - 写动作**同步返回错误**：对话框要显示宿主的四字段错误（空标题/重名等），
//     所以调用方在 goroutine 里调、拿 err 回填；壳不把错误吞成状态栏一句话。
//   - 写成功后**先改本地缓存再重拉**：让下一帧立刻看到新标题/消失的行。
package main

import (
	"context"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// workspaceWriteTimeout 是改名/删除的请求预算。与其它一次性写动作同档。
const workspaceWriteTimeout = 8 * time.Second

// cloneWorkspaces 在锁内做一份深拷贝（SessionIDs 也要拷，别把活切片交出去）。
func cloneWorkspaces(in []pluginapi.Workspace) []pluginapi.Workspace {
	if len(in) == 0 {
		return nil
	}
	out := make([]pluginapi.Workspace, len(in))
	for i, workspace := range in {
		out[i] = workspace
		out[i].SessionIDs = append([]string(nil), workspace.SessionIDs...)
	}
	return out
}

// refreshWorkspaces 后台拉一次 `workspace.registry`。
//
// 失败不清旧缓存（旧的工作区名/分组比"全丢了"有用），只记错误。
func (m *model) refreshWorkspaces() {
	if m.cli == nil {
		return
	}
	m.mu.Lock()
	if m.workspacesPending {
		m.mu.Unlock()
		return
	}
	m.workspacesPending = true
	m.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), workspaceWriteTimeout)
		defer cancel()
		registry, err := m.cli.WorkspaceRegistryView(ctx)

		m.mu.Lock()
		m.workspacesPending = false
		if err != nil {
			m.workspacesErr = err.Error()
		} else {
			m.workspaces = viewToPluginWorkspaces(registry.Workspaces)
			m.workspacesKnown = true
			m.workspacesErr = ""
		}
		m.mu.Unlock()
		m.inv.Invalidate()
	}()
}

// viewToPluginWorkspaces 把 apiclient 视图翻成插件契约的类型。
func viewToPluginWorkspaces(in []apiclient.WorkspaceView) []pluginapi.Workspace {
	if len(in) == 0 {
		return nil
	}
	out := make([]pluginapi.Workspace, 0, len(in))
	for _, workspace := range in {
		out = append(out, pluginapi.Workspace{
			WorkspaceID: workspace.WorkspaceID,
			Path:        workspace.Path,
			Title:       workspace.Title,
			SessionIDs:  append([]string(nil), workspace.SessionIDs...),
			CreatedAt:   workspace.CreatedAt,
			UpdatedAt:   workspace.UpdatedAt,
		})
	}
	return out
}

// renameWorkspace 是 `Actions.WorkspaceRename`：同步写宿主，成功后更新本地缓存。
func (m *model) renameWorkspace(workspaceID, title string) error {
	if m.cli == nil {
		return errNoWorkspaceClient
	}
	ctx, cancel := context.WithTimeout(context.Background(), workspaceWriteTimeout)
	defer cancel()
	workspace, err := m.cli.WorkspaceRename(ctx, workspaceID, title)
	if err != nil {
		return err
	}
	m.mu.Lock()
	for i := range m.workspaces {
		if m.workspaces[i].WorkspaceID == workspaceID {
			m.workspaces[i].Title = workspace.Title
			m.workspaces[i].UpdatedAt = workspace.UpdatedAt
			break
		}
	}
	m.mu.Unlock()
	m.inv.Invalidate()
	go m.refreshWorkspaces()
	return nil
}

// deleteWorkspace 是 `Actions.WorkspaceDelete`：同步写宿主，成功后从本地缓存摘掉。
func (m *model) deleteWorkspace(workspaceID string) error {
	if m.cli == nil {
		return errNoWorkspaceClient
	}
	ctx, cancel := context.WithTimeout(context.Background(), workspaceWriteTimeout)
	defer cancel()
	if err := m.cli.WorkspaceDelete(ctx, workspaceID); err != nil {
		return err
	}
	m.mu.Lock()
	kept := m.workspaces[:0]
	for _, workspace := range m.workspaces {
		if workspace.WorkspaceID != workspaceID {
			kept = append(kept, workspace)
		}
	}
	m.workspaces = kept
	m.mu.Unlock()
	m.inv.Invalidate()
	go m.refreshWorkspaces()
	return nil
}

// errNoWorkspaceClient 是壳没有传输客户端时的诚实错误（不是静默成功）。
var errNoWorkspaceClient = &apiclient.APIError{
	Code:    "unsupported",
	Message: "没有传输客户端，无法写工作区 registry",
	Hint:    "以真实宿主启动（-base）后再试",
}

// insertSessionBefore 是 `Actions.WorkspaceInsertSessionBefore`：把一次拖拽落到
// 宿主的 `sessionIds` 手动序上，成功后**原地更新本地缓存的那条工作区**。
//
// 为什么先改本地再重拉：拖拽是"松手就要看到结果"的交互，等一次 HTTP 往返才挪行
// 会让人以为没拖上。本地改的是**同一个真值**（宿主刚回的那份 Workspace），
// 不是自造的乐观值——重拉只是核对。
func (m *model) insertSessionBefore(workspaceID, sessionID, beforeSessionID string) error {
	if m.cli == nil {
		return errNoWorkspaceClient
	}
	ctx, cancel := context.WithTimeout(context.Background(), workspaceWriteTimeout)
	defer cancel()
	workspace, err := m.cli.WorkspaceInsertSessionBefore(ctx, workspaceID, sessionID, beforeSessionID)
	if err != nil {
		return err
	}
	m.mu.Lock()
	for i := range m.workspaces {
		if m.workspaces[i].WorkspaceID == workspaceID {
			m.workspaces[i].SessionIDs = append([]string(nil), workspace.SessionIDs...)
			m.workspaces[i].UpdatedAt = workspace.UpdatedAt
			break
		}
	}
	m.mu.Unlock()
	m.inv.Invalidate()
	go m.refreshWorkspaces()
	return nil
}
