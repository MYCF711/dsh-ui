package dscommands

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// shadowNear 是阴影数值的比较（这些值都是有限小数，1e-6 足够）。
func shadowNear(a, b float32) bool {
	if a > b {
		return a-b < 1e-6
	}
	return b-a < 1e-6
}

// TestCommandPanelPaintsOfficialProminentElevation 钉命令面板的阴影来源。
//
// 官方（PopupSelectView.module.css:28-29）：
//
//	--dsw-elevation-stroke-color: var(--dsw-alias-border-l1);
//	box-shadow: var(--dsw-elevation-prominent);
//
// = 0.5px border-l1 发丝描边 + `0 3px 8px 0 rgba(0,0,0,.04)` + `0 0 20px 0 rgba(0,0,0,.05)`
// （gradient-shadow-text.css:33,36-37）。
//
// 这里替换掉的旧值是 viva 的 `gfx.ShadowMD`：Tailwind
// `0 4px 6px -1px .10 + 0 2px 4px -2px .10`（viva@v0.6.4/gfx/canvas.go:123）——
// 档位、位移、模糊、负扩展、透明度没有一项对，而且**没有描边层**。
func TestCommandPanelPaintsOfficialProminentElevation(t *testing.T) {
	p := &Plugin{}
	canvas := renderCommandPanel(t, p, scriptedActions(func() {}))
	theme := p.theme
	if theme == nil {
		t.Fatal("渲染后面板还没有主题")
	}
	panel, ok := commandPanelSurface(canvas, theme)
	if !ok {
		t.Fatal("找不到命令面板本体")
	}

	var layers []gfx.Shadow
	for _, shadow := range canvas.Shadows() {
		if shadow.Bounds != panel.Bounds || !shadowNear(shadow.Radius, theme.Radii.MD) {
			continue
		}
		// 同一帧只该出现一次；harness 画两帧，所以按"层"去重后应为 2。
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
	if len(layers) != 2 {
		t.Fatalf("官方 elevation-prominent 是两层柔光，实际去重后 %d 层：%+v", len(layers), layers)
	}

	prominent := theme.Shadows.Prominent
	for i, want := range prominent.Glows[:] {
		got := layers[i]
		if !shadowNear(got.Style.OffsetY, want.OffsetY) || !shadowNear(got.Style.Blur, want.Blur) ||
			!shadowNear(got.Color.Alpha, want.Alpha) {
			t.Errorf("第 %d 层柔光应是官方 elevation-prominent 的 {OffsetY:%v Blur:%v Alpha:%v}，实际 %+v alpha=%v",
				i, want.OffsetY, want.Blur, want.Alpha, got.Style, got.Color.Alpha)
		}
		if got.HasClip {
			t.Errorf("第 %d 层柔光带上了裁剪 %+v：面板自己 `.Clip()`，在 Draw 钩子里画的阴影会被整层裁掉"+
				"（viva/ui/paint.go:50-52 先 Clip、:132 才跑钩子），必须放在 AfterDraw 里画", i, got.ClipOf)
		}
	}

	// 描边：`--dsw-elevation-stroke: 0 0 0 0.5px` 画在外扩 0.5 的矩形上，
	// 颜色是面板自己重绑的 border-l1。
	assertPanelStroke(t, canvas, theme.BorderL1, panel, prominent.Stroke)
}

// assertPanelStroke 核对 0.5px 发丝描边：颜色、以及它确实外扩在面板矩形之外。
func assertPanelStroke(t *testing.T, canvas *gfx.Canvas, stroke gfx.Color, panel gfx.Rectangle, width float32) {
	t.Helper()
	minX, minY := float32(1e9), float32(1e9)
	maxX, maxY := float32(-1e9), float32(-1e9)
	found := 0
	for _, tri := range canvas.Triangles() {
		if tri.Color != stroke {
			continue
		}
		inside := true
		for _, p := range tri.Points {
			if p.X < panel.Bounds.X-width-1e-3 || p.Y < panel.Bounds.Y-width-1e-3 ||
				p.X > panel.Bounds.X+panel.Bounds.Width+width+1e-3 ||
				p.Y > panel.Bounds.Y+panel.Bounds.Height+width+1e-3 {
				inside = false
			}
		}
		if !inside {
			continue // 同色的别处描边（面板内外都有 border-l1），只要面板这一圈
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
		t.Fatalf("找不到 %+v 色的面板描边：官方 elevation 三件套的描边层没了", stroke)
	}
	for _, want := range []struct {
		name string
		got  float32
		want float32
	}{
		{"左边界", minX, panel.Bounds.X - width},
		{"上边界", minY, panel.Bounds.Y - width},
		{"右边界", maxX, panel.Bounds.X + panel.Bounds.Width + width},
		{"下边界", maxY, panel.Bounds.Y + panel.Bounds.Height + width},
	} {
		if !shadowNear(want.got, want.want) {
			t.Errorf("描边%s应为 %v（外扩 %vpx），实际 %v", want.name, want.want, width, want.got)
		}
	}
}
