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
	"fmt"
	"math"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
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
	box flex.Box
}

// dbgOnce 只打一次布局诊断（临时）。
var dbgOnce = true

// Plugin 实现 pluginapi.Plugin / Panels。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives

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
	bodyBox flex.Box
	// collapsedOnce 记录调试开关下被用户手动收起过的，避免压不住点击。
	collapsedOnce map[int]bool
	// lastAct / lastMsgs / lastApproval 是绘制期快照，按键时要用。
	lastAct  pluginapi.Actions
	lastMsgs []apiclient.Message
	// expanded 记录哪些思考块被展开（键是消息 seq，切会话后依然有效）。
	// 缺省全收起：真实语料里思考比正文多八倍，默认展开会把消息流淹没。
	expanded map[int]bool
	// toolOpen 记录哪些工具调用被展开（官方默认收起，只留一行）。
	toolOpen map[int]bool
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
	p.lastAct = env.Act
	p.lastMsgs = env.Data.Msgs
	if p.expanded == nil {
		p.expanded = map[int]bool{}
	}
	if p.collapsedOnce == nil {
		p.collapsedOnce = map[int]bool{}
	}
	if !p.initialized {
		p.initialized = true
		if p.SearchOpen {
			p.searchOn = true
			p.searchField.SetText(p.SearchQuery)
		}
	}

	if len(msgs) == 0 && env.Data.Stream == "" {
		f.Row().FillWidth().Pad(24).Children(func() {
			f.Text("← 左边选一个会话").FontSize(t.Fonts.Body).Color(t.LabelTertiary)
		})
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
	if len(vis) == 0 {
		f.Row().FillWidth().Pad(16).Children(func() {
			f.Text("没有匹配「" + p.searchField.Text() + "」的消息").
				FontSize(t.Fonts.Secondary).Color(t.LabelCaption)
		})
		return
	}

	// 每个会话一个滚动 id：否则切会话时滚动位置会串（viva flex/scroll.go:174）。
	scrID := env.Tree.ID(env.Feat, "msgs-"+env.Data.Cur)
	// 内容列必须**显式给宽度**：ScrollArea 里用 FillWidth 时宽度算不出来，
	// 元素会被推到父容器外面（踩过两次：一次是消息画到侧栏上，一次是现在这样）。
	// 用百分比而不是像素：像素得先知道列宽，而槽位里拿不到。
	p.bodyBox = f.ScrollArea(scrID).WidthPercent(100).Grow(1).MinHeight(160).Gap(6).Follow(true).Clip().Children(func() {
		f.Column().WidthPercent(100).Gap(6).Children(func() {
			p.drawStreaming(f, env)

			// 视口虚拟化：先算每条的估算高度和累计位置，视口外的只放 Slot
			// ——不建 Text 就不会生成 GPU 文本纹理（99.1 → 72.5 MB）。
			w0 := width - 40
			heights := make([]float32, len(vis))
			offs := make([]float32, len(vis))
			var acc float32
			for k, i := range vis {
				heights[k] = p.estimateMsgHeight(msgs[i], w0)
				offs[k] = acc
				acc += heights[k] + 6
			}

			var vpOffset, vpHeight float32
			if env.Scroll != nil {
				if st := env.Scroll(scrID); st != nil {
					vpOffset = st.Offset
				}
			}
			vpHeight = f.Bounds().Height - 220
			if vpHeight < 240 {
				vpHeight = 240
			}
			const overscan = 800.0

			for k, i := range vis {
				if offs[k]+heights[k] < vpOffset-overscan || offs[k] > vpOffset+vpHeight+overscan {
					f.Slot().Height(heights[k])
					continue
				}
				p.drawMessage(f, env, i, width)
			}
		})
	})
}

// drawStreaming 画「正在流式输出」的那一条。
func (p *Plugin) drawStreaming(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	if env.Data.Stream == "" {
		return
	}
	f.Column().FillWidth().Pad(8).Radius(t.Radii.MD).Children(func() {
		f.Row().Gap(6).Items(flex.Center).Children(func() {
			f.Text("助手").FontSize(t.Fonts.Tiny).Color(t.LabelTertiary)
			f.Spinner()
		})
		f.Wrap(env.Data.Stream).FontSize(t.Fonts.Body)
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

	switch roleKind(msg.Role) {
	case kindThinking:
		p.drawThinking(f, env, msg)

	case kindTool:
		p.drawTool(f, env, msg)

	case kindUser:
		// 用户消息：右对齐气泡（官方 .LOmM4q_userStack：
		//   max-width = min(chat-content-width × .702, 82%)，flex-column + align-items:flex-end，gap 8
		//   气泡本体 .bubble：radius xl、specific-bubble 底、正文 14/22）
		// 外观交给 prims.Bubble；这里只负责"右对齐"和"宽度上限"两件事。
		f.Row().WidthPercent(100).Justify(flex.End).Children(func() {
			// 官方的 82%/748px 是相对**会话内容列**算的，而槽位里拿不到列宽；
			// viva 的文字会自然换行，气泡按内容宽度收缩，所以不需要像素上限（见验收说明）。
			p.prims.Bubble(f, func() {
				f.Wrap(msg.Text).FontSize(t.Fonts.Body)
			})
		})

	default:
		// 助手消息：平铺，老消息降级成纯文本（LOD）。
		if i < len(env.Data.Msgs)-markdownBudget {
			f.Column().WidthPercent(100).Children(func() {
				f.Wrap(msg.Text).FontSize(t.Fonts.Secondary).Color(t.LabelSecondary)
			})
			return
		}
		w := width - 40
		h := p.layoutMarkdown(env.Canvas, i, msg.Text, w)
		p.slots = append(p.slots, mdSlot{idx: i, box: f.Slot().Height(h)})
		p.drawActions(f, env, msg, i == len(env.Data.Msgs)-1)
	}
}

// drawActions 画消息下面那排小操作。
//
// 为什么要有它：Ctrl+Shift+C / Ctrl+R 是看不见的快捷键，不给可见入口等于没做。
// 只在"有正文的助手消息"下面画，而且是 Ghost 小按钮——它不该抢正文的注意力。
func (p *Plugin) drawActions(f *flex.Frame, env *pluginapi.Env, msg apiclient.Message, last bool) {
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
		if env.Act.Copy != nil && f.Button(env.Tree.ID(env.Feat, "copy-"+itoa(msg.Seq)), "复制").
			Ghost().Small().Clicked() {
			env.Act.Copy(text)
		}
		if last && env.Act.Retry != nil && f.Button(env.Tree.ID(env.Feat, "retry-"+itoa(msg.Seq)), "重试").
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
func (p *Plugin) drawThinking(f *flex.Frame, env *pluginapi.Env, msg apiclient.Message) {
	t := p.theme
	open := p.expanded[msg.Seq] || (p.ExpandAllThinking && !p.collapsedOnce[msg.Seq])

	arrow := "▸"
	if open {
		arrow = "▾"
	}
	label := "思考 · " + humanRunes(len([]rune(msg.Text))) + " 字"

	// 官方折叠行（如 .LOmM4q_compactionRow）：高 24、radius sm、左侧 16px 展开位、
	// 标题 13px。高度用 MinHeight + PadY 撑，不用 Height(x).Shrink(0)——那会塌成 0 高。
	if f.Row().WidthPercent(100).MinHeight(24).PadX(4).Radius(t.Radii.SM).
		Items(flex.Center).Gap(6).HoverBackground(t.Hover).
		Hit(env.Tree.ID(env.Feat, "think-"+itoa(msg.Seq)), "").Children(func() {
		// 左侧 16px 展开位（和工具行同一个位置，视线不用跳）
		f.Row().Width(16).Items(flex.Center).Children(func() {
			f.Text(arrow).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
		})
		f.Text(label).FontSize(t.Fonts.Secondary).Color(t.LabelCaption).Ellipsis()
	}).Clicked() {
		p.expanded[msg.Seq] = !open
		if p.ExpandAllThinking {
			p.collapsedOnce[msg.Seq] = open
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
// summarize 压成一行并截断（工具标题行只有一行，长参数必须收住）。
func summarize(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

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
// 这里就是「一切皆插件」的显示契约落点：工具**不渲染自己**，只在
// `msg.View` 里声明 Kind（card / table / plain），界面按 Kind 选渲染器。
// 没声明就退化成一行纯文本——不认识 == 没声明。
func (p *Plugin) drawTool(f *flex.Frame, env *pluginapi.Env, msg apiclient.Message) {
	t := p.theme
	// 照官方 .V4vHZW_callRow + .BE0jCW_* 的结构：
	//   · 整行只有 border-radius(sm)，**没有边框色**，悬停才有一层浅底；
	//   · 左侧 16px 的 leading 图标位（官方 .BE0jCW_leading），右侧一行
	//     「工具名 · 摘要」，字号 13/行高 24，超长省略；
	//   · 点这一行展开/收起正文（展开体最大高度 min(400px, 50vh)）。
	open := p.toolOpen[msg.Seq]
	width := p.contentWidth(env)

	title := msg.Tool
	if title == "" && msg.View != nil {
		title = msg.View.Title
	}
	summary := msg.Summary
	if summary == "" {
		summary = msg.Args
	}
	if msg.View != nil && msg.View.Badge == "error" {
		summary = "错误：" + summary
	}

	// 标题行：点它展开
	head := f.Row().WidthPercent(100).PadX(4).PadY(2).Radius(t.Radii.SM).
		Items(flex.Center).Gap(6).HoverBackground(t.Hover).
		Hit(env.Tree.ID(env.Feat, "tool-"+itoa(msg.Seq)), "").Children(func() {
		// leading：16px 方框里的展开箭头（官方也是这个位置放 chevron）
		f.Row().Width(16).Items(flex.Center).Children(func() {
			arrow := "▸"
			if open {
				arrow = "▾"
			}
			f.Text(arrow).FontSize(t.Fonts.Tiny).Color(t.LabelSecondary)
		})
		f.Text(title).Mono().FontSize(t.Fonts.Secondary).Color(t.LabelSecondary)
		if summary != "" {
			f.Text("·").FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
			f.Text(summarize(summary, 120)).FontSize(t.Fonts.Secondary).
				Color(t.LabelTertiary).MaxWidth(width - 120).Ellipsis()
		}
	})
	if head.Clicked() {
		p.toolOpen[msg.Seq] = !open
	}

	if !open {
		return
	}
	// 展开体：官方 max-height:min(400px,50vh)，我们按行数近似（不引入滚动区，
	// 这里的展开体本身就是普通盒子，超长由下面的 MaxLines 截断）。
	// 展开体：官方同样是 callRow 的一部分（radius sm、无边框），
	// 底色用最浅一层 BgLayer1 把正文托起来，不描边。
	f.Column().WidthPercent(100).PadX(22).PadY(4).Gap(4).Radius(t.Radii.SM).
		Background(t.BgLayer1).Children(func() {
		if msg.View != nil {
			p.drawView(f, env, msg.View)
			return
		}
		if msg.Args != "" {
			f.Text(msg.Args).Mono().FontSize(t.Fonts.Tiny).Color(t.LabelCaption).MaxLines(6)
		}
		if msg.Summary != "" {
			f.Text(msg.Summary).FontSize(t.Fonts.Caption).Color(t.LabelTertiary).MaxLines(12)
		}
	})
}

// contentWidth 是主区内容可用的最大像素宽。
//
// 为什么不用百分比：槽位里 f.Bounds() 拿到的是整窗宽（实测 1180），
// 百分比也按整窗解析，结果就是卡片内容一路画到窗口边缘、Ellipsis 没有边界。
// 唯一可靠的事实是壳保证了侧栏 280，主区再来回扣 56 的内边距/滚动条。
func (p *Plugin) contentWidth(env *pluginapi.Env) float32 {
	w := env.MainWidth - 40 // 扣掉列内边距和卡片内边距
	if w < 200 {
		w = 200
	}
	return w
}

// drawView 按 ViewSpec 的 Kind 选渲染器。
func (p *Plugin) drawView(f *flex.Frame, env *pluginapi.Env, v *apiclient.ViewSpec) {
	t := p.theme

	// 头：工具名 + 徽标
	if v.Title != "" || v.Badge != "" {
		f.Row().Gap(8).Items(flex.Center).Children(func() {
			f.Text(v.Title).Mono().Bold().FontSize(t.Fonts.Caption).Color(t.LabelSecondary)
			f.Spacer()
			if v.Badge != "" {
				color := t.Success
				if v.Badge == "error" {
					color = t.Error
				}
				p.prims.Badge(f, v.Badge, color)
			}
		})
	}

	switch v.Kind {
	case "table":
		p.drawTable(f, env, v)
	case "plain":
		// 纯文本工具结果只给一眼：12 行会把一条工具卡撑满半屏（实测过），
		// 想看全文应该点开或去 artifacts，而不是在消息流里翻。
		f.Text(v.Body).Mono().FontSize(t.Fonts.Tiny).Color(t.LabelTertiary).MaxLines(6)
	case "card", "":
		p.drawCard(f, env, v)
	default:
		// 不认识的 Kind：退化成正文，别丢内容也别崩。
		f.Text(v.Body).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
	}
}

// drawCard 是 Kind=card：一组键值。
func (p *Plugin) drawCard(f *flex.Frame, env *pluginapi.Env, v *apiclient.ViewSpec) {
	t := p.theme
	cw := p.contentWidth(env)
	for _, fv := range v.Fields {
		f.Row().WidthPercent(100).Gap(8).Children(func() {
			f.Text(fv.Key).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
			// Value 必须 Grow(1)：不撑满的话 Ellipsis 没有可省略的边界，
			// 长命令会直接画到窗口外（实测：bash 的 command 被切在右边缘）。
			f.Text(fv.Value).Mono().FontSize(t.Fonts.Tiny).Color(t.LabelSecondary).
				MaxWidth(cw - 120).Ellipsis()
		})
	}
}

// drawTable 是 Kind=table：列头 + 行。
//
// 行宽必须等于列数；不等就截断到列数（**不许 panic**：这是外部数据）。
func (p *Plugin) drawTable(f *flex.Frame, env *pluginapi.Env, v *apiclient.ViewSpec) {
	t := p.theme
	cw := p.contentWidth(env)
	if len(v.Columns) == 0 {
		return
	}
	f.Row().FillWidth().Gap(8).Children(func() {
		for _, c := range v.Columns {
			f.Text(c).FontSize(t.Fonts.Tiny).Color(t.LabelCaption).
				MaxWidth(cw / float32(len(v.Columns))).Ellipsis()
		}
	})
	const maxRows = 8
	for r := 0; r < len(v.Rows) && r < maxRows; r++ {
		row := v.Rows[r]
		f.Row().FillWidth().Gap(8).Children(func() {
			for c := range v.Columns {
				cell := ""
				if c < len(row) {
					cell = row[c]
				}
				f.Text(cell).Mono().FontSize(t.Fonts.Tiny).Color(t.LabelTertiary).
					MaxWidth(cw / float32(len(v.Columns))).Ellipsis()
			}
		})
	}
	if len(v.Rows) > maxRows {
		f.Text(fmt.Sprintf("… 另有 %d 行", len(v.Rows)-maxRows)).
			FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
	}
}

// layoutMarkdown 排版一条消息的 Markdown，返回高度。
//
// MarkdownView.Layout 很贵：只有文本或宽度变了才重排。
func (p *Plugin) layoutMarkdown(canvas *ui.Canvas, i int, text string, width float32) float32 {
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
		p.docs[i].Parse(text)
		p.heights[i] = p.views[i].Layout(canvas, components.MarkdownStyle{
			Size: p.theme.Fonts.Body, Highlight: mdlite.Highlight,
		}, &p.docs[i], width)
	}
	return p.heights[i]
}

// afterDraw 是覆画阶段：把排好版的 Markdown 画到占位上。
func (p *Plugin) afterDraw(canvas *ui.Canvas, env *pluginapi.Env) {
	if len(p.slots) == 0 {
		return
	}
	look := p.markdownLook()
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
		p.views[slot.idx].Paint(canvas, components.MarkdownPaint{
			Look: look,
			X:    r.X,
			Y:    r.Y,
			// 裁剪右边界用**消息区视口**的右边，不是窗口右边：用整窗的话
			// 宽表格/宽代码会横穿右栏（实测 1180 宽时表格直接画到「轨迹」上面）。
			// 留 24px 给滚动条，否则会看见贴边碎片。
			Clip: clip,
		})
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
func (p *Plugin) estimateMsgHeight(m apiclient.Message, width float32) float32 {
	const lineH = 22.0
	const gapPad = 28.0
	if width < 120 {
		width = 120
	}
	if m.Role == "thinking" {
		open := p.expanded[m.Seq] || (p.ExpandAllThinking && !p.collapsedOnce[m.Seq])
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
		if !p.toolOpen[m.Seq] {
			return 28 // 收起时只有标题一行（官方就是这么一行）
		}
		if m.View != nil {
			// 有结构化显示的工具卡：头 + 若干行
			return float32(50 + 20*(len(m.View.Fields)+min(len(m.View.Rows), 8)))
		}
		return 70
	}
	perLine := float64(width) / (14 * 0.62)
	if perLine < 8 {
		perLine = 8
	}
	lines := 0
	for _, para := range strings.Split(m.Text, "\n") {
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
