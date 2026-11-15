// workspace_files.go —— 官方 `workspaceFiles.*` 的文件读取面（#18 file-actions 的数据源）。
//
// # 为什么需要它
//
// 官方 `dsh-client-ui-file-actions` 的两条动作（下载 / 另存为）都要**原始字节**：
// `FileActionSource.read` 走的是 `ctx.remote.workspaceFiles.readBytes`，不是 `fs.read`
// （`fs.read` 是文本预览，见 internal/apiclient/fs.go），也不是 stat。
//
//	OFF:ui-file-actions/src/client/context-menus.tsx:23-28
//	  const read = (sessionId, path, mediaType, error) => async (signal) => {
//	    const result = await ctx.remote.workspaceFiles.readBytes(sessionId, path, {}, signal)
//	    if (!result.ok) throw new Error(error)
//	    return new Blob([new Uint8Array(result.value.data)], { type: mediaType })
//	  }
//
// # 宿主是真的（不是注释）
//
//	HOST:internal/transport/handlers_workspace_official.go:70   方法名 workspaceFiles.readBytes
//	HOST:internal/transport/handlers_workspace_official.go:997  handler（scope/path/options 三个位置参数）
//	HOST:cmd/dshd/serve_workspace_api.go:570                   生产接线 FilesReadBytes: h.filesReadBytes
//	HOST:cmd/dshd/serve_workspace_api.go:1208                 真实现（工作区根解析 + 符号链接拒绝）
//
// 所以这不是"等宿主补"的能力，是界面侧一直没接。
//
// # 与官方的一处显式差异：`data` 的编码
//
// 官方 typert 里 `data` 是 `Uint8Array`；L1 的 JSON 表达是 **base64 + `encoding:"base64"`**
// （HOST:internal/transport/handlers_workspace_official.go:208-220 的注释把这条差异写死了）。
// 本方法在客户端**解码**成 []byte 再交给界面，所以调用方拿不到"到底是不是 base64"
// 这种中间态——坏数据在这里就是错误，不会流到绘制路径上。
package apiclient

import (
	"context"
	"encoding/base64"
	"fmt"
)

// WorkspaceFileBytes 是 `workspaceFiles.readBytes` 的结果（客户端已解码）。
//
// 字段与宿主 `OfficialWorkspaceFileBytes` 一一对应，只有 Data 从 base64 字符串
// 变成 []byte。
type WorkspaceFileBytes struct {
	// Offset 是这次窗口在文件里的起始字节。
	Offset int64
	// Data 是**已解码**的原始字节。
	Data []byte
	// EOF 表示这次窗口已经到文件末尾。
	EOF bool
	// AbsolutePath 是宿主解析出来的绝对路径（工作区根的解析在宿主侧，
	// 客户端不许自己拼——见本文件头）。
	AbsolutePath string
	// Version 是宿主的版本令牌（stat/changes 同源），界面不用它，留着给"同一份内容"判断。
	Version string
	// Bytes 是文件总字节数；nil = 宿主没给。
	Bytes *int64
}

// WorkspaceFileReadBytes 读一个工作区文件的原始字节（官方 `workspaceFiles.readBytes`）。
//
// 参数形状（宿主 DisallowUnknownFields，逐字核对）：
//
//	sessionId  会话身份（宿主同一字段的别名还有 workspaceFileScopeId / agentId）
//	path       工作区内的**相对路径**（根目录给空串）
//	options    官方必填对象；读整个文件给 {}
//
// 失败：宿主按官方返回 `workspace-file/not-found` / `not-regular-file` 这类码，
// 四字段错误原样上抛，调用方不许把"读不到"当成"文件是空的"。
func (c *Client) WorkspaceFileReadBytes(ctx context.Context, sessionID, path string) (*WorkspaceFileBytes, error) {
	var out struct {
		Offset       int64  `json:"offset"`
		Data         string `json:"data"`
		Encoding     string `json:"encoding"`
		EOF          bool   `json:"eof"`
		AbsolutePath string `json:"absolutePath"`
		Version      string `json:"version"`
		Bytes        *int64 `json:"bytes,omitempty"`
	}
	params := map[string]any{
		"sessionId": sessionID,
		"path":      path,
		// options 是官方必填参数（HOST:handlers_workspace_official.go:1130-1135：
		// 缺它宿主直接 400），读整个文件就是空对象。
		"options": struct{}{},
	}
	if err := c.Call(ctx, "workspaceFiles.readBytes", params, &out); err != nil {
		return nil, err
	}
	// 官方 encoding 只有 base64 一种（宿主缺省也是 base64）；给了别的就别猜。
	if out.Encoding == "" {
		out.Encoding = "base64"
	}
	if out.Encoding != "base64" {
		return nil, fmt.Errorf("workspaceFiles.readBytes: 不认识的 data 编码 %q", out.Encoding)
	}
	data, err := base64.StdEncoding.DecodeString(out.Data)
	if err != nil {
		return nil, fmt.Errorf("workspaceFiles.readBytes: data 不是合法 base64: %w", err)
	}
	return &WorkspaceFileBytes{
		Offset: out.Offset, Data: data, EOF: out.EOF,
		AbsolutePath: out.AbsolutePath, Version: out.Version, Bytes: out.Bytes,
	}, nil
}
