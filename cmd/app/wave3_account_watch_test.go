// wave3_account_watch_test.go —— 账号状态推送（官方 `account.watch`）的壳侧证据。
//
// 三条判据，每条都能被"删掉实现"弄红：
//
//	① **逐帧同值**：同一串状态变化，推送路径与轮询路径落进缓存的快照逐字段相等
//	               （判据不是"看起来一样"，是 `AccountSnapshot` 深比较）。
//	② **推送顶替轮询**：推送在线时，30s 那一格**不再问** `account.getState`，
//	               只做资料/余额的续期（接线前那一格也是 30s，节奏不变）。
//	③ **断开回落**：流断 / 宿主没这条流 / 连上却不发首帧 —— 轮询全部接回来，
//	               且失败表现（`accountFailed`）与接线前同一条判据。
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/plugins/dssettings"
)

// ── 假宿主 ──────────────────────────────────────────────────────────────

// accountWatchModes 是 account.watch 在假宿主上的四种表现。
const (
	watchStream         = "stream"        // 正常长流（默认）
	watchUnimplemented  = "unimplemented" // 老宿主：501 not_implemented
	watchSilent         = "silent"        // 开流但一帧不发（首帧承诺没兑现）
	watchStateErrorMode = "state-error"   // 让 account.getState 回 500
)

type accountWatchFixture struct {
	mu      sync.Mutex
	view    apiclient.AccountView
	profile *apiclient.AccountProfile
	balance *apiclient.AccountBalance
	mode    string
	frames  chan string
	// closeNow 掐断**当前**这条流（宿主重启 / 连接被掐断的真实形态）。
	closeNow chan struct{}

	watchHits, stateHits, profileHits, balanceHits int
	// watchClosed 在 account.watch 的 handler 返回时关一次
	// （= 客户端那边把这次订阅结束了：取消 / 首帧超时 / 流被关）。
	watchClosed     chan struct{}
	watchClosedOnce sync.Once
}

func newAccountWatchFixture(t *testing.T) (*accountWatchFixture, *httptest.Server) {
	t.Helper()
	fx := &accountWatchFixture{
		mode:        watchStream,
		frames:      make(chan string, 8),
		closeNow:    make(chan struct{}, 4),
		watchClosed: make(chan struct{}),
	}
	srv := httptest.NewServer(http.HandlerFunc(fx.serve))
	t.Cleanup(srv.Close)
	return fx, srv
}

func (fx *accountWatchFixture) markWatchClosed() {
	fx.watchClosedOnce.Do(func() { close(fx.watchClosed) })
}

func (fx *accountWatchFixture) setMode(mode string) {
	fx.mu.Lock()
	fx.mode = mode
	fx.mu.Unlock()
}

// setState 换一次"宿主眼里的账号状态"（含资料与余额）。
func (fx *accountWatchFixture) setState(view apiclient.AccountView, profile *apiclient.AccountProfile, balance *apiclient.AccountBalance) {
	fx.mu.Lock()
	fx.view, fx.profile, fx.balance = view, profile, balance
	fx.mu.Unlock()
}

func (fx *accountWatchFixture) snapshotState() (apiclient.AccountView, *apiclient.AccountProfile, *apiclient.AccountBalance) {
	fx.mu.Lock()
	defer fx.mu.Unlock()
	return fx.view, fx.profile, fx.balance
}

func (fx *accountWatchFixture) hits() (watch, state, profile, balance int) {
	fx.mu.Lock()
	defer fx.mu.Unlock()
	return fx.watchHits, fx.stateHits, fx.profileHits, fx.balanceHits
}

// push 把一帧推给客户端（官方 `{type,state}` 形状）。
func (fx *accountWatchFixture) push(t *testing.T, typ string, view apiclient.AccountView) {
	t.Helper()
	raw, err := json.Marshal(struct {
		Type  string                `json:"type"`
		State apiclient.AccountView `json:"state"`
	}{Type: typ, State: view})
	if err != nil {
		t.Fatalf("帧编码失败：%v", err)
	}
	select {
	case fx.frames <- string(raw):
	case <-time.After(3 * time.Second):
		t.Fatal("推送队列堵住了（客户端没在读）")
	}
}

// closeStream 让宿主把当前这条流关掉（客户端会看到 EOF）。
func (fx *accountWatchFixture) closeStream(t *testing.T) {
	t.Helper()
	select {
	case fx.closeNow <- struct{}{}:
	case <-time.After(3 * time.Second):
		t.Fatal("没有在跑的订阅可关")
	}
}

func (fx *accountWatchFixture) serve(w http.ResponseWriter, r *http.Request) {
	fx.mu.Lock()
	fx.mu.Unlock()
	switch r.URL.Path {
	case "/api/account.getState":
		fx.mu.Lock()
		fx.stateHits++
		mode, view := fx.mode, fx.view
		fx.mu.Unlock()
		if mode == watchStateErrorMode {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"code":"internal","message":"账号栈挂了","retryable":false}}`))
			return
		}
		raw, _ := json.Marshal(view)
		_, _ = w.Write(raw)
	case "/api/account.getProfile":
		fx.mu.Lock()
		fx.profileHits++
		profile := fx.profile
		fx.mu.Unlock()
		raw, _ := json.Marshal(struct {
			Profile *apiclient.AccountProfile `json:"profile"`
		}{Profile: profile})
		_, _ = w.Write(raw)
	case "/api/account.getBalance":
		fx.mu.Lock()
		fx.balanceHits++
		balance := fx.balance
		fx.mu.Unlock()
		raw, _ := json.Marshal(struct {
			Balance *apiclient.AccountBalance `json:"balance"`
		}{Balance: balance})
		_, _ = w.Write(raw)
	case "/api/account.watch":
		fx.mu.Lock()
		fx.watchHits++
		mode := fx.mode
		fx.mu.Unlock()
		defer fx.markWatchClosed()
		if mode == watchUnimplemented {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"account.watch 未接线","retryable":false}}`))
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		if flusher != nil {
			flusher.Flush()
		}
		if mode == watchSilent {
			<-r.Context().Done()
			return
		}
		for {
			select {
			case <-r.Context().Done():
				return
			case <-fx.closeNow:
				return
			case frame := <-fx.frames:
				_, _ = w.Write([]byte(frame + "\n"))
				if flusher != nil {
					flusher.Flush()
				}
			}
		}
	default:
		http.NotFound(w, r)
	}
}

// ── 小工具 ──────────────────────────────────────────────────────────────

func accountView(status string) apiclient.AccountView {
	return apiclient.AccountView{
		Status: status,
		Links:  apiclient.AccountLinks{UsageURL: "https://platform.test/usage", TopUpURL: "https://platform.test/topup"},
	}
}

func readyProfile(id, name string) *apiclient.AccountProfile {
	return &apiclient.AccountProfile{
		Status: "ready",
		Value:  &apiclient.AccountProfileValue{ID: &id, Name: &name, Contact: &name},
	}
}

func readyBalance(amount string) *apiclient.AccountBalance {
	return &apiclient.AccountBalance{
		Status: "ready",
		Value:  []apiclient.AccountWallet{{Currency: "CNY", Balance: amount}},
	}
}

// ── ① 逐帧同值 ──────────────────────────────────────────────────────────

// TestAccountWatchPushMatchesPollingFrameForFrame 是"不改可见行为"的主判据：
//
//	同一串状态变化（未登录 → 已登录 → 换账号 → 登出），
//	推送落库的快照与轮询落库的快照**逐字段相等**。
//
// 判据选 `reflect.DeepEqual` 而不是"几个字段看起来对"：口径是"逐帧同值"。
// 去掉推送接线后，推送侧会一直停在第 0 帧的快照 ⇒ 第 1 步就不等 ⇒ 必红。
func TestAccountWatchPushMatchesPollingFrameForFrame(t *testing.T) {
	fx, srv := newAccountWatchFixture(t)

	pollWiring := newWave3Wiring()
	pollModel := &model{cli: apiclient.New(srv.URL)}

	pushWiring := newWave3Wiring()
	pushModel := &model{cli: apiclient.New(srv.URL)}
	pushWiring.startAccountWatch(pushModel)
	defer pushWiring.stopAccountWatch()

	steps := []struct {
		name    string
		view    apiclient.AccountView
		profile *apiclient.AccountProfile
		balance *apiclient.AccountBalance
	}{
		{"初始未登录", accountView("signed-out"), nil, nil},
		{"登录成功", accountView(apiclient.AccountCredentialStored), readyProfile("u-1", "用户甲"), readyBalance("8.01")},
		{"换账号（凭据变更）", accountView(apiclient.AccountCredentialStored), readyProfile("u-2", "用户乙"), readyBalance("12.00")},
		{"登出", accountView("signed-out"), nil, nil},
	}

	var polled, pushed []apiclient.AccountSnapshot
	for _, step := range steps {
		fx.setState(step.view, step.profile, step.balance)
		// 轮询路径：就是接线前那条 fetchAccount。
		pollWiring.fetchAccount(pollModel)
		want := pollWiring.accountDetailsSnapshot()

		// 推送路径：真 HTTP 流，一帧 `{type,state}`。
		typ := apiclient.AccountUpdateSnapshot
		if step.name == "换账号（凭据变更）" || step.name == "登出" {
			typ = apiclient.AccountUpdateCredentialsChanged
		}
		fx.push(t, typ, step.view)
		waitUntil(t, "推送快照与轮询快照一致（"+step.name+"）", func() bool {
			return reflect.DeepEqual(pushWiring.accountDetailsSnapshot(), want)
		})

		polled = append(polled, want)
		pushed = append(pushed, pushWiring.accountDetailsSnapshot())
	}

	if len(polled) != len(steps) || len(pushed) != len(steps) {
		t.Fatalf("快照数不对：polled=%d pushed=%d", len(polled), len(pushed))
	}
	for i := range steps {
		if !reflect.DeepEqual(polled[i], pushed[i]) {
			t.Fatalf("%s：推送与轮询不等\n推送 = %+v\n轮询 = %+v", steps[i].name, pushed[i], polled[i])
		}
	}
	// 序列本身必须真的变化（否则上面的"相等"是空等出来的）。
	if reflect.DeepEqual(polled[0], polled[1]) {
		t.Fatal("未登录与已登录的快照相同 —— 夹具没有真的换状态，等价性证明无效")
	}
	if reflect.DeepEqual(polled[1], polled[2]) {
		t.Fatal("换账号前后快照相同 —— 夹具没换上第二份资料，等价性证明无效")
	}
	// 推送在线：状态确实是推来的，不是轮询补的。
	if !pushWiring.watchOnline() {
		t.Fatal("推送应当在线（watchLive）")
	}
}

// ── ② 推送顶替轮询 ──────────────────────────────────────────────────────

// TestAccountWatchReplacesGetStateWhileLive 钉住"轮询换成推送"这件事本身：
//
//	推送在线时，30s 那一格**不许**再问 `account.getState`，只做资料/余额续期
//	（续期是接线前就有的节奏，不能少）；推送断了，`getState` 必须回来。
func TestAccountWatchReplacesGetStateWhileLive(t *testing.T) {
	fx, srv := newAccountWatchFixture(t)
	w := newWave3Wiring()
	// 设置面板是真壳必装的消费者；没有它 `refreshDue` 里那一格根本不成立。
	m := &model{cli: apiclient.New(srv.URL), settings: &dssettings.Plugin{}}

	fx.setState(accountView(apiclient.AccountCredentialStored), readyProfile("u-1", "用户甲"), readyBalance("8.01"))
	w.startAccountWatch(m)
	defer w.stopAccountWatch()

	// 首帧把状态推下来（宿主订阅即发当前状态）。
	fx.push(t, apiclient.AccountUpdateSnapshot, accountView(apiclient.AccountCredentialStored))
	waitUntil(t, "首帧落地", func() bool {
		return w.accountDetailsSnapshot().Profile != nil
	})
	_, stateHitsWhileLive, profileHitsWhileLive, _ := fx.hits()
	if stateHitsWhileLive != 0 {
		t.Fatalf("推送在线时不该问 account.getState，实得 %d 次", stateHitsWhileLive)
	}

	// 30s 那一格该到点了：仍然**不问状态**，但资料/余额要续期（节奏不变）。
	w.mu.Lock()
	w.accountAt = time.Now().Add(-accountPollEvery - time.Second)
	w.mu.Unlock()
	if accountDue, _ := w.refreshDue(m, ""); !accountDue {
		t.Fatal("30s 详情续期没到点 —— 资料/余额的刷新节奏被改慢了")
	}
	w.fetchAccount(m)
	_, stateHitsAfter, profileHitsAfter, balanceHitsAfter := fx.hits()
	if stateHitsAfter != 0 {
		t.Fatalf("推送在线时续期也不许问 account.getState，实得 %d 次", stateHitsAfter)
	}
	if profileHitsAfter <= profileHitsWhileLive || balanceHitsAfter == 0 {
		t.Fatalf("资料/余额没有续期：profile=%d→%d balance=%d", profileHitsWhileLive, profileHitsAfter, balanceHitsAfter)
	}

	// 断流：宿主关掉连接（重启/被掐断）。
	fx.closeStream(t)
	waitUntil(t, "断流后回落（watchLive=false）", func() bool {
		online, _, _ := w.accountWatchState()
		return !online
	})

	// 回落之后：状态必须重新由 getState 负责，且能刷新到新值。
	_, beforeFallback, _, _ := fx.hits()
	fx.setState(accountView("signed-out"), nil, nil)
	w.mu.Lock()
	w.accountAt = time.Now().Add(-accountPollEvery - time.Second)
	w.mu.Unlock()
	if accountDue, _ := w.refreshDue(m, ""); !accountDue {
		t.Fatal("推送断开后轮询必须到点（否则界面永远不刷新）")
	}
	w.fetchAccount(m)
	_, afterFallback, _, _ := fx.hits()
	if afterFallback <= beforeFallback {
		t.Fatal("回落之后没有重新问 account.getState —— 推送断了就再也不刷新了")
	}
	snapshot := w.accountDetailsSnapshot()
	if snapshot.View == nil || snapshot.View.SignedIn() {
		t.Fatalf("回落后的轮询没拿到新状态：%+v", snapshot.View)
	}
}

// ── ③ 三种坏法都回落 ────────────────────────────────────────────────────

// TestAccountWatchUnimplementedHostStaysOnPolling：老宿主上必须与接线前逐帧同值（纯轮询）。
func TestAccountWatchUnimplementedHostStaysOnPolling(t *testing.T) {
	fx, srv := newAccountWatchFixture(t)
	fx.setMode(watchUnimplemented)
	w := newWave3Wiring()
	// 设置面板是真壳必装的消费者；没有它 `refreshDue` 里那一格根本不成立。
	m := &model{cli: apiclient.New(srv.URL), settings: &dssettings.Plugin{}}

	fx.setState(accountView(apiclient.AccountCredentialStored), readyProfile("u-1", "用户甲"), readyBalance("8.01"))
	w.startAccountWatch(m)
	defer w.stopAccountWatch()

	waitUntil(t, "判出宿主没有这条流", func() bool {
		_, unusable, _ := w.accountWatchState()
		return unusable
	})
	if w.watchOnline() {
		t.Fatal("宿主没有这条流时 watchLive 必须是 false")
	}
	// 轮询照旧：到点 → 问状态 → 拉详情。
	if accountDue, _ := w.refreshDue(m, ""); !accountDue {
		t.Fatal("老宿主上轮询必须照旧（首帧之前 accountAt 是零值，应当立即到点）")
	}
	w.fetchAccount(m)
	if _, stateHits, _, _ := fx.hits(); stateHits != 1 {
		t.Fatalf("老宿主上 fetchAccount 应当问一次 account.getState，实得 %d 次", stateHits)
	}
	if snapshot := w.accountDetailsSnapshot(); !snapshot.Known || !snapshot.View.SignedIn() || snapshot.Profile == nil {
		t.Fatalf("老宿主上的快照不对：%+v", snapshot)
	}
}

// TestAccountWatchFirstFrameTimeoutFallsBackToPolling："连上但一帧不给"也要回落。
//
// 不设这层，watchLive 会一直是 true、轮询被抑制，界面从此不再刷新 ——
// 正是本单要防的"永远不刷新"。
func TestAccountWatchFirstFrameTimeoutFallsBackToPolling(t *testing.T) {
	fx, srv := newAccountWatchFixture(t)
	fx.setMode(watchSilent)
	w := newWave3Wiring()
	w.watchFirstFrame = 80 * time.Millisecond // 把 6s 压到毫秒级
	w.watchRetry = time.Hour                  // 这一轮不要重连（测的是"这一次订阅结束"）
	m := &model{cli: apiclient.New(srv.URL), settings: &dssettings.Plugin{}}

	w.startAccountWatch(m)
	defer w.stopAccountWatch()

	select {
	case <-fx.watchClosed:
	case <-time.After(3 * time.Second):
		t.Fatal("首帧超时没有把这次订阅结束掉（客户端还挂在一条不说话的流上）")
	}
	if w.watchOnline() {
		t.Fatal("一帧没给的订阅不许算在线（否则轮询被抑制、界面永不刷新）")
	}
	// 轮询接着干活。
	fx.setState(accountView("signed-out"), nil, nil)
	fx.setMode(watchStream)
	if accountDue, _ := w.refreshDue(m, ""); !accountDue {
		t.Fatal("首帧超时后轮询必须到点")
	}
	w.fetchAccount(m)
	if _, stateHits, _, _ := fx.hits(); stateHits != 1 {
		t.Fatalf("首帧超时后应当回到轮询，实得 stateHits=%d", stateHits)
	}
	if snapshot := w.accountDetailsSnapshot(); !snapshot.Known {
		t.Fatalf("回落后的轮询没落地：%+v", snapshot)
	}
}

// ── 代次与失败表现 ──────────────────────────────────────────────────────

// TestAccountWatchDropsStaleGenerationFrames：旧代次的帧不许盖新值
// （官方 `frame.generation !== accountGeneration` 那条判据在本仓的等价物）。
func TestAccountWatchDropsStaleGenerationFrames(t *testing.T) {
	w := newWave3Wiring()
	m := &model{}

	first := w.beginWatch()  // 第 1 代
	second := w.beginWatch() // 第 2 代（重连/重开）
	if second != first+1 {
		t.Fatalf("代次应当每开一次 +1：%d → %d", first, second)
	}

	w.applyAccountUpdate(m, first, apiclient.AccountUpdate{
		Type:  apiclient.AccountUpdateSnapshot,
		State: accountView(apiclient.AccountCredentialStored),
	})
	if w.accountDetailsSnapshot().Known {
		t.Fatal("旧代次的帧被应用了（会盖掉新订阅的真实状态）")
	}

	w.applyAccountUpdate(m, second, apiclient.AccountUpdate{
		Type:  apiclient.AccountUpdateSnapshot,
		State: accountView(apiclient.AccountCredentialStored),
	})
	snapshot := w.accountDetailsSnapshot()
	if !snapshot.Known || !snapshot.View.SignedIn() {
		t.Fatalf("当前代次的帧必须应用：%+v", snapshot)
	}
	if !w.watchOnline() {
		t.Fatal("应用过一帧就应当算在线")
	}
}

// TestAccountWatchDoesNotFakeFailure：流断开**不是**账号失败。
//
// 接线前，`accountFailed` 只由 `getState` 的真失败决定（能力缺失按未知）。
// 推送断开只是"换回轮询"，不许自己翻成 failed —— 那会让失败表现与接线前不同
// （通知控制器读到 Failed 会停轮询、界面会画错误态）。
func TestAccountWatchDoesNotFakeFailure(t *testing.T) {
	fx, srv := newAccountWatchFixture(t)
	w := newWave3Wiring()
	// 设置面板是真壳必装的消费者；没有它 `refreshDue` 里那一格根本不成立。
	m := &model{cli: apiclient.New(srv.URL), settings: &dssettings.Plugin{}}

	fx.setState(accountView(apiclient.AccountCredentialStored), readyProfile("u-1", "用户甲"), readyBalance("8.01"))
	w.startAccountWatch(m)
	defer w.stopAccountWatch()
	fx.push(t, apiclient.AccountUpdateSnapshot, accountView(apiclient.AccountCredentialStored))
	waitUntil(t, "首帧落地", func() bool { return w.accountDetailsSnapshot().Profile != nil })

	// 断流 → 回落，但**不许**把 failed 翻起来。
	fx.closeStream(t)
	waitUntil(t, "断流后回落", func() bool {
		online, _, _ := w.accountWatchState()
		return !online
	})
	if snapshot := w.accountDetailsSnapshot(); snapshot.Failed {
		t.Fatalf("流断开不许被记成账号失败：%+v", snapshot)
	}

	// 真的问不到时才 failed（与接线前同一条判据）。
	fx.setMode(watchStateErrorMode)
	w.fetchAccount(m)
	if snapshot := w.accountDetailsSnapshot(); !snapshot.Failed {
		t.Fatalf("getState 真失败时 Failed 必须为真：%+v", snapshot)
	}
}

// TestAccountWatchFatalPredicate：哪些错误该停止重连。
func TestAccountWatchFatalPredicate(t *testing.T) {
	fatal := []error{
		&apiclient.APIError{Code: "not_implemented"},
		&apiclient.APIError{Code: "unsupported"},
		&apiclient.APIError{Code: "not_found"},
		&apiclient.APIError{Code: "method_not_found"},
		&apiclient.APIError{Code: "bad_request"},
	}
	for _, err := range fatal {
		if !accountWatchFatal(err) {
			t.Fatalf("%v 应当判为致命（重连不会好）", err)
		}
	}
	soft := []error{
		nil,
		&apiclient.APIError{Code: "internal"},
		&apiclient.APIError{Code: "unauthorized"},
		errAccountWatchNoFrame,
	}
	for _, err := range soft {
		if accountWatchFatal(err) {
			t.Fatalf("%v 不该判为致命（下一次重连可能就好）", err)
		}
	}
}
