// Package pluginapi 是界面线的插件契约：全部接口 + 共享数据类型，**零实现**。
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
	"github.com/vivaui/viva/flex"
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
	Fields() []*flex.Field
}

// KeyHandler 是「想接键盘的插件」的可选接口。
//
// 壳把按键按装配顺序依次交给插件，**第一个返回 true 的赢**，壳自己不认识任何具体快捷键——
// 这样「↑ 切会话」是侧栏插件的能力，不是壳里写死的 if。
type KeyHandler interface {
	HandleKey(ev ui.KeyEvent) bool
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
)

// Painter 是一块可挂载的绘制单元。
type Painter struct {
	// ID 在同一个插件内唯一，用于日志和调试（不是交互 id）。
	ID string
	// Slot 挂在哪。
	Slot Slot
	// Order 决定同一个槽里谁先画：小的先（= 贴在上面）。
	Order int
	// Draw 画自己。frame 已经是这块槽的位置和尺寸，别自己算全局坐标。
	Draw func(f *flex.Frame, env *Env)
	// AfterDraw 在整棵树布局完成之后调用，给那些「必须先知道位置才能画」的东西用：
	// MarkdownView.Paint、自绘文本排版、代码块高亮。canvas 是全屏画布，
	// 坐标要自己从 Draw 阶段量到的矩形来（拿 Box.Rect() 存起来）。
	//
	// 为什么需要它：viva 的绘制顺序不受调用方控制，在 Draw 里画一条线会被
	// 后面的节点覆盖（踩过）。所以分两个阶段：先布局（Draw），再覆画（AfterDraw）。
	AfterDraw func(canvas *ui.Canvas, env *Env)
	// Children 让一个槽位在画完自己之后，再把 f 交给下一层槽位。
	//
	// 这就是"高塔"的做法：壳只画 root 一个槽，框架插件在 root 里切出
	// shell.leading / serveral 个列 / shell.bottom，再把 f 交给
	// sidebar.* / main.* / rightbar。插件不需要知道自己在第几层。
	Children func(f *flex.Frame, env *Env, drawSlot func(Slot))
}

// Primitives 是基础组件的画法，由根本依赖插件（ds-core）提供。
//
// 做成**接口**而不是具体类型，是为了让下游插件只认契约、不认 dscore 这个包：
// 哪天要换一套视觉实现（浅色变体、紧凑变体），下游一行都不用改。
// 实现方必须自己加 `var _ pluginapi.Primitives = (*X)(nil)` 断言。
type Primitives interface {
	// ListRow 一条列表行：选中/悬停底色由实现决定，调用方只给状态。
	ListRow(f *flex.Frame, id ui.ID, active bool, height float32, body func()) flex.Box
	// SectionHeader 一个分组头（标题 + 条数 + 展开/收起箭头）。
	SectionHeader(f *flex.Frame, id ui.ID, title string, count int, open bool) flex.Box
	// Badge 行尾的小徽标。
	Badge(f *flex.Frame, text string, color ui.Color) flex.Box
	// Dot 一个状态圆点。
	Dot(f *flex.Frame, color ui.Color, size float32) flex.Box
	// Divider 一条最轻的分隔线。
	Divider(f *flex.Frame)
	// CircleButton 一个 28px 圆形图标按钮（官方 .action 的尺寸），返回是否被点击。
	CircleButton(f *flex.Frame, id ui.ID, glyph string, filled bool) bool
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
	Tree *ui.Tree
	Feat ui.Scope
	// Canvas 只在 Draw 阶段有效：Markdown 排版这类必须先量后画的东西要它。
	// （AfterDraw 收到的 canvas 是布局后的全屏画布，两者是同一个对象。）
	Canvas *ui.Canvas
	// Scroll 取某个滚动区的状态。viva 要求"要的时候再取"，所以给的是函数不是指针。
	Scroll func(id ui.ID) *flex.ScrollState
	// Has 判断某个槽位有没有插件挂东西。框架插件靠它决定"这一列画不画"——
	// 没有侧栏插件时不该留一条空栏。
	Has func(Slot) bool
	// MainWidth 是主区那一列的内容宽度（壳按 窗口 - 侧栏 - 右栏 算好）。
	// 插件要按像素定宽时只能用它：槽位里 f.Bounds() 给的是整窗宽（实测 1180）。
	MainWidth float32
	// Bounds 是这一帧的整窗尺寸。AfterDraw 拿到的 canvas 没有尺寸接口，
	// 要在覆画时算裁剪区就用这个。
	Bounds ui.Rect
}

// Snapshot 是一帧的数据快照，壳在锁内拷好再传进来，插件里不用加锁。
type Snapshot struct {
	Sessions []apiclient.Session
	Cur      string
	Search   string
	Status   string
	Msgs     []apiclient.Message
	Total    int
	Stream   string
	Approval *Approval
	// Usage 是当前会话的 token 用量（壳从 session.load 拿到的合计）。
	// Usage.Known() 为 false 时插件不许显示任何 token 数字。
	Usage Usage
	// ShowRightbar 是右栏当前是开还是关（壳持有这个状态，插件只读）。
	ShowRightbar bool
	// Truncated 表示这次只渲染了末尾一段（视口虚拟化 / -limit）。
	Truncated bool
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

// Approval 是待审批的一次工具调用。
type Approval struct {
	Tool   string
	Cmd    string
	Reason string // 模型自述，**不可信**，界面上必须标出来
}

// Actions 是插件能触发的动作。
//
// 只放「用户点了会改状态」的入口；读状态走 Snapshot。
// nil 的动作表示当前不可用，插件要自己判断（别假设一定有）。
type Actions struct {
	OpenSession func(id string)
	NewSession  func()
	// Notify 让插件在状态栏说一句话（"已复制"这类反馈）。
	// 插件不直接写状态栏：状态栏是另一个插件的地盘。
	Notify func(msg string)
	// Copy 把文本放进系统剪贴板（插件不 import app，剪贴板归壳管）。
	Copy func(text string)
	// ToggleRightbar 收起/展开右栏（官方也能收）。
	ToggleRightbar func()
	// Retry 重发上一条用户消息。
	Retry func()
	// Send 把这段文字发出去。输入框由哪个插件画，文字就由哪个插件给——
	// 壳不再持有草稿，所以不用 SetDraft 回灌。
	Send     func(text string)
	SetDraft func(text string)
	Allow    func()
	Deny     func()
}
