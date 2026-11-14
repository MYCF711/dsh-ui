package dsdeliverables

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

// changesFake 是壳那侧数据面的测试替身，同时记调用次数（缓存判据要用）。
type changesFake struct {
	summary    *apiclient.ChangesSummary
	summaryErr error
	diff       *apiclient.ChangesDiff
	diffErr    error

	summaryCalls int
	diffCalls    int
	lastIndex    int
	openCalls    int
}

func (f *changesFake) Summary(context.Context, string, int) (*apiclient.ChangesSummary, error) {
	f.summaryCalls++
	return f.summary, f.summaryErr
}

func (f *changesFake) Diff(_ context.Context, _ string, _, index int) (*apiclient.ChangesDiff, error) {
	f.diffCalls++
	f.lastIndex = index
	return f.diff, f.diffErr
}

func (f *changesFake) Open(context.Context, string, int, int, string, string) error {
	f.openCalls++
	return nil
}

// changesEnv 造一帧只带"当前会话 + 一条 workspace/changes 帧"的环境。
func changesEnv(session string, seq int) *pluginapi.Env {
	return &pluginapi.Env{Data: pluginapi.Snapshot{
		Cur: session,
		Frames: []apiclient.Frame{
			{Seq: 1, Type: "user/message"},
			{Seq: seq, Type: ChangesFrameType},
		},
	}}
}

func missingErr() error {
	return &apiclient.ChangesError{Status: 404, Kind: apiclient.ChangesStatusMissing, Code: "not_found"}
}

// TestLatestChangesSeqTakesNewest 验：seq 只从 `workspace/changes` 帧里取**最新**一条。
//
// 取错 seq 的后果不是报错而是 404——然后被误当成"这一轮没有变更"，
// 所以这条测试守的是"别把别的帧的 seq 当自己的"。
func TestLatestChangesSeqTakesNewest(t *testing.T) {
	frames := []apiclient.Frame{
		{Seq: 3, Type: ChangesFrameType},
		{Seq: 5, Type: "assistant/message"},
		{Seq: 9, Type: ChangesFrameType},
		{Seq: 7, Type: ChangesFrameType}, // 乱序也要取最大那条
	}
	seq, ok := LatestChangesSeq(frames)
	if !ok || seq != 9 {
		t.Fatalf("seq = %d/%v，want 9/true", seq, ok)
	}
	if _, ok := LatestChangesSeq([]apiclient.Frame{{Seq: 4, Type: "user/message"}}); ok {
		t.Fatal("没有 workspace/changes 帧时不该给出 seq（拿它去问只会 404）")
	}
}

// TestChangedIndexMatchesPathForms 验路径匹配：相对路径、display、cwd 拼绝对路径、
// 绝对后缀都能命中；**同名多义时报 false**（宁可退回全文预览，也不能画错文件的 diff）。
func TestChangedIndexMatchesPathForms(t *testing.T) {
	files := []apiclient.ChangesFile{
		{Path: "src/a.go", Display: "src/a.go"},
		{Path: "docs/b.md", Display: "文档/b.md"},
		{Path: "src/c.go", Display: "src/c.go"},
	}
	cases := []struct {
		name    string
		clicked string
		cwd     string
		want    int
		ok      bool
	}{
		{"相对路径逐字命中", "src/a.go", "", 0, true},
		{"反斜杠折成正斜杠", `src\a.go`, "", 0, true},
		{"display 命中", "文档/b.md", "", 1, true},
		{"cwd 拼绝对路径", `D:\w\src\c.go`, `D:\w`, 2, true},
		{"cwd 未知时按后缀", "/home/u/proj/src/a.go", "", 0, true},
		{"只给文件名且唯一", "b.md", "", 1, true},
		{"摘要里没有这个文件", "src/z.go", "", 0, false},
		{"空路径", "", "", 0, false},
	}
	for _, tc := range cases {
		got, ok := ChangedIndex(files, tc.clicked, tc.cwd)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Fatalf("%s: ChangedIndex(%q, cwd=%q) = %d/%v，want %d/%v",
				tc.name, tc.clicked, tc.cwd, got, ok, tc.want, tc.ok)
		}
	}
	// 同名多义（两个目录里都有 a.go）：**不猜**，宁可退回全文预览。
	ambiguous := []apiclient.ChangesFile{{Path: "x/a.go", Display: "x/a.go"}, {Path: "y/a.go", Display: "y/a.go"}}
	if got, ok := ChangedIndex(ambiguous, "a.go", ""); ok {
		t.Fatalf("同名多义时必须返回 false，得到 index=%d", got)
	}
}

// TestHunkRowsNumbersBothSides 验行号：上下文两侧都数、`+` 只数新侧、`-` 只数旧侧
// （官方 `hunkRows`，OFF:FileDiff.tsx:66-77），并且正文去掉首字符。
func TestHunkRowsNumbersBothSides(t *testing.T) {
	rows := hunkRows(apiclient.ChangesHunk{
		OldStart: 10, OldLines: 3, NewStart: 20, NewLines: 4,
		Lines: []string{" keep", "-old", "+new", "+extra", " tail"},
	})
	if len(rows) != 6 {
		t.Fatalf("行数 = %d，want 6（1 个 hunk 头 + 5 行）", len(rows))
	}
	if rows[0].Kind != rowHunk || rows[0].Text != "@@ -10,3 +20,4 @@" {
		t.Fatalf("hunk 头 = %+v（官方形状 OFF:FileDiff.tsx:165-167）", rows[0])
	}
	want := []diffRow{
		{Kind: rowContext, OldNo: 10, NewNo: 20, Text: "keep"},
		{Kind: rowDel, OldNo: 11, Text: "old"},
		{Kind: rowAdd, NewNo: 21, Text: "new"},
		{Kind: rowAdd, NewNo: 22, Text: "extra"},
		{Kind: rowContext, OldNo: 12, NewNo: 23, Text: "tail"},
	}
	for i, w := range want {
		if rows[i+1] != w {
			t.Fatalf("第 %d 行 = %+v，want %+v", i+1, rows[i+1], w)
		}
	}
}

// TestDiffRowsTruncatesAtBudget 验渲染预算：到 300 行就停，而且**停在半路**
// （不跳过剩下的 hunk 继续画——那会让行号看起来连续，中间其实缺了一段）。
func TestDiffRowsTruncatesAtBudget(t *testing.T) {
	hunk := apiclient.ChangesHunk{OldStart: 1, OldLines: maxDiffRows + 5, NewStart: 1, NewLines: maxDiffRows + 5}
	for i := 0; i < maxDiffRows+5; i++ {
		hunk.Lines = append(hunk.Lines, " x")
	}
	rows, truncated := diffRows([]apiclient.ChangesHunk{hunk})
	if !truncated || len(rows) != maxDiffRows {
		t.Fatalf("截断 = %v/%d 行，want true/%d", truncated, len(rows), maxDiffRows)
	}
	if rows[len(rows)-1].Kind == rowHunk {
		t.Fatal("最后一行是 hunk 头：说明截断把整段丢掉了")
	}
	if small, truncated := diffRows([]apiclient.ChangesHunk{{Lines: []string{" a"}}}); truncated || len(small) != 2 {
		t.Fatalf("小 diff 不该截断：%v/%d", truncated, len(small))
	}
}

// TestPlanOverlayNoRecordFallsBackToFullText 是这一条的核心判据：
// **404 / 没接数据源 / 非 git 会话** 一律退回全文预览，并写明"这一轮没有变更记录"，
// 而且**一行 diff 都不画**（画空框会让人以为"这个文件没改"）。
func TestPlanOverlayNoRecordFallsBackToFullText(t *testing.T) {
	plan := planOverlay(changesView{Outcome: outcomeNoRecord}, "a.go")
	if len(plan.Notes) != 1 || plan.Notes[0] != msgNoChangesRecord {
		t.Fatalf("说明行 = %v，want [%q]", plan.Notes, msgNoChangesRecord)
	}
	// 这是**已登记裁剪**的配套口径：404 要显示成"没有记录这个事实"，
	// 不能是官方那句"这轮改动的内容已不可用"，更不能简化成"没有变更"。
	if plan.Notes[0] == msgDiffMissing || plan.Notes[0] == "没有变更" {
		t.Fatalf("404 的说明行写成了 %q", plan.Notes[0])
	}
	if !strings.Contains(plan.Notes[0], "没有变更记录") {
		t.Fatalf("说明行没写清「没有变更记录」：%q", plan.Notes[0])
	}
	if !plan.FullText {
		t.Fatal("必须退回全文预览")
	}
	if len(plan.Rows) != 0 || plan.Retry {
		t.Fatalf("不该画 diff 行/重试：%+v", plan)
	}
	if plan.Path != "a.go" {
		t.Fatalf("路径 = %q", plan.Path)
	}
}

// TestPlanOverlayDiffMissingAndFailed 验另两种"没有 diff 可画"的结局：
// diff 404 用官方 `diff.missing`；读失败用官方 `diff.error` **并给重试**。
func TestPlanOverlayDiffMissingAndFailed(t *testing.T) {
	missing := planOverlay(changesView{Outcome: outcomeDiffMissing}, "a.go")
	if len(missing.Notes) != 1 || missing.Notes[0] != msgDiffMissing || !missing.FullText {
		t.Fatalf("diff 404 的结局 = %+v", missing)
	}
	failed := planOverlay(changesView{Outcome: outcomeFailed, Err: errors.New("boom")}, "a.go")
	if len(failed.Notes) != 1 || failed.Notes[0] != msgDiffError || !failed.Retry || !failed.FullText {
		t.Fatalf("读失败的结局 = %+v", failed)
	}
}

// TestPlanOverlayTextDiffDrawsHunks 验正常路径：头部用 display 路径与 +N/-M 计数，
// 正文是 hunk 行，**说明行一条都不加**（官方 noteOf 只在 新建/删除/无差异 时说话）。
func TestPlanOverlayTextDiffDrawsHunks(t *testing.T) {
	v := changesView{
		Outcome: outcomeReady,
		File:    &apiclient.ChangesFile{Path: "src/a.go", Display: "src/a.go", Added: 2, Deleted: 1},
		Diff: &apiclient.ChangesDiff{Kind: "text", Path: "src/a.go", Display: "src/a.go",
			Before: true, After: true,
			Hunks: []apiclient.ChangesHunk{{OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 3,
				Lines: []string{" one", "-two", "+two bis", "+three"}}}},
	}
	plan := planOverlay(v, "fallback")
	if plan.Path != "src/a.go" || !plan.ShowCounts || plan.Added != 2 || plan.Deleted != 1 {
		t.Fatalf("头部 = %q / counts=%v(%d,%d)", plan.Path, plan.ShowCounts, plan.Added, plan.Deleted)
	}
	if len(plan.Notes) != 0 {
		t.Fatalf("普通改动不该有说明行：%v", plan.Notes)
	}
	if plan.FullText || plan.Retry {
		t.Fatalf("有 diff 就不该退全文：%+v", plan)
	}
	if len(plan.Rows) != 5 {
		t.Fatalf("行数 = %d，want 5（1 个头 + 4 行）", len(plan.Rows))
	}
}

// TestPlanOverlayNotesForCreatedDeletedUnchanged 验官方三条 note
// （OFF:FileDiff.tsx:128-133）：新文件、被删文件、两侧相同。
func TestPlanOverlayNotesForCreatedDeletedUnchanged(t *testing.T) {
	file := &apiclient.ChangesFile{Path: "a.go", Display: "a.go"}
	cases := []struct {
		name  string
		diff  *apiclient.ChangesDiff
		notes []string
		rows  int
	}{
		{"新建", &apiclient.ChangesDiff{Kind: "text", Path: "a.go", Display: "a.go", After: true,
			Hunks: []apiclient.ChangesHunk{{NewStart: 1, NewLines: 1, Lines: []string{"+hi"}}}},
			[]string{msgDiffCreated}, 2},
		{"删除", &apiclient.ChangesDiff{Kind: "text", Path: "a.go", Display: "a.go", Before: true,
			Hunks: []apiclient.ChangesHunk{{OldStart: 1, OldLines: 1, Lines: []string{"-hi"}}}},
			[]string{msgDiffDeleted}, 2},
		{"两侧相同", &apiclient.ChangesDiff{Kind: "text", Path: "a.go", Display: "a.go", Before: true, After: true},
			[]string{msgDiffUnchanged}, 0},
	}
	for _, tc := range cases {
		plan := planOverlay(changesView{Outcome: outcomeReady, File: file, Diff: tc.diff}, "a.go")
		if len(plan.Notes) != len(tc.notes) {
			t.Fatalf("%s: 说明行 = %v，want %v", tc.name, plan.Notes, tc.notes)
		}
		for i, n := range tc.notes {
			if plan.Notes[i] != n {
				t.Fatalf("%s: 说明行[%d] = %q，want %q", tc.name, i, plan.Notes[i], n)
			}
		}
		if len(plan.Rows) != tc.rows {
			t.Fatalf("%s: 行数 = %d，want %d", tc.name, len(plan.Rows), tc.rows)
		}
		if plan.FullText {
			t.Fatalf("%s: 有 diff 就不该退回全文预览", tc.name)
		}
	}
}

// TestPlanOverlayBinaryAndOversizedDrawNoHunks 验拒绝支**只显示官方那两句状态词**，
// 一行假 hunk 都不画（宿主也没给正文，画出来只能是编的）。
func TestPlanOverlayBinaryAndOversizedDrawNoHunks(t *testing.T) {
	file := &apiclient.ChangesFile{Path: "a.bin", Display: "a.bin", Binary: true}
	plan := planOverlay(changesView{Outcome: outcomeReady, File: file,
		Diff: &apiclient.ChangesDiff{Kind: "binary", Path: "a.bin", Display: "a.bin"}}, "a.bin")
	if len(plan.Notes) != 1 || plan.Notes[0] != msgDiffBinary || len(plan.Rows) != 0 || plan.FullText {
		t.Fatalf("binary 的结局 = %+v", plan)
	}
	if plan.CountsLabel != msgChangesBinary || plan.ShowCounts {
		t.Fatalf("头部状态词 = %q（counts=%v），want %q", plan.CountsLabel, plan.ShowCounts, msgChangesBinary)
	}

	big := &apiclient.ChangesFile{Path: "big.txt", Display: "big.txt", Oversized: true}
	plan = planOverlay(changesView{Outcome: outcomeReady, File: big,
		Diff: &apiclient.ChangesDiff{Kind: "oversized", Path: "big.txt", Display: "big.txt"}}, "big.txt")
	if len(plan.Notes) != 1 || plan.Notes[0] != msgDiffOversized || len(plan.Rows) != 0 {
		t.Fatalf("oversized 的结局 = %+v", plan)
	}
}

// TestPlanOverlayTruncationNoteStatesTheDrawnCount 验截断说明行里的数字是**我们真的画了几行**，
// 不是官方那个 DOM 预算（说错了就是撒谎）。
func TestPlanOverlayTruncationNoteStatesTheDrawnCount(t *testing.T) {
	h := apiclient.ChangesHunk{OldStart: 1, OldLines: maxDiffRows + 1, NewStart: 1, NewLines: maxDiffRows + 1}
	for i := 0; i < maxDiffRows+1; i++ {
		h.Lines = append(h.Lines, " x")
	}
	plan := planOverlay(changesView{Outcome: outcomeReady,
		File: &apiclient.ChangesFile{Path: "a", Display: "a"},
		Diff: &apiclient.ChangesDiff{Kind: "text", Path: "a", Display: "a", Before: true, After: true,
			Hunks: []apiclient.ChangesHunk{h}}}, "a")
	want := "只显示前 300 行"
	if len(plan.Notes) != 1 || plan.Notes[0] != want {
		t.Fatalf("截断说明 = %v，want [%q]", plan.Notes, want)
	}
	if len(plan.Rows) != maxDiffRows {
		t.Fatalf("行数 = %d，want %d", len(plan.Rows), maxDiffRows)
	}
}

// TestFetchChangesMapsHostAnswers 验数据面的四种宿主回答各自翻成哪一档：
// 404（摘要）→ 没有记录；摘要里没有这个文件 → 没有记录；diff 404 → diff 缺失；别的错 → 可重试的失败。
func TestFetchChangesMapsHostAnswers(t *testing.T) {
	key := changesKey{session: "s1", seq: 9, path: "src/a.go"}

	// 摘要 404：非 git 会话的典型回答。
	v := fetchChanges(&changesFake{summaryErr: missingErr()}, key)
	if v.Outcome != outcomeNoRecord {
		t.Fatalf("摘要 404 → %v，want outcomeNoRecord", v.Outcome)
	}

	// 宿主没接线（501）：对用户同样是"没有记录源"。
	v = fetchChanges(&changesFake{summaryErr: &apiclient.ChangesError{
		Status: 501, Kind: apiclient.ChangesStatusNotWired, Code: "not_implemented"}}, key)
	if v.Outcome != outcomeNoRecord {
		t.Fatalf("501 → %v，want outcomeNoRecord", v.Outcome)
	}

	// 摘要里有别的文件、没有这个：对这个文件就是"没有记录"。
	v = fetchChanges(&changesFake{summary: &apiclient.ChangesSummary{Turn: 1,
		Files: []apiclient.ChangesFile{{Path: "other.go", Display: "other.go"}}}}, key)
	if v.Outcome != outcomeNoRecord {
		t.Fatalf("摘要里没有该文件 → %v，want outcomeNoRecord", v.Outcome)
	}

	// diff 404：有摘要、比较已被丢弃 → 官方 diff.missing。
	f := &changesFake{
		summary: &apiclient.ChangesSummary{Turn: 1, Files: []apiclient.ChangesFile{{Path: "src/a.go", Display: "src/a.go"}}},
		diffErr: missingErr(),
	}
	v = fetchChanges(f, key)
	if v.Outcome != outcomeDiffMissing || f.lastIndex != 0 {
		t.Fatalf("diff 404 → %v（index=%d），want outcomeDiffMissing/0", v.Outcome, f.lastIndex)
	}

	// 传输失败：可重试那一档，带上原错误。
	f = &changesFake{
		summary: &apiclient.ChangesSummary{Turn: 1, Files: []apiclient.ChangesFile{{Path: "src/a.go", Display: "src/a.go"}}},
		diffErr: errors.New("连不上"),
	}
	if v = fetchChanges(f, key); v.Outcome != outcomeFailed || v.Err == nil {
		t.Fatalf("传输失败 → %+v，want outcomeFailed 带错误", v)
	}

	// 正常：拿到的 File 就是摘要里那个下标的那条（不是按路径重造的对象）。
	want := apiclient.ChangesFile{Path: "src/a.go", Display: "src/a.go", Added: 3, Deleted: 1}
	f = &changesFake{
		summary: &apiclient.ChangesSummary{Turn: 1, Files: []apiclient.ChangesFile{want}},
		diff:    &apiclient.ChangesDiff{Kind: "text", Path: "src/a.go", Display: "src/a.go", Before: true, After: true},
	}
	if v = fetchChanges(f, key); v.Outcome != outcomeReady || v.File == nil || *v.File != want {
		t.Fatalf("正常路径 → %+v", v)
	}
}

// TestResolveChangesCacheAndNoSource 验缓存判据：
//   - 没接数据源时**不缓存**（壳可能在 Load 之后才注入，下一帧就该恢复）；
//   - 会话里没有 workspace/changes 帧 = 没记录；
//   - 同一个 (会话, seq, 路径) 只拉一次，seq/路径一变就重拉。
func TestResolveChangesCacheAndNoSource(t *testing.T) {
	p := &Plugin{preview: "src/a.go"}
	env := changesEnv("s1", 9)

	SetChangesSource(nil)
	if v := p.resolveChanges(env); v.Outcome != outcomeNoRecord {
		t.Fatalf("没接数据源 → %v，want outcomeNoRecord", v.Outcome)
	}

	f := &changesFake{
		summary: &apiclient.ChangesSummary{Turn: 1, Files: []apiclient.ChangesFile{
			{Path: "src/a.go", Display: "src/a.go"},
			{Path: "docs/b.md", Display: "docs/b.md"},
		}},
		diff: &apiclient.ChangesDiff{Kind: "text", Path: "src/a.go", Display: "src/a.go", Before: true, After: true},
	}
	SetChangesSource(f)
	t.Cleanup(func() { SetChangesSource(nil) })

	p.changesValid = false
	if v := p.resolveChanges(env); v.Outcome != outcomeReady {
		t.Fatalf("接了数据源 → %v，want outcomeReady", v.Outcome)
	}
	p.resolveChanges(env)
	p.resolveChanges(env)
	if f.summaryCalls != 1 || f.diffCalls != 1 {
		t.Fatalf("同一 (会话,seq,路径) 被重复拉取：summary=%d diff=%d", f.summaryCalls, f.diffCalls)
	}

	// 换 seq（新的一轮）：必须重拉，否则会把上一轮的 diff 画在这一轮上。
	p.resolveChanges(changesEnv("s1", 12))
	if f.summaryCalls != 2 || f.diffCalls != 2 {
		t.Fatalf("seq 变了却没重拉：summary=%d diff=%d", f.summaryCalls, f.diffCalls)
	}
	// 换文件：同样重拉。
	p.preview = "docs/b.md"
	p.resolveChanges(changesEnv("s1", 12))
	if f.summaryCalls != 3 || f.diffCalls != 3 {
		t.Fatalf("路径变了却没重拉：summary=%d diff=%d", f.summaryCalls, f.diffCalls)
	}
	// 换会话：重拉。
	p.resolveChanges(changesEnv("s2", 12))
	if f.summaryCalls != 4 {
		t.Fatalf("会话变了却没重拉：summary=%d", f.summaryCalls)
	}
}

// TestResolveChangesNoFrameAndNoSession 验两个"不猜"的前置：
// 会话里没有 workspace/changes 帧（非 git 会话）→ 没有记录；
// 还不知道当前会话 id → 没有记录（猜一个 id 就是把别人的 diff 画上来）。
func TestResolveChangesNoFrameAndNoSession(t *testing.T) {
	f := &changesFake{summary: &apiclient.ChangesSummary{Turn: 1}}
	SetChangesSource(f)
	t.Cleanup(func() { SetChangesSource(nil) })

	p := &Plugin{preview: "src/a.go"}
	noFrame := &pluginapi.Env{Data: pluginapi.Snapshot{Cur: "s1", Frames: []apiclient.Frame{{Seq: 1, Type: "user/message"}}}}
	if v := p.resolveChanges(noFrame); v.Outcome != outcomeNoRecord {
		t.Fatalf("没有 workspace/changes 帧 → %v", v.Outcome)
	}
	noSession := &pluginapi.Env{Data: pluginapi.Snapshot{Frames: []apiclient.Frame{{Seq: 9, Type: ChangesFrameType}}}}
	if v := p.resolveChanges(noSession); v.Outcome != outcomeNoRecord {
		t.Fatalf("不知道会话 id → %v", v.Outcome)
	}
	if f.summaryCalls != 0 {
		t.Fatalf("这两种情况都不该发请求，实际 %d 次", f.summaryCalls)
	}
	if v := p.resolveChanges(nil); v.Outcome != outcomeNoRecord {
		t.Fatalf("env 为 nil → %v", v.Outcome)
	}
}

// TestInvalidateChangesRefetches 验「重试」丢的只是缓存，下一次解析会重拉。
func TestInvalidateChangesRefetches(t *testing.T) {
	f := &changesFake{
		summary: &apiclient.ChangesSummary{Turn: 1, Files: []apiclient.ChangesFile{{Path: "src/a.go", Display: "src/a.go"}}},
		diff:    &apiclient.ChangesDiff{Kind: "text", Path: "src/a.go", Display: "src/a.go", Before: true, After: true},
	}
	SetChangesSource(f)
	t.Cleanup(func() { SetChangesSource(nil) })

	p := &Plugin{preview: "src/a.go"}
	env := changesEnv("s1", 9)
	p.resolveChanges(env)
	p.invalidateChanges()
	p.resolveChanges(env)
	if f.summaryCalls != 2 || f.diffCalls != 2 {
		t.Fatalf("重试没重拉：summary=%d diff=%d", f.summaryCalls, f.diffCalls)
	}
}

// TestChangesSourceContractCoversThreeRoutes 验契约面本身：
// `*changesFake` 实现了 ChangesSource（三条路由都在），
// 壳接上去就有一个编译期判据，不会出现"实现了两条忘了第三条"。
func TestChangesSourceContractCoversThreeRoutes(t *testing.T) {
	var _ ChangesSource = (*changesFake)(nil)
	SetChangesSource(nil)
	if currentChangesSource() != nil {
		t.Fatal("注入 nil 之后不该还有数据源")
	}
}
