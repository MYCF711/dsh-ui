package dsstatusbar

// official_stats_format_test.go —— 弹窗文案与数值格式（官方 locale 逐字移植）。
//
// 官方口径（`dsh-client-ui-chat/src/client/locale.ts` + `StatsPills.tsx`）：
//
//	stats.counts             {turns} 轮 {steps} 步        locale.ts:84
//	duration.compactSeconds  {seconds}秒                  locale.ts:81
//	duration.compactMinutes  {minutes}分{seconds}秒        locale.ts:82
//	message.tokensPerSecond  {tps} tok/s                  locale.ts:192
//	formatDuration           <60s 一位小数（整数不补 .0）；>=60s 整秒拆成分秒
//	formatTokensPerSecond    >=10 取整、<10 一位小数（metrics.go 的 rate）
//
// 这一组钉的是**格式**，与"有没有真值"无关（后者见 official_stats_dialog_test.go）。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// TestStatsDurationFollowsOfficialFormatDuration 钉住官方 formatDuration。
func TestStatsDurationFollowsOfficialFormatDuration(t *testing.T) {
	cases := []struct {
		name string
		ms   float64
		want string
	}{
		// <60s：`{Math.round(s*10)/10}秒`，整数不补 .0（JS String(3) = "3"）。
		{"0ms", 0, "0秒"},
		{"1ms", 1, "0秒"},
		{"61ms（工具真值）", 61, "0.1秒"},
		{"999ms", 999, "1秒"},
		{"1000ms", 1000, "1秒"},
		{"1527.5ms（ttft 平均）", 1527.5, "1.5秒"},
		{"3267ms（模型用时）", 3267, "3.3秒"},
		{"45200ms", 45200, "45.2秒"},
		{"59999ms（四舍五入到 60 秒）", 59999, "60秒"},
		// >=60s：`{⌊w/60⌋}分{w%60}秒`，w = Math.round(s)。
		{"60000ms", 60000, "1分0秒"},
		{"61000ms", 61000, "1分1秒"},
		{"90000ms", 90000, "1分30秒"},
		{"3723000ms", 3723000, "62分3秒"},
		// 负值/NaN 是防御分支：不写 `NaN秒`，夹到 0。
		{"负值", -5, "0秒"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := statsDuration(tc.ms); got != tc.want {
				t.Fatalf("statsDuration(%v) = %q，want %q", tc.ms, got, tc.want)
			}
		})
	}
}

// TestStatsCountsTextIsOfficialLocale 钉住 `stats.counts`（计数之间是空格）。
func TestStatsCountsTextIsOfficialLocale(t *testing.T) {
	got := statsCountsText(&apiclient.SessionStats{Turns: 2, Steps: 3})
	if want := "2 轮 3 步"; got != want {
		t.Fatalf("statsCountsText = %q，want %q（官方 locale.ts:84 是空格，不是 ·）", got, want)
	}
	if got := statsCountsText(nil); got != "" {
		t.Fatalf("没有投影时 counts = %q，want 空串（由调用方决定写「未知」）", got)
	}
}

// TestStatsDialogRowsGateLikeOfficial 钉住四条计时的**出现条件与顺序**。
func TestStatsDialogRowsGateLikeOfficial(t *testing.T) {
	cases := []struct {
		name  string
		stats *apiclient.SessionStats
		want  []string // 逐行 "标签=读数"
	}{
		{
			name:  "投影缺席",
			stats: nil,
			want:  nil,
		},
		{
			name:  "没有 step",
			stats: &apiclient.SessionStats{Turns: 3},
			want:  nil,
		},
		{
			name:  "四个计时全 0（官方退回 PlainPill）",
			stats: &apiclient.SessionStats{Turns: 1, Steps: 2},
			want:  nil,
		},
		{
			name:  "只有模型用时",
			stats: &apiclient.SessionStats{Turns: 1, Steps: 1, LLMMs: 3267},
			want:  []string{"模型用时=3.3秒"},
		},
		{
			name:  "只有工具用时",
			stats: &apiclient.SessionStats{Turns: 1, Steps: 1, ToolMs: 61000},
			want:  []string{"工具调用用时=1分1秒"},
		},
		{
			name:  "只有 TTFT（摊到 ttftSteps）",
			stats: &apiclient.SessionStats{Turns: 1, Steps: 2, TTFTMs: 3055, TTFTSteps: 2},
			want:  []string{"首 token 平均（TTFT）=1.5秒"},
		},
		{
			name:  "只有速率（decodeMs 决定它存不存在）",
			stats: &apiclient.SessionStats{Turns: 1, Steps: 1, DecodeMs: 212, DecodeTokens: 87},
			want:  []string{"输出速度（TPS）=410 tok/s"},
		},
		{
			name: "八字段齐全：四条全出现，顺序与官方 JSX 一致",
			stats: &apiclient.SessionStats{
				Turns: 2, Steps: 3, LLMMs: 3267, ToolMs: 61, TTFTMs: 3055,
				TTFTSteps: 2, DecodeMs: 212, DecodeTokens: 87,
			},
			want: []string{
				"模型用时=3.3秒",
				"工具调用用时=0.1秒",
				"首 token 平均（TTFT）=1.5秒",
				"输出速度（TPS）=410 tok/s",
			},
		},
		{
			name:  "decodeTokens 为 0 但 decodeMs > 0：真实 0 tok/s（不是未知）",
			stats: &apiclient.SessionStats{Turns: 1, Steps: 1, DecodeMs: 5000},
			want:  []string{"输出速度（TPS）=0 tok/s"},
		},
		{
			name:  "低速保留一位小数（官方 <10 不取整）",
			stats: &apiclient.SessionStats{Turns: 1, Steps: 1, DecodeMs: 10000, DecodeTokens: 22},
			want:  []string{"输出速度（TPS）=2.2 tok/s"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := statsDialogRows(tc.stats)
			if len(rows) != len(tc.want) {
				t.Fatalf("行数 = %d，want %d：%+v", len(rows), len(tc.want), rows)
			}
			for i, want := range tc.want {
				got := rows[i].Label + "=" + rows[i].Value
				if got != want {
					t.Fatalf("第 %d 行 = %q，want %q", i, got, want)
				}
			}
		})
	}
}

// TestStatsDialogTTFTAverageUsesFloatDivision 钉住"先除再格式化"（先取整会改边界）。
func TestStatsDialogTTFTAverageUsesFloatDivision(t *testing.T) {
	// 3051ms / 2 = 1525.5ms → 1.5255s → Math.round(15.255) = 15 → "1.5秒"
	// 若先整除掉成 1525ms → 1.525s → round(15.25) = 15 → 同样是 1.5秒，但边界
	// 以下是不同结果的地方：3xxx.5 情形。这里钉的是 4001/2 = 2000.5ms → 2.0005s
	// → round(20.005) = 20 → "2秒"；整除掉是 2000ms → "2秒"——两条相等，
	// 所以用 3999/2 = 1999.5ms → 1.9995s → round(19.995) = 20 → "2秒"。
	rows := statsDialogRows(&apiclient.SessionStats{Turns: 1, Steps: 1, TTFTMs: 3999, TTFTSteps: 2})
	if len(rows) != 1 || rows[0].Value != "2秒" {
		t.Fatalf("TTFT 平均 = %+v，want「2秒」", rows)
	}
	// 反向：1599ms / 2 = 799.5ms → 0.7995s → round(7.995) = 8 → "0.8秒"。
	rows = statsDialogRows(&apiclient.SessionStats{Turns: 1, Steps: 1, TTFTMs: 1599, TTFTSteps: 2})
	if len(rows) != 1 || rows[0].Value != "0.8秒" {
		t.Fatalf("TTFT 平均 = %+v，want「0.8秒」", rows)
	}
}
