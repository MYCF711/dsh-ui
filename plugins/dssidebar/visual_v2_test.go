package dssidebar

import (
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// v2RecordedRow 记录真实 drawSession 传给图元的选中态、行高和最终布局矩形。
type v2RecordedRow struct {
	id     gfx.ID
	active bool
	height float32
	box    ui.Box
}

// v2RecordingPrimitives 只记录、不改写 dscore 的真实布局与命中行为。
type v2RecordingPrimitives struct {
	*dscore.Primitives
	rows    []v2RecordedRow
	headers []ui.Box
	dots    int
}

func (r *v2RecordingPrimitives) ListRow(f ui.C, id gfx.ID, active bool, height float32, body func()) ui.Box {
	box := r.Primitives.ListRow(f, id, active, height, body)
	r.rows = append(r.rows, v2RecordedRow{id: id, active: active, height: height, box: box})
	return box
}

func (r *v2RecordingPrimitives) SectionHeader(f ui.C, id gfx.ID, title string, count int, open bool) ui.Box {
	box := r.Primitives.SectionHeader(f, id, title, count, open)
	r.headers = append(r.headers, box)
	return box
}

func (r *v2RecordingPrimitives) Dot(f ui.C, color ui.Color, size float32) ui.Box {
	r.dots++
	return r.Primitives.Dot(f, color, size)
}

func v2Canvas(tree *gfx.Tree) *gfx.Canvas {
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	return canvas
}

func v2Fixture() (*Plugin, *v2RecordingPrimitives, *pluginapi.Env, *gfx.Tree, *gfx.Canvas) {
	theme := dscore.Dark()
	recorder := &v2RecordingPrimitives{Primitives: &dscore.Primitives{Theme: theme}}
	tree := &gfx.Tree{}
	feature := tree.Feature("visual-v2-test")
	plugin := &Plugin{theme: theme, prims: recorder, width: 280, collapsed: map[string]bool{}}
	env := &pluginapi.Env{Theme: theme, Tree: tree, Feat: feature}
	return plugin, recorder, env, tree, v2Canvas(tree)
}

func TestV2WorkspaceRootIsNotUnknown(t *testing.T) {
	if got := workspaceKey("/"); got != "/" {
		t.Fatalf("POSIX 根工作区被归成 %q，want /（不能冒充未指定工作区）", got)
	}
}

// TestV2ActivityDotOnlyForCurrentSessionWithTruth 是活动点的边界：
// 当前会话的 Busy=true / Stream 非空都是正向真值，可以显示活动点；
// Busy=false 只表示未知（历史真机记录证明它可能漏报），不表示“空闲”。
// 其他会话没有逐会话活动字段，无论前台 Busy/Stream 如何都必须留空。
func TestV2ActivityDotOnlyForCurrentSessionWithTruth(t *testing.T) {
	draw := func(busy bool, stream, cur string) int {
		p, recorder, env, _, canvas := v2Fixture()
		env.Data = pluginapi.Snapshot{Cur: cur, Busy: busy, Stream: stream}
		env.Act.OpenSession = func(string) {}
		row := sess("current", "真实标题", `D:\work`, "2026-11-03T20:00:00+08:00")
		pageTheme := dscore.VivaTheme(p.theme)
		view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
		view.Draw(canvas, ui.Rect{Width: 268, Height: 40}, func(f ui.C) {
			p.drawSession(f, env, row)
		})
		return recorder.dots
	}

	if got := draw(true, "", "current"); got != 1 {
		t.Fatalf("Busy=true 是当前会话的正向活动真值，应有 1 个点，实际 %d 个", got)
	}
	if got := draw(false, "正在写正文", "current"); got != 1 {
		t.Fatalf("Stream 非空的当前会话行应有一个活动点，实际 %d 个", got)
	}
	if got := draw(false, "", "current"); got != 0 {
		t.Fatalf("Busy/Stream 都为空时不该画点，实际 %d 个", got)
	}
	if got := draw(true, "前台正文", "other"); got != 0 {
		t.Fatalf("不是当前会话的行不该画点（没有逐会话真值），实际 %d 个", got)
	}
}

// TestV2UnknownTimeIsHidden 防止把坏值、未来值或空值画成貌似真实的时间。
func TestV2UnknownTimeIsHidden(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{name: "缺失", in: ""},
		{name: "非法值", in: "not-a-time"},
		{name: "非 RFC3339 日期", in: "2026-11-03"},
		{name: "未来值", in: time.Now().Add(2 * time.Hour).Format(time.RFC3339)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := relativeTime(tc.in); got != "" {
				t.Fatalf("未知时间 %q 必须隐藏，实际显示 %q", tc.in, got)
			}
		})
	}
}

// TestV2SearchUsesFieldAndNarrowsRealRows 不只测比较函数：搜索词经过实际 Field，
// 再经过 drawList，最终可见行与键盘顺序必须同时缩到命中项。
func TestV2SearchUsesFieldAndNarrowsRealRows(t *testing.T) {
	p, recorder, env, _, canvas := v2Fixture()
	env.Data.Sessions = []apiclient.Session{
		sess("a", "Alpha 真实标题", `D:\work\one`, "2026-11-03T10:00:00Z"),
		sess("b", "Beta", `D:\work\two`, "2026-11-03T11:00:00Z"),
	}
	env.Data.Cur = "b"
	p.search.SetText("  ALPHA  ")
	pageTheme2 := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme2})
	view.Draw(canvas, ui.Rect{Width: 268, Height: 300}, func(f ui.C) {
		p.drawList(f, env)
	})
	if len(p.lastOrder) != 1 || p.lastOrder[0] != "a" {
		t.Fatalf("Field 搜索后的可见顺序 = %v，want [a]", p.lastOrder)
	}
	if len(recorder.rows) != 1 || recorder.rows[0].active {
		t.Fatalf("搜索后的实际绘制行 = %+v，want 仅未选中的 a", recorder.rows)
	}
}

// TestV2SelectionAndClickUseRealSessionID 验证真实图元收到的选中态，以及点击后
// 原样进入 OpenSession 的会话 ID；不是仅检查一个布尔辅助函数。
func TestV2SelectionAndClickUseRealSessionID(t *testing.T) {
	p, recorder, env, tree, canvas := v2Fixture()
	env.Data.Cur = "b"
	var opened string
	env.Act.OpenSession = func(id string) { opened = id }
	a := sess("a", "Alpha", `D:\work`, "2026-11-03T10:00:00Z")
	b := sess("b", "Beta", `D:\work`, "2026-11-03T11:00:00Z")
	pageTheme3 := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme3})
	draw := func() {
		view.Draw(canvas, ui.Rect{Width: 268, Height: 80}, func(f ui.C) {
			ui.Column(f).Children(func() {
				p.drawSession(f, env, a)
				p.drawSession(f, env, b)
			})
		})
	}
	draw()
	if len(recorder.rows) != 2 || recorder.rows[0].active || !recorder.rows[1].active {
		t.Fatalf("Cur=b 时实际选中态 = %+v", recorder.rows)
	}
	for i, row := range recorder.rows {
		if row.height != 32 || row.box.Rect().Height != 32 {
			t.Fatalf("第 %d 行高度参数/布局 = %v/%v，want 32/32", i, row.height, row.box.Rect().Height)
		}
	}
	canvas.Reset()
	canvas.BeginEventPass(tree.ID(env.Feat, "s-a"), gfx.ClickInfo{})
	draw()
	canvas.EndEventPass()
	if opened != "a" {
		t.Fatalf("点击 a 后 OpenSession = %q，want a", opened)
	}
}

// TestV2GroupGeometryAndScroll 直接量 flex 布局：组头 34、会话 32、
// 组头到首行 2、上一组末行到下一组头 4；wheel 必须移动这个列表自己的滚动区。
func TestV2GroupGeometryAndScroll(t *testing.T) {
	p, recorder, env, _, canvas := v2Fixture()
	for i := 0; i < 12; i++ {
		env.Data.Sessions = append(env.Data.Sessions,
			sess(itoa(i), "Alpha", `D:\work\a`, "2026-11-03T10:00:00Z"))
	}
	env.Data.Sessions = append(env.Data.Sessions,
		sess("b", "Beta", `D:\work\b`, "2026-11-03T11:00:00Z"))
	pageTheme4 := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme4})
	draw := func() {
		view.Draw(canvas, ui.Rect{Width: 268, Height: 180}, func(f ui.C) {
			ui.Column(f).FillHeight().Children(func() { p.drawList(f, env) })
		})
	}
	draw()
	// 最新组 b 在前：第一组头 y=0，第一行 y=36；第二组头应在第一行结束后 4px。
	if len(recorder.rows) < 2 {
		t.Fatalf("实际行数 %d，无法检查组间距", len(recorder.rows))
	}
	first, second := recorder.rows[0].box.Rect(), recorder.rows[1].box.Rect()
	if first.Y != 36 || first.Height != 32 {
		t.Fatalf("首行矩形 = %+v，want y=36 height=32", first)
	}
	if len(recorder.headers) != 2 {
		t.Fatalf("实际组头数 = %d，want 2", len(recorder.headers))
	}
	firstHeader, secondHeader := recorder.headers[0].Rect(), recorder.headers[1].Rect()
	if firstHeader.Height != 34 || secondHeader.Height != 34 {
		t.Fatalf("组头高度 = %v/%v，want 34/34", firstHeader.Height, secondHeader.Height)
	}
	if got := first.Y - (firstHeader.Y + firstHeader.Height); got != 2 {
		t.Fatalf("组头到首行间距 = %v，want 2", got)
	}
	if got := secondHeader.Y - (first.Y + first.Height); got != 4 {
		t.Fatalf("上一组末行到下一组头间距 = %v，want 4", got)
	}
	if got := second.Y - (secondHeader.Y + secondHeader.Height); got != 2 {
		t.Fatalf("第二组头到首行间距 = %v，want 2", got)
	}
	// v0.6.0：没有 ID→ScrollState 查表，滚动状态从声明期的 Box 取。
	state := p.scrollBox.ScrollState()
	if state.Content <= state.Viewport || state.Max() <= 0 {
		t.Fatalf("长列表没有形成可滚动内容：content=%v viewport=%v", state.Content, state.Viewport)
	}
	last := recorder.rows[len(recorder.rows)-1].box.Rect()
	if got := state.Content - (last.Y + last.Height); got != 16 {
		t.Fatalf("滚动内容底部留白 = %v，want 16（官方 .GAkraW_list padding-bottom）", got)
	}
	result := view.Wheel(gfx.ScrollEvent{X: 20, Y: 50, DeltaY: 24, Precise: true})
	if !result.Handled || state.Offset != 24 {
		t.Fatalf("wheel 未滚动侧栏：handled=%v offset=%v，want true/24", result.Handled, state.Offset)
	}
}
