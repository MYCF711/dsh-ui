//go:build windows

package main

// he_stats_dialog_real_chain_test.go —— **真机验收**：宿主真值 → 状态栏「活动」
// 胶囊可点开 → 官方 Activity 详情弹窗把 `sessionStats` 八字段**逐个画成真值**。
//
// 判据（task-53 §任务 5）：
//
//	① 真 `dshd serve`（临时 home + `--addr 127.0.0.1:0`）+ 真 apiclient；
//	② 真跑一轮**含工具调用**的 prompt（bash `echo HE-TOOL-OK`）→ 宿主真写
//	   `step/end` + `tool/call|result`（llmMs / toolMs / ttft / decode 的来源）；
//	③ 产品接线（`wave3Wiring.fetchProjections` → `Snapshot.SessionStats` →
//	   `plugins/dsstatusbar`）之后：点活动胶囊 → 弹窗里出现**由真投影算出的**
//	   counts / 模型用时 / 工具调用用时 / 首 token 平均 / 输出速度五段读数；
//	④ 八个字段都能追到真值（turns/steps 在标题，llmMs/toolMs/ttftMs+ttftSteps/
//	   decodeMs+decodeTokens 在四条明细里）；
//	⑤ 两态：另建一个**空会话**（没 prompt）→ 胶囊写「活动 未知」、**不可点**、
//	   点不出弹窗、不出现 `0 轮`/`0 步`；
//	⑥ 截图三张（收起态 / 展开态 / 空会话降级态）。
//
// 跑法（不给 DSHD_BIN 就跳过）：
//
//	export DSHD_BIN=".../tmp/GO8-RC2/_he/bin/dshd.exe"
//	export DSHD_HOME_ROOT=".../tmp/GO8-RC2/_he/home"
//	export HE_CAPTURE_DIR=".../tmp/GO8-RC2/_he/shots"
//	go test ./cmd/app -run 'TestRealChainHESessionStatsDialog' -count=1 -v
//
// 依赖本机 magpie 网关（`dshd` 默认 `DSHD_LLM_BASE=http://127.0.0.1:3425/v1`）；
// 网关连不上时**跳过**并说清原因——不拿"没跑到"冒充通过。

import (
	"context"
	"fmt"
	"math"
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
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/MYCF711/dsh-ui/plugins/dsstatusbar"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

const (
	// heRealPrompt 让真模型调一次 bash 工具：标记串只可能来自工具输出。
	heRealPrompt = "只做两件事，不要解释：\n1) 用 bash 工具执行 echo HE-TOOL-OK\n2) 最后单独一行回复：HE-STATS-DIALOG-OK"
	// heActivityPillHit 是 dsstatusbar 里活动胶囊的交互名（与插件内的常量同名，
	// 两边都是"活动胶囊"这一个身份；改一处必须改另一处，报告里登记了这条耦合）。
	heActivityPillHit = "activity-pill"
	// heDialogTitle / heDialogLabels 是官方 zh 文案（locale.ts:86-91），弹窗里必须有。
	heDialogTitle       = "会话统计"
	heLabelLLMTime      = "模型用时"
	heLabelToolTime     = "工具调用用时"
	heLabelTTFT         = "首 token 平均（TTFT）"
	heLabelSpeed        = "输出速度（TPS）"
	hePillActivityValue = "活动 未知"
)

// heRealProduct 是真机验收用的**产品形状**装配：官方骨架 + 输入卡片 + dock。
//
// 为什么不用 gyRealProduct：那个装配里也装了 dsgoal/dsquestions，而那一批文件
// 此刻正被另一单并行改写（同一个 worktree）。本任务的判据只涉及 composer dock，
// 少装两个插件既不影响座位形状，也不会被别人的半成品连坐。
type heRealProduct struct {
	kernel *uikernel.Kernel
	theme  *pluginapi.Theme
	tree   *gfx.Tree
	feat   gfx.Scope
	opts   uikernel.ShellOptions
}

func newHERealProduct(t *testing.T) *heRealProduct {
	t.Helper()
	tier := uikernel.NewShellTier()
	tier.Set(true)
	p := &heRealProduct{kernel: uikernel.New("he-stats-dialog"), tree: &gfx.Tree{}}
	p.kernel.Root().Provide(uikernel.KeyShellTier, tier)
	if err := p.kernel.Load(
		&dscore.Plugin{Dark: true}, &dsshell.Plugin{},
		&dscomposer.Plugin{}, &dsstatusbar.Plugin{},
	); err != nil {
		t.Fatalf("装配产品插件集失败：%v", err)
	}
	p.theme = p.kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.feat = p.tree.Feature("he-stats-dialog")
	m := &model{kernel: p.kernel, cur: "s-he"}
	p.opts = shellOptionsFor(m, p.kernel, 1182, 832)
	if p.opts.BridgeLegacyComposer {
		t.Fatal("官方 composer 座位有占用方时 composer 不该走兼容桥")
	}
	return p
}

// paint 画一帧产品路径（真 flex + 真 canvas），click 非空时按该交互名造一次点击。
func (p *heRealProduct) paint(t *testing.T, snap pluginapi.Snapshot, click string) *gfx.Canvas {
	t.Helper()
	canvas := &gfx.Canvas{}
	canvas.SetTree(p.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: p.theme, Data: snap, Tree: p.tree, Feat: p.feat, Canvas: canvas,
		Bounds:    ui.Rect{Width: 1182, Height: 832},
		MainWidth: 902,
		Geometry:  pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902},
		Has:       func(slot pluginapi.Slot) bool { return p.kernel.Has(slot) },
		Act:       pluginapi.Actions{Send: func(string) {}, Cancel: func() {}},
	}
	if click != "" {
		canvas.BeginEventPass(p.tree.ID(p.feat, click))
	}
	var view ui.Page
	view.Draw(canvas, dscore.VivaTheme(p.theme), env.Bounds, func(f ui.C) {
		uikernel.FirstScreen(f, env, p.opts, func(slot pluginapi.Slot) {
			p.kernel.DrawSlot(f, slot, env)
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
	return canvas
}

// heDuration 是官方 `formatDuration`（StatsPills.tsx:112-120）的**独立**第二实现：
// 期望值由测试自己按官方规则算出来，不调用被测代码，才叫"验真值"。
func heDuration(ms float64) string {
	seconds := ms / 1000
	if seconds < 60 {
		rounded := math.Round(seconds*10) / 10
		if rounded == math.Trunc(rounded) {
			return fmt.Sprintf("%d秒", int(rounded))
		}
		return fmt.Sprintf("%.1f秒", rounded)
	}
	whole := int(math.Round(seconds))
	return fmt.Sprintf("%d分%d秒", whole/60, whole%60)
}

// heSpeed 是官方 `formatTokensPerSecond`（message-chrome.ts:71-74）。
func heSpeed(stats *apiclient.SessionStats) string {
	tps := float64(stats.DecodeTokens) / (float64(stats.DecodeMs) / 1000)
	if tps < 0 {
		tps = 0
	}
	if tps >= 10 {
		return fmt.Sprintf("%d tok/s", int(math.Round(tps)))
	}
	rounded := math.Round(tps*10) / 10
	if rounded == math.Trunc(rounded) {
		return fmt.Sprintf("%d tok/s", int(rounded))
	}
	return fmt.Sprintf("%.1f tok/s", rounded)
}

// TestRealChainHESessionStatsDialog 硬验收：真宿主真值 → 真弹窗。
func TestRealChainHESessionStatsDialog(t *testing.T) {
	host := startRealHost(t)
	client, _ := connectRealGUI(t, host)

	createCtx, createCancel := context.WithTimeout(context.Background(), 30*time.Second)
	sessionID, err := client.Create(createCtx, host.home, "workspace-write", "he-stats")
	createCancel()
	if err != nil {
		t.Fatalf("session.create 失败：%v", err)
	}
	t.Logf("真实会话 id=%s（home=%s）", sessionID, host.home)

	// ── ② 真跑一轮含工具调用的 prompt。
	promptCtx, promptCancel := context.WithTimeout(context.Background(), 60*time.Second)
	if _, err := client.Prompt(promptCtx, sessionID, heRealPrompt); err != nil {
		promptCancel()
		t.Fatalf("session.prompt 失败：%v", err)
	}
	promptCancel()
	frames := waitTurnDone(t, client, sessionID, 240*time.Second)
	var stepEnds, toolCalls, toolResults int
	for _, frame := range frames {
		switch frame.Type {
		case "step/end":
			stepEnds++
		case "tool/call":
			toolCalls++
		case "tool/result":
			toolResults++
		}
	}
	t.Logf("真实帧 %d 条：step/end=%d tool/call=%d tool/result=%d", len(frames), stepEnds, toolCalls, toolResults)
	if stepEnds == 0 {
		t.Fatalf("宿主没写 step/end（steps 只能是 0，弹窗无从谈真值）：frames=%v", hcFrameTypes(frames))
	}
	if toolCalls == 0 || toolResults == 0 {
		t.Fatalf("这一轮没有真工具调用（toolMs 无从配对）：frames=%v", hcFrameTypes(frames))
	}

	// ── ③ 产品接线：真 HTTP 拉 sessionStats 投影 → Snapshot。
	m := &model{cli: client}
	wiring := newWave3Wiring()
	wiring.tier.Set(true)
	wiring.fetchProjections(m, sessionID)
	wiring.feed(sessionID, 280)
	stats := wiring.sessionStatsSnapshot(sessionID)
	if stats == nil {
		t.Fatal("产品接线后 SessionStats 仍为 nil")
	}
	t.Logf("真 sessionStats：turns=%d steps=%d llmMs=%d toolMs=%d ttftMs=%d ttftSteps=%d decodeMs=%d decodeTokens=%d",
		stats.Turns, stats.Steps, stats.LLMMs, stats.ToolMs, stats.TTFTMs, stats.TTFTSteps,
		stats.DecodeMs, stats.DecodeTokens)
	if stats.Steps < 1 || stats.Turns < 1 || stats.LLMMs <= 0 || stats.ToolMs <= 0 || stats.TTFTSteps < 1 {
		t.Fatalf("八字段里有不是真值的：%+v", stats)
	}

	// ── ③④ 真画一帧、点活动胶囊、读弹窗。
	product := newHERealProduct(t)
	snap := pluginapi.Snapshot{
		Cur: sessionID, Status: "已连接", CWD: host.home,
		SessionStats: stats,
	}
	closed := product.paint(t, snap, "")
	closedTexts := heTexts(closed)
	if !heHasText(closed, "活动 "+fmt.Sprintf("%d 轮 %d 步", stats.Turns, stats.Steps)) &&
		!heHasActivityReading(closedTexts) {
		t.Fatalf("收起态没有活动读数；texts=%v", closedTexts)
	}
	if heHasText(closed, heDialogTitle) {
		t.Fatalf("没点开就出现了弹窗：%v", closedTexts)
	}

	openCanvas := product.paint(t, snap, heActivityPillHit)
	openTexts := heTexts(openCanvas)
	if !heHasText(openCanvas, heDialogTitle) {
		t.Fatalf("点活动胶囊没有打开详情弹窗；texts=%v", openTexts)
	}

	// ⑧ 字段逐个追真值：期望值由测试自己按官方规则算出。
	wantCounts := fmt.Sprintf("%d 轮 %d 步", stats.Turns, stats.Steps)
	wantRows := []string{
		heLabelLLMTime,
		heDuration(float64(stats.LLMMs)),
		heLabelToolTime,
		heDuration(float64(stats.ToolMs)),
		heLabelTTFT,
		heDuration(float64(stats.TTFTMs) / float64(stats.TTFTSteps)),
	}
	if stats.DecodeMs > 0 {
		wantRows = append(wantRows, heLabelSpeed, heSpeed(stats))
	}
	for _, want := range append([]string{wantCounts}, wantRows...) {
		if !heHasText(openCanvas, want) {
			t.Fatalf("弹窗没渲染真值 %q；texts=%v", want, openTexts)
		}
	}
	t.Logf("弹窗真值：counts=%q 模型用时=%q 工具调用用时=%q 首token平均=%q 输出速度=%q",
		wantCounts, heDuration(float64(stats.LLMMs)), heDuration(float64(stats.ToolMs)),
		heDuration(float64(stats.TTFTMs)/float64(stats.TTFTSteps)),
		func() string {
			if stats.DecodeMs > 0 {
				return heSpeed(stats)
			}
			return "(decodeMs=0，官方不画速率)"
		}())
	// 五段读数都不许是"编的 0"：轮/步为 0 的假读数已经在单测里禁掉，这里反向
	// 断言真机画面上没有 `0 轮`/`0 步`。
	for _, text := range openTexts {
		for _, banned := range []string{"0 轮", "0 步"} {
			if strings.Contains(text, banned) {
				t.Fatalf("真机弹窗出现了编造计数 %q：%v", banned, openTexts)
			}
		}
	}
	// 弹窗必须在胶囊上方（官方 bottom: anchor(top)+8）。
	pill, pillOK := heHitByName(t, product, openCanvas, heActivityPillHit)
	panelLabel := heHitByLabel(openCanvas, heDialogTitle)
	if !pillOK || !panelLabel.ok {
		t.Fatalf("缺命中区：pill=%v panel=%v", pillOK, panelLabel.ok)
	}
	if got, want := panelLabel.hit.Bounds.Y+panelLabel.hit.Bounds.Height, pill.Bounds.Y-8; math.Abs(float64(got-want)) > 0.01 {
		t.Fatalf("弹窗底沿 = %v，want 胶囊顶沿-8 = %v", got, want)
	}
	if !pill.Expanded {
		t.Fatal("展开后胶囊 aria-expanded 不是 true")
	}

	// ── ⑥ 截图目录。
	dir := os.Getenv("HE_CAPTURE_DIR")
	if dir == "" {
		dir = "D:/DeepSeek Harness/default-workspace/DSH-GO/tmp/GO8-RC2/_he/shots"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建截图目录失败：%v", err)
	}
	stamp := time.Now().Unix()
	openShot := filepath.Join(dir, fmt.Sprintf("he-stats-dialog-open-%d.png", stamp))
	heCapture(t, product, snap, openShot)
	t.Logf("HE_CAPTURE open path=%s", openShot)

	// 收起态截图：重新装配一个产品（弹窗状态只在插件里），先画一帧不点。
	closedProduct := newHERealProduct(t)
	closedProduct.paint(t, snap, "")
	closedShot := filepath.Join(dir, fmt.Sprintf("he-stats-dialog-closed-%d.png", stamp))
	heCapture(t, closedProduct, snap, closedShot)
	t.Logf("HE_CAPTURE closed path=%s", closedShot)

	// ── ⑤ 空会话：没 prompt → 宿主给全 0 真值 → 胶囊「活动 未知」+ 不可点 + 点不开。
	emptyCtx, emptyCancel := context.WithTimeout(context.Background(), 30*time.Second)
	emptySessionID, err := client.Create(emptyCtx, host.home, "workspace-write", "he-stats-empty")
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
	if emptyStats.Steps != 0 || emptyStats.Turns != 0 {
		t.Fatalf("空会话 stats = %+v，want turns=0 steps=0", emptyStats)
	}
	emptyProduct := newHERealProduct(t)
	emptySnap := pluginapi.Snapshot{Cur: emptySessionID, SessionStats: emptyStats}
	emptyClosed := emptyProduct.paint(t, emptySnap, "")
	if !heHasText(emptyClosed, hePillActivityValue) {
		t.Fatalf("空会话活动读数 = %v，want「活动 未知」", heTexts(emptyClosed))
	}
	if _, ok := heHitByName(t, emptyProduct, emptyClosed, heActivityPillHit); ok {
		t.Fatal("空会话的活动胶囊仍有命中区（假入口）")
	}
	emptyClicked := emptyProduct.paint(t, emptySnap, heActivityPillHit)
	if heHasText(emptyClicked, heDialogTitle) {
		t.Fatalf("空会话点出了弹窗：%v", heTexts(emptyClicked))
	}
	for _, text := range heTexts(emptyClicked) {
		for _, banned := range []string{"0 轮", "0 步", heDialogTitle} {
			if strings.Contains(text, banned) {
				t.Fatalf("空会话编造了 %q：%v", banned, heTexts(emptyClicked))
			}
		}
	}
	emptyShot := filepath.Join(dir, fmt.Sprintf("he-stats-dialog-empty-%d.png", stamp))
	heCapture(t, emptyProduct, emptySnap, emptyShot)
	t.Logf("HE_CAPTURE empty path=%s", emptyShot)
}

// heTexts 列画布文本。
func heTexts(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

// heHasText 报告画布上有没有**逐字相等**的文本。
func heHasText(canvas *gfx.Canvas, want string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == want {
			return true
		}
	}
	return false
}

// heHasActivityReading 报告画布上有没有形如 `N 轮 M 步…` 的活动读数。
func heHasActivityReading(texts []string) bool {
	for _, text := range texts {
		if strings.Contains(text, " 轮 ") && strings.Contains(text, " 步") {
			return true
		}
	}
	return false
}

// heHitByName 按交互名找命中区。
func heHitByName(t *testing.T, p *heRealProduct, canvas *gfx.Canvas, name string) (gfx.Hit, bool) {
	t.Helper()
	id := p.tree.ID(p.feat, name)
	for _, hit := range canvas.Hits() {
		if hit.ID == id {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

type heLabeledHit struct {
	hit gfx.Hit
	ok  bool
}

// heHitByLabel 按 aria 名找命中区。
func heHitByLabel(canvas *gfx.Canvas, label string) heLabeledHit {
	for _, hit := range canvas.Hits() {
		if hit.Label == label {
			return heLabeledHit{hit: hit, ok: true}
		}
	}
	return heLabeledHit{}
}

// heCapture 用真 headless 窗口过程（真文本测量）把**当前插件状态**落成 PNG。
func heCapture(t *testing.T, p *heRealProduct, snap pluginapi.Snapshot, path string) {
	t.Helper()
	shotTree := &gfx.Tree{}
	shotFeat := shotTree.Feature("he-shot")
	var invalidator app.Invalidator
	config := app.Config{
		Title: "he-stats-dialog", Width: 1182, Height: 832, Tree: shotTree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, p.theme.BgBase)
		env := &pluginapi.Env{
			Theme: p.theme, Data: snap, Tree: shotTree, Feat: shotFeat, Canvas: canvas,
			Bounds: bounds, MainWidth: 902,
			Geometry: pluginapi.Geometry{Width: size.Width, Height: size.Height, SidebarWidth: 280, MainWidth: 902},
			Has:      func(slot pluginapi.Slot) bool { return p.kernel.Has(slot) },
		}
		var view ui.Page
		view.Draw(canvas, dscore.VivaTheme(p.theme), bounds, func(f ui.C) {
			uikernel.FirstScreen(f, env, p.opts, func(slot pluginapi.Slot) {
				p.kernel.DrawSlot(f, slot, env)
			})
		})
		for _, painter := range p.kernel.AllPainters() {
			if painter.AfterDraw != nil {
				painter.AfterDraw(canvas, env)
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
