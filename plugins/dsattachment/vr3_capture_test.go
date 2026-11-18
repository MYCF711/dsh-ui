//go:build darwin || windows || linux

// vr3_capture_test.go —— 第三轮真实渲染审计：附件 rail 的** hover 显隐 / focus-visible /
// 上传中 / 失败态 / 溢出翻页**。
//
// 上一轮侧栏栽在"该隐藏的东西常显"。附件卡上正有一个同形状的开关：
// 官方 `FileCard.module.css`（rc.2 asar 字节偏移 28546292 起的 `css$2` 常量）里
//
//	.EEnIVa_remove{…width:18px;height:18px;…opacity:0;…top:6px;right:6px}
//	.EEnIVa_remove:focus-visible{opacity:1}
//	@media (hover:hover){.EEnIVa_card:hover .EEnIVa_remove{opacity:1}}
//	.EEnIVa_removeFailed{background:var(--dsw-alias-state-error-primary);
//	                     color:var(--dsw-alias-label-on-color);opacity:1}
//
// 也就是**三条独立路径**：普通态 0、卡 hover 才 1、按钮自身键盘 focus-visible 也 1、
// 失败态常显。这一轮三条都用**真指针 / 真 Tab** 各验一遍。
//
// 另外两条：`.EEnIVa_progressTrack{height:2px;position:absolute;bottom:5px;
// left:12px;right:12px}` + `.EEnIVa_progressBar{width:35%}`（同段 CSS），
// 以及 rail 放不下时的翻页箭头（`AttachmentRail.module.css` rc.2 偏移 28536285
// 的 `.Onur9G_arrow{width:24px;height:24px;…top:50%;translateY(-50%)}`、`left:4px`/`right:4px`）。
//
// 截图落 `$VR3_CAPTURE_DIR`（未设置时用临时目录）。
package dsattachment

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func vr3CaptureDir(t *testing.T) string {
	t.Helper()
	base := os.Getenv("VR3_CAPTURE_DIR")
	if base == "" {
		return t.TempDir()
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	return base
}

// vr3Sink 是第三轮的假宿主：gate 非 nil 时 Put 卡住（把「上传中」钉在画面上），
// fail 非 nil 时每个 Put 都失败（失败态）。
type vr3Sink struct {
	mu       sync.Mutex
	puts     int
	gate     chan struct{}
	fail     error
	byteSize int64
}

func (s *vr3Sink) Put(_ context.Context, path string) (*apiclient.Attachment, error) {
	if s.gate != nil {
		<-s.gate
	}
	s.mu.Lock()
	s.puts++
	fail := s.fail
	size := s.byteSize
	s.mu.Unlock()
	if fail != nil {
		return nil, fail
	}
	if size == 0 {
		size = 1536
	}
	return &apiclient.Attachment{
		ID:       "vr3-" + baseName(path),
		Name:     baseName(path),
		ByteSize: size,
		MIME:     "text/plain",
	}, nil
}

func (*vr3Sink) Attach(context.Context, string, apiclient.Attachment) error { return nil }

func (*vr3Sink) Upload(context.Context, string, apiclient.Attachment) (*apiclient.FileUploadReceipt, error) {
	return &apiclient.FileUploadReceipt{ReceiptID: "vr3-receipt"}, nil
}

func (*vr3Sink) CurrentSession() string { return "s-vr3" }

// vr3Rail 是一台装配好的真窗口：uikernel + dscore + 本插件，只画
// `ui.input.attachments` 这一槽（官方 composer 的附件区），AfterDraw 照壳的顺序跑。
type vr3Rail struct {
	t       *testing.T
	session *headless.Session
	plugin  *Plugin
	dir     string
	width   int
	height  int
}

func vr3OpenRail(t *testing.T, dir string, width, height int, sink AttachmentSink, paths []string) *vr3Rail {
	t.Helper()
	kernel := uikernel.New("attachment-vr3")
	p := &Plugin{}
	// 同步 run：拖入/上传在同一线程落定，第一帧就能看到稳定态。
	p.run = func(fn func()) { fn() }
	if err := kernel.Load(&dscore.Plugin{Dark: true}, p); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	SetSink(sink)
	t.Cleanup(func() { SetSink(nil) })
	if len(paths) > 0 {
		// 走产品同一条入口（拖入 → addPaths → 每路径一次 Put），不直接改 p.items。
		if result := HandleFileDrop(gfx.FileDropEvent{Phase: gfx.FileDropDrop, Paths: paths}); !result.Handled {
			t.Fatalf("有 sink 时拖入应该被消费：%+v", result)
		}
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("attachment-vr3")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	rail := &vr3Rail{t: t, plugin: p, dir: dir, width: width, height: height}
	config := app.Config{
		Title: "attachment-vr3", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			Geometry: pluginapi.Geometry{Width: bounds.Width, Height: bounds.Height},
			Has:      func(slot pluginapi.Slot) bool { return kernel.Has(slot) },
			Act:      pluginapi.Actions{Invalidate: invalidator.Invalidate},
		}
		view.Draw(canvas, bounds, func(f ui.C) {
			kernel.DrawSlot(f, uikernel.OffInputAttachments, env)
		})
		for _, painter := range kernel.AllPainters() {
			if painter.AfterDraw != nil {
				painter.AfterDraw(canvas, env)
			}
		}
	}
	config.OnScroll = view.Wheel
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless.Open 失败：%v", err)
	}
	t.Cleanup(session.Close)
	rail.session = session
	// 两帧：afterDraw 要先用**上一帧**量到的 rail 宽算"一页几张"。
	rail.pump()
	rail.pump()
	return rail
}

func (r *vr3Rail) pump() {
	r.t.Helper()
	if err := r.session.Driver().Pump(); err != nil {
		r.t.Fatal(err)
	}
}

func (r *vr3Rail) shot(name string) *image.RGBA {
	r.t.Helper()
	path := filepath.Join(r.dir, name)
	if err := r.session.Screenshot(path); err != nil {
		r.t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		r.t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	img, err := r.session.Image()
	if err != nil {
		r.t.Fatal(err)
	}
	r.t.Logf("VR3_CAPTURE path=%s size=%v", path, img.Bounds())
	return img
}

// vr3PixelDiff 数两张图的差异像素。
func vr3PixelDiff(a, b *image.RGBA) int {
	if a.Bounds() != b.Bounds() {
		return -1
	}
	n := 0
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				n++
			}
		}
	}
	return n
}

// vr3InkInside 数 rect 里与 reference 颜色不同的像素 —— 用来判"这块区域有没有东西被画出来"。
func vr3InkInside(img *image.RGBA, rect gfx.Rect, reference gfx.Color) (ink int, samples int) {
	ref := vr3Pixel(reference)
	x0, y0 := int(rect.X)+1, int(rect.Y)+1
	x1, y1 := int(rect.X+rect.Width)-1, int(rect.Y+rect.Height)-1
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > img.Bounds().Max.X {
		x1 = img.Bounds().Max.X
	}
	if y1 > img.Bounds().Max.Y {
		y1 = img.Bounds().Max.Y
	}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			samples++
			pixel := img.RGBAAt(x, y)
			// 8 级容差：抗锯齿边缘不算"画了东西"。
			if abs8(int(pixel.R)-ref[0]) > 8 || abs8(int(pixel.G)-ref[1]) > 8 ||
				abs8(int(pixel.B)-ref[2]) > 8 || abs8(int(pixel.A)-ref[3]) > 8 {
				ink++
			}
		}
	}
	return ink, samples
}

func abs8(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// vr3Pixel 把主题色换算成 image.RGBA 里存的那 4 个字节。
//
// 渲染目标是**预乘**的（`headless/render_windows.go:61` "the target is premultiplied"），
// 所以每个通道都是 `值 × alpha`，再乘 255。
func vr3Pixel(c gfx.Color) [4]int {
	scale := func(v float32) int {
		scaled := float64(v) * float64(c.Alpha) * 255
		switch {
		case scaled < 0:
			return 0
		case scaled > 255:
			return 255
		}
		return int(scaled + 0.5)
	}
	alpha := int(float64(c.Alpha)*255 + 0.5)
	return [4]int{scale(c.Red), scale(c.Green), scale(c.Blue), alpha}
}

// vr3RemoveRect 返回第 i 张卡的删除按钮矩形（`.remove{top:6;right:6;18×18}` 的直接投影）。
func vr3RemoveRect(card gfx.Rect) gfx.Rect {
	return gfx.Rect{
		X: card.X + card.Width - removeRight - removeSize, Y: card.Y + removeTop,
		Width: removeSize, Height: removeSize,
	}
}

// vr3PrimitivesWithColor 找某个颜色、某一尺寸的矩形 primitive（判"这个盒子画没画"）。
func vr3PrimitivesWithColor(snapshot app.Snapshot, color gfx.Color) []app.Primitive {
	out := make([]app.Primitive, 0, 4)
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind == app.RectanglePrimitive && primitive.Color == color {
			out = append(out, primitive)
		}
	}
	return out
}

// ── ① 普通态 opactiy:0 / 卡 hover → 1 ───────────────────────────────────────

// TestVR3RemoveHiddenIdleShownOnCardHover 走真指针：卡外静止时删除按钮**一个像素都不许有**，
// 指针移到卡上之后必须出现。
//
// 官方依据（rc.2 asar 偏移 28546292）：`.EEnIVa_remove{…opacity:0…}` +
// `@media (hover:hover){.EEnIVa_card:hover .EEnIVa_remove{opacity:1}}`。
func TestVR3RemoveHiddenIdleShownOnCardHover(t *testing.T) {
	dir := vr3CaptureDir(t)
	rail := vr3OpenRail(t, dir, 1180, 240, &vr3Sink{}, []string{`D:\ws\a.txt`})
	theme := dscore.Dark()
	if len(rail.plugin.boxes.Cards) != 1 {
		t.Fatalf("这一帧画了 %d 张卡，want 1", len(rail.plugin.boxes.Cards))
	}
	card := rail.plugin.boxes.Cards[0].Rect()
	button := vr3RemoveRect(card)

	// 指针停在窗口左上角（卡外）。
	if err := rail.session.Move(4, 4); err != nil {
		t.Fatal(err)
	}
	rail.pump()
	idle := rail.shot("rail-remove-idle.png")
	idleInk, samples := vr3InkInside(idle, button, theme.InputMajor)
	if samples == 0 {
		t.Fatalf("取样区域为空：button=%+v", button)
	}
	if idleInk != 0 {
		t.Errorf("静止态删除按钮区域有 %d 个非卡片底色的像素（官方 .remove 普通态 opacity:0）", idleInk)
	}
	// 图元层面再确认一次：ButtonContrastFill 的那张 18×18 圆面不该以不透明形式出现。
	for _, primitive := range vr3PrimitivesWithColor(rail.session.Snapshot(), theme.ButtonContrastFill) {
		if primitive.Bounds == button && primitive.Color.Alpha > 0.01 {
			t.Errorf("静止态画出了不透明的删除按钮圆面：%+v alpha=%v", primitive.Bounds, primitive.Color.Alpha)
		}
	}

	// 真指针移到卡中央（卡 hover）。
	if err := rail.session.Move(card.X+card.Width/2, card.Y+card.Height/2); err != nil {
		t.Fatal(err)
	}
	rail.pump()
	hover := rail.shot("rail-remove-hover.png")
	hoverInk, _ := vr3InkInside(hover, button, theme.InputMajor)
	if hoverInk == 0 {
		t.Fatalf("卡 hover 之后删除按钮仍然一个像素都没有（官方 .card:hover .remove{opacity:1}）")
	}
	// hover 时标题右侧让出 18px（`.card:hover .name{padding-right:18px}`）：
	// 名字文本右缘必须比静止态左移条数 ≥ 名字原本就贴着右边的情况才可见；
	// 这里钉"按钮区域真的多出了 ink"，再钉名字未被裁掉。
	if !rail.session.HasText("a.txt") {
		t.Errorf("hover 后卡片标题不见了：texts=%q", rail.session.Texts())
	}
	t.Logf("删除按钮区域：静止 ink=%d，hover ink=%d（共 %d 像素）", idleInk, hoverInk, samples)
}

// ── ② 按钮自身 focus-visible → 1 ────────────────────────────────────────────

// TestVR3RemoveShownByOwnFocusVisible 用真 Tab 把焦点走到删除按钮上：
// 官方 `.EEnIVa_remove:focus-visible{opacity:1}` 要求**没有 hover 也要显示**。
func TestVR3RemoveShownByOwnFocusVisible(t *testing.T) {
	dir := vr3CaptureDir(t)
	rail := vr3OpenRail(t, dir, 1180, 240, &vr3Sink{}, []string{`D:\ws\a.txt`})
	theme := dscore.Dark()
	card := rail.plugin.boxes.Cards[0].Rect()
	button := vr3RemoveRect(card)
	remove, ok := rail.session.Hit("移除文件 a.txt")
	if !ok {
		t.Fatalf("真实帧里找不到删除按钮命中区：texts=%q", rail.session.Texts())
	}
	if d := remove.Bounds.X - button.X; d > 1 || d < -1 {
		t.Errorf("删除按钮命中区 x=%v，官方 .remove{right:6}（18 宽）应在 %v 附近", remove.Bounds.X, button.X)
	}
	if d := remove.Bounds.Y - button.Y; d > 1 || d < -1 {
		t.Errorf("删除按钮命中区 y=%v，官方 .remove{top:6} 应在 %v 附近", remove.Bounds.Y, button.Y)
	}
	if remove.Bounds.Width != removeSize || remove.Bounds.Height != removeSize {
		t.Errorf("删除按钮命中区=%vx%v，官方 .remove{width:18px;height:18px}",
			remove.Bounds.Width, remove.Bounds.Height)
	}

	// 指针挪开（避免 hover 冒充 focus-visible）。
	if err := rail.session.Move(4, 4); err != nil {
		t.Fatal(err)
	}
	rail.pump()
	idle := rail.shot("rail-focus-00-idle.png")
	idleInk, _ := vr3InkInside(idle, button, theme.InputMajor)

	var focused gfx.ID
	for i := 1; i <= 12; i++ {
		if err := rail.session.Keys("tab"); err != nil {
			t.Fatalf("第 %d 次 Tab 失败：%v", i, err)
		}
		rail.pump()
		focused = rail.session.Snapshot().Focused
		if focused == remove.ID {
			break
		}
	}
	if focused != remove.ID {
		t.Fatalf("按 12 次 Tab 焦点没走到删除按钮（最后焦点=%v）——按钮不是键盘停靠点", focused)
	}
	focusFrame := rail.shot("rail-focus-remove.png")
	focusInk, _ := vr3InkInside(focusFrame, button, theme.InputMajor)
	if focusInk == 0 {
		t.Fatalf("删除按钮拿到键盘焦点（focus-visible）却仍然不可见："+
			"官方 `.EEnIVa_remove:focus-visible{opacity:1}`（rc.2 asar 偏移 28546292）没生效"+
			"（静止 ink=%d）", idleInk)
	}
	if diff := vr3PixelDiff(idle, focusFrame); diff == 0 {
		t.Errorf("焦点落到删除按钮，画面零像素变化")
	}
	t.Logf("focus-visible：ink %d → %d，diff=%d", idleInk, focusInk, vr3PixelDiff(idle, focusFrame))
}

// ── ③ 失败态常显 ───────────────────────────────────────────────────────────

// TestVR3FailedCardShowsRemoveAndHostText 失败态（`stateError`）：
//   - `.removeFailed{background:state-error-primary;color:label-on-color;opacity:1}`：常显；
//   - `.metaFailed{color:state-error-primary}`：第二行换成**宿主原文**；
//   - `.failed{border-color:state-error-primary}`：卡描边转 error 色。
func TestVR3FailedCardShowsRemoveAndHostText(t *testing.T) {
	dir := vr3CaptureDir(t)
	hostText := "附件不能是 0 bytes"
	sink := &vr3Sink{fail: errors.New(hostText)}
	rail := vr3OpenRail(t, dir, 1180, 240, sink, []string{`D:\ws\broken.txt`})
	theme := dscore.Dark()

	items := rail.plugin.itemSnapshot()
	if len(items) != 1 || items[0].State != stateError {
		t.Fatalf("上传失败后状态=%v，want stateError", items)
	}
	card := rail.plugin.boxes.Cards[0].Rect()
	button := vr3RemoveRect(card)

	// 指针挪开：失败态的删除按钮**不需要 hover** 也必须可见。
	if err := rail.session.Move(4, 4); err != nil {
		t.Fatal(err)
	}
	rail.pump()
	frame := rail.shot("rail-failed.png")
	if !rail.session.HasText(hostText) {
		t.Fatalf("失败卡片没有显示宿主原文 %q：texts=%q", hostText, rail.session.Texts())
	}
	var meta *gfx.Text
	for _, one := range rail.session.Snapshot().Texts {
		if one.Value == hostText {
			copied := one
			meta = &copied
			break
		}
	}
	if meta == nil {
		t.Fatal("快照里找不到失败文案")
	}
	if meta.Style.Color != theme.Error {
		t.Errorf("失败文案颜色=%v，官方 .metaFailed{color:state-error-primary}=%v",
			meta.Style.Color, theme.Error)
	}
	if meta.Style.Size != metaFontSize || meta.Bounds.Height != metaLineH {
		t.Errorf("失败文案字号/行盒=%v/%v，官方 .meta 是 12/15", meta.Style.Size, meta.Bounds.Height)
	}
	// 删除按钮常显：圆形底该以 Error 色出现。
	foundErrorButton := false
	for _, primitive := range vr3PrimitivesWithColor(rail.session.Snapshot(), theme.Error) {
		b := primitive.Bounds
		if b.Width != removeSize || b.Height != removeSize {
			continue
		}
		if d := b.X - button.X; d <= 1 && d >= -1 {
			if primitive.Radius == removeSize/2 {
				foundErrorButton = true
			}
		}
	}
	if !foundErrorButton {
		t.Errorf("失败态没有画出官方 .removeFailed 的 error 色圆面（bounds=%+v）", button)
	}
	if ink, _ := vr3InkInside(frame, button, theme.InputMajor); ink == 0 {
		t.Errorf("失败态删除按钮区域没有任何 ink（官方 .removeFailed{opacity:1} 常显）")
	}
}

// ── ④ 上传中的进度条 ───────────────────────────────────────────────────────

// TestVR3UploadingProgressTrackGeometry 上传中态的两条官方几何：
//
//	.EEnIVa_progressTrack{height:2px;position:absolute;bottom:5px;left:12px;right:12px}
//	.EEnIVa_progressBar{width:35%}
//
// （rc.2 asar 偏移 28546292 起的 FileCard css 常量）。进度未知时官方画的就是
// 35% 那条不定态宽度——实现不许自己编一个 100%。
func TestVR3UploadingProgressTrackGeometry(t *testing.T) {
	dir := vr3CaptureDir(t)
	gate := make(chan struct{})
	sink := &vr3Sink{gate: gate}
	var once sync.Once
	release := func() { once.Do(func() { close(gate) }) }
	t.Cleanup(release)

	// 异步 run：Put 卡在 gate 上，「上传中」这一态才留得住。
	kernel := uikernel.New("attachment-vr3-uploading")
	p := &Plugin{}
	p.run = func(fn func()) { go fn() }
	if err := kernel.Load(&dscore.Plugin{Dark: true}, p); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	SetSink(sink)
	t.Cleanup(func() { SetSink(nil) })
	if result := HandleFileDrop(gfx.FileDropEvent{Phase: gfx.FileDropDrop, Paths: []string{`D:\ws\big.png`}}); !result.Handled {
		t.Fatal("拖入没有被消费")
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("attachment-vr3-uploading")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{
		Title: "attachment-vr3-uploading", Width: 1180, Height: 240, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			Geometry: pluginapi.Geometry{Width: bounds.Width, Height: bounds.Height},
			Has:      func(slot pluginapi.Slot) bool { return kernel.Has(slot) },
			Act:      pluginapi.Actions{Invalidate: invalidator.Invalidate},
		}
		view.Draw(canvas, bounds, func(f ui.C) {
			kernel.DrawSlot(f, uikernel.OffInputAttachments, env)
		})
		for _, painter := range kernel.AllPainters() {
			if painter.AfterDraw != nil {
				painter.AfterDraw(canvas, env)
			}
		}
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	for range 3 {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	rail := &vr3Rail{t: t, session: session, plugin: p, dir: dir, width: 1180, height: 240}
	if got := p.itemSnapshot(); len(got) != 1 || got[0].State != stateUploading {
		t.Fatalf("gate 未放行时状态=%v，want stateUploading", got)
	}
	rail.shot("rail-uploading.png")

	card := p.boxes.Cards[0].Rect()
	if !session.HasText(zhFileUploading) {
		t.Fatalf("上传中卡片没有「%s」：texts=%q", zhFileUploading, session.Texts())
	}
	// 轨道：left/right 12、height 2、bottom 5。
	var track *app.Primitive
	for _, primitive := range vr3PrimitivesWithColor(session.Snapshot(), theme.FileUploadTrack) {
		if primitive.Bounds.Height != progressHeight {
			continue
		}
		copied := primitive
		track = &copied
		break
	}
	if track == nil {
		t.Fatalf("没有画出进度轨道（--dsw-specific-file-upload-track 的 2px 条）")
	}
	// `.card{box-sizing:border-box;border:.5px solid}`：绝对定位的参照是卡的 padding box，
	// 所以 left:12 落在 0.5px 描边**之内**，可用宽 = 240 - 1（两侧描边）。
	wantX := card.X + cardBorder + progressInsetX
	wantW := card.Width - 2*cardBorder - 2*progressInsetX
	wantY := card.Y + card.Height - progressBottom - progressHeight
	if d := track.Bounds.X - wantX; d > 1 || d < -1 {
		t.Errorf("轨道 x=%v，官方 .progressTrack{left:12px}（含 0.5px 描边）应为 %v", track.Bounds.X, wantX)
	}
	if d := track.Bounds.Width - wantW; d > 1 || d < -1 {
		t.Errorf("轨道宽=%v，官方 .progressTrack{left:12;right:12} 应为 %v", track.Bounds.Width, wantW)
	}
	if d := track.Bounds.Y - wantY; d > 1 || d < -1 {
		t.Errorf("轨道 y=%v，官方 .progressTrack{bottom:5px;height:2px} 应为 %v", track.Bounds.Y, wantY)
	}
	if track.Radius != progressRadius {
		t.Errorf("轨道圆角=%v，官方 border-radius:1px", track.Radius)
	}
	// 进度条：35% 宽、brand-primary。
	var bar *app.Primitive
	for _, primitive := range vr3PrimitivesWithColor(session.Snapshot(), theme.BrandPrimary) {
		if primitive.Bounds.Height != progressHeight {
			continue
		}
		copied := primitive
		bar = &copied
		break
	}
	if bar == nil {
		t.Fatalf("没有画出进度条（--dsw-alias-brand-primary 的 2px 条）")
	}
	wantBarW := wantW * progressWidthPercent / 100
	if diff := bar.Bounds.Width - wantBarW; diff > 1 || diff < -1 {
		t.Errorf("进度条宽=%v，官方 .progressBar{width:35%%} 应为 %v（轨道宽的 35%%）",
			bar.Bounds.Width, wantBarW)
	}
	release()
}

// ── ⑤ 溢出翻页（多附件 rail）────────────────────────────────────────────────

// TestVR3RailOverflowPaginatesWithArrows 8 张卡塞进 900 宽的 rail（一页 3 张）：
// 只能画一页、必须出现翻页箭头（`AttachmentRail.module.css` rc.2 偏移 28536285：
// `.arrow{24×24;top:50%;translateY(-50%)}`、`.arrowLeft{left:4px}`/`.arrowRight{right:4px}`），
// 且点右箭头真的换页。
func TestVR3RailOverflowPaginatesWithArrows(t *testing.T) {
	dir := vr3CaptureDir(t)
	paths := make([]string, 0, 8)
	for i := 1; i <= 8; i++ {
		paths = append(paths, fmt.Sprintf(`D:\ws\f%02d.txt`, i))
	}
	rail := vr3OpenRail(t, dir, 900, 240, &vr3Sink{}, paths)
	first := rail.shot("rail-overflow-page1.png")

	cards := rail.plugin.boxes.Cards
	railWidth := rail.plugin.boxes.Rail.Rect().Width
	per := int((railWidth + itemGap) / (cardWidth + itemGap))
	if per < 1 {
		per = 1
	}
	if len(cards) > per {
		t.Errorf("一页画了 %d 张卡，900 宽 rail 只放得下 %d 张——溢出了", len(cards), per)
	}
	t.Logf("rail 宽=%v，一页 %d 张，这一帧画了 %d 张", railWidth, per, len(cards))

	// 箭头几何：24×24、左 4 / 右 4、垂直居中（top 50%）。
	railRect := rail.plugin.boxes.Rail.Rect()
	arrows := rail.plugin.boxes.Arrows
	if len(arrows) != 1 {
		t.Fatalf("第 1 页应只有「向右」一枚箭头（左边那枚在官方 `edges.left=false` 时不渲染），实际 %d 枚", len(arrows))
	}
	right := arrows[0].Rect()
	if right.Width != arrowSize || right.Height != arrowSize {
		t.Errorf("箭头尺寸=%vx%v，官方 .arrow{width:24px;height:24px}", right.Width, right.Height)
	}
	if gap := railRect.X + railRect.Width - (right.X + right.Width); gap < arrowInset-1 || gap > arrowInset+1 {
		t.Errorf("右箭头右距=%v，官方 .arrowRight{right:4px}", gap)
	}
	centerGap := (right.Y + right.Height/2) - (railRect.Y + railRect.Height/2)
	if centerGap > 1 || centerGap < -1 {
		t.Errorf("箭头垂直中心偏 %v，官方 .arrow{top:50%%;transform:translateY(-50%%)}", centerGap)
	}

	// 真点右箭头 → 换页：第一张卡必须换人。
	// 换页换的是**内容**，不是卡片的 x/y（每页第一张都摆在 rail 同一位置），
	// 所以判据必须落在"这一页第一张卡叫什么"上。
	firstCardName := func() string {
		for _, label := range rail.session.Snapshot().Labels {
			if len(label.Value) > 2 && label.Value[:2] == "f0" {
				return label.Value
			}
		}
		return ""
	}
	before := firstCardName()
	if before == "" {
		t.Fatalf("第 1 页没画出任何卡片标题：texts=%q", rail.session.Texts())
	}
	if err := rail.session.Click(right.X+right.Width/2, right.Y+right.Height/2); err != nil {
		t.Fatal(err)
	}
	rail.pump()
	rail.pump()
	second := rail.shot("rail-overflow-page2.png")
	if len(rail.plugin.boxes.Cards) == 0 {
		t.Fatal("翻页后一张卡都没有")
	}
	after := firstCardName()
	if after == before {
		t.Errorf("点右箭头后第一张卡还是 %q——没有真的换页", after)
	}
	if diff := vr3PixelDiff(first, second); diff == 0 {
		t.Errorf("翻页前后画面零像素变化")
	}
	// 第一页最后一张 / 翻页后那张，都只能在 rail 里。
	for _, card := range rail.plugin.boxes.Cards {
		rect := card.Rect()
		if rect.X < railRect.X-1 || rect.X+rect.Width > railRect.X+railRect.Width+1 {
			t.Errorf("卡片越出 rail：%+v（rail=%+v）", rect, railRect)
		}
	}
}
