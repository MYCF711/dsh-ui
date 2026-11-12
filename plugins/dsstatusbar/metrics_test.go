package dsstatusbar

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
)

func int64ptr(v int64) *int64 { return &v }

func TestActivityPillRequiresOfficialWholeLogStats(t *testing.T) {
	tests := []struct {
		name  string
		stats *apiclient.SessionStats
		want  string
	}{
		{name: "projection 缺席", stats: nil, want: ""},
		{name: "没有 assistant step", stats: &apiclient.SessionStats{Turns: 4}, want: ""},
		// 官方 zh 文案 `stats.counts` = `{turns} 轮 {steps} 步`（locale.ts:84）：
		// 两个计数之间是空格，不是分隔符。
		{name: "唯一 turn 与 assistant step", stats: &apiclient.SessionStats{Turns: 2, Steps: 3}, want: "2 轮 3 步"},
		// 官方 `formatTokensPerSecond`：≥10 取整（40.0 → "40 tok/s"）。
		{name: "decode 真值速率", stats: &apiclient.SessionStats{Turns: 2, Steps: 3, DecodeTokens: 200, DecodeMs: 5000}, want: "2 轮 3 步 · 40 tok/s"},
		// <10 保留一位小数（2.2 不能被取整成 2）。
		{name: "低速保留一位小数", stats: &apiclient.SessionStats{Turns: 1, Steps: 2, DecodeTokens: 22, DecodeMs: 10000}, want: "1 轮 2 步 · 2.2 tok/s"},
		// 真实 0 token 速率是"报了 0"，不是未知——与"缺 decodeMs"是两件事。
		{name: "真实零速率", stats: &apiclient.SessionStats{Turns: 1, Steps: 1, DecodeMs: 5000}, want: "1 轮 1 步 · 0 tok/s"},
		{name: "缺 decodeMs 不编速率", stats: &apiclient.SessionStats{Turns: 2, Steps: 3, DecodeTokens: 200}, want: "2 轮 3 步"},
		// 八字段全给（含 llm/tool/ttft）时读数不变：官方 ActivityPill 的**读数**
		// 只有 counts + 速率，llmMs/toolMs/ttftMs/ttftSteps 只在 dialog 里出现。
		{
			name: "八字段齐全",
			stats: &apiclient.SessionStats{
				Turns: 3, Steps: 4, LLMMs: 1500, ToolMs: 800, TTFTMs: 120,
				TTFTSteps: 2, DecodeMs: 5000, DecodeTokens: 204,
			},
			// 204/5 = 40.8 → 官方取整 41。
			want: "3 轮 4 步 · 41 tok/s",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := activityText(tc.stats); got != tc.want {
				t.Fatalf("activityText() = %q，want %q", got, tc.want)
			}
		})
	}
}

// TestActivitySegmentTwoStates 钉住「有投影显示真值 / 没投影写未知」两态，
// 并反向断言**绝不出现 0 轮 0 步**（未知不是零）。
func TestActivitySegmentTwoStates(t *testing.T) {
	unknown := pluginapi.Snapshot{Cur: "s"}
	if got := activitySegment(unknown); got != zhActivityUnknown {
		t.Fatalf("没有 sessionStats 投影时 = %q，want %q", got, zhActivityUnknown)
	}
	for _, banned := range []string{"0 轮", "0 步", "tok/s"} {
		if strings.Contains(activitySegment(unknown), banned) {
			t.Fatalf("没有投影时编造了 %q：%q", banned, activitySegment(unknown))
		}
	}

	// 投影存在但 steps=0（空会话）：同样是「未知」，不是 "0 轮 0 步"。
	empty := pluginapi.Snapshot{Cur: "s", SessionStats: &apiclient.SessionStats{}}
	if got := activitySegment(empty); got != zhActivityUnknown {
		t.Fatalf("空会话（steps=0）时 = %q，want %q", got, zhActivityUnknown)
	}

	known := pluginapi.Snapshot{Cur: "s", SessionStats: &apiclient.SessionStats{
		Turns: 2, Steps: 3, DecodeMs: 5000, DecodeTokens: 200,
	}}
	if got := activitySegment(known); got != "2 轮 3 步 · 40 tok/s" {
		t.Fatalf("有投影时 = %q，want 官方读数", got)
	}
	if strings.Contains(activitySegment(known), "未知") {
		t.Fatalf("有真值时仍写「未知」：%q", activitySegment(known))
	}
}

func TestWindowMessagesNeverBecomeOfficialTurnsOrSteps(t *testing.T) {
	d := pluginapi.Snapshot{Msgs: []apiclient.Message{
		{Role: "assistant"}, {Role: "assistant"}, {Role: "tool"},
	}}
	text := strings.Join(usageSegments(d), " · ")
	for _, banned := range []string{"轮", "步", "tok/s"} {
		if strings.Contains(text, banned) {
			t.Fatalf("窗口消息被冒充成官方 Activity：%q 含 %q", text, banned)
		}
	}
}

func TestCacheHitPercentNeedsEveryDenominatorBucket(t *testing.T) {
	full := &tokenUsage{
		UncachedInputTokens: int64ptr(40),
		CacheReadTokens:     int64ptr(50),
		CacheWriteTokens:    int64ptr(10),
	}
	tests := []struct {
		name  string
		usage *tokenUsage
		want  string
	}{
		{name: "projection 缺席", usage: nil, want: ""},
		{name: "缺 uncachedInput", usage: &tokenUsage{CacheReadTokens: int64ptr(50), CacheWriteTokens: int64ptr(10)}, want: ""},
		{name: "缺 cacheRead", usage: &tokenUsage{UncachedInputTokens: int64ptr(40), CacheWriteTokens: int64ptr(10)}, want: ""},
		{name: "缺 cacheWrite", usage: &tokenUsage{UncachedInputTokens: int64ptr(40), CacheReadTokens: int64ptr(50)}, want: ""},
		{name: "分母为零", usage: &tokenUsage{UncachedInputTokens: int64ptr(0), CacheReadTokens: int64ptr(0), CacheWriteTokens: int64ptr(0)}, want: ""},
		{name: "官方三项分母", usage: full, want: "50%"},
		{name: "真实零命中不是未知", usage: &tokenUsage{UncachedInputTokens: int64ptr(100), CacheReadTokens: int64ptr(0), CacheWriteTokens: int64ptr(0)}, want: "0%"},
		{name: "全命中", usage: &tokenUsage{UncachedInputTokens: int64ptr(0), CacheReadTokens: int64ptr(100), CacheWriteTokens: int64ptr(0)}, want: "100%"},
		{name: "部分命中不四舍五入成一百", usage: &tokenUsage{UncachedInputTokens: int64ptr(1), CacheReadTokens: int64ptr(999), CacheWriteTokens: int64ptr(0)}, want: "99.9%"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cacheHitPercent(tc.usage); got != tc.want {
				t.Fatalf("cacheHitPercent() = %q，want %q", got, tc.want)
			}
		})
	}
}

func TestContextPercentNeedsUsedTokensAndWindow(t *testing.T) {
	tests := []struct {
		name     string
		pressure *contextPressure
		want     string
	}{
		{name: "projection 缺席", pressure: nil, want: ""},
		{name: "只有 usedTokens", pressure: &contextPressure{UsedTokens: int64ptr(2000)}, want: ""},
		{name: "只有 contextWindow", pressure: &contextPressure{ContextWindow: int64ptr(10000)}, want: ""},
		{name: "容量为零", pressure: &contextPressure{UsedTokens: int64ptr(2000), ContextWindow: int64ptr(0)}, want: ""},
		{name: "真实占用", pressure: &contextPressure{UsedTokens: int64ptr(2000), ContextWindow: int64ptr(10000)}, want: "20%"},
		{name: "真实零占用不是未知", pressure: &contextPressure{UsedTokens: int64ptr(0), ContextWindow: int64ptr(10000)}, want: "0%"},
		{name: "官方上限一百", pressure: &contextPressure{UsedTokens: int64ptr(12000), ContextWindow: int64ptr(10000)}, want: "100%"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := contextPercent(tc.pressure); got != tc.want {
				t.Fatalf("contextPercent() = %q，want %q", got, tc.want)
			}
		})
	}
}

func TestConnectionStatePriority(t *testing.T) {
	approval := &pluginapi.Approval{}
	tests := []struct {
		name      string
		data      pluginapi.Snapshot
		wantLabel string
		wantTone  connectionTone
	}{
		{name: "连接正常", data: pluginapi.Snapshot{}, wantLabel: "就绪", wantTone: connectionReady},
		{name: "待审批", data: pluginapi.Snapshot{Approval: approval}, wantLabel: "待审批", wantTone: connectionApproval},
		{name: "断线优先于审批", data: pluginapi.Snapshot{Disconnected: true, Approval: approval}, wantLabel: "已断开", wantTone: connectionDisconnected},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			label, tone := connectionState(tc.data)
			if label != tc.wantLabel || tone != tc.wantTone {
				t.Fatalf("connectionState() = (%q, %v)，want (%q, %v)", label, tone, tc.wantLabel, tc.wantTone)
			}
		})
	}
}

func TestOfficialDockGeometryDoesNotReuseLegacy24OrCaption40(t *testing.T) {
	if dockPaddingTop != 4 || dockGap != 12 {
		t.Fatalf("dock padding/gap = %v/%v，官方 4/12", dockPaddingTop, dockGap)
	}
	if pillLineHeight != 20 || pillPadY != 1 || pillPadX != 8 || pillGap != 6 {
		t.Fatalf("pill 规格 = line %v pad %v/%v gap %v，官方 20、1/8、6", pillLineHeight, pillPadY, pillPadX, pillGap)
	}
	if got, want := dockHeight([]string{"总量 150"}), float32(26); got != want {
		t.Fatalf("非空 dock 高 = %v，官方 4+20+2 = %v；不是旧 bottom 24 或 caption 40", got, want)
	}
	if got := dockHeight(nil); got != 0 {
		t.Fatalf("空 dock 高 = %v，必须不占位", got)
	}
	if dockHeight([]string{"x"}) == 24 || dockHeight([]string{"x"}) == 40 {
		t.Fatal("旧 shell.bottom 24 / Windows caption 40 被误用为官方 dock 高")
	}
	if contextMeterSize != 14 || contextMeterStroke != 2 {
		t.Fatalf("ContextMeter = %v/%v，官方 SVG 14、stroke 2", contextMeterSize, contextMeterStroke)
	}
}

func TestPanelsSeparateComposerDockFromConnectionBottom(t *testing.T) {
	p := &Plugin{}
	panels := p.Panels()
	// 三个面：legacy dock / 连接行 / 官方 dock（见 official_dock.go）。
	if len(panels) != 3 {
		t.Fatalf("Panels 数 = %d，want legacy dock + connection + 官方 dock 三个", len(panels))
	}
	byID := map[string]pluginapi.Painter{}
	for _, panel := range panels {
		byID[panel.ID] = panel
	}
	if byID["dock"].Slot != pluginapi.SlotMainComposer || byID["dock"].Order <= 10 {
		t.Fatalf("legacy dock 必须在 composer(order10) 后：%+v", byID["dock"])
	}
	if byID["connection"].Slot != pluginapi.SlotShellBottom {
		t.Fatalf("连接状态必须保留独立 shell.bottom 出口：%+v", byID["connection"])
	}
	// 官方座位：挂在 chain 座位 `conversation.composer`，order 必须在输入卡片(10)之后。
	official := byID["dock-official"]
	if official.Slot != uikernel.OffConvComposer {
		t.Fatalf("官方 dock 座位 = %q，want conversation.composer", official.Slot)
	}
	if official.Order <= 10 {
		t.Fatalf("官方 dock order = %d，必须大于 composer 的 10（dock 在卡片下方）", official.Order)
	}
}
