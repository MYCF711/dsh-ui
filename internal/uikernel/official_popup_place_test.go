package uikernel

// official_popup_place_test.go —— `PlaceAbove` 的翻面判据。
//
// 判据直接照 v0.5.1 `Popup().Above(...)` 的 `sideOf`（`ui/popup.go`）：
// 请求的那一侧放得下就用它；放不下时去"另一侧空间更大"的那边；两侧一样宽
// 时留在请求的那侧（不做无谓抖动）。这里逐条钉住，因为 v0.6.0 把 `Popup`
// 整个删掉了，等价行为只能靠这个文件证明。

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestPlaceAboveFlipsWhenNoRoomAbove(t *testing.T) {
	const (
		gap   = 8
		inset = 12
		winH  = 800
	)
	// 锚点贴在窗口底部：它上方 700 多像素都是空的。
	bottomAnchor := gfx.Rect{X: 100, Y: 760, Width: 120, Height: 28}
	// 锚点贴在窗口顶部：它上方只有 inset 往下那点空间（40-8-12=20）。
	topAnchor := gfx.Rect{X: 100, Y: 40, Width: 120, Height: 28}

	for _, tc := range []struct {
		name     string
		anchor   gfx.Rect
		height   float32
		wantSide PopupSide
	}{
		{
			name:   "底部锚点 + 200 高面板 → 留在上方（正常产品形态）",
			anchor: bottomAnchor, height: 200, wantSide: PopupAbove,
		},
		{
			name:   "顶部锚点 + 200 高面板 → 翻到下方",
			anchor: topAnchor, height: 200, wantSide: PopupBelow,
		},
		{
			name:   "顶部锚点 + 0 高（还没量过）→ 按放得下处理，留在上方",
			anchor: topAnchor, height: 0, wantSide: PopupAbove,
		},
		{
			name:   "顶部锚点 + 恰好等于上方空间 → 留在上方（边界，含等号）",
			anchor: topAnchor, height: 40 - gap - inset, wantSide: PopupAbove,
		},
		{
			name:   "顶部锚点 + 比上方空间高 1px → 翻到下方",
			anchor: topAnchor, height: 40 - gap - inset + 1, wantSide: PopupBelow,
		},
		{
			// 面板比窗口还高：两侧都放不下 → 去空间更大的那边（不是永远翻）。
			name:   "两侧都放不下 + 下方更宽 → 翻到下方",
			anchor: gfx.Rect{X: 10, Y: 40, Width: 40, Height: 8},
			height: 500, wantSide: PopupBelow,
		},
		{
			name:   "两侧都放不下 + 上方更宽 → 留在上方",
			anchor: gfx.Rect{X: 10, Y: 700, Width: 40, Height: 8},
			height: 500, wantSide: PopupAbove,
		},
		{
			// 两侧空间完全相同：留在请求的那一侧（稳定，不来回抖）。
			// Y-gap-inset == (winH-inset)-(Y+H+gap) → Y=380, H=40，上下各 360。
			name:   "两侧等宽 → 留在上方",
			anchor: gfx.Rect{X: 0, Y: 380, Width: 40, Height: 40},
			height: 500, wantSide: PopupAbove,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if side := PlaceAbove(tc.anchor, tc.height, gap, winH, inset); side != tc.wantSide {
				t.Fatalf("side = %v，want %v", side, tc.wantSide)
			}
		})
	}
}

func TestPopupSideString(t *testing.T) {
	if PopupAbove.String() != "above" || PopupBelow.String() != "below" {
		t.Fatalf("PopupSide.String 坏了：%q / %q", PopupAbove.String(), PopupBelow.String())
	}
}
