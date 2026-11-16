package dsrightbar

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func ledgerTestFrame(seq int, at int64, typ, data string) apiclient.Frame {
	return apiclient.Frame{Seq: seq, Time: at, Type: typ, Data: json.RawMessage(data)}
}

func ledgerTestInt64(value int64) *int64 { return &value }

func ledgerTestRecord(t *testing.T, model *ledgerModel, id string) *ledgerRecord {
	t.Helper()
	for _, record := range model.Records {
		if record.ID == id {
			return record
		}
	}
	t.Fatalf("record %q 不存在；全部记录：%+v", id, model.Records)
	return nil
}

func ledgerTestDense(records []*ledgerRecord) error {
	for i, record := range records {
		if record.Index != i+1 {
			return fmt.Errorf("Records[%d].Index=%d，期望 %d（ID=%s）", i, record.Index, i+1, record.ID)
		}
	}
	return nil
}

func TestLedgerAllKindsAssistantCallsAndTiming(t *testing.T) {
	frames := []apiclient.Frame{
		ledgerTestFrame(1, 100, "system/message", `{"turn":1,"step":1,"message":{"id":"sys","role":"system","source":{"kind":"system-prompt"},"content":[{"type":"text","text":"be precise"}]}}`),
		ledgerTestFrame(2, 110, "user/message", `{"id":"u1","role":"user","source":{"kind":"user"},"content":[{"type":"text","text":"hello"}],"surfaceOp":"append"}`),
		ledgerTestFrame(3, 120, "developer/message", `{"turn":1,"step":1,"message":{"id":"d1","role":"developer","source":{"kind":"agent-instructions"},"content":[{"type":"text","text":"developer fact"}]}}`),
		ledgerTestFrame(4, 125, "user/message", `{"id":"ctx","role":"user","source":{"kind":"notice","form":"notice"},"content":[{"type":"text","text":"context fact"}],"surfaceOp":"append"}`),
		ledgerTestFrame(5, 1000, "step/start", `{"turn":1,"step":1}`),
		ledgerTestFrame(6, 1010, "request/header", `{"reason":"initial","header":{"config":{"provider":"route","model":"model-a","temperature":0.25,"maxTokens":900},"tools":[{"name":"bash","description":"run","parameters":{"type":"object","properties":{"command":{"type":"string"}}}}]}}`),
		ledgerTestFrame(7, 1200, "assistant/live-chunk", `{"turn":1,"step":1,"chunk":{"type":"text-delta","index":0,"text":"Hel"}}`),
		ledgerTestFrame(8, 1250, "assistant/reasoning", `{"turn":1,"step":1,"index":1,"text":"think"}`),
		ledgerTestFrame(9, 1300, "tool/call", `{"turn":1,"step":1,"callId":"call-root","name":"bash","arguments":"{\"command\":\"pwd\"}"}`),
		ledgerTestFrame(10, 1400, "tool/ptc-dispatch-start", `{"rootCallId":"call-root","parentCallId":"call-root","subCallId":"call-child","name":"read","arguments":{"path":"a.txt"}}`),
		ledgerTestFrame(11, 1410, "tool/ptc-dispatch-start", `{"rootCallId":"call-root","parentCallId":"call-child","subCallId":"call-grandchild","name":"grep","arguments":{"pattern":"x"}}`),
		ledgerTestFrame(12, 1500, "tool/ptc-dispatch", `{"rootCallId":"call-root","parentCallId":"call-child","subCallId":"call-grandchild","name":"grep","arguments":{"pattern":"x"},"isError":false,"content":[{"type":"text","text":"deep output"}]}`),
		ledgerTestFrame(13, 1600, "tool/ptc-dispatch", `{"rootCallId":"call-root","parentCallId":"call-root","subCallId":"call-child","name":"read","arguments":{"path":"a.txt"},"isError":false,"content":[{"type":"text","text":"child output"}]}`),
		ledgerTestFrame(14, 1700, "tool/result", `{"turn":1,"step":1,"message":{"id":"tr","role":"tool","source":{"kind":"tool","callId":"call-root"},"toolCallId":"call-root","content":[{"type":"text","text":"root output"}]}}`),
		ledgerTestFrame(15, 1800, "assistant/message", `{"turn":1,"step":1,"message":{"id":"a1","role":"assistant","source":{"kind":"model","provider":"actual-provider","model":"actual-model"},"content":[{"type":"text","text":"Hello"},{"type":"reasoning","text":"think"},{"type":"tool-call","id":"call-root","name":"bash","arguments":"{\"command\":\"pwd\"}"}]},"stream":[],"usage":{"inputTokens":10,"cacheReadTokens":20,"cacheWriteTokens":30,"outputTokens":40,"reasoningTokens":50}}`),
		ledgerTestFrame(16, 1900, "step/end", `{"turn":1,"step":1}`),
		ledgerTestFrame(17, 2000, "turn/end", `{"turn":1,"reason":{"kind":"completed"}}`),
	}

	partial := foldLedger(frames[:6])
	assistantID := ledgerAssistantID(1, 1)
	placeholder := ledgerTestRecord(t, partial, assistantID)
	if !placeholder.RequestOnly {
		t.Fatal("step/start 后、assistant 前必须是 request-only 行")
	}
	placeholderIndex := placeholder.Index

	model := foldLedger(frames)
	if err := ledgerTestDense(model.Records); err != nil {
		t.Fatal(err)
	}
	assistant := ledgerTestRecord(t, model, assistantID)
	if assistant.RequestOnly || assistant.Index != placeholderIndex {
		t.Fatalf("assistant 未在原位填实：before=%d after=%d requestOnly=%v", placeholderIndex, assistant.Index, assistant.RequestOnly)
	}
	if assistant.Output != "Hello" || assistant.Thinking != "think" {
		t.Fatalf("assistant 正文/思考错误：%+v", assistant)
	}
	if assistant.StartedAt == nil || *assistant.StartedAt != 1000 ||
		assistant.FirstTokenAt == nil || *assistant.FirstTokenAt != 1200 ||
		assistant.CompletedAt == nil || *assistant.CompletedAt != 1800 ||
		assistant.DurationMS == nil || *assistant.DurationMS != 800 {
		t.Fatalf("assistant 计时不是来自帧 time：%+v", assistant)
	}
	request := model.Requests["assistant:1:1"]
	if request == nil || request.Number != 1 || request.Status != "complete" || request.ResultSeq != 15 {
		t.Fatalf("assistant request 错误：%+v", request)
	}
	if request.Provider != "actual-provider" || request.Model != "actual-model" || !strings.Contains(request.Options, `"maxTokens": 900`) {
		t.Fatalf("request route/options 未保留：%+v", request)
	}
	if assistant.Options == "" || !strings.Contains(assistant.Prompt, "be precise") || !strings.Contains(assistant.Tools, `"bash"`) {
		t.Fatalf("assistant Inspector 请求数据不全：%+v", assistant)
	}
	if got := ledgerUsageJSON(assistant.Usage); got != `{"inputTokens":10,"cacheReadTokens":20,"cacheWriteTokens":30,"outputTokens":40,"reasoningTokens":50}` {
		t.Fatalf("usage 五桶错误：%s", got)
	}

	root := ledgerTestRecord(t, model, ledgerToolID(ledgerTool, "call-root"))
	child := ledgerTestRecord(t, model, ledgerToolID(ledgerSubtool, "call-child"))
	grandchild := ledgerTestRecord(t, model, ledgerToolID(ledgerSubtool, "call-grandchild"))
	if assistant.Index+1 != root.Index || root.Index+1 != child.Index || child.Index+1 != grandchild.Index {
		t.Fatalf("TOOL/SUBTOOL 未紧随 owner：assistant=%d root=%d child=%d grandchild=%d", assistant.Index, root.Index, child.Index, grandchild.Index)
	}
	if root.AssistantID != assistantID || root.ParentID != assistantID || root.Depth != 1 ||
		child.AssistantID != assistantID || child.ParentID != root.ID || child.Depth != 2 ||
		grandchild.AssistantID != assistantID || grandchild.ParentID != child.ID || grandchild.Depth != 3 {
		t.Fatalf("工具树身份/深度错误：root=%+v child=%+v grandchild=%+v", root, child, grandchild)
	}
	if root.Input != `{"command":"pwd"}` || root.Output != "root output" || !strings.Contains(root.Schema, `"properties"`) {
		t.Fatalf("root Inspector 数据错误：%+v", root)
	}
	if root.DurationMS == nil || *root.DurationMS != 400 || child.DurationMS == nil || *child.DurationMS != 200 || grandchild.DurationMS == nil || *grandchild.DurationMS != 90 {
		t.Fatalf("工具耗时错误：root=%v child=%v grandchild=%v", root.DurationMS, child.DurationMS, grandchild.DurationMS)
	}
	if child.Output != "child output" || grandchild.Output != "deep output" || !strings.Contains(child.Source, "child output") {
		t.Fatalf("subtool 输出/source 错误：child=%+v grandchild=%+v", child, grandchild)
	}

	ledgerTestRecord(t, model, ledgerSeqID(ledgerSystem, 1))
	ledgerTestRecord(t, model, ledgerSeqID(ledgerUser, 2))
	ledgerTestRecord(t, model, ledgerSeqID(ledgerContext, 3))
	ledgerTestRecord(t, model, ledgerSeqID(ledgerContext, 4))
	if len(model.Turns) != 1 || model.Turns[0].Turn != 1 || len(model.Turns[0].Records) != len(model.Records) {
		t.Fatalf("Turn 分组错误：%+v", model.Turns)
	}
	if search := ledgerSearchText(assistant); !strings.Contains(search, "actual-provider") || !strings.Contains(search, "hello") {
		t.Fatalf("assistant 搜索字段不完整：%q", search)
	}
}

func TestLedgerCompactionAndSessionCumulativeUsage(t *testing.T) {
	frames := []apiclient.Frame{
		ledgerTestFrame(1, 1000, "step/start", `{"turn":1,"step":1}`),
		ledgerTestFrame(2, 1100, "assistant/message", `{"turn":1,"step":1,"message":{"id":"a1","source":{"kind":"model","provider":"p","model":"m"},"content":[{"type":"text","text":"one"}]},"stream":[],"usage":{"inputTokens":1,"cacheReadTokens":2,"cacheWriteTokens":3,"outputTokens":4,"reasoningTokens":5}}`),
		ledgerTestFrame(3, 1200, "step/end", `{"turn":1,"step":1}`),
		ledgerTestFrame(4, 2000, "compaction/start", `{"compactionId":"compact-1","turn":null}`),
		ledgerTestFrame(5, 2300, "compaction/summary", `{"compactionId":"compact-1","summary":[{"type":"text","text":"safe summary"}],"rawOutput":[{"type":"reasoning","text":"raw thinking"},{"type":"text","text":"raw summary"}],"provider":"compact-provider","model":"compact-model","maxTokens":321,"usage":{"inputTokens":10,"cacheReadTokens":20,"cacheWriteTokens":30,"outputTokens":40,"reasoningTokens":50},"shadowedRange":{"start":1,"end":2},"shadowedSeqs":[1,2],"shadowedTokenCount":7}`),
		ledgerTestFrame(6, 2500, "compaction/end", `{"compactionId":"compact-1","turn":null}`),
		ledgerTestFrame(7, 3000, "step/start", `{"turn":2,"step":1}`),
		ledgerTestFrame(8, 3100, "assistant/message", `{"turn":2,"step":1,"message":{"id":"a2","source":{"kind":"model","provider":"p","model":"m"},"content":[{"type":"text","text":"two"}]},"stream":[],"usage":{"inputTokens":100,"cacheReadTokens":200,"cacheWriteTokens":300,"outputTokens":400,"reasoningTokens":500}}`),
		ledgerTestFrame(9, 3200, "step/end", `{"turn":2,"step":1}`),
	}
	model := foldLedger(frames)
	first := model.Requests["assistant:1:1"]
	compaction := model.Requests["compaction:4"]
	last := model.Requests["assistant:2:1"]
	if first == nil || compaction == nil || last == nil || first.Number != 1 || compaction.Number != 2 || last.Number != 3 {
		t.Fatalf("请求号未按 session 顺序：first=%+v compaction=%+v last=%+v", first, compaction, last)
	}
	if got := ledgerUsageJSON(first.CumulativeUsage); got != `{"inputTokens":1,"cacheReadTokens":2,"cacheWriteTokens":3,"outputTokens":4,"reasoningTokens":5}` {
		t.Fatalf("首个累计错误：%s", got)
	}
	if got := ledgerUsageJSON(compaction.CumulativeUsage); got != `{"inputTokens":11,"cacheReadTokens":22,"cacheWriteTokens":33,"outputTokens":44,"reasoningTokens":55}` {
		t.Fatalf("compaction 累计错误：%s", got)
	}
	if got := ledgerUsageJSON(last.CumulativeUsage); got != `{"inputTokens":111,"cacheReadTokens":222,"cacheWriteTokens":333,"outputTokens":444,"reasoningTokens":555}` {
		t.Fatalf("session 累计错误：%s", got)
	}
	record := ledgerTestRecord(t, model, ledgerSeqID(ledgerCompacted, 4))
	if record.Kind != ledgerCompacted || record.Turn != 0 || record.Output != "safe summary" || record.Thinking != "raw thinking" {
		t.Fatalf("compaction 行错误：%+v", record)
	}
	if !strings.Contains(record.Raw, "raw summary") || !strings.Contains(record.Source, "raw thinking") {
		t.Fatalf("compaction rawOutput 未保留：%+v", record)
	}
	if compaction.Provider != "compact-provider" || compaction.Model != "compact-model" ||
		!strings.Contains(compaction.Options, `"maxTokens": 321`) || !strings.Contains(compaction.Options, `"purpose": "compaction"`) ||
		compaction.Status != "complete" || !compaction.Compaction || compaction.ResultSeq != 5 {
		t.Fatalf("compaction request facts 不全：%+v", compaction)
	}
	if record.StartedAt == nil || *record.StartedAt != 2000 || record.CompletedAt == nil || *record.CompletedAt != 2500 || record.DurationMS == nil || *record.DurationMS != 500 {
		t.Fatalf("compaction 计时错误：%+v", record)
	}
}

func TestLedgerSystemUpdateDiffAndPromptSnapshots(t *testing.T) {
	frames := []apiclient.Frame{
		ledgerTestFrame(1, 100, "system/message", `{"turn":1,"step":1,"message":{"source":{"kind":"system-prompt"},"content":[{"type":"text","text":"line 1\nold line\nline 3"}]}}`),
		ledgerTestFrame(2, 110, "request/header", `{"reason":"initial","header":{"config":{"provider":"p","model":"m"},"tools":[{"name":"read","description":"r","parameters":{"type":"object"}}]}}`),
		ledgerTestFrame(3, 200, "system/message", `{"turn":1,"step":1,"message":{"source":{"kind":"system-prompt"},"content":[{"type":"text","text":"line 1\nnew line\nline 3"}]},"surfaceOp":"append"}`),
		ledgerTestFrame(4, 210, "request/header", `{"reason":"change","header":{"config":{"provider":"p2","model":"m2"},"tools":[{"name":"write","description":"w","parameters":{"type":"object"}}]}}`),
	}
	model := foldLedger(frames)
	initial := ledgerTestRecord(t, model, ledgerSeqID(ledgerSystem, 1))
	updated := ledgerTestRecord(t, model, ledgerSeqID(ledgerSystem, 3))
	if initial.Text != "Initial system prompt" || !strings.Contains(initial.Prompt, "old line") || initial.PreviousPrompt != "" {
		t.Fatalf("initial system 投影错误：%+v", initial)
	}
	if updated.Text != "System prompt updated" || !strings.Contains(updated.Prompt, "new line") || !strings.Contains(updated.PreviousPrompt, "old line") {
		t.Fatalf("system previous/current 未保留：%+v", updated)
	}
	if !strings.Contains(updated.Diff, "--- before") || !strings.Contains(updated.Diff, "-old line") || !strings.Contains(updated.Diff, "+new line") || !strings.Contains(updated.Diff, " line 1") {
		t.Fatalf("system diff 不是真实行差分：%q", updated.Diff)
	}
	// in-history system 更新已经在 seq 3 展示；后续 header 只新增真实的 tools 变化。
	toolsUpdated := ledgerTestRecord(t, model, ledgerSeqID(ledgerSystem, 4))
	if toolsUpdated.Text != "Tools updated" || !strings.Contains(toolsUpdated.Options, `"provider": "p2"`) || !strings.Contains(toolsUpdated.Tools, `"write"`) || strings.Contains(toolsUpdated.Tools, `"read"`) {
		t.Fatalf("header options/tools 未更新：%+v", toolsUpdated)
	}
	if diff := ledgerDiff("same", "same"); diff != "" {
		t.Fatalf("相同文本不应产生 diff：%q", diff)
	}
}

func TestLedgerErrorSanitization(t *testing.T) {
	frames := []apiclient.Frame{
		ledgerTestFrame(1, 1000, "step/start", `{"turn":1,"step":1}`),
		ledgerTestFrame(2, 1100, "llm/retry", `{"turn":1,"step":1,"provider":"p","mode":"normal","retry":2,"maxRetries":4,"delayMs":250,"failure":{"code":"AUTH","message":"secret-token-do-not-echo"}}`),
		ledgerTestFrame(3, 1200, "tool/call", `{"turn":1,"step":1,"callId":"auth-tool","name":"secure","arguments":"{}"}`),
		ledgerTestFrame(4, 1300, "tool/result", `{"turn":1,"step":1,"message":{"source":{"kind":"tool","callId":"auth-tool"},"isError":true,"content":[]},"error":{"name":"AuthError","code":"AUTH","reason":"tool-secret-do-not-echo"}}`),
		ledgerTestFrame(5, 1400, "turn/end", `{"turn":1,"reason":{"kind":"error","error":{"code":"AUTH","message":"turn-secret-do-not-echo"}}}`),
	}
	model := foldLedger(frames)
	request := model.Requests["assistant:1:1"]
	if request == nil || request.Error != "" || request.ErrorCode != "AUTH" || request.Retry == nil || *request.Retry != 2 || request.MaxRetries == nil || *request.MaxRetries != 4 || request.RetryDelayMS == nil || *request.RetryDelayMS != 250 {
		t.Fatalf("AUTH retry 未净化或重试字段丢失：%+v", request)
	}
	tool := ledgerTestRecord(t, model, ledgerToolID(ledgerTool, "auth-tool"))
	if tool.Output != "AuthError: AUTH" || strings.Contains(tool.Raw, "tool-secret") {
		t.Fatalf("工具 AUTH 错误泄露或摘要错误：%+v", tool)
	}
	encoded, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"secret-token-do-not-echo", "tool-secret-do-not-echo", "turn-secret-do-not-echo"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("模型含敏感 AUTH message %q：%s", secret, encoded)
		}
	}
	if search := ledgerSearchText(ledgerTestRecord(t, model, ledgerAssistantID(1, 1))); strings.Contains(search, "secret") {
		t.Fatalf("搜索索引含敏感信息：%q", search)
	}

	nonAuth := foldLedger([]apiclient.Frame{
		ledgerTestFrame(1, 1000, "step/start", `{"turn":1,"step":1}`),
		ledgerTestFrame(2, 1100, "llm/retry", `{"turn":1,"step":1,"mode":"always","retry":1,"delayMs":5,"failure":{"code":"RATE_LIMIT","message":"slow down"}}`),
	})
	if got := nonAuth.Requests["assistant:1:1"]; got.Error != "slow down" || got.ErrorCode != "RATE_LIMIT" || got.MaxRetries != nil {
		t.Fatalf("非 AUTH 错误或 always 模式投影错误：%+v", got)
	}
}

func TestLedgerPrependStableIdentity(t *testing.T) {
	older := []apiclient.Frame{
		ledgerTestFrame(1, 100, "system/message", `{"turn":1,"step":1,"message":{"source":{"kind":"system-prompt"},"content":[{"type":"text","text":"sys"}]}}`),
		ledgerTestFrame(2, 110, "user/message", `{"id":"u","source":{"kind":"user"},"content":[{"type":"text","text":"first"}]}`),
		ledgerTestFrame(3, 1000, "step/start", `{"turn":1,"step":1}`),
		ledgerTestFrame(4, 1200, "tool/call", `{"turn":1,"step":1,"callId":"stable-call","name":"read","arguments":"{}"}`),
		ledgerTestFrame(5, 1300, "assistant/message", `{"turn":1,"step":1,"message":{"id":"a","source":{"kind":"model","provider":"p","model":"m"},"content":[{"type":"tool-call","id":"stable-call","name":"read","arguments":"{}"}]},"stream":[]}`),
		ledgerTestFrame(6, 1400, "step/end", `{"turn":1,"step":1}`),
	}
	newer := []apiclient.Frame{
		ledgerTestFrame(7, 2000, "step/start", `{"turn":2,"step":1}`),
		ledgerTestFrame(8, 2100, "assistant/message", `{"turn":2,"step":1,"message":{"id":"b","source":{"kind":"model","provider":"p","model":"m"},"content":[{"type":"text","text":"new"}]},"stream":[]}`),
	}
	full := foldLedger(append(append([]apiclient.Frame(nil), older...), newer...))
	suffix := foldLedger(newer)
	identities := map[string]bool{}
	for _, record := range full.Records {
		identities[record.ID] = true
	}
	for _, id := range []string{
		ledgerSeqID(ledgerSystem, 1), ledgerSeqID(ledgerUser, 2),
		ledgerAssistantID(1, 1), ledgerToolID(ledgerTool, "stable-call"),
	} {
		if !identities[id] {
			t.Errorf("prepend 后旧身份 %q 消失", id)
		}
	}
	for _, record := range suffix.Records {
		if !identities[record.ID] {
			t.Errorf("prepend 后 suffix 身份 %q 改变", record.ID)
		}
	}
	if got := ledgerTestRecord(t, full, ledgerToolID(ledgerTool, "stable-call")).ID; got != "tool\x00call\x00stable-call" {
		t.Fatalf("工具 ID 不是 callId 身份：%q", got)
	}
	if err := ledgerTestDense(full.Records); err != nil {
		t.Fatal(err)
	}
}

func TestLedgerPTCCycleDepthAndJSONArguments(t *testing.T) {
	frames := []apiclient.Frame{
		ledgerTestFrame(1, 1000, "tool/call", `{"turn":1,"step":1,"callId":"root","name":"run_code","arguments":"{}"}`),
	}
	for i := 1; i <= 255; i++ {
		parent := "root"
		if i > 1 {
			parent = fmt.Sprintf("sub-%d", i-1)
		}
		data := fmt.Sprintf(`{"rootCallId":"root","parentCallId":%q,"subCallId":"sub-%d","name":"read","arguments":"text"}`, parent, i)
		frames = append(frames, ledgerTestFrame(i+1, int64(1000+i), "tool/ptc-dispatch-start", data))
	}
	frames = append(frames,
		ledgerTestFrame(257, 2000, "tool/ptc-dispatch-start", `{"rootCallId":"root","parentCallId":"sub-255","subCallId":"overflow","name":"read","arguments":{}}`),
		ledgerTestFrame(258, 2001, "tool/ptc-dispatch-start", `{"rootCallId":"root","parentCallId":"sub-1","subCallId":"root","name":"run_code","arguments":{}}`),
	)
	model := foldLedger(frames)
	if len(model.Records) != 256 {
		t.Fatalf("深度上限应保留 root + 255 个 subtool，实际 %d", len(model.Records))
	}
	first := ledgerTestRecord(t, model, ledgerToolID(ledgerSubtool, "sub-1"))
	last := ledgerTestRecord(t, model, ledgerToolID(ledgerSubtool, "sub-255"))
	if first.Input != `"text"` || first.Depth != 2 || last.Depth != 256 {
		t.Fatalf("PTC JSON arguments/depth 错误：first=%+v last=%+v", first, last)
	}
	for _, record := range model.Records {
		if record.ID == ledgerToolID(ledgerSubtool, "overflow") || record.ID == ledgerToolID(ledgerSubtool, "root") {
			t.Fatalf("越界/环边被接受：%+v", record)
		}
	}
	if err := ledgerTestDense(model.Records); err != nil {
		t.Fatal(err)
	}
}

func TestLedgerCompactStreamTimingAndUnknownTimes(t *testing.T) {
	frames := []apiclient.Frame{
		ledgerTestFrame(1, 1000, "step/start", `{"turn":1,"step":1}`),
		ledgerTestFrame(2, 1500, "assistant/message", `{"turn":1,"step":1,"message":{"id":"a","source":{"kind":"model","provider":"p","model":"m"},"content":[{"type":"text","text":"ab"}]},"stream":[{"type":"text-chunks","time0":1100,"index":0,"dt":[100],"texts":["","a"]},{"type":"chunk","time":1400,"chunk":{"type":"usage","usage":{"inputTokens":1,"outputTokens":2}}}],"usage":{"inputTokens":1,"outputTokens":2}}`),
		ledgerTestFrame(3, 1600, "step/end", `{"turn":1,"step":1}`),
		ledgerTestFrame(4, 0, "step/start", `{"turn":2,"step":1}`),
		ledgerTestFrame(5, 0, "assistant/text", `{"turn":2,"step":1,"index":0,"text":"unknown"}`),
		ledgerTestFrame(6, 0, "assistant/message", `{"turn":2,"step":1,"message":{"id":"b","source":{"kind":"model","provider":"p","model":"m"},"content":[{"type":"text","text":"unknown"}]},"stream":[]}`),
		ledgerTestFrame(7, 0, "step/end", `{"turn":2,"step":1}`),
	}
	model := foldLedger(frames)
	timed := ledgerTestRecord(t, model, ledgerAssistantID(1, 1))
	if timed.FirstTokenAt == nil || *timed.FirstTokenAt != 1200 || timed.DurationMS == nil || *timed.DurationMS != 500 {
		t.Fatalf("packed stream TTFT/完成时间错误：%+v", timed)
	}
	unknown := ledgerTestRecord(t, model, ledgerAssistantID(2, 1))
	if unknown.StartedAt != nil || unknown.FirstTokenAt != nil || unknown.CompletedAt != nil || unknown.DurationMS != nil {
		t.Fatalf("缺失 time 被补成 0 时间：%+v", unknown)
	}
	if unknown.Request.StartedAt != nil || unknown.Request.CompletedAt != nil {
		t.Fatalf("request 缺失 time 被补值：%+v", unknown.Request)
	}
}

func TestLedgerLiveUsageRetryResetAndUsageJSON(t *testing.T) {
	frames := []apiclient.Frame{
		ledgerTestFrame(1, 1000, "step/start", `{"turn":1,"step":1}`),
		ledgerTestFrame(2, 1100, "assistant/live-chunk", `{"turn":1,"step":1,"chunk":{"type":"text-delta","index":0,"text":"first attempt"}}`),
		ledgerTestFrame(3, 1110, "assistant/live-chunk", `{"turn":1,"step":1,"chunk":{"type":"usage","usage":{"inputTokens":1,"outputTokens":2}}}`),
		ledgerTestFrame(4, 1200, "llm/retry", `{"turn":1,"step":1,"mode":"normal","retry":1,"maxRetries":2,"delayMs":10,"failure":{"code":"RATE_LIMIT","message":"retry"}}`),
		ledgerTestFrame(5, 1300, "assistant/live-chunk", `{"turn":1,"step":1,"chunk":{"type":"text-delta","index":0,"text":"second attempt"}}`),
		ledgerTestFrame(6, 1310, "assistant/live-chunk", `{"turn":1,"step":1,"chunk":{"type":"usage","usage":{"inputTokens":3,"cacheReadTokens":4,"outputTokens":5}}}`),
		ledgerTestFrame(7, 1400, "step/end", `{"turn":1,"step":1}`),
	}
	model := foldLedger(frames)
	record := ledgerTestRecord(t, model, ledgerAssistantID(1, 1))
	if record.Output != "second attempt" || strings.Contains(record.Output, "first attempt") {
		t.Fatalf("retry 未重置可见 attempt：%q", record.Output)
	}
	if record.FirstTokenAt == nil || *record.FirstTokenAt != 1100 {
		t.Fatalf("retry 不应丢掉首个 token 时间：%+v", record.FirstTokenAt)
	}
	if got := ledgerUsageJSON(record.Usage); got != `{"inputTokens":4,"cacheReadTokens":4,"outputTokens":7}` {
		t.Fatalf("live usage 未跨 attempt 累计：%s", got)
	}
	if !record.IsError || record.Request.Status != "error" || record.DurationMS == nil || *record.DurationMS != 400 {
		t.Fatalf("未 settle 的 step/end 边界错误：%+v", record)
	}
	zero := int64(0)
	if got := ledgerUsageJSON(&ledgerUsage{Input: &zero}); got != `{"inputTokens":0}` {
		t.Fatalf("真实 0 bucket 被吞掉：%s", got)
	}
	if got := ledgerUsageJSON(nil); got != "" {
		t.Fatalf("未知 usage 应隐藏：%q", got)
	}
}

func TestLedgerSessionEndSeedInterruptsOnlyRunningCompaction(t *testing.T) {
	model := foldLedger([]apiclient.Frame{
		ledgerTestFrame(1, 1000, "compaction/start", `{"compactionId":"open","turn":null}`),
		ledgerTestFrame(2, 1100, "compaction/start", `{"compactionId":"closed","turn":null}`),
		ledgerTestFrame(3, 1200, "compaction/end", `{"compactionId":"closed","turn":null}`),
		ledgerTestFrame(4, 1500, "session/end-seed", `{"inherited":true}`),
		ledgerTestFrame(5, 1600, "compaction/start", `{"compactionId":"after","turn":null}`),
	})
	interrupted := model.Requests["compaction:1"]
	if interrupted.Status != "error" || interrupted.Error != ledgerCompactionInterrupted || interrupted.ErrorCode != ledgerCompactionInterrupted || interrupted.CompletedAt == nil || *interrupted.CompletedAt != 1500 {
		t.Fatalf("session/end-seed 未中断 running compaction：%+v", interrupted)
	}
	record := ledgerTestRecord(t, model, ledgerSeqID(ledgerCompacted, 1))
	if !record.IsError || record.Text != ledgerCompactionInterrupted || record.DurationMS == nil || *record.DurationMS != 500 {
		t.Fatalf("compaction interrupted 行错误：%+v", record)
	}
	if closed := model.Requests["compaction:2"]; closed.Status != "complete" || closed.CompletedAt == nil || *closed.CompletedAt != 1200 {
		t.Fatalf("已完成 compaction 被边界改写：%+v", closed)
	}
	if after := model.Requests["compaction:5"]; after.Status != "running" || after.CompletedAt != nil {
		t.Fatalf("boundary 之后的新 compaction 被误中断：%+v", after)
	}
}

func TestLedgerSteeringClaimCanceledAndOrdinaryTurn(t *testing.T) {
	model := foldLedger([]apiclient.Frame{
		ledgerTestFrame(1, 100, "user/message", `{"id":"ordinary-1","source":{"kind":"user"},"content":[{"type":"text","text":"first"}]}`),
		ledgerTestFrame(2, 200, "assistant/message", `{"turn":1,"step":1,"message":{"id":"a1","source":{"kind":"model","provider":"p","model":"m"},"content":[{"type":"text","text":"answer"}]},"stream":[]}`),
		ledgerTestFrame(3, 300, "agent/inbox/spliced", `{"target":"next-step","start":0,"removedCount":0,"inserted":[{"id":"steering-1"}]}`),
		ledgerTestFrame(4, 310, "agent/inbox/spliced", `{"target":"next-step","start":0,"removedCount":1,"inserted":[]}`),
		ledgerTestFrame(5, 320, "user/message", `{"id":"steering-1","source":{"kind":"user"},"content":[{"type":"text","text":"steer"}]}`),
		ledgerTestFrame(6, 400, "agent/inbox/spliced", `{"target":"next-step","start":0,"removedCount":0,"inserted":[{"id":"canceled-1"}]}`),
		ledgerTestFrame(7, 410, "agent/inbox/spliced", `{"target":"next-step","start":0,"removedCount":1,"inserted":[],"outcome":"canceled"}`),
		ledgerTestFrame(8, 420, "user/message", `{"id":"canceled-1","source":{"kind":"user"},"content":[{"type":"text","text":"not steering"}]}`),
		ledgerTestFrame(9, 430, "user/message", `{"id":"ordinary-2","source":{"kind":"user"},"content":[{"type":"text","text":"next turn"}]}`),
		ledgerTestFrame(10, 500, "agent/inbox/spliced", `{"target":"next-step","start":0,"removedCount":0,"inserted":[{"id":"default-not-claimed"}]}`),
		ledgerTestFrame(11, 510, "agent/inbox/spliced", `{"target":"next-step","start":0,"inserted":[{"id":"replacement"}]}`),
		ledgerTestFrame(12, 520, "user/message", `{"id":"default-not-claimed","source":{"kind":"user"},"content":[{"type":"text","text":"default zero"}]}`),
	})
	steering := ledgerTestRecord(t, model, ledgerSeqID(ledgerUser, 5))
	if steering.Kind != ledgerUser || steering.Turn != 1 || steering.Group != "Message" {
		t.Fatalf("current claim 未归为前一轮 steering：%+v", steering)
	}
	canceled := ledgerTestRecord(t, model, ledgerSeqID(ledgerUser, 8))
	if canceled.Turn != 2 {
		t.Fatalf("canceled claim 被错误认领为 steering：%+v", canceled)
	}
	ordinary := ledgerTestRecord(t, model, ledgerSeqID(ledgerUser, 9))
	if ordinary.Turn != 2 {
		t.Fatalf("普通下一轮被旧 claim 错误认领：%+v", ordinary)
	}
	defaultZero := ledgerTestRecord(t, model, ledgerSeqID(ledgerUser, 12))
	if defaultZero.Turn != 2 {
		t.Fatalf("removedCount 缺省应按官方 ?? 0 处理而不是 claim：%+v", defaultZero)
	}
}

func TestLedgerPTCLateParentAdoptsAndReorders(t *testing.T) {
	model := foldLedger([]apiclient.Frame{
		// grandchild 先于 parent、parent 又先于 native root 到达。
		ledgerTestFrame(1, 1100, "tool/ptc-dispatch-start", `{"rootCallId":"late-root","parentCallId":"late-parent","subCallId":"late-grandchild","name":"grep","arguments":{"pattern":"x"}}`),
		ledgerTestFrame(2, 1200, "tool/ptc-dispatch-start", `{"rootCallId":"late-root","parentCallId":"late-root","subCallId":"late-parent","name":"read","arguments":{"path":"a"}}`),
		ledgerTestFrame(3, 1300, "tool/call", `{"turn":1,"step":1,"callId":"late-root","name":"run_code","arguments":"{}"}`),
	})
	if len(model.Records) != 3 {
		t.Fatalf("late parent adopt 后记录数应为 3，实际 %d：%+v", len(model.Records), model.Records)
	}
	root := ledgerTestRecord(t, model, ledgerToolID(ledgerTool, "late-root"))
	parent := ledgerTestRecord(t, model, ledgerToolID(ledgerSubtool, "late-parent"))
	grandchild := ledgerTestRecord(t, model, ledgerToolID(ledgerSubtool, "late-grandchild"))
	if root.Index+1 != parent.Index || parent.Index+1 != grandchild.Index {
		t.Fatalf("late parent 未按 root→parent→grandchild 重排：%+v", model.Records)
	}
	assistantID := ledgerAssistantID(1, 1)
	if root.Depth != 1 || parent.Depth != 2 || grandchild.Depth != 3 ||
		parent.ParentID != root.ID || grandchild.ParentID != parent.ID ||
		parent.AssistantID != assistantID || grandchild.AssistantID != assistantID {
		t.Fatalf("late parent 身份/深度未正确补挂：root=%+v parent=%+v grandchild=%+v", root, parent, grandchild)
	}
	if parent.Input == "" || grandchild.Input == "" || parent.StartedAt == nil || grandchild.StartedAt == nil {
		t.Fatalf("late parent 保留了空 placeholder 而丢真实内容：parent=%+v grandchild=%+v", parent, grandchild)
	}
	if err := ledgerTestDense(model.Records); err != nil {
		t.Fatal(err)
	}
}

func TestLedgerAssistantAttemptSettlesTimingWithoutMessage(t *testing.T) {
	model := foldLedger([]apiclient.Frame{
		ledgerTestFrame(1, 1000, "step/start", `{"turn":1,"step":1}`),
		ledgerTestFrame(2, 1100, "llm/retry", `{"turn":1,"step":1,"mode":"normal","retry":1,"maxRetries":2,"delayMs":10,"failure":{"code":"RATE_LIMIT","message":"retry"}}`),
		ledgerTestFrame(3, 1500, "assistant/attempt", `{"turn":1,"step":1,"stream":[{"type":"text-chunks","time0":1200,"index":0,"dt":[50],"texts":["","x"]}]}`),
		ledgerTestFrame(4, 1600, "step/end", `{"turn":1,"step":1}`),
	})
	record := ledgerTestRecord(t, model, ledgerAssistantID(1, 1))
	if !record.RequestOnly || record.Output != "" || record.Request.ResultSeq != 0 {
		t.Fatalf("attempt 被伪造成 completed assistant：%+v", record)
	}
	if record.StartedAt == nil || *record.StartedAt != 1000 || record.FirstTokenAt == nil || *record.FirstTokenAt != 1250 || record.CompletedAt == nil || *record.CompletedAt != 1600 || record.DurationMS == nil || *record.DurationMS != 600 {
		t.Fatalf("attempt durable stream 未提供真实 TTFT/decode 边界：%+v", record)
	}
	if !record.IsError || record.Request.Status != "error" || record.Request.Error != "retry" {
		t.Fatalf("无最终 message 的 attempt 状态未按真实边界收束：%+v", record.Request)
	}
}

func TestLedgerPTCMissingParentKeepsRealChildWithoutFakeRoot(t *testing.T) {
	model := foldLedger([]apiclient.Frame{
		ledgerTestFrame(1, 1100, "tool/ptc-dispatch", `{"rootCallId":"missing-root","parentCallId":"missing-root","subCallId":"orphan-child","name":"read","arguments":{"path":"real"},"isError":false,"content":[{"type":"text","text":"real child output"}]}`),
	})
	if len(model.Records) != 1 {
		t.Fatalf("永久缺父只能保留真实 child，实际 %d：%+v", len(model.Records), model.Records)
	}
	child := ledgerTestRecord(t, model, ledgerToolID(ledgerSubtool, "orphan-child"))
	if child.ParentID != ledgerToolID(ledgerTool, "missing-root") || child.Depth != 2 || child.Output != "real child output" || child.Input == "" {
		t.Fatalf("缺父 child 的真实 ID/内容未保留：%+v", child)
	}
	if err := ledgerTestDense(model.Records); err != nil {
		t.Fatal(err)
	}
}
