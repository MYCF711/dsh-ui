// dsh-ui 的最小界面：左栏会话、右栏消息、底部输入框。
//
// 数据全部来自 Transport 契约（mockd 或真宿主），界面不读任何文件、不碰会话日志。
//
//	go run ./cmd/app            # 默认连 http://127.0.0.1:7788
//	go run ./cmd/app -addr http://127.0.0.1:9000
package main

import (
	"context"
	"math"
	"strings"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

var (
	tree = ui.Tree{}
	home = tree.Feature("home")

	btnNew  = tree.ID(home, "new")
	sideScr = tree.ID(home, "side")
	msgScr  = tree.ID(home, "msgs")
	draftID = tree.ID(home, "draft")
	btnSend  = tree.ID(home, "send")
	searchID = tree.ID(home, "search")
	btnAllow = tree.ID(home, "allow")
	btnDeny  = tree.ID(home, "deny")
	btnAlways = tree.ID(home, "always")
)

// cachedSession 是切换会话用的温缓存：切回来先显示它，再后台刷新。
// 上限 5 个（约 3 MB/1000 条），超了就淘汰最久没碰的那个。
type cachedSession struct {
	msgs  []apiclient.Message
	total int
	title string
	at    time.Time
}

const cacheLimit = 5

type model struct {
	view   flex.View
	draft  flex.Field
	search flex.Field // 侧栏过滤（按标题 + cwd 匹配）
	inv   app.Invalidator
	cli   *apiclient.Client
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

	// 流式：当前订阅的取消函数 + 正在拼接的助手文本（delta 累积）
	mux        *apiclient.Mux
	subCancel  context.CancelFunc
	streamText string
	lastSeq    int

	mu       sync.Mutex
	sessions []apiclient.Session
	cur      string
	msgs     []apiclient.Message
	status   string
	loading  bool
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
	hit := false
	if cs, ok := m.cache[id]; ok {
		m.msgs = cs.msgs
		m.status = fmt.Sprintf("%s · %d 条（总 %d）· 温缓存", cs.title, len(cs.msgs), cs.total)
		m.loading = false
		hit = true
	} else {
		m.msgs = nil
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
	m.cache[id] = &cachedSession{msgs: r.Messages, total: r.Total, title: r.Title, at: time.Now()}
	m.evictLocked(id)
	if m.cur == id { // 用户可能已经切走了
		m.msgs = r.Messages
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
		m.status = fmt.Sprintf("%s · %d 条（总 %d）", r.Title, len(r.Messages), r.Total)
		m.loading = false
	}
	fireSmoke := smokeSend && !m.smokeFired
	if fireSmoke {
		m.smokeFired = true
	}
	m.mu.Unlock()
	m.inv.Invalidate()
	if fireSmoke {
		m.draft.SetText("smoke-" + time.Now().Format("150405"))
		m.send()
	}
}

// layoutMarkdown 保证第 i 条消息的 Markdown 文档与视图就绪，返回高度。
func (m *model) layoutMarkdown(canvas *ui.Canvas, i int, text string, width float32) float32 {
	for len(m.mdDocs) <= i {
		m.mdDocs = append(m.mdDocs, markdown.Document{})
		m.mdViews = append(m.mdViews, components.MarkdownView{})
		m.mdText = append(m.mdText, "")
		m.mdWidth = append(m.mdWidth, -1)
		m.mdH = append(m.mdH, 0)
	}
	// MarkdownView.Layout 是昂贵的：只在文本或宽度变化时才重排（这是 111MB 那一版的教训）
	if m.mdText[i] != text || m.mdWidth[i] != width {
		m.mdText[i] = text
		m.mdWidth[i] = width
		m.mdDocs[i].Parse(text)
		m.mdH[i] = m.mdViews[i].Layout(canvas, components.MarkdownStyle{Size: 14}, &m.mdDocs[i], width)
	}
	return m.mdH[i]
}

// mdSlot 记录一条助手消息占位用的 Slot，等布局完成后在它的位置画 Markdown。
type mdSlot struct {
	idx int
	box flex.Box
}

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

// subscribe 订阅某个会话的流；切换会话时先取消旧订阅。
func (m *model) subscribe(id string) {
	m.mu.Lock()
	if m.subCancel != nil {
		m.subCancel()
		m.subCancel = nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.subCancel = cancel
	since := m.lastSeq
	m.mu.Unlock()

	go func() {
		ch, err := m.mux.Subscribe(ctx, "session:"+id, since)
		if err != nil {
			return // 流不可用不致命：界面仍能靠 session.load 工作
		}
		for ev := range ch {
			if ev.Kind == "gap" {
				// 断号：按契约去补这一段，绝不整体刷新
				m.refreshSession(id, true)
				continue
			}
			m.applyEvent(id, ev)
		}
	}()
}

// applyEvent 把一个 ev 帧吃进 model：delta 累积，done 落成一条消息。
func (m *model) applyEvent(id string, ev apiclient.Event) {
	var d struct {
		Delta string `json:"delta"`
		Done  bool   `json:"done"`
		Text  string `json:"text"`
	}
	_ = json.Unmarshal(ev.Data, &d)

	m.mu.Lock()
	if ev.Seq > m.lastSeq {
		m.lastSeq = ev.Seq
	}
	if d.Delta != "" {
		m.streamText += d.Delta
	}
	if d.Done {
		if m.streamText != "" {
			m.msgs = append(m.msgs, apiclient.Message{Seq: ev.Seq, Role: "assistant", Text: m.streamText})
		}
		m.streamText = ""
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

func (m *model) send() {
	text := m.draft.Text()
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
	m.draft.SetText("")
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
		m.setStatus("已发送 · turn " + turn)
	}()
}

// approvalReq 是一条待审批的工具调用。
type approvalReq struct {
	Tool   string
	Cmd    string // 完整命令（不截断——用户要看的就是这个）
	Reason string // 模型自述，界面必须标注"不可信"
}

// estimateMsgHeight 估算一条消息的高度。系数 0.62 来自 docs/virt.md 的实测反推
// （中英混排每行字数 ≈ 可用宽度 / (字号 × 0.62)）。视口外只靠它占位，不精确也没关系：
// 固定高度盒子保证累计位置可预测，滚动不会漂移。
func estimateMsgHeight(m apiclient.Message, width float32) float32 {
	const lineH = 22.0
	const gapPad = 28.0
	if width < 120 {
		width = 120
	}
	text := m.Text
	if m.Role == "tool" {
		return 70 // 工具卡是固定形状
	}
	perLine := width / (14 * 0.62)
	if perLine < 8 {
		perLine = 8
	}
	lines := 0
	for _, para := range strings.Split(text, "\n") {
		n := float64(len([]rune(para)))
		l := int(math.Ceil(n / float64(perLine)))
		if l < 1 {
			l = 1
		}
		lines += l
	}
	return float32(float64(lines)*lineH + gapPad)
}

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

var (
	theme        = ui.DarkTheme()
	renderLimit  int    // 0 = 不限制
	forceSession string // 非空则启动后直接打开
	tailCount    int    // session.load 的 tailMessages
	demoTool     bool   // 在消息列表里插假的工具卡
	smokeSend    bool   // 启动后自动发一条
)

func newConfig(m *model) app.Config {
	cfg := app.Config{Tree: &tree, Title: "dsh-ui", Width: 1180, Height: 800,
		MinWidth: 760, MinHeight: 520, Invalidator: &m.inv}
	cfg.TextInput = flex.Fields(&m.draft)

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
		if renderLimit > 0 && len(msgs) > renderLimit {
			status = fmt.Sprintf("%s（渲染末尾 %d / %d）", status, renderLimit, len(msgs))
			msgs = msgs[len(msgs)-renderLimit:]
		}
		m.mu.Unlock()

		var mdSlots []mdSlot
		m.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
			f.Row().Fill().Children(func() {

				// ── 左栏：会话列表 ──
				f.Column().Width(260).FillHeight().Pad(10).Gap(6).Border(1, theme.Border).Children(func() {
					f.Row().Gap(6).Items(flex.Center).Children(func() {
						f.Text("会话").FontSize(13).Bold()
						f.Spacer()
						if f.Button(btnNew, "新建").Small().Clicked() {
							m.newSession()
						}
						f.Field(&m.search, searchID, "搜索", "过滤会话…").Small().Grow(1)
					})
					f.Divider()
					f.ScrollArea(sideScr).Width(240).Grow(1).MinHeight(120).Gap(3).Children(func() {
						f.Column().Gap(3).Children(func() {
							q := strings.ToLower(strings.TrimSpace(m.search.Text()))
							shown := 0
							for i := range sessions {
								s := sessions[i]
								if q != "" && !strings.Contains(strings.ToLower(s.Title+" "+s.Cwd), q) {
									continue
								}
								shown++
								id := tree.ID(home, "s-"+s.ID)
								mark := "   "
								if s.ID == cur {
									mark = "●  "
								}
								// Hit 的标签留空：只当点击区用。
								// 传 s.Title 会让它自己也渲染一遍标签，和下面那行 Text 打架，
								// 结果是宽度算不出来、标题竖排成两三个字（实测踩过）。
								if f.Row().Width(236).PadX(8).PadY(6).Radius(5).
									Hit(id, "").Children(func() {
									f.Text(mark + s.Title).FontSize(13).Ellipsis()
								}).Clicked() {
									m.openSession(s.ID)
								}
							}
						})
					})
				})

				// ── 右栏：消息 + 输入 ──
				f.Column().Grow(1).FillHeight().Pad(12).Gap(8).Children(func() {
					f.Text(status).FontSize(12).Muted()
					f.Divider()
					// 每个会话用不同的 scroll id：否则切会话时滚动位置会串（viva flex/scroll.go:174）
					scrID := msgScr
					if cur != "" {
						scrID = tree.ID(home, "msgs-"+cur)
					}
					f.ScrollArea(scrID).Width(f.Bounds().Width-278).Grow(1).MinHeight(160).Gap(6).Follow(true).Clip().Children(func() {
						f.Column().Gap(6).Children(func() {
							if len(msgs) == 0 {
								f.Text("← 左边选一个会话").Muted()
							}
							if streamText != "" {
								f.Column().FillWidth().Pad(8).Radius(6).Border(1, theme.Border).Children(func() {
									f.Row().Gap(6).Items(flex.Center).Children(func() {
										f.Text("助手").FontSize(11).Muted()
										f.Spinner()
									})
									f.Wrap(streamText).FontSize(14)
								})
							}
							// 视口虚拟化（docs/virt.md）：先算高度和累计位置，
							// 视口外只放 Slot —— 不建 Text 就不生成 GPU 文本纹理。
							w0 := f.Bounds().Width - 300
							if w0 < 200 {
								w0 = 200
							}
							heights := make([]float32, len(msgs))
							offs := make([]float32, len(msgs))
							var acc float32
							for i := range msgs {
								heights[i] = estimateMsgHeight(msgs[i], float32(w0))
								offs[i] = acc
								acc += heights[i] + 6
							}
							var vpOffset, vpHeight float32
							if st := m.view.ScrollState(scrID); st != nil {
								vpOffset = st.Offset
							}
							vpHeight = f.Bounds().Height - 220
							if vpHeight < 240 {
								vpHeight = 240
							}
							const overscan = 800.0

							for i := range msgs {
								msg := msgs[i]
								if offs[i]+heights[i] < vpOffset-overscan || offs[i] > vpOffset+vpHeight+overscan {
									f.Slot().Height(heights[i])
									continue
								}
								who := "你"
								switch msg.Role {
								case "tool":
									who = "" // 工具卡自己带标题，不套「助手」
								case "assistant":
									who = "助手"
								}
								f.Column().FillWidth().Pad(8).Radius(6).Border(1, theme.Border).Children(func() {
									if who != "" {
										f.Text(who).FontSize(11).Muted()
									}
									if msg.Role == "tool" {
										// 工具调用卡片：名字 + 参数 + 一行结果摘要（不再套「助手」标签）
										f.Column().Pad(10).Radius(8).Background(theme.Muted).Children(func() {
											f.Row().Gap(8).Items(flex.Center).Children(func() {
												f.Text(msg.Tool).Mono().Bold().FontSize(13)
												f.Text(msg.Args).Mono().Muted().FontSize(12).Ellipsis()
											})
											if msg.Summary != "" {
												f.Text(msg.Summary).Muted().FontSize(12)
											}
										})
									} else if msg.Role == "user" {
										f.Wrap(msg.Text).FontSize(14)
									} else if i < len(msgs)-markdownBudget {
										// LOD：老消息只画纯文本。真实会话里有 200 条长消息，
										// 全上 Markdown 会让文档树常驻 + 首次排版 4 秒（实测 168.8 MB）。
										f.Wrap(msg.Text).FontSize(13).Muted()
									} else {
										// 最近的消息走 MarkdownView：先算高度占位，Draw 之后再画
										mi := i
										w := f.Bounds().Width - 300
										if w < 200 {
											w = 200
										}
										h := m.layoutMarkdown(canvas, mi, msg.Text, w)
										mdSlots = append(mdSlots, mdSlot{idx: mi, box: f.Slot().Height(h)})
									}
								})
							}
						})
					})
					if ap := approval; ap != nil {
						// 审批气泡：先命令、后模型自述（且标注不可信）——对齐宿主线的审批框规矩
						f.Column().Pad(12).Radius(8).Background(theme.Muted).Border(1, theme.Border).Gap(8).Children(func() {
							f.Row().Gap(8).Items(flex.Center).Children(func() {
								f.Text("需要审批").Bold().FontSize(13)
								f.Text(ap.Tool).Mono().Muted().FontSize(12)
								f.Spacer()
							})
							f.Text(ap.Cmd).Mono().FontSize(13)
							if ap.Reason != "" {
								f.Text("模型自述（不可信）：" + ap.Reason).Muted().FontSize(11)
							}
							f.Row().Gap(8).Justify(flex.End).Children(func() {
								if f.Button(btnDeny, "拒绝").Outline().Small().Clicked() {
									m.approval = nil
									m.setStatus("已拒绝")
								}
								if f.Button(btnAllow, "允许").Small().Clicked() {
									m.approval = nil
									m.setStatus("已允许")
								}
							})
						})
					}
					f.Row().Gap(8).Items(flex.Center).Children(func() {
						if f.Field(&m.draft, draftID, "输入", "说点什么…（Enter 发送）").Grow(1).Submitted() {
							m.send()
						}
						if f.Button(btnSend, "发送").Clicked() {
							m.send()
						}
					})
				})
			})
		})

		// 布局已定：恢复待恢复的滚动位置，再画 Markdown
		m.applyPendingScroll()
		look := components.MarkdownLookFor(theme)
		for _, slot := range mdSlots {
			if slot.idx >= len(m.mdViews) {
				continue
			}
			r := slot.box.Rect()
			if r.Width <= 0 || r.Height <= 0 {
				continue
			}
			m.mdViews[slot.idx].Paint(canvas, components.MarkdownPaint{
				Look: look,
				X:    r.X,
				Y:    r.Y,
				// Clip 的右边界要到窗口边：Markdown 里的表格/代码块会比卡片宽，
				// 只裁到 r.Width 会溢出画到侧栏上（实测过一次）。
				Clip: ui.Rect{X: r.X, Y: bounds.Y, Width: bounds.Width - r.X, Height: bounds.Height},
			})

		}
	}
	return cfg
}

func main() {
	addr := flag.String("addr", "http://127.0.0.1:7788", "Transport base URL")
	limit := flag.Int("limit", 0, "只渲染最后 N 条消息（0 = 全部）；用于量视口虚拟化的收益")
	sess := flag.String("session", "", "启动后直接打开这个会话 id")
	tail := flag.Int("tail", 40, "session.load 拉多少条尾巴（实测每条消息约 0.46 MB 纹理，40 条约 100 MB）")
	smoke := flag.Bool("smoke", false, "启动后自动发一条消息，验证 send 链路（不做按键模拟）")
	demo := flag.Bool("demo", false, "在消息列表里插一条假的工具卡，验证渲染")
	flag.Parse()
	renderLimit = *limit
	forceSession = *sess
	tailCount = *tail
	smokeSend = *smoke
	demoTool  = *demo

	m := &model{cli: apiclient.New(*addr), status: "连接 " + *addr + " …", loading: true,
		cache:      map[string]*cachedSession{},
		mux:        apiclient.NewMux(*addr),
		scrollPos:  map[string]float32{},
}
	m.refreshList()

	var canvas ui.Canvas
	if err := app.RunCanvas(newConfig(m), &canvas); err != nil {
		log.Fatal(err)
	}
}
