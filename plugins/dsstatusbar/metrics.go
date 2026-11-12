package dsstatusbar

import (
	"math"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// 这些类型只描述官方 projection 的统计口径，不是 Transport 契约的替代品。
// 指针字段刻意保留“字段缺席”和“真实 0”的区别（`SessionStats` 用 nil 表示整份
// 投影缺席）；Activity、缓存率和上下文百分比不会靠窗口消息或零值出现。
type tokenUsage struct {
	UncachedInputTokens *int64
	CacheReadTokens     *int64
	CacheWriteTokens    *int64
}

type contextPressure struct {
	UsedTokens    *int64
	ContextWindow *int64
}

// activityText 对照官方 ActivityPill 的**非 compact 读数**
// （`StatsPills.tsx:299-305`）：`stats.counts` + 可选速率。
//
// 官方文案（`locale.ts:84` 的 zh 字典，逐字）是 `{turns} 轮 {steps} 步`——
// 两个计数之间是空格，不是分隔符；只有 counts 与速率之间才是 ` · `（`reading()`）。
//
// 速率只由 `decodeTokens/decodeMs` 得出（官方 `decodeSpeed`），且只有
// `decodeMs > 0` 时才画：没有 decode 墙钟就没有速率，**不写 0 tok/s 冒充**。
// 没有 projection（nil）或没有 assistant step（`steps <= 0`）时整枚隐藏——
// 官方 `stats.steps === 0 → null`；调用方据此写「活动 未知」。
func activityText(stats *apiclient.SessionStats) string {
	if stats == nil || stats.Steps <= 0 {
		return ""
	}
	parts := []string{itoa(int(stats.Turns)) + " 轮 " + itoa(int(stats.Steps)) + " 步"}
	if stats.DecodeMs > 0 {
		perSecond := float64(stats.DecodeTokens) / (float64(stats.DecodeMs) / 1000)
		parts = append(parts, rate(perSecond))
	}
	return strings.Join(parts, " · ")
}

// rate 是官方 `formatTokensPerSecond` 的移植（`message-chrome.ts:71-74`）：
//
//	clamped >= 10  → 取整（`Math.round`）
//	clamped <  10  → 一位小数（`Math.round(x*10)/10`）
//
// 为什么不是"永远一位小数"：官方 ≥10 时取整，40.7 显示成 `41 tok/s`。
// 负速率没有语义（墙钟回退已在投影里按 `Math.max(0, …)` 夹过），这里再夹一次 0；
// NaN 不写成 `NaN tok/s`（那不是一个读数），按 0 处理。
func rate(v float64) string {
	clamped := v
	if math.IsNaN(clamped) || clamped < 0 {
		clamped = 0
	}
	if clamped >= 10 {
		return itoa(int(math.Round(clamped))) + " tok/s"
	}
	return compactDecimal(math.Round(clamped*10)/10) + " tok/s"
}

// compactDecimal 复刻 JS `String(x)` 的数字形状：整数不补 `.0`（`String(4)` = "4"），
// 否则用一位小数（`String(4.4)` = "4.4"）。官方速率走的就是这个形状。
func compactDecimal(v float64) string {
	if v == math.Trunc(v) {
		return itoa(int(v))
	}
	return fmt1(v)
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
