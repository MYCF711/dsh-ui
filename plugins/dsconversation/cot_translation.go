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
	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
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

// drawCotToolbar 画译文工具栏（按钮 + pending spinner + 失败文案与重试）。
//
// 几何逐条取官方 `Translation.module.css:1-13`：
//
//	`.toolbar{display:flex;align-items:center;flex-wrap:wrap;gap:4px;min-height:28px;margin-bottom:4px}`
//	`.toolbar > .viewToggle{margin-inline-start:-10px;color:label-tertiary}`
//	`.failure{color:label-primary;font-size:13px}`
//	pending 出的是 `IconLoadingOutlineRegular size=14` 的转圈图标 + `aria-label=translating`，
//	不是一行文字（`OFF:TranslationBody.tsx:53-55`）。本仓用 Viva 的 `ui.Spinner` 画同一个
//	意思——它没有官方那个 14px 图标的几何，这一点登记在账本行。
//
// 文案逐字来自 OFF:src/client/locales.ts（中文那一份）。
func (p *Plugin) drawCotToolbar(f ui.C, env *pluginapi.Env, key string, entry *cotEntry, unavailable bool) {
	t := p.theme
	// failed 时官方强制显示原文（`showOriginal = original || state.failed`），
	// 所以按钮文案按"显示的是原文"算，且此时按钮不可用（官方 `disabled={state.failed}`）。
	showOriginal := entry.failed || entry.original
	label := cotLabelShowTranslation
	if !showOriginal {
		label = cotLabelShowOriginal
	}
	ui.Row(f).WidthPercent(100).MinHeight(28).Items(ui.Center).Gap(4).
		PadSides(0, 0, 4, 0).Children(func() {
		ui.Row(f).Shrink(0).Children(func() {
			// `.toolbar > .viewToggle{margin-inline-start:-10px}`：把 sm Button 的
			// 10px 内边距抵掉，让按钮文字与正文左缘对齐。
			ui.Row(f).PadLeft(-cotToolbarToggleInset).Children(func() {
				if ui.Button(f, label).Ghost().Small().Disabled(entry.failed).Clicked() {
					entry.original = !showOriginal
				}
			})
		})
		if entry.pending && !entry.failed {
			// 官方是 `IconLoadingOutlineRegular` 14px + `aria-label=translating`。
			ui.Spinner(f)
		}
		if entry.failed {
			ui.Text(f, cotLabelFailed).FontSize(cotFailFontSize).Color(t.LabelPrimary)
			if ui.Button(f, cotLabelRetry).Ghost().Small().
				Hit(env.Tree.ID(env.Feat, "cot-retry-"+key), cotLabelRetry).Clicked() {
				p.cotRetry(key)
			}
		}
		if unavailable {
			ui.Text(f, cotLabelProviderUnavailable).FontSize(t.Fonts.Secondary).Color(t.LabelSecondary)
		}
	})
}

// cotToolbarToggleInset 是官方 `.viewToggle{margin-inline-start:-10px}`
// （`OFF:Translation.module.css:5`）。
const cotToolbarToggleInset float32 = 10

// cotFailFontSize 是官方 `.failure{font-size:var(--dsh-content-font-size-secondary,13px)}`
// （`OFF:Translation.module.css:10`）。
const cotFailFontSize float32 = 13

// 官方中文文案（OFF:src/client/locales.ts），不改写。
const (
	cotLabelShowTranslation = "查看译文"
	cotLabelShowOriginal    = "查看原文"
	cotLabelRetry           = "重试"
	cotLabelFailed          = "翻译暂不可用，已显示原文"
	cotLabelTranslating     = "正在翻译思考内容"
	// cotLabelProviderUnavailable 是官方 `providerUnavailable`
	// （`OFF:locales.ts:31` 中文逐字）：偏好里的 provider 不在可用清单里。
	cotLabelProviderUnavailable = "此服务暂不可用，请配置 DeepSeek 账号或官方 API 服务，或选择可用的翻译服务"
)

// drawCotBody 画正文：默认显示译文，用户点了「查看原文」或翻译失败时显示原文。
//
// **正文走官方那条 Markdown 工厂**：官方 `TranslationBody.tsx:60` 是
// `renderFactorySlot('conversation.chat.reasoning.content', {text, running})`，
// 而这个工厂的默认实现就是 `MarkdownText variant="compact"`
// （`OFF:dsh-client-ui-chat/src/client/chat/ReasoningContent.tsx:14-17`）——
// 所以译文/原文都要 Markdown 排版（带代码块工具条），不是纯文本折行。
// 本仓没有工厂槽，等价物是"排版后覆画"的 `mdSlot`（`conversation.go` 里助手正文
// 用的是同一套）；这里把正文排成一条 reasoning 专用的 slot，由 afterDraw 画。
func (p *Plugin) drawCotBody(f ui.C, env *pluginapi.Env, key string, entry *cotEntry, original string, _ bool) {
	t := p.theme
	showOriginal := entry == nil || entry.failed || entry.original
	text := original
	if !showOriginal {
		// 没翻完的段在 cotDisplayText 里自动退回原文，所以这里不会出现空正文。
		text = cotDisplayText(entry.fragments, entry.cache)
	}
	width := env.MainWidth - cotBodyWidthInset
	if width < cotBodyMinWidth {
		width = cotBodyMinWidth
	}
	// 下标只从 key 散列一次：同一个 key 必须命中同一格 Markdown 缓存，
	// 否则每帧都会重新 Parse+Layout（实测写成两处调用时正文一个字都画不出来）。
	slot := reasoningSlotSeq(key)
	height := p.layoutReasoningMarkdown(env.Canvas, slot, text, width)
	p.slots = append(p.slots, mdSlot{
		idx: slot,
		key: key + "\x00reasoning",
		box: ui.Slot(f).Height(height),
	})
	_ = t
}

// convReasoningSlotBase 是思考正文在 Markdown 缓存里占的下标区起点。
//
// 助手正文用 0..len(Msgs)-1；思考正文用 1000 起的**稳定散列**下标，两者不撞，
// 也不随消息数变化把彼此挤掉（p.docs/p.views 是按下标增长的切片，见
// `layoutMarkdownWithFiles`）。
const convReasoningSlotBase = 1000

// reasoningSlotSeq 把一条思考的 key 映射成 [base, base+1024) 里的稳定下标。
//
// 用 FNV-1a 而不是 len：key 含会话 id 与消息下标，散列后同一段思考永远同一格，
// 换会话/换顺序不会互相踩。碰撞概率在这里无所谓——撞了只是共用一份排版缓存，
// 而两张卡同帧不会同时展开同一段思考。
func reasoningSlotSeq(key string) int {
	const offset, prime = 2166136261, 16777619
	hash := uint32(offset)
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= prime
	}
	return convReasoningSlotBase + int(hash%1024)
}

// layoutReasoningMarkdown 排版一段思考正文并返回它的高度（"槽位"同一次排完）。
//
// 与 `layoutMarkdownWithFiles` 的差别只有三点，都是官方的：
//   - 字号用 `variant="compact"`（`ReasoningContent.tsx:16`）：正文小一号
//     （theme.Fonts.Secondary）、行高同一档；
//   - 不做文件链接改写（思考里没有"落盘路径"上下文）；
//   - 流式状态交给 Markdown 自己（`streaming={running}`）。
func (p *Plugin) layoutReasoningMarkdown(canvas *gfx.Canvas, slot int, text string, width float32) float32 {
	if p.codeBlocks == nil {
		p.codeBlocks = map[int][]messageCode{}
	}
	if p.fileLinks == nil {
		p.fileLinks = map[int]map[string]string{}
	}
	for len(p.docs) <= slot {
		p.docs = append(p.docs, markdown.Document{})
		p.views = append(p.views, components.MarkdownView{})
		p.texts = append(p.texts, "")
		p.rendered = append(p.rendered, "")
		p.linkKeys = append(p.linkKeys, "")
		p.widths = append(p.widths, -1)
		p.heights = append(p.heights, 0)
	}
	// 切片增长时 **texts 等缓存键的零值恰好等于空串 / -1**，而空正文也真是 ""：
	// 只比 `p.texts[slot] != text` 会让"第一帧正文就是空"的槽位永远不排版
	// （实测：span 有了、Markdown 高度 0、一个字都不画）。所以空正文也要排一次。
	if p.texts[slot] != text || p.widths[slot] != width || p.heights[slot] == 0 {
		p.texts[slot] = text
		p.rendered[slot] = text
		p.linkKeys[slot] = ""
		p.widths[slot] = width
		p.docs[slot].Parse(text)
		p.views[slot].Layout(canvas, components.MarkdownStyle{
			Size: p.theme.Fonts.Secondary, Highlight: mdlite.Highlight,
		}, &p.docs[slot], width)
	}
	p.rebuildCodeBlocks(canvas, slot, width)
	p.heights[slot] = p.views[slot].Height() + p.markdownExtraHeight(slot)
	return p.heights[slot]
}

// cotBodyWidthInset 是思考正文的可用宽度扣除：展开态盒子的左右内边距
// （`Pad(8)` = 8px×2）再留一点余量给 Markdown 自己的边距。
const cotBodyWidthInset float32 = 20

// cotBodyMinWidth 是排版的最小宽度（窄窗口下不许排成 0 宽）。
const cotBodyMinWidth float32 = 120

// cotToolbarAvailable 判断这一帧能不能画译文入口。
//
// 判据是"宿主真的能翻"：壳接了 limits，且清单里至少有 provider。
// **偏好里的 provider 不在清单内也要画**：官方那时照样挂座位，只是会多一句
// `providerUnavailable`（`OFF:TranslationSettings.tsx:58-60`）——这是设置里
// 可选的服务问题，不是"这个功能没有"，藏掉入口只会让人找不到原因。
func cotToolbarAvailable(act pluginapi.Actions) (apiclient.CotTranslationLimits, bool) {
	if act.CotTranslationLimits == nil || act.CotTranslationTranslate == nil || act.CotTranslationResult == nil {
		return apiclient.CotTranslationLimits{}, false
	}
	limits, ok := act.CotTranslationLimits()
	if !ok || len(limits.AvailableProviders) == 0 {
		return apiclient.CotTranslationLimits{}, false
	}
	return limits, true
}

// cotProviderUnavailable 是"当前 provider 不在可用清单里"（官方 `providerUnavailable`，
// `OFF:locales.ts:31` 中文逐字）。只有宿主确实报了清单（Loaded）才判——
// 还没读到时把 provider 说成不可用就是编。
func cotProviderUnavailable(limits apiclient.CotTranslationLimits) bool {
	if len(limits.AvailableProviders) == 0 {
		return false
	}
	for _, provider := range limits.AvailableProviders {
		if provider == limits.Preferences.Provider {
			return false
		}
	}
	return true
}
