// cot_translation_wiring.go —— #63 思考译文（`dsh-experimental-client-ui-cot-translation`）
// 的两条数据面接线。
//
// # 数据从哪来（官方与宿主各一半）
//
//	偏好 + 上限      ← 宿主 `cotTranslation.limits`（零参数，不发送文本）
//	译文            ← 宿主 `cotTranslation.translate`（裸字符串；本仓走本机模型网关）
//
// 两者都不在 `settings.describe` 里，所以**各归各的缓存**：limits 拉失败不该让
// 译文状态一起空掉（反过来也一样，译文失败只影响那一段）。
//
// # 缓存策略（与 WebProviderList / AgentPresetRoster 同一套）
//
//   - `CotTranslationLimits` **同步返回缓存**：插件在 draw 里调，等 HTTP 会卡首帧；
//   - 首次问触发一次后台拉取，按 pending 位防重入；
//   - 拉失败**保留上一次**，原因写状态栏（别把已经能用的 provider/上限清空）；
//   - 拉成功后 `Invalidate`，插件下一帧读到新值。
//
// # 翻译为什么是"发起 + 取结果"而不是阻塞
//
// 一次翻译要打模型网关，宿主侧超时 90s。插件在绘制线程上，**不能**等它。
// 所以壳只发起、把终态按 jobID 留在 `cotResults`，插件下一帧用
// `CotTranslationResult(jobID)` 取走（取走即清空，同一结果不会重放）。
//
// sessionID 只为让宿主把它记进那条会活的会话（官方 translate 也带 sessionId）；
// 本仓宿主对不存在的会话不报错，所以老壳/换会话都不会因此炸。
package main

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// cotLimitsTimeout 是一次 limits 读的上限（与设置面同一档，它不联网）。
const cotLimitsTimeout = 8 * time.Second

// cotTranslateTimeout 必须**大于**宿主侧的那一次（`cotTranslationTimeout = 90s`）：
// 客户端先超时会把"宿主还在跑模型"报成网络失败，用户看到的原因就是错的。
const cotTranslateTimeout = 150 * time.Second

// refreshCotTranslationLimits 重新拉一次偏好与上限并发布到缓存。
func (m *model) refreshCotTranslationLimits() {
	if m.cli == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), cotLimitsTimeout)
	defer cancel()
	limits, err := m.cli.CotTranslationLimits(ctx)

	m.mu.Lock()
	m.cotLimitsPending = false
	if err != nil {
		// 保留上一次（可能是零值 = 还没成功过），只报状态栏。
		m.cotLimitsErr = err
		m.mu.Unlock()
		m.setStatus("读思考翻译偏好失败: " + err.Error())
		return
	}
	m.cotLimits = limits
	m.cotLimitsLoaded = true
	m.cotLimitsErr = nil
	m.mu.Unlock()
	m.inv.Invalidate()
}

// cotTranslationLimits 是给插件用的同步快照（首次问触发后台拉取）。
func (m *model) cotTranslationLimits() (apiclient.CotTranslationLimits, bool) {
	m.mu.Lock()
	limits, loaded, pending := m.cotLimits, m.cotLimitsLoaded, m.cotLimitsPending
	m.mu.Unlock()
	if !loaded && !pending && m.cli != nil {
		m.mu.Lock()
		m.cotLimitsPending = true
		m.mu.Unlock()
		go m.refreshCotTranslationLimits()
	}
	return limits, loaded
}

// cotTranslationTranslate 发起一次翻译，返回 jobID（空串 = 没发起）。
//
// 参数明显不合法（空文本、无 provider）时就地返回空串：那种请求打出去也只会
// 换回一个 400，还得让界面解释一次。
func (m *model) cotTranslationTranslate(sessionID, text, targetLanguage, provider string) string {
	if m.cli == nil || text == "" {
		return ""
	}
	m.mu.Lock()
	m.cotJobSeq++
	jobID := "cot-" + strconv.FormatInt(m.cotJobSeq, 10)
	m.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), cotTranslateTimeout)
		defer cancel()
		translated, err := m.cli.CotTranslationTranslate(ctx, apiclient.CotTranslationRequest{
			Text:           text,
			TargetLanguage: targetLanguage,
			Provider:       provider,
			SessionID:      sessionID,
		})
		result := pluginapi.CotTranslationResult{}
		if err != nil {
			result.Code, result.Message = cotTranslateFailure(err)
		} else {
			result.Text = translated
			result.OK = true
		}
		m.mu.Lock()
		if m.cotResults == nil {
			m.cotResults = map[string]pluginapi.CotTranslationResult{}
		}
		m.cotResults[jobID] = result
		m.mu.Unlock()
		m.inv.Invalidate()
	}()
	return jobID
}

// cotTranslateFailure 把四字段错误压成"码 + 一句原因"。
//
// 界面只显示一句（官方只有一个 failed 文案位），所以原因取宿主的 message——
// 它已经点名缺什么（provider 不可用 / 模型空输出 / 网关连不上）。
func cotTranslateFailure(err error) (code, message string) {
	// 用 errors.As 而不是类型断言：Client.Call 会把连接失败包成
	// `fmt.Errorf("连不上 %s：%w", …)`，断言会漏掉那一层。
	var apiErr *apiclient.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code, apiErr.Message
	}
	return "transport", err.Error()
}

// cotTranslationTakeResult 取走某个 jobID 的终态（取走即清空，避免同一结果重放）。
func (m *model) cotTranslationTakeResult(jobID string) (pluginapi.CotTranslationResult, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result, ok := m.cotResults[jobID]
	if ok {
		delete(m.cotResults, jobID)
	}
	return result, ok
}
