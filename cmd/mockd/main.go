// mockd —— Transport 契约的假宿主。
//
// 用途：在真宿主接上之前，让界面先能跑起来；同时验证 `testdata/api.snapshot.json`
// 里的 13 个 method 形状是否够用。**它不碰会话日志，也不连 dshd。**
//
//	go run ./cmd/mockd            # 监听 127.0.0.1:7788
//	curl -s -XPOST 127.0.0.1:7788/api/session.list -d '{}'
//
// 入参形状：**平铺**（`{"id":"…","tailMessages":2}`）是唯一契约，与真宿主一致；
// 旧的 `{"params":{…}}` 只有这里为历史脚本继续收，见 decodeParams 的注释。
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
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
	// role == "tool" 时这三个字段描述一次调用（与 apiclient.Message 同名同义）。
	Tool    string `json:"tool,omitempty"`
	Args    string `json:"args,omitempty"`
	Summary string `json:"summary,omitempty"`
	// Usage 只在这一条是 assistant/message 时有值（宿主给的 usage 原样 + 我们算的速率）。
	Usage *usageInfo `json:"usage,omitempty"`
	// View 让工具自己声明这块结果怎么显示（形状见 viewSpec）。
	// 界面对 View 零容忍：Kind 不认识就退化成 Text——不认识 == 没声明。
	View *viewSpec `json:"view,omitempty"`
}

// usageInfo 是 token 用量。TokPerSec 是我们自己算的（宿主只给用量，不给耗时）。
type usageInfo struct {
	InputTokens     int64   `json:"inputTokens"`
	OutputTokens    int64   `json:"outputTokens"`
	TotalTokens     int64   `json:"totalTokens"`
	CacheReadTokens int64   `json:"cacheReadTokens"`
	TokPerSec       float64 `json:"tokPerSec,omitempty"`
	HasRate         bool    `json:"hasRate,omitempty"`
}

// viewSpec 的形状逐字对齐宿主线的 spi.ViewSpec（DSH-GO/internal/spi/tool.go:103）：
// 界面线的插件靠它做到「一切皆插件」——工具不渲染自己，只声明 Kind 和内容，
// 由界面侧按 Kind 走对应的渲染器。字段名、可空性都不许改；改了要同步
// testdata/api.snapshot.json 与 DSH-GO 那边的结构体。
type viewSpec struct {
	Kind    string      `json:"kind"`              // card | table | plain
	Title   string      `json:"title,omitempty"`   // 卡片标题（一般是工具名）
	Badge   string      `json:"badge,omitempty"`   // 右上角小标：ok / error / 耗时
	Fields  []viewField `json:"fields,omitempty"`  // Kind=card
	Columns []string    `json:"columns,omitempty"` // Kind=table
	Rows    [][]string  `json:"rows,omitempty"`    // Kind=table，每行长度必须 == len(Columns)
	Body    string      `json:"body,omitempty"`    // Kind=plain
}

// viewField 是 card 的一行键值。Value 允许换行（多行输出照原样放进来）。
type viewField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// realDB 非空时，session.list / session.load 走真实会话（-sessions 指定目录）。
var realDB *realStore

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
	sessionsDir := flag.String("sessions", "", "真实会话根目录（空=用假数据）；只读，不会写回")
	tailMsgs := flag.Int("tail", 200, "每个会话最多读多少条消息")
	acpExe := flag.String("acp", "", "dshd 可执行文件路径；给了就接真模型（否则用假回答）")
	acpMani := flag.String("acp-manifest", "", "dshd 的 manifest（dsh.toml 列了没编译进去的模块时必须给精简版）")
	acpDir := flag.String("acp-cwd", "D:/DeepSeek Harness/default-workspace/dsh-ui", "ACP 会话的工作目录")
	// 端口默认由 OS 分配（:0），启动后把真实地址打到日志。
	//
	// 为什么不能写死 7788：测试夹具和调试进程会抢同一个端口——实测有一次
	// `go test` 一直红，查了半天才发现是我自己起的 mockd 占着 7788，
	// 测试连到了那个已被订阅过的实例（跳号被消耗光）。
	// 需要固定端口的场景用 -addr 写死。
	listenAddr := flag.String("addr", "127.0.0.1:0", "监听地址（:0 = 由 OS 分配，从日志读真实地址）")

	flag.Parse()

	// /ws 是流；/api/ 是一元 method 的兜底前缀，两者不冲突。
	http.HandleFunc("/ws", muxHandler)
	http.HandleFunc("/api/", route)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"ok": "mockd", "time": time.Now().Format(time.RFC3339)})
	})
	if *sessionsDir != "" {
		st, err := loadRealSessions(*sessionsDir, *tailMsgs)
		if err != nil {
			log.Printf("WARN 读真实会话失败，回落到假数据: %v", err)
		} else {
			realDB = st
			log.Printf("真实会话已载入: %d 个（扫到 %d 个文件，跳过 %d 个坏文件）", len(st.byID), st.files, st.bad)
		}
	}

	if *acpExe != "" {
		startACP(*acpExe, *acpMani, *acpDir)
	}

	// 端口默认由 OS 分配（:0），启动后把真实地址打到 stdout。
	//
	// 为什么不能写死 7788：测试夹具和调试进程会抢同一个端口——实测有一次
	// `go test` 一直红，查了半天才发现是我自己起的 mockd 占着 7788，
	// 测试连到了那个已被订阅过的实例（跳号被消耗光）。
	// 需要固定端口的场景用 -addr。
	addr := *listenAddr
	src := "fake data"
	if acp != nil {
		src = "ACP → dshd（真模型）"
	}
	if realDB != nil {
		src = fmt.Sprintf("%d real sessions", len(realDB.byID))
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("监听失败：%v", err)
	}
	// 这一行是给脚本/测试读的：`mockd listening on http://127.0.0.1:PORT`
	log.Printf("mockd listening on http://%s (14 methods + /ws, %s)", ln.Addr().String(), src)
	log.Fatal(http.Serve(ln, nil))
}

// ---------------------------------------------------------------------------
// /ws —— Transport 契约的流（H14 基线：路径 /ws，帧 {seq,type,data,time}，见 docs/api-contract.md §2）
//
// 假数据，只为把客户端的三条路走一遍：
//   - 收到 sub 后每 200ms 推一条 ev；kind 在 user/message 与 assistant/message 间交替；
//     seq 从客户端给的 sinceSeq 逐条 +1。
//   - **第 7 条之后故意跳两个号**：先发 gap{fromSeq,toSeq}，再继续推更大的 seq，
//     逼客户端把"补洞"那条路真的走一遍。
//   - ping → pong。
// ---------------------------------------------------------------------------

// clientFrame 是客户端控制帧（H14：t=sub/unsub/ping）。
type clientFrame struct {
	T        string `json:"t"`
	Topic    string `json:"topic,omitempty"`
	SinceSeq int    `json:"sinceSeq,omitempty"`
}

// serverFrame 是服务端事件帧（H14 唯一基线）：{seq,type,data,time}。
//
// `type` 是 v4 原始事件名；心跳回包是 {"type":"pong"}。
// **没有 topic**：连接即会话归属（一个会话一条连接）。
// **不发 gap 帧**：断号由客户端按 seq 自己发现，再调 session.history 补洞。
type serverFrame struct {
	Seq  int             `json:"seq"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
	Time int64           `json:"time,omitempty"`
}

// mockApprovalFlag：流里推一条**真的** approval/request 帧（形状照宿主 docs/h13-approval.md）。
//
// 为什么需要：宿主 H13 执行链没闭环，真宿主推不出这个帧，而界面要在「帧一到就能用」
// 上被验证。开了它，客户端走的就是完整链路：WS 帧 → 审批条 → 点按钮 → POST session.approve。
var mockApprovalFlag = flag.Bool("approval", false, "流里推一条 approval/request 帧（验界面审批链路用）")

// 这两个必须是**包级**：放在连接里的话，客户端断号重连后 n 归零、又跳一次号，
// 于是"跳号→断号→重连→跳号"无限循环（实测 10 秒内重连几十次）。
var (
	mockGappedOnce   bool // 跳号只做一次
	mockApprovalOnce bool // approval 帧只推一次
)

const (
	muxTick     = 200 * time.Millisecond // 背景假事件
	muxTickChar = 60 * time.Millisecond  // 流式回答：一个字一帧
)

func muxHandler(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		log.Printf("mux: accept: %v", err)
		return
	}
	defer c.CloseNow()
	ctx := r.Context()

	// 读侧只把命令投进 chan，不自己写 —— 全程单写，避免并发写。
	cmds := make(chan clientFrame, 8)
	go func() {
		defer close(cmds)
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var f clientFrame
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

	writeFrame := func(f serverFrame) bool {
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

	// 字符流：60ms 一个字，推 session.prompt 登记的那条回答。
	charTick := time.NewTicker(muxTickChar)
	defer charTick.Stop()

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
					if !writeFrame(serverFrame{Type: "error", Data: json.RawMessage(`{"reason":"sub 缺 topic"}`)}) {
						return
					}
					continue
				}
				topic, seq, n, subbed, gapped = f.Topic, f.SinceSeq, 0, true, false
				log.Printf("mux: sub topic=%q sinceSeq=%d", topic, seq)
			case "unsub":
				subbed = false
			case "ping":
				if !writeFrame(serverFrame{Type: "pong"}) {
					return
				}
			}

		case ev, ok := <-acpLive:
			// 真模型的流：ACP 的 update 已经翻成 acpEvent，这里只负责编号 + 写帧。
			// acpLive 为 nil 时这个 case 永远阻塞 —— 没开 -acp 就完全不影响。
			if !ok || !subbed {
				continue
			}
			seq++
			data := map[string]any{}
			switch ev.Kind {
			case "assistant/message":
				if ev.Done {
					data["done"] = true
				} else {
					data["delta"] = ev.Delta
				}
			case "assistant/thought":
				data["delta"] = ev.Delta
			case "tool/call":
				data["tool"] = ev.Tool
				data["args"] = ev.Args
			case "approval/asked":
				data["tool"] = ev.Tool
			}
			b, _ := json.Marshal(data)
			if !writeFrame(serverFrame{Seq: seq, Type: ev.Kind, Data: b, Time: time.Now().UnixMilli()}) {
				return
			}

		case <-charTick.C:
			if !subbed {
				continue
			}
			sid := strings.TrimPrefix(topic, "session:")
			ch, ok, lineDone, allDone := popPendingChar(sid)
			if !ok {
				if allDone {
					commitPending(sid)
				}
				continue
			}
			seq++
			d, _ := json.Marshal(map[string]string{"delta": string(ch)})
			if !writeFrame(serverFrame{Seq: seq, Type: "assistant/message", Data: d, Time: time.Now().UnixMilli()}) {
				return
			}
			if lineDone {
				seq++
				dd, _ := json.Marshal(map[string]bool{"done": true})
				if !writeFrame(serverFrame{Seq: seq, Type: "assistant/message", Data: dd, Time: time.Now().UnixMilli()}) {
					return
				}
			}
			if allDone {
				full := commitPending(sid)
				log.Printf("mux: 流式回答推完，已写入 history（%d 字符）", len([]rune(full)))
			}

		case <-ticker.C:
			if !subbed {
				continue
			}
			n++
			if n == 8 && !gapped && !mockGappedOnce {
				// H14：服务端**不发 gap 帧**。这里只把 seq 跳过两个号，
				// 客户端按 seq 连续性自己发现断号、再调 session.history 补洞。
				seq += 2
				gapped = true
				mockGappedOnce = true
				log.Printf("mux: 故意跳号，下一个 seq=%d（客户端应自己发现）", seq+1)
			}
			// 第 3 条之后插一条审批帧：形状与宿主完全一致（含 allow-once/reject-once
			// 两个 optionId 和对应的 kind——注意宿主的 id 是连字符、kind 是下划线）。
			if *mockApprovalFlag && n == 3 && !mockApprovalOnce {
				mockApprovalOnce = true
				seq++
				apData := json.RawMessage(`{"requestId":"perm-mock-1","sessionId":"mock","toolCall":{"toolCallId":"perm-mock-1","title":"bash: echo approval-test","kind":"execute","status":"pending","rawInput":{"command":"echo approval-test"}},"options":[{"optionId":"allow-once","name":"允许这一次","kind":"allow_once"},{"optionId":"reject-once","name":"拒绝","kind":"reject_once"}]}`)
				if !writeFrame(serverFrame{Seq: seq, Type: "approval/request", Data: apData, Time: time.Now().UnixMilli()}) {
					return
				}
				log.Printf("mux: 推了一条 approval/request（requestId=perm-mock-1）")
			}
			seq++
			kind := "user/message"
			if n%2 == 0 {
				kind = "assistant/message"
			}
			data, _ := json.Marshal(map[string]string{
				"text": fmt.Sprintf("第 %d 条假事件（topic=%s）", seq, topic),
			})
			if !writeFrame(serverFrame{Seq: seq, Type: kind, Data: data, Time: time.Now().UnixMilli()}) {
				return
			}
		}
	}
}

// ── 入参：平铺是唯一真相，{"params":{…}} 只为历史脚本继续收 ────────────────
//
// 真宿主每个 handler 都是 `json.NewDecoder(r.Body).Decode(&in)` 直接解到**平铺**
// 结构（../DSH-GO/internal/transport/server.go：session.prompt:330、cancel:352、
// create:369、load:412、approve:434、history:464），它自己的契约测试
// envelope_test.go:74 也是平铺。
//
// 两边形状不一致的后果是**参数静默变零值**：session.list 照常返回（它不要参数）、
// session.load 报 not_found、session.prompt 拿到空 id 与空 text —— 看起来像
// "会话坏了"，其实是解包解错了。所以：
//
//   - 新代码只发平铺；
//   - mockd 继续收 {"params":{…}}（老的 curl 示例、cmd/acpsmoke、历史脚本都在用）。
//
// 13 个 method 共用下面这一处解包，别在 handler 里各写一遍。

// decodeParams 从请求体取出真正的入参。
//
// 规则：顶层有 "params" 且是对象 → 用它（兼容层）；否则整个 body 就是入参。
// 第二个返回值 false 表示 body 不是合法 JSON 对象（返回 400，不猜）。
func decodeParams(raw []byte) (json.RawMessage, bool) {
	body := bytes.TrimSpace(raw)
	if len(body) == 0 {
		// 没 body：参数全是零值。宿主也是这个行为（Decode 的 EOF 被 `_ =` 吞掉）。
		return nil, true
	}
	if body[0] != '{' {
		// 数组/字符串/数字：协议只认对象。这类 body 从来不是"参数少给了"而是发错了。
		return nil, false
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, false
	}
	if p, has := top["params"]; has {
		var wrapped map[string]json.RawMessage
		if json.Unmarshal(p, &wrapped) == nil { // 必须是对象；不是对象就当平铺用
			return p, true
		}
	}
	return body, true
}

// readParams 读请求体（上限 1 MiB，mockd 没有大 body 的正当用途）再解包。
func readParams(r *http.Request) (json.RawMessage, bool) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, false
	}
	return decodeParams(raw)
}

// 下面几个结构体的字段名**逐个对齐真宿主 handler**，一个名字都不能少：
// 同一个字段两个名字，只认一个就是静默 404（宿主 server.go:382 的 fillID 注释
// 记的就是这个坑，H9/H10）。

// loadParams 是 session.load：id/sessionId 都收（sessionId 优先），
// tailMessages/limit 都收（limit 优先，与宿主 firstNonZero(in.Limit, in.TailMessages) 一致）。
type loadParams struct {
	SessionID    string `json:"sessionId"`
	ID           string `json:"id"`
	Limit        int    `json:"limit"`
	TailMessages int    `json:"tailMessages"`
}

func (p loadParams) sessionID() string { return fillSessionID(p.SessionID, p.ID) }
func (p loadParams) tail() int         { return firstNonZeroInt(p.Limit, p.TailMessages) }

// historyParams 是 session.history。
type historyParams struct {
	SessionID string `json:"sessionId"`
	ID        string `json:"id"`
	SinceSeq  int    `json:"sinceSeq"`
	Limit     int    `json:"limit"`
}

func (p historyParams) sessionID() string { return fillSessionID(p.SessionID, p.ID) }

// promptParams 是 session.prompt（宿主只读 text，不读 prompt/attachments）。
type promptParams struct {
	SessionID string `json:"sessionId"`
	ID        string `json:"id"`
	Text      string `json:"text"`
}

func (p promptParams) sessionID() string { return fillSessionID(p.SessionID, p.ID) }

// cancelParams 是 session.cancel。
type cancelParams struct {
	SessionID string `json:"sessionId"`
	ID        string `json:"id"`
}

func (p cancelParams) sessionID() string { return fillSessionID(p.SessionID, p.ID) }

// createParams 是 session.create（宿主只读 mode）。
type createParams struct {
	Mode string `json:"mode"`
}

// fillSessionID 与宿主 server.go:395 同义：sessionId 优先，其次 id。
func fillSessionID(sessionID, id string) string {
	if sessionID != "" {
		return sessionID
	}
	return id
}

// firstNonZeroInt 与宿主 server.go:388 同义：取第一个非零值。
func firstNonZeroInt(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

func route(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, errObj{Code: "invalid_arg", Message: "只接受 POST", Hint: `用 POST /api/<method>，body 形如 {"id":"s-001"}`})
		return
	}
	method := strings.TrimPrefix(r.URL.Path, "/api/")
	params, ok := readParams(r)
	if !ok {
		fail(w, 400, errObj{
			Code:    "invalid_arg",
			Message: "body 不是合法 JSON 对象",
			Hint:    `平铺形如 {"id":"s-001"}；老的 {"params":{…}} 也收`,
		})
		return
	}
	mu.Lock()
	defer mu.Unlock()

	switch method {
	case "session.list":
		if realDB != nil {
			out := make([]session, 0, len(realDB.order))
			for _, id := range realDB.order {
				rs := realDB.byID[id]
				title := firstUserLine(rs)
				out = append(out, session{
					ID: rs.ID, Title: title, Cwd: rs.Cwd,
					UpdatedAt: rs.UpdatedAt.Format(time.RFC3339), Mode: "default",
				})
			}
			writeJSON(w, 200, envelope{Result: map[string]any{"sessions": out}})
			return
		}
		writeJSON(w, 200, envelope{Result: map[string]any{"sessions": sessions}})

	case "session.create":
		var p createParams
		_ = json.Unmarshal(params, &p)
		mode := p.Mode
		if mode == "" {
			mode = "default"
		}
		nextN++
		s := session{ID: fmt.Sprintf("s-%03d", nextN), Title: "新会话", Cwd: "", UpdatedAt: time.Now().Format(time.RFC3339), Mode: mode}
		sessions = append([]session{s}, sessions...)
		writeJSON(w, 200, envelope{Result: map[string]any{"id": s.ID, "sessionId": s.ID, "createdAt": s.UpdatedAt, "mode": s.Mode}})

	case "session.load":
		var p loadParams
		_ = json.Unmarshal(params, &p)
		id, tail := p.sessionID(), p.tail()
		if realDB != nil {
			rs, ok := realDB.byID[id]
			if !ok {
				fail(w, 404, errObj{Code: "not_found", Message: "会话不存在: " + id, Hint: "先调 session.list 拿一个真实 id"})
				return
			}
			// 索引期只解到第一条 user 消息；第一次 load 这个会话时才全量解并缓存。
			// 判断条件必须是 Full，不能是 len(Messages)==0（见 realSession.Full 的注释）。
			if !rs.Full && rs.Path != "" {
				if full, err := decodeSession(rs.Path, 0, false); err == nil {
					realDB.byID[id] = full
					rs = full
				}
			}
			msgs := rs.Messages
			if tail > 0 && len(msgs) > tail {
				msgs = msgs[len(msgs)-tail:]
			}
			writeJSON(w, 200, envelope{Result: map[string]any{
				"id": rs.ID, "title": firstUserLine(rs), "cwd": rs.Cwd, "mode": "default",
				"messages": msgs, "truncated": len(msgs) < rs.Total, "total": rs.Total,
				"usage": rs.Usage,
			}})
			return
		}
		h, ok := history[id]
		if !ok {
			fail(w, 404, errObj{Code: "not_found", Message: "会话不存在: " + id, Hint: "先调 session.list 拿一个真实 id"})
			return
		}
		if tail <= 0 || tail > len(h) {
			tail = len(h)
		}
		writeJSON(w, 200, envelope{Result: map[string]any{
			"id": id, "title": "假会话", "cwd": "D:/DeepSeek Harness/default-workspace", "mode": "default",
			"messages": h[len(h)-tail:], "truncated": tail < len(h), "total": len(h),
		}})

	case "session.history":
		// H14 基线：**返回 journal 原始帧**（{seq,type,data,time}），不是 messages 投影。
		//
		// 为什么 mockd 也要按这个形状：客户端的补洞（FillGap）拿回来直接喂 applyEvent，
		// 形状不对就没法验"断号之后真能补回来"。宿主实测就是这个形状
		// （docs/reconnect.md：history{"sinceSeq":166} → frames 167..277）。
		var p historyParams
		_ = json.Unmarshal(params, &p)
		if p.sessionID() == "" {
			fail(w, 404, errObj{Code: "not_found", Message: "会话不存在"})
			return
		}
		limit := p.Limit
		if limit <= 0 {
			limit = 10
		}
		frames := make([]map[string]any, 0, limit)
		for i := p.SinceSeq + 1; i <= p.SinceSeq+limit; i++ {
			frames = append(frames, map[string]any{
				"seq":  i,
				"type": "assistant/message",
				"data": map[string]any{"text": fmt.Sprintf("补洞补回来的第 %d 条", i)},
				"time": time.Now().UnixMilli(),
			})
		}
		writeJSON(w, 200, envelope{Result: map[string]any{
			"frames":       frames,
			"lastSeq":      p.SinceSeq + len(frames),
			"journalEpoch": 1,
		}})

	case "session.prompt":
		var p promptParams
		_ = json.Unmarshal(params, &p)
		id := p.sessionID()
		// ACP 模式下可以往"真实会话"发消息，那些不在假的 history 表里
		_, inFake := history[id]
		inReal := realDB != nil && realDB.byID[id] != nil
		if !inFake && !inReal {
			fail(w, 404, errObj{Code: "not_found", Message: "会话不存在: " + id, Hint: "先调 session.list 拿一个真实 id"})
			return
		}
		if strings.TrimSpace(p.Text) == "" {
			fail(w, 400, errObj{Code: "invalid_arg", Message: "text 为空", Hint: "至少一个非空白字符"})
			return
		}
		if busy[id] {
			fail(w, 409, errObj{Code: "busy", Message: "该会话有 turn 在跑", Retryable: true, Hint: "先 session.cancel，或等 turn 结束"})
			return
		}
		busy[id] = true
		if inFake {
			h := history[id]
			h = append(h, message{Seq: len(h) + 1, Role: "user", Text: p.Text})
			history[id] = h
		}
		if acp != nil {
			go runACPPrompt(id, p.Text) // 真模型：流事件会塞进 acpLive；结束时自己清 busy
		} else {
			enqueueReplyLocked(id, p.Text) // 假回答：排队等 /ws 的订阅者来取
		}
		nextN++
		writeJSON(w, 200, envelope{Result: map[string]any{"turnId": fmt.Sprintf("t-%d", nextN), "accepted": true}})

	case "session.cancel":
		var p cancelParams
		_ = json.Unmarshal(params, &p)
		busy[p.sessionID()] = false
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

	case "session.approve":
		var in struct {
			RequestID string `json:"requestId"`
			OptionID  string `json:"optionId"`
		}
		_ = json.Unmarshal(params, &in)
		if in.RequestID == "" || in.OptionID == "" {
			fail(w, 400, errObj{Code: "invalid_arg", Message: "session.approve 需要 requestId 与 optionId",
				Hint: `形如 {"requestId":"perm-mock-1","optionId":"allow-once"}`})
			return
		}
		// 记录到 stderr：验收时要看到"界面真的把用户的决定送回来了"。
		log.Printf("approve: requestId=%s optionId=%s（界面回传成功）", in.RequestID, in.OptionID)
		writeJSON(w, 200, envelope{Result: map[string]any{"accepted": true, "optionId": in.OptionID}})

	case "session.delete", "session.fork", "fs.read", "fs.write", "config.set":
		fail(w, 501, errObj{Code: "unsupported", Message: method + " 属于第二批，mockd 暂未实现", Hint: "见 docs/api-contract.md §3"})

	default:
		fail(w, 404, errObj{Code: "unsupported", Message: "未知 method: " + method, Hint: "13 个合法值见 docs/api-contract.md §3"})
	}
}

// ── 工具结果的显示声明（假宿主里的 ViewSpec 镜像）──────────────────────
//
// 宿主的真身是 spi.ViewSpec（DSH-GO/internal/spi/tool.go:103）：工具不渲染自己，
// 只声明 Kind（card / table / plain）和内容，界面按 Kind 选渲染器。
// 一条铁律：只显示参数与结果里真的有的东西；推不出来就返回 nil，界面退回纯文本。

const tvNL = "\n"

// tvCall 是解码期暂存的一次工具调用（tool/call 先来，tool/result 用 callId 配对）。
type tvCall struct {
	name      string
	arguments string
}

// tvJoinBlocks 把 content 里的 text 块拼成纯文本。
func tvJoinBlocks(blocks []block) string {
	var b strings.Builder
	for _, c := range blocks {
		if c.Type != "text" || c.Text == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString(tvNL)
		}
		b.WriteString(c.Text)
	}
	return b.String()
}

// tvToolView 把一次工具调用压成界面能直接画的声明。
func tvToolView(name, args, result string, isError bool) *viewSpec {
	if name == "" {
		return nil
	}
	badge := "ok"
	if isError {
		badge = "error"
	}
	params := tvParseArgs(args)

	switch {
	case tvIsShell(name):
		v := &viewSpec{Kind: "card", Title: name, Badge: badge}
		if cmd := tvFirstParam(params, "command", "cmd", "script", "commandLine"); cmd != "" {
			v.Fields = append(v.Fields, viewField{Key: "命令", Value: tvOneLine(cmd, 160)})
		} else if args != "" {
			v.Fields = append(v.Fields, viewField{Key: "参数", Value: tvOneLine(args, 160)})
		}
		if first := tvFirstLine(result); first != "" {
			v.Fields = append(v.Fields, viewField{Key: "输出", Value: tvOneLine(first, 160)})
		}
		if len(v.Fields) == 0 {
			return nil
		}
		return v

	case tvIsList(name):
		rows := tvRows(result, 8)
		if len(rows) == 0 {
			return nil
		}
		return &viewSpec{Kind: "table", Title: name, Badge: badge,
			Columns: []string{"结果"}, Rows: rows}

	case tvIsRead(name):
		if strings.TrimSpace(result) == "" {
			return nil
		}
		return &viewSpec{Kind: "plain", Title: name, Badge: badge,
			Body: tvTrunc(result, 1200)}
	}

	v := &viewSpec{Kind: "card", Title: name, Badge: badge}
	if q := tvFirstParam(params, "file_path", "path", "pattern", "query", "url", "prompt", "task"); q != "" {
		v.Fields = append(v.Fields, viewField{Key: "参数", Value: tvOneLine(q, 160)})
	}
	if first := tvFirstLine(result); first != "" {
		v.Fields = append(v.Fields, viewField{Key: "结果", Value: tvOneLine(first, 160)})
	}
	if len(v.Fields) == 0 {
		return nil
	}
	return v
}

func tvParseArgs(args string) map[string]any {
	args = strings.TrimSpace(args)
	if args == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(args), &m); err != nil {
		return nil
	}
	return m
}

func tvFirstParam(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func tvIsShell(name string) bool {
	switch tvBase(name) {
	case "bash", "powershell", "shell", "exec", "run", "job_run":
		return true
	}
	return false
}

func tvIsList(name string) bool {
	switch tvBase(name) {
	case "ls", "glob", "find", "grep", "list", "search":
		return true
	}
	return false
}

func tvIsRead(name string) bool {
	switch tvBase(name) {
	case "read", "write", "edit", "view", "cat":
		return true
	}
	return false
}

func tvBase(name string) string {
	if i := strings.LastIndex(name, "__"); i >= 0 && i+2 < len(name) {
		return name[i+2:]
	}
	return name
}

func tvRows(result string, limit int) [][]string {
	var lines []string
	for _, line := range strings.Split(result, tvNL) {
		if t := strings.TrimSpace(line); t != "" {
			lines = append(lines, t)
		}
	}
	if len(lines) == 0 {
		return nil
	}
	rows := make([][]string, 0, limit+1)
	for i := 0; i < len(lines) && i < limit; i++ {
		rows = append(rows, []string{tvTrunc(lines[i], 100)})
	}
	if len(lines) > limit {
		rows = append(rows, []string{"… 另有 " + tvItoa(len(lines)-limit) + " 行"})
	}
	return rows
}

func tvOneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	return tvTrunc(s, max)
}

func tvFirstLine(s string) string {
	for _, line := range strings.Split(s, tvNL) {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}

func tvTrunc(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func tvItoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 && i > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
