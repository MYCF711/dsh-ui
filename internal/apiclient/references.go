// references.go —— 官方 rc.2 `fileReferences.list` / `sessionReferenceResolver.candidates`
// 的客户端。
//
// # 为什么界面需要这两条
//
// 官方 composer 的 `@` 候选来自两个**注册的 source**（`ui-reference/src/client/index.ts:80-135`）：
//
//	file    → fileReferences.list(agentId, query)
//	session → sessionReferenceResolver.candidates(agentId, query)
//
// 本仓此前只接了文件（而且走的是 `workspace.list` 那条**自定**接口），会话引用
// 这条完全没有。两条方法宿主都已在（`HOST:cmd/dshd/serve_agent_api.go:1299/1734`），
// 所以这一层是纯客户端。
//
// 契约（宿主 `internal/transport/handlers_agent_official.go:1758-1785`、`:2042-2070`）：
//
//	fileReferences.list
//	  入参 { agentId: sessionId, query: string }（query 必填，可为空串）
//	  出参 [{ path, kind: "file"|"directory" }]
//
//	sessionReferenceResolver.candidates
//	  入参 { agentId: sessionId, query: string }
//	  出参 [{ mention, sessionId, label, displayTitle?, cwd?, sameWorkspace, createdAt }]
//
// 两者的失败都**不许折叠成空数组**：501 = 宿主没这条能力（界面隐藏该 source），
// 404/500 = 真错误（界面不画候选，但报告里要能看出是哪种）。
package apiclient

import "context"

// FileReference 是 `fileReferences.list` 的一行。
type FileReference struct {
	Path string `json:"path"`
	// Kind 是 `file` 或 `directory`（宿主只回这两个值）。
	Kind string `json:"kind"`
}

// FileReferences 列 `@` 之后那段路径的候选。
func (c *Client) FileReferences(ctx context.Context, sessionID, query string) ([]FileReference, error) {
	var out []FileReference
	params := map[string]any{"agentId": sessionID, "query": query}
	if err := c.Call(ctx, "fileReferences.list", params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SessionReferenceCandidate 是 `sessionReferenceResolver.candidates` 的一行。
type SessionReferenceCandidate struct {
	Mention       string `json:"mention"`
	SessionID     string `json:"sessionId"`
	Label         string `json:"label"`
	DisplayTitle  string `json:"displayTitle,omitempty"`
	Cwd           string `json:"cwd,omitempty"`
	SameWorkspace bool   `json:"sameWorkspace"`
	CreatedAt     int64  `json:"createdAt"`
}

// SessionReferenceCandidates 列 `@` 之后的会话引用候选。
func (c *Client) SessionReferenceCandidates(ctx context.Context, sessionID, query string) ([]SessionReferenceCandidate, error) {
	var out []SessionReferenceCandidate
	params := map[string]any{"agentId": sessionID, "query": query}
	if err := c.Call(ctx, "sessionReferenceResolver.candidates", params, &out); err != nil {
		return nil, err
	}
	return out, nil
}
