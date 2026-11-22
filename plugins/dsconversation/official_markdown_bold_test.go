package dsconversation

// official_markdown_bold_test.go —— live transcript 的**整块加粗**（官方 `<strong>`）。
//
// 判据分三层，这里是最便宜的一层（真 viva 画布 + 确定性字宽测量器）：
//
//	① 整块 `**…**` → 画出来的那段文字 `Style.Weight == gfx.WeightBold`（700 档）；
//	② 混排（`**要点**：说明`）→ 仍是 `WeightRegular`（viva 盒级没有行内 run，
//	   见 `spansAllBold` 的注释：这是记录在案的能力边界，不是漏做）；
//	③ 常规段落 → 仍是 `WeightRegular` 且行高/位置与加粗那一份**同一套几何**
//	   （字重只换字形，不改行盒高度——官方 `strong` 也只换字重）。
//
// 真像素判据（粗体比常规**宽**）在 `official_markdown_bold_visual_test.go`。

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// assistantMarkdownText 造一条助手正文（markdown 源文本）。
func assistantMarkdownText(text string) []TranscriptEntry {
	return []TranscriptEntry{{
		Key: "md-bold", Kind: ChatNodeAssistantStep, Seq: 1, Turn: 1,
		Visibility: ChatVisibilityVisible, Data: TranscriptData{Text: text},
	}}
}

// TestAllBoldParagraphPaintsBoldFace：整块 `**…**` 必须画成 700 档。
func TestAllBoldParagraphPaintsBoldFace(t *testing.T) {
	_, tree := newTranscriptCanvas()
	_, canvas := renderTranscript(t, assistantMarkdownText("**重点**"), hostViewport(tree))
	bold := mustCanvasText(t, canvas, "重点")
	if bold.Style.Weight != gfx.WeightBold {
		t.Fatalf("整块加粗的段落字重 = %v，应为 gfx.WeightBold(%v)", bold.Style.Weight, gfx.WeightBold)
	}
	// 字重不改字号/行高（官方 `strong` 只换字重，不换行盒高度）。
	if bold.Style.Size != 14 {
		t.Fatalf("整块加粗的段落字号 = %v，应为正文 14（字重不该改字号）", bold.Style.Size)
	}
}

// TestAllBoldParagraphDrawsTheSameLineBox：加粗与常规的行高、行位必须一样。
//
// 这条钉的是"字重只换字形"：如果哪天把加粗做成换字号/换行高，几何表里的
// 正文 14/22 就不再是这一段的行盒读数了。
func TestAllBoldParagraphDrawsTheSameLineBox(t *testing.T) {
	_, plainTree := newTranscriptCanvas()
	_, plainCanvas := renderTranscript(t, assistantMarkdownText("重点"), hostViewport(plainTree))
	plain := mustCanvasText(t, plainCanvas, "重点")

	_, boldTree := newTranscriptCanvas()
	_, boldCanvas := renderTranscript(t, assistantMarkdownText("**重点**"), hostViewport(boldTree))
	bold := mustCanvasText(t, boldCanvas, "重点")

	if bold.Bounds.Height != plain.Bounds.Height || bold.Bounds.Y != plain.Bounds.Y {
		t.Fatalf("整块加粗后的行盒 = {Y:%v H:%v}，常规 = {Y:%v H:%v}；字重不该动行盒几何",
			bold.Bounds.Y, bold.Bounds.Height, plain.Bounds.Y, plain.Bounds.Height)
	}
}

// TestPlainParagraphStaysRegularWeight：没加粗的段落仍然是常规字重（反向测试的画布层）。
func TestPlainParagraphStaysRegularWeight(t *testing.T) {
	_, tree := newTranscriptCanvas()
	_, canvas := renderTranscript(t, assistantMarkdownText("重点"), hostViewport(tree))
	text := mustCanvasText(t, canvas, "重点")
	if text.Style.Weight != gfx.WeightRegular {
		t.Fatalf("常规段落字重 = %v，应为 gfx.WeightRegular", text.Style.Weight)
	}
}

// TestMixedBoldParagraphStaysRegularWeight 钉住**能力边界**（不是"以后再说"的 TODO）。
//
// viva v0.6.12-ow 的盒级文字只有一个 `gfx.TextStyle`：一个 `Wrap` 一套字重，
// 没有行内 run 出口（`base/text_wrap.go` 的 `Node.textStyle` + `base/paint.go`
// 的 `paintText`）。所以 `**要点**：说明` 这种混排只能整段一个字重——现在取常规
// （与改动前逐像素一致），而不是"整段加粗"这种把半句话画错的方案。
//
// viva 有了盒级行内 run 之后，这一条要跟着改成"只有「要点」两个字是 700"。
func TestMixedBoldParagraphStaysRegularWeight(t *testing.T) {
	_, tree := newTranscriptCanvas()
	_, canvas := renderTranscript(t, assistantMarkdownText("**要点**：说明"), hostViewport(tree))
	text := mustCanvasText(t, canvas, "要点：说明")
	if text.Style.Weight != gfx.WeightRegular {
		t.Fatalf("混排段落字重 = %v；盒级没有行内 run 时只能取常规（要么整段错、要么半句错，取前者）", text.Style.Weight)
	}
}

// TestAllBoldListItemPaintsBoldFace：列表项整块加粗同样走 700（标记槽不动）。
func TestAllBoldListItemPaintsBoldFace(t *testing.T) {
	_, tree := newTranscriptCanvas()
	_, canvas := renderTranscript(t, assistantMarkdownText("- **要点**"), hostViewport(tree))
	text := mustCanvasText(t, canvas, "要点")
	if text.Style.Weight != gfx.WeightBold {
		t.Fatalf("整块加粗的列表项字重 = %v，应为 gfx.WeightBold", text.Style.Weight)
	}
	// 标记槽仍是常规字重与固定宽（加粗不该把标记顶宽）。
	marker := mustCanvasText(t, canvas, MarkdownListBullet)
	if marker.Style.Weight != gfx.WeightRegular {
		t.Fatalf("列表标记字重 = %v，应为常规", marker.Style.Weight)
	}
	if marker.Bounds.Width != MarkdownListMarkerWidth {
		t.Fatalf("列表标记槽宽 = %v，应为 %v", marker.Bounds.Width, MarkdownListMarkerWidth)
	}
}

// TestAllBoldParagraphIsStillOneLabel："整块加粗"不许把一个段落拆成一堆盒子。
//
// 这一条是"不变慢、不加步骤"的机器判据：画出来的字数与常规段落**逐条相同**。
func TestAllBoldParagraphIsStillOneLabel(t *testing.T) {
	value := "重点要点重点要点"
	_, plainTree := newTranscriptCanvas()
	_, plainCanvas := renderTranscript(t, assistantMarkdownText(value), hostViewport(plainTree))
	_, boldTree := newTranscriptCanvas()
	_, boldCanvas := renderTranscript(t, assistantMarkdownText("**"+value+"**"), hostViewport(boldTree))

	if len(boldCanvas.Texts()) != len(plainCanvas.Texts()) {
		t.Fatalf("加粗段落画了 %d 段文字，常规段落画了 %d 段；整块加粗不该多拆盒子",
			len(boldCanvas.Texts()), len(plainCanvas.Texts()))
	}
	if _, ok := canvasText(boldCanvas, value); !ok {
		t.Fatalf("加粗段落没有整段画出来：%v", boldCanvas.Texts())
	}
}

// TestBoldMeasureStyleMatchesTheDrawnWeight：行内排版**量宽的字样**必须与绘制同源。
//
// 段落里夹图时（`drawMarkdownInlineLayout`）宽度是按 `markdownInlineTextStyle`
// 算的、字是按 `textRun` 画的。两边字重不同源，图与文字的相对位置会漂——
// 而这一档在单测里看不见（要有图才走到），所以把它钉在量宽入口上。
func TestBoldMeasureStyleMatchesTheDrawnWeight(t *testing.T) {
	regular := markdownInlineTextStyle(markdownInlineFont{Size: 14}, false)
	bold := markdownInlineTextStyle(markdownInlineFont{Size: 14, Bold: true}, false)
	if regular.Weight != gfx.WeightRegular {
		t.Fatalf("常规量宽字重 = %v，应为 gfx.WeightRegular", regular.Weight)
	}
	if bold.Weight != gfx.WeightBold {
		t.Fatalf("加粗量宽字重 = %v，应为 gfx.WeightBold（与画出来的那一档同源）", bold.Weight)
	}
	if bold.Size != regular.Size || bold.Mono != regular.Mono {
		t.Fatalf("加粗量宽字样 = %+v，除字重外应与常规 %+v 相同", bold, regular)
	}
}
