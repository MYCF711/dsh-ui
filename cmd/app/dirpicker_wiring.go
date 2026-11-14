// dirpicker_wiring.go —— 把「官方目录浏览器」接进产品：壳注入数据源 + 一键打开。
//
// # 为什么这两步必须由壳做
//
// `plugins/dsdirpicker`（官方 `dsh-client-ui-directory-picker-browse` 的落地）
// 自己拿不到 `*apiclient.Client`，也不该拿：插件只拿 `*Scope`（红线 R2）。
// 所以它暴露两个注入点（见 `plugins/dsdirpicker/browse.go:216,225`）：
//
//	SetSource(DirSource)        —— 数据面：列目录 / 建目录 / 注册工作区
//	SetOnPicked(func(path))     —— 「打开」成功之后的回调
//
// 壳是唯一同时知道 `cli` 和两个插件实例的地方，所以由壳转这一手。
//
// # 数据从哪来（三条都是宿主真实现，不是壳自己算的）
//
//   - `directoryPicker.list`            → `HOST:cmd/dshd/serve_workspace_api.go:611`（`List: h.pickerList`）
//     handler `HOST:internal/transport/handlers_workspace_official.go:1282`；
//     真实现 `HOST:cmd/dshd/serve_workspace_api.go:1668` 起（`os.ReadDir` 真目录浏览）。
//   - `directoryPicker.createDirectory` → 同上 `:612`（`CreateDirectory: h.pickerCreateDirectory`）
//   - `workspace.create`                → `HOST:cmd/dshd/serve_workspace_api.go:583`（`Create: h.createWorkspace`）
//
// # 为什么不用 `workspace.list` 顶替
//
// 本仓的 `workspace.list` 是**自定契约**（列目录条目，见 `internal/apiclient/fs.go:8-12`），
// 官方的目录浏览是 `directoryPicker.*`。两者语义不同，混用会把「宿主 home」与
// 「当前 cwd」搞错——`directoryPicker.list` 省略 `path` 时列的是**宿主 home**。
package main

import (
	"context"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/plugins/dsdirpicker"
)

// dirSource 是壳给 `dsdirpicker` 的数据源实现。
//
// 它把三个宿主方法原样转出去，**不做任何加工**：不做路径猜测、不缓存、
// 失败了就把宿主错误原文交回去（对话框负责显示原文，见 `browse.go` 的错误行）。
type dirSource struct{ cli *apiclient.Client }

func (s dirSource) List(ctx context.Context, path string) (*apiclient.DirectoryListing, error) {
	return s.cli.DirectoryList(ctx, path)
}

func (s dirSource) Create(ctx context.Context, parent, name string) (string, error) {
	return s.cli.DirectoryCreate(ctx, parent, name)
}

func (s dirSource) Register(ctx context.Context, path string) error {
	return s.cli.WorkspaceCreate(ctx, path)
}

// wireDirPicker 把数据源与回调注入目录浏览器插件。
//
// `cli == nil`（无头的部分测试会这么构造）时**什么都不注入**——插件拿到 nil
// 就不会画任何东西（官方「空槽不占位」同义），测试里也不会意外发请求。
func wireDirPicker(m *model) {
	if m.cli == nil {
		return
	}
	dsdirpicker.SetSource(dirSource{cli: m.cli})
	// 「打开」成功后：把这个目录登记为工作区之后，壳要做的事。
	//
	// 现在只做一件事：**把侧栏的会话列表刷新一下**，让新工作区下一个会话落进去时
	// 就能出现在列表里。这里刻意**不**弹提示、不切会话——官方 ui-workspace 的
	// directory-flow 在 adopt 一个路径之后也只是把工作区加进注册表，UI 由
	// 侧栏自己的数据流反映。我们没有 `workspace.registry` 的读通道
	// （GUI apiclient 尚无该客户端），所以看不到注册表的形状，
	// **也就不假装刷新了工作区树**：只刷会话列表，这是真能生效的那一半。
	dsdirpicker.SetOnPicked(func(path string) {
		go m.refreshList()
	})
}

// openDirectoryPicker 是 `Actions.OpenDirectoryPicker` 的实现：
// 从 kernel 里找到目录浏览器插件并打开它。
//
// 为什么遍历而不是让壳持有实例：壳持有的实例（`side`/`settings`/`statusbar`）
// 都是为了**装配顺序**或**无头截图**才持的（见 main.go 里那几个字段的注释）。
// 目录选择器两者都不需要，所以按能力查表更稳——插件没装时这里返回 false，
// 侧栏那个按钮就不会画（`Actions.OpenDirectoryPicker` 为 nil 也是这个语义）。
func (m *model) openDirectoryPicker() bool {
	if m.kernel == nil {
		return false
	}
	for _, p := range m.kernel.Plugins() {
		if opener, ok := p.(interface{ Open() bool }); ok && p.ID() == "ds-dirpicker" {
			return opener.Open()
		}
	}
	return false
}
