package pluginapi

// context_menu.go —— 官方 `dsh-client-ui-context-menu` 的公共契约（rc.2）。
//
// 官方那份包是**纯 UI 基础设施**，不是独立可见面：
//
//	OFF:ui-context-menu/src/client/index.ts:98-134   ctx.contextMenus 服务
//	OFF:ui-context-menu/src/client/index.ts:135-145 contextMenu.button 工厂槽
//	OFF:ui-context-menu/src/client/index.ts:170-181 shell.overlay 上的 MenuOverlay
//
// 官方给业务两条腿：
//   - `getTriggerProps(target)`：把右键（以及键盘 Shift+F10）绑到一个有身份的元素上；
//   - `contextMenu.button` 工厂：在行尾画省略号按钮，**只有该 target kind 有
//     已注册贡献者时才出现**（index.ts:37-40 的注释原文）。
//
// 本仓是 Go+Viva：右键/锚点菜单由 dscontextmenu 按官方 primitives CSS 用 Overlay
// 渲染；贡献仍走同一套「按 target kind 注册、按 order 排序、按 id 去重」的注册表。
// 这样 `contextMenu.button` 的语义（空 kind 不画按钮）由服务本身保证，而不是
// 每个消费方各自猜一遍。

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// KeyContextMenus 是 context menu 服务键。值是 ContextMenus。
const KeyContextMenus = "ui.contextMenus"

// target kind 全集（官方 `ContextMenuTargetMap`，OFF:ui-context-menu/src/client/types.ts:29-40）。
//
// 官方那张表是**可扩展**的（`External plugins extend this map with their own
// target families`，types.ts:26-28），rc.2 自带六个官方 kind，另有插件加的
// `workspace`（OFF:ui-open-in-app/src/client/context-menus.tsx:74-75）。本仓认这
// 七个名字：
//
//	session    会话行（dssidebar）
//	workspace  工作区组头（dssidebar）——官方由 open-in-app 扩展出来的族
//	file       工作区里的**文件**（dsfiles 的文件行；贡献者 ds-fileactions）
//	image      一张图片（贡献者 ds-fileactions）
//	directory / url / tab  官方有、本仓暂无生产方（见 docs/official-file-actions-evidence-rc2.md §4）
//
// 缺 target 的生产方时对应贡献**永远不会被求值**（`HasTarget` 为假 → 菜单不弹），
// 所以"声明了但没人挂"是可见的空白，不是静默错误。
const (
	// TargetSession 是官方 `session`（types.ts:29）：{sessionId, displayTitle}。
	TargetSession = "session"
	// TargetWorkspace 是官方 open-in-app 扩展的 `workspace`：{path}。
	TargetWorkspace = "workspace"
	// TargetFile 是官方 `file`（types.ts:30）：{sessionId, path, line?, paneId?}。
	TargetFile = "file"
	// TargetImage 是官方 `image`（types.ts:33-39）：{sessionId?, url, name?, mediaType?, path?}。
	TargetImage = "image"
)

// ContextMenuTarget 是**被点中的那个对象**（官方 ContextMenuTarget 的 Go 投影）。
//
// 官方的 target 是 discriminated union（session/file/directory/url/image/tab +
// 插件扩展的 workspace），所以这里用一个带 Kind 的平面结构：Kind 是判别键，
// 其余字段按 Kind 解释（每个字段的注释写着它是官方哪个 kind 的哪个字段）。
// 新增 target kind 时**不改接口形状**，只加字段并登记唯一 id。
//
// 一条硬规矩（官方同上）：菜单打开时**捕获** target，动作执行时只能用捕获值，
// 不许回头去读"当前选中"——菜单打开后用户可能已经切走。
type ContextMenuTarget struct {
	// Kind 是目标类别，取 TargetSession / TargetWorkspace / TargetFile / TargetImage。
	// 空 = 没有目标。
	Kind string
	// ID 是业务身份（工作区 id / 会话 id）。菜单动作执行时必须用它，
	// 不能执行时再去读"当前选中"——菜单打开后用户可能已经切走。
	//
	// file/image target **不用**它：官方那两个族的身份是 (SessionID, Path) 与 URL，
	// 见下面各自的字段。
	ID string
	// DisplayTitle 是官方 `displayTitle`：菜单/对话框文案里显示的名字。
	DisplayTitle string
	// Path 是官方 `path`：
	//   workspace → 工作区目录（绝对路径）
	//   file      → 工作区里的**相对路径**（官方 `workspaceFiles` 的 path 参数）
	//   image     → 图片文件的工作区相对路径（可选；只有落盘图片才有）
	Path string
	// SessionID 是官方 file/directory/url 的必填 `sessionId`、image 的可选 `sessionId`。
	//
	// 它不是"当前会话"的副本：动作要用它去调 `workspaceFiles.readBytes` 这类
	// 按会话解析工作区根的方法，拿"现在选中的会话"会读错工作区。
	SessionID string
	// Line 是官方 file target 的可选 `line`（1 基行号，用于"在侧栏打开并定位"）。
	// 0 = 没给（官方是 `line?: number`，缺省不是 0 行）。
	Line int
	// URL 是官方 image target 的必填 `url`（原始内容的 URL）。
	URL string
	// Name 是官方 image target 的可选 `name`（建议文件名）。
	Name string
	// MediaType 是官方 image target 的可选 `mediaType`（原始 MIME）。
	MediaType string
}

// ContextMenuAction 是一条菜单动作（官方 MenuAction 的 Go 投影）。
type ContextMenuAction struct {
	// ID 在一次 resolution 内唯一；ID 会参与去重，重复即装配错误。
	ID string
	// Label 是已本地化的文案（调用方给中文，服务不查字典）。
	Label string
	// Danger 画成破坏性颜色（官方 `.destructive`）。
	Danger bool
	// Disabled 置灰且不可选择（官方 `disabled`）。
	Disabled bool
	// Icon 是官方 `contextMenu.icon` 的 key（如 workspace.rename）。
	// 空 = 官方这一条本来没有图标，不用自造图形补位。
	Icon string
	// SeparatorBefore 在本条前插一条分隔线（官方 `separatorBefore`）。
	SeparatorBefore bool
	// Run 执行动作。返回后菜单关闭。nil = 只读展示项。
	Run func()
}

// ContextMenuContribution 是一个业务对某类 target 的贡献（官方 MenuContribution）。
type ContextMenuContribution struct {
	// ID 在同一个 target kind 内唯一（官方 `Duplicate menu contribution`）。
	ID string
	// Target 是 target kind（TargetWorkspace / TargetSession / TargetFile / TargetImage）。
	Target string
	// Order 小的先出现（官方按 order 再按 id 排序）。
	Order int
	// Resolve 在菜单打开时求值；返回空 = 该 target 当前没有可用动作
	// （官方允许贡献者"拒绝"）。
	Resolve func(ContextMenuTarget) []ContextMenuAction
}

// ContextMenus 是 context menu 服务。
type ContextMenus interface {
	// RegisterContribution 登记一个贡献；返回反注册函数。同一 target 内重复 id 返回错误。
	RegisterContribution(contribution ContextMenuContribution) (unregister func(), err error)
	// HasTarget 报告某 target kind 是否有贡献者。`contextMenu.button` 用它决定
	// 画不画（官方「没有贡献者就没有按钮」）。
	HasTarget(kind string) bool
	// Attach 把右键菜单绑到一个已有身份的元素上。id 必须与 target 的 Hit 身份
	// 一致（Viva 的 Box 不导出该身份）；target 必须是本帧画出来的盒子。
	Attach(c ui.C, id gfx.ID, target ui.Box, obj ContextMenuTarget)
	// Button 画官方 `contextMenu.button` 的等价物：20×20 省略号按钮，
	// 只有 HasTarget(obj.Kind) 才画。id 是调用方用 env.Tree.ID 造的身份。
	// 返回 true = 本帧画了按钮。
	Button(c ui.C, id gfx.ID, obj ContextMenuTarget, label string) bool
}
