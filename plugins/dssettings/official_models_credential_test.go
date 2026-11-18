// official_models_credential_test.go —— #41 差异 (a) 的判据测试。
//
// 每个用例的**依据**写在注释里（官方 file:line），断言的是官方那条规则，
// 不是"当前实现碰巧算出来的值"。
package dssettings

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// TestDeriveCredentialKeyRefIsTheOfficialFormula 钉官方 store.ts:125-127 的推导式：
// 大写 + 把**连续**的非字母数字折成**一个** `_` + `_API_KEY`。
func TestDeriveCredentialKeyRefIsTheOfficialFormula(t *testing.T) {
	cases := map[string]string{
		"deepseek-official": "DEEPSEEK_OFFICIAL_API_KEY",
		"openai":            "OPENAI_API_KEY",
		// 官方 `/[^A-Z0-9]+/g` 是**一次匹配一串**，所以 `a-b__c` 只折成一个下划线。
		"a-b__c":    "A_B_C_API_KEY",
		"vendor.ai": "VENDOR_AI_API_KEY",
		// 首尾的非字母数字也会被折成下划线（官方不 trim）。
		"-edge-": "_EDGE__API_KEY",
	}
	for provider, want := range cases {
		if got := deriveCredentialKeyRef(provider); got != want {
			t.Errorf("deriveCredentialKeyRef(%q) = %q，官方口径是 %q", provider, got, want)
		}
	}
}

// TestCredentialRefPrefersTheNamedProfileReference 钉官方 refFor
// （ProviderEditor.tsx:167-181）与 apiKeyEnvOf（store.ts:172-182）：
// 生效 profile 里命名了 `apiKeyEnv` 就用它，否则用推导值；空串/非字符串等于没命名。
func TestCredentialRefPrefersTheNamedProfileReference(t *testing.T) {
	describe := &apiclient.SettingsDescribeResult{Namespaces: []apiclient.SettingsNamespaceView{
		{NS: "llm", Value: json.RawMessage(`{"providers":{"named":{"apiKeyEnv":"CUSTOM_REF"},"blank":{"apiKeyEnv":""},"numeric":{"apiKeyEnv":7},"derived":{}}}`)},
		// whole-section 形态：settingsPath 为空时读的是命名空间根上的 apiKeyEnv。
		{NS: "llm-deepseek", Value: json.RawMessage(`{"apiKeyEnv":"TOP_LEVEL_REF"}`)},
	}}
	cases := []struct {
		provider string
		ns       string
		path     []string
		want     string
	}{
		{"named", "llm", []string{"providers", "named"}, "CUSTOM_REF"},
		{"blank", "llm", []string{"providers", "blank"}, "BLANK_API_KEY"},
		{"numeric", "llm", []string{"providers", "numeric"}, "NUMERIC_API_KEY"},
		{"derived", "llm", []string{"providers", "derived"}, "DERIVED_API_KEY"},
		{"deepseek-official", "llm-deepseek", nil, "TOP_LEVEL_REF"},
		{"missing-ns", "unknown-ns", nil, "MISSING_NS_API_KEY"},
	}
	for _, c := range cases {
		if got := credentialRefFor(describe, c.provider, c.ns, c.path); got != c.want {
			t.Errorf("credentialRefFor(%s) = %q，应为 %q", c.provider, got, c.want)
		}
	}
	if _, ok := profileAPIKeyEnv(nil, "llm", nil); ok {
		t.Error("describe 为 nil 时不能凭空给出命名引用")
	}
}

// TestModelsCredentialRefsMatchTheOfficialSet 钉官方 store.ts:274-275：
// 命名了取命名那一个；没命名取推导 + nativeAuth.env；account 行整条跳过；结果去重。
func TestModelsCredentialRefsMatchTheOfficialSet(t *testing.T) {
	rows := []ModelsProvider{
		{Provider: "deepseek-account"},
		{Provider: "named-route", APIKeyEnv: "NAMED_REF"},
		{Provider: "derived-route"},
		{Provider: "native-route", NativeAuth: &apiclient.ProviderNativeAuth{Env: []string{"AWS_PROFILE"}}},
		{Provider: "named-route-2", APIKeyEnv: "NAMED_REF"},
	}
	got := modelsCredentialRefs(rows)
	// native-route 没命名引用：官方向导出的 ref + 它自己声明的 env，去重后一起问。
	// DERIVED_ROUTE_API_KEY 已经被 derived-route 问过，所以这里只剩 NATIVE_ROUTE_API_KEY。
	want := []string{"NAMED_REF", "DERIVED_ROUTE_API_KEY", "NATIVE_ROUTE_API_KEY", "AWS_PROFILE"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("refs = %v，官方口径是 %v", got, want)
	}
}

// TestApplyModelsCredentialsKeepsNamedAndDerivedApart 钉官方 store.ts:293-310。
func TestApplyModelsCredentialsKeepsNamedAndDerivedApart(t *testing.T) {
	rows := []ModelsProvider{
		{Provider: "deepseek-account"},
		{Provider: "named", APIKeyEnv: "NAMED_REF"},
		{Provider: "derived", CredentialRef: "DERIVED_API_KEY"},
		{Provider: "unanswered", APIKeyEnv: "MISSING_REF"},
	}
	applyModelsCredentials(rows, func(refs []string) map[string]apiclient.CredentialInfo {
		return map[string]apiclient.CredentialInfo{
			"NAMED_REF":       {Configured: true, Source: credentialSourceCatalog, Writable: true},
			"DERIVED_API_KEY": {Configured: false},
			// MISSING_REF 刻意不答：缺一个键 = 不知道，不是"未配置"。
		}
	})
	if rows[1].Credential == nil || !rows[1].Credential.Configured {
		t.Fatalf("命名引用应落到 row.Credential：%+v", rows[1].Credential)
	}
	if rows[1].DerivedCredential != nil {
		t.Fatal("命名引用不该再算一份 derived")
	}
	if rows[2].Credential != nil || rows[2].DerivedCredential == nil {
		t.Fatalf("没命名引用时结论只能落 DerivedCredential：%+v", rows[2])
	}
	if rows[3].Credential != nil {
		t.Fatal("describe 没答的 ref 必须留 nil（不知道 ≠ 未配置）")
	}
	if rows[0].Credential != nil || rows[0].CredentialRef != "" {
		t.Fatal("account 行不参与凭据 join")
	}

	// 壳没接（nil）：所有格保持空，且不 panic。
	empty := []ModelsProvider{{Provider: "x", APIKeyEnv: "X_REF"}}
	applyModelsCredentials(empty, nil)
	if empty[0].Credential != nil {
		t.Fatal("壳没接时不许编一个状态出来")
	}
}

// TestApplyModelsCredentialsChunksAtSixtyFour 钉官方 store.ts:278 的 64 条上限。
func TestApplyModelsCredentialsChunksAtSixtyFour(t *testing.T) {
	rows := make([]ModelsProvider, 0, 70)
	for i := 0; i < 70; i++ {
		rows = append(rows, ModelsProvider{Provider: "route" + string(rune('a'+i%26)) + strings.Repeat("x", i)})
	}
	var sizes []int
	applyModelsCredentials(rows, func(refs []string) map[string]apiclient.CredentialInfo {
		sizes = append(sizes, len(refs))
		return nil
	})
	if len(sizes) != 2 || sizes[0] != 64 || sizes[1] != 6 {
		t.Fatalf("describe 分片 = %v，官方是 64 + 余数", sizes)
	}
}

// TestModelsCredentialBadgeOnlyForNamedReference 钉官方 ModelsSection.tsx:459-500。
func TestModelsCredentialBadgeOnlyForNamedReference(t *testing.T) {
	cases := []struct {
		name      string
		row       ModelsProvider
		wantLabel string
		wantMiss  bool
		wantOK    bool
	}{
		{"没命名引用 → 不画点", ModelsProvider{Provider: "p", Credential: &apiclient.CredentialInfo{Configured: true}}, "", false, false},
		{"describe 没答 → 不画点", ModelsProvider{Provider: "p", APIKeyEnv: "REF"}, "", false, false},
		{"已配置 → 成功点", ModelsProvider{Provider: "p", APIKeyEnv: "REF", Credential: &apiclient.CredentialInfo{Configured: true}},
			zhModelsCredentialConfigured, false, true},
		{"明确未配置 → 缺失点", ModelsProvider{Provider: "p", APIKeyEnv: "REF", Credential: &apiclient.CredentialInfo{}},
			zhModelsCredentialMissing, true, true},
	}
	for _, c := range cases {
		label, missing, ok := modelsCredentialBadge(c.row)
		if label != c.wantLabel || missing != c.wantMiss || ok != c.wantOK {
			t.Errorf("%s：got (%q,%v,%v)，want (%q,%v,%v)", c.name, label, missing, ok, c.wantLabel, c.wantMiss, c.wantOK)
		}
	}
}

// TestModelsKeyStateFollowsTheOfficialFourBranches 钉官方 ProviderEditor.tsx:437-496。
func TestModelsKeyStateFollowsTheOfficialFourBranches(t *testing.T) {
	// ① profile 没命名引用：placeholder 用无实参那一句，说明行整块不画。
	noName := modelsKeyState(ModelsProvider{Provider: "p"})
	if noName.Placeholder != zhModelsKeyPlaceholder || noName.Hint != "" || noName.Known {
		t.Fatalf("无命名引用：%+v", noName)
	}

	// ② describe 没答：placeholder 走回退那句，说明是"尚未确认"（中性色）。
	unanswered := modelsKeyState(ModelsProvider{Provider: "p", APIKeyEnv: "P_API_KEY"})
	if unanswered.Placeholder != "留空使用环境认证（P_API_KEY）" || unanswered.Hint != zhModelsKeyFallbackUnchecked {
		t.Fatalf("describe 没答：%+v", unanswered)
	}
	if unanswered.Tone != modelsKeyHintMuted {
		t.Fatalf("未确认必须是中性色（官方 advancedHint）：%q", unanswered.Tone)
	}

	// ③ 本地存储层有值：掩码当 placeholder + 成功色"使用已保存的"。
	saved := modelsKeyState(ModelsProvider{Provider: "p", APIKeyEnv: "P_API_KEY",
		Credential: &apiclient.CredentialInfo{Configured: true, Source: credentialSourceCatalog, Writable: true, Masked: "sk-••••1234"}})
	if !saved.Saved || saved.Placeholder != "sk-••••1234" || saved.Hint != zhModelsKeySaved || saved.Tone != modelsKeyHintSuccess {
		t.Fatalf("已存密钥：%+v", saved)
	}
	if !saved.Writable {
		t.Fatal("describe 说可写就必须可写")
	}

	// ④ env 层供得上：成功色"已检测到环境变量"。
	env := modelsKeyState(ModelsProvider{Provider: "p", APIKeyEnv: "P_API_KEY",
		Credential: &apiclient.CredentialInfo{Configured: true, Source: "env"}})
	if env.Saved || env.Hint != "已检测到环境变量（P_API_KEY）" || env.Tone != modelsKeyHintSuccess {
		t.Fatalf("env 回退：%+v", env)
	}

	// ⑤ 明确没有：warn 色 + 官方那句"请填写或设置环境变量"。
	missing := modelsKeyState(ModelsProvider{Provider: "p", APIKeyEnv: "P_API_KEY",
		Credential: &apiclient.CredentialInfo{}})
	if missing.Hint != "未检测到环境变量。请填写 API 密钥，或设置环境变量 P_API_KEY。" || missing.Tone != modelsKeyHintWarn {
		t.Fatalf("明确缺失：%+v", missing)
	}
}

// TestModelsAPIKeyFailureMirrorsApiKeyTs 钉官方 apiKey.ts:51-58 的五条判据。
func TestModelsAPIKeyFailureMirrorsApiKeyTs(t *testing.T) {
	cases := map[string]string{
		"":                "",
		"sk-abc123":       "",
		"sk-a_b-C.9":      "",
		"   ":             zhModelsKeyBlank,
		"\t\n":            zhModelsKeyBlank,
		"OPENAI_KEY=sk-1": zhModelsKeyIllegalCharacters,
		"`sk-abc`":        zhModelsKeyIllegalCharacters,
		"\"sk-abc\"":      zhModelsKeyIllegalCharacters,
		"'sk-abc'":        zhModelsKeyIllegalCharacters,
		"sk-abc 123":      zhModelsKeyIllegalCharacters, // 空格不在 \x21-\x7E 里
		"sk-密钥":           zhModelsKeyIllegalCharacters, // 非 ASCII
		"sk-abc=":         "",                           // 尾随 '=' 仍是合法字符
		"openai_key=sk-1": "",                           // 名字必须全大写（官方 ENV_LINE）
		"sk-abc==def":     "",                           // '=' 后面不是 '=' 才像赋值行
	}
	for draft, want := range cases {
		if got := modelsAPIKeyFailure(draft); got != want {
			t.Errorf("modelsAPIKeyFailure(%q) = %q，want %q", draft, got, want)
		}
	}
}

// TestModelsKeyHiddenOnlyForKeylessNativeAuth 钉官方 keylessNativeAuth
// （ProviderEditor.tsx:420-424,461-462）。
func TestModelsKeyHiddenOnlyForKeylessNativeAuth(t *testing.T) {
	if !modelsKeyHidden(ModelsProvider{Provider: "p", NativeAuth: &apiclient.ProviderNativeAuth{OAuth: true}}) {
		t.Fatal("oauth 且无 env 的 provider 不该画密钥框")
	}
	if modelsKeyHidden(ModelsProvider{Provider: "p", NativeAuth: &apiclient.ProviderNativeAuth{Env: []string{"AWS_PROFILE"}}}) {
		t.Fatal("有 env 的 provider 要画密钥框（环境回退说明就靠它）")
	}
	if modelsKeyHidden(ModelsProvider{Provider: "p", APIKeyEnv: "P_API_KEY", NativeAuth: &apiclient.ProviderNativeAuth{OAuth: true}}) {
		t.Fatal("命名了引用就不算 keyless（官方 nativeAuth 此时是 undefined）")
	}
}

// modelsCredentialFixture 造一份带 llm.providers 的 describe（provider 卡可打开）。
func modelsCredentialFixture(profile string) *apiclient.SettingsDescribeResult {
	describe := describeFixture()
	for index := range describe.Namespaces {
		if describe.Namespaces[index].NS == apiclient.SettingsNSLLM {
			describe.Namespaces[index].Value = json.RawMessage(profile)
		}
	}
	return describe
}

// TestModelsPageDrawsKeyRowAndCredentialDot 钉"驱动到可见面"这一层：
// 卡片打开时密钥行在、provider 行头带凭据点（官方 ModelsSection.tsx:459-500）。
func TestModelsPageDrawsKeyRowAndCredentialDot(t *testing.T) {
	describe := modelsCredentialFixture(`{"providers":{"deepseek":{"apiKeyEnv":"DEEPSEEK_REF","retryPolicy":{"mode":"normal"}}}}`)
	snapshot := &apiclient.ModelsPageSnapshot{
		Registered: []apiclient.ProviderInfo{{ID: "deepseek"}},
		Configurable: []apiclient.ConfigurableProvider{
			{Provider: "deepseek", DisplayName: "DeepSeek", SettingsNamespace: apiclient.SettingsNSLLM, SettingsPath: []string{"providers", "deepseek"}},
		},
	}
	status := func(refs []string) map[string]apiclient.CredentialInfo {
		if len(refs) != 1 || refs[0] != "DEEPSEEK_REF" {
			t.Fatalf("describe 问的 refs = %v，命名引用应只问那一个", refs)
		}
		return map[string]apiclient.CredentialInfo{"DEEPSEEK_REF": {Configured: true, Source: credentialSourceCatalog, Writable: true, Masked: "sk-••••9"}}
	}
	edit, err := newRetryEditorState(describe, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	edit.credentialRef = "DEEPSEEK_REF"
	panel, _ := BuildPanel(describe, fullDesktopFacts()).Section("models")
	panel = buildModelsPage(panel, describe, nil, snapshot, edit, status)

	head, ok := panel.Row("provider-edit-deepseek")
	if !ok {
		t.Fatal("缺 provider 行")
	}
	if head.Badge != PanelBadgeCredentialConfigured || head.BadgeLabel != zhModelsCredentialConfigured {
		t.Fatalf("provider 行头的凭据点 = %q/%q", head.Badge, head.BadgeLabel)
	}
	key, ok := panel.Row("key-deepseek")
	if !ok {
		t.Fatal("provider 卡打开时必须有密钥行")
	}
	if key.Kind != ItemSecret || key.Title != zhModelsKeyInput {
		t.Fatalf("密钥行形状 = %+v", key)
	}
	if key.Placeholder != "sk-••••9" || key.Desc != zhModelsKeySaved || key.Tone != PanelToneSuccess {
		t.Fatalf("已存密钥的可见面 = placeholder %q / desc %q / tone %q", key.Placeholder, key.Desc, key.Tone)
	}
	if len(key.Path) != 0 {
		t.Fatal("密钥行不許带 settings 写入路径（它走凭据域）")
	}

	// 明确未配置 → 行头换成缺失点（官方 credentialDotMissing）。
	missingStatus := func([]string) map[string]apiclient.CredentialInfo {
		return map[string]apiclient.CredentialInfo{"DEEPSEEK_REF": {}}
	}
	panel, _ = BuildPanel(describe, fullDesktopFacts()).Section("models")
	panel = buildModelsPage(panel, describe, nil, snapshot, nil, missingStatus)
	head, _ = panel.Row("provider-edit-deepseek")
	if head.Badge != PanelBadgeCredentialMissing {
		t.Fatalf("明确未配置时行头应是缺失点：%q", head.Badge)
	}
	if _, ok := panel.Row("key-deepseek"); ok {
		t.Fatal("卡片没打开时不该有密钥行")
	}
}

// fakeCredentials 记录 credentials.set 的调用。
type fakeCredentials struct {
	calls []struct{ ref, value string }
	err   error
	log   *[]string
}

func (f *fakeCredentials) set(ref, value string) error {
	f.calls = append(f.calls, struct{ ref, value string }{ref, value})
	if f.log != nil {
		*f.log = append(*f.log, "credential:"+ref)
	}
	return f.err
}

// TestModelsApplyKeyOnlyDoesNotTouchSettings 钉官方 ProviderEditor.tsx:364-368：
// 只打密钥时**不发** settings.mutate，只发一次 credentials.set。
func TestModelsApplyKeyOnlyDoesNotTouchSettings(t *testing.T) {
	describe := modelsCredentialFixture(`{"providers":{"deepseek":{"apiKeyEnv":"DEEPSEEK_REF"}}}`)
	edit, err := newRetryEditorState(describe, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	edit.credentialRef = "DEEPSEEK_REF"
	writer := &fakeWriter{namespace: apiclient.SettingsNamespaceView{NS: "llm", Revision: 5}}
	creds := &fakeCredentials{}
	plugin := &Plugin{}
	official := plugin.ensureOfficial()
	official.describe, official.writer, official.modelsEdit = describe, writer, edit
	official.credentialSet = creds.set

	state := plugin.applyModelsEditor("  sk-live-12345  ")
	if !state.Success() {
		t.Fatalf("只打密钥的 Apply 应成功：%+v", state)
	}
	if len(writer.mutateCalls) != 0 || len(writer.setCalls) != 0 {
		t.Fatalf("只打密钥不该写设置：set=%d mutate=%d", len(writer.setCalls), len(writer.mutateCalls))
	}
	if len(creds.calls) != 1 || creds.calls[0].ref != "DEEPSEEK_REF" || creds.calls[0].value != "sk-live-12345" {
		t.Fatalf("credentials.set 调用 = %+v（值必须 trim 过，官方 keyValue）", creds.calls)
	}
	if official.modelsEdit != nil {
		t.Fatal("Apply 成功后应关闭编辑卡")
	}
}

// TestModelsApplyWritesSettingsBeforeCredential 钉官方顺序
// （ProviderEditor.tsx:355-378）：settings.mutate 在前、credentials.set 在后。
func TestModelsApplyWritesSettingsBeforeCredential(t *testing.T) {
	describe := modelsCredentialFixture(`{"providers":{"deepseek":{"apiKeyEnv":"DEEPSEEK_REF","retryPolicy":{"mode":"normal"}}}}`)
	edit, err := newRetryEditorState(describe, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	edit.credentialRef = "DEEPSEEK_REF"
	edit.ensurePolicy()
	edit.policy["mode"] = "always"
	var order []string
	writer := &orderedWriter{namespace: apiclient.SettingsNamespaceView{NS: "llm", Revision: 6}, log: &order}
	creds := &fakeCredentials{log: &order}
	plugin := &Plugin{}
	official := plugin.ensureOfficial()
	official.describe, official.writer, official.modelsEdit = describe, writer, edit
	official.credentialSet = creds.set

	state := plugin.applyModelsEditor("sk-order-1")
	if !state.Success() {
		t.Fatalf("Apply 失败：%+v", state)
	}
	if strings.Join(order, ",") != "mutate,credential:DEEPSEEK_REF" {
		t.Fatalf("写序 = %v，官方是先设置后凭据", order)
	}
}

// TestModelsApplyCredentialFailureKeepsCardOpen 钉"密钥没落库就不许关卡"。
func TestModelsApplyCredentialFailureKeepsCardOpen(t *testing.T) {
	describe := modelsCredentialFixture(`{"providers":{"deepseek":{"apiKeyEnv":"DEEPSEEK_REF"}}}`)
	edit, err := newRetryEditorState(describe, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	edit.credentialRef = "DEEPSEEK_REF"
	writer := &fakeWriter{namespace: apiclient.SettingsNamespaceView{NS: "llm", Revision: 5}}
	creds := &fakeCredentials{err: errors.New("凭据库只读")}
	plugin := &Plugin{}
	official := plugin.ensureOfficial()
	official.describe, official.writer, official.modelsEdit = describe, writer, edit
	official.credentialSet = creds.set

	state := plugin.applyModelsEditor("sk-reject-1")
	if state.Success() {
		t.Fatal("凭据写失败不能报成功")
	}
	if state.Message != "凭据库只读" {
		t.Fatalf("失败原文 = %q", state.Message)
	}
	if state.NS != credentialNS {
		t.Fatalf("失败归属 = %q，应为凭据域而不是设置命名空间", state.NS)
	}
	if official.modelsEdit == nil {
		t.Fatal("失败必须保留草稿（卡片不关）")
	}
}

// TestModelsApplyRejectsIllegalKeyBeforeAnyRequest 钉官方 apiKeyFailure 是权威门：
// 非法密钥一次请求都不发（设置写也不发）。
func TestModelsApplyRejectsIllegalKeyBeforeAnyRequest(t *testing.T) {
	describe := modelsCredentialFixture(`{"providers":{"deepseek":{"apiKeyEnv":"DEEPSEEK_REF","retryPolicy":{"mode":"always"}}}}`)
	edit, err := newRetryEditorState(describe, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	edit.credentialRef = "DEEPSEEK_REF"
	edit.ensurePolicy()
	edit.policy["mode"] = "normal"
	writer := &fakeWriter{namespace: apiclient.SettingsNamespaceView{NS: "llm", Revision: 5}}
	creds := &fakeCredentials{}
	plugin := &Plugin{}
	official := plugin.ensureOfficial()
	official.describe, official.writer, official.modelsEdit = describe, writer, edit
	official.credentialSet = creds.set

	state := plugin.applyModelsEditor("MY_KEY=sk-1")
	if state.Kind != WriteInvalid || state.Message != zhModelsKeyIllegalCharacters {
		t.Fatalf("非法密钥的状态 = %+v", state)
	}
	if len(writer.mutateCalls) != 0 || len(creds.calls) != 0 {
		t.Fatalf("非法输入不许发出任何请求：mutate=%d set=%d", len(writer.mutateCalls), len(creds.calls))
	}
	if official.modelsEdit == nil {
		t.Fatal("非法输入必须保留草稿")
	}
}

// TestToggleModelsEditorPinsCredentialRef 钉官方 `keyRef` 在开卡时定下来
// （ProviderEditor.tsx:224）：卡片打开后 describe 变了也不改这张卡要写的引用。
func TestToggleModelsEditorPinsCredentialRef(t *testing.T) {
	describe := modelsCredentialFixture(`{"providers":{"deepseek":{"apiKeyEnv":"DEEPSEEK_REF"}}}`)
	plugin := &Plugin{}
	official := plugin.ensureOfficial()
	official.describe = describe

	plugin.toggleModelsEditor("deepseek", apiclient.SettingsNSLLM, []string{"providers", "deepseek"})
	if official.modelsEdit == nil {
		t.Fatal("点开卡片应建草稿")
	}
	if official.modelsEdit.credentialRef != "DEEPSEEK_REF" {
		t.Fatalf("credentialRef = %q", official.modelsEdit.credentialRef)
	}
	// 换一个 profile 描述也还是这张卡的引用（引用是开卡那一刻钉住的）。
	official.describe = modelsCredentialFixture(`{"providers":{"deepseek":{"apiKeyEnv":"CHANGED_REF"}}}`)
	if got := official.modelsEdit.credentialRef; got != "DEEPSEEK_REF" {
		t.Fatalf("开卡后 credentialRef 被改成 %q", got)
	}

	// 关闭卡片要丢弃草稿（官方 onClose(false)）。
	plugin.toggleModelsEditor("deepseek", apiclient.SettingsNSLLM, []string{"providers", "deepseek"})
	if official.modelsEdit != nil {
		t.Fatal("再点一次应关卡")
	}
	official.mu.Lock()
	pending := official.modelsKeyClearPending
	official.mu.Unlock()
	if !pending {
		t.Fatal("关卡必须请求清空密钥输入框（否则下一个 provider 会继承上一个的密钥）")
	}
}

// orderedWriter 是那个既记 mutate 又记**顺序**的写入替身（官方"先设置后凭据"要它）。
type orderedWriter struct {
	namespace apiclient.SettingsNamespaceView
	log       *[]string
}

func (w *orderedWriter) SettingsFormSet(_ context.Context, _ string, _ []string, _ any, _ *int) (apiclient.SettingsSectionView, error) {
	return apiclient.SettingsSectionView{}, nil
}

func (w *orderedWriter) SettingsFormReset(_ context.Context, _ string, _ []string, _ *int) (apiclient.SettingsSectionView, error) {
	return apiclient.SettingsSectionView{}, nil
}

func (w *orderedWriter) SettingsMutate(_ context.Context, ns string, _ []apiclient.SettingsOp, _ *int) (apiclient.SettingsNamespaceView, error) {
	if w.log != nil {
		*w.log = append(*w.log, "mutate")
	}
	view := w.namespace
	view.NS = ns
	return view, nil
}
