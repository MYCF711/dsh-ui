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
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/ansi"
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/internal/uiplugins"
	pubapi "github.com/MYCF711/dsh-ui/pluginapi"
	_ "github.com/MYCF711/dsh-ui/plugins/dsactioncards"
	"github.com/MYCF711/dsh-ui/plugins/dsattachment"
	_ "github.com/MYCF711/dsh-ui/plugins/dsbrand"
	_ "github.com/MYCF711/dsh-ui/plugins/dscommands"
	_ "github.com/MYCF711/dsh-ui/plugins/dscomposer"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	_ "github.com/MYCF711/dsh-ui/plugins/dsdecl"
	_ "github.com/MYCF711/dsh-ui/plugins/dsdeliverables"
	_ "github.com/MYCF711/dsh-ui/plugins/dsdemo"
	_ "github.com/MYCF711/dsh-ui/plugins/dsdirpicker"
	"github.com/MYCF711/dsh-ui/plugins/dsfiles"
	_ "github.com/MYCF711/dsh-ui/plugins/dsgoal"
	_ "github.com/MYCF711/dsh-ui/plugins/dsjobs"
	_ "github.com/MYCF711/dsh-ui/plugins/dsopen"
	_ "github.com/MYCF711/dsh-ui/plugins/dsquestions"
	_ "github.com/MYCF711/dsh-ui/plugins/dsrightbar"
	_ "github.com/MYCF711/dsh-ui/plugins/dsschedule"
	"github.com/MYCF711/dsh-ui/plugins/dssettings"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	_ "github.com/MYCF711/dsh-ui/plugins/dsshortcuts"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar"
	_ "github.com/MYCF711/dsh-ui/plugins/dsskill"
	"github.com/MYCF711/dsh-ui/plugins/dsstatusbar"
	_ "github.com/MYCF711/dsh-ui/plugins/dsterminal"
	_ "github.com/MYCF711/dsh-ui/plugins/dstool"
	_ "github.com/MYCF711/dsh-ui/plugins/dstopbar"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

// cachedSession 是切换会话用的温缓存：切回来先显示它，再后台刷新。
// 上限 5 个（约 3 MB/1000 条），超了就淘汰最久没碰的那个。
type cachedSession struct {
	msgs      []apiclient.Message
	total     int
	title     string
	truncated bool
	cwd       string
	usage     apiclient.Usage
	// 宿主 §12 H1/H4 的字段跟 usage 一起缓存：切回来时先显示上一次的真值，
	// 而不是先掉回零值（那会让状态栏闪一下"没有数据"）。
	usageTotal    apiclient.Usage
	contextTokens int64
	contextWindow int64
	model         string
	at            time.Time
}

// rightbarSessionState 把用户选择留在各自会话里；explicit=false 时使用壳的显式默认值。
type rightbarSessionState struct {
	open     bool
	explicit bool
}

const cacheLimit = 5

type model struct {
	// view 是整窗的 ui.Page（v0.6.0 把 flex.View 并进了 ui.Page）。
	// 主题是**构造期**给的：这里传的是包级 `theme` 的地址，所以运行时换主题
	// （applyPendingTheme 在 Paint 里改 theme）下一帧就生效，不必重建 Page。
	view   *ui.Page
	draft  ui.Field
	search ui.Field // 侧栏过滤（按标题 + cwd 匹配）
	inv    app.Invalidator
	cli    *apiclient.Client
	cache  map[string]*cachedSession
	// cwd 是当前会话的工作目录（来自 session.load.cwd）。插件拼文件路径要用它，
	// 空串表示不知道——那时它们不该拼。
	cwd string
	// fsDirs 是按目录缓存的名字列表（workspace.list 的结果）。
	// 键是目录绝对路径；空切片表示"正在拉"，不是"目录是空的"。
	fsDirs map[string][]pluginapi.FSEntry
	// fsPending 记录哪些目录正在拉，避免同一目录每帧都发一次请求。
	fsPending map[string]bool
	// fileRefs/sessionRefs 是 `@` 候选的按查询缓存（键 = sessionID + NUL + query）。
	// pending 防重入：插件每帧都可能问同一个 query。
	fileRefs           map[string][]pluginapi.FileReferenceCandidate
	fileRefsPending    map[string]bool
	fileRefsFailed     map[string]bool
	sessionRefs        map[string][]pluginapi.SessionReferenceCandidate
	sessionRefsPending map[string]bool
	sessionRefsFailed  map[string]bool
	// skills 是 `/` 候选（宿主 skills.list，按会话）；与 dsskill 的 resolver 同源。
	skills        map[string][]pluginapi.SkillCandidate
	skillsPending map[string]bool
	skillsFailed  map[string]bool
	// feedback 是消息反馈缓存（键 = sessionID → messageID → item）。
	// loaded=false 表示还没拉到（插件不画反馈按钮的"已记录"态）。
	feedback        map[string]map[string]pluginapi.FeedbackItem
	feedbackLoaded  map[string]bool
	feedbackPending map[string]bool
	feedbackFailed  map[string]bool
	// feedbackResults 是最近一次 put/delete 的终态（插件取走后清空）。
	feedbackResults map[string]pluginapi.FeedbackResult
	// schedules 是定时任务的缓存（宿主 <home>/state/schedules/ 的投影）。
	schedules []pluginapi.ScheduleItem
	// schedCatalog 是官方 `schedule.catalog` 的缓存（7-kind union 的全量目录）。
	// loaded=false 表示还没成功拉到过——界面必须画"加载中"，不能画空状态。
	schedCatalog        []apiclient.ScheduleEntry
	schedCatalogLoaded  bool
	schedCatalogErr     string
	schedCatalogPending bool
	// schedHistory 是按任务 id 的投递历史缓存（官方 schedule.history）。
	schedHistory        map[string]*apiclient.ScheduleHistoryPage
	schedHistoryLoaded  map[string]bool
	schedHistoryErr     map[string]string
	schedHistoryPending map[string]bool
	// schedActionFail 是最近一次 schedule 动作（删除/暂停/执行）的失败动作 id
	// （"delete"/"update"/"run"）。插件取走后清空：用户动作的失败必须当场说一次。
	// 宿主的原始错误文本由壳写状态栏，界面只画官方那一句本地化文案。
	schedActionFail string
	// jobs 是后台任务的缓存（宿主 loopRunner.jobs 的投影）。
	//
	// **它靠轮询刷新**：宿主那边是子进程推上来的一份快照，不会主动通知界面。
	// 所以壳每隔一会儿问一次（见 refreshJobs 的调用点），而不是等事件。
	jobs []pluginapi.Job
	// goal 是这条会话当前在做的事（session.load 的投影）。
	// 空 = 没设过，GoalBar 不画。
	goal string
	// termID/termText/termNext 是这个会话的长连接终端。
	// **只有一份**：面板不是终端模拟器，是"看一眼命令在跑什么"；
	// 多开会让"哪个是哪个"变成一个新的问题，而那不是当前要解决的问题。
	termID      string
	termText    string // 已剥掉 ANSI 的可读文本（累计）
	termNext    int    // 增量水位
	termPending bool

	// models 是可选模型清单（宿主 models.list 的投影），启动拉一次。
	models []pluginapi.Model
	// settingsDescribe 是宿主 settings.describe 的缓存（设置面板的数据面）。
	// nil = 还没拉到（界面显示「未知」，不编值）。
	settingsDescribe *apiclient.SettingsDescribeResult
	// settingsDescribePending 防重入：每帧都可能问，别每帧发一次请求。
	settingsDescribePending bool
	// settingsDescribeErr 是最近一次拉取失败的原因（只报状态栏，不清缓存）。
	settingsDescribeErr error
	// pluginInventory 是官方 `pluginInventory.list` 的缓存（Plugins 设置页的数据面）。
	// nil = 还没成功拉到（界面显示「未知·宿主未接」，**不画假卡片**）。
	pluginInventory *apiclient.PluginInventoryResult
	// pluginInventoryPending 防重入：每帧都可能问，别每帧发一次请求。
	pluginInventoryPending bool
	// pluginInventoryErr 是最近一次拉取失败的原因（只报状态栏，不清缓存）。
	pluginInventoryErr error
	// shellChannelServed 报告宿主的壳更新渠道服务能不能应答
	// （`desktopUpdate.channel.get`）。false = 宿主没这条能力 -> 「更新渠道」行不出现。
	shellChannelServed bool
	// shellChannel 是宿主报的当前渠道；nil = 用户还没选过（官方 channel: null）。
	shellChannel *string
	// shellChannelFailure 是宿主报的壳级故障（read/format/channel/cleanup）。
	shellChannelFailure string
	// desktopUpdate 是宿主 `desktopUpdate.status` 的缓存（设置行第 3 格
	// 「桌面更新指示器」的唯一来源）。nil = 还不知道 / 宿主没这条能力。
	desktopUpdate *apiclient.DesktopUpdatePresentation
	// desktopUpdateFailed 是官方 `DesktopUpdateView.failed`：桥**存在**但调用失败。
	//
	// 与 nil 的区别是语义级的：nil = "没有这枚指示器"（官方没有 updates 桥），
	// failed = "有桥，但它坏了"（官方画成「重试更新」）。把前者写成后者
	// 就是编了一条不存在的事实。
	desktopUpdateFailed bool
	// desktopUpdateOpening 是官方 `DesktopUpdateView.opening`：用户点了更新入口、
	// 动作还在飞。期间指示器禁止再次触发（官方 `DesktopUpdateSource.open()`）。
	desktopUpdateOpening bool
	// desktopUpdatePending 防重入：拉取在途时不再发第二次请求（每帧都可能问）。
	desktopUpdatePending bool
	// desktopUpdateAt 是上一次拉取的完成时刻（决定要不要再问一次）。
	desktopUpdateAt time.Time
	// jobsPending 防重入：刷新在途时不再发第二次请求（每帧都可能问）。
	jobsPending bool
	// jobOutputs 是后台任务输出的按需缓存（键 = sessionID + "\x00" + id）。
	// 只在用户展开某个任务时才有条目；插件拿的是**累计视图**（见 jobs_actions.go）。
	jobOutputs map[string]*jobOutputCache
	smokeFired bool

	// 待审批的请求（每会话一条）；前台指针只是 approvals[cur] 的发布副本。
	approval  *approvalReq
	approvals map[string]*approvalReq

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
	epochs map[string]int64
	// frames 是每会话的**原始 journal 帧**（主区 transcript 的数据面，见 frames.go）。
	//
	// 与 msgs 是两件事：msgs 是 session.load 的**投影**（首屏用），frames 是
	// journal 原文（steering / turn-trigger / reason.kind 这些只有原文才分得出来）。
	// 绘制期在锁内拷一份进快照，插件读不到活切片。
	frames     map[string]*frameJournal
	streamText string
	// turnBusy：当前会话有 turn 在跑。发 prompt 置 true，流结束/取消/失败置 false。
	turnBusy bool
	// lastSeq 是**当前会话**已收到的最大 seq。per-session 的水位在 muxSub.lastSeq，
	// 这里保留一份是为了"打开会话时给 since"和审批帧用，不要拿它当全局水位。
	lastSeq int
	// streamStates 按会话保存 canonical Stream；前台 streamText/busy 只是发布副本。
	// 后台帧也走同一套 apiclient 归约，切回不靠取消订阅或整页重放补内容。
	streamStates map[string]*sessionStreamState

	// ── 插件运行时（界面线的「一切皆插件」）──
	// 壳只做三件事：装配插件、给槽位、把快照递进去。
	// 部件怎么分组、怎么排序、展开还是收起，全是插件自己的事。
	kernel *uikernel.Kernel
	theme  *pluginapi.Theme
	prims  pluginapi.Primitives
	side   *dssidebar.Plugin
	// settings 是设置浮层。壳持有实例只为一件事：`-open-settings` 无头截图要调它，
	// 以及把它排在装配顺序最前（HandleKey 第一个拿到键才能先关掉自己）。
	settings *dssettings.Plugin
	// statusbar 是状态栏插件（官方 composer dock + 连接指示 + Activity 弹窗）。
	//
	// 壳持有实例的理由与 settings 相同：它必须排在**装配顺序最前**——
	// kernel 按装配顺序转发按键，第一个返回 true 的赢。dscomposer 在没有审批时
	// 把 Esc 当"停止 turn"（composer.go 的 HandleKey），而 Activity 弹窗的 Esc
	// 关闭在 dsstatusbar 里。排在 dscomposer 之后 → 跑 turn 时按 Esc 只会停 turn，
	// 弹窗关不掉（官方是 document 级监听，两件事互不抢）。
	statusbar *dsstatusbar.Plugin
	actions   pluginapi.Actions
	chrome    frameChrome
	// shellMode 是首屏结构档位（auto/legacy/official），见 shell.go。
	// 注意：它决定「谁切三栏」，不是外观选项——两块实现几何口径不同。
	shellMode shellMode
	// wave3 是 GUI 第三波两个父容器的壳侧数据接线（账号态 / 会话投影 / apiclient）。
	// 见 wave3_wiring.go：它只喂缓存，不在绘制里发请求。
	wave3 wave3Wiring

	mu           sync.Mutex
	chromeLayout frameChromeLayout // 由 Paint 写入，原生截图报告在 mu 下读取。
	// themeMode 是用户选的模式（dark/light/system），pendingTheme 是"排着队、
	// 等下一帧 Paint 去消费"的那一个。见 theme.go 里为什么必须排队。
	themeMode    string
	pendingTheme string
	// pendingDraft / pendingSmoke 和 pendingTheme 同一形状：草稿是 Viva
	// `*flex.Field` 的状态，只有 UI 线程能碰——`text.(*Editor).String()` 名字像
	// 取值，实现里却会惰性写 `editor.text`/`editor.textValid`，后台 goroutine
	// 读它就已经是数据竞争。背景 goroutine 只排队，由 applyPendingDraft 消费。
	// 上游缺陷说明见 docs/viva-upstream-editor-string-race.md。
	pendingDraft *draftRestoreRequest
	pendingSmoke *string
	sessions     []apiclient.Session
	cur          string
	msgs         []apiclient.Message
	usage        apiclient.Usage
	// 宿主 §12 H1/H4/H5：累计用量、当前上下文、模型名、审批策略。
	// 零值/空串 = 不知道，界面据此整段不画（绝不拿 0 或 mode 冒充）。
	usageTotal    apiclient.Usage
	contextTokens int64
	contextWindow int64
	modelName     string
	permission    string
	// showRightbar 是当前会话的右栏开合（壳持有，插件只读）。
	showRightbar bool
	// rightbarDefault 保留 -rightbar 的显式默认；普通启动为 false。
	rightbarDefault  bool
	rightbarSessions map[string]rightbarSessionState
	status           string
	loading          bool

	// WP-C1 的本地发送与原始终局均按会话隔离；不进入 apiclient.Message.Seq。
	outgoing   map[string][]pubapi.OutgoingMessage
	msgMeta    map[string]map[string]pubapi.MessageMetadata
	turnEvents map[string][]pubapi.TurnEvent
	localSend  uint64
}

func (m *model) setStatus(s string) {
	m.mu.Lock()
	m.status = s
	m.loading = false
	m.mu.Unlock()
	m.inv.Invalidate()
}

// syncRightbarLocked 默认收起右栏；工具历史不自动抢占正文，用户明确选择后按会话记忆。
// 调用方必须持有 m.mu。
func (m *model) syncRightbarLocked() {
	state := m.rightbarSessions[m.cur]
	if !state.explicit {
		state.open = m.rightbarDefault
		if m.cur != "" {
			if m.rightbarSessions == nil {
				m.rightbarSessions = map[string]rightbarSessionState{}
			}
			m.rightbarSessions[m.cur] = state
		}
	}
	m.showRightbar = state.open
}

func (m *model) toggleRightbar() {
	m.mu.Lock()
	m.showRightbar = !m.showRightbar
	if m.cur != "" {
		if m.rightbarSessions == nil {
			m.rightbarSessions = map[string]rightbarSessionState{}
		}
		m.rightbarSessions[m.cur] = rightbarSessionState{open: m.showRightbar, explicit: true}
	}
	on := m.showRightbar
	m.mu.Unlock()
	if on {
		m.setStatus("右栏：显示")
	} else {
		m.setStatus("右栏：收起")
	}
}

func (m *model) refreshList() {
	// 官方任务目录：与 schedule.list 一样在启动/新会话入口拉一次（见
	// schedule_catalog.go）。这里不阻塞 refreshList 的既有链路。
	go m.refreshScheduleCatalog()
	// **UI 配置在这里拉**：`refreshList` 是启动与新建会话的公共入口
	// （`-session` 的自动打开也在它内部），放这儿比放 `newSession` 早且全覆盖。
	//
	// 为什么必须壳主动拉：`dsshell` 的拖拽宽度在第一帧之后轮询
	// `Actions.ConfigGet`，而那个返回的是**壳的缓存**。壳不拉一次，缓存永远是 nil
	// —— 持久化“读回来”这条链在真机上就从来没通过（我第一版只接了 set 方向）。
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cfg, err := m.cli.ConfigGet(ctx)
		if err != nil {
			return // 读不到就用默认，别打扰用户
		}
		m.mu.Lock()
		uiConfig = cfg
		m.mu.Unlock()

		// 上次在命令面板里选的主题：没有显式 `-theme` 就恢复它。
		// 只排队（不落盘）——这是"读回来"，再写回去只会多一次无意义的写。
		if !themeExplicit {
			if v, ok := cfg["ui.theme"]; ok && validThemeMode(v) {
				m.queueTheme(v)
			}
		}

		// 定时任务：读 <home>/state/schedules/，失败就算了（只是界面上的一块）。
		//
		// **必须在启动时拉一次**：`dsschedule` 插件的 `Actions.ScheduleList` 读的是
		// 这里的缓存。不拉就是空数组，而空数组在插件那边等于"没有定时任务"——
		// 它不会画空标题，所以界面上什么都看不到（**看起来像功能没做**）。
		if scheds, serr := m.cli.ScheduleList(ctx); serr == nil {
			items := make([]pluginapi.ScheduleItem, 0, len(scheds))
			for _, it := range scheds {
				items = append(items, pluginapi.ScheduleItem{
					ID: it.ID, Title: it.Title, Kind: it.Kind, NextAt: it.NextAt,
					EverySec: it.EverySec, DailyAt: it.DailyAt, WeeklyAt: it.WeeklyAt,
				})
			}
			m.mu.Lock()
			m.schedules = items
			m.mu.Unlock()
		}
		// 模型清单：启动拉一次（网关的模型不会每分钟变；变了重新打开就好）。
		// **失败不写状态栏**：它只是"能不能选模型"，不该盖掉更重要的提示。
		if ms, merr := m.cli.ModelsList(ctx); merr == nil {
			out := make([]pluginapi.Model, 0, len(ms))
			for _, it := range ms {
				out = append(out, pluginapi.Model{ID: it.ID, OwnedBy: it.OwnedBy})
			}
			m.mu.Lock()
			m.models = out
			m.mu.Unlock()
		}
		// 后台任务是**轮询**的：宿主那边的数据靠子进程推上来，不会主动通知界面。
		// 5 秒一次——够快让用户看到起止，又不会把宿主问烦。只有内容变了才重绘。
		go func() {
			t := time.NewTicker(5 * time.Second)
			defer t.Stop()
			for range t.C {
				m.mu.Lock()
				pending := m.jobsPending
				m.mu.Unlock()
				if !pending {
					m.refreshJobs()
				}
				// 终端和后台任务同一个 ticker：都是"隔一会儿问一次"的形状。
				m.refreshTerminal()
			}
		}()
	}()
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

// publishContextWindowLocked 是 WP-C2 publishSessionLocked 的 STATS-WIRE 扩展：
// 容量与占用一样按会话发布，后台会话的 load 不得改写前台容量。
// 调用方必须持有 m.mu。
func (m *model) publishContextWindowLocked(id string) {
	if id != m.cur {
		return
	}
	m.contextWindow = 0
	if cache := m.cache[id]; cache != nil {
		m.contextWindow = cache.contextWindow
	}
}

// integrateLoadWithContextWindowLocked 复用 WP-C2 的 integrateLoadLocked 作为唯一
// 正文归并路径，并在同一临界区补齐容量缓存。WP-C2 owner 文件保持不动。
// 调用方必须持有 m.mu。
func (m *model) integrateLoadWithContextWindowLocked(id string, result *apiclient.LoadResult, metadata map[string]pubapi.MessageMetadata) {
	m.cachedSessionLocked(id).contextWindow = result.ContextWindow
	m.integrateLoadLocked(id, result, metadata)
	m.publishContextWindowLocked(id)
}

func (m *model) openSession(id string) {
	// 滚动位置不用在这里存：m.recordScroll 每帧都在抄（v0.6.0 没有可回头查的
	// 滚动状态入口，见 recordScroll 的注释）。

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
	// 反馈是宿主 sidecar，可能在别处改过；每次打开会话重新拉一次，
	// 顺带清掉上一次的失败抑制（否则一次网络抖动会让这条会话永远不再加载反馈）。
	if m.feedbackLoaded != nil {
		delete(m.feedbackLoaded, id)
	}
	if m.feedbackFailed != nil {
		delete(m.feedbackFailed, id)
	}
	// 水位跟着会话走；后台事件已经持续归约，所以切回绝不取消订阅重放。
	m.lastSeq = m.seqs[id]
	// 审批同样按会话保存，恢复原 requestId/options，不在切换时丢请求。
	m.approval = m.approvals[id]
	hit := false
	if cs, ok := m.cache[id]; ok {
		m.publishSessionLocked(id)
		m.publishContextWindowLocked(id)
		m.status = fmt.Sprintf("温缓存 · %d 条（总 %d）", len(cs.msgs), cs.total)
		m.loading = false
		hit = true
	} else {
		// 换成没缓存的会话时必须清掉上一份前台真值。
		m.msgs = nil
		m.usage = apiclient.Usage{}
		m.usageTotal = apiclient.Usage{}
		m.contextTokens = 0
		m.contextWindow = 0
		m.modelName = ""
		m.cwd = ""
		state := m.streamStateLocked(id)
		m.streamText = state.text
		m.turnBusy = state.busy
		m.loading = true
		m.status = "加载 " + id + " …"
	}
	m.syncRightbarLocked()
	m.mu.Unlock()
	m.inv.Invalidate()

	// skill 目录是**按会话**的（随该会话 cwd / agent preset 变），所以切会话时
	// 重新取一次；失败只降级成"没有目录"，不阻塞下面的加载。
	go m.refreshSkillCatalog(id)

	if hit {
		// 温切直接使用 canonical 缓存与仍在归约的订阅：不 load、不 refresh、
		// 不取消重订，正在生成的正文和阅读位置都留在原会话状态里。
		m.subscribe(id)
		return
	}

	// 冷载先取 journal 基线再订阅，避免 load 后又从 0 重放整条 journal。
	{
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if r, _, err := m.loadSession(ctx, id, tailCount); err == nil {
			m.mu.Lock()
			if r.LastSeq > m.seqs[id] {
				m.seqs[id] = r.LastSeq
			}
			if r.JournalEpoch > 0 {
				m.epochs[id] = r.JournalEpoch
			}
			m.lastSeq = m.seqs[id]
			m.mu.Unlock()
		}
		cancel()
	}
	m.subscribe(id)
	go m.refreshSession(id, false)
	// 主区 transcript 的数据面：WS 只从 lastSeq 之后推，之前的历史只有
	// `session.history` 能拿到（load 是投影，没有 frames 键）。
	go m.refreshFrames(id)
}

// refreshSession 拉一次 session.load；quiet=true 时失败不覆盖状态栏。
func (m *model) refreshSession(id string, quiet bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, metadata, err := m.loadSession(ctx, id, tailCount)
	if err != nil {
		if !quiet {
			m.setStatus("加载失败: " + err.Error())
		}
		return
	}
	m.mu.Lock()
	before := 0
	if cache := m.cache[id]; cache != nil {
		before = len(cache.msgs)
	}
	// load 与 WS 可能并发完成：先并入 canonical 缓存，不能把飞行期间到达的
	// 后台正文用一份较旧 tail 覆盖掉。当前/后台会话走完全相同的归并路径。
	m.integrateLoadWithContextWindowLocked(id, r, metadata)
	m.evictLocked(id)
	if msgsDebug > 0 {
		msgsDebug--
		log.Printf("DBG load session:%s 宿主 tail=%d total=%d | 覆盖前 %d 条 → 之后 %d 条",
			id, len(r.Messages), r.Total, before, len(m.cache[id].msgs))
	}
	if r.JournalEpoch > 0 {
		m.epochs[id] = r.JournalEpoch
	}
	if r.LastSeq > m.seqs[id] {
		m.seqs[id] = r.LastSeq
		if id == m.cur {
			m.lastSeq = r.LastSeq
		}
	}
	// permission 是进程级审批策略，不是会话 mode。
	m.permission = r.Permission
	// goal 跟 permission 同款：来自 load 的投影，空串表示没设过。
	m.goal = r.Goal
	if m.cur == id {
		if demoTool && len(m.msgs) > 0 {
			req := &approvalReq{Tool: "bash", Cmd: "git status --short",
				Reason: "想看一下当前工作区有没有未提交的改动"}
			m.approval = req
			if m.approvals == nil {
				m.approvals = map[string]*approvalReq{}
			}
			m.approvals[id] = req
			m.cache[id].msgs = append(m.cache[id].msgs,
				apiclient.Message{Seq: -2, Role: "tool", Tool: "bash",
					Args:    "git status --short",
					Summary: "exit 0 · 3 files changed · 用了 12 ms"},
				apiclient.Message{Seq: -3, Role: "tool", Tool: "read",
					Args:    "internal/apiclient/mux.go",
					Summary: "1.2 KB · 78 行 · 读完了"},
				apiclient.Message{Seq: -4, Role: "assistant", Text: demoWideCode})
			m.cache[id].total = len(m.cache[id].msgs)
			m.publishSessionLocked(id)
		}
		// 调试胶水（1 行）：-demo-workflow 时往 journal 塞一组假的
		// tool-workflow/* 帧；默认开关关着，正式路径不看它。
		m.injectDemoWorkflowLocked(id)
		m.syncRightbarLocked()
		// **不写连接措辞**：连接状态的家是官方 `ConnectionIndicator`
		// （`Snapshot.Link`），composer 那一行只放动作状态（HH 单：两处重复）。
		m.status = fmt.Sprintf("已加载 %d 条（总 %d）", len(r.Messages), r.Total)
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
		// 这里在后台 goroutine 里：草稿只能排队，写和发送都由 UI 线程做
		//（见 applyPendingDraft 与 docs/viva-upstream-editor-string-race.md）。
		m.queueSmokeSend(text)
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

// refreshTerminal 拉一次终端增量，剥掉 ANSI 之后拼到累计文本上。
//
// **只有内容真变了才重绘**（和 refreshJobs 同一个理由）：轮询每 2 秒一次，
// 无脑 Invalidate 会让空闲的终端也每 2 秒重画一遍。
func (m *model) refreshTerminal() {
	m.mu.Lock()
	id := m.termID
	since := m.termNext
	pending := m.termPending
	if !pending {
		m.termPending = true
	}
	m.mu.Unlock()
	if id == "" || pending {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := m.cli.TerminalOutput(ctx, id, since)

	m.mu.Lock()
	m.termPending = false
	if err != nil {
		m.mu.Unlock()
		return // 拉失败就留着上一次的，别把面板清空
	}
	changed := out.Output != ""
	if changed {
		m.termText += ansi.Strip(out.Output)
	}
	m.termNext = out.Next
	m.mu.Unlock()

	if changed {
		m.inv.Invalidate()
	}
}

// refreshJobs 拉一次后台任务清单，**只有内容真变了才重绘**。
//
// 为什么这么小心：这是个轮询（宿主不会主动通知），每 5 秒一次；
// 无脑 Invalidate 会让界面每秒重画一次，而 99% 的时候清单没变。
func (m *model) refreshJobs() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	list, err := m.cli.JobsList(ctx)

	m.mu.Lock()
	m.jobsPending = false
	if err != nil {
		m.mu.Unlock()
		return // 拉失败就留着上一次的，别把界面清空
	}
	out := make([]pluginapi.Job, 0, len(list))
	for _, j := range list {
		out = append(out, pluginapi.Job{
			ID: j.ID, SessionID: j.SessionID, Command: j.Command, Done: j.Done,
			ExitCode: j.ExitCode, Bytes: j.Bytes,
		})
	}
	changed := !sameJobs(m.jobs, out)
	m.jobs = out
	// 任务离开清单后，输出缓存也一并丢掉（不然每次后台跑完都会留一份文本）。
	m.pruneJobOutputs(out)
	m.mu.Unlock()

	if changed {
		m.inv.Invalidate()
	}
}

// sameJobs 比两份清单是否一致（只比界面会显示的那几个字段）。
func sameJobs(a, b []pluginapi.Job) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Done != b[i].Done || a[i].ExitCode != b[i].ExitCode || a[i].Bytes != b[i].Bytes {
			return false
		}
	}
	return true
}

// loadDir 后台拉一个目录，成功后写缓存并重绘。
//
// **空 dir 不拉**：那会退化成"进程 cwd"，而这条链上我们要的一直是"这条会话的
// 工作目录"，拿进程 cwd 冒充在多工作区下就是错的。
func (m *model) loadDir(dir string) {
	if dir == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ents, err := m.cli.WorkspaceList(ctx, dir)
	m.mu.Lock()
	if m.fsDirs == nil {
		m.fsDirs = map[string][]pluginapi.FSEntry{}
	}
	if m.fsPending == nil {
		m.fsPending = map[string]bool{}
	}
	delete(m.fsPending, dir)
	if err == nil {
		es := make([]pluginapi.FSEntry, 0, len(ents))
		for _, e := range ents {
			es = append(es, pluginapi.FSEntry{Name: e.Name, IsDir: e.IsDir})
		}
		sort.Slice(es, func(i, j int) bool { return es[i].Name < es[j].Name })
		m.fsDirs[dir] = es
	}
	m.mu.Unlock()
	m.inv.Invalidate() // 拉到了就重绘，插件下一帧才看得见
}

// settleScroll 是 Draw 返回后对滚动区的全部处理：先恢复待恢复的位置，再记本帧偏移。
//
// **顺序不能反**：recordScroll 会覆盖 m.scrollPos，而切会话那一帧新滚动区是从 0
// 布局的——先记再恢复就会把"切走前的位置"覆盖成 0，恢复等于没恢复。
// 回归测试：TestPendingScrollRestoresOffset（它调的就是这个函数）。
func (m *model) settleScroll(box ui.Box, handed bool) bool {
	repaint := m.applyPendingScroll(box, handed)
	m.recordScroll(box, handed)
	return repaint
}

// recordScroll 把本帧消息流滚动区的偏移抄进 m.scrollPos。
//
// v0.6.0 起**没有**"按 ID 查 ScrollState"的入口：滚动状态由声明方（消息流插件）
// 持在 `Box` 上，`Box.ScrollState()` 只在同一个 paint 内有效。所以壳不再在
// "切会话那一刻回头查上一帧"，改成**每帧抄一次**——切走时 m.scrollPos[prev]
// 已经是上一帧的真值，语义等价，且不依赖任何跨帧的 Box/指针。
//
// box/handed 为空（本帧没有消息流滚动区，比如没有 ds-conversation 插件）时跳过。
func (m *model) recordScroll(box ui.Box, handed bool) {
	if !handed {
		return
	}
	st := box.ScrollState()
	offset := st.Offset
	m.mu.Lock()
	if m.scrollPos == nil {
		m.scrollPos = map[string]float32{}
	}
	if m.cur != "" {
		m.scrollPos[m.cur] = offset
	}
	m.mu.Unlock()
}

// applyPendingScroll 在首批消息完成布局后恢复滚动位置，并报告是否必须补一帧。
//
// Follow 到布局末尾才把 Offset 移到尾部，而消息虚拟化在声明时读的是旧 Offset。
// 空消息 Hero 帧不能消费 pendingScroll；否则首载只有尾部占位，没有下一帧实画。
//
// 恢复必须在布局**之后**做（ScrollTo 的落位要拿本次布局的 Content/Viewport 判断
// "是否停在末尾"），所以用本帧插件交回来的 Box；没有任何滚动区时只报告"要不要补帧"。
func (m *model) applyPendingScroll(box ui.Box, handed bool) bool {
	m.mu.Lock()
	id := m.pendingScroll
	if id == "" || id != m.cur || len(m.msgs) == 0 {
		m.mu.Unlock()
		return false
	}
	off, restore := m.scrollPos[id]
	m.pendingScroll = ""
	m.mu.Unlock()

	if !handed {
		return false
	}
	if restore {
		box.ScrollState().ScrollTo(off)
	}
	return true
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

// reconnectNow 是官方 `ConnectionIndicator` 的「立即重连」（`ctx.connection.reconnect()`）。
//
// 语义三条（与 pluginapi.Actions.Reconnect 的注释同一份契约）：
//
//	① 取消当前会话的订阅（旧 goroutine 从 `Subscribe`/退避里退出）；
//	② **立刻**重新订阅，不睡退避——用户点这一下就是要"现在试"，等 1s→30s
//	   的退避等于点了没反应；
//	③ 水位沿用 `m.lastSeq`（已经收到的帧不重放）。
//
// 连接事实由 `runSubscription` 自己写：新尝试一开始就是 `LinkConnecting`
// （界面「重新连接中」），出结果再变 connected / down。
//
// 幂等：没有当前会话时只重绘一帧（按钮仍然"点得动"，但没东西可连）。
func (m *model) reconnectNow() {
	m.mu.Lock()
	id := m.cur
	if sub, ok := m.subs[id]; ok {
		if sub.cancel != nil {
			sub.cancel()
		}
		delete(m.subs, id)
	}
	m.mu.Unlock()

	if id == "" {
		m.inv.Invalidate()
		return
	}
	log.Printf("立即重连 session:%s（用户点了连接指示）", id)
	m.subscribe(id)
	m.inv.Invalidate()
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
	authNotified := false // 凭据失效只提示一次，别刷屏
	for {
		if ctx.Err() != nil {
			return
		}
		// **每条连接一个可取消的子 ctx**：reset / stale_epoch 之后要 break 出去重连，
		// 必须把旧连接关掉。共用外层 ctx 的话，旧 readLoop 会一直挂在 conn.Read 上，
		// out（缓冲 64）填满后永远阻塞——每换一代 journal 就漏一个 socket 加两个
		// goroutine，而且它还会继续把新 journal 的帧读进一个没人消费的 channel。
		connCtx, connCancel := context.WithCancel(ctx)
		// 一次订阅尝试**开始**：官方 connecting 态（「重新连接中」）就是这一刻。
		// 写事实与读数同一把锁（-race 门禁）；订阅是同步调用，拿到结果立刻改写。
		m.mu.Lock()
		wsConnecting = true
		m.mu.Unlock()
		m.inv.Invalidate()
		ch, err := m.mux.Subscribe(connCtx, "session:"+id, since)
		if err != nil {
			connCancel()
			// 401 = 凭据失效（宿主重启换了 launch token，cookie 在它内存里）。
			// 这不是"网络抖一下"，重试一万次都好不了——必须让用户看见，
			// 否则界面永远静默地连不上，而用户以为只是"有点慢"。
			if isAuthErr(err) && !authNotified {
				authNotified = true
				log.Printf("mux 订阅被拒（凭据失效）：%v", err)
				m.setStatus("宿主凭据已失效（多半是重启过）：请用新的 launch token 重新连接")
			}
			log.Printf("mux 订阅失败（%v），%s 后重试 session:%s", err, backoff, id)
			// 连接层的**可见措辞只属于官方 `ConnectionIndicator`**（侧栏设置行，
			// 见 plugins/dsstatusbar/connection.go）：这里只写连接事实，不再往
			// 壳的状态串里塞"连不上宿主"——那会和官方座位在同一个窗口里说两遍
			// （HH 单的 composer 那行重复就是它带出来的）。
			//
			// 事实本身必须写：早先这里只 log、连接事实从不变，于是宿主被杀 33 秒后
			// 界面还写着「已连接 · 就绪」——UI 在说谎。
			m.mu.Lock()
			wsDown = true
			wsUp = false         // 连过又断：断线也是"已经拿到事实"，但结论是"断"
			wsConnecting = false // 这次尝试已经出结果（失败），不再处于 connecting
			m.mu.Unlock()
			m.inv.Invalidate()
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
		// 订阅真的建起来了：这是"连着"的**唯一证据**（不是"没报错所以算连上"）。
		wsUp = true
		wsDown = false
		wsConnecting = false // 尝试出结果（成功）：connecting 态结束，success 态入场
		m.mu.Unlock()
		log.Printf("mux 已订阅 session:%s since=%d epoch=%d", id, since, ep)

		for ev := range ch {
			if ev.Kind == "gap" {
				resub := false
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
						// 补洞也要报世代：宿主拿它判"这条水位还作不作数"，
						// 不带就只剩按 seq 硬补——换代之后补回来的是一段错位数据。
						ep := int(m.epochs[id])
						m.mu.Unlock()
						n, err := apiclient.FillGap(ctx, m.cli, id, from, to, ep, func(f apiclient.Frame) {
							m.applyHistoryFrame(id, f)
						})
						if err != nil {
							log.Printf("history 补洞失败 session:%s [%d,%d]: %v", id, from, to, err)
							// 409 stale_epoch = journal 换代了，旧水位没意义，**这一条
							// 补不回来**（宿主提示原文就是「重新 load 拿 journalEpoch/
							// lastSeq」）。这是 `session/reset` 的 HTTP 面，处理完全一样：
							// 丢三份水位 + 局部 since 归零 → 重连。
							//
							// **必须重连**：这条连接的读循环还拿旧 since 当基线，
							// 新 journal 的 seq 从 1 起，会被它当"重复帧"静默丢光。
							if strings.Contains(err.Error(), "stale_epoch") {
								m.mu.Lock()
								oldSeq, oldEpoch := since, m.epochs[id]
								since = m.onJournalResetLocked(id)
								m.mu.Unlock()
								m.refreshSession(id, true)
								// 主区数据面同样要重建基线（load 只有投影，没有 frames）。
								go m.refreshFrames(id)
								m.mu.Lock()
								if base := m.seqs[id]; base > since {
									since = base // 新基线之后重连，别把新 journal 整条重放
								}
								base, now := m.seqs[id], since
								m.mu.Unlock()
								log.Printf("journal 换代（stale_epoch）session:%s 旧水位 seq=%d epoch=%d；新基线 lastSeq=%d，从 %d 重连",
									id, oldSeq, oldEpoch, base, now)
								resub = true
							}
						} else {
							log.Printf("history 补洞完成 session:%s 补回 %d 帧", id, n)
						}
					}
				}
				if resub {
					break // 世代变了：这条连接的水位基线已经作废，重连
				}
				continue
			}
			if ev.Kind == "session/reset" {
				// 宿主说"你在旧世代上"：journal 换代了，旧 seq 属于**另一个序号域**。
				// 契约要求的动作是「丢弃旧水位 → 重新 load 拿 journalEpoch/lastSeq」。
				//
				// **局部 since 也要归零**：只删 map 不够——重连用的是本函数的局部
				// since，它还是旧值。拿旧值去要新 journal 的帧，宿主不报错，只是
				// 从错位处开始推（界面症状是"少了几条"）。这条是 U0 复核点名的
				// 「reset 后局部 since 未归零」。
				m.mu.Lock()
				oldSeq, oldEpoch := since, m.epochs[id]
				since = m.onJournalResetLocked(id)
				m.mu.Unlock()
				// **不清空已有消息**——先把新基线拉回来，界面再整体替换，
				// 这样用户看到的是"刷新了一下"，而不是"内容突然没了"。
				m.refreshSession(id, true)
				// 主区数据面同样要重建基线（load 只有投影，没有 frames）。
				go m.refreshFrames(id)
				// refreshSession 会把新基线写回 m.seqs[id]（新 journal 的 lastSeq）。
				// 有它就从他之后重连：从 0 重连会把新 journal 整条重放一遍，而那些
				// 内容刚刚已经由 load 画上去了（同一条消息出现两次）。
				m.mu.Lock()
				if base := m.seqs[id]; base > since {
					since = base
				}
				base, now := m.seqs[id], since
				m.mu.Unlock()
				log.Printf("mux session/reset session:%s（journal 换代）旧水位 seq=%d epoch=%d；新基线 lastSeq=%d，从 %d 重连",
					id, oldSeq, oldEpoch, base, now)
				break
			}
			m.applyEvent(id, ev)
			// 前台与后台都已经消费并归约这条帧，水位必须同步前进；否则后台
			// 内容虽在缓存里，重连/切回仍会重复投递同一段 journal。
			m.mu.Lock()
			if sub := m.subs[id]; sub != nil && ev.Seq > sub.lastSeq {
				sub.lastSeq = ev.Seq
			}
			if ev.Seq > m.seqs[id] {
				m.seqs[id] = ev.Seq
			}
			if ev.Seq > 0 {
				since = ev.Seq
			}
			m.mu.Unlock()
		}
		// 连接已经不用了：reset / stale_epoch 之后是**我们**主动 break 出来的，
		// 那条 socket 还在读（ctx 没取消），不关掉就是漏一个连接加两个 goroutine。
		connCancel()
		// channel 关闭 = 连接断了：带水位重连，而不是退回"下次点开再 load"。
		if ctx.Err() != nil {
			return
		}
		log.Printf("mux 连接断开 session:%s，带水位 %d 重连", id, since)
	}
}

// onJournalResetLocked 是「journal 换代」时本地该做的事：把这一代的水位全部丢掉。
//
// 返回重连该用的 since，**恒为 0**：journal 换代了，旧 seq 属于另一个序号域；
// 带着它去要新 journal 的帧，宿主要么判 stale_epoch、要么从错位处推——后者
// 不报错也不丢帧，只是"少了几条"，这是最难发现的一种错。
//
// 三份水位都要清，少一份就等于没清：
//   - `seqs[id]`：per-session 水位（切会话、重连时取 since 用）；
//   - `lastSeq`：前台水位（applyEvent / 审批帧用它判过期）；
//   - `subs[id].lastSeq`：订阅结构里那一份（重连时 subscribe 从这里取）。
//
// 两个触发点共用它：WS 的 `session/reset`（无 seq 的控制帧）与 history 的
// 409 `stale_epoch`——它们是同一件事的两个面。
//
// 调用方必须持有 m.mu。
func (m *model) onJournalResetLocked(id string) int {
	delete(m.seqs, id)
	delete(m.epochs, id)
	// **原始帧也要整段丢弃**：旧 seq 属于另一个序号域，留着就是"新旧两代混排"，
	// 界面上表现为消息偶尔跳一下（最难看出来的那种错）。重建由 refreshFrames 负责。
	if journal := m.frames[id]; journal != nil {
		journal.reset()
	}
	if sub := m.subs[id]; sub != nil {
		sub.lastSeq = 0
	}
	if id == m.cur {
		// 只清前台会话：别的会话的水位跟这条连接没关系，清掉就是丢别人的进度。
		m.lastSeq = 0
	}
	state := m.streamStateLocked(id)
	state.stream.Reset()
	state.text = ""
	state.busy = false
	state.activeTurnID = ""
	state.activeTurn = nil
	m.publishSessionLocked(id)
	return 0
}

// isAuthErr 判断一条错误是不是"凭据失效"。
//
// coder/websocket 在握手被拒时返回的错误里带 HTTP 状态码（`got 401`）；
// 这里只认 401/403——别的 4xx 是路径/参数问题，重试同样没用但原因不同。
func isAuthErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "got 401") || strings.Contains(msg, "got 403")
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
		state := m.streamStateLocked(id)
		state.busy = false
		state.activeTurnID = ""
		state.activeTurn = nil
		m.publishSessionLocked(id)
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
		for session, pending := range m.approvals {
			if pending != nil && pending.RequestID == requestID {
				delete(m.approvals, session)
			}
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
		for session, pending := range m.approvals {
			if pending != nil && pending.RequestID == req.RequestID {
				delete(m.approvals, session)
			}
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
	m.applyEventAt(id, ev, ev.Time)
}

// applyHistoryFrame 让 history 重放与 live WS 使用同一个保真 Event 边界，
// 包括 Kind/Seq/Topic/Data/Time；不再另手写只有 Data 的事件。
func (m *model) applyHistoryFrame(id string, frame apiclient.Frame) {
	m.applyEvent(id, frame.Event("session:"+id))
}

func (m *model) applyEventAt(id string, ev apiclient.Event, envelopeTime int64) {
	// 兼容既有显式 envelope 测试入口；生产路径由 applyEvent 传入 ev.Time。
	// 这是同一个信封真值，不是在 Event.Time==0 时重新猜 data.time。
	if ev.Time == 0 {
		ev.Time = envelopeTime
	}
	// **原始帧先进数据面日志**（主区 transcript 的唯一来源），再走下面的归约。
	//
	// 为什么放在最前面、连审批帧也进：journal 是 append-only 的**一个**序号域，
	// 少记一条（哪怕它不是消息行）就会在补洞时留下一个"假洞"——宿主会重新推
	// 一遍那段，而客户端按 seq 判重只看得到自己记过的那几条。投影器负责跳过
	// 非消息行（approval/* 等），数据面负责**一条不丢**。
	//
	// 这一帧的 Time 已经是 canonical 真值（上面刚补齐），不再另取墙钟。
	m.mu.Lock()
	m.appendFrameLocked(id, apiclient.Frame{Seq: ev.Seq, Type: ev.Kind, Data: ev.Data, Time: ev.Time})
	m.mu.Unlock()

	// 审批**作废**：宿主的超时按拒绝结算并推这一帧（serve_approval.go），
	// 再 approve 过去只会拿到 404 `no_such_request`。
	//
	// 不清的话审批条会一直挂在界面上——用户点它得到"请求已失效"，
	// 而更糟的是他一直以为"还有东西等着我处理"。这是**僵尸审批条**。
	//
	// 只清"正是那一条"的：迟到的超时帧不该把后一个还没答的审批一起抹掉。
	if ev.Kind == "approval/timeout" || ev.Kind == "approval/cancel" {
		// 载荷和 approval/request **同一形状**（宿主 emit 的是同一份 payload），
		// 所以复用同一个解析器，不另写一份"只读 requestId"的结构——
		// 两份解析器迟早会漂，而漂了的表现是"超时帧认不出是哪一条"。
		//
		// 解不开就当"宿主没说是哪一条"——那种情况仍然要清：
		// 留一条点不动的审批条，比多清一次更糟。
		requestID := ""
		if req, err := apiclient.ParseApprovalRequest(ev.Data); err == nil {
			requestID = req.RequestID
		}
		m.mu.Lock()
		cur := m.approvals[id]
		matched := cur != nil && (requestID == "" || cur.RequestID == requestID)
		if matched {
			delete(m.approvals, id)
			if id == m.cur {
				m.approval = nil
			}
		}
		if m.seqs == nil {
			m.seqs = map[string]int{}
		}
		if ev.Seq > m.seqs[id] {
			m.seqs[id] = ev.Seq
		}
		isCur := id == m.cur
		m.mu.Unlock()
		if matched && isCur {
			m.setStatus("审批已超时（宿主已作废这条请求，按拒绝处理）")
			log.Printf("审批作废 kind=%s requestId=%s", ev.Kind, requestID)
		}
		m.inv.Invalidate()
		return
	}

	// 审批帧不是正文流；按会话保存完整原始 requestId/options，切回直接恢复，
	// 不需要靠重订让宿主重推，也绝不会在 B 的前台显示 A 的请求。
	if ev.Kind == "approval/request" {
		req, err := apiclient.ParseApprovalRequest(ev.Data)
		if err != nil {
			log.Printf("审批帧解析失败（忽略）：%v", err)
			return
		}
		m.mu.Lock()
		approval := &approvalReq{
			RequestID: req.RequestID,
			Tool:      req.Tool(),
			Title:     req.ToolCall.Title,
			Kind:      req.ToolCall.Kind,
			Args:      req.Args(),
			Cmd:       req.Command(),
			Detail:    req.Detail(),
			Options:   req.Options,
		}
		if m.approvals == nil {
			m.approvals = map[string]*approvalReq{}
		}
		m.approvals[id] = approval
		if m.seqs == nil {
			m.seqs = map[string]int{}
		}
		if ev.Seq > m.seqs[id] {
			m.seqs[id] = ev.Seq
		}
		isCur := id == m.cur
		if isCur {
			m.approval = approval
			m.lastSeq = maxInt(m.lastSeq, ev.Seq)
		}
		m.mu.Unlock()
		if isCur {
			log.Printf("待审批 requestId=%s tool=%s options=%d", req.RequestID, req.Tool(), len(req.Options))
		}
		m.inv.Invalidate()
		return
	}

	m.mu.Lock()
	state := m.streamStateLocked(id)
	cache := m.cachedSessionLocked(id)
	turnID, turnNumber := eventTurnIdentity(ev)
	if turnID != "" || turnNumber != nil {
		state.activeTurnID = turnID
		state.activeTurn = turnNumber
	}
	if m.seqs == nil {
		m.seqs = map[string]int{}
	}
	if ev.Seq > m.seqs[id] {
		m.seqs[id] = ev.Seq
	}
	if id == m.cur && ev.Seq > m.lastSeq {
		m.lastSeq = ev.Seq
	}
	if stoppedEvent(ev) && !ev.End() {
		// 显式 stopped/interrupted 先落已有部分正文；取消按钮本身不产生此帧。
		end := ev
		end.Kind = "turn/end"
		if state.stream.Apply(end) {
			cache.msgs = apiclient.Merge(cache.msgs, &state.stream)
			state.text = ""
			state.stream.Reset()
			state.busy = false
		}
	}
	durableUserText := ""
	if state.stream.Apply(ev) {
		if ev.Kind == "user/message" {
			durableUserText = state.stream.Text()
		}
		before := len(cache.msgs)
		cache.msgs = apiclient.Merge(cache.msgs, &state.stream)
		if added := len(cache.msgs) - before; added > 0 {
			cache.total += added
		}
		if cache.total < len(cache.msgs) {
			cache.total = len(cache.msgs)
		}
		cache.at = time.Now()
		if id == m.cur {
			m.syncRightbarLocked()
		}
		if streamDebug > 0 && state.stream.Thinking() != "" && state.stream.Text() == "" {
			streamDebug--
			log.Printf("mux 思考增量（累计 %d 字）", len([]rune(state.stream.Thinking())))
		}
		if state.stream.Done() {
			state.text = ""
			state.stream.Reset()
			state.busy = false
			state.activeTurnID = ""
			state.activeTurn = nil
		} else {
			state.text = state.stream.Text()
		}
	}
	if ev.End() || stoppedEvent(ev) {
		state.busy = false
		state.activeTurnID = ""
		state.activeTurn = nil
	}
	m.recordTurnEventsLocked(id, ev)
	m.reconcileOutgoingLocked(id, durableUserText, turnID)
	m.publishSessionLocked(id)
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
		// 原始帧跟着温缓存一起淘汰：两者都是"这一会话的首屏内容"，留一份不留
		// 另一份只会让内存涨到没边（长会话一帧约 200 B，十条会话就是几十 MB）。
		// 淘汰之后重开走冷载，`refreshFrames` 会重新拉回来——与 msgs 同款语义。
		delete(m.frames, oldestID)
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
	for i := len(m.outgoing[m.cur]) - 1; i >= 0; i-- {
		if m.outgoing[m.cur][i].State == pubapi.OutgoingFailed {
			key := m.outgoing[m.cur][i].Key
			m.mu.Unlock()
			m.retrySend(key)
			return
		}
	}
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

func (m *model) sendText(text string) { m.sendLocal(text) }

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
var msgsDebug = 0 // -debug 时置为 N：打 N 条 load 诊断（宿主 tail/total 与覆盖前后条数）
var streamDebug = 3

var (
	winTitle string
	shotPath string
	theme    ui.Theme // 由 dscore 的令牌算出来，启动时赋值（见 main）
	// themeExplicit：用户显式给了 -theme。给了就不再被 ui-config 里的 ui.theme 覆盖——
	// 命令行是"这一次我要这样"，配置文件是"上次我选过这样"，前者优先。
	themeExplicit bool
	renderLimit   int    // 0 = 不限制
	forceSession  string // 非空则启动后直接打开
	tailCount     int    // session.load 的 tailMessages
	wsDown        bool   // WS 订阅当前连不上（Snapshot.Disconnected / Link == LinkDown）
	// wsUp 是"WS 订阅已经建起来"的**唯一成功证据**。与 wsDown 相反方向：
	// 两个都为假 = 壳还没拿到任何连接事实 = `LinkUnknown`（界面必须写「未知」，
	// 不许写「已连接」）。写入点只有 runSubscription 的两处：订阅成功 / 订阅失败，
	// 都在 m.mu 里写，读数也在 m.mu 里取（-race 干净是这个仓的门禁）。
	wsUp bool
	// wsConnecting 是"有一次订阅尝试正在飞行中"（首连 / 退避到点 / 用户点立即重连）。
	//
	// 它是官方 `ConnectionIndicator` 的 `connecting` 态的唯一事实来源：真实
	// apiclient 的 `Mux.Subscribe` 是同步调用（拨号 + 握手），调用前写 true、
	// 拿到结果（成功或失败）立刻写 false。**不许**拿"还没结果"当"连着"。
	//
	// 三个事实（wsUp / wsDown / wsConnecting）同一把锁里写、同一把锁里读，
	// 由 `linkStateOf` 压成一个 `LinkState`（顺序见那个函数）。
	wsConnecting bool
	uiConfig     map[string]string // 宿主 config.get 的缓存（插件同步读，壳后台刷新）
	demoTool     bool              // 在消息列表里插假的工具卡
	smokeSend    bool              // 启动后自动发一条
	promptText   string            // -prompt 覆盖 smoke 文本
)

// drawSlot 把一个槽位上的插件按序画出来。
//
// 槽位空着就什么都不画——壳不需要知道哪个槽有人挂，也就不会因为少装一个
// 插件而画出一块空白边框。
// drawSlot 画一个槽位。
//
// 壳里只剩这一句：槽位树（谁在谁里面）由插件自己声明（Painter.Children），
// 壳不知道有几层、也不知道哪一层是什么部件——这就是「高塔」。
func drawSlot(f ui.C, k *uikernel.Kernel, slot pluginapi.Slot, env *pluginapi.Env) {
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
	// 分隔符未必是 1 字节：`×` 是 UTF-8 里的 2 字节（U+00D7）。
	// 早先写死 `s[i+1:]`，于是 `1024×768` 这种"文档明说支持"的写法
	// **解析失败**——注释和实现对不上，是单测把这条抓出来的。
	_, sepLen := utf8.DecodeRuneInString(s[i:])
	if i+sepLen >= len(s) {
		return 0, 0, false
	}
	w, err1 := strconv.Atoi(strings.TrimSpace(s[:i]))
	h, err2 := strconv.Atoi(strings.TrimSpace(s[i+sepLen:]))
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
	fields := []*ui.Field{&m.draft}
	for _, p := range m.kernel.Plugins() {
		if withFields, ok := p.(pluginapi.Fields); ok {
			fields = append(fields, withFields.Fields()...)
		}
	}
	// 官方 `ui-attachment` 的「拖入即添加」：Viva 的系统拖放事件（原生目标给绝对路径）。
	// 插件自己拿不到窗口配置，所以由壳转发；返回它是否消费了这次事件。
	cfg.OnFileDrop = func(ev gfx.FileDropEvent) gfx.EventResult { return dsattachment.HandleFileDrop(ev) }
	configureFrameWindow(&cfg, runtime.GOOS == "windows", theme)
	cfg.TextInput = m.chrome.wrapTextInput(ui.Fields(fields...))
	m.chrome.newSession = m.newSession
	m.chrome.toggleRightbar = m.toggleRightbar

	cfg.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		if runtime.GOOS == "windows" {
			if id, ok := canvas.ClickTarget(); ok {
				if err := m.chrome.handleClick(id); err != nil {
					m.setStatus("菜单动作失败: " + err.Error())
				}
			}
		}
		// 主题：**在这一帧开画之前**消费排队的那一个。
		// 放这里是因为它写包级 `theme` 变量，而只有绘制线程可以写。
		m.applyPendingTheme()
		// 草稿写回同理：Viva 的 `*flex.Field` 是 UI 线程状态，后台 goroutine 只排队。
		m.applyPendingDraft()
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)
		contentBounds := bounds
		if runtime.GOOS == "windows" {
			chromeLayout := m.chrome.paintCaption(canvas, theme, size)
			contentBounds = chromeLayout.Content
			m.mu.Lock()
			m.chromeLayout = chromeLayout
			m.mu.Unlock()
		}

		m.mu.Lock()
		sessions := append([]apiclient.Session(nil), m.sessions...)
		cur := m.cur
		msgs := append([]apiclient.Message(nil), m.msgs...)
		metadata := map[string]pubapi.MessageMetadata{}
		for key, value := range m.msgMeta[cur] {
			metadata[key] = value
		}
		turnEvents := append([]pubapi.TurnEvent(nil), m.turnEvents[cur]...)
		outgoing := append([]pubapi.OutgoingMessage(nil), m.outgoing[cur]...)
		total := len(msgs)
		truncated := false
		if cache := m.cache[cur]; cache != nil {
			total = cache.total
			truncated = cache.truncated
			if total < len(msgs) {
				total = len(msgs)
			}
		}
		var activeTurnID string
		var activeTurn *int64
		if state := m.streamStates[cur]; state != nil {
			activeTurnID = state.activeTurnID
			activeTurn = cloneTurnNumber(state.activeTurn)
		}
		status := m.status
		streamText := m.streamText
		// 主区 transcript 的数据面：原始 journal 帧 + 世代。
		// 锁内拷贝（journal 会被 WS goroutine 插入，活切片不能交出去）。
		frames, framesEpoch := m.snapshotFramesLocked(cur)
		// 向前分页状态（loadOlder 五态）：同样是锁内拷贝的**值**，
		// 插件据此画 `.older` 那一行（还有没有更早 / 在拉 / 失败）。
		historyPaging := m.snapshotHistoryPagingLocked(cur)
		approval := m.approval
		usage := m.usage
		// H1/H4/H5：会话语义的真值原样带进快照，插件自己决定画不画。
		usageTotal := m.usageTotal
		contextTokens := m.contextTokens
		contextWindow := m.contextWindow
		modelName := m.modelName
		permission := m.permission
		showRightbar := m.showRightbar
		if renderLimit > 0 && len(msgs) > renderLimit {
			status = fmt.Sprintf("%s（渲染末尾 %d / %d）", status, renderLimit, len(msgs))
			msgs = msgs[len(msgs)-renderLimit:]
			truncated = true
		}
		m.mu.Unlock()

		// 主区宽度：壳算好交给插件，插件不该猜（侧栏与右栏的宽度只有壳知道）。
		//
		// 右栏**不能写死 300**：真实宽度是 geomRightWidth 算出来的（这个尺寸下
		// 1180 宽的窗口右栏占 500），写死 300 会让插件以为主列比实际宽 200px，
		// 于是按 600 排版、真主列只有 ~495，内容被卡片硬裁（省略号都不画）。
		// 症状是审批条/长参数的末尾直接被切掉。
		//
		// 每帧按**这一帧的** bounds 重算：buildGeometry 是纯函数，没有可残留的
		// 缓存，所以 resize 之后不会沿用上一帧的列宽（真机三档见 tmp/v4b-shots）。
		hasRightbar := showRightbar && m.kernel.Has(pluginapi.SlotRightbar)
		m.chrome.rightbarOpen = showRightbar
		// 结构档位：auto 在内核有 shell.root 占用方时仍走 dsshell 旧三栏。
		shellTier := m.shellMode.resolve(m.kernel)
		var shellOpts uikernel.ShellOptions
		var geom pluginapi.Geometry
		if shellTier == shellOfficial {
			// 官方骨架自己解三列；Geometry 与真画出来的必须是同一份结果
			//（旧代码 buildGeometry 与 dsshell 各算一遍，分叉过一次）。
			shellOpts = shellOptionsFor(m, m.kernel, contentBounds.Width, contentBounds.Height)
			geom = officialGeometry(contentBounds.Width, contentBounds.Height, shellOpts)
		} else {
			geom = buildGeometry(contentBounds.Width, contentBounds.Height, m.side.Width(), hasRightbar,
				m.kernel.Has(pluginapi.SlotShellBottom))
		}
		// 官方骨架在旧部件还占槽时会整段换成旧三栏排版（`BridgeLegacy`）：
		// 那时官方子槽（sidebar.settings / conversation.session.header.actions）
		// **不会被 dispatch**，旧槽（sidebar.footer / main.header）会。插件必须知道
		// 这一帧到底该挂哪边，否则会把画挂在永远不 dispatch 的槽上（2026-11-12 真机
		// `-shell official` 下设置启动器入口不显示，就是这么来的）。
		{
			bridgeLegacy := shellTier == shellLegacy
			if shellTier == shellOfficial {
				bridgeLegacy = shellOpts.BridgeLegacy
			}
			m.wave3.tier.SetLegacyBridge(bridgeLegacy)
		}
		mainWidth := geom.MainWidth

		// 一帧一份快照：锁内拷好，插件里不用加锁，也就没法在绘制期发请求。
		m.mu.Lock()
		busy := m.turnBusy
		// 连接三态与 busy 同一把锁里取：runSubscription 写 wsUp/wsDown 也在锁里，
		// 分开取会在 `-race` 下变成真的数据竞争。
		link := linkStateOf(wsUp, wsDown, wsConnecting)
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
		// 本帧消息流滚动区：插件在声明后交回来（见 env.ScrollBox）。
		// Box 只在同一个 paint 内有效，所以这里只用一帧、用完即弃。
		var scrollBox ui.Box
		var hasScrollBox bool
		env := &pluginapi.Env{
			Theme: m.theme,
			Data: pluginapi.Snapshot{
				Sessions: sessions, Cur: cur, Status: status,
				Msgs: msgs, Frames: frames, JournalEpoch: framesEpoch,
				// 向前分页五态：**只在"确实开过会话"时才有意义**；
				// 零值（Baseline/HasMore 全 false、无错）界面不画那一行。
				History: pluginapi.HistoryPaging{
					Baseline: historyPaging.Baseline,
					HasMore:  historyPaging.HasMore,
					Partial:  historyPaging.Partial,
					Loading:  historyPaging.Loading,
					Error:    historyPaging.Error,
				},
				Total: total, Stream: streamText, Truncated: truncated,
				MessageMeta: metadata, TurnEvents: turnEvents, Outgoing: outgoing,
				ActiveTurnID: activeTurnID, ActiveTurn: activeTurn,
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
				ContextWindow: contextWindow,
				// whole-log `sessionStats` 投影（状态栏「活动」的唯一来源）。
				// nil = 还没拉到 / 老宿主没有这个键 → 写「未知」，不拿 Msgs 凑。
				SessionStats: m.wave3.sessionStatsSnapshot(cur),
				// whole-log `tokenUsage` 投影四桶（用量胶囊 + 用量详情弹窗的唯一来源）。
				// nil = 还没拉到 / 老宿主没有这个键 → 写「未知」，不拿 UsageTotal 反推。
				TokenUsage:   m.wave3.tokenUsageSnapshot(cur),
				Model:        modelName,
				Permission:   permission,
				Approval:     approvalView,
				Busy:         busy,
				ShowRightbar: showRightbar,
				// 连接是一等事实：三态由壳按真实 apiclient 订阅结果写
				// （订阅成功 = LinkConnected / 失败 = LinkDown / 还没拿过 = LinkUnknown）。
				// 插件据此画官方 `ConnectionIndicator`；`LinkUnknown` 时写「未知」，
				// **不许**写「已连接」——那是"没有证据的断言"。
				Link: link,
				// 兼容旧字段：`Disconnected` 恒等于 `Link == LinkDown`，
				// 由同一个 link 推出，不会与 Link 自相矛盾。
				Disconnected: link == pluginapi.LinkDown,
				// 当前会话的工作目录：插件拼文件路径的前提，空串=不知道。
				CWD: m.cwd,
				// 桌面更新指示（官方设置行第 3 格 `DesktopUpdateIndicator`）的事实。
				// nil = 官方没有 updates 桥（本仓宿主没接 `desktopUpdate.status`，
				// 或还没问到）⇒ 按官方 idle 规则整枚不渲染。见 desktop_update_wiring.go。
				DesktopUpdate: m.desktopUpdateSnapshot(),
			},
			Act:       m.actions,
			Tree:      &tree,
			Feat:      home,
			Canvas:    canvas,
			Bounds:    contentBounds,
			MainWidth: mainWidth,
			// v0.6.0 没有"按 ID 查滚动状态"的入口了：滚动区状态由声明方
			// （消息流插件）持在 Box 上，且只在同一个 paint 内有效。所以改成
			// 声明方把盒子交回来，壳在 Draw 返回后（布局已定）立刻用完就丢。
			ScrollBox: func(box ui.Box) { scrollBox, hasScrollBox = box, true },
		}
		env.Has = func(slot pluginapi.Slot) bool { return m.kernel.Has(slot) }
		// 三栏几何由壳算：它知道侧栏要的宽度、右栏开不开、白名单里有没有底部整行。
		// 框架插件照着摆，不再自己猜（猜错过：白名单里漏掉 shell，部件就上下堆叠）。
		// 与上面 env.MainWidth 用的是**同一份**求解结果，不允许两处各算一遍。
		env.Geometry = geom

		// 两个父容器的每帧输入：账号态（通知入口可见性）+ 会话投影（子代理目录）。
		// 只喂缓存、不发请求；拉取按节流在后台（见 wave3_wiring.go）。
		{
			m.mu.Lock()
			curForWave3 := m.cur
			m.mu.Unlock()
			m.wave3.feed(curForWave3, geom.SidebarWidth)
			if accountDue, projDue := m.wave3.refreshDue(m, curForWave3); accountDue || projDue {
				if accountDue {
					go m.wave3.fetchAccount(m)
				}
				if projDue {
					go m.wave3.fetchProjections(m, curForWave3)
				}
			}
		}

		m.view.Draw(canvas, contentBounds, func(f ui.C) {
			if shellTier == shellOfficial {
				// 官方骨架：结构/槽名/几何来自 internal/uikernel，内容仍是插件。
				drawOfficialFirstScreen(f, m, env, shellOpts)
				return
			}
			// 旧路径：壳只画一个槽，三栏怎么切全在 plugins/dsshell 里。
			drawSlot(f, m.kernel, pluginapi.SlotShellRoot, env)
		})

		// 布局已定：先恢复待恢复的滚动位置，再记本帧偏移，然后跑插件的 AfterDraw
		// （插件负责 Markdown 这类"先布局后覆画"的东西，壳不再画内容）。
		//
		// **顺序不能反**：recordScroll 会把本帧的 Offset 写进 m.scrollPos，
		// 而切会话那一帧的新滚动区是从 0 布局的——先记再恢复就会把"切走前的位置"
		// 覆盖成 0，恢复等于没恢复（这条有专门的回归测试：
		// TestPendingScrollRestoresOffset）。
		//
		// 第一批消息的高度/尾部 Offset 刚确定，必须再绘一帧让虚拟化实画尾部。
		if m.settleScroll(scrollBox, hasScrollBox) {
			m.inv.Invalidate()
		}
		for _, painter := range m.kernel.AllPainters() {
			if painter.AfterDraw != nil {
				painter.AfterDraw(canvas, env)
			}
		}
		if runtime.GOOS == "windows" {
			m.chrome.paintPopup(canvas, theme, size)
		}
	}
	// 窗口变了必须全屏重绘：Viva 只重画变化区域，缩小后旧帧会残留在边缘。
	// 键盘：先给插件（↑↓ 切会话、Esc 拒审批、Ctrl+Shift+C 复制…），
	// 插件都不要才算壳的。壳里因此**没有任何具体快捷键的判断**。
	// 滚轮：**不接就是 bug，不是"没测的性能项"**。
	//
	// viva 的窗口过程长这样（window_windows.go）：
	//     case wmWheel: if OnScroll != nil { ... }; return 0
	// 也就是 OnScroll == nil 时 WM_MOUSEWHEEL 被**直接吞掉**，连 canvas 的
	// 默认偏移都没有。所以真机上鼠标滚轮滚不动消息列表（只能 ↑↓ 或自动跟随），
	// 而 PostMessage 测滚动也永远量不到东西——不是事件没到，是到了没人接。
	//
	// 接到消息区的 ScrollState（它自己管跟随与边界），会话 id 用当前会话。
	// 用 viva 自带的 `View.Wheel`：它按**指针位置**找最内层能滚的 ScrollArea，
	// 滚到头再往外层找，行为跟浏览器一致。自己拿当前会话的 ScrollState 去 Wheel
	// 只覆盖"消息区"一个区域，侧栏/轨迹栏在指针下时就不该滚消息区。
	cfg.OnScroll = func(ev gfx.ScrollEvent) gfx.EventResult {
		for _, plugin := range m.kernel.Plugins() {
			if handler, ok := plugin.(pubapi.ScrollHandler); ok {
				if result := handler.HandleScroll(ev); result.Handled {
					return result
				}
			}
		}
		return m.view.Wheel(ev)
	}

	cfg.OnKey = func(ev gfx.KeyEvent) gfx.EventResult {
		if ev.Release {
			return gfx.EventResult{}
		}
		if runtime.GOOS == "windows" {
			if result := m.chrome.handleKey(ev); result.Handled {
				m.inv.Invalidate()
				return result
			}
		}
		if m.kernel.HandleKey(ev) {
			// **必须显式 Invalidate**：viva 的 EventResult.Repaint 不足以让
			// shell.overlay 里的画重新执行。实测：F1 把插件状态切成 shown=true，
			// 但之后一帧都没再画，面板永远不出现（key 16 handled=true 却看不见东西）。
			m.inv.Invalidate()
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		return gfx.EventResult{}
	}

	cfg.OnResize = func(ui.Size) { m.inv.Invalidate() }
	return cfg
}

// parseKeys 把 "down,shift+f2" 翻成按键事件（只认导航键，够验证路由用）。
//
// 为什么返回 KeyEvent 而不是 ui.Key：**修饰键也是路由的一部分**。
// 设置浮层的入口是 Shift+F2，而普通 F2 是命令面板——只翻键名会把 Shift 丢掉，
// 注入的按键就永远打不开设置，而"打不开"看起来会像"插件没装/没接上"。
func parseKeys(seq string) []gfx.KeyEvent {
	var out []gfx.KeyEvent
	for _, raw := range strings.Split(seq, ",") {
		name := strings.TrimSpace(strings.ToLower(raw))
		var mods gfx.Modifiers
		if rest, ok := strings.CutPrefix(name, "shift+"); ok {
			mods, name = gfx.ModifierShift, rest
		}
		key, ok := keyFromName(name)
		if !ok {
			continue
		}
		out = append(out, gfx.KeyEvent{Key: key, Modifiers: mods})
	}
	return out
}

// keyFromName 只认 `-keys` 用得上的键名（不是完整的键表）。
//
// 加插件键位的同一条规矩：插件吃哪个键，这里就得认识哪个名字，
// 否则截图时按键静默无效（F5 加的时候踩过）。
func keyFromName(name string) (gfx.Key, bool) {
	switch name {
	case "down":
		return gfx.KeyDown, true
	case "up":
		return gfx.KeyUp, true
	case "f1":
		return gfx.KeyF1, true
	case "f2":
		return gfx.KeyF2, true
	case "f5":
		// 终端面板（dsterminal）。
		return gfx.KeyF5, true
	case "f6":
		return gfx.KeyF6, true
	case "f7":
		return gfx.KeyF7, true
	case "f8":
		return gfx.KeyF8, true
	case "f12":
		return gfx.KeyF12, true
	case "home":
		return gfx.KeyHome, true
	case "end":
		return gfx.KeyEnd, true
	case "esc":
		return gfx.KeyEscape, true
	case "enter":
		// 审批条可见时 Enter = 允许（dscomposer 的 HandleKey 只在可见时接管）。
		// 加它不是为了别的：验「允许之后宿主真的执行工具」必须有办法按下允许。
		return gfx.KeyEnter, true
	}
	return 0, false
}

// fixedFirst 返回必须常驻的结构性插件。
//
// 白名单模式下列没列它们也要装：ds-core 是所有插件的 Requires，ds-shell 负责切三栏。
// 这一步直接在壳里实例化，因为白名单可能把它们漏掉，而它们是地基不是部件。
func fixedFirst(configPath, themeName string) []pluginapi.Plugin {
	// 无论哪个模式都要带上：skip 把它们从注册表结果里排掉了（壳自己注入实例）。
	// 全装模式下漏掉 ds-core，所有需要 ui.theme 的插件都会装配失败（实测）。
	// 主题由 -theme 决定；它必须在装配**之前**定，因为 ds-core 是第一个被装的插件，
	// 后面所有插件都从它 Inject 颜色令牌——装完再换就等于没换。
	return []pluginapi.Plugin{&dscore.Plugin{Dark: themeName != "light"}, &dsshell.Plugin{}}
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

// buildGeometry 把这一帧的三栏几何算成一份 pluginapi.Geometry（纯函数）。
//
// 为什么要单独一个函数：这三行以前内联在 Paint 闭包里，唯一的验证方式是起窗口
// 截图量像素，"传下去的值"对不对只能靠看。抽出来以后 resize 各档的**传入值**
// 可以直接单测（TestGeometryMatchesOfficialColumns），
// 而且纯函数顺带保证了 resize 安全——值只取决于这一帧的 bounds，
// 没有任何能跨帧残留的状态。
//
// width/height 是**客户区**尺寸（跟 pluginapi.Geometry 的语义一致），
// 不是整窗：Viva 的 Paint 给的 size 已经是客户区，别再自己减标题栏。
func buildGeometry(width, height, sidebar float32, hasRightbar, hasBottom bool) pluginapi.Geometry {
	// 照搬官方 computeColumns：**右栏放不下就整列消失**。
	//
	// 早先是 geomRightWidth：中栏被压到 400 以下就把右栏硬压到 300 硬塞。
	// 结果是 960 宽时中栏掉到 389、右栏 291——正是官方明确不允许的状态
	// （dsshell 一侧已按官方实画 0，这里不改就会让 Geometry 传出去的值
	// 和实际画的列宽对不上：壳说右栏 300、dsshell 画 0）。
	right := float32(0)
	if hasRightbar {
		if avail := width - sidebar - 400; avail >= 300 {
			right = width * 0.45
			if right > avail {
				right = avail
			}
			if right > width*0.7 {
				right = width * 0.7
			}
			if right < 300 {
				right = 300
			}
		}
	}
	main := width - sidebar - right
	if main < 0 {
		main = 0
	}
	return pluginapi.Geometry{
		Width:        width,
		Height:       height,
		SidebarWidth: sidebar,
		RightWidth:   right,
		MainWidth:    main,
		BottomHeight: condF(hasBottom, 24, 0),
	}
}

// condF 是三目（Go 没有，写个小函数比每处写 if 清楚）。
func condF(ok bool, a, b float32) float32 {
	if ok {
		return a
	}
	return b
}

// orderPlugins 按**按键优先级**排序（`Kernel.HandleKey` 按装配顺序转发，
// 第一个返回 true 的插件赢）。
//
//	ds-core       唯一"无依赖"的插件（提供 ui.theme / ui.primitives）。
//	              它必须在最前：`Load` 按轮次装配，排在 core 前面的插件第一轮
//	              "依赖还没就位"会被推到下一轮——想靠 prepend 把某个插件提前，
//	              反而会把它推到最后一轮。
//	ds-settings   设置浮层（Shift+F2 / 浮层开着时的 Esc）
//	ds-statusbar  状态栏（Activity 弹窗开着时的 Esc）
//	ds-shell      切三栏（它跟别的部件没有依赖关系，只画槽位）
//	其余          按传入顺序
//
// 为什么 settings/statusbar 必须早于 dscomposer：composer 在没有审批时把 Esc
// 当"停止当前 turn"，排在它后面的话，跑 turn 时按 Esc 只会停 turn、弹窗关不掉
// （HH 单第 5 件）。官方是 document 级监听，两件事互不抢。
func orderPlugins(ps []pluginapi.Plugin) []pluginapi.Plugin {
	priority := []string{"ds-core", "ds-settings", "ds-statusbar", "ds-shell"}
	out := make([]pluginapi.Plugin, 0, len(ps))
	placed := make(map[string]bool, len(ps))
	for _, id := range priority {
		for _, p := range ps {
			if p.ID() != id || placed[id] {
				continue
			}
			placed[id] = true
			out = append(out, p)
		}
	}
	for _, p := range ps {
		if !placed[p.ID()] {
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
	// `-h` 打的是产品名：否则人对着 "Usage of app.exe" 还得猜这是哪个程序。
	// 产物名不用再重复一遍——它**就是**产品名加 `.exe`（brand.go 的 binaryName），
	// 再打一次只会得到「DSH GUI GO dev（DSH GUI GO.exe）」这种同义反复。
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "%s %s\n\n参数：\n", productName, version)
		flag.PrintDefaults()
	}
	// --version：只读打印一行然后退出。它是**查身份**用的（哪个产物、哪一版），
	// 不能反过来去连宿主、建窗口——那种"顺手做点事"的版本查询会让脚本多等几十秒。
	showVersion := flag.Bool("version", false, "只打印版本（"+productName+" <版本>）然后退出")
	pluginConfig := flag.String("plugins", "", "插件配置文件（JSON 白名单；空=全装）")
	addr := flag.String("addr", "http://127.0.0.1:7788", "Transport base URL")
	limit := flag.Int("limit", 0, "只渲染最后 N 条消息（0 = 全部）；用于量视口虚拟化的收益")
	sess := flag.String("session", "", "启动后直接打开这个会话 id")
	openFile := flag.String("open-file", "", "无头截图用：启动后直接打开这个文件名的预览（走 dsfiles.Open，不模拟鼠标）")
	openSettings := flag.Bool("open-settings", false, "无头截图用：启动后直接打开设置浮层（走 dssettings.Open，不模拟热键）；产品入口是 Shift+F2")
	shellTierFlag := flag.String("shell", "auto", "首屏结构档位：auto（默认；有 shell.root 占用方时走旧三栏）/ legacy / official（官方骨架）")
	themeName := flag.String("theme", "dark", "主题：dark / light / system（system 跟着操作系统的明暗走）")
	tail := flag.Int("tail", 40, "session.load 拉多少条尾巴（实测每条消息约 0.46 MB 纹理，40 条约 100 MB）")
	smoke := flag.Bool("smoke", false, "启动后自动发一条消息，验证 send 链路（不做按键模拟）")
	prompt := flag.String("prompt", "", "调试：-smoke 时发这条文本（默认 smoke-<时间>）；验工具卡/审批用")
	cookie := flag.String("cookie", "", "调试：直接用这条会话 cookie（形如 dshd_session=…），跳过 /connect。token 一次性时用来与 curl 共用同一个宿主")
	demo := flag.Bool("demo", false, "在消息列表里插一条假的工具卡，验证渲染")
	title := flag.String("title", productName, "窗口标题（并行跑多个实例时用来区分；默认就是产品名）")
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
	rightbarOn := flag.Bool("rightbar", false, "启动时是否默认展开右栏；工具历史不会自动展开，用户选择按会话记忆")
	token := flag.String("token", "", "宿主 launch token：给了就先 /connect 换 cookie（真宿主 dshd serve 必须给）")
	searchAtStart := flag.String("search", "", "调试：启动就把会话内搜索打开，并带上这个词（截搜索态用）")
	shotWait := flag.Int("shot-wait", 4, "无头抓图前等多少秒（流式/发送类验证要给大一点）")
	shot := flag.String("shot", "", "抓一张窗口图存到这个路径然后退出（窗口会挪到屏幕外，不弹给用户）")
	chromeReport := flag.String("chrome-report", "", "与 -shot 同用：把真实 Paint 的 caption/菜单几何写成 JSON")
	chromeProbe := flag.Bool("chrome-probe", false, "与 -shot 同用：点击本进程窗口控制并断言 OS 状态，最后从关闭钮退出")
	winSize := flag.String("size", "", "调试：窗口初始尺寸 WxH（如 1600x900）；空 = 1180x800。验证几何在不同宽度下是否成立")
	flag.Parse()
	shellTier, err := parseShellMode(*shellTierFlag)
	if err != nil {
		log.Fatalf("%v", err)
	}
	// 版本查询排在所有参数校验之前：`"DSH GUI GO.exe" --version` 不管别的参数写成什么，
	// 都只该打印一行就退（退出码 0），否则脚本里那句"先确认产物是哪一版"会被
	// 别的 flag 的合法性检查带成判红。
	if *showVersion {
		fmt.Printf("%s %s\n", productName, version)
		return
	}
	if (*chromeProbe || *chromeReport != "") && *shot == "" {
		log.Fatal("-chrome-probe / -chrome-report 必须与 -shot 同用")
	}
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
		cache:            map[string]*cachedSession{},
		seqs:             map[string]int{},
		epochs:           map[string]int64{},
		mux:              apiclient.NewMux(*addr),
		scrollPos:        map[string]float32{},
		showRightbar:     *rightbarOn,
		rightbarDefault:  *rightbarOn,
		rightbarSessions: map[string]rightbarSessionState{},
		outgoing:         map[string][]pubapi.OutgoingMessage{},
		msgMeta:          map[string]map[string]pubapi.MessageMetadata{},
		turnEvents:       map[string][]pubapi.TurnEvent{},
		approvals:        map[string]*approvalReq{},
		streamStates:     map[string]*sessionStreamState{},
		shellMode:        shellTier,
		// GUI 第三波两个父容器的壳侧数据接线（见 wave3_wiring.go）。
		wave3: newWave3Wiring(),
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
	m.settings = &dssettings.Plugin{}
	m.statusbar = &dsstatusbar.Plugin{}
	// ds-core（令牌 + 图元）与 ds-shell（三栏）是结构性的：它们不在白名单里也要装，
	// 否则其余部件没有令牌可 Inject、或者各自独占整个窗口。
	// 这两条不许被 ui-plugins.json 关掉——配置管的是"装哪些部件"，不是"要不要有地基"。
	m.kernel = uikernel.New(productName)
	// 壳侧能力必须在 Load **之前**给出：插件的 Register 里就会 ctx.Inject。
	//	ui.shell.tier   壳档位（official/legacy）的共享持有者
	//	ui.apiclient    壳唯一的传输客户端（第三波两个容器的数据/写面）
	m.wave3.provide(m.kernel, m.cli)

	// 插件从哪来：**壳不认识任何具体插件**。
	// 内置的自己在 init 里 pluginapi.Register 报到（blank import 即接入）；
	// 装哪些可以由 <home>/ui-plugins.json 限量（白名单 / enabled:false）。
	// 需要壳注入运行期状态的少数几个（侧栏实例、搜索开关）在这里补。
	plugins, err := uiplugins.Load(*pluginConfig, pluginapi.Builtin(), func(id string) bool {
		// 这两个实例由壳自己 append（壳要读它们的 Fields/Width/搜索开关）。
		return id == "ds-sidebar" || id == "ds-conversation" || id == "ds-core" || id == "ds-shell" || id == "ds-settings" || id == "ds-statusbar"
	})
	if err != nil {
		log.Fatalf("插件来源加载失败: %v", err)
	}
	plugins = append(plugins, m.side, &dsconversation.Plugin{
		ExpandAllThinking: *expandThink,
		SearchOpen:        *searchAtStart != "",
		SearchQuery:       *searchAtStart,
	})
	// ds-shell 必须**排在 ds-core 之后、所有部件之前**：它负责切三栏，
	// 少了它，其余部件会各自独占整个窗口（实测：白名单里漏掉 ds-shell，
	// 侧栏和消息流就上下堆在一起了）。顺序由下面的 `orderPlugins` 统一排
	// （`ds-core → ds-settings → ds-statusbar → ds-shell → 其余`）——
	// 用 prepend 提没有用：`Load` 按轮次拓扑装配，排在 core 前面的插件
	// 第一轮"依赖还没就位"会被推到最后一轮（HH 单第 5 件踩的就是这个）。
	// 主题模式：`system` 要在这里**解析成明/暗**再交给 dscore 插件——
	// 插件构造主题时只知道"深还是浅"，"跟着系统"是壳这一层的事。
	if !validThemeMode(*themeName) {
		log.Fatalf("--theme 只能是 dark|light|system，收到 %q", *themeName)
	}
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "theme" {
			themeExplicit = true
		}
	})
	_, resolvedTheme := resolveThemeMode(*themeName)
	plugins = append(plugins, fixedFirst(*pluginConfig, resolvedTheme)...)
	// 设置浮层排在**装配顺序最前**：按键是按装配顺序转发的，第一个返回 true 的赢。
	// 它只吃 Shift+F2 和"浮层开着时的 Esc/↑↓/Enter"，其余一律放行；
	// 排在最前才能保证浮层开着时 Esc 先关浮层，而不是被 composer 当成"停止 turn"。
	// 排在后面则相反：composer 先看到 Esc，浮层永远关不掉。
	// 两个"按键优先"的插件（设置浮层 / 状态栏弹窗）**不能靠 prepend 提前**：
	// `Load` 按轮次拓扑装配，prepend 到 ds-core 前面会让它们第一轮"依赖还没
	// 就位"、被推到最后一轮（HH 单第 5 件的真实原因）。顺序由 `orderPlugins`
	// 统一排：ds-core → ds-settings → ds-statusbar → ds-shell → 其余。
	plugins = append(plugins, m.settings, m.statusbar)
	if err := m.kernel.Load(orderPlugins(plugins)...); err != nil {
		log.Fatalf("插件装配失败: %v", err)
	}
	// 装配完成：取回两个容器插件的服务、写死壳档位、接好"打开账号设置"。
	m.wave3.attach(m)
	m.theme = m.kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	m.prims = m.kernel.MustGet(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	theme = dscore.VivaTheme(m.theme)
	// 整窗的 Page：一个 Page 活过所有帧（名字/作用域/滚动状态都挂在它身上），
	// 所以必须在这里建一次，不能在 Paint 里每帧新建。
	// Theme 传包级 `theme` 的地址 → applyPendingTheme 改 theme，下一帧跟着换。
	// Tree 给整窗的交互树（与 app.Config{Tree: &tree} 是同一棵）。
	m.view = ui.NewPage(ui.PageConfig{Tree: &tree, Theme: &theme})
	// 官方目录浏览器：把数据源与「打开后」的回调注入插件（见 dirpicker_wiring.go）。
	wireDirPicker(m)
	wireOfficialPanels(m)
	m.actions = pluginapi.Actions{
		// 宿主 ui-config.json 的读写。ConfigGet 给的是**缓存**（插件要同步返回），
		// ConfigSet 异步写、失败只报状态栏——插件不等待也不重试。
		ConfigGet: func() map[string]string {
			m.mu.Lock()
			defer m.mu.Unlock()
			return uiConfig
		},
		ConfigSet: func(kv map[string]string) {
			if len(kv) == 0 {
				return
			}
			m.mu.Lock()
			if uiConfig == nil {
				uiConfig = map[string]string{}
			}
			for k, v := range kv {
				uiConfig[k] = v
			}
			m.mu.Unlock()
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := m.cli.ConfigSet(ctx, kv); err != nil {
					m.setStatus("保存设置失败: " + err.Error())
				}
			}()
		},
		// 工作区根目录（一层）+ 读文件。**都是同步的**：插件在 draw 里调，
		// 让它等 HTTP 会阻塞首帧。所以列目录读缓存、读文件**由插件自己看情况调**
		// （它知道自己在画什么，比壳更该决定要不要阻塞）。
		FSList: func(dir string) []pluginapi.FSEntry {
			m.mu.Lock()
			if m.fsDirs == nil {
				m.fsDirs = map[string][]pluginapi.FSEntry{}
			}
			if m.fsPending == nil {
				m.fsPending = map[string]bool{}
			}
			names, ok := m.fsDirs[dir]
			pending := m.fsPending[dir]
			if !ok && !pending {
				m.fsPending[dir] = true
			}
			m.mu.Unlock()
			// 没拉到过 → 后台拉一次，拉到后重绘（插件下一帧就能拿到）。
			// **不在 draw 里等 HTTP**：那会让首帧卡住。
			if !ok && !pending {
				go m.loadDir(dir)
			}
			return names
		},
		// 官方目录选择器（「添加工作区」的落点）。nil 时侧栏不画那个按钮。
		OpenDirectoryPicker: func() { m.openDirectoryPicker() },
		ScheduleList: func() []pluginapi.ScheduleItem {
			m.mu.Lock()
			defer m.mu.Unlock()
			return m.schedules
		},
		// 官方 schedule.catalog / history / 三条写动作（见 schedule_catalog.go）。
		// 与上面的 ScheduleList 并存：那个是侧栏摘要，这个是官方任务页的数据面。
		ScheduleCatalog:     m.scheduleCatalog,
		ScheduleHistory:     m.scheduleHistory,
		ScheduleHistoryMore: m.scheduleHistoryMore,
		ScheduleDelete:      m.scheduleDelete,
		ScheduleSetActive:   m.scheduleSetActive,
		ScheduleRun:         m.scheduleRun,
		ScheduleActionError: m.scheduleActionError,
		ScheduleRefresh:     m.scheduleRefresh,
		// 后台任务：同步返回缓存。**首次问会触发一次后台拉取**——
		// 否则插件第一帧永远拿不到东西，而它自己没法申请刷新。
		JobsList: func() []pluginapi.Job {
			m.mu.Lock()
			jobs := m.jobs
			pending := m.jobsPending
			if !pending {
				m.jobsPending = true
			}
			m.mu.Unlock()
			if !pending {
				go m.refreshJobs()
			}
			return jobs
		},
		// 后台任务的两条真实动作（宿主 `jobs.output` / `jobs.stop`）。
		// 读走缓存 + 后台拉；停止不乐观更新（见 jobs_actions.go 顶部三条约束）。
		JobsOutput: m.jobOutput,
		JobsStop:   m.jobStop,
		// 模型清单（启动时拉一次，见 refreshList）；没拉到就返回 nil，
		// 插件据此不画"选模型"命令（画了也选不出东西）。
		ModelsList: func() []pluginapi.Model {
			m.mu.Lock()
			defer m.mu.Unlock()
			return m.models
		},
		// 模型覆盖：写宿主 <home>/state/session-model.json。
		// **空串 = 清除覆盖**，回到宿主默认。异步、失败只写状态栏。
		SetModel: func(modelID string) {
			m.mu.Lock()
			id := m.cur
			m.mu.Unlock()
			if id == "" {
				m.setStatus("没有当前会话，改不了模型")
				return
			}
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := m.cli.SetSessionModel(ctx, id, modelID); err != nil {
					m.setStatus("改模型失败: " + err.Error())
					return
				}
				if modelID == "" {
					m.setStatus("模型已恢复默认")
				} else {
					m.setStatus("模型已设为 " + modelID)
				}
			}()
		},
		// 目标条：读壳缓存。**同步**——它只有一行文本，不值得为它做轮询。
		Goal: func() string {
			m.mu.Lock()
			defer m.mu.Unlock()
			return m.goal
		},
		// 终端：一次会话一个。**读走缓存**（后台轮询填充），不在这里发 HTTP——
		// 插件每帧都会调 Read，同步发请求会卡住绘制。
		TerminalOpen: func() string {
			m.mu.Lock()
			id := m.termID
			m.mu.Unlock()
			if id != "" {
				return id
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			newID, err := m.cli.TerminalOpen(ctx, "")
			if err != nil {
				log.Printf("开终端失败: %v", err)
				m.setStatus("开终端失败: " + err.Error())
				return ""
			}
			log.Printf("终端已开: %s", newID)
			m.mu.Lock()
			m.termID, m.termText, m.termNext = newID, "", 0
			m.mu.Unlock()
			go m.refreshTerminal()
			return newID
		},
		TerminalInput: func(id, data string) {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := m.cli.TerminalInput(ctx, id, data); err != nil {
					m.setStatus("写终端失败: " + err.Error())
				}
			}()
		},
		TerminalRead: func(id string) string {
			m.mu.Lock()
			defer m.mu.Unlock()
			return m.termText
		},
		// 审批策略：写宿主 <home>/state/session-permission.json。
		// **异步、失败只写状态栏**——和 ConfigSet 同一个策略：插件不等待、不重试。
		SetPermission: func(perm string) {
			m.mu.Lock()
			id := m.cur
			m.mu.Unlock()
			if id == "" {
				// 没有当前会话就别发：宿主那边 `id` 必填，发了也是 400。
				// 而且界面上看起来会像"改了但没用"。
				m.setStatus("没有当前会话，改不了权限")
				return
			}
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := m.cli.SetSessionPermission(ctx, id, perm); err != nil {
					m.setStatus("改权限失败: " + err.Error())
					return
				}
				if perm == "" {
					m.setStatus("权限已恢复默认")
				} else {
					m.setStatus("权限已设为 " + perm)
				}
			}()
		},
		// 设置面（HOST-SETTINGS-FORM，rc.2 官方 SettingsRoot）：
		// describe 同步返回缓存、首次问触发后台拉取；写路径同步（调用方在
		// goroutine 里）、冲突原样回给插件渲染。见 settings_panel_wiring.go。
		SettingsDescribe:  m.settingsDescribeSnapshot,
		SettingsHostFacts: m.settingsHostFacts,
		SettingsFormSet:   m.settingsFormSet,
		SettingsFormReset: m.settingsFormReset,
		SettingsMutate:    m.settingsMutate,
		// 内置插件页的数据面：官方 pluginInventory.list 的缓存。
		// 与 describe 分开一处（它不是 Host 设置，是 Loader 的只读投影），
		// 见 plugin_inventory_wiring.go。
		PluginInventory:        m.pluginInventorySnapshot,
		PluginInventoryRefresh: m.pluginInventoryRefresh,
		FSRead: func(path string) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			c, err := m.cli.FSRead(ctx, path)
			if err != nil {
				return "", err
			}
			return c.Content, nil
		},
		// 文件引用 chip 的点击：应用内预览（官方 `sidebarRight.openResource`）。
		// 与 `OpenFile`（系统默认程序）是两条能力，不能互相顶替。
		PreviewFile: func(path string) {
			if m.kernel == nil {
				return
			}
			for _, pl := range m.kernel.Plugins() {
				if f, ok := pl.(*dsfiles.Plugin); ok {
					f.Open(m.cwd, path)
					m.inv.Invalidate()
					return
				}
			}
		},
		// `@` / `/` 候选：同步读壳缓存，首次问触发后台拉取（见 reference_wiring.go）。
		FileReferences:             m.fileReferences,
		SessionReferenceCandidates: m.sessionReferenceCandidates,
		Skills:                     m.skillCandidates,
		// 消息反馈：durable sidecar（宿主 messageFeedback.*），异步写、结果留给插件取。
		MessageFeedback:       m.messageFeedback,
		MessageFeedbackPut:    m.messageFeedbackPut,
		MessageFeedbackDelete: m.messageFeedbackDelete,
		FeedbackTakeResult:    m.feedbackTakeResult,
		OpenSession:           m.openSession,
		DeleteSession:         m.deleteSession,
		CompactSession:        m.compactSession,
		ForkSession:           m.forkSession,
		NewSession:            m.newSession,
		Send:                  m.sendText,
		Copy:                  func(text string) { _ = m.copyText(text) },
		CopyResult:            m.copyText,
		OpenExternal:          m.openExternal,
		OpenFile:              m.openFile,
		Notify:                m.setStatus,
		// 桌面更新指示器的点击（官方 `DesktopUpdateBridge.open()`）：
		// 走宿主拿更新入口 URL，再交给**系统默认浏览器**。
		// 见 desktop_update_wiring.go。
		DesktopUpdateOpen: m.desktopUpdateOpen,
		// 立即重连：取消当前 sub + 马上重订（官方连接胶囊按钮的动作）。
		Reconnect: m.reconnectNow,
		// 只重绘、不写状态栏：给插件的时间动画（官方 150ms 离场淡出）用。
		Invalidate:     func() { m.inv.Invalidate() },
		ToggleRightbar: m.toggleRightbar,
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
		Cancel: m.cancelTurn,
		// 向上翻一页历史：网络请求**必须**离开绘制线程（点一下卡一帧就是手感坏），
		// 所以排到后台 goroutine；`frameJournal.paging.Loading` 保证重复点击只发一次。
		//
		// 会话 id 在**点击这一帧**锁定（不是后台再读 m.cur）：用户点的就是屏幕上
		// 那个会话，请求途中切走也不该把这一页补到别人头上。journal 按会话存，
		// 切回来就是完整的一页。
		LoadOlder: func() {
			m.mu.Lock()
			id := m.cur
			m.mu.Unlock()
			go m.loadOlderFrames(id)
		},
		// SetDraft 把文字灌进输入框。草稿是 Viva 的 UI 线程状态，所以这里也只
		// 排队——调用点可能来自插件的后台动作（见 applyPendingDraft）。
		SetDraft:  func(text string) { m.queueDraftOverwrite(text) },
		Retry:     m.retryLast,
		RetrySend: m.retrySend,

		// 主题：dark / light / system。**排队到下一帧 Paint 去改**（见 theme.go）。
		// 命令面板、将来的设置页都走这一个入口。
		SetTheme: m.requestTheme,
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

	// **只给无头截图用**：-shot 没法注入鼠标，所以这里直接调插件的公开入口。
	// 产品路径不走它（用户点是 drawList 里的点击）。
	if *openFile != "" {
		go func(name string) {
			time.Sleep(time.Duration(*keysAfter) * time.Second)
			for _, pl := range m.kernel.Plugins() {
				if f, ok := pl.(*dsfiles.Plugin); ok {
					f.Open(m.cwd, name)
					m.inv.Invalidate()
					log.Printf("已打开预览: %s", name)
					return
				}
			}
			log.Printf("-open-file 给了 %q，但没找到 ds-files 插件", name)
		}(*openFile)
	}
	// 设置浮层：截图路径没有鼠标/热键，只能调插件的公开入口。
	// 它和 Shift+F2 走的是**同一个** Open()，不是测试专用实现。
	if *openSettings {
		go func() {
			time.Sleep(time.Duration(*keysAfter) * time.Second)
			if m.settings == nil || !m.settings.Open() {
				log.Printf("-open-settings：这一帧没有任何真实设置动作，浮层不打开")
				return
			}
			m.inv.Invalidate()
			log.Printf("设置浮层已打开")
		}()
	}
	if *keysSeq != "" {
		go func(seq string) {
			time.Sleep(time.Duration(*keysAfter) * time.Second) // 等会话列表/消息/审批帧到位
			for _, ev := range parseKeys(seq) {
				ok := m.kernel.HandleKey(ev)
				log.Printf("key %v mods=%v handled=%v", ev.Key, ev.Modifiers, ok)
				// **注入路径必须自己重绘**：它直接调 kernel.HandleKey，绕过了
				// cfg.OnKey 里那句 Invalidate。插件把 shown 切成 true 之后，
				// 一帧都不会再画——日志 handled=true 而屏幕上什么都没有。
				// 真按键盘时走 OnKey 那条路，本来就会重绘，所以这个坑只在
				// 「无头注入 + 按键改状态」的组合下暴露。
				m.inv.Invalidate()
				time.Sleep(350 * time.Millisecond)
			}
			m.mu.Lock()
			log.Printf("注入后当前会话: %s", m.cur)
			m.mu.Unlock()
		}(*keysSeq)
	}

	if shotPath != "" {
		startShot(m, shotPath, time.Duration(*shotWait)*time.Second, *chromeReport, *chromeProbe)
	}

	var canvas gfx.Canvas
	if err := app.RunCanvas(newConfig(m), &canvas); err != nil {
		log.Fatal(err)
	}
}
