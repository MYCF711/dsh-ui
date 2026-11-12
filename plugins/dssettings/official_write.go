// official_write.go —— 设置写路径的**失败态**（`settings.form.set|reset` /
// `settings.mutate` 的界面侧）。
//
// # 官方语义（结构树 §2.3）
//
//   - 每次 mutate 使用**显式 expectedRevision**（否则用最新 snapshot.revision）；
//   - 只把最新写答案折回 mirror，被后续写 supersede 的答案只推进 pendingRevision；
//   - **失败先 recovery reload**，不在 UI 假报成功；
//   - `settings/conflict` 显示冲突并**保留编辑上下文**（不要把用户输入清掉）。
//
// 官方原文（ui-settings/src/client/config-form.ts:129-155）：
//
//	写失败 → recovery reload（:135-137、:150-155）→ 冲突时保留编辑上下文
//
// # 本仓的等价物
//
// 写路径是一个**纯函数 + 小状态机**：调用方给 Writer（真实是 *apiclient.Client），
// 这里负责发请求、分类错误、产出可渲染的 WriteState。绘制层只读 WriteState，
// 不做任何「本地先变绿」的乐观更新——那正是官方禁止的「假报成功」。
package dssettings

import (
	"context"
	"errors"
	"fmt"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// WriteKind 是一次写入结果的种类。
type WriteKind string

const (
	// WriteIdle 没有进行中的写。
	WriteIdle WriteKind = "idle"
	// WriteSaving 正在写（界面显示「保存中」，允许后续写 supersede）。
	WriteSaving WriteKind = "saving"
	// WriteSaved 写成功（宿主已落盘；revision 是新的）。
	WriteSaved WriteKind = "saved"
	// WriteConflict 409 `revision_conflict`：别的写者先改了，**不得覆盖**。
	WriteConflict WriteKind = "conflict"
	// WriteReadOnly 命名空间只读（`read_only`）。
	WriteReadOnly WriteKind = "read_only"
	// WriteInvalid 校验失败（bad_request / invalid_path / unknown_field /
	// invalid_enum / out_of_range / invalid_type）。
	WriteInvalid WriteKind = "invalid"
	// WriteFailed 宿主内部错误（internal，retryable 可能为真）。
	WriteFailed WriteKind = "failed"
)

// WriteState 是一次写的结果（绘制层唯一读的东西）。
type WriteState struct {
	Kind WriteKind
	// NS / Path 是这次写的目标（错误提示要指明「哪个字段」）。
	NS   string
	Path []string
	// Code 是宿主四字段错误的 code（成功时为空）。
	Code string
	// Message / Hint 是宿主的 message / hint 原文。
	Message string
	Hint    string
	// Retryable 是宿主四字段的 retryable（仅 WriteFailed 有意义）。
	Retryable bool
	// Revision 是写成功后的新 revision（冲突时是宿主当前的 revision，若有）。
	Revision int
}

// Success 报告这次写是否成功。
func (s WriteState) Success() bool { return s.Kind == WriteSaved }

// Failure 报告这次写是否失败（冲突/只读/校验/内部错误）。
func (s WriteState) Failure() bool {
	switch s.Kind {
	case WriteConflict, WriteReadOnly, WriteInvalid, WriteFailed:
		return true
	default:
		return false
	}
}

// NeedsReload 报告失败后是否必须 recovery reload（官方 §2.3：失败先重读）。
//
// 冲突一定要重读（本地 revision 已过期）；只读与校验错误不需要（数据没变）。
func (s WriteState) NeedsReload() bool { return s.Kind == WriteConflict }

// Render 是失败态的界面文案（成功/空闲返回空串——成功反馈由行内状态给）。
//
// 文案必须说清「发生了什么 + 下一步做什么」，不许只说「保存失败」。
func (s WriteState) Render() string {
	switch s.Kind {
	case WriteSaving:
		return "保存中…"
	case WriteSaved:
		return "已保存"
	case WriteConflict:
		return "冲突：设置已被外部修改（revision 已变化）。已保留你的编辑内容，请重新载入后再提交。"
	case WriteReadOnly:
		return "只读：该命名空间未开放写入。"
	case WriteInvalid:
		if s.Hint != "" {
			return "校验失败：" + s.Message + "（" + s.Hint + "）"
		}
		return "校验失败：" + s.Message
	case WriteFailed:
		if s.Retryable {
			return "保存失败：" + s.Message + "（可重试）"
		}
		return "保存失败：" + s.Message
	default:
		return ""
	}
}

// SettingsWriter 是写路径需要的最小客户端能力。
//
// 用接口而不是直接依赖 *apiclient.Client：测试可以注入计数/报错替身，
// 而真实产品仍走 Transport（R7：插件不直连宿主内部）。
type SettingsWriter interface {
	SettingsFormSet(ctx context.Context, ns string, path []string, value any, expectedRevision *int) (apiclient.SettingsSectionView, error)
	SettingsFormReset(ctx context.Context, ns string, path []string, expectedRevision *int) (apiclient.SettingsSectionView, error)
	SettingsMutate(ctx context.Context, ns string, ops []apiclient.SettingsOp, expectedRevision *int) (apiclient.SettingsNamespaceView, error)
}

// revisionPtr 把 0 当作「不 fence」（官方：省略 expectedRevision 用最新 revision）。
func revisionPtr(r int) *int {
	if r <= 0 {
		return nil
	}
	return &r
}

// ApplySet 写一个字段（`settings.form.set`）。
//
// 返回的 WriteState 是**只有成功才写 Revision** 的状态；失败按四字段错误分类。
func ApplySet(ctx context.Context, w SettingsWriter, ns string, path []string, value any, expectedRevision int) WriteState {
	state := WriteState{Kind: WriteSaving, NS: ns, Path: append([]string(nil), path...)}
	section, err := w.SettingsFormSet(ctx, ns, path, value, revisionPtr(expectedRevision))
	if err != nil {
		return ClassifyWriteError(ns, path, err)
	}
	state.Kind = WriteSaved
	state.Revision = section.Revision
	return state
}

// ApplyReset 清一个字段（`settings.form.reset`，path 非 nil）或整个命名空间。
func ApplyReset(ctx context.Context, w SettingsWriter, ns string, path []string, expectedRevision int) WriteState {
	state := WriteState{Kind: WriteSaving, NS: ns, Path: append([]string(nil), path...)}
	section, err := w.SettingsFormReset(ctx, ns, path, revisionPtr(expectedRevision))
	if err != nil {
		return ClassifyWriteError(ns, path, err)
	}
	state.Kind = WriteSaved
	state.Revision = section.Revision
	return state
}

// ApplyMutate 原子提交一批路径操作（`settings.mutate`，retryPolicy 走这条）。
func ApplyMutate(ctx context.Context, w SettingsWriter, ns string, ops []apiclient.SettingsOp, expectedRevision int) WriteState {
	state := WriteState{Kind: WriteSaving, NS: ns}
	if len(ops) > 0 {
		state.Path = append([]string(nil), ops[0].Path...)
	}
	view, err := w.SettingsMutate(ctx, ns, ops, revisionPtr(expectedRevision))
	if err != nil {
		return ClassifyWriteError(ns, state.Path, err)
	}
	state.Kind = WriteSaved
	state.Revision = view.Revision
	return state
}

// ClassifyWriteError 把宿主四字段错误翻成 WriteState。
//
// 分类**只看 code**（不匹配错误文本）：`revision_conflict` → 冲突，
// `read_only` → 只读，其余 4xx 校验类 → 校验失败，其它 → 内部失败。
func ClassifyWriteError(ns string, path []string, err error) WriteState {
	state := WriteState{Kind: WriteFailed, NS: ns, Path: append([]string(nil), path...)}
	if err == nil {
		// 不该发生：调用方只在 err != nil 时进来。保持 fail-loud。
		state.Message = "未知错误（调用方传入了 nil error）"
		return state
	}
	var apiErr *apiclient.APIError
	if errors.As(err, &apiErr) {
		state.Code = apiErr.Code
		state.Message = apiErr.Message
		state.Hint = apiErr.Hint
		state.Retryable = apiErr.Retryable
	} else {
		// 传输层错误（连不上 / 超时）：可重试，但不是宿主的四字段。
		state.Message = err.Error()
		state.Retryable = true
	}
	switch state.Code {
	case apiclient.SettingsCodeRevisionConflict:
		state.Kind = WriteConflict
	case apiclient.SettingsCodeReadOnly:
		state.Kind = WriteReadOnly
	case apiclient.SettingsCodeBadRequest,
		apiclient.SettingsCodeInvalidPath,
		apiclient.SettingsCodeUnknownField,
		apiclient.SettingsCodeInvalidEnum,
		apiclient.SettingsCodeOutOfRange,
		apiclient.SettingsCodeInvalidType,
		apiclient.SettingsCodeNotFound,
		apiclient.SettingsCodeUnauthorized:
		state.Kind = WriteInvalid
	default:
		state.Kind = WriteFailed
	}
	return state
}

// WriteFailureFixture 造一个用于渲染断言的四字段失败（测试与演示用）。
//
// 刻意**只造宿主真实会返回的 code**；未知 code 会让测试以为分类是通的。
func WriteFailureFixture(kind WriteKind, ns string, path []string) WriteState {
	switch kind {
	case WriteConflict:
		return WriteState{
			Kind: WriteConflict, NS: ns, Path: path, Code: apiclient.SettingsCodeRevisionConflict,
			Message: fmt.Sprintf("%s revision 已变化：expected=2 actual=3，未覆盖新值", ns),
			Hint:    "重新 settings.describe/get，基于最新 revision 再提交",
		}
	case WriteReadOnly:
		return WriteState{
			Kind: WriteReadOnly, NS: ns, Path: path, Code: apiclient.SettingsCodeReadOnly,
			Message: ns + " 命名空间只读",
			Hint:    "该命名空间由宿主管理，界面只能读取",
		}
	case WriteInvalid:
		return WriteState{
			Kind: WriteInvalid, NS: ns, Path: path, Code: apiclient.SettingsCodeInvalidEnum,
			Message: "值不在枚举内",
			Hint:    "只提交 field.enum 原样列出的值",
		}
	default:
		return WriteState{
			Kind: WriteFailed, NS: ns, Path: path, Code: apiclient.SettingsCodeInternal,
			Message: "写盘失败", Retryable: true,
		}
	}
}
