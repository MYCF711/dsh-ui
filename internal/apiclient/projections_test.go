// projections_test.go —— `session.projections` 的客户端契约测试（子代理目录的数据面）。
//
// 判据：
//   - 路径 `/api/session.projections`，入参 `{sessionId}`（宿主 DisallowUnknownFields，
//     字段名写错会直接 400，所以这条必须钉住）；
//   - 结果 `{asOfSeq, values}`：**键缺失 = 能力不存在**，不是空数组；
//     本包只解 subagentCatalog 与 teamMode 两个键，其余原样留着不动；
//   - 键存在但值是 `null` → 同样算"未知"（官方 optional 的 null 语义）。
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionProjectionsWireShape(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{
			"asOfSeq": 42,
			"values": {
				"subagentCatalog": [
					{"id":"child-1","createdAt":1762900000000,"mode":"continuable","label":"调研"},
					{"id":"child-2","createdAt":1762900100000,"mode":"continuable","label":""}
				],
				"teamMode": {"enabled": true},
				"sessionStats": {
					"turns": 3, "steps": 4, "llmMs": 1500, "toolMs": 800,
					"ttftMs": 120, "ttftSteps": 2, "decodeMs": 5000, "decodeTokens": 204
				},
				"modelSelection": {
					"lastUsed": {"provider":"deepseek","model":"group/deepseek-v4.1-flash-1017","reasoningEffort":"max"},
					"next": null
				},
				"title": "别的键我们不解码",
				"todos": null
			}
		}`))
	}))
	defer srv.Close()

	proj, err := New(srv.URL).SessionProjections(context.Background(), "sess-root")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotPath != "/api/session.projections" {
		t.Fatalf("路径应为 /api/session.projections，得到 %q", gotPath)
	}
	if body["sessionId"] != "sess-root" {
		t.Fatalf("入参必须是 sessionId（宿主 strict schema），得到 %v", body)
	}
	if proj.AsOfSeq != 42 {
		t.Fatalf("asOfSeq 应透出，得到 %d", proj.AsOfSeq)
	}

	entries, ok := proj.SubagentCatalog()
	if !ok {
		t.Fatal("subagentCatalog 键存在，应判为已知")
	}
	if len(entries) != 2 {
		t.Fatalf("应有 2 条子代理，得到 %d", len(entries))
	}
	if entries[0].ID != "child-1" || entries[0].Label != "调研" || entries[0].Mode != "continuable" {
		t.Fatalf("第 1 条解码不对：%+v", entries[0])
	}
	if entries[0].CreatedAt != 1762900000000 {
		t.Fatalf("createdAt 应原样透出，得到 %d", entries[0].CreatedAt)
	}

	mode, ok := proj.TeamMode()
	if !ok || mode == nil || !mode.Enabled {
		t.Fatalf("teamMode.enabled=true 应解出来，得到 ok=%v mode=%+v", ok, mode)
	}

	// sessionStats：八个字段一个都不能少、不能串位（状态栏「活动」的整份读数）。
	stats, ok := proj.SessionStats()
	if !ok || stats == nil {
		t.Fatal("sessionStats 键存在，应判为已知")
	}
	wantStats := SessionStats{
		Turns: 3, Steps: 4, LLMMs: 1500, ToolMs: 800,
		TTFTMs: 120, TTFTSteps: 2, DecodeMs: 5000, DecodeTokens: 204,
	}
	if *stats != wantStats {
		t.Fatalf("sessionStats 解错：%+v，want %+v", *stats, wantStats)
	}

	// modelSelection：lastUsed 是 Team 开关"关闭前确认"的判据，next 允许 null。
	selection, ok := proj.ModelSelection()
	if !ok || selection == nil {
		t.Fatal("modelSelection 键存在，应判为已知")
	}
	if selection.LastUsed == nil || selection.LastUsed.Model != "group/deepseek-v4.1-flash-1017" ||
		selection.LastUsed.Provider != "deepseek" || selection.LastUsed.ReasoningEffort != "max" {
		t.Fatalf("modelSelection.lastUsed 解错：%+v", selection.LastUsed)
	}
	if selection.Next != nil {
		t.Fatalf("next=null 必须解成 nil（不是空结构体），得到 %+v", selection.Next)
	}
}

func TestSessionProjectionsMissingKeysAreUnknown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 宿主注释明写这三个键可能整个不出现；键缺失 = 该能力不存在。
		_, _ = w.Write([]byte(`{"asOfSeq":-1,"values":{"title":"空会话"}}`))
	}))
	defer srv.Close()

	proj, err := New(srv.URL).SessionProjections(context.Background(), "s-empty")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if _, ok := proj.SubagentCatalog(); ok {
		t.Fatal("subagentCatalog 键缺失时必须回 known=false（不许当成空目录）")
	}
	if _, ok := proj.TeamMode(); ok {
		t.Fatal("teamMode 键缺失时必须回 known=false（不许当成已关闭）")
	}
	if _, ok := proj.SessionStats(); ok {
		t.Fatal("sessionStats 键缺失时必须回 known=false（状态栏写「未知」，不编 0 步）")
	}
	if _, ok := proj.ModelSelection(); ok {
		t.Fatal("modelSelection 键缺失时必须回 known=false（Team 开关禁用）")
	}
}

func TestSessionProjectionsNullValuesAreUnknown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"asOfSeq":7,"values":{"subagentCatalog":null,"teamMode":null,"sessionStats":null,"modelSelection":null}}`))
	}))
	defer srv.Close()

	proj, err := New(srv.URL).SessionProjections(context.Background(), "s1")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if _, ok := proj.SubagentCatalog(); ok {
		t.Fatal("subagentCatalog=null 是“没有”，不是空数组")
	}
	if _, ok := proj.TeamMode(); ok {
		t.Fatal("teamMode=null 是“未知”，不是 enabled=false")
	}
	if _, ok := proj.SessionStats(); ok {
		t.Fatal("sessionStats=null 是“未知”，不是全 0 的真值")
	}
	if _, ok := proj.ModelSelection(); ok {
		t.Fatal("modelSelection=null 是“未知”，不是空选择")
	}
}

// TestSessionProjectionsModelSelectionPresentButNull 钉住一处**容易混淆**的区别：
// `modelSelection` 键存在、但两侧都是 null，是**已知**（这个会话还没选过模型），
// 不是"没有这个能力"。官方 TeamModeToggle 的可用性判据是 `modelSelection !== undefined`，
// 把"键在"当成"键不在"会让开关在本该可用的会话上永远禁用。
func TestSessionProjectionsModelSelectionPresentButNull(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"asOfSeq":9,"values":{"modelSelection":{"lastUsed":null,"next":null}}}`))
	}))
	defer srv.Close()

	proj, err := New(srv.URL).SessionProjections(context.Background(), "s1")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	selection, ok := proj.ModelSelection()
	if !ok || selection == nil {
		t.Fatal("modelSelection 键存在（两侧 null）应判为已知")
	}
	if selection.LastUsed != nil || selection.Next != nil {
		t.Fatalf("两侧都应为 nil，得到 %+v / %+v", selection.LastUsed, selection.Next)
	}
}

func TestSessionProjectionsBadRequestIsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"bad_request","message":"session.projections 需要非空 sessionId","hint":"","retryable":false}}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL).SessionProjections(context.Background(), "")
	if err == nil {
		t.Fatal("空 sessionId 必须报错")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("应是 *APIError，得到 %T", err)
	}
	if apiErr.Code != "bad_request" {
		t.Fatalf("code 应为 bad_request，得到 %q", apiErr.Code)
	}
}

// TestSessionProjectionsSubagentTimingWireShape 钉住宿主 `subagent` 与
// `subagentTiming` 两个键的形状（子代理目录行的 token/时长数据面）。
//
// HOST:internal/transport/handlers_session_official.go:370-393：
//
//	subagent       {mode,label,seq}          —— 子会话自己的身份
//	subagentTiming {settledMs, active?:{since,through}, lastTurnCompleted?}
//
// 三个"未知"都必须是未知：键缺失 / 值为 null / active 缺席。
func TestSessionProjectionsSubagentTimingWireShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"asOfSeq": 7,
			"values": {
				"subagent": {"mode":"continuable","label":"","seq":3},
				"subagentTiming": {
					"settledMs": 65000,
					"active": {"since":1762900000000,"through":1762900005000},
					"lastTurnCompleted": true
				},
				"tokenUsage": {"uncachedInputTokens":100,"outputTokens":20,"cacheReadTokens":400,"cacheWriteTokens":30}
			}
		}`))
	}))
	defer srv.Close()

	proj, err := New(srv.URL).SessionProjections(context.Background(), "child-1")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	identity, ok := proj.SubagentIdentity()
	if !ok || identity.Mode != "continuable" || identity.Seq != 3 {
		t.Fatalf("subagent 投影没解对：%+v（ok=%v）", identity, ok)
	}
	timing, ok := proj.SubagentTiming()
	if !ok {
		t.Fatal("subagentTiming 键在，必须解出来")
	}
	if timing.SettledMs != 65000 || timing.Active == nil || timing.Active.Since != 1762900000000 {
		t.Fatalf("timing 没解对：%+v", timing)
	}
	if timing.LastTurnCompleted == nil || !*timing.LastTurnCompleted {
		t.Fatalf("lastTurnCompleted 没解对：%+v", timing.LastTurnCompleted)
	}
	// 已闭合轮 + 未闭合轮增量：through − since = 5000。
	if ms, ok := timing.DurationMs(0); !ok || ms != 70000 {
		t.Fatalf("DurationMs = (%d,%v)，want (70000,true)", ms, ok)
	}
	usage, ok := proj.TokenUsage()
	if !ok || usage.TotalTokens() != 550 {
		t.Fatalf("tokenUsage 四桶合计 = %v（ok=%v），want 550", usage, ok)
	}
}

// TestSessionProjectionsSubagentTimingMissingIsUnknown：
// 键缺失 / 值为 null 都必须是"未知"，不许当成"没有子代理"或"0 秒"。
func TestSessionProjectionsSubagentTimingMissingIsUnknown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"asOfSeq":1,"values":{"subagent":null,"subagentTiming":null}}`))
	}))
	defer srv.Close()

	proj, err := New(srv.URL).SessionProjections(context.Background(), "root")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if _, ok := proj.SubagentIdentity(); ok {
		t.Fatal("subagent: null 必须解成未知")
	}
	if _, ok := proj.SubagentTiming(); ok {
		t.Fatal("subagentTiming: null 必须解成未知")
	}
}
