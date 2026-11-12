//go:build windows

package main

// hf_real_chain_test.go —— **真机验收**：连接指示在侧栏官方座位、目标条图标是官方 sprite。
//
// 判据（对应 task-54 §任务 4）：
//
//	① 真 `dshd serve`（临时 home + `--addr 127.0.0.1:0`）+ 真 apiclient（/connect 换 cookie）；
//	② 真会话 + 真 `session.setGoal`（宿主读回一致）；
//	③ **连接事实来自真 apiclient**：跑壳自己的 `model.runSubscription`，
//	   真 `Mux.Subscribe` 成功 → `wsUp` → `linkStateOf` = `LinkConnected`；
//	   另用一个连不上的 Base 做一次真订阅 → 失败 → `linkStateOf(false,true)` = `LinkDown`；
//	④ `-shell official` 的产品形状下（旧 sidebar 还在、整列兼容桥开着）：连接指示画在
//	   **侧栏 footer 的官方位置**；另起一帧"官方骨架独占左栏"证明 `sidebar.settings`
//	   真的会被 dispatch、指示器真的画在那个座位上；
//	⑤ 目标条图标是官方 sprite（没有 4px 圆点矩形、有描边图元、文案在）；
//	⑥ 截图落 PNG（连着 + 断开两张）。
//
// 跑法（不给 DSHD_BIN 就跳过）：
//
//	export DSHD_BIN=".../tmp/GO8-RC2/_hf/bin/dshd.exe"
//	export DSHD_HOME_ROOT=".../tmp/GO8-RC2/_hf"
//	export HF_CAPTURE_DIR=".../tmp/GO8-RC2/_hf/shots"
//	go test ./cmd/app -run 'TestRealChainHFConnectionGoalIcon' -count=1 -v

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
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsstatusbar"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// hfRealGoal 是写进宿主的目标（标记串只可能出现在 goal 值里）。
const hfRealGoal = "HF-GOAL-OK 目标条用官方 sprite glyph"

// hfPaint 用产品插件集画一帧，**先按 main.go 的做法写一次 `LegacyBridge`**。
//
// 为什么必须写：`OfficialSlotsLive()` = `Official() && !LegacyBridge()`。
// 产品形状里旧 sidebar 还占着槽（BridgeLegacy=true），官方 `sidebar.settings`
// 那一帧不会被 dispatch，连接指示必须走 `sidebar.footer` 兼容桥；如果这一帧
// 忘了写桥标记，两个座位都会以为自己不该画——真机上就是"界面空白且不报错"。
// 壳在 `cmd/app/main.go` 的绘制前就是这么写的（`m.wave3.tier.SetLegacyBridge`），
// 测试替身必须同源，否则替身验的就不是产品行为。
func hfPaint(t *testing.T, product *gyRealProduct, snap pluginapi.Snapshot, goal string) (*gfx.Canvas, []string, map[pluginapi.Slot]int) {
	t.Helper()
	if tier, ok := product.kernel.MustGet(uikernel.KeyShellTier).(*uikernel.ShellTier); ok {
		tier.SetLegacyBridge(product.opts.BridgeLegacy)
	}
	return product.paint(t, snap, goal)
}

// hfDrawSlot 与 `uikernel.Kernel.DrawSlot` 同义，额外记录每槽派发次数
// （本测试要断言"官方座位那一帧真的被派发"以及"桥与官方座位同帧只活一个"）。
func hfDrawSlot(f ui.C, k *uikernel.Kernel, slot pluginapi.Slot, env *pluginapi.Env, counts map[pluginapi.Slot]int) {
	counts[slot]++
	for _, painter := range k.Painters(slot) {
		if painter.Draw != nil {
			painter.Draw(f, env)
		}
		if painter.Children != nil {
			painter.Children(f, env, func(next pluginapi.Slot) {
				hfDrawSlot(f, k, next, env, counts)
			})
		}
	}
}

// hfOfficialSidebarFrame 画一帧"官方骨架独占左栏"（没有旧 sidebar 占槽）。
//
// 这一帧存在的意义：产品形状里 `sidebar.settings` 永远被兼容桥顶掉，
// 只有把旧部件摘掉才看得到官方座位本身。它证明连接指示**注册在
// `sidebar.settings` 上并且真的会被画**，而不是"登记了但永远画不到"。
func hfOfficialSidebarFrame(t *testing.T, snap pluginapi.Snapshot) (*gfx.Canvas, map[pluginapi.Slot]int) {
	t.Helper()
	tier := uikernel.NewShellTier()
	tier.Set(true)
	kernel := uikernel.New("hf-official-sidebar")
	kernel.Root().Provide(uikernel.KeyShellTier, tier)
	// 只有 dscore + dsstatusbar：没有 dssidebar/dsshell 占旧槽 → 兼容桥关 → 官方骨架自己画左栏。
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dsstatusbar.Plugin{}); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: theme, Data: snap, Tree: tree, Feat: tree.Feature("hf-official-sidebar"),
		Canvas: canvas, Bounds: ui.Rect{Width: 1182, Height: 832}, MainWidth: 902,
		Geometry: pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902},
	}
	env.Has = func(slot pluginapi.Slot) bool { return kernel.Has(slot) }
	opts := shellOptionsFor(&model{cur: "official-sidebar"}, kernel, 1182, 832)
	if opts.BridgeLegacy {
		t.Fatal("这一帧必须没有旧部件占槽（否则官方 sidebar.settings 不会被 dispatch）")
	}
	// 与 main.go 同源：这一帧没走兼容桥，官方子槽活着。
	tier.SetLegacyBridge(opts.BridgeLegacy)
	counts := map[pluginapi.Slot]int{}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		uikernel.FirstScreen(f, env, opts, func(slot pluginapi.Slot) {
			hfDrawSlot(f, kernel, slot, env, counts)
		})
	})
	for _, painter := range kernel.AllPainters() {
		if painter.AfterDraw != nil {
			painter.AfterDraw(canvas, env)
		}
	}
	return canvas, counts
}

// hfCanvasTexts 收画布文字。
func hfCanvasTexts(canvas *gfx.Canvas) []string {
	values := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		values = append(values, text.Value)
	}
	return values
}

// hfCanvasHas 前缀匹配（长文本会被省略号截断）。
func hfCanvasHas(canvas *gfx.Canvas, want string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == want || strings.HasPrefix(text.Value, want) {
			return true
		}
	}
	return false
}

// hfTextHas 前缀匹配（文本切片版）。
func hfTextHas(texts []string, want string) bool {
	for _, text := range texts {
		if text == want || strings.HasPrefix(text, want) {
			return true
		}
	}
	return false
}

// hfWaitLink 轮询壳里的连接三态（globals 在 m.mu 里写，这里也在锁里读）。
func hfWaitLink(t *testing.T, m *model, want pluginapi.LinkState, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		got := linkStateOf(wsUp, wsDown, wsConnecting)
		m.mu.Unlock()
		if got == want {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	m.mu.Lock()
	got := linkStateOf(wsUp, wsDown, wsConnecting)
	up, down, connecting := wsUp, wsDown, wsConnecting
	m.mu.Unlock()
	t.Fatalf("%s 内连接态没变成 %v（当前 %v：wsUp=%v wsDown=%v wsConnecting=%v）",
		timeout, want, got, up, down, connecting)
}

// TestRealChainHFConnectionGoalIcon 真宿主 + 真 apiclient + 官方座位 + 官方 sprite。
func TestRealChainHFConnectionGoalIcon(t *testing.T) {
	host := startRealHost(t)
	client, mux := connectRealGUI(t, host)

	// ── ① 真会话 + 真 goal。
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	sessionID, err := client.Create(ctx, host.home, "allow", "hf-connection-goal")
	cancel()
	if err != nil {
		t.Fatalf("session.create 失败：%v", err)
	}
	setCtx, setCancel := context.WithTimeout(context.Background(), 20*time.Second)
	var goalOut map[string]any
	err = client.Call(setCtx, "session.setGoal", map[string]any{"id": sessionID, "goal": hfRealGoal}, &goalOut)
	setCancel()
	if err != nil {
		t.Fatalf("session.setGoal 失败：%v", err)
	}
	load, err := client.Load(context.Background(), sessionID, 0)
	if err != nil {
		t.Fatalf("session.load 失败：%v", err)
	}
	if load.Goal != hfRealGoal {
		t.Fatalf("宿主读回 goal = %q，want %q", load.Goal, hfRealGoal)
	}
	t.Logf("真宿主：session=%s goal=%q", sessionID, load.Goal)

	// ── ② 真连接事实（连着）：跑壳自己的订阅循环，成功即 wsUp。
	m := newRealModel(client, mux, sessionID)
	subCtx, subCancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); m.runSubscription(subCtx, sessionID, 0) }()
	hfWaitLink(t, m, pluginapi.LinkConnected, 30*time.Second)
	t.Logf("真订阅成功：linkStateOf = %v", pluginapi.LinkConnected)

	snapConnected := pluginapi.Snapshot{
		Cur: sessionID, CWD: host.home, Status: "已连接",
		Link:          pluginapi.LinkConnected,
		Disconnected:  false,
		Msgs:          load.Messages,
		UsageTotal:    gyUsage(load.UsageTotal),
		Usage:         gyUsage(load.Usage),
		ContextTokens: load.ContextTokens,
		ContextWindow: load.ContextWindow,
		Model:         load.Model,
		Permission:    load.Permission,
		Total:         load.Total,
	}

	// ── ③ 产品形状（`-shell official`）真画一帧：连接指示在侧栏、目标条图标是 sprite。
	product := newGYRealProduct(t)
	productCanvas, texts, counts := hfPaint(t, product, snapConnected, load.Goal)
	// 产品形状：旧 sidebar 占着槽 → 整列兼容桥开着 → 官方**座位**不画，
	// 但「设置行」那一行（`SettingsRoot.triggerRow`）在两种档位下都 dispatch：
	// 官方档位画官方座位，兼容桥档位画桥。行容器只有一个，所以计数必须是 1
	// （不是 0——0 就意味着连接指示又被挤回独立行了，HH 单第 1 件要拆的正是那个）。
	if counts[uikernel.OffSidebarSettings] != 1 {
		t.Fatalf("设置行（triggerRow）每帧必须 dispatch 1 次，得到 %d：%v",
			counts[uikernel.OffSidebarSettings], counts)
	}
	if counts[pluginapi.SlotSidebarFooter] != 1 {
		t.Fatalf("侧栏 footer 的其余行也必须画（每帧 1 次）：%v", counts)
	}
	for _, want := range []string{dsstatusbar.OfficialConnectedLabel, "目标", hfRealGoal} {
		if !hfTextHas(texts, want) {
			t.Fatalf("产品形状画布缺 %q：%v", want, texts)
		}
	}
	// 目标条图标：官方 sprite 的描边（三角化），且**没有** 4×4 圆点矩形。
	for _, rect := range productCanvas.Rectangles() {
		if rect.Bounds.Width == 4 && rect.Bounds.Height == 4 {
			t.Fatalf("目标条还在画 4px 圆点：%+v", rect)
		}
	}
	if len(productCanvas.Triangles()) == 0 {
		t.Fatal("目标条/连接指示的描边图标一个三角形都没有（图标没画出来）")
	}

	// ── ④ 官方座位单独一帧：`sidebar.settings` 真被 dispatch，指示器真画在那里。
	officialCanvas, officialCounts := hfOfficialSidebarFrame(t, snapConnected)
	if officialCounts[uikernel.OffSidebarSettings] != 1 {
		t.Fatalf("官方骨架那一帧 sidebar.settings 派发 %d 次，want 1：%v",
			officialCounts[uikernel.OffSidebarSettings], officialCounts)
	}
	if !hfCanvasHas(officialCanvas, dsstatusbar.OfficialConnectedLabel) {
		t.Fatalf("sidebar.settings 上没画出连接指示：%v", hfCanvasTexts(officialCanvas))
	}
	t.Logf("官方座位：sidebar.settings 派发 %d 次，连接指示已画",
		officialCounts[uikernel.OffSidebarSettings])

	// ── ⑤ 截图（真 headless 文本测量 → PNG）。
	dir := os.Getenv("HF_CAPTURE_DIR")
	if dir == "" {
		dir = "D:/DeepSeek Harness/default-workspace/DSH-GO/tmp/GO8-RC2/_hf/shots"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建截图目录失败：%v", err)
	}
	shotProduct := filepath.Join(dir, fmt.Sprintf("hf-official-connected-%d.png", time.Now().Unix()))
	hfCapture(t, product, snapConnected, load.Goal, shotProduct)
	t.Logf("HF_CAPTURE connected=%s", shotProduct)

	// ── ⑥ 真失败事实（断开）：换一个连不上的 Base，真订阅失败。
	badCtx, badCancel := context.WithTimeout(context.Background(), 5*time.Second)
	badMux := &apiclient.Mux{Base: "http://127.0.0.1:1", Cookies: client.Cookies()}
	if _, err := badMux.Subscribe(badCtx, "session:"+sessionID, 0); err == nil {
		badCancel()
		t.Fatal("连不上的 Base 居然订阅成功了？")
	}
	badCancel()
	down := linkStateOf(false, true, false)
	if down != pluginapi.LinkDown {
		t.Fatalf("订阅失败的事实 = %v，want LinkDown", down)
	}
	snapDown := snapConnected
	snapDown.Link = down
	snapDown.Disconnected = true
	// 壳的状态串不再写连接措辞（HH 单第 4 件：composer 那行与官方座位重复），
	// 连接层的可见措辞只由 `Snapshot.Link` 驱动的官方胶囊给。
	snapDown.Status = "已加载 1 条（总 1）"
	_, downTexts, downCounts := hfPaint(t, product, snapDown, load.Goal)
	if downCounts[pluginapi.SlotSidebarFooter] != 1 {
		t.Fatalf("断开态也要在侧栏 footer 画一次：%v", downCounts)
	}
	if !hfTextHas(downTexts, dsstatusbar.OfficialDisconnectedLabel) {
		t.Fatalf("断开态没写 %q：%v", dsstatusbar.OfficialDisconnectedLabel, downTexts)
	}
	if hfTextHas(downTexts, dsstatusbar.OfficialConnectedLabel) {
		t.Fatalf("断开态还写着 %q：%v", dsstatusbar.OfficialConnectedLabel, downTexts)
	}
	shotDown := filepath.Join(dir, fmt.Sprintf("hf-official-disconnected-%d.png", time.Now().Unix()))
	hfCapture(t, product, snapDown, load.Goal, shotDown)
	t.Logf("HF_CAPTURE disconnected=%s", shotDown)

	// 收尾：先停订阅循环（Cleanup 会 kill 宿主进程）。
	subCancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Log("订阅循环 10s 内没退出（宿主被 kill 后 readLoop 才返回，不影响结论）")
	}
}

// hfCapture 用真 headless 渲染产品的一帧并存 PNG。
func hfCapture(t *testing.T, product *gyRealProduct, snap pluginapi.Snapshot, goal, path string) {
	t.Helper()
	// 截图这一帧也要写桥标记（同 hfPaint）：否则连接指示两个座位都不画。
	if tier, ok := product.kernel.MustGet(uikernel.KeyShellTier).(*uikernel.ShellTier); ok {
		tier.SetLegacyBridge(product.opts.BridgeLegacy)
	}
	shotTree := &gfx.Tree{}
	shotFeat := shotTree.Feature("hf-shot")
	var invalidator app.Invalidator
	config := app.Config{
		Title: "hf-official", Width: 1182, Height: 832, Tree: shotTree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(c *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		c.Fill(bounds, product.theme.BgBase)
		env := &pluginapi.Env{
			Theme: product.theme, Data: snap, Tree: shotTree, Feat: shotFeat, Canvas: c,
			Bounds: bounds, MainWidth: 902,
			Geometry: pluginapi.Geometry{Width: size.Width, Height: size.Height, SidebarWidth: 280, MainWidth: 902},
			Has:      product.kernel.Has,
			Act:      pluginapi.Actions{Goal: func() string { return goal }},
		}
		pageTheme2 := dscore.VivaTheme(product.theme)
		view := ui.NewPage(ui.PageConfig{Theme: &pageTheme2})
		view.Draw(c, bounds, func(f ui.C) {
			uikernel.FirstScreen(f, env, product.opts, func(slot pluginapi.Slot) {
				product.kernel.DrawSlot(f, slot, env)
			})
		})
		for _, painter := range product.kernel.AllPainters() {
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
}
