package dssettings

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

func modelsSnapshotFixture() *apiclient.ModelsPageSnapshot {
	return &apiclient.ModelsPageSnapshot{
		Registered: []apiclient.ProviderInfo{{ID: "zeta", Name: "Zeta route"}, {ID: "deepseek-official", Name: "DeepSeek"}, {ID: "registered-only", Name: "Registered only"}},
		Configurable: []apiclient.ConfigurableProvider{
			{Provider: "zeta", DisplayName: "Zeta configurable", SettingsNamespace: "llm", SettingsPath: []string{"providers", "zeta"}},
			{Provider: "deepseek-official", DisplayName: "DeepSeek official", SettingsNamespace: "llm-deepseek"},
			{Provider: "dormant", DisplayName: "Dormant", SettingsNamespace: "llm", SettingsPath: []string{"providers", "dormant"}},
		},
		Catalog: &apiclient.ModelCatalog{
			Default: apiclient.ModelSelection{Provider: "zeta", Model: "zeta-1"},
			Groups: []apiclient.ModelProvider{
				{ID: "zeta", Name: "Zeta", Models: []apiclient.CatalogModel{{ID: "zeta-1", Name: "Zeta One"}, {ID: "zeta-2", Name: "Zeta Two"}}},
				{ID: "deepseek-official", Name: "DeepSeek", Models: []apiclient.CatalogModel{{ID: "deepseek-chat", Name: "DeepSeek Chat"}}},
			},
		},
	}
}

func TestModelsPageJoinsRealDirectoryAndCatalog(t *testing.T) {
	describe := describeFixture()
	describe.Namespaces = append(describe.Namespaces, apiclient.SettingsNamespaceView{NS: "llm-deepseek", Value: json.RawMessage(`{}`)})
	snapshot := modelsSnapshotFixture()
	model := BuildPanel(describe, fullDesktopFacts())
	panel, _ := model.Section("models")
	panel = buildModelsPage(panel, describe, nil, snapshot, nil)
	if panel.Models == nil || len(panel.Models.Providers) != 4 {
		t.Fatalf("目录 join = %+v，应为 4 条真实 route", panel.Models)
	}
	// account/official 稳定提前，其余保持 configurable 声明顺序，注册补充不丢。
	if panel.Models.Providers[0].Provider != "deepseek-official" || panel.Models.Providers[1].Provider != "zeta" {
		t.Fatalf("provider 顺序 = %+v", panel.Models.Providers)
	}
	if !panel.Models.Providers[0].Active || !panel.Models.Providers[0].Configured {
		t.Fatalf("whole-section provider 的 active/configured 算错：%+v", panel.Models.Providers[0])
	}
	if !panel.Models.Providers[1].Active || panel.Models.Providers[1].Configured {
		t.Fatalf("zeta 的 active/configured 算错：%+v", panel.Models.Providers[1])
	}
	var defaultRow PanelRow
	for _, row := range panel.Rows {
		if row.ID == "model-default" {
			defaultRow = row
		}
	}
	for _, modelID := range []string{"zeta-1", "zeta-2", "deepseek-chat"} {
		found := false
		for _, choice := range defaultRow.Choices {
			if choice.Value == modelID {
				found = true
			}
		}
		if !found {
			t.Errorf("默认模型候选缺 %q：%+v", modelID, defaultRow.Choices)
		}
	}
	for _, text := range []string{"模型目录 · Zeta", "模型目录 · DeepSeek"} {
		found := false
		for _, group := range panel.Groups {
			if group.Title == text {
				found = true
			}
		}
		if !found {
			t.Errorf("缺真实模型分组 %q：%+v", text, panel.Groups)
		}
	}
}

func TestModelsPageFailuresAreNotEmptySuccess(t *testing.T) {
	describe := describeFixture()
	snapshot := &apiclient.ModelsPageSnapshot{RegisteredError: "registered 501", CatalogError: "catalog unavailable"}
	panel, _ := BuildPanel(describe, fullDesktopFacts()).Section("models")
	panel = buildModelsPage(panel, describe, nil, snapshot, nil)
	if len(panel.Models.Errors) != 2 || panel.Note == "" {
		t.Fatalf("失败必须可见：errors=%v note=%q", panel.Models.Errors, panel.Note)
	}
	if panel.Models.Loading {
		t.Fatal("请求结束后不能停在 Loading")
	}
}

func TestRetryPolicyDraftDoesNotWriteUntilAtomicApply(t *testing.T) {
	describe := describeFixture()
	edit, err := newRetryEditorState(describe, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	rows := edit.rows()
	var timeout PanelRow
	for _, row := range rows {
		if row.ID == "retry-deepseek-code-TIMEOUT" {
			timeout = row
		}
	}
	if err := edit.change(timeout); err != nil {
		t.Fatal(err)
	}
	ops := edit.ops()
	if len(ops) != 2 { // 老宿主只给 Value 时，首次草稿需显式 mode + codes。
		t.Fatalf("草稿 ops = %+v，应只含 mode 与完整 retryableCodes", ops)
	}
	for _, op := range ops {
		path := strings.Join(op.Path, ".")
		if path != "providers.deepseek.retryPolicy.mode" && path != "providers.deepseek.retryPolicy.retryableCodes" {
			t.Fatalf("把继承字段也写进了 user 层：%+v", ops)
		}
	}

	writer := &fakeWriter{namespace: apiclient.SettingsNamespaceView{NS: "llm", Revision: 6}}
	plugin := &Plugin{}
	official := plugin.ensureOfficial()
	official.describe, official.writer, official.modelsEdit = describe, writer, edit
	state := plugin.applyModelsRetry()
	if !state.Success() || state.Revision != 6 {
		t.Fatalf("Apply 状态 = %+v", state)
	}
	if len(writer.mutateCalls) != 1 || len(writer.setCalls) != 0 {
		t.Fatalf("Apply 必须且只能发一次 mutate：set=%d mutate=%d", len(writer.setCalls), len(writer.mutateCalls))
	}
	call := writer.mutateCalls[0]
	if call.expectedRev == nil || *call.expectedRev != 5 {
		t.Fatalf("expectedRevision = %v，应取开卡时的 5", call.expectedRev)
	}
	if official.modelsEdit != nil {
		t.Fatal("Apply 成功后应关闭编辑卡")
	}
}

func TestRetryPolicyCancelAndConflictKeepTruth(t *testing.T) {
	describe := describeFixture()
	edit, err := newRetryEditorState(describe, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	// 不做编辑时 Apply 是 no-op：不发请求、不伪造 saved。
	writer := &fakeWriter{namespace: apiclient.SettingsNamespaceView{NS: "llm", Revision: 6}}
	plugin := &Plugin{}
	official := plugin.ensureOfficial()
	official.describe, official.writer, official.modelsEdit = describe, writer, edit
	if state := plugin.applyModelsRetry(); state.Kind != WriteIdle || len(writer.mutateCalls) != 0 {
		t.Fatalf("无改动 Apply = %+v, writes=%d", state, len(writer.mutateCalls))
	}

	edit, _ = newRetryEditorState(describe, "deepseek")
	official.modelsEdit = edit
	if err := edit.change(edit.rows()[1]); err != nil {
		t.Fatal(err)
	}
	plugin.activateModelsRow(retryFooterRows("deepseek", true)[0])
	if official.modelsEdit != nil || len(writer.mutateCalls) != 0 {
		t.Fatal("Cancel 必须丢弃草稿且不发写")
	}

	edit, _ = newRetryEditorState(describe, "deepseek")
	rows := edit.rows()
	if err := edit.change(rows[1]); err != nil { // maxRetries：只改本地草稿
		t.Fatal(err)
	}
	writer.mutateErr = apiError(apiclient.SettingsCodeRevisionConflict, "revision moved", "reload", false)
	official.modelsEdit = edit
	state := plugin.applyModelsRetry()
	if state.Kind != WriteConflict || official.modelsEdit == nil {
		t.Fatalf("冲突必须保留草稿：state=%+v edit=%p", state, official.modelsEdit)
	}

	// 继承模式通过一次 unset 恢复 base，而不是把默认字段逐项写进 user。
	edit.policy = nil
	ops := edit.ops()
	if edit.original == nil {
		if len(ops) != 0 {
			t.Fatalf("原本就继承时不应 unset：%+v", ops)
		}
	} else if len(ops) != 1 || ops[0].Op != "unset" {
		t.Fatalf("恢复继承必须 unset 整个 user retryPolicy：%+v", ops)
	}
}

func TestRetryPolicyInheritedModeUnsetsUserPolicy(t *testing.T) {
	edit := &retryEditorState{route: "deepseek", original: map[string]any{"mode": "normal", "maxRetries": json.Number("7")}}
	edit.policy = nil
	ops := edit.ops()
	if len(ops) != 1 || ops[0].Op != "unset" || strings.Join(ops[0].Path, ".") != "providers.deepseek.retryPolicy" {
		t.Fatalf("恢复继承必须只 unset user retryPolicy：%+v", ops)
	}
}

func TestModelsProviderFailuresStayUnknown(t *testing.T) {
	describe := describeFixture()
	snapshot := &apiclient.ModelsPageSnapshot{
		RegisteredError: "listProviders failed",
		Configurable:    []apiclient.ConfigurableProvider{{Provider: "deepseek", DisplayName: "DeepSeek", SettingsNamespace: "llm", SettingsPath: []string{"providers", "deepseek"}}},
	}
	panel, _ := BuildPanel(describe, fullDesktopFacts()).Section("models")
	panel = buildModelsPage(panel, describe, nil, snapshot, nil)
	row, ok := panel.Row("provider-edit-deepseek")
	if !ok || !strings.Contains(row.Desc, "注册未知") || strings.Contains(row.Desc, "未注册") {
		t.Fatalf("首次 listProviders 失败被写成确定状态：%+v", row)
	}
	fallback := fallbackProviders(describe)
	if len(fallback) == 0 || fallback[0].ActiveKnown {
		t.Fatalf("settings fallback 不得伪造 ActiveKnown：%+v", fallback)
	}

	snapshot = &apiclient.ModelsPageSnapshot{
		Registered:        []apiclient.ProviderInfo{{ID: "deepseek", Name: "DeepSeek"}},
		ConfigurableError: "configurable failed",
	}
	panel = buildModelsPage(panel, describe, nil, snapshot, nil)
	row, ok = panel.Row("provider-edit-deepseek")
	if !ok || !strings.Contains(row.Desc, "配置未知") || !strings.Contains(row.Reason, "配置状态未知") || strings.Contains(row.Reason, "没有可写") {
		t.Fatalf("首次 configurable 失败被写成没有配置：%+v", row)
	}
}

func TestCatalogDefaultAlsoSelectsCurrentChoice(t *testing.T) {
	describe := describeFixture()
	for index := range describe.Sections {
		if describe.Sections[index].NS == apiclient.SettingsNSModel {
			describe.Sections[index].Fields[0].Current = ""
		}
	}
	panel, _ := BuildPanel(describe, fullDesktopFacts()).Section("models")
	panel = buildModelsPage(panel, describe, nil, modelsSnapshotFixture(), nil)
	row, _ := panel.Row("model-default")
	if row.Selected != "zeta-1" || row.Value != "zeta-1" {
		t.Fatalf("catalog 生效默认值没有同步 Selected：%+v", row)
	}
}

func TestDefaultModelEmptyChoiceUsesReset(t *testing.T) {
	writer := &fakeWriter{section: apiclient.SettingsSectionView{NS: "model", Revision: 4}}
	plugin := &Plugin{}
	plugin.ensureOfficial().writer = writer
	row := PanelRow{ID: "model-default", Field: "defaultModel", NS: "model", Path: []string{"defaultModel"},
		Kind: ItemSelector, Known: true, Selected: "zeta-1", Revision: 3,
		Choices: []PanelChoice{{Value: "zeta-1"}, {Value: "", Label: "跟随启动参数"}}}
	state := plugin.applyRowWrite(row)
	if !state.Success() || len(writer.resetCalls) != 1 || len(writer.setCalls) != 0 {
		t.Fatalf("空选项必须 reset：state=%+v set=%d reset=%d", state, len(writer.setCalls), len(writer.resetCalls))
	}
}

func TestActionWriterRevisionZeroUsesPointerChannel(t *testing.T) {
	rev := 0
	called := false
	writer := actionWriter{act: pluginapi.Actions{SettingsMutateRevision: func(_ string, _ []apiclient.SettingsOp, expected *int) (apiclient.SettingsNamespaceView, error) {
		called = true
		if expected == nil || *expected != 0 {
			t.Fatalf("revision 0 在 actionWriter 丢失：%v", expected)
		}
		return apiclient.SettingsNamespaceView{Revision: 1}, nil
	}}}
	if _, err := writer.SettingsMutate(context.Background(), "llm", nil, &rev); err != nil || !called {
		t.Fatalf("pointer 写口未被调用：called=%v err=%v", called, err)
	}
	legacy := actionWriter{act: pluginapi.Actions{SettingsMutate: func(string, []apiclient.SettingsOp, int) (apiclient.SettingsNamespaceView, error) {
		t.Fatal("老壳收到 revision 0 会静默不 fence，不应调用")
		return apiclient.SettingsNamespaceView{}, nil
	}}}
	if _, err := legacy.SettingsMutate(context.Background(), "llm", nil, &rev); err == nil {
		t.Fatal("老壳 revision 0 必须明确失败，不能降级覆盖")
	}
}

func TestRetryPolicyRevisionZeroRemainsFenced(t *testing.T) {
	describe := describeFixture()
	for index := range describe.Namespaces {
		if describe.Namespaces[index].NS == apiclient.SettingsNSLLM {
			describe.Namespaces[index].Revision = 0
		}
	}
	edit, err := newRetryEditorState(describe, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	if err := edit.change(edit.rows()[1]); err != nil {
		t.Fatal(err)
	}
	writer := &fakeWriter{namespace: apiclient.SettingsNamespaceView{NS: "llm", Revision: 1}}
	plugin := &Plugin{}
	official := plugin.ensureOfficial()
	official.describe, official.writer, official.modelsEdit = describe, writer, edit
	if state := plugin.applyModelsRetry(); !state.Success() {
		t.Fatalf("revision 0 Apply 失败：%+v", state)
	}
	call := writer.mutateCalls[0]
	if call.expectedRev == nil || *call.expectedRev != 0 {
		t.Fatalf("真实 revision 0 被省略：%v", call.expectedRev)
	}
}

type reopenDuringMutateWriter struct {
	fakeWriter
	onMutate func()
}

func (w *reopenDuringMutateWriter) SettingsMutate(ctx context.Context, ns string, ops []apiclient.SettingsOp, revision *int) (apiclient.SettingsNamespaceView, error) {
	w.fakeWriter.mutateCalls = append(w.fakeWriter.mutateCalls, mutateCall{ns: ns, ops: ops, expectedRev: revision})
	if w.onMutate != nil {
		w.onMutate()
	}
	return w.fakeWriter.namespace, nil
}

func TestRetryPolicyOldApplyCannotCloseReopenedDraft(t *testing.T) {
	describe := describeFixture()
	edit, _ := newRetryEditorState(describe, "deepseek")
	edit.generation = 1
	if err := edit.change(edit.rows()[1]); err != nil {
		t.Fatal(err)
	}
	plugin := &Plugin{}
	official := plugin.ensureOfficial()
	reopened := &retryEditorState{route: "deepseek", generation: 2, policy: map[string]any{"mode": "always"}}
	writer := &reopenDuringMutateWriter{fakeWriter: fakeWriter{namespace: apiclient.SettingsNamespaceView{NS: "llm", Revision: 6}}}
	writer.onMutate = func() {
		official.mu.Lock()
		official.modelsEdit = reopened
		official.mu.Unlock()
	}
	official.describe, official.writer, official.modelsEdit = describe, writer, edit
	if state := plugin.applyModelsRetry(); !state.Success() {
		t.Fatal(state.Render())
	}
	if official.modelsEdit != reopened {
		t.Fatal("旧 Apply 成功关闭了重开后的新草稿")
	}
}

func TestRetryPolicyCodesReturnToInheritedRemovesOverride(t *testing.T) {
	describe := describeFixture()
	for index := range describe.Namespaces {
		if describe.Namespaces[index].NS != apiclient.SettingsNSLLM {
			continue
		}
		// 非 canonical 顺序也必须按集合判断，不能在往返后多写 override。
		describe.Namespaces[index].Base = json.RawMessage(`{"providers":{"deepseek":{"retryPolicy":{"mode":"normal","retryableCodes":["SERVER","RATE_LIMIT"]}}}}`)
		describe.Namespaces[index].User = json.RawMessage(`{"providers":{"deepseek":{"retryPolicy":{"mode":"normal"}}}}`)
	}
	edit, err := newRetryEditorState(describe, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	var timeout PanelRow
	for _, row := range edit.rows() {
		if row.ID == "retry-deepseek-code-TIMEOUT" {
			timeout = row
		}
	}
	if err := edit.change(timeout); err != nil {
		t.Fatal(err)
	}
	for _, row := range edit.rows() { // 视图更新后再取 TIMEOUT。
		if row.ID == "retry-deepseek-code-TIMEOUT" {
			timeout = row
		}
	}
	if err := edit.change(timeout); err != nil {
		t.Fatal(err)
	}
	if ops := edit.ops(); len(ops) != 0 {
		t.Fatalf("回到继承 codes 仍产生 user override：%+v", ops)
	}
}
