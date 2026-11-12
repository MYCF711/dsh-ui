// official_retry.go —— `llm.providers.<route>.retryPolicy` 的**可写渲染**。
//
// # 官方原件
//
//	dsh-llm/src/retry-policy.ts:84-98   RetryPolicySchema
//	dsh-llm/src/retry-policy.ts:29      "retryPolicy belongs under each provider configuration"
//	dsh-llm/src/error.ts:17-79          LLM_FAILURE_CODES（retryChoice 标记的 8 个 code）
//	dsh-llm/src/error.ts:76-79          RETRY_CODE_CHOICES（按定义顺序）
//
// # 两个「容易写错成 0」的边界
//
//  1. `backoff.initialDelayMs` / `maxDelayMs` 的 schema 是
//     `z.number().min(Number.MIN_VALUE).max(MAX_TIMER_DELAY_MS)`。
//     `Number.MIN_VALUE` 是 **5e-324**（最小正 double），不是 0——
//     把下界写成 0 会让「0 延迟」通过校验，而官方明确拒绝 0（`<= 0` 就抛）。
//  2. `jitterRatio` 的下界**才是** 0（`min(0).max(1)`）。两者不要混。
//
// 8 个 code 候选（官方定义顺序，`error.ts` 里 `retryChoice: true` 的条目）：
//
//	EMPTY_RESPONSE / MALFORMED_RESPONSE / QUOTA / RATE_LIMIT / SERVER /
//	STREAM_CLOSED / TIMEOUT / TRANSPORT
//
// 注意官方默认 `DEFAULT_RETRYABLE_CODES` 只有 5 个（EMPTY_RESPONSE /
// RATE_LIMIT / SERVER / TIMEOUT / TRANSPORT）——**候选 8 个 ≠ 默认勾选 5 个**，
// 界面必须把 8 个都列出来，未勾选的不许藏。
package dssettings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// 官方 retryPolicy 常量（dsh-llm/src/retry-policy.ts:8-11、:84-98）。
const (
	RetryModeNormal = "normal"
	RetryModeAlways = "always"

	RetryDefaultMaxRetries     = 5
	RetryDefaultInitialDelayMs = 500
	RetryDefaultMaxDelayMs     = 10000
	RetryDefaultJitterRatio    = 0.1

	// RetryMaxTimerDelayMs 是官方 MAX_TIMER_DELAY_MS（2^31-1）。
	RetryMaxTimerDelayMs = 1<<31 - 1
	// RetryMaxSafeInteger 是官方 Number.MAX_SAFE_INTEGER（2^53-1）。
	RetryMaxSafeInteger = 1<<53 - 1
)

// PositiveLowerBound 是官方 `z.number().min(Number.MIN_VALUE)` 的**正下界**：
// 最小正 double = 5e-324。0 与负数都必须被拒绝。
var PositiveLowerBound = math.SmallestNonzeroFloat64

// PositiveLowerBoundText 是官方 `Number.MIN_VALUE.toString()` 的文本形状
// （JS 渲染成 `5e-324`；Go 的 `%g` 会渲染成 4.94…e-324，两者数值相同、
// 文本不同——界面要跟官方一致，所以这里给出确定文本）。
const PositiveLowerBoundText = "5e-324"

// RetryCodeChoices 是官方 8 个可重试 code 候选，**按 error.ts 定义顺序**。
//
// 顺序是契约的一部分：官方 `RETRY_CODE_CHOICES` 由 `Object.keys(LLM_FAILURE_CODES)`
// 过滤而来，顺序即定义顺序；界面按它排列，测试逐位断言。
var RetryCodeChoices = []string{
	"EMPTY_RESPONSE",
	"MALFORMED_RESPONSE",
	"QUOTA",
	"RATE_LIMIT",
	"SERVER",
	"STREAM_CLOSED",
	"TIMEOUT",
	"TRANSPORT",
}

// DefaultRetryableCodes 是官方 `DEFAULT_RETRYABLE_CODES`（5 个，retry-policy.ts:12-18）。
var DefaultRetryableCodes = []string{
	"EMPTY_RESPONSE",
	"RATE_LIMIT",
	"SERVER",
	"TIMEOUT",
	"TRANSPORT",
}

// RetryPolicyView 是一个 provider route 的已解析重试策略。
//
// Known=false 表示**宿主没给 / 给了非法值**：那时界面整组显示未知，
// **绝不回落到默认策略**（"我设了"与"实际跑的"必须是同一个，官方 fail-loud）。
type RetryPolicyView struct {
	Route          string
	Mode           string
	MaxRetries     int
	RetryableCodes []string
	InitialDelayMs float64
	MaxDelayMs     float64
	JitterRatio    float64
	Known          bool
	// Err 是解析失败原因（Known=false 时非空）。
	Err string
}

// ParseRetryPolicyView 解析官方形状的 retryPolicy JSON。
//
// 与 `resolveRetryPolicy` 的差别：**不填默认值**。官方 resolve 会把缺省字段补成
// 默认（500 / 10000 / 0.1 / 5 / 五个 code），但界面要显示的是「宿主实际存的」，
// 补默认会让用户以为盘上写着这些值。所以：缺省字段保留零值 + `Known=true`
// （因为策略本身存在），由渲染层区分「未设置」与「设成了默认值」。
//
// 非法值一律返回 error（fail-loud）：负延迟、空 code 数组、未知键、越界
// maxRetries 全都拒绝，不静默截断。
func ParseRetryPolicyView(route string, raw json.RawMessage) (RetryPolicyView, error) {
	view := RetryPolicyView{Route: route}
	if len(raw) == 0 {
		return RetryPolicyView{Route: route}, fmt.Errorf("retryPolicy 缺失（宿主未给该 provider 的策略）")
	}
	var obj map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&obj); err != nil {
		return view, fmt.Errorf("retryPolicy 不是 JSON 对象: %w", err)
	}
	if obj == nil {
		return view, fmt.Errorf("retryPolicy 必须是 JSON 对象（不能是 null）")
	}

	allowed := map[string]bool{"mode": true, "maxRetries": true, "retryableCodes": true, "backoff": true}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !allowed[k] {
			return view, fmt.Errorf("retryPolicy: unknown key %q（官方只认 mode/maxRetries/retryableCodes/backoff）", k)
		}
	}

	rawMode, ok := obj["mode"]
	if !ok {
		return view, fmt.Errorf(`retryPolicy.mode 必须显式写 "normal" 或 "always"`)
	}
	if err := json.Unmarshal(rawMode, &view.Mode); err != nil {
		return view, fmt.Errorf("retryPolicy.mode 必须是 string: %w", err)
	}
	if view.Mode != RetryModeNormal && view.Mode != RetryModeAlways {
		return view, fmt.Errorf(`retryPolicy.mode 必须是 "normal" 或 "always"（实得 %q）`, view.Mode)
	}

	if rawRetries, ok := obj["maxRetries"]; ok {
		n, err := parseInt(rawRetries)
		if err != nil {
			return view, fmt.Errorf("retryPolicy.maxRetries 必须是整数: %w", err)
		}
		if n < 0 {
			return view, fmt.Errorf("retryPolicy.maxRetries 不能为负数（实得 %d）", n)
		}
		if int64(n) > RetryMaxSafeInteger {
			return view, fmt.Errorf("retryPolicy.maxRetries 超过 MAX_SAFE_INTEGER（实得 %d）", n)
		}
		view.MaxRetries = n
	}
	if rawCodes, ok := obj["retryableCodes"]; ok {
		var codes []string
		if err := json.Unmarshal(rawCodes, &codes); err != nil {
			return view, fmt.Errorf("retryPolicy.retryableCodes 必须是字符串数组: %w", err)
		}
		if len(codes) == 0 {
			return view, fmt.Errorf("retryPolicy.retryableCodes 不能为空数组（那等于永不重试，请删掉这个键）")
		}
		seen := map[string]bool{}
		for _, code := range codes {
			if code == "" {
				return view, fmt.Errorf("retryPolicy.retryableCodes 含空字符串")
			}
			if seen[code] {
				return view, fmt.Errorf("retryPolicy.retryableCodes 含重复值 %q", code)
			}
			seen[code] = true
		}
		view.RetryableCodes = codes
	}
	if rawBackoff, ok := obj["backoff"]; ok {
		if err := parseBackoff(rawBackoff, &view); err != nil {
			return view, err
		}
	}
	view.Known = true
	return view, nil
}

// parseBackoff 解析 `retryPolicy.backoff`（官方 backoffSchema，retry-policy.ts:84-88）。
func parseBackoff(raw json.RawMessage, view *RetryPolicyView) error {
	var obj map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&obj); err != nil {
		return fmt.Errorf("retryPolicy.backoff 必须是 JSON 对象: %w", err)
	}
	if obj == nil {
		return fmt.Errorf("retryPolicy.backoff 必须是 JSON 对象（不能是 null）")
	}
	allowed := map[string]bool{"initialDelayMs": true, "maxDelayMs": true, "jitterRatio": true}
	for k := range obj {
		if !allowed[k] {
			return fmt.Errorf("retryPolicy.backoff: unknown key %q（官方只认 initialDelayMs/maxDelayMs/jitterRatio）", k)
		}
	}
	if rawInitial, ok := obj["initialDelayMs"]; ok {
		v, err := parseFloat(rawInitial)
		if err != nil {
			return fmt.Errorf("retryPolicy.backoff.initialDelayMs 必须是数字: %w", err)
		}
		if err := validatePositiveDelay(v, "initialDelayMs"); err != nil {
			return err
		}
		view.InitialDelayMs = v
	}
	if rawMax, ok := obj["maxDelayMs"]; ok {
		v, err := parseFloat(rawMax)
		if err != nil {
			return fmt.Errorf("retryPolicy.backoff.maxDelayMs 必须是数字: %w", err)
		}
		if err := validatePositiveDelay(v, "maxDelayMs"); err != nil {
			return err
		}
		view.MaxDelayMs = v
	}
	if view.InitialDelayMs > 0 && view.MaxDelayMs > 0 && view.InitialDelayMs > view.MaxDelayMs {
		return fmt.Errorf("retryPolicy.backoff.initialDelayMs 必须小于等于 maxDelayMs（实得 %v > %v）",
			view.InitialDelayMs, view.MaxDelayMs)
	}
	if rawJitter, ok := obj["jitterRatio"]; ok {
		v, err := parseFloat(rawJitter)
		if err != nil {
			return fmt.Errorf("retryPolicy.backoff.jitterRatio 必须是数字: %w", err)
		}
		if math.IsNaN(v) || v < 0 || v > 1 {
			return fmt.Errorf("retryPolicy.backoff.jitterRatio 必须在 [0, 1] 内（实得 %v）", v)
		}
		view.JitterRatio = v
	}
	return nil
}

// validatePositiveDelay 校验延迟是**正**有限数且不超过 MAX_TIMER_DELAY_MS。
//
// 官方原文：`!Number.isFinite(x) || x <= 0 || x > MAX_TIMER_DELAY_MS` → 抛错。
// 注意 `x <= 0`：0 与负数都拒绝。
func validatePositiveDelay(v float64, field string) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return fmt.Errorf("retryPolicy.backoff.%s 必须是正的有限数（官方下界 %s，实得 %v）",
			field, PositiveLowerBoundText, v)
	}
	if v > RetryMaxTimerDelayMs {
		return fmt.Errorf("retryPolicy.backoff.%s 不能超过 MAX_TIMER_DELAY_MS（%d，实得 %v）",
			field, RetryMaxTimerDelayMs, v)
	}
	return nil
}

// DelayWithinBounds 报告一个延迟值是否在官方边界内（含正下界）。
//
// 给渲染层的 stepper 用：按 step 调整后先校验，越界就不提交。
func DelayWithinBounds(v float64) bool {
	return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) && v <= RetryMaxTimerDelayMs
}

// JitterWithinBounds 报告 jitterRatio 是否在 [0,1]。
func JitterWithinBounds(v float64) bool {
	return !math.IsNaN(v) && v >= 0 && v <= 1
}

// RetryPolicyRows 把一个 route 的策略渲染成行。
//
// 行序（与官方表单一致）：
//
//	mode → maxRetries → 8 个 code 候选 → initialDelayMs → maxDelayMs → jitterRatio
//
// `always` 模式下 maxRetries 与 code 行仍然**可见但禁用**：官方 schema 允许在
// always 下保留这些值（`ALWAYS_POLICY_KEYS` 含它们），只是不生效；隐藏会让用户
// 切换回 normal 时发现值没了。
func RetryPolicyRows(route string, view RetryPolicyView, revision int, writable bool) []PanelRow {
	prefix := "retry-" + route + "-"
	known := view.Known
	disabled := !writable || !known
	reason := ""
	if !known {
		if view.Err != "" {
			reason = "未知：" + view.Err
		} else {
			reason = "未知：宿主未提供该 provider 的 retryPolicy"
		}
	} else if !writable {
		reason = "只读：llm 命名空间未开放写入"
	}

	modeRow := PanelRow{
		ID: prefix + "mode", Title: "重试模式", Desc: "normal = 有次数上限；always = 一直重试到成功/取消",
		Kind: ItemSelector, NS: apiclient.SettingsNSLLM,
		Path:     apiclient.RetryPolicyPath(route, "mode"),
		Revision: revision, Known: known, Value: view.Mode, Selected: view.Mode,
		Disabled: disabled, Reason: reason,
		Choices: []PanelChoice{
			{Value: RetryModeNormal, Label: "normal", Desc: "按 maxRetries 与 retryableCodes 重试"},
			{Value: RetryModeAlways, Label: "always", Desc: "每次失败都重试，直到取消"},
		},
	}
	rows := []PanelRow{modeRow}

	normalMode := view.Mode == RetryModeNormal
	inactiveReason := ""
	if known && !normalMode {
		inactiveReason = "always 模式下不生效（官方保留该值）"
	}

	maxRetriesRow := PanelRow{
		ID: prefix + "maxRetries", Title: "最大重试次数", Desc: "首发之后还能重试几次（0 = 不重试）",
		Kind: ItemStepper, NS: apiclient.SettingsNSLLM,
		Path:     apiclient.RetryPolicyPath(route, "maxRetries"),
		Revision: revision, Known: known, Disabled: disabled,
		Bounds: Bounds{HasMin: true, HasMax: true, Min: 0, Max: float64(RetryMaxSafeInteger), Step: 1},
		Reason: reason,
	}
	if known {
		maxRetriesRow.Value = strconv.Itoa(view.MaxRetries)
		if !normalMode {
			maxRetriesRow.Disabled = true
			maxRetriesRow.Reason = inactiveReason
		}
	}
	rows = append(rows, maxRetriesRow)

	selected := map[string]bool{}
	for _, code := range view.RetryableCodes {
		selected[code] = true
	}
	// 8 个官方候选**一个不少**（这是本任务的硬断言之一）；宿主额外写的
	// 自定义 code 追加在官方候选之后，不丢。
	codes := append([]string(nil), RetryCodeChoices...)
	for _, code := range view.RetryableCodes {
		if !containsString(RetryCodeChoices, code) {
			codes = append(codes, code)
		}
	}
	for _, code := range codes {
		row := PanelRow{
			ID: prefix + "code-" + code, Title: code, Desc: "可重试失败码",
			Kind: ItemSwitch, NS: apiclient.SettingsNSLLM,
			Path:     apiclient.RetryPolicyPath(route, "retryableCodes"),
			Revision: revision, Known: known,
			Value:    boolText(selected[code]),
			Selected: boolText(selected[code]),
			Disabled: disabled, Reason: reason,
		}
		if !known {
			row.Value, row.Selected = "", ""
		} else if !normalMode {
			row.Disabled = true
			row.Reason = inactiveReason
		}
		rows = append(rows, row)
	}

	rows = append(rows,
		delayRow(route, "initialDelayMs", "首次重试延迟（毫秒）", view.InitialDelayMs, known, disabled, reason, revision, normalMode),
		delayRow(route, "maxDelayMs", "最大重试延迟（毫秒）", view.MaxDelayMs, known, disabled, reason, revision, normalMode),
		jitterRow(route, view.JitterRatio, known, disabled, reason, revision, normalMode),
	)
	return rows
}

// delayRow 造一行延迟（正下界 5e-324）。
func delayRow(route, field, title string, value float64, known, disabled bool, reason string, revision int, normalMode bool) PanelRow {
	row := PanelRow{
		ID: "retry-" + route + "-" + field, Title: title,
		Desc: "官方边界 (" + PositiveLowerBoundText + ", " + strconv.Itoa(RetryMaxTimerDelayMs) + "] 毫秒",
		Kind: ItemStepper, NS: apiclient.SettingsNSLLM,
		Path:     apiclient.RetryPolicyBackoffPath(route, field),
		Revision: revision, Known: known, Disabled: disabled, Reason: reason,
		Bounds: Bounds{
			HasMin: true, HasMax: true,
			Min: PositiveLowerBound, Max: float64(RetryMaxTimerDelayMs), Step: 1,
		},
	}
	if known {
		row.Value = formatDelay(value)
	}
	if known && !normalMode {
		row.Disabled = true
		row.Reason = "always 模式下不生效（官方保留该值）"
	}
	return row
}

// jitterRow 造 jitterRatio 行（下界 0，上界 1）。
func jitterRow(route string, value float64, known, disabled bool, reason string, revision int, normalMode bool) PanelRow {
	row := PanelRow{
		ID: "retry-" + route + "-jitterRatio", Title: "抖动比例",
		Desc: "每次延迟的对称随机范围 [0, 1]",
		Kind: ItemStepper, NS: apiclient.SettingsNSLLM,
		Path:     apiclient.RetryPolicyBackoffPath(route, "jitterRatio"),
		Revision: revision, Known: known, Disabled: disabled, Reason: reason,
		Bounds: Bounds{HasMin: true, HasMax: true, Min: 0, Max: 1, Step: 0.05},
	}
	if known {
		row.Value = formatDelay(value)
	}
	if known && !normalMode {
		row.Disabled = true
		row.Reason = "always 模式下不生效（官方保留该值）"
	}
	return row
}

// RetryPolicyWriteOps 把「改一个字段」翻成 `settings.mutate` 的路径操作。
//
// 为什么走 mutate 而不是 form.set：官方 `settings.form.set` 的 path 是**单段字段
// 名**（`settings-form.md` §4：非单段路径 400 `invalid_path`），而 retryPolicy
// 在 `providers.<route>.retryPolicy` 下面；`settings.mutate` 才是官方支持的
// 路径操作入口（`{op:"set",path:[…],value}`）。
func RetryPolicyWriteOps(route, field string, value any) []apiclient.SettingsOp {
	switch field {
	case "initialDelayMs", "maxDelayMs", "jitterRatio":
		return []apiclient.SettingsOp{apiclient.SettingsSetOp(apiclient.RetryPolicyBackoffPath(route, field), value)}
	default:
		return []apiclient.SettingsOp{apiclient.SettingsSetOp(apiclient.RetryPolicyPath(route, field), value)}
	}
}

// RetryPolicyToggleCodeOps 造「勾选/取消一个 code」的操作。
//
// 官方的 retryableCodes 是**整个数组**（schema 是 array），所以取消勾选要提交
// 完整的新数组；空数组官方拒绝（`min(1)`），所以最后一个 code 不允许取消——
// 调用方必须先判断长度。
func RetryPolicyToggleCodeOps(route string, codes []string, code string) ([]apiclient.SettingsOp, error) {
	next := make([]string, 0, len(codes)+1)
	found := false
	for _, c := range codes {
		if c == code {
			found = true
			continue
		}
		next = append(next, c)
	}
	if !found {
		next = append(next, code)
		sort.SliceStable(next, func(i, j int) bool { return codeRank(next[i]) < codeRank(next[j]) })
	}
	if len(next) == 0 {
		return nil, fmt.Errorf("retryableCodes 不能为空：至少保留一个可重试 code（官方 schema min(1)）")
	}
	return []apiclient.SettingsOp{apiclient.SettingsSetOp(apiclient.RetryPolicyPath(route, "retryableCodes"), next)}, nil
}

// codeRank 按官方定义顺序给 code 排序（自定义 code 排在最后）。
func codeRank(code string) int {
	for i, c := range RetryCodeChoices {
		if c == code {
			return i
		}
	}
	return len(RetryCodeChoices)
}

// RetryPolicyBoundsText 是给界面/报告用的边界文案。
func RetryPolicyBoundsText() string {
	return "maxRetries ∈ [0, " + strconv.Itoa(RetryMaxSafeInteger) + "]；" +
		"initialDelayMs / maxDelayMs ∈ (" + PositiveLowerBoundText + ", " + strconv.Itoa(RetryMaxTimerDelayMs) + "]；" +
		"jitterRatio ∈ [0, 1]；retryableCodes 非空、无重复"
}

// formatDelay 是延迟/jitter 的显示格式：整数就显示整数，否则用最短浮点表示。
func formatDelay(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// boolText 把布尔渲染成官方 wire 文本（`true` / `false`）。
func boolText(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// parseInt 从 json.Number/float64 解整数。
func parseInt(raw json.RawMessage) (int, error) {
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		if i, err := n.Int64(); err == nil {
			return int(i), nil
		}
		if f, err := n.Float64(); err == nil && f == math.Trunc(f) {
			return int(f), nil
		}
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		if f != math.Trunc(f) {
			return 0, fmt.Errorf("不是整数（实得 %v）", f)
		}
		return int(f), nil
	}
	return 0, fmt.Errorf("不是数字")
}

// parseFloat 从 JSON 解浮点。
func parseFloat(raw json.RawMessage) (float64, error) {
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return 0, err
	}
	return f, nil
}
