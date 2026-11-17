package dscore

import (
	"reflect"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 这个包以前一个测试都没有，而它是**所有插件共用的那一层**：
// 颜色令牌、圆角、字号、viva 主题映射。它错了，错的是整个界面。
//
// 这里只钉一类最要命的错：**令牌没设**。
// `gfx.Color` 是 {Red,Green,Blue,Alpha float32}，零值 = 全透明。
// 所以"少写一个字段"不会报错、不会崩，只会让用到它的那块**看不见**——
// 而这正是这个仓反复撞上的那一类现象（按钮在、点得动、屏幕上没有）。

var colorType = reflect.TypeOf(gfx.Color{})

// colorTokens 把一个主题里所有 gfx.Color 字段摊成 名字 → 颜色。
func colorTokens(t *pluginapi.Theme) map[string]gfx.Color {
	out := map[string]gfx.Color{}
	v := reflect.ValueOf(*t)
	typ := v.Type()
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.Type != colorType {
			continue
		}
		out[f.Name] = v.Field(i).Interface().(gfx.Color)
	}
	return out
}

func TestEveryColorTokenIsVisible(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   *pluginapi.Theme
	}{{"Dark", Dark()}, {"Light", Light()}} {
		tokens := colorTokens(tc.th)
		if len(tokens) < 30 {
			t.Fatalf("%s 只反射出 %d 个颜色令牌，结构体是不是改了？", tc.name, len(tokens))
		}
		for name, c := range tokens {
			if c.Alpha <= 0 {
				t.Errorf("%s.%s 的 Alpha = %v（零值 = 全透明）：用到它的地方会**看不见**",
					tc.name, name, c.Alpha)
			}
		}
	}
}

// TestLabelContrastOnBackground 是"看不见"的另一半：颜色都在，但两两撞上了。
//
// 判据用 WCAG 的相对亮度比。阈值取 4.5（AA 正文）而不是更高的 7：
// 我们抄的是官方取值，官方自己未必按 WCAG 调过——**这里卡的是"明显撞色"，
// 不是"审美不达标"**。真有证据说某个值该改，再来调这个数。
func TestLabelContrastOnBackground(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   *pluginapi.Theme
	}{{"Dark", Dark()}, {"Light", Light()}} {
		ratio := contrastRatio(tc.th.LabelPrimary, tc.th.BgBase)
		if ratio < 4.5 {
			t.Errorf("%s 主题：正文色对背景的对比度只有 %.2f:1（阈值 4.5）——正文会看不清",
				tc.name, ratio)
		}
		t.Logf("%s 主题：正文/背景对比度 %.2f:1", tc.name, ratio)
	}
}

// TestVivaThemeHasNoTransparentSlot 是同一个错在**映射层**的版本。
//
// 插件不直接读 Theme 字段时走 `VivaTheme`（viva 自己的主题结构）。
// 那里少写一个字段同样不报错——viva 会用它默认的灰蓝，和我们照抄的官方色打架；
// 而 Chart 这种数组一旦整段忘了填，图表就是全透明。
func TestVivaThemeHasNoTransparentSlot(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   *pluginapi.Theme
	}{{"Dark", Dark()}, {"Light", Light()}} {
		v := VivaTheme(tc.th)
		rv := reflect.ValueOf(v)
		rt := rv.Type()
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			fv := rv.Field(i)
			switch {
			case f.Type == colorType:
				c := fv.Interface().(gfx.Color)
				if c.Alpha <= 0 {
					t.Errorf("%s.VivaTheme.%s 的 Alpha = %v：viva 会回落到它自己的默认色", tc.name, f.Name, c.Alpha)
				}
			case f.Type.Kind() == reflect.Array && f.Type.Elem() == colorType:
				for k := 0; k < fv.Len(); k++ {
					c := fv.Index(k).Interface().(gfx.Color)
					if c.Alpha <= 0 {
						t.Errorf("%s.VivaTheme.%s[%d] 的 Alpha = %v", tc.name, f.Name, k, c.Alpha)
					}
				}
			}
		}
		// Chart 是最容易整段漏填的：5 个槽位必须都在。
		if v.Chart[0].Alpha <= 0 {
			t.Errorf("%s.VivaTheme.Chart[0] 是空的", tc.name)
		}
	}
}

// TestThemesDiffer 钉住"明暗真的是两套"。
//
// 抄错一个值（把 Dark 的底色粘到 Light 里）不会崩，只会让浅色主题看起来
// 像深色主题——而那种问题在截图里非常难认，因为"看起来都挺暗的"。
func TestThemesDiffer(t *testing.T) {
	dark, light := Dark(), Light()
	if dark.Name == light.Name || dark.Dark == light.Dark {
		t.Fatalf("两个主题的 Name/Dark 没区分：%q/%v vs %q/%v", dark.Name, dark.Dark, light.Name, light.Dark)
	}
	for name, pair := range map[string][2]gfx.Color{
		"BgBase":       {dark.BgBase, light.BgBase},
		"LabelPrimary": {dark.LabelPrimary, light.LabelPrimary},
	} {
		if pair[0] == pair[1] {
			t.Errorf("%s 在两个主题里是同一个颜色 %+v——一定有一边抄错了", name, pair[0])
		}
	}
	// 深色主题的正文该比背景亮，浅色主题该反过来。
	if luminance(dark.LabelPrimary) <= luminance(dark.BgBase) {
		t.Errorf("深色主题的正文不比背景亮")
	}
	if luminance(light.LabelPrimary) >= luminance(light.BgBase) {
		t.Errorf("浅色主题的正文不比背景暗")
	}
}

// luminance 是 WCAG 相对亮度的近似（viva 的 RGB24 把 sRGB 值当线性用，
// 这里不额外做 gamma 反变换——比较的是**同一个空间里的相对关系**，够了）。
func luminance(c gfx.Color) float64 {
	return 0.2126*float64(c.Red) + 0.7152*float64(c.Green) + 0.0722*float64(c.Blue)
}

func contrastRatio(a, b gfx.Color) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// TestOfficialTokenValues 把取值钉在官方 CSS 的**具体那一行**上。
//
// 这个包的值不是我们定的，是从
// `dsh-client-ui-theme/src/styles/design-platform.css` 抄的。抄错一个颜色不会崩、
// 不会报错，只会让整块界面偏色；而"浅色主题抄成深色那一套"这种错**看截图看不出来**
// （两边都"挺白的"）。2026-11-17 这轮审计就在浅色一侧抓到四处：bg-module-platform、
// label-primary、markdown-inline-code、bg-document-selection。
//
// 改值前先把官方那一行查出来，连同 file:line 一起改；上表的 source 就是查的地点。
func TestOfficialTokenValues(t *testing.T) {
	dark, light := Dark(), Light()
	for _, tc := range []struct {
		name      string
		got, want gfx.Color
		source    string
	}{
		{"Light.BgModulePlatform", light.BgModulePlatform, gfx.RGB24(0xf5f6f7),
			"design-platform.css:189 → --dsw-static-neutral-bluish-60 亮值 rgb(245,246,247)（:68）"},
		{"Dark.BgModulePlatform", dark.BgModulePlatform, gfx.RGB24(0x353638),
			"design-platform.css:341 → --dsw-static-neutral-bluish-800 rgb(53,54,56)（:72）"},
		{"Light.LabelPrimary", light.LabelPrimary, gfx.RGB24(0x0f1115),
			"design-platform.css:259 → --dsw-static-neutral-bluish-1000 rgb(15,17,21)（:57）"},
		{"Dark.LabelPrimary", dark.LabelPrimary, gfx.RGB24(0xf9fafb),
			"design-platform.css:411 → --dsw-static-neutral-bluish-50 rgb(249,250,251)（:65）"},
		{"Light.InlineCode", light.InlineCode, gfx.RGB24(0xfafafa),
			"design-platform.css:270 → --dsw-static-neutral-50 rgb(250,250,250)（:49）"},
		{"Dark.InlineCode", dark.InlineCode, gfx.RGB24(0x292929),
			"design-platform.css:422 → --dsw-static-neutral-800 rgb(41,41,41)（:53）"},
		{"Light.Selection", light.Selection, ui.RGBA(59, 130, 246, 0.40),
			"design-platform.css:179 → color-mix(--dsw-static-blue-500 40%, transparent)"},
		{"Dark.Selection", dark.Selection, ui.RGBA(59, 130, 246, 0.40),
			"design-platform.css:331 → color-mix(--dsw-static-blue-500 40%, transparent)"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %+v，官方值 %+v（%s）", tc.name, tc.got, tc.want, tc.source)
		}
	}

	// 圆角六档：base.css:18-23，一个不能差。
	if want := (pluginapi.Radii{XS: 4, SM: 8, MD: 12, LG: 16, XL: 20, Panel: 28}); light.Radii != want {
		t.Errorf("Radii = %+v，官方 base.css:18-23 是 %+v", light.Radii, want)
	}

	// 字号取官方四档（默认正文 14 时）：
	// --dsw-font-s-14 14/22、-xs-13 13/20、-xxs-12 12/18、-xxxs-11 11/14
	// （gradient-shadow-text.css:220,234,248,262）。
	f := light.Fonts
	for _, c := range []struct {
		name      string
		got, want float32
	}{
		{"Body", f.Body, 14}, {"BodyLine", f.BodyLine, 22},
		{"Secondary", f.Secondary, 13}, {"SecondaryLine", f.SecondaryLine, 20},
		{"Caption", f.Caption, 12}, {"CaptionLine", f.CaptionLine, 18},
		{"Tiny", f.Tiny, 11},
	} {
		if c.got != c.want {
			t.Errorf("Fonts.%s = %v，官方 %v（gradient-shadow-text.css:220-267）", c.name, c.got, c.want)
		}
	}
}
