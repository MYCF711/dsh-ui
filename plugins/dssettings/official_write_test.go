package dssettings

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// fakeWriter 是 SettingsWriter 的记录型替身：真实产品走 Transport，
// 这里只验「发了什么 + 失败怎么分类」。
type fakeWriter struct {
	setCalls    []writeCall
	resetCalls  []writeCall
	mutateCalls []mutateCall

	setErr    error
	resetErr  error
	mutateErr error

	section   apiclient.SettingsSectionView
	namespace apiclient.SettingsNamespaceView
}

type writeCall struct {
	ns          string
	path        []string
	value       any
	expectedRev *int
}

type mutateCall struct {
	ns          string
	ops         []apiclient.SettingsOp
	expectedRev *int
}

func (w *fakeWriter) SettingsFormSet(_ context.Context, ns string, path []string, value any, expectedRevision *int) (apiclient.SettingsSectionView, error) {
	w.setCalls = append(w.setCalls, writeCall{ns: ns, path: path, value: value, expectedRev: expectedRevision})
	if w.setErr != nil {
		return apiclient.SettingsSectionView{}, w.setErr
	}
	return w.section, nil
}

func (w *fakeWriter) SettingsFormReset(_ context.Context, ns string, path []string, expectedRevision *int) (apiclient.SettingsSectionView, error) {
	w.resetCalls = append(w.resetCalls, writeCall{ns: ns, path: path, expectedRev: expectedRevision})
	if w.resetErr != nil {
		return apiclient.SettingsSectionView{}, w.resetErr
	}
	return w.section, nil
}

func (w *fakeWriter) SettingsMutate(_ context.Context, ns string, ops []apiclient.SettingsOp, expectedRevision *int) (apiclient.SettingsNamespaceView, error) {
	w.mutateCalls = append(w.mutateCalls, mutateCall{ns: ns, ops: ops, expectedRev: expectedRevision})
	if w.mutateErr != nil {
		return apiclient.SettingsNamespaceView{}, w.mutateErr
	}
	return w.namespace, nil
}

func apiError(code, message, hint string, retryable bool) *apiclient.APIError {
	return &apiclient.APIError{Code: code, Message: message, Hint: hint, Retryable: retryable}
}

// TestApplySetSuccessRecordsRevision 钉住成功路径。
func TestApplySetSuccessRecordsRevision(t *testing.T) {
	writer := &fakeWriter{section: apiclient.SettingsSectionView{NS: "permission", Revision: 9}}
	state := ApplySet(context.Background(), writer, "permission", []string{"defaultPreset"}, "deny", 8)
	if !state.Success() || state.Failure() {
		t.Fatalf("状态 = %+v，应为成功", state)
	}
	if state.Revision != 9 {
		t.Fatalf("成功后的 revision = %d，应取写后 section.revision", state.Revision)
	}
	if state.Render() != "已保存" {
		t.Fatalf("成功文案 = %q", state.Render())
	}
	if len(writer.setCalls) != 1 {
		t.Fatalf("调用次数 = %d", len(writer.setCalls))
	}
	call := writer.setCalls[0]
	if call.ns != "permission" || strings.Join(call.path, ".") != "defaultPreset" || call.value != "deny" {
		t.Fatalf("调用参数 = %+v", call)
	}
	if call.expectedRev == nil || *call.expectedRev != 8 {
		t.Fatalf("expectedRevision = %v，应显式传 8", call.expectedRev)
	}
}

// TestApplySetWithoutRevisionOmitsFence 钉住「revision=0 → 不 fence」。
func TestApplySetWithoutRevisionOmitsFence(t *testing.T) {
	writer := &fakeWriter{section: apiclient.SettingsSectionView{Revision: 1}}
	ApplySet(context.Background(), writer, "model", []string{"defaultModel"}, "x", 0)
	if writer.setCalls[0].expectedRev != nil {
		t.Fatalf("expectedRevision = %v，0 应省略（宿主用最新 revision）", *writer.setCalls[0].expectedRev)
	}
}

// TestWriteConflictRendering 是 **409 revision_conflict 的渲染断言**。
func TestWriteConflictRendering(t *testing.T) {
	writer := &fakeWriter{setErr: apiError(apiclient.SettingsCodeRevisionConflict,
		"permission revision 已变化：expected=2 actual=3，未覆盖新值",
		"重新 settings.describe/get，基于最新 revision 再提交", false)}
	state := ApplySet(context.Background(), writer, "permission", []string{"defaultPreset"}, "deny", 2)

	if state.Kind != WriteConflict {
		t.Fatalf("状态 = %q，409 revision_conflict 应分类为 conflict", state.Kind)
	}
	if !state.Failure() || state.Success() {
		t.Fatal("冲突必须是失败态（不许假报成功）")
	}
	if !state.NeedsReload() {
		t.Fatal("冲突必须触发 recovery reload（官方 §2.3：失败先重读）")
	}
	text := state.Render()
	for _, want := range []string{"冲突", "revision", "保留", "重新载入"} {
		if !strings.Contains(text, want) {
			t.Fatalf("冲突文案缺 %q：%q", want, text)
		}
	}
	if state.Code != apiclient.SettingsCodeRevisionConflict {
		t.Fatalf("code = %q", state.Code)
	}
}

// TestWriteReadOnlyRendering 是 **read_only 的渲染断言**。
func TestWriteReadOnlyRendering(t *testing.T) {
	writer := &fakeWriter{setErr: apiError(apiclient.SettingsCodeReadOnly,
		"session-log-deepseek 命名空间只读", "该命名空间由宿主管理", false)}
	state := ApplySet(context.Background(), writer, "session-log-deepseek", []string{"enabled"}, true, 3)
	if state.Kind != WriteReadOnly {
		t.Fatalf("状态 = %q，read_only 应分类为只读", state.Kind)
	}
	if state.NeedsReload() {
		t.Fatal("只读不需要重读（数据没变）")
	}
	if !strings.Contains(state.Render(), "只读") {
		t.Fatalf("只读文案 = %q", state.Render())
	}
}

// TestWriteValidationRendering 钉住校验失败（4xx）的渲染。
func TestWriteValidationRendering(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{apiclient.SettingsCodeInvalidEnum, "校验失败"},
		{apiclient.SettingsCodeOutOfRange, "校验失败"},
		{apiclient.SettingsCodeInvalidPath, "校验失败"},
		{apiclient.SettingsCodeUnknownField, "校验失败"},
		{apiclient.SettingsCodeInvalidType, "校验失败"},
		{apiclient.SettingsCodeNotFound, "校验失败"},
		{apiclient.SettingsCodeBadRequest, "校验失败"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			writer := &fakeWriter{setErr: apiError(tc.code, "值不合法", "按 field.enum 提交", false)}
			state := ApplySet(context.Background(), writer, "permission", []string{"defaultPreset"}, "block", 1)
			if state.Kind != WriteInvalid {
				t.Fatalf("%s 应分类为 invalid，实得 %q", tc.code, state.Kind)
			}
			text := state.Render()
			if !strings.Contains(text, tc.want) || !strings.Contains(text, "按 field.enum 提交") {
				t.Fatalf("文案 = %q", text)
			}
		})
	}
}

// TestWriteInternalRetryableRendering 钉住 500 internal 的可重试文案。
func TestWriteInternalRetryableRendering(t *testing.T) {
	writer := &fakeWriter{setErr: apiError(apiclient.SettingsCodeInternal, "写盘失败", "", true)}
	state := ApplySet(context.Background(), writer, "model", []string{"defaultModel"}, "x", 1)
	if state.Kind != WriteFailed {
		t.Fatalf("状态 = %q", state.Kind)
	}
	if !state.Retryable {
		t.Fatal("internal 的 retryable 应透传")
	}
	if !strings.Contains(state.Render(), "可重试") {
		t.Fatalf("文案 = %q", state.Render())
	}
}

// TestWriteTransportErrorIsRetryable 钉住传输层失败（非四字段）也可重试。
func TestWriteTransportErrorIsRetryable(t *testing.T) {
	writer := &fakeWriter{setErr: errors.New("连不上 http://127.0.0.1:1：connection refused")}
	state := ApplySet(context.Background(), writer, "model", []string{"defaultModel"}, "x", 1)
	if state.Kind != WriteFailed || !state.Retryable {
		t.Fatalf("状态 = %+v，传输错误应可重试", state)
	}
	if !strings.Contains(state.Render(), "保存失败") {
		t.Fatalf("文案 = %q", state.Render())
	}
}

// TestApplyResetAndMutate 钉住另外两条写路径。
func TestApplyResetAndMutate(t *testing.T) {
	writer := &fakeWriter{
		section:   apiclient.SettingsSectionView{Revision: 4},
		namespace: apiclient.SettingsNamespaceView{NS: "llm", Revision: 6},
	}
	state := ApplyReset(context.Background(), writer, "permission", []string{"defaultPreset"}, 3)
	if !state.Success() || state.Revision != 4 {
		t.Fatalf("reset 状态 = %+v", state)
	}
	if len(writer.resetCalls) != 1 || writer.resetCalls[0].path[0] != "defaultPreset" {
		t.Fatalf("reset 调用 = %+v", writer.resetCalls)
	}

	ops := RetryPolicyWriteOps("deepseek", "maxRetries", 9)
	state = ApplyMutate(context.Background(), writer, "llm", ops, 5)
	if !state.Success() || state.Revision != 6 {
		t.Fatalf("mutate 状态 = %+v", state)
	}
	if len(writer.mutateCalls) != 1 {
		t.Fatalf("mutate 调用 = %+v", writer.mutateCalls)
	}
	mc := writer.mutateCalls[0]
	if mc.ns != "llm" || len(mc.ops) != 1 || mc.expectedRev == nil || *mc.expectedRev != 5 {
		t.Fatalf("mutate 参数 = %+v", mc)
	}
	if got := strings.Join(mc.ops[0].Path, "."); got != "providers.deepseek.retryPolicy.maxRetries" {
		t.Fatalf("mutate 路径 = %q", got)
	}
}

// TestWriteFailureFixturesMatchKinds 钉住测试夹具本身（防止夹具漂移）。
func TestWriteFailureFixturesMatchKinds(t *testing.T) {
	for _, tc := range []struct {
		kind WriteKind
		code string
	}{
		{WriteConflict, apiclient.SettingsCodeRevisionConflict},
		{WriteReadOnly, apiclient.SettingsCodeReadOnly},
		{WriteInvalid, apiclient.SettingsCodeInvalidEnum},
		{WriteFailed, apiclient.SettingsCodeInternal},
	} {
		state := WriteFailureFixture(tc.kind, "permission", []string{"defaultPreset"})
		if state.Kind != tc.kind {
			t.Fatalf("夹具 kind = %q，期望 %q", state.Kind, tc.kind)
		}
		if state.Code != tc.code {
			t.Fatalf("夹具 code = %q，期望 %q", state.Code, tc.code)
		}
		if !state.Failure() {
			t.Fatalf("%q 夹具应是失败态", tc.kind)
		}
		if state.Render() == "" {
			t.Fatalf("%q 夹具没有可渲染文案", tc.kind)
		}
	}
}

// TestWriteIdleRendersNothing 钉住空闲态不显示任何失败提示。
func TestWriteIdleRendersNothing(t *testing.T) {
	var state WriteState
	if state.Render() != "" {
		t.Fatalf("空闲态文案 = %q，应为空", state.Render())
	}
	if state.Failure() || state.Success() {
		t.Fatal("零值既不是成功也不是失败")
	}
	saving := WriteState{Kind: WriteSaving}
	if got := saving.Render(); got == "" || fmt.Sprint(got) != "保存中…" {
		t.Fatalf("保存中文案 = %q", got)
	}
}
