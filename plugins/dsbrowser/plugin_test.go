package dsbrowser

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 假载具 ──────────────────────────────────────────────────────────────

// fakePage 是 pluginapi.WebPage 的假实现：只记调用，不开窗口。
//
// 后四个字段摆的是**引擎事实**（官方 `element.getTitle()/canGoBack()/
// canGoForward()/isLoading()`，ElectronWebViewImpl.ts:220,224-226）：
// 用例直接改它们来表示"引擎此刻这么答"，不经过任何状态机会员。
type fakePage struct {
	id     gfx.NativeViewID
	loaded []string
	closed bool

	title      string
	canBack    bool
	canForward bool
	loading    bool
	// crash 是引擎"这一页的进程没了"的假事实：`IsCrashed()`/`ProcessFailure()`
	// 直接答它（对应 Viva 的 `WebView.IsCrashed/ProcessFailure`，
	// OFF:viva@0.6.9-procfail/app/web.go:399-409）。
	crash pluginapi.WebProcessFailure

	backs    int
	forwards int
}

func (p *fakePage) ID() gfx.NativeViewID { return p.id }
func (p *fakePage) Load(url string)      { p.loaded = append(p.loaded, url) }
func (p *fakePage) Reload()              { p.loaded = append(p.loaded, "<reload>") }
func (p *fakePage) Back()                { p.backs++ }
func (p *fakePage) Forward()             { p.forwards++ }
func (p *fakePage) Focus()               {}
func (p *fakePage) Close()               { p.closed = true }

func (p *fakePage) Title() string                               { return p.title }
func (p *fakePage) CanGoBack() bool                             { return p.canBack }
func (p *fakePage) CanGoForward() bool                          { return p.canForward }
func (p *fakePage) IsLoading() bool                             { return p.loading }
func (p *fakePage) IsCrashed() bool                             { return p.crash != pluginapi.WebProcessNoFailure }
func (p *fakePage) ProcessFailure() pluginapi.WebProcessFailure { return p.crash }

// fakeHost 是 pluginapi.WebHost 的假实现。
type fakeHost struct {
	page    *fakePage
	opened  int
	openErr error
	lastOn  func(pluginapi.WebLoad)
	onLoads []func(pluginapi.WebLoad)
	// onProcessFailures 是每次 Open 交进去的 `OnProcessFailed`；用例拿它模拟
	// "引擎报页面进程没了"（官方 render-process-gone / destroyed）。
	onProcessFailures []func(pluginapi.WebProcessFailure)
	// lastNewWindow 是本次 Open 交进去的 `OnNewWindow`（官方 BrowserPageOptions.openRequested
	// 的等价物）；用例用它模拟"页面自己 window.open 了一个地址"。
	lastNewWindow func(url string) bool
	// lastStorageAccount 是本次 Open 交进去的**存储账号**（官方
	// `bridge.acquire(workspace)` 的那个 workspace 入参，`src/types.ts:22`）。
	// 壳拿它换分区名（`cmd/app/web_pages.go` 的 webPartitionCache）。
	lastStorageAccount string
}

func (h *fakeHost) Open(options pluginapi.WebOpenOptions) (pluginapi.WebPage, error) {
	h.opened++
	h.lastStorageAccount = options.StorageAccount
	h.lastOn = options.OnLoad
	h.onLoads = append(h.onLoads, options.OnLoad)
	h.onProcessFailures = append(h.onProcessFailures, options.OnProcessFailed)
	h.lastNewWindow = options.OnNewWindow
	if h.openErr != nil {
		return nil, h.openErr
	}
	// 每次 Open 都是独立 occurrence 的载具，不能复用同一个 NativeViewID。
	h.page = &fakePage{id: gfx.NativeViewID(6 + h.opened)}
	return h.page, nil
}

// ── 画一帧 ──────────────────────────────────────────────────────────────

// renderBrowser 画一帧右栏 tab 体（右栏宽 360、高 600，与官方右栏档同一量级）。
func renderBrowser(t *testing.T, p *Plugin, env *pluginapi.Env) *gfx.Canvas {
	t.Helper()
	return renderBrowserInteraction(t, p, env, gfx.ID{}, gfx.ID{})
}

// renderBrowserInteraction 同上，但先记下一次交互状态（悬停/按住/焦点）：
// 官方的 `:hover` / `:focus-within` 两支靠它判定，测试必须能摆出这两种状态。
func renderBrowserInteraction(t *testing.T, p *Plugin, env *pluginapi.Env, hovered, focused gfx.ID) *gfx.Canvas {
	t.Helper()
	if p.theme == nil {
		p.theme = dscore.Dark()
	}
	tree := &gfx.Tree{}
	feat := tree.Feature("browser-test")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	env.Theme = p.theme
	env.Tree = tree
	env.Feat = feat
	canvas.SetInteraction(hovered, gfx.ID{}, focused, false)
	env.Canvas = canvas
	env.Bounds = gfx.Rect{Width: 360, Height: 600}
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		ui.Column(f).Width(360).Height(600).Children(func() { p.drawBody(f, env) })
	})
	return canvas
}

// ── 断言 ────────────────────────────────────────────────────────────────

// TestPanelsRegisterTheOfficialTwoSeats 对着官方两阶段注册的第二阶段：
// 体挂 `sidebar.right.pane.tab`、标题挂 `sidebar.right.pane.tab.title`，
// 两处 **同一个 key**（OFF:dsh-client-ui-sidebar-browser/src/client/index.ts:81-102）。
func TestPanelsRegisterTheOfficialTwoSeats(t *testing.T) {
	p := &Plugin{}
	panels := p.Panels()
	if len(panels) != 2 {
		t.Fatalf("应当注册 2 块画（体 + 标题），得到 %d", len(panels))
	}
	body, title := panels[0], panels[1]
	if body.Slot != pluginapi.SlotRightPaneTab {
		t.Fatalf("体挂在 %q，应当是 %q", body.Slot, pluginapi.SlotRightPaneTab)
	}
	if title.Slot != pluginapi.SlotRightPaneTabTitle {
		t.Fatalf("标题挂在 %q，应当是 %q", title.Slot, pluginapi.SlotRightPaneTabTitle)
	}
	if body.Key != RightTabBrowser || title.Key != RightTabBrowser {
		t.Fatalf("两个座位必须同一个 key：体 %q 标题 %q", body.Key, title.Key)
	}
	if RightTabBrowser != "browser" {
		t.Fatalf("kind 应当是官方 BROWSER_KIND='browser'，得到 %q", RightTabBrowser)
	}
	// 芯片文案 = 官方 zh `type.label`。
	if body.Label != "浏览器" {
		t.Fatalf("芯片文案 = %q，应当是官方 type.label 的「浏览器」", body.Label)
	}
	if body.IconImage.ViewBox != 16 || body.IconImage.Stroke != 1 || body.IconMarginRight != 4 {
		t.Fatalf("芯片标题图标应对应官方 globe 16 viewBox / stroke 1 / margin-right 4：%+v margin=%v",
			body.IconImage, body.IconMarginRight)
	}
	// 官方 tab 类型没有 order 字段，名录顺序 = 注册顺序；零值是"按装配顺序"。
	if body.Order != 0 || title.Order != 0 {
		t.Fatalf("order 应当是 0（官方没有这个字段），得到 %d/%d", body.Order, title.Order)
	}
	if !body.Multiple {
		t.Fatal("官方 definition.tsx:19 multiple:true 必须进入右栏类型注册")
	}
	if body.Draw == nil || body.AfterDraw == nil {
		t.Fatal("体必须同时有 Draw（布局）与 AfterDraw（摆 native view）")
	}
	if title.Draw == nil {
		t.Fatal("标题必须有 Draw")
	}
}

// TestBodyDrawsNothingUntilTheUserNavigates 是官方
// `BrowserController.start(undefined)` 的判据：**保存的地址不会自己开页**。
//
// 所以第一帧既不能开页，也不能画成功态——只有那条官方占位文案。
func TestBodyDrawsNothingUntilTheUserNavigates(t *testing.T) {
	host := &fakeHost{}
	p := &Plugin{}
	renderBrowser(t, p, &pluginapi.Env{Web: host})
	if host.opened != 0 {
		t.Fatalf("第一帧就开了页（opened=%d），保存的地址不该自己开页", host.opened)
	}
	if p.page != nil {
		t.Fatal("第一帧就有了页面")
	}
	if p.nav.value.Navigation.State != navEmpty {
		t.Fatalf("历史状态应当是 empty，得到 %v", p.nav.value.Navigation.State)
	}
}

// TestSubmitValidatesBeforeOpeningTheCarrier 是官方 `loadUrl` 的两段：
// 先 `parseBrowserAddress`，**不通过就只记原因、不开页**
// （OFF:src/client/browser/BrowserController.ts:132-146）。
func TestSubmitValidatesBeforeOpeningTheCarrier(t *testing.T) {
	host := &fakeHost{}
	env := &pluginapi.Env{Web: host}
	p := &Plugin{}
	p.theme = dscore.Dark()

	// ① 被拒的地址：记原因，不碰载具。
	p.address.SetText("file:///etc/passwd")
	p.submit(env)
	if p.failure != failProtocol {
		t.Fatalf("failure = %q，应当是 %q", p.failure, failProtocol)
	}
	if host.opened != 0 {
		t.Fatalf("被拒的地址开了页（opened=%d）", host.opened)
	}
	if addressFailureCopy(p.failure) == "" {
		t.Fatal("官方 error.* 文案缺一条（界面会画成空行）")
	}

	// ② 合法地址：开页 + 导航到**校验之后**的地址（不是用户原始输入）。
	p.address.SetText("example.com/a")
	p.submit(env)
	if p.failure != "" {
		t.Fatalf("合法地址仍带 failure=%q", p.failure)
	}
	if host.opened != 1 {
		t.Fatalf("opened = %d，应当是 1", host.opened)
	}
	if p.page == nil {
		t.Fatal("页面没留下")
	}
	loaded := host.page.loaded
	if len(loaded) != 1 || loaded[0] != "https://example.com/a" {
		t.Fatalf("导航地址 = %v，应当是校验后的 https://example.com/a", loaded)
	}
}

// TestOpenFailureKeepsThePageUntouched 是"载具放不了网页"那一档：
// 不许假装成功（官方同判据：两个载具都没有时这个 tab 类型根本不该被注册）。
func TestOpenFailureKeepsThePageUntouched(t *testing.T) {
	host := &fakeHost{openErr: pluginapi.ErrWebUnsupported}
	p := &Plugin{}
	p.theme = dscore.Dark()
	p.address.SetText("example.com")
	p.submit(&pluginapi.Env{Web: host})
	if p.page != nil {
		t.Fatal("载具失败却留下了页面")
	}
	if p.openErr == nil {
		t.Fatal("载具失败没有记原因")
	}
}

// TestAfterDrawOnlyPlacesThePageWhileTheBodyIsDrawn 是本真机唯一会"糊屏"的一处：
//
// native view 一旦声明就会出现。右栏收起 / 切到别的 tab 时 tab 体不再被画，
// 若此时还声明，网页会盖在别的界面上（而且没有任何 Viva 绘制能盖回来）。
func TestAfterDrawOnlyPlacesThePageWhileTheBodyIsDrawn(t *testing.T) {
	host := &fakeHost{}
	env := &pluginapi.Env{Web: host}
	p := &Plugin{}
	p.theme = dscore.Dark()

	// ① 有页面、但本帧画过体 → 正好一个 native view，落在内容区里。
	p.address.SetText("example.com")
	p.submit(env)
	canvas := renderBrowser(t, p, env)
	p.afterDraw(canvas, env)
	views := canvas.NativeViews()
	if len(views) != 1 {
		t.Fatalf("画过体之后应当摆 1 个 native view，得到 %d", len(views))
	}
	if views[0].ID != 7 {
		t.Fatalf("native view id = %d，应当是页面的 id 7", views[0].ID)
	}
	if views[0].Bounds.Height <= 0 || views[0].Bounds.Width <= 0 {
		t.Fatalf("native view 矩形没有面积：%+v", views[0].Bounds)
	}
	// 工具条 38px + 0.5px 分隔线之下才是网页，所以纵向不能从 0 开始。
	if views[0].Bounds.Y < toolbarHeight {
		t.Fatalf("网页矩形 y=%v，应当在工具条（%v）之下", views[0].Bounds.Y, toolbarHeight)
	}

	// ② 没有重画就再跑一次 afterDraw → 不许再声明（这正是"切走就隐藏"）。
	canvas2 := renderBrowser(t, p, env)
	_ = canvas2
	p.afterDraw(canvas2, env)
	p.afterDraw(canvas2, env)
	if got := len(canvas2.NativeViews()); got != 1 {
		t.Fatalf("第二次 afterDraw 又声明了一次（共 %d 次），切走时网页会糊在别的界面上", got)
	}
}

// TestTitleFallsBackToTheOfficialTypeLabel 是官方 `BrowserTitle`：
// `entry?.title ?? tab.title`（OFF:src/client/view/BrowserTitle.tsx:15-17）。
func TestTitleFallsBackToTheOfficialTypeLabel(t *testing.T) {
	p := &Plugin{}
	p.theme = dscore.Dark()
	if label := titleLabelFor(p); label != "浏览器" {
		t.Fatalf("没有目标时标题 = %q，应当是官方 type.label「浏览器」", label)
	}
	p.nav.navigate(browserTarget{Kind: kindHTTPS, URL: "https://example.com/x", Title: "example.com"})
	if label := titleLabelFor(p); label != "example.com" {
		t.Fatalf("有目标时标题 = %q，应当是主机名 example.com", label)
	}
}

// titleLabelFor 取 drawTitle 用的那份文案：**直接调生产判据**，不再在测试里
// 复制一份回退链（复制过的那一版在"标题来自引擎"之后就必然漂移）。
func titleLabelFor(p *Plugin) string { return p.currentTitle() }

func TestToolbarIconSizesMatchOfficialSprite(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  float32
		want float32
	}{
		{"chevron-left default 14", iconSizeChevron, 14},
		{"refresh default 16", iconSizeTool, 16},
		{"external explicit size={14}", iconSizeExternal, 14},
		{"globe default 14", iconSizeTitle, 14},
	} {
		if tc.got != tc.want {
			t.Fatalf("%s：实现 %v，官方 %v", tc.name, tc.got, tc.want)
		}
	}
	if browserTitleIconMargin != 4 || browserTitleGap != 9 {
		t.Fatalf("标题间距 = margin %v + gap 5，官方合成 9，得到 %v", browserTitleIconMargin, browserTitleGap)
	}
	if browserTitleLineHeight != 18.2 {
		t.Fatalf("标题行高 = %v，官方 13×1.4=18.2", browserTitleLineHeight)
	}
}
