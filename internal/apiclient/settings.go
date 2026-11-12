// settings.go —— HOST-SETTINGS-FORM 的**客户端**（settings.describe / settings.form.*
// / settings.mutate 六个方法）。
//
// 契约唯一来源：`../DSH-GO/docs/transport-contract.md` §3A 与
// `../DSH-GO/docs/settings-form.md`（宿主侧实现与 golden 夹具）。这里只做
// 「按 wire 形状发请求 + 解结果」，不做任何本地默认值：
//
//   - 宿主没给的 namespace **不出现在结果里**（不是给一个空 section）；
//   - field 的 `current` 是 `any` 而不是 string 再转 float——**不许把缺失变成 0**；
//   - revision 是持久乐观锁：冲突回 409 `revision_conflict`，调用方必须把
//     冲突原样交给界面渲染（不准重试覆盖）。
//
// 六个方法都在既有 `POST /api/<method>` 上：裸参数对象、成功 `{result:…}`、
// 失败四字段 `{error:{code,message,hint,retryable}}`。
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// 设置面的命名空间常量（宿主已实现三个；`llm` 里是 `providers.<route>.retryPolicy`）。
const (
	SettingsNSModel      = "model"
	SettingsNSPermission = "permission"
	SettingsNSLLM        = "llm"
)

// 设置面错误码（`settings-form.md` §4 表格，逐条与宿主一致）。
const (
	SettingsCodeUnauthorized     = "unauthorized"
	SettingsCodeBadRequest       = "bad_request"
	SettingsCodeInvalidPath      = "invalid_path"
	SettingsCodeUnknownField     = "unknown_field"
	SettingsCodeInvalidEnum      = "invalid_enum"
	SettingsCodeOutOfRange       = "out_of_range"
	SettingsCodeInvalidType      = "invalid_type"
	SettingsCodeNotFound         = "not_found"
	SettingsCodeRevisionConflict = "revision_conflict"
	SettingsCodeInternal         = "internal"
	// SettingsCodeReadOnly 是只读命名空间的失败码。
	//
	// 宿主当前对 `permission`/`model` 是可写的，所以正常路径不会出现它；但
	// `llm` 之外的后置命名空间（agent-loop / shell / subagent …）会以只读
	// 形式暴露，界面必须能渲染这个失败态，而不是显示「已保存」。
	SettingsCodeReadOnly = "read_only"
)

// SettingsOp 是一次路径写操作（官方 `set` / `unset` 两种）。
//
// **不是整张 descriptor 覆盖**：官方 `settings.mutate(ns, ops, revision)` 只提交
// 路径操作，未提交的字段保持原值。
type SettingsOp struct {
	Op   string   `json:"op"`   // "set" | "unset"
	Path []string `json:"path"` // 单段路径（首切：field 名）
	// Value 只在 op=="set" 时有意义；`unset` 时必须是 nil（宿主拒绝多余 value）。
	Value any `json:"value,omitempty"`
}

// SettingsNamespaceView 是官方 `SettingsNamespaceView`（describe 的 namespaces 一项）。
//
// Schema/Value/Base/User 用 RawMessage：它们是**宿主原始 JSON**，客户端不解释，
// 界面按需再投影。这样「字段缺失」与「字段是 null」不会在客户端被混淆。
type SettingsNamespaceView struct {
	NS           string          `json:"ns"`
	Schema       json.RawMessage `json:"schema,omitempty"`
	Value        json.RawMessage `json:"value,omitempty"`
	Base         json.RawMessage `json:"base,omitempty"`
	User         json.RawMessage `json:"user,omitempty"`
	AutoGenerate bool            `json:"autoGenerate,omitempty"`
	Applies      string          `json:"applies,omitempty"`
	Secrets      []string        `json:"secrets,omitempty"`
	Revision     int             `json:"revision"`
}

// SettingsFieldView 是一个字段的投影（`settings-form.md` §2 的字段清单）。
type SettingsFieldView struct {
	Name       string   `json:"name"`
	Path       []string `json:"path"`
	Type       string   `json:"type"`
	Current    any      `json:"current"`
	Default    any      `json:"default"`
	Overridden bool     `json:"overridden"`
	Writable   bool     `json:"writable"`
	// 以下只在适用时出现（指针保留「没有这个约束」与「约束是 0」的区别）。
	Enum      []string `json:"enum,omitempty"`
	MinLength *int     `json:"minLength,omitempty"`
	MaxLength *int     `json:"maxLength,omitempty"`
}

// SettingsSectionView 是官方 `settings.form.get/set/reset` 的 result。
type SettingsSectionView struct {
	NS       string              `json:"ns"`
	Title    string              `json:"title"`
	Writable bool                `json:"writable"`
	Revision int                 `json:"revision"`
	Fields   []SettingsFieldView `json:"fields"`
	Errors   []string            `json:"errors"`
}

// SettingsDescribeResult 是 `settings.describe` / `settings.form.describe` 的 result。
type SettingsDescribeResult struct {
	Writable    bool                    `json:"writable"`
	HasDocument bool                    `json:"hasDocument"`
	Namespaces  []SettingsNamespaceView `json:"namespaces"`
	Sections    []SettingsSectionView   `json:"sections"`
	Errors      []string                `json:"errors"`
}

// Namespace 按名字取一个 namespace；没有就是 false（**不给零值兜底**）。
func (r SettingsDescribeResult) Namespace(ns string) (SettingsNamespaceView, bool) {
	for _, n := range r.Namespaces {
		if n.NS == ns {
			return n, true
		}
	}
	return SettingsNamespaceView{}, false
}

// Section 按 namespace 取一个 section；没有就是 false。
func (r SettingsDescribeResult) Section(ns string) (SettingsSectionView, bool) {
	for _, s := range r.Sections {
		if s.NS == ns {
			return s, true
		}
	}
	return SettingsSectionView{}, false
}

// Field 按名字取一个字段；没有就是 false（未知字段要隐藏/显示未知，不许编）。
func (s SettingsSectionView) Field(name string) (SettingsFieldView, bool) {
	for _, f := range s.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return SettingsFieldView{}, false
}

// SettingsDescribe 读全部设置描述符。
func (c *Client) SettingsDescribe(ctx context.Context) (SettingsDescribeResult, error) {
	var out SettingsDescribeResult
	if err := c.Call(ctx, "settings.describe", struct{}{}, &out); err != nil {
		return SettingsDescribeResult{}, err
	}
	return out, nil
}

// SettingsFormDescribe 是显式通用表单描述的入口（与 describe 同形）。
func (c *Client) SettingsFormDescribe(ctx context.Context) (SettingsDescribeResult, error) {
	var out SettingsDescribeResult
	if err := c.Call(ctx, "settings.form.describe", struct{}{}, &out); err != nil {
		return SettingsDescribeResult{}, err
	}
	return out, nil
}

// SettingsFormGet 读一个 namespace 的 section。
func (c *Client) SettingsFormGet(ctx context.Context, ns string) (SettingsSectionView, error) {
	var out SettingsSectionView
	if err := c.Call(ctx, "settings.form.get", map[string]any{"ns": ns}, &out); err != nil {
		return SettingsSectionView{}, err
	}
	return out, nil
}

// SettingsFormSet 写一个字段，返回**写后的 section**（不是 `{ok:true}`）。
//
// expectedRevision 是持久乐观锁：传 nil = 不 fence（宿主用最新 revision）；
// 传值 = 必须等于宿主当前 revision，否则 409 `revision_conflict`。
func (c *Client) SettingsFormSet(ctx context.Context, ns string, path []string, value any, expectedRevision *int) (SettingsSectionView, error) {
	params := map[string]any{"ns": ns, "path": path, "value": value}
	if expectedRevision != nil {
		params["expectedRevision"] = *expectedRevision
	}
	var out SettingsSectionView
	if err := c.Call(ctx, "settings.form.set", params, &out); err != nil {
		return SettingsSectionView{}, err
	}
	return out, nil
}

// SettingsFormReset 清一个字段（path 非 nil）或整个 namespace（path == nil）。
//
// **显式 `path: null` 会被宿主拒绝**（`invalid_path`）：清整个 namespace 只能
// 省略 path。所以这里用 nil 切片表达「省略」，并在 nil 时真的不写这个键。
func (c *Client) SettingsFormReset(ctx context.Context, ns string, path []string, expectedRevision *int) (SettingsSectionView, error) {
	params := map[string]any{"ns": ns}
	if path != nil {
		params["path"] = path
	}
	if expectedRevision != nil {
		params["expectedRevision"] = *expectedRevision
	}
	var out SettingsSectionView
	if err := c.Call(ctx, "settings.form.reset", params, &out); err != nil {
		return SettingsSectionView{}, err
	}
	return out, nil
}

// SettingsMutate 原子提交一批路径操作，返回单个 `SettingsNamespaceView`。
func (c *Client) SettingsMutate(ctx context.Context, ns string, ops []SettingsOp, expectedRevision *int) (SettingsNamespaceView, error) {
	params := map[string]any{"ns": ns, "ops": ops}
	if expectedRevision != nil {
		params["expectedRevision"] = *expectedRevision
	}
	var out SettingsNamespaceView
	if err := c.Call(ctx, "settings.mutate", params, &out); err != nil {
		return SettingsNamespaceView{}, err
	}
	return out, nil
}

// SettingsSetOp / SettingsUnsetOp 造两种路径操作（避免调用方手写字面量拼错）。
func SettingsSetOp(path []string, value any) SettingsOp {
	return SettingsOp{Op: "set", Path: path, Value: value}
}

// SettingsUnsetOp 造一个 unset 操作。
func SettingsUnsetOp(path []string) SettingsOp {
	return SettingsOp{Op: "unset", Path: path}
}

// SettingsErrorCode 取设置错误的四字段 code；不是 API 错误时返回空串。
//
// 界面据此渲染失败态（409 `revision_conflict` / `read_only` / 校验错误），
// **不靠匹配错误文本**。
func SettingsErrorCode(err error) string {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	return ""
}

// SettingsRetryable 报告一个设置错误是否可重试（宿主四字段的 retryable）。
func SettingsRetryable(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Retryable
	}
	return false
}

// RetryPolicyPath 拼出 `providers.<route>.retryPolicy` 的字段路径。
//
// 官方形状（dsh-llm/src/retry-policy.ts:29 "retryPolicy belongs under each
// provider configuration"）：`llm.providers.<route>.retryPolicy.<field>`。
func RetryPolicyPath(route, field string) []string {
	return []string{"providers", route, "retryPolicy", field}
}

// RetryPolicyBackoffPath 拼出 `providers.<route>.retryPolicy.backoff.<field>`。
func RetryPolicyBackoffPath(route, field string) []string {
	return []string{"providers", route, "retryPolicy", "backoff", field}
}

// SettingValueString 把 field 的 `current` 渲染成界面文本。
//
// **未知就是未知**：nil（字段缺失/null）→ ("", false)；调用方必须显示「未知」，
// 不许补 0、不许补 ""。数字用 strconv 的默认格式，保证 5e-324 这类正下界
// 不会被格式化成 0。
func SettingValueString(v any) (string, bool) {
	switch value := v.(type) {
	case nil:
		return "", false
	case string:
		return value, true
	case bool:
		if value {
			return "true", true
		}
		return "false", true
	case float64:
		return formatFloat(value), true
	case int:
		return fmt.Sprintf("%d", value), true
	case json.Number:
		return value.String(), true
	default:
		// 数组/对象（例如 retryableCodes）不是单值行能表达的，交给专用渲染器。
		raw, err := json.Marshal(value)
		if err != nil {
			return "", false
		}
		return string(raw), true
	}
}

// formatFloat 是**不丢精度**的浮点格式（strconv.FormatFloat(v,'g',-1,64) 的
// 等价物，但避免为了一个格式引 strconv 之外的依赖）。
func formatFloat(v float64) string {
	return trimFloat(fmt.Sprintf("%g", v))
}

// trimFloat 去掉 `%g` 可能留下的指数前导零（`1e-05` → `1e-5`），
// 只为让界面文本与官方 `Number.prototype.toString` 一致。
func trimFloat(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] != 'e' {
			continue
		}
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		for j+1 < len(s) && s[j] == '0' && s[j+1] >= '0' && s[j+1] <= '9' {
			s = s[:j] + s[j+1:]
		}
		break
	}
	return s
}
