// navigation.go —— 地址历史与"iframe 观察态"状态机。
//
// 官方 `OFF:dsh-client-ui-sidebar-browser/src/client/browser/BrowserNavigation.ts`
// 是一个**纯状态机**：它只管"应用自己知道的地址历史"，以及"页面是不是自己又
// 跳到别处了"。它不认识 iframe、不认识 Electron webview，所以搬过来一行都不用改
// 语义。
//
// 逐条对锚（官方行号 = BrowserNavigation.ts）：
//
//	:5    MAX_BROWSER_HISTORY = 100（每条 tab 保留的应用历史条数上限）
//	:14   `empty()`：entries []、index -1、request undefined、navigation empty
//	:24   `current(state)`：index < 0 → 没有
//	:30   `canGoBack(state)`：status !== 'unknown' && index > 0
//	:37   `canGoForward`：status !== 'unknown' && index >= 0 && index < entries.length - 1
//	:74   `navigate(target)`：截断前进分支、追加、超 100 从头部丢
//	:91   `back()` / :102 `forward()` / :112 `reload()`
//	:124  `frameLoaded(revision)`：loading → known；known → unknown（**只认同一 revision**）
//	:140  `request()`：revision 单调 +1、status 置 loading、清 failure
//
// 为什么"known → unknown"要单独一档：iframe 里页面自己跳到别的地址之后，
// 应用**读不到**新地址（跨源）。这时候地址栏里那条旧地址就不再是真相，
// 官方用 `.addressUnknown`（灰底）+ 一行 "页面已跳转；当前载体无法读取新的 URL。"
// 如实说明，而不是继续显示旧地址冒充当前地址。这边的 `<webview>`/WebView2
// 同样读不到导航后的地址（只有 `OnLoad` 报 URL，不保证与地址栏一致），
// 所以这一档原样保留。
//
// # 接了 Committed 之后 unknown 还剩什么
//
// `commit`（官方 `did-navigate` → `address='observed'`）会把手里的地址换成引擎
// **已提交**的那个 URL，于是"页面自己跳走了、应用读不到新地址"这一档变窄了：
// 只要这一代导航看到过 Committed，地址就是可信的。`unknown` 现在只剩一种来路——
// **一次 Done 找不到对应的开始**（这一代没看到 Started/Committed）：那说明应用对
// 这次导航确实一无所知，如实降档而不是继续冒充旧地址。判据仍是 frameLoaded 的
// `known → unknown` 那一跳，revision 守卫没动。
package dsbrowser

// maxBrowserHistory 是每条 tab 保留的应用历史条数上限（官方同值）。
const maxBrowserHistory = 100

// navState 是官方 `BrowserNavigationStatus.status`（BrowserPersistence.ts:10-14）。
type navState uint8

const (
	navEmpty navState = iota
	navLoading
	navKnown
	navUnknown
)

// navRequest 是官方 `BrowserTabState['request']`：一次导航请求 + 它的代数。
type navRequest struct {
	Revision int           `json:"revision"`
	Target   browserTarget `json:"target"`
}

// navStatus 是官方 `BrowserNavigationStatus`（带 revision 的那三档）。
type navStatus struct {
	State    navState `json:"status"`
	Revision int      `json:"revision,omitempty"`
}

// browserFailure 是官方 BrowserTabState.failure 的可序列化形状。
type browserFailure struct {
	Kind   string         `json:"kind"`
	Reason addressFailure `json:"reason"`
}

// browserTabState 是官方 `BrowserTabState`（BrowserPersistence.ts:17-24）。
type browserTabState struct {
	Entries    []browserTarget `json:"entries"`
	Index      int             `json:"index"`
	Request    *navRequest     `json:"request,omitempty"`
	Navigation navStatus       `json:"navigation"`
	Failure    *browserFailure `json:"failure,omitempty"`
}

// browserNavigation 是官方 `BrowserNavigation` 类。
type browserNavigation struct {
	value browserTabState
}

// newBrowserNavigation 造一个空历史（官方 `new BrowserNavigation()`）。
func newBrowserNavigation() browserNavigation {
	return browserNavigation{value: browserTabState{Index: -1}}
}

// currentTarget 是官方 `current()`（BrowserNavigation.ts:24-27 / BrowserPersistence.ts:37-40）。
//
// 注意它只认"应用自己知道的最后一条"，**不是**页面此刻的真实地址。
func currentTarget(state browserTabState) (browserTarget, bool) {
	if state.Index < 0 || state.Index >= len(state.Entries) {
		return browserTarget{}, false
	}
	return state.Entries[state.Index], true
}

// canGoBack 是官方 `canGoBack`。
func (n *browserNavigation) canGoBack() bool {
	return n.value.Navigation.State != navUnknown && n.value.Index > 0
}

// canGoForward 是官方 `canGoForward`。
func (n *browserNavigation) canGoForward() bool {
	state := n.value
	return state.Navigation.State != navUnknown &&
		state.Index >= 0 && state.Index < len(state.Entries)-1
}

// navigate 追加一个受控目标并丢掉它之后的前进分支。
func (n *browserNavigation) navigate(target browserTarget) *navRequest {
	end := n.value.Index + 1
	if end < 0 {
		end = 0
	}
	entries := make([]browserTarget, 0, end+1)
	entries = append(entries, n.value.Entries[:minInt(end, len(n.value.Entries))]...)
	entries = append(entries, target)
	if len(entries) > maxBrowserHistory {
		entries = entries[len(entries)-maxBrowserHistory:]
	}
	basis := n.value
	basis.Entries = entries
	basis.Index = len(entries) - 1
	return n.request(target, basis)
}

// back 选中前一条应用地址。
func (n *browserNavigation) back() *navRequest {
	if !n.canGoBack() {
		return nil
	}
	basis := n.value
	basis.Index--
	target := basis.Entries[basis.Index]
	return n.request(target, basis)
}

// forward 选中后一条应用地址。
func (n *browserNavigation) forward() *navRequest {
	if !n.canGoForward() {
		return nil
	}
	basis := n.value
	basis.Index++
	target := basis.Entries[basis.Index]
	return n.request(target, basis)
}

// reload 重新加载当前应用地址（不新增历史项）。
func (n *browserNavigation) reload() *navRequest {
	target, ok := currentTarget(n.value)
	if !ok {
		return nil
	}
	return n.request(target, n.value)
}

// frameLoaded 记录一次"渲染出来的那一帧"的加载完成。
//
// 官方只认**同一个 revision**：晚到的旧帧事件不能推进新请求的状态，
// 否则连续两次导航时会用第一次的 load 把第二次标成 known。
func (n *browserNavigation) frameLoaded(revision int) {
	navigation := n.value.Navigation
	if navigation.State == navEmpty || navigation.Revision != revision {
		return
	}
	switch navigation.State {
	case navLoading:
		n.value.Navigation = navStatus{State: navKnown, Revision: revision}
	case navKnown:
		n.value.Navigation = navStatus{State: navUnknown, Revision: revision}
	}
}

// started 记录一次**页面自己发起的**导航开始。
//
// 官方 ElectronWebViewImpl.ts:168-172 的 `did-start-navigation`（主框架）只做一件事：
// `loading: true`。它不新增应用历史项——`entries` 只在 `navigate()` 里追加
// （BrowserNavigation.ts:74-89）。本仓把"正在加载"寄存在请求的 revision 上，
// 所以这里开**新一代请求**（target 不变，不追加 entries）：页内点链接 / 重定向 /
// `location=` 因此也进 loading 态，而"晚到的旧 revision 被丢弃"这条判据
// （frameLoaded）原样保留——旧一代的 Done 拿不到新 revision。
//
// 没有应用历史项时什么都不做：那种状态（navEmpty）没有可观察的请求，
// 官方那一档是独立布尔，本仓没有它的落点。
func (n *browserNavigation) started() bool {
	target, ok := currentTarget(n.value)
	if !ok {
		return false
	}
	basis := n.value
	n.request(target, basis)
	return true
}

// commit 采纳引擎**已收下**的文档地址（官方 `did-navigate` → `address='observed'`）。
//
// 官方 `observeReady`（ElectronWebViewImpl.ts:203-232）在这一步把 `target` 换成
// 已提交的 URL（`title: element.getTitle() || parsed.target.title`），地址栏、
// 持久化 checkpoint 与标题都跟着它走；本仓 entries[index] 就是那个 target 的落点，
// 所以原地替换，**不新增历史项**（页面自己发起的导航在官方也不进应用历史）。
//
// 状态机不变：Committed 不是"加载完成"，known 仍由 frameLoaded 在 Done 时推进。
func (n *browserNavigation) commit(target browserTarget) bool {
	if _, ok := currentTarget(n.value); !ok {
		// 页面不是经应用导航打开的（没有应用历史项）：补一条，之后地址栏 / 标题 /
		// 持久化才有落点。没有 request 就没有 revision，这一档不推进状态机。
		n.value.Entries = []browserTarget{target}
		n.value.Index = 0
		return true
	}
	// 没看到 Started 就来了 Committed（载具只报后两档、或 Started 被上一帧丢掉）：
	// 补一次"开始"。否则 Done 回来时状态已是 known，frameLoaded 的第二跳会把它
	// 记成 unknown——而地址其实刚刚被我们读到过。
	if n.value.Navigation.State != navLoading {
		n.started()
	}
	entries := make([]browserTarget, len(n.value.Entries))
	copy(entries, n.value.Entries)
	entries[n.value.Index] = target
	n.value.Entries = entries
	return true
}

// request 是一条新请求的唯一出处（官方 BrowserNavigation.ts:140-155）。
func (n *browserNavigation) request(target browserTarget, basis browserTabState) *navRequest {
	revision := 1
	if n.value.Request != nil {
		revision = n.value.Request.Revision + 1
	}
	request := &navRequest{Revision: revision, Target: target}
	basis.Request = request
	basis.Navigation = navStatus{State: navLoading, Revision: revision}
	n.value = basis
	return request
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
