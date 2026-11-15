// cot_translation.go —— 官方 `cotTranslation.limits` / `cotTranslation.translate` 的客户端
// （包 `dsh-experimental-client-ui-cot-translation`，GUI 侧账本 #63）。
//
// 契约唯一来源是**官方 rc.2 源码**：
//
//	OFF:dsh-experimental-client-ui-cot-translation/src/index.ts:46-53   @Remote limits(signal)
//	OFF:dsh-experimental-client-ui-cot-translation/src/index.ts:55-…    @Remote translate(request, signal)
//	OFF:.../lib/typert.remote-client.js:37-51                          'cotTranslation/limits' 的参数/结果
//	OFF:.../lib/typert.remote-client.js:53-81                          'cotTranslation/translate' 的参数/结果
//	OFF:.../src/types.ts                                                CotTranslationSnapshot
//	OFF:dsh-experimental-translator/src/types.ts                        TranslationRequest / TranslationSpec
//
// 两条形状上的硬事实：
//
//	limits    **零参数**，结果 {maxTextChars, preferences{provider,targetLanguage}, availableProviders[]}
//	translate 参数 {text, targetLanguage, sourceLanguage?, provider?, sessionId?}，
//	          结果**裸字符串**（官方 zod 是 `z.string()`，不是对象）——所以这里
//	          `Call` 的 out 是 `*string`，不是结构体。写成结构体会静默解成零值。
//
// 宿主侧实现（不冒充官方，字段口径见文件内注释）：
//
//	../DSH-GO/internal/transport/handlers_cot_translation_limits_official.go:71-83
//	../DSH-GO/internal/transport/handlers_media_market_official.go:572-700
//
// 宿主把 translate 映射成两种真实行为，客户端必须能分辨（它决定界面画不画"不可用"）：
//
//	deepseek-official / deepseek-account / 省略 → 走本机模型网关，返回真译文；
//	google / bing                             → 501 not_implemented（本仓不代理匿名翻译端点）。
//
// 后一条对应官方界面里的 `providerUnavailable`。`limits.availableProviders` 由宿主
// 按"真的能翻"发布，所以界面**只需要**照它渲染菜单/挑 provider，不要自己按四个
// 官方枚举去补——那会把 501 的路径列成"可用"。
package apiclient

import "context"

// 官方 provider 的四个取值（OFF:src/preferences.ts:10-12 的 PROVIDER_PREFERENCES，
// 与宿主 cotProvider* 常量逐字一致）。
const (
	CotTranslationProviderGoogle   = "google"
	CotTranslationProviderBing     = "bing"
	CotTranslationProviderAccount  = "deepseek-account"
	CotTranslationProviderOfficial = "deepseek-official"
)

// CotTranslationPreferences 是已生效的翻译偏好（官方 CotTranslationPreferences）。
//
// TargetLanguage 取 `auto` 或 BCP 47；`auto` 表示"跟随界面语言"，由调用方解析成
// 具体 tag 再发（官方 TranslationBody 就是这么做的：`targetLanguage === 'auto' ?
// locale.active : targetLanguage`）。
type CotTranslationPreferences struct {
	Provider       string `json:"provider"`
	TargetLanguage string `json:"targetLanguage"`
}

// CotTranslationLimits 是 `cotTranslation.limits` 的结果（字段逐字对齐官方 zod）。
//
// MaxTextChars 的口径是 **UTF-16 码元**（官方 translator 的判据，宿主
// `cotTextUTF16Len` 同一口径）：增补平面字符算 2。按 `len([]rune)` 切段会把
// emoji 算成 1，照上限发满时会被宿主多拒一段。
type CotTranslationLimits struct {
	MaxTextChars       int                       `json:"maxTextChars"`
	Preferences        CotTranslationPreferences `json:"preferences"`
	AvailableProviders []string                  `json:"availableProviders"`
}

// CotTranslationRequest 是一次翻译请求（官方 TranslationRequest + sessionId）。
//
// SourceLanguage 是可选的"原文语言提示"；官方只在偏好里显式给了语言时才带，
// 其余情况让 provider 自己判。
type CotTranslationRequest struct {
	Text           string `json:"text"`
	TargetLanguage string `json:"targetLanguage"`
	SourceLanguage string `json:"sourceLanguage,omitempty"`
	Provider       string `json:"provider,omitempty"`
	SessionID      string `json:"sessionId,omitempty"`
}

// CotTranslationLimits 读已生效的偏好与单次请求上限。
//
// 它**不发送任何文本**（官方 doc：'Read accepted translation preferences and the
// current request limit without sending text'），所以界面一展开就可以问，不必等用户
// 点"查看译文"。零参数：官方 `parameters: []`，宿主严格无参校验，发空对象。
func (c *Client) CotTranslationLimits(ctx context.Context) (CotTranslationLimits, error) {
	var out CotTranslationLimits
	if err := c.Call(ctx, "cotTranslation.limits", struct{}{}, &out); err != nil {
		return CotTranslationLimits{}, err
	}
	return out, nil
}

// CotTranslationTranslate 翻译一段文本，返回**裸字符串**译文。
//
// 失败时不要拿原文顶替译文：宿主对"模型输出空文本"也是报错而不是回原文
// （handlers_media_market_official.go:689-694），界面照官方画"翻译暂不可用，
// 已显示原文"。
func (c *Client) CotTranslationTranslate(ctx context.Context, req CotTranslationRequest) (string, error) {
	var out string
	if err := c.Call(ctx, "cotTranslation.translate", req, &out); err != nil {
		return "", err
	}
	return out, nil
}
