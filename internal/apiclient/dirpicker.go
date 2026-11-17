// dirpicker.go —— 官方目录选择器的**二选一组合**：browse 两条通道 + native 一条 +
// 「把这个目录建成工作区」。
//
// 官方把目录选择做成两套**互斥后端**，一个 Context 只能组合一套：
//
//	native：`pick`                    —— OS 原生选择器（`dsh-client-ui-directory-picker-native`）
//	browse：`list` + `createDirectory` —— 应用内浏览器（`dsh-client-ui-directory-picker-browse`）
//
// 两个包填的是**同一对** `directoryFlow` 洞，所以同时存在即冲突（客户端流程重复、
// 槽位重复）。宿主的组合决策在 `dsh-host-directory-picker-auto`，本仓宿主移植见
// `HOST:cmd/dshd/serve_directory_picker_seam.go`；**不属于本次组合的动词一律回 501**
// 并带 `details.capability`（`OFF:dsh-api-workspace-controller/src/directory-picker.ts:105-119`
// 的 `requireCapability`）。
//
// # 客户端怎么问出组合（`ProbeDirectoryPicker`）
//
//   - 打**只读**的 `directoryPicker.list`：200 ⇒ 本次组合是 browse；
//   - 501 且 `details.capability = native` ⇒ 本次组合是 native；
//   - 其他一切（含形状对不上的 501）⇒ **失败**，不猜、不静默退化成某一套。
//
// **绝不拿 `pick` 当探针**：它是交互动词，打一下会真的弹出原生目录框。
//
// 宿主侧拒绝的信封逐字（`HOST:internal/transport/handlers_workspace_official.go`
// 的 `directoryPickerUnavailable`，HTTP 501）：`error.code = "not_implemented"`、
// `error.message` 以 `directory-picker/unavailable: ` 开头、**信封顶层**
// `details.capability` = 本次声明的 kind。
//
// # browse 的三条方法
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
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ── 组合（官方互斥的二选一） ─────────────────────────────────────────────

// DirectoryPickerKind 是宿主本次启动的 `directoryPicker` **组合**。
//
// 取值只有官方的两个字面量（`dsh-host-directory-picker/src/index.ts:19-28` 的
// native、`:35-59` 的 browse）。空串 = 没探到 / 探针失败 = **两套都不服务**，
// 界面按"隐藏入口"处理，不拿它当某个默认值。
type DirectoryPickerKind string

const (
	// DirectoryPickerKindBrowse：宿主服务 `list` + `createDirectory`（应用内浏览器）。
	DirectoryPickerKindBrowse DirectoryPickerKind = "browse"
	// DirectoryPickerKindNative：宿主服务 `pick`（OS 原生选择器）。
	DirectoryPickerKindNative DirectoryPickerKind = "native"
)

// String 让日志/报错里看到的是 `browse` / `native`，不是一串字节。
func (k DirectoryPickerKind) String() string { return string(k) }

// Valid 报告 k 是否是官方组合之一（空串与拼错的值都不是）。
func (k DirectoryPickerKind) Valid() bool {
	return k == DirectoryPickerKindBrowse || k == DirectoryPickerKindNative
}

// directoryPickerUnavailablePrefix 是宿主"这个动词不属于本次组合"的 message 前缀。
//
// 逐字来自官方码 `directory-picker/unavailable`（`OFF:.../directory-picker.ts:108`）；
// 本仓四字段信封是闭合码表，所以官方码落在 message 前缀里，`{capability}` 落在
// 信封顶层 `details` 那一格（`HOST:internal/transport/handlers_workspace_official.go`
// 的 `directoryPickerUnavailable`）。
const directoryPickerUnavailablePrefix = "directory-picker/unavailable:"

// directoryPickerRefusal 判断一次失败是不是**官方的组合拒绝**，是就返回它点名的 kind。
//
// 四条线必须**同时**对上，缺一条都不算（少认一条会把别的 501 当成组合信息）：
//
//	HTTP 501     —— 宿主用 `http.StatusNotImplemented` 表达"这台宿主没有这个交互"
//	error.code   —— `not_implemented`（本仓四字段里的对应码）
//	message 前缀 —— `directory-picker/unavailable:`（官方码）
//	details.capability —— 本次声明的 kind（客户端靠它二选一）
//
// 状态码这一条是本方法自己看的：`APIError` 不带 HTTP 状态（四字段是冻结契约），
// 所以调用方把状态一起传进来——**不靠猜**。
func directoryPickerRefusal(status int, err error) (DirectoryPickerKind, bool) {
	if status != http.StatusNotImplemented {
		return "", false
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return "", false
	}
	if apiErr.Code != "not_implemented" {
		return "", false
	}
	if !strings.HasPrefix(apiErr.Message, directoryPickerUnavailablePrefix) {
		return "", false
	}
	served, _ := apiErr.Details["capability"].(string)
	kind := DirectoryPickerKind(served)
	if !kind.Valid() {
		return "", false
	}
	return kind, true
}

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
// 官方 owner 并不是在 pick 后直接采用 path：`WorkspacePickFlow.adoptDirectory`
// 先等待 createWorkspace，再把**返回值的 workspaceId** 交给 onPick
// （`OFF:ui-workspace/src/client/WorkspacePicker.tsx:152-161`）。所以这里必须保留
// `{workspace,created}` 中的 workspace；缺 id/path 是宿主违约，不能拿请求里的 path
// 猜一份 WorkspaceView 冒充成功。
func (c *Client) WorkspaceCreate(ctx context.Context, path string) (WorkspaceView, error) {
	var out struct {
		Workspace WorkspaceView `json:"workspace"`
		Created   bool          `json:"created"`
	}
	if err := c.Call(ctx, "workspace.create", map[string]any{"path": path}, &out); err != nil {
		return WorkspaceView{}, err
	}
	if out.Workspace.WorkspaceID == "" || out.Workspace.Path == "" {
		return WorkspaceView{}, fmt.Errorf("workspace.create 没有返回可选中的 workspace（workspaceId/path 缺一不可）")
	}
	return out.Workspace, nil
}

// WorkspaceRegistryUnsupported 报告这次失败是不是"宿主根本没有工作区注册表"。
//
// 判据是**协议事实**，不是中文文本：宿主没接 `workspace.create` 时回
// HTTP 501 + 四字段 `code = "not_implemented"`（`HOST:internal/transport/
// handlers_workspace_official.go` 的 `officialNotImplemented` →
// `officialWorkspaceFaultStatus` 对 `not_implemented`/`workspace/unsupported`
// 都映射成这个码）。调用方据此决定要不要退回"会话自己拥有该目录"的官方 `{cwd}`。
//
// 其余失败（bad_request / not_found / 权限 …）**不**算这一类：那是这次路径的问题，
// 退回 `{cwd}` 只会把真实错误掩掉。
func WorkspaceRegistryUnsupported(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Code == "not_implemented"
}

// ── 组合探测（二选一的唯一判据） ──────────────────────────────────────────

// ProbeDirectoryPicker 问出宿主本次启动的组合：browse 还是 native。
//
// 判据逐字照宿主 seam 的任务书（**只打只读动词**）：
//
//	打 directoryPicker.list
//	  → 200                                  ⇒ browse
//	  → 501 且 details.capability = native   ⇒ native
//	  → 其它                                  ⇒ 失败（不进任何一套）
//
// **为什么绝不拿 `pick` 探**：`pick` 是交互动词，打一下会真的在宿主屏幕上
// 弹出原生目录框。探针必须是只读的、点开前就能安全执行的；`list` 只读盘。
//
// 返回的 kind 只在成功时非空。失败时调用方**必须隐藏入口**——不许退回到
// 某一套默认（那会把"宿主不支持"画成"点了没反应"或"打开了空对话框"）。
//
// 这个调用**每次启动只该发一次**：探针有网络开销（而且 native 组合下它必然
// 是一条 501）。调用方（壳）缓存结果，不要每次打开对话框都再探一次。
func (c *Client) ProbeDirectoryPicker(ctx context.Context) (DirectoryPickerKind, error) {
	var listing DirectoryListing
	status, err := c.CallStatus(ctx, "directoryPicker.list", struct{}{}, &listing)
	if err == nil {
		// 200 ⇒ browse。但 200 的**载荷**也要是目录列表才算数：宿主回
		// `{"result":null}` 时 `Call` 不报错、解出来是零值——那是宿主坏了，
		// 不是"browse 可用"。不查这一下，坏宿主会被当成 browse，然后用户点开
		// 一个永远是空的对话框。
		if status < 200 || status >= 300 {
			return "", fmt.Errorf("directoryPicker.list 回了 %d（不是 2xx）", status)
		}
		if listing.Path == "" && listing.Home == "" {
			return "", fmt.Errorf("directoryPicker.list 回了 2xx 但载荷不是目录列表（path 与 home 都是空）")
		}
		return DirectoryPickerKindBrowse, nil
	}
	if kind, ok := directoryPickerRefusal(status, err); ok {
		// 官方的 501 意思是"本次组合服务的是另一套"。只有 native 是本次要用的
		// 那个答案；capability 回 `browse` 而 list 却被拒，是宿主自相矛盾，
		// 当失败处理（不拿矛盾的自述去选一套界面）。
		if kind == DirectoryPickerKindNative {
			return DirectoryPickerKindNative, nil
		}
		return "", fmt.Errorf(
			"directoryPicker.list 被拒，但宿主自称服务的是 %q（与 200=browse 矛盾）：%w", kind, err)
	}
	return "", fmt.Errorf("探不出 directoryPicker 组合（打的是只读的 directoryPicker.list）：%w", err)
}

// ── native 的 pick ──────────────────────────────────────────────────────

// untimed 返回一个**不带整体超时**的客户端副本。
//
// 只有 `pick` 用它：原生目录框是**人工交互**——用户在框里逛三分钟是完全正常的
// 用法，而 `New` 给的 8s 整体超时会把正常操作变成一次失败（而且失败得莫名其妙：
// 宿主那边框还开着）。其余方法一律继续用带超时的客户端。
//
// 副本是按值拷的：`http.Client` 没有锁，拷出去改 Timeout 不影响原客户端
// （cookies 是共享的只读切片，两边都只是读）。
func (c *Client) untimed() *Client {
	cc := *c
	if c.HTTP != nil {
		hc := *c.HTTP
		hc.Timeout = 0
		cc.HTTP = &hc
	}
	return &cc
}

// DirectoryPick 弹宿主的 OS 原生目录选择器（宿主 `directoryPicker.pick`）。
//
// 只有**本次组合是 native** 时才能调：组合是 browse 的宿主会给这条回 501
// （`details.capability = browse`）。调用前请先用 `ProbeDirectoryPicker` 问过。
//
// 返回 (path, true, nil) = 选中了这个目录；(_, false, nil) = **操作员取消**
// ——官方结果类型就是 `string | null`（`OFF:.../handlers_native_official.go` 的
// `writeOfficialNullResult`），null 是**值**不是"没有载荷"，所以取消不是错误。
//
// 没有给 ctx 加超时：调用方自己决定等多久（原生框可以开很久）。壳那边给的
// 是一个**足够长**的上限，不是 8s 的传输超时。
func (c *Client) DirectoryPick(ctx context.Context) (string, bool, error) {
	// 用 `*string` 接：官方 null 解成 nil（"取消"），解成 `string` 会得到空串，
	// 于是"取消"与"返回了空路径"就分不开了。
	var out *string
	if err := c.untimed().Call(ctx, "directoryPicker.pick", struct{}{}, &out); err != nil {
		return "", false, err
	}
	if out == nil {
		return "", false, nil
	}
	path := strings.TrimSpace(*out)
	if path == "" {
		// 宿主回了 `""`：官方契约里不是合法结果（取消必须是 null）。这不是
		// "取消了"，是宿主坏了——按失败报，不静默当成取消。
		return "", false, fmt.Errorf("directoryPicker.pick 回了空路径（取消应当是 null，不是空串）")
	}
	return path, true, nil
}
