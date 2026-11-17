package dscreationreco

// official_geometry_test.go —— 本轮「视觉复刻：创建推荐面板」把几何与样式逐条对回 rc.2
// 官方原件之后钉下的值。
//
// 三条证据链（都在 plugin.go / icons.go 的常量注释里给了 file:line）：
//
//	① 卡片自己的 CSS：OFF:dsh-client-ui-creation-recommendations/src/client/Card.module.css
//	   （28 行；.infoButton{width:28px} / .locationButton{font-size:13px} …）；
//	② 按钮原语：OFF:dsh-web-frontend/dist/assets/index-CSAdOBx6.css 的 primitives
//	   `_button_1ulci_2` 块（`._sm_1ulci_28{height:28px;font-size:12px;line-height:18px;
//	   padding:0 10px;border-radius:var(--dsw-radius-sm)}`）——官方 primitives 不随包发 src；
//	③ 图标：sprite `index-ghOpC9uV.js`（sha256=061de8ce…）里 `warning-triangle-outline`
//	   的完整 SVG 原文（viewBox=0 0 16 16、stroke-width=1、linecap/linejoin=round）。
//
// 这些断言挡的是**回退**：谁把 info 按钮的 28 高去掉、把按钮字号改回主题的 14、
// 把 info 底色换回 layer-3、或把图标换回那枚 info，这里都会红。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// TestOfficialWarningTriangleIcon 钉住官方 `IconWarningTriangleOutlineRegular` 的几何。
//
// 官方那一枚是 **viewBox 16** 的纯描边图标（显示尺寸才是 14），两条 path：
// 三角轮廓（含三段 r=1.33 的圆弧，已按标准 arc→cubic 换算）+ 感叹号。
func TestOfficialWarningTriangleIcon(t *testing.T) {
	icon := warningIcon()
	if icon.ViewBox != 16 {
		t.Fatalf("viewBox 应为官方 16（显示尺寸才是 14），实际 %v", icon.ViewBox)
	}
	if icon.Stroke != 1 {
		t.Fatalf("stroke-width 应为 sprite regular 档的 1，实际 %v", icon.Stroke)
	}
	if icon.Cap != gfx.CapRound || icon.Join != gfx.JoinRound {
		t.Fatalf("linecap/linejoin 应为 round（官方 svg 属性），实际 %v/%v", icon.Cap, icon.Join)
	}
	want := []gfx.PathCommand{
		gfx.MoveTo(6.87, 2.6), // M6.87 2.6
		gfx.CubicTo(7.11261, 2.20915, 7.53997, 1.97143, 8, 1.97143),            // a1.33 1.33 0 0 1 2.26 0（前 58°）
		gfx.CubicTo(8.46003, 1.97143, 8.88739, 2.20915, 9.13, 2.6),             // 后半段，收在官方终点 9.13
		gfx.LineTo(14.47, 11.93),                                               // l5.34 9.33
		gfx.CubicTo(14.74693, 12.34319, 14.77015, 12.87637, 14.5302, 13.31207), // A1.33 1.33 0 0 1 13.33 14（前段）
		gfx.CubicTo(14.29025, 13.74777, 13.82723, 14.01316, 13.33, 14),         // 后半段
		gfx.LineTo(2.67, 14), // H2.67
		gfx.CubicTo(2.17277, 14.01316, 1.70975, 13.74777, 1.4698, 13.31207), // a1.33 1.33 0 0 1 -1.14 -2.07（前段）
		gfx.CubicTo(1.22985, 12.87637, 1.25307, 12.34319, 1.53, 11.93),      // 后半段
		gfx.ClosePath(),         // Z
		gfx.MoveTo(8, 6),        // M8 6
		gfx.LineTo(8, 9),        // v3
		gfx.MoveTo(8, 11.33),    // m0 2.33
		gfx.LineTo(8.01, 11.33), // h.01（靠 round cap 成点）
	}
	if len(icon.Commands) != len(want) {
		t.Fatalf("命令数 %d，官方原文换算后应为 %d", len(icon.Commands), len(want))
	}
	for i := range want {
		if icon.Commands[i] != want[i] {
			t.Fatalf("第 %d 条命令 %v，应为 %v", i, icon.Commands[i], want[i])
		}
	}
}

// TestOfficialCreationButtons 钉住本轮从两份官方 CSS 读出的按钮几何。
func TestOfficialCreationButtons(t *testing.T) {
	checks := []struct {
		name string
		got  float32
		want float32
	}{
		// :3 `.infoButton{width:28px;padding:0}` + sm 的 height 28 → 28×28。
		{"info 按钮宽", createdInfoButtonW, 28},
		{"info 按钮高", createdInfoButtonH, 28},
		{"info 图标", createdInfoIcon, 14},
		// :21 `.locationButton{gap:8px;font-size:13px}`。
		{"位置按钮字号", createdLocationButtonFont, 13},
		{"位置按钮 chevron", createdLocationChevron, 14},
		// primitives `._sm_1ulci_28`：12 字号（viva 的 Small() 只有高度/内边距/圆角对得上）。
		{"小按钮字号", createdSmallButtonFont, 12},
		// :4 `.info{ margin:12px 20px 0; padding:12px; font-size:13px; line-height:1.7 }`。
		{"info 上边距", createdInfoMarginTop, 12},
		{"info 内边距", createdInfoPad, 12},
		{"info 字号", createdInfoFont, 13},
		{"info 行高", createdInfoLineH, 22.1},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %v，官方值 %v", check.name, check.got, check.want)
		}
	}
}

// TestInfoBlockUsesModulePlatform 钉住说明块底色取自 `--dsw-alias-bg-module-platform`。
//
// 用两套主题里**故意取不同值**的字段来判：只有取 BgModulePlatform 才两种都过，
// 取 BgLayer3（旧实现的错档）会在下面任何一半失败。
func TestInfoBlockUsesModulePlatform(t *testing.T) {
	for _, theme := range []*pluginapi.Theme{
		{Dark: true, BgModulePlatform: ui.RGBA(0x35, 0x36, 0x38, 1), BgLayer3: ui.RGBA(0x35, 0x36, 0x38, 1)},
		{Dark: false, BgModulePlatform: ui.RGBA(0xf9, 0xfa, 0xfb, 1), BgLayer3: ui.RGBA(0xff, 0xff, 0xff, 1)},
	} {
		if got := noticeFill(theme); got != theme.BgModulePlatform {
			t.Fatalf("暗色=%v：说明块底色应取 BgModulePlatform %v，实际 %v",
				theme.Dark, theme.BgModulePlatform, got)
		}
	}
}
