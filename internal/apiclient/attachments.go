// attachments.go —— 附件三条宿主通道：`attachment.put` / `attachment.get` /
// `session.attachment`（官方 `dsh-client-ui-attachment` 的上传面）。
//
// # 为什么这三条必须在这里补
//
// `docs/official-parity.md` 里 `dsh-client-ui-attachment` 是 🟡 PARTIAL，缺的正是
// 「拖入/上传入口」：`conversation.input.attachments` 这个 rail 槽**有座位、有派发，
// 但全仓没有 Panels() 注册者**。座位空着不是"官方不给"，而是本仓没接线——
// 而接线最低一层就是这三条方法（宿主侧全都真的实现了，见下）。
//
// 归属：官方「接收拖入」在 `dsh-client-ui-attachment/src/client/drop-events.ts`，
// **不是** `dsh-client-ui-file-drag`（那个是"把文件从应用拖出去"，Viva 没有 OS 拖源）。
//
// # 每个方法的宿主证据（逐条 file:line，照实抄，不许凭印象）
//
//   - `attachment.put`
//     入参 `{"path": "非空绝对路径"}`（`HOST:internal/transport/server.go:844-862`
//     的 `struct{ Path string }`；path 为空回 400 `attachment.put 需要非空 path`）。
//     回 `{"attachment":{id,name,byteSize,mime,sha256,width?,height?}}`（同处
//     `:861` 的 `map[string]any{"attachment": …}`）。
//     类型：`HOST:internal/transport/api.go:338-346`（本文件的 `Attachment` 与它逐字同 tag）。
//     真实现：`HOST:cmd/dshd/attachments.go:124`（`func (s *attachmentStore) Put`）；
//     **0 字节与超限都在那里回 400**（`:129-131`「附件不能是 0 bytes」、
//     `:132-134`「附件 N bytes 超过 M bytes 上限」，上限 `8<<20` 在 `:30`）。
//     挂载：`HOST:cmd/dshd/serve.go:541`（`Attachments: attachments`）。
//   - `attachment.get`
//     入参 `{"id"}`（也收 `attachmentId`，见 `fillID`），回
//     `{"attachment": …, "dataBase64": "…"}` —— handler
//     `HOST:internal/transport/server.go:864`，base64 唯一出口在 `:890-893`
//     （宿主刻意不把字节放进 Attachment 结构，见 `api.go:347-354` 的注释）；
//     真实现 `HOST:cmd/dshd/attachments.go:493`。
//   - `session.attachment`
//     handler `HOST:internal/transport/handlers_session_official.go:1497`
//     （请求体解码在 `:1509-1526`，校验在 `:1520-1567`，结果
//     `OfficialAttachmentResult` 在 `:296-299`）。
//     **它只承载图片**：非 `image/png|jpeg|webp|gif` 的 MIME 一律 400
//     （`:1534-1540` 的 `officialImageMediaType` 门）。
//     语义（宿主注释 `:1492-1496`）：sessionId 只是网关权限范围，宿主**不检查**
//     会话是否存在、也不检查这条会话引用过它——所以别把 200 理解成"已挂到消息上"。
//
// # 为什么是平铺信封
//
// 本仓契约 `testdata/api.snapshot.json` 的 `envelope.request.style = "flat"`：
// 真宿主 handler 就是 `json.NewDecoder(r.Body).Decode(&in)` 直接解平铺结构。
// 外面再套 `{"params":…}` 会让每个参数解成零值（`client.go:272-287` 有完整事故记录）。
package apiclient

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
)

// ErrAttachmentPath 是 `attachment.put` 的**本地**入参校验失败。
//
// 只挡"空路径"这一件事：宿主那边的判据更严（绝对路径 / 常规文件 / 非符号链接 /
// 0 字节 / 上限，`HOST:cmd/dshd/attachments.go:97-122`、`:129-134`），**且它的中文
// 错误原文要原样显示给用户**——所以本文件的任何一层都不得把宿主错误改写成自己编的话。
var ErrAttachmentPath = errors.New("attachment.put 需要非空 path")

// Attachment 是一份真实本地文件的元数据。
//
// json tag 与 `HOST:internal/transport/api.go:338-346` **逐字一致**：
// `byteSize`（不是 byte_size）、`sha256`（小写）、`width`/`height` 省略零值
// （宿主对非图片不填这两个字段）。改 tag 等于改契约，别顺手美化。
type Attachment struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ByteSize int64  `json:"byteSize"`
	MIME     string `json:"mime"`
	SHA256   string `json:"sha256"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

// AttachmentPut 把宿主能读到的一个**绝对路径**登记成受控附件（`attachment.put`）。
//
// path 为空时**本地**就报错（`ErrAttachmentPath`）：发一个 `{"path":""}` 过去只会
// 换来一句宿主的 400，而且调用方会以为"是宿主挑食"。其余拒绝了什么（非绝对路径、
// 目录、符号链接、0 字节、超过 8 MiB）一律由宿主回原文，这里不改写。
func (c *Client) AttachmentPut(ctx context.Context, path string) (*Attachment, error) {
	if strings.TrimSpace(path) == "" {
		return nil, ErrAttachmentPath
	}
	var out struct {
		Attachment Attachment `json:"attachment"`
	}
	if err := c.Call(ctx, "attachment.put", map[string]any{"path": path}, &out); err != nil {
		return nil, err
	}
	return &out.Attachment, nil
}

// AttachmentGet 取回一份已登记附件的元数据与**原始字节**（`attachment.get`）。
//
// 宿主在唯一出口把字节编成 `dataBase64`（`HOST:internal/transport/server.go:890-893`），
// 所以这里必须解码；解码失败是**错误**而不是空字节——空字节会让调用方以为
// "这个附件本来就是 0 字节"，而 0 字节的附件宿主根本不会登记（`attachments.go:129`）。
func (c *Client) AttachmentGet(ctx context.Context, id string) (*Attachment, []byte, error) {
	if strings.TrimSpace(id) == "" {
		return nil, nil, errors.New("attachment.get 需要非空 id")
	}
	var out struct {
		Attachment Attachment `json:"attachment"`
		DataBase64 string     `json:"dataBase64"`
	}
	if err := c.Call(ctx, "attachment.get", map[string]any{"id": id}, &out); err != nil {
		return nil, nil, err
	}
	data, err := base64.StdEncoding.DecodeString(out.DataBase64)
	if err != nil {
		return nil, nil, err
	}
	return &out.Attachment, data, nil
}

// SessionAttach 把一份**已经上传**的附件按会话范围交给宿主（`session.attachment`）。
//
// 入参形状照 `HOST:internal/transport/handlers_session_official.go:1509-1526` 逐字：
//
//	{"sessionId": …, "attachment": {"attachmentId", "mediaType", "bytes",
//	                               "width", "height", "name", "originalDimensions"}}
//
// 三个取舍，都是宿主判据直接决定的：
//
//	① `bytes`/`width`/`height` 只在非零时发。宿主把非零值当**一致性断言**
//	   （`:1549-1563`：与存储真值不一致就 400 说"引用过期或被改过"），
//	   而 0 表示"这一项不校验"。发 0 是安全的，发一个猜出来的数是错的。
//	② `originalDimensions` **不发**：本仓没有"用户原图尺寸"这第二份真值
//	   （官方由浏览器 `File` 侧探测），宿主也没读它（只解码、不校验）。
//	③ 只有图片能过：非图片 MIME 宿主回 400（`:1534-1540`）。这不是本方法的限制，
//	   是 `session.attachment` 的官方语义——附件 rail 里的普通文件**不该**走到这里。
func (c *Client) SessionAttach(ctx context.Context, sessionID string, a Attachment) error {
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("session.attachment 需要非空 sessionId")
	}
	if strings.TrimSpace(a.ID) == "" {
		return errors.New("session.attachment 需要非空 attachmentId")
	}
	reference := map[string]any{"attachmentId": a.ID}
	if a.MIME != "" {
		reference["mediaType"] = a.MIME
	}
	if a.ByteSize != 0 {
		reference["bytes"] = a.ByteSize
	}
	if a.Width != 0 {
		reference["width"] = a.Width
	}
	if a.Height != 0 {
		reference["height"] = a.Height
	}
	if a.Name != "" {
		reference["name"] = a.Name
	}
	params := map[string]any{"sessionId": sessionID, "attachment": reference}
	// 结果体本客户端不用（`OfficialAttachmentResult`）：宿主回的是同一份真值 +
	// base64 字节，而字节已经在 `AttachmentGet` 那条路上取过。用 map 接住，
	// 宿主将来加字段不至于解崩。
	var out map[string]any
	return c.Call(ctx, "session.attachment", params, &out)
}
