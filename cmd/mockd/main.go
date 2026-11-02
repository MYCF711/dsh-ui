// mockd —— Transport 契约的假宿主。
//
// 用途：在真宿主接上之前，让界面先能跑起来；同时验证 `testdata/api.snapshot.json`
// 里的 13 个 method 形状是否够用。**它不碰会话日志，也不连 dshd。**
//
//	go run ./cmd/mockd            # 监听 127.0.0.1:7788
//	curl -s -XPOST 127.0.0.1:7788/api/session.list -d '{"params":{}}'
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

type errObj struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Hint      string `json:"hint,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
}

type envelope struct {
	Result any     `json:"result,omitempty"`
	Error  *errObj `json:"error,omitempty"`
}

type session struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Cwd       string `json:"cwd"`
	UpdatedAt string `json:"updatedAt"`
	Mode      string `json:"mode"`
}

type message struct {
	Seq   int    `json:"seq"`
	Role  string `json:"role"`
	Text  string `json:"text"`
	SeqID string `json:"-"`
}

var (
	mu       sync.Mutex
	sessions = []session{
		{ID: "s-001", Title: "界面线 spike 讨论", Cwd: "D:/DeepSeek Harness/default-workspace", UpdatedAt: "2026-11-03T05:40:00+08:00", Mode: "default"},
		{ID: "s-002", Title: "UI 插件化架构方案讨论", Cwd: "D:/DeepSeek Harness/default-workspace", UpdatedAt: "2026-11-03T04:10:00+08:00", Mode: "default"},
		{ID: "s-003", Title: "1000 条压力会话", Cwd: "D:/DeepSeek Harness/default-workspace", UpdatedAt: "2026-11-03T06:00:00+08:00", Mode: "default"},
	}
	history = map[string][]message{
		"s-001": seedHistory("s-001", 40),
		"s-002": seedHistory("s-002", 12),
		"s-003": seedHistory("s-003", 1000),
	}
	busy  = map[string]bool{}
	nextN = 100
)

func seedHistory(id string, n int) []message {
	out := make([]message, 0, n)
	for i := 1; i <= n; i++ {
		role := "user"
		if i%2 == 0 {
			role = "assistant"
		}
		out = append(out, message{Seq: i, Role: role, Text: fmt.Sprintf("第 %d 条：这里是中文与 English 混排的示例文本，用来验证滚动与虚拟化。", i)})
	}
	return out
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, e errObj) {
	writeJSON(w, code, envelope{Error: &e})
}

func main() {
	// /api/mux 比 /api/ 长，ServeMux 会优先精确匹配到它，不影响 13 个一元 method。
	http.HandleFunc("/api/mux", muxHandler)
	http.HandleFunc("/api/", route)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"ok": "mockd", "time": time.Now().Format(time.RFC3339)})
	})
	addr := "127.0.0.1:7788"
	log.Printf("mockd listening on http://%s (13 methods + /api/mux, fake data)", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// ---------------------------------------------------------------------------
// /api/mux —— Transport 契约的流（docs/api-contract.md §2）
//
// 假数据，只为把客户端的三条路走一遍：
//   - 收到 sub 后每 200ms 推一条 ev；kind 在 user/message 与 assistant/message 间交替；
//     seq 从客户端给的 sinceSeq 逐条 +1。
//   - **第 7 条之后故意跳两个号**：先发 gap{fromSeq,toSeq}，再继续推更大的 seq，
//     逼客户端把"补洞"那条路真的走一遍。
//   - ping → pong。
// ---------------------------------------------------------------------------

// muxFrame 收发同构：客户端只填 t/topic/sinceSeq，服务端只填其余字段。
type muxFrame struct {
	T        string          `json:"t"`
	Topic    string          `json:"topic,omitempty"`
	SinceSeq int             `json:"sinceSeq,omitempty"`
	Seq      int             `json:"seq,omitempty"`
	Kind     string          `json:"kind,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
	FromSeq  int             `json:"fromSeq,omitempty"`
	ToSeq    int             `json:"toSeq,omitempty"`
	Reason   string          `json:"reason,omitempty"`
}

const muxTick = 200 * time.Millisecond

func muxHandler(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		log.Printf("mux: accept: %v", err)
		return
	}
	defer c.CloseNow()
	ctx := r.Context()

	// 读侧只把命令投进 chan，不自己写 —— 全程单写，避免并发写。
	cmds := make(chan muxFrame, 8)
	go func() {
		defer close(cmds)
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var f muxFrame
			if json.Unmarshal(data, &f) != nil {
				continue
			}
			select {
			case cmds <- f:
			case <-ctx.Done():
				return
			}
		}
	}()

	writeFrame := func(f muxFrame) bool {
		b, err := json.Marshal(f)
		if err != nil {
			return false
		}
		return c.Write(ctx, websocket.MessageText, b) == nil
	}

	var (
		topic  string
		seq    int
		n      int
		subbed bool
		gapped bool
	)
	ticker := time.NewTicker(muxTick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case f, ok := <-cmds:
			if !ok {
				return // 对端断开
			}
			switch f.T {
			case "sub":
				if f.Topic == "" {
					if !writeFrame(muxFrame{T: "gap", Reason: "sub 缺 topic"}) {
						return
					}
					continue
				}
				topic, seq, n, subbed, gapped = f.Topic, f.SinceSeq, 0, true, false
				log.Printf("mux: sub topic=%q sinceSeq=%d", topic, seq)
			case "unsub":
				subbed = false
			case "ping":
				if !writeFrame(muxFrame{T: "pong"}) {
					return
				}
			}

		case <-ticker.C:
			if !subbed {
				continue
			}
			n++
			if n == 8 && !gapped {
				from := seq + 1
				to := from + 1
				seq = to
				gapped = true
				if !writeFrame(muxFrame{T: "gap", Topic: topic, FromSeq: from, ToSeq: to, Reason: "mockd: 故意跳号，客户端应去 session.history 补洞"}) {
					return
				}
				log.Printf("mux: 跳号 gap from=%d to=%d", from, to)
			}
			seq++
			kind := "user/message"
			if n%2 == 0 {
				kind = "assistant/message"
			}
			data, _ := json.Marshal(map[string]string{
				"text": fmt.Sprintf("第 %d 条假事件（topic=%s）", seq, topic),
			})
			if !writeFrame(muxFrame{T: "ev", Topic: topic, Seq: seq, Kind: kind, Data: data}) {
				return
			}
		}
	}
}

func route(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, errObj{Code: "invalid_arg", Message: "只接受 POST", Hint: "用 POST /api/<method>，body 形如 {\"params\":{}}"})
		return
	}
	method := strings.TrimPrefix(r.URL.Path, "/api/")
	var body struct {
		Params json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, 400, errObj{Code: "invalid_arg", Message: "body 不是合法 JSON", Hint: `形如 {"params":{}}`})
		return
	}
	mu.Lock()
	defer mu.Unlock()

	switch method {
	case "session.list":
		writeJSON(w, 200, envelope{Result: map[string]any{"sessions": sessions}})

	case "session.create":
		nextN++
		s := session{ID: fmt.Sprintf("s-%03d", nextN), Title: "新会话", Cwd: "", UpdatedAt: time.Now().Format(time.RFC3339), Mode: "default"}
		sessions = append([]session{s}, sessions...)
		writeJSON(w, 200, envelope{Result: map[string]any{"id": s.ID, "createdAt": s.UpdatedAt, "mode": s.Mode}})

	case "session.load":
		var p struct {
			ID           string `json:"id"`
			TailMessages int    `json:"tailMessages"`
		}
		_ = json.Unmarshal(body.Params, &p)
		h, ok := history[p.ID]
		if !ok {
			fail(w, 404, errObj{Code: "not_found", Message: "会话不存在: " + p.ID, Hint: "先调 session.list 拿一个真实 id"})
			return
		}
		tail := p.TailMessages
		if tail <= 0 || tail > len(h) {
			tail = len(h)
		}
		writeJSON(w, 200, envelope{Result: map[string]any{
			"id": p.ID, "title": "假会话", "cwd": "D:/DeepSeek Harness/default-workspace", "mode": "default",
			"messages": h[len(h)-tail:], "truncated": tail < len(h), "total": len(h),
		}})

	case "session.history":
		var p struct {
			ID     string `json:"id"`
			Limit  int    `json:"limit"`
		}
		_ = json.Unmarshal(body.Params, &p)
		h, ok := history[p.ID]
		if !ok {
			fail(w, 404, errObj{Code: "not_found", Message: "会话不存在: " + p.ID})
			return
		}
		writeJSON(w, 200, envelope{Result: map[string]any{"messages": h, "nextBefore": nil}})

	case "session.prompt":
		var p struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		}
		_ = json.Unmarshal(body.Params, &p)
		if _, ok := history[p.ID]; !ok {
			fail(w, 404, errObj{Code: "not_found", Message: "会话不存在: " + p.ID})
			return
		}
		if strings.TrimSpace(p.Text) == "" {
			fail(w, 400, errObj{Code: "invalid_arg", Message: "text 为空", Hint: "至少一个非空白字符"})
			return
		}
		if busy[p.ID] {
			fail(w, 409, errObj{Code: "busy", Message: "该会话有 turn 在跑", Retryable: true, Hint: "先 session.cancel，或等 turn 结束"})
			return
		}
		busy[p.ID] = true
		h := history[p.ID]
		h = append(h, message{Seq: len(h) + 1, Role: "user", Text: p.Text})
		history[p.ID] = h
		nextN++
		writeJSON(w, 200, envelope{Result: map[string]any{"turnId": fmt.Sprintf("t-%d", nextN), "accepted": true}})

	case "session.cancel":
		var p struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(body.Params, &p)
		busy[p.ID] = false
		writeJSON(w, 200, envelope{Result: map[string]any{"cancelled": true}})

	case "workspace.list":
		writeJSON(w, 200, envelope{Result: map[string]any{"roots": []map[string]any{
			{"path": "D:/DeepSeek Harness/default-workspace", "name": "default-workspace", "isGit": false},
			{"path": "D:/DeepSeek Harness/default-workspace/DSH-GO", "name": "DSH-GO", "isGit": true},
			{"path": "D:/DeepSeek Harness/default-workspace/dsh-ui", "name": "dsh-ui", "isGit": true},
		}}})

	case "config.get":
		writeJSON(w, 200, envelope{Result: map[string]any{"values": map[string]any{
			"model": "deepseek-v4.1", "theme": "dark", "locale": "zh-CN",
		}}})

	case "session.delete", "session.fork", "fs.read", "fs.write", "config.set":
		fail(w, 501, errObj{Code: "unsupported", Message: method + " 属于第二批，mockd 暂未实现", Hint: "见 docs/api-contract.md §3"})

	default:
		fail(w, 404, errObj{Code: "unsupported", Message: "未知 method: " + method, Hint: "13 个合法值见 docs/api-contract.md §3"})
	}
}
