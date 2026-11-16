# #59 `dsh-client-ui-trajectory` · rc.2 完整账本证据

> 官方包：`@deepseek-ai/dsh-client-ui-trajectory@0.2.2-rc.2`。
> 本文的 `src/client/...` 均指官方包内路径；实现路径均指本仓 `plugins/dsrightbar/`。
> 数据源是 `pluginapi.Snapshot.Frames` 的原始 Session journal，不是从 `Msgs` 摘要反推。

## 1. 原账本六项剩余差异的收口

| 原差异 | 官方依据 | 本轮实现 |
|---|---|---|
| 只有 role=tool，缺完整 record | `trajectory-record.ts:8-107` 定义 `system/user/context/compacted/message/tool/subtool` 及详情字段；`trajectory-message-definitions.ts:78-195` 区分 user/context/steering；`trajectory-compaction-definition.ts:40-132` 聚合 compaction/session boundary | `trajectory_ledger_model.go:190` 直接折叠 `system/message`、`request/header`、`user/message`、`developer/message`、assistant step/stream/attempt/final、tool/result、PTC dispatch、compaction、inbox splice 与 turn/step/session 边界。七类 record 的 input/output/thinking/source/options/schema/prompt/tools/raw 都保留真实载荷；request-only step 后续由 assistant 原位填实；steering 归前一 Turn，`session/end-seed` 将仍在运行的 compaction 合成为 interrupted。 |
| assistant calls、递归 subcalls、三套全局折叠 | `trajectory-tool-definition.ts:123-169,230-275` 是根调用 + PTC 树，带环/深度防护；`TrajectoryView.tsx:141-159,433-536` 是三套独立 Set 与全局切换；`TrajectoryTable.tsx:618-744` 是 summary 语义 | `trajectory_ledger_model.go:700,815` 把 assistant 的 tool-call 块与根/子工具按 callId 对齐，深度优先紧跟父节点，最大深度 256；child→parent→root 乱序会 adopt 重排，父永久缺失也保留真实 subtool，不生成空 root。`trajectory_ledger_view.go:669` 分别实现 Turn / Assistant / Tool 折叠；`drawLedgerToolbar` 的「轮次 / 调用 / 子调用」只切换当前可折叠 ID，搜索绕过折叠但不清状态。 |
| duration / actual-time、TTFT/解码段、缩放拖选 timeline | `timeline.ts:8-185` 定义 sequence/time/duration/actual 与三 lane；`TrajectoryTimeline.tsx:51-82,295-386,432-568` 定义计时分解、滚轮公式和拖选 | `trajectory_ledger_view.go:262,358` 实现四种投影、Input/Model/Tools 三轨、真实 startedAt/duration、assistant TTFT/解码分段、搜索变暗、range 闭区间聚焦、左键拖选、右键平移、选择记录自动回到 viewport。`HandleScroll`（`:1741`）使用 `exp(deltaY*0.0015)`、sequence 4 项 / timed 20ms 下限和 99.9% 恢复全览；`HandleKey`（`:1776`）仅在有 range 时消费 Esc。时长偏好写 `dsh.trajectory.duration`，actualTime 为每会话临时状态。 |
| request/options/usage/timing/schema/diff 多页 Inspector | `TrajectoryTable.tsx:178-244,798-902,993-1028,1378-1493,1661-1892,3002-3600` | `trajectory_ledger_view.go:1135,1188` 实现 request 与 record 两种互斥选择及分栏/窄屏 overlay。request 有概述/选项/用量/计时；system 有差异/系统提示词/工具；compacted 有概述/原始输出；user/context/assistant 有概述/预览/原始内容/来源；tool/subtool 有概述/参数或代码/结果/Schema/计时。usage 显示本次与累计、缓存/推理分项；system+tools 都生成真实 diff；run_code 显示 description、行号代码和原始 JSON。 |
| 未接 LoadOlder，仍限当前窗口与最近 80 条 | `TrajectoryView.tsx:35-36,160-211` 的 resident 50 条分页与宿主 loadOlder；`TrajectoryTable.tsx:48-53,2600-2655` 的加载入口 | `trajectory_ledger_view.go:812` 使用 resident 50 条增页；仍有本地记录时不发请求，用尽后才调用已经存在的 `Actions.LoadOlder`。loading/error/at-start 均使用 `Snapshot.History` 真值。完整 Frames 路径不再经过 `maxEntries=80`；旧 Msgs-only 回退仍保留 80 条边界，不影响老宿主。 |
| 旧三栏靠视口偏移桥 | 官方只注册 `conversation.view` 的 trajectory 条目；本仓槽证据见 `official-view-tabs-evidence-rc2.md` | `rightbar.go:97-186` 删除 `view-chat-gate`、`trajectory-view` 两个 `main.body` painter 及 `ViewportOffset` 临时快照适配，只保留 `conversation.view` / `trajectory` / `order:10` 和右栏 tab 两个正式座位。`dsshell/shell.go:782-791` 在有名录时直接 keyed 派发；`dsconversation/official_transcript_wiring.go:96-106` 让 Msgs-only chat 也从正式座位回退，`conversation.go:803-815` 阻止非 chat 帧覆画已失效的旧 Markdown Box。 |

## 2. 搜索、虚拟化与真值边界

- 搜索按官方 `trajectory-search-index.ts:119-132`：trim、空白分词、大小写不敏感、逐词 AND；索引由 `trajectory_ledger_model.go:1392` 汇总 Turn/Step/kind/text/input/output/thinking/schema/source/options/prompt/tools/raw/callId。匹配时只保留命中行，timeline 非匹配 span 变暗。
- 完整账本按估计行高做可见区 + overscan 布局，未可见行用同高 Slot 补偿；搜索、折叠与选择身份均使用稳定 record ID，prepend 不换 ID。
- 未知 startedAt、firstToken、completedAt、usage、schema 不补零：列表时长为 `—`，Inspector 显示官方「不可用 / 未记录 / 未报告用量 / Schema 不可用」；usage 只画 provider 实际给出的桶，不把 nil 写成 0。
- requestOnly 是 9px、可打开 request Inspector 的分隔锚点，不计成 assistant 内容、不画 50px 假消息；partial assistant 与未结束工具显示等待中，成功空结果仍为已完成并保留 Result 页的「无输出」。
- AUTH failure 的 message/reason 不进入 UI 模型或搜索索引；Inspector 显示「API 密钥无效」。工具 error 无正文时可显示非敏感的 `name: code`。
- Raw result 是官方独立 live watch 服务，不是 v4 history 字段（官方 `index.ts:96-126`）。本仓没有该 watch 通道，因此结果页保留真实 Rendered result，Raw result 明确显示「原始结果仅在实时工具调用中可用」，不伪造实时订阅或历史缓存。
- 图片 gallery 按官方 `trajectory-contract.ts:96-104` 属于可替换 slot；未注册授权 loader 时省略图片，原始 source blocks 与文件元数据仍留在详情 JSON 中。

## 3. 测试与命令

- 模型测试 `trajectory_ledger_model_test.go`：七类 record、assistant calls、PTC 递归/环/深度/乱序/缺父、compaction/session boundary、inbox steering/canceled、request options/schema、system diff、五桶 usage 与累计、流式计时、retry 清块但保首 token、AUTH 消毒、prepend 稳定 ID。
- 交互测试 `trajectory_ledger_view_test.go`：真实 Viva headless Paint/HitTest/TextInput，pump 到条件成立后断言工具栏、request Options/Usage、全局折叠、AND 搜索、resident→host LoadOlder；另有四模式/滚轮/Esc、双区结果、run_code、requestOnly 9px 锚点、running/settled 空态、缺桶不补零、座位与稳定身份测试。`dstopbar/frame_interaction_test.go` 保持原有点击断言，fixture 改走与 `cmd/app` 相同的 keyed 派发，覆盖 chat→trajectory→chat 与逐会话记忆。
- 定向命令：`CGO_ENABLED=0 go test ./plugins/dsrightbar -count=1`，本轮 EXIT=0。
- 全仓门禁：`CGO_ENABLED=0 go build ./...` EXIT=0；`go vet ./...` EXIT=0；`CGO_ENABLED=0 go test ./... -count=1` EXIT=0；独立 gofmt 检查 EXIT=0；`parity-count.py --check`、`doc-links.py`、`scripts-index.py` 全 PASS。日志在 `tmp/GO8-RC2/_tr/delivery-*`。
- 独立只读复核最终结论：assistant/attempt、reasoning-only、run_code description 三处均闭合，未再发现 blocker。
- 本任务没有新增 Transport 方法，`testdata/api.snapshot.json` 无需变更。
