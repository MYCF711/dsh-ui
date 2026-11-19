//go:build darwin || windows || linux

// visual_test.go —— #64a 三块座位的**真实像素帧**（Viva headless，pump 到条件成立）。
//
// 为什么单测不够：座位注册对、状态算对，仍然可能"算了但没画"（本仓踩过多次）。
// 所以这里各留一帧，判据全部是**官方文案或官方几何**：
//
//	hero      分支芯片 + 新建工作树（官方 WorktreeToggle，13px/20px、28px 高）
//	dock      本地变更卡：本地变更 / HEAD → main / +12 −3 / 3 个文件
//	changes   头部 HEAD → main、范围芯片"分支"、文件卡、`@@ -1,2 +1,3 @@`、+/- 行
//
// 两条**否定**判据同样重要：
//   - 没有数据面（`Forge == nil`）时三块**都不画**——三张截图里不该出现任何 forge 文案；
//   - 会话不匹配时同样不画（换会话那一帧不许把上个会话的改动画进来）。
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

// sampleDiff 造一份字段齐全的 branch 范围 diff（形状照官方 ForgeLocalDiff 摆）。
func sampleDiff() *pluginapi.ForgeDiff {
	previous := "old/name.go"
	return &pluginapi.ForgeDiff{
		SessionID: "s-1",
		Scope:     apiclient.ForgeScopeBranch,
		Base:      "main",
		Files: []apiclient.ForgeDiffFile{
			{
				Path: "a.go", Status: apiclient.ForgeFileModified,
				Additions: 8, Deletions: 2, Layer: apiclient.ForgeLayerImplementation,
				Digest: "d-a",
				Hunks: []apiclient.ForgeHunk{{
					OldStart: 1, OldLines: 2, NewStart: 1, NewLines: 3,
					Lines: []string{" func main() {", "-\told()", "+\tnew()", "+\tmore()"},
				}},
			},
			{
				Path: "b_test.go", Status: apiclient.ForgeFileAdded, Untracked: true,
				Additions: 4, Layer: apiclient.ForgeLayerTest, Digest: "d-b",
				Hunks: []apiclient.ForgeHunk{{
					OldStart: 0, OldLines: 0, NewStart: 1, NewLines: 1,
					Lines: []string{"+package demo"},
				}},
			},
			{
				Path: "c.go", Status: apiclient.ForgeFileRenamed, PreviousPath: &previous,
				Binary: true, Digest: "d-c",
			},
		},
		Omitted: 2,
	}
}

// sampleView 造一份"数据齐"的 A 档数据面。
func sampleView() *pluginapi.ForgeView {
	diff := sampleDiff()
	from := "feat/base"
	return &pluginapi.ForgeView{
		SessionID: "s-1",
		Checkout: pluginapi.ForgeCheckout{
			Kind: pluginapi.ForgeCheckoutReady, Branch: "feat/base",
		},
		Branches:      []string{"main", "feat/base"},
		BranchesKnown: true,
		Worktree:      pluginapi.ForgeWorktree{Requested: true, From: &from},
		Local: pluginapi.ForgeLocalSummary{
			Known: true, Files: 3, Additions: 12, Deletions: 3, Base: "main",
		},
		Diff: *diff,
	}
}

// drawOnce 把三块座位各画一帧（同一张画布里自上而下），返回 pump 后的快照。
//
// 三块共用一张画布是故意的：它同时验证"三块能同时存在而不互相顶掉"。
func drawOnce(t *testing.T, view *pluginapi.ForgeView, cur string) app.Snapshot {
	t.Helper()
	return drawOncePrepared(t, view, cur, nil)
}

// drawOncePrepared 同 drawOnce，另在首帧之前给测试一步改插件内部态的机会。
//
// 需要它的是**浮层菜单**：Viva 的弹层条目由框架自己持有身份（`MenuItems.Item`
// 内部分配 scope），测试点不到那一条，所以只能直接把"菜单已打开"这个界面态摆好，
// 再看这一帧画出了什么。
func drawOncePrepared(t *testing.T, view *pluginapi.ForgeView, cur string, prepare func(*Plugin)) app.Snapshot {
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
	feat := tree.Feature("forge-headless")
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{
		Title: "forge", Width: 1182, Height: 832, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	var act pluginapi.Actions
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: feat,
		Bounds:    gfx.Rect{Width: 1182, Height: 832},
		MainWidth: 902,
		Geometry:  pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902},
	}
	if view != nil {
		act.Forge = &pluginapi.ForgeActions{
			Refresh:         func() {},
			LoadBranches:    func() {},
			LoadScope:       func(string) {},
			LoadBases:       func(string) {},
			LoadText:        func(string) {},
			RequestWorktree: func(*string) {},
			CancelWorktree:  func() {},
		}
	}
	act.SelectRightTab = func(string) {}
	env.Act = act
	env.Data = pluginapi.Snapshot{Cur: cur, Forge: view}

	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env.Canvas = canvas
		page.Draw(canvas, bounds, func(f ui.C) {
			ui.Row(f).FillWidth().FillHeight().Children(func() {
				// 左：主列（hero 勾选 + dock 本地卡）。
				ui.Column(f).Width(700).FillHeight().Pad(16).Gap(12).Children(func() {
					p.drawWorktreeToggle(f, env)
					p.drawLocalCard(f, env)
				})
				// 右：变更视图（右栏宽度按官方 --forge-tree-width 附近的 420）。
				ui.Column(f).Width(420).FillHeight().Children(func() {
					p.drawChangesBody(f, env)
				})
			})
		})
		// 布局已定：跑插件的 AfterDraw（#64 ③ 的左右对比正文在 canvas 上画）。
		// 走 Painter 里接好的那个钩子，顺带钉住「AfterDraw 不许漏接」。
		if after := painterAfterDraw(p, "forge-changes-tab"); after != nil {
			after(canvas, env)
		}
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	invalidator.Invalidate()
	// pump 到条件成立：viva 的字体测量在第一帧之后才稳定，一帧就断言会时红时绿。
	for i := 0; i < 20; i++ {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
		if text := drawnText(session.Snapshot()); strings.Count(text, "分支") >= 2 {
			return session.Snapshot()
		}
	}
	return session.Snapshot()
}

// drawnText 把这一帧画出来的文字拼起来（命中区标签 + 图元节点上的文字）。
func drawnText(snapshot app.Snapshot) string {
	values := make([]string, 0, len(snapshot.Labels))
	for _, label := range snapshot.Labels {
		values = append(values, label.Value)
	}
	return strings.Join(values, "\n")
}

func TestForgeSeatsDrawOfficialCopy(t *testing.T) {
	snapshot := drawOnce(t, sampleView(), "s-1")
	text := drawnText(snapshot)
	// hero：已请求态 → "基于 feat/base"（官方 `worktree.base`）+ 勾选文案。
	for _, want := range []string{"新建工作树", "基于 feat/base"} {
		if !strings.Contains(text, want) {
			t.Fatalf("hero 座位缺 %q：\n%s", want, text)
		}
	}
	// dock：本地卡三行（标题 / refs / 计数 + 文件数）。
	for _, want := range []string{"本地变更", "HEAD → main", "+12", "−3", "3 个文件"} {
		if !strings.Contains(text, want) {
			t.Fatalf("本地变更卡缺 %q：\n%s", want, text)
		}
	}
	// changes：头部基准、范围芯片、文件卡、hunk 头与增删行。
	for _, want := range []string{"HEAD → main", "分支", "a.go", "b_test.go", "@@ -1,2 +1,3 @@", "func main() {"} {
		if !strings.Contains(text, want) {
			t.Fatalf("变更视图缺 %q：\n%s", want, text)
		}
	}
	// 二进制文件说"无法显示变更"，不是画一段空代码。
	if !strings.Contains(text, "二进制文件，无法显示变更") {
		t.Fatalf("二进制文件没有官方提示：\n%s", text)
	}
	// 被略掉的未跟踪文件数要如实说（官方 `diff.omitted`）。
	if !strings.Contains(text, "另有 2 个未跟踪的文件没有列出") {
		t.Fatalf("省略提示缺失：\n%s", text)
	}
	// 三块都画了命中区（可点）：hero 勾选、本地卡、范围选择、刷新。
	for _, label := range []string{"新建工作树", "本地变更", "分支", "刷新变更"} {
		if !hasHitLabel(snapshot, label) {
			t.Fatalf("命中区缺 %q（画了但点不到）", label)
		}
	}
}

// TestDiffRowsDrawBothLineNumbers 验双行号**真的画出来了**：
// 官方每一行给两侧行号（`OFF:src/client/ChangesTab.module.css:604` 的四轨 `.line`），
// 像素层必须能看到"旧号 · 新号 · 符号 · 正文"，而不是只有符号+正文。
//
// 判据用**每行内**的号码序列，不用 tokens.Contains：`+2` / `−3` 这类计数 chip
// 里也有数字，顶层 Contains 会被它们蒙混过去（实测去掉行号仍然通过）。
func TestDiffRowsDrawBothLineNumbers(t *testing.T) {
	snapshot := drawOnce(t, sampleView(), "s-1")
	// a.go 的 hunk 是 `@@ -1,2 +1,3 @@`，四行、两侧号码逐行如下：
	//   1|1| ` func main() {`   2| | `−old()`    |2| `+new()`    |3| `+more()`
	// 号码落在 `ui.Text` 里就是标签值本身（`"2"` / `"3"`），而行内一定还有正文。
	// 所以判据：存在一个"纯号码"标签，且标签值里既有旧侧的 2、也有新侧的 2 与 3，
	// 并且这些号码标签的总数 ≥ 行数×每行号码数（1+1+2+2=6）。
	numbers := map[string]bool{}
	numberLabels := 0
	for _, label := range snapshot.Labels {
		if isLineNo(label.Value) {
			numbers[label.Value] = true
			numberLabels++
		}
	}
	for _, want := range []string{"1", "2", "3"} {
		if !numbers[want] {
			t.Fatalf("diff 行缺行号 %q（只画了符号+正文）：\n%s", want, drawnText(snapshot))
		}
	}
	// 四行一共 6 格号码（上下文 2 格、删 1 格、加 2 格、加 2 格 = 7 格里 6 格有号；
	// 第 2 行的旧号 2 与第 4 行的新号 3 都算）。给下界足够钉住"每行都有"。
	if numberLabels < 6 {
		t.Fatalf("号码格只有 %d 个，少于 hunk 需要的 6 格：\n%s", numberLabels, drawnText(snapshot))
	}
}

// isLineNo 判断一个标签是不是纯行号（1~6 位十进制）。
func isLineNo(value string) bool {
	if value == "" || len(value) > 6 {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func TestForgeSeatsDrawNothingWithoutData(t *testing.T) {
	text := drawnText(drawOnce(t, nil, "s-1"))
	for _, forbidden := range []string{"新建工作树", "本地变更", "变更", "分支"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("没有数据面时不许画 %q（比空着更坏）：\n%s", forbidden, text)
		}
	}
}

func TestForgeSeatsDrawNothingForAnotherSession(t *testing.T) {
	// 数据面属于 s-1，当前会话是 s-2：一个字都不画。
	text := drawnText(drawOnce(t, sampleView(), "s-2"))
	for _, forbidden := range []string{"新建工作树", "本地变更", "没有变更"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("换会话那一帧不许画 %q：\n%s", forbidden, text)
		}
	}
}

func TestCheckedWorktreeShowsHeadWhenNoBase(t *testing.T) {
	view := sampleView()
	view.Worktree = pluginapi.ForgeWorktree{Requested: true}
	text := drawnText(drawOnce(t, view, "s-1"))
	if !strings.Contains(text, "基于当前 HEAD") {
		t.Fatalf("基于 HEAD 的请求要画官方那句：\n%s", text)
	}
}

// TestScopeMenuIsAnOverlayWithOfficialOrder 钉住范围菜单的两条官方形状。
//
// ① **它是浮层**：官方 `ScopeMenu` 是 `portal: true` 的 `primitives.Menu`
// （`OFF:lib/client.js:6436-6479`：`portal`/`align:"start"`/`selectedId`），打开时
// 不占布局。本仓以前把它画成头部下方的一列，会把工具栏与文件卡整体下推——这一条
// 就是那个差异的判据。
// ② **条目顺序**：未提交 → 未暂存 → 已暂存 → 分隔线 → 分支（同一个 items 数组；
// 分支单独一组，它比整段分支范围）。
func TestScopeMenuIsAnOverlayWithOfficialOrder(t *testing.T) {
	closed := drawOnce(t, sampleView(), "s-1")
	opened := drawOncePrepared(t, sampleView(), "s-1", func(p *Plugin) {
		p.open.menu["s-1"] = true
	})

	// ① 打开菜单不许动到下面的内容：同一句正文（hunk 头）的 Y 必须一格不差。
	closedY, okClosed := labelY(closed, "@@ -1,2 +1,3 @@")
	openY, okOpen := labelY(opened, "@@ -1,2 +1,3 @@")
	if !okClosed || !okOpen {
		t.Fatalf("两帧里都该画 hunk 头（closed=%v opened=%v）", okClosed, okOpen)
	}
	if closedY != openY {
		t.Fatalf("范围菜单占用了布局：hunk 头 Y 从 %v 变成 %v（官方是浮层，不许下推内容）", closedY, openY)
	}

	// ② 关着的时候菜单条目一句都不该出现（"未提交"除外：未跟踪文件的徽标就是这四个字）。
	closedText := drawnText(closed)
	for _, absent := range []string{"未暂存", "已暂存"} {
		if strings.Contains(closedText, absent) {
			t.Fatalf("菜单没开却画了 %q：\n%s", absent, closedText)
		}
	}

	// ③ 打开后四个条目按官方顺序出现。取**最后一次**出现：它们是弹层里那几条，
	// 画在页面内容之后（"未提交"还会作为文件卡徽标先出现一次）。
	openText := drawnText(opened)
	prev := -1
	for _, want := range []string{"未提交", "未暂存", "已暂存", "分支"} {
		at := strings.LastIndex(openText, want)
		if at < 0 {
			t.Fatalf("打开的菜单缺条目 %q：\n%s", want, openText)
		}
		if at <= prev {
			t.Fatalf("菜单条目顺序不是官方的：%q 落在 %d，前一条在 %d\n%s", want, at, prev, openText)
		}
		prev = at
	}

	// ④ **位置**：浮层画在锚点（范围芯片）**下方**、不遮住它——官方 `anchor` 是那个
	// `.select` 按钮、`align:"start"`、`portal:true`（`OFF:lib/client.js:6474-6485`）。
	// 芯片文字是这一帧里第一个"分支"（它先于弹层画），菜单条目排在它后面。
	// （两条要求互补：`items.Above()` 在本帧会因为面板上沿没空间而**翻回下方**，
	// 所以这里钉的是「浮层在芯片下方、不盖住芯片」，不是"绝不向上翻"。）
	chipY, okChip := labelY(opened, "分支")
	if !okChip {
		t.Fatalf("范围芯片的文字没了（菜单没有可锚的东西）")
	}
	menuY, okMenu := labelY(opened, "未暂存")
	if !okMenu {
		t.Fatalf("打开的菜单里找不到条目“未暂存”：\n%s", openText)
	}
	if menuY <= chipY {
		t.Fatalf("范围菜单没画在芯片下方：菜单 Y=%v，芯片 Y=%v（官方是锚在 .select 下方的浮层）", menuY, chipY)
	}
}

// TestChooseScopeSendsTheScope 钉住菜单选中之后那一步（官方
// `onSelect={(id) => { setOpen(false); choose(id) }}`，`OFF:lib/client.js:6477-6480`）：
// 选中哪个就发哪个；没有动作面时静默返回而不是崩。
func TestChooseScopeSendsTheScope(t *testing.T) {
	p := &Plugin{}
	var got []string
	env := &pluginapi.Env{Act: pluginapi.Actions{Forge: &pluginapi.ForgeActions{
		LoadScope: func(scope string) { got = append(got, scope) },
	}}}
	p.chooseScope(env, apiclient.ForgeScopeStaged)
	if len(got) != 1 || got[0] != apiclient.ForgeScopeStaged {
		t.Fatalf("选中范围要发 LoadScope(%q)，实际 %v", apiclient.ForgeScopeStaged, got)
	}
	// 没有动作面（`Forge == nil`）：静默返回，不许 panic。
	p.chooseScope(&pluginapi.Env{}, apiclient.ForgeScopeBranch)
	if len(got) != 1 {
		t.Fatalf("没有动作面时不该发出请求，实际 %v", got)
	}
}

// labelY 找一句画出来的文字的 Y（用来判"有没有被下推"）。
func labelY(snapshot app.Snapshot, value string) (float32, bool) {
	for _, label := range snapshot.Labels {
		if label.Value == value {
			return label.Bounds.Y, true
		}
	}
	return 0, false
}

// hasHitLabel 判断这一帧有没有那个命中区标签。
func hasHitLabel(snapshot app.Snapshot, label string) bool {
	for _, hit := range snapshot.Hits {
		if hit.Label == label {
			return true
		}
	}
	return false
}
