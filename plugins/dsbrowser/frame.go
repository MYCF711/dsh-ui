// frame.go —— 引擎侧那三档：标题 / 历史可用性 / 加载中 / 失败详情。
//
// # 为什么和 navigation.go 分两份
//
// 官方把这些字段放在**同一个** `BrowserFrameState` 里
// （OFF:dsh-client-ui-sidebar-browser/src/client/browser/BrowserFrame.ts:12-21：
// `target` / `address` / `loading` / `canGoBack` / `canGoForward` / `error`），
// 但只有 `BrowserTabState` 进持久化（同包 BrowserPersistence.ts:17-24）——
// `canGoBack` 这种**引擎事实**恢复出来一定是假的（重启后引擎历史是空的）。
// 所以本仓按"能不能持久化"把它们分成两份：
//
//	navigation.go 的 browserNavigation   应用侧地址记录 + 观测档位，进 ui-config
//	本文件        browserFrame           引擎侧四个观测，只活在进程里
//
// 这与官方的 `BrowserController` 把 `frame` 与 `checkpoint` 分开持有是同一件事
// （BrowserController.ts:36-40）。
//
// # 逐条对锚（官方 ElectronWebViewImpl.ts）
//
//	:82    loadUrl      → loading=true、error=undefined（请求一发出就这么显示，不等引擎）
//	:126   navigate     → goBack / goForward / reload 同一步
//	:168-172 did-start-navigation（主框架）→ loading=true、error=undefined
//	:173-176 page-title-updated → 只在 address==='observed' 时重新观测
//	:178-183 did-fail-load → 主框架且 errorCode !== -3 才记 code/description
//	:184-186 render-process-gone / destroyed → dropGuest() + failed()（进程没了）
//	:220   标题 = `element.getTitle() || parsed.target.title`
//	:224   loading = `error === undefined && element.isLoading()`
//	:225-226 canGoBack / canGoForward = 引擎历史
//	:240-245 failed() → loading=false、记 error、历史可用性**保留上次观测值**
//
// Viva 这边三个引擎把导航归成同样的三档（OFF:viva@0.6.8-webstate/app/web.go:177-188）
// 并给出 `WebView.Title/CanGoBack/CanGoForward/IsLoading`（同文件 :344-355），
// 于是上面这一组里，Viva 有对等物的是这五条：
//
//	Title ← WebView.Title() / WebLoad.Title
//	canGoBack / canGoForward ← WebView.CanGoBack() / WebView.CanGoForward()
//	loading ← WebView.IsLoading()（三档 + 请求时乐观置真）
//	error ← WebLoad.Code / WebLoad.Description
//	进程没了 ← WebViewConfig.OnProcessFailed + WebView.IsCrashed/ProcessFailure
//	          （OFF:viva@v0.6.9-procfail/app/web.go:108-115,399-409）
//
// # 进程没了**不等于**加载失败（本仓有意比官方多走一步）
//
// 官方把两条事件并到同一条 `failed()`：`render-process-gone`/`destroyed` 先
// `dropGuest()` 再 `failed()`（ElectronWebViewImpl.ts:184-186），于是用户看到的
// 就是 `.failure` 那一行的 `load.failed`——"页面加载失败；请刷新重试或在系统浏览器
// 中打开。"（BrowserBody.tsx:112-114、locales.ts:26）。**官方是 X：崩溃与加载失败
// 同一句话、同一档状态。**
//
// **我们 Y：分成两档。** `crashed` 与 `failed` 是两个布尔、两套文案（见 copy()）：
// 进程没了说"进程没了/可能丢输入/刷新能重开"，加载失败说官方的
// `load.failed(.detail)`。**好在哪**：用户看到的差别是真实的——加载失败是网络或
// 服务端问题，刷新/换系统浏览器都可能白费；进程崩了是页面里的东西跟着没了，
// 值得提醒"没保存的输入可能丢了"（Viva 为这件事专门给了原因码，原话见
// internal/platform/web_process_failure.go:5-9）。两条事件的**动作面**也不一样：
// 崩溃后引擎历史已经没了，而加载失败后历史还在。
//
// 不改官方形状的部分照抄：崩溃后 `canGoBack/canGoForward` 落 false（官方
// `dropGuest()` 之后 `ready=false`，`failed()` 里 `canGoBack: this.ready && …`
// 两个都成 false，:240-245,252-256）、地址栏保留目标地址、刷新按钮仍在（官方
// `reload()` 在 `!ready || error` 时重新造 guest 再加载，:99-109）。
package dsbrowser

import (
	"strings"

	"github.com/MYCF711/dsh-ui/pluginapi"
)

// loadFailure 是官方 `BrowserLoadError`（OFF:…/browser/BrowserFrame.ts:6-9）。
//
// 两个字段在官方都可缺：`did-fail-load` 两条都有，`render-process-gone` 两条都没有
// （ElectronWebViewImpl.ts:181 vs :185）。这套"缺"的语义直接决定文案选哪条，
// 见下面的 copy()。
type loadFailure struct {
	Code        pluginapi.WebLoadError
	Description string
}

// browserFrame 是官方 `BrowserFrameState` 里"来处是引擎"的那四档
// （OFF:…/browser/BrowserFrame.ts:12-21 的 loading / canGoBack / canGoForward / error）。
//
// `target` 与 `address` 不在这里：它们要持久化，留在 browserNavigation。
type browserFrame struct {
	loading      bool
	canGoBack    bool
	canGoForward bool
	// failed / failure 是官方那两档的 error：failed 单独一个布尔，因为官方
	// `error !== undefined` 这个判据**不能**用 Code 的零值代替——`WebLoadNoError`
	// 就是"加载成功"，而"失败但没有 code"是另一回事。
	failed  bool
	failure loadFailure
	// crashed / crash 是进程没了这一档：官方把它塞进 `error`（`failed()` 不带 code，
	// ElectronWebViewImpl.ts:184-186,240-245），本仓**有意分开**——原因见文件头
	// 「进程没了**不等于**加载失败」。两个布尔互斥地描述两种"页面画不出来"。
	crashed bool
	crash   pluginapi.WebProcessFailure
}

// begin 是官方每次"请求一次导航"时的那一步：立刻进 loading、清掉上一次的错误，
// 不等引擎回话（ElectronWebViewImpl.ts:82 的 loadUrl、:126 的 navigate、:105 的 reload）。
//
// 清崩溃态也在这一步：Viva 说 `ProcessFailure` 一直留着，"until the page starts a
// navigation again, which is what clears it"（OFF:viva@0.6.9-procfail/app/web.go:399-402），
// 而官方 reload 在 `!ready || error` 时立刻清 error、进 loading（:99-109）。本仓的
// 每一次导航请求（地址栏 / 前进后退 / 刷新 / 恢复）都经过 begin，所以崩溃提示只会在
// "用户还没有重新请求页面"的这段时间里挂着。
func (f *browserFrame) begin() {
	f.loading, f.failed, f.failure = true, false, loadFailure{}
	f.crashed, f.crash = false, pluginapi.WebProcessNoFailure
}

// observe 是官方 `observeReady` 的后半（ElectronWebViewImpl.ts:224-226）：
// 三个值都**向引擎要**，本仓不自己维持——
//
//	loading      = error === undefined && element.isLoading()
//	canGoBack    = element.canGoBack()
//	canGoForward = element.canGoForward()
//
// 页面还没开（page == nil）时回到零值：没有引擎就没有任何"可用的历史"。
//
// 进程没了的时候三个都保持零：官方崩溃后 `dropGuest()` 把 `ready` 置假，`failed()`
// 里写的是 `canGoBack: this.ready && current.canGoBack`，两个都成 false
// （:240-245,252-256）；本仓照这个结果，不让一个已经死掉的进程继续报"能后退"。
func (f *browserFrame) observe(page pluginapi.WebPage) {
	if page == nil {
		*f = browserFrame{}
		return
	}
	if f.crashed {
		f.loading, f.canGoBack, f.canGoForward = false, false, false
		return
	}
	f.loading = !f.failed && page.IsLoading()
	f.canGoBack = page.CanGoBack()
	f.canGoForward = page.CanGoForward()
}

// fail 是官方 `failed()`（ElectronWebViewImpl.ts:240-245）：loading 结束、记下引擎给的原因，
// 而 `canGoBack/canGoForward` **保留上一次观测到的值**（那边写的是
// `canGoBack: this.ready && current.canGoBack`），不是清零。
func (f *browserFrame) fail(cause loadFailure) {
	f.failed, f.failure, f.loading = true, cause, false
}

// processFailed 是官方 `render-process-gone` / `destroyed` 那一步
// （ElectronWebViewImpl.ts:184-186）的**分档版**：
//
//	官方  dropGuest() + failed()  → 与加载失败同一档、同一句话
//	我们  crashed 单独一档       → 文案不同、还可以说要重开（见 copy()）
//
// 状态上必须与 `fail` 划清：`failed=false`（这样 `copy()` 不会画 `load.failed`、
// `drawContentState` 的占位判据也不会把崩溃当成加载失败），`failure` 清空，
// `loading` 收掉（引擎自己那一步也是 `loading=false`，
// OFF:viva@0.6.9-procfail/internal/platform/windows/web_windows.go:424）。
func (f *browserFrame) processFailed(failure pluginapi.WebProcessFailure) {
	f.crashed, f.crash = true, failure
	f.failed, f.failure = false, loadFailure{}
	f.loading, f.canGoBack, f.canGoForward = false, false, false
}

// apply 把一次载具事件落进引擎侧状态。
//
// 三条分支与官方三个事件一一对应；进程没了是第四条（`processFailed`），
// 由 `handleProcessFailed` 直接调用，不折进这里——它不是一次导航事件。
func (f *browserFrame) apply(page pluginapi.WebPage, load pluginapi.WebLoad) {
	switch load.Phase {
	case pluginapi.WebLoadStart:
		// did-start-navigation（主框架）：只做 loading=true、清 error
		// （ElectronWebViewImpl.ts:168-172）。历史可用性不动。
		//
		// 崩溃态也在这里清：Viva 的引擎面就是"重新开始一次导航就把
		// ProcessFailure 清掉"（OFF:viva@0.6.9-procfail/app/web.go:399-402、
		// internal/platform/windows/web_windows.go:492-494）。
		f.loading, f.failed, f.failure = true, false, loadFailure{}
		f.crashed, f.crash = false, pluginapi.WebProcessNoFailure
	case pluginapi.WebLoadCommitted:
		// did-navigate → observe（:164）。这一步不是"加载完成"：loading 仍由
		// IsLoading() 说了算，所以 Committed 之后 loading 一般还是 true。
		//
		// 这里顺手清 error，比官方多做半步：官方靠 did-start-navigation 清（:170），
		// 而本仓的载具在"只报后两档"的情况下可能先来 Committed（navigation.go 的
		// commit 就是为这一档写的）。既然文档已经被引擎收下，上一次导航的错误行
		// 再挂在屏幕上就是假的——这不是"猜", 是把官方两个事件里的清错误动作合到
		// 这一档上，顺序上仍然只在"新文档已被收下"之后发生。
		//
		// **崩溃态不在这里清**：晚到的 Committed 不是"页面活过来了"的证据，
		// 清掉就会把一条真实的崩溃提示抹成一块空白（官方那边 guest 已被 drop、
		// 监听器随 signal 一起失效，也不会有这个事件）。
		f.failed, f.failure = false, loadFailure{}
		f.observe(page)
	case pluginapi.WebLoadDone:
		if cause, ok := failedLoad(load); ok {
			f.fail(cause)
			return
		}
		// did-stop-loading → observe（:173-176 那一组）：失败的那一档已经在上面的
		// 分支里处理掉了，走到这里就是"这次导航结束了、没报错"。
		f.failed, f.failure = false, loadFailure{}
		f.observe(page)
	}
}

// failedLoad 判断一条 Done 事件是不是**真的要算失败**。
//
// 官方 `did-fail-load`（ElectronWebViewImpl.ts:178-183）有两条前置判据：
//
//	主框架       —— 子框架失败不算这次导航失败（Viva 的 WebLoad 只报主框架，无需再判）
//	errorCode !== -3 —— Chromium 的 ERR_ABORTED：被新导航打断、或策略取消加载，
//	                   不是"页面加载失败"，官方整条跳过
//
// 本仓的 `-3` 对等物是 `WebLoadCancelled`（"被停掉的导航"，Viva 自己就这么写：
// OFF:viva@0.6.8-webstate/internal/platform/web_load_error.go:16-18），所以同样跳过。
func failedLoad(load pluginapi.WebLoad) (loadFailure, bool) {
	if load.Phase != pluginapi.WebLoadDone || load.Code == pluginapi.WebLoadNoError {
		return loadFailure{}, false
	}
	if load.Code == pluginapi.WebLoadCancelled {
		return loadFailure{}, false
	}
	return loadFailure{Code: load.Code, Description: strings.TrimSpace(load.Description)}, true
}

// copy 是官方 `.failure` 那一行选哪句（OFF:…/BrowserBody.tsx:112-114）：
//
//	error.code !== undefined && error.description !== undefined
//	  → t('load.failed.detail', { code: String(error.code), description: error.description })
//	  → t('load.failed')
//
// 本仓的 `Code` 一旦失败就非零（Viva 的分类里不存在"失败但没分类"，未知也是
// `WebLoadUnknownError`），所以这条判据退化成"引擎有没有给 description"：
// 给了就画 detail，没给就画官方那句 `load.failed`。
//
// **这条退化是如实记着的差异**：Windows 的 WebView2 只给状态、不给文本
// （OFF:viva@0.6.8-webstate/internal/platform/web_windows.go:391 传的是空串），
// 所以 detail 在 Windows 上不显形；macOS/Linux 有 NSError/GError 原文时会显形
// （darwin/web_darwin.go:345、linux/gtk_web_linux.go:678）。不是契约缺，是引擎不给文本。
//
// 进程没了走**另一条**：官方把 `render-process-gone` 也塞进这条（`failed()` 不带
// code，于是落到 `load.failed`），本仓不塞——见 processFailureCopy。
func (f browserFrame) copy() string {
	if f.crashed {
		return processFailureCopy(f.crash)
	}
	if !f.failed {
		return ""
	}
	if f.failure.Description == "" {
		return copyLoadFailed
	}
	return loadFailureDetail(f.failure)
}

// 进程没了这一档的文案。**官方没有这几句**：官方并进 `load.failed`
// （ElectronWebViewImpl.ts:184-186 → BrowserBody.tsx:112-114 → locales.ts:26），
// 用户看到的与网络失败一模一样。
//
// 五档各说各的原因，因为 Viva 给原因码的整个理由就是"要不要提醒丢输入"取决于原因
// （OFF:viva@0.6.9-procfail/internal/platform/web_process_failure.go:5-9）：
// 崩了可能丢了没保存的东西，被系统在内存压力下回收的页面什么都没做错，
// 根本没起来的页面一次都没跑过。句式沿用官方 `load.failed` 的两段结构：
// 先说发生了什么，再给动作（刷新 / 系统浏览器）——用户不用学新交互。
const (
	copyProcessCrashed      = "页面进程已崩溃，页面里未保存的输入可能已丢失；刷新可重新打开。"
	copyProcessKilled       = "页面进程已被系统结束（例如内存不足）；刷新可重新打开。"
	copyProcessUnresponsive = "页面无响应，进程已被结束；刷新可重新打开。"
	copyProcessLaunchFailed = "页面进程没能启动；刷新重试或在系统浏览器中打开。"
	copyProcessUnknown      = "页面进程已退出（原因未知）；刷新可重新打开。"
)

// processFailureCopy 给原因码挑一句话。认不出的原因也必须说话（"已退出，原因未知"），
// 只有零值 `WebProcessNoFailure` 是"没有这一档"，返回空串——那是"进程还在"，
// 不该被画成一次崩溃。
func processFailureCopy(failure pluginapi.WebProcessFailure) string {
	switch failure {
	case pluginapi.WebProcessNoFailure:
		return ""
	case pluginapi.WebProcessCrashed:
		return copyProcessCrashed
	case pluginapi.WebProcessKilled:
		return copyProcessKilled
	case pluginapi.WebProcessUnresponsive:
		return copyProcessUnresponsive
	case pluginapi.WebProcessLaunchFailed:
		return copyProcessLaunchFailed
	}
	return copyProcessUnknown
}

// loadFailureDetail 渲染官方 `load.failed.detail`（locales.ts:27，中文逐字：
// `页面加载失败 ({code}): {description}`）。
//
// `{code}` 官方插的是 Chromium 的数字错误码（负数），Viva 没有数字码、
// 只有归一化的分类（`WebLoadError.String()`），所以这里插分类名——
// **换的是值不是形状**：模板、两个占位符、括号与冒号都逐字官方。
func loadFailureDetail(failure loadFailure) string {
	rendered := strings.ReplaceAll(copyLoadFailedDetail, "{code}", failure.Code.String())
	return strings.ReplaceAll(rendered, "{description}", failure.Description)
}
