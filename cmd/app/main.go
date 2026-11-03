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
	"strings"
	"sync"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscomposer"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsrightbar"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar"
	"github.com/MYCF711/dsh-ui/plugins/dsstatusbar"
	"github.com/MYCF711/dsh-ui/plugins/dstopbar"
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
	at    time.Time
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

	// 流式：当前订阅的取消函数 + 正在拼接的助手文本（delta 累积）
	mux        *apiclient.Mux
	subCancel  context.CancelFunc
	streamText string
	lastSeq    int

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
	hit := false
	if cs, ok := m.cache[id]; ok {
		m.msgs = cs.msgs
		m.usage = cs.usage
		m.status = fmt.Sprintf("温缓存 · %d 条（总 %d）", len(cs.msgs), cs.total)
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
	m.cache[id] = &cachedSession{msgs: r.Messages, total: r.Total, title: r.Title, usage: r.Usage, at: time.Now()}
	m.evictLocked(id)
	if m.cur == id { // 用户可能已经切走了
		m.msgs = r.Messages
		m.usage = r.Usage
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
		m.draft.SetText("smoke-" + time.Now().Format("150405"))
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
	winTitle     string
	shotPath     string
	theme        ui.Theme // 由 dscore 的令牌算出来，启动时赋值（见 main）
	renderLimit  int      // 0 = 不限制
	forceSession string   // 非空则启动后直接打开
	tailCount    int      // session.load 的 tailMessages
	demoTool     bool     // 在消息列表里插假的工具卡
	smokeSend    bool     // 启动后自动发一条
)

// drawSlot 把一个槽位上的插件按序画出来。
//
// 槽位空着就什么都不画——壳不需要知道哪个槽有人挂，也就不会因为少装一个
// 插件而画出一块空白边框。
func drawSlot(f *flex.Frame, k *uikernel.Kernel, slot pluginapi.Slot, env *pluginapi.Env) {
	for _, p := range k.Painters(slot) {
		p.Draw(f, env)
	}
}

func newConfig(m *model) app.Config {
	cfg := app.Config{Tree: &tree, Title: winTitle, Width: 1180, Height: 800,
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
		showRightbar := m.showRightbar
		if renderLimit > 0 && len(msgs) > renderLimit {
			status = fmt.Sprintf("%s（渲染末尾 %d / %d）", status, renderLimit, len(msgs))
			msgs = msgs[len(msgs)-renderLimit:]
		}
		m.mu.Unlock()

		// 主区宽度：壳算好交给插件，插件不该猜（侧栏 280 + 右栏 300 只有壳知道）。
		const rightbarW = 300.0
		hasRightbar := showRightbar && m.kernel.Has(pluginapi.SlotRightbar)
		mainWidth := bounds.Width - m.side.Width()
		if hasRightbar {
			mainWidth -= rightbarW
		}
		if mainWidth < 320 {
			mainWidth = 320
		}

		// 一帧一份快照：锁内拷好，插件里不用加锁，也就没法在绘制期发请求。
		approvalView := (*pluginapi.Approval)(nil)
		if approval != nil {
			approvalView = &pluginapi.Approval{Tool: approval.Tool, Cmd: approval.Cmd, Reason: approval.Reason}
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
				Approval:     approvalView,
				ShowRightbar: showRightbar,
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

		m.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
			f.Column().Fill().Children(func() {
				// 主体用 Grow(1) 而不是 Fill：Fill 会把状态栏那 24px 挤成 0。
				f.Row().FillWidth().Grow(1).Children(func() {

					// ── 左栏：会话列表（插件 ds-sidebar 挂上来的）──
					// 壳只给列宽和底色。分组、排序、折叠、点击行为全在 plugins/dssidebar 里，
					// 这就是「一个部件一个插件」：换侧栏不用动壳。
					f.Column().Width(m.side.Width()).FillHeight().Pad(12).Gap(8).
						Background(m.theme.SidebarFill).Children(func() {
						drawSlot(f, m.kernel, pluginapi.SlotSidebarHeader, env)
						m.prims.Divider(f)
						f.Column().Grow(1).MinHeight(120).Children(func() {
							drawSlot(f, m.kernel, pluginapi.SlotSidebarList, env)
						})
					})

					// ── 右栏：主区的每一块都是插件挂上来的 ──
					// 头部 / 消息流 / 输入区各占一个槽；槽空着就整块不出现。
					// 宽度要钉死：插件里用百分比定宽时，父级宽度必须是确定的，
					// 否则 % 会按整窗解析，内容被推到窗口外（踩过：气泡跑到右上角）。
					// 右栏只在有插件挂上来时才占位置（官方也是可以收起的）。
					mainW := mainWidth
					f.Column().Width(mainW).Grow(1).FillHeight().Pad(12).Gap(8).Children(func() {
						drawSlot(f, m.kernel, pluginapi.SlotMainHeader, env)
						drawSlot(f, m.kernel, pluginapi.SlotMainBody, env)
						// 输入区：插件没到位时退回壳自带的那一行，保证随时能打字。
						if m.kernel.Has(pluginapi.SlotMainComposer) {
							drawSlot(f, m.kernel, pluginapi.SlotMainComposer, env)
						} else {
							f.Row().Gap(8).Items(flex.Center).Children(func() {
								if f.Field(&m.draft, draftID, "输入", "说点什么…（Enter 发送）").Grow(1).Submitted() {
									m.send()
								}
								if f.Button(btnSend, "发送").Clicked() {
									m.send()
								}
							})
						}
					})

					if hasRightbar {
						f.Column().Width(rightbarW).FillHeight().Pad(12).Gap(8).
							Background(m.theme.SidebarFill).Children(func() {
							drawSlot(f, m.kernel, pluginapi.SlotRightbar, env)
						})
					}
				})

				// 底部状态栏分两步：先用一个 24px 的空行把位置占住（让主体和侧栏都提前让开），
				// 再把状态栏绝对定位钉在窗口底边画在上面。
				// 只绝对定位不占位的话，主体和侧栏会一直画到窗口底边、被状态栏压住（踩过）。
				if m.kernel.Has(pluginapi.SlotStatusbar) {
					f.Row().FillWidth().Height(24)
					f.Row().Absolute().Left(0).Bottom(0).Width(bounds.Width).Height(24).
						Children(func() {
							drawSlot(f, m.kernel, pluginapi.SlotStatusbar, env)
						})
				}
			})
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
	addr := flag.String("addr", "http://127.0.0.1:7788", "Transport base URL")
	limit := flag.Int("limit", 0, "只渲染最后 N 条消息（0 = 全部）；用于量视口虚拟化的收益")
	sess := flag.String("session", "", "启动后直接打开这个会话 id")
	tail := flag.Int("tail", 40, "session.load 拉多少条尾巴（实测每条消息约 0.46 MB 纹理，40 条约 100 MB）")
	smoke := flag.Bool("smoke", false, "启动后自动发一条消息，验证 send 链路（不做按键模拟）")
	demo := flag.Bool("demo", false, "在消息列表里插一条假的工具卡，验证渲染")
	title := flag.String("title", "dsh-ui", "窗口标题（并行跑多个实例时用来区分）")
	keysSeq := flag.String("keys", "", "调试：启动后依次注入按键（down,up,home,end,esc），走的是插件的 HandleKey，用来验证键盘路由")
	expandThink := flag.Bool("expand-think", false, "调试：把思考块默认展开（截展开态用）")
	collapseAll := flag.Bool("collapse", false, "调试：启动时把所有工作区分组收起（截折叠态用）")
	rightbarOn := flag.Bool("rightbar", true, "启动时是否展开右栏（-rightbar=false 收起来）")
	token := flag.String("token", "", "宿主 launch token：给了就先 /connect 换 cookie（真宿主 dshd serve 必须给）")
	shot := flag.String("shot", "", "抓一张窗口图存到这个路径然后退出（窗口会挪到屏幕外，不弹给用户）")
	flag.Parse()
	renderLimit = *limit
	forceSession = *sess
	tailCount = *tail
	smokeSend = *smoke
	demoTool = *demo
	winTitle = *title
	shotPath = *shot

	m := &model{cli: apiclient.New(*addr), status: "连接 " + *addr + " …", loading: true,
		cache:        map[string]*cachedSession{},
		mux:          apiclient.NewMux(*addr),
		scrollPos:    map[string]float32{},
		showRightbar: *rightbarOn,
	}
	m.refreshList()

	// 真宿主（dshd serve）要求先把 launch token 换成 cookie，否则 /api 一律 401。
	// mockd 不需要，所以这一步只在给了 -token 时做。
	if *token != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := m.cli.Connect(ctx, *token)
		cancel()
		if err != nil {
			log.Fatalf("/connect 失败: %v", err)
		}
		log.Print("已用 launch token 换到 cookie")
	}

	// 装插件：先根本依赖（ds-core：设计令牌 + 基础组件），再依赖它的部件。
	// 装配器按 Requires 排序；ds-sidebar 少一个键就直接报错，不会静默少画一块。
	m.side = &dssidebar.Plugin{CollapseAll: *collapseAll}
	m.kernel = uikernel.New("dsh-ui")
	if err := m.kernel.Load(
		&dscore.Plugin{Dark: true}, // 根本依赖插件：令牌 + 基础组件
		m.side,                     // 左栏会话列表
		&dsconversation.Plugin{ExpandAllThinking: *expandThink}, // 主区消息流 + 工具卡 + 审批
		&dscomposer.Plugin{},  // 输入区
		&dsstatusbar.Plugin{}, // 底部状态栏
		&dstopbar.Plugin{},    // 主区顶部条
		&dsrightbar.Plugin{},  // 右栏工具轨迹
	); err != nil {
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
		// 审批：宿主侧真正的「批准/拒绝」接口还没进契约（见 DSH-GO/AGENTS.md §12），
		// 这里只把本地气泡收掉并给一句状态——以前内联版本就是这么做的。
		// 注意：**不能**留 nil，否则插件里判 nil 静默跳过，按钮点了没反应。
		Allow: func() {
			m.mu.Lock()
			m.approval = nil
			m.mu.Unlock()
			m.setStatus("已允许")
		},
		Deny: func() {
			m.mu.Lock()
			m.approval = nil
			m.mu.Unlock()
			m.setStatus("已拒绝")
		},
		SetDraft: func(text string) { m.draft.SetText(text) },
		Retry:    m.retryLast,
	}
	log.Printf("插件 %v，主题 %s", m.kernel.Order(), m.theme.Name)

	if *keysSeq != "" {
		go func(seq string) {
			time.Sleep(2500 * time.Millisecond) // 等会话列表和消息都到位
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
		startShot(shotPath, 4*time.Second)
	}

	var canvas ui.Canvas
	if err := app.RunCanvas(newConfig(m), &canvas); err != nil {
		log.Fatal(err)
	}
}
