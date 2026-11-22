// official_markdown_inline.go —— 正文行内混排：图夹在句子里（`#18` (b) ①）。
//
// # 官方语义（rc.5，坐标都在 `OFF:` 前缀的 rc.5 前端包里）
//
//	OFF:dist/assets/index-Ciz7DZxv.js  function lK(...)
//	  <button type=button class=imageButton onClick=开灯箱><img class=image ...></button>
//	  （`preview` 恒有 —— `fileImages` 由 `MarkdownDelegateProvider` 注入，见
//	   dsh-client-ui-chat/lib/client.js 的 ChatView：`{fileImages:{resolve,resourceAddress,labels}}`）
//	OFF:dist/assets/index-CSAdOBx6.css
//	  ._imageButton_13jw0_471{display:inline-block;max-width:100%;...;vertical-align:middle;cursor:zoom-in}
//	  ._imageButton_13jw0_471 ._image_13jw0_300{max-width:min(100%,640px);max-height:360px}
//	  ._image_13jw0_300{display:block;...}      ← 说的是 button **内部**的那张图
//
// 所以官方正文里的图是 `inline-block`（inline-level 原子盒）：`<p>文字 <button>图
// </button> 文字</p>` 三者在**同一个行内流**里。块级那种画法会让图独占一行，
// 「图夹在句中」比官方多一次换行 —— 旧账登记的正是这一条。
//
// # 这一层怎么画
//
// 行的**切分与宽度**在 `mdlite.LayoutMarkdownInline`（纯几何）；这里负责：
//
//	量宽      canvas.TextWidthStyle（与 viva 的 Wrap 同一个量法，见 textStyle）
//	行盒高/基线 CSS 2.1 §10.8：行高 = max(A) + max(D)，基线在行顶下 max(A)
//	垂直对齐   `vertical-align:middle` → 图的 A = H/2 - 半个 x-height（见 markdownInlineImageAscent）
//	落点      每行一个 Row，行内元素按计算好的宽度左起排、带各自的 A 偏移
//
// **没有图片的段落不走这里**（`drawMarkdownInline` 里那条老路径逐像素不变）；
// 「图单独成段」也不走这里（老路径整块画，行为与像素都不动）。

package dsconversation

import (
	"strconv"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// markdownInlineFont 是调用方的文字阶梯（段落 / 标题 / 列表项各不相同）。
type markdownInlineFont struct {
	// Size 是正文（或标题）字号。
	Size float32
	// Line 是行高（viva `LineHeight`，也是行盒的"文字那一份"高度）。
	Line float32
	// Weight 是字重（0 = 不设，与 textRun 的 Box.Weight 一致）。
	Weight float32
	// Bold 表示这一块整块加粗（官方 `**…**` 盖满整块）：量宽要用绘制时那一档字重，
	// 否则图与文字的相对位置是拿常规字重算出来的（见 `spansAllBold`）。
	Bold bool
	// Italic 表示这一块整块斜体（官方 `*…*` 盖满整块）：量宽必须走绘制时那一档
	// **斜体面**（viva `Canvas.TextWidthStyle` 在 `style.Italic` 时问
	// `gfx.StyledTextEngine` 的斜体面宽），否则图与文字的位置是拿直立字面算的。
	// 同 Bold 的理由，见 `spansAllItalic`。
	//
	// 删除线（`Strike`）**不在这里**：它只往文字上画一条线，viva 明说
	// "a Text keeps its own width and wrapping, so nothing about the layout moves
	// when a text is struck"（`base/box.go` 的 `Strike`），量宽与直立无差别。
	Italic bool
	// MonoSize 是行内代码的字号（`t.Fonts.Code`）；0 = 与 Size 相同。
	MonoSize float32
}

// markdownInlineTextStyle 是量宽用的字样：必须与 textRun 画出来的那一份**同源**。
//
// viva 的 `Wrap` 换行判据是 `TextWidthStyle(line, n.textStyle()) <= rect.Width - padX`
// （`viva@base/text_wrap.go` 的 `wrapText`），这里照抄同一个 style 组成：
// 字号 / 字重 / 是否等宽（face 缺省 0 = 系统等宽面，与 `.Mono()` 一致）。
//
// `MonoSize == 0` 表示**调用方的 textRun 不换等宽字体**（标题就是这样：官方标题里的
// 行内代码也用标题字体），那行内代码就按比例字量宽 —— 量法必须跟画法一致，
// 否则图会被算到错的位置。
func markdownInlineTextStyle(font markdownInlineFont, mono bool) gfx.TextStyle {
	style := gfx.TextStyle{Size: font.Size, Weight: font.Weight}
	if font.Bold {
		// 量宽与绘制同源：加粗那一档的进宽比常规宽（`viva@internal/textmetrics`
		// 按字重分表），只改绘制不改量宽会让行内图的位置跟着漂。
		style.Weight = gfx.WeightBold
	}
	if font.Italic {
		// 量宽与绘制同源：斜体面的进宽与直立面不是同一套（`Canvas.TextWidthStyle`
		// 在 `style.Italic` 时问 `gfx.StyledTextEngine` 的斜体面宽），只改绘制不改
		// 量宽同样会让行内图的位置漂。
		//
		// 等宽面不用另算：viva 的 `TextWidthStyle` 里 `Mono` 排在 `Italic` 前面
		// （先走 `monoWidth` 的网格宽），这里只把标志带过去，量法与画法仍然一致。
		style.Italic = true
	}
	if mono && font.MonoSize > 0 {
		style.Size = font.MonoSize
		style.Mono = true
	}
	return style
}

// markdownInlineAscent 是一段文字在自己的行盒里"基线以上多高"。
//
// 就是 viva `paintText` 画每一行时用的那条基线：`bounds.Y = 行顶 + k×lineHeight`、
// 行盒高 = lineHeight，`LabelAligned` 在其中按 `LabelBaseline`（比例字）或
// `MonoBaseline`（等宽面量得到时，`viva@gfx/baseline.go:44-53` 的 `textRow`）
// 定基线。这里把同一套判据搬出来，才能"把文字盒往下推 A_max - A_i"。
func markdownInlineAscent(canvas *gfx.Canvas, font markdownInlineFont, text mdlite.MarkdownInlineText) float32 {
	style := markdownInlineTextStyle(font, text.Mono)
	if style.Mono {
		if row := canvas.MonoBaseline(style.Size, gfx.MonoFace(0), font.Line); row > 0 {
			return row
		}
	}
	return canvas.LabelBaseline(gfx.Rect{Height: font.Line}, gfx.TextStyle{Size: style.Size})
}

// markdownInlineImageAscent 是图在行盒里"基线以上多高"：H/2 - 半个 x-height。
//
// `vertical-align:middle` 的原文是"盒子竖直中点对齐 **父基线 + 半个 x-height**"
// （CSS 2.1 §10.8.1）——y 向下增长，所以中点落在基线**下方** 半个 x-height 处：
//
//	中线 = 基线 + x/2   ⇒   上沿 = 基线 + x/2 - H/2   ⇒   A = H/2 - x/2
//
// 反过来那一半（H/2 + x/2）在基线以下，两者相加正好是 H —— 所以「图单独成段」时
// 行盒高度就是图的显示高，与老的块级画法逐像素一致。
func markdownInlineImageAscent(font markdownInlineFont, image *mdlite.MarkdownInlineImage) float32 {
	return image.Height/2 - MarkdownInlineImageXHeight*font.Size
}

// drawMarkdownInlineLayout 按官方行内流画一段「文字 + 图 + 文字」的正文。
//
// textRun 由调用方给：它建一个**单行**文字盒（样式随段落/标题/列表项），
// 这里负责按算好的宽度与基线偏移摆它。
func (v *OfficialTranscriptView) drawMarkdownInlineLayout(f ui.C, env *pluginapi.Env,
	slots []mdlite.MarkdownInlineSlot, key string,
	font markdownInlineFont, textRun func(string, bool) ui.Box) {
	canvas := env.Canvas
	column := v.markdownColumnWidth(env)
	measure := func(text string, mono bool) float32 {
		return canvas.TextWidthStyle(text, markdownInlineTextStyle(font, mono))
	}
	lines := mdlite.LayoutMarkdownInline(slots, column, measure)
	if len(lines) == 0 {
		return
	}
	t := v.Theme
	if t == nil {
		t = env.Theme
	}
	images := 0
	columnBox := ui.Column(f).FillWidth()
	columnBox.Children(func() {
		for _, line := range lines {
			if len(line.Items) == 0 {
				// 硬换行留下的空行：一个行高的空盒（与 `x\n\ny` 里那个空行同位）。
				ui.Row(f).FillWidth().Height(font.Line)
				continue
			}
			// CSS 2.1 §10.8：行盒高 = max(A) + max(D)，基线在行顶下 max(A)。
			type placedItem struct {
				item    mdlite.MarkdownInlineItem
				ascent  float32
				descent float32
			}
			placed := make([]placedItem, 0, len(line.Items))
			var maxAscent, maxDescent float32
			for _, item := range line.Items {
				var ascent, descent float32
				if item.Image != nil {
					ascent = markdownInlineImageAscent(font, item.Image)
					descent = item.Image.Height - ascent
				} else {
					ascent = markdownInlineAscent(canvas, font, item.Text)
					descent = font.Line - ascent
				}
				if ascent > maxAscent {
					maxAscent = ascent
				}
				if descent > maxDescent {
					maxDescent = descent
				}
				placed = append(placed, placedItem{item: item, ascent: ascent, descent: descent})
			}
			row := ui.Row(f).FillWidth().Height(maxAscent + maxDescent)
			row.Children(func() {
				for _, p := range placed {
					// 把元素自己的基线放到这一行的基线上：往下推 max(A) - A_i。
					offset := maxAscent - p.ascent
					if p.item.Image != nil {
						// 图：外面套一层只带内边距的竖列，避免 `Size` 被内边距挤掉
						// （viva 的 Rect 是 border-box，直接在图盒上加 padding 会缩图）。
						cell := ui.Column(f).Width(p.item.Width).PadSides(offset, 0, 0, 0)
						cell.Children(func() {
							v.drawMarkdownImageBox(f, env, p.item.Image, key+":img"+strconv.Itoa(images))
						})
						images++
						continue
					}
					if p.item.Text.Alt {
						// 官方 `.imageAlt`：行内 alt 文案（tertiary 色，不是正文色）。
						ui.Wrap(f, p.item.Text.Text).FontSize(font.Size).LineHeight(font.Line).
							Color(t.LabelTertiary).Width(p.item.Width).PadSides(offset, 0, 0, 0)
						continue
					}
					textRun(p.item.Text.Text, p.item.Text.Mono).Width(p.item.Width).PadSides(offset, 0, 0, 0)
				}
			})
		}
	})
}

// markdownInlineSlots 把 `SplitMarkdownInline` 的切分结果翻成 mdlite 的 slots。
//
// 图片段按官方的四态走：**只有 Ready 才是原子盒**（有显示尺寸），
// Pending / Failed / Unresolved 三档官方画的是 alt 文案，那是**行内文字**。
// 这也解释了为什么"字节还没回来"时图不会把段落撑开：那一档根本没有盒。
func (v *OfficialTranscriptView) markdownInlineSlots(env *pluginapi.Env, runs []mdlite.MarkdownInlineRun,
	allowPath bool) ([]mdlite.MarkdownInlineSlot, bool) {
	column := v.markdownColumnWidth(env)
	slots := make([]mdlite.MarkdownInlineSlot, 0, len(runs))
	mixed := false
	for i := range runs {
		run := runs[i]
		if run.Image == "" {
			slots = append(slots, mdlite.MarkdownInlineSlot{Text: run.Text, Mono: run.Mono})
			if strings.TrimSpace(run.Text) != "" {
				mixed = true
			}
			continue
		}
		image := v.markdownImages.Lookup(sessionIDOf(env), cwdOf(env), run.Image, allowPath)
		if image.State == mdlite.MarkdownImageReady {
			width, height := mdlite.FitMarkdownImage(image.Bitmap, column)
			if width > 0 && height > 0 {
				slots = append(slots, mdlite.MarkdownInlineSlot{Image: &mdlite.MarkdownInlineImage{
					Destination: run.Image, Alt: run.Text, Width: width, Height: height, Bitmap: image.Bitmap,
				}})
				continue
			}
		}
		// 非 Ready（或解码出 0×0）：官方画 alt 文案 —— 行内文字，一个字都不许少。
		text := mdlite.MarkdownImageAltText(run.Text, run.Image)
		if image.State == mdlite.MarkdownImageFailed {
			text = mdlite.MarkdownImageFailedText(run.Text, run.Image)
		}
		slots = append(slots, mdlite.MarkdownInlineSlot{Text: text, Alt: true})
	}
	return slots, mixed
}
