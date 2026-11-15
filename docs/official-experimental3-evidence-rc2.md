# 实验包三连（#63 / #64 / #66）：官方默认状态、依赖与落点

> 口径：官方 rc.2 解包根记作 `OFF` =
> `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai`（本仓之外，只读）；
> 宿主指 `../DSH-GO`。**默认状态先查后判**（用户裁定：官方的都要有，关闭的就默认关闭），
> 三个包**官方都是"随安装发运、默认不启用"**——所以"默认关闭"不等于"不做"。

## 1. 官方默认状态：三个都默认关闭，证据三条

| 证据 | 位置 | 实际值 |
|---|---|---|
| 不在 base 补丁里 | `OFF/dsh-base/cordis.patch.yml` | 全文 21 KB，`grep experimental` **0 命中**：默认 profile 里根本没有它们 |
| 各自 bundle 无 `disabled` 行 | `OFF/dsh-experimental-cot-translation-bundle/cordis.patch.yml:4-8`<br>`OFF/dsh-experimental-voice-input-bundle/cordis.patch.yml:12-13`<br>`OFF/dsh-experimental-forge-bundle/cordis.patch.yml:12-13` | `- id: cot-translation` / `- id: ui-voice-input` / `- id: forge-panel` 都是**裸 insert**，一旦装进 profile 就是开的 |
| 官方把"默认关闭"钉在可选清单 | `OFF/dsh-official-bundles/lib/index.js:4-10`、`:25-31`、`:33-49`（`OPTIONAL_BUNDLES`）、`:119-134`（`EXPERIMENTAL_BUNDLES`） | `PROFILE_TEMPLATES` 五个模板**一个 experimental 都没有**；注释原文 "selected by no shipped template, and **offered switched off by the plugin manager**"；`OPTIONAL_BUNDLES` 里 `:41` voice-input、`:42` cot-translation、`:48` forge **三行全在** |

另外 cot-translation 不在插件管理页主列表，在「更多」里：`index.js:106-117`（`:110`），
voice / forge 在 `EXTENSION_BUNDLES:97-101`。

**所以 GUI 侧的正确形状是「部件在、默认关闭」，不是「不画」。**

## 2. #63 cot-translation：真做了（❌ → 🟡）

### 宿主侧依赖：两条都是真实现，**不需要外部能力**

| 方法 | 官方 | 宿主 |
|---|---|---|
| `cotTranslation.limits` | `OFF/.../src/index.ts:46-53`，零参数 | **真** `HOST:internal/transport/handlers_cot_translation_limits_official.go:71-83`（`maxTextChars=100000`、`provider=deepseek-official`、`targetLanguage=auto`、`availableProviders=[deepseek-official, deepseek-account]`），路由 `HOST:internal/transport/server.go:568-571` |
| `cotTranslation.translate` | `OFF/.../src/index.ts:55-…`，结果**裸字符串** | **真** `HOST:internal/transport/handlers_media_market_official.go:572-700`：走本机模型网关（`internal/llm`，同一套 `DSHD_LLM_*`）；`google`/`bing` 回 501（不代理匿名端点）；模型空输出**报错而不回原文** |

### 本仓落点

| 层 | 文件 | 内容 |
|---|---|---|
| 契约快照 | `testdata/api.snapshot.json` 的 `cotTranslation.limits` / `cotTranslation.translate`（batch 19） | 参数/结果形状 + 两条"为什么"（`counts.total` 78 → 80） |
| 客户端 | `internal/apiclient/cot_translation.go` | 两条方法；`translate` 的 out 是 `*string`（写成结构体会静默解成零值） |
| 壳 | `cmd/app/cot_translation_wiring.go` | limits 同步读缓存 + 首问后台拉（失败保留上一次）；translate **只发起**，终态按 `jobID` 留在壳里，插件取走即清空；`cotTranslateTimeout=150s` **大于**宿主侧 90s |
| Actions | `pluginapi/plugin.go`（三条）、`pluginapi/cot_translation.go`（终态类型） | nil = 宿主没接 → 界面不画入口 |
| 界面 | `plugins/dsconversation/cot_translation.go` + `conversation.go:665-685` | 官方 `conversation.chat.reasoning.body` 座位：工具栏（`查看译文`/`查看原文` · `正在翻译思考内容` · `翻译暂不可用，已显示原文` + `重试`）+ 正文两态 |
| 测试 | `plugins/dsconversation/cot_translation_test.go`（逻辑）、`cot_translation_visual_test.go`（真 Viva 绘制+点击，pump 到条件成立）、`cmd/app/cot_translation_wiring_test.go`（真 HTTP） | 分段/拼接/状态机/失败/重试/入口可用性/取走即清空/参数形状 |

### 与官方的差异（都在下面登记，没有一条是"看着像"）

1. **官方 `run()` 是 async 顺序循环，本仓是帧驱动**（每帧最多一次迁移，同一时刻仍只有一个请求在飞、仍按段序、失败即停）。逐条对照 `OFF:src/client/translation.ts:96-113`。
2. **官方正文走 `renderFactorySlot('conversation.chat.reasoning.content')`（Markdown + 代码块工具条），本仓原文/译文都是 `ui.Wrap` 纯文本**。这是 `reasoning.go` 文件头那条既有的"刻意差异"（展开态原样显示落盘文本），译文跟着同一条画法，不另造一套 Markdown。
3. **设置页 HIDE**：官方第二处座位是 `plugins.bundle.config`（`OFF:src/client/mount.ts:105-111`）的 provider + targetLanguage 表单。本仓宿主**没有 `cot-translation` 命名空间的设置写入路径**（`grep -rn "cot-translation" ../DSH-GO/internal ../DSH-GO/cmd` 零命中；`limits` 固定回 `deepseek-official`/`auto`），所以那一页没有可写的真值——按 HIDE 处理，不画不能保存的表单。
4. `targetLanguage` 的 `auto` 在**客户端**解析成界面语言（`zh-CN`），与官方 `TranslationBody` 同一句；本仓界面只有中文（与 `cmd/app/plugin_manager.go:37` 同一口径）。
5. 待翻译文本用 `reasoningDisplayText`（去掉 `<signature>` 签名块）；展开态正文仍显示 `msg.Text` 原文——签名是加密块，翻它没有意义，也白占请求预算。

## 3. #64 forge：本轮不做，先要宿主 provider 线

**官方默认状态**：默认关闭（同 §1，`OPTIONAL_BUNDLES:48`、`EXPERIMENTAL_BUNDLES:125`）。

**规模**：85 文件 / 1094 KB（`src/client/**` 46 个 tsx + 39 个 ts，含 18 份 CSS module），
官方座位一处在右栏、一处在会话列表行，另需宿主 5 个插件
（`experimental-forge` / `forge-github` / `forge-session` / `worktree` / `tool-worktree`）。

**宿主真伪（20 个 `forgePanel.*`）**：

| 真 | 方法 | 宿主文件 |
|---|---|---|
| ✅ 3 | `branches` / `localDiff` / `localText` | `HOST:internal/transport/handlers_forge_git_official.go:50-56` + `internal/forge/git.go` |
| ✅ 6 | `requestWorktree` / `cancelWorktree` / `retryWorktree` / `stopCreation` / `restoreWorktree` / `continueInProject` | `HOST:handlers_forge_worktree_official.go:68-…` + `internal/worktree/worktree.go` |
| ✅ 1 | `changeRequestStates` | `HOST:handlers_forge_change_request_official.go:21` |
| ❌ 10 | `changeRequestChecks` / `changeRequestDiff` / `changeRequestText` / `dismiss` / `follow` / `markReady` / `merge` / `refresh` / `submitReview` / `track` | `HOST:handlers_forge_liveshare_official.go:212-380` 逐条结构化未实现 |

那 10 条缺的是**三种宿主能力**：GitHub provider + `gh` 登录态、forge inspection 调度器与面板状态流、
sandbox 模式与 checkout 边界。**面板本体的主体（PR 卡 / checks / 评审线程 / merge / 跟踪）全落在它们身上**
——现在做 GUI 只可能画出一堆「未实现」的灰块，那正是"为了凑账本数字做空壳"。
**结论：拆独立任务，宿主 forge provider 先行。**

## 4. #66 voice-input：HIDE + 登记缺什么

**官方默认状态**：默认关闭（`OPTIONAL_BUNDLES:41`、`EXTENSION_BUNDLES:98`）。

**规模**：15 文件 / 155 KB。

**宿主 STT 这次是真的**：`HOST:cmd/dshd/serve_speech.go:5-60` `newSpeechAPI()` 接
`internal/speech`（受管 C# helper 跑 Windows System.Speech，单次超时 + 可取消 + 进程 kill），
6 个方法（`catalog` / `follow` / `configure` / `prepare` / `cancelPreparation` / `transcribe`）全实现，
路由 `HOST:internal/transport/handlers_speech_official.go:86-93`。
（`handlers_media_market_official.go` 里那 4 条"没有语音识别引擎"的 501 是**旧口径残留**，
`IsOfficialSpeechMethod` 分支排在它之前，见 `HOST:internal/transport/server.go:536-539`。）

**缺的是客户端采集这一层**（三样，一样都不在本仓）：

| 缺什么 | 官方怎么做的 | 本仓为什么没有 |
|---|---|---|
| 麦克风枚举与录音 | `OFF:src/client/audio.ts:59-63` `navigator.mediaDevices.getUserMedia` + `:43,95` `MediaRecorder` | GUI 是 Go+Viva，没有 DOM/WebAudio；`grep -ln "WASAPI\|waveIn\|Microphone" cmd internal plugins` **零命中**；基线 `CGO_ENABLED=0`（`cmd/plugingen/main.go:156`），要 WASAPI 就是 COM syscall |
| 重采样到 16 kHz 单声道 | `OFF:src/client/audio.ts:152` `OfflineAudioContext(1, …, 16000)` | 需要自写重采样（宿主只认 16 kHz WAV） |
| WAV 编码 + base64 | 官方把结果塞进 `speech/transcribe` 的 `request.audioBase64` | 需要 WAV 头 + base64 编码这一层 |

**结论：HIDE**——界面本体（录音按钮 / 波形 / 麦克风选择 / 设置对话框）全部依赖上面三样，
一样都没有就画不出真东西；等音频采集层落地再连同 `readiness` 流一起做。
这是正确结论，不是失败：宿主那半边已经齐了，就差客户端采集这一层。
