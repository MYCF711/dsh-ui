# API-TIME：canonical Event.time 真值

> 日期：2026-11-04。Owner：API-TIME。
>
> 施工目录：`C:\Users\Administrator\AppData\Local\Temp\dsh-ui-api-time-isolated`。
> Base：`d71c65ed4ce6589ab511806e593869b0e3ce3240`（`d71c65e`）+ 当前共享导出（含 WP-C1）。共享导出原本就是未提交工作树，本包没有 commit/push。

## 1. 实现与契约

- `apiclient.Event` 新增 `Time int64`，与既有 `Frame.Time` 类型完全一致，单位沿用宿主契约的 epoch 毫秒；`0` 表示未知，不用本地墙钟补值。
- WS 的 `serverFrame` 直接复用 `Frame` 的 canonical JSON 解码，`session.history` 与 live WS 因而只有一套时间规则：
  1. 信封 `time` 字段存在时优先于 `data.time`；
  2. 只有信封字段完全缺失，才读取顶层 `data.time`；
  3. 只接受可解码为 `int64` 的正整数；零、负数、字符串、小数、`null` 等都是未知（`0`）；
  4. 信封存在但为零或非法时，不再回退 `data.time`，也不吞掉该事件的 `seq/type/data`。
- 新增 `Frame.Event(topic)` 保真转换，`Kind/Seq/Topic/Data/Time` 与 live 事件一致，供 history 补洞和重放复用。合成 gap 仍没有伪造时间。
- 未改 `HistoryResult`、`lastSeq/journalEpoch`、Transport seq 水位、gap 区间或消息身份归约。无 seq 控制帧仍按原规则向调用方暴露 `Seq=0`，不参与水位。

改动范围仅为 `internal/apiclient/history.go`、`internal/apiclient/mux.go`、新增 `internal/apiclient/event_time_test.go` 与本文。

## 2. 先红后绿与行为断言

旧实现红灯（在写实现前执行）：

```bash
CGO_ENABLED=0 go test ./internal/apiclient -run 'TestCanonicalEventTime|TestHistoryLiveAndReplayPreserveSameEventTime' -count=1
```

结果：exit 1，编译明确报告 `Event{}.Time undefined`、`ev.Time undefined` 与 `history.Frames[0].Event undefined`。实现后同一命令 exit 0。

新增测试的实际断言：

| 测试 | 断言 |
|---|---|
| `TestCanonicalEventTime` | 编译期断言 `Event.Time` 是 `int64`；同一组线上形状分别经 `Frame` 与真实 WS 订阅解码；覆盖 envelope 优先、无 envelope 的 data 回退、双缺失、零、负数、字符串、小数、`null`；每个 WS 用例还检查 `Kind/Seq/Topic` 未变 |
| `TestHistoryLiveAndReplayPreserveSameEventTime` | 同一原始帧经 `session.history`、live WS 与 `Frame.Event` 重放后得到同一正整数时刻；同时断言 `Kind/Seq/Topic/Data` 一致以及 `lastSeq=7/journalEpoch=9` 未变 |
| 既有 apiclient 全套 | 继续覆盖 `TestMultiSessionSeqGapIsReported`、`TestHistorySendsJournalEpoch`、`TestFillGapForwardsEpoch`、`TestSameV4MessageReplayDeduplicatesAcrossTransportSeq`、`TestSameMessageIDReplayDeduplicatesWithoutV4Seq`、`TestMissingV4IdentityDoesNotAliasTransportSeq`，防止 gap、epoch 与 MSGID/内容身份语义回归 |

## 3. 验收命令与结果

以下命令均在上述隔离目录、Go `1.27.3 windows/amd64` 下执行：

| 命令 | 退出码 | 结果 |
|---|---:|---|
| `CGO_ENABLED=0 go test ./internal/apiclient -count=1` | 0 | 全部通过，含新增 time 测试与既有 seq/gap/journalEpoch/MSGID 测试 |
| `CGO_ENABLED=0 go vet ./internal/apiclient` | 0 | 无输出 |
| `CGO_ENABLED=0 go test ./cmd/app ./plugins/dsconversation -count=1` | 0 | 两个直接消费包均通过 |
| `CGO_ENABLED=0 go test ./... -count=1` | 0 | 全仓通过 |
| `CGO_ENABLED=0 go vet ./...` | 0 | 全仓通过，无输出 |
| `CGO_ENABLED=0 go build ./...` | 0 | 全仓通过，无输出 |

施工前也在同一共享导出上运行过 `CGO_ENABLED=0 go test ./... -count=1` 与 `CGO_ENABLED=0 go vet ./...`，两者均 exit 0。

## 4. WP-C1 接入点

WP-C1 文档 §5 第 2 项指出的 apiclient 缺口已由本包补齐；以下壳侧接线不属于本 owner 的允许路径，需由 Lead/壳 owner 后续应用：

1. `cmd/app/main.go` 的 live `applyEvent` 目前仍调用 `applyEventAt(id, ev, 0)`；接入时应把已解析的 `ev.Time` 传给同一元数据路径，而不是继续传零或读取本地墙钟。
2. history 回调目前手写 `apiclient.Event` 并另传 `f.Time`。可改用 `f.Event("session:"+id)`，让重放事件自身也携带同一时间，再走与 live 相同的消费路径。
3. `cmd/app/wpc1_data.go` 的 `recordTurnEventsLocked` 当前是 `data.time` 优先、信封补零，和新优先级相反。接入后应直接消费 apiclient 已解析的时间；不要在 `Event.Time==0` 时再次猜测 `data.time`，否则“信封存在但零/非法”的未知语义会被破坏。apiclient 已在且仅在信封缺失时完成合法 `data.time` 回退。
4. `session.load` 的原始消息 `time/stopped` 保留逻辑不属于本次 canonical Frame/Event 改动，继续沿用 WP-C1 现有路径。

## 5. 未验项

- 未启动真实 dshd 做生成、断线或重连端到端；WS 证据来自 `httptest` 假宿主，history 证据来自真实 HTTP 客户端对测试服务器。
- 未修改 `cmd/app`、WP-C1、`pluginapi`、宿主或共享契约，因此本包只证明真值已在 apiclient 的 live/history/replay 边界可用，不宣称界面已经显示 live envelope time。
- 未做截图、tok/s 真机测算或跨重启持久化验证；这些不影响本次字段解码与保真断言。

最终交付 patch：`C:\Users\Administrator\AppData\Local\Temp\api-time-final.patch`。只对必要新文件执行 `git add -N`，没有 commit/push。
