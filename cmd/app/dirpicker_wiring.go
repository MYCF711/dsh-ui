// dirpicker_wiring.go —— 把「官方目录选择器」接进产品：**先问出宿主支持哪一套后端**，
// 再按那一套注入数据源 / 动作；一套都不是就把入口整块隐藏。
//
// # 为什么必须先问（官方两个后端是互斥的）
//
// 官方把「选择工作区目录」拆成两个**互斥**的客户端包，它们填的是同一对
// `directoryFlow` 洞，所以同时存在即冲突：
//
//	native（`dsh-client-ui-directory-picker-native`）—— 无界面流程，每次 `open`
//	  边沿调一次 `pick()`，结果走 `onPicked` / `onCancel` / `onError`
//	  （`OFF:dpn/src/client/flow.ts:20-90`）。**只在**
//	  `remoteAccess.whenAccess('directoryPicker/pick')` 为真时注册
//	  （`OFF:dpn/src/client/index.ts:32-41`）。
//	browse（`dsh-client-ui-directory-picker-browse`）—— 应用内 Miller 双列对话框，
//	  注入 `listDirectory` / `createDirectory`（`OFF:dpb/src/client/index.ts:176-184`），
//	  **无门控**注册同一对洞。
//
// 本仓没有 `remoteAccess` 服务，等价的问题就是"宿主这次启动声明的是哪一套"。
// 宿主侧的 seam（`HOST:cmd/dshd/serve_directory_picker_seam.go`）给的判据是：
//
//	打只读的 directoryPicker.list
//	  → 200                                  ⇒ browse
//	  → 501 且 details.capability = native   ⇒ native
//	  → 其它                                  ⇒ 失败
//
// **刻意不拿 `pick` 当探针**：它是交互动词，打一下会真的弹出宿主屏幕上的原生框。
//
// # 三种结局（都不许静默退化）
//
//	browse —— 现有路径不变：`plugins/dsdirpicker`（对话框）+ `dirSource` 数据面。
//	native —— 没有应用内对话框：点「添加工作区」= 调 `directoryPicker.pick`，
//	          选到路径后 `workspace.create` 采用它（与 browse 同一个"采用"语义）。
//	都不可用 —— **入口整块隐藏**（`Actions.OpenDirectoryPicker = nil`），并把
//	          探针失败的原因写进日志。不画"点了没反应"的按钮，也不退回到某一套
//	          默认——那等于假装宿主有能力。
//
// # 探针只在启动时打一次
//
// `wireDirPicker` 在装配末尾（第一帧之前）跑一次，结果缓存在 `m.dirPicker`。
// 之后每次打开都读缓存：探针有网络开销，而且 native 组合下它必然是一条 501。
//
// # 数据从哪来（三条都是宿主真实现，不是壳自己算的）
//
//   - `directoryPicker.list`            → `HOST:cmd/dshd/serve_workspace_api.go:611`（`List: h.pickerList`）
//     handler `HOST:internal/transport/handlers_workspace_official.go:1282`；
//     真实现 `HOST:cmd/dshd/serve_workspace_api.go:1668` 起（`os.ReadDir` 真目录浏览）。
//   - `directoryPicker.createDirectory` → 同上 `:612`（`CreateDirectory: h.pickerCreateDirectory`）
//   - `workspace.create`                → `HOST:cmd/dshd/serve_workspace_api.go:583`（`Create: h.createWorkspace`）
//   - `directoryPicker.pick`            → `HOST:internal/transport/handlers_native_official.go`
//     的 `MethodDirectoryPickerPick`（真实现 `HOST:internal/native`，Windows COM 文件对话框）
//
// # 为什么不用 `workspace.list` 顶替
//
// 本仓的 `workspace.list` 是**自定契约**（列目录条目，见 `internal/apiclient/fs.go:8-12`），
// 官方的目录浏览是 `directoryPicker.*`。两者语义不同，混用会把「宿主 home」与
// 「当前 cwd」搞错——`directoryPicker.list` 省略 `path` 时列的是**宿主 home**。
package main

import (
	"context"
	"log"
	"time"

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

func (s dirSource) Register(ctx context.Context, path string) (apiclient.WorkspaceView, error) {
	return s.cli.WorkspaceCreate(ctx, path)
}

// dirPickerMode 是本次启动的 `directoryPicker` 组合（零值 = 一套都不可用）。
type dirPickerMode uint8

const (
	// dirPickerUnavailable 是零值：探不到组合 / 探针失败 / 宿主两套都不服务。
	// 这个状态下**入口整块隐藏**——不画一个点了没反应的按钮。
	dirPickerUnavailable dirPickerMode = iota
	// dirPickerBrowse：宿主服务 `list` + `createDirectory`，走应用内对话框。
	dirPickerBrowse
	// dirPickerNative：宿主服务 `pick`，点入口直接弹 OS 原生选择器。
	dirPickerNative
)

// dirPickerProbeTimeout 是启动探针的上限。
//
// 探针是**只读本地 HTTP 调用**（列宿主 home），正常是毫秒级；给 3s 是为了
// "宿主没起来/卡住"时不把窗口启动拖到 8s 传输超时。超时 = 探针失败 = 隐藏入口。
const dirPickerProbeTimeout = 3 * time.Second

// dirPickerPickTimeout 是一次原生选择的上限。
//
// 这里**不是**传输超时那 8s：原生框是人工交互，用户逛三分钟是正常用法。
// 取 5 分钟只是防"框被遗弃在后台、never answered"——那时至少错误是可见的。
const dirPickerPickTimeout = 5 * time.Minute

// dirPickerState 是壳缓存的组合判定结果。
//
// mode/err/onPicked 由 `wireDirPicker` 在第一帧之前写一次，之后只读；nativeBusy 由 m.mu 保护。
type dirPickerState struct {
	// mode 是本次启动的组合；dirPickerUnavailable = 入口隐藏。
	mode dirPickerMode
	// err 是探针失败的原因（mode=unavailable 且宿主真起了时才有值）。
	// 它只用于日志/诊断——界面不编文案，直接隐藏入口。
	err error
	// onPicked 是 createWorkspace 结算后的壳动作；与官方一致传返回的 WorkspaceView，
	// 由其中的 workspaceId 驱动选择，不把请求 path 当工作区身份。
	onPicked func(workspace apiclient.WorkspaceView)
	// nativeBusy 对应官方 armed/in-flight：原生框未结算时不再启动第二个 pick。
	// 只由 m.mu 保护读写；mode/err 仍是启动期写一次、之后只读。
	nativeBusy bool
}

// wireDirPicker 探一次组合并按结果接线。
//
// 调用点在装配末尾（`main.go` 的 `wireDirPicker(m)`）：那时 `/connect` 已经
// 换过 cookie、`refreshList` 已经发过第一轮请求，但**第一帧还没画**——所以
// 同步探一次不会卡住任何已显示的窗口，也不会有"先画再看"的闪烁。
//
// `cli == nil`（无头的部分测试会这么构造）时**什么都不接**：插件拿到 nil 就不会
// 画任何东西（官方"空槽不占位"同义），测试里也不会意外发请求。
func wireDirPicker(m *model) {
	if m == nil || m.cli == nil {
		return
	}
	// 两种后端共用同一条"采用"语义：createWorkspace 返回 WorkspaceView 后，
	// 官方 owner 用它的 workspaceId 选择空白会话。browse 由对话框回调；native 由
	// `pickNativeDirectory` 自己调 `workspace.create` 后再调同一个入口。
	m.dirPicker.onPicked = m.adoptPickedWorkspace
	dsdirpicker.SetOnPicked(m.dirPicker.onPicked)

	ctx, cancel := context.WithTimeout(context.Background(), dirPickerProbeTimeout)
	defer cancel()
	kind, err := m.cli.ProbeDirectoryPicker(ctx)
	if err != nil {
		// 如实报错 + **隐藏入口**：不退回到某一套默认。
		m.dirPicker.mode = dirPickerUnavailable
		m.dirPicker.err = err
		dsdirpicker.SetSource(nil)
		log.Printf("目录选择器：%v —— 「添加工作区」入口隐藏（不静默退化成某一套）", err)
		return
	}

	switch kind {
	case apiclient.DirectoryPickerKindBrowse:
		m.dirPicker.mode = dirPickerBrowse
		dsdirpicker.SetSource(dirSource{cli: m.cli})
	case apiclient.DirectoryPickerKindNative:
		// native 组合没有应用内对话框：**不注入对话本源**，否则 dsdirpicker 会
		// 画出一个 list 一定被宿主拒（501）的浏览器。
		m.dirPicker.mode = dirPickerNative
		dsdirpicker.SetSource(nil)
	default:
		// ProbeDirectoryPicker 只在成功时返回合法 kind；走到这里说明它的
		// 契约被改坏了。按"不可用"处理（隐藏），不猜。
		m.dirPicker.mode = dirPickerUnavailable
		m.dirPicker.err = errUnknownDirPickerKind(kind)
		dsdirpicker.SetSource(nil)
		log.Printf("目录选择器：%v —— 「添加工作区」入口隐藏", m.dirPicker.err)
	}
}

// dirPickerAction 返回壳交给侧栏的那一格动作；nil = **入口整块隐藏**。
//
// 侧栏（`plugins/dssidebar/frame.go:249`）只在它非 nil 时画「添加工作区」——
// 这是"不画死按钮"的唯一判据，所以"不可用"必须表达成 nil，不能表达成
// "一个点了什么都不做的函数"。
func (m *model) dirPickerAction() func() {
	if m == nil || m.dirPicker.mode == dirPickerUnavailable {
		return nil
	}
	return func() { m.openDirectoryPicker() }
}

// openDirectoryPicker 是 `Actions.OpenDirectoryPicker` 的实现：按探到的组合分派。
//
// 返回值 = "这次点击有没有真的打开一个选择流程"（native 那条是异步的，返回 true
// 表示已经把请求发出去了）。
func (m *model) openDirectoryPicker() bool {
	if m == nil {
		return false
	}
	switch m.dirPicker.mode {
	case dirPickerBrowse:
		return m.openBrowseDirectoryPicker()
	case dirPickerNative:
		m.mu.Lock()
		if m.dirPicker.nativeBusy {
			m.mu.Unlock()
			return false // 官方 armed=true：同一个 open 生命周期不再 pick 第二次。
		}
		m.dirPicker.nativeBusy = true
		m.mu.Unlock()
		m.inv.Invalidate()
		go func() {
			defer m.finishNativeDirectoryPick()
			m.pickNativeDirectory()
		}()
		return true
	}
	return false
}

// openBrowseDirectoryPicker 从 kernel 里找到目录浏览器插件并打开它（browse 组合）。
//
// 为什么遍历而不是让壳持有实例：壳持有的实例（`side`/`settings`/`statusbar`）
// 都是为了**装配顺序**或**无头截图**才持的（见 main.go 里那几个字段的注释）。
// 目录选择器两者都不需要，所以按能力查表更稳——插件没装时这里返回 false，
// 侧栏那个按钮就不会画（`Actions.OpenDirectoryPicker` 为 nil 也是这个语义）。
func (m *model) openBrowseDirectoryPicker() bool {
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

// finishNativeDirectoryPick 在 pick 的三种结局（选中/取消/失败）后统一 re-arm。
// 没有 finally 的 Go 版本必须走 defer，否则一次宿主错误会让入口永久锁死。
func (m *model) finishNativeDirectoryPick() {
	m.mu.Lock()
	m.dirPicker.nativeBusy = false
	m.mu.Unlock()
	m.inv.Invalidate()
}

// pickNativeDirectory 走 native 组合：真弹宿主的 OS 选择器，选到就采用它。
//
// 官方语义逐条对得上（`OFF:dpn/src/client/flow.ts:56-72`）：
//
//	path !== null ⇒ createWorkspace → onPick(workspace.workspaceId)
//	path === null ⇒ onCancel()       —— 操作员取消，**什么都不做、不报错**
//	rejected      ⇒ onError(message) —— 宿主原文进状态栏，不另编文案
//
// 它是同步的（`openDirectoryPicker` 用 goroutine 包着调）：测试直接调它，
// 断言就在同一线程落定，不必等 goroutine。
func (m *model) pickNativeDirectory() {
	if m == nil || m.cli == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), dirPickerPickTimeout)
	defer cancel()
	path, picked, err := m.cli.DirectoryPick(ctx)
	if err != nil {
		// 官方 `onError(reason)`：把宿主/传输的原文交给用户，不翻译。
		m.setStatus("目录选择器失败: " + err.Error())
		return
	}
	if !picked {
		return // 官方 null = 取消：不是错误，不打扰用户。
	}
	workspace, err := m.cli.WorkspaceCreate(ctx, path)
	if err != nil {
		m.setStatus("打开目录失败: " + err.Error())
		return
	}
	if m.dirPicker.onPicked != nil {
		m.dirPicker.onPicked(workspace)
	}
}

// errUnknownDirPickerKind 是"探针成功但给出一个不认识的 kind"的报错。
func errUnknownDirPickerKind(kind apiclient.DirectoryPickerKind) error {
	return &unknownDirPickerKindError{kind: kind}
}

type unknownDirPickerKindError struct{ kind apiclient.DirectoryPickerKind }

func (e *unknownDirPickerKindError) Error() string {
	return "探到了一个不认识的 directoryPicker 组合: " + e.kind.String()
}
