// chatimport.go —— 官方 rc.5 `dsh-deepseek-chat-import` 的两支客户端调用。
//
// # 为什么不是普通 Call
//
// 这条能力有两个 Method 面：
//
//	POST /api/deepseekChatImport/limits        普通 unary（JSON 信封）
//	POST /api/deepseek-chat-import/archive     裸字节上传 + NDJSON 进度流
//
// 第二条不是 `POST /api/<method>`（请求体是裸 ZIP、响应是逐行流），所以不能走
// `Client.Call`。契约原件（rc.5 解包树）：
//
//	@deepseek-ai/dsh-deepseek-chat-import/src/protocol.ts:4   UPLOAD_PATH
//	src/upload.ts:53-58                                       响应头
//	src/types.ts:34-73                                        结果/进度行/outcome 行
//	src/client/upload.ts:148,217-218                          非 200 只当文本报错
//
// 宿主侧实现见 DSH-GO `internal/transport/handlers_chat_import.go` 与
// `cmd/dshd/serve_chat_import_api.go`。
package apiclient

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// ChatImportUploadPath 是上传路由（官方 protocol.ts:4，逐字）。
const ChatImportUploadPath = "/api/deepseek-chat-import/archive"

// ChatImportLimits 是 `deepseekChatImport/limits` 的结果（官方 types.ts:50-53）。
type ChatImportLimits struct {
	MaxUploadBytes int64 `json:"maxUploadBytes"`
}

// ChatImportSkip 是一个没能导入的对话（官方 types.ts:24-31）。
type ChatImportSkip struct {
	ConversationID string  `json:"conversationId"`
	Title          *string `json:"title"`
	Reason         string  `json:"reason"`
}

// ChatImportResult 是一次完成的导入（官方 types.ts:34-47）。
type ChatImportResult struct {
	WorkspaceID       string           `json:"workspaceId"`
	WorkspacePath     string           `json:"workspacePath"`
	SessionIDs        []string         `json:"sessionIds"`
	ConversationCount int              `json:"conversationCount"`
	OmittedByLimit    int              `json:"omittedByLimit"`
	Skipped           []ChatImportSkip `json:"skipped"`
}

// ChatImportProgress 是一次导入的进度快照。
//
// Phase 取 `uploading`（本机往宿主发字节）、`reading`（宿主在读归档）、
// `importing`（宿主在写会话）。官方客户端同样的三段（src/client/upload.ts）。
type ChatImportProgress struct {
	Phase string
	// Fraction 只在 `uploading` 阶段有意义（0..1）。
	Fraction float64
	// Done/Total 只在 `importing` 阶段有意义（已写会话 / 计划写的会话）。
	Done  int
	Total int
}

// ChatImportLimits 问宿主上传上限（官方 limits，src/index.ts:111-118）。
func (c *Client) ChatImportLimits(ctx context.Context) (*ChatImportLimits, error) {
	var out ChatImportLimits
	if err := c.Call(ctx, "deepseekChatImport/limits", map[string]any{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ChatImportArchive 把一个本机导出 ZIP 发给宿主导入到 workspaceID。
//
// archivePath 是**本机绝对路径**：本仓没有浏览器 File 对象，导出就是磁盘上的
// 一个文件，直接读它比先拷进内存再传更省一次拷贝。
//
// onProgress 可为 nil。返回的结果是宿主 outcome 行里的 `value`；
// 失败按官方原因码翻成 error（reason 保留在 error 文本里，便于界面分类）。
func (c *Client) ChatImportArchive(
	ctx context.Context, workspaceID, archivePath string, onProgress func(ChatImportProgress),
) (*ChatImportResult, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, errors.New("导入需要非空 workspaceId")
	}
	if strings.TrimSpace(archivePath) == "" {
		return nil, errors.New("导入需要导出的 ZIP 路径")
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		return nil, fmt.Errorf("读不到导出文件 %s：%w", archivePath, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s 不是一个普通文件", archivePath)
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("打不开导出文件 %s：%w", archivePath, err)
	}
	defer file.Close()

	progress := onProgress
	if progress == nil {
		progress = func(ChatImportProgress) {}
	}
	progress(ChatImportProgress{Phase: "uploading", Fraction: 0})

	var body io.Reader = file
	if info.Size() > 0 {
		// 上传阶段的真实字节进度：宿主那边的 progress 行是"读归档/写会话"，
		// 不是"字节发出去多少"，两者不能混成一个数。
		body = &progressReader{inner: file, total: info.Size(), report: func(sent, total int64) {
			progress(ChatImportProgress{Phase: "uploading", Fraction: float64(sent) / float64(total)})
		}}
	}
	url := c.Base + ChatImportUploadPath + "?workspaceId=" + urlQueryEscape(workspaceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.ContentLength = info.Size()
	req.Header.Set("Content-Type", "application/octet-stream")
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}
	// 上传可能很久（大包 + 导入本身），不能套用普通调用的 8s 超时。
	client := *c.HTTP
	client.Timeout = 0
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("上传失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if contentType := resp.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/x-ndjson") {
		return nil, fmt.Errorf("宿主回了 %q，导入应答必须是 application/x-ndjson", contentType)
	}
	return readChatImportAnswer(resp.Body, progress)
}

// readChatImportAnswer 逐行读 NDJSON：进度行若干 + 恰好一行 outcome。
func readChatImportAnswer(body io.Reader, progress func(ChatImportProgress)) (*ChatImportResult, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 4096), 1<<20)
	sawOutcome := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var envelope struct {
			Type    string `json:"type"`
			Done    int    `json:"done"`
			Total   int    `json:"total"`
			Outcome *struct {
				OK    bool              `json:"ok"`
				Value *ChatImportResult `json:"value"`
				Error *struct {
					Reason  string `json:"reason"`
					Message string `json:"message"`
				} `json:"error"`
			} `json:"outcome"`
		}
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			return nil, fmt.Errorf("宿主回的进度行不是 JSON：%.120s", line)
		}
		switch envelope.Type {
		case "progress":
			progress(ChatImportProgress{Phase: "importing", Done: envelope.Done, Total: envelope.Total})
		case "outcome":
			if sawOutcome {
				return nil, errors.New("宿主回了不止一行 outcome")
			}
			sawOutcome = true
			if envelope.Outcome == nil {
				return nil, errors.New("outcome 行没有 outcome 载荷")
			}
			if !envelope.Outcome.OK {
				if envelope.Outcome.Error == nil {
					return nil, errors.New("导入失败但没给原因")
				}
				return nil, &chatImportFailure{
					reason:  envelope.Outcome.Error.Reason,
					message: envelope.Outcome.Error.Message,
				}
			}
			if envelope.Outcome.Value == nil {
				return nil, errors.New("导入成功但没给结果")
			}
			return envelope.Outcome.Value, nil
		default:
			return nil, fmt.Errorf("未知的导入行类型 %q", envelope.Type)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读导入应答失败：%w", err)
	}
	if !sawOutcome {
		// 官方在客户端断开/取消时同样不写 outcome 行；这里如实报"没有结论"，
		// 不把一次没有结论的上传当成成功。
		return nil, errors.New("上传结束了但没有 outcome 行（导入未完成）")
	}
	return nil, errors.New("导入应答不完整")
}

// chatImportFailure 是带官方原因码的导入失败。
type chatImportFailure struct {
	reason  string
	message string
}

func (e *chatImportFailure) Error() string {
	if e.reason == "" {
		return e.message
	}
	return e.reason + ": " + e.message
}

// Reason 返回官方的失败原因码（too-large / unreadable-archive / …）。
func (e *chatImportFailure) Reason() string { return e.reason }

// ChatImportFailureReason 取 err 里的官方原因码；不是导入失败时返回空串。
func ChatImportFailureReason(err error) string {
	var failure *chatImportFailure
	if errors.As(err, &failure) {
		return failure.reason
	}
	return ""
}

// progressReader 把"已发出去多少字节"报给调用方。
type progressReader struct {
	inner  io.Reader
	total  int64
	sent   int64
	report func(sent, total int64)
}

func (r *progressReader) Read(buffer []byte) (int, error) {
	read, err := r.inner.Read(buffer)
	if read > 0 {
		r.sent += int64(read)
		if r.report != nil {
			r.report(r.sent, r.total)
		}
	}
	return read, err
}

// urlQueryEscape 是只有 workspaceId 一个参数时的最小编码
// （workspace id 是 uuid 形状，不碰 net/url 的整条 URL 拼装）。
func urlQueryEscape(value string) string {
	var builder strings.Builder
	for _, char := range []byte(value) {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9', char == '-', char == '_', char == '.', char == '~':
			builder.WriteByte(char)
		default:
			builder.WriteString(fmt.Sprintf("%%%02X", char))
		}
	}
	return builder.String()
}
