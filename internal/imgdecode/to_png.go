package imgdecode

// to_png.go —— 反方向：把图片字节变成**能写进剪贴板的 PNG**（`#18` ①）。
//
// # 为什么需要它
//
// 官方「复制图片」在渲染层做两步（`OFF:dsh-client-ui-file-actions/src/client/files.ts:65-89`）：
//
//	if (blob.type === 'image/png') return blob          // 已经是 PNG：原样交出去
//	…canvas.drawImage → canvas.toBlob('image/png')      // 不是 PNG：浏览器转码
//	navigator.clipboard.write([new ClipboardItem({ 'image/png': png })])
//
// 本仓的剪贴板写入口是 viva 的 `app.SetClipboardImage(png []byte) error`，它**只收
// PNG**：空字节、不以 PNG 魔数开头、超过 `app.MaxClipboardImageBytes`（32 MiB）都
// 是错误，且**不动剪贴板**（`viva@v0.6.10-clip/app/clipboard_image.go:74-90` 与
// `internal/platform/clipboard_image.go:32-42` 的 `CheckClipboardPNG`）。所以"不是
// PNG 的图"必须先转码——这一步就是官方那个 `<canvas>` 的等价物。
//
// 解码器注册仍只有一份（`imgdecode.go` 的包注释 + 那里的 `_ "image/*"`），
// 两种格式判断（"能上屏的"与"能进剪贴板的"）不允许各写一份。

import (
	"bytes"
	"errors"
	"image"
	"image/png"
)

// pngSignature 是 PNG 的 8 字节魔数。
//
// 与 viva 的 `platform.CheckClipboardPNG` 同一判据（同一份字节前缀），
// 所以"我们说它是 PNG"与"写入口愿意收"永远一致。
var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// ErrNoImage 表示输入里根本没有图片字节（不是"图是空的"，是"没有图"）。
var ErrNoImage = errors.New("imgdecode: 没有图片字节")

// IsPNG 报告 body 是否已经是一份 PNG。
//
// **按魔数判，不看扩展名、也不看媒体类型**：官方按 `blob.type === 'image/png'`
// 判（`files.ts:69`），而 blob 的 type 来自路径扩展名——一个改了后缀的 JPEG
// 会被官方原样交给 `ClipboardItem` 然后写坏。按魔数判严格更好且零代价：
// 错后缀的图会被我们**转码**成正牌 PNG 再写，而不是把坏字节塞进剪贴板。
func IsPNG(body []byte) bool { return bytes.HasPrefix(body, pngSignature) }

// ToPNG 把图片字节变成可以交给 `app.SetClipboardImage` 的 PNG。
//
//	已经是 PNG → 原样返回**同一份字节**（不重编码：官方同样是直接交出去，
//	             重编码只会多一次无意义的往返，还会丢掉原始压缩结果）
//	其他格式   → 解码 + 重新编码成 PNG（官方用 canvas，本仓用标准库）
//
// 压缩级别取 `png.DefaultCompression` 而不是 `BestSpeed`：剪贴板写入口有
// 32 MiB 上限，一张 4K JPEG 用 BestSpeed 编出来可能顶到上限而被拒；默认级别
// 慢几十毫秒但稳过。**宁可慢一点，也不能"点了复制却什么都没进剪贴板"**。
//
// 认的格式就是包注释那一套（PNG / JPEG / GIF）。GIF 只取第一帧——官方 canvas
// 画 `drawImage` 同样只有第一帧，不是我们的简化。WebP 没有解码器：如实报错，
// 由调用方写官方的失败文案。
func ToPNG(body []byte) ([]byte, error) {
	if len(body) == 0 {
		return nil, ErrNoImage
	}
	if IsPNG(body) {
		return body, nil
	}
	decoded, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	// bytes.Buffer 上的 png.Encode 不会失败（内存写）；错误照原样上抛，不吞。
	if err := png.Encode(&out, decoded); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
