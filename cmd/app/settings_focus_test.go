//go:build darwin || windows || linux

// settings_focus_test.go —— 设置模态里 Tab 必须**看得见地把焦点挪一格**。
//
// 官方 `useModalLayer(panel, true, onClose)`（结构树 §2.2）的 Tab 语义是
// "下一个 focusable" —— 它同时意味着两件事：焦点真的在面板里，且用户能看见
// 它在哪一格。本仓的 `ModalLayer.Focused()`（plugins/dssettings/official_modal.go）
// 只维护插件侧的一份字符串账；渲染器的焦点要靠 `gfx.Canvas.RequestFocus` 才会动。
//
// 2026-11-22 修之前：设置打开后连按 Tab，`Snapshot().Focused` 一直是零，
// 画面也**一个像素都不变**——键盘用户在面板里完全看不到自己在哪，
// Enter 也没有落点。
//
// 判据（全是真键盘）：
//
//	① Tab 之后渲染器的焦点非零，且落在面板卡片矩形内；
//	② 下一次 Tab 焦点换到**另一个**命中区（不是原地不动）；
//	③ 焦点格的像素与上一帧不同（Focus 视觉真的画出来了）。
package main

import (
	"image"
	"testing"

	"github.com/MYCF711/dsh-ui/plugins/dssettings"
	"github.com/vivaui/viva/gfx"
)

// openSettingsOverlay 打开官方四区设置面板（与 hitsFrames 的 open 同一段逻辑）。
func openSettingsOverlay(t *testing.T, p *hitsProduct) {
	t.Helper()
	pl, ok := p.plugins["ds-settings"].(*dssettings.Plugin)
	if !ok {
		t.Fatalf("ds-settings 不是 *dssettings.Plugin：%T", p.plugins["ds-settings"])
	}
	opened := pl.Open()
	if pl.OfficialEnabled() {
		opened = pl.OpenOfficial()
	}
	if !opened {
		t.Fatalf("设置浮层打不开（OfficialEnabled=%v）", pl.OfficialEnabled())
	}
}

// settingsCardRect 是这一尺寸下设置卡片的矩形（与绘制用同一个求解器）。
func settingsCardRect(t *testing.T, width, height float32) gfx.Rect {
	t.Helper()
	layout, err := dssettings.LayoutSettingsPanel(width, height, dssettings.PanelOverlayTop)
	if err != nil {
		t.Fatalf("设置面板布局求解失败：%v", err)
	}
	return gfx.Rect{X: layout.X, Y: layout.Y, Width: layout.Width, Height: layout.Height}
}

// frameDiff 数两张同尺寸图里有差异的像素。
func frameDiff(a, b *image.RGBA) int {
	if a == nil || b == nil || a.Bounds() != b.Bounds() {
		return -1
	}
	diff := 0
	bounds := a.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				diff++
			}
		}
	}
	return diff
}

func TestSettingsModalTabMovesVisibleFocus(t *testing.T) {
	const width, height = 1182, 832
	frame := openDXFrame(t, hitsFrame{
		name:   "settings-tab",
		snap:   hitsSessionFixture(),
		open:   openSettingsOverlay,
		width:  width,
		height: height,
	}, width, height)

	card := settingsCardRect(t, width, height)
	if card.Width <= 0 || card.Height <= 0 {
		t.Fatalf("设置卡片矩形无效：%+v", card)
	}

	var seen []gfx.ID
	for i := 0; i < 6; i++ {
		beforeImg, err := frame.session.Image()
		if err != nil {
			t.Fatalf("取帧：%v", err)
		}
		if err := frame.session.Keys("tab"); err != nil {
			t.Fatalf("第 %d 次 Tab：%v", i+1, err)
		}
		focused := frame.snapshot().Focused
		if focused.IsZero() {
			t.Fatalf("第 %d 次 Tab 之后渲染器焦点是零：Tab 的落点没有交给渲染器 "+
				"（插件侧账本再准，画面上也看不见）", i+1)
		}
		hit, ok := frame.hitByID(focused)
		if !ok {
			t.Fatalf("第 %d 次 Tab 的焦点 %v 不在本帧命中区里", i+1, focused)
		}
		cx, cy := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
		if cx < card.X || cy < card.Y || cx >= card.X+card.Width || cy >= card.Y+card.Height {
			t.Fatalf("第 %d 次 Tab 的焦点 %q 在设置卡片外面：焦点 %+v，卡片 %+v",
				i+1, hit.Label, hit.Bounds, card)
		}
		if i > 0 && focused == seen[i-1] {
			t.Fatalf("第 %d 次 Tab 焦点没有移动（仍是 %q）：ModalLayer 的 Tab 账要交给渲染器",
				i+1, hit.Label)
		}
		seen = append(seen, focused)

		afterImg, err := frame.session.Image()
		if err != nil {
			t.Fatalf("取帧：%v", err)
		}
		if got := frameDiff(beforeImg, afterImg); got <= 0 {
			t.Fatalf("第 %d 次 Tab 之后画面没有任何变化（焦点在 %q 上不可见）",
				i+1, hit.Label)
		}
	}
	if len(seen) < 2 {
		t.Fatalf("只走到 %d 个焦点停靠点", len(seen))
	}
}
