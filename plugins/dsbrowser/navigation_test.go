package dsbrowser

import "testing"

// target 造一个测试用的导航目标。
func target(host string) browserTarget {
	return browserTarget{Kind: kindHTTPS, URL: "https://" + host + "/", Title: host}
}

// TestNavigateTruncatesTheForwardBranchAndBoundsHistory 对着
// OFF:dsh-client-ui-sidebar-browser/src/client/browser/BrowserNavigation.ts:74-89。
//
// 两件事都必须成立，否则 Back/Forward 会走错：
//   - 导航一次就丢掉"当前位置之后"的整段前进分支（浏览器语义）；
//   - 超过 100 条从头部丢（:6 的 MAX_BROWSER_HISTORY）。
func TestNavigateTruncatesTheForwardBranchAndBoundsHistory(t *testing.T) {
	n := newBrowserNavigation()
	n.navigate(target("a"))
	n.navigate(target("b"))
	n.navigate(target("c"))
	if !n.canGoBack() || n.canGoForward() {
		t.Fatalf("三条之后 Back=%v Forward=%v，应当是 Back=true Forward=false", n.canGoBack(), n.canGoForward())
	}
	if _, ok := currentTarget(n.value); !ok || n.value.Index != 2 {
		t.Fatalf("当前索引 = %d，应当是 2", n.value.Index)
	}
	// 回退一条，然后导航到新地址：原来的 "c" 必须被丢掉。
	if n.back() == nil {
		t.Fatal("应当能回退")
	}
	n.navigate(target("d"))
	if len(n.value.Entries) != 3 {
		t.Fatalf("截断后应当剩 3 条，得到 %d：%+v", len(n.value.Entries), n.value.Entries)
	}
	if got := n.value.Entries[2].Title; got != "d" {
		t.Fatalf("末条应当是 d，得到 %q（前进分支没被丢掉）", got)
	}
	if n.canGoForward() {
		t.Fatal("刚导航完不该还有前进分支")
	}

	// 上限：连推 150 条，只留最后 100 条。
	over := newBrowserNavigation()
	for i := 0; i < 150; i++ {
		over.navigate(target("h" + string(rune('a'+i%26))))
	}
	if len(over.value.Entries) != maxBrowserHistory {
		t.Fatalf("历史条数 = %d，应当是 %d", len(over.value.Entries), maxBrowserHistory)
	}
	if over.value.Index != maxBrowserHistory-1 {
		t.Fatalf("索引 = %d，应当是 %d", over.value.Index, maxBrowserHistory-1)
	}
}

// TestBackAndForwardFollowTheOfficialBounds 对着 BrowserNavigation.ts:30-45。
func TestBackAndForwardFollowTheOfficialBounds(t *testing.T) {
	n := newBrowserNavigation()
	if n.canGoBack() || n.canGoForward() {
		t.Fatal("空历史不该有 Back/Forward")
	}
	if n.back() != nil || n.forward() != nil {
		t.Fatal("空历史不该返回请求")
	}
	n.navigate(target("a"))
	if n.canGoBack() || n.canGoForward() {
		t.Fatal("只有一条时两端都不通")
	}
	n.navigate(target("b"))
	if !n.canGoBack() {
		t.Fatal("两条时应当能回退")
	}
	request := n.back()
	if request == nil || request.Target.Title != "a" {
		t.Fatalf("回退目标 = %+v，应当是 a", request)
	}
	if !n.canGoForward() {
		t.Fatal("回退之后应当能前进")
	}
	forward := n.forward()
	if forward == nil || forward.Target.Title != "b" {
		t.Fatalf("前进目标 = %+v，应当是 b", forward)
	}
}

// TestReloadKeepsTheSameEntry 对着 :112-116：重载不新增历史项，只换代。
func TestReloadKeepsTheSameEntry(t *testing.T) {
	n := newBrowserNavigation()
	first := n.navigate(target("a"))
	n.navigate(target("b"))
	before := n.back()
	if before == nil {
		t.Fatal("应当能回退")
	}
	entries := len(n.value.Entries)
	revisionBefore := n.value.Request.Revision
	reload := n.reload()
	if reload == nil {
		t.Fatal("应当能重载")
	}
	if len(n.value.Entries) != entries {
		t.Fatalf("重载改了历史条数：%d → %d", entries, len(n.value.Entries))
	}
	if reload.Revision != revisionBefore+1 {
		t.Fatalf("revision = %d，应当是 %d（每次请求单调 +1）", reload.Revision, revisionBefore+1)
	}
	if n.value.Request.Target.Title != "a" {
		t.Fatalf("重载目标 = %q，应当是 a", n.value.Request.Target.Title)
	}
	_ = first
}

// TestFrameLoadedWalksTheObservationStates 对着 :124-138。
//
// 三档是这条状态机的全部意义：
//
//	loading → known     （受控的那一帧加载完了）
//	known   → unknown   （页面自己又跳了；应用读不到新地址）
//	其他                 （空态 / 换代过的事件一律忽略）
func TestFrameLoadedWalksTheObservationStates(t *testing.T) {
	n := newBrowserNavigation()
	// 空态：任何 load 都不能改变它。
	n.frameLoaded(1)
	if n.value.Navigation.State != navEmpty {
		t.Fatalf("空态被推进到 %v", n.value.Navigation.State)
	}
	request := n.navigate(target("a"))
	if n.value.Navigation.State != navLoading {
		t.Fatalf("导航后应当是 loading，得到 %v", n.value.Navigation.State)
	}
	// 过期的一代：忽略。
	n.frameLoaded(request.Revision - 1)
	if n.value.Navigation.State != navLoading {
		t.Fatalf("过期的 load 推进了状态：%v", n.value.Navigation.State)
	}
	n.frameLoaded(request.Revision)
	if n.value.Navigation.State != navKnown {
		t.Fatalf("应当是 known，得到 %v", n.value.Navigation.State)
	}
	n.frameLoaded(request.Revision)
	if n.value.Navigation.State != navUnknown {
		t.Fatalf("第二次 load 应当是 unknown，得到 %v", n.value.Navigation.State)
	}
	// unknown 之后两端都不通（应用不知道自己在哪，不能假装能回退）。
	if n.canGoBack() || n.canGoForward() {
		t.Fatal("unknown 态不该报告可回退/可前进")
	}
}

// TestNewRequestClearsTheUnknownState 是 :140-155 的 `failure: undefined` +
// `navigation: loading` 那一跳：重新导航必须让地址栏回到"可信"。
func TestNewRequestClearsTheUnknownState(t *testing.T) {
	n := newBrowserNavigation()
	request := n.navigate(target("a"))
	n.frameLoaded(request.Revision) // loading → known
	n.frameLoaded(request.Revision) // known  → unknown
	if n.value.Navigation.State != navUnknown {
		t.Fatal("前置状态没走到 unknown")
	}
	n.navigate(target("b"))
	if n.value.Navigation.State != navLoading {
		t.Fatalf("新导航后应当是 loading，得到 %v", n.value.Navigation.State)
	}
	if !n.canGoBack() {
		t.Fatal("新导航后应当能回退")
	}
}
