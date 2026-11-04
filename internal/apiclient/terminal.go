package apiclient

import "context"

// TerminalOutput 是 `terminal.output` 的返回。
type TerminalOutput struct {
	Output string `json:"output"`
	// Next 是下次该传的 `since`（字节偏移）。**增量**——
	// 每帧整段传会把界面拖死（一个跑了半小时的终端可能有几十 KB）。
	Next int  `json:"next"`
	Dead bool `json:"dead"`
}

// TerminalOpen 起一条长连接 shell（宿主 `terminal.open`）。
//
// `cwd` 空 = 宿主的当前工作目录。
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

// TerminalInput 往终端写一段。**原样**（要换行就自己带 `\n`）。
func (c *Client) TerminalInput(ctx context.Context, id, data string) error {
	return c.Call(ctx, "terminal.input", map[string]any{"id": id, "data": data}, nil)
}

// TerminalOutput 取增量输出。
func (c *Client) TerminalOutput(ctx context.Context, id string, since int) (TerminalOutput, error) {
	var out TerminalOutput
	err := c.Call(ctx, "terminal.output", map[string]any{"id": id, "since": since}, &out)
	return out, err
}
