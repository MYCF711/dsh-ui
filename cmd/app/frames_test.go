package main

// frames_test.go —— 主区 transcript **数据面**的机器断言。
//
// 五组：
//
//	① 有序插入：journal 是 append-only，行序 = seq 顺序（补洞补的是中间那段）
//	② 去重：同 seq 再来一次只能是重放，丢弃（不重复）
//	③ 水位：lastSeq 只增不减；reset 之后归零
//	④ 基线：`session.history` 分页拉全（不是只拉一页），世代随基线写回
//	⑤ 换代：journalEpoch 不一致 → 整段丢弃重建（不混排）

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// testFrame 造一条原始 journal 帧。
func testFrame(seq int, kind, data string) apiclient.Frame {
	frame := apiclient.Frame{Seq: seq, Type: kind, Time: int64(seq) * 1000}
	if data != "" {
		frame.Data = json.RawMessage(data)
	}
	return frame
}

func frameSeqs(frames []apiclient.Frame) []int {
	seqs := make([]int, 0, len(frames))
	for _, frame := range frames {
		seqs = append(seqs, frame.Seq)
	}
	return seqs
}

func equalSeqs(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ── ① 有序插入 ────────────────────────────────────────────────────────

// TestFrameJournalInsertKeepsSeqOrder 补洞补的是**中间**那段：插入必须按 seq
// 就位，不能按到达顺序追加（否则行序错乱，界面上是"消息顺序跳了一下"）。
func TestFrameJournalInsertKeepsSeqOrder(t *testing.T) {
	journal := &frameJournal{}
	// 断号：先到 3，再补 1、2（history 回来的顺序也可能乱）。
	for _, seq := range []int{3, 1, 2, 5, 4} {
		if !journal.insert(testFrame(seq, "user/message", "")) {
			t.Fatalf("seq=%d 应该被插入", seq)
		}
	}
	if got := frameSeqs(journal.snapshot()); !equalSeqs(got, []int{1, 2, 3, 4, 5}) {
		t.Fatalf("帧序 = %v，想要 [1 2 3 4 5]", got)
	}
	if journal.lastSeq != 5 {
		t.Fatalf("lastSeq = %d，想要 5", journal.lastSeq)
	}
}

// ── ② 去重 ────────────────────────────────────────────────────────────

// TestFrameJournalInsertDedupesReplay 同 seq 再来一次只能是重放。
//
// 判据不能只看"比 lastSeq 大"：补洞回来的是**中间**的 seq，比 lastSeq 小
// 但没消费过——所以按有序表查找，不看水位。
func TestFrameJournalInsertDedupesReplay(t *testing.T) {
	journal := &frameJournal{}
	journal.insert(testFrame(1, "user/message", `{"a":1}`))
	journal.insert(testFrame(2, "assistant/message", `{"a":2}`))
	if journal.insert(testFrame(1, "user/message", `{"a":1}`)) {
		t.Fatal("seq=1 重放被当成新帧（界面会重复一条消息）")
	}
	if journal.insert(testFrame(3, "user/message", "")) != true {
		t.Fatal("seq=3 是新帧，应该插入")
	}
	if len(journal.snapshot()) != 3 {
		t.Fatalf("帧数 = %d，想要 3", len(journal.snapshot()))
	}
}

// TestFrameJournalInsertAllDedupesAndSorts 批量并入（history 基线）同款语义。
func TestFrameJournalInsertAllDedupesAndSorts(t *testing.T) {
	journal := &frameJournal{}
	journal.insert(testFrame(2, "assistant/message", ""))
	added := journal.insertAll([]apiclient.Frame{
		testFrame(3, "user/message", ""),
		testFrame(1, "user/message", ""),
		testFrame(2, "assistant/message", ""),
	})
	if added != 2 {
		t.Fatalf("新增 = %d，想要 2（seq=2 是重放）", added)
	}
	if got := frameSeqs(journal.snapshot()); !equalSeqs(got, []int{1, 2, 3}) {
		t.Fatalf("帧序 = %v，想要 [1 2 3]", got)
	}
}

// TestFrameJournalRejectsControlFrames 控制帧没有 seq，进不了"按 seq 去重"的模型。
func TestFrameJournalRejectsControlFrames(t *testing.T) {
	journal := &frameJournal{}
	if journal.insert(testFrame(0, "pong", "")) {
		t.Fatal("seq=0 的控制帧不该进数据面（没有序号可去重）")
	}
	if journal.insert(testFrame(-1, "session/reset", "")) {
		t.Fatal("负数 seq 不该进数据面")
	}
	if len(journal.snapshot()) != 0 {
		t.Fatalf("控制帧混进数据面：%v", frameSeqs(journal.snapshot()))
	}
}

// ── ③ 水位 / reset ───────────────────────────────────────────────────

func TestFrameJournalResetClearsWatermark(t *testing.T) {
	journal := &frameJournal{}
	journal.insert(testFrame(7, "user/message", ""))
	journal.epoch = 3
	journal.reset()
	if journal.lastSeq != 0 || journal.epoch != 0 || len(journal.snapshot()) != 0 {
		t.Fatalf("reset 之后还有残留：lastSeq=%d epoch=%d frames=%v",
			journal.lastSeq, journal.epoch, frameSeqs(journal.snapshot()))
	}
}

// TestJournalResetDropsFrames 钉住 `session/reset` 的完整动作。
//
// `onJournalResetLocked` 必须把**三份水位 + 帧日志**一起清：少清一份就等于
// 新旧两代混排（界面上表现为消息偶尔跳一下，最难看出来的一种错）。
func TestJournalResetDropsFrames(t *testing.T) {
	m := &model{}
	m.mu.Lock()
	m.appendFrameLocked("s-1", testFrame(1, "user/message", ""))
	m.appendFrameLocked("s-1", testFrame(2, "assistant/message", ""))
	m.mu.Unlock()

	m.mu.Lock()
	m.onJournalResetLocked("s-1")
	journal := m.frames["s-1"]
	m.mu.Unlock()
	if journal == nil {
		t.Fatal("reset 之后帧日志不见了（应该留一个空的）")
	}
	if len(journal.snapshot()) != 0 || journal.lastSeq != 0 || journal.epoch != 0 {
		t.Fatalf("reset 之后还有残留：%v", frameSeqs(journal.snapshot()))
	}
}

// TestSnapshotFramesReturnsCopy 绘制期拿到的是**副本**：WS goroutine 还在
// 往 journal 里插入，活切片交出去就是数据竞争。
func TestSnapshotFramesReturnsCopy(t *testing.T) {
	m := &model{}
	m.mu.Lock()
	m.appendFrameLocked("s-1", testFrame(1, "user/message", ""))
	frames, _ := m.snapshotFramesLocked("s-1")
	m.mu.Unlock()
	frames[0].Seq = 999
	if got := m.frames["s-1"].snapshot()[0].Seq; got != 1 {
		t.Fatalf("快照与 journal 共享底层数组（改快照影响 journal）：seq=%d", got)
	}
}

// ── ④ 基线（session.history 分页）────────────────────────────────────

// newHistoryServer 起一个只实现 `session.history` 的假宿主。
//
// 参数是"每次最多返回多少条"，用来验分页：真宿主单次有上限（journal capacity），
// 只拉一页会让中间那段永远补不回来——而界面上看不出少了东西。
func newHistoryServer(t *testing.T, frames []apiclient.Frame, pageSize int, epoch int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session.history" {
			http.NotFound(w, r)
			return
		}
		var params struct {
			ID           string `json:"id"`
			SinceSeq     int    `json:"sinceSeq"`
			Limit        int    `json:"limit"`
			JournalEpoch int    `json:"journalEpoch"`
		}
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// 契约：sinceSeq 是**开区间**（传 166 拿到 167 起）。
		out := make([]apiclient.Frame, 0, pageSize)
		for _, frame := range frames {
			if frame.Seq <= params.SinceSeq {
				continue
			}
			out = append(out, frame)
			if pageSize > 0 && len(out) >= pageSize {
				break
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"result":{"frames":%s,"lastSeq":%d,"journalEpoch":%d}}`,
			mustJSON(out), lastSeqOf(frames), epoch)
	}))
}

func mustJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func lastSeqOf(frames []apiclient.Frame) int {
	if len(frames) == 0 {
		return 0
	}
	return frames[len(frames)-1].Seq
}

// makeFrames 造 n 条连续 seq 的帧。
func makeFrames(n int) []apiclient.Frame {
	frames := make([]apiclient.Frame, 0, n)
	for i := 1; i <= n; i++ {
		frames = append(frames, testFrame(i, "user/message",
			`{"surfaceOp":"append","source":{"kind":"user"},"content":[{"type":"text","text":"消息 `+strconv.Itoa(i)+`"}]}`))
	}
	return frames
}

// TestRefreshFramesPagesThroughHistory 钉住"**分批**拉全"。
//
// 缺口比单次上限大时必须接着拉：早先只拉一次，大缺口会静默漏掉中段。
func TestRefreshFramesPagesThroughHistory(t *testing.T) {
	all := makeFrames(frameHistoryChunk*2 + 7) // 两页多一点
	server := newHistoryServer(t, all, frameHistoryChunk, 4)
	defer server.Close()

	m := &model{cli: apiclient.New(server.URL), cur: "s-1"}
	m.refreshFrames("s-1")

	journal := m.frames["s-1"]
	if journal == nil {
		t.Fatal("基线没建起来")
	}
	got := frameSeqs(journal.snapshot())
	if len(got) != len(all) {
		t.Fatalf("基线帧数 = %d，想要 %d（分页没接上，中段丢了）", len(got), len(all))
	}
	for i := range got {
		if got[i] != i+1 {
			t.Fatalf("第 %d 条 seq = %d，想要 %d", i, got[i], i+1)
		}
	}
	if journal.epoch != 4 {
		t.Fatalf("epoch = %d，想要 4（基线要带上世代）", journal.epoch)
	}
}

// TestRefreshFramesRebuildsOnEpochChange 世代不一致 → 整段丢弃重建。
//
// 宿主重启换 journal 后 seq 从 1 起；拿旧水位硬补，宿主要么回 409，
// 要么从错位处推——**不报错也不丢帧，只是少了几条**。
func TestRefreshFramesRebuildsOnEpochChange(t *testing.T) {
	first := makeFrames(3)
	serverA := newHistoryServer(t, first, 0, 1)
	defer serverA.Close()
	m := &model{cli: apiclient.New(serverA.URL), cur: "s-1"}
	m.refreshFrames("s-1")
	if got := frameSeqs(m.frames["s-1"].snapshot()); !equalSeqs(got, []int{1, 2, 3}) {
		t.Fatalf("第一代基线 = %v", got)
	}

	// 换代：新 journal 的 seq 也从 1 起，但只有 2 条。
	second := makeFrames(2)
	serverB := newHistoryServer(t, second, 0, 2)
	defer serverB.Close()
	m.cli = apiclient.New(serverB.URL)
	m.refreshFrames("s-1")

	journal := m.frames["s-1"]
	if got := frameSeqs(journal.snapshot()); !equalSeqs(got, []int{1, 2}) {
		t.Fatalf("换代后基线 = %v，想要 [1 2]（旧代必须整段丢弃）", got)
	}
	if journal.epoch != 2 {
		t.Fatalf("换代后 epoch = %d，想要 2", journal.epoch)
	}
}

// TestRefreshFramesFailureKeepsBaseline 拉基线失败**不许**清掉已有帧。
//
// 症状对比：清掉 = 主区突然空一下（用户以为消息丢了）；保留 = 就停在上一帧。
func TestRefreshFramesFailureKeepsBaseline(t *testing.T) {
	server := newHistoryServer(t, makeFrames(3), 0, 1)
	defer server.Close()
	m := &model{cli: apiclient.New(server.URL), cur: "s-1"}
	m.refreshFrames("s-1")
	before := frameSeqs(m.frames["s-1"].snapshot())

	// 宿主断开：连不上。
	m.cli = apiclient.New("http://127.0.0.1:1")
	m.refreshFrames("s-1")
	if got := frameSeqs(m.frames["s-1"].snapshot()); !equalSeqs(got, before) {
		t.Fatalf("失败后基线从 %v 变成 %v（不该清）", before, got)
	}
}

// TestStaleEpochErrorMatches 钉住 409 `stale_epoch` 的判据（只认码本身）。
func TestStaleEpochErrorMatches(t *testing.T) {
	if !staleEpochError(fmt.Errorf("history: %w", &apiclient.APIError{Code: "stale_epoch"})) {
		t.Fatal("stale_epoch 没被识别")
	}
	if staleEpochError(&apiclient.APIError{Code: "busy"}) {
		t.Fatal("busy 被当成 stale_epoch")
	}
	if staleEpochError(nil) {
		t.Fatal("nil 被当成 stale_epoch")
	}
}
