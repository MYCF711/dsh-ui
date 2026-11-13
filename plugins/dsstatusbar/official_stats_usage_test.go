package dsstatusbar

// official_stats_usage_test.go —— 官方**用量弹窗**（UsagePill 的 DialogPill）的
// 四桶真值 / 格式 / 降级两态断言。
//
// 判据（task-59 §任务 3/4）：
//
//	① 四桶逐个渲染：只改一个桶，弹窗里对应的那一行必须跟着变（4 个子用例）；
//	② 行顺序与官方 JSX 一致：缓存命中 → 未缓存输入 → 缓存读取 →〔缓存写入〕→ 输出；
//	③ 键缺席降级：`TokenUsage == nil` → 读数「用量 未知」+ **没有命中区** + 点不开；
//	④ `0` vs 未知：四桶全 0（键在）→ 读数 `0 tok`（宿主说 0），
//	   与「用量 未知」是**两态**，且全 0 时官方不画弹窗（没有可点入口）；
//	⑤ 缓存写入为 0 时整行不画、非 0 时出现（官方 `:336` 的条件渲染）；
//	⑥ 与 `sessionStats.decodeTokens` 交叉一致：同一轮的输出桶就是 decodeTokens。
//
// 期望值**不调被测函数**：格式表（formatTokens / formatExactTokens /
// formatCacheHitPercent）在下面按官方 token-format.ts 的规则独立实现一遍。

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
)

// usageFull 是四桶都有值的那一份（缓存写入非 0 → 5 行全画）。
func usageFull() *apiclient.TokenUsage {
	return &apiclient.TokenUsage{
		UncachedInputTokens: 1500,
		OutputTokens:        123,
		CacheReadTokens:     4096,
		CacheWriteTokens:    7,
	}
}

// usageStats 是活动胶囊可点的那份八字段（借用 HE 的真机形状）。
func usageStats() *apiclient.SessionStats { return statsFullStats() }

// usageSnapshot 造一帧数据：官方 `tokenUsage` 投影 + 可点开的活动胶囊。
func usageSnapshot(usage *apiclient.TokenUsage) pluginapi.Snapshot {
	return pluginapi.Snapshot{
		Cur:          "s-usage",
		SessionStats: usageStats(),
		TokenUsage:   usage,
	}
}

// hjCompact 是官方 `formatTokens`（token-format.ts:9-15）的独立第二实现。
func hjCompact(value int64) string {
	scaled := func(candidate float64) string {
		if candidate >= 100 {
			return fmt.Sprintf("%d", int(math.Round(candidate)))
		}
		rounded := math.Round(candidate*10) / 10
		if rounded == math.Trunc(rounded) {
			return fmt.Sprintf("%d", int(rounded))
		}
		return fmt.Sprintf("%.1f", rounded)
	}
	switch {
	case value < 1_000:
		return fmt.Sprintf("%d", value)
	case value < 1_000_000:
		return scaled(float64(value)/1_000) + "K"
	default:
		return scaled(float64(value)/1_000_000) + "M"
	}
}

// hjExact 是官方 `formatExactTokens`（token-format.ts:23-30）的独立第二实现：
// 三位分组 + `number.groupSeparator`（= `,`）。
func hjExact(value int64) string {
	digits := fmt.Sprintf("%d", value)
	var out strings.Builder
	for i, digit := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(digit)
	}
	return out.String()
}

// hjCount 是官方 `exactCount` = `{exact} tok`（StatsPills.tsx:143-145）。
func hjCount(value int64) string { return hjExact(value) + " tok" }

// ① 数值格式逐条对齐官方 token-format.ts。
func TestUsageTokenFormatFollowsOfficial(t *testing.T) {
	compact := []struct {
		in   int64
		want string
	}{
		{0, "0"}, {517, "517"}, {999, "999"}, {1000, "1K"}, {1049, "1K"},
		{5726, "5.7K"}, {12345, "12.3K"}, {100000, "100K"}, {517000, "517K"},
		{999999, "1000K"}, {1000000, "1M"}, {1200000, "1.2M"},
	}
	for _, tc := range compact {
		if got := officialTokensCompact(tc.in); got != tc.want || got != hjCompact(tc.in) {
			t.Fatalf("formatTokens(%d) = %q，want %q（官方规则算 %q）", tc.in, got, tc.want, hjCompact(tc.in))
		}
	}
	exact := []struct {
		in   int64
		want string
	}{
		{0, "0"}, {12, "12"}, {999, "999"}, {1000, "1,000"},
		{5726, "5,726"}, {1234567, "1,234,567"},
	}
	for _, tc := range exact {
		if got := officialTokensExact(tc.in); got != tc.want || got != hjExact(tc.in) {
			t.Fatalf("formatExactTokens(%d) = %q，want %q（官方规则算 %q）", tc.in, got, tc.want, hjExact(tc.in))
		}
	}
	if got, want := usageCountText(5726), "5,726 tok"; got != want {
		t.Fatalf("exactCount(5726) = %q，want %q", got, want)
	}
}

// ②' 缓存命中率是**客户端算的整数百分比**，边界逐条对齐官方
// `formatCacheHitPercent`（token-format.ts:32-98）。
func TestUsageCacheHitPercentFollowsOfficial(t *testing.T) {
	cases := []struct {
		name                string
		cacheRead, unbilled int64
		want                string
	}{
		{"四桶那份", 4096, 1500, "73"},    // 4096/5603 = 73.1%
		{"全未命中", 0, 900, "0"},         // 0/900
		{"全命中", 900, 0, "100"},        // 900/900
		{"一半", 1, 1, "50"},            // 1/2
		{"一三分之一（向下）", 1, 2, "33"},     // 33.3% → 33
		{"三分之二（向上）", 2, 1, "67"},      // 66.7% → 67
		{"差一个 token", 999, 1, "99.9"}, // 999/1000：不许四舍五入成 100%
		{"没有 prompt 侧", 0, 0, ""},     // 分母 0 → 官方 null
	}
	for _, tc := range cases {
		usage := &apiclient.TokenUsage{
			UncachedInputTokens: tc.unbilled,
			CacheReadTokens:     tc.cacheRead,
		}
		if got := usageCacheHitPercent(usage); got != tc.want {
			t.Fatalf("%s：cacheHitPercent = %q，want %q", tc.name, got, tc.want)
		}
	}
}

// ①' 胶囊读数：`{compact total} tok`〔` · 缓存命中 p%`〕（官方 reading()）。
func TestUsagePillTextFollowsOfficialReading(t *testing.T) {
	cases := []struct {
		name  string
		usage *apiclient.TokenUsage
		want  string
	}{
		{
			name:  "四桶都有值",
			usage: usageFull(),
			// total = 1500+4096+7+123 = 5726 → 5.7K；缓存率 4096/5603 = 73%
			want: "5.7K tok · 缓存命中 73%",
		},
		{
			name:  "没有缓存命中（分母成立，百分比是真实的 0）",
			usage: &apiclient.TokenUsage{UncachedInputTokens: 900, OutputTokens: 100},
			want:  "1K tok · 缓存命中 0%",
		},
		{
			name:  "四桶全 0：宿主说 0，不是未知",
			usage: &apiclient.TokenUsage{},
			want:  "0 tok",
		},
	}
	for _, tc := range cases {
		if got := usagePillText(tc.usage); got != tc.want {
			t.Fatalf("%s：用量读数 = %q，want %q", tc.name, got, tc.want)
		}
	}
}

// ② 明细行顺序与条件渲染逐条对齐官方 JSX（StatsPills.tsx:324-345）。
func TestUsageDialogRowsFollowOfficialOrder(t *testing.T) {
	rows := usageDialogRows(usageFull())
	want := []statsRow{
		{Label: zhUsageCacheHit, Value: "73%"},
		{Label: zhUsageInput, Value: "1,500 tok"},
		{Label: zhUsageCacheRead, Value: "4,096 tok"},
		{Label: zhUsageCacheWrite, Value: "7 tok"},
		{Label: zhUsageOutput, Value: "123 tok"},
	}
	assertRows(t, "五桶全画", rows, want)

	// 缓存写入 = 0 → 整行不画（官方 `:336` 的条件渲染），其余四行顺序不变。
	noWrite := &apiclient.TokenUsage{UncachedInputTokens: 900, OutputTokens: 100}
	assertRows(t, "缓存写入为 0", usageDialogRows(noWrite), []statsRow{
		{Label: zhUsageCacheHit, Value: "0%"},
		{Label: zhUsageInput, Value: "900 tok"},
		{Label: zhUsageCacheRead, Value: "0 tok"},
		{Label: zhUsageOutput, Value: "100 tok"},
	})
}

// assertRows 逐行逐字段比较（顺序也算）。
func assertRows(t *testing.T, name string, got, want []statsRow) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s：明细 %d 行，want %d：%+v", name, len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s：第 %d 行 = %+v，want %+v", name, i, got[i], want[i])
		}
	}
}

// ④ 未知 vs 0：两态在读数上必须不同，且全 0 时官方不画弹窗。
func TestUsagePillDistinguishesUnknownFromZero(t *testing.T) {
	unknown := pluginapi.Snapshot{Cur: "s"}
	zero := pluginapi.Snapshot{Cur: "s", TokenUsage: &apiclient.TokenUsage{}}
	known := pluginapi.Snapshot{Cur: "s", TokenUsage: usageFull()}

	if got := usagePillSegment(unknown); got != zhUsageUnknown {
		t.Fatalf("键缺席读数 = %q，want「%s」（不编 0）", got, zhUsageUnknown)
	}
	if got := usagePillSegment(zero); got != "0 tok" {
		t.Fatalf("四桶全 0 读数 = %q，want「0 tok」（宿主真值，不是未知）", got)
	}
	if usagePillSegment(unknown) == usagePillSegment(zero) {
		t.Fatal("未知与 0 是两态，读数不许相同")
	}
	if !usageHasDialog(known.TokenUsage) {
		t.Fatal("有计费活动时必须能开弹窗")
	}
	if usageHasDialog(zero.TokenUsage) {
		t.Fatal("四桶全 0 时官方整枚不渲染（没有可点入口）")
	}
	// 老宿主（只有 UsageTotal）仍走旧读数，不被新投影的缺席打断。
	if got := usagePillSegment(pluginapi.Snapshot{
		Cur:        "s",
		UsageTotal: pluginapi.Usage{TotalTokens: 12345},
	}); got != "总量 12.3k" {
		t.Fatalf("老宿主读数 = %q，want 旧读数「总量 12.3k」", got)
	}
}

// ① 四桶逐个渲染：只改一个桶，弹窗里那一行 + 标题值都必须跟着变（4 个子用例）。
func TestUsageDialogRendersEachBucket(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(u *apiclient.TokenUsage)
		label  string
		value  func(u *apiclient.TokenUsage) string
	}{
		{"uncachedInputTokens", func(u *apiclient.TokenUsage) { u.UncachedInputTokens = 4242 }, zhUsageInput,
			func(u *apiclient.TokenUsage) string { return hjCount(u.UncachedInputTokens) }},
		{"cacheReadTokens", func(u *apiclient.TokenUsage) { u.CacheReadTokens = 31337 }, zhUsageCacheRead,
			func(u *apiclient.TokenUsage) string { return hjCount(u.CacheReadTokens) }},
		{"cacheWriteTokens", func(u *apiclient.TokenUsage) { u.CacheWriteTokens = 99 }, zhUsageCacheWrite,
			func(u *apiclient.TokenUsage) string { return hjCount(u.CacheWriteTokens) }},
		{"outputTokens", func(u *apiclient.TokenUsage) { u.OutputTokens = 777 }, zhUsageOutput,
			func(u *apiclient.TokenUsage) string { return hjCount(u.OutputTokens) }},
	}
	for _, tc := range cases {
		usage := usageFull()
		tc.mutate(usage)
		canvas := openUsageDialog(t, usageSnapshot(usage))
		if !usageDialogHasRow(canvas, statsRow{Label: tc.label, Value: tc.value(usage)}) {
			t.Fatalf("%s：弹窗缺行 {%s %s}；texts=%v", tc.name, tc.label, tc.value(usage),
				statsDialogTexts(canvas))
		}
		// 标题右侧那一格是 exactCount(total)（官方 titleValue）。
		if want := hjCount(usage.TotalTokens()); !statsDialogHasText(canvas, want) {
			t.Fatalf("%s：标题值缺 %q；texts=%v", tc.name, want, statsDialogTexts(canvas))
		}
	}
}

// ③ 键缺席降级：读数「用量 未知」+ 没有命中区 + 点不开 + 不编 0。
func TestUsageDialogHiddenWithoutProjection(t *testing.T) {
	h := newStatsDialogHarness(t)
	data := usageSnapshot(nil)
	bounds := gfx.Rect{Width: 1182, Height: 832}
	h.frame(t, bounds, data, "", "") // 先画一帧，缓存胶囊矩形
	canvas := h.frame(t, bounds, data, usagePillHitName, "")
	if _, ok := statsDialogHitByName(t, h, canvas, usagePillHitName); ok {
		t.Fatal("没有 tokenUsage 投影时用量胶囊仍有命中区（假入口）")
	}
	if statsDialogHasText(canvas, zhUsageDialogTitle) {
		t.Fatalf("没有投影时点出了用量弹窗：%v", statsDialogTexts(canvas))
	}
	if !statsDialogHasText(canvas, zhUsageUnknown) {
		t.Fatalf("没有投影时读数 = %v，want「%s」", statsDialogTexts(canvas), zhUsageUnknown)
	}
	for _, text := range statsDialogTexts(canvas) {
		// 未知时既不许出现"累计 0"这个读数，也不许出现四桶的标签（那说明弹窗画了）。
		if text == "0 tok" {
			t.Fatalf("未知时编造了「0 tok」：%v", statsDialogTexts(canvas))
		}
		for _, banned := range []string{zhUsageInput, zhUsageCacheRead, zhUsageOutput} {
			if strings.Contains(text, banned) {
				t.Fatalf("未知时画出了四桶标签 %q：%v", banned, statsDialogTexts(canvas))
			}
		}
	}
}

// ④' 四桶全 0：读数 `0 tok`（真值），但官方不画弹窗 → 没有命中区、点不开。
func TestUsageZeroBucketsHaveNoDialog(t *testing.T) {
	h := newStatsDialogHarness(t)
	data := usageSnapshot(&apiclient.TokenUsage{})
	bounds := gfx.Rect{Width: 1182, Height: 832}
	closed := h.frame(t, bounds, data, "", "")
	if !statsDialogHasText(closed, "0 tok") {
		t.Fatalf("全 0 读数 = %v，want「0 tok」", statsDialogTexts(closed))
	}
	if statsDialogHasText(closed, zhUsageUnknown) {
		t.Fatalf("全 0 不许写成「未知」：%v", statsDialogTexts(closed))
	}
	clicked := h.frame(t, bounds, data, usagePillHitName, "")
	if _, ok := statsDialogHitByName(t, h, clicked, usagePillHitName); ok {
		t.Fatal("全 0 时官方整枚不渲染，不许有命中区")
	}
	if statsDialogHasText(clicked, zhUsageDialogTitle) {
		t.Fatalf("全 0 时点出了弹窗：%v", statsDialogTexts(clicked))
	}
}

// ⑤ 点用量胶囊打开官方 Token 用量弹窗：标题 + 头条值 + 四桶明细 + 锚点在胶囊上方。
func TestUsageDialogOpensFromPill(t *testing.T) {
	usage := usageFull()
	h, canvas := openUsageDialogWithHarness(t, usageSnapshot(usage))
	if !statsDialogHasText(canvas, zhUsageDialogTitle) {
		t.Fatalf("点用量胶囊没有打开「%s」弹窗；texts=%v", zhUsageDialogTitle, statsDialogTexts(canvas))
	}
	if want := hjCount(usage.TotalTokens()); !statsDialogHasText(canvas, want) {
		t.Fatalf("弹窗缺标题值 %q；texts=%v", want, statsDialogTexts(canvas))
	}
	for _, row := range usageDialogRows(usage) {
		if !usageDialogHasRow(canvas, row) {
			t.Fatalf("弹窗缺行 %+v；texts=%v", row, statsDialogTexts(canvas))
		}
	}
	// 命中区：aria 名就是胶囊读数；role=button + expandable + expanded=true。
	hit, ok := statsDialogHitByName(t, h, canvas, usagePillHitName)
	if !ok {
		t.Fatalf("用量胶囊没有命中区；hits=%v", hitLabels(canvas))
	}
	if hit.Role != gfx.RoleButton || !hit.Expandable || !hit.Expanded {
		t.Fatalf("用量胶囊 a11y = role=%v expandable=%v expanded=%v，want button/true/true",
			hit.Role, hit.Expandable, hit.Expanded)
	}
	if hit.Label != usagePillText(usage) {
		t.Fatalf("命中区 aria 名 = %q，want 胶囊读数 %q", hit.Label, usagePillText(usage))
	}
	// 弹窗底沿 = 用量胶囊顶沿 - 8（官方 bottom: anchor(top)+8）。
	panel, ok := statsDialogHit(canvas, zhUsageDialogTitle)
	if !ok {
		t.Fatalf("弹窗没有 aria 名 %q：%v", zhUsageDialogTitle, hitLabels(canvas))
	}
	if got, want := panel.Bounds.Y+panel.Bounds.Height, hit.Bounds.Y-StatsDialogAnchorGap; math.Abs(float64(got-want)) > 0.5 {
		t.Fatalf("弹窗底沿 = %v，want 用量胶囊顶沿-8 = %v", got, want)
	}
	// 弹窗的左沿跟着**用量**胶囊走（不是活动胶囊）。
	if delta := panel.Bounds.X - hit.Bounds.X; delta < -StatsDialogWindowInset || delta > StatsDialogWindowInset {
		t.Fatalf("弹窗左沿 = %v，用量胶囊左沿 = %v", panel.Bounds.X, hit.Bounds.X)
	}
}

// ⑤' 再点一次用量胶囊关闭弹窗（官方 setOpen(!open)）。
func TestUsageDialogTogglesOnSecondClick(t *testing.T) {
	h, open := openUsageDialogWithHarness(t, usageSnapshot(usageFull()))
	if !statsDialogHasText(open, zhUsageDialogTitle) {
		t.Fatalf("第一下没打开：%v", statsDialogTexts(open))
	}
	bounds := gfx.Rect{Width: 1182, Height: 832}
	closed := h.frame(t, bounds, usageSnapshot(usageFull()), usagePillHitName, "")
	if statsDialogHasText(closed, zhUsageDialogTitle) {
		t.Fatalf("第二下没有关闭：%v", statsDialogTexts(closed))
	}
}

// ⑥ 与 `sessionStats.decodeTokens` 交叉一致：同一轮的输出桶 = decodeTokens。
func TestUsageOutputMatchesSessionStatsDecodeTokens(t *testing.T) {
	stats := &apiclient.SessionStats{
		Turns: 1, Steps: 2, LLMMs: 2871, ToolMs: 67, TTFTMs: 1179, TTFTSteps: 1,
		DecodeMs: 10, DecodeTokens: 123,
	}
	usage := &apiclient.TokenUsage{UncachedInputTokens: 1500, OutputTokens: 123}
	data := pluginapi.Snapshot{Cur: "s", SessionStats: stats, TokenUsage: usage}
	canvas := openUsageDialog(t, data)
	if !usageDialogHasRow(canvas, statsRow{Label: zhUsageOutput, Value: hjCount(stats.DecodeTokens)}) {
		t.Fatalf("输出行 ≠ decodeTokens(%d)；texts=%v", stats.DecodeTokens, statsDialogTexts(canvas))
	}
	if usage.OutputTokens != stats.DecodeTokens {
		t.Fatalf("tokenUsage.outputTokens=%d 与 decodeTokens=%d 不一致（同一轮应逐字相等）",
			usage.OutputTokens, stats.DecodeTokens)
	}
}

// openUsageDialog 画两帧把用量弹窗点开（第一帧让 AfterDraw 缓存胶囊矩形）。
func openUsageDialog(t *testing.T, data pluginapi.Snapshot) *gfx.Canvas {
	t.Helper()
	_, canvas := openUsageDialogWithHarness(t, data)
	return canvas
}

// openUsageDialogWithHarness 同上，返回 harness 以便按交互名查命中区。
func openUsageDialogWithHarness(t *testing.T, data pluginapi.Snapshot) (*statsDialogHarness, *gfx.Canvas) {
	t.Helper()
	h := newStatsDialogHarness(t)
	bounds := gfx.Rect{Width: 1182, Height: 832}
	h.frame(t, bounds, data, "", "")
	return h, h.frame(t, bounds, data, usagePillHitName, "")
}

// usageDialogHasRow 报告画布上有没有一对（dt, dd）逐字相等的明细行。
func usageDialogHasRow(canvas *gfx.Canvas, row statsRow) bool {
	return statsDialogHasText(canvas, row.Label) && statsDialogHasText(canvas, row.Value)
}

// hitLabels 列命中区的 aria 名（失败信息用）。
func hitLabels(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Hits()))
	for _, hit := range canvas.Hits() {
		out = append(out, hit.Label)
	}
	return out
}
