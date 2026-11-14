package dssidebar

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// fakeMenus 记录 dssidebar 在 Register 期登记了什么；渲染入口只计数。
type fakeMenus struct {
	contributions []pluginapi.ContextMenuContribution
	has           map[string]bool
	attached      int
	buttons       int
}

func (f *fakeMenus) RegisterContribution(c pluginapi.ContextMenuContribution) (func(), error) {
	f.contributions = append(f.contributions, c)
	if f.has == nil {
		f.has = map[string]bool{}
	}
	f.has[c.Target] = true
	return func() {}, nil
}
func (f *fakeMenus) HasTarget(kind string) bool { return f.has[kind] }
func (f *fakeMenus) Attach(ui.C, ui.Box, pluginapi.ContextMenuTarget) {
	f.attached++
}
func (f *fakeMenus) Button(ui.C, gfx.ID, pluginapi.ContextMenuTarget, string) bool {
	f.buttons++
	return true
}

type fakeContext struct {
	menus  pluginapi.ContextMenus
	effect []func()
}

func (f *fakeContext) Name() string        { return "test" }
func (f *fakeContext) Provide(string, any) {}
func (f *fakeContext) Inject(key string) any {
	if key == pluginapi.KeyContextMenus {
		return f.menus
	}
	return nil
}
func (f *fakeContext) Effect(fn func())    { f.effect = append(f.effect, fn) }
func (f *fakeContext) Logf(string, ...any) {}

func TestRegisterContextMenusRegistersWorkspaceAndSession(t *testing.T) {
	menus := &fakeMenus{}
	p := &Plugin{menus: menus}
	if err := p.registerContextMenus(&fakeContext{menus: menus}); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, c := range menus.contributions {
		got[c.Target+"/"+c.ID] = true
	}
	for _, want := range []string{"workspace/workspace.rename", "workspace/workspace.delete", "session/session.copy-id"} {
		if !got[want] {
			t.Fatalf("缺少菜单贡献 %q；实际 %v", want, got)
		}
	}
}

func TestWorkspaceMenuResolveRequiresRealActions(t *testing.T) {
	menus := &fakeMenus{}
	p := &Plugin{menus: menus}
	if err := p.registerContextMenus(&fakeContext{menus: menus}); err != nil {
		t.Fatal(err)
	}
	var rename, remove pluginapi.ContextMenuContribution
	for _, c := range menus.contributions {
		switch c.ID {
		case "workspace.rename":
			rename = c
		case "workspace.delete":
			remove = c
		}
	}
	target := pluginapi.ContextMenuTarget{Kind: "workspace", ID: "w1", DisplayTitle: "A"}
	if got := rename.Resolve(target); len(got) != 0 {
		t.Fatalf("壳没接 WorkspaceRename 时不能画动作：%+v", got)
	}
	if got := remove.Resolve(target); len(got) != 0 {
		t.Fatalf("壳没接 WorkspaceDelete 时不能画动作：%+v", got)
	}
	p.act = pluginapi.Actions{
		WorkspaceRename: func(string, string) error { return nil },
		WorkspaceDelete: func(string) error { return nil },
	}
	if got := rename.Resolve(target); len(got) != 1 || got[0].Label != "重命名" {
		t.Fatalf("rename 动作 = %+v", got)
	}
	if got := remove.Resolve(target); len(got) != 1 || got[0].Label != "删除工作区" || !got[0].Danger || !got[0].SeparatorBefore {
		t.Fatalf("delete 动作 = %+v（必须 destructive + 分隔线）", got)
	}
}

func TestWorkspaceMenuTargetMatchesRegistryPath(t *testing.T) {
	workspaces := []pluginapi.Workspace{
		{WorkspaceID: "w1", Path: `D:\work\Alpha`, Title: "客户项目"},
		{WorkspaceID: "w2", Path: `D:\work\beta`},
	}
	target, ok := workspaceMenuTarget(workspaces, `d:/WORK/alpha/`)
	if !ok || target.ID != "w1" || target.DisplayTitle != "客户项目" {
		t.Fatalf("大小写/斜杠不同的同一路径应命中 w1：%+v ok=%v", target, ok)
	}
	target, ok = workspaceMenuTarget(workspaces, `D:\work\beta`)
	if !ok || target.DisplayTitle != "beta" {
		t.Fatalf("没有 title 时显示名退回目录基名：%+v", target)
	}
	if _, ok := workspaceMenuTarget(workspaces, `D:\work\gamma`); ok {
		t.Fatal("registry 里没有的组不能假装有工作区身份")
	}
}

func TestRequestWorkspaceActionSeedsDraftAndClosesOnlyWhenIdle(t *testing.T) {
	p := &Plugin{}
	p.requestWorkspaceAction(workspaceActionRename, pluginapi.ContextMenuTarget{
		Kind: "workspace", ID: "w1", DisplayTitle: "旧名字", Path: `D:\a`,
	})
	request, busy, errText := p.workspaceActionSnapshot()
	if request.kind != workspaceActionRename || request.workspaceID != "w1" || busy || errText != "" {
		t.Fatalf("请求 = %+v busy=%v err=%q", request, busy, errText)
	}
	if got := p.wsRename.Text(); got != "旧名字" {
		t.Fatalf("重命名输入框预填 = %q，want 旧名字", got)
	}
	p.wsMu.Lock()
	p.wsBusy = true
	p.wsMu.Unlock()
	p.closeWorkspaceAction()
	if request, _, _ := p.workspaceActionSnapshot(); request.kind == "" {
		t.Fatal("写操作在途时不能关闭对话框（官方 renaming/deleting 同义）")
	}
	p.wsMu.Lock()
	p.wsBusy = false
	p.wsMu.Unlock()
	p.closeWorkspaceAction()
	if request, _, _ := p.workspaceActionSnapshot(); request.kind != "" {
		t.Fatal("空闲时应能关闭")
	}
}

// TestWorkspaceDialogsRenderBothModes 走真实 Page 绘制，证明两种模态都能声明出来
// （TextField + 取消/确认按钮），不是只在状态层测过。
func TestWorkspaceDialogsRenderBothModes(t *testing.T) {
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feature := tree.Feature("workspace-dialog-test")
	env := &pluginapi.Env{Theme: theme, Tree: tree, Feat: feature, Bounds: gfx.Rect{Width: 800, Height: 600}}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	for _, kind := range []string{workspaceActionRename, workspaceActionDelete} {
		p := &Plugin{theme: theme}
		p.requestWorkspaceAction(kind, pluginapi.ContextMenuTarget{
			Kind: "workspace", ID: "w1", DisplayTitle: "客户项目", Path: `D:\work\alpha`,
		})
		page.Draw(canvas, gfx.Rect{Width: 800, Height: 600}, func(f ui.C) {
			p.drawWorkspaceDialogs(f, env)
		})
	}
}
