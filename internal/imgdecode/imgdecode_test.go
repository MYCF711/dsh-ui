package imgdecode

// imgdecode_test.go —— 共享图片解码器的像素级断言。
//
// 为什么值得单测：这是**两个图片面共用**的一层（头像 + 消息区图片面）。它错在
// 通道顺序上时不会报错，只会让所有图变色——那种错在测试里必须钉住。

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// TestBitmapIsBGRA：`gfx.Bitmap.Pix` 的顺序是 B,G,R,A。
//
// 判据来自 viva `gfx/bitmap_test.go` 的夹具（`0,0,255,255` 是纯蓝）：
// 红色像素必须落在第 3 个字节上。
func TestBitmapIsBGRA(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 2, 1))
	source.Set(0, 0, color.RGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF}) // 纯红
	source.Set(1, 0, color.RGBA{R: 0x00, G: 0x80, B: 0x00, A: 0x40}) // 半透明绿

	bitmap, err := Bitmap(source)
	if err != nil {
		t.Fatalf("Bitmap 失败：%v", err)
	}
	if bitmap.Width != 2 || bitmap.Height != 1 || bitmap.Stride != 8 {
		t.Fatalf("尺寸/Stride = %d×%d/%d，期望 2×1/8", bitmap.Width, bitmap.Height, bitmap.Stride)
	}
	red := bitmap.Pix[0:4]
	if red[0] != 0x00 || red[1] != 0x00 || red[2] != 0xFF || red[3] != 0xFF {
		t.Fatalf("纯红像素 = B%02X G%02X R%02X A%02X，期望 00 00 FF FF", red[0], red[1], red[2], red[3])
	}
	green := bitmap.Pix[4:8]
	if green[0] != 0x00 || green[1] != 0x80 || green[2] != 0x00 || green[3] != 0x40 {
		t.Fatalf("半透明绿 = B%02X G%02X R%02X A%02X，期望 00 80 00 40", green[0], green[1], green[2], green[3])
	}
}

// TestDecodePNGAndFailures：PNG 能解；乱字节与 0×0 都**如实失败**。
func TestDecodePNGAndFailures(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			source.Set(x, y, color.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xFF})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, source); err != nil {
		t.Fatalf("编 PNG 失败：%v", err)
	}
	bitmap, err := Decode(buf.Bytes())
	if err != nil {
		t.Fatalf("解 PNG 失败：%v", err)
	}
	if bitmap.Width != 3 || bitmap.Height != 2 {
		t.Fatalf("解码尺寸 = %d×%d，期望 3×2", bitmap.Width, bitmap.Height)
	}

	if _, err := Decode([]byte("这不是图片")); err == nil {
		t.Fatal("乱字节必须失败（不认识的格式不许当成功）")
	}
	if _, err := Bitmap(image.NewRGBA(image.Rect(0, 0, 0, 0))); err != ErrEmpty {
		t.Fatalf("0×0 必须回 ErrEmpty，实际 %v", err)
	}
}

// TestEmptyBitmapIsZero：失败时回的是**零位图**，调用方据此不画。
func TestEmptyBitmapIsZero(t *testing.T) {
	bitmap, err := Decode(nil)
	if err == nil {
		t.Fatal("空字节必须失败")
	}
	if bitmap.Width != 0 || bitmap.Height != 0 || len(bitmap.Pix) != 0 {
		t.Fatalf("失败时必须是零位图：%+v", gfx.Bitmap{Width: bitmap.Width, Height: bitmap.Height})
	}
}
