package main

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/app"
)

// TestWebLoadErrorMappingCoversEveryVivaClassification 钉住"壳只翻译、不判断"的前提：
// 契约里的失败分类必须与 Viva 的一一对应，一个不多、一个不少。
//
// 为什么值得单测：`webLoadError` 是 11 个 case 的显式 switch（不许强转，见那里的注释），
// 上游将来插一个新分类时，这里会先红——而不会在插件里静默变成 `WebLoadNoError`
// 把一次加载失败画成成功。
func TestWebLoadErrorMappingCoversEveryVivaClassification(t *testing.T) {
	// ① 每一个 Viva 分类都要翻译成**同名的**契约分类（名字逐字相等是这道判据的核心：
	//    值相等但名字漂了，插件文案就会在没人发现的时候变味）。
	if len(vivaClassifications) != len(contractClassifications) {
		t.Fatalf("分类个数不一致：Viva %d 个，契约 %d 个", len(vivaClassifications), len(contractClassifications))
	}
	for index, viva := range vivaClassifications {
		contract := webLoadError(viva)
		if contract.String() != viva.String() {
			t.Fatalf("第 %d 个分类：契约 %q vs Viva %q", index, contract, viva)
		}
		if contract != contractClassifications[index] {
			t.Fatalf("第 %d 个分类没有落到契约的同名常量：%v vs %v", index, contract, contractClassifications[index])
		}
	}

	// ② 认不出的值 → 未知，不静默当成功（Viva 自己对未知状态码就是这个处理）。
	if got := webLoadError(app.WebLoadError(200)); got != pluginapi.WebLoadUnknownError {
		t.Fatalf("未知分类翻成了 %q，应当是 unknown error", got)
	}
}

// vivaClassifications / contractClassifications 是两边分类的**全量**清单。
// 它同时是"上游新增分类"的探针：上面的个数断言会先红。
var vivaClassifications = []app.WebLoadError{
	app.WebLoadNoError,
	app.WebLoadUnknownError,
	app.WebLoadCancelled,
	app.WebLoadTimeout,
	app.WebLoadHostNotFound,
	app.WebLoadServerUnreachable,
	app.WebLoadConnectionLost,
	app.WebLoadBadServerResponse,
	app.WebLoadBadCertificate,
	app.WebLoadRedirectFailed,
	app.WebLoadAuthenticationRequired,
}

var contractClassifications = []pluginapi.WebLoadError{
	pluginapi.WebLoadNoError,
	pluginapi.WebLoadUnknownError,
	pluginapi.WebLoadCancelled,
	pluginapi.WebLoadTimeout,
	pluginapi.WebLoadHostNotFound,
	pluginapi.WebLoadServerUnreachable,
	pluginapi.WebLoadConnectionLost,
	pluginapi.WebLoadBadServerResponse,
	pluginapi.WebLoadBadCertificate,
	pluginapi.WebLoadRedirectFailed,
	pluginapi.WebLoadAuthenticationRequired,
}

// TestWebLoadErrorNamesMatchVivaWordForWord 是上一条的另一半：名字逐字同上游
// （OFF:viva@0.6.8-webstate/internal/platform/web_load_error.go:43-75）。
func TestWebLoadErrorNamesMatchVivaWordForWord(t *testing.T) {
	for _, code := range contractClassifications {
		want := app.WebLoadError(code).String()
		if got := code.String(); got != want {
			t.Fatalf("%d 的名字：契约 %q，Viva %q", code, got, want)
		}
	}
	// 越界值的兜底文案也得同形（上游是 `web load error N`，逐个数字，不猜名字）。
	if got, want := pluginapi.WebLoadError(200).String(), app.WebLoadError(200).String(); got != want {
		t.Fatalf("越界值的兜底文案：契约 %q，Viva %q", got, want)
	}
}
