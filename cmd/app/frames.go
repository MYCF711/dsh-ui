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
}

// reset 丢弃全部帧与水位（journal 换代 / 切会话时调用）。
func (j *frameJournal) reset() {
	j.frames = j.frames[:0]
	j.epoch = 0
	j.lastSeq = 0
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

// refreshFrames 用 `session.history` 从 0 拉一条会话的**全部原始帧**，重建基线。
//
// 为什么必须走 history 而不是 load：load 是投影后的 messages，且没有 frames 键
// （契约 §恢复）。第一次打开会话时 WS 只从 lastSeq 之后推，之前的历史只有
// history 能拿到。
//
// 失败只记日志、不编数据：transcript 数据面为空时插件回退旧渲染路径，
// 界面上是"旧的消息列表"，不是"空白主区"。
func (m *model) refreshFrames(id string) {
	if id == "" || m.cli == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), frameHistoryTimeout)
	defer cancel()

	var (
		all   []apiclient.Frame
		epoch int
		since int
	)
	for {
		// epoch 传 0 = 「我还没有基线，宿主不核」。这是**唯一**该传 0 的地方：
		// 补洞必须带世代（见 apiclient.FillGap），否则换代之后补回来的是错位数据。
		res, err := m.cli.History(ctx, id, since, frameHistoryChunk, 0)
		if err != nil {
			log.Printf("history 基线失败 session:%s since=%d: %v", id, since, err)
			return
		}
		all = append(all, res.Frames...)
		if res.JournalEpoch > 0 {
			epoch = res.JournalEpoch
		}
		if len(res.Frames) < frameHistoryChunk {
			break
		}
		last := res.Frames[len(res.Frames)-1].Seq
		if last <= since {
			break // 没有前进：别死循环
		}
		since = last
	}

	m.mu.Lock()
	journal := m.frameJournalLocked(id)
	// 世代不一致 = 宿主换过 journal：整段丢弃重建（旧 seq 属于另一个序号域）。
	// 与 projection 层 `ApplyHistory` 的判据**逐字一致**，两处不能各写一套。
	if journal.epoch != 0 && epoch != 0 && int64(epoch) != journal.epoch {
		log.Printf("transcript 基线换代 session:%s 旧 epoch=%d 新 epoch=%d", id, journal.epoch, epoch)
		journal.reset()
	}
	if epoch > 0 {
		journal.epoch = int64(epoch)
		if m.epochs == nil {
			m.epochs = map[string]int64{}
		}
		m.epochs[id] = int64(epoch)
	}
	added := journal.insertAll(all)
	m.mu.Unlock()

	if added > 0 {
		log.Printf("transcript 基线 session:%s 拉回 %d 帧（epoch=%d，累计 %d）", id, added, epoch, len(all))
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
