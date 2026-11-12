package dscore

import (
	"reflect"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
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
