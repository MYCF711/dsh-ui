// load_phase_test.go —— 载具三档加载事件的接线判据（账本 #50 ② 的 A 类那一半）。
//
// 官方 ElectronWebViewImpl.ts:159-186 有七个页面事件；Viva 只给了 `WebLoad` 的三档
// （`WebLoadStarted` / `WebLoadCommitted` / `WebLoadFinished`，
// OFF:viva@v0.6.5/app/web.go:177-190）。三档的官方含义逐条搬进本插件：
//
//	did-start-navigation（主框架）→ Loading true        ElectronWebViewImpl.ts:168-172
//	did-navigate                → address='observed'    ElectronWebViewImpl.ts:164,203-232
//	dom-ready / did-stop-loading→ loading=isLoading()   ElectronWebViewImpl.ts:159-186
//
// 本文件钉的是**行为**（状态机 + 地址栏 + 标题 + 持久化），不是形状：
// 页内导航（不调 submit）也要进 loading、Committed 到的地址要采纳成当前地址、
// 别的 occurrence 的事件不许传染、Done 的 revision 守卫不许被写坏。
package dsbrowser

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
)

// TestInPageNavigationEnterLoadingAndAdoptsTheCommittedURL 是 ② 的主判据：
// 页面自己跳走（页内点链接 / 重定向 / `location=`）**不经过** `submit()`，
// 今天也应该显示"正在打开…"，并把已提交的地址采纳成当前地址。
func TestInPageNavigationEnterLoadingAndAdoptsTheCommittedURL(t *testing.T) {
	cfg := map[string]string{}
	host := &fakeHost{}
	p := &Plugin{}
	env := occurrenceEnv("s1", "tab1", host, cfg)
	navigateOccurrence(p, env, "https://example.com/start")
	if len(host.onLoads) != 1 || host.onLoads[0] == nil {
		t.Fatalf("开页没有交 OnLoad：%d 个订阅", len(host.onLoads))
	}
	load := host.onLoads[0]
	firstRequest := p.nav.value.Request
	if firstRequest == nil {
		t.Fatal("前置：提交之后应当有请求")
	}
	if got := p.address.Text(); got != "https://example.com/start" {
		t.Fatalf("前置地址 = %q", got)
	}

	// ① Started：页内导航开始 → 进 loading（官方 did-start-navigation → loading:true）。
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadStart, URL: "https://other.example/landing"})
	if got := p.nav.value.Navigation.State; got != navLoading {
		t.Fatalf("页内导航开始后状态 = %v，应当是 loading", got)
	}
	next := p.nav.value.Request
	if next == nil || next.Revision <= firstRequest.Revision {
		t.Fatalf("页内导航必须换代：revision %v → %v", firstRequest.Revision, next.Revision)
	}
	// Started 还不采纳地址：官方要到 committed（`address='observed'`）才换 target。
	if got := p.address.Text(); got != "https://example.com/start" {
		t.Fatalf("Started 就把地址改了：%q", got)
	}
	if got := len(p.nav.value.Entries); got != 1 {
		t.Fatalf("页内导航不许新增应用历史项：%d 条", got)
	}

	// ② Committed：已提交的地址 = 当前地址（地址栏 + 标题 + 持久化都跟着走）。
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadCommitted, URL: "https://other.example/landing"})
	if got := p.address.Text(); got != "https://other.example/landing" {
		t.Fatalf("Committed 之后地址栏 = %q，应当是已提交的地址", got)
	}
	target, ok := currentTarget(p.nav.value)
	if !ok || target.Title != "other.example" || target.URL != "https://other.example/landing" {
		t.Fatalf("Committed 之后当前目标 = %+v，应当是 other.example 的已提交地址", target)
	}
	if got := p.RightTabTitle("s1", "tab1"); got != "other.example" {
		t.Fatalf("tab 标题 = %q，应当跟着已提交的地址变成 other.example", got)
	}
	// 官方 Committed 是"引擎收下文档"，还不是"加载完成"。
	if got := p.nav.value.Navigation.State; got != navLoading {
		t.Fatalf("Committed 之后状态 = %v，仍应当是 loading", got)
	}
	if got := len(p.nav.value.Entries); got != 1 {
		t.Fatalf("Committed 不许新增历史项：%d 条", got)
	}
	// 持久化 checkpoint 也必须已经是新地址（官方 persist(target) 在 observeReady 里）。
	if got := savedBrowserTabs(t, cfg, "s1")["tab1"]; got.Index != 0 ||
		len(got.Entries) != 1 || got.Entries[0].URL != "https://other.example/landing" {
		t.Fatalf("Committed 之后持久化 = %+v，应当是已提交的地址", got)
	}

	// ③ Done：不再是 loading（frameLoaded 仍然只认当前这一代）。
	load(pluginapi.WebLoad{Phase: pluginapi.WebLoadDone, URL: "https://other.example/landing"})
	if got := p.nav.value.Navigation.State; got != navKnown {
		t.Fatalf("Done 之后状态 = %v，应当是 known", got)
	}
	if p.unknown() {
		t.Fatal("Done 之后落进了 unknown：地址刚刚被采纳过，不该说「页面已跳转」")
	}
	if got := p.address.Text(); got != "https://other.example/landing" {
		t.Fatalf("Done 之后地址栏被改回去了：%q", got)
	}
}

// TestCommittedWithoutStartStillEndsKnown 是"载具只报后两档"的那一支：
// Committed 补一次开始，Done 回来时不许把**已经读到过**的地址记成 unknown。
//
// 依据：未知档（`navUnknown`）的官方含义是"页面跳走了、应用**读不到**新地址"
// （BrowserNavigation.ts:124-138 + `.addressUnknown`），而 committed 恰恰把地址
// 读到了。
func TestCommittedWithoutStartStillEndsKnown(t *testing.T) {
	p := &Plugin{nav: newBrowserNavigation()}
	p.nav.navigate(target("example.com"))
	request := p.nav.value.Request
	p.nav.frameLoaded(request.Revision)
	if p.nav.value.Navigation.State != navKnown {
		t.Fatalf("前置状态 = %v，应当是 known", p.nav.value.Navigation.State)
	}

	committed, changed := applyWebLoad(&p.nav, pluginapi.WebLoad{
		Phase: pluginapi.WebLoadCommitted, URL: "https://other.example/landing",
	})
	if !changed || committed == nil || committed.URL != "https://other.example/landing" {
		t.Fatalf("Committed 没有被采纳：changed=%v committed=%+v", changed, committed)
	}
	if got := p.nav.value.Navigation.State; got != navLoading {
		t.Fatalf("没看到 Started 的 Committed 之后状态 = %v，应当补成 loading", got)
	}
	if _, changed := applyWebLoad(&p.nav, pluginapi.WebLoad{Phase: pluginapi.WebLoadDone}); !changed {
		t.Fatal("Done 没有被当成一次状态推进")
	}
	if got := p.nav.value.Navigation.State; got != navKnown {
		t.Fatalf("Done 之后状态 = %v，应当是 known（地址已经读到过）", got)
	}
}

// TestCommittedAddressMustPassTheSameWhitelist 是地址白名单这一条：
// 引擎报回来的 URL 也走 `parseBrowserAddress`（官方 `parseBrowserAddress(element.getURL())`
// 在 observeReady 里，解析不通过就不改 target，ElectronWebViewImpl.ts:205-213）。
func TestCommittedAddressMustPassTheSameWhitelist(t *testing.T) {
	p := &Plugin{nav: newBrowserNavigation()}
	wanted := target("example.com")
	p.nav.navigate(wanted)

	committed, changed := applyWebLoad(&p.nav, pluginapi.WebLoad{
		Phase: pluginapi.WebLoadCommitted, URL: "file:///C:/secret.txt",
	})
	if changed || committed != nil {
		t.Fatalf("非 HTTP(S) 地址被采纳了：changed=%v committed=%+v", changed, committed)
	}
	if got := p.address.Text(); got != "" {
		t.Fatalf("被拒的提交地址改写了地址栏：%q", got)
	}
	if got := p.nav.value.Entries[0].URL; got != wanted.URL {
		t.Fatalf("被拒的提交地址改写了历史项：%q", got)
	}
}

// TestCommittedFromAnotherOccurrenceStaysOnItsOwnTab 是"陈旧事件"的另一半：
// 别的 tab / occurrence 的 Start / Committed 只能落在它自己身上。
//
// 判据与既有 Done 分支同源（`browserInstanceKey` + `p.currentKey`）：晚到的事件按
// openPage 时冻结的那份 Env 找 occurrence，找不到"当前"就只改那一份。
func TestCommittedFromAnotherOccurrenceStaysOnItsOwnTab(t *testing.T) {
	cfg := map[string]string{}
	host := &fakeHost{}
	p := &Plugin{}
	first := occurrenceEnv("s1", "tab1", host, cfg)
	second := occurrenceEnv("s1", "tab2", host, cfg)
	navigateOccurrence(p, first, "https://example.com/one")
	navigateOccurrence(p, second, "https://other.org/two")
	if p.currentKey != browserInstanceKey("s1", "tab2") {
		t.Fatalf("前置：当前 occurrence = %q", p.currentKey)
	}
	beforeAddress := p.address.Text()
	beforeTarget, _ := currentTarget(p.nav.value)

	// tab1 的载具（更早开的页）晚到一个页内导航。
	tab1Load := host.onLoads[0]
	tab1Load(pluginapi.WebLoad{Phase: pluginapi.WebLoadStart, URL: "https://intruder.example/x"})
	tab1Load(pluginapi.WebLoad{Phase: pluginapi.WebLoadCommitted, URL: "https://intruder.example/x"})

	if got := p.address.Text(); got != beforeAddress {
		t.Fatalf("别的 tab 的 Committed 改写了当前地址栏：%q → %q", beforeAddress, got)
	}
	if p.nav.value.Navigation.State != navLoading {
		t.Fatalf("别的 tab 的事件推进了当前状态机：%v", p.nav.value.Navigation.State)
	}
	if target, ok := currentTarget(p.nav.value); !ok || target.URL != beforeTarget.URL {
		t.Fatalf("别的 tab 的 Committed 改写了当前目标：%+v", target)
	}

	// 它必须落在 tab1 自己身上（历史项 + 地址草稿）。
	tab1 := p.instances[browserInstanceKey("s1", "tab1")]
	if tab1 == nil {
		t.Fatal("tab1 的 occurrence 丢了")
	}
	if target, ok := currentTarget(tab1.nav.value); !ok || target.URL != "https://intruder.example/x" {
		t.Fatalf("tab1 没有采纳自己的已提交地址：%+v", target)
	}
	if tab1.addressDraft != "https://intruder.example/x" {
		t.Fatalf("tab1 的地址草稿 = %q", tab1.addressDraft)
	}
}

// TestDoneWithAStaleRevisionIsStillDropped 钉住**没被写坏**的那条老判据：
// 晚到的旧 revision 的 Done 不许推进新请求（BrowserNavigation.ts:124-138）。
//
// 页内导航现在会换代（started），所以这条守卫更重要：上一代导航的 Done 必须失效。
func TestDoneWithAStaleRevisionIsStillDropped(t *testing.T) {
	p := &Plugin{nav: newBrowserNavigation()}
	first := p.nav.navigate(target("a.example"))
	second := p.nav.navigate(target("b.example"))
	if second.Revision != first.Revision+1 {
		t.Fatalf("revision 不是单调 +1：%d → %d", first.Revision, second.Revision)
	}
	// 第一代（更早那条导航）的 Done 晚到。
	p.nav.frameLoaded(first.Revision)
	if got := p.nav.value.Navigation.State; got != navLoading {
		t.Fatalf("旧 revision 的 Done 推进了状态机：%v", got)
	}
	p.nav.frameLoaded(second.Revision)
	if got := p.nav.value.Navigation.State; got != navKnown {
		t.Fatalf("当前 revision 的 Done 之后状态 = %v，应当是 known", got)
	}
}

// TestPageInitiatedNavigationBumpsTheRevisionSoLateDoneIsDropped 是上一条的端到端版本：
// 页内导航换代之后，**上一代**的 Done 不能把这一代标成完成。
func TestPageInitiatedNavigationBumpsTheRevisionSoLateDoneIsDropped(t *testing.T) {
	p := &Plugin{nav: newBrowserNavigation()}
	p.nav.navigate(target("example.com"))
	appRequest := p.nav.value.Request
	if _, changed := applyWebLoad(&p.nav, pluginapi.WebLoad{Phase: pluginapi.WebLoadStart}); !changed {
		t.Fatal("页内导航开始没有被登记")
	}
	pageRequest := p.nav.value.Request
	if pageRequest.Revision == appRequest.Revision {
		t.Fatal("页内导航没有换代")
	}
	// 应用那一次导航的 Done 晚到：按它自己的 revision 处理（plugin 里的 Done 用的是
	// 当前请求的 revision，所以这里直接喂旧一代，验证状态机丢弃它）。
	p.nav.frameLoaded(appRequest.Revision)
	if got := p.nav.value.Navigation.State; got != navLoading {
		t.Fatalf("旧一代的 Done 推进了状态机：%v", got)
	}
	p.nav.frameLoaded(pageRequest.Revision)
	if got := p.nav.value.Navigation.State; got != navKnown {
		t.Fatalf("这一代的 Done 之后状态 = %v，应当是 known", got)
	}
}
