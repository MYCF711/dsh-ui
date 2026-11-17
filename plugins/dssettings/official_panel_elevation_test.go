package dssettings

// 这个文件钉住官方设置面板的 elevation：**取值**与**画法**。
//
// 取值：官方 `SettingsRoot.module.css:110`
//
//	.AnXs9G_panel{box-shadow:var(--dsw-elevation-prominent)}
//
// 这一处**没有**重绑 `--dsw-elevation-stroke-color`，所以描边走默认的
// `--dsw-alias-border-l4`（ui-theme/gradient-shadow-text.css:17）。
// 原来本包用的是 viva 的 `gfx.ShadowXS`（Tailwind `0 1px 2px .05`，只有一层、
// 没有描边层），档位、层数、模糊全不是官方那一档。
//
// 画法：卡片自己 `.Clip()`（官方 `.AnXs9G_panel{overflow:hidden}` 的等价物），
// 所以阴影必须在 AfterDraw 里画。viva 是**先推 Clip、再跑 Draw 钩子**
// （`ui/paint.go:50-52` 与 `:132`），在 Draw 钩子里画的阴影带着那道裁剪、
// 又被自己的投射矩形挖空（`gfx/canvas.go:78-80`），画出来是空的——#150 在命令
// 面板上实证过这个 bug。本测试因此断言每层 `HasClip == false`：把阴影搬回
// Draw 钩子，这一条立刻红。

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func shadowNear(a, b float32) bool {
	if a > b {
		return a-b < 1e-6
	}
	return b-a < 1e-6
}

// TestOfficialPanelCastsProminentElevation 钉住设置面板画的是官方
// `--dsw-elevation-prominent` 的两层柔光 + 0.5px border-l4 发丝描边，
// 且柔光没有带上裁剪。
func TestOfficialPanelCastsProminentElevation(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()

	if !h.p.OpenOfficial() {
		t.Fatal("壳接了 settings.describe，OpenOfficial 应该打开")
	}
	h.pump()
	h.pump()
	if !h.p.OfficialShown() {
		t.Fatal("官方面板没打开")
	}
	if h.canvas == nil {
		t.Fatal("没有捕获到画布")
	}

	layout := h.p.PanelLayout()
	card := gfx.Rect{X: layout.X, Y: layout.Y, Width: layout.Width, Height: layout.Height}
	theme := h.theme
	prominent := theme.Shadows.Prominent

	var layers []gfx.Shadow
	for _, shadow := range h.canvas.Shadows() {
		if shadow.Bounds != card || !shadowNear(shadow.Radius, theme.Radii.Panel) {
			continue
		}
		// harness 画多帧：同 (style, alpha) 的算同一层。
		duplicate := false
		for _, seen := range layers {
			if seen.Style == shadow.Style && shadowNear(seen.Color.Alpha, shadow.Color.Alpha) {
				duplicate = true
			}
		}
		if duplicate {
			continue
		}
		if shadow.HasClip {
			t.Errorf("柔光带上了裁剪 %+v：卡片自己 `.Clip()`，在 Draw 钩子里画的阴影会被整层裁掉"+
				"（viva/ui/paint.go:50-52 先 Clip、:132 才跑钩子），必须放在 AfterDraw 里画",
				shadow.ClipOf)
		}
		layers = append(layers, shadow)
	}

	want := prominent.Glows[:]
	if len(layers) != len(want) {
		t.Fatalf("官方 elevation-prominent 是 %d 层柔光，实际画了 %d 层：%+v "+
			"（ShadowXS 只有一层，层数不对就是退回预设档了）", len(want), len(layers), layers)
	}
	for i, w := range want {
		got := layers[i]
		if !shadowNear(got.Style.OffsetY, w.OffsetY) || !shadowNear(got.Style.Blur, w.Blur) ||
			!shadowNear(got.Color.Alpha, w.Alpha) {
			t.Errorf("第 %d 层应是官方 elevation-prominent 的 {OffsetY:%v Blur:%v Alpha:%v}，"+
				"实际 %+v alpha=%v", i, w.OffsetY, w.Blur, w.Alpha, got.Style, got.Color.Alpha)
		}
	}

	assertPanelHairline(t, h, card, theme.BorderL4, prominent.Stroke)
}

// assertPanelHairline 钉 `--dsw-elevation-stroke: 0 0 0 0.5px` 那一圈：
// 颜色是这一处解析出来的描边色，位置在**卡片外扩 stroke** 的矩形上。
func assertPanelHairline(t *testing.T, h *officialHarness, card gfx.Rect, stroke gfx.Color, width float32) {
	t.Helper()
	minX, minY := float32(1e9), float32(1e9)
	maxX, maxY := float32(-1e9), float32(-1e9)
	found := 0
	for _, tri := range h.canvas.Triangles() {
		if tri.Color != stroke {
			continue
		}
		inside := true
		for _, p := range tri.Points {
			if p.X < card.X-width-1e-3 || p.Y < card.Y-width-1e-3 ||
				p.X > card.X+card.Width+width+1e-3 ||
				p.Y > card.Y+card.Height+width+1e-3 {
				inside = false
			}
		}
		if !inside {
			continue // 同色的别处描边（面板里也有 border-l4），只要卡片这一圈
		}
		found++
		for _, p := range tri.Points {
			minX = min(minX, p.X)
			minY = min(minY, p.Y)
			maxX = max(maxX, p.X)
			maxY = max(maxY, p.Y)
		}
	}
	if found == 0 {
		t.Fatalf("找不到 %+v 色的卡片发丝描边：官方 elevation 三件套的描边层没了", stroke)
	}
	// 容差 1px：`RoundedOutline` 在 `pixelScale > 0` 时按 `borderWidth()` 把宽度吸附到
	// 整设备像素（gfx/rounded_outline.go:11-14），100% 缩放下 0.5px 的环可能画成 ~1px。
	// 这是 viva 渲染粒度的限制，不是令牌取值偏差（docs/official-elevation-tokens-evidence-rc2.md §9）。
	const snap = 1.0
	if d := minX - (card.X - width); d > snap || d < -snap {
		t.Errorf("描边左缘 = %v，want ≈%v（官方 `0 0 0 0.5px` 是 spread 0.5，画在卡片外扩 0.5 的矩形上）",
			minX, card.X-width)
	}
	if d := maxY - (card.Y + card.Height + width); d > snap || d < -snap {
		t.Errorf("描边下缘 = %v，want ≈%v", maxY, card.Y+card.Height+width)
	}
	// 四个方向都要在卡片之外（否则画成了内描边或贴着内容边）。
	if minX > card.X+snap || minY > card.Y+snap ||
		maxX < card.X+card.Width-snap || maxY < card.Y+card.Height-snap {
		t.Errorf("描边没有绕在卡片外面：包围盒 %v,%v – %v,%v，卡片 %v,%v – %v,%v",
			minX, minY, maxX, maxY, card.X, card.Y, card.X+card.Width, card.Y+card.Height)
	}
	if minX > maxX || minY > maxY {
		t.Errorf("描边包围盒为空：%v,%v – %v,%v", minX, minY, maxX, maxY)
	}
}
