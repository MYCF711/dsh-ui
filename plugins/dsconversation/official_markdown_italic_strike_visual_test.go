//go:build windows || darwin || linux

// official_markdown_italic_strike_visual_test.go —— 整块斜体与整块删除线的**真像素**证据。
//
// 判据（本仓成熟做法：不看文字标签的声明，看画出来的像素）：
//
//	① 斜体正判据：字形**真的倾斜** —— 同一段文字，斜面下顶行的墨迹比底行整体
//	   右移（`TestItalicGlyphsReallySlant`）。只看 `Style.Italic` 是 flag 层，
//	   证明不了"画的时候真的用了斜体面"。
//	② 斜体量宽判据：斜体面量出来的宽度与直立面**不同**，且画面上量宽与绘制
//	   用的是同一面（`TestItalicWidthComesFromTheItalicFace`）。
//	③ 删除线正判据：字中间真的多出一条**1 像素高**的横线，落在行盒中线上、
//	   横跨整段文字（`TestStrikeDrawsOnePixelLineThroughTheMiddle`）。
//	④ 反向判据：**常规文本逐像素不变** —— 把 transcript 画的常规段落（以及
//	   混排段落，`*alpha bravo* charlie` / `~~alpha bravo~~ charlie`）与"同一档
//	   正文样式直接摆一个纯文本 `Wrap`"的参照页逐像素比对，必须全等
//	   （`TestPlainAndMixedPixelsEqualAPlainWrap`）。混排那两条同时是
//	   "不许误判"的反向断言。
//	⑤ 布局判据：斜面与删除线都不动行盒（`TestItalicDoesNotMoveTheLineBox`）。
//
// 为什么用参照页而不是钉一张 golden PNG：golden 会随机器字体/DPI 变化，而"常规
// 段落 == 同一档样式的纯 `Wrap`"是机器无关的等式。夹具函数 `openTranscriptPixels`
// / `openPlainWrapPixels` / `paintedText` / `inkBounds` 与 `#236` 的
// `official_markdown_bold_visual_test.go` 共用同一套（同一档正文样式）。
package dsconversation

import (
	"image"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
)

// rowInkEdges 是一个有墨迹的像素行的左右边界。
type rowInkEdges struct {
	y           int
	left, right int
}

// rowsWithInk 收集矩形范围内**每一行**的墨迹左右边界（自上而下）。
//
// 与 `inkBounds` 同一个墨迹判据（亮度差超过 `boldInkThreshold`），只是按行切开：
// 判断"字形是否倾斜"要看逐行的水平位置，而不是整块的包围框。
func rowsWithInk(picture image.Image, rect gfx.Rect, background float64) []rowInkEdges {
	bounds := picture.Bounds()
	var rows []rowInkEdges
	for y := int(rect.Y); y < int(rect.Y+rect.Height); y++ {
		if y < bounds.Min.Y || y >= bounds.Max.Y {
			continue
		}
		left, right := -1, -1
		for x := int(rect.X); x < int(rect.X+rect.Width); x++ {
			if x < bounds.Min.X || x >= bounds.Max.X {
				continue
			}
			r, g, b, _ := picture.At(x, y).RGBA()
			if diff := luma(r, g, b) - background; diff < boldInkThreshold && diff > -boldInkThreshold {
				continue
			}
			if left < 0 {
				left = x
			}
			right = x
		}
		if left >= 0 {
			rows = append(rows, rowInkEdges{y: y, left: left, right: right})
		}
	}
	return rows
}

// TestItalicGlyphsReallySlant 是斜体的真像素判据：**字形真的倾斜**。
//
// 夹具用单个大写 `H`：它有两条贯通全高的竖笔和一条中间横梁，直立时每一行的
// 左右边界完全相同；换到斜体面之后整个字形向右倒，**顶行右移、底行不动**。
// 于是"顶行的左边界 > 底行的左边界"且"顶行的右边界 > 底行的右边界"就是
// "真的倾斜"的机器判据，与 `Style.Italic` 这个 flag 无关。
//
// 同一条判据在直立页上必须给出**零位移**——否则说明这个判据本身会把直立字形
// 读成斜的（那样下面的断言就没有意义了）。
func TestItalicGlyphsReallySlant(t *testing.T) {
	const value = "H"
	plainSession, theme, plainPicture := openTranscriptPixels(t, value)
	italicSession, _, italicPicture := openTranscriptPixels(t, "*"+value+"*")
	background := backgroundLuma(theme)

	plainText, plainWidth := paintedText(t, plainSession, value)
	italicText, italicWidth := paintedText(t, italicSession, value)

	if plainText.Style.Italic {
		t.Fatalf("常规段落 Style.Italic = true；斜体只该落在整块 `*…*` 上")
	}
	if !italicText.Style.Italic {
		t.Fatalf("整块斜体段落 Style.Italic = false，应为 true")
	}

	plainRows := rowsWithInk(plainPicture, plainText.Bounds, background)
	italicRows := rowsWithInk(italicPicture, italicText.Bounds, background)
	if len(plainRows) < 3 || len(italicRows) < 3 {
		t.Fatalf("`H` 的行数太少（常规 %d 行、斜体 %d 行），判倾斜要足够高", len(plainRows), len(italicRows))
	}

	// 直立：顶行与底行的左右边界必须完全一样（柱状字形）。
	plainTop, plainBottom := plainRows[0], plainRows[len(plainRows)-1]
	if plainTop.left != plainBottom.left || plainTop.right != plainBottom.right {
		t.Fatalf("直立 `H` 的顶行 = x%d..%d、底行 = x%d..%d；直立字形上下边界该一样（判据本身有问题）",
			plainTop.left, plainTop.right, plainBottom.left, plainBottom.right)
	}

	// 斜面：顶行要整体比底行靠右。
	italicTop, italicBottom := italicRows[0], italicRows[len(italicRows)-1]
	leftShift := italicTop.left - italicBottom.left
	rightShift := italicTop.right - italicBottom.right
	if leftShift <= 0 {
		t.Fatalf("斜体 `H` 顶行左边界 x=%d、底行 x=%d（位移 %d）；斜体字形的顶行必须比底行靠右",
			italicTop.left, italicBottom.left, leftShift)
	}
	if rightShift <= 0 {
		t.Fatalf("斜体 `H` 顶行右边界 x=%d、底行 x=%d（位移 %d）；斜体字形的顶行必须比底行靠右",
			italicTop.right, italicBottom.right, rightShift)
	}
	if italicWidth >= plainWidth {
		t.Fatalf("斜体量宽 = %v，直立 = %v；换到斜体面之后进宽必须变（否则量宽没走斜体面）",
			italicWidth, plainWidth)
	}
	t.Logf("ITALIC_SLANT 直立量宽=%.3f 顶行 x%d..%d 底行 x%d..%d | 斜体量宽=%.3f 顶行 x%d..%d 底行 x%d..%d（位移 左%d 右%d）",
		plainWidth, plainTop.left, plainTop.right, plainBottom.left, plainBottom.right,
		italicWidth, italicTop.left, italicTop.right, italicBottom.left, italicBottom.right,
		leftShift, rightShift)
}

// TestItalicDoesNotMoveTheLineBox：斜面只换字形面，不改行盒高度与行位。
//
// 与 `#236` 的加粗同一条判据：几何表里正文 14/22 的行盒读数不因斜面变化。
func TestItalicDoesNotMoveTheLineBox(t *testing.T) {
	const value = "emphasis"
	plainSession, _, _ := openTranscriptPixels(t, value)
	italicSession, _, _ := openTranscriptPixels(t, "*"+value+"*")

	plainText, _ := paintedText(t, plainSession, value)
	italicText, _ := paintedText(t, italicSession, value)

	if italicText.Bounds.Y != plainText.Bounds.Y || italicText.Bounds.Height != plainText.Bounds.Height {
		t.Fatalf("斜体行盒 = {Y:%v H:%v}，常规 = {Y:%v H:%v}；斜面不该动行盒几何",
			italicText.Bounds.Y, italicText.Bounds.Height, plainText.Bounds.Y, plainText.Bounds.Height)
	}
}

// TestStrikeDrawsOnePixelLineThroughTheMiddle 是**段落**删除线的真像素判据。
//
// 判据全部落在 `assertStrikeLine` 里（段落与列表项共用）；这里只给夹具：
// 一段不带样式的正文 vs 同一句话整块 `~~…~~`。
func TestStrikeDrawsOnePixelLineThroughTheMiddle(t *testing.T) {
	const value = "Hamburgefonstiv"
	assertStrikeLine(t, value, "~~"+value+"~~", value)
}

// TestStrikeListItemDrawsTheLine 是**列表项**删除线的同一条判据。
//
// `drawListItem` 有两条绘制路径（无图一条 `Wrap`、有图一条 `textRun` 回调），
// 与段落是两份代码 —— 只测段落会让列表项上那一处 `.Strike()` 漏掉而全绿。
func TestStrikeListItemDrawsTheLine(t *testing.T) {
	const value = "Hamburgefonstiv"
	assertStrikeLine(t, "- "+value, "- ~~"+value+"~~", value)
}

// assertStrikeLine 断言 `struckMarkdown` 相对 `plainMarkdown` 只多了**一条 1 像素
// 高的横线**，且这条线落在行盒中线上、横跨整段文字。
//
// 证据分三条，缺一不可：
//
//	① 删除线**只**增加一个像素行：多出来的墨迹全部落在同一个 `y` 上 —— 这就是
//	   "1 像素高的横线"；
//	② 那一行的位置是**行盒中线**：`Bounds.Y + Bounds.Height/2`（viva 的
//	   `paintStrike` 就画在行盒中线上，见 `base/paint.go`）；
//	③ 它**横跨整段文字**：该行墨迹在 x 上的跨度接近这段文字的量宽。
//
// 另外钉住删除线不动布局：两页的文字包围盒与 `TextStyle` 必须逐字段相同。
func assertStrikeLine(t *testing.T, plainMarkdown, struckMarkdown, value string) {
	t.Helper()
	plainSession, theme, plainPicture := openTranscriptPixels(t, plainMarkdown)
	strikeSession, _, strikePicture := openTranscriptPixels(t, struckMarkdown)
	background := backgroundLuma(theme)

	plainText, plainWidth := paintedText(t, plainSession, value)
	strikeText, _ := paintedText(t, strikeSession, value)

	if strikeText.Bounds != plainText.Bounds {
		t.Fatalf("删除线后的包围盒 = %v，常规 = %v；删除线是叠加，不该动布局",
			strikeText.Bounds, plainText.Bounds)
	}
	if strikeText.Style != plainText.Style {
		t.Fatalf("删除线后的字样 = %+v，常规 = %+v；删除线不该改 TextStyle", strikeText.Style, plainText.Style)
	}

	// 逐像素求差：只在删除线页有墨迹、常规页没有的位置。
	extraRows := map[int][]int{}
	for y := int(strikeText.Bounds.Y); y < int(strikeText.Bounds.Y+strikeText.Bounds.Height); y++ {
		for x := int(strikeText.Bounds.X); x < int(strikeText.Bounds.X+strikeText.Bounds.Width); x++ {
			if !inkAt(strikePicture, x, y, background) || inkAt(plainPicture, x, y, background) {
				continue
			}
			extraRows[y] = append(extraRows[y], x)
		}
	}
	if len(extraRows) == 0 {
		t.Fatalf("%q 与 %q 逐像素相同：一个字都没划", struckMarkdown, plainMarkdown)
	}
	if len(extraRows) != 1 {
		rows := make([]int, 0, len(extraRows))
		for y := range extraRows {
			rows = append(rows, y)
		}
		t.Fatalf("删除线多出来的墨迹落在 %d 个像素行 %v；1 像素高的横线只该占**一行**", len(extraRows), rows)
	}

	var line int
	var columns []int
	for y, xs := range extraRows {
		line, columns = y, xs
	}
	middle := int(strikeText.Bounds.Y + strikeText.Bounds.Height/2)
	if line != middle {
		t.Fatalf("横线在第 %d 行，行盒中线是第 %d 行（Bounds=%v）；删除线要画在字中间",
			line, middle, strikeText.Bounds)
	}
	span := columns[len(columns)-1] - columns[0] + 1
	if float32(span) < plainWidth*0.8 {
		t.Fatalf("%q 的横线在 x 上只跨了 %d 像素，这段文字量宽 %v（要求 ≥80%%=%v）；删除线要横跨整段文字",
			struckMarkdown, span, plainWidth, plainWidth*0.8)
	}
	t.Logf("STRIKE_LINE %q 行=%d（中线=%d）横线 x=%d..%d 跨 %d 像素 / 量宽 %.3f 像素数=%d",
		struckMarkdown, line, middle, columns[0], columns[len(columns)-1], span, plainWidth, len(columns))
}

// paintedTextPrefix 在帧里按**前缀**找一段文字，连它这一帧量出来的宽度一起返回。
//
// 行内图分支把正文按原文切段，文字段保留原文空格（`"Hamburgefonstiv "`），
// 用 `paintedText` 的整值匹配会落空 —— 这正是"这一段真的走了行内 run 路径"的
// 旁证，所以测试按前缀取。
func paintedTextPrefix(t *testing.T, session *headless.Session, prefix string) (gfx.Text, float32) {
	t.Helper()
	snapshot := session.Snapshot()
	for i, text := range snapshot.Texts {
		if !strings.HasPrefix(text.Value, prefix) {
			continue
		}
		width := float32(0)
		if i < len(snapshot.TextWidths) {
			width = snapshot.TextWidths[i]
		}
		return text, width
	}
	var got []string
	for _, text := range snapshot.Texts {
		got = append(got, text.Value)
	}
	t.Fatalf("帧里没有以 %q 开头的文字；实际文本：%v", prefix, got)
	return gfx.Text{}, 0
}

// inkAt 报告某个像素在底色上算不算墨迹。
func inkAt(picture image.Image, x, y int, background float64) bool {
	if !image.Pt(x, y).In(picture.Bounds()) {
		return false
	}
	r, g, b, _ := picture.At(x, y).RGBA()
	diff := luma(r, g, b) - background
	return diff >= boldInkThreshold || diff <= -boldInkThreshold
}

// TestPlainAndMixedPixelsEqualAPlainWrap 是**反向判据**：常规文本逐像素不变。
//
// 三条一起比，缺一不可：
//
//	plain        完全没写样式的段落 —— 改动前后都必须与纯文本 `Wrap` 全等；
//	mixedItalic  `*alpha bravo* charlie` —— 混排按设计整段直立（盒级没有行内
//	             run），所以它必须与纯文本**逐像素相同**，不是"差不多"；
//	mixedStrike  `~~alpha bravo~~ charlie` —— 同理：绝不能整段划掉。
//
// 后两条同时是"不许误判"的反向断言：任何把整块判据放宽到混排的改动都会让它们红
// （划线/倾斜的那几十个像素立刻对不上）。
func TestPlainAndMixedPixelsEqualAPlainWrap(t *testing.T) {
	for _, tc := range []struct{ name, markdown, value string }{
		{"plain", "alpha bravo charlie", "alpha bravo charlie"},
		{"mixedItalic", "*alpha bravo* charlie", "alpha bravo charlie"},
		{"mixedStrike", "~~alpha bravo~~ charlie", "alpha bravo charlie"},
	} {
		t.Run(tc.name, func(t *testing.T) { assertUprightPixels(t, tc.markdown, tc.value) })
	}
}

// assertUprightPixels 断言 markdown 画出来的那段文字是**直立的**，且与"纯文本
// `Wrap`"参照页逐像素相同。
func assertUprightPixels(t *testing.T, markdown, value string) {
	t.Helper()
	session, _, picture := openTranscriptPixels(t, markdown)
	text, _ := paintedText(t, session, value)
	if text.Style.Italic {
		t.Fatalf("%q 画成了斜体；只有整块 `*…*` 才该生效（盒级没有行内 run）", markdown)
	}

	reference, referencePicture := openPlainWrapPixels(t, value, text.Bounds.X, text.Bounds.Y)
	referenceText, _ := paintedText(t, reference, value)
	if referenceText.Bounds.X != text.Bounds.X || referenceText.Bounds.Y != text.Bounds.Y {
		t.Fatalf("参照页文字位置 = (%v,%v)，transcript = (%v,%v)；参照页必须同位",
			referenceText.Bounds.X, referenceText.Bounds.Y, text.Bounds.X, text.Bounds.Y)
	}
	if referenceText.Style != text.Style {
		t.Fatalf("参照页字样 = %+v，transcript = %+v；两边必须同一档样式", referenceText.Style, text.Style)
	}

	x0, y0 := int(text.Bounds.X), int(text.Bounds.Y)
	x1, y1 := int(text.Bounds.X+text.Bounds.Width), int(text.Bounds.Y+text.Bounds.Height)
	compared := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			ar, ag, ab, aa := picture.At(x, y).RGBA()
			br, bg, bb, ba := referencePicture.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				t.Fatalf("常规文本像素在 (%d,%d) 变了：transcript=%v，参照页=%v；%q 必须逐像素不变",
					x, y, []uint32{ar, ag, ab, aa}, []uint32{br, bg, bb, ba}, markdown)
			}
			compared++
		}
	}
	if compared == 0 {
		t.Fatal("没有比较任何像素：矩形是空的")
	}
	t.Logf("PLAIN_PIXELS_IDENTICAL %q 比较了 %d 个像素（矩形 %v）", markdown, compared, text.Bounds)
}

// TestStrikeIsTheOnlyDifferenceInAStruckParagraph 是删除线的**反向**判据。
//
// 删除线页与常规页的差别必须**只有那一条横线**：把横线那一行的额外像素剔除之后，
// 两页在文字矩形内必须逐像素相同（字形一个都没换、位置一个都没动）。
// 这条与上面"横线只占一行"互为因果：一边证明线在，一边证明除了线没别的变化。
func TestStrikeIsTheOnlyDifferenceInAStruckParagraph(t *testing.T) {
	const value = "Hamburgefonstiv"
	plainSession, theme, plainPicture := openTranscriptPixels(t, value)
	strikeSession, _, strikePicture := openTranscriptPixels(t, "~~"+value+"~~")
	background := backgroundLuma(theme)

	plainText, _ := paintedText(t, plainSession, value)
	strikeText, _ := paintedText(t, strikeSession, value)
	if strikeText.Bounds != plainText.Bounds {
		t.Fatalf("删除线动了布局：%v ≠ %v", strikeText.Bounds, plainText.Bounds)
	}

	line := -1
	for y := int(strikeText.Bounds.Y); y < int(strikeText.Bounds.Y+strikeText.Bounds.Height); y++ {
		for x := int(strikeText.Bounds.X); x < int(strikeText.Bounds.X+strikeText.Bounds.Width); x++ {
			if !inkAt(strikePicture, x, y, background) || inkAt(plainPicture, x, y, background) {
				continue
			}
			if line < 0 {
				line = y
				continue
			}
			if line != y {
				t.Fatalf("删除线多出来的墨迹横跨第 %d 行与第 %d 行；只该有一条 1 像素横线", line, y)
			}
		}
	}
	if line < 0 {
		t.Fatal("删除线页与常规页逐像素相同：一个字都没划")
	}

	compared := 0
	for y := int(strikeText.Bounds.Y); y < int(strikeText.Bounds.Y+strikeText.Bounds.Height); y++ {
		if y == line {
			continue
		}
		for x := int(strikeText.Bounds.X); x < int(strikeText.Bounds.X+strikeText.Bounds.Width); x++ {
			ar, ag, ab, aa := strikePicture.At(x, y).RGBA()
			br, bg, bb, ba := plainPicture.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				t.Fatalf("横线以外的像素在 (%d,%d) 变了：删除线页=%v，常规页=%v；删除线只该加一条线",
					x, y, []uint32{ar, ag, ab, aa}, []uint32{br, bg, bb, ba})
			}
			compared++
		}
	}
	if compared == 0 {
		t.Fatal("没有比较任何像素：矩形是空的")
	}
	t.Logf("STRIKE_ONLY_LINE 横线在第 %d 行；其余 %d 个像素与常规页逐像素相同", line, compared)
}

// TestStrikeListItemInlineImageRunDrawsTheLine 钉**列表项行内图分支**上的删除线。
//
// 这一处的 `.Strike()` 与 `.Italic()` 是同一个回调里的两行、却和上面两条测试
// 不同的代码（`drawListItem` 有图那条），所以单独钉一条：变异实测去掉那一行
// 只有本条会红。
//
// 夹具用空 alt 的 `![](./shot.png)`：alt 文案分支（`drawMarkdownInlineLayout` 的
// `p.item.Text.Alt`）**不经过** `textRun` 回调，那是既有边界（`#236` 也一样，
// 本单不动它）；空 alt 让那一档画不出字，横线判据就落在真正的文字段上。
func TestStrikeListItemInlineImageRunDrawsTheLine(t *testing.T) {
	const prefix = "Hamburgefonstiv"
	plainSession, theme, plainPicture := openTranscriptPixels(t, "- "+prefix+" ![](./shot.png)")
	strikeSession, _, strikePicture := openTranscriptPixels(t, "- ~~"+prefix+" ![](./shot.png)~~")
	background := backgroundLuma(theme)

	plainText, plainWidth := paintedTextPrefix(t, plainSession, prefix)
	strikeText, strikeWidth := paintedTextPrefix(t, strikeSession, prefix)

	if strikeText.Bounds != plainText.Bounds {
		t.Fatalf("删除线后的包围盒 = %v，常规 = %v；删除线不该动布局", strikeText.Bounds, plainText.Bounds)
	}
	if strikeText.Style != plainText.Style {
		t.Fatalf("删除线后的字样 = %+v，常规 = %+v；删除线不该改 TextStyle", strikeText.Style, plainText.Style)
	}

	extraRows := map[int][]int{}
	for y := int(strikeText.Bounds.Y); y < int(strikeText.Bounds.Y+strikeText.Bounds.Height); y++ {
		for x := int(strikeText.Bounds.X); x < int(strikeText.Bounds.X+strikeText.Bounds.Width); x++ {
			if !inkAt(strikePicture, x, y, background) || inkAt(plainPicture, x, y, background) {
				continue
			}
			extraRows[y] = append(extraRows[y], x)
		}
	}
	if len(extraRows) == 0 {
		t.Fatal("列表项行内图分支上没有画出删除线：字一个都没划")
	}
	if len(extraRows) != 1 {
		t.Fatalf("删除线多出来的墨迹落在 %d 个像素行；1 像素高的横线只该占一行", len(extraRows))
	}
	var line int
	var columns []int
	for y, xs := range extraRows {
		line, columns = y, xs
	}
	middle := int(strikeText.Bounds.Y + strikeText.Bounds.Height/2)
	if line != middle {
		t.Fatalf("横线在第 %d 行，行盒中线是第 %d 行；删除线要画在字中间", line, middle)
	}
	span := columns[len(columns)-1] - columns[0] + 1
	if float32(span) < plainWidth*0.8 {
		t.Fatalf("横线在 x 上只跨了 %d 像素，这段文字量宽 %v（要求 ≥80%%）", span, plainWidth)
	}
	t.Logf("STRIKE_LINE_ITEM_INLINE x=%d..%d 跨 %d 像素 / 量宽 %.3f 行=%d（中线=%d）",
		columns[0], columns[len(columns)-1], span, strikeWidth, line, middle)
}
