# `#66 dsh-experimental-client-ui-voice-input`：从 ❌ 到 🟡 的实现与证据（2026-11-17）

> 一句话：**麦克风听写的界面半边这次真接上了**（采集 → 识别 → 入草稿，全链真实现 + 测试），
> 剩余差异与宿主侧的阻断逐条写在下面；端到端今天跑不通的部分**在宿主**，不在界面。

## 1. 官方包要什么（逐行，源码取自 rc.2 asar 的 **未打包 src**）

`app.asar` 里 `@deepseek-ai/dsh-experimental-client-ui-voice-input/src/client/` 有 15 个文件，
本行相关的四个：

| 官方 | 位置 | 要平台给什么 |
|---|---|---|
| `audio.ts` | `:59-63` `navigator.mediaDevices.getUserMedia({audio:{deviceId}})`；`:43,95` `MediaRecorder`；`:152` `new OfflineAudioContext(1, …, 16e3)`；`encodeWave` | 麦克风采集 + 重采样到 16 kHz 单声道 + WAV 编码 |
| `VoiceInput.tsx` | `:58` 五档 phase；`:147-166` 折叠态麦克风钮；`:167-181` 展开态 `.captureRow`；`:122` `inputActions.insertText(text, span)` | composer 的 activity 座位 + 草稿光标插入 |
| `Waveform.tsx` | `:20` `height = 1 + min(1, level*5) * 17`；`:29-31` 80 根 `line`、`viewBox 0 0 640 40` | RMS 电平 |
| `mount.ts` / `index.ts` | `slots.register({name:'conversation.input.activity', …})` | 座位 `OFF:ui-conversation/…/slots.ts:331` |

宿主侧六方法 `speech/catalog|prepare|cancelPreparation|configure|transcribe|follow`
（`OFF:…/lib/client.js:542-560` 的 remote 定义）**本仓宿主早已实现**。

## 2. 本轮实现（4 个新包 + 1 处接线）

| 落点 | 内容 | 测试 |
|---|---|---|
| `internal/audio` | Windows `winmm` `waveIn*` 采集（**纯 `syscall`，无 cgo**，满足 CI 的 `CGO_ENABLED=0`）；设备枚举（`waveInGetDevCapsW`）；RMS 电平；WAV 编码；非 Windows 结构化 `ErrUnsupported` | `audio_test.go`（WAV 头逐字段、byteRate/blockAlign、RMS 刻度、Seconds）+ `capture_windows_test.go`（真机枚举/采集/Stop 前未 Start/幂等 Close） |
| `internal/apiclient/speech.go` | 六方法客户端：`SpeechCatalog/Configure/Prepare/CancelPreparation/Transcribe/Follow`，按**宿主 wire 形状**发（`request`/`patch`/`options` 子对象、裸 base64、NDJSON 流） | `speech_test.go` 8 条（含"空音频不发请求"、"follow 回调失败即停"） |
| `cmd/app/speech_voice.go` | 壳侧五档状态机 + 60 s 自动停 + 取消代次防串台 + 草稿写入排队 + 设备/目录缓存 | `speech_voice_test.go` 8 条（假宿主 + 假采集器：全链、空结果不写草稿、宿主错误原文、麦打不开、取消丢弃） |
| `plugins/dsvoice` | `conversation.input.activity` 占用方：28px 麦克风钮 / `.captureRow`（32px 圆钮 + 官方 80 柱波形 + 文案）；图元用**官方精灵 path 原文** | `voice_test.go` 7 条（headless 真像素：装进真内核后座位上确有占用方、不可用不画、图标墨迹可点且 `Begin` 被调、波形柱高按官方公式、四段官方中文文案、feedback 重试钮） |
| 接线 | `cmd/app/main.go`（插件导入 + `Actions.Voice`）、`pluginapi`（`VoiceActions/VoiceCapability/VoiceState/VoiceDevice/VoiceProvider` + `DraftInsert`）、`testdata/api.snapshot.json`（batch25 五方法，`counts.total` 122→127） | `TestEveryPluginDirIsImported`、`TestSnapshot*` 全绿 |

几何全部取自官方 `VoiceInput.module.css`（`.trigger{28px}`、`.captureRow{gap:12px;min-height:34px}`、
`.roundButton{32×32,r:50%}`、`.waveform{height:24px}`、`.activityMessage{12px}`）与 `VoiceInput.tsx` 的 size 实参；
文案逐字取 `locales.ts` 的 zh 词典（`start='开始录音'`、`stop='停止并识别'`、`requesting='请允许使用麦克风…'`、
`transcribingShort='识别中…'`、`retryRecording='重新录音'`、`empty='未识别到语音'`、`failed='语音识别失败：{message}'`）。

麦克风图标不是手画的：官方 `@deepseek-ai/dsh-client-ui-primitives` 的精灵表里
`"microphone-outline"` 的 `<rect x=5 y=1.5 w=6 h=9 rx=3>` + `M2.35 8.675C…` 原样搬过来
（rect 用 Viva 原生圆角矩形，因为 `gfx/path.go:9-14` 的路径词汇没有弧）；`close-outline` 与 `stop-fill` 同理。

## 3. 真机证据（HTTP 全链）

真实 `dshd serve`（临时 home + `--addr 127.0.0.1:0`）＋ 本仓真实 `apiclient`：

```
CONNECT_OK
CATALOG_OK n=1                       id="MS-2052-80-DESK" lang="zh-CN"
CONFIGURE_OK cfg={ProviderID:MS-2052-80-DESK Language:zh-CN}
PREPARE_OK / CANCEL_OK
```

`speech.transcribe` **失败**，两种形态（同一份 `synth.wav`、同一个 providerId）：

1. 默认 `%TEMP%`：
   `internal: open C:\Users\…\Temp\dsh-speech-audio-503651648\audio.wav: Access is denied`
   —— 宿主在**写临时 WAV** 这一步就被文件策略拒了，根本没走到 helper。
2. `TMP`/`TEMP` 指到工作区内后写盘通过，但 helper 抛
   `System.ArgumentException: 找不到具有所需 ID 的识别器 ---> System.PlatformNotSupportedException: 在此系统上语音识别不可用。找不到 SAPI 和语音识别引擎。`
   —— 新编译的 helper 建不起 `SpeechRecognitionEngine`；**同一份源码 2026-11-08 编译的旧 helper
   用同样参数可以正常识别**（`{"text":"然后我给市水产"}`）。

对照：**不经 dshd、直接调 `internal/speech`** 的探针（`Catalog` → `Synthesize` → `Transcribe`）成功。
所以宿主缺陷的边界是"在 dshd 进程里跑 helper 这条链"，不是识别器本身 → 归 backend 修，本行登记。

本机输入设备：只有 `麦克风 (花再 Halo PixelBar)`，实测交付**数字静音**
（1.2 s 采 14849 样本、峰值 1 LSB）→ 采集链与 WAV 已验证，**真实说话未验**。

## 4. 仍是 🟡 的六条差异

| # | 官方 | 本仓 | 为什么 |
|---|---|---|---|
| 1 | `insertText(text, span)`：插在**采集时光标**处 + span 冲突检测，失败进 `pending` 并画「插入文字」 | 追加到草稿末尾（`DraftInsert` 策略），`pending` 恒空 | `viva/ui/field.go:82-95` 的 `Field` 只公开 `Text/SetText`；composer 的 editor 不经 `pluginapi` 暴露，拿不到光标 |
| 2 | `VoiceSetupPrompt.tsx`（`plugins.bundle.activation` 引导） | 不画 | 它讲"去下载本地模型"，本仓宿主用 Windows 已装识别器，`prepare` 是 no-op（`HOST:internal/speech/engine.go:284-300`） |
| 3 | `PreparationCard.tsx`（`plugins.bundle.config` 下载进度/下载源/空间估算） | 不画 | 宿主 `speech/catalog` 只回 `id/name/language`，没有 `preparation.phase`/`location`/`downloadSources` |
| 4 | `MicrophonePicker.tsx`（设备选择 + 实时电平预览） | 只有壳里的 `audio.Devices` + `Actions.Voice.SelectDevice`，没有设置页 | 设置页那一层 UI 未做 |
| 5 | `speech/follow` 状态流驱动 `preparation.phase` | 客户端已实现（NDJSON 逐行），但**未被消费** | 宿主 `prepare` 是 no-op、`transcribe` 不看 prepared，没有相位可显 |
| 6 | 波形/`StateDot` 动画随 rAF | 随壳的 50 ms 电平 tick；`StateDot` 画静帧 | 录音之外没有逐帧重绘源 |

## 5. 复跑方式

```bash
export PATH="/c/Program Files/Go/bin:/c/toolchains/mingw64/bin:$PATH"
CGO_ENABLED=0 go test ./internal/audio/ ./internal/apiclient/ ./plugins/dsvoice/ -count=1
CGO_ENABLED=0 go test ./cmd/app/ -run 'TestVoice|TestJoinInline' -count=1 -v
```
