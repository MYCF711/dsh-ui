package dssettings

import (
	"math"
	"strings"
	"testing"
)

// TestGeometryTableIsSelfConsistent 逐行核对几何表。
//
// 判据（与首屏骨架 official_geometry.go 同一套）：
//   - aligned 行：实现值必须等于官方值；
//   - derived 行：必须写清官方公式（calc/clamp），且实现值是**等价取值**；
//   - deviation 行：必须写清原因；
//   - 每行必须有 rc.2 锚点。
func TestGeometryTableIsSelfConsistent(t *testing.T) {
	if len(OfficialSettingsGeometry) < 60 {
		t.Fatalf("几何表只有 %d 行，设置面结构容器不止这些", len(OfficialSettingsGeometry))
	}
	aligned, derived, deviation := GeoCount()
	if deviation != 0 {
		t.Fatalf("几何表有 %d 行 deviation；本任务要求结构容器全部 aligned/derived", deviation)
	}
	if aligned+derived+deviation != len(OfficialSettingsGeometry) {
		t.Fatalf("状态计数对不上：%d+%d+%d != %d", aligned, derived, deviation, len(OfficialSettingsGeometry))
	}
	for i, row := range OfficialSettingsGeometry {
		if row.Node == "" || row.Prop == "" {
			t.Fatalf("第 %d 行缺 node/prop：%+v", i, row)
		}
		if row.Anchor == "" || !strings.Contains(row.Anchor, "src/") {
			t.Fatalf("第 %d 行（%s.%s）锚点不是 rc.2 src/**：%q", i, row.Node, row.Prop, row.Anchor)
		}
		if row.Unit == "" {
			t.Fatalf("第 %d 行（%s.%s）缺量纲", i, row.Node, row.Prop)
		}
		switch row.Status {
		case GeoAligned:
			if row.Official != row.Impl {
				t.Fatalf("%s.%s 标了 aligned 但官方 %v != 实现 %v", row.Node, row.Prop, row.Official, row.Impl)
			}
		case GeoDerived:
			if strings.TrimSpace(row.Formula) == "" {
				t.Fatalf("%s.%s 标了 derived 但没写官方公式", row.Node, row.Prop)
			}
		case GeoDeviation:
			if strings.TrimSpace(row.Reason) == "" {
				t.Fatalf("%s.%s 标了 deviation 但没写原因", row.Node, row.Prop)
			}
		default:
			t.Fatalf("%s.%s 的状态 %q 不认识", row.Node, row.Prop, row.Status)
		}
	}
}

// TestGeometryCoversTheFourGridAreas 钉住 rc.2 四区网格（D1）的官方值。
func TestGeometryCoversTheFourGridAreas(t *testing.T) {
	cases := []struct {
		node, prop string
		want       float32
	}{
		{"SettingsPanel.panel", "width", 800},
		{"SettingsPanel.panel", "grid-template-columns[0]", 188},
		{"SettingsPanel.overlay", "z-index", 1000},
		{"SettingsRoot.nav", "padding-top", 10},
		{"SettingsRoot.navTitle", "padding-left", 24},
		{"SettingsRoot.navCell", "height", 40},
		{"SettingsRoot.navCell", "padding-right", 16},
		{"SettingsRoot.header", "height", 54},
		{"SettingsRoot.options", "padding-inline", 24},
		{"PreferenceRow.row", "padding-block", 16},
		{"PreferenceRow.selector", "height", 36},
		{"ModelsSection.deleteDialog", "width", 480},
	}
	for _, tc := range cases {
		row, ok := geometryRow(tc.node, tc.prop)
		if !ok {
			t.Fatalf("几何表缺 %s.%s", tc.node, tc.prop)
		}
		if row.Official != tc.want || row.Impl != tc.want {
			t.Fatalf("%s.%s：官方 %v / 实现 %v，期望都是 %v", tc.node, tc.prop, row.Official, row.Impl, tc.want)
		}
	}
}

// TestPanelSizeFollowsOfficialCalc 钉住 `.panel` 三条尺寸的官方公式。
func TestPanelSizeFollowsOfficialCalc(t *testing.T) {
	// 大窗口：宽 800、高被 800 上限夹住。
	w, h := PanelSize(1920, 1032, PanelOverlayTop)
	if w != 800 || h != 800 {
		t.Fatalf("1920×1032 下卡片 = %v×%v，官方是 800×min(800, 1032-48)=800", w, h)
	}
	// 矮窗口：高度按 calc 收缩。
	w, h = PanelSize(1920, 700, PanelOverlayTop)
	if w != 800 {
		t.Fatalf("矮窗口宽 = %v，官方 width 固定 800", w)
	}
	if want := float32(700 - 2*24); h != want {
		t.Fatalf("矮窗口高 = %v，官方 calc(100vh - 2*24) = %v", h, want)
	}
	// 窄窗口：max-width 收窄（官方 calc(100vw - 48px)）。
	w, _ = PanelSize(700, 800, PanelOverlayTop)
	if want := float32(700 - 48); w != want {
		t.Fatalf("窄窗口宽 = %v，官方 max-width calc(100vw - 48px) = %v", w, want)
	}
	// 极窄窗口：不能变成负数。
	if w, _ = PanelSize(10, 800, PanelOverlayTop); w < 0 {
		t.Fatalf("极窄窗口宽 = %v，不能为负", w)
	}
	// overlayTop 小于默认值时按 24 算（官方 max(24px, var(...))）。
	if _, h := PanelSize(1920, 1032, 4); h != 800 {
		t.Fatalf("overlayTop=4 时高 = %v，官方 max(24px,4px)=24 → 800", h)
	}
}

// TestPanelCentering 钉住卡片居中（官方 .overlay 是 flex center）。
func TestPanelCentering(t *testing.T) {
	w, h := PanelSize(1182, 832, PanelOverlayTop)
	if got := PanelLeft(1182, w); got != (1182-w)/2 {
		t.Fatalf("左沿 = %v，应水平居中", got)
	}
	if got := PanelTop(832, h); got != (832-h)/2 {
		t.Fatalf("上沿 = %v，应垂直居中", got)
	}
	// 窗口比卡片小时不能变负。
	if got := PanelLeft(100, 800); got != 0 {
		t.Fatalf("窄窗口左沿 = %v，应为 0", got)
	}
}

// TestNarrowViewportBreakpoint 钉住官方窄窗口断点（767/768）。
func TestNarrowViewportBreakpoint(t *testing.T) {
	if NarrowViewportBreakpoint != 767 {
		t.Fatalf("窄窗口断点 = %v，官方 @media (width <= 767px)", NarrowViewportBreakpoint)
	}
	if !NarrowViewport(700) {
		t.Fatal("700 应判定为窄窗口")
	}
	if NarrowViewport(768) {
		t.Fatal("768 不是窄窗口（官方是 <=767）")
	}
}

// TestPositiveLowerBoundIsFiveEMinus324 钉住官方正下界 5e-324。
func TestPositiveLowerBoundIsFiveEMinus324(t *testing.T) {
	if PositiveLowerBound != math.SmallestNonzeroFloat64 {
		t.Fatalf("PositiveLowerBound = %v，应为 math.SmallestNonzeroFloat64", PositiveLowerBound)
	}
	if !(PositiveLowerBound > 0) {
		t.Fatalf("PositiveLowerBound = %v，必须为正", PositiveLowerBound)
	}
	if PositiveLowerBoundText != "5e-324" {
		t.Fatalf("正下界文本 = %q，官方 Number.MIN_VALUE.toString() = 5e-324", PositiveLowerBoundText)
	}
	if !DelayWithinBounds(PositiveLowerBound) {
		t.Fatal("5e-324 应在边界内")
	}
	if DelayWithinBounds(0) || DelayWithinBounds(-1) {
		t.Fatal("0 与负数都必须被拒绝（官方 x <= 0 抛错）")
	}
	if DelayWithinBounds(float64(RetryMaxTimerDelayMs) + 1) {
		t.Fatal("超过 MAX_TIMER_DELAY_MS 必须拒绝")
	}
	if !JitterWithinBounds(0) || !JitterWithinBounds(1) {
		t.Fatal("jitterRatio 的边界是闭区间 [0,1]")
	}
	if JitterWithinBounds(-0.1) || JitterWithinBounds(1.1) {
		t.Fatal("jitterRatio 越界必须拒绝")
	}
}

func geometryRow(node, prop string) (GeoRow, bool) {
	for _, row := range OfficialSettingsGeometry {
		if row.Node == node && row.Prop == prop {
			return row, true
		}
	}
	return GeoRow{}, false
}
