// cot_translation_test.go —— #63 思考译文里**能脱离真机验证**的判据。
//
// 三块：
//
//	分段       cotFragments 逐条对齐官方 reasoningFragments（含代理对与超长切分）
//	拼接       cotDisplayText 对齐官方 emit()：有译文用译文、没有用原文
//	状态机     cotAdvance / cotRetry：同一时刻一个请求、失败即停、重试只补没翻的
//
// 绘制与点击由 cot_translation_visual_test.go 走真 Viva 验（pump 到条件成立）。
package dsconversation

import (
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// cotFakeActions 是三条 Action 的假实现：请求先挂起，测试决定何时"回包"。
//
// 这样测的正是真实时序——插件发起 → 壳还没回 → 插件不该发第二个；
// 回包之后才推进下一段。
type cotFakeActions struct {
	limits  apiclient.CotTranslationLimits
	loaded  bool
	calls   []cotCall
	pending map[string]cotCall
	results map[string]pluginapi.CotTranslationResult
	seq     int
}

// cotCall 是一次被发起的翻译。
type cotCall struct {
	sessionID      string
	text           string
	targetLanguage string
	provider       string
}

func newCotFakeActions() *cotFakeActions {
	return &cotFakeActions{
		limits: apiclient.CotTranslationLimits{
			MaxTextChars: 100000,
			Preferences: apiclient.CotTranslationPreferences{
				Provider:       apiclient.CotTranslationProviderOfficial,
				TargetLanguage: "auto",
			},
			AvailableProviders: []string{apiclient.CotTranslationProviderOfficial, apiclient.CotTranslationProviderAccount},
		},
		loaded:  true,
		pending: map[string]cotCall{},
		results: map[string]pluginapi.CotTranslationResult{},
	}
}

// actions 把假实现装成插件看到的那组 Action。
func (f *cotFakeActions) actions() pluginapi.Actions {
	return pluginapi.Actions{
		CotTranslationLimits: func() (apiclient.CotTranslationLimits, bool) {
			return f.limits, f.loaded
		},
		CotTranslationTranslate: func(sessionID, text, targetLanguage, provider string) string {
			f.seq++
			job := fmt.Sprintf("job-%d", f.seq)
			call := cotCall{sessionID: sessionID, text: text, targetLanguage: targetLanguage, provider: provider}
			f.calls = append(f.calls, call)
			f.pending[job] = call
			return job
		},
		CotTranslationResult: func(job string) (pluginapi.CotTranslationResult, bool) {
			result, ok := f.results[job]
			if ok {
				delete(f.results, job)
			}
			return result, ok
		},
	}
}

// reply 让最老的那个在飞请求回包（翻译成 "T(<原文>)" 便于断言）。
func (f *cotFakeActions) reply(t *testing.T) {
	t.Helper()
	for job, call := range f.pending {
		delete(f.pending, job)
		f.results[job] = pluginapi.CotTranslationResult{Text: "T(" + call.text + ")", OK: true}
		return
	}
	t.Fatalf("没有在飞的请求可回包；calls=%+v", f.calls)
}

// replyFail 让最老的那个在飞请求以失败回包。
func (f *cotFakeActions) replyFail(t *testing.T, code, message string) {
	t.Helper()
	for job := range f.pending {
		delete(f.pending, job)
		f.results[job] = pluginapi.CotTranslationResult{Code: code, Message: message}
		return
	}
	t.Fatalf("没有在飞的请求可回包；calls=%+v", f.calls)
}

// TestCotFragmentsSplitsOnBlankLineParagraphs 钉住官方分段判据。
func TestCotFragmentsSplitsOnBlankLineParagraphs(t *testing.T) {
	text := "第一段\n\n第二段\n\n第三段还在写"
	fragments := cotFragments(text, true, 1000)
	if len(fragments) != 3 {
		t.Fatalf("段数=%d，期望 3：%+v", len(fragments), fragments)
	}
	if fragments[0].Text != "第一段" || fragments[0].Suffix != "\n\n" {
		t.Fatalf("第 0 段=%+v，期望 text=第一段 suffix=两个换行", fragments[0])
	}
	if !fragments[0].Complete || !fragments[1].Complete {
		t.Fatalf("有分隔符的段落必须 Complete：%+v", fragments[:2])
	}
	if fragments[2].Complete {
		t.Fatalf("running 时最后一段不许算完成：%+v", fragments[2])
	}
	// 拼接必须逐字还原原文（Suffix 是原文的一部分，不能丢）。
	if got := cotDisplayText(fragments, nil); got != text {
		t.Fatalf("无译文时拼接=%q，期望原文 %q", got, text)
	}

	// 不 running 时最后一段也算完成（官方 `complete = match !== null || !running`）。
	done := cotFragments(text, false, 1000)
	if !done[2].Complete {
		t.Fatalf("running=false 时最后一段必须 Complete：%+v", done[2])
	}
}

// TestCotFragmentsWidensPastSurrogatePair 钉住"不许劈开代理对"。
//
// 官方切点规则是**先按 UTF-16 码元数退切点、再退到空白**；emoji 占 2 个码元，
// 切在它中间会切出半个字符（UTF-8 里就是损坏的 rune），译文直接乱码。
func TestCotFragmentsWidensPastSurrogatePair(t *testing.T) {
	text := "ab😀cd"
	// 上限 3：UTF-16 是 a(1) b(1) 😀(2) → 3 个码元刚好是 "ab"，不该把 emoji 劈成一半。
	fragments := cotFragments(text, false, 3)
	for _, fragment := range fragments {
		if !utf8.ValidString(fragment.Text) || !utf8.ValidString(fragment.Suffix) {
			t.Fatalf("切出了非法 UTF-8（劈开了代理对）：%+v", fragments)
		}
	}
	if got := cotDisplayText(fragments, nil); got != text {
		t.Fatalf("拼接=%q，期望原文 %q", got, text)
	}
	// 每段都不许超过上限（按 UTF-16 码元）。
	for _, fragment := range fragments {
		if fragment.Text != "" && cotUTF16Len(fragment.Text) > 3 && fragment.Complete {
			t.Fatalf("片段超过 maxTextChars=%d：%q", 3, fragment.Text)
		}
	}
}

// TestCotFragmentsCutsLongParagraphAtWhitespace 钉住超长段落的切分与 UTF-16 口径。
func TestCotFragmentsCutsLongParagraphAtWhitespace(t *testing.T) {
	text := "aaaa bbbb cccc"
	fragments := cotFragments(text, false, 6)
	if len(fragments) < 2 {
		t.Fatalf("超长段落没被切：%+v", fragments)
	}
	if got := cotDisplayText(fragments, nil); got != text {
		t.Fatalf("拼接=%q，期望原文 %q（Suffix 丢了吗）", got, text)
	}
	for _, fragment := range fragments {
		if fragment.Complete && cotUTF16Len(fragment.Text) > 6 {
			t.Fatalf("片段 %q 超过上限 6（UTF-16）", fragment.Text)
		}
	}
	// 切点要落在空白边界上：任何片段都不该以半个词开头。
	if fragments[0].Text != "aaaa" {
		t.Fatalf("第一段=%q，期望切在空白处 aaaa", fragments[0].Text)
	}
}

// TestCotUTF16LenCountsAstralAsTwo 钉住 UTF-16 口径（与宿主 cotTextUTF16Len 一致）。
func TestCotUTF16LenCountsAstralAsTwo(t *testing.T) {
	if got := cotUTF16Len("ab😀"); got != 4 {
		t.Fatalf("cotUTF16Len(ab😀)=%d，期望 4（emoji 算 2 个码元）", got)
	}
	if got := len([]rune("ab😀")); got != 3 {
		t.Fatalf("前置断言错：rune 数应为 3，得到 %d", got)
	}
}

// TestCotDisplayTextMixesTranslatedAndOriginal 钉住官方 emit() 的拼接规则。
func TestCotDisplayTextMixesTranslatedAndOriginal(t *testing.T) {
	fragments := []cotFragment{
		{Text: "第一段", Suffix: "\n\n", Complete: true},
		{Text: "第二段", Suffix: "\n\n", Complete: true},
		{Text: "第三段还在写", Complete: false},
	}
	cache := map[string]string{"第一段": "P1"}
	got := cotDisplayText(fragments, cache)
	want := "P1\n\n第二段\n\n第三段还在写"
	if got != want {
		t.Fatalf("拼接=%q，期望 %q", got, want)
	}
	// 未完成段落即使被塞进 cache 也不许显示译文（官方 `fragment.complete ? … : original`）。
	cache["第三段还在写"] = "不该出现"
	if got := cotDisplayText(fragments, cache); got != want {
		t.Fatalf("未完成段落显示了译文：%q", got)
	}
}

// TestCotAdvanceSendsOneRequestAtATime 钉住"同一时刻只有一个请求在飞"。
func TestCotAdvanceSendsOneRequestAtATime(t *testing.T) {
	p := newStreamingMarkdownTestPlugin()
	fake := newCotFakeActions()
	act := fake.actions()
	source := "第一段\n\n第二段"

	entry := p.cotAdvance(act, "sess-1", "k1", source, false, fake.limits)
	if len(fake.calls) != 1 {
		t.Fatalf("首帧应发一次请求，实际 %d：%+v", len(fake.calls), fake.calls)
	}
	if !entry.pending {
		t.Fatal("发起后必须 pending")
	}
	if call := fake.calls[0]; call.sessionID != "sess-1" || call.text != "第一段" {
		t.Fatalf("第一次请求=%+v，期望 session=sess-1 text=第一段", call)
	}
	// 还没回包：再推进不许发第二个。
	p.cotAdvance(act, "sess-1", "k1", source, false, fake.limits)
	if len(fake.calls) != 1 {
		t.Fatalf("回包前又发了请求：%d 个", len(fake.calls))
	}

	fake.reply(t) // 第一段回包
	p.cotAdvance(act, "sess-1", "k1", source, false, fake.limits)
	if got := entry.cache["第一段"]; got != "T(第一段)" {
		t.Fatalf("第一段译文=%q，期望 T(第一段)", got)
	}
	// 同一帧只做一次迁移：这一帧刚收包，下一帧才发第二段。
	if len(fake.calls) != 1 {
		t.Fatalf("收包当帧就发了下一段（每帧最多一次迁移）：%d", len(fake.calls))
	}
	p.cotAdvance(act, "sess-1", "k1", source, false, fake.limits)
	if len(fake.calls) != 2 {
		t.Fatalf("下一帧应发第二段，实际 %d 个请求", len(fake.calls))
	}
	if fake.calls[1].text != "第二段" {
		t.Fatalf("第二段请求=%q", fake.calls[1].text)
	}

	fake.reply(t)
	p.cotAdvance(act, "sess-1", "k1", source, false, fake.limits)
	if entry.pending {
		t.Fatal("全部翻完仍是 pending")
	}
	if got, want := cotDisplayText(entry.fragments, entry.cache), "T(第一段)\n\nT(第二段)"; got != want {
		t.Fatalf("拼好的译文=%q，期望 %q", got, want)
	}
	// 再推进不许再发（全部已缓存）。
	p.cotAdvance(act, "sess-1", "k1", source, false, fake.limits)
	if len(fake.calls) != 2 {
		t.Fatalf("翻完后又发了请求：%d", len(fake.calls))
	}
}

// TestCotAdvanceTargetLanguageResolvesAuto 钉住 auto → 界面语言（官方同一句）。
func TestCotAdvanceTargetLanguageResolvesAuto(t *testing.T) {
	p := newStreamingMarkdownTestPlugin()
	fake := newCotFakeActions()
	act := fake.actions()
	p.cotAdvance(act, "sess-1", "k1", "一段话", false, fake.limits)
	if len(fake.calls) != 1 {
		t.Fatalf("没发请求")
	}
	if fake.calls[0].targetLanguage != "zh-CN" {
		t.Fatalf("targetLanguage=%q，期望 auto 解析成 zh-CN", fake.calls[0].targetLanguage)
	}
	if fake.calls[0].provider != apiclient.CotTranslationProviderOfficial {
		t.Fatalf("provider=%q，期望沿用 limits 里的偏好", fake.calls[0].provider)
	}

	// 显式语言码照传，不覆盖。
	p2 := newStreamingMarkdownTestPlugin()
	fake2 := newCotFakeActions()
	fake2.limits.Preferences.TargetLanguage = "ja"
	p2.cotAdvance(fake2.actions(), "sess-1", "k1", "一段话", false, fake2.limits)
	if fake2.calls[0].targetLanguage != "ja" {
		t.Fatalf("显式语言被改写成了 %q", fake2.calls[0].targetLanguage)
	}
}

// TestCotAdvanceStopsOnFailureAndRetryResumes 钉住失败即停 + 重试只补没翻的。
func TestCotAdvanceStopsOnFailureAndRetryResumes(t *testing.T) {
	p := newStreamingMarkdownTestPlugin()
	fake := newCotFakeActions()
	act := fake.actions()
	source := "第一段\n\n第二段"

	entry := p.cotAdvance(act, "sess-1", "k1", source, false, fake.limits)
	fake.reply(t)
	p.cotAdvance(act, "sess-1", "k1", source, false, fake.limits) // 收包
	p.cotAdvance(act, "sess-1", "k1", source, false, fake.limits) // 发第二段
	fake.replyFail(t, "not_implemented", "宿主没有 cotTranslation.translate")
	p.cotAdvance(act, "sess-1", "k1", source, false, fake.limits)
	if !entry.failed {
		t.Fatal("失败后必须 failed")
	}
	if got := len(fake.calls); got != 2 {
		t.Fatalf("失败后不该继续发请求，实际 %d 个", got)
	}
	// 失败态由 failed **强制**显示原文（官方 `showOriginal = original || failed`），
	// 用户的 original 偏好不用被改写——重试成功后仍回到译文视图。
	if entry.original {
		t.Fatal("失败不该把用户的 original 偏好改掉")
	}

	p.cotRetry("k1")
	if entry.failed {
		t.Fatal("retry 之后 failed 必须清掉")
	}
	p.cotAdvance(act, "sess-1", "k1", source, false, fake.limits)
	if got := len(fake.calls); got != 3 {
		t.Fatalf("retry 没有重新发起：%d 个请求", got)
	}
	if fake.calls[2].text != "第二段" {
		t.Fatalf("retry 重发了 %q，期望只补没翻的第二段", fake.calls[2].text)
	}
}

// TestCotToolbarAvailableRequiresRealCapability 钉住"宿主能翻才画入口"。
func TestCotToolbarAvailableRequiresRealCapability(t *testing.T) {
	fake := newCotFakeActions()
	if _, ok := cotToolbarAvailable(pluginapi.Actions{}); ok {
		t.Fatal("三条 Action 全 nil 时不许说可用")
	}
	if _, ok := cotToolbarAvailable(fake.actions()); !ok {
		t.Fatal("宿主能翻时必须可用")
	}

	// 还没拉到 limits：不许画（不猜上限、不猜 provider）。
	fake.loaded = false
	if _, ok := cotToolbarAvailable(fake.actions()); ok {
		t.Fatal("limits 还没拉到时不许说可用")
	}
	fake.loaded = true

	// 偏好里的 provider 不在宿主清单里：入口照样在（官方 providerUnavailable 只加一句话），
	// 但 cotProviderUnavailable 必须判出来——这是官方的"服务暂不可用"提示。
	fake.limits.Preferences.Provider = apiclient.CotTranslationProviderBing
	limits, ok := cotToolbarAvailable(fake.actions())
	if !ok {
		t.Fatal("provider 不可用时入口不该消失（官方同一个座位 + providerUnavailable 文案）")
	}
	if !cotProviderUnavailable(limits) {
		t.Fatal("provider 不在 availableProviders 里时必须判成不可用")
	}
	fake.limits.Preferences.Provider = apiclient.CotTranslationProviderOfficial
	if limits, ok := cotToolbarAvailable(fake.actions()); !ok || cotProviderUnavailable(limits) {
		t.Fatal("provider 在清单里时不该判成不可用")
	}

	// 空清单 = 一个都翻不了。
	fake.limits.AvailableProviders = nil
	if _, ok := cotToolbarAvailable(fake.actions()); ok {
		t.Fatal("空 provider 清单时不许说可用")
	}
}

// TestCotAdvanceWithoutTranslatorFailsLoud 钉住"壳没接 translate 就失败，不空转"。
func TestCotAdvanceWithoutTranslatorFailsLoud(t *testing.T) {
	p := newStreamingMarkdownTestPlugin()
	fake := newCotFakeActions()
	act := fake.actions()
	act.CotTranslationTranslate = func(string, string, string, string) string { return "" }
	entry := p.cotAdvance(act, "sess-1", "k1", "一段话", false, fake.limits)
	if !entry.failed {
		t.Fatal("translate 返回空 jobID 时必须 failed（不是永远 pending）")
	}
	if entry.pending {
		t.Fatal("failed 时不许同时 pending")
	}
}

// TestCotSourceTextDropsSignature 钉住"只翻可显示的思考，不翻签名块"。
func TestCotSourceTextDropsSignature(t *testing.T) {
	msg := apiclient.Message{Text: "<signature>c2ln</signature>\n真正的思考"}
	if got := cotSourceText(msg); got != "真正的思考" {
		t.Fatalf("待翻译文本=%q，期望去掉签名块", got)
	}
}
