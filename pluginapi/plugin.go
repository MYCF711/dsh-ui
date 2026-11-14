// Package pluginapi（公开版）是界面线的插件契约：全部接口 + 共享数据类型，**零实现**。
//
// 分工照抄宿主线（DSH-GO）的做法，别自创第二套：
//
//	宿主线                     界面线
//	internal/spi   （契约）  →  internal/pluginapi （本包）
//	internal/kernel（运行时）→  internal/uikernel  （作用域 / 注册表 / 装配）
//	modules/*      （插件）  →  plugins/*          （插件）
//
// 本包的红线（与宿主线 R1 / R2 同源）：
//
//	① 只 import viva 的 ui 与 flex，**不** import components / markdown：
//	   契约不许绑定某一种绘制实现，否则换渲染器就得改所有插件。
//	② 插件不许 import cmd/app：拿不到窗口、拿不到 model 单例，
//	   只能通过 Env 里给的快照和 Action 做事。这就是宿主线的 R2。
//	③ 本包不 import 任何 plugins/*：依赖方向永远是 插件 → 契约。
//
// 「一个部件一个部件移植」的落地方式：每个官方组件（侧栏 / 会话流 / 输入框 /
// 状态栏 …）各成一个 plugins/<id>，声明自己挂在哪个 Slot 上、依赖哪些服务键。
// 先装根本依赖插件（ds-core：设计令牌 + 基础组件），它的 Provides 就是后面
// 所有插件的 Requires。
package pluginapi

import (
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 服务键 ────────────────────────────────────────────────────────────────
//
// 服务键是插件之间**唯一**的耦合方式（对齐宿主线 kernel.Scope 的 Provide/Inject）。
// 键名用 "<域>.<东西>"，全局唯一，写死在这里——散落在各插件里就会拼错。
const (
	// KeyTheme 由根本依赖插件 ds-core 提供，值是 *Theme。
	// 所有画东西的插件都必须 Requires 它，这样装配器保证它先就位。
	KeyTheme = "ui.theme"
	// KeyPrimitives 由 ds-core 提供，值是 *Primitives（基础组件的画法）。
	KeyPrimitives = "ui.primitives"
)

// Source 提供一批插件。
//
// 为什么要它：壳不该知道"装了哪些插件"。文件系统扫描、配置读取、外部 module、
// 甚至测试替身，都实现这个接口——装配起来时是一次性的 []Plugin，
// 装配之后没有任何区别。第三方插件因此不需要改壳的代码。
type Source interface {
	// Plugins 返回这一源能提供的全部插件。返回错误 = 这个源失败了：
	// 内置源的失败是致命的，外部源的失败按调用方策略决定是警告还是退出。
	Plugins() ([]Plugin, error)
}

// SourceFunc 让一个函数满足 Source。
type SourceFunc func() ([]Plugin, error)

// Plugins 实现 Source。
func (f SourceFunc) Plugins() ([]Plugin, error) { return f() }

// Plugin 是一个界面插件。字段形状刻意和宿主线的 kernel.Module 对齐。
type Plugin interface {
	// ID 全局唯一、kebab-case、稳定（写进日志和依赖表）。
	ID() string
	// Provides 是本插件注册后能提供的能力（服务键）。
	Provides() []string
	// Requires 是本插件开工前必须已经就位的能力。缺一个就装配失败，
	// 不许静默降级——「插件少画了一块」是最难查的 bug。
	Requires() []string
	// Register 在窗口起来之前调用一次。这里只做注册，不许画东西。
	Register(ctx Context) error
}

// Fields 是「要接键盘输入的插件」的可选接口：壳会把这些 Field 交给 viva 的
// TextInput。不实现就等于这个插件收不到任何按键（搜索框、输入框都必须实现）。
type Fields interface {
	Fields() []*ui.Field
}

// KeyHandler 是「想接键盘的插件」的可选接口。
//
// 壳把按键按装配顺序依次交给插件，**第一个返回 true 的赢**，壳自己不认识任何具体快捷键——
// 这样「↑ 切会话」是侧栏插件的能力，不是壳里写死的 if。
type KeyHandler interface {
	HandleKey(ev gfx.KeyEvent) bool
}

// ScrollHandler 让拥有内部横滚视口的部件先处理自己的手势；返回 Handled=false
// 时壳继续走 View.Wheel，因此代码块的普通纵向滚动仍归消息区。
type ScrollHandler interface {
	HandleScroll(ev gfx.ScrollEvent) gfx.EventResult
}

// Panels 是「会画东西的插件」的可选接口。
// 一个插件可以不实现它（纯服务提供者，比如 ds-core）。
type Panels interface {
	Panels() []Painter
}

// Context 是插件在注册期唯一能碰到的东西。
//
// 故意做成接口而不是 uikernel.Scope：否则 uikernel 要 import pluginapi，
// pluginapi 又要 import uikernel，成环。宿主线用同样的办法绕开了
// （kernel.Module.Register 收的是 *Scope，而 spi 里根本提不到 Scope）。
type Context interface {
	// Name 是这个插件的作用域名（一般等于 ID）。
	Name() string
	// Provide 登记一个能力。同一个键被两个插件提供是装配错误。
	Provide(key string, value any)
	// Inject 取一个能力；没提供过返回 nil。
	Inject(key string) any
	// Effect 登记一个要在卸载时执行的清理函数（goroutine / 订阅 / 定时器）。
	Effect(fn func())
	// Logf 打一行日志，前缀已是插件 ID。
	Logf(format string, args ...any)
}

// Slot 是界面上的一个可挂载区域。
//
// 槽位名写死：插件挂在哪一块由壳决定，插件不许自己算坐标。
// 这样「换个布局」只是壳的事，插件一行不用改。
type Slot string

const (
	SlotSidebarHeader Slot = "sidebar.header"
	SlotSidebarList   Slot = "sidebar.list"
	SlotSidebarFooter Slot = "sidebar.footer"
	SlotMainHeader    Slot = "main.header"
	SlotMainBody      Slot = "main.body"
	SlotMainComposer  Slot = "main.composer"
	// ── 框架层（L3 ds-shell 提供）──
	// 这三个是官方的 shell.* 槽：跨三列的整行、左上角座、覆盖整个框架的浮层。
	SlotShellRoot    Slot = "shell.root"
	SlotShellLeading Slot = "shell.leading"
	SlotShellBottom  Slot = "shell.bottom"
	SlotShellOverlay Slot = "shell.overlay"

	SlotRightbar  Slot = "rightbar"
	SlotStatusbar Slot = "statusbar"

	// ── 官方 rc.2 的**选择器座位**（keyed / list）──────────────────────────
	//
	// 这四个是「一个槽装多个 view / tab」的官方座位，也是插件唯一的注册入口：
	//
	//	SlotRightbarSession    官方 rightbar 的每会话面板座（single/session）
	//	SlotRightPaneTab       官方右栏 tab **体**（keyed/session，key = 类型 id）
	//	SlotRightPaneTabTitle  官方右栏 tab **标题**（keyed/session，同一个 key）
	//	SlotConvView           官方会话 view（list/session，一次只渲染一个）
	//
	// 形状与锚点：
	//
	//	OFF:dsh-client-ui-sidebar-right/src/client/contract/slots.ts:43（rightbar.session）
	//	OFF:dsh-client-ui-sidebar-right/src/client/contract/slots.ts:59（pane tab）
	//	OFF:dsh-client-ui-sidebar-right/src/client/contract/slots.ts:73（pane tab title）
	//	OFF:dsh-client-ui-conversation/src/client/contract/slots.ts:285（conversation.view）
	//
	// 注意 `SlotConvView` 与 `SlotMainBody` 是**互斥**的两条路径（同一帧只有一个活）：
	// 官方骨架走前者，dsshell 三栏的旧部件兼容桥走后者（见 cmd/app/shell.go 的
	// BridgeLegacy 注释）。插件两边都挂、各自判「这一帧有没有活」，不自己猜壳是哪个档。
	SlotRightbarSession   Slot = "rightbar.session"
	SlotRightPaneTab      Slot = "sidebar.right.pane.tab"
	SlotRightPaneTabTitle Slot = "sidebar.right.pane.tab.title"
	SlotConvView          Slot = "conversation.view"
)

// 官方已注册的**会话视图 id**（各自包注册 `conversation.view` 时给的 `id`）。
//
// 它们是官方 package 的事实，不是我们编的名字；插件注册时用这些常量当
// `Painter.Key`，壳据此投影名录、按 key 派发。
const (
	// ViewChat 是 ui-chat 的固定主视图。
	// OFF:dsh-client-ui-chat/src/client/apply.ts:239（`id: 'chat'`、`order: 0`）
	ViewChat = "chat"
	// ViewTrajectory 是 ui-trajectory 的轨迹视图。
	// OFF:dsh-client-ui-trajectory/src/client/index.ts:82（`id: 'trajectory'`、`order: 10`）
	ViewTrajectory = "trajectory"
	// ViewStats 是 ui-stats 的性能统计视图。
	// OFF:dsh-experimental-client-ui-stats/src/client/index.ts:77（`id: 'stats'`、`order: 40`）
	ViewStats = "stats"
)

// Painter 是一块可挂载的绘制单元。
type Painter struct {
	// ID 在同一个插件内唯一，用于日志和调试（不是交互 id）。
	ID string
	// Slot 挂在哪。
	Slot Slot
	// Order 决定同一个槽里谁先画：小的先（= 贴在上面）。
	//
	// 对 **keyed 槽**（`sidebar.right.pane.tab` / `conversation.view`）它还是
	// **注册表顺序**：官方 view tab 行的排法就是注册顺序（ui-conversation 的
	// `ViewTab{id,label}` 由注册顺序投影出来，contract/views.ts:4-8），
	// 右栏 tab 类型同理（tab-registry.ts 的 `order`）。
	Order int
	// Key 是 **keyed 槽的 key**，非 keyed 槽留空。
	//
	// 官方把槽分成 single / list / keyed / chain / hoc / factory 六种，
	// 其中 keyed 是「同一个槽按 key 取一份」：
	//
	//	sidebar.right.pane.tab  key = tab **类型**的 id
	//	  OFF:dsh-client-ui-sidebar-right/src/client/contract/slots.ts:59-64
	//	  「dispatched with the `id` of the type in force for `tab.kind`」
	//	conversation.view       key = view id（'chat' / 'trajectory' / 'stats'）
	//	  OFF:dsh-client-ui-conversation/src/client/contract/slots.ts:285（list 槽）
	//	  + contract/views.ts:8 的 `ViewTab{id,label}`，投影自注册选项
	//
	// 本仓的 keyed 槽**只画 key 命中的那一份**（`Kernel.DrawSlotKeyed`），
	// 不再「把挂在这一槽上的全都画一遍」——后者是 rc.2 之前的行为，
	// 会让两个 view 同时画进同一块矩形。
	Key string
	// Label 是 keyed 槽在选择器上显示的名字：会话 view tab 的芯片文案、
	// 右栏 tab 的芯片文案。官方两者都是 `label` 取值（`ViewTab.label`），
	// 右栏侧是 tab 类型的 `title`（tab-registry.ts 的 `title: () => string`）。
	// 非选择器上的画留空。
	Label string
	// Icon 是芯片前的字形；官方 tab 类型可以带图标，没有就是纯文字。
	Icon string
	// Draw 画自己。frame 已经是这块槽的位置和尺寸，别自己算全局坐标。
	Draw func(f ui.C, env *Env)
	// AfterDraw 在整棵树布局完成之后调用，给那些「必须先知道位置才能画」的东西用：
	// MarkdownView.Paint、自绘文本排版、代码块高亮。canvas 是全屏画布，
	// 坐标要自己从 Draw 阶段量到的矩形来（拿 Box.Rect() 存起来）。
	//
	// 为什么需要它：viva 的绘制顺序不受调用方控制，在 Draw 里画一条线会被
	// 后面的节点覆盖（踩过）。所以分两个阶段：先布局（Draw），再覆画（AfterDraw）。
	AfterDraw func(canvas *gfx.Canvas, env *Env)
	// Children 让一个槽位在画完自己之后，再把 f 交给下一层槽位。
	//
	// 这就是"高塔"的做法：壳只画 root 一个槽，框架插件在 root 里切出
	// shell.leading / serveral 个列 / shell.bottom，再把 f 交给
	// sidebar.* / main.* / rightbar。插件不需要知道自己在第几层。
	Children func(f ui.C, env *Env, drawSlot func(Slot))
}

// Primitives 是基础组件的画法，由根本依赖插件（ds-core）提供。
//
// 做成**接口**而不是具体类型，是为了让下游插件只认契约、不认 dscore 这个包：
// 哪天要换一套视觉实现（浅色变体、紧凑变体），下游一行都不用改。
// 实现方必须自己加 `var _ pluginapi.Primitives = (*X)(nil)` 断言。
type Primitives interface {
	// ListRow 一条列表行：选中/悬停底色由实现决定，调用方只给状态。
	ListRow(f ui.C, id gfx.ID, active bool, height float32, body func()) ui.Box
	// SectionHeader 一个分组头（标题 + 条数 + 展开/收起箭头）。
	SectionHeader(f ui.C, id gfx.ID, title string, count int, open bool) ui.Box
	// Badge 行尾的小徽标。
	Badge(f ui.C, text string, color ui.Color) ui.Box
	// Dot 一个状态圆点。
	Dot(f ui.C, color ui.Color, size float32) ui.Box
	// Divider 一条最轻的分隔线。
	Divider(f ui.C)
	// CircleButton 一个 28px 圆形图标按钮（官方 .action 的尺寸），返回是否被点击。
	CircleButton(f ui.C, id gfx.ID, glyph string, filled bool) bool
	// Chip 一个只读小标签（官方的 .chip：inline-flex、gap 3px、business-primary 文字）。
	Chip(f ui.C, label string) ui.Box
	// Bubble 画一个用户气泡容器（官方 .bubble：radius xl、specific-bubble 底）。
	// 调用方传进内容，气泡只管外观。
	Bubble(f ui.C, body func()) ui.Box
}

// Tab 是一条注册出来的**选择器条目**：会话 view tab 或右栏 tab。
//
// 它对应官方两个地方的同一种东西，合成一个形状（两边字段一一对得上）：
//
//	conversation.view   `ViewTab{id, label}`（OFF:dsh-client-ui-conversation/src/client/contract/views.ts:8）
//	                    —— 由 `conversation.view` 槽条目的注册选项投影出来
//	                    注册例子：OFF:dsh-client-ui-chat/src/client/apply.ts:237-242（id 'chat'、order 0）
//	                              OFF:dsh-client-ui-trajectory/src/client/index.ts:80-86（id 'trajectory'、order 10）
//	sidebar.right.pane.tab  tab 类型的 `{id, kind, title}`
//	                    （OFF:dsh-client-ui-sidebar-right/src/client/tab-registry.ts:106-…）
//	                    注册例子：OFF:dsh-client-ui-sidebar-terminal/src/client/index.ts:72-75（kind 'terminal'）
//
// 壳把内核里的 keyed painter 投影成这张表（`uikernel.ViewTabs` / `RightTabs`），
// 插件只负责注册，不查表、不排序、不算激活。
type Tab struct {
	// Key 是 keyed 槽的 key = 官方那条条目的 `id`。
	Key string
	// Label 是芯片上的文案；官方是 thunk（`label: () => t('view.chat')`），
	// 本仓按当前语言在注册处取好。
	Label string
	// Icon 是可选的字形，没有就是纯文字。
	Icon string
	// Order 是注册顺序（官方 `order`；小的在前）。
	Order int
}

// Env 是绘制期拿到的全部东西：只读快照 + 动作 + 主题。
//
// 明确**不给** model / 互斥锁 / 客户端：插件不该知道数据怎么来的，
// 也就不会有人偷偷在 Paint 里发请求。
type Env struct {
	Theme *Theme
	Data  Snapshot
	Act   Actions
	// Tree / Feat 是这个槽所属的交互树与 feature 作用域。
	// 插件自己用 env.Tree.ID(env.Feat, "名字") 造交互 id——不许写死数字。
	Tree *gfx.Tree
	Feat gfx.Scope
	// Canvas 只在 Draw 阶段有效：Markdown 排版这类必须先量后画的东西要它。
	// （AfterDraw 收到的 canvas 是布局后的全屏画布，两者是同一个对象。）
	Canvas *gfx.Canvas
	// ScrollBox 把本帧的会话滚动区交给壳：插件声明完消息流滚动区后调用一次。
	//
	// 为什么不是"壳按 ID 查状态"：viva v0.6.0 删掉了 `ScrollState(id)` 这种按身份
	// 查表的公开入口（滚动状态由**声明方**持有，`Box.ScrollState()` 且只在同一个
	// paint 内有效）。壳要在布局之后存/恢复偏移，只能由声明方把盒子交回来，
	// 壳在 `Draw` 返回后立刻用完就地丢弃（不跨帧保存 Box）。
	ScrollBox func(box ui.Box)
	// ViewportOffset 覆盖本帧视口虚拟化用的滚动偏移（非 nil 时用它，而不是真实偏移）。
	// 给"主区被另一个视图接管、正文不该生成文本"的场景用（dsrightbar 的轨迹视图）：
	// 偏移给一个远大于内容高度的值，消息就全部退化成 Slot。
	// 生命周期只在本帧的 painter 链内：写者负责恢复原值。
	ViewportOffset *float32
	// Has 判断某个槽位有没有插件挂东西。框架插件靠它决定"这一列画不画"——
	// 没有侧栏插件时不该留一条空栏。
	Has func(Slot) bool
	// Views 是本帧注册的**会话视图**名录（官方 `conversation.view` 的 list 槽投影）。
	//
	// 由壳从内核的 painter 表投影出来（`uikernel.ViewTabs`），已按 (Order, 装配顺序)
	// 排好、同 key 去重。空 = 一个视图都没注册（那时不画 view tab 行）。
	Views []Tab
	// ActiveView 是本帧选中的会话视图 key；空 = 还没解析出来，主区不画 view。
	//
	// 官方两个判据都在这上面（OFF:dsh-client-ui-conversation/src/client/view-selection.ts:15-21）：
	// 存着的偏好命中名录就用它，否则落 `chat`，再没有就是「没有可画的 view」。
	ActiveView string
	// RightTabs 是本帧注册的**右栏 tab 类型**名录（官方 `sidebar.right.pane.tab` 的 keyed 槽投影）。
	RightTabs []Tab
	// ActiveRightTab 是本帧选中的右栏 tab 类型 key；空 = 没有可画的 tab。
	ActiveRightTab string
	// DrawKeyed 把一个 **keyed 子槽**按 key 交回壳去画：只画 key 命中的那几份。
	//
	// 为什么要有它：`Painter.Children` 拿到的 `drawSlot` 只带槽名，按官方 keyed 语义
	// 派发必须带 key；而改 `Children` 的签名等于改全仓每一块画。壳在装配 Env 时把它
	// 接到 `Kernel.DrawSlotKeyed` 上，插件因此不需要认识 kernel。
	//
	// **f 必须传调用点上那个**：viva 的 `ui.C` 是布局游标的句柄，子槽要落在
	// 调用点所在的容器里。传根 frame 会把子槽画成根的同级兄弟（画得出来，位置全错）。
	// `Painter.Children` 拿到的 `drawSlot` 就是同一个道理——它是按当前 f 现造的闭包。
	//
	// key 传空串时按「这个槽上什么都没命中」处理（不画），不是「全画一遍」。
	DrawKeyed func(f ui.C, slot Slot, key string)
	// Geometry 是本帧三栏的几何（壳算好，框架插件照着摆）。
	// 为什么壳算：它已经知道侧栏插件要的宽度、右栏开合、白名单里有没有这些部件。
	// 框架插件拿到的是"结果"，不是"猜"。
	Geometry Geometry
	// MainWidth 是主区那一列的内容宽度（壳按 窗口 - 侧栏 - 右栏 算好）。
	// 插件要按像素定宽时只能用它：槽位里 f.Bounds() 给的是整窗宽（实测 1180）。
	MainWidth float32
	// Bounds 是这一帧的整窗尺寸。AfterDraw 拿到的 canvas 没有尺寸接口，
	// 要在覆画时算裁剪区就用这个。
	Bounds ui.Rect
}

// Geometry 是一帧的三栏几何（像素，已扣掉 padding 之前的外框宽）。
type Geometry struct {
	Width        float32 // 整窗
	Height       float32
	SidebarWidth float32 // 0 = 没有侧栏
	RightWidth   float32 // 0 = 没有右栏
	MainWidth    float32 // 剩下的全部给中栏
	BottomHeight float32 // 0 = 没有底部整行
}

// Model 是可选模型的一行（宿主 `models.list` 的投影）。
//
// 用 `OwnedBy` 按渠道分组——同一个网关后面确实挂着多个 provider
// （wps-lingxi / workbuddy / streamlake / monkeycodeintl …），
// 只列 id 的话几十条挤在一起，用户分不清哪个是自己要的。
type Model struct {
	ID      string
	OwnedBy string
}

// Job 是后台任务的一行。
//
// **它是一份快照，不是活句柄**：宿主那边的数据由子进程推上来，
// 这里拿到的是"上一次推的时候"的样子。所以 Done 为 false **不代表此刻还在跑**
// （可能刚结束，还没推）。
//
// 输出与停止是**另外两条真实动作**：`JobsOutput`（读一页输出）与
// `JobsStop`（请求停止）。它们都必须带 SessionID——宿主拒绝"跨会话猜任务"
// （`HOST:internal/transport/handlers_jobs.go:109-112`），所以这一格不许自己编，
// 空串就表示"不知道归属，别画能点开/能停的控件"。
type Job struct {
	ID        string
	SessionID string
	Command   string
	Done      bool
	ExitCode  int
	Bytes     int
}

// JobOutput 是 `JobsOutput` 准备好的一页（或**累计**）输出。
//
// 字段语义与宿主 `transport.JobOutput` 一一对应：
//   - Cursor/NextCursor 的单位是 **Unicode code point**，不是字节；
//   - HasMore 表示还有下一页（继续按 NextCursor 要）；
//   - Truncated 表示 1 MiB 保留窗口之前已有输出被淘汰——它与"没有输出"是
//     两件事，界面必须分别说；
//   - Err 非空表示这一页**没读到**（轮次结束、任务 owner 不在、跨会话被拒）：
//     这时 Output 一律为空，不许拿"看起来没有输出"冒充成功。
type JobOutput struct {
	ID         string
	SessionID  string
	Output     string
	Done       bool
	ExitCode   int
	Cursor     int
	NextCursor int
	HasMore    bool
	Total      int
	Truncated  bool
	Err        string
}

// Workspace 是官方 `workspace.registry` 的一条工作区（WorkspaceView 的投影）。
//
// 官方字段见 `HOST:internal/transport/handlers_workspace_official.go`
// 的 `OfficialWorkspace`：workspaceId / path / title / sessionIds / createdAt /
// updatedAt。它是**真实 registry**（宿主 `<home>/state/workspaces.json`）的视图，
// 不是按 cwd 猜出来的分组。
type Workspace struct {
	WorkspaceID string
	Path        string
	Title       string
	SessionIDs  []string
	CreatedAt   string
	UpdatedAt   string
}

// ScheduleItem 是定时任务的一行。
type ScheduleItem struct {
	ID       string
	Title    string
	Kind     string
	NextAt   string
	EverySec int
	DailyAt  string
	WeeklyAt string
}

// Snapshot 是一帧的数据快照，壳在锁内拷好再传进来，插件里不用加锁。
// FSEntry 是 FSList 的一项。
//
// **必须带 IsDir**：只回名字的话，插件分不清"点它该进目录还是该读文件"——
// 点目录会去 fs.read 然后拿到"文件不存在"，界面上看着像读不了。
type FSEntry struct {
	Name  string
	IsDir bool
}

type Snapshot struct {
	Sessions []apiclient.Session
	// Workspaces 是官方 `workspace.registry` 的缓存投影（壳在启动/改名/删除后刷新）。
	//
	// 空 = 宿主没有 registry 或还没拉到：消费方必须退回"按 cwd 分组"的旧路径，
	// 不能把空切片当成"用户没有任何工作区"而画空状态。
	Workspaces []Workspace
	Cur        string
	Search     string
	Status     string
	Msgs       []apiclient.Message
	// Frames 是当前会话 journal 的**原始 Transport 帧**（`session.history` 的
	// `frames[]` 与 WS 推送**同一个形状**），按 seq 升序、已去重。
	//
	// 为什么必须单独一份、不能从 Msgs 反推：宿主契约（`DSH-GO/docs/transport-contract.md`
	// §恢复）写明 `session.history` 返回**原始 frames**，`session.load` 只是首屏投影。
	// 官方主区 transcript 的投影器消费的是 v4 原始事件（`user/message` /
	// `assistant/message` / `tool/call` …），而 `surfaceOp` / `source.kind` /
	// `reason.kind` 这些**只有原始帧才有**——从"抽过 text 的 messages"反推，
	// 这几种节点就再也分不出来了。
	//
	// 空 = 还没有基线（老宿主 / mockd / 首帧尚未拉到）。插件据此回退旧渲染路径，
	// 不许画一个空的消息区。
	Frames []apiclient.Frame
	// JournalEpoch 是 Frames 所属的 journal 世代（`session.load`/`session.history`
	// 的 `journalEpoch`）。0 = 未知。
	//
	// 世代不一致 = 宿主换过 journal（重启），旧 seq 属于**另一个序号域**：
	// 消费方必须丢弃旧基线重建，不能按 seq 硬补——后者不报错也不丢帧，
	// 只是"少了几条"，是这一层最难发现的一种错。
	JournalEpoch int64
	Total        int
	Stream       string
	// Busy 表示当前会话**有一个 turn 在跑**（已发 prompt、还没结束/取消/失败）。
	//
	// 为什么不能只看 Stream：只有思考（reasoning）没有正文时 Stream 是空的，
	// 但 turn 照样在跑——那时按 Esc 应该能停（实测踩过：只判 Stream 就停不了）。
	Busy     bool
	Approval *Approval
	// CWD 是当前会话的工作目录（绝对路径，Windows 风格）。
	//
	// **空串 = 不知道**（老宿主没这个字段）。插件拼文件路径时必须以它为前提：
	// 空着就别拼，否则拼出来的 `/name` 会被 `fs.read` 当"文件不存在"。
	// 多会话可能属于不同工作区，所以它跟着 `Cur` 变，不是全局常量。
	CWD string
	// Disconnected 表示 WS 订阅当前连不上宿主（壳在重连）。
	//
	// 为什么单独一个字段而不是让插件去读 Status 里的字：状态栏那颗绿点写「就绪」
	// 是**连接层**的断言，不该靠人肉匹配中文字符串来判断。默认 false = 正常，
	// 所以没接这个字段的插件行为不变。
	//
	// **它是 `Link == LinkDown` 的旧式投影**：新代码读 `Link`，因为它能表达
	// "壳还没拿到任何连接事实"这一态（`Disconnected == false` 分不出"连着"
	// 和"还不知道"）。两个字段由壳在同一处赋值，不会互相矛盾。
	Disconnected bool
	// Link 是壳对"与宿主的流连接"的**三态真值**。
	//
	// 为什么不能只有 `Disconnected bool`：那个布尔的 false 同时表示
	// "已连接"和"还不知道"，于是首帧 / 老宿主 / 还没有会话（还没订阅）时，
	// 任何据此写「已连接」的界面都在**编一句没有证据的话**。
	//
	// 官方 `ui-settings-general` 的 `SettingsRoot` 由 `useConnectionState` 驱动，
	// 正是这个语义（`connecting` / `disconnected`，其余不画）；本仓没有那条框架
	// 钩子，等价事实由壳在**真实 apiclient 回调**里写（`cmd/app/main.go` 的
	// `runSubscription`：订阅成功 = LinkConnected，订阅失败 = LinkDown，未拿过
	// 任何事实 = LinkUnknown）。
	//
	// 零值 = `LinkUnknown`：**不知道就不许写「已连接」**，写「未知」。
	Link LinkState
	// DesktopUpdate 是壳对"桌面更新"的**事实呈现**（官方 `DesktopUpdateIndicator`
	// 的 `view.presentation`）。
	//
	// nil / Phase==idle = 官方那句 `(state === undefined || state.phase === 'idle')`
	// → **整枚指示器不渲染**（`DesktopUpdateIndicator.tsx:66`）。所以它不是
	// "占位空着"，而是官方在"没有更新这件事"时的正确形状；本仓今天没有桌面
	// 更新通道（宿主没有 updater 方法），这个字段常为 nil，报告里登记为差距。
	//
	// 为什么放在 Snapshot 而不是某个插件自己查：官方由 `settings.launcher` 那一级
	// 父容器注入（`useDesktopUpdate` face），只有壳知道更新器的事实。
	DesktopUpdate *DesktopUpdatePresentation
	// Usage 是当前会话的 token 用量（壳从 session.load 拿到的合计）。
	// Usage.Known() 为 false 时插件不许显示任何 token 数字。
	//
	// **兼容字段**：mockd 只填这一个。真宿主（session.load）改填下面的
	// UsageTotal / ContextTokens；那两个全零时才回退到这份旧数据。
	Usage Usage
	// UsageTotal 是宿主 §12 H1 给的**全会话累计**用量，不随 tail 切页变化。
	//
	// 与 ContextTokens 是**两件事**：累计一直涨，上下文会随压缩回落。
	// 不许相加、不许互相替代。零值 = 不知道，插件要整段不画。
	UsageTotal Usage
	// ContextTokens 是宿主 §12 H1 给的**最近一条** assistant 的 totalTokens，
	// 也就是"当前上下文占多少"。没有正容量时 0 仍按未知处理，不单独显示"上下文 0"。
	ContextTokens int64
	// ContextWindow 是宿主 session.load 给的上下文容量上限；0 = 未知。
	// 只有 ContextWindow>0 且 ContextTokens>=0 时才可计算真实占用百分比；
	// used>capacity 是异常，不得钳成 100%。
	ContextWindow int64
	// SessionStats 是宿主 whole-log `sessionStats` 投影（八字段，官方
	// `dsh-session-stats/lib/types/projection.js:154-167`），状态栏「活动」
	// 读数的**唯一**来源。
	//
	// **nil = 未知**（老宿主 / 还没拉到 / 键缺失 / 查询失败）：那时状态栏写
	// 「活动 未知」，**不许**拿 `Msgs` 窗口数轮/步凑一个看起来像真值的数字
	// （窗口是 tail，既不是全量也不区分 step——官方 ActivityPill 的注释点名
	// 过这份折叠只能由 durable 投影给出）。
	//
	// 非 nil 时八个数全是宿主的真值，包括真实的 0（空会话 = 0 步，那是真话；
	// 显示什么由插件按官方「steps === 0 整枚不渲染」的规则决定）。
	SessionStats *apiclient.SessionStats
	// TokenUsage 是宿主 whole-log `tokenUsage` 投影的四个桶（累计，整数、非负，
	// 官方 `dsh-token-meter/src/usage-projection.ts:45-50`），官方用量胶囊
	// `UsagePill` 与其详情弹窗的**唯一**数据来源。
	//
	// **nil = 未知**（老宿主没有这个键 / 还没拉到 / 查询失败）：那时用量写
	// 「未知」，**不许**拿 `UsageTotal`（宿主自有的人类可读合计）反推四桶——
	// 那是另一种"编"，缓存率的分母根本不成立（`metrics.go` 的
	// `cacheHitPercent` 早就拒绝这种口径）。
	//
	// 非 nil 时四个数全是宿主的真值，**包括真实的 0**：官方 wire 没有
	// `Measured` 位，四桶全 0 = "这份日志没有 provider 回报的用量样本"。
	// 界面按官方 gate 决定画不画：`billed === 0 && output === 0` 时不画弹窗，
	// 但读数仍写成 `0 tok`（宿主说 0），**不是**「未知」。
	TokenUsage *apiclient.TokenUsage
	// Model 是宿主 §12 H4 给的最近一次 request/context 的模型名。
	// 空串 = 不知道；插件不许拿自己的默认值冒充。
	Model string
	// Permission 是宿主 §12 H5 给的这一进程真正的审批策略
	// （deny / ask / allow / auto），**不是会话 mode**。
	// 空串 = 不知道；插件不许拿 mode 顶替。
	Permission string
	// ShowRightbar 是右栏当前是开还是关（壳持有这个状态，插件只读）。
	ShowRightbar bool
	// Truncated 表示这次只渲染了末尾一段（视口虚拟化 / -limit）。
	//
	// **当前无人使用**：壳从不设它、插件也从不读它。真正需要"截断必须让人看得见"
	// 的地方（dsrightbar 的「更早的还有 N 次未显示」）用的是它自己算出来的 hidden
	// 计数。留着是因为将来做视口虚拟化时它是对的位置，但现在**别以为有人在维护它**。
	Truncated bool

	// MessageMeta 按 MessageKey(session,msg,index) 保留 load 原始 time/stopped。
	// 缺 key 或零值都代表未知，消息区不得补 0 时间或虚构 stopped。
	MessageMeta map[string]MessageMetadata
	// TurnEvents 保留已到达的 turn/error 与显式 stopped/interrupted 终局。
	TurnEvents []TurnEvent
	// Outgoing 是当前会话尚未与 durable user event 对账的本地发送及失败草稿。
	Outgoing []OutgoingMessage
	// ActiveTurnID/ActiveTurn 是当前正在归约的真实 Turn 身份。缺真值时保持
	// 零值；消息区只给身份完全匹配的 Turn 页脚标“进行中”，不猜最新一组。
	ActiveTurnID string
	ActiveTurn   *int64

	// History 是主区 transcript 的**向前分页**状态（官方 `hasMore` /
	// `historyWasPartial` / `loadingOlder` / `historyLoadStatus` 四元组）。
	//
	// 为什么必须单独一个结构而不是从 Frames 反推：**"还有更早的没拉"不是
	// "当前有几帧"的函数**——空会话、只有一页的短会话、拉到头的长会话都可能是
	// 同一种帧数，界面画错就是"点了没反应"或者"明明还有却说到头了"。
	//
	// 官方判据（`ChatView.tsx:253-273`）：`(hasMore || historyWasPartial)` 时
	// 才画 `.older` 那一行；按钮 `disabled={!hasMore}`；文案五态由
	// `loadingOlder` / `historyLoadStatus === 'error'` / 其余决定。
	History HistoryPaging
}

// HistoryPaging 是「加载更早」单按钮的**五态输入**（additive）。
//
// 五个状态与官方文案一一对应（`dsh-client-ui-chat/src/client/locale.ts:94-98`）：
//
//	首次（首屏基线还在拉）      → `.hint`「正在加载历史…」        chat.loadingHistory
//	有更多                      → 按钮「加载更早」                chat.loadOlder
//	加载中                      → 按钮「正在加载」（禁用）        chat.loadingOlder
//	无更多                      → 按钮「已到会话开头」（禁用）    chat.historyStart
//	失败                        → 按钮「重新加载」                chat.retryOlder
//
// **零值 = 没有分页信息**（老宿主 / mockd）：界面不画这一行，也不因此把
// transcript 判成"到头了"。
type HistoryPaging struct {
	// Baseline 报告首屏基线是否已经**试过**（成功或失败都算）。
	// false = 第一页还在路上（此时若已经有 load 投影出来的消息，官方画 .hint）。
	Baseline bool
	// HasMore 报告宿主还有更早的帧没拉。false = 已到可加载的最早处。
	HasMore bool
	// Partial 报告**基线是截断的**（首屏那一页之前还有帧）。
	//
	// 官方 `historyWasPartial` 是**粘性**的：它是"这一行该不该留在屏幕上"的判据，
	// 不是"现在还有没有更多"。全部拉完之后它仍然为 true，那一行才会从
	// 「加载更早」变成「已到会话开头」，而不是整行消失（用户会以为坏了）。
	Partial bool
	// Loading 报告正在拉一页（首屏基线或更早一页）。
	Loading bool
	// Error 是最近一次失败的原因（空 = 没有失败）。失败态按钮可点 = 重试。
	Error string
}

// Usage 是 token 用量的界面视图。
//
// 不动 apiclient 的类型是为了让契约包保持"只认 wire 形状"；
// 这一份是给插件用的，多一个 Known() 让"没有数据"和"数据是 0"能分开。
type Usage struct {
	InputTokens     int64
	OutputTokens    int64
	TotalTokens     int64
	CacheReadTokens int64
	TokPerSec       float64
	HasRate         bool
}

// Known 表示宿主真的给过 usage；false 时界面不该画任何 token 数字。
func (u Usage) Known() bool { return u.TotalTokens > 0 || u.InputTokens > 0 || u.OutputTokens > 0 }

// Approval 是一次待审批的工具调用（跨线 §12 H13）。
//
// 每个字段的来源都是 WS `approval/request` 帧的 `data.*`（契约见
// `../DSH-GO/docs/h13-approval.md`），注释里逐个写明是哪个键。
//
// **零值 = 宿主没说**：界面要显示"未知"或整段不画，
// 不许自己补一个看起来合理的值——不编工具名、不编参数、不编 optionId，
// 尤其**不编风险等级**（宿主没有这个字段，界面也不许从命令里算一个出来）。
type Approval struct {
	// RequestID ← `data.requestId`。
	// 回传（`session.approve`）时必须**原样**带回去；界面不许自己造一个。
	// 空串 = 宿主没给 → 这次审批回传不出去，按钮只能画成禁用态。
	RequestID string

	// Tool ← `data.toolCall.title` 里冒号前那一段（宿主 approvalTitle 的约定：
	// "bash: go test ./..." → "bash"；"写入 <路径>" / "修改 <路径>" 这种没有冒号的
	// 前缀不一定是工具名，所以解析不出来就是空串）。
	// 空串 = 不知道用什么名字显示，界面只画"需要审批"，不猜。
	Tool string

	// Title ← `data.toolCall.title`：宿主给的一行摘要（已经是单行）。
	// 空串 = 宿主没给，界面不画这一行。
	Title string

	// Kind ← `data.toolCall.kind`：execute / edit / other。
	// 只用来决定措辞（"要执行"/"要改文件"），**不是风险等级**。
	// 空串 = 不知道，界面按 other 处理（不写"低风险"这类话）。
	Kind string

	// Args ← `data.toolCall.rawInput` 的原文（一般是 JSON，如 {"command":"go test ./..."}）。
	// 宿主给什么就是什么，界面只压成一行、超长截断，**不改写、不美化**。
	// 空串 = 没有参数摘要（title 已经说完了），界面不画参数行。
	Args string

	// Cmd 是**兼容字段**：旧路径（mockd / 本地假审批）只填这一个，代表 bash 的完整命令。
	// 新路径请填 Args；界面在 Args 为空时回退到 Cmd，两者都空就不画参数行。
	Cmd string

	// Detail ← `data.toolCall.content[].content.text`：完整说明。
	// 宿主那边已经把"模型自述（仅供参考）"的标注拼进去了，界面原样显示即可。
	// 空串 = 没有。
	Detail string

	// Reason 是模型自述，**不可信**，界面上必须标出来。
	// 兼容字段：新帧里通常为空（内容已经在 Detail 里）。
	Reason string

	// Options ← `data.options`，按宿主给的顺序。
	// **界面不许自己造 optionId**：回传的每一个 id 都必须来自这里
	// （"允许这一次"和"以后都允许"是两件事，编一个就等于替用户做了决定）。
	// 空 = 宿主没给可选项 → 这次审批点不动。
	Options []ApprovalOption
}

// ApprovalOption 是 `data.options` 里的一项。
type ApprovalOption struct {
	// OptionID ← `data.options[].optionId`（宿主当前给的是 "allow-once" /
	// "allow-always" / "reject-once"）。回传时原样交给 Actions.Approve。
	// 空串 = 这一项不能用。
	OptionID string
	// Name ← `data.options[].name`：宿主给的按钮文案（"允许这一次" / "拒绝"）。
	// 空串 = 用界面的默认文案（"允许" / "拒绝"）。
	Name string
	// Kind ← `data.options[].kind`：allow_once / allow_always / reject_once / reject_always。
	// 界面只按它分"允许 / 拒绝"两类，**不据此判断风险**。
	// 空串 = 不知道是哪一类，这一项不会被选中（不猜）。
	Kind string
}

// Actions 是插件能触发的动作。
//
// 只放「用户点了会改状态」的入口；读状态走 Snapshot。
// nil 的动作表示当前不可用，插件要自己判断（别假设一定有）。
type Actions struct {
	OpenSession func(id string)
	// ForkSession **把当前会话整份复制成一个新分支**，并切过去。
	//
	// 为什么不收 id：和 CompactSession 一样，永远是"对正在看的这个会话做"。
	//
	// ⚠️ 它不是"从某条消息往后分支"：宿主 `serveFork` 没有切点参数，
	// 整份复制。所以界面文案必须说"复制"，不能画在某条消息旁边说"从这里分支"。
	//
	// nil = 壳没接。插件这时不该画入口。
	ForkSession func()

	// CompactSession 压缩**当前**会话：把较早的消息折成一段摘要。
	//
	// 为什么不收 id：这一条永远是"对正在看的这个会话做"，而壳知道是哪个；
	// 让插件传 id 只会多一次"插件手里的 id 过期了"的机会。
	//
	// **它不是删除**：v4 日志不改写，被折进去的只是不再投影到前台
	// （宿主回 `shadowed` 条数）。所以不需要二次确认；但**它会调一次模型**，
	// 误触是有代价的——文案得说清在做什么。
	//
	// nil = 壳没接。插件这时不该画入口。
	CompactSession func()

	// WorkspaceRename 改一个已注册工作区的显示名（宿主 `workspace.rename`）。
	//
	// **同步返回错误**：官方 WorkspaceDialogs 的"重命名"按钮需要把宿主的四字段
	// 错误显示在对话框里（重名/空标题等），不能像后台写那样只写状态栏。
	// 调用方必须在 goroutine 里调，别阻塞绘制线程。
	// nil = 壳没接（老宿主）→ 侧栏不画"重命名"菜单项。
	WorkspaceRename func(workspaceID, title string) error
	// WorkspaceDelete 把一条工作区注册从 registry 移除（宿主 `workspace.delete`）。
	//
	// 语义与官方 `delete.desc` 一致：**只移除工作区列表，不删文件夹、不删会话记录**；
	// 该工作区的会话会落到"未分组"。同步返回错误，调用方在 goroutine 里调。
	// nil = 壳没接 → 不画"删除工作区"菜单项。
	WorkspaceDelete func(workspaceID string) error

	// DeleteSession 删掉一个会话（宿主 `session.delete`，**从磁盘上删**）。
	//
	// 这是**破坏性动作**：壳只负责发请求和报结果，**要不要确认由插件自己决定**——
	// 壳不知道插件当时是把删除做成一次点击还是两次确认。所以插件调用它之前，
	// 必须已经问过用户。
	//
	// nil = 壳没接（老壳）。插件这时不该画删除入口——画一个点了没反应的东西，
	// 比不画更让人以为"删掉了"。
	DeleteSession func(id string)
	NewSession    func()
	// Notify 让插件在状态栏说一句话（"已复制"这类反馈）。
	// 插件不直接写状态栏：状态栏是另一个插件的地盘。
	//
	// **当前零调用**：壳接了线，但没有插件用它——"已复制"这类反馈是各插件
	// 自己调 Copy/Notify 之外的方式给的（看 composer 的复制按钮实现）。
	// 当空转接口看。
	Notify func(msg string)
	// Copy 把文本放进系统剪贴板（插件不 import app，剪贴板归壳管）。
	Copy func(text string)
	// CopyResult 与 Copy 使用同一个真实 OS 动作，但把失败返回给需要原位反馈的
	// CodeBlock；壳同时更新状态栏，调用方不得把 error 当成功。
	CopyResult func(text string) error
	// OpenExternal 只打开明确的 http/https/mailto 目的地。
	OpenExternal func(destination string) error
	// OpenFile 只打开显式 file: 链接指向的文档；普通字符串不得在这里猜成路径。
	OpenFile func(destination string) error
	// DesktopUpdateOpen 是官方 `DesktopUpdateBridge.open()`：用户点了桌面更新指示器。
	//
	// 官方那一下弹的是原生更新面板（检查 / 下载 / 安装）；本仓没有下载器与安装器，
	// 宿主 `desktopUpdate.open` 的诚实等价物是**把更新入口交出来**（一条 release
	// 页或 releases 列表页的 URL），由壳唤起系统默认浏览器打开。
	//
	// 语义（壳的实现必须一致）：
	//
	//	① 置 `Snapshot.DesktopUpdate.Opening = true`（官方 `view.opening`，
	//	   期间指示器 `aria-disabled`、不重复触发）；
	//	② 调宿主 `desktopUpdate.open`，把结果里的 presentation 收进缓存；
	//	③ 用返回的 URL 唤起系统默认浏览器；**打不开要如实提示**（状态栏），
	//	   不许静默失败——用户点了一下什么都没发生，比报错更坏；
	//	④ 失败按官方 `view.failed = true` 记：指示器转成「重试更新」。
	//
	// 幂等：`Opening` 期间重复调用直接返回（官方 `DesktopUpdateSource.open()`
	// 的同一句：`if (state.opening || …) return`）。
	//
	// nil = 壳没接（老壳 / 测试）：插件这时**不把这枚指示器画成按钮**——
	// 画一个点了没反应的东西比不画更坏。
	DesktopUpdateOpen func()
	// ToggleRightbar 收起/展开右栏（官方也能收）。
	ToggleRightbar func()
	// SelectView 把当前会话的主区视图切到名录里的某一条（官方
	// `conversation.session.header.views.actions` 的 `onOpen` / `selectView`，
	// 见 OFF:dsh-client-ui-conversation/src/client/contract/slots.ts:510）。
	//
	// 传一个名录里没有的 key 时**什么都不做**（官方 `resolveActiveView` 也只会
	// 落到 chat，不会创造视图）。nil = 壳没接，调用方不要画可点的页签。
	SelectView func(view string)
	// SelectRightTab 把右栏面板切到名录里的某个 tab 类型。
	//
	// 官方对应 `sidebarRight.openTab(kind)`（OFF:dsh-client-ui-sidebar-right/src/client/index.ts:228）。
	// nil = 壳没接，调用方不要画可点的芯片。
	SelectRightTab func(key string)
	// Retry 重发上一条用户消息。
	Retry func()
	// RetrySend 按本地发送 key 重试同一份失败草稿，不另取最后一条消息冒充。
	RetrySend func(key string)
	// Send 把这段文字发出去。输入框由哪个插件画，文字就由哪个插件给——
	// 壳不再持有草稿，所以不用 SetDraft 回灌。
	Send func(text string)
	// Cancel 打断当前会话正在跑的 turn（H13 的 session.cancel）。
	//
	// nil = 壳没接；插件要自己判，别假设一定有。
	Cancel func()

	// Reconnect **立刻重连**：取消当前 WS 订阅，然后不等退避、马上重新订阅。
	//
	// 它是官方 `ConnectionIndicator` 那个按钮的动作（`SettingsRoot` 的
	// `reconnect` 来自 `ctx.connection.reconnect()`）：warning 态的胶囊是
	// `<button onClick={onReconnect}>`，点了要真的重新连，不是画个假动作。
	//
	// 语义（壳的实现必须一致）：
	//
	//	① 取消当前订阅（旧 goroutine 退出，不再退避等待）；
	//	② 立刻发起一次新订阅，**水位用当前 lastSeq**（不重放已收到的帧）；
	//	③ 这次尝试进行中 `Snapshot.Link` = `LinkConnecting`（官方 connecting 态），
	//	   成功 → LinkConnected，失败 → LinkDown 并恢复退避重试。
	//
	// 幂等：没有会话 / 没有订阅时也安全（壳只重绘，不报错）。
	//
	// nil = 壳没接（老壳 / 测试）：插件这时**不把 warning 胶囊画成按钮**——
	// 画一个点了没反应的东西比不画更坏。
	Reconnect func()

	// Invalidate 请求壳**重绘一帧**，不写状态栏。
	//
	// 用途只有一个：插件里的**时间动画**（官方 CSS transition 的等价物）。
	// 官方连接胶囊有 150ms 离场淡出（`._indicator{transition:…opacity .15s}` +
	// `._leaving{opacity:0}`），React 靠 `setTimeout` + 重渲染完成；本仓没有
	// 渲染循环，插件改完状态后必须自己申请下一帧，否则要等下一个外部事件
	// （WS 帧 / 按键）才重绘——淡出会卡在半透明。
	//
	// 与 Notify 的区别：Notify 会**写状态栏文案**（"已复制"这类给用户看的反馈）；
	// 动画帧不需要也不该改任何用户可见文案。
	//
	// nil = 壳没接：插件退化为"下次自然重绘时再推进"（动画不完整，但不崩）。
	Invalidate func()

	// LoadOlder 向上翻一页历史（`session.history` 的**更早**那一页）。
	//
	// 为什么是一个动作而不是让插件自己发请求：网络与 journal 世代都归壳管
	// （插件不建 Transport，界面线 R2/R7），插件只画按钮。
	//
	// **壳可能异步完成**（网络请求必然不在绘制线程里）：调用方不能假设返回时
	// `Snapshot.History.HasMore` 已经变了——下一帧才变。
	//
	// nil = 壳没接（老壳）：插件这时不画入口——画一个点了没反应的东西，
	// 比不画更让人以为"已经在加载了"。
	LoadOlder func()
	// SetDraft 把文字灌进输入框。
	//
	// **当前没有任何插件调用它**（壳已接线，但调用点为零）。设计意图本来是
	// "某个插件想替用户填草稿"，现在那个场景不存在——输入框的文字由画输入框的
	// 插件自己持有。留着是为了将来真有需求时不必改公共 API，
	// 但请把它当**空转接口**：不要因为它存在就假设有人在用。
	SetDraft func(text string)

	// ConfigGet 取宿主的 UI 配置（`config.get`，纯 key/value，宿主不解释）。
	//
	// **同步返回的是壳缓存的那份**：壳在启动与每次 ConfigSet 之后刷新。
	// 所以插件在 Register / 首帧读到的可能是空 map——**必须按"可能没有"来写**，
	// 不要假设某个键一定在。nil = 壳没接（旧的 mockd），同样按"没有配置"处理。
	ConfigGet func() map[string]string
	// ConfigSet 写若干键（**合并**语义：只覆盖你给的键）。
	//
	// 壳异步落盘、失败只写状态栏——插件不等待、不重试。
	ConfigSet func(kv map[string]string)

	// FSList 列一个目录（宿主 `workspace.list`，**已支持 path 入参**）。
	//
	// **同步返回的是壳的缓存**：首次问某个 dir 拿到空切片，壳在后台拉，
	// 拉到后重绘。所以插件必须能接受"这一帧没有、下一帧才有"——
	// 画"加载中"或者干脆先不画都行，但**不能把它当成"这个目录是空的"**。
	// nil = 壳没接。
	FSList func(dir string) []FSEntry
	// FSRead 读一个文件（**绝对路径**，Windows 风格）。
	//
	// 路径怎么来：`Snapshot.CWD` + "/" + `FSList` 的名字。宿主不做工作区内限制，
	// **调用方自己判**——别拿用户输入直接拼。
	FSRead func(path string) (string, error)
	// PreviewFile 把文件交给侧栏预览（官方 `sidebarRight.openResource` 的等价物，
	// 落点是 `plugins/dsfiles` 的浮层）。参数是**绝对路径**或相对当前会话 CWD 的路径。
	//
	// 为什么不是 `OpenFile`：那是"用系统默认程序打开"，官方文件引用 chip 的点击
	// 是"在应用内预览"，两条能力不能互相顶替。
	// nil = 壳没接 → 引用 chip **不画成可点**（死按钮比没有更坏）。
	PreviewFile func(path string)

	// FileReferences 列 `@` 之后的文件候选（宿主 `fileReferences.list`）。
	//
	// **同步返回壳缓存**：查询键是会话 + 原文 query；首次问会触发后台拉取，
	// 拉到后 Invalidate，下一帧再给真值。nil = 壳没接（老宿主）→ `@` 菜单
	// 只保留本仓已有的目录树 source（见 dscomposer 的 trigger.go）。
	FileReferences func(sessionID, query string) []FileReferenceCandidate
	// SessionReferenceCandidates 列 `@` 之后的会话候选
	//（宿主 `sessionReferenceResolver.candidates`）。语义同 FileReferences。
	SessionReferenceCandidates func(sessionID, query string) []SessionReferenceCandidate
	// Skills 是某个会话可见的 `/` 技能候选（宿主 `skills.list`）。
	//
	// 与 `dsskill` 的 `SetInspect` 同源但不同消费者：那条要的是 `name -> path`
	// 的 resolver（打开 SKILL.md），这里要的是候选清单（`/` 菜单）。
	// 空切片 = 真没有技能；nil = 还没拉到（两者对菜单的效果都是"这一帧没有"）。
	Skills func(sessionID string) []SkillCandidate

	// MessageFeedback 读某会话已记录的消息反馈（壳缓存，启动/切换会话时后台拉）。
	//
	// 返回的 map 键是消息身份（宿主 messageId 或本仓 v4 seq 适配值，见
	// dsconversation/feedback.go）；第二个返回值 false = 还没拉到。
	MessageFeedback func(sessionID string) (map[string]FeedbackItem, bool)
	// MessageFeedbackPut 写一条消息反馈。**异步**：终态由 FeedbackTakeResult
	// 交给插件显示 toast，插件不等待、不重试。
	MessageFeedbackPut func(sessionID, messageID, rating, note, category string)
	// MessageFeedbackDelete 撤回一条消息反馈。**异步**，语义同上。
	MessageFeedbackDelete func(sessionID, messageID string)
	// FeedbackTakeResult 取走最近一次 put/delete 的终态（取走即清空）。
	// ok=false = 没有待处理结果。插件在下一帧读它来显示 toast。
	FeedbackTakeResult func(sessionID string) (FeedbackResult, bool)

	// ScheduleList 列定时任务（宿主 `schedule.list`）。
	//
	// **同步返回壳缓存**：壳启动后拉一次、之后每次 `schedule.*` 变更再拉。
	// nil = 壳没接（老宿主）。
	//
	// 注：这是本仓宿主的**扩展面**（按会话落盘的精简投影）。官方任务管理页
	// 消费的是 `schedule.catalog`，见下面的 ScheduleCatalog。
	ScheduleList func() []ScheduleItem

	// ScheduleCatalog 返回官方 `schedule.catalog` 的全量任务目录（壳缓存）。
	//
	// 与 ScheduleList 的区别：catalog 带官方 `ScheduleCatalogEntry` 的 7-kind
	// union、`status`、`target`、`involvedSessionIds`、`lastDelivery`、
	// `deliveryFloorEndsAt`——官方任务管理页的列表/状态/下次运行全靠它。
	//
	// 三个返回值：
	//   - entries：最近一次成功拉到的目录；nil = 还没拉到过；
	//   - loaded：是否已经有过一次成功读取（**不能拿空数组当"没有任务"**：
	//     还没拉到与真的没有是两件事，界面必须画成"加载中"而不是空状态）；
	//   - errText：最近一次失败的原因（宿主原文）；空串 = 没有失败。
	//
	// **首次调用会触发一次后台拉取**（与 JobsList 同一模式）：插件每帧问，
	// 壳只在没有缓存时发一次请求。nil = 壳没接（老壳）。
	ScheduleCatalog func() (entries []apiclient.ScheduleEntry, loaded bool, errText string)

	// ScheduleHistory 读一条任务的投递历史（官方 `schedule.history`）。
	//
	// 同步返回壳缓存；第一次问会触发后台拉取。page/loaded/errText 语义同上，
	// 另外 page.NextBefore 非空表示"更旧还有"，由 ScheduleHistoryMore 续读。
	ScheduleHistory func(id string) (page apiclient.ScheduleHistoryPage, loaded bool, errText string)

	// ScheduleHistoryMore 续读更旧的一页（`before=page.NextBefore`）。**异步**。
	ScheduleHistoryMore func(id string)

	// ScheduleDelete 删除一条任务（官方 `schedule.edit` action=delete）。**异步**。
	//
	// 删除是破坏性动作：壳只在宿主回 `{id,deleted:true}` 后重拉目录；
	// miss（schedule_not_found / schedule_conflict）不是错误，走重拉收敛。
	ScheduleDelete func(id string)

	// ScheduleSetActive 暂停/恢复一条任务（`schedule.edit` patch.active）。**异步**。
	ScheduleSetActive func(id string, active bool, expectedUpdatedAt string)

	// ScheduleRun 立即执行一次（官方 `schedule.run`）。**异步**。
	ScheduleRun func(id string)

	// ScheduleActionError 取走最近一次 schedule 动作失败的动作 id（取走即清空）。
	//
	// 返回值是 "delete" / "update" / "run" 之一（对应官方
	// `toast.deleteFailed` / `host.saveError` / `host.runError` 三句本地化文案）；
	// ok=false = 没有待显示的动作失败。宿主的原始错误文本由壳写进状态栏，
	// **不在界面上另编一句话**（官方 toast 也只有那一句）。
	ScheduleActionError func() (action string, ok bool)

	// ScheduleRefresh 重新拉一次官方任务目录（官方 CatalogFeedback 的重试）。
	// **异步**；拉取失败写进 ScheduleCatalog 的 errText。
	ScheduleRefresh func()

	// ── 官方插件管理页（`dsh-client-ui-plugin-manager`，侧栏「插件」）──────

	// PluginBundles 返回官方 `pluginManager.listBundles` 的全量 bundle 清单（壳缓存）。
	//
	// 四个返回值：
	//   - bundles：最近一次成功拉到的清单；nil = 还没拉到过；
	//   - loaded：是否已经有过一次成功读取（不能拿空数组当"没有插件"）；
	//   - unavailable：宿主**没有这个能力**（501 not_implemented）。插件据此画
	//     官方 `unavailable`（"本部署没有可管理的 profile"），**不是**错误 + 重试
	//     ——重试一个不存在的方法永远同一个 501；
	//   - errText：最近一次失败的原因（宿主原文）；空串 = 没有失败。
	//
	// **首次调用触发一次后台拉取**（插件每帧问，壳只在没有缓存时发一次请求）。
	// nil = 壳没接（老壳）：插件这时不画这一页。
	PluginBundles func() (bundles []apiclient.PluginBundle, loaded bool, unavailable bool, errText string)
	// PluginBundlesRefresh 重新拉一次 bundle 清单（官方 refresh / 动作后重拉）。**异步**。
	PluginBundlesRefresh func()
	// PluginBundleDetail 读一个 bundle 的 README 文档（官方 `readBundleDetail`）。
	//
	// 同步返回壳缓存；第一次问触发后台拉取（键是 bundle 名）。
	// loaded=false 且 errText 空 = 还在读；errText 非空 = 读失败（画重试）。
	PluginBundleDetail func(name string) (detail apiclient.PluginBundleDetail, loaded bool, errText string)
	// PluginBundleDetailRefresh 重读一个 bundle 的文档（失败后的重试）。**异步**。
	PluginBundleDetailRefresh func(name string)
	// PluginSetBundleEnabled 启用/停用一个 bundle（官方 `setBundleEnabled`）。**异步**。
	//
	// 写动作**不做乐观更新**：壳成功后重拉清单，失败把动作名交给
	// PluginActionError，由插件画官方那一句本地化文案。
	PluginSetBundleEnabled func(name string, enabled bool)
	// PluginRemoveBundle 卸载一个 bundle（官方 `removeBundle`）。**异步**。
	//
	// 破坏性动作：确认由插件负责（官方是 Modal，本仓按仓规用"同一按钮再点一次"）。
	PluginRemoveBundle func(name string)
	// PluginActionError 取走最近一次 pluginManager 写动作的失败（取走即清空）。
	//
	// action 是 "enable" / "disable" / "uninstall" 之一，对应官方
	// `failedEnable` / `failedDisable` / `failedUninstall` 三句本地化文案；
	// name 是失败的那个 bundle（界面据此清掉它自己的"在飞"标记）；
	// ok=false = 没有待显示的动作失败。宿主原文由壳写状态栏，
	// 界面**不另编一句话**。
	PluginActionError func() (action string, name string, ok bool)

	// OpenDirectoryPicker 打开**官方目录选择器**（官方 `dsh-client-ui-directory-picker-browse`
	// 的 Miller 双列对话框），用它「添加工作区」。
	//
	// 官方归属：这个对话框是 `ui-workspace` 的 directory-flow 洞的两个填充者之一
	// （另一个是 `ui-directory-picker-native`，走 OS 原生选择器）。本仓只落地了
	// browse 那一个 backend，所以这里只有一个入口，没有"二选一"。
	//
	// **为什么由壳转一手而不是插件互相找**：对话框本体在 `plugins/dsdirpicker` 里，
	// 而触发它的「添加工作区」按钮在 `plugins/dssidebar` 的工作区工具带上
	// （官方同一位置）。插件之间不 direct 引用（R2 只拿 *Scope）；壳知道两者都在，
	// 所以由壳把这一下转过去。`plugins/dssidebar/frame.go` 那句
	// 「没有 OS 目录选择/workspace mutation，官方"添加工作区"整钮隐藏」的能力前提
	// 已经不成立：宿主 `directoryPicker.list/createDirectory` 是真目录浏览
	// （`HOST:cmd/dshd/serve_workspace_api.go:1668`）、`workspace.create` 是真注册表
	// （`:583`）。
	//
	// nil = 壳没接 → **插件不许画那个按钮**（画一个点了没反应的按钮比不画更糟）。
	OpenDirectoryPicker func()

	// JobsList 列后台任务（宿主 `jobs.list`）。
	//
	// **同步返回壳缓存**，和 ScheduleList 一样。区别在于数据新鲜度：
	// schedule 是磁盘文件、壳每次问都能拿最新；jobs 是**子进程推上来的快照**
	// （`@@JOBS@@` 行），所以刚起的任务可能要等下一次推送才出现——
	// 插件画的时候不该把"这一帧没有"理解成"没有后台任务"。
	// nil = 壳没接（老宿主）。
	JobsList func() []Job

	// JobsOutput 取一个后台任务**已就绪的那部分**输出（宿主 `jobs.output`）。
	//
	// 同步返回壳缓存，**不会在绘制线程里发请求**：缓存没有这一页时返回
	// ok=false，壳在后台拉、拉到后 Invalidate，插件下一帧再问一次。
	// 所以调用方必须把 ok=false 画成"加载中/还没到"，不能画成"没有输出"。
	//
	// cursor=0 表示"从开头那一页开始"；继续翻页时传上一页的 NextCursor。
	// 返回的是**累计视图**：Output 是从 0 到当前已加载游标的全部文本，
	// HasMore/NextCursor 指向下一页，所以插件可以整段重绘。
	// sessionID 空 = 不知道归属：壳直接返回 ok=false，不猜。
	// nil = 壳没接（老宿主）→ 插件不画"展开输出"。
	JobsOutput func(sessionID, id string, cursor int) (JobOutput, bool)

	// JobsStop 请求停止一个后台任务（宿主 `jobs.stop`）。
	//
	// **异步、不返回结果**：成功时壳刷新清单并重绘，失败只写状态栏。
	// 插件不能假设"点了就停了"——真正的停止以清单里 Done/ExitCode 变化为准。
	// nil = 壳没接（老宿主）→ 插件不画停止按钮（死按钮比没有更坏）。
	JobsStop func(sessionID, id string)

	// ModelsList 列可选模型（壳缓存，启动后拉一次）。
	// nil = 壳没接（老宿主）。
	ModelsList func() []Model
	// SetModel 改当前会话的模型覆盖（空串 = 清除）。
	SetModel func(id string)

	// Goal 读这条会话当前在做的事（壳缓存，来自 session.load 的投影）。
	// 空 = 没设过，插件据此不画目标条。
	Goal func() string

	// TerminalOpen 起一条长连接 shell，返回它的 id（空 = 开不了）。
	//
	// **一次会话一个就够**：这个面板不是终端模拟器，是"看一眼命令在跑什么"。
	TerminalOpen func() string
	// TerminalInput 往终端写一段（**原样**，要换行自己带）。
	TerminalInput func(id, data string)
	// TerminalRead 取终端**累计**的可读文本（壳已剥掉 ANSI），并推进内部水位。
	//
	// 返回的是"到目前为止的全部"（不是增量）——插件画的时候要能整段重绘，
	// 让它自己维护增量只会多一处可能对不上的状态。
	TerminalRead func(id string) string

	// SetPermission 改当前会话的审批策略（deny/ask/allow/auto）。
	//
	// **空串 = 清除覆盖**（回到宿主的 --policy）。宿主会白名单校验，
	// 传没定义的值会 400——插件不该把"看起来合理的词"直接递过去。
	// nil = 壳没接（老宿主）。
	SetPermission func(perm string)

	// SetTheme 换主题：`dark` / `light` / `system`（跟着操作系统走）。
	//
	// 官方把这件事做成 Appearance 设置行（`dsh-client-ui-theme`：
	// "light/dark/system state"）；我们还没有设置面，所以入口在命令面板。
	//
	// **运行时立刻生效，不用重启**：壳是就地改那个 `*Theme` 对象，
	// 所有插件手里是同一个指针，下一帧就是新颜色。
	//
	// 传不认识的模式**不改**（壳会把原因写状态栏）——悄悄忽略比显示一个
	// 用户以为生效了的名字好。nil = 壳还没接线。
	SetTheme func(mode string)

	// Approve 回一次审批（H13 的 `session.approve`）。
	//
	//	requestID ← Approval.RequestID，optionID ← Approval.Options[].OptionID
	//
	// 两个 id 都必须**原样**转发：requestID 是宿主认这次审批的唯一凭据，
	// optionID 决定"允许这一次 / 以后都允许 / 拒绝"，插件自己造一个就等于
	// 替用户改了决定。壳拿到之后走 HTTP + 失败反馈（404 已失效 / 400 未知选项），
	// 插件只管把按钮点成了哪一项。
	//
	// nil = 壳还没接线（mockd 那条旧路径）：插件这时才退回下面的 Allow/Deny，
	// 那两个只清本地气泡，**不产生真实审批**（H13 明确：气泡消失 ≠ 已批准）。
	Approve func(requestID, optionID string)

	// Allow / Deny 是**旧接口**，只为还没接 Approve 的壳保命用。
	// 新代码一律用 Approve：这两个没有 id，宿主那边无从对应。
	Allow func()
	Deny  func()

	// ── 设置面（HOST-SETTINGS-FORM，rc.2 官方 SettingsRoot）──
	//
	// 这三个是设置面板的**数据面**。为什么放在 Actions 而不是让插件自己发 HTTP：
	// 插件不 import 壳、也不自己建 Transport（R2/R7），数据只经壳的缓存。
	//
	// 同步返回**壳缓存的那一份**（与 ModelsList / ScheduleList 同一个形状）：
	// 首次问会触发一次后台拉取，插件下一帧就能拿到；nil = 壳没接。
	//
	// 判据：`SettingsDescribe != nil` 时设置面板画**官方 5 section 四区面板**；
	// nil 时退回 rc.1 的最小浮层（兼容桥，见 plugins/dssettings 的注释）。
	SettingsDescribe func() *apiclient.SettingsDescribeResult
	// SettingsHostFacts 返回这一帧的宿主事实（决定动态注册哪些 section/item）。
	//
	// 零值 = 什么都提供不了：那时所有条件项都不注册，面板只画固定项。
	// **不许把零值当"默认开"**——那会画出宿主根本没提供的行。
	SettingsHostFacts func() SettingsHostFacts
	// SettingsFormSet 写一个字段（宿主 `settings.form.set`），返回写后的 section。
	//
	// expectedRevision 是持久乐观锁：0 = 不 fence（用宿主最新 revision）。
	// 冲突回 409 `revision_conflict`，插件**不得重试覆盖**。
	SettingsFormSet func(ns string, path []string, value any, expectedRevision int) (apiclient.SettingsSectionView, error)
	// SettingsFormReset 清一个字段 / 整个 namespace（宿主 `settings.form.reset`）。
	SettingsFormReset func(ns string, path []string, expectedRevision int) (apiclient.SettingsSectionView, error)
	// SettingsMutate 原子提交一批路径操作（宿主 `settings.mutate`）。
	//
	// retryPolicy 的嵌套路径（`providers.<route>.retryPolicy.*`）走这条：
	// 官方 `settings.form.set` 只接受单段字段名。
	SettingsMutate func(ns string, ops []apiclient.SettingsOp, expectedRevision int) (apiclient.SettingsNamespaceView, error)

	// PluginInventory 返回壳缓存的官方 `pluginInventory.list` 结果
	// —— 设置面板「内置插件」页的数据面。
	//
	// 为什么它是**独立**于 SettingsDescribe 的一路：官方那份清单不是 Host 设置
	// （`settings.describe` 里没有它），它是 Cordis Loader 的只读投影
	// （`dsh-host-plugin-inventory/src/index.ts:82-121`）。本仓宿主同形：
	// `pluginInventory.list` 与 `pluginManager.listPlugins` 共用同一份
	// Manager/Loader（`DSH-GO/internal/transport/handlers_plugin_inventory_official.go:26-69`）。
	//
	// 与 SettingsDescribe 同一套缓存形状：同步返回**壳缓存的那一份**，
	// 首次问会触发一次后台拉取；nil = 壳没接这条能力（那时插件页退回
	// 「未知·宿主未接」的说明，不画假卡片）。
	PluginInventory func() *apiclient.PluginInventoryResult
	// PluginInventoryRefresh 触发一次后台重读（官方 Plugins 页的「重试」按钮，
	// `ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.tsx:321-324`）。
	//
	// 调用后不等结果：壳拉完会 Invalidate，下一帧插件读到的就是新值。
	PluginInventoryRefresh func()
}

// SettingsHostFacts 是设置面板需要的**宿主事实**（决定动态注册）。
//
// 字段名对齐官方来源：`dshDesktop.protocolVersion` / `dshDesktop.updates.channels`
// / `sidebarRightTabs` / `session-log-deepseek` / `DSH_CLIENT_VERSION` /
// `permissionPresets.catalog` / `ui-chat.transcriptView` / `snapshot.view.status`。
type SettingsHostFacts struct {
	// DesktopProtocolVersion 是 `dshDesktop.protocolVersion`；0 = 没有桌面桥。
	DesktopProtocolVersion int
	// UpdatesChannels 是 `dshDesktop.updates.channels`；空 = 桥没给通道。
	UpdatesChannels []string
	// ShellChannelServed 报告**宿主**的壳更新渠道服务能不能应答。
	ShellChannelServed bool
	// ShellChannel 是宿主报的当前渠道；nil = 用户还没选过。
	ShellChannel *string
	// ShellChannelFailure 是宿主报的壳级故障。
	ShellChannelFailure string
	// ShortcutCount 是**本窗口**真实生效的键位数（GUI 自己的键位表条目数）。
	//
	// 官方 `shortcuts` 是窗口本地服务，不是 Host 设置；本仓也不例外。
	// 0 = 键位表没装 -> 「快捷键」行显示未知。
	ShortcutCount int
	// SidebarRightTabs 表示 `sidebarRightTabs` 服务存在。
	SidebarRightTabs bool
	// SessionLogServed 表示 `session-log-deepseek` namespace 被宿主服务。
	SessionLogServed bool
	// ClientVersion 是 build 注入的 `DSH_CLIENT_VERSION`；空 = 没注入。
	ClientVersion string
	// PermissionCatalog 表示 `permissionPresets.catalog` 已就绪。
	PermissionCatalog bool
	// TranscriptView 是 `ui-chat.transcriptView` 的当前值；空 = 未知。
	TranscriptView string
	// CredentialStored 表示 `snapshot.view.status === "credential-stored"`。
	CredentialStored bool
}
