package main

// frames.go —— 主区 transcript 的**数据面**：每条会话的原始 Transport journal 帧。
//
// # 为什么单独留一份原始帧
//
// 宿主契约（`DSH-GO/docs/transport-contract.md` §恢复）写得很清楚：
//
//	`session.load` 是**首屏视图投影**，同时返回 journalEpoch/lastSeq；
//	`session.history` 返回同一 journal 的**原始 frames:[]ServerFrame**，
//	 不是投影后的 messages。
//
// 官方主区 transcript 的投影器（`plugins/dsconversation/official_transcript_projection.go`）
// 消费的是 v4 原始事件：`user/message` 的 `surfaceOp` / `source.kind`、
// `turn/end` 的 `reason.kind`、`llm/retry` 的 `attempt/maxRetries`……
// 这些字段**只有原始帧才有**。拿 `session.load` 的 messages 反推，等于把
// "steering / turn-trigger / manual-compaction / question-reply" 这几种节点
// 全部塌成"一条普通消息"——界面上看不出报错，只是少了几种行。
//
// # 三份水位，各管各的
//
//	seqs[id]      每会话已消费到哪一段（WS 订阅用，跨切会话存活）
//	epochs[id]    journal 世代（订阅时带上，宿主换代就发 session/reset）
//	frames[id]    原始帧本身（本文件；主区 transcript 的数据面）
//
// 三者**必须同时清**：`session/reset` 与 history 的 409 `stale_epoch` 是同
// 一件事的两个面（见 onJournalResetLocked 的注释）。
//
// # 不丢帧、不重复
//
// journal 是 append-only，所以：
//   - 同 seq 再来一次只能是**重放**，丢弃（不重复）；
//   - seq 乱序（history 补洞补的是中间那段）按 seq **插入**，不按到达顺序追加
//     （不丢帧，也不把行序搞乱——行序就是 journal 顺序）；
//   - 世代不同 → 整段丢弃重建（旧 seq 属于另一个序号域）。
//
// # 向前分页（loadOlder 五态）
//
// 契约里 `session.history` 只有 **`sinceSeq`（开区间下界，向后取）**，没有
// `beforeSeq`。所以"加载更早"不是调一个新方法，而是用已知水位**算窗口**：
//
//	首屏基线  = 尾窗 [lastSeq-frameHistoryChunk+1, lastSeq]（`refreshFrames`）
//	加载更早  = 前窗 [earliest-1-chunk+1, earliest-1]（`loadOlderFrames`）
//
// 两个窗口都只用 `session.history` 的既有语义（`seq > sinceSeq` 的前 limit 条），
// **不重拉整页**：每次点击只多一页，拉过的帧按 seq 去重（重放帧自然被丢弃）。
//
// 为什么基线取尾窗而不是从 0 开始：从 0 开始拿到的是**最老**的一页，长会话上
// 用户打开看到的是几十轮之前的内容；而且那样 `HasMore` 恒为 false（前面没有更早的），
// 分页按钮永远画不出来。
//
// journalEpoch 与分页的协同（三条）：
//   - 基线请求**不带**世代（0 = "我还没有基线，宿主不用核"）；
//   - 向前分页请求**必须带**世代（补洞跨了几秒，期间宿主完全可能换代）；
//   - 回包世代与本地不一致 → 整段丢弃重建基线（旧 seq 属于另一个序号域）。
//
// 宿主对两种 409 都用 `stale_epoch`（`server.go` handleSessionHistory）：
// journal 换代、或 `sinceSeq` 之前的历史已滑出内存 journal。后者**就是
// "没有更早的了"**（见 `journalEvictedError`），不能当成失败去重建——重建会把
// 尾窗再拉一遍，然后同一次点击又能再点，用户看到的是"点了永远在转圈"。

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// frameHistoryChunk 是 `session.history` 单次拉多少帧。
//
// 与 `apiclient.fillChunk` 同量级：宿主单次返回有上限（journal capacity），
// 缺口比它大时必须分批，否则中间那段永远补不回来——界面上还看不出少了东西。
const frameHistoryChunk = 200

// frameHistoryTimeout 是**基线**拉取的上限（不是补洞：补洞跟 WS 共用请求）。
//
// 会话打开时在后台拉，用户此刻已经能看见 load 投影出来的消息；这里超时只
// 影响 transcript 什么时候接上，不该把界面卡住。
const frameHistoryTimeout = 10 * time.Second

// frameJournal 是一条会话的原始 journal 帧（按 seq 升序、同 seq 去重）。
//
// 零值可用；所有方法都**不是**并发安全的——调用方必须持有 m.mu。
type frameJournal struct {
	frames []apiclient.Frame
	// epoch 是这批帧所属的 journal 世代（0 = 还没有基线）。
	epoch int64
	// lastSeq 是已并入的最高 seq（0 = 还没有）。只增不减，除非 reset。
	lastSeq int
	// paging 是「加载更早」五态的状态（基线是否试过 / 还有没有更多 / 在拉 / 失败原因）。
	paging framePaging
}

// framePaging 是一条会话 transcript 的**向前分页状态**（loadOlder 五态的输入）。
//
// 与 `pluginapi.HistoryPaging` 字段一一对应；分开定义是为了让数据面不 import
// 插件契约（快照那一层做一次拷贝，见 `snapshotHistoryPagingLocked`）。
type framePaging struct {
	// Baseline 报告首屏基线是否已经**试过**（成功或失败都算）。
	Baseline bool
	// HasMore 报告还有更早的帧没拉（false = 已到可加载的最早处）。
	HasMore bool
	// Partial 报告基线是**截断**的（首屏那一页之前还有帧）。**粘性**：
	// 全部拉完之后仍为 true，那一行才会显示「已到会话开头」而不是整行消失。
	Partial bool
	// Loading 报告正在拉一页。
	Loading bool
	// Error 是最近一次失败原因（空 = 没有失败）。
	Error string
}

// reset 丢弃全部帧与水位（journal 换代 / 切会话时调用）。
func (j *frameJournal) reset() {
	j.frames = j.frames[:0]
	j.epoch = 0
	j.lastSeq = 0
	j.paging = framePaging{}
}

// earliestSeq 是当前已加载的**最早** seq（0 = 还没有帧）。
func (j *frameJournal) earliestSeq() int {
	if len(j.frames) == 0 {
		return 0
	}
	return j.frames[0].Seq
}

// insert 按 seq 有序并入一条原始帧，返回是否真的新增。
//
// seq <= 0 的控制帧（pong / session/reset）不是内容帧：它们没有序号，
// 进不了"按 seq 去重"的模型，投影器也不消费它们。
func (j *frameJournal) insert(frame apiclient.Frame) bool {
	if frame.Seq <= 0 {
		return false
	}
	at := sort.Search(len(j.frames), func(i int) bool { return j.frames[i].Seq >= frame.Seq })
	if at < len(j.frames) && j.frames[at].Seq == frame.Seq {
		// 已经消费过：journal 是 append-only，重复帧只能是重放。
		return false
	}
	j.frames = append(j.frames, apiclient.Frame{})
	copy(j.frames[at+1:], j.frames[at:])
	j.frames[at] = frame
	if frame.Seq > j.lastSeq {
		j.lastSeq = frame.Seq
	}
	return true
}

// insertAll 批量并入（history 基线 / 补洞），返回新增条数。
//
// 为什么先 append 再排序而不是逐条 insert：基线可能有上千帧，逐条插入是
// O(n²) 的搬移；一次性排序 + 去重是 O(n log n)。
//
// **去重保留既有那条**（稳定排序 + 先出现的赢）：既有帧可能已经被消费过，
// 用重放帧替换它没有任何好处，反而会让同一行换一个时间戳。
func (j *frameJournal) insertAll(frames []apiclient.Frame) int {
	if len(frames) == 0 {
		return 0
	}
	before := len(j.frames)
	j.frames = append(j.frames, frames...)
	sort.SliceStable(j.frames, func(a, b int) bool { return j.frames[a].Seq < j.frames[b].Seq })
	out := j.frames[:0]
	last := 0
	for _, frame := range j.frames {
		if frame.Seq <= 0 {
			continue
		}
		if frame.Seq == last {
			continue // 同 seq 重放
		}
		last = frame.Seq
		out = append(out, frame)
	}
	j.frames = out
	if n := len(j.frames); n > 0 && j.frames[n-1].Seq > j.lastSeq {
		j.lastSeq = j.frames[n-1].Seq
	}
	if added := len(j.frames) - before; added > 0 {
		return added
	}
	return 0
}

// snapshot 返回帧的**独立副本**（绘制期在锁外读，必须拷贝）。
func (j *frameJournal) snapshot() []apiclient.Frame {
	if len(j.frames) == 0 {
		return nil
	}
	return append([]apiclient.Frame(nil), j.frames...)
}

// frameJournalLocked 取（或建）一条会话的帧日志。调用方必须持有 m.mu。
func (m *model) frameJournalLocked(id string) *frameJournal {
	if m.frames == nil {
		m.frames = map[string]*frameJournal{}
	}
	journal := m.frames[id]
	if journal == nil {
		journal = &frameJournal{}
		m.frames[id] = journal
	}
	return journal
}

// appendFrameLocked 把一条 WS 推送的原始帧并入日志。调用方必须持有 m.mu。
//
// 只在 `applyEventAt` 里调：history 补洞走 `applyHistoryFrame` → `applyEvent`
// → 这里，**同一条路径**，所以补回来的帧和 WS 推的帧在日志里完全同形。
func (m *model) appendFrameLocked(id string, frame apiclient.Frame) bool {
	if id == "" || frame.Seq <= 0 {
		return false
	}
	return m.frameJournalLocked(id).insert(frame)
}

// snapshotFramesLocked 取当前会话这一帧要交给插件的原始帧 + 世代。
// 调用方必须持有 m.mu。
func (m *model) snapshotFramesLocked(id string) ([]apiclient.Frame, int64) {
	journal := m.frames[id]
	if journal == nil {
		return nil, 0
	}
	return journal.snapshot(), journal.epoch
}

// snapshotHistoryPagingLocked 取当前会话这一帧的**向前分页状态**。
// 调用方必须持有 m.mu。
//
// 没有帧日志（老宿主 / 还没开过会话）= 零值 = 界面不画那一行。
func (m *model) snapshotHistoryPagingLocked(id string) framePaging {
	if journal := m.frames[id]; journal != nil {
		return journal.paging
	}
	return framePaging{}
}

// frameTailWindow 算**尾窗**：以 anchor 为最高 seq，向前取 size 帧。
//
// anchor <= 0（水位未知）时退化成"从 0 开始的一页"——那不是最优窗口，
// 但比不画好；调用方在 anchor 未知时应当先探一次 lastSeq。
func frameTailWindow(anchor, size int) (sinceSeq, limit int) {
	if anchor <= 0 || size <= 0 {
		return 0, size
	}
	sinceSeq = anchor - size
	if sinceSeq < 0 {
		sinceSeq = 0
	}
	limit = anchor - sinceSeq
	if limit <= 0 {
		limit = size
	}
	return sinceSeq, limit
}

// frameOlderWindow 算**前窗**：在 earliest 之前紧邻的 size 帧。
//
// 返回 (0,0) 表示"没有更早的了"（earliest 已经是第 1 帧）。
func frameOlderWindow(earliest, size int) (sinceSeq, limit int) {
	if earliest <= 1 || size <= 0 {
		return 0, 0
	}
	sinceSeq = earliest - 1 - size
	if sinceSeq < 0 {
		sinceSeq = 0
	}
	limit = earliest - 1 - sinceSeq
	return sinceSeq, limit
}

// refreshFrames 拉**首屏一页**原始帧（尾窗），建立 transcript 基线。
//
// 为什么必须走 history 而不是 load：load 是投影后的 messages，且没有 frames 键
// （契约 §恢复）。第一次打开会话时 WS 只从 lastSeq 之后推，之前的历史只有
// history 能拿到。
//
// **不是全量循环**：早先这里从 0 拉到拉完（长会话几百次分页，打开一个会话要
// 好几秒）。现在只拉尾窗一页，更早的交给用户在 `.older` 上按需翻（`loadOlderFrames`）。
//
// 失败只记日志、不编数据：`paging.Error` 非空时那行按钮变成「重新加载」，
// 点它就是再走一次本函数（见 `loadOlderFrames` 的兜底分支）。
func (m *model) refreshFrames(id string) {
	if id == "" || m.cli == nil {
		return
	}
	m.mu.Lock()
	journal := m.frameJournalLocked(id)
	if journal.paging.Loading {
		// 已经在拉：重复点击/重复触发不叠加请求（一次只翻一页）。
		m.mu.Unlock()
		return
	}
	// 锚点优先用 load 的 lastSeq；journal 里已经消费过更高 seq（WS 先到）时取大的。
	anchor := journal.lastSeq
	if base := m.seqs[id]; base > anchor {
		anchor = base
	}
	// 基线重建：上一轮的"还有更多/失败"都作废，等这一页回来重算。
	journal.paging.Loading = true
	journal.paging.Error = ""
	m.mu.Unlock()
	m.inv.Invalidate()

	ctx, cancel := context.WithTimeout(context.Background(), frameHistoryTimeout)
	defer cancel()

	// 水位未知（load 没跑成 / 空会话 / 老宿主）：先探一帧拿 lastSeq。
	// 不探的话只能从 0 开始，而那样拿到的是最老的一页——长会话上等于回到几小时前。
	if anchor <= 0 {
		probe, err := m.cli.History(ctx, id, 0, 1, 0)
		if err != nil {
			m.finishBaseline(id, nil, 0, err)
			return
		}
		anchor = probe.LastSeq
	}
	sinceSeq, limit := frameTailWindow(anchor, frameHistoryChunk)
	res, err := m.cli.History(ctx, id, sinceSeq, limit, 0)
	if err != nil {
		m.finishBaseline(id, nil, 0, err)
		return
	}
	m.finishBaseline(id, res.Frames, int64(res.JournalEpoch), nil)
}

// finishBaseline 把首屏那一页并进日志，并重算分页状态。err != nil 时只记失败。
func (m *model) finishBaseline(id string, frames []apiclient.Frame, epoch int64, err error) {
	m.mu.Lock()
	journal := m.frameJournalLocked(id)
	journal.paging.Loading = false
	journal.paging.Baseline = true
	if err != nil {
		journal.paging.Error = err.Error()
		m.mu.Unlock()
		log.Printf("history 基线失败 session:%s: %v", id, err)
		m.inv.Invalidate()
		return
	}
	journal.paging.Error = ""
	// 世代不一致 = 宿主换过 journal：整段丢弃重建（旧 seq 属于另一个序号域）。
	// 与 projection 层 `ApplyHistory` 的判据**逐字一致**，两处不能各写一套。
	if journal.epoch != 0 && epoch != 0 && epoch != journal.epoch {
		log.Printf("transcript 基线换代 session:%s 旧 epoch=%d 新 epoch=%d", id, journal.epoch, epoch)
		journal.reset()
		journal.paging.Baseline = true
	}
	if epoch > 0 {
		journal.epoch = epoch
		if m.epochs == nil {
			m.epochs = map[string]int64{}
		}
		m.epochs[id] = epoch
	}
	added := journal.insertAll(frames)
	// 尾窗之前还有没有帧：earliest > 1 就是"宿主手里还有更早的"。
	// 空会话（0 帧）= 没有更多；只有一页的短会话同理。
	earliest := journal.earliestSeq()
	journal.paging.HasMore = earliest > 1
	journal.paging.Partial = earliest > 1
	m.mu.Unlock()

	if added > 0 {
		log.Printf("transcript 基线 session:%s 拉回 %d 帧（epoch=%d，最早 seq=%d，还有更早=%v）",
			id, added, epoch, earliest, earliest > 1)
		m.inv.Invalidate()
	}
}

// loadOlderFrames 向上翻一页（loadOlder）：拉**当前最早 seq 之前**紧邻的一页。
//
// 三条判据（按顺序）：
//   - 正在拉 → 忽略（一次只翻一页）；
//   - 基线没建起来（首屏就失败了）→ 这一下就是「重新加载」，走基线；
//   - 已经到第 1 帧 / 没有更多 → 什么都不做（按钮本来就是禁用态）。
//
// 网络请求在调用方的 goroutine 里（按钮回调必须立刻返回，绘制线程不能等网络）。
func (m *model) loadOlderFrames(id string) {
	if id == "" || m.cli == nil {
		return
	}
	m.mu.Lock()
	journal := m.frameJournalLocked(id)
	if journal.paging.Loading {
		m.mu.Unlock()
		return
	}
	if !journal.paging.Baseline || (journal.paging.Error != "" && len(journal.frames) == 0) {
		// 首屏基线都没成功过：这一下的语义是「重新加载」，不是「更早一页」。
		m.mu.Unlock()
		m.refreshFrames(id)
		return
	}
	earliest := journal.earliestSeq()
	if earliest <= 1 || !journal.paging.HasMore {
		journal.paging.HasMore = false
		m.mu.Unlock()
		return
	}
	sinceSeq, limit := frameOlderWindow(earliest, frameHistoryChunk)
	// 向前分页**必须带世代**：这一页可能跨几秒，期间宿主完全可能换代；
	// 带同一代的水位去要新一代的帧，宿主会 409 而不是给出错位数据。
	epoch := int(journal.epoch)
	journal.paging.Loading = true
	journal.paging.Error = ""
	m.mu.Unlock()
	m.inv.Invalidate()

	ctx, cancel := context.WithTimeout(context.Background(), frameHistoryTimeout)
	defer cancel()
	res, err := m.cli.History(ctx, id, sinceSeq, limit, epoch)
	if err != nil {
		m.finishOlder(id, nil, 0, limit, err)
		return
	}
	m.finishOlder(id, res.Frames, int64(res.JournalEpoch), limit, nil)
}

// finishOlder 把「更早一页」并进日志，并重算分页状态。
//
// limit 是这一页**要了多少条**：回包不足 limit 条就说明宿主手里没有更早的了
// （不是"失败"，是"到头了"），这一页之后按钮变成「已到会话开头」。
func (m *model) finishOlder(id string, frames []apiclient.Frame, epoch int64, limit int, err error) {
	m.mu.Lock()
	journal := m.frameJournalLocked(id)
	journal.paging.Loading = false
	if err != nil {
		switch {
		case journalEvictedError(err):
			// 更早的历史已经滑出宿主内存 journal：**这就是"没有更多"**。
			// 不重建基线——重建只会把尾窗再拉一遍，然后同一次点击又能再点。
			journal.paging.HasMore = false
			journal.paging.Error = ""
			m.mu.Unlock()
			log.Printf("transcript 向前分页 session:%s 已到宿主保留的最早处（%v）", id, err)
		case staleEpochError(err):
			// journal 换代：旧帧属于另一个序号域，整段丢弃，重建基线。
			oldEpoch := journal.epoch
			journal.reset()
			m.mu.Unlock()
			log.Printf("transcript 向前分页遇到换代 session:%s 旧 epoch=%d 新 epoch=%d → 重建基线",
				id, oldEpoch, epoch)
			m.refreshFrames(id)
			return
		default:
			journal.paging.Error = err.Error()
			m.mu.Unlock()
			log.Printf("transcript 向前分页失败 session:%s: %v", id, err)
		}
		m.inv.Invalidate()
		return
	}
	journal.paging.Error = ""
	added := journal.insertAll(frames)
	earliest := journal.earliestSeq()
	// 还有更早的 = 这一页要满了（说明后面大概还有）且最早 seq 还没到 1。
	// 两条件缺一不可：短会话最后一页不足 limit，earliest 却仍 > 1（journal 从中间开始）。
	journal.paging.HasMore = added > 0 && len(frames) >= limit && earliest > 1
	m.mu.Unlock()

	if added > 0 {
		log.Printf("transcript 向前分页 session:%s 补回 %d 帧（最早 seq=%d，还有更早=%v）",
			id, added, earliest, len(frames) >= limit && earliest > 1)
		m.inv.Invalidate()
	}
}

// staleEpochError 判断一条错误是不是 journal 换代的 409 `stale_epoch`。
//
// 契约：宿主回 409 时错误体是四字段，`code` 就是 `stale_epoch`；apiclient
// 把它包成 `APIError`，这里只匹配码本身，不猜别的 409。
func staleEpochError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "stale_epoch")
}

// journalEvictedError 判断一条 409 是不是「更早的历史已经滑出宿主内存 journal」。
//
// 宿主对两种 409 回**同一个码** `stale_epoch`（`internal/transport/server.go`
// 的 handleSessionHistory）：
//
//	① journal 换代                 → 该重建基线；
//	② sinceSeq 之前已滑出内存 journal → **这就是"没有更早的了"**，不是失败。
//
// 错误体只有四字段（code/message/hint/retryable），没有能区分的结构化字段；
// 宿主在 message 里写清了是哪种（"…已经滑出内存 journal（只保留最近 N 帧）…"），
// 这里按那句话分。**判错的代价**：当成 ① 会多重建一次基线（点第二次才到头），
// 当成 ② 会少一页——所以下面这条测试同时钉住"能认出来"和"认不出时的兜底"。
func journalEvictedError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "滑出内存")
}
