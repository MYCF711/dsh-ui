// wave3_wiring.go —— GUI 第三波：两个父容器的**壳侧数据接线**。
//
// 两个容器（`plugins/dsnotifications` 的设置启动器入口、`plugins/dsagentteam` 的
// 子代理目录）都只负责"画"，数据必须由壳喂——这是本仓的 R7：界面不直连平台，
// 插件更不自己发请求。本文件就是那条喂数据的路，只有三件事：
//
//  1. `Provide(ui.apiclient)`：把壳唯一的 `*apiclient.Client` 交给插件。
//     （GA 的 L0 没提供过它；GB 报告 §7.2 第 2 条把它列为待接线。）
//  2. `Controller.Update(AccountState)`：账号态驱动通知入口的可见性
//     （官方 `update(state, reset)`，结构树 §5.5）。数据来源是宿主真实的
//     `account.getState`（本波新增的客户端方法），**不是猜**。
//  3. `Store.SetView(View)`：把 `session.projections` 的两块真值喂给目录容器
//     （`subagentCatalog` → 目录行，`teamMode` → Team 开关状态，
//     `modelSelection` → 开关的**可用性**判据）。
//  4. `Snapshot.SessionStats`：状态栏「活动」的 whole-log `sessionStats` 投影
//     （八字段；nil = 未知 → 写「未知」，不编 0）。
//
// # 三条"不编"（与 AGENTS §2 同口径）
//
//   - 查询失败 ≠ 未登录：`account.getState` 回 not_implemented 时按**未知**处理
//     （隐藏），不写成 `signed-out`，也不画"未登录"文案。
//   - 没有数据 ≠ 0：`subagentCatalog` 键缺失时 `CatalogKnown=false`（隐藏），
//     不画"0 个子智能体"。
//   - 打不开 ≠ 打开了：`SetOpenAccount` 只包**真的能打开设置浮层**的那个能力；
//     打不开时控制器自己会说"还没接线"（`Controller.OpenAccount` 返回 false）。
//
// # 节奏
//
// 两条数据都是**低频轮询**（账号 30s、投影 10s + 切会话立刻拉一次）：
// 宿主没有推送通道（`account.watch` 是流式，本波不引），所以只能问。
// 拉取在后台 goroutine，绘制里**不做网络等待**（否则首帧会卡）。
package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dsagentteam"
	"github.com/MYCF711/dsh-ui/plugins/dsnotifications"
)

// 能力键（与两个插件 `RemoteKey` / `ServiceKey` 逐字一致）。
const (
	// keyAPIClient 是 `plugins/{dsnotifications,dsagentteam}.RemoteKey` 的值。
	keyAPIClient = "ui.apiclient"
	// keyAccountNotifications 是 `dsnotifications.ServiceKey`（值 *Controller）。
	keyAccountNotifications = "ui.account.notifications"
	// keyAgentTeam 是 `dsagentteam.ServiceKey`（值 *Store）。
	keyAgentTeam = "ui.agent-team"
)

const (
	// accountPollEvery 是账号状态的轮询间隔。
	//
	// 账号态变化很慢（登录/登出/失败），30s 足够让通知入口跟上，
	// 又不会把宿主问烦；官方 `account.watch` 是流，本波不引流。
	accountPollEvery = 30 * time.Second
	// projectionsPollEvery 是会话投影的轮询间隔（切会话会立刻拉一次）。
	projectionsPollEvery = 10 * time.Second
	// wave3FetchTimeout 是单次查询的超时（离屏/断链时不许把 goroutine 挂死）。
	wave3FetchTimeout = 6 * time.Second
)

// wave3Wiring 是第三波两个容器的壳侧状态：缓存 + 服务句柄 + 节流标志。
//
// 放在独立结构里（不是塞进 `model` 的一堆散字段）：这些值只服务这两个容器，
// 生命周期与它们一致，读起来才知道"谁在用"。
type wave3Wiring struct {
	// tier 是壳档位（official / legacy）：两个容器据此选官方挂载点或过渡桥。
	tier *uikernel.ShellTier
	// notices 是账号通知运行时（nil = 插件没装配）。
	notices *dsnotifications.Controller
	// team 是 Team 运行时（nil = 插件没装配）。
	team *dsagentteam.Store

	mu sync.Mutex

	// ── 账号态（account.getState 的缓存）──
	accountKnown    bool
	accountSignedIn bool
	accountFailed   bool
	accountAt       time.Time
	accountPending  bool

	// ── 会话投影（session.projections 的缓存）──
	projSession      string
	projCatalog      []dsagentteam.CatalogEntry
	projCatalogKnown bool
	projCatalogErr   string
	projTeamEnabled  bool
	projTeamKnown    bool
	// projStats 是 whole-log `sessionStats` 投影（nil = 未知）。指向的结构体
	// 存进缓存后**永不原地改**（每次拉取换一个新指针），所以读方拿指针安全。
	projStats *apiclient.SessionStats
	// projUsage 是 whole-log `tokenUsage` 投影的四桶（nil = 未知，与 projStats
	// 同一份 `session.projections`、同一次拉取；同样"换指针不原地改"）。
	projUsage *apiclient.TokenUsage
	// projModelKnown / projModelLastUsed 是 `modelSelection` 投影的两个官方判据：
	// 键存在 = 开关可用（`modelSelection !== undefined`）；
	// `lastUsed !== null` = 关闭前要先弹停用确认（TeamModeToggle.tsx:43/47）。
	projModelKnown    bool
	projModelLastUsed bool
	projAt            time.Time
	projPending       bool
}

// newWave3Wiring 造一份接线状态（壳在装配前调用）。
func newWave3Wiring() wave3Wiring {
	return wave3Wiring{tier: uikernel.NewShellTier()}
}

// provide 在 `kernel.Load` **之前**把壳侧能力交给插件。
//
// 只能在此之前：插件的 `Register` 里就会 `ctx.Inject` 这两个键。
func (w *wave3Wiring) provide(k *uikernel.Kernel, cli *apiclient.Client) {
	if k == nil {
		return
	}
	k.Root().Provide(uikernel.KeyShellTier, w.tier)
	if cli != nil {
		k.Root().Provide(keyAPIClient, cli)
	}
}

// attach 在 `kernel.Load` **之后**取回两个插件提供的服务，并接好动作。
//
// `openAccount` 只包真实能力：`dssettings` 的 `ui.settings.open`（"打开设置浮层"）。
// 设置浮层里**没有账号分区**（`dssettings` 目前只有主题/插件/说明三节），
// 这条差距登记在报告里——所以这里宁可只做"打开设置"，也不假装跳到了账号页。
func (w *wave3Wiring) attach(m *model) {
	if m == nil || m.kernel == nil {
		return
	}
	// 档位：Load 之后才解析得出（auto 要看 shell.root 有没有占用方）。
	w.tier.Set(m.shellMode.resolve(m.kernel) == shellOfficial)

	if v := m.kernel.Get(keyAccountNotifications); v != nil {
		if ctrl, ok := v.(*dsnotifications.Controller); ok {
			w.notices = ctrl
		}
	}
	if v := m.kernel.Get(keyAgentTeam); v != nil {
		if store, ok := v.(*dsagentteam.Store); ok {
			w.team = store
		}
	}
	// 子代理目录容器要知道档位（它的 painter 归 agent_team.go 登记，见 catalog.go）。
	for _, p := range m.kernel.Plugins() {
		if a, ok := p.(interface{ AttachShellTier(*uikernel.ShellTier) }); ok {
			a.AttachShellTier(w.tier)
		}
	}
	if w.notices != nil {
		w.notices.SetOpenAccount(func() bool {
			if m.settings == nil {
				return false // 没装设置面板：让卡片如实说"还没有"
			}
			opened := m.settings.Open()
			if opened {
				m.inv.Invalidate()
			}
			return opened
		})
	}
}

// accountSnapshot 取缓存的账号态（known=false = 未知，不是"未登录"）。
func (w *wave3Wiring) accountSnapshot() (signedIn, failed, known bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.accountSignedIn, w.accountFailed, w.accountKnown
}

// projectionSnapshot 取某会话的投影缓存。
func (w *wave3Wiring) projectionSnapshot(session string) (catalog []dsagentteam.CatalogEntry,
	known bool, errText string, teamEnabled, teamKnown bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if session == "" || session != w.projSession {
		// 切到还没拉过的会话：目录**未知**（隐藏），等后台拉到再画。
		return nil, false, "", false, false
	}
	return w.projCatalog, w.projCatalogKnown, w.projCatalogErr, w.projTeamEnabled, w.projTeamKnown
}

// sessionStatsSnapshot 取某会话的 whole-log `sessionStats` 投影。
//
// 返回 nil = **未知**（切到还没拉过的会话 / 查询失败 / 老宿主没有这个键）。
// 状态栏据此写「活动 未知」——不许拿当前窗口的 `Msgs` 数轮/步（那是 tail，
// 既不是全量也不区分 step）。
//
// 返回的指针在缓存里被换掉而不是原地改（见 wave3Wiring 的字段注释），
// 所以调用方跨帧持有也安全。
func (w *wave3Wiring) sessionStatsSnapshot(session string) *apiclient.SessionStats {
	w.mu.Lock()
	defer w.mu.Unlock()
	if session == "" || session != w.projSession {
		return nil
	}
	return w.projStats
}

// tokenUsageSnapshot 取某会话的 whole-log `tokenUsage` 投影四桶。
//
// 返回 nil = **未知**（切到还没拉过的会话 / 查询失败 / 老宿主没有这个键）：
// 用量胶囊据此写「未知」，**不是**写 0 桶。四桶全 0 但键在时返回非 nil——
// 那是"宿主说这份日志没有用量样本"的真值，两者不能互相顶替。
func (w *wave3Wiring) tokenUsageSnapshot(session string) *apiclient.TokenUsage {
	w.mu.Lock()
	defer w.mu.Unlock()
	if session == "" || session != w.projSession {
		return nil
	}
	return w.projUsage
}

// modelSelectionSnapshot 取 `modelSelection` 投影的两个官方判据。
//
// known=false = 键缺失（老宿主 / 没拉到）→ 官方 `modelSelection === undefined`，
// Team 开关**禁用**（不是隐藏；隐藏由 teamMode 缺失那一层负责）。
// lastUsed=true = 这个会话已经用过模型 → 关闭 Team 前要弹停用确认。
func (w *wave3Wiring) modelSelectionSnapshot(session string) (known, lastUsed bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if session == "" || session != w.projSession {
		return false, false
	}
	return w.projModelKnown, w.projModelLastUsed
}

// feed 每帧把真值喂给两个运行时（绘制之前调）。
//
// `wide` 由壳按侧栏真实宽度折算（官方这个值是设置外壳的运行时 props；
// 本仓没有那个外壳，见 `dsnotifications.LauncherWide`）。
func (w *wave3Wiring) feed(sessionID string, sidebarWidth float32) {
	wide := dsnotifications.LauncherWide(sidebarWidth)
	signedIn, failed, known := w.accountSnapshot()
	if w.notices != nil {
		// Active 是官方 "通知区是否可见"：已登录 + 宽态（AccountMenu.tsx:60）。
		w.notices.Update(dsnotifications.AccountState{
			Active:   known && signedIn && wide,
			SignedIn: known && signedIn,
			Failed:   failed,
			Wide:     wide,
		})
	}
	if w.team != nil {
		catalog, catKnown, catErr, teamEnabled, teamKnown := w.projectionSnapshot(sessionID)
		// 官方 TeamModeToggle 的两个附加判据（同一份 `session.projections`）：
		// `modelSelection === undefined` → 开关禁用；`lastUsed !== null` → 关闭前确认。
		modelKnown, modelLastUsed := w.modelSelectionSnapshot(sessionID)
		w.team.SetView(dsagentteam.View{
			SessionID: sessionID,
			// 官方投影叫 `agentTeam`（enabled + tasks + failure）；本仓宿主只有
			// `teamMode.enabled`（tasks 面还没实现），所以只喂 enabled——
			// tasks 空就是空，不编任务。
			Projection:      dsagentteam.TeamProjection{Enabled: teamEnabled},
			ProjectionKnown: teamKnown,
			Catalog:         catalog,
			CatalogKnown:    catKnown,
			CatalogError:    catErr,
			Members:         len(catalog),
			// 缺 `modelSelection` 时开关**禁用**（官方 undefined 语义），不是隐藏。
			ModelSelectionKnown: modelKnown,
			ModelLastUsed:       modelLastUsed,
		})
	}
}

// refreshDue 判断这一帧要不要发起后台拉取（每帧调，不阻塞）。
func (w *wave3Wiring) refreshDue(m *model, sessionID string) (account, projections bool) {
	now := time.Now()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.notices != nil && !w.accountPending && now.Sub(w.accountAt) >= accountPollEvery {
		w.accountPending = true
		account = true
	}
	if w.team != nil && sessionID != "" && !w.projPending &&
		(sessionID != w.projSession || now.Sub(w.projAt) >= projectionsPollEvery) {
		w.projPending = true
		projections = true
	}
	return account, projections
}

// fetchAccount 拉一次账号态（后台 goroutine；只写缓存 + 触发重绘）。
func (w *wave3Wiring) fetchAccount(m *model) {
	defer func() {
		w.mu.Lock()
		w.accountPending = false
		w.accountAt = time.Now()
		w.mu.Unlock()
	}()
	if m.cli == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), wave3FetchTimeout)
	defer cancel()
	view, err := m.cli.GetAccountState(ctx)

	w.mu.Lock()
	switch {
	case err != nil:
		if isCapabilityMissing(err) {
			// 宿主没有账号状态机：**未知**（隐藏），不是"未登录"。
			w.accountKnown = false
			w.accountSignedIn = false
			w.accountFailed = false
		} else {
			// 真失败（连不上 / 500）：保留上一轮的真值，只标 failed。
			w.accountFailed = true
		}
	case view == nil:
		w.accountKnown = false
		w.accountSignedIn = false
		w.accountFailed = false
	default:
		w.accountKnown = true
		w.accountSignedIn = view.SignedIn()
		w.accountFailed = false
	}
	w.mu.Unlock()

	// 与壳里另外二十处一致：Invalidator 是值类型，没法判 nil（装配后才有 waker）。
	m.inv.Invalidate()
}

// fetchProjections 拉一次会话投影（后台 goroutine）。
func (w *wave3Wiring) fetchProjections(m *model, sessionID string) {
	defer func() {
		w.mu.Lock()
		w.projPending = false
		w.projAt = time.Now()
		w.mu.Unlock()
	}()
	if m.cli == nil || sessionID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), wave3FetchTimeout)
	defer cancel()
	proj, err := m.cli.SessionProjections(ctx, sessionID)
	if err != nil {
		w.mu.Lock()
		w.projSession = sessionID
		w.projCatalog = nil
		w.projTeamKnown = false
		w.projTeamEnabled = false
		// 查询失败 = **未知**：状态栏写「活动 未知」、Team 开关禁用/隐藏，
		// 不沿用上一个会话的数字（跨会话复用比"未知"更坏）。
		w.projStats = nil
		w.projUsage = nil
		w.projModelKnown = false
		w.projModelLastUsed = false
		if isCapabilityMissing(err) {
			// 宿主没有 session.projections：目录**未知**（隐藏），不画错误行。
			w.projCatalogKnown = false
			w.projCatalogErr = ""
		} else {
			// 真失败：官方错误态要能看见并重试 → 保留错误文案（容器会画出来）。
			w.projCatalogKnown = false
			w.projCatalogErr = err.Error()
		}
		w.mu.Unlock()
		m.inv.Invalidate()
		return
	}

	entries, catalogKnown := proj.SubagentCatalog()
	mode, teamKnown := proj.TeamMode()
	// whole-log 统计（状态栏「活动」）与模型选择（Team 开关可用性）。
	stats, statsKnown := proj.SessionStats()
	// whole-log 用量四桶（状态栏「用量」与它的详情弹窗）。
	usage, usageKnown := proj.TokenUsage()
	selection, selectionKnown := proj.ModelSelection()

	w.mu.Lock()
	w.projSession = sessionID
	w.projCatalog = make([]dsagentteam.CatalogEntry, 0, len(entries))
	for _, e := range entries {
		w.projCatalog = append(w.projCatalog, dsagentteam.CatalogEntry{
			ID:        e.ID,
			Label:     e.Label,
			Mode:      e.Mode,
			CreatedAt: e.CreatedAt,
		})
	}
	w.projCatalogKnown = catalogKnown
	w.projCatalogErr = ""
	w.projTeamKnown = teamKnown
	w.projTeamEnabled = teamKnown && mode != nil && mode.Enabled
	if statsKnown {
		w.projStats = stats
	} else {
		w.projStats = nil
	}
	if usageKnown {
		w.projUsage = usage
	} else {
		w.projUsage = nil
	}
	w.projModelKnown = selectionKnown
	w.projModelLastUsed = selectionKnown && selection != nil && selection.LastUsed != nil
	w.mu.Unlock()

	// 与壳里另外二十处一致：Invalidator 是值类型，没法判 nil（装配后才有 waker）。
	m.inv.Invalidate()
}

// isCapabilityMissing 判断错误是不是"这个宿主没有这个能力"。
//
// 三种都要当"未知"而不是"失败"：① 宿主明确回 not_implemented（501）；
// ② 老宿主不认这个方法（unsupported/not_found）；③ 界面自己的契约里没有它
// （mockd 一类夹具）。这三种都**不该**在界面上画成"加载失败"。
func isCapabilityMissing(err error) bool {
	var apiErr *apiclient.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case "not_implemented", "unsupported", "not_found", "method_not_found":
			return true
		}
		return false
	}
	return false
}
