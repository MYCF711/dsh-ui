package dsconversation

// official_markdown_inline_test.go —— 正文行内混排（`#18` (b) ①：图夹在句子里）。
//
// # 这条守的是什么
//
// 官方正文里的图是**行内原子盒**（`lK` 把它包进 `._imageButton_13jw0_471{display:inline-block;
// vertical-align:middle}`，`OFF:dist/assets/index-CSAdOBx6.css`），`<p>文字 <button>图</button>
// 文字</p>` 三者在同一行内流里。旧实现把图切成块级 ⇒「图夹在句中」比官方**多一次换行**。
//
// 四个用例是一组：
//
//	TestMarkdownInlineImageSharesTheTextLine      真差异的判据：图与前后文字同一行、
//	                                              图在文字右边、`vertical-align:middle` 的基线关系
//	TestMarkdownInlineAltTextStaysOnTheLine       加载中（Pending）那一档是**行内 alt 文案**，
//	                                              与官方 `.imageAlt` 同色
//	TestMarkdownImageAloneKeepsTheBlockLayout     反向：图单独成段**行为不变**（图带里没有文字、
//	                                              上下各是一段文字、图贴列左缘）
//	TestMarkdownTextOnlyKeepsTheOldSingleBox      反向：没有图的段落仍是**一个**文字盒
//
// 真窗口像素判据在 `official_markdown_inline_headless_test.go`。

import (
	"math"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
)

// renderMarkdownInlineReady 造一个"图已就绪"的视图与一次真画布渲染。
//
// 取数闸门先关着、放闸后等缓存真的进 Ready，与 `official_markdown_image_test.go`
// 的既有夹具同一套（那里钉的是"图怎么画"，这里钉的是"图怎么参与排版"）。
func renderMarkdownInlineReady(t *testing.T, text string) (*gfx.Canvas, *OfficialTranscriptView) {
	t.Helper()
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	body := solidColorPNG(t, 300, 150, messageImageFixtureColor)
	release := make(chan struct{})
	act := pluginapi.Actions{
		MarkdownImage: func(_, _, destination string) (pluginapi.MarkdownImageBytes, bool) {
			if destination != "./shot.png" {
				return pluginapi.MarkdownImageBytes{}, false
			}
			<-release
			return pluginapi.MarkdownImageBytes{Bytes: body, MIME: "image/png"}, true
		},
	}
	entries := markdownImageEntry(text, false)
	// 第一帧把取数请求发出去（状态停在 Pending）。
	_ = renderMarkdownImages(t, view, entries, act)
	close(release)
	waitMarkdownImage(t, view, "./shot.png", mdlite.MarkdownImageReady)
	return renderMarkdownImages(t, view, entries, act), view
}

// TestMarkdownInlineImageSharesTheTextLine 是「多一次换行」那条旧账的**像素判据**。
//
// 一段「文字 + 图 + 文字」里，官方三者同排：
//
//	① 恰好一张位图，且两个文字盒都在图的**行带**里（上下有重叠）；
//	② 图紧跟在图前那段文字右边，图后的文字紧跟在图右边；
//	③ `vertical-align:middle`：图的垂直中点 = 文字基线 + 半个 x-height
//	   （x-height 取 0.5em，见 `MarkdownInlineImageXHeight`）。
//
// 旧实现（图切成块级）在这三条上全红：文字各占一行，一个在图上方、一个在下方。
func TestMarkdownInlineImageSharesTheTextLine(t *testing.T) {
	canvas, _ := renderMarkdownInlineReady(t, "看图 ![示意图](./shot.png) 结尾")
	images := canvas.Images()
	if len(images) != 1 {
		t.Fatalf("屏上位图 %d 张，期望 1 张；文本：%v", len(images), canvasTexts(canvas))
	}
	img := images[0]
	if img.Bounds.Width != 300 || img.Bounds.Height != 150 {
		t.Fatalf("显示框 = %v×%v，期望 300×150（原尺寸，未超 640×360）",
			img.Bounds.Width, img.Bounds.Height)
	}
	before, ok := canvasText(canvas, "看图 ")
	if !ok {
		t.Fatalf("图前那段文字应是一整段（含图前那格空白）；实际文本：%v", canvasTexts(canvas))
	}
	after, ok := canvasText(canvas, " 结尾")
	if !ok {
		t.Fatalf("图后那段文字应是一整段（含图后那格空白）；实际文本：%v", canvasTexts(canvas))
	}

	// ① 行带：两个文字盒都要与图在垂直方向重叠（同一行），且不许一个在上一个在下。
	if !rectsOverlapVertically(before.Bounds, img.Bounds) {
		t.Fatalf("图前文字 %v 与图 %v 不在同一行带（旧账的「多一次换行」回来了）", before.Bounds, img.Bounds)
	}
	if !rectsOverlapVertically(after.Bounds, img.Bounds) {
		t.Fatalf("图后文字 %v 与图 %v 不在同一行带（旧账的「多一次换行」回来了）", after.Bounds, img.Bounds)
	}

	// ② 水平顺序：文字 / 图 / 文字，且首尾相接（中间只差那格空白）。
	if math.Abs(float64(before.Bounds.X+before.Bounds.Width-img.Bounds.X)) > 0.01 {
		t.Fatalf("图的左缘 = %v，期望贴在图前文字右缘 %v", img.Bounds.X,
			before.Bounds.X+before.Bounds.Width)
	}
	if math.Abs(float64(after.Bounds.X-(img.Bounds.X+img.Bounds.Width))) > 0.01 {
		t.Fatalf("图后文字左缘 = %v，期望贴在图右缘 %v", after.Bounds.X,
			img.Bounds.X+img.Bounds.Width)
	}

	// ③ vertical-align:middle：图中心 = 文字基线 + 半个 x-height（0.25 × 字号）。
	// `canvas.LabelBaseline(bounds, style)` 直接给的就是**画布坐标里的基线 y**
	// （实现是 `bounds.Y + labelBaselineRow(...)`，所以不能再加一次 bounds.Y）。
	baseline := float64(canvas.LabelBaseline(before.Bounds, before.Style))
	want := float64(MarkdownInlineImageXHeight * markdownInlineBodyFont(t))
	if got := float64(img.Bounds.Y+img.Bounds.Height/2) - baseline; math.Abs(got-want) > 0.01 {
		t.Fatalf("图中心比文字基线低 %v，期望 %v（vertical-align:middle 的半个 x-height）", got, want)
	}

	// 行盒高度：图是这一行最高的盒子 ⇒ 图顶落在行顶（偏移 0），整段高度就是图高。
	span := transcriptOpsSpan(canvas)
	if math.Abs(float64(span.bottom-span.top)-150) > 0.5 {
		t.Fatalf("这一段的竖直跨度 = %v，期望 150（只有一行，图那一行）", span.bottom-span.top)
	}
}

// TestMarkdownInlineAltTextStaysOnTheLine 钉加载中那一档：图还没到，官方画的是
// `span.imageAlt` —— **行内文字**（tertiary 色），不是把图那一行换成块。
func TestMarkdownInlineAltTextStaysOnTheLine(t *testing.T) {
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	gated := make(chan struct{})
	defer close(gated)
	act := pluginapi.Actions{
		MarkdownImage: func(_, _, destination string) (pluginapi.MarkdownImageBytes, bool) {
			<-gated
			return pluginapi.MarkdownImageBytes{}, false
		},
	}
	canvas := renderMarkdownImages(t, view, markdownImageEntry("看图 ![示意图](./shot.png) 结尾", false), act)
	if images := canvas.Images(); len(images) != 0 {
		t.Fatalf("字节没回来就画了 %d 张位图", len(images))
	}
	before, okBefore := canvasText(canvas, "看图 ")
	alt, okAlt := canvasText(canvas, "示意图")
	after, okAfter := canvasText(canvas, " 结尾")
	if !okBefore || !okAlt || !okAfter {
		t.Fatalf("三档文字都应在一行里；实际文本：%v", canvasTexts(canvas))
	}
	if !rectsShareBand(before.Bounds, alt.Bounds) || !rectsShareBand(alt.Bounds, after.Bounds) {
		t.Fatalf("alt 文案没跟正文同排：前 %v / alt %v / 后 %v", before.Bounds, alt.Bounds, after.Bounds)
	}
	if alt.Style.Color != theme.LabelTertiary {
		t.Fatalf("alt 文案色 = %v，期望官方 .imageAlt 的 label-tertiary %v", alt.Style.Color, theme.LabelTertiary)
	}
}

// TestMarkdownInlineListItemSharesTheLine 钉列表项那一条调用点：列表项的正文也走
// 同一套行内排版（`drawListItem` → `drawMarkdownInline`），图与文字同排、且图缩进在
// 标记槽右侧（不是压在 `•` 上）。
func TestMarkdownInlineListItemSharesTheLine(t *testing.T) {
	canvas, _ := renderMarkdownInlineReady(t, "- 看图 ![示意图](./shot.png) 结尾")
	images := canvas.Images()
	if len(images) != 1 {
		t.Fatalf("屏上位图 %d 张，期望 1 张；文本：%v", len(images), canvasTexts(canvas))
	}
	img := images[0]
	before, okBefore := canvasText(canvas, "看图 ")
	after, okAfter := canvasText(canvas, " 结尾")
	bullet, okBullet := canvasText(canvas, "•")
	if !okBefore || !okAfter || !okBullet {
		t.Fatalf("列表项里的文字/标记都得在；实际文本：%v", canvasTexts(canvas))
	}
	if !rectsOverlapVertically(before.Bounds, img.Bounds) || !rectsOverlapVertically(after.Bounds, img.Bounds) {
		t.Fatalf("列表项里图与文字不同排：前 %v / 图 %v / 后 %v", before.Bounds, img.Bounds, after.Bounds)
	}
	if img.Bounds.X <= bullet.Bounds.X {
		t.Fatalf("图的左缘 = %v，应缩进在标记槽 %v 右侧", img.Bounds.X, bullet.Bounds)
	}
}

// TestMarkdownImageAloneKeepsTheBlockLayout 是**反向**判据：图单独成段时行为不许变。
//
// 官方这一种写法（行盒里只有那张图）本来就是"图独占一行"：图的 A+D 正好是 H。
// 所以它不是"凑合的旧实现"，而是官方那一种写法的正确结果——本轮**一个像素都不许动**。
func TestMarkdownImageAloneKeepsTheBlockLayout(t *testing.T) {
	canvas, _ := renderMarkdownInlineReady(t, "看图\n\n![示意图](./shot.png)\n\n结尾")
	images := canvas.Images()
	if len(images) != 1 {
		t.Fatalf("屏上位图 %d 张，期望 1 张；文本：%v", len(images), canvasTexts(canvas))
	}
	img := images[0]
	if img.Bounds.Width != 300 || img.Bounds.Height != 150 {
		t.Fatalf("显示框 = %v×%v，期望 300×150", img.Bounds.Width, img.Bounds.Height)
	}
	above, okAbove := canvasText(canvas, "看图")
	below, okBelow := canvasText(canvas, "结尾")
	if !okAbove || !okBelow {
		t.Fatalf("图上下两段文字都得在；实际文本：%v", canvasTexts(canvas))
	}
	// 图带里不许有任何文字：上下两段都必须**完全**在图之上 / 之下。
	for _, text := range []gfx.Text{above, below} {
		if rectsShareBand(text.Bounds, img.Bounds) {
			t.Fatalf("图单独成段时 %q 却与图同排了（%v vs %v）：行内混排把这一种写法也改了",
				text.Value, text.Bounds, img.Bounds)
		}
	}
	if above.Bounds.Y+above.Bounds.Height > img.Bounds.Y {
		t.Fatalf("上一段文字 %v 压到了图上沿 %v 之后", above.Bounds, img.Bounds.Y)
	}
	if below.Bounds.Y < img.Bounds.Y+img.Bounds.Height {
		t.Fatalf("下一段文字 %v 顶到了图下沿 %v 之前", below.Bounds, img.Bounds.Y+img.Bounds.Height)
	}
	// 图贴列左缘：块级那一档的像素位置（不再被行内排版推开）。
	if math.Abs(float64(img.Bounds.X-above.Bounds.X)) > 0.01 {
		t.Fatalf("图的左缘 = %v，期望与正文列左缘 %v 一致", img.Bounds.X, above.Bounds.X)
	}
	// 竖直位置逐块相接：上一段 + (块间距) = 图顶，图 + (块间距) = 下一段顶。
	// 这一条钉的是**块级堆叠的算术**：任何"把行内那套基线偏移漏进块级路径"的改法
	//（比如给图加一个行高偏移）都会在这里差出几十像素。
	if want := above.Bounds.Y + above.Bounds.Height + AssistantBlockGap; math.Abs(float64(img.Bounds.Y-want)) > 0.01 {
		t.Fatalf("图顶 = %v，期望 = 上一段底 %v + 块间距 %v = %v（块级堆叠被改了）",
			img.Bounds.Y, above.Bounds.Y+above.Bounds.Height, AssistantBlockGap, want)
	}
	if want := img.Bounds.Y + img.Bounds.Height + AssistantBlockGap; math.Abs(float64(below.Bounds.Y-want)) > 0.01 {
		t.Fatalf("下一段顶 = %v，期望 = 图底 %v + 块间距 %v = %v（块级堆叠被改了）",
			below.Bounds.Y, img.Bounds.Y+img.Bounds.Height, AssistantBlockGap, want)
	}
}

// TestMarkdownTextOnlyKeepsTheOldSingleBox 是**反向**判据的另一半：没有图的段落
// 仍是一次 `Wrap`（一个文字盒、整段文字一个字不差），行内排版不许碰到它。
func TestMarkdownTextOnlyKeepsTheOldSingleBox(t *testing.T) {
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	text := "只有文字，没有图的一段。"
	canvas := renderMarkdownImages(t, view, markdownImageEntry(text, false), pluginapi.Actions{})
	texts := canvas.Texts()
	if len(texts) != 1 {
		t.Fatalf("没有图的段落应只有一个文字盒，实际 %d 个：%v", len(texts), canvasTexts(canvas))
	}
	if texts[0].Value != text {
		t.Fatalf("整段文字被切了：%q，期望 %q", texts[0].Value, text)
	}
	// 仍然是**一条 `Wrap`**：盒子吃满正文列（`FillWidth`），不是行内排版那种"按量出来的
	// 宽度定尺"的盒子。这一条把"老路径"和"行内路径"分开钉住。
	natural := float32(len([]rune(text))) * theme.Fonts.Body
	if texts[0].Bounds.Width <= natural*2 {
		t.Fatalf("文字盒宽 = %v（自然宽 %v）：没有图的段落不该被行内排版按自然宽定尺",
			texts[0].Bounds.Width, natural)
	}
}

// markdownInlineBodyFont 取正文行内排版的字号（与 drawParagraph 传下去的那一档同源）。
func markdownInlineBodyFont(t *testing.T) float32 {
	t.Helper()
	theme := dscore.Dark()
	return theme.Fonts.Body
}

// rectsOverlapVertically 报告两块矩形在竖直方向有重叠（同一行带的判据）。
func rectsOverlapVertically(a, b gfx.Rect) bool {
	return a.Y < b.Y+b.Height && b.Y < a.Y+a.Height
}

// rectsShareBand 报告两块矩形的**竖直中线**落在同一段里（同排：基线一致）。
func rectsShareBand(a, b gfx.Rect) bool {
	centerA := a.Y + a.Height/2
	centerB := b.Y + b.Height/2
	return centerA >= b.Y && centerA <= b.Y+b.Height || centerB >= a.Y && centerB <= a.Y+a.Height
}

// transcriptOpsSpan 返回这一帧所有图元（位图 + 文字）的竖直跨度。
func transcriptOpsSpan(canvas *gfx.Canvas) struct{ top, bottom float32 } {
	span := struct{ top, bottom float32 }{top: math.MaxFloat32}
	for _, image := range canvas.Images() {
		span.top = minFloat(span.top, image.Bounds.Y)
		span.bottom = maxFloat(span.bottom, image.Bounds.Y+image.Bounds.Height)
	}
	for _, text := range canvas.Texts() {
		span.top = minFloat(span.top, text.Bounds.Y)
		span.bottom = maxFloat(span.bottom, text.Bounds.Y+text.Bounds.Height)
	}
	return span
}
