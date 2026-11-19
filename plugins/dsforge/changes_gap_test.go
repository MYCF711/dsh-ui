//go:build darwin || windows || linux

// changes_gap_test.go —— #64 ① 「未变更行 gap 可点开」的判据。
//
// 三层：
//
//	① 纯函数：`changesSegmentsOf()` 与官方 `diff-rows.ts:151-184` 逐条同形
//	   （gap 计数、展开整段、末尾 gap 的 null 计数、endsNewSide）。
//	② **真实文件**：临时 git 仓库 → 真 `git diff` 的 hunk + 真工作树文件 →
//	   展开出来的每一行必须等于磁盘上那一行（不是拿夹具自证）。
//	③ 真实像素帧：gap 条纹可点、点开后面板里出现真实正文；请求在飞时不可点；
//	   官方没有单 gap 收起路径，第二次点击不改展开态。
package dsforge

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// gapNumberedLines 造 n 行 "L01".."Lnn"（宽 2，够 30 行用）。
func gapNumberedLines(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("L%02d", i+1)
	}
	return lines
}

func gapContextLines(all []string, from, to int) []string {
	out := make([]string, 0, to-from+1)
	for at := from; at <= to; at++ {
		out = append(out, " "+all[at-1])
	}
	return out
}

func gapHunk(oldStart, oldLines, newStart, newLines int, lines ...string) apiclient.ForgeHunk {
	return apiclient.ForgeHunk{OldStart: oldStart, OldLines: oldLines, NewStart: newStart, NewLines: newLines, Lines: lines}
}

// gapThreeHunkFile 是三个 hunk 的修改文件（hunk 落在 4-6 / 11-13 / 21-23）。
//
// 对应的 gap 依次是：1-3（3 行）、7-10（4 行）、14-20（7 行）、24-30（7 行）。
func gapThreeHunkFile() (apiclient.ForgeDiffFile, []string) {
	all := gapNumberedLines(30)
	file := apiclient.ForgeDiffFile{
		Path: "a.txt", Status: apiclient.ForgeFileModified, Additions: 3, Deletions: 3, Digest: "d-gap",
		Hunks: []apiclient.ForgeHunk{
			gapHunk(4, 3, 4, 3, gapContextLines(all, 4, 6)...),
			// oldStart 比 newStart 大 1：此前有一行新增，旧号偏移 +1。
			gapHunk(12, 3, 11, 3, gapContextLines(all, 11, 13)...),
			gapHunk(21, 3, 21, 3, gapContextLines(all, 21, 23)...),
		},
	}
	return file, all
}

// ── ① 纯函数：段模型 ────────────────────────────────────────────────────

func TestSegmentsOfSplitsBeforeBetweenAndAfterHunks(t *testing.T) {
	file, all := gapThreeHunkFile()
	segments := changesSegmentsOf(file, all, nil)
	// 期望：gap(0,3) rows gap(1,4) rows gap(2,7) rows gap(3,7)。
	want := []struct {
		kind  changesSegmentKind
		index int
		count int // -1 = null
	}{
		{changesSegmentGap, 0, 3},
		{changesSegmentRows, 0, 0},
		{changesSegmentGap, 1, 4},
		{changesSegmentRows, 0, 0},
		{changesSegmentGap, 2, 7},
		{changesSegmentRows, 0, 0},
		{changesSegmentGap, 3, 7},
	}
	if len(segments) != len(want) {
		t.Fatalf("段数 = %d，要 %d：%+v", len(segments), len(want), segments)
	}
	for i, expect := range want {
		got := segments[i]
		if got.kind != expect.kind {
			t.Fatalf("第 %d 段 kind = %v，要 %v", i, got.kind, expect.kind)
		}
		if got.kind != changesSegmentGap {
			continue
		}
		if got.index != expect.index {
			t.Fatalf("第 %d 段 gap.index = %d，要 %d（官方 index 是它**前面**那个 hunk 的下标）", i, got.index, expect.index)
		}
		if got.count == nil {
			t.Fatalf("第 %d 段 gap.count = null，要 %d", i, expect.count)
		}
		if *got.count != expect.count {
			t.Fatalf("第 %d 段 gap.count = %d，要 %d", i, *got.count, expect.count)
		}
	}
}

// TestSegmentsOfGapCountIsNotOffByOne 钉住旧实现的 off-by-one：
// 旧代码 `gap := hunk.NewStart - prevNewEnd; if gap > 1 { 画 gap-1 }` 会把
// 7-10 那 4 行说成 3 行。官方是 `hunk.newStart - 1 > newEnd` 且 count = to-from+1。
func TestSegmentsOfGapCountIsNotOffByOne(t *testing.T) {
	file, all := gapThreeHunkFile()
	segments := changesSegmentsOf(file, all, nil)
	counts := map[int]int{}
	for _, segment := range segments {
		if segment.kind == changesSegmentGap && segment.count != nil {
			counts[segment.index] = *segment.count
		}
	}
	for index, want := range map[int]int{0: 3, 1: 4, 2: 7, 3: 7} {
		if counts[index] != want {
			t.Fatalf("gap %d 计数 = %d，要 %d（全部：%v）", index, counts[index], want, counts)
		}
	}
}

func TestSegmentsOfOpensTheWholeGapNotAChunk(t *testing.T) {
	file, all := gapThreeHunkFile()
	open := map[int]bool{1: true}
	segments := changesSegmentsOf(file, all, open)
	gaps := 0
	var expanded []changesRow
	for _, segment := range segments {
		if segment.kind == changesSegmentGap {
			gaps++
			if segment.index == 1 {
				t.Fatalf("gap 1 已经点开，不该再出现 gap 段")
			}
			continue
		}
		for _, row := range segment.rows {
			if row.new != nil && *row.new >= 7 && *row.new <= 10 {
				expanded = append(expanded, row)
			}
		}
	}
	if gaps != 3 {
		t.Fatalf("打开 1 个 gap 后剩余 gap 段 = %d，要 3（0/2/3）", gaps)
	}
	// 官方展开**整段**（不是固定行数、不是按块）：7-10 四行全部摊开。
	if len(expanded) != 4 {
		t.Fatalf("gap 1 展开行数 = %d，要 4（整段 7-10）：%+v", len(expanded), expanded)
	}
	for i, row := range expanded {
		wantNew := 7 + i
		wantOld := wantNew + 1 // hunk1 的 oldStart-newStart = +1
		if row.new == nil || *row.new != wantNew || row.old == nil || *row.old != wantOld {
			t.Fatalf("第 %d 个展开行号 = (old=%v,new=%v)，要 (old=%d,new=%d)",
				i, row.old, row.new, wantOld, wantNew)
		}
		if row.text != all[wantNew-1] {
			t.Fatalf("第 %d 个展开正文 = %q，要真实第 %d 行 %q", i, row.text, wantNew, all[wantNew-1])
		}
		if row.sign != " " {
			t.Fatalf("展开行必须是 context（符号空），实得 %q", row.sign)
		}
	}
}

func TestSegmentsOfTrailingGapCountIsNullUntilTextArrives(t *testing.T) {
	file, all := gapThreeHunkFile()
	segments := changesSegmentsOf(file, nil, nil)
	last := segments[len(segments)-1]
	if last.kind != changesSegmentGap || last.index != 3 || last.count != nil {
		t.Fatalf("正文未读到时末尾 gap = %+v，要 index=3 count=null（官方 :179）", last)
	}
	if got := gapText(last); got != copyDiffUnchangedRest {
		t.Fatalf("null 计数文案 = %q，要官方 %q", got, copyDiffUnchangedRest)
	}
	segments = changesSegmentsOf(file, all, nil)
	last = segments[len(segments)-1]
	if last.count == nil || *last.count != 7 {
		t.Fatalf("正文读到后末尾 gap 计数 = %v，要 7（24-30）", last.count)
	}
	if got := gapText(last); got != "7 行未变更" {
		t.Fatalf("计数文案 = %q，要 %q", got, "7 行未变更")
	}
}

func TestSegmentsOfHasNoTrailingGapForAddedDeletedOrFinalNewline(t *testing.T) {
	// 新增/删除文件没有"末尾未变更行"（官方 :178 直接排除）。
	for _, status := range []string{apiclient.ForgeFileAdded, apiclient.ForgeFileDeleted} {
		file := apiclient.ForgeDiffFile{
			Path: "a", Status: status,
			Hunks: []apiclient.ForgeHunk{gapHunk(1, 1, 1, 1, " only")},
		}
		segments := changesSegmentsOf(file, []string{"only", "extra"}, nil)
		for _, segment := range segments {
			if segment.kind == changesSegmentGap {
				t.Fatalf("%s 文件不该有 gap：%+v", status, segment)
			}
		}
	}
	// 最后一个 hunk 的 `\ No newline at end of file` 跟在非删除行后面 = 已经到新侧末尾。
	file := apiclient.ForgeDiffFile{
		Path: "a", Status: apiclient.ForgeFileModified,
		Hunks: []apiclient.ForgeHunk{gapHunk(1, 1, 1, 1, " tail", `\ No newline at end of file`)},
	}
	for _, segment := range changesSegmentsOf(file, []string{"tail", "more"}, nil) {
		if segment.kind == changesSegmentGap {
			t.Fatalf("endsNewSide 为真时不该有末尾 gap：%+v", segment)
		}
	}
}

func TestMatchesHunksAcceptsTheRealNewSideAndRejectsStaleText(t *testing.T) {
	file, all := gapThreeHunkFile()
	lines := append([]string(nil), all...)
	if !changesMatchesHunks(lines, file.Hunks) {
		t.Fatal("真实新侧文本必须通过 matchesHunks")
	}
	lines[4] = "changed after the diff was read"
	if changesMatchesHunks(lines, file.Hunks) {
		t.Fatal("diff 读后文件又改过时必须拒绝（官方 toast.expandStale 的判据）")
	}
}

func TestTextLinesKeepsCarriageReturnsAndDrawnStripsThem(t *testing.T) {
	lines := changesTextLines("a\r\nb\r\n")
	if len(lines) != 2 || lines[0] != "a\r" || lines[1] != "b\r" {
		t.Fatalf("textLines = %q，要保留 \\r 且末尾换行不多一行（官方 :104-108）", lines)
	}
	if got := changesDrawnText(lines[0]); got != "a" {
		t.Fatalf("drawn() = %q，要去掉行尾 \\r", got)
	}
}

// ── ② 真实文件：临时 git 仓库 ───────────────────────────────────────────

func runGapGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// parseGapPatch 把 `git diff -U3` 的输出解析成官方 hunk 形状（测试专用，只处理本用例）。
func parseGapPatch(t *testing.T, patch string) []apiclient.ForgeHunk {
	t.Helper()
	var hunks []apiclient.ForgeHunk
	var current *apiclient.ForgeHunk
	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "@@ ") {
			// `@@ -a,b +c,d @@`；单行 hunk 会省掉 `,b`。
			fields := strings.Fields(strings.TrimSuffix(line, " @@"))
			if len(fields) < 3 {
				t.Fatalf("看不懂的 hunk 头：%q", line)
			}
			oldStart, oldLines := parseGapRange(t, strings.TrimPrefix(fields[1], "-"))
			newStart, newLines := parseGapRange(t, strings.TrimPrefix(fields[2], "+"))
			hunks = append(hunks, apiclient.ForgeHunk{OldStart: oldStart, OldLines: oldLines, NewStart: newStart, NewLines: newLines})
			current = &hunks[len(hunks)-1]
			continue
		}
		if current == nil || line == "" {
			continue
		}
		if line[0] == ' ' || line[0] == '+' || line[0] == '-' || line[0] == '\\' {
			current.Lines = append(current.Lines, line)
		}
	}
	return hunks
}

func parseGapRange(t *testing.T, value string) (int, int) {
	t.Helper()
	parts := strings.SplitN(value, ",", 2)
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		t.Fatalf("看不懂的行号 %q：%v", value, err)
	}
	if len(parts) == 1 {
		return start, 1
	}
	lines, err := strconv.Atoi(parts[1])
	if err != nil {
		t.Fatalf("看不懂的行数 %q：%v", value, err)
	}
	return start, lines
}

// TestGapExpansionShowsTheRealFileLines 是「内容与真实文件一致」的硬判据：
// 真 git 仓库 → 真 `git diff` 的 hunk → 真工作树文件 → 展开的每一行逐字比对磁盘。
func TestGapExpansionShowsTheRealFileLines(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("本机没有 git，跳过真实文件一致性判据")
	}
	dir := t.TempDir()
	runGapGit(t, dir, "init", "-q")
	runGapGit(t, dir, "config", "user.email", "gap@example.test")
	runGapGit(t, dir, "config", "user.name", "gap test")
	runGapGit(t, dir, "config", "core.autocrlf", "false")

	all := gapNumberedLines(60)
	name := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(name, []byte(strings.Join(all, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGapGit(t, dir, "add", "a.txt")
	runGapGit(t, dir, "commit", "-q", "-m", "base")

	// 改三处，保证 hunk 之间确实有未变更行（5 / 25 / 40 行）。
	edited := append([]string(nil), all...)
	edited[4] = "changed at 5"
	edited[24] = "changed at 25"
	edited[39] = "changed at 40"
	if err := os.WriteFile(name, []byte(strings.Join(edited, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := runGapGit(t, dir, "diff", "-U3", "--", "a.txt")
	hunks := parseGapPatch(t, patch)
	if len(hunks) < 2 {
		t.Fatalf("真实 patch 的 hunk 数 = %d，至少要 2 个（否则没有 hunk 间 gap）\n%s", len(hunks), patch)
	}

	// 工作树文件就是 `localText(unstaged/uncommitted)` 会读到的那份原文。
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	lines := changesTextLines(string(raw))
	if len(lines) != 60 {
		t.Fatalf("工作树行数 = %d，要 60", len(lines))
	}
	if !changesMatchesHunks(lines, hunks) {
		t.Fatalf("真实工作树文本没通过 matchesHunks —— 说明取的不是新侧原文\npatch:\n%s", patch)
	}

	file := apiclient.ForgeDiffFile{Path: "a.txt", Status: apiclient.ForgeFileModified, Hunks: hunks}
	closed := changesSegmentsOf(file, lines, nil)
	var target *changesSegment
	for i, segment := range closed {
		if segment.kind == changesSegmentGap && segment.count != nil && *segment.count > 0 {
			target = &closed[i]
			break
		}
	}
	if target == nil {
		t.Fatalf("真实 patch 里找不到可展开的 gap\n%s", patch)
	}
	open := map[int]bool{target.index: true}
	expanded := changesSegmentsOf(file, lines, open)
	matched := 0
	for _, segment := range expanded {
		if segment.kind != changesSegmentRows {
			continue
		}
		for _, row := range segment.rows {
			if row.new == nil || row.hunk >= 0 {
				continue
			}
			// 每一行都必须等于磁盘上同一新侧行号的那一行。
			if *row.new-1 >= len(lines) || row.text != lines[*row.new-1] {
				t.Fatalf("展开行 new=%d = %q，磁盘上是 %q", *row.new, row.text, lines[*row.new-1])
			}
			matched++
		}
	}
	if matched < *target.count {
		t.Fatalf("展开出的真实行 = %d，少于 gap 声明的 %d 行", matched, *target.count)
	}
}

// ── ③ 真实像素帧：可点、展开、禁用、不可收起 ────────────────────────────

// gapView 造一份两 hunk 的修改文件，并把**真实新侧全文**放进 `Texts`
// （形状与壳里 `forgeLoadText` 落的一致）。
func gapView() (*pluginapi.ForgeView, []string) {
	all := gapNumberedLines(30)
	file := apiclient.ForgeDiffFile{
		Path: "a.txt", Status: apiclient.ForgeFileModified, Additions: 1, Deletions: 1, Digest: "d-gap-view",
		Hunks: []apiclient.ForgeHunk{
			gapHunk(4, 3, 4, 3, gapContextLines(all, 4, 6)...),
			gapHunk(12, 3, 11, 3, gapContextLines(all, 11, 13)...),
		},
	}
	view := &pluginapi.ForgeView{
		SessionID: "s-1",
		Checkout:  pluginapi.ForgeCheckout{Kind: pluginapi.ForgeCheckoutReady, Branch: "feat/base"},
		Branches:  []string{"main", "feat/base"}, BranchesKnown: true,
		Diff: pluginapi.ForgeDiff{
			SessionID: "s-1", Scope: apiclient.ForgeScopeBranch,
			Files: []apiclient.ForgeDiffFile{file},
			Texts: map[string]string{"a.txt": strings.Join(all, "\n") + "\n"},
		},
	}
	return view, all
}

// drawGapChanges 与 `drawChangesWide` 同构，但 `LoadText` 会像壳一样把真实
// 新侧全文落进 `view.Diff.Texts[path]`（可注入失败与延迟）。
func drawGapChanges(t *testing.T, width float32, view *pluginapi.ForgeView,
	load func(path string)) (*Plugin, *pluginapi.Env, *headless.Session, *app.Invalidator) {
	t.Helper()
	return drawGapChangesPrepared(t, width, view, load, nil)
}

func drawGapChangesPrepared(t *testing.T, width float32, view *pluginapi.ForgeView,
	load func(path string), prepare func(*Plugin)) (*Plugin, *pluginapi.Env, *headless.Session, *app.Invalidator) {
	t.Helper()
	theme := dscore.Dark()
	p := &Plugin{}
	if err := p.Register(testContext{theme: theme}); err != nil {
		t.Fatal(err)
	}
	if prepare != nil {
		prepare(p)
	}
	tree := &gfx.Tree{}
	feat := tree.Feature("forge-gap-headless")
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{
		Title: "forge-gap", Width: 1182, Height: 832, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: feat,
		Bounds:    gfx.Rect{Width: 1182, Height: 832},
		MainWidth: 1182 - width,
		Geometry:  pluginapi.Geometry{Width: 1182, Height: 832, RightWidth: width, MainWidth: 1182 - width},
	}
	var act pluginapi.Actions
	act.Forge = &pluginapi.ForgeActions{
		Refresh: func() {}, LoadBranches: func() {}, LoadScope: func(string) {},
		LoadBases: func(string) {}, LoadText: load,
	}
	act.SelectRightTab = func(string) {}
	env.Act = act
	env.Data = pluginapi.Snapshot{Cur: "s-1", Forge: view}

	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env.Canvas = canvas
		page.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).Width(width).FillHeight().Children(func() {
				p.drawChangesBody(f, env)
			})
		})
		if after := painterAfterDraw(p, "forge-changes-tab"); after != nil {
			after(canvas, env)
		}
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	for i := 0; i < 24; i++ {
		invalidator.Invalidate()
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	return p, env, session, &invalidator
}

func gapHit(session *headless.Session, env *pluginapi.Env, path string, index int) (gfx.Hit, bool) {
	id := gapRowID(env, path, index)
	for _, candidate := range session.Snapshot().Hits {
		if candidate.ID == id {
			return candidate, true
		}
	}
	return gfx.Hit{}, false
}

func gapBodyTexts(session *headless.Session) []string {
	out := make([]string, 0, len(session.Snapshot().Labels))
	for _, label := range session.Snapshot().Labels {
		out = append(out, label.Value)
	}
	return out
}

func gapContains(session *headless.Session, want string) bool {
	for _, text := range gapBodyTexts(session) {
		if strings.Contains(text, want) {
			return true
		}
	}
	return false
}

// gapPump 重画 n 帧（改 view 之后必须 Invalidate：壳是靠 Invalidate 驱动重画的，
// 只 Pump 不 Invalidate 会拿到上一帧的快照）。
func gapPump(t *testing.T, session *headless.Session, invalidator *app.Invalidator, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		invalidator.Invalidate()
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
}

// TestGapRowExpandsRealLinesInUnifiedView：单栏下 gap 是可点的真实 UI 行；
// 点开后面板里必须出现**真实文件**那几行（7-10），而不是编的字。
func TestGapRowExpandsRealLinesInUnifiedView(t *testing.T) {
	view, all := gapView()
	plugin, env, session, invalidator := drawGapChangesPrepared(t, 700, view, func(string) {}, func(p *Plugin) {
		p.open.split["s-1"] = false // 单栏
	})
	if !gapContains(session, "4 行未变更") {
		t.Fatalf("单栏初始帧没有 gap 1 的 %q：\n%v", "4 行未变更", gapBodyTexts(session))
	}
	hit, ok := gapHit(session, env, "a.txt", 1)
	if !ok {
		t.Fatalf("gap 1 没有命中区（画了但点不到）：%v", session.Snapshot().Hits)
	}
	if err := session.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	gapPump(t, session, invalidator, 8)
	if !plugin.open.gapSet("s-1", "a.txt")[1] {
		t.Fatal("点击后 gap 1 没有进入展开集合")
	}
	for _, want := range all[6:10] { // 第 7-10 行
		if !gapContains(session, want) {
			t.Fatalf("展开后缺真实行 %q：\n%v", want, gapBodyTexts(session))
		}
	}
	if gapContains(session, "4 行未变更") {
		t.Fatalf("展开后不该再画 %q：\n%v", "4 行未变更", gapBodyTexts(session))
	}
}

// TestGapRowIsDisabledWhileTheTextLoads：官方 `disabled={text === 'loading'}`
// （DiffFile.tsx:411）——请求在飞时这一条不给命中区。
func TestGapRowIsDisabledWhileTheTextLoads(t *testing.T) {
	view, all := gapView()
	// 壳里的真实行为：请求发出时 Texts 还是空的。
	view.Diff.Texts = nil
	plugin, env, session, invalidator := drawGapChangesPrepared(t, 700, view, func(string) {}, func(p *Plugin) {
		p.open.split["s-1"] = false
	})
	gapPump(t, session, invalidator, 10)
	hit, ok := gapHit(session, env, "a.txt", 1)
	if !ok {
		t.Fatal("还没点之前 gap 应该可点")
	}
	if err := session.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	gapPump(t, session, invalidator, 4)
	if !plugin.open.gapWanted("s-1", "a.txt") {
		t.Fatalf("点击后应进入在飞态（Texts 仍为空）：Texts=%v gaps=%v",
			view.Diff.Texts, plugin.open.gapSet("s-1", "a.txt"))
	}
	if _, ok := gapHit(session, env, "a.txt", 1); ok {
		t.Fatal("在飞时 gap 仍是可点命中区（官方 disabled={text==='loading'}）")
	}
	// 真实正文到了之后自动展开（壳重画 → 插件读到 Texts）。
	view.Diff.Texts = map[string]string{"a.txt": strings.Join(all, "\n") + "\n"}
	gapPump(t, session, invalidator, 8)
	if !gapContains(session, all[6]) {
		t.Fatalf("正文到达后没有展开真实行 %q：texts=%v open=%v want=%v\n%v",
			all[6], forgeDiffOf(env).Texts != nil, plugin.open.gapSet("s-1", "a.txt"),
			plugin.open.gapWanted("s-1", "a.txt"), gapBodyTexts(session))
	}
}

// TestGapHasNoCollapseLikeOfficial：官方 `ChangesTab.tsx:778` 只做并集，
// **没有**单 gap 收起路径。第二次点同一处不该改变展开态，展开的行还在。
func TestGapHasNoCollapseLikeOfficial(t *testing.T) {
	view, all := gapView()
	plugin, env, session, invalidator := drawGapChangesPrepared(t, 700, view, func(string) {}, func(p *Plugin) {
		p.open.split["s-1"] = false
	})
	hit, ok := gapHit(session, env, "a.txt", 1)
	if !ok {
		t.Fatal("gap 1 没有命中区")
	}
	x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
	if err := session.Click(x, y); err != nil {
		t.Fatal(err)
	}
	gapPump(t, session, invalidator, 8)
	before := plugin.open.gapSet("s-1", "a.txt")[1]
	// 同一坐标再点一次（此时那里已经是展开的正文行）。
	if err := session.Click(x, y); err != nil {
		t.Fatal(err)
	}
	gapPump(t, session, invalidator, 6)
	if !before || !plugin.open.gapSet("s-1", "a.txt")[1] {
		t.Fatal("官方没有单 gap 收起路径：第二次点击不该把展开态去掉")
	}
	if !gapContains(session, all[6]) {
		t.Fatalf("第二次点击后展开行消失了：\n%v", gapBodyTexts(session))
	}
}

// TestSplitGapExpandsRealLines：默认左右对比下 gap 在 canvas 上画，命中区照样可点。
func TestSplitGapExpandsRealLines(t *testing.T) {
	view, all := gapView()
	plugin, env, session, invalidator := drawGapChanges(t, 900, view, func(string) {})
	if !gapContains(session, "4 行未变更") {
		t.Fatalf("左右对比初始帧没有 gap 条纹：\n%v", gapBodyTexts(session))
	}
	hit, ok := gapHit(session, env, "a.txt", 1)
	if !ok {
		t.Fatalf("左右对比的 gap 没有命中区（画了但点不到）：%v", session.Snapshot().Hits)
	}
	if err := session.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	gapPump(t, session, invalidator, 10)
	if !plugin.open.gapSet("s-1", "a.txt")[1] {
		t.Fatal("左右对比里点击没有展开 gap 1")
	}
	for _, want := range all[6:10] {
		if !gapContains(session, want) {
			t.Fatalf("左右对比展开后缺真实行 %q：\n%v", want, gapBodyTexts(session))
		}
	}
}

// TestGapLinesRefuseTextFromAStaleRead 钉住插件层也做一次 `matchesHunks`：
// 壳里 `Texts` 可能是"读 diff 之后又被改过"的那份文件，官方在
// `ChangesTab.tsx:783-786` 拒绝展开并报 stale；本仓同样不给行。
func TestGapLinesRefuseTextFromAStaleRead(t *testing.T) {
	view, all := gapView()
	diff := &view.Diff
	file := diff.Files[0]
	if lines := gapLinesOf(diff, file); lines == nil || len(lines) != len(all) {
		t.Fatalf("真实新侧全文应当可用：%v", lines)
	}
	stale := append([]string(nil), all...)
	stale[10] = "被改过的一行"
	diff.Texts["a.txt"] = strings.Join(stale, "\n") + "\n"
	if lines := gapLinesOf(diff, file); lines != nil {
		t.Fatalf("过期正文必须拒绝（官方 toast.expandStale 同一判据），实得 %d 行", len(lines))
	}
}

// TestExpandGapOnlyEverOpens：官方 `expand()` 的 `open` 写入是**并集**
// （`ChangesTab.tsx:778` `new Set([...open[path] ?? [], index])`），
// 文件里没有删除路径；再点一次同一个 gap 也只是再并一次，不会收起。
// 这条直接打动作入口（界面里展开后那条按钮已经消失，点不到了）。
func TestExpandGapOnlyEverOpens(t *testing.T) {
	view, _ := gapView()
	plugin, env, _, _ := drawGapChangesPrepared(t, 700, view, func(string) {}, func(p *Plugin) {
		p.open.split["s-1"] = false
	})
	file := view.Diff.Files[0]
	plugin.expandGap(env, "s-1", file, 1)
	if !plugin.open.gapSet("s-1", "a.txt")[1] {
		t.Fatal("第一次 expand 应当展开 gap 1")
	}
	plugin.expandGap(env, "s-1", file, 1)
	if !plugin.open.gapSet("s-1", "a.txt")[1] {
		t.Fatal("官方没有单 gap 收起路径：再 expand 一次不该把它收起来")
	}
	plugin.expandGap(env, "s-1", file, 0)
	set := plugin.open.gapSet("s-1", "a.txt")
	if !set[0] || !set[1] {
		t.Fatalf("展开集合应当累计：%v", set)
	}
}

// TestSplitGapIsClickableWithoutWrap：关掉换行时左右两列各有自己的滚动视口命中区
// （铺满整列），gap 条纹的命中区必须**后登记**才不被盖住——官方那里 gap 是
// 一个真实 `<button>`，在任何一态都得点得动。
func TestSplitGapIsClickableWithoutWrap(t *testing.T) {
	view, all := gapView()
	plugin, env, session, invalidator := drawGapChangesPrepared(t, 900, view, func(string) {}, func(p *Plugin) {
		p.open.wrap["s-1"] = false
	})
	gapPump(t, session, invalidator, 6)
	hit, ok := gapHit(session, env, "a.txt", 1)
	if !ok {
		t.Fatalf("不换行的左右对比里 gap 没有命中区（被列的视口命中区盖住了？）：%v", session.Snapshot().Hits)
	}
	if err := session.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	gapPump(t, session, invalidator, 8)
	if !plugin.open.gapSet("s-1", "a.txt")[1] {
		t.Fatal("不换行时点 gap 没有展开（命中区被两列的滚动视口吃掉）")
	}
	for _, want := range all[6:10] {
		if !gapContains(session, want) {
			t.Fatalf("不换行展开后缺真实行 %q：\n%v", want, gapBodyTexts(session))
		}
	}
}

// TestGapCountUsesThousandsSeparator：官方 `DiffFile.tsx:414` 用
// `count.toLocaleString('en-US')`，1200 行要写 "1,200 行未变更"，不是 "1200"。
func TestGapCountUsesThousandsSeparator(t *testing.T) {
	count := 1200
	if got := gapText(changesSegment{count: &count}); got != "1,200 行未变更" {
		t.Fatalf("千行 gap 文案 = %q，要 %q", got, "1,200 行未变更")
	}
	for n, want := range map[int]string{0: "0", 7: "7", 999: "999", 1000: "1,000", 1234567: "1,234,567"} {
		if got := changesThousands(n); got != want {
			t.Fatalf("changesThousands(%d) = %q，要 %q", n, got, want)
		}
	}
}

// TestChevronsUpDownIconIsTheOfficialSprite：gap 条纹的图标必须是官方 sprite
// `chevrons-up-down-outline` 的两条描边 path（viewBox 16、stroke 1、round cap/join），
// 不是自画的三角/箭头。几何范围逐点等于官方 d 里的端点。
func TestChevronsUpDownIconIsTheOfficialSprite(t *testing.T) {
	icon, ok := chevronsUpDownIcon()
	if !ok {
		t.Fatal("官方 chevrons-up-down-outline 素材应当可画")
	}
	if icon.ViewBox != 16 || icon.Stroke != 1 {
		t.Fatalf("图标 viewBox/stroke = %v/%v，要 16/1", icon.ViewBox, icon.Stroke)
	}
	minX, minY, maxX, maxY, ok := dsassets.Bounds(chevronsUpDownAsset)
	if !ok {
		t.Fatal("素材没有可解析的 path")
	}
	if minX != 5.1 || minY != 3.1 || maxX != 10.9 || maxY != 12.9 {
		t.Fatalf("图标几何范围 = (%v,%v)-(%v,%v)，要官方 (5.1,3.1)-(10.9,12.9)",
			minX, minY, maxX, maxY)
	}
}

// gapLabelColor 取一帧里某段文字的绘制颜色（找不到返回零值）。
func gapLabelColor(snapshot app.Snapshot, want string) (ui.Color, bool) {
	for _, text := range snapshot.Texts {
		if text.Value == want {
			return text.Style.Color, true
		}
	}
	return ui.Color{}, false
}

// TestGapRowHoverSwitchesInkToPrimary：官方 `.gap:hover{color:label-primary}`
// （`ChangesTab.module.css:758-762`）。Viva 的子文本不继承父节点 hover 态，
// 所以这条必须由插件自己按 `canvas.Hovered()` 换字色——否则 hover 只剩底色。
func TestGapRowHoverSwitchesInkToPrimary(t *testing.T) {
	view, _ := gapView()
	_, env, session, invalidator := drawGapChangesPrepared(t, 700, view, func(string) {}, func(p *Plugin) {
		p.open.split["s-1"] = false
	})
	gapPump(t, session, invalidator, 6)
	hit, ok := gapHit(session, env, "a.txt", 1)
	if !ok {
		t.Fatal("gap 1 没有命中区")
	}
	before, okBefore := gapLabelColor(session.Snapshot(), "4 行未变更")
	if !okBefore {
		t.Fatalf("这一帧没有 gap 文案：%+v", session.Snapshot().Texts)
	}
	if err := session.Move(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	gapPump(t, session, invalidator, 4)
	after, okAfter := gapLabelColor(session.Snapshot(), "4 行未变更")
	if !okAfter {
		t.Fatal("悬停后 gap 文案消失了")
	}
	if before == after {
		t.Fatalf("悬停没有换字色（官方 label-tertiary → label-primary），两帧都是 %+v", before)
	}
}
