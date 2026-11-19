//go:build darwin || windows || linux

// official_models_cred_rest_visual_test.go —— #41 收尾 ②③④的**真实绘制 + 真按键**证据。
//
// 判据不是"有这一行"，而是三件事各自在真实窗口里成立：
//
//	③ 已存密钥那一态按 Backspace/Delete 会**请求**清除（官方 ApiKeyInput.tsx:44-49
//	   → saved.clear() → setKeyDraft({ref})，ProviderEditor.tsx:496），Apply 才真的
//	   发 `credentials.unset`（:350-356）；
//	④ 那枚掩码占位的颜色是官方 `--dsw-alias-label-primary`
//	   （ModelsSection.module.css:847-850），不是 Viva 默认的 muted；
//	② EditorFooter 提交中把 commit 文案换成「保存中…」、并拒收 Cancel
//	   （EditorFooter.tsx:52,63；locales.ts:210）。
package dssettings

import (
	"testing"

	"github.com/vivaui/viva/gfx"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// openModelsSavedKeyCard 打开设置 → Models → deepseek 卡，并让 describe 说这个
// profile 命名了凭据引用、凭据库里也有值（服务端掩码）。返回时卡片已展开。
func openModelsSavedKeyCard(t *testing.T, h *officialHarness, masked string) {
	t.Helper()
	h.mu.Lock()
	h.describe = modelsCredentialDescribe()
	h.credentials = map[string]apiclient.CredentialInfo{
		"DEEPSEEK_KEY": {Configured: true, Source: credentialSourceCatalog, Writable: true, Masked: masked},
	}
	h.mu.Unlock()
	if err := h.session.Driver().Key(gfx.KeyF2, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	h.click("settings-nav-models")
	h.click("settings-row-provider-edit-deepseek")
}

// TestModelsSavedKeyDraftClearRequestsRemovalAndApplyUnserts 钉 ③：
// 已存密钥那一态把草稿删空 = 官方 `onChange` 的 `”` 分支
// （ProviderEditor.tsx:507-509）→ `setKeyDraft({ref})`，也就是"请求清除已存密钥"；
// 真的删除发生在 Apply 里（:350-356）。
//
// 为什么不是"按 Backspace 就删密钥"（官方 ApiKeyInput.tsx:44-49）：聚焦输入框后
// Backspace/Delete 由**平台文本系统**消费（app/driver.go:679-685 → textKey →
// `TextDeleteBackward`），应用层的 `Box.OnKey` 收不到（ui/field.go:236-262）。
// 本仓因此接官方同一条判据的另一半：草稿变空（Backspace 删掉最后一个字符就是
// 这个动作）且已有本地存储密钥 → 请求清除。复制/剪切同理见文件末尾的说明。
func TestModelsSavedKeyDraftClearRequestsRemovalAndApplyUnserts(t *testing.T) {
	const masked = "sk-••••••3f9c"
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	openModelsSavedKeyCard(t, h, masked)

	o := h.p.ensureOfficial()
	o.mu.Lock()
	edit := o.modelsEdit
	o.mu.Unlock()
	if edit == nil || edit.credentialRef != "DEEPSEEK_KEY" {
		t.Fatalf("开卡应钉住引用：%+v", edit)
	}

	// 点击输入框拿到键盘（官方同样要先聚焦这个框），打一个字：那一刻 `saved`
	// 属性整块不传（ProviderEditor.tsx:492），掩码占位消失。
	if err := h.session.ClickLabel(zhModelsKeyInput); err != nil {
		t.Fatalf("点密钥框：%v", err)
	}
	if err := h.session.Type("x"); err != nil {
		t.Fatalf("打字：%v", err)
	}
	h.pump()
	h.pump()
	if h.hasText(masked) {
		t.Fatalf("有草稿时不该再画掩码占位；文本=%v", h.session.Texts())
	}
	if got := edit.keyRemovalState(); got != modelsKeyRemovalNone {
		t.Fatalf("有草稿时还不该进入待删：%v", got)
	}

	// Backspace 删掉最后一个字符 → 草稿为空 → 官方 `''` 分支 → 待删。
	if err := h.session.Driver().Key(gfx.KeyBackspace, 0); err != nil {
		t.Fatalf("按 Backspace：%v", err)
	}
	h.pump()
	h.pump()
	if draft := o.modelsKeyField.Text(); draft != "" {
		t.Fatalf("Backspace 之后草稿应为空，实际 %q", draft)
	}
	if got := edit.keyRemovalState(); got != modelsKeyRemovalPending {
		t.Fatalf("草稿清空后应进入「待删」：%v", got)
	}
	if !h.hasText("保存以清除已保存的 API key。之后使用环境认证（DEEPSEEK_KEY）。") {
		t.Fatalf("待删说明没画出来（官方 keyRemovePendingEnvironment）；文本=%v", h.session.Texts())
	}
	// 删除只是待办：Apply 之前一个请求都不许发。
	h.mu.Lock()
	unsetsBefore := len(h.credentialUnsets)
	h.mu.Unlock()
	if unsetsBefore != 0 || h.writer.callCount() != 0 {
		t.Fatalf("待删阶段不许发请求：unset=%d writes=%d", unsetsBefore, h.writer.callCount())
	}

	// Apply → 真的 `credentials.unset`，卡片按已应用关闭，且没有多余的 set。
	h.clickScrolled("settings-row-retry-deepseek-apply")
	for i := 0; i < 400; i++ {
		h.mu.Lock()
		done := len(h.credentialUnsets) > 0
		h.mu.Unlock()
		if done {
			break
		}
		h.pump()
	}
	h.mu.Lock()
	unsets := append([]string(nil), h.credentialUnsets...)
	sets := append([]credentialSetCall(nil), h.credentialSets...)
	h.mu.Unlock()
	if len(unsets) != 1 || unsets[0] != "DEEPSEEK_KEY" {
		t.Fatalf("credentials.unset = %v，应为开卡钉住的那个引用", unsets)
	}
	if len(sets) != 0 {
		t.Fatalf("没打新密钥就不许调 credentials.set：%+v", sets)
	}
	for i := 0; i < 400 && h.p.WriteState().Kind == WriteSaving; i++ {
		h.pump()
	}
	if !h.p.WriteState().Success() {
		t.Fatalf("只删密钥成功后应按已应用关闭：%+v", h.p.WriteState())
	}
	o.mu.Lock()
	open := o.modelsEdit != nil
	o.mu.Unlock()
	if open {
		t.Fatal("Apply 成功后编辑卡应关闭")
	}
}

// TestModelsSavedKeyMaskPaintsOfficialPlaceholderColor 钉 ④：
// 已存密钥的掩码占位必须是官方 label-primary（不是 Viva 默认的 muted），
// 且草稿一出现就换回普通占位.
func TestModelsSavedKeyMaskPaintsOfficialPlaceholderColor(t *testing.T) {
	const masked = "sk-••••••abcd"
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	openModelsSavedKeyCard(t, h, masked)

	found := false
	for _, text := range h.canvas.Texts() {
		if text.Value != masked {
			continue
		}
		found = true
		// ModelsSection.module.css:847-850：
		// `.savedKeyInput .input::placeholder { color: var(--dsw-alias-label-primary); opacity: 1 }`
		if text.Style.Color != h.theme.LabelPrimary {
			t.Fatalf("掩码占位色 = %+v，官方是 label-primary %+v（不是 muted）",
				text.Style.Color, h.theme.LabelPrimary)
		}
		if text.Style.Color == h.theme.LabelTertiary {
			t.Fatal("掩码占位落在了 weak/tertiary 色上——那是没接官方 .savedKeyInput 的样子")
		}
	}
	if !found {
		t.Fatalf("掩码占位没画出来；文本=%v", h.session.Texts())
	}

	// 打一个字符：官方那一刻不再传 `saved`，掩码消失、换回 keyPlaceholderFallback。
	if err := h.session.ClickLabel(zhModelsKeyInput); err != nil {
		t.Fatalf("点密钥框：%v", err)
	}
	if err := h.session.Type("x"); err != nil {
		t.Fatalf("打字：%v", err)
	}
	h.pump()
	if h.hasText(masked) {
		t.Fatalf("有草稿时不该再画掩码占位；文本=%v", h.session.Texts())
	}
	// 官方那一刻说明换成 `keyReplacePending`（ProviderEditor.tsx:446）：
	// 「保存以替换已保存的 API key。」——否则用户会以为这一笔是"保持原密钥"。
	if !h.hasText(zhModelsKeyReplacePending) {
		t.Fatalf("有草稿时应换成 %q；文本=%v", zhModelsKeyReplacePending, h.session.Texts())
	}
}

// TestModelsEditorFooterSwapsToApplyingAndRefusesCancel 钉 ②：
// 提交中 commit 文案换成「保存中…」（EditorFooter.tsx:63 + locales.ts:210）；
// 同一时刻 Cancel 被官方 `disabled={props.busy}` 拒收（:52）。
//
// 拒收本身在 `activateModelsRow` 的 busy 门里判（视觉上按钮已禁用，点不到），
// 所以这里钉可见文案，拒收由单测 TestModelsEditorCancelRefusedWhileSaving 钉逻辑。
func TestModelsEditorFooterSwapsToApplyingAndRefusesCancel(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	openModelsSavedKeyCard(t, h, "sk-••••7777")

	if hasExactText(h, zhEditorApplying) {
		t.Fatal("空闲时不该出现「保存中…」")
	}
	if !hasExactText(h, zhEditorApply) || !hasExactText(h, zhEditorCancel) {
		t.Fatalf("空闲时应有「保存」「取消」；文本=%v", h.session.Texts())
	}

	h.p.setWriteState(WriteState{Kind: WriteSaving, NS: apiclient.SettingsNSLLM,
		Path: []string{"providers", "deepseek", "retryPolicy"}})
	h.pump()
	if !h.hasText(zhEditorApplying) {
		t.Fatalf("提交中 commit 文案应换成 %q；文本=%v", zhEditorApplying, h.session.Texts())
	}
	// 精确匹配一眼：`HasText` 是子串匹配，而「保存中…」里也含「保存」，
	// 所以这里直接查画布上的独立文本。
	if hasExactText(h, zhEditorApply) {
		t.Fatalf("提交中 commit 按钮不该还画着空闲文案 %q；文本=%v", zhEditorApply, h.session.Texts())
	}
	if !hasExactText(h, zhEditorApplying) {
		t.Fatalf("提交中 commit 按钮应有独立的 %q 文本；文本=%v", zhEditorApplying, h.session.Texts())
	}

	// 回到空闲：文案变回「保存」，Cancel 重新可用并真的能关卡片。
	h.p.setWriteState(WriteState{Kind: WriteIdle})
	h.pump()
	if hasExactText(h, zhEditorApplying) {
		t.Fatalf("空闲后不该还写着「保存中…」；文本=%v", h.session.Texts())
	}
	h.clickScrolled("settings-row-retry-deepseek-cancel")
	o := h.p.ensureOfficial()
	o.mu.Lock()
	open := o.modelsEdit != nil
	o.mu.Unlock()
	if open {
		t.Fatal("空闲时 Cancel 应关掉卡片")
	}
}

// hasExactText 报告画布上有没有**一模一样**的一段文本（`Session.HasText` 是子串
// 匹配，「保存中…」会让「保存」也算命中，判不了按钮文案）。
func hasExactText(h *officialHarness, want string) bool {
	for _, text := range h.canvas.Texts() {
		if text.Value == want {
			return true
		}
	}
	return false
}
