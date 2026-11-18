//go:build darwin || windows || linux

// panel_flow_badge_test.go —— 两件只有真渲染才量得到的事：
//
//	A. `.status{flex-wrap:wrap}`：窄面板里状态文字与按钮要**折行**，不是压成一行截断。
//	   （旧账写「缺 Viva 容器 wrap」，v0.6.4 的 `ui.Flow()` 就是 CSS flex-wrap 的等价物，
//	   见 viva@v0.6.4/ui/frame.go:533-544 与 declarative-layout.md:36。）
//	B. 芯片徽标的数据源：壳给的 `TerminalSnapshot` 里 origin=agent 时，
//	   `RightTabBadge` 必须真的给出官方那枚 `Tag`——这是 #54 条①的最后一段。
package dsterminal

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// drawPanelBodyWith 把终端面板本体画进真 headless 窗口，返回会话与插件。
func drawPanelBodyWith(t *testing.T, width, height int, state pluginapi.TerminalViewState) *headless.Session {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("terminal-panel")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	p := &Plugin{theme: theme}
	config := app.Config{
		Title: "terminal-panel", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width,
			Data:           pluginapi.Snapshot{ShowRightbar: true},
			ActiveRightTab: RightTabTerminal,
			RightTabID:     "tab1",
			Act: pluginapi.Actions{
				TerminalEnsure:         func(string) {},
				TerminalSnapshot:       func(string) pluginapi.TerminalViewState { return state },
				TerminalRetry:          func(string) {},
				TerminalRenameOfficial: func(string, string) error { return nil },
				OpenRightTab:           func(string) {},
				Notify:                 func(string) {},
			},
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) { p.drawPanelBody(f, env) })
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	if err := session.Resize(width, height); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	return session
}

// TestTerminalStatusWrapsLikeOfficialFlow 钉住 A。
func TestTerminalStatusWrapsLikeOfficialFlow(t *testing.T) {
	// 官方 `.status` 的两个子元素：状态文字 + 一枚 sm Button。
	// 只读档（phase=connected、state=running、writable=false）。
	state := pluginapi.TerminalViewState{
		Phase:    "connected",
		Writable: false,
		Info:     &apiclient.TerminalInfo{State: "running", Title: "终端"},
	}
	narrow := drawPanelBodyWith(t, 180, 220, state)
	y := map[string]float32{}
	for _, text := range narrow.Snapshot().Texts {
		switch text.Value {
		case "此页面当前只读。", "接管输入":
			y[text.Value] = text.Bounds.Y
		}
	}
	statusY, okStatus := y["此页面当前只读。"]
	buttonY, okButton := y["接管输入"]
	if !okStatus || !okButton {
		t.Fatalf("状态条没画全：%v（texts=%v）", y, narrow.Texts())
	}
	// 一行的高度是 18（13px × 1.4 / 按钮 line-height 18），折行的行距至少一整行。
	if buttonY-statusY < terminalButtonLine {
		t.Fatalf("180px 宽里状态文字(y=%v)与按钮(y=%v)差 %v，不到一行 —— `.status{flex-wrap:wrap}` 没生效",
			statusY, buttonY, buttonY-statusY)
	}
	if buttonY < statusY {
		t.Fatalf("折行顺序反了：按钮 y=%v 在状态文字 y=%v 之上", buttonY, statusY)
	}

	// 宽面板里同一对必须在同一行（否则是"永远折行"，同样是错的）。
	wide := drawPanelBodyWith(t, 720, 220, state)
	y = map[string]float32{}
	for _, text := range wide.Snapshot().Texts {
		switch text.Value {
		case "此页面当前只读。", "接管输入":
			y[text.Value] = text.Bounds.Y
		}
	}
	// 同一行里两段文字的基线差只是 1px 级别（按钮的文字在 28 高的盒里居中），
	// 远小于一行；用"不到一行"作判据，不拿浮点相等当真理。
	if delta := y["接管输入"] - y["此页面当前只读。"]; delta < 0 || delta >= terminalButtonLine {
		t.Fatalf("720px 宽里状态文字(y=%v)与按钮(y=%v)差 %v —— 这一档不该折行",
			y["此页面当前只读。"], y["接管输入"], delta)
	}
}

// TestRightTabBadgeReadsShellSnapshot 钉住 B：壳快照 → 徽标。
func TestRightTabBadgeReadsShellSnapshot(t *testing.T) {
	cases := []struct {
		name    string
		info    *apiclient.TerminalInfo
		want    bool
		text    string
		tone    pluginapi.TabBadgeTone
		hintHas string
	}{
		{
			name: "Agent 控制 + 只读沙箱（两段提示拼起来）",
			info: &apiclient.TerminalInfo{Origin: "agent", Control: "agent", SandboxMode: "read-only"},
			want: true, text: zhBadgeAgent, tone: pluginapi.TabBadgeInfo, hintHas: zhBadgeReadOnlyHint,
		},
		{
			name: "Agent 已交控制权、工作区沙箱",
			info: &apiclient.TerminalInfo{Origin: "agent", Control: "user", SandboxMode: "workspace-write"},
			want: true, text: zhBadgeSandbox, tone: pluginapi.TabBadgeNeutral, hintHas: zhBadgeWriteHint,
		},
		{
			name: "用户终端（origin=user）不画徽标",
			info: &apiclient.TerminalInfo{Origin: "user", Control: "user"},
			want: false,
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			theme := dscore.Dark()
			tree := &gfx.Tree{}
			feat := tree.Feature("badge")
			pageTheme := dscore.VivaTheme(theme)
			view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
			p := &Plugin{theme: theme}
			state := pluginapi.TerminalViewState{
				Phase: "connected", Writable: true, Info: one.info, Screen: "agent$ ",
			}
			config := app.Config{Title: "badge", Width: 360, Height: 240, Tree: tree, WindowMode: app.WindowFrameless}
			config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
				bounds := gfx.Rect{Width: size.Width, Height: size.Height}
				env := &pluginapi.Env{
					Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
					Data: pluginapi.Snapshot{ShowRightbar: true}, ActiveRightTab: RightTabTerminal,
					RightTabID: "tab1",
					Act: pluginapi.Actions{
						TerminalEnsure:         func(string) {},
						TerminalSnapshot:       func(string) pluginapi.TerminalViewState { return state },
						TerminalRetry:          func(string) {},
						TerminalRenameOfficial: func(string, string) error { return nil },
						Notify:                 func(string) {},
					},
				}
				canvas.Fill(bounds, theme.BgBase)
				// 画一帧面板本体：`capture(env)` 就是把壳的 Actions 灌进插件的那一步。
				view.Draw(canvas, bounds, func(f ui.C) { p.drawPanelBody(f, env) })
			}
			session, err := headless.Open(config, nil, headless.Options{Scale: 1})
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			if err := session.Resize(360, 240); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 4; i++ {
				if err := session.Driver().Pump(); err != nil {
					t.Fatal(err)
				}
			}
			badge, ok := p.RightTabBadge("s1", "tab1")
			if ok != one.want {
				t.Fatalf("有徽标 = %v，want %v（badge=%+v）", ok, one.want, badge)
			}
			if !ok {
				return
			}
			if badge.Text != one.text || badge.Tone != one.tone {
				t.Fatalf("徽标 = %+v，want {%q %v}", badge, one.text, one.tone)
			}
			if one.hintHas != "" && !contains(badge.Hint, one.hintHas) {
				t.Fatalf("提示 = %q，缺 %q", badge.Hint, one.hintHas)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
