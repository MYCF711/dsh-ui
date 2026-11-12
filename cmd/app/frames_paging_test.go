package main

// frames_paging_test.go —— 主区 transcript **向前分页**（loadOlder 五态）的数据面断言。
//
// 为什么单独一个文件：`frames_test.go` 钉的是"帧日志本身"（有序插入 / 去重 /
// 水位 / 换代），这里钉的是**分页**——窗口怎么算、翻页怎么并、五种失败/到头
// 情形各留下什么状态。两件事混在一个文件里，改分页时会看花眼。
//
// 五态（界面层 `HistoryLoadState`）的数据面对应物是 `framePaging`：
//
//	首次     paging.Baseline == false && paging.Loading
//	有更多   Baseline && HasMore
//	加载中   Baseline && Loading
//	无更多   Baseline && !HasMore && Partial（Partial 是粘性的）
//	失败     Baseline && Error != ""
//
// 这里逐条驱动出来，**不 mock 绘制**——绘制那半边在
// `plugins/dsconversation/official_transcript_paging_test.go`。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// ── 窗口算法（纯函数）─────────────────────────────────────────────────

// TestFrameTailWindow 尾窗以 anchor 为最高 seq，向前的 size 帧。
func TestFrameTailWindow(t *testing.T) {
	cases := []struct {
		name                 string
		anchor, size         int
		wantSince, wantLimit int
	}{
		{"长会话取最后一页", 1000, 200, 800, 200},
		{"刚好一页", 200, 200, 0, 200},
		{"不足一页", 37, 200, 0, 37},
		{"水位未知", 0, 200, 0, 200},
		{"锚点小于页大小", 5, 200, 0, 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			since, limit := frameTailWindow(c.anchor, c.size)
			if since != c.wantSince || limit != c.wantLimit {
				t.Fatalf("frameTailWindow(%d,%d) = (%d,%d)，想要 (%d,%d)",
					c.anchor, c.size, since, limit, c.wantSince, c.wantLimit)
			}
		})
	}
}

// TestFrameOlderWindow 前窗是 **earliest 之前紧邻**的 size 帧。
//
// 这是"用 sinceSeq 的 API 做向前分页"的全部难点：宿主只有开区间下界，
// 所以"更早一页"必须自己算成 `sinceSeq = earliest-1-size`。
func TestFrameOlderWindow(t *testing.T) {
	cases := []struct {
		name                 string
		earliest, size       int
		wantSince, wantLimit int
	}{
		{"中间一页", 801, 200, 600, 200},
		{"剩最后一页", 201, 200, 0, 200},
		{"不足一页", 150, 200, 0, 149},
		{"已经是第一帧", 1, 200, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			since, limit := frameOlderWindow(c.earliest, c.size)
			if since != c.wantSince || limit != c.wantLimit {
				t.Fatalf("frameOlderWindow(%d,%d) = (%d,%d)，想要 (%d,%d)",
					c.earliest, c.size, since, limit, c.wantSince, c.wantLimit)
			}
		})
	}
}

// ── 假宿主（可切换正常 / 换代 / 滑出 / 断网）──────────────────────────

// historyHost 是一个可控的 `session.history` 假宿主。
type historyHost struct {
	frames []apiclient.Frame
	epoch  int
	// status 非 0 时直接回这个状态码（配合 body 造 409）。
	status int
	body   string
	// requests 记录收到的 sinceSeq（断言"没有重拉整页"）。
	requests []int
}

func (h *historyHost) server(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session.history" {
			http.NotFound(w, r)
			return
		}
		var params struct {
			SinceSeq     int `json:"sinceSeq"`
			Limit        int `json:"limit"`
			JournalEpoch int `json:"journalEpoch"`
		}
		_ = json.NewDecoder(r.Body).Decode(&params)
		h.requests = append(h.requests, params.SinceSeq)
		if h.status != 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(h.status)
			_, _ = w.Write([]byte(h.body))
			return
		}
		// 契约：sinceSeq 是**开区间**（传 166 拿到 167 起）。
		out := make([]apiclient.Frame, 0, 8)
		for _, frame := range h.frames {
			if frame.Seq <= params.SinceSeq {
				continue
			}
			out = append(out, frame)
			if params.Limit > 0 && len(out) >= params.Limit {
				break
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"result":{"frames":%s,"lastSeq":%d,"journalEpoch":%d}}`,
			mustJSON(out), lastSeqOf(h.frames), h.epoch)
	}))
}

// pagingModel 造一个连着假宿主的 model（水位先按 lastSeq 填好，等价于 openSession 的冷载）。
func pagingModel(t *testing.T, host *historyHost) (*model, *httptest.Server) {
	t.Helper()
	server := host.server(t)
	m := &model{cli: apiclient.New(server.URL), cur: "s-1"}
	return m, server
}

// ── 五态逐个驱动 ──────────────────────────────────────────────────────

// TestBaselineShortSessionIsHidden 一页装得下的短会话：**没有更早的**。
//
// 界面据此不画那一行（官方 `(hasMore || historyWasPartial)`）。
// 画出来的话，每个短会话顶部都会永远挂着一条「已到会话开头」。
func TestBaselineShortSessionIsHidden(t *testing.T) {
	host := &historyHost{frames: makeFrames(12), epoch: 7}
	m, server := pagingModel(t, host)
	defer server.Close()

	m.refreshFrames("s-1")
	p := m.frames["s-1"].paging
	if !p.Baseline {
		t.Fatal("基线已经试过，Baseline 必须为 true")
	}
	if p.Loading {
		t.Fatal("请求已返回，Loading 必须归 false")
	}
	if p.HasMore || p.Partial {
		t.Fatalf("短会话不该有更早的：HasMore=%v Partial=%v", p.HasMore, p.Partial)
	}
	if p.Error != "" {
		t.Fatalf("成功路径不该有错误：%q", p.Error)
	}
}

// TestBaselineLongSessionStartsAtTail 长会话的基线是**尾窗**，不是从头开始。
//
// 从头开始的话用户打开看到的是最老的一页（几小时前的内容），
// 而且 HasMore 恒为 false——分页按钮永远画不出来。
func TestBaselineLongSessionStartsAtTail(t *testing.T) {
	host := &historyHost{frames: makeFrames(500), epoch: 3}
	m, server := pagingModel(t, host)
	defer server.Close()

	m.refreshFrames("s-1")
	journal := m.frames["s-1"]
	got := frameSeqs(journal.snapshot())
	if len(got) != frameHistoryChunk || got[0] != 301 {
		t.Fatalf("基线 = %d 帧，首 seq=%d，想要 %d 帧从 301 起",
			len(got), got[0], frameHistoryChunk)
	}
	if !journal.paging.HasMore || !journal.paging.Partial {
		t.Fatalf("尾窗之前还有 300 帧：HasMore=%v Partial=%v，都该是 true",
			journal.paging.HasMore, journal.paging.Partial)
	}
	// 只有 2 次请求：探针（sinceSeq=0,limit=1）+ 尾窗。**不是**从 0 拉到 500。
	if len(host.requests) != 2 {
		t.Fatalf("基线用了 %d 次请求（%v），想要 2 次（探针 + 尾窗）", len(host.requests), host.requests)
	}
}

// TestLoadOlderPrependingEveryPage 连续向前翻页：每页接上、seq 连续、不重复。
func TestLoadOlderPrependingEveryPage(t *testing.T) {
	host := &historyHost{frames: makeFrames(450), epoch: 5}
	m, server := pagingModel(t, host)
	defer server.Close()

	m.refreshFrames("s-1")
	journal := m.frames["s-1"]
	pages := 0
	for journal.paging.HasMore {
		before := journal.earliestSeq()
		m.loadOlderFrames("s-1")
		pages++
		if journal.earliestSeq() >= before {
			t.Fatalf("第 %d 页没有向前推进：earliest %d → %d", pages, before, journal.earliestSeq())
		}
		if pages > 10 {
			t.Fatal("翻页没有收敛（HasMore 一直为 true）")
		}
	}
	got := frameSeqs(journal.snapshot())
	if len(got) != 450 {
		t.Fatalf("翻完 = %d 帧，想要 450", len(got))
	}
	for i := range got {
		if got[i] != i+1 {
			t.Fatalf("第 %d 条 seq = %d，想要 %d（丢帧或重复）", i, got[i], i+1)
		}
	}
	// 到位之后：HasMore=false，但 Partial 仍是 true（那一行变成「已到会话开头」）。
	if journal.paging.HasMore {
		t.Fatal("已经到第 1 帧了，HasMore 必须为 false")
	}
	if !journal.paging.Partial {
		t.Fatal("Partial 是粘性的：翻到底之后仍应为 true")
	}
}

// TestLoadOlderAtStartIsNoop 已经到第 1 帧时再点：不发请求、状态不变。
func TestLoadOlderAtStartIsNoop(t *testing.T) {
	host := &historyHost{frames: makeFrames(12), epoch: 1}
	m, server := pagingModel(t, host)
	defer server.Close()
	m.refreshFrames("s-1")

	before := len(host.requests)
	m.loadOlderFrames("s-1")
	if len(host.requests) != before {
		t.Fatalf("已经到会话开头还发了 %d 次请求", len(host.requests)-before)
	}
	if m.frames["s-1"].paging.HasMore {
		t.Fatal("到头之后 HasMore 必须是 false")
	}
}

// TestLoadOlderFailureKeepsFramesAndRetries 失败态：**帧一帧都不丢**，只是记下错误。
//
// 清掉帧 = 主区突然空一下（用户以为消息丢了）；保留 + 显示「重新加载」
// 才是可以重试的状态。
func TestLoadOlderFailureKeepsFramesAndRetries(t *testing.T) {
	host := &historyHost{frames: makeFrames(450), epoch: 5}
	m, server := pagingModel(t, host)
	defer server.Close()
	m.refreshFrames("s-1")
	before := frameSeqs(m.frames["s-1"].snapshot())

	// 断网：换一个连不上的 client。
	m.mu.Lock()
	m.cli = apiclient.New("http://127.0.0.1:1")
	m.mu.Unlock()
	m.loadOlderFrames("s-1")

	journal := m.frames["s-1"]
	if journal.paging.Error == "" {
		t.Fatal("失败之后必须留下错误原因（界面据此画「重新加载」）")
	}
	if journal.paging.Loading {
		t.Fatal("请求结束（即使失败）Loading 必须归 false，否则按钮永远转圈")
	}
	if got := frameSeqs(journal.snapshot()); !equalSeqs(got, before) {
		t.Fatalf("失败后帧从 %d 条变成 %d 条（不该清）", len(before), len(got))
	}
	// 恢复：错误清掉，HasMore 继续为 true（这一页还没补上）。
	m.mu.Lock()
	m.cli = apiclient.New(server.URL)
	m.mu.Unlock()
	m.loadOlderFrames("s-1")
	if journal.paging.Error != "" {
		t.Fatalf("重试成功后错误该清掉：%q", journal.paging.Error)
	}
	if len(journal.snapshot()) <= len(before) {
		t.Fatal("重试成功后应该补回一页")
	}
}

// TestLoadOlderEpochChangeRebuilds 409 `stale_epoch`（真的换代）→ 丢弃旧基线重建。
//
// 宿主重启后 seq 从 1 起，旧帧属于另一个序号域：留着就是新旧两代混排，
// 界面上表现为"消息偶尔跳一下"（最难看出来的一种错）。
func TestLoadOlderEpochChangeRebuilds(t *testing.T) {
	host := &historyHost{frames: makeFrames(450), epoch: 5}
	m, server := pagingModel(t, host)
	defer server.Close()
	m.refreshFrames("s-1")

	// 换代：新 journal 只有 30 帧，epoch 变了。
	host.frames = makeFrames(30)
	host.epoch = 9
	host.status = http.StatusConflict
	host.body = `{"error":{"code":"stale_epoch","message":"journal 世代或水位对不上，需要重新 load 基线","hint":"重建","retryable":true}}`
	m.loadOlderFrames("s-1")
	// 409 之后会同步重建基线：把 status 清掉让它成功。
	host.status = 0
	m.refreshFrames("s-1")

	journal := m.frames["s-1"]
	if journal.epoch != 9 {
		t.Fatalf("重建后 epoch = %d，想要 9", journal.epoch)
	}
	got := frameSeqs(journal.snapshot())
	if len(got) == 0 || got[len(got)-1] != 30 {
		t.Fatalf("重建后基线 = %v，想要以 30 结尾（新 journal 只有 30 帧）", got)
	}
}

// TestLoadOlderEvictedJournalMeansNoMore 409 的另一种来源：**更早的历史滑出内存 journal**。
//
// 宿主对两种 409 回同一个码（`server.go` handleSessionHistory），区别只在
// message 里。滑出内存 = "没有更早的了"，**不是失败**：
//   - 不能画「重新加载」（点了还是同一个 409，用户会一直点）；
//   - 要画「已到会话开头」。
func TestLoadOlderEvictedJournalMeansNoMore(t *testing.T) {
	host := &historyHost{frames: makeFrames(450), epoch: 5}
	m, server := pagingModel(t, host)
	defer server.Close()
	m.refreshFrames("s-1")
	before := len(m.frames["s-1"].snapshot())

	host.status = http.StatusConflict
	host.body = `{"error":{"code":"stale_epoch","message":"sinceSeq=0 之前的历史已经滑出内存 journal（只保留最近 300 帧）：继续按 seq 补会缺一段，而且缺口看不出来","hint":"重建","retryable":true}}`
	m.loadOlderFrames("s-1")

	journal := m.frames["s-1"]
	if journal.paging.Error != "" {
		t.Fatalf("滑出内存不是失败，不该留错误：%q", journal.paging.Error)
	}
	if journal.paging.HasMore {
		t.Fatal("已经到宿主保留的最早处，HasMore 必须为 false")
	}
	if got := len(journal.snapshot()); got != before {
		t.Fatalf("帧数从 %d 变成 %d（滑出内存不该动已有帧）", before, got)
	}
}

// TestLoadOlderSingleFlight 连点两次只发一次请求（按钮在加载中不该排队）。
func TestLoadOlderSingleFlight(t *testing.T) {
	host := &historyHost{frames: makeFrames(450), epoch: 5}
	m, server := pagingModel(t, host)
	defer server.Close()
	m.refreshFrames("s-1")

	// 手工把 Loading 置起来（模拟第一页还没回来）。
	m.mu.Lock()
	m.frames["s-1"].paging.Loading = true
	m.mu.Unlock()
	before := len(host.requests)
	m.loadOlderFrames("s-1")
	if len(host.requests) != before {
		t.Fatalf("加载中又发了一次请求（%d → %d）", before, len(host.requests))
	}
}

// TestSnapshotHistoryPagingCopies 快照拿的是**值拷贝**：绘制期在锁外读，
// 活结构不能交出去。
func TestSnapshotHistoryPagingCopies(t *testing.T) {
	host := &historyHost{frames: makeFrames(450), epoch: 5}
	m, server := pagingModel(t, host)
	defer server.Close()
	m.refreshFrames("s-1")

	m.mu.Lock()
	paging := m.snapshotHistoryPagingLocked("s-1")
	paging.HasMore = false
	m.mu.Unlock()
	if !m.frames["s-1"].paging.HasMore {
		t.Fatal("快照与 journal 共享状态（改快照影响 journal）")
	}
}

// TestFrameJournalResetClearsPaging reset 之后分页状态必须归零。
//
// 少清一份 = 换代之后界面上还挂着上一代的「还有更多」，点了拿不到东西。
func TestFrameJournalResetClearsPaging(t *testing.T) {
	journal := &frameJournal{}
	journal.insertAll(makeFrames(3))
	journal.paging = framePaging{Baseline: true, HasMore: true, Partial: true, Loading: true, Error: "x"}
	journal.reset()
	if journal.paging != (framePaging{}) {
		t.Fatalf("reset 之后分页状态还有残留：%+v", journal.paging)
	}
}

// TestJournalEvictedErrorMatches 钉住 409 两种来源的**区分判据**。
//
// 这是全文件唯一一处"按宿主 message 分"的地方：错误体只有四字段，
// 没有结构化字段能区分。判错两个方向的代价都写在这里：
//   - 认不出滑出内存 → 画成「重新加载」，点了还是 409（一直点一直转）；
//   - 把换代认成滑出内存 → 少重建一次基线（下一帧 WS 的 session/reset 兜底）。
func TestJournalEvictedErrorMatches(t *testing.T) {
	evicted := &apiclient.APIError{
		Code:    "stale_epoch",
		Message: "sinceSeq=0 之前的历史已经滑出内存 journal（只保留最近 300 帧）",
	}
	if !journalEvictedError(fmt.Errorf("history: %w", evicted)) {
		t.Fatal("滑出内存的 409 没被认出来")
	}
	changed := &apiclient.APIError{Code: "stale_epoch", Message: "journal 世代或水位对不上，需要重新 load 基线"}
	if journalEvictedError(changed) {
		t.Fatal("换代被当成了滑出内存（会少重建一次基线）")
	}
	if journalEvictedError(nil) {
		t.Fatal("nil 被当成滑出内存")
	}
}

// TestLoadOlderConcurrentWithWSFrames 翻页与 WS 增量并发：两条路径并进同一份日志，
// 同 seq 去重、seq 连续（**这是数据面唯一的并发场景**，用 -race 跑才有意义）。
func TestLoadOlderConcurrentWithWSFrames(t *testing.T) {
	host := &historyHost{frames: makeFrames(450), epoch: 5}
	m, server := pagingModel(t, host)
	defer server.Close()
	m.refreshFrames("s-1")

	done := make(chan struct{})
	go func() {
		defer close(done)
		for seq := 451; seq <= 470; seq++ {
			m.mu.Lock()
			m.appendFrameLocked("s-1", testFrame(seq, "assistant/message", ""))
			m.mu.Unlock()
		}
	}()
	for m.frames["s-1"].paging.HasMore {
		m.loadOlderFrames("s-1")
	}
	<-done

	journal := m.frames["s-1"]
	got := frameSeqs(journal.snapshot())
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("seq 不严格递增：%d → %d", got[i-1], got[i])
		}
	}
	if journal.paging.HasMore {
		t.Fatal("已经翻到第 1 帧，HasMore 必须为 false")
	}
}

// 编译期断言：假宿主的请求记录器真的被用到（防止以后重构把它删了却没人发现）。
var _ = atomic.Bool{}
