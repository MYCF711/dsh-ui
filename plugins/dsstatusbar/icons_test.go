package dsstatusbar

// icons_test.go —— 官方 sprite 图标的**逐点断言**（rc.2 口径）。
//
// 三条判据：
//
//	① spinner 是**真动画**：1.5s 周期里不同时刻的旋转角/弧长不同，周期末尾回到起点
//	   （官方 `dsh-state-dot-spin` / `dsh-state-dot-dash` 两条 keyframes）；
//	② 减动效档按官方 media query 停成静态弧（dasharray 18 / offset -3）；
//	③ gauge / database 是官方 sprite 原文（viewBox 16、weight regular=1、首点逐位相同、
//	   gauge 的圆点是 fill 层），不是几何近似。

import (
	"math"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// TestSpinnerFrameAnimatesOverTime 断言"不同时间点角度不同"（官方 1.5s 线性旋转 +
// 1.5s ease-in-out 的 dash 动画）。
func TestSpinnerFrameAnimatesOverTime(t *testing.T) {
	t0 := time.Unix(0, 0)
	start := spinnerFrameAt(t0, false)
	t.Logf("t=0      rotate=%.3f° sweep=%.3f° start=%.3f° dash=%.2f offset=%.2f",
		start.Rotate, start.sweepDeg(), start.startDeg(), start.Dash, start.Offset)

	// 官方 dasharray 起点 12 → 弧长 12/59.6903×360 = 72.3736°。
	if diff := start.sweepDeg() - 72.3736; math.Abs(float64(diff)) > 0.01 {
		t.Fatalf("t=0 弧长 = %.4f°，官方 stroke-dasharray:12 应为 72.3736°", start.sweepDeg())
	}
	if start.Rotate != 0 {
		t.Fatalf("t=0 旋转角 = %v，应为 0", start.Rotate)
	}

	// 250ms（周期 1/6）：线性旋转 60°。
	quarter := spinnerFrameAt(t0.Add(250*time.Millisecond), false)
	if math.Abs(float64(quarter.Rotate-60)) > 0.01 {
		t.Fatalf("250ms 旋转角 = %v，官方 1.5s linear 应为 60°", quarter.Rotate)
	}
	if math.Abs(float64(quarter.startDeg()-start.startDeg())) < 1 {
		t.Fatalf("250ms 起角与 t=0 相同（%v vs %v）——spinner 没有在转",
			start.startDeg(), quarter.startDeg())
	}

	// 750ms（周期 1/2）：dash 到 50% 关键帧，弧长 24/59.6903×360 = 144.7472°。
	mid := spinnerFrameAt(t0.Add(750*time.Millisecond), false)
	if math.Abs(float64(mid.sweepDeg()-144.7472)) > 0.02 {
		t.Fatalf("750ms 弧长 = %.4f°，官方 keyframes 50%%{dasharray:24} 应为 144.7472°", mid.sweepDeg())
	}
	if math.Abs(float64(mid.Rotate-180)) > 0.01 {
		t.Fatalf("750ms 旋转角 = %v，应为 180°", mid.Rotate)
	}
	// 50% 关键帧的 dashoffset=-6 → 起角比纯旋转多 36.1868°。
	wantMidStart := float32(180 + 36.1868)
	if diff := mid.startDeg() - wantMidStart; math.Abs(float64(diff)) > 0.05 {
		t.Fatalf("750ms 起角 = %.3f°，官方 dashoffset:-6 应为 %.3f°", mid.startDeg(), wantMidStart)
	}

	// 两个不同时刻的角度必须不同（任务书点名的那条断言）。
	if start.startDeg() == quarter.startDeg() || start.sweepDeg() == mid.sweepDeg() {
		t.Fatal("spinner 两帧完全相同——动画没跑起来")
	}

	// 两段各自的 `ease-in-out`（不能把整条动画当一个段插值）：
	//   375ms = 第一段走一半 → dash 12+12·easeInOut(0.5) = 21.3067
	//   1125ms = 第二段走一半 → dash 12+12·(1-easeInOut(0.5)) = 14.6933
	firstHalf := spinnerFrameAt(t0.Add(375*time.Millisecond), false)
	if diff := firstHalf.Dash - float32(12+12*0.775561); math.Abs(float64(diff)) > 0.01 {
		t.Fatalf("375ms dash = %v，官方第一段 ease-in-out 半程应为 %v", firstHalf.Dash, 12+12*0.775561)
	}
	secondHalf := spinnerFrameAt(t0.Add(1125*time.Millisecond), false)
	if diff := secondHalf.Dash - float32(12+12*(1-0.775561)); math.Abs(float64(diff)) > 0.01 {
		t.Fatalf("1125ms dash = %v，官方第二段 ease-in-out 半程应为 %v",
			secondHalf.Dash, 12+12*(1-0.775561))
	}
	// 第二段必须是"往回收"（dash 减小），不是又涨上去。
	if secondHalf.Dash >= mid.Dash {
		t.Fatalf("1125ms dash = %v 应小于 750ms 的 %v（第三段在收）", secondHalf.Dash, mid.Dash)
	}
	if math.Abs(float64(secondHalf.Offset-(-6*(1-0.775561)))) > 0.02 {
		t.Fatalf("1125ms offset = %v，官方第二段半程应为 %v", secondHalf.Offset, -6*(1-0.775561))
	}

	// 一个周期后回到起点。
	next := spinnerFrameAt(t0.Add(spinnerPeriod), false)
	if math.Abs(float64(next.Rotate-start.Rotate)) > 0.01 || math.Abs(float64(next.Dash-start.Dash)) > 1e-4 {
		t.Fatalf("一个周期后 (%v,%v) 应回到起点 (%v,%v)", next.Rotate, next.Dash, start.Rotate, start.Dash)
	}
}

// TestSpinnerReducedMotionIsStaticArc 断言减动效档（官方
// `@media (prefers-reduced-motion:reduce){.spinnerMotion,.spinnerArc{animation:none}
// .spinnerArc{stroke-dasharray:18 150;stroke-dashoffset:-3}}`）。
func TestSpinnerReducedMotionIsStaticArc(t *testing.T) {
	t0 := time.Unix(0, 0)
	a := spinnerFrameAt(t0, true)
	b := spinnerFrameAt(t0.Add(500*time.Millisecond), true)
	if a != b {
		t.Fatalf("减动效档两帧不同：%+v vs %+v", a, b)
	}
	if a.Rotate != 0 {
		t.Fatalf("减动效档旋转 = %v，官方 animation:none 应为 0", a.Rotate)
	}
	if a.Dash != 18 || a.Offset != -3 {
		t.Fatalf("减动效档 dash/offset = %v/%v，官方 18/-3", a.Dash, a.Offset)
	}
	if diff := a.sweepDeg() - 108.5604; math.Abs(float64(diff)) > 0.01 {
		t.Fatalf("减动效弧长 = %.4f°，官方 18 单位应为 108.5604°", a.sweepDeg())
	}
}

// TestSpinnerGlyphMatchesOfficialGeometry 断言 spinner 的官方几何：
// viewBox 24（官方 `<svg viewBox="0 0 24 24">`）、stroke 2、圆头、r=9.5 的圆环 track
// （25% alpha + EvenOdd）。
func TestSpinnerGlyphMatchesOfficialGeometry(t *testing.T) {
	icon := spinnerGlyph(spinnerFrameAt(time.Unix(0, 0), false), gfx.Color{Red: 1, Alpha: 1})
	if icon.ViewBox != 24 || icon.Stroke != 2 {
		t.Fatalf("spinner viewBox/stroke = %v/%v，官方 24/2", icon.ViewBox, icon.Stroke)
	}
	if icon.Cap != gfx.CapRound {
		t.Fatalf("spinner cap = %v，官方 stroke-linecap:round", icon.Cap)
	}
	if len(icon.Layers) != 1 {
		t.Fatalf("spinner 应有 1 层 track（25%% 圆环），实得 %d", len(icon.Layers))
	}
	layer := icon.Layers[0]
	if layer.FillRule != gfx.PathFillEvenOdd {
		t.Fatalf("track 填充规则 = %v，圆环必须 EvenOdd", layer.FillRule)
	}
	if math.Abs(float64(layer.Fill.Alpha-0.25)) > 1e-6 {
		t.Fatalf("track alpha = %v，官方 opacity:.25", layer.Fill.Alpha)
	}
	// track 是两个同心圆（外 r+1 / 内 r-1），各 4 段贝塞尔 + Close。
	moves := 0
	for _, command := range layer.Commands {
		if command.Kind == gfx.PathCommandMove {
			moves++
		}
	}
	if moves != 2 {
		t.Fatalf("track 轮廓数 = %d，want 2（外圆 + 内圆）", moves)
	}
}

// TestGaugeIconIsOfficialSprite 断言 gauge 是 rc.2 sprite `gauge-outline` 原文：
// viewBox 16 / weight regular=1 / 三条 path（两条 stroke + 一条 fill 圆点）/
// 首点与官方 d 的起点逐位相同。
func TestGaugeIconIsOfficialSprite(t *testing.T) {
	tint := gfx.Color{Red: 0.2, Green: 0.4, Blue: 0.8, Alpha: 1}
	icon := gaugeOutlineIcon(tint)
	if icon.ViewBox != 16 {
		t.Fatalf("gauge viewBox = %v，官方 `viewBox=\"0 0 16 16\"`", icon.ViewBox)
	}
	if icon.Stroke != 1 {
		t.Fatalf("gauge stroke = %v，官方 weight regular = 1", icon.Stroke)
	}
	first := icon.Commands[0]
	if first.Kind != gfx.PathCommandMove || first.Point != (ui.Point{X: 3.4041, Y: 13.096}) {
		t.Fatalf("gauge 首命令 = %+v，官方 d 起点 M3.4041 13.096", first)
	}
	// 第二条 stroke：指针 M8 8.49994 L11.6114 4.88855。
	pointer := -1
	for i, command := range icon.Commands {
		if command.Kind == gfx.PathCommandMove && command.Point == (ui.Point{X: 8, Y: 8.49994}) {
			pointer = i
		}
	}
	if pointer < 0 {
		t.Fatal("gauge 缺官方指针 path（M8 8.49994 L11.6114 4.88855）")
	}
	if icon.Commands[pointer+1] != gfx.LineTo(11.6114, 4.88855) {
		t.Fatalf("gauge 指针终点 = %+v，官方 L11.6114 4.88855", icon.Commands[pointer+1])
	}
	// 第三条是 fill 圆点 → 必须落在 Layers（stroke 命令画不了 fill）。
	if len(icon.Layers) != 1 {
		t.Fatalf("gauge fill 层数 = %d，官方第三条 path 是 fill=\"currentColor\"", len(icon.Layers))
	}
	if icon.Layers[0].Fill != tint {
		t.Fatalf("gauge 圆点色 = %+v，应等于同一枚 tint（currentColor）", icon.Layers[0].Fill)
	}
	knob := icon.Layers[0].Commands
	if knob[0].Kind != gfx.PathCommandMove || knob[0].Point != (ui.Point{X: 8, Y: 9.75}) {
		t.Fatalf("gauge 圆点首命令 = %+v，官方 M8 9.75", knob[0])
	}
}

// TestDatabaseIconIsOfficialSprite 断言 database 是 rc.2 sprite `database-outline` 原文：
// viewBox 16 / stroke 1 / 五条 stroke path（顶盖 + 两条母线 + 两条层线）/ 无 fill 层。
func TestDatabaseIconIsOfficialSprite(t *testing.T) {
	icon := databaseOutlineIcon()
	if icon.ViewBox != 16 || icon.Stroke != 1 {
		t.Fatalf("database viewBox/stroke = %v/%v，官方 16/1", icon.ViewBox, icon.Stroke)
	}
	if len(icon.Layers) != 0 {
		t.Fatalf("database 不该有 fill 层，实得 %d", len(icon.Layers))
	}
	first := icon.Commands[0]
	if first.Kind != gfx.PathCommandMove || first.Point != (ui.Point{X: 13.1967, Y: 5.1869}) {
		t.Fatalf("database 首命令 = %+v，官方 d 起点 M13.1967 5.1869", first)
	}
	moves := 0
	for _, command := range icon.Commands {
		if command.Kind == gfx.PathCommandMove {
			moves++
		}
	}
	if moves != 5 {
		t.Fatalf("database path 段数 = %d，官方 sprite 是 5 条 path", moves)
	}
}

// TestEaseCurvesMatchOfficialBezier 断言两条缓动就是官方 CSS 的 timing function
// （不能把 t 直接当参数用——那样 0.5 处会差 8%）。
func TestEaseCurvesMatchOfficialBezier(t *testing.T) {
	// ease-out = cubic-bezier(0,0,.58,1)：0.25/0.5/0.75 的参考值见测试常量。
	for _, tc := range []struct{ t, want float32 }{
		{0, 0}, {0.25, 0.378138}, {0.5, 0.684643}, {0.75, 0.906535}, {1, 1},
	} {
		if got := easeOut(tc.t); absDiff(got, tc.want) > 5e-4 {
			t.Fatalf("easeOut(%v) = %v，官方 cubic-bezier(0,0,.58,1) 应为 %v", tc.t, got, tc.want)
		}
	}
	// ease-in-out = cubic-bezier(.4,0,.2,1)（结构树 §动效的 --ds-ease-in-out）。
	for _, tc := range []struct{ t, want float32 }{
		{0, 0}, {0.25, 0.236587}, {0.5, 0.775561}, {0.75, 0.959368}, {1, 1},
	} {
		if got := easeInOut(tc.t); absDiff(got, tc.want) > 5e-4 {
			t.Fatalf("easeInOut(%v) = %v，官方 cubic-bezier(.4,0,.2,1) 应为 %v", tc.t, got, tc.want)
		}
	}
	// 端点外的值必须夹住（CSS 的 progress 只在 [0,1]）。
	if easeOut(-1) != 0 || easeOut(2) != 1 || easeInOut(-1) != 0 || easeInOut(2) != 1 {
		t.Fatal("缓动函数没有把进度夹到 [0,1]")
	}
}
