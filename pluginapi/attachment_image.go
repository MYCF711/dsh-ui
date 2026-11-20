package pluginapi

// AttachmentBlob 是一份已登记附件的**元数据 + 原始字节**（宿主 `attachment.get`）。
//
// 为什么字节要走到界面这一层：官方消息里的图片面是浏览器 `<img>`，
// URL 由 `Gateway` 授权后直接交给渲染器（`OFF:dsh-client-ui-attachment/src/
// MessageImage.tsx:11-13` 的 `ImageLoader`）。本仓没有浏览器，`<img>` 的等价物是
// viva 的 `ui.Image(c, gfx.Bitmap)`——它只吃**解码后的位图**，所以必须先把字节
// 取回来自己解码（与头像同一条路，见 `plugins/dssettings/official_account_avatar.go`）。
//
// 宿主侧证据：真读文件字节、在唯一出口编成 `dataBase64`
// （`HOST:internal/transport/api.go:119-122,135-137,384-395`；handler
// `HOST:internal/transport/server.go:967-1027`，路由 `:823-826`）。本仓客户端
// `internal/apiclient/attachments.go:95-110` 已返回 `(*Attachment, []byte, error)`。
type AttachmentBlob struct {
	// ID 是宿主登记的附件 id（`attachmentId`）。
	ID string
	// Name 是原始文件名。
	Name string
	// MIME 是宿主探测出的真实 MIME（`image/png` 等）。
	MIME string
	// ByteSize 是字节数（宿主真值，0 字节的附件宿主根本不会登记）。
	ByteSize int64
	// Width / Height 是**图片**的原始像素尺寸；宿主对非图片不填 → 0。
	//
	// 这两个值就是官方 `singleFit` 要的 `dimensions`
	// （`OFF:dsh-client-ui-attachment/src/MessageImage.tsx:36-48` 的入参，
	// 官方由 `ImageAttachmentRef` 带过来）。**不要**用解码后的位图尺寸顶替：
	// 宿主在 Put 时已经按真实字节校验过（`HOST:cmd/dshd/attachments.go:540-546`
	// 的 MIME/尺寸一致性断言），两份真值不一致时该信宿主那一份。
	Width  int
	Height int
	// Bytes 是原始字节（未解码）。
	Bytes []byte
}
