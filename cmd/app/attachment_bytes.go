// attachment_bytes.go —— 消息区图片面的取数入口（`#18` ②③ 的壳侧一半）。
//
// 官方图片面是浏览器 `<img>`：URL 由会话授权的 loader 解析后直接交给渲染器
// （`OFF:dsh-client-ui-attachment/src/MessageImage.tsx:11-13` 的 `ImageLoader`、
// `:103-110` 的 `load(attachment).then(setLoaded).catch(setError)`）。浏览器自己做
// "取字节 → 解码 → 显示"三步，插件只拿一个 URL。
//
// 本仓没有浏览器 URL 层，等价物是**把字节取回来交给插件自己解码**：
//
//	插件（消息区）        壳（这里）                       宿主
//	Actions.AttachmentGet → attachmentGetAction → apiclient.AttachmentGet → attachment.get
//	                      ← AttachmentBlob{…, Bytes}
//	imgdecode.Decode
//	ui.Image(c, gfx.Bitmap)
//
// # 为什么是同步的、为什么必须在 goroutine 里调
//
// 宿主 `attachment.get` 是把整份文件读出来再 base64（`HOST:internal/transport/
// server.go:890-893` 的唯一出口），8 MiB 上限的附件编码后能到 11 MiB。
// 这种调用不能进绘制帧——`writeDraft` 那条规矩同理（Viva 的绘制帧里没有事件循环）。
// 所以接口是**同步返回**、由插件自己起 goroutine，壳只负责带上限超时。
package main

import (
	"context"
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// attachmentGetTimeout 是一次 `attachment.get` 的上限。
//
// 与 `voiceCatalogTimeout`(8s) 同一量级：这条要走"读盘 + MIME 探测 + 解码尺寸 +
// base64"四步（`HOST:cmd/dshd/attachments.go` 的 Get），8 MiB 图的实测在百毫秒级，
// 5 秒是"慢盘也不会误杀"的余量。超时按失败返回 → 界面画官方失败态（可重试），
// 不静默当成功。
const attachmentGetTimeout = 5 * time.Second

// attachmentGetAction 是 `Actions.AttachmentGet` 的实现。
//
// 返回 false 的五条：没接线、id 空、传输失败、宿主回了 0 字节、宿主回了空 id。
// **0 字节必须算失败**：宿主根本不会登记 0 字节附件（`HOST:cmd/dshd/attachments.go:129-131`
// 的「附件不能是 0 bytes」），所以拿到 0 字节只可能是链上出了问题——把它当成功
// 会让界面画一张"什么都没有"的图，比报错更坏。
func (m *model) attachmentGetAction(id string) (pluginapi.AttachmentBlob, bool) {
	if strings.TrimSpace(id) == "" {
		return pluginapi.AttachmentBlob{}, false
	}
	m.mu.Lock()
	client := m.cli
	m.mu.Unlock()
	if client == nil {
		return pluginapi.AttachmentBlob{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), attachmentGetTimeout)
	defer cancel()
	attachment, data, err := client.AttachmentGet(ctx, id)
	if err != nil || attachment == nil || len(data) == 0 {
		return pluginapi.AttachmentBlob{}, false
	}
	return pluginapi.AttachmentBlob{
		ID:       attachment.ID,
		Name:     attachment.Name,
		MIME:     attachment.MIME,
		ByteSize: attachment.ByteSize,
		Width:    attachment.Width,
		Height:   attachment.Height,
		Bytes:    data,
	}, true
}
