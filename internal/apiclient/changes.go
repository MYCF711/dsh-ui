// changes.go —— 官方 deliverables 那三条**变更路由**的客户端（#14 的 per-file diff 半条）。
//
// # 为什么不能走 `Call`
//
// `Call` 认的是 `POST /api/<method>` + `{result:…}` 信封（client.go:272-345）。
// 这三条是**文档相对路由**，路径本身就是端点，不是方法名
// （OFF:dsh-client-ui-deliverables/src/changes.ts:6-12；宿主逐字复核在
// HOST:internal/transport/handlers_changes_routes.go:28-36）；消费端
// `response.json()` 之后**直接读顶层字段**（OFF:src/changes.ts:65-94 的
// `isChangesSummary` / `isChangesDiff`），没有 result 信封。用 `Call` 会两处错：
// 请求形状被当成方法调用，而回包在 `env.Result` 里找不到东西 → 一个字段都解不出来。
//
// # 契约（逐条对宿主 handlers_changes_routes.go）
//
//	GET  /api/changes.summary?sessionId=&seq=
//	GET  /api/changes.diff?sessionId=&seq=&index=
//	POST /api/changes.open?sessionId=&seq=&index=[&action=reveal][&application=]
//
// 认证与 `POST /api/<method>` 完全相同：带 `/connect` 换来的 cookie
// （client.go:177-180 的 `c.cookies`），这里复用同一份，不另建认证。
//
// 状态码语义（HOST:handlers_changes_routes.go:24-32）：
//
//	400 参数非法（sessionId 空 / seq、index 不是非负整数）
//	404 该 seq（或 index）**没有记录**
//	422 本机没有原生打开通道（官方客户端映射成 `nativeUnavailable`，OFF:present-open.ts:138）
//	501 宿主未接线这三条路由
//	500 读记录 / 算 diff 失败
//
// # 404 是独立分类，不许翻成空摘要
//
// 已登记的裁剪：**非 git 工作目录的会话没有 changes 记录**，宿主回 404 而
// **不**伪造空摘要（HOST:handlers_changes_routes.go:96-99）。所以 404 必须映射成
// `ChangesStatusMissing` 这一个独立分类——调用方据此显示"这一轮没有变更记录"，
// 绝不能画一个 `{files: []}` 的空列表（那等于替宿主断言"没有变更"）。
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// 三条路由的**逐字路径**（OFF:src/changes.ts:6-12；宿主同名常量在
// HOST:internal/transport/handlers_changes_routes.go:39-48）。
const (
	ChangesSummaryRoute = "/api/changes.summary"
	ChangesDiffRoute    = "/api/changes.diff"
	ChangesOpenRoute    = "/api/changes.open"
)

// ChangesStatus 是这三条路由的**状态码分类**。
//
// 为什么不只用 HTTP 码：消费端要按语义分支（404 = 没记录要退全文预览、
// 422 = 没有原生通道、501 = 宿主没接线），拿数字散在各处判断迟早会漏一个。
type ChangesStatus int

const (
	// ChangesStatusOK 是 2xx。
	ChangesStatusOK ChangesStatus = iota
	// ChangesStatusBadRequest 是 400：参数非法（本客户端的 bug，不是用户操作）。
	ChangesStatusBadRequest
	// ChangesStatusMissing 是 404：**这个 seq/index 没有记录**。
	ChangesStatusMissing
	// ChangesStatusNativeUnavailable 是 422：本机没有原生打开通道。
	ChangesStatusNativeUnavailable
	// ChangesStatusNotWired 是 501：宿主没接线这三条路由（旧宿主）。
	ChangesStatusNotWired
	// ChangesStatusFailed 是其余（500 与任何别的失败）。
	ChangesStatusFailed
)

// classifyChangesStatus 把 HTTP 状态码翻成分类。
//
// 只认宿主真的会回的五个码（HOST:handlers_changes_routes.go:24-32 的注释块
// 就是这张表的出处）；其余一律 Failed——**不猜**，不把未知码当成功。
func classifyChangesStatus(status int) ChangesStatus {
	switch status {
	case http.StatusBadRequest:
		return ChangesStatusBadRequest
	case http.StatusNotFound:
		return ChangesStatusMissing
	case http.StatusUnprocessableEntity:
		return ChangesStatusNativeUnavailable
	case http.StatusNotImplemented:
		return ChangesStatusNotWired
	default:
		return ChangesStatusFailed
	}
}

// ChangesError 是这三条路由的错误，带**裸四字段**解出来的内容。
//
// 它实现 error，所以调用方既能 `errors.As` 拿分类，也能直接当普通错误传。
type ChangesError struct {
	// Status 是原始 HTTP 状态码（诊断用；分支判断请用 Kind）。
	Status int
	Kind   ChangesStatus
	// 四字段（HOST:internal/transport/server.go:1660-1667 的 errorObject）。
	Code      string
	Message   string
	Hint      string
	Retryable bool
}

func (e *ChangesError) Error() string {
	switch {
	case e.Hint != "":
		return fmt.Sprintf("changes HTTP %d %s: %s（%s）", e.Status, e.Code, e.Message, e.Hint)
	case e.Message != "":
		return fmt.Sprintf("changes HTTP %d %s: %s", e.Status, e.Code, e.Message)
	default:
		return fmt.Sprintf("changes HTTP %d（无错误体）", e.Status)
	}
}

// decodeChangesError 解**裸**四字段错误体。
//
// 宿主把四字段嵌在顶层 `error` 里（server.go:1523 的 errorBodyFor 造出
// `{"error":{code,message,hint,retryable}}`）。这里另外容忍两种老/未来形状：
// `{"error":"unauthorized"}`（client.go:249 记过的那种）与顶层直接摊平的四字段。
// 三种都**不套 result 信封**——所以绝不能拿 parseError 那套信封解析器来套。
func decodeChangesError(status int, raw []byte) *ChangesError {
	e := &ChangesError{Status: status, Kind: classifyChangesStatus(status)}
	var envelope struct {
		Error json.RawMessage `json:"error"`
		Code  string          `json:"code"`
		// message/hint/retryable 只在真的摊平那种形状里出现
		Message   string `json:"message"`
		Hint      string `json:"hint"`
		Retryable bool   `json:"retryable"`
	}
	if json.Unmarshal(raw, &envelope) == nil {
		if len(envelope.Error) > 0 {
			var four struct {
				Code      string `json:"code"`
				Message   string `json:"message"`
				Hint      string `json:"hint"`
				Retryable bool   `json:"retryable"`
			}
			if json.Unmarshal(envelope.Error, &four) == nil && (four.Code != "" || four.Message != "") {
				e.Code, e.Message, e.Hint, e.Retryable = four.Code, four.Message, four.Hint, four.Retryable
				return e
			}
			var s string
			if json.Unmarshal(envelope.Error, &s) == nil && s != "" {
				e.Code, e.Message = s, s
				return e
			}
		}
		if envelope.Code != "" || envelope.Message != "" {
			e.Code, e.Message, e.Hint, e.Retryable = envelope.Code, envelope.Message, envelope.Hint, envelope.Retryable
			return e
		}
	}
	// 一个字段都没解出来（HTML 错误页、被截断的正文…）：把原文裁短带上，
	// 让日志里能看到"到底回了什么"，而不是一句没有线索的 HTTP 500。
	e.Code = "changes/unparsable"
	e.Message = truncateChangesBody(raw)
	return e
}

// truncateChangesBody 把解不动的正文裁成一行，避免把整页 HTML 灌进日志。
func truncateChangesBody(raw []byte) string {
	const limit = 200
	s := strings.TrimSpace(string(raw))
	if len(s) > limit {
		s = s[:limit] + "…"
	}
	if s == "" {
		return "（空正文）"
	}
	return s
}

// IsChangesMissing 报告错误是不是"宿主明确说这条记录不存在"（404）。
//
// 这是**唯一**允许被显示成"这一轮没有变更记录"的分类；传输失败、500、501
// 都不算——那些是"读不到"，不是"没有"。
func IsChangesMissing(err error) bool {
	var ce *ChangesError
	return errors.As(err, &ce) && ce.Kind == ChangesStatusMissing
}

// IsChangesNativeUnavailable 报告错误是不是"本机没有原生打开通道"（422）。
func IsChangesNativeUnavailable(err error) bool {
	var ce *ChangesError
	return errors.As(err, &ce) && ce.Kind == ChangesStatusNativeUnavailable
}

// ChangesFile 是摘要里的一行（宿主 ChangesSummaryFile 的逐字投影，
// HOST:internal/transport/handlers_changes_routes.go:66-74；官方
// WorkspaceChangedFile 的线上子集，OFF:src/changes.ts:52-58 的校验就是这几个字段）。
type ChangesFile struct {
	// Path 是相对会话工作目录的路径（不是绝对路径）。
	Path string `json:"path"`
	// Display 是给人看的路径（官方由宿主拼好，界面不重算）。
	Display string `json:"display"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
	// Binary/Oversized 为真时 added/deleted 没有意义，界面显示官方那两句状态词。
	Binary    bool `json:"binary,omitempty"`
	Oversized bool `json:"oversized,omitempty"`
}

// ChangesSnapshotIDs 是这一轮前后两张快照树的 id（官方 WorkspaceChangesSummary.snapshot）。
//
// 界面不用它——列出来是因为宿主会发，客户端解析器漏掉字段会让"以后要用"时
// 误以为宿主没给。
type ChangesSnapshotIDs struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

// ChangesSummary 是一条 `workspace/changes` 事件的摘要。
type ChangesSummary struct {
	Turn    int           `json:"turn"`
	Files   []ChangesFile `json:"files"`
	Total   int           `json:"total"`
	Added   int           `json:"added"`
	Deleted int           `json:"deleted"`
	// Cwd 是 files[].path 相对的那个会话工作目录（可选，老宿主不给）。
	Cwd string `json:"cwd,omitempty"`
	// Snapshot 只在真的拍了快照时出现（没有 git 的会话本来就没有记录）。
	Snapshot *ChangesSnapshotIDs `json:"snapshot,omitempty"`
}

// ChangesHunk 是统一 diff 的一段（宿主 ChangesHunk 的逐字投影）。
//
// Lines 里每行**带一个前缀字符**：' ' 上下文、'+' 新增、'-' 删除
// （宿主只挑这三种，HOST:cmd/dshd/workspace_changes_diff.go:206-207）；
// 行号要从 oldStart/newStart 现推，见 deliverables 侧的 hunkRows。
type ChangesHunk struct {
	OldStart int      `json:"oldStart"`
	OldLines int      `json:"oldLines"`
	NewStart int      `json:"newStart"`
	NewLines int      `json:"newLines"`
	Lines    []string `json:"lines"`
}

// ChangesDiff 是官方 `WorkspaceFileDiff` 那个**三支 union** 的 Go 投影。
//
// # 为什么用一个结构体而不是 `any`
//
// 官方 union 的三支靠 `kind` 判别（OFF:src/changes.ts:85-94）：`text` 带
// before/after/hunks/coarse，`binary`/`oversized` **只有 path/display**
// （服务端不读正文，HOST:cmd/dshd/workspace_changes_diff.go:41-46）。
// 用 Kind 判别一支结构体，比 `any` + 类型断言少一次"断言错了静默走零值"的机会。
type ChangesDiff struct {
	// Kind 取 `text` / `binary` / `oversized`；别的值解码时报错（见 ChangesDiff 方法）。
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Display string `json:"display"`
	// Before/After 说的是"这一侧在快照里存不存在"（新文件 before=false，
	// 删除的文件 after=false）——不是"有没有变化"。
	Before bool          `json:"before"`
	After  bool          `json:"after"`
	Hunks  []ChangesHunk `json:"hunks"`
	// Coarse 恒为 false：本仓用 git 的统一 diff，没有官方 JS 行比较的
	// "超时退化成整文件替换"那种事件（HOST:cmd/dshd/workspace_changes_diff.go:10-16）。
	Coarse bool `json:"coarse"`
}

// changesDo 发一条**裸路由**请求并回原始正文（2xx）或 *ChangesError。
//
// 与 `Call` 的两处刻意不同：
//   - 不在 `c.Base` 后面加 `/api/`+方法名，路径由调用方给全（这三条就是路径本身）；
//   - 不解析 `{result:…}` 信封，正文原样交回调用方自己解。
func (c *Client) changesDo(ctx context.Context, method, route string, q url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.Base+route+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	for _, ck := range c.cookies {
		req.AddCookie(ck)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连不上 %s：%w", c.Base, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, decodeChangesError(resp.StatusCode, raw)
	}
	return raw, nil
}

// ChangesSummary 读一条 `workspace/changes` 事件的摘要（宿主 `GET /api/changes.summary`）。
//
// seq 必须是那个事件的 seq，**不能猜**：调用方从 Transport 帧里取
// （`workspace/changes` 帧的 seq）。
func (c *Client) ChangesSummary(ctx context.Context, sessionID string, seq int) (*ChangesSummary, error) {
	q := url.Values{"sessionId": {sessionID}, "seq": {strconv.Itoa(seq)}}
	raw, err := c.changesDo(ctx, http.MethodGet, ChangesSummaryRoute, q)
	if err != nil {
		return nil, err
	}
	var out ChangesSummary
	if err := json.Unmarshal(raw, &out); err != nil {
		// 成功响应是**裸对象**，不是信封：解不动就说明对面不是这三条路由
		// （拿到 `{"result":…}` 时这里会成功解出一个全零结构体——所以下面还要断言 turn）。
		return nil, fmt.Errorf("changes.summary 响应不是裸 JSON 对象: %w", err)
	}
	if out.Turn < 1 {
		// 官方 `isChangesSummary` 就是拿 turn >= 1 当"这确实是一份摘要"的判据
		// （OFF:src/changes.ts:68）。全零结构体在这里被挡掉，不会冒充一份空摘要。
		return nil, fmt.Errorf("changes.summary 响应缺少 turn（拿到的是信封或别的端点？）")
	}
	return &out, nil
}

// ChangesDiff 读摘要里第 index 个文件的 before/after 比较
// （宿主 `GET /api/changes.diff`）。
//
// index 是**摘要 files 数组里的下标**，不是路径——所以调用方必须先拿 summary
// 定位，不能自己按路径拼一个。
func (c *Client) ChangesDiff(ctx context.Context, sessionID string, seq, index int) (*ChangesDiff, error) {
	q := url.Values{
		"sessionId": {sessionID},
		"seq":       {strconv.Itoa(seq)},
		"index":     {strconv.Itoa(index)},
	}
	raw, err := c.changesDo(ctx, http.MethodGet, ChangesDiffRoute, q)
	if err != nil {
		return nil, err
	}
	var out ChangesDiff
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("changes.diff 响应不是裸 JSON 对象: %w", err)
	}
	switch out.Kind {
	case "binary", "oversized":
		// 拒绝支只有 path/display——**不补零值 hunks**，界面照样不画假 hunk。
		out.Before, out.After, out.Hunks, out.Coarse = false, false, nil, false
	case "text":
		// 官方 `isHunk` 要求每行以 '+','-',' ' 开头（OFF:src/changes.ts:77）。
		// 宿主只产出这三种前缀（HOST:cmd/dshd/workspace_changes_diff.go:206-207），
		// 所以出现第四种就说明这不是我们能画的东西——报错好过画出看不懂的行。
		for _, h := range out.Hunks {
			for _, line := range h.Lines {
				if line == "" || (line[0] != '+' && line[0] != '-' && line[0] != ' ') {
					return nil, fmt.Errorf("changes.diff 的 hunk 行没有 +/-/空格 前缀: %q", line)
				}
			}
		}
	default:
		return nil, fmt.Errorf("changes.diff 的 kind 未知: %q", out.Kind)
	}
	if out.Path == "" {
		return nil, fmt.Errorf("changes.diff 响应缺少 path")
	}
	return &out, nil
}

// ChangesOpen 让**宿主桌面**打开摘要里第 index 个文件的当前路径
// （宿主 `POST /api/changes.open`）。
//
// action 为空 = 用默认程序打开；`reveal` = 在文件管理器里定位；
// application 非空 = 用指定程序打开。三者的拼参方式与官方客户端逐字一致
// （OFF:src/client/present-open.ts:135-137：只有 reveal 才带 action，
// 只有给了 application 才带 application）。
//
// 422（本机没有原生打开通道）会以 *ChangesError 回，调用方用
// IsChangesNativeUnavailable 判——**不是**失败重试，是"这台机器没这能力"。
func (c *Client) ChangesOpen(ctx context.Context, sessionID string, seq, index int, action, application string) error {
	q := url.Values{
		"sessionId": {sessionID},
		"seq":       {strconv.Itoa(seq)},
		"index":     {strconv.Itoa(index)},
	}
	if action == "reveal" {
		q.Set("action", "reveal")
	}
	if application != "" {
		q.Set("application", application)
	}
	if _, err := c.changesDo(ctx, http.MethodPost, ChangesOpenRoute, q); err != nil {
		return err
	}
	// 成功体是 `{"opened":true}` 的裸对象。这里**不解它**：官方消费端也只判
	// `response.ok`（OFF:src/client/present-open.ts:138），去解 `opened` 只会
	// 多一个"字段名变了就报错"的失败点。
	return nil
}
