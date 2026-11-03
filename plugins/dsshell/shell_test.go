package dsshell

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/ui"
)

// TestSolveColumnsMatchesOfficial 用官方 computeColumns 的算例逐条对拍。
//
// 期望值不是抄我们的实现，是按官方 JS 原函数手算的：
//
//	const s = clampWidth(sidebar, 264, 420);
//	const available = viewport - s - 400;
//	const r = rightbar === 0 || available < 300 ? 0
//	        : Math.min(available, clampWidth(rightbar, 300, viewport * .7));
//	center = viewport - s - r
func TestSolveColumnsMatchesOfficial(t *testing.T) {
	cases := []struct {
		name                string
		viewport            float32
		sidebar             float32
		rightbar            float32
		rightbarOn          bool
		wantS, wantC, wantR float32
	}{
		// 1920×1032：右栏首次打开 = 1920*.45 = 864；available = 1240 ≥ 300，
		// 864 也在 [300, 1344] 内 → 右栏 864，中栏 776。
		{"1920 右栏开", 1920, 280, 864, true, 280, 776, 864},
		// 1182×832：偏好 1182*.45 = 531.9 → 532，但 available = 502，
		// 所以右栏被压到 502，中栏正好落在 400 下限。
		{"1182 右栏开", 1182, 280, 532, true, 280, 400, 502},
		// 1182 右栏关：中栏吃掉全部剩余。
		{"1182 右栏关", 1182, 280, 532, false, 280, 902, 0},
		// 960：available = 280 < 300 → 官方右栏**整列消失**，中栏 680。
		// 这条是这一版修掉的真实偏差：旧实现会硬塞 300，把中栏挤到 380。
		{"960 右栏放不下", 960, 280, 432, true, 280, 680, 0},
		// 刚好够：available = 300 → 右栏 300，中栏 400。
		{"980 右栏刚好够", 980, 280, 441, true, 280, 400, 300},
		// 1000：available = 320 < 偏好 450 → 右栏 320，中栏 400。
		{"1000 右栏被 available 卡住", 1000, 280, 450, true, 280, 400, 320},
		// 侧栏偏好低于下限 → 抬到 264。
		{"侧栏下限", 1920, 200, 864, true, 264, 792, 864},
		// 侧栏偏好高于上限 → 压到 420。
		{"侧栏上限", 1920, 600, 864, true, 420, 636, 864},
		// 右栏偏好超过视口 70% → 压到 70%。
		{"右栏 70% 上限", 4000, 280, 3500, true, 280, 920, 2800},
		// 没有视口就不编宽度。
		{"无视口", 0, 280, 864, true, 0, 0, 0},
		// 没有侧栏占用方（collapsedWidth=0）→ 整列不占位，中栏从 0 起。
		{"无侧栏占用方", 1920, 0, 864, true, 0, 1056, 864},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := solveColumns(c.viewport, c.sidebar, c.rightbar, c.rightbarOn, 0)
			if got.sidebar != c.wantS || got.center != c.wantC || got.rightbar != c.wantR {
				t.Fatalf("solveColumns(%v,%v,%v,on=%v) = %+v，想要 {%v %v %v}",
					c.viewport, c.sidebar, c.rightbar, c.rightbarOn, got,
					c.wantS, c.wantC, c.wantR)
			}
		})
	}
}

// TestSolveColumnsFillsViewport 三列之和必须严格等于视口。
//
// 差 1px 就会在右边缘留一条缝，或者把最后一列挤出窗口
// （旧实现踩过：中栏 + 右栏一起超宽时侧栏被压到 239）。
func TestSolveColumnsFillsViewport(t *testing.T) {
	for vw := float32(320); vw <= 3840; vw += 7 {
		for _, on := range []bool{false, true} {
			got := solveColumns(vw, 280, vw*rightbarOpenFrac, on, 0)
			sum := got.sidebar + got.center + got.rightbar
			if sum != vw {
				t.Fatalf("视口 %v：三列之和 %v（%+v）", vw, sum, got)
			}
			if got.rightbar > 0 && got.center < mainMin {
				t.Fatalf("视口 %v：右栏在场时中栏只有 %v，低于官方下限 %v", vw, got.center, mainMin)
			}
		}
	}
}

// TestLayoutUsesShellGeometry 验证 layout 对壳输入的处理：
// 有 Geometry.Width 就用它、没有就用 Bounds、右栏没给偏好就取视口 45%。
func TestLayoutUsesShellGeometry(t *testing.T) {
	hasAll := func(pluginapi.Slot) bool { return true }
	hasNone := func(pluginapi.Slot) bool { return false }

	p := &Plugin{SidebarWidth: 280}

	// 壳给了完整几何：与直接调 solveColumns 一致。
	env := &pluginapi.Env{
		Geometry: pluginapi.Geometry{Width: 1920, SidebarWidth: 280, RightWidth: 864},
		Bounds:   ui.Rect{Width: 1920, Height: 1032},
		Data:     pluginapi.Snapshot{ShowRightbar: true},
		Has:      hasAll,
	}
	if got := p.layout(env); got != (columns{sidebar: 280, center: 776, rightbar: 864}) {
		t.Fatalf("1920 有壳几何：%+v", got)
	}

	// 壳没给 Geometry.Width → 退回 Bounds（整窗），右栏偏好取 45%。
	env2 := &pluginapi.Env{
		Geometry: pluginapi.Geometry{SidebarWidth: 280},
		Bounds:   ui.Rect{Width: 1182, Height: 832},
		Data:     pluginapi.Snapshot{ShowRightbar: true},
		Has:      hasAll,
	}
	if got := p.layout(env2); got != (columns{sidebar: 280, center: 400, rightbar: 502}) {
		t.Fatalf("1182 退回 Bounds：%+v", got)
	}

	// 没有右栏占用方：右栏不出现，中栏吃掉全部剩余。
	env3 := &pluginapi.Env{
		Geometry: pluginapi.Geometry{Width: 1920, SidebarWidth: 280, RightWidth: 864},
		Bounds:   ui.Rect{Width: 1920, Height: 1032},
		Data:     pluginapi.Snapshot{ShowRightbar: true},
		Has:      func(s pluginapi.Slot) bool { return s != pluginapi.SlotRightbar },
	}
	if got := p.layout(env3); got != (columns{sidebar: 280, center: 1640, rightbar: 0}) {
		t.Fatalf("没有右栏占用方：%+v", got)
	}

	// 壳说收起：即使占用方在也不出现。
	env4 := &pluginapi.Env{
		Geometry: pluginapi.Geometry{Width: 1920, SidebarWidth: 280, RightWidth: 864},
		Bounds:   ui.Rect{Width: 1920, Height: 1032},
		Data:     pluginapi.Snapshot{ShowRightbar: false},
		Has:      hasAll,
	}
	if got := p.layout(env4); got != (columns{sidebar: 280, center: 1640, rightbar: 0}) {
		t.Fatalf("右栏收起：%+v", got)
	}

	// 没有侧栏占用方：侧栏整列不占位（本插件既有约定），中栏从 0 起。
	env5 := &pluginapi.Env{
		Geometry: pluginapi.Geometry{Width: 1920, SidebarWidth: 280, RightWidth: 864},
		Bounds:   ui.Rect{Width: 1920, Height: 1032},
		Data:     pluginapi.Snapshot{ShowRightbar: true},
		Has: func(s pluginapi.Slot) bool {
			return s != pluginapi.SlotSidebarHeader && s != pluginapi.SlotSidebarList &&
				s != pluginapi.SlotSidebarFooter
		},
	}
	if got := p.layout(env5); got != (columns{sidebar: 0, center: 1056, rightbar: 864}) {
		t.Fatalf("没有侧栏占用方：%+v", got)
	}

	// 连视口都没有：不编宽度。
	env6 := &pluginapi.Env{Has: hasNone}
	if got := p.layout(env6); got != (columns{}) {
		t.Fatalf("无视口：%+v", got)
	}
}

// TestClampWidthRoundsLikeOfficial 官方 clampWidth 里有一次 Math.round。
func TestClampWidthRoundsLikeOfficial(t *testing.T) {
	cases := []struct{ in, lo, hi, want float32 }{
		{531.9, 300, 827.4, 532},
		{280, 264, 420, 280},
		{263.4, 264, 420, 264},
		{420.6, 264, 420, 420},
	}
	for _, c := range cases {
		if got := clampWidth(c.in, c.lo, c.hi); got != c.want {
			t.Fatalf("clampWidth(%v,%v,%v) = %v，想要 %v", c.in, c.lo, c.hi, got, c.want)
		}
	}
}

// TestSyncGeometry 把壳说的几何校正成画出来的：
// 视口未知（center=0）时不覆盖壳的值；有中栏宽时三列宽一并校正。
func TestSyncGeometry(t *testing.T) {
	env := &pluginapi.Env{
		MainWidth: 380,
		Geometry:  pluginapi.Geometry{Width: 1920, SidebarWidth: 280, RightWidth: 300, MainWidth: 380},
	}
	syncGeometry(env, columns{sidebar: 280, center: 680})
	if env.MainWidth != 680 || env.Geometry.MainWidth != 680 {
		t.Fatalf("有中栏宽时应校正成 680，得到 MainWidth=%v Geometry.MainWidth=%v",
			env.MainWidth, env.Geometry.MainWidth)
	}

	env2 := &pluginapi.Env{
		MainWidth: 380,
		Geometry:  pluginapi.Geometry{Width: 1920, SidebarWidth: 280, RightWidth: 300, MainWidth: 380},
	}
	syncGeometry(env2, columns{})
	if env2.MainWidth != 380 || env2.Geometry.SidebarWidth != 280 {
		t.Fatalf("视口未知时不该覆盖壳的值，得到 MainWidth=%v Sidebar=%v",
			env2.MainWidth, env2.Geometry.SidebarWidth)
	}

	// 用户拖过侧栏：Geometry 里壳的旧值（280）必须被改成画出来的 344，
	// 否则侧栏的活动面板浮层会按 280 定位，停在新侧栏的中间。
	env3 := &pluginapi.Env{
		MainWidth: 776,
		Geometry:  pluginapi.Geometry{Width: 1920, SidebarWidth: 280, RightWidth: 864, MainWidth: 776},
	}
	syncGeometry(env3, columns{sidebar: 344, center: 712, rightbar: 864})
	if env3.Geometry.SidebarWidth != 344 || env3.Geometry.MainWidth != 712 || env3.Geometry.RightWidth != 864 {
		t.Fatalf("拖动后几何未校正：%+v", env3.Geometry)
	}
}
