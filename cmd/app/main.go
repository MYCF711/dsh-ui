// dsh-ui 的最小界面：左栏会话、右栏消息、底部输入框。
//
// 数据全部来自 Transport 契约（mockd 或真宿主），界面不读任何文件、不碰会话日志。
//
//	go run ./cmd/app            # 默认连 http://127.0.0.1:7788
//	go run ./cmd/app -addr http://127.0.0.1:9000
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/internal/uiplugins"
	_ "github.com/MYCF711/dsh-ui/plugins/dscomposer"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	_ "github.com/MYCF711/dsh-ui/plugins/dsdecl"
	_ "github.com/MYCF711/dsh-ui/plugins/dsdemo"
	_ "github.com/MYCF711/dsh-ui/plugins/dsrightbar"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar"
	_ "github.com/MYCF711/dsh-ui/plugins/dsstatusbar"
	_ "github.com/MYCF711/dsh-ui/plugins/dstopbar"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

var (
	tree = ui.Tree{}
	home = tree.Feature("home")

	btnNew    = tree.ID(home, "new")
	sideScr   = tree.ID(home, "side")
	msgScr    = tree.ID(home, "msgs")
	draftID   = tree.ID(home, "draft")
	btnSend   = tree.ID(home, "send")
	searchID  = tree.ID(home, "search")
	btnAllow  = tree.ID(home, "allow")
	btnDeny   = tree.ID(home, "deny")
	btnAlways = tree.ID(home, "always")
)

// cachedSession 是切换会话用的温缓存：切回来先显示它，再后台刷新。
// 上限 5 个（约 3 MB/1000 条），超了就淘汰最久没碰的那个。
type cachedSession struct {
	msgs  []apiclient.Message
	total int
	title string
	usage apiclient.Usage
	// 宿主 §12 H1/H4 的字段跟 usage 一起缓存：切回来时先显示上一次的真值，
	// 而不是先掉回零值（那会让状态栏闪一下"没有数据"）。
	usageTotal    apiclient.Usage
	contextTokens int64
	model         string
	at            time.Time
}

const cacheLimit = 5

type model struct {
	view       flex.View
	draft      flex.Field
	search     flex.Field // 侧栏过滤（按标题 + cwd 匹配）
	inv        app.Invalidator
	cli        *apiclient.Client
	cache      map[string]*cachedSession
	smokeFired bool

	// 待审批的请求（v1 只支持一条；工具名 + 命令 + 模型自述）
	approval *approvalReq

	// 每条助手消息一份 Markdown 文档 + 视图（按消息下标复用，避免每帧重建）
	// 每个会话的滚动位置（只存 float32，不存 *ScrollState —— viva 要求"要的时候再取"）
	scrollPos     map[string]float32
	pendingScroll string

	mdDocs  []markdown.Document
	mdViews []components.MarkdownView
	mdText  []string
	mdWidth []float32 // 上次 Layout 用的宽度
	mdH     []float32 // 上次 Layout 的高度：文本和宽度都没变就不重新排版

	// 流式：每个会话一条独立订阅（H14：一个会话一条 WS 连接）。
	// **切会话不退订**：后台会话的连接照收，只是不前台渲染。
	mux  *apiclient.Mux
	subs map[string]*muxSub
	// seqs 是**每会话**已消费到哪一段（切走不丢）。
	//
	// 为什么必须独立存：subs 里那份会在切会话时被删掉（为的是重订重放），
	// 删了水位就没了——实测切回已看过的会话时 since=0，等于从头重放一遍，
	// 与"按水位续、不整页刷新"的契约相反。
	seqs map[string]int
	// epochs 是每会话的 journal 世代（从 session.load 的 journalEpoch 拿）。
	// 订阅时带上它，宿主换代了就会发 session/reset——不带的话旧水位会静默错位。
	epochs     map[string]int64
	streamText string
	// turnBusy：当前会话有 turn 在跑。发 prompt 置 true，流结束/取消/失败置 false。
	turnBusy bool
	// lastSeq 是**当前会话**已收到的最大 seq。per-session 的水位在 muxSub.lastSeq，
	// 这里保留一份是为了"打开会话时给 since"和审批帧用，不要拿它当全局水位。
	lastSeq int
	// stream 归约当前这一回合的流式块（text / reasoning 交错到达）。
	// 为什么交给 apiclient：事件名与 content 块的形状是**线上契约**，
	// 归约规则（同 seq 更新、思考先于正文）也要在能被单测的地方。
	stream apiclient.Stream

	// ── 插件运行时（界面线的「一切皆插件」）──
	// 壳只做三件事：装配插件、给槽位、把快照递进去。
	// 部件怎么分组、怎么排序、展开还是收起，全是插件自己的事。
	kernel  *uikernel.Kernel
	theme   *pluginapi.Theme
	prims   pluginapi.Primitives
	side    *dssidebar.Plugin
	actions pluginapi.Actions

	mu       sync.Mutex
	sessions []apiclient.Session
	cur      string
	msgs     []apiclient.Message
	usage    apiclient.Usage
	// 宿主 §12 H1/H4/H5：累计用量、当前上下文、模型名、审批策略。
	// 零值/空串 = 不知道，界面据此整段不画（绝不拿 0 或 mode 冒充）。
	usageTotal    apiclient.Usage
	contextTokens int64
	modelName     string
	permission    string
	// showRightbar 是右栏开合（壳持有，插件只读）。
	showRightbar bool
	status       string
	loading      bool
}

func (m *model) setStatus(s string) {
	m.mu.Lock()
	m.status = s
	m.loading = false
	m.mu.Unlock()
	m.inv.Invalidate()
}

func (m *model) refreshList() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ss, err := m.cli.List(ctx)
		if err != nil {
			m.setStatus("列会话失败: " + err.Error())
			return
		}
		m.mu.Lock()
		m.sessions = ss
		m.status = fmt.Sprintf("%d 个会话", len(ss))
		m.loading = false
		empty := m.cur == ""
		m.mu.Unlock()
		m.inv.Invalidate()
		if empty && len(ss) > 0 {
			id := ss[0].ID
			for i := range ss {
				if forceSession != "" && ss[i].ID == forceSession {
					id = forceSession
				}
			}
			if forceSession != "" && id != forceSession {
				id = forceSession // mockd 里可能不在列表前部
			}
			m.openSession(id)
		}
	}()
}

func (m *model) openSession(id string) {
	// 切走之前记下当前会话滚到哪了
	m.mu.Lock()
	prev := m.cur
	m.mu.Unlock()
	m.saveScroll(prev)

	// 切会话必须丢掉上一个会话的 Markdown 文档树，否则切三次就是三份 200 条的
	// Layout 结果全常驻（实测 168.7 MB）
	m.mdDocs = m.mdDocs[:0]
	m.mdViews = m.mdViews[:0]
	m.mdText = m.mdText[:0]
	m.mdWidth = m.mdWidth[:0]
	m.mdH = m.mdH[:0]

	m.mu.Lock()
	m.cur = id
	m.pendingScroll = id
	// 水位跟着会话走。不复位就会拿上一个会话的 seq 去判连续性：
	// 切到 B 时 lastSeq 还是 A 的，B 的第一条事件要么被当成"断号"、
	// 要么被 `ev.Seq > lastSeq` 直接丢掉——这是"多会话串流"的真正成因。
	m.lastSeq = m.seqs[id]
	// 切回来时把该会话的旧订阅掐掉重订一次：后台期间的事件没被消费，
	// 只有"带着切走时的水位重新 sub"才能让宿主把它们重放出来。
	// 水位已经在上一行取到 m.lastSeq 里，删 sub 不会丢它。
	if sub := m.subs[id]; sub != nil {
		if sub.cancel != nil {
			sub.cancel()
		}
		delete(m.subs, id)
	}
	// 待审批也属于具体会话：切走时清掉，绝不能在 B 的界面上批准 A 的工具。
	// （代价：切回来要等宿主重推该请求；宿主 120s 无应答会自己拒，不会永远挂着。）
	m.approval = nil
	hit := false
	if cs, ok := m.cache[id]; ok {
		m.msgs = cs.msgs
		m.usage = cs.usage
		m.usageTotal = cs.usageTotal
		m.contextTokens = cs.contextTokens
		m.modelName = cs.model
		m.status = fmt.Sprintf("温缓存 · %d 条（总 %d）", len(cs.msgs), cs.total)
		m.loading = false
		hit = true
	} else {
		// 换成没缓存的会话时必须把上一份用量清掉：留着它就成了"这个会话的
		// 数据"，而实际是上一个会话的——比不显示严重得多。
		m.msgs = nil
		m.usage = apiclient.Usage{}
		m.usageTotal = apiclient.Usage{}
		m.contextTokens = 0
		m.modelName = ""
		m.loading = true
		m.status = "加载 " + id + " …"
	}
	m.mu.Unlock()
	m.inv.Invalidate()
	m.subscribe(id)
	if hit {
		// 有缓存就立刻可看，后台静默刷新
		go m.refreshSession(id, true)
		return
	}
	go m.refreshSession(id, false)
}

// refreshSession 拉一次 session.load；quiet=true 时失败不覆盖状态栏。
func (m *model) refreshSession(id string, quiet bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, err := m.cli.Load(ctx, id, tailCount)
	if err != nil {
		if !quiet {
			m.setStatus("加载失败: " + err.Error())
		}
		return
	}
	m.mu.Lock()
	m.cache[id] = &cachedSession{msgs: r.Messages, total: r.Total, title: r.Title, usage: r.Usage,
		usageTotal: r.UsageTotal, contextTokens: r.ContextTokens, model: r.Model, at: time.Now()}
	m.evictLocked(id)
	if m.cur == id { // 用户可能已经切走了
		m.msgs = r.Messages
		m.usage = r.Usage
		// H1/H4/H5：真宿主给的字段原样带进快照；mockd 不给时是零值，
		// 状态栏/顶栏据此整段不画。H1 的两个量是两件事，各自独立存。
		m.usageTotal = r.UsageTotal
		m.contextTokens = r.ContextTokens
		m.modelName = r.Model
		// 水位基线：epoch 用来核对世代，lastSeq 是宿主那边已经到哪了。
		if r.JournalEpoch > 0 {
			m.epochs[id] = r.JournalEpoch
		}
		// permission 是**进程级**的审批策略（不是会话 mode），每次 load 都刷新。
		m.permission = r.Permission
		if demoTool && len(m.msgs) > 0 {
			// 注意：这里已经在 m.mu 内（refreshSession 的锁），不能再 Lock —— 会自死锁
			m.approval = &approvalReq{Tool: "bash", Cmd: "git status --short",
				Reason: "想看一下当前工作区有没有未提交的改动"}
			m.msgs = append(m.msgs,
				apiclient.Message{Seq: -2, Role: "tool", Tool: "bash",
					Args:    "git status --short",
					Summary: "exit 0 · 3 files changed · 用了 12 ms"},
				apiclient.Message{Seq: -3, Role: "tool", Tool: "read",
					Args:    "internal/apiclient/mux.go",
					Summary: "1.2 KB · 78 行 · 读完了"},
				// 带 129 字符单行代码的助手消息：验横向滚动（清单 3.3）
				apiclient.Message{Seq: -4, Role: "assistant", Text: demoWideCode})
		}
		m.status = fmt.Sprintf("已连接 · %d 条（总 %d）", len(r.Messages), r.Total)
		m.loading = false
	}
	fireSmoke := smokeSend && !m.smokeFired
	if fireSmoke {
		m.smokeFired = true
	}
	m.mu.Unlock()
	m.inv.Invalidate()
	if fireSmoke {
		// -prompt 给了就用它：验工具卡/审批这类链路时，光发 "smoke-时间戳"
		// 是触发不到工具调用的。
		text := promptText
		if text == "" {
			text = "smoke-" + time.Now().Format("150405")
		}
		m.draft.SetText(text)
		m.send()
	}
}

// layoutMarkdown 保证第 i 条消息的 Markdown 文档与视图就绪，返回高度。

// mdSlot 记录一条助手消息占位用的 Slot，等布局完成后在它的位置画 Markdown。

// newSession 建一个会话并直接打开。
func (m *model) newSession() {
	m.setStatus("新建会话…")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		id, err := m.cli.Create(ctx, "", "default", "")
		if err != nil {
			m.setStatus("新建失败: " + err.Error())
			return
		}

		// 订阅时带上 journal 世代（H14）：宿主换代后发 session/reset，界面据重建基线。
		m.mux.EpochOf = func(topic string) int64 {
			id := strings.TrimPrefix(topic, "session:")
			m.mu.Lock()
			e := m.epochs[id]
			m.mu.Unlock()
			return e
		}
		m.refreshList()

		m.openSession(id)
	}()
}

// saveScroll 记下某个会话当前的滚动偏移。
func (m *model) saveScroll(id string) {
	if id == "" {
		return
	}
	st := m.view.ScrollState(tree.ID(home, "msgs-"+id))
	if st == nil {
		return
	}
	m.mu.Lock()
	if m.scrollPos == nil {
		m.scrollPos = map[string]float32{}
	}
	m.scrollPos[id] = st.Offset
	m.mu.Unlock()
}

// applyPendingScroll 在某次布局之后把记住的偏移恢复回去（只做一次）。
func (m *model) applyPendingScroll() {
	m.mu.Lock()
	id := m.pendingScroll
	m.pendingScroll = ""
	off, ok := m.scrollPos[id]
	m.mu.Unlock()
	if !ok {
		return
	}
	st := m.view.ScrollState(tree.ID(home, "msgs-"+id))
	if st != nil {
		st.ScrollTo(off)
	}
}

// subscribe 给**每个会话**建一条独立订阅（H14：一个会话一条 WS 连接）。
//
// 三条纪律，都是踩出来的：
//
//  1. **切会话不退订**。以前 `subscribe` 一上来就 cancel 旧 ctx，等于切走就把后台
//     会话的流掐了——模型还在跑、事件全丢，切回来只能整页重载。现在按会话保存订阅，
//     切走只是不再"前台渲染"，连接照收、seq 照记。
//  2. **水位 per-session**。`lastSeq` 以前是全局字段：切到 B 之后还带着 A 的水位，
//     断号检测立刻错乱（甚至把 B 的事件当成 A 的续号）。现在水位挂在会话上。
//  3. **断号只重订、不整页刷新**（宿主 §11.6）。整页 load 会闪、会丢滚动位置；
//     正确做法是带着 lastSeq 重新 sub，宿主从水位之后继续推。
func (m *model) subscribe(id string) {
	if id == "" {
		return
	}
	m.mu.Lock()
	if sub, ok := m.subs[id]; ok && sub.cancel != nil {
		m.mu.Unlock()
		return // 已经在订了
	}
	ctx, cancel := context.WithCancel(context.Background())
	// 用 m.lastSeq（openSession 刚按目标会话设过），而不是 subs 里那份——
	// 切回来时旧 sub 已被删掉，水位只在 m.lastSeq 里。
	sub := &muxSub{cancel: cancel, lastSeq: m.lastSeq}
	if m.subs == nil {
		m.subs = map[string]*muxSub{}
	}
	m.subs[id] = sub
	since := sub.lastSeq
	m.mu.Unlock()

	go m.runSubscription(ctx, id, since)
}

// muxSub 是一个会话的订阅状态。
type muxSub struct {
	cancel  context.CancelFunc
	lastSeq int
}

// runSubscription 跑一条会话的流，断了就带水位重连。
//
// 重连不做退避上限之外的花活：1s→2s→4s→…封顶 30s，ctx 取消就退出。
// 目的只有一个——**别让用户在切回来时才发现流早就断了**。
func (m *model) runSubscription(ctx context.Context, id string, since int) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		ch, err := m.mux.Subscribe(ctx, "session:"+id, since)
		if err != nil {
			log.Printf("mux 订阅失败（%v），%s 后重试 session:%s", err, backoff, id)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		m.mu.Lock()
		ep := m.epochs[id]
		m.mu.Unlock()
		log.Printf("mux 已订阅 session:%s since=%d epoch=%d", id, since, ep)

		for ev := range ch {
			if ev.Kind == "gap" {
				// 断号：**只上报，不重连**。
				//
				// 以前这里是 break 出去重订，结果无限循环：重订用的水位还是旧的，
				// 宿主从同一位置推同样的东西、客户端又报同样的 gap —— 实测日志 4 秒
				// 内重连三次、永远收不到后面的内容。
				//
				// 正确做法就是契约里写的：断号是**补洞**的信号，补洞走
				// session.history（只补缺失区间），跟重连是两件事。这里先如实记下来，
				// 继续消费后续事件（readLoop 已经把水位推进到断号后的 seq）。
				if ev.Gap != nil {
					log.Printf("mux 断号 session:%s [%d,%d]，用 session.history 补这一段", id, ev.Gap.FromSeq, ev.Gap.ToSeq)
					// 真补洞：只补缺失区间，补回来的帧直接喂 applyEvent
					// （宿主 history 返回的就是 journal 原始帧，与 WS 同形状）。
					//
					// 补之前把水位压回到缺口起点——否则补回来的帧会被
					// `ev.Seq > lastSeq` 当成过期的丢掉（那样就等于白补）。
					from, to := ev.Gap.FromSeq, ev.Gap.ToSeq
					if from > 0 && to >= from {
						m.mu.Lock()
						if m.seqs[id] >= from {
							m.seqs[id] = from - 1
						}
						if id == m.cur {
							m.lastSeq = from - 1
						}
						m.mu.Unlock()
						n, err := apiclient.FillGap(ctx, m.cli, id, from, to, func(f apiclient.Frame) {
							m.applyEvent(id, apiclient.Event{Kind: f.Type, Seq: f.Seq, Topic: "session:" + id, Data: f.Data})
						})
						if err != nil {
							// 409 stale_epoch 之类：补不回来就别硬补，记下来（退回 load 也要用户看得见）。
							log.Printf("history 补洞失败 session:%s [%d,%d]: %v（缺口保留，不整页刷新）", id, from, to, err)
						} else {
							log.Printf("history 补洞完成 session:%s 补回 %d 帧", id, n)
						}
					}
				}
				continue
			}
			if ev.Kind == "session/reset" {
				// 宿主说"你在旧世代上"：丢掉旧水位，重新 load 拿新基线。
				// **不清空已有消息**——先把新基线拉回来，界面再整体替换，
				// 这样用户看到的是"刷新了一下"，而不是"内容突然没了"。
				log.Printf("mux session/reset session:%s（journal 换代，重建基线）", id)
				m.mu.Lock()
				delete(m.seqs, id)
				delete(m.epochs, id)
				m.mu.Unlock()
				m.refreshSession(id, true)
				break
			}
			m.applyEvent(id, ev)
			// 水位只在**前台**会话上前进：后台会话不消费事件（见 applyEvent），
			// 若这里也前进水位，切回来时就"跳过了没消费的那一段"——字会永久丢。
			m.mu.Lock()
			if id == m.cur {
				if sub := m.subs[id]; sub != nil && ev.Seq > sub.lastSeq {
					sub.lastSeq = ev.Seq
				}
				if ev.Seq > m.seqs[id] {
					m.seqs[id] = ev.Seq // 水位独立存：切走删 subs 也不会丢
				}
				since = ev.Seq
			}
			m.mu.Unlock()
		}
		// channel 关闭 = 连接断了：带水位重连，而不是退回"下次点开再 load"。
		if ctx.Err() != nil {
			return
		}
		log.Printf("mux 连接断开 session:%s，带水位 %d 重连", id, since)
	}
}

// lastSeqOfLocked 取某会话已收到的最大 seq（没有就是 0）。
func (m *model) lastSeqOfLocked(id string) int {
	if sub := m.subs[id]; sub != nil {
		return sub.lastSeq
	}
	return 0
}

// applyEvent 把一个 ev 帧吃进 model：增量攒进流状态，按 seq 落成消息。
//
// 这里**不再只认 delta/done/text**：真宿主 follow 推的是 v4 原始事件
// （`assistant/message` 带 `content` 块，思考块 type=reasoning），
// 只读 delta 就等于整条流一个字都进不来。分类规则在 apiclient.Chunks 里，
// 它认识 delta / text / content 块三种形状，并且**不翻译事件名**。
// cancelTurn 打断当前会话正在跑的 turn。
//
// 失败要显示出来：用户点了停止却什么都没发生，比没有停止键更糟。
func (m *model) cancelTurn() {
	m.mu.Lock()
	id := m.cur
	m.mu.Unlock()
	if id == "" {
		m.setStatus("没有选中的会话，无法取消")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := m.cli.Cancel(ctx, id); err != nil {
			log.Printf("cancel 失败 id=%s: %v", id, err)
			m.setStatus("取消失败：" + err.Error())
			return
		}
		log.Printf("cancel 已发送 id=%s", id)
		m.mu.Lock()
		m.turnBusy = false
		m.mu.Unlock()
		m.setStatus("已取消")
	}()
}

// approve 走 H13 新接口：requestID 与 optionID 都由插件原样给出。
//
// 与 decide 的分工：decide 是"老接口+按 kind 找选项"，approve 是"插件已经选好了"。
// 两条路都要做同一件事——**只有宿主 200 且 accepted 才收气泡**。
func (m *model) approve(requestID, optionID string) {
	if requestID == "" || optionID == "" {
		m.setStatus("审批失败：请求缺少 requestId/optionId")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := m.cli.Approve(ctx, requestID, optionID); err != nil {
			log.Printf("approve 失败 requestId=%s option=%s: %v", requestID, optionID, err)
			m.setStatus("审批没送达：" + err.Error())
			return
		}
		log.Printf("approve 成功 requestId=%s option=%s", requestID, optionID)
		m.mu.Lock()
		if m.approval != nil && m.approval.RequestID == requestID {
			m.approval = nil
		}
		m.mu.Unlock()
		m.inv.Invalidate()
		m.setStatus("已审批（" + optionID + "）")
	}()
}

// approvalOptions 把 apiclient 的选项转成契约包的类型（两边都不 import 对方）。
func approvalOptions(in []apiclient.ApprovalOption) []pluginapi.ApprovalOption {
	if len(in) == 0 {
		return nil
	}
	out := make([]pluginapi.ApprovalOption, 0, len(in))
	for _, o := range in {
		out = append(out, pluginapi.ApprovalOption{
			OptionID: o.OptionID,
			Name:     o.Name,
			Kind:     o.Kind,
		})
	}
	return out
}

// decide 把一个审批决定原样回传宿主。
//
// 三种失败都要让用户看见，不许静默：
//   - 没有 RequestID（帧外来源）→ 直接拒绝，不能编一个 id 发过去；
//   - 宿主没给这个 kind 的选项 → 报"宿主没给这个选项"，不退回写死的 optionId；
//   - 网络/HTTP 失败 → 气泡留着、状态栏报错，可以再点。
//
// 成功才把气泡收掉：**本地气泡消失不等于已批准**。
func (m *model) decide(kind string) {
	m.mu.Lock()
	req := m.approval
	m.mu.Unlock()
	if req == nil {
		return
	}
	if req.RequestID == "" {
		m.setStatus("审批失败：这条待审批没有 requestId（不是真宿主发来的帧）")
		return
	}
	optID := ""
	for _, o := range req.Options {
		if o.Kind == kind {
			optID = o.OptionID
			break
		}
	}
	if optID == "" {
		m.setStatus("审批失败：宿主没给 " + kind + " 这个选项")
		return
	}
	label := map[string]string{"allow_once": "已允许", "allow_always": "本会话已允许", "reject_once": "已拒绝"}[kind]
	if label == "" {
		label = "已" + kind
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := m.cli.Approve(ctx, req.RequestID, optID); err != nil {
			log.Printf("approve 失败 requestId=%s option=%s: %v", req.RequestID, optID, err)
			m.setStatus("审批没送达：" + err.Error())
			return
		}
		log.Printf("approve 成功 requestId=%s option=%s", req.RequestID, optID)
		m.mu.Lock()
		// 只收掉**同一条**：期间可能已经来了新的审批请求。
		if m.approval != nil && m.approval.RequestID == req.RequestID {
			m.approval = nil
		}
		m.mu.Unlock()
		m.inv.Invalidate()
		m.setStatus(label)
	}()
}

// maxInt 只给 seq 比较用（stdlib 没 generic max，为一行引入泛型不值）。
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m *model) applyEvent(id string, ev apiclient.Event) {
	// 审批帧不是流事件，不能进 stream.Apply（那会把 tool-call 之类混进正文）。
	// 它只对**当前会话**有意义：后台会话的审批等切回去再显示，
	// 否则用户会在 A 的界面上批准 B 的工具——线上事故级别的错误。
	if ev.Kind == "approval/request" {
		req, err := apiclient.ParseApprovalRequest(ev.Data)
		if err != nil {
			log.Printf("审批帧解析失败（忽略）：%v", err)
			return
		}
		m.mu.Lock()
		isCur := id == m.cur
		if isCur {
			m.approval = &approvalReq{
				RequestID: req.RequestID,
				Tool:      req.Tool(),
				Title:     req.ToolCall.Title,
				Kind:      req.ToolCall.Kind,
				Args:      req.Args(),
				Cmd:       req.Command(),
				Detail:    req.Detail(),
				Options:   req.Options,
			}
			m.lastSeq = maxInt(m.lastSeq, ev.Seq)
		}
		m.mu.Unlock()
		if isCur {
			log.Printf("待审批 requestId=%s tool=%s options=%d", req.RequestID, req.Tool(), len(req.Options))
			m.inv.Invalidate()
		}
		return
	}

	m.mu.Lock()
	// **后台会话的帧不能写前台视图**。
	//
	// 以前切走就退订，所以这条没暴露；现在不退订了（方向是对的），
	// 若还往 m.msgs / m.stream 里 merge，B 的字就会出现在 A 的列表里——
	// 这就是"多会话串流"的真正落点，而且 A 的 cache 会被 B 的内容污染。
	//
	// 后台只做一件事：什么都不做（水位也不前进，见 runSubscription）。
	// 切回来时用旧水位重订，宿主把这段时间的事件重放一遍——用时间换正确。
	if id != m.cur {
		m.mu.Unlock()
		return
	}
	if ev.Seq > m.lastSeq {
		m.lastSeq = ev.Seq
	}
	if m.stream.Apply(ev) {
		// 思考一有增量就落（边想边看）；正文等这段说完才落（流式期间由气泡显示）。
		m.msgs = apiclient.Merge(m.msgs, &m.stream)
		if streamDebug > 0 && m.stream.Thinking() != "" && m.stream.Text() == "" {
			streamDebug--
			log.Printf("mux 思考增量（累计 %d 字）", len([]rune(m.stream.Thinking())))
		}
		if m.stream.Done() {
			m.streamText = ""
			m.stream.Reset()
			m.turnBusy = false
		} else {
			m.streamText = m.stream.Text()
		}
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}

// evictLocked 在持有 mu 时淘汰最旧的缓存，保护当前会话。
func (m *model) evictLocked(keep string) {
	for len(m.cache) > cacheLimit {
		oldest, oldestID := time.Now(), ""
		for id, cs := range m.cache {
			if id == keep || id == m.cur {
				continue
			}
			if cs.at.Before(oldest) {
				oldest, oldestID = cs.at, id
			}
		}
		if oldestID == "" {
			return
		}
		delete(m.cache, oldestID)
	}
}

// send 是旧的入口：从壳自己的输入框发。插件化之后输入框归 ds-composer，
// 它走 sendText；这里留着是给 -smoke 和键盘快捷键用。
func (m *model) send() { m.sendText(m.draft.Text()) }

// retryLast 重发最后一条用户消息（消息级「重试」）。
//
// 不新造一条：Prompt 本来就是"以这条为准再走一轮"，界面侧只负责把它再发一遍。
func (m *model) retryLast() {
	m.mu.Lock()
	var last string
	for i := len(m.msgs) - 1; i >= 0; i-- {
		if m.msgs[i].Role == "user" && m.msgs[i].Text != "" {
			last = m.msgs[i].Text
			break
		}
	}
	m.mu.Unlock()
	if last == "" {
		m.setStatus("没有可重试的用户消息")
		return
	}
	m.sendText(last)
}

func (m *model) sendText(text string) {
	if text == "" {
		return
	}
	m.mu.Lock()
	cur := m.cur
	m.mu.Unlock()
	if cur == "" {
		m.setStatus("先选一个会话")
		return
	}
	if m.draft.Text() == text {
		m.draft.SetText("")
	}
	m.msgs = append(m.msgs, apiclient.Message{Seq: -1, Role: "user", Text: text})
	m.inv.Invalidate()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		turn, err := m.cli.Prompt(ctx, cur, text)
		if err != nil {
			m.setStatus("发送失败: " + err.Error())
			return
		}
		m.mu.Lock()
		m.turnBusy = true // 从现在起"有 turn 在跑"，停止键/ Esc 才有意义
		m.mu.Unlock()
		m.setStatus("已发送 · turn " + turn)
	}()
}

// approvalReq 是一条待审批的工具调用。
type approvalReq struct {
	// RequestID 是宿主给的请求标识，回传 session.approve 时必须**原样**带上。
	// 空串表示这条待审批不是从真实帧来的（老的 demo 路径），此时按钮要报错而不是假装成功。
	RequestID string
	Tool      string // toolCall.title 冒号前那段；解析不出来就空（不猜）
	Title     string // toolCall.title 原文（单行摘要）
	Kind      string // execute / edit / other；不是风险等级
	Args      string // toolCall.rawInput 原文（JSON）
	Cmd       string // 兼容字段：老路径只有这个（bash 的完整命令）
	Detail    string // toolCall.content[].content.text
	Reason    string // 模型自述，界面必须标注"不可信"
	// Options 是宿主给的可选项，**原样**保留：不自己造 optionId，也不改名字。
	Options []apiclient.ApprovalOption
}

// estimateMsgHeight 估算一条消息的高度。系数 0.62 来自 docs/virt.md 的实测反推
// （中英混排每行字数 ≈ 可用宽度 / (字号 × 0.62)）。视口外只靠它占位，不精确也没关系：
// 固定高度盒子保证累计位置可预测，滚动不会漂移。

// markdownBudget 是"最近的多少条消息用 Markdown 渲染"，其余降级成纯文本。
// 依据：真实会话 442 条、tail=200 时全量 Markdown 要 168.8 MB / 首排 4 s。
const markdownBudget = 30

// newline 是换行符。写成常量而不是字面量 "\n"：这个仓库里已经有三次
// 通过 heredoc / python 拼 Go 源码时把 "\n" 变成真换行，编译直接红。
// 用一个名字，就再也不会踩。
const newline = "\n"

// demoWideCode 是一条围栏代码块里带 129 字符单行的消息，用来验横向滚动（清单 3.3）。
const demoWideCode = "超宽代码块验证（这一行 129 个字符，卡片放不下）：\n\n" +
	"```go\n" +
	"func veryLongFunctionName(alpha int, beta string, gamma float64, delta []byte, epsilon map[string]any) (int, error) {\n" +
	"	return alpha, nil\n" +
	"}\n" +
	"```\n\n" +
	"上面那行应该能横向滚动，而不是被硬切。"

// streamDebug 只打前几条增量，用来证明"是流出来的、不是重新 load 出来的"。
var streamDebug = 3

var (
	winTitle     string
	shotPath     string
	theme        ui.Theme // 由 dscore 的令牌算出来，启动时赋值（见 main）
	renderLimit  int      // 0 = 不限制
	forceSession string   // 非空则启动后直接打开
	tailCount    int      // session.load 的 tailMessages
	demoTool     bool     // 在消息列表里插假的工具卡
	smokeSend    bool     // 启动后自动发一条
	promptText   string   // -prompt 覆盖 smoke 文本
)

// drawSlot 把一个槽位上的插件按序画出来。
//
// 槽位空着就什么都不画——壳不需要知道哪个槽有人挂，也就不会因为少装一个
// 插件而画出一块空白边框。
// drawSlot 画一个槽位。
//
// 壳里只剩这一句：槽位树（谁在谁里面）由插件自己声明（Painter.Children），
// 壳不知道有几层、也不知道哪一层是什么部件——这就是「高塔」。
func drawSlot(f *flex.Frame, k *uikernel.Kernel, slot pluginapi.Slot, env *pluginapi.Env) {
	k.DrawSlot(f, slot, env)
}

// 窗口初始尺寸：默认 1180x800，-size 可改（只为无头截图量几何用）。
var (
	winWidth  = 1180
	winHeight = 800
)

// parseWinSize 解析 "WxH"。只认一个 'x'（大小写都行，也接受 '×'），
// 不做宽容解析：这个开关只给无头截图用，写错就报错，别猜。
func parseWinSize(s string) (int, int, bool) {
	i := strings.IndexAny(s, "xX×")
	if i <= 0 || i >= len(s)-1 {
		return 0, 0, false
	}
	w, err1 := strconv.Atoi(strings.TrimSpace(s[:i]))
	h, err2 := strconv.Atoi(strings.TrimSpace(s[i+1:]))
	if err1 != nil || err2 != nil || w < 320 || h < 240 {
		return 0, 0, false
	}
	return w, h, true
}

func newConfig(m *model) app.Config {
	cfg := app.Config{Tree: &tree, Title: winTitle, Width: winWidth, Height: winHeight,
		MinWidth: 760, MinHeight: 520, Invalidator: &m.inv}
	// 插件的输入框也要登记，否则收不到键盘（viva 只把登记过的 Field 当输入焦点候选）。
	// 走一次统一的聚合：以后再加插件不用改这里。
	fields := []*flex.Field{&m.draft}
	for _, p := range m.kernel.Plugins() {
		if withFields, ok := p.(pluginapi.Fields); ok {
			fields = append(fields, withFields.Fields()...)
		}
	}
	cfg.TextInput = flex.Fields(fields...)

	cfg.Paint = func(canvas *ui.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)

		m.mu.Lock()
		sessions := append([]apiclient.Session(nil), m.sessions...)
		cur := m.cur
		msgs := append([]apiclient.Message(nil), m.msgs...)
		status := m.status
		streamText := m.streamText
		approval := m.approval
		usage := m.usage
		// H1/H4/H5：会话语义的真值原样带进快照，插件自己决定画不画。
		usageTotal := m.usageTotal
		contextTokens := m.contextTokens
		modelName := m.modelName
		permission := m.permission
		showRightbar := m.showRightbar
		if renderLimit > 0 && len(msgs) > renderLimit {
			status = fmt.Sprintf("%s（渲染末尾 %d / %d）", status, renderLimit, len(msgs))
			msgs = msgs[len(msgs)-renderLimit:]
		}
		m.mu.Unlock()

		// 主区宽度：壳算好交给插件，插件不该猜（侧栏与右栏的宽度只有壳知道）。
		//
		// 右栏**不能写死 300**：真实宽度是 geomRightWidth 算出来的（这个尺寸下
		// 1180 宽的窗口右栏占 500），写死 300 会让插件以为主列比实际宽 200px，
		// 于是按 600 排版、真主列只有 ~495，内容被卡片硬裁（省略号都不画）。
		// 症状是审批条/长参数的末尾直接被切掉。
		hasRightbar := showRightbar && m.kernel.Has(pluginapi.SlotRightbar)
		var rightbarW float32
		if hasRightbar {
			rightbarW = geomRightWidth(bounds.Width, m.side.Width(), hasRightbar)
		}
		mainWidth := bounds.Width - m.side.Width() - rightbarW
		if mainWidth < 320 {
			mainWidth = 320
		}

		// 一帧一份快照：锁内拷好，插件里不用加锁，也就没法在绘制期发请求。
		m.mu.Lock()
		busy := m.turnBusy
		m.mu.Unlock()
		approvalView := (*pluginapi.Approval)(nil)
		if approval != nil {
			// **九个字段一个都不能少**：早先只传了 Tool/Cmd/Reason，结果
			// requestId 和 options 全丢，插件只能显示「宿主没给 requestId，
			// 这次审批回传不出去」——按钮点了也发不出去（真实 WS 帧里明明有）。
			approvalView = &pluginapi.Approval{
				RequestID: approval.RequestID,
				Tool:      approval.Tool,
				Title:     approval.Title,
				Kind:      approval.Kind,
				Args:      approval.Args,
				Cmd:       approval.Cmd,
				Detail:    approval.Detail,
				Reason:    approval.Reason,
				Options:   approvalOptions(approval.Options),
			}
		}
		// 调试胶水（3 行）：-demo-approval 时用假审批截图；默认返回 nil，正式路径不看它。
		if approvalView == nil {
			approvalView = demoApprovalView()
		}
		env := &pluginapi.Env{
			Theme: m.theme,
			Data: pluginapi.Snapshot{
				Sessions: sessions, Cur: cur, Status: status,
				Msgs: msgs, Total: len(msgs), Stream: streamText,
				Usage: pluginapi.Usage{
					InputTokens:     usage.InputTokens,
					OutputTokens:    usage.OutputTokens,
					TotalTokens:     usage.TotalTokens,
					CacheReadTokens: usage.CacheReadTokens,
					TokPerSec:       usage.TokPerSec,
					HasRate:         usage.HasRate,
				},
				// 宿主 §12 H1：累计与上下文是两个量，分开传，谁也不顶替谁。
				UsageTotal: pluginapi.Usage{
					InputTokens:     usageTotal.InputTokens,
					OutputTokens:    usageTotal.OutputTokens,
					TotalTokens:     usageTotal.TotalTokens,
					CacheReadTokens: usageTotal.CacheReadTokens,
					TokPerSec:       usageTotal.TokPerSec,
					HasRate:         usageTotal.HasRate,
				},
				ContextTokens: contextTokens,
				Model:         modelName,
				Permission:    permission,
				Approval:      approvalView,
				Busy:          busy,
				ShowRightbar:  showRightbar,
			},
			Act:       m.actions,
			Tree:      &tree,
			Feat:      home,
			Canvas:    canvas,
			Bounds:    bounds,
			MainWidth: mainWidth,
			// viva 要求滚动状态"要的时候再取"，所以传函数而不是指针。
			Scroll: m.view.ScrollState,
		}
		env.Has = func(slot pluginapi.Slot) bool { return m.kernel.Has(slot) }
		// 三栏几何由壳算：它知道侧栏要的宽度、右栏开不开、白名单里有没有底部整行。
		// 框架插件照着摆，不再自己猜（猜错过：白名单里漏掉 shell，部件就上下堆叠）。
		env.Geometry = pluginapi.Geometry{
			Width:        bounds.Width,
			Height:       bounds.Height,
			SidebarWidth: m.side.Width(),
			RightWidth:   geomRightWidth(bounds.Width, m.side.Width(), hasRightbar),
			MainWidth:    mainWidth,
			BottomHeight: condF(m.kernel.Has(pluginapi.SlotShellBottom), 24, 0),
		}

		m.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
			// 壳只画一个槽：三栏怎么切、底栏多高、右栏占多少，全在 plugins/dsshell 里。
			drawSlot(f, m.kernel, pluginapi.SlotShellRoot, env)
		})

		// 布局已定：先恢复待恢复的滚动位置，再跑插件的 AfterDraw
		// （插件负责 Markdown 这类"先布局后覆画"的东西，壳不再画内容）。
		m.applyPendingScroll()
		for _, painter := range m.kernel.AllPainters() {
			if painter.AfterDraw != nil {
				painter.AfterDraw(canvas, env)
			}
		}
	}
	// 窗口变了必须全屏重绘：Viva 只重画变化区域，缩小后旧帧会残留在边缘。
	// 键盘：先给插件（↑↓ 切会话、Esc 拒审批、Ctrl+Shift+C 复制…），
	// 插件都不要才算壳的。壳里因此**没有任何具体快捷键的判断**。
	cfg.OnKey = func(ev ui.KeyEvent) ui.EventResult {
		if ev.Release {
			return ui.EventResult{}
		}
		if m.kernel.HandleKey(ev) {
			return ui.EventResult{Handled: true, Repaint: true}
		}
		return ui.EventResult{}
	}

	cfg.OnResize = func(ui.Size) { m.inv.Invalidate() }
	return cfg
}

// parseKeys 把 "down,up" 翻成 ui.Key（只认导航键，够验证路由用）。
func parseKeys(seq string) []ui.Key {
	var out []ui.Key
	for _, name := range strings.Split(seq, ",") {
		switch strings.TrimSpace(strings.ToLower(name)) {
		case "down":
			out = append(out, ui.KeyDown)
		case "up":
			out = append(out, ui.KeyUp)
		case "home":
			out = append(out, ui.KeyHome)
		case "end":
			out = append(out, ui.KeyEnd)
		case "esc":
			out = append(out, ui.KeyEscape)
		case "enter":
			// 审批条可见时 Enter = 允许（dscomposer 的 HandleKey 只在可见时接管）。
			// 加它不是为了别的：验「允许之后宿主真的执行工具」必须有办法按下允许。
			out = append(out, ui.KeyEnter)
		}
	}
	return out
}

// fixedFirst 返回必须常驻的结构性插件。
//
// 白名单模式下列没列它们也要装：ds-core 是所有插件的 Requires，ds-shell 负责切三栏。
// 这一步直接在壳里实例化，因为白名单可能把它们漏掉，而它们是地基不是部件。
func fixedFirst(configPath string) []pluginapi.Plugin {
	// 无论哪个模式都要带上：skip 把它们从注册表结果里排掉了（壳自己注入实例）。
	// 全装模式下漏掉 ds-core，所有需要 ui.theme 的插件都会装配失败（实测）。
	return []pluginapi.Plugin{&dscore.Plugin{Dark: true}, &dsshell.Plugin{}}
}

// geomRightWidth 算右栏宽度：官方首次打开占视口 45%，扣完中栏最小 400 还不够就压到 300。
func geomRightWidth(total, sidebar float32, has bool) float32 {
	if !has {
		return 0
	}
	w := total * 0.45
	if total-w-sidebar < 400 {
		w = total - sidebar - 400
	}
	if w < 300 {
		w = 300
	}
	return w
}

// condF 是三目（Go 没有，写个小函数比每处写 if 清楚）。
func condF(ok bool, a, b float32) float32 {
	if ok {
		return a
	}
	return b
}

// orderPlugins 把框架插件排到最前。
//
// 装配顺序本身由 Requires 拓扑排序决定，但 ds-shell 与别的部件之间**没有依赖关系**
// （它只是画槽位，不许 Requires 那些部件），所以白名单里它可能排在后面。
// 排在后面时，部件会先占满整个窗口，框架再切三栏就晚了。
func orderPlugins(ps []pluginapi.Plugin) []pluginapi.Plugin {
	out := make([]pluginapi.Plugin, 0, len(ps))
	for _, p := range ps {
		if p.ID() == "ds-shell" {
			out = append(out, p)
		}
	}
	for _, p := range ps {
		if p.ID() != "ds-shell" {
			out = append(out, p)
		}
	}
	return out
}

// itoaLocal 只给状态文字用（壳里没有别的地方需要它）。
func itoaLocal(n int) string {
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

func main() {
	pluginConfig := flag.String("plugins", "", "插件配置文件（JSON 白名单；空=全装）")
	addr := flag.String("addr", "http://127.0.0.1:7788", "Transport base URL")
	limit := flag.Int("limit", 0, "只渲染最后 N 条消息（0 = 全部）；用于量视口虚拟化的收益")
	sess := flag.String("session", "", "启动后直接打开这个会话 id")
	tail := flag.Int("tail", 40, "session.load 拉多少条尾巴（实测每条消息约 0.46 MB 纹理，40 条约 100 MB）")
	smoke := flag.Bool("smoke", false, "启动后自动发一条消息，验证 send 链路（不做按键模拟）")
	prompt := flag.String("prompt", "", "调试：-smoke 时发这条文本（默认 smoke-<时间>）；验工具卡/审批用")
	cookie := flag.String("cookie", "", "调试：直接用这条会话 cookie（形如 dshd_session=…），跳过 /connect。token 一次性时用来与 curl 共用同一个宿主")
	demo := flag.Bool("demo", false, "在消息列表里插一条假的工具卡，验证渲染")
	title := flag.String("title", "dsh-ui", "窗口标题（并行跑多个实例时用来区分）")
	keysSeq := flag.String("keys", "", "调试：启动后依次注入按键（down,up,home,end,esc,enter），走的是插件的 HandleKey，用来验证键盘路由")
	expandThink := flag.Bool("expand-think", false, "调试：把思考块默认展开（截展开态用）")
	collapseAll := flag.Bool("collapse", false, "调试：启动时把所有工作区分组收起（截折叠态用）")
	// -keys-after：按键在启动后多少秒注入。审批这类场景要等模型先响应完
	// （真宿主上从 prompt 到 approval/request 大约 3–5 秒），2.5 秒会按早了。
	keysAfter := flag.Int("keys-after", 3, "调试：-keys 在启动后多少秒开始注入")
	// -switch：按顺序切会话（逗号分隔的 id），每 switchEvery 秒切一个。
	// 为什么要这个开关：侧栏在无头环境点不到，而"进程内切会话"和"进程重启"是
	// 两种语义（后者每次都新建 model，前者要复用 mux/cache/stream 等状态）——
	// 只有真在同一个进程里切，才能验出"切走时流没断、切回来内容还在"。
	switchSeq := flag.String("switch", "", "调试：启动后按顺序切换会话（逗号分隔 id）")
	switchEvery := flag.Int("switch-every", 3, "调试：-switch 每隔多少秒切一个")
	rightbarOn := flag.Bool("rightbar", true, "启动时是否展开右栏（-rightbar=false 收起来）")
	token := flag.String("token", "", "宿主 launch token：给了就先 /connect 换 cookie（真宿主 dshd serve 必须给）")
	searchAtStart := flag.String("search", "", "调试：启动就把会话内搜索打开，并带上这个词（截搜索态用）")
	shotWait := flag.Int("shot-wait", 4, "无头抓图前等多少秒（流式/发送类验证要给大一点）")
	shot := flag.String("shot", "", "抓一张窗口图存到这个路径然后退出（窗口会挪到屏幕外，不弹给用户）")
	winSize := flag.String("size", "", "调试：窗口初始尺寸 WxH（如 1600x900）；空 = 1180x800。验证几何在不同宽度下是否成立")
	flag.Parse()
	if *winSize != "" {
		w, h, ok := parseWinSize(*winSize)
		if !ok {
			log.Fatalf("-size 要写成 WxH（宽>=320、高>=240），收到 %q", *winSize)
		}
		winWidth, winHeight = w, h
	}
	renderLimit = *limit
	forceSession = *sess
	tailCount = *tail
	smokeSend = *smoke
	promptText = *prompt
	demoTool = *demo
	winTitle = *title
	shotPath = *shot

	m := &model{cli: apiclient.New(*addr), status: "连接 " + *addr + " …", loading: true,
		cache:        map[string]*cachedSession{},
		seqs:         map[string]int{},
		epochs:       map[string]int64{},
		mux:          apiclient.NewMux(*addr),
		scrollPos:    map[string]float32{},
		showRightbar: *rightbarOn,
	}
	// **先换 cookie，再拉数据**。
	// 反过来（先 refreshList 再 Connect）的后果实测过：第一次 list 必然 401，
	// 界面停在"0 个会话 / 列会话失败: unauthorized"，而 /connect 本身是成功的——
	// 看日志完全像认证失败，实际只是顺序错了。
	//
	// 真宿主（dshd serve）要求 cookie；mockd 不需要，所以只在给了 -token 时做。
	if *cookie != "" {
		// 调试路径：token 是一次性的，先 curl /connect 换好 cookie 再传进来，
		// 这样界面与 curl 能同时打同一个宿主（多会话/审批要两路并发验证）。
		name, value, ok := strings.Cut(*cookie, "=")
		if !ok || name == "" || value == "" {
			log.Fatalf("-cookie 要写成 name=value，收到 %q", *cookie)
		}
		cks := []*http.Cookie{{Name: name, Value: value, Path: "/"}}
		m.cli.SetCookies(cks)
		m.mux.Cookies = cks
		log.Printf("已注入会话 cookie（%s）", name)
	} else if *token != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := m.cli.Connect(ctx, *token)
		cancel()
		if err != nil {
			log.Fatalf("/connect 失败: %v", err)
		}
		// WS 与 HTTP 同一套认证：cookie 也要给流客户端，否则 /ws 401。
		m.mux.Cookies = m.cli.Cookies()
		log.Print("已用 launch token 换到 cookie")
	}
	m.refreshList()

	// 装插件：先根本依赖（ds-core：设计令牌 + 基础组件），再依赖它的部件。
	// 装配器按 Requires 排序；ds-sidebar 少一个键就直接报错，不会静默少画一块。
	m.side = &dssidebar.Plugin{CollapseAll: *collapseAll}
	// ds-core（令牌 + 图元）与 ds-shell（三栏）是结构性的：它们不在白名单里也要装，
	// 否则其余部件没有令牌可 Inject、或者各自独占整个窗口。
	// 这两条不许被 ui-plugins.json 关掉——配置管的是"装哪些部件"，不是"要不要有地基"。
	m.kernel = uikernel.New("dsh-ui")

	// 插件从哪来：**壳不认识任何具体插件**。
	// 内置的自己在 init 里 pluginapi.Register 报到（blank import 即接入）；
	// 装哪些可以由 <home>/ui-plugins.json 限量（白名单 / enabled:false）。
	// 需要壳注入运行期状态的少数几个（侧栏实例、搜索开关）在这里补。
	plugins, err := uiplugins.Load(*pluginConfig, pluginapi.Builtin(), func(id string) bool {
		// 这两个实例由壳自己 append（壳要读它们的 Fields/Width/搜索开关）。
		return id == "ds-sidebar" || id == "ds-conversation" || id == "ds-core" || id == "ds-shell"
	})
	if err != nil {
		log.Fatalf("插件来源加载失败: %v", err)
	}
	plugins = append(plugins, m.side, &dsconversation.Plugin{
		ExpandAllThinking: *expandThink,
		SearchOpen:        *searchAtStart != "",
		SearchQuery:       *searchAtStart,
	})
	// 白名单模式下，cs-shell 必须永远是第一个：它负责切三栏。
	// 少了它，其余部件会各自独占整个窗口（实测：白名单里漏掉 ds-shell，
	// 侧栏和消息流就上下堆在一起了）。
	plugins = append(plugins, fixedFirst(*pluginConfig)...)
	if err := m.kernel.Load(orderPlugins(plugins)...); err != nil {
		log.Fatalf("插件装配失败: %v", err)
	}
	m.theme = m.kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	m.prims = m.kernel.MustGet(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	theme = dscore.VivaTheme(m.theme)
	m.actions = pluginapi.Actions{
		OpenSession: m.openSession,
		NewSession:  m.newSession,
		Send:        m.sendText,
		Copy: func(text string) {
			if err := app.SetClipboardText(text); err != nil {
				m.setStatus("复制失败: " + err.Error())
				return
			}
			m.setStatus("已复制 " + itoaLocal(len([]rune(text))) + " 字")
		},
		Notify: m.setStatus,
		ToggleRightbar: func() {
			m.mu.Lock()
			m.showRightbar = !m.showRightbar
			on := m.showRightbar
			m.mu.Unlock()
			if on {
				m.setStatus("右栏：显示")
			} else {
				m.setStatus("右栏：收起")
			}
		},
		// 审批：真调 session.approve，把宿主的 requestId 与它自己的 optionId 原样回传。
		// **只有宿主 200 且 accepted 才收气泡**——本地收掉不等于批准了（宿主 §11.7）。
		// 失败时气泡留着、状态栏报错，用户可以再点一次。
		// H13 新接口：插件把**用户点中的那个 optionId** 原样交回来（可能是
		// allow_once / allow_always / reject_once，也可能是宿主以后加的新选项）。
		// 壳不再自己按 kind 猜——猜就等于替用户做了决定。
		Approve: func(requestID, optionID string) { m.approve(requestID, optionID) },
		// 旧接口保留：还没接 Approve 的插件走这里，按 kind 找选项。
		Allow: func() { m.decide("allow_once") },
		Deny:  func() { m.decide("reject_once") },
		// 取消：用户点停止键时打断当前会话的 turn。
		Cancel:   m.cancelTurn,
		SetDraft: func(text string) { m.draft.SetText(text) },
		Retry:    m.retryLast,
	}
	log.Printf("插件 %v，主题 %s", m.kernel.Order(), m.theme.Name)

	if *switchSeq != "" {
		go func(seq string, every time.Duration) {
			ids := strings.Split(seq, ",")
			time.Sleep(2 * time.Second) // 等首个会话加载完
			for _, id := range ids {
				id = strings.TrimSpace(id)
				if id == "" {
					continue
				}
				log.Printf("switch → %s", id)
				m.openSession(id)
				time.Sleep(every)
			}
			m.mu.Lock()
			log.Printf("switch 结束后当前会话: %s", m.cur)
			m.mu.Unlock()
		}(*switchSeq, time.Duration(*switchEvery)*time.Second)
	}

	if *keysSeq != "" {
		go func(seq string) {
			time.Sleep(time.Duration(*keysAfter) * time.Second) // 等会话列表/消息/审批帧到位
			for _, name := range parseKeys(seq) {
				ok := m.kernel.HandleKey(ui.KeyEvent{Key: name})
				log.Printf("key %v handled=%v", name, ok)
				time.Sleep(350 * time.Millisecond)
			}
			m.mu.Lock()
			log.Printf("注入后当前会话: %s", m.cur)
			m.mu.Unlock()
		}(*keysSeq)
	}

	if shotPath != "" {
		startShot(shotPath, time.Duration(*shotWait)*time.Second)
	}

	var canvas ui.Canvas
	if err := app.RunCanvas(newConfig(m), &canvas); err != nil {
		log.Fatal(err)
	}
}
