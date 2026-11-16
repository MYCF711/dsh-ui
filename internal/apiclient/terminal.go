package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// TerminalOutput 是 `terminal.output` 的返回。
type TerminalOutput struct {
	Output string `json:"output"`
	// Next 是下次该传的 `since`（字节偏移）。**增量**——
	// 每帧整段传会把界面拖死（一个跑了半小时的终端可能有几十 KB）。
	Next int  `json:"next"`
	Dead bool `json:"dead"`
}

// TerminalOpen 起一条旧式长连接 shell（宿主 `terminal.open`）。
// 新的用户终端控制面使用 TerminalCreate；旧入口保留给既有调用方。
func (c *Client) TerminalOpen(ctx context.Context, cwd string) (string, error) {
	params := map[string]any{}
	if cwd != "" {
		params["cwd"] = cwd
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := c.Call(ctx, "terminal.open", params, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// TerminalInput 往旧式终端写一段。**原样**（要换行就自己带 `\n`）。
func (c *Client) TerminalInput(ctx context.Context, id, data string) error {
	return c.Call(ctx, "terminal.input", map[string]any{"id": id, "data": data}, nil)
}

// TerminalOutput 取增量输出。
func (c *Client) TerminalOutput(ctx context.Context, id string, since int) (TerminalOutput, error) {
	var out TerminalOutput
	err := c.Call(ctx, "terminal.output", map[string]any{"id": id, "since": since}, &out)
	return out, err
}

// TerminalShell 对应官方 TerminalShell（types.ts:24-29）。Supported 是本宿主
// additive 的可创建判据；nil 按官方契约理解为由 create 最终判定。
type TerminalShell struct {
	Path      string   `json:"path"`
	Args      []string `json:"args"`
	Name      string   `json:"name"`
	Supported *bool    `json:"supported,omitempty"`
}

// TerminalInfo 对应 rc.2 WebTerminalInfo（types.ts:53-70）。
type TerminalInfo struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Shell       TerminalShell `json:"shell"`
	Cwd         string        `json:"cwd"`
	Cols        int           `json:"cols"`
	Rows        int           `json:"rows"`
	State       string        `json:"state"`
	ExitCode    *int          `json:"exitCode"`
	Error       string        `json:"error,omitempty"`
	Origin      string        `json:"origin"`
	Control     string        `json:"control"`
	SandboxMode string        `json:"sandboxMode,omitempty"`
}

// TerminalEnvironment 对应官方 TerminalEnvironment（types.ts:31-41）。
type TerminalEnvironment struct {
	Cwd              string `json:"cwd"`
	MaxInputBytes    int    `json:"maxInputBytes"`
	MaxBufferedBytes int    `json:"maxBufferedBytes"`
	MaxCols          int    `json:"maxCols"`
	MaxRows          int    `json:"maxRows"`
	Scrollback       int    `json:"scrollback"`
}

// TerminalCreateRequest 对应官方 TerminalCreateRequest（types.ts:107-114）。
type TerminalCreateRequest struct {
	ShellPath string `json:"shellPath,omitempty"`
	ID        string `json:"id"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

func (c *Client) TerminalCreate(ctx context.Context, sessionID string, request TerminalCreateRequest) (TerminalInfo, error) {
	var out TerminalInfo
	// 官方 Remote 的参数是 agentId + request；宿主 HTTP 同时接受 sessionId。
	err := c.Call(ctx, "terminal.create", map[string]any{"agentId": sessionID, "request": request}, &out)
	return out, err
}

func (c *Client) TerminalList(ctx context.Context, sessionID string) ([]TerminalInfo, error) {
	var out []TerminalInfo
	err := c.Call(ctx, "terminal.list", map[string]any{"sessionId": sessionID}, &out)
	return out, err
}

func (c *Client) TerminalClose(ctx context.Context, sessionID, id string) error {
	return c.Call(ctx, "terminal.close", map[string]any{"agentId": sessionID, "id": id}, nil)
}

func (c *Client) TerminalEnvironment(ctx context.Context, sessionID string) (TerminalEnvironment, error) {
	var out TerminalEnvironment
	err := c.Call(ctx, "terminal.environment", map[string]any{"agentId": sessionID}, &out)
	return out, err
}

func (c *Client) TerminalShells(ctx context.Context, sessionID string) ([]TerminalShell, error) {
	var out []TerminalShell
	err := c.Call(ctx, "terminal.shells", map[string]any{"agentId": sessionID}, &out)
	return out, err
}

func (c *Client) TerminalRename(ctx context.Context, sessionID, id, title string) error {
	return c.Call(ctx, "terminal.rename", map[string]any{"agentId": sessionID, "id": id, "title": title}, nil)
}

// TerminalWrite 是官方 TerminalInput.write 的 HTTP 上行投影：data 原样写入，
// typed 只由真实人员手势产生；协议回包不冒充 xterm 协议回复。
func (c *Client) TerminalWrite(ctx context.Context, sessionID, id, data string, typed bool) error {
	return c.Call(ctx, "terminal.input", map[string]any{
		"sessionId": sessionID, "id": id, "data": data, "typed": typed,
	}, nil)
}

// TerminalFrame 对应官方 TerminalFrame（types.ts:128-132）。不同 type 只消费
// 自己拥有的字段；Sequence 在 snapshot/output/ack 中有效。
type TerminalFrame struct {
	Type     string       `json:"type"`
	Sequence int64        `json:"sequence"`
	Screen   string       `json:"screen"`
	Data     string       `json:"data"`
	Info     TerminalInfo `json:"info"`
	Writable bool         `json:"writable"`
	Refusal  string       `json:"refusal,omitempty"`
}

// TerminalStream 是宿主官方 mode:'stream' 方法的 NDJSON 载体。一行是一帧，
// 没有 result 包壳；中途错误是 type:error + 四字段 error。
type TerminalStream struct {
	body    io.ReadCloser
	decoder *json.Decoder
}

func (c *Client) openTerminalStream(ctx context.Context, method string, params any) (*TerminalStream, error) {
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Base+"/api/"+method, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/x-ndjson")
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}
	// 长连接不能继承普通一元调用的 8s 整体超时；取消只由 ctx/ Close 决定。
	httpClient := *c.HTTP
	httpClient.Timeout = 0
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连不上 %s：%w", c.Base, err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if readErr != nil {
			return nil, readErr
		}
		var env envelope
		if json.Unmarshal(raw, &env) == nil && len(env.Error) > 0 {
			if apiErr := parseError(env.Error); apiErr != nil {
				return nil, apiErr
			}
		}
		return nil, fmt.Errorf("%s 返回 HTTP %d: %.200s", method, resp.StatusCode, raw)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/x-ndjson") {
		resp.Body.Close()
		return nil, fmt.Errorf("%s 必须返回 application/x-ndjson，拿到 %q", method, got)
	}
	return &TerminalStream{body: resp.Body, decoder: json.NewDecoder(resp.Body)}, nil
}

func (s *TerminalStream) Close() error {
	if s == nil || s.body == nil {
		return nil
	}
	return s.body.Close()
}

func (s *TerminalStream) decode(out any) error {
	if s == nil || s.decoder == nil {
		return io.EOF
	}
	var raw json.RawMessage
	if err := s.decoder.Decode(&raw); err != nil {
		return err
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var kind struct {
			Type  string          `json:"type"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(raw, &kind); err != nil {
			return fmt.Errorf("终端流帧不是 JSON 对象：%w", err)
		}
		if kind.Type == "error" {
			if apiErr := parseError(kind.Error); apiErr != nil {
				return apiErr
			}
			return fmt.Errorf("终端流返回未命名错误")
		}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("终端流帧形状错误：%w", err)
	}
	return nil
}

func (s *TerminalStream) Next() (TerminalFrame, error) {
	var frame TerminalFrame
	err := s.decode(&frame)
	return frame, err
}

func (s *TerminalStream) NextRoster() ([]TerminalInfo, error) {
	var roster []TerminalInfo
	err := s.decode(&roster)
	return roster, err
}

func (s *TerminalStream) NextRetained() error {
	var frame struct {
		Type string `json:"type"`
	}
	if err := s.decode(&frame); err != nil {
		return err
	}
	if frame.Type != "retained" {
		return fmt.Errorf("terminal.retain 首帧必须是 retained，拿到 %q", frame.Type)
	}
	return nil
}

func (c *Client) TerminalAttach(ctx context.Context, sessionID, id string) (*TerminalStream, error) {
	return c.openTerminalStream(ctx, "terminal.attach", map[string]any{"agentId": sessionID, "id": id})
}

func (c *Client) TerminalWatch(ctx context.Context, sessionID string) (*TerminalStream, error) {
	return c.openTerminalStream(ctx, "terminal.watch", map[string]any{"sessionId": sessionID})
}

func (c *Client) TerminalRetain(ctx context.Context, sessionID, id string) (*TerminalStream, error) {
	return c.openTerminalStream(ctx, "terminal.retain", map[string]any{"sessionId": sessionID, "id": id})
}
