package dsfileactions

// plugin_test.go —— 官方 file-actions 两段菜单判据的钉子（OFF:context-menus.tsx:49-104）。
//
// 只钉**判据**（哪个 target 给哪几项、什么情况下一项都不给、Run 交给谁），
// 不钉像素：本包不画任何可见面，唯一的绘制入口是那个"只记账"的 painter。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// fakeMenus 是 context menu 服务的替身：只把注册项收起来，供求值用。
type fakeMenus struct {
	contributions []pluginapi.ContextMenuContribution
	buttons       int
	attached      []pluginapi.ContextMenuTarget
}

func (m *fakeMenus) RegisterContribution(c pluginapi.ContextMenuContribution) (func(), error) {
	m.contributions = append(m.contributions, c)
	return func() {}, nil
}

func (m *fakeMenus) HasTarget(kind string) bool {
	for _, c := range m.contributions {
		if c.Target == kind {
			return true
		}
	}
	return false
}

func (m *fakeMenus) Attach(_ ui.C, _ gfx.ID, _ ui.Box, target pluginapi.ContextMenuTarget) {
	m.attached = append(m.attached, target)
}

func (m *fakeMenus) Button(_ ui.C, _ gfx.ID, _ pluginapi.ContextMenuTarget, _ string) bool {
	m.buttons++
	return true
}

// resolve 按 target kind 求值（与 dscontextmenu 的注册表同一口径：order 升序）。
func (m *fakeMenus) resolve(kind string, target pluginapi.ContextMenuTarget) []pluginapi.ContextMenuAction {
	var out []pluginapi.ContextMenuAction
	for _, c := range m.contributions {
		if c.Target != kind {
			continue
		}
		out = append(out, c.Resolve(target)...)
	}
	return out
}

// actionIDs 把动作摊成 id 列表（断言顺序用）。
func actionIDs(actions []pluginapi.ContextMenuAction) []string {
	out := make([]string, 0, len(actions))
	for _, a := range actions {
		out = append(out, a.ID)
	}
	return out
}

// newPlugin 造一个已接上菜单服务、且拿到壳动作的插件。
func newPlugin(save func(sessionID, path, name, mode string)) (*Plugin, *fakeMenus) {
	menus := &fakeMenus{}
	p := &Plugin{menus: menus}
	if save != nil {
		p.act = pluginapi.Actions{SaveWorkspaceFile: save}
	}
	if err := p.registerMenus(fakeContext{}); err != nil {
		panic(err)
	}
	return p, menus
}

// fakeContext 是最小 Context：只用 Effect 收卸载函数。
type fakeContext struct{ effects []func() }

func (fakeContext) Name() string        { return ServiceID }
func (fakeContext) Provide(string, any) {}
func (fakeContext) Inject(string) any   { return nil }
func (fakeContext) Effect(func())       {}
func (fakeContext) Logf(string, ...any) {}

// TestRegistersOfficialTargets 钉住"注册的是哪两个 target、什么顺序"。
func TestRegistersOfficialTargets(t *testing.T) {
	_, menus := newPlugin(func(string, string, string, string) {})
	if len(menus.contributions) != 2 {
		t.Fatalf("贡献数 = %d，want 2（file + image）", len(menus.contributions))
	}
	file, image := menus.contributions[0], menus.contributions[1]
	if file.ID != "file-actions.file" || file.Target != pluginapi.TargetFile || file.Order != 200 {
		t.Fatalf("file 贡献 = %s/%s/%d，want file-actions.file/file/200", file.ID, file.Target, file.Order)
	}
	if image.ID != "file-actions.image" || image.Target != pluginapi.TargetImage || image.Order != 100 {
		t.Fatalf("image 贡献 = %s/%s/%d，want file-actions.image/image/100", image.ID, image.Target, image.Order)
	}
	if !menus.HasTarget(pluginapi.TargetFile) || !menus.HasTarget(pluginapi.TargetImage) {
		t.Fatal("两个 target 都必须有贡献者（否则 contextMenu.button 不画）")
	}
}

// TestFileMenuKeepsImageDownload 钉住官方的三段展开条件在 canSaveAs=true 下的结果：
//
//	图片 → download + saveAs
//	非图片 → 只有 saveAs（官方 download 只在 image/ 或 !canSaveAs 时出现）
//
// copyImage 本仓不画（viva 没有剪贴板图片写入口），所以这里**只应有**这两项。
func TestFileMenuKeepsImageDownload(t *testing.T) {
	_, menus := newPlugin(func(string, string, string, string) {})
	target := pluginapi.ContextMenuTarget{Kind: pluginapi.TargetFile, SessionID: "s1", Path: "docs/a.png"}

	got := actionIDs(menus.resolve(pluginapi.TargetFile, target))
	if len(got) != 2 || got[0] != actionDownload || got[1] != actionSaveAs {
		t.Fatalf("图片 file target 的动作 = %v，want [download saveAs]（copyImage 按缺口不画）", got)
	}
	if labels := []string{menus.resolve(pluginapi.TargetFile, target)[0].Label, "下载"}; labels[0] != labels[1] {
		t.Fatalf("下载文案 = %q，want 官方 zh 字典的 %q", labels[0], labels[1])
	}

	plain := pluginapi.ContextMenuTarget{Kind: pluginapi.TargetFile, SessionID: "s1", Path: "docs/a.md"}
	got = actionIDs(menus.resolve(pluginapi.TargetFile, plain))
	if len(got) != 1 || got[0] != actionSaveAs {
		t.Fatalf("非图片 file target 的动作 = %v，want [saveAs]", got)
	}
}

// TestFileMenuNeedsIdentityAndShellAction 钉住"不给死按钮"：
// 缺会话身份 / 缺路径 / 壳没接动作 → 一个动作都不给（菜单不弹）。
func TestFileMenuNeedsIdentityAndShellAction(t *testing.T) {
	_, menus := newPlugin(func(string, string, string, string) {})
	for _, target := range []pluginapi.ContextMenuTarget{
		{Kind: pluginapi.TargetFile, Path: "a.png"},
		{Kind: pluginapi.TargetFile, SessionID: "s1"},
	} {
		if got := menus.resolve(pluginapi.TargetFile, target); len(got) != 0 {
			t.Fatalf("target %+v 求值出 %d 个动作，want 0", target, len(got))
		}
	}
	_, bare := newPlugin(nil) // 壳没接 SaveWorkspaceFile
	if got := bare.resolve(pluginapi.TargetFile, pluginapi.ContextMenuTarget{SessionID: "s1", Path: "a.png"}); len(got) != 0 {
		t.Fatalf("壳没接动作时求值出 %d 个动作，want 0", len(got))
	}
}

// TestImageMenuUsesCapturedTarget 钉住三件事：
//   - image target 需要 (sessionId, path)，只有 url 时不给动作（本仓取不到 URL 字节）；
//   - name 优先用 target.Name，缺了才退回路径最后一段；
//   - Run 交给壳的四个字段是**打开菜单时捕获的**，不是"当前选中"。
func TestImageMenuUsesCapturedTarget(t *testing.T) {
	var got [][4]string
	calls := func(sessionID, path, name, mode string) {
		got = append(got, [4]string{sessionID, path, name, mode})
	}
	_, menus := newPlugin(calls)

	urlOnly := pluginapi.ContextMenuTarget{Kind: pluginapi.TargetImage, URL: "https://example/a.png"}
	if actions := menus.resolve(pluginapi.TargetImage, urlOnly); len(actions) != 0 {
		t.Fatalf("只有 url 的 image target 求值出 %d 个动作，want 0（本仓取不到 URL 字节）", len(actions))
	}

	target := pluginapi.ContextMenuTarget{
		Kind: pluginapi.TargetImage, SessionID: "s7", Path: "img/shot.png", Name: "截图.png",
	}
	actions := menus.resolve(pluginapi.TargetImage, target)
	if ids := actionIDs(actions); len(ids) != 2 || ids[0] != actionDownload || ids[1] != actionSaveAs {
		t.Fatalf("image target 的动作 = %v，want [download saveAs]", ids)
	}
	actions[0].Run()
	actions[1].Run()
	if len(got) != 2 {
		t.Fatalf("Run 调用了 %d 次壳动作，want 2", len(got))
	}
	if got[0] != [4]string{"s7", "img/shot.png", "截图.png", pluginapi.FileDownload} {
		t.Fatalf("download 参数 = %v", got[0])
	}
	if got[1] != [4]string{"s7", "img/shot.png", "截图.png", pluginapi.FileSaveAs} {
		t.Fatalf("saveAs 参数 = %v", got[1])
	}
}

// TestImageMenuFallsBackToPathName 钉住 name 的兜底（官方 image target 的 name 可选）。
func TestImageMenuFallsBackToPathName(t *testing.T) {
	var name string
	_, menus := newPlugin(func(_, _, n, _ string) { name = n })
	actions := menus.resolve(pluginapi.TargetImage, pluginapi.ContextMenuTarget{
		Kind: pluginapi.TargetImage, SessionID: "s1", Path: `pics\a b.png`,
	})
	if len(actions) == 0 {
		t.Fatal("有 (sessionId, path) 的 image target 必须给动作")
	}
	actions[1].Run()
	if name != "a b.png" {
		t.Fatalf("建议文件名 = %q，want %q（反斜杠也要认）", name, "a b.png")
	}
}

// TestPainterRefreshesActions 钉住那个"不画东西的 painter"：
// 它每帧把 env.Act 抄进插件，菜单求值时用到的就是**这一帧**的壳动作。
func TestPainterRefreshesActions(t *testing.T) {
	p, menus := newPlugin(nil)
	if len(p.Panels()) != 1 {
		t.Fatalf("painter 数 = %d，want 1（只用来取 Actions）", len(p.Panels()))
	}
	painter := p.Panels()[0]
	if painter.Slot != pluginapi.SlotShellOverlay {
		t.Fatalf("painter 槽 = %q，want %q", painter.Slot, pluginapi.SlotShellOverlay)
	}
	// 空 env 不 panic（壳在某些装配里可能给 nil）。
	painter.Draw(nil, nil)
	var called bool
	painter.Draw(nil, &pluginapi.Env{Act: pluginapi.Actions{
		SaveWorkspaceFile: func(string, string, string, string) { called = true },
	}})
	actions := menus.resolve(pluginapi.TargetFile, pluginapi.ContextMenuTarget{SessionID: "s1", Path: "a.png"})
	if len(actions) != 2 {
		t.Fatalf("刷新后动作数 = %d，want 2", len(actions))
	}
	actions[1].Run()
	if !called {
		t.Fatal("Run 没有调用本帧注入的壳动作")
	}
}
