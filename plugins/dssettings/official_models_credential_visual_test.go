//go:build darwin || windows || linux

// official_models_credential_visual_test.go —— #41 (a) 的**真实绘制 + 真点击**证据。
//
// 判据不是"有这一行"：密钥框必须有真命中区、能被点中聚焦、能收到键盘、Apply 后
// 真的调到 `credentials.set` 且引用名与值都对（官方 ApiKeyInput.tsx:1-68 +
// operations.ts:78-81）。凭据状态来自 `credentials.describe`（store.ts:276-291）。
package dssettings

import (
	"encoding/json"
	"strings"

	"github.com/vivaui/viva/gfx"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// modelsCredentialDescribe 造一份"provider 命名了凭据引用"的设置文档：
// 官方 keyRef 推导只在**没有** apiKeyEnv 时发生（ProviderEditor.tsx:167-181），
// 所以这里命名一个，用来验证"已存密钥 → 掩码占位 + 成功色说明"那条分支。
func modelsCredentialDescribe() *apiclient.SettingsDescribeResult {
	describe := describeFixture()
	for index := range describe.Namespaces {
		if describe.Namespaces[index].NS == apiclient.SettingsNSLLM {
			describe.Namespaces[index].Value = json.RawMessage(`{"providers":{"deepseek":{"apiKeyEnv":"DEEPSEEK_KEY"}}}`)
		}
	}
	return describe
}

func TestModelsKeyFieldRendersAndStoresCredential(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()

	h.mu.Lock()
	h.describe = modelsCredentialDescribe()
	h.credentials = map[string]apiclient.CredentialInfo{
		"DEEPSEEK_KEY": {Configured: true, Source: credentialSourceCatalog, Writable: true, Masked: "sk-••••••3f9c"},
	}
	h.mu.Unlock()

	if err := h.session.Driver().Key(gfx.KeyF2, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	h.click("settings-nav-models")
	h.click("settings-row-provider-edit-deepseek")

	// ① 字段真的画出来了，且已存密钥用服务端掩码当占位（官方 `saved.masked`）。
	if !h.hasText(zhModelsKeyInput) {
		t.Fatalf("Models 卡里没有密钥字段的 label；文本=%v", h.session.Texts())
	}
	if !h.hasText("sk-••••••3f9c") {
		t.Fatalf("没有把服务端掩码画成占位；文本=%v", h.session.Texts())
	}
	if !h.hasText(zhModelsKeySaved) {
		t.Fatalf("缺官方的「使用已保存的 API key」说明；文本=%v", h.session.Texts())
	}
	// ② 行头那枚凭据点由 describe 决定（已配置 → 成功点）。
	models, _ := h.p.PanelModel().Section("models")
	head, ok := models.Row("provider-edit-deepseek")
	if !ok || head.Badge != PanelBadgeCredentialConfigured {
		t.Fatalf("provider 行头的凭据点 = %+v", head)
	}
	// 密钥行必须带真命中区（不是只画了字）。
	h.hitFor("settings-models-key")
	// label 后面那枚 `?` 帮助钮（官方 `.helpButton`）也必须是真元素，且正文里带引用名
	// （官方 `keyHelp` 的 `{env}` 实参 = visibleRefs.join(' / ')，ProviderEditor.tsx:460-462）。
	h.hitFor("settings-models-key-help")
	keyRow, ok := models.Row("key-deepseek")
	if !ok {
		t.Fatal("缺密钥行")
	}
	if !strings.Contains(keyRow.HelpText, "DEEPSEEK_KEY") {
		t.Fatalf("帮助正文缺引用名：%q", keyRow.HelpText)
	}
	if !strings.Contains(keyRow.HelpText, "$DSH_HOME/.env") {
		t.Fatalf("帮助正文不是官方 keyHelp 原文：%q", keyRow.HelpText)
	}

	// ③ 点中字段 → 打字 → Apply。草稿只在内存，Apply 之前一次凭据写都不该发生。
	if err := h.session.ClickLabel(zhModelsKeyInput); err != nil {
		t.Fatalf("点密钥框：%v", err)
	}
	if err := h.session.Type("  sk-live-777  "); err != nil {
		t.Fatalf("往密钥框打字：%v", err)
	}
	h.pump()
	h.mu.Lock()
	writes := len(h.credentialSets)
	h.mu.Unlock()
	if writes != 0 {
		t.Fatalf("打字阶段不许写凭据，却写了 %d 次", writes)
	}

	// ④ 草稿非空时官方的 `keyToggle` 才出现（ApiKeyInput.tsx:31）：它必须有真命中区，
	// 点一下能把密码态翻成明文态，且不动草稿。
	h.hitFor("settings-models-key-toggle")
	if err := h.session.ClickLabel(zhModelsKeyReveal); err != nil {
		t.Fatalf("点显隐钮：%v", err)
	}
	h.pump()
	o := h.p.ensureOfficial()
	if o.modelsKeyField.Secure {
		t.Fatal("点「显示密钥」之后字段不该还是密码态")
	}
	if got := o.modelsKeyField.Text(); got != "  sk-live-777  " {
		t.Fatalf("显隐不该改草稿，实际 %q", got)
	}
	if err := h.session.ClickLabel(zhModelsKeyHide); err != nil {
		t.Fatalf("再点显隐钮：%v", err)
	}
	h.pump()
	if !o.modelsKeyField.Secure {
		t.Fatal("再点一次应回到密码态")
	}

	h.clickScrolled("settings-row-retry-deepseek-apply")
	for i := 0; i < 400; i++ {
		h.mu.Lock()
		done := len(h.credentialSets) > 0
		h.mu.Unlock()
		if done {
			break
		}
		h.pump()
	}
	h.mu.Lock()
	sets := append([]credentialSetCall(nil), h.credentialSets...)
	h.mu.Unlock()
	if len(sets) != 1 {
		t.Fatalf("Apply 后 credentials.set 调用 = %d 次，应为 1", len(sets))
	}
	if sets[0].ref != "DEEPSEEK_KEY" || sets[0].value != "sk-live-777" {
		t.Fatalf("credentials.set = %+v（引用必须是 profile 命名的那个，值必须 trim）", sets[0])
	}
	// ④ 卡片关闭 + 输入框按官方 `onClose` 丢掉草稿。
	for i := 0; i < 400 && h.p.WriteState().Kind == WriteSaving; i++ {
		h.pump()
	}
	if !h.p.WriteState().Success() {
		t.Fatalf("凭据写成功后状态应为已保存：%+v", h.p.WriteState())
	}
	o.mu.Lock()
	open := o.modelsEdit != nil
	o.mu.Unlock()
	if open {
		t.Fatal("Apply 成功后编辑卡应关闭")
	}
	h.pump()
	if got := o.modelsKeyField.Text(); got != "" {
		t.Fatalf("关卡后输入框应清空，仍有 %q", got)
	}
}

func TestModelsKeyFieldRefusesIllegalDraftWithoutWriting(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	h.mu.Lock()
	h.describe = modelsCredentialDescribe()
	h.mu.Unlock()

	if err := h.session.Driver().Key(gfx.KeyF2, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	h.click("settings-nav-models")
	h.click("settings-row-provider-edit-deepseek")
	if err := h.session.ClickLabel(zhModelsKeyInput); err != nil {
		t.Fatalf("点密钥框：%v", err)
	}
	// 官方 `ENV_LINE`：粘贴进来的 `NAME=value` 一律拒（apiKey.ts:23,51-58）。
	if err := h.session.Type("MY_KEY=sk-1"); err != nil {
		t.Fatalf("打字：%v", err)
	}
	h.pump()
	h.clickScrolled("settings-row-retry-deepseek-apply")
	for i := 0; i < 200; i++ {
		h.pump()
	}
	state := h.p.WriteState()
	if state.Kind != WriteInvalid || !strings.Contains(state.Message, zhModelsKeyIllegalCharacters) {
		t.Fatalf("非法密钥应被拒并可读：%+v", state)
	}
	if !h.hasText("校验失败") {
		t.Fatalf("失败态必须画在右页上；文本=%v", h.session.Texts())
	}
	h.mu.Lock()
	writes := len(h.credentialSets)
	h.mu.Unlock()
	if writes != 0 {
		t.Fatalf("非法密钥不许落库，却写了 %d 次", writes)
	}
	o := h.p.ensureOfficial()
	o.mu.Lock()
	open := o.modelsEdit != nil
	o.mu.Unlock()
	if !open {
		t.Fatal("校验失败必须保留草稿（卡片不关）")
	}
}
