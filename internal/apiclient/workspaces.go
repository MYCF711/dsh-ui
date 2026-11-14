// workspaces.go —— 官方 workspace registry 的读/写客户端（#62 的数据面）。
//
// # 为什么不用 `workspace.list`
//
// 本仓既有的 `workspace.list` 是**自定契约**：它列某个目录里的条目（fs.go），
// 不是官方 workspace registry。官方工作区列表是 `workspace.registry`
// （`HOST:internal/transport/handlers_workspace_registry.go:12-40`），
// 返回 Order / Workspaces / DefaultWorkspaceId / archivedSessionIds /
// pinnedSessionIds——名字、顺序、默认工作区、归档/置顶集合都在里面。
//
// # 契约（逐条对宿主，不是凭印象）
//
//	workspace.registry  无参数；结果 {version, defaultWorkspaceId?, order,
//	                    workspaces[], archivedSessionIds[], pinnedSessionIds[],
//	                    registryPath?}。空集合必须是数组不是 null（宿主已规整）。
//	workspace.rename    {workspaceId, title}；结果 {workspace}（title 已规整）。
//	workspace.delete    {workspaceId}；结果 {deleted: true}。
//
// 三条都是 POST /api/<method> + 平铺参数 + {result}/四字段错误；
// 失败（重名/空标题/不存在）是四字段错误，**不许翻成成功**。
package apiclient

import "context"

// WorkspaceView 是官方 `workspace.registry` 里的一条工作区。
//
// 字段名逐字对齐宿主 `OfficialWorkspace`（handlers_workspace_official.go:116-125）。
// `SessionIDs` 是"按手动顺序记在这个工作区名下的会话"；空数组 = 没有会话。
type WorkspaceView struct {
	WorkspaceID string   `json:"workspaceId"`
	Path        string   `json:"path"`
	Title       string   `json:"title"`
	SessionIDs  []string `json:"sessionIds"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

// WorkspaceRegistry 是 `workspace.registry` 的结果。
type WorkspaceRegistry struct {
	Version            int             `json:"version"`
	DefaultWorkspaceID string          `json:"defaultWorkspaceId,omitempty"`
	Order              []string        `json:"order"`
	Workspaces         []WorkspaceView `json:"workspaces"`
	ArchivedSessionIDs []string        `json:"archivedSessionIds"`
	PinnedSessionIDs   []string        `json:"pinnedSessionIds"`
	RegistryPath       string          `json:"registryPath,omitempty"`
}

// WorkspaceRegistryView 读工作区注册表（宿主 `<home>/state/workspaces.json`）。
func (c *Client) WorkspaceRegistryView(ctx context.Context) (*WorkspaceRegistry, error) {
	var out WorkspaceRegistry
	if err := c.Call(ctx, "workspace.registry", struct{}{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// WorkspaceRename 改一条工作区的显示名（宿主 `workspace.rename`）。
//
// 宿主会 trim 标题；空标题/重复标题回四字段错误（不是静默修正）。
func (c *Client) WorkspaceRename(ctx context.Context, workspaceID, title string) (*WorkspaceView, error) {
	var out struct {
		Workspace WorkspaceView `json:"workspace"`
	}
	params := map[string]any{"workspaceId": workspaceID, "title": title}
	if err := c.Call(ctx, "workspace.rename", params, &out); err != nil {
		return nil, err
	}
	return &out.Workspace, nil
}

// WorkspaceInsertSessionBefore 把一个会话插到某条工作区名册里 `beforeSessionID` 之前
// （宿主 `workspace.insertSessionBefore`）。
//
// 这是侧栏拖拽排会话顺序的唯一写路径：官方的 `sessionIds` 就是"手动顺序"
// （`OFF:dsh-api-workspace-controller/src/feed.ts:22-33`），拖一次 = 在目标位置前插一次
// （`OFF:dsh-api-workspace-controller/src/index.ts:149`）。
//
// `beforeSessionID` 传空串 = 插到该组末尾：**那时这个字段要整个省略**，
// 宿主 `insertSessionBefore` 对"给了但是空串"回 400
// （`HOST:internal/transport/handlers_workspace_official.go:840-842`）。
// 宿主会校验工作区与会话都真实存在，不存在回四字段错误——**不许**在客户端把它翻成成功。
//
// 参数按宿主接受的两支形状里**平铺**那一支（`officialPickObject` 也收
// `{request:{…}}`，但本仓其余 workspace 方法一律平铺，不混两种形状）。
func (c *Client) WorkspaceInsertSessionBefore(ctx context.Context, workspaceID, sessionID, beforeSessionID string) (*WorkspaceView, error) {
	var out struct {
		Workspace WorkspaceView `json:"workspace"`
	}
	params := map[string]any{"workspaceId": workspaceID, "sessionId": sessionID}
	if beforeSessionID != "" {
		params["beforeSessionId"] = beforeSessionID
	}
	if err := c.Call(ctx, "workspace.insertSessionBefore", params, &out); err != nil {
		return nil, err
	}
	return &out.Workspace, nil
}

// WorkspaceDelete 把一条工作区注册从 registry 移除（宿主 `workspace.delete`）。
//
// **不删文件夹、不删会话记录**：官方 `delete.desc` 明确会话会落到"未分组"。
// 结果 `deleted` 不是 true 视为失败（宿主契约是 literal true）。
func (c *Client) WorkspaceDelete(ctx context.Context, workspaceID string) error {
	var out struct {
		Deleted bool `json:"deleted"`
	}
	if err := c.Call(ctx, "workspace.delete", map[string]any{"workspaceId": workspaceID}, &out); err != nil {
		return err
	}
	if !out.Deleted {
		return &APIError{Code: "internal", Message: "workspace.delete 没有确认删除（deleted=false）"}
	}
	return nil
}
