//go:build darwin || windows || linux

package dstopbar

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscomposer"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsrightbar"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// TestConversationTabsReallyClickAndRememberSession 使用同一个 Viva 无头窗口、
// 真实 kernel 绘制顺序和真实鼠标命中，验证切页不是只改标题。
func TestConversationTabsReallyClickAndRememberSession(t *testing.T) {
	data := pluginapi.Snapshot{
		Cur: "session-a",
		Sessions: []apiclient.Session{
			{ID: "session-a", Title: "会话 A", Cwd: `D:\work\a`},
			{ID: "session-b", Title: "会话 B", Cwd: `D:\work\b`},
			{ID: "session-c", Title: "会话 C", Cwd: `D:\work\c`},
		},
		Msgs: []apiclient.Message{
			{Role: "user", Text: "CHAT-MARKER-A", Seq: 1},
			{Role: "tool", Tool: "bash", Summary: "TRAJECTORY-TOOL-A", Seq: 2, View: &apiclient.ViewSpec{Badge: "ok"}},
		},
		Busy: true,
	}
	var loads, cancels, draftWrites int
	composer := &dscomposer.Plugin{}
	topbar := &Plugin{}
	rightbar := &dsrightbar.Plugin{}
	conversation := &dsconversation.Plugin{}
	kernel := uikernel.New("fg3-click")
	// 与 app 注册顺序一致：rightbar 在 topbar 前声明，chat 由壳随后追加。
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dsshell.Plugin{}, rightbar, topbar, conversation, composer); err != nil {
		t.Fatal(err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("fg3-click")
	var view flex.View
	config := app.Config{Title: "fg3-tabs", Width: 1182, Height: 832, Tree: tree, WindowMode: app.WindowFrameless}
	config.TextInput = ui.Fields(composer.Fields()...)
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		data.Total = len(data.Msgs)
		env := fg3TestEnv(theme, tree, feat, canvas, bounds, data, kernel)
		env.Act = pluginapi.Actions{
			OpenSession: func(string) { loads++ },
			Cancel:      func() { cancels++ },
			SetDraft:    func(string) { draftWrites++ },
			ToggleRightbar: func() {
				data.ShowRightbar = !data.ShowRightbar
			},
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f *flex.Frame) {
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
	composer.Fields()[0].SetText("draft-keep")
	repaint := func() {
		t.Helper()
		if err := session.Resize(1183, 832); err != nil {
			t.Fatal(err)
		}
	}
	assertNoChatResidue := func() {
		t.Helper()
		for _, residue := range []string{"没有匹配", "探索未至之境"} {
			if session.HasText(residue) {
				t.Fatalf("trajectory 残留 chat 文本 %q：%q", residue, session.Texts())
			}
		}
	}
	repaint()
	if !session.HasText("CHAT-MARKER-A") {
		t.Fatalf("初始 chat 未实画：%q", session.Texts())
	}
	// 即使历史里已有 tool，ShowRightbar=false 也必须保持官方默认收起。
	if session.HasText("1 次") {
		t.Fatalf("含 tool 的会话未经点击就展开了右栏：%q", session.Texts())
	}
	if err := session.ClickLabel("打开右侧边栏"); err != nil {
		t.Fatalf("28px corner 不能展开：%v", err)
	}
	if !session.HasText("1 次") {
		t.Fatalf("corner 展开后右栏不可见：%q", session.Texts())
	}
	if err := session.ClickLabel("收起右侧边栏"); err != nil {
		t.Fatalf("右栏 PanelChrome 不能收起：%v", err)
	}
	if session.HasText("1 次") {
		t.Fatalf("右栏收起后仍可见：%q", session.Texts())
	}
	if err := session.ClickLabel("轨迹"); err != nil {
		t.Fatalf("真实点击轨迹页签失败：%v", err)
	}
	if session.HasText("CHAT-MARKER-A") || !session.HasText("TRAJECTORY-TOOL-A") {
		t.Fatalf("chat→trajectory 正文没有真实切换：%q", session.Texts())
	}
	assertNoChatResidue()
	if got := composer.Fields()[0].Text(); got != "draft-keep" {
		t.Fatalf("切页改了 draft：%q", got)
	}

	data.Cur = "session-b"
	data.Msgs = []apiclient.Message{{Role: "user", Text: "CHAT-MARKER-B", Seq: 11}}
	data.Busy = false
	repaint()
	if !session.HasText("CHAT-MARKER-B") {
		t.Fatalf("新会话没有从默认 chat 开始：%q", session.Texts())
	}
	if err := session.ClickLabel("轨迹"); err != nil {
		t.Fatal(err)
	}
	if session.HasText("CHAT-MARKER-B") || !session.HasText("本会话还没有工具调用") {
		t.Fatalf("无工具会话的轨迹空态没有真实切换：%q", session.Texts())
	}
	assertNoChatResidue()
	if err := session.ClickLabel("对话"); err != nil {
		t.Fatal(err)
	}
	if !session.HasText("CHAT-MARKER-B") {
		t.Fatalf("trajectory→chat 没有恢复正文：%q", session.Texts())
	}

	// 活跃但尚无历史消息的边界也不能漏出 Hero/“没有匹配”或虚构工具条目。
	data.Cur = "session-c"
	data.Msgs = nil
	data.Busy = true
	repaint()
	if err := session.ClickLabel("轨迹"); err != nil {
		t.Fatal(err)
	}
	if !session.HasText("本会话还没有工具调用") {
		t.Fatalf("零消息活跃会话的轨迹空态缺失：%q", session.Texts())
	}
	assertNoChatResidue()

	data.Cur = "session-a"
	data.Msgs = []apiclient.Message{
		{Role: "user", Text: "CHAT-MARKER-A", Seq: 1},
		{Role: "tool", Tool: "bash", Summary: "TRAJECTORY-TOOL-A", Seq: 2, View: &apiclient.ViewSpec{Badge: "ok"}},
	}
	data.Busy = true
	repaint()
	if session.HasText("CHAT-MARKER-A") || !session.HasText("TRAJECTORY-TOOL-A") {
		t.Fatalf("切回 A 没有恢复它自己的 trajectory 选择：%q", session.Texts())
	}
	assertNoChatResidue()
	if err := session.ClickLabel("对话"); err != nil {
		t.Fatal(err)
	}
	if !session.HasText("CHAT-MARKER-A") {
		t.Fatalf("A 返回 chat 失败：%q", session.Texts())
	}
	captureDir := os.Getenv("FG3_CAPTURE_DIR")
	if captureDir == "" {
		captureDir = t.TempDir()
	} else if err := os.MkdirAll(captureDir, 0o755); err != nil {
		t.Fatal(err)
	}
	capture := func(width, height int, viewName string) {
		t.Helper()
		if err := session.Resize(width, height); err != nil {
			t.Fatal(err)
		}
		boxes := topbar.headerBoxes
		if got := boxes.Body.Rect().Height; got != 76 {
			t.Fatalf("%dx%d active Header=%v，官方 76", width, height, got)
		}
		if got := boxes.TitleRow.Rect().Height; got != 30 {
			t.Fatalf("%dx%d titleRow=%v，官方 30", width, height, got)
		}
		if got := boxes.Tabs.Rect().Height; got != 25 {
			t.Fatalf("%dx%d tabs=%v，官方 25", width, height, got)
		}
		corner := boxes.Corner.Rect()
		if corner.Width != 28 || corner.Height != 28 {
			t.Fatalf("%dx%d corner=%+v，官方 28×28", width, height, corner)
		}
		tabTop := boxes.Tabs.Rect().Y - (boxes.TitleRow.Rect().Y + boxes.TitleRow.Rect().Height)
		tabGap := boxes.TrajectoryTab.Rect().X - (boxes.ChatTab.Rect().X + boxes.ChatTab.Rect().Width)
		if tabTop != 10 || tabGap != 20 {
			t.Fatalf("%dx%d tabs 间距：top=%v/10 gap=%v/20", width, height, tabTop, tabGap)
		}
		name := "frame-header-" + viewName + "-" + strconv.Itoa(width) + "x" + strconv.Itoa(height) + ".png"
		path := filepath.Join(captureDir, name)
		if err := session.Screenshot(path); err != nil {
			t.Fatal(err)
		}
		picture, err := session.Image()
		if err != nil {
			t.Fatal(err)
		}
		if picture.Bounds().Dx() != width || picture.Bounds().Dy() != height {
			t.Fatalf("截图尺寸=%v，want %dx%d", picture.Bounds(), width, height)
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("截图未落盘：path=%s info=%v err=%v", path, info, err)
		}
		t.Logf("FG3_CAPTURE path=%s size=%dx%d bytes=%d view=%s header=76 title=30 tabs=25 corner=28 top=10 gap=20", path, width, height, info.Size(), viewName)
	}
	capture(1182, 832, "chat")
	if err := session.ClickLabel("轨迹"); err != nil {
		t.Fatal(err)
	}
	assertNoChatResidue()
	capture(1182, 832, "trajectory")
	if err := session.ClickLabel("对话"); err != nil {
		t.Fatal(err)
	}
	capture(1920, 1032, "chat")
	if err := session.ClickLabel("轨迹"); err != nil {
		t.Fatal(err)
	}
	assertNoChatResidue()
	capture(1920, 1032, "trajectory")
	if loads != 0 || cancels != 0 || draftWrites != 0 {
		t.Fatalf("切页触发了禁用的副作用：loads=%d cancels=%d draftWrites=%d", loads, cancels, draftWrites)
	}
}

func fg3TestEnv(theme *pluginapi.Theme, tree *gfx.Tree, feat gfx.Scope, canvas *gfx.Canvas, bounds ui.Rect, data pluginapi.Snapshot, kernel *uikernel.Kernel) *pluginapi.Env {
	var right float32
	if data.ShowRightbar {
		right = bounds.Width * 0.45
		if available := bounds.Width - 280 - 400; available < 300 {
			right = 0
		} else if right > available {
			right = available
		}
	}
	return &pluginapi.Env{
		Theme: theme, Data: data, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
		MainWidth: bounds.Width - 280 - right,
		Geometry: pluginapi.Geometry{
			Width: bounds.Width, Height: bounds.Height, SidebarWidth: 280,
			RightWidth: right, MainWidth: bounds.Width - 280 - right,
		},
		Has: func(slot pluginapi.Slot) bool { return kernel.Has(slot) },
	}
}
