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
	resp, err := c.HTTP.Do(req)
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

type envelope struct {
	Result json.RawMessage `json:"result"`
	Error  *APIError       `json:"error"`
}

// Call 发一次一元调用，把 result 解到 out。
func (c *Client) Call(ctx context.Context, method string, params any, out any) error {
	body, err := json.Marshal(map[string]any{"params": params})
	if err != nil {
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
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("响应不是合法 JSON（HTTP %d）: %.200s", resp.StatusCode, raw)
	}
	if env.Error != nil {
		return env.Error
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %.200s", resp.StatusCode, raw)
	}
	if out == nil || len(env.Result) == 0 {
		return nil
	}
	return json.Unmarshal(env.Result, out)
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
	if err := c.Call(ctx, "session.load", map[string]any{"id": id, "tailMessages": tail}, &r); err != nil {
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
		ID string `json:"id"`
	}
	if err := c.Call(ctx, "session.create", params, &r); err != nil {
		return "", err
	}
	return r.ID, nil
}

// Prompt 是 session.prompt。
func (c *Client) Prompt(ctx context.Context, id, text string) (string, error) {
	var r struct {
		TurnID   string `json:"turnId"`
		Accepted bool   `json:"accepted"`
	}
	if err := c.Call(ctx, "session.prompt", map[string]any{"id": id, "text": text}, &r); err != nil {
		return "", err
	}
	return r.TurnID, nil
}
