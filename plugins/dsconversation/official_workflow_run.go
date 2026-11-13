// official_workflow_run.go —— §5.20 工作流运行面板（官方 `WorkflowRunPanel`，489 行）。
//
// # 官方包与座位
//
// 官方包：`@deepseek-ai/dsh-client-ui-workflow-run@0.2.2-rc.2`
// （`sha256 src/client/WorkflowRunPanel.tsx = 7d51903eb8b76579565fdb858271bbd6443e5db49c6c4d92d0b0c337fde78439`）。
//
// 注册点（唯一）：

//	ctx.uiConversation.events.register(workflowRunDefinition)   // 数据：事件 → 节点
//	ctx.slots.inject('conversation.chat.node', () => ctx.slots.register({
//	    name: 'conversation.chat.node', key: 'workflow-run', locale: 'workflowRun',
//	    inject: () => ({ openSession: t => ctx.uiWorkspace.openSession(t) }),
//	}, WorkflowRunPanel))
//	  —— `dsh-client-ui-workflow-run/src/client/index.ts:27-36`
//
// # 为什么落在 `plugins/dsconversation` 而不是新建 `plugins/dsworkflowrun/`
//
// 判据是**座位语义**，不是包名：
//
//	① 官方挂的是 `conversation.chat.node` 的 **keyed 子槽**（`key: 'workflow-run'`），
//	   不是独立 Panel 座位。本仓 20 种 `ChatNodeKind` 全部由
//	   `OfficialTranscriptView.drawRow` 的 switch 分派（`official_transcript_view.go:265`），
//	   节点表 `official_transcript_nodes.go:299-307` 早已登记 `ChatNodeWorkflowRun`。
//	② 本仓的槽位归属是「一个槽一个插件」：新建 `plugins/dsworkflowrun/` 也仍然要
//	   在 `dsconversation` 的分派里出现一次，等于把同一张表拆两处——正是节点表
//	   注释里点名要避免的「两处都要改，漏改的表现是静默少画一行」。
//	③ 官方该包的战位依赖（`ui-chat` / `ui-conversation` / `ui-renderer` /
//	   `ui-session` / `ui-workspace` / `api-session-controller` / `locale`）在本仓
//	   全部由 `plugins/dsconversation` 一家持有。
//
// 所以：**并入 `dsconversation`**，本文件是它的渲染 + 状态机实现。
//
// # 三层嵌套视图（官方真值，不得压平）
//
//	run（section[data-workflow-run][data-run-status]）
//	└── phase（div.phase）
//	    └── member（button.memberButton | div.memberRow[data-member-status]）
//
// 每一层都是**一条**官方 `DisclosureRow`：run 与 phase 各自可展开/折叠，
// member 是最内层叶子行（不可展开，只有"能不能点进子会话"两态）。
//
// # 状态机（官方 `WorkflowRunPanel.tsx:72-168` 逐函数移植）
//
// 披露态的"自动开合"不是简单的 `open = status === 'running'`，官方用
// **clean / running / abnormal** 三档 + `activityCount` 做事实比较：
//
//	clean     → 折叠（没有值得看的东西）
//	running   → 展开（有活在看）
//	abnormal  → 强制展开（失败/取消/中断优先于用户的折叠）
//
// 外加两条延迟规则（`pendingCleanCollapse`）：
//
//	① 从"有活"变 clean 时，若焦点还在内容里 → 先挂起，等 blur 再收
//	② 外层被卸载（Phase 内容消失）时拿不到 blur → 由 `useLayoutEffect` 收尾
//
// # 与官方的唯一实现差异（derived，登记在几何表与报告里）
//
// **Viva 没有 DOM 焦点模型**：官方 `focusIsWithin(element)` 读
// `element.ownerDocument.activeElement`，本仓渲染器拿不到这个事实，因此
// 渲染路径**恒传 focusWithin = false**。这不是"少实现一条"：官方在
// `focusWithin === false` 时的分支就是"立刻折叠"，与我们的行为逐字相同。
// 状态机本身保留 `focusWithin` 形参并被单测两条分支都覆盖（
// `TestWorkflowDisclosureFocusWithinBranches`），将来 Viva 有焦点事实时
// 只需把实参改成真值。
package dsconversation

import (
	"strconv"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 数据模型（官方 `workflow-definition.ts:11-35`）─────────────────────

// WorkflowRunStatus 是 run / phase / member 三处共用的状态词（官方闭合五值）。
//
// **闭合**：`dotState` 的 switch 对每个变体都有分支，多一个值就是编译期
// 不可达；少一个值会让 `status.running` 这类文案查不到。
type WorkflowRunStatus string

const (
	// WorkflowStatusRunning 是 run 未 settle、member 未 settle 的活态。
	WorkflowStatusRunning WorkflowRunStatus = "running"
	// WorkflowStatusCompleted 是脚本跑到 `return`（`stopReason: 'completed'`）。
	WorkflowStatusCompleted WorkflowRunStatus = "completed"
	// WorkflowStatusFailed 是脚本抛错（`stopReason: 'error'` → `failed`）。
	WorkflowStatusFailed WorkflowRunStatus = "failed"
	// WorkflowStatusCancelled 是调用方取消。
	WorkflowStatusCancelled WorkflowRunStatus = "cancelled"
	// WorkflowStatusInterrupted 是**没有 run-end 但 Turn 已终结**（崩溃 / fork 边界）。
	WorkflowStatusInterrupted WorkflowRunStatus = "interrupted"
)

// WorkflowRunMemberData 是一个成员行的最终渲染数据（官方 `WorkflowRunMemberData`）。
type WorkflowRunMemberData struct {
	// Seq 是 `agent()` 调用在本次 run 里的 1 基序号（也是官方 React key）。
	Seq int
	// Label 是显示标签（`label` 选项，或 prompt 摘要）。空串是**合法身份**
	// （官方用 `member.empty` 文案兜底），不是"缺失"。
	Label string
	// ChildID 是子会话 id（`subagent` 缝上的身份）。
	ChildID string
	// Status 是这一行的状态。
	Status WorkflowRunStatus
}

// WorkflowRunPhaseData 是一个阶段分组的最终渲染数据。
//
// Phase 用 `*string` 而不是 string：官方是 `string | null`，其中
// **`null`（字段缺省）与 `""`（空阶段名）是两个不同身份**，必须能区分——
// 否则 `phase.unassigned` 与 `phase.empty` 两条文案会落到同一行。
type WorkflowRunPhaseData struct {
	// Key 是官方 `workflowPhaseKey(phase)`：`missing` 或 `value:<len>:<phase>`。
	Key string
	// Phase 是官方原文；nil = 字段缺省。
	Phase *string
	// Members 是这一阶段的成员，顺序 = `agent-start` 到达顺序。
	Members []WorkflowRunMemberData
}

// WorkflowRunChatData 是 `workflow-run` 节点的完整载荷（官方 `WorkflowRunChatData`）。
type WorkflowRunChatData struct {
	// Name 是 workflow 名（`meta.name`）。空串是合法身份。
	Name string
	// Status 是 run 级状态。
	Status WorkflowRunStatus
	// Phases 是阶段分组，顺序 = 首次出现顺序（官方 Map 插入序）。
	Phases []WorkflowRunPhaseData
}

// MemberCount 是 run 的成员总数（= 各 phase 成员数之和，官方 `runFacts.activityCount`）。
func (d *WorkflowRunChatData) MemberCount() int {
	if d == nil {
		return 0
	}
	total := 0
	for _, phase := range d.Phases {
		total += len(phase.Members)
	}
	return total
}

// WorkflowPhaseKey 造无碰撞的阶段键，保留"缺省 vs 空串"两个身份。
//
// 官方 `workflowPhaseKey`（`workflow-definition.ts:59-61`）逐字移植：
//
//	null → "missing"
//	其他 → "value:" + len(phase) + ":" + phase
//
// 为什么带长度：`value:1:x` 与 `value:2:xx` 不会撞；不带长度时
// 相邻两段拼起来可能撞（`"value:"+a+b` 的经典形状）。
func WorkflowPhaseKey(phase *string) string {
	if phase == nil {
		return "missing"
	}
	return "value:" + strconv.Itoa(len(*phase)) + ":" + *phase
}

// ── StateDot（官方 primitives 五值 → 本仓四色）─────────────────────────

// WorkflowDotState 是官方 `StateDotState` 在面板里实际用到的四个值。
//
// 官方五值是 `ongoing | done | error | warning | idle`；`idle` 面板不会产出
// （`dotState` 的 switch 覆盖 `WorkflowRunStatus` 全部五值，没有落到 idle 的）。
type WorkflowDotState string

const (
	// WorkflowDotOngoing 是 running（品牌色转圈）。
	WorkflowDotOngoing WorkflowDotState = "ongoing"
	// WorkflowDotDone 是 completed（成功色实心点）。
	WorkflowDotDone WorkflowDotState = "done"
	// WorkflowDotError 是 failed（错误色实心点）。
	WorkflowDotError WorkflowDotState = "error"
	// WorkflowDotWarning 是 cancelled / interrupted（警告色实心点）。
	WorkflowDotWarning WorkflowDotState = "warning"
)

// WorkflowDotStateOf 把 run 状态映射成 StateDot 状态（官方 `dotState`，:39-49）。
func WorkflowDotStateOf(status WorkflowRunStatus) WorkflowDotState {
	switch status {
	case WorkflowStatusRunning:
		return WorkflowDotOngoing
	case WorkflowStatusCompleted:
		return WorkflowDotDone
	case WorkflowStatusFailed:
		return WorkflowDotError
	case WorkflowStatusCancelled, WorkflowStatusInterrupted:
		return WorkflowDotWarning
	}
	// 官方这里写 `default: return status satisfies never`（闭合性检查）。
	// Go 没有 satisfies，用零值兜底 + 专门断言钉住"不存在第六种状态"。
	return ""
}

// WorkflowDotColor 取 StateDot 的颜色。
//
// 官方色（`index-CSAdOBx6.css` L1:2971/3047/3123/3271）：
//
//	done     → var(--dsw-alias-state-success-primary)
//	warning  → var(--dsw-alias-state-warn-primary)
//	error    → var(--dsw-alias-state-error-primary)
//	ongoing  → var(--dsw-alias-label-tertiary)
func WorkflowDotColor(t *pluginapi.Theme, state WorkflowDotState) ui.Color {
	switch state {
	case WorkflowDotDone:
		return t.SuccessPrimary
	case WorkflowDotWarning:
		return t.Warn
	case WorkflowDotError:
		return t.Error
	default:
		return t.LabelTertiary
	}
}

// ── 文案（官方 `locales.ts` 的 zh 字典，逐条照抄）──────────────────────

const (
	// WorkflowTextRunEmpty 是「没有任何 phase」时的空态（`run.empty`）。
	WorkflowTextRunEmpty = "没有启动成员"
	// WorkflowTextPhaseUnassigned 是 `phase === null` 的兜底（`phase.unassigned`）。
	WorkflowTextPhaseUnassigned = "未分阶段"
	// WorkflowTextPhaseEmpty 是 `phase === ""` 的兜底（`phase.empty`）。
	WorkflowTextPhaseEmpty = "空阶段名"
	// WorkflowTextMemberEmpty 是 `label === ""` 的兜底（`member.empty`）。
	WorkflowTextMemberEmpty = "空成员名"
	// WorkflowTextMemberCountSuffix 是成员计数后缀（`run.members.*` = "{count} 个成员"）。
	WorkflowTextMemberCountSuffix = " 个成员"
)

// workflowStatusText 是五个状态的显示文案（官方 `status.*`）。
var workflowStatusText = map[WorkflowRunStatus]string{
	WorkflowStatusRunning:     "运行中",
	WorkflowStatusCompleted:   "已完成",
	WorkflowStatusFailed:      "失败",
	WorkflowStatusCancelled:   "已取消",
	WorkflowStatusInterrupted: "已中断",
}

// WorkflowStatusText 取状态文案；未知状态返回空串（**不编**）。
func WorkflowStatusText(status WorkflowRunStatus) string {
	return workflowStatusText[status]
}

// WorkflowMemberCount 造成员计数文案（官方 `memberCount`，:68-70）。
//
// 官方按 `count === 1` 选 `run.members.one` / `run.members.other`，
// 而中文两条**同形**（都是 "{count} 个成员"）——所以这里不需要复数分支，
// 但 key 的选择逻辑仍保留在注释里（英文/日文等语言非同形时才有意义）。
func WorkflowMemberCount(count int) string {
	return strconv.Itoa(count) + WorkflowTextMemberCountSuffix
}

// WorkflowStatusCount 造 "状态 {count}" 文案（官方 `statusCount.<status>`）。
func WorkflowStatusCount(status WorkflowRunStatus, count int) string {
	return WorkflowStatusText(status) + " " + strconv.Itoa(count)
}

// WorkflowReadablePhase 取阶段标题（官方 `readablePhase`，:51-54）。
func WorkflowReadablePhase(phase *string) string {
	if phase == nil {
		return WorkflowTextPhaseUnassigned
	}
	if *phase == "" {
		return WorkflowTextPhaseEmpty
	}
	return *phase
}

// WorkflowReadableMember 取成员标签（官方 `readableMember`，:56-58）。
func WorkflowReadableMember(label string) string {
	if label == "" {
		return WorkflowTextMemberEmpty
	}
	return label
}

// WorkflowPhaseStatusSummary 造阶段行的状态摘要（官方 `phaseStatusSummary`，:170-181）。
//
// 规则（**顺序即语义**，不得重排）：
//
//	① 统计每个状态的成员数
//	② 取"活态"四个：running / failed / cancelled / interrupted（**不含 completed**）
//	③ 一个活态都没有 → 只报 `已完成 {n}`
//	④ 否则：若 interrupted 与 completed **同时存在**，把 completed 提到最前
//	⑤ 用 " · " 连接
//
// 为什么第 ④ 条单独写：中断的 run 里"已完成的成员"是真实信息（说明有些活干完了），
// 而纯 cancelled 的 run 里报 completed 会让人以为整体成功。这个特例在官方是
// 显式分支，不是巧合。
func WorkflowPhaseStatusSummary(members []WorkflowRunMemberData) string {
	counts := map[WorkflowRunStatus]int{}
	for _, member := range members {
		counts[member.Status]++
	}
	active := make([]WorkflowRunStatus, 0, 4)
	for _, status := range []WorkflowRunStatus{
		WorkflowStatusRunning, WorkflowStatusFailed,
		WorkflowStatusCancelled, WorkflowStatusInterrupted,
	} {
		if counts[status] > 0 {
			active = append(active, status)
		}
	}
	if len(active) == 0 {
		return WorkflowStatusCount(WorkflowStatusCompleted, counts[WorkflowStatusCompleted])
	}
	visible := active
	if counts[WorkflowStatusInterrupted] > 0 && counts[WorkflowStatusCompleted] > 0 {
		visible = append([]WorkflowRunStatus{WorkflowStatusCompleted}, active...)
	}
	summary := ""
	for i, status := range visible {
		if i > 0 {
			summary += " · "
		}
		summary += WorkflowStatusCount(status, counts[status])
	}
	return summary
}

// ── 披露状态机（官方 `WorkflowRunPanel.tsx:72-168` 逐函数移植）──────────

// WorkflowDisclosureMode 是披露行的三档事实（官方 `DisclosureMode`）。
type WorkflowDisclosureMode string

const (
	// WorkflowDisclosureClean 是"没有活、没有异常"——应当折叠。
	WorkflowDisclosureClean WorkflowDisclosureMode = "clean"
	// WorkflowDisclosureRunning 是"有活在看"——应当展开。
	WorkflowDisclosureRunning WorkflowDisclosureMode = "running"
	// WorkflowDisclosureAbnormal 是"有失败/取消/中断"——强制展开。
	WorkflowDisclosureAbnormal WorkflowDisclosureMode = "abnormal"
)

// WorkflowDisclosureFacts 是一次渲染能算出的**事实**（不含用户意图）。
type WorkflowDisclosureFacts struct {
	// Mode 是三档。
	Mode WorkflowDisclosureMode
	// ActivityCount 是这一层的成员数（run 层是总和）。
	ActivityCount int
}

// WorkflowDisclosureState 是运行态：事实 + 用户的开合意图 + 待收尾标记。
type WorkflowDisclosureState struct {
	WorkflowDisclosureFacts
	// Open 是当前是否展开。
	Open bool
	// PendingCleanCollapse 是"该收了但焦点还在里面"的挂起态。
	PendingCleanCollapse bool
}

// WorkflowAbnormal 报告状态是否属于异常三态（官方 `abnormal`，:95-97）。
func WorkflowAbnormal(status WorkflowRunStatus) bool {
	return status == WorkflowStatusFailed ||
		status == WorkflowStatusCancelled ||
		status == WorkflowStatusInterrupted
}

// WorkflowPhaseDisclosureFacts 算一个阶段的事实（官方 `phaseDisclosureFacts`，:99-104）。
//
// 优先级：异常 > 运行中 > clean。**异常优先**是因为一个失败的成员
// 比一个还在跑的成员更需要用户看到。
func WorkflowPhaseDisclosureFacts(phase WorkflowRunPhaseData) WorkflowDisclosureFacts {
	mode := WorkflowDisclosureClean
	for _, member := range phase.Members {
		if WorkflowAbnormal(member.Status) {
			mode = WorkflowDisclosureAbnormal
			break
		}
	}
	if mode == WorkflowDisclosureClean {
		for _, member := range phase.Members {
			if member.Status == WorkflowStatusRunning {
				mode = WorkflowDisclosureRunning
				break
			}
		}
	}
	return WorkflowDisclosureFacts{Mode: mode, ActivityCount: len(phase.Members)}
}

// WorkflowRunDisclosureFacts 算 run 级事实（官方 `runDisclosureFacts`，:106-117）。
//
// **run 级 abnormal 会被任何 phase 的 abnormal 传染**——一段失败就是整段失败，
// 这是官方的显式选择（`phases.some(([, facts]) => facts.mode === 'abnormal')`）。
func WorkflowRunDisclosureFacts(status WorkflowRunStatus, phases []WorkflowDisclosureFacts) WorkflowDisclosureFacts {
	mode := WorkflowDisclosureClean
	if WorkflowAbnormal(status) {
		mode = WorkflowDisclosureAbnormal
	} else {
		for _, facts := range phases {
			if facts.Mode == WorkflowDisclosureAbnormal {
				mode = WorkflowDisclosureAbnormal
				break
			}
		}
	}
	if mode == WorkflowDisclosureClean {
		if status == WorkflowStatusRunning {
			mode = WorkflowDisclosureRunning
		} else {
			for _, facts := range phases {
				if facts.Mode == WorkflowDisclosureRunning {
					mode = WorkflowDisclosureRunning
					break
				}
			}
		}
	}
	total := 0
	for _, facts := range phases {
		total += facts.ActivityCount
	}
	return WorkflowDisclosureFacts{Mode: mode, ActivityCount: total}
}

// WorkflowInitialDisclosureState 是首次见到这一层时的开合（官方 `initialDisclosureState`）。
//
// 判据只有一条：**clean 就折叠，其余全展开**。所以刷新页面时
// 已完成的 run 是收起的、正在跑的 run 是展开的。
func WorkflowInitialDisclosureState(facts WorkflowDisclosureFacts) WorkflowDisclosureState {
	return WorkflowDisclosureState{
		WorkflowDisclosureFacts: facts,
		Open:                    facts.Mode != WorkflowDisclosureClean,
		PendingCleanCollapse:    false,
	}
}

// WorkflowAdvanceDisclosureState 在事实变化时推进状态（官方 `advanceDisclosureState`，:123-141）。
//
// 四条分支（**顺序即语义**）：
//
//	① 事实完全没变 → 原样返回（除非挂起了收尾且焦点已离开 → 收）
//	② 新事实是 clean → 保留"焦点在里面就先不收"的延迟
//	③ 旧 clean 或 旧非 abnormal 而新 abnormal → 强制展开（异常覆盖用户的折叠）
//	④ 其余 → 保留用户意图，清掉挂起标记
func WorkflowAdvanceDisclosureState(
	current WorkflowDisclosureState,
	facts WorkflowDisclosureFacts,
	focusWithin bool,
) WorkflowDisclosureState {
	sameFacts := current.Mode == facts.Mode && current.ActivityCount == facts.ActivityCount
	if sameFacts {
		if !current.PendingCleanCollapse || focusWithin {
			return current
		}
		return WorkflowDisclosureState{
			WorkflowDisclosureFacts: current.WorkflowDisclosureFacts,
			Open:                    false,
			PendingCleanCollapse:    false,
		}
	}
	if facts.Mode == WorkflowDisclosureClean {
		deferCollapse := current.Open && focusWithin
		return WorkflowDisclosureState{
			WorkflowDisclosureFacts: facts,
			Open:                    deferCollapse,
			PendingCleanCollapse:    deferCollapse,
		}
	}
	if current.Mode == WorkflowDisclosureClean ||
		(facts.Mode == WorkflowDisclosureAbnormal && current.Mode != WorkflowDisclosureAbnormal) {
		return WorkflowDisclosureState{
			WorkflowDisclosureFacts: facts,
			Open:                    true,
			PendingCleanCollapse:    false,
		}
	}
	return WorkflowDisclosureState{
		WorkflowDisclosureFacts: facts,
		Open:                    current.Open,
		PendingCleanCollapse:    false,
	}
}

// WorkflowCollapsePending 在 blur 时收尾挂起的折叠（官方 `collapsePending`，:148-151）。
func WorkflowCollapsePending(state WorkflowDisclosureState) WorkflowDisclosureState {
	if !state.PendingCleanCollapse {
		return state
	}
	return WorkflowDisclosureState{
		WorkflowDisclosureFacts: state.WorkflowDisclosureFacts,
		Open:                    false,
		PendingCleanCollapse:    false,
	}
}

// ── 披露态存储（跨帧）─────────────────────────────────────────────────

// WorkflowRunDisclosures 是**跨帧**的披露状态（官方每个面板实例一份 React state）。
//
// 为什么放在渲染器而不是投影：开合是**用户意图**，不是 journal 事实。
// 放进投影会让"重放同一批帧"得到不同的开合态。
//
// 键的组成：run 用 entry key；phase 用 `entryKey + "\x1f" + phaseKey`。
// 用 `\x1f`（unit separator）而不是 `|`：phase 原文可能含任意可打印字符，
// 而 `\x1f` 不可能出现在用户文本里，所以拼键不会撞。
type WorkflowRunDisclosures struct {
	runs   map[string]WorkflowDisclosureState
	phases map[string]WorkflowDisclosureState
}

// NewWorkflowRunDisclosures 造一个空存储。
func NewWorkflowRunDisclosures() *WorkflowRunDisclosures {
	return &WorkflowRunDisclosures{
		runs:   map[string]WorkflowDisclosureState{},
		phases: map[string]WorkflowDisclosureState{},
	}
}

// phaseStoreKey 造 phase 的复合键。
func phaseStoreKey(entryKey, phaseKey string) string {
	return entryKey + "\x1f" + phaseKey
}

// Sync 按官方 `useLayoutEffect`（:373-400）推进一帧的披露态。
//
// 入参 `focusWithin` 对 run 与 phase 是**同一个值**：官方分别读两个 ref
// （`runContentRef` / `phaseContentRefs.get(key)`），而 Viva 没有焦点事实，
// 渲染路径恒传 false（见文件头 derived 说明）。
//
// 返回 run 级状态，供绘制使用；phase 级用 `Phase` 取。
func (d *WorkflowRunDisclosures) Sync(entryKey string, data *WorkflowRunChatData, focusWithin bool) WorkflowDisclosureState {
	phaseFacts := make([]WorkflowDisclosureFacts, len(data.Phases))
	phaseStartedCycle := false
	for i, phase := range data.Phases {
		facts := WorkflowPhaseDisclosureFacts(phase)
		phaseFacts[i] = facts
		key := phaseStoreKey(entryKey, phase.Key)
		previous, ok := d.phases[key]
		if !ok {
			d.phases[key] = WorkflowInitialDisclosureState(facts)
			continue
		}
		d.phases[key] = WorkflowAdvanceDisclosureState(previous, facts, focusWithin)
		// 官方 `phaseStartedCycle`：**上一帧是 clean** 且这一帧不再 clean
		// （或者成员数变了）。它专门用来把外层的 run 重新拉开——
		// 一个已经收起的 run 里新开了一段活，用户必须看得见。
		if previous.Mode == WorkflowDisclosureClean &&
			(facts.Mode != WorkflowDisclosureClean || facts.ActivityCount != previous.ActivityCount) {
			phaseStartedCycle = true
		}
	}
	// 清掉**本 run 自己**这一帧已经不存在的 phase（run 被重建 / 换了成员集合）。
	//
	// 只扫本 run 的键前缀：不加前缀判断的话，渲染 run A 会把 run B 的
	// phase 披露态一起删掉（同一个 transcript 里可以有多个 workflow 节点）。
	prefix := entryKey + "\x1f"
	for key := range d.phases {
		if len(key) <= len(prefix) || key[:len(prefix)] != prefix {
			continue
		}
		if !hasPhaseKey(key[len(prefix):], data.Phases) {
			delete(d.phases, key)
		}
	}

	runFacts := WorkflowRunDisclosureFacts(data.Status, phaseFacts)
	current, ok := d.runs[entryKey]
	if !ok {
		current = WorkflowInitialDisclosureState(runFacts)
	}
	advancedRun := WorkflowAdvanceDisclosureState(current, runFacts, focusWithin)
	run := advancedRun
	if phaseStartedCycle && runFacts.Mode != WorkflowDisclosureClean && !advancedRun.Open {
		run = WorkflowDisclosureState{
			WorkflowDisclosureFacts: advancedRun.WorkflowDisclosureFacts,
			Open:                    true,
			PendingCleanCollapse:    false,
		}
	}
	d.runs[entryKey] = run
	return run
}

// hasPhaseKey 报告某个 phase 键是否仍在 `phases` 里。
func hasPhaseKey(phaseKey string, phases []WorkflowRunPhaseData) bool {
	for _, phase := range phases {
		if phase.Key == phaseKey {
			return true
		}
	}
	return false
}

// Phase 取某个 phase 的披露态；不存在时按事实算初始值（不落库——纯读）。
func (d *WorkflowRunDisclosures) Phase(entryKey string, phase WorkflowRunPhaseData) WorkflowDisclosureState {
	if state, ok := d.phases[phaseStoreKey(entryKey, phase.Key)]; ok {
		return state
	}
	return WorkflowInitialDisclosureState(WorkflowPhaseDisclosureFacts(phase))
}

// Run 取某个 run 的披露态；不存在时按事实算初始值（纯读）。
func (d *WorkflowRunDisclosures) Run(entryKey string, data *WorkflowRunChatData) WorkflowDisclosureState {
	if state, ok := d.runs[entryKey]; ok {
		return state
	}
	phaseFacts := make([]WorkflowDisclosureFacts, len(data.Phases))
	for i, phase := range data.Phases {
		phaseFacts[i] = WorkflowPhaseDisclosureFacts(phase)
	}
	return WorkflowInitialDisclosureState(WorkflowRunDisclosureFacts(data.Status, phaseFacts))
}

// ToggleRun 翻转 run 的开合（官方 `toggleRun`，:402-411）。
func (d *WorkflowRunDisclosures) ToggleRun(entryKey string, data *WorkflowRunChatData) {
	current := d.Run(entryKey, data)
	current.Open = !current.Open
	current.PendingCleanCollapse = false
	d.runs[entryKey] = current
}

// TogglePhase 翻转一个 phase 的开合（官方 `togglePhase`，:412-423）。
func (d *WorkflowRunDisclosures) TogglePhase(entryKey string, phase WorkflowRunPhaseData) {
	current := d.Phase(entryKey, phase)
	current.Open = !current.Open
	current.PendingCleanCollapse = false
	d.phases[phaseStoreKey(entryKey, phase.Key)] = current
}

// SettleRunBlur 在 run 内容失焦时收尾（官方 `settleRunBlur`，:424-430）。
func (d *WorkflowRunDisclosures) SettleRunBlur(entryKey string, data *WorkflowRunChatData) {
	current := d.Run(entryKey, data)
	next := WorkflowCollapsePending(current)
	if next != current {
		d.runs[entryKey] = next
	}
}

// SettlePhaseBlur 在 phase 内容失焦时收尾（官方 `settlePhaseBlur`，:431-441）。
func (d *WorkflowRunDisclosures) SettlePhaseBlur(entryKey string, phase WorkflowRunPhaseData) {
	current := d.Phase(entryKey, phase)
	next := WorkflowCollapsePending(current)
	if next != current {
		d.phases[phaseStoreKey(entryKey, phase.Key)] = next
	}
}

// Reset 丢弃全部披露态（切会话 / 重建基线时调用）。
func (d *WorkflowRunDisclosures) Reset() {
	d.runs = map[string]WorkflowDisclosureState{}
	d.phases = map[string]WorkflowDisclosureState{}
}

// ── 图标（官方 sprite，逐 path 照抄）──────────────────────────────────

// workflowChevronRightIcon 是官方 `IconChevronRightOutlineRegular`。
//
// 来源：`dsh-web-frontend/dist/assets/index-ghOpC9uV.js` L129:12142（registry 条目），
// 该文件 sha256 = `061de8cefa15e1fbb87d301463ad4ebb79259b197ca08e83406f6997c1af8bb8`。
//
//	viewBox="0 0 16 16" fill="none" stroke-width=1
//	path d="M6 12L9.29289 8.70711C9.68342 8.31658 9.68342 7.68342 9.29289 7.29289L6 4"
//
// 显示尺寸 14×14（registry 的 `size:14`，**不是** viewBox 16）。
func workflowChevronRightIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Cap:     gfx.CapRound,
		Join:    gfx.JoinRound,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(6, 12),
			gfx.LineTo(9.29289, 8.70711),
			gfx.CubicTo(9.68342, 8.31658, 9.68342, 7.68342, 9.29289, 7.29289),
			gfx.LineTo(6, 4),
		},
	}
}

// workflowChevronUpIcon 是官方 `IconChevronUpOutlineRegular`（展开态）。
//
// 来源同上文件 L129:12142 相邻条目：
//
//	viewBox="0 0 16 16" fill="none" stroke-width=1
//	path d="M12 10L8.70711 6.70711C8.31658 6.31658 7.68342 6.31658 7.29289 6.70711L4 10"
//
// 为什么展开态换图标而不是旋转：官方 `DisclosureRow` 的 `G = open ? <Kw/> : V`
// ——展开时**换一个**上箭头，不是给右箭头加 rotate。DSH 的箭头是"语义方向"
// 不是"物理旋转"，两者字重/端点不完全等价。
func workflowChevronUpIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Cap:     gfx.CapRound,
		Join:    gfx.JoinRound,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(12, 10),
			gfx.LineTo(8.70711, 6.70711),
			gfx.CubicTo(8.31658, 6.31658, 7.68342, 6.31658, 7.29289, 6.70711),
			gfx.LineTo(4, 10),
		},
	}
}

// ── 绘制 ──────────────────────────────────────────────────────────────

// WorkflowRunPanel 是一次绘制的入参：节点载荷 + 交互 id 种子 + 开合存储。
type WorkflowRunPanel struct {
	// Data 是投影好的载荷；nil = 这一帧不画（调用方保证非 nil）。
	Data WorkflowRunChatData
	// EntryKey 是节点稳定键（官方 `context.key`），也是披露态的键。
	EntryKey string
	// ID 造交互 id（`env.Tree.ID(env.Feat, …)` 的绑定）；nil = 不可交互。
	ID func(name string) gfx.ID
	// OpenSession 是成员行的导航回调（官方注入的 `openSession`）。
	//
	// nil 表示**这条渲染路径没有导航能力**：此时成员行退化成官方
	// `MemberRow` 的 `renderButton === false` 分支（`div.memberRow`，无 button、
	// 无 aria-label、tabIndex=-1）。这不是"少实现"，是官方两条分支里的一条。
	OpenSession func(childID string)
	// Disclosures 是跨帧的披露态；nil = 用一次性存储（测试/单帧渲染）。
	Disclosures *WorkflowRunDisclosures
	// Theme 是主题；nil 时用渲染器自己的。
	Theme *pluginapi.Theme
	// Narrow 对应官方 `@media (max-width:560px)`：把 phaseList / members
	// 的左缩进从 16 收到 12（WorkflowRunPanel.module.css:267-272）。
	//
	// 官方用媒体查询读**视口**宽度；Viva 没有媒体查询，调用方传
	// `env.Bounds.Width <= WorkflowRunNarrowMaxWidth`。断点值本身也在几何表里
	// （`WorkflowRunPanel.phaseList.media-max-width`），所以"同一个断点"是可断言的。
	Narrow bool
}

// phaseListPadLeft 取 phaseList / members 的左缩进（16 或窄窗 12）。
func (p *WorkflowRunPanel) phaseListPadLeft() float32 {
	if p.Narrow {
		return WorkflowRunPhaseListPadLeftNarrow
	}
	return WorkflowRunPhaseListPadLeft
}

// membersPadLeft 取 `.members` 的左缩进（与 phaseList 同档）。
func (p *WorkflowRunPanel) membersPadLeft() float32 {
	if p.Narrow {
		return WorkflowRunPhaseListPadLeftNarrow
	}
	return WorkflowRunMembersPadLeft
}

// Draw 画完整面板：run 披露行 → phase 列表 → member 行。
//
// 返回这一帧的 run 披露态（测试断言"该展开的真的展开了"用）。
func (p *WorkflowRunPanel) Draw(f ui.C) WorkflowDisclosureState {
	t := p.Theme
	store := p.Disclosures
	if store == nil {
		store = NewWorkflowRunDisclosures()
	}
	// Viva 没有焦点事实 → 恒 false（见文件头 derived 说明）。
	runState := store.Sync(p.EntryKey, &p.Data, false)

	root := ui.Column(f).FillWidth().MinWidth(WorkflowRunRootMinWidth)
	root.Children(func() {
		p.drawRunHeader(f, t, store, runState)
	})
	return runState
}

// drawRunHeader 画 run 层：官方 `RunHeader` + `section.root`（:206-243 / :443-457）。
//
// 几何（`WorkflowRunPanel.module.css:1-22` + primitives `_row_` 合并后）：
//
//	section.root       width 100%; min-width 0
//	.runHeader         box-sizing border-box; flex 1 1 auto; display flex;
//	                   align-items center; gap 6; width 100%; min-width 0;
//	                   height 32; padding 0 8; border-radius md;
//	                   background --dsw-alias-bg-module-platform; color label-tertiary
//	.runLeading        inline-flex; flex none; 16×16; center; margin-right 0
//	.runTitle          flex none; max-width 42%; 13px/24px; weight 510; ellipsis
//	.runSummary        flex 1; min-width 0; 13px/18px; ellipsis
//	.statusTail        inline-flex; flex none; height 20; align center; gap 4;
//	                   11px/16px; weight 510
func (p *WorkflowRunPanel) drawRunHeader(
	f ui.C,
	t *pluginapi.Theme,
	store *WorkflowRunDisclosures,
	state WorkflowDisclosureState,
) {
	section := ui.Column(f).FillWidth().MinWidth(WorkflowRunRootMinWidth)
	section.Children(func() {
		row := ui.Row(f).FillWidth().Items(ui.Center).Gap(WorkflowRunHeaderGap).
			Height(WorkflowRunHeaderHeight).
			PadSides(0, WorkflowRunHeaderPadX, 0, WorkflowRunHeaderPadX).
			Radius(t.Radii.MD).Background(t.BgLayer3).MinWidth(0)
		// 整行是命中区（官方 `DisclosureRow` 的 `row` 在 `expandOnRowClick`
		// 时挂 `role="button"` + `onClick`）。
		if p.ID != nil {
			// 整行是命中区：可访问性标签用 run 名（**不是**阶段兜底文案）。
			if row.Hit(p.ID("workflow-run-header-"+p.EntryKey), p.Data.Name).Clicked() {
				store.ToggleRun(p.EntryKey, &p.Data)
			}
		}
		row.Children(func() {
			// .runLeading：16×16 的图标槽（margin-right 0——间距由行的 gap 6 给）。
			ui.Row(f).Width(WorkflowRunLeadingSize).Height(WorkflowRunLeadingSize).
				Center().Children(func() {
				ui.Icon(f, p.chevron(state.Open)).
					Size(WorkflowRunChevronSize, WorkflowRunChevronSize).
					Color(t.LabelTertiary)
			})
			// .runTitle：42% 上限的单行省略。
			title := ui.Text(f, p.Data.Name).
				FontSize(t.Fonts.Secondary).LineHeight(WorkflowRunRowLineHeight).
				Weight(WorkflowRunTitleWeight).Color(t.LabelPrimary).
				MaxWidthPercent(WorkflowRunTitleMaxWidth).Ellipsis()
			title.Shrink(0)
			// 折叠态内容（官方 `keepContentWhenOpen` 为 true：**展开时也画**）。
			ui.Row(f).Items(ui.Center).Gap(WorkflowRunHeaderContentGap).
				Grow(1).MinWidth(0).Children(func() {
				// .separator：2×2 圆点，label-tertiary。
				ui.Slot(f).Size(WorkflowRunSeparatorSize, WorkflowRunSeparatorSize).
					Radius(WorkflowRunSeparatorSize / 2).Background(t.LabelTertiary)
				// .runSummary：flex 1，13px/18px。
				ui.Text(f, WorkflowMemberCount(p.Data.MemberCount())).
					FontSize(t.Fonts.Secondary).LineHeight(WorkflowRunSummaryLineHeight).
					Color(t.LabelSecondary).Grow(1).MinWidth(0).Ellipsis()
				// .statusTail：StateDot + 状态文案。
				ui.Row(f).Items(ui.Center).Gap(WorkflowRunStatusTailGap).
					Height(WorkflowRunStatusTailHeight).Shrink(0).Children(func() {
					p.drawStateDot(f, t, WorkflowDotStateOf(p.Data.Status))
					ui.Text(f, WorkflowStatusText(p.Data.Status)).
						FontSize(WorkflowRunStatusTailFontSize).
						LineHeight(WorkflowRunStatusTailLineHeight).
						Weight(WorkflowRunTitleWeight).
						Color(t.LabelSecondary)
				})
			})
		})
		if !state.Open {
			return
		}
		p.drawPhaseList(f, t, store)
	})
}

// drawPhaseList 画 phase 列表（官方 `.phaseList`，:145-151 / :460-485）。
//
//	.phaseList  display flex; flex-direction column; gap 4; min-width 0;
//	            padding 4px 0 0 16px
//	.empty      color label-tertiary; 13px/20px; padding 0
func (p *WorkflowRunPanel) drawPhaseList(f ui.C, t *pluginapi.Theme, store *WorkflowRunDisclosures) {
	list := ui.Column(f).FillWidth().Gap(WorkflowRunPhaseListGap).MinWidth(0).
		PadSides(WorkflowRunPhaseListPadTop, 0, 0, p.phaseListPadLeft())
	list.Children(func() {
		if len(p.Data.Phases) == 0 {
			ui.Text(f, WorkflowTextRunEmpty).
				FontSize(t.Fonts.Secondary).LineHeight(WorkflowRunPhaseCountLineHeight).
				Color(t.LabelTertiary)
			return
		}
		for _, phase := range p.Data.Phases {
			p.drawPhase(f, t, store, phase)
		}
	})
}

// drawPhase 画一个阶段（官方 `PhaseSection`，:292-348）。
//
//	.phase          min-width 0
//	.phaseHeader    box-sizing border-box; display flex; align-items center;
//	                gap 6; width 100%; min-width 0; height 32
//	.phaseLeading   16×16（同 runLeading）
//	.phaseTitle     flex 0 1 auto; min-width 0; max-width 42%; 13px/24px
//	.phaseCount     flex 1; min-width 0; 13px/20px
//	.phaseStatus    flex none; width 132px; 13px/20px; text-align right
//	.members        display flex; flex-direction column; gap 2; min-width 0;
//	                padding 0 0 0 16px
func (p *WorkflowRunPanel) drawPhase(
	f ui.C,
	t *pluginapi.Theme,
	store *WorkflowRunDisclosures,
	phase WorkflowRunPhaseData,
) {
	state := store.Phase(p.EntryKey, phase)
	wrap := ui.Column(f).FillWidth().MinWidth(WorkflowRunRootMinWidth)
	wrap.Children(func() {
		row := ui.Row(f).FillWidth().Items(ui.Center).Gap(WorkflowRunHeaderGap).
			Height(WorkflowRunHeaderHeight).MinWidth(0)
		if p.ID != nil {
			if row.Hit(p.ID("workflow-run-phase-"+p.EntryKey+"-"+phase.Key),
				WorkflowReadablePhase(phase.Phase)).Clicked() {
				store.TogglePhase(p.EntryKey, phase)
			}
		}
		row.Children(func() {
			ui.Row(f).Width(WorkflowRunLeadingSize).Height(WorkflowRunLeadingSize).
				Center().Children(func() {
				ui.Icon(f, p.chevron(state.Open)).
					Size(WorkflowRunChevronSize, WorkflowRunChevronSize).
					Color(t.LabelTertiary)
			})
			// .phaseTitle 是 `flex:0 1 auto`——**与 runTitle 不同**：
			// phase 标题允许被压缩（run 标题是 flex:none）。
			title := ui.Text(f, WorkflowReadablePhase(phase.Phase)).
				FontSize(t.Fonts.Secondary).LineHeight(WorkflowRunRowLineHeight).
				Color(t.LabelSecondary).MinWidth(0).
				MaxWidthPercent(WorkflowRunTitleMaxWidth).Ellipsis()
			title.Grow(0).Shrink(1)
			ui.Row(f).Items(ui.Center).Gap(WorkflowRunHeaderContentGap).
				Grow(1).MinWidth(0).Children(func() {
				ui.Slot(f).Size(WorkflowRunSeparatorSize, WorkflowRunSeparatorSize).
					Radius(WorkflowRunSeparatorSize / 2).Background(t.LabelTertiary)
				ui.Text(f, WorkflowMemberCount(len(phase.Members))).
					FontSize(t.Fonts.Secondary).LineHeight(WorkflowRunPhaseCountLineHeight).
					Color(t.LabelSecondary).Grow(1).MinWidth(0).Ellipsis()
				ui.Text(f, WorkflowPhaseStatusSummary(phase.Members)).
					FontSize(t.Fonts.Secondary).LineHeight(WorkflowRunPhaseCountLineHeight).
					Color(t.LabelSecondary).Shrink(0).
					Width(WorkflowRunPhaseStatusWidth).TextEnd()
			})
		})
		if !state.Open {
			return
		}
		members := ui.Column(f).FillWidth().Gap(WorkflowRunMembersGap).MinWidth(0).
			PadSides(0, 0, 0, p.membersPadLeft())
		members.Children(func() {
			for _, member := range phase.Members {
				p.drawMember(f, t, member)
			}
		})
	})
}

// drawMember 画一行成员（官方 `MemberRow`，:245-290）。
//
// 两条形态（官方 `renderButton = navigable || focused`）：
//
//	navigable          → `button.memberButton`（cursor pointer；label 用 link 色 + 500 字重；
//	                     aria-label = `member.open`）
//	不可导航且未聚焦    → `div.memberRow`（无 button、无 aria-label、tabIndex=-1）
//
// 几何（`.memberRow, .memberButton` 合并）：
//
//	display flex; align-items center; gap 12; width 100%; min-width 0;
//	min-height 24; padding 0; border 0; border-radius sm;
//	background transparent; color label-secondary; font inherit; text-align left
//
//	.dotSlot          inline-flex; flex none; 16×24; center; overflow hidden
//	.memberLabelWrap  display flex; overflow hidden; flex 1; min-width 0;
//	                  height 24; align-items center; padding 0 2; border-radius sm
//	.memberLabel      overflow hidden; flex 1; min-width 0; color label-secondary;
//	                  14px/24px; ellipsis
//	.memberStatus     flex none; overflow hidden; width 64; color label-secondary;
//	                  13px/20px; text-align right; ellipsis
func (p *WorkflowRunPanel) drawMember(f ui.C, t *pluginapi.Theme, member WorkflowRunMemberData) {
	navigable := p.OpenSession != nil && member.ChildID != "" &&
		member.Status == WorkflowStatusRunning
	name := WorkflowReadableMember(member.Label)
	content := func() {
		ui.Row(f).Width(WorkflowRunDotSlotWidth).Height(WorkflowRunMemberMinHeight).
			Center().Clip().Children(func() {
			p.drawStateDot(f, t, WorkflowDotStateOf(member.Status))
		})
		ui.Row(f).Height(WorkflowRunMemberLabelWrapHeight).Items(ui.Center).
			PadSides(0, WorkflowRunMemberLabelWrapPadX, 0, WorkflowRunMemberLabelWrapPadX).
			Radius(t.Radii.SM).Grow(1).MinWidth(0).Clip().Children(func() {
			ui.Text(f, name).FontSize(t.Fonts.Body).
				LineHeight(WorkflowRunMemberLabelWrapHeight).
				Weight(WorkflowRunMemberLabelWeight).
				Color(t.LabelSecondary).Grow(1).MinWidth(0).Ellipsis()
		})
		ui.Text(f, WorkflowStatusText(member.Status)).
			FontSize(t.Fonts.Secondary).LineHeight(WorkflowRunPhaseCountLineHeight).
			Color(t.LabelSecondary).Shrink(0).Width(WorkflowRunMemberStatusWidth).TextEnd()
	}
	if !navigable {
		// 不可导航：官方 `div.memberRow`（**不是** button）。
		ui.Row(f).FillWidth().Items(ui.Center).Gap(WorkflowRunMemberGap).
			MinHeight(WorkflowRunMemberMinHeight).MinWidth(0).Children(content)
		return
	}
	button := ui.Row(f).FillWidth().Items(ui.Center).Gap(WorkflowRunMemberGap).
		MinHeight(WorkflowRunMemberMinHeight).MinWidth(0).Radius(t.Radii.SM)
	if p.ID != nil {
		if button.Hit(p.ID("workflow-run-member-"+p.EntryKey+"-"+strconv.Itoa(member.Seq)),
			name).Clicked() {
			p.OpenSession(member.ChildID)
		}
	}
	button.Children(content)
}

// drawStateDot 画官方 `StateDot`（`index-ghOpC9uV.js` L129:82467 的 dot 分支）。
//
// dot 分支的 DOM 与几何（`index-CSAdOBx6.css` L1:2788 / L1:2851）：
//
//	._dot_   position relative; display inline-block; flex none
//	._dot_:after  content ""; position absolute; inset 20%; border-radius 50%;
//	             corner-shape round; background currentColor
//	尺寸     ongoing → 14（spinner 分支）；done/error/warning → 10
//
// 所以实心圆直径 = 10 × (1 - 2×20%) = 6px，与 `dsagentteam` 的 6px dot 同值。
//
// ongoing 官方是 1.5s 线性旋转的 SVG spinner（viewBox 24、两圆 r=9.5、stroke 2、
// track opacity .25、arc dasharray "12 150"）。**Viva 画布是静态的**：这里画
// 同一份几何但**不旋转**——旋转是时长事实，静态截图与几何断言都拿不到它。
// 该差异登记为 derived（几何表 WorkflowRunPanel.stateDot.spinner 行）。
func (p *WorkflowRunPanel) drawStateDot(f ui.C, t *pluginapi.Theme, state WorkflowDotState) {
	color := WorkflowDotColor(t, state)
	size := WorkflowRunDotSize
	if state == WorkflowDotOngoing {
		size = WorkflowRunDotSizeOngoing
	}
	box := ui.Row(f).Width(size).Height(size).Center()
	box.Children(func() {
		ui.Slot(f).Size(WorkflowRunDotCoreSize, WorkflowRunDotCoreSize).
			Radius(WorkflowRunDotCoreSize / 2).Background(color)
	})
}

// chevron 取折叠箭头：展开 → 上箭头，折叠 → 右箭头（官方 `G = open ? <Kw/> : V`）。
func (p *WorkflowRunPanel) chevron(open bool) gfx.Icon {
	if open {
		return workflowChevronUpIcon()
	}
	return workflowChevronRightIcon()
}
