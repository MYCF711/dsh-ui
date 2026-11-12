//go:build windows

package main

// gy_real_chain_test.go —— **真机验收**：真 `dshd serve` + 真 apiclient +
// `-shell official` 下，目标条 / 提问卡 / 状态栏三件都在官方座位上真画出来。
//
// 判据（task-47 §任务 5）：
//
//	① 真宿主真值：`session.setGoal` 落一个**活跃 goal**，`session.load` 读回来一致；
//	② 真待答问题：让真模型调一次 `ask_user_question`，`session.load` 的**最后一条**
//	   是那条 tool 消息（之后没有 user 消息 = 还在等）；
//	③ `-shell official` 的产品形状（旧 sidebar/正文 + 官方 composer 座位）下：
//	   `conversation.composer` 派发 1 次、`main.composer` 0 次（三个部件都挂在这个
//	   chain 座位上——官方子槽今天由 composer 占用方转发，本仓占用方还没转发）；
//	④ 画布上：目标条的「目标」+ 目标正文、提问卡的「在等你回答」+ 问题、
//	   状态栏的读数胶囊（活动「未知」+ 用量真值）各出现一次；
//	   注：本快照**刻意不喂** `SessionStats`——这条钉的是"没有投影时写未知"的
//	   降级路径（老宿主 / 首帧）；有真值时的活动读数由 `hc_real_chain_test.go`
//	   的真机验收钉住。
//	⑤ 截图落 PNG（真 headless 文本测量）。
//
// 跑法（不给 DSHD_BIN 就跳过）：
//
//	export DSHD_BIN=".../tmp/GO8-RC2/_gr/dshd.exe"
//	export DSHD_HOME_ROOT=".../tmp/GO8-RC2/_gy"
//	export GY_CAPTURE_DIR=".../tmp/GO8-RC2/_gy"
//	go test ./cmd/app -run 'TestRealChainGYGoalQuestionsStatusbar' -count=1 -v
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
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscomposer"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsgoal"
	"github.com/MYCF711/dsh-ui/plugins/dsquestions"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar"
	"github.com/MYCF711/dsh-ui/plugins/dsstatusbar"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// gyRealGoal 是写进宿主的那条活跃 goal（**标记串**只可能出现在目标值里）。
const gyRealGoal = "GY-GOAL-OK 把目标条/提问卡/状态栏迁到官方槽位"

// gyRealAskPrompt 让真模型只调一次 ask_user_question：两个约束（不用别的工具、
// 不解释）能把这一轮压成一次工具调用，验收稳定在十几秒。
const gyRealAskPrompt = "请只调用一次 ask_user_question 工具，问我「先做哪个？」，" +
	"选项两个：label「目标条」说明 dsgoal、label「提问卡」说明 dsquestions。" +
	"header 写「确认」。不要使用其它工具，不要输出解释文字。"

// gyUsage 把 host 的 usage 投影转成插件快照的 usage（字段逐个搬，不整份互转：
// 两个类型各自演进，整份转换会在加字段时静默丢数据）。
func gyUsage(u apiclient.Usage) pluginapi.Usage {
	return pluginapi.Usage{
		InputTokens: u.InputTokens, OutputTokens: u.OutputTokens, TotalTokens: u.TotalTokens,
		CacheReadTokens: u.CacheReadTokens, TokPerSec: u.TokPerSec, HasRate: u.HasRate,
	}
}

// gyRealProduct 是产品形状的一帧装配（真插件，不是探针）。
type gyRealProduct struct {
	kernel     *uikernel.Kernel
	theme      *pluginapi.Theme
	tree       *ui.Tree
	feat       ui.Scope
	opts       uikernel.ShellOptions
	dispatched map[pluginapi.Slot]int
}

// newGYRealProduct 装配产品插件集并解出这一帧的壳选项（official 档位）。
func newGYRealProduct(t *testing.T) *gyRealProduct {
	t.Helper()
	tier := uikernel.NewShellTier()
	tier.Set(true)
	p := &gyRealProduct{
		kernel:     uikernel.New("gy-real-chain"),
		tree:       &ui.Tree{},
		dispatched: map[pluginapi.Slot]int{},
	}
	p.kernel.Root().Provide(uikernel.KeyShellTier, tier)
	if err := p.kernel.Load(
		&dscore.Plugin{Dark: true}, &dsshell.Plugin{}, &dssidebar.Plugin{},
		&dsconversation.Plugin{}, &dscomposer.Plugin{}, &dsgoal.Plugin{},
		&dsquestions.Plugin{}, &dsstatusbar.Plugin{},
	); err != nil {
		t.Fatalf("装配产品插件集失败：%v", err)
	}
	p.theme = p.kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.feat = p.tree.Feature("gy-real-chain")
	m := &model{kernel: p.kernel, cur: "s-real"}
	p.opts = shellOptionsFor(m, p.kernel, 1182, 832)
	if !p.opts.BridgeLegacy {
		t.Fatal("旧 sidebar/正文还占着槽，整列兼容桥必须开着")
	}
	if p.opts.BridgeLegacyComposer {
		t.Fatal("官方 composer 座位有占用方时 composer 不该走兼容桥")
	}
	return p
}

// paint 画一帧产品路径（真 flex + 真 canvas），返回画布与每槽派发次数。
//
// goal 走 `Actions.Goal` 闭包（不是快照字段）：官方 GoalBar 读的是会话投影，
// 本仓壳把它包成每帧调用的闭包（`cmd/app/main.go` 的 `Act.Goal`）。
func (p *gyRealProduct) paint(t *testing.T, snap pluginapi.Snapshot, goal string) (*ui.Canvas, []string, map[pluginapi.Slot]int) {
	t.Helper()
	canvas := &ui.Canvas{}
	canvas.SetTree(p.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: p.theme, Data: snap, Tree: p.tree, Feat: p.feat, Canvas: canvas,
		Bounds:    ui.Rect{Width: 1182, Height: 832},
		MainWidth: 902,
		Geometry:  pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902},
		Has:       p.kernel.Has,
		Act:       pluginapi.Actions{Goal: func() string { return goal }, Send: func(string) {}, Cancel: func() {}},
	}
	p.dispatched = map[pluginapi.Slot]int{}
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(p.theme), env.Bounds, func(f *flex.Frame) {
		uikernel.FirstScreen(f, env, p.opts, func(slot pluginapi.Slot) {
			gyRealDrawSlot(f, p.kernel, slot, env, p.dispatched)
		})
	})
	for _, painter := range p.kernel.AllPainters() {
		if painter.AfterDraw != nil {
			painter.AfterDraw(canvas, env)
		}
	}
	texts := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		texts = append(texts, text.Value)
	}
	return canvas, texts, p.dispatched
}

// gyRealDrawSlot 与 `uikernel.Kernel.DrawSlot` 同义（Draw → Children 转发），
// 额外把**每个槽被派发几次**记下来。
//
// 为什么不用 `kernel.DrawSlot`：它把 `Children` 的转发交回给内核自己的递归闭包，
// 调用方拿不到子槽的派发计数——而本测试要断言的正是「官方子槽每帧 1 次」。
// 语义逐行对齐（见 `uikernel/kernel.go` 的 drawSlot），只多了计数。
func gyRealDrawSlot(f *flex.Frame, k *uikernel.Kernel, slot pluginapi.Slot, env *pluginapi.Env, counts map[pluginapi.Slot]int) {
	counts[slot]++
	for _, painter := range k.Painters(slot) {
		if painter.Draw != nil {
			painter.Draw(f, env)
		}
		if painter.Children != nil {
			painter.Children(f, env, func(next pluginapi.Slot) {
				gyRealDrawSlot(f, k, next, env, counts)
			})
		}
	}
}

// waitPendingQuestion 等宿主那边出现一条**还在等**的 `ask_user_question` 消息。
//
// 判据与 `plugins/dsquestions.pending` 同源：消息流最后一段里出现
// `role=tool`、`tool=ask_user_question`，且它之后**没有** user 消息。
func waitPendingQuestion(t *testing.T, client *apiclient.Client, sessionID string, timeout time.Duration) (*apiclient.LoadResult, []apiclient.Message) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		load, err := client.Load(ctx, sessionID, 0)
		cancel()
		if err == nil {
			for i := len(load.Messages) - 1; i >= 0; i-- {
				m := load.Messages[i]
				if m.Role == "user" {
					break // 提问之后有人说过话 = 已经答过了
				}
				if m.Role == "tool" && m.Tool == "ask_user_question" {
					return load, load.Messages
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("%s 内没等到待答问题（真模型没调 ask_user_question）", timeout)
	return nil, nil
}

// TestRealChainGYGoalQuestionsStatusbar 真宿主 + 真 apiclient + 官方三座位。
func TestRealChainGYGoalQuestionsStatusbar(t *testing.T) {
	host := startRealHost(t)
	client, _ := connectRealGUI(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	sessionID, err := client.Create(ctx, host.home, "workspace-write", "gy-goal-parts")
	cancel()
	if err != nil {
		t.Fatalf("session.create 失败：%v", err)
	}
	t.Logf("真实会话 id=%s（home=%s）", sessionID, host.home)

	// ── ① 造一个活跃 goal（宿主 session.setGoal 的插件胶水）。
	var goalOut map[string]any
	setCtx, setCancel := context.WithTimeout(context.Background(), 20*time.Second)
	err = client.Call(setCtx, "session.setGoal", map[string]any{"id": sessionID, "goal": gyRealGoal}, &goalOut)
	setCancel()
	if err != nil {
		t.Fatalf("session.setGoal 失败：%v", err)
	}
	load, err := client.Load(context.Background(), sessionID, 0)
	if err != nil {
		t.Fatalf("session.load 失败：%v", err)
	}
	if load.Goal != gyRealGoal {
		t.Fatalf("宿主读回 goal = %q，want %q（活跃 goal 没落盘）", load.Goal, gyRealGoal)
	}
	t.Logf("真宿主 goal=%q", load.Goal)

	// ── ② 造一个待答问题：真模型真调 ask_user_question。
	promptCtx, promptCancel := context.WithTimeout(context.Background(), 60*time.Second)
	if _, err := client.Prompt(promptCtx, sessionID, gyRealAskPrompt); err != nil {
		promptCancel()
		t.Fatalf("session.prompt 失败：%v", err)
	}
	promptCancel()
	load, msgs := waitPendingQuestion(t, client, sessionID, 240*time.Second)
	if load.Goal != gyRealGoal {
		t.Fatalf("问题落盘后 goal 变了：%q", load.Goal)
	}
	last := msgs[len(msgs)-1]
	t.Logf("待答问题真值：tool=%q args=%.120s（共 %d 条消息）", last.Tool, last.Args, len(msgs))

	// ── ③④ 真画一帧 `-shell official` 的产品形状。
	snap := pluginapi.Snapshot{
		Cur: sessionID, CWD: host.home, Status: "已连接",
		Msgs:          msgs,
		UsageTotal:    gyUsage(load.UsageTotal),
		Usage:         gyUsage(load.Usage),
		ContextTokens: load.ContextTokens,
		ContextWindow: load.ContextWindow,
		Model:         load.Model,
		Permission:    load.Permission,
		Total:         load.Total,
	}
	canvas, texts, counts := newGYRealProduct(t).paint(t, snap, load.Goal)
	if got := counts[uikernel.OffConvComposer]; got != 1 {
		t.Fatalf("conversation.composer 派发次数 = %d，want 1（counts=%v）", got, counts)
	}
	if got := counts[pluginapi.SlotMainComposer]; got != 0 {
		t.Fatalf("官方档位下 main.composer 被派发 %d 次（双画）：%v", got, counts)
	}
	for _, want := range []string{"目标", gyRealGoal, "在等你回答", "先做哪个？", "活动 未知"} {
		found := false
		for _, text := range texts {
			if text == want || strings.HasPrefix(text, want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("真机画布缺 %q：%v", want, texts)
		}
	}
	usagePills := 0
	for _, text := range texts {
		if strings.HasPrefix(text, "总量 ") || strings.HasPrefix(text, "↑") || strings.HasPrefix(text, "↓") {
			usagePills++
		}
	}
	if usagePills == 0 {
		t.Fatalf("状态栏没画出用量真值（load.UsageTotal=%+v）：%v", load.UsageTotal, texts)
	}
	t.Logf("真机状态栏：usage 胶囊 %d 枚，texts=%v", usagePills, texts)

	// ── ⑤ 截图（真 headless 文本测量，真画布 → PNG）。
	dir := os.Getenv("GY_CAPTURE_DIR")
	if dir == "" {
		dir = "D:/DeepSeek Harness/default-workspace/DSH-GO/tmp/GO8-RC2/_gy"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建截图目录失败：%v", err)
	}
	shot := filepath.Join(dir, fmt.Sprintf("gy-official-web-%d.png", time.Now().Unix()))

	product := newGYRealProduct(t)
	shotTree := &ui.Tree{}
	shotFeat := shotTree.Feature("gy-shot")
	var invalidator app.Invalidator
	config := app.Config{
		Title: "gy-official", Width: 1182, Height: 832, Tree: shotTree,
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
			Act:       pluginapi.Actions{Goal: func() string { return load.Goal }},
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
	if err := session.Screenshot(shot); err != nil {
		t.Fatalf("截图失败：%v", err)
	}
	if info, err := os.Stat(shot); err != nil || info.Size() == 0 {
		t.Fatalf("截图文件无效 path=%s info=%v err=%v", shot, info, err)
	}
	_ = canvas
	t.Logf("GY_CAPTURE path=%s", shot)
}
