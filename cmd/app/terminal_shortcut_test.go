// terminal_shortcut_test.go —— 官方 `terminal.new`（`Ctrl+Backquote`）与芯片改名的 Esc 归属。
//
// 两者都是**壳层**的行为（不在某个插件的 HandleKey 里），所以按 `Ctrl+O` 的老规矩
// 直接喂 `gfx.KeyEvent` 验：viva 只把带修饰键的可打印键送到 `app.Config.OnShortcut`
// （`viva@v0.6.4/app/app.go:130-142`），OnKey 那一层收到的是 `KeyUnknown + Text`。
package main

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsterminal"
	"github.com/vivaui/viva/gfx"
)

func newShortcutModel(t *testing.T, sessionID string) *model {
	t.Helper()
	kernel := uikernel.New("terminal-shortcut")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dsterminal.Plugin{}); err != nil {
		t.Fatalf("装配内核：%v", err)
	}
	m := &model{kernel: kernel}
	if sessionID != "" {
		m.cur = sessionID
		m.rightTabLoaded = map[string]bool{sessionID: true}
	}
	return m
}

// terminalTabsAfterProjection 触发一次右栏投影（它会播种产品入口那枚），返回播种后的数量。
func terminalTabsAfterProjection(m *model, sessionID string) []string {
	m.rightTabsFor(nil)
	return terminalTabs(m, sessionID)
}

func terminalTabs(m *model, sessionID string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	surface := m.rightTabSurfaces[sessionID]
	if surface == nil {
		return nil
	}
	var out []string
	for _, tab := range surface.Tabs {
		if tab.Kind == rightTabKindTerminal {
			out = append(out, tab.ID)
		}
	}
	return out
}

// TestTerminalNewShortcutOpensTerminalTab 钉住官方 `run: openTabFromTarget('terminal', target)`。
func TestTerminalNewShortcutOpensTerminalTab(t *testing.T) {
	m := newShortcutModel(t, "s1")
	ev := gfx.KeyEvent{Key: gfx.KeyUnknown, Modifiers: gfx.ModifierControl, Text: "`"}
	// 右栏布局里本来就有产品入口种下的那一枚（`defaultRightTabSurface`，
	// 每个注册类型先一枚），所以判据是"每按一次 +1"，不是"总共一枚"。
	// 第一次触发布局投影时才会播种，所以基线要在**按下之前**取。
	if got := len(terminalTabs(m, "s1")); got != 0 {
		t.Fatalf("初次投影前不该有 occurrence，实际 %d", got)
	}
	seeded := len(terminalTabsAfterProjection(m, "s1"))
	if !m.handleTerminalNewShortcut(ev) {
		t.Fatal("Ctrl+` 没被当成 terminal.new 处理")
	}
	// terminal 是 multiple:true：每次按都铸一枚新的（不是复选/复用）。
	if got := len(terminalTabs(m, "s1")); got != seeded+1 {
		t.Fatalf("第一次 Ctrl+` 后终端 tab 数 = %d（%v），want %d", got, terminalTabs(m, "s1"), seeded+1)
	}
	if !m.handleTerminalNewShortcut(ev) {
		t.Fatal("第二次 Ctrl+` 没被处理")
	}
	if got := len(terminalTabs(m, "s1")); got != seeded+2 {
		t.Fatalf("第二次 Ctrl+` 后终端 tab 数 = %d，want %d（official multiple:true）", got, seeded+2)
	}
}

// TestTerminalNewShortcutBlockedWithoutSession 钉住官方
// `{ status:'blocked', reason: t('shortcut.noSession') }`：不静默。
func TestTerminalNewShortcutBlockedWithoutSession(t *testing.T) {
	m := newShortcutModel(t, "")
	ev := gfx.KeyEvent{Key: gfx.KeyUnknown, Modifiers: gfx.ModifierControl, Text: "`"}
	if !m.handleTerminalNewShortcut(ev) {
		t.Fatal("没有会话时 Ctrl+` 也应被吃掉（官方 blocked 也返回 handled 那一支的理由）")
	}
	if m.status != terminalNewShortcutNoSession {
		t.Fatalf("状态栏 = %q，want 官方文案 %q", m.status, terminalNewShortcutNoSession)
	}
	if len(m.rightTabSurfaces) != 0 {
		t.Fatalf("没有会话时不该铸 occurrence：%v", m.rightTabSurfaces)
	}
}

// TestTerminalNewShortcutIgnoresOtherChords：只认 Ctrl+` 这一档，别的 chord 一律放行。
func TestTerminalNewShortcutIgnoresOtherChords(t *testing.T) {
	cases := []struct {
		name string
		ev   gfx.KeyEvent
	}{
		{"Ctrl+O 归 workspace.add", gfx.KeyEvent{Key: gfx.KeyUnknown, Modifiers: gfx.ModifierControl, Text: "o"}},
		{"不带修饰的反引号是普通输入", gfx.KeyEvent{Key: gfx.KeyUnknown, Text: "`"}},
		{"Ctrl+Shift+` 不是这个 chord", gfx.KeyEvent{Key: gfx.KeyUnknown, Modifiers: gfx.ModifierControl | gfx.ModifierShift, Text: "`"}},
		{"Ctrl+Alt+` 不是这个 chord", gfx.KeyEvent{Key: gfx.KeyUnknown, Modifiers: gfx.ModifierControl | gfx.ModifierAlt, Text: "`"}},
		{"具名键不走这一层", gfx.KeyEvent{Key: gfx.KeyF5, Modifiers: gfx.ModifierControl}},
		{"按下不算重复处理", gfx.KeyEvent{Key: gfx.KeyUnknown, Modifiers: gfx.ModifierControl, Text: "`", Release: true}},
	}
	m := newShortcutModel(t, "s1")
	for _, one := range cases {
		if m.handleTerminalNewShortcut(one.ev) {
			t.Fatalf("%s：不该被 terminal.new 吃掉", one.name)
		}
	}
	if tabs := terminalTabs(m, "s1"); len(tabs) != 0 {
		t.Fatalf("这些 chord 不该开 tab，实际开了 %v", tabs)
	}
}

// TestShortcutHandlerWiresTerminalNew 钉住**接线**：`cfg.OnShortcut` 指到 shortcutHandler，
// 而 shortcutHandler 真的把 Ctrl+Backquote 交给 terminal.new。
//
// 为什么单独一条：把 main.go 里那句 `cfg.OnShortcut = m.shortcutHandler` 注释掉，
// 只测 handler 的用例仍然全绿（2026-11-18 变异实测），所以接线必须有测试。
func TestShortcutHandlerWiresTerminalNew(t *testing.T) {
	m := newShortcutModel(t, "s1")
	seeded := len(terminalTabsAfterProjection(m, "s1"))
	result := m.shortcutHandler(gfx.KeyEvent{Key: gfx.KeyUnknown, Modifiers: gfx.ModifierControl, Text: "`"})
	if !result.Handled || !result.Repaint {
		t.Fatalf("Ctrl+` 经 shortcutHandler 未被处理：%+v", result)
	}
	if got := len(terminalTabs(m, "s1")); got != seeded+1 {
		t.Fatalf("shortcutHandler 没把 Ctrl+` 落到 terminal.new：%d → %d", seeded, got)
	}
	// Ctrl+O 仍然归 workspace.add（没有目录选择器时它返回 blocked，也是 handled）。
	if result := m.shortcutHandler(gfx.KeyEvent{Key: gfx.KeyUnknown, Modifiers: gfx.ModifierControl, Text: "o"}); !result.Handled {
		t.Fatalf("Ctrl+O 经 shortcutHandler 未被处理：%+v", result)
	}
	// 真窗口的那条接线：`newConfig` 必须把 OnShortcut 指到同一个函数。
	// 这一段是关键——只测 shortcutHandler 而不断言 cfg.OnShortcut，把 main.go
	// 里那一行换掉照样全绿（2026-11-18 变异实测）。
	cfg := newConfig(m)
	if cfg.OnShortcut == nil {
		t.Fatal("newConfig 没接 OnShortcut：Ctrl+` 与 Ctrl+O 都按不出来")
	}
	before := len(terminalTabs(m, "s1"))
	if result := cfg.OnShortcut(gfx.KeyEvent{Key: gfx.KeyUnknown, Modifiers: gfx.ModifierControl, Text: "`"}); !result.Handled {
		t.Fatalf("cfg.OnShortcut 没处理 Ctrl+`：%+v", result)
	}
	if got := len(terminalTabs(m, "s1")); got != before+1 {
		t.Fatalf("cfg.OnShortcut 没把 Ctrl+` 落到 terminal.new：%d → %d", before, got)
	}
}

// TestShortcutKeysCatalog：向导卡的 `ShortcutKeys` 数据面。
//
// 官方 `terminal.new` 的默认键位是 `{code:'Backquote', modifiers:['control']}`
// （index.ts:57-63）；官方 `ShortcutKeys` 把 `"+"` 当一个独立元素（separator），
// 所以这里必须是三项，不是连成一串。
func TestShortcutKeysCatalog(t *testing.T) {
	m := newShortcutModel(t, "s1")
	got := m.shortcutKeys(terminalNewCommandID)
	want := []string{"Ctrl", "+", "`"}
	if len(got) != len(want) {
		t.Fatalf("terminal.new keys = %v，want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("terminal.new keys[%d] = %q，want %q", i, got[i], want[i])
		}
	}
	if other := m.shortcutKeys("browser.new"); other != nil {
		t.Fatalf("没登记的命令应返回 nil，实际 %v", other)
	}
}

// TestCancelRightTabRenameSwallowsEscape：改名编辑态下 Esc 只取消改名。
//
// 官方那一刻 input 有焦点，Esc 做的是 `cancelled.current = true; blur()`
// （TerminalTitle.tsx:64-66）——**不提交**，也不该继续往下走。漏了这一步，
// 同一个 Esc 会被 dscomposer 当成 reject_once，顺手拒掉一条审批。
func TestCancelRightTabRenameSwallowsEscape(t *testing.T) {
	m := newShortcutModel(t, "s1")
	if m.cancelRightTabRename() {
		t.Fatal("没在改名时 Esc 不该被吃掉（否则审批再也拒不掉）")
	}
	m.rightTabRename.Begin("tab1", "终端")
	if !m.cancelRightTabRename() {
		t.Fatal("改名编辑态下 Esc 应被吃掉")
	}
	if m.rightTabRename.TabID != "" {
		t.Fatalf("Esc 之后仍在编辑态：%q", m.rightTabRename.TabID)
	}
}
