package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// changesServer 起一个假宿主，记录收到的最后一次请求，再按脚本回包。
//
// 它**刻意不回信封**：这三条路由的正文就是裸对象（HOST:handlers_changes_routes.go:126-141
// 的 writeChangesJSON）。测试要是拿 `{"result":…}` 去喂，就变成在验一个宿主不发的东西。
func changesServer(t *testing.T, status int, body string, seen *http.Request) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			*seen = *r.Clone(r.Context())
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// changesClient 造一个带 cookie 的客户端——这三条路由和 `POST /api/<method>` 同一套认证，
// 不带 cookie 时真宿主回 401。
func changesClient(t *testing.T, base string) *Client {
	t.Helper()
	c := New(base)
	c.SetCookies([]*http.Cookie{{Name: "dshd_session", Value: "tok"}})
	return c
}

// TestChangesSummaryDecodesBareJSON 验：**裸 JSON**（没有 result 信封）能解出字段，
// 并且请求带上了 cookie（认证路径与 POST /api/<method> 相同）。
func TestChangesSummaryDecodesBareJSON(t *testing.T) {
	var seen http.Request
	srv := changesServer(t, http.StatusOK, `{"turn":3,"files":[{"path":"src/a.go","display":"a.go",
		"added":4,"deleted":1}],"total":1,"added":4,"deleted":1,"cwd":"D:\\w",
		"snapshot":{"before":"t1","after":"t2"}}`, &seen)

	got, err := changesClient(t, srv.URL).ChangesSummary(context.Background(), "s1", 7)
	if err != nil {
		t.Fatalf("裸 JSON 应当成功：%v", err)
	}
	if got.Turn != 3 || got.Total != 1 || got.Added != 4 || got.Deleted != 1 {
		t.Fatalf("摘要字段丢了：%+v", got)
	}
	if len(got.Files) != 1 || got.Files[0].Path != "src/a.go" || got.Files[0].Display != "a.go" {
		t.Fatalf("files[:] 解错：%+v", got.Files)
	}
	if got.Snapshot == nil || got.Snapshot.Before != "t1" || got.Snapshot.After != "t2" {
		t.Fatalf("snapshot 丢了：%+v", got.Snapshot)
	}
	if seen.URL.Path != ChangesSummaryRoute {
		t.Fatalf("路径 = %q，want %q（文档相对路由，不是 /api/<method>）", seen.URL.Path, ChangesSummaryRoute)
	}
	if seen.Method != http.MethodGet {
		t.Fatalf("方法 = %q，want GET", seen.Method)
	}
	if got := seen.URL.Query().Get("sessionId"); got != "s1" {
		t.Fatalf("sessionId = %q", got)
	}
	if got := seen.URL.Query().Get("seq"); got != "7" {
		t.Fatalf("seq = %q", got)
	}
	if len(seen.Cookies()) != 1 || seen.Cookies()[0].Value != "tok" {
		t.Fatalf("没带 /connect 换来的 cookie：%+v", seen.Cookies())
	}
}

// TestChangesSummaryRejectsResultEnvelope 验：**信封形状不算摘要**。
//
// 这条守的是"裸 JSON"这个契约本身：`Call` 那种 `{result:…}` 回包在这里必须解不出来，
// 否则"宿主漏挂路由、别的端点接住了"会被当成一份空摘要画出去。
func TestChangesSummaryRejectsResultEnvelope(t *testing.T) {
	srv := changesServer(t, http.StatusOK,
		`{"result":{"turn":1,"files":[],"total":0,"added":0,"deleted":0}}`, nil)

	if got, err := changesClient(t, srv.URL).ChangesSummary(context.Background(), "s1", 7); err == nil {
		t.Fatalf("信封被当成了摘要：%+v", got)
	}
}

// TestChangesDiffDecodesTextUnion 验：text 支的 hunk 逐字段解出来，index 进查询串。
func TestChangesDiffDecodesTextUnion(t *testing.T) {
	var seen http.Request
	srv := changesServer(t, http.StatusOK, `{"kind":"text","path":"src/a.go","display":"a.go",
		"before":true,"after":true,"coarse":false,
		"hunks":[{"oldStart":1,"oldLines":2,"newStart":1,"newLines":3,
			"lines":[" one","-two","+two bis","+three"]}]}`, &seen)

	got, err := changesClient(t, srv.URL).ChangesDiff(context.Background(), "s1", 7, 2)
	if err != nil {
		t.Fatalf("text diff 应当成功：%v", err)
	}
	if got.Kind != "text" || !got.Before || !got.After || got.Coarse {
		t.Fatalf("text 支字段解错：%+v", got)
	}
	if len(got.Hunks) != 1 || got.Hunks[0].OldStart != 1 || got.Hunks[0].NewStart != 1 ||
		got.Hunks[0].OldLines != 2 || got.Hunks[0].NewLines != 3 || len(got.Hunks[0].Lines) != 4 {
		t.Fatalf("hunk 解错：%+v", got.Hunks)
	}
	if got := seen.URL.Query().Get("index"); got != "2" {
		t.Fatalf("index = %q", got)
	}
	if seen.URL.Path != ChangesDiffRoute {
		t.Fatalf("路径 = %q", seen.URL.Path)
	}
}

// TestChangesDiffDecodesRefusalUnion 验：binary / oversized 两支只有 path/display，
// **不补零值 hunks**（补了界面就会画出一个空 diff）。
func TestChangesDiffDecodesRefusalUnion(t *testing.T) {
	for _, kind := range []string{"binary", "oversized"} {
		srv := changesServer(t, http.StatusOK,
			`{"kind":"`+kind+`","path":"a.bin","display":"a.bin"}`, nil)
		got, err := changesClient(t, srv.URL).ChangesDiff(context.Background(), "s1", 7, 0)
		if err != nil {
			t.Fatalf("%s 应当成功：%v", kind, err)
		}
		if got.Kind != kind || got.Path != "a.bin" {
			t.Fatalf("%s 支解错：%+v", kind, got)
		}
		if got.Hunks != nil || got.Before || got.After || got.Coarse {
			t.Fatalf("%s 支不该有 hunk/before/after：%+v", kind, got)
		}
	}
}

// TestChangesDiffRejectsUnknownKindAndBadLines 验：未知 kind、以及没有 +/-/空格
// 前缀的 hunk 行都**报错**（官方 isChangesDiff 同样判 false → 客户端显示"无法读取改动"）。
// 报错好过画出看不懂的行。
func TestChangesDiffRejectsUnknownKindAndBadLines(t *testing.T) {
	cases := map[string]string{
		"未知 kind": `{"kind":"mystery","path":"a","display":"a"}`,
		"无 path":  `{"kind":"text","path":"","display":"a","before":true,"after":true,"hunks":[]}`,
		"行前缀坏": `{"kind":"text","path":"a","display":"a","before":true,"after":true,
			"hunks":[{"oldStart":1,"oldLines":1,"newStart":1,"newLines":1,"lines":["@@ x @@"]}]}`,
		"空行": `{"kind":"text","path":"a","display":"a","before":true,"after":true,
			"hunks":[{"oldStart":1,"oldLines":1,"newStart":1,"newLines":1,"lines":[""]}]}`,
	}
	for name, body := range cases {
		srv := changesServer(t, http.StatusOK, body, nil)
		if got, err := changesClient(t, srv.URL).ChangesDiff(context.Background(), "s1", 7, 0); err == nil {
			t.Fatalf("%s 应当报错，得到 %+v", name, got)
		}
	}
}

// TestChangesRoutesClassifyStatusCodes 验五档状态码分类，以及**四字段裸错误体**能解出来。
func TestChangesRoutesClassifyStatusCodes(t *testing.T) {
	cases := []struct {
		status int
		kind   ChangesStatus
		body   string
	}{
		{http.StatusBadRequest, ChangesStatusBadRequest,
			`{"error":{"code":"bad_request","message":"需要非负整数 seq","hint":"","retryable":false}}`},
		{http.StatusNotFound, ChangesStatusMissing,
			`{"error":{"code":"not_found","message":"这个会话在 seq=9 没有 workspace/changes 记录","hint":"","retryable":false}}`},
		{http.StatusUnprocessableEntity, ChangesStatusNativeUnavailable,
			`{"error":{"code":"not_implemented","message":"本机没有原生打开通道","hint":"","retryable":false}}`},
		{http.StatusNotImplemented, ChangesStatusNotWired,
			`{"error":{"code":"not_implemented","message":"changes.summary 未接线","hint":"","retryable":false}}`},
		{http.StatusInternalServerError, ChangesStatusFailed,
			`{"error":{"code":"internal","message":"读记录失败","hint":"稍后重试","retryable":true}}`},
	}
	for _, tc := range cases {
		srv := changesServer(t, tc.status, tc.body, nil)
		_, err := changesClient(t, srv.URL).ChangesSummary(context.Background(), "s1", 9)
		if err == nil {
			t.Fatalf("HTTP %d 应当报错", tc.status)
		}
		var ce *ChangesError
		if !errors.As(err, &ce) {
			t.Fatalf("HTTP %d 的错误类型 = %T（want *ChangesError）：%v", tc.status, err, err)
		}
		if ce.Kind != tc.kind || ce.Status != tc.status {
			t.Fatalf("HTTP %d 分类 = %v/%d，want %v", tc.status, ce.Kind, ce.Status, tc.kind)
		}
		if ce.Code == "" || ce.Message == "" {
			t.Fatalf("HTTP %d 四字段没解出来：%+v", tc.status, ce)
		}
	}
}

// TestChangesErrorRetryableAndHint 验四字段里的 hint/retryable 是**宿主说了算**，
// 客户端不按状态码自己猜。
func TestChangesErrorRetryableAndHint(t *testing.T) {
	srv := changesServer(t, http.StatusInternalServerError,
		`{"error":{"code":"internal","message":"算 diff 失败","hint":"稍后重试","retryable":true}}`, nil)
	_, err := changesClient(t, srv.URL).ChangesDiff(context.Background(), "s1", 9, 0)
	var ce *ChangesError
	if !errors.As(err, &ce) {
		t.Fatalf("错误类型 = %T", err)
	}
	if !ce.Retryable || ce.Hint != "稍后重试" || ce.Code != "internal" {
		t.Fatalf("四字段解错：%+v", ce)
	}
	// 404 不是"重试能好"的那一类：IsChangesMissing 是界面退全文预览的唯一判据。
	srvMissing := changesServer(t, http.StatusNotFound, `{"error":{"code":"not_found","message":"没有记录"}}`, nil)
	_, err = changesClient(t, srvMissing.URL).ChangesSummary(context.Background(), "s1", 9)
	if !IsChangesMissing(err) {
		t.Fatalf("404 没被认成 missing：%v", err)
	}
	if IsChangesMissing(nil) {
		t.Fatal("nil 不该算 missing")
	}
}

// TestChangesOpenSendsActionAndApplication 验开文件那条 POST 的拼参：
// 默认不带 action、reveal 才带 action=reveal、application 非空才带。
func TestChangesOpenSendsActionAndApplication(t *testing.T) {
	cases := []struct {
		action, application string
		wantAction          string
		wantApp             string
	}{
		{"", "", "", ""},
		{"reveal", "", "reveal", ""},
		{"", "vscode", "", "vscode"},
	}
	for _, tc := range cases {
		var seen http.Request
		srv := changesServer(t, http.StatusOK, `{"opened":true}`, &seen)
		if err := changesClient(t, srv.URL).ChangesOpen(context.Background(), "s1", 7, 1, tc.action, tc.application); err != nil {
			t.Fatalf("open 应当成功：%v", err)
		}
		if seen.Method != http.MethodPost {
			t.Fatalf("open 方法 = %q，want POST", seen.Method)
		}
		if seen.URL.Path != ChangesOpenRoute {
			t.Fatalf("open 路径 = %q", seen.URL.Path)
		}
		if got := seen.URL.Query().Get("action"); got != tc.wantAction {
			t.Fatalf("action = %q，want %q", got, tc.wantAction)
		}
		if got := seen.URL.Query().Get("application"); got != tc.wantApp {
			t.Fatalf("application = %q，want %q", got, tc.wantApp)
		}
		if got := seen.URL.Query().Get("index"); got != "1" {
			t.Fatalf("index = %q", got)
		}
	}
}

// TestChangesOpenNativeUnavailable 验 422 被认成"本机没有原生打开通道"，
// 而不是一个可重试的失败。
func TestChangesOpenNativeUnavailable(t *testing.T) {
	srv := changesServer(t, http.StatusUnprocessableEntity,
		`{"error":{"code":"not_implemented","message":"本机没有原生打开通道"}}`, nil)
	err := changesClient(t, srv.URL).ChangesOpen(context.Background(), "s1", 7, 0, "", "")
	if !IsChangesNativeUnavailable(err) {
		t.Fatalf("422 没被认成 nativeUnavailable：%v", err)
	}
}

// TestChangesErrorAcceptsFlatAndStringShapes 验容错：顶层摊平的四字段、
// 以及 `{"error":"unauthorized"}` 那种老形状都能解，且都**不套 result 信封**。
func TestChangesErrorAcceptsFlatAndStringShapes(t *testing.T) {
	flat := changesServer(t, http.StatusInternalServerError,
		`{"code":"internal","message":"炸了","hint":"","retryable":false}`, nil)
	_, err := changesClient(t, flat.URL).ChangesSummary(context.Background(), "s1", 1)
	var ce *ChangesError
	if !errors.As(err, &ce) || ce.Code != "internal" || ce.Message != "炸了" {
		t.Fatalf("摊平四字段没解出来：%v", err)
	}

	str := changesServer(t, http.StatusUnauthorized, `{"error":"unauthorized"}`, nil)
	_, err = changesClient(t, str.URL).ChangesSummary(context.Background(), "s1", 1)
	if !errors.As(err, &ce) || ce.Code != "unauthorized" {
		t.Fatalf("字符串 error 没解出来：%v", err)
	}
	if ce.Kind != ChangesStatusFailed {
		t.Fatalf("401 分类 = %v（want Failed：没登记的码不猜）", ce.Kind)
	}

	// 连 JSON 都不是（代理回了一页 HTML）：错误里要能看见原文，不能只报状态码。
	html := changesServer(t, http.StatusBadGateway, `<html>bad gateway</html>`, nil)
	_, err = changesClient(t, html.URL).ChangesSummary(context.Background(), "s1", 1)
	if !errors.As(err, &ce) || ce.Message != "<html>bad gateway</html>" {
		t.Fatalf("非 JSON 错误体没带原文：%v", err)
	}
}

// TestChangesSummaryEmptyFilesStaysNil 验：宿主回空 files 时**原样**空，
// 客户端不替宿主造一个空切片（404 与"空列表"是两件事，都不许在这里被抹平）。
func TestChangesSummaryEmptyFilesStaysNil(t *testing.T) {
	srv := changesServer(t, http.StatusOK, `{"turn":1,"total":0,"added":0,"deleted":0}`, nil)
	got, err := changesClient(t, srv.URL).ChangesSummary(context.Background(), "s1", 1)
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if got.Files != nil {
		t.Fatalf("客户端不该伪造空切片：%+v", got.Files)
	}
	raw, _ := json.Marshal(got)
	if got := string(raw); got == "" {
		t.Fatal("不可达")
	}
}
