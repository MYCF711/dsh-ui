// Package apiclient 是 Transport 契约的客户端实现（对照 testdata/api.snapshot.json）。
//
// 它只依赖 net/http：不做重试、不做连接池调优，先把 13 个 method 里第一批 8 个跑通。
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Session 对应 session.list 的一项。
type Session struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Cwd       string `json:"cwd"`
	UpdatedAt string `json:"updatedAt"`
	Mode      string `json:"mode"`
}

// Message 对应 session.load 的 messages 一项（v1 先只有纯文本）。
type Message struct {
	Seq  int    `json:"seq"`
	Role string `json:"role"` // user / assistant / tool
	Text string `json:"text"`
	// role == "tool" 时这三个字段描述一次工具调用（契约 v1 先只给摘要）
	Tool    string `json:"tool,omitempty"`
	Args    string `json:"args,omitempty"`
	Summary string `json:"summary,omitempty"`
	// View 是工具对这块结果的显示声明（形状逐字对齐宿主 spi.ViewSpec）。
	// 界面按 Kind 选渲染器；不认识就退化成 Text——不认识 == 没声明。
	View *ViewSpec `json:"view,omitempty"`
	// Usage 只在这一条是 assistant/message 时有值。
	Usage *Usage `json:"usage,omitempty"`
}

// ViewKind 是 ViewSpec 的渲染种类。
type ViewKind string

const (
	ViewCard  ViewKind = "card"  // 一组键值
	ViewTable ViewKind = "table" // 列 + 行
	ViewPlain ViewKind = "plain" // 一段文本
)

// ViewSpec 是工具结果的显示契约（对齐 DSH-GO internal/spi/tool.go:103）。
type ViewSpec struct {
	Kind    string      `json:"kind"`
	Title   string      `json:"title,omitempty"`
	Badge   string      `json:"badge,omitempty"`
	Fields  []ViewField `json:"fields,omitempty"`
	Columns []string    `json:"columns,omitempty"`
	Rows    [][]string  `json:"rows,omitempty"`
	Body    string      `json:"body,omitempty"`
}

// ViewField 是 card 的一行键值。
type ViewField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Usage 是一次或一段会话的 token 用量（宿主 spi 里对应 usage 事件）。
//
// TokPerSec 是我们自己算的：一段 assistant/message 的 outputTokens 除以
// 「上一条 user/message 到这一条的墙钟时间」。宿主不提供速率，只有时间戳。
type Usage struct {
	InputTokens     int64   `json:"inputTokens"`
	OutputTokens    int64   `json:"outputTokens"`
	TotalTokens     int64   `json:"totalTokens"`
	CacheReadTokens int64   `json:"cacheReadTokens"`
	TokPerSec       float64 `json:"tokPerSec,omitempty"` // 0 = 算不出来（缺时间戳）
	HasRate         bool    `json:"hasRate,omitempty"`
}

// LoadResult 是 session.load 的返回。
type LoadResult struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Cwd       string    `json:"cwd"`
	Mode      string    `json:"mode"`
	Messages  []Message `json:"messages"`
	Truncated bool      `json:"truncated"`
	Total     int       `json:"total"`
	// Usage 是整段会话的用量合计（所有 assistant/message 的 usage 相加）。
	// 没有 usage 的会话这里是零值——界面要据此不显示，而不是显示 0。
	Usage Usage `json:"usage"`

	// 以下是真宿主（H1/H4/H5/H12）补的字段，mockd 不产时保持零值。
	// 零值 = 不知道，界面必须整段不显示，不许拿 0 冒充。
	//
	// UsageTotal 是**全会话累计**（不会随 tail 切页变化）。
	UsageTotal Usage `json:"usageTotal"`
	// ContextTokens 是**最近一条** assistant 的 totalTokens，即"现在上下文占多少"。
	// 与 UsageTotal 是两件事：累计一直涨，上下文会随压缩回落。
	ContextTokens int64 `json:"contextTokens"`
	// Model 是最近一次 request/context 的模型名；空 = 不知道。
	Model string `json:"model"`
	// Permission 是这一进程真正的审批策略（deny/ask/allow/auto），不是 mode。
	Permission string `json:"permission"`
	// ContentWidth 是宿主知道的会话内容列宽；0 = 不知道（界面自己算）。
	ContentWidth int `json:"contentWidth"`
}

// APIError 是契约里的 error 对象。
type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Hint      string `json:"hint"`
	Retryable bool   `json:"retryable"`
}

func (e *APIError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s: %s（%s）", e.Code, e.Message, e.Hint)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Client 只认一个 baseURL，比如 http://127.0.0.1:7788。
type Client struct {
	Base string
	HTTP *http.Client

	cookies []*http.Cookie // /connect 换来的会话 cookie
}

func New(base string) *Client {
	return &Client{Base: base, HTTP: &http.Client{Timeout: 8 * time.Second}}
}

// Connect 用一次性 launch token 换 session cookie，之后所有请求都带上它。
// 真宿主（dshd serve）要求这个；mockd 不要求，所以调用方可以只在真宿主时调。
func (c *Client) Connect(ctx context.Context, token string) error {
	if token == "" {
		return fmt.Errorf("token 为空")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Base+"/connect?token="+token, nil)
	if err != nil {
		return err
	}
	// 关键：**不许跟随重定向**。
	// 真宿主的 /connect 成功时返回 303 + Set-Cookie: dshd_session=…，
	// Go 的默认客户端会跟着 Location 去 GET "/"，那一步是 404，
	// 于是我们只看到 "not found"、拿不到 cookie（踩过：app 起不来，报 /connect 返回 404）。
	noRedirect := *c.HTTP
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := noRedirect.Do(req)
	if err != nil {
		return fmt.Errorf("连不上 %s：%w", c.Base, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent, http.StatusSeeOther, http.StatusFound:
	default:
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("/connect 返回 %d：%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	if len(resp.Cookies()) == 0 {
		return fmt.Errorf("/connect 没发 cookie")
	}
	c.cookies = resp.Cookies()
	return nil
}

// Cookies 返回 /connect 换来的会话 cookie，给流客户端（Mux）复用。
//
// 为什么要有：WS 与 HTTP 用**同一套认证语义**（H14 §11.4），
// 而 Mux 是独立对象、不共享 Client 的 cookie。不显式传过去的话，
// /ws 会回 401（实测：路径改对之后下一步就撞这个）。
func (c *Client) Cookies() []*http.Cookie {
	return c.cookies
}

// SetCookies 直接注入会话 cookie，跳过 /connect。
//
// 为什么需要：launch token 是**一次性**的，所以"界面 + curl 同时验证同一个宿主"
// 做不到——谁先 connect 谁把 token 用掉。调试时先用 curl 换一次 cookie，
// 再把同一个值给界面和后续 curl，两边就能并存。
func (c *Client) SetCookies(cks []*http.Cookie) {
	c.cookies = cks
}

// envelope 是约定的响应信封（见 testdata/api.snapshot.json）。
//
// Error 用 RawMessage 而不是 *APIError：真宿主的错误体是字符串
// `{"error":"unauthorized"}`，直接解成结构体会让整个响应"不是合法 JSON"，
// 于是真正的原因（401 未授权）被吞成一条看不懂的报错。
type envelope struct {
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

// parseError 把 error 字段的两种形状都解成 APIError。
func parseError(raw json.RawMessage) *APIError {
	var e APIError
	if err := json.Unmarshal(raw, &e); err == nil && (e.Code != "" || e.Message != "") {
		return &e
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return &APIError{Code: s, Message: s}
	}
	return nil
}

// Call 发一次一元调用，把 result 解到 out。
func (c *Client) Call(ctx context.Context, method string, params any, out any) error {
	// 入参是**平铺**的：真 dshd 的 handler 直接 decode 到
	// `struct{ SessionID string `json:"sessionId"`; ID string `json:"id"`; ... }`，
	// 外面再套一层 `{"params":…}` 会让每个参数都解成零值——症状极隐蔽：
	// list 照常返回（它不需要参数），而 load 报 no_such_session、prompt 的
	// 子进程拿到空的 --session/--prompt（实测踩到，见 docs/progress.md）。
	//
	// 旧实现发的是 `{"params":…}`（那是 mockd 的早期形状）。两仓已经统一的
	// 契约是平铺，见 docs/api-contract.md 与 testdata/api.snapshot.json。
	var body []byte
	var err error
	if params == nil {
		body = []byte("{}")
	} else if body, err = json.Marshal(params); err != nil {
		return fmt.Errorf("encode params: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Base+"/api/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for _, ck := range c.cookies {
		req.AddCookie(ck)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("连不上 %s：%w", c.Base, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	var env envelope
	envOK := json.Unmarshal(raw, &env) == nil

	// 错误体两种都认：{"error":{code,message,hint,retryable}}（约定）
	// 和 {"error":"unauthorized"}（真宿主当前形状，见 DSH-GO/AGENTS.md §12 的 H8）。
	if envOK && len(env.Error) > 0 {
		if apiErr := parseError(env.Error); apiErr != nil {
			return apiErr
		}
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %.200s", resp.StatusCode, raw)
	}

	// 约定信封：有 result 就解 result。
	if envOK && len(env.Result) > 0 {
		if out == nil {
			return nil
		}
		return json.Unmarshal(env.Result, out)
	}

	// 真宿主目前**不套信封**，直接返回 payload（{"sessions":[...]}）。
	// 这里退一步把整个 body 当 result，否则界面永远看到 0 个会话（踩过）。
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("响应不是合法 JSON（HTTP %d）: %.200s", resp.StatusCode, raw)
	}
	return nil
}

// List 是 session.list。
func (c *Client) List(ctx context.Context) ([]Session, error) {
	var r struct {
		Sessions []Session `json:"sessions"`
	}
	if err := c.Call(ctx, "session.list", map[string]any{}, &r); err != nil {
		return nil, err
	}
	return r.Sessions, nil
}

// Load 是 session.load。
func (c *Client) Load(ctx context.Context, id string, tail int) (*LoadResult, error) {
	var r LoadResult
	// 两个名字都发：mockd 读 id/tailMessages，真宿主读 sessionId/limit。
	// 契约对不齐这件事已记在 DSH-GO/AGENTS.md §12（H9），对不齐之前先两边都兼容。
	params := map[string]any{"id": id, "sessionId": id, "tailMessages": tail, "limit": tail}
	if err := c.Call(ctx, "session.load", params, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// Create 是 session.create。
func (c *Client) Create(ctx context.Context, cwd, mode, title string) (string, error) {
	params := map[string]any{"mode": mode}
	if cwd != "" {
		params["cwd"] = cwd
	}
	if title != "" {
		params["title"] = title
	}
	var r struct {
		ID        string `json:"id"`
		SessionID string `json:"sessionId"` // 真宿主用这个键
	}
	if err := c.Call(ctx, "session.create", params, &r); err != nil {
		return "", err
	}
	if r.ID != "" {
		return r.ID, nil
	}
	return r.SessionID, nil
}

// Prompt 是 session.prompt。
func (c *Client) Prompt(ctx context.Context, id, text string) (string, error) {
	var r struct {
		TurnID   string `json:"turnId"`
		Accepted bool   `json:"accepted"`
	}
	if err := c.Call(ctx, "session.prompt", map[string]any{
		"id": id, "sessionId": id, "text": text, "prompt": text,
	}, &r); err != nil {
		return "", err
	}
	return r.TurnID, nil
}
