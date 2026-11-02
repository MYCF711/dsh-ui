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
	Role string `json:"role"`
	Text string `json:"text"`
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
}

func New(base string) *Client {
	return &Client{Base: base, HTTP: &http.Client{Timeout: 8 * time.Second}}
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
