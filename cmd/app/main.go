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
	"sync"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
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
	cache map[string]*cachedSession

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
)

func newConfig(m *model) app.Config {
	cfg := app.Config{Tree: &tree, Title: "dsh-ui", Width: 1180, Height: 800, Invalidator: &m.inv}
	cfg.TextInput = flex.Fields(&m.draft)

	cfg.Paint = func(canvas *ui.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)

		m.mu.Lock()
		sessions := append([]apiclient.Session(nil), m.sessions...)
		cur := m.cur
		msgs := append([]apiclient.Message(nil), m.msgs...)
		status := m.status
		if renderLimit > 0 && len(msgs) > renderLimit {
			status = fmt.Sprintf("%s（渲染末尾 %d / %d）", status, renderLimit, len(msgs))
			msgs = msgs[len(msgs)-renderLimit:]
		}
		m.mu.Unlock()

		m.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
			f.Row().Fill().Children(func() {

				// ── 左栏：会话列表 ──
				f.Column().Width(260).FillHeight().Pad(10).Gap(6).Border(1, theme.Border).Children(func() {
					f.Row().Gap(6).Items(flex.Center).Children(func() {
						f.Text("会话").FontSize(13).Bold()
						f.Spacer()
						if f.Button(btnNew, "新建").Small().Clicked() {
							m.refreshList()
						}
					})
					f.Divider()
					f.ScrollArea(sideScr).Grow(1).Gap(3).Children(func() {
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
					f.ScrollArea(msgScr).Grow(1).Gap(6).Children(func() {
						f.Column().Gap(6).Children(func() {
							if len(msgs) == 0 {
								f.Text("← 左边选一个会话").Muted()
							}
							for i := range msgs {
								msg := msgs[i]
								who := "你"
								if msg.Role != "user" {
									who = "助手"
								}
								f.Column().Pad(8).Radius(6).Border(1, theme.Border).Children(func() {
									f.Text(who).FontSize(11).Muted()
									f.Wrap(msg.Text).FontSize(14)
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

	}
	return cfg
}

func main() {
	addr := flag.String("addr", "http://127.0.0.1:7788", "Transport base URL")
	limit := flag.Int("limit", 0, "只渲染最后 N 条消息（0 = 全部）；用于量视口虚拟化的收益")
	sess := flag.String("session", "", "启动后直接打开这个会话 id")
	tail := flag.Int("tail", 200, "session.load 拉多少条尾巴")
	flag.Parse()
	renderLimit = *limit
	forceSession = *sess
	tailCount = *tail

	m := &model{cli: apiclient.New(*addr), status: "连接 " + *addr + " …", loading: true,
		cache: map[string]*cachedSession{}}
	m.refreshList()

	var canvas ui.Canvas
	if err := app.RunCanvas(newConfig(m), &canvas); err != nil {
		log.Fatal(err)
	}
}
