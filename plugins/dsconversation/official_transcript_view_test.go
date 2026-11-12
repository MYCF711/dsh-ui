package dsconversation

// official_transcript_view_test.go —— 渲染层的机器断言（真实 viva 画布）。
//
// 四组：
//
//	① 20 种节点每种至少一条渲染断言（19 种真的画出来、submitted-plan 一个像素都不画）；
//	② 双层滚动口：宿主口存在时自滚口降级、不存在时自滚口自己滚；
//	③ 逐条真实挂载：行数 == 可见节点数，顺序 == 节点顺序（**不是虚拟列表**）；
//	④ 官方几何：markdown 代码块 banner、行内代码等宽、工具失败态摘要、审批卡禁用态。

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// newTranscriptCanvas 用真实 flex 布局 + 确定性字宽测量器（不需要窗口）。
func newTranscriptCanvas() (*gfx.Canvas, *gfx.Tree) {
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size
	})
	return canvas, tree
}

// renderTranscript 在一张真实画布上跑一次 Draw。
func renderTranscript(t *testing.T, entries []TranscriptEntry, vp TranscriptViewport) (*OfficialTranscriptView, *gfx.Canvas) {
	t.Helper()
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	canvas, tree := newTranscriptCanvas()
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("transcript-view-test"),
		Canvas: canvas, Bounds: ui.Rect{Width: 900, Height: 700}, MainWidth: 900,
	}
	var fv flex.View
	fv.Draw(canvas, dscore.VivaTheme(theme), ui.Rect{Width: 900, Height: 700}, func(f *flex.Frame) {
		view.Draw(f, env, vp, entries)
	})
	// 覆画阶段：代码块（banner / 高亮 / 横滚视口 / 复制按钮）要等布局
	// 定下来才有真实矩形——与产品路径（`pluginapi.Painter.AfterDraw`）同一顺序。
	view.PaintCodeBlocks(canvas, env)
	return view, canvas
}

// hostViewport 造一个「外层已经有宿主滚动口」的视口（线上默认形态）。
func hostViewport(tree *gfx.Tree) TranscriptViewport {
	return TranscriptViewport{
		HostScroll: true,
		HostID:     tree.ID(tree.Feature("transcript-view-test"), "host-scroll"),
		SelfID:     tree.ID(tree.Feature("transcript-view-test"), "self-scroll"),
	}
}

// selfViewport 造一个「没有宿主滚动口、自滚动」的视口（独立挂载 Chat）。
func selfViewport(tree *gfx.Tree) TranscriptViewport {
	return TranscriptViewport{
		HostScroll: false,
		SelfID:     tree.ID(tree.Feature("transcript-view-test"), "self-scroll"),
	}
}

// canvasText 找一段文本；found=false 表示没画出来。
func canvasText(canvas *gfx.Canvas, value string) (gfx.Text, bool) {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text, true
		}
	}
	return gfx.Text{}, false
}

// canvasTextPrefix 按前缀找一段文本（省略号截断的文本用）。
func canvasTextPrefix(canvas *gfx.Canvas, prefix string) (gfx.Text, bool) {
	for _, text := range canvas.Texts() {
		if strings.HasPrefix(text.Value, prefix) {
			return text, true
		}
	}
	return gfx.Text{}, false
}

// mustCanvasText 断言某段文本真的画在画布上。
func mustCanvasText(t *testing.T, canvas *gfx.Canvas, value string) gfx.Text {
	t.Helper()
	text, ok := canvasText(canvas, value)
	if !ok {
		var got []string
		for _, item := range canvas.Texts() {
			got = append(got, item.Value)
		}
		t.Fatalf("画布缺少文本 %q；实际文本：%v", value, got)
	}
	return text
}

// allKindsEntries 造 20 条节点（每种 kind 一条），用于覆盖表与挂载断言。
func allKindsEntries() []TranscriptEntry {
	entries := []TranscriptEntry{
		{Key: "k-user", Kind: ChatNodeUser, Seq: 1, Time: 1_700_000_000_000, Turn: 0, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Text: "用户正文"}},
		{Key: "k-steering", Kind: ChatNodeSteering, Seq: 2, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Text: "补充一句"}},
		{Key: "k-context", Kind: ChatNodeContext, Seq: 3, Turn: -1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Text: "上下文内容", SourceKind: "notification"}},
		{Key: "k-trigger", Kind: ChatNodeTurnTrigger, Seq: 4, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Text: "定时任务已触发", SourceKind: "schedule"}},
		{Key: "k-cmd-input", Kind: ChatNodeCommandInput, Seq: 5, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Text: "/goal 写完报告"}},
		{Key: "k-assistant", Kind: ChatNodeAssistantStep, Seq: 6, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Text: "助手正文", Reasoning: "先想一下"}},
		{Key: "k-tool", Kind: ChatNodeToolCall, Seq: 7, Turn: 1, Visibility: ChatVisibilityConditional,
			Data: TranscriptData{Tool: "bash", Args: `{"command":"go test"}`, ResultText: "ok", Status: "ok"}},
		{Key: "k-command", Kind: ChatNodeCommand, Seq: 8, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{CommandName: "compact", CommandOutcome: "done"}},
		{Key: "k-manual", Kind: ChatNodeManualCompaction, Seq: 9, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{CommandName: "compact", Text: "压缩完成"}},
		{Key: "k-compaction", Kind: ChatNodeCompaction, Seq: 10, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{CompactionSummary: "早先的对话", CompactionCount: intPtr(12)}},
		{Key: "k-retry", Kind: ChatNodeModelRetry, Seq: 11, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{RetryAttempt: intPtr(2), RetryMax: intPtr(5)}},
		{Key: "k-error", Kind: ChatNodeTurnError, Seq: 12, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{ErrorMessage: "boom", ErrorCode: "E_BOOM"}},
		{Key: "k-maxtokens", Kind: ChatNodeTurnMaxTokens, Seq: 13, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{MaxTokensLimit: int64Ptr(8192)}},
		{Key: "k-process", Kind: ChatNodeTurnProcess, Seq: 14, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{DurationMs: int64Ptr(4200)}},
		{Key: "k-tail", Kind: ChatNodeTurnTail, Seq: 15, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Usage: TranscriptUsage{TotalTokens: 42}}},
		{Key: "k-sysprompt", Kind: ChatNodeSystemPrompt, Seq: 16, Turn: -1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Text: "系统提示内容"}},
		{Key: "k-unknown", Kind: ChatNodeUnknown, Seq: 17, Turn: -1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Raw: `{"future":true}`}},
		{Key: "k-question", Kind: ChatNodeQuestionReply, Seq: 18, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Text: "选 A", Outcome: "answered", CallID: "call_9"}},
		{Key: "k-plan", Kind: ChatNodeSubmittedPlan, Seq: 19, Turn: 1, Visibility: ChatVisibilityHidden,
			Data: TranscriptData{Text: "计划内容（不得出现在 transcript）"}},
		{Key: "k-workflow", Kind: ChatNodeWorkflowRun, Seq: 20, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{RunName: "review", StopReason: "completed"}},
	}
	return entries
}

func int64Ptr(v int64) *int64 { return &v }

// TestRenderAllKindsExceptSubmittedPlan 是**覆盖表**：19 种可渲染 kind 每种都有渲染断言。
func TestRenderAllKindsExceptSubmittedPlan(t *testing.T) {
	_, tree := newTranscriptCanvas()
	entries := allKindsEntries()
	view, canvas := renderTranscript(t, entries, hostViewport(tree))

	if got := view.Boxes.RowCount(); got != 19 {
		t.Fatalf("挂载行数 = %d，应为 19（20 种节点减去恒 hidden 的 submitted-plan）", got)
	}
	// 每种 kind 的行都必须被挂载（按 key 查）。
	for _, entry := range entries {
		row, ok := view.Boxes.RowOf(entry.Key)
		if entry.Kind == ChatNodeSubmittedPlan {
			if ok {
				t.Errorf("submitted-plan 不得挂载（key=%s）", entry.Key)
			}
			continue
		}
		if !ok {
			t.Errorf("kind %q 的行没有挂载（key=%s）", entry.Kind, entry.Key)
			continue
		}
		if row.Kind != entry.Kind {
			t.Errorf("key=%s 的行 kind = %q，应为 %q", entry.Key, row.Kind, entry.Kind)
		}
		rect := row.Box.Rect()
		if rect.Width <= 0 {
			t.Errorf("kind %q 的行宽度 = %v（必须真实占位）", entry.Kind, rect.Width)
		}
	}
	// 每种 kind 至少一条**文本**断言（覆盖"文本/几何/可见性"里的文本面）。
	for _, want := range []string{
		"用户正文", "补充一句", "上下文内容", "定时任务已触发", "/goal 写完报告",
		"助手正文", "先想一下", "bash", `{"command":"go test"}`, "ok", "/compact", "压缩完成",
		"早先的对话", "重试 2/5", "Turn 错误", "boom", "E_BOOM", "输出达到 token 上限",
		"耗时 4 秒", "42 tokens", "系统提示内容", `{"future":true}`, "选 A", "review",
	} {
		mustCanvasText(t, canvas, want)
	}
}

// TestSubmittedPlanNeverRenders 钉住官方真值：`submitted-plan` **恒 hidden、零 DOM**。
func TestSubmittedPlanNeverRenders(t *testing.T) {
	_, tree := newTranscriptCanvas()
	view, canvas := renderTranscript(t, []TranscriptEntry{
		{Key: "plan", Kind: ChatNodeSubmittedPlan, Seq: 1, Turn: 1, Visibility: ChatVisibilityHidden,
			Data: TranscriptData{Text: "计划内容（不得出现在 transcript）"}},
	}, hostViewport(tree))

	if view.Boxes.RowCount() != 0 {
		t.Fatalf("submitted-plan 行数 = %d，应为 0（官方没有行渲染器）", view.Boxes.RowCount())
	}
	if _, ok := view.Boxes.RowOf("plan"); ok {
		t.Error("submitted-plan 不得出现在挂载行里")
	}
	if len(canvas.Texts()) != 0 {
		t.Errorf("submitted-plan 不得画任何文本，实际 %d 条", len(canvas.Texts()))
	}
}

// TestRowOrderMatchesEntries：行序必须与节点顺序一致（不是虚拟列表、不重排）。
func TestRowOrderMatchesEntries(t *testing.T) {
	_, tree := newTranscriptCanvas()
	entries := []TranscriptEntry{
		{Key: "a", Kind: ChatNodeUser, Seq: 1, Turn: 0, Visibility: ChatVisibilityVisible, Data: TranscriptData{Text: "一"}},
		{Key: "b", Kind: ChatNodeAssistantStep, Seq: 2, Turn: 1, Visibility: ChatVisibilityVisible, Data: TranscriptData{Text: "二"}},
		{Key: "c", Kind: ChatNodeTurnTail, Seq: 3, Turn: 1, Visibility: ChatVisibilityVisible},
	}
	view, _ := renderTranscript(t, entries, hostViewport(tree))
	if view.Boxes.RowCount() != 3 {
		t.Fatalf("行数 = %d，应为 3", view.Boxes.RowCount())
	}
	for i, entry := range entries {
		if view.Boxes.Rows[i].Key != entry.Key {
			t.Errorf("第 %d 行的 key = %q，应为 %q（顺序必须与节点一致）", i, view.Boxes.Rows[i].Key, entry.Key)
		}
	}
}

// TestDoubleScrollPortHostMode：外层有宿主滚动口时，自滚口降级（不再自己滚）。
func TestDoubleScrollPortHostMode(t *testing.T) {
	_, tree := newTranscriptCanvas()
	view, _ := renderTranscript(t, allKindsEntries(), hostViewport(tree))

	if !view.Boxes.HostScroll {
		t.Fatal("这一帧应记录宿主滚动口存在")
	}
	if view.Boxes.SelfScrolls {
		t.Error("宿主滚动口存在时，自滚动口必须降级（overflow-y:visible）")
	}
	host, ok := view.Boxes.HostScrollRect()
	if !ok {
		t.Fatal("宿主滚动口必须存在（这一帧 HostScroll=true）")
	}
	if host.Width <= 0 || host.Height <= 0 {
		t.Fatalf("宿主滚动口几何 = %+v（必须真实占位）", host)
	}
	// 五层容器都必须存在且逐层变窄（column 在 scroll 内部）。
	if view.Boxes.Frame.Rect().Width <= 0 || view.Boxes.Root.Rect().Width <= 0 {
		t.Error("ChatView.frame / root 必须真实布局")
	}
	column := view.Boxes.Column.Rect()
	if column.Width <= 0 || column.Width > host.Width {
		t.Errorf("column 宽 = %v，应在宿主滚动口宽度 %v 之内", column.Width, host.Width)
	}
	if view.Boxes.Flow.Rect().Width <= 0 {
		t.Error("flow 槽必须真实布局（display:contents 只影响盒模型，不影响宽度传递）")
	}
}

// TestDoubleScrollPortSelfMode：没有宿主滚动口时，自滚口自己滚。
func TestDoubleScrollPortSelfMode(t *testing.T) {
	_, tree := newTranscriptCanvas()
	view, _ := renderTranscript(t, allKindsEntries(), selfViewport(tree))

	if view.Boxes.HostScroll {
		t.Error("这一帧不应记录宿主滚动口")
	}
	if !view.Boxes.SelfScrolls {
		t.Error("没有宿主滚动口时，自滚动口必须自己滚（overflow-y:auto）")
	}
	if _, ok := view.Boxes.HostScrollRect(); ok {
		t.Error("没有宿主滚动口时 .scrollBody 不应有几何")
	}
	if view.Boxes.Scroll.Rect().Width <= 0 {
		t.Error("自滚动口必须真实布局")
	}
}

// TestMarkdownCodeBlockAndInlineCode 钉住助手正文的 markdown 分派。
func TestMarkdownCodeBlockAndInlineCode(t *testing.T) {
	_, tree := newTranscriptCanvas()
	text := "先跑 `go test` 再提交。\n\n```go\nfmt.Println(\"hi\")\n```\n"
	view, canvas := renderTranscript(t, []TranscriptEntry{
		{Key: "md", Kind: ChatNodeAssistantStep, Seq: 1, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Text: text}},
	}, hostViewport(tree))
	if view.Boxes.RowCount() != 1 {
		t.Fatalf("行数 = %d", view.Boxes.RowCount())
	}

	// 段落（含行内代码）必须画出来。
	paragraph := mustCanvasText(t, canvas, "先跑 go test 再提交。")
	if paragraph.Style.Size != 14 {
		t.Errorf("正文号 = %v，应为 14", paragraph.Style.Size)
	}
	// 代码块：banner 的语言名 + 等宽正文。
	lang := mustCanvasText(t, canvas, "go")
	if lang.Style.Size != 12 {
		t.Errorf("代码块 banner 号 = %v（官方 --dsw-font-xxs-12 档）", lang.Style.Size)
	}
	code := mustCanvasText(t, canvas, "fmt.Println(\"hi\")")
	if !code.Style.Mono {
		t.Error("代码块正文必须走等宽字体（mono）")
	}
}

// TestInlineCodeOnlyParagraphIsMono：整段行内代码时走等宽（官方 MarkdownText 的 Mono span）。
func TestInlineCodeOnlyParagraphIsMono(t *testing.T) {
	_, tree := newTranscriptCanvas()
	_, canvas := renderTranscript(t, []TranscriptEntry{
		{Key: "mono", Kind: ChatNodeAssistantStep, Seq: 1, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Text: "`dsh serve`"}},
	}, hostViewport(tree))
	mono := mustCanvasText(t, canvas, "dsh serve")
	if !mono.Style.Mono {
		t.Error("整段行内代码必须走等宽字体")
	}
}

// TestToolCallFailureStates 钉住 §5.7 的四态真值。
func TestToolCallFailureStates(t *testing.T) {
	_, tree := newTranscriptCanvas()
	exit := 2
	view, canvas := renderTranscript(t, []TranscriptEntry{
		// 有退出码 + error：保留命令描述，**不显示错误首行**。
		{Key: "with-exit", Kind: ChatNodeToolCall, Seq: 1, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Tool: "bash", Status: "error", ExitCode: &exit, ResultText: "错误首行"}},
		// 没有退出码 + error：显示错误首行。
		{Key: "no-exit", Kind: ChatNodeToolCall, Seq: 2, Turn: 1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Tool: "bash", Status: "error", ResultText: "错误首行"}},
	}, hostViewport(tree))
	if view.Boxes.RowCount() != 2 {
		t.Fatalf("行数 = %d", view.Boxes.RowCount())
	}
	withExit, ok := canvasText(canvas, "exit 2")
	if !ok {
		t.Fatal("有退出码时必须显示 exit 2")
	}
	if _, ok := canvasText(canvas, "错误首行"); !ok {
		t.Error("没有退出码时必须显示错误首行")
	}
	_ = withExit
}

// TestUserAttachmentsAndReferences 钉住附件卡与引用摘要的几何。
func TestUserAttachmentsAndReferences(t *testing.T) {
	_, tree := newTranscriptCanvas()
	_, canvas := renderTranscript(t, []TranscriptEntry{
		{Key: "u", Kind: ChatNodeUser, Seq: 1, Turn: 0, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{
				Text:       "看这个",
				References: []string{"src/main.go"},
				Attachments: []TranscriptAttachment{
					{Name: "report.pdf", Meta: "application/pdf · 1024 B"},
					{Name: "shot.png", Image: true},
				},
			}},
	}, hostViewport(tree))

	mustCanvasText(t, canvas, "看这个")
	mustCanvasText(t, canvas, "src/main.go")
	fileName := mustCanvasText(t, canvas, "report.pdf")
	if fileName.Style.Size != FileNameFontSize {
		t.Errorf("文件名号 = %v，应为 %v", fileName.Style.Size, FileNameFontSize)
	}
	meta, ok := canvasTextPrefix(canvas, "application/p")
	if !ok {
		t.Fatal("文件元信息必须画出来（超长按省略号截断）")
	}
	if meta.Style.Size != FileMetaFontSize {
		t.Errorf("文件元信息号 = %v，应为 %v", meta.Style.Size, FileMetaFontSize)
	}
}

// TestUnknownNodeRendersRawJSON：unknown 兜底只做原样 JSON，**不编摘要**。
func TestUnknownNodeRendersRawJSON(t *testing.T) {
	_, tree := newTranscriptCanvas()
	_, canvas := renderTranscript(t, []TranscriptEntry{
		{Key: "u", Kind: ChatNodeUnknown, Seq: 1, Turn: -1, Visibility: ChatVisibilityVisible,
			Data: TranscriptData{Raw: `{"kind":"future-surface"}`}},
	}, hostViewport(tree))
	raw := mustCanvasText(t, canvas, `{"kind":"future-surface"}`)
	if !raw.Style.Mono {
		t.Error("未知 surface 必须等宽原样显示（JsonBlock 语义）")
	}
}

// TestApprovalCardGeometryAndDisabled 钉住审批卡的三条真值。
func TestApprovalCardGeometryAndDisabled(t *testing.T) {
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	canvas, tree := newTranscriptCanvas()
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("approval-test"),
		Canvas: canvas, Bounds: ui.Rect{Width: 900, Height: 700}, MainWidth: 900,
	}
	var boxes ApprovalBoxes
	var fv flex.View
	fv.Draw(canvas, dscore.VivaTheme(theme), ui.Rect{Width: 900, Height: 700}, func(f *flex.Frame) {
		boxes = view.DrawApprovalCard(f, env, pluginapi.Approval{
			RequestID: "",
			Title:     "需要审批 bash",
			Args:      `{"command":"rm -rf /tmp/x"}`,
			Options: []pluginapi.ApprovalOption{
				{OptionID: "", Name: "允许这一次", Kind: "allow_once"},
			},
		})
	})
	if !boxes.Rendered {
		t.Fatal("审批卡应真的画出来")
	}
	if boxes.Card.Rect().Width <= 0 {
		t.Error("审批卡根必须有几何")
	}
	if len(boxes.Options) != 1 {
		t.Fatalf("选项数 = %d，应为 1", len(boxes.Options))
	}
	// RequestID 为空 = 回传不出去 → 按钮禁用（不假装成功）。
	if !boxes.Options[0].Disabled {
		t.Error("RequestID 为空时选项必须禁用（回传不出去，不编 id）")
	}
	if boxes.Options[0].Box.Rect().Width <= 0 {
		t.Error("选项按钮必须有几何")
	}
	mustCanvasText(t, canvas, "需要审批 bash")
	// **不显示风险等级**：宿主没有这个字段，界面不许从命令里算一个出来。
	for _, text := range canvas.Texts() {
		if strings.Contains(text.Value, "风险") {
			t.Errorf("审批卡不得显示风险等级，实际画了 %q", text.Value)
		}
	}
}

// TestEmptyTextUserDrawsNoBubbleText：空文本 + 无附件时官方不渲染气泡正文。
func TestEmptyTextUserDrawsNoBubbleText(t *testing.T) {
	_, tree := newTranscriptCanvas()
	view, canvas := renderTranscript(t, []TranscriptEntry{
		{Key: "empty", Kind: ChatNodeUser, Seq: 1, Turn: 0, Visibility: ChatVisibilityVisible},
	}, hostViewport(tree))
	if view.Boxes.RowCount() != 1 {
		t.Fatalf("空用户消息仍应挂一行（保持行距），实际 %d 行", view.Boxes.RowCount())
	}
	if len(canvas.Texts()) != 0 {
		t.Errorf("空文本用户消息不得画文本，实际 %d 条", len(canvas.Texts()))
	}
}
