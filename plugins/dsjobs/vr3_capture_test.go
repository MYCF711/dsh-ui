//go:build darwin || windows || linux

// vr3_capture_test.go —— 第三轮真实渲染审计：后台任务（官方 ui-jobs）的
// **空态 / loading / hover / 滚动 / 窄宽**。
//
// 前两轮看过的东西这里一律不重复：数值常量由 visual_geometry_test.go 钉住，
// 「有没有画出来」由 official_jobs_test.go 钉住。这一轮只验**只有真渲染才暴露**
// 的五件事，每一条都对官方原件：
//
//	① 空态：官方 `JobListAction` 在 `visibleCount === 0` 时 `return null`
//	   （`tmp/official/dsh-client-ui-jobs/client.js:407`）。本仓 `drawTrigger`
//	   同形早退（`official_jobs.go:161-164`），`drawPanel` 也是
//	   （`official_jobs.go:231-233`）——空态必须是「一个字都不画、一条命中区都没有，
//	   也没有残留的边框/底色」，不是"画个空框"。
//
//	② loading：运行中的 StateDot 是 14px spinner（官方
//	   `index-ghOpC9uV.js:129` 的 `n = size ?? (ongoing ? 14 : 10)`），而官方
//	   **真的在转**：`index-CSAdOBx6.css:1` 的
//	   `._spinnerMotion_1i3xo_42{transform-origin:center;animation:_dsh-state-dot-spin
//	   _1i3xo_1 1.5s linear infinite}` + `@keyframes ...{to{transform:rotate(360deg)}}`。
//	   判据是渲染器自己的动画时钟：`headless.Session.Transitioning()` 报告"上一帧
//	   有没有要下一帧"。转的东西会一直要帧；本仓的静态弧不会。
//
//	③ hover：官方 `.row:hover{background:var(--dsw-alias-fill-l1)}` 与
//	   `@media (hover:hover){.stop:hover{border-color:color-mix(in srgb,
//	   var(--dsw-alias-state-error-primary) 45%, transparent);color:...error-primary}}`
//	   ——指针进到行/停止钮上必须**有像素变化**。
//
//	④ 滚动：输出区 `--dsl-terminal-output-max-height:288px`（官方
//	   `JobListAction.module.css:387`），长输出必须在这 288px 里滚，
//	   滚到底最后一行要**切在行边界**上（不是半个字），底下的「复制/加载更多」
//	   不许压在输出区上。
//
//	⑤ 窄宽：`@container (max-width:460px)`（同文件 `:39` 附近的 compact 段）
//	   触发后触发器变 28×28 纯图标；900×700 窗口下 `.menu{right:12px}`
//	   的右缘必须落在 900-12=888。
//
// 截图落 `$VR3_CAPTURE_DIR`（未设置时用临时目录），每张都非空并 t.Logf 出路径。
package dsjobs

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// ── 落图与像素小工具 ─────────────────────────────────────────────────

// vr3Dir 是第三轮审计的落图目录（与其他审计员共用 $VR3_CAPTURE_DIR）。
func vr3Dir(t *testing.T) string {
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

// vr3Sample 取一个像素当"底色基准"。
//
// 为什么不拿 ui.Color 换算：viva 的 Color 是**线性** RGBA，帧缓冲是 sRGB 8 位，
// 自己换算容易把误差算进断言里。真实审计要的是"这块区域里除了底色还有没有东西"，
// 底色直接从同一帧里取，零换算。
func vr3Sample(img *image.RGBA, x, y int) color.RGBA {
	if img == nil {
		return color.RGBA{}
	}
	return img.RGBAAt(x, y)
}

// vr3Ink 在 rect 里数"与 bg 不同的像素"，并给出这块墨迹的包围盒（逻辑像素）。
//
// 为什么用像素而不是拿 ui.Box：viva 的 Box 只在本次 paint 内有效，跨帧读不到；
// 而"这 14px 的 spinner 有没有被裁"只有真画面能回答。
func vr3Ink(img *image.RGBA, rect gfx.Rect, bg color.RGBA) (box gfx.Rect, count int) {
	if img == nil || rect.Width <= 0 || rect.Height <= 0 {
		return gfx.Rect{}, 0
	}
	r, g, b := bg.R, bg.G, bg.B
	minX, minY := float32(1e9), float32(1e9)
	maxX, maxY := float32(-1e9), float32(-1e9)
	left, top := int(rect.X), int(rect.Y)
	right, bottom := int(rect.X+rect.Width), int(rect.Y+rect.Height)
	bounds := img.Bounds()
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			if x < bounds.Min.X || y < bounds.Min.Y || x >= bounds.Max.X || y >= bounds.Max.Y {
				continue
			}
			p := img.RGBAAt(x, y)
			if absDiff(p.R, r) <= 6 && absDiff(p.G, g) <= 6 && absDiff(p.B, b) <= 6 {
				continue
			}
			count++
			if fx := float32(x); fx < minX {
				minX = fx
			} else if fx > maxX {
				maxX = fx
			}
			if fy := float32(y); fy < minY {
				minY = fy
			} else if fy > maxY {
				maxY = fy
			}
		}
	}
	if count == 0 {
		return gfx.Rect{}, 0
	}
	return gfx.Rect{X: minX, Y: minY, Width: maxX - minX + 1, Height: maxY - minY + 1}, count
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a) - int(b)
	}
	return int(b) - int(a)
}

// vr3PixelDiff 数两张图的差异像素数（hover 态的唯一硬证据）。
func vr3PixelDiff(a, b *image.RGBA) int {
	if a == nil || b == nil || a.Bounds() != b.Bounds() {
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

// ── 真窗口现场 ──────────────────────────────────────────────────────

// vr3Jobs 是一台真窗口：官方档触发器 + 遮罩 + 弹层都在同一帧里画。
type vr3Jobs struct {
	t       *testing.T
	p       *Plugin
	act     *fakeJobs
	theme   *pluginapi.Theme
	tree    *gfx.Tree
	feat    gfx.Scope
	session *headless.Session
	dir     string
	// containerW 是 dispatch 容器的宽度（compact 断点与环境宽度都吃它）。
	containerW float32
}

const (
	// vr3HeaderX/Y 是触发器在窗口里的落点（会话 header 动作带的位置）。
	vr3HeaderX float32 = 120
	vr3HeaderY float32 = 10
)

// vr3OpenJobs 起窗口。windowW/H 是窗口尺寸，containerW 是 dispatch 容器宽度。
func vr3OpenJobs(t *testing.T, dir string, windowW, windowH int, containerW float32,
	jobs []pluginapi.Job, output func(string, string, int) (pluginapi.JobOutput, bool)) *vr3Jobs {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("jobs-vr3")
	p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
	act := &fakeJobs{jobs: jobs, output: output}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	h := &vr3Jobs{t: t, p: p, act: act, theme: theme, tree: tree, feat: feat, dir: dir, containerW: containerW}
	config := app.Config{
		Title: "jobs-vr3", Width: windowW, Height: windowH, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
		// 滚轮必须接给页面（壳在 cmd/app/main.go:2226 做同一件事）；
		// 不接的话 WM_MOUSEWHEEL 被直接吞掉，任何滚动区都不动。
		OnScroll: view.Wheel,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		// 真壳里触发器在会话 header 动作带（容器宽 = 动作带宽度），弹层在
		// shell.overlay（容器宽 = 整个窗口）。两者 Env.Bounds 不同，而插件两种
		// 宽度都只从 Env.Bounds 读（official_jobs.go:173、:237），所以这里分别喂。
		headerEnv := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: gfx.Rect{Width: containerW, Height: size.Height}, MainWidth: containerW,
			Act: act.actions(),
		}
		headerEnv.Act.Invalidate = invalidator.Invalidate
		overlayEnv := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width, Act: act.actions(),
		}
		overlayEnv.Act.Invalidate = invalidator.Invalidate
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			ui.Row(f).Absolute().Left(vr3HeaderX).Top(vr3HeaderY).Children(func() {
				p.drawTrigger(f, headerEnv)
			})
			p.drawPanelBackdrop(f, overlayEnv)
			p.drawPanel(f, overlayEnv, nil)
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless.Open(%dx%d) 失败：%v", windowW, windowH, err)
	}
	t.Cleanup(session.Close)
	h.session = session
	h.pump()
	h.pump()
	return h
}

func (h *vr3Jobs) pump() {
	h.t.Helper()
	if err := h.session.Driver().Pump(); err != nil {
		h.t.Fatal(err)
	}
}

// id 造出与插件同一条公式的交互 id。
func (h *vr3Jobs) id(name string) gfx.ID { return h.tree.ID(h.feat, name) }

// hit 在**本帧快照**里找命中区（真 HitTest 的产物）。
func (h *vr3Jobs) hit(name string) (gfx.Hit, bool) {
	want := h.id(name)
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID == want {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// hitLabels 把本帧所有命中区压成 label@bounds（失败时看得见"到底画了什么"）。
func (h *vr3Jobs) hitLabels() []string {
	out := make([]string, 0, len(h.session.Snapshot().Hits))
	for _, hit := range h.session.Snapshot().Hits {
		out = append(out, fmt.Sprintf("%q@%v", hit.Label, hit.Bounds))
	}
	return out
}

// shot 落 PNG、回读像素、断言非空。
func (h *vr3Jobs) shot(name string) *image.RGBA {
	h.t.Helper()
	path := filepath.Join(h.dir, name)
	if err := h.session.Screenshot(path); err != nil {
		h.t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		h.t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	img, err := h.session.Image()
	if err != nil {
		h.t.Fatal(err)
	}
	h.t.Logf("VR3_CAPTURE path=%s size=%v bytes=%d", path, img.Bounds(), info.Size())
	return img
}

// move 真移指针（hover 态只有真走指针才作数）。
func (h *vr3Jobs) move(x, y float32) {
	h.t.Helper()
	if err := h.session.Move(x, y); err != nil {
		h.t.Fatal(err)
	}
	h.pump()
	h.pump()
}

// click 真点击（真命中测试 → 真事件）。
func (h *vr3Jobs) click(x, y float32) {
	h.t.Helper()
	if err := h.session.Click(x, y); err != nil {
		h.t.Fatal(err)
	}
	h.pump()
	h.pump()
}

func (h *vr3Jobs) clickHit(hit gfx.Hit) {
	h.t.Helper()
	h.click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2)
}

func (h *vr3Jobs) texts() []string {
	out := make([]string, 0, len(h.session.Snapshot().Texts))
	for _, text := range h.session.Snapshot().Texts {
		out = append(out, text.Value)
	}
	return out
}

func (h *vr3Jobs) hasText(want string) bool {
	for _, value := range h.texts() {
		if value == want {
			return true
		}
	}
	return false
}

// ── ① 空态 ──────────────────────────────────────────────────────────

// TestVR3JobsEmptyStateDrawsNothing 验"没有任务时一个字都不画"。
//
// 官方依据（两条）：
//
//	`tmp/official/dsh-client-ui-jobs/client.js:407`  `if (visibleCount === 0) return null;`
//	`tmp/official/dsh-client-ui-jobs/client.js:370`  `if (visibleCount === 0 && open) setOpen(false);`
//
// 也就是说：空清单**没有触发器**、没有弹层，也没有"0 个后台任务"这种编出来的计数。
// 空态最坏的样子不是空白，而是"留一个空框/一条残留描边"——所以这里同时数像素。
func TestVR3JobsEmptyStateDrawsNothing(t *testing.T) {
	h := vr3OpenJobs(t, vr3Dir(t), 900, 700, 900, nil, nil)

	if got := h.texts(); len(got) != 0 {
		t.Fatalf("空清单画出了文字：%v（官方 visibleCount===0 → return null，client.js:407）", got)
	}
	if hits := h.session.Snapshot().Hits; len(hits) != 0 {
		t.Fatalf("空清单留下了 %d 条命中区：%+v（官方不渲染任何元素）", len(hits), hits)
	}
	img := h.shot("jobs-empty-900x700.png")

	// 连"残留底色/边框"都不许有：整窗只允许主题底色一种像素。
	box, count := vr3Ink(img, gfx.Rect{Width: 900, Height: 700}, vr3Sample(img, 1, 1))
	if count != 0 {
		t.Fatalf("空态窗口里有多余墨迹 %d 像素（bbox=%+v）：官方空清单什么都不画", count, box)
	}

	// 反过来钉住"有任务才有触发器"，否则上面那条在任何实现下都成立。
	h2 := vr3OpenJobs(t, h.dir, 900, 700, 900,
		[]pluginapi.Job{{ID: "job-1", SessionID: "s-1", Command: "npm test"}}, nil)
	if !h2.hasText("1 个后台任务") {
		t.Fatalf("有活任务时触发器没画计数：%v", h2.texts())
	}
}

// ── ② loading：ongoing 14px + 到底转不转 ────────────────────────────

// TestVR3JobsOngoingSpinnerFrozen 验 14px 的口径与"转不转"。
//
// 官方三处一起看：
//
//	`tmp/official/dsh-client-ui-jobs/client.js` 的 StateDot 调用点给 ongoing 不给 size →
//	  默认 14（`index-ghOpC9uV.js:129` `n = size ?? (ongoing ? 14 : 10)`）；
//	`index-CSAdOBx6.css:1` `._spinnerMotion_1i3xo_42{transform-origin:center;
//	  animation:_dsh-state-dot-spin_1i3xo_1 1.5s linear infinite}`；
//	同文件 `@keyframes _dsh-state-dot-spin_1i3xo_1{to{transform:rotate(360deg)}}`。
//
// 判据：真窗口的动画时钟 `Transitioning()` 报告"上一帧有没有要下一帧"。
// 官方那个 infinite 动画会**一直**要帧；静态弧不会——这就是"没有在转"的机读证据，
// 不靠肉眼。顺带用像素钉住"14px 的弧没被裁"。
func TestVR3JobsOngoingSpinnerFrozen(t *testing.T) {
	h := vr3OpenJobs(t, vr3Dir(t), 900, 700, 900,
		[]pluginapi.Job{{ID: "job-1", SessionID: "s-1", Command: "npm test"}}, nil)

	before := h.shot("jobs-ongoing-trigger.png")
	// 官方动画 1.5s 一轮；推 750ms（半圈）再看画面。
	if err := h.session.Advance(750 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	h.pump()
	after := h.shot("jobs-ongoing-trigger-adv750.png")
	if !h.session.Transitioning() {
		// 这句话就是"没在转"：官方 `.spinnerMotion` 是 infinite 动画，
		// 真转的话渲染器此时会要下一帧。
		t.Errorf("推 750ms 后渲染器没有要求下一帧：运行中的 StateDot 是静止的，" +
			"官方 `._spinnerMotion_1i3xo_42{animation:... 1.5s linear infinite}`" +
			"（index-CSAdOBx6.css:1）要求它一直转")
	}
	if diff := vr3PixelDiff(before, after); diff != 0 {
		t.Logf("VR3_SPINNER_FRAME_DIFF=%d（转了的话这里应持续非零）", diff)
	}

	// 14px 的活动槽：官方 `.activitySlot{width:14px;height:14px}`。
	// 本仓把 spinner 画在触发器里第一个 14×14 的盒子里（official_jobs.go:188-196）。
	hit, ok := h.hit("jobs-trigger")
	if !ok {
		t.Fatalf("找不到触发器命中区")
	}
	slot := gfx.Rect{X: hit.Bounds.X + 2, Y: hit.Bounds.Y + (hit.Bounds.Height-14)/2, Width: 14, Height: 14}
	box, count := vr3Ink(before, slot, vr3Sample(before, 1, 1))
	if count == 0 {
		t.Fatalf("触发器活动槽 %+v 里没有墨迹：ongoing 的 spinner 没画出来", slot)
	}
	if box.Width > 14.5 || box.Height > 14.5 {
		t.Fatalf("spinner 墨迹 %+v 超出 14px 活动槽 %+v：被裁或被撑大（官方 .activitySlot 14×14；"+
			"ongoing 默认 size 14，index-ghOpC9uV.js:129）", box, slot)
	}
	if box.Width < 8 || box.Height < 8 {
		t.Fatalf("spinner 墨迹只有 %+v：14px 的环（r=9.5/24 viewBox、stroke=2）画不出来这么小", box)
	}
	t.Logf("VR3_SPINNER_INK box=%+v count=%d slot=%+v", box, count, slot)
}

// ── ③ 弹层正文：官方 `height:fit-content` 的对照 ────────────────────

// TestVR3JobsPanelBodyKeepsContentHeight 验弹层**真的长得下它的正文**。
//
// 官方三件套（rc.2 asar，`@deepseek-ai/dsh-client-ui-jobs` 的 JobListAction.module.css）：
//
//	`.Ag-ohG_menu{…width:500px;max-width:min(560px,100vw - 32px);
//	   max-height:min(480px,100dvh - 140px);height:fit-content;…overflow:hidden}`
//	`.Ag-ohG_menuBody{flex-direction:column;gap:1px;min-height:0;padding:4px;
//	   display:flex;overflow:auto}`
//	`.Ag-ohG_row{…min-height:28px;…padding:4px 7px;…}`
//
// 三者合起来的含义只有一条：菜单高 = **正文内容高**（fit-content），只有超过
// 480/视口才被裁。一行 28px + 上下 padding 4+4 = 36 是最低下界。
//
// 本仓 `drawPanel` 把正文放进 `ui.Scroll(...).Grow(1)`（official_jobs.go:272-273），
// 而弹层是个**只有 MaxHeight、没有 Height** 的 `ui.Column`（:257-258）。
// viva 的 flex 里 `Grow(1)` 需要确定的主轴尺寸；MaxHeight 只是上限，
// 于是正文拿到 0 高，弹层塌成只剩 padding。
func TestVR3JobsPanelBodyKeepsContentHeight(t *testing.T) {
	dir := vr3Dir(t)
	h := vr3OpenJobs(t, dir, 900, 700, 900, []pluginapi.Job{
		{ID: "job-1", SessionID: "s-1", Command: "npm test"},
		{ID: "job-2", SessionID: "s-1", Command: "go build", Done: true, Bytes: 4096},
	}, nil)
	trigger, ok := h.hit("jobs-trigger")
	if !ok {
		t.Fatalf("找不到触发器命中区")
	}
	h.clickHit(trigger)
	if !h.p.isOpen() {
		t.Fatalf("点触发器没打开弹层")
	}
	menu, ok := h.hit("jobs-menu")
	if !ok {
		t.Fatalf("弹层没有命中区；hits=%v", h.hitLabels())
	}
	h.shot("jobs-panel-open-900x700.png")

	// 至少要有「1 行 28px + 上下 4+4 padding」这么高；官方 height:fit-content
	// 下这里是内容高，不是 0 也不是 8。
	minHeight := float32(2*jobsMenuPad + jobsRowMinHeight)
	if menu.Bounds.Height < minHeight {
		t.Errorf("弹层高 %v，连一行 28px + padding 4×2 = %v 都不到：正文塌了。"+
			"官方 `.Ag-ohG_menu{height:fit-content}` + `.Ag-ohG_menuBody{padding:4px;overflow:auto}`"+
			" + `.Ag-ohG_row{min-height:28px}`（rc.2 asar / dsh-client-ui-jobs JobListAction.module.css）",
			menu.Bounds.Height, minHeight)
	}
	// 后果一：行根本点不到（没有命中区）。
	if _, ok := h.hit("jobs-row-s-1\x00job-1"); !ok {
		t.Errorf("活任务行没有命中区（弹层 %+v）：用户点不到这一行，展开不了输出", menu.Bounds)
	}
	// 后果二：已结束段头也画在塌掉的正文里，同样点不到。
	if _, ok := h.hit("jobs-settled-toggle"); !ok {
		t.Errorf("已结束段折叠钮没有命中区（弹层 %+v）", menu.Bounds)
	}
	t.Logf("VR3_JOB_PANEL menu=%+v texts=%v hits=%v", menu.Bounds, h.texts(), h.hitLabels())
}

// ── ④ 288px 输出区（在可达的侧栏清单上验官方上限）───────────────────

// TestVR3JobsOutputScroll288 验长输出在 288px 里滚、滚到底切得干净。
//
// 官方 `JobListAction.module.css`（rc.2 asar）：
//
//	`.Ag-ohG_panel{--dsl-terminal-command-whitespace:pre-wrap;
//	   --dsl-terminal-line-whitespace:pre-wrap;
//	   --dsl-terminal-output-max-height:288px;margin:0 8px 8px}`
//	`.Ag-ohG_panel [data-terminal]{--dsl-terminal-gutter:12px}`
//
// 官方弹层那条路径这一轮**点不到**（见上一个用例：正文塌陷），所以这里在
// 侧栏清单这条**可达**路径上验同一组数值：`drawOutput` 与
// `drawOutputWithHeight` 共用 `drawOutputBlock`（jobs.go:373 与 official_jobs.go:529），
// 288 与 12 是同一份常量，验一边就是验实现。
func TestVR3JobsOutputScroll288(t *testing.T) {
	dir := vr3Dir(t)
	lines := make([]string, 0, 80)
	for i := 0; i < 80; i++ {
		lines = append(lines, fmt.Sprintf("line %02d of the long job output", i))
	}
	body := strings.Join(lines, "\n")
	h := vr3OpenLegacy(t, dir, 280, 700,
		[]pluginapi.Job{{ID: "job-1", SessionID: "s-1", Command: "npm test"}},
		func(sessionID, id string, cursor int) (pluginapi.JobOutput, bool) {
			return pluginapi.JobOutput{ID: id, SessionID: sessionID, Output: body, Total: len(body)}, true
		})
	row, ok := h.hit("job-row-s-1\x00job-1")
	if !ok {
		t.Fatalf("侧栏清单没有行命中区；texts=%v hits=%v", h.texts(), h.hitLabels())
	}
	h.clickHit(row)
	if !h.hasText("line 00 of the long job output") {
		t.Fatalf("展开后没画输出；texts=%v", h.texts())
	}
	h.shot("jobs-legacy-output-top.png")

	// ① 输出正文被一个高 288 的裁剪盒裁住。
	clip, ok := vr3OutputClip(h.session.Snapshot())
	if !ok {
		t.Fatalf("输出正文没有任何裁剪盒：官方 `--dsl-terminal-output-max-height:288px`" +
			"（JobListAction.module.css，rc.2 asar）要求它在 288px 里滚")
	}
	if want := float32(jobOutputMaxH); clip.Height != want {
		t.Fatalf("输出滚动盒高 %v，官方 `--dsl-terminal-output-max-height:288px`", clip.Height)
	}

	// ② 「复制」按钮排在 288 盒下面，不许压在输出上。
	copyHit, ok := h.hit("job-btn-s-1\x00job-1-copy")
	if !ok {
		t.Fatalf("有输出时应画「复制」；texts=%v", h.texts())
	}
	if copyHit.Bounds.Y < clip.Y+clip.Height-0.5 {
		t.Fatalf("「复制」按钮 y=%v 压在输出区（%v..%v）上", copyHit.Bounds.Y, clip.Y, clip.Y+clip.Height)
	}

	// ③ 真滚到底：最后一行必须是第 79 行，且完整落在裁剪盒里（不留半个字）。
	//
	// viva 的符号约定：**正的 DeltaY 揭示下方内容**（`ui/scroll.go:187-189`
	// "a positive delta reveals content below"）；负值在顶部是 0 位移。
	h.move(clip.X+clip.Width/2, clip.Y+clip.Height/2)
	for range 12 {
		if err := h.session.Scroll(gfx.ScrollEvent{
			X: clip.X + clip.Width/2, Y: clip.Y + clip.Height/2, DeltaY: 300, Precise: true,
		}); err != nil {
			t.Fatal(err)
		}
		h.pump()
	}
	img := h.shot("jobs-legacy-output-bottom.png")

	last, lastBounds := "", gfx.Rect{}
	for _, text := range h.session.Snapshot().Texts {
		if !strings.HasPrefix(text.Value, "line ") || text.Bounds.Y < clip.Y-0.5 {
			continue
		}
		if last == "" || text.Bounds.Y > lastBounds.Y {
			last, lastBounds = text.Value, text.Bounds
		}
	}
	if last == "" {
		t.Fatalf("滚到底之后输出区里一行都没有")
	}
	if !strings.HasPrefix(last, "line 79") {
		t.Fatalf("滚到底后最后可见行是 %q，应为 line 79：内容没滚完（盒高 %v）", last, clip.Height)
	}
	if bottom := lastBounds.Y + lastBounds.Height; bottom > clip.Y+clip.Height+0.5 {
		t.Fatalf("最后一行 %q 的底 %v 越过裁剪盒底 %v", last, bottom, clip.Y+clip.Height)
	}
	if _, count := vr3Ink(img, gfx.Rect{X: clip.X, Y: clip.Y + clip.Height - 1, Width: clip.Width, Height: 2},
		vr3Sample(img, int(clip.X)+2, int(clip.Y)+2)); count > 0 {
		t.Logf("VR3_OUTPUT_EDGE ink=%d（裁剪盒底边 2px 内的字迹，允许是最后一行的下沿）", count)
	}
	t.Logf("VR3_OUTPUT_CLIP=%+v last=%q bounds=%+v copy=%+v", clip, last, lastBounds, copyHit.Bounds)
}

// vr3OutputClip 找出"裁住输出正文"的那个裁剪盒（正文行都以 line NN 开头）。
func vr3OutputClip(snapshot app.Snapshot) (gfx.Rect, bool) {
	seen := map[gfx.Rect]int{}
	for _, text := range snapshot.Texts {
		if !text.HasClip || !strings.HasPrefix(text.Value, "line ") {
			continue
		}
		seen[text.ClipOf]++
	}
	best, bestN := gfx.Rect{}, 0
	for rect, n := range seen {
		if n > bestN {
			best, bestN = rect, n
		}
	}
	return best, bestN > 0
}

// ── ⑤ hover（侧栏清单是可点到的那一档）──────────────────────────────

// TestVR3JobsLegacyRowHover 验行 hover 底色。
//
// 官方 `@media (hover:hover){.Ag-ohG_row:hover{background:var(--dsw-alias-fill-l1)}}`
// 与 `@media (hover:hover){.Ag-ohG_rowLineLive:hover{background:color-mix(in srgb,
// var(--dsw-alias-label-primary) 9%,transparent)}}`（rc.2 asar）。
// 判据只有一条：指针进入行之后**行上的像素要变**。零变化 = 没有 hover 态。
func TestVR3JobsLegacyRowHover(t *testing.T) {
	dir := vr3Dir(t)
	h := vr3OpenLegacy(t, dir, 280, 700, []pluginapi.Job{
		{ID: "job-1", SessionID: "s-1", Command: "npm test"},
		{ID: "job-2", SessionID: "s-1", Command: "go build", Done: true, Bytes: 2048},
	}, nil)
	row, ok := h.hit("job-row-s-1\x00job-2")
	if !ok {
		t.Fatalf("找不到已结束行命中区；texts=%v hits=%v", h.texts(), h.hitLabels())
	}
	h.move(276, 690)
	rest := h.shot("jobs-legacy-row-rest.png")
	h.move(row.Bounds.X+row.Bounds.Width/2, row.Bounds.Y+row.Bounds.Height/2)
	hover := h.shot("jobs-legacy-row-hover.png")
	diff := vr3PixelDiff(rest, hover)
	if diff <= 0 {
		t.Errorf("指针进入行 %+v 后画面零变化：没有 hover 底色。官方 "+
			"`.Ag-ohG_row:hover{background:var(--dsw-alias-fill-l1)}`"+
			"（rc.2 asar / dsh-client-ui-jobs JobListAction.module.css）", row.Bounds)
	}
	t.Logf("VR3_JOB_ROW_HOVER diff=%d row=%+v", diff, row.Bounds)
}

// TestVR3JobsLegacyStopHover 验停止钮 hover。
//
// 官方 `@media (hover:hover){.Ag-ohG_stop:hover{border-color:color-mix(in srgb,
// var(--dsw-alias-state-error-primary) 45%,transparent);color:var(--dsw-alias-
// state-error-primary)}}`（rc.2 asar）：指针在停止钮上时描边和图标换成 error 色。
func TestVR3JobsLegacyStopHover(t *testing.T) {
	dir := vr3Dir(t)
	h := vr3OpenLegacy(t, dir, 280, 700, []pluginapi.Job{
		{ID: "job-1", SessionID: "s-1", Command: "npm test"},
	}, nil)
	stop, ok := h.hit("job-stop-s-1\x00job-1")
	if !ok {
		t.Fatalf("运行中的行没有停止钮；texts=%v hits=%v", h.texts(), h.hitLabels())
	}
	h.move(276, 690)
	rest := h.shot("jobs-legacy-stop-rest.png")
	h.move(stop.Bounds.X+stop.Bounds.Width/2, stop.Bounds.Y+stop.Bounds.Height/2)
	hover := h.shot("jobs-legacy-stop-hover.png")
	if diff := vr3PixelDiff(rest, hover); diff <= 0 {
		t.Errorf("指针进入停止钮 %+v 后画面零变化：没有 hover 态。官方 "+
			"`@media(hover:hover){.Ag-ohG_stop:hover{border-color:color-mix(in srgb,"+
			"var(--dsw-alias-state-error-primary) 45%%,transparent);"+
			"color:var(--dsw-alias-state-error-primary)}}`（rc.2 asar）", stop.Bounds)
	}
}

// ── ⑥ 窄宽：compact 断点与 900 宽窗口的右缘 ─────────────────────────

// TestVR3JobsCompactBreakpointAnd900Window 验 460px 断点与右缘 12px。
//
// 官方（rc.2 asar，dsh-client-ui-jobs JobListAction.module.css）：
//
//	`.Ag-ohG_trigger{min-height:28px;padding:3px 2px;gap:4px;font-size:12px;line-height:18px}`
//	`@container (width<=460px){.Ag-ohG_trigger{box-sizing:border-box;width:28px;
//	   height:28px;padding:0} … .Ag-ohG_count{display:none}}`
//	`.Ag-ohG_activitySlot{width:14px;height:14px}`；compact 下 `.activitySlot>svg{15px}`
//	`.Ag-ohG_menu{width:500px;max-width:min(560px,100vw - 32px);
//	   max-height:min(480px,100dvh - 140px);right:12px;top:max(12px,anchor(bottom)+5px)}`
//
// 判据：断点是闭区间（`width<=460px` → 460 也在内），两侧命中区尺寸必须一个是
// 28×28、一个是内容宽；900 宽窗口下菜单右缘落在 888。
func TestVR3JobsCompactBreakpointAnd900Window(t *testing.T) {
	dir := vr3Dir(t)
	jobs := []pluginapi.Job{{ID: "job-1", SessionID: "s-1", Command: "npm test"}}

	// 460 → compact：28×28 纯图标，连计数都不画。
	narrow := vr3OpenJobs(t, dir, 900, 700, jobsTriggerCompactBreakpoint, jobs, nil)
	narrowHit, ok := narrow.hit("jobs-trigger")
	if !ok {
		t.Fatalf("compact 下没有触发器命中区")
	}
	if narrowHit.Bounds.Width != jobsTriggerCompactSize || narrowHit.Bounds.Height != jobsTriggerCompactSize {
		t.Fatalf("容器宽 %v（断点值）时触发器 = %v×%v，官方 compact 段是 28×28"+
			"（`@container (width<=460px)`）",
			jobsTriggerCompactBreakpoint, narrowHit.Bounds.Width, narrowHit.Bounds.Height)
	}
	if narrow.hasText("1 个后台任务") {
		t.Fatalf("compact 下仍画计数：%v（官方 `.count{display:none}`）", narrow.texts())
	}
	narrow.shot("jobs-compact-460.png")

	// 461 → 非 compact：回到内容宽 + 计数。
	wide := vr3OpenJobs(t, dir, 900, 700, jobsTriggerCompactBreakpoint+1, jobs, nil)
	wideHit, ok := wide.hit("jobs-trigger")
	if !ok {
		t.Fatalf("非 compact 下没有触发器命中区")
	}
	if wideHit.Bounds.Width <= jobsTriggerCompactSize {
		t.Fatalf("容器宽 %v（刚过断点）时触发器仍只有 %v 宽：断点提前触发了",
			jobsTriggerCompactBreakpoint+1, wideHit.Bounds.Width)
	}
	if !wide.hasText("1 个后台任务") {
		t.Fatalf("非 compact 下没画计数：%v", wide.texts())
	}
	if wideHit.Bounds.Height < jobsTriggerMinHeight {
		t.Fatalf("触发器高 %v，官方 `.trigger{min-height:28px}`", wideHit.Bounds.Height)
	}

	// 900×700 窗口：菜单右缘 = 900-12 = 888，宽 500。
	trigger, _ := wide.hit("jobs-trigger")
	wide.clickHit(trigger)
	menu, ok := wide.hit("jobs-menu")
	if !ok {
		t.Fatalf("菜单没有命中区")
	}
	if got, want := menu.Bounds.X+menu.Bounds.Width, float32(900-jobsMenuRight); got != want {
		t.Fatalf("菜单右缘 = %v，官方 `.Ag-ohG_menu{right:12px}` 在 900 宽窗口下应为 %v（菜单 %+v）",
			got, want, menu.Bounds)
	}
	if menu.Bounds.Width != jobsMenuWidth {
		t.Fatalf("菜单宽 %v，官方 `.Ag-ohG_menu{width:500px}`", menu.Bounds.Width)
	}
	if menu.Bounds.Height > jobsMenuMaxHeight+0.5 {
		t.Fatalf("菜单高 %v 超过官方 `max-height:min(480px,100dvh - 140px)` 的 480", menu.Bounds.Height)
	}
	wide.shot("jobs-900x700-open.png")
	t.Logf("VR3_JOBS_MENU bounds=%+v right=%v compact=%v", menu.Bounds,
		menu.Bounds.X+menu.Bounds.Width, narrowHit.Bounds)
}

// ── 侧栏清单（旧三栏档）现场 ────────────────────────────────────────

// vr3OpenLegacy 起一台只画侧栏清单的窗口（jobs.go 的 `draw`）。
//
// 为什么要单独一台：官方弹层这条路线上这一轮点不到任何行（正文塌陷），
// 而 288px 输出区与 hover 都必须在**能点到**的表面上验。
func vr3OpenLegacy(t *testing.T, dir string, windowW, windowH int,
	jobs []pluginapi.Job, output func(string, string, int) (pluginapi.JobOutput, bool)) *vr3Jobs {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("jobs-vr3-legacy")
	p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
	act := &fakeJobs{jobs: jobs, output: output}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	h := &vr3Jobs{t: t, p: p, act: act, theme: theme, tree: tree, feat: feat,
		dir: dir, containerW: float32(windowW)}
	config := app.Config{
		Title: "jobs-vr3-legacy", Width: windowW, Height: windowH, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
		OnScroll: view.Wheel,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width, Act: act.actions(),
		}
		env.Act.Invalidate = invalidator.Invalidate
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			p.draw(f, env)
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless.Open(%dx%d) 失败：%v", windowW, windowH, err)
	}
	t.Cleanup(session.Close)
	h.session = session
	h.pump()
	h.pump()
	return h
}
