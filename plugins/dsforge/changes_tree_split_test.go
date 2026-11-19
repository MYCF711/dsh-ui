//go:build darwin || windows || linux

// changes_tree_split_test.go —— #64 ②③ 的判据（文件树模型、splitRows 配对、宽度钳、
// 宽窄分支、左右两列独立横滚、换行行高）。
//
// 像素判据全部走真实 headless 帧（不是"算对了但没画"）；纯函数判据对应官方
// `file-tree.ts` / `diff-rows.ts` 的逐条语义。
package dsforge

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// ── 纯函数：文件树（官方 file-tree.ts）────────────────────────────────────

func TestChangedFileTreeCompactsAndSortsLikeOfficial(t *testing.T) {
	files := []apiclient.ForgeDiffFile{
		{Path: "src/a/b.go"},
		{Path: "src/a/c.go"},
		{Path: "docs/readme.md"},
		{Path: "main.go"},
		{Path: "z.go"},
	}
	nodes := changedFileTree(files)
	// 目录组在前（docs、src/a），文件组在后（main.go、z.go），各自按 name 排序。
	// `src` 只有 `a` 一个目录且没有文件 → 压成 "src/a"（file-tree.ts:21-26）。
	if len(nodes) != 4 {
		t.Fatalf("顶层节点 = %d，要 4（2 目录 + 2 文件）：%+v", len(nodes), nodes)
	}
	if nodes[0].kind != changedNodeDirectory || nodes[0].name != "docs" || nodes[0].path != "docs" {
		t.Fatalf("第 1 个节点 = %+v，要是压缩后的 docs 目录", nodes[0])
	}
	if nodes[1].kind != changedNodeDirectory || nodes[1].name != "src/a" || nodes[1].path != "src/a" {
		t.Fatalf("第 2 个节点 = %+v，要是压缩后的 src/a 目录", nodes[1])
	}
	if len(nodes[1].children) != 2 || nodes[1].children[0].name != "b.go" || nodes[1].children[1].name != "c.go" {
		t.Fatalf("src/a 子节点 = %+v，要 b.go、c.go", nodes[1].children)
	}
	if nodes[2].kind != changedNodeFile || nodes[2].name != "main.go" || nodes[3].name != "z.go" {
		t.Fatalf("文件组 = %+v / %+v，要 main.go / z.go", nodes[2], nodes[3])
	}
}

func TestChangedFileTreeOrdersDirectoriesBeforeFiles(t *testing.T) {
	nodes := changedFileTree([]apiclient.ForgeDiffFile{
		{Path: "z.go"},
		{Path: "alpha/x.go"},
		{Path: "a.go"},
	})
	if len(nodes) != 3 {
		t.Fatalf("节点数 = %d", len(nodes))
	}
	if nodes[0].kind != changedNodeDirectory {
		t.Fatalf("目录必须排在文件前（官方 [...ordered(directories), ...ordered(files)]）：%+v", nodes)
	}
	if nodes[1].name != "a.go" || nodes[2].name != "z.go" {
		t.Fatalf("文件排序 = %q,%q，要 a.go,z.go", nodes[1].name, nodes[2].name)
	}
}

func TestFilesInTreeOrderIsDepthFirst(t *testing.T) {
	files := []apiclient.ForgeDiffFile{
		{Path: "src/a/b.go"},
		{Path: "docs/readme.md"},
		{Path: "main.go"},
	}
	order := filesInTreeOrder(changedFileTree(files))
	want := []string{"docs/readme.md", "src/a/b.go", "main.go"}
	if len(order) != len(want) {
		t.Fatalf("顺序长度 = %d", len(order))
	}
	for i, path := range want {
		if order[i].Path != path {
			t.Fatalf("第 %d 个 = %q，要 %q", i, order[i].Path, path)
		}
	}
}

// ── 纯函数：splitRows 配对（官方 diff-rows.ts:69-97）──────────────────────

func TestSplitRowsPairsDeletionsWithFollowingAdditions(t *testing.T) {
	rows := []changesRow{
		{old: intPtr(1), new: intPtr(1), sign: " ", text: "func main() {"},
		{old: intPtr(2), sign: "−", text: "old()"},
		{old: intPtr(3), sign: "−", text: "older()"},
		{new: intPtr(2), sign: "+", text: "new()"},
		{new: intPtr(3), sign: "+", text: "newer()"},
		{new: intPtr(4), sign: "+", text: "newest()"},
		{old: intPtr(4), new: intPtr(5), sign: " ", text: "}"},
	}
	pairs := changesSplitRows(rows)
	if len(pairs) != 5 {
		t.Fatalf("配对数 = %d，要 5（context + 3 对 + context）", len(pairs))
	}
	if pairs[0].left == nil || pairs[0].right == nil || pairs[0].left.no != 1 || pairs[0].right.no != 1 {
		t.Fatalf("context 行两边都要同号：%+v", pairs[0])
	}
	if pairs[1].left == nil || pairs[1].right == nil || pairs[1].left.text != "old()" || pairs[1].right.text != "new()" {
		t.Fatalf("第 1 对 = %+v，要 old()/new()", pairs[1])
	}
	if pairs[2].left.text != "older()" || pairs[2].right.text != "newer()" {
		t.Fatalf("第 2 对 = %+v", pairs[2])
	}
	// 第 3 个新增没有对应的删除：左列空、右列有（官方 `deleted[at] ?? null`）。
	if pairs[3].left != nil || pairs[3].right == nil || pairs[3].right.text != "newest()" {
		t.Fatalf("第 3 对 = %+v，要左空右 newest()", pairs[3])
	}
	if pairs[4].left == nil || pairs[4].right == nil || pairs[4].left.no != 4 || pairs[4].right.no != 5 {
		t.Fatalf("末尾 context = %+v，要 4/5", pairs[4])
	}
}

func TestSplitRowsFlushOnDeletionAfterAddition(t *testing.T) {
	// 官方语义：删除块与**随后**的新增块配对（diff-rows.ts:82-88）。
	// 所以 "增/删" 必须先 flush 掉前面的新增，删单独成行；否则会把删和
	// **前面**的新增凑成一对（顺序反了）。
	pairs := changesSplitRows([]changesRow{
		{new: intPtr(1), sign: "+", text: "b"},
		{old: intPtr(1), sign: "−", text: "c"},
	})
	if len(pairs) != 2 {
		t.Fatalf("配对数 = %d，要 2（先增后删不能凑一对）", len(pairs))
	}
	if pairs[0].left != nil || pairs[0].right == nil || pairs[0].right.text != "b" {
		t.Fatalf("第 1 对 = %+v，要左空右 b", pairs[0])
	}
	if pairs[1].left == nil || pairs[1].left.text != "c" || pairs[1].right != nil {
		t.Fatalf("第 2 对 = %+v，要左 c 右空", pairs[1])
	}
}

func TestSplitRowsSkipsNoNewlineMarker(t *testing.T) {
	pairs := changesSplitRows([]changesRow{
		{sign: " ", text: "\\ No newline at end of file"},
		{new: intPtr(1), sign: "+", text: "x"},
	})
	if len(pairs) != 1 || pairs[0].right == nil || pairs[0].right.text != "x" {
		t.Fatalf("`\\ No newline` 不是一行（官方 hunkRows 跳过）：%+v", pairs)
	}
}

// ── 纯函数：宽度钳（FileTreePane.tsx:9-11,38）──────────────────────────

func TestTreeWidthClampsMatchOfficial(t *testing.T) {
	cases := []struct {
		available, maximum float32
	}{
		{760, 440},  // min(600, 760-320)
		{1000, 600}, // 600 封顶
		{900, 580},  // 900-320
		{500, 200},  // max(200, 180)
	}
	for _, tc := range cases {
		if got := changesTreeMaximum(tc.available); got != tc.maximum {
			t.Fatalf("changesTreeMaximum(%v) = %v，官方 %v", tc.available, got, tc.maximum)
		}
	}
	state := &ui.SplitterState{Pos: 900}
	if got := treeWidthOf(state, 1000); got != 600 {
		t.Fatalf("超出 MAX_WIDTH 要钳到 600，实得 %v", got)
	}
	state.Pos = 10
	if got := treeWidthOf(state, 1000); got != 200 {
		t.Fatalf("低于 MIN_WIDTH 要钳到 200，实得 %v", got)
	}
	// Viva 的 hi = available - 8 - minSecond；minSecond 反算后必须等于官方 maximum。
	for _, available := range []float32{760, 900, 1000} {
		_, second := changesSplitterMins(available)
		hi := available - changesTreeSplitterThickness - second
		if want := changesTreeMaximum(available); hi != want {
			t.Fatalf("available=%v 时 Splitter hi=%v，要官方 maximum=%v", available, hi, want)
		}
	}
}

func TestTreeVisibleOnlyAt760(t *testing.T) {
	if changesTreeVisible(changesTreeBreakpoint - 0.5) {
		t.Fatal("759.5 不该显示文件树（官方 @container min-width:760px）")
	}
	if !changesTreeVisible(changesTreeBreakpoint) {
		t.Fatal("760 应当显示文件树")
	}
}

// ── 像素：宽栏出现文件树、窄栏没有 ────────────────────────────────────────

// treeSplitDiff 造一份带嵌套目录与超长行的 diff：
// 目录用于钉「宽栏才有树」，长行用于钉「不折行时每列各自的横向滚动」。
func treeSplitDiff() *pluginapi.ForgeDiff {
	return &pluginapi.ForgeDiff{
		SessionID: "s-1",
		Scope:     apiclient.ForgeScopeBranch,
		Base:      "main",
		Files: []apiclient.ForgeDiffFile{
			{
				Path: "src/a/long.go", Status: apiclient.ForgeFileModified,
				Additions: 2, Deletions: 1, Digest: "d-long",
				Hunks: []apiclient.ForgeHunk{{
					OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 2,
					Lines: []string{
						"-" + strings.Repeat("old", 90),
						"+" + strings.Repeat("new", 90),
						"+" + strings.Repeat("more", 60),
					},
				}},
			},
			{
				Path: "src/a/other.go", Status: apiclient.ForgeFileModified,
				Additions: 1, Deletions: 1, Digest: "d-other",
				Hunks: []apiclient.ForgeHunk{{
					OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 1,
					Lines: []string{"-short()", "+tiny()"},
				}},
			},
		},
	}
}

func treeSplitView() *pluginapi.ForgeView {
	return &pluginapi.ForgeView{
		SessionID: "s-1",
		Checkout:  pluginapi.ForgeCheckout{Kind: pluginapi.ForgeCheckoutReady, Branch: "feat/base"},
		Branches:  []string{"main", "feat/base"}, BranchesKnown: true,
		Diff: *treeSplitDiff(),
	}
}

// drawChangesWide 只在给定宽度的右栏里画 changes 体，返回插件与 env（供命中 id 用）。
func drawChangesWide(t *testing.T, width float32, view *pluginapi.ForgeView, prepare func(*Plugin)) (*Plugin, *pluginapi.Env, app.Snapshot) {
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
	feat := tree.Feature("forge-tree-headless")
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{
		Title: "forge-tree", Width: 1182, Height: 832, Tree: tree,
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
		Refresh:      func() {},
		LoadBranches: func() {},
		LoadScope:    func(string) {},
		LoadBases:    func(string) {},
		LoadText:     func(string) {},
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
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	return p, env, session.Snapshot()
}

// painterAfterDraw 取某个画家的 AfterDraw：像素判据必须走真实接线的钩子，
// 否则「忘了在 Panels() 里接 AfterDraw」这种缺陷测不出来（变异实测）。
func painterAfterDraw(p *Plugin, id string) func(*gfx.Canvas, *pluginapi.Env) {
	for _, painter := range p.Panels() {
		if painter.ID == id {
			return painter.AfterDraw
		}
	}
	return nil
}

func labelExists(snapshot app.Snapshot, value string) bool {
	for _, label := range snapshot.Labels {
		if label.Value == value {
			return true
		}
	}
	return false
}

func hitCount(snapshot app.Snapshot, label string) int {
	count := 0
	for _, hit := range snapshot.Hits {
		if hit.Label == label {
			count++
		}
	}
	return count
}

func TestWideChangesShowsFileTreePane(t *testing.T) {
	_, _, snapshot := drawChangesWide(t, 900, treeSplitView(), nil)
	if !labelExists(snapshot, "src/a") {
		t.Fatalf("宽 900 的文件树缺压缩目录名 src/a：\n%s", drawnText(snapshot))
	}
	if !labelExists(snapshot, "long.go") {
		t.Fatalf("文件树缺 long.go：\n%s", drawnText(snapshot))
	}
	// 树栏的可拖宽手柄必须是 Viva Splitter 的 RoleSlider 命中区。
	sliders := 0
	for _, hit := range snapshot.Hits {
		if hit.Role == gfx.RoleSlider {
			sliders++
		}
	}
	if sliders == 0 {
		t.Fatal("没有可拖宽分栏的 RoleSlider 命中区（ui.Splitter 没接上）")
	}
}

func TestNarrowChangesHidesFileTreePane(t *testing.T) {
	_, _, snapshot := drawChangesWide(t, 600, treeSplitView(), nil)
	if labelExists(snapshot, "src/a") {
		t.Fatalf("600 宽不该显示文件树（官方 @container min-width:760px）：\n%s", drawnText(snapshot))
	}
}

func TestTreeDirectoryCollapseHidesChildren(t *testing.T) {
	_, _, snapshot := drawChangesWide(t, 900, treeSplitView(), func(p *Plugin) {
		p.open.toggleTreeDir("s-1", "src/a")
	})
	if labelExists(snapshot, "long.go") {
		t.Fatalf("收起 src/a 后子文件不该出现在树里：\n%s", drawnText(snapshot))
	}
}

func labelPos(snapshot app.Snapshot, value string) (gfx.Rect, bool) {
	for _, label := range snapshot.Labels {
		if label.Value == value {
			return label.Bounds, true
		}
	}
	return gfx.Rect{}, false
}

// pointInRect 判一个点是否落在矩形里（含边界）。
func pointInRect(bounds gfx.Rect, x, y float32) bool {
	return x >= bounds.X && x <= bounds.X+bounds.Width && y >= bounds.Y && y <= bounds.Y+bounds.Height
}

func TestTreeIndentsChildrenByFourteen(t *testing.T) {
	_, _, snapshot := drawChangesWide(t, 900, treeSplitView(), nil)
	parent, okParent := labelPos(snapshot, "src/a")
	child, okChild := labelPos(snapshot, "long.go")
	if !okParent || !okChild {
		t.Fatalf("树上找不到 src/a 或 long.go：parent=%v child=%v", okParent, okChild)
	}
	// 官方 `paddingInlineStart = 8 + depth*14`（ChangedFiles.tsx:47）。
	if got := child.X - parent.X; got < 13 || got > 15 {
		t.Fatalf("子节点相对父节点的缩进 = %vpx，官方每层 14px", got)
	}
}

func TestTreeSelectedRowPaintsHoverBackground(t *testing.T) {
	theme := dscore.Dark()
	_, _, plain := drawChangesWide(t, 900, treeSplitView(), nil)
	_, _, selected := drawChangesWide(t, 900, treeSplitView(), func(p *Plugin) {
		p.open.selectFile("s-1", "src/a/long.go")
	})
	pos, ok := labelPos(selected, "long.go")
	if !ok {
		t.Fatal("树上找不到 long.go")
	}
	point := pos
	point.X = pos.X + 1
	point.Y = pos.Y + pos.Height/2

	hasHover := func(snapshot app.Snapshot) bool {
		for _, primitive := range snapshot.Primitives {
			if primitive.Color != ui.Color(theme.Hover) || primitive.Bounds.Width < 50 {
				continue
			}
			if pointInRect(primitive.Bounds, point.X, point.Y) {
				return true
			}
		}
		return false
	}
	if !hasHover(selected) {
		t.Fatal("选中的树行没有官方 `.selected` 底色（--dsw-alias-interactive-bg-hover）")
	}
	if hasHover(plain) {
		t.Fatal("未选中的树行不该有 `.selected` 底色")
	}
}

func TestTreeSplitterKeyboardFollowsOfficialSteps(t *testing.T) {
	p, _, snapshot := drawChangesWide(t, 900, treeSplitView(), nil)
	var splitterID gfx.ID
	for _, hit := range snapshot.Hits {
		if hit.Role == gfx.RoleSlider {
			splitterID = hit.ID
			break
		}
	}
	if splitterID.IsZero() {
		t.Fatal("没有分栏命中区")
	}
	state := p.open.treeSplit("s-1")
	start := state.Pos
	if !p.HandleKey(gfx.KeyEvent{Target: splitterID, Key: gfx.KeyRight}) {
		t.Fatal("分栏聚焦时 ArrowRight 没有被接管")
	}
	if state.Pos != start+16 {
		t.Fatalf("ArrowRight 后 Pos = %v，官方一档 16px（%v）", state.Pos, start+16)
	}
	if !p.HandleKey(gfx.KeyEvent{Target: splitterID, Key: gfx.KeyHome}) {
		t.Fatal("Home 没有被接管")
	}
	if state.Pos != changesTreeMinWidth {
		t.Fatalf("Home 后 Pos = %v，官方落在 MIN_WIDTH=200", state.Pos)
	}
	if p.HandleKey(gfx.KeyEvent{Target: splitterID, Key: gfx.KeyTab}) {
		t.Fatal("不该接管无关按键")
	}
}

// TestSplitLayoutWidthMatchesDiffColumn 钉「Draw 排版的宽度 == Viva 实际给 diff 栏的宽度」。
//
// 这是曾经的实缺陷：Draw 用整栏宽（900）排、AfterDraw 用扣掉树后的 631 复排，
// 于是每份文件缓存里有两个宽度，**占位高度按 879 算**、正文却按 631 折行，
// 长行会溢出到下一张文件卡上。判据：缓存里每份文件只有一个宽度，且等于
// `栏宽 - 树宽 - 8px 分栏 - 2*10px 内距 - 2*0.5px 描边`。
func TestSplitLayoutWidthMatchesDiffColumn(t *testing.T) {
	p, _, _ := drawChangesWide(t, 900, treeSplitView(), nil)
	treeWidth := p.open.treeSplit("s-1").Pos
	want := 900 - treeWidth - changesTreeSplitterThickness - 2*changesFilesInset - 2*changesFileBorder
	if len(p.open.layouts) != len(treeSplitDiff().Files) {
		t.Fatalf("排版缓存有 %d 条，要 %d 条（同一文件不许按两个宽度各排一次）：%v",
			len(p.open.layouts), len(treeSplitDiff().Files), keysOfLayouts(p))
	}
	for key, layout := range p.open.layouts {
		if !nearlyEqual(layout.width, want) {
			t.Fatalf("缓存 %q 的排版宽度 = %v，diff 栏正文宽 = %v", key, layout.width, want)
		}
	}
}

func keysOfLayouts(p *Plugin) []string {
	out := make([]string, 0, len(p.open.layouts))
	for key := range p.open.layouts {
		out = append(out, key)
	}
	return out
}

// ── 像素：左右对比 + 两列独立横滚 ─────────────────────────────────────────

func TestSplitViewDrawsBothSides(t *testing.T) {
	_, _, snapshot := drawChangesWide(t, 900, treeSplitView(), nil)
	// wrap=true（官方默认）时同一行正文左右各画一次。
	if got := strings.Count(drawnText(snapshot), "short()"); got < 1 {
		t.Fatalf("左右对比缺删除侧 short()：\n%s", drawnText(snapshot))
	}
	if got := strings.Count(drawnText(snapshot), "tiny()"); got < 1 {
		t.Fatalf("左右对比缺新增侧 tiny()：\n%s", drawnText(snapshot))
	}
}

func TestUnwrappedSplitHasOneScrollbarPerColumn(t *testing.T) {
	_, _, snapshot := drawChangesWide(t, 900, treeSplitView(), func(p *Plugin) {
		p.open.toggleWrap("s-1")
	})
	if got := hitCount(snapshot, "Horizontal scrollbar"); got < 2 {
		t.Fatalf("左右两列各自一个横向滚动条，实得 %d 个：%v", got, snapshot.Hits)
	}
	// 换行模式不该有横向滚动条（内容按列宽折行）。
	_, _, wrapped := drawChangesWide(t, 900, treeSplitView(), nil)
	if got := hitCount(wrapped, "Horizontal scrollbar"); got != 0 {
		t.Fatalf("data-wrap 模式不该出现横向滚动条，实得 %d 个", got)
	}
}

func TestSplitColumnsScrollIndependently(t *testing.T) {
	p, env, _ := drawChangesWide(t, 900, treeSplitView(), func(p *Plugin) {
		p.open.toggleWrap("s-1")
	})
	leftID := diffViewportID(env, "src/a/long.go", changesDiffSideLeft)
	rightID := diffViewportID(env, "src/a/long.go", changesDiffSideRight)
	if leftID.IsZero() || rightID.IsZero() {
		t.Fatal("左右列没有声明视口命中区（HandleScroll 收不到事件）")
	}
	result := p.HandleScroll(gfx.ScrollEvent{Target: leftID, DeltaX: 40})
	if !result.Handled {
		t.Fatal("左列视口上的横向滚轮没有被处理")
	}
	state := p.open.diffScroll("s-1", "src/a/long.go")
	if state.left.offset <= 0 {
		t.Fatalf("左列 offset = %v，应当被滚轮推动", state.left.offset)
	}
	if state.right.offset != 0 {
		t.Fatalf("右列 offset = %v，左列滚动不该动右列", state.right.offset)
	}
	if result := p.HandleScroll(gfx.ScrollEvent{Target: rightID, DeltaX: 20}); !result.Handled {
		t.Fatal("右列视口上的横向滚轮没有被处理")
	}
	if state.right.offset <= 0 {
		t.Fatalf("右列 offset = %v，应当被推动", state.right.offset)
	}
	// 竖向手势不属于列（交回外层文件列表滚动）。
	if result := p.HandleScroll(gfx.ScrollEvent{Target: leftID, DeltaY: 120}); result.Handled {
		t.Fatal("竖向滚轮不该被左右列吃掉")
	}
}

func TestWrapMakesLongLinesTaller(t *testing.T) {
	_, _, wrapped := drawChangesWide(t, 900, treeSplitView(), nil)
	_, _, plain := drawChangesWide(t, 900, treeSplitView(), func(p *Plugin) {
		p.open.toggleWrap("s-1")
	})
	// 文件卡头画完整路径（树里只画 basename），用完整路径才不会被树行抢到。
	wrappedY, okWrapped := labelY(wrapped, "src/a/other.go")
	plainY, okPlain := labelY(plain, "src/a/other.go")
	if !okWrapped || !okPlain {
		t.Fatalf("找不到 src/a/other.go 的文件卡标题：wrap=%v plain=%v", okWrapped, okPlain)
	}
	if wrappedY <= plainY {
		t.Fatalf("换行模式的长行应当把后面的文件卡推低：wrap Y=%v，不换行 Y=%v", wrappedY, plainY)
	}
}

// intPtr 是一个取地址的小工具（changesRow 的 old/new 是可空行号）。
func intPtr(v int) *int { return &v }
