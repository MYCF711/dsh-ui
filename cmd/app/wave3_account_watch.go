// wave3_account_watch.go —— 账号状态的**推送**通道（官方 `account.watch` 的壳侧消费方）。
//
// # 接线前是什么
//
// `cmd/app/wave3_wiring.go` 的 30s 轮询：每 30s 一次 `account.getState`，
// 已登录再拉 `account.getProfile` / `account.getBalance`。头注当时写着
// "宿主没有推送通道（`account.watch` 是流式，本波不引），所以只能问"。
// **那句话现在不再成立**：宿主侧早有 `serveAccountWatch`，本文件就是它的消费方。
//
// # 接线后与接线前的**唯一**差异：状态从哪儿来
//
//	             账号状态 (status/links/attempt)        资料/余额 (profile/balance)
//	接线前        30s 轮询 account.getState              同一次轮询的尾巴（30s）
//	接线后        watch 帧，到达即应用                    ① 每帧之后（官方 index.ts:146
//	                                                        也在这里 void refresh()）
//	                                                     ② 30s 的详情续期（节奏不变）
//
// 也就是说：轮询那一格**没有被删掉**，它换成"详情续期"；状态的来源换成推送。
// 为什么详情续期必须留着：官方自己也有 30s 之外没有的刷法——它只在帧和显式
// 动作时刷新（`index.ts:69-98` 的 `refresh` 只被帧和用户动作触发）。若我们
// 照抄"只跟帧刷"，余额/资料的刷新就比现在**慢**（今天 30s 一次）⇒ 那是让用户
// 付代价。所以两条都保留，任何一条都不比现在差。
//
// # 落库路径与轮询**同一份代码**（"逐帧同值"的机制保证）
//
// 推送帧走 `applyAccountView` —— 与 `fetchAccount` 的 `getState` 结果用的是同一个
// 函数；资料/余额走 `fetchAccountDetails` —— 也是同一份。同一串状态变化，无论
// 从 `getState` 回来还是从 watch 推来，写进缓存的值只可能一样，因为**只有一处写**。
//
// # 回落
//
// 三种坏法都退回轮询，且退回后轮询与会话前完全一样（含 30s 节奏、6s 超时、
// 能力缺失按未知/真失败按 failed 的判据）：
//
//  1. 流报错 / 宿主关流 / 响应不是 ndjson / 帧非法 —— `AccountWatch` 回 error；
//  2. 连上了却一帧不给（宿主承诺首帧是当前状态，见 `internal/account/account.go:951-991`）
//     —— `accountWatchFirstFrame` 到点自己取消，按失败处理；
//  3. 宿主根本没有这条流（`not_implemented` 等）—— 判为不可用，**不再重连**，
//     轮询照旧。这一条与接线前逐帧同值（老宿主上就是纯轮询）。
package main

import (
	"context"
	"errors"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

const (
	// accountWatchRetry 是推送断开后的首次重连间隔（与轮询节奏同一格）。
	//
	// 不设更短：重连本身不发诊断信息，而断开期间轮询已经在兜底，界面不会停在
	// 旧值上 —— 没有任何理由比"下一次轮询"更急。
	accountWatchRetry = 30 * time.Second
	// accountWatchRetryMax 是连续失败时的重连上限（退避翻倍到它为止）。
	//
	// 为什么需要退避：宿主每次开会都失败（例如 500）时，固定 30s 会变成
	// "轮询 30s + 重连 30s" 的双份请求。翻倍之后最坏 5 分钟一次，不刷屏。
	accountWatchRetryMax = 5 * time.Minute
	// accountWatchFirstFrame 是"连上了但一帧都不给"的容忍时长的**默认值**
	// （实际值挂在 wave3Wiring.watchFirstFrame 上，测试要把它压到毫秒级）。
	//
	// 宿主订阅时立刻把当前状态放进 pending（`internal/account/account.go:951-991`），
	// 所以首帧是**承诺**。到点还没有 = 这条流坏了；不设这层，watchLive 会一直是
	// true、轮询被抑制，界面从此再也不刷新 —— 那正是本单要防的"永远不刷新"。
	accountWatchFirstFrame = wave3FetchTimeout
)

// startAccountWatch 起账号状态的推送订阅（壳装配完成后调一次；cli 为 nil 是空操作）。
//
// 幂等：重复调用不会起第二条订阅。壳退出时用 `stopAccountWatch` 收尾。
func (w *wave3Wiring) startAccountWatch(m *model) {
	if m == nil || m.cli == nil {
		return
	}
	w.mu.Lock()
	if w.watchCancel != nil {
		w.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.watchCancel = cancel
	w.mu.Unlock()
	go w.accountWatchLoop(ctx, m)
}

// stopAccountWatch 停订阅（壳退出；测试收尾也用它）。
func (w *wave3Wiring) stopAccountWatch() {
	w.mu.Lock()
	cancel := w.watchCancel
	w.watchCancel = nil
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// watchOnline 报告推送是否在线（refreshDue / fetchAccount 用它决定要不要走 getState）。
func (w *wave3Wiring) watchOnline() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.watchLive
}

// accountWatchState 交回订阅的三个簿记位（测试与日志用）。
func (w *wave3Wiring) accountWatchState() (online, unusable bool, generation int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.watchLive, w.watchUnusable, w.watchGen
}

// accountWatchLoop 是订阅的主循环：开流 → 读帧 → 断了按退避重连。
func (w *wave3Wiring) accountWatchLoop(ctx context.Context, m *model) {
	retry := w.retryInterval()
	for ctx.Err() == nil {
		frames, err := w.watchOnce(ctx, m)
		// 先放下"在线"，再判断要不要重连：轮询必须能立刻接手（这就是回落）。
		w.markWatchDown()
		if ctx.Err() != nil {
			return
		}
		if accountWatchFatal(err) {
			// 宿主没有这条流 / 参数错：重连也不会好。停在"不可用"，轮询照旧。
			w.mu.Lock()
			w.watchUnusable = true
			w.mu.Unlock()
			return
		}
		if frames > 0 {
			retry = w.retryInterval() // 这一代是"读到过帧才断的"，按首次间隔重来
		} else if retry < accountWatchRetryMax {
			retry *= 2
			if retry > accountWatchRetryMax {
				retry = accountWatchRetryMax
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(retry):
		}
	}
}

// firstFrameTimeout 是首帧容忍时长（零值回落到默认常量，便于 `wave3Wiring{}`
// 这类字面量在测试里也能安全工作）。
func (w *wave3Wiring) firstFrameTimeout() time.Duration {
	if w.watchFirstFrame > 0 {
		return w.watchFirstFrame
	}
	return accountWatchFirstFrame
}

// retryInterval 是首次重连间隔（同上，零值回落默认）。
func (w *wave3Wiring) retryInterval() time.Duration {
	if w.watchRetry > 0 {
		return w.watchRetry
	}
	return accountWatchRetry
}

// watchOnce 开一次订阅并读到它结束。返回本次订阅应用过的帧数。
func (w *wave3Wiring) watchOnce(parent context.Context, m *model) (int, error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	generation := w.beginWatch()
	// 首帧超时：**必须**在拿到第一帧后停表，否则长流的正常空闲期会被误杀。
	firstFrame := time.AfterFunc(w.firstFrameTimeout(), cancel)
	defer firstFrame.Stop()

	frames := 0
	err := m.cli.AccountWatch(ctx, func(update apiclient.AccountUpdate) error {
		if frames == 0 {
			firstFrame.Stop()
		}
		frames++
		w.applyAccountUpdate(m, generation, update)
		return nil
	})
	if err == nil && frames == 0 && parent.Err() == nil {
		// 连上却一帧没给就结束：宿主没兑现首帧承诺，按失败处理（回落轮询）。
		return 0, errAccountWatchNoFrame
	}
	return frames, err
}

// beginWatch 开一代新订阅：代次 +1，旧代次之后的帧全部作废。
func (w *wave3Wiring) beginWatch() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.watchGen++
	return w.watchGen
}

// markWatchDown 把"在线"放下（轮询接手）。
func (w *wave3Wiring) markWatchDown() {
	w.mu.Lock()
	w.watchLive = false
	w.mu.Unlock()
}

// applyAccountUpdate 把一帧推送折进缓存。
//
// 落库只有一处写（`applyAccountView`），所以与轮询**逐帧同值**。
// 官方 `index.ts:126-146` 的动作顺序（判凭据变更 → 应用 state → 重读详情）
// 在这里是：先判代次（旧代次的帧不许盖新值）→ 应用 state → 已登录则重读详情。
func (w *wave3Wiring) applyAccountUpdate(m *model, generation int, update apiclient.AccountUpdate) {
	w.mu.Lock()
	stale := generation != w.watchGen
	w.mu.Unlock()
	if stale {
		return
	}
	// 与本地登录/登出动作、与轮询串行：三者都写同一份缓存，必须同一把锁。
	w.accountOpMu.Lock()
	defer w.accountOpMu.Unlock()

	w.mu.Lock()
	stale = generation != w.watchGen
	w.mu.Unlock()
	if stale {
		return
	}

	state := update.State
	// credentials-changed 与 snapshot 落到同一处：`applyAccountView` 在登出时会清掉
	// 资料/余额，在登录时会重置失败位 —— 这就是官方 `reset` 在本仓的等价物
	// （本仓的通知控制器没有单独的 reset 入口，它读的是 Failed/SignedIn 两格）。
	w.applyAccountView(&state)
	w.mu.Lock()
	w.watchLive = true
	w.mu.Unlock()
	if m != nil {
		m.inv.Invalidate()
	}
	if !state.SignedIn() {
		return
	}
	// 官方每帧 `void refresh()`（`index.ts:146`）：凭据/身份可能刚变，资料与余额
	// 不能等下一个 30s。这一条是**额外**的刷新点，不会让任何刷新的时机变慢。
	w.refreshAccountDetails(m)
}

// refreshAccountDetails 立即重读资料/余额（调用方必须持 accountOpMu）。
func (w *wave3Wiring) refreshAccountDetails(m *model) {
	if m == nil || m.cli == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), wave3FetchTimeout)
	defer cancel()
	w.fetchAccountDetails(m, ctx)
}

// accountWatchFatal 报告"重连也不会好"的错误：宿主没这条流（not_implemented /
// unsupported / not_found / method_not_found）或参数错（bad_request）。
//
// 判据与 `isCapabilityMissing` 同一套 **加上** bad_request：参数错也是重连不会好的。
func accountWatchFatal(err error) bool {
	if err == nil {
		return false
	}
	if isCapabilityMissing(err) {
		return true
	}
	var apiErr *apiclient.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == "bad_request"
	}
	return false
}

// errAccountWatchNoFrame 是"连上却一帧不给"的错误（非致命：下一轮重连）。
var errAccountWatchNoFrame = errors.New("account.watch 首帧超时：宿主开了流却没发当前状态")
