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
type serverFrame struct {
	Seq  int             `json:"seq"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
	Time int64           `json:"time,omitempty"`
}

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
			out <- Event{Kind: f.Type, Seq: 0, Topic: topic, Data: f.Data}
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
		// topic 用客户端订阅的那个：服务端帧不带 topic。
		out <- Event{Kind: f.Type, Seq: f.Seq, Topic: topic, Data: f.Data}
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
}

// packet 是 ev 帧 data 的联合形状。三种都真出现过：
//
//	{"delta":"…"} / {"done":true}                    增量流（mockd 逐字推、ACP）
//	{"content":[…]} / {"message":{"content":[…]}}    v4 原始事件（真宿主 follow 原样推）
//	{"tool":"bash","args":"…"}                       工具调用
//
// 只认一种的后果实测过：真宿主推的是 content 块，只读 delta 就等于
// **整条流一个字都没进界面**，而屏幕上看起来只是"没反应"。
type packet struct {
	Delta   string        `json:"delta"`
	Text    string        `json:"text"`
	Done    bool          `json:"done"`
	Tool    string        `json:"tool"`
	Args    string        `json:"args"`
	Content []packetBlock `json:"content"`
	// Role 用来区分"这条 content 是工具结果"还是普通正文。
	// 宿主的 tool/result 帧长这样：
	//   {"message":{"content":[{"type":"text","text":"[exit 0] …"}],
	//               "role":"tool","toolCallId":"call_…"}}
	// 不读 role 的话，工具输出会被当成 assistant 正文流进气泡里——
	// 界面上看到的是"回答里混着命令输出"，而不是一张工具卡。
	Role    string `json:"role"`
	Message *struct {
		Content []packetBlock `json:"content"`
		Role    string        `json:"role"`
	} `json:"message"`
}

// packetBlock 是 v4 content 数组里的一块。tool-call 块的字段名按 v4 原样。
type packetBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
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
			return []Chunk{{Block: BlockReasoning, Text: s}}
		}
		return nil

	case "assistant/text":
		if s := firstText(p.Delta, p.Text); s != "" {
			return []Chunk{{Block: BlockText, Text: s}}
		}
		return nil

	case "tool/call":
		if p.Tool != "" || p.Args != "" {
			return []Chunk{{Block: BlockTool, Tool: p.Tool, Args: p.Args}}
		}
		return nil

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
func (p packet) chunks() []Chunk {
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
			return []Chunk{{Block: BlockTool, Text: b.String(), Full: true}}
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
				out = append(out, Chunk{Block: BlockTool, Tool: b.Name, Args: b.Arguments, Full: true})
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

// Stream 归约一次助手回合的流式增量。
//
// 为什么需要它：流上是**帧**，消息流是**条**。reasoning 与 text 会交错到达，
// 而一条消息往往由几十条增量帧拼成。这里把增量攒成两条消息的原料：
// 思考一条（Role=thinking）、正文一条（Role=assistant），
// **两条都用这一段的第一帧 seq** —— 同一段的增量落在同一条上。
//
// 为什么不用每帧自己的 seq：真实语料里 reasoning:text ≈ 8:1，
// 一帧一条消息的话，一屏全是思考碎片，正文被挤到看不见的地方。
type Stream struct {
	seq      int
	thinking strings.Builder
	text     strings.Builder
	done     bool
	// tools 是这一段里真发生过的工具调用/结果，按发生顺序。
	//
	// 为什么必须在这里攒：工具卡要有 Tool/Args/结果三样才画得出来，
	// 而这三样分散在 tool/call 和 tool/result 两帧里。早先 case BlockTool
	// 是空的（"暂时不落消息"），于是**实时跑的工具完全不出现在右栏轨迹里**，
	// 只有下次 session.load 才带回来——用户看到的是"我明明跑了工具，轨迹是空的"。
	tools []Message
}

// Seq 是本段消息的 seq（0 = 还没有内容）。
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
func (s *Stream) Reset() {
	s.seq = 0
	s.thinking.Reset()
	s.text.Reset()
	s.done = false
}

// Apply 吃一帧，返回状态是否变了（调用方只在变了时重绘）。
func (s *Stream) Apply(ev Event) bool {
	chunks := ev.Chunks()
	changed := false
	if len(chunks) > 0 {
		if s.seq == 0 {
			s.seq = ev.Seq
		}
		for _, c := range chunks {
			if c.Full {
				// 完整的一条：这条消息的身份就是帧自己的 seq（与宿主的日志 seq 对齐，
				// 重连重放同一条时才能靠 seq 更新掉，而不是追加第二份）。
				s.seq = ev.Seq
				break
			}
		}
		for _, c := range chunks {
			switch c.Block {
			case BlockReasoning:
				s.thinking.WriteString(c.Text)
				changed = true
			case BlockText:
				s.text.WriteString(c.Text)
				changed = true
			case BlockTool:
				// tool/call 带 Tool+Args，tool/result 带 Text（见 packet.isToolResult）：
				// 前者开一条工具消息，后者把结果填到最近那条上。这样实时就能画工具卡，
				// 不必等下一次 session.load 才把轨迹带回来。
				switch {
				case c.Tool != "" || c.Args != "":
					s.tools = append(s.tools, Message{Role: "tool", Tool: c.Tool, Args: c.Args})
					changed = true
				case c.Text != "" && len(s.tools) > 0:
					s.tools[len(s.tools)-1].Summary = c.Text
					changed = true
				}
			}
			if c.Full {
				s.done = true
				changed = true
			}
		}
	}
	if ev.End() && s.seq != 0 && !s.done {
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
		out = append(out, Message{Seq: s.seq, Role: "thinking", Text: th})
	}
	// 工具消息**不等 done**：tool/call 一到就要能画出来（用户能立刻看到"正在跑"），
	// 结果到了再补 Summary。等 done 的话，一个跑满几轮的长工具全程都是空白。
	out = append(out, s.tools...)
	if s.done {
		if tx := s.text.String(); tx != "" {
			out = append(out, Message{Seq: s.seq, Role: "assistant", Text: tx})
		}
	}
	return out
}

// MergeMessages 把 fresh 并进 keep：**keep 里已有的不删**，同 (Seq,Role) 的更新。
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

// Merge 把流状态并进消息列表：同 seq 同角色**更新**，绝不重复追加。
//
// 为什么按 seq 去重：断线重连、同帧重放、以及"先流式落了一条、随后
// session.load 又给了同一条"这三条路都会让同一 seq 再来一次。
// 追加的话用户会看到两遍一模一样的内容；去重唯一可靠的事实就是宿主给的 seq。
func Merge(msgs []Message, s *Stream) []Message {
	for _, m := range s.Messages() {
		msgs = upsertMsg(msgs, m)
	}
	return msgs
}

// upsertMsg 按 (Seq, Role) 更新或插入一条消息。
//
// 工具消息是例外：同一轮里可能有多个工具调用，它们的 seq 都是这轮的 seq，
// 按 (Seq, Role) 认就会互相覆盖——所以 tool 按 (Tool, Args) 认身份。
func upsertMsg(msgs []Message, want Message) []Message {
	if want.Role == "tool" {
		for i := range msgs {
			if msgs[i].Role == "tool" && msgs[i].Tool == want.Tool && msgs[i].Args == want.Args {
				msgs[i] = want
				return msgs
			}
		}
		return append(msgs, want)
	}
	for i := range msgs {
		if msgs[i].Seq == want.Seq && msgs[i].Role == want.Role {
			msgs[i] = want
			return msgs
		}
	}
	// 新的思考块排在同 seq 的正文前面：先想后说。
	if want.Role == "thinking" {
		for i := range msgs {
			if msgs[i].Seq == want.Seq && msgs[i].Role == "assistant" {
				msgs = append(msgs, Message{})
				copy(msgs[i+1:], msgs[i:])
				msgs[i] = want
				return msgs
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
