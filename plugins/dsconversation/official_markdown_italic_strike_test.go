package dsconversation

// official_markdown_italic_strike_test.go —— live transcript 的**整块斜体**与**整块删除线**。
//
// 形状与 `#236` 的 `official_markdown_bold_test.go` 逐条对应（同一套判据、同一个
// 粒度、同一个能力边界），只换样式位：
//
//	① 整块 `*…*`   → 画出来的那段文字 `Style.Italic == true`（真斜体面）；
//	② 整块 `~~…~~` → 文字与布局**一个像素都不动**（删除线是叠加，不是换字形）；
//	③ 混排（`*要点*：说明` / `~~要点~~：说明`）→ 仍直立、仍无删除线
//	   （viva 盒级没有行内 run，见 `spansAllItalic`；这是记录在案的能力边界）；
//	④ 常规段落 → 仍直立，且行盒几何与斜体那一份相同（斜面不换行盒）。
//
// 真像素判据（字形真的倾斜、字中间真有一条 1px 横线）在
// `official_markdown_italic_strike_visual_test.go`。

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
)

// markdownSpansOf 把一段 markdown 源解析成**第一个指定种类**块的 spans。
//
// 判据函数（`spansAllItalic` / `spansAllStrike`）吃的是 `doc.Spans`，测试要拿
// **真的解析结果**去喂它们 —— 手搓 `markdown.Span` 只能自证循环，证明不了
// "`*要点*` 解析出来确实每一位都带 Italic"。
func markdownSpansOf(t *testing.T, src string, kind markdown.Kind) []markdown.Span {
	t.Helper()
	var doc markdown.Document
	doc.Parse(src)
	for i := range doc.Blocks {
		if doc.Blocks[i].Kind == kind {
			return doc.Spans(&doc.Blocks[i])
		}
	}
	t.Fatalf("夹具 %q 没解析出 %v 块", src, kind)
	return nil
}

// mustCanvasTextTrimmed 断言某段文本（**去掉首尾空格后**等于 want）真的画在画布上。
//
// 行内图分支把正文按原文切段，文字段保留原文的空格（`"前文 "` / `" 后文"`），
// 整值查找会落空 —— 去空格再比才拿得到那一段盒子。
func mustCanvasTextTrimmed(t *testing.T, canvas *gfx.Canvas, want string) gfx.Text {
	t.Helper()
	for _, text := range canvas.Texts() {
		if strings.TrimSpace(text.Value) == want {
			return text
		}
	}
	t.Fatalf("画布缺少（去空格后）等于 %q 的文本；实际文本：%v", want, canvasTexts(canvas))
	return gfx.Text{}
}

// ── ① 判据：真解析结果喂给 spansAllXxx ────────────────────────────────

// TestSpansAllItalicAndStrike 钉三个判据在**真解析结果**上的结论。
//
// 混排那两行是**反向断言**（"不许误判"）：`*要点*：说明` 里只有前两个字带说明，
// 后两个字不带 —— 整块判据必须返回 false，否则半句话会被画成斜体。这条与
// `#236` 的 `TestMixedBoldParagraphStaysRegularWeight` 是同一件事的斜体/删除线版。
func TestSpansAllItalicAndStrike(t *testing.T) {
	for _, tc := range []struct {
		name              string
		source            string
		kind              markdown.Kind
		bold, italic, str bool
	}{
		{"italic-whole", "*要点*", markdown.Paragraph, false, true, false},
		{"strike-whole", "~~要点~~", markdown.Paragraph, false, false, true},
		{"bold-whole", "**要点**", markdown.Paragraph, true, false, false},
		// `***…***`：官方 `<strong><em>` 的叠加，两位都要成立。
		{"bold-italic-whole", "***要点***", markdown.Paragraph, true, true, false},
		{"plain", "要点", markdown.Paragraph, false, false, false},
		// 混排边界：`*要点*：说明` / `~~要点~~：说明` 必须整块判否。
		{"italic-mixed", "*要点*：说明", markdown.Paragraph, false, false, false},
		{"strike-mixed", "~~要点~~：说明", markdown.Paragraph, false, false, false},
		// 列表项走的是另一条绘制路径（`drawListItem`），判据必须一样。
		{"italic-item", "- *要点*", markdown.Item, false, true, false},
		{"strike-item", "- ~~要点~~", markdown.Item, false, false, true},
		{"italic-item-mixed", "- *要点*：说明", markdown.Item, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spans := markdownSpansOf(t, tc.source, tc.kind)
			if got := spansAllBold(spans); got != tc.bold {
				t.Fatalf("spansAllBold(%q) = %v，要 %v", tc.source, got, tc.bold)
			}
			if got := spansAllItalic(spans); got != tc.italic {
				t.Fatalf("spansAllItalic(%q) = %v，要 %v", tc.source, got, tc.italic)
			}
			if got := spansAllStrike(spans); got != tc.str {
				t.Fatalf("spansAllStrike(%q) = %v，要 %v", tc.source, got, tc.str)
			}
		})
	}
}

// TestSpansAllStyleWithoutAnyGlyph 钉空内容不算"整块某种样式"。
//
// 判据的骨架复用 `allMonoSpans`：空串与换行分隔不算反对票。但**一个可画字符都
// 没有**时不该返回 true（没有东西可画，也就谈不上"整块"）；返回 true 会让
// `spansAllStyle` 在空块上凭空点亮样式。
func TestSpansAllStyleWithoutAnyGlyph(t *testing.T) {
	for _, tc := range []struct {
		name  string
		spans []markdown.Span
	}{
		{"empty", nil},
		{"blank", []markdown.Span{{Text: ""}}},
		{"newlines", []markdown.Span{{Text: markdown.Newline}, {Text: "\n"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, bit := range []markdown.Style{markdown.Bold, markdown.Italic, markdown.Strike} {
				if spansAllStyle(tc.spans, bit) {
					t.Fatalf("%s：没有可画字符却判成整块带样式位 %v", tc.name, bit)
				}
			}
		})
	}
}

// TestSpansAllStyleIgnoresBlankSpans 钉"空串与换行分隔不算反对票"。
//
// `**a**\n**b**` 解析出来是 `[Bold "a"] [Newline] [Bold "b"]`：中间的换行 span
// 不带样式位，但它不是"反对票"——否则一个两行的整块加粗段会被判成混排。
func TestSpansAllStyleIgnoresBlankSpans(t *testing.T) {
	spans := markdownSpansOf(t, "**第一行**\n**第二行**", markdown.Paragraph)
	if !spansAllBold(spans) {
		t.Fatalf("两行整块加粗被判成混排：%+v", spans)
	}
	italic := markdownSpansOf(t, "*第一行*\n*第二行*", markdown.Paragraph)
	if !spansAllItalic(italic) {
		t.Fatalf("两行整块斜体被判成混排：%+v", italic)
	}
	struck := markdownSpansOf(t, "~~第一行~~\n~~第二行~~", markdown.Paragraph)
	if !spansAllStrike(struck) {
		t.Fatalf("两行整块删除线被判成混排：%+v", struck)
	}
}

// ── ② 绘制：段落 / 列表项 ─────────────────────────────────────────────

// TestAllItalicParagraphPaintsItalicFace：整块 `*…*` 必须画在斜体面上。
func TestAllItalicParagraphPaintsItalicFace(t *testing.T) {
	_, tree := newTranscriptCanvas()
	_, canvas := renderTranscript(t, assistantMarkdownText("*重点*"), hostViewport(tree))
	italic := mustCanvasText(t, canvas, "重点")
	if !italic.Style.Italic {
		t.Fatalf("整块斜体的段落 Style.Italic = false，应为 true（样式：%+v）", italic.Style)
	}
	if italic.Style.Size != 14 {
		t.Fatalf("整块斜体的段落字号 = %v，应为正文 14（斜面不该改字号）", italic.Style.Size)
	}
	if italic.Style.Weight != gfx.WeightRegular {
		t.Fatalf("整块斜体的段落字重 = %v，应为常规（斜面不该改字重）", italic.Style.Weight)
	}
}

// TestAllItalicParagraphDrawsTheSameLineBox：斜体与常规的行高、行位必须一样。
//
// 钉的是"斜面只换字形面"：如果哪天把斜体做成换字号或换行高，几何表里正文的
// 14/22 就不再是这一段的行盒读数了。与 `#236` 的加粗那条同一条判据。
func TestAllItalicParagraphDrawsTheSameLineBox(t *testing.T) {
	_, plainTree := newTranscriptCanvas()
	_, plainCanvas := renderTranscript(t, assistantMarkdownText("重点"), hostViewport(plainTree))
	plain := mustCanvasText(t, plainCanvas, "重点")

	_, italicTree := newTranscriptCanvas()
	_, italicCanvas := renderTranscript(t, assistantMarkdownText("*重点*"), hostViewport(italicTree))
	italic := mustCanvasText(t, italicCanvas, "重点")

	if italic.Bounds.Height != plain.Bounds.Height || italic.Bounds.Y != plain.Bounds.Y {
		t.Fatalf("整块斜体后的行盒 = {Y:%v H:%v}，常规 = {Y:%v H:%v}；斜面不该动行盒几何",
			italic.Bounds.Y, italic.Bounds.Height, plain.Bounds.Y, plain.Bounds.Height)
	}
}

// TestPlainParagraphStaysUpright：没写斜体的段落仍然是直立面（反向测试的画布层）。
func TestPlainParagraphStaysUpright(t *testing.T) {
	_, tree := newTranscriptCanvas()
	_, canvas := renderTranscript(t, assistantMarkdownText("重点"), hostViewport(tree))
	text := mustCanvasText(t, canvas, "重点")
	if text.Style.Italic {
		t.Fatalf("常规段落 Style.Italic = true；斜体只该落在整块 `*…*` 上")
	}
}

// TestMixedItalicParagraphStaysUpright 钉住**能力边界**，同时是一条反向断言。
//
// viva 的盒级文字只有一个 `gfx.TextStyle`：一个 `Wrap` 一套斜面，没有行内 run
// 出口。所以 `*要点*：说明` 这种混排只能整段一个斜面 —— 现在取直立（与改动前
// 逐像素一致），而不是"整段斜体"这种把半句话画错的方案。
//
// viva 有了盒级行内 run 之后，这一条要跟着改成"只有「要点」两个字斜"。
func TestMixedItalicParagraphStaysUpright(t *testing.T) {
	_, tree := newTranscriptCanvas()
	_, canvas := renderTranscript(t, assistantMarkdownText("*要点*：说明"), hostViewport(tree))
	text := mustCanvasText(t, canvas, "要点：说明")
	if text.Style.Italic {
		t.Fatalf("混排段落 Style.Italic = true；盒级没有行内 run 时只能取直立（要么整段错、要么半句错，取前者）")
	}
}

// TestMixedStrikeParagraphKeepsItsText：混排删除线同样不许"整段划掉"。
//
// 结构层看不到 `Strike` 位（`gfx.Text` 只暴露 `TextStyle`，删除线是绘制期叠的
// 一条线），所以这里钉两件能钉的：判据返回 false，且**文字一个字都没被吃掉**
// （曾经的错误做法是把 `~~` 之间的内容当成"要删掉"，那会少字）。
func TestMixedStrikeParagraphKeepsItsText(t *testing.T) {
	_, tree := newTranscriptCanvas()
	_, canvas := renderTranscript(t, assistantMarkdownText("~~要点~~：说明"), hostViewport(tree))
	text := mustCanvasText(t, canvas, "要点：说明")
	if text.Style.Italic {
		t.Fatalf("混排删除线不该顺带变成斜体：%+v", text.Style)
	}
	spans := markdownSpansOf(t, "~~要点~~：说明", markdown.Paragraph)
	if spansAllStrike(spans) {
		t.Fatal("`~~要点~~：说明` 被判成整块删除线：那样「：说明」也会被划掉")
	}
}

// TestAllItalicListItemPaintsItalicFace：列表项整块斜体同样生效（标记槽不动）。
func TestAllItalicListItemPaintsItalicFace(t *testing.T) {
	_, tree := newTranscriptCanvas()
	_, canvas := renderTranscript(t, assistantMarkdownText("- *要点*"), hostViewport(tree))
	text := mustCanvasText(t, canvas, "要点")
	if !text.Style.Italic {
		t.Fatalf("整块斜体的列表项 Style.Italic = false，应为 true")
	}
	// 标记槽仍是常规面与固定宽（正文的斜面不该染到 `::marker`）。
	marker := mustCanvasText(t, canvas, MarkdownListBullet)
	if marker.Style.Italic {
		t.Fatalf("列表标记 Style.Italic = true；官方 `::marker` 不在 `<em>` 里面")
	}
	if marker.Bounds.Width != MarkdownListMarkerWidth {
		t.Fatalf("列表标记槽宽 = %v，应为 %v", marker.Bounds.Width, MarkdownListMarkerWidth)
	}
}

// TestStrikeLeavesTheLayoutExactlyAsItWas 是删除线在**布局层**的判据。
//
// viva 的 `Box.Strike()` 契约明写 "a Text keeps its own width and wrapping, so
// nothing about the layout moves when a text is struck"：删除线是绘制期往字中间
// 叠一条线，不换字形、不改宽度、不改行盒。所以 `~~…~~` 与 `…` 的几何必须逐字段
// 相同 —— 这条同时是"不变慢"的机器判据（没有多出来的排版步骤）。
func TestStrikeLeavesTheLayoutExactlyAsItWas(t *testing.T) {
	value := "重点要点重点"
	_, plainTree := newTranscriptCanvas()
	_, plainCanvas := renderTranscript(t, assistantMarkdownText(value), hostViewport(plainTree))
	plain := mustCanvasText(t, plainCanvas, value)

	_, struckTree := newTranscriptCanvas()
	_, struckCanvas := renderTranscript(t, assistantMarkdownText("~~"+value+"~~"), hostViewport(struckTree))
	struck := mustCanvasText(t, struckCanvas, value)

	if struck.Bounds != plain.Bounds {
		t.Fatalf("删除线后的包围盒 = %v，常规 = %v；删除线不该动布局", struck.Bounds, plain.Bounds)
	}
	if struck.Style != plain.Style {
		t.Fatalf("删除线后的字样 = %+v，常规 = %+v；删除线不该改 TextStyle", struck.Style, plain.Style)
	}
	if len(struckCanvas.Texts()) != len(plainCanvas.Texts()) {
		t.Fatalf("删除线段落画了 %d 段文字，常规 %d 段；删除线不该多拆盒子",
			len(struckCanvas.Texts()), len(plainCanvas.Texts()))
	}
}

// TestAllItalicParagraphIsStillOneLabel："整块斜体"不许把一个段落拆成一堆盒子。
//
// 与 `#236` 的加粗那条同一个判据：画出来的字数与常规段落**逐条相同**。
func TestAllItalicParagraphIsStillOneLabel(t *testing.T) {
	value := "重点要点重点要点"
	_, plainTree := newTranscriptCanvas()
	_, plainCanvas := renderTranscript(t, assistantMarkdownText(value), hostViewport(plainTree))
	_, italicTree := newTranscriptCanvas()
	_, italicCanvas := renderTranscript(t, assistantMarkdownText("*"+value+"*"), hostViewport(italicTree))

	if len(italicCanvas.Texts()) != len(plainCanvas.Texts()) {
		t.Fatalf("斜体段落画了 %d 段文字，常规段落画了 %d 段；整块斜体不该多拆盒子",
			len(italicCanvas.Texts()), len(plainCanvas.Texts()))
	}
	if _, ok := canvasText(italicCanvas, value); !ok {
		t.Fatalf("斜体段落没有整段画出来：%v", italicCanvas.Texts())
	}
}

// ── ③ 量宽同源 ────────────────────────────────────────────────────────

// TestItalicMeasureStyleMatchesTheDrawnFace：行内排版**量宽的字样**必须与绘制同源。
//
// 段落里夹图时（`drawMarkdownInlineLayout`）宽度是按 `markdownInlineTextStyle`
// 算的、字是按 `textRun` 画的。斜体面的进宽与直立面不是同一套（像素层实测
// 104.077 vs 106.996，见视觉测试），只改绘制不改量宽会让图与文字的相对位置漂。
func TestItalicMeasureStyleMatchesTheDrawnFace(t *testing.T) {
	regular := markdownInlineTextStyle(markdownInlineFont{Size: 14}, false)
	italic := markdownInlineTextStyle(markdownInlineFont{Size: 14, Italic: true}, false)
	if regular.Italic {
		t.Fatalf("常规量宽字样 = %+v，不该带 Italic", regular)
	}
	if !italic.Italic {
		t.Fatalf("整块斜体的量宽字样 = %+v，应为 Italic（与画出来的那一面同源）", italic)
	}
	if italic.Size != regular.Size || italic.Weight != regular.Weight || italic.Mono != regular.Mono {
		t.Fatalf("斜体量宽字样 = %+v，除斜面外应与常规 %+v 相同", italic, regular)
	}

	// 叠加：`***…***` 量宽要同时是粗体与斜面。
	both := markdownInlineTextStyle(markdownInlineFont{Size: 14, Bold: true, Italic: true}, false)
	if both.Weight != gfx.WeightBold || !both.Italic {
		t.Fatalf("又粗又斜的量宽字样 = %+v，要 Weight=%v 且 Italic", both, gfx.WeightBold)
	}
}

// TestStrikeDoesNotReachTheMeasureStyle：删除线**不进**量宽字样。
//
// 这是 viva 契约（`Box.Strike()` 注释：布局不因删除线移动）；把它挡在量宽之外
// 才不会出现"为了划一条线把整段文字重新量一遍"的隐性代价。
func TestStrikeDoesNotReachTheMeasureStyle(t *testing.T) {
	style := markdownInlineTextStyle(markdownInlineFont{Size: 14}, false)
	if style.Italic {
		t.Fatalf("量宽字样带上了 Italic：%+v", style)
	}
	// 字号/字重/等宽面三样都与直立一致就是"没进量宽"（`gfx.TextStyle` 本身没有
	// 删除线字段，能表达删除线的只有绘制期的 `Box.Strike()`）。
	if style.Size != 14 || style.Weight != gfx.WeightRegular || style.Mono {
		t.Fatalf("量宽字样 = %+v，应与直立完全一致", style)
	}
}

// ── ④ 行内图路径（`#236` 在 `drawListItem` 的 `:716` 也加了样式，别把它弄坏）──

// TestAllItalicInlineImageRunPaintsItalicFace 钉**行内图分支**上的斜体。
//
// 段落里夹一张图时，正文不再是一个 `Wrap`，而是 `drawMarkdownInline` 回调出来
// 的**每个文字段一个盒子**（`official_transcript_view.go` 的 `drawListItem` /
// `drawParagraph` 内的 `textRun`）。这条分支与无图分支是两份代码，漏一处就会
// 出现"有图的段落不加粗 / 不斜"。
func TestAllItalicInlineImageRunPaintsItalicFace(t *testing.T) {
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	// 图片段也要在 `*…*` **里面**：官方 `<img>` 那一段的样式位继承自外层 emphasis，
	// 包在外面的话它不给整块判据投票以外的语义（图片本身没有斜面），而判据会因此
	// 返回 false —— 那是**正确**的混排结论，不是漏做（见下一条混排测试）。
	canvas := renderMarkdownImages(t, view, markdownImageEntry("*前文 ![示意](./shot.png) 后文*", false), pluginapi.Actions{})

	for _, want := range []string{"前文", "后文"} {
		text := mustCanvasTextTrimmed(t, canvas, want)
		if !text.Style.Italic {
			t.Fatalf("行内图分支上的 %q Style.Italic = false；这一段整块 `*…*`，斜面必须在行内 run 上也生效", want)
		}
	}
}

// TestAllBoldInlineImageRunStillPaintsBoldFace 是 `#236` 那条分支的**回归守卫**。
//
// 本单在同一个 `textRun` 回调里加了 `.Italic()` / `.Strike()`，最容易的破坏
// 方式是把 `if bold { box.Weight(...) }` 那段顺序或条件改错。这条钉住"加粗仍在"。
func TestAllBoldInlineImageRunStillPaintsBoldFace(t *testing.T) {
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	canvas := renderMarkdownImages(t, view, markdownImageEntry("**前文 ![示意](./shot.png) 后文**", false), pluginapi.Actions{})

	for _, want := range []string{"前文", "后文"} {
		text := mustCanvasTextTrimmed(t, canvas, want)
		if text.Style.Weight != gfx.WeightBold {
			t.Fatalf("行内图分支上的 %q 字重 = %v，应为 gfx.WeightBold（`#236` 的行为不许回退）",
				want, text.Style.Weight)
		}
		if text.Style.Italic {
			t.Fatalf("行内图分支上的 %q 被顺手画成斜体了：%+v", want, text.Style)
		}
	}
}

// TestMixedInlineImageRunStaysUpright 是行内图分支上的**混排边界**。
//
// `*前文*：说明` 里「：说明」不带样式位 ⇒ 整块判否 ⇒ 两段文字都该直立。
func TestMixedInlineImageRunStaysUpright(t *testing.T) {
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	canvas := renderMarkdownImages(t, view, markdownImageEntry("*前文*：说明 ![示意](./shot.png)", false), pluginapi.Actions{})

	text := mustCanvasTextTrimmed(t, canvas, "前文：说明")
	if text.Style.Italic {
		t.Fatalf("行内图分支的混排段落被画成斜体：%+v；只有整块 `*…*` 才该生效", text.Style)
	}
}

// TestAllItalicListItemInlineImageRunPaintsItalicFace 钉**列表项**里夹图的那条分支。
//
// 它与上面那条（段落里夹图）是**两份代码**：`drawListItem` 有图时自己建
// `ui.Column` + `drawMarkdownInline` 回调（`official_transcript_view.go` 的列表项
// 分支），`drawParagraph` 走的是另一份。只测段落会让 `drawListItem` 那一处的样式
// 漏掉而全绿 —— 本条就是补这个洞（变异实测：去掉那里一行 `.Italic()` 只有它能红）。
func TestAllItalicListItemInlineImageRunPaintsItalicFace(t *testing.T) {
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	canvas := renderMarkdownImages(t, view, markdownImageEntry("- *前文 ![示意](./shot.png) 后文*", false), pluginapi.Actions{})

	for _, want := range []string{"前文", "后文"} {
		text := mustCanvasTextTrimmed(t, canvas, want)
		if !text.Style.Italic {
			t.Fatalf("列表项行内图分支上的 %q Style.Italic = false；这一段整块 `*…*`", want)
		}
	}
}

// TestAllBoldListItemInlineImageRunStillPaintsBoldFace 是 `#236` 在 `:716` 那处
// 加粗的**回归守卫**（列表项版本）。
func TestAllBoldListItemInlineImageRunStillPaintsBoldFace(t *testing.T) {
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	canvas := renderMarkdownImages(t, view, markdownImageEntry("- **前文 ![示意](./shot.png) 后文**", false), pluginapi.Actions{})

	for _, want := range []string{"前文", "后文"} {
		text := mustCanvasTextTrimmed(t, canvas, want)
		if text.Style.Weight != gfx.WeightBold {
			t.Fatalf("列表项行内图分支上的 %q 字重 = %v，应为 gfx.WeightBold（`#236` 的行为不许回退）",
				want, text.Style.Weight)
		}
		if text.Style.Italic {
			t.Fatalf("列表项行内图分支上的 %q 被顺手画成斜体了：%+v", want, text.Style)
		}
	}
}

// TestMixedListItemInlineImageRunStaysUpright 是列表项行内图分支上的**混排边界**。
func TestMixedListItemInlineImageRunStaysUpright(t *testing.T) {
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	canvas := renderMarkdownImages(t, view, markdownImageEntry("- *前文*：说明 ![示意](./shot.png)", false), pluginapi.Actions{})

	text := mustCanvasTextTrimmed(t, canvas, "前文：说明")
	if text.Style.Italic {
		t.Fatalf("列表项行内图分支的混排段落被画成斜体：%+v；只有整块 `*…*` 才该生效", text.Style)
	}
}
