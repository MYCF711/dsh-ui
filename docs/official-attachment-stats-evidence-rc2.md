# #6 attachment / #65 stats 证据（rc.2）

> 本轮把 GUI 账本里仅剩的两条「有真实落点、可继续做实」的 PARTIAL 推进到 IMPLEMENTED：
> `dsh-client-ui-attachment`（#6）与 `dsh-experimental-client-ui-stats`（#65）。
> 剩余差异在 §3 / §5 逐条列出，不藏。未 commit / 未 push。

## 1. #6 官方要什么

官方 attachment 包不是「一个上传按钮」，而是三件事：

1. **输入条上的附件 rail**：拖入/选择文件 → 卡片三态（uploading / ready / error）→ 可移除、可翻页
   （`OFF:dsh-client-ui-attachment/src/client/ComposerAttachments.tsx:62-120`、
   `FileCard.tsx:18-78`）。
2. **上传到会话作用域的暂存区**：浏览器字节 → `fileUploads.upload` → `{receiptId,file}`
   （`OFF:dsh-client-file-upload/src/client/runtime.ts:192-212`、`fileUploadValue` 校验 `:355-360`）。
3. **把回执带进 `session.prompt` 的 `content`**：附件在前、文本最后
   （`OFF:dsh-client-ui-conversation/src/client/service.ts:247-303`，顺序在 `:302`）。

宿主侧已经合并官方通道（`HOST:cmd/dshd/attachments_official_prompt.go` 的
`NormalizeOfficialContent`、`HOST:cmd/dshd/serve_agent_api.go:1417-1495` 的
`fileUploads.upload`、`HOST:internal/transport/handlers_agent_official.go:801-818`
的 `OfficialFileUploadRequest/Value`）。本仓此前缺的是**GUI 发送链**：`dispatchSend`
只调 `m.cli.Prompt(session,text)`，`AttachReady` 全仓无调用点。

## 2. #6 本轮落点（代码 + 测试）

| 层 | 落点 | 说明 |
|---|---|---|
| 契约客户端 | `internal/apiclient/prompt_parts.go:36-116` | `FileUploadReceipt` / `UploadedFile` / `FileUploadsUpload` / `PromptPart` / `PromptParts`；平铺 `{sessionId,data(base64),name}`，`PromptParts` 按官方顺序附件在前、文本最后 |
| rail → 发送 | `plugins/dsattachment/rail.go:574-730` | `SendBatch` + `PreparePrompt`：只把 **全部 ready** 的卡换成 `{type:'file',receiptId}`；任何 uploading/error 卡都拒绝整批（官方 `uploadFor` 同判据）；成功后 `Commit` 只删快照里那几张 |
| 壳侧上传 | `cmd/app/official_panels_wiring.go:68-78` | `attachSink.Upload`：先 `attachment.get` 取回宿主已暂存的字节（不重新读路径），再 `fileUploads.upload(sessionId,data,name)` |
| 发送链 | `cmd/app/wpc1_send.go:64-91` | `dispatchSend` 先 `PreparePrompt`（60s），再 `PromptParts`（5s）；失败保留草稿与 rail |
| 附件-only | `cmd/app/wpc1_send.go:34-40`、`plugins/dsattachment/rail.go:624-631` | `HasItems` 让空文本在 rail 有附件时也能发送（官方 `text===''` 时 content 只放附件） |
| 历史渲染 | `plugins/dsconversation/official_transcript_projection.go:585-603` | durable `type:"attachment"` 块投影成文件卡/图片瓦片；0 字节保留为 `0 B` |
| 测试 | `internal/apiclient/prompt_parts_test.go`、`cmd/app/official_panels_wiring_test.go`、`plugins/dsattachment/rail_test.go:` `TestPreparePrompt*`、`plugins/dsconversation/official_transcript_projection_test.go:` `TestUserAttachmentBlocksRenderAsFileAndImage` | 覆盖平铺/回执/顺序/失败卡/无会话/历史块 |

## 3. #6 与官方的差异（如实登记）

- **图片像素没有解码路径**：官方 rail 与消息图片是真实缩略图 + 灯箱；本仓 rail 只画官方
  240×64 文件卡（图片也按文件卡展示），消息区图片瓦片是 64×64 占位槽。原因：GUI 没有
  图片解码/缓存层（Viva `ui.Image` 需要 `gfx.Bitmap`），本轮不引入第二个图片管线。
- 官方 `FileCard` 的 SVG 文件类型图标用扩展名文字顶替（仓规：无素材不手画 SVG）。
- 官方 `attachment.dropDesc`（条数/单张上限提示）没有画：宿主没有「条数上限」事实，
  8 MiB 是总量上限，语义不同，不编数字。

## 4. #65 官方要什么

官方实验包只注册一个 `conversation.view` 条目：

- `id:'stats'`、`order:40`（`OFF:dsh-experimental-client-ui-stats/src/client/index.ts:74-81`）。
- 页面是一张逐 turn / 逐 step 的性能表：11 列
  `scope, prompt(触发), did(执行内容), context, output, cache, ttft, model, tps, tool, time`
  （`StatsView.tsx:233-245`；列宽 `stats.module.css:49-59`）。
- 折叠来自原始事件：`step/start` → `assistant/text|reasoning`（首 token）→
  `assistant/message`（usage）→ `tool/call`+`tool/result`（工具耗时）→ `step/end`；
  turn 由 `turn/start`/`turn/end` 定界（`stats-fold.ts` 的 `foldStepState`/`stepMetrics`）。
- 指标公式：`context = input+cacheRead+cacheWrite`、`cache = cacheRead/context`、
  `ttft = firstToken-start`、`model = message-start`、`tps = output/decodeSeconds`
  （`stats-usage.ts`、`stats-format.ts`）。

## 5. #65 本轮落点与差异

| 层 | 落点 | 说明 |
|---|---|---|
| 视图注册 | `plugins/dsstatusbar/statusbar.go:129-130` | `conversation.view` 的 `{Key: stats, Label: 统计, Order: 40}` |
| 折叠 | `plugins/dsstatusbar/stats_view.go:197-305` | 原始帧 → turn/step 模型；turn/step 缺失按上一帧兜底；乱序按 step 号稳定排序 |
| 指标 | `plugins/dsstatusbar/stats_view.go:400-585` | `measureStep` / `measureTurn` 逐条对齐官方公式；缺桶就是 `—`，不写 0 |
| 页面 | `plugins/dsstatusbar/stats_view.go:705-1070` | 11 列官方表头、最新一轮默认展开、点轮行展开更早轮、总用时条（14px 轮 / 6px step）、全量/窗口合计 |
| 测试 | `plugins/dsstatusbar/stats_view_test.go` | 帧折叠数值（context=400、TTFT=200、model=500、TPS≈166.7、tool=100、wall=800、cache=75%）、官方表头、最新轮展开、点击展开、空窗口不编数字、view 注册 order/label |

**剩余差异**（未做，不冒充）：

- compaction 事务行与非 step 事件行（system-prompt / tool-set / retry / approval / prune…）没画；
  官方 `StatsTable` 的这些行需要 `stats-compaction.ts` / `stats-event.ts` 的两套折叠，本轮只做默认行。
- 官方工具栏三个开关（context delta / show events / context basis）没有；固定官方默认
  `contextMode=latest`、事件行隐藏。
- 官方「加载更早」分页没有；本仓按当前已加载帧窗口统计，合计行会如实标「本窗口合计」。
- 官方 Code-Mode 内层 dispatch 合并没做：`tool/ptc-dispatch` 帧不另开行。

## 6. 门禁（本轮实跑）

```bash
CGO_ENABLED=0 go build ./...                 # EXIT=0
CGO_ENABLED=0 go vet ./...                   # EXIT=0
CGO_ENABLED=0 go test ./... -count=1         # EXIT=0，41 包 ok、0 FAIL
python scripts/parity-count.py --check       # PASS（66 个：✅44 / 🟡5 / ❌17）
python scripts/doc-links.py --check          # PASS
python scripts/scripts-index.py --check      # PASS
```

未 commit / 未 push；工作树只含本任务的文件。
