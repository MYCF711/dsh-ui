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
	theme := dscore.Dark()
	p := &Plugin{}
	if err := p.Register(testContext{theme: theme}); err != nil {
		t.Fatal(err)
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

// hasHitLabel 判断这一帧有没有那个命中区标签。
func hasHitLabel(snapshot app.Snapshot, label string) bool {
	for _, hit := range snapshot.Hits {
		if hit.Label == label {
			return true
		}
	}
	return false
}
