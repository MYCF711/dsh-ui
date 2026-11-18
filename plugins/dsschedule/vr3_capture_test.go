//go:build darwin || windows || linux

// vr3_capture_test.go —— 第三轮真实渲染审计：定时任务（官方 ui-schedule）的
// **空态 / 长列表 / 窄宽溢出 / hover 面**。
//
// 先说清这一档的形态，否则下面的判据会被误读：
//
//	官方 = 会话 header 的 28×28 图标触发器 + 336px 目录弹层
//	       （rc.2 asar，`@deepseek-ai/dsh-client-ui-schedule` 的
//	        ScheduleCatalogAction.module.css：`.trigger{width:28px;height:28px;
//	        border-radius:28px}`:12-23、`.menu{width:336px;height:fit-content;
//	        max-height:min(420px,100vh - 140px);padding:3px;overflow:auto}`:43-64、
//	        `.row{min-height:48px;padding:10px 12px;border-radius:10px}`:76-85）。
//	本仓 = 侧栏底部的一段只读清单（`plugins/dsschedule/schedule.go`，登记为
//	       `sidebar.footer`，derived）。**没有触发器、没有弹层、没有开关**——
//	       包注释写明"它只做看见"：宿主 `schedule.*` 只有 list，建/删/暂停都没有
//	       Transport 方法，画一个点不动的开关就是死按钮。
//
// 所以这一轮只验**它自己承诺的那些**能不能站住，以及"空/长/窄"三种极端下
// 会不会画出格；官方 336px 弹层那一套几何作为对照写在报告里，不当断言用：
// 拿不到官方 surface 却断言它的数值，是拿装饰当验收。
//
// 截图落 `$VR3_CAPTURE_DIR`（未设置时用临时目录）。
package dsschedule

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// ── 落图与像素小工具（vr3 前缀防撞名）───────────────────────────────

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

func vr3Sample(img *image.RGBA, x, y int) color.RGBA {
	if img == nil {
		return color.RGBA{}
	}
	return img.RGBAAt(x, y)
}

func vr3AbsDiff(a, b uint8) int {
	if a > b {
		return int(a) - int(b)
	}
	return int(b) - int(a)
}

// vr3Ink 数 rect 里与 bg 不同的像素并给出墨迹包围盒。
func vr3Ink(img *image.RGBA, rect gfx.Rect, bg color.RGBA) (box gfx.Rect, count int) {
	if img == nil || rect.Width <= 0 || rect.Height <= 0 {
		return gfx.Rect{}, 0
	}
	minX, minY := float32(1e9), float32(1e9)
	maxX, maxY := float32(-1e9), float32(-1e9)
	bounds := img.Bounds()
	for y := int(rect.Y); y < int(rect.Y+rect.Height); y++ {
		for x := int(rect.X); x < int(rect.X+rect.Width); x++ {
			if x < bounds.Min.X || y < bounds.Min.Y || x >= bounds.Max.X || y >= bounds.Max.Y {
				continue
			}
			p := img.RGBAAt(x, y)
			if vr3AbsDiff(p.R, bg.R) <= 6 && vr3AbsDiff(p.G, bg.G) <= 6 && vr3AbsDiff(p.B, bg.B) <= 6 {
				continue
			}
			count++
			fx, fy := float32(x), float32(y)
			if fx < minX {
				minX = fx
			}
			if fx > maxX {
				maxX = fx
			}
			if fy < minY {
				minY = fy
			}
			if fy > maxY {
				maxY = fy
			}
		}
	}
	if count == 0 {
		return gfx.Rect{}, 0
	}
	return gfx.Rect{X: minX, Y: minY, Width: maxX - minX + 1, Height: maxY - minY + 1}, count
}

// ── 真窗口现场 ──────────────────────────────────────────────────────

// vr3Schedule 是一台只画侧栏底部清单的真窗口。
type vr3Schedule struct {
	t       *testing.T
	p       *Plugin
	items   []pluginapi.ScheduleItem
	theme   *pluginapi.Theme
	tree    *gfx.Tree
	feat    gfx.Scope
	session *headless.Session
	dir     string
	// footerW/footerH 是侧栏底栏这个容器的尺寸（官方侧栏 280 宽，底栏高度随内容）。
	footerW float32
	footerH float32
}

func vr3OpenSchedule(t *testing.T, dir string, windowW, windowH int, footerW, footerH float32,
	items []pluginapi.ScheduleItem) *vr3Schedule {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("schedule-vr3")
	p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	h := &vr3Schedule{t: t, p: p, items: items, theme: theme, tree: tree, feat: feat,
		dir: dir, footerW: footerW, footerH: footerH}
	config := app.Config{
		Title: "schedule-vr3", Width: windowW, Height: windowH, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
		OnScroll: view.Wheel,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width,
			Act: pluginapi.Actions{
				ScheduleList: func() []pluginapi.ScheduleItem { return items },
				Invalidate:   invalidator.Invalidate,
			},
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			// 侧栏底栏：左贴边、底贴边、定宽定高（壳里就是这一块）。
			ui.Row(f).Absolute().Left(0).Bottom(0).Width(footerW).Height(footerH).Children(func() {
				p.draw(f, env)
			})
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

func (h *vr3Schedule) pump() {
	h.t.Helper()
	if err := h.session.Driver().Pump(); err != nil {
		h.t.Fatal(err)
	}
}

func (h *vr3Schedule) shot(name string) *image.RGBA {
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

func (h *vr3Schedule) texts() []string {
	snapshot := h.session.Snapshot()
	out := make([]string, 0, len(snapshot.Texts))
	for _, text := range snapshot.Texts {
		out = append(out, text.Value)
	}
	return out
}

func (h *vr3Schedule) hasText(want string) bool {
	for _, value := range h.texts() {
		if value == want {
			return true
		}
	}
	return false
}

// footerRect 是底栏容器的窗口矩形（左下角贴边）。
func (h *vr3Schedule) footerRect() gfx.Rect {
	return gfx.Rect{X: 0, Y: float32(700) - h.footerH, Width: h.footerW, Height: h.footerH}
}

// vr3ScheduleItems 造 n 条定时任务（标题 + 下次时间都是宿主真字段）。
func vr3ScheduleItems(n int) []pluginapi.ScheduleItem {
	out := make([]pluginapi.ScheduleItem, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, pluginapi.ScheduleItem{
			ID:     fmt.Sprintf("sched-%02d", i),
			Title:  fmt.Sprintf("每日巡检 %02d", i),
			Kind:   "daily",
			NextAt: fmt.Sprintf("2026-11-%02dT09:%02d:00+08:00", (i%28)+1, i%60),
		})
	}
	return out
}

// ── ① 空态 ──────────────────────────────────────────────────────────

// TestVR3ScheduleEmptyDrawsNothing 验"没有定时任务时一个字都不画"。
//
// 官方（rc.2 asar，`@deepseek-ai/dsh-client-ui-schedule` 的 ScheduleCatalogAction.tsx）：
//
//	:77  `if (!visible) return null`
//	:81  `if (records.length === 0 && status !== 'error') return null`
//
// 空清单连触发器都没有，更不该留一个"定时任务"的空标题——那会让侧栏看着像坏了。
func TestVR3ScheduleEmptyDrawsNothing(t *testing.T) {
	h := vr3OpenSchedule(t, vr3Dir(t), 900, 700, 240, 300, nil)
	if got := h.texts(); len(got) != 0 {
		t.Fatalf("空清单画出了文字：%v（官方 records.length===0 → return null，ScheduleCatalogAction.tsx:81）", got)
	}
	if hits := h.session.Snapshot().Hits; len(hits) != 0 {
		t.Fatalf("空清单留下了 %d 条命中区：%+v", len(hits), hits)
	}
	img := h.shot("schedule-empty-900x700.png")
	if box, count := vr3Ink(img, h.footerRect(), vr3Sample(img, 1, 1)); count != 0 {
		t.Fatalf("空态底栏里有多余墨迹 %d 像素（bbox=%+v）：官方空清单什么都不画", count, box)
	}
}

// ── ② 长列表：条数上限 + 不许画出格 ─────────────────────────────────

// TestVR3ScheduleLongListClamp 验长列表的两件事：
//
//	① 条数上限生效：maxShown=5，超出写「还有 N 条」（schedule.go:15、:59-62、:74-76）。
//	② **整块内容不越出底栏**。官方这一格是 336px 弹层里 `overflow:auto` 的列表
//	   （ScheduleCatalogAction.module.css:59-64：`width:336px;height:fit-content;
//	   max-height:min(420px,100vh - 140px);padding:3px;overflow:auto`），
//	   越界由弹层自己滚；本仓画在侧栏底栏里，底栏高度是**固定**的，
//	   所以内容必须落在这块矩形里，否则会盖住上面的会话列表。
func TestVR3ScheduleLongListClamp(t *testing.T) {
	h := vr3OpenSchedule(t, vr3Dir(t), 900, 700, 240, 300, vr3ScheduleItems(12))
	texts := h.texts()
	if !h.hasText("还有 7 条") {
		t.Fatalf("12 条时应画「还有 7 条」（maxShown=5）；texts=%v", texts)
	}
	rows := 0
	for _, value := range texts {
		if strings.HasPrefix(value, "每日巡检 ") {
			rows++
		}
	}
	if rows != 5 {
		t.Fatalf("画了 %d 行任务，maxShown 应为 5（schedule.go:15）；texts=%v", rows, texts)
	}
	img := h.shot("schedule-long-12.png")

	// 所有字迹都必须落在底栏矩形里。
	footer := h.footerRect()
	for _, text := range h.session.Snapshot().Texts {
		b := text.Bounds
		if b.X < footer.X-0.5 || b.Y < footer.Y-0.5 ||
			b.X+b.Width > footer.X+footer.Width+0.5 || b.Y+b.Height > footer.Y+footer.Height+0.5 {
			t.Fatalf("字 %q 的矩形 %+v 越出底栏 %+v：底栏是固定高的，盖到上面的会话列表就是画错",
				text.Value, b, footer)
		}
	}
	// 更低一档：连"墨迹"（图标/描边）都不许出格。
	if box, count := vr3Ink(img, gfx.Rect{Width: 900, Height: 700}, vr3Sample(img, 1, 1)); count > 0 {
		if box.Y < footer.Y-0.5 || box.X < footer.X-0.5 ||
			box.X+box.Width > footer.X+footer.Width+0.5 || box.Y+box.Height > footer.Y+footer.Height+0.5 {
			t.Fatalf("墨迹 bbox %+v 越出底栏 %+v", box, footer)
		}
	}
	// 官方上限对照：弹层 max-height = min(420px, 100vh-140px)（:61），
	// 700 高窗口下是 420；本仓底栏 300 高，内容必须更小。
	if footer.Height > 420 {
		t.Logf("底栏高 %v 已超过官方弹层 max-height 420：这不是本仓的形状，见报告", footer.Height)
	}
}

// ── ③ 窄宽：长标题必须省略，时间不许被挤出去 ────────────────────────

// TestVR3ScheduleNarrowRowStaysInside 验 240px 宽底栏里一行两个字段的布局。
//
// 官方一行是 `.row{min-height:48px;padding:10px 12px;gap:2px}`（:76-85）里放
// `.title{font-size:13px;line-height:18px}`（:119-121）与
// `.metadata,.nextRun{font-size:10px;line-height:15px}`（:156-165）。
// 本仓把"标题 + 下次时间"压成一行（title 12px / time 11px，schedule.go:68-72），
// 几何与官方不同（那是形态差异，见报告），但**不越界**这一条两边都成立：
// 标题省略号截断、时间右缘仍在底栏内。
func TestVR3ScheduleNarrowRowStaysInside(t *testing.T) {
	items := []pluginapi.ScheduleItem{{
		ID:     "sched-1",
		Title:  "一个非常非常长的定时任务标题，长到必须被省略号截断才算对",
		Kind:   "daily",
		NextAt: "2026-11-05T09:00:00+08:00",
	}}
	h := vr3OpenSchedule(t, vr3Dir(t), 900, 700, 240, 120, items)
	img := h.shot("schedule-narrow-240.png")

	var title, when gfx.Rect
	for _, text := range h.session.Snapshot().Texts {
		switch {
		case strings.HasPrefix(text.Value, "一个非常"):
			title = text.Bounds
		case strings.HasPrefix(text.Value, "11-05 "):
			when = text.Bounds
		}
	}
	if title.Width == 0 || when.Width == 0 {
		t.Fatalf("没有同时画出标题与下次时间：texts=%v", h.texts())
	}
	// 时间槽在右、标题在左，两个框不许重叠（重叠就是挤在一起）。
	if when.X < title.X+title.Width-0.5 {
		t.Fatalf("标题框 %+v 与时间框 %+v 重叠", title, when)
	}
	if right := when.X + when.Width; right > 240+0.5 {
		t.Fatalf("时间框右缘 %v 越出 240 宽底栏", right)
	}
	if title.Width > 240 {
		t.Fatalf("标题框宽 %v 超过底栏宽 240：没有省略就溢出", title.Width)
	}
	// 像素上：底栏矩形**右边之外**的那条竖带里不许有字迹（240..260）。
	// 注意这一档没有横向 padding（schedule.go:64 只有 PadY(4)），所以字可以贴到
	// 240 的边；越过 240 才是真的出格。
	if box, count := vr3Ink(img, gfx.Rect{X: 240, Y: h.footerRect().Y, Width: 20, Height: h.footerH},
		vr3Sample(img, 1, 1)); count > 0 {
		t.Fatalf("底栏右侧之外出现 %d 像素墨迹（bbox=%+v）：内容越出了 240 宽的容器", count, box)
	}
	t.Logf("VR3_SCHEDULE_ROW title=%+v when=%+v", title, when)
}

// ── ④ 行间距：行与行不许贴死 ────────────────────────────────────────

// TestVR3ScheduleRowRhythm 验多行的纵向节距。
//
// 官方一行 min-height 48（:83），行间没有额外 gap（`.menu{gap:1px}`:58）。
// 本仓是紧凑清单（无 min-height），但行与行之间必须**有可见间隔**且**互不重叠**，
// 否则两行的字形会叠在一起——这是"看着像坏了"的典型形态。
func TestVR3ScheduleRowRhythm(t *testing.T) {
	h := vr3OpenSchedule(t, vr3Dir(t), 900, 700, 240, 300, vr3ScheduleItems(5))
	var rows []gfx.Rect
	for _, text := range h.session.Snapshot().Texts {
		if strings.HasPrefix(text.Value, "每日巡检 ") {
			rows = append(rows, text.Bounds)
		}
	}
	if len(rows) != 5 {
		t.Fatalf("应有 5 行，实际 %d：texts=%v", len(rows), h.texts())
	}
	for i := 1; i < len(rows); i++ {
		if gap := rows[i].Y - (rows[i-1].Y + rows[i-1].Height); gap < 0 {
			t.Fatalf("第 %d 行与第 %d 行重叠 %v：行距为负", i-1, i, -gap)
		}
	}
	h.shot("schedule-five-rows.png")
}
