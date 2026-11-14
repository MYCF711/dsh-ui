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
// 本仓是 Go+Viva：右键/锚点菜单由 viva 的 `ui.ContextMenu` / `ui.Menu` 渲染，
// 贡献仍走同一套「按 target kind 注册、按 order 排序、按 id 去重」的注册表。
// 这样 `contextMenu.button` 的语义（空 kind 不画按钮）由服务本身保证，而不是
// 每个消费方各自猜一遍。

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// KeyContextMenus 是 context menu 服务键。值是 ContextMenus。
const KeyContextMenus = "ui.contextMenus"

// ContextMenuTarget 是**被点中的那个对象**（官方 ContextMenuTarget 的 Go 投影）。
//
// 官方的 target 是 discriminated union（session/file/directory/url/image/tab），
// 本仓当前落地的业务只有 workspace/session 两类，所以这里用一个带 Kind 的平面
// 结构：Kind 是判别键，ID 是业务身份，其余字段按 Kind 解释。新增 target kind
// 时**不改接口形状**，只加字段并登记唯一 id。
type ContextMenuTarget struct {
	// Kind 是目标类别，例如 "workspace" / "session"。空 = 没有目标。
	Kind string
	// ID 是业务身份（工作区 id / 会话 id）。菜单动作执行时必须用它，
	// 不能执行时再去读"当前选中"——菜单打开后用户可能已经切走。
	ID string
	// DisplayTitle 是官方 `displayTitle`：菜单/对话框文案里显示的名字。
	DisplayTitle string
	// Path 是官方 workspace target 的 `path`（工作区目录）。仅 workspace 使用。
	Path string
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
	// SeparatorBefore 在本条前插一条分隔线（官方 `separatorBefore`）。
	SeparatorBefore bool
	// Run 执行动作。返回后菜单关闭。nil = 只读展示项。
	Run func()
}

// ContextMenuContribution 是一个业务对某类 target 的贡献（官方 MenuContribution）。
type ContextMenuContribution struct {
	// ID 在同一个 target kind 内唯一（官方 `Duplicate menu contribution`）。
	ID string
	// Target 是 target kind（"workspace" / "session"）。
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
	// Attach 把右键菜单绑到一个已有身份的元素上。target 必须是本帧画出来的、
	// 带 Hit 身份的盒子（官方 getTriggerProps 的等价物）。
	Attach(c ui.C, target ui.Box, obj ContextMenuTarget)
	// Button 画官方 `contextMenu.button` 的等价物：20×20 省略号按钮，
	// 只有 HasTarget(obj.Kind) 才画。id 是调用方用 env.Tree.ID 造的身份。
	// 返回 true = 本帧画了按钮。
	Button(c ui.C, id gfx.ID, obj ContextMenuTarget, label string) bool
}
