// web_pages.go —— 窗口里的**网页宿主**（官方右栏浏览器那一层的载具面）。
//
// # 为什么在壳里
//
// `app.WebHost` 是**窗口级**的：它必须在 `app.Config.Web` 里交给窗口，窗口建好
// 才会回调 `OnReady`（OFF:viva@v0.6.4/app/web.go:40-57、:158-166 的 `bind()`）。
// 也就是说网页宿主只能由壳创建，插件拿不到窗口配置（pluginapi 的红线②）。
//
// 所以分工是：
//
//	壳   建 `app.WebHost` → 填 `Config.Web` → 包成 pluginapi.WebHost 放进 Env.Web
//	插件 只认 `Env.Web`，`Open` 一个页面，拿 `page.ID()` 交给 canvas 摆位
//
// 这条线与官方同形：官方的桌面载具也是**主进程**批一个 `<webview>`
// （`src/client/electron/ElectronWebViewImpl.ts` 拿的是 `carrier.browser`，
//
//	由 `src/types.ts:12-30` 的 `DesktopBrowserBridge` 定义），web 载具才在页面里
//
// 自己造 iframe。
package main

import (
	"errors"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// webPages 是 pluginapi.WebHost 的实现，包住 Viva 的窗口级 WebHost。
type webPages struct{ host *app.WebHost }

var _ pluginapi.WebHost = webPages{}

// Open 打开一个页面。
//
// 策略照官方 iframe 的固定沙箱（OFF:dsh-client-ui-sidebar-browser/src/client/
// view/IframePresentation.ts:96-99）：
//
//	WEB_BROWSER_SANDBOX = 'allow-scripts allow-forms allow-same-origin
//	                       allow-popups allow-popups-to-escape-sandbox'
//
// 里面对应的三条：
//
//	allow-popups  → NewWindow 交回系统浏览器（本仓没有"应用内新窗口"这层；
//	                官方 `External` 按钮做的就是这件事，`BrowserBody.tsx:69-72`）
//	无 allow-downloads → Download 一律拒绝（页面的下载不会落盘）
//	无 allow-top-navigation → 见下面的 Navigate
//
// **不设 `Ephemeral`**：官方 web 载具的 iframe 与页面同源、共用存储；桌面载具才
// 按 workspace 分存储分区（`src/client/electron/workspace.ts`）。本仓是原生窗口、
// 没有 workspace 分区这一层，所以用引擎默认 profile，并在
// docs/official-parity.md 的 #50 行登记这条差异。
func (w webPages) Open(options pluginapi.WebOpenOptions) (pluginapi.WebPage, error) {
	if w.host == nil {
		return nil, pluginapi.ErrWebUnsupported
	}
	page, err := w.host.Open(webViewConfig(options))
	if err != nil {
		if errors.Is(err, app.ErrUnsupported) {
			return nil, pluginapi.ErrWebUnsupported
		}
		return nil, err
	}
	return &webPage{page: page}, nil
}

// webViewConfig 把契约选项翻成 Viva 的窗口配置，是**唯一**产生配置的地方。
//
// 为什么单拎出来：两个回调（导航 / 进程崩溃）就是"事件接没接上"的全部，而 `Open`
// 要一个真 `app.WebHost` 才能跑；把配置放进纯函数，壳这一层的接线就能在不需要
// 引擎的单测里钉住（cmd/app/web_pages_test.go 的进程崩溃那三条）。
func webViewConfig(options pluginapi.WebOpenOptions) app.WebViewConfig {
	config := app.WebViewConfig{
		URL: options.URL,
		// 插件接了 `OnNewWindow`（官方 `BrowserController.openRequested`）时，那个地址
		// 由插件决定去向：解析通过就开成**应用内右栏新标签**，所以这里必须回 false
		// ——viva 的语义是「false = 什么都不开、true = 交系统浏览器」
		// （OFF:viva@v0.6.4/app/web.go:110-113、internal/platform/windows/web_windows.go:407-418）。
		// 插件没接 / 明确不要（false）时保持旧行为：交系统浏览器。
		NewWindow: func(target string) bool {
			if options.OnNewWindow != nil && options.OnNewWindow(target) {
				return false
			}
			return true
		},
		// 官方沙箱没有 `allow-downloads`，所以这里一律拒。
		Download: func(target string) bool { return false },
	}
	if options.OnLoad != nil {
		config.OnLoad = func(load app.WebLoad) {
			// 三个新字段逐条转出：`Title` 是这一步引擎持有的文档标题，`Code`/`Description`
			// 是 `did-fail-load` 的 code/description（官方 ElectronWebViewImpl.ts:173-183）。
			// **不在这里做取舍**：官方那三条判据（只在 observed 时采纳标题、-3 不算失败、
			// 两个字段都在才画 detail）都在插件里，载具只报事实。
			options.OnLoad(pluginapi.WebLoad{
				URL:         load.URL,
				Phase:       webPhase(load.Phase),
				Title:       load.Title,
				Code:        webLoadError(load.Code),
				Description: load.Description,
			})
		}
	}
	if options.OnProcessFailed != nil {
		config.OnProcessFailed = func(failure app.WebProcessFailure) {
			// 进程没了是**另一件事**，不折进 OnLoad：官方也是两条独立事件
			// （`did-fail-load` 与 `render-process-gone`，ElectronWebViewImpl.ts:178-186），
			// 载具不许替插件决定"这两个看起来一样"。
			options.OnProcessFailed(webProcessFailure(failure))
		}
	}
	return config
}

// webPage 是 pluginapi.WebPage 的实现。
type webPage struct{ page *app.WebView }

var _ pluginapi.WebPage = (*webPage)(nil)

func (p *webPage) ID() gfx.NativeViewID { return p.page.ID() }
func (p *webPage) Load(url string)      { p.page.Load(url) }
func (p *webPage) Reload()              { p.page.Reload() }
func (p *webPage) Back()                { p.page.Back() }
func (p *webPage) Forward()             { p.page.Forward() }
func (p *webPage) Focus()               { p.page.Focus() }
func (p *webPage) Close()               { p.page.Close() }

// 四个查询直接落到 Viva 的同名方法（OFF:viva@0.6.8-webstate/app/web.go:344-355），
// 一个字节的判断都不在这里做：官方那三处判据在插件里。
func (p *webPage) Title() string      { return p.page.Title() }
func (p *webPage) CanGoBack() bool    { return p.page.CanGoBack() }
func (p *webPage) CanGoForward() bool { return p.page.CanGoForward() }
func (p *webPage) IsLoading() bool    { return p.page.IsLoading() }

// 崩溃观测同理，只翻译：`IsCrashed` 与 `ProcessFailure` 都是 Viva 的即时查询
// （OFF:viva@0.6.9-procfail/app/web.go:399-409），插件在画那一帧时问一次。
func (p *webPage) IsCrashed() bool { return p.page.IsCrashed() }
func (p *webPage) ProcessFailure() pluginapi.WebProcessFailure {
	return webProcessFailure(p.page.ProcessFailure())
}

// webPhase 把 Viva 的相位翻成契约相位。
func webPhase(phase app.WebLoadPhase) pluginapi.WebLoadPhase {
	switch phase {
	case app.WebLoadStarted:
		return pluginapi.WebLoadStart
	case app.WebLoadCommitted:
		return pluginapi.WebLoadCommitted
	}
	return pluginapi.WebLoadDone
}

// webLoadError 把 Viva 的失败分类翻成契约分类。
//
// 逐条列举而不是 `pluginapi.WebLoadError(code)` 强转：两份枚举的**顺序**现在一致，
// 但契约不许依赖上游的数值（上游插一个值就会静默错位），所以这里显式对齐每一个名字，
// 认不出的一律 `WebLoadUnknownError`——那正是 Viva 自己对未知状态码的处理
// （OFF:viva@0.6.8-webstate/internal/platform/web_load_error.go:119-122）。
// `cmd/app/web_pages_test.go` 用一张全量表钉住两边取值逐一相等。
func webLoadError(code app.WebLoadError) pluginapi.WebLoadError {
	switch code {
	case app.WebLoadNoError:
		return pluginapi.WebLoadNoError
	case app.WebLoadUnknownError:
		return pluginapi.WebLoadUnknownError
	case app.WebLoadCancelled:
		return pluginapi.WebLoadCancelled
	case app.WebLoadTimeout:
		return pluginapi.WebLoadTimeout
	case app.WebLoadHostNotFound:
		return pluginapi.WebLoadHostNotFound
	case app.WebLoadServerUnreachable:
		return pluginapi.WebLoadServerUnreachable
	case app.WebLoadConnectionLost:
		return pluginapi.WebLoadConnectionLost
	case app.WebLoadBadServerResponse:
		return pluginapi.WebLoadBadServerResponse
	case app.WebLoadBadCertificate:
		return pluginapi.WebLoadBadCertificate
	case app.WebLoadRedirectFailed:
		return pluginapi.WebLoadRedirectFailed
	case app.WebLoadAuthenticationRequired:
		return pluginapi.WebLoadAuthenticationRequired
	}
	return pluginapi.WebLoadUnknownError
}

// webProcessFailure 把 Viva 的进程失败原因翻成契约原因。
//
// 与 webLoadError 同一套规矩：逐条列举、不强转（两份枚举现在同序，但契约不许依赖
// 上游数值）、认不出的归 `WebProcessUnknown`——那正是 Viva 自己对没分类原因的处理
// （OFF:viva@v0.6.9-procfail/internal/platform/web_process_failure.go:98-117）。
// 零值必须落到 `WebProcessNoFailure`：那是"进程还在"，不能因为翻译漏一档就把
// 一次正常页面变成一个崩溃提示。
func webProcessFailure(failure app.WebProcessFailure) pluginapi.WebProcessFailure {
	switch failure {
	case app.WebProcessNoFailure:
		return pluginapi.WebProcessNoFailure
	case app.WebProcessCrashed:
		return pluginapi.WebProcessCrashed
	case app.WebProcessKilled:
		return pluginapi.WebProcessKilled
	case app.WebProcessUnresponsive:
		return pluginapi.WebProcessUnresponsive
	case app.WebProcessLaunchFailed:
		return pluginapi.WebProcessLaunchFailed
	case app.WebProcessUnknown:
		return pluginapi.WebProcessUnknown
	}
	return pluginapi.WebProcessUnknown
}
