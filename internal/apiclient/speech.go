package apiclient

// speech.go —— 官方 `speech.*` 六方法（`dsh-experimental-api-speech-to-text`）。
//
// # 官方契约原件
//
// 方法名与入参形状取自官方包的 Remote 定义
// （`OFF:dsh-experimental-client-ui-voice-input/lib/client.js:542-560`
// 的 `package: "@deepseek-ai/dsh-experimental-api-speech-to-text"` 段）：
//
//	speech/catalog            () → provider[]
//	speech/prepare            (providerId, {downloadSource}?) → void
//	speech/cancelPreparation  (providerId) → void
//	speech/configure          ({providerId, language}) → config
//	speech/transcribe         ({providerId, language, audioBase64}) → {text, audioSeconds, inferenceSeconds}
//	speech/follow             () → status 流
//
// # 宿主的 wire 形状（不是我们猜的）
//
// `HOST:internal/transport/handlers_speech_official.go:124-205` 逐条写的：
// `catalog` 直接返回**数组**；`prepare`/`cancelPreparation` 返回 `{ok:true}`；
// `configure` 从 `patch` 子对象取（没有 `patch` 时从顶层取）；`transcribe` 从
// `request` 子对象取（没有 `request` 时从顶层取）；`follow` 是 NDJSON。
// 所以这里的入参按**宿主认的**形状发，不按 JS 端看起来的形状发。
//
// # 为什么 follow 只给一条流式入口
//
// 宿主 `Follow` 有真实状态源（`HOST:internal/speech/engine.go:394-420`），
// 而且第一条永远是基线帧（它刻意不造"假进度"）。界面靠它知道
// `idle / preparing / prepared / transcribing / error`，见 `SpeechFollow`。

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SpeechProvider 是 `speech/catalog` 的一行（宿主 `OfficialSpeechProvider`）。
//
// 注意宿主**只给三样**：id / name / language。官方 JS 端还会带
// `preparation.phase`、`location`、`downloadSources`、`setupEstimate`（`client.js:174-350`
// 的 zod schema），本仓宿主不返回这些 —— 界面不得凭空画"下载进度"或"安装步骤"，
// 只能按 `id/name/language` 画真实目录。
type SpeechProvider struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Language string `json:"language"`
}

// SpeechConfig 是 `speech/configure` 的入参与回参（宿主 `OfficialSpeechConfig`）。
type SpeechConfig struct {
	ProviderID string `json:"providerId,omitempty"`
	Language   string `json:"language,omitempty"`
}

// SpeechTranscription 是 `speech/transcribe` 的结果（宿主 `OfficialSpeechTranscription`）。
type SpeechTranscription struct {
	Text             string  `json:"text"`
	AudioSeconds     float64 `json:"audioSeconds"`
	InferenceSeconds float64 `json:"inferenceSeconds"`
}

// SpeechStatus 是 `speech/follow` 的一帧（宿主 `OfficialSpeechStatus`）。
//
// `State` 的取值来自宿主引擎：`idle` / `preparing` / `prepared` /
// `transcribing` / `canceled` / `error`。`Prepared` 单独一格是因为宿主在
// `prepared` 之外还会用它表达"这条 provider 已经就绪但当前没有活动任务"。
type SpeechStatus struct {
	Type       string `json:"type"`
	ProviderID string `json:"providerId"`
	State      string `json:"state"`
	Prepared   bool   `json:"prepared"`
	Error      string `json:"error,omitempty"`
}

// SpeechCatalog 列可用的识别器。
//
// 返回空切片 = 宿主没有识别器（不是"还没加载"）：宿主 `Catalog` 在
// helper 报错时返回 `ErrNoEngine`，HTTP 侧转成错误体，所以能走到这里的空
// 切片只可能是"引擎说有、但一条都没有"这种不正常情况，调用方按"不可用"处理。
func (c *Client) SpeechCatalog(ctx context.Context) ([]SpeechProvider, error) {
	var out []SpeechProvider
	if err := c.Call(ctx, "speech.catalog", struct{}{}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SpeechConfigure 写当前的 provider / 语言选择。
//
// 宿主把它存在引擎的 settings 里（进程内），**不落盘**：重启宿主就回到
// catalog 的第一条。所以界面每次启动都要重新 `configure`（壳在拿到 catalog
// 后就发一次），不能假设"上次选的那条还在"。
func (c *Client) SpeechConfigure(ctx context.Context, patch SpeechConfig) (SpeechConfig, error) {
	var out SpeechConfig
	if err := c.Call(ctx, "speech.configure", map[string]any{"patch": patch}, &out); err != nil {
		return SpeechConfig{}, err
	}
	return out, nil
}

// SpeechPrepare 让宿主准备一条 provider。
//
// 本仓宿主用 Windows **已安装**的识别器，所以 `prepare` 是"确认存在 + 标记 prepared"
// （`HOST:internal/speech/engine.go:284-300`），不下载任何模型。`downloadSource`
// 因此通常为空；非空也不会被消费，宿主只在 provider 不存在时把它写进错误里。
func (c *Client) SpeechPrepare(ctx context.Context, providerID, downloadSource string) error {
	if strings.TrimSpace(providerID) == "" {
		return fmt.Errorf("speech.prepare: providerId 必填")
	}
	params := map[string]any{"providerId": providerID}
	if downloadSource != "" {
		params["options"] = map[string]any{"downloadSource": downloadSource}
	}
	return c.Call(ctx, "speech.prepare", params, nil)
}

// SpeechCancelPreparation 取消一条 provider 的准备。
func (c *Client) SpeechCancelPreparation(ctx context.Context, providerID string) error {
	if strings.TrimSpace(providerID) == "" {
		return fmt.Errorf("speech.cancelPreparation: providerId 必填")
	}
	return c.Call(ctx, "speech.cancelPreparation", map[string]any{"providerId": providerID}, nil)
}

// SpeechTranscribe 把一段 16 kHz 单声道 WAV（base64）交给宿主识别。
//
// `audioBase64` 是**裸 base64**，不带 `data:audio/wav;base64,` 前缀：
// 宿主直接 `base64.StdEncoding.DecodeString`（`HOST:cmd/dshd/serve_speech.go:37-43`），
// 带前缀会解码失败并被当成 `invalid_audio`。
//
// 失败按宿主的四字段错误返回，调用方必须把 message 显示出来：
// `empty`（没识别出文字）与 `invalid_audio`（音频坏了）是两种完全不同的
// 用户可见结果，不能都画成"识别失败"。
func (c *Client) SpeechTranscribe(ctx context.Context, providerID, language, audioBase64 string) (SpeechTranscription, error) {
	if audioBase64 == "" {
		return SpeechTranscription{}, fmt.Errorf("speech.transcribe: audioBase64 必填")
	}
	var out SpeechTranscription
	req := map[string]any{"audioBase64": audioBase64}
	if providerID != "" {
		req["providerId"] = providerID
	}
	if language != "" {
		req["language"] = language
	}
	if err := c.Call(ctx, "speech.transcribe", map[string]any{"request": req}, &out); err != nil {
		return SpeechTranscription{}, err
	}
	return out, nil
}

// SpeechFollow 订阅识别器状态流，每来一帧调一次 fn；ctx 取消即结束。
//
// 这是**服务端流**（宿主写 `application/x-ndjson`，一帧一行），所以不能走
// `Call`（它把整个 body 读完才返回）。这里自己起请求、逐行解。
//
// 基线帧语义：宿主保证至少给一帧（`HOST:internal/speech/engine.go:394-410`），
// 所以 fn 一定至少被调用一次；**没有帧**等于这条流是坏的，不是"没变化"。
func (c *Client) SpeechFollow(ctx context.Context, fn func(SpeechStatus) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Base+"/api/speech.follow", strings.NewReader("{}"))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for _, ck := range c.cookies {
		req.AddCookie(ck)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("连不上 %s：%w", c.Base, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("HTTP %d: %.200s", resp.StatusCode, raw)
	}
	sc := bufio.NewScanner(resp.Body)
	// 一帧 status 只有几百字节，但给足余量：将来宿主给 provider 加字段不该炸。
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var st SpeechStatus
		if err := json.Unmarshal([]byte(line), &st); err != nil {
			return fmt.Errorf("speech.follow 帧不是合法 JSON: %.200s", line)
		}
		if err := fn(st); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}
