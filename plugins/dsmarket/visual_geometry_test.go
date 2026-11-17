package dsmarket

import (
	"testing"

	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
)

// TestOfficialMarketGeometry 钉住本轮从 rc.2 CSS 读出的原语值；旧实现的
// Tag 18/7/10、Pill 18/7/10 与无规格输入框会在这里直接失败。
func TestOfficialMarketGeometry(t *testing.T) {
	checks := []struct {
		name string
		got  float32
		want float32
	}{
		{"tag height", tagHeight, 19},
		{"tag padding-y", tagPadY, 1},
		{"tag padding-x", tagPadX, 8},
		{"tag font", tagFont, 11},
		{"tag line", tagLine, 17},
		{"pill height", marketPillHeight, 24},
		{"pill padding-x", marketPillPadX, 8},
		{"pill font", marketPillFont, 12},
		{"input height", marketInputHeight, 32},
		{"input padding-x", marketInputPadX, 8},
		{"input gap", marketInputGap, 6},
		{"input icon slot", marketInputIcon, 16},
		{"input glyph", marketInputGlyph, 14},
		{"input font", marketInputFont, 14},
		{"input line", marketInputLine, 22},
		{"small button height", marketButtonHeight, 28},
		{"small button padding-x", marketButtonPadX, 10},
		{"small button gap", marketButtonGap, 4},
		{"small button font", marketButtonFont, 12},
		{"small button line", marketButtonLine, 18},
		{"small button icon slot", marketButtonIcon, 16},
		{"artwork frame", discoverIconSize, 48},
		{"artwork glyph", marketArtworkSize, 36},
		{"notice gap", noticeGap, 8},
		{"notice icon", noticeIconSize, 14},
		{"notice icon top", noticeIconTop, 2},
		{"skeleton bar height", marketSkeletonBarHeight, 12},
		{"skeleton bar margin", marketSkeletonBarMargin, 4},
		{"skeleton title max", marketSkeletonTitleMax, 144},
		{"skeleton description max", marketSkeletonDescMax, 280},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %v，官方值 %v", check.name, check.got, check.want)
		}
	}
}

// TestOfficialMarketAssets 验证素材不是文字/首字近似：全部来自官方 path，
// 默认插件图还保留 36 viewBox 与 userSpaceOnUse 渐变端点。
func TestOfficialMarketAssets(t *testing.T) {
	white := gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	for name, build := range map[string]func(gfx.Color) (gfx.Icon, bool){
		"refresh": dsassets.MarketRefreshOutline,
		"warning": dsassets.MarketWarningOutline,
		"trash":   dsassets.MarketTrashOutline,
		"plus":    dsassets.MarketPlusOutline,
	} {
		icon, ok := build(white)
		if !ok || icon.ViewBox != 16 || len(icon.Commands) == 0 {
			t.Errorf("%s 官方 path 没有完整编译：ok=%v viewBox=%v commands=%d", name, ok, icon.ViewBox, len(icon.Commands))
		}
	}
	artwork, ok := dsassets.MarketPluginArtworkDefault()
	if !ok || artwork.ViewBox != 36 || len(artwork.Layers) != 1 || artwork.Layers[0].Gradient == nil {
		t.Fatalf("PluginArtworkDefault 未保留官方 path/渐变：%+v", artwork)
	}
	gradient := artwork.Layers[0].Gradient
	if gradient.Start != (gfx.Point{X: 15.1481, Y: 9.94188}) ||
		gradient.End != (gfx.Point{X: 15.1481, Y: 13.6375}) ||
		gradient.From != gfx.RGB24(0x54ece7) || gradient.To != gfx.RGB24(0x658eff) {
		t.Fatalf("PluginArtworkDefault 渐变不符：%+v", *gradient)
	}
}

func TestOfficialMarketThemeTokens(t *testing.T) {
	for _, tc := range []struct {
		name         string
		dark         bool
		module       gfx.Color
		activeBorder gfx.Color
		skeleton     gfx.Color
	}{
		{name: "dark", dark: true, module: gfx.RGB24(0x353638), activeBorder: gfx.RGB24(0x81858c),
			skeleton: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 0.08}},
		{name: "light", module: gfx.RGB24(0xf9fafb), activeBorder: gfx.RGB24(0x979da6),
			skeleton: gfx.Color{Alpha: 0.04}},
	} {
		theme := dscore.Light()
		if tc.dark {
			theme = dscore.Dark()
		}
		if theme.BgModulePlatform != tc.module {
			t.Errorf("%s bg-module-platform = %+v，官方值 %+v", tc.name, theme.BgModulePlatform, tc.module)
		}
		if theme.ButtonGhostActiveBorder != tc.activeBorder {
			t.Errorf("%s button-ghost-active-border = %+v，官方值 %+v", tc.name, theme.ButtonGhostActiveBorder, tc.activeBorder)
		}
		if theme.BgSkeleton != tc.skeleton {
			t.Errorf("%s bg-skeleton = %+v，官方值 %+v", tc.name, theme.BgSkeleton, tc.skeleton)
		}
	}
}
