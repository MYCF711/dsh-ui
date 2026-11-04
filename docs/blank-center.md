# G7 · 直接打开大会话时中栏空白

owner：G7 消息区空白修复。任务 base：`f007fad`。

## 结论

已修复。根因在允许修改的壳层滚动收尾，不在插件：

1. `dsconversation` 在声明阶段按上一帧 `ScrollState.Offset` 做视口虚拟化；首批消息到来时它是 `0`，所以尾部消息只占位。
2. Viva `Follow(true)` 到布局末尾才按本帧内容高度把 Offset 移到尾部。这一帧仍然只画了顶部附近的消息，必须再有下一帧才能按尾部 Offset 实画。
3. 旧 `applyPendingScroll` 在之前的空消息 Hero 帧就清空 `pendingScroll`，首批消息布局后既不恢复也不补帧，空白因此定格。切走再切回有额外 open/refresh 重绘，所以又能显示。

修复位于 [`cmd/app/main.go`](../cmd/app/main.go#L468-L490) 和 [`cmd/app/main.go`](../cmd/app/main.go#L1200-L1205)：空消息时不消费 `pendingScroll`；首批消息布局后恢复已保存位置（若有），返回 `true` 并由 `cfg.Paint` 调一次 `Invalidate`。只补一次，不形成持续重绘。插件未改。

## rev 与修改边界

| 项 | 值 |
|---|---|
| 任务指定 base | `f007fadd18964a1f2dad0aefa9a939855ee23bcc` |
| 回归测试最初跑红时 | `f007fad` 之后工作树已有其他 owner 改动；G7 测试为新增未提交改动 |
| 截图二进制构建时 HEAD | `f298be8226a14fcedffa05333bef8ce771765b7a` + G7 未提交补丁 |
| 最终测试时 HEAD | `9a155c71510179a52255a0e068342d6d4c999f55` + 未提交 `main_test.go` |
| 截图时 G7 补丁快照 | [`tmp/blank-center/fix.patch`](../tmp/blank-center/fix.patch) |
| 最终 `main.go` 相对截图构建的额外差异 | 并发 owner 在 `OnKey` 成功路由后增加一次 `Invalidate`；本轮截图没有注入按键，不改变本条消息加载/滚动路径 |
| 本任务源码修改 | `cmd/app/main.go`、`cmd/app/main_test.go` |
| 本任务报告 | 新建 `docs/blank-center.md` |
| 未修改 | 插件、`docs/progress.md`、UIPLAN、宿主仓 |
| commit / push | 本任务没有执行。验证期间其他 owner 将 HEAD 从 `f298be8` 推进到 `9a155c7`，该提交已包含同一份 G7 `main.go` 改动；本任务未做回退或覆盖 |

截图 app SHA-256：`8de962689b12f3f90825498433d0f6214c73cee4cf981ee095c7f9e32a50e695`；mockd SHA-256：`d789cf8df1059a792c263bb7999404b4763245d0b933b2785439328c3f8b6724`。见 [`bin/sha256.txt`](../tmp/blank-center/bin/sha256.txt)。

## 先红后绿

新增 [`TestFirstLoadedMessagesGetFollowUpFrame`](../cmd/app/main_test.go#L323-L400)，用真实 `flex.View` / `ScrollState` 复现声明期虚拟化与布局末尾 Follow 的两帧时序，不需要 OS 窗口。初版先检查空帧不得消费 `pendingScroll`，修复后又扩展为检查“只请求一次补帧、第一帧尾部未实画、第二帧尾消息出现”。

### 旧实现跑红

命令：

```bash
CGO_ENABLED=0 go test ./cmd/app -run '^TestFirstLoadedMessagesGetFollowUpFrame$' -count=1 -v
```

退出码：**1**。实际失败：

```text
=== RUN   TestFirstLoadedMessagesGetFollowUpFrame
    main_test.go:339: 空消息首帧消费了 pendingScroll；首批消息布局后不会再有恢复/补帧机会
--- FAIL: TestFirstLoadedMessagesGetFollowUpFrame (0.00s)
FAIL
TEST_EXIT=1
```

### 修复后单测与最终验收

```bash
CGO_ENABLED=0 go test ./cmd/app -run '^TestFirstLoadedMessagesGetFollowUpFrame$' -count=1 -v
# PASS；TEST_EXIT=0

CGO_ENABLED=0 go test ./cmd/app -count=1
# ok github.com/MYCF711/dsh-ui/cmd/app 0.052s
# FINAL_TEST_EXIT=0
```

最终命令在 `9a155c7` 上重跑，退出码 **0**。原始日志：[`final-test.log`](../tmp/blank-center/final-test.log)。

构建截图二进制：

```bash
CGO_ENABLED=0 go build -o tmp/blank-center/bin/app.exe ./cmd/app
CGO_ENABLED=0 go build -o tmp/blank-center/bin/mockd.exe ./cmd/mockd
# 两条均 exit 0
```

## 真实会话副本与 mockd

- 输入：`tmp/perf-gap/home-template/sessions` 下 6 个真实会话，复制到 `tmp/blank-center/home/sessions`；源只读，未写真实 home。
- primary：`session-858445da-685e-43d3-ba73-f25ade9619a8`。
- primary 源/副本 SHA-256 均为 `d3826673ff7be788522f222880398c8bc7277cf0a72787ed18e7a98fbdf867ef`，见 [`session-copy-sha256.txt`](../tmp/blank-center/session-copy-sha256.txt)。
- mockd：`mockd.exe -sessions tmp/blank-center/home/sessions -tail 0 -addr 127.0.0.1:0`。
- OS 分配地址：`http://127.0.0.1:56220`；启动日志确认 `6 个 / 6 文件 / 0 坏文件`，见 [`mockd.log`](../tmp/blank-center/mockd.log)。

### 为什么验收图经过 WS 保持代理

直接连该 mockd 时，`/ws` 每 200ms 无条件注入“第 N 条假事件”。第一轮诊断图虽然证明中栏已有内容，却把这些合成事件追加到真实历史，顶栏从宿主 40/200 错增为 105/265，不能用于计数验收；诊断日志保留在 `tmp/blank-center/direct-tail40.log`、`tmp/blank-center/g7-1/`。

最终图使用同一 mockd 的临时代理 `http://127.0.0.1:56808`（同为 OS 分配端口）：HTTP body/响应原样转发，`GET /ws` 只保持握手 60 秒，不返回任何合成事件。代理前后的 `session.load tail=40` 响应逐字节一致（`cmp` exit 0）。因此最终图只验真实历史的加载、布局和切回，**不验 WS 流**。

宿主对账原始响应：[`host-tail40.json`](../tmp/blank-center/host-tail40.json)、[`host-tail200.json`](../tmp/blank-center/host-tail200.json)；摘要：[`host-counts.json`](../tmp/blank-center/host-counts.json)。

| tail | 宿主 `len(messages)` | 宿主 `total` | 首/末 seq | 末条角色 |
|---:|---:|---:|---|---|
| 40 | 40 | 2065 | 7459 / 7707 | tool |
| 200 | 200 | 2065 | 6657 / 7707 | tool |

## 首载 / 切回图与计数

窗口均为 1196×839（客户区 1180×800），使用既有 `-shot` 无头路径；app 均抓图后自行退出，退出码均为 **0**。

| 场景 | 截图 | app 日志 | 画面计数与宿主对账 | 中栏唯一色数 |
|---|---|---|---|---:|
| 直接打开，tail 40 | [`direct-tail40.png`](../tmp/blank-center/real-only/direct-tail40.png) | [`direct-tail40.log`](../tmp/blank-center/real-only/direct-tail40.log) | 顶栏“共 40”，状态“已连接 · 40 条（总 2065）” = 宿主 40 / 2065 | **226** |
| 直接打开，tail 200 | [`ref.png`](../tmp/blank-center/real-only/g7-1/ref.png) | [`ref.log`](../tmp/blank-center/real-only/g7-1/ref.log) | 顶栏“共 200”，状态“已连接 · 200 条（总 2065）” = 宿主 200 / 2065 | **226** |
| 切走 5 个再切回 primary，tail 200 | [`switched.png`](../tmp/blank-center/real-only/g7-1/switched.png) | [`switched.log`](../tmp/blank-center/real-only/g7-1/switched.log) | 顶栏“共 200”，状态“已连接 · 200 条（总 2065）” = 宿主 200 / 2065 | **226** |

三图中栏均有真实历史消息/工具行，没有“第 N 条假事件”。旧实测首开唯一色数为 1；本轮 tail40/tail200 首开均为 226。统计源：[`image-stats.json`](../tmp/blank-center/real-only/image-stats.json)，SHA-256：[`image-sha256.txt`](../tmp/blank-center/real-only/image-sha256.txt)。

实际 app 参数（tail 40 例；tail 200 将数字替换为 200）：

```bash
tmp/blank-center/bin/app.exe \
  -addr http://127.0.0.1:56808 \
  -session session-858445da-685e-43d3-ba73-f25ade9619a8 \
  -tail 40 -title dsh-ui-blank-center-real40 \
  -shot tmp/blank-center/real-only/direct-tail40.png -shot-wait 12
# REAL_DIRECT_TAIL40_EXIT=0
```

切回序列：`d70b7c2f → 226dbf4f → 62bcb7d2 → 1286ce28 → a09a4535 → 858445da`，`-switch-every 1 -shot-wait 13`；日志明确最终当前会话为 primary，`REAL_SWITCHED_TAIL200_EXIT=0`。

### 区域差分

直接打开 tail200 与切回 tail200：

| 区域 | 差分像素 |
|---|---:|
| 原 G7 统计中栏 `(300,102)-(676,680)` | **0** |
| 完整中栏 `(280,34)-(680,776)` | **0** |
| 标题栏 | 116（两轮使用不同调试窗口标题） |
| 状态栏 | 1422（切回取消旧 WS 保持握手后显示“已断开”） |

证据：[`region-diff.json`](../tmp/blank-center/real-only/region-diff.json)。通用整窗脚本 [`g7-matrix.json`](../tmp/blank-center/real-only/g7-matrix.json) 报 1538 像素差异，所以其整窗 verdict 仍是 FAIL；**本任务只据此宣称消息区一致，不宣称整窗逐像素 PASS**。`pngmatrix.py` 本身 exit 0 只表示分析完成，不表示 verdict 通过。

## 清理

- app 三个进程均为自己的 `-shot` 正常退出（exit 0）。
- 仅回收本任务启动的后台 job：`bash-9c48809fd2394522`（mockd）和 `bash-6de4b05dbd15496d`（WS 保持代理），两者最终状态均为 `killed`。
- 回收后 `curl --max-time 1` 检查 `56220`、`56808` 均为 `CLOSED`；未按镜像名杀进程，未触碰其他 owner 进程、DSH 或 magpie。

## 未验项

1. 本轮按任务要求使用临时 mockd + 真实会话只读副本，**没有重新起真 dshd**；不把结果写成真宿主整链验收。
2. 为排除 mockd 周期假事件，最终图的 WS 握手被保持；实时流式、history 补洞和重连不在本轮视觉验收内。
3. 整窗逐像素仍非零，原因与数值见上；已分别验证完整中栏为 0，不把整窗 FAIL 隐去。
4. 未做 OS 键盘/滚轮、流式期间切换、长时滚动手感测试；本修复只覆盖首载和切回的历史消息可见性。
