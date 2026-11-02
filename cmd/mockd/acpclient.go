// acpclient.go —— dshd 的 ACP 客户端（最小可用）。
//
// ACP 是裸 JSONL：一行一个 JSON-RPC 2.0 消息，没有 Content-Length 头
// （dshd 的 internal/acp 用 bufio.Scanner 按行读）。
//
// 只做四件事：initialize、session/new、session/prompt、以及**必须应答**的
// session/request_permission（不应答会让整轮永久挂起且毫无报错）。
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// acpUpdate 是 session/update 里我们关心的那几类。
type acpUpdate struct {
	Kind string // agent_message_chunk / agent_thought_chunk / tool_call / tool_call_update
	Text string // chunk 的正文
	Tool string // tool_call 的工具名
	Args string // tool_call 的一行摘要
}

// ACPClient 是一条到 dshd acp 子进程的连接。
type ACPClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr io.ReadCloser

	mu     sync.Mutex
	nextID int
	// pending 是「我们发出去的请求 id → 收结果的 channel」
	pending map[int]chan json.RawMessage
	// notify 处理 agent 主动发来的通知与请求
	notify func(method string, params json.RawMessage, id json.RawMessage)

	onUpdate func(acpUpdate)
	done     chan struct{}
	err      error
}

type rpcMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// StartACP 起 dshd acp 子进程并完成 initialize。
func StartACP(exe, manifest, workDir string) (*ACPClient, error) {
	args := []string{"acp"}
	if manifest != "" {
		args = append(args, "-manifest", manifest)
	}
	cmd := exec.Command(exe, args...)
	cmd.Dir = workDir
	cmd.Env = os.Environ()
	// stderr 留给人看（dshd 的 fail-loud 信息都在这里，比如缺模块）
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("起 dshd acp 失败：%w", err)
	}

	c := &ACPClient{
		cmd:     cmd,
		stdin:   in,
		stdout:  bufio.NewReaderSize(out, 1<<20),
		pending: map[int]chan json.RawMessage{},
		done:    make(chan struct{}),
	}
	go c.readLoop()

	// initialize：dshd 要先拿到它才会处理 session/prompt
	if _, err := c.call("initialize", map[string]any{
		"protocolVersion":   1,
		"clientCapabilities": map[string]any{},
	}, 10*time.Second); err != nil {
		c.Close()
		return nil, fmt.Errorf("initialize 失败：%w", err)
	}
	return c, nil
}

// readLoop 读 stdout，把响应分给等待者，把通知/请求交给 notify。
func (c *ACPClient) readLoop() {
	defer close(c.done)
	for {
		line, err := c.stdout.ReadBytes('\n')
		if err != nil {
			return
		}
		var m rpcMsg
		if err := json.Unmarshal(line, &m); err != nil {
			continue // 一行坏 JSON 不该打断整条连接
		}
		switch {
		case m.Method != "" && len(m.ID) > 0:
			// agent → client 的请求（session/request_permission）
			if c.notify != nil {
				c.notify(m.Method, m.Params, m.ID)
			}
		case m.Method != "":
			// 通知（session/update 等）
			if m.Method == "session/update" {
				c.handleUpdate(m.Params)
			}
		case len(m.ID) > 0:
			var id int
			if json.Unmarshal(m.ID, &id) == nil {
				c.mu.Lock()
				ch := c.pending[id]
				delete(c.pending, id)
				c.mu.Unlock()
				if ch != nil {
					ch <- line
					close(ch)
				}
			}
		}
	}
}

func (c *ACPClient) handleUpdate(params json.RawMessage) {
	if c.onUpdate == nil {
		return
	}
	var p struct {
		Update struct {
			SessionUpdate string `json:"sessionUpdate"`
			Content       struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			Title string `json:"title"`
			Kind  string `json:"kind"`
		} `json:"update"`
	}
	if json.Unmarshal(params, &p) != nil {
		return
	}
	u := acpUpdate{Kind: p.Update.SessionUpdate}
	switch u.Kind {
	case "agent_message_chunk", "agent_thought_chunk":
		if p.Update.Content.Type == "text" {
			u.Text = p.Update.Content.Text
		}
	case "tool_call", "tool_call_update":
		u.Tool = p.Update.Title
		if u.Tool == "" {
			u.Tool = p.Update.Kind
		}
	}
	if u.Text != "" || u.Tool != "" {
		c.onUpdate(u)
	}
}

// call 发一个请求并等它的响应。
func (c *ACPClient) call(method string, params any, timeout time.Duration) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan json.RawMessage, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	c.mu.Lock()
	_, err := c.stdin.Write(append(body, '\n'))
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	select {
	case raw := <-ch:
		var m rpcMsg
		_ = json.Unmarshal(raw, &m)
		if m.Error != nil {
			return nil, fmt.Errorf("%s: %s", method, m.Error.Message)
		}
		return m.Result, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("%s 超时（%s）", method, timeout)
	case <-c.done:
		return nil, fmt.Errorf("%s: 连接已断", method)
	}
}

// NewSession 建一个会话，返回 sessionId。
func (c *ACPClient) NewSession(cwd string) (string, error) {
	raw, err := c.call("session/new", map[string]any{"cwd": cwd, "mcpServers": []any{}}, 15*time.Second)
	if err != nil {
		return "", err
	}
	var r struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", err
	}
	return r.SessionID, nil
}

// Prompt 发一条消息，流式回调 update，直到 stopReason。
func (c *ACPClient) Prompt(sessionID, text string, onUpdate func(acpUpdate), timeout time.Duration) (string, error) {
	c.onUpdate = onUpdate
	// 权限请求：固定回 allow-once。不应答会永久挂起且无报错。
	c.notify = func(method string, params json.RawMessage, id json.RawMessage) {
		if method != "session/request_permission" {
			return
		}
		var p struct {
			ToolCall struct {
				Title string `json:"title"`
			} `json:"toolCall"`
			Options []struct {
				OptionID string `json:"optionId"`
				Kind     string `json:"kind"`
			} `json:"options"`
		}
		_ = json.Unmarshal(params, &p)
		opt := ""
		for _, o := range p.Options {
			if o.Kind == "allow_once" || o.Kind == "allow_always" {
				opt = o.OptionID
				break
			}
		}
		if opt == "" && len(p.Options) > 0 {
			opt = p.Options[0].OptionID
		}
		// id 必须原样回：ACP 的 id 可能是数字也可能是字符串，
		// 转成 int 再回会在类型不匹配时静默失配 —— 模型就一直等，整轮挂住。
		quoted, _ := json.Marshal(opt)
		var b []byte
		b = append(b, []byte(`{"jsonrpc":"2.0","id":`)...)
		b = append(b, id...)
		b = append(b, []byte(`,"result":{"outcome":{"outcome":"selected","optionId":`)...)
		b = append(b, quoted...)
		b = append(b, []byte(`}}}`)...)
		c.mu.Lock()
		c.stdin.Write(append(b, '\n'))
		c.mu.Unlock()
		if onUpdate != nil {
			onUpdate(acpUpdate{Kind: "approval_asked", Tool: p.ToolCall.Title})
		}
	}

	raw, err := c.call("session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt":    []any{map[string]any{"type": "text", "text": text}},
	}, timeout)
	if err != nil {
		return "", err
	}
	var r struct {
		StopReason string `json:"stopReason"`
	}
	_ = json.Unmarshal(raw, &r)
	return r.StopReason, nil
}

// Close 结束子进程。
func (c *ACPClient) Close() {
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait()
	}
}
