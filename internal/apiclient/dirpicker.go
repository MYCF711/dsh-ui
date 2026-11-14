// dirpicker.go —— 官方目录选择器的两条 browse 通道 + 「把这个目录建成工作区」。
//
// 这三条对应官方 `dsh-client-ui-directory-picker-browse` 驱动的那三个 Host 方法
// （该包 `index.ts:132-135` 的 `listDirectory` / `createDirectory` 注入面，
// 以及流程 owner 在「打开」时对工作区的采用）。
//
// # 每个方法的宿主证据
//
//   - `directoryPicker.list`
//     请求结构体：`HOST:internal/transport/handlers_workspace_official.go:286-299`
//     （`path` 一个字段；省略/空 = 宿主 home）。
//     handler：`HOST:internal/transport/handlers_workspace_official.go:1282`。
//     真实现（`os.ReadDir` 真浏览）：`HOST:cmd/dshd/serve_workspace_api.go:1668` 起。
//     响应：`{path, home, crumbs:[{name,path,hidden}], entries:[{name,path,hidden}], truncated}`。
//   - `directoryPicker.createDirectory`
//     请求 `{path: 父目录绝对路径, name: 单个路径段}`；
//     handler：`HOST:internal/transport/handlers_workspace_official.go:1316`。
//     响应是**一个裸 JSON 字符串**（新目录的绝对路径），不是对象——所以下面必须用
//     `var out string` 解；解成结构体会静默拿到零值。
//   - `workspace.create`
//     handler 走 `officialPickObject(p, "request", []string{"path"})`（**嵌套与平铺都收**），
//     真实现：`HOST:cmd/dshd/serve_workspace_api.go:583`（`Create: h.createWorkspace`）。
//     本客户端发**平铺**（见下）。
//
// # 为什么是「平铺信封」
//
// 本仓的契约（`testdata/api.snapshot.json` 的 `envelope.request.style = "flat"`）写明：
// 真宿主 handler 就是 `json.NewDecoder(r.Body).Decode(&in)` 直接解平铺结构，
// 外面再套 `{"params":…}` 会让每个参数都解成零值（`client.go:272-287` 有完整的
// 事故记录）。所以这里一律平铺：`{"path": …}` / `{"path": …, "name": …}` /
// `{"path": …}`，与 `fs.go` 的 `WorkspaceList` / `FSRead` 同形。
//
// # 别拿 `workspace.list` 顶替 `directoryPicker.list`
//
// GUI 的 `workspace.list` 是**本仓自定契约**（列某个目录的条目，`fs.go:15-41`），
// 官方 `workspace.registry` 是列"已注册的工作区"，两者都不是官方目录选择器要的
// `directoryPicker.list`（它额外给 `home` / `crumbs` / `truncated`，这三个是
// 面包屑与「文件夹过多」提示的唯一来源）。混用会画出没有证据的面包屑。
package apiclient

import (
	"context"
	"encoding/json"
)

// DirectoryItem 是 `directoryPicker.list` 响应里 `crumbs` / `entries` 的一项。
//
// 两个切片共用一个类型：官方 wire 上它们同形（`DirectoryEntry`），
// 面包屑只是"到这一层为止的祖先链"。`Hidden` 由**宿主**标（点开头的名字），
// 界面不许自己按名字猜。
type DirectoryItem struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Hidden bool   `json:"hidden"`
}

// DirectoryListing 是 `directoryPicker.list` 的响应。
//
// `Home` 不是"这次列的那一层"：它是宿主自己的家目录，界面拿它判断
// 「面包屑要从哪一段开始显示成『主目录』」（官方 `displayCrumbs`，
// `DirectoryBrowser.tsx:116-121`），也拿它推断平台的路径分隔符
// （`:132-134`，本仓只用来拼提示，不拼路径）。
type DirectoryListing struct {
	Path      string          `json:"path"`
	Home      string          `json:"home"`
	Crumbs    []DirectoryItem `json:"crumbs"`
	Entries   []DirectoryItem `json:"entries"`
	Truncated bool            `json:"truncated"`
}

// DirectoryList 列一层目录（宿主 `directoryPicker.list`）。
//
// path 为空 = **宿主的 home 目录**：官方契约里 `path?` 省略即此意
// （`DirectoryBrowser.tsx:57` 的 `listDirectory: (path?: string, …)`），
// 所以空串发 `{}`，不发 `{"path":""}` ——后者会被宿主当成一个空路径去 resolve。
func (c *Client) DirectoryList(ctx context.Context, path string) (*DirectoryListing, error) {
	var params any = struct{}{}
	if path != "" {
		params = map[string]any{"path": path}
	}
	var out DirectoryListing
	if err := c.Call(ctx, "directoryPicker.list", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DirectoryCreate 在一个已存在的父目录下建一个子目录（宿主 `directoryPicker.createDirectory`）。
//
// parent 是父目录的**绝对路径**，name 是**单个路径段**（宿主不接受带分隔符的名字）。
// 响应是裸 JSON 字符串 = 新目录的绝对路径；空串 = 宿主没给（调用方按失败处理，
// 不许自己拼一个"应该是这个"的路径去选中）。
func (c *Client) DirectoryCreate(ctx context.Context, parent, name string) (string, error) {
	params := map[string]any{"path": parent, "name": name}
	var out string
	if err := c.Call(ctx, "directoryPicker.createDirectory", params, &out); err != nil {
		return "", err
	}
	return out, nil
}

// WorkspaceCreate 把一个目录注册成工作区（宿主 `workspace.create`）。
//
// 响应体本客户端**不用**：官方「打开」的语义只是"采用这个目录"
// （`flow.ts:34` 的 `onOpen: props.onPicked`），工作区的后续投影由壳自己重拉。
// 用 `json.RawMessage` 接住，是为了将来宿主在响应里加字段时不至于解崩。
func (c *Client) WorkspaceCreate(ctx context.Context, path string) error {
	var out json.RawMessage
	return c.Call(ctx, "workspace.create", map[string]any{"path": path}, &out)
}
