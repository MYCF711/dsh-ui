package dsshell

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// TestSolveColumnsLimitsUserSidebar 用户拖出来的侧栏走 180–400，
// 官方默认值那条路（264–420）一点不动。
func TestSolveColumnsLimitsUserSidebar(t *testing.T) {
	cases := []struct {
		name                string
		viewport            float32
		sidebar             float32
		sideLo, sideHi      float32
		wantS, wantC, wantR float32
	}{
		// 官方口径：200 → 抬到 264（这一段必须与 TestSolveColumnsMatchesOfficial 同源）。
		{"官方下限 264", 1920, 200, sidebarMin, sidebarMax, 264, 792, 864},
		// 用户口径：同样的 200 不再被抬到 264。
		{"用户下限 180", 1920, 200, userSidebarMin, userSidebarMax, 200, 856, 864},
		// 用户拖到 180：三列之和仍等于视口。
		{"用户拖到 180", 1920, 180, userSidebarMin, userSidebarMax, 180, 876, 864},
		// 用户拖到 400：中栏让位，右栏按 available 保持 864。
		{"用户拖到 400", 1920, 400, userSidebarMin, userSidebarMax, 400, 656, 864},
		// 用户上限 400：500 被夹回 400，而不是官方上限 420。
		{"用户上限 400", 1920, 500, userSidebarMin, userSidebarMax, 400, 656, 864},
		// 侧栏拖宽到右栏放不下：官方规则不变——右栏整列消失，中栏吃满。
		{"拖宽后右栏放不下", 960, 400, userSidebarMin, userSidebarMax, 400, 560, 0},
		// 同一档窄窗口、侧栏只到 200：available = 360 ≥ 300，右栏按 available 留下。
		{"拖宽后右栏刚够", 960, 200, userSidebarMin, userSidebarMax, 200, 400, 360},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := solveColumnsLimits(c.viewport, c.sidebar, c.viewport*rightbarOpenFrac, true, 0, c.sideLo, c.sideHi)
			if got.sidebar != c.wantS || got.center != c.wantC || got.rightbar != c.wantR {
				t.Fatalf("solveColumnsLimits(%v,%v,lo=%v,hi=%v) = %+v，想要 {%v %v %v}",
					c.viewport, c.sidebar, c.sideLo, c.sideHi, got, c.wantS, c.wantC, c.wantR)
			}
			if sum := got.sidebar + got.center + got.rightbar; sum != c.viewport {
				t.Fatalf("三列之和 %v ≠ 视口 %v（%+v）", sum, c.viewport, got)
			}
		})
	}
}

// TestLayoutUserWidthBeatsShellGeometry 用户宽度压过壳的 Geometry，
// 而壳的 Geometry 仍然是「用户没拖过」时的首选（这是拖动能在重排后留住的原因）。
func TestLayoutUserWidthBeatsShellGeometry(t *testing.T) {
	hasAll := func(pluginapi.Slot) bool { return true }
	geometry := pluginapi.Geometry{Width: 1920, SidebarWidth: 280, RightWidth: 864}

	// 没拖过：照壳的几何摆。
	p := &Plugin{SidebarWidth: 280}
	env := &pluginapi.Env{
		Geometry: geometry, Bounds: ui.Rect{Width: 1920},
		Data: pluginapi.Snapshot{ShowRightbar: true}, Has: hasAll,
	}
	if got := p.layout(env); got != (columns{sidebar: 280, center: 776, rightbar: 864}) {
		t.Fatalf("没拖过时应照壳几何：%+v", got)
	}

	// 拖过侧栏：壳还说 280，画出来必须是 344。
	p.UserSidebarWidth = 344
	if got := p.layout(env); got != (columns{sidebar: 344, center: 712, rightbar: 864}) {
		t.Fatalf("用户宽度应压过壳几何：%+v", got)
	}

	// 拖过右栏：右栏按用户值，中栏重新分配。
	p.UserRightbarWidth = 700
	if got := p.layout(env); got != (columns{sidebar: 344, center: 876, rightbar: 700}) {
		t.Fatalf("用户右栏宽度应压过壳几何：%+v", got)
	}

	// 「重启后按住户值恢复」走的就是这条路：全新插件实例 + 两个字段被灌值。
	restarted := &Plugin{SidebarWidth: 280, UserSidebarWidth: 344, UserRightbarWidth: 700}
	if got := restarted.layout(env); got != (columns{sidebar: 344, center: 876, rightbar: 700}) {
		t.Fatalf("重启恢复：%+v", got)
	}

	// 拖过侧栏但这一帧没有侧栏占用方：侧栏整列不占位，用户值不硬撑出一列。
	noSidebar := &Plugin{SidebarWidth: 280, UserSidebarWidth: 344}
	envNoSidebar := &pluginapi.Env{
		Geometry: geometry, Bounds: ui.Rect{Width: 1920},
		Data: pluginapi.Snapshot{ShowRightbar: true},
		Has: func(s pluginapi.Slot) bool {
			return s != pluginapi.SlotSidebarHeader && s != pluginapi.SlotSidebarList &&
				s != pluginapi.SlotSidebarFooter
		},
	}
	if got := noSidebar.layout(envNoSidebar); got != (columns{sidebar: 0, center: 1056, rightbar: 864}) {
		t.Fatalf("没有侧栏占用方时不该按用户值硬留一列：%+v", got)
	}
}

// TestRightbarDragCeiling 右栏拖动上限是视口 40%，
// 唯独「按下时就已经比 40% 宽」（官方默认打开是 45%）时不往下塌。
func TestRightbarDragCeiling(t *testing.T) {
	cases := []struct {
		name       string
		vw, startW float32
		want       float32
	}{
		{"1920 从 300 起", 1920, 300, 768},
		{"1920 从 600 起", 1920, 600, 768},
		{"1920 官方 45%", 1920, 864, 864}, // 不塌回 768
		{"1182 官方 45%", 1182, 532, 532}, // 532 > 472.8，同样不塌
		{"窄窗口下限保护", 700, 300, 300},      // 700*0.4=280 < 300 → 下限 300
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rightbarDragCeiling(c.vw, c.startW); got != c.want {
				t.Fatalf("rightbarDragCeiling(%v,%v) = %v，想要 %v", c.vw, c.startW, got, c.want)
			}
		})
	}
}

// dragFixture 在真实 flex 树上跑一层 dsshell，专门用来驱动拖拽。
type dragFixture struct {
	t      *testing.T
	plugin *Plugin
	theme  *pluginapi.Theme
	env    *pluginapi.Env
	tree   *ui.Tree
	canvas *ui.Canvas
	bounds ui.Rect
	boxes  map[pluginapi.Slot]flex.Box
	// host 是宿主 ui-config.json 的替身：拖拽松手时插件会往它里面写。
	host *fakeHostConfig
}

func newDragFixture(t *testing.T, width, height float32) *dragFixture {
	t.Helper()
	theme := dscore.Dark()
	tree := &ui.Tree{}
	canvas := &ui.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	f := &dragFixture{
		t:      t,
		plugin: &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}, SidebarWidth: 280},
		theme:  theme,
		tree:   tree,
		canvas: canvas,
		bounds: ui.Rect{Width: width, Height: height},
		boxes:  map[pluginapi.Slot]flex.Box{},
		host:   &fakeHostConfig{kv: map[string]string{}},
	}
	f.env = &pluginapi.Env{
		Theme: theme,
		Data:  pluginapi.Snapshot{ShowRightbar: true},
		Tree:  tree, Feat: tree.Feature("shell-drag-test"), Canvas: canvas, Bounds: f.bounds,
		Geometry: pluginapi.Geometry{
			Width: width, Height: height, SidebarWidth: 280,
			RightWidth: width * rightbarOpenFrac, MainWidth: width - 280 - width*rightbarOpenFrac,
			BottomHeight: 24,
		},
		MainWidth: width - 280 - width*rightbarOpenFrac,
		Act:       f.host.actions(),
		Has:       func(slot pluginapi.Slot) bool { return slot != pluginapi.SlotShellOverlay },
	}
	return f
}

// paint 只画一帧（不住窗口），命中区留在 canvas.Hits() 里。
func (f *dragFixture) paint() {
	f.canvas.Reset()
	f.boxes = map[pluginapi.Slot]flex.Box{}
	var view flex.View
	view.Draw(f.canvas, dscore.VivaTheme(f.theme), f.bounds, func(fr *flex.Frame) {
		f.plugin.children(fr, f.env, func(slot pluginapi.Slot) {
			switch slot {
			case pluginapi.SlotMainBody:
				f.boxes[slot] = fr.Slot().FillWidth()
			case pluginapi.SlotRightbar:
				f.boxes[slot] = fr.Slot().FillWidth().FillHeight()
			case pluginapi.SlotShellBottom:
				f.boxes[slot] = fr.Slot().FillWidth().FillHeight()
			default:
				f.boxes[slot] = fr.Slot().FillWidth()
			}
		})
	})
}

// event 跑一次指针事件的事件通道（和运行时 app/painter.go 里那条路一样）。
// 返回控件有没有 Consume。
func (f *dragFixture) event(kind ui.PointerEventKind, target ui.ID, x float32) bool {
	return f.canvas.DeliverPointer(ui.PointerEvent{
		Kind: kind, Button: ui.PointerButtonPrimary, X: x, Y: 120, Target: target,
	}, ui.Size{Width: f.bounds.Width, Height: f.bounds.Height}, func(c *ui.Canvas, size ui.Size) {
		// DeliverPointer 已经 Reset/BeginFrame，这里不再自己开帧。
		var view flex.View
		view.Draw(c, dscore.VivaTheme(f.theme), f.bounds, func(fr *flex.Frame) {
			f.plugin.children(fr, f.env, func(slot pluginapi.Slot) {
				fr.Slot().FillWidth()
			})
		})
	})
}

// hit 在最近一帧的命中区里找 id 对应的那一块。
func (f *dragFixture) hit(id ui.ID) (ui.Hit, bool) {
	for _, h := range f.canvas.Hits() {
		if h.ID == id {
			return h, true
		}
	}
	return ui.Hit{}, false
}

// TestDragSidebarHandle 走真事件通道拖侧栏：命中带位置、指针形状、
// 按下→移动→松手之后的宽度，以及上下限。
func TestDragSidebarHandle(t *testing.T) {
	f := newDragFixture(t, 1920, 1032)
	id := f.tree.ID(f.env.Feat, "layout-handle-sidebar")

	f.paint()
	h, ok := f.hit(id)
	if !ok {
		t.Fatalf("侧栏把手没有命中区；hits=%+v", f.canvas.Hits())
	}
	// 命中带骑在分隔线上：左缘 = 280 - 4，宽 8，高 = 主体行高（视口减底部整行）。
	wantRect := ui.Rect{X: 280 - handleWidth/2, Y: 0, Width: handleWidth, Height: 1032 - 24}
	assertRect(t, "侧栏把手命中带", h.Bounds, wantRect)
	if h.Cursor != ui.PointerCursorResizeHorizontal {
		t.Fatalf("把手光标应为 SizeWE（PointerCursorResizeHorizontal），得到 %v", h.Cursor)
	}
	if h.Role != ui.RoleSlider {
		t.Fatalf("把手角色应为 slider，得到 %v", h.Role)
	}

	// 平时不画线：分隔线上没有 2px/3px 的竖条（官方两栏之间本来就没有分隔线）。
	for _, r := range f.canvas.Rectangles() {
		if r.Bounds.X >= 276 && r.Bounds.X < 280 && r.Bounds.Width >= 2 && r.Bounds.Width <= 3 {
			t.Fatalf("悬停前不该画拖拽线；rectangles=%+v", r)
		}
	}

	if !f.event(ui.PointerDown, id, 280) {
		t.Fatal("按下把手必须有控件 Consume（否则会被当成点击，也不会立刻重绘）")
	}
	if !f.plugin.dragging.active || f.plugin.dragging.kind != handleSidebar {
		t.Fatalf("按下后应进入侧栏拖拽：%+v", f.plugin.dragging)
	}
	// 往右拖 64px：实时改宽，三列重新分配。
	if !f.event(ui.PointerMove, id, 344) {
		t.Fatal("拖动必须 Consume（否则鼠标和边线脱节）")
	}
	if f.plugin.UserSidebarWidth != 344 {
		t.Fatalf("拖到 344 后侧栏宽应为 344，得到 %v", f.plugin.UserSidebarWidth)
	}
	if sum := f.env.Geometry.SidebarWidth + f.env.Geometry.MainWidth + f.env.Geometry.RightWidth; sum != 1920 {
		t.Fatalf("拖动后 Geometry 三列之和 %v ≠ 1920：%+v", sum, f.env.Geometry)
	}
	if f.env.Geometry.SidebarWidth != 344 {
		t.Fatalf("拖动后 Geometry.SidebarWidth 应校正成 344，得到 %v", f.env.Geometry.SidebarWidth)
	}
	if !f.event(ui.PointerUp, id, 344) {
		t.Fatal("松手必须 Consume")
	}
	if f.plugin.dragging.active {
		t.Fatal("松手后拖拽状态应清掉")
	}
	// 松手后宽度保留（这就是「松手后保留」的断言：再画一帧仍然是 344）。
	f.paint()
	if f.env.Geometry.SidebarWidth != 344 {
		t.Fatalf("松手后再画一帧仍应是 344，得到 %v", f.env.Geometry.SidebarWidth)
	}

	// 下限 180：往左狠拖也不会低于 180。
	f.event(ui.PointerDown, id, 344)
	f.event(ui.PointerMove, id, -5000)
	if got := f.plugin.UserSidebarWidth; got != userSidebarMin {
		t.Fatalf("侧栏下限应为 %v，得到 %v", userSidebarMin, got)
	}
	f.event(ui.PointerUp, id, -5000)
	// 上限 400：往右狠拖也不会超过 400（不是官方的 420）。
	f.event(ui.PointerDown, id, 180)
	f.event(ui.PointerMove, id, 5000)
	if got := f.plugin.UserSidebarWidth; got != userSidebarMax {
		t.Fatalf("侧栏上限应为 %v，得到 %v", userSidebarMax, got)
	}
	f.event(ui.PointerUp, id, 5000)
}

// TestDragRightbarHandle 拖右栏：往左变宽、往右变窄，上限 40%（起手比 40% 宽时不塌）。
func TestDragRightbarHandle(t *testing.T) {
	f := newDragFixture(t, 1920, 1032)
	id := f.tree.ID(f.env.Feat, "layout-handle-rightbar")

	f.paint()
	h, ok := f.hit(id)
	if !ok {
		t.Fatalf("右栏把手没有命中区；hits=%+v", f.canvas.Hits())
	}
	// 右栏左缘 = 1920 - 864 = 1056；命中带骑在上面。
	assertRect(t, "右栏把手命中带", h.Bounds,
		ui.Rect{X: 1056 - handleWidth/2, Y: 0, Width: handleWidth, Height: 1032 - 24})
	if h.Cursor != ui.PointerCursorResizeHorizontal {
		t.Fatalf("右栏把手光标应为 SizeWE，得到 %v", h.Cursor)
	}

	f.event(ui.PointerDown, id, 1056)
	f.event(ui.PointerMove, id, 1121) // 往右 65px → 右栏窄 65
	if got := f.plugin.UserRightbarWidth; got != 799 {
		t.Fatalf("往右拖 65px 后右栏应为 799，得到 %v", got)
	}
	if f.env.Geometry.RightWidth != 799 || f.env.Geometry.MainWidth != 841 {
		t.Fatalf("右栏拖动后几何不对：%+v", f.env.Geometry)
	}
	// 继续往左推过 40% 上限：从 864 起手，上限就是 864（不塌回 768）。
	f.event(ui.PointerMove, id, -4000)
	if got := f.plugin.UserRightbarWidth; got != 864 {
		t.Fatalf("起手 864 时上限应为 864（官方首次打开 45%%），得到 %v", got)
	}
	f.event(ui.PointerUp, id, -4000)

	// 从 300 起手往左推：上限是视口 40% = 768。
	f.plugin.UserRightbarWidth = 300
	f.event(ui.PointerDown, id, 1620)
	f.event(ui.PointerMove, id, -4000)
	if got := f.plugin.UserRightbarWidth; got != 768 {
		t.Fatalf("从 300 起手上限应为视口 40%% = 768，得到 %v", got)
	}
	f.event(ui.PointerUp, id, -4000)

	// 下限 300：往右狠拖也不会更窄。
	f.plugin.UserRightbarWidth = 500
	f.event(ui.PointerDown, id, 1420)
	f.event(ui.PointerMove, id, 5000)
	if got := f.plugin.UserRightbarWidth; got != userRightbarMin {
		t.Fatalf("右栏下限应为 %v，得到 %v", userRightbarMin, got)
	}
	f.event(ui.PointerUp, id, 5000)
}

// TestDragIgnoresOtherHandleAndRightBar 拖动侧栏时右栏把手的事件不应改右栏，
// 非主键按下也不该开始拖。
func TestDragIgnoresOtherHandleAndRightBar(t *testing.T) {
	f := newDragFixture(t, 1920, 1032)
	sideID := f.tree.ID(f.env.Feat, "layout-handle-sidebar")
	rightID := f.tree.ID(f.env.Feat, "layout-handle-rightbar")
	f.paint()

	// 右键按在侧栏把手上：不开始拖。
	if f.event2(ui.PointerDown, ui.PointerButtonSecondary, sideID, 280) {
		t.Fatal("右键不该被当成拖拽起点")
	}
	if f.plugin.dragging.active {
		t.Fatal("右键之后不该处于拖拽状态")
	}

	// 左键在侧栏把手上按下：随后「指针跑到右栏把手上」的移动事件（target 仍是侧栏把手，
	// 因为按下时指针被捕获）只能改侧栏。
	f.event(ui.PointerDown, sideID, 280)
	f.event(ui.PointerMove, sideID, 320)
	if f.plugin.UserRightbarWidth != 0 {
		t.Fatalf("拖侧栏不该改右栏，得到 %v", f.plugin.UserRightbarWidth)
	}
	f.event(ui.PointerUp, sideID, 320)

	// 右栏把手的按下事件在侧栏拖拽未结束时被忽略。
	f.event(ui.PointerDown, sideID, 320)
	if f.event(ui.PointerDown, rightID, 1040) {
		t.Fatal("侧栏拖拽进行中，右栏把手的按下不该被 Consume")
	}
	if f.plugin.dragging.kind != handleSidebar {
		t.Fatalf("拖拽身份被抢走了：%+v", f.plugin.dragging)
	}
	f.event(ui.PointerUp, sideID, 320)
}

// event2 是带按键的 event（只用来验「非主键不开始拖」）。
func (f *dragFixture) event2(kind ui.PointerEventKind, button ui.PointerButton, target ui.ID, x float32) bool {
	return f.canvas.DeliverPointer(ui.PointerEvent{
		Kind: kind, Button: button, X: x, Y: 120, Target: target,
	}, ui.Size{Width: f.bounds.Width, Height: f.bounds.Height}, func(c *ui.Canvas, size ui.Size) {
		var view flex.View
		view.Draw(c, dscore.VivaTheme(f.theme), f.bounds, func(fr *flex.Frame) {
			f.plugin.children(fr, f.env, func(slot pluginapi.Slot) {
				fr.Slot().FillWidth()
			})
		})
	})
}

// TestHandlesAbsentWithoutColumn 没有那一列就没有把手：右栏收起 / 没有侧栏占用方时
// 都不该留一条能拖的带子。
func TestHandlesAbsentWithoutColumn(t *testing.T) {
	f := newDragFixture(t, 960, 832) // 960 宽时右栏放不下，整列消失
	f.paint()
	if _, ok := f.hit(f.tree.ID(f.env.Feat, "layout-handle-rightbar")); ok {
		t.Fatal("右栏整列消失时不该有右栏把手")
	}
	if _, ok := f.hit(f.tree.ID(f.env.Feat, "layout-handle-sidebar")); !ok {
		t.Fatal("侧栏在，把手应该在")
	}

	// 没有侧栏占用方：侧栏把手消失。
	g := newDragFixture(t, 1182, 832)
	g.env.Has = func(slot pluginapi.Slot) bool {
		return slot != pluginapi.SlotShellOverlay && slot != pluginapi.SlotSidebarHeader &&
			slot != pluginapi.SlotSidebarList && slot != pluginapi.SlotSidebarFooter
	}
	g.paint()
	if _, ok := g.hit(g.tree.ID(g.env.Feat, "layout-handle-sidebar")); ok {
		t.Fatal("没有侧栏占用方时不该有侧栏把手")
	}
}

// TestReadConfigRestoresWidths 启动时把 ui-config.json 读回来：
// 壳第一帧之后才拿得到配置，所以 readConfig 是"前几帧每帧问一次"。
func TestReadConfigRestoresWidths(t *testing.T) {
	hasAll := func(pluginapi.Slot) bool { return true }
	newEnv := func(act pluginapi.Actions) *pluginapi.Env {
		return &pluginapi.Env{
			Geometry: pluginapi.Geometry{Width: 1920, SidebarWidth: 280, RightWidth: 864},
			Bounds:   ui.Rect{Width: 1920},
			Data:     pluginapi.Snapshot{ShowRightbar: true},
			Has:      hasAll,
			Act:      act,
		}
	}

	// 一帧空配置（壳还没拿到），第二帧才有值：最终必须用上。
	emptyFirst := true
	act := pluginapi.Actions{ConfigGet: func() map[string]string {
		if emptyFirst {
			emptyFirst = false
			return map[string]string{}
		}
		return map[string]string{cfgKeySidebar: "344", cfgKeyRightbar: "700"}
	}}
	p := &Plugin{SidebarWidth: 280}
	env := newEnv(act)
	p.readConfig(env)
	if p.UserSidebarWidth != 0 {
		t.Fatalf("空配置不该灌值，得到 %v", p.UserSidebarWidth)
	}
	p.readConfig(env)
	if p.UserSidebarWidth != 344 || p.UserRightbarWidth != 700 {
		t.Fatalf("第二次读到配置应灌成 344/700，得到 %v/%v", p.UserSidebarWidth, p.UserRightbarWidth)
	}
	if got := p.layout(env); got != (columns{sidebar: 344, center: 876, rightbar: 700}) {
		t.Fatalf("恢复后的三栏几何：%+v", got)
	}

	// 越界的存量值：侧栏夹回 180–400，右栏抬到下限 300。
	clamped := &Plugin{SidebarWidth: 280}
	clamped.readConfig(newEnv(pluginapi.Actions{ConfigGet: func() map[string]string {
		return map[string]string{cfgKeySidebar: "9000", cfgKeyRightbar: "120"}
	}}))
	if clamped.UserSidebarWidth != userSidebarMax {
		t.Fatalf("9000 应夹到 %v，得到 %v", userSidebarMax, clamped.UserSidebarWidth)
	}
	if clamped.UserRightbarWidth != userRightbarMin {
		t.Fatalf("120 应抬到 %v，得到 %v", userRightbarMin, clamped.UserRightbarWidth)
	}

	// 壳没接线（ConfigGet 为 nil）：什么都不灌，也不 panic。
	none := &Plugin{SidebarWidth: 280}
	none.readConfig(newEnv(pluginapi.Actions{}))
	none.readConfig(&pluginapi.Env{Has: hasAll})
	if none.UserSidebarWidth != 0 || none.UserRightbarWidth != 0 {
		t.Fatalf("没接线时不该有用户宽度：%v/%v", none.UserSidebarWidth, none.UserRightbarWidth)
	}
}

// TestConfigDoesNotOverrideDrag 配置晚到时不能盖掉用户刚拖过的结果。
func TestConfigDoesNotOverrideDrag(t *testing.T) {
	p := &Plugin{SidebarWidth: 280, UserSidebarWidth: 333}
	env := &pluginapi.Env{
		Geometry: pluginapi.Geometry{Width: 1920},
		Has:      func(pluginapi.Slot) bool { return true },
		Act: pluginapi.Actions{ConfigGet: func() map[string]string {
			return map[string]string{cfgKeySidebar: "200", cfgKeyRightbar: "700"}
		}},
	}
	p.readConfig(env)
	if p.UserSidebarWidth != 333 {
		t.Fatalf("用户拖过的侧栏不该被配置改成 200，得到 %v", p.UserSidebarWidth)
	}
	// 没碰过的右栏照样接受配置。
	if p.UserRightbarWidth != 700 {
		t.Fatalf("没拖过的右栏应接受配置 700，得到 %v", p.UserRightbarWidth)
	}
}

// TestDragPersistsOnlyWhenMoved 按一下不动不落盘；拖动一次只写一次。
func TestDragPersistsOnlyWhenMoved(t *testing.T) {
	f := newDragFixture(t, 1920, 1032)
	id := f.tree.ID(f.env.Feat, "layout-handle-sidebar")
	f.paint()

	// 按下即松手：宽度没变，不该写配置。
	f.event(ui.PointerDown, id, 280)
	f.event(ui.PointerUp, id, 280)
	if f.host.sets != 0 {
		t.Fatalf("没拖动不该写配置，写了 %d 次：%v", f.host.sets, f.host.snapshot())
	}

	// 拖一次（含多次 move）：只在松手时写一次，写的是最终宽度。
	f.event(ui.PointerDown, id, 280)
	f.event(ui.PointerMove, id, 300)
	f.event(ui.PointerMove, id, 330)
	if f.host.sets != 0 {
		t.Fatalf("拖动过程中不该写配置，写了 %d 次", f.host.sets)
	}
	f.event(ui.PointerUp, id, 330)
	if f.host.sets != 1 {
		t.Fatalf("松手应写一次，实际 %d 次", f.host.sets)
	}
	if got := f.host.value(cfgKeySidebar); got != "330" {
		t.Fatalf("%s 应写成 330，实际 %q", cfgKeySidebar, got)
	}
}

// TestParseWidth 落盘格式：整数像素；空的/非数字/<=0 都算"没配置"。
func TestParseWidth(t *testing.T) {
	ok := []struct {
		in   string
		want float32
	}{
		{"360", 360},
		{" 344 ", 344},
		{"180.6", 180.6},
	}
	for _, c := range ok {
		if got, valid := parseWidth(c.in); !valid || got != c.want {
			t.Fatalf("parseWidth(%q) = %v,%v，想要 %v", c.in, got, valid, c.want)
		}
	}
	for _, in := range []string{"", " ", "abc", "0", "-5", "360px"} {
		if got, valid := parseWidth(in); valid {
			t.Fatalf("parseWidth(%q) 应判为没配置，得到 %v", in, got)
		}
	}
	if got := formatWidth(360.4); got != "360" {
		t.Fatalf("formatWidth(360.4) = %q，想要 \"360\"", got)
	}
}

type fakeHostConfig struct {
	kv   map[string]string
	sets int
}

func (h *fakeHostConfig) actions() pluginapi.Actions {
	return pluginapi.Actions{
		// ConfigGet 每次给一份新 map：插件的契约是「只读」，不该被它改到宿主那份。
		ConfigGet: func() map[string]string {
			out := make(map[string]string, len(h.kv))
			for k, v := range h.kv {
				out[k] = v
			}
			return out
		},
		ConfigSet: func(kv map[string]string) {
			h.sets++
			for k, v := range kv {
				h.kv[k] = v
			}
		},
	}
}

func (h *fakeHostConfig) value(key string) string { return h.kv[key] }

func (h *fakeHostConfig) snapshot() map[string]string {
	out := make(map[string]string, len(h.kv))
	for k, v := range h.kv {
		out[k] = v
	}
	return out
}
