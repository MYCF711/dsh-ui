// fs.go —— 宿主 `workspace.list` / `fs.read` / `fs.write` 的客户端。
//
// 这三个能力**通了但没人用**（B 类里的 sidebar-files / documentpreview 就是要用它们）。
//
// ## 已知限制（照实写，别让调用方以为能做的事没做）
//
// `workspace.list` 回的是 `[]WorkspaceEntry{Name, IsDir, Size}`——**没有 path**，
// 也没有"列某个子目录"的参数。所以它只能列**当前工作区根**，进不了子目录。
// 想在侧栏做完整文件树，得先在宿主 `workspace.list` 上补 `path` 入参
// （跨线契约变更，不在界面侧能解决的范围）。
package apiclient

import "context"

// WorkspaceEntry 是 workspace.list 的一项。
type WorkspaceEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
	// Path 是完整路径。**宿主目前不给**（它只回 name/isDir/size），
	// 所以客户端要自己在 dir 基础上拼。等宿主补了就直接用。
	Path string `json:"path,omitempty"`
}

// WorkspaceList 列一个目录。
//
// dir 为空 = 宿主的进程工作目录（旧行为）；给绝对路径就列那个目录——
// 有了它文件树才能一层层往下走（旧版只能列根，因为宿主不收 path）。
func (c *Client) WorkspaceList(ctx context.Context, dir string) ([]WorkspaceEntry, error) {
	var out struct {
		Entries []WorkspaceEntry `json:"entries"`
	}
	var params any = struct{}{}
	if dir != "" {
		params = map[string]any{"path": dir}
	}
	if err := c.Call(ctx, "workspace.list", params, &out); err != nil {
		return nil, err
	}
	return out.Entries, nil
}

// FSContent 是 fs.read 的返回。
type FSContent struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
}

// FSRead 读一个文件（**绝对路径**；宿主不做工作区内限制，调用方自己判）。
func (c *Client) FSRead(ctx context.Context, path string) (*FSContent, error) {
	var out FSContent
	if err := c.Call(ctx, "fs.read", map[string]any{"path": path}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
