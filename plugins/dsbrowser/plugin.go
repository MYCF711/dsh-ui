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

	// `.failure{padding:6px 12px;color:state-error-primary;
	//   font:xxxs-11;background:color-mix(in srgb, state-error-primary 8%, transparent)}`
	// （Browser.module.css:218-224）：错误行的底色是错误色的 8% 叠色。
	failureTint float32 = 0.08
	// 官方 `--dsw-font-xxxs-11` 是 11px/14px（gradient-shadow-text.css:262），
	// `.failure` 与 `.limit` 两行都用它。
	lineHeightXXS float32 = 14
	// `.addressChanged{position:absolute;top:6px;right:8px;padding-left:8px;
	//   color:label-tertiary;font:xxxs-11;line-height:16px;background:bg-layer-2}`
	// （Browser.module.css:98-108）：地址栏右上角那枚「URL 已变化」角标。
	addressChangedTop  float32 = 6
	addressChangedRgt  float32 = 8
	addressChangedPadL float32 = 8
	addressChangedLine float32 = 16
	// `.address:focus{outline:1px solid state-business-primary;outline-offset:-1px}`
	// （Browser.module.css:115-118）：焦点描边宽 1。
	addressFocusRing float32 = 1
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
			// 官方向导页的入口卡（`browserDefinition.guide`，definition.tsx:22-25）：
			// id 'new' / commandId 'browser.new' / order 30 / title '浏览器' /
			// description '浏览网页'（locales.ts:4-5）。本包**不注册**卡片本体，
			// 所以向导页画官方那枚标准胶囊（GuideBody.tsx:155 的 `fallback: <EntryBox>`）。
			Guide: []pluginapi.TabGuide{{
				ID: "new", Order: 30, Title: "浏览器", Description: "浏览网页",
				CommandID: "browser.new",
			}},
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
	// 官方 `address.changed`（locales.ts:8）。
	copyAddressChanged  = "URL 已变化"
	copyStart           = "输入 HTTP(S) 地址开始浏览"
	copyLoading         = "正在打开…"
	copyBack            = "后退"
	copyForward         = "前进"
	copyReload          = "刷新"
	copyGo              = "前往"
	copyExternal        = "在系统浏览器中打开"
	copyRestorePrevious = "上次打开"
	copyRestoreAction   = "恢复页面"
	copyLoadFailed      = "页面加载失败；请刷新重试或在系统浏览器中打开。"
	copyAddressUnknown  = "页面已跳转；当前载体无法读取新的 URL。"
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
	instance := p.activate(env)
	// 官方 `tab.navigation.params` 的消费点：`BrowserBody.tsx:53,65` 把
	// `tab.navigation.params?.url` 当 initialUrl 交给 `controller.start(initialUrl)`。
	// 必须在这里（不是 afterDraw）：openPage 要改写地址栏与历史状态。
	p.applyNavigationParams(env, instance)
	p.drawn = false
	p.content = ui.Box{}
	t := p.theme
	ui.Column(f).FillWidth().FillHeight().Background(t.BgBase).Children(func() {
		p.drawToolbar(f, env)
		// `.toolbar` 的 `border-bottom:.5px solid border-l3`：viva 没有单边描边，
		// 用一条 0.5px 的实色行表达同一条线。官方 `.toolbar{height:38px}` +
		// `box-sizing:border-box` 意味着**这条线算在 38 之内**（内容盒 37.5），
		// 所以工具条行高取 `38 - 0.5`，两条合计正好 38（Browser.module.css:11-20）。
		ui.Column(f).FillWidth().Height(hairline).Shrink(0).Background(t.BorderL3)
		// 官方两行**并列**而不是二选一：`error`（载具加载错误）
		// 与 `addressFailure`（地址被拒）是两个独立分支（BrowserBody.tsx:112-115）。
		if p.openErr != nil {
			p.drawStatusRow(f, copyLoadFailed)
		}
		if copy := addressFailureCopy(p.failure); copy != "" {
			p.drawStatusRow(f, copy)
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
	addressID := env.Tree.ID(env.Feat, "browser-address")
	// 官方 `.addressBox:focus-within` 的两处效果（`.addressGo` 显形、`.addressChanged`
	// 隐去）都看"地址框是否持有焦点"。本仓的判据是画布上的焦点 id：同一个 id
	// 既交给 TextFieldID，也在这里比一次，语义与 focus-within 相同。
	addressFocused := env.Canvas != nil && !addressID.IsZero() && env.Canvas.Focused() == addressID
	unknown := p.unknown()
	ui.Row(f).FillWidth().Height(toolbarHeight-hairline).
		PadSides(toolbarPadY, toolbarPadX, toolbarPadY, toolbarPadX).
		Items(ui.Center).Gap(toolbarGap).Children(func() {
		p.drawToolButton(f, env, "browser-back", copyBack, iconChevronLeft, iconSizeChevron,
			p.nav.canGoBack(), func() { p.goBack(env) })
		p.drawToolButton(f, env, "browser-forward", copyForward, iconChevronRight, iconSizeChevron,
			p.nav.canGoForward(), func() { p.goForward(env) })
		_, hasTarget := currentTarget(p.nav.value)
		p.drawToolButton(f, env, "browser-reload", copyReload, iconRefresh, iconSizeTool,
			hasTarget, func() { p.reload(env) })

		// `.addressBox`
		ui.Row(f).Grow(1).Height(addressHeight).Shrink(1).Children(func() {
			// `.addressUnknown{color:label-tertiary;background:bg-layer-2}` +
			// `.addressUnknown:focus{color:label-primary;background:bg-layer-1}`
			// （Browser.module.css:85-96）：地址读不回来时整格降一档。
			background, foreground := t.BgLayer1, t.LabelPrimary
			if unknown && !addressFocused {
				background, foreground = t.BgLayer2, t.LabelTertiary
			}
			field := ui.TextFieldID(f, addressID, &p.address,
				copyAddressPlaceholder, copyAddressPlaceholder).
				FillWidth().Height(addressHeight).FontSize(t.Fonts.Caption).Color(foreground).
				Background(background).Border(hairline, t.BorderL2).Radius(t.Radii.SM).
				PadSides(0, addressPadRight, 0, addressPadLeft)
			// `.address:focus{outline:1px solid state-business-primary;outline-offset:-1px}`
			// （:115-118）。Viva 的 Ring 画在盒子外侧、宽 1（不改内容盒），
			// 与官方"描边不占位"一致；差异只是 offset 0 而非 -1（差 1px，已登记）。
			if addressFocused {
				field = field.Ring(addressFocusRing, t.Info)
			}
			if field.Submitted() {
				p.submit(env)
			}
			// `.addressChanged`（:98-108）：「URL 已变化」角标，`:focus-within` 时隐去。
			if unknown && !addressFocused {
				ui.Row(f).Height(addressChangedLine).Shrink(0).
					Absolute().Top(addressChangedTop).Right(addressChangedRgt).
					PadSides(0, 0, 0, addressChangedPadL).
					Background(t.BgLayer2).Children(func() {
					ui.Text(f, copyAddressChanged).FontSize(t.Fonts.Tiny).
						LineHeight(addressChangedLine).Color(t.LabelTertiary)
				})
			}
			// `.addressGo{position:absolute;top:0;right:0;visibility:hidden;opacity:0}` +
			// `.addressBox:focus-within .addressGo{visibility:visible;opacity:1}`
			// （:72-83）。`.address` 的 `padding-right:34px` 就是给它的位。
			if addressFocused {
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
			}
		})

		_, hasExternal := currentTarget(p.nav.value)
		// 官方 `externalUrl = unknown ? undefined : target?.url`（BrowserBody.tsx:73）：
		// 页面自己跳走后旧地址不再可信，所以这一枚按钮跟着 `disabled`（:99）。
		p.drawToolButton(f, env, "browser-external", copyExternal, iconRightUp, iconSizeExternal,
			hasExternal && !unknown && env.Act.OpenExternal != nil, func() {
				target, ok := currentTarget(p.nav.value)
				if !ok || env.Act.OpenExternal == nil {
					return
				}
				_ = env.Act.OpenExternal(target.URL)
			})
	})
}

// drawToolButton 画官方 `.tool`（28×28 圆角）并报告是否被点。
//
// 三档外观全部来自 Browser.module.css：
//
//	常态   color:label-secondary                                  （:22-35）
//	hover  color:label-primary + background:interactive-bg-hover   （:37-42）
//	禁用   color:label-quaternary + cursor:default                 （:44-47）
//
// 禁用色如实说明：`--dsw-alias-label-quaternary` 在 rc.2 的**全部官方产物里只有
// 引用、没有定义**（`design-platform.css` 全文 0 处赋值，`app.asar` 内 4 处
// `var(--dsw-alias-label-quaternary)` 全是用例），按 CSS 层叠规则这属于
// invalid-at-computed-value，`color` 回退成继承值（`.root` 的 label-primary）。
// 本仓取 `LabelDimmed` 让"禁用"在界面上看得出来，这一处登记在
// docs/official-parity.md 的 #50 行。
func (p *Plugin) drawToolButton(f ui.C, env *pluginapi.Env, name, label string,
	icon func(gfx.Color) gfx.Icon, iconSize float32, enabled bool, run func()) {
	t := p.theme
	var id gfx.ID
	if env.Tree != nil {
		id = env.Tree.ID(env.Feat, name)
	}
	color := t.LabelSecondary
	if enabled && env.Canvas != nil && (env.Canvas.Hovered() == id || env.Canvas.Pressed() == id) {
		color = t.LabelPrimary
	}
	if !enabled {
		color = t.LabelDimmed
	}
	box := ui.Row(f).Width(toolSize).Height(toolSize).Shrink(0).
		Items(ui.Center).Justify(ui.Center).Radius(t.Radii.SM)
	if enabled {
		box = box.Hover(ui.Look{}.Background(t.Hover)).Hit(id, label)
	}
	box.Children(func() {
		ui.Icon(f, icon(color)).Size(iconSize, iconSize).Color(color)
	})
	if enabled && box.Clicked() {
		run()
	}
}

// drawStatusRow 画官方 `.failure` 一行：6/12、xxxs-11、错误色文字、错误色 8% 底。
func (p *Plugin) drawStatusRow(f ui.C, copy string) {
	t := p.theme
	background := t.Error
	background.Alpha *= failureTint
	ui.Row(f).FillWidth().PadSides(statusRowPadY, statusRowPadX, statusRowPadY, statusRowPadX).
		Background(background).Children(func() {
		ui.Paragraph(f, copy, 2).FontSize(t.Fonts.Tiny).LineHeight(lineHeightXXS).
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
			ui.Text(f, copyAddressUnknown).FontSize(t.Fonts.Tiny).LineHeight(lineHeightXXS).
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
	if p.page == nil && p.openErr == nil {
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
	// 官方 `command()` 每次命令都清 addressFailure（BrowserController.ts:163-168），
	// 否则被拒的红行会一直挂在屏幕上。
	p.failure = ""
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
	p.failure = ""
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
	p.failure = ""
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

// applyNavigationParams 消费官方 `tab.navigation.params`（contract/params.ts:31-46 的
// `browser: { url?: string }`）。
//
// 官方的行为是**一次性**的：`BrowserController.start(initialUrl)` 里有
// `if (this.started || this.disposed) return`（BrowserController.ts:90-98），
// 所以同一 occurrence 再被 open 一次也不会重新导航。这里逐条照搬：
//
//	params.url 为空（官方 undefined） → 什么都不做
//	这一份参数已经消费过         → 什么都不做（同一帧重绘不会重复开页）
//	这个 occurrence 已经开过页面 → 什么都不做（官方 started）
//	否则                        → 过地址白名单，通过就导航，拒了就记 addressFailure
func (p *Plugin) applyNavigationParams(env *pluginapi.Env, instance *browserInstance) {
	if instance == nil {
		return
	}
	raw := env.RightTabParams.URL
	if instance.paramsURL == raw {
		return
	}
	instance.paramsURL = raw
	if raw == "" || instance.started {
		return
	}
	instance.started = true
	parsed := parseBrowserAddress(raw, "")
	if !parsed.OK {
		p.failure = parsed.Reason
		p.persistActive(env)
		return
	}
	p.failure = ""
	p.address.SetText(parsed.Target.URL)
	if p.nav.navigate(parsed.Target) == nil {
		return
	}
	p.openPage(env, parsed.Target)
}

// newWindowRequested 是官方 `BrowserController.openRequested` 的逐条移植：
//
//	OFF:…/browser/BrowserController.ts:54-59
//	  openRequested: value => { const r = parseBrowserAddress(value, applicationOrigin)
//	                            if (!r.ok) { this.addressFailed(r.reason); return }
//	                            options.openTab(r.target.url) }
//	OFF:…/view/BrowserBody.tsx:62-70
//	  openTab: url => tab.actions.openTab('browser', { params: { url }, revealIfOpened: false })
//
// 返回值语义见 `pluginapi.WebOpenOptions.OnNewWindow`：true = 应用内已经接下这次新窗口。
func (p *Plugin) newWindowRequested(env *pluginapi.Env, raw string) bool {
	parsed := parseBrowserAddress(raw, "")
	if !parsed.OK {
		// 地址被拒：官方记在 addressFailure 上（不开任何窗口），状态行由工具条下方那行显示。
		p.failure = parsed.Reason
		p.persistActive(env)
		return true
	}
	if env.Act.OpenRightTabParams == nil {
		// 壳没接带参数的 openTab：**不丢地址**，交回平台默认（系统浏览器）。
		return false
	}
	// `revealIfOpened: false` 在官方意味着"允许并存第二份同样的内容"。本仓 browser
	// 注册为 multiple:true，openRightTabParams 对 multiple 类型本来就每次新建，
	// 所以这一条语义天然成立（见 cmd/app/right_tab_occurrences.go）。
	env.Act.OpenRightTabParams(RightTabBrowser, pluginapi.RightTabParams{URL: parsed.Target.URL})
	return true
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
		page, err := env.Web.Open(pluginapi.WebOpenOptions{
			OnLoad: func(load pluginapi.WebLoad) {
				p.handleLoad(&loadEnv, load)
			},
			// 页面自己 `window.open` / `target=_blank`：官方载具把它交给
			// `BrowserController.openRequested`（BrowserController.ts:54-59），
			// 再由 `BrowserBody.tsx:66` 开成应用内新标签。这里同一条路。
			OnNewWindow: func(url string) bool {
				return p.newWindowRequested(&loadEnv, url)
			},
		})
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
