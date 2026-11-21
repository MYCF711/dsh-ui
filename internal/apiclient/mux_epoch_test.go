package apiclient

import (
	"sync"
	"testing"
)

// TestMuxEpochOfSetAndReadConcurrently 钉「世代回调可以被并发装/读」。
//
// 为什么需要这条：壳在 `newSession` 的 goroutine 里装这个回调，而**每个会话**
// 的订阅 goroutine 会同时在 Subscribe 里读它（`epochLookup`）。旧写法是一个裸
// 导出字段，于是 newSession 与订阅 goroutine 之间就是一处数据竞争——实际后果
// 是 sub 帧可能带上 epoch=0，宿主把「同一代」当成「第一次订阅」，界面在换代
// 之后按旧水位续订（不报错，只是少几条）。
//
// 这条用例在 `-race` 下是**红/绿判据**：去掉 epochMu 就报 DATA RACE。
func TestMuxEpochOfSetAndReadConcurrently(t *testing.T) {
	m := &Mux{}
	m.SetEpochOf(func(string) int64 { return 7 })
	if got := m.epochLookup("session:s1"); got != 7 {
		t.Fatalf("世代回调没生效：%d", got)
	}
	// 没装回调时按「不知道」返回 0（与旧字段 nil 语义一致）。
	if got := (&Mux{}).epochLookup("session:s1"); got != 0 {
		t.Fatalf("没装回调时必须是 0，拿到 %d", got)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 1; i <= 500; i++ {
			n := i
			m.SetEpochOf(func(string) int64 { return int64(n) })
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			if got := m.epochLookup("session:s1"); got < 0 {
				t.Errorf("世代不能是负数：%d", got)
				return
			}
		}
	}()
	wg.Wait()
}
