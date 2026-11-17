// forge_test.go —— #64a 的单元判据（座位、可见性闸门、界面态、文案）。
//
// 这一层不画像素（像素在 visual_test.go），只钉三件事：
//
//	① 座位与官方一致（槽名 / key / 标签 / order），少一个座位就等于少一块界面；
//	② **可见性闸门**：数据面缺失、会话不匹配、动作面没接时**一个字都不画**
//	   —— 这是本仓最容易犯的错（"画了但没有真值"）；
//	③ 官方文案逐字（改文案 = 改界面语言，必须与 locales.ts 对得上）。
package dsforge

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// newPlugin 造一个已拿到主题的插件（与壳装配后的状态一致）。
func newPlugin(t *testing.T) *Plugin {
	t.Helper()
	p := &Plugin{}
	if err := p.Register(testContext{theme: testTheme()}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return p
}

// ── 座位 ────────────────────────────────────────────────────────────────

func TestSeatsMatchOfficialRegistrations(t *testing.T) {
	p := newPlugin(t)
	painters := p.Panels()
	byID := map[string]pluginapi.Painter{}
	for _, painter := range painters {
		byID[painter.ID] = painter
	}
	// 官方 mount.ts:389-391 —— conversation.hero.options（工作树勾选）
	hero, ok := byID["forge-worktree-option"]
	if !ok {
		t.Fatal("没有工作树勾选座位（官方 conversation.hero.options，mount.ts:389-391）")
	}
	if hero.Slot != uikernel.OffHeroOptions {
		t.Fatalf("工作树勾选挂在 %q，官方是 %q", hero.Slot, uikernel.OffHeroOptions)
	}
	// 官方 mount.ts:392-394 —— conversation.input.dock order 5
	dock, ok := byID["forge-local-card"]
	if !ok {
		t.Fatal("没有本地变更卡座位（官方 conversation.input.dock，mount.ts:392-394）")
	}
	if dock.Slot != uikernel.OffInputDock {
		t.Fatalf("本地卡挂在 %q，官方是 %q", dock.Slot, uikernel.OffInputDock)
	}
	if dock.Order != 5 {
		t.Fatalf("本地卡 order = %d，官方 dock 那条是 5", dock.Order)
	}
	// 官方 mount.ts:364-369 —— sidebar.right.pane.tab + .title，同一个 key
	body, ok := byID["forge-changes-tab"]
	if !ok {
		t.Fatal("没有变更视图的体（官方 sidebar.right.pane.tab，mount.ts:364-366）")
	}
	if body.Slot != pluginapi.SlotRightPaneTab || body.Key != changesTabKey {
		t.Fatalf("变更视图体 = (%q,%q)，要 (%q,%q)", body.Slot, body.Key, pluginapi.SlotRightPaneTab, changesTabKey)
	}
	if body.Label != "变更" {
		t.Fatalf("tab 芯片文案 = %q，官方 diff.tab 中文是 变更", body.Label)
	}
	title, ok := byID["forge-changes-tab-title"]
	if !ok {
		t.Fatal("没有变更视图的标题（官方 sidebar.right.pane.tab.title，mount.ts:367-369）")
	}
	if title.Slot != pluginapi.SlotRightPaneTabTitle || title.Key != changesTabKey {
		t.Fatalf("标题 = (%q,%q)，要与体同一个 key", title.Slot, title.Key)
	}
	// tab key 必须与官方那个完整包名一致：官方 `CHANGES_ID` 就是它。
	if changesTabKey != "@deepseek-ai/dsh-experimental-client-ui-forge/changes" {
		t.Fatalf("tab key 必须逐字等于官方 CHANGES_ID，实得 %q", changesTabKey)
	}
	if len(painters) != 4 {
		t.Fatalf("座位应当恰好 4 个（2 个 tab 槽 + 2 个 dock/hero），实得 %d", len(painters))
	}
}

// ── 可见性闸门 ──────────────────────────────────────────────────────────

func TestForgeViewHiddenWithoutData(t *testing.T) {
	if got := forgeView(&pluginapi.Env{}); got != nil {
		t.Fatalf("没有 Forge 数据面时必须返回 nil，实得 %+v", got)
	}
	if got := forgeDiffOf(&pluginapi.Env{}); got != nil {
		t.Fatalf("没有数据面时 diff 必须 nil，实得 %+v", got)
	}
}

func TestForgeViewHiddenForAnotherSession(t *testing.T) {
	// 换会话那一帧：数据面还是旧会话的 → 不许画（否则上一个会话的 diff 会画进这一个）。
	env := &pluginapi.Env{Data: pluginapi.Snapshot{
		Cur:   "s-new",
		Forge: &pluginapi.ForgeView{SessionID: "s-old", Checkout: pluginapi.ForgeCheckout{Kind: pluginapi.ForgeCheckoutReady}},
	}}
	if got := forgeView(env); got != nil {
		t.Fatalf("会话不匹配时必须 nil，实得 %+v", got)
	}
	// diff 自己还有一路会话判定：数据面对上、diff 对不上也不画。
	env.Data.Forge.SessionID = ""
	env.Data.Cur = "s-new"
	env.Data.Forge = &pluginapi.ForgeView{
		SessionID: "s-new",
		Diff:      pluginapi.ForgeDiff{SessionID: "s-other"},
	}
	if got := forgeDiffOf(env); got != nil {
		t.Fatalf("diff 的会话不匹配时必须 nil，实得 %+v", got)
	}
	env.Data.Forge.Diff.SessionID = "s-new"
	if got := forgeDiffOf(env); got == nil {
		t.Fatal("同一会话的 diff 应当可画")
	}
}

// ── 界面态（官方 changes-store / viewed-files 的等价物）──────────────────

func TestViewedThreeStatesFollowDigest(t *testing.T) {
	open := newChangesOpen()
	if got := open.viewedState("s", "a.go", "d1"); got != viewedUnviewed {
		t.Fatalf("没存过应当是未读，实得 %q", got)
	}
	open.markViewed("s", "a.go", "d1", true)
	if got := open.viewedState("s", "a.go", "d1"); got != viewedDone {
		t.Fatalf("digest 相同应当是已读，实得 %q", got)
	}
	// 文件改了（digest 变了）→ 官方叫"已变更"，不是"已读"。
	if got := open.viewedState("s", "a.go", "d2"); got != viewedChanged {
		t.Fatalf("digest 变了应当是已变更，实得 %q", got)
	}
	open.markViewed("s", "a.go", "d1", false)
	if got := open.viewedState("s", "a.go", "d1"); got != viewedUnviewed {
		t.Fatalf("撤销后应当回到未读，实得 %q", got)
	}
	// 已读状态按会话隔离：别的会话同路径不受影响。
	open.markViewed("s", "a.go", "d1", true)
	if got := open.viewedState("other", "a.go", "d1"); got != viewedUnviewed {
		t.Fatalf("已读标记不该跨会话，实得 %q", got)
	}
}

func TestFileCardsDefaultExpandedAndToggle(t *testing.T) {
	open := newChangesOpen()
	if !open.fileExpanded("s", "a.go") {
		t.Fatal("文件卡默认展开（官方只有未滚到的文件才收成占位行）")
	}
	open.toggleFile("s", "a.go")
	if open.fileExpanded("s", "a.go") {
		t.Fatal("toggle 之后应当收起")
	}
	open.toggleFile("s", "a.go")
	if !open.fileExpanded("s", "a.go") {
		t.Fatal("再 toggle 应当展开")
	}
	if open.localExpanded("s") {
		t.Fatal("本地卡默认收起（官方 useState(false)）")
	}
	open.toggleLocal("s")
	if !open.localExpanded("s") {
		t.Fatal("toggle 之后本地卡应当展开")
	}
}

// ── 文案与格式化（官方 locales.ts 逐字）────────────────────────────────

func TestCopyMatchesOfficialLocales(t *testing.T) {
	cases := map[string]string{
		copyScopeBranch:      "分支",
		copyScopeUncommitted: "未提交",
		copyScopeUnstaged:    "未暂存",
		copyScopeStaged:      "已暂存",
		copyLocalTitle:       "本地变更",
		copyDiffEmpty:        "没有变更",
		copyDiffError:        "无法读取变更",
		copyDiffBinary:       "二进制文件，无法显示变更",
		copyDiffFileTooLarge: "文件太大，无法显示变更",
		copyWorktreeLabel:    "新建工作树",
		copyWorktreeBaseHead: "基于当前 HEAD",
		changesTabLabel:      "变更",
	}
	if len(cases) != 12 {
		// 防"复制粘贴时丢一条"：这张表就是逐字清单，条数变了要一起改注释。
		t.Fatalf("逐字表应当 12 条，实得 %d", len(cases))
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("%q != %q", got, want)
		}
	}
}

func TestFormatters(t *testing.T) {
	if got := scopeLabel(apiclient.ForgeScopeUnstaged); got != "未暂存" {
		t.Fatalf("scopeLabel(unstaged) = %q", got)
	}
	// 未知范围**不编中文**：原样返回（官方 scope 是封闭联合，走到这里说明上游变过）。
	if got := scopeLabel("future-scope"); got != "future-scope" {
		t.Fatalf("未知范围应当原样返回，实得 %q", got)
	}
	hunk := apiclient.ForgeHunk{OldStart: 3, OldLines: 2, NewStart: 4, NewLines: 5}
	if got := hunkHeader(hunk); got != "@@ -3,2 +4,5 @@" {
		t.Fatalf("hunkHeader = %q", got)
	}
	previous := "old/path.go"
	if got := fileBadge(apiclient.ForgeDiffFile{Status: apiclient.ForgeFileRenamed, PreviousPath: &previous}); got != "由 old/path.go 重命名" {
		t.Fatalf("重命名标签 = %q", got)
	}
	// 没有 previousPath 的重命名：**不画**标签（不编"从哪儿来"）。
	if got := fileBadge(apiclient.ForgeDiffFile{Status: apiclient.ForgeFileRenamed}); got != "" {
		t.Fatalf("缺 previousPath 时不该有标签，实得 %q", got)
	}
	if got := fileBadge(apiclient.ForgeDiffFile{Status: apiclient.ForgeFileAdded}); got != "新文件" {
		t.Fatalf("新文件标签 = %q", got)
	}
	if got := fileBadge(apiclient.ForgeDiffFile{Status: apiclient.ForgeFileModified, Untracked: true}); got != "未提交" {
		t.Fatalf("未跟踪标签 = %q", got)
	}
	if got := sprintf(copyDiffOmitted, itoa(3)); got != "另有 3 个未跟踪的文件没有列出" {
		t.Fatalf("省略提示 = %q", got)
	}
	if got := localCountsText(pluginapi.ForgeLocalSummary{Additions: 12, Deletions: 3}); got != "+12 −3" {
		t.Fatalf("本地计数 = %q", got)
	}
	if !strings.Contains(sprintf(copyWorktreeBase, "main"), "main") {
		t.Fatal("基准文案插值失败")
	}
}

// ── 双行号（官方 `.line` 四轨）────────────────────────────────────────
//
// 官方每一行给**两侧**行号：上下文两边都数、删行只数旧侧、加行只数新侧，
// `\ No newline at end of file` 不算一行（`OFF:src/client/diff-rows.ts:53-68`
// 的 `hunkRows()`）。这里逐条钉住这个口径。
func TestDiffRowNumbersMatchOfficialHunkRows(t *testing.T) {
	// hunk 从旧 10 / 新 10 起：context、删、删、加、context。
	lines := []string{" keep", "-old1", "-old2", "+new1", " tail"}
	oldNo, newNo := 10, 10
	type want struct {
		old, new *int
		sign     string
	}
	n := func(v int) *int { return &v }
	wants := []want{
		{old: n(10), new: n(10), sign: " "},
		{old: n(11), new: nil, sign: "−"},
		{old: n(12), new: nil, sign: "−"},
		{old: nil, new: n(11), sign: "+"},
		{old: n(13), new: n(12), sign: " "},
	}
	for i, line := range lines {
		row := changesDiffRowOf(line, oldNo, newNo)
		if row.old != nil {
			oldNo++
		}
		if row.new != nil {
			newNo++
		}
		if row.sign != wants[i].sign {
			t.Fatalf("第 %d 行符号 = %q，要 %q", i, row.sign, wants[i].sign)
		}
		if !sameLineNo(row.old, wants[i].old) || !sameLineNo(row.new, wants[i].new) {
			t.Fatalf("第 %d 行行号 = (old=%s,new=%s)，要 (old=%s,new=%s)",
				i, lineNoText(row.old), lineNoText(row.new), lineNoText(wants[i].old), lineNoText(wants[i].new))
		}
	}
	// `\ No newline at end of file`：两侧都不给号，也**不推进**任何一侧。
	row := changesDiffRowOf(`\ No newline at end of file`, 7, 9)
	if row.old != nil || row.new != nil {
		t.Fatalf("no-newline 标记不该带行号，实得 (old=%s,new=%s)", lineNoText(row.old), lineNoText(row.new))
	}
}

// sameLineNo 比较两个可选行号（nil 只等于 nil）。
func sameLineNo(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// lineNoText 让失败信息里的 nil 看得见。
func lineNoText(v *int) string {
	if v == nil {
		return "nil"
	}
	return itoa(*v)
}

// TestNumberColumnMatchesOfficialWidth 钉住 `--forge-number-width`
// （`OFF:src/client/DiffFile.tsx:194-195`：`max(2, 最大行号位数)ch + 12px`）。
func TestNumberColumnMatchesOfficialWidth(t *testing.T) {
	file := apiclient.ForgeDiffFile{Hunks: []apiclient.ForgeHunk{
		{OldStart: 1, OldLines: 9, NewStart: 1, NewLines: 9},
	}}
	// 行号最大 9 → 1 位，但官方下限是 2 位。
	if got, want := changesNumberW(file, 12), float32(2)*12*changesNumberChEm+changesNumberExtra; got != want {
		t.Fatalf("两位下限时列宽 = %v，要 %v", got, want)
	}
	// 行号到 128 → 3 位。
	file.Hunks[0] = apiclient.ForgeHunk{OldStart: 100, OldLines: 29, NewStart: 100, NewLines: 29}
	if got, want := changesNumberW(file, 12), float32(3)*12*changesNumberChEm+changesNumberExtra; got != want {
		t.Fatalf("三位时列宽 = %v，要 %v", got, want)
	}
	// 空 hunk 也取官方下限 2 位（不是 0/负数）。
	if got, want := changesNumberW(apiclient.ForgeDiffFile{}, 12), float32(2)*12*changesNumberChEm+changesNumberExtra; got != want {
		t.Fatalf("无 hunk 时列宽 = %v，要 %v", got, want)
	}
}

// TestSingleSideMatchesOfficialStatus 钉住 `singleSide`
// （`OFF:src/client/DiffFile.tsx:190`：新增只有新侧、删除只有旧侧）。
func TestSingleSideMatchesOfficialStatus(t *testing.T) {
	cases := map[string]string{
		apiclient.ForgeFileAdded:    "R",
		apiclient.ForgeFileDeleted:  "L",
		apiclient.ForgeFileModified: "",
		apiclient.ForgeFileRenamed:  "",
	}
	for status, want := range cases {
		if got := changesSingleSide(apiclient.ForgeDiffFile{Status: status}); got != want {
			t.Fatalf("status=%q 的单侧 = %q，要 %q", status, got, want)
		}
	}
}

// ── 测试替身（只够本文件用，不引壳）────────────────────────────────────

type testContext struct{ theme *pluginapi.Theme }

func (c testContext) Name() string        { return "ds-forge-test" }
func (c testContext) Provide(string, any) {}
func (c testContext) Inject(string) any   { return c.theme }
func (c testContext) Effect(func())       {}
func (c testContext) Logf(string, ...any) {}

// testTheme 造一份最小主题（只给本插件用到的令牌；颜色取 dscore 的真实取值
// 会把插件测试绑到另一个包上，这里只要"非零且互不相同"就够判可见性）。
func testTheme() *pluginapi.Theme {
	return &pluginapi.Theme{
		Name: "test", Dark: true,
		BgBase: colorOf(1), BgLayer1: colorOf(2), BgLayer2: colorOf(3), BgLayer3: colorOf(4),
		Hover:    colorOf(5),
		BorderL2: colorOf(6), BorderL3: colorOf(7), BorderL4: colorOf(8),
		LabelPrimary: colorOf(9), LabelSecondary: colorOf(10),
		LabelTertiary: colorOf(11), LabelCaption: colorOf(12), LabelDimmed: colorOf(13),
		BrandPrimary: colorOf(14), ButtonPrimaryFg: colorOf(15),
		Success: colorOf(16), Error: colorOf(17), Warn: colorOf(18), Info: colorOf(19),
		CodeComment: colorOf(20),
		Radii:       pluginapi.Radii{SM: 6, MD: 10},
		Fonts: pluginapi.Fonts{
			Body: 14, BodyLine: 22, Secondary: 13, SecondaryLine: 20,
			Caption: 12, CaptionLine: 18, Code: 12, CodeLine: 18, Title: 20, Tiny: 11,
		},
	}
}

// colorOf 造一个不透明的测试色（通道值由 n 决定，保证互不相同）。
func colorOf(n int) ui.Color {
	return ui.RGBA(uint8(20*n), uint8(3*n), uint8(5*n), 255)
}
