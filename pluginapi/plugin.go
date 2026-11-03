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
	// Chip 一个只读小标签（官方的 .chip：inline-flex、gap 3px、business-primary 文字）。
	Chip(f *flex.Frame, label string) flex.Box
	// Bubble 画一个用户气泡容器（官方 .bubble：radius xl、specific-bubble 底）。
	// 调用方传进内容，气泡只管外观。
	Bubble(f *flex.Frame, body func()) flex.Box
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
	Cur      string
	Search   string
	Status   string
	Msgs     []apiclient.Message
	Total    int
	Stream   string
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
	Disconnected bool
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
	// 也就是"当前上下文占多少"。0 = 不知道（不是 0 个 token），不许显示"上下文 0"。
	ContextTokens int64
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
	NewSession  func()
	// Notify 让插件在状态栏说一句话（"已复制"这类反馈）。
	// 插件不直接写状态栏：状态栏是另一个插件的地盘。
	//
	// **当前零调用**：壳接了线，但没有插件用它——"已复制"这类反馈是各插件
	// 自己调 Copy/Notify 之外的方式给的（看 composer 的复制按钮实现）。
	// 当空转接口看。
	Notify func(msg string)
	// Copy 把文本放进系统剪贴板（插件不 import app，剪贴板归壳管）。
	Copy func(text string)
	// ToggleRightbar 收起/展开右栏（官方也能收）。
	ToggleRightbar func()
	// Retry 重发上一条用户消息。
	Retry func()
	// Send 把这段文字发出去。输入框由哪个插件画，文字就由哪个插件给——
	// 壳不再持有草稿，所以不用 SetDraft 回灌。
	Send func(text string)
	// Cancel 打断当前会话正在跑的 turn（H13 的 session.cancel）。
	//
	// nil = 壳没接；插件要自己判，别假设一定有。
	Cancel func()
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
}
