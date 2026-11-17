//go:build darwin || windows || linux

package dsentrycustom

// entry_hover_test.go —— #17 左栏行尾的**悬停三态**（2026-11-17 补）。
//
// 官方那一整块在 `@media (hover:hover)` 里（OFF:EntryMenu.module.css:20-32），
// 行内三件东西随**行**悬停一起变：
//
//	静止                           行悬停
//	固定按钮 opacity 0（:26）  →   1（:28）
//	对勾     opacity 1（:27）  →   0（:30）
//	固定图标 color tertiary   →   color primary（:22-25，跟**按钮自己**的悬停）
//
// 断言口径：这两枚图标都是矢量（描边 → `Snapshot.Primitives`），显隐由 opacity 变成
// **图元 alpha**，颜色由令牌决定 —— 所以"visible(某令牌)"就是判据本身，
// 不去截图比像素。

import (
	"math"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// primVisible 是"这枚图元真的画得出来"的阈值：opacity 0 的子树要么被 cull 掉、
// 要么以 alpha 0 落到画布上，两种都是"看不见"。
const primVisible = 0.01

// hoverActiveEntries 是一条**当前选中且未固定**的左栏条目：行尾同时存在
// 固定按钮与对勾（官方正是靠 opacity 让这一格二选一）。
func hoverActiveEntries() []pluginapi.EntryMenuEntry {
	return []pluginapi.EntryMenuEntry{
		{ID: "schedules", Label: "自动化任务", Glyph: "◷", Pinned: false, Active: true},
	}
}

// nearRGB 只比 RGB：官方用 `opacity` 控制显隐，淡出后 RGB 仍是令牌值。
func nearRGB(a, b gfx.Color) bool {
	const tolerance = 0.004
	return math.Abs(float64(a.Red-b.Red)) < tolerance &&
		math.Abs(float64(a.Green-b.Green)) < tolerance &&
		math.Abs(float64(a.Blue-b.Blue)) < tolerance
}

// primRect 给出图元真正占的矩形。
//
// 图标的描边在 viva 里是一串**抗锯齿三角形**（`app.TrianglePrimitive`），几何在
// `Points` 里而 `Bounds` 是零值——所以先按 Kind 分派，不能直接读 Bounds
// （`app/driver.go:985-1017`）。
func primRect(p app.Primitive) gfx.Rect {
	if p.Kind != app.TrianglePrimitive {
		return p.Bounds
	}
	minX, maxX := p.Points[0].X, p.Points[0].X
	minY, maxY := p.Points[0].Y, p.Points[0].Y
	for _, pt := range p.Points[1:] {
		minX, maxX = min32(minX, pt.X), max32(maxX, pt.X)
		minY, maxY = min32(minY, pt.Y), max32(maxY, pt.Y)
	}
	return gfx.Rect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
}

// inRect 判一个矩形是否落在另一个矩形内（相交即算，容差 1px 给抗锯齿）。
func inRect(outer, inner gfx.Rect) bool {
	return inner.X+inner.Width >= outer.X-1 && inner.X <= outer.X+outer.Width+1 &&
		inner.Y+inner.Height >= outer.Y-1 && inner.Y <= outer.Y+outer.Height+1
}

// rowPrims 收集**落在某一行矩形内**、RGB 等于令牌 want 的图元。
func (f *entryFixture) rowPrims(row gfx.Rect, want gfx.Color) []app.Primitive {
	var out []app.Primitive
	for _, prim := range f.session.Snapshot().Primitives {
		if !nearRGB(prim.Color, want) {
			continue
		}
		if inRect(row, primRect(prim)) {
			out = append(out, prim)
		}
	}
	return out
}

// inside 判一个矩形是否整个落在另一个矩形里（容差 1px）：用来区分"整行底色"
// 与"按钮自己那一格底色"。
func inside(outer, inner gfx.Rect) bool {
	return inner.X >= outer.X-1 && inner.Y >= outer.Y-1 &&
		inner.X+inner.Width <= outer.X+outer.Width+1 &&
		inner.Y+inner.Height <= outer.Y+outer.Height+1
}

// countVisible 数其中"画得出来"的那些。
func countVisible(prims []app.Primitive) int {
	n := 0
	for _, prim := range prims {
		if prim.Color.Alpha > primVisible {
			n++
		}
	}
	return n
}

// moveTo 把指针移到 (x,y) 并重画，返回重画后的快照（悬停态由驱动在指针移动时更新）。
func (f *entryFixture) moveTo(x, y float32) app.Snapshot {
	f.t.Helper()
	if err := f.session.Move(x, y); err != nil {
		f.t.Fatal(err)
	}
	f.pump()
	f.pump()
	return f.session.Snapshot()
}

// TestSidebarHoverSwapsPinAndActiveMark 钉住官方那一对**反向**规则：
// 静止看对勾、行悬停看固定按钮，同一时刻只有一枚看得见。
func TestSidebarHoverSwapsPinAndActiveMark(t *testing.T) {
	f := newEntryFixture(t, pluginapi.EntryRegionSidebar, hoverActiveEntries(), &fakeNavigation{})
	defer f.close()

	row := f.mustHit("entry-open-schedules").Bounds
	primary, tertiary := f.theme.LabelPrimary, f.theme.LabelTertiary

	// ① 静止（指针还没进窗口）：对勾可见、固定按钮不可见（`:27` / `:26`）。
	if got := countVisible(f.rowPrims(row, primary)); got == 0 {
		t.Fatal("静止态没画出对勾：官方 `.sidebarEntry .activeMark{opacity:1}`（:27）在静止时是可见的")
	}
	if got := countVisible(f.rowPrims(row, tertiary)); got != 0 {
		t.Fatalf("静止态画出了 %d 枚固定按钮图元：官方 `.pinAnchor{opacity:0}`（:26）静止时完全透明", got)
	}

	// ② 指针落在行上：换成固定按钮可见、对勾隐去（`:28` / `:30`）。
	snapshot := f.moveTo(row.X+row.Width/2, row.Y+row.Height/2)
	if snapshot.Hovered != f.tree.ID(f.feat, "entry-open-schedules") {
		t.Fatalf("本帧悬停身份 = %v，want 行本身（指针在行中央）", snapshot.Hovered)
	}
	if got := countVisible(f.rowPrims(row, tertiary)); got == 0 {
		t.Fatal("行悬停后固定按钮没显形：官方 `.sidebarEntry:hover .pinAnchor{opacity:1}`（:28）")
	}
	if got := countVisible(f.rowPrims(row, primary)); got != 0 {
		t.Fatalf("行悬停后对勾还画着 %d 枚图元：官方 `.sidebarEntry:hover .activeMark{opacity:0}`（:30）", got)
	}

	// ③ 指针移到固定按钮上：它自己还在（CSS 的 :hover 沿祖先链传播，
	//    `.sidebarEntry:hover` 依然成立），所以不许"悬停就把自己弄没"。
	//
	// 这一档**不能按颜色判对勾**：此刻按钮图标已被 `.pinButton:hover`（:23）换成
	// label-primary，与对勾同色。改为数**图元条数**：按钮自身的描边+填充是固定的一串，
	// 对勾不在其中；若对勾还画着，条数会比"只有按钮"那一档多出它的那十几条。
	rowHoverPin := countVisible(f.rowPrims(row, tertiary))
	pin := f.mustHit("entry-pin-schedules").Bounds
	snapshot = f.moveTo(pin.X+pin.Width/2, pin.Y+pin.Height/2)
	if snapshot.Hovered != f.tree.ID(f.feat, "entry-pin-schedules") {
		t.Fatalf("本帧悬停身份 = %v，want 固定按钮（指针在按钮中央）", snapshot.Hovered)
	}
	if got := countVisible(f.rowPrims(row, tertiary)); got != 0 {
		t.Fatalf("指针落在固定按钮上，图标却还是 label-tertiary（%d 枚图元）：`.pinButton:hover{color:label-primary}`（:23）", got)
	}
	if got := countVisible(f.rowPrims(row, primary)); got != rowHoverPin {
		t.Fatalf("按钮悬停态的图元条数 %d ≠ 行悬停态 %d：多出来的就是对勾（官方 :30 在按钮悬停时同样隐去）",
			got, rowHoverPin)
	}
}

// TestPinButtonHoverChangesIconColor 钉住 `.pinButton:hover:not(:disabled)`（:22-25）：
// **背景保持透明**，只有图标色从 label-tertiary 变 label-primary。
func TestPinButtonHoverChangesIconColor(t *testing.T) {
	f := newEntryFixture(t, pluginapi.EntryRegionSidebar, hoverActiveEntries(), &fakeNavigation{})
	defer f.close()

	row := f.mustHit("entry-open-schedules").Bounds
	pin := f.mustHit("entry-pin-schedules").Bounds
	primary, tertiary := f.theme.LabelPrimary, f.theme.LabelTertiary

	// 行悬停（指针不在按钮上）：按钮显形，颜色仍是 label-tertiary。
	f.moveTo(row.X+8, row.Y+row.Height/2)
	if got := countVisible(f.rowPrims(row, tertiary)); got == 0 {
		t.Fatal("行悬停时固定按钮没显形，无法比较颜色")
	}
	if got := countVisible(f.rowPrims(row, primary)); got != 0 {
		t.Fatalf("指针不在按钮上，图标却已变成 label-primary（%d 枚图元）", got)
	}

	// 指针落在按钮上：图标色换 label-primary（**描边与填充层都要换**——sprite 里
	// 尾 path 是 `fill="currentColor"`），且**不出现**背景色块。
	rowHoverTransTertiary := countVisible(f.rowPrims(row, tertiary))
	f.moveTo(pin.X+pin.Width/2, pin.Y+pin.Height/2)
	swapped := countVisible(f.rowPrims(row, primary))
	if swapped == 0 {
		t.Fatal("指针落在固定按钮上，图标色没有变成 label-primary：官方 `.pinButton:hover:not(:disabled){color:label-primary}`（:23）")
	}
	if swapped != rowHoverTransTertiary {
		t.Fatalf("换色后可见图元 %d 枚，行悬停时 %d 枚：差数就是没跟 currentColor 换色的那一层（sprite 尾 path 是 fill=\"currentColor\"）",
			swapped, rowHoverTransTertiary)
	}
	if got := countVisible(f.rowPrims(row, tertiary)); got != 0 {
		t.Fatalf("指针落在固定按钮上，图标仍是 label-tertiary（%d 枚图元）：填充层没跟着 currentColor 换色", got)
	}
	// `.pinButton:hover:not(:disabled){color:label-primary;background:0 0}`（:24）：
	// 按钮自己**不铺底色**。判据是"没有落在按钮 22×22 里的底色矩形"——整行的
	// hover 底色（`.sidebarEntry:hover>div:first-child>button`，:21）确实该亮，
	// 它铺的是整行、不是按钮那一格。
	for _, prim := range f.rowPrims(row, f.theme.Hover) {
		if prim.Kind != app.RectanglePrimitive || prim.Color.Alpha <= primVisible {
			continue
		}
		if inside(pin, primRect(prim)) {
			t.Fatalf("固定按钮自己铺了底色 %+v：官方 `:hover{background:0 0}`（:24）只有图标换色", prim.Bounds)
		}
	}
	// 反过来，祖先级规则要生效：指针在按钮上时整行底色是亮的（:21）。
	if countVisible(f.rowPrims(row, f.theme.Hover)) == 0 {
		t.Fatal("指针落在固定按钮上时整行底色没亮：官方 `.sidebarEntry:hover>div:first-child>button{background:interactive-bg-hover}`（:21）")
	}
}

// TestSidebarActiveMarkGeometry 钉住 `.sidebarEntry .activeMark` 的落点
// （OFF:EntryMenu.module.css:7-14）：`right:9px`、14×14、竖直居中。
//
// check-outline 的路径在 16 的 viewBox 里 x 跨 2.25..13.75 —— 关于 8 对称，
// 所以画到 14 的盒子里也关于盒中心对称：**图元包围盒的中线就是盒子中线**。
func TestSidebarActiveMarkGeometry(t *testing.T) {
	f := newEntryFixture(t, pluginapi.EntryRegionSidebar, hoverActiveEntries(), &fakeNavigation{})
	defer f.close()

	row := f.mustHit("entry-open-schedules").Bounds
	marks := f.rowPrims(row, f.theme.LabelPrimary)
	if countVisible(marks) == 0 {
		t.Fatal("静止态没有对勾图元，无法量几何")
	}
	minX, maxX := float32(math.MaxFloat32), float32(-math.MaxFloat32)
	minY, maxY := float32(math.MaxFloat32), float32(-math.MaxFloat32)
	for _, prim := range marks {
		if prim.Color.Alpha <= primVisible {
			continue
		}
		b := primRect(prim)
		minX, maxX = min32(minX, b.X), max32(maxX, b.X+b.Width)
		minY, maxY = min32(minY, b.Y), max32(maxY, b.Y+b.Height)
	}
	centerX := (minX + maxX) / 2
	wantCenterX := row.X + row.Width - sidebarActiveMarkRight - sidebarActiveMarkBox/2
	if delta := float32(math.Abs(float64(centerX - wantCenterX))); delta > 1 {
		t.Fatalf("对勾中线 %v，want %v（`.activeMark{right:9px}` + 14 宽的盒子中线 = 行右边 - 9 - 7）",
			centerX, wantCenterX)
	}
	centerY := (minY + maxY) / 2
	rowCenterY := row.Y + row.Height/2
	if delta := float32(math.Abs(float64(centerY - rowCenterY))); delta > 1 {
		t.Fatalf("对勾中线 %v，want %v（官方 `top:50%%` + `translateY(-50%%)` 竖直居中）", centerY, rowCenterY)
	}
	// 同一行里对勾与固定按钮**不并排**：对勾右缘（行右 - 9）落在固定按钮
	// 右缘（行右 - 6）左侧，两枚靠 opacity 互斥（见上一条用例）。
	pin := f.mustHit("entry-pin-schedules").Bounds
	if maxX > row.X+row.Width-sidebarActiveMarkRight+1 {
		t.Fatalf("对勾右缘 %v 越过了 `.activeMark{right:9px}`（行右 - 9 = %v）",
			maxX, row.X+row.Width-sidebarActiveMarkRight)
	}
	if delta := float32(math.Abs(float64(pin.X + pin.Width - (row.X + row.Width - sidebarPinRight)))); delta > 0.5 {
		t.Fatalf("固定按钮右缘 %v，want 行右 - 6 = %v",
			pin.X+pin.Width, row.X+row.Width-sidebarPinRight)
	}
}

// TestConversationActiveMarkIgnoresHover 钉住官方**只有左栏**才有那两条悬停规则：
// 会话头那一侧 `.activeMark{flex:none;margin-left:auto}`（:3）没有 `.sidebarEntry`
// 祖先，所以 :12 的 `opacity:0` 与 :30 的悬停隐去都不适用 —— 恒可见。
func TestConversationActiveMarkIgnoresHover(t *testing.T) {
	entries := []pluginapi.EntryMenuEntry{
		{ID: "trajectory", Label: "轨迹", Pinned: true, Active: true},
	}
	f := newEntryFixture(t, pluginapi.EntryRegionConversation, entries, &fakeNavigation{})
	defer f.close()

	row := f.mustHit("entry-open-trajectory").Bounds
	primary := f.theme.LabelPrimary
	if countVisible(f.rowPrims(row, primary)) == 0 {
		t.Fatal("会话头的 active 对勾没画：官方 `.activeMark{flex:none;margin-left:auto}` 不受 region 限制")
	}
	f.moveTo(row.X+row.Width/2, row.Y+row.Height/2)
	if countVisible(f.rowPrims(row, primary)) == 0 {
		t.Fatal("会话头行悬停后对勾消失了：那两条悬停规则只在 `.sidebarEntry` 下（:26-31），会话头恒可见")
	}
}

// TestPinButtonKeepsOfficialTwoLineCopy 钉住官方那一格是**两层文案**
// （OFF:EntryMenuContent.tsx:54-55）：
//
//	Tooltip  label={t('pinTooltip')}      → 可见浮条「固定到左侧栏」
//	Button   aria-label={t('pin',{name})} → 无障碍名「将{name}固定到侧边栏」
//
// 少了可见浮条，就是"官方可见部件没实现"；少了 aria-label 则连这枚图标按钮
// 该怎么说都说不出来。
func TestPinButtonKeepsOfficialTwoLineCopy(t *testing.T) {
	f := newEntryFixture(t, pluginapi.EntryRegionSidebar, hoverActiveEntries(), &fakeNavigation{})
	defer f.close()

	if got, want := f.mustHit("entry-pin-schedules").Label, "将自动化任务固定到侧边栏"; got != want {
		t.Fatalf("无障碍名 = %q，want %q（官方 `t('pin',{name})`）", got, want)
	}
	// 浮条是悬停才出：先落到按钮上，再把时间推过工具条的延迟。
	pin := f.mustHit("entry-pin-schedules").Bounds
	f.moveTo(pin.X+pin.Width/2, pin.Y+pin.Height/2)
	if err := f.session.Wait(600 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	f.pump()
	if !f.session.HasText(zhPinTooltip) {
		t.Fatalf("悬停固定按钮后没有可见浮条：官方 `Tooltip label={t('pinTooltip')}`（%q）", zhPinTooltip)
	}
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
