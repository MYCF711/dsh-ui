// official_models_cred_rest_test.go —— #41 收尾 ①②的判据测试。
//
// 三条判据都来自官方 rc.2 随包 src（`dsh-client-ui-settings-models/src/client`）：
//
//	OFF:operations.ts:82-85        `removeCredential` → `credentials.unset(ref)`
//	OFF:ProviderEditor.tsx:350-356 删密钥排在设置写**之前**，删完就地记成 removed
//	OFF:ProviderEditor.tsx:446-459 删除两档说明（待删 / 已删）
//	OFF:ProviderEditor.tsx:673-686 EditorFooter 的三档：busy 标签、close 文案
//	OFF:EditorFooter.tsx:52,63     Cancel 忙碌时禁用；commit 忙碌时换成 applying
//	OFF:locales.ts:207-210         关闭 / 取消 / 保存 / 保存中…
package dssettings

import (
	"errors"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// TestModelsKeyStateRemovalBranches 钉官方 ProviderEditor.tsx:446-459 的删除两档。
//
// 判据链（照官方三元的顺序）：`keyText` 为空 → `keyRemoval !== undefined` 优先于
// 凭据状态；`removed === true` 时是「密钥已清除…无法撤销」，否则是「保存以清除…」。
// 两档都不再显示掩码占位——那一刻官方 `saved` 属性整块不传（:492）。
func TestModelsKeyStateRemovalBranches(t *testing.T) {
	saved := ModelsProvider{Provider: "p", APIKeyEnv: "P_API_KEY",
		Credential: &apiclient.CredentialInfo{Configured: true, Source: credentialSourceCatalog,
			Writable: true, Masked: "sk-••••1234"}}

	pending := modelsKeyState(saved, modelsKeyRemovalPending)
	if pending.Hint != "保存以清除已保存的 API key。之后使用环境认证（P_API_KEY）。" {
		t.Fatalf("待删说明 = %q（官方 keyRemovePendingEnvironment）", pending.Hint)
	}
	if pending.Tone != modelsKeyHintWarn {
		t.Fatalf("待删语气 = %q，官方是 .notice（warn）", pending.Tone)
	}
	if pending.Saved || pending.Placeholder == "sk-••••1234" {
		t.Fatalf("待删时不该再显示掩码占位：%+v", pending)
	}

	// 没有可见环境变量时用不带实参的那一句（官方 keyRemovePending）。
	noVisible := modelsKeyState(ModelsProvider{Provider: "p", APIKeyEnv: "P_OAUTH_TOKEN"}, modelsKeyRemovalPending)
	if noVisible.Hint != zhModelsKeyRemovePending {
		t.Fatalf("无可见引用时的待删说明 = %q", noVisible.Hint)
	}

	applied := modelsKeyState(saved, modelsKeyRemovalApplied)
	if applied.Hint != zhModelsKeyRemoveApplied {
		t.Fatalf("已删说明 = %q（官方 keyRemoveApplied）", applied.Hint)
	}
	if applied.Tone != modelsKeyHintWarn {
		t.Fatalf("已删语气 = %q，官方是 .notice（warn）", applied.Tone)
	}
}

// TestModelsKeyRowFollowsTheOfficialRemovalDisabled 钉 ProviderEditor.tsx:505：
// `disabled={disabled || keyRemoval?.removed === true}`——待删还能编辑，已删停用。
func TestModelsKeyRowFollowsTheOfficialRemovalDisabled(t *testing.T) {
	provider := ModelsProvider{Provider: "p", APIKeyEnv: "P_API_KEY", SettingsNS: apiclient.SettingsNSLLM,
		Credential: &apiclient.CredentialInfo{Configured: true, Source: credentialSourceCatalog, Writable: true, Masked: "sk-••…"}}

	pending, ok := modelsKeyRow(provider, true, modelsKeyRemovalPending)
	if !ok || pending.Disabled {
		t.Fatalf("待删的密钥行不该停用：%+v", pending)
	}
	if pending.Saved {
		t.Fatal("待删行不许再带 Saved（掩码占位与那两条拦截都失效）")
	}
	applied, ok := modelsKeyRow(provider, true, modelsKeyRemovalApplied)
	if !ok || !applied.Disabled {
		t.Fatalf("已删的密钥行必须停用：%+v", applied)
	}
	none, ok := modelsKeyRow(provider, true, modelsKeyRemovalNone)
	if !ok || !none.Saved || none.Placeholder != "sk-••…" {
		t.Fatalf("未删除时应保留掩码占位并标 Saved：%+v", none)
	}
}

// TestRetryFooterFollowsTheOfficialEditorFooter 钉 EditorFooter 三档里的两档：
// dismiss 的文案（cancel/close）与只读卡片仍可关闭。
func TestRetryFooterFollowsTheOfficialEditorFooter(t *testing.T) {
	idle := retryFooterRows("deepseek", true, false)
	if len(idle) != 2 {
		t.Fatalf("footer 行数 = %d", len(idle))
	}
	if idle[0].Title != zhEditorCancel || idle[1].Title != zhEditorApply {
		t.Fatalf("空闲文案 = %q / %q，官方是 %q / %q", idle[0].Title, idle[1].Title, zhEditorCancel, zhEditorApply)
	}
	// 官方 EditorFooter.tsx:10-11：卡片不可写也必须能关掉，所以 Cancel 不许因只读而禁用。
	readOnly := retryFooterRows("deepseek", false, false)
	if readOnly[0].Disabled {
		t.Fatal("只读卡片的 Cancel 必须仍可点（官方注释：a card the deployment cannot write to must still be dismissable）")
	}
	if !readOnly[1].Disabled {
		t.Fatal("只读卡片的 Apply 必须禁用")
	}
	// 密钥已删：dismiss 换成官方 'close'（ProviderEditor.tsx:683）。
	closed := retryFooterRows("deepseek", true, true)
	if closed[0].Title != zhEditorClose {
		t.Fatalf("密钥已删后 dismiss 文案 = %q，官方是 %q", closed[0].Title, zhEditorClose)
	}
}

// TestModelsApplyUnsetsBeforeWritingSettings 钉官方写序：
// `removeCredential` → `settings.mutate` → `storeCredential`（ProviderEditor.tsx:350-369）。
func TestModelsApplyUnsetsBeforeWritingSettings(t *testing.T) {
	describe := modelsCredentialFixture(`{"providers":{"deepseek":{"apiKeyEnv":"DEEPSEEK_REF","retryPolicy":{"mode":"normal"}}}}`)
	edit, err := newRetryEditorState(describe, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	edit.credentialRef = "DEEPSEEK_REF"
	edit.ensurePolicy()
	edit.policy["mode"] = "always"
	edit.requestKeyRemoval()

	var order []string
	writer := &orderedWriter{namespace: apiclient.SettingsNamespaceView{NS: "llm", Revision: 6}, log: &order}
	creds := &fakeCredentials{log: &order}
	plugin := &Plugin{}
	official := plugin.ensureOfficial()
	official.describe, official.writer, official.modelsEdit = describe, writer, edit
	official.credentialSet = creds.set
	official.credentialUnset = creds.unset

	state := plugin.applyModelsEditor("")
	if !state.Success() {
		t.Fatalf("Apply 失败：%+v", state)
	}
	if got := strings.Join(order, ","); got != "unset:DEEPSEEK_REF,mutate" {
		t.Fatalf("写序 = %q，官方是先删密钥再写设置", got)
	}
	if len(creds.unsets) != 1 || creds.unsets[0] != "DEEPSEEK_REF" {
		t.Fatalf("credentials.unset 调用 = %v", creds.unsets)
	}
	if len(creds.calls) != 0 {
		t.Fatalf("没打新密钥就不许调 credentials.set：%+v", creds.calls)
	}
	if official.modelsEdit != nil {
		t.Fatal("删密钥 + 写设置都成功后应关闭卡片")
	}
}

// TestModelsApplyRemovalOnlyClosesWithoutSettingsWrite 钉"只删密钥"这一路：
// 设置没改就一次 mutate 都不发（官方 :358 的 `ops.length > 0` 门）。
func TestModelsApplyRemovalOnlyClosesWithoutSettingsWrite(t *testing.T) {
	describe := modelsCredentialFixture(`{"providers":{"deepseek":{"apiKeyEnv":"DEEPSEEK_REF"}}}`)
	edit, err := newRetryEditorState(describe, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	edit.credentialRef = "DEEPSEEK_REF"
	edit.requestKeyRemoval()
	writer := &fakeWriter{namespace: apiclient.SettingsNamespaceView{NS: "llm", Revision: 5}}
	creds := &fakeCredentials{}
	plugin := &Plugin{}
	official := plugin.ensureOfficial()
	official.describe, official.writer, official.modelsEdit = describe, writer, edit
	official.credentialSet = creds.set
	official.credentialUnset = creds.unset

	state := plugin.applyModelsEditor("")
	if !state.Success() {
		t.Fatalf("只删密钥的 Apply 应成功：%+v", state)
	}
	if len(writer.mutateCalls) != 0 || len(writer.setCalls) != 0 {
		t.Fatalf("设置没改就不该写：mutate=%d set=%d", len(writer.mutateCalls), len(writer.setCalls))
	}
	if len(creds.unsets) != 1 {
		t.Fatalf("应恰好删一次：%v", creds.unsets)
	}
	if official.modelsEdit != nil {
		t.Fatal("删成功后应关闭卡片")
	}
}

// TestModelsApplySettingsFailureAfterRemovalKeepsCardOpen 钉官方那句
// 「A later settings failure cannot restore a secret that was deleted」：
// 删除已落库后设置写失败 → 卡片留着、removed 已置位（footer 的 dismiss 变 close、
// 说明变成"密钥已清除，请重试保存"），而且**不会**再删第二次。
func TestModelsApplySettingsFailureAfterRemovalKeepsCardOpen(t *testing.T) {
	describe := modelsCredentialFixture(`{"providers":{"deepseek":{"apiKeyEnv":"DEEPSEEK_REF","retryPolicy":{"mode":"normal"}}}}`)
	edit, err := newRetryEditorState(describe, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	edit.credentialRef = "DEEPSEEK_REF"
	edit.ensurePolicy()
	edit.policy["mode"] = "always"
	edit.requestKeyRemoval()
	writer := &fakeWriter{
		namespace: apiclient.SettingsNamespaceView{NS: "llm", Revision: 6},
		mutateErr: apiError(apiclient.SettingsCodeRevisionConflict, "revision moved", "reload", false),
	}
	creds := &fakeCredentials{}
	plugin := &Plugin{}
	official := plugin.ensureOfficial()
	official.describe, official.writer, official.modelsEdit = describe, writer, edit
	official.credentialSet = creds.set
	official.credentialUnset = creds.unset

	state := plugin.applyModelsEditor("")
	if state.Kind != WriteConflict {
		t.Fatalf("设置写冲突应如实报：%+v", state)
	}
	if official.modelsEdit == nil {
		t.Fatal("设置写失败不能让卡片关掉（用户还要重试）")
	}
	if !official.modelsEdit.keyRemovalApplied {
		t.Fatal("删除已落库就必须记成 removed（官方 :354）：否则重试会再删一次、footer 也不会变 close")
	}
	// 重试：设置写成功，**不该**再发一次 unset。
	writer.mutateErr = nil
	if retry := plugin.applyModelsEditor(""); !retry.Success() {
		t.Fatalf("重试失败：%+v", retry)
	}
	if len(creds.unsets) != 1 {
		t.Fatalf("removed 之后重试不许再删一次：%v", creds.unsets)
	}
}

// TestModelsApplyRemovalFailureKeepsDraft 钉"删除失败不许关卡"：
// 失败要如实归属凭据域，设置的写一次都不许发（删都失败就别把设置改了）。
func TestModelsApplyRemovalFailureKeepsDraft(t *testing.T) {
	describe := modelsCredentialFixture(`{"providers":{"deepseek":{"apiKeyEnv":"DEEPSEEK_REF","retryPolicy":{"mode":"normal"}}}}`)
	edit, err := newRetryEditorState(describe, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	edit.credentialRef = "DEEPSEEK_REF"
	edit.ensurePolicy()
	edit.policy["mode"] = "always"
	edit.requestKeyRemoval()
	writer := &fakeWriter{namespace: apiclient.SettingsNamespaceView{NS: "llm", Revision: 6}}
	creds := &fakeCredentials{unsetErr: errors.New("凭据库只读")}
	plugin := &Plugin{}
	official := plugin.ensureOfficial()
	official.describe, official.writer, official.modelsEdit = describe, writer, edit
	official.credentialSet = creds.set
	official.credentialUnset = creds.unset

	state := plugin.applyModelsEditor("")
	if state.Success() {
		t.Fatal("删除失败不能报成功")
	}
	if state.NS != credentialNS || state.Message != "凭据库只读" {
		t.Fatalf("失败归属/原文 = %q/%q", state.NS, state.Message)
	}
	if len(writer.mutateCalls) != 0 {
		t.Fatalf("删密钥失败时不该继续写设置：%d 次", len(writer.mutateCalls))
	}
	if official.modelsEdit == nil || official.modelsEdit.keyRemovalApplied {
		t.Fatal("失败必须保留草稿且不能记成已删")
	}
}

// TestModelsApplyUnsetNotWiredSaysSo 钉"壳没接不等于删成功"：
// Actions.CredentialUnset 为 nil 时给一句可读的失败，且一个请求都不发。
func TestModelsApplyUnsetNotWiredSaysSo(t *testing.T) {
	describe := modelsCredentialFixture(`{"providers":{"deepseek":{"apiKeyEnv":"DEEPSEEK_REF"}}}`)
	edit, err := newRetryEditorState(describe, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	edit.credentialRef = "DEEPSEEK_REF"
	edit.requestKeyRemoval()
	writer := &fakeWriter{namespace: apiclient.SettingsNamespaceView{NS: "llm", Revision: 5}}
	plugin := &Plugin{}
	official := plugin.ensureOfficial()
	official.describe, official.writer, official.modelsEdit = describe, writer, edit

	state := plugin.applyModelsEditor("")
	if state.Success() || !strings.Contains(state.Message, "credentials.unset") {
		t.Fatalf("壳没接 unset 必须说清楚：%+v", state)
	}
	if len(writer.mutateCalls) != 0 {
		t.Fatal("壳没接 unset 时不该继续写设置")
	}
}

// TestRequestModelsKeyRemovalOnlyForTheOpenCard 钉删除待办只落在**开着的这张卡**上：
// 官方 clear 闭包钉的是这张卡 describe 出来的 ref（ProviderEditor.tsx:496）。
func TestRequestModelsKeyRemovalOnlyForTheOpenCard(t *testing.T) {
	describe := modelsCredentialFixture(`{"providers":{"deepseek":{"apiKeyEnv":"DEEPSEEK_REF"}}}`)
	plugin := &Plugin{}
	official := plugin.ensureOfficial()
	official.describe = describe
	plugin.toggleModelsEditor("deepseek", apiclient.SettingsNSLLM, []string{"providers", "deepseek"})
	if official.modelsEdit == nil {
		t.Fatal("点开卡片应建草稿")
	}
	// 别的 route 的按键不许改这张卡。
	plugin.requestModelsKeyRemoval(nil, PanelRow{ID: modelsKeyRowPrefix + "other"})
	if official.modelsEdit.keyRemovalState() != modelsKeyRemovalNone {
		t.Fatal("别的 route 的按键不能把删除记到这张卡上")
	}
	plugin.requestModelsKeyRemoval(nil, PanelRow{ID: modelsKeyRowPrefix + "deepseek"})
	if official.modelsEdit.keyRemovalState() != modelsKeyRemovalPending {
		t.Fatalf("待删状态 = %v", official.modelsEdit.keyRemovalState())
	}
	if official.modelsEdit.keyRemovalRef != "DEEPSEEK_REF" {
		t.Fatalf("待删引用 = %q，应为开卡时钉住的那个", official.modelsEdit.keyRemovalRef)
	}
}

// TestModelsEditorCancelRefusedWhileSaving 钉官方 EditorFooter.tsx:52
// `disabled={props.busy}`：提交中 Cancel 拒收（点它不能把卡片关掉），空闲时能关。
//
// 为什么在逻辑层钉：视觉上按钮那时已经禁用、点不到，能钉的是这条忙门本身。
func TestModelsEditorCancelRefusedWhileSaving(t *testing.T) {
	describe := modelsCredentialFixture(`{"providers":{"deepseek":{"apiKeyEnv":"DEEPSEEK_REF"}}}`)
	plugin := &Plugin{}
	official := plugin.ensureOfficial()
	official.describe = describe
	plugin.toggleModelsEditor("deepseek", apiclient.SettingsNSLLM, []string{"providers", "deepseek"})
	if official.modelsEdit == nil {
		t.Fatal("点开卡片应建草稿")
	}
	cancel := retryFooterRows("deepseek", true, false)[0]
	official.write = WriteState{Kind: WriteSaving, NS: apiclient.SettingsNSLLM,
		Path: []string{"providers", "deepseek", "retryPolicy"}}
	plugin.activateModelsRow(cancel)
	if official.modelsEdit == nil {
		t.Fatal("提交中 Cancel 必须被拒（官方 disabled={props.busy}）")
	}
	official.write = WriteState{Kind: WriteIdle}
	plugin.activateModelsRow(cancel)
	if official.modelsEdit != nil {
		t.Fatal("空闲时 Cancel 应关掉卡片")
	}
}

// TestModelsPageFooterSaysCloseAfterKeyRemoval 钉官方 EditorFooter 第三档：
// 密钥删掉之后 dismiss 的文案是 'close'（ProviderEditor.tsx:683），而 Cancel
// 本身仍然可点（它是"关掉卡片"，不是"撤销删除"——删除撤不回来）。
func TestModelsPageFooterSaysCloseAfterKeyRemoval(t *testing.T) {
	describe := modelsCredentialFixture(`{"providers":{"deepseek":{"apiKeyEnv":"DEEPSEEK_REF"}}}`)
	snapshot := &apiclient.ModelsPageSnapshot{
		Registered: []apiclient.ProviderInfo{{ID: "deepseek"}},
		Configurable: []apiclient.ConfigurableProvider{{Provider: "deepseek", DisplayName: "DeepSeek",
			SettingsNamespace: apiclient.SettingsNSLLM, SettingsPath: []string{"providers", "deepseek"}}},
	}
	edit, err := newRetryEditorState(describe, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	edit.credentialRef = "DEEPSEEK_REF"
	edit.requestKeyRemoval()
	edit.keyRemovalApplied = true
	panel, _ := BuildPanel(describe, fullDesktopFacts()).Section("models")
	panel = buildModelsPage(panel, describe, nil, snapshot, edit, nil)

	cancel, ok := panel.Row("retry-deepseek-cancel")
	if !ok {
		t.Fatal("缺 footer 的 dismiss 行")
	}
	if cancel.Title != zhEditorClose {
		t.Fatalf("密钥已删后 dismiss 文案 = %q，官方是 %q", cancel.Title, zhEditorClose)
	}
	if cancel.Disabled {
		t.Fatal("dismiss 不许因删除而禁用（它只是关卡片）")
	}
	key, ok := panel.Row("key-deepseek")
	if !ok {
		t.Fatal("缺密钥行")
	}
	if !key.Disabled {
		t.Fatal("已删的密钥输入框必须停用（官方 disabled={… || keyRemoval?.removed === true}）")
	}
	if key.Desc != zhModelsKeyRemoveApplied {
		t.Fatalf("已删说明 = %q", key.Desc)
	}
}

// TestModelsKeyRowMaskComesOnlyFromTheHost 钉官方 `placeholder={saved.masked}`
// （ApiKeyInput.tsx:41）的一个边界：宿主没给预览时是**空串**，界面不许自己拼一个
// 假掩码画出去（掩码是凭据域给的事实，不是本页算出来的）。
func TestModelsKeyRowMaskComesOnlyFromTheHost(t *testing.T) {
	provider := ModelsProvider{Provider: "p", APIKeyEnv: "P_KEY",
		Credential: &apiclient.CredentialInfo{Configured: true, Source: credentialSourceCatalog, Writable: true}}
	row, ok := modelsKeyRow(provider, true, modelsKeyRemovalNone)
	if !ok {
		t.Fatal("应有密钥行")
	}
	if !row.Saved {
		t.Fatal("本地凭据存储有值 → 官方传 saved")
	}
	if row.SavedMask != "" {
		t.Fatalf("宿主没给 masked 时 SavedMask 必须是空串，实际 %q", row.SavedMask)
	}
	if row.Placeholder != "留空使用环境认证（P_KEY）" {
		t.Fatalf("没有掩码时占位退回 keyPlaceholderFallback：%q", row.Placeholder)
	}
	withMask := provider
	withMask.Credential = &apiclient.CredentialInfo{Configured: true, Source: credentialSourceCatalog,
		Writable: true, Masked: "sk-••••zzzz"}
	masked, _ := modelsKeyRow(withMask, true, modelsKeyRemovalNone)
	if masked.SavedMask != "sk-••••zzzz" || masked.Placeholder != "sk-••••zzzz" {
		t.Fatalf("有掩码时 SavedMask 与占位都取它：%+v", masked)
	}
}
