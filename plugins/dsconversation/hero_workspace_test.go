package dsconversation

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type heroWorkspaceHarness struct {
	p      *Plugin
	env    *pluginapi.Env
	tree   *gfx.Tree
	canvas *gfx.Canvas
	view   *ui.Page
}

func newHeroWorkspaceHarness(t *testing.T, data pluginapi.Snapshot, act pluginapi.Actions) *heroWorkspaceHarness {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	canvas := newHeroTestCanvas(tree)
	pageTheme := dscore.VivaTheme(theme)
	return &heroWorkspaceHarness{
		p:      &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}},
		env:    &pluginapi.Env{Theme: theme, Data: data, Act: act, Tree: tree, Feat: tree.Feature("hero-workspace-test"), Canvas: canvas},
		tree:   tree,
		canvas: canvas,
		view:   ui.NewPage(ui.PageConfig{Theme: &pageTheme}),
	}
}

func (h *heroWorkspaceHarness) frame(click string) {
	if click != "" {
		h.canvas.BeginEventPass(h.tree.ID(h.env.Feat, click), gfx.ClickInfo{})
	}
	h.canvas.Reset()
	h.view.Draw(h.canvas, gfx.Rect{Width: 900, Height: 700}, func(f ui.C) { h.p.drawHero(f, h.env) })
	if click != "" {
		h.canvas.EndEventPass()
	}
}

func TestHeroEmptyRegistryClickDirectlyOpensDirectoryFlow(t *testing.T) {
	opened := 0
	h := newHeroWorkspaceHarness(t, pluginapi.Snapshot{WorkspacesKnown: true}, pluginapi.Actions{
		OpenDirectoryPicker: func() { opened++ },
		SelectWorkspace:     func(string) {},
	})
	h.frame("")
	heroTestText(t, h.canvas, heroChooseWorkspaceText)
	// pump 到动作发生；断言的是点击真实 chip 后目录流升起，不是直接调用 Action。
	for i := 0; i < 8 && opened == 0; i++ {
		h.frame("hero-workspace")
	}
	if opened != 1 {
		t.Fatalf("空 registry 的官方单行优化应直接打开目录流一次，实际 %d", opened)
	}
	if h.p.workspaceMenuOpen {
		t.Fatal("空 registry 不应再展开只有一个添加项的菜单")
	}
}

func TestHeroWorkspaceMenuListsRealRegistryAndAddAction(t *testing.T) {
	h := newHeroWorkspaceHarness(t, pluginapi.Snapshot{
		WorkspacesKnown: true,
		Workspaces: []pluginapi.Workspace{
			{WorkspaceID: "w1", Title: "Alpha", Path: `D:\work\alpha`},
			{WorkspaceID: "w2", Title: "Beta", Path: `D:\work\beta`},
		},
	}, pluginapi.Actions{OpenDirectoryPicker: func() {}, SelectWorkspace: func(string) {}})
	h.frame("")
	for i := 0; i < 8 && !h.p.workspaceMenuOpen; i++ {
		h.frame("hero-workspace")
	}
	h.frame("")
	for _, want := range []string{"Alpha", "Beta", heroAddWorkspaceText} {
		heroTestText(t, h.canvas, want)
	}
	if !h.p.workspaceMenuOpen {
		t.Fatal("有多个可选目标时 chip 应展开菜单，而不是直接打开目录流")
	}
}

func TestHeroWorkspaceLabelPendingAndDeletedOwner(t *testing.T) {
	env := &pluginapi.Env{Data: pluginapi.Snapshot{
		Cur: "s1", WorkspacesKnown: true, PendingWorkspaceID: "w2",
		Workspaces: []pluginapi.Workspace{
			{WorkspaceID: "w1", Title: "Old", SessionIDs: []string{"s1"}},
			{WorkspaceID: "w2", Title: "Pending"},
		},
	}}
	if got := heroWorkspaceLabel(env); got != "Pending" {
		t.Fatalf("pending 工作区应优先于旧 owner，实际 %q", got)
	}
	env.Data.PendingWorkspaceID = ""
	env.Data.Workspaces = env.Data.Workspaces[1:]
	env.Data.Sessions = nil
	env.Data.CWD = `D:\deleted\old`
	if got := heroWorkspaceLabel(env); got != "" {
		t.Fatalf("registry ready 且 owner 已删除时必须回 placeholder，实际 %q", got)
	}
}
