package dscore

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
)

// 这一组测试钉的是**官方四档阴影 + 三件套的精确数值**。
//
// 为什么单独立一组：阴影是"看得见但不报错"那一类——写错一个数不会崩、
// 不会红，只会让每张浮层的边都跟官方差一点。而这次系统化的起因就是
// 20+ 插件各自写 `gfx.ShadowMD`（Tailwind 阶梯），谁也没发现自己抄错了档。
//
// 全部取值出自
// `@deepseek-ai/dsh-client-ui-theme/src/styles/gradient-shadow-text.css`
// （官方 rc.2 包内随源码发布，逐条 file:line 见下）。

// shadowAbs 是 float32 的绝对差，阴影数值都是有限的十进制小数，1e-6 足够严。
func shadowAbs(a, b float32) float32 {
	if a > b {
		return a - b
	}
	return b - a
}

type wantLayer struct {
	x, y, blur, spread, alpha float32
}

type wantShadow struct {
	token string
	got   []pluginapi.ShadowLayer
	want  []wantLayer
	src   string
}

// wantShadows 从一套主题里摊出「令牌名 / 拿到的层 / 官方应有的层 / 出处」。
// Light 与 Dark 都过同一张表：官方只有一个 `body{}` 值，没有暗色覆盖。
func wantShadows(sh pluginapi.Shadows) []wantShadow {
	return []wantShadow{
		{"--dsw-shadow-lv1", sh.Lv1[:],
			[]wantLayer{{0, 2, 4, 0, 0.05}}, "gradient-shadow-text.css:5"},
		{"--dsw-shadow-lv1-blur", sh.Lv1Blur[:],
			[]wantLayer{{0, 4, 12, 0, 0.02}}, "gradient-shadow-text.css:6"},
		{"--dsw-shadow-lv2", sh.Lv2[:],
			[]wantLayer{
				{0, 4, 12, 0, 0.02}, // `0 4px 12px 0 rgba(0,0,0,.02)`
				{0, 2, 8, 0, 0.04},  // `0 2px 8px 0 rgba(0,0,0,.04)`
			}, "gradient-shadow-text.css:7"},
		{"--dsw-shadow-lv3", sh.Lv3[:],
			[]wantLayer{
				{0, 0, 1, 0, 0.20},   // `0 0 1px 0 rgba(0,0,0,.2)`
				{0, 0, 4, 0, 0.02},   // `0 0 4px 0 rgba(0,0,0,.02)`
				{0, 12, 32, 0, 0.08}, // `0 12px 32px 0 rgba(0,0,0,.08)`
			}, "gradient-shadow-text.css:8-9"},
	}
}

func TestOfficialShadowTokens(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   *pluginapi.Theme
	}{{"Light", Light()}, {"Dark", Dark()}} {
		for _, want := range wantShadows(tc.th.Shadows) {
			if len(want.got) != len(want.want) {
				t.Errorf("%s %s 应为 %d 层，实际 %d 层：%+v",
					tc.name, want.token, len(want.want), len(want.got), want.got)
				continue
			}
			for i, w := range want.want {
				g := want.got[i]
				if shadowAbs(g.OffsetX, w.x) > 1e-6 || shadowAbs(g.OffsetY, w.y) > 1e-6 ||
					shadowAbs(g.Blur, w.blur) > 1e-6 || shadowAbs(g.Spread, w.spread) > 1e-6 ||
					shadowAbs(g.Alpha, w.alpha) > 1e-6 {
					t.Errorf("%s %s 第 %d 层 ≠ 官方（%s）：\n  得到 %+v\n  应为 {OffsetX:%v OffsetY:%v Blur:%v Spread:%v Alpha:%v}",
						tc.name, want.token, i, want.src, g, w.x, w.y, w.blur, w.spread, w.alpha)
				}
			}
		}
	}
}

// TestOfficialElevationTokens 钉三件套：描边 0.5px + 两层柔光。
//
// 官方的 `--dsw-elevation-stroke: 0 0 0 0.5px <色>`（:33）是 spread 0.5 的
// 实心阴影，viva 画不出 blur=0 的层，所以令牌里它单列成 Stroke。
func TestOfficialElevationTokens(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   *pluginapi.Theme
	}{{"Light", Light()}, {"Dark", Dark()}} {
		for _, want := range []struct {
			token string
			got   pluginapi.Elevation
			glows []wantLayer
			src   string
		}{
			{"--dsw-elevation-panel", tc.th.Shadows.Panel,
				[]wantLayer{{0, 3, 8, 0, 0.03}, {0, 0, 16, 0, 0.02}},
				"gradient-shadow-text.css:34-35"},
			{"--dsw-elevation-prominent", tc.th.Shadows.Prominent,
				[]wantLayer{{0, 3, 8, 0, 0.04}, {0, 0, 20, 0, 0.05}},
				"gradient-shadow-text.css:36-37"},
			{"--dsw-elevation-soft", tc.th.Shadows.Soft,
				[]wantLayer{{0, 4, 16, 0, 0.03}, {0, 0, 24, 0, 0.03}},
				"gradient-shadow-text.css:38-39"},
		} {
			// 描边宽度：官方三档共用一条 `--dsw-elevation-stroke`，都是 0.5px（:33）。
			if shadowAbs(want.got.Stroke, 0.5) > 1e-6 {
				t.Errorf("%s %s 的描边宽度应为 0.5px（gradient-shadow-text.css:33），实际 %v",
					tc.name, want.token, want.got.Stroke)
			}
			glows := want.got.Glows[:]
			if len(glows) != len(want.glows) {
				t.Errorf("%s %s 应有 %d 层柔光，实际 %d 层：%+v",
					tc.name, want.token, len(want.glows), len(glows), glows)
				continue
			}
			for i, w := range want.glows {
				g := glows[i]
				if shadowAbs(g.OffsetX, w.x) > 1e-6 || shadowAbs(g.OffsetY, w.y) > 1e-6 ||
					shadowAbs(g.Blur, w.blur) > 1e-6 || shadowAbs(g.Spread, w.spread) > 1e-6 ||
					shadowAbs(g.Alpha, w.alpha) > 1e-6 {
					t.Errorf("%s %s 第 %d 层柔光 ≠ 官方（%s）：\n  得到 %+v\n  应为 {OffsetX:%v OffsetY:%v Blur:%v Spread:%v Alpha:%v}",
						tc.name, want.token, i, want.src, g, w.x, w.y, w.blur, w.spread, w.alpha)
				}
			}
		}
	}
}

// TestShadowsHaveNoDarkOverride 钉一个**事实**，不是偏好：
//
// 官方全包 grep `--dsw-shadow` 只命中 gradient-shadow-text.css:5-8，三件套在
// :31-40 的 `body, body *` 里也只声明一次；`body[data-ds-dark-theme]` 那两段
// （:42-45、:23-25）只换线性渐变与菜单面的描边色。所以浅/暗两套阴影数值必须
// **完全相同**。哪天有人给暗色"调深一点"，这里会红——那就得先拿出官方证据。
func TestShadowsHaveNoDarkOverride(t *testing.T) {
	light, dark := Light().Shadows, Dark().Shadows
	for _, pair := range []struct {
		token       string
		light, dark []pluginapi.ShadowLayer
	}{
		{"--dsw-shadow-lv1", light.Lv1[:], dark.Lv1[:]},
		{"--dsw-shadow-lv1-blur", light.Lv1Blur[:], dark.Lv1Blur[:]},
		{"--dsw-shadow-lv2", light.Lv2[:], dark.Lv2[:]},
		{"--dsw-shadow-lv3", light.Lv3[:], dark.Lv3[:]},
		{"--dsw-elevation-panel", light.Panel.Glows[:], dark.Panel.Glows[:]},
		{"--dsw-elevation-prominent", light.Prominent.Glows[:], dark.Prominent.Glows[:]},
		{"--dsw-elevation-soft", light.Soft.Glows[:], dark.Soft.Glows[:]},
	} {
		if len(pair.light) != len(pair.dark) {
			t.Errorf("%s 浅/暗层数不同：%d vs %d", pair.token, len(pair.light), len(pair.dark))
			continue
		}
		for i := range pair.light {
			if pair.light[i] != pair.dark[i] {
				t.Errorf("%s 第 %d 层浅/暗不同：浅 %+v / 暗 %+v（官方只有一套值）",
					pair.token, i, pair.light[i], pair.dark[i])
			}
		}
	}
	if shadowAbs(light.Panel.Stroke, dark.Panel.Stroke) > 1e-6 ||
		shadowAbs(light.Prominent.Stroke, dark.Prominent.Stroke) > 1e-6 ||
		shadowAbs(light.Soft.Stroke, dark.Soft.Stroke) > 1e-6 {
		t.Error("三件套描边宽度浅/暗不同；官方只有一条 --dsw-elevation-stroke（:33）")
	}
}

// TestEveryShadowLayerIsPaintable 是 theme_test.go「令牌没设」那条的另一半：
//
// `gfx.Canvas.Shadow` 对 Blur<=0 或 Alpha<=0 直接 return（canvas.go:1158）。
// 所以令牌里少写一个数不会崩，只会让这一层**静默地不存在**——和"看不见的
// 按钮"同一类坑。这里要求每一层都真的能被画出来。
func TestEveryShadowLayerIsPaintable(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   *pluginapi.Theme
	}{{"Light", Light()}, {"Dark", Dark()}} {
		sh := tc.th.Shadows
		all := map[string][]pluginapi.ShadowLayer{
			"Lv1":       sh.Lv1[:],
			"Lv1Blur":   sh.Lv1Blur[:],
			"Lv2":       sh.Lv2[:],
			"Lv3":       sh.Lv3[:],
			"Panel":     sh.Panel.Glows[:],
			"Prominent": sh.Prominent.Glows[:],
			"Soft":      sh.Soft.Glows[:],
		}
		for name, layers := range all {
			if len(layers) == 0 {
				t.Errorf("%s.Shadows.%s 一层都没有：官方这一档是显式声明的，不是可选", tc.name, name)
				continue
			}
			for i, layer := range layers {
				if layer.Blur <= 0 {
					t.Errorf("%s.Shadows.%s[%d].Blur = %v：viva 画不出 blur=0 的层（canvas.go:1158），这一层等于不存在",
						tc.name, name, i, layer.Blur)
				}
				if layer.Alpha <= 0 {
					t.Errorf("%s.Shadows.%s[%d].Alpha = %v：全透明层不画任何东西",
						tc.name, name, i, layer.Alpha)
				}
			}
		}
		for name, stroke := range map[string]float32{
			"Panel": sh.Panel.Stroke, "Prominent": sh.Prominent.Stroke, "Soft": sh.Soft.Stroke,
		} {
			if stroke <= 0 {
				t.Errorf("%s.Shadows.%s.Stroke = %v：官方 0.5px 发丝描边不能为 0", tc.name, name, stroke)
			}
		}
	}
}

// TestPaintElevationOverlayGeometry 钉画法：描边画在外扩 Stroke 的矩形上、半径同步
// 加 Stroke，柔光层画在元素自己的矩形上。
//
// 为什么外扩：官方 `0 0 0 0.5px` 是 spread 0.5 的实心阴影，落点在 border box
// **之外**；贴着 d.Rect 的边画等于吃掉 0.5px 的可见差。
func TestPaintElevationOverlayGeometry(t *testing.T) {
	canvas := &gfx.Canvas{}
	rect := gfx.Rect{X: 100, Y: 200, Width: 300, Height: 120}
	radius := float32(12)
	stroke := gfx.Color{Red: 0, Green: 0, Blue: 0, Alpha: 0.16}
	PaintElevationOverlay(canvas, rect, radius, Dark().Shadows.Panel, stroke)

	shadows := canvas.Shadows()
	if len(shadows) != 2 {
		t.Fatalf("panel 应画 2 层柔光，实际 %d 层：%+v", len(shadows), shadows)
	}
	for i, want := range []gfx.ShadowStyle{
		{OffsetY: 3, Blur: 8},
		{Blur: 16},
	} {
		got := shadows[i]
		if got.Bounds != rect {
			t.Errorf("第 %d 层柔光应画在元素矩形 %+v 上，实际 %+v", i, rect, got.Bounds)
		}
		if got.Style != want {
			t.Errorf("第 %d 层柔光样式应为 %+v，实际 %+v", i, want, got.Style)
		}
	}

	// 描边：RoundedOutline 拆成三角形，取它们的包围盒核对"外扩 0.5"。
	var ring []gfx.Triangle
	minX, minY := float32(1e9), float32(1e9)
	maxX, maxY := float32(-1e9), float32(-1e9)
	for _, tri := range canvas.Triangles() {
		if tri.Color != stroke {
			continue
		}
		ring = append(ring, tri)
		for _, p := range tri.Points {
			minX = min(minX, p.X)
			minY = min(minY, p.Y)
			maxX = max(maxX, p.X)
			maxY = max(maxY, p.Y)
		}
	}
	if len(ring) == 0 {
		t.Fatal("没有画出 0.5px 发丝描边：官方 elevation 三件套的第一层就是它（gradient-shadow-text.css:33）")
	}
	for _, want := range []struct {
		name string
		got  float32
		want float32
	}{
		{"左边界", minX, rect.X - 0.5},
		{"上边界", minY, rect.Y - 0.5},
		{"右边界", maxX, rect.X + rect.Width + 0.5},
		{"下边界", maxY, rect.Y + rect.Height + 0.5},
	} {
		if shadowAbs(want.got, want.want) > 1e-6 {
			t.Errorf("描边%s应为 %v（外扩 0.5px），实际 %v", want.name, want.want, want.got)
		}
	}
}

// TestPaintShadowOverlayPaintsEveryLayer 钉 `--dsw-shadow-lv*` 的画法：层数、
// 顺序、以及每层都落在元素矩形上（柔光不外扩矩形，扩的是 Blur）。
func TestPaintShadowOverlayPaintsEveryLayer(t *testing.T) {
	canvas := &gfx.Canvas{}
	rect := gfx.Rect{X: 0, Y: 0, Width: 200, Height: 80}
	lv3 := Dark().Shadows.Lv3[:]
	PaintShadowOverlay(canvas, rect, 8, lv3)

	shadows := canvas.Shadows()
	if len(shadows) != len(lv3) {
		t.Fatalf("lv3 有 %d 层，画出来 %d 层", len(lv3), len(shadows))
	}
	for i, got := range shadows {
		want := lv3[i]
		if got.Bounds != rect || got.Radius != 8 {
			t.Errorf("第 %d 层的位置/圆角不对：%+v radius=%v", i, got.Bounds, got.Radius)
		}
		if shadowAbs(got.Style.OffsetY, want.OffsetY) > 1e-6 ||
			shadowAbs(got.Style.Blur, want.Blur) > 1e-6 ||
			shadowAbs(got.Color.Alpha, want.Alpha) > 1e-6 {
			t.Errorf("第 %d 层 ≠ 令牌：得到 %+v alpha=%v，令牌 %+v alpha=%v",
				i, got.Style, got.Color.Alpha, want, want.Alpha)
		}
	}
}
