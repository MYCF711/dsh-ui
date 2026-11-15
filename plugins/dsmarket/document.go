// document.go —— 详情文档那一段（官方 `PluginDetailSection` + `DetailDocument`）。
//
// # 官方它长什么样
//
//	OFF:../dsh-client-ui-plugin-market/src/client/DiscoverSection.tsx:290-297
//	     详情页里一个 `.detailDocument` 容器（`margin-top:16px`；空了就整段不占位）
//	OFF:../dsh-client-ui-plugin-market/src/client/PluginDetailSection.tsx:25-47
//	     容器里是官方 `DetailDocument` 原语：它接 4 种状态（loading / ready /
//	     ready-but-undefined / error），错误文案按六个失败档取
//	OFF:../dsh-client-ui-plugin-market/src/client/market-store.ts:258-283
//	     读取：`pluginMarket.readEntryDetail(url, name, version, language, {refresh})`，
//	     连接重置后所有读作废重来
//
// # 本仓怎么落地
//
//  1. **markdown 用现成的 `components.MarkdownView`**（仓里 `plugins/dsplan` /
//     `dsconversation` 同一套：`mdlite.Highlight` 上色、`MarkdownPaint` 覆画）。
//     不自己解析 markdown——那是仓规。
//  2. 读取不在插件里发：壳通过 `env.Act.MarketEntryDetail` 把结果交进来，这里只分状态。
//  3. `DetailDocument` 自己的**外框**（标题行 / 复制按钮 / 换行开关）不画：
//     那个原语在 `@deepseek-ai/dsh-client-ui-primitives` 里，而那份包随包**不发 CSS**
//     （本机 rc.2 的 node_modules 里没有它），几何没有证据。按仓规不发明外框，
//     只画正文 + 三行状态文案（三者都是官方 zh 原文）。
package dsmarket

import (
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

// documentState 是详情文档的四种可见状态 + "壳没接"那一种。
//
// 官方 `DetailDocument` 的 `detailDocumentState` 形状（`PluginDetailSection.tsx:43-45`）：
// `undefined` / `{status:'loading'}` / `{status:'ready', markdown?}` / `{status:'error'}`。
type documentState uint8

const (
	// documentAbsent：壳没接详情读（`Act.MarketEntryDetail` 为 nil）→ 整段不画，
	// 与官方 `.detailDocument:empty{display:none}` 的视觉效果一致。
	documentAbsent documentState = iota
	// documentLoading：壳还没读到（官方 `loading`）。
	documentLoading
	// documentNone：宿主明确回答"这一版没有文档"（官方 `ready` + markdown undefined）。
	documentNone
	// documentFound：有正文。
	documentFound
	// documentFailed：读失败，文案按六档取。
	documentFailed
)

// documentView 是本帧文档段的一次投影。
type documentView struct {
	state    documentState
	markdown string
	failure  apiclient.PluginMarketDetailFailure
	// retryable = 壳接了重读（`Act.MarketEntryDetailRetry`）。没接就不画重试按钮。
	retryable bool
}

// docCache 是正文排版缓存（与 `plugins/dsplan/preview.go` 同一条判据：
// 正文与宽度都没变就复用同一份 Document/MarkdownView）。
type docCache struct {
	doc    markdown.Document
	view   components.MarkdownView
	text   string
	width  float32
	height float32
}

// document 读一次详情状态（纯读：不在这里发请求、不写盘）。
//
// 数据来自 `env.Act.MarketEntryDetail`（壳缓存 + 壳发请求）。三个判据：
//
//	壳没接（Action 为 nil） → absent：整段不画（官方 `.detailDocument:empty{display:none}`）
//	没有版本号             → absent：宿主 readEntryDetail 缺 version 会回 400
//	failure 非空            → failed：按官方六档取文案
func (p *Page) document(env *pluginapi.Env, request ReadRequest) documentView {
	if env == nil || env.Act.MarketEntryDetail == nil {
		return documentView{state: documentAbsent}
	}
	if request.Version == "" {
		return documentView{state: documentAbsent}
	}
	detail, loaded, failure := env.Act.MarketEntryDetail(request.URL, request.Name, request.Version, p.language)
	if failure != "" {
		return documentView{state: documentFailed, failure: failure, retryable: env.Act.MarketEntryDetailRetry != nil}
	}
	if !loaded {
		return documentView{state: documentLoading}
	}
	if !detail.Found() {
		return documentView{state: documentNone}
	}
	return documentView{state: documentFound, markdown: detail.Markdown}
}

// documentRequest 拼出这一页要读的身份。
func (p *Page) documentRequest(version Version, hasVersion bool) ReadRequest {
	if !hasVersion {
		return ReadRequest{URL: p.marketURL, Name: p.entry.Name}
	}
	return ReadRequest{URL: p.marketURL, Name: p.entry.Name, Version: version.Version}
}

// layoutDocument 量出正文高度（0 = 这一帧没有正文要画）。
//
// 为什么先量：正文下面还有版本列表与 facts 两段，DOM 会自己流，Go 的布局树不会。
// 与 `plugins/dsplan/preview.go` 的做法一致：Draw 里用 `MarkdownView.Layout` 量，
// AfterDraw 里 `Paint` 到量到的那块矩形上。
func (p *Page) layoutDocument(env *pluginapi.Env, width float32, version Version, hasVersion bool) float32 {
	view := p.document(env, p.documentRequest(version, hasVersion))
	if view.state != documentFound {
		return 0
	}
	if env == nil || env.Canvas == nil || width <= 0 {
		return 0
	}
	if p.doc.text == view.markdown && p.doc.width == width {
		return p.doc.height
	}
	p.doc.text, p.doc.width = view.markdown, width
	p.doc.doc.Parse(view.markdown)
	p.doc.height = p.doc.view.Layout(env.Canvas, components.MarkdownStyle{
		Size:       p.theme.Fonts.Body,
		LineHeight: p.theme.Fonts.BodyLine,
		Highlight:  mdlite.Highlight,
	}, &p.doc.doc, width)
	return p.doc.height
}

// drawDocument 画详情文档那一段（`.detailDocument`）。
func (p *Page) drawDocument(f ui.C, env *pluginapi.Env, height float32, version Version, hasVersion bool) {
	view := p.document(env, p.documentRequest(version, hasVersion))
	t := p.theme
	switch view.state {
	case documentAbsent, documentNone:
		// 官方 `.detailDocument:empty{display:none}`：没有正文就没有这一段。
		return
	case documentLoading:
		ui.Wrap(f, zhDetailLoading).FillWidth().FontSize(metaSize).LineHeight(metaLine).
			Color(t.LabelTertiary).PadSides(documentTop, 0, 0, 0)
		return
	case documentFailed:
		ui.Column(f).FillWidth().PadSides(documentTop, 0, 0, 0).Gap(sectionGap).Children(func() {
			ui.Wrap(f, failureText(view.failure)).FillWidth().FontSize(metaSize).LineHeight(metaLine).
				Color(t.LabelTertiary)
			if !view.retryable {
				return
			}
			id := gfx.ID{}
			if env != nil && env.Tree != nil {
				id = env.Tree.ID(env.Feat, "market-detail-retry")
			}
			// 重试按钮沿用本仓既有的**文字按钮**约定（28px 高、半径 MD、BgLayer3 底，
			// 与 `plugins/dssidebar/plugins_page.go:457` 的详情动作按钮同一形状）：
			// 官方那枚是 primitives 的 Button，随包不发 CSS（见 §4 缺口 5）。
			button := ui.Row(f).Shrink(0).Height(28).Radius(t.Radii.MD).
				PadSides(0, 10, 0, 10).Items(ui.Center).Justify(ui.Center).
				Background(t.BgLayer3).Hit(id, zhRetry).Children(func() {
				ui.Text(f, zhRetry).FontSize(metaSize).LineHeight(metaLine).Color(t.LabelPrimary)
			})
			if button.Clicked() && env != nil && env.Act.MarketEntryDetailRetry != nil {
				request := p.documentRequest(version, hasVersion)
				env.Act.MarketEntryDetailRetry(request.URL, request.Name, request.Version, p.language)
			}
		})
		return
	}
	// documentFound：先占位（高度是量出来的），正文在 AfterDraw 里覆画。
	ui.Column(f).FillWidth().PadSides(documentTop, 0, 0, 0).Children(func() {
		if height <= 0 {
			// 还没量到（首帧没有 canvas）：占一个最小高度，不让版本列表跳到顶上。
			p.boxes.holder = ui.Column(f).FillWidth().MinHeight(p.theme.Fonts.BodyLine)
			return
		}
		p.boxes.holder = ui.Column(f).FillWidth().MinHeight(height)
	})
}

// afterDraw 是覆画阶段：把排好版的正文画到占位上。
//
// 为什么必须分两段：viva 的绘制顺序由布局树决定，Draw 里直接 `Paint` 会被后画的
// 兄弟节点盖住（`pluginapi.Painter.AfterDraw` 的注释、dsplan 的 preview 都写着）。
func (p *Page) afterDraw(canvas *gfx.Canvas, env *pluginapi.Env) {
	if canvas == nil || env == nil || !p.open {
		return
	}
	if !p.boxes.holder.Valid() || !p.boxes.scroll.Valid() || p.doc.view.Height() <= 0 {
		return
	}
	holder := p.boxes.holder.Rect()
	if holder.Width <= 0 {
		return
	}
	// 下一帧按这个宽度排版：它比 `env.MainWidth` 减去内距更接近真实可用宽度。
	p.widthSeen = holder.Width

	content := gfx.Rect{X: holder.X, Y: holder.Y, Width: holder.Width, Height: p.doc.view.Height()}
	clip := intersectRect(content, p.boxes.scroll.Rect())
	if clip.Width <= 0 || clip.Height <= 0 {
		return
	}
	var scope gfx.Scope
	if env.Tree != nil {
		scope = env.Tree.Feature("market-detail-links")
	}
	p.doc.view.Paint(canvas, components.MarkdownPaint{
		Look: p.markdownLook(), X: holder.X, Y: holder.Y, Clip: clip, Links: scope,
	})
	if scope.IsZero() || p.doc.view.LinkCount() == 0 {
		return
	}
	clicked, ok := canvas.ClickedIn(scope)
	if !ok || clicked < 0 || clicked >= p.doc.view.LinkCount() {
		return
	}
	p.openLink(env, p.doc.view.Link(clicked))
}

// markdownLook 是行内标记/代码块的配色，取主题的语义色（与 transcript 同一份）。
func (p *Page) markdownLook() components.MarkdownLook {
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
// 只认**明确 scheme**（http/https/mailto），与 `plugins/dsplan` 同一条判据
// （`mdlite.LinkTargetForURL`）：组件不把任意字符串猜成宿主文件路径。
// 官方 README 自述 "relative links are not clickable"，相对链接点不开是官方行为；
// 官方另有 `/api/plugin-market/detail-file` 解析文档旁的图片，本仓没有那条路由，
// 所以图片一律不出（不给假图，见包注释第 2 条）。
func (p *Page) openLink(env *pluginapi.Env, destination string) {
	if env == nil || env.Act.OpenExternal == nil {
		return
	}
	target, ok := mdlite.LinkTargetForURL(destination)
	if !ok {
		return
	}
	_ = env.Act.OpenExternal(target.Destination)
}

// failureText 是官方六个失败档的 zh 文案（`locales.ts:87-92` 的 FAILURE_KEYS）。
func failureText(failure apiclient.PluginMarketDetailFailure) string {
	switch failure {
	case apiclient.PluginMarketDetailTooLarge:
		return zhDetailTooLarge
	case apiclient.PluginMarketDetailNetwork:
		return zhDetailNetwork
	case apiclient.PluginMarketDetailDenied:
		return zhDetailDenied
	case apiclient.PluginMarketDetailGitUnavailable:
		return zhDetailGitUnavailable
	case apiclient.PluginMarketDetailInvalid:
		return zhDetailInvalid
	}
	return zhDetailFailed
}

// intersectRect 求两个矩形的交集（与 dsplan 的同名工具同一条实现，不跨包引用私有件）。
func intersectRect(a, b gfx.Rect) gfx.Rect {
	x1, y1 := maxFloat(a.X, b.X), maxFloat(a.Y, b.Y)
	x2 := minFloat(a.X+a.Width, b.X+b.Width)
	y2 := minFloat(a.Y+a.Height, b.Y+b.Height)
	if x2 <= x1 || y2 <= y1 {
		return gfx.Rect{}
	}
	return gfx.Rect{X: x1, Y: y1, Width: x2 - x1, Height: y2 - y1}
}

func maxFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
