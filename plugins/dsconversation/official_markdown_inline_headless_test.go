//go:build darwin || windows || linux

package dsconversation

// official_markdown_inline_headless_test.go —— 行内混排的**真窗口像素**回归（`#18` (b) ①）。
//
// 与 `official_markdown_inline_test.go` 的分工：那边是纯画布（图元矩形与基线关系），
// 这边开一个真 headless 窗口跑完整绘制链（Page → transcript → 滚动座位 → 合成），
// 用**像素**回答旧账那句「图夹在句中会多一次换行」：
//
//	① 图的**行带**里，图的左边有墨（图前那句文字）、右边有墨（图后那句文字）
//	   —— 三者同一行是像素事实，不是图元账上的巧合；
//	② 图的下沿之后那一条**一个墨点都没有**（多出来的那一次换行会让图后文字落在这里）；
//	③ 反向：图单独成段的夹具（`看图\n\n![…](…)\n\n结尾`）在图带里**右侧无墨**，
//	   图下沿之后有墨 —— 那一种写法的像素没有变。
//
// 判据用"与底色不同的像素"数，不依赖字体渲染细节：夹具底色是主题 `BgBase`
// （`Paint` 里先整窗填底），正文与图都不是这个色。

import (
	"image"
	"image/color"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// markdownInlineInk 数一块矩形里"不是底色"的像素。
func markdownInlineInk(picture *image.RGBA, box image.Rectangle, background color.RGBA) int {
	box = box.Intersect(picture.Bounds())
	count := 0
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			if picture.RGBAAt(x, y) != background {
				count++
			}
		}
	}
	return count
}

// gfxColorToRGBA 把主题色换成截图里的 RGBA（主题色是 0..1 的 float，按 8 位取整）。
func gfxColorToRGBA(c gfx.Color) color.RGBA {
	return color.RGBA{
		R: uint8(c.Red*255 + 0.5), G: uint8(c.Green*255 + 0.5),
		B: uint8(c.Blue*255 + 0.5), A: uint8(c.Alpha*255 + 0.5),
	}
}

// openMarkdownInlineHeadless 开一个真窗口，渲染一条助手消息，等图就绪后再画一帧。
//
// 返回：这一帧的位图矩形、截图、以及整窗的包（供逐区域数像素）。
func openMarkdownInlineHeadless(t *testing.T, text string) (*image.RGBA, image.Rectangle, *headless.Session, color.RGBA) {
	t.Helper()
	const width, height = 900, 600
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	body := solidColorPNG(t, 300, 150, messageImageFixtureColor)
	release := make(chan struct{})
	markdownImage := func(_, _, destination string) (pluginapi.MarkdownImageBytes, bool) {
		if destination != "./shot.png" {
			return pluginapi.MarkdownImageBytes{}, false
		}
		<-release
		return pluginapi.MarkdownImageBytes{Bytes: body, MIME: "image/png"}, true
	}

	tree := &gfx.Tree{}
	feat := tree.Feature("markdown-inline-headless")
	var invalidator app.Invalidator
	var mu sync.Mutex
	var drawn []gfx.Image

	entry := TranscriptEntry{
		Key: "a-md-inline", Kind: ChatNodeAssistantStep, Seq: 1, Turn: 1, Visibility: ChatVisibilityVisible,
		Data: TranscriptData{Text: text},
	}
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{
		Title: "markdown-inline-headless", Width: width, Height: height,
		Tree: tree, WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, current ui.Size) {
		bounds := gfx.Rect{Width: current.Width, Height: current.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds, MainWidth: bounds.Width,
			Data: pluginapi.Snapshot{Cur: markdownImageTestSession, CWD: markdownImageTestCWD},
			Act: pluginapi.Actions{
				MarkdownImage: markdownImage,
				Invalidate:    func() { invalidator.Invalidate() },
			},
		}
		page.Draw(canvas, bounds, func(f ui.C) {
			view.Draw(f, env, TranscriptViewport{HostScroll: true}, []TranscriptEntry{entry})
		})
		view.PaintCodeBlocks(canvas, env)
		mu.Lock()
		drawn = append([]gfx.Image(nil), canvas.Images()...)
		mu.Unlock()
	}

	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)

	pump := func() {
		t.Helper()
		invalidator.Invalidate()
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	pump()
	pump()
	close(release)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if view.markdownImages.Lookup(markdownImageTestSession, markdownImageTestCWD, "./shot.png", true).State == mdlite.MarkdownImageReady {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("等正文图片解码超时，状态 = %v",
				view.markdownImages.Lookup(markdownImageTestSession, markdownImageTestCWD, "./shot.png", true))
		}
		time.Sleep(5 * time.Millisecond)
	}
	pump()
	pump()

	mu.Lock()
	images := append([]gfx.Image(nil), drawn...)
	mu.Unlock()
	if len(images) != 1 {
		t.Fatalf("屏上图元 %d 张，期望 1 张", len(images))
	}
	box := image.Rect(int(images[0].Bounds.X), int(images[0].Bounds.Y),
		int(images[0].Bounds.X+images[0].Bounds.Width), int(images[0].Bounds.Y+images[0].Bounds.Height))
	t.Logf("IMAGE_OP Bounds={%.2f,%.2f,%.2f,%.2f}",
		images[0].Bounds.X, images[0].Bounds.Y, images[0].Bounds.Width, images[0].Bounds.Height)

	picture, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	return picture, box, session, gfxColorToRGBA(theme.BgBase)
}

// TestMarkdownInlinePixelsShareTheImageRow 是「多一次换行」那条旧账的像素判据。
func TestMarkdownInlinePixelsShareTheImageRow(t *testing.T) {
	const lineHeight = 22 // 正文行高（`Fonts.BodyLine`）
	picture, box, session, background := openMarkdownInlineHeadless(t, "看图 ![示意图](./shot.png) 结尾")
	if !session.HasText("看图 ") || !session.HasText(" 结尾") {
		t.Fatalf("两段正文都得在屏上；实际 %q", session.Texts())
	}
	ink := func(r image.Rectangle) int { return markdownInlineInk(picture, r, background) }

	// ① 图的行带里，图左边有墨（图前文字）、图右边有墨（图后文字）。
	left := ink(image.Rect(0, box.Min.Y, box.Min.X, box.Max.Y))
	right := ink(image.Rect(box.Max.X, box.Min.Y, picture.Bounds().Max.X, box.Max.Y))
	t.Logf("BAND ink left=%d right=%d（图带 %v）", left, right, box)
	if left == 0 {
		t.Fatalf("图左边的行带里没有墨：图前那句文字没跟图同排（图带 %v）", box)
	}
	if right == 0 {
		t.Fatalf("图右边的行带里没有墨：图后那句文字没跟图同排（旧账的「多一次换行」回来了；图带 %v）", box)
	}

	// ② 图下沿之后那一条：多出来的换行会让图后文字落在这里。
	below := ink(image.Rect(0, box.Max.Y, picture.Bounds().Max.X, box.Max.Y+lineHeight))
	t.Logf("BELOW ink=%d（图下沿 %d 起 %d 行）", below, box.Max.Y, lineHeight)
	if below != 0 {
		t.Fatalf("图下沿之后还有 %d 个墨点：图后文字被挤到了下一行（多一次换行）", below)
	}
}

// TestMarkdownImageAlonePixelsKeepTheBlockLayout 是**反向**像素判据：图单独成段那一档
// 的像素与旧行为一致（图带里右侧无墨、图下沿之后有墨）。
func TestMarkdownImageAlonePixelsKeepTheBlockLayout(t *testing.T) {
	const lineHeight = 22
	picture, box, _, background := openMarkdownInlineHeadless(t, "看图\n\n![示意图](./shot.png)\n\n结尾")
	ink := func(r image.Rectangle) int { return markdownInlineInk(picture, r, background) }
	right := ink(image.Rect(box.Max.X, box.Min.Y, picture.Bounds().Max.X, box.Max.Y))
	below := ink(image.Rect(0, box.Max.Y, picture.Bounds().Max.X, box.Max.Y+lineHeight))
	t.Logf("BLOCK band right=%d below=%d（图带 %v）", right, below, box)
	if right != 0 {
		t.Fatalf("图单独成段时图带右侧却有 %d 个墨点：这一种写法的像素被行内混排改了", right)
	}
	if below == 0 {
		t.Fatalf("图单独成段时图下沿之后没有墨：图后那段文字不见了（图带 %v）", box)
	}
}
