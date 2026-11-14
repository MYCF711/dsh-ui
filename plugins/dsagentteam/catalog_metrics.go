// catalog_metrics.go —— 子代理目录行上的**指标**（token / 时长）与展开判据。
//
// 全部逐行移植官方 `SubagentHeaderLineage.tsx` 的纯函数：
//
//	formatTokens         :69-77      （<1000 原样；<1e6 → K；否则 M）
//	activityDuration     :89-104     （settledMs + 未闭合轮的增量）
//	formatDuration       :130-160    （多档，判定顺序必须照抄）
//	formatExactDuration  :162-178    （≥1 天走 exactDays，否则复用 compact）
//	canExpandCatalog     :209-213    （mode ≠ external ∧ 子目录有行）
//
// 文案取官方 zh 字典 `OFF:ui-subagent/src/client/locales.ts:14-27,40-41`（逐字）。
// **不画的部分**：activity 三态圆点需要"该子会话是否在跑"的真值，本仓宿主没有
// 这个投影（见 docs/official-subagent-evidence-rc2.md 的登记），所以 status dot 与
// `progress.*` 段保持"不画"，而不是编一个中性点。
package dsagentteam

import (
	"fmt"
	"strconv"
	"strings"
)

// 官方 zh 文案（逐字；`{...}` 是占位符）。
const (
	zhTokensTotal    = "{value} tok"
	zhTokensThousand = "{value}K"
	zhTokensMillion  = "{value}M"

	zhDurationSeconds    = "{seconds}秒"
	zhDurationMinutes    = "{minutes}分{seconds}秒"
	zhDurationHours      = "{hours}小时{minutes}分{seconds}秒"
	zhDurationDays       = "{days}天"
	zhDurationDaysHours  = "{days}天{hours}小时"
	zhDurationMonths     = "约{months}个月"
	zhDurationMonthsDays = "约{months}个月{days}天"
	zhDurationYears      = "约{years}年"
	zhDurationYearsMonth = "约{years}年{months}个月"
	zhDurationExactDays  = "{days}天{hours}小时{minutes}分{seconds}秒"
	zhDurationExactTitle = "总活跃耗时：{duration}"

	zhBranchExpand   = "展开 {label} 的下级子智能体"
	zhBranchCollapse = "收起 {label} 的下级子智能体"
)

const (
	msPerSecond = int64(1000)
	msPerMinute = 60 * msPerSecond
	msPerHour   = 60 * msPerMinute
	msPerDay    = 24 * msPerHour
)

// fill 把模板里的 `{name}` 换成值（官方 t() 的同形最小实现）。
func fillTile(tpl string, pairs ...string) string {
	out := tpl
	for i := 0; i+1 < len(pairs); i += 2 {
		out = strings.ReplaceAll(out, "{"+pairs[i]+"}", pairs[i+1])
	}
	return out
}

// catalogFormatTokens 是官方 `formatTokens`（:69-77）。
func catalogFormatTokens(value int64) string {
	if value < 1000 {
		return strconv.FormatInt(value, 10)
	}
	scaled := func(next float64) string {
		if next >= 100 {
			return strconv.FormatInt(int64(next+0.5), 10)
		}
		return strconv.FormatFloat(float64(int64(next*10+0.5))/10, 'f', -1, 64)
	}
	if value < 1_000_000 {
		return fillTile(zhTokensThousand, "value", scaled(float64(value)/1000))
	}
	return fillTile(zhTokensMillion, "value", scaled(float64(value)/1_000_000))
}

// catalogTokenMetric 是官方 `tokenMetric`（:311-313）：`{value} tok`。
func catalogTokenMetric(total int64) string {
	return fillTile(zhTokensTotal, "value", catalogFormatTokens(total))
}

// pad2 是官方模板里 `String(x).padStart(2,'0')` 的等价物。
func pad2(v int64) string { return fmt.Sprintf("%02d", v) }

// catalogDurationCompact 是官方 `formatDuration`（:130-160）的逐档移植。
//
// 判定顺序（照抄，不许重排）：365 天 → 年/年月；30 天 → 月/月天；>0 天 → 天/天小时；
// 有整小时 → 时分秒；有整分钟 → 分秒；否则秒。
func catalogDurationCompact(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	days := ms / msPerDay
	if days >= 365 {
		years := days / 365
		months := (days % 365) / 30
		if months > 0 {
			return fillTile(zhDurationYearsMonth, "years", strconv.FormatInt(years, 10), "months", strconv.FormatInt(months, 10))
		}
		return fillTile(zhDurationYears, "years", strconv.FormatInt(years, 10))
	}
	if days >= 30 {
		months := days / 30
		remDays := days % 30
		if remDays > 0 {
			return fillTile(zhDurationMonthsDays, "months", strconv.FormatInt(months, 10), "days", strconv.FormatInt(remDays, 10))
		}
		return fillTile(zhDurationMonths, "months", strconv.FormatInt(months, 10))
	}
	if days > 0 {
		hours := (ms % msPerDay) / msPerHour
		if hours > 0 {
			return fillTile(zhDurationDaysHours, "days", strconv.FormatInt(days, 10), "hours", strconv.FormatInt(hours, 10))
		}
		return fillTile(zhDurationDays, "days", strconv.FormatInt(days, 10))
	}
	totalHours := ms / msPerHour
	if totalHours > 0 {
		minutes := (ms % msPerHour) / msPerMinute
		seconds := (ms % msPerMinute) / msPerSecond
		return fillTile(zhDurationHours, "hours", strconv.FormatInt(totalHours, 10), "minutes", pad2(minutes), "seconds", pad2(seconds))
	}
	totalMinutes := ms / msPerMinute
	if totalMinutes > 0 {
		seconds := (ms % msPerMinute) / msPerSecond
		return fillTile(zhDurationMinutes, "minutes", strconv.FormatInt(totalMinutes, 10), "seconds", pad2(seconds))
	}
	return fillTile(zhDurationSeconds, "seconds", strconv.FormatInt(ms/msPerSecond, 10))
}

// catalogDurationExact 是官方 `formatExactDuration`（:162-178）：
// 不足一天复用 compact，否则 `exactDays` 全展开。
func catalogDurationExact(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	days := ms / msPerDay
	if days == 0 {
		return catalogDurationCompact(ms)
	}
	hours := (ms % msPerDay) / msPerHour
	minutes := (ms % msPerHour) / msPerMinute
	seconds := (ms % msPerMinute) / msPerSecond
	return fillTile(zhDurationExactDays,
		"days", strconv.FormatInt(days, 10),
		"hours", pad2(hours), "minutes", pad2(minutes), "seconds", pad2(seconds))
}

// catalogCanExpand 是官方 `canExpandCatalog`（:209-213）：
//
//	mode !== 'external' ∧ 子会话自己的 catalog 有行
//
// （官方的另两个条件 `summary.formatStatus !== 'migration-required'` 与
// `catalogs[id].state !== 'migration-required'` 在本仓没有对应真值：
// 宿主不产出 migration-required 会话态——不猜，少画就是少画。）
func catalogCanExpand(entry CatalogEntry, child ChildProjection) bool {
	if entry.Mode == "external" {
		return false
	}
	return child.CatalogKnown && len(child.Catalog) > 0
}

// catalogReserveDisclosure 是官方 `reserveDisclosure`（:259）：
// **这一层只要有任意一行可展开**，兄弟叶子行就给展开列让出位置。
func catalogReserveDisclosure(entries []CatalogEntry, children map[string]ChildProjection) bool {
	for _, entry := range entries {
		if catalogCanExpand(entry, children[entry.ID]) {
			return true
		}
	}
	return false
}

// catalogRowMetrics 返回这一行要画的三个指标串（没数据就是空串 → 那一格不画）。
//
//	token    第 1 行（`.metricToken`）
//	compact  第 2 行（`.metricDuration` 可见文本）
//	exact    第 2 行的 title/aria（官方 `duration.exactTitle`）
func catalogRowMetrics(child ChildProjection, nowMs int64) (token, compact, exact string) {
	if child.TokenTotal != nil {
		token = catalogTokenMetric(*child.TokenTotal)
	}
	if ms, ok := child.Timing.DurationMs(nowMs); ok {
		compact = catalogDurationCompact(ms)
		exact = fillTile(zhDurationExactTitle, "duration", catalogDurationExact(ms))
	}
	return token, compact, exact
}
