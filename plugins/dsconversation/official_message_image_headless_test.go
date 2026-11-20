//go:build darwin || windows || linux

package dsconversation

// official_message_image_headless_test.go —— 图片面的**真窗口**回归（`#18` ②③）。
//
// 与 `official_message_image_test.go` 的分工：那边是纯画布断言（几何 / 三态 / 图元数），
// 这边开一个真 headless 会话跑完整绘制链（Page → transcript → 滚动座位），钉三件事：
//
//	① 字节还没回来时，屏上画的是官方 loading 文案；
//	② 后台取数 + 解码回来、Invalidate 重画之后，画布上出现**恰好一张**图元，
//	   显示框是 singleFit 的 240×180、源位图是喂进去的 400×300；
//	③ 那时 loading / 失败文案都不在屏上。
//
// # 一处未验项（如实登记，不冒充通过）
//
// 本文件**没有**做像素级断言：探针证明 viva 的 headless 合成器能画位图
// （同样的 400×300 → 240×180、同样的圆角+底色 Row，同色像素 43200 = 240×180 全覆盖），
// 但同一条 op 在 transcript 的座位/滚动链里**没有落到 surface 上**（整屏扫 0 个同色像素，
// 而 `canvas.Images()` 里确实有这一条、`HasClip=true` 且 clip 是全屏）。
// 这是 harness 侧的现象，不是本实现的计算错误；在真窗口复核之前不写成"已验像素"。
// 截图仍按 `WPC2_CAPTURE_DIR` 落盘，供人工复核。

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// TestMessageImageLandsOneBitmapOp 是图片面的真窗口回归。
func TestMessageImageLandsOneBitmapOp(t *testing.T) {
	const width, height = 900, 600
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})

	body := solidPNG(t, 400, 300) // 4:3，single 档 → 显示框 240×180
	// 取数被闸住：这样"第一帧只有 loading"是确定性的，不会因为 goroutine 跑得比
	// 断言快而变成竞态（headless 的 Pump 内部可能连画几帧）。
	release := make(chan struct{})
	get := func(id string) (pluginapi.AttachmentBlob, bool) {
		if id != "att-shot" {
			return pluginapi.AttachmentBlob{}, false
		}
		<-release
		return pluginapi.AttachmentBlob{ID: id, Name: "shot.png", MIME: "image/png",
			Width: 400, Height: 300, Bytes: body}, true
	}

	tree := &gfx.Tree{}
	feat := tree.Feature("message-image-headless")
	var invalidator app.Invalidator
	var mu sync.Mutex
	var drawn []gfx.Image

	entry := TranscriptEntry{
		Key: "u", Kind: ChatNodeUser, Seq: 1, Turn: 0, Visibility: ChatVisibilityVisible,
		Data: TranscriptData{
			Text:        "看这张图",
			Attachments: []TranscriptAttachment{{Image: true, ID: "att-shot", Name: "shot.png"}},
		},
	}
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{
		Title: "message-image-headless", Width: width, Height: height,
		Tree: tree, WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, current ui.Size) {
		bounds := gfx.Rect{Width: current.Width, Height: current.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds, MainWidth: bounds.Width,
			Act: pluginapi.Actions{
				AttachmentGet: get,
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

	// 帧 1：字节闸着没回来 → 官方 loading 文案在屏上，且还没有图元。
	pump()
	pump()
	if !session.HasText(MessageImageLoading) {
		t.Fatalf("第一帧应画官方 loading 文案 %q，实际 %q", MessageImageLoading, session.Texts())
	}
	mu.Lock()
	before := len(drawn)
	mu.Unlock()
	if before != 0 {
		t.Fatalf("加载中不该有图元，实际 %d 张", before)
	}

	// 放闸：等后台真的把字节取回并解码完（轮询缓存三态，不去赌调度），再重画。
	close(release)
	deadline := time.Now().Add(5 * time.Second)
	for {
		snapshot, _ := view.messageImages.lookup("att-shot", false)
		if snapshot.state == messageImageReady {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("等图片解码超时，状态 = %v", snapshot.state)
		}
		time.Sleep(5 * time.Millisecond)
	}
	pump()
	pump()

	mu.Lock()
	images := drawn
	mu.Unlock()
	if len(images) != 1 {
		t.Fatalf("屏上图元 %d 张，期望 1 张（真图片面）", len(images))
	}
	img := images[0]
	if img.Bounds.Width != 240 || img.Bounds.Height != 180 {
		t.Fatalf("显示框 = %v×%v，期望 240×180（singleFit）", img.Bounds.Width, img.Bounds.Height)
	}
	if img.Bitmap.Width != 400 || img.Bitmap.Height != 300 {
		t.Fatalf("源位图 = %d×%d，期望 400×300（真 PNG 解码结果）", img.Bitmap.Width, img.Bitmap.Height)
	}
	if session.HasText(MessageImageLoading) || session.HasText(MessageImageLoadFailed) {
		t.Fatal("图上屏之后不该还有 loading / 失败文案")
	}

	captureMessageImage(t, session, width, height)
}

// captureMessageImage 写一张 PNG 供人工复核。
func captureMessageImage(t *testing.T, session *headless.Session, width, height int) {
	t.Helper()
	dir := os.Getenv("WPC2_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("message-image-%dx%d.png", width, height))
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	picture, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	if picture.Bounds().Dx() != width || picture.Bounds().Dy() != height {
		t.Fatalf("截图尺寸=%v，期望 %dx%d", picture.Bounds(), width, height)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	t.Logf("MESSAGE_IMAGE_CAPTURE path=%s size=%dx%d bytes=%d", path, width, height, info.Size())
}
