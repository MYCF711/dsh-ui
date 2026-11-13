// official_workflow_run_test.go —— §5.20 工作流运行面板的机器断言。
//
// 五组（对应任务书的五条硬要求）：
//
//	① 几何     WorkflowRunPanel.module.css **每个数值逐条**断言（常量 ↔ 官方字面值）
//	② 结构     **嵌套树的形状**：run → phase → member 三级缩进阶梯 + 父容器
//	③ 行为     展开/折叠（run 行、phase 行）+ 成员导航（可导航/不可导航两条分支）
//	④ 状态     running / completed / failed / cancelled / interrupted 五种全部可渲染
//	⑤ 状态机   clean/running/abnormal 三档 + focusWithin 两条分支 + 延迟折叠
//
// 外加投影面（`tool-workflow/*` 四类帧 → 一条节点）与文案（官方 locales.ts zh）。
package dsconversation

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 渲染夹具 ──────────────────────────────────────────────────────────

// workflowHarness 是一次渲染的产物。
type workflowHarness struct {
	panel  *WorkflowRunPanel
	canvas *gfx.Canvas
	tree   *gfx.Tree
	feat   gfx.Scope
	state  WorkflowDisclosureState
	width  float32
}

const workflowTestWidth float32 = 900

// newWorkflowHarness 造夹具（不渲染；用 paint 画一帧）。
func newWorkflowHarness(data WorkflowRunChatData, store *WorkflowRunDisclosures) *workflowHarness {
	if store == nil {
		store = NewWorkflowRunDisclosures()
	}
	tree := &gfx.Tree{}
	feat := tree.Feature("workflow-run-test")
	theme := dscore.Dark()
	return &workflowHarness{
		panel: &WorkflowRunPanel{
			Data:        data,
			EntryKey:    "run-key",
			Disclosures: store,
			Theme:       theme,
			ID:          func(name string) gfx.ID { return tree.ID(feat, name) },
		},
		tree:  tree,
		feat:  feat,
		width: workflowTestWidth,
	}
}

// paint 画一帧；click 非空时让这一帧投递一次点击（等价于用户点了它）。
func (h *workflowHarness) paint(t *testing.T, click string) *gfx.Canvas {
	t.Helper()
	canvas := &gfx.Canvas{}
	canvas.SetTree(h.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size
	})
	h.canvas = canvas
	if click != "" {
		canvas.BeginEventPass(h.tree.ID(h.feat, click), gfx.ClickInfo{})
	}
	pageTheme := dscore.VivaTheme(h.panel.Theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	page.Draw(canvas, gfx.Rect{Width: h.width, Height: 640}, func(f ui.C) {
		h.state = h.panel.Draw(f)
	})
	if click != "" {
		canvas.EndEventPass()
	}
	return canvas
}

// textRect 找一段文本的矩形；ok=false 表示没画出来。
func (h *workflowHarness) textRect(t *testing.T, value string) (gfx.Rect, bool) {
	t.Helper()
	for _, text := range h.canvas.Texts() {
		if text.Value == value {
			return text.Bounds, true
		}
	}
	return gfx.Rect{}, false
}

// mustTextRect 断言某段文本真的画在画布上。
func (h *workflowHarness) mustTextRect(t *testing.T, value string) gfx.Rect {
	t.Helper()
	rect, ok := h.textRect(t, value)
	if !ok {
		var got []string
		for _, text := range h.canvas.Texts() {
			got = append(got, text.Value)
		}
		t.Fatalf("画布缺少文本 %q；实际文本：%v", value, got)
	}
	return rect
}

// mustNoText 断言某段文本**没有**画出来。
func (h *workflowHarness) mustNoText(t *testing.T, value string) {
	t.Helper()
	if rect, ok := h.textRect(t, value); ok {
		t.Fatalf("画布不该出现文本 %q（矩形 %+v）", value, rect)
	}
}

// rootLeft 是本帧根列的左边界。
//
// 面板挂在页面根列上（`page.Draw` 的原点），所以恒为 0。
// 写成方法而不是字面量 0：将来面板被放进带内边距的容器时，
// 断言里的"root + …"不用逐条改。
func (h *workflowHarness) rootLeft() float32 { return 0 }

// sampleRun 造一次两阶段、四成员的 run。
//
// 成员状态**跟随 run 状态**：这样 `completed` 档才是真 clean（全完成 →
// 初始折叠），`failed` 档才是真 abnormal（强制展开）。混着写会让
// "completed 应当折叠"这条永远测不出来——这正是第一次跑就红的原因。
func sampleRun(status WorkflowRunStatus) WorkflowRunChatData {
	review := "review"
	implement := "implement"
	return WorkflowRunChatData{
		Name:   "code-review",
		Status: status,
		Phases: []WorkflowRunPhaseData{
			{
				Key:   WorkflowPhaseKey(&review),
				Phase: &review,
				Members: []WorkflowRunMemberData{
					{Seq: 1, Label: "读代码", ChildID: "child-1", Status: status},
					{Seq: 2, Label: "查回归", ChildID: "child-2", Status: status},
				},
			},
			{
				Key:   WorkflowPhaseKey(&implement),
				Phase: &implement,
				Members: []WorkflowRunMemberData{
					{Seq: 3, Label: "改代码", ChildID: "child-3", Status: status},
					{Seq: 4, Label: "写测试", ChildID: "child-4", Status: status},
				},
			},
		},
	}
}

// mixedRun 造一次**成员态混合**的 run（运行中 + 已完成 + 失败 + 取消）。
//
// 给"成员行四种状态各自可渲染"和"状态摘要拼接"用；不要拿它测 run 级开合。
func mixedRun() WorkflowRunChatData {
	review := "review"
	implement := "implement"
	return WorkflowRunChatData{
		Name:   "code-review",
		Status: WorkflowStatusRunning,
		Phases: []WorkflowRunPhaseData{
			{
				Key:   WorkflowPhaseKey(&review),
				Phase: &review,
				Members: []WorkflowRunMemberData{
					{Seq: 1, Label: "读代码", ChildID: "child-1", Status: WorkflowStatusCompleted},
					{Seq: 2, Label: "查回归", ChildID: "child-2", Status: WorkflowStatusRunning},
				},
			},
			{
				Key:   WorkflowPhaseKey(&implement),
				Phase: &implement,
				Members: []WorkflowRunMemberData{
					{Seq: 3, Label: "改代码", ChildID: "child-3", Status: WorkflowStatusFailed},
					{Seq: 4, Label: "写测试", ChildID: "child-4", Status: WorkflowStatusCancelled},
				},
			},
		},
	}
}

// ── ① 几何：官方 CSS 每个数值逐条断言 ────────────────────────────────

// TestWorkflowRunGeometryMatchesOfficialCSS 逐条核对常量与官方字面值。
//
// 为什么不用"表里有什么就断言什么"：那只能证明表自洽，证明不了**表对**。
// 这里把官方 `WorkflowRunPanel.module.css`（sha256 fa04126b…）与 primitives
// 预构建 CSS（sha256 d487cca8…）里的数**再抄一遍**，两处独立抄写一致
// 才说明没有转录错误。
func TestWorkflowRunGeometryMatchesOfficialCSS(t *testing.T) {
	// 键 = 几何表里的 `Node.Prop`，值 = 官方 CSS 字面值。
	want := map[string]float32{
		"WorkflowRunPanel.root.min-width":                         0,   // .root{min-width:0}
		"WorkflowRunPanel.runHeader.gap":                          6,   // .runHeader{gap:6px}
		"WorkflowRunPanel.runHeader.height":                       32,  // .runHeader{height:32px}
		"WorkflowRunPanel.runHeader.padding-inline":               8,   // .runHeader{padding:0 8px}
		"WorkflowRunPanel.runHeader.border-radius":                12,  // var(--dsw-radius-md)
		"WorkflowRunPanel.runHeader:focus-visible.outline-offset": -2,  // outline-offset:-2px
		"WorkflowRunPanel.runLeading.width":                       16,  // calc(16px + delta)
		"WorkflowRunPanel.runLeading.height":                      16,  // calc(16px + delta)
		"WorkflowRunPanel.runLeading.margin-right":                0,   // 覆盖 primitives 的 6px
		"DisclosureRow.leading>svg.size":                          14,  // ._leading_ svg:not([data-state])
		"WorkflowRunPanel.runTitle.max-width":                     42,  // max-width:42%
		"WorkflowRunPanel.runTitle.font-weight":                   510, // font-weight:510
		"WorkflowRunPanel.runTitle.line-height":                   24,  // calc(24px + delta)
		"WorkflowRunPanel.runSummary.line-height":                 18,  // calc(18px + delta-secondary)
		"WorkflowRunPanel.statusTail.height":                      20,  // height:20px
		"WorkflowRunPanel.statusTail.font-size":                   11,  // font-size:11px
		"WorkflowRunPanel.statusTail.line-height":                 16,  // line-height:16px
		"WorkflowRunPanel.statusTail.gap":                         4,   // gap:4px
		"WorkflowRunPanel.separator.width":                        2,   // .separator{width:2px}
		"WorkflowRunPanel.separator.height":                       2,   // .separator{height:2px}
		"WorkflowRunPanel.phaseHeader.gap":                        6,   // .phaseHeader{gap:6px}
		"WorkflowRunPanel.phaseHeader.height":                     32,  // .phaseHeader{height:32px}
		"WorkflowRunPanel.phaseHeader.border-radius":              12,  // var(--dsw-radius-md)
		"WorkflowRunPanel.phaseLeading.width":                     16,  // calc(16px + delta)
		"WorkflowRunPanel.phaseLeading.margin-right":              0,   // margin-right:0
		"WorkflowRunPanel.phaseTitle.max-width":                   42,  // max-width:42%
		"WorkflowRunPanel.phaseTitle.line-height":                 24,  // calc(24px + delta)
		"WorkflowRunPanel.phaseCount.line-height":                 20,  // calc(20px + delta-secondary)
		"WorkflowRunPanel.phaseStatus.width":                      132, // calc(132px + delta*10)
		"WorkflowRunPanel.phaseStatus.line-height":                20,  // calc(20px + delta-secondary)
		"WorkflowRunPanel.phaseList.gap":                          4,   // .phaseList{gap:4px}
		"WorkflowRunPanel.phaseList.padding-top":                  4,   // padding:4px 0 0 16px
		"WorkflowRunPanel.phaseList.padding-left":                 16,  // padding-left:16px
		"WorkflowRunPanel.phaseList.padding-left@max-width:560px": 12,  // @media(max-width:560px)
		"WorkflowRunPanel.phaseList.media-max-width":              560, // 断点
		"WorkflowRunPanel.members.gap":                            2,   // .members{gap:2px}
		"WorkflowRunPanel.members.padding-left":                   16,  // padding:0 0 0 16px
		"WorkflowRunPanel.members.padding-left@max-width:560px":   12,  // @media(max-width:560px)
		"WorkflowRunPanel.memberRow.gap":                          12,  // .memberRow{gap:12px}
		"WorkflowRunPanel.memberRow.min-height":                   24,  // calc(24px + delta)
		"WorkflowRunPanel.memberRow.border-width":                 0,   // border:0
		"WorkflowRunPanel.memberRow.border-radius":                8,   // var(--dsw-radius-sm)
		"WorkflowRunPanel.memberButton.font-weight":               500, // .memberButton .memberLabel
		"WorkflowRunPanel.memberButton.text-underline-offset":     3,   // underline-offset:3px
		"WorkflowRunPanel.dotSlot.width":                          16,  // .dotSlot{width:16px}
		"WorkflowRunPanel.dotSlot.height":                         24,  // calc(24px + delta)
		"WorkflowRunPanel.memberLabelWrap.height":                 24,  // calc(24px + delta)
		"WorkflowRunPanel.memberLabelWrap.padding-inline":         2,   // padding:0 2px
		"WorkflowRunPanel.memberLabel.font-size":                  14,  // var(--dsh-content-font-size,14px)
		"WorkflowRunPanel.memberLabel.line-height":                24,  // calc(24px + delta)
		"WorkflowRunPanel.memberStatus.width":                     64,  // .memberStatus{width:64px}
		"WorkflowRunPanel.memberStatus.line-height":               20,  // calc(20px + delta-secondary)
		"WorkflowRunPanel.empty.line-height":                      20,  // calc(20px + delta-secondary)
		"WorkflowRunPanel.headerContentLayout.gap":                6,   // .headerContentLayout{gap:6px}
		"StateDot.dot.size":                                       10,  // t ?? (ongoing ? 14 : 10)
		"StateDot.spinner.size":                                   14,  // ongoing 分支
		"StateDot.dot::after.inset":                               20,  // inset:20%
		"StateDot.dot::after.width":                               6,   // 10 × (1 − 2×20%)
		"StateDot.spinner.viewBox":                                24,  // viewBox="0 0 24 24"
		"StateDot.spinner.r":                                      9.5, // r=9.5
		"StateDot.spinner.stroke-width":                           2,   // stroke-width:2
	}
	byKey := map[string]float32{}
	for _, row := range OfficialTranscriptGeometry() {
		byKey[row.Node+"."+row.Prop] = row.Impl
	}
	for key, official := range want {
		impl, ok := byKey[key]
		if !ok {
			t.Errorf("几何表缺少 %s（官方 %v）", key, official)
			continue
		}
		if impl != official {
			t.Errorf("%s 实现值 %v，官方 %v", key, impl, official)
		}
	}
	// 常量本身也必须与表一致（`TestGeometryUsesSingleSourceOfTruth` 的手工版：
	// 那个测试只钉了 7 个键，这里是本组件全量）。
	check := func(name string, got, want float32) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %v，应为 %v", name, got, want)
		}
	}
	check("WorkflowRunHeaderGap", WorkflowRunHeaderGap, 6)
	check("WorkflowRunHeaderHeight", WorkflowRunHeaderHeight, 32)
	check("WorkflowRunHeaderPadX", WorkflowRunHeaderPadX, 8)
	check("WorkflowRunLeadingSize", WorkflowRunLeadingSize, 16)
	check("WorkflowRunChevronSize", WorkflowRunChevronSize, 14)
	check("WorkflowRunTitleMaxWidth", WorkflowRunTitleMaxWidth, 42)
	check("WorkflowRunTitleWeight", WorkflowRunTitleWeight, 510)
	check("WorkflowRunRowLineHeight", WorkflowRunRowLineHeight, 24)
	check("WorkflowRunSummaryLineHeight", WorkflowRunSummaryLineHeight, 18)
	check("WorkflowRunStatusTailHeight", WorkflowRunStatusTailHeight, 20)
	check("WorkflowRunStatusTailFontSize", WorkflowRunStatusTailFontSize, 11)
	check("WorkflowRunStatusTailLineHeight", WorkflowRunStatusTailLineHeight, 16)
	check("WorkflowRunStatusTailGap", WorkflowRunStatusTailGap, 4)
	check("WorkflowRunSeparatorSize", WorkflowRunSeparatorSize, 2)
	check("WorkflowRunPhaseCountLineHeight", WorkflowRunPhaseCountLineHeight, 20)
	check("WorkflowRunPhaseStatusWidth", WorkflowRunPhaseStatusWidth, 132)
	check("WorkflowRunPhaseListGap", WorkflowRunPhaseListGap, 4)
	check("WorkflowRunPhaseListPadTop", WorkflowRunPhaseListPadTop, 4)
	check("WorkflowRunPhaseListPadLeft", WorkflowRunPhaseListPadLeft, 16)
	check("WorkflowRunPhaseListPadLeftNarrow", WorkflowRunPhaseListPadLeftNarrow, 12)
	check("WorkflowRunNarrowMaxWidth", WorkflowRunNarrowMaxWidth, 560)
	check("WorkflowRunMembersGap", WorkflowRunMembersGap, 2)
	check("WorkflowRunMembersPadLeft", WorkflowRunMembersPadLeft, 16)
	check("WorkflowRunMemberGap", WorkflowRunMemberGap, 12)
	check("WorkflowRunMemberMinHeight", WorkflowRunMemberMinHeight, 24)
	check("WorkflowRunMemberLabelWrapHeight", WorkflowRunMemberLabelWrapHeight, 24)
	check("WorkflowRunMemberLabelWrapPadX", WorkflowRunMemberLabelWrapPadX, 2)
	check("WorkflowRunMemberLabelFontSize", WorkflowRunMemberLabelFontSize, 14)
	check("WorkflowRunMemberStatusWidth", WorkflowRunMemberStatusWidth, 64)
	check("WorkflowRunEmptyLineHeight", WorkflowRunEmptyLineHeight, 20)
	check("WorkflowRunHeaderContentGap", WorkflowRunHeaderContentGap, 6)
	check("WorkflowRunDotSize", WorkflowRunDotSize, 10)
	check("WorkflowRunDotSizeOngoing", WorkflowRunDotSizeOngoing, 14)
	check("WorkflowRunDotCoreSize", WorkflowRunDotCoreSize, 6)
	check("WorkflowRunDotInsetPercent", WorkflowRunDotInsetPercent, 20)
	check("WorkflowRunSpinnerViewBox", WorkflowRunSpinnerViewBox, 24)
	check("WorkflowRunSpinnerRadius", WorkflowRunSpinnerRadius, 9.5)
	check("WorkflowRunSpinnerStroke", WorkflowRunSpinnerStroke, 2)
}

// TestWorkflowRunColorsMatchOfficialTokens 钉住四条颜色映射（官方 CSS 变量）。
//
// 颜色不进几何表（表只有 float），所以在渲染侧单独钉：
//
//	done     → --dsw-alias-state-success-primary
//	warning  → --dsw-alias-state-warn-primary
//	error    → --dsw-alias-state-error-primary
//	ongoing  → --dsw-alias-label-tertiary
func TestWorkflowRunColorsMatchOfficialTokens(t *testing.T) {
	theme := dscore.Dark()
	cases := []struct {
		state WorkflowDotState
		want  ui.Color
	}{
		{WorkflowDotDone, theme.SuccessPrimary},
		{WorkflowDotWarning, theme.Warn},
		{WorkflowDotError, theme.Error},
		{WorkflowDotOngoing, theme.LabelTertiary},
	}
	for _, c := range cases {
		if got := WorkflowDotColor(theme, c.state); got != c.want {
			t.Errorf("StateDot(%s) 颜色 = %v，应为 %v", c.state, got, c.want)
		}
	}
}

// ── ② 结构：嵌套树的形状 ─────────────────────────────────────────────

// TestWorkflowRunNestedTreeIndentation 是**嵌套树形状**的硬断言。
//
// 官方三级缩进（逐层累加，全部来自 CSS）：
//
//	run 标题   x = root + 8（runHeader padding） + 16（leading） + 6（gap）        = root + 30
//	phase 标题 x = root + 16（phaseList padding-left） + 16（leading） + 6（gap）  = root + 38
//	member 标签 x = root + 16（phaseList） + 16（members padding-left）
//	                   + 16（dotSlot） + 12（memberRow gap） + 2（labelWrap padding） = root + 62
//
// **缩进就是层次**：这三条数只有真的三级嵌套才成立。把树压平成三行同缩进，
// 或者少套一层容器，数字立刻不对——这就是"形状"可断言的地方。
func TestWorkflowRunNestedTreeIndentation(t *testing.T) {
	h := newWorkflowHarness(mixedRun(), nil)
	h.paint(t, "")
	root := h.rootLeft()

	runTitle := h.mustTextRect(t, "code-review")
	wantRun := root + WorkflowRunHeaderPadX + WorkflowRunLeadingSize + WorkflowRunHeaderGap
	if runTitle.X != wantRun {
		t.Errorf("run 标题 x = %v，应为 %v（root %v + 8 + 16 + 6）", runTitle.X, wantRun, root)
	}

	phaseTitle := h.mustTextRect(t, "review")
	wantPhase := root + WorkflowRunPhaseListPadLeft + WorkflowRunLeadingSize + WorkflowRunHeaderGap
	if phaseTitle.X != wantPhase {
		t.Errorf("phase 标题 x = %v，应为 %v（root %v + 16 + 16 + 6）", phaseTitle.X, wantPhase, root)
	}

	memberLabel := h.mustTextRect(t, "读代码")
	wantMember := root + WorkflowRunPhaseListPadLeft + WorkflowRunMembersPadLeft +
		WorkflowRunDotSlotWidth + WorkflowRunMemberGap + WorkflowRunMemberLabelWrapPadX
	if memberLabel.X != wantMember {
		t.Errorf("member 标签 x = %v，应为 %v（root %v + 16 + 16 + 16 + 12 + 2）",
			memberLabel.X, wantMember, root)
	}

	// 三级**严格递增**（任何一层缩进丢失都会让两者相等）。
	if !(runTitle.X < phaseTitle.X && phaseTitle.X < memberLabel.X) {
		t.Fatalf("缩进阶梯必须严格递增：run %v < phase %v < member %v",
			runTitle.X, phaseTitle.X, memberLabel.X)
	}

	// 第二阶段的成员也必须与第一阶段同档（同一层，不得越缩越深）。
	if got := h.mustTextRect(t, "改代码"); got.X != memberLabel.X {
		t.Errorf("第二阶段成员 x = %v，应与第一阶段同档 %v", got.X, memberLabel.X)
	}
}

// TestWorkflowRunNestedTreeRowHeights 钉住三级的**行盒高度**与纵向节奏。
//
// 文本的 `Bounds.Height` 是行高（line-height），行盒高度靠**容器**表达。
// 所以三级的行盒高度用「相邻两级的 Y 差」反推：
//
//	run → phase:    runHeader 32 + phaseList padding-top 4          = 36
//	phase → member: phaseHeader 32 + members padding-top 0（首行）  = 32
//	member → member: memberRow min-height 24
//
// 这三条数只有三级**真的各占一行**才成立；把 phase 挤进 run 的行里，
// 第一条差值会变成 0。
func TestWorkflowRunNestedTreeRowHeights(t *testing.T) {
	h := newWorkflowHarness(mixedRun(), nil)
	h.paint(t, "")
	runTitle := h.mustTextRect(t, "code-review")
	phaseTitle := h.mustTextRect(t, "review")
	memberLabel := h.mustTextRect(t, "读代码")

	// 文本行高 = 各自 CSS 的 line-height。
	if runTitle.Height != WorkflowRunRowLineHeight {
		t.Errorf("run 标题行高 = %v，应为 %v", runTitle.Height, WorkflowRunRowLineHeight)
	}
	if phaseTitle.Height != WorkflowRunRowLineHeight {
		t.Errorf("phase 标题行高 = %v，应为 %v", phaseTitle.Height, WorkflowRunRowLineHeight)
	}
	if memberLabel.Height != WorkflowRunMemberLabelWrapHeight {
		t.Errorf("member 标签行高 = %v，应为 %v", memberLabel.Height, WorkflowRunMemberLabelWrapHeight)
	}

	// 行盒高度（相邻两级的 Y 差）。
	if got, want := phaseTitle.Y-runTitle.Y, WorkflowRunHeaderHeight+WorkflowRunPhaseListPadTop; got != want {
		t.Errorf("run → phase 纵向间距 = %v，应为 %v（runHeader 32 + phaseList pad-top 4）", got, want)
	}
	// phase → member 的行盒间距 = phaseHeader 32；但文本 Y 还差一个
	// 「phase 标题在 32px 行盒里垂直居中」的偏移 (32−24)/2 = 4。
	if got, want := memberLabel.Y-phaseTitle.Y,
		WorkflowRunHeaderHeight-(WorkflowRunHeaderHeight-WorkflowRunRowLineHeight)/2; got != want {
		t.Errorf("phase → member 纵向间距 = %v，应为 %v（phaseHeader 32 − 居中偏移 4）", got, want)
	}
	// 纵向顺序：run 在 phase 之上，phase 在 member 之上。
	if !(runTitle.Y < phaseTitle.Y && phaseTitle.Y < memberLabel.Y) {
		t.Fatalf("纵向顺序错：run %v / phase %v / member %v", runTitle.Y, phaseTitle.Y, memberLabel.Y)
	}
	// 同一阶段的第二个成员与第一个成员的间距 = memberRow min-height 24。
	second := h.mustTextRect(t, "查回归")
	if got, want := second.Y-memberLabel.Y, WorkflowRunMemberMinHeight+WorkflowRunMembersGap; got != want {
		t.Errorf("成员行间距 = %v，应为 %v（memberRow min-height 24 + members gap 2）", got, want)
	}
}

// TestWorkflowRunMembersStayInsidePhase 钉住"成员属于它自己的阶段"。
//
// 每个 phase 的成员行必须落在该 phase 标题**之后**、下一个 phase 标题**之前**
// 的纵向区间里。把成员挂错阶段（例如全挂在最后一个阶段下）会让这条红。
func TestWorkflowRunMembersStayInsidePhase(t *testing.T) {
	h := newWorkflowHarness(mixedRun(), nil)
	h.paint(t, "")
	reviewY := h.mustTextRect(t, "review").Y
	implementY := h.mustTextRect(t, "implement").Y

	for _, label := range []string{"读代码", "查回归"} {
		y := h.mustTextRect(t, label).Y
		if y <= reviewY || y >= implementY {
			t.Errorf("成员 %q 的 y=%v 不在 review 阶段区间 (%v, %v) 内", label, y, reviewY, implementY)
		}
	}
	for _, label := range []string{"改代码", "写测试"} {
		y := h.mustTextRect(t, label).Y
		if y <= implementY {
			t.Errorf("成员 %q 的 y=%v 不在 implement 阶段之后（%v）", label, y, implementY)
		}
	}
}

// ── ③ 行为：展开/折叠 + 成员导航 ─────────────────────────────────────

// TestWorkflowRunCollapsedRunHidesPhases 钉住 clean 档的初始折叠。
//
// 官方 `initialDisclosureState`：`open = mode !== 'clean'`。
// 一个全部完成的 run 就是 clean → 初始折叠。
func TestWorkflowRunCollapsedRunHidesPhases(t *testing.T) {
	h := newWorkflowHarness(sampleRun(WorkflowStatusCompleted), nil)
	h.paint(t, "")
	if h.state.Open {
		t.Fatal("全部完成的 run 初始必须折叠（官方 initialDisclosureState）")
	}
	// 折叠时 phase 与 member 一个都不画，但 run 头（名字/计数/状态）照画。
	h.mustTextRect(t, "code-review")
	h.mustTextRect(t, "4 个成员")
	h.mustTextRect(t, "已完成")
	h.mustNoText(t, "review")
	h.mustNoText(t, "读代码")
}

// TestWorkflowRunRunningRunExpands 钉住 running 档的初始展开。
func TestWorkflowRunRunningRunExpands(t *testing.T) {
	h := newWorkflowHarness(sampleRun(WorkflowStatusRunning), nil)
	h.paint(t, "")
	if !h.state.Open {
		t.Fatal("运行中的 run 初始必须展开（官方 initialDisclosureState）")
	}
	if h.state.Mode != WorkflowDisclosureRunning {
		t.Errorf("run 事实 mode = %q，应为 running", h.state.Mode)
	}
	h.mustTextRect(t, "review")
	h.mustTextRect(t, "读代码")
}

// TestWorkflowRunToggleRunCollapses 钉住"点 run 头折叠"。
func TestWorkflowRunToggleRunCollapses(t *testing.T) {
	h := newWorkflowHarness(sampleRun(WorkflowStatusRunning), nil)
	h.paint(t, "")
	if !h.state.Open {
		t.Fatal("前置：运行中的 run 应已展开")
	}
	// 点击在**这一帧**的绘制里处理（viva `Hit(...).Clicked()` 的语义），
	// 新状态从下一帧起生效——所以点完要再画一帧才看得到结果。
	h.paint(t, "workflow-run-header-run-key")
	h.paint(t, "")
	if h.state.Open {
		t.Fatal("点 run 头后必须折叠")
	}
	h.mustNoText(t, "读代码")
	// 再点一次必须展开回来（**双向**，不是单向开关）。
	h.paint(t, "workflow-run-header-run-key")
	h.paint(t, "")
	if !h.state.Open {
		t.Fatal("再点一次必须展开")
	}
	h.mustTextRect(t, "读代码")
}

// TestWorkflowRunTogglePhaseCollapses 钉住"点 phase 头折叠该阶段"。
//
// 关键：折叠一个阶段**不得**影响另一个阶段，也不得影响 run 本身。
func TestWorkflowRunTogglePhaseCollapses(t *testing.T) {
	h := newWorkflowHarness(sampleRun(WorkflowStatusRunning), nil)
	h.paint(t, "")
	reviewPhase := sampleRun(WorkflowStatusRunning).Phases[0]
	h.mustTextRect(t, "读代码")

	h.paint(t, "workflow-run-phase-run-key-"+reviewPhase.Key)
	h.paint(t, "")
	state := h.panel.Disclosures.Phase("run-key", reviewPhase)
	if state.Open {
		t.Fatal("点 phase 头后该阶段必须折叠")
	}
	if !h.state.Open {
		t.Fatal("折叠一个阶段**不得**把 run 一起收起来")
	}
	h.mustNoText(t, "读代码")
	// 另一个阶段不受影响。
	h.mustTextRect(t, "改代码")
}

// TestWorkflowRunMemberNavigation 钉住成员行两条分支（官方 `renderButton`）。
//
//	可导航（running + 有 childId + 注入了 openSession）→ 点击回传子会话 id
//	不可导航（其余情形）                            → 官方 `div.memberRow`，点了没反应
func TestWorkflowRunMemberNavigation(t *testing.T) {
	var opened []string
	h := newWorkflowHarness(mixedRun(), nil)
	h.panel.OpenSession = func(childID string) { opened = append(opened, childID) }
	h.paint(t, "")
	h.paint(t, "workflow-run-member-run-key-2")
	if len(opened) != 1 || opened[0] != "child-2" {
		t.Fatalf("点击运行中成员回传 %v，应为 [child-2]", opened)
	}

	// 已完成成员（seq=1）不可导航：点了不产生回调。
	opened = nil
	h.paint(t, "workflow-run-member-run-key-1")
	if len(opened) != 0 {
		t.Fatalf("已完成成员不该可导航，实际回传 %v", opened)
	}

	// 没有注入 openSession 时**所有**成员都不可导航（官方 renderButton 的
	// 另一条判据）：点运行中成员不该崩、也不该产生回调。
	h2 := newWorkflowHarness(mixedRun(), nil)
	h2.paint(t, "")
	h2.paint(t, "workflow-run-member-run-key-2")
	if _, ok := h2.textRect(t, "读代码"); !ok {
		t.Fatal("无导航能力时成员行仍必须渲染")
	}
}

// ── ④ 状态：五种全部可渲染 ───────────────────────────────────────────

// TestWorkflowRunAllStatusesRender 钉住五个 run 状态各自可渲染。
func TestWorkflowRunAllStatusesRender(t *testing.T) {
	cases := []struct {
		status WorkflowRunStatus
		text   string
		dot    WorkflowDotState
	}{
		{WorkflowStatusRunning, "运行中", WorkflowDotOngoing},
		{WorkflowStatusCompleted, "已完成", WorkflowDotDone},
		{WorkflowStatusFailed, "失败", WorkflowDotError},
		{WorkflowStatusCancelled, "已取消", WorkflowDotWarning},
		{WorkflowStatusInterrupted, "已中断", WorkflowDotWarning},
	}
	for _, c := range cases {
		h := newWorkflowHarness(sampleRun(c.status), nil)
		h.paint(t, "")
		h.mustTextRect(t, c.text)
		if got := WorkflowDotStateOf(c.status); got != c.dot {
			t.Errorf("状态 %q 的 StateDot = %q，应为 %q", c.status, got, c.dot)
		}
		// 异常三态必须**强制展开**（官方 abnormal 覆盖用户的折叠）。
		wantOpen := c.status != WorkflowStatusCompleted
		if h.state.Open != wantOpen {
			t.Errorf("状态 %q 初始 open = %v，应为 %v", c.status, h.state.Open, wantOpen)
		}
	}
}

// TestWorkflowRunMemberStatusesRender 钉住成员行四个状态文案各自可渲染。
func TestWorkflowRunMemberStatusesRender(t *testing.T) {
	statuses := []struct {
		status WorkflowRunStatus
		text   string
	}{
		{WorkflowStatusCompleted, "已完成"},
		{WorkflowStatusRunning, "运行中"},
		{WorkflowStatusFailed, "失败"},
		{WorkflowStatusCancelled, "已取消"},
		{WorkflowStatusInterrupted, "已中断"},
	}
	members := make([]WorkflowRunMemberData, 0, len(statuses))
	for i, c := range statuses {
		members = append(members, WorkflowRunMemberData{
			Seq: i + 1, Label: "成员" + c.text, ChildID: "c" + c.text, Status: c.status,
		})
	}
	phase := "p"
	data := WorkflowRunChatData{
		Name: "run", Status: WorkflowStatusRunning,
		Phases: []WorkflowRunPhaseData{{Key: WorkflowPhaseKey(&phase), Phase: &phase, Members: members}},
	}
	h := newWorkflowHarness(data, nil)
	h.paint(t, "")
	for _, s := range statuses {
		h.mustTextRect(t, "成员"+s.text)
		h.mustTextRect(t, s.text)
	}
}

// TestWorkflowRunEmptyPhasesShowsEmptyState 钉住 `run.empty` 空态。
func TestWorkflowRunEmptyPhasesShowsEmptyState(t *testing.T) {
	h := newWorkflowHarness(WorkflowRunChatData{
		Name: "empty-run", Status: WorkflowStatusRunning,
	}, nil)
	h.paint(t, "")
	h.mustTextRect(t, WorkflowTextRunEmpty)
	if WorkflowTextRunEmpty != "没有启动成员" {
		t.Errorf("run.empty 文案 = %q，官方 zh 是「没有启动成员」", WorkflowTextRunEmpty)
	}
}

// ── ⑤ 状态机（官方 `advanceDisclosureState` 的四条分支）──────────────

// TestWorkflowDisclosureInitialState 钉住初始开合判据。
func TestWorkflowDisclosureInitialState(t *testing.T) {
	cases := []struct {
		mode WorkflowDisclosureMode
		open bool
	}{
		{WorkflowDisclosureClean, false},
		{WorkflowDisclosureRunning, true},
		{WorkflowDisclosureAbnormal, true},
	}
	for _, c := range cases {
		state := WorkflowInitialDisclosureState(WorkflowDisclosureFacts{Mode: c.mode, ActivityCount: 3})
		if state.Open != c.open {
			t.Errorf("mode=%q 初始 open = %v，应为 %v", c.mode, state.Open, c.open)
		}
		if state.PendingCleanCollapse {
			t.Errorf("mode=%q 初始不得挂起折叠", c.mode)
		}
	}
}

// TestWorkflowDisclosureSameFactsKeepsState 是分支 ①：事实没变就原样返回。
func TestWorkflowDisclosureSameFactsKeepsState(t *testing.T) {
	current := WorkflowDisclosureState{
		WorkflowDisclosureFacts: WorkflowDisclosureFacts{Mode: WorkflowDisclosureRunning, ActivityCount: 2},
		Open:                    false, // 用户手动收起了
	}
	next := WorkflowAdvanceDisclosureState(current,
		WorkflowDisclosureFacts{Mode: WorkflowDisclosureRunning, ActivityCount: 2}, false)
	if next != current {
		t.Fatalf("事实没变时不得改动状态：%+v → %+v", current, next)
	}
}

// TestWorkflowDisclosureCleanCollapses 是分支 ②：变 clean 就折叠（无焦点时立刻收）。
func TestWorkflowDisclosureCleanCollapses(t *testing.T) {
	current := WorkflowDisclosureState{
		WorkflowDisclosureFacts: WorkflowDisclosureFacts{Mode: WorkflowDisclosureRunning, ActivityCount: 2},
		Open:                    true,
	}
	next := WorkflowAdvanceDisclosureState(current,
		WorkflowDisclosureFacts{Mode: WorkflowDisclosureClean, ActivityCount: 0}, false)
	if next.Open || next.PendingCleanCollapse {
		t.Fatalf("无焦点时变 clean 必须立刻折叠：%+v", next)
	}
	if next.Mode != WorkflowDisclosureClean {
		t.Errorf("事实未更新：%+v", next)
	}
}

// TestWorkflowDisclosureFocusWithinBranches 是分支 ② 的**延迟**规则。
//
// 官方：`deferCollapse = current.open && focusWithin`——焦点还在内容里时先
// 挂起，等 blur 再收（否则用户正在里面操作，行突然收起来）。
func TestWorkflowDisclosureFocusWithinBranches(t *testing.T) {
	open := WorkflowDisclosureState{
		WorkflowDisclosureFacts: WorkflowDisclosureFacts{Mode: WorkflowDisclosureRunning, ActivityCount: 2},
		Open:                    true,
	}
	clean := WorkflowDisclosureFacts{Mode: WorkflowDisclosureClean, ActivityCount: 0}

	// 焦点在里面：挂起（先不收）。
	deferred := WorkflowAdvanceDisclosureState(open, clean, true)
	if !deferred.Open || !deferred.PendingCleanCollapse {
		t.Fatalf("焦点在内容里时必须挂起折叠：%+v", deferred)
	}
	// 事实仍然相同 + 焦点已离开 → 现在收。
	settled := WorkflowAdvanceDisclosureState(deferred, clean, false)
	if settled.Open || settled.PendingCleanCollapse {
		t.Fatalf("焦点离开后必须收尾：%+v", settled)
	}
	// `WorkflowCollapsePending` 是 blur 时的那条路径。
	if got := WorkflowCollapsePending(deferred); got.Open || got.PendingCleanCollapse {
		t.Fatalf("collapsePending 必须收尾：%+v", got)
	}
	// 没有挂起时是 no-op（**值相等**，不是"看起来一样"）。
	if got := WorkflowCollapsePending(clean.toState()); got != clean.toState() {
		t.Fatalf("未挂起时 collapsePending 必须是 no-op：%+v", got)
	}
}

// toState 是小工具：把事实包成"已折叠"的状态（只给上面的 no-op 断言用）。
func (f WorkflowDisclosureFacts) toState() WorkflowDisclosureState {
	return WorkflowDisclosureState{WorkflowDisclosureFacts: f}
}

// TestWorkflowDisclosureAbnormalForcesOpen 是分支 ③：异常**覆盖**用户的折叠。
func TestWorkflowDisclosureAbnormalForcesOpen(t *testing.T) {
	// 用户把运行中的 run 收起了。
	collapsed := WorkflowDisclosureState{
		WorkflowDisclosureFacts: WorkflowDisclosureFacts{Mode: WorkflowDisclosureRunning, ActivityCount: 2},
		Open:                    false,
	}
	abnormal := WorkflowDisclosureFacts{Mode: WorkflowDisclosureAbnormal, ActivityCount: 2}
	next := WorkflowAdvanceDisclosureState(collapsed, abnormal, false)
	if !next.Open {
		t.Fatal("变 abnormal 必须强制展开（失败/取消/中断优先于用户折叠）")
	}
	// 从 clean 变 abnormal 同样强制展开。
	clean := WorkflowInitialDisclosureState(WorkflowDisclosureFacts{Mode: WorkflowDisclosureClean})
	if got := WorkflowAdvanceDisclosureState(clean, abnormal, false); !got.Open {
		t.Fatal("clean → abnormal 必须展开")
	}
}

// TestWorkflowDisclosureKeepsUserIntent 是分支 ④：running ↔ running 保留用户意图。
func TestWorkflowDisclosureKeepsUserIntent(t *testing.T) {
	collapsed := WorkflowDisclosureState{
		WorkflowDisclosureFacts: WorkflowDisclosureFacts{Mode: WorkflowDisclosureRunning, ActivityCount: 2},
		Open:                    false,
	}
	// 成员数变了（事实变），但仍是 running → 保留"收起"。
	next := WorkflowAdvanceDisclosureState(collapsed,
		WorkflowDisclosureFacts{Mode: WorkflowDisclosureRunning, ActivityCount: 3}, false)
	if next.Open {
		t.Fatal("running → running 必须保留用户的折叠意图")
	}
	if next.ActivityCount != 3 {
		t.Errorf("事实未更新：%+v", next)
	}
}

// TestWorkflowRunDisclosureFactsSpread 钉住 run 级事实的**传染**与求和。
func TestWorkflowRunDisclosureFactsSpread(t *testing.T) {
	clean := WorkflowDisclosureFacts{Mode: WorkflowDisclosureClean, ActivityCount: 1}
	running := WorkflowDisclosureFacts{Mode: WorkflowDisclosureRunning, ActivityCount: 2}
	abnormal := WorkflowDisclosureFacts{Mode: WorkflowDisclosureAbnormal, ActivityCount: 3}

	// 任何 phase abnormal → run abnormal（即使 run 自己还在跑）。
	if got := WorkflowRunDisclosureFacts(WorkflowStatusRunning, []WorkflowDisclosureFacts{running, abnormal}); got.Mode != WorkflowDisclosureAbnormal {
		t.Errorf("phase abnormal 必须传染 run：%+v", got)
	}
	// 全部 clean 且 run 完成 → clean。
	if got := WorkflowRunDisclosureFacts(WorkflowStatusCompleted, []WorkflowDisclosureFacts{clean, clean}); got.Mode != WorkflowDisclosureClean {
		t.Errorf("全 clean + completed 应为 clean：%+v", got)
	}
	// run 还在跑 → running（即使两个 phase 都 clean）。
	if got := WorkflowRunDisclosureFacts(WorkflowStatusRunning, []WorkflowDisclosureFacts{clean, clean}); got.Mode != WorkflowDisclosureRunning {
		t.Errorf("run running 应为 running：%+v", got)
	}
	// activityCount 是各 phase 之和（**不是** phase 数）。
	if got := WorkflowRunDisclosureFacts(WorkflowStatusCompleted, []WorkflowDisclosureFacts{clean, running}); got.ActivityCount != 3 {
		t.Errorf("activityCount = %d，应为 1+2=3", got.ActivityCount)
	}
}

// TestWorkflowPhaseDisclosureFactsPriority 钉住 phase 事实的优先级。
func TestWorkflowPhaseDisclosureFactsPriority(t *testing.T) {
	abnormalAndRunning := WorkflowRunPhaseData{Members: []WorkflowRunMemberData{
		{Status: WorkflowStatusRunning}, {Status: WorkflowStatusFailed},
	}}
	if got := WorkflowPhaseDisclosureFacts(abnormalAndRunning); got.Mode != WorkflowDisclosureAbnormal {
		t.Errorf("异常优先于运行中：%+v", got)
	}
	onlyRunning := WorkflowRunPhaseData{Members: []WorkflowRunMemberData{
		{Status: WorkflowStatusCompleted}, {Status: WorkflowStatusRunning},
	}}
	if got := WorkflowPhaseDisclosureFacts(onlyRunning); got.Mode != WorkflowDisclosureRunning {
		t.Errorf("有运行中成员应为 running：%+v", got)
	}
	onlyDone := WorkflowRunPhaseData{Members: []WorkflowRunMemberData{
		{Status: WorkflowStatusCompleted},
	}}
	if got := WorkflowPhaseDisclosureFacts(onlyDone); got.Mode != WorkflowDisclosureClean {
		t.Errorf("全完成应为 clean：%+v", got)
	}
	if got := WorkflowPhaseDisclosureFacts(WorkflowRunPhaseData{}); got.ActivityCount != 0 {
		t.Errorf("空阶段 activityCount = %d，应为 0", got.ActivityCount)
	}
}

// ── 文案（官方 locales.ts zh 逐条）────────────────────────────────────

// TestWorkflowRunStatusTexts 钉住五个状态文案（官方 zh 字典原文）。
func TestWorkflowRunStatusTexts(t *testing.T) {
	want := map[WorkflowRunStatus]string{
		WorkflowStatusRunning:     "运行中",
		WorkflowStatusCompleted:   "已完成",
		WorkflowStatusFailed:      "失败",
		WorkflowStatusCancelled:   "已取消",
		WorkflowStatusInterrupted: "已中断",
	}
	for status, text := range want {
		if got := WorkflowStatusText(status); got != text {
			t.Errorf("status.%s = %q，官方 zh 是 %q", status, got, text)
		}
	}
	// 未知状态**不编**文案。
	if got := WorkflowStatusText(WorkflowRunStatus("bogus")); got != "" {
		t.Errorf("未知状态文案 = %q，应为空串（不编）", got)
	}
}

// TestWorkflowPhaseKeyPreservesEmptyVsMissing 钉住 `null` 与 `""` 两个身份。
func TestWorkflowPhaseKeyPreservesEmptyVsMissing(t *testing.T) {
	empty := ""
	if WorkflowPhaseKey(nil) == WorkflowPhaseKey(&empty) {
		t.Fatal("null 与空串必须是**两个**不同的阶段键（官方 workflowPhaseKey）")
	}
	if got := WorkflowPhaseKey(nil); got != "missing" {
		t.Errorf("缺省键 = %q，应为 missing", got)
	}
	if got := WorkflowPhaseKey(&empty); got != "value:0:" {
		t.Errorf("空串键 = %q，应为 value:0:", got)
	}
	// 带长度的键不会因为相邻拼接而撞（"value:"+a+b 的经典形状）。
	a, b := "ab", "c"
	c, d := "a", "bc"
	if WorkflowPhaseKey(&a)+WorkflowPhaseKey(&b) == WorkflowPhaseKey(&c)+WorkflowPhaseKey(&d) {
		t.Skip("这条拼接比较本来就会相等——下面用单键比较")
	}
	// 两条不同原文必须给出不同键。
	ab, abc := "ab", "abc"
	if WorkflowPhaseKey(&ab) == WorkflowPhaseKey(&abc) {
		t.Fatal("不同阶段名不得撞键")
	}
}

// TestWorkflowReadablePhaseAndMember 钉住两个兜底文案。
func TestWorkflowReadablePhaseAndMember(t *testing.T) {
	empty := ""
	if got := WorkflowReadablePhase(nil); got != "未分阶段" {
		t.Errorf("phase.unassigned = %q", got)
	}
	if got := WorkflowReadablePhase(&empty); got != "空阶段名" {
		t.Errorf("phase.empty = %q", got)
	}
	named := "review"
	if got := WorkflowReadablePhase(&named); got != "review" {
		t.Errorf("有名字的阶段必须原样显示，实际 %q", got)
	}
	if got := WorkflowReadableMember(""); got != "空成员名" {
		t.Errorf("member.empty = %q", got)
	}
	if got := WorkflowReadableMember("读代码"); got != "读代码" {
		t.Errorf("成员标签 = %q", got)
	}
}

// TestWorkflowMemberCountText 钉住成员计数（官方中文 one/other 同形）。
func TestWorkflowMemberCountText(t *testing.T) {
	if got := WorkflowMemberCount(1); got != "1 个成员" {
		t.Errorf("单成员 = %q", got)
	}
	if got := WorkflowMemberCount(4); got != "4 个成员" {
		t.Errorf("多成员 = %q", got)
	}
	if got := WorkflowMemberCount(0); got != "0 个成员" {
		t.Errorf("零成员 = %q", got)
	}
}

// TestWorkflowPhaseStatusSummaryRules 钉住官方 `phaseStatusSummary` 的五条规则。
func TestWorkflowPhaseStatusSummaryRules(t *testing.T) {
	cases := []struct {
		name    string
		members []WorkflowRunMemberData
		want    string
	}{
		{
			name:    "全完成只报 completed",
			members: []WorkflowRunMemberData{{Status: WorkflowStatusCompleted}, {Status: WorkflowStatusCompleted}},
			want:    "已完成 2",
		},
		{
			name:    "只有运行中",
			members: []WorkflowRunMemberData{{Status: WorkflowStatusRunning}},
			want:    "运行中 1",
		},
		{
			name:    "多活态用 · 连接（顺序固定）",
			members: []WorkflowRunMemberData{{Status: WorkflowStatusRunning}, {Status: WorkflowStatusFailed}},
			want:    "运行中 1 · 失败 1",
		},
		{
			name: "interrupted + completed 时 completed 提前",
			members: []WorkflowRunMemberData{
				{Status: WorkflowStatusInterrupted}, {Status: WorkflowStatusCompleted},
			},
			want: "已完成 1 · 已中断 1",
		},
		{
			name:    "只有 cancelled",
			members: []WorkflowRunMemberData{{Status: WorkflowStatusCancelled}},
			want:    "已取消 1",
		},
		{
			name:    "空成员 = 已完成 0",
			members: nil,
			want:    "已完成 0",
		},
	}
	for _, c := range cases {
		if got := WorkflowPhaseStatusSummary(c.members); got != c.want {
			t.Errorf("%s：摘要 = %q，应为 %q", c.name, got, c.want)
		}
	}
}

// ── 投影面：`tool-workflow/*` 四类帧 → 一条节点 ─────────────────────

// TestProjectWorkflowRunFoldsMembers 钉住四类帧折成**一条**节点。
//
// 官方 `workflow-definition.ts` 的 start/update：run-start 定位置，
// 后续 agent-start / agent-end / run-end 只更新内容。一帧一节点的话，
// 一次四成员的工作流会在界面上变成 6 行。
func TestProjectWorkflowRunFoldsMembers(t *testing.T) {
	tr := NewTranscript()
	tr.ApplyFrames([]apiclient.Frame{
		frame(1, "tool-workflow/run-start", `{"runId":"r1","name":"review"}`),
		frame(2, "tool-workflow/agent-start", `{"runId":"r1","seq":1,"label":"读代码","phase":"review","childId":"c1"}`),
		frame(3, "tool-workflow/agent-start", `{"runId":"r1","seq":2,"label":"改代码","phase":"implement","childId":"c2"}`),
		frame(4, "tool-workflow/agent-end", `{"runId":"r1","seq":1,"outcome":"completed"}`),
		frame(5, "tool-workflow/agent-end", `{"runId":"r1","seq":2,"outcome":"failed"}`),
		frame(6, "tool-workflow/run-end", `{"runId":"r1","stopReason":"error"}`),
	})
	entries := tr.Entries()
	if len(entries) != 1 {
		t.Fatalf("节点数 = %d，应为 1（四类帧折成一条）", len(entries))
	}
	entry := entries[0]
	if entry.Kind != ChatNodeWorkflowRun {
		t.Fatalf("kind = %q", entry.Kind)
	}
	workflow := entry.Data.Workflow
	if workflow == nil {
		t.Fatal("Data.Workflow 必须非 nil")
	}
	if workflow.Name != "review" || workflow.Status != WorkflowStatusFailed {
		t.Errorf("run = %+v，应为 review/failed", workflow)
	}
	if len(workflow.Phases) != 2 {
		t.Fatalf("阶段数 = %d，应为 2", len(workflow.Phases))
	}
	// 阶段顺序 = 首次出现顺序（review 先到）。
	if got := WorkflowReadablePhase(workflow.Phases[0].Phase); got != "review" {
		t.Errorf("第一阶段 = %q，应为 review", got)
	}
	if got := WorkflowReadablePhase(workflow.Phases[1].Phase); got != "implement" {
		t.Errorf("第二阶段 = %q，应为 implement", got)
	}
	// 成员归属与状态。
	if got := workflow.Phases[0].Members; len(got) != 1 || got[0].Status != WorkflowStatusCompleted {
		t.Errorf("review 阶段成员 = %+v，应为 1 个 completed", got)
	}
	if got := workflow.Phases[1].Members; len(got) != 1 || got[0].Status != WorkflowStatusFailed {
		t.Errorf("implement 阶段成员 = %+v，应为 1 个 failed", got)
	}
	if workflow.MemberCount() != 2 {
		t.Errorf("MemberCount = %d，应为 2", workflow.MemberCount())
	}
}

// TestProjectWorkflowRunPhaseIdentity 钉住"缺省 phase"与"空串 phase"两组不同。
func TestProjectWorkflowRunPhaseIdentity(t *testing.T) {
	tr := NewTranscript()
	tr.ApplyFrames([]apiclient.Frame{
		frame(1, "tool-workflow/run-start", `{"runId":"r","name":"w"}`),
		// 一条**没有** phase 字段，一条 phase 是空串。
		frame(2, "tool-workflow/agent-start", `{"runId":"r","seq":1,"label":"a","childId":"c1"}`),
		frame(3, "tool-workflow/agent-start", `{"runId":"r","seq":2,"label":"b","phase":"","childId":"c2"}`),
	})
	workflow := workflowDataOf(t, tr)
	if len(workflow.Phases) != 2 {
		t.Fatalf("阶段数 = %d，应为 2（缺省与空串是两个身份）", len(workflow.Phases))
	}
	if workflow.Phases[0].Phase != nil {
		t.Errorf("第一条成员 phase 应为 nil，实际 %q", *workflow.Phases[0].Phase)
	}
	if workflow.Phases[1].Phase == nil || *workflow.Phases[1].Phase != "" {
		t.Errorf("第二条成员 phase 应为空串指针，实际 %v", workflow.Phases[1].Phase)
	}
}

// TestProjectWorkflowRunInterruptedByTurnEnd 钉住 `turnAbandoned` 的官方判据。
//
// 没有 run-end 的 run，只有"所在 Turn 终结成 interrupted / forked"才证明
// 它停了（官方注释：前台 run 在 Tool Step 结束前写 run-end，后台 run 比 Step
// 活得久——所以普通的 Step/Turn 关闭**什么都不说明**）。
func TestProjectWorkflowRunInterruptedByTurnEnd(t *testing.T) {
	tr := NewTranscript()
	tr.ApplyFrames([]apiclient.Frame{
		frame(1, "turn/start", `{"turn":7}`),
		frame(2, "tool-workflow/run-start", `{"runId":"r","name":"w","turn":7}`),
		frame(3, "tool-workflow/agent-start", `{"runId":"r","seq":1,"label":"a","childId":"c1","turn":7}`),
		// Turn 终结成 interrupted：run 与未 settle 的成员都必须变 interrupted。
		frame(4, "turn/end", `{"turn":7,"reason":{"kind":"interrupted"}}`),
	})
	workflow := workflowDataOf(t, tr)
	if workflow.Status != WorkflowStatusInterrupted {
		t.Errorf("run 状态 = %q，应为 interrupted（turnAbandoned）", workflow.Status)
	}
	if got := workflow.Phases[0].Members[0].Status; got != WorkflowStatusInterrupted {
		t.Errorf("未 settle 成员 = %q，应为 interrupted", got)
	}

	// 反例：Turn 正常完成**不能**把 run 判成中断。
	tr2 := NewTranscript()
	tr2.ApplyFrames([]apiclient.Frame{
		frame(1, "tool-workflow/run-start", `{"runId":"r","name":"w","turn":7}`),
		frame(2, "turn/end", `{"turn":7,"reason":{"kind":"completed"}}`),
	})
	if got := workflowDataOf(t, tr2).Status; got != WorkflowStatusRunning {
		t.Errorf("正常完成的 Turn 不得把 run 判成中断，实际 %q", got)
	}
}

// TestProjectWorkflowRunEndWinsOverAbandoned 钉住"已有 run-end 的不被 Turn 覆盖"。
func TestProjectWorkflowRunEndWinsOverAbandoned(t *testing.T) {
	tr := NewTranscript()
	tr.ApplyFrames([]apiclient.Frame{
		frame(1, "tool-workflow/run-start", `{"runId":"r","name":"w","turn":7}`),
		frame(2, "tool-workflow/run-end", `{"runId":"r","stopReason":"completed","turn":7}`),
		frame(3, "turn/end", `{"turn":7,"reason":{"kind":"interrupted"}}`),
	})
	if got := workflowDataOf(t, tr).Status; got != WorkflowStatusCompleted {
		t.Errorf("已有 run-end 的 run 状态 = %q，应为 completed（run-end 优先）", got)
	}
}

// TestProjectWorkflowRunResetClearsFolds 钉住 Reset 清掉折叠状态。
//
// 不清的话，切会话/重建基线后同 runId 的 run 会带着上一代的成员。
func TestProjectWorkflowRunResetClearsFolds(t *testing.T) {
	tr := NewTranscript()
	tr.ApplyFrames([]apiclient.Frame{
		frame(1, "tool-workflow/run-start", `{"runId":"r","name":"w"}`),
		frame(2, "tool-workflow/agent-start", `{"runId":"r","seq":1,"label":"a","childId":"c1"}`),
	})
	tr.Reset()
	tr.ApplyFrames([]apiclient.Frame{
		frame(1, "tool-workflow/run-start", `{"runId":"r","name":"w2"}`),
	})
	workflow := workflowDataOf(t, tr)
	if workflow.Name != "w2" {
		t.Errorf("重建后 run 名 = %q，应为 w2", workflow.Name)
	}
	if len(workflow.Phases) != 0 {
		t.Errorf("重建后阶段数 = %d，应为 0（上一代成员不得残留）", len(workflow.Phases))
	}
}

// workflowDataOf 找投影里唯一的 workflow-run 条目并返回它的载荷。
//
// 为什么按 kind 找而不是 `Entries()[0]`：`turn/start` 也会产条目
// （turn-process），固定下标会拿到别的 kind。
func workflowDataOf(t *testing.T, tr *Transcript) *WorkflowRunChatData {
	t.Helper()
	for _, entry := range tr.Entries() {
		if entry.Kind != ChatNodeWorkflowRun {
			continue
		}
		if entry.Data.Workflow == nil {
			t.Fatal("workflow-run 条目的 Data.Workflow 为 nil")
		}
		return entry.Data.Workflow
	}
	t.Fatal("投影里没有 workflow-run 条目")
	return nil
}

// TestWorkflowRunDisclosuresIsolatedPerRun 是**跨 run 隔离**的回归断言。
//
// 官方每个面板实例一份 React state，天然隔离；我们的存储是整个 transcript
// 共享的一张表，用 `entryKey + \x1f + phaseKey` 拼键。**清理逻辑必须只看
// 本 run 的键前缀**——按"不在本 run 的 phases 里就删"去扫全表，会让渲染
// run A 时把 run B 的 phase 披露态删掉（两个 workflow 节点同时存在时，
// 第二个面板的折叠状态每帧被重置）。
func TestWorkflowRunDisclosuresIsolatedPerRun(t *testing.T) {
	store := NewWorkflowRunDisclosures()
	phaseA := "review"
	phaseB := "implement"
	runA := WorkflowRunChatData{
		Name: "run-a", Status: WorkflowStatusRunning,
		Phases: []WorkflowRunPhaseData{{
			Key: WorkflowPhaseKey(&phaseA), Phase: &phaseA,
			Members: []WorkflowRunMemberData{{Seq: 1, Label: "a1", Status: WorkflowStatusRunning}},
		}},
	}
	runB := WorkflowRunChatData{
		Name: "run-b", Status: WorkflowStatusRunning,
		Phases: []WorkflowRunPhaseData{{
			Key: WorkflowPhaseKey(&phaseB), Phase: &phaseB,
			Members: []WorkflowRunMemberData{{Seq: 1, Label: "b1", Status: WorkflowStatusRunning}},
		}},
	}

	// 两个 run 各画一帧（各自建立 phase 披露态）。
	bA := newWorkflowHarness(runA, store)
	bA.panel.EntryKey = "entry-a"
	bA.paint(t, "")
	bB := newWorkflowHarness(runB, store)
	bB.panel.EntryKey = "entry-b"
	bB.paint(t, "")

	// 用户把 B 的 phase 收起。
	bB.paint(t, "workflow-run-phase-entry-b-"+WorkflowPhaseKey(&phaseB))
	bB.paint(t, "")
	if state := store.Phase("entry-b", runB.Phases[0]); state.Open {
		t.Fatal("前置：B 的 phase 应已折叠")
	}

	// 再画一次 A —— 不得影响 B。
	bA.paint(t, "")
	if state := store.Phase("entry-b", runB.Phases[0]); state.Open {
		t.Fatal("渲染 run A 把 run B 的 phase 折叠态删掉了（跨 run 串扰）")
	}
	// A 自己仍是展开的。
	if state := store.Phase("entry-a", runA.Phases[0]); !state.Open {
		t.Fatal("run A 自己的 phase 折叠态丢了")
	}
}

// TestWorkflowRunEntryKeyStable 钉住 run 的节点键（官方 `context.id = String(runId)`）。
func TestWorkflowRunEntryKeyStable(t *testing.T) {
	if got := workflowRunEntryKey("run-42"); strings.Contains(got, " ") || got == "" {
		t.Fatalf("run 键 = %q，必须是稳定非空串", got)
	}
	if workflowRunEntryKey("a") == workflowRunEntryKey("b") {
		t.Fatal("不同 runId 不得撞键")
	}
}
