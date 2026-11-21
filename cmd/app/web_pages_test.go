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

// TestWebProcessFailureMappingCoversEveryVivaClassification 钉住进程崩溃这一档的翻译：
// 官方 render-process-gone / destroyed 的原因码要逐档对上（OFF:viva@v0.6.9-procfail/
// internal/platform/web_process_failure.go:11-48），一个不多、一个不少。
//
// 为什么值得单测：`webProcessFailure` 是 6 个 case 的显式 switch（不许强转，理由同
// webLoadError）。上游将来插一档时这里会先红；而漏掉某个 case 的后果比加载失败更糟——
// 零值落错档会把一个**正常页面**画成"进程已退出"。
func TestWebProcessFailureMappingCoversEveryVivaClassification(t *testing.T) {
	if len(vivaProcessFailures) != len(contractProcessFailures) {
		t.Fatalf("原因码个数不一致：Viva %d 个，契约 %d 个",
			len(vivaProcessFailures), len(contractProcessFailures))
	}
	for index, viva := range vivaProcessFailures {
		contract := webProcessFailure(viva)
		if contract.String() != viva.String() {
			t.Fatalf("第 %d 个原因：契约 %q vs Viva %q", index, contract, viva)
		}
		if contract != contractProcessFailures[index] {
			t.Fatalf("第 %d 个原因没有落到契约的同名常量：%v vs %v",
				index, contract, contractProcessFailures[index])
		}
	}
	// 零值必须是"进程还在"：翻译漏一档就把正常页面变成崩溃提示。
	if got := webProcessFailure(app.WebProcessNoFailure); got != pluginapi.WebProcessNoFailure {
		t.Fatalf("零值翻成了 %q，应当是 no failure", got)
	}
	// 认不出的值 → unknown，不静默当"没崩"（Viva 自己对没分类原因就是这个处理）。
	if got := webProcessFailure(app.WebProcessFailure(200)); got != pluginapi.WebProcessUnknown {
		t.Fatalf("未知原因翻成了 %q，应当是 unknown failure", got)
	}
}

// vivaProcessFailures / contractProcessFailures 是两边原因码的**全量**清单，
// 同时是"上游新增一档"的探针（上面的个数断言会先红）。
var vivaProcessFailures = []app.WebProcessFailure{
	app.WebProcessNoFailure,
	app.WebProcessCrashed,
	app.WebProcessKilled,
	app.WebProcessUnresponsive,
	app.WebProcessLaunchFailed,
	app.WebProcessUnknown,
}

var contractProcessFailures = []pluginapi.WebProcessFailure{
	pluginapi.WebProcessNoFailure,
	pluginapi.WebProcessCrashed,
	pluginapi.WebProcessKilled,
	pluginapi.WebProcessUnresponsive,
	pluginapi.WebProcessLaunchFailed,
	pluginapi.WebProcessUnknown,
}

// TestWebProcessFailureNamesMatchVivaWordForWord 是上一条的另一半：名字逐字同上游
// （OFF:viva@0.6.9-procfail/internal/platform/web_process_failure.go:34-48）。
func TestWebProcessFailureNamesMatchVivaWordForWord(t *testing.T) {
	for _, failure := range contractProcessFailures {
		want := app.WebProcessFailure(failure).String()
		if got := failure.String(); got != want {
			t.Fatalf("%d 的名字：契约 %q，Viva %q", failure, got, want)
		}
	}
	// 越界值的兜底文案也得同形（上游是 `web process failure N`）。
	if got, want := pluginapi.WebProcessFailure(200).String(), app.WebProcessFailure(200).String(); got != want {
		t.Fatalf("越界值的兜底文案：契约 %q，Viva %q", got, want)
	}
}

// TestWebViewConfigRoutesTheProcessFailureEvent 钉壳这一层的**接线**：
// 插件给的 `OnProcessFailed` 必须接到 Viva 的 `WebViewConfig.OnProcessFailed` 上，
// 原因码走翻译，而且**不许**折进 `OnLoad`（两件事，两个回调）。
//
// 为什么能在没有引擎的单测里验：配置由纯函数 `webViewConfig` 产生（`Open` 才需要真
// `app.WebHost`）。漏接这根线时，插件永远收不到崩溃事件，界面上就永远没有那一行。
func TestWebViewConfigRoutesTheProcessFailureEvent(t *testing.T) {
	var got []pluginapi.WebProcessFailure
	config := webViewConfig(pluginapi.WebOpenOptions{
		OnProcessFailed: func(failure pluginapi.WebProcessFailure) {
			got = append(got, failure)
		},
	})
	if config.OnProcessFailed == nil {
		t.Fatal("进程崩溃事件没有接到 Viva 的 WebViewConfig 上")
	}
	config.OnProcessFailed(app.WebProcessCrashed)
	config.OnProcessFailed(app.WebProcessKilled)
	if len(got) != 2 || got[0] != pluginapi.WebProcessCrashed || got[1] != pluginapi.WebProcessKilled {
		t.Fatalf("回调收到的原因 = %v，应当是 [crashed killed]", got)
	}
	// 不许折进 OnLoad：只接崩溃时 OnLoad 必须仍是 nil，否则插件会把崩溃当导航事件。
	if config.OnLoad != nil {
		t.Fatal("进程崩溃被折进了 OnLoad：两件事必须走两个回调")
	}
	// 插件没接就保持 nil：壳不替插件造一个"忽略崩溃"的回调。
	if plain := webViewConfig(pluginapi.WebOpenOptions{}); plain.OnProcessFailed != nil {
		t.Fatal("插件没要进程崩溃事件，壳却接了一个")
	}
}
