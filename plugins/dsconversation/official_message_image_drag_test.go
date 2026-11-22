package dsconversation

// official_message_image_drag_test.go —— 附件图拖出（`#19` ③）的判据。
//
// # 这一组要证明的四件事
//
//	① **文件名**：`resolveImageDragFileName` 逐条对上官方 `resolveDragFileName`
//	   （后缀兼容就留、不兼容就换、没后缀就补、Windows 保留名加 `_`、按 UTF-8 字节截断）；
//	② **交出去的是真东西**：拖出那一刻磁盘上那个文件的名字与字节，正是那张附件的原始
//	   字节（不是解码后的位图、不是裁切结果），而且**拖完就删**（不留垃圾、也不在
//	   目标还读着的时候抽走——`StartFileDrag` 是阻塞的，它返回时已经读完）；
//	③ **手势门槛**：只按下不动 = 点图（开灯箱），越过 4px 才是拖出；
//	④ **等价性**（硬要求）：拖出这条链**不许**影响"不拖也能正常看图"——
//	   同一张图，只按不拖仍然要开得出灯箱；反过来拖过一次也不许把灯箱顺便打开。
//
// 真实的那一次 `DoDragDrop` 在单测里按不出来（要真鼠标、而且会阻塞到松手），
// 所以 `startImageFileDrag` 是可替换的包级变量，这里换成记录器。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// dragCall 是一次被替换下来的"发起拖出"的记录。
type dragCall struct {
	paths   []string
	allowed app.FileDragEffect
	// name / body / dir 是**调用当时**从磁盘读到的东西（拖出期间的真相）。
	name string
	body []byte
	dir  string
	// existed 报告那个文件在调用时是不是真的存在。
	existed bool
	// removedAtReturn 由测试在 OnPointer 返回之后填：临时目录是否已经删干净。
}

// stubImageFileDrag 把 `startImageFileDrag` 换成记录器，返回记录槽与还原函数。
//
// 它**立刻**把文件读出来（模拟目标在 drop 时读文件），所以"拖完才删"这条判据
// 验的是真实时序：读发生在删之前。
func stubImageFileDrag(t *testing.T) (*[]dragCall, func()) {
	t.Helper()
	calls := make([]dragCall, 0, 1)
	previous := startImageFileDrag
	startImageFileDrag = func(paths []string, allowed app.FileDragEffect) (app.FileDragEffect, error) {
		call := dragCall{paths: append([]string(nil), paths...), allowed: allowed}
		if len(paths) == 1 {
			call.existed = true
			call.dir = filepath.Dir(paths[0])
			call.name = filepath.Base(paths[0])
			body, err := os.ReadFile(paths[0])
			if err != nil {
				t.Errorf("拖出时读不到临时文件 %q：%v", paths[0], err)
			}
			call.body = body
		}
		calls = append(calls, call)
		return app.FileDragCopy, nil
	}
	return &calls, func() { startImageFileDrag = previous }
}

// dragHarness 是"一张就绪的附件图 + 它的拖出源身份"。
type dragHarness struct {
	*messageImageLightboxHarness
	hitID gfx.ID
}

// newDragHarness 造夹具、等图就绪、拿到那个可拖出源在**上一帧**登记的交互 id。
func newDragHarness(t *testing.T, body []byte) *dragHarness {
	t.Helper()
	h := newMessageImageLightboxHarness(t, body).ready()
	h.frame(gfx.ID{})
	hitID := h.tree.ID(h.feat, "message-image-open-u-0")
	if hitID.IsZero() {
		t.Fatal("拿不到附件图的交互 id")
	}
	attachmentID, ok := h.view.imageDragSourceFor(hitID)
	if !ok || attachmentID != "att-shot" {
		t.Fatalf("就绪的附件图没有登记成拖出源：ok=%v id=%q", ok, attachmentID)
	}
	return &dragHarness{messageImageLightboxHarness: h, hitID: hitID}
}

// down / move / up 是壳会转进来的三个指针事件。
func (h *dragHarness) down(x, y float32) bool {
	return h.plugin.OnPointer(gfx.PointerEvent{Kind: gfx.PointerDown,
		Button: gfx.PointerButtonPrimary, Target: h.hitID, X: x, Y: y})
}

func (h *dragHarness) move(x, y float32) bool {
	return h.plugin.OnPointer(gfx.PointerEvent{Kind: gfx.PointerMove, Target: h.hitID, X: x, Y: y})
}

func (h *dragHarness) up(x, y float32) bool {
	return h.plugin.OnPointer(gfx.PointerEvent{Kind: gfx.PointerUp, Target: h.hitID, X: x, Y: y})
}

// TestResolveImageDragFileName 逐条对上官方 `resolveDragFileName`。
//
// 官方（`OFF:…/dsh-client-ui-file-drag/lib/client.js` 的 `resolveDragFileName`）：
//
//	suffix 兼容声明 MIME → 保留原名后缀
//	suffix 为空、或与 MIME 不兼容 → 换成 MIME 的默认扩展名
//	没有后缀且 MIME 不认识 → 保持原名（`preferred === null` 那一支）
//	Windows 保留设备名 → 前面加 `_`
//	按 UTF-8 字节把主体截到 255 - 后缀（后缀≤32 字节）
func TestResolveImageDragFileName(t *testing.T) {
	cases := []struct {
		name, mediaType, want string
		why                   string
	}{
		{"shot.png", "image/png", "shot.png", "后缀与声明 MIME 兼容 ⇒ 不动"},
		{"shot.jpg", "image/png", "shot.png", "后缀不兼容 ⇒ 换成声明的那一个"},
		{"blob", "image/png", "blob.png", "没后缀 ⇒ 补 MIME 的默认扩展名"},
		{"shot.JPEG", "image/jpeg", "shot.JPEG", "jpg/jpeg 同族后缀都算兼容"},
		{"shot.png", "", "shot.png", "MIME 不认识 ⇒ preferred 为空，保持原名"},
		{"shot", "", "shot", "没后缀且 MIME 不认识 ⇒ 保持原名"},
		{"a<b>c:d\"e/f\\g|h?i*j", "image/png", "a_b_c_d_e_f_g_h_i_j.png", "非法字符全换下划线"},
		{"...hidden.png", "image/png", "hidden.png", "掐掉首尾的点"},
		{"CON", "image/png", "_CON.png", "Windows 保留设备名加 _"},
		{"con.txt", "text/plain", "_con.txt", "保留名带后缀也加 _"},
		{"lpt3", "image/png", "_lpt3.png", "lpt1-9 同样是保留名"},
		{"console", "image/png", "console.png", "console 不是保留名（con 才判）"},
		{"", "image/png", "file.png", "空名兜底 file"},
	}
	for _, c := range cases {
		if got := resolveImageDragFileName(c.name, c.mediaType); got != c.want {
			t.Errorf("resolveImageDragFileName(%q, %q) = %q，期望 %q（%s）",
				c.name, c.mediaType, got, c.want, c.why)
		}
	}
}

// TestResolveImageDragFileNameTruncatesByUTF8Bytes 钉 255 字节上限与"不切断中文"。
func TestResolveImageDragFileNameTruncatesByUTF8Bytes(t *testing.T) {
	// 200 个汉字 = 600 字节，远超 255。
	long := strings.Repeat("图", 200)
	got := resolveImageDragFileName(long+".png", "image/png")
	if len(got) > imageDragMaxNameBytes {
		t.Fatalf("文件名 %d 字节，超过官方上限 %d 字节", len(got), imageDragMaxNameBytes)
	}
	if !strings.HasSuffix(got, ".png") {
		t.Fatalf("截断把后缀吃掉了：%q", got)
	}
	if !strings.HasPrefix(got, "图") {
		t.Fatalf("截断把开头吃掉了：%q", got)
	}
	// 不许把一个汉字切成半个（截断点必须是合法 UTF-8）。
	if !isValidUTF8(got) {
		t.Fatalf("截断切断了多字节字符：%q", got)
	}
}

// isValidUTF8 是 test-local 的合法性判断（不引 unicode/utf8 只为一条断言）。
func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == '\uFFFD' {
			return false
		}
	}
	return true
}

// TestImageDragWritesTheRealBytesThenRemovesThem 钉 ②。
func TestImageDragWritesTheRealBytesThenRemovesThem(t *testing.T) {
	body := solidPNG(t, 400, 300)
	h := newDragHarness(t, body)
	calls, restore := stubImageFileDrag(t)
	defer restore()

	if h.down(100, 100) {
		t.Fatal("按下不该被消费（页面还要靠它把点击置起来）")
	}
	if !h.move(120, 100) {
		t.Fatal("越过阈值后应当被消费（这次移动被拖出吃掉）")
	}
	if len(*calls) != 1 {
		t.Fatalf("发起拖出 %d 次，期望 1 次（拖出错误：%q）", len(*calls), h.plugin.imageDragErr)
	}
	call := (*calls)[0]

	// 交出去的是**一份绝对路径**，而且文件名是附件原名。
	if len(call.paths) != 1 {
		t.Fatalf("拖出路径 %d 条，期望 1 条", len(call.paths))
	}
	if !filepath.IsAbs(call.paths[0]) {
		t.Fatalf("拖出路径不是绝对路径：%q（viva 的 StartFileDrag 直接报错）", call.paths[0])
	}
	if call.name != "shot.png" {
		t.Fatalf("临时文件名 = %q，期望 %q（资源管理器显示的就是这个）", call.name, "shot.png")
	}
	// 字节必须与宿主给的一模一样。
	if string(call.body) != string(body) {
		t.Fatalf("临时文件字节与附件原始字节不一致：%d 字节 vs %d 字节", len(call.body), len(body))
	}
	// 效果：复制 + 移动（链接不给）。
	if call.allowed&app.FileDragCopy == 0 {
		t.Fatalf("拖出没有提供复制效果：%v", call.allowed)
	}
	// 拖完就删：`OnPointer` 返回之后那个临时目录必须已经不在了。
	if _, err := os.Stat(call.dir); !os.IsNotExist(err) {
		t.Fatalf("拖出结束后临时目录还在：%q（err=%v）", call.dir, err)
	}
}

// TestImageDragNeedsAPressAndAMove 钉 ③：门槛。
func TestImageDragNeedsAPressAndAMove(t *testing.T) {
	h := newDragHarness(t, solidPNG(t, 400, 300))
	calls, restore := stubImageFileDrag(t)
	defer restore()

	// 只按下再松开：一次拖出都不许发起。
	h.down(100, 100)
	h.up(100, 100)
	if len(*calls) != 0 {
		t.Fatalf("只点了一下就发起了 %d 次拖出", len(*calls))
	}

	// 按下后只挪 3px（< 阈值 4）：还是不许。
	h.down(100, 100)
	h.move(103, 100)
	if len(*calls) != 0 {
		t.Fatalf("挪了 3px（阈值 %d）就发起了拖出", imageDragThreshold)
	}

	// 再挪到 5px：这次要发起，而且**只发起一次**（状态已清）。
	h.move(105, 100)
	if len(*calls) != 1 {
		t.Fatalf("越过阈值后发起 %d 次，期望 1 次", len(*calls))
	}
	h.move(130, 100)
	if len(*calls) != 1 {
		t.Fatalf("同一次按下只该发起一次拖出，实际 %d 次", len(*calls))
	}
}

// TestImageDragNeedsThePressedImage 钉"按在别的目标上不算"。
func TestImageDragNeedsThePressedImage(t *testing.T) {
	h := newDragHarness(t, solidPNG(t, 400, 300))
	calls, restore := stubImageFileDrag(t)
	defer restore()

	other := h.tree.ID(h.feat, "not-an-image")
	h.plugin.OnPointer(gfx.PointerEvent{Kind: gfx.PointerDown,
		Button: gfx.PointerButtonPrimary, Target: other, X: 100, Y: 100})
	h.plugin.OnPointer(gfx.PointerEvent{Kind: gfx.PointerMove, Target: other, X: 200, Y: 100})
	if len(*calls) != 0 {
		t.Fatalf("按在非图片目标上也发起了 %d 次拖出", len(*calls))
	}

	// 右键按在图上也不算（官方 `.frame` 是按钮，拖出只认主键）。
	h.plugin.OnPointer(gfx.PointerEvent{Kind: gfx.PointerDown,
		Button: gfx.PointerButtonSecondary, Target: h.hitID, X: 100, Y: 100})
	h.plugin.OnPointer(gfx.PointerEvent{Kind: gfx.PointerMove, Target: h.hitID, X: 200, Y: 100})
	if len(*calls) != 0 {
		t.Fatalf("右键拖也发起了 %d 次拖出", len(*calls))
	}
}

// TestImageClickStillOpensTheLightbox 钉 ④ 的正面：**不拖也能正常看图**。
//
// 同一张图、同一块命中区：按下 → 松开（不挪）必须是"开灯箱"，而拖出那条链
// 一次都没被碰过。这条是"拖出没有把看图这条路弄坏"的等价性断言。
func TestImageClickStillOpensTheLightbox(t *testing.T) {
	h := newDragHarness(t, solidPNG(t, 400, 300))
	calls, restore := stubImageFileDrag(t)
	defer restore()

	h.down(100, 100)
	h.up(100, 100)
	if len(*calls) != 0 {
		t.Fatalf("只点了一下却发起了拖出（那会把点击吃掉）")
	}
	if h.view.imageLightboxOpen() {
		t.Fatal("前置失败：只按下松开不该自己开灯箱（要页面投递一次点击）")
	}
	// 页面把这一下当成点击：走的就是那条 `Clicked()` 路径。
	h.frame(h.hitID)
	if !h.view.imageLightboxOpen() {
		t.Fatal("点图没有打开灯箱——拖出把看图这条路弄坏了")
	}
	if len(*calls) != 0 {
		t.Fatalf("开灯箱时顺带发起了 %d 次拖出", len(*calls))
	}
}

// TestImageDragDoesNotOpenTheLightbox 钉 ④ 的反面：拖过之后不许顺便开灯箱。
//
// 官方靠 `DraggableImage` 的 `onClickCapture` 里那个 `dragging` 标志
// （`xb` 的 `c.current`）挡掉这一下；本仓靠"发起拖出那次移动被消费、松手被
// `DoDragDrop` 的模态循环吃掉"达成同一效果。这条判据证明它成立。
func TestImageDragDoesNotOpenTheLightbox(t *testing.T) {
	h := newDragHarness(t, solidPNG(t, 400, 300))
	calls, restore := stubImageFileDrag(t)
	defer restore()

	h.down(100, 100)
	h.move(120, 100) // 发起拖出
	if len(*calls) != 1 {
		t.Fatalf("前置失败：拖出没发起（%d 次）", len(*calls))
	}
	// 拖完之后再画一帧：灯箱必须是关的。
	h.frame(gfx.ID{})
	if h.view.imageLightboxOpen() {
		t.Fatal("拖过一次之后灯箱自己开了——拖出与点开必须互斥")
	}
}

// TestImageDragSkipsImagesThatAreNotReady 钉"没字节就不当拖出源"。
func TestImageDragSkipsImagesThatAreNotReady(t *testing.T) {
	theme := newLightboxHarness(t, pluginapi.Actions{}).env.Theme
	_ = theme
	// 一个拒绝取数的夹具：图永远到不了 Ready。
	h := newMessageImageLightboxHarness(t, solidPNG(t, 400, 300))
	h.env.Act.AttachmentGet = func(string) (pluginapi.AttachmentBlob, bool) {
		return pluginapi.AttachmentBlob{}, false
	}
	h.frame(gfx.ID{})
	h.frame(gfx.ID{})
	hitID := h.tree.ID(h.feat, "message-image-open-u-0")
	if _, ok := h.view.imageDragSourceFor(hitID); ok {
		t.Fatal("图还没就绪就登记成了拖出源")
	}
	calls, restore := stubImageFileDrag(t)
	defer restore()
	h.plugin.OnPointer(gfx.PointerEvent{Kind: gfx.PointerDown,
		Button: gfx.PointerButtonPrimary, Target: hitID, X: 100, Y: 100})
	h.plugin.OnPointer(gfx.PointerEvent{Kind: gfx.PointerMove, Target: hitID, X: 200, Y: 100})
	if len(*calls) != 0 {
		t.Fatalf("没就绪的图也发起了 %d 次拖出", len(*calls))
	}
}

// TestImageDragFailureNotifiesTheStatusBar 钉失败反馈的形状：**状态栏一句话，不是弹窗**。
//
// 官方这一处是 `shell.overlay` 上的 Toast，文案 `file-drag.failed`「拖放失败，请重试」。
// 本仓没有浮条服务，落状态栏（既有差异 `#19` ②）；但**文案不许改**，原因接在冒号后。
func TestImageDragFailureNotifiesTheStatusBar(t *testing.T) {
	h := newDragHarness(t, solidPNG(t, 400, 300))
	previous := startImageFileDrag
	startImageFileDrag = func([]string, app.FileDragEffect) (app.FileDragEffect, error) {
		return app.FileDragNone, errImageDragNotReady
	}
	defer func() { startImageFileDrag = previous }()

	var notified []string
	h.plugin.lastAct = pluginapi.Actions{Notify: func(msg string) { notified = append(notified, msg) }}

	h.down(100, 100)
	if !h.move(120, 100) {
		t.Fatal("拖出失败也应当被消费（这一下已经不属于页面了）")
	}
	if len(notified) != 1 {
		t.Fatalf("状态栏收到 %d 条反馈，期望 1 条：%v", len(notified), notified)
	}
	if !strings.HasPrefix(notified[0], imageDragFailedText) {
		t.Fatalf("反馈文案 = %q，官方 `file-drag.failed` 是 %q", notified[0], imageDragFailedText)
	}
	// 不许"顺便开灯箱"：失败路径同样不能碰看图。
	h.frame(gfx.ID{})
	if h.view.imageLightboxOpen() {
		t.Fatal("拖出失败后灯箱自己开了")
	}
}
