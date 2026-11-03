package apiclient

import "context"

// JobInfo 是 `jobs.list` 返回的一行（宿主 `spi.JobInfo` 的投影）。
type JobInfo struct {
	ID       string `json:"id"`
	Command  string `json:"command"`
	Done     bool   `json:"done"`
	ExitCode int    `json:"exit_code"`
	Bytes    int    `json:"bytes"`
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
