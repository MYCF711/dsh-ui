package mdlite

// markdown 正文的**行内排版**（`#18` (b) ①：图夹在句子里）。
//
// # 为什么需要这一层
//
// 官方正文里的图片是**行内原子盒**，不是块：
//
//	OFF:dist/assets/index-CSAdOBx6.css
//	  ._imageButton_13jw0_471{display:inline-block;...;vertical-align:middle;cursor:zoom-in}
//	  ._imageButton_13jw0_471 ._image_13jw0_300{max-width:min(100%,640px);max-height:360px}
//
// `._image_13jw0_300` 那一条确实是 `display:block`，但**外层的 `<button>` 是
// `inline-block`**（`lK` 每次都套这层，chat 里 `fileImages` 恒有），所以整张图以
// 一个 inline-level 原子盒参与段落的行内流：它跟前后文字排在同一行，
// 垂直方向按 `vertical-align:middle` 对齐。把它画成块级会让图独占一行，
// 「文字 + 图 + 文字」这种写法就比官方**多一次换行**。
//
// 这一层只做**纯几何 + 纯切分**：不碰 ui、不碰位图、不认识颜色。谁量宽度、
// 每行多高、基线在哪，都是调用方的事（`plugins/dsconversation`）。
//
// 行盒的高度与基线用 CSS 2.1 §10.8 的公式：每个元素给出"基线以上多高(A)/
// 基线以下多深(D)"，行高 = max(A) + max(D)，基线在行顶下 max(A) 处。
// 文字段的 A 由调用方按它自己的字体算，图的 A/D 是 `H/2 ∓ 半个 x-height`（见调用方）。

import (
	"strings"

	"github.com/vivaui/viva/gfx"
)

// MarkdownInlineImage 是一张**行内原子盒**：显示尺寸已按官方两条 max-* 定好
// （`FitMarkdownImage`）。
type MarkdownInlineImage struct {
	// Destination 是目的地原文（灯箱与命中区按它区分）。
	Destination string
	// Alt 是官方的 alt（`![alt](目的地)` 里那个）。
	Alt string
	// Width / Height 是显示尺寸（像素，已取整）。
	Width  float32
	Height float32
	// Bitmap 是这一帧要画的那张位图（Ready 态才有）。
	Bitmap gfx.Bitmap
}

// MarkdownInlineSlot 是行内流上的一项：一段文字，或者一张原子图。
//
// 调用方按**原文顺序**把 `SplitMarkdownInline` 的切分结果翻成 slots：
// 文字段原样进来，图片段要么带 `Image`（Ready 态、有尺寸），要么退成
// **文字 slot**（Pending/Failed/Unresolved 三档官方画的都是 alt 文案，
// 那时它是行内文字，不是盒）。
type MarkdownInlineSlot struct {
	// Text 是这段文字。
	Text string
	// Mono 表示这段是行内代码（官方 `code`，等宽）。
	Mono bool
	// Alt 表示这段是图片的 alt / 失败文案（官方 `.imageAlt`，tertiary 色）。
	Alt bool
	// Image 非空 = 这是一个原子图（此时 Text/Mono/Alt 无意义）。
	Image *MarkdownInlineImage
}

// MarkdownInlineText 是同一行里**连续的同质**文字（合并相邻 token 之后）。
type MarkdownInlineText struct {
	// Text 是这一段的文字（行首空白已折掉，行尾空白只在后面还有图时保留）。
	Text string
	// Mono 见 MarkdownInlineSlot.Mono。
	Mono bool
	// Alt 见 MarkdownInlineSlot.Alt。
	Alt bool
}

// MarkdownInlineItem 是行上的一个元素：一段文字或一张图。
type MarkdownInlineItem struct {
	// Text 是文字段（Image == nil 时有意义）。
	Text MarkdownInlineText
	// Image 是原子图。
	Image *MarkdownInlineImage
	// Width 是这一项占的宽度：文字段 = 量出来的宽度，图 = 显示宽。
	Width float32
}

// MarkdownInlineLine 是一行。
type MarkdownInlineLine struct {
	// Items 是这一行从左到右的元素。
	Items []MarkdownInlineItem
	// Width 是这一行占的总宽（含行内空格；行尾空白已折掉）。
	Width float32
}

// MarkdownInlineMeasure 量一段文字在某档字体下的宽度。
//
// mono 由调用方映射成它自己的等宽字体（本仓正文字号 14 / 行内代码 13）。
type MarkdownInlineMeasure func(text string, mono bool) float32

// inlineToken 是排版时不可再分的一小段。
type inlineToken struct {
	text  string
	mono  bool
	alt   bool
	image *MarkdownInlineImage
	width float32
	space bool
	hard  bool
}

// LayoutMarkdownInline 按宽度把 slots 排成行。
//
// 规则照官方的行内流（CSS 2.1 §9.4.2 的贪心换行）：
//
//	词 / CJK 字 / 空白     正常换行（CJK 每字可断，与 `overflow-wrap:anywhere` 一致）
//	比整行还宽的词          按 rune 硬切（官方 `overflow-wrap:anywhere`）
//	图                      原子：放不下就整张换到下一行（不缩、不拆）
//	换行符（`markdown.Newline`） 硬断行（**与本仓既有 Wrap 同一条口径**）
//	行首 / 行尾空白         折叠（行尾空白只在后面还有图时保留——图是行内的）
//
// maxWidth 是正文列宽；measure 是调用方的量宽入口（nil → 返回 nil，不排版）。
func LayoutMarkdownInline(slots []MarkdownInlineSlot, maxWidth float32, measure MarkdownInlineMeasure) []MarkdownInlineLine {
	if measure == nil || len(slots) == 0 {
		return nil
	}
	if maxWidth < 1 {
		maxWidth = 1
	}
	var tokens []inlineToken
	for _, slot := range slots {
		if slot.Image != nil {
			tokens = append(tokens, inlineToken{image: slot.Image, width: slot.Image.Width})
			continue
		}
		tokens = append(tokens, markdownInlineTokens(slot, maxWidth, measure)...)
	}
	var lines []MarkdownInlineLine
	cur := make([]inlineToken, 0, len(tokens))
	var curWidth float32
	flush := func(force bool) {
		line, ok := mergeMarkdownInlineTokens(cur, measure)
		if ok || force {
			lines = append(lines, line)
		}
		cur = cur[:0]
		curWidth = 0
	}
	for _, tok := range tokens {
		if tok.hard {
			// 硬换行：有内容就断；本身就是空行时也占一行（与本仓 Wrap 的
			// `splitLine` 同一条口径：`"a\n\nb"` 是三个行盒）。
			flush(len(cur) == 0)
			continue
		}
		if tok.space && len(cur) == 0 {
			continue // 行首空白折叠
		}
		if curWidth+tok.width > maxWidth && len(cur) > 0 {
			flush(false)
			if tok.space {
				continue
			}
		}
		cur = append(cur, tok)
		curWidth += tok.width
	}
	flush(false)
	return lines
}

// markdownInlineTokens 把一段文字切成词 / 空白 / 硬换行 token。
//
// 切法与 `usertext.go` 的 `textTokens` 同一条口径（那个渲染器已经在做行内混排）：
// 空白断词、CJK 每字一个 token、其余按词。区别只有两条：换行符是硬断行，
// 以及比整行还宽的词按 rune 硬切（官方 `overflow-wrap:anywhere`）。
func markdownInlineTokens(slot MarkdownInlineSlot, maxWidth float32, measure MarkdownInlineMeasure) []inlineToken {
	out := make([]inlineToken, 0, 8)
	var word strings.Builder
	flushWord := func() {
		if word.Len() == 0 {
			return
		}
		text := word.String()
		word.Reset()
		width := measure(text, slot.Mono)
		if width > maxWidth {
			out = append(out, markdownInlineCutRunes(text, slot, maxWidth, measure)...)
			return
		}
		out = append(out, inlineToken{text: text, mono: slot.Mono, alt: slot.Alt, width: width})
	}
	for _, r := range slot.Text {
		switch {
		case r == ' ' || r == '\t':
			flushWord()
			text := " "
			out = append(out, inlineToken{text: text, mono: slot.Mono, alt: slot.Alt, space: true, width: measure(text, slot.Mono)})
		case r == '\n' || r == '\r':
			flushWord()
			out = append(out, inlineToken{hard: true})
		case isMarkdownInlineWideRune(r):
			// CJK：每个字都可断行（官方正文 `overflow-wrap:anywhere` + CJK 的默认断行）。
			flushWord()
			text := string(r)
			out = append(out, inlineToken{text: text, mono: slot.Mono, alt: slot.Alt, width: measure(text, slot.Mono)})
		default:
			word.WriteRune(r)
		}
	}
	flushWord()
	return out
}

// markdownInlineCutRunes 把比整行还宽的词按 rune 硬切成若干段，每段不超过 maxWidth。
//
// 一个 rune 就比整行还宽（超宽 CJK 字体 / 一个 emoji）：**原样保留**，
// 让它溢出——拆开一个字符只会画出半截字，比溢出更坏。
func markdownInlineCutRunes(text string, slot MarkdownInlineSlot, maxWidth float32, measure MarkdownInlineMeasure) []inlineToken {
	var out []inlineToken
	var cur strings.Builder
	var curWidth float32
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		out = append(out, inlineToken{text: cur.String(), mono: slot.Mono, alt: slot.Alt, width: curWidth})
		cur.Reset()
		curWidth = 0
	}
	for _, r := range text {
		part := string(r)
		width := measure(part, slot.Mono)
		if curWidth+width > maxWidth && cur.Len() > 0 {
			flush()
		}
		cur.WriteRune(r)
		curWidth += width
	}
	flush()
	return out
}

// isMarkdownInlineWideRune 是"每字可断"的那一片：CJK / 假名 / 全角标点 / 兼容汉字。
func isMarkdownInlineWideRune(r rune) bool {
	return r >= 0x2e80 && r <= 0x9fff || r >= 0xf900 && r <= 0xfaff || r >= 0xff00 && r <= 0xffef
}

// mergeMarkdownInlineTokens 把一行的 token 合并成 items：相邻同质文字合成一段。
//
// 为什么要合并：一段文字画成一个 label，字形才是**整段整形**的结果（连字、字距）；
// 逐个 token 画会把词与词之间的空隙理由交给布局，像素会和整段画差一点。
// 合并后的宽度**重新量一遍**（不拿 token 宽度相加）：量法唯一，图与文字的
// 相对位置才不会被两种算法拼出来。
func mergeMarkdownInlineTokens(tokens []inlineToken, measure MarkdownInlineMeasure) (MarkdownInlineLine, bool) {
	line := MarkdownInlineLine{}
	for i := 0; i < len(tokens); {
		tok := tokens[i]
		if tok.image != nil {
			line.Items = append(line.Items, MarkdownInlineItem{Image: tok.image, Width: tok.width})
			line.Width += tok.width
			i++
			continue
		}
		run := MarkdownInlineText{Mono: tok.mono, Alt: tok.alt}
		var b strings.Builder
		for i < len(tokens) && tokens[i].image == nil && tokens[i].mono == run.Mono && tokens[i].alt == run.Alt {
			b.WriteString(tokens[i].text)
			i++
		}
		text := b.String()
		if text == "" {
			continue
		}
		width := measure(text, run.Mono)
		run.Text = text
		line.Items = append(line.Items, MarkdownInlineItem{Text: run, Width: width})
		line.Width += width
	}
	// 行尾空白：后面还有图时保留（图是行内的，文字与图之间那格空白要占位），
	// 否则折掉（正是 `Wrap` 的 `trimSpaceRight`）。
	if n := len(line.Items); n > 0 && line.Items[n-1].Image == nil {
		last := &line.Items[n-1]
		if trimmed := strings.TrimRight(last.Text.Text, " "); trimmed != last.Text.Text {
			last.Text.Text = trimmed
			last.Width = measure(trimmed, last.Text.Mono)
			line.Width = 0
			for _, item := range line.Items {
				line.Width += item.Width
			}
		}
	}
	// 全空白的一行（只有折叠掉的行尾空白）：按空行处理，有东西才返回 true。
	painted := false
	for _, item := range line.Items {
		if item.Image != nil || item.Text.Text != "" {
			painted = true
			break
		}
	}
	return line, painted
}
