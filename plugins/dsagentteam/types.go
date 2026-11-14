// types.go —— 官方数据形状 + 语言域（`dsh-agent-team/src/types.ts`、`locales.ts`）。
//
// 界面只读视图，不做写操作：任务板**只读**（README：Agents mutate tasks through
// tools; the panel provides no mutation controls）。
package dsagentteam

import "strings"

// TaskStatus 是官方 `TeamTaskStatus`（`TeamTaskView.status`）。
type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusCompleted  TaskStatus = "completed"
	// StatusDeleted 只出现在投影里，TeamBoard 一律滤掉（TeamBoard.tsx:19,86）。
	StatusDeleted TaskStatus = "deleted"
)

// TeamTask 是官方 `TeamTaskView`（rc.2 `dsh-agent-team/src/types.ts:78-92`）。
//
// 界面**只显示** subject / description / owner / status / ready；
// `blockedBy` / `writeScopes` / `writeScopeWarnings` 是宿主内部字段，官方 README
// 明写不在界面展示——但 `ready === false` 的 pending 任务必须画 warning 圆点。
type TeamTask struct {
	ID          string
	Revision    int
	Subject     string
	Description string
	Status      TaskStatus
	BlockedBy   []string
	WriteScopes []string
	OwnerID     string
	OwnerName   string
	// Ready 是"依赖都完成了"。零值 false 对 pending 任务意味着**被阻塞**，
	// 所以上游必须给真值；投影里缺这个字段时界面按"阻塞"画（宁可多提示）。
	Ready bool
}

// TeamProjection 是官方 `TeamProjection`（`types.ts:98-102`，projection key `agentTeam`）。
type TeamProjection struct {
	Enabled bool
	Tasks   []TeamTask
	// Failure 是投影自带的失败说明（`failure?: string`）。空 = 没有失败。
	Failure string
}

// View 是父级这一帧喂给运行时的快照（官方 hooks 的 Go 等价物）。
type View struct {
	// SessionID 是当前会话（根会话才有开关）。
	SessionID string

	// Projection / ProjectionKnown 对应 `useProjection('agentTeam')`：
	// **Known=false 等价于 undefined** → 任务板与开关都不渲染
	// （TeamBoard.tsx:86 的 `!team?.enabled`、TeamModeToggle.tsx:43 的 `mode === undefined`）。
	Projection      TeamProjection
	ProjectionKnown bool

	// Catalog 是官方 `subagentCatalog` 投影的行数据（ui-subagent 的目录内容）。
	//
	// 官方这一份数据由 `sessions.projectionsBySession[...].values.subagentCatalog`
	// 提供；本仓宿主同一个键也实现了（handlers_session_official.go:875 的
	// session.projections），壳经 apiclient 取回后喂进来。
	Catalog []CatalogEntry
	// CatalogKnown 对应官方的 `presentedCatalog !== undefined`：
	// **false = 目录未知**（键不存在/查询失败）→ 整个目录容器不渲染，
	// 不是"画一个 0 个子智能体的空目录"（那是把"不知道"说成"没有"）。
	CatalogKnown bool
	// CatalogError 是目录读取失败的原因（官方 `presentedCatalog.state === 'error'`）。
	// 非空时容器仍然渲染（官方错误态要能看见并重试），行里显示失败原因。
	CatalogError string

	// Members 是 `subagentCatalog` 的条数（只用于 data-with-members 的 4px 上边距）。
	Members int

	// Children 是"每个子会话自己的投影"（键 = 子会话 id）。
	//
	// 官方 `CatalogRows` 逐层递归：行的 metrics 与本层的下一级目录都来自
	// `summaries[id].projectionValues`（OFF:SubagentHeaderLineage.tsx:209-213 的
	// `canExpandCatalog`、:305-322 的 metrics）。本仓没有 sessions store，
	// 所以由壳按子会话逐个拉 `session.projections` 后喂进来。
	// nil 或键缺失 = **未知**（不画 metrics、不画 disclosure）。
	Children map[string]ChildProjection

	// Locked 对应官方的 `locked`：宿主说这个会话当前不能改 Team 模式。
	Locked bool
	// SubagentChild 对应 `useSession(s => s.subagent !== null)`。
	SubagentChild bool
	// ListedAsSubagent 对应 `useSessions(s => s.byId[sessionId]?.origin === 'subagent')`。
	ListedAsSubagent bool
	// ModelSelectionKnown 对应 `modelSelection !== undefined`（累加器缺失时开关禁用）。
	ModelSelectionKnown bool
	// ModelLastUsed 对应 `modelSelection.lastUsed !== null`：
	// **关闭**时只有它为真才弹确认（结构树 §2.2）。
	ModelLastUsed bool
}

// CatalogEntry 是官方 `subagentCatalog` 的一项（ui-subagent 目录里的一行）。
//
// 官方行里还有 activity（running/completed/inactive）、token、duration 等指标；
// 本仓宿主的 `subagentCatalog` 只有 id/createdAt/mode/label，那三类指标来自
// **该子会话自己**的 `session.projections`（`tokenUsage` / `subagentTiming`），
// 由 `View.Children` 逐个喂进来（见 ChildProjection）。
type CatalogEntry struct {
	// ID 是子会话 id（点击 = 打开该子会话）。
	ID string
	// Label 官方 continuable 必填；本仓宿主给空串（无 durable 创建标签）。
	// 空 label 时按官方兜底只显示 id 尾部，**不编一段标题**。
	Label string
	// Mode 是官方闭集四值：continuable | one-shot | external | unknown。
	Mode string
	// CreatedAt 是创建时刻（毫秒，官方 createdAt）。
	CreatedAt int64
}

// SubagentTiming 是宿主 `subagentTiming` 投影的界面视图
// （官方 `summaries[id].projectionValues.subagentTiming`）。
//
//	HOST:internal/transport/handlers_session_official.go:382-393
//	  { settledMs, active?:{since,through}, lastTurnCompleted? }
type SubagentTiming struct {
	// SettledMs 是已闭合轮的墙钟总和。
	SettledMs int64
	// Active 为真表示当前有一个未闭合轮；Since/Through 是它的时间界。
	Active        bool
	ActiveSince   int64
	ActiveThrough int64
	// LastTurnCompleted 最近闭合轮是否成功收尾；nil = 未知（不判"完成"）。
	LastTurnCompleted *bool
}

// DurationMs 是官方 `activityDuration`（OFF:SubagentHeaderLineage.tsx:89-104）：
// 有未闭合轮时 = `SettledMs + max(0, (running ? now : active.through) − active.since)`，
// 否则 = `SettledMs`；没有这个投影（nil）时返回 (0,false) = 这一格不画。
func (t *SubagentTiming) DurationMs(now int64) (int64, bool) {
	if t == nil {
		return 0, false
	}
	ms := t.SettledMs
	if t.Active {
		// 官方这里还有一位 `running`（会话 status）：在跑用 now，不在跑用
		// active.through。本仓宿主**没有** running 真值，所以优先用宿主给的
		// through（它是真值，且不依赖客户端时钟）；只有宿主没给 through 时
		// 才用当前时刻兜底。
		through := t.ActiveThrough
		if through <= 0 {
			through = now
		}
		if through > t.ActiveSince {
			ms += through - t.ActiveSince
		}
	}
	if ms <= 0 {
		return 0, false
	}
	return ms, true
}

// ChildProjection 是**子会话自己**的投影（官方 `summaries[id].projectionValues` 的等价物）。
//
// 为什么要逐个喂：官方 `subagentCatalog` 只给父→子的行身份；行上的 token 与
// 时长来自该子会话的 `session.projections`（`tokenUsage` / `subagentTiming`），
// 目录展开时用的下一层 catalog 也来自同一个方法。
type ChildProjection struct {
	// Catalog 是这一层的 `subagentCatalog`；CatalogKnown=false = 还没读到
	// （**不是**"没有子代理"）。
	Catalog      []CatalogEntry
	CatalogKnown bool
	// Timing 是 `subagentTiming`；nil = 未知（不画时长、不判完成）。
	Timing *SubagentTiming
	// TokenTotal 是 `tokenUsage` 四桶之和；nil = 未知（不画 token 指标）。
	TokenTotal *int64
}

// catalogLabel 是这一行的显示文案：官方 `primaryLabel`（label 优先，没有就用 id）。
func (e CatalogEntry) catalogLabel() string {
	if strings.TrimSpace(e.Label) != "" {
		return e.Label
	}
	return e.ID
}

// CommandOutcome 是官方命令结果（`CommandNode.outcome`）：success | error。
type CommandOutcome struct {
	Kind string // "success" | "error"
	Text string
}

// CommandNode 是官方 `conversation.chat.commandview` 的 `node` 入参。
//
// 本仓的聊天流还没有"命令节点"这种节点类型（dsconversation 只画消息），
// 所以由父级通过 Store.SetCommand 喂进来；`Command` 不是 `team` 时本包不画
// （官方由 ui-chat 的 key 过滤完成；我们这层再守一次，免得误挂到别的命令上）。
type CommandNode struct {
	Command string
	Outcome *CommandOutcome
}

// ── 语言域（官方 `locales.ts` 的 zh 字典，逐字）────────────────────────────
//
// 官方 NS = 'agent-team'，22 个键 × 8 语言。这里只落本包用到的 18 个；
// 缺的三个（loading/unavailable/empty）本仓还画不到（没有加载态与空态入口，
// 任务板空且无失败时整块不渲染），不硬编一个"暂无共享任务"进去冒充官方文案。
const (
	zhProgressCompleted = "{count} 已完成"
	zhProgressActive    = "{count} 进行中"
	zhProgressPending   = "{count} 待处理"
	zhCommandTitle      = "智能体团队协作"
	zhCommandRunning    = "执行中"
	zhCommandDone       = "已完成"
	zhCommandFailed     = "执行失败"
	zhCommandFailedWhy  = "执行失败：{reason}"
	zhTrigger           = "任务板"
	zhModeLabel         = "启用智能体团队协作"
	zhModeInfoLabel     = "关于智能体团队协作"
	zhModeDescription   = "通过共享任务板协调多个智能体的分工与进度，复杂任务表现更佳，token 开销也相应更高。"
	zhModeFailed        = "无法确认智能体团队协作设置已保存，请重试：{message}"
	zhDisableTitle      = "确认停用智能体团队协作？"
	zhDisableDesc       = "对话过程中停用智能体团队协作，将停止共享任务工具和团队协调指令。"
	zhDisableWarning    = "停用后可能影响缓存复用、增加后续 token 用量与开销，并影响当前任务的协作效果。"
	zhDisableCancel     = "取消"
	zhDisableConfirm    = "确认停用"
	zhDisableClose      = "关闭"
	zhTasks             = "共享任务"
	zhOwner             = "负责人"
	zhUnowned           = "未分配"
	zhBlocked           = "被依赖阻塞"
	zhStatusPending     = "待处理"
	zhStatusInProgress  = "进行中"
	zhStatusCompleted   = "已完成"
	zhFailure           = "任务记录无效：{message}"
)

// 进度分隔符：官方 `join('\u2002·\u2002')`（en-space + · + en-space）。
const progressSeparator = "\u2002·\u2002"

// statusLabel 把状态翻成官方文案（statusKeys 映射，TeamBoard.tsx:21-23）。
func statusLabel(status TaskStatus) string {
	switch status {
	case StatusPending:
		return zhStatusPending
	case StatusInProgress:
		return zhStatusInProgress
	case StatusCompleted:
		return zhStatusCompleted
	}
	return string(status)
}

// ownerLabel 是官方优先级 `ownerName ?? ownerId ?? t('unowned')`（TeamBoard.tsx:46）。
func (t TeamTask) ownerLabel() string {
	if n := strings.TrimSpace(t.OwnerName); n != "" {
		return n
	}
	if id := strings.TrimSpace(t.OwnerID); id != "" {
		return id
	}
	return zhUnowned
}

// blocked 是官方 `status === 'pending' && !ready`（TeamBoard.tsx:47）。
func (t TeamTask) blocked() bool { return t.Status == StatusPending && !t.Ready }

// visibleTasks 滤掉 deleted 并保持服务端顺序（官方 `tasks.filter(status !== 'deleted')`）。
func visibleTasks(tasks []TeamTask) []TeamTask {
	out := make([]TeamTask, 0, len(tasks))
	for _, task := range tasks {
		if task.Status == StatusDeleted {
			continue
		}
		out = append(out, task)
	}
	return out
}

// progressLabel 是官方 `progressLabel`（TeamBoard.tsx:29-38）：
// 完成/进行中/待处理三段，**计数为 0 的段不出现**，段间用 en-space·en-space 连接。
func progressLabel(tasks []TeamTask) string {
	completed, active := 0, 0
	for _, task := range tasks {
		switch task.Status {
		case StatusCompleted:
			completed++
		case StatusInProgress:
			active++
		}
	}
	pending := len(tasks) - completed - active
	var parts []string
	if completed > 0 {
		parts = append(parts, formatCount(zhProgressCompleted, completed))
	}
	if active > 0 {
		parts = append(parts, formatCount(zhProgressActive, active))
	}
	if pending > 0 {
		parts = append(parts, formatCount(zhProgressPending, pending))
	}
	return strings.Join(parts, progressSeparator)
}

// formatCount 只替换 `{count}`（官方 i18n 的最小子集）。
func formatCount(template string, count int) string {
	return strings.ReplaceAll(template, "{count}", itoa(count))
}

// formatMessage 只替换 `{message}` / `{reason}`。
func formatMessage(template, value string) string {
	out := strings.ReplaceAll(template, "{message}", value)
	return strings.ReplaceAll(out, "{reason}", value)
}

// itoa 避免为了一个数字引 fmt（同 dsnotifications 的做法）。
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	if i < 0 {
		return "-" + itoa(-i)
	}
	digits := [12]byte{}
	n := len(digits)
	for i > 0 {
		n--
		digits[n] = byte('0' + i%10)
		i /= 10
	}
	return string(digits[n:])
}
