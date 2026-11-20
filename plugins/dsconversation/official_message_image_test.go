package dsconversation

// official_message_image_test.go —— 官方图片面的机器断言（`#18` ②③）。
//
// 四组：
//
//	① singleFit 逐条对齐官方规则（长边 240 / 比例夹 [0.25,4] / 不放大 / 锚点）；
//	② object-fit:cover 的裁切：比例保持、锚点正确、越界不外扩；
//	③ 三态：加载中画官方 loading 文案、失败画官方「点击重试」、成功画**真位图**；
//	④ 变体：单图走 single 大图、多附件降级 64×64 瓦片（官方 ImageGallery:174）。
//
// "真位图"是真的 PNG 字节经 `internal/imgdecode` 解码后的 `gfx.Bitmap` 上屏：
// 断言落在 `gfx.Canvas.Images()`（`viva@0.6.5/gfx/canvas.go:2100` 的只读快照），
// 不是"画了个框就算过"。

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// solidPNG 编一张真 PNG（宽×高、纯色），返回字节。
//
// 为什么在测试里现编：图片面必须用**真图**验，不许手画占位冒充；
// 现编也没有夹具文件随仓漂移的问题。
func solidPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	source := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			source.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 0x40, A: 0xFF})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, source); err != nil {
		t.Fatalf("编 PNG 失败：%v", err)
	}
	return buf.Bytes()
}

// TestSingleFitOfficialRule 逐条对齐 `MessageImage.tsx:36-48`。
func TestSingleFitOfficialRule(t *testing.T) {
	cases := []struct {
		name        string
		width       int
		height      int
		wantW       float32
		wantH       float32
		wantAnchor  messageImageAnchor
		whyOfficial string
	}{
		{"横图在比例范围内", 4000, 3000, 240, 180, anchorCenter,
			"ratio=1.33 → box 240×180，scale=1（原图够大）"},
		{"竖图在比例范围内", 3000, 4000, 180, 240, anchorCenter,
			"ratio=0.75 → box 180×240"},
		{"极高图夹到 0.25 并保住顶部", 100, 1000, 60, 240, anchorCenterTop,
			"natural=0.1<0.25 → ratio=0.25 → box 60×240，锚点 center top"},
		{"极宽图夹到 4 并保住左边", 3000, 100, 240, 60, anchorLeftCenter,
			"natural=30>4 → ratio=4 → box 240×60，锚点 left center"},
		{"小图不放大", 100, 50, 100, 50, anchorCenter,
			"scale=min(1, 100/240)=0.4167 → 回到原尺寸（never upscaled）"},
		{"尺寸未知按 240 方裁", 0, 0, 240, 240, anchorCenter,
			"dimensions undefined → {240,240,center}（:96-98）"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotW, gotH, gotAnchor := singleFit(c.width, c.height)
			if gotW != c.wantW || gotH != c.wantH {
				t.Fatalf("%d×%d → %v×%v，期望 %v×%v（%s）", c.width, c.height,
					gotW, gotH, c.wantW, c.wantH, c.whyOfficial)
			}
			if gotAnchor != c.wantAnchor {
				t.Fatalf("%d×%d 的 object-position 档 = %v，期望 %v", c.width, c.height, gotAnchor, c.wantAnchor)
			}
		})
	}
}

// TestImageCoverCropKeepsAspect：裁切保持比例、锚点保住该保住的边。
func TestImageCoverCropKeepsAspect(t *testing.T) {
	// 一张 100×1000 的极高图（每行像素都不同，便于验锚点）。
	source := gfx.Bitmap{Width: 100, Height: 1000, Stride: 400, Pix: make([]byte, 400*1000)}
	for y := 0; y < 1000; y++ {
		for x := 0; x < 100; x++ {
			source.Pix[y*400+x*4] = byte(y % 256) // B 通道记行号
		}
	}

	// 官方极高档：框 60×240（ratio 0.25）。源比例 0.1 < 0.25 → 纵向裁到 100×400。
	crop := imageCoverCrop(source, 60, 240, anchorCenterTop)
	if crop.Width != 100 || crop.Height != 400 {
		t.Fatalf("裁切尺寸 = %d×%d，期望 100×400（100/0.25）", crop.Width, crop.Height)
	}
	if crop.Stride != 400 {
		t.Fatalf("Stride = %d，期望 400", crop.Stride)
	}
	// `center top`：保住顶部 → 第一行还是源第 0 行。
	if crop.Pix[0] != 0 {
		t.Fatalf("center top 必须从源第 0 行起，实际首行 B=%d", crop.Pix[0])
	}

	// 居中档：同一张图按 center 裁 → 从源第 300 行起。
	centered := imageCoverCrop(source, 60, 240, anchorCenter)
	if got := centered.Pix[0]; got != 300%256 {
		t.Fatalf("center 应从源第 300 行起，实际首行 B=%d", got)
	}

	// 极宽档：源 1000×100、框 240×60（ratio 4）。源比例 10 > 4 → 横向裁到 400×100。
	wide := gfx.Bitmap{Width: 1000, Height: 100, Stride: 4000, Pix: make([]byte, 4000*100)}
	for y := 0; y < 100; y++ {
		for x := 0; x < 1000; x++ {
			wide.Pix[y*4000+x*4] = byte(x % 256)
		}
	}
	left := imageCoverCrop(wide, 240, 60, anchorLeftCenter)
	if left.Width != 400 || left.Height != 100 {
		t.Fatalf("横向裁切 = %d×%d，期望 400×100", left.Width, left.Height)
	}
	if left.Pix[0] != 0 {
		t.Fatalf("left center 必须从源第 0 列起，实际首列 B=%d", left.Pix[0])
	}

	// 比例本来就等于框 → 不裁（避免无意义的内存拷贝改变像素）。
	square := gfx.Bitmap{Width: 64, Height: 64, Stride: 256, Pix: make([]byte, 256*64)}
	if same := imageCoverCrop(square, 64, 64, anchorCenter); same.Width != 64 || same.Height != 64 {
		t.Fatalf("同比例不该裁：%d×%d", same.Width, same.Height)
	}
}

// newImageTestEnv 造一份带 `AttachmentGet` 的 Env（渲染真实画布）。
func newImageTestEnv(t *testing.T, canvas *gfx.Canvas, tree *gfx.Tree, act pluginapi.Actions) *pluginapi.Env {
	t.Helper()
	theme := dscore.Dark()
	return &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("message-image-test"),
		Canvas: canvas, Bounds: gfx.Rect{Width: 900, Height: 700}, MainWidth: 900,
		Act: act,
	}
}

// drawUserWithAttachment 在一个真画布上画一条带附件的用户消息。
func drawUserWithAttachment(t *testing.T, view *OfficialTranscriptView, env *pluginapi.Env,
	entry TranscriptEntry) *gfx.Canvas {
	t.Helper()
	canvas, _ := newTranscriptCanvas()
	env.Canvas = canvas
	pageTheme := dscore.VivaTheme(dscore.Dark())
	fv := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	fv.Draw(canvas, gfx.Rect{Width: 900, Height: 700}, func(f ui.C) {
		view.Draw(f, env, TranscriptViewport{HostScroll: true}, []TranscriptEntry{entry})
	})
	return canvas
}

// TestMessageImageStates 覆盖官方三态里的两档可见态（loading / failed）。
func TestMessageImageStates(t *testing.T) {
	theme := dscore.Dark()

	t.Run("加载中画官方文案", func(t *testing.T) {
		view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
		_, tree := newTranscriptCanvas()
		env := newImageTestEnv(t, nil, tree, pluginapi.Actions{})
		// 先在缓存里占一个"正在飞"的位：这样这一帧不会再起请求，
		// 状态稳定停在 pending，断言才是确定性的。
		view.messageImages.lookup("att-loading", false)
		canvas := drawUserWithAttachment(t, view, env, TranscriptEntry{
			Key: "u", Kind: ChatNodeUser, Seq: 1, Turn: 0, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Attachments: []TranscriptAttachment{{Image: true, ID: "att-loading"}}},
		})
		mustCanvasText(t, canvas, MessageImageLoading)
		if _, ok := canvasText(canvas, MessageImageLoadFailed); ok {
			t.Fatal("还没失败就不许画失败文案")
		}
		if len(canvas.Images()) != 0 {
			t.Fatalf("加载中不该有图元，实际 %d 张", len(canvas.Images()))
		}
	})

	t.Run("失败画官方重试文案", func(t *testing.T) {
		view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
		_, tree := newTranscriptCanvas()
		// 壳回"没有字节"：图片面必须画失败态（可点重试），不是空框。
		env := newImageTestEnv(t, nil, tree, pluginapi.Actions{
			AttachmentGet: func(string) (pluginapi.AttachmentBlob, bool) { return pluginapi.AttachmentBlob{}, false },
		})
		view.messageImages.loadMessageImage("att-bad", env.Act.AttachmentGet, nil)
		canvas := drawUserWithAttachment(t, view, env, TranscriptEntry{
			Key: "u", Kind: ChatNodeUser, Seq: 1, Turn: 0, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Attachments: []TranscriptAttachment{{Image: true, ID: "att-bad"}}},
		})
		mustCanvasText(t, canvas, MessageImageLoadFailed)
		if len(canvas.Images()) != 0 {
			t.Fatal("失败态不该画图")
		}
	})
}

// TestMessageImagePaintsRealBitmap：成功态画的是**解码后的真位图**，尺寸按官方
// singleFit 定（single 档），不是 64×64 空槽。
func TestMessageImagePaintsRealBitmap(t *testing.T) {
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	_, tree := newTranscriptCanvas()

	body := solidPNG(t, 400, 300) // 4:3
	get := func(id string) (pluginapi.AttachmentBlob, bool) {
		if id != "att-real" {
			return pluginapi.AttachmentBlob{}, false
		}
		return pluginapi.AttachmentBlob{ID: id, Name: "shot.png", MIME: "image/png",
			Width: 400, Height: 300, Bytes: body}, true
	}
	env := newImageTestEnv(t, nil, tree, pluginapi.Actions{AttachmentGet: get})
	view.messageImages.loadMessageImage("att-real", get, nil)

	canvas := drawUserWithAttachment(t, view, env, TranscriptEntry{
		Key: "u", Kind: ChatNodeUser, Seq: 1, Turn: 0, Visibility: ChatVisibilityVisible,
		Data: TranscriptData{Attachments: []TranscriptAttachment{{Image: true, ID: "att-real", Name: "shot.png"}}},
	})

	images := canvas.Images()
	if len(images) != 1 {
		t.Fatalf("真图片面应有 1 张图元，实际 %d", len(images))
	}
	// 单张附件 → 官方 single 档：4:3 且原图大于 240 长边 → 240×180。
	if images[0].Bounds.Width != 240 || images[0].Bounds.Height != 180 {
		t.Fatalf("单图显示框 = %v×%v，期望 240×180（singleFit）",
			images[0].Bounds.Width, images[0].Bounds.Height)
	}
	if images[0].Bitmap.Width != 400 || images[0].Bitmap.Height != 300 {
		t.Fatalf("位图 = %d×%d，期望源尺寸 400×300", images[0].Bitmap.Width, images[0].Bitmap.Height)
	}
	if _, ok := canvasText(canvas, MessageImageLoading); ok {
		t.Fatal("加载成功之后不该还有 loading 文案")
	}
	if _, ok := canvasText(canvas, MessageImageLoadFailed); ok {
		t.Fatal("加载成功之后不该有失败文案")
	}
}

// TestMessageImageTileWhenSeveralAttachments：附件超过一个 → 官方降级 64×64 瓦片
// （`UserContent.tsx:18` 的 `compactImages` + `ImageGallery:174`）。
func TestMessageImageTileWhenSeveralAttachments(t *testing.T) {
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	_, tree := newTranscriptCanvas()

	body := solidPNG(t, 400, 300)
	get := func(id string) (pluginapi.AttachmentBlob, bool) {
		return pluginapi.AttachmentBlob{ID: id, Name: id + ".png", MIME: "image/png",
			Width: 400, Height: 300, Bytes: body}, true
	}
	env := newImageTestEnv(t, nil, tree, pluginapi.Actions{AttachmentGet: get})
	view.messageImages.loadMessageImage("att-1", get, nil)
	view.messageImages.loadMessageImage("att-2", get, nil)

	canvas := drawUserWithAttachment(t, view, env, TranscriptEntry{
		Key: "u", Kind: ChatNodeUser, Seq: 1, Turn: 0, Visibility: ChatVisibilityVisible,
		Data: TranscriptData{Attachments: []TranscriptAttachment{
			{Image: true, ID: "att-1", Name: "a.png"},
			{Image: true, ID: "att-2", Name: "b.png"},
		}},
	})
	images := canvas.Images()
	if len(images) != 2 {
		t.Fatalf("两张图应画 2 个图元，实际 %d", len(images))
	}
	for i, img := range images {
		if img.Bounds.Width != ImageTileSize || img.Bounds.Height != ImageTileSize {
			t.Fatalf("第 %d 张尺寸 = %v×%v，期望瓦片 %v×%v",
				i, img.Bounds.Width, img.Bounds.Height, ImageTileSize, ImageTileSize)
		}
	}
}
