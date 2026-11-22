//go:build darwin || windows || linux

package dsconversation

// official_markdown_image_headless_test.go —— markdown 正文图片的**真窗口像素**回归（`#18` (b)）。
//
// 与 `official_markdown_image_test.go` 的分工：那边是纯画布断言（图元账 / 文案 / 上限），
// 这边开一个真 headless 窗口跑完整绘制链（Page → transcript → 滚动座位 → 合成），
// 钉三件事：
//
//	① 字节还没回来时：屏上是官方 alt 文本，且**一个图片色像素都没有**
//	   （反向断言：证明下面"数出 43200 像素"不是判据太松）；
//	② 后台取数 + 解码回来、Invalidate 重画之后：屏上出现**恰好一张**位图，
//	   显示框是原始尺寸 300×150（没超官方 640×360 上限，所以不缩放）；
//	③ 那时 alt 文本不在屏上，而同一段正文的其它文字仍在（"该渲染的仍然渲染"）。
//
// 夹具是**纯色**（`messageImageFixtureColor`）：像素判据要"严格相等"，渐变图缩放后
// 几乎没有输出像素等于任何源像素色——那条测量假象的来龙去脉写在
// `official_message_image_headless_test.go` 的文件头，这里不重复。

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
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

// TestMarkdownImageLandsPixels 是正文图片的真窗口回归。
func TestMarkdownImageLandsPixels(t *testing.T) {
	const width, height = 900, 600
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})

	// 300×150 既没超 640 宽也没超 360 高 ⇒ 官方规则显示原尺寸（不缩放、不放大），
	// 这样"数严格相等的像素"才站得住。
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
	feat := tree.Feature("markdown-image-headless")
	var invalidator app.Invalidator
	var mu sync.Mutex
	var drawn []gfx.Image

	entry := TranscriptEntry{
		Key: "a-md-image", Kind: ChatNodeAssistantStep, Seq: 1, Turn: 1, Visibility: ChatVisibilityVisible,
		Data: TranscriptData{Text: "看图\n\n![示意图](./shot.png)"},
	}
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{
		Title: "markdown-image-headless", Width: width, Height: height,
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
	defer session.Close()

	pump := func() {
		t.Helper()
		invalidator.Invalidate()
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}

	// 帧 1：字节闸着 → 官方 alt 文本在屏上，没有图元，也没有图片色像素。
	pump()
	pump()
	if !session.HasText("示意图") {
		t.Fatalf("加载中应显示官方 alt 文本 %q，实际 %q", "示意图", session.Texts())
	}
	mu.Lock()
	before := len(drawn)
	mu.Unlock()
	if before != 0 {
		t.Fatalf("加载中不该有图元，实际 %d 张", before)
	}
	loadingPicture, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	if got := countFixturePixels(loadingPicture, loadingPicture.Bounds()); got != 0 {
		t.Fatalf("加载中已经有 %d 个图片色像素，期望 0（判据太松）", got)
	}

	// 放闸：等后台真的把字节取回并解码完（轮询缓存状态，不赌调度），再重画。
	close(release)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if view.markdownImages.Lookup(markdownImageTestSession, markdownImageTestCWD, "./shot.png", true).State == mdlite.MarkdownImageReady {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("等正文图片解码超时，状态 = %v",
				view.markdownImages.Lookup(markdownImageTestSession, markdownImageTestCWD, "./shot.png", true).State)
		}
		time.Sleep(5 * time.Millisecond)
	}
	pump()
	pump()

	mu.Lock()
	images := drawn
	mu.Unlock()
	if len(images) != 1 {
		t.Fatalf("屏上图元 %d 张，期望 1 张（真正文图片）", len(images))
	}
	img := images[0]
	if img.Bounds.Width != 300 || img.Bounds.Height != 150 {
		t.Fatalf("显示框 = %v×%v，期望 300×150（原尺寸）", img.Bounds.Width, img.Bounds.Height)
	}
	if img.Bitmap.Width != 300 || img.Bitmap.Height != 150 {
		t.Fatalf("源位图 = %d×%d，期望 300×150（真 PNG 解码结果）", img.Bitmap.Width, img.Bitmap.Height)
	}
	t.Logf("IMAGE_OP Bounds={%.2f,%.2f,%.2f,%.2f} ClipOf={%.2f,%.2f,%.2f,%.2f} HasClip=%v",
		img.Bounds.X, img.Bounds.Y, img.Bounds.Width, img.Bounds.Height,
		img.ClipOf.X, img.ClipOf.Y, img.ClipOf.Width, img.ClipOf.Height, img.HasClip)
	if img.HasClip {
		left := maxFloat(img.Bounds.X, img.ClipOf.X)
		top := maxFloat(img.Bounds.Y, img.ClipOf.Y)
		right := minFloat(img.Bounds.X+img.Bounds.Width, img.ClipOf.X+img.ClipOf.Width)
		bottom := minFloat(img.Bounds.Y+img.Bounds.Height, img.ClipOf.Y+img.ClipOf.Height)
		if right <= left || bottom <= top {
			t.Fatalf("clip 与 Bounds 不相交，合成器会丢掉这条 op：Bounds=%v ClipOf=%v", img.Bounds, img.ClipOf)
		}
	}
	if session.HasText("示意图") {
		t.Fatal("图出来之后 alt 文本不该还在屏上")
	}
	// 反向断言：同一段正文的其它文字必须照旧在屏上。
	if !session.HasText("看图") {
		t.Fatalf("正文首句被图片渲染吃掉了；屏上文本 %q", session.Texts())
	}

	// 像素级：图元自己的 Bounds 里必须**真的是这张图**。
	picture, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	box := image.Rect(int(img.Bounds.X), int(img.Bounds.Y),
		int(img.Bounds.X+img.Bounds.Width), int(img.Bounds.Y+img.Bounds.Height))
	landed := countFixturePixels(picture, box)
	total := box.Dx() * box.Dy()
	if landed < total*99/100 {
		t.Fatalf("图片落点 %v 里只有 %d/%d 像素是图片色，期望 ≥99%%（真图真的上屏）", box, landed, total)
	}
	t.Logf("IMAGE_PIXELS box=%v landed=%d/%d", box, landed, total)

	// 人工复核用截图（`WPC2_CAPTURE_DIR` 落盘；不设就落临时目录，不污染仓库）。
	captureMarkdownImage(t, session, width, height)
}

// captureMarkdownImage 把这一帧写成 PNG，供人工/截图对比复核。
func captureMarkdownImage(t *testing.T, session *headless.Session, width, height int) {
	t.Helper()
	dir := os.Getenv("WPC2_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("markdown-image-%dx%d.png", width, height))
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Fatalf("截图没落盘或为空：%v", err)
	}
	t.Logf("CAPTURE %s", path)
}
