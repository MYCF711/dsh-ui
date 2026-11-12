package dssettings

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// TestRetryCodeChoicesAreTheOfficialEight 钉住 8 个 code 候选与官方定义顺序。
//
// 官方 `RETRY_CODE_CHOICES` 由 `Object.keys(LLM_FAILURE_CODES)` 过滤
// `retryChoice: true` 而来（dsh-llm/src/error.ts:17-79），顺序即定义顺序。
func TestRetryCodeChoicesAreTheOfficialEight(t *testing.T) {
	want := []string{
		"EMPTY_RESPONSE", "MALFORMED_RESPONSE", "QUOTA", "RATE_LIMIT",
		"SERVER", "STREAM_CLOSED", "TIMEOUT", "TRANSPORT",
	}
	if len(RetryCodeChoices) != 8 {
		t.Fatalf("code 候选 = %d 个，官方 8 个：%v", len(RetryCodeChoices), RetryCodeChoices)
	}
	for i, code := range want {
		if RetryCodeChoices[i] != code {
			t.Fatalf("第 %d 个 code = %q，官方定义顺序是 %q", i, RetryCodeChoices[i], code)
		}
	}
	// 默认勾选的 5 个必须是候选的子集（候选 8 个 ≠ 默认 5 个）。
	for _, code := range DefaultRetryableCodes {
		if !containsString(RetryCodeChoices, code) {
			t.Fatalf("默认 code %q 不在候选里", code)
		}
	}
	if len(DefaultRetryableCodes) != 5 {
		t.Fatalf("默认 code = %d 个，官方 DEFAULT_RETRYABLE_CODES 是 5 个", len(DefaultRetryableCodes))
	}
}

// TestParseRetryPolicyAcceptsOfficialShape 钉住官方合法值能解出来。
func TestParseRetryPolicyAcceptsOfficialShape(t *testing.T) {
	raw := json.RawMessage(`{
		"mode": "normal",
		"maxRetries": 5,
		"retryableCodes": ["EMPTY_RESPONSE","RATE_LIMIT","SERVER","TIMEOUT","TRANSPORT"],
		"backoff": {"initialDelayMs": 500, "maxDelayMs": 10000, "jitterRatio": 0.1}
	}`)
	view, err := ParseRetryPolicyView("deepseek", raw)
	if err != nil {
		t.Fatalf("官方合法值解析失败: %v", err)
	}
	if !view.Known {
		t.Fatal("Known 应为 true")
	}
	if view.Mode != RetryModeNormal || view.MaxRetries != 5 {
		t.Fatalf("解析结果 = %+v", view)
	}
	if view.InitialDelayMs != 500 || view.MaxDelayMs != 10000 || view.JitterRatio != 0.1 {
		t.Fatalf("backoff 解析 = %+v", view)
	}
	if len(view.RetryableCodes) != 5 {
		t.Fatalf("codes = %v", view.RetryableCodes)
	}
}

// TestParseRetryPolicyRejectsIllegalValues 钉住 fail-loud（不静默截断）。
func TestParseRetryPolicyRejectsIllegalValues(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"缺 mode", `{"maxRetries":1}`, "mode"},
		{"mode 非法", `{"mode":"sometimes"}`, "normal"},
		{"未知键", `{"mode":"normal","extra":1}`, "unknown key"},
		{"负 maxRetries", `{"mode":"normal","maxRetries":-1}`, "负数"},
		{"maxRetries 超 MAX_SAFE_INTEGER", `{"mode":"normal","maxRetries":9007199254740992}`, "MAX_SAFE_INTEGER"},
		{"空 code 数组", `{"mode":"normal","retryableCodes":[]}`, "不能为空数组"},
		{"code 含空串", `{"mode":"normal","retryableCodes":[""]}`, "空字符串"},
		{"code 重复", `{"mode":"normal","retryableCodes":["RATE_LIMIT","RATE_LIMIT"]}`, "重复"},
		{"initialDelayMs = 0（官方正下界拒绝）", `{"mode":"normal","backoff":{"initialDelayMs":0}}`, "正的有限数"},
		{"initialDelayMs 负数", `{"mode":"normal","backoff":{"initialDelayMs":-5}}`, "正的有限数"},
		{"maxDelayMs = 0", `{"mode":"normal","backoff":{"maxDelayMs":0}}`, "正的有限数"},
		{"超过 MAX_TIMER_DELAY_MS", `{"mode":"normal","backoff":{"initialDelayMs":2147483648}}`, "MAX_TIMER_DELAY_MS"},
		{"initial > max", `{"mode":"normal","backoff":{"initialDelayMs":9000,"maxDelayMs":1000}}`, "小于等于"},
		{"jitterRatio 越界", `{"mode":"normal","backoff":{"jitterRatio":1.5}}`, "jitterRatio"},
		{"backoff 未知键", `{"mode":"normal","backoff":{"foo":1}}`, "unknown key"},
		{"null", `null`, "对象"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseRetryPolicyView("route", json.RawMessage(tc.raw))
			if err == nil {
				t.Fatalf("非法值必须报错：%s", tc.raw)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("错误信息 %q 未包含 %q", err.Error(), tc.want)
			}
		})
	}
}

// TestParseRetryPolicyAcceptsPositiveLowerBound 钉住 5e-324 是合法下界。
func TestParseRetryPolicyAcceptsPositiveLowerBound(t *testing.T) {
	raw := json.RawMessage(`{"mode":"normal","backoff":{"initialDelayMs":5e-324,"maxDelayMs":10000}}`)
	view, err := ParseRetryPolicyView("route", raw)
	if err != nil {
		t.Fatalf("5e-324 必须被接受（官方 min(Number.MIN_VALUE)）: %v", err)
	}
	if view.InitialDelayMs != math.SmallestNonzeroFloat64 {
		t.Fatalf("initialDelayMs = %v，应为 5e-324", view.InitialDelayMs)
	}
}

// TestRetryPolicyRowsAreOrderedAndBounded 钉住行序与官方边界。
func TestRetryPolicyRowsAreOrderedAndBounded(t *testing.T) {
	view, err := ParseRetryPolicyView("deepseek", json.RawMessage(`{
		"mode":"normal","maxRetries":3,"retryableCodes":["RATE_LIMIT"],
		"backoff":{"initialDelayMs":500,"maxDelayMs":10000,"jitterRatio":0.1}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	rows := RetryPolicyRows("deepseek", view, 7, true)

	// 行序：mode → maxRetries → 8 code → initialDelayMs → maxDelayMs → jitterRatio。
	if len(rows) != 2+8+3 {
		t.Fatalf("行数 = %d，官方是 2+8+3 = 13：%v", len(rows), rowIDs(rows))
	}
	if rows[0].ID != "retry-deepseek-mode" {
		t.Fatalf("第一行 = %q", rows[0].ID)
	}
	if rows[1].ID != "retry-deepseek-maxRetries" {
		t.Fatalf("第二行 = %q", rows[1].ID)
	}
	for i, code := range RetryCodeChoices {
		if got := rows[2+i].ID; got != "retry-deepseek-code-"+code {
			t.Fatalf("第 %d 个 code 行 = %q，期望 retry-deepseek-code-%s", i, got, code)
		}
	}
	if got := rows[len(rows)-1].ID; got != "retry-deepseek-jitterRatio" {
		t.Fatalf("最后一行 = %q", got)
	}

	// 官方边界：正下界 5e-324 / MAX_TIMER_DELAY_MS / [0,1] / [0, MAX_SAFE_INTEGER]。
	if got := rows[2+8].Bounds; got.Min != math.SmallestNonzeroFloat64 || got.Max != float64(RetryMaxTimerDelayMs) {
		t.Fatalf("initialDelayMs 边界 = %+v", got)
	}
	if got := rows[len(rows)-1].Bounds; got.Min != 0 || got.Max != 1 {
		t.Fatalf("jitterRatio 边界 = %+v", got)
	}
	if got := rows[1].Bounds; got.Min != 0 || got.Max != float64(RetryMaxSafeInteger) {
		t.Fatalf("maxRetries 边界 = %+v", got)
	}
	if !strings.Contains(rows[2+8].Desc, PositiveLowerBoundText) {
		t.Fatalf("延迟行说明必须写出正下界 %s，实得 %q", PositiveLowerBoundText, rows[2+8].Desc)
	}

	// revision 必须带下去（乐观锁种子）。
	for _, row := range rows {
		if row.Revision != 7 {
			t.Fatalf("%s 的 revision = %d，应来自 section.revision=7", row.ID, row.Revision)
		}
	}
	// 写路径：多段路径（providers.<route>.retryPolicy.*）。
	for _, row := range rows[1:] {
		if len(row.Path) < 4 {
			t.Fatalf("%s 的写路径 = %v，retryPolicy 必须是多段路径", row.ID, row.Path)
		}
	}
}

// TestRetryPolicyAlwaysModeDisablesInactiveRows 钉住 always 模式的行态。
func TestRetryPolicyAlwaysModeDisablesInactiveRows(t *testing.T) {
	view, err := ParseRetryPolicyView("route", json.RawMessage(`{"mode":"always"}`))
	if err != nil {
		t.Fatal(err)
	}
	rows := RetryPolicyRows("route", view, 1, true)
	if !rows[0].Known || rows[0].Selected != "always" {
		t.Fatalf("mode 行 = %+v", rows[0])
	}
	if !rows[1].Disabled {
		t.Fatal("always 模式下 maxRetries 应禁用（官方保留值但不生效）")
	}
	if !strings.Contains(rows[1].Reason, "always") {
		t.Fatalf("禁用原因应说明 always 模式：%q", rows[1].Reason)
	}
	for _, row := range rows[2 : 2+len(RetryCodeChoices)] {
		if !row.Disabled {
			t.Fatalf("%s 在 always 模式下应禁用", row.ID)
		}
	}
}

// TestRetryPolicyUnknownIsNotDefaulted 钉住「未知不回落默认策略」。
func TestRetryPolicyUnknownIsNotDefaulted(t *testing.T) {
	rows := RetryPolicyRows("route", RetryPolicyView{Route: "route", Err: "宿主未给"}, 0, true)
	for _, row := range rows {
		if row.Known {
			t.Fatalf("%s 不该有真值（未知不许用默认值冒充）", row.ID)
		}
		if row.Display() != "未知" {
			t.Fatalf("%s 显示 = %q，应为「未知」", row.ID, row.Display())
		}
	}
}

// TestRetryPolicyWriteOpsUseMutatePaths 钉住写路径（providers.<route>.retryPolicy.*）。
func TestRetryPolicyWriteOpsUseMutatePaths(t *testing.T) {
	ops := RetryPolicyWriteOps("deepseek", "maxRetries", 7)
	if len(ops) != 1 || ops[0].Op != "set" {
		t.Fatalf("ops = %+v", ops)
	}
	want := []string{"providers", "deepseek", "retryPolicy", "maxRetries"}
	if strings.Join(ops[0].Path, ".") != strings.Join(want, ".") {
		t.Fatalf("路径 = %v，官方形状是 %v", ops[0].Path, want)
	}
	if ops[0].Value != 7 {
		t.Fatalf("值 = %v", ops[0].Value)
	}

	backoff := RetryPolicyWriteOps("deepseek", "initialDelayMs", 1)
	wantBackoff := []string{"providers", "deepseek", "retryPolicy", "backoff", "initialDelayMs"}
	if strings.Join(backoff[0].Path, ".") != strings.Join(wantBackoff, ".") {
		t.Fatalf("backoff 路径 = %v，官方形状是 %v", backoff[0].Path, wantBackoff)
	}
}

// TestRetryPolicyToggleCodeKeepsOrderAndRejectsEmpty 钉住 code 勾选语义。
func TestRetryPolicyToggleCodeKeepsOrderAndRejectsEmpty(t *testing.T) {
	ops, err := RetryPolicyToggleCodeOps("route", []string{"RATE_LIMIT", "SERVER"}, "TIMEOUT")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := ops[0].Value.([]string)
	if !ok {
		t.Fatalf("值类型 = %T", ops[0].Value)
	}
	want := []string{"RATE_LIMIT", "SERVER", "TIMEOUT"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("新增后 = %v，应按官方定义顺序 = %v", got, want)
	}
	// 取消一个。
	ops, err = RetryPolicyToggleCodeOps("route", []string{"RATE_LIMIT", "SERVER"}, "RATE_LIMIT")
	if err != nil {
		t.Fatal(err)
	}
	got = ops[0].Value.([]string)
	if len(got) != 1 || got[0] != "SERVER" {
		t.Fatalf("取消后 = %v", got)
	}
	// 最后一个不允许取消（官方 min(1)）。
	if _, err := RetryPolicyToggleCodeOps("route", []string{"SERVER"}, "SERVER"); err == nil {
		t.Fatal("清空 retryableCodes 必须报错（官方 schema min(1)）")
	}
}

// TestRetryPolicyBoundsTextIsComplete 钉住边界文案（报告/界面共用）。
func TestRetryPolicyBoundsTextIsComplete(t *testing.T) {
	text := RetryPolicyBoundsText()
	for _, want := range []string{
		"maxRetries", "0", strconv.Itoa(RetryMaxSafeInteger),
		"initialDelayMs", "maxDelayMs", PositiveLowerBoundText,
		strconv.Itoa(RetryMaxTimerDelayMs), "jitterRatio", "retryableCodes",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("边界文案缺 %q：%q", want, text)
		}
	}
}

func rowIDs(rows []PanelRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

var _ = apiclient.SettingsOp{}
