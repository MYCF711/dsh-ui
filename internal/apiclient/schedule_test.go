// schedule_test.go —— 官方 schedule 族客户端的契约测试。
//
// 判据对着宿主 handler（`HOST:internal/transport/handlers_schedule_official.go`）：
//
//	① catalog 是**裸数组**，无参数（发 `{}` 不是不存在的字段）；
//	② edit/run 的 union 三种形态都要能分辨：条目 / {id,code} / {id,deleted:true}；
//	③ miss（schedule_not_found / schedule_conflict / ...）是 HTTP 200 的成功结果，
//	   不是传输错误——把它当错误抛，调用方就永远看不到"任务已在别处被删"这个事实；
//	④ history 的 nextBefore 只在"更旧还有"时出现，空串表示到底了。
package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestScheduleCatalogIsBareArrayWithNoParams(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/schedule.catalog" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("body 不是 JSON：%v", err)
		}
		_, _ = w.Write([]byte(`{"result":[
			{"id":"s1","kind":"daily","title":"晨报","prompt":"汇总文章","time":"09:00",
			 "timeZone":"Asia/Shanghai","scheduledAt":"2026-11-15T09:00:00+08:00",
			 "target":{"kind":"session","sessionId":"sess-1"},
			 "involvedSessionIds":["sess-1"],"status":"active",
			 "createdAt":"2026-11-14T20:00:00+08:00","updatedAt":"2026-11-14T20:00:00+08:00"},
			{"id":"s2","kind":"every","title":"巡检","prompt":"检查",
			 "everySeconds":3600,"scheduledAt":"2026-11-15T00:00:00Z",
			 "target":{"kind":"new"},"involvedSessionIds":[],"status":"paused",
			 "deliveryFloorEndsAt":"2026-11-15T01:00:00Z"}
		]}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).ScheduleCatalog(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if len(body) != 0 {
		t.Fatalf("catalog 的参数必须是空对象，实际 %v", body)
	}
	if len(got) != 2 {
		t.Fatalf("catalog = %d 条，应为 2", len(got))
	}
	if got[0].Kind != "daily" || got[0].Time != "09:00" || got[0].TimeZone != "Asia/Shanghai" {
		t.Fatalf("daily 的时钟字段丢了：%+v", got[0])
	}
	if got[0].Target.Kind != "session" || got[0].Target.SessionID != "sess-1" {
		t.Fatalf("target 丢了：%+v", got[0].Target)
	}
	if got[1].Status != "paused" || got[1].EverySeconds != 3600 {
		t.Fatalf("第 2 条 = %+v", got[1])
	}
	if got[1].DeliveryFloorEndsAt == "" {
		t.Fatal("deliveryFloorEndsAt 丢了：下次运行要按它和 scheduledAt 取后者")
	}
}

func TestScheduleEditUnionDistinguishesDeletedAndMiss(t *testing.T) {
	cases := []struct {
		name string
		body string
		want ScheduleEditOutcome
	}{
		{"deleted", `{"result":{"id":"s1","deleted":true}}`, ScheduleEditOutcome{Deleted: true}},
		{"not-found", `{"result":{"id":"s1","code":"schedule_not_found"}}`, ScheduleEditOutcome{Code: "schedule_not_found"}},
		{"conflict", `{"result":{"id":"s1","code":"schedule_conflict"}}`, ScheduleEditOutcome{Code: "schedule_conflict"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&body)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			got, err := New(srv.URL).ScheduleDelete(context.Background(), "s1")
			if err != nil {
				t.Fatalf("miss 不是错误：%v", err)
			}
			if got != tc.want {
				t.Fatalf("outcome = %+v，want %+v", got, tc.want)
			}
			// 没有 id 就不发请求（本地先挡），这里顺带确认平铺形状。
			if body["id"] != "s1" || body["action"] != "delete" {
				t.Fatalf("入参 = %v", body)
			}
		})
	}
}

func TestScheduleSetActiveSendsPatchAndExpectedUpdatedAt(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/schedule.edit" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":{"id":"s1","kind":"daily","title":"晨报","prompt":"p",
			"time":"09:00","timeZone":"Asia/Shanghai","scheduledAt":"2026-11-15T09:00:00+08:00",
			"target":{"kind":"session","sessionId":"sess-1"},"involvedSessionIds":["sess-1"],
			"status":"paused"}}`))
	}))
	defer srv.Close()

	out, err := New(srv.URL).ScheduleSetActive(context.Background(), "s1", false, "2026-11-14T20:00:00+08:00")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if out.Entry == nil || out.Entry.Status != "paused" {
		t.Fatalf("outcome = %+v", out)
	}
	patch, _ := body["patch"].(map[string]any)
	if patch["active"] != false {
		t.Fatalf("patch.active 没发出去：%v", body)
	}
	if body["expectedUpdatedAt"] != "2026-11-14T20:00:00+08:00" {
		t.Fatalf("乐观锁丢了：%v", body)
	}
}

func TestScheduleHistoryReturnsPageAndMissCode(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":{"id":"s1","records":[
			{"scheduledAt":"2026-11-14T09:00:00+08:00","deliveredAt":"2026-11-14T09:00:03+08:00",
			 "messageId":"m1","targetSessionId":"sess-1","prompt":"晨报"}
		],"earlierRecordsUnavailable":false,"earlierRecordsPruned":true,
		"retention":{"days":30,"records":100},"nextBefore":"m1"}}`))
	}))
	defer srv.Close()

	page, code, err := New(srv.URL).ScheduleHistory(context.Background(), "s1", 20, "")
	if err != nil || code != "" {
		t.Fatalf("err=%v code=%q", err, code)
	}
	if len(page.Records) != 1 || page.Records[0].MessageID != "m1" {
		t.Fatalf("records = %+v", page.Records)
	}
	if !page.EarlierRecordsPruned || page.Retention.Days != 30 || page.NextBefore != "m1" {
		t.Fatalf("page = %+v", page)
	}
	if body["limit"] != float64(20) {
		t.Fatalf("limit = %v", body["limit"])
	}
	if _, ok := body["before"]; ok {
		t.Fatalf("第一页不该带 before：%v", body)
	}
}

func TestScheduleHistoryLimitRangeIsEnforcedLocally(t *testing.T) {
	if _, _, err := New("http://127.0.0.1:1").ScheduleHistory(context.Background(), "s1", 0, ""); err == nil {
		t.Fatal("limit=0 应被本地挡下")
	}
	if _, _, err := New("http://127.0.0.1:1").ScheduleHistory(context.Background(), "s1", 101, ""); err == nil {
		t.Fatal("limit=101 应被本地挡下")
	}
	if _, err := New("http://127.0.0.1:1").ScheduleDelete(context.Background(), " "); err == nil {
		t.Fatal("空 id 应被本地挡下")
	}
}
