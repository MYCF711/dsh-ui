package dsconversation

// official_markdown_image_test.go —— markdown **正文图片**（`#18` (b)）。
//
// # 这条守的是什么
//
// 官方正文里的 `![alt](目的地)` 会被画成一张 `<img>`（`OFF:dist/assets/index-Ciz7DZxv.js`
// 的 `case"image"`），本仓旧实现把整段 spans 拍平成文字，图片只能退成 alt。
// 这三个用例钉的就是"现在真的会出图，而且三档表现与官方一致"：
//
//	TestMarkdownImageLandsOnCanvas      取到字节 → 画面上出现**恰好一张**位图，
//	                                    显示尺寸按官方 640×360 上限；alt 文本消失，
//	                                    同一段正文的其它文字照旧（反向断言）
//	TestMarkdownImageFailureShowsOfficialText  取不到 → 官方失败文案（逐字），仍然没有位图
//	TestMarkdownImageNeverLoadsRefusedDestination  拒绝表里的 scheme 一次都不取
//	TestMarkdownImageStreamingSkipsPath  流式期间路径图不出（官方 streaming 分支）
//	TestMarkdownImageFitCaps            两条 max-* 上限（640 / 360）
//
// 像素级判据在 `official_markdown_image_headless_test.go`（真 headless 窗口 +
// 纯色夹具 + 严格相等像素计数），这边钉的是图元账与文案。

import (
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 渲染夹具的会话与 cwd（相对路径图要靠 cwd 落地）。
const (
	markdownImageTestSession = "sess-md-image"
	markdownImageTestCWD     = `D:\ws`
)

// markdownImageEntry 造一条正文带图片的助手节点。
func markdownImageEntry(text string, streaming bool) []TranscriptEntry {
	return []TranscriptEntry{{
		Key: "md-image", Kind: ChatNodeAssistantStep, Seq: 1, Turn: 1, Visibility: ChatVisibilityVisible,
		Streaming: streaming,
		Data:      TranscriptData{Text: text},
	}}
}

// renderMarkdownImages 在真实画布上跑一次 Draw，并注入正文图片的取数动作。
func renderMarkdownImages(t *testing.T, view *OfficialTranscriptView, entries []TranscriptEntry, act pluginapi.Actions) *gfx.Canvas {
	t.Helper()
	theme := view.Theme
	canvas, tree := newTranscriptCanvas()
	// 快照带会话与 cwd：相对路径要靠 cwd 才落地（官方 `resolveWorkspacePath`），
	// cwd 为空时官方也只画 alt——测试不该依赖那条边界。
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("markdown-image-test"),
		Canvas: canvas, Bounds: gfx.Rect{Width: 900, Height: 700}, MainWidth: 900, Act: act,
		Data: pluginapi.Snapshot{Cur: markdownImageTestSession, CWD: markdownImageTestCWD},
	}
	pageTheme := dscore.VivaTheme(theme)
	fv := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	fv.Draw(canvas, gfx.Rect{Width: 900, Height: 700}, func(f ui.C) {
		view.Draw(f, env, hostViewport(tree), entries)
	})
	// 覆画阶段（代码块）；正文图片是布局期图元，不在这里画。
	view.PaintCodeBlocks(canvas, env)
	return canvas
}

// waitMarkdownImage 等一张正文图进入某个终态（取数在后台 goroutine 里跑）。
func waitMarkdownImage(t *testing.T, view *OfficialTranscriptView, destination string, want mdlite.MarkdownImageState) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if view.markdownImages.Lookup(markdownImageTestSession, markdownImageTestCWD, destination, true).State == want {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("等 %q 进入状态 %v 超时（实际 %v）", destination, want,
		view.markdownImages.Lookup(markdownImageTestSession, markdownImageTestCWD, destination, true).State)
}

// TestMarkdownImageLandsOnCanvas 钉"真图真的画上去了"。
//
// 三件事一起钉：
//
//	① 字节闸着时：画面上只有 alt 文本（官方 `<img>` 没出来时浏览器显示 alt），零位图；
//	② 放闸之后：恰好一张位图，显示尺寸 = 原尺寸（300×150，未超 640/360 上限），
//	   源位图是真解码出来的 300×150；
//	③ 此时的 alt 文本消失，而同一段正文里的**其它文字照旧**——这是"不误判"的
//	   反向测试：不许为了让图出现把整段正文吃掉。
func TestMarkdownImageLandsOnCanvas(t *testing.T) {
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
	text := "看图\n\n![示意图](./shot.png)\n\n结尾"
	entries := markdownImageEntry(text, false)

	// 帧 1：字节闸着 → alt 文本在、位图不在。
	canvas := renderMarkdownImages(t, view, entries, act)
	if images := canvas.Images(); len(images) != 0 {
		t.Fatalf("取数还没回来就有 %d 张位图，期望 0 张", len(images))
	}
	if _, ok := canvasText(canvas, "示意图"); !ok {
		t.Fatalf("加载中应显示 alt 文本；实际文本：%v", canvasTexts(canvas))
	}

	// 放闸：等后台取数 + 解码落库。
	close(release)
	waitMarkdownImage(t, view, "./shot.png", mdlite.MarkdownImageReady)

	canvas = renderMarkdownImages(t, view, entries, act)
	images := canvas.Images()
	if len(images) != 1 {
		t.Fatalf("屏上位图 %d 张，期望 1 张；文本：%v", len(images), canvasTexts(canvas))
	}
	image := images[0]
	if image.Bounds.Width != 300 || image.Bounds.Height != 150 {
		t.Fatalf("显示框 = %v×%v，期望 300×150（原尺寸，未超 640×360 上限）",
			image.Bounds.Width, image.Bounds.Height)
	}
	if image.Bitmap.Width != 300 || image.Bitmap.Height != 150 {
		t.Fatalf("源位图 = %d×%d，期望 300×150（真 PNG 解码结果）",
			image.Bitmap.Width, image.Bitmap.Height)
	}
	if _, ok := canvasText(canvas, "示意图"); ok {
		t.Fatal("图出来之后 alt 文本不该还在（官方 `<img>` 显示了就不显示 alt）")
	}
	// 反向断言：同一段正文的其它文字必须照旧。
	for _, want := range []string{"看图", "结尾"} {
		if _, ok := canvasText(canvas, want); !ok {
			t.Fatalf("正文文字 %q 被图片渲染吃掉了；实际文本：%v", want, canvasTexts(canvas))
		}
	}
}

// TestMarkdownImageStreamingSkipsPath 钉流式期间不取路径（官方 `aK` 的 streaming 分支）。
func TestMarkdownImageStreamingSkipsPath(t *testing.T) {
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	body := solidColorPNG(t, 40, 20, messageImageFixtureColor)
	var mu sync.Mutex
	calls := 0
	act := pluginapi.Actions{
		MarkdownImage: func(string, string, string) (pluginapi.MarkdownImageBytes, bool) {
			mu.Lock()
			calls++
			mu.Unlock()
			return pluginapi.MarkdownImageBytes{Bytes: body, MIME: "image/png"}, true
		},
	}
	entries := markdownImageEntry("![示意](./shot.png)", true)
	canvas := renderMarkdownImages(t, view, entries, act)
	time.Sleep(5 * time.Millisecond)
	mu.Lock()
	got := calls
	mu.Unlock()
	if got != 0 {
		t.Fatalf("流式期间对路径图调了 %d 次取数，期望 0 次", got)
	}
	if images := canvas.Images(); len(images) != 0 {
		t.Fatalf("流式期间不该出位图，实际 %d 张", len(images))
	}
	if _, ok := canvasText(canvas, "示意"); !ok {
		t.Fatalf("流式期间应显示 alt 文本；实际文本：%v", canvasTexts(canvas))
	}
}

// TestMarkdownImageFitCaps 钉两条官方 max-* 上限（640 宽 / 360 高）。
//
// 夹具用 2000×200（宽超 640）与 200×2000（高超 360）两张，断言显示框就是
// 官方那条规则的输出——不是"原始尺寸"，也不是"列宽的某个百分比"。
func TestMarkdownImageFitCaps(t *testing.T) {
	theme := dscore.Dark()
	cases := []struct {
		name      string
		sourceW   int
		sourceH   int
		wantW     float32
		wantH     float32
		bodyImage string
	}{
		{name: "宽超 640", sourceW: 2000, sourceH: 200, wantW: 640, wantH: 64},
		{name: "高超 360", sourceW: 200, sourceH: 2000, wantW: 36, wantH: 360},
		{name: "都不超", sourceW: 320, sourceH: 200, wantW: 320, wantH: 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
			body := solidColorPNG(t, c.sourceW, c.sourceH, messageImageFixtureColor)
			act := pluginapi.Actions{
				MarkdownImage: func(string, string, string) (pluginapi.MarkdownImageBytes, bool) {
					return pluginapi.MarkdownImageBytes{Bytes: body, MIME: "image/png"}, true
				},
			}
			entries := markdownImageEntry("![图](./big.png)", false)
			_ = renderMarkdownImages(t, view, entries, act)
			waitMarkdownImage(t, view, "./big.png", mdlite.MarkdownImageReady)
			canvas := renderMarkdownImages(t, view, entries, act)
			images := canvas.Images()
			if len(images) != 1 {
				t.Fatalf("位图 %d 张，期望 1 张", len(images))
			}
			if images[0].Bounds.Width != c.wantW || images[0].Bounds.Height != c.wantH {
				t.Fatalf("显示框 = %v×%v，期望 %v×%v",
					images[0].Bounds.Width, images[0].Bounds.Height, c.wantW, c.wantH)
			}
		})
	}
}

// canvasTexts 收集画布上的全部文本（失败信息用）。
func canvasTexts(canvas *gfx.Canvas) []string {
	texts := canvas.Texts()
	values := make([]string, 0, len(texts))
	for _, text := range texts {
		values = append(values, text.Value)
	}
	return values
}

// TestMarkdownImageFailureShowsOfficialText 钉失败态文案（逐字官方）。
//
// 官方 `lK` 的 onError 分支：“ `${labels.failed} · ${alt||destination}` “，
// `image.failed` 的 zh 是「图片无法预览」。同时**不许**画位图占位。
func TestMarkdownImageFailureShowsOfficialText(t *testing.T) {
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	act := pluginapi.Actions{
		MarkdownImage: func(string, string, string) (pluginapi.MarkdownImageBytes, bool) {
			return pluginapi.MarkdownImageBytes{}, false
		},
	}
	entries := markdownImageEntry("![示意图](./missing.png)", false)
	_ = renderMarkdownImages(t, view, entries, act)
	waitMarkdownImage(t, view, "./missing.png", mdlite.MarkdownImageFailed)
	canvas := renderMarkdownImages(t, view, entries, act)
	if images := canvas.Images(); len(images) != 0 {
		t.Fatalf("失败态不该有位图，实际 %d 张", len(images))
	}
	want := "图片无法预览 · 示意图"
	if _, ok := canvasText(canvas, want); !ok {
		t.Fatalf("失败态文案不是官方那一句 %q；实际文本：%v", want, canvasTexts(canvas))
	}
}

// TestMarkdownImageNeverLoadsRefusedDestination 钉拒绝表：危险 scheme 一次都不取。
//
// 官方 `Gb` 把 javascript/vbscript/data/blob 退成空串，`qW` 只放 http(s)。
// 这条同时钉"没有请求就没有代价"：loader 调用次数必须是 0。
func TestMarkdownImageNeverLoadsRefusedDestination(t *testing.T) {
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	var mu sync.Mutex
	calls := 0
	act := pluginapi.Actions{
		MarkdownImage: func(string, string, string) (pluginapi.MarkdownImageBytes, bool) {
			mu.Lock()
			calls++
			mu.Unlock()
			return pluginapi.MarkdownImageBytes{}, false
		},
	}
	entries := markdownImageEntry("![坏图](data:image/png;base64,iVBORw0KGgo=)", false)
	canvas := renderMarkdownImages(t, view, entries, act)
	time.Sleep(5 * time.Millisecond)
	mu.Lock()
	got := calls
	mu.Unlock()
	if got != 0 {
		t.Fatalf("拒绝表里的目的地调了 %d 次取数，期望 0 次", got)
	}
	if images := canvas.Images(); len(images) != 0 {
		t.Fatalf("拒绝表里的目的地画出了 %d 张位图", len(images))
	}
	if _, ok := canvasText(canvas, "坏图"); !ok {
		t.Fatalf("取不到地址时应退成 alt 文本；实际文本：%v", canvasTexts(canvas))
	}
}
