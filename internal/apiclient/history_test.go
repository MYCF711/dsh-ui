// history_test.go —— 断号补洞的客户端自测。
//
// 为什么必须钉住 sinceSeq 的传法：宿主实测语义是**开区间**
// （`sinceSeq:166` → frames 167 起，见 docs/reconnect.md），而 gap 区间是闭区间
// `[from,to]`。传错一个数就会少补一帧或多补一帧，而且这种错在界面上看不出来。
package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHistoryParsesFrames(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"result":{"frames":[
			{"seq":167,"type":"assistant/message","data":{"content":[]},"time":1},
			{"seq":168,"type":"turn/done","data":{},"time":2}
		],"lastSeq":277,"journalEpoch":1}}`))
	}))
	defer srv.Close()

	res, err := New(srv.URL).History(context.Background(), "s-1", 166, 2, 1)
	if err != nil {
		t.Fatalf("history 不该报错：%v", err)
	}
	if len(res.Frames) != 2 || res.Frames[0].Seq != 167 || res.Frames[1].Type != "turn/done" {
		t.Fatalf("帧解析不对：%+v", res.Frames)
	}
	if res.LastSeq != 277 || res.JournalEpoch != 1 {
		t.Fatalf("lastSeq/journalEpoch 要带上：%+v", res)
	}
	// 传进去的就是传出来的（调用方自己决定语义），别在这里偷偷 +1/-1。
	if int(gotBody["sinceSeq"].(float64)) != 166 {
		t.Fatalf("sinceSeq 应原样发送，实际 %v", gotBody["sinceSeq"])
	}
	if int(gotBody["journalEpoch"].(float64)) != 1 {
		t.Fatalf("journalEpoch 应原样发送，实际 %v", gotBody["journalEpoch"])
	}
}

// TestHistorySendsJournalEpoch 钉住「补洞要报世代」这条契约（U0 剩余项之一）。
//
// 宿主拿它判"你手里的水位是不是这一代的"（`server.go:497`：
// `in.JournalEpoch != 0 && in.JournalEpoch != j.Epoch()` → 409 stale_epoch）。
// 不带这个字段时宿主只能按 seq 硬补，重启换 journal 之后补回来的是一段
// **错位**的数据：不报错、也不丢帧，只是内容对不上号——界面上看不出来。
func TestHistorySendsJournalEpoch(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"result":{"frames":[],"lastSeq":0,"journalEpoch":3}}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL).History(context.Background(), "s-1", 9, 0, 3); err != nil {
		t.Fatalf("history 不该报错：%v", err)
	}
	// 字段必须**存在**，不是"值对就算"：缺席和值错在宿主那边都表现为 409，
	// 但缺席是客户端漏了契约，值错是水位本身过期——两件事得分开钉。
	raw, ok := gotBody["journalEpoch"]
	if !ok {
		t.Fatalf("请求 body 里没有 journalEpoch 字段：%v", gotBody)
	}
	if int(raw.(float64)) != 3 {
		t.Fatalf("journalEpoch 应原样发送 3，实际 %v", raw)
	}
	// limit<=0 时补默认值，但 sinceSeq 与 id 不能被动过。
	if int(gotBody["sinceSeq"].(float64)) != 9 || gotBody["id"] != "s-1" {
		t.Fatalf("sinceSeq/id 也要一起发：%v", gotBody)
	}
	if int(gotBody["limit"].(float64)) != 200 {
		t.Fatalf("limit<=0 应补默认 200，实际 %v", gotBody["limit"])
	}
}

// TestFillGapForwardsEpoch 钉住「FillGap 把它知道的世代传下去」。
//
// 分批补洞时**每一批**都要带同一个世代：只带第一批的话，第二批就是拿旧水位
// 去问新 journal 的帧，宿主照样给 409（或者更糟：静默错位）。
func TestFillGapForwardsEpoch(t *testing.T) {
	var epochs []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		epochs = append(epochs, int(body["journalEpoch"].(float64)))
		since := int(body["sinceSeq"].(float64))
		limit := int(body["limit"].(float64))
		var frames []map[string]any
		for i := since + 1; i <= since+limit && i <= 500; i++ {
			frames = append(frames, map[string]any{"seq": i, "type": "x", "time": 1})
		}
		out, _ := json.Marshal(map[string]any{"result": map[string]any{
			"frames": frames, "lastSeq": 500, "journalEpoch": 7,
		}})
		_, _ = w.Write(out)
	}))
	defer srv.Close()

	n, err := FillGap(context.Background(), New(srv.URL), "s-1", 1, 500, 7, func(Frame) {})
	if err != nil {
		t.Fatalf("不该报错：%v", err)
	}
	if n != 500 {
		t.Fatalf("应补 500 帧，实际 %d", n)
	}
	if len(epochs) < 3 {
		t.Fatalf("500 帧要分批（≥3 次），实际 %d 次", len(epochs))
	}
	for i, e := range epochs {
		if e != 7 {
			t.Fatalf("第 %d 批带错了世代：%d，期望 7", i+1, e)
		}
	}
}

func TestFillGapOnlyAppliesRequestedRange(t *testing.T) {
	// 宿主可能多给（它按 limit 返回），补洞只该吃 [from,to]。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		// 断言传的是 from-1（开区间语义）
		if int(body["sinceSeq"].(float64)) != 9 {
			t.Errorf("FillGap 应传 from-1=9，实际 %v", body["sinceSeq"])
		}
		_, _ = w.Write([]byte(`{"result":{"frames":[
			{"seq":9,"type":"late","time":1},
			{"seq":10,"type":"a","time":2},
			{"seq":11,"type":"b","time":3},
			{"seq":12,"type":"beyond","time":4}
		],"lastSeq":12}}`))
	}))
	defer srv.Close()

	var got []int
	n, err := FillGap(context.Background(), New(srv.URL), "s-1", 10, 11, 1, func(f Frame) { got = append(got, f.Seq) })
	if err != nil {
		t.Fatalf("不该报错：%v", err)
	}
	if n != 2 || len(got) != 2 || got[0] != 10 || got[1] != 11 {
		t.Fatalf("只该补 10,11 两帧，得到 n=%d got=%v", n, got)
	}
}

func TestFillGapSurfacesStaleEpoch(t *testing.T) {
	// 宿主重启后 journal 换代：补不了就是补不了，必须冒泡让调用方知道。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":{"code":"stale_epoch","message":"journal 已换代","hint":"重新 load","retryable":false}}`))
	}))
	defer srv.Close()

	_, err := FillGap(context.Background(), New(srv.URL), "s-1", 10, 11, 1, func(Frame) {})
	if err == nil {
		t.Fatal("stale_epoch 必须冒泡")
	}
}

// TestFillGapBatchesLargeGap 钉住"缺口比单次上限大时要分批补"。
//
// 早先只调一次 history，大缺口会静默漏掉中段——界面上看不出少了东西，
// 用户只会觉得"翻上去怎么缺了几条"。
func TestFillGapBatchesLargeGap(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		since := int(body["sinceSeq"].(float64))
		limit := int(body["limit"].(float64))
		var frames []map[string]any
		for i := since + 1; i <= since+limit; i++ {
			if i > 500 {
				break
			}
			frames = append(frames, map[string]any{"seq": i, "type": "x", "time": 1})
		}
		out, _ := json.Marshal(map[string]any{"result": map[string]any{
			"frames": frames, "lastSeq": 500, "journalEpoch": 1,
		}})
		_, _ = w.Write(out)
	}))
	defer srv.Close()

	var got []int
	n, err := FillGap(context.Background(), New(srv.URL), "s-1", 1, 500, 1, func(f Frame) { got = append(got, f.Seq) })
	if err != nil {
		t.Fatalf("不该报错：%v", err)
	}
	if n != 500 || len(got) != 500 {
		t.Fatalf("500 帧的缺口要补齐，得到 n=%d len=%d", n, len(got))
	}
	if calls < 3 {
		t.Fatalf("500 帧应分多批拉（每批 ≤200），实际只调了 %d 次", calls)
	}
	if got[0] != 1 || got[499] != 500 {
		t.Fatalf("首尾不对：%d..%d", got[0], got[len(got)-1])
	}
}
