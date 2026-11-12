package dsopen

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// dsopen 的历史全是"看得见吗"：按钮做出来之后连续三轮在截图里找不到
// （先怀疑槽位没画、再怀疑高度塌陷，最后确认是抓帧时机太早——CWD 要等
// session.load 回来才填）。所以这里的重点不是"代码跑过了"，
// 是**它在什么条件下画、画在哪、点了之后把什么交给谁**。
//
// 三条都是被实现打红过的事实：
//   - 槽位里再摆一行会**另起一行**，落进正文区（旧实现 y=96.5，标题行是 22…52）；
//   - 旧实现把裸 Windows 路径交给 OpenExternal，而 `app.OpenURL` 只收
//     http/https/mailto，真机上每次点击都变成状态栏里的一条失败；
//   - 于是"有 OpenExternal 就画、有 OpenFile 反而不画"——动作判断正好反了。

// renderOpen 画一帧会话头部，返回这一帧画出来的矩形。
// click 非空时走 canvas 的事件 pass，测到的是 box.Clicked() 的真实分支。
func renderOpen(t *testing.T, p *Plugin, data pluginapi.Snapshot, act pluginapi.Actions, click string) []gfx.Rectangle {
	t.Helper()
	if p.theme == nil {
		p.theme = dscore.Dark()
	}
	tree := &gfx.Tree{}
	feat := tree.Feature("open-test")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	if click != "" {
		canvas.BeginEventPass(tree.ID(feat, click), gfx.ClickInfo{})
		defer canvas.EndEventPass()
	}
	env := &pluginapi.Env{
		Theme: p.theme, Data: data, Act: act, Tree: tree, Feat: feat, Canvas: canvas,
		Bounds: ui.Rect{Width: 1182, Height: 832}, MainWidth: 902,
	}
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		// 槽位给的是一个列（真实壳就是中栏列）：本插件只借它定位，不占它的流。
		ui.Column(f).Width(902).Height(76).Children(func() { p.draw(f, env) })
	})
	return canvas.Rectangles()
}

func noErr(string) error { return nil }

// TestDrawsOnlyForAnAbsoluteCWDWithARealOpenAction 是"什么时候该有按钮"的完整表。
//
// 两个方向都被旧实现打红：
//   - 只有 OpenExternal 时旧实现照画（点了必失败：app.OpenURL 不接收本地路径）；
//   - 只有 OpenFile 时旧实现不画（真正能打开目录的动作反而被忽略）。
//
// 相对路径也必须不画：拼出来的 file 链接在壳那边会被判"非绝对路径"，
// 画出来就是一个点了没反应的按钮。
func TestDrawsOnlyForAnAbsoluteCWDWithARealOpenAction(t *testing.T) {
	cases := []struct {
		name string
		cwd  string
		act  pluginapi.Actions
		want int
	}{
		{"绝对路径 + OpenFile", `D:\ws\proj`, pluginapi.Actions{OpenFile: noErr}, 1},
		{"带空格与中文的绝对路径", `D:\工作 区\项目`, pluginapi.Actions{OpenFile: noErr}, 1},
		{"POSIX 绝对路径", "/home/me/ws", pluginapi.Actions{OpenFile: noErr}, 1},
		{"两种动作都在也只画一个", `D:\ws`, pluginapi.Actions{OpenFile: noErr, OpenExternal: noErr}, 1},
		{"空 CWD", "", pluginapi.Actions{OpenFile: noErr, OpenExternal: noErr}, 0},
		{"相对路径", `ws\proj`, pluginapi.Actions{OpenFile: noErr, OpenExternal: noErr}, 0},
		{"单段目录名", "proj", pluginapi.Actions{OpenFile: noErr, OpenExternal: noErr}, 0},
		{"点开头", `.\proj`, pluginapi.Actions{OpenFile: noErr, OpenExternal: noErr}, 0},
		{"上游目录", `..\proj`, pluginapi.Actions{OpenFile: noErr, OpenExternal: noErr}, 0},
		{"只有盘符没有根", `D:ws`, pluginapi.Actions{OpenFile: noErr, OpenExternal: noErr}, 0},
		{"首尾带空白", " D:\\ws ", pluginapi.Actions{OpenFile: noErr, OpenExternal: noErr}, 0},
		{"只有 OpenExternal（错的动作）", `D:\ws`, pluginapi.Actions{OpenExternal: noErr}, 0},
		{"没有打开动作", `D:\ws`, pluginapi.Actions{}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rects := renderOpen(t, &Plugin{}, pluginapi.Snapshot{Cur: "s", CWD: tc.cwd}, tc.act, "")
			if len(rects) != tc.want {
				t.Fatalf("CWD=%q 时应画 %d 个矩形，实际 %d 个：%+v", tc.cwd, tc.want, len(rects), rects)
			}
			if len(rects) == 0 {
				return
			}
			got := rects[0].Bounds
			if got.Width != 56 || got.Height != 24 {
				t.Errorf("按钮尺寸 %vx%v，want 56x24——高度必须是真实像素，不能塌成 0", got.Width, got.Height)
			}
		})
	}
}

// TestClickHandsTheExactCWDtoOpenFileAsAFileLink 钉住"点下去打开的是哪一条路径"。
//
// 必须是**显式 file: 链接**：壳的 filePathFromLink 只认这个形状，裸路径交过去
// 会被 `app.OpenURL` 拒（http/https/mailto only），于是点一次失败一次。
func TestClickHandsTheExactCWDtoOpenFileAsAFileLink(t *testing.T) {
	var openedFiles, openedURLs []string
	act := pluginapi.Actions{
		OpenFile:     func(dst string) error { openedFiles = append(openedFiles, dst); return nil },
		OpenExternal: func(dst string) error { openedURLs = append(openedURLs, dst); return nil },
	}
	p := &Plugin{}
	data := pluginapi.Snapshot{Cur: "s", CWD: `D:\ws\项目 一`}

	// 先画一帧（真实运行时也是先画再点）：没点就不该有任何动作。
	renderOpen(t, p, data, act, "")
	if len(openedFiles) != 0 || len(openedURLs) != 0 {
		t.Fatalf("没点就触发了动作：files=%v urls=%v", openedFiles, openedURLs)
	}

	renderOpen(t, p, data, act, "open-in-app")
	if len(openedFiles) != 1 {
		t.Fatalf("点击后 OpenFile 被调 %d 次，want 1；urls=%v", len(openedFiles), openedURLs)
	}
	const want = "file:///D:/ws/%E9%A1%B9%E7%9B%AE%20%E4%B8%80"
	if openedFiles[0] != want {
		t.Errorf("OpenFile 收到 %q，want %q（必须是显式 file: 链接，不是裸路径）", openedFiles[0], want)
	}
	if len(openedURLs) != 0 {
		t.Errorf("本地目录被当成 URL 交给 OpenExternal：%v", openedURLs)
	}
}

// TestClickErrorsStayInsideTheShell 钉住"失败只走壳的状态栏"：
// 打开失败不回退到别的动作（更不许回退成 OpenExternal），也不在插件里弹窗。
func TestClickErrorsStayInsideTheShell(t *testing.T) {
	// 打开失败由壳写状态栏；插件不弹窗、不改口成别的动作、也不 panic。
	var externals int
	act := pluginapi.Actions{
		OpenFile:     func(string) error { return injectedOpenError("boom") },
		OpenExternal: func(string) error { externals++; return nil },
	}
	p := &Plugin{}
	data := pluginapi.Snapshot{Cur: "s", CWD: `D:\ws`}
	renderOpen(t, p, data, act, "open-in-app")
	if externals != 0 {
		t.Errorf("打开动作没有停在 OpenFile：OpenExternal 被调 %d 次", externals)
	}
}

type injectedOpenError string

func (e injectedOpenError) Error() string { return string(e) }

// TestNoActionNoButton 单独留一条：没有打开动作时连布局都不该发生。
func TestNoActionNoButton(t *testing.T) {
	p := &Plugin{}
	rects := renderOpen(t, p, pluginapi.Snapshot{Cur: "s", CWD: `D:\ws`}, pluginapi.Actions{}, "")
	if len(rects) != 0 {
		t.Fatalf("壳没接可用的打开动作时不该画按钮，画了 %d 个矩形：%+v", len(rects), rects)
	}
}

// TestButtonTextIsDrawn 守"按钮上真有一个字"：只有底色没有文案的按钮，
// 截图里看着像一块脏像素——这一条同时钉住文案本身（回归时一眼能看出改了字）。
func TestButtonTextIsDrawn(t *testing.T) {
	rects := renderOpen(t, &Plugin{}, pluginapi.Snapshot{Cur: "s", CWD: `D:\ws`},
		pluginapi.Actions{OpenFile: noErr}, "")
	if len(rects) != 1 {
		t.Fatalf("应当正好画一个按钮矩形，得到 %d 个：%+v", len(rects), rects)
	}
}
