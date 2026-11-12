package uikernel

// official_popup_flip_test.go —— 翻面的**行为**证据（不是只测算式的纯函数）。
//
// 纯算式由 `official_popup_place_test.go` 钉住；这里回答产品真正的问题：
// **面板画出来之后，整块是不是都在窗口里、里面那个命中区点不点得着。**
//
// 为什么必须单独一条：v0.6.0 的 `Absolute` 不翻面，锚点靠上时面板会顶出窗口
// 上沿——顶出去的那部分**既看不见也点不着**，而编译、单测、门禁全绿。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// drawModelMenuOnce 画一帧模型菜单，返回画布。
func drawModelMenuOnce(t *testing.T, bounds gfx.Rect, anchor gfx.Rect, panelHeight float32) *gfx.Canvas {
	t.Helper()
	env := containerEnv(bounds.Width, bounds.Height)
	opts := ModelMenuOptions{
		Theme:  menuTheme(),
		Anchor: anchor,
		Bounds: bounds,
		Open:   true,
		Known:  true,
		Providers: []ModelProviderOption{
			{Provider: "deepseek", DisplayName: "DeepSeek"},
			{Provider: "openai", DisplayName: "OpenAI-compatible"},
		},
		PanelHeight: panelHeight,
	}
	return renderContainer(t, bounds.Width, bounds.Height, env, func(f ui.C) {
		DrawModelMenu(f, env, opts)
	})
}

// menuSurfaceRect 返回菜单面的外框（`.menu` 的 Background 矩形）。
//
// 菜单面是这一帧**最大的**那个矩形（provider 行只有文字，没有底），所以取面积
// 最大者即可；找不到就报错——那说明菜单根本没画出来。
func menuSurfaceRect(t *testing.T, canvas *gfx.Canvas) gfx.Rect {
	t.Helper()
	var best gfx.Rect
	bestArea := float32(0)
	for _, r := range canvas.Rectangles() {
		if area := r.Bounds.Width * r.Bounds.Height; area > bestArea {
			best, bestArea = r.Bounds, area
		}
	}
	if bestArea <= 0 {
		t.Fatalf("菜单面一个矩形都没有：canvas.Rectangles()=%v", canvas.Rectangles())
	}
	return best
}

// TestModelMenuFlipsBelowWhenNoRoomAbove 是**翻面的产品判据**：
// 锚点靠上时菜单整块落在窗口内（不顶出去），且它在锚点**下方**。
func TestModelMenuFlipsBelowWhenNoRoomAbove(t *testing.T) {
	const (
		winW = 900
		winH = 400
		// 锚点靠上：上方只有 40 - 8 - 12 = 20px，200 高的面板放不下。
		anchorY = 40
		anchorH = 28
	)
	bounds := gfx.Rect{Width: winW, Height: winH}
	anchor := gfx.Rect{X: 600, Y: anchorY, Width: 120, Height: anchorH}

	// 上一帧量到 200 高（> 上方可用 20px）→ 这一帧必须翻到下方。
	canvas := drawModelMenuOnce(t, bounds, anchor, 200)
	menu := menuSurfaceRect(t, canvas)

	if menu.Y < anchor.Y+anchor.Height-0.001 {
		t.Fatalf("菜单没有翻到锚点下方：menu.Y=%v 锚点底=%v", menu.Y, anchor.Y+anchor.Height)
	}
	if menu.Y < ModelMenuEdgeMargin-0.001 {
		t.Fatalf("菜单顶出窗口上沿：menu.Y=%v（inset=%v）", menu.Y, ModelMenuEdgeMargin)
	}
	if menu.Y+menu.Height > winH-ModelMenuEdgeMargin+0.001 {
		t.Fatalf("菜单底出窗口下沿：menu.Bottom=%v 窗口=%v", menu.Y+menu.Height, winH)
	}
}

// TestModelMenuStaysAboveWithBottomAnchor 是正常产品形态的回归：
// 锚点贴窗口底部（composer 里的模型 chip 就在这儿）时菜单留在上方。
func TestModelMenuStaysAboveWithBottomAnchor(t *testing.T) {
	const (
		winW = 900
		winH = 800
	)
	bounds := gfx.Rect{Width: winW, Height: winH}
	anchor := gfx.Rect{X: 600, Y: 740, Width: 120, Height: 28}

	canvas := drawModelMenuOnce(t, bounds, anchor, 200)
	menu := menuSurfaceRect(t, canvas)

	// 判据：整块落在锚点顶再减 gap 之上（高度由内容决定，不假定具体值）。
	if limit := anchor.Y - ModelMenuAnchorGap; menu.Y+menu.Height > limit+0.001 {
		t.Fatalf("菜单应在锚点上方：menu.Bottom=%v 上限=%v", menu.Y+menu.Height, limit)
	}
	if menu.Height <= 0 || menu.Width <= 0 {
		t.Fatalf("菜单面没有面积（点不着）：%+v", menu)
	}
}

// 保证 pluginapi 的 Slot 类型在本文件里可用（DrawModelMenu 的 Dispatch 形参）。
var _ = func(s pluginapi.Slot) {}
