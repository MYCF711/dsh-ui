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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// webPages 是 pluginapi.WebHost 的实现，包住 Viva 的窗口级 WebHost。
type webPages struct {
	host *app.WebHost
	// partitions 是本窗口的存储分区名缓存（官方 `DesktopBrowserGuests.partitions`）。
	//
	// **必须跨帧活着**：`webPages` 是每帧新建的值，缓存不能挂在它上面，否则每帧都
	// 新造一批分区名（那就变成"每次随机"，同一个 workspace 的两个 tab 会分到两套
	// cookies）。指针由 model 持有（`main.go` 的 `m.webPartitions`）。
	partitions *webPartitionCache
}

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
// **不设 `Ephemeral`（账号为空时）**：那时的行为与这条线出现之前一致——用引擎默认
// profile。官方 web 载具的 iframe 与页面同源、共用存储；桌面载具才按 workspace 分
// 存储分区（`src/client/electron/workspace.ts`）。本仓是原生窗口，分区那条线走
// `webPartitionCache`（见下），没有账号时**一个字都不下发**。
func (w webPages) Open(options pluginapi.WebOpenOptions) (pluginapi.WebPage, error) {
	if w.host == nil {
		return nil, pluginapi.ErrWebUnsupported
	}
	page, err := w.host.Open(w.config(options))
	if err != nil {
		if errors.Is(err, app.ErrUnsupported) {
			return nil, pluginapi.ErrWebUnsupported
		}
		return nil, err
	}
	return &webPage{page: page}, nil
}

// config 是一页的窗口配置：翻译（`webViewConfig`）+ 分区（`webPartitionCache`）。
//
// 单拎出来是为了让"分区真的进了配置"这条接线能在没有引擎的单测里钉住——`Open`
// 必须先有 `app.WebHost` 才能跑，而配置本身是纯数据。
func (w webPages) config(options pluginapi.WebOpenOptions) app.WebViewConfig {
	config := webViewConfig(options)
	w.partitions.apply(&config, options.StorageAccount)
	return config
}

// ── 存储分区（官方 `lib/main.js` 的 DesktopBrowserGuests）─────────────────

// webPartitionPrefix 是分区名前缀（官方 `dsh-sidebar-browser-${randomUUID()}`，
// OFF:app.asar:lib/main.js:18109）。**21 + 36 = 57 字符**，在 WebView2 的 64
// 字符上限内（`viva@v0.6.11-part/app/web.go:306-309` 的 `webPartitionNameLength`），
// 字符集也全在它那张表里（字母数字 + `-`）。
const webPartitionPrefix = "dsh-sidebar-browser-"

// webPartitionCache 按**存储账号**缓存分区名，是官方
// `DesktopBrowserGuests.partitions`（`lib/main.js:18093` 的 `partitions = new Map()`）
// 的对等物。进程内活一次：分区名一旦发给某个账号就不再变，同账号后来的 tab 拿到的
// 是同一个名字——这正是"同一 workspace 的两个 tab 共用 cookies"的唯一实现方式。
//
// 锁是它自己的（不与 `m.mu` 混用）：网页事件在窗口线程、`Open` 也可能从别处来，
// `-race` 下必须自己守住。
type webPartitionCache struct {
	mu    sync.Mutex
	names map[string]string
	// newID 造一个分区名里的 UUID 文本；nil = `randomUUID`。
	//
	// 留这个口子只为**钉住那条安全分支**：铸不出名字时必须不下发分区，而不是让
	// 所有账号共用一个名字。crypto/rand 事实上不会失败，那条分支用真随机源测不到，
	// 而它一旦错了就是"看起来隔离、其实共用一套 cookies"。
	newID func() string
}

// mint 造一个新的 UUID 文本（测试可以替换 newID）。
func (c *webPartitionCache) mint() string {
	if c.newID != nil {
		return c.newID()
	}
	return randomUUID()
}

// apply 给一份窗口配置写上分区（名字 + `Ephemeral`）。
//
//   - 账号为空 → **什么都不写**。零值 `Partition`/`Ephemeral` 就是引擎默认 profile，
//     与这条线出现之前逐字节相同（这是硬边界：没有身份时绝不静默造一个全局共享分区，
//     也不许把页面塞进别人的分区）。
//   - 账号非空 → 取（或铸）这个名字，并置 `Ephemeral`。**官方浏览器侧是内存会话**
//     （`partition` 没有 `persist:` 前缀 = 进程内 session，
//     `lib/main.js:18109` 与 Electron session 语义），所以这里也必须 `Ephemeral`：
//     `Partition` 单给是**持久**的（`viva@v0.6.11-part/app/web.go:136-152`），
//     那会把官方"进程结束就没了"的内存会话变成写盘，多出官方没有的磁盘残留。
func (c *webPartitionCache) apply(config *app.WebViewConfig, account string) {
	if name := c.name(account); name != "" {
		config.Partition = name
		config.Ephemeral = true
	}
}

// name 返回账号的分区名（空账号 → 空串）。第一次见到的账号铸一个新名字并记住。
//
// 为什么名字里不含账号：名字会落进 WebView2 的 profile 元数据（`Ephemeral` 只是把
// cookies/缓存放内存，profile 的登记仍在 user-data 目录里，
// `viva@v0.6.11-part/internal/platform/windows/web_windows.go:226-229`），把
// `cwd:<工作区路径>` 拼进名字等于把本机路径写进磁盘。官方用随机 UUID 就是为了这个，
// 我们也用随机的——账号只当 Map 的键，不进名字。
//
// 为什么**不**像官方那样给账号加 4096 字符上限（`lib/main.js:18106` 抛
// "a workspace storage identity is required"）：那条校验是为跨 IPC 的入参把关，
// 而这里账号全程在进程内、又只当键用（名字是定长 UUID），加个上限只会让一个长
// 工作区路径的会话退回默认 profile——那是**更坏**的结果（与他人共用存储）。
func (c *webPartitionCache) name(account string) string {
	// nil 缓存或空账号 = 没有可用的分区依据 → 不下发。零值 `webPages{}`（测试夹具、
	// 老调用方）因此保持这条线出现之前的行为，不会因为少接一根线而 panic。
	if c == nil || account == "" {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if name, ok := c.names[account]; ok {
		return name
	}
	id := c.mint()
	if id == "" {
		// 铸不出名字就不下发（见 randomUUID 的注释）：**不缓存**，下一次还能成功。
		return ""
	}
	name := webPartitionPrefix + id
	if c.names == nil {
		c.names = map[string]string{}
	}
	c.names[account] = name
	return name
}

// randomUUID 造一个 v4 UUID 的小写文本（官方 `randomUUID()` 同形）。
//
// 为什么不用 `math/rand`：分区名是跨账号隔离的**边界**，两台机器/两次运行撞名不该
// 由"种子一样"决定。crypto/rand 失败时退回时间戳+计数器没有意义（撞名比报错更坏），
// 所以这里让名字退化成空，调用方就不下发分区——宁可退回默认 profile，也不给两个
// 账号同一个名字。
func randomUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	b[6] = b[6]&0x0f | 0x40 // version 4
	b[8] = b[8]&0x3f | 0x80 // variant 10xx
	text := hex.EncodeToString(b[:])
	return text[0:8] + "-" + text[8:12] + "-" + text[12:16] + "-" + text[16:20] + "-" + text[20:32]
}

// webViewConfig 把契约选项翻成 Viva 的窗口配置，是**唯一**翻译配置的地方
// （唯一产生最终配置的是它上面那层 `webPages.config`：翻译之后补存储分区）。
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
