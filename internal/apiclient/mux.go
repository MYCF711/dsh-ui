// mux.go —— Transport 契约的流客户端（对照 testdata/api.snapshot.json 的 stream 段）。
//
// 契约要点（docs/api-contract.md §2）：
//   - 一条 WebSocket，话题 `session:<id>` / `turn:<id>`；
//   - 服务端帧：ev / gap / pong；
//   - **seq 必须连续**；断号发 gap 帧，调用方拿 gap 去 `session.history` 补洞，
//     **绝不整体刷新**（长会话会闪、滚动位置会丢）。
//
// 本文件只做客户端。v1 一条订阅一条连接；连接复用（一条 WS 多 topic）留给 U1，
// 那时候需要服务端按 topic 分流，mockd 现在只有一个 topic，先不为此造复杂度。
package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Event 是流上的一条消息。
//
// Kind 为 "gap" 时 Seq 无意义，看 Gap 字段：调用方应当用 session.history
// 拉 [FromSeq, ToSeq] 这段补上，**不要**重载整个会话。
type Event struct {
	Kind  string          // v4 事件名（user/message / assistant/message / …）；断号时是 "gap"
	Seq   int             // 事件序号；Kind=="gap" 时为 0
	Topic string          // 所属话题
	Data  json.RawMessage // 事件载荷
	Gap   *GapInfo        // 仅 Kind=="gap"
}

// GapInfo 描述一段缺失的序号区间（闭区间）。
type GapInfo struct {
	FromSeq int    `json:"fromSeq"`
	ToSeq   int    `json:"toSeq"`
	Reason  string `json:"reason"`
}

// Mux 是一条流连接。
type Mux struct {
	// Base 是宿主根地址，形如 http://127.0.0.1:7788（会被换成 ws://）。
	Base string
	// Dial 默认按 Base 推导 ws(s)://.../api/mux；测试里可以替换。
	Dial func(ctx context.Context, urlStr string) (*websocket.Conn, error)
	// PingEvery 是应用层心跳间隔；<=0 用 20s。
	PingEvery time.Duration
}

// NewMux 用和 Client 同一个 base 建流客户端。
func NewMux(base string) *Mux {
	return &Mux{Base: base}
}

func (m *Mux) muxURL() (string, error) {
	u, err := url.Parse(m.Base)
	if err != nil {
		return "", fmt.Errorf("Base 不是合法 URL（%q）：%w", m.Base, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
		// 已经是对的
	default:
		return "", fmt.Errorf("Base 的 scheme 必须是 http/https/ws/wss，收到 %q", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/mux"
	return u.String(), nil
}

// serverFrame 是服务端帧的线上形状。
type serverFrame struct {
	T       string          `json:"t"`
	Topic   string          `json:"topic,omitempty"`
	Seq     int             `json:"seq,omitempty"`
	Kind    string          `json:"kind,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
	FromSeq int             `json:"fromSeq,omitempty"`
	ToSeq   int             `json:"toSeq,omitempty"`
	Reason  string          `json:"reason,omitempty"`
}

type clientFrame struct {
	T        string `json:"t"`
	Topic    string `json:"topic,omitempty"`
	SinceSeq int    `json:"sinceSeq,omitempty"`
}

// Subscribe 订阅一个话题，从 sinceSeq 之后开始收。
//
// 返回的 channel 在三种情况下关闭：ctx 取消、连接断开、或读循环出错。
// **调用方必须一直读到 channel 关闭**，否则内部 goroutine 会卡在写 channel 上。
func (m *Mux) Subscribe(ctx context.Context, topic string, sinceSeq int) (<-chan Event, error) {
	if topic == "" {
		return nil, fmt.Errorf("topic 不能为空（形如 session:<id>）")
	}
	u, err := m.muxURL()
	if err != nil {
		return nil, err
	}
	dial := m.Dial
	if dial == nil {
		dial = func(ctx context.Context, urlStr string) (*websocket.Conn, error) {
			c, _, err := websocket.Dial(ctx, urlStr, &websocket.DialOptions{
				HTTPClient: nil, // 默认
			})
			return c, err
		}
	}
	conn, err := dial(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("连不上 %s：%w", u, err)
	}

	// 订阅帧必须是最先发出去的。
	sub, _ := json.Marshal(clientFrame{T: "sub", Topic: topic, SinceSeq: sinceSeq})
	if err := conn.Write(ctx, websocket.MessageText, sub); err != nil {
		conn.CloseNow()
		return nil, fmt.Errorf("发 sub 失败：%w", err)
	}

	out := make(chan Event, 64)
	go m.readLoop(ctx, conn, topic, sinceSeq, out)
	return out, nil
}

func (m *Mux) readLoop(ctx context.Context, conn *websocket.Conn, topic string, sinceSeq int, out chan<- Event) {
	defer close(out)
	defer conn.CloseNow()

	// 心跳：契约用应用层 ping/pong，不用 WS 控制帧（那样服务端要另写一套）。
	pingEvery := m.PingEvery
	if pingEvery <= 0 {
		pingEvery = 20 * time.Second
	}
	pingCtx, stopPing := context.WithCancel(ctx)
	defer stopPing()
	var pingWG sync.WaitGroup
	pingWG.Add(1)
	go func() {
		defer pingWG.Done()
		t := time.NewTicker(pingEvery)
		defer t.Stop()
		for {
			select {
			case <-pingCtx.Done():
				return
			case <-t.C:
				b, _ := json.Marshal(clientFrame{T: "ping"})
				if err := conn.Write(ctx, websocket.MessageText, b); err != nil {
					return
				}
			}
		}
	}()
	defer pingWG.Wait()

	last := sinceSeq
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return // 断开 / ctx 取消
		}
		var f serverFrame
		if json.Unmarshal(data, &f) != nil {
			continue
		}
		switch f.T {
		case "pong":
			continue

		case "gap":
			out <- Event{Kind: "gap", Topic: f.Topic, Gap: &GapInfo{FromSeq: f.FromSeq, ToSeq: f.ToSeq, Reason: f.Reason}}
			// 补洞之后从 ToSeq 接着算连续性，避免把"已经被告知的缺口"再报一次。
			if f.ToSeq > last {
				last = f.ToSeq
			}

		case "ev":
			// 客户端自己也查一遍连续性：服务端漏发 gap 时，这里兜底。
			if last != 0 && f.Seq != last+1 {
				out <- Event{Kind: "gap", Topic: f.Topic, Gap: &GapInfo{
					FromSeq: last + 1,
					ToSeq:   f.Seq - 1,
					Reason:  "客户端检测到断号（服务端未发 gap 帧）",
				}}
			}
			last = f.Seq
			out <- Event{Kind: f.Kind, Seq: f.Seq, Topic: f.Topic, Data: f.Data}

		default:
			// 未知帧类型：忽略，不打断流（前向兼容）。
		}
	}
}

// listenHost 只是为了在错误信息里说人话，保留给后续 U1 用。
func listenHost(base string) string {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return base
	}
	h, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		return u.Host
	}
	return h
}
