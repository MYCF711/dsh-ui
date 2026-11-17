package dsbrowser

// window_test.go —— 官方「window.open 开应用内新标签」与 `tab.navigation.params` 的断言。
//
// 官方链路（逐条有出处）：
//
//	guest 页 window.open(url)
//	  → 载具 openRequested（OFF:…/electron/ElectronWebViewImpl.ts:155-158）
//	  → BrowserController.openRequested：先过地址白名单，拒就记 addressFailure
//	    （OFF:…/browser/BrowserController.ts:54-59）
//	  → tab.actions.openTab('browser', { params: { url }, revealIfOpened: false })
//	    （OFF:…/view/BrowserBody.tsx:62-70）
//	打开参数 `browser: { url?: string }`（OFF:…/index.ts:32-37）
//	initialUrl 只在第一次 start 时消费（OFF:…/browser/BrowserController.ts:90-98）

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
)

// windowEnv 造一个"浏览器 tab 的体"在画的 env。
func windowEnv(host pluginapi.WebHost) (*Plugin, *pluginapi.Env) {
	theme := dscore.Dark()
	p := &Plugin{theme: theme, nav: newBrowserNavigation()}
	p.instances = map[string]*browserInstance{}
	p.loadedSessions = map[string]bool{}
	p.dirtySessions = map[string]bool{}
	env := &pluginapi.Env{
		Theme: theme, Web: host,
		Data:       pluginapi.Snapshot{Cur: "s1"},
		RightTabID: "tab1",
	}
	env.Act.Notify = func(string) {}
	return p, env
}

func TestNewWindowOpensInAppTabWithURLParam(t *testing.T) {
	host := &fakeHost{}
	p, env := windowEnv(host)
	opened := pluginapi.RightTabParams{}
	gotKind := ""
	env.Act.OpenRightTabParams = func(kind string, params pluginapi.RightTabParams) {
		gotKind, opened = kind, params
	}
	// 一个真的已开页面（模拟打开 / 关掉无关，这里只为拿到 OnNewWindow）。
	p.openPage(env, browserTarget{Kind: kindHTTPS, URL: "https://start.example/", Title: "start.example"})
	if host.lastNewWindow == nil {
		t.Fatal("WebOpenOptions 没带 OnNewWindow：window.open 只能丢地址")
	}
	// 官方 openRequested 返回 true = 应用内接下这次新窗口。
	if consumed := host.lastNewWindow("https://example.com/path"); !consumed {
		t.Fatal("OnNewWindow 没在应用内接下这个地址")
	}
	if gotKind != RightTabBrowser || opened.URL != "https://example.com/path" {
		t.Fatalf("openTab 调用 = %q %+v，want browser + url", gotKind, opened)
	}
	if p.failure != "" {
		t.Fatalf("合法地址不该产生 addressFailure：%q", p.failure)
	}
}

func TestNewWindowRejectsInvalidAddressWithoutOpening(t *testing.T) {
	host := &fakeHost{}
	p, env := windowEnv(host)
	called := false
	env.Act.OpenRightTabParams = func(string, pluginapi.RightTabParams) { called = true }
	p.openPage(env, browserTarget{Kind: kindHTTPS, URL: "https://start.example/", Title: "start.example"})
	// 官方 BrowserController.ts:56-57：解析失败只记 addressFailure，不开任何标签。
	if consumed := host.lastNewWindow("file:///C:/secret.txt"); !consumed {
		t.Fatal("被拒的 new-window 也必须算应用内处理（不要漏给系统浏览器）")
	}
	if called {
		t.Fatal("被拒的地址不该开新标签")
	}
	if p.failure != failProtocol {
		t.Fatalf("addressFailure = %q，want %q", p.failure, failProtocol)
	}
}

func TestNewWindowKeepsURLWhenShellHasNoParamsChannel(t *testing.T) {
	host := &fakeHost{}
	p, env := windowEnv(host)
	env.Act.OpenRightTabParams = nil
	p.openPage(env, browserTarget{Kind: kindHTTPS, URL: "https://start.example/", Title: "start.example"})
	// 壳没接带参数的 openTab：交回平台默认（系统浏览器），**不能**吞掉地址。
	if consumed := host.lastNewWindow("https://example.com/"); consumed {
		t.Fatal("壳没有 params 通道时应交回平台默认，而不是假装已经开好")
	}
}

func TestNavigationParamsOpenInitialURLOnce(t *testing.T) {
	host := &fakeHost{}
	p, env := windowEnv(host)
	key := browserInstanceKey("s1", "tab1")
	instance := &browserInstance{nav: newBrowserNavigation()}
	p.instances[key] = instance
	p.currentKey = key
	env.RightTabParams = pluginapi.RightTabParams{URL: "https://example.com/first"}

	p.applyNavigationParams(env, instance)
	if host.opened != 1 {
		t.Fatalf("首次打开次数 = %d，want 1", host.opened)
	}
	if len(host.page.loaded) != 1 || host.page.loaded[0] != "https://example.com/first" {
		t.Fatalf("载具加载 = %v，want 参数里的那个地址", host.page.loaded)
	}
	// 同一份参数再来一遍（同一帧重绘 / 切回这个 occurrence）不该再开第二个页面。
	p.applyNavigationParams(env, instance)
	if host.opened != 1 {
		t.Fatalf("同一份 navigation.params 被消费了 %d 次", host.opened)
	}
	// 官方 `start` 是一次性的：即使之后换了参数，已经 started 的 occurrence 也不再导航。
	env.RightTabParams = pluginapi.RightTabParams{URL: "https://example.com/second"}
	p.applyNavigationParams(env, instance)
	if host.opened != 1 {
		t.Fatalf("已 started 的 occurrence 又开了一个页面：%d", host.opened)
	}
}

func TestNavigationParamsRejectInvalidURL(t *testing.T) {
	host := &fakeHost{}
	p, env := windowEnv(host)
	instance := &browserInstance{nav: newBrowserNavigation()}
	env.RightTabParams = pluginapi.RightTabParams{URL: "ftp://example.com/"}
	p.applyNavigationParams(env, instance)
	if host.opened != 0 {
		t.Fatal("被拒的参数地址不该开页面")
	}
	if p.failure != failProtocol {
		t.Fatalf("addressFailure = %q，want %q", p.failure, failProtocol)
	}
}
