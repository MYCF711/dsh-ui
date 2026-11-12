// slots_test.go —— 四个官方槽位的注册点 + 每个槽位的**三种降级**
// （未授权 / 未启用 / 数据缺失）。
//
// 这是本单最重要的文件：官方 rc.2 对这两个新包的验收标准就是"槽位在不在、
// 不该出现时会不会消失"。所以每个槽位都按同一张表逐个断言：
//
//	槽位            未授权                未启用                 数据缺失
//	① 子代理 footer  投影缺失               enabled=false          任务为空 / 全 deleted
//	② 模型菜单开关   投影缺失               子会话（两种判定）       modelSelection 缺失 → 禁用（仍渲染）
//	③ 停用确认       remote 未接线         没有停用请求            ——（确认框不需要投影）
//	④ 命令回执       ——                     命令名不是 team         node 为空
package dsagentteam

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// fakeRemote 是 Remote 的测试替身：记录调用、可注入错误、可阻塞。
type fakeRemote struct {
	mu sync.Mutex

	calls    []fakeCall
	err      error
	block    chan struct{}
	returned chan struct{}
	once     sync.Once
}

type fakeCall struct {
	sessionID string
	enabled   bool
}

func (f *fakeRemote) SetAgentTeamEnabled(ctx context.Context, sessionID string, enabled bool) error {
	f.mu.Lock()
	f.calls = append(f.calls, fakeCall{sessionID: sessionID, enabled: enabled})
	block, returned := f.block, f.returned
	f.mu.Unlock()
	if block != nil {
		<-block
	}
	if returned != nil {
		f.once.Do(func() { close(returned) })
	}
	f.mu.Lock()
	err := f.err
	f.mu.Unlock()
	return err
}

func (f *fakeRemote) callList() []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeCall(nil), f.calls...)
}

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("等不到：%s", what)
}

// newTestPlugin 造一个不经过 kernel 的插件实例。
func newTestPlugin(remote Remote) *Plugin {
	theme := dscore.Dark()
	p := &Plugin{theme: theme}
	p.team = NewStore(remote)
	return p
}

// renderSlot 只画某个槽位上的那一块（一个插件可以有四块画）。
func renderSlot(t *testing.T, p *Plugin, slot pluginapi.Slot) *ui.Canvas {
	t.Helper()
	var target *pluginapi.Painter
	for i, painter := range p.Panels() {
		if painter.Slot == slot {
			target = &p.Panels()[i]
			break
		}
	}
	if target == nil {
		t.Fatalf("槽位 %q 上没有注册任何画", slot)
	}
	return renderWith(t, p, target.Draw)
}

// renderWith 用给定的 Draw 画一帧。
func renderWith(t *testing.T, p *Plugin, draw func(f *flex.Frame, env *pluginapi.Env)) *ui.Canvas {
	t.Helper()
	theme := dscore.Dark()
	tree := &ui.Tree{}
	feat := tree.Feature("agent-team-test")
	canvas := &ui.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: feat, Canvas: canvas,
		Bounds: ui.Rect{Width: 1180, Height: 800}, MainWidth: 900,
	}
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(theme), env.Bounds, func(f *flex.Frame) {
		draw(f, env)
	})
	return canvas
}

// teamView 造一份"根会话、投影在、开关可用"的基线视图。
func teamView() View {
	return View{
		SessionID:       "sess-root",
		ProjectionKnown: true,
		Projection: TeamProjection{
			Enabled: true,
			Tasks: []TeamTask{
				{ID: "t1", Subject: "收集素材", Status: StatusCompleted, Ready: true, OwnerName: "qa"},
				{ID: "t2", Subject: "写实现", Status: StatusInProgress, Ready: true, OwnerID: "dev"},
				{ID: "t3", Subject: "等依赖", Status: StatusPending, Ready: false},
			},
		},
		ModelSelectionKnown: true,
	}
}

func hasText(canvas *ui.Canvas, want string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == want {
			return true
		}
	}
	return false
}

func anyTextContains(canvas *ui.Canvas, want string) bool {
	for _, text := range canvas.Texts() {
		if strings.Contains(text.Value, want) {
			return true
		}
	}
	return false
}

// TestOfficialSlotNames：四个槽位名逐字等于官方（写错一个字符父槽就认不到）。
func TestOfficialSlotNames(t *testing.T) {
	cases := map[pluginapi.Slot]string{
		SlotSubagentFooter:    "conversation.session.header.actions.subagent.footer",
		SlotInputModelOptions: "conversation.input.model.options",
		SlotChatCommandView:   "conversation.chat.commandview",
		SlotDisableOverlay:    "shell.overlay",
	}
	for got, want := range cases {
		if string(got) != want {
			t.Fatalf("槽位名应为 %q，得到 %q", want, got)
		}
	}
	if SlotDisableOverlay != pluginapi.SlotShellOverlay {
		t.Fatal("停用确认必须复用既有 shell.overlay 槽（官方 mount.ts:40-56）")
	}
}

// TestPanelsRegisterFourOfficialSlots：四个官方槽位各恰好一块画，槽位与官方 id/key 一一对应。
//
// 计数口径从"len(panels)==4"改成"按 (Slot, ID) 唯一 + 四个官方 ID 齐全"，因为
// **父容器**（模型菜单 / 命令节点渲染器 / 子代理目录）也会往同一个插件里加 painter：
//
//	shell.overlay 会合法地出现两块（`agent-team.disable` + 子代理目录的 `agent-team.catalog-panel`）
//	conversation.session.header.actions 是子代理目录触发器的父槽（GC 的容器 B）
//
// 所以这里钉的是"官方四个槽位贡献都在、且每块只有一个"，而不是"总共只有 4 块"。
func TestPanelsRegisterFourOfficialSlots(t *testing.T) {
	p := newTestPlugin(nil)
	panels := p.Panels()
	want := map[pluginapi.Slot]string{
		SlotSubagentFooter:    "agent-team.board",
		SlotInputModelOptions: "agent-team.toggle",
		SlotDisableOverlay:    "agent-team.disable",
		SlotChatCommandView:   "agent-team.command",
	}
	seenID := map[string]bool{}
	for _, painter := range panels {
		if painter.Draw == nil && painter.Children == nil {
			t.Fatalf("painter %q 既没有 Draw 也没有 Children = 挂上去什么都不做", painter.ID)
		}
		if painter.ID == "" {
			t.Fatalf("槽 %q 上有一块没有 ID 的画（日志/命中区前缀没法对照）", painter.Slot)
		}
		if seenID[painter.ID] {
			t.Fatalf("painter id %q 重复注册", painter.ID)
		}
		seenID[painter.ID] = true
	}
	for slot, id := range want {
		found := 0
		for _, painter := range panels {
			if painter.Slot != slot || painter.ID != id {
				continue
			}
			found++
			if painter.Draw == nil {
				t.Fatalf("槽 %q 的槽位贡献 %q 必须是 Draw（父容器才用 Children）", slot, id)
			}
		}
		if found != 1 {
			t.Fatalf("槽 %q 的贡献 %q 应恰好 1 块，得到 %d", slot, id, found)
		}
	}
	// 官方 id 字面量也钉住（日志/命中区前缀与官方对照用）。
	if SlotIDBoard != "agent-team" || SlotIDToggle != "agent-team" ||
		SlotIDDisable != "agent-team.disable" || CommandKey != "team" {
		t.Fatalf("官方 id/key 字面量对不上：%q %q %q %q", SlotIDBoard, SlotIDToggle, SlotIDDisable, CommandKey)
	}
}

// TestContainerPanelsAreMounted 断言两个父容器的 painter 真的挂在槽上。
//
// 判据：模型菜单在**官方座位**（`conversation.input.model`，本包画真实 trigger）
// 与**旧三栏座位**（`main.composer`，只画透明命中区）各一条；两个 composer 座位
// 每帧只派发其中一个槽，所以永远不会同帧双菜单。命令节点同样两个座位各一条。
func TestContainerPanelsAreMounted(t *testing.T) {
	p := newTestPlugin(nil)
	panels := p.Panels()
	want := []struct {
		id   string
		slot pluginapi.Slot
	}{
		{"agent-team.model-menu", uikernel.OffInputModel},
		{"agent-team.model-menu-legacy", pluginapi.SlotMainComposer},
		{"agent-team.command-node", uikernel.OffConvView},
		{"agent-team.command-node-legacy", pluginapi.SlotMainBody},
	}
	for _, tc := range want {
		var found *pluginapi.Painter
		for i, painter := range panels {
			if painter.ID == tc.id {
				found = &panels[i]
				break
			}
		}
		if found == nil {
			t.Fatalf("父容器 painter %q 没有注册（容器缺失 → 槽位永远画不出来）", tc.id)
		}
		if found.Slot != tc.slot {
			t.Fatalf("父容器 %q 应挂在 %q，实际 %q", tc.id, tc.slot, found.Slot)
		}
		if found.Children == nil {
			t.Fatalf("父容器 %q 必须有 Children（父容器的全部意义就是里面那次 drawSlot）", tc.id)
		}
	}
	// 两个座位**不得**挂在同一个模型槽上：那会让同一帧画两个菜单面。
	var modelPainters int
	for _, painter := range panels {
		if painter.Slot == uikernel.OffInputModel {
			modelPainters++
		}
	}
	if modelPainters != 1 {
		t.Fatalf("官方模型槽 %q 上有 %d 个 painter，want 1（双菜单来源）", uikernel.OffInputModel, modelPainters)
	}
}

// TestBoardDegradations 是槽 ① 的三种降级 + 一条"该画"的正例。
func TestBoardDegradations(t *testing.T) {
	cases := []struct {
		name  string
		view  View
		empty bool
	}{
		{"未授权：投影缺失", View{SessionID: "s"}, true},
		{"未启用：enabled=false（有任务也不画）", func() View {
			v := teamView()
			v.Projection.Enabled = false
			return v
		}(), true},
		{"数据缺失：任务为空", func() View {
			v := teamView()
			v.Projection.Tasks = nil
			return v
		}(), true},
		{"数据缺失：任务全 deleted", func() View {
			v := teamView()
			v.Projection.Tasks = []TeamTask{{ID: "t-old", Status: StatusDeleted}}
			return v
		}(), true},
		{"正例：enabled + 有任务", teamView(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newTestPlugin(nil)
			p.team.SetView(tc.view)
			canvas := renderSlot(t, p, SlotSubagentFooter)
			rects := canvas.Rectangles()
			if tc.empty {
				if len(rects) != 0 {
					t.Fatalf("该整块不渲染，却画了 %d 个矩形", len(rects))
				}
				return
			}
			if len(rects) == 0 {
				t.Fatal("该画任务板，却什么都没画")
			}
			if !hasText(canvas, zhTasks) {
				t.Fatalf("缺标题 %q；文本=%v", zhTasks, textValues(canvas))
			}
			if !anyTextContains(canvas, "1 已完成") {
				t.Fatalf("缺进度文案；文本=%v", textValues(canvas))
			}
		})
	}
}

// TestToggleDegradations 是槽 ② 的三种降级 + 一条正例。
func TestToggleDegradations(t *testing.T) {
	cases := []struct {
		name  string
		view  View
		empty bool
	}{
		{"未授权：投影缺失（mode===undefined）", View{SessionID: "s"}, true},
		{"未启用：子代理会话（subagent!==null）", func() View {
			v := teamView()
			v.SubagentChild = true
			return v
		}(), true},
		{"未启用：会话 origin===subagent", func() View {
			v := teamView()
			v.ListedAsSubagent = true
			return v
		}(), true},
		{"数据缺失：modelSelection 缺失仍渲染（官方禁用，不隐藏）", func() View {
			v := teamView()
			v.ModelSelectionKnown = false
			return v
		}(), false},
		{"正例：根会话 + 投影在", teamView(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newTestPlugin(nil)
			p.team.SetView(tc.view)
			canvas := renderSlot(t, p, SlotInputModelOptions)
			if tc.empty {
				if len(canvas.Rectangles()) != 0 || len(canvas.Texts()) != 0 {
					t.Fatalf("该整块不渲染，却画了 %d 矩形 / %d 文本",
						len(canvas.Rectangles()), len(canvas.Texts()))
				}
				return
			}
			if !hasText(canvas, zhModeLabel) {
				t.Fatalf("缺开关文案 %q；文本=%v", zhModeLabel, textValues(canvas))
			}
			// 开关本体必须有命中区（否则用户点不了）。
			if !hasSwitchHit(canvas) {
				t.Fatalf("缺开关命中区：hits=%v", hitLabels(canvas))
			}
		})
	}
}

// TestConfirmDegradations 是槽 ③ 的降级 + 正例。
func TestConfirmDegradations(t *testing.T) {
	// 没有停用请求 → 整块不渲染（官方 request === null → null）。
	p := newTestPlugin(nil)
	p.team.SetView(teamView())
	if rects := renderSlot(t, p, SlotDisableOverlay).Rectangles(); len(rects) != 0 {
		t.Fatalf("没有停用请求时不该画任何东西，画了 %d 个矩形", len(rects))
	}

	// 有请求 → 画确认框（标题 / 两段说明 / 两个按钮）。
	p.team.RequestDisable("sess-root")
	canvas := renderSlot(t, p, SlotDisableOverlay)
	if len(canvas.Rectangles()) == 0 {
		t.Fatal("有停用请求时应当画确认框")
	}
	for _, want := range []string{zhDisableTitle, zhDisableDesc, zhDisableWarning, zhDisableCancel, zhDisableConfirm} {
		if !hasText(canvas, want) {
			t.Fatalf("确认框缺 %q；文本=%v", want, textValues(canvas))
		}
	}

	// 未接线（remote=nil）：点确认必须**如实失败**，不能假装成功。
	p.team.ConfirmDisable()
	if got := p.team.Confirm(); got.Kind != ConfirmFailed || got.Message == "" {
		t.Fatalf("未接线时确认应停在 failed 并有说明，得到 %+v", got)
	}
	if p.team.DisableRequest() == nil {
		t.Fatal("失败时不能关掉确认框（官方：错误一直可见到重试或取消）")
	}
}

// TestCommandDegradations 是槽 ④ 的降级 + 正例。
func TestCommandDegradations(t *testing.T) {
	p := newTestPlugin(nil)
	// 数据缺失：没有节点。
	if rects := renderSlot(t, p, SlotChatCommandView).Rectangles(); len(rects) != 0 {
		t.Fatalf("没有命令节点时不该画任何东西，画了 %d 个矩形", len(rects))
	}
	// 未启用：命令名不是 team（官方 key 过滤）。
	p.team.SetCommand(&CommandNode{Command: "compact", Outcome: &CommandOutcome{Kind: "success", Text: "ok"}})
	if rects := renderSlot(t, p, SlotChatCommandView).Rectangles(); len(rects) != 0 {
		t.Fatalf("非 team 命令不该画，画了 %d 个矩形", len(rects))
	}
	// 正例：team 命令，运行中。
	p.team.SetCommand(&CommandNode{Command: CommandKey, Outcome: nil})
	canvas := renderSlot(t, p, SlotChatCommandView)
	if len(canvas.Rectangles()) == 0 {
		t.Fatal("team 命令回执应当有内容")
	}
	if !hasText(canvas, zhCommandTitle) || !hasText(canvas, zhCommandRunning) {
		t.Fatalf("缺标题或运行中文案；文本=%v", textValues(canvas))
	}
}

// TestStopGraceful：四块画都不该 panic（槽位空数据时的最基本要求）。
func TestStopGraceful(t *testing.T) {
	p := newTestPlugin(nil)
	p.team.SetView(View{})
	for _, painter := range p.Panels() {
		_ = renderWith(t, p, painter.Draw)
	}
}

// ── 小工具 ──

func textValues(canvas *ui.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

func hitLabels(canvas *ui.Canvas) []string {
	out := make([]string, 0, len(canvas.Hits()))
	for _, hit := range canvas.Hits() {
		out = append(out, hit.Label)
	}
	return out
}

func hitContains(canvas *ui.Canvas, fragment string) (ui.Hit, bool) {
	for _, hit := range canvas.Hits() {
		if strings.Contains(hit.Value, fragment) {
			return hit, true
		}
	}
	return ui.Hit{}, false
}

// hasSwitchHit 判断画面里有没有 Switch。
//
// 注：viva 的 `Switch` 不给 label（只给 id），命中区的 Label 是框架默认的 "switch"，
// 所以判据是"存在一个 switch 命中区"，而不是按文案找。
func hasSwitchHit(canvas *ui.Canvas) bool {
	for _, hit := range canvas.Hits() {
		if hit.Label == "switch" {
			return true
		}
	}
	return false
}
