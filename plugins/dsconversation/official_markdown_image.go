// official_markdown_image.go —— markdown **正文**里的图片（`#18` (b)）。
//
// # 官方怎么画（rc.5，坐标都在 `OFF:` 前缀的 rc.5 前端包里）
//
//	OFF:dist/assets/index-Ciz7DZxv.js
//	  case"image":return ik(e.url,e.alt??"",t,r)   // mdast 的 image 节点
//	  function aK(...)                              // 目的地不可解析 → 只画 alt 文本
//	  function lK(...)                              // <img loading=lazy decoding=async>；onError → alt 文本
//	OFF:dist/assets/index-CSAdOBx6.css
//	  ._image_13jw0_300   display:block; width:auto; max-width:100%; height:auto;
//	                     border-radius:var(--dsw-radius-md);
//	                     background:var(--dsw-alias-bg-base); object-fit:contain
//	  ._imageButton_13jw0_471 ._image_13jw0_300    max-width:min(100%,640px); max-height:360px
//	  ._imageAlt_13jw0_300                         color:var(--dsw-alias-label-tertiary); font-style:italic
//	OFF:dsh-client-ui-chat/lib/client.js（zh 词典）  image.failed = 图片无法预览
//
// 三档表现（**都没有额外 chrome**）：
//
//	字节没回来/不可解析   alt 文本（官方 `<img>` 没出来时浏览器显示的就是 alt）
//	失败                  `图片无法预览 · alt||目的地`
//	成功                  `<img>`，几何按上面两条 CSS
//
// # 官方为什么是"行内"
//
// `._image_13jw0_300{display:block}` 说的是 `<button>` **里面**那张图；外面那层
// `._imageButton_13jw0_471{display:inline-block;vertical-align:middle}` 才是它
// 在段落里的身份 —— 一个 inline-level 原子盒，与前后文字同排。chat 里这层**恒有**
// （`fileImages` 由 `MarkdownDelegateProvider` 注入），所以官方正文的图是行内的。
//
// 尺寸仍是那两条 max-*：`min(100%, 640px)` × `360px`，从不放大
// （`mdlite.FitMarkdownImage`）。
//
// # 本仓怎么画
//
// 本仓主区的正文是自绘渲染器（`OfficialTranscriptView.drawMarkdown`），
// 旧实现把一段 spans 拍平成一行文字，图片只能退成 alt。这里换成"先切段再各画各的"：
//
//	mdlite.SplitMarkdownInline   把 spans 按原文顺序切成「文字段 / 图片段」
//	mdlite.LayoutMarkdownInline  行内混排的行切分与宽度（纯几何，`#18` (b) ①）
//
// 三条路，判据在 `drawMarkdownInline`：
//
//	没有图片    老路径：一次 `Wrap`（**逐像素与旧实现完全一致**）
//	图单独成段  老路径：图片段整块画（**行盒里只有图时 A+D 正好是 H**，官方这一种
//	            写法的正确结果，本轮一个像素都不动）
//	图夹在句中  行内排版：图与前后文字同一行，垂直按 `vertical-align:middle` 对齐
//	            （`official_markdown_inline.go`）
//
// 三档态的**画法**没变（alt 文本 / `图片无法预览 · alt||目的地` / 位图），
// 只是"图没出来"那两档在混排里也是**行内文字**，不再把段落拆成块。

package dsconversation

import (
	"strconv"

	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

// markdownSpansHaveImage 报告这段 spans 里有没有图片。
func markdownSpansHaveImage(spans []markdown.Span) bool {
	for _, span := range spans {
		if span.Image != "" {
			return true
		}
	}
	return false
}

// markdownSpansPaintable 报告这段 spans 有没有**任何可画的东西**（文字或图）。
//
// 为什么不能再用"拍平后的文本非空"当判据：`![ref](./a.png)` 这种写法 alt 是空的，
// 拍平后文本是空串——旧判据会把整段丢掉（现在那是一张必须画出来的图）。
func markdownSpansPaintable(spans []markdown.Span) bool {
	for _, span := range spans {
		if span.Image != "" || span.Text != "" {
			return true
		}
	}
	return false
}

// markdownImageLoader 把壳动作包成 mdlite 的取数入口。
//
// nil = 壳没接这条能力（老壳 / 无头 / 测试）→ 图片退成 alt 文本
// （官方 `src === undefined` 那一档），不画假图也不假装失败。
func markdownImageLoader(env *pluginapi.Env) mdlite.MarkdownImageLoader {
	if env == nil || env.Act.MarkdownImage == nil {
		return nil
	}
	return func(sessionID, cwd, destination string) ([]byte, bool) {
		blob, ok := env.Act.MarkdownImage(sessionID, cwd, destination)
		if !ok || len(blob.Bytes) == 0 {
			// 0 字节按失败处理：宿主不会把"本来就没有内容"当成一张图（同 attachment 链）。
			return nil, false
		}
		return blob.Bytes, true
	}
}

// markdownImageRedraw 取壳的重画入口：图片取回来之后要立刻上屏。
func markdownImageRedraw(env *pluginapi.Env) func() {
	if env == nil || env.Act.Invalidate == nil {
		return nil
	}
	return env.Act.Invalidate
}

// drawMarkdownInline 按官方顺序画一段行内内容：文字段照旧、图片段出图。
//
// textRun 由调用方给（段落 / 标题 / 列表项的正文样式各不相同），它建**一条**文字盒
// 并返回；行内混排（图夹在句中）时由 `drawMarkdownInlineLayout` 决定宽度与基线偏移。
// 没有图片时它只会被调一次、且文本与旧的 `spansText` 逐字节相同——
// 这条是既有正文渲染的唯一路径，不许因为"顺手支持图片"改动它的任何一个像素。
//
// 三条路（`#18` (b) ①）：
//
//	没有图片     老路径：一次 textRun（逐像素不变）
//	图单独成段   老路径：图片段整块画（`drawMarkdownInlineBlock`，逐像素、行为都不变）
//	图夹在句中   `drawMarkdownInlineLayout`：官方行内流（图与前后文字同一行）
//
// key 是这一块的稳定渲染键（`drawMarkdown` 的 entry key + 块序号）：正文图那张
// `<button>` 的交互 id 由它派生（官方每次重渲都拿同一个 `<button>`，id 变了
// 焦点/悬停会在两帧之间丢）。
func (v *OfficialTranscriptView) drawMarkdownInline(f ui.C, env *pluginapi.Env, spans []markdown.Span,
	allowPath bool, key string, font markdownInlineFont, textRun func(string, bool) ui.Box) {
	runs := mdlite.SplitMarkdownInline(spans)
	if !markdownSpansHaveImage(spans) {
		if len(runs) > 0 {
			textRun(runs[0].Text, runs[0].Mono)
		}
		return
	}
	slots, mixed := v.markdownInlineSlots(env, runs, allowPath)
	// 混排要按宽度量字：没有画布（老壳/离线工具）时退回块级，宁可少一次行内，
	// 也不拿估算宽度画正文。
	if !mixed || env == nil || env.Canvas == nil {
		v.drawMarkdownInlineBlock(f, env, runs, allowPath, key, textRun)
		return
	}
	v.drawMarkdownInlineLayout(f, env, slots, key, font, textRun)
}

// drawMarkdownInlineBlock 是"图单独成段"的块级路径（本轮**行为与像素都不动**）。
//
// 图独占一行、两侧文字各成一段：官方在"图片单独成段"这个主流写法下就是这个形状
// （行盒里只有那张图时，图的 A+D 正好是 H），所以这条路径不是"凑合"，而是官方
// 那一种写法的正确结果。
func (v *OfficialTranscriptView) drawMarkdownInlineBlock(f ui.C, env *pluginapi.Env, runs []mdlite.MarkdownInlineRun,
	allowPath bool, key string, textRun func(string, bool) ui.Box) {
	images := 0
	for i := range runs {
		run := runs[i]
		if run.Image == "" {
			textRun(run.Text, run.Mono)
			continue
		}
		// 同一块里的第几张图：一张段落里可以有多张 `![…](…)`，各自是不同的按钮。
		imageKey := key + ":img" + strconv.Itoa(images)
		images++
		v.drawMarkdownImage(f, env, run.Image, run.Text, allowPath, imageKey)
	}
}

// drawMarkdownImage 按官方三态画一张正文图。
//
// allowPath=false 是**流式期间**：官方 `aK` 在 `streaming` 时根本不调
// `fileImages.resolve`（只有绝对 http(s) 直接出），所以路径图先显示 alt、
// 定稿后才出图。照抄它，避免流式期间为一个半成品路径起一堆取数请求。
//
// key 是这张图的稳定渲染键（见 drawMarkdownInline）。
//
// # 「查看大图」（官方 `_imageButton_13jw0_471`）
//
// 官方把出好的图包在一枚 `<button cursor:zoom-in title=查看大图>` 里，
// 点它才挂灯箱（`lK` 的 `onClick={() => setOpen(true)}`）。本仓在同一处加命中区：
// **只有 Ready 态才有按钮**——字节没到/失败时官方连 `<img>` 都没有，那时正文
// 画的是 alt 文本，不存在"点开一个空灯箱"的路径。
//
// 灯箱本体在 `official_image_lightbox.go`（`shell.overlay` 上的另一块画）。
func (v *OfficialTranscriptView) drawMarkdownImage(f ui.C, env *pluginapi.Env, destination, alt string,
	allowPath bool, key string) {
	t := v.Theme
	if t == nil && env != nil {
		t = env.Theme
	}
	image := v.markdownImages.Lookup(sessionIDOf(env), cwdOf(env), destination, allowPath)
	switch image.State {
	case mdlite.MarkdownImageReady:
		width, height := mdlite.FitMarkdownImage(image.Bitmap, v.markdownColumnWidth(env))
		if width <= 0 || height <= 0 {
			// 位图解码成功却是 0×0：按失败态画，不画一个空白块。
			drawMarkdownImageText(f, mdlite.MarkdownImageFailedText(alt, destination), t)
			return
		}
		v.drawMarkdownImageBox(f, env, &mdlite.MarkdownInlineImage{
			Destination: destination, Alt: alt, Width: width, Height: height, Bitmap: image.Bitmap,
		}, key)
	case mdlite.MarkdownImageFailed:
		drawMarkdownImageText(f, mdlite.MarkdownImageFailedText(alt, destination), t)
	default: // Pending / Unresolved：官方这两档都只是 alt 文本
		drawMarkdownImageText(f, mdlite.MarkdownImageAltText(alt, destination), t)
	}
}

// drawMarkdownImageBox 画一张 Ready 态的正文图（官方 `.image` 盒 + `_imageButton` 的
// 「查看大图」命中区），返回它的盒子。
//
// 块级路径（图单独成段）与行内路径（图夹在句中）**共用这一份**：盒子的几何、
// 圆角、底色、命中区与灯箱入口只有一处实现，两种路径不会各画一套。
func (v *OfficialTranscriptView) drawMarkdownImageBox(f ui.C, env *pluginapi.Env,
	image *mdlite.MarkdownInlineImage, key string) ui.Box {
	t := v.Theme
	if t == nil && env != nil {
		t = env.Theme
	}
	if image == nil || image.Width <= 0 || image.Height <= 0 {
		return ui.Box{}
	}
	// `.image`：radius = --dsw-radius-md、底 = --dsw-alias-bg-base、等比 contain。
	// 尺寸已经按两条 max-* 算好（ui.Image 的缩放是 object-fit:fill，
	// 直接交给它会把极端比例的图拉变形）。
	row := ui.Row(f).Size(image.Width, image.Height).Radius(t.Radii.MD).Background(t.BgBase).Clip()
	// 官方的 `cursor:zoom-in`（viva 的 PointerCursor 没有这一档，用可点手型，
	// 见 official_image_lightbox.go 的差异 ①）。
	if id := markdownImageHitID(env, key); !id.IsZero() {
		row = row.HitWith(id, markdownImageOpenLabel(image.Alt), gfx.HitOptions{
			Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand,
		})
	}
	row.Children(func() {
		ui.Image(f, image.Bitmap).Size(image.Width, image.Height)
	})
	if row.Clicked() {
		v.openMarkdownLightbox(env, image.Destination, image.Alt,
			mdlite.MarkdownImage{State: mdlite.MarkdownImageReady, Bitmap: image.Bitmap})
	}
	return row
}

// markdownImageHitID 造正文图那枚"查看大图"按钮的交互 id。
//
// 名字里带 block key + 图序号：同一个会话两帧之间必须是**同一个** id，
// 否则 viva 会把按钮当成新节点重挂（悬停/焦点每帧都丢）——这条与
// `historyButtonID` 同一条理由。零值 = 没有交互树（老壳/测试），那句话就只是画。
func markdownImageHitID(env *pluginapi.Env, key string) gfx.ID {
	if env == nil || env.Tree == nil {
		return gfx.ID{}
	}
	return env.Tree.ID(env.Feat, "transcript-image-"+key)
}

// markdownImageOpenLabel 是官方 `aria-label`：
// `alt ? `${t('image.open')}: ${alt}` : t('image.open')`（`lK` 的 aria-label）。
func markdownImageOpenLabel(alt string) string {
	if alt == "" {
		return ImageLightboxOpen
	}
	return ImageLightboxOpen + ": " + alt
}

// drawMarkdownImageText 画官方 `.imageAlt`：label-tertiary 的正文文本。
//
// 官方还有 `font-style:italic`，viva 的盒级文本没有斜体开关（`gfx.TextStyle.Italic`
// 只在画布层），所以斜体这一处是**已知差异**：色号一致、斜体没跟。登记在
// `docs/official-parity.md` #18 行。
func drawMarkdownImageText(f ui.C, text string, t *pluginapi.Theme) {
	if text == "" {
		return
	}
	ui.Wrap(f, text).FontSize(t.Fonts.Body).LineHeight(t.Fonts.BodyLine).Color(t.LabelTertiary)
}

// markdownColumnWidth 是正文列的可用宽度（图片按它做 100%）。
//
// 与 `codeMeasureWidth` 同一算式：主区宽 - 滚动口左右内边距。图片与代码块都以
// "这一列有多宽"为基准，各自再叠自己的上限，所以共用同一个估算值不是重复。
func (v *OfficialTranscriptView) markdownColumnWidth(env *pluginapi.Env) float32 {
	return v.codeMeasureWidth(env)
}

// sessionIDOf / cwdOf 从快照里取缓存键的两个坐标（缺一个就是另一张图）。
func sessionIDOf(env *pluginapi.Env) string {
	if env == nil {
		return ""
	}
	return env.Data.Cur
}

func cwdOf(env *pluginapi.Env) string {
	if env == nil {
		return ""
	}
	return env.Data.CWD
}
