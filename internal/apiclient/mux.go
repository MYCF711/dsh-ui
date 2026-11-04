// mux.go —— Transport 契约的流客户端（对照 testdata/api.snapshot.json 的 stream 段）。
//
// 契约要点（H14 唯一线上基线，见 DSH-GO/AGENTS.md §11.5-6）：
//   - **WS 路径是 `/ws`**（不是 `/api/mux`——那是界面早期的自定形状，已废弃）；
//   - 客户端控制帧用 `t=sub/unsub/ping`；
//   - **服务端事件帧是 `{seq,type,data,time}`**，`type` 是 v4 原始事件名或 `pong`；
//     不是 `{t:"ev",kind:…}`（那套两真相已禁止）；
//   - `ServerFrame` 不带 topic，所以**一个会话一条连接**，按连接归属隔离事件，
//     不许在同一条连接上混订多个会话；
//   - **seq 连不连续由客户端查**：断号时调 `session.history` 补洞，
//     不指望服务端发明 gap 帧，也**绝不整体刷新**（长会话会闪、滚动位置会丢）。
//
// 本文件只做客户端。
package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	Time  int64           // canonical 帧时间（epoch 毫秒）；0 = 未知，不用本地墙钟补
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
	// Dial 默认按 Base 推导 ws(s)://.../ws；测试里可以替换。
	Dial func(ctx context.Context, urlStr string) (*websocket.Conn, error)
	// PingEvery 是应用层心跳间隔；<=0 用 20s。
	PingEvery time.Duration
	// Cookies 是 /connect 换来的会话 cookie。WS 与 HTTP 同一套认证语义，
	// 不带上就是 401（H14 §11.4）。
	Cookies []*http.Cookie
	// EpochOf 返回某个 topic 当前 journal 世代（壳从 session.load 的
	// journalEpoch 拿到）。用回调而不是改 Subscribe 签名：订阅点很多，
	// 而世代是"壳每帧都可能刷新"的东西，回调能随时取最新值。
	// nil 或返回 0 = 不知道，按首次订阅处理。
	EpochOf func(topic string) int64
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
	u.Path = strings.TrimRight(u.Path, "/") + "/ws"
	return u.String(), nil
}

// serverFrame 是服务端帧的线上形状（H14 基线）：
//
//	{"seq":12,"type":"assistant/message","data":{…},"time":1762…}
//
// `type` 是 v4 原始事件名；心跳回包是 `{"type":"pong"}`。
// 没有 topic 字段——连接本身就是会话归属。
// WS 与 session.history 复用 Frame 的 canonical 解码，避免两条路径的时间规则漂移。
type serverFrame = Frame

type clientFrame struct {
	T        string `json:"t"`
	Topic    string `json:"topic,omitempty"`
	SinceSeq int    `json:"sinceSeq,omitempty"`
	// JournalEpoch 是客户端以为的世代（H14）。0 = 首次订阅；带上且对不上，
	// 宿主会发 session/reset 让客户端丢掉旧水位重建基线。不带的话，
	// 宿主重启换 journal 之后客户端会拿旧水位静默错位——这是 A 实测的第四层坑。
	JournalEpoch int64 `json:"journalEpoch,omitempty"`
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
		// cookie 走 HTTPHeader（websocket 库没有 jar 概念）：
		// 与 /api 的 cookie 是同一个值，服务端认的是 cookie 不是头名。
		var hdr http.Header
		if len(m.Cookies) > 0 {
			hdr = http.Header{}
			for _, ck := range m.Cookies {
				hdr.Add("Cookie", ck.Name+"="+ck.Value)
			}
		}
		dial = func(ctx context.Context, urlStr string) (*websocket.Conn, error) {
			c, _, err := websocket.Dial(ctx, urlStr, &websocket.DialOptions{
				HTTPHeader: hdr,
			})
			return c, err
		}
	}
	conn, err := dial(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("连不上 %s：%w", u, err)
	}

	// 订阅帧必须是最先发出去的。
	var epoch int64
	if m.EpochOf != nil {
		epoch = m.EpochOf(topic)
	}
	sub, _ := json.Marshal(clientFrame{T: "sub", Topic: topic, SinceSeq: sinceSeq, JournalEpoch: epoch})
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
	// **顺序很关键**：先 stopPing 让 ping goroutine 退出，再 Wait。
	// 反过来（defer stopPing 在前、defer Wait 在后，LIFO 先 Wait）会死锁：
	// 对端断开但 ctx 未取消时，ping goroutine 卡在 conn.Write 上不返回，
	// Wait 永远等不到 → close(out) 永不执行 → 调用方 for-range 永不结束。
	defer func() {
		stopPing()
		pingWG.Wait()
	}()

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
		if f.Type == "" {
			continue // 心跳回包或空帧
		}
		if f.Type == "pong" {
			continue
		}
		// seq<=0 的帧（error / session/reset 这类控制帧）**不参与水位**。
		// 曾经直接 `last = f.Seq`，于是收到一个 error 帧就把水位归零，
		// 后面的正常事件全部漏掉断号检测（实测：seq 1 → 5 中间缺 2/3/4 却报 0 个 gap）。
		if f.Seq <= 0 {
			// session/reset 也走这里：它没有 seq（不代表某个事件），但必须交给
			// 调用方——否则客户端不知道"你手里的水位作废了，重新 load"，
			// 会拿旧水位继续订阅，被宿主按 seq>sinceSeq 静默过滤。
			out <- Event{Kind: f.Type, Seq: 0, Topic: topic, Data: f.Data, Time: f.Time}
			continue
		}
		// 重复 / 倒退的 seq：调用方已经消费过这一号，**静默丢弃**。
		// 重连时宿主按块重放、或同一帧重复投递都会走到这里；再投一次就是消息重影。
		// （实测：seq=3 推两次会给调用方两份同样的内容。）
		if last != 0 && f.Seq <= last {
			continue
		}
		// 连续性由客户端查（H14：服务端只发事件帧，不发 gap）。
		// 断号时把缺口区间交给调用方，由它带水位重订——绝不整页 load。
		if last != 0 && f.Seq > last+1 {
			out <- Event{Kind: "gap", Topic: topic, Gap: &GapInfo{
				FromSeq: last + 1,
				ToSeq:   f.Seq - 1,
				Reason:  "客户端检测到断号",
			}}
		}
		last = f.Seq
		// topic 用客户端订阅的那个：服务端帧不带 topic；Data/Time 原样保真。
		out <- f.Event(topic)
	}
}

// ── 流事件 → 内容块（v4 原始词汇，不另造事件名）──────────────────────

// Block 是流上一段内容的种类，取值直接照 v4 的 content 块类型写：
// reasoning（思考）/ text（正文）/ tool（工具调用）。
//
// **不要在这里发明新名字。** 宿主写日志用什么词，界面就用什么词；
// 一旦翻译成 "thinking" 之类，就多出一处会漂移的映射，而且没人知道它对应 v4 的哪一块。
type Block string

const (
	BlockReasoning Block = "reasoning"
	BlockText      Block = "text"
	BlockTool      Block = "tool"
)

// Chunk 是一帧里的一段内容。
//
// 一帧可能是增量（`{"delta":"…"}`），也可能是一整条 v4 事件
// （`{"content":[{"type":"reasoning","text":…},{"type":"text",…}]}`，
// 真宿主 follow 把日志事件原样推过来就是这么一副形状）。
// Full=true 表示这段本身就是完整的，要立刻落成消息，不要攒。
type Chunk struct {
	Block Block
	Text  string
	Tool  string
	Args  string
	Full  bool

	// 内容身份来自 data，而不是 Event.Seq。result 仅供 Stream 区分 tool/result
	// 与 tool/call；二者都可能以完整块到达，但完整工具块不等于 turn 结束。
	V4Seq     *int64
	MessageID string
	TurnID    string
	Turn      *int64
	Step      *int64
	CallID    string
	role      string
	result    bool
}

// packet 是 ev 帧 data 的联合形状。三种都真出现过：
//
//	{"delta":"…"} / {"done":true}                    增量流（mockd 逐字推、ACP）
//	{"content":[…]} / {"message":{"content":[…]}}    v4 原始事件（真宿主 follow 原样推）
//	{"tool":"bash","args":"…"}                       工具调用（也兼容 v4 name/arguments）
//
// 只认一种的后果实测过：真宿主推的是 content 块，只读 delta 就等于
// **整条流一个字都没进界面**，而屏幕上看起来只是"没反应"。
type packet struct {
	Delta     string        `json:"delta"`
	Text      string        `json:"text"`
	Done      bool          `json:"done"`
	Tool      string        `json:"tool"`
	Args      string        `json:"args"`
	Name      string        `json:"name"`
	Arguments string        `json:"arguments"`
	Content   []packetBlock `json:"content"`

	// data 内的内容身份。v4 seq=0 是合法值，所以数值字段都必须用指针。
	Seq        *int64 `json:"seq"`
	V4Seq      *int64 `json:"v4Seq"`
	MessageID  string `json:"messageId"`
	ID         string `json:"id"`
	TurnID     string `json:"turnId"`
	Turn       *int64 `json:"turn"`
	Step       *int64 `json:"step"`
	CallID     string `json:"callId"`
	ToolCallID string `json:"toolCallId"`

	// Role 用来区分"这条 content 是工具结果"还是普通正文。
	// 宿主的 tool/result 帧长这样：
	//   {"message":{"content":[{"type":"text","text":"[exit 0] …"}],
	//               "role":"tool","toolCallId":"call_…"}}
	// 不读 role 的话，工具输出会被当成 assistant 正文流进气泡里——
	// 界面上看到的是"回答里混着命令输出"，而不是一张工具卡。
	Role    string         `json:"role"`
	Message *packetMessage `json:"message"`
}

type packetMessage struct {
	Content    []packetBlock `json:"content"`
	Role       string        `json:"role"`
	Seq        *int64        `json:"seq"`
	V4Seq      *int64        `json:"v4Seq"`
	MessageID  string        `json:"messageId"`
	ID         string        `json:"id"`
	TurnID     string        `json:"turnId"`
	Turn       *int64        `json:"turn"`
	Step       *int64        `json:"step"`
	ToolCallID string        `json:"toolCallId"`
	Source     struct {
		CallID string `json:"callId"`
	} `json:"source"`
}

// packetBlock 是 v4 content 数组里的一块。tool-call 块的字段名按 v4 原样。
type packetBlock struct {
	Type       string `json:"type"`
	Text       string `json:"text"`
	Name       string `json:"name"`
	Arguments  string `json:"arguments"`
	ID         string `json:"id"`
	CallID     string `json:"callId"`
	ToolCallID string `json:"toolCallId"`
}

// blocks 取 content 块：顶层优先，其次 message.content（两种落法都见过）。
func (p packet) blocks() []packetBlock {
	if len(p.Content) > 0 {
		return p.Content
	}
	if p.Message != nil {
		return p.Message.Content
	}
	return nil
}

// firstText 取第一个非空串（增量在 delta，也有生产者放在 text）。
func firstText(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstInt64(vals ...*int64) *int64 {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

// decorate 把帧级身份补到每个块；块级 callId（v4 tool-call.id）优先。
// 所有值只做读取和复制，不把 Transport seq 或生成的字符串填进真值字段。
func (p packet) decorate(out []Chunk) []Chunk {
	for i := range out {
		if out[i].V4Seq == nil && p.Message != nil {
			out[i].V4Seq = firstInt64(p.V4Seq, p.Seq, p.Message.V4Seq, p.Message.Seq)
		} else if out[i].V4Seq == nil {
			out[i].V4Seq = firstInt64(p.V4Seq, p.Seq)
		}
		if out[i].MessageID == "" {
			if p.Message != nil {
				out[i].MessageID = firstText(p.MessageID, p.ID, p.Message.MessageID, p.Message.ID)
			} else {
				out[i].MessageID = firstText(p.MessageID, p.ID)
			}
		}
		if out[i].TurnID == "" {
			out[i].TurnID = p.TurnID
			if out[i].TurnID == "" && p.Message != nil {
				out[i].TurnID = p.Message.TurnID
			}
		}
		if out[i].Turn == nil && p.Message != nil {
			out[i].Turn = firstInt64(p.Turn, p.Message.Turn)
		} else if out[i].Turn == nil {
			out[i].Turn = p.Turn
		}
		if out[i].Step == nil && p.Message != nil {
			out[i].Step = firstInt64(p.Step, p.Message.Step)
		} else if out[i].Step == nil {
			out[i].Step = p.Step
		}
		if out[i].CallID == "" {
			out[i].CallID = firstText(p.CallID, p.ToolCallID)
			if out[i].CallID == "" && p.Message != nil {
				out[i].CallID = firstText(p.Message.ToolCallID, p.Message.Source.CallID)
			}
		}
		if out[i].role == "" {
			out[i].role = p.Role
			if out[i].role == "" && p.Message != nil {
				out[i].role = p.Message.Role
			}
		}
	}
	return out
}

func chunksWithRole(out []Chunk, role string) []Chunk {
	for i := range out {
		out[i].role = role
	}
	return out
}

// Chunks 把一帧拆成若干段内容，块顺序保持 v4 原样。不认识的帧返回 nil。
func (e Event) Chunks() []Chunk {
	var p packet
	if len(e.Data) > 0 {
		_ = json.Unmarshal(e.Data, &p)
	}
	switch e.Kind {
	case "assistant/reasoning", "assistant/thought":
		// 宿主 agent 层的事件名是 assistant/reasoning（internal/agent/loop.go）；
		// ACP 桥（mockd -acp）把同一个 v4 块叫 assistant/thought。
		// 两条到达路径，同一种内容，都当思考；这**不是**新造事件名。
		if s := firstText(p.Delta, p.Text); s != "" {
			return p.decorate([]Chunk{{Block: BlockReasoning, Text: s}})
		}
		return nil

	case "assistant/text":
		if s := firstText(p.Delta, p.Text); s != "" {
			return p.decorate([]Chunk{{Block: BlockText, Text: s}})
		}
		return nil

	case "tool/call":
		tool, args := firstText(p.Tool, p.Name), firstText(p.Args, p.Arguments)
		if tool != "" || args != "" {
			return p.decorate([]Chunk{{Block: BlockTool, Tool: tool, Args: args}})
		}
		return nil

	case "user/message":
		return chunksWithRole(p.chunks(), "user")

	case "assistant/message":
		return p.chunks()

	default:
		// 其它 kind 只在真带了 content 块时才当内容（turn/end 之类不该凭空造正文）。
		return p.chunks()
	}
}

// isToolResult 判断这一帧是不是工具结果（宿主的 role 在 message 里，mockd 的也有在顶层）。
func (p packet) isToolResult() bool {
	if p.Message != nil && p.Message.Role == "tool" {
		return true
	}
	return p.Role == "tool"
}

// chunks 把数据包拆成块；没有块就退回增量形状。
func (p packet) chunks() (out []Chunk) {
	defer func() { out = p.decorate(out) }()
	if p.isToolResult() {
		// 工具结果整条进来（一条 text 块），交给 Stream 记成 role=="tool" 的消息，
		// 不要拆成正文增量——那是"命令输出混进回答里"的来源。
		var b strings.Builder
		for _, blk := range p.blocks() {
			if blk.Type == "text" {
				b.WriteString(blk.Text)
			}
		}
		if b.Len() > 0 {
			return []Chunk{{Block: BlockTool, Text: b.String(), Full: true, result: true}}
		}
		return nil
	}
	if blocks := p.blocks(); len(blocks) > 0 {
		out := make([]Chunk, 0, len(blocks))
		for _, b := range blocks {
			switch b.Type {
			case "reasoning":
				if b.Text != "" {
					out = append(out, Chunk{Block: BlockReasoning, Text: b.Text, Full: true})
				}
			case "text":
				if b.Text != "" {
					out = append(out, Chunk{Block: BlockText, Text: b.Text, Full: true})
				}
			case "tool-call":
				out = append(out, Chunk{
					Block: BlockTool, Tool: b.Name, Args: b.Arguments, Full: true,
					CallID: firstText(b.CallID, b.ToolCallID, b.ID),
				})
			}
		}
		return out
	}
	if p.Delta != "" {
		return []Chunk{{Block: BlockText, Text: p.Delta}}
	}
	if p.Text != "" {
		// 带 text 而不是 delta 的帧是**整条**（mockd 的假事件就是这种）。
		return []Chunk{{Block: BlockText, Text: p.Text, Full: true}}
	}
	return nil
}

// End 报告这一帧是否说明"这一段说完了"。
//
// 三种结束信号都要认：mockd 的 `{"done":true}`、宿主的 turn/done 与 turn/error。
// 漏掉 turn/done 的后果是正文永远停在流式气泡里、history 里没有这一条——
// 刷新一次就整段消失。
func (e Event) End() bool {
	switch e.Kind {
	case "turn/done", "turn/error", "turn/end":
		return true
	}
	var p packet
	if len(e.Data) > 0 && json.Unmarshal(e.Data, &p) == nil {
		return p.Done
	}
	return false
}

// contentMeta 是 data 里的真值；它绝不包含 Transport Event.Seq。
type contentMeta struct {
	v4Seq     *int64
	messageID string
	turnID    string
	turn      *int64
	step      *int64
}

func metaOf(c Chunk) contentMeta {
	return contentMeta{v4Seq: c.V4Seq, messageID: c.MessageID, turnID: c.TurnID, turn: c.Turn, step: c.Step}
}

func (m contentMeta) identified() bool {
	return m.v4Seq != nil || m.messageID != "" || m.turnID != "" || m.turn != nil
}

func (m contentMeta) seq() int {
	if m.v4Seq == nil {
		return 0
	}
	v := int(*m.v4Seq)
	if int64(v) != *m.v4Seq { // 保留 V4Seq 原始值；旧 int 字段无法容纳时不截断。
		return 0
	}
	return v
}

func stepKey(step *int64) string {
	if step == nil {
		return "unknown"
	}
	return strconv.FormatInt(*step, 10)
}

// baseKey 的优先级是 v4 seq → messageId → turnId/turn(+step) → 调用方明确给出的
// 局部 fallback。turn+step 是复合作用域，不把单独的 turn 当成一条消息的全局 ID。
func (m contentMeta) baseKey(fallback string) string {
	switch {
	case m.v4Seq != nil:
		return "v4:" + strconv.FormatInt(*m.v4Seq, 10)
	case m.messageID != "":
		return "message:" + strconv.Quote(m.messageID)
	case m.turnID != "":
		return "turn-id:" + strconv.Quote(m.turnID) + ":step:" + stepKey(m.step)
	case m.turn != nil:
		return "turn:" + strconv.FormatInt(*m.turn, 10) + ":step:" + stepKey(m.step)
	default:
		return fallback
	}
}

func contentKey(meta contentMeta, role, fallback string) string {
	return meta.baseKey(fallback) + "\x00" + role
}

func fillContentFields(m *Message, meta contentMeta) {
	m.Seq = meta.seq()
	m.V4Seq = meta.v4Seq
	m.MessageID = meta.messageID
	m.TurnID = meta.turnID
	m.Turn = meta.turn
	m.Step = meta.step
}

// Stream 归约一次助手回合的流式增量。Event.Seq 始终只归 Mux 水位/gap/replay；
// 这里使用的序号与身份全部来自 Event.Data。
type Stream struct {
	seq           int
	content       contentMeta
	hasTextMeta   bool
	hasContent    bool
	thinking      strings.Builder
	text          strings.Builder
	textRole      string
	done          bool
	seenReplay    map[string]struct{}
	streamID      uint64
	generation    uint64
	nextToolLocal int

	// tools 是这一段里真发生过的工具调用/结果，按发生顺序。
	tools []Message
}

var nextStreamScope atomic.Uint64

func (s *Stream) scope() string {
	if s.streamID == 0 {
		s.streamID = nextStreamScope.Add(1)
	}
	return "stream:" + strconv.FormatUint(s.streamID, 10) + ":generation:" + strconv.FormatUint(s.generation, 10)
}

// Seq 是本段内容的 v4 seq；未知或真实值为零时都返回 0。需要区分二者时看
// Messages()[].V4Seq。它不再是 Transport 水位。
func (s *Stream) Seq() int { return s.seq }

// ToolMessages 是这一段里发生过的工具消息（role=="tool"），按发生顺序返回副本。
func (s *Stream) ToolMessages() []Message {
	out := make([]Message, len(s.tools))
	copy(out, s.tools)
	return out
}

// Thinking 是已攒下的思考正文。
func (s *Stream) Thinking() string { return s.thinking.String() }

// Text 是已攒下的正文。
func (s *Stream) Text() string { return s.text.String() }

// Done 报告这一段是否已经说完（说完才把正文落成消息）。
func (s *Stream) Done() bool { return s.done }

// Reset 清空，准备下一段（**已经落进 msgs 的消息不受影响**）。
// tools 与局部序号也必须清：否则下一轮同 Tool+Args 会继承上一轮结果。
func (s *Stream) Reset() {
	s.seq = 0
	s.content = contentMeta{}
	s.hasTextMeta = false
	s.hasContent = false
	s.thinking.Reset()
	s.text.Reset()
	s.textRole = ""
	s.done = false
	s.seenReplay = nil
	s.tools = nil
	s.nextToolLocal = 0
	s.generation++
}

func (s *Stream) message(role, text string) Message {
	m := Message{Role: role, Text: text}
	fillContentFields(&m, s.content)
	m.identity = contentKey(s.content, role, s.scope())
	return m
}

func (s *Stream) toolScope(meta contentMeta) string {
	// 无 callId 的配对永远再套 Stream+Reset 代次；即使 turnId 相同，也不把
	// 这个局部配对规则提升成可跨轮、可跨进程使用的身份。
	return s.scope() + ":" + meta.baseKey("unscoped")
}

func (s *Stream) newTool(c Chunk) Message {
	meta := metaOf(c)
	m := Message{Role: "tool", Tool: c.Tool, Args: c.Args, CallID: c.CallID}
	fillContentFields(&m, meta)
	m.identity = contentKey(meta, "tool", s.scope())
	m.toolScope = s.toolScope(meta)
	if c.CallID != "" {
		m.toolIdentity = "call:" + strconv.Quote(c.CallID)
	} else {
		m.toolIdentity = m.toolScope + ":local:" + strconv.Itoa(s.nextToolLocal)
		s.nextToolLocal++
	}
	if c.result {
		m.Summary = c.Text
	}
	return m
}

func (s *Stream) updateToolMeta(m *Message, c Chunk) {
	meta := metaOf(c)
	// result 的 v4/message 身份对应 load 的 tool/result 投影，优先于 call；
	// result 先到时，后来的 call 不得反过来擦掉这个结果身份。
	strong := meta.v4Seq != nil || meta.messageID != ""
	currentStrong := m.V4Seq != nil || m.MessageID != ""
	if (c.result && (strong || !currentStrong)) || (!c.result && (m.Summary == "" || !currentStrong)) {
		m.Seq = meta.seq()
		m.V4Seq = meta.v4Seq
		m.MessageID = meta.messageID
		m.identity = contentKey(meta, "tool", s.scope())
	}
	if meta.turnID != "" || m.TurnID == "" {
		m.TurnID = meta.turnID
	}
	if meta.turn != nil || m.Turn == nil {
		m.Turn = meta.turn
	}
	if meta.step != nil || m.Step == nil {
		m.Step = meta.step
	}
	if c.CallID != "" {
		m.CallID = c.CallID
	}
}

func (s *Stream) applyTool(c Chunk) {
	if !c.result {
		if c.CallID != "" {
			for i := range s.tools {
				if s.tools[i].CallID == c.CallID {
					s.tools[i].Tool, s.tools[i].Args = c.Tool, c.Args
					s.updateToolMeta(&s.tools[i], c)
					return
				}
			}
		}
		s.tools = append(s.tools, s.newTool(c))
		return
	}

	if c.CallID != "" {
		for i := range s.tools {
			if s.tools[i].CallID == c.CallID {
				s.tools[i].Summary = c.Text
				s.updateToolMeta(&s.tools[i], c)
				return
			}
		}
		// result 先到也保留；后续同 callId 的 call 会补 Tool/Args。
		s.tools = append(s.tools, s.newTool(c))
		return
	}

	scope := s.toolScope(metaOf(c))
	for i := len(s.tools) - 1; i >= 0; i-- {
		m := &s.tools[i]
		if m.CallID == "" && m.Summary == "" && (m.Tool != "" || m.Args != "") && m.toolScope == scope {
			m.Summary = c.Text
			s.updateToolMeta(m, c)
			return
		}
	}
	// 无 callId 且结果先到/turn 不同：没有安全配对依据，独立保留结果，
	// 绝不挂到“最近一条”或另一轮调用上。
	s.tools = append(s.tools, s.newTool(c))
}

func replayKey(ev Event, chunks []Chunk) string {
	if len(chunks) == 0 {
		return ""
	}
	allFull := true
	for _, c := range chunks {
		allFull = allFull && c.Full
	}
	// v4 seq 是事件身份；messageId 只对完整消息做重放短路，不能误杀同 ID 的增量。
	if chunks[0].V4Seq == nil && !(allFull && chunks[0].MessageID != "") {
		return ""
	}
	identity := ""
	if chunks[0].V4Seq != nil {
		identity = "v4:" + strconv.FormatInt(*chunks[0].V4Seq, 10)
	} else {
		identity = "message:" + strconv.Quote(chunks[0].MessageID)
	}
	return ev.Kind + "\x00" + identity + "\x00" + string(ev.Data)
}

// Apply 吃一帧，返回状态是否变了（调用方只在变了时重绘）。
func (s *Stream) Apply(ev Event) bool {
	chunks := ev.Chunks()
	if key := replayKey(ev, chunks); key != "" {
		if _, seen := s.seenReplay[key]; seen {
			return false
		}
		if s.seenReplay == nil {
			s.seenReplay = make(map[string]struct{})
		}
		s.seenReplay[key] = struct{}{}
	}
	changed := false
	for _, c := range chunks {
		s.hasContent = true
		switch c.Block {
		case BlockReasoning, BlockText:
			meta := metaOf(c)
			if !s.hasTextMeta || meta.identified() {
				s.content = meta
				s.hasTextMeta = true
			}
			s.seq = s.content.seq()
			if c.Block == BlockReasoning {
				s.thinking.WriteString(c.Text)
			} else {
				if c.role != "" {
					s.textRole = c.role
				}
				s.text.WriteString(c.Text)
			}
			changed = true
		case BlockTool:
			s.applyTool(c)
			changed = true
		}
		if c.Full && c.Block != BlockTool {
			// 完整文本/思考消息可结束这一段；完整 tool/call 或 tool/result 不能，
			// 否则壳在 result 后立刻 Reset，后续 call/result 就没有配对上下文。
			s.done = true
			changed = true
		}
	}
	if ev.End() && s.hasContent && !s.done {
		s.done = true
		changed = true
	}
	return changed
}

// Messages 把当前积累落成消息：思考在前、正文在后；空的不产出。
//
// 正文只在 done 之后才产出——流式期间正文由界面上的流式气泡显示，
// 提前落一条只会让屏幕上有两份同样的字。
// **思考不一样：一有增量就要落**，否则"边想边看"就变成"想完才看见"。
func (s *Stream) Messages() []Message {
	var out []Message
	if th := s.thinking.String(); th != "" {
		out = append(out, s.message("thinking", th))
	}
	// 工具消息**不等 done**：tool/call 一到就要能画出来（用户能立刻看到"正在跑"），
	// 结果到了再补 Summary。等 done 的话，一个跑满几轮的长工具全程都是空白。
	out = append(out, s.tools...)
	if s.done {
		if tx := s.text.String(); tx != "" {
			role := s.textRole
			if role == "" || role == "tool" {
				role = "assistant"
			}
			out = append(out, s.message(role, tx))
		}
	}
	return out
}

// MergeMessages 把 fresh 并进 keep：**keep 里已有的不删**，相同内容身份的更新。
// 旧 load 投影只有 (Seq,Role)，仍按原规则兼容；流消息有更强的 data 身份时优先用它。
//
// 用途是"刷新不能吞历史"。`session.load` 给的是**尾巴**（tail），而界面手里
// 可能已经有更完整的一份——直接 `msgs = r.Messages` 会把尾巴之前的内容整段吃掉。
// 实测症状：切走再切回后，界面「共 2 条（总 2）」而宿主是 4 条，第一轮问答没了。
func MergeMessages(keep, fresh []Message) []Message {
	out := make([]Message, len(keep))
	copy(out, keep)
	for _, m := range fresh {
		out = upsertMsg(out, m)
	}
	return out
}

// Merge 把流状态并进消息列表。Event.Seq 不参与这里：v4 seq/messageId/turn+call
// 身份优先；完全没有真值时只用 Stream+Reset 代次的局部键防误覆盖。
func Merge(msgs []Message, s *Stream) []Message {
	for _, m := range s.Messages() {
		msgs = upsertMsg(msgs, m)
	}
	return msgs
}

func matchesV4Seq(m Message, seq int64) bool {
	if m.V4Seq != nil {
		return *m.V4Seq == seq
	}
	// 无 identity 的 Message 是旧 load/调用方字面量，其 Seq 本来就是 v4 域。
	return m.identity == "" && int64(m.Seq) == seq
}

// sameContent 双向兼容“旧 load 裸 Seq”与“流消息 V4Seq”，但局部 fallback
// 不会因为数值碰巧相同就退化成 Seq 匹配。
func sameContent(have, want Message) bool {
	if have.Role != want.Role {
		return false
	}
	if have.identity != "" && have.identity == want.identity {
		return true
	}
	if want.V4Seq != nil && matchesV4Seq(have, *want.V4Seq) {
		return true
	}
	if have.V4Seq != nil && want.identity == "" && *have.V4Seq == int64(want.Seq) {
		return true
	}
	if want.MessageID != "" && have.MessageID == want.MessageID {
		return true
	}
	if have.identity == "" && want.identity == "" {
		return have.Seq == want.Seq
	}
	return false
}

func sameToolContent(have, want Message) bool {
	// turn 只是配对作用域，不是一次工具调用的唯一身份；不同 callId/local
	// 调用即便同 turn、同 Tool+Args，也不能在内容合并阶段再次折叠。
	if want.V4Seq == nil && want.MessageID == "" && have.V4Seq == nil && have.MessageID == "" &&
		(want.identity != "" || have.identity != "") {
		return false
	}
	return sameContent(have, want)
}

func identityBase(key string) string {
	if i := strings.LastIndex(key, "\x00"); i >= 0 {
		return key[:i]
	}
	return key
}

func sameMessageScope(have, want Message) bool {
	if have.identity != "" && want.identity != "" && identityBase(have.identity) == identityBase(want.identity) {
		return true
	}
	if want.V4Seq != nil && matchesV4Seq(have, *want.V4Seq) {
		return true
	}
	return want.MessageID != "" && have.MessageID == want.MessageID
}

func insertMsgBefore(msgs []Message, at int, want Message) []Message {
	msgs = append(msgs, Message{})
	copy(msgs[at+1:], msgs[at:])
	msgs[at] = want
	return msgs
}

// upsertMsg 按内容身份更新或插入。工具首先按 callId，其次按本 Stream 的
// 配对键或 v4/message 身份；旧投影兼容键也含 v4 Seq，不全局匹同 Tool+Args。
func upsertMsg(msgs []Message, want Message) []Message {
	if want.Role == "tool" {
		for i := range msgs {
			if msgs[i].Role != "tool" {
				continue
			}
			if want.CallID != "" && msgs[i].CallID == want.CallID {
				msgs[i] = want
				return msgs
			}
			if want.toolIdentity != "" && msgs[i].toolIdentity == want.toolIdentity {
				msgs[i] = want
				return msgs
			}
		}
		for i := range msgs {
			if msgs[i].Role == "tool" && sameToolContent(msgs[i], want) {
				msgs[i] = want
				return msgs
			}
		}
		if want.identity == "" && want.toolIdentity == "" {
			for i := range msgs {
				if msgs[i].Role == "tool" && msgs[i].identity == "" && msgs[i].toolIdentity == "" &&
					msgs[i].Seq == want.Seq && msgs[i].Tool == want.Tool && msgs[i].Args == want.Args {
					msgs[i] = want
					return msgs
				}
			}
		}
		return append(msgs, want)
	}
	for i := range msgs {
		if sameContent(msgs[i], want) {
			msgs[i] = want
			return msgs
		}
	}
	// 新的思考块排在对应正文前面。没有真值的局部 fallback 只能兼容旧的
	// “已落正文、后补思考”调用方式；不能插到另一个 Stream 代次的正文前面。
	if want.Role == "thinking" {
		for i := range msgs {
			if msgs[i].Role == "assistant" && sameMessageScope(msgs[i], want) {
				return insertMsgBefore(msgs, i, want)
			}
		}
		if strings.HasPrefix(want.identity, "stream:") {
			for i := len(msgs) - 1; i >= 0; i-- {
				if msgs[i].Role == "assistant" && msgs[i].identity == "" {
					return insertMsgBefore(msgs, i, want)
				}
			}
		}
	}
	return append(msgs, want)
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
