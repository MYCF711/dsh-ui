package dsconversation

// official_message_image_lightbox_test.go —— **附件图**（`MessageImage`）的灯箱（`#18`）。
//
// # 与正文图那一组的分工
//
// `official_image_lightbox_test.go` 守的是正文 markdown 图（`#233` 接的那一处）。
// 本文件守官方 `MessageImage.tsx:135-145` 那一处——同一个官方 `ImageLightbox`
// 组件（`Jb`）的**第二个**打开点，但触发条件与"放大的到底是哪张图"完全不同：
//
//	正文图   `src` 就是正文那个目的地，位图即最终显示物
//	附件图   `src` 是**原图**，缩略图那 240×180 / 64×64 是 `object-fit:cover` 裁出来的
//	         ⇒ 灯箱里必须是**整张原图**，不许把裁切结果放大（那就成了"点开看放大的缩略图"）
//
// 所以本文件里最关键的一条是 `TestAttachmentLightboxShowsTheWholeImageNotTheCrop`：
// 它拿一张 4000×600 的宽图（`singleFit` 会把它裁成 240×240 的方框），点开之后
// 灯箱里那张图必须是 **4000:600 的比例**，不是 1:1。
//
// 反向判据（"不点图不许有灯箱"）与正文图那组同一条规矩，本文件单独再钉一次：
// 附件图的成功态是**新加的**可点区域，它旁边还并着失败态的重试按钮，两处 clikck
// 只要串一次就是"点重试弹出灯箱"这种事。

import (
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// messageImageLightboxHarness 是"一张已就绪的附件图 + 灯箱座位"的跨帧夹具。
type messageImageLightboxHarness struct {
	t      *testing.T
	view   *OfficialTranscriptView
	plugin *Plugin
	tree   *gfx.Tree
	feat   gfx.Scope
	page   *ui.Page
	env    *pluginapi.Env
	entry  []TranscriptEntry
}

// newMessageImageLightboxHarness 造夹具：附件 id `att-shot`、文件名 `shot.png`。
func newMessageImageLightboxHarness(t *testing.T, body []byte) *messageImageLightboxHarness {
	t.Helper()
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	tree := &gfx.Tree{}
	feat := tree.Feature("message-image-lightbox-test")
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme, Tree: tree, Root: "message-image-lightbox-test"})
	bounds := gfx.Rect{Width: 900, Height: 700}
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: feat, Bounds: bounds, MainWidth: bounds.Width,
		Act: pluginapi.Actions{
			AttachmentGet: func(id string) (pluginapi.AttachmentBlob, bool) {
				if id != "att-shot" {
					return pluginapi.AttachmentBlob{}, false
				}
				return pluginapi.AttachmentBlob{ID: id, Name: "shot.png", MIME: "image/png",
					Width: 0, Height: 0, Bytes: body}, true
			},
		},
		Data: pluginapi.Snapshot{Cur: markdownImageTestSession, CWD: markdownImageTestCWD},
	}
	return &messageImageLightboxHarness{
		t: t, view: view, plugin: &Plugin{officialTranscript: view},
		tree: tree, feat: feat, page: page, env: env,
		entry: []TranscriptEntry{{
			Key: "u", Kind: ChatNodeUser, Seq: 1, Turn: 0, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{
				Text:        "看这张图",
				Attachments: []TranscriptAttachment{{Image: true, ID: "att-shot", Name: "shot.png"}},
			},
		}},
	}
}

// frame 画一帧（正文座位 + `shell.overlay` 的灯箱）。clickID 非零时是它的点击通行。
func (h *messageImageLightboxHarness) frame(clickID gfx.ID) *gfx.Canvas {
	h.t.Helper()
	canvas := &gfx.Canvas{}
	canvas.SetTree(h.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size
	})
	h.env.Canvas = canvas
	bounds := h.env.Bounds
	if !clickID.IsZero() {
		canvas.BeginEventPass(clickID, gfx.ClickInfo{})
	}
	h.page.Draw(canvas, bounds, func(f ui.C) {
		h.view.Draw(f, h.env, hostViewport(h.tree), h.entry)
		h.plugin.drawImageLightbox(f, h.env)
	})
	if !clickID.IsZero() {
		canvas.EndEventPass()
	}
	return canvas
}

// ready 造夹具并等那张附件图真的解码就绪（再画一帧让它上屏）。
func (h *messageImageLightboxHarness) ready() *messageImageLightboxHarness {
	h.t.Helper()
	h.frame(gfx.ID{})
	deadline := time.Now().Add(5 * time.Second)
	for {
		snapshot, _ := h.view.messageImages.lookup("att-shot", false)
		if snapshot.state == messageImageReady {
			break
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("等附件图解码超时，状态 = %v", snapshot.state)
		}
		time.Sleep(2 * time.Millisecond)
	}
	h.frame(gfx.ID{})
	return h
}

// openButton 是本帧那张图的「点开灯箱」按钮（官方 `aria-label = openNamed(label)`）。
func (h *messageImageLightboxHarness) openButton(canvas *gfx.Canvas) gfx.Hit {
	h.t.Helper()
	label := messageImageOpenLabel("shot.png")
	hit, ok := hitByLabel(canvas, label)
	if !ok {
		h.t.Fatalf("附件图没有「查看原图」按钮（label=%q）；本帧命中区：%v", label, hitLabels(canvas))
	}
	return hit
}

// lightboxMask 找灯箱那层遮罩的命中区（`role=dialog` 的可访问名就是 `image.dialog`）。
func (h *messageImageLightboxHarness) lightboxMask(canvas *gfx.Canvas) (gfx.Hit, bool) {
	return hitByLabel(canvas, ImageLightboxDialog)
}

// TestAttachmentLightboxNeedsAUserClick 钉反向判据：没点图，灯箱一个命中区都没有。
//
// 这条是本单最重要的一条：附件图这一处是**新加的可点区域**，它旁边并着失败态重试
// 按钮；如果灯箱能被别的东西（比如"只要图就绪就画"）触发，用户会看到一层没有来源的
// 遮罩。判据是"遮罩的命中区不存在"，不是"某个结构没找到"。
func TestAttachmentLightboxNeedsAUserClick(t *testing.T) {
	body := solidPNG(t, 400, 300)
	h := newMessageImageLightboxHarness(t, body).ready()

	// 就绪那一帧：按钮在（成功态可点），灯箱不在。
	canvas := h.frame(gfx.ID{})
	h.openButton(canvas)
	if _, ok := h.lightboxMask(canvas); ok {
		t.Fatalf("没点图就出现了灯箱遮罩；本帧命中区：%v", hitLabels(canvas))
	}
	if h.view.imageLightboxOpen() {
		t.Fatal("没点图，灯箱状态却已经是开的")
	}

	// 再画几帧（消息重排、窗口尺寸变化）也不许自己冒出来。
	for i := 0; i < 3; i++ {
		canvas = h.frame(gfx.ID{})
		if _, ok := h.lightboxMask(canvas); ok {
			t.Fatalf("第 %d 次重画后灯箱自己冒出来了", i+2)
		}
	}
}

// TestAttachmentLightboxOpensOnClick 钉正向：点那张图 → 灯箱出现，且三件齐全
// （遮罩 / 图 / 关闭按钮），与官方 `Jb` 的结构一致。
func TestAttachmentLightboxOpensOnClick(t *testing.T) {
	body := solidPNG(t, 400, 300)
	h := newMessageImageLightboxHarness(t, body).ready()
	canvas := h.frame(gfx.ID{})
	button := h.openButton(canvas)

	canvas = h.frame(button.ID)
	if !h.view.imageLightboxOpen() {
		t.Fatal("点了图，灯箱没开")
	}
	mask, ok := h.lightboxMask(canvas)
	if !ok {
		t.Fatalf("灯箱开着却找不到遮罩；命中区：%v", hitLabels(canvas))
	}
	if mask.Bounds.Width != h.env.Bounds.Width || mask.Bounds.Height != h.env.Bounds.Height {
		t.Fatalf("遮罩几何 = %v×%v，官方 `._mask{inset:0}` 要铺满 overlay 盒 %v×%v",
			mask.Bounds.Width, mask.Bounds.Height, h.env.Bounds.Width, h.env.Bounds.Height)
	}
	if _, ok := hitByLabel(canvas, ImageLightboxClose); !ok {
		t.Fatalf("灯箱没有关闭按钮；命中区：%v", hitLabels(canvas))
	}
	// 灯箱里那张图**吃点击**（官方 `img` 在 `._mask` 之上、自己没有 handler）：
	// 不给它命中区的话，点图会落到遮罩上直接关掉，比官方多一种"手一滑就关"。
	if _, ok := hitByLabel(canvas, "shot.png"); !ok {
		t.Fatalf("灯箱里的图没有命中区；命中区：%v", hitLabels(canvas))
	}
}

// TestAttachmentLightboxShowsTheWholeImageNotTheCrop 钉"放大的是原图"。
//
// 夹具是 4000×600（比例 6.67:1）：`singleFit` 会把**缩略图**夹到 [0.25,4] ⇒
// 显示框是 240×60 里再 `cover` 裁成 240×240 的方框。灯箱如果错手拿裁切结果，
// 比例会是 1:1；正确的话是 4000:600 = 20:3。
//
// 窗口 900×700、stage padding 40 ⇒ 内容盒 820×620；`min(100%,1600px)` 取 820。
// 4000×600 等比缩到宽 820 ⇒ 高 123。所以判据是 820×123，不是 820×820。
func TestAttachmentLightboxShowsTheWholeImageNotTheCrop(t *testing.T) {
	const sourceWidth, sourceHeight = 4000, 600
	body := solidPNG(t, sourceWidth, sourceHeight)
	h := newMessageImageLightboxHarness(t, body)
	// 尺寸真值由位图兜底（宿主一份都没报），所以这里必须真解码。
	h.ready()

	canvas := h.frame(gfx.ID{})
	button := h.openButton(canvas)
	canvas = h.frame(button.ID)
	if !h.view.imageLightboxOpen() {
		t.Fatal("点了图，灯箱没开")
	}

	var found *gfx.Image
	for _, img := range canvas.Images() {
		if img.Bitmap.Width == sourceWidth && img.Bitmap.Height == sourceHeight {
			copied := img
			found = &copied
			break
		}
	}
	if found == nil {
		t.Fatalf("灯箱里没有 %d×%d 的原图图元（拿到的是裁切结果？）", sourceWidth, sourceHeight)
	}
	wantWidth, wantHeight := lightboxImageFit(gfx.Bitmap{Width: sourceWidth, Height: sourceHeight}, h.env.Bounds)
	if found.Bounds.Width != wantWidth || found.Bounds.Height != wantHeight {
		t.Fatalf("灯箱图显示框 = %v×%v，期望 %v×%v（官方 max-width min(100%%,1600px) + object-fit:contain）",
			found.Bounds.Width, found.Bounds.Height, wantWidth, wantHeight)
	}
	if found.Bounds.Width == found.Bounds.Height {
		t.Fatalf("灯箱图是方的（%v×%v）—— 放大的是 cover 裁切后的缩略图，不是原图",
			found.Bounds.Width, found.Bounds.Height)
	}
}

// TestAttachmentLightboxClosePaths 钉三条关闭路径，外加"点图本身不关"。
func TestAttachmentLightboxClosePaths(t *testing.T) {
	body := solidPNG(t, 400, 300)
	open := func(h *messageImageLightboxHarness) *gfx.Canvas {
		t.Helper()
		canvas := h.frame(gfx.ID{})
		button := h.openButton(canvas)
		canvas = h.frame(button.ID)
		if !h.view.imageLightboxOpen() {
			t.Fatal("前置失败：灯箱没开")
		}
		return canvas
	}

	// ① 点遮罩
	h := newMessageImageLightboxHarness(t, body).ready()
	canvas := open(h)
	mask, _ := h.lightboxMask(canvas)
	h.frame(mask.ID)
	if h.view.imageLightboxOpen() {
		t.Fatal("点遮罩没关掉灯箱")
	}

	// ② 点关闭按钮
	h = newMessageImageLightboxHarness(t, body).ready()
	canvas = open(h)
	closeHit, _ := hitByLabel(canvas, ImageLightboxClose)
	h.frame(closeHit.ID)
	if h.view.imageLightboxOpen() {
		t.Fatal("点关闭按钮没关掉灯箱")
	}

	// ③ **点图本身不关**（官方 `img` 没有 handler）
	h = newMessageImageLightboxHarness(t, body).ready()
	canvas = open(h)
	imageHit, ok := hitByLabel(canvas, "shot.png")
	if !ok {
		t.Fatal("灯箱里的图没有命中区")
	}
	h.frame(imageHit.ID)
	if !h.view.imageLightboxOpen() {
		t.Fatal("点灯箱里的图把灯箱关了——官方点图不关")
	}
}

// TestAttachmentLightboxNeedsReadyImage 钉"没就绪就没有可点入口"。
//
// 官方 `MessageImage.tsx:135-145` 的 onClick 是 `if (src !== null) setOpen(true)`：
// 字节没回来时连 `<img>` 都没有。本仓同一道门：加载中那帧不该有"查看原图"按钮。
func TestAttachmentLightboxNeedsReadyImage(t *testing.T) {
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	tree := &gfx.Tree{}
	feat := tree.Feature("message-image-not-ready")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size
	})
	env := &pluginapi.Env{Theme: theme, Tree: tree, Feat: feat, Canvas: canvas,
		Bounds: gfx.Rect{Width: 900, Height: 700}, MainWidth: 900,
		Act: pluginapi.Actions{AttachmentGet: func(string) (pluginapi.AttachmentBlob, bool) {
			return pluginapi.AttachmentBlob{}, false
		}}}
	entry := TranscriptEntry{
		Key: "u", Kind: ChatNodeUser, Seq: 1, Visibility: ChatVisibilityVisible,
		Data: TranscriptData{Attachments: []TranscriptAttachment{{Image: true, ID: "att-shot", Name: "shot.png"}}},
	}
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme, Tree: tree, Root: "message-image-not-ready"})
	page.Draw(canvas, env.Bounds, func(f ui.C) {
		view.Draw(f, env, hostViewport(tree), []TranscriptEntry{entry})
	})
	if _, ok := hitByLabel(canvas, messageImageOpenLabel("shot.png")); ok {
		t.Fatalf("图还没就绪就有「查看原图」按钮；命中区：%v", hitLabels(canvas))
	}
}
