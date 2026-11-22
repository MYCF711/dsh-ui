//go:build darwin || windows || linux

package dsconversation

// official_image_lightbox_headless_test.go —— 正文图灯箱的**真窗口像素**回归（`#18` (a)）。
//
// 与 `official_image_lightbox_test.go` 的分工：那边是纯画布（图元账 / 命中区 /
// 几何算式 / 关闭状态机），这边开一个真 headless 窗口跑完整绘制链
// （Page → transcript → 合成 → 真像素），钉三件事：
//
//	① **反向**：没点图时窗口右下角那个像素就是底色本身——灯箱那层遮罩**没有**
//	   出现（判据是像素相等，不是"没找到某个结构"）；同时只有正文那一张位图。
//	② 点正文图之后：遮罩真的压暗了那个像素；灯箱里的图是官方那一套
//	   `min(100%,1600px) × 100%` 等比尺寸（900×600 窗口、1200×600 原图 ⇒ 820×410），
//	   而且那个盒子里 ≥99% 像素是夹具色（真图真的上屏）；关闭按钮那块 36×36
//	   里 ≥40% 是它的 `--dsw-specific-input-major` 底。
//	③ 三条关闭路径都回到"底色 + 一张位图"：Esc（页面级 `ui.Shortcut`，官方
//	   capture 阶段的等价物）、点遮罩、点关闭按钮。
//
// 夹具是**纯色**：像素判据要"严格相等/相等计数"，渐变图缩放后几乎没有输出像素
// 等于任何源像素色——那条测量假象写在 `official_message_image_headless_test.go`
// 的文件头，这里不重复。

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

// lightboxFrame 是某一帧画完之后读得出来的东西（图元 + 命中区）。
type lightboxFrame struct {
	images []gfx.Image
	hits   []gfx.Hit
}

// lightboxPixels 在真窗口里跑一遍「点开灯箱 / 关掉灯箱」的全过程。
type lightboxPixels struct {
	t       *testing.T
	session *headless.Session
	pump    func()
	frame   func() lightboxFrame
	// fieldID 是页面顶部那个文本框的交互 id：用来钉「灯箱打开时焦点必须
	// 从输入框移到关闭按钮」（官方 closeRef.current?.focus()），否则输入框
	// 握着焦点时页面级 Esc 会被 `p.typing(event.Target)` 挡掉。
	fieldID gfx.ID
}

// localNear 允许 ±2 的量化误差（gfx.Color 是 float32，落到 8 位像素会舍入）。
func localNear(value, want uint32) bool {
	d := int(value) - int(want)
	return d >= -2 && d <= 2
}

// colorAt 把窗口里某个像素读成 8 位三元组。
func colorAt(picture *image.RGBA, x, y int) (uint32, uint32, uint32) {
	r, g, b, _ := picture.At(x, y).RGBA()
	return r >> 8, g >> 8, b >> 8
}

// countColorPixels 数 box 里与 want **相等（±2）** 的像素。
func countColorPixels(picture *image.RGBA, box image.Rectangle, want gfx.Color) int {
	wr := uint32(want.Red*255 + 0.5)
	wg := uint32(want.Green*255 + 0.5)
	wb := uint32(want.Blue*255 + 0.5)
	count := 0
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			r, g, b, _ := picture.At(x, y).RGBA()
			if localNear(r>>8, wr) && localNear(g>>8, wg) && localNear(b>>8, wb) {
				count++
			}
		}
	}
	return count
}

// openLightboxWindow 开一个 900×600 的真 headless 窗口，正文里有一张 1200×600 的图。
//
// 1200×600 是刻意的：正文里它被 `max-width:min(100%,640px)` 压成 640×320，
// 灯箱里被 `min(100%,1600px)` 压成 820×410 —— 两处尺寸不同，判据才能区分
// "开的是灯箱"还是"只是正文那张图"。
func openLightboxWindow(t *testing.T) *lightboxPixels {
	t.Helper()
	const width, height = 900, 600
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	plugin := &Plugin{officialTranscript: view}

	body := solidColorPNG(t, 1200, 600, messageImageFixtureColor)
	markdownImage := func(_, _, destination string) (pluginapi.MarkdownImageBytes, bool) {
		if destination != "./shot.png" {
			return pluginapi.MarkdownImageBytes{}, false
		}
		return pluginapi.MarkdownImageBytes{Bytes: body, MIME: "image/png"}, true
	}

	tree := &gfx.Tree{}
	feat := tree.Feature("image-lightbox-headless")
	fieldID := tree.ID(feat, "test-input")
	var input ui.Field
	var invalidator app.Invalidator
	var mu sync.Mutex
	var drawn lightboxFrame

	entry := TranscriptEntry{
		Key: "a-image-lightbox", Kind: ChatNodeAssistantStep, Seq: 1, Turn: 1, Visibility: ChatVisibilityVisible,
		Data: TranscriptData{Text: "看图\n\n![示意图](./shot.png)"},
	}
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme, Tree: tree, Root: "image-lightbox-headless"})
	config := app.Config{
		Title: "image-lightbox-headless", Width: width, Height: height,
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
			ui.Column(f).FillWidth().FillHeight().Children(func() {
				// 顶部一个文本框：先让它拿到焦点，再点图开灯箱——官方打开灯箱时
				// 会把焦点挪到关闭按钮（`closeRef.current?.focus()`）。
				ui.TextFieldID(f, fieldID, &input, "输入", "").Width(220).Height(28)
				view.Draw(f, env, TranscriptViewport{HostScroll: true}, []TranscriptEntry{entry})
			})
			// `shell.overlay` 的灯箱座位（Order 110）永远最后画。
			plugin.drawImageLightbox(f, env)
		})
		view.PaintCodeBlocks(canvas, env)
		mu.Lock()
		drawn = lightboxFrame{
			images: append([]gfx.Image(nil), canvas.Images()...),
			hits:   append([]gfx.Hit(nil), canvas.Hits()...),
		}
		mu.Unlock()
	}
	// 与真实壳同一条路由：`cfg.OnKey` 先把键送页面（页面级 Shortcut 唯一的驱动源）。
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

	// 等后台取数 + 解码落库（不赌调度）。
	deadline := time.Now().Add(5 * time.Second)
	for {
		pump()
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
	return p
}

// TestImageLightboxPixels 是灯箱的真窗口回归（正向 + 反向 + 三条关闭路径）。
func TestImageLightboxPixels(t *testing.T) {
	const width, height = 900, 600
	p := openLightboxWindow(t)

	// ── 反向：没人点图 ⇒ 没有遮罩那一层像素，也没有第二张位图 ──────────────
	before := p.frame()
	if len(before.images) != 1 {
		t.Fatalf("没点图时屏上 %d 张位图，期望 1 张（正文那张）", len(before.images))
	}
	button, ok := hitInFrame(before, markdownImageOpenLabel("示意图"))
	if !ok {
		t.Fatalf("正文图没有「查看大图」按钮；本帧命中区：%v", frameLabels(before))
	}
	picture, err := p.session.Image()
	if err != nil {
		t.Fatal(err)
	}
	// 右下角（远离正文、也远离图）是"底色本身"——灯箱的遮罩一旦出现就会压暗它。
	const cornerX, cornerY = width - 5, height - 5
	wantR, wantG, wantB := colorAt(picture, cornerX, cornerY)
	if wantR == 0 && wantG == 0 && wantB == 0 {
		t.Fatal("底色是纯黑，\"压暗\"判据在这套主题上不成立（判据太松）")
	}
	if _, ok := hitInFrame(before, ImageLightboxDialog); ok {
		t.Fatal("没点图就有灯箱遮罩命中区")
	}

	// 先把焦点放进输入框：产品里 composer 基本总是握着焦点。
	if err := p.session.Click(100, 14); err != nil {
		t.Fatalf("点输入框失败：%v", err)
	}
	p.pump()
	if got := p.session.Snapshot().Focused; got != p.fieldID {
		t.Fatalf("点输入框之后焦点 = %v，期望 %v", got, p.fieldID)
	}

	// ── 点正文图 ⇒ 灯箱出现（并且焦点移到关闭按钮）─────────────────────
	if err := p.session.Click(button.Bounds.X+button.Bounds.Width/2, button.Bounds.Y+button.Bounds.Height/2); err != nil {
		t.Fatalf("点正文图失败：%v", err)
	}
	p.pump()
	opened := p.frame()
	if len(opened.images) != 2 {
		t.Fatalf("点开之后 %d 张位图，期望 2 张（正文 + 灯箱）", len(opened.images))
	}
	lens := opened.images[len(opened.images)-1]
	// 官方 `._image_19zw2_26{max-width:min(100%,1600px);max-height:100%}` +
	// `._stage_19zw2_9{padding:40px}`：900×600 窗口、1200×600 原图 ⇒ 820×410 居中。
	if lens.Bounds.Width != 820 || lens.Bounds.Height != 410 {
		t.Fatalf("灯箱图显示框 = %v，期望 820×410", lens.Bounds)
	}
	wantLens := gfx.Rect{X: (width - 820) / 2, Y: (height - 410) / 2, Width: 820, Height: 410}
	if lens.Bounds != wantLens {
		t.Fatalf("灯箱图落点 = %v，期望居中 %v", lens.Bounds, wantLens)
	}
	closeHit, ok := hitInFrame(opened, ImageLightboxClose)
	if !ok {
		t.Fatalf("灯箱里没有关闭按钮；本帧命中区：%v", frameLabels(opened))
	}
	// 官方打开时 `closeRef.current?.focus()`：焦点必须从输入框挪到关闭按钮，
	// 否则输入框握着焦点时页面级 Esc 会被 `p.typing(event.Target)` 挡掉。
	if got := p.session.Snapshot().Focused; got != closeHit.ID {
		t.Fatalf("灯箱打开后焦点 = %v，期望关闭按钮 %v（官方 closeRef.current?.focus()）", got, closeHit.ID)
	}

	picture, err = p.session.Image()
	if err != nil {
		t.Fatal(err)
	}
	// 遮罩：底角被压暗（旧值 R,G,B 都变小），且不是巧合（三个通道同时变）。
	nr, ng, nb := colorAt(picture, cornerX, cornerY)
	if !(nr < wantR && ng < wantG && nb < wantB) {
		t.Fatalf("点了图但底角像素没被遮罩压暗：(before %d,%d,%d → after %d,%d,%d)",
			wantR, wantG, wantB, nr, ng, nb)
	}
	// 灯箱那张图真的上屏：盒子里 ≥99% 是夹具色。
	box := image.Rect(int(lens.Bounds.X), int(lens.Bounds.Y),
		int(lens.Bounds.X+lens.Bounds.Width), int(lens.Bounds.Y+lens.Bounds.Height))
	landed := countFixturePixels(picture, box)
	total := box.Dx() * box.Dy()
	if landed < total*99/100 {
		t.Fatalf("灯箱图落点 %v 里只有 %d/%d 像素是夹具色，期望 ≥99%%", box, landed, total)
	}
	t.Logf("LIGHTBOX_PIXELS box=%v landed=%d/%d corner=%d,%d,%d", box, landed, total, nr, ng, nb)

	// 关闭按钮那块 36×36：≥40% 是 `--dsw-specific-input-major` 底
	// （圆角与 16px 关闭叉各占一部分，所以不能要求满覆盖）。
	theme := dscore.Dark()
	closeBox := image.Rect(width-56, 20, width-20, 56)
	filled := countColorPixels(picture, closeBox, theme.InputMajor)
	if filled < 36*36*2/5 {
		t.Fatalf("关闭按钮 %v 里只有 %d 像素是 input-major 底，期望 ≥%d", closeBox, filled, 36*36*2/5)
	}
	t.Logf("LIGHTBOX_CLOSE_PIXELS box=%v filled=%d", closeBox, filled)

	// Tab 锁在这枚按钮上（官方 `d.key === "Tab"` → preventDefault + closeRef.focus()）。
	// 灯箱里没有别的可聚焦元素，焦点一旦跑掉就说明焦点陷阱没生效。
	if err := p.session.Keys("tab"); err != nil {
		t.Fatalf("按 Tab 失败：%v", err)
	}
	p.pump()
	if got := p.session.Snapshot().Focused; got != closeHit.ID {
		t.Fatalf("Tab 之后焦点 = %v，期望仍锁在关闭按钮 %v（官方 preventDefault + focus(close)）", got, closeHit.ID)
	}
	if len(p.frame().images) != 2 {
		t.Fatal("按 Tab 把灯箱关掉了")
	}

	captureLightbox(t, p.session, width, height, "open")

	// ── 关闭路径 1：Esc（页面级 Shortcut，官方 capture 阶段的等价物）────────
	if err := p.session.Keys("escape"); err != nil {
		t.Fatalf("按 Esc 失败：%v", err)
	}
	p.pump()
	assertLightboxClosed(t, p, width, height, "Esc", wantR, wantG, wantB)

	// ── 关闭路径 2：点遮罩（官方 `._mask` 的 onMouseDown）───────────────────
	reopen := func(what string) {
		t.Helper()
		button, ok := hitInFrame(p.frame(), markdownImageOpenLabel("示意图"))
		if !ok {
			t.Fatalf("%s：关掉之后正文图按钮不见了", what)
		}
		if err := p.session.Click(button.Bounds.X+button.Bounds.Width/2, button.Bounds.Y+button.Bounds.Height/2); err != nil {
			t.Fatalf("%s：重新点开灯箱失败：%v", what, err)
		}
		p.pump()
		if len(p.frame().images) != 2 {
			t.Fatalf("%s：重新点开之后位图不是 2 张", what)
		}
	}
	reopen("遮罩")
	if err := p.session.Click(5, height-5); err != nil {
		t.Fatalf("点遮罩失败：%v", err)
	}
	p.pump()
	assertLightboxClosed(t, p, width, height, "点遮罩", wantR, wantG, wantB)

	// ── 关闭路径 3：点关闭按钮 ───────────────────────────────────────────
	reopen("关闭按钮")
	if err := p.session.Click(width-38, 38); err != nil {
		t.Fatalf("点关闭按钮失败：%v", err)
	}
	p.pump()
	assertLightboxClosed(t, p, width, height, "点关闭按钮", wantR, wantG, wantB)
}

// hitInFrame 在某一帧的命中区里按可访问名找一块。
func hitInFrame(frame lightboxFrame, label string) (gfx.Hit, bool) {
	for _, hit := range frame.hits {
		if hit.Label == label {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// frameLabels 是整帧命中区名字（失败诊断用）。
func frameLabels(frame lightboxFrame) []string {
	out := make([]string, 0, len(frame.hits))
	for _, hit := range frame.hits {
		out = append(out, hit.Label)
	}
	return out
}

// assertLightboxClosed 钉「关掉之后回到点开前的样子」：一张位图 + 底角回到原色。
func assertLightboxClosed(t *testing.T, p *lightboxPixels, width, height int, how string,
	wantR, wantG, wantB uint32) {
	t.Helper()
	frame := p.frame()
	if len(frame.images) != 1 {
		t.Fatalf("%s 之后屏上 %d 张位图，期望 1 张（灯箱图必须消失）", how, len(frame.images))
	}
	for _, label := range []string{ImageLightboxDialog, ImageLightboxClose} {
		if _, ok := hitInFrame(frame, label); ok {
			t.Fatalf("%s 之后灯箱命中区 %q 还在", how, label)
		}
	}
	picture, err := p.session.Image()
	if err != nil {
		t.Fatal(err)
	}
	r, g, b := colorAt(picture, width-5, height-5)
	if !localNear(r, wantR) || !localNear(g, wantG) || !localNear(b, wantB) {
		t.Fatalf("%s 之后底角像素 = %d,%d,%d，期望回到 %d,%d,%d（遮罩没撤掉）",
			how, r, g, b, wantR, wantG, wantB)
	}
	captureLightbox(t, p.session, width, height, "closed")
}

// captureLightbox 落一张截图供人工复核（不设 WPC2_CAPTURE_DIR 就落临时目录）。
func captureLightbox(t *testing.T, session *headless.Session, width, height int, state string) {
	t.Helper()
	dir := os.Getenv("WPC2_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("image-lightbox-%s-%dx%d.png", state, width, height))
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Fatalf("截图没落盘或为空：%v", err)
	}
	t.Logf("CAPTURE %s", path)
}
