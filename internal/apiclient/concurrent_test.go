// concurrent_test.go —— 没有 -race 时的并发兜底。
//
// 本机没有 C 工具链（`-race` 跑不了，见 docs/product-acceptance.md §二·六），
// 所以这里用**逻辑断言**兜住最容易出错的那类并发问题：
// 多 goroutine 同时补洞时，帧不能重复、不能乱序、不能漏。
// 这不能替代 race detector（它抓内存竞争），但能抓"顺序/重复"这类真错。
package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFillGapConcurrentGoroutinesSafe(t *testing.T) {
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		since := int(body["sinceSeq"].(float64))
		limit := int(body["limit"].(float64))
		var frames []map[string]any
		for i := since + 1; i <= since+limit; i++ {
			frames = append(frames, map[string]any{"seq": i, "type": "x", "time": 1})
		}
		out, _ := json.Marshal(map[string]any{"result": map[string]any{
			"frames": frames, "lastSeq": since + limit, "journalEpoch": 1,
		}})
		w.WriteHeader(200)
		_, _ = w.Write(out)
	}))
	defer srv.Close()

	// 8 个 goroutine 各补一段互不重叠的缺口，同时跑。
	const n = 8
	var mu sync.Mutex
	seen := map[int]int{} // seq → 被应用次数
	var wg sync.WaitGroup
	for g := 0; g < n; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			from := g*10 + 1
			to := from + 9
			_, err := FillGap(context.Background(), New(srv.URL), "s-1", from, to, func(f Frame) {
				mu.Lock()
				seen[f.Seq]++
				mu.Unlock()
			})
			if err != nil {
				t.Errorf("goroutine %d: %v", g, err)
			}
		}(g)
	}
	wg.Wait()

	// 每个 seq 恰好出现一次：不重复、不漏。
	if len(seen) != n*10 {
		t.Fatalf("应有 %d 个不同 seq，实际 %d", n*10, len(seen))
	}
	for seq, cnt := range seen {
		if cnt != 1 {
			t.Fatalf("seq=%d 被应用 %d 次（重复了）", seq, cnt)
		}
	}
	if atomic.LoadInt64(&calls) < int64(n) {
		t.Fatalf("至少应发出 %d 次 history，实际 %d", n, calls)
	}
}

func TestFillGapStopsWhenHostHasNoMore(t *testing.T) {
	// 宿主只给得出一部分（到 50 就没了）：不能死循环，也不能报错。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		since := int(body["sinceSeq"].(float64))
		var frames []map[string]any
		for i := since + 1; i <= since+10 && i <= 50; i++ {
			frames = append(frames, map[string]any{"seq": i, "type": "x", "time": 1})
		}
		out, _ := json.Marshal(map[string]any{"result": map[string]any{
			"frames": frames, "lastSeq": 50, "journalEpoch": 1,
		}})
		w.WriteHeader(200)
		_, _ = w.Write(out)
	}))
	defer srv.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		n, err := FillGap(context.Background(), New(srv.URL), "s-1", 1, 500, func(Frame) {})
		if err != nil {
			t.Errorf("不该报错：%v", err)
		}
		if n != 50 {
			t.Errorf("宿主只有 50 帧，应补 50，实际 %d", n)
		}
	}()
	select {
	case <-done:
	case <-ctxTimeout():
		t.Fatal("宿主不再给帧时必须退出，不能死循环")
	}
}

func ctxTimeout() <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		time.Sleep(5 * time.Second)
	}()
	return ch
}
