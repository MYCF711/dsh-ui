// official_stats_usage.go —— 官方 **用量（Usage）胶囊 + 详情弹窗** 的读数与格式。
//
// # 官方结构（rc.2 `dsh-client-ui-chat/src/client/chat/`）
//
//	UsagePill（StatsPills.tsx:300-347）
//	├─ usage === undefined                       → null（整枚不渲染）
//	├─ billed === 0 && output === 0               → null（没有计费活动，不画）
//	├─ compact 模式                              → PlainPill（只画缓存命中率）
//	└─ 否则                                       → DialogPill
//	   ├─ 读数   `{total} tok`〔` · 缓存命中 {p}%`〕
//	   ├─ 标题   `Token 用量` + `{exactTotal} tok`
//	   └─ dl.details（行的顺序逐字照官方 JSX）
//	      ├─ 缓存命中   `{p}%`                （缓存率算不出来 → 整行不画）
//	      ├─ 未缓存输入 `{uncachedInput} tok`
//	      ├─ 缓存读取   `{cacheRead} tok`
//	      ├─ 缓存写入   `{cacheWrite} tok`    （**只有非 0 才画**，:336-341）
//	      └─ 输出       `{output} tok`
//
// # 两份数值格式（别混用）
//
//	胶囊读数  formatTokens      517 / 12.2K / 517K / 1.2M   （token-format.ts:9-15）
//	明细/标题 formatExactTokens 1,234,567                   （token-format.ts:23-30）
//
// 千位分隔符来自 locale `number.groupSeparator`（zh/en/… 都是 `,`，locale.ts:80），
// 缩放单位来自 `number.thousand` = `{value}K` / `number.million` = `{value}M`
// （`dsh-client-locale/src/locales/zh.ts:45-46` 的同名键）。
//
// # 缓存命中率是**客户端算的**
//
// 官方 `cacheHitPercent`（StatsPills.tsx:108-111）=
// `formatCacheHitPercent(cacheReadTokens, billedInputTokens)`：整数百分比、正数
// ties-up，且"差一个 token"绝不显示成 100%（token-format.ts:32-98）。分母是
// **prompt 侧三桶之和**，不是 totalTokens（输出桶不在其中）。
// 本仓这份实现复用 `metrics.go` 的 `formatCacheHitPercent`——同一个官方函数的
// 移植，别再写第三份。
//
// # 上下文占用百分比不在这里
//
// 官方用量弹窗**没有**上下文行：占用百分比属于 ContextMeter
// （`dsh-client-ui-conversation/src/client/skeleton/ContextMeter.tsx:57-143`），
// 数据是另一个投影键 `contextPressure`，算法在 `context-occupancy.ts:15-25`
// （客户端算，`apiclient.ContextPressure.Occupancy` 是它的移植）。所以本文件
// 不往用量弹窗塞一行"上下文占用"——那是自造结构。
package dsstatusbar

import (
	"math"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// 官方用量弹窗文案（zh 字典逐字；改字等于改官方界面，必须带 locale.ts 行号复核）。
const (
	zhUsageDialogTitle = "Token 用量" // stats.dialog.usageTitle  locale.ts:87
	zhUsageCacheHit    = "缓存命中"     // message.turnUsage.cacheHit  locale.ts:196
	zhUsageInput       = "未缓存输入"    // message.turnUsage.input  locale.ts:197
	zhUsageCacheRead   = "缓存读取"     // message.turnUsage.cacheRead  locale.ts:198
	zhUsageCacheWrite  = "缓存写入"     // message.turnUsage.cacheWrite  locale.ts:199
	zhUsageOutput      = "输出"       // message.turnUsage.output  locale.ts:200

	// zhTokenUnit 是 `message.turnUsage.count` = `{count} tok`（locale.ts:202）
	// 的单位段：所有 token 读数都以它结尾。
	zhTokenUnit = " tok"
)

// officialTokensCompact 是官方 `formatTokens`（token-format.ts:9-15）的移植：
//
//	scaled(x) = x >= 100 ? round(x) : round(x*10)/10      （整数不补 .0）
//	v < 1e3   → String(v)
//	v < 1e6   → scaled(v/1e3)  + "K"
//	否则       → scaled(v/1e6) + "M"
//
// 所以 999 是 `999`、1000 是 `1K`、12345 是 `12.3K`、999999 是 `1000K`
// （不是 `1M`——官方先判 < 1e6 再缩放，进位只发生在缩放函数里）。
// 负值没有语义（投影契约四桶非负），夹到 0；那是防御分支，不是"未知"的表达。
func officialTokensCompact(value int64) string {
	if value < 0 {
		value = 0
	}
	scaled := func(candidate float64) string {
		if candidate >= 100 {
			return itoa(int(math.Round(candidate)))
		}
		return compactDecimal(math.Round(candidate*10) / 10)
	}
	switch {
	case value < 1_000:
		return itoa(int(value))
	case value < 1_000_000:
		return scaled(float64(value)/1_000) + "K"
	default:
		return scaled(float64(value)/1_000_000) + "M"
	}
}

// officialTokensExact 是官方 `formatExactTokens`（token-format.ts:23-30）：
// 整数原样、每三位插一个 `number.groupSeparator`（= `,`），**不四舍五入**。
func officialTokensExact(value int64) string {
	if value < 0 {
		value = 0
	}
	digits := itoa64(value)
	groups := make([]string, 0, len(digits)/3+1)
	for end := len(digits); end > 0; end -= 3 {
		start := end - 3
		if start < 0 {
			start = 0
		}
		groups = append([]string{digits[start:end]}, groups...)
	}
	out := ""
	for i, group := range groups {
		if i > 0 {
			out += ","
		}
		out += group
	}
	return out
}

// itoa64 是 `itoa` 的 int64 版本（值域是 token 累计，int 在 32 位平台上不够）。
func itoa64(value int64) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for value > 0 {
		pos--
		buf[pos] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[pos:])
}

// usageCountText 是官方 `exactCount`（StatsPills.tsx:143-145）：`{exact} tok`。
func usageCountText(value int64) string {
	return officialTokensExact(value) + zhTokenUnit
}

// usageTotalText 是官方胶囊读数的第一段（StatsPills.tsx:310-311）：
// `t('message.turnUsage.count', {count: formatTokens(total)})`。
func usageTotalText(usage *apiclient.TokenUsage) string {
	return officialTokensCompact(usage.TotalTokens()) + zhTokenUnit
}

// usageCacheHitPercent 是官方 `cacheHitPercent`（StatsPills.tsx:108-111）：
// `formatCacheHitPercent(cacheRead, billedInput)`；返回空串 = 官方 null
// （缓存率不成立：没有 prompt 侧计费，或宿主自相矛盾）。
func usageCacheHitPercent(usage *apiclient.TokenUsage) string {
	if usage == nil {
		return ""
	}
	billed := usage.BilledInputTokens()
	read := usage.CacheReadTokens
	if billed <= 0 || read < 0 || read > billed {
		// 官方分母为 0 → null；负数/超过分母是坏数据（不写 100% 冒充完美命中）。
		return ""
	}
	return formatCacheHitPercent(read, billed, 0)
}

// usagePillText 是官方 `UsagePill` 的**读数**（`reading(totalText, cacheHitText)`，
// StatsPills.tsx:141-152 + :303-311）：
//
//	`{total} tok`〔` · 缓存命中 {p}%`〕
//
// 第二段只在缓存率成立时出现（官方 `cacheHitText === null` 就没有这一段）。
// 四桶全 0 时读数是 `0 tok`：那是宿主说"累计 0"，与「未知」是两回事
// （官方此时整枚不渲染，本仓 dock 行常驻，见 official_dock.go 头部的 deviation）。
func usagePillText(usage *apiclient.TokenUsage) string {
	if usage == nil {
		return ""
	}
	text := usageTotalText(usage)
	if percent := usageCacheHitPercent(usage); percent != "" {
		// 官方 `reading()` 的分隔符是 ` · `（StatsPills.tsx:148）。
		// 文案是 `stats.cacheHit` = `缓存命中 {percent}%`（locale.ts:85）。
		text += " · " + zhUsageCacheHit + " " + percent + "%"
	}
	return text
}

// usageHasDialog 是官方 UsagePill 的**画不画**判据（StatsPills.tsx:302）：
//
//	if (usage === undefined || (billedInputTokens(usage) === 0
//	    && usage.outputTokens === 0)) return null
//
// 即：键缺席 → 不画；键在但一次计费活动都没有 → 也不画（没有行就不开空弹窗）。
func usageHasDialog(usage *apiclient.TokenUsage) bool {
	if usage == nil {
		return false
	}
	return !(usage.BilledInputTokens() == 0 && usage.OutputTokens == 0)
}

// usageTitleValue 是官方弹窗标题右侧那一格（`titleValue={exactCount(total)}`，
// StatsPills.tsx:318）：精确值 + ` tok`，与胶囊上的紧凑读数**不是**同一份格式。
func usageTitleValue(usage *apiclient.TokenUsage) string {
	if usage == nil {
		return ""
	}
	return usageCountText(usage.TotalTokens())
}

// usageDialogRows 是官方用量弹窗的明细行（StatsPills.tsx:324-345），顺序逐字：
//
//	缓存命中   {p}%              （缓存率算不出来 → 整行不画）
//	未缓存输入 {exact} tok       （**总是**画，哪怕是真实的 0）
//	缓存读取   {exact} tok       （同上）
//	缓存写入   {exact} tok       （**只有非 0 才画**，`:336`）
//	输出       {exact} tok       （总是画）
//
// 为什么"未缓存输入"和"缓存读取"在 0 时也要画：官方注释写明这几个桶在这个
// 组件里**总是存在**（`the buckets are always present here`，:321-323），
// 与"未知"不是一回事——未知是整枚胶囊/整份投影缺席。
func usageDialogRows(usage *apiclient.TokenUsage) []statsRow {
	if usage == nil || !usageHasDialog(usage) {
		return nil
	}
	rows := make([]statsRow, 0, 5)
	if percent := usageCacheHitPercent(usage); percent != "" {
		rows = append(rows, statsRow{Label: zhUsageCacheHit, Value: percent + "%"})
	}
	rows = append(rows,
		statsRow{Label: zhUsageInput, Value: usageCountText(usage.UncachedInputTokens)},
		statsRow{Label: zhUsageCacheRead, Value: usageCountText(usage.CacheReadTokens)},
	)
	if usage.CacheWriteTokens != 0 {
		rows = append(rows, statsRow{Label: zhUsageCacheWrite, Value: usageCountText(usage.CacheWriteTokens)})
	}
	rows = append(rows, statsRow{Label: zhUsageOutput, Value: usageCountText(usage.OutputTokens)})
	return rows
}

// usagePillSegment 是 dock 上「用量」那一枚的读数，优先级与官方一致：
//
//	有 `tokenUsage` 投影 → 官方读数（`{total} tok`〔` · 缓存命中 p%`〕）
//	没有投影但老宿主给了 UsageTotal/Usage → 旧读数（`总量 12.3k` 等，见 metrics/statusbar）
//	两者都没有 → 「用量 未知」
//
// 为什么不是"有投影就一律用新格式、不管有没有真值"：投影在（四桶全 0）时读数
// 就是 `0 tok`——宿主说累计 0，这是真值；「未知」只留给键缺席。
func usagePillSegment(data pluginapi.Snapshot) string {
	if data.TokenUsage != nil {
		return usagePillText(data.TokenUsage)
	}
	return orUnknown(usageTokenSegment(data), zhUsageUnknown)
}
