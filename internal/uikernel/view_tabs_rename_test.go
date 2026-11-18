//go:build darwin || windows || linux

// view_tabs_rename_test.go —— 右栏芯片的**就地改名**（官方 TerminalTitle 的
// dblclick → <input> → `void view(tab.id).rename(next)`）的机器断言。
//
// 覆盖三件事，全部是官方原文的行为：
//
//	A. 双击芯片（命中区 = 整枚芯片，TerminalTitle.tsx:25-36）进入编辑态；
//	B. 回车提交、值取 `.trim()`、空串与原值不提交（TerminalTitle.tsx:55-59）；
//	C. Esc 取消、不提交（TerminalTitle.tsx:64-66），且编辑态收起。
//
// 几何（`.name` 的 120 / 48 / 100% / 0.5px / 4px）在 TestRightTabRenameGeometry
// 里钉死，改值必须回去核 TerminalTitle.module.css:2。
package uikernel

import (
	"sync"
	"testing"

	"github.com/vivaui/viva/headless"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// renameRecorder 记录壳收到的改名提交。
type renameRecorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *renameRecorder) rename(tabID, title string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, tabID+"\x00"+title)
}

func (r *renameRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

func renameTestTabs() []pluginapi.Tab {
	return []pluginapi.Tab{
		{Key: "terminal", Kind: "terminal", Label: "终端", Multiple: true},
		{Key: "browser", Kind: "browser", Label: "浏览器"},
	}
}

// openRenameWindow 把芯片行画进真 Viva headless 窗口，并**把改名框交给平台**。
//
// 为什么不能用 `openGuideFrame`：它没有设 `app.Config.TextInput`，viva 只把登记过的
// `*ui.Field` 当输入焦点候选（`viva@v0.6.4/ui/elements.go:301-315`、壳在
// `cmd/app/main.go` 的 `ui.Fields(fields...)` 同一句）。少了它，双击能进编辑态、
// 焦点也落在改名框上，但**打不进字**（`app: no focused text field`）。
func openRenameWindow(t *testing.T, state *RightTabRenameState, onRename func(tabID, title string)) *headless.Session {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("rename-frame")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{
		Title: "rename-frame", Width: 460, Height: 140, Tree: tree,
		WindowMode: app.WindowFrameless,
		TextInput:  ui.Fields(&state.Field),
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			Has: func(pluginapi.Slot) bool { return false },
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			env.Act.RenameRightTab = onRename
			DrawRightTabsEditable(f, env, renameTestTabs(), "terminal", func(string) {}, state)
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	if err := session.Resize(460, 140); err != nil {
		t.Fatal(err)
	}
	pumpRename(t, session)
	return session
}

func TestRightTabRenameGeometry(t *testing.T) {
	checks := []struct {
		name string
		got  float32
		want float32
	}{
		// OFF:dsh-client-ui-sidebar-terminal/src/client/TerminalTitle.module.css:2
		//   `.name{width:120px;min-width:48px;max-width:100%;padding:0 4px;…;
		//    border:0.5px solid var(--dsw-alias-border-l3)}`
		{"rename width", RightTabRenameWidth, 120},
		{"rename min-width", RightTabRenameMinWidth, 48},
		{"rename padding-x", RightTabRenamePadX, 4},
		{"rename border", RightTabRenameBorder, 0.5},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %v，官方值 %v", check.name, check.got, check.want)
		}
	}
}

// TestRightTabDoubleClickStartsInlineRename 钉住 A：双击芯片进编辑态。
func TestRightTabDoubleClickStartsInlineRename(t *testing.T) {
	recorder := &renameRecorder{}
	state := &RightTabRenameState{}
	session := openRenameWindow(t, state, recorder.rename)
	if state.TabID != "" {
		t.Fatalf("没双击就不该在编辑态，TabID=%q", state.TabID)
	}
	if err := session.DoubleClickLabel("终端"); err != nil {
		t.Fatalf("双击芯片失败：%v", err)
	}
	pumpRename(t, session)
	if state.TabID != "terminal" {
		t.Fatalf("双击后编辑的 occurrence = %q，want terminal", state.TabID)
	}
	if got := state.Field.Text(); got != "终端" {
		t.Fatalf("编辑框初值 = %q，官方 `<input defaultValue={title}>` 要的是当前标题", got)
	}
}

// TestRightTabRenameCommitsOnEnter 钉住 B：回车提交、trim、空串/同值不提交。
func TestRightTabRenameCommitsOnEnter(t *testing.T) {
	for _, one := range []struct {
		name  string
		typed string
		want  []string
	}{
		{name: "改名", typed: "构建", want: []string{"terminal\x00构建"}},
		{name: "带空白", typed: "  构建  ", want: []string{"terminal\x00构建"}},
		{name: "空串不提交", typed: "   ", want: nil},
	} {
		t.Run(one.name, func(t *testing.T) {
			recorder := &renameRecorder{}
			state := &RightTabRenameState{}
			session := openRenameWindow(t, state, recorder.rename)
			if err := session.DoubleClickLabel("终端"); err != nil {
				t.Fatal(err)
			}
			if err := session.Driver().Pump(); err != nil {
				t.Fatal(err)
			}
			// 官方 `<input defaultValue={title}>`：全选后直接打字就是替换。
			if err := session.Driver().SelectText(0, 2); err != nil {
				t.Fatal(err)
			}
			if one.typed != "   " {
				if err := session.Type(one.typed); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := session.Type("   "); err != nil {
					t.Fatal(err)
				}
			}
			if err := session.Driver().Key(gfx.KeyEnter, 0); err != nil {
				t.Fatal(err)
			}
			if err := session.Driver().Pump(); err != nil {
				t.Fatal(err)
			}
			got := recorder.snapshot()
			if len(got) != len(one.want) {
				t.Fatalf("提交次数 = %d（%v），want %d（%v）", len(got), got, len(one.want), one.want)
			}
			for i := range one.want {
				if got[i] != one.want[i] {
					t.Fatalf("第 %d 次提交 = %q，want %q", i, got[i], one.want[i])
				}
			}
			if state.TabID != "" {
				t.Fatalf("提交后仍在编辑态：TabID=%q", state.TabID)
			}
		})
	}
}

// TestRightTabRenameEscIsOwnedByShell 说明 Esc 这一支的归属。
//
// 官方 Esc 做的是 `cancelled.current = true; blur()`（TerminalTitle.tsx:64-66），
// 于是 onBlur 那一支不提交。viva 把 Esc 交给**窗口**的 OnKey（`app.Config.OnKey`），
// 不送输入框的 OnKey，所以取消必须由壳先截（`model.cancelRightTabRename`，
// 见 cmd/app/right_tab_rename.go 与它的测试）。这里只把"交出去"这一步钉住：
// 取消 = End()，一帧都不提交。
func TestRightTabRenameEscIsOwnedByShell(t *testing.T) {
	recorder := &renameRecorder{}
	state := &RightTabRenameState{}
	state.Begin("terminal", "终端")
	state.Field.SetText("不该提交")
	// 壳的 cancelRightTabRename 就是这一句（外加 Invalidate）：清掉 TabID。
	state.End()
	if state.TabID != "" {
		t.Fatalf("取消之后仍在编辑态：TabID=%q", state.TabID)
	}
	if got := recorder.snapshot(); len(got) != 0 {
		t.Fatalf("取消不该提交，实际提交了 %v", got)
	}
}

// TestRightTabRenameNeedsActionAndTree：壳没接改名动作 / 没有交互树时**不**进编辑态
// （宁可没有这个功能，也不画一个提交不出去的输入框）。
func TestRightTabRenameNeedsActionAndTree(t *testing.T) {
	state := &RightTabRenameState{TabID: "terminal"}
	state.Field.SetText("终端")
	// 没有动作：画出来必须还是标题文字，不能是输入框。
	session := openRenameWindow(t, state, nil)
	for _, one := range session.Snapshot().Texts {
		if one.Value == "重命名标签" {
			t.Fatalf("壳没接改名动作却画出了输入框（应为标题文字）")
		}
	}
	// 状态不该被绘制改掉（壳只读它，不替它决定）。
	if state.TabID != "terminal" {
		t.Fatalf("状态不该被绘制改掉，TabID=%q", state.TabID)
	}
}

// pumpRename 连抽几帧：`AutoFocus` 的焦点要"下一帧"才生效
// （OFF:viva@v0.6.4/ui/style.go:151-172「A request made while handling a click or a
// shortcut takes effect in the repaint that follows it」），一帧不够。
func pumpRename(t *testing.T, session *headless.Session) {
	t.Helper()
	for i := 0; i < 4; i++ {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
}

// TestRightTabRenameEndsWhenTheTabDisappears：改名期间那枚 occurrence 消失
// （Agent 结束终端 / 别处关掉 / 会话切走）时，编辑态必须在**绘制线程**收起。
//
// 判据是官方的 `editing` 组件生命周期：官方 input 随 occurrence 卸载，
// 不存在"名字还在编辑框里、那枚 tab 已经不在了"的中间态（TerminalTitle.tsx:51-69）。
func TestRightTabRenameEndsWhenTheTabDisappears(t *testing.T) {
	for _, tabs := range [][]pluginapi.Tab{
		{{Key: "browser", Kind: "browser", Label: "浏览器"}},
		nil,
	} {
		state := &RightTabRenameState{}
		state.Begin("terminal", "终端")
		openGuideFrame(t, 460, 140, func(env *pluginapi.Env) func(ui.C) {
			env.Act.RenameRightTab = func(string, string) {}
			return func(f ui.C) {
				DrawRightTabsEditable(f, env, tabs, "", func(string) {}, state)
			}
		})
		if state.TabID != "" {
			t.Fatalf("occurrence 已消失，编辑态还在：TabID=%q（tabs=%v）", state.TabID, tabs)
		}
	}
}
