# MSGID：Transport 水位与 v4 内容身份分离

> 范围：`internal/apiclient/**`。基线任务为 UI `f007fad`；实际开工时 HEAD 是其后代 `f298be8`，未 reset、未覆盖其他工作区改动。本文只记录适配层能保证的行为，不把缺少宿主真值的分支写成已闭环。

## 1. 两个序号域，不再互用

| 值 | 唯一语义 | 可以用于 | 不得用于 |
|---|---|---|---|
| `Event.Seq` / WS 信封 `seq` | 每会话、每进程的 Transport journal 序号 | 订阅水位、断号检测、`session.history` 补洞、重连 replay | `Message.Seq`、消息 upsert、tool 配对、跨 load/WS 去重 |
| `Message.Seq` | 旧 `session.load` 投影及流式 data 中的 v4 seq | 内容身份与旧调用方兼容 | Transport 水位 |
| `Message.V4Seq` | 可空指针形式的原始 v4 seq | 区分“字段缺失”和合法的 `seq=0`，保留超出旧 `int` 可表达范围的原值 | 本地编号 |

宿主证据：`cmd/dshd/transport_source.go:223,230-243` 把 `ev.Seq` 写入 load 投影；`internal/transport/journal.go:62-67` 在 WS/history 帧入账时另发 Transport seq；`docs/transport-contract.md:81` 明确二者不同。

`Stream.Seq()` 现在也只返回内容 v4 seq；未知时返回 0。需要区分未知与真实零值时，读取 `Messages()[].V4Seq != nil`。`Event.Seq` 本身及其公开字段未改，壳的订阅/水位代码可继续原样使用。

## 2. data 中识别并保留的字段

字符串原样复制，不加前缀、不生成“看起来像宿主 ID”的新值。数值使用 `*int64`，所以 `0` 不会与缺失混淆。

| 身份 | 接受的 JSON 路径（均在 WS 的 `data` 内） | 用途 |
|---|---|---|
| v4 seq | `v4Seq`（优先）、`seq`、`message.v4Seq`、`message.seq` | 首选内容事件身份；填充兼容字段 `Message.Seq` 与原始指针 `Message.V4Seq` |
| messageId | `messageId`、`id`、`message.messageId`、`message.id` | 无 v4 seq 时的稳定消息身份 |
| turnId | `turnId`、`message.turnId` | 消息与工具的轮次作用域 |
| 数值 turn/step | `turn`、`step`、`message.turn`、`message.step` | 旧 v4 形状的复合作用域；不会把数值 turn 改写成字符串 `turnId` |
| callId | `callId`、`toolCallId`、`message.toolCallId`、`message.source.callId` | tool/call ↔ tool/result 的首选配对键 |
| content tool-call ID | `content[].callId`、`content[].toolCallId`、`content[].id`（仅 `type:"tool-call"`） | v4 `tool-call` 块的 callId；块级值优先于帧级值 |

同时补上了真实 v4 `tool/call` 的 `name`/`arguments` 读取；旧 `tool`/`args` 形状仍兼容。`user/message` 重放保留 `user` 角色，不再为了走 Stream 而变成一条 assistant 消息。

## 3. 内容 upsert 的优先级与兼容策略

非工具消息按以下顺序认身份，角色始终是键的一部分（同一消息的 reasoning 与 text 不互覆盖）：

1. `v4Seq + role`；可直接与旧 load 的 `Seq + Role` 对账。
2. `messageId + role`。
3. `turnId + step + role`，或数值 `turn + step + role`。`step` 缺失显式记为 `unknown`，不会编 0。
4. 完全没有上述真值时，使用 **`Stream 实例 ID + Reset 代次 + role`** 的进程内局部键。

第 4 项只防止局部 fallback 与旧消息或其他轮次误并。它不持久、不跨 Stream、不跨进程，也不承诺 load↔WS 去重。局部 fallback 不再退回“数值碰巧相同的 `Seq + Role`”匹配。

旧调用方手写的裸 `Message` 没有内部身份键，仍兼容：

- 非工具按 `(Seq, Role)` 更新；
- 工具按 `(Seq, Tool, Args)` 更新，避免不同 v4 seq 的同参数调用跨轮覆盖；旧测试中双方 Seq 都未指定的零值用法仍可按原方式合并。

相同 v4/messageId 完整事件在同一个 Stream 内逐字重放是幂等的；跨 Stream/load 则靠上述强内容键去重。只有 messageId 的增量不会被按“整条重放”误杀。

## 4. 工具配对与到达顺序

| 到达情况 | 契约处理 |
|---|---|
| 有 callId，call → result | 按 callId 更新同一张工具卡；同 Tool+Args 不影响其他 callId |
| 有 callId，result → call | 先保留独立结果，后续同 callId 的 call 补 Tool/Args；不丢结果、不挂到最近一条 |
| 无 callId，同一 Stream 代次且同 turn，call → result | 保留旧行为，补到该作用域内最近一条尚未有结果的 call |
| 无 callId，result → call，或 turn 不同 | 结果独立保留；不与之前或之后的 call 猜配 |
| 两轮同 Tool+Args | callId 不同则分开；无 callId 时 Stream+Reset 代次与 turn 共同隔离 |
| `Reset` | 清空 tools、局部工具序号和重放记录；已合入消息列表的副本不受影响 |

工具的 v4/message 身份与 call/result 配对键是两件事：配对优先 callId；result 带 v4/message 身份时保留 result 的身份，以便与 load 的 `tool/result` 投影对账。完整 tool/call 或 tool/result 不再被当成 turn 结束；否则壳会在两帧之间 Reset，配对上下文必然丢失。文本/思考的既有完整消息结束行为及 `turn/done`、`turn/error`、`turn/end` 保持不变。

无 callId 且同 turn 有多个相同 Tool+Args 的未完成 call 时，事件顺序不足以证明全局一一对应；这里只保留原有“顺序到达时补最近未完成项”的局部语义，不声称能恢复乱序下的真实配对。

## 5. 未识别、未推断的字段与边界

以下字段**不用于发明内容身份**：

- `sourceEventSeqs`：来源事件引用，不等于当前消息 ID；当前适配层不实现其回溯投影。
- `surfaceOp`：append/replace 的表面操作，不是消息身份；replace 区间语义仍属于后续工作。
- `stream`、`meta`、`usage`、`view`、`requestId`：分别是流状态、工具元数据、用量、显示声明、审批请求，不冒充 messageId/turnId/callId。
- `step` 不能单独标识一轮，数值 `turn` 也不能单独标识一条消息；只有它们组成的复合作用域才用于 fallback。
- 未列出的 ID 形字段、字符串形式的 `seq/turn/step`、未知 content 块类型不猜测、不转换。
- 没有 v4 seq/messageId 时，不能跨 Stream 或跨 load/WS 证明“两个相同 turn+step 的完整消息一定是同一条”；强身份缺失时宁可保留局部边界，也不编跨轮 ID。

## 6. 仍需宿主与壳层的后续工作

### 宿主（本任务未改）

当前 `cmd/dshd/serve_journal.go:21-33` 只把原始 `ev.Data` 放进 Transport `ServerFrame`，Transport seq 由 journal 另发。`internal/session/log.go:183-198` 的 assistant data 有 `turn/step/message/content`，但没有 v4 `seq` 或稳定 `messageId`。因此：

1. 在 WS/history 的 `data` 中附加原始 `v4Seq`（或明确约定为 data 内 `seq`）与稳定 `messageId`，至少覆盖 user/assistant/tool 三类内容事件；字段应是加法，不能改 Transport 信封 `seq` 的含义。
2. `session.load` 的 `transport.Message` 当前只有投影 `seq/role/text/tool/args/...`（`internal/transport/api.go:40-59`）。若希望仅靠 messageId/callId 跨 load↔WS 对账，投影也需携带同一组原始 `messageId/turnId/callId`；否则至少保证两边都能拿到相同 v4 seq。
3. 宿主契约 owner 需先在 `../DSH-GO/docs/transport-contract.md` 登记字段、提供端/消费端验证与两份快照，再改实现。本任务没有单方修改共享契约。

在上述真值补齐前，本适配层保证“不把 Transport seq 当内容身份、不误覆盖、不跨轮猜配工具”，**不保证缺真值场景的持久去重**。

### `cmd/app` 与渲染消费者（本任务未改）

- 本次适配不需要改 `cmd/app` 即可完成归约：壳应继续只用 `Event.Seq` 更新 `m.seqs/lastSeq/sinceSeq`，不得改用 `Message.Seq` 做水位。
- `cmd/app/main.go:960` 的本地 user echo 仍使用固定 `Seq:-1`。后续应给本地 echo 独立的会话内作用域，或在宿主真值回来后按 messageId 对账；不要借 Transport seq 编号。
- `plugins/dsconversation/conversation.go:365,383,476` 等渲染状态仍以 `msg.Seq` 做复制/展开 key。未知 v4 seq 现在不会再伪造 Transport 正数，因此这些 UI key 需要后续改为专门的内容/局部渲染键；这是展示状态问题，不在本任务允许路径内，不影响本次 Merge 防覆盖断言。

## 7. 先红后绿与验收

新增测试：`internal/apiclient/message_identity_test.go`，并补一条旧投影跨 v4 seq 的工具合并测试。

旧实现先跑 8 条核心用例，**8/8 行为断言 FAIL**（不是编译失败）：

- load v4=1/100 后 WS transport=1 覆盖 v4=1；
- 同 v4 seq、同 messageId 重放各多一条；
- 缺 v4 身份时 Transport seq 覆盖旧消息；
- 同 Tool+Args 的不同 callId 被折叠且结果串配；
- result 先到时结果丢失；
- 无 callId 的结果跨 turn 填入最近 call；
- Reset 后 tools 残留。

实现后验收命令：

```bash
export PATH="/c/Program Files/Go/bin:$PATH"
CGO_ENABLED=0 go test ./internal/apiclient -count=1
CGO_ENABLED=0 go vet ./internal/apiclient
CGO_ENABLED=0 go build ./cmd/app ./cmd/mockd
```

2026-11-04 最终实测（工作树含其他 owner 的并行改动）：

- `CGO_ENABLED=0 go test ./internal/apiclient -count=1`：**exit 0**，`ok ... 5.691s`；本文件新增 12 条测试，另有 `merge_test.go` 的跨 v4 seq 工具回归。
- `CGO_ENABLED=0 go vet ./internal/apiclient`：**exit 0**。
- `CGO_ENABLED=0 go build ./cmd/app ./cmd/mockd`：**exit 0**。
- 直接消费方 `CGO_ENABLED=0 go test ./cmd/app ./plugins/dsconversation ./plugins/dsrightbar -count=1`：三包均 **exit 0**。
- 额外全树回归 `CGO_ENABLED=0 go vet ./...` 与 `CGO_ENABLED=0 go build ./...`：均 **exit 0**。
- `gofmt -l internal/apiclient` 为空，`git diff --check -- internal/apiclient docs/message-identity.md`：**exit 0**。

未运行真实 dshd 端到端生成/重连；以上只证明适配层归约、直接消费方回归和当前工作树编译/静态检查，不扩大声明为真实宿主整链验收。
