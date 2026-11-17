//go:build darwin || windows || linux

// shell_full_render_test.go —— **主窗口骨架的真实渲染截图**（顶栏 + 侧栏 + 中栏 + 右栏 + 状态栏）。
//
// 为什么要单独一张：`layoutWindow`（drag_visual_test.go）只装壳与四根柱子，
// 中栏的消息流/输入卡是空的黑块——那能证明分栏几何，证明不了"整窗画出来是不是
// 官方那个样子"。这里把**内容插件**也装进去（ds-conversation / ds-composer /
// ds-rightbar / ds-statusbar / ds-topbar），走真 Viva headless 的 Paint，
// 落 PNG 给人眼复核。
//
// 判据不是"图里有东西"，而是这几条**可机读**的事实同时成立：
//
//	① 每条真槽（main.header / main.body / main.composer / rightbar / shell.bottom）
//	   在同一帧里各被派发，且中栏确实画出了会话正文与输入卡；
//	② 右栏（ShowRightbar=true）真的占了一列，宽度等于官方
//	   `clampWidth(rightbar,300,viewport*.7)` 的首次打开值（viewport*.45）；
//	③ 截图尺寸 = 请求尺寸（不是被 DPI 缩放过的）。
//
// 截图落 `$VR_SHELL_CAPTURE_DIR`（默认 t.TempDir()）。
package dsshell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscomposer"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsrightbar"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar"
	"github.com/MYCF711/dsh-ui/plugins/dsstatusbar"
	"github.com/MYCF711/dsh-ui/plugins/dstopbar"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// renderShellData 是一份**够画满四栏**的假数据：三条会话、一段带工具卡与
// 代码块的正文、真值用量与权限。只为了让截图里出现真实行高/行距，
// 不证明任何宿主链路（那由 cmd/app 的 real_chain 用例负责）。
func renderShellData() pluginapi.Snapshot {
	return pluginapi.Snapshot{
		Cur:          "render-a",
		ShowRightbar: true,
		Status:       "就绪",
		Link:         pluginapi.LinkConnected,
		Model:        "group/deepseek-v4.1-flash-1017",
		Permission:   "ask",
		Sessions: []apiclient.Session{
			{ID: "render-a", Title: "渲染验收：主窗口骨架", Cwd: "D:/work/dsh-ui", UpdatedAt: "2026-11-18T03:00:00Z"},
			{ID: "render-b", Title: "官方 computeColumns 对拍", Cwd: "D:/work/dsh-ui", UpdatedAt: "2026-11-18T02:00:00Z"},
			{ID: "render-c", Title: "插件底座", Cwd: "D:/work/dsh-ui", UpdatedAt: "2026-11-18T01:00:00Z"},
		},
		Msgs: []apiclient.Message{
			{Role: "user", Text: "把主窗口画出来看一眼：顶栏、侧栏、中栏、右栏都在不在？"},
			{Role: "assistant", Text: "四栏都在。`computeColumns` 给的是 280 / 902 / 708，分隔线不画。",
				Usage: &apiclient.Usage{InputTokens: 1200, OutputTokens: 96, TotalTokens: 1296}},
			{Role: "tool", Tool: "bash", Args: "CGO_ENABLED=0 go test ./plugins/dsshell",
				Summary: "exit 0 · 12 passed · 0.26 s", View: &apiclient.ViewSpec{Badge: "ok"}},
			{Role: "assistant", Text: "## 结论\n\n三栏骨架与状态栏都按官方几何落位，右栏默认关闭、打开时占视口 45%。"},
		},
		UsageTotal:    pluginapi.Usage{InputTokens: 1200, OutputTokens: 96, TotalTokens: 1296},
		ContextTokens: 1296, ContextWindow: 128000,
	}
}

// TestFullShellRendersFourColumns 渲染主窗口骨架并落 PNG。
func TestFullShellRendersFourColumns(t *testing.T) {
	dir := os.Getenv("VR_SHELL_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, size := range []struct {
		name          string
		width, height int
	}{{"1182x832", 1182, 832}, {"1920x1032", 1920, 1032}} {
		t.Run(size.name, func(t *testing.T) {
			captureFullShell(t, dir, size.width, size.height)
		})
	}
}

func captureFullShell(t *testing.T, dir string, width, height int) {
	t.Helper()
	kernel := uikernel.New("vr-shell")
	if err := kernel.Load(
		&dscore.Plugin{Dark: true},
		&Plugin{}, &dssidebar.Plugin{}, &dstopbar.Plugin{},
		&dsconversation.Plugin{}, &dscomposer.Plugin{},
		&dsrightbar.Plugin{}, &dsstatusbar.Plugin{},
	); err != nil {
		t.Fatalf("装配产品插件集失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("vr-shell")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	data := renderShellData()

	const sideWidth = 280
	var invalidator app.Invalidator
	config := app.Config{
		Title: "vr-shell", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		rightPref := size.Width * rightbarOpenFrac
		env := &pluginapi.Env{
			Theme: theme, Data: data, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			Geometry: pluginapi.Geometry{
				Width: size.Width, Height: size.Height, SidebarWidth: sideWidth,
				RightWidth: rightPref, MainWidth: size.Width - sideWidth - rightPref,
			},
			MainWidth: size.Width - sideWidth - rightPref,
			Has:       kernel.Has,
			Act: pluginapi.Actions{
				Send: func(string) {}, Cancel: func() {}, NewSession: func() {},
			},
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			kernel.DrawSlot(f, pluginapi.SlotShellRoot, env)
		})
		for _, painter := range kernel.AllPainters() {
			if painter.AfterDraw != nil {
				painter.AfterDraw(canvas, env)
			}
		}
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for range 2 {
		invalidator.Invalidate()
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}

	// ① 中栏真的画出了正文与输入卡（不是空黑块）。只挑**不会因换行位置变化**的串：
	//    CJK 长句按可用宽度断行，跨平台字宽不同断点就不同；拿整句去 Contains 是在测字体。
	joined := strings.Join(session.Texts(), "\u0000")
	for _, want := range []string{
		"computeColumns",  // assistant 正文（ASCII 词，不会被断在中间）
		"bash",            // 工具卡标题行
		"渲染验收：主窗口骨架",      // 侧栏列表 + 顶栏面包屑的会话标题
		"说点什么…（Enter 发送）", // composer 真实输入框占位（单条 Text）
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("%dx%d 骨架首帧缺 %q；画布文本（%d 条）：%q", width, height, want, len(session.Texts()), session.Texts())
		}
	}
	// ② 三列几何：右栏按官方 computeColumns 求解（首次打开偏好视口 45%，
	//    再被 available = viewport - sidebar - 400 截断；放不下 300 时整列消失）。
	handle, ok := findHit(session.Snapshot(), handleLabel(handleRightbar))
	if !ok {
		t.Fatal("找不到右栏分隔线命中的抓取带（官方 .handle 8px）")
	}
	available := float32(width) - sideWidth - mainMin
	pref := float32(width) * rightbarOpenFrac
	if pref < rightbarMin {
		pref = rightbarMin
	}
	if pref > float32(width)*rightbarMaxFrac {
		pref = float32(width) * rightbarMaxFrac
	}
	rightW := min(available, pref)
	wantRight := float32(width) - rightW
	if got := handle.Bounds.X + handleOffset; got < wantRight-1 || got > wantRight+1 {
		t.Fatalf("右栏列边界 x=%.1f，官方 computeColumns 应为 %.1f（右栏 %.1f）", got, wantRight, rightW)
	}
	if handle.Bounds.Width != handleWidth {
		t.Fatalf("抓取带宽 = %.1f，官方 .handle{width:8px}", handle.Bounds.Width)
	}

	path := filepath.Join(dir, "shell-"+itoa(width)+"x"+itoa(height)+".png")
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	img, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != width || img.Bounds().Dy() != height {
		t.Fatalf("截图尺寸 = %v，期望 %dx%d", img.Bounds(), width, height)
	}
	t.Logf("VR_SHELL_CAPTURE path=%s", path)
}

// itoa 是本包测试共用的小工具（避免引入 strconv 只为一个尺寸名）。
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for v > 0 {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
	}
	return string(digits[i:])
}
