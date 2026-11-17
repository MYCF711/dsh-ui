package dsshortcuts

// 这个文件钉住面板的阴影**取值与画法**。原来面板用的是 viva 的
// `gfx.ShadowLG`（Tailwind `0 10px 15px -3px .10` + `0 4px 6px -4px .10` 两层），
// 与官方的三层完全不是一套数。
//
// 官方出处 `Reference.module.css:63`：
//
//	.dialog{box-shadow:0 0 1px #0003, 0 0 4px #00000005, 0 12px 32px #00000014}
//
// `#0003` = rgba(0,0,0,.2)、`#00000005` ≈ .02、`#00000014` ≈ .08，三层
// 偏移/模糊/透明度逐项等于 `--dsw-shadow-lv3`（ui-theme/gradient-shadow-text.css:8-9）
//
//	0 0 1px 0 rgba(0,0,0,.2), 0 0 4px 0 rgba(0,0,0,.02), 0 12px 32px 0 rgba(0,0,0,.08)
//
// 所以本包画的是令牌 `Shadows.Lv3`（**不是** elevation 三件套：lv3 没有描边层）。
//
// 另一件同样重要的事：面板自己 `.Clip()`，阴影只能在 AfterDraw 里画——本测试
// 因此断言每层 `HasClip == false`。这一条是 #150 在命令面板上踩出来的真 bug：
// viva 先推 Clip（`ui/paint.go:50-52`）再跑 Draw 钩子（`:132`），在 Draw 里画的
// 阴影带着那道裁剪、又被自己的投射矩形挖空，画出来是空的。

import (
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// shadowNear 是阴影数值的比较（都是有限小数，1e-6 足够）。
func shadowNear(a, b float32) bool {
	if a > b {
		return a-b < 1e-6
	}
	return b-a < 1e-6
}

// renderSheet 用真 viva 帧画两遍面板，每遍布局结束后跑一次本插件的 afterDraw，
// 跑的次序与壳一致（`cmd/app/main.go:2178-2182`：布局定了再按序跑 AfterDraw）。
func renderSheet(t *testing.T, p *Plugin) *gfx.Canvas {
	t.Helper()
	p.theme = dscore.Dark()
	p.shown = true
	tree := &gfx.Tree{}
	feat := tree.Feature("shortcuts-shadow")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme:  p.theme,
		Tree:   tree,
		Feat:   feat,
		Canvas: canvas,
		Bounds: gfx.Rect{Width: 1180, Height: 800},
	}
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	for range 2 {
		view.Draw(canvas, env.Bounds, func(f ui.C) { p.draw(f, env) })
		p.afterDraw(canvas, env)
	}
	return canvas
}

// TestSheetCastsOfficialLv3Shadow 钉面板的阴影是官方 `--dsw-shadow-lv3` 的三层，
// 而不是 viva 的 `ShadowLG` 两层。
func TestSheetCastsOfficialLv3Shadow(t *testing.T) {
	p := &Plugin{}
	canvas := renderSheet(t, p)
	theme := p.theme

	rect := p.sheetBox.Rect()
	if rect.Width != sheetWidth || rect.Height != sheetHeight {
		t.Fatalf("面板矩形 = %+v，want %v×%v（官方 .dialog）", rect, sheetWidth, sheetHeight)
	}

	var layers []gfx.Shadow
	for _, shadow := range canvas.Shadows() {
		if shadow.Bounds != rect || !shadowNear(shadow.Radius, sheetRadius) {
			continue
		}
		// 两帧各画一次：同 (style, alpha) 的算同一层。
		duplicate := false
		for _, seen := range layers {
			if seen.Style == shadow.Style && shadowNear(seen.Color.Alpha, shadow.Color.Alpha) {
				duplicate = true
			}
		}
		if !duplicate {
			layers = append(layers, shadow)
		}
	}

	want := theme.Shadows.Lv3[:]
	if len(layers) != len(want) {
		t.Fatalf("官方 --dsw-shadow-lv3 是 %d 层，实际画了 %d 层：%+v（ShadowLG 只有两层，"+
			"层数不对就说明又退回预设档了）", len(want), len(layers), layers)
	}
	for i, w := range want {
		got := layers[i]
		if !shadowNear(got.Style.OffsetX, w.OffsetX) || !shadowNear(got.Style.OffsetY, w.OffsetY) ||
			!shadowNear(got.Style.Blur, w.Blur) || !shadowNear(got.Style.Spread, w.Spread) ||
			!shadowNear(got.Color.Alpha, w.Alpha) {
			t.Errorf("第 %d 层应是官方 lv3 的 {offsetX:%v offsetY:%v blur:%v spread:%v alpha:%v}，"+
				"实际 {offsetX:%v offsetY:%v blur:%v spread:%v} alpha=%v",
				i, w.OffsetX, w.OffsetY, w.Blur, w.Spread, w.Alpha,
				got.Style.OffsetX, got.Style.OffsetY, got.Style.Blur, got.Style.Spread, got.Color.Alpha)
		}
		if got.HasClip {
			t.Errorf("第 %d 层带上了裁剪 %+v：面板自己 `.Clip()`，在 Draw 钩子里画的阴影会被整层裁掉"+
				"（viva/ui/paint.go:50-52 先 Clip、:132 才跑钩子），必须放在 AfterDraw 里画",
				i, got.ClipOf)
		}
	}

	// lv3 **没有描边层**：官方这条规则里没有 0.5px 的环，画了就是自造。
	if theme.Shadows.Lv3[0].Blur == 0 {
		t.Fatal("lv3 第一层 blur=0：viva 画不出 blur=0 的层（gfx/canvas.go:1158 直接 return）")
	}
}

// TestSheetElevationIgnoresDrawHook 钉住"阴影不在 Draw 钩子里画"：
//
// 只跑 Draw、不跑 afterDraw 时，画布上不应该有面板尺寸的阴影——否则说明又有人
// 把阴影写回了 Draw 钩子（那样它会被自己的 Clip 吞掉，等于没画）。
func TestSheetElevationIgnoresDrawHook(t *testing.T) {
	p := &Plugin{}
	p.theme = dscore.Dark()
	p.shown = true
	tree := &gfx.Tree{}
	feat := tree.Feature("shortcuts-shadow-drawonly")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme:  p.theme,
		Tree:   tree,
		Feat:   feat,
		Canvas: canvas,
		Bounds: gfx.Rect{Width: 1180, Height: 800},
	}
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) { p.draw(f, env) })

	rect := p.sheetBox.Rect()
	for _, shadow := range canvas.Shadows() {
		if shadow.Bounds == rect && shadow.Radius == sheetRadius {
			t.Fatalf("只跑 Draw 就已经出现了面板阴影 %+v：它必须留在 afterDraw 里画，"+
				"面板 `.Clip()` 会把 Draw 钩子里的阴影整层裁掉", shadow.Style)
		}
	}
}
