// cot_translation.go —— 插件侧看到的「思考译文」类型（账本 #63）。
//
// 只放**投影**：插件拿到的是壳已经处理过的终态，不是 HTTP 响应。理由与
// feedback 那一套相同（见 references.go 的 FeedbackResult）：插件在 draw 里跑，
// 不能等 HTTP；而"这次翻译成没成"必须在界面上当场说一次。
package pluginapi

// CotTranslationResult 是一次翻译请求的终态，按 jobID 由插件取走（取走即清空）。
//
// OK=false 时 Code 是传输层/宿主业务码摘要，Message 是给人看的一句原因：
//
//	未接线（宿主没有 cotTranslation.translate）
//	400 —— text 超 maxTextChars / targetLanguage 空 / provider 不在官方取值
//	501 —— provider 是 google/bing（本仓不代理匿名翻译端点）
//	网关失败 —— 连不上本机模型网关 / 模型输出空文本
//
// 无论哪种失败，壳都**不会**把原文塞进 Text 冒充译文：界面照官方画
// 「翻译暂不可用，已显示原文」。
type CotTranslationResult struct {
	// Text 是译文（官方 translate 的 result 就是裸字符串）。
	// OK=false 时它一定是空串。
	Text string
	// OK 表示这一次真的拿到了译文。
	OK bool
	// Code 是失败时的机器码摘要（成功时为空）。
	Code string
	// Message 是失败时的原因（成功时为空）。
	Message string
}
