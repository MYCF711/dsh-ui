//go:build darwin || windows || linux

package main

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsopen"
	"github.com/MYCF711/dsh-ui/plugins/dsrightbar"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar"
	"github.com/MYCF711/dsh-ui/plugins/dstopbar"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// OPEN-IN-APP 的第 1 条验收：有效绝对 Snapshot.CWD + 真实打开动作时，
// 按钮在 1182×832 与 1920×1032 都**画在会话头部标题行里**、非 0 高、
// 不挤压页签；CWD 缺失/相对/没有动作时整颗消失。
//
// 这条用 cmd/app 的真实槽位树（ds-core + ds-shell + ds-rightbar + ds-topbar
// + ds-open）跑，不用"模拟一个头部行"的裸 canvas：旧实现正是死在真实列里——
// 它把按钮变成 header 后面的另一行（y=96.5，标题行 22…52），几何断言全红。

type openInAppRig struct {
	session *headless.Session
	data    *pluginapi.Snapshot
	actions *pluginapi.Actions
	// repaint 强制重画一帧：改 Snapshot/Actions 之后必须调它。
	repaint func()
	// close 关窗口（每个尺寸一个）。
	close func()
}

// newOpenInAppRig 起一个真实装配的无头窗口，只装会话头部这条链需要的插件。
//
// 正文/输入框刻意不装：本项要验的是头部几何与打开动作，装得越多，
// 断言失败时越难判断是谁动了标题行。
func newOpenInAppRig(t *testing.T, m *model, width, height int) openInAppRig {
	t.Helper()
	data := &pluginapi.Snapshot{
		Cur:      m.cur,
		CWD:      m.cwd,
		Sessions: []apiclient.Session{{ID: m.cur, Title: "会话一", Cwd: m.cwd}},
		Msgs:     []apiclient.Message{{Role: "user", Text: "hello", Seq: 1}},
		Total:    1,
	}
	actions := &pluginapi.Actions{
		OpenFile:       m.openFile,
		OpenExternal:   m.openExternal,
		ToggleRightbar: func() {},
	}
	kernel := uikernel.New("open-in-app-header")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dsshell.Plugin{}, &dssidebar.Plugin{},
		&dsrightbar.Plugin{}, &dstopbar.Plugin{}, &dsopen.Plugin{}); err != nil {
		t.Fatal(err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("open-in-app-header")
	var (
		view        ui.Page
		invalidator app.Invalidator
	)
	config := app.Config{
		Title: "open-in-app-header", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: *data, Act: *actions, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: bounds.Width - 280,
			Geometry: pluginapi.Geometry{
				Width: bounds.Width, Height: bounds.Height, SidebarWidth: 280,
				MainWidth: bounds.Width - 280,
			},
			Has: func(slot pluginapi.Slot) bool { return kernel.Has(slot) },
		}
		view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f ui.C) {
			kernel.DrawSlot(f, pluginapi.SlotShellRoot, env)
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	rig := openInAppRig{
		session: session, data: data, actions: actions,
		repaint: func() {
			t.Helper()
			if err := session.Resize(width+1, height); err != nil {
				t.Fatal(err)
			}
			if err := session.Resize(width, height); err != nil {
				t.Fatal(err)
			}
		},
		close: func() { session.Close() },
	}
	rig.repaint()
	return rig
}

// titleRowBand 从真实页签的位置反推标题行：页签顶 = 标题行底 + tabsMarginTop(10)，
// 标题行高 30（官方 titleRow min-height）。
func titleRowBand(t *testing.T, session *headless.Session, width, height int) (top, bottom float32) {
	t.Helper()
	tabs, ok := session.Hit("对话")
	if !ok {
		t.Fatalf("%dx%d 找不到「对话」页签：texts=%q", width, height, session.Texts())
	}
	const (
		tabsMarginTop  = 10
		titleRowHeight = 30
	)
	bottom = tabs.Bounds.Y - tabsMarginTop
	return bottom - titleRowHeight, bottom
}

func TestOpenInAppButtonLivesInHeaderAtBothSizes(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{name: "1182x832", width: 1182, height: 832}, {name: "1920x1032", width: 1920, height: 1032}} {
		t.Run(size.name, func(t *testing.T) {
			cwd := t.TempDir()
			m := &model{cur: "s-open", cwd: cwd}
			rig := newOpenInAppRig(t, m, size.width, size.height)
			defer rig.close()

			hit, ok := rig.session.Hit("在工作目录里打开")
			if !ok {
				t.Fatalf("%dx%d 按钮不可见：texts=%q", size.width, size.height, rig.session.Texts())
			}
			if hit.Bounds.Width != 56 || hit.Bounds.Height != 24 {
				t.Fatalf("%dx%d 按钮 %vx%v，want 56x24（0 高等于看不见）",
					size.width, size.height, hit.Bounds.Width, hit.Bounds.Height)
			}
			rowTop, rowBottom := titleRowBand(t, rig.session, size.width, size.height)
			if hit.Bounds.Y < rowTop || hit.Bounds.Y+hit.Bounds.Height > rowBottom {
				t.Fatalf("%dx%d 按钮 y=%v..%v 不在标题行 %v..%v 里（旧实现落在正文区 y=96.5）",
					size.width, size.height, hit.Bounds.Y, hit.Bounds.Y+hit.Bounds.Height, rowTop, rowBottom)
			}
			if center, want := hit.Bounds.Y+hit.Bounds.Height/2, rowTop+15; center != want {
				t.Errorf("%dx%d 按钮垂直中心 %v，want 标题行中心 %v", size.width, size.height, center, want)
			}
			if !rig.session.HasText("打开") {
				t.Errorf("%dx%d 按钮只有底色、没有文案：%q", size.width, size.height, rig.session.Texts())
			}

			// 与右上角 28px 展开钮的关系：官方 headerUtilities gap 8，不许盖住它。
			corner, ok := rig.session.Hit("打开右侧边栏")
			if !ok {
				t.Fatalf("%dx%d 找不到 28px corner：texts=%q", size.width, size.height, rig.session.Texts())
			}
			if gap := corner.Bounds.X - (hit.Bounds.X + hit.Bounds.Width); gap != 8 {
				t.Errorf("%dx%d 按钮右缘到 corner 左缘 = %v，want 8", size.width, size.height, gap)
			}
			t.Logf("OPEN_IN_APP_BUTTON size=%dx%d button=%+v titleRow=%v..%v corner=%+v",
				size.width, size.height, hit.Bounds, rowTop, rowBottom, corner.Bounds)
		})
	}
}

// TestOpenInAppButtonDisappearsWithoutARealTarget 覆盖三种"不该画"：
// 缺 CWD、相对 CWD、壳没接 OpenFile。三者都必须**整颗消失**（不是画成灰的），
// 且消失前后页签/标题行的几何一位都不动。
func TestOpenInAppButtonDisappearsWithoutARealTarget(t *testing.T) {
	cwd := t.TempDir()
	m := &model{cur: "s-open", cwd: cwd}
	rig := newOpenInAppRig(t, m, 1182, 832)
	defer rig.close()

	baseline, ok := rig.session.Hit("对话")
	if !ok {
		t.Fatalf("找不到页签：%q", rig.session.Texts())
	}
	if _, ok := rig.session.Hit("在工作目录里打开"); !ok {
		t.Fatalf("前置条件不成立：有绝对 CWD 与 OpenFile 时没有按钮")
	}

	cases := []struct {
		name string
		drop func()
		undo func()
	}{
		{name: "缺 CWD", drop: func() { rig.data.CWD = "" }, undo: func() { rig.data.CWD = cwd }},
		{name: "相对 CWD", drop: func() { rig.data.CWD = `ws\proj` }, undo: func() { rig.data.CWD = cwd }},
		{name: "没有 OpenFile", drop: func() { rig.actions.OpenFile = nil }, undo: func() { rig.actions.OpenFile = m.openFile }},
	}
	for _, tc := range cases {
		tc.drop()
		rig.repaint()
		if hit, ok := rig.session.Hit("在工作目录里打开"); ok {
			t.Fatalf("%s 时仍然画着按钮：%+v；texts=%q", tc.name, hit.Bounds, rig.session.Texts())
		}
		tabs, ok := rig.session.Hit("对话")
		if !ok || tabs.Bounds != baseline.Bounds {
			t.Fatalf("%s 时页签几何变了：%+v → %+v", tc.name, baseline.Bounds, tabs.Bounds)
		}
		tc.undo()
		rig.repaint()
		if _, ok := rig.session.Hit("在工作目录里打开"); !ok {
			t.Fatalf("%s 恢复后按钮没回来", tc.name)
		}
	}
}
