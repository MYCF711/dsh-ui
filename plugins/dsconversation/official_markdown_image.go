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
// # 本仓怎么画
//
// 本仓主区的正文是自绘的块级渲染器（`OfficialTranscriptView.drawMarkdown`），
// 旧实现把一段 spans 拍平成一行文字，图片只能退成 alt。这里换成"先切段再各画各的"：
//
//	mdlite.SplitMarkdownInline 把 spans 按原文顺序切成「文字段 / 图片段」
//	文字段 仍走原来的 Wrap（**没有图片时逐像素与旧路径完全一致**）
//	图片段 走 drawMarkdownImage：三态 + 官方几何
//
// 已知差异（如实登记，不假装没有）：官方图片是**行内**元素，一段文字中间夹一张图
// 会与文字同排；本仓切成块级后图会独占一行（附图两侧的文字各成一段）。这与
// 官方在"图片单独成段"这个主流写法下**完全一致**，只是"图夹在句中"这一种少见写法
// 会多一次换行。要做成严格行内，需要给正文换一套带内联原子排版的渲染器
// （viva 的 `MarkdownView` 就是这个形状），那是整条正文渲染路径的替换，
// 不属于"补一条图片"的范围。

package dsconversation

import (
	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
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
// textRun 由调用方给（段落 / 标题 / 列表项的正文样式各不相同）。
// 没有图片时它只会被调一次、且文本与旧的 `spansText` 逐字节相同——
// 这条是既有正文渲染的唯一路径，不许因为"顺手支持图片"改动它的任何一个像素。
func (v *OfficialTranscriptView) drawMarkdownInline(f ui.C, env *pluginapi.Env, spans []markdown.Span,
	allowPath bool, textRun func(string, bool)) {
	runs := mdlite.SplitMarkdownInline(spans)
	for _, run := range runs {
		if run.Image == "" {
			textRun(run.Text, run.Mono)
			continue
		}
		v.drawMarkdownImage(f, env, run.Image, run.Text, allowPath)
	}
}

// drawMarkdownImage 按官方三态画一张正文图。
//
// allowPath=false 是**流式期间**：官方 `aK` 在 `streaming` 时根本不调
// `fileImages.resolve`（只有绝对 http(s) 直接出），所以路径图先显示 alt、
// 定稿后才出图。照抄它，避免流式期间为一个半成品路径起一堆取数请求。
func (v *OfficialTranscriptView) drawMarkdownImage(f ui.C, env *pluginapi.Env, destination, alt string, allowPath bool) {
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
		// `.image`：radius = --dsw-radius-md、底 = --dsw-alias-bg-base、等比 contain。
		// 尺寸已经在 FitMarkdownImage 里按两条 max-* 算好（ui.Image 的缩放是
		// object-fit:fill，直接交给它会把极端比例的图拉变形）。
		row := ui.Row(f).Size(width, height).Radius(t.Radii.MD).Background(t.BgBase).Clip()
		row.Children(func() {
			ui.Image(f, image.Bitmap).Size(width, height)
		})
	case mdlite.MarkdownImageFailed:
		drawMarkdownImageText(f, mdlite.MarkdownImageFailedText(alt, destination), t)
	default: // Pending / Unresolved：官方这两档都只是 alt 文本
		drawMarkdownImageText(f, mdlite.MarkdownImageAltText(alt, destination), t)
	}
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
