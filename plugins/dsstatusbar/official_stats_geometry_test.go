package dsstatusbar

// official_stats_geometry_test.go —— 几何常量表 + 弹窗三态几何断言。
//
// 判据（task-53 §任务 2「几何落常量表（官方值 ↔ 实现值，三态）」）：
//
//	① 表自洽：aligned 行实现值 = 官方值；derived 行写明官方公式；deviation 行写明
//	   原因；每行都有 rc.2 `src/**` 锚点与量纲；
//	② 三态几何（真实绘制）：
//	     常态   —— 宽窗 1182：官方 min-width 300 生效，弹窗在胶囊上方 8px；
//	     窄窗   —— 320 宽：官方 max-width min(440, 100vw-24) 生效，左右各留 12；
//	     靠右   —— 锚点贴右边缘：官方 left: max(12px, anchor(left)) 之后再往左推，
//	               保证整块不出窗（实现比官方更严一档，已在函数注释与报告里写明）；
//	③ 明细网格：dt 从内容左沿起、dd 右沿贴内容右沿（官方 `text-align: right`）。

import (
	"math"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// TestOfficialStatsGeometryTableIsSelfConsistent 逐行核对几何表。
func TestOfficialStatsGeometryTableIsSelfConsistent(t *testing.T) {
	if len(OfficialStatsGeometry) < 24 {
		t.Fatalf("几何表只有 %d 行，弹窗 + 胶囊的结构值不止这些", len(OfficialStatsGeometry))
	}
	aligned, derived, deviation := StatsGeoCount()
	if aligned+derived+deviation != len(OfficialStatsGeometry) {
		t.Fatalf("状态计数对不上：%d+%d+%d != %d", aligned, derived, deviation, len(OfficialStatsGeometry))
	}
	if deviation > 3 {
		t.Fatalf("deviation 行 %d 条，本单登记上限 3（图标 ×2 + 不可量化的 text-align ×1）", deviation)
	}
	for i, row := range OfficialStatsGeometry {
		if row.Node == "" || row.Prop == "" {
			t.Fatalf("第 %d 行缺 node/prop：%+v", i, row)
		}
		if !strings.Contains(row.Anchor, "src/") {
			t.Fatalf("第 %d 行（%s.%s）锚点不是 rc.2 src/**：%q", i, row.Node, row.Prop, row.Anchor)
		}
		if row.Unit == "" {
			t.Fatalf("第 %d 行（%s.%s）缺量纲", i, row.Node, row.Prop)
		}
		switch row.Status {
		case StatsGeoAligned:
			if row.Official != row.Impl {
				t.Fatalf("%s.%s 标了 aligned 但官方 %v != 实现 %v", row.Node, row.Prop, row.Official, row.Impl)
			}
		case StatsGeoDerived:
			if strings.TrimSpace(row.Formula) == "" {
				t.Fatalf("%s.%s 标了 derived 但没写官方公式", row.Node, row.Prop)
			}
		case StatsGeoDeviation:
			if strings.TrimSpace(row.Reason) == "" {
				t.Fatalf("%s.%s 标了 deviation 但没写原因", row.Node, row.Prop)
			}
		default:
			t.Fatalf("%s.%s 的状态 %q 不认识", row.Node, row.Prop, row.Status)
		}
	}
}

// TestOfficialStatsGeometryPinsOfficialValues 钉住几条最容易漂移的官方值。
func TestOfficialStatsGeometryPinsOfficialValues(t *testing.T) {
	cases := []struct {
		node, prop string
		want       float32
	}{
		{"StatsDialog.panel", "padding", 16},
		{"StatsDialog.panel", "min-width", 300},
		{"StatsDialog.panel", "max-width", 440},
		{"StatsDialog.panel", "window-inset", 12},
		{"StatsDialog.panel", "anchor-gap", 8},
		{"StatsDialog.panel", "font-size", 12},
		{"StatsDialog.panel", "line-height", 18},
		{"StatsDialog.title", "margin-bottom", 8},
		{"StatsDialog.title", "gap", 16},
		{"StatsDialog.title", "font-weight", 500},
		{"StatsDialog.titleLabel", "gap", 6},
		{"StatsDialog.titleRule", "margin-bottom", 10},
		{"StatsDialog.titleRule", "border-top", 0.5},
		{"StatsDialog.details", "column-gap", 16},
		{"StatsDialog.details", "row-gap", 6},
		{"StatsDialog.details.dt", "min-width", 76},
	}
	for _, tc := range cases {
		row, ok := statsGeoRow(tc.node, tc.prop)
		if !ok {
			t.Fatalf("几何表缺 %s.%s", tc.node, tc.prop)
		}
		if row.Official != tc.want {
			t.Fatalf("%s.%s 官方值 = %v，want %v", tc.node, tc.prop, row.Official, tc.want)
		}
		if row.Impl != tc.want {
			t.Fatalf("%s.%s 实现值 = %v，want %v（这些行都不允许偏离）", tc.node, tc.prop, row.Impl, tc.want)
		}
	}
	// 胶囊几何走 statusbar.go 的既有常量，表里引用的是同一份，不能再有一套。
	if pillHeight != 22 || pillPadX != 8 || pillPadY != 1 || pillGap != 6 || pillLineHeight != 20 {
		t.Fatalf("胶囊常量被改动：h=%v padX=%v padY=%v gap=%v line=%v",
			pillHeight, pillPadX, pillPadY, pillGap, pillLineHeight)
	}
}

// TestStatsDialogGeometryStates 是三态几何：常态 / 窄窗 / 靠右锚点。
func TestStatsDialogGeometryStates(t *testing.T) {
	// ── 常态：宽窗 + 胶囊贴底 → 弹窗在胶囊上方 8px、宽 = 官方 min-width 300。
	h := newStatsDialogHarness(t)
	data := statsSnapshot(statsFullStats())
	bounds := ui.Rect{Width: 1182, Height: 832}
	h.frame(t, bounds, data, "", "")
	open := h.frame(t, bounds, data, statsPillHitName, "")
	pill, ok := statsDialogHitByName(t, h, open, statsPillHitName)
	if !ok {
		t.Fatal("常态：没有胶囊命中区")
	}
	panel, ok := statsDialogHitByName(t, h, open, statsDialogPanelHit)
	if !ok {
		t.Fatal("常态：没有弹窗命中区")
	}
	if panel.Bounds.Width != StatsDialogMinWidth {
		t.Fatalf("常态弹窗宽 = %v，want %v（官方 min-width 生效：内容天然宽 %v 不足 300）",
			panel.Bounds.Width, StatsDialogMinWidth, statsDialogContentWidth(nil, statsDialogRows(statsFullStats()), "2 轮 3 步"))
	}
	if got, want := panel.Bounds.Y+panel.Bounds.Height, pill.Bounds.Y-StatsDialogAnchorGap; math.Abs(float64(got-want)) > 0.01 {
		t.Fatalf("常态弹窗底沿 = %v，want 胶囊顶沿-8 = %v（官方 bottom: anchor(top)+8）", got, want)
	}
	if panel.Bounds.Y+panel.Bounds.Height > pill.Bounds.Y {
		t.Fatal("常态弹窗没有在胶囊上方展开（官方首选方向是上方）")
	}
	if panel.Bounds.X < StatsDialogWindowInset {
		t.Fatalf("常态弹窗左沿 = %v，want >= 12", panel.Bounds.X)
	}

	// ── 窄窗：320 宽 → 官方 max-width min(440, 100vw-24) 生效 = 296，左右各 12。
	narrow := newStatsDialogHarness(t)
	narrowBounds := ui.Rect{Width: 320, Height: 832}
	narrow.frame(t, narrowBounds, data, "", "")
	narrowOpen := narrow.frame(t, narrowBounds, data, statsPillHitName, "")
	narrowPanel, ok := statsDialogHitByName(t, narrow, narrowOpen, statsDialogPanelHit)
	if !ok {
		t.Fatal("窄窗：没有弹窗命中区")
	}
	if want := narrowBounds.Width - 2*StatsDialogWindowInset; narrowPanel.Bounds.Width != want {
		t.Fatalf("窄窗弹窗宽 = %v，want %v（100vw - 24）", narrowPanel.Bounds.Width, want)
	}
	if narrowPanel.Bounds.X != StatsDialogWindowInset {
		t.Fatalf("窄窗弹窗左沿 = %v，want 12", narrowPanel.Bounds.X)
	}
	if right := narrowPanel.Bounds.X + narrowPanel.Bounds.Width; right != narrowBounds.Width-StatsDialogWindowInset {
		t.Fatalf("窄窗弹窗右沿 = %v，want %v（右侧也留 12）", right, narrowBounds.Width-StatsDialogWindowInset)
	}

	// ── 靠右锚点：胶囊贴右边缘时，左沿被往左推到"整块不出窗"。
	rightAnchor := ui.Rect{X: 1100, Y: 810, Width: 120, Height: 22}
	atRight := statsDialogAnchor(rightAnchor, StatsDialogMinWidth, ui.Rect{Width: 1182, Height: 832})
	if want := float32(1182 - StatsDialogWindowInset - StatsDialogMinWidth); atRight.X != want {
		t.Fatalf("靠右锚点左沿 = %v，want %v（官方 left: max(12px, anchor(left)) 后再夹进窗）", atRight.X, want)
	}
	if atRight.X+atRight.Width > 1182-StatsDialogWindowInset {
		t.Fatalf("靠右锚点仍出窗：%+v", atRight)
	}
	// 靠左锚点：官方 max(12px, anchor(left)) 的下界生效。
	atLeft := statsDialogAnchor(ui.Rect{X: 2, Y: 810, Width: 120, Height: 22}, StatsDialogMinWidth, ui.Rect{Width: 1182, Height: 832})
	if atLeft.X != StatsDialogWindowInset {
		t.Fatalf("靠左锚点左沿 = %v，want 12（官方 max(12px, …)）", atLeft.X)
	}
}

// TestStatsDialogWidthClampsLikeOfficial 钉住 `width: max-content` 的两条 clamp。
func TestStatsDialogWidthClampsLikeOfficial(t *testing.T) {
	rows := statsDialogRows(statsFullStats())
	cases := []struct {
		name   string
		bounds float32
		want   float32
	}{
		{"宽窗（min-width 生效）", 1182, StatsDialogMinWidth},
		{"窗口 400 仍是 min-width 300（300 <= 100vw-24）", 400, StatsDialogMinWidth},
		{"更宽也还是 300（内容没到 440 的 max-width）", 4000, StatsDialogMinWidth},
		{"窗口 260：可用宽 236 < 300，按 100vw-24", 260, 260 - 2*StatsDialogWindowInset},
		{"窗口极窄时按可用宽", 200, 200 - 2*StatsDialogWindowInset},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := &pluginapi.Env{Bounds: ui.Rect{Width: tc.bounds, Height: 832}}
			if got := statsDialogWidth(env, rows, "2 轮 3 步"); got != tc.want {
				t.Fatalf("宽 %v 时弹窗宽 = %v，want %v", tc.bounds, got, tc.want)
			}
		})
	}
}

// TestStatsDialogDetailsAlignLikeOfficialGrid 钉住 dt 左沿 / dd 右沿（官方 grid + dd 右对齐）。
func TestStatsDialogDetailsAlignLikeOfficialGrid(t *testing.T) {
	h := newStatsDialogHarness(t)
	data := statsSnapshot(statsFullStats())
	bounds := ui.Rect{Width: 1182, Height: 832}
	h.frame(t, bounds, data, "", "")
	open := h.frame(t, bounds, data, statsPillHitName, "")
	panel, ok := statsDialogHitByName(t, h, open, statsDialogPanelHit)
	if !ok {
		t.Fatal("没有弹窗命中区")
	}
	contentLeft := panel.Bounds.X + StatsDialogPad
	contentRight := panel.Bounds.X + panel.Bounds.Width - StatsDialogPad
	find := func(want string) (gfx.Text, bool) {
		for _, text := range open.Texts() {
			if text.Value == want {
				return text, true
			}
		}
		return gfx.Text{}, false
	}
	// dt：四条标签都从内容左沿起（官方 grid 的 dt 列）。
	for _, label := range []string{zhStatsDialogLLMTime, zhStatsDialogToolTime, zhStatsDialogTTFT, zhStatsDialogSpeed} {
		text, ok := find(label)
		if !ok {
			t.Fatalf("弹窗缺标签 %q", label)
		}
		if text.Bounds.X != contentLeft {
			t.Fatalf("dt %q 左沿 = %v，want %v（内容左沿）", label, text.Bounds.X, contentLeft)
		}
	}
	// dd：每个读数都贴内容右沿（官方 `.details dd { text-align: right }`）。
	for _, value := range []string{"3.3秒", "0.1秒", "1.5秒", "410 tok/s"} {
		text, ok := find(value)
		if !ok {
			t.Fatalf("弹窗缺读数 %q", value)
		}
		if right := text.Bounds.X + text.Bounds.Width; math.Abs(float64(right-contentRight)) > 0.01 {
			t.Fatalf("dd %q 右沿 = %v，want %v（官方 text-align: right）", value, right, contentRight)
		}
	}
	// 标题：章节名贴内容左沿、头条计数贴内容右沿。
	title, _ := find(zhStatsDialogTitle)
	counts, _ := find("2 轮 3 步")
	if title.Bounds.X != contentLeft {
		t.Fatalf("标题左沿 = %v，want %v", title.Bounds.X, contentLeft)
	}
	if right := counts.Bounds.X + counts.Bounds.Width; math.Abs(float64(right-contentRight)) > 0.01 {
		t.Fatalf("头条计数右沿 = %v，want %v（官方 .title space-between）", right, contentRight)
	}
	// 细线在标题与明细之间：标题 mb8 + 线 0.5 + 线 mb10 之后才是第一行明细
	// （细线本身不产生 Text，所以用第一行 dt 的 y 核对这三段间距）。
	firstRow, _ := find(zhStatsDialogLLMTime)
	wantRowY := title.Bounds.Y + title.Bounds.Height + StatsDialogTitleMarginBottom +
		StatsDialogTitleRuleHeight + StatsDialogTitleRuleMarginBottom
	if math.Abs(float64(firstRow.Bounds.Y-wantRowY)) > 0.01 {
		t.Fatalf("第一条明细 y = %v，want %v（标题 + mb8 + 线 0.5 + mb10）", firstRow.Bounds.Y, wantRowY)
	}
	// 行距：官方 `gap: 6px 16px` 的行间距。
	first, _ := find(zhStatsDialogLLMTime)
	second, _ := find(zhStatsDialogToolTime)
	if gap := second.Bounds.Y - (first.Bounds.Y + first.Bounds.Height); math.Abs(float64(gap-StatsDialogDetailsRowGap)) > 0.01 {
		t.Fatalf("明细行距 = %v，want %v", gap, StatsDialogDetailsRowGap)
	}
}
