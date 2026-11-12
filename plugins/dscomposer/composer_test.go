package dscomposer

import (
	"math"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// renderComposer 用真实 flex 布局和真实 ds-core 图元画一帧，不启动窗口。
// click 非空时走 canvas 的事件 pass，因此测试到的是按钮实际 Clicked 分支，
// 不是直接调用一个与界面脱节的辅助函数。
// renderComposer 按 v0.5.x 的写法点一帧：click 是一个**名字**，用它在本次的树里
// 造出交互 ID。ds-core 的 prims 控件（发送/停止/提及面板…）仍然是显式 ID 的，
// 所以这条老路径对它们依旧成立。
func renderComposer(t *testing.T, p *Plugin, data pluginapi.Snapshot, act pluginapi.Actions, mainWidth, windowWidth float32, click string) *gfx.Canvas {
	t.Helper()
	return renderComposerAt(t, p, data, act, mainWidth, windowWidth, click, gfx.ID{})
}

// renderComposerAt 是唯一渲染入口：点击目标要么按名字造（clickName），要么直接用
// 真实命中区 ID（clickID）。v0.6.0 的 ui.Button 不再有显式交互 ID，approval.go 里
// 那几个按钮只能先画一帧拿到真实 ID 再点（见 renderComposerClickLabel）。
func renderComposerAt(t *testing.T, p *Plugin, data pluginapi.Snapshot, act pluginapi.Actions, mainWidth, windowWidth float32, clickName string, clickID gfx.ID) *gfx.Canvas {
	t.Helper()
	if p.theme == nil {
		p.theme = dscore.Dark()
	}
	if p.prims == nil {
		p.prims = &dscore.Primitives{Theme: p.theme}
	}
	tree := &gfx.Tree{}
	feat := tree.Feature("composer-test")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	if clickName != "" {
		clickID = tree.ID(feat, clickName)
	}
	if !clickID.IsZero() {
		canvas.BeginEventPass(clickID, gfx.ClickInfo{})
		defer canvas.EndEventPass()
	}
	env := &pluginapi.Env{
		Theme: p.theme, Data: data, Act: act, Tree: tree, Feat: feat,
		Canvas: canvas, Bounds: ui.Rect{Width: windowWidth, Height: 400},
		MainWidth: mainWidth,
		Geometry: pluginapi.Geometry{
			Width: windowWidth, Height: 400, SidebarWidth: 280, MainWidth: mainWidth,
		},
	}
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		// 模拟 dsshell 的中栏内容框：MainWidth 减去壳的左右 12px padding。
		// 直接在最外层调用会把整窗误当中栏，测的就不是真实装配几何了。
		ui.Column(f).Width(mainWidth - 24).Children(func() { p.draw(f, env) })
	})
	p.afterDraw(canvas, env)
	return canvas
}

// renderComposerClickLabel 按**文案**点一帧。
//
// 为什么需要它：v0.6.0 的 `ui.Button` 没有显式交互 ID（身份来自声明位置），
// 审批卡（approval.go）用的正是 ui.Button，所以 v0.5.x 那套"按 ID 造点击目标"
// 对它不再成立。这里先普通画一帧拿到真实命中区，再用它的 ID 走一次事件 pass；
// 点中的仍然是同一个按钮、触发同一个回调，断言强度不变。
func renderComposerClickLabel(t *testing.T, p *Plugin, data pluginapi.Snapshot, act pluginapi.Actions, mainWidth, windowWidth float32, label string) *gfx.Canvas {
	t.Helper()
	probe := renderComposerAt(t, p, data, act, mainWidth, windowWidth, "", gfx.ID{})
	var target gfx.ID
	labels := make([]string, 0, len(probe.Hits()))
	for _, hit := range probe.Hits() {
		labels = append(labels, hit.Label)
		if hit.Label == label && hit.Role == gfx.RoleButton {
			target = hit.ID
		}
	}
	if target.IsZero() {
		t.Fatalf("没有文案为 %q 的按钮命中区；实际命中区文案=%v", label, labels)
	}
	return renderComposerAt(t, p, data, act, mainWidth, windowWidth, "", target)
}

func largestSurface(canvas *gfx.Canvas, color ui.Color) (gfx.Rectangle, bool) {
	var best gfx.Rectangle
	found := false
	for _, r := range canvas.Rectangles() {
		if r.Color != color {
			continue
		}
		area := r.Bounds.Width * r.Bounds.Height
		if !found || area > best.Bounds.Width*best.Bounds.Height {
			best, found = r, true
		}
	}
	return best, found
}

func closeEnough(got, want float32) bool { return math.Abs(float64(got-want)) <= 0.01 }

func TestComposerCardMatchesOfficialGeometry(t *testing.T) {
	// 官方 ConversationRoot + InputBar：
	// content=clamp(680, main*.64, 920)，cardMax=content+32；
	// shell 左右各 12，composer root 左右各 16，卡片在剩余空间中居中。
	// hero（无会话）input 52 → 卡高 114；active（有会话）input 36 → 卡高 98。
	// 其余都是 top 8 + gap 12 + controlRow(2+34+6)。
	cases := []struct {
		name       string
		main       float32
		window     float32
		cur        string
		wantWidth  float32
		wantLocalX float32
		wantHeight float32
	}{
		{name: "1920 hero", main: 1640, window: 1920, wantWidth: 952, wantLocalX: 332, wantHeight: 114},
		{name: "1920 active", main: 1640, window: 1920, cur: "s-001", wantWidth: 952, wantLocalX: 332, wantHeight: 98},
		{name: "1182 hero", main: 902, window: 1182, wantWidth: 712, wantLocalX: 83, wantHeight: 114},
		{name: "1182 active", main: 902, window: 1182, cur: "s-001", wantWidth: 712, wantLocalX: 83, wantHeight: 98},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &Plugin{}
			canvas := renderComposer(t, p, pluginapi.Snapshot{Cur: tc.cur}, pluginapi.Actions{}, tc.main, tc.window, "")
			card, ok := largestSurface(canvas, p.theme.InputMajor)
			if !ok {
				t.Fatal("找不到 composer 的 InputMajor 面板")
			}
			if !closeEnough(card.Bounds.Width, tc.wantWidth) {
				t.Errorf("卡片宽 = %.2f，官方 %.2f", card.Bounds.Width, tc.wantWidth)
			}
			if !closeEnough(card.Bounds.X, tc.wantLocalX) {
				t.Errorf("卡片在主区内容框中的 x = %.2f，官方居中 x %.2f", card.Bounds.X, tc.wantLocalX)
			}
			if !closeEnough(card.Bounds.Height, tc.wantHeight) {
				t.Errorf("卡片高 = %.2f，官方 %.2f", card.Bounds.Height, tc.wantHeight)
			}
			if !closeEnough(card.Radius, 28) {
				t.Errorf("卡片圆角 = %.2f，官方 radius-panel 28", card.Radius)
			}
		})
	}
}

func TestPrimaryButtonMatchesOfficialSizeAndColor(t *testing.T) {
	p := &Plugin{}
	p.field.SetText("draft")
	canvas := renderComposer(t, p, pluginapi.Snapshot{}, pluginapi.Actions{Send: func(string) {}}, 902, 1182, "")
	primary, ok := largestSurface(canvas, p.theme.Info)
	if !ok {
		t.Fatal("发送键应使用官方 button-info-fill 对应的 theme.Info")
	}
	if !closeEnough(primary.Bounds.Width, 34) || !closeEnough(primary.Bounds.Height, 34) {
		t.Fatalf("主按钮 = %.2fx%.2f，官方 34x34", primary.Bounds.Width, primary.Bounds.Height)
	}
	if primary.Radius != gfx.RadiusFull && !closeEnough(primary.Radius, 17) {
		t.Fatalf("主按钮圆角 = %.2f，官方 999px 全圆角应映射为 RadiusFull/17", primary.Radius)
	}
}

func TestComposerShowsHostTruthInOfficialSeats(t *testing.T) {
	p := &Plugin{}
	p.field.SetText("draft")
	data := pluginapi.Snapshot{Model: "deepseek-v4-pro", Permission: "ask", Status: "已连接"}
	canvas := renderComposer(t, p, data, pluginapi.Actions{Send: func(string) {}}, 902, 1182, "")
	var values []string
	for _, text := range canvas.Texts() {
		values = append(values, text.Value)
	}
	joined := strings.Join(values, "\n")
	for _, want := range []string{"deepseek-v4-pro", "逐次询问", "已连接"} {
		if !strings.Contains(joined, want) {
			t.Errorf("composer 没显示宿主真值 %q；实际文本：%q", want, joined)
		}
	}
	// 官方控件层级：permission 在左 tools；model、activity、primary 依次在右 trailing。
	var modelX, permissionX, statusX float32 = -1, -1, -1
	for _, text := range canvas.Texts() {
		switch text.Value {
		case "deepseek-v4-pro":
			modelX = text.Bounds.X
		case "逐次询问":
			permissionX = text.Bounds.X
		case "已连接":
			statusX = text.Bounds.X
		}
	}
	primary, ok := largestSurface(canvas, p.theme.Info)
	if !ok {
		t.Fatal("找不到 34px 主按钮")
	}
	primaryX := primary.Bounds.X
	if !(permissionX >= 0 && permissionX < modelX && modelX < statusX && statusX < primaryX) {
		t.Errorf("控件顺序应为 permission < model < status < primary；x=%v/%v/%v/%v",
			permissionX, modelX, statusX, primaryX)
	}
	card, _ := largestSurface(canvas, p.theme.InputMajor)
	// 官方 DOM 明确：modes 只有 permission/plan；model 在右侧 standardControls。
	if modelX < card.Bounds.X+card.Bounds.Width/2 {
		t.Errorf("model 必须落在右 trailing，而不是左 modes：modelX=%.2f cardCenter=%.2f",
			modelX, card.Bounds.X+card.Bounds.Width/2)
	}
}

func TestComposerUnknownFieldsAndUnsupportedControlsStayHidden(t *testing.T) {
	p := &Plugin{}
	canvas := renderComposer(t, p, pluginapi.Snapshot{}, pluginapi.Actions{}, 902, 1182, "")
	var values []string
	for _, text := range canvas.Texts() {
		values = append(values, text.Value)
	}
	joined := strings.Join(values, "\n")
	// 没有目录和 SelectModel/SetPermission/AddFiles 行为，就不能出现看似可点的
	// 默认模型、权限占位、命令/附件加号或下拉箭头。
	for _, banned := range []string{"选择模型", "默认模型", "权限未知", "附件", "+", "⌄"} {
		if strings.Contains(joined, banned) {
			t.Errorf("空契约下出现假控件/假真值 %q；实际文本：%q", banned, joined)
		}
	}
}

func TestSendButtonPreservesTrimAndClearBehavior(t *testing.T) {
	p := &Plugin{}
	p.field.SetText("  hello composer  ")
	var sent []string
	act := pluginapi.Actions{Send: func(text string) { sent = append(sent, text) }}
	renderComposer(t, p, pluginapi.Snapshot{}, act, 902, 1182, "send")
	if len(sent) != 1 || sent[0] != "hello composer" {
		t.Fatalf("发送按钮应 trim 后发送一次，得到 %#v", sent)
	}
	if got := p.field.Text(); got != "" {
		t.Fatalf("发送成功后草稿应清空，得到 %q", got)
	}
}

func TestStopButtonPreservesCancelBehavior(t *testing.T) {
	p := &Plugin{}
	cancelled, sent := 0, 0
	act := pluginapi.Actions{
		Cancel: func() { cancelled++ },
		Send:   func(string) { sent++ },
	}
	renderComposer(t, p, pluginapi.Snapshot{Busy: true}, act, 902, 1182, "stop")
	if cancelled != 1 || sent != 0 {
		t.Fatalf("忙碌时主按钮必须只取消一次：cancel=%d send=%d", cancelled, sent)
	}
}

func TestEscapeWithoutApprovalPreservesCancelRoute(t *testing.T) {
	p := &Plugin{}
	cancelled := 0
	p.lastData = pluginapi.Snapshot{}
	p.lastAct = pluginapi.Actions{Cancel: func() { cancelled++ }}
	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) {
		t.Fatal("无审批时 Esc 应走停止路由")
	}
	if cancelled != 1 {
		t.Fatalf("Esc 应取消一次，得到 %d", cancelled)
	}
}

func TestApprovalCardMatchesOfficialSurface(t *testing.T) {
	p := &Plugin{}
	ap := hostFrame()
	canvas := renderComposer(t, p, pluginapi.Snapshot{Cur: "s-001", Approval: &ap}, pluginapi.Actions{}, 902, 1182, "")
	foundOuter, foundFill := false, false
	var outer, composer gfx.Rectangle
	for _, r := range canvas.Rectangles() {
		if r.Color == p.theme.Warn && closeEnough(r.Radius, 20) && closeEnough(r.Bounds.Width, 680) {
			foundOuter, outer = true, r
		}
		if r.Color == p.theme.InputMajor && closeEnough(r.Radius, 19) && closeEnough(r.Bounds.Width, 678) {
			foundFill = true
		}
		if r.Color == p.theme.InputMajor && closeEnough(r.Radius, 28) && closeEnough(r.Bounds.Width, 712) {
			composer = r
		}
	}
	if !foundOuter || !foundFill {
		t.Fatalf("审批卡应为 Warn 1px 外框 + InputMajor 填充、radius-xl 20、内容宽 680；outer=%v fill=%v", foundOuter, foundFill)
	}
	// ApprovalPanel root bottom 12 + ConversationRoot composerStack gap 6 = 18。
	if gap := composer.Bounds.Y - (outer.Bounds.Y + outer.Bounds.Height); !closeEnough(gap, 18) {
		t.Errorf("审批卡到输入卡的净间距 = %.2f，官方 18", gap)
	}
	for _, text := range canvas.Texts() {
		switch text.Value {
		case ap.Title:
			if !closeEnough(text.Bounds.Height, 24) {
				t.Errorf("审批标题行高 = %.2f，官方 24", text.Bounds.Height)
			}
		case ap.Args:
			if !closeEnough(text.Bounds.Height, 20) {
				t.Errorf("审批参数行高 = %.2f，官方 20", text.Bounds.Height)
			}
		}
	}
}

func TestActivityLabelTruthAndEmptyStates(t *testing.T) {
	cases := []struct {
		name string
		data pluginapi.Snapshot
		want string
	}{
		{name: "未知整段隐藏", data: pluginapi.Snapshot{}, want: ""},
		{name: "宿主状态原样", data: pluginapi.Snapshot{Status: "  已连接  "}, want: "已连接"},
		{name: "Busy 推导运行", data: pluginapi.Snapshot{Busy: true}, want: "正在生成"},
		{name: "Stream 推导运行", data: pluginapi.Snapshot{Stream: "x"}, want: "正在生成"},
		{name: "真状态优先于推导", data: pluginapi.Snapshot{Status: "宿主状态", Busy: true}, want: "宿主状态"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := activityLabel(tc.data); got != tc.want {
				t.Fatalf("activityLabel(%+v) = %q，want %q", tc.data, got, tc.want)
			}
		})
	}
}

func TestApprovalButtonsPreserveOriginalIDsThroughLayout(t *testing.T) {
	// v0.6.0 起 ui.Button 没有显式交互 ID，按**文案**点（文案来自宿主的
	// ApprovalOption.Name，approval.go 直接用它当按钮文字）。
	cases := []struct {
		name  string
		label string
		want  string
	}{
		{name: "允许这一次", label: "允许这一次", want: "allow-once"},
		{name: "以后都允许", label: "这个会话里以后都允许", want: "allow-always"},
		{name: "拒绝", label: "拒绝", want: "reject-once"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &Plugin{}
			ap := hostFrame()
			var gotReq, gotOpt string
			calls := 0
			act := pluginapi.Actions{Approve: func(requestID, optionID string) {
				calls++
				gotReq, gotOpt = requestID, optionID
			}}
			renderComposerClickLabel(t, p, pluginapi.Snapshot{Approval: &ap}, act, 902, 1182, tc.label)
			if calls != 1 || gotReq != ap.RequestID || gotOpt != tc.want {
				t.Fatalf("审批按钮回传错误：calls=%d req=%q opt=%q", calls, gotReq, gotOpt)
			}
		})
	}
}
