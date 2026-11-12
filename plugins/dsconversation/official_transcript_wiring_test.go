package dsconversation

// official_transcript_wiring_test.go —— **产品绘制路径**的机器断言。
//
// GT2 的测试证明"渲染器自己能画"；本文件证明"**产品真的把它接上了**"，
// 两件事不能互相替代（渲染器全绿而主区空白的正是本任务要干掉的那种假通过）。
//
// 五组：
//
//	① 槽位：painter 挂在官方 `conversation.view` 字面量上（不是 main.body 冒充）
//	② 绘制：真实 viva 画布上出现**原始帧里那条消息的正文**（不是 mock 节点）
//	③ 回退：没有原始帧时交回旧渲染路径（不给用户空白主区）
//	④ 世代：journalEpoch 换代 → 整段重建（不丢帧、不混排）
//	⑤ 切会话：两条会话 seq 都从 1 起，切过去必须 Reset（否则新会话一条都不画）

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// productPlugin 造一个装配好的插件实例（主题/图元来自 ds-core，与线上一致）。
func productPlugin() *Plugin {
	theme := dscore.Dark()
	return &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
}

// renderSeat 用真实 viva 画布跑一次某个座位的 painter。
func renderSeat(t *testing.T, p *Plugin, snap pluginapi.Snapshot, painterID string) *gfx.Canvas {
	t.Helper()
	var painter *pluginapi.Painter
	for _, candidate := range p.Panels() {
		if candidate.ID == painterID {
			found := candidate
			painter = &found
			break
		}
	}
	if painter == nil {
		t.Fatalf("没有 painter %q", painterID)
	}
	canvas := &gfx.Canvas{}
	tree := &gfx.Tree{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size
	})
	env := &pluginapi.Env{
		Theme: p.theme, Data: snap, Tree: tree, Feat: tree.Feature("transcript-wiring-test"),
		Canvas: canvas, Bounds: ui.Rect{Width: 900, Height: 700}, MainWidth: 900,
	}
	env.Has = func(pluginapi.Slot) bool { return false }
	var view ui.Page
	view.Draw(canvas, dscore.VivaTheme(p.theme), ui.Rect{Width: 900, Height: 700}, func(f ui.C) {
		painter.Draw(f, env)
	})
	// 旧渲染路径的 Markdown 是**布局后覆画**（viva 的绘制顺序不受调用方控制）：
	// 只跑 Draw 看不到正文。壳对每条 painter 都会跑一次 AfterDraw，这里同形。
	if painter.AfterDraw != nil {
		painter.AfterDraw(canvas, env)
	}
	return canvas
}

// rawFrame 造一条原始 journal 帧。
func rawFrame(seq int, kind, data string) apiclient.Frame {
	frame := apiclient.Frame{Seq: seq, Type: kind, Time: 1_700_000_000_000 + int64(seq)}
	if data != "" {
		frame.Data = json.RawMessage(data)
	}
	return frame
}

// realFrames 造三条**真实形状**的 v4 帧：用户消息 → 助手正文 → 工具调用。
func realFrames() []apiclient.Frame {
	return []apiclient.Frame{
		rawFrame(1, "user/message",
			`{"surfaceOp":"append","source":{"kind":"user"},"id":"msg-user-1","content":[{"type":"text","text":"真实用户消息"}]}`),
		rawFrame(2, "assistant/message",
			`{"id":"msg-assistant-1","content":[{"type":"text","text":"真实助手回复"}]}`),
		rawFrame(3, "tool/call",
			`{"name":"bash","arguments":"{\"command\":\"go test ./...\"}","callId":"call_1"}`),
	}
}

func hasCanvasText(canvas *gfx.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}

func canvasValues(canvas *gfx.Canvas) []string {
	values := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		values = append(values, text.Value)
	}
	return values
}

// ── ① 槽位 ─────────────────────────────────────────────────────────────

// TestPanelsMountTranscriptOnOfficialConversationView 钉住挂载点是**官方槽名字面量**。
//
// 为什么单独断言：`main.body` 是旧槽，官方骨架根本不 dispatch 它。挂错了
// 的表现是"插件说自己挂了、界面上什么都没有"——没有任何报错。
func TestPanelsMountTranscriptOnOfficialConversationView(t *testing.T) {
	p := productPlugin()
	if SlotConversationView != "conversation.view" {
		t.Fatalf("官方槽名字面量被改过：%q", SlotConversationView)
	}
	var (
		official bool
		legacy   bool
	)
	for _, painter := range p.Panels() {
		switch painter.Slot {
		case pluginapi.Slot("conversation.view"):
			official = painter.Draw != nil
		case pluginapi.SlotMainBody:
			legacy = painter.Draw != nil
		}
	}
	if !official {
		t.Fatal("conversation.view 上没有 painter：官方骨架那一帧主区会是空白")
	}
	if !legacy {
		t.Fatal("main.body 上没有 painter：旧三栏 / 兼容桥那一帧主区会是空白")
	}
}

// newKernelWith 把 ds-core + 本插件装进一个真内核（Register 会注入主题/图元）。
func newKernelWith(t *testing.T, p *Plugin) *uikernel.Kernel {
	t.Helper()
	kernel := uikernel.New("transcript-wiring")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, p); err != nil {
		t.Fatalf("装配失败: %v", err)
	}
	return kernel
}

// TestOfficialSeatDispatchesThroughDrawSlot 用**真内核**证明 `drawSlot` 真的派发。
//
// 只断言 Panels() 里有个 painter 是不够的：内核不认识这个槽名时同样不会报错。
func TestOfficialSeatDispatchesThroughDrawSlot(t *testing.T) {
	p := productPlugin()
	kernel := newKernelWith(t, p)
	canvas := &gfx.Canvas{}
	tree := &gfx.Tree{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size
	})
	env := &pluginapi.Env{
		Theme: p.theme, Tree: tree, Feat: tree.Feature("transcript-drawslot-test"),
		Canvas: canvas, Bounds: ui.Rect{Width: 900, Height: 700}, MainWidth: 900,
		Data: pluginapi.Snapshot{Cur: "s-1", Frames: realFrames(), JournalEpoch: 1},
	}
	env.Has = func(slot pluginapi.Slot) bool { return kernel.Has(slot) }
	var view ui.Page
	view.Draw(canvas, dscore.VivaTheme(p.theme), ui.Rect{Width: 900, Height: 700}, func(f ui.C) {
		kernel.DrawSlot(f, pluginapi.Slot("conversation.view"), env)
	})
	if !hasCanvasText(canvas, "真实用户消息") {
		t.Fatalf("conversation.view 的 drawSlot 没有真的画消息；画布文本：%v", canvasValues(canvas))
	}
}

// ── ② 绘制：真实帧 → 画布文本 ──────────────────────────────────────────

// TestProductPathDrawsRealFrameText 是本任务的核心断言：**画布上出现原始帧里的
// 消息正文**。不是"投影里有几条"，是像素级画出来了。
func TestProductPathDrawsRealFrameText(t *testing.T) {
	p := productPlugin()
	canvas := renderSeat(t, p, pluginapi.Snapshot{Cur: "s-1", Frames: realFrames(), JournalEpoch: 1},
		"official-transcript")

	for _, want := range []string{"真实用户消息", "真实助手回复", "bash"} {
		if !hasCanvasText(canvas, want) && !hasCanvasTextPrefix(canvas, want) {
			t.Fatalf("画布缺少 %q；实际文本：%v", want, canvasValues(canvas))
		}
	}
	// 挂载 = 逐条真实行（不是虚拟列表）：三条帧 → 三条可见行。
	if got := len(p.transcriptRowKeys()); got != 3 {
		t.Fatalf("挂载行数 = %d，想要 3（逐条真实挂载）；keys=%v", got, p.transcriptRowKeys())
	}
	kinds := p.transcriptRowKinds()
	want := []ChatNodeKind{ChatNodeUser, ChatNodeAssistantStep, ChatNodeToolCall}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("第 %d 行 kind = %s，想要 %s（全部：%v）", i, kinds[i], want[i], kinds)
		}
	}
}

func hasCanvasTextPrefix(canvas *gfx.Canvas, prefix string) bool {
	for _, text := range canvas.Texts() {
		if strings.HasPrefix(text.Value, prefix) {
			return true
		}
	}
	return false
}

// TestLegacySeatDrawsSameTranscript 旧座位（main.body）画的是**同一份**节点。
//
// 两个座位只有滚动口形态不同（宿主口 vs 自滚口），内容必须一致——否则
// 切档位时用户看到的消息会变。
func TestLegacySeatDrawsSameTranscript(t *testing.T) {
	p := productPlugin()
	canvas := renderSeat(t, p, pluginapi.Snapshot{Cur: "s-1", Frames: realFrames(), JournalEpoch: 1},
		"stream")
	if !hasCanvasText(canvas, "真实用户消息") {
		t.Fatalf("旧座位没有画官方 transcript；画布文本：%v", canvasValues(canvas))
	}
	if got := len(p.transcriptRowKeys()); got != 3 {
		t.Fatalf("旧座位挂载行数 = %d，想要 3", got)
	}
}

// ── ③ 回退 ────────────────────────────────────────────────────────────

// TestOfficialSeatDrawsNothingWithoutFrames：没有原始帧时**不许**画空白骨架。
//
// 判据是"交回旧路径"：官方座位这一帧一个节点都不挂，旧座位照旧画消息列表。
func TestOfficialSeatDrawsNothingWithoutFrames(t *testing.T) {
	p := productPlugin()
	snap := pluginapi.Snapshot{
		Cur:  "s-1",
		Msgs: []apiclient.Message{{Seq: 1, Role: "assistant", Text: "旧投影里的回答"}},
	}
	canvas := renderSeat(t, p, snap, "official-transcript")
	if got := len(canvas.Texts()); got != 0 {
		t.Fatalf("没有原始帧时官方座位画了 %d 段文本，想要 0（应回退旧路径）", got)
	}
	legacy := renderSeat(t, p, snap, "stream")
	if !hasCanvasText(legacy, "旧投影里的回答") {
		t.Fatalf("没有原始帧时旧座位也没有消息：主区空白；文本：%v", canvasValues(legacy))
	}
}

// ── ④ 世代 ────────────────────────────────────────────────────────────

// TestEpochChangeRebuildsBaseline 钉住「journalEpoch 不匹配 → 重建基线」。
//
// 宿主重启换 journal 后 seq 从 1 起：拿旧水位硬补，宿主要么回 409、
// 要么从错位处推——**不报错也不丢帧，只是少了几条**。所以换代必须整段丢弃。
func TestEpochChangeRebuildsBaseline(t *testing.T) {
	p := productPlugin()
	first := realFrames()
	renderSeat(t, p, pluginapi.Snapshot{Cur: "s-1", Frames: first, JournalEpoch: 1}, "official-transcript")
	if got := len(p.transcriptRowKeys()); got != 3 {
		t.Fatalf("第一代挂载 %d 行，想要 3", got)
	}

	// 换代：新 journal 的 seq 从 1 起，内容完全不同（旧的三条必须整段消失）。
	second := []apiclient.Frame{
		rawFrame(1, "user/message",
			`{"surfaceOp":"append","source":{"kind":"user"},"content":[{"type":"text","text":"换代后的新消息"}]}`),
	}
	canvas := renderSeat(t, p, pluginapi.Snapshot{Cur: "s-1", Frames: second, JournalEpoch: 2}, "official-transcript")
	if got := len(p.transcriptRowKeys()); got != 1 {
		t.Fatalf("换代后挂载 %d 行，想要 1（旧代必须整段丢弃）；keys=%v", got, p.transcriptRowKeys())
	}
	if !hasCanvasText(canvas, "换代后的新消息") {
		t.Fatalf("换代后的正文没画出来：%v", canvasValues(canvas))
	}
	for _, stale := range []string{"真实用户消息", "真实助手回复"} {
		if hasCanvasText(canvas, stale) {
			t.Fatalf("旧世代的 %q 还留在画布上（新旧混排）", stale)
		}
	}
}

// TestSameEpochDoesNotDuplicate 同世代重放同一帧不许长出第二行。
func TestSameEpochDoesNotDuplicate(t *testing.T) {
	p := productPlugin()
	frames := realFrames()
	snap := pluginapi.Snapshot{Cur: "s-1", Frames: frames, JournalEpoch: 1}
	for i := 0; i < 3; i++ {
		renderSeat(t, p, snap, "official-transcript")
	}
	if got := len(p.transcriptRowKeys()); got != 3 {
		t.Fatalf("重放三帧三次后有 %d 行，想要 3（同 seq 只能算一次）", got)
	}
}

// TestGapFillInsertsInJournalOrder 补洞回来的帧按 **seq 顺序**落行，不按到达顺序。
//
// 断号时 WS 先推到 to+1，history 才把 [from,to] 补回来；按到达顺序追加会让
// 行序错乱（界面上是"消息顺序跳了一下"）。
func TestGapFillInsertsInJournalOrder(t *testing.T) {
	p := productPlugin()
	// 快照契约：frames **按 seq 升序**（排序由数据面保证，见 cmd/app/frames.go
	// 的 frameJournal.insert）。这里钉住的是"投影按 seq 顺序落行"这一半。
	frames := []apiclient.Frame{
		rawFrame(1, "user/message",
			`{"surfaceOp":"append","source":{"kind":"user"},"content":[{"type":"text","text":"第一条"}]}`),
		rawFrame(2, "assistant/message", `{"turn":1,"step":1,"content":[{"type":"text","text":"第二条"}]}`),
		rawFrame(3, "assistant/message", `{"turn":1,"step":2,"content":[{"type":"text","text":"第三条"}]}`),
	}
	renderSeat(t, p, pluginapi.Snapshot{Cur: "s-1", Frames: frames, JournalEpoch: 1}, "official-transcript")
	entries := p.TranscriptEntries()
	if len(entries) != 3 {
		t.Fatalf("补洞后 %d 行，想要 3", len(entries))
	}
	for i, want := range []string{"第一条", "第二条", "第三条"} {
		if entries[i].Data.Text != want {
			t.Fatalf("第 %d 行 = %q，想要 %q（行序必须 = journal seq 顺序）", i, entries[i].Data.Text, want)
		}
	}
}

// ── ⑤ 切会话 ──────────────────────────────────────────────────────────

// TestSessionSwitchResetsProjection：切会话必须 Reset。
//
// 两条会话的 journal seq 都从 1 起；不 Reset，新会话的帧会被当成"重放"丢掉，
// 界面症状是"切过去主区空白"。
func TestSessionSwitchResetsProjection(t *testing.T) {
	p := productPlugin()
	renderSeat(t, p, pluginapi.Snapshot{Cur: "s-a", Frames: realFrames(), JournalEpoch: 1}, "official-transcript")
	if p.transcriptSession != "s-a" {
		t.Fatalf("transcriptSession = %q，想要 s-a", p.transcriptSession)
	}

	other := []apiclient.Frame{
		rawFrame(1, "user/message",
			`{"surfaceOp":"append","source":{"kind":"user"},"content":[{"type":"text","text":"B 会话的消息"}]}`),
	}
	canvas := renderSeat(t, p, pluginapi.Snapshot{Cur: "s-b", Frames: other, JournalEpoch: 1}, "official-transcript")
	if !hasCanvasText(canvas, "B 会话的消息") {
		t.Fatalf("切到 B 之后没画 B 的消息（seq 被当成重放）；文本：%v", canvasValues(canvas))
	}
	if hasCanvasText(canvas, "真实用户消息") {
		t.Fatal("切到 B 之后 A 的消息还留着（跨会话串行）")
	}
}

// ── ⑥ 控制帧 / Transport 终局帧 ────────────────────────────────────────

// TestControlFramesProduceNoRows：审批帧与 pong 不是消息行。
//
// 官方审批由 composer 接管（conversation.go 的 Panels 注释：同屏两条
// 「需要审批」是明确要避免的）。落成 unknown 行会让主区每轮多出几行原始 JSON。
func TestControlFramesProduceNoRows(t *testing.T) {
	p := productPlugin()
	frames := []apiclient.Frame{
		rawFrame(1, "pong", "{}"),
		rawFrame(2, "approval/request", `{"requestId":"req-1","toolCall":{"title":"bash: ls"}}`),
		rawFrame(3, "user/message",
			`{"surfaceOp":"append","source":{"kind":"user"},"content":[{"type":"text","text":"只有这条是消息"}]}`),
	}
	renderSeat(t, p, pluginapi.Snapshot{Cur: "s-1", Frames: frames, JournalEpoch: 1}, "official-transcript")
	if got := len(p.transcriptRowKeys()); got != 1 {
		t.Fatalf("挂载 %d 行，想要 1（审批帧/pong 不是消息行）；keys=%v", got, p.transcriptRowKeys())
	}
}

// TestMetadataFramesProduceNoRows 钉住"**非 surface 帧不是消息行**"。
//
// 真实宿主一轮会推 `request/context`、`step/end`、`turn/done`、
// `session/title-llm-request` 这些**元数据**帧。它们没有 `surfaceOp`、没有
// content 块——落成 unknown 行的话，主区每轮会多出几行原始 JSON，
// 而用户完全不知道那是什么。
func TestMetadataFramesProduceNoRows(t *testing.T) {
	p := productPlugin()
	frames := []apiclient.Frame{
		rawFrame(1, "request/context", `{"model":"group/deepseek-v4.1-flash-1017"}`),
		rawFrame(2, "step/end", `{"step":1,"turn":1,"v4Seq":8}`),
		rawFrame(3, "turn/done", `{"turnId":"turn-1"}`),
		rawFrame(4, "turn/error", `{"turnId":"turn-2","error":"模型调用失败：timeout"}`),
		rawFrame(5, "session/title-llm-request", `{"titleProvider":"session-title-first-prompt-llm"}`),
		rawFrame(6, "usage/update", `{"used":10,"size":1000}`),
	}
	renderSeat(t, p, pluginapi.Snapshot{Cur: "s-1", Frames: frames, JournalEpoch: 1}, "official-transcript")
	if got := p.transcriptRowKeys(); len(got) != 0 {
		t.Fatalf("元数据帧落了 %d 行消息（%v），想要 0", len(got), got)
	}
}

// TestAssistantStepMergesPerTurnStep 钉住官方 start/update 状态机。
//
// 真实宿主一步推：step/start → assistant/text×N（增量）→ assistant/message
// （完整产物 + usage）→ step/end。按 seq 一帧一节点的话，界面上一步回答会
// 变成 4~5 行（前几行空、正文重复两遍）。
func TestAssistantStepMergesPerTurnStep(t *testing.T) {
	p := productPlugin()
	frames := []apiclient.Frame{
		rawFrame(1, "step/start", `{"step":1,"turn":1,"v4Seq":5}`),
		rawFrame(2, "assistant/text", `{"text":"GT3-REAL-","turn":1,"step":1,"v4Seq":6}`),
		rawFrame(3, "assistant/text", `{"text":"FRAME-OK","turn":1,"step":1,"v4Seq":7}`),
		rawFrame(4, "assistant/message",
			`{"turn":1,"step":1,"content":[{"type":"text","text":"GT3-REAL-FRAME-OK"}],"usage":{"inputTokens":6288,"outputTokens":11,"totalTokens":6299}}`),
		rawFrame(5, "step/end", `{"step":1,"turn":1,"v4Seq":8}`),
	}
	canvas := renderSeat(t, p, pluginapi.Snapshot{Cur: "s-1", Frames: frames, JournalEpoch: 1}, "official-transcript")
	if got := len(p.transcriptRowKeys()); got != 1 {
		t.Fatalf("一步回答落了 %d 行，想要 1（按 (turn,step) 合键）", got)
	}
	entries := p.TranscriptEntries()
	if entries[0].Data.Text != "GT3-REAL-FRAME-OK" {
		t.Fatalf("合并后正文 = %q，想要一步完整产物（不是两段增量拼错）", entries[0].Data.Text)
	}
	if !entries[0].Data.Usage.Known() {
		t.Fatalf("assistant/message 的用量丢了：%+v", entries[0].Data.Usage)
	}
	if !hasCanvasText(canvas, "GT3-REAL-FRAME-OK") {
		t.Fatalf("合并后的正文没画出来：%v", canvasValues(canvas))
	}
}
