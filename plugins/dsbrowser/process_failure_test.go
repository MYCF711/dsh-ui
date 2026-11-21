// process_failure_test.go —— 账本 #50 里那条「引擎进程崩事件」的解封判据。
//
// 2026-11-21 之前这一条在 docs/official-parity.md 的 #50 行写着「Viva 缺，不许猜」。
// `viva v0.6.9-procfail` 补上了 `WebViewConfig.OnProcessFailed`、
// `WebView.IsCrashed()`、`WebView.ProcessFailure()`（OFF:viva@v0.6.9-procfail/app/web.go:108-115,399-409，
// 原因码在 internal/platform/web_process_failure.go:11-75），于是官方那条
// `render-process-gone` / `destroyed` 有了对等物：
//
//	官方  electron/ElectronWebViewImpl.ts:184-186  element.addEventListener('render-process-gone'|'destroyed',
//	                                               () => { dropGuest(); failed() })
//	落点  plugins/dsbrowser/frame.go 的 processFailed + plugin.go 的 handleProcessFailed
//
// 本文件钉的是**用户看得见的那一层**：进程没了与加载失败是两句话、两档状态、
// 两种动作面。官方把两者并成一句 `load.failed`（BrowserBody.tsx:112-114、locales.ts:26），
// 本仓有意分开（理由见 frame.go 文件头「进程没了**不等于**加载失败」）。
package dsbrowser

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
)

// crashCallback 是某个 occurrence 的 `OnProcessFailed`（开页时交进去的那个）。
func crashCallback(t *testing.T, host *fakeHost, index int) func(pluginapi.WebProcessFailure) {
	t.Helper()
	if len(host.onProcessFailures) <= index || host.onProcessFailures[index] == nil {
		t.Fatalf("第 %d 次开页没有交 OnProcessFailed：%v", index, host.onProcessFailures)
	}
	return host.onProcessFailures[index]
}

// ── 主判据：崩溃与加载失败在用户可见层面不同 ─────────────────────────────

// TestProcessFailureAndLoadFailureAreDifferentOnScreen 是本轮的主判据：
//
//	加载失败（did-fail-load） → 官方 `load.failed` / `load.failed.detail`
//	进程没了（render-process-gone / destroyed） → 本仓自己的崩溃文案，且**不是**失败态
//
// 两页同样的地址、同样的引擎，只有事件不同；画出来的那一行必须是两句不同的话，
// 而且各自只出现自己那句（官方那两条判据是二选一，混着画会让用户以为出了两件事）。
func TestProcessFailureAndLoadFailureAreDifferentOnScreen(t *testing.T) {
	// ① 加载失败：没有 description → 官方那句 `load.failed`（locales.ts:26）。
	failed, _, failedEnv, _, failedLoad := openBrowser(t, "https://missing.invalid/")
	failedLoad(pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://missing.invalid/",
		Code: pluginapi.WebLoadHostNotFound})
	if got := failed.frame.copy(); got != copyLoadFailed {
		t.Fatalf("加载失败文案 = %q，应当是官方 load.failed %q", got, copyLoadFailed)
	}

	// ② 进程没了：同一页、同一地址，只是引擎报的事件换成崩溃。
	crashed, crashedHost, crashedEnv, _, crashedLoad := openBrowser(t, "https://example.com/start")
	crashedLoad(pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://example.com/start"})
	crashCallback(t, crashedHost, 0)(pluginapi.WebProcessCrashed)
	if got := crashed.frame.copy(); got != copyProcessCrashed {
		t.Fatalf("崩溃文案 = %q，应当是 %q", got, copyProcessCrashed)
	}
	if got := crashed.frame.copy(); got == copyLoadFailed {
		t.Fatal("崩溃被画成了官方的 load.failed：用户会去查网络，而页面里的输入已经丢了")
	}

	// ③ 真画一遍：两句各自上屏，且互不串台。
	loadCanvas := renderBrowser(t, failed, failedEnv)
	if !containsText(loadCanvas, copyLoadFailed) {
		t.Fatalf("加载失败那一行没上屏：%v", loadCanvas.Texts())
	}
	if containsText(loadCanvas, copyProcessCrashed) {
		t.Fatal("加载失败画出了崩溃文案")
	}
	crashCanvas := renderBrowser(t, crashed, crashedEnv)
	if !containsText(crashCanvas, copyProcessCrashed) {
		t.Fatalf("崩溃那一行没上屏：%v", crashCanvas.Texts())
	}
	if containsText(crashCanvas, copyLoadFailed) {
		t.Fatal("崩溃画出了加载失败文案：两件事必须让用户分得开")
	}
}

// TestCrashIsNotRecordedAsAFailedLoad 钉状态机的那一半：官方把崩溃塞进 `failed()`
// （ElectronWebViewImpl.ts:184-186,240-245），本仓**不许**照抄这一塞。
// `failed` 是"这次导航失败"；崩溃是"页面进程没了"，两档各有各的字段。
func TestCrashIsNotRecordedAsAFailedLoad(t *testing.T) {
	p, host, _, _, _ := openBrowser(t, "https://example.com/start")
	crashCallback(t, host, 0)(pluginapi.WebProcessKilled)

	if !p.frame.crashed || p.frame.crash != pluginapi.WebProcessKilled {
		t.Fatalf("崩溃没记下来：crashed=%v crash=%v", p.frame.crashed, p.frame.crash)
	}
	if p.frame.failed {
		t.Fatal("崩溃被记成了加载失败：failed=true 会画 load.failed，用户看不出进程没了")
	}
	if p.frame.failure.Code != pluginapi.WebLoadNoError || p.frame.failure.Description != "" {
		t.Fatalf("崩溃往失败原因里写了东西：%+v", p.frame.failure)
	}
	if p.frame.loading {
		t.Fatal("崩溃之后仍是 loading：进程没了不会再完成")
	}
}

// TestEachProcessFailureReasonSaysSomethingDifferent 是原因码的用处：
// Viva 给五档是为了让应用说对人话（OFF:viva@0.6.9-procfail/internal/platform/
// web_process_failure.go:5-9）。塌成一句话就等于没用上这个 API。
func TestEachProcessFailureReasonSaysSomethingDifferent(t *testing.T) {
	reasons := []pluginapi.WebProcessFailure{
		pluginapi.WebProcessCrashed,
		pluginapi.WebProcessKilled,
		pluginapi.WebProcessUnresponsive,
		pluginapi.WebProcessLaunchFailed,
		pluginapi.WebProcessUnknown,
	}
	seen := map[string]pluginapi.WebProcessFailure{}
	for _, reason := range reasons {
		copy := processFailureCopy(reason)
		if copy == "" {
			t.Fatalf("%v 没有文案", reason)
		}
		if copy == copyLoadFailed || strings.HasPrefix(copy, "页面加载失败") {
			t.Fatalf("%v 的文案与加载失败混在一起：%q", reason, copy)
		}
		if other, ok := seen[copy]; ok {
			t.Fatalf("%v 与 %v 说了同一句话：%q", reason, other, copy)
		}
		seen[copy] = reason
	}
	// 崩了才提醒丢输入；被系统在内存压力下回收的页面什么都没做错，不许吓用户。
	if !strings.Contains(copyProcessCrashed, "输入") {
		t.Fatalf("崩溃文案没提可能丢输入：%q", copyProcessCrashed)
	}
	if strings.Contains(copyProcessKilled, "输入") {
		t.Fatalf("被系统结束的页面被说成丢了输入：%q", copyProcessKilled)
	}
	// 零值是"进程还在"，不是崩溃——不许给它编一句话。
	if got := processFailureCopy(pluginapi.WebProcessNoFailure); got != "" {
		t.Fatalf("零值原因画出了 %q", got)
	}
}

// TestALateLoadDoneDoesNotEraseTheCrashNotice 是崩溃态的清除判据：
// 只有**新一次导航开始**才清（Viva：`until the page starts a navigation again,
// which is what clears it`，OFF:viva@0.6.9-procfail/app/web.go:399-402）。
// 一条晚到的 Done 不是"页面活过来了"的证据，清掉就等于把真崩溃抹成一块空白。
func TestALateLoadDoneDoesNotEraseTheCrashNotice(t *testing.T) {
	p, host, _, _, load := openBrowser(t, "https://example.com/start")
	page := p.page.(*fakePage)
	page.canBack, page.canForward = true, true
	crashCallback(t, host, 0)(pluginapi.WebProcessCrashed)
	// 晚到的 Done 会让 state machine 再向引擎对一次表（frame.observe）：
	// 死掉的进程照样可能答"有历史"，界面不许跟着把两个按钮点亮。
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://example.com/start"})
	if !p.frame.crashed {
		t.Fatal("晚到的 Done 把崩溃提示抹掉了")
	}
	if got := p.frame.copy(); got != copyProcessCrashed {
		t.Fatalf("崩溃文案 = %q", got)
	}
	if p.frame.canGoBack || p.frame.canGoForward {
		t.Fatalf("晚到的 Done 把死进程的历史又点亮了：back=%v forward=%v",
			p.frame.canGoBack, p.frame.canGoForward)
	}

	// 真正重新导航（Start）才清：与 Viva 的清零点一致。
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadStart, URL: "https://example.com/start"})
	if p.frame.crashed || p.frame.crash != pluginapi.WebProcessNoFailure {
		t.Fatalf("新导航开始后崩溃态还在：%+v", p.frame)
	}
	if got := p.frame.copy(); got != "" {
		t.Fatalf("新导航开始后还挂着 %q", got)
	}
}

// ── 崩溃之后的界面：地址栏 / 按钮 / native view / 重载入口 ────────────────

// TestCrashDisablesEngineHistoryAndKeepsTheReloadEntry 对官方那两个动作面：
//
//	官方 dropGuest() 之后 ready=false → failed() 里 canGoBack/canGoForward 都是 false
//	  （ElectronWebViewImpl.ts:240-245,252-256）→ 两个按钮 disabled（BrowserBody.tsx:82-83）
//	官方 reload() 在 `!ready || error` 时重新造 guest 再加载（:99-109）→ 刷新是重开入口
//
// 本仓照抄结果：历史两个都不亮、刷新仍可点、地址栏保留目标地址（官方 failed() 不碰 target）。
func TestCrashDisablesEngineHistoryAndKeepsTheReloadEntry(t *testing.T) {
	p, host, env, _, load := openBrowser(t, "https://example.com/start")
	page := p.page.(*fakePage)
	// 崩溃前引擎说"两个方向都有历史"，屏幕上因此两个按钮都亮。
	page.canBack, page.canForward = true, true
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://example.com/start"})
	if !toolbarHit(t, p, env, "browser-back") || !toolbarHit(t, p, env, "browser-forward") {
		t.Fatal("前置：崩溃前历史按钮应当亮着")
	}

	crashCallback(t, host, 0)(pluginapi.WebProcessCrashed)
	// 假引擎的事实没变（进程死了它照样可能答 true）：界面**不许**再问它。
	if p.frame.canGoBack || p.frame.canGoForward {
		t.Fatalf("崩溃后还拿引擎的历史当真：back=%v forward=%v", p.frame.canGoBack, p.frame.canGoForward)
	}
	if toolbarHit(t, p, env, "browser-back") || toolbarHit(t, p, env, "browser-forward") {
		t.Fatal("崩溃后后退/前进还亮着：那个引擎历史已经没了")
	}
	if !toolbarHit(t, p, env, "browser-reload") {
		t.Fatal("崩溃后没有重载入口：官方 reload() 就是重新打开页面的那条路")
	}
	if got := p.address.Text(); got != "https://example.com/start" {
		t.Fatalf("崩溃后地址栏 = %q，应当保留目标地址（官方 failed() 不动 target）", got)
	}
}

// TestCrashStopsPlacingTheDeadPagesView 是 native view 的那一半：
// 官方 `dropGuest()` 把 guest 元素从 DOM 摘掉（ElectronWebViewImpl.ts:184-186,252-256）；
// 本仓对应"这一帧不声明 native view"，否则死掉引擎的空白视图会盖住 Viva 画的东西，
// 而且用户会以为页面还在。
func TestCrashStopsPlacingTheDeadPagesView(t *testing.T) {
	p, host, env, _, _ := openBrowser(t, "https://example.com/start")
	live := renderBrowser(t, p, env)
	p.afterDraw(live, env)
	if got := len(live.NativeViews()); got != 1 {
		t.Fatalf("前置：正常页面应当摆 1 个 native view，得到 %d", got)
	}

	crashCallback(t, host, 0)(pluginapi.WebProcessCrashed)
	dead := renderBrowser(t, p, env)
	p.afterDraw(dead, env)
	if got := len(dead.NativeViews()); got != 0 {
		t.Fatalf("崩溃后还摆了 %d 个 native view：死页面会盖住提示", got)
	}
}

// TestReloadBringsACrashedPageBack 是重载入口的行为判据：
// 官方 reload() 清 error、进 loading、重新造 guest 加载 pending target（:99-109）；
// 本仓 begin() 清崩溃态后调 `WebPage.Reload()`——Viva 写明"a page that lost its process
// … cannot draw again until it is loaded again, which WebView.Reload does"
// （OFF:viva@0.6.9-procfail/app/web.go:405-409）。
func TestReloadBringsACrashedPageBack(t *testing.T) {
	p, host, env, _, _ := openBrowser(t, "https://example.com/start")
	page := p.page.(*fakePage)
	crashCallback(t, host, 0)(pluginapi.WebProcessCrashed)
	if got := p.frame.copy(); got != copyProcessCrashed {
		t.Fatalf("前置：崩溃文案 = %q", got)
	}

	p.reload(env)
	if !hasValue(page.loaded, "<reload>") {
		t.Fatalf("刷新没有真的重载页面：%v", page.loaded)
	}
	if p.frame.crashed || p.frame.crash != pluginapi.WebProcessNoFailure {
		t.Fatalf("刷新之后崩溃态还在：%+v", p.frame)
	}
	if got := p.frame.copy(); got != "" {
		t.Fatalf("刷新之后还挂着 %q", got)
	}
	if !p.frame.loading {
		t.Fatal("刷新之后没进 loading：官方 reload 立刻置 loading=true")
	}
	if got := p.address.Text(); got != "https://example.com/start" {
		t.Fatalf("刷新改掉了地址：%q", got)
	}

	// 重开之后 native view 又该摆上了（上一帧的"不摆"只对崩溃帧有效）。
	canvas := renderBrowser(t, p, env)
	p.afterDraw(canvas, env)
	if got := len(canvas.NativeViews()); got != 1 {
		t.Fatalf("刷新之后应当重新摆 1 个 native view，得到 %d", got)
	}
}

// TestCrashStaysInItsOwnOccurrence 钉 occurrence 隔离：每个 Open 交自己那份
// `OnProcessFailed`（与官方每个 guest 自己挂监听同形，ElectronWebViewImpl.ts:184-186），
// 旁路 tab 崩了不许把当前正在看的 tab 画成崩溃。
func TestCrashStaysInItsOwnOccurrence(t *testing.T) {
	cfg := map[string]string{}
	host := &fakeHost{}
	p := &Plugin{}
	p.theme = dscore.Dark()
	first := occurrenceEnv("s1", "tab1", host, cfg)
	second := occurrenceEnv("s1", "tab2", host, cfg)
	navigateOccurrence(p, first, "https://example.com/one")
	navigateOccurrence(p, second, "https://other.org/two")
	if host.opened != 2 {
		t.Fatalf("前置：两个 occurrence 应当各开一次页，得到 %d", host.opened)
	}

	// tab1 在后台崩溃；当前显示的是 tab2。崩溃前它的引擎说"两个方向都有历史"，
	// 切回来时不许用一个死进程的答案把按钮点亮。
	firstPage := p.instances[browserInstanceKey("s1", "tab1")].page.(*fakePage)
	firstPage.canBack, firstPage.canForward = true, true
	crashCallback(t, host, 0)(pluginapi.WebProcessCrashed)
	if p.frame.crashed {
		t.Fatal("tab1 的崩溃画到了 tab2 上")
	}
	if got := p.frame.copy(); got != "" {
		t.Fatalf("tab2 被 tab1 的崩溃带出了一行 %q", got)
	}

	// 切回 tab1：它自己的崩溃现场还在（切 occurrence 时对表不许把它 observe 掉）。
	p.activate(first)
	if !p.frame.crashed || p.frame.crash != pluginapi.WebProcessCrashed {
		t.Fatalf("切回 tab1 后崩溃现场丢了：%+v", p.frame)
	}
	if got := p.frame.copy(); got != copyProcessCrashed {
		t.Fatalf("切回 tab1 后文案 = %q", got)
	}
	if p.frame.canGoBack || p.frame.canGoForward {
		t.Fatalf("切回崩溃 tab 时历史按钮被引擎的假答案点亮了：back=%v forward=%v",
			p.frame.canGoBack, p.frame.canGoForward)
	}
	// tab2 自己的现场没被写脏。
	p.activate(second)
	if p.frame.crashed {
		t.Fatal("切回 tab2 时把它标成了崩溃")
	}
}

// hasValue 报告 values 里有没有 want。
func hasValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
