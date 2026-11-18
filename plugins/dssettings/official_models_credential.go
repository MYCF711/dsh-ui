// official_models_credential.go —— Models 页的 API 密钥单字段与凭据状态（账本 #41 差异 (a)）。
//
// 官方把密钥**从设置文档里分出来**，这是本文件存在的全部理由：
//
//	OFF:dsh-client-ui-settings-models/src/client/ApiKeyInput.tsx:1-68
//	    写-only 输入（`type=password`、`autoComplete="new-password"`），
//	    已存密钥只以服务端掩码当 placeholder（`:41`），复制/剪切被禁（`:42-43`）。
//	OFF:.../operations.ts:78-85
//	    `credentials.set` / `credentials.unset` —— 不是 `settings.mutate`。
//	OFF:.../store.ts:276-291
//	    读侧只有 `credentials.describe`：**没有值，只有"设没设"**；失败只降级徽标，
//	    不把整页读挂（注释原文 "a failure degrades the badge instead of failing the load"）。
//	OFF:.../store.ts:274-275
//	    问哪些 ref：命名了的取那一个，没命名的取 `deriveKeyRef(provider)` + `nativeAuth.env`。
//	OFF:.../ProviderEditor.tsx:167-181
//	    引用名推导 `refFor`：profile 里 `apiKeyEnv` 优先，否则推导。
//	OFF:.../store.ts:125-127
//	    推导算法 `deriveKeyRef`：大写 + 非字母数字串折成 `_` + `_API_KEY`。
//	OFF:.../apiKey.ts:51-58
//	    字段级校验：空 = 不是失败；纯空白 = keyBlank；`NAME=value` 行 / 成对引号 /
//	    非可打印 ASCII 或含空格 = keyIllegalCharacters。
//
// # 与本仓数据面的关系
//
// 宿主侧 `credentials.describe` / `credentials.set` 已是真实现
// （`../DSH-GO/internal/transport/handlers_settings_official.go:491,576`），
// 客户端在 `internal/apiclient/credentials.go`，壳经 `pluginapi.Actions.CredentialStatus`
// / `CredentialSet` 交到插件。本文件只做**投影与判据**，不发请求。
//
// # 两条"不知道"不许写成"没有"
//
//   - `describe` 只回**问到过的** ref：map 里没有某个 ref = 不知道，画中性说明；
//   - 官方 `missingRefs` 要求"每个 checked ref 都明确 false"，本文件照抄这条：
//     只要有一条没答，就不说"缺失"。
package dssettings

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// ── 官方文案（逐字，锚点是 rc.2 随包 src 的 locales.ts）──────────────────

const (
	// zhModelsCredentialConfigured / Missing 是行头那枚凭据点的两种说明（locales.ts:212-213）。
	zhModelsCredentialConfigured = "API 密钥已配置"
	zhModelsCredentialMissing    = "API 密钥缺失"
	// zhModelsKeyInput 是密钥字段的 label（locales.ts:219）。
	zhModelsKeyInput = "API 密钥"
	// zhModelsKeyPlaceholder 是没有命名引用时的占位（locales.ts:222）。
	zhModelsKeyPlaceholder = "输入 API 密钥"
	// zhModelsKeyPlaceholderFallback 是命名引用存在时的占位（locales.ts:223，`{hint}` 实参）。
	zhModelsKeyPlaceholderFallback = "留空使用环境认证（%s）"
	// zhModelsKeySaved 是"已有本地存储密钥"的说明（locales.ts:224）。
	zhModelsKeySaved = "使用已保存的 API key。"
	// zhModelsKeyFallbackMissing / Detected / Unchecked 是环境回退三态（locales.ts:229/230/232）。
	zhModelsKeyFallbackMissing   = "未检测到环境变量。请填写 API 密钥，或设置环境变量 %s。"
	zhModelsKeyFallbackDetected  = "已检测到环境变量（%s）"
	zhModelsKeyFallbackUnchecked = "环境凭据状态尚未确认"
	// zhModelsKeyBlank / IllegalCharacters 是字段级校验文案（locales.ts:304,306）。
	zhModelsKeyBlank             = "请输入 API 密钥；留空则保持已存储的密钥。"
	zhModelsKeyIllegalCharacters = "该 API 密钥格式错误，请检查。"
	// zhModelsKeyReveal / Hide 是显隐按钮的 aria 文案（locales.ts:220-221）。
	zhModelsKeyReveal = "显示密钥"
	zhModelsKeyHide   = "隐藏密钥"
	// zhModelsKeyHelp 是 label 后面那枚 `?` 的悬浮帮助正文（locales.ts:233，`{env}` 实参）。
	zhModelsKeyHelp = "优先使用已保存的凭据。此处显示的环境后备按以下顺序排列：%s。来源依次为启动环境、$cwd/.env、$DSH_HOME/.env。"
)

const (
	// modelsProviderAccount 是官方唯一不参与凭据 join 的 route（store.ts:263,275：
	// `entry.provider === 'deepseek-account'` 时 `apiKeyEnv` 与 refs 都跳过）。
	modelsProviderAccount = "deepseek-account"
	// credentialsDescribeMaxRefs 是一次 describe 的引用上限（store.ts:278 注释原文
	// 「credentials.describe accepts at most 64 references per request」；宿主同一道
	// grammar guard）。
	credentialsDescribeMaxRefs = 64
	// credentialSourceCatalog 是"本地凭据存储"那一层的 source 名（宿主
	// `OfficialCredentialInfo.Source`；官方对应 `source:'file'`）。
	credentialSourceCatalog = "catalog"
)

// modelsKeyRowPrefix 拼密钥行的 id（`key-<route>`）。
const modelsKeyRowPrefix = "key-"

// credentialNS 是凭据域的命名空间名，只用于写失败时的展示（官方把它当独立
// Remote 命名空间 `remote.credentials`，不在 settings 文档里）。写成 `credentials`
// 而不是某个 settings ns：错了会让用户以为要去看设置文档。
const credentialNS = "credentials"

// ── 引用名推导 ─────────────────────────────────────────────────────────

// credentialRefNonAlnumRun 是官方 `replace(/[^A-Z0-9]+/g,'_')`：**连续**的非字母
// 数字**折成一个**下划线（不是逐个替换）。
var credentialRefNonAlnumRun = regexp.MustCompile(`[^A-Z0-9]+`)

// deriveCredentialKeyRef 是官方 `deriveKeyRef`（store.ts:125-127）。
//
// 官方原文：
//
//	`${provider.toUpperCase().replace(/[^A-Z0-9]+/g, '_')}_API_KEY`
func deriveCredentialKeyRef(provider string) string {
	return credentialRefNonAlnumRun.ReplaceAllString(strings.ToUpper(provider), "_") + "_API_KEY"
}

// profileAPIKeyEnv 读某个 profile 里**命名**的凭据引用（官方 `apiKeyEnvOf`，
// store.ts:172-182）。
//
// 读的是 namespace 的 **value 层**（官方 `schema.getPath(namespace.value, path)`）：
// 那一层是"现在真正生效的 profile"，base 只是它下面的继承。缺失、非字符串、空串
// 一律算"没有命名"——官方判据就是 `typeof ref === 'string' && ref.length > 0`。
func profileAPIKeyEnv(describe *apiclient.SettingsDescribeResult, ns string, path []string) (string, bool) {
	if describe == nil || ns == "" {
		return "", false
	}
	namespace, ok := describe.Namespace(ns)
	if !ok {
		return "", false
	}
	full := append(append([]string(nil), path...), "apiKeyEnv")
	value, found := valueAtPath(namespace.Value, full)
	if !found {
		return "", false
	}
	ref, ok := value.(string)
	if !ok || ref == "" {
		return "", false
	}
	return ref, true
}

// credentialRefFor 是官方 `refFor`（ProviderEditor.tsx:167-181）：命名优先，否则推导。
func credentialRefFor(describe *apiclient.SettingsDescribeResult, provider, ns string, path []string) string {
	if ref, ok := profileAPIKeyEnv(describe, ns, path); ok {
		return ref
	}
	return deriveCredentialKeyRef(provider)
}

// ── 状态 join ─────────────────────────────────────────────────────────

// modelsCredentialRefs 是官方 store.ts:274-275 的引用集合：每行"命名的那一个"或
// "推导的那个 + provider 自己声明的原生鉴权变量"，去重后一次性问。
func modelsCredentialRefs(rows []ModelsProvider) []string {
	seen := make(map[string]bool, len(rows)*2)
	out := make([]string, 0, len(rows)*2)
	add := func(ref string) {
		if ref == "" || seen[ref] {
			return
		}
		seen[ref] = true
		out = append(out, ref)
	}
	for _, row := range rows {
		if row.Provider == modelsProviderAccount {
			continue
		}
		if row.APIKeyEnv != "" {
			add(row.APIKeyEnv)
			continue
		}
		add(deriveCredentialKeyRef(row.Provider))
		if row.NativeAuth != nil {
			for _, ref := range row.NativeAuth.Env {
				add(ref)
			}
		}
	}
	return out
}

// applyModelsCredentials 把 `credentials.describe` 的结果落到目录行上。
//
// 两条官方语义必须保住：
//  1. **命名优先**：行里 `apiKeyEnv` 有名 → 结论落在 `Credential`（官方 `row.credential`）；
//     没有名 → 结论落在 `DerivedCredential`（官方 `row.derivedCredential`）。两者都可能是
//     nil，**nil 不是"未配置"**，是"没答"。
//  2. **失败只降级**：`statuses` 为 nil（壳没接）或没答到某个 ref 时，行上什么都留空，
//     由投影层画中性说明——不把整页读挂（store.ts:281-283 的注释）。
func applyModelsCredentials(rows []ModelsProvider, statuses func([]string) map[string]apiclient.CredentialInfo) {
	got := map[string]apiclient.CredentialInfo{}
	if statuses != nil {
		refs := modelsCredentialRefs(rows)
		for start := 0; start < len(refs); start += credentialsDescribeMaxRefs {
			end := start + credentialsDescribeMaxRefs
			if end > len(refs) {
				end = len(refs)
			}
			for ref, info := range statuses(refs[start:end]) {
				got[ref] = info
			}
		}
	}
	for index := range rows {
		row := &rows[index]
		if row.Provider == modelsProviderAccount {
			continue
		}
		if row.APIKeyEnv != "" {
			if info, found := got[row.APIKeyEnv]; found {
				copied := info
				row.Credential = &copied
			}
			continue
		}
		if info, found := got[deriveCredentialKeyRef(row.Provider)]; found {
			copied := info
			row.DerivedCredential = &copied
		}
	}
}

// modelsCredentialBadge 是行头上那枚 8px 圆点的判据（ModelsSection.tsx:459-500）。
//
// 官方只有两态、且都挂在**命名引用**上：
//
//	credentialConfigured = row.credential?.configured === true
//	credentialMissing    = !configured && row.apiKeyEnv !== undefined && row.credential?.configured === false
//
// 所以：没命名引用不画点；describe 没答不画点（不知道 ≠ 缺失）。
func modelsCredentialBadge(provider ModelsProvider) (label string, missing, ok bool) {
	if provider.APIKeyEnv == "" || provider.Credential == nil {
		return "", false, false
	}
	if provider.Credential.Configured {
		return zhModelsCredentialConfigured, false, true
	}
	return zhModelsCredentialMissing, true, true
}

// ── 密钥字段的 placeholder / 说明 ──────────────────────────────────────

// modelsKeyHintTone 是说明行的语气，对应官方三个 CSS 类：
// `.notice`（warn）/ `.savedNotice`（success）/ `.advancedHint`（tertiary）。
type modelsKeyHintTone string

const (
	modelsKeyHintNone    modelsKeyHintTone = ""
	modelsKeyHintWarn    modelsKeyHintTone = "warn"
	modelsKeyHintSuccess modelsKeyHintTone = "success"
	modelsKeyHintMuted   modelsKeyHintTone = "muted"
)

// modelsKeyField 是一个 provider 的密钥字段该显示什么（不含用户正在打的草稿）。
type modelsKeyField struct {
	// Placeholder 是空输入框里的灰字（官方 `keyPlaceholder` 或 `keyPlaceholderFallback`，
	// 已有本地存储密钥时是服务端掩码 `saved.masked`，ApiKeyInput.tsx:41）。
	Placeholder string
	// Hint 是输入框下面那行说明（官方 `credentialHint`，ProviderEditor.tsx:446-459）。
	Hint string
	// Tone 是说明行的语气。
	Tone modelsKeyHintTone
	// Known 表示 describe 真的答过这个引用。
	Known bool
	// Saved 表示本地凭据存储里有值（官方 `savedCredential`：`source === 'file'`）。
	Saved bool
	// Writable 表示 `credentials.set/unset` 能改它（describe 的 `writable`）。
	Writable bool
	// Help 是 label 那枚 `?` 的帮助正文（官方 `keyHelp` + `visibleRefs.join(' / ')`）；
	// 空 = 官方 `keyHelp === undefined`（没有任何可见引用）→ 不画帮助钮。
	Help string
}

// modelsKeyState 把官方那串三元判据收成一个纯函数。
//
// 判据来源（逐条对应 ProviderEditor.tsx）：
//
//	:437-441  namedRef = 生效 profile 的 apiKeyEnv；authRefs = [namedRef] 或 nativeAuth.env
//	:446-459  credentialHint 的四档
//	:466-468  keyPlaceholder = keyHint 未定义 ? keyPlaceholder : keyPlaceholderFallback
//	:496      credentialHint 的 CSS 类（missingRefs→notice / 未答→advancedHint / 有检测→savedNotice）
//	:473-483  savedCredential = detected.source === 'file' && 该 ref 可见
func modelsKeyState(provider ModelsProvider) modelsKeyField {
	field := modelsKeyField{Placeholder: zhModelsKeyPlaceholder}
	if provider.APIKeyEnv == "" {
		// 官方 keyHint === undefined：没有命名引用就谈不上"环境回退"，placeholder 用
		// 无实参那一句，说明行整块不画。
		return field
	}
	named := provider.APIKeyEnv
	field.Placeholder = fmt.Sprintf(zhModelsKeyPlaceholderFallback, named)
	// 官方 visibleRefs：命名引用或 nativeAuth.env，去掉名字里带 oauth 的
	// （ProviderEditor.tsx:432-435），再 `{env}` 用 ` / ` 连起来（`:460-462`）。
	var visible []string
	for _, ref := range append([]string{named}, nativeAuthEnv(provider)...) {
		if ref == "" || strings.Contains(strings.ToLower(ref), "oauth") {
			continue
		}
		visible = append(visible, ref)
	}
	if len(visible) > 0 {
		field.Help = fmt.Sprintf(zhModelsKeyHelp, strings.Join(visible, " / "))
	}
	info := provider.Credential
	if info == nil {
		// describe 没答：官方 `keyFallbackUnchecked`（:446-459 的 fallback 分支）。
		field.Hint = zhModelsKeyFallbackUnchecked
		field.Tone = modelsKeyHintMuted
		return field
	}
	field.Known = true
	field.Writable = info.Writable
	if info.Configured && info.Source == credentialSourceCatalog {
		// 本地存储层有值：官方用掩码当 placeholder、用成功色说明"已保存"。
		field.Saved = true
		field.Hint = zhModelsKeySaved
		field.Tone = modelsKeyHintSuccess
		if info.Masked != "" {
			field.Placeholder = info.Masked
		}
		return field
	}
	if info.Configured {
		// env / dotenv 层供得上：官方 `keyFallbackDetected`（成功色，:492）。
		field.Hint = fmt.Sprintf(zhModelsKeyFallbackDetected, named)
		field.Tone = modelsKeyHintSuccess
		return field
	}
	// 明确没有：官方 `keyFallbackMissing`（warn，missingRefs 分支）。
	field.Hint = fmt.Sprintf(zhModelsKeyFallbackMissing, named)
	field.Tone = modelsKeyHintWarn
	return field
}

// nativeAuthEnv 是 provider 自己声明的原生鉴权变量（官方 `nativeAuth?.env ?? []`）。
func nativeAuthEnv(provider ModelsProvider) []string {
	if provider.NativeAuth == nil {
		return nil
	}
	return provider.NativeAuth.Env
}

// modelsKeyHidden 是官方 `keylessNativeAuth`：provider 的鉴权**根本不经过密钥**
// （`nativeAuth.env` 为空且走 oauth），且 profile 没命名引用 → 整块不画。
// 依据 ProviderEditor.tsx:420-424,461-462。
func modelsKeyHidden(provider ModelsProvider) bool {
	if provider.APIKeyEnv != "" {
		return false
	}
	if provider.NativeAuth == nil {
		return false
	}
	return len(provider.NativeAuth.Env) == 0 && provider.NativeAuth.OAuth
}

// modelsKeyRow 是 ProviderEditor 里那个 API 密钥字段的一行投影。
//
// 它只是**投影**：真正的输入框是 `officialPanel.modelsKeyField`（Viva 的 ui.Field
// 必须挂在窗口线程上，不能每帧重造）。这里给出上层的静态部分——label、placeholder、
// 说明与语气、可写性。
func modelsKeyRow(provider ModelsProvider, writable bool) (PanelRow, bool) {
	if provider.Provider == modelsProviderAccount || modelsKeyHidden(provider) {
		return PanelRow{}, false
	}
	state := modelsKeyState(provider)
	row := PanelRow{
		ID:          modelsKeyRowPrefix + provider.Provider,
		Title:       zhModelsKeyInput,
		Kind:        ItemSecret,
		NS:          provider.SettingsNS,
		Placeholder: state.Placeholder,
		HelpText:    state.Help,
		Desc:        state.Hint,
		Tone:        panelToneOf(state.Tone),
		Known:       state.Known,
		Disabled:    !writable,
		// 密钥**永远**走凭据域，不走 settings.mutate：这里刻意不给 Path，
		// 免得任何通用写路径把它当成设置文档字段。
		Value: "",
	}
	if !writable {
		row.Reason = "只读：设置文档不可写"
	}
	return row, true
}

// panelToneOf 把密钥说明的语气映射成 PanelRow 的通用语气（画法在 draw 层）。
func panelToneOf(tone modelsKeyHintTone) PanelTone {
	switch tone {
	case modelsKeyHintWarn:
		return PanelToneWarn
	case modelsKeyHintSuccess:
		return PanelToneSuccess
	case modelsKeyHintMuted:
		return PanelToneMuted
	default:
		return PanelToneNone
	}
}

// ── 字段级校验 ─────────────────────────────────────────────────────────

var (
	// legalAPIKey 是官方 `LEGAL_API_KEY`（apiKey.ts:12）：可打印 ASCII，**空格除外**。
	legalAPIKey = regexp.MustCompile(`^[\x21-\x7E]+$`)
	// envLine 是官方 `ENV_LINE`（apiKey.ts:23）：`NAME=value` 形状的粘贴行。
	// 名字必须全大写（`sk-` 这类在连字符处断开），`=` 后面不能紧跟另一个 `=`。
	envLine = regexp.MustCompile(`^[A-Z][A-Z0-9_]*=[^=]`)
)

// isQuotedAPIKey 是官方 `isQuoted`（apiKey.ts:35-39）：成对引号包裹。
func isQuotedAPIKey(value string) bool {
	if value == "" {
		return false
	}
	first := value[0]
	if first != '"' && first != '\'' && first != '`' {
		return false
	}
	return len(value) > 1 && value[len(value)-1] == first
}

// modelsAPIKeyFailure 是官方 `apiKeyFailure`（apiKey.ts:51-58）的 Go 等价物。
//
// 返回空串 = 可提交（官方 `undefined`）。三条判据逐条照抄：
//
//	draft.length === 0            → undefined（空框不是失败：每张卡都空着开，
//	                                "空"= 保留已存的那一把）
//	trim() 后为空                  → keyBlank（纯空白不能静默丢弃）
//	ENV_LINE / 成对引号 / 非法字符   → keyIllegalCharacters
func modelsAPIKeyFailure(draft string) string {
	if draft == "" {
		return ""
	}
	value := strings.TrimSpace(draft)
	if value == "" {
		return zhModelsKeyBlank
	}
	if envLine.MatchString(value) || isQuotedAPIKey(value) {
		return zhModelsKeyIllegalCharacters
	}
	if !legalAPIKey.MatchString(value) {
		return zhModelsKeyIllegalCharacters
	}
	return ""
}

// modelsKeyDraftValue 是官方 `keyValue = keyText.trim()`（ProviderEditor.tsx:283）：
// 真正写进凭据域的是去掉首尾空白之后的值；空 = "这次没给密钥"。
func modelsKeyDraftValue(draft string) string { return strings.TrimSpace(draft) }
