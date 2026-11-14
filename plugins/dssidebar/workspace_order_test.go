// workspace_order_test.go —— #62 的两半：分组/手动顺序按 `workspace.registry`，
// 以及拖拽排会话序。
//
// 判据全部照官方 `dsh-client-ui-workspace`（file:line 见 grouping.go / drag.go 的头注释）：
//
//	OFF:src/client/tree.ts:26-32      owningGroupKey（名册归属，UNGROUPED_KEY）
//	OFF:src/client/tree.ts:325-367    groupByWorkspace（组序 = workspaces 顺序、
//	                                  组内序 = sessionIds、未归属收尾进「未分组」）
//	OFF:src/client/rows/WorkspaceBrowser.tsx:218-238  sessionDragOrder（落点换算）
//	OFF:src/client/rows/Rows.tsx:215-218               rowHalf（上半区/下半区）
//	OFF:src/client/rows/Rows.module.css:370-408        落点插入线几何
package dssidebar

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 纯函数层：分组与顺序 ────────────────────────────────────────────────

func ws(id, path, title string, sessionIDs ...string) pluginapi.Workspace {
	return pluginapi.Workspace{WorkspaceID: id, Path: path, Title: title, SessionIDs: sessionIDs}
}

func ids(sessions []apiclient.Session) []string {
	out := make([]string, 0, len(sessions))
	for i := range sessions {
		out = append(out, sessions[i].ID)
	}
	return out
}

func equalIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func idsIn(plan groupPlan, key string) []string {
	for i := range plan.Groups {
		if plan.Groups[i].Key == key {
			return plan.Groups[i].sessionIDs()
		}
	}
	return nil
}

// TestRegistryMembershipBeatsCwd ①：归属看 registry 的 sessionIds，不看 cwd 字符串。
//
// 三条会话里 b1 的 cwd 是 alpha、名册却在 beta：按 cwd 分组会把它分进 alpha，
// 按官方 `owningGroupKey` 必须进 beta。组序 = workspaces 数组顺序，组键 = workspaceId。
func TestRegistryMembershipBeatsCwd(t *testing.T) {
	sessions := []apiclient.Session{
		sess("a1", "A 第一条", `D:\work\alpha`, "2026-11-03T10:00:00Z"),
		sess("b1", "B 第一条", `D:\work\alpha`, "2026-11-03T11:00:00Z"),
		sess("b2", "B 第二条", `D:\work\beta`, "2026-11-03T12:00:00Z"),
	}
	workspaces := []pluginapi.Workspace{
		ws("w-a", `D:\work\alpha`, "客户项目", "a1"),
		ws("w-b", `D:\work\beta`, "另一个项目", "b1", "b2"),
	}
	plan := planGroups(sessions, workspaces, "")
	if !plan.Registry {
		t.Fatal("有 registry 时必须走 registry 分组")
	}
	if got := plan.Keys(); !equalIDs(got, []string{"w-a", "w-b"}) {
		t.Fatalf("组顺序 = %v，want registry 顺序 [w-a w-b]", got)
	}
	if got := idsIn(plan, "w-b"); !equalIDs(got, []string{"b1", "b2"}) {
		t.Fatalf("w-b 成员 = %v，want [b1 b2]（b1 的 cwd 在 alpha，但名册在 beta）", got)
	}
	if got := idsIn(plan, "w-a"); !equalIDs(got, []string{"a1"}) {
		t.Fatalf("w-a 成员 = %v，want [a1]", got)
	}
	if plan.Groups[0].Label != "客户项目" || plan.Groups[1].Label != "另一个项目" {
		t.Fatalf("组头文案 = %q/%q，want registry 标题", plan.Groups[0].Label, plan.Groups[1].Label)
	}
	for i := range plan.Groups {
		if plan.Groups[i].Key == ungroupedKey {
			t.Fatal("所有会话都在名册里，不该有「未分组」组")
		}
		if plan.Groups[i].Workspace == nil {
			t.Fatalf("组 %q 丢了工作区身份（改名/删除/拖拽都要它）", plan.Groups[i].Key)
		}
	}
	// 反向对照：同一个 cwd 的两条会话在 cwd 分组里是**同一组**——证明两条路径真的不同。
	if _, cwdOrder := groupByWorkspace(sessions, ""); len(cwdOrder) != 2 {
		t.Fatalf("cwd 分组组数 = %v，want 2（alpha/beta）", cwdOrder)
	}
}

// TestRegistryGroupOrderIsManualSessionIDs ②：组内顺序 == sessionIds 的手动顺序，
// **不**按 updatedAt 重排（官方 tree.ts:337-343 按名册顺序 push）。
func TestRegistryGroupOrderIsManualSessionIDs(t *testing.T) {
	sessions := []apiclient.Session{
		sess("newest", "最新", `D:\w`, "2026-11-03T20:00:00Z"),
		sess("oldest", "最旧", `D:\w`, "2026-11-01T08:00:00Z"),
		sess("mid", "中间", `D:\w`, "2026-11-02T09:00:00Z"),
	}
	workspaces := []pluginapi.Workspace{ws("w", `D:\w`, "工作区", "oldest", "mid", "newest")}
	plan := planGroups(sessions, workspaces, "")
	if got := idsIn(plan, "w"); !equalIDs(got, []string{"oldest", "mid", "newest"}) {
		t.Fatalf("组内顺序 = %v，want 名册顺序 [oldest mid newest]", got)
	}
	// 对照：cwd 回退那条路是按最近更新降序的（newest→mid→oldest），两者必须不一样。
	cwdGroups, _ := groupByWorkspace(sessions, "")
	if got := ids(cwdGroups[`D:\w`]); !equalIDs(got, []string{"newest", "mid", "oldest"}) {
		t.Fatalf("cwd 回退组内顺序 = %v，want 按 updatedAt 降序", got)
	}
}

// TestRegistrySkipsMembersWithoutSummary 覆盖官方 tree.ts:339 的那一支：
// 名册里领先于会话列表的成员先跳过、等它到货，且**不会被算成未归属**。
func TestRegistrySkipsMembersWithoutSummary(t *testing.T) {
	sessions := []apiclient.Session{sess("a", "A", `D:\w`, "2026-11-03T10:00:00Z")}
	workspaces := []pluginapi.Workspace{ws("w", `D:\w`, "工作区", "not-yet", "a")}
	plan := planGroups(sessions, workspaces, "")
	if got := idsIn(plan, "w"); !equalIDs(got, []string{"a"}) {
		t.Fatalf("到货的成员 = %v，want [a]", got)
	}
	if len(plan.Groups) != 1 {
		t.Fatalf("组数 = %d，want 1（没到货的那条不该另起一组）", len(plan.Groups))
	}
}

// TestUnaccountedSessionsGoToUngrouped ③：不属于任何工作区的会话进「未分组」，
// 组排在最后、组头用官方 `group.ungrouped`、组内按最近更新降序（官方
// tree.ts:352-365 + orderedUngrouped 的 recency 分支）。
func TestUnaccountedSessionsGoToUngrouped(t *testing.T) {
	sessions := []apiclient.Session{
		sess("in-ws", "名册里的", `D:\w`, "2026-11-03T09:00:00Z"),
		sess("stray-old", "没人管的旧", `D:\elsewhere`, "2026-11-01T08:00:00Z"),
		sess("stray-new", "没人管的新", `D:\elsewhere`, "2026-11-03T20:00:00Z"),
	}
	workspaces := []pluginapi.Workspace{ws("w", `D:\w`, "工作区", "in-ws")}
	plan := planGroups(sessions, workspaces, "")
	if got := plan.Keys(); !equalIDs(got, []string{"w", ungroupedKey}) {
		t.Fatalf("组顺序 = %v，want 工作区在前、「未分组」垫底", got)
	}
	last := plan.Groups[len(plan.Groups)-1]
	if last.Key != ungroupedKey || last.Label != "未分组" {
		t.Fatalf("末组 = %q/%q，want 空键/官方 group.ungrouped「未分组」", last.Key, last.Label)
	}
	if last.Workspace != nil {
		t.Fatal("「未分组」没有工作区身份：不该给它能改名/删除/拖拽的假身份")
	}
	if got := last.sessionIDs(); !equalIDs(got, []string{"stray-new", "stray-old"}) {
		t.Fatalf("「未分组」顺序 = %v，want 按最近更新降序", got)
	}
}

// TestEmptyRegistryFallsBackToCwd ④：`Workspaces` 空 = 宿主没有 registry 或还没拉到
// （pluginapi.Snapshot 的字段语义），必须**原样**退回 cwd 分组——老宿主/mockd 上
// 这是唯一诚实的结果，不能把空当成"用户没有任何工作区"。
func TestEmptyRegistryFallsBackToCwd(t *testing.T) {
	sessions := []apiclient.Session{
		sess("a", "A", `D:\work\a`, "2026-11-03T10:00:00Z"),
		sess("b", "B", `D:\work\b`, "2026-11-03T11:00:00Z"),
		sess("c", "C", `D:\work\a`, "2026-11-03T12:00:00Z"),
		sess("d", "没有 cwd", "", "2026-11-03T09:00:00Z"),
	}
	wantGroups, wantOrder := groupByWorkspace(sessions, "")
	plan := planGroups(sessions, nil, "")
	if plan.Registry {
		t.Fatal("registry 为空时必须走 cwd 回退")
	}
	if got := plan.Keys(); !equalIDs(got, wantOrder) {
		t.Fatalf("组顺序 = %v，want %v（与 groupByWorkspace 完全一致）", got, wantOrder)
	}
	for i := range plan.Groups {
		group := plan.Groups[i]
		if got, want := group.sessionIDs(), ids(wantGroups[group.Key]); !equalIDs(got, want) {
			t.Fatalf("组 %q 成员 = %v，want %v", group.Key, got, want)
		}
		if group.Workspace != nil {
			t.Fatalf("组 %q 在 cwd 回退里没有工作区身份", group.Key)
		}
	}
	// 空 cwd 仍然归「未指定工作区」那一档（回退路径的措辞**不回退**，
	// 与官方 UNGROUPED_KEY 那组的「未分组」是两套）。
	if got := idsIn(plan, ""); !equalIDs(got, []string{"d"}) {
		t.Fatalf("空 cwd 的组 = %v，want [d]", got)
	}
	for i := range plan.Groups {
		if plan.Groups[i].Key == "" && plan.Groups[i].Label != "未指定工作区" {
			t.Fatalf("cwd 回退的空键组头 = %q，want「未指定工作区」（回退措辞不变）",
				plan.Groups[i].Label)
		}
	}
}

// TestOwningGroupKeyFollowsFirstWorkspace 钉住官方 owningGroupKey 的"取第一条"语义。
func TestOwningGroupKeyFollowsFirstWorkspace(t *testing.T) {
	workspaces := []pluginapi.Workspace{
		ws("w1", `D:\w1`, "一", "x"),
		ws("w2", `D:\w2`, "二", "x", "y"),
	}
	if got := owningGroupKey(workspaces, "x"); got != "w1" {
		t.Fatalf("重复归属取第一条：got %q want w1", got)
	}
	if got := owningGroupKey(workspaces, "y"); got != "w2" {
		t.Fatalf("got %q want w2", got)
	}
	if got := owningGroupKey(workspaces, "zzz"); got != ungroupedKey {
		t.Fatalf("没有归属 = %q，want UNGROUPED_KEY(%q)", got, ungroupedKey)
	}
}

// ── 纯函数层：落点换算 ⑤ ────────────────────────────────────────────────

// TestDropBeforeIDAnchor 覆盖落点 -> `beforeSessionId` 的换算：
// 上半区落在目标行之前、下半区落在它之后、**移到组末尾 = 空串**（宿主对"给了但是
// 空串"回 400，所以空串必须走"省略字段"那一支），以及"拖回原位不产生写"。
func TestDropBeforeIDAnchor(t *testing.T) {
	order := []string{"a", "b", "c", "d"}
	cases := []struct {
		name    string
		session string
		target  string
		after   bool
		want    string
		changed bool
	}{
		{"a 落到 c 上半区（c 之前）", "a", "c", false, "c", true},
		{"a 落到 c 下半区（c 之后）", "a", "c", true, "d", true},
		{"d 落到 a 上半区（组首）", "d", "a", false, "a", true},
		{"a 落到末行下半区 = 移到末尾", "a", "d", true, "", true},
		{"a 落到末行上半区（d 之前）", "a", "d", false, "d", true},
		{"b 落到 c 上半区 = 原位，不写", "b", "c", false, "", false},
		{"b 落到 a 下半区 = 原位，不写", "b", "a", true, "", false},
		{"c 落到 d 上半区 = 原位，不写", "c", "d", false, "", false},
		{"c 落到 d 下半区 = 移到末尾", "c", "d", true, "", true},
		{"落点 = 自己，不写", "b", "b", false, "", false},
		{"被拖的不在名册里，不写", "zzz", "a", false, "", false},
		{"落点不在名册里，不写", "a", "zzz", false, "", false},
		{"空 id，不写", "", "a", false, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := dropBeforeID(order, tc.session, tc.target, tc.after)
			if got != tc.want || changed != tc.changed {
				t.Fatalf("dropBeforeID(%v, %q, %q, after=%v) = (%q, %v)，want (%q, %v)",
					order, tc.session, tc.target, tc.after, got, changed, tc.want, tc.changed)
			}
		})
	}
	// 单元素/空名册都不该 panic。
	if _, changed := dropBeforeID([]string{"only"}, "only", "only", false); changed {
		t.Fatal("名册只有一条时没有可写的顺序变化")
	}
	if _, changed := dropBeforeID(nil, "a", "b", false); changed {
		t.Fatal("空名册不该产生写")
	}
}

// ── 事件层：真画一帧 + 真指针事件 ───────────────────────────────────────

// recordingInsert 是 `Actions.WorkspaceInsertSessionBefore` 的替身：记下每一次调用
// （三个平铺参数），也能装成失败。
type recordingInsert struct {
	mu     sync.Mutex
	seen   [][3]string
	fail   error
	notify []string
	// calls 让测试等"松手后那次 goroutine 里的写"——拖动本身不该写，只有松手写一次。
	calls chan [3]string
}

func (r *recordingInsert) insert(workspaceID, sessionID, beforeSessionID string) error {
	call := [3]string{workspaceID, sessionID, beforeSessionID}
	r.mu.Lock()
	r.seen = append(r.seen, call)
	err := r.fail
	r.mu.Unlock()
	select {
	case r.calls <- call:
	default:
	}
	return err
}

func (r *recordingInsert) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.seen)
}

func (r *recordingInsert) notifyFn(msg string) {
	r.mu.Lock()
	r.notify = append(r.notify, msg)
	r.mu.Unlock()
}

func (r *recordingInsert) notices() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.notify...)
}

// waitCall 等松手之后那次写（写发生在 goroutine 里）。
func (r *recordingInsert) waitCall(t *testing.T) [3]string {
	t.Helper()
	select {
	case call := <-r.calls:
		return call
	case <-time.After(3 * time.Second):
		t.Fatal("松手之后没有等到 WorkspaceInsertSessionBefore 的调用")
		return [3]string{}
	}
}

// sessionDragFixture 是一个"真画一帧"的侧栏：行矩形与命中区都留在 canvas 里，
// 指针事件走与运行时相同的 `Canvas.DeliverPointer` 通道。
type sessionDragFixture struct {
	t      *testing.T
	p      *Plugin
	env    *pluginapi.Env
	tree   *gfx.Tree
	canvas *gfx.Canvas
	page   *ui.Page
	bounds gfx.Rect
	// pressed 模拟运行时的按下状态（app/painter.go 的 paintCanvas 会把它交给
	// canvas.SetInteraction）。侧栏用它判断"这次拖拽的按下还在不在"。
	pressed gfx.ID
}

func newSessionDragFixture(t *testing.T, sessions []apiclient.Session,
	workspaces []pluginapi.Workspace, act pluginapi.Actions) *sessionDragFixture {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	pageTheme := dscore.VivaTheme(theme)
	f := &sessionDragFixture{
		t:      t,
		p:      &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}, width: 280, collapsed: map[string]bool{}},
		tree:   tree,
		canvas: canvas,
		page:   ui.NewPage(ui.PageConfig{Theme: &pageTheme}),
		bounds: gfx.Rect{Width: 268, Height: 600},
	}
	act.OpenSession = func(string) {}
	f.env = &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("sidebar-drag-test"),
		Canvas: canvas, Bounds: f.bounds, Act: act,
		Data: pluginapi.Snapshot{Sessions: sessions, Workspaces: workspaces},
	}
	return f
}

// draw 画一帧会话列表（不给事件）。
func (f *sessionDragFixture) draw(c *gfx.Canvas) {
	f.page.Draw(c, f.bounds, func(cc ui.C) {
		ui.Column(cc).Width(268).FillHeight().Children(func() { f.p.drawList(cc, f.env) })
	})
}

func (f *sessionDragFixture) paint() {
	f.canvas.Reset()
	f.canvas.SetInteraction(f.pressed, f.pressed, gfx.ID{}, false)
	f.draw(f.canvas)
}

// deliver 跑一次真指针事件（与 app/painter.go 的 deliverPointer 同一条路）。
//
// 同时按运行时的口径摆好 interaction：按下之后 `pressed` 一直是被按下的那个 id，
// 直到事件通道**结束之后**才清（app/painter.go:282 的 `pointer.Up()` 在 switch 尾部）。
func (f *sessionDragFixture) deliver(kind gfx.PointerEventKind, target gfx.ID, y float32) bool {
	f.pressed = target
	consumed := f.canvas.DeliverPointer(gfx.PointerEvent{
		Kind: kind, Button: gfx.PointerButtonPrimary, X: 60, Y: y, Target: target,
	}, ui.Size{Width: f.bounds.Width, Height: f.bounds.Height}, func(c *gfx.Canvas, _ ui.Size) {
		c.SetInteraction(f.pressed, f.pressed, gfx.ID{}, false)
		f.draw(c)
	})
	if kind == gfx.PointerUp {
		f.pressed = gfx.ID{}
	}
	return consumed
}

// rowID / rowRect 读这一帧真实的会话行命中区。
func (f *sessionDragFixture) rowID(sessionID string) gfx.ID {
	return f.tree.ID(f.env.Feat, "s-"+sessionID)
}

func (f *sessionDragFixture) rowRect(sessionID string) gfx.Rect {
	f.t.Helper()
	id := f.rowID(sessionID)
	for _, hit := range f.canvas.Hits() {
		if hit.ID == id {
			return hit.Bounds
		}
	}
	f.t.Fatalf("这一帧没有会话行 %q 的命中区；hits=%+v", sessionID, f.canvas.Hits())
	return gfx.Rect{}
}

// markerLines 找出这一帧里 2px 高的落点线（官方插入线：高 2、位于行上/下缘外侧 2px）。
func (f *sessionDragFixture) markerLines() []gfx.Rect {
	out := make([]gfx.Rect, 0, 1)
	for _, rect := range f.canvas.Rectangles() {
		if rect.Bounds.Height == 2 && rect.Bounds.Width > 100 {
			out = append(out, rect.Bounds)
		}
	}
	return out
}

// errHostRejected 是宿主"会话/工作区不存在"那一档的四字段错误（这里只要一个非 nil 的错）。
var errHostRejected = errors.New("宿主拒绝：会话不在该工作区名册里")

func dragSessions() []apiclient.Session {
	return []apiclient.Session{
		sess("a", "A", `D:\w`, "2026-11-03T10:00:00Z"),
		sess("b", "B", `D:\w`, "2026-11-03T11:00:00Z"),
		sess("c", "C", `D:\w`, "2026-11-03T12:00:00Z"),
	}
}

// TestSessionDragWritesOnceOnDrop 走真事件通道：按下（不进入拖拽态）、越过阈值
// （进入拖拽态 + Consume + 画落点线）、移动多次（一次都不写）、松手（只写一次，
// 参数是 workspaceId / sessionId / 落点之后那一条）。
func TestSessionDragWritesOnceOnDrop(t *testing.T) {
	rec := &recordingInsert{calls: make(chan [3]string, 8)}
	f := newSessionDragFixture(t, dragSessions(),
		[]pluginapi.Workspace{ws("w-1", `D:\w`, "工作区", "a", "b", "c")},
		pluginapi.Actions{WorkspaceInsertSessionBefore: rec.insert})
	f.paint()

	rowA, rowC := f.rowRect("a"), f.rowRect("c")
	target := f.rowID("a")

	if f.deliver(gfx.PointerDown, target, rowA.Y+rowA.Height/2) {
		t.Fatal("按下不该被 Consume：那一下还可能只是点击（点击要能打开会话）")
	}
	if f.p.drag.dragging {
		t.Fatal("按下就进入拖拽态了：那一下的点击会被吃掉")
	}

	// 越过 4px 阈值：进入拖拽态，Move 被 Consume。
	if !f.deliver(gfx.PointerMove, target, rowC.Y+rowC.Height-1) {
		t.Fatal("拖拽中的 Move 该被 Consume（否则松手还会触发点击）")
	}
	if !f.p.drag.dragging {
		t.Fatal("越过阈值之后应进入拖拽态")
	}
	if f.p.drag.over != "c" || !f.p.drag.after {
		t.Fatalf("落点 = %q after=%v，want c/true（c 行的下半区）", f.p.drag.over, f.p.drag.after)
	}
	// 再拖几下：只有状态变化，一次都不该写宿主。
	f.deliver(gfx.PointerMove, target, rowC.Y+rowC.Height-2)
	f.deliver(gfx.PointerMove, target, rowC.Y+rowC.Height-1)
	if rec.count() != 0 {
		t.Fatalf("拖动过程中写了 %d 次宿主：只有松手才写一次", rec.count())
	}
	// 落点线画在这一帧：官方是行下缘外侧 2px 的 2px 横线
	// （OFF:rows/Rows.module.css:370-408 的 .dropAfter）。
	f.paint()
	lines := f.markerLines()
	if len(lines) != 1 {
		t.Fatalf("这一帧的落点线 = %+v，want 恰好 1 根（行下缘外侧 2px）", lines)
	}
	if got := lines[0].Y; got != rowC.Y+rowC.Height {
		t.Fatalf("落点线 y = %v，want %v（丢在 c 行下半区 → 线在 c 下缘往下 2px 的位置）",
			got, rowC.Y+rowC.Height)
	}

	// 松手：只写一次。
	if !f.deliver(gfx.PointerUp, target, rowC.Y+rowC.Height-1) {
		t.Fatal("拖拽之后的 Up 该被 Consume")
	}
	if f.p.drag.dragging || f.p.drag.pressing {
		t.Fatalf("松手后拖拽状态没清：%+v", f.p.drag)
	}
	call := rec.waitCall(t)
	if call != [3]string{"w-1", "a", ""} {
		t.Fatalf("写参数 = %v，want [w-1 a \"\"]（a 落到末行 c 之后 = 移到组末尾，锚点空串）", call)
	}
	if got := rec.count(); got != 1 {
		t.Fatalf("松手写了 %d 次，want 1", got)
	}
}

// TestSessionDragAnchorIsTheNextRow 覆盖锚点不是空串的那一半：把 c 拖到 a 行上半区
// → c 落在 a 之前，锚点 = a。
func TestSessionDragAnchorIsTheNextRow(t *testing.T) {
	rec := &recordingInsert{calls: make(chan [3]string, 8)}
	f := newSessionDragFixture(t, dragSessions(),
		[]pluginapi.Workspace{ws("w-1", `D:\w`, "工作区", "a", "b", "c")},
		pluginapi.Actions{WorkspaceInsertSessionBefore: rec.insert})
	f.paint()

	rowA, rowC := f.rowRect("a"), f.rowRect("c")
	target := f.rowID("c")

	f.deliver(gfx.PointerDown, target, rowC.Y+rowC.Height/2)
	if !f.deliver(gfx.PointerMove, target, rowA.Y+1) {
		t.Fatal("拖拽中的 Move 该被 Consume")
	}
	if f.p.drag.over != "a" || f.p.drag.after {
		t.Fatalf("落点 = %q after=%v，want a/false（a 行的上半区）", f.p.drag.over, f.p.drag.after)
	}
	f.deliver(gfx.PointerUp, target, rowA.Y+1)
	if call := rec.waitCall(t); call != [3]string{"w-1", "c", "a"} {
		t.Fatalf("写参数 = %v，want [w-1 c a]", call)
	}
}

// TestSessionDragWithoutWritePathStaysIdle ⑥：壳没接 `WorkspaceInsertSessionBefore`
// （nil）时侧栏**不进入拖拽态**——不是"拖了没反应"：连 PointerOn 都不问，
// 事件通道整个是关的（viva 的 DeliverPointer 在"上一帧没问过 PointerOn"时直接返回 false）。
func TestSessionDragWithoutWritePathStaysIdle(t *testing.T) {
	f := newSessionDragFixture(t, dragSessions(),
		[]pluginapi.Workspace{ws("w-1", `D:\w`, "工作区", "a", "b", "c")},
		pluginapi.Actions{}) // WorkspaceInsertSessionBefore == nil
	f.paint()

	rowA, rowC := f.rowRect("a"), f.rowRect("c")
	target := f.rowID("a")
	if f.deliver(gfx.PointerDown, target, rowA.Y+rowA.Height/2) {
		t.Fatal("没有写面时按下不该被接管")
	}
	if f.deliver(gfx.PointerMove, target, rowC.Y+rowC.Height/2) {
		t.Fatal("没有写面时移动不该被接管（不该进入拖拽态）")
	}
	if f.p.drag.dragging || f.p.drag.pressing {
		t.Fatalf("没有写面时不该有拖拽状态：%+v", f.p.drag)
	}
	if lines := f.markerLines(); len(lines) != 0 {
		t.Fatalf("没有拖拽却画了落点线：%+v", lines)
	}
}

// TestCwdFallbackGroupIsNotDraggable：cwd 回退组（没有工作区身份）即使壳接了写动作
// 也不可拖——它没有 workspaceId，拖了也没有写路径。
func TestCwdFallbackGroupIsNotDraggable(t *testing.T) {
	rec := &recordingInsert{calls: make(chan [3]string, 8)}
	f := newSessionDragFixture(t, dragSessions(), nil,
		pluginapi.Actions{WorkspaceInsertSessionBefore: rec.insert})
	f.paint()

	rowA, rowC := f.rowRect("a"), f.rowRect("c")
	target := f.rowID("a")
	f.deliver(gfx.PointerDown, target, rowA.Y+rowA.Height/2)
	if f.deliver(gfx.PointerMove, target, rowC.Y+rowC.Height/2) {
		t.Fatal("cwd 回退组不该进入拖拽态")
	}
	if f.p.drag.dragging || f.p.drag.pressing {
		t.Fatalf("cwd 回退组不该有拖拽状态：%+v", f.p.drag)
	}
	if rec.count() != 0 {
		t.Fatal("cwd 回退组不该写宿主")
	}
}

// TestCommitSessionDropSurfacesHostError：写失败必须把宿主的错误交给壳的状态栏通道，
// **不许**静默当成成功；落点不产生顺序变化时一次都不写。
func TestCommitSessionDropSurfacesHostError(t *testing.T) {
	rec := &recordingInsert{calls: make(chan [3]string, 8)}
	act := pluginapi.Actions{WorkspaceInsertSessionBefore: rec.insert, Notify: rec.notifyFn}
	p := &Plugin{}

	// 落点 = 原位：不写。
	if p.commitSessionDrop(act, sessionDrag{
		dragging: true, sessionID: "b", workspaceID: "w", order: []string{"a", "b", "c"},
		over: "c", after: false,
	}) {
		t.Fatal("落点在原位时不该写")
	}
	// 没有落点：不写。
	if p.commitSessionDrop(act, sessionDrag{
		dragging: true, sessionID: "b", workspaceID: "w", order: []string{"a", "b", "c"},
	}) {
		t.Fatal("没有落点时不该写")
	}
	if rec.count() != 0 {
		t.Fatalf("不该有请求，实际 %d 次", rec.count())
	}

	// 宿主回错：错误必须冒到壳的状态栏通道。
	rec.fail = errHostRejected
	if !p.commitSessionDrop(act, sessionDrag{
		dragging: true, sessionID: "a", workspaceID: "w", order: []string{"a", "b", "c"},
		over: "c", after: true,
	}) {
		t.Fatal("参数合法时应该发出请求（返回值只表示「顺序变了」）")
	}
	if call := rec.waitCall(t); call != [3]string{"w", "a", ""} {
		t.Fatalf("写参数 = %v，want [w a \"\"]", call)
	}
	notices := rec.notices()
	if len(notices) != 1 || !strings.Contains(notices[0], errHostRejected.Error()) {
		t.Fatalf("宿主错误没有显示出来：%v（want 一条含 %q 的提示）", notices, errHostRejected.Error())
	}
}

// TestSessionRowHitIDsMatchDrawSessionIDs 钉住"落点表与行 id 同一口径"：
// 拖拽用的是预先算好的整组命中 id，它必须等于画出来的行 id。
func TestSessionRowHitIDsMatchDrawSessionIDs(t *testing.T) {
	f := newSessionDragFixture(t, dragSessions(),
		[]pluginapi.Workspace{ws("w-1", `D:\w`, "工作区", "a", "b", "c")},
		pluginapi.Actions{WorkspaceInsertSessionBefore: func(string, string, string) error { return nil }})
	f.paint()
	ids := sessionRowHitIDs(f.env, dragSessions())
	if len(ids) != 3 {
		t.Fatalf("命中 id 数 = %d，want 3", len(ids))
	}
	for i, want := range []string{"a", "b", "c"} {
		if ids[i] != f.rowID(want) {
			t.Fatalf("第 %d 个命中 id = %v，want 行 %q 的 id %v", i, ids[i], want, f.rowID(want))
		}
	}
}

// TestSessionDragStateClearedWhenPressIsGone：平台抢走按下时（PointerCancel）viva
// 只清按下状态、不会再送 PointerUp。下一帧发现"按下已经不在"必须把拖拽态收掉，
// 否则落点线会永远停在屏幕上（拖拽没有别的收尾路径）。
func TestSessionDragStateClearedWhenPressIsGone(t *testing.T) {
	rec := &recordingInsert{calls: make(chan [3]string, 8)}
	f := newSessionDragFixture(t, dragSessions(),
		[]pluginapi.Workspace{ws("w-1", `D:\w`, "工作区", "a", "b", "c")},
		pluginapi.Actions{WorkspaceInsertSessionBefore: rec.insert})
	f.paint()

	rowA, rowC := f.rowRect("a"), f.rowRect("c")
	target := f.rowID("a")
	f.deliver(gfx.PointerDown, target, rowA.Y+rowA.Height/2)
	if !f.deliver(gfx.PointerMove, target, rowC.Y+rowC.Height-1) {
		t.Fatal("越过阈值后 Move 该被 Consume")
	}
	if !f.p.drag.dragging {
		t.Fatal("应该已经进入拖拽态")
	}

	f.pressed = gfx.ID{} // 平台抢走了按下：interaction 里没有按下了
	f.paint()
	if f.p.drag.dragging || f.p.drag.pressing {
		t.Fatalf("按下已经不在，还留着拖拽态：%+v", f.p.drag)
	}
	if lines := f.markerLines(); len(lines) != 0 {
		t.Fatalf("拖拽态收了但落点线还在：%+v", lines)
	}
	if rec.count() != 0 {
		t.Fatalf("这次拖拽没有松手，不该写宿主；实际写了 %d 次", rec.count())
	}
}

// TestSessionDragKeepsLastMarkerOutsideTheRows：官方 dragend 提交的是 `drag.over`
// ——最后一次**落在行上**的落点，不是松手坐标；指针滑到行间空隙或列表外时落点保持
// 不变（OFF:rows/WorkspaceBrowser.tsx:769-772，以及文件头 121-123 的"指针滑出列表
// 不该被当成一次被拒绝的落点"）。这样指示线也不会在行间那 2px 空隙里一闪一闪。
func TestSessionDragKeepsLastMarkerOutsideTheRows(t *testing.T) {
	rec := &recordingInsert{calls: make(chan [3]string, 8)}
	f := newSessionDragFixture(t, dragSessions(),
		[]pluginapi.Workspace{ws("w-1", `D:\w`, "工作区", "a", "b", "c")},
		pluginapi.Actions{WorkspaceInsertSessionBefore: rec.insert})
	f.paint()

	rowA, rowC := f.rowRect("a"), f.rowRect("c")
	target := f.rowID("a")
	f.deliver(gfx.PointerDown, target, rowA.Y+rowA.Height/2)
	if !f.deliver(gfx.PointerMove, target, rowC.Y+1) {
		t.Fatal("拖拽中的 Move 该被 Consume")
	}
	if f.p.drag.over != "c" || f.p.drag.after {
		t.Fatalf("落点 = %q after=%v，want c/false", f.p.drag.over, f.p.drag.after)
	}
	// 滑到列表外的空地：落点保持上一次。
	if !f.deliver(gfx.PointerMove, target, f.bounds.Height-2) {
		t.Fatal("拖拽中的 Move 该被 Consume")
	}
	if f.p.drag.over != "c" || f.p.drag.after {
		t.Fatalf("滑出行外之后落点 = %q after=%v，want 保持 c/false", f.p.drag.over, f.p.drag.after)
	}
	f.deliver(gfx.PointerUp, target, f.bounds.Height-2)
	if call := rec.waitCall(t); call != [3]string{"w-1", "a", "c"} {
		t.Fatalf("写参数 = %v，want [w-1 a c]（a 落到 c 行之前）", call)
	}
}
