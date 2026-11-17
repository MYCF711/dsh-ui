// Package dsbrowser 是右栏的 **HTTP(S) 浏览器 tab**。
//
// 官方 `@deepseek-ai/dsh-client-ui-sidebar-browser`
// （*Sandboxed Web browser tabs for the right Sidebar*）。
//
// # 上一批判 HIDE，判错了
//
// 上一批给 #50 的结论是「本仓**没有 iframe / Electron webview 这一层**——
// Go 即时模式界面里嵌不了网页」，据此 HIDE。考古结论是**错的**：
//
//	Viva v0.6.4 自带原生网页宿主
//	  OFF:viva@v0.6.4/app/web.go:40          `type WebHost`（零值可用）
//	  OFF:viva@v0.6.4/app/web.go:205         `Host.Open(WebViewConfig) (*WebView, error)`
//	  OFF:viva@v0.6.4/app/web.go:193         `type WebView`（Load/Reload/Back/Forward/Focus/Close）
//	  OFF:viva@v0.6.4/app/app.go:225         `Config.Web *WebHost`
//	  OFF:viva@v0.6.4/app/run_canvas.go:164  RunCanvas 把它接到 painter
//	  OFF:viva@v0.6.4/gfx/native_view.go:32  `Canvas.NativeView(id, bounds)`
//	  OFF:viva@v0.6.4/ui/native_view.go:14   `Box.NativeView(id)`
//	  OFF:viva@v0.6.4/docs/web-views.md:3-5  「WebKitGTK on Linux, WKWebView on macOS,
//	                                          WebView2 on Windows」，引擎由系统提供、不打包
//
// 宿主本机是否真有引擎也查了：`C:\Program Files (x86)\Microsoft\EdgeWebView\
// Application\` 下有 156.0.4314.58 / 157.0.4341.42 / 157.0.4341.46 三份 WebView2
// Runtime —— 不是"理论能力"。
//
// 所以 #50 的形状是：**载具这一层官方本来就有对等物**，缺的只是"把插件画的地方
// 变成一块真网页"的那条线，本轮把它接上了。
//
// # 结构：官方四层，本仓四层
//
//	官方                                           本仓
//	src/client/index.ts:56-105  注册/派发           本文件 Panels()/register.go
//	src/client/definition.tsx   类型 + 向导          右栏 tab 类型（keyed 座位，key = "browser"）
//	src/client/browser/url.ts   地址白名单           plugins/dsbrowser/url.go
//	src/client/browser/BrowserNavigation.ts  历史     plugins/dsbrowser/navigation.go
//	src/client/browser/BrowserFrame.ts       载具面   pluginapi.WebPage / WebHost
//	src/client/view/BrowserBody.tsx          工具条   本文件 drawToolbar / drawBody
//	src/client/view/Browser.module.css       几何     本文件的常量表
//
// # 数据源 → 容器 → 渲染
//
//	数据源  用户在地址栏里敲的字符串 + 页面自己报回来的 OnLoad 事件。
//	        **零宿主方法**：官方这个包 `inject` 里没有任何 remote / host 调用
//	        （`src/client/index.ts:45` 只有 slots/locale/sidebarRight/sidebarRightTabs）。
//	容器    右栏 tab 类型 "browser"：体挂 `sidebar.right.pane.tab`（keyed）、
//	        标题挂 `sidebar.right.pane.tab.title`（同 key）——
//	        官方 `src/client/index.ts:81-90` 与 `:99-102` 注册的就是这两处。
//	渲染    Draw 画工具条与占位/失败行并记下内容区矩形；
//	        AfterDraw 用 `canvas.NativeView(page.ID(), rect)` 把真网页摆进去
//	        （官方对应 `BrowserPresentation.mount(viewportId)` 把 iframe/webview
//	         挂到 `.viewport` 那块 DOM 上，`src/client/view/IframePresentation.ts:53-72`）。
package dsbrowser

import (
	"errors"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var (
	_ pluginapi.Plugin = (*Plugin)(nil)
	_ pluginapi.Panels = (*Plugin)(nil)
	_ pluginapi.Fields = (*Plugin)(nil)
)

// RightTabBrowser 是本包注册的右栏 tab **类型** key（官方 `BROWSER_KIND='browser'`，
// `OFF:dsh-client-ui-sidebar-browser/src/client/definition.tsx:8`）。
//
// 本仓只用这一个 key（= 官方的 `kind`）：官方的 `id`（包名）与 `kind` 分开是为了让
// extension 顶掉同名 builtin，那条 `priority` 覆盖机制本仓还没建（见
// docs/official-view-tabs-evidence-rc2.md 的 G2），所以现在没有第二种实现来抢它。
const RightTabBrowser = "browser"

// ── 官方 Browser.module.css 的几何常量（逐条对锚）────────────────────────
//
//	.toolbar{height:38px;padding:5px 6px;gap:4px;border-bottom:.5px border-l3}
//	.tool{width:28px;height:28px;border-radius:radius-sm}
//	.address{height:28px;padding:0 34px 0 9px;background:bg-layer-1;
//	         border:.5px border-l2;border-radius:radius-sm}
//	.failure{padding:6px 12px;font:xxxs-11}
//	.start{padding:24px;font:xs-13}   .limit{padding:4px 10px;font:xxxs-11}
const (
	toolbarHeight   float32 = 38
	toolbarPadY     float32 = 5
	toolbarPadX     float32 = 6
	toolbarGap      float32 = 4
	toolSize        float32 = 28
	addressHeight   float32 = 28
	addressPadLeft  float32 = 9
	addressPadRight float32 = 34
	statusRowPadY   float32 = 6
	statusRowPadX   float32 = 12
	limitRowPadY    float32 = 4
	limitRowPadX    float32 = 10
	startPad        float32 = 24
	hairline        float32 = 0.5
	goButtonSize    float32 = 28
)

// Plugin 是右栏浏览器。
type Plugin struct {
	theme *pluginapi.Theme

	// address 是地址栏。**必须登记给壳**（`pluginapi.Fields`），否则 viva 不会把
	// 它当输入焦点候选，真机上打不进字（dscomposer 踩过同一个坑）。
	address ui.Field

	// nav 是应用侧历史（官方 BrowserNavigation）。
	nav browserNavigation

	// page 是已打开的网页；nil = 还没开（官方 `BrowserController.start` 只在
	// 有 initialUrl 时才导航，保存的地址**不会**自己开页）。
	page pluginapi.WebPage
	// openErr 记住开页失败的原因（ErrWebUnsupported = 这台机器放不了网页）。
	openErr error
	// failure 是地址被拒的原因（官方 `BrowserControllerState.addressFailure`）。
	failure addressFailure

	// content 是本帧内容区的盒子；AfterDraw 用它的矩形摆 native view。
	content ui.Box
	// drawn 表示本帧真的画了 tab 体——**不画就不能摆**：native view 一旦声明
	// 就会出现，右栏收起 / 切到别的 tab 时继续声明等于把网页糊在别的界面上。
	drawn bool

	// instances 按 (session, tabId) 保存独立 occurrence；上面的 nav/page 等字段
	// 只是当前激活实例的工作副本。地址框指针保持不动，切换时交换各自草稿。
	instances      map[string]*browserInstance
	currentKey     string
	loadedSessions map[string]bool
	dirtySessions  map[string]bool
}

func (*Plugin) ID() string { return "ds-browser" }

func (*Plugin) Provides() []string { return nil }

func (*Plugin) Requires() []string { return []string{pluginapi.KeyTheme} }

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.nav = newBrowserNavigation()
	p.instances = map[string]*browserInstance{}
	p.loadedSessions = map[string]bool{}
	p.dirtySessions = map[string]bool{}
	ctx.Effect(p.dispose)
	return nil
}

// Fields 把地址栏登记给壳（viva 只把登记过的 Field 当输入焦点候选）。
func (p *Plugin) Fields() []*ui.Field { return []*ui.Field{&p.address} }

// Panels 注册右栏 tab 的**体**与**标题**（官方两阶段注册的第二阶段）。
//
// 官方第一阶段是"类型进 `ctx.sidebarRightTabs`"；本仓的类型名录由壳从 keyed 槽的
// 占用方投影出来（`internal/uikernel/view_tabs.go:97` 的 `RightTabs`），所以这里只
// 注册体，类型（key + 芯片文案 + 顺序）由 Painter 的 Key/Label/Order 承载。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{
			ID: "browser-tab", Slot: pluginapi.SlotRightPaneTab, Key: RightTabBrowser,
			// 芯片文案 = 官方 `type.label`：中文「浏览器」（locales.ts:4）。
			Label: browserTypeLabel,
			// BrowserTitle.tsx:3,16 的地球；Browser.module.css:246-249 额外右 margin 4px。
			IconImage: iconGlobe(gfx.Color{}), IconMarginRight: browserTitleIconMargin,
			// 官方 tab 类型没有 `order`（`SidebarRightTabDefinition` 里没这个字段，
			// tab-registry.ts:106-…），名录顺序 = 注册顺序；Tab.Order 的零值就是
			// "按装配顺序"，所以这里留 0，不编一个数。
			Order: 0, Multiple: true, Draw: p.drawBody, AfterDraw: p.afterDraw,
		},
		{
			ID: "browser-tab-title", Slot: pluginapi.SlotRightPaneTabTitle, Key: RightTabBrowser,
			Order: 0, Draw: p.drawTitle,
		},
	}
}

// browserTypeLabel 是官方 `sidebarBrowser.type.label`（中文，locales.ts:4）。
const browserTypeLabel = "浏览器"

const (
	// Browser.module.css:246-249 的 .titleIcon{margin-right:4px}。
	browserTitleIconMargin float32 = 4
	// dockkit .tabTitle{gap:5px;line-height:1.4} 再叠加上面的 4px 右 margin。
	browserTitleGap        = 5 + browserTitleIconMargin
	browserTitleLineHeight = 13 * 1.4
)

// 官方 `sidebarBrowser` 词表里本插件用得到的几条（locales.ts:1-30，中文逐字）。
const (
	copyAddressPlaceholder = "输入 HTTP(S) 地址"
	copyStart              = "输入 HTTP(S) 地址开始浏览"
	copyLoading            = "正在打开…"
	copyBack               = "后退"
	copyForward            = "前进"
	copyReload             = "刷新"
	copyGo                 = "前往"
	copyExternal           = "在系统浏览器中打开"
	copyRestorePrevious    = "上次打开"
	copyRestoreAction      = "恢复页面"
	copyLoadFailed         = "页面加载失败；请刷新重试或在系统浏览器中打开。"
	copyAddressUnknown     = "页面已跳转；当前载体无法读取新的 URL。"
)

// addressFailureCopy 是官方 `sidebarBrowser.error.*`（locales.ts:17-21，中文逐字）。
func addressFailureCopy(reason addressFailure) string {
	switch reason {
	case failEmpty:
		return "请输入地址。"
	case failInvalid:
		return "这个地址无效或过长。"
	case failProtocol:
		return "只支持 HTTP 和 HTTPS 地址；本地文件请使用文档预览。"
	case failCredentials:
		return "地址不能包含用户名或密码。"
	case failApplicationOrigin:
		return "不能在嵌入浏览器中打开 DSH 应用自身。"
	}
	return ""
}

// ── 绘制 ────────────────────────────────────────────────────────────────

// drawBody 画工具条 + 状态行 + 内容区（官方 BrowserBody.tsx 的渲染分支）。
func (p *Plugin) drawBody(f ui.C, env *pluginapi.Env) {
	p.activate(env)
	p.drawn = false
	p.content = ui.Box{}
	t := p.theme
	ui.Column(f).FillWidth().FillHeight().Background(t.BgBase).Children(func() {
		p.drawToolbar(f, env)
		// `.toolbar` 的 `border-bottom:.5px solid border-l3`：viva 没有单边描边，
		// 用一条 0.5px 的实色行表达同一条线。
		ui.Column(f).FillWidth().Height(hairline).Shrink(0).Background(t.BorderL3)
		// `.failure`：地址被拒 / 开页失败（官方两处都画这一行的样式）。
		if copy := addressFailureCopy(p.failure); copy != "" {
			p.drawStatusRow(f, copy)
		} else if p.openErr != nil {
			if errors.Is(p.openErr, pluginapi.ErrWebUnsupported) {
				p.drawStatusRow(f, copyLoadFailed)
			} else {
				p.drawStatusRow(f, copyLoadFailed)
			}
		}
		// `.content` / `.viewport`：这块矩形就是网页要填的洞。
		p.content = ui.Column(f).FillWidth().Grow(1).Shrink(1).Clip()
		p.content.Children(func() { p.drawContentState(f, env) })
		// `.limit`（Browser.module.css）：unknown 态在底部那行如实说明，
		// 文案逐字取自官方 locales.ts:30。
		if p.unknown() {
			p.drawLimitRow(f)
		}
	})
	p.drawn = true
}

// drawToolbar 画官方 `.toolbar`（38px：后退 / 前进 / 刷新 + 地址框 + 外部打开）。
func (p *Plugin) drawToolbar(f ui.C, env *pluginapi.Env) {
	t := p.theme
	ui.Row(f).FillWidth().Height(toolbarHeight).
		PadSides(toolbarPadY, toolbarPadX, toolbarPadY, toolbarPadX).
		Items(ui.Center).Gap(toolbarGap).Children(func() {
		p.drawToolButton(f, env, "browser-back", copyBack, iconChevronLeft(t.LabelSecondary), iconSizeChevron, t.LabelSecondary,
			p.nav.canGoBack(), func() { p.goBack(env) })
		p.drawToolButton(f, env, "browser-forward", copyForward, iconChevronRight(t.LabelSecondary), iconSizeChevron, t.LabelSecondary,
			p.nav.canGoForward(), func() { p.goForward(env) })
		_, hasTarget := currentTarget(p.nav.value)
		p.drawToolButton(f, env, "browser-reload", copyReload, iconRefresh(t.LabelSecondary), iconSizeTool, t.LabelSecondary,
			hasTarget, func() { p.reload(env) })

		// `.addressBox`
		ui.Row(f).Grow(1).Height(addressHeight).Shrink(1).Children(func() {
			field := ui.TextFieldID(f, env.Tree.ID(env.Feat, "browser-address"), &p.address,
				copyAddressPlaceholder, copyAddressPlaceholder).
				FillWidth().Height(addressHeight).FontSize(t.Fonts.Caption).
				Background(t.BgLayer1).Border(hairline, t.BorderL2).Radius(t.Radii.SM).
				PadSides(0, addressPadRight, 0, addressPadLeft)
			if field.Submitted() {
				p.submit(env)
			}
			// `.addressGo`（官方绝对定位在输入框右内边距里，`.address` 的
			// `padding-right:34px` 就是给它留的位）。官方只在 `:focus-within`
			// 时让它可见；本仓没有"字段是否聚焦"的公开判据，改为**常显**。
			// 这一处差异登记在 docs/official-parity.md 的 #50 行。
			goBox := ui.Row(f).Width(goButtonSize).Height(addressHeight).Shrink(0).
				Absolute().Top(0).Right(0).
				Items(ui.Center).Justify(ui.Center).Radius(t.Radii.SM).
				Hover(ui.Look{}.Background(t.Hover)).
				Hit(env.Tree.ID(env.Feat, "browser-go"), copyGo).Children(func() {
				ui.Icon(f, iconLink(t.LabelSecondary)).Size(iconSizeTool, iconSizeTool).Color(t.LabelSecondary)
			})
			if goBox.Clicked() {
				p.submit(env)
			}
		})

		_, hasExternal := currentTarget(p.nav.value)
		p.drawToolButton(f, env, "browser-external", copyExternal, iconRightUp(t.LabelSecondary), iconSizeExternal, t.LabelSecondary,
			hasExternal && env.Act.OpenExternal != nil, func() {
				target, ok := currentTarget(p.nav.value)
				if !ok || env.Act.OpenExternal == nil {
					return
				}
				_ = env.Act.OpenExternal(target.URL)
			})
	})
}

// drawToolButton 画官方 `.tool`（28×28 圆角；禁用时不变色也不吃点击）并报告是否被点。
func (p *Plugin) drawToolButton(f ui.C, env *pluginapi.Env, name, label string, icon gfx.Icon, iconSize float32,
	color ui.Color, enabled bool, run func()) {
	t := p.theme
	if !enabled {
		color = t.LabelDimmed
	}
	box := ui.Row(f).Width(toolSize).Height(toolSize).Shrink(0).
		Items(ui.Center).Justify(ui.Center).Radius(t.Radii.SM)
	if enabled {
		box = box.Hover(ui.Look{}.Background(t.Hover)).Hit(env.Tree.ID(env.Feat, name), label)
	}
	box.Children(func() {
		ui.Icon(f, icon).Size(iconSize, iconSize).Color(color)
	})
	if enabled && box.Clicked() {
		run()
	}
}

// drawStatusRow 画官方 `.failure` 一行（6px/12px、xxxs-11）。
func (p *Plugin) drawStatusRow(f ui.C, copy string) {
	t := p.theme
	ui.Row(f).FillWidth().PadSides(statusRowPadY, statusRowPadX, statusRowPadY, statusRowPadX).
		Children(func() {
			ui.Paragraph(f, copy, 2).FontSize(t.Fonts.Tiny).LineHeight(t.Fonts.CaptionLine).
				Color(t.Error).FillWidth()
		})
}

// unknown 表示页面已经自己跳到别的地址、当前载具读不回新地址
// （官方 `frame.address === 'unknown'`，BrowserBody.tsx:95）。
func (p *Plugin) unknown() bool { return p.nav.value.Navigation.State == navUnknown }

// drawLimitRow 画官方 `.limit` 一行（4px/10px、xxxs-11、上边一条 0.5px 线）。
func (p *Plugin) drawLimitRow(f ui.C) {
	t := p.theme
	ui.Column(f).FillWidth().Shrink(0).Height(hairline).Background(t.BorderL3)
	ui.Row(f).FillWidth().Shrink(0).PadSides(limitRowPadY, limitRowPadX, limitRowPadY, limitRowPadX).
		Children(func() {
			ui.Text(f, copyAddressUnknown).FontSize(t.Fonts.Tiny).LineHeight(t.Fonts.CaptionLine).
				Color(t.LabelTertiary).Ellipsis().FillWidth()
		})
}

// drawContentState 画内容区里"还没有网页"时的两态（官方 BrowserBody 的
// `.start` / `.loading` 分支）。网页开好之后这块是空的——真网页由 AfterDraw 的
// native view 填，Viva 在同一层或更低层画的东西会被它遮住。
func (p *Plugin) drawContentState(f ui.C, env *pluginapi.Env) {
	t := p.theme
	if p.page != nil && p.openErr == nil {
		return
	}
	// 官方 .restore：checkpoint 仍在、但本进程尚未请求页面时，显示保存的
	// title/url 和显式「恢复页面」按钮；不能把它当成普通 start 自动开页。
	if p.page == nil {
		if target, ok := currentTarget(p.nav.value); ok {
			ui.Column(f).FillWidth().FillHeight().Pad(startPad).Gap(10).Items(ui.Center).Children(func() {
				ui.Row(f).Grow(1).Items(ui.Center).Justify(ui.Center).Children(func() {
					ui.Text(f, copyRestorePrevious).FontSize(t.Fonts.Caption).LineHeight(t.Fonts.CaptionLine).
						Color(t.LabelTertiary)
				})
				ui.Paragraph(f, target.Title, 3).FontSize(t.Fonts.Secondary).LineHeight(t.Fonts.SecondaryLine).
					Weight(gfx.WeightMedium).TextCenter().FillWidth()
				ui.Paragraph(f, target.URL, 4).FontSize(t.Fonts.Caption).LineHeight(t.Fonts.CaptionLine).
					Color(t.LabelSecondary).TextCenter().FillWidth()
				button := ui.Button(f, copyRestoreAction).Small().Background(t.ButtonPrimaryFill).
					Color(t.ButtonPrimaryFg)
				if button.Clicked() {
					p.restore(env)
				}
				ui.Row(f).Grow(1)
			})
			return
		}
	}
	copy := copyStart
	if p.nav.value.Navigation.State == navLoading {
		copy = copyLoading
	}
	ui.Row(f).FillWidth().FillHeight().Pad(startPad).Items(ui.Center).Justify(ui.Center).
		Children(func() {
			ui.Paragraph(f, copy, 2).FontSize(t.Fonts.Secondary).LineHeight(t.Fonts.SecondaryLine).
				Color(t.LabelTertiary).TextCenter().FillWidth()
		})
}

// drawTitle 画 tab 标题：官方 `BrowserTitle` = 地球图标 + 当前主机名，
// 没有目标时退回 tab 类型名（`entry?.title ?? tab.title`，BrowserTitle.tsx:15-17）。
func (p *Plugin) drawTitle(f ui.C, env *pluginapi.Env) {
	p.activate(env)
	t := p.theme
	label := browserTypeLabel
	if target, ok := currentTarget(p.nav.value); ok {
		label = target.Title
	}
	ui.Row(f).Items(ui.Center).Gap(browserTitleGap).Children(func() {
		ui.Icon(f, iconGlobe(t.LabelSecondary)).Size(iconSizeTitle, iconSizeTitle).Color(t.LabelSecondary)
		ui.Text(f, label).FontSize(t.Fonts.Secondary).LineHeight(browserTitleLineHeight).
			Color(t.LabelPrimary).Ellipsis()
	})
}

// ── 交互 ────────────────────────────────────────────────────────────────

// submit 处理地址栏回车 / 「前往」：**先校验再导航**（官方 `BrowserController.loadUrl`）。
func (p *Plugin) submit(env *pluginapi.Env) {
	p.activate(env)
	parsed := parseBrowserAddress(p.address.Text(), "")
	if !parsed.OK {
		p.failure = parsed.Reason
		return
	}
	p.failure = ""
	p.address.SetText(parsed.Target.URL)
	if current, ok := currentTarget(p.nav.value); ok && sameTarget(current, parsed.Target) {
		if p.nav.reload() == nil {
			return
		}
	} else if p.nav.navigate(parsed.Target) == nil {
		return
	}
	p.openPage(env, parsed.Target)
}

// goBack / goForward / reload 都走同一条"应用历史 → 载具"的路。
func (p *Plugin) goBack(env *pluginapi.Env) {
	p.activate(env)
	request := p.nav.back()
	if request == nil {
		return
	}
	p.address.SetText(request.Target.URL)
	if p.page == nil {
		p.openPage(env, request.Target)
		return
	}
	p.page.Load(request.Target.URL)
	p.persistActive(env)
}

func (p *Plugin) goForward(env *pluginapi.Env) {
	p.activate(env)
	request := p.nav.forward()
	if request == nil {
		return
	}
	p.address.SetText(request.Target.URL)
	if p.page == nil {
		p.openPage(env, request.Target)
		return
	}
	p.page.Load(request.Target.URL)
	p.persistActive(env)
}

func (p *Plugin) reload(env *pluginapi.Env) {
	p.activate(env)
	target, ok := currentTarget(p.nav.value)
	if !ok {
		return
	}
	if p.page == nil {
		p.restore(env)
		return
	}
	p.nav.reload()
	p.address.SetText(target.URL)
	p.page.Reload()
	p.persistActive(env)
}

// restore 只响应用户显式动作；保存的 checkpoint 本身绝不创建网页。
func (p *Plugin) restore(env *pluginapi.Env) {
	p.activate(env)
	target, ok := currentTarget(p.nav.value)
	if !ok {
		return
	}
	if p.nav.reload() == nil {
		return
	}
	p.failure = ""
	p.address.SetText(target.URL)
	p.openPage(env, target)
}

// openPage 在需要时打开一个真网页，然后导航过去。
//
// 载具只认**校验过的**地址（官方 `BrowserFrame.loadUrl(target)` 收的就是
// `BrowserTarget`，不是裸字符串）。
func (p *Plugin) openPage(env *pluginapi.Env, target browserTarget) {
	if env.Web == nil {
		// 这个部署没有窗口级网页宿主（无头驱动 / 非桌面）。
		// **不假装加载成功**：记下原因，界面画官方那条失败行。
		p.openErr = pluginapi.ErrWebUnsupported
		p.persistActive(env)
		return
	}
	if p.page == nil {
		loadEnv := *env // 固定本次 occurrence；壳下一帧会复用并改写原 Env。
		page, err := env.Web.Open(pluginapi.WebOpenOptions{OnLoad: func(load pluginapi.WebLoad) {
			p.handleLoad(&loadEnv, load)
		}})
		if err != nil {
			p.openErr = err
			p.persistActive(env)
			return
		}
		p.openErr = nil
		p.page = page
	}
	p.page.Load(target.URL)
	p.persistActive(env)
}

// handleLoad 把载具的加载事件翻成 `nav.frameLoaded(revision)`。
//
// 官方 `IframeImpl.handleLoaded(revision)` 只认"当前这一代请求"的 load；
// 这里也一样：事件回来时取**当前**请求的 revision 交给状态机，由它按
// `navigation.revision !== revision` 自己丢弃过期的（navigation.go 的 frameLoaded）。
func (p *Plugin) handleLoad(env *pluginapi.Env, load pluginapi.WebLoad) {
	if load.Phase != pluginapi.WebLoadDone {
		return
	}
	sessionID, tabID := browserEnvIdentity(env)
	key := browserInstanceKey(sessionID, tabID)
	if key != p.currentKey {
		instance := p.instances[key]
		if instance == nil || instance.page == nil {
			return
		}
		if request := instance.nav.value.Request; request != nil {
			instance.nav.frameLoaded(request.Revision)
			p.saveActive()
			if err := p.persistSession(env, sessionID); err != nil && env.Act.Notify != nil {
				env.Act.Notify("保存浏览器状态失败: " + err.Error())
			}
		}
		return
	}
	if request := p.nav.value.Request; request != nil {
		p.nav.frameLoaded(request.Revision)
		p.persistActive(env)
	}
}

// afterDraw 把真网页摆进内容区。
//
// 这是"网页"与"Viva 画的东西"之间唯一的那条线：`canvas.NativeView` 声明一个洞，
// 平台把 WebView2 / WKWebView / WebKitGTK 的页面放进那个矩形
// （OFF:viva@v0.6.4/docs/web-views.md:3-5、gfx/native_view.go:1-16）。
func (p *Plugin) afterDraw(canvas *gfx.Canvas, env *pluginapi.Env) {
	page := p.page
	drawn := p.drawn
	box := p.content
	// 本帧没画 tab 体（右栏收起 / 切到别的 tab）→ 什么都不声明，网页自然隐藏。
	p.drawn = false
	if !drawn || page == nil || canvas == nil {
		return
	}
	rect := box.Rect()
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	canvas.NativeView(page.ID(), rect)
}
