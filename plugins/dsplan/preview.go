package dsplan

import (
	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

// 右栏全文的几何：`OFF:PlanPreview.module.css:37-43` 逐条（含 ≤767px 响应式）。
//
//	.preview  { position:relative; height:100%; overflow:auto; padding:20px 24px 40px }
//	.document { font-size:14px; line-height:1.75 }
//	.message  { padding:24px; font-size:14px; color:label-secondary }
const (
	previewPadTop    float32 = 20
	previewPadX      float32 = 24
	previewPadBottom float32 = 40
	// PlanPreview.module.css:41-43：整窗宽度不超过 767px 时的官方值。
	previewCompactViewport float32 = 767
	previewCompactPadTop   float32 = 16
	previewCompactPadX     float32 = 18
	previewCompactPadBot   float32 = 32
	previewFont            float32 = 14
	// `.document{line-height:1.75}`：官方给的是倍数不是像素，乘出来是 24.5。
	previewLineFactor float32 = 1.75
	messagePad        float32 = 24
	// 首帧还没量到宽度时的估算值：右栏最窄也在 240 以上，给一个不会撑爆的数。
	previewFallbackWidth float32 = 320
)

// previewStatus 是右栏全文的四种非正文状态。
//
// 官方对应 `PlanPreview.tsx:27-31` 的四种取值（expired / unavailable / failed /
// loading）外加 `planFailureLine`（OFF:failure-line.ts:14-20）的两条资源错误。
// 本仓 durable 数据就在 `env.Data.Frames` 里，所以没有异步 resource 的 loading/failed；
// 临时 plan-review 正文只在本进程内，地址还在但参数已丢时画真实 previewExpired。
type previewStatus uint8

const (
	previewReady previewStatus = iota
	previewUnavailable
	previewNotFound
	previewInvalidAddress
	previewExpired
)

// statusText 把状态映射到官方原文。
func statusText(status previewStatus) string {
	switch status {
	case previewInvalidAddress:
		return zhInvalidAddress
	case previewNotFound:
		return zhNotFound
	case previewUnavailable:
		return zhUnavailable
	case previewExpired:
		return zhExpired
	}
	return ""
}

// previewState 是右栏全文的排版缓存。
//
// `MarkdownView.Layout` 很贵（要量每一行），所以正文/宽度不变就复用同一个
// Document 与 MarkdownView——与 `plugins/dsconversation/conversation.go:752-780`
// 同一条缓存判据。
type previewState struct {
	doc   markdown.Document
	view  components.MarkdownView
	text  string
	width float32
	// widthSeen 是上一帧量到的**真实**正文宽度（holder 的 rect 宽）。
	// 首帧为 0，那时按右栏宽度估算；测到之后就一直用它排版——这样即使右栏
	// 宽度与 `Geometry.RightWidth` 之间有边框/内距的差，也不会一直按错的宽度换行。
	widthSeen float32
	height    float32
}

// previewBoxes 是本帧的几何。**只在同一帧内有效**：Draw 里写、AfterDraw 里读。
type previewBoxes struct {
	scroll ui.Box
	holder ui.Box
}

// drawPreview 画右栏 `sidebar.right.pane.tab` 的 plan 体（官方 `PlanPreview`）。
//
// 数据来源与官方同一个事实：已提交计划的正文是 `exit_plan_mode` 的调用参数
// （OFF:plan-resource.ts:24-70 从 session history 逐页翻找），本仓直接从
// `env.Data.Frames` 里取（同一个 journal 的原始帧，壳已经缓存好）。
//
// 正文身份来自本帧 occurrence 的官方 `tab.navigation.address`（`env.RightTabAddress`）。
// 按地址打开（点计划卡走 `Actions.OpenRightResource`）时一枚 tab 就是一个地址；
// 空地址是"默认种下的类型 tab / 老壳"，这时退回插件记的最近一次打开，仍没有再退到
// "本会话最新一份计划"——不然用户点开这个 tab 只会看到一句"不可用"，而会话里明明有计划。
func (p *Plugin) drawPreview(f ui.C, env *pluginapi.Env) {
	p.boxes = previewBoxes{}
	t := p.theme
	if t == nil || env == nil {
		return
	}
	plans := SubmittedPlans(env.Data.Frames)
	// 与 drawCards 同一条：右栏正文自己也可能成为"这一帧唯一见过计划帧的地方"
	// （会话视图切到轨迹、或 Turn 尾被别的东西顶掉时），标题缓存照样要填。
	p.rememberPlanTitles(plans)
	plan, status := p.resolve(env, plans)
	if status != previewReady {
		ui.Column(f).FillWidth().FillHeight().
			PadSides(messagePad, messagePad, messagePad, messagePad).Children(func() {
			ui.Text(f, statusText(status)).FontSize(previewFont).Color(t.LabelSecondary)
		})
		return
	}

	padTop, padX, padBottom := previewInsets(env)
	width := p.preview.widthSeen
	if width <= 0 {
		width = previewFallbackWidth
		if env.Geometry.RightWidth > 0 {
			width = env.Geometry.RightWidth - 2*padX
		}
	}
	height := p.layoutPreview(env.Canvas, plan.Markdown, width)

	ui.Column(f).FillWidth().FillHeight().Children(func() {
		scroll := ui.Scroll(f.Key("plan-preview")).FillWidth().Grow(1).Clip()
		p.boxes.scroll = scroll
		scroll.Children(func() {
			// `.document` 的字体/行高在外层（官方 `.document{font-size:14px;line-height:1.75}`
			// 是容器，正文里面的块级样式由 MarkdownText 自己定）。
			ui.Column(f).FillWidth().
				PadSides(padTop, padX, padBottom, padX).Children(func() {
				p.boxes.holder = ui.Column(f).FillWidth().MinHeight(height)
			})
		})
	})
}

// previewInsets 对应官方 @media(max-width:767px)；媒体查询看整窗，不看右栏宽。
func previewInsets(env *pluginapi.Env) (top, x, bottom float32) {
	viewport := env.Bounds.Width
	if viewport <= 0 {
		viewport = env.Geometry.Width
	}
	if viewport > 0 && viewport <= previewCompactViewport {
		return previewCompactPadTop, previewCompactPadX, previewCompactPadBot
	}
	return previewPadTop, previewPadX, previewPadBottom
}

// afterDrawPreview 是覆画阶段：把排好版的 Markdown 画到占位上。
//
// 为什么必须两段：viva 的绘制顺序由布局树决定，在 Draw 里直接 `Paint` 会被后画的
// 兄弟节点盖住（仓里踩过，见 `pluginapi.Painter.AfterDraw` 的注释）。
//
// 裁剪用的是**滚动视口**的矩形：正文比视口高时，只有视口里那一段该出现。
// holder 的 rect 已经带上了本帧的滚动偏移（viva 在布局阶段就把它算进子节点坐标，
// `ui/layout.go:520` 的 `pos -= snapScroll(n.scroll)`），所以这里不用再减一次。
func (p *Plugin) afterDrawPreview(canvas *gfx.Canvas, env *pluginapi.Env) {
	if canvas == nil || env == nil {
		return
	}
	if !p.boxes.holder.Valid() || !p.boxes.scroll.Valid() {
		return
	}
	holder := p.boxes.holder.Rect()
	if holder.Width <= 0 || p.preview.view.Height() <= 0 {
		return
	}
	// 下一帧按这个宽度排版：它比 `Geometry.RightWidth` 更接近真实可用宽度。
	p.preview.widthSeen = holder.Width

	content := gfx.Rect{X: holder.X, Y: holder.Y, Width: holder.Width, Height: p.preview.view.Height()}
	clip := intersectPreview(content, p.boxes.scroll.Rect())
	if clip.Width <= 0 || clip.Height <= 0 {
		return
	}
	var scope gfx.Scope
	if env.Tree != nil {
		scope = env.Tree.Feature("plan-preview-links")
	}
	p.preview.view.Paint(canvas, components.MarkdownPaint{
		Look: p.markdownLook(), X: holder.X, Y: holder.Y, Clip: clip, Links: scope,
	})
	if scope.IsZero() || p.preview.view.LinkCount() == 0 {
		return
	}
	clicked, ok := canvas.ClickedIn(scope)
	if !ok || clicked < 0 || clicked >= p.preview.view.LinkCount() {
		return
	}
	p.openLink(env, p.preview.view.Link(clicked))
}

// layoutPreview 排版正文并返回高度；正文与宽度都没变就复用上一份。
func (p *Plugin) layoutPreview(canvas *gfx.Canvas, text string, width float32) float32 {
	if canvas == nil || width <= 0 {
		return 0
	}
	if p.preview.text == text && p.preview.width == width {
		return p.preview.height
	}
	p.preview.text, p.preview.width = text, width
	p.preview.doc.Parse(text)
	p.preview.height = p.preview.view.Layout(canvas, components.MarkdownStyle{
		Size:       previewFont,
		LineHeight: previewFont * previewLineFactor,
		Highlight:  mdlite.Highlight,
	}, &p.preview.doc, width)
	return p.preview.height
}

// markdownLook 是行内标记/代码块的配色，取主题的语义色（与 transcript 同一份）。
func (p *Plugin) markdownLook() components.MarkdownLook {
	look := components.MarkdownLookFor(dscore.VivaTheme(p.theme))
	look.Code[components.CodeComment-1] = p.theme.CodeComment
	look.Code[components.CodeString-1] = p.theme.CodeString
	look.Code[components.CodeNumber-1] = p.theme.CodeNumber
	look.Code[components.CodeKeyword-1] = p.theme.CodeKeyword
	look.Code[components.CodeFunction-1] = p.theme.CodeFunction
	look.Code[components.CodeVariable-1] = p.theme.CodeVariable
	return look
}

// openLink 打开正文里的一处链接。
//
// 只认**明确 scheme**（http/https/mailto），这是仓里 `mdlite.LinkTargetForURL`
// 的既有判据：组件不把任意字符串猜成宿主文件路径。计划正文里的相对路径链接
// 因此点不开——官方会把它当工作区文件打开，本仓没有那条解析（差异已登记）。
func (p *Plugin) openLink(env *pluginapi.Env, destination string) {
	target, ok := mdlite.LinkTargetForURL(destination)
	if !ok {
		return
	}
	if env.Act.OpenExternal != nil {
		_ = env.Act.OpenExternal(target.Destination)
	}
}

// resolve 决定右栏该画哪份计划。
//
// **地址优先**（官方语义）：本帧派发的 occurrence 带 `env.RightTabAddress`
// （官方 `tab.navigation.address`，contract/slots.ts:109-121）时就按它找——
// 那是这枚 tab 自己的资源，与"用户最后点了哪张卡"无关。空 = 这枚 tab 不是按地址
// 打开的（默认种下的类型 tab / 老壳），退回插件记的 `openAddress`。
//
// 判据顺序：
//
//	① 临时审阅地址 → 本进程的临时文档；文档没了 = `expired`（官方同一条）
//	② 计划地址     → 按 (会话, callId) 在该会话的帧里找：解析失败 `invalidAddress`
//	③ 地址属于别的会话 → 按"没选过"处理（右栏 tab 是全局的，切会话后这条地址在
//	   本会话里没有意义；不报"找不到"，因为用户没在找它）
//	④ 都没有地址 → 本会话**最新**一份计划（官方没有这一态：它的 tab 由地址打开）
//	⑤ 会话里一份都没有 → `unavailable`（官方原文 `计划预览不可用`）
func (p *Plugin) resolve(env *pluginapi.Env, plans []Document) (Document, previewStatus) {
	session := env.Data.Cur
	openAddress := p.openAddress
	if env.RightTabAddress != "" {
		openAddress = env.RightTabAddress
	}
	if openAddress != "" {
		if addressSession, _, reviewAddress := parseReviewPreviewAddress(openAddress); reviewAddress {
			if addressSession != session {
				return Document{}, previewExpired
			}
			if plan, exists := p.temporaryReviews[openAddress]; exists {
				return plan, previewReady
			}
			return Document{}, previewExpired
		}
		addressSession, callID, ok := parsePlanAddress(openAddress)
		if !ok {
			return Document{}, previewInvalidAddress
		}
		if addressSession != session {
			// 地址属于**别的会话**：右栏 tab 是全局的，切会话后这个地址在本会话
			// 里没有意义。这时不报"找不到"（用户没在找它），按"没选过"处理——
			// 退到本会话最新一份计划；本会话没有就是 `unavailable`。
			if len(plans) == 0 {
				return Document{}, previewUnavailable
			}
			return plans[len(plans)-1], previewReady
		}
		for _, plan := range plans {
			if plan.CallID == callID {
				return plan, previewReady
			}
		}
		return Document{}, previewNotFound
	}
	if len(plans) == 0 {
		return Document{}, previewUnavailable
	}
	return plans[len(plans)-1], previewReady
}

// intersectPreview 求两个矩形的交集。
func intersectPreview(a, b gfx.Rect) gfx.Rect {
	x1, y1 := maxPreviewFloat(a.X, b.X), maxPreviewFloat(a.Y, b.Y)
	x2 := minPreviewFloat(a.X+a.Width, b.X+b.Width)
	y2 := minPreviewFloat(a.Y+a.Height, b.Y+b.Height)
	if x2 <= x1 || y2 <= y1 {
		return gfx.Rect{}
	}
	return gfx.Rect{X: x1, Y: y1, Width: x2 - x1, Height: y2 - y1}
}

func maxPreviewFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func minPreviewFloat(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
