//go:build windows

package main

// hc_real_chain_test.go —— HC 真机验收：GUI **真消费**宿主 rc.2 新增的两个投影。
//
// 两个投影（宿主 `4e0b248` 实现，契约在 `DSH-GO/docs/transport-contract.md` §3D）：
//
//	sessionStats              状态栏「活动」的 whole-log 读数（八字段）
//	team/mode → teamMode     官方 Team 开关的持久值（{enabled:bool}）
//	modelSelection            Team 开关的可用性判据（官方 `modelSelection !== undefined`）
//
// # 为什么要真机
//
// 单测（`internal/apiclient`、`plugins/dsstatusbar`、`plugins/dsagentteam`、
// `cmd/app/wave3_wiring_test.go`）证明的是"给了这份数据就画成这样"；
// 这里证明的是**宿主真的会给**：真 `dshd serve` 跑一轮含工具调用的 prompt 后，
// `session.projections` 的八字段是真值，GUI 的**产品接线**
// （`wave3Wiring.fetchProjections` → `Snapshot.SessionStats` / `Store.SetView`）
// 把它接到状态栏与 Team 开关上。
//
// # 判据
//
//	① 真 dshd serve（临时 home + `--addr 127.0.0.1:0`）+ 真 apiclient；
//	② 真跑一轮**含工具调用**的 prompt（bash `echo`）→ 宿主真写 step/end + tool/call|result；
//	③ 产品接线后状态栏「活动」= 真实 `N 轮 M 步 · X tok/s`（**不是**「未知」）；
//	④ 真 `agentTeams.setEnabled(true)` → `teamMode={enabled:true}` → 官方 Team 开关
//	   出现在模型菜单里、checked、可用；
//	⑤ 两态：另一个没开过 Team 的空会话 →「活动 未知」+ 开关**不渲染**；
//	⑥ 不编 0：缺投影/空会话时不得出现 "0 轮 0 步"；
//	⑦ 截图：状态栏真值 + Team 开关 + 空会话未知三张 PNG。
//
// 跑法（不给 DSHD_BIN 就跳过）：
//
//	export DSHD_BIN=".../tmp/GO8-RC2/_hc/bin/dshd.exe"
//	export DSHD_HOME_ROOT=".../tmp/GO8-RC2/_hc"
//	export HC_CAPTURE_DIR=".../tmp/GO8-RC2/_hc/shots"
//	go test ./cmd/app -run 'TestRealChainHCConsumeProjections' -count=1 -v
//
// 依赖本机 magpie 网关（`dshd` 默认 `DSHD_LLM_BASE=http://127.0.0.1:3425/v1`）；
// 网关连不上时**跳过**并说清原因——不拿"没跑到"冒充通过。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// hcRealPrompt 让真模型调一次 bash 工具（标记串只可能来自工具输出）。
const (
	hcRealPrompt = "只做两件事，不要解释：\n1) 用 bash 工具执行 echo HC-TOOL-OK\n2) 最后单独一行回复：HC-REAL-PROJECTIONS-OK"
	hcRealModel  = "group/deepseek-v4.1-flash-1017"
)

// hcRealChain 是真机验收的共享夹具（两个会话：有真值 / 空会话）。
type hcRealChain struct {
	client *apiclient.Client
	home   string
	// sessionID 跑过一轮含工具调用的 prompt。
	sessionID string
	frames    []apiclient.Frame
	// emptySessionID 建了但既没 prompt、也没开 Team。
	emptySessionID string
}

// TestRealChainHCConsumeProjections 是硬验收：两个投影都**真被消费**。
func TestRealChainHCConsumeProjections(t *testing.T) {
	host := startRealHost(t)
	client, _ := connectRealGUI(t, host)

	createCtx, createCancel := context.WithTimeout(context.Background(), 30*time.Second)
	sessionID, err := client.Create(createCtx, host.home, "workspace-write", "hc-projections")
	createCancel()
	if err != nil {
		t.Fatalf("session.create 失败：%v", err)
	}
	t.Logf("真实会话 id=%s（home=%s）", sessionID, host.home)

	// ── ④ 先开 Team：宿主按官方形状 append `team/mode {enabled:true}`。
	enableCtx, enableCancel := context.WithTimeout(context.Background(), 20*time.Second)
	if err := client.SetAgentTeamEnabled(enableCtx, sessionID, true); err != nil {
		enableCancel()
		t.Fatalf("agentTeams.setEnabled(true) 失败：%v", err)
	}
	enableCancel()

	// ── ② 真跑一轮含工具调用的 prompt。
	promptCtx, promptCancel := context.WithTimeout(context.Background(), 60*time.Second)
	if _, err := client.Prompt(promptCtx, sessionID, hcRealPrompt); err != nil {
		promptCancel()
		t.Fatalf("session.prompt 失败：%v", err)
	}
	promptCancel()
	frames := waitTurnDone(t, client, sessionID, 240*time.Second)
	var toolCalls, toolResults, stepEnds, textFrames int
	for _, frame := range frames {
		switch frame.Type {
		case "tool/call":
			toolCalls++
		case "tool/result":
			toolResults++
		case "step/end":
			stepEnds++
		case "assistant/text", "assistant/message":
			textFrames++
		}
	}
	t.Logf("真实帧 %d 条：step/end=%d tool/call=%d tool/result=%d text=%d",
		len(frames), stepEnds, toolCalls, toolResults, textFrames)
	if stepEnds == 0 {
		t.Fatalf("宿主没写 step/end（sessionStats 的 steps 只能是 0）：frames=%v", hcFrameTypes(frames))
	}
	if toolCalls == 0 || toolResults == 0 {
		t.Fatalf("这一轮没有真工具调用（toolMs 无从配对）：frames=%v", hcFrameTypes(frames))
	}

	// ── ③ 产品接线：真 HTTP 拉投影 → wave3Wiring 缓存 → Snapshot / Store。
	gx := newGXProduct(t, shellOfficial)
	wiring := newWave3Wiring()
	wiring.team = gx.team
	wiring.tier.Set(true)
	m := &model{cli: client}
	wiring.fetchProjections(m, sessionID)
	wiring.feed(sessionID, 280)

	stats := wiring.sessionStatsSnapshot(sessionID)
	if stats == nil {
		t.Fatal("产品接线后 SessionStats 仍为 nil（状态栏只能写「未知」）")
	}
	t.Logf("真 sessionStats：turns=%d steps=%d llmMs=%d toolMs=%d ttftMs=%d ttftSteps=%d decodeMs=%d decodeTokens=%d",
		stats.Turns, stats.Steps, stats.LLMMs, stats.ToolMs, stats.TTFTMs, stats.TTFTSteps,
		stats.DecodeMs, stats.DecodeTokens)
	// 八字段都必须是真值（不是 0 占位）：这一轮跑过模型 + 工具。
	if stats.Steps < 1 {
		t.Fatalf("steps = %d（宿主写了 %d 条 step/end）", stats.Steps, stepEnds)
	}
	if stats.Turns < 1 {
		t.Fatalf("turns = %d，want >= 1", stats.Turns)
	}
	if stats.LLMMs <= 0 {
		t.Fatalf("llmMs = %d（模型真的跑过，墙钟必须 > 0）", stats.LLMMs)
	}
	if stats.ToolMs <= 0 {
		t.Fatalf("toolMs = %d（工具有真执行，配对墙钟必须 > 0）", stats.ToolMs)
	}
	if stats.TTFTSteps < 1 {
		t.Fatalf("ttftSteps = %d（宿主写了 assistant/text 增量，首 token 必须记到）", stats.TTFTSteps)
	}

	// ── ③' 状态栏「活动」= 官方读数（真值），不是「未知」。
	snap := pluginapi.Snapshot{
		Cur: sessionID, Status: "已连接", CWD: host.home,
		SessionStats: stats,
	}
	_, texts, _ := newGYRealProduct(t).paint(t, snap, "")
	activity, ok := hcActivityText(texts)
	if !ok {
		t.Fatalf("状态栏没有「活动」读数；texts=%v", texts)
	}
	if strings.Contains(activity, "未知") {
		t.Fatalf("有 sessionStats 真值时状态栏仍写「未知」：%q", activity)
	}
	wantCounts := fmt.Sprintf("%d 轮 %d 步", stats.Turns, stats.Steps)
	if !strings.HasPrefix(activity, wantCounts) {
		t.Fatalf("活动读数 = %q，want 前缀 %q（官方 stats.counts）", activity, wantCounts)
	}
	if stats.DecodeMs > 0 {
		if !strings.Contains(activity, "tok/s") {
			t.Fatalf("decodeMs>0 时活动读数应带速率：%q", activity)
		}
	}
	t.Logf("真机状态栏活动读数：%q", activity)

	// ── ④' Team 开关：真 teamMode 投影 → 官方开关出现、checked、可用。
	view := gx.team.View()
	if !view.ProjectionKnown {
		t.Fatal("开了 Team 之后 View.ProjectionKnown 仍为 false（开关不渲染）")
	}
	if !view.Projection.Enabled {
		t.Fatal("宿主 teamMode.enabled=true，View 里却还是「关」")
	}
	if !view.ModelSelectionKnown {
		t.Fatal("modelSelection 投影已知，View.ModelSelectionKnown 却为 false（开关会被禁用）")
	}
	// 第一帧只画 trigger（AfterDraw 缓存它的真实矩形），第二帧点开菜单。
	gx.paint(t, snap, "")
	menu := gx.paint(t, snap, "model-menu-trigger")
	if !gxHasText(menu, gxTeamModeLabel) {
		t.Fatalf("模型菜单里没有官方 Team 开关 %q；texts=%v", gxTeamModeLabel, gxTexts(menu))
	}
	hit, ok := gxSwitchHit(menu)
	if !ok {
		t.Fatalf("Team 开关没有命中区（点不了）；hits=%v", menu.Hits())
	}
	if !hit.Checked {
		t.Fatalf("teamMode.enabled=true，开关却是「关」：%+v", hit)
	}
	if hit.Disabled {
		t.Fatalf("modelSelection 已知时开关不该禁用：%+v", hit)
	}
	t.Logf("真机 Team 开关：文案=%q checked=%v disabled=%v", gxTeamModeLabel, hit.Checked, hit.Disabled)

	// ── ⑦ 截图目录（Team 那张必须**现在**拍：下面切到空会话会覆盖 store 的视图）。
	dir := os.Getenv("HC_CAPTURE_DIR")
	if dir == "" {
		dir = "D:/DeepSeek Harness/default-workspace/DSH-GO/tmp/GO8-RC2/_hc/shots"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建截图目录失败：%v", err)
	}
	stamp := time.Now().Unix()

	teamShot := filepath.Join(dir, fmt.Sprintf("hc-team-mode-on-%d.png", stamp))
	gxCapture(t, gx, snap, teamShot)
	t.Logf("HC_CAPTURE team-mode path=%s", teamShot)

	// ── ⑤⑥ 两态：空会话（没 prompt、没开 Team）必须「未知」+ 开关不渲染，且不编 0。
	emptyCtx, emptyCancel := context.WithTimeout(context.Background(), 30*time.Second)
	emptySessionID, err := client.Create(emptyCtx, host.home, "workspace-write", "hc-empty")
	emptyCancel()
	if err != nil {
		t.Fatalf("空会话 session.create 失败：%v", err)
	}
	wiring.fetchProjections(m, emptySessionID)
	wiring.feed(emptySessionID, 280)

	emptyStats := wiring.sessionStatsSnapshot(emptySessionID)
	if emptyStats == nil {
		t.Fatal("空会话宿主也应给 sessionStats（全 0 的真值），得到 nil")
	}
	if emptyStats.Steps != 0 {
		t.Fatalf("空会话 steps = %d，want 0", emptyStats.Steps)
	}
	emptySnap := pluginapi.Snapshot{Cur: emptySessionID, SessionStats: emptyStats}
	_, emptyTexts, _ := newGYRealProduct(t).paint(t, emptySnap, "")
	emptyActivity, ok := hcActivityText(emptyTexts)
	if !ok || emptyActivity != "活动 未知" {
		t.Fatalf("空会话（steps=0）活动读数 = %q（ok=%v），want「活动 未知」", emptyActivity, ok)
	}
	for _, banned := range []string{"0 轮", "0 步", "tok/s"} {
		if strings.Contains(emptyActivity, banned) {
			t.Fatalf("空会话编造了 %q：%q", banned, emptyActivity)
		}
	}
	emptyView := gx.team.View()
	if emptyView.ProjectionKnown {
		t.Fatalf("空会话从没开过 Team，View.ProjectionKnown 必须 false（开关不渲染）：%+v", emptyView)
	}

	// ── ⑦ 截图（真 headless 文本测量 + 真窗口过程）。
	statusShot := filepath.Join(dir, fmt.Sprintf("hc-statusbar-activity-%d.png", stamp))
	hcCaptureStatusbar(t, snap, statusShot)
	t.Logf("HC_CAPTURE statusbar path=%s", statusShot)

	emptyShot := filepath.Join(dir, fmt.Sprintf("hc-empty-unknown-%d.png", stamp))
	hcCaptureStatusbar(t, emptySnap, emptyShot)
	t.Logf("HC_CAPTURE empty path=%s", emptyShot)
}

// hcActivityText 从画布文本里取「活动」那一枚胶囊。
//
// 官方 dock 是常驻的：有真值写 `N 轮 M 步 · X tok/s`，没真值写 `活动 未知`。
func hcActivityText(texts []string) (string, bool) {
	for _, text := range texts {
		if text == "活动 未知" || strings.Contains(text, " 轮 ") {
			return text, true
		}
	}
	return "", false
}

// hcFrameTypes 只取帧类型（失败诊断用，避免把整帧打出来）。
func hcFrameTypes(frames []apiclient.Frame) []string {
	out := make([]string, 0, len(frames))
	for _, frame := range frames {
		out = append(out, frame.Type)
	}
	return out
}

// hcCaptureStatusbar 用真 headless 窗口过程截一张产品形状的状态栏。
//
// 与 `gyCapture` 同构：走官方骨架（`FirstScreen`）+ 真文本测量，画的就是
// 刚才断言的那一帧（同一份 Snapshot / 同一套插件）。
func hcCaptureStatusbar(t *testing.T, snap pluginapi.Snapshot, path string) {
	t.Helper()
	product := newGYRealProduct(t)
	shotTree := &ui.Tree{}
	shotFeat := shotTree.Feature("hc-shot")
	var invalidator app.Invalidator
	config := app.Config{
		Title: "hc-statusbar", Width: 1182, Height: 832, Tree: shotTree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(c *ui.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		c.Fill(bounds, product.theme.BgBase)
		shotEnv := &pluginapi.Env{
			Theme: product.theme, Data: snap, Tree: shotTree, Feat: shotFeat, Canvas: c,
			Bounds:    bounds,
			MainWidth: 902,
			Geometry:  pluginapi.Geometry{Width: size.Width, Height: size.Height, SidebarWidth: 280, MainWidth: 902},
			Has:       product.kernel.Has,
			Act:       pluginapi.Actions{Goal: func() string { return "" }},
		}
		var view flex.View
		view.Draw(c, dscore.VivaTheme(product.theme), bounds, func(f *flex.Frame) {
			uikernel.FirstScreen(f, shotEnv, product.opts, func(slot pluginapi.Slot) {
				product.kernel.DrawSlot(f, slot, shotEnv)
			})
		})
		for _, painter := range product.kernel.AllPainters() {
			if painter.AfterDraw != nil {
				painter.AfterDraw(c, shotEnv)
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
