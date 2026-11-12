package dsstatusbar

// official_icons_test.go —— 官方图标**画进产品位置**的断言（不是只有图标本体的单测）。
//
// 三条判据：
//
//	① 活动/用量胶囊各画官方 14px glyph（gauge / database），胶囊宽度 = 8+14+6+文字+8；
//	② Activity 弹窗标题的 `.titleLabel` 是「14px glyph + gap6 + 章节名」，标题文字
//	   必须比面板内沿右移 StatsDialogTitleIconSize+StatsDialogTitleLabelGap；
//	③ 两处都引用同一张几何表的常量（图标 14 / gap 6），没有第二份魔法数。

import (
	"math"
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
)

// iconTrianglesIn 数落在一个矩形里的**不透明**三角形（图标描边的判据）。
//
// 为什么按矩形数：胶囊本身没有底色（官方 `.pill{background:transparent}`），
// 所以胶囊左侧那 14px 里除了图标不该有任何不透明图元——数出来 >0 就是"图标真的画了"。
func iconTrianglesIn(canvas *gfx.Canvas, rect gfx.Rect) int {
	const opaque = 0.5
	n := 0
	for _, triangle := range canvas.Triangles() {
		if triangle.Color.Alpha < opaque {
			continue
		}
		for _, point := range triangle.Points {
			if point.X >= rect.X && point.X <= rect.X+rect.Width &&
				point.Y >= rect.Y && point.Y <= rect.Y+rect.Height {
				n++
				break
			}
		}
	}
	return n
}

// ① 胶囊：官方 14px 图标 + gap6 + 文字。
//
// 官方出处：
//
//	StatsPills.tsx:255  const icon = <IconGaugeOutlineRegular />      ← 活动
//	StatsPills.tsx:305  const icon = <IconDatabaseOutlineRegular />   ← 用量
//	StatsPills.module.css:17  .pill{gap:6px}
//	StatsPills.module.css:30-34  .pill svg{width:14px;height:14px;flex:none}
func TestOfficialPillDrawsGaugeAndDatabaseIcons(t *testing.T) {
	data := statsSnapshot(statsFullStats())
	specs := officialPillSpecs(data)
	if len(specs) != 2 {
		t.Fatalf("胶囊数 = %d，want 2（活动 / 用量）", len(specs))
	}
	if specs[0].icon != glyphGauge {
		t.Fatalf("活动胶囊图标 = %v，官方 IconGaugeOutlineRegular（sprite gauge-outline）", specs[0].icon)
	}
	if specs[1].icon != glyphDatabase {
		t.Fatalf("用量胶囊图标 = %v，官方 IconDatabaseOutlineRegular（sprite database-outline）", specs[1].icon)
	}

	h := newStatsDialogHarness(t)
	bounds := gfx.Rect{Width: 1182, Height: 832}
	canvas := h.frame(t, bounds, data, "", "")

	// 活动胶囊是这一行的 DialogPill（有计时行 → 可点），矩形在 AfterDraw 里缓存。
	rect := h.plugin.dialog.pillRect
	if rect.Width <= 0 {
		t.Fatal("没有量到活动胶囊的矩形（弹窗锚点没缓存）")
	}
	// 宽度 = 左 pad 8 + 图标 14 + gap 6 + 文字 + 右 pad 8。
	textWidth := statsTextWidth(&pluginapi.Env{Canvas: canvas}, specs[0].text)
	want := 2*pillPadX + pillGap + StatsPillIconSize + textWidth
	if math.Abs(float64(rect.Width-want)) > 1 {
		t.Fatalf("活动胶囊宽 = %v，官方公式 8+14+6+文字(%v)+8 = %v", rect.Width, textWidth, want)
	}
	// 左侧 14px 内必须有图标描边（文本从 x=28 才开始，不可能落进来）。
	iconRect := gfx.Rect{X: rect.X + pillPadX, Y: rect.Y, Width: StatsPillIconSize, Height: pillHeight}
	if n := iconTrianglesIn(canvas, iconRect); n == 0 {
		t.Fatalf("活动胶囊左侧 14px 里没有图元——gauge 图标没画（rect=%+v）", iconRect)
	}
}

// 用量胶囊是 PlainPill（没有官方 tokenUsage 投影前不可点），但**图标照画**：
// 官方那一枚的 icon 与可点性无关（`const icon = <IconDatabaseOutlineRegular />`
// 在 compact / plain / dialog 三个分支里用的是同一枚）。
func TestUsagePillDrawsDatabaseIcon(t *testing.T) {
	h := newStatsDialogHarness(t)
	bounds := gfx.Rect{Width: 1182, Height: 832}
	data := statsSnapshot(statsFullStats())
	canvas := h.frame(t, bounds, data, "", "")
	activity := h.plugin.dialog.pillRect
	if activity.Width <= 0 {
		t.Fatal("活动胶囊矩形缺失")
	}
	// 用量胶囊在活动胶囊右侧一个 dockGap(=12) 处；它的左 14px 里必须有图元。
	usageX := activity.X + activity.Width + dockGap
	iconRect := gfx.Rect{X: usageX + pillPadX, Y: activity.Y, Width: StatsPillIconSize, Height: pillHeight}
	if n := iconTrianglesIn(canvas, iconRect); n == 0 {
		t.Fatalf("用量胶囊左侧 14px 里没有图元——database 图标没画（rect=%+v）", iconRect)
	}
}

// ② 弹窗标题的 `.titleLabel`：官方 `stat-dialog.module.css:64-75`
//
//	.titleLabel{display:inline-flex;align-items:center;gap:6px;min-width:0}
//	.titleLabel svg{width:14px;height:14px;flex:none}
//
// 断言方式是**位置**：标题章节名的 x 必须等于「面板内沿 + 14 + 6」，
// 少一分说明图标没占位，多一分说明 gap 写错。
func TestStatsDialogTitleLabelStartsWithOfficialGlyph(t *testing.T) {
	h := newStatsDialogHarness(t)
	data := statsSnapshot(statsFullStats())
	canvas := h.openDialog(t, data)

	panel := h.plugin.dialog.panelRect
	if panel.Width <= 0 {
		t.Fatal("没有量到弹窗矩形")
	}
	var titleX, titleY float32
	found := false
	for _, text := range canvas.Texts() {
		if text.Value == zhStatsDialogTitle {
			titleX, titleY, found = text.Bounds.X, text.Bounds.Y, true
			break
		}
	}
	if !found {
		t.Fatalf("弹窗里没有章节名 %q：%v", zhStatsDialogTitle, statsDialogTexts(canvas))
	}
	wantX := panel.X + StatsDialogPad + StatsDialogTitleIconSize + StatsDialogTitleLabelGap
	if math.Abs(float64(titleX-wantX)) > 1 {
		t.Fatalf("章节名 x = %v，官方 titleLabel = 面板内沿 %v + 图标 %v + gap %v = %v",
			titleX, panel.X+StatsDialogPad, StatsDialogTitleIconSize, StatsDialogTitleLabelGap, wantX)
	}
	// 图标占位区（章节名左侧 20px）里要有 gauge 的描边图元。
	iconRect := gfx.Rect{
		X: titleX - StatsDialogTitleIconSize - StatsDialogTitleLabelGap, Y: titleY,
		Width: StatsDialogTitleIconSize, Height: StatsDialogTitleIconSize,
	}
	if n := iconTrianglesIn(canvas, iconRect); n == 0 {
		t.Fatalf("titleLabel 图标区没有图元——gauge 图标没画（rect=%+v）", iconRect)
	}
}

// ③ 几何表：两条 icon 行必须与官方值一致（aligned），且实现值就是绘制用的常量。
func TestStatsIconGeometryRowsAreAligned(t *testing.T) {
	for _, tc := range []struct {
		node, prop string
		want       float32
	}{
		{"StatsPills.pill.icon", "size", 14},
		{"StatsDialog.titleLabel.icon", "size", 14},
	} {
		row, ok := statsGeoRow(tc.node, tc.prop)
		if !ok {
			t.Fatalf("几何表缺 %s/%s", tc.node, tc.prop)
		}
		if row.Status != StatsGeoAligned {
			t.Fatalf("%s/%s 状态 = %v，图标已按官方 sprite 转写，应为 aligned（原因：%s）",
				tc.node, tc.prop, row.Status, row.Reason)
		}
		if row.Impl != tc.want || row.Official != tc.want {
			t.Fatalf("%s/%s = impl %v / official %v，want %v", tc.node, tc.prop, row.Impl, row.Official, tc.want)
		}
	}
	if StatsPillIconSize != 14 || StatsDialogTitleIconSize != 14 || StatsDialogTitleLabelGap != 6 || pillGap != 6 {
		t.Fatalf("绘制常量漂了：pill icon %v / title icon %v / title gap %v / pill gap %v",
			StatsPillIconSize, StatsDialogTitleIconSize, StatsDialogTitleLabelGap, pillGap)
	}
}

// 保证测试里用的度量与产品同一口径（产品用 canvas 的 measurer；这里只做阈值断言）。
var _ = utf8.RuneCountInString
