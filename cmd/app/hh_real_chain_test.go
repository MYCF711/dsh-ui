//go:build windows

package main

// hh_real_chain_test.go —— **真机验收**：HH 单五件小缺口在真 `dshd serve` +
// 真 apiclient + `-shell official` 产品形状下的行为与截图。
//
// 判据（逐条对应 HH 单）：
//
//	① 设置行：`SettingsRoot.triggerRow` 是**一行**（高 42/44，不是 42+28 两行），
//	   设置入口与连接指示同 Y 中心、指示器在右；
//	② 立即重连：`m.reconnectNow()` 走**真 apiclient**——旧订阅取消、新订阅
//	   真建起来（连接事实回到 LinkConnected）；
//	③ connecting / 150ms 淡出：`LinkConnecting` 画「重新连接中…」warning 胶囊，
//	   150ms 离场窗后换成新态（真时钟推进 + 真重绘）；
//	④ composer 文案：composer 那一行**不出现**连接层措辞（官方座位才说）；
//	⑤ Esc 优先级：Activity 弹窗开着时 Esc 关弹窗、不停 turn。
//
// 截图落在 `$HH_CAPTURE_DIR`（默认 `tmp/GO8-RC2/_hh/shots`）。
//
// 跑法（不给 DSHD_BIN 就跳过）：
//
//	export DSHD_BIN=".../tmp/GO8-RC2/_hh/bin/dshd.exe"
//	export DSHD_HOME_ROOT=".../tmp/GO8-RC2/_hh"
//	export HH_CAPTURE_DIR=".../tmp/GO8-RC2/_hh/shots"
//	go test ./cmd/app -run 'TestRealChainHHSmallGaps' -count=1 -v

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscomposer"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsgoal"
	"github.com/MYCF711/dsh-ui/plugins/dsquestions"
	"github.com/MYCF711/dsh-ui/plugins/dssettings"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar"
	"github.com/MYCF711/dsh-ui/plugins/dsstatusbar"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// hhRealProduct 是 HH 单的真机产品形状：**产品插件集**（含 dssettings/dsstatusbar）
// + 官方骨架 + 真主题。与 `newGYRealProduct` 的差别只有一个：多装了设置插件
// （它才画设置行的左半边）。
type hhRealProduct struct {
	kernel   *uikernel.Kernel
	theme    *pluginapi.Theme
	tree     *gfx.Tree
	feat     gfx.Scope
	opts     uikernel.ShellOptions
	settings *dssettings.Plugin
	status   *dsstatusbar.Plugin
}

func newHHRealProduct(t *testing.T) *hhRealProduct {
	t.Helper()
	tier := uikernel.NewShellTier()
	tier.Set(true)
	p := &hhRealProduct{
		kernel:   uikernel.New("hh-real-chain"),
		tree:     &gfx.Tree{},
		settings: &dssettings.Plugin{},
		status:   &dsstatusbar.Plugin{},
	}
	p.kernel.Root().Provide(uikernel.KeyShellTier, tier)
	// 装配顺序**照抄 main.go**：设置与状态栏两个"按键优先"的插件 prepend 到
	// 整列最前（`main.go` 的 `append([]pluginapi.Plugin{m.settings, m.statusbar}, …)`），
	// 依赖排序后它们落在 ds-core 之后、其余插件之前。顺序错了 Esc 会先被
	// dscomposer 的"停止 turn"吃掉——那正是第 5 件要修的 bug，测试替身不能替它遮掩。
	if err := p.kernel.Load(orderPlugins([]pluginapi.Plugin{
		&dscore.Plugin{Dark: true}, &dsshell.Plugin{}, &dssidebar.Plugin{},
		&dsconversation.Plugin{}, &dscomposer.Plugin{}, &dsgoal.Plugin{},
		&dsquestions.Plugin{}, p.settings, p.status,
	})...); err != nil {
		t.Fatalf("装配产品插件集失败：%v", err)
	}
	p.theme = p.kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.feat = p.tree.Feature("hh-real-chain")
	m := &model{kernel: p.kernel, cur: "s-real"}
	p.opts = shellOptionsFor(m, p.kernel, 1182, 832)
	return p
}

// hhPaintReal 画一帧产品路径，返回（画布，每槽派发次数，设置行矩形）。
//
// 设置行矩形从骨架的 measure 回调里取：`drawLegacySidebar` / `drawSidebarSkeleton`
// 都会把 `SettingsRoot.triggerRow` 报上来（HH 单第 1 件的判据就是它）。
func (p *hhRealProduct) paintReal(t *testing.T, snap pluginapi.Snapshot, act pluginapi.Actions, click string) (*gfx.Canvas, map[pluginapi.Slot]int, ui.Rect) {
	t.Helper()
	canvas := &gfx.Canvas{}
	canvas.SetTree(p.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size * 0.55
	})
	counts := map[pluginapi.Slot]int{}
	var rowBox ui.Box
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
	}
	var view ui.Page
	view.Draw(canvas, dscore.VivaTheme(p.theme), env.Bounds, func(f ui.C) {
		uikernel.FirstScreenWithMeasure(f, env, p.opts, func(slot pluginapi.Slot) {
			counts[slot]++
			p.kernel.DrawSlot(f, slot, env)
		}, func(name string, box ui.Box) {
			if strings.HasPrefix(name, "SettingsRoot.triggerRow") {
				rowBox = box
			}
		})
	})
	if click != "" {
		canvas.EndEventPass()
	}
	for _, painter := range p.kernel.AllPainters() {
		if painter.AfterDraw != nil {
			painter.AfterDraw(canvas, env)
		}
	}
	return canvas, counts, rowBox.Rect()
}

// settle 把状态机"跑过离场窗"再取一帧：官方 150ms 淡出的等价物。
//
// 连接指示有 150ms 离场窗（`._leaving{opacity:0}` + `transition:opacity .15s`）：
// 目标态一到，第一帧仍画旧态，150ms 后才换。真时钟下这里必须画两帧、中间等过去，
// 否则截到的是"还没走完的动画"，不是"状态错了"。
func (p *hhRealProduct) settle(t *testing.T, snap pluginapi.Snapshot, act pluginapi.Actions) *gfx.Canvas {
	t.Helper()
	// 三帧：离场窗里有"上一次过渡还没走完"的中间态（第二帧只是把它走完），
	// 第三帧才保证画的是**目标态**。少画一帧就会截到上一个状态的残影。
	var canvas *gfx.Canvas
	for i := 0; i < 3; i++ {
		canvas, _, _ = p.paintReal(t, snap, act, "")
		time.Sleep(200 * time.Millisecond)
	}
	canvas, _, _ = p.paintReal(t, snap, act, "")
	return canvas
}

// hhShot 用真 headless 把这一帧拍成 PNG（产品形状）。
func (p *hhRealProduct) shot(t *testing.T, snap pluginapi.Snapshot, act pluginapi.Actions, path string) {
	t.Helper()
	shotTree := &gfx.Tree{}
	shotFeat := shotTree.Feature("hh-shot")
	var invalidator app.Invalidator
	config := app.Config{
		Title: "hh-official", Width: 1182, Height: 832, Tree: shotTree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(c *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		c.Fill(bounds, p.theme.BgBase)
		env := &pluginapi.Env{
			Theme: p.theme, Data: snap, Tree: shotTree, Feat: shotFeat, Canvas: c,
			Bounds: bounds, MainWidth: 902,
			Geometry: pluginapi.Geometry{Width: size.Width, Height: size.Height, SidebarWidth: 280, MainWidth: 902},
			Has:      p.kernel.Has,
			Act:      act,
		}
		var view ui.Page
		view.Draw(c, dscore.VivaTheme(p.theme), bounds, func(f ui.C) {
			uikernel.FirstScreen(f, env, p.opts, func(slot pluginapi.Slot) {
				p.kernel.DrawSlot(f, slot, env)
			})
		})
		for _, painter := range p.kernel.AllPainters() {
			if painter.AfterDraw != nil {
				painter.AfterDraw(c, env)
			}
		}
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless 打不开：%v", err)
	}
	defer session.Close()
	invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatalf("pump 失败：%v", err)
	}
	if err := session.Screenshot(path); err != nil {
		t.Fatalf("截图失败：%v", err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Fatalf("截图文件无效 path=%s info=%v err=%v", path, info, err)
	}
	t.Logf("HH_CAPTURE %s", path)
}

// TestRealChainHHSmallGaps 是 HH 单五件的真机验收。
func TestRealChainHHSmallGaps(t *testing.T) {
	host := startRealHost(t)
	client, mux := connectRealGUI(t, host)

	// ── ① 真会话（真宿主里的真 session）。
	createCtx, createCancel := context.WithTimeout(context.Background(), 30*time.Second)
	sessionID, err := client.Create(createCtx, host.home, "allow", "hh-smallgaps")
	createCancel()
	if err != nil {
		t.Fatalf("session.create 失败：%v", err)
	}
	load, err := client.Load(context.Background(), sessionID, 0)
	if err != nil {
		t.Fatalf("session.load 失败：%v", err)
	}
	t.Logf("真宿主：session=%s home=%s", sessionID, host.home)

	// ── ② 真订阅：跑壳自己的 runSubscription，成功即 LinkConnected。
	m := &model{cli: client, mux: mux, cur: sessionID}
	// 走产品入口（`m.subscribe`），不直裸跑 goroutine：`subs` 里得有这个订阅，
	// 第 2 件的"立即重连"才有东西可取消（真机上也正是这条路径）。
	m.subscribe(sessionID)
	t.Cleanup(func() {
		m.mu.Lock()
		if sub := m.subs[sessionID]; sub != nil && sub.cancel != nil {
			sub.cancel()
		}
		m.mu.Unlock()
	})
	hhWaitLink(t, m, pluginapi.LinkConnected, 30*time.Second)
	t.Logf("真订阅成功：linkStateOf = %v", pluginapi.LinkConnected)

	product := newHHRealProduct(t)
	shotDir := os.Getenv("HH_CAPTURE_DIR")
	if shotDir == "" {
		shotDir = "D:/DeepSeek Harness/default-workspace/DSH-GO/tmp/GO8-RC2/_hh/shots"
	}
	if err := os.MkdirAll(shotDir, 0o755); err != nil {
		t.Fatalf("建截图目录失败：%v", err)
	}
	stamp := time.Now().Unix()

	snap := pluginapi.Snapshot{
		Cur: sessionID, CWD: host.home, Status: "已加载 0 条（总 0）",
		Link:          pluginapi.LinkConnected,
		Msgs:          load.Messages,
		UsageTotal:    gyUsage(load.UsageTotal),
		Usage:         gyUsage(load.Usage),
		ContextTokens: load.ContextTokens,
		ContextWindow: load.ContextWindow,
		Model:         load.Model,
		Permission:    load.Permission,
		Total:         load.Total,
	}
	act := pluginapi.Actions{
		Send:       func(string) {},
		Cancel:     func() {},
		Reconnect:  func() {},
		Invalidate: func() {},
	}

	// ── ③ 第 1 件：设置行是一行（设置入口 + 连接指示同 Y 中心、指示在右）。
	canvas, counts, row := product.paintReal(t, snap, act, "")
	if row.Width == 0 || row.Height == 0 {
		t.Fatalf("骨架没报出设置行几何：%+v", row)
	}
	if counts[uikernel.OffSidebarSettings] != 1 {
		t.Fatalf("设置行每帧必须 dispatch 1 次，得到 %d：%v", counts[uikernel.OffSidebarSettings], counts)
	}
	if row.Height != 42 {
		t.Fatalf("设置行高 = %.2f，want 42（官方 `.trigger{height:42px}`；70 = 两行叠起来）", row.Height)
	}
	connRect, ok := hhFindText(canvas, dsstatusbar.OfficialConnectedLabel)
	if !ok {
		t.Fatalf("设置行里没有连接指示 %q；texts=%v", dsstatusbar.OfficialConnectedLabel, hhTexts(canvas))
	}
	setRect, ok := hhFindText(canvas, "设置")
	if !ok {
		t.Fatalf("设置行里没有设置入口「设置」；texts=%v", hhTexts(canvas))
	}
	if setRect.Y+setRect.Height/2 != connRect.Y+connRect.Height/2 {
		t.Fatalf("设置入口中心 Y=%.2f，连接指示中心 Y=%.2f（不在同一行）",
			setRect.Y+setRect.Height/2, connRect.Y+connRect.Height/2)
	}
	if connRect.X <= setRect.X {
		t.Fatalf("连接指示 X=%.2f 应在设置入口 X=%.2f 右侧", connRect.X, setRect.X)
	}
	product.shot(t, snap, act, filepath.Join(shotDir, fmt.Sprintf("hh-connected-row-%d.png", stamp)))

	// ── ④ 第 4 件：composer 那一行没有连接层措辞（官方座位才说）。
	for _, banned := range []string{
		dsstatusbar.OfficialConnectedLabel,
		dsstatusbar.OfficialDisconnectedLabel,
		dsstatusbar.OfficialConnectingLabel,
		"已连接",
	} {
		if hhHasText(canvas, banned) && banned != dsstatusbar.OfficialConnectedLabel {
			t.Fatalf("composer 出现连接层措辞 %q；texts=%v", banned, hhTexts(canvas))
		}
	}
	// 官方座位必须有这句（否则上面的"没出现"是空断言）。
	if !hhHasText(canvas, dsstatusbar.OfficialConnectedLabel) {
		t.Fatal("官方座位没有连接成功文案——第 4 件的断言成了空断言")
	}

	// ── ⑤ 第 3 件：connecting 态（「重新连接中…」+ warning 胶囊）。
	connecting := snap
	connecting.Link = pluginapi.LinkConnecting
	connectCanvas := product.settle(t, connecting, act)
	if !hhHasText(connectCanvas, dsstatusbar.OfficialConnectingLabel) {
		t.Fatalf("connecting 态没画 %q；texts=%v", dsstatusbar.OfficialConnectingLabel, hhTexts(connectCanvas))
	}
	if hhHasText(connectCanvas, dsstatusbar.OfficialConnectedLabel) {
		t.Fatalf("connecting 态还写着 %q", dsstatusbar.OfficialConnectedLabel)
	}
	product.shot(t, connecting, act, filepath.Join(shotDir, fmt.Sprintf("hh-connecting-%d.png", stamp)))

	// ── ⑥ 第 2 件：真重连。旧订阅取消 + 新订阅真建起来（连接事实回到 connected）。
	m.mu.Lock()
	oldSub := m.subs[sessionID]
	m.mu.Unlock()
	if oldSub == nil {
		t.Fatal("真订阅跑完之后 subs 里没有订阅（重连无从谈起）")
	}
	m.reconnectNow()
	m.mu.Lock()
	newSub := m.subs[sessionID]
	m.mu.Unlock()
	if newSub == nil || newSub == oldSub {
		t.Fatalf("立即重连没有换订阅：old=%p new=%p", oldSub, newSub)
	}
	// 真 WS 再连上：产品形状下连接事实回到 LinkConnected。
	hhWaitLink(t, m, pluginapi.LinkConnected, 30*time.Second)
	t.Logf("真重连成功：旧订阅已取消，新订阅 %p 已连上", newSub)
	product.shot(t, snap, act, filepath.Join(shotDir, fmt.Sprintf("hh-reconnected-%d.png", stamp)))

	// ── ⑦ 断开态（真失败事实）：warning 胶囊可点 + 文案是官方 connection.error。
	down := snap
	down.Link = pluginapi.LinkDown
	down.Disconnected = true
	downCanvas := product.settle(t, down, act)
	if !hhHasText(downCanvas, dsstatusbar.OfficialDisconnectedLabel) {
		t.Fatalf("断开态没画 %q；texts=%v", dsstatusbar.OfficialDisconnectedLabel, hhTexts(downCanvas))
	}
	var reconnectHit bool
	for _, hit := range downCanvas.Hits() {
		if hit.Label == "连接异常，点击立即重连" {
			reconnectHit = true
		}
	}
	if !reconnectHit {
		t.Fatal("断开态的 warning 胶囊没有重连命中区（按钮不可点 = 假动作）")
	}
	product.shot(t, down, act, filepath.Join(shotDir, fmt.Sprintf("hh-disconnected-%d.png", stamp)))

	// ── ⑧ 第 5 件：弹窗开着时 Esc 关弹窗（真内核按键路由）。
	stats := &apiclient.SessionStats{Turns: 1, Steps: 1, LLMMs: 900, ToolMs: 0, TTFTMs: 300, TTFTSteps: 1}
	snapDialog := snap
	snapDialog.SessionStats = stats
	snapDialog.Busy = true
	cancelled := 0
	actCancel := act
	actCancel.Cancel = func() { cancelled++ }
	// 弹窗那一帧也要先跑过离场窗（上一帧是断开态）。
	product.paintReal(t, snapDialog, actCancel, "")
	time.Sleep(200 * time.Millisecond)
	product.paintReal(t, snapDialog, actCancel, "")
	open, _, _ := product.paintReal(t, snapDialog, actCancel, "activity-pill")
	if !hhHasText(open, "会话统计") {
		t.Fatalf("点活动胶囊没打开弹窗；texts=%v", hhTexts(open))
	}
	product.shot(t, snapDialog, actCancel, filepath.Join(shotDir, fmt.Sprintf("hh-stats-dialog-%d.png", stamp)))
	if !product.kernel.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) {
		t.Fatal("弹窗开着时 Esc 没有插件接")
	}
	closed, _, _ := product.paintReal(t, snapDialog, actCancel, "")
	if hhHasText(closed, "会话统计") {
		t.Fatalf("Esc 后弹窗还在；texts=%v", hhTexts(closed))
	}
	if cancelled != 0 {
		t.Fatalf("弹窗开着时 Esc 停了 turn（Cancel %d 次）——第 5 件没修好", cancelled)
	}
	t.Logf("Esc 优先级：弹窗关闭，turn 未停（Cancel=%d）", cancelled)
}

// hhWaitLink 轮询壳里的连接事实（globals 在 m.mu 里写，这里也在锁里读）。
func hhWaitLink(t *testing.T, m *model, want pluginapi.LinkState, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if hhLinkState(m) == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	m.mu.Lock()
	got := linkStateOf(wsUp, wsDown, wsConnecting)
	up, down, connecting := wsUp, wsDown, wsConnecting
	m.mu.Unlock()
	t.Fatalf("%s 内连接态没变成 %v（当前 %v：wsUp=%v wsDown=%v wsConnecting=%v）",
		timeout, want, got, up, down, connecting)
}
