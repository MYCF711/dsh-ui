// Package dsconversation 是「主区消息流」这个部件的移植 —— 界面里最大的一块。
//
// 官方对应物：`dsh-client-ui-chat`（消息流 / 代码块 / 输入区外壳）+
// `dsh-client-ui-conversation`（一轮对话的折叠与分隔）+ `dsh-client-ui-approval`（审批）。
//
// 这块代码原来长在 `cmd/app/main.go` 里，直接摸 model、直接拼锁。搬到这里之后
// 它只认 `pluginapi.Env`：只读快照 + Action，没有锁、没有客户端、没有全局变量。
//
// 两个阶段（viva 的绘制顺序不受调用方控制，所以必须分开）：
//   Draw      布局：算出每条消息的高度，把视口外的消息只留 Slot（不建文本纹理）
//   AfterDraw 覆画：MarkdownView.Paint 必须等布局定下来才知道画在哪
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
	// lastWidth 是本帧内容列宽度（工具卡要按它定宽）。
	lastWidth float32
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
		{ID: "approval", Slot: pluginapi.SlotMainBody, Order: 30, Draw: p.drawApproval},
	}
}

// draw 是布局阶段。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	msgs := env.Data.Msgs
	p.slots = p.slots[:0]

	if len(msgs) == 0 && env.Data.Stream == "" {
		f.Row().FillWidth().Pad(24).Children(func() {
			f.Text("← 左边选一个会话").FontSize(t.Fonts.Body).Color(t.LabelTertiary)
		})
		return
	}

	// 注意：槽位里 f.Bounds() 拿到的是**整窗**宽度（实测 1180），不是本列宽度。
	// 所以这里一律用百分比定宽，别拿 Bounds 去算列宽——算了就会把内容推到窗口外。
	width := f.Bounds().Width // 只用来估高度，不用于布局

	// 每个会话一个滚动 id：否则切会话时滚动位置会串（viva flex/scroll.go:174）。
	scrID := env.Tree.ID(env.Feat, "msgs-"+env.Data.Cur)
	// 内容列必须**显式给宽度**：ScrollArea 里用 FillWidth 时宽度算不出来，
	// 元素会被推到父容器外面（踩过两次：一次是消息画到侧栏上，一次是现在这样）。
	// 用百分比而不是像素：像素得先知道列宽，而槽位里拿不到。
	f.ScrollArea(scrID).WidthPercent(100).Grow(1).MinHeight(160).Gap(6).Follow(true).Clip().Children(func() {
		f.Column().WidthPercent(100).Gap(6).Children(func() {
			p.drawStreaming(f, env)

			// 视口虚拟化：先算每条的估算高度和累计位置，视口外的只放 Slot
			// ——不建 Text 就不会生成 GPU 文本纹理（99.1 → 72.5 MB）。
			w0 := width - 40
			heights := make([]float32, len(msgs))
			offs := make([]float32, len(msgs))
			var acc float32
			for i := range msgs {
				heights[i] = estimateMsgHeight(msgs[i], w0)
				offs[i] = acc
				acc += heights[i] + 6
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

			for i := range msgs {
				if offs[i]+heights[i] < vpOffset-overscan || offs[i] > vpOffset+vpHeight+overscan {
					f.Slot().Height(heights[i])
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

// drawMessage 画一条消息。
//
// 视觉规矩照官方：**不要给每条消息套一个带边框的卡片**（官方是平铺的），
// 只有用户消息有气泡（`--dsw-specific-bubble`，圆角 XL），工具调用是一行
// 淡底色的小卡（圆角 SM）。
func (p *Plugin) drawMessage(f *flex.Frame, env *pluginapi.Env, i int, width float32) {
	t := p.theme
	msg := env.Data.Msgs[i]

	switch {
	case msg.Role == "tool":
		p.drawTool(f, env, msg)

	case msg.Role == "user":
		// 用户消息：右对齐气泡。
		f.Row().WidthPercent(100).Justify(flex.End).Children(func() {
			f.Column().MaxWidthPercent(78).Pad(10).Radius(t.Radii.XL).
				Background(t.Bubble).Children(func() {
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
	}
}

// drawTool 画一次工具调用。
//
// 这里就是「一切皆插件」的显示契约落点：工具**不渲染自己**，只在
// `msg.View` 里声明 Kind（card / table / plain），界面按 Kind 选渲染器。
// 没声明就退化成一行纯文本——不认识 == 没声明。
func (p *Plugin) drawTool(f *flex.Frame, env *pluginapi.Env, msg apiclient.Message) {
	t := p.theme
	f.Column().WidthPercent(100).Children(func() {
		f.Column().FillWidth().Pad(8).Radius(t.Radii.SM).
			Background(t.BgLayer1).Border(1, t.BorderL1).Gap(4).Children(func() {
			if msg.View != nil {
				p.drawView(f, msg.View)
				return
			}
			f.Row().Gap(8).Items(flex.Center).Children(func() {
				f.Text(msg.Tool).Mono().Bold().FontSize(t.Fonts.Caption).Color(t.LabelSecondary)
				f.Text(msg.Args).Mono().FontSize(t.Fonts.Tiny).Color(t.LabelCaption).Ellipsis()
			})
			if msg.Summary != "" {
				f.Text(msg.Summary).FontSize(t.Fonts.Caption).Color(t.LabelTertiary).MaxLines(3)
			}
		})
	})
}

// drawView 按 ViewSpec 的 Kind 选渲染器。
func (p *Plugin) drawView(f *flex.Frame, v *apiclient.ViewSpec) {
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
		p.drawTable(f, v)
	case "plain":
		f.Text(v.Body).Mono().FontSize(t.Fonts.Tiny).Color(t.LabelTertiary).MaxLines(12)
	case "card", "":
		p.drawCard(f, v)
	default:
		// 不认识的 Kind：退化成正文，别丢内容也别崩。
		f.Text(v.Body).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
	}
}

// drawCard 是 Kind=card：一组键值。
func (p *Plugin) drawCard(f *flex.Frame, v *apiclient.ViewSpec) {
	t := p.theme
	for _, fv := range v.Fields {
		f.Row().FillWidth().Gap(8).Children(func() {
			f.Text(fv.Key).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
			// Value 必须 Grow(1)：不撑满的话 Ellipsis 没有可省略的边界，
			// 长命令会直接画到窗口外（实测：bash 的 command 被切在右边缘）。
			f.Text(fv.Value).Mono().FontSize(t.Fonts.Tiny).Color(t.LabelSecondary).
				Grow(1).Ellipsis()
		})
	}
}

// drawTable 是 Kind=table：列头 + 行。
//
// 行宽必须等于列数；不等就截断到列数（**不许 panic**：这是外部数据）。
func (p *Plugin) drawTable(f *flex.Frame, v *apiclient.ViewSpec) {
	t := p.theme
	if len(v.Columns) == 0 {
		return
	}
	f.Row().FillWidth().Gap(8).Children(func() {
		for _, c := range v.Columns {
			f.Text(c).FontSize(t.Fonts.Tiny).Color(t.LabelCaption).Grow(1).Ellipsis()
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
				f.Text(cell).Mono().FontSize(t.Fonts.Tiny).Color(t.LabelTertiary).Grow(1).Ellipsis()
			}
		})
	}
	if len(v.Rows) > maxRows {
		f.Text(fmt.Sprintf("… 另有 %d 行", len(v.Rows)-maxRows)).
			FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
	}
}

// drawApproval 画审批气泡：先命令、后模型自述（并明确标不可信）。
func (p *Plugin) drawApproval(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	ap := env.Data.Approval
	if ap == nil {
		return
	}
	f.Column().FillWidth().Pad(12).Radius(t.Radii.LG).
		Background(t.BgLayer2).Border(1, t.BorderL2).Gap(8).Children(func() {
		f.Row().Gap(8).Items(flex.Center).Children(func() {
			f.Text("需要审批").Bold().FontSize(t.Fonts.Caption).Color(t.LabelPrimary)
			f.Text(ap.Tool).Mono().FontSize(t.Fonts.Tiny).Color(t.LabelTertiary)
			f.Spacer()
		})
		f.Text(ap.Cmd).Mono().FontSize(t.Fonts.Caption).Color(t.LabelSecondary)
		if ap.Reason != "" {
			f.Text("模型自述（不可信）："+ap.Reason).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
		}
		f.Row().Gap(8).Justify(flex.End).Children(func() {
			if f.Button(env.Tree.ID(env.Feat, "deny"), "拒绝").Outline().Small().Clicked() {
				if env.Act.Deny != nil {
					env.Act.Deny()
				}
			}
			if f.Button(env.Tree.ID(env.Feat, "allow"), "允许").Small().Clicked() {
				if env.Act.Allow != nil {
					env.Act.Allow()
				}
			}
		})
	})
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
	for _, slot := range p.slots {
		if slot.idx >= len(p.views) {
			continue
		}
		r := slot.box.Rect()
		if r.Width <= 0 || r.Height <= 0 {
			continue
		}
		p.views[slot.idx].Paint(canvas, components.MarkdownPaint{
			Look: look,
			X:    r.X,
			Y:    r.Y,
			// Clip 右边界要到窗口边：表格/代码块会比卡片宽，只裁到 r.Width
			// 会画到侧栏上。右边留 24px 给滚动条，否则会看见贴边碎片。
			Clip: ui.Rect{X: r.X, Y: bounds.Y, Width: bounds.Width - r.X - 24, Height: bounds.Height},
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
func estimateMsgHeight(m apiclient.Message, width float32) float32 {
	const lineH = 22.0
	const gapPad = 28.0
	if width < 120 {
		width = 120
	}
	if m.Role == "tool" {
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
	return float32(float64(lines)*lineH + gapPad)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var (
	_ pluginapi.Plugin = (*Plugin)(nil)
	_ pluginapi.Panels = (*Plugin)(nil)
)
