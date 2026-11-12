package dsconversation

// official_transcript_projection_test.go —— 帧 → 节点投影的机器断言。
//
// 三组断言：
//
//	① 每种 v4 事件都投影出**官方对应的 kind**（逐条对齐 Definition 的 match）；
//	② journalEpoch 不一致必须**重建基线**（丢弃旧条目），一致则追加；
//	③ 宿主没给的字段显示「未知」，绝不编 0（时间 / Turn / 退出码 / 用量）。

import (
	"encoding/json"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// frame 造一条 journal 原始帧（data 走 JSON，与线上同形状）。
func frame(seq int, kind string, data string) apiclient.Frame {
	f := apiclient.Frame{Seq: seq, Type: kind}
	if data != "" {
		f.Data = json.RawMessage(data)
	}
	return f
}

func mustProject(t *testing.T, f apiclient.Frame) TranscriptEntry {
	t.Helper()
	tr := NewTranscript()
	tr.ApplyFrames([]apiclient.Frame{f})
	entries := tr.Entries()
	if len(entries) != 1 {
		t.Fatalf("帧 %s 应投影出 1 条节点，实际 %d 条", f.Type, len(entries))
	}
	return entries[0]
}

func TestProjectUserMessage(t *testing.T) {
	entry := mustProject(t, frame(1, "user/message",
		`{"surfaceOp":"append","id":"m1","content":[{"type":"text","text":"你好"}]}`))
	if entry.Kind != ChatNodeUser {
		t.Fatalf("普通用户消息 kind = %q，应为 user", entry.Kind)
	}
	if entry.Data.Text != "你好" {
		t.Errorf("正文 = %q，应为 你好", entry.Data.Text)
	}
	if entry.Data.MessageID != "m1" {
		t.Errorf("消息 id = %q，应为 m1", entry.Data.MessageID)
	}
	if entry.Visibility != ChatVisibilityVisible {
		t.Errorf("可见性 = %q，应为 visible", entry.Visibility)
	}
}

// TestProjectSteering：活动 Turn 内的用户消息（带 turn 坐标）是 steering。
func TestProjectSteering(t *testing.T) {
	entry := mustProject(t, frame(2, "user/message",
		`{"surfaceOp":"append","turn":3,"id":"m2","source":{"kind":"user"},"content":[{"type":"text","text":"补充一句"}]}`))
	if entry.Kind != ChatNodeSteering {
		t.Fatalf("带 turn 的用户消息 kind = %q，应为 steering", entry.Kind)
	}
	if !entry.TurnKnown() || entry.Turn != 3 {
		t.Errorf("Turn = %d（known=%v），应为 3", entry.Turn, entry.TurnKnown())
	}
}

// TestProjectContext：非 user 来源的注入是 context。
func TestProjectContext(t *testing.T) {
	entry := mustProject(t, frame(3, "user/message",
		`{"surfaceOp":"append","source":{"kind":"notification"},"content":[{"type":"text","text":"定时任务已触发"}]}`))
	if entry.Kind != ChatNodeContext {
		t.Fatalf("notification 注入 kind = %q，应为 context", entry.Kind)
	}
	if entry.Data.SourceKind != "notification" {
		t.Errorf("source.kind = %q，应为 notification", entry.Data.SourceKind)
	}
}

// TestProjectDeveloperMessage：developer/message 也走 context 呈现。
func TestProjectDeveloperMessage(t *testing.T) {
	entry := mustProject(t, frame(4, "developer/message",
		`{"message":{"content":[{"type":"text","text":"系统约定"}]}}`))
	if entry.Kind != ChatNodeContext {
		t.Fatalf("developer/message kind = %q，应为 context", entry.Kind)
	}
	if entry.Data.Text != "系统约定" {
		t.Errorf("正文 = %q", entry.Data.Text)
	}
}

func TestProjectAssistantStep(t *testing.T) {
	entry := mustProject(t, frame(5, "assistant/message",
		`{"content":[{"type":"reasoning","text":"想一下"},{"type":"text","text":"答案"}],"usage":{"inputTokens":10,"outputTokens":20,"totalTokens":30}}`))
	if entry.Kind != ChatNodeAssistantStep {
		t.Fatalf("assistant/message kind = %q，应为 assistant-step", entry.Kind)
	}
	if entry.Data.Text != "答案" {
		t.Errorf("正文 = %q", entry.Data.Text)
	}
	if entry.Data.Reasoning != "想一下" {
		t.Errorf("思考 = %q", entry.Data.Reasoning)
	}
	if !entry.Data.Usage.Known() || entry.Data.Usage.TotalTokens != 30 {
		t.Errorf("用量 = %+v，应已知 30", entry.Data.Usage)
	}
}

func TestProjectToolCallStates(t *testing.T) {
	cases := []struct {
		name   string
		frame  apiclient.Frame
		status string
	}{
		{"call-no-args", frame(10, "tool/call", `{"name":"bash","id":"c1"}`), "preparing"},
		{"call-with-args", frame(11, "tool/call", `{"name":"bash","arguments":"{\"command\":\"ls\"}"}`), "start"},
		{"result", frame(12, "tool/result", `{"message":{"role":"tool","content":[{"type":"text","text":"ok"}]}}`), "ok"},
	}
	for _, tc := range cases {
		entry := mustProject(t, tc.frame)
		if entry.Kind != ChatNodeToolCall {
			t.Errorf("%s: kind = %q，应为 tool-call", tc.name, entry.Kind)
			continue
		}
		if entry.Data.Status != tc.status {
			t.Errorf("%s: status = %q，应为 %q", tc.name, entry.Data.Status, tc.status)
		}
	}
}

// TestToolCallExitCodeUnknownIsNil 钉住「宿主没给退出码 = nil，不是 0」。
func TestToolCallExitCodeUnknownIsNil(t *testing.T) {
	entry := mustProject(t, frame(13, "tool/result", `{"message":{"content":[{"type":"text","text":"out"}]}}`))
	if entry.Data.ExitCode != nil {
		t.Fatalf("宿主没给退出码，ExitCode 必须是 nil，实际 %d", *entry.Data.ExitCode)
	}
	if got := entry.Data.ExitCodeText(); got != UnknownText {
		t.Errorf("退出码文案 = %q，应为 %q", got, UnknownText)
	}
}

func TestProjectCommandAndGoalInput(t *testing.T) {
	cmd := mustProject(t, frame(20, "command/run", `{"command":{"name":"compact","args":""}}`))
	if cmd.Kind != ChatNodeCommand {
		t.Fatalf("/compact kind = %q，应为 command", cmd.Kind)
	}
	if cmd.Data.CommandName != "compact" {
		t.Errorf("命令名 = %q", cmd.Data.CommandName)
	}

	goal := mustProject(t, frame(21, "command/run", `{"command":{"name":"goal","args":" 写完报告  "}}`))
	if goal.Kind != ChatNodeCommandInput {
		t.Fatalf("/goal kind = %q，应为 command-input", goal.Kind)
	}
	if goal.Data.Text != "/goal 写完报告" {
		t.Errorf("/goal 文本 = %q（应保留命令名并把尾部空白去掉）", goal.Data.Text)
	}
}

func TestProjectCompaction(t *testing.T) {
	entry := mustProject(t, frame(30, "compaction/summary", `{"summary":"早先的对话已压缩","count":12}`))
	if entry.Kind != ChatNodeCompaction {
		t.Fatalf("compaction/summary kind = %q，应为 compaction", entry.Kind)
	}
	if entry.Data.CompactionSummary != "早先的对话已压缩" {
		t.Errorf("摘要 = %q", entry.Data.CompactionSummary)
	}
	if entry.Data.CompactionCount == nil || *entry.Data.CompactionCount != 12 {
		t.Errorf("条数 = %v，应为 12", entry.Data.CompactionCount)
	}
}

func TestProjectModelRetry(t *testing.T) {
	entry := mustProject(t, frame(40, "llm/retry", `{"attempt":2,"maxRetries":5,"delayMs":1500,"mode":"normal"}`))
	if entry.Kind != ChatNodeModelRetry {
		t.Fatalf("llm/retry kind = %q，应为 model-retry", entry.Kind)
	}
	if entry.Data.RetryAttempt == nil || *entry.Data.RetryAttempt != 2 {
		t.Errorf("attempt = %v", entry.Data.RetryAttempt)
	}
	if got := entry.Data.RetryText(); got != "重试 2/5" {
		t.Errorf("重试文案 = %q", got)
	}
}

func TestProjectTurnEndBranches(t *testing.T) {
	cases := []struct {
		name string
		data string
		want ChatNodeKind
	}{
		{"error", `{"reason":{"kind":"error","error":{"message":"boom","code":"E_BOOM"}}}`, ChatNodeTurnError},
		{"interrupted", `{"reason":{"kind":"interrupted"}}`, ChatNodeTurnError},
		{"max-tokens", `{"reason":{"kind":"max-tokens","limit":8192}}`, ChatNodeTurnMaxTokens},
		{"completed", `{"reason":{"kind":"completed"}}`, ChatNodeTurnTail},
	}
	for _, tc := range cases {
		entry := mustProject(t, frame(50, "turn/end", tc.data))
		if entry.Kind != tc.want {
			t.Errorf("%s: kind = %q，应为 %q", tc.name, entry.Kind, tc.want)
		}
	}
	errEntry := mustProject(t, frame(51, "turn/end", `{"reason":{"kind":"error","error":{"message":"boom","code":"E_BOOM"}}}`))
	if errEntry.Data.ErrorMessage != "boom" || errEntry.Data.ErrorCode != "E_BOOM" {
		t.Errorf("错误载荷 = %+v", errEntry.Data)
	}
	interrupted := mustProject(t, frame(52, "turn/end", `{"reason":{"kind":"interrupted"}}`))
	if interrupted.Data.ErrorCode != "INTERRUPTED" {
		t.Errorf("interrupted 的码 = %q，应为 INTERRUPTED", interrupted.Data.ErrorCode)
	}
	maxTokens := mustProject(t, frame(53, "turn/end", `{"reason":{"kind":"max-tokens","limit":8192}}`))
	if maxTokens.Data.MaxTokensLimit == nil || *maxTokens.Data.MaxTokensLimit != 8192 {
		t.Errorf("max-tokens limit = %v", maxTokens.Data.MaxTokensLimit)
	}
}

func TestProjectTurnProcessAndTail(t *testing.T) {
	start := mustProject(t, frame(60, "turn/start", `{"turn":1}`))
	if start.Kind != ChatNodeTurnProcess {
		t.Fatalf("turn/start kind = %q，应为 turn-process", start.Kind)
	}
	tail := mustProject(t, frame(61, "turn/end", `{"reason":{"kind":"completed"},"usage":{"totalTokens":42}}`))
	if tail.Kind != ChatNodeTurnTail {
		t.Fatalf("turn/end(completed) kind = %q，应为 turn-tail", tail.Kind)
	}
	if !tail.Data.Usage.Known() || tail.Data.Usage.TotalTokens != 42 {
		t.Errorf("Turn 尾用量 = %+v", tail.Data.Usage)
	}
}

func TestProjectSystemPromptAndQuestionReply(t *testing.T) {
	sys := mustProject(t, frame(70, "system/message", `{"content":[{"type":"text","text":"你是助手"}]}`))
	if sys.Kind != ChatNodeSystemPrompt {
		t.Fatalf("system/message kind = %q，应为 system-prompt", sys.Kind)
	}
	header := mustProject(t, frame(71, "request/header", `{"content":[{"type":"text","text":"header"}]}`))
	if header.Kind != ChatNodeSystemPrompt {
		t.Fatalf("request/header kind = %q，应为 system-prompt", header.Kind)
	}
	reply := mustProject(t, frame(72, "user/message",
		`{"surfaceOp":"append","source":{"kind":"user-question-reply","callId":"call_9"},"content":[{"type":"text","text":"选 A"}]}`))
	if reply.Kind != ChatNodeQuestionReply {
		t.Fatalf("问题答复 kind = %q，应为 question-reply", reply.Kind)
	}
	if reply.Data.CallID != "call_9" {
		t.Errorf("callId = %q", reply.Data.CallID)
	}
}

func TestProjectWorkflowRun(t *testing.T) {
	entry := mustProject(t, frame(80, "tool-workflow/run-start", `{"runId":"r1","name":"review"}`))
	if entry.Kind != ChatNodeWorkflowRun {
		t.Fatalf("run-start kind = %q，应为 workflow-run", entry.Kind)
	}
	if entry.Data.RunName != "review" || entry.Data.RunID != "r1" {
		t.Errorf("工作流载荷 = %+v", entry.Data)
	}
}

// TestProjectUnknownFallback：不认识的帧必须落 unknown 且原样保留 JSON，**不编摘要**。
func TestProjectUnknownFallback(t *testing.T) {
	entry := mustProject(t, frame(90, "some/future-event", `{"foo":"bar"}`))
	if entry.Kind != ChatNodeUnknown {
		t.Fatalf("未知帧 kind = %q，应为 unknown", entry.Kind)
	}
	if entry.Data.Raw != `{"foo":"bar"}` {
		t.Errorf("未知帧原文 = %q（必须原样保留）", entry.Data.Raw)
	}
	if entry.Data.Text != "" || entry.Data.Summary != "" {
		t.Error("未知帧不得编造摘要/正文")
	}
}

// TestProjectAllRenderableKindsReachable 是**覆盖表**：19 种可渲染 kind 都能由真实帧投影出来。
//
// `submitted-plan` 不在表里——它没有对应的 v4 事件（官方真值）。
func TestProjectAllRenderableKindsReachable(t *testing.T) {
	frames := []apiclient.Frame{
		frame(1, "user/message", `{"surfaceOp":"append","id":"a","content":[{"type":"text","text":"u"}]}`),
		frame(2, "user/message", `{"surfaceOp":"append","turn":1,"id":"b","source":{"kind":"user"},"content":[{"type":"text","text":"s"}]}`),
		frame(3, "user/message", `{"surfaceOp":"append","source":{"kind":"notification"},"content":[{"type":"text","text":"c"}]}`),
		frame(4, "user/message", `{"surfaceOp":"replace","source":{"kind":"retry"},"content":[]}`),
		frame(5, "command/run", `{"command":{"name":"goal","args":"x"}}`),
		frame(6, "assistant/message", `{"content":[{"type":"text","text":"a"}]}`),
		frame(7, "tool/call", `{"name":"bash","arguments":"{}"}`),
		frame(8, "command/run", `{"command":{"name":"compact"}}`),
		frame(9, "user/message", `{"surfaceOp":"replace","source":{"kind":"compact-checkpoint"},"content":[{"type":"text","text":"m"}]}`),
		frame(10, "compaction/summary", `{"summary":"cp"}`),
		frame(11, "llm/retry", `{"attempt":1}`),
		frame(12, "turn/end", `{"reason":{"kind":"error","error":{"message":"e"}}}`),
		frame(13, "turn/end", `{"reason":{"kind":"max-tokens","limit":1}}`),
		frame(14, "turn/start", `{"turn":1}`),
		frame(15, "turn/end", `{"reason":{"kind":"completed"}}`),
		frame(16, "system/message", `{"content":[{"type":"text","text":"sp"}]}`),
		frame(17, "some/unknown", `{"raw":true}`),
		frame(18, "user/message", `{"surfaceOp":"append","source":{"kind":"user-question-reply","callId":"c"},"content":[]}`),
		frame(19, "tool-workflow/run-start", `{"runId":"r","name":"w"}`),
	}
	tr := NewTranscript()
	tr.ApplyFrames(frames)
	seen := map[ChatNodeKind]bool{}
	for _, entry := range tr.Entries() {
		seen[entry.Kind] = true
	}
	for _, kind := range ChatNodeKinds() {
		if kind == ChatNodeSubmittedPlan {
			if seen[kind] {
				t.Error("submitted-plan 不该由任何事件投影出来（官方没有渲染器/事件）")
			}
			continue
		}
		if !seen[kind] {
			t.Errorf("kind %q 没有任何事件投影出来（覆盖表缺一行）", kind)
		}
	}
	if len(seen) != 19 {
		t.Errorf("覆盖到 %d 种 kind，应为 19 种（20 减去恒 hidden 的 submitted-plan）", len(seen))
	}
}

// ── journalEpoch 与去重 ──────────────────────────────────────────────

func TestJournalEpochMismatchRebuildsBaseline(t *testing.T) {
	tr := NewTranscript()
	first := &apiclient.HistoryResult{
		JournalEpoch: 1,
		LastSeq:      5,
		Frames: []apiclient.Frame{
			frame(1, "user/message", `{"surfaceOp":"append","id":"a","content":[{"type":"text","text":"旧世代"}]}`),
		},
	}
	if applied, rebuilt := tr.ApplyHistory(first); applied != 1 || rebuilt {
		t.Fatalf("第一次 history：applied=%d rebuilt=%v，应为 1/false", applied, rebuilt)
	}
	if tr.Epoch() != 1 || tr.LastSeq() != 5 {
		t.Fatalf("基线 = epoch %d lastSeq %d，应为 1/5", tr.Epoch(), tr.LastSeq())
	}

	// 宿主重启换了 journal：epoch 变了，旧水位作废。
	second := &apiclient.HistoryResult{
		JournalEpoch: 2,
		LastSeq:      2,
		Frames: []apiclient.Frame{
			frame(1, "user/message", `{"surfaceOp":"append","id":"b","content":[{"type":"text","text":"新世代"}]}`),
			frame(2, "assistant/message", `{"content":[{"type":"text","text":"回答"}]}`),
		},
	}
	applied, rebuilt := tr.ApplyHistory(second)
	if !rebuilt {
		t.Fatal("journalEpoch 变了必须重建基线（rebuilt=true）")
	}
	if applied != 2 {
		t.Fatalf("重建后应落 2 条节点，实际 %d 条", applied)
	}
	entries := tr.Entries()
	if len(entries) != 2 {
		t.Fatalf("重建后 entries = %d 条，应为 2 条（旧条目必须被丢弃）", len(entries))
	}
	if entries[0].Data.Text != "新世代" {
		t.Errorf("第一条 = %q，应为新世代（旧世代必须整段丢弃）", entries[0].Data.Text)
	}
	if tr.Epoch() != 2 || tr.LastSeq() != 2 {
		t.Errorf("新基线 = epoch %d lastSeq %d，应为 2/2", tr.Epoch(), tr.LastSeq())
	}
}

func TestJournalEpochMatchAppends(t *testing.T) {
	tr := NewTranscript()
	if _, _ = tr.ApplyHistory(&apiclient.HistoryResult{
		JournalEpoch: 7, LastSeq: 1,
		Frames: []apiclient.Frame{frame(1, "user/message", `{"surfaceOp":"append","id":"a","content":[{"type":"text","text":"一"}]}`)},
	}); tr.Len() != 1 {
		t.Fatalf("首段 history 应落 1 条，实际 %d", tr.Len())
	}
	applied, rebuilt := tr.ApplyHistory(&apiclient.HistoryResult{
		JournalEpoch: 7, LastSeq: 2,
		Frames: []apiclient.Frame{frame(2, "assistant/message", `{"content":[{"type":"text","text":"二"}]}`)},
	})
	if rebuilt {
		t.Error("epoch 一致时不得重建基线")
	}
	if applied != 1 || tr.Len() != 2 {
		t.Fatalf("追加后 applied=%d len=%d，应为 1/2", applied, tr.Len())
	}
}

func TestDuplicateSeqSkipped(t *testing.T) {
	tr := NewTranscript()
	f := frame(1, "user/message", `{"surfaceOp":"append","id":"a","content":[{"type":"text","text":"一"}]}`)
	tr.ApplyFrames([]apiclient.Frame{f})
	applied := tr.ApplyFrames([]apiclient.Frame{f})
	if applied != 0 {
		t.Errorf("重复 seq 应被丢弃，实际落 %d 条", applied)
	}
	if tr.Len() != 1 {
		t.Errorf("重复帧不得产生第二条节点，实际 %d 条", tr.Len())
	}
}

func TestResetClearsBaseline(t *testing.T) {
	tr := NewTranscript()
	tr.ApplyHistory(&apiclient.HistoryResult{
		JournalEpoch: 3, LastSeq: 9,
		Frames: []apiclient.Frame{frame(9, "user/message", `{"surfaceOp":"append","content":[]}`)},
	})
	tr.Reset()
	if tr.Len() != 0 || tr.LastSeq() != 0 || tr.Epoch() != 0 {
		t.Errorf("Reset 后应全清，实际 len=%d lastSeq=%d epoch=%d", tr.Len(), tr.LastSeq(), tr.Epoch())
	}
}

// ── 「未知」不是 0 ───────────────────────────────────────────────────

func TestUnknownFieldsRenderAsUnknownNotZero(t *testing.T) {
	tr := NewTranscript()
	tr.ApplyFrames([]apiclient.Frame{frame(1, "user/message", `{"surfaceOp":"append","content":[]}`)})
	entry := tr.Entries()[0]

	if entry.TimeKnown() {
		t.Error("宿主没给事件时间，TimeKnown 必须为 false")
	}
	if got := entry.TimeText(); got != UnknownText {
		t.Errorf("时间文案 = %q，应为 %q（绝不显示 00:00:00）", got, UnknownText)
	}
	if entry.TurnKnown() {
		t.Error("宿主没给 Turn 坐标，TurnKnown 必须为 false")
	}
	if got := entry.TurnText(); got != UnknownText {
		t.Errorf("Turn 文案 = %q，应为 %q（绝不显示 Turn 0）", got, UnknownText)
	}
	var usage TranscriptUsage
	if usage.Known() {
		t.Error("零值用量必须是未知")
	}
	if got := usage.UsageText(); got != "" {
		t.Errorf("未知用量的文案 = %q，应为空串（调用方整段不画）", got)
	}
}

func TestKnownFieldsRenderRealValues(t *testing.T) {
	entry := TranscriptEntry{Time: 3_661_000, Turn: 0}
	if !entry.TimeKnown() || entry.TimeText() != "01:01:01" {
		t.Errorf("时间文案 = %q，应为 01:01:01", entry.TimeText())
	}
	if !entry.TurnKnown() || entry.TurnText() != "Turn 0" {
		t.Errorf("Turn 文案 = %q（Turn 0 是合法坐标，不能当未知）", entry.TurnText())
	}
	exit := 3
	data := TranscriptData{ExitCode: &exit}
	if got := data.ExitCodeText(); got != "exit 3" {
		t.Errorf("退出码文案 = %q", got)
	}
}

func TestRetryTextInfinityWhenModeNotNormal(t *testing.T) {
	data := TranscriptData{RetryAttempt: intPtr(1), RetryMode: "always"}
	if got := data.RetryText(); got != "重试 1/∞" {
		t.Errorf("非 normal 模式最大次数应为 ∞，实际 %q", got)
	}
	if got := (TranscriptData{}).RetryText(); got != "重试 未知/未知" {
		t.Errorf("全未知时 = %q", got)
	}
}

func intPtr(v int) *int { return &v }
