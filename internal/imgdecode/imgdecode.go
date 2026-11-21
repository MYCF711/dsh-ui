// Package imgdecode 把图片字节解码成 viva 的 `gfx.Bitmap`。
//
// # 为什么要有这一层
//
// 官方界面里所有图片面都是浏览器 `<img>`：字节进来、解码、绘制，三件事浏览器
// 都包了。本仓没有浏览器，viva 的 `ui.Image(c, gfx.Bitmap)` 只吃**解码后的位图**
// （`viva@0.6.5/ui/elements.go:53` → `gfx.Bitmap{Pix, Width, Height, Stride}`，
// Pix 顺序 B,G,R,A —— 见 viva `gfx/bitmap_test.go` 的夹具：`0,0,255,255` 是纯蓝）。
// 所以"字节 → 位图"是每个图片面都要走的一步。
//
// 两个使用方：头像（`plugins/dssettings/official_account_avatar.go`）与
// 消息区图片面（`plugins/dsconversation/official_message_image.go`）。它们**不是**
// 两份逻辑：解码格式集合与像素布局是同一个事实，重复实现迟早会漂。
//
// 第三个使用方是**反方向**的：`#18` ①「复制图片」要把字节写进系统剪贴板，而 viva
// 的写入口只收 PNG（`app.SetClipboardImage`），所以非 PNG 的图要先转码——
// 见 `to_png.go`。它复用同一套解码器注册，理由同上：格式集合只许有一份。
//
// # 认哪些格式
//
// 官方 `<img>` 能解 PNG / JPEG / GIF / WebP；`session.attachment` 的宿主门也只放
// `image/png|jpeg|webp|gif` 四种（`HOST:internal/transport/handlers_session_official.go:1534-1540`）。
// Go 标准库覆盖 PNG/JPEG/GIF（WebP 需要第三方包，本仓没有引入——不认识的格式
// **如实失败**，由调用方画官方失败态，不许拿零位图冒充成功）。
package imgdecode

import (
	"bytes"
	"errors"
	"image"
	// 官方浏览器能解这三种；标准库只需要注册解码器。
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/vivaui/viva/gfx"
)

// ErrEmpty 表示解码出来的图是 0×0（不是一个能画的东西）。
var ErrEmpty = errors.New("imgdecode: 空位图")

// Decode 把图片字节解码成 viva 位图。
func Decode(body []byte) (gfx.Bitmap, error) {
	decoded, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return gfx.Bitmap{}, err
	}
	return Bitmap(decoded)
}

// Bitmap 把 `image.Image` 转成 viva 的 BGRA 位图（Stride = Width*4）。
func Bitmap(source image.Image) (gfx.Bitmap, error) {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return gfx.Bitmap{}, ErrEmpty
	}
	stride := width * 4
	pixels := make([]byte, stride*height)
	for y := 0; y < height; y++ {
		row := pixels[y*stride : (y+1)*stride]
		for x := 0; x < width; x++ {
			red, green, blue, alpha := source.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			index := x * 4
			row[index] = byte(blue >> 8)
			row[index+1] = byte(green >> 8)
			row[index+2] = byte(red >> 8)
			row[index+3] = byte(alpha >> 8)
		}
	}
	return gfx.Bitmap{Pix: pixels, Width: width, Height: height, Stride: stride}, nil
}
