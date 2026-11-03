// reset_watermark_test.go —— 钉住「journal 换代后本地水位必须归零」。
//
// 为什么值得单独测：`session/reset`（无 seq 的控制帧）与 history 的 409
// `stale_epoch` 都只说明一件事——"你手里的水位属于上一代了"。清得不全时
// **没有任何报错**：界面只是从错位处开始收，症状是"少了几条"。
//
// 三份水位（`seqs[id]` / `lastSeq` / `subs[id].lastSeq`）漏清任何一份，坏法
// 都一样，所以三条都要断言；另外还要断言**局部 since 归零**——重连用的是
// runSubscription 里那个局部变量，只删 map 的话它照样带着旧值。
package main

import "testing"

func TestJournalResetZeroesEveryWatermark(t *testing.T) {
	// 两个会话都在订：reset 只作用在换代的**那一条连接**上。
	m := &model{
		seqs:    map[string]int{"s-a": 277, "s-b": 12},
		epochs:  map[string]int64{"s-a": 1, "s-b": 1},
		subs:    map[string]*muxSub{"s-a": {lastSeq: 277}, "s-b": {lastSeq: 12}},
		cur:     "s-a",
		lastSeq: 277,
	}

	m.mu.Lock()
	got := m.onJournalResetLocked("s-a")
	m.mu.Unlock()

	// 返回值就是重连要用的 since：旧 seq 属于另一个序号域，只能是 0。
	if got != 0 {
		t.Fatalf("重连用的 since 必须是 0，实际 %d", got)
	}
	if _, ok := m.seqs["s-a"]; ok {
		t.Fatalf("seqs[s-a] 没删：%d", m.seqs["s-a"])
	}
	if _, ok := m.epochs["s-a"]; ok {
		t.Fatalf("epochs[s-a] 没删：%d（留着的话下次 sub 还带旧世代，宿主继续 409）", m.epochs["s-a"])
	}
	if m.lastSeq != 0 {
		t.Fatalf("前台水位 m.lastSeq 没归零：%d", m.lastSeq)
	}
	if sub := m.subs["s-a"]; sub == nil || sub.lastSeq != 0 {
		t.Fatalf("subs[s-a].lastSeq 没归零：%+v", sub)
	}

	// 另一条连接（另一个会话）完全不受影响：reset 是 per-connection 的。
	if m.seqs["s-b"] != 12 || m.epochs["s-b"] != 1 || m.subs["s-b"].lastSeq != 12 {
		t.Fatalf("reset s-a 波及了 s-b：seqs=%d epochs=%d sub=%+v",
			m.seqs["s-b"], m.epochs["s-b"], m.subs["s-b"])
	}
}

func TestJournalResetOfBackgroundSessionKeepsForegroundWatermark(t *testing.T) {
	m := &model{
		seqs:    map[string]int{"bg": 40, "fg": 9},
		epochs:  map[string]int64{"bg": 2, "fg": 2},
		subs:    map[string]*muxSub{"bg": {lastSeq: 40}, "fg": {lastSeq: 9}},
		cur:     "fg",
		lastSeq: 9,
	}
	m.mu.Lock()
	got := m.onJournalResetLocked("bg")
	m.mu.Unlock()

	if got != 0 {
		t.Fatalf("后台会话换代也要返回 0，实际 %d", got)
	}
	if _, ok := m.seqs["bg"]; ok {
		t.Fatal("后台会话的水位没清")
	}
	if m.lastSeq != 9 {
		t.Fatalf("清后台会话不该动前台水位：lastSeq=%d", m.lastSeq)
	}
	if m.seqs["fg"] != 9 || m.subs["fg"].lastSeq != 9 {
		t.Fatal("清后台会话不该动另一条连接的水位")
	}
}

// TestJournalResetToleratesMissingState：第一次订阅就换代时，这条会话还没进过
// seqs/epochs/subs（map 可能是 nil）。清零动作不能 panic——它跑在订阅 goroutine 里，
// panic 等于整条流静默死掉。
func TestJournalResetToleratesMissingState(t *testing.T) {
	m := &model{cur: "s-new"}
	m.mu.Lock()
	got := m.onJournalResetLocked("s-new")
	m.mu.Unlock()
	if got != 0 {
		t.Fatalf("空状态也要返回 0，实际 %d", got)
	}
}
