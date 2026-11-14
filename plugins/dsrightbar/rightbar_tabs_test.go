//go:build darwin || windows || linux

package dsrightbar

// rightbar_tabs_test.go —— 右栏 tab 宿主的产品路径回归。
//
// 这一条走的是**和产品一样的链**：内核注册的 tab 类型 → 名录 → 芯片行 →
// 按 ActiveRightTab 派发那一块体。它盯的是三件容易一起坏的事：
//
//	① 芯片行画出来了（文案落到文字层）
//	② 面板体只画**选中那一个** tab（两个 tab 同屏画进同一块矩形是典型症状）
//	③ 点另一枚芯片会走到 Actions.SelectRightTab（不是只换高亮、不换内容）

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dstopbar"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// terminalTabStub 冒充 dsterminal 注册的那一枚右栏 tab 类型。
//
// 只挂一个可辨认的画，不复刻终端本体：这一条验的是**宿主派发**，
// 终端面板自己的内容由 plugins/dsterminal 的测试管。
type terminalTabStub struct{ drawn *bool }

func (*terminalTabStub) ID() string         { return "stub-terminal" }
func (*terminalTabStub) Provides() []string { return nil }
func (*terminalTabStub) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}
func (*terminalTabStub) Register(pluginapi.Context) error { return nil }
func (s *terminalTabStub) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{
		ID: "terminal-tab", Slot: pluginapi.SlotRightPaneTab, Key: "terminal",
		Label: "终端", Order: 20,
		Draw: func(f ui.C, env *pluginapi.Env) {
			*s.drawn = true
			ui.Text(f, "终端面板体").FontSize(12).Color(env.Theme.LabelSecondary)
		},
	}}
}

func TestRightbarTabHostDrawsStripAndDispatchesActiveOnly(t *testing.T) {
	terminalDrawn := false
	kernel := uikernel.New("rightbar-tabs")
	// ds-topbar 是 view store 的提供方（ds-rightbar 的 Requires 里有它），
	// 与产品装配一致。
	if err := kernel.Load(
		&dscore.Plugin{Dark: true},
		&dstopbar.Plugin{},
		&Plugin{},
		&terminalTabStub{drawn: &terminalDrawn},
	); err != nil {
		t.Fatal(err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("rightbar-tabs")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})

	selected := ""
	tabs := uikernel.RightTabs(kernel)
	if len(tabs) != 2 {
		t.Fatalf("右栏名录应当有 timeline + terminal 两项，实际 %+v", tabs)
	}

	width, height := 720, 620
	config := app.Config{Title: "rightbar-tabs", Width: width, Height: height, Tree: tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: theme,
			Data: pluginapi.Snapshot{
				Cur: "tabs-session", ShowRightbar: true,
				Sessions: []apiclient.Session{{ID: "tabs-session", Title: "右栏会话"}},
				Msgs: []apiclient.Message{
					{Role: "tool", Tool: "bash", Summary: "echo hi"},
				},
			},
			Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			MainWidth: float32(width) - 280,
			Geometry: pluginapi.Geometry{
				Width: float32(width), Height: float32(height), SidebarWidth: 280,
				RightWidth: float32(width) - 280, MainWidth: 280, BottomHeight: 0,
			},
			RightTabs:      tabs,
			ActiveRightTab: RightTabTimeline,
			Has:            func(pluginapi.Slot) bool { return false },
			Act:            pluginapi.Actions{SelectRightTab: func(key string) { selected = key }},
		}
		env.DrawKeyed = func(target ui.C, slot pluginapi.Slot, key string) {
			kernel.DrawSlotKeyed(target, slot, key, env)
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) { kernel.DrawSlot(f, pluginapi.SlotRightbar, env) })
	}

	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	session.Resize(width, height)

	// ① 两枚芯片都在。
	for _, chip := range []string{"轨迹", "终端"} {
		if !session.HasText(chip) {
			t.Fatalf("右栏 tab 芯片 %q 没画出来：%v", chip, session.Texts())
		}
	}
	// ② 选中的是 timeline，所以派发的是它；terminal 那一块**不该**画。
	if terminalDrawn {
		t.Fatal("没选中的 tab 也画了 = 两个面板体会叠在同一块矩形里")
	}
	if !session.HasText("bash") {
		t.Fatalf("选中的 timeline 面板体没画出来：%v", session.Texts())
	}
	// ③ 点另一枚芯片走选择动作。
	if err := session.ClickLabel("终端"); err != nil {
		t.Fatalf("点「终端」芯片失败：%v", err)
	}
	if selected != "terminal" {
		t.Fatalf("点芯片没有走到 Actions.SelectRightTab，实际 %q", selected)
	}
}
