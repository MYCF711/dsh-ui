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
	config := app.WebViewConfig{
		URL: options.URL,
		// `NewWindow` 为 true 表示"交给系统浏览器打开"，不会在窗口里再开页
		// （OFF:viva@v0.6.4/app/web.go:111-114）。官方 iframe 沙箱允许 popup；
		// 本仓没有应用内 popup 这一层，交回系统浏览器是同一意图的落点。
		NewWindow: func(target string) bool { return true },
		// 官方沙箱没有 `allow-downloads`，所以这里一律拒。
		Download: func(target string) bool { return false },
	}
	if options.OnLoad != nil {
		config.OnLoad = func(load app.WebLoad) {
			options.OnLoad(pluginapi.WebLoad{URL: load.URL, Phase: webPhase(load.Phase)})
		}
	}
	page, err := w.host.Open(config)
	if err != nil {
		if errors.Is(err, app.ErrUnsupported) {
			return nil, pluginapi.ErrWebUnsupported
		}
		return nil, err
	}
	return &webPage{page: page}, nil
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
