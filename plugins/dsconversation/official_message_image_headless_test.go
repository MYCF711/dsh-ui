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
// # 像素级判据（2026-11-20 复核：旧的"没上屏"是**测量假象**，不是渲染缺陷）
//
// 旧登记说"同一条 op 在 transcript 的座位/滚动链里整屏扫 0 个同色像素"。复核结论：
// **它上屏了**，而且是满覆盖。三条依据：
//
//	① 图元自己的值：Bounds={628.50,66.00,240.00,180.00}、ClipOf={628.00,66.00,240.00,180.00}、
//	   HasClip=true —— 两者相交正好是 240×180，`appendImage` 的三条丢弃路径
//	   （尺寸为零 / clip 不相交 / DevicePixels 后归零，见依赖里的
//	   `viva@v0.6.7-ole/internal/framebuild/framebuild.go:342` 与其调用的
//	   `intersectVisible` `:526`）一条都不成立。
//	② 像素：换成**纯色**夹具后，该 Bounds 里 43200/43200 像素就是这个颜色（全覆盖）。
//	③ 真窗口：另开的真窗口程序（`tmp/GO8-RC5/_imgsurface/winshot`）画同一条链，
//	   `scripts/shot.py` 截屏 + `scripts/visual-diff.py` 量图片矩形，主色占比 1.0000
//	   （43200 px），与"不传附件"的对照图差 100%，而同一张图别处差异 0 像素。
//
// 旧测量错在哪：旧夹具 `solidPNG` 是**渐变**（R=x%256、G=y%256、B=0x40）。400×300 线性
// 缩放成 240×180 之后，每个输出像素都是相邻源像素的混合，**几乎没有输出像素等于任何
// 源像素的颜色**——实测 43200 个输出像素里只有 360 个（0.83%）能命中源色。所以
// "数同色像素"这种取色方式在这个夹具上永远数不出 240×180，跟渲染无关。
// 判据改成纯色夹具 + 严格相等像素计数，并对 loading 帧反向断言（0 个），
// 这样"判据本身能不能红"也是被钉住的。
// 截图仍按 `WPC2_CAPTURE_DIR` 落盘，供人工复核。

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
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

// messageImageFixtureColor 是本文件那张夹具图的颜色。
//
// 为什么必须是**纯色**：像素判据要"严格相等"才站得住（见文件头"测量假象"那段）。
// 选 #11EE33 而不是主题里的任何令牌色：它不可能被背景 / 描边 / hover 顺带画出来，
// 屏上出现它就只可能是这张图。
var messageImageFixtureColor = color.RGBA{R: 0x11, G: 0xEE, B: 0x33, A: 0xFF}

// solidColorPNG 编一张纯色真 PNG（宽×高），返回字节。
//
// 与 `solidPNG`（渐变，用于几何三态那组）并存：这一个是给**像素判据**用的，
// 静置位图经 `imgdecode` 解码后逐像素就是同一个 RGBA，缩放也不会混色。
func solidColorPNG(t *testing.T, width, height int, fill color.RGBA) []byte {
	t.Helper()
	source := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			source.Set(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, source); err != nil {
		t.Fatalf("编纯色 PNG 失败：%v", err)
	}
	return buf.Bytes()
}

// countFixturePixels 数 picture 在 box 里与 `messageImageFixtureColor` **完全相等**的像素。
//
// 严格相等是刻意的：这条判据要回答的是"图真的上屏了、而且没被别的图层盖掉"，
// 容差只会把"底色/占位"也算成命中。抗锯齿达不到 240×180 的满覆盖，
// 所以门槛留了 200 像素的余量（240×180 = 43200）。
func countFixturePixels(picture *image.RGBA, box image.Rectangle) int {
	box = box.Intersect(picture.Bounds())
	count := 0
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			if picture.RGBAAt(x, y) == messageImageFixtureColor {
				count++
			}
		}
	}
	return count
}

// TestMessageImageLandsOneBitmapOp 是图片面的真窗口回归。
func TestMessageImageLandsOneBitmapOp(t *testing.T) {
	const width, height = 900, 600
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})

	body := solidColorPNG(t, 400, 300, messageImageFixtureColor) // 4:3，single 档 → 显示框 240×180
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
	// 反向断言同一取色判据：loading 帧屏上不该有这个颜色。
	// 这一条保证"数出 43200"不是判据太松（底色/描边不可能凑成 #11EE33）。
	loadingPicture, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	if got := countFixturePixels(loadingPicture, loadingPicture.Bounds()); got != 0 {
		t.Fatalf("loading 帧上已经有 %d 个图片色像素，期望 0（判据太松）", got)
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
	// 图元自己的裁剪账：clip 必须真的与 Bounds 相交，否则合成器会静默丢弃这条 op
	// （依赖里的 appendImage 三条丢弃路径）。这里把实际值打出来并钉住。
	t.Logf("IMAGE_OP Bounds={%.2f,%.2f,%.2f,%.2f} ClipOf={%.2f,%.2f,%.2f,%.2f} HasClip=%v Layer=%d Order=%d",
		img.Bounds.X, img.Bounds.Y, img.Bounds.Width, img.Bounds.Height,
		img.ClipOf.X, img.ClipOf.Y, img.ClipOf.Width, img.ClipOf.Height,
		img.HasClip, img.Layer, img.Order)
	if img.HasClip {
		left := maxFloat(img.Bounds.X, img.ClipOf.X)
		top := maxFloat(img.Bounds.Y, img.ClipOf.Y)
		right := minFloat(img.Bounds.X+img.Bounds.Width, img.ClipOf.X+img.ClipOf.Width)
		bottom := minFloat(img.Bounds.Y+img.Bounds.Height, img.ClipOf.Y+img.ClipOf.Height)
		if right <= left || bottom <= top {
			t.Fatalf("clip 与 Bounds 不相交，合成器会丢掉这条 op：Bounds=%v ClipOf=%v",
				img.Bounds, img.ClipOf)
		}
	}
	if session.HasText(MessageImageLoading) || session.HasText(MessageImageLoadFailed) {
		t.Fatal("图上屏之后不该还有 loading / 失败文案")
	}

	// 像素级：图元自己的 Bounds 里必须**真的是这张图**（不是底色、不是占位）。
	picture, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	box := image.Rect(int(img.Bounds.X), int(img.Bounds.Y),
		int(img.Bounds.X+img.Bounds.Width), int(img.Bounds.Y+img.Bounds.Height))
	landed := countFixturePixels(picture, box)
	total := box.Dx() * box.Dy()
	// 门槛是 99%：那条 op 的 Bounds.X 是 628.50 而 clip 从 628.00 起，左边缘会被裁掉
	// 半像素列（实测 43020/43200），所以"满覆盖"只能是近似；0 像素则一定红。
	if landed < total*99/100 {
		t.Fatalf("图片落点 %v 里只有 %d/%d 像素是图片色，期望 ≥99%%（真图真的上屏）",
			box, landed, total)
	}
	t.Logf("IMAGE_PIXELS box=%v landed=%d/%d", box, landed, total)

	captureMessageImage(t, session, width, height)
}

// maxFloat / minFloat 是裁剪账要用的两个小工具（不想为两行算式引 math）。
func maxFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
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

// twoTonePNG 编一张**非周期**的两色图：左半 left、右半 right，中间一条清晰分界。
//
// 为什么不能再只用纯色夹具：纯色能证明"图画上去了"，但**证明不了"只画了一份"**
// —— 把同一张纯色图平铺十次，像素统计与只画一次**完全一样**。
// 而原来的渐变夹具（R=x%256）又自带周期：源图 400 px 宽会自我重复 1.56 次，
// 于是"看起来在平铺"其实是夹具自己的取模图案，反过来会造假警报。
//
// 两色分段既非周期（左右各一段，不重复），又能让"分界"成为一个可数的判据：
// 平铺一次 → 分界 1 条；平铺三次 → 分界 3 条。纯色做不到这件事。
func twoTonePNG(t *testing.T, width, height int, left, right color.RGBA) []byte {
	t.Helper()
	source := image.NewRGBA(image.Rect(0, 0, width, height))
	half := width / 2
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if x < half {
				source.Set(x, y, left)
			} else {
				source.Set(x, y, right)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, source); err != nil {
		t.Fatalf("编 PNG 失败：%v", err)
	}
	return buf.Bytes()
}

// countColor 数 box 里与 want 完全相等的像素。
func countColor(picture *image.RGBA, box image.Rectangle, want color.RGBA) int {
	box = box.Intersect(picture.Bounds())
	count := 0
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			if picture.RGBAAt(x, y) == want {
				count++
			}
		}
	}
	return count
}

// TestMessageImageDrawsOneCopyNotTiled 证明图上屏是**一份**，不是平铺。
//
// 判据分两半，缺一不可：
//   - 左右两半各自是各自那一色（≥99%）→ 图真的上屏了，且左右关系没反；
//   - 沿显示框中线横着扫，两种颜色的**翻转次数恰好 1** → 只有一条分界。
//
// 平铺会把第二个判据变成 3、5、7…：这是纯色夹具永远抓不到的一类缺陷。
func TestMessageImageDrawsOneCopyNotTiled(t *testing.T) {
	const width, height = 900, 600
	left := color.RGBA{R: 0xE0, G: 0x20, B: 0x20, A: 0xFF}
	right := color.RGBA{R: 0x20, G: 0x40, B: 0xE0, A: 0xFF}
	body := twoTonePNG(t, 400, 300, left, right)

	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	get := func(id string) (pluginapi.AttachmentBlob, bool) {
		if id != "att-tile" {
			return pluginapi.AttachmentBlob{}, false
		}
		return pluginapi.AttachmentBlob{ID: id, Name: "tile.png", MIME: "image/png",
			Width: 400, Height: 300, Bytes: body}, true
	}

	tree := &gfx.Tree{}
	feat := tree.Feature("message-image-tile")
	var invalidator app.Invalidator
	var mu sync.Mutex
	var drawn []gfx.Image

	entry := TranscriptEntry{
		Key: "u", Kind: ChatNodeUser, Seq: 1, Turn: 0, Visibility: ChatVisibilityVisible,
		Data: TranscriptData{
			Text:        "看这张图",
			Attachments: []TranscriptAttachment{{Image: true, ID: "att-tile", Name: "tile.png"}},
		},
	}
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{
		Title: "message-image-tile", Width: width, Height: height,
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

	deadline := time.Now().Add(5 * time.Second)
	for {
		pump()
		if snapshot, _ := view.messageImages.lookup("att-tile", false); snapshot.state == messageImageReady {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("等图片解码超时")
		}
		time.Sleep(5 * time.Millisecond)
	}
	pump()

	mu.Lock()
	images := drawn
	mu.Unlock()
	if len(images) != 1 {
		t.Fatalf("屏上图元 %d 张，期望 1 张", len(images))
	}
	img := images[0]
	box := image.Rect(int(img.Bounds.X), int(img.Bounds.Y),
		int(img.Bounds.X+img.Bounds.Width), int(img.Bounds.Y+img.Bounds.Height))
	if box.Dx() != 240 || box.Dy() != 180 {
		t.Fatalf("显示框 = %v×%v，期望 240×180", box.Dx(), box.Dy())
	}

	picture, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}

	// 判据一：左右两半各自是各自那一色。左边缘被裁掉半像素列，所以留 1% 余量。
	mid := box.Min.X + box.Dx()/2
	leftBox := image.Rect(box.Min.X, box.Min.Y, mid, box.Max.Y)
	rightBox := image.Rect(mid, box.Min.Y, box.Max.X, box.Max.Y)
	leftTotal, rightTotal := leftBox.Dx()*leftBox.Dy(), rightBox.Dx()*rightBox.Dy()
	if got := countColor(picture, leftBox, left); got < leftTotal*99/100 {
		t.Fatalf("左半区只有 %d/%d 像素是左色 %v，期望 ≥99%%", got, leftTotal, left)
	}
	if got := countColor(picture, rightBox, right); got < rightTotal*99/100 {
		t.Fatalf("右半区只有 %d/%d 像素是右色 %v，期望 ≥99%%", got, rightTotal, right)
	}

	// 判据二：沿中线横着扫，两种颜色只翻转一次。平铺会让它变成 3、5、7…
	row := box.Min.Y + box.Dy()/2
	transitions := 0
	previous := color.RGBA{}
	started := false
	for x := box.Min.X; x < box.Max.X; x++ {
		at := picture.RGBAAt(x, row)
		if at != left && at != right {
			continue // 边界抗锯齿像素不参与翻转计数
		}
		if started && at != previous {
			transitions++
		}
		previous, started = at, true
	}
	if transitions != 1 {
		t.Fatalf("沿中线颜色翻转 %d 次，期望恰好 1 次（分界只有一条）—— >1 说明图被平铺了", transitions)
	}
	t.Logf("IMAGE_ONE_COPY box=%v transitions=%d", box, transitions)

	captureMessageImage(t, session, width, height)
}
