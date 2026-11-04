// Package dsconversation 是「主区消息流」这个部件的移植 —— 界面里最大的一块。
//
// 官方对应物：`dsh-client-ui-chat`（消息流 / 代码块 / 输入区外壳）+
// `dsh-client-ui-conversation`（一轮对话的折叠与分隔）+ `dsh-client-ui-approval`（审批）。
//
// 这块代码原来长在 `cmd/app/main.go` 里，直接摸 model、直接拼锁。搬到这里之后
// 它只认 `pluginapi.Env`：只读快照 + Action，没有锁、没有客户端、没有全局变量。
//
// 两个阶段（viva 的绘制顺序不受调用方控制，所以必须分开）：
//
//	Draw      布局：算出每条消息的高度，把视口外的消息只留 Slot（不建文本纹理）
//	AfterDraw 覆画：MarkdownView.Paint 必须等布局定下来才知道画在哪
package dsconversation

import (
	"github.com/MYCF711/dsh-ui/plugins/dsbrand"
	"math"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dstool"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

// markdownBudget 是「最近的多少条消息用 Markdown 渲染」，其余降级成纯文本。
//
// 真实会话里有 200 条长消息，全上 Markdown 会让文档树常驻 + 首次排版 4 秒
// （实测 168.8 MB）。这是 LOD，不是偷懒。
// nlChar 是换行符。写成常量是为了避开「用工具拼 Go 源码时换行字面量被吃」的老坑（踩过三次）。
const nlChar = "\n"

const markdownBudget = 30

// mdSlot 是「先占位、布局后再画」的一个槽。
type mdSlot struct {
	idx int
	key string
	box flex.Box
}

// dbgOnce 只打一次布局诊断（临时）。
var dbgOnce = true

// Plugin 实现 pluginapi.Plugin / Panels。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives

	// brand 是注入来的品牌面（`ui.brand`）。nil = 没有 ds-brand 或它没提供，
	// 这时退回包级默认 `dsbrand.Default()`——**画法只有一份**，不是两套实现。
	brand *dsbrand.Brand

	// Markdown 文档与视图按消息下标复用：只有文本或宽度变了才重排。
	// 每帧重建是 111 MB 那一版的教训。
	docs    []markdown.Document
	views   []components.MarkdownView
	texts   []string
	widths  []float32
	heights []float32

	looked bool
	look   components.MarkdownLook

	slots []mdSlot
	// bodyBox 是消息流滚动区的盒子：AfterDraw 覆画 Markdown 时，
	// 裁剪区必须用它（视口矩形），不能用 env.Bounds（那是整窗——
	// 会把消息画到顶栏/输入区上面去）。
	bodyBox   flex.Box
	bodyRect  ui.Rect
	bodyReady bool
	// turnBoxes 仅属于本帧；turnAnchors 是 AfterDraw 提取的上一帧矩形值，
	// 不把失效的 flex.Box 带到下一帧。
	turnBoxes   map[string]flex.Box
	turnAnchors map[string]ui.Rect
	// collapsedOnce 记录调试开关下被用户手动收起过的，避免压不住点击。
	collapsedOnce map[string]bool
	// lastAct / lastMsgs / lastApproval 是绘制期快照，按键时要用。
	lastAct  pluginapi.Actions
	lastMsgs []apiclient.Message
	// expanded 记录哪些思考块被展开（键是 MessageKey，状态不跨会话串用）。
	// 缺省全收起：真实语料里思考比正文多八倍，默认展开会把消息流淹没。
	expanded map[string]bool
	// codeBlocks / fileLinks 与 Markdown 缓存同下标。代码块保留自己的 wrap/横滚
	// 状态；文件链接只来自显式 file: Markdown destination 的映射。
	codeBlocks map[int][]messageCode
	fileLinks  map[int]map[string]string
	// toolKeys 把 MessageKey 适配到 dstool.Service 当前的 int 局部键。
	// 这只改变渲染状态编号，Renderer 分派与工具数据仍由原 service 负责。
	toolKeys    map[string]int
	nextToolKey int
	// SearchOpen / SearchQuery 是调试开关：启动就带着搜索词打开（截搜索态用）。
	SearchOpen  bool
	SearchQuery string
	initialized bool

	// ExpandAllThinking 是调试开关：把所有思考块初始化为展开。
	// 只为"无头截图验证展开态"存在，正常运行一直是 false。
	ExpandAllThinking bool
	// lastWidth 是本帧内容列宽度（工具卡要按它定宽）。
	lastWidth float32

	// 会话内搜索：Ctrl+F 打开，只显示命中的消息（最简单也最不容易出错的一种）。
	searchOn    bool
	searchField flex.Field
	// searchHits 是最近一帧的命中数，给搜索栏显示。
	searchHits int

	// heroBoxes 保存首屏关键容器的本帧几何，供布局回归测试与截图证据核对。
	heroBoxes heroBoxes

	// 消息反馈（👍/👎）的本机记录，见 feedback.go 顶部那段边界说明。
	// feedback 是内存表，键 = feedback.<sessionID>.<msgIndex>；ConfigGet/ConfigSet
	// 是它的来源与落点（壳接到宿主 ui-config.json）。
	feedback       map[string]string
	feedbackSeeded bool

	// tools 是工具卡渲染器（ds-tool 提供 ui.tool-card）。
	//
	// 工具卡在 2026-11-04 从本包抽成独立插件：它的展开状态、每种工具的画法
	// 都搬去了 plugins/dstool，这里只留一个引用。为什么它不再是一个 map：
	// 展开状态原先和消息流的滚动状态混在一起，改一种工具的画法要动整条消息流。
	tools dstool.Service
}

// drawSearchBar 画搜索栏：输入框 + 命中数 + 关闭。
//
// 不做"上下跳转"：过滤之后命中的消息全都在列表里，跳转是在没有过滤的视图里
// 才有意义的东西。少一个状态，就少一处会和滚动位置打架的地方。
func (p *Plugin) drawSearchBar(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	f.Row().WidthPercent(100).Gap(8).Items(flex.Center).PadY(2).Children(func() {
		f.Text("搜索").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		f.Field(&p.searchField, env.Tree.ID(env.Feat, "search-msgs"), "搜索", "在当前会话里找…").
			Grow(1).FontSize(t.Fonts.Caption)
		hits := itoa(p.searchHits) + " 条"
		f.Text(hits).FontSize(t.Fonts.Caption).Color(t.LabelCaption)
		if f.Button(env.Tree.ID(env.Feat, "search-close"), "关闭").Ghost().Small().Clicked() {
			p.searchOn = false
			p.searchField.SetText("")
		}
	})
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-conversation" }

// Provides 实现 pluginapi.Plugin。
func (*Plugin) Provides() []string { return nil }

// Requires 实现 pluginapi.Plugin。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

// Register 实现 pluginapi.Plugin。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	// 工具卡由 ds-tool 提供（见 toolCards 里为什么不写进 Requires）。
	if cards, ok := ctx.Inject(dstool.ServiceKey).(dstool.Service); ok {
		p.tools = cards
	} else {
		// 走到这里说明内核里没有 ds-tool（只有"把本插件单独装进一个 kernel"的
		// 测试会这样）。工具卡仍然画得出来，只是没有 ds-core 的徽标——
		// 所以不 fail，只留一行警告，免得"为什么这次截图里没有 exit 0 徽标"
		// 让人去猜。
		ctx.Logf("警告：没有 %s（ds-tool 未装配），工具卡走兜底渲染器", dstool.ServiceKey)
	}
	// 品牌面：有就注入、没有就用包级默认（同一个 FishIcon，只多一层可替换）。
	if svc, ok := ctx.Inject(dsbrand.ServiceKey).(dsbrand.Service); ok && svc.Get != nil {
		if b, ok := svc.Get(); ok {
			p.brand = &b
		}
	}
	return nil
}

// Fields 实现 pluginapi.Fields：搜索框要收键盘（壳会把它登记给 viva）。
func (p *Plugin) Fields() []*flex.Field { return []*flex.Field{&p.searchField} }

// Panels 实现 pluginapi.Panels：一块主体（消息流）+ 一块审批气泡。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{
			ID:        "stream",
			Slot:      pluginapi.SlotMainBody,
			Order:     20,
			Draw:      p.draw,
			AfterDraw: p.afterDraw,
		},
	}
	// 审批条**不在这里画**：它属于"用户要做决定的地方"，由 ds-composer 在输入框
	// 上方画（dscomposer/approval.go）。这里曾经也画一条气泡，结果同屏出现两条
	// 「需要审批 bash」——两个入口、两套按钮，用户不知道该点哪个。
}

// draw 是布局阶段。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	msgs := env.Data.Msgs
	p.slots = p.slots[:0]
	p.bodyReady = false
	p.lastAct = env.Act
	p.lastMsgs = env.Data.Msgs
	if p.expanded == nil {
		p.expanded = map[string]bool{}
	}
	if p.collapsedOnce == nil {
		p.collapsedOnce = map[string]bool{}
	}
	if p.codeBlocks == nil {
		p.codeBlocks = map[int][]messageCode{}
	}
	if p.fileLinks == nil {
		p.fileLinks = map[int]map[string]string{}
	}
	if p.toolKeys == nil {
		p.toolKeys = map[string]int{}
	}
	if !p.initialized {
		p.initialized = true
		if p.SearchOpen {
			p.searchOn = true
			p.searchField.SetText(p.SearchQuery)
		}
	}

	if len(msgs) == 0 && env.Data.Stream == "" && !env.Data.Busy && len(env.Data.Outgoing) == 0 && len(env.Data.TurnEvents) == 0 {
		// 官方首屏（HeroShell）：没有会话 / 会话为空时，主区正中是
		// 「鲸鱼标记 + 探索未至之境 + 预览版」。见 hero.go。
		// Busy 也属于真实活动：已发送、正文尚未到达时不能退回欢迎区。
		p.drawHero(f, env)
		return
	}

	// 注意：槽位里 f.Bounds() 拿到的是**整窗**宽度（实测 1180），不是本列宽度。
	// 所以这里一律用百分比定宽，别拿它去算列宽——算了就会把内容推到窗口外（踩过两次）。
	// 布局宽度只认 env.MainWidth（壳算好的主区宽度）；Bounds 仅作兜底，防止估高度除零。
	width := env.MainWidth
	if width < 200 {
		width = f.Bounds().Width
	}

	// 会话内搜索：命中判定只认纯文本（大小写不敏感），不解析 Markdown。
	q := ""
	if p.searchOn {
		q = strings.ToLower(strings.TrimSpace(p.searchField.Text()))
	}
	vis := make([]int, 0, len(msgs))
	for i := range msgs {
		if q == "" || strings.Contains(strings.ToLower(msgs[i].Text+" "+msgs[i].Tool+" "+msgs[i].Summary), q) {
			vis = append(vis, i)
		}
	}
	p.searchHits = len(vis)
	if p.searchOn {
		p.drawSearchBar(f, env)
	}
	if len(vis) == 0 && len(msgs) > 0 {
		f.Row().FillWidth().Pad(16).Children(func() {
			f.Text("没有匹配「" + p.searchField.Text() + "」的消息").
				FontSize(t.Fonts.Secondary).Color(t.LabelCaption)
		})
		return
	}

	// 每个会话一个滚动 id：否则切会话时滚动位置会串（viva flex/scroll.go:174）。
	scrID := env.Tree.ID(env.Feat, "msgs-"+env.Data.Cur)
	var scrollState *flex.ScrollState
	if env.Scroll != nil {
		scrollState = env.Scroll(scrID)
	}

	// Turn 导航只从当前已加载且带真实身份的消息段生成；搜索态不把过滤后的
	// 局部内容冒充完整 Turn。锚点来自上一帧真实布局，未加载历史没有入口。
	var turns []loadedTurn
	if q == "" {
		turns = buildLoadedTurns(env.Data, vis)
	}
	p.drawTurnNavigation(f, env, scrID, turns)
	p.turnBoxes = map[string]flex.Box{}
	starts := map[int]loadedTurn{}
	ends := map[int]loadedTurn{}
	for _, turn := range turns {
		starts[turn.start] = turn
		ends[turn.end] = turn
	}

	// 内容列必须**显式给宽度**：ScrollArea 里用 FillWidth 时宽度算不出来，
	// 元素会被推到父容器外面（踩过两次：一次是消息画到侧栏上，一次是现在这样）。
	// 用百分比而不是像素：像素得先知道列宽，而槽位里拿不到。
	p.bodyBox = f.ScrollArea(scrID).WidthPercent(100).Grow(1).MinHeight(160).Gap(6).Follow(true).Clip().Children(func() {
		f.Column().WidthPercent(100).Gap(6).Children(func() {
			if env.Data.Truncated {
				f.Row().WidthPercent(100).Pad(8).Children(func() {
					f.Text("更早内容未加载 · loadOlder 未提供").FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
				})
			}
			p.drawStreaming(f, env)

			// 视口虚拟化：先算每条的估算高度和累计位置，视口外的只放 Slot
			// ——不建 Text 就不会生成 GPU 文本纹理（99.1 → 72.5 MB）。
			w0 := width - 40
			heights := make([]float32, len(vis))
			offs := make([]float32, len(vis))
			var acc float32
			for k, i := range vis {
				heights[k] = p.estimateMsgHeight(msgs[i], w0, messageRenderKey(env, msgs[i], i))
				offs[k] = acc
				acc += heights[k] + 6
			}

			var vpOffset, vpHeight float32
			if scrollState != nil {
				vpOffset = scrollState.Offset
			}
			vpHeight = f.Bounds().Height - 220
			if vpHeight < 240 {
				vpHeight = 240
			}
			const overscan = 800.0

			for k, i := range vis {
				if turn, ok := starts[i]; ok {
					p.drawTurnHeader(f, env, turn)
				}
				if offs[k]+heights[k] < vpOffset-overscan || offs[k] > vpOffset+vpHeight+overscan {
					f.Slot().Height(heights[k])
				} else {
					p.drawMessage(f, env, i, width)
				}
				if turn, ok := ends[i]; ok {
					p.drawTurnFooter(f, env, turn)
				}
			}
			p.drawTurnEvents(f, env)
			p.drawOutgoing(f, env)
		})
	})

	p.bodyReady = true
	// Follow(true) 只在用户明确位于底部时跟随；用户回看后新帧不会改写 Offset。
	// 回底入口只在确有内容且当前非底部时出现，点击才显式恢复 Follow。
	hasContent := len(msgs) > 0 || env.Data.Stream != "" || len(env.Data.Outgoing) > 0 || len(env.Data.TurnEvents) > 0
	if hasContent && scrollState != nil && !scrollState.AtEnd() {
		f.Row().WidthPercent(100).Justify(flex.End).PadY(2).Children(func() {
			if f.Button(env.Tree.ID(env.Feat, "return-bottom-"+env.Data.Cur), "回到底部").Small().Clicked() {
				scrollState.ScrollToEnd()
			}
		})
	}
}

// drawStreaming 画「正在流式输出」的那一条。
//
// 与定稿共用 layoutMarkdown / MarkdownView，不再把增量当纯文本 Wrap。
// 缓存下标故意取 len(Msgs)：done 后助手正文正好落在这个下标，同文本同宽度
// 无需重排，未闭合围栏也不会在流式转定稿时换一种解析路径。
func (p *Plugin) drawStreaming(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	if env.Data.Stream == "" {
		return
	}
	width := env.MainWidth
	if width < 200 {
		width = f.Bounds().Width
	}
	streamIndex := len(env.Data.Msgs)
	h := p.layoutMarkdown(env.Canvas, streamIndex, env.Data.Stream, width-40)
	f.Column().FillWidth().Pad(8).Radius(t.Radii.MD).Children(func() {
		f.Row().Gap(6).Items(flex.Center).Children(func() {
			f.Text("助手").FontSize(t.Fonts.Tiny).Color(t.LabelTertiary)
			f.Spinner()
		})
		p.slots = append(p.slots, mdSlot{idx: streamIndex, key: env.Data.Cur + "\x00stream", box: f.Slot().Height(h)})
	})
}

// msgKind 是一条消息该走哪个绘制分支。
//
// 为什么要有这个名字：**流式 reasoning 落进 Msgs 之后，角色同样是 thinking**
// （见 internal/apiclient.Stream.Messages / Merge）。以前只有 session.load
// 回来的历史里有这个角色，于是"流式期间的思考块"其实没有绘制路径——
// 收得到、存得下，但画不出来。现在两条路合成一条，这里就是那条路的入口。
type msgKind int

const (
	kindAssistant msgKind = iota
	kindThinking
	kindTool
	kindUser
)

// roleKind 把消息角色翻成绘制分支。不认识的角色一律当助手正文（降级，不丢内容）。
func roleKind(role string) msgKind {
	switch role {
	case "thinking":
		return kindThinking
	case "tool":
		return kindTool
	case "user":
		return kindUser
	default:
		return kindAssistant
	}
}

// drawMessage 画一条消息。
//
// 视觉规矩照官方：**不要给每条消息套一个带边框的卡片**（官方是平铺的），
// 只有用户消息有气泡（`--dsw-specific-bubble`，圆角 XL），工具调用是一行
// 淡底色的小卡（圆角 SM）。
func (p *Plugin) drawMessage(f *flex.Frame, env *pluginapi.Env, i int, width float32) {
	t := p.theme
	msg := env.Data.Msgs[i]
	key := messageRenderKey(env, msg, i)

	switch roleKind(msg.Role) {
	case kindThinking:
		p.drawThinking(f, env, msg, key)
		p.drawMessageMeta(f, env, msg, key)

	case kindTool:
		p.drawTool(f, env, msg, key)
		p.drawMessageMeta(f, env, msg, key)

	case kindUser:
		// 用户消息：右对齐气泡（官方 .LOmM4q_userStack：
		//   max-width = min(chat-content-width × .702, 82%)，flex-column + align-items:flex-end，gap 8
		//   气泡本体 .bubble：radius xl、specific-bubble 底、正文 14/22）
		// 外观交给 prims.Bubble；这里只负责"右对齐"和"宽度上限"两件事。
		f.Row().WidthPercent(100).Justify(flex.End).Children(func() {
			// 官方的 82%/748px 是相对**会话内容列**算的，而槽位里拿不到列宽；
			// viva 的文字会自然换行，气泡按内容宽度收缩，所以不需要像素上限（见验收说明）。
			p.prims.Bubble(f, func() {
				// 用户粘进来的凭据在这里遮掉（见 mask.go）；**只遮显示**，
				// 复制与落盘的仍是原文。
				f.Wrap(userDisplayText(msg)).FontSize(t.Fonts.Body)
			})
		})
		p.drawMessageMeta(f, env, msg, key)

	default:
		// 助手消息：平铺，老消息降级成纯文本（LOD）。
		if i < len(env.Data.Msgs)-markdownBudget {
			f.Column().WidthPercent(100).Children(func() {
				f.Wrap(msg.Text).FontSize(t.Fonts.Secondary).Color(t.LabelSecondary)
			})
			p.drawMessageMeta(f, env, msg, key)
			return
		}
		w := width - 40
		h := p.layoutMarkdown(env.Canvas, i, msg.Text, w)
		p.slots = append(p.slots, mdSlot{idx: i, key: key, box: f.Slot().Height(h)})
		p.drawActions(f, env, msg, i, i == len(env.Data.Msgs)-1, key)
		p.drawMessageMeta(f, env, msg, key)
	}
}

// drawActions 画消息下面那排小操作。
//
// 为什么要有它：Ctrl+Shift+C / Ctrl+R 是看不见的快捷键，不给可见入口等于没做。
// 只在"有正文的助手消息"下面画，而且是 Ghost 小按钮——它不该抢正文的注意力。
//
// index 是这条消息在当前消息列表里的下标：反馈记录按它归属（见 feedback.go）。
func (p *Plugin) drawActions(f *flex.Frame, env *pluginapi.Env, msg apiclient.Message, index int, last bool, key string) {
	t := p.theme
	if strings.TrimSpace(msg.Text) == "" {
		return
	}
	text := msg.Text
	// 官方 .cVfUIq_actions / .xMeOpW_actions：margin-top 16（助手）/4（用户），
	// margin-left:-6 抵消 Ghost 按钮自身的内边距，视觉上贴左。
	marginTop := float32(4)
	if msg.Role == "assistant" {
		marginTop = 16
	}
	f.Row().WidthPercent(100).Items(flex.Center).MarginSides(marginTop, 0, 0, -6).Children(func() {
		if env.Act.Copy != nil && f.Button(env.Tree.ID(env.Feat, "copy-"+key), "复制").
			Ghost().Small().Clicked() {
			env.Act.Copy(text)
		}
		// 👍/👎 夹在「复制」和「重试」之间——官方图标动作行的次序就是
		// copy → feedback → branch，这里照摆。
		p.drawFeedback(f, env, index)
		if last && env.Act.Retry != nil && f.Button(env.Tree.ID(env.Feat, "retry-"+key), "重试").
			Ghost().Small().Clicked() {
			env.Act.Retry()
		}
	})
	_ = t
}

// drawThinking 画一条思考块：默认收起，点一下展开。
//
// 官方也是这个交互（折叠的思考），但官方把它做得更轻：一行灰字 + 一个小箭头。
// 这里保持同样的克制——收起时只占一行，不抢正文的注意力。
func (p *Plugin) drawThinking(f *flex.Frame, env *pluginapi.Env, msg apiclient.Message, key string) {
	t := p.theme
	open := p.expanded[key] || (p.ExpandAllThinking && !p.collapsedOnce[key])

	arrow := "▸"
	if open {
		arrow = "▾"
	}
	label := "思考 · " + humanRunes(len([]rune(msg.Text))) + " 字"

	// 官方折叠行（如 .LOmM4q_compactionRow）：高 24、radius sm、左侧 16px 展开位、
	// 标题 13px。高度用 MinHeight + PadY 撑，不用 Height(x).Shrink(0)——那会塌成 0 高。
	if f.Row().WidthPercent(100).MinHeight(24).PadX(4).Radius(t.Radii.SM).
		Items(flex.Center).Gap(6).HoverBackground(t.Hover).
		Hit(env.Tree.ID(env.Feat, "think-"+key), "").Children(func() {
		// 左侧 16px 展开位（和工具行同一个位置，视线不用跳）
		f.Row().Width(16).Items(flex.Center).Children(func() {
			f.Text(arrow).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
		})
		f.Text(label).FontSize(t.Fonts.Secondary).Color(t.LabelCaption).Ellipsis()
	}).Clicked() {
		p.expanded[key] = !open
		if p.ExpandAllThinking {
			p.collapsedOnce[key] = open
		}
	}

	if !open {
		return
	}
	// 展开态：浅底盒子，不描边（官方 callRow / compactionBody 都没有边框色）。
	f.Column().WidthPercent(100).Pad(8).Radius(t.Radii.SM).Background(t.BgLayer1).
		Children(func() {
			f.Wrap(msg.Text).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		})
}

// humanRunes 把字数压成「812 / 1.2k」。
func humanRunes(n int) string {
	if n < 1000 {
		return itoa(n)
	}
	return itoa(n/1000) + "." + itoa((n%1000)/100) + "k"
}

// itoa 避免为一个小数字 import strconv。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 && i > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg && i > 0 {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// drawTool 画一次工具调用。
//
// **画法不在这里**：工具卡是独立部件 ds-tool（plugins/dstool），它按
// `Card.Tool` 分派——bash 画命令 + 退出码 + 输出预览，read/edit/glob/grep
// 画路径/模式。消息流只做一件不属于它的事：把一条 `apiclient.Message`
// 翻译成工具卡认识的 Card。
//
// 这个切分是这一轮的重点：改某一种工具的长相，不用碰消息流一行代码
// （也不用再跑一遍消息流的截图）。
func (p *Plugin) drawTool(f *flex.Frame, env *pluginapi.Env, msg apiclient.Message, key string) {
	p.toolCards().Render(f, env, dstool.Card{
		// dstool 当前以 int 作为内部展开键；MessageKey 在本插件内映射成
		// 唯一的会话局部 int，避免 Seq=0 或跨会话同 Seq 共用状态。
		Seq:     p.toolRenderID(key),
		Tool:    msg.Tool,
		Args:    msg.Args,
		Summary: msg.Summary,
		Status:  toolStatus(msg, key, env.Data),
		View:    msg.View,
	})
}

// toolCards 取工具卡渲染器。
//
// 正常路径一定拿得到：ds-tool 提供 `ui.tool-card`，壳的 blank import 保证它
// 被装配（日志 `插件已装配 id=ds-tool`）。**没有写进 Requires** 是刻意的：
// 有两个既有测试把 ds-conversation 单独装进一个没有 ds-tool 的 kernel
// （dsshortcuts/overlay_test.go、dstopbar/frame_interaction_test.go），
// 列进 Requires 会让它们装配失败，而那两个文件不在本轮边界内。
//
// 兜底不是第二套画法：用的还是 dstool 自己的渲染器，并复用本插件已注入的
// ds-core 图元；只有 prims 也为 nil 时徽标才隐藏。**画法只有一份**。
func (p *Plugin) toolCards() dstool.Service {
	if p.tools == nil {
		p.tools = dstool.New(p.prims)
	}
	return p.tools
}

// layoutMarkdown 排版一条消息的 Markdown，返回高度。
//
// MarkdownView.Layout 很贵：只有文本或宽度变了才重排。
func (p *Plugin) layoutMarkdown(canvas *ui.Canvas, i int, text string, width float32) float32 {
	if p.codeBlocks == nil {
		p.codeBlocks = map[int][]messageCode{}
	}
	if p.fileLinks == nil {
		p.fileLinks = map[int]map[string]string{}
	}
	for len(p.docs) <= i {
		p.docs = append(p.docs, markdown.Document{})
		p.views = append(p.views, components.MarkdownView{})
		p.texts = append(p.texts, "")
		p.widths = append(p.widths, -1)
		p.heights = append(p.heights, 0)
	}
	if p.texts[i] != text || p.widths[i] != width {
		p.texts[i] = text
		p.widths[i] = width
		rewritten, files := rewriteExplicitFileLinks(text)
		p.fileLinks[i] = files
		p.docs[i].Parse(rewritten)
		p.views[i].Layout(canvas, components.MarkdownStyle{
			Size: p.theme.Fonts.Body, Highlight: mdlite.Highlight,
		}, &p.docs[i], width)
	}
	p.rebuildCodeBlocks(canvas, i, width)
	p.heights[i] = p.views[i].Height() + p.markdownExtraHeight(i)
	return p.heights[i]
}

// afterDraw 是覆画阶段：把排好版的 Markdown 画到占位上。
func (p *Plugin) afterDraw(canvas *ui.Canvas, env *pluginapi.Env) {
	// 只把布局后的矩形值交给下一帧 Turn 导航；flex.Box 自身随 Frame 复用，
	// 跨帧保存会在下一帧索引到错误节点。
	if p.bodyReady {
		p.bodyRect = p.bodyBox.Rect()
	}
	p.turnAnchors = make(map[string]ui.Rect, len(p.turnBoxes))
	for key, box := range p.turnBoxes {
		p.turnAnchors[key] = box.Rect()
	}
	if len(p.slots) == 0 {
		return
	}
	bounds := env.Bounds
	// 裁剪区必须是**消息流滚动区自己的视口**，不能用 env.Bounds（整窗）。
	// 踩过的坑：ScrollArea 里 slot 的 Y 是屏幕坐标（滚出去就是负数），
	// 用整窗当 Clip 等于不裁——排在上方的 Markdown 会画到顶栏和输入区上面，
	// 「顶栏消失」的真相就是它被消息盖住了（实测 view=(251,286 489x382)）。
	view := p.bodyBox.Rect()
	for _, slot := range p.slots {
		if slot.idx >= len(p.views) {
			continue
		}
		r := slot.box.Rect()
		if r.Width <= 0 || r.Height <= 0 {
			continue
		}
		clip := ui.Rect{
			X:      r.X,
			Y:      view.Y,
			Width:  view.X + view.Width - r.X - 24,
			Height: view.Height,
		}
		if view.Height <= 0 || view.Width <= 0 {
			// 兜底：拿不到视口就退回整窗（宁可多画，不可不画）。
			clip.X, clip.Y = r.X, bounds.Y
			clip.Width, clip.Height = bounds.Width-r.X-24, bounds.Height
		}
		p.paintMarkdown(canvas, env, slot, r, clip)
	}
}

// markdownLook 由主题算一次。
//
// MarkdownLookFor 不设 Code 数组（= 所有 token 都画成正文色），所以
// Highlight 返回 class 也没颜色。这里补上主题里的六个语法色。
func (p *Plugin) markdownLook() components.MarkdownLook {
	if p.looked {
		return p.look
	}
	look := components.MarkdownLookFor(dscore.VivaTheme(p.theme))
	look.Code[components.CodeComment-1] = p.theme.CodeComment
	look.Code[components.CodeString-1] = p.theme.CodeString
	look.Code[components.CodeNumber-1] = p.theme.CodeNumber
	look.Code[components.CodeKeyword-1] = p.theme.CodeKeyword
	look.Code[components.CodeFunction-1] = p.theme.CodeFunction
	look.Code[components.CodeVariable-1] = p.theme.CodeVariable
	p.look, p.looked = look, true
	return look
}

// estimateMsgHeight 估算一条消息占多高（只看字符数与换行，不排版）。
//
// 估算偏大不会错位，偏小会让滚动跳——所以宁可大一点。
func (p *Plugin) estimateMsgHeight(m apiclient.Message, width float32, key string) float32 {
	const lineH = 22.0
	const gapPad = 28.0
	if width < 120 {
		width = 120
	}
	if m.Role == "thinking" {
		open := p.expanded[key] || (p.ExpandAllThinking && !p.collapsedOnce[key])
		if !open {
			return 26 // 收起时只占一行
		}
		// 展开时必须按真实字数估：估小了滚动位置会错，内容被虚拟化掉变成白屏
		// （踩过：这里恒返回 26，展开后整块主区是空的）。
		per := width / (13 * 0.62)
		if per < 8 {
			per = 8
		}
		lines := 0
		for _, para := range strings.Split(m.Text, nlChar) {
			n := float64(len([]rune(para)))
			l := int(math.Ceil(n / float64(per)))
			if l < 1 {
				l = 1
			}
			lines += l
		}
		return float32(float64(lines)*18 + 40)
	}
	if m.Role == "tool" {
		// 工具卡占多高由 ds-tool 自己说了算：它知道这种工具展开后画几行。
		return p.toolCards().Height(p.toolRenderID(key), m.View)
	}
	perLine := float64(width) / (14 * 0.62)
	if perLine < 8 {
		perLine = 8
	}
	// 量的是**画出来的那份**。遮罩长度不变（见 secretmask/maskOne），
	// 所以这一步现在不改结果；留着是因为"量的和画的是同一份"这条约束
	// 不该靠"恰好等长"来维持——哪天遮罩改成塌缩，这里就该跟着错位。
	text := m.Text
	if m.Role == "user" {
		text = userDisplayText(m)
	}
	lines := 0
	for _, para := range strings.Split(text, "\n") {
		n := float64(len([]rune(para)))
		l := int(math.Ceil(n / perLine))
		if l < 1 {
			l = 1
		}
		lines += l
	}
	if m.Role == "assistant" {
		return float32(float64(lines)*lineH + gapPad + 26) // 多一排操作按钮
	}
	return float32(float64(lines)*lineH + gapPad)
}

// HandleKey 实现 pluginapi.KeyHandler：
//
//	Esc            拒绝当前审批（有审批时）
//	Ctrl+Shift+C   复制最后一条助手回复
//	Ctrl+R         重发上一条用户消息
func (p *Plugin) HandleKey(ev ui.KeyEvent) bool {
	switch {
	case isKey(ev, "f") && ev.Modifiers == ui.ModifierControl:
		p.searchOn = !p.searchOn
		if !p.searchOn {
			p.searchField.SetText("")
		}
		return true

	case ev.Key == ui.KeyEscape && ev.Modifiers == 0:
		if p.searchOn {
			p.searchOn = false
			p.searchField.SetText("")
			return true
		}
		if p.lastAct.Deny == nil {
			return false
		}
		p.lastAct.Deny()
		return true

	case isKey(ev, "r") && ev.Modifiers == ui.ModifierControl:
		if p.lastAct.Retry == nil {
			return false
		}
		p.lastAct.Retry()
		return true

	case isKey(ev, "c") && ev.Modifiers == ui.ModifierControl|ui.ModifierShift:
		if p.lastAct.Copy == nil {
			return false
		}
		if text := p.lastAssistant(); text != "" {
			p.lastAct.Copy(text)
			return true
		}
	}
	return false
}

// isKey 判断这是不是某个字母键。
//
// viva 里字母键没有 KeyXxx 常量：Key 只覆盖功能键，可打印键靠 ev.Text 识别
// （Text 是"不受修饰键影响"的字符，所以 Ctrl+Shift+C 也拿得到 "c"）。
func isKey(ev ui.KeyEvent, letter string) bool {
	return strings.EqualFold(ev.Text, letter) || strings.EqualFold(ev.Typed, letter)
}

// lastAssistant 取最后一条有正文的助手消息。
func (p *Plugin) lastAssistant() string {
	for i := len(p.lastMsgs) - 1; i >= 0; i-- {
		if p.lastMsgs[i].Role == "assistant" && strings.TrimSpace(p.lastMsgs[i].Text) != "" {
			return p.lastMsgs[i].Text
		}
	}
	return ""
}

var (
	_ pluginapi.Plugin     = (*Plugin)(nil)
	_ pluginapi.Panels     = (*Plugin)(nil)
	_ pluginapi.KeyHandler = (*Plugin)(nil)
	_ pluginapi.Fields     = (*Plugin)(nil)
)
