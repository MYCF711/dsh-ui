// stats_view_test.go —— 官方 #65 统计视图的折叠与渲染断言。
//
// 判据：
//  1. 折叠只认 journal 原始帧：turn/start、step/start、assistant/text、
//     assistant/message(usage)、tool/call+tool/result、step/end；
//  2. 指标公式逐条对官方 stats-usage.ts / stats-fold.ts（context = input+cacheRead+cacheWrite，
//     TTFT = firstToken-start，model = message-start，TPS = output/decode）；
//  3. 渲染：11 列官方表头 + 最新一轮默认展开 + 点轮行能展开更早的轮。
package dsstatusbar

import (
	"encoding/json"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func statsFrame(seq int, typ string, time int64, data any) apiclient.Frame {
	raw, _ := json.Marshal(data)
	return apiclient.Frame{Seq: seq, Type: typ, Data: raw, Time: time}
}

// statsTwoTurnFrames 造两轮各一步的完整事件窗口（数值故意可心算）。
func statsTwoTurnFrames() []apiclient.Frame {
	return []apiclient.Frame{
		statsFrame(1, "turn/start", 900, map[string]any{"turn": 1}),
		statsFrame(2, "user/message", 950, map[string]any{
			"content": []map[string]any{{"type": "text", "text": "第一问"}},
			"source":  map[string]any{"kind": "user"},
		}),
		statsFrame(3, "step/start", 1000, map[string]any{"turn": 1, "step": 1}),
		statsFrame(4, "assistant/text", 1200, map[string]any{"turn": 1, "step": 1, "text": "答"}),
		statsFrame(5, "assistant/message", 1500, map[string]any{
			"turn": 1, "step": 1,
			"message": map[string]any{"role": "assistant", "content": []map[string]any{{"type": "text", "text": "第一答"}}},
			"usage":   map[string]any{"inputTokens": 100, "cacheReadTokens": 300, "cacheWriteTokens": 0, "outputTokens": 50},
		}),
		statsFrame(6, "tool/call", 1600, map[string]any{"turn": 1, "step": 1, "callId": "c1", "name": "bash"}),
		statsFrame(7, "tool/result", 1700, map[string]any{
			"turn": 1, "step": 1,
			"message": map[string]any{"role": "tool", "toolCallId": "c1", "content": []map[string]any{{"type": "text", "text": "ok"}}},
		}),
		statsFrame(8, "step/end", 1800, map[string]any{"turn": 1, "step": 1}),
		statsFrame(9, "turn/end", 1900, map[string]any{"reason": map[string]any{"kind": "completed"}}),

		statsFrame(10, "turn/start", 2000, map[string]any{"turn": 2}),
		statsFrame(11, "user/message", 2050, map[string]any{
			"content": []map[string]any{{"type": "text", "text": "第二问"}},
			"source":  map[string]any{"kind": "user"},
		}),
		statsFrame(12, "step/start", 2100, map[string]any{"turn": 2, "step": 1}),
		statsFrame(13, "assistant/text", 2300, map[string]any{"turn": 2, "step": 1, "text": "答2"}),
		statsFrame(14, "assistant/message", 2600, map[string]any{
			"turn": 2, "step": 1,
			"message": map[string]any{"role": "assistant", "content": []map[string]any{{"type": "text", "text": "第二答"}}},
			"usage":   map[string]any{"inputTokens": 200, "cacheReadTokens": 200, "outputTokens": 80},
		}),
		statsFrame(15, "step/end", 2900, map[string]any{"turn": 2, "step": 1}),
		statsFrame(16, "turn/end", 3000, map[string]any{"reason": map[string]any{"kind": "completed"}}),
	}
}

func TestFoldStatsComputesOfficialStepMetrics(t *testing.T) {
	model := foldStats(statsTwoTurnFrames())
	if len(model.Turns) != 2 {
		t.Fatalf("应折出 2 轮，实际 %d", len(model.Turns))
	}
	step := model.Turns[0].Steps[0]
	m := measureStep(step)
	if !m.HasCtx || m.Context != 400 {
		t.Fatalf("context 应为 100+300+0=400，实际 %+v", m)
	}
	if !m.HasOutput || m.Output != 50 {
		t.Fatalf("output 应为 50，实际 %+v", m)
	}
	if !m.HasTTFT || m.TTFT != 200 {
		t.Fatalf("TTFT 应为 1200-1000=200，实际 %+v", m)
	}
	if !m.HasModel || m.Model != 500 {
		t.Fatalf("model 应为 1500-1000=500，实际 %+v", m)
	}
	if !m.HasDecode || m.Decode != 300 {
		t.Fatalf("decode 应为 1500-1200=300，实际 %+v", m)
	}
	if !m.HasTPS || m.TPS < 166 || m.TPS > 167 {
		t.Fatalf("TPS 应为 50/0.3≈166.7，实际 %+v", m)
	}
	if !m.HasTool || m.Tool != 100 {
		t.Fatalf("tool 应为 1700-1600=100，实际 %+v", m)
	}
	if !m.HasWall || m.Wall != 800 {
		t.Fatalf("wall 应为 1800-1000=800，实际 %+v", m)
	}
	if !m.HasCache || m.CacheHit < 0.749 || m.CacheHit > 0.751 {
		t.Fatalf("cache hit 应为 300/400=0.75，实际 %+v", m)
	}
	if model.Turns[0].Prompt != "第一问" || model.Turns[0].PromptSource != "user" {
		t.Fatalf("第一轮触发应来自 user 提示词：%q / %q", model.Turns[0].Prompt, model.Turns[0].PromptSource)
	}
}

// drawStats 直接调用产品绘制函数（不经过槽），断言官方表头与轮/步文本。
func (h *statsDialogHarness) drawStats(t *testing.T, bounds gfx.Rect, data pluginapi.Snapshot, click string) *gfx.Canvas {
	t.Helper()
	canvas := &gfx.Canvas{}
	canvas.SetTree(h.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: h.theme, Data: data, Tree: h.tree, Feat: h.feat, Canvas: canvas,
		Bounds: bounds, MainWidth: bounds.Width,
		Geometry: pluginapi.Geometry{Width: bounds.Width, Height: bounds.Height, MainWidth: bounds.Width},
		Has:      h.kernel.Has,
	}
	if click != "" {
		canvas.BeginEventPass(h.tree.ID(h.feat, click), gfx.ClickInfo{})
	}
	pageTheme := dscore.VivaTheme(h.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, bounds, func(f ui.C) {
		ui.Column(f).FillWidth().FillHeight().Children(func() {
			h.plugin.drawStatsView(f, env)
		})
	})
	if click != "" {
		canvas.EndEventPass()
	}
	return canvas
}

func TestDrawStatsViewShowsOfficialHeadersAndNewestTurn(t *testing.T) {
	h := newStatsDialogHarness(t)
	data := pluginapi.Snapshot{Cur: "s1", Frames: statsTwoTurnFrames()}
	canvas := h.drawStats(t, gfx.Rect{Width: 1182, Height: 832}, data, "")
	texts := statsDialogTexts(canvas)
	for _, want := range []string{
		"轮次 / 步骤", "触发", "执行内容", "上下文", "输出", "缓存命中",
		"首 token", "模型用时", "输出速度", "工具用时", "总用时",
		"第 2 轮", "本窗口合计", "75.00%",
	} {
		found := false
		for _, got := range texts {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("统计视图缺少文本 %q；实际=%v", want, texts)
		}
	}
	// 默认 newest：最新一轮展开、更早一轮收起（"第 1 步"只属于展开的第二轮）。
	countSteps := func(values []string) int {
		n := 0
		for _, got := range values {
			if got == "第 1 步" {
				n++
			}
		}
		return n
	}
	if got := countSteps(texts); got != 1 {
		t.Fatalf("默认只应展开最新一轮（1 个 step 行），实际 %d；%v", got, texts)
	}
	// 点第一轮 → 展开它。
	canvas = h.drawStats(t, gfx.Rect{Width: 1182, Height: 832}, data, "stats-turn-1")
	// 点击那一帧只改状态；展开是下一帧的渲染（产品里由 Act.Invalidate 触发）。
	canvas = h.drawStats(t, gfx.Rect{Width: 1182, Height: 832}, data, "")
	texts = statsDialogTexts(canvas)
	if got := countSteps(texts); got != 2 {
		t.Fatalf("点第一轮后应有两个 step 行，实际 %d；%v", got, texts)
	}
}

func TestDrawStatsViewEmptyWindowHasNoFabricatedValues(t *testing.T) {
	h := newStatsDialogHarness(t)
	canvas := h.drawStats(t, gfx.Rect{Width: 1182, Height: 832}, pluginapi.Snapshot{Cur: "s1"}, "")
	texts := statsDialogTexts(canvas)
	for _, got := range texts {
		if got == "0" || got == "0 tok/s" || got == "0.00%" {
			t.Fatalf("空窗口不应出现编造数字 %q；实际=%v", got, texts)
		}
	}
	if len(texts) != 1 || texts[0] != "当前窗口里还没有可以测量的步骤。" {
		t.Fatalf("空窗口应只有官方空态一行；实际=%v", texts)
	}
}

// 机制断言：#65 要的就是「往 conversation.view 注册一个 order:40 的 stats 条目」。
func TestStatsViewIsRegisteredAsOfficialConversationView(t *testing.T) {
	h := newStatsDialogHarness(t)
	tabs := uikernel.ViewTabs(h.kernel)
	var found bool
	for _, tab := range tabs {
		if tab.Key == pluginapi.ViewStats {
			found = true
			if tab.Order != 40 {
				t.Fatalf("stats 视图 order=%d，官方是 40", tab.Order)
			}
			if tab.Label != "统计" {
				t.Fatalf("stats 视图 label=%q，官方 zh 是 统计", tab.Label)
			}
		}
	}
	if !found {
		t.Fatalf("conversation.view 名录里没有 stats：%+v", tabs)
	}
}
