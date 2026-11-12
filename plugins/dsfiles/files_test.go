package dsfiles

import (
	"net/url"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 这个包在 2026-11-04 之前**一个测试都没有**。代价是：列表已经会进子目录了，
// 点文件却还拿根目录去拼路径，于是子目录里的文件一律"读不出来"——没人发现。
// 所以这里的测试不只是补覆盖率，是补一条**会失败的**回归。

// renderFiles 用真实 flex 布局画一帧文件列表（不启动窗口）。
// click 非空时走 canvas 的事件 pass，测到的是 row.Clicked() 的真实分支。
func renderFiles(t *testing.T, p *Plugin, data pluginapi.Snapshot, act pluginapi.Actions, click string) {
	t.Helper()
	if p.theme == nil {
		p.theme = dscore.Dark()
	}
	if p.prims == nil {
		p.prims = &dscore.Primitives{Theme: p.theme}
	}
	tree := &gfx.Tree{}
	feat := tree.Feature("files-test")
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
		Bounds:    ui.Rect{Width: 320, Height: 900},
		MainWidth: 320,
		Geometry:  pluginapi.Geometry{Width: 320, Height: 900, SidebarWidth: 280},
	}
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		p.drawList(f, env)
	})
}

func TestDirPath(t *testing.T) {
	// 拼路径只有这一处。两边（问列表 / 读文件）必须同一个算法。
	cases := []struct{ cwd, cur, want string }{
		{"", "", ""},
		{"", "plugins", ""},                   // 没 CWD 就不拼
		{`D:\ws`, "", `D:\ws`},                // 根目录
		{`D:\ws`, "plugins", `D:\ws/plugins`}, // 进一层
		{`D:\ws`, "plugins/dsopen", `D:\ws/plugins/dsopen`},
		{`D:\ws/`, "plugins", `D:\ws/plugins`},  // CWD 带尾斜杠不产生 "//"
		{`D:\ws`, "/plugins/", `D:\ws/plugins`}, // curDir 带前导/尾随斜杠
	}
	for _, tc := range cases {
		if got := dirPath(tc.cwd, tc.cur); got != tc.want {
			t.Errorf("dirPath(%q, %q) = %q，want %q", tc.cwd, tc.cur, got, tc.want)
		}
	}
}

func TestFileURL(t *testing.T) {
	got, err := fileURL(`D:\ws\plugins\open.go`)
	if err != nil {
		t.Fatalf("合法路径报了错：%v", err)
	}
	if got != "file:///D:/ws/plugins/open.go" {
		t.Errorf("fileURL = %q，want %q", got, "file:///D:/ws/plugins/open.go")
	}

	// 空格要转义，而且**解析回来必须还是原路径**——
	// 壳那边 `filePathFromLink` 就是 url.Parse 之后取 u.Path 的，
	// 转义错了它会说"不是可打开的显式 file 链接"，而用户只会看到"打开失败"。
	spaced, err := fileURL(`D:\ws\a b.txt`)
	if err != nil {
		t.Fatalf("含空格的路径报了错：%v", err)
	}
	u, err := url.Parse(spaced)
	if err != nil {
		t.Fatalf("造出来的链接自己解析不了：%v", err)
	}
	if u.Scheme != "file" || u.Host != "" {
		t.Errorf("scheme/host 不对：scheme=%q host=%q（壳要求 file + 空 host）", u.Scheme, u.Host)
	}
	if u.Path != "/D:/ws/a b.txt" {
		t.Errorf("解析回来 path = %q，want %q（空格没还原）", u.Path, "/D:/ws/a b.txt")
	}

	// 反向：不该造出链接的输入必须报错，而不是造一个"看起来像"的东西。
	for _, bad := range []string{"", "   ", " D:\\ws\\a.go", "相对/路径.txt"} {
		if dst, err := fileURL(bad); err == nil {
			t.Errorf("fileURL(%q) 应当报错，却造出了 %q", bad, dst)
		}
	}
}

// TestOpenFromSubdirReadsTheRightPath 是那个真 bug 的回归。
//
// 旧行为：列表能进 `plugins/`，点里面的 `open.go` 却去读 `CWD/open.go`，
// 于是预览永远显示"读不出来"。这个包当时没有测试，所以它活着。
func TestOpenFromSubdirReadsTheRightPath(t *testing.T) {
	const cwd = `D:\ws`
	listing := map[string][]pluginapi.FSEntry{
		cwd:              {{Name: "plugins", IsDir: true}},
		cwd + "/plugins": {{Name: "open.go"}},
	}
	var readPaths []string
	act := pluginapi.Actions{
		FSList: func(dir string) []pluginapi.FSEntry { return listing[dir] },
		FSRead: func(path string) (string, error) {
			readPaths = append(readPaths, path)
			return "package dsopen", nil
		},
	}
	data := pluginapi.Snapshot{Cur: "s-001", CWD: cwd}

	p := &Plugin{}
	// 第一帧：先让列表记住根目录（真实运行时也是先画再点）。
	renderFiles(t, p, data, act, "")
	if p.curDir != "" {
		t.Fatalf("还没点就进了目录：curDir=%q", p.curDir)
	}

	// 点目录行 → 进 plugins/
	renderFiles(t, p, data, act, "file-plugins")
	if p.curDir != "plugins" {
		t.Fatalf("点了目录之后 curDir = %q，want plugins", p.curDir)
	}

	// 点目录里的文件 → 必须按 CWD/plugins 拼，不能按 CWD 拼。
	renderFiles(t, p, data, act, "file-open.go")
	if len(readPaths) != 1 {
		t.Fatalf("fs.read 被调了 %d 次，want 1（%v）", len(readPaths), readPaths)
	}
	if want := cwd + "/plugins/open.go"; readPaths[0] != want {
		t.Errorf("读的路径 = %q，want %q\n"+
			"差一个 curDir 就是那个 bug：列表进得去子目录，点文件却回头读根目录。",
			readPaths[0], want)
	}
	if p.content != "package dsopen" {
		t.Errorf("预览内容没落到 p.content：%q", p.content)
	}
}

// TestListSkipsWhenHostHasNoCache 钉住"这一帧没有 ≠ 目录是空的"。
//
// 壳的 FSList 是异步缓存：第一次问某个目录**必然返回空**。
// 那时候什么都不画是对的；写一句"空目录"就是在编。
func TestListSkipsWhenHostHasNoCache(t *testing.T) {
	const cwd = `D:\ws`
	act := pluginapi.Actions{
		FSList: func(string) []pluginapi.FSEntry { return nil },
		FSRead: func(string) (string, error) { return "", nil },
	}
	p := &Plugin{}
	renderFiles(t, p, pluginapi.Snapshot{Cur: "s-001", CWD: cwd}, act, "")
	if p.sel != "" || p.curDir != "" {
		t.Errorf("空列表不该改状态：sel=%q curDir=%q", p.sel, p.curDir)
	}

	// 没有 CWD 时更不该有动作。
	p2 := &Plugin{}
	renderFiles(t, p2, pluginapi.Snapshot{Cur: "s-001"}, act, "")
	if p2.sel != "" || p2.curDir != "" {
		t.Errorf("没有 CWD 时不该改状态：sel=%q curDir=%q", p2.sel, p2.curDir)
	}
}

// TestPreviewOpensInDefaultApp 验「用默认程序打开」这条链。
//
// 官方 `open-in-app` 的那半句是 "the document preview's default-application
// controls for one file"。这里验的是插件造出来的 **file: 链接**是对的——
// 壳只认显式 file: URL，造错了它会拒绝，而用户看到的是"打开失败"。
func TestPreviewOpensInDefaultApp(t *testing.T) {
	var opened []string
	act := pluginapi.Actions{
		OpenFile: func(dst string) error {
			opened = append(opened, dst)
			return nil
		},
	}
	p := &Plugin{sel: "a.go", content: "package main", curDir: "plugins", theme: dscore.Dark()}
	p.prims = &dscore.Primitives{Theme: p.theme}

	drawPreviewOnce(t, p, pluginapi.Snapshot{Cur: "s", CWD: `D:\ws`}, act, "用默认程序打开")
	if len(opened) != 1 {
		t.Fatalf("点了「用默认程序打开」，OpenFile 被调 %d 次，want 1", len(opened))
	}
	if want := "file:///D:/ws/plugins/a.go"; opened[0] != want {
		t.Errorf("OpenFile 收到 %q，want %q", opened[0], want)
	}

	// 读失败时不该有这个按钮：那个路径本来就是坏的，开它只会再失败一次。
	p2 := &Plugin{sel: "a.go", err: "文件不存在", theme: dscore.Dark()}
	p2.prims = &dscore.Primitives{Theme: p2.theme}
	drawPreviewOnce(t, p2, pluginapi.Snapshot{Cur: "s", CWD: `D:\ws`}, act, "用默认程序打开")
	if len(opened) != 1 {
		t.Errorf("读失败时仍然调了 OpenFile（%v）", opened[1:])
	}

	// 壳没接 OpenFile（老壳）时按钮不画，点了也不该有任何动作。
	p3 := &Plugin{sel: "a.go", content: "x", theme: dscore.Dark()}
	p3.prims = &dscore.Primitives{Theme: p3.theme}
	drawPreviewOnce(t, p3, pluginapi.Snapshot{Cur: "s", CWD: `D:\ws`}, pluginapi.Actions{}, "用默认程序打开")
	if len(opened) != 1 {
		t.Errorf("OpenFile 为 nil 时仍然调用了（%v）", opened[1:])
	}
}

// drawPreviewOnce 画一帧预览浮层。
func drawPreviewOnce(t *testing.T, p *Plugin, data pluginapi.Snapshot, act pluginapi.Actions, clickLabel string) {
	t.Helper()
	// 先声明再赋值：闭包里要递归调用自己（画一帧 → 找到真实命中区 → 再画一次）。
	var draw func(clickID gfx.ID)
	draw = func(clickID gfx.ID) {
		tree := &gfx.Tree{}
		feat := tree.Feature("files-preview-test")
		canvas := &gfx.Canvas{}
		canvas.SetTree(tree)
		canvas.SetMeasurer(func(value string, size, weight float32) float32 {
			return float32(len([]rune(value))) * size * 0.55
		})
		if !clickID.IsZero() {
			canvas.BeginEventPass(clickID, gfx.ClickInfo{})
			defer canvas.EndEventPass()
		}
		env := &pluginapi.Env{
			Theme: p.theme, Data: data, Act: act, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: ui.Rect{Width: 1182, Height: 832},
		}
		pageTheme2 := dscore.VivaTheme(p.theme)
		view := ui.NewPage(ui.PageConfig{Theme: &pageTheme2})
		view.Draw(canvas, env.Bounds, func(f ui.C) {
			p.drawPreview(f, env)
		})
		if !clickID.IsZero() {
			return
		}
		// 无点击的一帧：如果给了文案，就用这一帧的真实命中区再点一次。
		// v0.6.0 的 ui.Button 没有显式交互 ID，v0.5.x 那套"按 ID 造点击目标"
		// 对预览按钮不再成立；点中的仍是同一个按钮、触发同一条链。
		if clickLabel == "" {
			return
		}
		var target gfx.ID
		for _, hit := range canvas.Hits() {
			if hit.Label == clickLabel && hit.Role == gfx.RoleButton {
				target = hit.ID
			}
		}
		if target.IsZero() {
			// 没有这个按钮是**被测行为本身**（读失败/壳没接 OpenFile 时按钮不画），
			// 所以这里不报错、也不再点：调用方的"OpenFile 被调 0 次"就是判据。
			return
		}
		draw(target)
	}
	draw(gfx.ID{})
}
