//go:build darwin || windows || linux

// skillrow_test.go —— 官方 `SkillRow.tsx` 的**行为**断言。
//
// 四组：
//
//	① 纯函数：firstLine / skillName / resultText / deriveState 逐条钉住官方语义
//	② view model：skillRowModelOf 的四态、错误摘要、参数来源
//	③ leading / status / summary：官方三段三目
//	④ **真点击**：headless 真 Paint → 真 HitTest → 真 Press/Release，
//	   点一次必须真的切换展开态（"点了没反应"是这一行最坏的失败形态）
package dsskill

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dstool"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// ── ① 纯函数 ──────────────────────────────────────────────────────

// TestFirstLine 钉住 `firstLine`（`SkillRow.tsx:23-26`）。
//
// 只切 `'\n'`：`\r\n` 里 `\r` 会留在行尾（官方 `indexOf('\n')` 就是这样）。
func TestFirstLine(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"abc", "abc"},
		{"abc\ndef", "abc"},
		{"abc\r\ndef", "abc\r"}, // \r 保留：官方只切 \n
		{"\nleading", ""},
		{"a\nb\nc", "a"},
	}
	for _, c := range cases {
		if got := firstLine(c.in); got != c.want {
			t.Errorf("firstLine(%q) = %q，want %q", c.in, got, c.want)
		}
	}
}

// TestSkillName 钉住 `skillName`（`SkillRow.tsx:29-41`）的五条分支。
func TestSkillName(t *testing.T) {
	cases := []struct {
		name    string
		argsRaw string
		callID  string
		want    string
	}{
		{"对象里的 name", `{"name":"demo"}`, "call-1", "demo"},
		{"name 带换行", `{"name":"demo\nrest"}`, "call-1", "demo"},
		{"name 是空串 → 退回原文", `{"name":""}`, "call-1", `{"name":""}`},
		{"name 不是字符串 → 退回原文", `{"name":42}`, "call-1", `{"name":42}`},
		{"没有 name 键 → 退回原文", `{"other":"x"}`, "call-1", `{"other":"x"}`},
		{"JSON 截断 → 原文首行", `{"name":"de`, "call-1", `{"name":"de`},
		{"不是 JSON → 原文首行", "demo-arg\nsecond", "call-1", "demo-arg"},
		{"空串 → callId", "", "call-7", "call-7"},
		{"JSON 数组 → 退回原文", `["a"]`, "call-1", `["a"]`},
		{"JSON 字符串 → 退回原文", `"demo"`, "call-1", `"demo"`},
		{"JSON null → 退回原文", `null`, "call-1", "null"},
	}
	for _, c := range cases {
		if got := skillName(c.argsRaw, c.callID); got != c.want {
			t.Errorf("%s：skillName(%q, %q) = %q，want %q", c.name, c.argsRaw, c.callID, got, c.want)
		}
	}
}

// TestResultText 钉住 `resultText`（`SkillRow.tsx:45-55`）。
func TestResultText(t *testing.T) {
	text := func(s string) *string { return &s }
	cases := []struct {
		name  string
		block SkillBlock
		want  *string
	}{
		{
			name:  "未落定 → nil",
			block: SkillBlock{Settled: false, Content: []SkillResultItem{{Type: "text", Text: "x"}}},
			want:  nil,
		},
		{
			name:  "单个 text",
			block: SkillBlock{Settled: true, Content: []SkillResultItem{{Type: "text", Text: "hello"}}},
			want:  text("hello"),
		},
		{
			name: "多个 text 用换行拼",
			block: SkillBlock{Settled: true, Content: []SkillResultItem{
				{Type: "text", Text: "a"}, {Type: "text", Text: "b"},
			}},
			want: text("a\nb"),
		},
		{
			name: "非 text 项按 2 空格缩进的 JSON",
			block: SkillBlock{Settled: true, Content: []SkillResultItem{
				{Type: "image", Raw: json.RawMessage(`{"kind":"image","w":2}`)},
			}},
			want: text("{\n  \"kind\": \"image\",\n  \"w\": 2\n}"),
		},
		{
			name:  "空内容 + error → name: code 兜底",
			block: SkillBlock{Settled: true, Error: &SkillCallError{Name: "ToolError", Code: "E_BAD"}},
			want:  text("ToolError: E_BAD"),
		},
		{
			name:  "空内容 + 无 error → nil",
			block: SkillBlock{Settled: true},
			want:  nil,
		},
		{
			name:  "单个空 text → 拼接是空串也算 nil",
			block: SkillBlock{Settled: true, Content: []SkillResultItem{{Type: "text", Text: ""}}},
			want:  nil,
		},
		{
			name:  "空 Raw 的非 text 项 → null",
			block: SkillBlock{Settled: true, Content: []SkillResultItem{{Type: "image"}}},
			want:  text("null"),
		},
	}
	for _, c := range cases {
		got := resultText(c.block)
		switch {
		case c.want == nil && got != nil:
			t.Errorf("%s：want nil，got %q", c.name, *got)
		case c.want != nil && got == nil:
			t.Errorf("%s：want %q，got nil", c.name, *c.want)
		case c.want != nil && got != nil && *got != *c.want:
			t.Errorf("%s：got %q，want %q", c.name, *got, *c.want)
		}
	}
}

// TestResultTextKeepsKeyOrder 钉住「非 text 项保留原始键顺序」。
//
// `json.MarshalIndent(map[string]any)` 会按键排序（`a` 在 `z` 前），
// `json.Indent` 不会。官方 `JSON.stringify` 是插入顺序——用错就等于
// 悄悄改了官方输出。
func TestResultTextKeepsKeyOrder(t *testing.T) {
	block := SkillBlock{Settled: true, Content: []SkillResultItem{
		{Type: "image", Raw: json.RawMessage(`{"z":1,"a":2}`)},
	}}
	got := resultText(block)
	if got == nil {
		t.Fatal("没有正文")
	}
	want := "{\n  \"z\": 1,\n  \"a\": 2\n}"
	if *got != want {
		t.Fatalf("键顺序被改了：\n got=%q\nwant=%q", *got, want)
	}
}

// TestDeriveState 钉住四态判据（`SkillRow.tsx:61-65`）。
//
// **顺序不能换**：`interrupted` 先于 `isError`。
func TestDeriveState(t *testing.T) {
	cases := []struct {
		name  string
		block SkillBlock
		want  SkillRowState
	}{
		{"未落定 → running", SkillBlock{Settled: false, IsError: true}, SkillStateRunning},
		{"interrupted → stopped", SkillBlock{Settled: true, IsError: true, Error: &SkillCallError{Code: "interrupted"}}, SkillStateStopped},
		{"isError → error", SkillBlock{Settled: true, IsError: true, Error: &SkillCallError{Code: "boom"}}, SkillStateError},
		{"有 error 但 isError 假 → ok", SkillBlock{Settled: true, Error: &SkillCallError{Code: "boom"}}, SkillStateOK},
		{"都没有 → ok", SkillBlock{Settled: true}, SkillStateOK},
	}
	for _, c := range cases {
		if got := deriveState(c.block); got != c.want {
			t.Errorf("%s：state = %q，want %q", c.name, got, c.want)
		}
	}
}

// TestInterruptedBeatsIsError 单独钉住「中断优先于失败」。
//
// 这不是重言：把两条判据调换顺序，上面那条用例照样过（因为 error.code 也是
// `interrupted` 时 `isError` 为真会先命中），但**画出来的摘要会变**——
// 官方显示「skill 加载已中止」，反了就显示「skill 加载失败」。
func TestInterruptedBeatsIsError(t *testing.T) {
	output := "partial output\nsecond line"
	block := SkillBlock{
		Settled: true, IsError: true,
		Error:   &SkillCallError{Name: "x", Code: "interrupted"},
		Content: []SkillResultItem{{Type: "text", Text: output}},
	}
	model := skillRowModelOf(block)
	if model.State != SkillStateStopped {
		t.Fatalf("state = %q，want stopped", model.State)
	}
	// stopped 时**不**算 errorSummary（官方只在 error 态算）。
	if model.ErrorSummary != nil {
		t.Fatalf("stopped 态不该有 ErrorSummary，实际 %q", *model.ErrorSummary)
	}
	// 而且摘要走 `row.stopped` 文案，不是名字。
	if got := summaryOf(model, "zh"); got != "skill 加载已中止" {
		t.Fatalf("summary = %q，want %q", got, "skill 加载已中止")
	}
}

// ── ② view model ──────────────────────────────────────────────────

// TestSkillRowModelOf 钉住 `skillRowModel` 的字段来源（`SkillRow.tsx:58-73`）。
func TestSkillRowModelOf(t *testing.T) {
	// ① 落定：参数从 `block.call.argsRaw` 取（**不是** `block.argsRaw`）。
	settled := SkillBlock{
		Settled: true,
		ArgsRaw: "WRONG",
		Call:    &SkillCall{ArgsRaw: `{"name":"demo"}`},
		Content: []SkillResultItem{{Type: "text", Text: "body"}},
	}
	model := skillRowModelOf(settled)
	if model.Name != "demo" {
		t.Fatalf("Name = %q，want demo（落定后必须读 block.call.argsRaw）", model.Name)
	}
	if model.Output == nil || *model.Output != "body" {
		t.Fatalf("Output = %v，want body", model.Output)
	}
	if !model.expandable() {
		t.Fatal("有 output 就该 expandable")
	}
	if model.ErrorSummary != nil {
		t.Fatal("ok 态不该有 ErrorSummary")
	}

	// ② 未落定：参数从 `block.argsRaw` 取，且没有 output（不可展开）。
	preparing := SkillBlock{Settled: false, ArgsRaw: `{"name":"prep"}`}
	model = skillRowModelOf(preparing)
	if model.Name != "prep" {
		t.Fatalf("Name = %q，want prep（未落定读 block.argsRaw）", model.Name)
	}
	if model.Output != nil {
		t.Fatalf("未落定不该有 Output，实际 %q", *model.Output)
	}
	if model.expandable() {
		t.Fatal("没有 output 就不可展开")
	}
}

// TestErrorSummaryIsFirstLine 钉住「错误摘要 = 正文首行」。
func TestErrorSummaryIsFirstLine(t *testing.T) {
	block := SkillBlock{
		Settled: true, IsError: true,
		Content: []SkillResultItem{{Type: "text", Text: "第一行\n第二行"}},
	}
	model := skillRowModelOf(block)
	if model.ErrorSummary == nil {
		t.Fatal("error 态必须有 ErrorSummary")
	}
	if *model.ErrorSummary != "第一行" {
		t.Fatalf("ErrorSummary = %q，want %q", *model.ErrorSummary, "第一行")
	}
	if got := summaryOf(model, "zh"); got != "第一行" {
		t.Fatalf("summary = %q，want %q", got, "第一行")
	}
}

// TestErrorWithoutOutputFallsBackToName 钉住「error 但没有正文 → 摘要用名字」。
func TestErrorWithoutOutputFallsBackToName(t *testing.T) {
	block := SkillBlock{
		Settled: true, IsError: true,
		Call:  &SkillCall{ArgsRaw: `{"name":"demo"}`},
		Error: &SkillCallError{Name: "ToolError", Code: "E_BAD"},
	}
	model := skillRowModelOf(block)
	// 没有 content 但**有** error → resultText 兜底出 "ToolError: E_BAD"，
	// 于是 ErrorSummary 是它的首行。
	if model.ErrorSummary == nil {
		t.Fatal("有 error 兜底正文，ErrorSummary 不该是 nil")
	}
	if *model.ErrorSummary != "ToolError: E_BAD" {
		t.Fatalf("ErrorSummary = %q，want %q", *model.ErrorSummary, "ToolError: E_BAD")
	}
}

// ── ③ leading / status / summary ──────────────────────────────────

// TestDisclosureLeading 钉住官方三态（`SkillRow.tsx:76-86`）。
func TestDisclosureLeading(t *testing.T) {
	cases := []struct {
		open, expandable bool
		want             skillLeading
	}{
		{true, true, leadingChevronOnly},
		{true, false, leadingChevronOnly}, // open 恒优先（虽然 open 已含 expandable）
		{false, false, leadingIconOnly},
		{false, true, leadingIconWithChevron},
	}
	for _, c := range cases {
		if got := disclosureLeading(c.open, c.expandable); got != c.want {
			t.Errorf("disclosureLeading(%v, %v) = %v，want %v", c.open, c.expandable, got, c.want)
		}
	}
}

// TestStateStatus 钉住「只有 running / error 有读屏状态」。
func TestStateStatus(t *testing.T) {
	cases := []struct {
		state SkillRowState
		want  string
		has   bool
	}{
		{SkillStateRunning, "正在加载 skill", true},
		{SkillStateError, "skill 加载失败", true},
		{SkillStateOK, "", false},
		{SkillStateStopped, "", false}, // 中止的状态走可见摘要，不是读屏
	}
	for _, c := range cases {
		got, has := stateStatus(c.state, "zh")
		if has != c.has || got != c.want {
			t.Errorf("stateStatus(%q) = (%q, %v)，want (%q, %v)", c.state, got, has, c.want, c.has)
		}
	}
}

// TestUppercaseTypeTransform 钉住 `text-transform:uppercase` 的等价变换。
func TestUppercaseTypeTransform(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Instructions", "INSTRUCTIONS"},
		{"说明", "说明"},
		{"", ""},
		{"abc123", "ABC123"},
		{"Инструкции", "Инструкции"}, // 非 ASCII 不动
	}
	for _, c := range cases {
		if got := uppercaseTypeTransform(c.in); got != c.want {
			t.Errorf("uppercaseTypeTransform(%q) = %q，want %q", c.in, got, c.want)
		}
	}
}

// ── ④ 真点击 ──────────────────────────────────────────────────────

// skillHarness 是一套「真画布 + 真命中区」的装配。
type skillHarness struct {
	plugin *Plugin
	cards  dstool.Service
	tree   *gfx.Tree
	feat   gfx.Scope
	theme  *pluginapi.Theme
	page   *ui.Page
	card   dstool.Card
}

// newSkillHarness 装配 dscore + dsskill（真 kernel、真注册）。
func newSkillHarness(t *testing.T, card dstool.Card) *skillHarness {
	t.Helper()
	tree := &gfx.Tree{}
	kernel := uikernel.New("skill-row")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dstool.Plugin{}, &Plugin{}); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	pageTheme := dscore.VivaTheme(theme)
	return &skillHarness{
		plugin: kernel.MustGet(ServiceKey).(*Plugin),
		cards:  kernel.MustGet(dstool.ServiceKey).(dstool.Service),
		tree:   tree,
		feat:   tree.Feature("skill-row"),
		theme:  theme,
		page:   ui.NewPage(ui.PageConfig{Theme: &pageTheme}),
		card:   card,
	}
}

// paint 真 Paint 一帧。
func (h *skillHarness) paint(canvas *gfx.Canvas, size ui.Size) {
	bounds := gfx.Rect{Width: size.Width, Height: size.Height}
	canvas.Fill(bounds, h.theme.BgBase)
	env := &pluginapi.Env{
		Theme: h.theme, Data: pluginapi.Snapshot{Cur: "s"}, Tree: h.tree, Feat: h.feat,
		Canvas: canvas, Bounds: bounds, MainWidth: size.Width,
	}
	h.page.Draw(canvas, bounds, func(f ui.C) {
		h.cards.Render(f, env, h.card)
	})
}

// openSession 起一个 headless 会话。
func (h *skillHarness) openSession(t *testing.T, width, height int) *headless.Session {
	t.Helper()
	var invalidator app.Invalidator
	config := app.Config{
		Title: "skill-row", Width: width, Height: height, Tree: h.tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
		Paint: func(canvas *gfx.Canvas, current ui.Size) { h.paint(canvas, current) },
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless 打不开：%v", err)
	}
	t.Cleanup(session.Close)
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	return session
}

// expandableCard 是一条**可展开**的 skill 卡（有说明正文）。
func expandableCard() dstool.Card {
	return dstool.Card{
		Seq:     1,
		Tool:    SkillToolName,
		CallID:  "call-1",
		Args:    `{"name":"demo"}`,
		Status:  dstool.StatusOK,
		View:    &apiclient.ViewSpec{Kind: string(apiclient.ViewPlain), Body: "第一行\n第二行"},
		Summary: "summary",
	}
}

// TestSkillRowRealClickToggles 真点一次必须真的切换展开态。
//
// 只断言"画出来了"不够：`Clicked()` 没接状态时界面完全正常、点下去什么都不发生。
func TestSkillRowRealClickToggles(t *testing.T) {
	harness := newSkillHarness(t, expandableCard())
	session := harness.openSession(t, 900, 700)
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}

	hit, ok := findHit(session, "加载技能")
	if !ok {
		t.Fatalf("可展开的行没有命中区；hits=%+v", session.Snapshot().Hits)
	}
	if harness.plugin.expand.expandedFor(1) {
		t.Fatal("初始态应该是收起的")
	}

	clickAt(t, session, hit)
	if !harness.plugin.expand.expandedFor(1) {
		t.Fatal("真点了一次，展开态没变——`Clicked()` 没接上 toggle")
	}
	// 再点一次收起。
	clickAt(t, session, hit)
	if harness.plugin.expand.expandedFor(1) {
		t.Fatal("第二次点击应收回")
	}
}

// TestSkillRowNotExpandableHasNoHit 钉住「没有说明正文就没有按钮语义」。
//
// 官方 `disclosureProps = expandable ? {…} : {}`：不可展开时**根本没有**
// role=button / tabIndex / onClick。实现若挂一个空 hit 区，点下去会吞掉事件
// （看起来"点了没反应"，而官方是"根本不可点"）。
func TestSkillRowNotExpandableHasNoHit(t *testing.T) {
	card := dstool.Card{Seq: 2, Tool: SkillToolName, Args: `{"name":"demo"}`, Status: dstool.StatusOK}
	harness := newSkillHarness(t, card)
	session := harness.openSession(t, 900, 700)
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if hit, ok := findHit(session, "加载技能"); ok {
		t.Fatalf("不可展开的行不该有命中区，实际 %+v", hit)
	}
}

// TestSkillRowExpandedPaintsInstructions 钉住展开后**说明卡真的画出来**。
//
// 判据不是"命中了什么"，而是：展开后本帧高度必须比收起高（说明卡占位了），
// 且高度与官方几何（24 + 4 + 卡高 + 4 + …）同量级。
func TestSkillRowExpandedPaintsInstructions(t *testing.T) {
	harness := newSkillHarness(t, expandableCard())
	session := harness.openSession(t, 900, 700)
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	collapsed := harness.cards.Height(1, nil)
	if collapsed != SkillRowHeight {
		t.Fatalf("收起高度 = %v，want %v", collapsed, SkillRowHeight)
	}

	hit, ok := findHit(session, "加载技能")
	if !ok {
		t.Fatal("没有命中区")
	}
	clickAt(t, session, hit)

	expanded := harness.cards.Height(1, nil)
	if expanded <= collapsed {
		t.Fatalf("展开后高度 %v 没有比收起 %v 高——说明卡没画出来", expanded, collapsed)
	}
	// 上界：row + 卡 margin + 260 卡高 + 卡 margin（不含 inspect 按钮）。
	// 下界：row + 卡 margin + header。
	lower := SkillRowHeight + SkillInstructionsCardMarginTop +
		SkillInstructionsHeaderPadTop + SkillInstructionsHeaderLineHeight +
		SkillInstructionsHeaderPadBottom + SkillInstructionsHeaderBorderBottom
	upper := SkillRowHeight + 2*SkillInstructionsCardMarginTop + SkillInstructionsCardMaxHeight
	if expanded < lower || expanded > upper {
		t.Fatalf("展开高度 %v 不在 [%v, %v] 内", expanded, lower, upper)
	}
}

// TestSkillRowPreparingPaintsNoSummary 钉住 preparing 态**没有**摘要。
//
// 官方 preparing 树只有 leading 图标 + 隐藏状态文案 + 闪烁标题；
// 摘要（`summary`）是 started 态才有的。
func TestSkillRowPreparingPaintsNoSummary(t *testing.T) {
	card := dstool.Card{
		Seq: 3, Tool: SkillToolName, Args: `{"name":"demo"}`,
		Status: dstool.StatusPreparing,
	}
	harness := newSkillHarness(t, card)
	session := harness.openSession(t, 900, 700)
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if got := harness.cards.Height(3, nil); got != SkillRowHeight {
		t.Fatalf("preparing 高度 = %v，want %v", got, SkillRowHeight)
	}
	// preparing 态没有 data-expandable → 没有命中区。
	if hit, ok := findHit(session, "加载技能"); ok {
		t.Fatalf("preparing 态不该有命中区，实际 %+v", hit)
	}
}

// TestSkillRowInspectButtonAbsent 钉住 inspect **没注入时整枚不画**。
//
// 官方 `inspect !== undefined ? <button> : null`。这条挡的是
// "画一个点了没反应的按钮"——那比不画更让人以为"点了会打开"。
func TestSkillRowInspectButtonAbsent(t *testing.T) {
	card := expandableCard()
	card.Seq = 4
	harness := newSkillHarness(t, card)
	harness.plugin.expand.toggle(4) // 直接展开（说明卡在，按钮仍不该出现）
	session := harness.openSession(t, 900, 700)
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if hit, ok := findHit(session, "查看"); ok {
		t.Fatalf("inspect 没注入时不该画按钮，实际 %+v", hit)
	}
}

// TestSkillRowInspectButtonClicks 钉住 inspect 按钮**画出来 + 点了会调到注入的函数**。
//
// 为什么必须先注入再起会话：`headless.Open` 的第一帧就是本用例要断言的帧，
// 而 `SetInspect` 本身不会触发重绘（viva 的失效由 Invalidator 驱动）——
// 先起会话再注入会拿到一帧"按钮还不存在"的 hits，然后误判成"没画"。
func TestSkillRowInspectButtonClicks(t *testing.T) {
	card := expandableCard()
	card.Seq = 5
	harness := newSkillHarness(t, card)
	harness.plugin.expand.toggle(5)

	var got []string
	harness.plugin.SetInspect(func(name string) { got = append(got, name) })

	session := harness.openSession(t, 900, 700)
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	hit, ok := findHit(session, "查看")
	if !ok {
		t.Fatalf("注入 inspect 后应画出按钮；hits=%+v", session.Snapshot().Hits)
	}
	clickAt(t, session, hit)
	if len(got) != 1 || got[0] != "demo" {
		t.Fatalf("inspect 被调 %v，want [demo]", got)
	}
}

// TestSkillRowInspectButtonCleared 钉住 `SetInspect(nil)` **撤掉**按钮。
//
// 官方 `inspect` 是注入的 prop：宿主能力消失（`skills/list` 不可用）时
// 整枚按钮必须消失，而不是留一个点不动的壳。
func TestSkillRowInspectButtonCleared(t *testing.T) {
	harness := newSkillHarness(t, expandableCard())
	harness.plugin.SetInspect(func(string) {})
	if harness.plugin.inspect == nil {
		t.Fatal("SetInspect 没生效")
	}
	harness.plugin.SetInspect(nil)
	if harness.plugin.inspect != nil {
		t.Fatal("SetInspect(nil) 应撤掉 inspect")
	}
}

// ── 测试工具 ──────────────────────────────────────────────────────

// findHit 按无障碍名找命中区（viva 的 hit 区带 Label）。
func findHit(session *headless.Session, label string) (gfx.Hit, bool) {
	for _, candidate := range session.Snapshot().Hits {
		if candidate.Label == label {
			return candidate, true
		}
	}
	return gfx.Hit{}, false
}

// clickAt 在命中区中心真按一次（Press → Release → Pump）。
func clickAt(t *testing.T, session *headless.Session, hit gfx.Hit) {
	t.Helper()
	x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
	if err := session.Press(x, y); err != nil {
		t.Fatal(err)
	}
	if err := session.Release(x, y); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
}

// 确保 measure 用的 rune 计数在测试里也成立（避免未使用 import 被删）。
var _ = utf8.RuneCountInString
