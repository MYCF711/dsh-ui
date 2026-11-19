package uikernel

// official_model_menu_shadow_test.go —— 钉住模型菜单面的阴影是**官方
// `--dsw-elevation-prominent`**，不是 Viva 的 `gfx.ShadowMD` 预设档（#64 的 ⑧）。
//
// 官方依据（`official_model_menu.go` 文件头本来就写着这一条，代码却用了预设档）：
// `ModelSelect.module.css:107-135` 的 MenuSurface = padding 4 / radius md /
// **elevation-prominent**。
//
// `--dsw-elevation-prominent`（gradient-shadow-text.css:36-37）=
// 0.5px 描边 + `0 3px 8px 0 rgba(0,0,0,.04)` + `0 0 20px 0 rgba(0,0,0,.05)`。
// `gfx.ShadowMD` 是 Tailwind 的 `0 4px 6px -1px .10` + `0 2px 4px -2px .10`：
// 位移/模糊/负扩展/透明度没有一项对，而且**没有描边层**。
//
// 判据双向：① 画布上出现 prominent 的两层柔光；② 不出现 shadow-md 的两层。
// ② 让"改回 ShadowMD"这种回退立刻红。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// nearFloat 是有限小数的比较容差。
func nearFloat(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-6
}

// styleIs 判断一层阴影是否就是给定参数。
func styleIs(s gfx.ShadowStyle, offsetX, offsetY, blur, spread float32) bool {
	return nearFloat(s.OffsetX, offsetX) && nearFloat(s.OffsetY, offsetY) &&
		nearFloat(s.Blur, blur) && nearFloat(s.Spread, spread)
}

// styleList 把画布阴影压成可读列表，失败信息里直接看数值。
func styleList(shadows []gfx.Shadow) []gfx.ShadowStyle {
	styles := make([]gfx.ShadowStyle, 0, len(shadows))
	for _, shadow := range shadows {
		styles = append(styles, shadow.Style)
	}
	return styles
}

// hasGlow 判断画布上是否存在给定的那一层柔光。
func hasGlow(shadows []gfx.Shadow, layer pluginapi.ShadowLayer) bool {
	for _, shadow := range shadows {
		if styleIs(shadow.Style, layer.OffsetX, layer.OffsetY, layer.Blur, layer.Spread) {
			return true
		}
	}
	return false
}

// hasShadowMDPreset 判断画布上是否存在 Tailwind shadow-md 的两层之一。
//
// 这两层来自 `viva/gfx/canvas.go` 的 shadowLayers[ShadowMD]：`0 4px 6px -1px` 与
// `0 2px 4px -2px`。**负扩展是关键特征**——prominent 两层都没有 Spread。
func hasShadowMDPreset(shadows []gfx.Shadow) bool {
	for _, shadow := range shadows {
		if styleIs(shadow.Style, 0, 4, 6, -1) || styleIs(shadow.Style, 0, 2, 4, -2) {
			return true
		}
	}
	return false
}

// TestModelMenuPaintsOfficialProminentElevation 钉模型菜单面的阴影档。
func TestModelMenuPaintsOfficialProminentElevation(t *testing.T) {
	bounds := gfx.Rect{Width: 1182, Height: 832}
	anchor := gfx.Rect{X: 600, Y: 700, Width: 200, Height: ModelTriggerHeight}
	theme := menuTheme()

	opts := ModelMenuOptions{
		Open:   true,
		Known:  true,
		Theme:  theme,
		Anchor: anchor,
		Bounds: bounds,
		Dispatch: func(pluginapi.Slot) {
			// 子槽的内容归占用方；这里只需菜单面被画出来。
			t.Log("子槽已 dispatch")
		},
		Providers: []ModelProviderOption{
			{Provider: "deepseek", DisplayName: "DeepSeek"},
		},
	}

	env := containerEnv(bounds.Width, bounds.Height)
	env.Theme = theme
	var canvas *gfx.Canvas
	canvas = renderContainer(t, bounds.Width, bounds.Height, env, func(f ui.C) {
		DrawModelMenu(f, env, opts)
	})

	if len(canvas.Rectangles()) == 0 {
		t.Fatal("模型菜单该画菜单面，却一个矩形都没有")
	}

	shadows := canvas.Shadows()
	if len(shadows) == 0 {
		t.Fatalf("菜单面一个阴影层都没画（应有两层 prominent 柔光 + 0.5px 描边）")
	}

	for i, layer := range theme.Shadows.Prominent.Glows {
		if layer.Alpha <= 0 {
			t.Fatalf("主题的 Prominent 第 %d 层没有透明度：%+v（令牌没接上）", i, layer)
		}
		if !hasGlow(shadows, layer) {
			t.Fatalf("画布上没有官方 prominent 第 %d 层 %+v；实际阴影 %+v",
				i, layer, styleList(shadows))
		}
	}
	if hasShadowMDPreset(shadows) {
		t.Fatalf("菜单面用了 Tailwind shadow-md 预设档；官方是 elevation-prominent：%+v",
			styleList(shadows))
	}
}
