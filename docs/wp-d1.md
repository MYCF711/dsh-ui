# WP-D1 · Trajectory 当前窗口交互与验收

> 日期：2026-11-04；最终集成基线：`b8f9f8d713b20bc8304d64d7df024fe38c517346`（`b8f9f8d`）+ 最新共享导出（含 `deliverables/questions/GoalBar`）。范围只含 `plugins/dsrightbar/**` 与本文。
> 本项不重做 A-TRAJ 分组，不改 `pluginapi`、`cmd/app`、`apiclient`、其他插件或共享契约。

## 最终集成记录

- WP-D1 原始 patch 的唯一冲突是 `plugins/dsrightbar/visual_test.go:13` 起始的上下文；final2 以当前共享文件为主，不再重放该文件，排除它后其余路径一次 apply 成功。全过程未 reset，也未覆盖其他共享导出。
- 三方/语义合并以当前 `visual_test.go` 为主，完整保留 FG/GoalBar 后续内容，包括 `ds-topbar` 装配、真实 `Sessions` / `ToggleRightbar`、右栏计数断言、全屏/恢复、收起/corner 重开及 1920×1032、1182×832、960×832 捕获。当前文件与 WP-D1 隔离成品的 SHA-256 均为 `4646510c551aff935030dbac5e75676a204df67d162a3c86ce75e22ff09e5fd2`，所以无需覆盖或回退任何一侧；WP-D1 新交互与两尺寸断言由 `wpd1.go`、`wpd1_test.go`、`wpd1_visual_test.go` 合入。
- 集成后 WP-D1 Go 源码与两尺寸 PNG 均和隔离成品一致；日志由 final2 重新生成。`deliverables/questions/GoalBar` 及其他包、宿主、共享契约均未改，未 commit/push。

## 1. 实现结论

现有共享导出里的 WP-D1 实现不是仅有基线绿：同一组定向测试在旧 `rightbar.go`、没有 `wpd1.go` 时三项失败，在当前实现上全部通过。实现入口是 `rightbar.go` 的 `drawTrajectoryWorkspace`，主体在 `wpd1.go`，真实 Viva 行为测试在 `wpd1_test.go` 与 `wpd1_visual_test.go`。

| 验收项 | 当前行为与实际断言 |
|---|---|
| 当前窗口搜索 | 右栏与主区 Trajectory 各有真实 `flex.Field`，避免同屏抢焦点。只扫描当前 `Snapshot.Msgs`，不发 `loadOlder`、历史或网络请求。查询先 `TrimSpace`、再做大小写不敏感包含匹配；覆盖 Turn/TurnID、Step、工具名、参数、结果、`View` 正文/字段及已有时间真值。测试通过真实焦点和 `Type` 输入 `tUrN-aLpHa`，再逐项验 `STEP 7`、工具名、参数、结果与错误正文；空白恢复、无结果和清除均有画面断言。 |
| Step 展开 / Inspect | 点击真实 Step 行展开或收起自己的 Inspect。Inspect 只画已有原值：`step`、`turnId`、`turn`、`args`、`result`、`callId`、`time`、`view.body`。测试断言多行结果、原始参数、`call-alpha` 与 `1762222445123` 实际出现，再点同一步后 Inspect 消失。 |
| Turn ↔ Step 选择联动 | Step 点击同时设置所属 Turn 与该 Step；Turn 点击设置该 Turn 及首个 Step。状态按会话保存，Step 身份使用共享 `MessageKey` 与过滤前的原始消息下标，不使用过滤后下标。测试读取的是插件真实选择状态，并连续点击两个 Step、收起/展开 Turn 验证联动。 |
| 缺字段隐藏 | 当前 `apiclient.Message` 没有 `parentCallId`、`isError`、`error` 字段，因此 Inspect 整段不画这些标签，不补空串、`false` 或推测值。`View.Badge="error"` 只用于既有失败徽标/圆点；Turn 级 `TurnEvents.Error` 只参与 Turn 搜索，不冒充某个 Step 的错误字段。测试逐项断言 `parentCallId`、`isError`、`error` 不出现在 Inspect。 |
| 1000 条全量搜索、布局有界 | 搜索发生在最近 80 条展示裁剪之前，因此能找到已被展示上限裁掉的 `tool-0000`。首屏保留真实总数 `1000 次` 与 `920` 条未显示提示；可见行加 overscan 才创建文本节点，其余只保留高度槽。测试断言首屏工具文本行数 `>0 且 <80`，搜索最早一步后恰好布局 1 个工具行。这里不宣称 1000 个节点全部实画。 |
| A-TRAJ 保持 | 沿用既有用户消息/宿主 Turn 分组、默认只展开最后一组、整组优先裁剪和最近 80 条上限；原有 `turns_test.go`、`collapse_test.go`、`grouped_visual_test.go` 继续通过。 |
| FG-3 保持 | `TestConversationTabsReallyClickAndRememberSession` 真点击验证：有工具也默认收起右栏、corner 展开/收起、chat→trajectory→chat、A/B 各自记忆视图、零消息空态、draft 不变且 load/cancel/draft 副作用均为 0。`TestV4AHeadlessResizeAndScreenshots` 另验全屏/恢复与右栏收起/corner 重开。 |

## 2. 先红后绿记录

所有命令均在仓库根目录执行，Go 为 `go1.27.3 windows/amd64`，`CGO_ENABLED=0`。日志中的 `EXIT` 是命令真实退出码。

| 阶段 | 命令 / 场景 | 结果 | 原始日志 |
|---|---|---|---|
| 旧实现反证 | 临时副本恢复 `HEAD:plugins/dsrightbar/rightbar.go`，停用 `wpd1.go` 和两尺寸测试，再运行 `go test ./plugins/dsrightbar -run '^TestWPD1' -count=1 -v` | **预期 exit 1**：搜索边界缺失、Step 不可点、1000 条首屏仍布局 80 行，三项均红 | [old-red.log](../plugins/dsrightbar/testdata/wp-d1/old-red.log) |
| WP-D1 当前实现 | `WPD1_CAPTURE_DIR=<testdata/wp-d1> go test ./plugins/dsrightbar -run '^TestWPD1' -count=1 -v` | **exit 0**：3 个行为测试 + 1 个两尺寸测试全部 PASS | [two-size-green.log](../plugins/dsrightbar/testdata/wp-d1/two-size-green.log) |
| 右栏整包 | `go test ./plugins/dsrightbar -count=1`；`go vet ./plugins/dsrightbar` | **TEST_EXIT=0 / VET_EXIT=0**，包含 A-TRAJ 与 V4A 测试 | [package-test-vet.log](../plugins/dsrightbar/testdata/wp-d1/package-test-vet.log) |
| FG-3 定向回归 | `go test ./plugins/dstopbar -run '^TestConversationTabsReallyClickAndRememberSession$' -count=1 -v` | **exit 0**，两尺寸 chat/trajectory 路径及几何断言通过 | [fg3-green.log](../plugins/dsrightbar/testdata/wp-d1/fg3-green.log) |
| 全仓验收 | `go test ./... -count=1`；`go vet ./...`；`go build ./...` | **TEST_EXIT=0 / VET_EXIT=0 / BUILD_EXIT=0** | [repo-test-vet-build.log](../plugins/dsrightbar/testdata/wp-d1/repo-test-vet-build.log) |

## 3. 真实 Viva 两尺寸证据

`TestWPD1TwoSizeRealSearchInspectSelectionAnd1000Evidence` 使用真实 `headless.Session`、Viva Paint/flex 布局、HitTest、文本输入和 PNG 编码；不是静态图片或只调用过滤函数。每个尺寸都重新走一遍：

1. 点击“搜索当前窗口”并真实输入 `result-needle`，断言只显示 `AlphaTool`；
2. 点击 `Turn 1 · Step 7 · AlphaTool`，断言 Inspect、`call-alpha`、时间与结果出现，且真实 Turn/Step 选择状态非空；
3. 再点同一步确认 Inspect 收起，清除搜索；
4. 换成 1000 条消息，断言首屏布局行数有界、总数与 920 条隐藏提示仍在；
5. 输入 `TOOL-0000`，断言全窗口搜索恰好布局最早的 1 条；blur 后确认查询词仍真实显示，再截图。

| 尺寸 | 搜索 + Step 展开 + 选择 + Inspect（含缺字段隐藏） | 1000 条中搜索最早 Step |
|---|---|---|
| 1182×832 | [search-inspect-1182x832.png](../plugins/dsrightbar/testdata/wp-d1/search-inspect-1182x832.png)<br>SHA-256 `431df5dc7ad770cd0b2f9572416dbc8243ec43a1432dc2e41f920e69e8e86360` | [1000-search-1182x832.png](../plugins/dsrightbar/testdata/wp-d1/1000-search-1182x832.png)<br>SHA-256 `b3b0e918c304c3fdad6004fe47a8444edb34c6a35d5e773895de0491a8244ea8` |
| 1920×1032 | [search-inspect-1920x1032.png](../plugins/dsrightbar/testdata/wp-d1/search-inspect-1920x1032.png)<br>SHA-256 `d738bd11c753fec738d9ab728527512d9273ea49266146989b225def7553cbb7` | [1000-search-1920x1032.png](../plugins/dsrightbar/testdata/wp-d1/1000-search-1920x1032.png)<br>SHA-256 `83ab5faba0cb17f5c85568998e93f43c50bc0d4447bb89931045cfc9e25815f9` |

搜索/Inspect 截图可见 `1 / 3 Step`、选中行、`step/turnId/args/result/callId/time`，且没有 `parentCallId/isError/error` 标签；1000 条截图可见 `1 / 1000 Step`、`TOOL-0000`、`tool-0000` 与 `Step 1`。

## 4. 证据边界

- 两尺寸测试装配真实 `ds-core + ds-shell + ds-topbar + ds-rightbar`；左区只保留 header/页签，正文刻意未装 conversation，因此空白不是“完整产品聊天区已验收”。FG-3 的完整 chat/conversation/composer 联动由上述定向回归另行验证。
- 证据是 Viva 无头窗口的真实绘制、命中与文本输入；未宣称物理 OS 鼠标/键盘、真实 dshd Transport、完整历史加载或真实长链性能已经由本项验证。
- 完整历史、跨会话搜索、完整工具树，以及宿主尚未提供的 `parentCallId/isError/error` 真值仍归后续 WP-D/契约工作；本项不伪造这些能力。

## 5. 最终集成验收（`b8f9f8d`）

以下命令均在最终集成工作区根目录执行，Go 为 `go1.27.3 windows/amd64`，`CGO_ENABLED=0`；退出码来自本次集成后的实际运行，不是沿用隔离成品的日志。

| 验收 | 命令 | 本次结果 |
|---|---|---|
| WP-D1 + 两尺寸 | `go test ./plugins/dsrightbar -run '^TestWPD1' -count=1 -v` | **exit 0**：3 个行为测试及 `TestWPD1TwoSizeRealSearchInspectSelectionAnd1000Evidence` 全部 PASS；1182×832 与 1920×1032 的搜索/Inspect、1000 条搜索 PNG 均通过非空和像素尺寸断言。 |
| FG-3 | `go test ./plugins/dstopbar -run '^TestConversationTabsReallyClickAndRememberSession$' -count=1 -v` | **exit 0**：两尺寸 chat/trajectory 真点击、每会话视图记忆及几何断言 PASS。 |
| 全仓测试 | `go test ./... -count=1` | **exit 0**。 |
| 全仓 vet | `go vet ./...` | **exit 0**。 |
| 全仓构建 | `go build ./...` | **exit 0**。 |
| 格式检查 | `gofmt -d` 检查本次集成的 5 个 Go 文件 | **exit 0**，无格式差异。 |
