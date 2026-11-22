//go:build darwin || windows || linux

package dsconversation

// official_message_image_lightbox_headless_test.go —— **附件图**灯箱的真窗口像素回归。
//
// 与正文图那份（`official_image_lightbox_headless_test.go`）的分工：那边钉的是 markdown
// 图那一处触发点与整套关闭状态机；本文件只钉附件图这一处**最容易错的一条**——
// 放大的到底是**原图**还是缩略图那块 `object-fit:cover` 裁出来的碎片。
//
// 夹具刻意选 4000×600（6.67:1）：
//
//	缩略图  `singleFit` 把比例夹到 [0.25,4] ⇒ 框 240×60，再 `cover` 裁成 2400×600 那片
//	        ⇒ 裁切结果的比例是 **4:1**
//	灯箱    `min(100%,1600px)` 取 820、`object-fit:contain` ⇒ **820×123**（6.67:1）
//	若错手拿裁切结果             ⇒ 820×205（4:1）
//
// 123 与 205 差 82px，像素判据不可能撞上。再加一条硬像素判据：灯箱盒子里 ≥99% 是
// 夹具色（真图真的上屏），以及反向判据：没点图时底角像素是底色本身、遮罩命中区零个。

import (
	"image"
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

// openAttachmentLightboxWindow 开一个真 headless 窗口，里面只有一张 4000×600 的附件图。
func openAttachmentLightboxWindow(t *testing.T) (*lightboxPixels, *OfficialTranscriptView) {
	t.Helper()
	const width, height = 900, 600
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	plugin := &Plugin{officialTranscript: view}

	body := solidColorPNG(t, 4000, 600, messageImageFixtureColor)

	tree := &gfx.Tree{}
	feat := tree.Feature("attach-lightbox-headless")
	fieldID := tree.ID(feat, "test-input")
	var input ui.Field
	var invalidator app.Invalidator
	var mu sync.Mutex
	var drawn lightboxFrame

	entry := TranscriptEntry{
		Key: "u", Kind: ChatNodeUser, Seq: 1, Turn: 0, Visibility: ChatVisibilityVisible,
		Data: TranscriptData{
			Text:        "看这张图",
			Attachments: []TranscriptAttachment{{Image: true, ID: "att-shot", Name: "shot.png"}},
		},
	}
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme, Tree: tree, Root: "attach-lightbox-headless"})
	config := app.Config{
		Title: "attach-lightbox-headless", Width: width, Height: height,
		Tree: tree, WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, current ui.Size) {
		bounds := gfx.Rect{Width: current.Width, Height: current.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds, MainWidth: bounds.Width,
			Act: pluginapi.Actions{
				AttachmentGet: func(id string) (pluginapi.AttachmentBlob, bool) {
					if id != "att-shot" {
						return pluginapi.AttachmentBlob{}, false
					}
					return pluginapi.AttachmentBlob{ID: id, Name: "shot.png",
						MIME: "image/png", Bytes: body}, true
				},
				Invalidate: func() { invalidator.Invalidate() },
			},
		}
		page.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).FillWidth().FillHeight().Children(func() {
				// 顶部一个文本框：先让它握焦点，再点图开灯箱——官方打开时会把焦点
				// 挪到关闭按钮（`closeRef.current?.focus()`），不先握焦点就验不出来。
				ui.TextFieldID(f, fieldID, &input, "输入", "").Width(220).Height(28)
				view.Draw(f, env, TranscriptViewport{HostScroll: true}, []TranscriptEntry{entry})
			})
			plugin.drawImageLightbox(f, env)
		})
		mu.Lock()
		drawn = lightboxFrame{
			images: append([]gfx.Image(nil), canvas.Images()...),
			hits:   append([]gfx.Hit(nil), canvas.Hits()...),
		}
		mu.Unlock()
	}
	config.OnKey = func(ev gfx.KeyEvent) gfx.EventResult {
		if ev.Release {
			return gfx.EventResult{}
		}
		return page.Key(ev)
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
	snapshot := func() lightboxFrame {
		mu.Lock()
		defer mu.Unlock()
		return drawn
	}
	p := &lightboxPixels{t: t, session: session, pump: pump, frame: snapshot, fieldID: fieldID}

	deadline := time.Now().Add(5 * time.Second)
	for {
		pump()
		state, _ := view.messageImages.lookup("att-shot", false)
		if state.state == messageImageReady {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("等附件图解码超时，状态 = %v", state.state)
		}
		time.Sleep(5 * time.Millisecond)
	}
	pump()
	pump()
	return p, view
}

// TestAttachmentLightboxPixels 是附件图灯箱的真窗口回归（反向 + 原图比例 + 真像素）。
func TestAttachmentLightboxPixels(t *testing.T) {
	const width, height = 900, 600
	p, view := openAttachmentLightboxWindow(t)

	// ── 反向：没人点图 ⇒ 底色没被压暗、没有遮罩命中区、只有缩略图那一张位图 ──
	before := p.frame()
	if len(before.images) != 1 {
		t.Fatalf("没点图时屏上 %d 张位图，期望 1 张（缩略图）", len(before.images))
	}
	if _, ok := hitInFrame(before, ImageLightboxDialog); ok {
		t.Fatal("没点图就有灯箱遮罩命中区")
	}
	button, ok := hitInFrame(before, messageImageOpenLabel("shot.png"))
	if !ok {
		t.Fatalf("附件图没有「查看原图」按钮；本帧命中区：%v", frameLabels(before))
	}
	picture, err := p.session.Image()
	if err != nil {
		t.Fatal(err)
	}
	const cornerX, cornerY = width - 5, height - 5
	wantR, wantG, wantB := colorAt(picture, cornerX, cornerY)
	if wantR == 0 && wantG == 0 && wantB == 0 {
		t.Fatal("底色是纯黑，\"压暗\"判据在这套主题上不成立（判据太松）")
	}

	// 先把焦点放进输入框（产品里 composer 基本总是握着焦点）。
	if err := p.session.Click(100, 14); err != nil {
		t.Fatalf("点输入框失败：%v", err)
	}
	p.pump()
	if got := p.session.Snapshot().Focused; got != p.fieldID {
		t.Fatalf("点输入框之后焦点 = %v，期望 %v", got, p.fieldID)
	}

	// ── 点附件图 ⇒ 灯箱出现 ─────────────────────────────────────────────
	if err := p.session.Click(button.Bounds.X+button.Bounds.Width/2,
		button.Bounds.Y+button.Bounds.Height/2); err != nil {
		t.Fatalf("点附件图失败：%v", err)
	}
	p.pump()
	opened := p.frame()
	if len(opened.images) != 2 {
		t.Fatalf("点开之后 %d 张位图，期望 2 张（缩略图 + 灯箱）", len(opened.images))
	}
	lens := opened.images[len(opened.images)-1]
	// 4000×600、窗口 900×600、stage padding 40 ⇒ 内容盒 820×520；
	// `min(100%,1600px)` = 820；等比 ⇒ 820×123（**不是**裁切结果的 820×205）。
	wantWidth, wantHeight := lightboxImageFit(gfx.Bitmap{Width: 4000, Height: 600}, gfx.Rect{Width: width, Height: height})
	if lens.Bounds.Width != wantWidth || lens.Bounds.Height != wantHeight {
		t.Fatalf("灯箱图显示框 = %v×%v，期望 %v×%v（官方 max-width min(100%%,1600px) + contain）",
			lens.Bounds.Width, lens.Bounds.Height, wantWidth, wantHeight)
	}
	if lens.Bounds.Height == 205 {
		t.Fatalf("灯箱图是 4:1（820×205）—— 放大的是 cover 裁切后的缩略图，不是原图")
	}
	wantLens := gfx.Rect{X: (width - wantWidth) / 2, Y: (height - wantHeight) / 2,
		Width: wantWidth, Height: wantHeight}
	if lens.Bounds != wantLens {
		t.Fatalf("灯箱图落点 = %v，期望居中 %v", lens.Bounds, wantLens)
	}
	// 位图本身必须是 4000×600 的原图，不是 2400×600 的裁切。
	if lens.Bitmap.Width != 4000 || lens.Bitmap.Height != 600 {
		t.Fatalf("灯箱位图 = %d×%d，期望 4000×600 原图（裁切结果是 2400×600）",
			lens.Bitmap.Width, lens.Bitmap.Height)
	}
	closeHit, ok := hitInFrame(opened, ImageLightboxClose)
	if !ok {
		t.Fatalf("灯箱里没有关闭按钮；本帧命中区：%v", frameLabels(opened))
	}
	if got := p.session.Snapshot().Focused; got != closeHit.ID {
		t.Fatalf("灯箱打开后焦点 = %v，期望关闭按钮 %v（官方 closeRef.current?.focus()）", got, closeHit.ID)
	}

	picture, err = p.session.Image()
	if err != nil {
		t.Fatal(err)
	}
	// 遮罩：底角被压暗（三个通道同时变小，排除"恰好撞色"）。
	nr, ng, nb := colorAt(picture, cornerX, cornerY)
	if !(nr < wantR && ng < wantG && nb < wantB) {
		t.Fatalf("点了附件图但底角像素没被遮罩压暗：(before %d,%d,%d → after %d,%d,%d)",
			wantR, wantG, wantB, nr, ng, nb)
	}
	// 灯箱那张图真的上屏：盒子里 ≥99% 是夹具色。
	box := image.Rect(int(lens.Bounds.X), int(lens.Bounds.Y),
		int(lens.Bounds.X+lens.Bounds.Width), int(lens.Bounds.Y+lens.Bounds.Height))
	landed := countFixturePixels(picture, box)
	total := box.Dx() * box.Dy()
	// 圆角那四个角不是图（radius 16 由 `ui.Image` 自己去角），所以整盒的期望值
	// 按"内缩 radius 之后必须满覆盖 + 整盒按圆角损失放宽"两段钉：
	// 内缩那块 = 图的主体，必须几乎满；整盒 = 主体 + 四个圆角缺口。
	inset := box.Inset(int(ImageLightboxImageRadius))
	insideLanded := countFixturePixels(picture, inset)
	insideTotal := inset.Dx() * inset.Dy()
	if insideTotal > 0 && insideLanded < insideTotal*999/1000 {
		t.Fatalf("灯箱图内圈 %v 里只有 %d/%d 像素是夹具色，期望 ≥99.9%%",
			inset, insideLanded, insideTotal)
	}
	// 整盒的允许缺口是**几何推出来的**，不是拍的：
	//	圆角    四个 radius=16 的角 ≈ 4*(16²-π·16²/4) ≈ 220 px
	//	半像素  居中把 123 高放进 600 ⇒ top = 238.5（不是整数），上下各有**半覆盖**
	//	        的一行 ⇒ 最多 2*820 = 1640 px
	//	抗锯齿  再留 400 px 余量
	// 合计上限 ≈ 2260；超过就说明不是"圆角+半像素"，而是图没铺满。
	allowed := 2*box.Dx() + 400
	if landed < total-allowed {
		t.Fatalf("灯箱图落点 %v 里只有 %d/%d 像素是夹具色（缺口 %d > 几何上界 %d）",
			box, landed, total, total-landed, allowed)
	}
	t.Logf("ATTACH_LIGHTBOX_GAP total=%d landed=%d missing=%d allowed=%d", total, landed, total-landed, allowed)
	t.Logf("ATTACH_LIGHTBOX_PIXELS box=%v landed=%d/%d corner=%d,%d,%d", box, landed, total, nr, ng, nb)

	captureLightbox(t, p.session, width, height, "attach-open")

	// ── Esc 关掉之后回到"底色 + 一张位图" ─────────────────────────────
	if err := p.session.Keys("escape"); err != nil {
		t.Fatalf("按 Esc 失败：%v", err)
	}
	p.pump()
	closed := p.frame()
	if len(closed.images) != 1 {
		t.Fatalf("Esc 之后 %d 张位图，期望 1 张", len(closed.images))
	}
	if _, ok := hitInFrame(closed, ImageLightboxDialog); ok {
		t.Fatal("Esc 之后灯箱遮罩还在")
	}
	if view.imageLightboxOpen() {
		t.Fatal("Esc 之后灯箱状态还是开的")
	}
	picture, err = p.session.Image()
	if err != nil {
		t.Fatal(err)
	}
	if r, g, b := colorAt(picture, cornerX, cornerY); r != wantR || g != wantG || b != wantB {
		t.Fatalf("Esc 之后底角像素 = %d,%d,%d，期望回到底色 %d,%d,%d", r, g, b, wantR, wantG, wantB)
	}
}
