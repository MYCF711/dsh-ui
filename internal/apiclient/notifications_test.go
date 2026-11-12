// notifications_test.go —— 官方 rc.2 账号通知三件套的**客户端契约**测试。
//
// 判据来自两处，都不是"我觉得"：
//   - 官方 `dsh-deepseek-account` 的 typert 描述符（参数/结果全字段）
//   - 本仓宿主 DSH-GO `internal/transport/handlers_account_official.go` 的
//     decode/writeJSON 真形状（2026-11-12 在 rc.2 集成 worktree 逐行核对）
//
// 三条硬约束各有用例：
//  1. `client` 是**必填**对象（三个字段都发），不是可选装饰；
//  2. `batch: null` / `content: null` 必须解成 nil，**不能**悄悄变成空批次/零值通知；
//  3. `reportNotificationEvent` 的结果是**裸 boolean**，不是 `{"accepted":…}`。
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// notificationMeta 是官方 AccountClientMetadata 的一份合法值。
func notificationMeta() NotificationClientMetadata {
	return NotificationClientMetadata{Version: "0.2.2-rc.2", Locale: "zh-CN", TimezoneOffsetSeconds: 28800}
}

// TestGetNotificationsSendsClientMetadata 钉住参数面：路径 + 三个 client 字段。
func TestGetNotificationsSendsClientMetadata(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"batch":{"nextPollAfterSeconds":600,"notifications":[]}}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL).GetNotifications(context.Background(), notificationMeta()); err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotPath != "/api/account.getNotifications" {
		t.Fatalf("路径应为 /api/account.getNotifications，得到 %q", gotPath)
	}
	client, ok := body["client"].(map[string]any)
	if !ok {
		t.Fatalf("缺 client 对象（官方 AccountClientMetadata 三个字段必填），body=%v", body)
	}
	for _, key := range []string{"version", "locale", "timezoneOffsetSeconds"} {
		if _, ok := client[key]; !ok {
			t.Fatalf("client 缺 %q，得到 %v", key, client)
		}
	}
	if client["locale"] != "zh-CN" {
		t.Fatalf("locale 必须原样发，得到 %v", client["locale"])
	}
	if off, _ := client["timezoneOffsetSeconds"].(float64); off != 28800 {
		t.Fatalf("timezoneOffsetSeconds 必须原样发，得到 %v", client["timezoneOffsetSeconds"])
	}
}

// TestGetNotificationsDecodesBatch 钉住结果面（含 displayUntil:null 与两条 action）。
func TestGetNotificationsDecodesBatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"batch":{"nextPollAfterSeconds":42.5,"notifications":[
			{"id":"n-1","createdAt":"2026-11-12T00:00:00Z","displayUntil":null,
			 "title":"赠金到账","body":"1000 tokens",
			 "actions":[{"label":"查看","style":"primary","target":{"type":"app_route","name":"account"}},
			            {"label":"关闭","style":"secondary","target":{"type":"dismiss"}}],
			 "data":{"kind":"bonus"}}]}}`))
	}))
	defer srv.Close()

	batch, err := New(srv.URL).GetNotifications(context.Background(), notificationMeta())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if batch == nil {
		t.Fatal("有批次时不该是 nil")
	}
	if batch.NextPollAfterSeconds != 42.5 {
		t.Fatalf("nextPollAfterSeconds 必须原样保留（下一轮间隔由服务端给），得到 %v", batch.NextPollAfterSeconds)
	}
	if len(batch.Notifications) != 1 {
		t.Fatalf("应有 1 条通知，得到 %d", len(batch.Notifications))
	}
	n := batch.Notifications[0]
	if n.ID != "n-1" || n.Title != "赠金到账" || n.Body != "1000 tokens" {
		t.Fatalf("通知字段解错：%+v", n)
	}
	if n.DisplayUntil != nil {
		t.Fatalf("displayUntil:null 表示永不过期，必须是 nil，得到 %q", *n.DisplayUntil)
	}
	if len(n.Actions) != 2 {
		t.Fatalf("应有 2 个动作，得到 %d", len(n.Actions))
	}
	if n.Actions[0].Target.Type != "app_route" || n.Actions[0].Target.Name != "account" {
		t.Fatalf("app_route 目标解错：%+v", n.Actions[0].Target)
	}
	if n.Actions[1].Target.Type != "dismiss" {
		t.Fatalf("dismiss 目标解错：%+v", n.Actions[1].Target)
	}
	if string(n.Data) != `{"kind":"bonus"}` {
		t.Fatalf("data 要原样保留，得到 %s", n.Data)
	}
}

// TestGetNotificationsNullBatchIsNil 是最要紧的一条：官方 null ≠ 空批次。
func TestGetNotificationsNullBatchIsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"batch":null}`))
	}))
	defer srv.Close()

	batch, err := New(srv.URL).GetNotifications(context.Background(), notificationMeta())
	if err != nil {
		t.Fatalf("null 批次不是错误：%v", err)
	}
	if batch != nil {
		t.Fatalf("null 必须解成 nil（未登录/授权在查询中变了），得到 %+v", batch)
	}
}

// TestGetNotificationsUnauthorizedIsAPIError 钉住未授权路径：错误要能被界面认出来。
func TestGetNotificationsUnauthorizedIsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"token revoked","hint":"重新登录","retryable":false}}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL).GetNotifications(context.Background(), notificationMeta())
	if err == nil {
		t.Fatal("401 必须报错")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("应该是 *APIError，得到 %T: %v", err, err)
	}
	if apiErr.Code != "unauthorized" {
		t.Fatalf("code 要原样保留，得到 %q", apiErr.Code)
	}
}

// TestGetNotificationContent 钉住第二件套：{id, client} → {content}。
func TestGetNotificationContent(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"content":{"id":"n-1","createdAt":"2026-11-12T00:00:00Z",
			"displayUntil":"2026-11-13T00:00:00Z","title":"刷新后的标题","body":"","actions":[]}}`))
	}))
	defer srv.Close()

	n, err := New(srv.URL).GetNotificationContent(context.Background(), "n-1", notificationMeta())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if n == nil || n.ID != "n-1" || n.Title != "刷新后的标题" {
		t.Fatalf("内容解错：%+v", n)
	}
	if n.DisplayUntil == nil || *n.DisplayUntil != "2026-11-13T00:00:00Z" {
		t.Fatalf("displayUntil 有值时必须保留，得到 %v", n.DisplayUntil)
	}
	if body["id"] != "n-1" {
		t.Fatalf("id 必须原样发，得到 %v", body["id"])
	}
	if _, ok := body["client"]; !ok {
		t.Fatalf("content 也要带 client，body=%v", body)
	}
}

// TestGetNotificationContentNullIsNil 钉住 content:null。
func TestGetNotificationContentNullIsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":null}`))
	}))
	defer srv.Close()

	n, err := New(srv.URL).GetNotificationContent(context.Background(), "gone", notificationMeta())
	if err != nil {
		t.Fatalf("null 不是错误：%v", err)
	}
	if n != nil {
		t.Fatalf("null 必须解成 nil，得到 %+v", n)
	}
}

// TestReportNotificationEventBareBoolean 钉住第三件套的结果形状：
// 官方是裸 boolean，不是 {"accepted":…}。照错形状写会让成功被当成失败。
func TestReportNotificationEventBareBoolean(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"accepted", `true`, true},
		{"rejected", `false`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&body)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			got, err := New(srv.URL).ReportNotificationEvent(context.Background(), "n-1", NotificationEventDisplayed, notificationMeta())
			if err != nil {
				t.Fatalf("应当成功：%v", err)
			}
			if got != tc.want {
				t.Fatalf("结果应为 %v，得到 %v", tc.want, got)
			}
			if body["event"] != "displayed" || body["id"] != "n-1" {
				t.Fatalf("参数必须原样发，得到 %v", body)
			}
			if _, ok := body["client"]; !ok {
				t.Fatalf("事件上报也要带 client，body=%v", body)
			}
		})
	}
}

// TestNotificationEventNames 钉住三个事件名（官方闭集）。写错一个宿主会 invalid_arg。
func TestNotificationEventNames(t *testing.T) {
	if NotificationEventDisplayed != "displayed" ||
		NotificationEventCTAClicked != "cta_clicked" ||
		NotificationEventDismissed != "dismissed" {
		t.Fatalf("事件名必须与官方逐字相同，得到 %q/%q/%q",
			NotificationEventDisplayed, NotificationEventCTAClicked, NotificationEventDismissed)
	}
}
