package dscore

// viva_fork_menu_surface_test.go —— 钉住「我们确实消费的是带 #355 的 Viva fork」。
//
// 背景：`vivaui/viva#355` 给 `DropdownMenuOptions` 加了 `Shadow` / `RowHeight`
// 两个字段（`components/dropdown_menu.go:158-170,257,511-519`）。上游 PR 还没合并，
// 所以 go.mod 用 `replace` 把我们指到自己的 fork 伪版本上（见 go.mod 末尾的注释）。
//
// 这个测试是那条 replace 的**行为证据**，不是编译烟测：
//   - 字段存在：`DropdownMenuOptions{Shadow: &level, RowHeight: 24}` 能编译；
//   - `Shadow` 生效：菜单面铺出去的阴影层，逐层等于同一 theme/radius/bounds 下
//     `gfx.Canvas.Elevate(..., level)` 的结果；
//   - `RowHeight` 生效：行高真的变成 24，而默认（0）时是主题的 control height。
//
// 一旦 replace 被撤掉（退回上游 v0.6.5 或换成没带 #355 的版本），这个文件立刻
// 编译失败 —— 那是**期望的**信号：先确认新版本已经包含这两个字段，再删本测试。

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// menuShadowStyles 把画布上记下的阴影压成「样式序列」，用于逐层比对。
func menuShadowStyles(shadows []gfx.Shadow) []gfx.ShadowStyle {
	styles := make([]gfx.ShadowStyle, 0, len(shadows))
	for _, shadow := range shadows {
		styles = append(styles, shadow.Style)
	}
	return styles
}

// elevatedShadowStyles 是不经过菜单、直接对同一矩形调 Elevate 的参照结果。
func elevatedShadowStyles(theme gfx.Theme, bounds gfx.Rect, level gfx.ShadowLevel) []gfx.ShadowStyle {
	var reference gfx.Canvas
	reference.Elevate(bounds, theme.Metrics.RadiusPopup(), level)
	return menuShadowStyles(reference.Shadows())
}

// sameStyles 逐层比较两组阴影样式。
func sameStyles(a, b []gfx.ShadowStyle) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestVivaForkDropdownMenuExposesShadowAndRowHeight 证明 fork 的两个新字段存在且生效。
func TestVivaForkDropdownMenuExposesShadowAndRowHeight(t *testing.T) {
	theme := gfx.LightTheme()
	bounds := gfx.Rect{X: 10, Y: 20, Width: 160, Height: 120}
	level := gfx.ShadowLG

	t.Run("shadow overrides the default", func(t *testing.T) {
		var canvas gfx.Canvas
		menu := components.BeginDropdownMenu(&canvas, theme, bounds, components.DropdownMenuOptions{
			Layer: 3, Shadow: &level,
		})
		menu.AddItem(components.DropdownMenuItem{ID: gfx.Item(1), Label: "Open"})

		got := menuShadowStyles(canvas.Shadows())
		want := elevatedShadowStyles(theme, bounds, gfx.ShadowLG)
		if !sameStyles(got, want) {
			t.Fatalf("shadow = %+v, want shadow-lg %+v", got, want)
		}
		// 反向判据：不能等于默认那一档，否则「字段被接受但没生效」也会绿。
		if def := elevatedShadowStyles(theme, bounds, gfx.ShadowMD); sameStyles(got, def) {
			t.Fatalf("shadow = %+v, the shadow-md default: Shadow had no effect", got)
		}
	})

	t.Run("nil shadow keeps shadow-md", func(t *testing.T) {
		var canvas gfx.Canvas
		menu := components.BeginDropdownMenu(&canvas, theme, bounds, components.DropdownMenuOptions{Layer: 3})
		menu.AddItem(components.DropdownMenuItem{ID: gfx.Item(1), Label: "Open"})

		got := menuShadowStyles(canvas.Shadows())
		want := elevatedShadowStyles(theme, bounds, gfx.ShadowMD)
		if !sameStyles(got, want) {
			t.Fatalf("default shadow = %+v, want shadow-md %+v", got, want)
		}
	})

	t.Run("shadow wins over the submenu escalation", func(t *testing.T) {
		var canvas gfx.Canvas
		menu := components.BeginDropdownMenu(&canvas, theme, bounds, components.DropdownMenuOptions{
			Layer: 3, Submenu: true, Shadow: &level,
		})
		menu.AddItem(components.DropdownMenuItem{ID: gfx.Item(1), Label: "Open"})

		got := menuShadowStyles(canvas.Shadows())
		want := elevatedShadowStyles(theme, bounds, gfx.ShadowLG)
		if !sameStyles(got, want) {
			t.Fatalf("submenu shadow = %+v, want the caller's shadow-lg %+v", got, want)
		}
	})

	t.Run("row height overrides the theme", func(t *testing.T) {
		var canvas gfx.Canvas
		menu := components.BeginDropdownMenu(&canvas, theme, bounds, components.DropdownMenuOptions{
			Layer: 3, RowHeight: 24,
		})
		row := menu.AddItem(components.DropdownMenuItem{ID: gfx.Item(1), Label: "Open"})
		if row.Height != 24 {
			t.Fatalf("row = %+v, want 24 high", row)
		}
	})

	t.Run("row height zero keeps the theme", func(t *testing.T) {
		var canvas gfx.Canvas
		menu := components.BeginDropdownMenu(&canvas, theme, bounds, components.DropdownMenuOptions{Layer: 3})
		row := menu.AddItem(components.DropdownMenuItem{ID: gfx.Item(1), Label: "Open"})
		if row.Height != theme.Metrics.ControlHeight {
			t.Fatalf("row = %+v, want the theme's %v", row, theme.Metrics.ControlHeight)
		}
	})
}
