// history.go —— `session.history` 的客户端（断号补洞用）。
//
// 为什么需要它：WS 流断了、或者客户端自己检测到 seq 断号时，**唯一正确的补法**
// 是只补缺失的那一段，而不是整页 `session.load`（那会闪、会丢滚动位置）。
// 宿主侧实测（2026-11-03，`docs/reconnect.md`）：
//
//	POST /api/session.history {"id":…,"sinceSeq":166,"limit":…}
//	→ {"result":{"frames":[{seq,type,data,time},…],"lastSeq":277,"journalEpoch":1}}
//
// 返回的帧与 WS 推的**同一个形状**（同一个 journal 的原始帧），所以补回来的东西
// 可以直接喂给 applyEvent，不需要第二套解析。
package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
)

// Frame 是 journal 里的一条原始帧（WS 与 history 共用这个形状）。
type Frame struct {
	Seq  int             `json:"seq"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
	Time int64           `json:"time,omitempty"`
}

// HistoryResult 是 session.history 的返回。
type HistoryResult struct {
	Frames       []Frame `json:"frames"`
	LastSeq      int     `json:"lastSeq"`
	JournalEpoch int     `json:"journalEpoch"`
}

// History 取某会话从 sinceSeq **之后**开始的一段原始帧。
//
// sinceSeq 的语义是开区间：传 166 会拿到 167 起（宿主实测如此，别传 FromSeq）。
// limit <= 0 时用一个保守默认值，避免一次拉爆。
func (c *Client) History(ctx context.Context, id string, sinceSeq, limit int) (*HistoryResult, error) {
	if id == "" {
		return nil, fmt.Errorf("history 需要会话 id")
	}
	if limit <= 0 {
		limit = 200
	}
	var out HistoryResult
	err := c.Call(ctx, "session.history", map[string]any{
		"id":       id,
		"sinceSeq": sinceSeq,
		"limit":    limit,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// FillGap 用 history 补一段缺口，把补回的帧按顺序交给 apply。
//
// 返回补了多少帧；调用方据此决定要不要提示"有内容缺失"。
//
// **失败不致命**：宿主可能回 409 stale_epoch（journal 换代，旧水位无意义）——
// 那种情况补洞本来就补不了，应当退回 `session.load` 重建基线，而不是重试到死。
func FillGap(ctx context.Context, c *Client, id string, from, to int, apply func(Frame)) (int, error) {
	if from <= 0 || to < from {
		return 0, nil
	}
	// 区间是闭区间 [from,to]，而 history 的 sinceSeq 是开区间 → 传 from-1。
	res, err := c.History(ctx, id, from-1, to-from+1)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, f := range res.Frames {
		if f.Seq < from || f.Seq > to {
			continue // 宿主多给的不要：补洞只补这一段
		}
		apply(f)
		n++
	}
	return n, nil
}
