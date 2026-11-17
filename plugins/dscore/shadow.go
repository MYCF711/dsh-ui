package dscore

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方阴影令牌：`--dsw-shadow-*` 四档 + `--dsw-elevation-*` 三件套。
//
// 取值逐字抄自
// `@deepseek-ai/dsh-client-ui-theme/src/styles/gradient-shadow-text.css`，
// 抄的是这份**显式声明**，不是 viva 的 Tailwind 预设：
//
//	:5    --dsw-shadow-lv1:      0 2px 4px 0 rgba(0, 0, 0, 0.05)
//	:6    --dsw-shadow-lv1-blur: 0 4px 12px 0 rgba(0, 0, 0, 0.02)
//	:7    --dsw-shadow-lv2:      0 4px 12px 0 rgba(0, 0, 0, 0.02), 0 2px 8px 0 rgba(0, 0, 0, 0.04)
//	:8-9  --dsw-shadow-lv3:      0 0 1px 0 rgba(0, 0, 0, 0.2), 0 0 4px 0 rgba(0, 0, 0, 0.02),
//	                             0 12px 32px 0 rgba(0, 0, 0, 0.08)
//	:33   --dsw-elevation-stroke: 0 0 0 0.5px var(--dsw-elevation-stroke-color)
//	:34-35 --dsw-elevation-panel:     var(--dsw-elevation-stroke), 0 3px 8px 0 rgba(0, 0, 0, 0.03),
//	                                  0 0 16px 0 rgba(0, 0, 0, 0.02)
//	:36-37 --dsw-elevation-prominent: var(--dsw-elevation-stroke), 0 3px 8px 0 rgba(0, 0, 0, 0.04),
//	                                  0 0 20px 0 rgba(0, 0, 0, 0.05)
//	:38-39 --dsw-elevation-soft:      var(--dsw-elevation-stroke), 0 4px 16px 0 rgba(0, 0, 0, 0.03),
//	                                  0 0 24px 0 rgba(0, 0, 0, 0.03)
//
// 为什么不能继续用 `gfx.ShadowMD`：viva 那套是 Tailwind 阶梯，ShadowMD 是
// `0 4px 6px -1px .10 + 0 2px 4px -2px .10`（`viva@v0.6.4/gfx/canvas.go:123`），
// 与官方 lv2 的 `0 4px 12px .02 + 0 2px 8px .04` 是两个东西（位移、模糊、
// 负扩展、透明度全不同），而官方 lv1/lv1-blur/lv3 与三件套在 viva 里**没有对应档**。
//
// 深色主题**没有第二套阴影值**：上面这些令牌只在 `body{}` 里声明一次
// （全官方包 grep `--dsw-shadow` 只命中 gradient-shadow-text.css:5-8），
// `body[data-ds-dark-theme]` 那一段只换线性渐变（:42-45）。所以 Light/Dark
// 两套阴影数值相同；随主题变的是**描边色**，而描边色由组件各自重绑
// `--dsw-elevation-stroke-color`（默认 `--dsw-alias-border-l4`；菜单面重绑 l1、
// 浮窗重绑 l2，见 :17/:24 与 PopupSelectView.module.css:28、dockkit :_float_123xe_158），
// 因此它不进令牌，由调用方传给 PaintElevation。
var (
	shadowLv1 = [1]pluginapi.ShadowLayer{
		{OffsetY: 2, Blur: 4, Alpha: 0.05}, // :5
	}
	shadowLv1Blur = [1]pluginapi.ShadowLayer{
		{OffsetY: 4, Blur: 12, Alpha: 0.02}, // :6
	}
	shadowLv2 = [2]pluginapi.ShadowLayer{
		{OffsetY: 4, Blur: 12, Alpha: 0.02}, // :7 第一层
		{OffsetY: 2, Blur: 8, Alpha: 0.04},  // :7 第二层
	}
	shadowLv3 = [3]pluginapi.ShadowLayer{
		{Blur: 1, Alpha: 0.20},               // :9 第一层（0 0 1px 0 rgba(0,0,0,.2)）
		{Blur: 4, Alpha: 0.02},               // :9 第二层
		{OffsetY: 12, Blur: 32, Alpha: 0.08}, // :9 第三层
	}
)

// elevationStroke 是 `--dsw-elevation-stroke: 0 0 0 0.5px` 的宽度（:33）。
const elevationStroke float32 = 0.5

// shadows 返回整套官方阴影令牌。Light() 与 Dark() 用同一份：官方没有暗色覆盖，
// 见上面那段注释；差别只在调用方传的描边色。
func shadows() pluginapi.Shadows {
	return pluginapi.Shadows{
		Lv1:     shadowLv1,
		Lv1Blur: shadowLv1Blur,
		Lv2:     shadowLv2,
		Lv3:     shadowLv3,

		Panel: pluginapi.Elevation{
			Stroke: elevationStroke,
			Glows: [2]pluginapi.ShadowLayer{
				{OffsetY: 3, Blur: 8, Alpha: 0.03}, // :35 第一层
				{Blur: 16, Alpha: 0.02},            // :35 第二层
			},
		},
		Prominent: pluginapi.Elevation{
			Stroke: elevationStroke,
			Glows: [2]pluginapi.ShadowLayer{
				{OffsetY: 3, Blur: 8, Alpha: 0.04}, // :37 第一层
				{Blur: 20, Alpha: 0.05},            // :37 第二层
			},
		},
		Soft: pluginapi.Elevation{
			Stroke: elevationStroke,
			Glows: [2]pluginapi.ShadowLayer{
				{OffsetY: 4, Blur: 16, Alpha: 0.03}, // :39 第一层
				{Blur: 24, Alpha: 0.03},             // :39 第二层
			},
		},
	}
}

// PaintShadow 画一档 `--dsw-shadow-*` 的全部层，围绕 d.Rect。
//
// 层按官方 CSS 的书写顺序画。叠放次序在视觉上可交换：官方各层都是纯黑，
// 同色不同 alpha 的合成与次序无关，所以不必像 CSS 那样反着来。
//
// 调用点注意：元素自己 `.Clip()` 时它的 Draw 钩子会被一起裁掉
// （`viva/ui/paint.go:50-52` 先 Clip、`:132` 才跑 Draw），阴影会整层消失；
// 这时改用 PaintElevationOverlay / PaintShadowOverlay，在 AfterDraw 里画。
func PaintShadow(d *ui.DrawContext, radius float32, layers []pluginapi.ShadowLayer) {
	paintShadow(d.Canvas, d.Rect, d.Opacity, radius, layers)
}

// PaintShadowOverlay 同 PaintShadow，给 AfterDraw 用：那个阶段没有 DrawContext。
// 元素透明度按 1 处理——标着淡出的浮层应留在 Draw 钩子里画。
func PaintShadowOverlay(canvas *gfx.Canvas, rect gfx.Rect, radius float32, layers []pluginapi.ShadowLayer) {
	paintShadow(canvas, rect, 1, radius, layers)
}

func paintShadow(canvas *gfx.Canvas, rect gfx.Rect, opacity, radius float32, layers []pluginapi.ShadowLayer) {
	for _, layer := range layers {
		canvas.Shadow(rect, radius, gfx.ShadowStyle{
			OffsetX: layer.OffsetX,
			OffsetY: layer.OffsetY,
			Blur:    layer.Blur,
			Spread:  layer.Spread,
		}, gfx.Color{Alpha: layer.Alpha * opacity})
	}
}

// PaintElevation 画一档 `--dsw-elevation-*`：先那一圈发丝描边，再柔光层。
//
// stroke 是 `--dsw-elevation-stroke-color` 解析出来的颜色，由调用方按官方在
// 那一处的重绑传（默认传 t.BorderL4；命令面板/菜单面传 t.BorderL1）。
//
// 描边画在**外扩 0.5px** 的矩形上：官方 `0 0 0 0.5px` 是 spread 0.5 的实心
// 阴影，落在 border box 之外，所以不能贴着 d.Rect 的边画。
func PaintElevation(d *ui.DrawContext, radius float32, e pluginapi.Elevation, stroke ui.Color) {
	paintElevation(d.Canvas, d.Rect, d.Opacity, radius, e, stroke)
}

// PaintElevationOverlay 同 PaintElevation，给 AfterDraw 用。
func PaintElevationOverlay(canvas *gfx.Canvas, rect gfx.Rect, radius float32, e pluginapi.Elevation, stroke ui.Color) {
	paintElevation(canvas, rect, 1, radius, e, stroke)
}

func paintElevation(canvas *gfx.Canvas, rect gfx.Rect, opacity, radius float32, e pluginapi.Elevation, stroke ui.Color) {
	if e.Stroke > 0 && stroke.Alpha > 0 {
		outer := rect
		outer.X -= e.Stroke
		outer.Y -= e.Stroke
		outer.Width += 2 * e.Stroke
		outer.Height += 2 * e.Stroke
		canvas.RoundedOutline(outer, radius+e.Stroke, e.Stroke, ui.Color{
			Red: stroke.Red, Green: stroke.Green, Blue: stroke.Blue, Alpha: stroke.Alpha * opacity,
		})
	}
	paintShadow(canvas, rect, opacity, radius, e.Glows[:])
}
