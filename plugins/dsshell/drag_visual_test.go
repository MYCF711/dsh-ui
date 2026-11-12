//go:build darwin || windows || linux

package dsshell

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsrightbar"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar"
	"github.com/MYCF711/dsh-ui/plugins/dsstatusbar"
	"github.com/MYCF711/dsh-ui/plugins/dstopbar"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

const (
	layoutW = 1920
	layoutH = 1032
)

// layoutData 是一份固定的假数据：三个会话 + 一条工具消息，够侧栏和右栏都画出东西。
// 只为了截图里有真实的行高/行距，不证明任何宿主链路。
func layoutData() pluginapi.Snapshot {
	return pluginapi.Snapshot{
		Cur:          "layout-a",
		ShowRightbar: true,
		Sessions: []apiclient.Session{
			{ID: "layout-a", Title: "分栏拖拽验证", Cwd: "D:/work/dsh-ui", UpdatedAt: "2026-11-04T03:00:00Z"},
			{ID: "layout-b", Title: "官方 computeColumns 对拍", Cwd: "D:/work/dsh-ui", UpdatedAt: "2026-11-04T02:00:00Z"},
			{ID: "layout-c", Title: "插件底座", Cwd: "D:/work/dsh-ui", UpdatedAt: "2026-11-04T01:00:00Z"},
		},
		Msgs: []apiclient.Message{
			{Role: "user", Text: "侧栏能拖吗？"},
			{Role: "tool", Tool: "bash", Summary: "CGO_ENABLED=0 go test ./plugins/dsshell", View: &apiclient.ViewSpec{Badge: "ok"}},
			{Role: "assistant", Text: "分隔线在 x=280，往右拖就是 344。"},
		},
	}
}

// layoutWindow 起一个真窗口（无头）跑 dscore+dsshell+dssidebar+dstopbar+dsrightbar+dsstatusbar。
// act 是壳给插件的动作（这里只用 ConfigGet/ConfigSet，模拟宿主的 ui-config.json）。
func layoutWindow(t *testing.T, shell *Plugin, act pluginapi.Actions) (*headless.Session, *pluginapi.Theme) {
	t.Helper()
	kernel := uikernel.New("layout-drag")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, shell, &dssidebar.Plugin{}, &dstopbar.Plugin{}, &dsrightbar.Plugin{}, &dsstatusbar.Plugin{}); err != nil {
		t.Fatal(err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("layout-drag")
	// 壳给的侧栏偏好宽：官方默认 280（dssidebar.Plugin.Width()）。这里写死是因为
	// 这条用例要验的正是「壳说 280、用户拖到 360 之后画出来的是 360」。
	const sideWidth = 280

	var view flex.View
	config := app.Config{
		Title: "layout-drag", Width: layoutW, Height: layoutH,
		Tree: tree, WindowMode: app.WindowFrameless,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		rightPref := size.Width * rightbarOpenFrac
		env := &pluginapi.Env{
			Theme: theme,
			Data:  layoutData(),
			Tree:  tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			// 壳的口径：侧栏 280、右栏视口 45%。用户拖过之后 dsshell 会自己覆盖，
			// 这正是这次要验的东西（Geometry 是「默认值」，不是「真值」）。
			// BottomHeight = 0：连接指示已迁到 `sidebar.settings`（见
			// plugins/dsstatusbar/connection.go），本插件集里再没有任何人占
			// `shell.bottom`——官方默认 bottom 就是空的。旧值 24 是那条被迁走的
			// 全宽状态行的残余，留着会让这条用例继续钉住一个已经不存在的层。
			Geometry: pluginapi.Geometry{
				Width: size.Width, Height: size.Height, SidebarWidth: sideWidth,
				RightWidth: rightPref, MainWidth: size.Width - sideWidth - rightPref,
			},
			MainWidth: size.Width - sideWidth - rightPref,
			Act:       act,
			Has:       kernel.Has,
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f *flex.Frame) {
			kernel.DrawSlot(f, pluginapi.SlotShellRoot, env)
		})
	}

	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session, theme
}

// findHit 在快照里按标签找命中区。
func findHit(snapshot app.Snapshot, label string) (gfx.Hit, bool) {
	for _, hit := range snapshot.Hits {
		if hit.Label == label {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// TestLayoutDragHeadless 真窗口 + 真指针事件：拖侧栏分隔线，
// 拖前/拖中/拖后各留一张 PNG，并逐项断言几何、光标和像素。
func TestLayoutDragHeadless(t *testing.T) {
	dir := os.Getenv("LAYOUT_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	host := &fakeHostConfig{kv: map[string]string{}}
	shell := &Plugin{}
	session, _ := layoutWindow(t, shell, host.actions())

	if err := session.Resize(layoutW, layoutH); err != nil {
		t.Fatal(err)
	}
	before := session.Snapshot()
	hit, ok := findHit(before, "拖动调整侧栏宽度")
	if !ok {
		t.Fatalf("启动时没有侧栏拖拽把手；hits=%+v", before.Hits)
	}
	// 底部整行已经没人占（BottomHeight 0）→ 把手贯穿整个内容高度。
	assertRect(t, "拖前侧栏把手", hit.Bounds,
		ui.Rect{X: 280 - handleWidth/2, Y: 0, Width: handleWidth, Height: layoutH})
	if _, ok := findHit(before, "拖动调整右栏宽度"); !ok {
		t.Fatal("右栏展开时应该有右栏拖拽把手")
	}

	// 拖前帧。
	beforePNG := filepath.Join(dir, "layout-before.png")
	if err := session.Screenshot(beforePNG); err != nil {
		t.Fatal(err)
	}
	beforeImage, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}

	// 悬停：指针必须变成 SizeWE（这就是验收里的「hover 变指针」）。
	if err := session.Move(280, 400); err != nil {
		t.Fatal(err)
	}
	hover := session.Snapshot()
	if hover.Cursor != app.CursorResizeHorizontal {
		t.Fatalf("悬停分隔线时指针应为 SizeWE（CursorResizeHorizontal），得到 %v", hover.Cursor)
	}
	hoverPNG := filepath.Join(dir, "layout-hover.png")
	if err := session.Screenshot(hoverPNG); err != nil {
		t.Fatal(err)
	}
	hoverImage, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	// 悬停时那一帧在分隔线上画了 2px 竖线（平时不画）：x=280 处像素变了。
	if sameColor(hoverImage.At(280, 600), beforeImage.At(280, 600)) {
		t.Fatalf("悬停时分隔线上应出现可见的拖拽线，x=280 仍是原色 %v", beforeImage.At(280, 600))
	}

	// 按下 → 拖动（中间多来几个 move，模拟真实的连续拖动）→ 松手。
	if err := session.Press(280, 400); err != nil {
		t.Fatal(err)
	}
	for _, x := range []float32{300, 320, 340, 360} {
		if err := session.Move(x, 400); err != nil {
			t.Fatal(err)
		}
	}
	during := session.Snapshot()
	if got, ok := findHit(during, "拖动调整侧栏宽度"); !ok || got.Bounds.X != 360-handleWidth/2 {
		t.Fatalf("拖动中把手没跟到 360：%+v ok=%v", got.Bounds, ok)
	}
	// 拖动过程中**不落盘**：一次拖拽只写一次（松手时），否则就是几十次 HTTP。
	if host.sets != 0 {
		t.Fatalf("拖动过程中不该写配置，已经写了 %d 次：%v", host.sets, host.snapshot())
	}
	if err := session.Release(360, 400); err != nil {
		t.Fatal(err)
	}
	// 松手落盘：键是宿主 ui-config.json 的 ui.sidebarWidth，值是像素整数。
	if host.sets != 1 {
		t.Fatalf("松手应写一次配置，实际 %d 次", host.sets)
	}
	if got := host.value(cfgKeySidebar); got != "360" {
		t.Fatalf("%s 应写成 360，实际 %q（整份=%v）", cfgKeySidebar, got, host.snapshot())
	}
	// 没拖过右栏：不该凭空写一个右栏值进去（不替用户决定他没碰过的列）。
	if v, ok := host.snapshot()[cfgKeyRightbar]; ok {
		t.Fatalf("没拖右栏却写了 %s=%q", cfgKeyRightbar, v)
	}

	after := session.Snapshot()
	if shell.UserSidebarWidth != 360 {
		t.Fatalf("松手后用户宽度应为 360，得到 %v", shell.UserSidebarWidth)
	}
	if shell.dragging.active {
		t.Fatal("松手后拖拽状态应结束")
	}
	afterHit, ok := findHit(after, "拖动调整侧栏宽度")
	if !ok {
		t.Fatalf("松手后把手不见了；hits=%+v", after.Hits)
	}
	assertRect(t, "拖后侧栏把手", afterHit.Bounds,
		ui.Rect{X: 360 - handleWidth/2, Y: 0, Width: handleWidth, Height: layoutH})
	// 右栏没被牵动：左缘仍是 1920-864=1056。
	if rb, ok := findHit(after, "拖动调整右栏宽度"); !ok || rb.Bounds.X != 1056-handleWidth/2 {
		t.Fatalf("拖侧栏不该动右栏：%+v ok=%v", rb.Bounds, ok)
	}

	afterPNG := filepath.Join(dir, "layout-after.png")
	if err := session.Screenshot(afterPNG); err != nil {
		t.Fatal(err)
	}
	afterImage, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	// 像素级证据：x=330 原来是主区底色，现在落在侧栏里（侧栏底色不同）。
	if sameColor(afterImage.At(330, 300), beforeImage.At(330, 300)) {
		t.Fatal("拖后 x=330 的像素没变——侧栏列没有真的变宽")
	}
	if !sameColor(afterImage.At(330, 300), afterImage.At(4, 300)) &&
		!sameColor(afterImage.At(330, 300), afterImage.At(4, 700)) {
		// x=4/y=300 可能落在某一行的高亮上，所以退一步看另一处纯底色点。
		t.Logf("提示：x=330 处颜色与侧栏底色采样点不同，仅作记录（%v）", afterImage.At(330, 300))
	}

	for _, p := range []string{beforePNG, hoverPNG, afterPNG} {
		info, err := os.Stat(p)
		if err != nil || info.Size() == 0 {
			t.Fatalf("截图为空：%s info=%v err=%v", p, info, err)
		}
		t.Logf("LAYOUT_CAPTURE path=%s bytes=%d", p, info.Size())
	}
	fmt.Println("LAYOUT drag 280 -> 360 completed")
}

// TestLayoutWidthRestoredHeadless 走完整的持久化回路，模拟「重启」：
//
//	第一次运行：拖侧栏到 344、拖右栏到 700 → Actions.ConfigSet 写进宿主配置
//	关窗（= 进程退出）
//	第二次运行：**全新插件实例**，Actions.ConfigGet 交回同一份配置 → 首帧就是 344/700
//
// 这里替掉的只有最后一跳（Actions.ConfigSet → apiclient → HTTP → ui-config.json），
// 那一段在壳里；插件这一侧从"写"到"读"是整条真路径。
func TestLayoutWidthRestoredHeadless(t *testing.T) {
	dir := os.Getenv("LAYOUT_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	host := &fakeHostConfig{kv: map[string]string{}}

	// ── 第一次运行：拖出 344 / 700，松手落盘 ──
	first := &Plugin{}
	s1, _ := layoutWindow(t, first, host.actions())
	if err := s1.Resize(layoutW, layoutH); err != nil {
		t.Fatal(err)
	}
	if err := s1.Move(280, 400); err != nil {
		t.Fatal(err)
	}
	if err := s1.Press(280, 400); err != nil {
		t.Fatal(err)
	}
	if err := s1.Move(344, 400); err != nil {
		t.Fatal(err)
	}
	if err := s1.Release(344, 400); err != nil {
		t.Fatal(err)
	}
	// 右栏：1056 → 往右 164px → 864-164 = 700。
	rightX := float32(layoutW) - 864
	if err := s1.Move(rightX, 400); err != nil {
		t.Fatal(err)
	}
	if err := s1.Press(rightX, 400); err != nil {
		t.Fatal(err)
	}
	if err := s1.Move(rightX+164, 400); err != nil {
		t.Fatal(err)
	}
	if err := s1.Release(rightX+164, 400); err != nil {
		t.Fatal(err)
	}
	if first.UserSidebarWidth != 344 || first.UserRightbarWidth != 700 {
		t.Fatalf("拖动结果应为 344/700，得到 %v/%v",
			first.UserSidebarWidth, first.UserRightbarWidth)
	}
	// 落盘：键名是宿主 ui-config.json 里的 ui.sidebarWidth / ui.rightbarWidth。
	if got := host.value(cfgKeySidebar); got != "344" {
		t.Fatalf("%s 应写成 344，实际 %q（整份=%v）", cfgKeySidebar, got, host.snapshot())
	}
	if got := host.value(cfgKeyRightbar); got != "700" {
		t.Fatalf("%s 应写成 700，实际 %q（整份=%v）", cfgKeyRightbar, got, host.snapshot())
	}
	t.Logf("LAYOUT_CONFIG %v", host.snapshot())
	s1.Close()

	// ── 第二次运行：新实例、同一份配置 ──
	second := &Plugin{}
	session, _ := layoutWindow(t, second, host.actions())
	if err := session.Resize(layoutW, layoutH); err != nil {
		t.Fatal(err)
	}
	snapshot := session.Snapshot()

	hit, ok := findHit(snapshot, "拖动调整侧栏宽度")
	if !ok {
		t.Fatalf("恢复后没有侧栏把手；hits=%+v", snapshot.Hits)
	}
	assertRect(t, "恢复的侧栏把手", hit.Bounds,
		ui.Rect{X: 344 - handleWidth/2, Y: 0, Width: handleWidth, Height: layoutH})
	// 右栏 700：左缘 = 1920-700 = 1220。
	rb, ok := findHit(snapshot, "拖动调整右栏宽度")
	if !ok {
		t.Fatalf("恢复后没有右栏把手；hits=%+v", snapshot.Hits)
	}
	assertRect(t, "恢复的右栏把手", rb.Bounds,
		ui.Rect{X: 1220 - handleWidth/2, Y: 0, Width: handleWidth, Height: layoutH})

	path := filepath.Join(dir, "layout-restored.png")
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("截图为空：%s info=%v err=%v", path, info, err)
	}
	t.Logf("LAYOUT_CAPTURE path=%s bytes=%d sidebar=%.0f rightbar=%.0f",
		path, info.Size(), second.UserSidebarWidth, second.UserRightbarWidth)
	fmt.Println("LAYOUT restored from ui-config 344 / 700 completed")
}

// sameColor 比较两个像素（只比 RGB，不比 alpha：PNG 里底色可能带 alpha）。
func sameColor(a, b interface{ RGBA() (r, g, b2, a2 uint32) }) bool {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	return ar == br && ag == bg && ab == bb
}
