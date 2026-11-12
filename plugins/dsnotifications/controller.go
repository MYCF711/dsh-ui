// controller.go —— 账号通知的运行时（官方 `notification-controller.ts` 的 Go 复刻）。
//
// # 官方生命周期（结构树 §5.4，逐命令对照）
//
//	begin()        幂等建 scope 并**立刻读一次**
//	end()          停表、清 scope、contentGeneration++、清 reported、清空卡片
//	refresh()      重启轮询并立刻读一次（**保留卡片**）
//	setLocale(v)   语言不变直接返回；变了只刷新内容（不重新列表）
//	shown/cta/dismiss  先就地过滤过期卡，再校验 `current[0].id == 传入 id`；
//	               只有前景卡才上报；每次交互只报一次、不重试
//	单次读          一次只有一个 poll；失败**保留卡片**并沿用旧间隔；成功后用
//	               `nextPollAfterSeconds` 覆盖间隔；表在请求 settle 之后才起
//	迟到保护        旧 poll / 旧 scope 的迟到回复不得发布、不得再排表
//	前景保留        已上报过 displayed 的前景卡即便后续列表省略也保留
//
// # 为什么这些规则要逐条照抄
//
// 通知是**服务端驱动**的：什么时候再来一条、这条还能显示多久、点了之后
// 还算不算数，全部由平台决定。界面自己"觉得应该清掉"或"觉得该立刻重拉"
// 都会让用户看到与服务端不一致的卡片——这类 bug 没有报错，只有"怎么又冒出来了"。
//
// # 上报是异步的（与官方一致）
//
// 官方 `report` 是 fire-and-forget：失败只 `console.error`，不改 UI、不回滚。
// 这里同样——`Shown/CTAClicked/Dismiss` 立刻改本地状态，上报在 goroutine 里发一次。
package dsnotifications

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// DefaultPollInterval 是官方 config 的首轮默认间隔
// （`notificationPollIntervalMs`，`Schema.natural().min(1).max(2147483647).default(600_000)`）。
//
// 只是**首轮**默认：随后每一轮的间隔由服务端的 `nextPollAfterSeconds` 决定。
const DefaultPollInterval = 600 * time.Second

// AccountState 是父级（账号菜单 / 设置启动器）这一帧给的账号态。
//
// 字段名对齐官方 `update(state, reset)` 读的那几项：
// `state.view?.status === 'credential-stored' && !state.failed` 才算已登录。
type AccountState struct {
	// Active 是官方渲染层的运行时 props：账号启动器是否打开。
	// 为假 → 整块不渲染（父级决定"何时可见"，本包不自己判）。
	Active bool
	// SignedIn 由父级按官方判据算好：status == credential-stored && !failed。
	SignedIn bool
	// Failed 是账号态失败（登录失败 / 刷新失败）。
	Failed bool
	// Wide 是官方 `hidden={!wide}` 的那个 `wide`：侧栏是否为宽态。
	//
	// 官方由设置外壳作为运行时 props 传给账号菜单（AccountMenu.tsx:60）；
	// 本仓没有那个外壳，所以由**壳**按侧栏真实宽度折算后经此字段注入
	// （`cmd/app` 的 `launcherWide` → `AccountState.Wide`）。
	// 零值 false = 收起轨：通知座位整段不渲染，启动器画 36×36 图标版。
	Wide bool
	// Locale 是当前界面语言（官方 setLocale 的入参）。
	Locale string
}

// Remote 是控制器要的三条远程能力（官方三条 typert 方法）。
//
// `*apiclient.Client` **直接满足**这个接口（方法签名逐字相同），所以壳只要
// 注入一个 `*apiclient.Client`，不需要任何适配层。
type Remote interface {
	GetNotifications(ctx context.Context, meta apiclient.NotificationClientMetadata) (*apiclient.NotificationBatch, error)
	GetNotificationContent(ctx context.Context, id string, meta apiclient.NotificationClientMetadata) (*apiclient.Notification, error)
	ReportNotificationEvent(ctx context.Context, id, event string, meta apiclient.NotificationClientMetadata) (bool, error)
}

// Controller 是一条账号通知的生命周期状态机。
//
// 零值不可用；用 NewController 造。所有方法都可从多个 goroutine 调。
type Controller struct {
	mu sync.Mutex

	remote Remote
	theme  any // 仅用于日志/调试时带了哪套主题，不参与判定
	meta   apiclient.NotificationClientMetadata

	interval time.Duration
	running  bool
	gen      int // 每次 begin/end/refresh 自增，用来丢弃迟到回复
	state    AccountState

	current  []apiclient.Notification
	reported map[string]bool // 已上报 displayed 的 id（前景保留 + 只报一次）

	timer *time.Timer
	now   func() time.Time
	log   *slog.Logger

	// openAccount 是 `target.type === 'app_route'` 的落地动作（官方 openAccount()）。
	// nil = 账号设置面还没接线：卡片照样画，点 CTA 要如实告诉用户"还没有"。
	// 返回 false = 接上了但这次打不开（见 SetOpenAccount 的注释）。
	openAccount func() bool
}

// NewController 造一个控制器。remote 可以为 nil（= 未授权/未接线：永远空态）。
func NewController(remote Remote) *Controller {
	return &Controller{
		remote:   remote,
		interval: DefaultPollInterval,
		now:      time.Now,
		log:      slog.Default(),
		meta: apiclient.NotificationClientMetadata{
			// 官方 AccountClientMetadata.version 是**必填**；这里给本仓基线版本。
			// locale 由父级 SetLocale 覆盖；时区偏移由 SetTimezoneOffset 覆盖。
			Version: "0.2.2-rc.2",
			Locale:  "zh-CN",
		},
	}
}

// SetRemote 接上（或摘掉）远程实现。
//
// 摘掉 = 未授权：下一次读不会再发请求，且**清空卡片**（官方 end 的语义）。
func (c *Controller) SetRemote(remote Remote) {
	c.mu.Lock()
	c.remote = remote
	c.mu.Unlock()
	if remote == nil {
		c.End()
	}
}

// SetTheme 只记下当前主题（日志用；控制器不画东西）。
func (c *Controller) SetTheme(theme any) {
	c.mu.Lock()
	c.theme = theme
	c.mu.Unlock()
}

// SetClock 注入时钟（测试用；生产不调）。
func (c *Controller) SetClock(now func() time.Time) {
	c.mu.Lock()
	if now != nil {
		c.now = now
	}
	c.mu.Unlock()
}

// SetLogger 注入日志器。
func (c *Controller) SetLogger(l *slog.Logger) {
	if l == nil {
		return
	}
	c.mu.Lock()
	c.log = l
	c.mu.Unlock()
}

// SetTimezoneOffset 设官方 AccountClientMetadata.timezoneOffsetSeconds。
func (c *Controller) SetTimezoneOffset(seconds float64) {
	c.mu.Lock()
	c.meta.TimezoneOffsetSeconds = seconds
	c.mu.Unlock()
}

// Meta 返回当前发给宿主/平台的客户端元数据（测试与日志用）。
func (c *Controller) Meta() apiclient.NotificationClientMetadata {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.meta
}

// Update 按账号态驱动生命周期（官方 `update(state, reset)` 的等价物）。
//
//	reset || !signedIn  → end()
//	signedIn && 未在跑   → begin()
//	已登录但语言变了     → 只刷新内容（官方 setLocale 的语义，不重新列表）
func (c *Controller) Update(st AccountState) {
	signedIn := st.SignedIn && !st.Failed

	c.mu.Lock()
	prevLocale := c.meta.Locale
	c.state = st
	if st.Locale != "" {
		c.meta.Locale = st.Locale
	}
	localeChanged := c.meta.Locale != prevLocale
	locale := c.meta.Locale
	wasRunning := c.running
	c.mu.Unlock()

	switch {
	case !signedIn:
		c.End()
	case !wasRunning:
		c.Begin()
	case localeChanged:
		c.refreshContents(locale)
	}
}

// SetOpenAccount 接上"打开账号设置"的动作（官方 openAccount）。
//
// nil 是合法状态：账号设置面还没落地时，CTA 只上报 cta_clicked 并给一条提示，
// 不假装跳转成功。
//
// 回调返回 **bool**：false = "接线了但这次没打开"（例如设置面板还没就绪）。
// 官方的 `openAccount()` 没有返回值，但本仓的设置面板是**另一个插件**提供的，
// 它可能拒绝打开（F2 面板没装/没就绪）——那种情况下 `OpenAccount()` 必须
// 如实回 false，让卡片说"还没有"，而不是把"点了没反应"说成成功。
func (c *Controller) SetOpenAccount(fn func() bool) {
	c.mu.Lock()
	c.openAccount = fn
	c.mu.Unlock()
}

// OpenAccount 执行"打开账号设置"。返回 false = 没接线，或这次没打开。
func (c *Controller) OpenAccount() bool {
	c.mu.Lock()
	fn := c.openAccount
	c.mu.Unlock()
	if fn == nil {
		return false
	}
	return fn()
}

// Begin 幂等地开始一轮生命周期，并立刻读一次列表。
func (c *Controller) Begin() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.gen++
	gen := c.gen
	c.reported = map[string]bool{}
	c.mu.Unlock()

	go c.fetch(gen)
}

// End 停表、作废在途回复、清空卡片与上报记录。
func (c *Controller) End() {
	c.mu.Lock()
	c.running = false
	c.gen++
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.current = nil
	c.reported = map[string]bool{}
	c.mu.Unlock()
}

// Refresh 重启轮询并立刻读一次；**保留**当前卡片（官方语义）。
func (c *Controller) Refresh() {
	c.mu.Lock()
	c.running = true
	c.gen++
	gen := c.gen
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.mu.Unlock()

	go c.fetch(gen)
}

// SetLocale 切语言：语言没变直接返回；变了只刷新**内容**（不重新列表）。
func (c *Controller) SetLocale(locale string) {
	if locale == "" {
		return
	}
	c.mu.Lock()
	if c.meta.Locale == locale {
		c.mu.Unlock()
		return
	}
	c.meta.Locale = locale
	c.mu.Unlock()
	c.refreshContents(locale)
}

// refreshContents 为当前每张卡并发拉一次内容（官方 readContent）。
//
// 并发拉取、保留 id 与顺序；旧代次 / 已关掉卡片的回复在 refreshContent 里丢弃。
func (c *Controller) refreshContents(locale string) {
	c.mu.Lock()
	ids := make([]string, 0, len(c.current))
	for _, n := range c.current {
		ids = append(ids, n.ID)
	}
	gen := c.gen
	c.mu.Unlock()
	_ = locale
	for _, id := range ids {
		id := id
		go c.refreshContent(id, gen)
	}
}

// Active 是父级给的"账号启动器是否打开"。
func (c *Controller) Active() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state.Active
}

// Visible 是**整块渲染**的唯一判据：父级说可见，且至少有一条未过期通知。
func (c *Controller) Visible() bool { return c.Active() && len(c.Notices()) > 0 }

// Notices 返回当前仍有效的通知（服务端顺序，前景在前）。
//
// 每次读都按"这一帧的时间"再过滤一次：卡片可能在两次轮询之间过期。
func (c *Controller) Notices() []apiclient.Notification {
	c.mu.Lock()
	defer c.mu.Unlock()
	live := liveNotices(c.current, c.now())
	if len(live) == 0 {
		return nil
	}
	out := make([]apiclient.Notification, len(live))
	copy(out, live)
	return out
}

// Shown 上报一次 displayed（官方要求"可见 + 过一帧"才报，由画层负责时机）。
//
// 只有**前景卡**能上报；非前景上报是无效调用（官方先校验 current[0].id）。
func (c *Controller) Shown(id string) { c.reportForeground(id, apiclient.NotificationEventDisplayed) }

// CTAClicked 上报一次 cta_clicked。卡片**保留**（官方：打开账号设置不关卡片）。
func (c *Controller) CTAClicked(id string) {
	c.reportForeground(id, apiclient.NotificationEventCTAClicked)
}

// Dismiss 关闭一条通知：本地立刻移除，再异步上报 dismissed。
func (c *Controller) Dismiss(id string) {
	if c.reportForeground(id, apiclient.NotificationEventDismissed) {
		c.mu.Lock()
		out := c.current[:0]
		for _, n := range c.current {
			if n.ID != id {
				out = append(out, n)
			}
		}
		c.current = out
		c.mu.Unlock()
	}
}

// reportForeground 校验"id 是不是前景卡"并异步上报一次。
//
// 返回 true 表示这条确实**是**前景卡且请求已发出。
func (c *Controller) reportForeground(id, event string) bool {
	c.mu.Lock()
	// 官方：上报前先就地过滤过期卡（过期前景不算前景）。
	c.current = liveNotices(c.current, c.now())
	if len(c.current) == 0 || c.current[0].ID != id {
		c.mu.Unlock()
		return false
	}
	if event == apiclient.NotificationEventDisplayed {
		if c.reported[id] {
			// 一次挂载只报一次 displayed（官方 reported ref）。
			c.mu.Unlock()
			return true
		}
		c.reported[id] = true
	}
	remote, meta, log := c.remote, c.meta, c.log
	c.mu.Unlock()

	if remote == nil {
		return true
	}
	go func() {
		// 失败只记日志：官方明确"不改 UI、不回滚、不重试"。
		if _, err := remote.ReportNotificationEvent(context.Background(), id, event, meta); err != nil {
			log.Warn("账号通知事件上报失败（只记录，不改界面）", "event", event, "id", id, "err", err)
		}
	}()
	return true
}

// fetch 是一次读：只发布**当前代次**的回复，失败保留卡片与旧间隔。
func (c *Controller) fetch(gen int) {
	c.mu.Lock()
	remote, meta := c.remote, c.meta
	c.mu.Unlock()

	if remote == nil {
		// 未授权/未接线：不发请求，也不排表（下一帧 Update 再说）。
		return
	}

	batch, err := remote.GetNotifications(context.Background(), meta)

	c.mu.Lock()
	if c.gen != gen || !c.running {
		// 旧 poll / 旧 scope 的迟到回复：**不得发布**、也不得再排表。
		c.mu.Unlock()
		return
	}
	if err != nil {
		// 失败：保留卡片，沿用旧间隔，等下一轮。
		c.log.Warn("账号通知读取失败（保留现有卡片）", "err", err)
	} else {
		c.applyLocked(batch)
	}
	next := c.interval
	if c.timer != nil {
		c.timer.Stop()
	}
	c.timer = time.AfterFunc(next, func() { c.tick(gen) })
	c.mu.Unlock()
}

// tick 是定时器的回调：代次没变才继续读。
func (c *Controller) tick(gen int) {
	c.mu.Lock()
	if c.gen != gen || !c.running {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()
	c.fetch(gen)
}

// applyLocked 把一次成功的批次并入当前卡片（调用方持锁）。
//
// 合并规则（官方 §5.4）：
//   - 服务端顺序就是卡片顺序；
//   - 已上报过 displayed 的前景卡，即便新列表省略了它，也**保留**在最前；
//   - 过期卡不进来（避免"刚拉下来就消失"的闪烁）；
//   - 间隔只在 `nextPollAfterSeconds` 为正时覆盖。
func (c *Controller) applyLocked(batch *apiclient.NotificationBatch) {
	if batch == nil {
		// 官方 null = 未登录 / 查询中授权变了。清空卡片（不是"没有通知"）。
		c.current = nil
		c.reported = map[string]bool{}
		return
	}
	live := liveNotices(batch.Notifications, c.now())

	var merged []apiclient.Notification
	if len(c.current) > 0 && c.reported[c.current[0].ID] {
		front := c.current[0]
		found := false
		for _, n := range live {
			if n.ID == front.ID {
				found = true
				break
			}
		}
		if !found {
			merged = append(merged, front)
		}
	}
	seen := map[string]bool{}
	for _, n := range merged {
		seen[n.ID] = true
	}
	for _, n := range live {
		if seen[n.ID] {
			continue
		}
		seen[n.ID] = true
		merged = append(merged, n)
	}
	c.current = merged

	if batch.NextPollAfterSeconds > 0 {
		c.interval = time.Duration(batch.NextPollAfterSeconds * float64(time.Second))
	}
}

// refreshContent 按语言刷新一条通知的文案（官方 readContent）。
//
// 保持 id/顺序不变；旧代次/已关掉卡片的回复丢弃。
func (c *Controller) refreshContent(id string, gen int) {
	c.mu.Lock()
	remote, meta := c.remote, c.meta
	c.mu.Unlock()
	if remote == nil {
		return
	}
	content, err := remote.GetNotificationContent(context.Background(), id, meta)
	if err != nil || content == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.gen != gen || !c.running {
		return
	}
	for i := range c.current {
		if c.current[i].ID != id {
			continue
		}
		// 只换文案/动作，id、createdAt、displayUntil 保持服务端原值。
		c.current[i].Title = content.Title
		c.current[i].Body = content.Body
		c.current[i].Actions = content.Actions
		return
	}
}

// liveNotices 过滤掉已过期的通知（官方 `live(displayUntil)`）。
//
// 三条规则逐字照抄（AccountNotifications.tsx:43-48）：
//   - `displayUntil === null` → 永不过期；
//   - 时间戳**解析不出来** → 不算过期（宁可多显示，也不因为脏时间吞掉服务端通知）；
//   - 只有 `expires > now` 才显示。
func liveNotices(in []apiclient.Notification, now time.Time) []apiclient.Notification {
	if len(in) == 0 {
		return nil
	}
	out := make([]apiclient.Notification, 0, len(in))
	for _, n := range in {
		if live(n, now) {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func live(n apiclient.Notification, now time.Time) bool {
	if n.DisplayUntil == nil {
		return true
	}
	until, err := time.Parse(time.RFC3339, *n.DisplayUntil)
	if err != nil {
		return true
	}
	return until.After(now)
}
