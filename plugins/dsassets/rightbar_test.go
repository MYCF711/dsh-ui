package dsassets

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestRightbarOfficialIcons(t *testing.T) {
	panel, ok := RightbarPanelLeft()
	if !ok || panel.ViewBox != 16 || panel.Stroke != 1 {
		t.Fatalf("panel-left = %+v, %v", panel, ok)
	}
	last := panel.Commands[len(panel.Commands)-2:]
	if last[0].Point.X != 10.5 || last[0].Point.Y != 1.5 || last[1].Point.X != 10.5 || last[1].Point.Y != 14.5 {
		t.Fatalf("scaleX(-1) 后分隔线错误：%+v", last)
	}
	full, ok := RightbarFullscreen()
	if !ok || full.ViewBox != 16 || len(full.Commands) == 0 || full.Stroke != 0 {
		t.Fatalf("fullscreen = %+v, %v", full, ok)
	}
	exit, ok := RightbarExitFullscreen()
	if !ok || exit.ViewBox != 16 || exit.Stroke != 1 || len(exit.Commands) == 0 {
		t.Fatalf("exit-fullscreen = %+v, %v", exit, ok)
	}
}

func TestFileTypeMarkdownUsesOfficialLayersAndTransform(t *testing.T) {
	base, mark := gfx.RGB24(0x4176e6), gfx.RGB24(0xffffff)
	icon, ok := FileTypeMarkdown(base, mark)
	if !ok || icon.ViewBox != 28 || len(icon.Layers) != 3 {
		t.Fatalf("markdown = %+v, %v", icon, ok)
	}
	if icon.Layers[0].Fill != base || icon.Layers[2].Fill != mark {
		t.Fatalf("markdown 颜色层错误：%+v", icon.Layers)
	}
	if got, want := icon.Layers[1].Fill.Alpha, mark.Alpha*0.7; got != want {
		t.Fatalf("折角 alpha=%v，官方 0.7 应为 %v", got, want)
	}
	first := icon.Layers[2].Commands[0].Point
	// (8.7588,19.5) 经官方 mu 变换为 (14+(x-14)*1.22,16+(y-16)*1.22)。
	wantX := float32(14) + (float32(8.7588)-14)*1.22
	wantY := float32(16) + (float32(19.5)-16)*1.22
	if first.X != wantX || first.Y != wantY {
		t.Fatalf("MD markTransform 错误：%+v", first)
	}
}
