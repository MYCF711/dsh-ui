package apiclient

import "context"

// JobInfo 是 `jobs.list` 返回的一行（宿主 `transport.JobInfo` 的投影）。
//
// SessionID 是**这条任务真的属于哪条会话**：宿主在父进程侧按推送它的子进程
// 补上（`HOST:cmd/dshd/serve_approval.go` 的 onJobsForSession），不是界面猜的。
// `jobs.output` / `jobs.stop` 都要求 sessionId 与 id 同时给出（宿主拒绝
// "跨会话猜任务"：`HOST:internal/transport/handlers_jobs.go:109`），
// 所以这一格必须原样透传。
type JobInfo struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId,omitempty"`
	Command   string `json:"command"`
	Done      bool   `json:"done"`
	ExitCode  int    `json:"exit_code"`
	Bytes     int    `json:"bytes"`
}

// JobOutput 是 `jobs.output` 返回的**一页**输出。
//
// 游标以 Unicode code point 为单位（宿主 `PageJobOutput` 按 rune 切），
// 所以分页不会把中文或 emoji 切成半个字符。`Output` 永远只是一页；
// `Total` 是当前保留文本的 rune 总数，`Truncated` 表示 1 MiB 保留窗口之前
// 已有输出被淘汰——这两件事不能混。
type JobOutput struct {
	ID         string `json:"id"`
	SessionID  string `json:"sessionId,omitempty"`
	Output     string `json:"output"`
	Done       bool   `json:"done"`
	ExitCode   int    `json:"exit_code"`
	Cwd        string `json:"cwd,omitempty"`
	Cursor     int    `json:"cursor"`
	NextCursor int    `json:"nextCursor"`
	HasMore    bool   `json:"hasMore"`
	Total      int    `json:"total"`
	Truncated  bool   `json:"truncated,omitempty"`
}

// JobStopResult 只在所属进程**确实停止后**才回 stopped=true（宿主 handler
// 对"没确认结束"直接 500，见 `HOST:internal/transport/handlers_jobs.go:284`）。
type JobStopResult struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	Stopped   bool   `json:"stopped"`
	Done      bool   `json:"done"`
	ExitCode  int    `json:"exit_code"`
}

// JobsList 列后台任务。
//
// 数据来源是**宿主缓存的那份**（子进程通过 `@@JOBS@@` 推上来，
// 宿主存在 loopRunner.jobs 里）。所以它反映的是"上一次子进程推的时候"，
// 不是"此刻"——刚起的任务可能要等下一次推送才出现。
func (c *Client) JobsList(ctx context.Context) ([]JobInfo, error) {
	var out struct {
		Jobs []JobInfo `json:"jobs"`
	}
	if err := c.Call(ctx, "jobs.list", struct{}{}, &out); err != nil {
		return nil, err
	}
	return out.Jobs, nil
}

// JobsOutput 读一个后台任务的输出页（`jobs.output`）。
//
// sessionID 必须是真的所属会话：宿主生产路径带 scoped 回调时**拒绝空 sessionId**
// （`handlers_jobs.go:109-112`）。cursor=0 从开头读，之后用上一页的 NextCursor；
// limit=0 用宿主默认页（16 KiB 的 **rune** 数）。
//
// 失败是**真实的**：轮次结束后子进程退出、任务 owner 不在时宿主要么 404 要么
// 409，界面必须把错误画出来，不能拿空字符串冒充"没有输出"。
func (c *Client) JobsOutput(ctx context.Context, sessionID, id string, cursor, limit int) (JobOutput, error) {
	var out JobOutput
	req := struct {
		SessionID string `json:"sessionId"`
		ID        string `json:"id"`
		Cursor    int    `json:"cursor"`
		Limit     int    `json:"limit"`
	}{SessionID: sessionID, ID: id, Cursor: cursor, Limit: limit}
	if err := c.Call(ctx, "jobs.output", req, &out); err != nil {
		return JobOutput{}, err
	}
	return out, nil
}

// JobsStop 请求停止一个后台任务（`jobs.stop`）。
//
// 只按调用方明确命名的 session+id 停（宿主不接受进程名/镜像名/全局 PID），
// 返回的 stopped 只有在所属进程**确实结束**后才为 true。
func (c *Client) JobsStop(ctx context.Context, sessionID, id string) (JobStopResult, error) {
	var out JobStopResult
	req := struct {
		SessionID string `json:"sessionId"`
		ID        string `json:"id"`
	}{SessionID: sessionID, ID: id}
	if err := c.Call(ctx, "jobs.stop", req, &out); err != nil {
		return JobStopResult{}, err
	}
	return out, nil
}
