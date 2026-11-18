//go:build darwin || windows || linux

// vr3_interaction_test.go —— 第三轮真实渲染审计：**交互态**（前两轮只看过静态外观）。
//
// 这一轮盯的三件事，都是"数值对但画出来不对 / 交互行为和官方不一致"：
//
//	① 键盘焦点环。官方 `SidebarRoot.module.css`（`128__SidebarRoot`）有两条：
//	     .iconButton.toggle:focus-visible{outline:var(--dsw-focus-ring-width) solid
//	       var(--dsw-focus-ring-color,var(--dsw-alias-state-business-primary));outline-offset:-2px}
//	     .panelRow:focus-visible{同上}
//	   `--dsw-focus-ring-width` 在 `focus.css:3` 是 **2px**（实现侧已由
//	   `plugins/dsplan/cards.go:75-76` 落成常量）。判据：Tab 走一遍，焦点每移动一格，
//	   画面**必须有像素变化**；一格都不变 = 焦点环根本没画。
//
//	② 窄窗（900×700）。判据：任何 hit / primitive / 文本都不许画出画布外；
//	   右栏总宽算下来放不下 `rightbarMin=300`（shell.go:68）时必须整列消失，
//	   不能画半列或者留一条画到窗口外的分隔线。
//
//	③ 滚动。长对话滚到底：正文不许溢出中栏，滚到底后最后一段必须真在视口里。
//
// 截图落 `$VR3_SHELL_CAPTURE_DIR`（未设置时用临时目录）。
package dsshell

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscomposer"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsrightbar"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar"
	"github.com/MYCF711/dsh-ui/plugins/dsstatusbar"
	"github.com/MYCF711/dsh-ui/plugins/dstopbar"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func vr3CaptureDir(t *testing.T) string {
	t.Helper()
	base := os.Getenv("VR3_SHELL_CAPTURE_DIR")
	if base == "" {
		return t.TempDir()
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	return base
}

// vr3Shell 是一台装配好的真壳：装配方式与 shell_full_render_test.go 同一条
// （dscore + dsshell + 侧栏/顶栏/对话/输入/右栏/状态栏）。
type vr3Shell struct {
	t        *testing.T
	session  *headless.Session
	dir      string
	width    int
	height   int
	viewport *float32
	// tree/feat 给断言用：侧栏面板行的命中 id 是 `panel-<route>`（frame.go:81-83）。
	tree  *gfx.Tree
	feat  gfx.Scope
	theme *pluginapi.Theme
}

func vr3OpenShell(t *testing.T, dir string, width, height int, data pluginapi.Snapshot) *vr3Shell {
	t.Helper()
	kernel := uikernel.New("vr3-shell")
	if err := kernel.Load(
		&dscore.Plugin{Dark: true},
		&Plugin{}, &dssidebar.Plugin{}, &dstopbar.Plugin{},
		&dsconversation.Plugin{}, &dscomposer.Plugin{},
		&dsrightbar.Plugin{}, &dsstatusbar.Plugin{},
	); err != nil {
		t.Fatalf("装配产品插件集失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("vr3-shell")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})

	const sideWidth = 280
	var invalidator app.Invalidator
	sh := &vr3Shell{t: t, dir: dir, width: width, height: height, tree: tree, feat: feat, theme: theme}
	config := app.Config{
		Title: "vr3-shell", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		rightPref := float32(0)
		if data.ShowRightbar {
			rightPref = size.Width * rightbarOpenFrac
		}
		env := &pluginapi.Env{
			Theme: theme, Data: data, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			Geometry: pluginapi.Geometry{
				Width: size.Width, Height: size.Height, SidebarWidth: sideWidth,
				RightWidth: rightPref, MainWidth: size.Width - sideWidth - rightPref,
			},
			MainWidth:      size.Width - sideWidth - rightPref,
			ViewportOffset: sh.viewport,
			Has:            kernel.Has,
			Act: pluginapi.Actions{
				Send: func(string) {}, Cancel: func() {}, NewSession: func() {},
			},
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			kernel.DrawSlot(f, pluginapi.SlotShellRoot, env)
		})
		for _, painter := range kernel.AllPainters() {
			if painter.AfterDraw != nil {
				painter.AfterDraw(canvas, env)
			}
		}
	}
	// 滚轮必须接到 page.Wheel 上（生产同一接线：cmd/app/main.go:2226）。
	// 不接的话 driver 只把 DeltaY 落到全局 scrollY 兜底，
	// ScrollArea 自己的偏移根本不会动——第一版这条就栽在这里。
	config.OnScroll = func(ev gfx.ScrollEvent) gfx.EventResult { return view.Wheel(ev) }
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless.Open(%dx%d) 失败：%v", width, height, err)
	}
	t.Cleanup(session.Close)
	sh.session = session
	for range 2 {
		invalidator.Invalidate()
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	return sh
}

// shot 落 PNG 并回读像素（回读是为了让调用方立刻做差分）。
func (s *vr3Shell) shot(name string) *image.RGBA {
	s.t.Helper()
	path := filepath.Join(s.dir, name)
	if err := s.session.Screenshot(path); err != nil {
		s.t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		s.t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	img, err := s.session.Image()
	if err != nil {
		s.t.Fatal(err)
	}
	s.t.Logf("VR3_CAPTURE path=%s size=%v", path, img.Bounds())
	return img
}

// vr3RingInkBox 取当前快照里该 id 的盒子，量它的环带 ink。
func vr3RingInkBox(img *image.RGBA, id gfx.ID, sh *vr3Shell) int {
	for _, hit := range sh.session.Snapshot().Hits {
		if hit.ID == id {
			return vr3RingInk(img, hit.Bounds, sh.theme.Info)
		}
	}
	return 0
}

// vr3RingInk 数焦点控件**内缘 0..2px 边带**里有多少像素接近官方环色
// （rc.2 focus.css：`outline-color:business-primary` = 主题 Info）。
func vr3RingInk(img *image.RGBA, box gfx.Rect, want gfx.Color) int {
	near := func(x, y int) bool {
		if !(image.Point{X: x, Y: y}).In(img.Bounds()) {
			return false
		}
		c := img.RGBAAt(x, y)
		dist := func(a, b uint8) int {
			if a > b {
				return int(a) - int(b)
			}
			return int(b) - int(a)
		}
		return dist(c.R, uint8(want.Red*255)) <= 18 && dist(c.G, uint8(want.Green*255)) <= 18 && dist(c.B, uint8(want.Blue*255)) <= 18
	}
	n := 0
	x0, y0 := int(box.X), int(box.Y)
	x1, y1 := int(box.X+box.Width), int(box.Y+box.Height)
	for y := y0 + (y1-y0)*2/10; y < y1-(y1-y0)*2/10; y++ {
		for dx := 0; dx <= 2; dx++ {
			if near(x0+dx, y) {
				n++
			}
			if near(x1-1-dx, y) {
				n++
			}
		}
	}
	for x := x0 + (x1-x0)*2/10; x < x1-(x1-x0)*2/10; x++ {
		for dy := 0; dy <= 2; dy++ {
			if near(x, y0+dy) {
				n++
			}
			if near(x, y1-1-dy) {
				n++
			}
		}
	}
	return n
}

// rowRing 数「焦点控件的盒子（含 3px 边带）」里有多少像素变了——
// 焦点环只会画在这个范围里（outline-offset:-2px 时 0..2px 边带全在盒内）。
func rowRing(a, b *image.RGBA, id gfx.ID, snap app.Snapshot) int {
	var box gfx.Rect
	found := false
	for _, hit := range snap.Hits {
		if hit.ID == id {
			box, found = hit.Bounds, true
			break
		}
	}
	if !found {
		return 0
	}
	x0 := int(box.X) - 3
	y0 := int(box.Y) - 3
	x1 := int(box.X+box.Width) + 3
	y1 := int(box.Y+box.Height) + 3
	if bounds := a.Bounds(); x0 < bounds.Min.X || y0 < bounds.Min.Y || x1 > bounds.Max.X || y1 > bounds.Max.Y {
		return 0
	}
	n := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				n++
			}
		}
	}
	return n
}

// vr3PixelDiff 数两张图有多少个像素不同。
func vr3PixelDiff(a, b *image.RGBA) int {
	if a.Bounds() != b.Bounds() {
		return -1
	}
	n := 0
	bounds := a.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				n++
			}
		}
	}
	return n
}

func (s *vr3Shell) pump() {
	s.t.Helper()
	if err := s.session.Driver().Pump(); err != nil {
		s.t.Fatal(err)
	}
}

// TestVR3TabFocusTraversal 按 Tab 走 14 格，逐格留图并统计焦点位移与像素变化。
//
// 这条测试的产出是**证据**：哪一格焦点动了、动了之后画面上有没有环。
// 焦点动了但像素一格没变，就是"官方有 focus-visible 环、实现没画"。
func TestVR3TabFocusTraversal(t *testing.T) {
	dir := vr3CaptureDir(t)
	sh := vr3OpenShell(t, dir, 1182, 832, renderShellData())

	focusable := 0
	for _, hit := range sh.session.Snapshot().Hits {
		if hit.Focusable && !hit.SkipTab {
			focusable++
		}
	}
	t.Logf("可 Tab 焦点命中区 = %d 个", focusable)

	// 焦点 ID → 是哪个控件（label/role/bounds）。没有这层映射，"焦点环没画"
	// 这条结论就落不到具体的那一行 CSS 上。
	describe := func(id gfx.ID) string {
		if id.IsZero() {
			return "none"
		}
		for _, hit := range sh.session.Snapshot().Hits {
			if hit.ID == id {
				return fmt.Sprintf("label=%q role=%v bounds=%v", hit.Label, hit.Role, hit.Bounds)
			}
		}
		return fmt.Sprintf("%v（不在 hits 里）", id)
	}

	// 官方 rc.2 只在**键盘**焦点上画环（focus.css：指针模态下 outline-color:transparent），
	// 所以这里的判据是「焦点落在该控件上时，该控件自己的像素必须有变化」。
	// 侧栏面板行是唯一已实现的环（.panelRow:focus-visible，offset:-2px）。
	panelRows := map[gfx.ID]string{
		sh.tree.ID(sh.feat, "panel-plugins"):   "插件",
		sh.tree.ID(sh.feat, "panel-schedules"): "自动化任务",
	}
	ringSeen := 0
	ringStops := 0

	before := sh.shot("focus-00-initial.png")
	prevFocus := sh.session.Snapshot().Focused
	moved, invisible := 0, 0
	for i := 1; i <= 14; i++ {
		if err := sh.session.Keys("tab"); err != nil {
			t.Fatalf("第 %d 次 Tab 失败：%v", i, err)
		}
		sh.pump()
		sh.pump()
		now := sh.shot(fmt.Sprintf("focus-%02d-tab.png", i))
		diff := vr3PixelDiff(before, now)
		focus := sh.session.Snapshot().Focused
		t.Logf("TAB %02d 焦点→ %s（上一格 %v）像素diff=%d", i, describe(focus), prevFocus, diff)
		if focus != prevFocus {
			moved++
			// 直接量环带颜色：焦点换人时上一格的环消失也会让整帧变，只有量
			// 本控件内缘 0..2px 的颜色才能说"它自己画了环 / 没画"。
			if ink := vr3RingInkBox(now, focus, sh); ink > 8 {
				ringStops++
				t.Logf("TAB %02d 环带 ink=%d（这个控件画了焦点环）", i, ink)
			}
			if want, ok := panelRows[focus]; ok {
				// 面板行必须画出焦点环：裁出这一行（含 3px 边带）单独比。
				if rowRing(before, now, focus, sh.session.Snapshot()) > 0 {
					ringSeen++
					t.Logf("TAB %02d 面板行「%s」画出焦点环", i, want)
				} else {
					t.Errorf("TAB %02d 面板行「%s」没有焦点环（rc.2 .panelRow:focus-visible outline-offset:-2px）", i, want)
				}
			}
			if diff == 0 {
				invisible++
				t.Logf("TAB %02d **焦点环不可见**：焦点落在 %s，画面零像素变化", i, describe(focus))
			}
		}
		before = now
		prevFocus = focus
	}
	t.Logf("SUMMARY 焦点移动 %d 格 / 环带可见 %d 格", moved, ringStops)
	// 这条断言只钉"焦点必须真的能走"：一格不动说明整窗没有 Tab 焦点链，
	// 那 56 个官方 CSS 里的 :focus-visible 规则全是死代码。
	if len(panelRows) > 0 && ringSeen == 0 {
		t.Fatalf("焦点扫过 %d 个面板行 id，一个焦点环都没画出来", len(panelRows))
	}
	if moved == 0 {
		t.Fatalf("按 14 次 Tab 焦点一次都没移动（可焦点命中区 %d 个）——键盘导航链断了", focusable)
	}
}

// TestVR3NarrowWindowOverflow 900×700：任何绘制物件都不许出画布。
func TestVR3NarrowWindowOverflow(t *testing.T) {
	dir := vr3CaptureDir(t)
	const w, h = 900, 700
	sh := vr3OpenShell(t, dir, w, h, renderShellData())
	img := sh.shot("narrow-900x700.png")
	if got := img.Bounds(); got.Dx() != w || got.Dy() != h {
		t.Fatalf("截图尺寸 = %v，期望 %dx%d", got, w, h)
	}
	snap := sh.session.Snapshot()
	// ① hit 区域越界（宽容 1px 抗锯齿）。
	for _, hit := range snap.Hits {
		b := hit.Bounds
		if b.X < -1 || b.Y < -1 || b.X+b.Width > float32(w)+1 || b.Y+b.Height > float32(h)+1 {
			t.Errorf("窄窗越界 hit：label=%q bounds=%+v（画布 %dx%d）", hit.Label, b, w, h)
		}
	}
	// ② primitive 越界：只报**完全在外**或明显越界的，浮点噪声不算。
	out := 0
	worst := ""
	for _, p := range snap.Primitives {
		b := p.Bounds
		if b.Width <= 0 || b.Height <= 0 {
			continue
		}
		if b.X+b.Width < -2 || b.Y+b.Height < -2 || b.X > float32(w)+2 || b.Y > float32(h)+2 {
			out++
			if worst == "" {
				worst = fmt.Sprintf("kind=%v bounds=%+v", p.Kind, b)
			}
		}
	}
	if out > 0 {
		t.Errorf("完全画在画布外的 primitive %d 个，首个：%s（画布 %dx%d）", out, worst, w, h)
	}
	// ③ 右栏：900 - 280 - 400 = 220 < rightbarMin(300)，官方 computeColumns 下整列消失。
	if _, ok := findHit(snap, handleLabel(handleRightbar)); ok {
		// 有分隔线抓取带就说明右栏还在——但 220 宽的右栏本身就不合官方下限。
		t.Errorf("900×700 下右栏分隔线仍在：available=%d 小于官方 rightbarMin=%d，右栏应整列消失",
			w-280-mainMin, rightbarMin)
	}
	// 状态栏（shell.bottom）在窄高下是否还在：官方状态栏是常驻槽，
	// 不该因为窗口变矮整条消失。
	statusSeen := sh.session.HasText("连接成功")
	// 文本画了但可能被裁到画布外：逐条量底部那段文本的 y。
	for _, text := range snap.Texts {
		if text.Bounds.Y+text.Bounds.Height > float32(h)+1 || text.Bounds.X+text.Bounds.Width > float32(w)+1 {
			t.Logf("窄窗越界文本：%q bounds=%+v（画布 %dx%d）", text.Value, text.Bounds, w, h)
		}
	}
	t.Logf("窄窗：hits=%d primitives=%d 越界=%d 状态栏可见=%v", len(snap.Hits), len(snap.Primitives), out, statusSeen)
	if !statusSeen {
		t.Errorf("900×700 下状态栏消失了（shell.bottom 常驻槽）")
	}
}

// TestVR3ScrollToBottom 长对话滚到底：内容不许出中栏，滚到底后末尾那段要能看见。
func TestVR3ScrollToBottom(t *testing.T) {
	dir := vr3CaptureDir(t)
	const w, h = 1182, 832
	data := renderShellData()
	data.Msgs = nil
	for i := range 24 {
		data.Msgs = append(data.Msgs,
			apiclient.Message{Role: "user", Text: fmt.Sprintf("第 %d 轮：把渲染审计的滚动路径走一遍。", i+1)},
			apiclient.Message{Role: "assistant", Text: fmt.Sprintf("第 %d 轮回复：这一段要足够长，好让正文超出一屏。", i+1)},
		)
	}
	data.Msgs = append(data.Msgs, apiclient.Message{Role: "assistant", Text: "TAIL-MARKER-最后一行"})
	sh := vr3OpenShell(t, dir, w, h, data)
	top := sh.shot("scroll-00-follow-bottom.png")

	// 先钉「已经贴底」：`.Follow(true)`（conversation.go:400）下首帧就该在末尾。
	if !sh.session.HasText("TAIL-MARKER") {
		t.Fatalf("Follow(true) 首帧看不到末尾标记——视图没有贴底；texts=%q", sh.session.Texts())
	}

	// 真滚轮向上：内容应整体下移（画面必须变）。
	wheel := func(dy float32) *image.RGBA {
		if err := sh.session.Scroll(gfx.ScrollEvent{X: 500, Y: 380, DeltaY: dy, Precise: true}); err != nil {
			t.Fatalf("滚轮失败：%v", err)
		}
		sh.pump()
		sh.pump()
		img, err := sh.session.Image()
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
	before := len(sh.session.Texts())
	up := top
	for range 3 {
		up = wheel(-400)
	}
	sh.shot("scroll-50-up.png")
	after := len(sh.session.Texts())
	t.Logf("上滚前 texts=%d，上滚后 texts=%d（正文有视口裁剪，条数变化即证明偏移变了）", before, after)
	if diff := vr3PixelDiff(top, up); diff == 0 {
		t.Fatalf("向上滚了 3×400，画面零像素变化——滚动根本不动（texts %d→%d）", before, after)
	}
	if sh.session.HasText("TAIL-MARKER") {
		t.Logf("上滚 3×400 后末尾标记仍在视口里（可能内容还不够长，未滚出一屏）")
	}
	// 再滚回底部：Follow 恢复后末尾必须在视口里。
	for range 6 {
		up = wheel(400)
	}
	sh.shot("scroll-99-back-bottom.png")
	if !sh.session.HasText("TAIL-MARKER") {
		t.Fatalf("滚回底部后末尾标记不在视口里——回底没真回；texts=%q", sh.session.Texts())
	}
	// 正文不许画到中栏以外：中栏右界 = 1182 - 右栏宽（showRightbar 时按 0.45 求解）。
	snap := sh.session.Snapshot()
	right := float32(w)
	if data.ShowRightbar {
		pref := float32(w) * rightbarOpenFrac
		if pref < rightbarMin {
			pref = rightbarMin
		}
		if avail := float32(w) - 280 - mainMin; pref > avail {
			pref = avail
		}
		right = float32(w) - pref
	}
	for _, text := range snap.Texts {
		// 只看**起画在中栏里**的文本；左栏/右栏自己的文本各有各的边界。
		if text.Bounds.X < 280 || text.Bounds.X >= right {
			continue
		}
		if text.Bounds.X+text.Bounds.Width > right+1 {
			t.Errorf("正文文本越过中栏右界 %.1f：%q bounds=%+v", right, text.Value, text.Bounds)
			break
		}
	}
	t.Logf("滚动：滚到底后 texts=%d，中栏右界=%.1f", len(snap.Texts), right)
}
