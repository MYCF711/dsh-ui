package dsconversation

// official_image_lightbox_test.go —— 正文图「查看大图」灯箱（`#18` (a)）的图元账。
//
// # 这条守的是什么
//
// 官方把出好的正文图包在一枚 `<button cursor:zoom-in title=查看大图>` 里，点它才挂
// 灯箱（`OFF:dist/assets/index-Ciz7DZxv.js` byte 978669 的 `lK`；灯箱本体是同文件
// byte 969168 的 `Jb`）。本仓原来没有这一层，用户点了没反应。
//
// 这组用例钉五件事：
//
//	① **反向**：没点图时灯箱一个像素、一个命中区都不许有（"不新增非用户触发的浮层"）；
//	② 点图 → 灯箱出现，几何按官方 1600 / 窗口 - 80 / 居中 / 右上 20 那一套；
//	③ 三条关闭路径（遮罩 / 关闭按钮 / Esc）都真的关，而**点图本身不关**（官方
//	   `img` 没有 handler，事件不冒泡到 `._mask`）；
//	④ Esc 走的是页面级 `ui.Shortcut`（官方 capture + stopPropagation 的等价物），
//	   并且 `HandleKey` 那支兜底在灯箱关着时**不吞**这个键（拒绝审批/停止 turn 照旧）；
//	⑤ 字节没回来 / 取失败时**没有**"查看大图"按钮（官方那时连 `<img>` 都没有）。
//
// 真窗口像素判据在 `official_image_lightbox_headless_test.go`。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// lightboxHarness 是跨帧的绘制夹具：树与页面共用，画布每帧新开（与产品同构：
// 一帧一次布局，交互 id 必须跨帧稳定）。
type lightboxHarness struct {
	t      *testing.T
	view   *OfficialTranscriptView
	plugin *Plugin
	tree   *gfx.Tree
	feat   gfx.Scope
	page   *ui.Page
	env    *pluginapi.Env
	entry  []TranscriptEntry
}

// newLightboxHarness 造一套"图已就绪"的夹具。
func newLightboxHarness(t *testing.T, act pluginapi.Actions) *lightboxHarness {
	t.Helper()
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	tree := &gfx.Tree{}
	// Root 与壳的 `pageRoot` 同位：页面级 `ui.Shortcut` 的 pageWide 判据要它
	// （`viva/base/shortcut.go` 的 `pageWide`；壳里 `pageRoot = "home"`）。
	feat := tree.Feature("image-lightbox-test")
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme, Tree: tree, Root: "image-lightbox-test"})
	bounds := gfx.Rect{Width: 900, Height: 700}
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: feat, Bounds: bounds, MainWidth: bounds.Width, Act: act,
		Data: pluginapi.Snapshot{Cur: markdownImageTestSession, CWD: markdownImageTestCWD},
	}
	h := &lightboxHarness{
		t: t, view: view, plugin: &Plugin{officialTranscript: view},
		tree: tree, feat: feat, page: page, env: env,
		entry: []TranscriptEntry{{
			Key: "md-image", Kind: ChatNodeAssistantStep, Seq: 1, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Text: "看图\n\n![示意图](./shot.png)"},
		}},
	}
	return h
}

// frame 画一帧：正文座位 + `shell.overlay` 的灯箱（与产品同样的先后）。
//
// clickID 非零时这一帧是那个交互 id 的**点击通行**（`Clicked()` 为真）。
func (h *lightboxHarness) frame(clickID gfx.ID) *gfx.Canvas {
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
		// `shell.overlay` 的灯箱座位永远最后画（Order 110）。
		h.plugin.drawImageLightbox(f, h.env)
	})
	if !clickID.IsZero() {
		canvas.EndEventPass()
	}
	return canvas
}

// hitByLabel 找本帧那个可访问名对应的命中区。
func hitByLabel(canvas *gfx.Canvas, label string) (gfx.Hit, bool) {
	for _, hit := range canvas.Hits() {
		if hit.Label == label {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// markdownImageButton 是正文图那枚按钮的命中区（官方 `aria-label` 逐字：
// `alt ? "查看大图: " + alt : "查看大图"`）。
func markdownImageButton(t *testing.T, canvas *gfx.Canvas, alt string) gfx.Hit {
	t.Helper()
	hit, ok := hitByLabel(canvas, markdownImageOpenLabel(alt))
	if !ok {
		t.Fatalf("正文图没有「查看大图」按钮（label=%q）；本帧命中区：%v",
			markdownImageOpenLabel(alt), hitLabels(canvas))
	}
	return hit
}

// hitLabels 是整帧的命中区名字（失败诊断用）。
func hitLabels(canvas *gfx.Canvas) []string {
	hits := canvas.Hits()
	out := make([]string, 0, len(hits))
	for _, hit := range hits {
		out = append(out, hit.Label)
	}
	return out
}

// readyLightboxHarness 造夹具并等正文图真的解码就绪。
func readyLightboxHarness(t *testing.T, body []byte) *lightboxHarness {
	t.Helper()
	h := newLightboxHarness(t, pluginapi.Actions{
		MarkdownImage: func(_, _, destination string) (pluginapi.MarkdownImageBytes, bool) {
			if destination != "./shot.png" {
				return pluginapi.MarkdownImageBytes{}, false
			}
			return pluginapi.MarkdownImageBytes{Bytes: body, MIME: "image/png"}, true
		},
	})
	// 第 1 帧触发取数，等后台解码落库，再画一帧让它上屏。
	h.frame(gfx.ID{})
	waitMarkdownImage(t, h.view, "./shot.png", mdlite.MarkdownImageReady)
	h.frame(gfx.ID{})
	return h
}

// TestImageLightboxNeedsAUserClick 钉 ①：没点图，灯箱一个像素、一个命中区都没有。
//
// 这是「不许新增非用户触发的浮层」的机器判据——正向用例（下面那些）如果哪天
// 变成"自己弹出来"，这条会立刻红。
func TestImageLightboxNeedsAUserClick(t *testing.T) {
	h := readyLightboxHarness(t, solidColorPNG(t, 300, 150, messageImageFixtureColor))

	canvas := h.frame(gfx.ID{})
	if h.view.markdownLightboxOpen() {
		t.Fatal("没人点图，灯箱却开着")
	}
	if images := canvas.Images(); len(images) != 1 {
		t.Fatalf("没点图时屏上 %d 张位图，期望 1 张（只有正文那张）", len(images))
	}
	for _, label := range []string{ImageLightboxDialog, ImageLightboxClose} {
		if hit, ok := hitByLabel(canvas, label); ok {
			t.Fatalf("没点图就出现了灯箱命中区 %q：%v", label, hit.Bounds)
		}
	}
	// 反过来：正文图那枚按钮必须在（下面三条关闭路径都从它开始）。
	button := markdownImageButton(t, canvas, "示意图")
	if button.Role != gfx.RoleButton {
		t.Fatalf("正文图按钮的 role = %v，期望 button（官方是 <button>）", button.Role)
	}
	if button.Cursor != gfx.PointerCursorHand {
		t.Fatalf("正文图按钮的指针 = %v，期望手型（官方 cursor:zoom-in，viva 无这一档，见差异 ①）", button.Cursor)
	}
}

// TestImageLightboxGeometry 钉 ②：点图之后灯箱的盒子、图、关闭按钮的几何。
func TestImageLightboxGeometry(t *testing.T) {
	// 300×150 没超上限 ⇒ 灯箱里也不放大（官方的 width/height:auto 从不放大），
	// 显示框 = 原始尺寸，居中在 900×700 的 stage 里。
	h := readyLightboxHarness(t, solidColorPNG(t, 300, 150, messageImageFixtureColor))
	button := markdownImageButton(t, h.frame(gfx.ID{}), "示意图")

	canvas := h.frame(button.ID)
	if !h.view.markdownLightboxOpen() {
		t.Fatal("点了正文图，灯箱没开")
	}

	// 遮罩：全窗（官方 `._mask_19zw2_19{position:absolute;inset:0}`）。
	backdrop, ok := hitByLabel(canvas, ImageLightboxDialog)
	if !ok {
		t.Fatalf("灯箱没有遮罩命中区（aria-label=%q）：%v", ImageLightboxDialog, hitLabels(canvas))
	}
	if backdrop.Bounds != h.env.Bounds {
		t.Fatalf("遮罩 = %v，期望整窗 %v", backdrop.Bounds, h.env.Bounds)
	}
	if backdrop.Role != gfx.RoleGroup {
		t.Fatalf("遮罩 role = %v，期望 group（viva 没有 RoleDialog，见文件头）", backdrop.Role)
	}

	// 图：两张位图 = 正文那张 + 灯箱这张。
	images := canvas.Images()
	if len(images) != 2 {
		t.Fatalf("灯箱打开时 %d 张位图，期望 2 张（正文 + 灯箱）", len(images))
	}
	lens := images[len(images)-1]
	want := gfx.Rect{X: (900 - 300) / 2, Y: (700 - 150) / 2, Width: 300, Height: 150}
	if lens.Bounds != want {
		t.Fatalf("灯箱图 = %v，期望 %v（官方 max-width:min(100%%,1600px)+max-height:100%%+居中）", lens.Bounds, want)
	}
	if lens.Bitmap.Width != 300 || lens.Bitmap.Height != 150 {
		t.Fatalf("灯箱图的源位图 = %d×%d，期望 300×150（原图，不是缩略）", lens.Bitmap.Width, lens.Bitmap.Height)
	}

	// 关闭按钮：右上 20、36×36（官方 `._close_19zw2_37{top:20px;right:20px;width/height:36px}`）。
	closeHit, ok := hitByLabel(canvas, ImageLightboxClose)
	if !ok {
		t.Fatalf("灯箱没有关闭按钮（aria-label=%q）：%v", ImageLightboxClose, hitLabels(canvas))
	}
	wantClose := gfx.Rect{X: 900 - 20 - 36, Y: 20, Width: 36, Height: 36}
	if closeHit.Bounds != wantClose {
		t.Fatalf("关闭按钮 = %v，期望 %v", closeHit.Bounds, wantClose)
	}
	if closeHit.Role != gfx.RoleButton {
		t.Fatalf("关闭按钮 role = %v，期望 button", closeHit.Role)
	}

	// 灯箱图自己也是一层命中区：它的唯一作用是吃掉点击（官方点图不关）。
	if _, ok := hitByLabel(canvas, "示意图"); !ok {
		t.Fatalf("灯箱里那张图没有命中区，点图会漏到遮罩上（官方点图不关）：%v", hitLabels(canvas))
	}
}

// TestImageLightboxFitCaps 钉 ②的缩放规则：1600 宽上限、窗口 - 80 的高度上限、
// 从不放大。判据是纯函数 + 真实绘制两条，两边必须一致。
func TestImageLightboxFitCaps(t *testing.T) {
	cases := []struct {
		name          string
		bitmapW       int
		bitmapH       int
		boundsW       float32
		boundsH       float32
		wantW, wantH  float32
		wantX, wantY  float32
		maxWidthGuard string
	}{
		// 原尺寸不出格：不放大。
		{"原尺寸", 300, 150, 900, 700, 300, 150, 300, 275, ""},
		// 宽超 stage 内容盒（900-80=820）⇒ 按宽压。
		{"宽压到窗口", 2000, 1000, 900, 700, 820, 410, 40, 145, ""},
		// 高超 stage 内容盒（700-80=620）⇒ 按高压。
		{"高压到窗口", 400, 3000, 900, 700, 83, 620, 408.5, 40, ""},
		// 窗口很宽时 1600 那条上限先咬住（没有它这里会是 1920 宽）。
		{"1600 上限", 1700, 900, 2000, 700, 1171, 620, 414.5, 40, "1600"},
		// 窗口很窄：宽度上限是 100%（不是 1600）。
		{"窄窗", 1600, 800, 500, 700, 420, 210, 40, 245, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bitmap := gfx.Bitmap{Width: tc.bitmapW, Height: tc.bitmapH}
			w, h := lightboxImageFit(bitmap, gfx.Rect{Width: tc.boundsW, Height: tc.boundsH})
			if w != tc.wantW || h != tc.wantH {
				t.Fatalf("fit(%d×%d, %v×%v) = %v×%v，期望 %v×%v",
					tc.bitmapW, tc.bitmapH, tc.boundsW, tc.boundsH, w, h, tc.wantW, tc.wantH)
			}
			gotX, gotY := (tc.boundsW-w)/2, (tc.boundsH-h)/2
			if gotX != tc.wantX || gotY != tc.wantY {
				t.Fatalf("居中位置 = (%.1f,%.1f)，期望 (%.1f,%.1f)", gotX, gotY, tc.wantX, tc.wantY)
			}
			if tc.maxWidthGuard == "1600" && w > ImageLightboxImageMaxWidth {
				t.Fatalf("宽 %v 超过 1600 上限", w)
			}
		})
	}
	// 0 尺寸 / 0 窗口不出图（不画一个空盒子）。
	if w, h := lightboxImageFit(gfx.Bitmap{}, gfx.Rect{Width: 900, Height: 700}); w != 0 || h != 0 {
		t.Fatalf("空位图 fit = %v×%v，期望 0×0", w, h)
	}
	if w, h := lightboxImageFit(gfx.Bitmap{Width: 10, Height: 10}, gfx.Rect{}); w != 0 || h != 0 {
		t.Fatalf("空窗口 fit = %v×%v，期望 0×0", w, h)
	}
}

// TestImageLightboxClosePaths 钉 ③：遮罩 / 关闭按钮真的关，而点图不关。
func TestImageLightboxClosePaths(t *testing.T) {
	openLightbox := func(t *testing.T) (*lightboxHarness, gfx.Hit) {
		t.Helper()
		h := readyLightboxHarness(t, solidColorPNG(t, 300, 150, messageImageFixtureColor))
		button := markdownImageButton(t, h.frame(gfx.ID{}), "示意图")
		h.frame(button.ID)
		if !h.view.markdownLightboxOpen() {
			t.Fatal("夹具没把灯箱打开")
		}
		return h, button
	}

	t.Run("点遮罩关闭", func(t *testing.T) {
		h, _ := openLightbox(t)
		backdrop, ok := hitByLabel(h.frame(gfx.ID{}), ImageLightboxDialog)
		if !ok {
			t.Fatal("没有遮罩命中区")
		}
		h.frame(backdrop.ID)
		if h.view.markdownLightboxOpen() {
			t.Fatal("点了遮罩，灯箱还开着")
		}
		// 关掉之后那一帧回到"一张位图 + 没有灯箱命中区"。
		canvas := h.frame(gfx.ID{})
		if images := canvas.Images(); len(images) != 1 {
			t.Fatalf("关掉之后 %d 张位图，期望 1 张", len(images))
		}
		for _, label := range []string{ImageLightboxDialog, ImageLightboxClose} {
			if _, ok := hitByLabel(canvas, label); ok {
				t.Fatalf("关掉之后还有灯箱命中区 %q", label)
			}
		}
	})

	t.Run("点关闭按钮关闭", func(t *testing.T) {
		h, _ := openLightbox(t)
		closeHit, ok := hitByLabel(h.frame(gfx.ID{}), ImageLightboxClose)
		if !ok {
			t.Fatal("没有关闭按钮命中区")
		}
		h.frame(closeHit.ID)
		if h.view.markdownLightboxOpen() {
			t.Fatal("点了关闭按钮，灯箱还开着")
		}
	})

	t.Run("点图本身不关", func(t *testing.T) {
		h, _ := openLightbox(t)
		// 灯箱图自己那层命中区（label = alt）。
		imageHit, ok := hitByLabel(h.frame(gfx.ID{}), "示意图")
		if !ok {
			t.Fatal("灯箱里那张图没有命中区")
		}
		h.frame(imageHit.ID)
		if !h.view.markdownLightboxOpen() {
			t.Fatal("点图把灯箱关掉了；官方点图不关（img 没有 handler）")
		}
	})

	t.Run("切会话自动关", func(t *testing.T) {
		h, _ := openLightbox(t)
		h.env.Data.Cur = "another-session"
		if h.view.markdownLightboxVisible(h.env) {
			t.Fatal("切了会话灯箱还可见；官方那时整段 DOM 已卸载")
		}
		if h.view.markdownLightboxOpen() {
			t.Fatal("切了会话灯箱状态还没清")
		}
	})
}

// TestImageLightboxEscapeOwnsTheKey 钉 ④：Esc 走页面级 `ui.Shortcut`（官方
// capture + stopPropagation），且 `HandleKey` 的兜底在灯箱关着时不吞这个键。
func TestImageLightboxEscapeOwnsTheKey(t *testing.T) {
	h := readyLightboxHarness(t, solidColorPNG(t, 300, 150, messageImageFixtureColor))
	button := markdownImageButton(t, h.frame(gfx.ID{}), "示意图")
	h.frame(button.ID)
	closeHit, ok := hitByLabel(h.frame(gfx.ID{}), ImageLightboxClose)
	if !ok {
		t.Fatal("没有关闭按钮命中区")
	}

	// 真实路由：`ui.Page.Key` → matchShortcut → `Activate` → 下一次画是那次点击的通行。
	// Target 传关闭按钮（真机上焦点被 trap 在它身上，见 SetInitialFocus）。
	result := h.page.Key(gfx.KeyEvent{Key: gfx.KeyEscape, Target: closeHit.ID})
	if !result.Handled || result.Activate.IsZero() {
		t.Fatalf("Esc 没被页面接住（handled=%v activate=%v）——官方是 capture 阶段 stopPropagation",
			result.Handled, result.Activate)
	}
	h.frame(result.Activate)
	if h.view.markdownLightboxOpen() {
		t.Fatal("Esc 之后灯箱还开着")
	}

	// 兜底那支：`HandleKey`。灯箱关着时不许劫走这个键——拒审批这一支照旧要赢。
	denied := 0
	p := &Plugin{officialTranscript: h.view}
	p.lastAct.Deny = func() { denied++ }
	p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape})
	if denied != 1 {
		t.Fatalf("灯箱关着时 Esc 没落到拒绝审批那一支（denied=%d）", denied)
	}

	// 灯箱开着时兜底也必须只关灯箱，不许顺带拒审批。
	h.frame(button.ID)
	if !h.view.markdownLightboxOpen() {
		t.Fatal("重新打开灯箱失败")
	}
	denied = 0
	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) {
		t.Fatal("灯箱开着，HandleKey 的 Esc 兜底没接住")
	}
	if h.view.markdownLightboxOpen() {
		t.Fatal("HandleKey 的 Esc 没关掉灯箱")
	}
	if denied != 0 {
		t.Fatalf("关灯箱顺带拒了审批（denied=%d）——官方 stopPropagation 不许下层收到这个键", denied)
	}
}

// TestImageLightboxNeedsReadyImage 钉 ⑤：字节没回来 / 取失败时没有"查看大图"按钮。
func TestImageLightboxNeedsReadyImage(t *testing.T) {
	t.Run("还没回来", func(t *testing.T) {
		release := make(chan struct{})
		h := newLightboxHarness(t, pluginapi.Actions{
			MarkdownImage: func(_, _, _ string) (pluginapi.MarkdownImageBytes, bool) {
				<-release
				return pluginapi.MarkdownImageBytes{}, false
			},
		})
		canvas := h.frame(gfx.ID{})
		if _, ok := hitByLabel(canvas, markdownImageOpenLabel("示意图")); ok {
			t.Fatal("字节没回来就有「查看大图」按钮——那时官方连 <img> 都没有")
		}
		close(release)
	})

	t.Run("取失败", func(t *testing.T) {
		h := newLightboxHarness(t, pluginapi.Actions{
			MarkdownImage: func(_, _, _ string) (pluginapi.MarkdownImageBytes, bool) {
				return pluginapi.MarkdownImageBytes{}, false
			},
		})
		h.frame(gfx.ID{})
		waitMarkdownImage(t, h.view, "./shot.png", mdlite.MarkdownImageFailed)
		canvas := h.frame(gfx.ID{})
		if _, ok := hitByLabel(canvas, markdownImageOpenLabel("示意图")); ok {
			t.Fatal("取失败还有「查看大图」按钮——官方的失败态是一行文案，没有 <img>")
		}
		if h.view.markdownLightboxOpen() {
			t.Fatal("失败态居然能打开灯箱")
		}
	})
}
