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
//
// 两条壳动作一起给：`SaveWorkspaceFile`（下载/另存为）与 `CopyWorkspaceImage`
// （复制图片，`#18` ①）。真壳两个总是一起接的；"只有其中一个"的老壳由
// TestMenusOmitCopyWithoutShellAction 单独钉。
func newPlugin(save func(sessionID, path, name, mode string)) (*Plugin, *fakeMenus) {
	menus := &fakeMenus{}
	p := &Plugin{menus: menus}
	if save != nil {
		p.act = pluginapi.Actions{
			SaveWorkspaceFile:  save,
			CopyWorkspaceImage: func(string, string) {},
		}
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

// TestFileMenuMatchesOfficialSequence 钉住官方的三段展开条件在 canSaveAs=true 下的结果：
//
//	图片   → copyImage + download + saveAs（官方顺序，OFF:context-menus.tsx:69-80）
//	非图片 → 只有 saveAs（官方 download 只在 image/ 或 !canSaveAs 时出现）
func TestFileMenuMatchesOfficialSequence(t *testing.T) {
	_, menus := newPlugin(func(string, string, string, string) {})
	target := pluginapi.ContextMenuTarget{Kind: pluginapi.TargetFile, SessionID: "s1", Path: "docs/a.png"}

	got := actionIDs(menus.resolve(pluginapi.TargetFile, target))
	if len(got) != 3 || got[0] != actionCopyImage || got[1] != actionDownload || got[2] != actionSaveAs {
		t.Fatalf("图片 file target 的动作 = %v，want [copyImage download saveAs]", got)
	}
	if labels := []string{menus.resolve(pluginapi.TargetFile, target)[0].Label, "复制图片"}; labels[0] != labels[1] {
		t.Fatalf("复制图片文案 = %q，want 官方 zh 字典的 %q", labels[0], labels[1])
	}
	if labels := []string{menus.resolve(pluginapi.TargetFile, target)[1].Label, "下载"}; labels[0] != labels[1] {
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

// TestImageMenuUsesCapturedTarget 钉住四件事：
//   - image target 需要 (sessionId, path)，只有 url 时不给动作（本仓取不到 URL 字节）；
//   - 三项的顺序与官方一致：copy → download → saveAs（OFF:context-menus.tsx:98-105）；
//   - name 优先用 target.Name，缺了才退回路径最后一段；
//   - 三条 Run 交给壳的字段都是**打开菜单时捕获的**，不是"当前选中"。
func TestImageMenuUsesCapturedTarget(t *testing.T) {
	var got [][4]string
	calls := func(sessionID, path, name, mode string) {
		got = append(got, [4]string{sessionID, path, name, mode})
	}
	var copied [][2]string
	p, menus := newPlugin(calls)
	p.act.CopyWorkspaceImage = func(sessionID, path string) {
		copied = append(copied, [2]string{sessionID, path})
	}

	urlOnly := pluginapi.ContextMenuTarget{Kind: pluginapi.TargetImage, URL: "https://example/a.png"}
	if actions := menus.resolve(pluginapi.TargetImage, urlOnly); len(actions) != 0 {
		t.Fatalf("只有 url 的 image target 求值出 %d 个动作，want 0（本仓取不到 URL 字节）", len(actions))
	}

	target := pluginapi.ContextMenuTarget{
		Kind: pluginapi.TargetImage, SessionID: "s7", Path: "img/shot.png", Name: "截图.png",
	}
	actions := menus.resolve(pluginapi.TargetImage, target)
	ids := actionIDs(actions)
	if len(ids) != 3 || ids[0] != actionCopy || ids[1] != actionDownload || ids[2] != actionSaveAs {
		t.Fatalf("image target 的动作 = %v，want [copy download saveAs]", ids)
	}
	actions[0].Run()
	actions[1].Run()
	actions[2].Run()
	if len(copied) != 1 {
		t.Fatalf("复制图片调用了 %d 次壳动作，want 1", len(copied))
	}
	if copied[0] != [2]string{"s7", "img/shot.png"} {
		t.Fatalf("复制图片参数 = %v，want 捕获的 s7|img/shot.png（剪贴板不需要文件名）", copied[0])
	}
	if len(got) != 2 {
		t.Fatalf("Run 调用了 %d 次保存动作，want 2", len(got))
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
	if len(actions) != 3 {
		t.Fatalf("有 (sessionId, path) 的 image target 必须给三条动作，实际 %d", len(actions))
	}
	actions[2].Run() // saveAs
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
	var saved, copied bool
	painter.Draw(nil, &pluginapi.Env{Act: pluginapi.Actions{
		SaveWorkspaceFile:  func(string, string, string, string) { saved = true },
		CopyWorkspaceImage: func(string, string) { copied = true },
	}})
	actions := menus.resolve(pluginapi.TargetFile, pluginapi.ContextMenuTarget{SessionID: "s1", Path: "a.png"})
	if len(actions) != 3 {
		t.Fatalf("刷新后动作数 = %d，want 3（copyImage/download/saveAs）", len(actions))
	}
	actions[0].Run()
	if !copied {
		t.Fatal("复制图片的 Run 没有调用本帧注入的壳动作")
	}
	actions[2].Run()
	if !saved {
		t.Fatal("另存为的 Run 没有调用本帧注入的壳动作")
	}
}

// TestFileMenuDisabledWhenStatFails 钉住官方的 **stat 预取**语义
// （OFF:context-menus.tsx:50-60 的 resolve 开头，以及 :71/:75/:79 的 `disabled: !available`）。
//
// 两条不能混：
//
//	stat 失败  → 动作**照样列出**、逐条 Disabled（菜单照弹，灰着）
//	身份不全   → 一个动作都不给（那是"要 stat 什么都不知道"，另一条用例）
//
// 第三档是壳没接这条探测（老宿主）：按可用画，不许悄悄全灰。
func TestFileMenuDisabledWhenStatFails(t *testing.T) {
	for _, tc := range []struct {
		name         string
		probe        func(sessionID, path string) bool
		wantDisabled bool
	}{
		{name: "stat 成功", probe: func(string, string) bool { return true }, wantDisabled: false},
		{name: "stat 失败", probe: func(string, string) bool { return false }, wantDisabled: true},
		{name: "壳没接探测", probe: nil, wantDisabled: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, menus := newPlugin(func(string, string, string, string) {})
			var asked string
			if tc.probe != nil {
				p.act.WorkspaceFileStat = func(sessionID, path string) bool {
					asked = sessionID + "|" + path
					return tc.probe(sessionID, path)
				}
			}
			target := pluginapi.ContextMenuTarget{
				Kind: pluginapi.TargetFile, SessionID: "s1", Path: "docs/a.png",
			}
			actions := menus.resolve(pluginapi.TargetFile, target)
			if len(actions) != 3 {
				t.Fatalf("动作数 = %d，want 3（copyImage/download/saveAs；stat 失败也必须照样列出，只是置灰）", len(actions))
			}
			for _, action := range actions {
				if action.Disabled != tc.wantDisabled {
					t.Fatalf("动作 %s 的 Disabled = %v，want %v", action.ID, action.Disabled, tc.wantDisabled)
				}
			}
			if tc.probe != nil && asked != "s1|docs/a.png" {
				t.Fatalf("stat 问的是 %q，want 打开菜单时捕获的 s1|docs/a.png", asked)
			}
		})
	}
}

// TestImageMenuNeverDisabled 钉住官方**只有 file 族**做 stat：image 族整段
// （OFF:context-menus.tsx:86-105）没有任何 disabled 判据，所以哪怕 file 族该灰，
// image 族也必须可点。
func TestImageMenuNeverDisabled(t *testing.T) {
	p, menus := newPlugin(func(string, string, string, string) {})
	p.act.WorkspaceFileStat = func(string, string) bool { return false }
	target := pluginapi.ContextMenuTarget{
		Kind: pluginapi.TargetImage, SessionID: "s1", Path: "img/a.png", Name: "a.png",
	}
	actions := menus.resolve(pluginapi.TargetImage, target)
	if len(actions) != 3 {
		t.Fatalf("image 动作数 = %d，want 3（copy/download/saveAs）", len(actions))
	}
	for _, action := range actions {
		if action.Disabled {
			t.Fatalf("image 动作 %s 被置灰：官方 image 菜单没有 stat/disabled（只有 file 族有）", action.ID)
		}
	}
}

// TestCopyImageMatchesOfficial 钉住**两处入口的复制项**逐字段对齐官方：
//
//	file  族 OFF:context-menus.tsx:70  id=copyImage · icon=file-actions.copy · separatorBefore=true
//	image 族 OFF:context-menus.tsx:99  id=copy      · icon=file-actions.copy · 无分隔线
//
// 两处文案同为 `menu.copyImage`（'复制图片'，OFF:locales.ts:9），disabled 只有
// file 族有（image 族整段没有 stat/available，见 TestImageMenuNeverDisabled）。
func TestCopyImageMatchesOfficial(t *testing.T) {
	_, menus := newPlugin(func(string, string, string, string) {})
	target := pluginapi.ContextMenuTarget{Kind: pluginapi.TargetFile, SessionID: "s1", Path: "docs/图 1.png"}

	fileActions := menus.resolve(pluginapi.TargetFile, target)
	copyImage := fileActions[0]
	if copyImage.ID != "copyImage" {
		t.Fatalf("file 族的复制项 id = %q，want %q（官方 :70）", copyImage.ID, "copyImage")
	}
	if copyImage.Icon != "file-actions.copy" {
		t.Fatalf("file 族复制项图标 = %q，want %q", copyImage.Icon, "file-actions.copy")
	}
	if !copyImage.SeparatorBefore {
		t.Fatal("file 族的复制项缺分隔线（官方 `separatorBefore: true`，:70）")
	}
	if copyImage.Label != "复制图片" {
		t.Fatalf("复制项文案 = %q，want 官方 zh 的 %q", copyImage.Label, "复制图片")
	}

	imageActions := menus.resolve(pluginapi.TargetImage, pluginapi.ContextMenuTarget{
		Kind: pluginapi.TargetImage, SessionID: "s1", Path: "img/a.png", Name: "a.png",
	})
	copyItem := imageActions[0]
	if copyItem.ID != "copy" {
		t.Fatalf("image 族的复制项 id = %q，want %q（官方 :99，两族 id 不同）", copyItem.ID, "copy")
	}
	if copyItem.Icon != "file-actions.copy" || copyItem.Label != "复制图片" {
		t.Fatalf("image 族复制项 = %q/%q，want 图标 file-actions.copy、文案 复制图片", copyItem.Icon, copyItem.Label)
	}
	if copyItem.SeparatorBefore {
		t.Fatal("image 族的复制项不该有分隔线（官方 :99 没有 separatorBefore）")
	}
}

// TestFileMenuCopyImageDisabledByStat 钉住 file 族复制项**同样吃 stat 判据**：
// 官方 `disabled: !available` 挂在这一条上（:71），stat 失败时它必须灰。
//
// 与 image 族区分开：那一族永远不灰（没有 stat）。
func TestFileMenuCopyImageDisabledByStat(t *testing.T) {
	p, menus := newPlugin(func(string, string, string, string) {})
	p.act.WorkspaceFileStat = func(string, string) bool { return false }
	actions := menus.resolve(pluginapi.TargetFile, pluginapi.ContextMenuTarget{SessionID: "s1", Path: "a.png"})
	if len(actions) != 3 {
		t.Fatalf("动作数 = %d，want 3", len(actions))
	}
	if !actions[0].Disabled {
		t.Fatal("stat 失败时复制图片仍可点：官方是 `disabled: !available`（:71）")
	}
}

// TestMenusOmitCopyWithoutShellAction 钉住"不给死按钮"：壳没接
// `Actions.CopyWorkspaceImage`（老壳 / 无头）时，复制项**不画**——
// 画出来就是点了没反应的项。另外两条保存项照旧（它们只依赖 SaveWorkspaceFile）。
func TestMenusOmitCopyWithoutShellAction(t *testing.T) {
	p, menus := newPlugin(func(string, string, string, string) {})
	p.act.CopyWorkspaceImage = nil

	fileIDs := actionIDs(menus.resolve(pluginapi.TargetFile, pluginapi.ContextMenuTarget{SessionID: "s1", Path: "a.png"}))
	if len(fileIDs) != 2 || fileIDs[0] != actionDownload || fileIDs[1] != actionSaveAs {
		t.Fatalf("壳没接复制图片时 file 族 = %v，want [download saveAs]", fileIDs)
	}
	imageIDs := actionIDs(menus.resolve(pluginapi.TargetImage, pluginapi.ContextMenuTarget{
		SessionID: "s1", Path: "img/a.png", Name: "a.png",
	}))
	if len(imageIDs) != 2 || imageIDs[0] != actionDownload || imageIDs[1] != actionSaveAs {
		t.Fatalf("壳没接复制图片时 image 族 = %v，want [download saveAs]", imageIDs)
	}
}

// TestCopyRunReadsHookAtCallTime 钉住 Run 里对 `p.act` 的 nil 复查：
//
// 菜单打开后壳侧动作面可能被换掉（painter 每帧覆盖 `p.act`，装配拆除时清空）。
// 那时 Run 必须**什么都不做**，而不是 panic —— 菜单项是用户已经点下去的回调，
// 崩在这里等于一次空指针崩溃。
func TestCopyRunReadsHookAtCallTime(t *testing.T) {
	p, menus := newPlugin(func(string, string, string, string) {})
	actions := menus.resolve(pluginapi.TargetFile, pluginapi.ContextMenuTarget{SessionID: "s1", Path: "a.png"})
	p.act.CopyWorkspaceImage = nil // 菜单已弹出，壳侧动作面被清空
	actions[0].Run()               // 不该 panic
}
