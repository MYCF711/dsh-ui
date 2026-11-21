package dsterminal

// guide_test.go —— 官方向导卡与芯片徽标的机器断言。
//
// 依据全在官方 rc.2 随包 src/**：
//
//	几何      OF:dsh-client-ui-sidebar-terminal/src/client/TerminalGuide.module.css:1-12
//	徽标数据  OF:…/TerminalTitle.tsx:17-20,42-50
//	徽标文案  OF:…/locales.ts:26-29
//	座位注册  OF:…/index.ts:72-75（guide 声明）、:87-94（guide.entry 座位）

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestOfficialTerminalGuideGeometry(t *testing.T) {
	checks := []struct {
		name string
		got  float32
		want float32
	}{
		{"entry gap", guideEntryGap, 14},
		{"entry min-height", guideEntryMinHeight, 56},
		{"entry padding-y", guideEntryPadY, 14},
		{"entry padding-x", guideEntryPadX, 20},
		{"entry border", guideEntryBorder, 0.5},
		{"icon box", guideIconBox, 26},
		{"text gap", guideTextGap, 3},
		{"titleRow gap", guideTitleRowGap, 4},
		{"title font", guideTitleFont, 14},
		{"title line", guideTitleLine, 14 * 1.4},
		{"description font", guideDescFont, 11},
		{"description line", guideDescLine, 11 * 1.4},
		{"trigger size", guideTriggerSize, 20},
		{"trigger radius", guideTriggerRadius, 4},
		{"trigger chevron", guideChevronSize, 14},
		// `.icon` 里 `PluginArtworkTerminal size={description === undefined ? 22 : 26}`
		// （TerminalGuide.tsx:58）：没有描述时官方画 22。
		{"icon box (no description)", guideIconBoxCompact, 22},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %v，官方值 %v", check.name, check.got, check.want)
		}
	}
	// `.menu{display:flex;flex:none}`（:11）：面与行都是 primitives `Menu` 的，
	// 读数只有一份（`plugins/dscore/menu.go`），这里钉住"没有另编一套数"。
	if guideMenuRowPadX != 8 || guideMenuFont != 13 || guideMenuLine != 20 || guideCheckSize != 16 {
		t.Errorf("菜单行度量漂了：padX=%v font=%v line=%v check=%v（官方 .cell 8 / 13 / 20 / sprite 16）",
			guideMenuRowPadX, guideMenuFont, guideMenuLine, guideCheckSize)
	}
	// 卡片图标是官方 `PluginArtworkTerminal` 的 36-box / 3.5 stroke，不是本包自画的
	// 16-box 终端字形（`dsh-web-frontend/dist/assets/index-ghOpC9uV.js` 的 xK 组件）。
	p := &Plugin{}
	if icon := p.guideIcon(); icon.ViewBox != 36 || icon.Stroke != 3.5 {
		t.Errorf("guide 图标几何 = viewBox %v / stroke %v，官方 36 / 3.5", icon.ViewBox, icon.Stroke)
	}
}

func TestRightTabBadgeFollowsTerminalTitleBranches(t *testing.T) {
	p := &Plugin{}
	// 壳没接终端快照：没有依据就是没有徽标。
	if _, ok := p.RightTabBadge("s1", "tab1"); ok {
		t.Fatal("TerminalSnapshot 未接线时不该给徽标")
	}
	states := map[string]pluginapi.TerminalViewState{}
	p.act.TerminalSnapshot = func(tabID string) pluginapi.TerminalViewState { return states[tabID] }

	cases := []struct {
		name      string
		state     pluginapi.TerminalViewState
		wantBadge bool
		wantText  string
		wantTone  pluginapi.TabBadgeTone
		wantHint  string
	}{
		{
			name:      "没有 info",
			state:     pluginapi.TerminalViewState{},
			wantBadge: false,
		},
		{
			name:      "用户终端（origin=user）",
			state:     pluginapi.TerminalViewState{Info: &apiclient.TerminalInfo{Origin: "user"}},
			wantBadge: false,
		},
		{
			name: "Agent 控制",
			state: pluginapi.TerminalViewState{Info: &apiclient.TerminalInfo{
				Origin: "agent", Control: "agent"}},
			wantBadge: true, wantText: zhBadgeAgent,
			wantTone: pluginapi.TabBadgeInfo, wantHint: zhBadgeAgentHint,
		},
		{
			name: "Agent 只读沙箱、未接管",
			state: pluginapi.TerminalViewState{Info: &apiclient.TerminalInfo{
				Origin: "agent", Control: "user", SandboxMode: "read-only"}},
			wantBadge: true, wantText: zhBadgeSandbox,
			wantTone: pluginapi.TabBadgeNeutral, wantHint: zhBadgeReadOnlyHint,
		},
		{
			name: "Agent 工作区沙箱、已控制（两段提示拼起来）",
			state: pluginapi.TerminalViewState{Info: &apiclient.TerminalInfo{
				Origin: "agent", Control: "agent", SandboxMode: "workspace-write"}},
			wantBadge: true, wantText: zhBadgeAgent,
			wantTone: pluginapi.TabBadgeInfo, wantHint: zhBadgeAgentHint + " " + zhBadgeWriteHint,
		},
		{
			name: "Agent 打开但没有控制权、沙箱未知",
			state: pluginapi.TerminalViewState{Info: &apiclient.TerminalInfo{
				Origin: "agent", Control: "user"}},
			wantBadge: false,
		},
	}
	for _, one := range cases {
		states["tab1"] = one.state
		badge, ok := p.RightTabBadge("s1", "tab1")
		if ok != one.wantBadge {
			t.Fatalf("%s：有徽标 = %v，want %v", one.name, ok, one.wantBadge)
		}
		if !ok {
			continue
		}
		if badge.Text != one.wantText || badge.Tone != one.wantTone || badge.Hint != one.wantHint {
			t.Fatalf("%s：徽标 = %+v，want {%q %v %q}", one.name, badge, one.wantText, one.wantTone, one.wantHint)
		}
	}
}

func TestGuideEntryRegistrationIsKeyedByKind(t *testing.T) {
	p := &Plugin{}
	var typePanel, cardPanel *pluginapi.Painter
	panels := p.Panels()
	for i := range panels {
		switch panels[i].Slot {
		case pluginapi.SlotRightPaneTab:
			typePanel = &panels[i]
		case pluginapi.SlotRightGuideEntry:
			cardPanel = &panels[i]
		}
	}
	if typePanel == nil {
		t.Fatal("类型画（sidebar.right.pane.tab）没注册")
	}
	if cardPanel == nil {
		t.Fatal("向导卡（sidebar.right.tab.guide.entry）没注册")
	}
	// 官方 index.ts:74 的 guide 声明：id 'new'、order 20、title '新建终端'、description。
	if len(typePanel.Guide) != 1 {
		t.Fatalf("guide 声明条数 = %d，want 1", len(typePanel.Guide))
	}
	decl := typePanel.Guide[0]
	if decl.ID != "new" || decl.Order != 20 || decl.Title != zhGuideTitle ||
		decl.Description != zhGuideDescription || decl.IconImage.ViewBox == 0 {
		t.Fatalf("guide 声明 = %+v，want {new 20 新建终端 在会话工作区运行命令 icon}", decl)
	}
	// keyed 座位的 key = 类型的 kind（官方 entryKey: entry.providerId；本仓类型身份就是 kind）。
	if cardPanel.Key != typePanel.Key || cardPanel.Key != RightTabTerminal {
		t.Fatalf("向导卡 key = %q，类型 key = %q，want 都是 %q",
			cardPanel.Key, typePanel.Key, RightTabTerminal)
	}
	if cardPanel.Draw == nil {
		t.Fatal("向导卡没有绘制函数")
	}
}

func TestOfficialChevronAndCheckAssets(t *testing.T) {
	for name, icon := range map[string]gfx.Icon{
		"chevron-down": terminalChevronDownIcon(),
		"check":        terminalCheckIcon(),
	} {
		if icon.ViewBox != 16 || icon.Stroke != 1 || len(icon.Commands) == 0 {
			t.Fatalf("%s 没有复用官方 sprite 路径：%+v", name, icon)
		}
	}
}

// guideVisualCanvas 用一块离屏 canvas 真画一帧向导卡片（与 visual_test.go 同一套夹具：
// 离屏 canvas + 固定文字测量），回来后读 `canvas.Texts()` / `canvas.Hits()`。
func guideVisualCanvas(t *testing.T, draw func(f ui.C, env *pluginapi.Env)) (*gfx.Canvas, *pluginapi.Env) {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("terminal-guide-visual"), Canvas: canvas,
		Data: pluginapi.Snapshot{Cur: "s1"},
		GuideEntry: &pluginapi.GuideEntryProps{
			EntryID: "new", Kind: RightTabTerminal,
			Title: zhGuideTitle, Description: zhGuideDescription,
		},
	}
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	page.Draw(canvas, gfx.Rect{Width: 380, Height: 240}, func(f ui.C) { draw(f, env) })
	return canvas, env
}

func TestTerminalGuideCardDrawsOfficialCopy(t *testing.T) {
	p := &Plugin{}
	opened := ""
	canvas, env := guideVisualCanvas(t, func(f ui.C, env *pluginapi.Env) {
		env.Act.OpenRightTab = func(kind string) { opened = kind }
		p.drawGuideEntry(f, env)
	})
	texts := map[string]gfx.Text{}
	for _, one := range canvas.Texts() {
		texts[one.Value] = one
	}
	title, ok := texts[zhGuideTitle]
	if !ok {
		t.Fatalf("卡片标题没画出来：%v", canvas.Texts())
	}
	// `.title{font-size:14px;line-height:1.4;color:label-primary}`
	if title.Style.Size != guideTitleFont || title.Style.Color != env.Theme.LabelPrimary {
		t.Fatalf("标题样式 = %+v，want 14px / label-primary", title.Style)
	}
	desc, ok := texts[zhGuideDescription]
	if !ok {
		t.Fatalf("卡片描述没画出来：%v", canvas.Texts())
	}
	// `.description{font-size:11px;line-height:1.4;color:label-tertiary}`
	if desc.Style.Size != guideDescFont || desc.Style.Color != env.Theme.LabelTertiary {
		t.Fatalf("描述样式 = %+v，want 11px / label-tertiary", desc.Style)
	}
	// `.trigger` 的无障碍名是官方 `t('shell')`（TerminalGuide.tsx:73 的 aria-label）。
	found := false
	for _, hit := range canvas.Hits() {
		if hit.Label == zhGuideShell {
			found = true
		}
	}
	if !found {
		t.Fatal("展开 shell 菜单的 trigger 没有官方 aria-label")
	}
	_ = opened
}

func TestTerminalGuideMenuDrawsShellStates(t *testing.T) {
	ready := pluginapi.TerminalShellChoices{
		Phase: pluginapi.TerminalShellsReady,
		Shells: []apiclient.TerminalShell{
			{Path: "/bin/bash", Name: "bash"},
			{Path: "/bin/zsh", Name: "zsh"},
		},
		Selected: "bash",
	}
	cases := []struct {
		name    string
		choices pluginapi.TerminalShellChoices
		want    []string
	}{
		{"ready", ready, []string{"bash", "zsh"}},
		{"loading", pluginapi.TerminalShellChoices{Phase: pluginapi.TerminalShellsLoading}, []string{zhGuideShellLoad}},
		{"failed", pluginapi.TerminalShellChoices{Phase: pluginapi.TerminalShellsFailed, Message: "boom"},
			[]string{zhGuideFailedLead + "boom", zhGuideRetry}},
		{"empty", pluginapi.TerminalShellChoices{Phase: pluginapi.TerminalShellsReady}, []string{zhGuideShellEmpty}},
	}
	for _, one := range cases {
		p := &Plugin{guideMenuOpen: true}
		canvas, env := guideVisualCanvas(t, func(f ui.C, env *pluginapi.Env) {
			env.Act.OpenRightTab = func(string) {}
			env.Act.OpenRightTabParams = func(string, pluginapi.RightTabParams) {}
			env.Act.TerminalLaunchShells = func() pluginapi.TerminalShellChoices { return one.choices }
			p.drawGuideEntry(f, env)
		})
		texts := map[string]bool{}
		for _, text := range canvas.Texts() {
			texts[text.Value] = true
		}
		for _, want := range one.want {
			if !texts[want] {
				t.Fatalf("%s：菜单行 %q 没画出来：%v", one.name, want, canvas.Texts())
			}
		}
		_ = env
	}
}

// TestTerminalGuideCardDrawsOfficialShortcutKeys 钉住官方 `ShortcutKeys`
// （TerminalGuide.tsx:81）的接线与几何：键序来自 `Actions.ShortcutKeys`，
// 每枚元素按 `._keys_38b9q_1` 的 12px/16、gap 3 画；没有目录时一个键都不画。
func TestTerminalGuideCardDrawsOfficialShortcutKeys(t *testing.T) {
	if guideShortcutGap != 3 || guideShortcutFont != 12 || guideShortcutLine != 16 {
		t.Fatalf("ShortcutKeys 几何漂了：gap=%v font=%v line=%v", guideShortcutGap, guideShortcutFont, guideShortcutLine)
	}
	draw := func(withCatalog bool) map[string]gfx.Text {
		p := &Plugin{}
		canvas, _ := guideVisualCanvas(t, func(f ui.C, env *pluginapi.Env) {
			env.Act.OpenRightTab = func(string) {}
			if withCatalog {
				env.Act.ShortcutKeys = func(commandID string) []string {
					if commandID != terminalNewCommandID {
						t.Fatalf("向导卡问错了 command id：%q", commandID)
					}
					return []string{"Ctrl", "+", "`"}
				}
			}
			p.drawGuideEntry(f, env)
		})
		out := map[string]gfx.Text{}
		for _, one := range canvas.Texts() {
			out[one.Value] = one
		}
		return out
	}
	texts := draw(true)
	for _, key := range []string{"Ctrl", "+", "`"} {
		one, ok := texts[key]
		if !ok {
			t.Fatalf("ShortcutKeys 元素 %q 没画出来：%v", key, texts)
		}
		if one.Style.Size != guideShortcutFont || one.Style.Color != dscore.Dark().LabelTertiary {
			t.Fatalf("ShortcutKeys 元素 %q 样式 = %+v", key, one.Style)
		}
	}
	if got := draw(false); got["Ctrl"].Value != "" {
		t.Fatal("没有快捷键目录时不该画 ShortcutKeys")
	}
}

func TestGuideMenuRowPicksShellAndOpensTab(t *testing.T) {
	p := &Plugin{guideMenuOpen: true}
	var params pluginapi.RightTabParams
	kind := ""
	selected := ""
	env := &pluginapi.Env{Data: pluginapi.Snapshot{Cur: "s1"}}
	env.Act.TerminalSelectShell = func(path string) { selected = path }
	env.Act.OpenRightTabParams = func(k string, got pluginapi.RightTabParams) { kind, params = k, got }
	env.Act.OpenRightTab = func(string) {}
	choices := pluginapi.TerminalShellChoices{
		Phase:  pluginapi.TerminalShellsReady,
		Shells: []apiclient.TerminalShell{{Path: "/bin/zsh", Name: "zsh"}},
	}
	rows := guideMenuRows(choices, env, p)
	if len(rows) != 1 || rows[0].label != "zsh" || rows[0].disabled || rows[0].pick == nil {
		t.Fatalf("ready 档菜单行 = %+v，want 一行可选 zsh", rows)
	}
	rows[0].pick()
	if kind != RightTabTerminal || params.ShellPath != "/bin/zsh" {
		t.Fatalf("选 shell 后 openTab = %q %+v，want terminal + shellPath", kind, params)
	}
	if selected != "/bin/zsh" {
		t.Fatalf("选 shell 没有调用 selectShell：%q", selected)
	}
	if p.guideMenuOpen {
		t.Fatal("选完 shell 菜单应关闭（官方 onSelect 里 setOpen(false)）")
	}
}
