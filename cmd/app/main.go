// dsh-ui 的最小界面：左栏会话、右栏消息、底部输入框。
//
// 数据全部来自 Transport 契约（mockd 或真宿主），界面不读任何文件、不碰会话日志。
//
//	go run ./cmd/app            # 默认连 http://127.0.0.1:7788
//	go run ./cmd/app -addr http://127.0.0.1:9000
package main

import (
	"context"
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
	btnSend = tree.ID(home, "send")
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
	view  flex.View
	draft flex.Field
	inv   app.Invalidator
	cli   *apiclient.Client
	cache      map[string]*cachedSession
	smokeFired bool

	// 每条助手消息一份 Markdown 文档 + 视图（按消息下标复用，避免每帧重建）
	mdDocs  []markdown.Document
	mdViews []components.MarkdownView
	mdText  []string

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
	m.mu.Lock()
	m.cur = id
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
	}
	if m.mdText[i] != text {
		m.mdText[i] = text
		m.mdDocs[i].Parse(text)
	}
	return m.mdViews[i].Layout(canvas, components.MarkdownStyle{Size: 14}, &m.mdDocs[i], width)
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

var (
	theme        = ui.DarkTheme()
	renderLimit  int    // 0 = 不限制
	forceSession string // 非空则启动后直接打开
	tailCount    int    // session.load 的 tailMessages
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
					})
					f.Divider()
					f.ScrollArea(sideScr).Grow(1).MinHeight(120).Gap(3).Children(func() {
						f.Column().Gap(3).Children(func() {
							for i := range sessions {
								s := sessions[i]
								id := tree.ID(home, "s-"+s.ID)
								mark := "   "
								if s.ID == cur {
									mark = "●  "
								}
								if f.Row().Width(228).PadX(8).PadY(6).Radius(5).
									Hit(id, s.Title).Children(func() {
									f.Text(mark + s.Title).FontSize(13)
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
					f.ScrollArea(scrID).Grow(1).MinHeight(160).Gap(6).Follow(true).Children(func() {
						f.Column().Gap(6).Children(func() {
							if len(msgs) == 0 {
								f.Text("← 左边选一个会话").Muted()
							}
							if streamText != "" {
								f.Column().Pad(8).Radius(6).Border(1, theme.Border).Children(func() {
									f.Row().Gap(6).Items(flex.Center).Children(func() {
										f.Text("助手").FontSize(11).Muted()
										f.Spinner()
									})
									f.Wrap(streamText).FontSize(14)
								})
							}
							for i := range msgs {
								msg := msgs[i]
								who := "你"
								if msg.Role != "user" {
									who = "助手"
								}
								f.Column().Pad(8).Radius(6).Border(1, theme.Border).Children(func() {
									f.Text(who).FontSize(11).Muted()
									if msg.Role == "user" {
										f.Wrap(msg.Text).FontSize(14)
									} else {
										// 助手消息走 MarkdownView：先算高度占位，Draw 之后再画
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

		// Draw 之后布局已定，Slot 的 Rect 才是最终位置
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
				Clip: ui.Rect{X: r.X, Y: bounds.Y, Width: r.Width, Height: bounds.Height},
			})
		}
	}
	return cfg
}

func main() {
	addr := flag.String("addr", "http://127.0.0.1:7788", "Transport base URL")
	limit := flag.Int("limit", 0, "只渲染最后 N 条消息（0 = 全部）；用于量视口虚拟化的收益")
	sess := flag.String("session", "", "启动后直接打开这个会话 id")
	tail := flag.Int("tail", 200, "session.load 拉多少条尾巴")
	smoke := flag.Bool("smoke", false, "启动后自动发一条消息，验证 send 链路（不做按键模拟）")
	flag.Parse()
	renderLimit = *limit
	forceSession = *sess
	tailCount = *tail
	smokeSend = *smoke

	m := &model{cli: apiclient.New(*addr), status: "连接 " + *addr + " …", loading: true,
		cache: map[string]*cachedSession{}, mux: apiclient.NewMux(*addr)}
	m.refreshList()

	var canvas ui.Canvas
	if err := app.RunCanvas(newConfig(m), &canvas); err != nil {
		log.Fatal(err)
	}
}
