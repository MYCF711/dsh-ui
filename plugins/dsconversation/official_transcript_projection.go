// official_transcript_projection.go —— 把 journal 原始帧**投影成逐条消息节点**。
//
// # 为什么要一层投影
//
// 宿主的 Transport journal 推的是 v4 原始事件（`user/message` / `assistant/message` /
// `tool/call` …），而主区要画的是**节点**（`ChatNodeDataMap` 的 20 个 kind）。
// 官方在 `dsh-client-ui-chat/src/client/conversation-nodes/*.ts` 里做这件事：
// 每个 Definition 声明 `match`（哪些事件归它）、`start/update`（状态机）、
// `buildViewNode`（产出 Chat 节点）。本文件是它的 Go 版骨架：
//
//	帧 ──match──> kind ──start/update──> 节点数据 ──> TranscriptEntry
//
// # 两条"不编"铁律
//
//	① **宿主没给的字段显示「未知」，绝不编 0。**
//	   `time` 缺失 → `TimeKnown=false`；`turn` 缺失 → `Turn=-1`；
//	   退出码缺失 → `ExitCode=nil`；usage 全零 → `Usage.Known()==false`。
//	   界面层据这些判据整段不画，而不是画一个"0 毫秒 / 0 token"的假数字。
//	② **不认识的帧落 `unknown`，原样 JSON 兜底。**
//	   官方 `fallback.ts` 的 match 是"任何 append surface 事件"，渲染器是
//	   `JsonBlock` 原样显示。**不许**给它编一个摘要（两级兜底都不得编造）。
//
// # journalEpoch：为什么不匹配就必须重建基线
//
// 宿主重启会换 journal，新 journal 的 seq 从 1 起。客户端拿着旧水位去补洞，
// 宿主要么回 409 `stale_epoch`、要么**从错位处推**——后者不报错也不丢帧，
// 只是"少了几条"。这是最难发现的一种错，所以：
//
//	epoch 不一致 → 丢弃全部旧条目 → 用这条 history 的 frames 重建基线
//
// 对应宿主契约见 `DSH-GO/docs/transport-contract.md` §journalEpoch 与
// `docs/reconnect.md §4`。
package dsconversation

import (
	"encoding/json"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// TranscriptEntry 是投影出来的**一条消息节点**（= 官方一个 Chat 节点）。
type TranscriptEntry struct {
	// Key 是稳定渲染键（官方 `context.key`）。同一会话内唯一。
	Key string
	// Kind 是渲染器分派键（20 种之一）。
	Kind ChatNodeKind
	// Seq 是节点在 journal 里的位置（官方 `anchorSeq`）。
	Seq int
	// Time 是事件时间（Unix 毫秒）；0 = 未知（**不许补本地墙钟**）。
	Time int64
	// Turn 是所属 Turn 号；-1 = 未知（官方坐标缺失）。
	Turn int64
	// Visibility 是这一条的可见性（`submitted-plan` 恒 hidden）。
	Visibility ChatNodeVisibility
	// Data 是渲染器自己的载荷（各类只填自己的字段）。
	Data TranscriptData
	// Streaming 标记这一条是**流式增量**（`assistant/text` / `assistant/reasoning`）：
	// 它要**追加**到同 key 的已有节点上，不是替换。
	//
	// 为什么不能一帧一条：真实宿主一步推 step/start → assistant/text×N →
	// assistant/message → step/end 五类帧。一帧一节点的话，界面上一步回答会
	// 变成 4~5 行（前几行空的、正文重复两遍）。官方 assistant.ts 的
	// start/update 状态机就是按 (turn, step) 一条节点。
	Streaming bool
}

// TimeKnown 报告事件时间是不是真值（0 = 宿主没给）。
func (e TranscriptEntry) TimeKnown() bool { return e.Time > 0 }

// TurnKnown 报告 Turn 坐标是不是真值（-1 = 宿主没给）。
func (e TranscriptEntry) TurnKnown() bool { return e.Turn >= 0 }

// TranscriptOption 是审批卡的一个可选项（`data.options[]`）。
type TranscriptOption struct {
	OptionID string
	Name     string
	Kind     string
}

// TranscriptAttachment 是用户消息里的一个附件（官方的 image / file 块）。
type TranscriptAttachment struct {
	// Name 是文件名（file 块）或图片标签（image 块）。空 = 未知。
	Name string
	// Meta 是元信息（大小 / 类型）。空 = 未知（不编 0 字节）。
	Meta string
	// Image 报告这是不是图片块（图片走 64×64 瓦片，文件走 240×64 卡片）。
	Image bool
}

// TranscriptUsage 是一条 assistant 消息携带的 token 用量。
//
// **零值 = 未知**：`Usage.Known()` 为 false 时界面整段不画，
// 不许显示 "0 tokens"。
type TranscriptUsage struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
}

// Known 报告宿主真的给过用量。
func (u TranscriptUsage) Known() bool {
	return u.InputTokens > 0 || u.OutputTokens > 0 || u.TotalTokens > 0
}

// TranscriptData 是节点载荷的并集。
//
// 为什么用一个 struct 而不是每个 kind 一个类型：Go 侧不需要 sum type，
// 而按 kind 分派读取的字段是**互斥**的（渲染器只看自己那几个）。
// 代价是"哪个 kind 该有哪个字段"只写在注释和渲染器里——所以每个字段都注明了来源。
type TranscriptData struct {
	// MessageID ← `data.id`（user/steering 的消息身份）。空 = 未知。
	MessageID string
	// Text ← content 里的 text 块（正文 / 上下文 / 系统提示 / 问题答复）。
	Text string
	// Reasoning ← content 里的 reasoning 块（assistant-step 的思考）。
	Reasoning string
	// SourceKind ← `data.source.kind`（user / notification / retry /
	// user-question-reply / compact-checkpoint …）。空 = 未知。
	SourceKind string

	// Tool ← `data.name`（工具名）。空 = 未知。
	Tool string
	// Args ← `data.arguments` 的原文（**不改写、不美化**）。
	Args string
	// Status 是工具四态：preparing / start / ok / error / stopped。
	Status string
	// Summary ← 宿主给的一行摘要。
	Summary string
	// ExitCode 是进程退出码；**nil = 未知**（不是 0）。
	ExitCode *int
	// ResultText 是 tool/result 的输出正文。
	ResultText string

	// CommandName ← `command/run` 的 `data.name`（斜杠命令名）。
	CommandName string
	// CommandOutcome ← `command/done` 的 `data.outcome`。
	CommandOutcome string

	// ErrorMessage ← `turn/end` 的 `data.reason.error.message`。
	ErrorMessage string
	// ErrorCode ← `turn/end` 的 `data.reason.error.code`。空 = 没有码。
	ErrorCode string
	// MaxTokensLimit ← `turn/end` 的 `data.reason.limit`；nil = 未知。
	MaxTokensLimit *int64

	// RetryAttempt ← `llm/retry` 的 `data.attempt`；nil = 未知。
	RetryAttempt *int
	// RetryMax ← `llm/retry` 的 `data.maxRetries`；nil = 未知（官方 normal 模式是 ∞）。
	RetryMax *int
	// RetryDelayMs ← `llm/retry` 的 `data.delayMs`；nil = 未知。
	RetryDelayMs *int64
	// RetryMode ← `llm/retry` 的 `data.mode`；空 = 未知。
	RetryMode string

	// CompactionSummary ← `compaction/summary` 的摘要文本。
	CompactionSummary string
	// CompactionCount ← 被压缩的消息条数；nil = 未知。
	CompactionCount *int

	// DurationMs ← `turn/end` 的耗时；nil = 未知（停止/失败不显示耗时）。
	DurationMs *int64
	// Usage 是 assistant 消息的用量；零值 = 未知。
	Usage TranscriptUsage

	// CallID ← 工具调用 id（question-reply 的配对键）。空 = 未知。
	CallID string
	// Outcome ← 问题答复的结局（question-reply）。
	Outcome string

	// RunID ← 工作流运行 id（workflow-run）。空 = 未知。
	RunID string
	// RunName ← 工作流名（workflow-run）。空 = 未知。
	RunName string
	// StopReason ← `tool-workflow/run-end` 的 `data.stopReason`。空 = 未知。
	StopReason string
	// Workflow 是 `workflow-run` 节点的**完整**载荷（官方 `WorkflowRunChatData`）。
	//
	// 为什么另开一个指针字段而不是把 name/status/phases 摊平进本结构：
	// 官方这个节点是**三层嵌套**（run → phase → member），摊平会逼着渲染器
	// 按 kind 去猜"哪些字段属于我"，而 RunID/RunName/StopReason 三个扁平字段
	// 已经是既有对外形状（`TestProjectWorkflowRun` 钉着），不能动。
	//
	// nil = 这一帧不是 workflow-run，或 run 还没 run-start（官方 `buildViewNode`
	// 在 `context.start === undefined` 时返回 null）。
	Workflow *WorkflowRunChatData

	// Options 是审批可选项（**原样**，不自己造 optionId）。
	Options []TranscriptOption
	// Attachments 是用户消息的附件（image / file 块）。
	Attachments []TranscriptAttachment
	// References 是这条消息引用的标签（引用摘要行）。空 = 没有引用。
	References []string
	// Raw 是未知节点的原始 JSON（`unknown` 兜底原样显示）。
	Raw string
}

// Transcript 是一条会话的消息节点投影。
//
// 零值不可用：用 `NewTranscript`。所有方法都**不是**并发安全的——
// 壳在单线程里跑（viva 的绘制线程），与既有 model 的约定一致。
type Transcript struct {
	epoch   int
	lastSeq int
	entries []TranscriptEntry
	// seqOf 记录每个 key 的序号，用于同 key 更新（官方 start/update）。
	seqOf map[string]int
	// rebuilt 记录上一次操作是否重建过基线（诊断/测试用）。
	rebuilt bool
	// workflows 是 `tool-workflow/*` 四类帧的**折叠状态**（官方 `WorkflowState`）。
	//
	// 为什么要状态而不是一帧一节点：官方 `workflow-definition.ts` 的
	// `start/update` 把 run-start → agent-start×N → agent-end×N → run-end
	// 折成**一条**节点。一帧一节点的话，界面上一次工作流会变成十几行。
	workflows map[string]*workflowRunFold
	// runsByTurn 是 `startTurn → []runId` 反查表：Turn 终结成
	// interrupted / forked 时，要把这一轮里开出去的 run 全部重投影成"中断"
	// （官方 `turnAbandoned` 的判据依赖 `location.turn.end.reason.kind`）。
	runsByTurn map[int64][]string
	// abandonedTurns 记录已终结为 interrupted / forked 的 Turn。
	abandonedTurns map[int64]bool
	// currentTurn 是**当前所在 Turn**（由 turn/start 设置、turn/end 清掉）。
	//
	// 为什么需要它：官方 `workflow-definition.ts` 的 `start` 是 `role: 'start'`，
	// 它的 `location.turn` 由**事件在流里的位置**决定，而不是 payload 字段——
	// `ToolWorkflowRunStartData` 只有 `{runId, name}` 两个字段，一个 turn 都
	// 没有。我们的投影是逐帧增量，所以必须自己跟住 turn/start 的位置。
	currentTurn int64
}

// workflowRunFold 是**一次** run 的折叠状态（官方 `WorkflowState`）。
type workflowRunFold struct {
	// name 是 `run-start` 的 `data.name`。
	name string
	// startTurn 是 `run-start` 所在 Turn（`-1` = 宿主没给）。
	startTurn int64
	// stopReason 是 `run-end` 的 `data.stopReason`；空 = 还没 settle。
	stopReason string
	// members 是已 `agent-start` 的成员（顺序 = 到达顺序）。
	members []workflowMemberFold
}

// workflowMemberFold 是单个成员的折叠状态（官方 `WorkflowMemberState`）。
type workflowMemberFold struct {
	seq int
	// label 是 `agent-start` 的 `data.label`。空串是**合法身份**。
	label string
	// hasPhase 区分 `phase` 字段缺省与空串（官方 `string | null`）。
	hasPhase bool
	phase    string
	childID  string
	// outcome 是 `agent-end` 的 `data.outcome`；空 = 还没 settle。
	outcome string
}

// NewTranscript 造一个空投影。
func NewTranscript() *Transcript {
	t := &Transcript{seqOf: map[string]int{}}
	t.resetWorkflows()
	return t
}

// Epoch 是当前 journal 世代（0 = 还没有基线）。
func (t *Transcript) Epoch() int { return t.epoch }

// LastSeq 是已消费的最高 journal seq（0 = 还没有）。
func (t *Transcript) LastSeq() int { return t.lastSeq }

// Len 是当前节点数。
func (t *Transcript) Len() int { return len(t.entries) }

// Rebuilt 报告上一次 Apply 是否因为世代不一致而重建了基线。
func (t *Transcript) Rebuilt() bool { return t.rebuilt }

// Entries 返回节点只读副本（顺序 = journal 顺序）。
func (t *Transcript) Entries() []TranscriptEntry {
	return append([]TranscriptEntry(nil), t.entries...)
}

// Reset 丢弃全部节点与水位（journal 换代 / 切会话时调用）。
func (t *Transcript) Reset() {
	t.entries = t.entries[:0]
	t.seqOf = map[string]int{}
	t.lastSeq = 0
	t.epoch = 0
	t.resetWorkflows()
}

// resetWorkflows 丢弃全部工作流折叠状态（与 Reset 同生命周期）。
func (t *Transcript) resetWorkflows() {
	t.workflows = map[string]*workflowRunFold{}
	t.runsByTurn = map[int64][]string{}
	t.abandonedTurns = map[int64]bool{}
	t.currentTurn = -1
}

// SetBaselineEpoch 记录 journal 世代（`session.load` 的 `journalEpoch`）。
//
// 与 `ApplyHistory` 的分工：load 给的是**基线**（可能没有 frames），
// history 给的是**补洞的一段帧**。只有后者需要判断"世代是否一致"。
func (t *Transcript) SetBaselineEpoch(epoch int) {
	if epoch > 0 {
		t.epoch = epoch
	}
}

// ApplyHistory 用 `session.history` 的结果补一段帧。
//
// 返回值 applied = 真正落成节点的帧数；rebuilt = 是否因为
// journalEpoch 不一致而**丢弃旧基线重建**。
//
// 判据（宿主契约）：`res.JournalEpoch` 与本地 `epoch` 都非 0 且不等
// → 旧水位属于另一个序号域，必须整段重建（不能按 seq 硬补）。
func (t *Transcript) ApplyHistory(res *apiclient.HistoryResult) (applied int, rebuilt bool) {
	t.rebuilt = false
	if res == nil {
		return 0, false
	}
	epochMismatch := t.epoch != 0 && res.JournalEpoch != 0 && res.JournalEpoch != t.epoch
	if epochMismatch {
		// 重建基线：**先丢旧条目**再灌新帧。
		// 只清水位不清条目 = 新旧两代混排，界面上表现为"消息偶尔跳一下"。
		t.entries = t.entries[:0]
		t.seqOf = map[string]int{}
		t.lastSeq = 0
		t.resetWorkflows()
		t.rebuilt = true
		rebuilt = true
	}
	if res.JournalEpoch > 0 {
		t.epoch = res.JournalEpoch
	}
	applied = t.applyFrames(res.Frames)
	// 宿主回的 lastSeq 是**这一代**的水位真值；比本地高才采用
	// （本地可能已经消费过更新的 WS 帧）。
	if res.LastSeq > t.lastSeq {
		t.lastSeq = res.LastSeq
	}
	return applied, rebuilt
}

// ApplyFrames 按顺序消费一批帧（WS 推送 / history 补洞共用）。
func (t *Transcript) ApplyFrames(frames []apiclient.Frame) int {
	t.rebuilt = false
	return t.applyFrames(frames)
}

func (t *Transcript) applyFrames(frames []apiclient.Frame) int {
	applied := 0
	for _, frame := range frames {
		if frame.Seq > 0 && frame.Seq <= t.lastSeq {
			// 已经消费过：journal 是 append-only，重复帧只能是重放。
			continue
		}
		if entry, ok := t.project(frame); ok {
			if entry.Streaming {
				t.appendStep(entry)
			} else {
				t.upsert(entry)
			}
			applied++
		}
		if frame.Seq > t.lastSeq {
			t.lastSeq = frame.Seq
		}
	}
	return applied
}

// upsert 按 key 落地节点：同 key 更新（官方 start/update），否则追加。
func (t *Transcript) upsert(entry TranscriptEntry) {
	if entry.Key == "" {
		entry.Key = "seq-" + itoa(entry.Seq)
	}
	if at, ok := t.seqOf[entry.Key]; ok && at < len(t.entries) {
		t.entries[at] = entry
		return
	}
	t.seqOf[entry.Key] = len(t.entries)
	t.entries = append(t.entries, entry)
}

// appendStep 把**流式增量**并进同 key 的节点（行位置不变，只并内容）。
//
// 行必须留在 step/start 那一条的位置：先到的 start 定位置，后续增量只长内容。
// 找不到同 key 时退化成追加——宁可多一行，也不要吞掉正文。
func (t *Transcript) appendStep(entry TranscriptEntry) {
	if entry.Key == "" {
		t.upsert(entry)
		return
	}
	at, ok := t.seqOf[entry.Key]
	if !ok || at >= len(t.entries) {
		t.seqOf[entry.Key] = len(t.entries)
		t.entries = append(t.entries, entry)
		return
	}
	existing := t.entries[at]
	existing.Data.Text += entry.Data.Text
	existing.Data.Reasoning += entry.Data.Reasoning
	t.entries[at] = existing
}

// stepKey 是官方 assistant 节点的稳定键：`(turn, step)`。
//
// 用坐标而不是 seq：同一 (turn, step) 的五类帧（start / text / reasoning /
// message / end）必须落到**同一条**节点上，而它们的 seq 各不相同。
func stepKey(data transcriptFrame) string {
	turn := UnknownText
	if data.Turn != nil {
		turn = itoa64(*data.Turn)
	}
	step := UnknownText
	if data.Step != nil {
		step = itoa64(*data.Step)
	}
	return "assistant-step-turn" + turn + "-step" + step
}

// ── 帧 → 节点 ─────────────────────────────────────────────────────────

// transcriptFrame 是 v4 事件 data 的联合形状（只解界面真用到的字段）。
//
// 为什么是宽松联合：宿主推的是 `{content:[…]}` / `{message:{content:[…]}}` /
// `{delta:"…"}` 三种形状（见 `internal/apiclient/mux.go` 的 packet 注释），
// 只认一种就会"整条流一个字都进不来"。
type transcriptFrame struct {
	SurfaceOp  string             `json:"surfaceOp"`
	Content    []transcriptBlock  `json:"content"`
	Message    *transcriptMessage `json:"message"`
	Source     *transcriptSource  `json:"source"`
	Turn       *int64             `json:"turn"`
	Step       *int64             `json:"step"`
	ToolCallID string             `json:"toolCallId"`
	CallID     string             `json:"callId"`
	Name       string             `json:"name"`
	Arguments  string             `json:"arguments"`
	ID         string             `json:"id"`
	// Text 是 `assistant/text` / `assistant/reasoning` 的**流式增量**（顶层
	// `text` 键，不是 content 块——真宿主 `internal/agent/loop.go` 就是这么推的）。
	Text string `json:"text"`

	Reason  *transcriptReason  `json:"reason"`
	Command *transcriptCommand `json:"command"`
	Usage   *transcriptUsage   `json:"usage"`

	Attempt    *int   `json:"attempt"`
	MaxRetries *int   `json:"maxRetries"`
	DelayMs    *int64 `json:"delayMs"`
	Mode       string `json:"mode"`

	Summary string `json:"summary"`
	Count   *int   `json:"count"`

	DurationMs *int64 `json:"durationMs"`
	Limit      *int64 `json:"limit"`

	RunID      string `json:"runId"`
	StopReason string `json:"stopReason"`

	// 以下四个只出现在 `tool-workflow/*` 载荷里（官方 `dsh-tool-workflow/src/types.ts`）。
	//
	// `Seq` 用指针：`agent-start` 的 `seq` 是 **1 基序号**，0 不是合法值，
	// 所以 `nil`（字段缺省）必须能与 0 区分——否则一条缺字段的帧会被
	// 静默当成第 0 个成员，与已有成员撞键。
	Seq *int `json:"seq"`
	// Label 是成员显示标签（`agent-start`）。
	Label string `json:"label"`
	// Phase 是阶段名；**nil 与空串是两个身份**（官方 `string | null`）。
	Phase *string `json:"phase"`
	// ChildID 是子会话 id。
	ChildID string `json:"childId"`

	// References 是这条消息引用的标签（引用摘要行）。
	References []string `json:"references"`

	Outcome string `json:"outcome"`
}

type transcriptBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	ID        string `json:"id"`
	// 附件块（官方 image / file / 宿主 attachment）的字段。
	FileName  string `json:"filename"`
	MediaType string `json:"mediaType"`
	Size      *int64 `json:"size"`
	MIME      string `json:"mime"`
	ByteSize  *int64 `json:"byteSize"`
}

type transcriptMessage struct {
	Content    []transcriptBlock `json:"content"`
	Role       string            `json:"role"`
	ToolCallID string            `json:"toolCallId"`
}

type transcriptSource struct {
	Kind   string `json:"kind"`
	CallID string `json:"callId"`
}

type transcriptReason struct {
	Kind  string `json:"kind"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
	Limit *int64 `json:"limit"`
}

type transcriptCommand struct {
	Name    string `json:"name"`
	Args    string `json:"args"`
	Outcome string `json:"outcome"`
}

type transcriptUsage struct {
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
	TotalTokens  int64 `json:"totalTokens"`
}

// blocks 取 content 块：顶层优先，其次 message.content（两种落法都见过）。
func (f transcriptFrame) blocks() []transcriptBlock {
	if len(f.Content) > 0 {
		return f.Content
	}
	if f.Message != nil {
		return f.Message.Content
	}
	return nil
}

// textOf 把 content 里的 text 块拼成正文（**保持原顺序、不重排**）。
func (f transcriptFrame) textOf() string {
	var out string
	for _, b := range f.blocks() {
		if b.Type == "" || b.Type == "text" {
			out += b.Text
		}
	}
	return out
}

// reasoningOf 把 content 里的 reasoning 块拼成思考。
func (f transcriptFrame) reasoningOf() string {
	var out string
	for _, b := range f.blocks() {
		if b.Type == "reasoning" {
			out += b.Text
		}
	}
	return out
}

// attachmentsOf 取 content 里的附件块（image / file）。
//
// **0 字节文件仍保留大小**（官方 `fileSizeText` 的真值：0 不是"未知"）。
func (f transcriptFrame) attachmentsOf() []TranscriptAttachment {
	var out []TranscriptAttachment
	for _, b := range f.blocks() {
		switch b.Type {
		case "image":
			out = append(out, TranscriptAttachment{
				Name:  firstNonEmpty(b.Text, b.Name, b.FileName),
				Meta:  b.MediaType,
				Image: true,
			})
		case "file":
			meta := b.MediaType
			if b.Size != nil {
				if meta != "" {
					meta += " · "
				}
				meta += itoa64(*b.Size) + " B"
			}
			out = append(out, TranscriptAttachment{
				Name: firstNonEmpty(b.FileName, b.Name, b.Text),
				Meta: meta,
			})
		case "attachment":
			// 宿主 durable user/message 的附件块（HOST:cmd/dshd/attachments.go:568-578
			// 的 ResolveBlocks）：附件 id 之外带 name/byteSize/mime，类型仍是 "attachment"。
			// 官方两个前端块类型 image/file 是浏览器提交形状，落盘后统一成这一个。
			meta := b.MIME
			if b.ByteSize != nil {
				if meta != "" {
					meta += " · "
				}
				meta += itoa64(*b.ByteSize) + " B"
			}
			out = append(out, TranscriptAttachment{
				Name:  b.Name,
				Meta:  meta,
				Image: strings.HasPrefix(b.MIME, "image/"),
			})
		}
	}
	return out
}

// firstNonEmpty 取第一个非空串（**不猜**：全空返回空串）。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// turnOf 取 Turn 坐标；缺失返回 -1（**不是 0**——0 是合法 Turn 号）。
func (f transcriptFrame) turnOf() int64 {
	if f.Turn != nil {
		return *f.Turn
	}
	return -1
}

// project 把一帧投影成一条节点。ok=false = 这一帧不产生 transcript 行。
//
// 映射规则逐条对齐官方 Definition 的 `match`（锚点写在 case 上）。
func (t *Transcript) project(frame apiclient.Frame) (TranscriptEntry, bool) {
	kindName := frame.Type
	if skipTranscriptFrame(kindName) {
		return TranscriptEntry{}, false
	}
	var data transcriptFrame
	if len(frame.Data) > 0 {
		if err := json.Unmarshal(frame.Data, &data); err != nil {
			// 解不开 = 不认识的载荷：落 unknown 原样兜底，**不猜**。
			return t.unknownEntry(frame), true
		}
	}
	entry := TranscriptEntry{
		Seq:  frame.Seq,
		Time: frame.Time,
		Turn: data.turnOf(),
		Key:  "",
		Data: TranscriptData{},
	}
	switch kindName {
	case "user/message":
		// message.ts:86-160：按 surfaceOp + source.kind 分派。
		//   replace + compact-checkpoint → manual-compaction
		//   append  + source.kind=user-question-reply → question-reply（user-questions 包）
		//   append  + source.kind=user             → user（或活动 Turn 内的 steering）
		//   其余                                   → context
		if data.Source != nil && data.Source.Kind == "compact-checkpoint" {
			entry.Kind = ChatNodeManualCompaction
			entry.Data.Text = data.textOf()
			entry.Data.SourceKind = data.Source.Kind
			break
		}
		// 记录型唤醒 / 手动续写回执（官方 message.ts:140-158 的 waking 分支）：
		// 替换型 + source.kind=retry + 空 content → turn-trigger。
		if data.SurfaceOp != "" && data.SurfaceOp != "append" &&
			data.Source != nil && data.Source.Kind == "retry" && len(data.blocks()) == 0 {
			entry.Kind = ChatNodeTurnTrigger
			entry.Data.SourceKind = data.Source.Kind
			break
		}
		if data.Source != nil && data.Source.Kind == "user-question-reply" {
			entry.Kind = ChatNodeQuestionReply
			entry.Data.Text = data.textOf()
			entry.Data.SourceKind = data.Source.Kind
			entry.Data.CallID = data.Source.CallID
			break
		}
		if data.SurfaceOp == "append" && (data.Source == nil || data.Source.Kind == "user") {
			// 活动 Turn 内的用户消息是 steering（官方靠 inbox 的 claim 判；
			// 骨架里用 Turn 坐标是否存在做等价判据，**不猜** claim 身份）。
			if data.Turn != nil {
				entry.Kind = ChatNodeSteering
			} else {
				entry.Kind = ChatNodeUser
			}
			entry.Data.MessageID = data.ID
			entry.Data.Text = data.textOf()
			entry.Data.Attachments = data.attachmentsOf()
			entry.Data.References = data.References
			if data.Source != nil {
				entry.Data.SourceKind = data.Source.Kind
			}
			break
		}
		entry.Kind = ChatNodeContext
		entry.Data.Text = data.textOf()
		if data.Source != nil {
			entry.Data.SourceKind = data.Source.Kind
		}
	case "developer/message":
		// message.ts:165-172 → context
		entry.Kind = ChatNodeContext
		entry.Data.Text = data.textOf()
		entry.Data.SourceKind = "developer"
	case "system/message", "request/header":
		// request-prompt.ts:68-110 → system-prompt（in-history 更新与首次提示两个文案）
		entry.Kind = ChatNodeSystemPrompt
		entry.Data.Text = data.textOf()
	// 注意 `step/end` **不在**这里：它是"这一步收工"的边界帧，不带正文，
	// 落成节点只会把刚合并好的正文**覆盖成空**（真实宿主每步都推它）。
	case "step/start", "assistant/attempt", "assistant/live-chunk",
		"assistant/text", "assistant/reasoning", "assistant/message":
		// assistant.ts:307-334 → assistant-step（官方 start/update 状态机：
		// **一个 (turn, step) 一条节点**，不是一帧一条）。
		//
		// 真实宿主一步推的是：step/start → assistant/text×N（流式增量）→
		// assistant/message（一步完整产物 + usage）→ step/end。四类帧的 seq
		// 各不相同，所以**必须按 (turn, step) 合键**——按 seq 就是一步 4 行，
		// 前几行空、正文重复两遍。
		entry.Kind = ChatNodeAssistantStep
		entry.Key = stepKey(data)
		switch kindName {
		case "assistant/text", "assistant/live-chunk":
			// 流式正文增量（宿主 `internal/agent/loop.go` 推的是 delta）：**追加**。
			entry.Data.Text = data.Text
			entry.Streaming = true
		case "assistant/reasoning":
			// 流式思考增量：追加到 reasoning 区（不占正文）。
			entry.Data.Reasoning = data.Text
			entry.Streaming = true
		case "assistant/message":
			// 一步的完整产物（权威正文 + 用量）。
			entry.Data.Text = data.textOf()
			entry.Data.Reasoning = data.reasoningOf()
			if data.Usage != nil {
				entry.Data.Usage = TranscriptUsage{
					InputTokens:  data.Usage.InputTokens,
					OutputTokens: data.Usage.OutputTokens,
					TotalTokens:  data.Usage.TotalTokens,
				}
			}
			if data.ID != "" {
				entry.Data.MessageID = data.ID
			}
		default:
			// step/start / assistant/attempt：只定位置，不带正文。
			entry.Data.Text = data.textOf()
			entry.Data.Reasoning = data.reasoningOf()
		}
	case "tool/call", "tool/ptc-dispatch-start", "tool/ptc-dispatch", "tool/result":
		// tool.ts:295-330 → tool-call（三态 preparing|start|result；四态渲染 ok/error/stopped/preparing）
		entry.Kind = ChatNodeToolCall
		entry.Data.Tool = data.Name
		entry.Data.Args = data.Arguments
		entry.Data.CallID = data.CallID
		if entry.Data.CallID == "" {
			entry.Data.CallID = data.ToolCallID
		}
		switch kindName {
		case "tool/result":
			entry.Data.Status = "ok"
			entry.Data.ResultText = data.textOf()
		case "tool/call":
			entry.Data.Status = "preparing"
			if data.Arguments != "" {
				entry.Data.Status = "start"
			}
		default:
			entry.Data.Status = "start"
		}
		if entry.Data.ResultText == "" {
			entry.Data.ResultText = data.textOf()
		}
	case "command/run", "command/done":
		// command.ts:177-199 → command；`/goal` 的输入由 goal 包接管（command-input）
		if data.Command != nil && data.Command.Name == "goal" {
			entry.Kind = ChatNodeCommandInput
			entry.Data.CommandName = data.Command.Name
			entry.Data.Text = commandInputText(data)
			break
		}
		entry.Kind = ChatNodeCommand
		if data.Command != nil {
			entry.Data.CommandName = data.Command.Name
			entry.Data.CommandOutcome = data.Command.Outcome
			if kindName == "command/done" {
				entry.Data.CommandOutcome = data.Command.Outcome
			}
		}
	case "compaction/start", "compaction/summary", "compaction/end":
		// compaction.ts:34-50 → compaction（检查点）
		entry.Kind = ChatNodeCompaction
		entry.Data.CompactionSummary = data.Summary
		entry.Data.CompactionCount = data.Count
		if entry.Data.CompactionSummary == "" {
			entry.Data.CompactionSummary = data.textOf()
		}
	case "llm/retry", "llm/retry-started":
		// retry.ts:44-60 → model-retry（一行）
		entry.Kind = ChatNodeModelRetry
		entry.Data.RetryAttempt = data.Attempt
		entry.Data.RetryMax = data.MaxRetries
		entry.Data.RetryDelayMs = data.DelayMs
		entry.Data.RetryMode = data.Mode
	case "turn/start":
		// turn-process.ts:252-256 → turn-process（Turn 级过程披露）
		entry.Kind = ChatNodeTurnProcess
		// 跟住位置：workflow-run 的 startTurn 取这里（官方 location 的等价物）。
		t.currentTurn = data.turnOf()
	case "turn/end":
		// 终态四选一（官方 turn-error.ts / turn-max-tokens.ts / turn-tail.ts）：
		//   reason.kind=error       → turn-error
		//   reason.kind=max-tokens  → turn-max-tokens
		//   其余（completed / aborted / interrupted …）→ turn-tail
		entry.Data.DurationMs = data.DurationMs
		switch {
		case data.Reason != nil && data.Reason.Kind == "error":
			entry.Kind = ChatNodeTurnError
			if data.Reason.Error != nil {
				entry.Data.ErrorMessage = data.Reason.Error.Message
				entry.Data.ErrorCode = data.Reason.Error.Code
			}
		case data.Reason != nil && data.Reason.Kind == "max-tokens":
			entry.Kind = ChatNodeTurnMaxTokens
			entry.Data.MaxTokensLimit = data.Limit
			if entry.Data.MaxTokensLimit == nil && data.Reason.Limit != nil {
				entry.Data.MaxTokensLimit = data.Reason.Limit
			}
		case data.Reason != nil && data.Reason.Kind == "interrupted":
			entry.Kind = ChatNodeTurnError
			entry.Data.ErrorMessage = "The DSH process exited before this turn finished."
			entry.Data.ErrorCode = "INTERRUPTED"
		default:
			entry.Kind = ChatNodeTurnTail
			if data.Usage != nil {
				entry.Data.Usage = TranscriptUsage{
					InputTokens:  data.Usage.InputTokens,
					OutputTokens: data.Usage.OutputTokens,
					TotalTokens:  data.Usage.TotalTokens,
				}
			}
		}
	case "tool-workflow/run-start", "tool-workflow/agent-start",
		"tool-workflow/agent-end", "tool-workflow/run-end":
		// workflow-definition.ts:157-182 → workflow-run
		//
		// 四类帧折成**一条**节点（键 = `workflow-run-<runId>`）：run-start 定位置，
		// 后续 agent-start / agent-end / run-end 只更新内容。见 `projectWorkflowFrame`。
		return t.projectWorkflowFrame(frame, data), true
	default:
		// fallback.ts:19-27：任何 **append surface 事件**都可以落 unknown 兜底。
		// **不编摘要**：原样 JSON 交给 JsonBlock。
		//
		// 非 surface 帧（`request/context`、`usage/update`、`step/end`、
		// `session/title*`、`turn/done`/`turn/error` …）是**元数据**，不是消息行：
		// 原样 JSON 画出来只会让主区每轮多几行看不懂的东西。判据是官方的
		// `surfaceOp`（真宿主每个 surface 事件都带），不靠白名单猜。
		if data.SurfaceOp == "" && len(data.blocks()) == 0 {
			return TranscriptEntry{}, false
		}
		return t.unknownEntry(frame), true
	}
	// Turn 终结成 interrupted / forked 时，这一轮里开出去的 run 若**没有**
	// run-end，就再也没有帧会来更新它了（`turnAbandoned` 的官方判据）。
	// 所以在这里主动重投影一次——否则界面上那条 run 会永远停在"运行中"。
	//
	// 为什么必须在 turn/end 这一帧做：官方是 React 响应式重算，每次投影都
	// 重新跑 `buildViewNode`；我们的投影是增量的，不主动刷就漏。
	if kindName == "turn/end" {
		if data.Reason != nil &&
			(data.Reason.Kind == "interrupted" || data.Reason.Kind == "forked") {
			t.markTurnAbandoned(entry.Turn)
		}
		t.currentTurn = -1
	}

	entry.Visibility = visibilityOf(entry.Kind)
	if entry.Key == "" {
		entry.Key = string(entry.Kind) + "-" + itoa(entry.Seq)
	}
	return entry, true
}

// ── 工作流 run 的折叠（官方 `workflow-definition.ts` 的 start/update）────

// workflowRunEntryKey 是 run 的稳定节点键（官方 `context.id = String(runId)`）。
func workflowRunEntryKey(runID string) string { return "workflow-run-" + runID }

// projectWorkflowFrame 把一条 `tool-workflow/*` 帧并进对应 run 的折叠状态，
// 并产出**折叠后**的节点（官方 `buildViewNode`）。
//
// 四条分支（对齐官方 `update`，:172-183）：
//
//	run-start   → 新建 run（name；members 清空）
//	agent-start → 追加成员（seq / label / phase? / childId）
//	agent-end   → 按 seq 给成员写 outcome
//	run-end     → 写 stopReason
//
// 未知 runId 的 update（agent-* / run-end 先于 run-start 到达）**静默丢弃**
// 吗？不——官方的 `match` 只在 run-start 时 `role: 'start'`，但 update 分支
// 照样会拿到一个不存在的 state 而崩。这里取**保守**做法：没有 run-start 就
// 建一个占位 run（name 空），**不丢帧**——丢一条 agent-end 会让一个真的
// 失败成员显示成"运行中"。
func (t *Transcript) projectWorkflowFrame(frame apiclient.Frame, data transcriptFrame) TranscriptEntry {
	key := workflowRunEntryKey(data.RunID)
	fold, ok := t.workflows[key]
	if !ok {
		// startTurn 取**当前所在 Turn**（turn/start 设置），不是 payload 字段：
		// 官方 payload 根本没有 turn（见 currentTurn 的注释）。带 turn 的
		// 载荷（宿主若扩了字段）优先用自己的。
		startTurn := t.currentTurn
		if data.Turn != nil {
			startTurn = *data.Turn
		}
		fold = &workflowRunFold{startTurn: startTurn}
		t.workflows[key] = fold
		if fold.startTurn >= 0 {
			t.runsByTurn[fold.startTurn] = append(t.runsByTurn[fold.startTurn], data.RunID)
		}
	}
	switch frame.Type {
	case "tool-workflow/run-start":
		fold.name = data.Name
		fold.members = nil
		fold.stopReason = ""
	case "tool-workflow/agent-start":
		seq := 0
		if data.Seq != nil {
			seq = *data.Seq
		}
		member := workflowMemberFold{
			seq:     seq,
			label:   data.Label,
			childID: data.ChildID,
		}
		if data.Phase != nil {
			member.hasPhase = true
			member.phase = *data.Phase
		}
		fold.members = append(fold.members, member)
	case "tool-workflow/agent-end":
		seq := 0
		if data.Seq != nil {
			seq = *data.Seq
		}
		for i := range fold.members {
			if fold.members[i].seq == seq {
				fold.members[i].outcome = data.Outcome
			}
		}
	case "tool-workflow/run-end":
		fold.stopReason = data.StopReason
	}
	entry := TranscriptEntry{
		Key:  key,
		Kind: ChatNodeWorkflowRun,
		Seq:  frame.Seq,
		Time: frame.Time,
		Turn: fold.startTurn,
		Data: TranscriptData{
			RunID:      data.RunID,
			RunName:    fold.name,
			StopReason: fold.stopReason,
			Workflow:   t.buildWorkflowData(fold),
		},
	}
	entry.Visibility = visibilityOf(entry.Kind)
	return entry
}

// buildWorkflowData 把折叠状态投影成渲染载荷（官方 `projectWorkflow`，:95-132）。
//
// 两组判据（都是官方的显式分支，不是巧合）：
//
//	run 级   stopReason 缺省 && turnAbandoned → interrupted
//	         stopReason 缺省               → running
//	         其余                          → statusFromStopReason
//	member   outcome 缺省 && interrupted → interrupted
//	         outcome 缺省               → running
//	         其余                       → statusFromOutcome
func (t *Transcript) buildWorkflowData(fold *workflowRunFold) *WorkflowRunChatData {
	interrupted := fold.stopReason == "" && t.abandonedTurns[fold.startTurn]
	order := make([]string, 0, 4)
	groups := map[string]*WorkflowRunPhaseData{}
	for _, member := range fold.members {
		var phase *string
		if member.hasPhase {
			value := member.phase
			phase = &value
		}
		phaseKey := WorkflowPhaseKey(phase)
		group, ok := groups[phaseKey]
		if !ok {
			group = &WorkflowRunPhaseData{Key: phaseKey, Phase: phase}
			groups[phaseKey] = group
			order = append(order, phaseKey)
		}
		status := WorkflowStatusRunning
		switch {
		case member.outcome != "":
			status = workflowStatusFromOutcome(member.outcome)
		case interrupted:
			status = WorkflowStatusInterrupted
		}
		group.Members = append(group.Members, WorkflowRunMemberData{
			Seq:     member.seq,
			Label:   member.label,
			ChildID: member.childID,
			Status:  status,
		})
	}
	runStatus := WorkflowStatusRunning
	switch {
	case fold.stopReason != "":
		runStatus = workflowStatusFromStopReason(fold.stopReason)
	case interrupted:
		runStatus = WorkflowStatusInterrupted
	}
	phases := make([]WorkflowRunPhaseData, 0, len(order))
	for _, phaseKey := range order {
		phases = append(phases, *groups[phaseKey])
	}
	return &WorkflowRunChatData{Name: fold.name, Status: runStatus, Phases: phases}
}

// markTurnAbandoned 记一个 Turn 已终结为 abandoned，并重投影它开出去的 run。
//
// 只重投影**还没有 run-end** 的 run：已经有 stopReason 的 run 用 run-end 的
// 事实，Turn 的中断不能覆盖它（否则一个正常完成的 run 会被改成 interrupted）。
func (t *Transcript) markTurnAbandoned(turn int64) {
	if turn < 0 {
		return
	}
	if t.abandonedTurns == nil {
		t.abandonedTurns = map[int64]bool{}
	}
	t.abandonedTurns[turn] = true
	for _, runID := range t.runsByTurn[turn] {
		key := workflowRunEntryKey(runID)
		fold, ok := t.workflows[key]
		if !ok || fold.stopReason != "" {
			continue
		}
		at, ok := t.seqOf[key]
		if !ok || at >= len(t.entries) {
			continue
		}
		t.entries[at].Data.Workflow = t.buildWorkflowData(fold)
	}
}

// workflowStatusFromStopReason 是 run 级状态映射（官方 `statusFromStopReason`）。
func workflowStatusFromStopReason(stopReason string) WorkflowRunStatus {
	switch stopReason {
	case "completed":
		return WorkflowStatusCompleted
	case "cancelled":
		return WorkflowStatusCancelled
	case "error":
		return WorkflowStatusFailed
	}
	// 官方闭合联合里只有三个值；宿主给了第四个值就是**契约漂移**，
	// 这里按 failed 兜底（把"不知道"报成成功是最坏的形状）。
	return WorkflowStatusFailed
}

// workflowStatusFromOutcome 是成员级状态映射（官方 `statusFromOutcome`）。
func workflowStatusFromOutcome(outcome string) WorkflowRunStatus {
	switch outcome {
	case "completed":
		return WorkflowStatusCompleted
	case "cancelled":
		return WorkflowStatusCancelled
	case "failed":
		return WorkflowStatusFailed
	}
	return WorkflowStatusFailed
}

// skipTranscriptFrame 报告一条 journal 帧**不产生消息行**。
//
// 为什么要有这个判据：Transport journal 里除了 v4 内容事件，还有控制帧
// （`pong` / `session/reset`）与**审批帧**（`approval/request|timeout|cancel`）。
// 审批不是消息行——官方由 composer 接管（见 conversation.go 的 Panels 注释：
// 同屏两条「需要审批」是明确要避免的）。不跳过它们，主区就会在每轮里多出
// 几行原始 JSON，而用户完全不知道那是什么。
func skipTranscriptFrame(kindName string) bool {
	switch kindName {
	case "", "pong", "session/reset",
		"approval/request", "approval/timeout", "approval/cancel":
		return true
	}
	return false
}

// 注：**非 surface 的未知帧**（元数据类）由 project 的 default 分支丢弃，
// 不在这里列清单——清单会随宿主新增事件类型腐化，`surfaceOp` 判据不会。

// commandInputText 拼 `/goal` 命令行的可见文本（goal-command-input.ts:36-38：
// `/${name}${args.trimEnd()}`）。
func commandInputText(data transcriptFrame) string {
	if data.Command == nil {
		return ""
	}
	args := data.Command.Args
	for len(args) > 0 && (args[len(args)-1] == ' ' || args[len(args)-1] == '\t' || args[len(args)-1] == '\n' || args[len(args)-1] == '\r') {
		args = args[:len(args)-1]
	}
	return "/" + data.Command.Name + args
}

// unknownEntry 造一条 unknown 兜底节点（原样 JSON，不编摘要）。
func (t *Transcript) unknownEntry(frame apiclient.Frame) TranscriptEntry {
	raw := string(frame.Data)
	if raw == "" {
		raw = "{}"
	}
	return TranscriptEntry{
		Key:        "unknown-" + itoa(frame.Seq),
		Kind:       ChatNodeUnknown,
		Seq:        frame.Seq,
		Time:       frame.Time,
		Turn:       -1,
		Visibility: ChatVisibilityVisible,
		Data: TranscriptData{
			Text:       "",
			SourceKind: "",
			Raw:        raw,
		},
	}
}

// visibilityOf 取 kind 的官方可见性。
//
// `submitted-plan` 恒 hidden（官方 plan 包没有行渲染器）；其余按表。
func visibilityOf(kind ChatNodeKind) ChatNodeVisibility {
	spec, ok := ChatNodeSpecFor(kind)
	if !ok {
		// 表外的 kind 只能是未知：按 visible 兜底（unknown 行本身是可见的）。
		return ChatVisibilityVisible
	}
	return spec.Visibility
}

// ── 「未知」文案（宿主没给就显示未知，绝不编 0）──────────────────────────

// UnknownText 是宿主没给字段时的统一文案。
const UnknownText = "未知"

// TimeText 把事件时间渲染成可显示文本；0 = 未知。
func (e TranscriptEntry) TimeText() string {
	if !e.TimeKnown() {
		return UnknownText
	}
	return formatMillis(e.Time)
}

// TurnText 把 Turn 号渲染成可显示文本；-1 = 未知。
func (e TranscriptEntry) TurnText() string {
	if !e.TurnKnown() {
		return UnknownText
	}
	return "Turn " + itoa64(e.Turn)
}

// ExitCodeText 把退出码渲染成可显示文本；nil = 未知（**不是 0**）。
func (d TranscriptData) ExitCodeText() string {
	if d.ExitCode == nil {
		return UnknownText
	}
	return "exit " + itoa(*d.ExitCode)
}

// UsageText 把用量渲染成可显示文本；未知时返回空串（调用方整段不画）。
func (u TranscriptUsage) UsageText() string {
	if !u.Known() {
		return ""
	}
	return itoa64(u.TotalTokens) + " tokens"
}

// RetryText 把重试信息渲染成一行文本；未知字段显示未知。
func (d TranscriptData) RetryText() string {
	attempt := UnknownText
	if d.RetryAttempt != nil {
		attempt = itoa(*d.RetryAttempt)
	}
	max := UnknownText
	if d.RetryMax != nil {
		max = itoa(*d.RetryMax)
	} else if d.RetryMode != "" && d.RetryMode != "normal" {
		// 官方：mode !== 'normal' 时最大次数显示 ∞。
		max = "∞"
	}
	return "重试 " + attempt + "/" + max
}

// itoa64 是包内小工具（stdlib 的 strconv 会多一层 import；这里只要十进制）。
func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// formatMillis 把 Unix 毫秒渲染成 `HH:MM:SS`（本地时区无关：只取 UTC 时分秒）。
//
// 为什么不用 time.Time：骨架只需要"稳定可断言"的文本；带时区会让同一条
// 用例在 CI 与本机产出不同字符串，而时间真值本身已经在 entry.Time 里。
func formatMillis(ms int64) string {
	sec := ms / 1000
	h := (sec / 3600) % 24
	m := (sec / 60) % 60
	s := sec % 60
	return twoDigits(h) + ":" + twoDigits(m) + ":" + twoDigits(s)
}

func twoDigits(v int64) string {
	if v < 0 {
		v = -v
	}
	if v < 10 {
		return "0" + itoa64(v)
	}
	return itoa64(v)
}
