// official_stats_format.go —— Activity 详情弹窗的**文案与数值格式**（官方逐字移植）。
//
// 官方弹窗（`dsh-client-ui-chat/src/client/chat/StatsPills.tsx:249-296`）里四条
// 明细的取值与格式全在 locale 字典与 `formatDuration` 上，本文件是它们的唯一出处：
//
//	stats.dialog.title       会话统计                     locale.ts:86
//	stats.dialog.llmTime     模型用时                     locale.ts:88
//	stats.dialog.toolTime    工具调用用时                 locale.ts:89
//	stats.dialog.ttft        首 token 平均（TTFT）        locale.ts:90
//	stats.dialog.speed       输出速度（TPS）              locale.ts:91
//	stats.counts             {turns} 轮 {steps} 步        locale.ts:84
//	duration.compactSeconds  {seconds}秒                 locale.ts:81
//	duration.compactMinutes  {minutes}分{seconds}秒       locale.ts:82
//	message.tokensPerSecond  {tps} tok/s                 locale.ts:192
//
// # 为什么时长不是「毫秒数」
//
// 官方弹窗用的是 `formatDuration`（StatsPills.tsx:112-120）：<60s 走
// `duration.compactSeconds`（一位小数、整数不补 `.0`），≥60s 走
// `duration.compactMinutes`（整秒四舍五入后拆成分与秒）。
// **不是** `dsh-experimental-client-ui-stats` 的 `formatMs`（那个是统计表格视图
// 的 `12.34 s` / `0.5 min` 写法，与这里不是同一个组件）。两个都用会让同一份
// 真值在两个地方显示成两种样子，所以这里只移植 StatsPills 这一套。
package dsstatusbar

import (
	"math"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// 弹窗文案（官方 zh 字典逐字；改字等于改官方界面，必须带 locale.ts 行号复核）。
const (
	zhStatsDialogTitle    = "会话统计"
	zhStatsDialogLLMTime  = "模型用时"
	zhStatsDialogToolTime = "工具调用用时"
	zhStatsDialogTTFT     = "首 token 平均（TTFT）"
	zhStatsDialogSpeed    = "输出速度（TPS）"
)

// statsRow 是弹窗明细网格的一行（官方一个 `<dt>` + 一个 `<dd>`）。
type statsRow struct {
	Label string
	Value string
}

// statsCountsText 是官方 `stats.counts`（locale.ts:84）：`{turns} 轮 {steps} 步`。
//
// 两个计数之间是**空格**，不是 ` · `——分隔符只出现在"读数第二段"那里
// （`reading()`，StatsPills.tsx:141-152）。弹窗标题右侧那一格用的也是这份文案
// （`titleValue={counts}`，:265），所以它必须与胶囊读数逐字一致。
func statsCountsText(stats *apiclient.SessionStats) string {
	if stats == nil {
		return ""
	}
	return itoa(int(stats.Turns)) + " 轮 " + itoa(int(stats.Steps)) + " 步"
}

// statsDuration 是官方 `formatDuration`（StatsPills.tsx:112-120）的移植。
//
//	s = ms / 1000
//	s < 60   → `{Math.round(s*10)/10}秒`      （3.267s → "3.3秒"，整数不补 .0）
//	s >= 60  → `{⌊w/60⌋}分{w%60}秒`，其中 w = Math.round(s)
//	           （61s → "1分1秒"；59.99s → "1分0秒"，因为 w 先被四舍五入成 60）
//
// 负值/NaN 没有语义（投影契约里八字段非负），夹到 0 后按 `0秒` 输出——不写
// `NaN秒`；这是防御分支，不是"未知"的表达（未知由整枚胶囊/整行不渲染表达）。
func statsDuration(ms float64) string {
	switch {
	case math.IsNaN(ms) || ms < 0:
		ms = 0
	}
	seconds := ms / 1000
	if seconds < 60 {
		return compactDecimal(math.Round(seconds*10)/10) + "秒"
	}
	whole := int(math.Round(seconds))
	return itoa(whole/60) + "分" + itoa(whole%60) + "秒"
}

// statsDialogRows 是官方 ActivityPill 弹窗的明细行（StatsPills.tsx:266-292）。
//
// 四条各自独立按真值出现，**顺序与官方 JSX 一致**：
//
//	stats.llmMs > 0       → 模型用时        formatDuration(llmMs)
//	stats.toolMs > 0      → 工具调用用时    formatDuration(toolMs)
//	stats.ttftSteps > 0   → 首 token 平均   formatDuration(ttftMs / ttftSteps)
//	speed !== null        → 输出速度        {formatTokensPerSecond(decodeTokens/decodeMs·1000)} tok/s
//
// 官方 `speed` 的判据是 `stats.decodeMs > 0`（:298）——**没有 decode 墙钟就没有
// 速率**，不写 `0 tok/s` 冒充。八字段里的 turns/steps 不在明细里，它们是弹窗
// 标题右侧那一格（`titleValue={counts}`，:265）。
//
// 返回空 = 官方那条 `if (stats.llmMs <= 0 && stats.toolMs <= 0 && stats.ttftSteps <= 0
// && speed === null) return <PlainPill …>`（:288-289）：弹窗没有一行可画，
// 胶囊退回**静态读数**（不可点、不画 hover）。
func statsDialogRows(stats *apiclient.SessionStats) []statsRow {
	if stats == nil || stats.Steps <= 0 {
		return nil
	}
	rows := make([]statsRow, 0, 4)
	if stats.LLMMs > 0 {
		rows = append(rows, statsRow{Label: zhStatsDialogLLMTime, Value: statsDuration(float64(stats.LLMMs))})
	}
	if stats.ToolMs > 0 {
		rows = append(rows, statsRow{Label: zhStatsDialogToolTime, Value: statsDuration(float64(stats.ToolMs))})
	}
	if stats.TTFTSteps > 0 {
		// 官方是浮点平均（`stats.ttftMs / stats.ttftSteps`），先除再格式化：
		// 先取整会改变 .05s 边界上的四舍五入结果。
		average := float64(stats.TTFTMs) / float64(stats.TTFTSteps)
		rows = append(rows, statsRow{Label: zhStatsDialogTTFT, Value: statsDuration(average)})
	}
	if stats.DecodeMs > 0 {
		rows = append(rows, statsRow{Label: zhStatsDialogSpeed, Value: statsSpeedText(stats)})
	}
	return rows
}

// statsSpeedText 是官方 `decodeSpeed`（StatsPills.tsx:170-174）：
// `formatTokensPerSecond(decodeTokens / (decodeMs/1000))` + ` tok/s`。
func statsSpeedText(stats *apiclient.SessionStats) string {
	if stats == nil || stats.DecodeMs <= 0 {
		return ""
	}
	return rate(float64(stats.DecodeTokens) / (float64(stats.DecodeMs) / 1000))
}
