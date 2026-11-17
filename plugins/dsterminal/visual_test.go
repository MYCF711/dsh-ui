package dsterminal

import (
	"reflect"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestOfficialTerminalGeometry(t *testing.T) {
	checks := []struct {
		name string
		got  float32
		want float32
	}{
		{"root padding-top", terminalRootPadTop, 8},
		{"root font", terminalRootFontSize, 13},
		{"screen padding", terminalScreenPad, 8},
		{"status gap", terminalStatusGap, 8},
		{"status padding-y", terminalStatusPadY, 5},
		{"status padding-x", terminalStatusPadX, 12},
		{"error padding-y", terminalErrorPadY, 10},
		{"error padding-x", terminalErrorPadX, 12},
		{"terminal font", terminalFontSize, 13},
		{"tab icon", terminalTabIconSize, 16},
		{"rename width", terminalRenameWidth, 120},
		{"rename min-width", terminalRenameMinWidth, 48},
		{"rename padding-x", terminalRenamePadX, 4},
		{"rename border", terminalRenameBorder, 0.5},
		{"small button height", terminalButtonHeight, 28},
		{"small button padding-x", terminalButtonPadX, 10},
		{"small button gap", terminalButtonGap, 4},
		{"small button font", terminalButtonFont, 12},
		{"small button line", terminalButtonLine, 18},
		{"small button border", terminalButtonBorder, 0.5},
		{"small button icon slot", terminalButtonIcon, 16},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %v，官方值 %v", check.name, check.got, check.want)
		}
	}
}

func TestOfficialTerminalIcons(t *testing.T) {
	icon := terminalTitleIcon()
	want := []gfx.PathCommand{
		gfx.MoveTo(3, 4), gfx.LineTo(7, 8), gfx.LineTo(3, 12),
		gfx.MoveTo(9, 12), gfx.LineTo(13, 12),
	}
	if icon.ViewBox != 16 || icon.Stroke != 1 || !reflect.DeepEqual(icon.Commands, want) {
		t.Fatalf("TerminalIcon 未逐命令复用官方两条 path：%+v", icon)
	}
	plus, ok := terminalPlusIcon()
	if !ok || plus.ViewBox != 16 || plus.Stroke != 1 || len(plus.Commands) != 4 {
		t.Fatalf("新建按钮必须复用官方 plus-outline：ok=%v icon=%+v", ok, plus)
	}
	panels := (&Plugin{}).Panels()
	if len(panels) == 0 || panels[0].IconImage.ViewBox != 16 || panels[0].IconSize != 16 {
		t.Fatalf("终端 tab 必须使用官方 16px TerminalIcon：%+v", panels)
	}
}

func terminalVisualFrame(t *testing.T, state pluginapi.TerminalViewState) (*gfx.Canvas, *pluginapi.Env) {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("terminal-visual"), Canvas: canvas,
		Data: pluginapi.Snapshot{ShowRightbar: true},
		Act: pluginapi.Actions{
			TerminalEnsure:        func(string) {},
			TerminalSnapshot:      func(string) pluginapi.TerminalViewState { return state },
			TerminalRetry:         func(string) {},
			OpenRightTab:          func(string) {},
			CloseRightTab:         func(string) {},
			TerminalWriteOfficial: func(string, string, bool) {},
		},
	}
	p := &Plugin{theme: theme}
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	page.Draw(canvas, gfx.Rect{Width: 360, Height: 400}, func(f ui.C) {
		p.drawPanelBody(f, env)
	})
	return canvas, env
}

func TestTerminalScreenUsesOfficialFontColorAndPadding(t *testing.T) {
	canvas, env := terminalVisualFrame(t, pluginapi.TerminalViewState{
		Phase: "connected", Writable: true, Screen: "$ echo hi\nhello",
		Info: &apiclient.TerminalInfo{Title: "终端 1", State: "running"},
	})
	var first, second gfx.Text
	var foundFirst, foundSecond bool
	for _, text := range canvas.Texts() {
		switch text.Value {
		case "$ echo hi":
			first, foundFirst = text, true
		case "hello":
			second, foundSecond = text, true
		}
	}
	if !foundFirst || !foundSecond {
		t.Fatalf("终端输出没有画出来：%+v", canvas.Texts())
	}
	for _, text := range []gfx.Text{first, second} {
		if !text.Style.Mono || text.Style.Size != terminalFontSize {
			t.Errorf("终端输出必须使用 13px 等宽字体：%+v", text.Style)
		}
		if text.Style.Color != env.Theme.LabelPrimary {
			t.Errorf("screen 文字必须是 label-primary，实际 %+v", text.Style.Color)
		}
		if text.Bounds.X != terminalScreenPad || text.Bounds.Height != terminalFontSize {
			t.Errorf("screen 应有 8px 左内距与 13px 行盒：%+v", text.Bounds)
		}
	}
	if first.Bounds.Y != terminalRootPadTop+terminalScreenPad {
		t.Errorf("screen 首行应从 y=16 开始，实际 %v", first.Bounds.Y)
	}
	if got := second.Bounds.Y - first.Bounds.Y; got != terminalFontSize {
		t.Errorf("官方 lineHeight=1，两行应相距 13px，实际 %v", got)
	}
}

func TestTerminalStatusAndSmallButtonGeometry(t *testing.T) {
	canvas, _ := terminalVisualFrame(t, pluginapi.TerminalViewState{Phase: "disconnected"})
	var statusFound, buttonTextFound bool
	for _, text := range canvas.Texts() {
		switch text.Value {
		case "连接已断开。":
			statusFound = true
			wantY := terminalRootPadTop + terminalStatusPadY + (terminalButtonHeight-text.Bounds.Height)/2
			if text.Style.Size != terminalRootFontSize || text.Bounds.X != terminalStatusPadX || text.Bounds.Y != wantY {
				t.Errorf("状态文字应在 28px 按钮行内居中，且保持 13px/12px 左内距：%+v", text)
			}
		case "重新连接":
			buttonTextFound = true
			if text.Style.Size != terminalButtonFont {
				t.Errorf("small Button 字号应为 12，实际 %v", text.Style.Size)
			}
		}
	}
	if !statusFound || !buttonTextFound {
		t.Fatalf("状态或重连按钮未画出：status=%v button=%v", statusFound, buttonTextFound)
	}
	var retryFound bool
	for _, hit := range canvas.Hits() {
		if hit.Label != "重新连接" {
			continue
		}
		retryFound = true
		if hit.Bounds.Height != terminalButtonHeight {
			t.Errorf("small Button 命中区应为 28px，实际 %v", hit.Bounds.Height)
		}
	}
	if !retryFound {
		t.Fatal("找不到重新连接按钮命中区")
	}
}

func TestTerminalStatusMatchesOfficialBranches(t *testing.T) {
	exitCode := 7
	status, ended, retry, readOnly := terminalStatus(pluginapi.TerminalViewState{
		Phase: "connected", Info: &apiclient.TerminalInfo{State: "running"}, Writable: false,
	})
	if status != "此页面当前只读。" || ended || retry || !readOnly {
		t.Fatalf("connected + running + !writable 分支不符：%q %v %v %v", status, ended, retry, readOnly)
	}
	status, ended, retry, readOnly = terminalStatus(pluginapi.TerminalViewState{
		Phase: "connected", Info: &apiclient.TerminalInfo{State: "exited", ExitCode: &exitCode}, Writable: true,
	})
	if status != "进程已退出（7）" || !ended || retry || readOnly {
		t.Fatalf("exited 分支不符：%q %v %v %v", status, ended, retry, readOnly)
	}
	_, ended, _, _ = terminalStatus(pluginapi.TerminalViewState{Phase: "closed"})
	if !ended {
		t.Fatal("官方 closed 即使 info 缺失也属于 ended，应提供新建终端")
	}
	status, ended, retry, readOnly = terminalStatus(pluginapi.TerminalViewState{Phase: "disconnected"})
	if status != "连接已断开。" || ended || !retry || readOnly {
		t.Fatalf("disconnected 分支不符：%q %v %v %v", status, ended, retry, readOnly)
	}
}
