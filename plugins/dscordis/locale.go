package dscordis

import (
	"strconv"
	"strings"
)

// locale.go —— 官方面板用到的文案。
//
// **只许用官方 `locales.ts` 的 `zh` 字典原文**：
// `OFF:dsh-client-ui-cordis/src/client/locales.ts:11-40`（`zh` 字典，行号写在每条旁边）。
// 一条都不新写、不改标点、不改大小写。官方其余语言的字典（en/ja/es/fr/de/ru/pt）
// 本仓界面只有中文，不录。
const (
	// zhPanelTitle 是 `panel.title`（locales.ts:30）＝「Cordis 插件」。
	zhPanelTitle = "Cordis 插件"
	// zhPanelEmpty 是 `panel.empty`（locales.ts:31）＝「还没有定义任何插件」。
	zhPanelEmpty = "还没有定义任何插件"
	// zhPanelLoading 是 `panel.loading`（locales.ts:32）＝「读取中…」。
	zhPanelLoading = "读取中…"
	// zhPanelReadFailed 是 `panel.readFailed`（locales.ts:33）的模板
	// 「读取插件清单失败：{message}」——`{message}` 先用宿主原文替换。
	zhPanelReadFailed = "读取插件清单失败：{message}"
	// zhPanelGroupCurrent / Others 是 `panel.group.current` / `panel.group.others`
	// （locales.ts:34-35）。
	zhPanelGroupCurrent = "当前会话"
	zhPanelGroupOthers  = "其他会话"
	// zhPanelTrigger 是 `panel.trigger`（locales.ts:28）＝「Cordis Plugin」（官方就是英文）。
	zhPanelTrigger = "Cordis Plugin"
	// zhPanelPluginsAria 是 `panel.plugins.aria`（locales.ts:26）——触发器的可访问名。
	zhPanelPluginsAria = "Cordis 插件"
	// zhPanelRunningCount 是 `panel.runningCount`（locales.ts:29）的模板
	// 「{count} running」——`{count}` 先替换（官方也是模板，不是计数文案）。
	zhPanelRunningCount = "{count} running"

	// zhStatusIdle / ClientPending / Running / Failed 是 `status.idle` /
	// `status.clientPending` / `status.running` / `status.failed`（locales.ts:18-22）。
	zhStatusIdle          = "待激活"
	zhStatusClientPending = "Client 待激活"
	zhStatusRunning       = "运行中"
	zhStatusFailed        = "运行失败"
	// zhStatusAwaitingApproval 是 `status.awaitingApproval`（locales.ts:19）＝「待审批」。
	//
	// 它只用在一条**本宿主永不进入**的分支上（`visiblePanelStatus` 的
	// `latestRun.status == "awaiting-approval"`）：本宿主没有 Cordis 审批队列，
	// 那条帧不会出现（HIDE 第 4 条）。保留它是为了让派生链与官方逐字同形——
	// 万一哪天宿主发了这条状态，界面画的是官方原文，而不是自己编的词。
	zhStatusAwaitingApproval = "待审批"

	// zhActionRun / Stop / Remove 是 `action.run` / `action.stop` / `action.remove`
	// （locales.ts:41-43）——动作按钮的可访问名（官方 `.actionButton` 的可读文案
	// 就是 `aria-label`，视觉上只有图标，见 CSS:318-320 的注释）。
	zhActionRun    = "运行"
	zhActionStop   = "停止"
	zhActionRemove = "移除"

	// `status.removed`（locales.ts:23「已移除」）**没有**用到：官方面板里被移除的行
	// 直接不在清单里（`inventory.retire()` 把它从 rows 里删掉，
	// OFF:inventory.ts:102-106），`status.removed` 只出现在四张历史工具卡上，
	// 而那四张卡是本包的 HIDE 第 2 条。这里不定义它，免得看起来"该有却没有"。
)

// formatCount 把官方 `panel.runningCount` 的 `{count}` 换成真值。
//
// 官方是 `t('panel.runningCount', { count: running })`（OFF:CordisPanel.tsx:500），
// 模板里只有这一个占位符，所以只替换第一个（`{count} running`）。
func formatCount(template string, count int) string {
	return strings.Replace(template, "{count}", strconv.Itoa(count), 1)
}

// formatReadFailed 把官方 `panel.readFailed` 的 `{message}` 换成宿主原文。
// 官方是 `t('panel.readFailed', { message: inventory.error })`（OFF:CordisPanel.tsx:466）。
func formatReadFailed(template, message string) string {
	return strings.Replace(template, "{message}", message, 1)
}
