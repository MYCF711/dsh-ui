# WP-C3 · 工具真值状态与专用卡

## 修订与边界

- 原始 base：`b43dcdcf296e7eeba2af6095cb347633c86181dd`（短 rev `b43dcdc`）。
- 施工 HEAD：`b7e9680f9b2bbd5c8b69814d7a8e7969f8db1da5`（短 rev `b7e9680`），叠加任务下发时已有的 WP-C1/C2、WP-D1、STATS-WIRE 等共享导出。
- 本轮只改 `plugins/dstool/**`、`plugins/dsconversation/**`，并新建本文档；未改 `apiclient`、`pluginapi`、`cmd/app`、宿主、共享契约或其他插件。
- 未 commit、未 push，未杀进程，未改 `dstool.Service` 的 Renderer 分派表或审批 ID 流程。

## 实现

### 1. 状态只来自可证明真值

状态归约位于 [dstool/status.go](../plugins/dstool/status.go) 与 [dsconversation/wpc3.go](../plugins/dsconversation/wpc3.go)，标题徽标由 [dstool/dstool.go](../plugins/dstool/dstool.go) 绘制。

| 状态 | 允许显示的依据 | 明确不做的推断 |
|---|---|---|
| `preparing` / 准备中 | `View.Badge == "preparing"` | 有 Tool/Args 不等于 preparing；无真值时 neutral、隐藏徽标 |
| `running` / 运行中 | 显式 `View.Badge == "running"`；或 `Busy` 且消息的 TurnID/Turn 精确匹配 ActiveTurn，同时没有结果 | 不用“最新消息”“数组下标”或其他 Turn 的 Busy 冒充 running |
| `ok` / 完成 | 显式 Summary/View、`View.Badge == "ok"`；bash 可解析的零退出码 | 没有结果时不默认成功 |
| `error` / 失败 | `View.Badge == "error"`；bash 可解析的非零退出码 | 不把 `turn/error` 擅自归给某次工具调用，不把无退出码文本猜成失败 |
| `stopped` / 已停止 | 原始 `MessageMetadata.Stopped`；匹配 Turn 的显式 stopped/interrupted 终局（壳统一为 `TurnEvent.Kind == "stopped"`）；或显式 stopped Badge | 已完成/失败的调用不因所属 Turn 后来停止而改写结果状态 |
| neutral | 上述真值都缺失或状态值无法识别 | 不占徽标位置，不显示“未知成功/准备中” |

已有消息 meta、Turn shell/footer 的 stopped 真值保留，因此 stopped 可同时出现在工具标题与原有终局区域；没有删除 WP-C1/C2 的展示路径。展开 View 时 Badge 原文仍保留，但只有明确 ok/error 分别着绿/红，preparing/running/stopped/耗时等不再统一涂成成功色。

### 2. 同参交错调用保持独立

沿用现有 `CallID → pluginapi.MessageKey → Plugin.toolRenderID → dstool 局部 int` 适配，没有引入最近结果查找，也没有改 Transport 或共享身份契约。

[dsconversation/wpc3_test.go](../plugins/dsconversation/wpc3_test.go) 使用两个 Tool、Args、Turn 均相同但 CallID 分别为 `call-A`/`call-B` 的调用，断言：

- MessageKey 与局部 int 均不同；
- `RESULT-A`、`RESULT-B` 各出现一次且顺序不交换；
- A 的完成与 B 的失败分别在自己的标题行，不串状态。

[dsconversation/wpc3_headless_test.go](../plugins/dsconversation/wpc3_headless_test.go) 再经真实命中区点击依次展开 A、B，断言两个局部 ID 不同且最终两者仍同时展开。

### 3. 专用卡只消费已有字段

[dstool/render.go](../plugins/dstool/render.go) 保持既有 bash 命令/退出码、glob/grep 模式与 Summary 优先，并补齐：

- read/write/edit 同时消费 `filePath` 与 `file_path`，展开体保留 `offset`/`limit`（起始行/行数）；
- 标题摘要顺序为 Summary → 有实际内容的 View → 参数派生值；展开体仍是 View 优先于定制 Renderer；
- edit 仅在 `old_string`/`oldString` 与 `new_string`/`newString` 两类字段都真实存在时展示“旧内容/新内容”；缺任一方都整段隐藏；
- 不计算或伪造 unified diff、行号、增删统计，不新增图片、子调用或 parentCallId 图形。

## 旧实现先红证据

实现前运行：

```bash
CGO_ENABLED=0 go test -count=1 -run WPC3 ./plugins/dstool ./plugins/dsconversation
```

结果为 **exit 1**。dsconversation 的行为红包括：preparing/running/ok/error 标题状态缺失、非零退出码未映射失败、stopped 只在 meta/Turn 区域而不在工具标题、同参调用状态无法独立。首次合并运行还发现新 dstool 测试支架把 `[]string` 传给 `strings.Contains` 的一处编译错误；只修正该测试支架后，在旧实现上重跑：

```bash
CGO_ENABLED=0 go test -count=1 -run WPC3 ./plugins/dstool
```

结果仍为 **exit 1**，这次是实际行为断言：edit 的 `file_path + offset/limit` 未展开显示、真实 old/new 未显示、View 优先时参数 old/new 仍泄漏到标题。原始输出保存在 `/c/Users/Administrator/AppData/Local/Temp/wp-c3-red.log`。unknown/neutral 与“缺 old/new 不伪造”本身在旧实现中已有部分正确行为，没有用相反断言人为制造失败；同一测试文件中的缺失能力使旧实现真实转红。

## 绿色验收命令与结果

环境：

```bash
export PATH="/c/Program Files/Go/bin:$PATH"
export GOPROXY=https://goproxy.cn,direct
```

| 命令 | 结果 | 证据/说明 |
|---|---:|---|
| `CGO_ENABLED=0 go test -count=1 -run WPC3 ./plugins/dstool ./plugins/dsconversation` | exit 0 | 两包均 ok；聚焦状态、交错、diff 与截图测试 |
| `CGO_ENABLED=0 go test -count=1 ./plugins/dstool ./plugins/dsconversation` | exit 0 | 两包均 ok（最终复跑） |
| `WPC3_CAPTURE_DIR='C:/Users/Administrator/AppData/Local/Temp/dsh-ui-wpc3-isolated/plugins/dsconversation/testdata/wp-c3' CGO_ENABLED=0 go test -count=1 -v -run TestWPC3RealStatusExpansionAndInterleavedEvidence ./plugins/dsconversation` | exit 0 | 两尺寸均 PASS，尺寸和字节数由测试实际读取图片核验 |
| `CGO_ENABLED=0 go test -count=1 ./...` | exit 0 | 全仓所有包通过；含 MSGID/MDSTREAM、WP-C1/C2、A-TRAJ/FG 与既有审批测试 |
| `CGO_ENABLED=0 go vet ./...` | exit 0 | 无输出 |
| `CGO_ENABLED=0 go build ./...` | exit 0 | 无输出，CGO_ENABLED=0 |

日志同时留在 `/c/Users/Administrator/AppData/Local/Temp/wp-c3-focused-second.log`、`wp-c3-target-tests.log`、`wp-c3-capture.log`、`wp-c3-all-test.log`、`wp-c3-all-vet.log`、`wp-c3-all-build.log`。

## 两尺寸真实绘制/展开证据

截图不是静态模板：测试把真实 `apiclient.Message` 快照交给 `Plugin.draw`/`afterDraw`，通过 Viva headless 的真实命中区 Press/Release 展开两个同参调用，再读取图片尺寸并保存 PNG。

| 尺寸 | 文件 | 字节 | SHA-256 |
|---|---|---:|---|
| 1182×832 | [conversation-wpc3-1182x832.png](../plugins/dsconversation/testdata/wp-c3/conversation-wpc3-1182x832.png) | 34676 | `90e5b0005398e99c3d9393fddc9718ffbac0f1ff694412dcb302ef70f09d3e71` |
| 1920×1032 | [conversation-wpc3-1920x1032.png](../plugins/dsconversation/testdata/wp-c3/conversation-wpc3-1920x1032.png) | 60144 | `8457190f943357c2f92ab4c32d4b05f500b01be9c1a60a36d0fa378dbabc62a2` |

两张图均可见：准备中、运行中、完成、失败、已停止；neutral 的 `edit · neutral.go` 没有状态徽标；两个 `printf same` 同参调用同时展开，分别显示绿色 `exit 0 · RESULT-A` 与红色 `exit 1 · RESULT-B`。

## 未验项与有意不做

- 本轮没有新起真 dshd/真模型做流式生成、取消或审批执行验收；证据是实际插件归约与 Viva 无头点击/像素，不冒充真实宿主整链。既有审批原 ID 与其他共享链路仅由全仓回归门确认未回退。
- 只做 Scale=1 的 1182×832 与 1920×1032；未验 OS 键盘、真实窗口焦点、高 DPI、人工滚动观感。
- 未做完整工具树，也未做 unified diff 算法；没有 old/new 真值时不画 diff，有真值时只展示原字段内容。
- 没有图片、子调用、parentCallId 真值输入，因此没有对应图形或伪造占位。
