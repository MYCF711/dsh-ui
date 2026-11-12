package dsstatusbar

// official_dock_test.go —— 状态栏**官方座位**（`conversation.composer.dock`）的断言。
//
// 四条判据（task-47 §任务 2/4）：
//
//	① 注册在官方 chain 座位 `conversation.composer`（官方 `conversation.composer.dock`
//	   今天由 composer 占用方转发，本仓占用方还没转发 → 挂同一 chain 座位，order 20
//	   表示"卡片下方"），每帧 1 次；
//	② **常驻**：有会话就画一行，两枚胶囊（活动 / 用量）永远在；
//	③ 缺字段写「未知」，**不写 0 / ↑0 / ↓0**（不编）；
//	④ 两个座位互斥（官方档位 legacy 不画，legacy 档位官方不画）。

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// drawSeat 模拟官方骨架 dispatch 一个槽（含 Children 转发）并计数。
// 语义与 `uikernel.Kernel.drawSlot` 一致，区别只是顺手记下每个槽派发几次。
func drawSeat(f ui.C, k *uikernel.Kernel, env *pluginapi.Env, slot pluginapi.Slot, counts map[pluginapi.Slot]int) {
	counts[slot]++
	for _, painter := range k.Painters(slot) {
		if painter.Draw != nil {
			painter.Draw(f, env)
		}
		if painter.Children != nil {
			painter.Children(f, env, func(next pluginapi.Slot) {
				drawSeat(f, k, env, next, counts)
			})
		}
	}
}

// dockFrame 画一帧并返回画布文本与每槽派发次数。
func dockFrame(t *testing.T, tierOfficial, drawOfficial bool, data pluginapi.Snapshot) ([]string, map[pluginapi.Slot]int) {
	t.Helper()
	tier := uikernel.NewShellTier()
	tier.Set(tierOfficial)
	kernel := uikernel.New("statusbar-official-dock")
	kernel.Root().Provide(uikernel.KeyShellTier, tier)
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &Plugin{}); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len([]rune(value))) * size * 0.55 })
	env := &pluginapi.Env{
		Theme: theme, Data: data, Tree: tree, Feat: tree.Feature("statusbar-official"), Canvas: canvas,
		Bounds: gfx.Rect{Width: 1182, Height: 832}, MainWidth: 902,
		Geometry: pluginapi.Geometry{MainWidth: 902}, Has: kernel.Has,
	}
	counts := map[pluginapi.Slot]int{}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		if drawOfficial {
			drawSeat(f, kernel, env, uikernel.OffConvComposer, counts)
			return
		}
		drawSeat(f, kernel, env, pluginapi.SlotMainComposer, counts)
	})
	texts := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		texts = append(texts, text.Value)
	}
	return texts, counts
}

// ① 官方座位登记 + 每帧被派发一次 + 内容真值。
func TestStatusbarRegistersOfficialComposerSeat(t *testing.T) {
	p := &Plugin{}
	byID := map[string]pluginapi.Painter{}
	for _, painter := range p.Panels() {
		byID[painter.ID] = painter
	}
	official, ok := byID["dock-official"]
	if !ok {
		t.Fatal("缺官方座位 painter（dock-official）")
	}
	if official.Slot != uikernel.OffConvComposer {
		t.Fatalf("官方座位 = %q，want conversation.composer（官方 conversation.composer.dock 今天不可达）", official.Slot)
	}
	if official.Order != dockOfficialOrder || dockOfficialOrder != 20 {
		t.Fatalf("官方 order = %d，want 20（排在输入卡片 10 之后 = 卡片下方）", official.Order)
	}

	data := pluginapi.Snapshot{
		Cur: "s", UsageTotal: pluginapi.Usage{InputTokens: 12000, OutputTokens: 345, TotalTokens: 12345},
		ContextTokens: 2000, ContextWindow: 10000,
	}
	texts, counts := dockFrame(t, true, true, data)
	if counts[uikernel.OffConvComposer] != 1 {
		t.Fatalf("conversation.composer 派发次数 = %d，want 1", counts[uikernel.OffConvComposer])
	}
	for _, want := range []string{"活动 未知", "总量 12.3k"} {
		found := false
		for _, text := range texts {
			if text == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("官方 dock 缺 %q：texts=%v", want, texts)
		}
	}
}

// ②③ 常驻 + 未知：没有任何真值时三枚胶囊仍在，且不编 0。
func TestOfficialDockStaysResidentAndNeverInventsZero(t *testing.T) {
	segs := officialSegments(pluginapi.Snapshot{Cur: "s"})
	// 上下文归 composer 的 ContextMeter（GR §3 的分工），这里只有两枚。
	if len(segs) != 2 {
		t.Fatalf("官方 dock 两枚胶囊（活动/用量），得到 %d：%v", len(segs), segs)
	}
	want := []string{zhActivityUnknown, zhUsageUnknown}
	for i, segment := range segs {
		if segment != want[i] {
			t.Fatalf("第 %d 枚 = %q，want %q", i, segment, want[i])
		}
	}
	joined := strings.Join(segs, " · ")
	for _, banned := range []string{"0 tok", "↑0", "↓0", "总量 0", "0 轮", "0 步", "上下文"} {
		if strings.Contains(joined, banned) {
			t.Fatalf("缺字段时出现了编造的数字 %q：%q", banned, joined)
		}
	}

	// 真值出现时按真值写（不是恒「未知」）。
	known := officialSegments(pluginapi.Snapshot{
		Cur:        "s",
		UsageTotal: pluginapi.Usage{InputTokens: 12000, OutputTokens: 345, TotalTokens: 12345},
	})
	if known[1] != "总量 12.3k" {
		t.Fatalf("有真值时不按真值写：%v", known)
	}
	// 只有一侧真值时保留可证明的那一侧，不给另一侧补 0。
	oneSide := officialSegments(pluginapi.Snapshot{
		Cur: "s", UsageTotal: pluginapi.Usage{InputTokens: 12000},
	})
	if oneSide[1] != "↑12.0k" {
		t.Fatalf("只有 input 真值时用量段 = %q，want「↑12.0k」（不补 ↓0）", oneSide[1])
	}
}

// ②' 有投影时活动段写**官方读数**（真消费 sessionStats），不是恒「未知」。
func TestOfficialDockConsumesSessionStats(t *testing.T) {
	data := pluginapi.Snapshot{
		Cur:          "s",
		SessionStats: &apiclient.SessionStats{Turns: 2, Steps: 3, DecodeMs: 5000, DecodeTokens: 200},
		UsageTotal:   pluginapi.Usage{TotalTokens: 12345},
	}
	texts, _ := dockFrame(t, true, true, data)
	want := "2 轮 3 步 · 40 tok/s"
	found := false
	for _, text := range texts {
		if text == want {
			found = true
		}
		if text == zhActivityUnknown {
			t.Fatalf("有 sessionStats 投影时仍写「未知」：%v", texts)
		}
	}
	if !found {
		t.Fatalf("官方 dock 缺真实活动读数 %q：texts=%v", want, texts)
	}

	// 没有投影：写「未知」，且不出现任何编造的轮/步数字。
	unknown, _ := dockFrame(t, true, true, pluginapi.Snapshot{Cur: "s"})
	found = false
	for _, text := range unknown {
		if text == zhActivityUnknown {
			found = true
		}
		for _, banned := range []string{"0 轮", "0 步", "tok/s"} {
			if strings.Contains(text, banned) {
				t.Fatalf("没有投影时编造了 %q：%v", banned, unknown)
			}
		}
	}
	if !found {
		t.Fatalf("没有投影时必须写「活动 未知」：texts=%v", unknown)
	}
}

// ④ 两个座位互斥；且重复转发只画一次。
func TestStatusbarSeatsAreMutuallyExclusive(t *testing.T) {
	data := pluginapi.Snapshot{
		Cur: "s", UsageTotal: pluginapi.Usage{TotalTokens: 12345}, ContextTokens: 2000,
	}
	// 官方档位 + 画 legacy 座位 → 空白。
	texts, _ := dockFrame(t, true, false, data)
	if len(texts) != 0 {
		t.Fatalf("官方档位下 legacy dock 还画了东西（双画）：%v", texts)
	}
	// legacy 档位 + 画官方座位 → 空白。
	texts, _ = dockFrame(t, false, true, data)
	if len(texts) != 0 {
		t.Fatalf("legacy 档位下官方 dock 还画了东西：%v", texts)
	}
	// legacy 档位 + 画 legacy 座位 → 有旧内容（旧路径没坏）。
	texts, _ = dockFrame(t, false, false, data)
	if len(texts) == 0 {
		t.Fatal("legacy 档位下 legacy dock 没画出任何文本")
	}
}

// ④' 每帧重画一次：两帧各出现一枚用量胶囊（像素/文本都不会累积）。
func TestOfficialDockRedrawsOncePerFrame(t *testing.T) {
	data := pluginapi.Snapshot{Cur: "s", UsageTotal: pluginapi.Usage{TotalTokens: 12345}}
	dockFrame(t, true, true, data)
	texts, _ := dockFrame(t, true, true, data)
	var usagePills int
	for _, text := range texts {
		if strings.HasPrefix(text, "总量 ") {
			usagePills++
		}
	}
	if usagePills != 1 {
		t.Fatalf("第二帧用量胶囊画了 %d 次，want 1", usagePills)
	}
}

// TestOfficialDockActivityIsNotClipped 钉住「活动」读数**完整可见**。
//
// 为什么值得一条：读数被省略号截断时用户看到的是「1 轮 2 步 · 571 tok…」——
// 数字对了但单位/含义丢了，等于没显示。这个位置的可用宽度是整窗（居中、两枚
// 胶囊），没有任何理由截断。
func TestOfficialDockActivityIsNotClipped(t *testing.T) {
	// 用真机跑出来的形状：turns=1 steps=2 decodeMs=21 decodeTokens=12 → 571.4 tok/s。
	data := pluginapi.Snapshot{Cur: "s", SessionStats: &apiclient.SessionStats{
		Turns: 1, Steps: 2, DecodeMs: 21, DecodeTokens: 12,
	}}
	texts, _ := dockFrame(t, true, true, data)
	var activity string
	for _, text := range texts {
		if strings.Contains(text, " 轮 ") {
			activity = text
		}
	}
	if activity == "" {
		t.Fatalf("没有活动读数：texts=%v", texts)
	}
	if strings.Contains(activity, "…") || strings.Contains(activity, "...") {
		t.Fatalf("活动读数被截断：%q", activity)
	}
	if want := "1 轮 2 步 · 571 tok/s"; activity != want {
		t.Fatalf("活动读数 = %q，want %q", activity, want)
	}
}
