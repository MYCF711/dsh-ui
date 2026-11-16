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
