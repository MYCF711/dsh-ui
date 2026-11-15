// cot_translation.go —— #63 `dsh-experimental-client-ui-cot-translation` 的界面本体
// （官方 `conversation.chat.reasoning.body` 那个座位：把展开的思考翻成界面语言）。
//
// # 官方判据（逐条对得上，不自己发明）
//
//	OFF:src/client/TranslationBody.tsx:30-61   工具栏 + 正文；按钮文案 translation/original，
//	                                            pending 出 spinner(aria-label=translating)，
//	                                            failed 出 failed 文案 + retry
//	OFF:src/client/translation.ts:20-42        reasoningFragments：按「空行段落」切分，
//	                                            超长段落再按 maxTextChars 切（不得劈开代理对）
//	OFF:src/client/translation.ts:82-94        emit()：逐段拼接，**有译文用译文、没有就用原文**
//	OFF:src/client/translation.ts:96-113       run()：同一时刻只有一个请求在飞；按段序推进；
//	                                            失败即 failed=true 并保留已翻好的部分
//	OFF:src/client/locales.ts:17-18            逐字中文文案
//
// # 与本仓形状的两处差异（都是"没有这条能力"，不是偷懒）
//
//  1. 官方的 `run()` 是 async/await 顺序循环；这里插件在 draw 里推进（同一帧最多
//     一次状态迁移：先收上一个 job 的终态，或发起下一段）。语义一致——同一时刻
//     只有一个请求、按段序、失败即停。
//  2. 官方正文走 `renderFactorySlot('conversation.chat.reasoning.content')`
//     （Markdown 渲染 + 代码块工具条）；本仓展开态一直是 `ui.Wrap` 纯文本
//     （见 reasoning.go 文件头那条"刻意差异"）。译文与原文用同一个画法，
//     所以译文也是纯文本——**不在这里另造一套 Markdown**。
//
// # 状态归属
//
// 键用 MessageKey（含会话），与 p.expanded 同一口径：换会话不会串用，
// 同一段思考反复收起展开也不会重复付费（缓存命中即不再发请求）。
package dsconversation

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/ui"
)

// cotFragment 是一段待翻译的原文与它后面原样保留的分隔符。
//
// Suffix 必须逐字保留：官方 emit() 是 `content + suffix`，段落之间的空行
// 就是靠它还原的；丢掉它译文会挤成一坨。
type cotFragment struct {
	Text     string
	Suffix   string
	Complete bool
}

// cotEntry 是一段思考的译文状态（**只活在这个 disclosure 挂载期间**的官方语义，
// 这里放宽到"这个会话这条消息"，因为本仓的思考块没有卸载事件）。
type cotEntry struct {
	fragments []cotFragment
	// cache 键是**原文片段**（官方同一口径）：同一段重复出现只付一次费。
	cache map[string]string
	// pending = 有请求在飞；failed = 已经失败过（等用户点"重试"）。
	pending bool
	failed  bool
	// job / jobText 是正在飞的那次请求的 jobID 与它对应的原文片段。
	job     string
	jobText string
	// original = 用户在**看原文**（官方 `original`，缺省 false = 看译文）。
	//
	// 缺省值就是官方的语义：这个功能的卖点是"思考默认被翻好"，所以展开后
	// 先显示译文（还没翻完的段仍显示原文，见 cotDisplayText），按钮提供
	// 「查看原文」；失败时官方强制回原文（`showOriginal = original || failed`）。
	original bool
	// source 是上一次算分段的输入，用来判断"文本没变就不重算"。
	source string
}

// cotFragments 把思考正文切成可翻译片段（官方 reasoningFragments 的逐条移植）。
//
// 规则（与 OFF:translation.ts:20-42 一行一行对应）：
//   - 段落分隔 = 一个换行 + 至少一个只含空白的行 + 换行（与 reasoning.go 的
//     nextParagraphBreak 同一判据，预览用的是它）；
//   - 分隔符本身进**前一段的 Suffix**（官方 `separator` 匹配到的整段）；
//   - 超过 maxTextChars 的段落要再切：切点不得落在 UTF-16 代理对的中间
//     （官方 `if (/[\uD800-\uDBFF]/.test(remaining.charAt(cut - 1))) cut -= 1`），
//     并尽量退到最近的空白处；切出去的片段 Complete=true（它已经定稿）；
//   - 段落的 Complete 只看"后面还有没有分隔符"或"这段还在写"：
//     `complete = match !== null || !running`。
func cotFragments(text string, running bool, maxTextChars int) []cotFragment {
	if maxTextChars <= 0 {
		maxTextChars = 1
	}
	var out []cotFragment
	start := 0
	for {
		sepAt, sepLen := nextParagraphBreak(text, start)
		remaining := text[start:]
		if sepAt >= 0 {
			remaining = text[start:sepAt]
		}
		// 超长段落先切：官方是 while 循环，切到剩不下为止。
		for cotUTF16Len(remaining) > maxTextChars {
			cut := cotCutPoint(remaining, maxTextChars)
			source := remaining[:cut]
			rest := remaining[cut:]
			suffix := ""
			if trimmed := strings.TrimLeft(rest, " \t\r\n"); trimmed != rest {
				suffix = rest[:len(rest)-len(trimmed)]
			}
			out = append(out, cotFragment{Text: source, Suffix: suffix, Complete: true})
			remaining = rest[len(suffix):]
		}
		suffix := ""
		if sepAt >= 0 {
			suffix = text[sepAt : sepAt+sepLen]
		}
		out = append(out, cotFragment{Text: remaining, Suffix: suffix, Complete: sepAt >= 0 || !running})
		if sepAt < 0 {
			return out
		}
		start = sepAt + sepLen
	}
}

// cotCutPoint 找一个不超过 maxTextChars（UTF-16 码元）的切点。
//
// 两条硬约束，顺序不能换：
//  1. 不许劈开代理对：切点落在高代理之后就把切点退一格（把整个字符留给下一段）；
//  2. 能退到空白就退到空白（官方 `search(/\s+\S*$/)`）：在词中间切开会让译文对不上。
func cotCutPoint(text string, maxTextChars int) int {
	units := 0
	cut := 0
	for i, r := range text {
		width := 1
		if r > 0xFFFF {
			width = 2
		}
		if units+width > maxTextChars {
			break
		}
		units += width
		cut = i + len(string(r))
	}
	if cut <= 0 {
		// 第一段就超限（比如上限 1 而首字符是增补平面）：整字符切出去，不空转。
		for i, r := range text {
			_ = i
			return len(string(r))
		}
		return 0
	}
	// 退到最近的空白处（不进则退：找不到就不退，总比空片段强）。
	if space := strings.LastIndexAny(text[:cut], " \t\r\n"); space > 0 {
		return space
	}
	return cut
}

// cotUTF16Len 数 UTF-16 码元（与宿主 cotTextUTF16Len / 官方 maxTextChars 同一口径）。
//
// 不能用 len([]rune)：那会把 emoji 算成 1，按上限发满时会被宿主多拒一段。
func cotUTF16Len(text string) int {
	n := 0
	for _, r := range text {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// cotDisplayText 是官方 emit() 的移植：逐段拼，有译文用译文、没有就用原文。
//
// 这条规则让"翻到一半"也能显示：已翻好的段落是译文，没翻/翻失败的段落还是原文。
func cotDisplayText(fragments []cotFragment, cache map[string]string) string {
	var b strings.Builder
	for _, fragment := range fragments {
		if translated, ok := cache[fragment.Text]; ok && fragment.Complete {
			b.WriteString(translated)
		} else {
			b.WriteString(fragment.Text)
		}
		b.WriteString(fragment.Suffix)
	}
	return b.String()
}

// cotNextFragment 找下一段该翻的原文（官方 run() 里那个 find 的同一判据）：
// **已完成、不是纯空白、还没有译文**的第一段。
func cotNextFragment(fragments []cotFragment, cache map[string]string) (cotFragment, bool) {
	for _, fragment := range fragments {
		if !fragment.Complete || strings.TrimSpace(fragment.Text) == "" {
			continue
		}
		if _, ok := cache[fragment.Text]; ok {
			continue
		}
		return fragment, true
	}
	return cotFragment{}, false
}

// cotEntryFor 取（必要时建）一条消息的译文状态。
func (p *Plugin) cotEntryFor(key string) *cotEntry {
	if p.cot == nil {
		p.cot = map[string]*cotEntry{}
	}
	entry, ok := p.cot[key]
	if !ok {
		entry = &cotEntry{cache: map[string]string{}}
		p.cot[key] = entry
	}
	if entry.cache == nil {
		entry.cache = map[string]string{}
	}
	return entry
}

// cotTargetLanguage 把偏好里的 `auto` 解析成界面语言（官方 TranslationBody 同一句）：
// `targetLanguage === 'auto' ? locale.active : targetLanguage`。
//
// 本仓界面只有中文，所以 active 固定 zh-CN（与 cmd/app/plugin_manager.go:37 的
// pluginDetailLanguage 同一口径）。注意解析发生在**客户端**，不是宿主：
// 宿主 translate 只认具体 tag。
func cotTargetLanguage(preferences apiclient.CotTranslationPreferences) string {
	if preferences.TargetLanguage == "" || preferences.TargetLanguage == "auto" {
		return cotUILanguage
	}
	return preferences.TargetLanguage
}

// cotUILanguage 是本仓界面的语言（本仓只有中文界面）。
const cotUILanguage = "zh-CN"

// cotSourceText 是**要翻译的**思考正文：去掉模型签名块后的文本。
//
// 为什么不直接翻 msg.Text：签名是模型侧的加密/校验块（base64），翻它没有意义，
// 而且它占掉一半请求预算。展开态正文仍然原样显示 msg.Text（reasoning.go 那条
// "刻意差异"），只有译文视图用清理过的这一份。
func cotSourceText(msg apiclient.Message) string { return reasoningDisplayText(msg.Text) }

// cotAdvance 推进一步译文状态机（每帧最多一次迁移）。
//
// 三种迁移：
//
//	收尾  有 job 在飞 → 问壳要终态；拿到就落缓存（或置 failed）并清 job；
//	发起  idle 且还有该翻的段 → 发起一次请求，记住 job/jobText；
//	放弃  壳没接（jobID 空）→ 置 failed（画「翻译暂不可用，已显示原文」），
//	      不画一个点了没反应的按钮。
//
// 返回 true 表示这一帧状态有变化（调用方不必处理，Viva 下一帧照常重画）。
func (p *Plugin) cotAdvance(act pluginapi.Actions, sessionID, key string, source string,
	running bool, limits apiclient.CotTranslationLimits) *cotEntry {
	entry := p.cotEntryFor(key)

	// 文本变了（流式还在写）就重算分段；官方同样每次 update 都重算。
	// 但**只在没有请求在飞时**重算：在飞的那一段的原文以 jobText 为准，
	// 重算不会把它的文本悄悄改掉（缓存键是原文，改了就会再翻一次）。
	if entry.source != source && !entry.pending {
		entry.fragments = cotFragments(source, running, limits.MaxTextChars)
		entry.source = source
	}

	// ① 收尾：有请求在飞就只要结果，不发起新的。
	if entry.pending {
		if entry.job == "" {
			entry.pending = false
			return entry
		}
		result, ok := act.CotTranslationResult(entry.job)
		if !ok {
			return entry // 还在飞
		}
		entry.job = ""
		entry.pending = false
		if !result.OK {
			entry.failed = true
			return entry
		}
		if entry.jobText != "" {
			entry.cache[entry.jobText] = result.Text
		}
		entry.jobText = ""
		return entry
	}

	// ② 失败过就停在这里，等用户点「重试」（官方 failed 分支同一语义）。
	if entry.failed {
		return entry
	}

	// ③ 发起下一段。
	next, ok := cotNextFragment(entry.fragments, entry.cache)
	if !ok {
		return entry
	}
	job := act.CotTranslationTranslate(sessionID, next.Text, cotTargetLanguage(limits.Preferences),
		limits.Preferences.Provider)
	if job == "" {
		entry.failed = true
		return entry
	}
	entry.job = job
	entry.jobText = next.Text
	entry.pending = true
	return entry
}

// cotRetry 清掉失败态并重新排队（官方 retry()：`failed=false; run()`）。
//
// 已翻好的段留在 cache 里，所以"重试"只补没翻的那几段，不重复付费。
func (p *Plugin) cotRetry(key string) {
	if entry, ok := p.cot[key]; ok {
		entry.failed = false
		entry.job = ""
		entry.jobText = ""
	}
}

// drawCotToolbar 画译文工具栏（按钮 + pending 状态 + 失败文案与重试）。
//
// 几何与官方同一克制：一行 28 高、按钮 ghost/small、失败文案 13px。
// 文案逐字来自 OFF:src/client/locales.ts:17-18（中文那一份）。
func (p *Plugin) drawCotToolbar(f ui.C, env *pluginapi.Env, key string, entry *cotEntry) {
	t := p.theme
	// failed 时官方强制显示原文（`showOriginal = original || state.failed`），
	// 所以按钮文案按"显示的是原文"算，且此时按钮不可用（官方 `disabled={state.failed}`）。
	showOriginal := entry.failed || entry.original
	label := cotLabelShowTranslation
	if !showOriginal {
		label = cotLabelShowOriginal
	}
	ui.Row(f).WidthPercent(100).MinHeight(28).Items(ui.Center).Gap(6).Children(func() {
		if ui.Button(f, label).Ghost().Small().Disabled(entry.failed).Clicked() {
			entry.original = !showOriginal
		}
		if entry.pending && !entry.failed {
			ui.Text(f, cotLabelTranslating).FontSize(t.Fonts.Secondary).Color(t.LabelCaption)
		}
		if entry.failed {
			ui.Text(f, cotLabelFailed).FontSize(t.Fonts.Secondary).Color(t.LabelPrimary)
			if ui.Button(f, cotLabelRetry).Ghost().Small().
				Hit(env.Tree.ID(env.Feat, "cot-retry-"+key), cotLabelRetry).Clicked() {
				p.cotRetry(key)
			}
		}
	})
}

// 官方中文文案（OFF:src/client/locales.ts:38-39），不改写。
const (
	cotLabelShowTranslation = "查看译文"
	cotLabelShowOriginal    = "查看原文"
	cotLabelRetry           = "重试"
	cotLabelFailed          = "翻译暂不可用，已显示原文"
	cotLabelTranslating     = "正在翻译思考内容"
)

// drawCotBody 画正文：默认显示译文，用户点了「查看原文」或翻译失败时显示原文。
//
// 与展开态原文同一个画法（FontSize Caption + LabelTertiary + ui.Wrap），
// 不另造 Markdown——理由见文件头第 2 条差异。
func (p *Plugin) drawCotBody(f ui.C, entry *cotEntry, original string) {
	t := p.theme
	showOriginal := entry == nil || entry.failed || entry.original
	text := original
	if !showOriginal {
		// 没翻完的段在 cotDisplayText 里自动退回原文，所以这里不会出现空正文。
		text = cotDisplayText(entry.fragments, entry.cache)
	}
	ui.Wrap(f, text).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
}

// cotToolbarAvailable 判断这一帧能不能画译文入口。
//
// 判据是"宿主真的能翻"：壳接了 limits，且清单里至少有 provider，
// 且偏好里的 provider 在清单内（不在就说明宿主把它列成不可用——官方那时画
// providerUnavailable，本仓没有设置页，就直接不画入口）。
func cotToolbarAvailable(act pluginapi.Actions) (apiclient.CotTranslationLimits, bool) {
	if act.CotTranslationLimits == nil || act.CotTranslationTranslate == nil || act.CotTranslationResult == nil {
		return apiclient.CotTranslationLimits{}, false
	}
	limits, ok := act.CotTranslationLimits()
	if !ok || len(limits.AvailableProviders) == 0 {
		return apiclient.CotTranslationLimits{}, false
	}
	for _, provider := range limits.AvailableProviders {
		if provider == limits.Preferences.Provider {
			return limits, true
		}
	}
	return apiclient.CotTranslationLimits{}, false
}
