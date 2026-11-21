package imgdecode

// to_png_test.go —— 「复制图片」那条链的前半段：图片字节 → 可写进剪贴板的 PNG。
//
// 为什么值得单测：viva 的 `app.SetClipboardImage` **只收 PNG**，而工作区里的图
// 可以是 JPEG / GIF。这一步错在"没转码"上时不会报错，只会让用户"点了复制、
// 粘出来是坏的"或者直接失败——两种都必须在这里钉住。

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// pngBytes 编一张 w×h 的纯色 PNG。
func pngBytes(t *testing.T, w, h int, fill color.RGBA) []byte {
	t.Helper()
	source := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			source.Set(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, source); err != nil {
		t.Fatalf("编 PNG 失败：%v", err)
	}
	return buf.Bytes()
}

// jpegBytes 编一张 w×h 的纯色 JPEG（不是 PNG，必须被转码）。
func jpegBytes(t *testing.T, w, h int, fill color.RGBA) []byte {
	t.Helper()
	source := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			source.Set(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, source, nil); err != nil {
		t.Fatalf("编 JPEG 失败：%v", err)
	}
	return buf.Bytes()
}

// decodeSize 解 PNG 并回报尺寸与中心像素色（证明转码出来的是**真图**，不是占位）。
func decodeSize(t *testing.T, body []byte) (int, int, color.RGBA) {
	t.Helper()
	decoded, err := png.Decode(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("转码结果不是能解的 PNG：%v", err)
	}
	bounds := decoded.Bounds()
	center := color.NRGBAModel.Convert(decoded.At(bounds.Dx()/2, bounds.Dy()/2)).(color.NRGBA)
	return bounds.Dx(), bounds.Dy(), color.RGBA{R: center.R, G: center.G, B: center.B, A: center.A}
}

// TestIsPNGUsesSignature：判据是**魔数**，不是扩展名或媒体类型。
//
// 官方按 `blob.type === 'image/png'` 判（type 来自路径扩展名），一个改了后缀的
// JPEG 会被它原样塞进 ClipboardItem。我们按字节判，那种图会被转码成正牌 PNG。
func TestIsPNGUsesSignature(t *testing.T) {
	if !IsPNG(pngBytes(t, 1, 1, color.RGBA{R: 0xFF, A: 0xFF})) {
		t.Fatal("真 PNG 被判成不是 PNG")
	}
	if IsPNG(jpegBytes(t, 1, 1, color.RGBA{R: 0xFF, A: 0xFF})) {
		t.Fatal("JPEG 被判成 PNG：它必须走转码，不能被原样写进剪贴板")
	}
	if IsPNG(nil) || IsPNG([]byte("这不是图片")) {
		t.Fatal("空字节 / 乱字节被判成 PNG")
	}
}

// TestToPNGPassthroughIsByteExact：已经是 PNG 就**一个字节都不动**。
//
// 官方同样是一条早退（`if (blob.type === 'image/png') return blob`，files.ts:69）：
// 重编码只会多一次往返，还会丢掉原始压缩结果。
func TestToPNGPassthroughIsByteExact(t *testing.T) {
	original := pngBytes(t, 4, 3, color.RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF})
	out, err := ToPNG(original)
	if err != nil {
		t.Fatalf("PNG 透传失败：%v", err)
	}
	if !bytes.Equal(out, original) {
		t.Fatalf("PNG 被改写了（%d 字节 → %d 字节）：剪贴板里必须是原始字节", len(original), len(out))
	}
}

// TestToPNGConvertsOtherFormats：JPEG 必须变成 PNG，像素与尺寸保持不变。
//
// 这一条就是官方 `<canvas>` 那一步的等价物（files.ts:70-86）。
func TestToPNGConvertsOtherFormats(t *testing.T) {
	fill := color.RGBA{R: 0x20, G: 0x90, B: 0xD0, A: 0xFF}
	source := jpegBytes(t, 3, 2, fill)
	if IsPNG(source) {
		t.Fatal("夹具本身是 PNG，测不到转码")
	}
	out, err := ToPNG(source)
	if err != nil {
		t.Fatalf("JPEG 转 PNG 失败：%v", err)
	}
	if !IsPNG(out) {
		t.Fatalf("转码结果不是 PNG（前 8 字节 %x）", out[:8])
	}
	width, height, center := decodeSize(t, out)
	if width != 3 || height != 2 {
		t.Fatalf("转码尺寸 = %d×%d，期望 3×2（缩放会改像素）", width, height)
	}
	// JPEG 有损，允许每通道 ±8 的误差；色调必须还是那只蓝色。
	if absDiff(center.R, fill.R) > 8 || absDiff(center.G, fill.G) > 8 || absDiff(center.B, fill.B) > 8 {
		t.Fatalf("转码后中心像素 = %+v，期望接近 %+v", center, fill)
	}
	if center.A != 0xFF {
		t.Fatalf("转码后 alpha = %d，期望不透明 255", center.A)
	}
}

// TestToPNGRejectsEmptyAndGarbage：没有图 / 不认识的格式都**如实失败**，
// 不许拿零字节或空 PNG 冒充成功（那会让剪贴板被清空却什么都没写进去）。
func TestToPNGRejectsEmptyAndGarbage(t *testing.T) {
	if _, err := ToPNG(nil); !errors.Is(err, ErrNoImage) {
		t.Fatalf("空字节 = %v，期望 ErrNoImage", err)
	}
	if _, err := ToPNG([]byte("这不是图片")); err == nil {
		t.Fatal("乱字节必须失败（不认识的格式不许当成功）")
	}
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}
