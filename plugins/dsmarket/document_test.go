// document_test.go —— 详情文档五种状态与六档失败文案。
//
// 判据对着官方两个文件：
//
//	状态投影  OFF:../dsh-client-ui-plugin-market/src/client/PluginDetailSection.tsx:37-46
//	失败文案  OFF:../dsh-client-ui-plugin-market/src/client/locales.ts:87-92（FAILURE_KEYS 的取值）
//
// 数据面走 `pluginapi.Actions.MarketEntryDetail`（壳缓存 + 壳发请求）：插件只读结果，
// 不持数据源，也就不可能在 Draw 里偷偷发请求。
package dsmarket

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

// envWithDetail 造一个"壳读到了"的 Env。
func envWithDetail(detail apiclient.PluginMarketEntryDetail, loaded bool,
	failure apiclient.PluginMarketDetailFailure) *pluginapi.Env {
	return &pluginapi.Env{Act: pluginapi.Actions{
		MarketEntryDetail: func(url, name, version, language string) (apiclient.PluginMarketEntryDetail, bool, apiclient.PluginMarketDetailFailure) {
			return detail, loaded, failure
		},
	}}
}

// envWithRetry 在 envWithDetail 之上加"能重读"，把每次重试的参数记下来。
func envWithRetry(detail apiclient.PluginMarketEntryDetail, loaded bool,
	failure apiclient.PluginMarketDetailFailure, seen *[4]string) *pluginapi.Env {
	env := envWithDetail(detail, loaded, failure)
	env.Act.MarketEntryDetailRetry = func(url, name, version, language string) {
		*seen = [4]string{url, name, version, language}
	}
	return env
}

// envNoSource 造一个"壳没接详情读"的 Env。
func envNoSource() *pluginapi.Env { return &pluginapi.Env{} }

// filledRequest 是一次形状完整的详情读身份（缺 version 的那种另有专门用例）。
func filledRequest() ReadRequest {
	return ReadRequest{URL: "https://m.example/index.json", Name: "demo", Version: "1.0.0"}
}

func newTestPage() *Page {
	return NewPage(nil)
}

func TestDocumentStateAbsentWhenShellHasNoSource(t *testing.T) {
	if got := newTestPage().document(envNoSource(), filledRequest()).state; got != documentAbsent {
		t.Fatalf("壳没接详情读时应当是 absent（整段不画），得到 %v", got)
	}
}

// TestDocumentStateAbsentWithoutVersion 钉住"没有版本号就不读"。
//
// 宿主 readEntryDetail 缺 version 会回 400；宁可在这里就不发请求，也不要去撞一个
// 注定失败的调用（界面会白闪一行失败文案）。
func TestDocumentStateAbsentWithoutVersion(t *testing.T) {
	env := envWithDetail(apiclient.PluginMarketEntryDetail{Status: apiclient.PluginDetailFound, Markdown: "x"}, true, "")
	if got := newTestPage().document(env, ReadRequest{Name: "demo"}).state; got != documentAbsent {
		t.Fatalf("没有版本号时应当是 absent，得到 %v", got)
	}
}

func TestDocumentStateLoadingBeforeFirstAnswer(t *testing.T) {
	page := newTestPage()
	if got := page.document(envWithDetail(apiclient.PluginMarketEntryDetail{}, false, ""), filledRequest()).state; got != documentLoading {
		t.Fatalf("还没有结果时应当是 loading，得到 %v", got)
	}
}

// TestDocumentStateNoneIsNotAnError 钉住"none 是回答不是失败"：宿主明确说这一版
// 没有文档时，不该给用户画一行红字。
func TestDocumentStateNoneIsNotAnError(t *testing.T) {
	for _, detail := range []apiclient.PluginMarketEntryDetail{
		{Status: apiclient.PluginDetailNone},
		// found 但正文为空，官方 `DetailDocument` 同样什么都不画（PluginDetailSection.tsx:44-45）。
		{Status: apiclient.PluginDetailFound, Markdown: ""},
	} {
		page := newTestPage()
		if got := page.document(envWithDetail(detail, true, ""), filledRequest()).state; got != documentNone {
			t.Fatalf("%+v 应当是 none，得到 %v", detail, got)
		}
	}
}

func TestDocumentStateFoundKeepsMarkdown(t *testing.T) {
	env := envWithDetail(apiclient.PluginMarketEntryDetail{
		Status: apiclient.PluginDetailFound, Origin: apiclient.PluginDetailOriginReadme, Markdown: "# 标题",
	}, true, "")
	view := newTestPage().document(env, filledRequest())
	if view.state != documentFound || view.markdown != "# 标题" {
		t.Fatalf("found 没把正文带出来：%+v", view)
	}
}

// TestDocumentPassesOfficialParameters 钉住交给壳的四个参数就是官方那四个位置参数。
func TestDocumentPassesOfficialParameters(t *testing.T) {
	var seen [4]string
	env := &pluginapi.Env{Act: pluginapi.Actions{
		MarketEntryDetail: func(url, name, version, language string) (apiclient.PluginMarketEntryDetail, bool, apiclient.PluginMarketDetailFailure) {
			seen = [4]string{url, name, version, language}
			return apiclient.PluginMarketEntryDetail{Status: apiclient.PluginDetailNone}, true, ""
		},
	}}
	page := newTestPage()
	page.SetLanguage("zh-CN")
	page.document(env, filledRequest())
	want := [4]string{"https://m.example/index.json", "demo", "1.0.0", "zh-CN"}
	if seen != want {
		t.Fatalf("详情读参数 = %v，想要 %v", seen, want)
	}
}

// TestDocumentFailureIsRetryableOnlyWhenShellCanRetry 钉住"画不画重试按钮"的判据：
// 壳没接重读时 retryable=false，界面就不该画一枚点了没反应的按钮。
func TestDocumentFailureIsRetryableOnlyWhenShellCanRetry(t *testing.T) {
	page := newTestPage()
	view := page.document(envWithDetail(apiclient.PluginMarketEntryDetail{}, true, apiclient.PluginMarketDetailNetwork), filledRequest())
	if view.state != documentFailed || view.failure != apiclient.PluginMarketDetailNetwork || view.retryable {
		t.Fatalf("失败态解错了：%+v", view)
	}
	var seen [4]string
	if view := page.document(envWithRetry(apiclient.PluginMarketEntryDetail{}, true, apiclient.PluginMarketDetailDenied, &seen), filledRequest()); !view.retryable {
		t.Fatalf("壳接了重读时应当可重试：%+v", view)
	}
}

// TestFailureTextUsesOfficialCopy 逐档钉住官方 zh 文案（locales.ts:87-92）。
func TestFailureTextUsesOfficialCopy(t *testing.T) {
	cases := map[apiclient.PluginMarketDetailFailure]string{
		apiclient.PluginMarketDetailTooLarge:       zhDetailTooLarge,
		apiclient.PluginMarketDetailNetwork:        zhDetailNetwork,
		apiclient.PluginMarketDetailDenied:         zhDetailDenied,
		apiclient.PluginMarketDetailGitUnavailable: zhDetailGitUnavailable,
		apiclient.PluginMarketDetailInvalid:        zhDetailInvalid,
		apiclient.PluginMarketDetailOther:          zhDetailFailed,
	}
	for failure, want := range cases {
		if got := failureText(failure); got != want || got == "" {
			t.Fatalf("%s 的文案 = %q，想要 %q", failure, got, want)
		}
	}
}

// TestDocumentRequestCarriesVersion 钉住详情读的身份是 (url, name, version)。
func TestDocumentRequestCarriesVersion(t *testing.T) {
	page := NewPage(nil)
	page.Open(Entry{Name: "demo"}, "https://m.example/index.json")
	req := page.documentRequest(Version{Version: "2.1.0"}, true)
	want := ReadRequest{URL: "https://m.example/index.json", Name: "demo", Version: "2.1.0"}
	if req != want {
		t.Fatalf("读的身份 = %+v，想要 %+v", req, want)
	}
	// 一条版本都没有时不给 version（宿主缺参数会回 400；宁可在这里就不发）。
	if req := page.documentRequest(Version{}, false); req.Version != "" {
		t.Fatalf("没有版本时不该带 version：%+v", req)
	}
}
