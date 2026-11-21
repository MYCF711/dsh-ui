package dsconversation

// official_transcript_backfill_test.go —— 主区**补回来的老帧**必须落成行。
//
// 为什么单独一个文件：`applyFrames` 早先按 `seq <= lastSeq` 判重，而 lastSeq 是
// "已消费的最高 seq"。首屏基线只拉**尾窗**（`cmd/app/frames.go` 的
// `frameTailWindow`），之后两种补帧都落在水位**之下**：
//
//	「加载更早」  → 前窗 [earliest-chunk, earliest-1]
//	断号补洞      → 缺口 [from, to]（WS 已经把 to+1 推上去过了）
//
// 按水位判重会把它们全当成"重放"丢掉：**不报错**，只是按钮已经变成「已到会话
// 开头」而主区一行都没多、补洞补回来的那几轮永远不出现。这两条都是"不报错但
// 结果错"，只能靠用例钉住。
//
// 顺带钉住三条不能破的：
//   - 行序 = journal seq 顺序（老帧排在前面，不是贴到最新一条下面）；
//   - 重投影**不能把流式正文翻倍**（清空后重来，不是原地上再并一遍）；
//   - 同一份快照重复画（每帧都喂一遍）不许多出行。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// userFrame 造一条带唯一 id 的用户消息帧（每条落成一行 user 节点）。
func userFrame(seq int, id, text string) apiclient.Frame {
	return rawFrame(seq, "user/message",
		`{"surfaceOp":"append","source":{"kind":"user"},"id":"`+id+
			`","content":[{"type":"text","text":"`+text+`"}]}`)
}

// TestLoadOlderRowsAppearAboveBaseline 钉住「加载更早」那一页真的多出几行。
//
// 首屏只拿到尾窗（seq 201..203）；用户点「加载更早」后壳把前窗并进 journal，
// 下一帧交给投影的是**升序全量** [199..203]。老帧必须排在前面。
func TestLoadOlderRowsAppearAboveBaseline(t *testing.T) {
	p := productPlugin()
	tail := []apiclient.Frame{
		userFrame(201, "m-201", "尾部一"),
		userFrame(202, "m-202", "尾部二"),
		userFrame(203, "m-203", "尾部三"),
	}
	renderSeat(t, p, pluginapi.Snapshot{Cur: "s-1", Frames: tail, JournalEpoch: 1}, "official-transcript")
	if got := len(p.TranscriptEntries()); got != 3 {
		t.Fatalf("尾窗基线投影 %d 行，想要 3", got)
	}

	// 壳补回前窗：快照变成升序全量 [199..203]。
	older := []apiclient.Frame{
		userFrame(199, "m-199", "更早的一"),
		userFrame(200, "m-200", "更早的二"),
	}
	all := append(append([]apiclient.Frame{}, older...), tail...)
	canvas := renderSeat(t, p, pluginapi.Snapshot{Cur: "s-1", Frames: all, JournalEpoch: 1}, "official-transcript")

	entries := p.TranscriptEntries()
	if len(entries) != 5 {
		t.Fatalf("补回前窗后 %d 行，想要 5（水位之下的老帧被当成重放丢掉了）", len(entries))
	}
	want := []string{"更早的一", "更早的二", "尾部一", "尾部二", "尾部三"}
	for i, text := range want {
		if entries[i].Data.Text != text {
			t.Fatalf("第 %d 行 = %q，想要 %q（行序必须 = journal seq 顺序）", i, entries[i].Data.Text, text)
		}
	}
	if !hasCanvasText(canvas, "更早的一") {
		t.Fatalf("补回来的老帧没画出来：%v", canvasValues(canvas))
	}
}

// TestGapFillRowsAppear 钉住断号补洞补回来的那几行真的出现。
//
// 真实时序：WS 先推 seq=8（中间 5..7 断号），壳再用 `session.history` 补 [5,7]
// 并进 journal。下一帧快照是 [1..8]，而水位已经是 8——补回来的三帧全在水位之下。
func TestGapFillRowsAppear(t *testing.T) {
	p := productPlugin()
	head := []apiclient.Frame{
		userFrame(1, "m-1", "第一条"),
		userFrame(2, "m-2", "第二条"),
		userFrame(3, "m-3", "第三条"),
		userFrame(4, "m-4", "第四条"),
	}
	jump := userFrame(8, "m-8", "第八条")
	renderSeat(t, p,
		pluginapi.Snapshot{Cur: "s-1", Frames: append(append([]apiclient.Frame{}, head...), jump), JournalEpoch: 1},
		"official-transcript")
	if got := len(p.TranscriptEntries()); got != 5 {
		t.Fatalf("断号时投影 %d 行，想要 5（1..4 + 8）", got)
	}

	// 壳把缺口 5..7 补回来，快照变成完整升序。
	filled := append(append([]apiclient.Frame{}, head...),
		userFrame(5, "m-5", "第五条"),
		userFrame(6, "m-6", "第六条"),
		userFrame(7, "m-7", "第七条"),
		jump,
	)
	canvas := renderSeat(t, p, pluginapi.Snapshot{Cur: "s-1", Frames: filled, JournalEpoch: 1}, "official-transcript")

	entries := p.TranscriptEntries()
	if len(entries) != 8 {
		t.Fatalf("补洞后 %d 行，想要 8（补回来的 5/6/7 被当成重放丢掉了）", len(entries))
	}
	for i := range entries {
		want := []string{"第一条", "第二条", "第三条", "第四条", "第五条", "第六条", "第七条", "第八条"}[i]
		if entries[i].Data.Text != want {
			t.Fatalf("第 %d 行 = %q，想要 %q（行序必须 = journal seq 顺序）", i, entries[i].Data.Text, want)
		}
	}
	if !hasCanvasText(canvas, "第五条") {
		t.Fatalf("补洞补回来的正文没画出来：%v", canvasValues(canvas))
	}
}

// TestBackfillRebuildDoesNotDoubleStreamingText 钉住重投影**不翻倍**流式正文。
//
// 补帧时整段重投影是唯一能保住行序的做法，但代价是"同一批帧会再投影一次"。
// 如果重投影不是"清空后重来"而是"原地上再并一遍"，流式增量（`assistant/text`）
// 会被 `appendStep` 再拼一次——界面上正文逐字翻倍，而且没有任何报错。
func TestBackfillRebuildDoesNotDoubleStreamingText(t *testing.T) {
	p := productPlugin()
	tail := []apiclient.Frame{
		rawFrame(11, "assistant/text", `{"turn":1,"step":1,"text":"你好"}`),
		rawFrame(12, "assistant/text", `{"turn":1,"step":1,"text":"，世界"}`),
	}
	renderSeat(t, p, pluginapi.Snapshot{Cur: "s-1", Frames: tail, JournalEpoch: 1}, "official-transcript")
	entries := p.TranscriptEntries()
	if len(entries) != 1 || entries[0].Data.Text != "你好，世界" {
		t.Fatalf("基线流式正文 = %+v，想要一条「你好，世界」", entries)
	}

	older := []apiclient.Frame{userFrame(9, "m-9", "上一轮的问题")}
	all := append(append([]apiclient.Frame{}, older...), tail...)
	renderSeat(t, p, pluginapi.Snapshot{Cur: "s-1", Frames: all, JournalEpoch: 1}, "official-transcript")

	entries = p.TranscriptEntries()
	if len(entries) != 2 {
		t.Fatalf("补帧后 %d 行，想要 2", len(entries))
	}
	if entries[1].Data.Text != "你好，世界" {
		t.Fatalf("重投影后流式正文 = %q，想要「你好，世界」（不能翻倍）", entries[1].Data.Text)
	}
}

// TestSameSnapshotRedrawKeepsRowCount 钉住"每帧喂同一份快照"不多出行。
func TestSameSnapshotRedrawKeepsRowCount(t *testing.T) {
	p := productPlugin()
	frames := []apiclient.Frame{
		userFrame(101, "m-101", "甲"),
		userFrame(102, "m-102", "乙"),
	}
	snap := pluginapi.Snapshot{Cur: "s-1", Frames: frames, JournalEpoch: 1}
	for i := 0; i < 5; i++ {
		renderSeat(t, p, snap, "official-transcript")
	}
	if got := len(p.TranscriptEntries()); got != 2 {
		t.Fatalf("同一份快照画五帧后有 %d 行，想要 2", got)
	}
}
