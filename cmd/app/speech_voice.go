// speech_voice.go —— 麦克风听写（官方 `dsh-experimental-client-ui-voice-input`）的壳侧实现。
//
// # 官方那个包到底要什么
//
// 官方客户端半边（`OFF:dsh-experimental-client-ui-voice-input/lib/client.js`）向平台要的能力
// 只有三样，全在浏览器里：
//
//	:1528-1536  navigator.mediaDevices.getUserMedia({audio:{deviceId}})
//	:1573-1577  new MediaRecorder(stream)
//	:1651       new OfflineAudioContext(1, …, 16e3)（重采样到 16 kHz 单声道）
//
// 第四样是宿主（`ctx.remote.speech.*` 六方法），本仓宿主**早就实现了**
// （`HOST:cmd/dshd/serve_speech.go` + `internal/speech`，真跑 Windows System.Speech）。
// 所以这个包的 gap **一直在客户端采集这一侧**，不在宿主。
//
// # 本仓怎么替代
//
//	官方浏览器能力                  本仓
//	getUserMedia({deviceId})        internal/audio（winmm waveIn，纯 syscall）
//	MediaRecorder                   采集循环逐块累积 PCM
//	OfflineAudioContext(…,16000)    向驱动直接要 16 kHz 单声道 16-bit
//	encodeWave + btoa               audio.EncodeWAV + base64
//	analyser → RMS 电平              audio.Recorder.Level()
//	ctx.remote.speech.*             internal/apiclient.Speech*
//
// # 状态机的四档（与官方同名字面量）
//
//	idle → requesting → recording → transcribing → feedback → idle
//
// `requesting` 是**真实存在**的一档，不是装饰：本机实测 `waveInStart` 之后要
// 约 300 ms 才回来第一块 PCM（USB 麦的周期），这段区间用户按"停止"必须被
// 忽略（官方同一处理：`active.phase === "recording"` 才允许 finish）。
package main

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/audio"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// voicePhase* 是官方 `client.js:1840` 的五个 phase 字面量。
const (
	voicePhaseIdle         = "idle"
	voicePhaseRequesting   = "requesting"
	voicePhaseRecording    = "recording"
	voicePhaseTranscribing = "transcribing"
	voicePhaseFeedback     = "feedback"
)

// voiceCatalogTimeout 是 `speech.catalog` 的超时。宿主这条要跑一次受管 helper
// （`--catalog`），冷启动时编译 + 进程启动能到几秒，给 8 秒。
const voiceCatalogTimeout = 8 * time.Second

// voiceTranscribeTimeout 是 `speech.transcribe` 的超时。
//
// 宿主侧是"写临时 WAV → 跑 helper 识别 → 读回 JSON"，helper 自带超时；
// 这里给 3 分钟是给长录音与慢机器留余量，不是要等这么久。
const voiceTranscribeTimeout = 3 * time.Minute

// voiceOpenRecorder 是"开一个采集器"的入口，单独一格为了测试能替换它。
//
// 为什么留这个缝：`audio.Open` 会**真的打开声卡**。采集逻辑本身已经在
// `internal/audio` 用真实设备测过（`capture_windows_test.go`），而这里要测的是
// 壳的状态机（requesting → recording → transcribing → 草稿），拿真麦克风跑
// 状态机只会让测试依赖"这台机器有没有麦、麦有没有静音"。
var voiceOpenRecorder = audio.Open

// voiceLevelTick 是电平刷新间隔：官方 analyser 是逐帧读的，本仓按 50 ms
// （20 Hz）刷新足够画一条会动的波形，又不至于把整棵树每帧重建。
const voiceLevelTick = 50 * time.Millisecond

// newVoiceWiring 造一份零状态。deviceID 必须初始化成 -1（= `WAVE_MAPPER`
// 系统默认），零值 0 会去开"设备 0"，那是**另一件事**：有两只麦的机器上
// 用户刚上手就固定选了第一只，而他从来没选过。
func newVoiceWiring() voiceWiring {
	return voiceWiring{deviceID: -1, phase: voicePhaseIdle}
}

// voiceWiring 是听写的壳侧全部状态。**所有字段都在 model.mu 下读写**。
type voiceWiring struct {
	// 录音设备名录（插拔后由 RefreshDevices 重读）。
	devices        []audio.Device
	devicesLoaded  bool
	devicesPending bool
	devicesErr     string
	// deviceID 是选中的设备序号；-1 = 系统默认（WAVE_MAPPER）。
	deviceID int

	// 识别器目录（宿主 speech/catalog）。
	providers        []apiclient.SpeechProvider
	providersLoaded  bool
	providersPending bool
	providersErr     string
	providerID       string
	language         string
	configured       bool

	// 会话状态。
	phase     string
	message   string
	errText   string
	level     float64
	startedAt time.Time
	seconds   float64
	gen       int
	rec       audio.Recorder
	// span 是**采集开始那一刻**的光标/选区快照（官方 `active.span =
	// inputActions.captureInsertion()`，VoiceInput.tsx:131-134）。
	// spanOK=false 表示 composer 没提供插入能力（老壳 / 没装插件）→ 退回追加到末尾。
	span   pluginapi.InsertionSpan
	spanOK bool
	// pending 是 `insertText` 没插进去时**保住的**识别文字（官方 `setPending`，
	// VoiceInput.tsx:122）。空串 = 没有待插入的文字。
	pending string
}

// voiceCapability 是 `VoiceActions.Capability`：同步返回缓存，首次问触发后台读取。
func (m *model) voiceCapability() pluginapi.VoiceCapability {
	m.mu.Lock()
	if !m.voice.devicesLoaded && !m.voice.devicesPending {
		m.voice.devicesPending = true
		go m.refreshVoiceDevices()
	}
	if !m.voice.providersLoaded && !m.voice.providersPending && m.cli != nil {
		m.voice.providersPending = true
		go m.refreshVoiceProviders()
	}
	devices := m.voice.devices
	deviceID := m.voice.deviceID
	providers := m.voice.providers
	providersLoaded := m.voice.providersLoaded
	devicesLoaded := m.voice.devicesLoaded
	devicesErr := m.voice.devicesErr
	providersErr := m.voice.providersErr
	providerID := m.voice.providerID
	language := m.voice.language
	m.mu.Unlock()

	out := pluginapi.VoiceCapability{
		ProviderID: providerID,
		Language:   language,
		DeviceID:   deviceID,
		MaxSeconds: audio.MaxSeconds,
	}
	for _, d := range devices {
		out.Devices = append(out.Devices, pluginapi.VoiceDevice{ID: d.ID, Name: d.Name})
	}
	for _, p := range providers {
		out.Providers = append(out.Providers, pluginapi.VoiceProvider{ID: p.ID, Name: p.Name, Language: p.Language})
	}
	// 判定顺序照官方的可用性判据：先要有麦克风（浏览器那边是 getUserMedia 能不能起），
	// 再要有识别器（`provider.preparation.phase === 'ready'` 那一档）。
	switch {
	case !devicesLoaded && devicesErr == "":
		out.Reason = "正在枚举录音设备"
	case devicesErr != "":
		out.Reason = devicesErr
	case len(devices) == 0:
		out.Reason = "这台机器没有录音设备"
	case !providersLoaded && providersErr == "":
		out.Reason = "正在读取语音识别器"
	case providersErr != "":
		out.Reason = providersErr
	case len(providers) == 0:
		out.Reason = "宿主没有可用的语音识别器"
	default:
		out.Available = true
	}
	return out
}

// voiceState 是 `VoiceActions.State`。
func (m *model) voiceState() pluginapi.VoiceState {
	m.mu.Lock()
	st := pluginapi.VoiceState{
		Phase:   m.voice.phase,
		Message: m.voice.message,
		Error:   m.voice.errText,
		Level:   m.voice.level,
		Seconds: m.voice.seconds,
		// Pending 是官方的 `pending`：`insertText` 因为草稿变过而没插进去时
		// **保住的**那段识别文字。界面据此把「重新录音」换成「插入文字」
		// （VoiceInput.tsx:175-178）。
		Pending: m.voice.pending,
	}
	recording := m.voice.phase == voicePhaseRecording
	startedAt := m.voice.startedAt
	m.mu.Unlock()
	if st.Phase == "" {
		st.Phase = voicePhaseIdle
	}
	if recording && !startedAt.IsZero() {
		st.Seconds = time.Since(startedAt).Seconds()
		if st.Seconds > audio.MaxSeconds {
			st.Seconds = audio.MaxSeconds
		}
	}
	return st
}

// refreshVoiceDevices 重读录音设备名录。
func (m *model) refreshVoiceDevices() {
	devices, err := audio.Devices()
	m.mu.Lock()
	m.voice.devicesPending = false
	if err != nil {
		m.voice.devicesErr = err.Error()
		m.voice.devices = nil
	} else {
		m.voice.devices = devices
		m.voice.devicesErr = ""
		if m.voice.deviceID >= len(devices) {
			// 插拔之后序号越界：退回系统默认，不要留一个指不到的序号。
			m.voice.deviceID = -1
		}
	}
	m.voice.devicesLoaded = true
	m.mu.Unlock()
	m.inv.Invalidate()
}

// refreshVoiceProviders 读一次 `speech/catalog`，并**顺手 configure 一次**。
//
// 为什么必须 configure：宿主把选择存在引擎的进程内 settings 里（
// `HOST:internal/speech/engine.go:305-317`），重启宿主就回到空。而
// `speech.transcribe` 在 providerId 为空时会回 `ErrNotPrepared`，所以不能
// 假设"目录里的第一条就是当前选择"。
func (m *model) refreshVoiceProviders() {
	ctx, cancel := context.WithTimeout(context.Background(), voiceCatalogTimeout)
	defer cancel()
	providers, err := m.cli.SpeechCatalog(ctx)

	m.mu.Lock()
	m.voice.providersPending = false
	if err != nil {
		m.voice.providersErr = err.Error()
		m.mu.Unlock()
		m.inv.Invalidate()
		return
	}
	m.voice.providers = providers
	m.voice.providersLoaded = true
	m.voice.providersErr = ""
	chosen := m.voice.providerID
	lang := m.voice.language
	if chosen == "" && len(providers) > 0 {
		chosen = providers[0].ID
		lang = providers[0].Language
		m.voice.providerID = chosen
		m.voice.language = lang
	}
	needConfigure := !m.voice.configured && chosen != ""
	if needConfigure {
		m.voice.configured = true
	}
	m.mu.Unlock()
	m.inv.Invalidate()

	if needConfigure {
		cfgCtx, cfgCancel := context.WithTimeout(context.Background(), voiceCatalogTimeout)
		defer cfgCancel()
		if _, err := m.cli.SpeechConfigure(cfgCtx, apiclient.SpeechConfig{ProviderID: chosen, Language: lang}); err != nil {
			m.mu.Lock()
			m.voice.configured = false
			m.voice.providersErr = "选择识别器失败：" + err.Error()
			m.mu.Unlock()
			m.inv.Invalidate()
		}
	}
}

// voiceBegin 是 `VoiceActions.Begin`。
func (m *model) voiceBegin() {
	m.mu.Lock()
	if m.voice.phase == voicePhaseRequesting || m.voice.phase == voicePhaseRecording || m.voice.phase == voicePhaseTranscribing {
		m.mu.Unlock()
		return // 已经有一次在飞：官方也只看 current.current 非空就 return
	}
	m.voice.gen++
	gen := m.voice.gen
	deviceID := m.voice.deviceID
	providersLoaded := m.voice.providersLoaded
	m.voice.phase = voicePhaseRequesting
	m.voice.message = ""
	m.voice.errText = ""
	m.voice.level = 0
	m.voice.seconds = 0
	// 官方在 `start()` 里就把光标快照存进 active.span（VoiceInput.tsx:131-134），
	// 之后整段识别都用它——识别要几秒，中途用户可能挪了光标，插的位置必须还是
	// "他按下麦克风时那个位置"。本仓这一句与 dsvoice 的点击**同一帧**（点击在
	// 绘制里处理），所以这里拿到的是真光标，不是上一帧的残影。
	m.voice.span, m.voice.spanOK = m.captureInsertionSpan()
	// 新一次采集清掉上一次的待插入文字（官方 `setPending('')`，同处 :136）。
	m.voice.pending = ""
	m.mu.Unlock()
	m.inv.Invalidate()

	go func() {
		if !providersLoaded {
			m.refreshVoiceProviders()
		}
		m.mu.Lock()
		stale := m.voice.gen != gen || m.voice.phase != voicePhaseRequesting
		m.mu.Unlock()
		if stale {
			return
		}
		rec, err := voiceOpenRecorder(deviceID)
		if err == nil {
			err = rec.Start()
		}
		if err != nil {
			if rec != nil {
				rec.Close()
			}
			m.voiceFail(gen, "麦克风打不开："+err.Error())
			return
		}
		m.mu.Lock()
		if m.voice.gen != gen || m.voice.phase != voicePhaseRequesting {
			m.mu.Unlock()
			rec.Close()
			return
		}
		m.voice.rec = rec
		m.voice.phase = voicePhaseRecording
		m.voice.startedAt = time.Now()
		m.mu.Unlock()
		m.inv.Invalidate()

		go m.voiceRecordLoop(gen, rec)
		go m.voiceAutoStop(gen)
	}()
}

// voiceRecordLoop 在录音期间刷新电平与时长；phase 一离开 recording 就退出。
func (m *model) voiceRecordLoop(gen int, rec audio.Recorder) {
	ticker := time.NewTicker(voiceLevelTick)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		alive := m.voice.gen == gen && m.voice.phase == voicePhaseRecording && m.voice.rec == rec
		if !alive {
			m.mu.Unlock()
			return
		}
		m.voice.level = rec.Level()
		m.voice.seconds = time.Since(m.voice.startedAt).Seconds()
		m.mu.Unlock()
		m.inv.Invalidate()
	}
}

// voiceAutoStop 是官方的 `active.timer`：到 MaxSeconds 自动结束并识别
// （`client.js:1962-1964`）。宿主目录不给我们上限，所以上限是 audio.MaxSeconds。
func (m *model) voiceAutoStop(gen int) {
	time.Sleep(time.Duration(audio.MaxSeconds) * time.Second)
	m.mu.Lock()
	due := m.voice.gen == gen && m.voice.phase == voicePhaseRecording
	m.mu.Unlock()
	if due {
		m.voiceStop()
	}
}

// voiceStop 是 `VoiceActions.Stop` / 自动停止：结束采集并把 WAV 交给宿主识别。
func (m *model) voiceStop() {
	m.mu.Lock()
	if m.voice.phase != voicePhaseRecording {
		m.mu.Unlock()
		return
	}
	gen := m.voice.gen
	rec := m.voice.rec
	providerID := m.voice.providerID
	language := m.voice.language
	m.voice.phase = voicePhaseTranscribing
	m.voice.level = 0
	m.mu.Unlock()
	m.inv.Invalidate()

	go func() {
		if rec == nil {
			m.voiceFail(gen, "采集器已经释放")
			return
		}
		wav, err := rec.Stop()
		rec.Close()
		m.mu.Lock()
		if m.voice.gen == gen {
			m.voice.rec = nil
		}
		m.mu.Unlock()
		if err != nil {
			m.voiceFail(gen, "录音失败："+err.Error())
			return
		}
		if len(wav) <= 44 {
			m.voiceFail(gen, "没有录到音频")
			return
		}
		if m.cli == nil {
			m.voiceFail(gen, "宿主客户端没有接线")
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), voiceTranscribeTimeout)
		defer cancel()
		tr, err := m.cli.SpeechTranscribe(ctx, providerID, language, base64.StdEncoding.EncodeToString(wav))
		if err != nil {
			m.voiceFail(gen, "识别失败："+err.Error())
			return
		}
		text := strings.TrimSpace(tr.Text)
		if text == "" {
			// 官方这一档是 `empty`（"没有识别出文字"），不是"失败"：两者对用户
			// 的意义完全不同——一个是"再说一次"，一个是"去找原因"。
			m.voiceFail(gen, "没有识别出文字，再说一次")
			return
		}
		m.mu.Lock()
		stale := m.voice.gen != gen
		if !stale {
			m.voice.phase = voicePhaseIdle
			m.voice.message = ""
			m.voice.errText = ""
			m.voice.seconds = 0
		}
		m.mu.Unlock()
		if stale {
			return
		}
		// 草稿是 Viva editor 的状态，只有 UI 线程能写（见 wpc1_send.go 的注释），
		// 所以这里只排队。
		//
		// 与官方同一语义（VoiceInput.tsx:122-123）：先试**插到采集时的光标处**，
		// 插不进去就把文字留在 pending 并进 feedback —— 不许丢、也不许硬塞到旧位置。
		// 真正的插入在 `applyPendingInsert`（UI 线程）里做。
		m.mu.Lock()
		req := &draftInsertRequest{text: text, span: m.voice.span, spanOK: m.voice.spanOK}
		m.pendingInsert = req
		m.mu.Unlock()
		m.inv.Invalidate()
	}()
}

// voiceCancel 是 `VoiceActions.Cancel`：放弃这一次听写。
func (m *model) voiceCancel() {
	m.mu.Lock()
	gen := m.voice.gen
	gen++ // 让所有在飞的阶段失效
	m.voice.gen = gen
	rec := m.voice.rec
	m.voice.rec = nil
	m.voice.phase = voicePhaseIdle
	m.voice.message = ""
	m.voice.errText = ""
	m.voice.level = 0
	m.voice.seconds = 0
	m.voice.startedAt = time.Time{}
	m.mu.Unlock()
	if rec != nil {
		rec.Close()
	}
	m.inv.Invalidate()
}

// voiceRetry 是 `VoiceActions.Retry`：从 feedback 直接重来一次。
func (m *model) voiceRetry() {
	m.mu.Lock()
	m.voice.phase = voicePhaseIdle
	m.voice.message = ""
	m.mu.Unlock()
	m.voiceBegin()
}

// voiceInsert 是 `VoiceActions.Insert`：官方「插入文字」按钮
// （`VoiceInput.tsx:175-178`）。
//
// 官方那一下是 `insertText(pending, captureInsertion())`——**重新采一次光标**，
// 不沿用采集时那个 span（那个多半已经过期了，否则当初就插进去了）。成功才清
// pending 并回 idle；失败就什么都不动（文字还在 pending 里，按钮还在）。
func (m *model) voiceInsert() {
	m.mu.Lock()
	text := m.voice.pending
	m.mu.Unlock()
	if text == "" {
		return
	}
	if m.insertDraftAtSpan(text) {
		m.mu.Lock()
		m.voice.pending = ""
		m.voice.message = ""
		m.voice.phase = voicePhaseIdle
		m.mu.Unlock()
		m.inv.Invalidate()
	}
}

// voiceConflictMessage 是官方 `conflict` 的 zh 原文
// （`OFF:dsh-experimental-client-ui-voice-input/src/client/locales.ts:100`）。
const voiceConflictMessage = "草稿已被修改。识别文字已保留，可在当前光标位置插入。"

// draftInsertion 取 composer 交出的草稿插入能力（nil = 没装/老壳）。
func (m *model) draftInsertion() pluginapi.DraftInsertion {
	if m.kernel == nil {
		return nil
	}
	if service, ok := m.kernel.Get(pluginapi.KeyDraftInsertion).(pluginapi.DraftInsertion); ok {
		return service
	}
	return nil
}

// captureInsertionSpan 取当前光标 / 选区快照（**UI 线程**）。
func (m *model) captureInsertionSpan() (pluginapi.InsertionSpan, bool) {
	if service := m.draftInsertion(); service != nil {
		return service.CaptureInsertion()
	}
	return pluginapi.InsertionSpan{}, false
}

// insertDraftAtSpan 把一段文字插到**现在**的光标处，返回是否插进去了（**UI 线程**）。
//
// composer 没提供能力时退回 `insertDraftInline`（追加到末尾）：宁可位置不对，
// 也不许把用户刚说的话丢掉。这条兜底只在"没装 ds-composer"时才会走到。
func (m *model) insertDraftAtSpan(text string) bool {
	service := m.draftInsertion()
	if service == nil {
		m.insertDraftInline(text)
		return true
	}
	span, ok := service.CaptureInsertion()
	if !ok {
		m.insertDraftInline(text)
		return true
	}
	return service.InsertText(text, span)
}

// voiceSelectDevice 换一只麦克风。只影响**下一次** Begin。
func (m *model) voiceSelectDevice(deviceID int) {
	m.mu.Lock()
	known := false
	for _, d := range m.voice.devices {
		if d.ID == deviceID {
			known = true
			break
		}
	}
	if known || deviceID < 0 {
		m.voice.deviceID = deviceID
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}

// voiceRefreshDevices 重新枚举设备（插拔后）。
func (m *model) voiceRefreshDevices() {
	m.mu.Lock()
	if m.voice.devicesPending {
		m.mu.Unlock()
		return
	}
	m.voice.devicesPending = true
	m.mu.Unlock()
	go m.refreshVoiceDevices()
}

// voiceFail 把一次失败落到 feedback 阶段（官方 `feedback(text)`）。
//
// `gen` 是防护：取消之后才回来的识别结果不许顶上界面。
func (m *model) voiceFail(gen int, message string) {
	m.mu.Lock()
	if m.voice.gen != gen {
		m.mu.Unlock()
		return
	}
	m.voice.phase = voicePhaseFeedback
	m.voice.message = message
	m.voice.errText = message
	m.voice.level = 0
	m.mu.Unlock()
	m.inv.Invalidate()
}

// insertDraftInline 把听写文字追加到草稿末尾（**只在 UI 线程调**）。
//
// 与官方 `inputActions.insertText(text, span)` 的差别：官方插在**采集时的光标**
// 位置并做 span 冲突检测，本仓只能追加到末尾（`ui.Field` 只公开 Text/SetText，
// composer 的 editor 不经 pluginapi 暴露，见 VoiceState.Pending 的注释）。
// 草稿为空时两者等价——那是听写的常态。
func (m *model) insertDraftInline(text string) {
	if text == "" {
		return
	}
	m.draft.SetText(joinInline(m.draft.Text(), text))
	if m.kernel == nil {
		return
	}
	for _, plugin := range m.kernel.Plugins() {
		if plugin.ID() != "ds-composer" {
			continue
		}
		fields, ok := plugin.(pluginapi.Fields)
		if !ok {
			continue
		}
		for _, field := range fields.Fields() {
			if field != nil {
				field.SetText(joinInline(field.Text(), text))
			}
		}
	}
}

// joinInline 是"追加到末尾"的连接规则：两边都有内容且都不是空白结尾时补一个空格。
//
// 不补空格会把"你好"和"世界"粘成"你好世界"——听写是往已有草稿后面续话，
// 不是拼词。这条规则只在**追加**语义下成立；官方插在光标处，那边由光标
// 两边的原文决定，不存在这条规则。
func joinInline(existing, addition string) string {
	if addition == "" {
		return existing
	}
	if existing == "" {
		return addition
	}
	if strings.HasSuffix(existing, " ") || strings.HasSuffix(existing, "\n") || strings.HasSuffix(existing, "\t") {
		return existing + addition
	}
	return existing + " " + addition
}
