// Package dstool 是「工具卡」这个独立部件。
//
// 官方对应物：`dsh-client-ui-tool`。在这之前工具卡长在 `dsconversation` 里，
// 只有一种通用画法（工具名 + 摘要）；抽出来之后这里按**工具类型分派**：
//
//	renderers map[string]Renderer   bash / read / edit / glob / grep / … → 各自的画法
//
// 一句话分清职责：
//
//	ds-conversation  决定「消息流里哪一条是工具消息、排在什么位置、占多高」
//	ds-tool（本包）  决定「一条工具消息长什么样」
//
// 为什么状态（展开/收起）也在本包：它是工具卡自己的交互，消息流只问
// `Open(seq)` 与 `Height(seq, view)`。抽出去之前那两个 map 在 dsconversation 里，
// 和消息流的滚动状态混在一起，改一个工具的画法要动整条消息流。
//
// 渲染器**不解析 Markdown、不画消息流**，只画这一张卡：标题行 + （展开时的）正文。
package dstool

import (
	"strconv"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// ServiceKey 是本插件提供的能力键：消息流用 `ctx.Inject(ServiceKey)` 拿到
// 一个 Service。和 ds-topbar 的 viewapi.ServiceKey 同一个用法。
const ServiceKey = "ui.tool-card"

// Card 是一条工具消息在界面上能用的全部信息（只读快照）。
//
// 刻意不含 Snapshot / Actions / 锁：工具卡没有任何"用户点了会改状态"的动作，
// 它只有一个自己的展开状态，那个状态在 Service 里。
type Card struct {
	// Seq 只是 dstool.Service 现有的局部 int 渲染键，不是 v4 seq。
	// ds-conversation 用 CallID/MessageKey 分配它，因此同参交错调用、Seq=0 与
	// 跨会话同序号都不会共享展开状态；本包不反向编造宿主消息身份。
	Seq int
	// Tool 是工具名（bash / read / glob …），分派就认它。
	Tool string
	// CallID 是宿主给这次调用的 id（`apiclient.Message.CallID`）。
	//
	// 为什么工具卡需要它：官方 `tool.call.toolview` 的 keyed 组件拿得到
	// `block.callId`，官方 `SkillRow` 在**参数为空**时用它当标题兜底
	// （`SkillRow.tsx:40` 的 `argsRaw === '' ? callId : firstLine(argsRaw)`）。
	// 空串 = 消息流还没填（老壳），消费方必须自己兜底，**不许**编一个假 id。
	CallID string
	// Args 是宿主给的参数原文（一般是 JSON，也可能是一行命令）。
	Args string
	// Summary 是宿主给的一行摘要（bash 的 "exit 0 · …" 就在这）。
	Summary string
	// Status 是消息流依据显式结果、原始终局或活动 Turn 算出的真值状态。
	// Neutral 不画徽标；渲染器不得仅凭 Args 或工具名补一个 preparing/running。
	Status Status
	// View 是宿主声明的结构化结果（card / table / plain）。
	// 有它就优先画它——"不认识 == 没声明"，不猜工具语义。
	View *apiclient.ViewSpec
}

// Service 是消息流看工具卡的三个方法：画它、问它展开了没有、问它占多高。
//
// 只有三个方法，是因为消息流**不该**知道工具卡内部怎么分派：
// 想知道 bash 长什么样，去问 renderers，不要问 ds-conversation。
type Service interface {
	// Render 画一条工具卡（标题行 + 展开体），并处理这一行的展开/收起点击。
	Render(f ui.C, env *pluginapi.Env, card Card)
	// Open 报告这条工具卡当前是否展开。
	Open(seq int) bool
	// Height 估算这条工具卡本帧占多高（消息流的视口虚拟化要用）。
	// 必须与 Render 画出来的东西保持同一个量级，否则滚出去的内容会缺失。
	Height(seq int, view *apiclient.ViewSpec) float32
}

// Renderer 是一种工具类型（bash / read / glob …）的定制画法。
//
// 两个可空字段就是两个定制点：
//   - Head 决定标题行那两段文字从哪来（宿主没给 Summary 时，从参数里取命令/路径/模式）
//   - Body 决定展开体画什么（bash 是命令 + 退出码 + 输出预览；文件类工具是路径/模式）
//
// 都不填 = 走默认（参数原文 + 摘要），也就是抽取前唯一的那种画法。
type Renderer struct {
	// Tool 与 Card.Tool 精确匹配。空串的渲染器不会被登记。
	Tool string
	// Head 派生标题行。实现里先调 defaultHead 拿到默认结果再改是对的；
	// 返回空串就是"这一段不画"（不是"退回默认"——默认由调用方在不实现 Head 时用）。
	Head func(c Card) (title, summary string)
	// Body 画展开体。nil = 默认（参数原文 + 摘要）。
	Body func(b *BodyCtx)
	// Draw 完全接管这张卡的绘制（**整行替换**，不是"在通用行上补两段"）。
	//
	// 为什么需要它：官方 `tool.call.toolview` 是 **keyed 槽**
	// （`dsh-client-ui-skill/src/client/index.ts:89-92` 的
	// `{name:'tool.call.toolview', key:'skill'}`）——注册在某个工具的 key 上
	// 就是那个工具**整行**由这个组件负责，不是"在 ToolRow 里插一段"。
	// 本包默认的 Head/Body 是**通用行的两个定制点**，它外面那层
	// `ui.Row(PadX(4).PadY(2).Radius(SM).Hover(…))` 和展开体的
	// `ui.Column(PadX(22).PadY(4).Background(BgLayer1))` 是写死的；
	// 官方 skill 行既没有那圈内边距也没有那层底色（它自己是 `.card` +
	// `.row` + `.bodyWrap` 三件套，几何见 plugins/dsskill/official_skill.go），
	// 套在通用行里**每个数值都会差**。
	//
	// 所以需要整行接管的渲染器（今天只有 skill）走这个字段：
	// 非 nil 时 Head/Body 不参与，`Render` 直接把 frame 交给它。
	//
	// 返回值是**本帧占的高度**（逻辑像素）。消息流的视口虚拟化
	// （`Service.Height`）要用它——整行接管的卡画出来的形状和默认行不同，
	// 用默认估算会把展开的说明卡算矮，滚出去的内容再也画不回来。
	Draw func(f ui.C, env *pluginapi.Env, c Card) float32
}

// BodyCtx 是画展开体时能碰到的东西。
//
// 只给这些：画框、只读环境、这张卡，以及一个画徽标的出口。
// **不给** Service 本体——渲染器不该顺手改别的卡的展开状态。
type BodyCtx struct {
	Frame ui.C
	Env   *pluginapi.Env
	Card  Card
	// Badge 画一个小徽标（走 ds-core 的 Primitives）。prims 没装时它不画，
	// 所以渲染器可以无条件调用它。
	Badge func(text string, color ui.Color)
}

// Cards 是 Service 的实现。
type Cards struct {
	prims     pluginapi.Primitives
	renderers map[string]Renderer
	open      map[int]bool
	// heights 记「整行接管」（`Renderer.Draw`）的卡上一帧画出来多高。
	//
	// 为什么记而不是算：`Height` 在**排版之前**被调用（消息流的视口虚拟化
	// 要先知道每条多高），而整行接管的卡高度由它自己的展开态与内容决定，
	// 外面算不出来。上一帧的真值比一个猜的数准，而且只在同一条卡上生效
	// （key 是 `Card.Seq`，跨会话同序号不会串——Seq 由消息流按消息身份分配）。
	heights map[int]float32
}

// New 造一个工具卡渲染器。prims 是 ds-core 的基础组件画法；
// 传 nil 也能用（徽标不画），只有"内核里没装 ds-tool"的兜底路径会这样。
func New(prims pluginapi.Primitives) *Cards {
	s := &Cards{
		prims:     prims,
		renderers: map[string]Renderer{},
		open:      map[int]bool{},
		heights:   map[int]float32{},
	}
	s.registerDefaults()
	return s
}

// registerDefaults 就是那张分派表。加一种工具 = 加一行，不动消息流一行代码。
func (s *Cards) registerDefaults() {
	bash := bashRenderer()
	s.renderers[bash.Tool] = bash
	for _, r := range fileRenderers() {
		s.renderers[r.Tool] = r
	}
}

// Register 让第三方插件给某个工具换一种画法。
//
// 后登记的覆盖先登记的（同一个工具名）。这也是"官方 tool 部件支持按工具
// 类型定制"的扩展点：宿主以后加了 mcp__foo 之类的工具，界面不用改消息流。
func (s *Cards) Register(r Renderer) {
	if r.Tool == "" {
		return
	}
	if s.renderers == nil {
		s.renderers = map[string]Renderer{}
	}
	s.renderers[r.Tool] = r
}

// Render 实现 Service。
func (s *Cards) Render(f ui.C, env *pluginapi.Env, c Card) {
	// 整行接管：官方 keyed toolview 的语义就是这个（见 Renderer.Draw 的注释）。
	// 放在最前面，通用行的 Hit/展开状态一概不参与——两套展开状态同时存在
	// 会让"点一下展开两次"这种错看起来像渲染抖动。
	if r, ok := s.renderers[c.Tool]; ok && r.Draw != nil {
		s.heights[c.Seq] = r.Draw(f, env, c)
		return
	}
	t := env.Theme
	open := s.open[c.Seq]
	r, known := s.renderers[c.Tool]

	title, summary := s.head(c)
	status := statusLabel(c.Status)

	width := contentWidth(env)
	summaryWidth := width - 120
	if status != "" {
		// 给有真值的状态徽标留位；neutral 完全不占这一格。
		summaryWidth -= 64
	}

	// 标题行：点它展开。结构逐字沿用抽取前的那一段（像素对比见 tmp/tool-*.png）。
	head := ui.Row(f).WidthPercent(100).PadX(4).PadY(2).Radius(t.Radii.SM).
		Items(ui.Center).Gap(6).Hover(ui.Look{}.Background(t.Hover)).
		Hit(env.Tree.ID(env.Feat, "tool-"+strconv.Itoa(c.Seq)), "").Children(func() {
		// leading：16px 方框里的展开箭头（官方也是这个位置放 chevron）
		ui.Row(f).Width(16).Items(ui.Center).Children(func() {
			arrow := "▸"
			if open {
				arrow = "▾"
			}
			ui.Text(f, arrow).FontSize(t.Fonts.Tiny).Color(t.LabelSecondary)
		})
		ui.Text(f, title).Mono().FontSize(t.Fonts.Secondary).Color(t.LabelSecondary)
		if summary != "" {
			ui.Text(f, "·").FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
			ui.Text(f, summarize(summary, 120)).FontSize(t.Fonts.Secondary).
				Color(t.LabelTertiary).MaxWidth(summaryWidth).Ellipsis()
		}
		if status != "" {
			ui.Spacer(f)
			s.badge(f, status, statusColor(t, c.Status))
		}
	})
	if head.Clicked() {
		s.open[c.Seq] = !open
	}

	if !open {
		return
	}
	// 展开体：官方同样是 callRow 的一部分（radius sm、无边框），
	// 底色用最浅一层 BgLayer1 把正文托起来，不描边。
	ui.Column(f).WidthPercent(100).PadX(22).PadY(4).Gap(4).Radius(t.Radii.SM).
		Background(t.BgLayer1).Children(func() {
		// 宿主声明的结构化结果优先：它比我们从参数里猜的准。
		if c.View != nil {
			s.drawView(f, env, c.View)
			return
		}
		if known && r.Body != nil {
			r.Body(&BodyCtx{Frame: f, Env: env, Card: c, Badge: func(text string, color ui.Color) {
				s.badge(f, text, color)
			}})
			return
		}
		// 默认画法 = 抽取前的那一种：参数原文 + 摘要。
		if c.Args != "" {
			ui.Text(f, c.Args).Mono().FontSize(t.Fonts.Tiny).Color(t.LabelCaption).MaxLines(6)
		}
		if c.Summary != "" {
			ui.Text(f, c.Summary).FontSize(t.Fonts.Caption).Color(t.LabelTertiary).MaxLines(12)
		}
	})
}

// Open 实现 Service。
func (s *Cards) Open(seq int) bool { return s.open[seq] }

// Height 实现 Service：数字与抽取前 `estimateMsgHeight` 的工具分支逐个相同。
//
// 为什么不按真实排版量：这个函数在**排版之前**被调用（消息流的视口虚拟化要
// 先知道每条多高），所以只能估。估小了会把还在视口里的消息判成"在视口外"
// 直接不画，所以宁大勿小。
func (s *Cards) Height(seq int, view *apiclient.ViewSpec) float32 {
	// 整行接管的卡：上一帧画出来的真值优先。没有任何一帧画过时退回默认估算
	// （见 Renderer.Draw 的注释：默认 28 是"收起时只有标题一行"，宁大勿小）。
	if h, ok := s.heights[seq]; ok {
		return h
	}
	if !s.open[seq] {
		return 28 // 收起时只有标题一行（官方就是这么一行）
	}
	if view != nil {
		// 有结构化显示的工具卡：头 + 若干行
		return float32(50 + 20*(len(view.Fields)+min(len(view.Rows), 8)))
	}
	return 70
}

// viewOverridesArgs 报告 View 是否带有可直接展示的结果内容。纯 Badge 是状态声明，
// 仍保留参数派生的命令/路径；一旦宿主给了 Body/Fields/Rows，就不再让参数原文
// （尤其 edit 的 old/new）抢在结构化结果前面。
func viewOverridesArgs(view *apiclient.ViewSpec) bool {
	return view != nil && (view.Body != "" || len(view.Fields) > 0 || len(view.Columns) > 0 || len(view.Rows) > 0)
}

// defaultHead 的顺序是：工具名 → （没名字才用 View.Title）；
// Summary → 有内容的 View → 参数原文。宿主显式结果优先于界面从参数派生的摘要。
func defaultHead(c Card) (title, summary string) {
	title = c.Tool
	if title == "" && c.View != nil {
		title = c.View.Title
	}
	summary = c.Summary
	if summary == "" {
		if viewOverridesArgs(c.View) {
			summary = c.View.Body
		} else {
			summary = c.Args
		}
	}
	return title, summary
}

// head 派生一条工具卡的标题行：默认规则 → 该工具的定制 Head → 宿主的错误标记。
//
// 错误标记**最后**加，而且不交给渲染器：渲染器只回答"从参数里取哪一段"，
// 不该有能力把"这次失败了"抹掉（第一版就是这么丢的：bash 的 Head 从参数取命令，
// 把 defaultHead 已经加好的"错误："覆盖掉了，单元测试当场抓到）。
func (s *Cards) head(c Card) (title, summary string) {
	title, summary = defaultHead(c)
	if r, ok := s.renderers[c.Tool]; ok && r.Head != nil {
		title, summary = r.Head(c)
	}
	if c.View != nil && c.View.Badge == "error" && summary != "" {
		summary = "错误：" + summary
	}
	return title, summary
}

// contentWidth 是主区内容可用的最大像素宽。
//
// 为什么不用百分比：槽位里 f.Bounds() 拿到的是整窗宽（实测 1180），
// 百分比也按整窗解析，结果就是卡片内容一路画到窗口边缘、Ellipsis 没有边界。
// 唯一可靠的事实是壳保证了侧栏，主区再来回扣 40 的内边距/滚动条。
func contentWidth(env *pluginapi.Env) float32 {
	w := env.MainWidth - 40
	if w < 200 {
		w = 200
	}
	return w
}

// badge 画徽标；prims 没装时安静地不画（兜底路径）。
func (s *Cards) badge(f ui.C, text string, color ui.Color) {
	if s.prims == nil || text == "" {
		return
	}
	s.prims.Badge(f, text, color)
}

// 确保实现满足契约。
var _ Service = (*Cards)(nil)
