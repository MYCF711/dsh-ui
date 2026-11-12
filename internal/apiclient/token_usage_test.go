package apiclient

// token_usage_test.go —— `tokenUsage` 四桶与 `contextPressure` 三字段的**未知语义**。
//
// 判据（task-59 §任务 1/3/4）：
//
//	① 四桶逐个解析（累计值原样，不做加总/换算）；
//	② 键缺席 = 未知（不是 0）；键存在但四桶全 0 = **已知的 0**（两态必须可区分）；
//	③ 官方合计公式：total = uncached + cacheRead + cacheWrite + output；
//	④ contextPressure 三字段各自可选；占用百分比是客户端算的
//	   （官方 `context-occupancy.ts:15-25`），缺 used 或 window 就是未知。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// projectionsServer 起一个返回固定 values 的 httptest 服务。
func projectionsServer(t *testing.T, values string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"asOfSeq":42,"values":` + values + `}`))
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL)
}

// ① 四桶逐个解析：四个数各自独立，任何一个是别的桶的值都算错。
func TestTokenUsageParsesFourBuckets(t *testing.T) {
	client := projectionsServer(t, `{
		"tokenUsage": {
			"uncachedInputTokens": 1500,
			"outputTokens": 123,
			"cacheReadTokens": 4096,
			"cacheWriteTokens": 7
		}
	}`)
	proj, err := client.SessionProjections(context.Background(), "s1")
	if err != nil {
		t.Fatalf("session.projections 失败：%v", err)
	}
	usage, ok := proj.TokenUsage()
	if !ok || usage == nil {
		t.Fatal("tokenUsage 键存在时必须判为已知")
	}
	cases := []struct {
		name string
		got  int64
		want int64
	}{
		{"uncachedInputTokens", usage.UncachedInputTokens, 1500},
		{"outputTokens", usage.OutputTokens, 123},
		{"cacheReadTokens", usage.CacheReadTokens, 4096},
		{"cacheWriteTokens", usage.CacheWriteTokens, 7},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Fatalf("%s = %d，want %d（四桶逐个解析）", tc.name, tc.got, tc.want)
		}
	}
	// ③ 官方合计：prompt 侧三桶 + 输出（StatsPills.tsx:311）。
	if got, want := usage.BilledInputTokens(), int64(1500+4096+7); got != want {
		t.Fatalf("billedInputTokens = %d，want %d（三桶之和，不含输出）", got, want)
	}
	if got, want := usage.TotalTokens(), int64(1500+4096+7+123); got != want {
		t.Fatalf("total = %d，want %d", got, want)
	}
}

// ② 键缺席 = 未知，**不是四桶全 0**。
func TestTokenUsageMissingKeyIsUnknown(t *testing.T) {
	client := projectionsServer(t, `{"sessionStats":{"turns":1}}`)
	proj, err := client.SessionProjections(context.Background(), "s1")
	if err != nil {
		t.Fatalf("session.projections 失败：%v", err)
	}
	if usage, ok := proj.TokenUsage(); ok || usage != nil {
		t.Fatalf("键缺席必须判为未知，得到 ok=%v usage=%+v", ok, usage)
	}
	// 字面 null 同样是"没有这个能力"（与 subagentCatalog 同一口径）。
	nullClient := projectionsServer(t, `{"tokenUsage":null}`)
	nullProj, err := nullClient.SessionProjections(context.Background(), "s1")
	if err != nil {
		t.Fatalf("session.projections 失败：%v", err)
	}
	if usage, ok := nullProj.TokenUsage(); ok || usage != nil {
		t.Fatalf("tokenUsage=null 必须判为未知，得到 ok=%v usage=%+v", ok, usage)
	}
}

// ②' 键存在但四桶全 0 = **已知的 0**：与"未知"是两态，不能互相顶替。
func TestTokenUsageZeroesAreMeasuredNotUnknown(t *testing.T) {
	client := projectionsServer(t, `{"tokenUsage":{
		"uncachedInputTokens":0,"outputTokens":0,"cacheReadTokens":0,"cacheWriteTokens":0}}`)
	proj, err := client.SessionProjections(context.Background(), "s1")
	if err != nil {
		t.Fatalf("session.projections 失败：%v", err)
	}
	usage, ok := proj.TokenUsage()
	if !ok || usage == nil {
		t.Fatal("四桶全 0 但键在，必须判为**已知**（与键缺席相反）")
	}
	if usage.TotalTokens() != 0 || usage.BilledInputTokens() != 0 {
		t.Fatalf("全 0 的四桶合计必须是 0，得到 billed=%d total=%d",
			usage.BilledInputTokens(), usage.TotalTokens())
	}
}

// ④ contextPressure 三字段各自可选，缺就是缺（不补 0）。
func TestContextPressureOptionalFields(t *testing.T) {
	client := projectionsServer(t, `{"contextPressure":{
		"contextWindow":1000000,"pressureTokens":62000,"projectedTokens":65000}}`)
	proj, err := client.SessionProjections(context.Background(), "s1")
	if err != nil {
		t.Fatalf("session.projections 失败：%v", err)
	}
	pressure, ok := proj.ContextPressure()
	if !ok || pressure == nil {
		t.Fatal("contextPressure 键存在时必须判为已知")
	}
	for _, tc := range []struct {
		name string
		got  *int64
		want int64
	}{
		{"contextWindow", pressure.ContextWindow, 1000000},
		{"pressureTokens", pressure.PressureTokens, 62000},
		{"projectedTokens", pressure.ProjectedTokens, 65000},
	} {
		if tc.got == nil || *tc.got != tc.want {
			t.Fatalf("%s = %v，want %d", tc.name, tc.got, tc.want)
		}
	}
	// 官方 `contextOccupancy`：used 优先 projectedTokens（`??` 语义），百分比四舍五入。
	used, window, percent, ok := pressure.Occupancy()
	if !ok || used != 65000 || window != 1000000 || percent != 7 {
		t.Fatalf("occupancy = (%d, %d, %d%%, %v)，want (65000, 1000000, 7%%, true)", used, window, percent, ok)
	}

	// 只有 contextWindow（宿主给 `{}` 或只给容量）→ 占用未知，不写 0%。
	partial := projectionsServer(t, `{"contextPressure":{"contextWindow":1000000}}`)
	partialProj, err := partial.SessionProjections(context.Background(), "s1")
	if err != nil {
		t.Fatalf("session.projections 失败：%v", err)
	}
	partialPressure, ok := partialProj.ContextPressure()
	if !ok || partialPressure == nil {
		t.Fatal("contextPressure 键存在时必须判为已知（哪怕只有容量）")
	}
	if partialPressure.PressureTokens != nil || partialPressure.ProjectedTokens != nil {
		t.Fatalf("缺的字段必须保持 nil：%+v", partialPressure)
	}
	if _, _, _, ok := partialPressure.Occupancy(); ok {
		t.Fatal("缺 usedTokens 时占用必须未知（官方 return null），不得按 0% 画")
	}

	// 键缺席 → 未知。
	absent := projectionsServer(t, `{"sessionStats":{"turns":1}}`)
	absentProj, err := absent.SessionProjections(context.Background(), "s1")
	if err != nil {
		t.Fatalf("session.projections 失败：%v", err)
	}
	if pressure, ok := absentProj.ContextPressure(); ok || pressure != nil {
		t.Fatalf("键缺席必须判为未知，得到 ok=%v pressure=%+v", ok, pressure)
	}
}

// ④' 上下文占用百分比按官方 `??` 回退：没有 projectedTokens 时用 pressureTokens。
func TestContextOccupancyFallsBackToPressureTokens(t *testing.T) {
	pressure := &ContextPressure{ContextWindow: int64Ptr(200), PressureTokens: int64Ptr(50)}
	used, window, percent, ok := pressure.Occupancy()
	if !ok || used != 50 || window != 200 || percent != 25 {
		t.Fatalf("occupancy = (%d, %d, %d%%, %v)，want (50, 200, 25%%, true)", used, window, percent, ok)
	}
	// 超容量按官方 min(100, …) 夹到 100（不是"不要钳"——官方圆环长度就是这个口径）。
	over := &ContextPressure{ContextWindow: int64Ptr(100), PressureTokens: int64Ptr(140)}
	if _, _, percent, ok := over.Occupancy(); !ok || percent != 100 {
		t.Fatalf("超容量占比 = %d%%（ok=%v），want 100%%", percent, ok)
	}
	// 容量非正是坏值：按未知处理，不除零、不写 0%。
	zero := &ContextPressure{ContextWindow: int64Ptr(0), PressureTokens: int64Ptr(10)}
	if _, _, _, ok := zero.Occupancy(); ok {
		t.Fatal("容量为 0 时必须判为未知")
	}
}

// int64Ptr 是小工具（测试里造"可选字段"）。
func int64Ptr(v int64) *int64 { return &v }
