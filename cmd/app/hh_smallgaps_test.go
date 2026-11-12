package main

// hh_smallgaps_test.go —— HH 单五件小缺口的**行为断言**。
//
// 为什么不能只断言"画出来了"：这五件每一件的坏法都是"界面看起来正常，但
// 行为是错的/假的"——
//
//	① 设置行       两个部件各画一行：文字都在、位置却在两行（视觉错层）
//	② 立即重连     warning 胶囊是 `<button>`，点了什么都不发生（假动作）
//	③ 150ms 淡出   状态一切就换：没有官方那 150ms 的离场过渡
//	④ composer 文案 同一个窗口里说两遍连接状态（重复措辞）
//	⑤ Esc 优先级   turn 在跑时 Esc 先被 composer 吃掉，弹窗关不掉
//
// 所以这里每一条都走**真 flex 布局 + 真内核 + 真按键路由**，断言落在
// 几何/调用/文本上，不落在"函数被调用过"。

import (
	"strings"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscomposer"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dssettings"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar"
	"github.com/MYCF711/dsh-ui/plugins/dsstatusbar"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// hhProduct 是 HH 单用的产品形状：真内核 + 真插件 + 官方骨架。
type hhProduct struct {
	kernel *uikernel.Kernel
	theme  *pluginapi.Theme
	tree   *ui.Tree
	feat   ui.Scope
	opts   uikernel.ShellOptions
	// measure 收骨架报上来的几何（行/列盒子），断言"同一行"就靠它。
	//
	// 存的是**盒子**不是矩形：`flex.Box.Rect()` 要等这一帧布局跑完才有效
	// （在 measure 回调里取会拿到全 0），所以绘制结束后统一解一次。
	measure map[string]flex.Box
	// settings 是设置插件实例（触发按钮在它身上）。
	settings *dssettings.Plugin
	// statusbar 是状态栏插件实例（连接指示 + Activity 弹窗在它身上）。
	statusbar *dsstatusbar.Plugin
}

// newHHProduct 装产品插件集：官方骨架 + 设置 + 状态栏 + composer。
//
// launcherHeight > 0 时额外挂一个**假的** `settings.launcher` 占用方（模拟
// dsnotifications 的账号启动器：官方这一行左边那一格是 `flex:1` 的启动器，
// 不是设置齿轮按钮）。launcherHeight == 0 时走官方 fallback（设置齿轮按钮）。
func newHHProduct(t *testing.T, launcherHeight float32) *hhProduct {
	t.Helper()
	tier := uikernel.NewShellTier()
	tier.Set(true)
	p := &hhProduct{
		kernel:    uikernel.New("hh-smallgaps"),
		tree:      &ui.Tree{},
		measure:   map[string]flex.Box{},
		settings:  &dssettings.Plugin{},
		statusbar: &dsstatusbar.Plugin{},
	}
	p.kernel.Root().Provide(uikernel.KeyShellTier, tier)
	plugins := []pluginapi.Plugin{
		&dscore.Plugin{Dark: true},
		p.settings,
		p.statusbar,
		&dscomposer.Plugin{},
	}
	if launcherHeight > 0 {
		plugins = append(plugins, &hhLauncherProbe{height: launcherHeight})
	}
	if err := p.kernel.Load(plugins...); err != nil {
		t.Fatalf("装配产品插件集失败：%v", err)
	}
	p.theme = p.kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.feat = p.tree.Feature("hh-smallgaps")
	m := &model{kernel: p.kernel, cur: "s-hh"}
	p.opts = shellOptionsFor(m, p.kernel, 1182, 832)
	return p
}

// hhLauncherProbe 是假的账号启动器：官方 triggerRow 里 `flex:1` 的那一格。
//
// 只画一个定高盒子（官方 AccountMenu `.trigger{height:44px;flex:1}`），
// 用来量"设置按钮与连接指示是不是同一行"。
type hhLauncherProbe struct{ height float32 }

func (p *hhLauncherProbe) ID() string                       { return "hh-launcher-probe" }
func (p *hhLauncherProbe) Requires() []string               { return []string{"ui.theme"} }
func (p *hhLauncherProbe) Provides() []string               { return nil }
func (p *hhLauncherProbe) Register(pluginapi.Context) error { return nil }
func (p *hhLauncherProbe) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{
		ID: "account-launcher.row", Slot: pluginapi.Slot("sidebar.settings"), Order: 60,
		Draw: func(f *flex.Frame, env *pluginapi.Env) {
			f.Row().FillWidth().Height(p.height).Children(func() {
				f.Text("已登录").FontSize(14).LineHeight(22)
			})
		},
	}}
}

// paint 画一帧产品路径（官方骨架 + 真 flex + 真 canvas）。
func (p *hhProduct) paint(t *testing.T, snap pluginapi.Snapshot, act pluginapi.Actions, click string) *ui.Canvas {
	t.Helper()
	canvas := &ui.Canvas{}
	canvas.SetTree(p.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: p.theme, Data: snap, Tree: p.tree, Feat: p.feat, Canvas: canvas,
		Bounds:    ui.Rect{Width: 1182, Height: 832},
		MainWidth: 902,
		Geometry:  pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902},
		Has:       func(slot pluginapi.Slot) bool { return p.kernel.Has(slot) },
		Act:       act,
	}
	if click != "" {
		canvas.BeginEventPass(p.tree.ID(p.feat, click))
		defer canvas.EndEventPass()
	}
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(p.theme), env.Bounds, func(f *flex.Frame) {
		uikernel.FirstScreenWithMeasure(f, env, p.opts, func(slot pluginapi.Slot) {
			p.kernel.DrawSlot(f, slot, env)
		}, func(name string, box flex.Box) {
			p.measure[name] = box
		})
	})
	for _, painter := range p.kernel.AllPainters() {
		if painter.AfterDraw != nil {
			painter.AfterDraw(canvas, env)
		}
	}
	return canvas
}

// hhTexts 收画布上的文字。
func hhTexts(canvas *ui.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

// hhFindText 找一条文字的矩形（找不到返回 ok=false）。
func hhFindText(canvas *ui.Canvas, value string) (ui.Rect, bool) {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text.Bounds, true
		}
	}
	return ui.Rect{}, false
}

// hhHasText 看画布上有没有这条文字（含前缀匹配：省略号/后缀不影响）。
func hhHasText(canvas *ui.Canvas, want string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == want || strings.HasPrefix(text.Value, want) {
			return true
		}
	}
	return false
}

// TestHHTriggerRowPutsSettingsAndConnectionOnOneLine 是第 1 件的**几何**断言。
//
// 官方 `SettingsRoot.triggerRow` 是一行 flex：
//
//	[设置按钮(flex:1)] [连接指示(flex:none)] [更新指示]
//
// 判据（三条都要成立，少一条就不叫"同一行"）：
//
//	① 行高 = max(子项高) = 42 / 44 —— **不是** 42+28=70（两行叠起来的高度）；
//	② 两个子项的**垂直中心**是同一个 Y（`align-items:center`）；
//	③ 连接指示在设置按钮**右侧**（`flex:1` 把它推过去）。
func TestHHTriggerRowPutsSettingsAndConnectionOnOneLine(t *testing.T) {
	for _, tc := range []struct {
		name           string
		launcherHeight float32
		leftLabel      string // 左边那一格的字（启动器 / 设置齿轮按钮）
		wantRowHeight  float32
	}{
		// 官方 fallback：没有账号启动器时画设置齿轮按钮（`.trigger{height:42px}`）。
		{name: "设置按钮 fallback", launcherHeight: 0, leftLabel: "设置", wantRowHeight: 42},
		// 产品形状：账号启动器占 `settings.launcher`（`.trigger{height:44px}`）。
		{name: "账号启动器", launcherHeight: 44, leftLabel: "已登录", wantRowHeight: 44},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newHHProduct(t, tc.launcherHeight)
			canvas := p.paint(t, pluginapi.Snapshot{Cur: "s-hh", Link: pluginapi.LinkConnected}, pluginapi.Actions{}, "")

			rowBox, ok := p.measure["SettingsRoot.triggerRow"]
			if !ok {
				t.Fatalf("骨架没报出设置行几何；量到的盒子：%v", p.measure)
			}
			row := rowBox.Rect()
			if row.Width == 0 || row.Height == 0 {
				t.Fatalf("设置行几何是空的：%+v（布局没跑完就取了矩形？）", row)
			}
			if row.Height != tc.wantRowHeight {
				t.Fatalf("设置行高 = %.2f，want %.2f（%.2f = 两行叠起来，正是第 1 件要拆的错层）",
					row.Height, tc.wantRowHeight, float32(42+28))
			}
			left, ok := hhFindText(canvas, tc.leftLabel)
			if !ok {
				t.Fatalf("设置行里没有 %q；texts=%v", tc.leftLabel, hhTexts(canvas))
			}
			conn, ok := hhFindText(canvas, dsstatusbar.OfficialConnectedLabel)
			if !ok {
				t.Fatalf("设置行里没有连接指示 %q；texts=%v",
					dsstatusbar.OfficialConnectedLabel, hhTexts(canvas))
			}
			// ② 垂直中心同一个 Y（同一行、都居中）。
			leftCenter := left.Y + left.Height/2
			connCenter := conn.Y + conn.Height/2
			if leftCenter != connCenter {
				t.Fatalf("%q 中心 Y = %.2f，连接指示中心 Y = %.2f（不在同一行）",
					tc.leftLabel, leftCenter, connCenter)
			}
			// 两个子项都必须落在这一行里（不是"行在，子项跑到别处"）。
			for _, box := range []ui.Rect{left, conn} {
				if box.Y < row.Y || box.Y+box.Height > row.Y+row.Height {
					t.Fatalf("子项 %+v 越出设置行 %+v", box, row)
				}
			}
			// ③ 连接指示在右（`flex:1` 的左边那一格把它推过去）。
			if conn.X <= left.X {
				t.Fatalf("连接指示 X = %.2f，%q X = %.2f（应在右侧）", conn.X, tc.leftLabel, left.X)
			}
			// 行右缘 = 侧栏内容右缘（官方 `width:calc(100% + 4px);margin:4px -2px`）。
			if conn.X+conn.Width > row.X+row.Width+0.01 {
				t.Fatalf("连接指示越出行右缘：connRight=%.2f rowRight=%.2f", conn.X+conn.Width, row.X+row.Width)
			}
		})
	}
}

// TestHHTriggerRowRendersInLegacySidebarToo 是第 1 件的**路径覆盖**断言：
// 产品默认（dsshell 旧三栏 / uikernel 兼容桥）也必须画出这一行。
//
// 为什么单独钉：官方骨架与旧三栏是两条绘制路径，"只在官方骨架里改好"是
// 这单最容易留下的半成品——真机默认跑的恰恰是旧三栏那条。
func TestHHTriggerRowRendersInLegacySidebarToo(t *testing.T) {
	tier := uikernel.NewShellTier()
	tier.Set(true)
	kernel := uikernel.New("hh-legacy-row")
	kernel.Root().Provide(uikernel.KeyShellTier, tier)
	status := &dsstatusbar.Plugin{}
	settings := &dssettings.Plugin{}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, settings, status); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &ui.Tree{}
	feat := tree.Feature("hh-legacy-row")
	canvas := &ui.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: theme, Data: pluginapi.Snapshot{Cur: "s", Link: pluginapi.LinkConnected},
		Tree: tree, Feat: feat, Canvas: canvas,
		Bounds:    ui.Rect{Width: 1182, Height: 832},
		MainWidth: 902,
		Geometry:  pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902},
		Has:       func(slot pluginapi.Slot) bool { return kernel.Has(slot) },
	}
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(theme), env.Bounds, func(f *flex.Frame) {
		// 兼容桥档位：整列走旧三栏（`drawLegacySidebar`），设置行必须照样画出来。
		uikernel.FirstScreenWithMeasure(f, env,
			uikernel.ShellOptions{Phase: uikernel.PhaseActive, SidebarPreferred: 280, BridgeLegacy: true},
			func(slot pluginapi.Slot) { kernel.DrawSlot(f, slot, env) }, nil)
	})
	if !hhHasText(canvas, dsstatusbar.OfficialConnectedLabel) {
		t.Fatalf("旧三栏路径没画连接指示（真机默认路径会空白）：%v", hhTexts(canvas))
	}
}

// TestHHEscClosesStatsDialogBeforeStoppingTurn 是第 5 件的**按键路由**断言。
//
// 装配顺序决定按键优先级（`Kernel.HandleKey` 第一个返回 true 的赢）：
//
//	dssettings → dsstatusbar → … → dscomposer
//
// 弹窗开着时 Esc 必须关弹窗、**不许**停 turn；弹窗关着时 Esc 才落到 composer
// 的"停止当前 turn"。这两半都要断言——只断言"弹窗关了"的话，"顺手把 turn
// 也停了"这种情况会漏过去。
func TestHHEscClosesStatsDialogBeforeStoppingTurn(t *testing.T) {
	p := newHHProduct(t, 0)
	cancelled := 0
	act := pluginapi.Actions{
		Send:   func(string) {},
		Cancel: func() { cancelled++ },
	}
	stats := &apiclient.SessionStats{
		Turns: 2, Steps: 3,
		LLMMs: 1234, ToolMs: 567, TTFTMs: 400, TTFTSteps: 2,
		DecodeMs: 800, DecodeTokens: 160,
	}
	snap := pluginapi.Snapshot{
		Cur: "s-hh", Status: "已加载 3 条（总 3）", Busy: true,
		Link: pluginapi.LinkConnected, SessionStats: stats,
	}
	// 两帧把弹窗点开：第一帧缓存活动胶囊的锚点矩形，第二帧才点得中。
	p.paint(t, snap, act, "")
	opened := p.paint(t, snap, act, "activity-pill")
	if !hhHasText(opened, "会话统计") {
		t.Fatalf("点活动胶囊没有打开弹窗；texts=%v", hhTexts(opened))
	}

	// Esc：弹窗开着 → 关弹窗，turn 不许停。
	if !p.kernel.HandleKey(ui.KeyEvent{Key: ui.KeyEscape}) {
		t.Fatal("弹窗开着时 Esc 没有插件接（返回 false）")
	}
	if cancelled != 0 {
		t.Fatalf("弹窗开着时 Esc 停了 turn（Cancel 被调 %d 次）——装配顺序还是 composer 在前？", cancelled)
	}
	closed := p.paint(t, snap, act, "")
	if hhHasText(closed, "会话统计") {
		t.Fatalf("Esc 后弹窗还在：%v", hhTexts(closed))
	}

	// 再按一次 Esc：弹窗已关 → 落到 composer 的"停止当前 turn"。
	if !p.kernel.HandleKey(ui.KeyEvent{Key: ui.KeyEscape}) {
		t.Fatal("弹窗关着时 Esc 没有插件接（composer 的停止键丢了）")
	}
	if cancelled != 1 {
		t.Fatalf("弹窗关着时 Esc 应停 turn（Cancel 1 次），实际 %d 次", cancelled)
	}
}

// TestHHComposerHasNoConnectionWording 是第 4 件的**文本**断言。
//
// 连接层的可见措辞只属于官方座位（`ConnectionIndicator`，在侧栏设置行）。
// composer 那一条状态位只放**动作状态**（"已加载 N 条"），不许把同一个
// 结论再说一遍——否则同一个窗口里两处连接措辞。
func TestHHComposerHasNoConnectionWording(t *testing.T) {
	banned := []string{
		dsstatusbar.OfficialConnectedLabel,    // 连接成功
		dsstatusbar.OfficialDisconnectedLabel, // 连接异常，刷新重试
		dsstatusbar.OfficialConnectingLabel,   // 重新连接中
		"已连接",                                 // 旧壳那句"编出来的已连接"
	}
	// 产品的两种真实状态：连着 / 断开。壳写的状态串是动作状态（已加载 N 条）。
	for _, tc := range []struct {
		name   string
		link   pluginapi.LinkState
		status string
	}{
		{"连着", pluginapi.LinkConnected, "已加载 3 条（总 3）"},
		{"断开", pluginapi.LinkDown, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			composer := &dscomposer.Plugin{}
			kernel := uikernel.New("hh-composer")
			if err := kernel.Load(&dscore.Plugin{Dark: true}, composer); err != nil {
				t.Fatalf("装配失败：%v", err)
			}
			theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
			tree := &ui.Tree{}
			feat := tree.Feature("hh-composer")
			canvas := &ui.Canvas{}
			canvas.SetTree(tree)
			canvas.SetMeasurer(func(value string, size, weight float32) float32 {
				return float32(len([]rune(value))) * size * 0.55
			})
			env := &pluginapi.Env{
				Theme: theme,
				Data:  pluginapi.Snapshot{Cur: "s", Link: tc.link, Status: tc.status},
				Act:   pluginapi.Actions{Send: func(string) {}},
				Tree:  tree, Feat: feat, Canvas: canvas,
				Bounds:    ui.Rect{Width: 902, Height: 400},
				MainWidth: 902,
				Geometry:  pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902},
				Has:       func(slot pluginapi.Slot) bool { return kernel.Has(slot) },
			}
			var view flex.View
			view.Draw(canvas, dscore.VivaTheme(theme), env.Bounds, func(f *flex.Frame) {
				f.Column().Width(878).Children(func() { kernel.DrawSlot(f, uikernel.OffConvComposer, env) })
			})
			joined := strings.Join(hhTexts(canvas), " ")
			for _, word := range banned {
				if strings.Contains(joined, word) {
					t.Fatalf("composer 出现连接层措辞 %q（官方座位已经说过一遍）；texts=%q",
						word, joined)
				}
			}
			// 反向：动作状态照样要显示（不是"把状态位整条删掉"）。
			if tc.status != "" && !strings.Contains(joined, tc.status) {
				t.Fatalf("composer 没显示动作状态 %q；texts=%q", tc.status, joined)
			}
		})
	}
}

// TestHHReconnectCancelsAndResubscribes 是第 2 件的**真动作**断言。
//
// `Actions.Reconnect` 的契约（pluginapi.Actions.Reconnect 注释）：
//
//	① 取消当前订阅（旧 goroutine 从退避里退出）
//	② 立刻重新订阅（不睡退避）
//	③ 新尝试失败时连接事实变 LinkDown（不是假装还连着）
//
// 这里连一个**连不上**的 Base：重连必然失败，正好能验 ③；同时验 ①②
// （旧 cancel 被调、subs 里换成了新的那一个）。
func TestHHReconnectCancelsAndResubscribes(t *testing.T) {
	m := &model{
		mux: apiclient.NewMux("http://127.0.0.1:1"),
		cur: "s-reconnect",
		// 当前会话已收到的水位（`subscribe` 用它当重订水位）。
		lastSeq: 7,
	}
	cancelled := 0
	old := &muxSub{cancel: func() { cancelled++ }, lastSeq: 7}
	m.subs = map[string]*muxSub{"s-reconnect": old}

	m.reconnectNow()
	if cancelled != 1 {
		t.Fatalf("重连没有取消旧订阅（cancel 调用 %d 次）", cancelled)
	}
	m.mu.Lock()
	next, ok := m.subs["s-reconnect"]
	m.mu.Unlock()
	if !ok || next == nil {
		t.Fatal("重连后没有立刻重新订阅（subs 里是空的）")
	}
	if next == old {
		t.Fatal("重连后还是旧的那一个订阅")
	}
	if next.lastSeq != 7 {
		t.Fatalf("重订水位 = %d，want 7（已收到的帧不重放）", next.lastSeq)
	}

	// ③ 真失败事实：连不上的 Base 让 runSubscription 写 LinkDown。
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if hhLinkState(m) == pluginapi.LinkDown {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := hhLinkState(m); got != pluginapi.LinkDown {
		t.Fatalf("重连失败后连接事实 = %v，want LinkDown", got)
	}
	// 收尾：取消订阅循环（不让它带着 1s 退避继续跑）。
	if next.cancel != nil {
		next.cancel()
	}
}

// hhLinkState 在锁里读连接三事实（与壳的写入点同一把锁，-race 门禁）。
func hhLinkState(m *model) pluginapi.LinkState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return linkStateOf(wsUp, wsDown, wsConnecting)
}

// TestHHAssemblyOrderPutsDialogsBeforeComposer 是第 5 件的**装配顺序**断言。
//
// `Kernel.HandleKey` 按 `k.order`（装配顺序）转发，第一个返回 true 的赢。
// 这条顺序有两个坑，都真机上踩过：
//
//	① 用 prepend 把设置/状态栏提到最前**没有用**：`Load` 按轮次拓扑装配，
//	   排在 ds-core 前面时"依赖还没就位"会被推到最后一轮——顺序反而最靠后。
//	② 顺序错了的后果不是崩溃，是"跑 turn 时 Esc 只停 turn、弹窗关不掉"。
//
// 所以这里直接断言 `k.order` 的四条相对位置。
func TestHHAssemblyOrderPutsDialogsBeforeComposer(t *testing.T) {
	kernel := uikernel.New("hh-order")
	plugins := []pluginapi.Plugin{
		&dscore.Plugin{Dark: true},
		&dsshell.Plugin{},
		&dssidebar.Plugin{},
		&dsconversation.Plugin{},
		&dscomposer.Plugin{},
		&dssettings.Plugin{},
		&dsstatusbar.Plugin{},
	}
	if err := kernel.Load(orderPlugins(plugins)...); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	pos := map[string]int{}
	for i, id := range kernel.Order() {
		pos[id] = i
	}
	if pos["ds-core"] != 0 {
		t.Fatalf("ds-core 必须是第 0 个（唯一无依赖的插件），实际顺序：%v", kernel.Order())
	}
	if !(pos["ds-settings"] < pos["ds-composer"]) {
		t.Fatalf("ds-settings 必须排在 ds-composer 之前（否则 Esc 先停 turn）：%v", kernel.Order())
	}
	if !(pos["ds-statusbar"] < pos["ds-composer"]) {
		t.Fatalf("ds-statusbar 必须排在 ds-composer 之前（否则 Esc 先停 turn）：%v", kernel.Order())
	}
	// 两个模态都开着时 Esc 先关设置（用户最后打开的那个是状态栏弹窗，
	// 但设置是"整块模态"，官方语义先关它；顺序固定下来，避免"看运气"）。
	if !(pos["ds-settings"] < pos["ds-statusbar"]) {
		t.Fatalf("ds-settings 应排在 ds-statusbar 之前：%v", kernel.Order())
	}
}
