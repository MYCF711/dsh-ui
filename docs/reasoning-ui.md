# 思考块：最近完成段落预览、签名清理与真实时长

> 对应 `docs/conversation-gap.md` 第 12 项（思考折叠、摘要与流式预览）。
> 本文件只覆盖 REASONING-UI 这一项；没有改宿主、契约、共享客户端或其他插件。

## 0. 结论

收起行从「`▸ 思考 · N 字`」变成「`▸ 思考 · 想了 N 秒 · 最近完成段落第一行`」：

- 预览取**最近一个已经写完的段落**的第一行，不再是原始文本开头 120 字；
- `<signature>…</signature>` 只从预览里清掉，展开仍是 `msg.Text` 原文；
- 「想了 N 秒」只在 `MessageMetadata.Time` / 同 Turn `TurnEvent.Time` 给出**起止两端**真值时出现；
  缺一端、时间倒退、不足 1 秒、或这一段仍在跑 → 整段隐藏，不读墙钟；
- reasoning-only assistant 仍走 thinking 分支，收起行不会因为预览为空而消失。

## 1. 基线与环境

| 项 | 值 |
|---|---|
| 隔离副本 | `C:\Users\Administrator\AppData\Local\Temp\dsh-ui-reasoning-ui-isolated` |
| base commit | `b396f3ad9b194a53c5c11b9b0edc3928d7d3fe18`（`git rev-parse HEAD`） |
| 工作树附加导出 | 未提交的共享改动仍在：`plugins/dsconversation/file_refs.go`、`file_refs_test.go`、`cmd/app/main.go` 的 FILE-REFS 接线、`docs/file-refs.md` 等；本项只在其上叠加 |
| Go | `go1.27.3 windows/amd64`，`CGO_ENABLED=0` |
| 采样时间 | 2026-11-04 14:13–14:20 (+08:00) |
| 允许改动 | `plugins/dsconversation/**` 与新建 `docs/reasoning-ui.md` |

改动文件（4 个代码/文档 + 4 张证据图，未 commit、未 push）：

```
M plugins/dsconversation/conversation.go
A plugins/dsconversation/reasoning.go
A plugins/dsconversation/reasoning_test.go
A plugins/dsconversation/reasoning_visual_test.go
A docs/reasoning-ui.md
A plugins/dsconversation/testdata/reasoning-ui/*.png（4 张）
```

## 2. 判据与实现位置

### 2.1 折叠预览 = 最近完成段落的第一行

[`reasoning.go:78`](../plugins/dsconversation/reasoning.go#L78) `reasoningPreview` →
[`reasoning.go:98`](../plugins/dsconversation/reasoning.go#L98) `latestCompletedParagraphFirstLine`
→ [`reasoning.go:123`](../plugins/dsconversation/reasoning.go#L123) `nextParagraphBreak`。

判据与官方 `dsh-client-ui-chat` 的 `lib/types/client/chat/ReasoningRow.js`
（`latestCompletedParagraphFirstLine`，`tmp/official/dsh-client-ui-chat/client.js:6153-6176`）同一套：

- 段落分隔 = 一个换行 + 至少一个只含空白的行 + 换行（等价 `/\r?\n(?:[\t ]*\r?\n)+/`）；
- 段落里出现第二个换行（包括分隔符自己的换行）→ 该段第一行已写完，可作预览；
- 最后一段只有一行（还在写）→ 保留上一个完成段落的第一行；
- 预览去掉 `**` 标记；没有可用完成段落（单行文本）时退回第一行。

`reasoningPreviewRunes = 120` **只做选定之后的兜底截断**，不参与选段——旧的「固定开头 120 字」被选段规则取代。

### 2.2 `<signature>` 只从预览消失

[`reasoning.go:40`](../plugins/dsconversation/reasoning.go#L40) `reasoningDisplayText`：
清理文本里所有 `<signature>…</signature>`；未闭合的 `<signature>` 视为「其后全是签名」；
流式刚到半个标签（`<sig`、`<signature`）时返回空。清理后去前导空行。

展开正文走 [`reasoning.go:72`](../plugins/dsconversation/reasoning.go#L72) `reasoningBodyText`，
**原样返回 `msg.Text`**，没有 `Replace`。[`conversation.go:551`](../plugins/dsconversation/conversation.go#L551)
的展开盒子画它就是原文。

> 与官方的一处刻意差异：官方把清理后的文本也交给展开态 Body Slot；本轮验收明确要求
> 「展开全文仍完整真实」，所以清理只进预览。差异只影响展开态是否显示签名块。

### 2.3 「想了 N 秒」只用真值

[`reasoning.go:181`](../plugins/dsconversation/reasoning.go#L181) `thinkingElapsed`：

| 端点 | 来源 | 缺失时 |
|---|---|---|
| 起点 | 前一条消息的 `MessageMetadata.Time`（输入边界：上一条 user/tool 结果，或上一条 assistant 记录时刻） | 隐藏 |
| 终点 | 本思考块自己的 `MessageMetadata.Time` | 退回同一 `TurnID` 的终局 `TurnEvent.Time`（done/error/stopped）；仍无 → 隐藏 |

附加规则：

- 两条消息都带真 `TurnID` 且不同 → 隐藏（不把两次等待拼在一起）；
- `end <= start`、`round(Δms/1000) < 1` → 隐藏（不画「想了 0 秒」）；
- 运行中的尾块由 [`reasoning.go:216`](../plugins/dsconversation/reasoning.go#L216) `thinkingElapsedFor`
  直接隐藏（`Busy && index == len(Msgs)-1`），即使两端都有历史真值；
- `reasoning.go` **没有 `time.Now()`**：`grep -n "time\." plugins/dsconversation/reasoning.go`
  只命中两行注释（14:19 实测）。

### 2.4 绘制接线

[`conversation.go:507`](../plugins/dsconversation/conversation.go#L507) `drawThinking` 新增 `index` 形参
（[`conversation.go:429`](../plugins/dsconversation/conversation.go#L429) 从 `drawMessage` 传原下标），
收起行按「标题 · 真值时长 · 最近完成段落」拼一行，`Ellipsis()` 只截尾部：

```go
label := "思考"
if seconds, ok := thinkingElapsedFor(env.Data, index); ok {
    label += " · 想了 " + itoa(seconds) + " 秒"
}
if preview := reasoningPreview(msg.Text); preview != "" {
    label += " · " + preview
} else {
    label += " · " + humanRunes(len([]rune(msg.Text))) + " 字"
}
```

预览为空（整段只有签名）时退回真实字数，**行本身没有 return 分支**，所以
reasoning-only assistant 不会消失。

## 3. 红：旧实现上必红

为了「先写必红测试」，先落一个**旧行为可编译桩**（预览固定取开头 120 rune、不清理签名、
时长恒未知、展开画原文），再写测试。命令与结果（14:14，退出码 1）：

```
$ CGO_ENABLED=0 go test ./plugins/dsconversation/ -run 'TestReasoning|TestThinking' -count=1
--- FAIL: TestReasoningPreviewUsesLatestCompletedParagraph
    折叠预览必须取最近完成段落的第一行，实际 "开头第一段：……\n\n中间一段：……\n\n最近完成段落第一行\n最近段落还在写"
--- FAIL: TestReasoningPreviewStripsSignature
    清理签名后预览应为最近完成段落，实际 "<signature>c2lnbmF0dXJlLWJsb2Nr</signature>\n第一段：……"
--- FAIL: TestReasoningPreviewSignatureShapes/整段只有签名
    reasoningPreview("<signature>abc</signature>") = "<signature>abc</signature>"，want ""
--- FAIL: TestReasoningPreviewDropsMarkdownBold
    预览应去掉 ** 标记，实际 "开头。\n\n**最近完成段落**第一行\n第二行还在写"
--- FAIL: TestThinkingElapsedUsesRealTimestamps
    真实时间戳 7s 应显示 7 秒，实际 (0,false)
--- FAIL: TestThinkingElapsedTurnEventFallback
    同 Turn 终局事件应给出 5 秒，实际 (0,false)
--- FAIL: TestReasoningUIPaintHitTestEvidence/1182x832
    真实起止时间戳没有显示「想了 7 秒」：["先检查一下仓库，再看流式输出。" "▸" "思考 · 110 字" …]
--- FAIL: TestReasoningUIHidesUnknownAndRunningDurations/时间真值缺失
    未知时长只许隐藏时长，预览仍要在：["先检查一下仓库，再看流式输出。" "▸" "思考 · 110 字" …]
--- FAIL: TestReasoningOnlyAssistantStillPaints
    reasoning-only 的真实时长没显示：["开始。" "▸" "思考 · 16 字" …]
FAIL
```

（旧行只有 `▸ 思考 · 110 字`：既没有预览，也没有时长——两条验收点都在这一跑里红。）

## 4. 绿：实现后

以下命令在 14:16–14:19 实测；`gofmt -w` 补上三个新文件的结尾换行后，14:21 又完整复跑
`go test ./...` / `go vet ./...` / `go build ./...`，仍然全 exit 0（`go vet ./...` 与
`go build ./...` 用不带管道的写法单独取过退出码）。

```
$ CGO_ENABLED=0 go test ./plugins/dsconversation/ -run 'TestReasoning|TestThinking' -count=1
ok  github.com/MYCF711/dsh-ui/plugins/dsconversation  0.392s        # 退出码 0

$ CGO_ENABLED=0 go vet ./plugins/dsconversation/
（无输出，退出码 0）

$ CGO_ENABLED=0 go test ./plugins/dsconversation/ -count=1
ok  github.com/MYCF711/dsh-ui/plugins/dsconversation  1.206s        # 退出码 0

$ CGO_ENABLED=0 go test ./... -count=1
ok cmd/app 1.138s / ok cmd/mockd 9.855s / ok internal/* / ok pluginapi 0.073s
ok plugins/*（含 dsconversation 1.869s、dscomposer、dstopbar、dstool…）  # 退出码 0

$ CGO_ENABLED=0 go vet ./...       # 退出码 0
$ CGO_ENABLED=0 go build ./...     # 退出码 0
```

新增测试（12 个顶层用例，含子用例）：

| 文件 | 用例 | 断言 |
|---|---|---|
| `reasoning_test.go` | `TestReasoningPreviewUsesLatestCompletedParagraph` | 多段文本预览 = 最后完成段落首行，且不含开头段落 |
| | `TestReasoningPreviewStripsSignature` | 预览无 `<signature>`；`reasoningBodyText` 与原始文本逐字相同 |
| | `TestReasoningPreviewSignatureShapes` | 整段签名 / 未闭合 / 签名在中间 / 无签名 / 单行 |
| | `TestReasoningPreviewDropsMarkdownBold` | `**` 去掉，段落选择不变 |
| | `TestThinkingElapsedUsesRealTimestamps` | 7s、12s；历史时间戳只算差值 |
| | `TestThinkingElapsedHiddenWhenUnknown` | 无元数据 / 只有起点 / 只有终点 / 时间倒退 / 不足 1 秒 / 第一条消息 → 全隐藏 |
| | `TestThinkingElapsedTurnEventFallback` | 同 Turn 终局事件 5s；TurnID 不同 → 隐藏 |
| | `TestThinkingElapsedHiddenWhileRunning` | Busy 尾块隐藏，收尾后 7s |
| | `TestReasoningOnlyThinkingKeepsThinkingBranch` | reasoning-only → thinking 分支；签名-only 预览为空但仍合法 |
| `reasoning_visual_test.go` | `TestReasoningUIPaintHitTestEvidence`（1182×832 / 1920×1032） | 真实 Paint + HitTest：收起→点击展开→展开后全文 |
| | `TestReasoningUIHidesUnknownAndRunningDurations`（两尺寸） | 无真值 / 运行中：不画「想了 」但预览仍在 |
| | `TestReasoningOnlyAssistantStillPaints` | reasoning-only 真实绘制：思考行 + 想了 4 秒 + 折叠预览 |

## 5. 视觉与命中证据（两尺寸，展开前后）

复现命令（14:16）：

```
$ REASONING_CAPTURE_DIR="$PWD/plugins/dsconversation/testdata/reasoning-ui" \
  CGO_ENABLED=0 go test ./plugins/dsconversation/ \
  -run 'TestReasoningUIPaintHitTestEvidence|TestReasoningOnlyAssistantStillPaints' -count=1 -v
--- PASS: TestReasoningUIPaintHitTestEvidence (0.26s)
    --- PASS: .../1182x832 (0.15s)
    --- PASS: .../1920x1032 (0.11s)
--- PASS: TestReasoningOnlyAssistantStillPaints (0.00s)
```

> **2026-11-04 补记（入库时）**：这四张图原来**一条都不在仓库里** —— 文档写着尺寸和
> 哈希，文件却从没入库，链接全是坏的（`scripts/doc-links.py` 就是这轮被这件事催出来的）。
> 补跑 `REASONING_CAPTURE_DIR=testdata/reasoning-ui go test -run Reasoning` 重新生成并入库：
> **字节数与文档完全一致**（26 029 / 35 870 / 47 077 / 56 552），三张哈希也一致；
> 只有 1920 宽展开态那张与原文尾部不同（原文 `…f30c9c9f`，本机连跑两次都是 `…f30c9cf6`），
> 已按本机实测更新。结论：这四张**在本机逐字节可复现**，但那一张不是跨机器可复现。

| 证据 | 尺寸 | 字节 | sha256（前 16 位） | 画面 |
|---|---|---|---|---|
| [reasoning-collapsed-1182x832.png](../plugins/dsconversation/testdata/reasoning-ui/reasoning-collapsed-1182x832.png) | 1182×832 | 26 029 | `65a33a6bd0bc8552…` | 收起：`▸ 思考 · 想了 7 秒 · 最近完成段落第一行`；无开头段落、无签名 |
| [reasoning-expanded-1182x832.png](../plugins/dsconversation/testdata/reasoning-ui/reasoning-expanded-1182x832.png) | 1182×832 | 35 870 | `9df2257c1c1c5c7c…` | 展开：签名原文 + 开头/中间/最后一段全部可见 |
| [reasoning-collapsed-1920x1032.png](../plugins/dsconversation/testdata/reasoning-ui/reasoning-collapsed-1920x1032.png) | 1920×1032 | 47 077 | `19c56715f30c9cf6…` | 同上，宽屏 |
| [reasoning-expanded-1920x1032.png](../plugins/dsconversation/testdata/reasoning-ui/reasoning-expanded-1920x1032.png) | 1920×1032 | 56 552 | `1e7e5cffbe614f6d…` | 同上，宽屏 |

命中路径（不是内部布尔，是真实 HitTest）：

1. `Paint` 产出命中区 `id = tree.ID(feat, "think-"+MessageKey)`；
2. 在 `session.Snapshot().Hits` 里查出该 `id`，取 `Bounds` 中心；
3. `session.Press/Release` 中心点 → `Pump` 两次；
4. 断言 `p.expanded[key] == true`，且展开画面出现 `<signature>c2lnbmF0dXJlLWJsb2Nr</signature>`、
   开头段落、末尾「最近段落还在写」。

合成数据：用户消息（Time `1762222444000`）→ 思考块（`+7000ms`）→ 助手正文（`+9000ms`），
思考原文 = 签名 + 三段，最后一段两行（首行已完成、第二行仍在写）。

## 6. 不回退项（本轮实测）

| 关注点 | 证据 |
|---|---|
| MDSTREAM | `TestStreamingMarkdownReallyDrawsHeadingListAndInlineCode`、`TestStreamingMarkdownOpenFenceStaysCodeAndKeepsText`、`TestStreamingMarkdownReusesFinalMessageCacheSlot`、`TestStreamingMarkdownLayoutCacheBoundsRepeatedFrames`、`TestStreamingMarkdownHeadlessFrames` 全 PASS |
| WP-C1 | `TestWPC1MessageAndTerminalTruthIsVisible`、`TestWPC1ToolRenderIDIsStableAndSessionLocal`、`TestWPC1ExplicitFileLinkRewriteDoesNotGuessPlainTextOrCode` 全 PASS |
| WP-C2 | `TestWPC2TurnNavigationReadingPositionAndPrependEvidence`（1182×832 / 1920×1032）、`TestWPC2LoadedTurnShellFooterAndMissingPrependTruth` PASS |
| WP-C3 | `TestWPC3RealStatusExpansionAndInterleavedEvidence`（两尺寸）、`TestWPC3ToolStatusOnlyComesFromTruth`、`TestWPC3InterleavedIdenticalCallsKeepIdentityResultAndStatus` PASS |
| FILE-REFS | `TestFileRefsKnownRelativeAndAbsolutePathsBecomeFileLinks`、`TestFileRefsMissingCwdDoesNotGuessRelativePath`、`TestFileRefsMarkdownCacheReusesFramesAndTracksCWD`、`TestFileRefsUnknownTextURLsAndCodeAreNeverPromotedToFiles`、`cmd/app: TestFileRefsHeadlessRealClickAndTwoSizeEvidence` PASS |
| 审批原 ID | `dscomposer: TestApproveForwardsOriginalIDs`、`TestRejectForwardsOriginalIDs`、`TestWeirdIDsAreNotRewritten`、`TestApprovalButtonsPreserveOriginalIDsThroughLayout`；`cmd/app: TestApprovalOptionsConversion`、`TestTimeoutClearsTheStalePrompt`、`TestCancelBehavesLikeTimeout` PASS |
| 工具卡 | `TestToolCardCapture` PASS |

本项只改了 `drawThinking` 的收起行文案/展开正文来源与 `drawMessage` 的调用形参；
工具卡、Markdown 槽位、Turn 页眉页脚、审批绘制路径均未触碰。

## 7. 未验项

1. **真实宿主端到端未跑**：本轮是隔离副本的插件层 + viva headless 真实 Paint/HitTest，
   没有起 dshd、没有 `/connect`、没有真实会话。`session.load` 的 reasoning-only → thinking
   转换在 `cmd/app`（他人文件，本轮禁改），这里用 `apiclient.ReasoningOnlyThinking` 单测
   + 插件真实绘制证明「不消失」，不等于宿主整链复验。
2. **运行中真实思考的时长**：实时消息当前**不带** `MessageMetadata.Time`（壳只在 load 时填），
   所以运行中恒隐藏；真实会话里「想了 N 秒」只会在切回/重载、且两条消息都有真时间后出现。
   真机上的体感（是否总是延迟到重载才显示）未验。
3. **TurnEvent 终点回退**：只在合成数据上验过。当前壳只记「本进程观察到的终局事件」，
   旧历史没有事件，因此真机上这条回退可能永不命中；不命中时按设计隐藏。
4. **官方视觉细节未做**：sticky header（`.VJlF5q_root[data-expanded]`）、
   `data-streaming` 的 preview mask/shimmer、官方 13px 摘要字色——本项只做内容判据与真实时长，
   样式对拍留给外观线。
5. **英文文案**：界面没有 i18n 层，只实现中文「想了 N 秒」；英文 `Thought for Ns` 未做。
6. **签名其它形态**：只认文本里的 `<signature>…</signature>`；宿主若改为单独字段/结构化块
   （或 Anthropic `redacted_thinking` 形状），需要另立判据。
7. **真实 OS 窗口**：截图来自 viva headless renderer（真实像素、真实命中区），
   不是用户屏幕；OS 级 DPI/字体/窗口装饰未验。

## 8. 补丁与证据留存

- 文本补丁：`/c/Users/Administrator/AppData/Local/Temp/reasoning-ui-final.patch`
  （仅 `conversation.go` 的相对基线 hunk + 4 个新文件；二进制 PNG 不进文本补丁）。
- 证据图副本：`/c/Users/Administrator/AppData/Local/Temp/reasoning-ui-evidence/`
  （与 `plugins/dsconversation/testdata/reasoning-ui/` 同一批文件）。
- 复跑证据图命令见 §5；不带 `REASONING_CAPTURE_DIR` 时测试仍会实画到 `t.TempDir()`。
