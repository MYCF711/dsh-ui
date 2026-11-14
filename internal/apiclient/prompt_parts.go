// prompt_parts.go —— 官方 rc.2 `PromptContentPart` 的客户端两支：文件回执与内联图片。
//
// # 为什么必须走这条
//
// 宿主已经合并了官方形状（`HOST:cmd/dshd/attachments_official_prompt.go`）：
// `session.prompt` 的 `content` 除了宿主的 `{type:"attachment", attachmentId}`，
// 还认官方 `PromptContentPart`：
//
//	{type:'file',  receiptId}                       ← 先 fileUploads.upload 拿回执
//	{type:'image', mediaType, data, name?}          ← 内联 base64（官方只在无上传通道时走）
//
// 官方客户端（`OFF:dsh-client-ui-conversation/src/client/service.ts:298-303`）的
// 内容和顺序是**附件在前、文本在最后**：`content = [...uploaded, …text]`。
// 顺序是语义（图在文字前），这里逐字照抄，不重排。
//
// # 为什么单独 fileUploads.upload
//
// 宿主的 `attachment.put` 吃**本机绝对路径**（桌面拖入可用），但官方客户端只有
// 浏览器 File 字节，没有路径。官方通道是把字节 base64 后 POST 给
// `fileUploads.upload`，拿回 `{receiptId, file:{attachmentId,name,bytes}}`；
// `session.prompt` 再用回执引用它。回执是**会话作用域**的（宿主会校验），
// 所以 sessionId 必须与随后 prompt 的会话一致——这里没有默认值，空就本地报错。
package apiclient

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
)

// FileUploadReceipt 是 `fileUploads.upload` 的结果。
//
// json tag 与 `HOST:internal/transport/handlers_agent_official.go:807-818` 的
// `OfficialFileUploadValue` / `OfficialUploadedFile` 逐字一致。
type FileUploadReceipt struct {
	ReceiptID string       `json:"receiptId"`
	File      UploadedFile `json:"file"`
}

// UploadedFile 是内容寻址存储里的一份文件引用（展示名 + 字节数，不是附件元数据）。
type UploadedFile struct {
	AttachmentID string `json:"attachmentId"`
	Name         string `json:"name"`
	Bytes        int64  `json:"bytes"`
}

// FileUploadsUpload 把一份完整文件字节暂存到宿主，返回会话作用域回执。
//
// 入参形状按本仓契约是**平铺**（`envelope.request.style = "flat"`）：
// `{sessionId, data:<base64>, name}`。宿主 `officialFileUploadsUpload` 既接受
// `request` 嵌套也接受平铺，平铺与本仓其它方法一致，不混两种形状。
//
// 上限：宿主 `agentUploadMaxBytes`（8 MiB 解码后）。这里不做第二份上限——
// 多一个数字就多一处会漂的判据，宿主原文才是用户能照着动作的那一句。
func (c *Client) FileUploadsUpload(ctx context.Context, sessionID string, data []byte, name string) (*FileUploadReceipt, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, errors.New("fileUploads.upload 需要非空 sessionId")
	}
	if len(data) == 0 {
		return nil, errors.New("fileUploads.upload 需要非空字节")
	}
	var out FileUploadReceipt
	params := map[string]any{
		"sessionId": sessionID,
		"data":      base64.StdEncoding.EncodeToString(data),
	}
	if strings.TrimSpace(name) != "" {
		params["name"] = name
	}
	if err := c.Call(ctx, "fileUploads.upload", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PromptPart 是官方 `PromptContentPart` 的本仓 wire 形状。
//
// 只保留宿主 `transport.ContentBlock` 认的字段（`HOST:internal/transport/api.go`
// 的 `ContentBlock` 注释点名了这三个 online-only 字段：receiptId / mediaType / data）。
// 字段用 omitempty：官方每个 part 的 union 分支只带自己那几项，多发一个空键
// 就是把"没有这个字段"写成"有且为空"。
type PromptPart struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ReceiptID string `json:"receiptId,omitempty"`
	MediaType string `json:"mediaType,omitempty"`
	Data      string `json:"data,omitempty"`
	Name      string `json:"name,omitempty"`
}

// PromptParts 发一条带附件块的用户消息（官方 `session.prompt` 的 content 形状）。
//
// parts 是**附件在前**的顺序（调用方负责保持配方顺序）；text 非空时追加在最后，
// 与官方 `service.ts:302` 一致。text 为空且 parts 为空时退回 Prompt 的旧路径，
// 避免把"空消息"当附件消息发出去（宿主对空 content 的语义是续跑，不是发送）。
func (c *Client) PromptParts(ctx context.Context, id, text string, parts []PromptPart) (string, error) {
	if id == "" {
		return "", errors.New("session.prompt 需要会话 id")
	}
	if len(parts) == 0 {
		return c.Prompt(ctx, id, text)
	}
	content := make([]PromptPart, 0, len(parts)+1)
	content = append(content, parts...)
	if text != "" {
		content = append(content, PromptPart{Type: "text", Text: text})
	}
	var r struct {
		TurnID   string `json:"turnId"`
		Accepted bool   `json:"accepted"`
	}
	if err := c.Call(ctx, "session.prompt", map[string]any{
		"id": id, "sessionId": id, "content": content,
	}, &r); err != nil {
		return "", err
	}
	return r.TurnID, nil
}
