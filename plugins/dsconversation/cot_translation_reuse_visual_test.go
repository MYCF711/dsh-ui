//go:build darwin || windows || linux

// cot_translation_reuse_visual_test.go —— #63「收起/重开不许重新付费」的守卫。
//
// # 这条为什么值一个真机守卫
//
// 官方 `ReasoningTranslation` 的 cache 只活在一次 disclosure 挂载期间：收起 =
// unmount = `controller.dispose()` = `abort()`，重开就是一个空 cache。官方之所以
// 仍然「重开不重复付费」，是因为**宿主**有一层按会话登记的持久译文缓存
// （`OFF:dsh-experimental-translator/src/storage.ts:52-54` 的 identityKey =
// provider + 原文 + 源语言 + 目标语言 + recipe，落成 `plugin:translator/request`
// / `plugin:translator/result` 两条会话记录），所以文案才敢写「重新展开会复用
// 已保存的译文」（`OFF:src/client/locales.ts` 的 paidNotice 中文原文）。
//
// 本仓宿主**没有**这层持久缓存（`HOST:internal/transport/handlers_media_market_official.go`
// 的 `officialCotTranslationTranslate` 把 sessionId 解析完就丢掉，每次都真打模型网关），
// 所以「不重复付费」只能靠界面侧的 per-fragment cache 兜住（key = 原文片段，
// 与官方 identityKey 同口径 ⇒ 原文变了自然是新键，没有"旧译文套新原文"这种失效问题）。
//
// 这条行为是**用户的真金白银**，但之前没有任何测试钉它：改坏 cache 的生命周期
// （收起时清 `p.cot[key]`、或每次 `cotEntryFor` 重建 entry）不会有任何断言变红。
// 本文件补上这两条。
package dsconversation

import (
	"fmt"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// cotHeldTranslator 是"结果晚一点到"的假宿主：前 `holdFor` 次取结果都回 not-ok，
// 用来把请求**卡在 pending**，好在收起态观察"在飞的那次请求有没有被丢掉"。
type cotHeldTranslator struct {
	holdFor  int
	takes    int
	calls    []cotCall
	results  map[string]pluginapi.CotTranslationResult
	jobSeq   int
	released bool
}

func newCotHeldTranslator(holdFor int) *cotHeldTranslator {
	return &cotHeldTranslator{holdFor: holdFor, results: map[string]pluginapi.CotTranslationResult{}}
}

func (a *cotHeldTranslator) actions() pluginapi.Actions {
	return pluginapi.Actions{
		CotTranslationLimits: func() (apiclient.CotTranslationLimits, bool) {
			return apiclient.CotTranslationLimits{
				MaxTextChars: 100000,
				Preferences: apiclient.CotTranslationPreferences{
					Provider:       apiclient.CotTranslationProviderOfficial,
					TargetLanguage: "auto",
				},
				AvailableProviders: []string{apiclient.CotTranslationProviderOfficial},
			}, true
		},
		CotTranslationTranslate: func(sessionID, text, targetLanguage, provider string) string {
			a.jobSeq++
			job := fmt.Sprintf("held-%d", a.jobSeq)
			a.calls = append(a.calls, cotCall{sessionID: sessionID, text: text,
				targetLanguage: targetLanguage, provider: provider})
			a.results[job] = pluginapi.CotTranslationResult{Text: "T(" + text + ")", OK: true}
			return job
		},
		CotTranslationResult: func(job string) (pluginapi.CotTranslationResult, bool) {
			result, ok := a.results[job]
			if !ok {
				return pluginapi.CotTranslationResult{}, false
			}
			if !a.released && a.takes < a.holdFor {
				a.takes++
				return pluginapi.CotTranslationResult{}, false
			}
			delete(a.results, job)
			return result, true
		},
	}
}

// TestCotTranslationReexpandReusesTheCache 是本单的核心守卫：**收起再展开，一次都不许再发请求**。
//
// 判据取的是"翻译请求条数"，不是"界面看起来一样"——后者在缓存被清掉时也会成立
// （重新翻一遍，最后还是画出同样的译文），根本分辨不出有没有重复付费。
func TestCotTranslationReexpandReusesTheCache(t *testing.T) {
	translator := newCotAutoTranslator()
	h := openCotHarness(t, 1182, 832, cotShotData(), translator.actions())
	defer h.close()

	h.expandThinking(1)
	h.pumpUntil("两段都翻完", func() bool {
		return h.hasLine("T(先看目录结构。)") && h.hasLine("T(再对齐会话加载路径。)")
	})
	afterFirstExpand := len(translator.calls)
	if afterFirstExpand == 0 {
		t.Fatal("画了译文却一个请求都没发")
	}

	// 收起（再点同一条思考行），并在收起态多推几帧：收起态**不许**推进状态机。
	h.expandThinking(1)
	for i := 0; i < 5; i++ {
		h.pump()
	}
	if h.hasLine("T(先看目录结构。)") {
		t.Fatalf("收起后仍在画展开态译文：%q", h.session.Texts())
	}

	// 重新展开：译文必须立刻回来，且**不许**再发请求。
	h.expandThinking(1)
	h.pumpUntil("译文回来", func() bool {
		return h.hasLine("T(先看目录结构。)") && h.hasLine("T(再对齐会话加载路径。)")
	})
	for i := 0; i < 5; i++ {
		h.pump()
	}
	if got := len(translator.calls); got != afterFirstExpand {
		texts := make([]string, 0, got)
		for _, c := range translator.calls[afterFirstExpand:] {
			texts = append(texts, c.text)
		}
		t.Fatalf("收起再展开又发了 %d 次翻译请求（累计 %d，首次展开后 %d）：同一段原文被重复付费，重复的片段=%q",
			got-afterFirstExpand, got, afterFirstExpand, texts)
	}
}

// TestCotCollapseKeepsTheInFlightFragmentPaid 钉住"收起时不取消在飞请求"这个**刻意的省钱选择**。
//
// 官方收起即 abort：那次请求的钱已经花了一半、结果却被丢掉，重开要重发一次
// （官方靠宿主持久缓存把这一笔补回来，本仓宿主没有那层，只能靠不丢结果）。
// 本仓的选择是：请求跑完、结果照样落 cache，所以**同一片段一辈子只付一次**。
// 变异点：收起时把 entry 清掉 / 把 job 丢掉 ⇒ 重开时会看到 `held-2` 这种第二次请求。
func TestCotCollapseKeepsTheInFlightFragmentPaid(t *testing.T) {
	translator := newCotHeldTranslator(8)
	h := openCotHarness(t, 1182, 832, cotShotData(), translator.actions())
	defer h.close()

	h.expandThinking(1)
	// 第一段已经发出去、结果还没到（每次 take 都回 not-ok）。
	h.pumpUntil("第一段请求在飞", func() bool { return len(translator.calls) == 1 })
	if got := translator.calls[0].text; got != "先看目录结构。" {
		t.Fatalf("第一段请求的原文=%q", got)
	}

	// 在飞期间收起，多推几帧（收起态不推进 ⇒ 结果仍留在宿主的 results 里）。
	h.expandThinking(1)
	for i := 0; i < 5; i++ {
		h.pump()
	}

	// 放行并重新展开：那次在飞的请求必须被复用，不许重新发一次。
	translator.released = true
	h.expandThinking(1)
	h.pumpUntil("两段都翻完", func() bool {
		return h.hasLine("T(先看目录结构。)") && h.hasLine("T(再对齐会话加载路径。)")
	})
	if got := len(translator.calls); got != 2 {
		texts := make([]string, 0, got)
		for _, c := range translator.calls {
			texts = append(texts, c.text)
		}
		t.Fatalf("收起期间在飞的那一段被丢了：期望共 2 次请求（两段各一次），实际 %d 次，原文=%q", got, texts)
	}
	if translator.calls[0].text == translator.calls[1].text {
		t.Fatalf("同一段原文被发了两次：%q", translator.calls[0].text)
	}
}
