package dsassets

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// DropUpload 由 OFF:dsh-client-ui-attachment/src/DropOverlay.tsx:43-62 的
// UploadIllustration 原 SVG 经 `viva icons` 编译；编译源与哈希留在
// DSH-GO/tmp/GO8-RC2/_vca/drop-upload.svg（sha256=80bad5c3…271e4d5c2）。
func TestAttachmentDropUploadUsesOfficialVector(t *testing.T) {
	icon := DropUpload
	if icon.ViewBox != 115 || icon.ViewHeight != 84 {
		t.Fatalf("插画 viewBox=%vx%v，官方=115x84", icon.ViewBox, icon.ViewHeight)
	}
	if icon.Stroke != 3 || icon.Cap != gfx.CapButt || icon.Join != gfx.JoinMiter {
		t.Fatalf("插画描边=%v cap=%v join=%v，官方=3/butt/miter", icon.Stroke, icon.Cap, icon.Join)
	}
	// 官方 SVG 的 4 条描边 path 各含 2/2/2/5 条命令（`M…L…` ×3 + 一条 4 段 Cubic），
	// 合计 11；Layers 是 5 个填充层。数字来自源 SVG 逐 path 数出来，
	// 不是估的——_vca/drop-upload.svg 是编译源。
	if len(icon.Commands) != 11 || len(icon.Layers) != 5 {
		t.Fatalf("官方四条描边/五个填充层被改动：commands=%d（应 2+2+2+5=11） layers=%d", len(icon.Commands), len(icon.Layers))
	}
	want := []gfx.Color{
		gfx.RGB24(0x9ce5ed), gfx.RGB24(0x679efe), gfx.RGB24(0xffffff),
		gfx.RGB24(0x3964fe), gfx.RGB24(0xffffff),
	}
	for i, layer := range icon.Layers {
		if !nearColor(layer.Fill, want[i]) {
			t.Errorf("layer[%d]=%v，官方=%v", i, layer.Fill, want[i])
		}
	}
}

func nearColor(a, b gfx.Color) bool {
	const tolerance = 0.00001
	return nearChannel(a.Red, b.Red, tolerance) && nearChannel(a.Green, b.Green, tolerance) &&
		nearChannel(a.Blue, b.Blue, tolerance) && nearChannel(a.Alpha, b.Alpha, tolerance)
}

func nearChannel(a, b, tolerance float32) bool {
	delta := a - b
	return delta < tolerance && delta > -tolerance
}
