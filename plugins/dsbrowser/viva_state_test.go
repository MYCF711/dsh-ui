// viva_state_test.go —— 账本 #50 四条「卡在 Viva 契约」的解封判据。
//
// 2026-11-20 之前这四条在 docs/official-parity.md 的 #50 行里写着「仍拿不到」，
// 依据是 viva@v0.6.5 的 `app.WebLoad` 只有 Phase/URL、`*app.WebView` 没有
// Title/CanGoBack/CanGoForward/IsLoading。v0.6.8-webstate 把它们都补上了
// （OFF:viva@0.6.8-webstate/app/web.go:189-236 的 WebLoad 五字段、:344-355 的四个方法），
// 于是官方那边的四条对等物逐条接得住：
//
//	① 标题      官方 `page-title-updated` / `element.getTitle()`   ElectronWebViewImpl.ts:173-176,220
//	② 失败详情  官方 `did-fail-load` 的 code/description            同文件 :178-183 → BrowserBody.tsx:112-114
//	③ 历史可用  官方 `element.canGoBack()/canGoForward()`          同文件 :225-226 → BrowserBody.tsx:82-83
//	④ 加载中    官方 `element.isLoading()`                         同文件 :224 → BrowserBody.tsx:116,127
//
// 本文件钉的是**行为**：标题取引擎真值并按官方那三段回退、失败文案按官方两条判据选、
// 后退/前进的可用性与动作都归引擎、加载态只有一个出处。
package dsbrowser

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
)

// ── 夹具 ────────────────────────────────────────────────────────────────

// openBrowser 打开一个 occurrence 并导航到 address（等价于用户敲回车）。
// 返回的 load 是这个载具的交事件口：`load(pluginapi.WebLoad{...})` 模拟引擎报事件。
func openBrowser(t *testing.T, address string) (p *Plugin, host *fakeHost, env *pluginapi.Env,
	cfg map[string]string, load func(pluginapi.WebLoad)) {
	t.Helper()
	cfg = map[string]string{}
	host = &fakeHost{}
	p = &Plugin{}
	p.theme = dscore.Dark()
	env = occurrenceEnv("s1", "tab1", host, cfg)
	navigateOccurrence(p, env, address)
	if len(host.onLoads) != 1 || host.onLoads[0] == nil {
		t.Fatalf("开页没有交 OnLoad：%d 个订阅", len(host.onLoads))
	}
	if p.page == nil {
		t.Fatal("前置：页面没开出来")
	}
	return p, host, env, cfg, host.onLoads[0]
}

// toolbarHit 画一帧工具条，报告某个按钮是否**登记了命中区**。
//
// 官方那三个按钮的禁用判据是 DOM 的 `disabled`（不接事件、也不进 tab 序）；
// 本仓的等价物是 `drawToolButton` 只在 `enabled` 时 `.Hit(...)`
// （plugin.go 的 drawToolButton），所以"有没有命中区"就是"亮不亮"的可测形式。
func toolbarHit(t *testing.T, p *Plugin, env *pluginapi.Env, name string) bool {
	t.Helper()
	tree := &gfx.Tree{}
	feat := tree.Feature("browser-viva-state")
	canvas := renderBrowserInteractionWith(t, p, env, tree, feat, gfx.ID{})
	_, hit := canvas.HitBounds(tree.ID(feat, name))
	return hit
}

// ── ① 标题 ──────────────────────────────────────────────────────────────

// TestTabTitleUsesTheEngineTitle 是 ① 的主判据，判据链逐条对官方：
//
//	Committed 报来的标题 → 应用历史项的标题（官方 `element.getTitle() || parsed.target.title`，:220）
//	tab 标题             → 引擎此刻的 Title()（官方 `page-title-updated`，:173-176）
//	引擎没有标题         → 地址解析出的主机名（同一行的 `||` 右支）
//	连目标都没有         → tab 类型名（官方 `entry?.title ?? tab.title`，BrowserTitle.tsx:15-17）
func TestTabTitleUsesTheEngineTitle(t *testing.T) {
	p, _, _, cfg, load := openBrowser(t, "https://example.com/start")
	page := p.page.(*fakePage)

	// 还没有任何目标时：官方 `tab.title` = 类型名（locales `type.label`）。
	empty := &Plugin{}
	empty.theme = dscore.Dark()
	if got := empty.currentTitle(); got != browserTypeLabel {
		t.Fatalf("没有目标时标题 = %q，应当是类型名 %q", got, browserTypeLabel)
	}

	// Committed：引擎报来真标题 → 应用历史项跟着换（官方 observeReady 那一步）。
	page.title = "Example Domain"
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadCommitted, URL: "https://example.com/start", Title: "Example Domain"})
	target, ok := currentTarget(p.nav.value)
	if !ok || target.Title != "Example Domain" {
		t.Fatalf("Committed 之后历史项标题 = %q，应当是引擎标题 Example Domain", target.Title)
	}
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://example.com/start", Title: "Example Domain"})
	if got := p.currentTitle(); got != "Example Domain" {
		t.Fatalf("tab 标题 = %q，应当是引擎标题 Example Domain", got)
	}
	// 持久化 checkpoint 里也必须是它：官方每次 observeReady 都 persist(target)（:231）。
	if got := savedBrowserTabs(t, cfg, "s1")["tab1"].Entries[0].Title; got != "Example Domain" {
		t.Fatalf("持久化标题 = %q，应当是引擎标题", got)
	}

	// 引擎后来把标题清空（页面换文档、还没标题）：退回地址解析出的主机名。
	page.title = ""
	if got := p.currentTitle(); got != "example.com" {
		t.Fatalf("引擎没标题时 = %q，应当退回主机名 example.com", got)
	}
}

// TestCommittedWithoutATitleFallsBackToTheParsedHostname 是官方 `||` 的右支：
// `element.getTitle() || parsed.target.title`（ElectronWebViewImpl.ts:220）。
// **不许**退回空串，也不许拿上一次的标题冒充新文档。
func TestCommittedWithoutATitleFallsBackToTheParsedHostname(t *testing.T) {
	p, _, _, cfg, load := openBrowser(t, "https://example.com/start")
	page := p.page.(*fakePage)
	page.title = ""

	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadCommitted, URL: "https://other.example/landing"})
	target, ok := currentTarget(p.nav.value)
	if !ok || target.Title != "other.example" {
		t.Fatalf("引擎没给标题也没有主机名兜底：%+v", target)
	}
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://other.example/landing"})
	if got := p.currentTitle(); got != "other.example" {
		t.Fatalf("tab 标题 = %q，应当是主机名 other.example", got)
	}
	if got := savedBrowserTabs(t, cfg, "s1")["tab1"].Entries[0].Title; got != "other.example" {
		t.Fatalf("持久化标题 = %q，应当是主机名", got)
	}
}

// TestTitleArrivingAfterTheCommitIsPersisted 是 `page-title-updated` 那条**晚到**的标题：
//
// 官方 did-navigate 那一刻 `getTitle()` 常常还是空的（新文档还没解析出标题），于是
// observeReady 用 `|| parsed.target.title` 退回主机名；标题真正到位时是后面那次
// page-title-updated（ElectronWebViewImpl.ts:173-176）再走一遍同一个 observeReady，
// 那一步也会 persist(target)（:231）——所以**晚到的标题必须进持久化**，
// 否则重启后恢复卡上永远是主机名。
func TestTitleArrivingAfterTheCommitIsPersisted(t *testing.T) {
	p, _, _, cfg, load := openBrowser(t, "https://example.com/start")
	page := p.page.(*fakePage)

	// Committed 时引擎还没标题 → 官方退回主机名。
	page.title = ""
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadCommitted, URL: "https://example.com/start"})
	if target, _ := currentTarget(p.nav.value); target.Title != "example.com" {
		t.Fatalf("Committed 时标题 = %q，应当退回主机名", target.Title)
	}
	// Done 时标题到位 → 写回历史项与 checkpoint。
	page.title = "Example Domain"
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://example.com/start", Title: "Example Domain"})
	if got := savedBrowserTabs(t, cfg, "s1")["tab1"].Entries[0].Title; got != "Example Domain" {
		t.Fatalf("晚到的标题没有进持久化：%q", got)
	}
	if got := p.currentTitle(); got != "Example Domain" {
		t.Fatalf("tab 标题 = %q", got)
	}
}

// TestTitleIsNotAdoptedBeforeTheAddressIsObserved 是 `page-title-updated` 的**时机**：
//
//	官方 ElectronWebViewImpl.ts:173-176
//	  page-title-updated → observe(name === 'page-title-updated' && address === 'observed')
//
// 也就是"地址已经是 observed"才采纳标题；导航进行中报来的是**正要离开**那一页的标题
// （viva 的 `WebLoadStart` 同样写着"still the title of the document being left"，
// OFF:viva@0.6.8-webstate/app/web.go:226-229），不许拿来冒充新页面。
func TestTitleIsNotAdoptedBeforeTheAddressIsObserved(t *testing.T) {
	p, _, _, _, load := openBrowser(t, "https://example.com/start")
	page := p.page.(*fakePage)
	page.title = "旧文档标题"

	// 页内导航开始：此刻引擎手里还是旧文档的标题。
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadStart, URL: "https://other.example/landing", Title: "旧文档标题"})
	if got := p.currentTitle(); got != "example.com" {
		t.Fatalf("导航进行中标题 = %q，应当还是主机名（官方只在 observed 之后采纳标题）", got)
	}
	if target, _ := currentTarget(p.nav.value); target.Title == "旧文档标题" {
		t.Fatal("Started 的标题被写进了应用历史项（那是正要离开那一页的标题）")
	}

	// Committed 之后新文档已经收下：新标题这时才算数。
	page.title = "新文档标题"
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadCommitted, URL: "https://other.example/landing", Title: "新文档标题"})
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://other.example/landing", Title: "新文档标题"})
	if got := p.currentTitle(); got != "新文档标题" {
		t.Fatalf("Done 之后标题 = %q，应当是引擎报来的新文档标题", got)
	}
}

// TestTitleFollowsTheEngineAfterTheLoadFinished 是官方"标题在加载完成之后还会变"那一条
// （ElectronWebViewImpl.ts:173-176 每次 page-title-updated 都重观测；Viva 没有这个事件，
// 但 `WebView.Title()` 是即时查询，OFF:viva@0.6.8-webstate/app/web.go:343-345 的注释
// 就写明"toolbar reads it on the paint it draws for"）。
func TestTitleFollowsTheEngineAfterTheLoadFinished(t *testing.T) {
	p, _, _, _, load := openBrowser(t, "https://example.com/start")
	page := p.page.(*fakePage)
	page.title = "标题一"
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadCommitted, URL: "https://example.com/start", Title: "标题一"})
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://example.com/start", Title: "标题一"})
	if got := p.currentTitle(); got != "标题一" {
		t.Fatalf("前置标题 = %q", got)
	}
	// 页面自己改了 document.title（SPA）：没有任何事件，标题也必须立刻跟上。
	page.title = "标题二（未读消息）"
	if got := p.currentTitle(); got != "标题二（未读消息）" {
		t.Fatalf("页面改标题之后 tab 还写着 %q", got)
	}
}

// containsPrefix 报告画布上是否画过以 prefix 开头的 text run（换行段落按行落库）。
func containsPrefix(canvas *gfx.Canvas, prefix string) bool {
	for _, text := range canvas.Texts() {
		if strings.HasPrefix(text.Value, prefix) {
			return true
		}
	}
	return false
}

// ── ② 失败详情 ──────────────────────────────────────────────────────────

// TestLoadFailureShowsTheOfficialDetailLine 是 ② 的主判据：官方 `.failure` 那一行
//
//	BrowserBody.tsx:112-114
//	  error.code !== undefined && error.description !== undefined
//	    ? t('load.failed.detail', { code: String(error.code), description: error.description })
//	    : t('load.failed')
//
// 模板逐字照抄 locales.ts:27 的中文：`页面加载失败 ({code}): {description}`。
func TestLoadFailureShowsTheOfficialDetailLine(t *testing.T) {
	p, _, env, _, load := openBrowser(t, "https://missing.invalid/")
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadStart, URL: "https://missing.invalid/"})
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://missing.invalid/",
		Code: pluginapi.WebLoadHostNotFound, Description: "net::ERR_NAME_NOT_RESOLVED"})

	want := "页面加载失败 (host name not found): net::ERR_NAME_NOT_RESOLVED"
	if got := p.frame.copy(); got != want {
		t.Fatalf("失败文案 = %q，应当是官方的 load.failed.detail 形状 %q", got, want)
	}
	// 真画一遍：`.failure` 那一行必须是**详情**那一档。
	// 断言用小前缀而不是整句：`.failure` 是换行段落（ui.Paragraph，2 行上限），
	// 长句会被折成两条 text run，整句相等会误判。
	canvas := renderBrowser(t, p, env)
	if !containsPrefix(canvas, "页面加载失败 (host name not found):") {
		t.Fatalf("画布上没有失败详情行（前 30 字）：%v", canvas.Texts())
	}
	if containsText(canvas, copyLoadFailed) {
		t.Fatal("同时画了通用的 load.failed：官方那两条判据是二选一")
	}
	// 官方 `failed()` 同时把 loading 收掉（ElectronWebViewImpl.ts:243）。
	if p.frame.loading {
		t.Fatal("失败之后仍是 loading：官方 failed() 里 loading=false")
	}
}

// TestLoadFailureWithoutADescriptionIsTheOfficialPlainLine 是 Windows 的那一档：
// WebView2 只给状态、不给文本（OFF:viva@0.6.8-webstate/internal/platform/web_windows.go:391
// 传的是空串），官方那两条判据里 "description !== undefined" 不成立，于是画 `load.failed`。
//
// **不许**把分类名塞进 detail 模板冒充"引擎说了什么"。
func TestLoadFailureWithoutADescriptionIsTheOfficialPlainLine(t *testing.T) {
	p, _, env, _, load := openBrowser(t, "https://missing.invalid/")
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://missing.invalid/",
		Code: pluginapi.WebLoadHostNotFound, Description: ""})

	if got := p.frame.copy(); got != copyLoadFailed {
		t.Fatalf("没有 description 时 = %q，应当是官方 load.failed %q", got, copyLoadFailed)
	}
	if strings.Contains(p.frame.copy(), "host name not found") {
		t.Fatal("引擎没给 description，却把分类名塞进了 detail 模板")
	}
	if canvas := renderBrowser(t, p, env); !containsText(canvas, copyLoadFailed) {
		t.Fatal("画布上没有官方那句 load.failed")
	}
}

// TestCancelledNavigationIsNotAFailure 是官方 `errorCode !== -3` 那条（ElectronWebViewImpl.ts:180）：
// Chromium 的 ERR_ABORTED（被新导航打断、被策略取消）不算"页面加载失败"。
// Viva 的 -3 对等物是 `WebLoadCancelled`（"被停掉的导航"，viva 自己的注释口径）。
func TestCancelledNavigationIsNotAFailure(t *testing.T) {
	p, _, env, _, load := openBrowser(t, "https://example.com/start")
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://example.com/start",
		Code: pluginapi.WebLoadCancelled, Description: "cancelled"})
	if got := p.frame.copy(); got != "" {
		t.Fatalf("被取消的导航被画成了失败：%q", got)
	}
	if p.frame.failed {
		t.Fatal("被取消的导航记成了 failed")
	}
	if canvas := renderBrowser(t, p, env); containsText(canvas, copyLoadFailed) {
		t.Fatal("画布上出现了 load.failed 行")
	}
}

// TestANewNavigationClearsTheFailure 是官方两个入口共同的清错误动作：
// loadUrl（:82）与 did-start-navigation（:170）都写 `error: undefined`。
func TestANewNavigationClearsTheFailure(t *testing.T) {
	p, _, env, _, load := openBrowser(t, "https://missing.invalid/")
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://missing.invalid/",
		Code: pluginapi.WebLoadHostNotFound})
	if !p.frame.failed {
		t.Fatal("前置：失败没有记下来")
	}
	// ① 用户再敲一个地址 → 立刻清（官方 :82）。
	p.address.SetText("https://example.com/next")
	p.submit(env)
	if p.frame.failed || p.frame.copy() != "" {
		t.Fatalf("新导航没有清掉上一次的失败：%q", p.frame.copy())
	}
	// ② 页面自己发起一次导航 → 也清（官方 did-start-navigation，:170）。
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://example.com/next",
		Code: pluginapi.WebLoadServerUnreachable})
	if !p.frame.failed {
		t.Fatal("前置：第二次失败没有记下来")
	}
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadStart, URL: "https://example.com/next"})
	if p.frame.failed {
		t.Fatal("页内导航开始没有清掉上一次的失败")
	}
}

// ── ③ 前进/后退可用性 ───────────────────────────────────────────────────

// TestHistoryButtonsFollowTheEngineNotTheAppHistory 是 ③ 的主判据。
//
// 官方 BrowserBody.tsx:82-83 的 `disabled={!frame.canGoBack}` / `!frame.canGoForward}`
// 里的 `frame.canGoBack` 来自 `element.canGoBack()`（ElectronWebViewImpl.ts:225-226）。
// 本仓应用历史里明明有两条（旧实现会因此把「后退」点亮），引擎说没有就不能亮。
func TestHistoryButtonsFollowTheEngineNotTheAppHistory(t *testing.T) {
	p, _, env, _, load := openBrowser(t, "https://example.com/one")
	navigateOccurrence(p, env, "https://example.com/two")
	page := p.page.(*fakePage)
	// 前置：应用历史确实有两条、index=1（旧判据 `nav.canGoBack()` 会返回 true）。
	if len(p.nav.value.Entries) < 2 || !p.nav.canGoBack() {
		t.Fatalf("前置：应用历史应当有多条，entries=%d", len(p.nav.value.Entries))
	}

	// ① 引擎说两个方向都没有历史 → 两个按钮都不给命中区（= 禁用）。
	page.canBack, page.canForward = false, false
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadCommitted, URL: "https://example.com/two"})
	if toolbarHit(t, p, env, "browser-back") {
		t.Fatal("引擎说不能后退，后退按钮却是亮的（还在拿应用历史猜）")
	}
	if toolbarHit(t, p, env, "browser-forward") {
		t.Fatal("引擎说不能前进，前进按钮却是亮的")
	}

	// ② 引擎说两个方向都有 → 两个按钮都亮。
	page.canBack, page.canForward = true, true
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadCommitted, URL: "https://example.com/two"})
	if !toolbarHit(t, p, env, "browser-back") {
		t.Fatal("引擎说能后退，后退按钮却没亮")
	}
	if !toolbarHit(t, p, env, "browser-forward") {
		t.Fatal("引擎说能前进，前进按钮却没亮")
	}
}

// TestBackAndForwardMoveTheEngineHistory 是 ③ 的另一半：**动作**也归引擎。
//
// 官方 BrowserController.goBack → frame.goBack → `if (!canGoBack) return; navigate('goBack')`
// → `element.goBack()`（BrowserController.ts:118、ElectronWebViewImpl.ts:89-96）。
// 不许再拿应用历史里的 URL 去 `Load()`——那是 iframe 载具的做法（IframeImpl.ts:59-61）。
func TestBackAndForwardMoveTheEngineHistory(t *testing.T) {
	p, _, env, _, _ := openBrowser(t, "https://example.com/one")
	navigateOccurrence(p, env, "https://example.com/two")
	page := p.page.(*fakePage)
	loadsBefore := len(page.loaded)

	// 引擎说两边都不可用：按了也不许动（官方 `if (!snapshot.canGoBack) return`）。
	p.goBack(env)
	p.goForward(env)
	if page.backs != 0 || page.forwards != 0 {
		t.Fatalf("引擎说没有历史，却调了 Back/Forward：%d/%d", page.backs, page.forwards)
	}

	// 引擎说都能用：动作打到引擎，且**不再**用应用历史拼一个 Load。
	page.canBack, page.canForward = true, true
	p.goBack(env)
	if page.backs != 1 {
		t.Fatalf("后退没有打到引擎历史：Back 调用 %d 次", page.backs)
	}
	if len(page.loaded) != loadsBefore {
		t.Fatalf("后退又去 Load 了应用历史里的地址：%v", page.loaded[loadsBefore:])
	}
	p.goForward(env)
	if page.forwards != 1 {
		t.Fatalf("前进没有打到引擎历史：Forward 调用 %d 次", page.forwards)
	}
	if len(page.loaded) != loadsBefore {
		t.Fatalf("前进又去 Load 了应用历史里的地址：%v", page.loaded[loadsBefore:])
	}
	// 官方 navigate() 会换代（`this.revision++`，:124）：上一代晚到的 Done 因此被丢掉。
	if p.nav.value.Navigation.State != navLoading {
		t.Fatalf("后退之后应用档位 = %v，应当进入 loading（等引擎 Committed）", p.nav.value.Navigation.State)
	}
}

// ── ④ 加载中 ────────────────────────────────────────────────────────────

// TestSubmitEntersLoadingBeforeTheEngineAnswers 是官方 loadUrl 的乐观置真
// （ElectronWebViewImpl.ts:82）：地址一提交，界面就是"正在打开…"，不等引擎回话。
func TestSubmitEntersLoadingBeforeTheEngineAnswers(t *testing.T) {
	p, host, _, _, _ := openBrowser(t, "https://example.com/start")
	if !p.frame.loading {
		t.Fatal("提交之后没有进 loading（官方 :82 是乐观置真）")
	}
	if p.contentPlaceholder() != copyLoading {
		t.Fatalf("占位文案 = %q，应当是 %q", p.contentPlaceholder(), copyLoading)
	}
	if host.page.loading {
		t.Fatal("前置：夹具里的引擎还没开始加载，这一条验的就是乐观那一档")
	}
}

// TestLoadingFollowsTheEngineOnEveryEvent 是 ④ 的核心：`loading` 的**唯一出处**是
// `element.isLoading()`（ElectronWebViewImpl.ts:224：`current.error === undefined &&
// element.isLoading()`），事件到一次就重新对一次表。
func TestLoadingFollowsTheEngineOnEveryEvent(t *testing.T) {
	p, _, _, _, load := openBrowser(t, "https://example.com/start")
	page := p.page.(*fakePage)

	// 引擎已经开始加载：Committed 时仍是 loading（官方这一步不是"加载完成"）。
	page.loading = true
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadCommitted, URL: "https://example.com/start"})
	if !p.frame.loading {
		t.Fatal("引擎说在加载，界面却不是 loading")
	}
	// 引擎说加载完了，但应用侧的档位还停在 loading（这一代还没收到 Done）：
	// **界面必须听引擎的**，不许两套状态打架。
	page.loading = false
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadCommitted, URL: "https://example.com/start"})
	if p.frame.loading {
		t.Fatal("引擎说加载完了，界面还写着正在打开（加载态被应用侧档位带跑了）")
	}
	// Done 也走同一次 observe（官方 did-stop-loading / dom-ready，:159-176）：
	// 引擎这时说还在加载（例如同一文档里又起了一次子资源加载），界面就得还是 loading。
	page.loading = true
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://example.com/start"})
	if !p.frame.loading {
		t.Fatal("Done 时引擎说在加载，界面却没进 loading（Done 那一步没有向引擎对表）")
	}
	page.loading = false
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://example.com/start"})
	if p.frame.loading {
		t.Fatal("Done 之后仍是 loading")
	}
}

// TestLoadingPlaceholderHasOneSource 直接钉"不许两套状态打架"：把应用侧档位摆成
// loading、引擎说没在加载，占位文案必须是 `start` 那一档；反过来也一样。
func TestLoadingPlaceholderHasOneSource(t *testing.T) {
	p, _, _, _, _ := openBrowser(t, "https://example.com/start")
	// ① 应用侧档位 = loading，引擎说不在加载 → 不画"正在打开…"。
	p.nav.value.Navigation = navStatus{State: navLoading, Revision: 1}
	p.frame.loading = false
	if got := p.contentPlaceholder(); got != copyStart {
		t.Fatalf("应用侧还在 loading、引擎没在加载时 = %q，应当按引擎走 %q", got, copyStart)
	}
	// ② 应用侧档位 = known，引擎说在加载 → 画"正在打开…"。
	p.nav.value.Navigation = navStatus{State: navKnown, Revision: 1}
	p.frame.loading = true
	if got := p.contentPlaceholder(); got != copyLoading {
		t.Fatalf("应用侧已经 known、引擎仍在加载时 = %q，应当按引擎走 %q", got, copyLoading)
	}
}
