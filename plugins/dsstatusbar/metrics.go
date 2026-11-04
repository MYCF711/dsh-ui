package dsstatusbar

import (
	"math"
	"strings"
)

// 这些类型只描述官方 projection 的统计口径，不是 Transport 契约的替代品。
// 指针字段刻意保留“字段缺席”和“真实 0”的区别；C-STATS 接线前，生产 Snapshot
// 不会构造它们，因此 Activity、缓存率和上下文百分比不会靠窗口消息或零值出现。
type sessionStats struct {
	Turns        int
	Steps        int
	DecodeTokens int64
	DecodeMs     int64
}

type tokenUsage struct {
	UncachedInputTokens *int64
	CacheReadTokens     *int64
	CacheWriteTokens    *int64
}

type contextPressure struct {
	UsedTokens    *int64
	ContextWindow *int64
}

// activityText 对照官方 ActivityPill：turns 是 whole-log 唯一 turn 数，steps 是
// assistant/step 数，速率只由 decodeTokens/decodeMs 得出。没有 projection 或没有
// assistant step 时整枚隐藏；只有墙钟时间、消息 role 或 tool 数都不能进入这里。
func activityText(stats *sessionStats) string {
	if stats == nil || stats.Steps <= 0 {
		return ""
	}
	parts := []string{itoa(stats.Turns) + " 轮 · " + itoa(stats.Steps) + " 步"}
	if stats.DecodeMs > 0 && stats.DecodeTokens >= 0 {
		perSecond := float64(stats.DecodeTokens) / (float64(stats.DecodeMs) / 1000)
		parts = append(parts, rate(perSecond))
	}
	return strings.Join(parts, " · ")
}

// cacheHitPercent 对照官方 UsagePill。命中率是 cacheRead /
// (uncachedInput + cacheRead + cacheWrite)，不是 cacheRead/totalTokens。
// 三个分母桶缺任何一个都不知道官方口径，必须隐藏；分母真实为 0 也隐藏。
func cacheHitPercent(usage *tokenUsage) string {
	if usage == nil || usage.UncachedInputTokens == nil ||
		usage.CacheReadTokens == nil || usage.CacheWriteTokens == nil {
		return ""
	}
	cacheRead := *usage.CacheReadTokens
	denominator := *usage.UncachedInputTokens + cacheRead + *usage.CacheWriteTokens
	if denominator <= 0 || cacheRead < 0 || cacheRead > denominator {
		return ""
	}
	percent := formatCacheHitPercent(cacheRead, denominator, 0)
	if percent == "" {
		return ""
	}
	return percent + "%"
}

// formatCacheHitPercent 是官方 formatCacheHitPercent 的整数移植。
// 普通值按正数 ties-up 取整；部分命中即使极接近 100，也增加刚好可区分的小数位，
// 绝不把“还差一个 token”显示成 100%。decimalPlaces 只接受官方使用的 0 或 1。
func formatCacheHitPercent(cacheReadTokens, denominator int64, decimalPlaces int) string {
	if denominator <= 0 || cacheReadTokens < 0 || cacheReadTokens > denominator {
		return ""
	}
	missed := denominator - cacheReadTokens
	if missed == 0 {
		return "100"
	}
	units := roundedPercentUnits(cacheReadTokens, denominator, decimalPlaces)
	if (decimalPlaces == 0 && units < 100) || (decimalPlaces == 1 && units < 1000) {
		return displayPercentUnits(units, decimalPlaces)
	}

	distinguishingPlaces := 1
	scaledDoubleGap := missed * 200
	denominatorTens := denominator / 10
	for scaledDoubleGap <= denominatorTens {
		scaledDoubleGap *= 10
		distinguishingPlaces++
	}
	denominatorOnes := denominator % 10
	roundedLoss := 5
	for loss := int64(1); loss < 5; loss++ {
		factor := loss*2 + 1
		threshold := factor*denominatorTens + factor*denominatorOnes/10
		if scaledDoubleGap <= threshold {
			roundedLoss = int(loss)
			break
		}
	}
	return "99." + strings.Repeat("9", distinguishingPlaces-1) + itoa(10-roundedLoss)
}

// roundedPercentUnits 对照官方二分求整，避免浮点误差改变边界百分比。
func roundedPercentUnits(cacheReadTokens, denominator int64, decimalPlaces int) int64 {
	scale := int64(100)
	if decimalPlaces != 0 {
		scale = 1000
	}
	doubledScale := scale * 2
	quotient := denominator / doubledScale
	remainder := denominator % doubledScale
	lower, upper := int64(0), scale
	for lower < upper {
		candidate := (lower + upper + 1) / 2
		factor := candidate*2 - 1
		threshold := factor*remainder + doubledScale - 1
		if cacheReadTokens >= factor*quotient+threshold/doubledScale {
			lower = candidate
		} else {
			upper = candidate - 1
		}
	}
	return lower
}

func displayPercentUnits(units int64, decimalPlaces int) string {
	if decimalPlaces == 0 {
		return itoa(int(units))
	}
	whole, tenths := units/10, units%10
	if tenths == 0 {
		return itoa(int(whole))
	}
	return itoa(int(whole)) + "." + itoa(int(tenths))
}

// contextPercent 对照官方 contextOccupancy：usedTokens 与 contextWindow 必须同时
// 存在且容量为正。只有 ContextTokens 时没有百分比；真实 0/100 则可以显示。
func contextPercent(pressure *contextPressure) string {
	if pressure == nil || pressure.UsedTokens == nil || pressure.ContextWindow == nil {
		return ""
	}
	used, window := *pressure.UsedTokens, *pressure.ContextWindow
	if used < 0 || window <= 0 {
		return ""
	}
	percent := int(math.Round(float64(used) / float64(window) * 100))
	if percent > 100 {
		percent = 100
	}
	return itoa(percent) + "%"
}
