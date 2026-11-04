# FG-5 · Composer dock 与指标真值交接

> 基线：UI `9a155c7` + 当前工作树；执行日期 2026-11-04。
>
> 本包只改 `plugins/dsstatusbar/**`，新建本文；未改 `cmd/app`、`pluginapi`、`internal/apiclient`、其他插件或共享文档，未 commit/push。

## 1. 先纠真值：当前生产画面显示什么

官方依据：

- [`deriveStats`](../tmp/official/dsh-client-ui-chat/client.js#L7435-L7472)：`turns` 是唯一 turn 数，`steps` 是 assistant/step 数；whole-log `sessionStats` 优先，窗口 fold 只是 fallback。
- [`ActivityPill`](../tmp/official/dsh-client-ui-chat/client.js#L7598-L7634)：速率只来自 `decodeTokens/decodeMs`。
- [`UsagePill` 与 `billedInputTokens`](../tmp/official/dsh-client-ui-chat/client.js#L7495-L7505)：缓存分母是 `uncachedInput + cacheRead + cacheWrite`，不是 `totalTokens`；总用量见同文件 [7635-7672 行](../tmp/official/dsh-client-ui-chat/client.js#L7635-L7672)。
- [`contextOccupancy`](../tmp/official/dsh-client-ui-conversation/client.js#L18635-L18649)：used tokens 与 `contextWindow` 必须同时存在，否则返回 null。
- [`ContextMeter`](../tmp/official/dsh-client-ui-conversation/client.js#L18681-L18748)：14×14 SVG、stroke 2；缺分子或容量时整个部件不画。

| 指标 | 当前 `Snapshot` 能否证明 | 本包生产行为 |
|---|---|---|
| 轮 / 步 | 不能。assistant 条数不是唯一 turn，tool 条数不是 assistant step；tail 也不是 whole-log | **撤下**。不再遍历 `Msgs` 生成“N 轮 · N 步” |
| tok/s | 没有 `decodeTokens/decodeMs`；旧墙钟差不是官方 decode 口径 | **从 dock 撤下**。官方公式函数只在有完整 stats 时接受它 |
| whole-log total | `UsageTotal.totalTokens` 是宿主全日志累计；宿主当前实现为累计 input+output | 显示 **`总量 N`**。不把它改称已验证的官方 billed-input 口径 |
| input / output | total 缺席时，现有正值仍可证明；当前零值表示未知 | 只显示有真值的一侧；不再输出 `↑0` / `↓0` 补零 |
| 缓存命中率 | 只有 `cacheReadTokens`，缺 uncached/cacheWrite 或等价 billedInput 分母 | **隐藏**。不拿 `totalTokens` 当分母，不显示 0% |
| 当前上下文 token 数 | `ContextTokens` 是原始真值，与累计量独立，压缩后可回落 | 保留 **`上下文 N`** 原文，不画成百分比 |
| 上下文百分比 / 环 | 缺 `contextWindow` | **隐藏百分比和环**；不把连接点冒充 ContextMeter |
| 连接状态 | `Disconnected`、Approval 与既有 Status 是真值 | 保留独立 `shell.bottom`；优先级 **断线 > 待审批 > 就绪** |

`metrics.go` 中的 `sessionStats/tokenUsage/contextPressure` 是插件内的官方公式适配层，不冒充 Transport 已有字段。指针保留“缺席”和“真实 0”的区别；当前生产快照没有这些 projections，所以对应 pill 不会出现。

## 2. 24px / 40px 几何校正

**24px 和 40px 都不是官方 composer dock 固定高。**

- 24px：当前壳给 `shell.bottom` 的高度，连接状态仍在那里；本包没有改壳，也不宣称这条全宽行已移除。
- 40px：FG-1 的 Windows caption，不属于 FG-5 dock。
- 官方 [`InputBar.dock`](../tmp/asar-css/039__InputBar.module.css.mjs.css)：`padding-top:4px; gap:12px; justify-content:center`。
- 官方 [`StatsPills`](../tmp/asar-css/028__StatsPills.module.css.mjs.css)：12/20 文字、pill `padding:1px 8px`、内部 gap 6。
- 因而内容盒为 `20 + 2×1 = 22px`，非空 dock 为 `4 + 22 = 26px`；空 dock 为 0。InputBar root 另有 bottom 4px，不能把它误算成 dock 固定高。

本包实画的 pill 使用主题 Caption（官方默认 12px）、LabelTertiary、22px 内容高、dock 26px；没有弹层真值时保持只读，不画可点击假象。

## 3. 位移到底完成了哪一层

已完成，不需要改共享文件：

- `ds-statusbar` 新增 `dock` painter：`SlotMainComposer`，`Order=20`。
- 现有 `ds-composer` 卡片是 `Order=10`，同一列中先画卡片、后画 dock，所以 dock 实际位于卡片下方。
- 两者使用同一个主列中心轴和“官方 clearance 16 − shell 已给 12 = 4”的净边距。
- `connection` painter 独立留在 `SlotShellBottom`，统计不再占这条全宽行的左侧。

没有冒报完成的部分：

1. **没有新增官方 `conversation.composer.dock` 槽契约。** 当前是利用既有 `main.composer` 多 painter 完成的等价位置集成，不是官方 slot registry 的完整移植。
2. **Activity/Usage/Context 三个完整官方部件尚未齐。** 当前只有真值文本 dock；官方 Activity、缓存命中、14px 上下文环、右侧独立 ContextMeter 和弹层仍等 C-STATS。
3. **24px 连接行仍存在。** “统计不再占 shell.bottom”不等于“shell.bottom 已删除”。
4. 真宿主 tail=1/50 whole-log 对账、A/B 会话不串和流式 projection 更新未在本包伪报通过；当前实图是 Viva 无头真实布局，不是 C-STATS 真宿主证据。

### 后续完整官方槽的明确集成点（均不在本包写入范围）

- `pluginapi/plugin.go`：若要求第三方也能注册官方 dock，需由 Lead 增加 `conversation.composer.dock` 槽及 session scope/排序语义。
- `plugins/dscomposer/composer.go`：在 InputBar 卡片后的 dock 容器中绘制该子槽，并把 ContextMeter 放在独立右侧 seat；当前 FG-5 未改此文件。
- `plugins/dsshell/shell.go` / `cmd/app/main.go`：仅在调整主列净 padding、移除或迁移 24px 连接行、令无占用 `BottomHeight=0` 时需要联动；当前位移不依赖这些改动。
- `internal/apiclient` + `cmd/app`：由 Lead 将 C-STATS wire projections 映射进按会话隔离的 `Snapshot`，再在 `dsstatusbar` 接到现有公式适配层。
- 跨仓契约只能由 Lead 按规程写入 `../DSH-GO/docs/transport-contract.md` 并同步双仓 snapshot/golden；本包没有越权修改。

## 4. C-STATS 需要的字段

建议按官方 projection 名称提供以下**每会话、whole-log** 数据。projection 本身可缺席；一旦出现，标为“必填”的数值字段必须全部存在，不能用 `omitempty` 把真实 0 与缺失混成同一种形状。

| projection | 字段 | 必填性 / 含义 | 消费与验收 |
|---|---|---|---|
| `sessionStats` | `turns` | 必填；whole-log 唯一 turn 数，不是 assistant 条数 | 同会话任意 tail 不变；多 assistant step 同属一个 turn 时只计一次 |
| 同上 | `steps` | 必填；assistant/step 数，不是 tool 数 | `steps=0` 时 Activity 整枚隐藏 |
| 同上 | `decodeTokens` | 必填；用于 decode 速率的 output token 数 | 与 `decodeMs` 配对；缺任一则不显示速率 |
| 同上 | `decodeMs` | 必填；decode 累计毫秒 | `TPS=decodeTokens/(decodeMs/1000)`；不能拿消息墙钟差替代 |
| `tokenUsage` | `uncachedInputTokens` | 必填；未缓存 prompt 桶 | 官方缓存分母三项之一；现有 `inputTokens` 不得未经语义确认改名顶替 |
| 同上 | `cacheReadTokens` | 必填；缓存读桶，真实 0 合法 | 分子；同时属于分母 |
| 同上 | `cacheWriteTokens` | 必填；缓存写桶，真实 0 合法 | 缺它时缓存率隐藏，不能默认为 0 |
| 同上 | `outputTokens` | 必填；whole-log 输出 | 官方 Usage total 为三个 prompt 桶之和 + output |
| `contextPressure` | `usedTokens` | 必填；提供端可由官方 `projectedTokens ?? pressureTokens` 归一化 | 与容量同时存在才显示 ContextMeter |
| 同上 | `contextWindow` | 必填；当前路由容量，真实正数 | 缺字段或为 0 时百分比和环隐藏；`min(100, round(used/window×100))` |
| `contextBreakdown` | `systemTokens` | 弹层可选；系统占用 | 只有弹层明细需要；缺整个 breakdown 时仍可按官方单色环显示 |
| 同上 | `toolsTokens` | 弹层可选；工具定义占用 | 同上 |
| 同上 | `messageTokens` | 弹层可选；会话内容占用 | 同上 |

可选替代：若提供端能明确证明 `billedInputTokens = uncachedInput + cacheRead + cacheWrite`，可先给 `billedInputTokens` 作为缓存率分母；但官方 Usage 弹层和总量仍需上述分类桶，不能把现有 `inputTokens` 或 `totalTokens` 直接当 billedInput。

Activity 弹层另需 whole-log `llmMs/toolMs/ttftMs/ttftSteps`，不属于第一枚 pill 的最低字段集。投影必须随 `session.load` 的会话身份返回；若支持生成中实时更新，WS 更新还需沿用同一会话的 `journalEpoch/seq` 归约，切会话不得保留上一会话数字。

C-STATS 合并后的 golden 至少覆盖：

1. 同会话 tail=1 与 tail=50 的 whole-log turns/steps/usage/cache 完全一致；压缩只让 context pressure 按真值回落。
2. 两个 assistant step 共享一个 turn 时 `turns=1, steps=2`；tool 数变化不改变 steps。
3. 缓存分母三项任一缺失即隐藏；真实 0% 与 100% 可显示；999/1000 显示 99.9%，不能变成 100%。
4. used/window 缺任一即隐藏；真实 0% 可显示，超过容量封顶 100%。
5. A/B 会话切换与 stale epoch 后不显示对方或上一世代的数据。

## 5. 先红后绿与实图证据

### 红灯

- `plugins/dsstatusbar/testdata/fg5-red.log`：新官方统计/几何/连接测试先写时，相关类型和函数尚不存在，`go test ./plugins/dsstatusbar` **exit 1**。
- `plugins/dsstatusbar/testdata/fg5-red-unknown-zero.log`：补“未知 input/output 不补零”用例后，旧逻辑实际输出 `↑100 ↓0`、`↑0 ↓50`，**exit 1**；随后才改生产逻辑。

### 绿灯命令

```bash
go test -count=1 ./plugins/dsstatusbar
# ok github.com/MYCF711/dsh-ui/plugins/dsstatusbar；exit 0

go vet ./plugins/dsstatusbar
# vet_exit=0

CGO_ENABLED=0 go build -o /tmp/fg5-app.exe ./cmd/app
# build_exit=0
```

机器可读/原始终端记录：

- `plugins/dsstatusbar/testdata/fg5-test.log`
- `plugins/dsstatusbar/testdata/fg5-visual.log`
- `plugins/dsstatusbar/testdata/fg5-vet.log`
- `plugins/dsstatusbar/testdata/fg5-app-build.log`

第一次 app build 读到并行 FG-3 的 `dstopbar` 中间态，报 `undefined: label`；未修改越权文件。该 owner 更新后按当前工作树重跑，最终结果以上面的 `fg5-app-build.log` **exit 0** 为准。

### Viva 无头真实布局

测试使用真实 `ds-composer + ds-statusbar + ds-core + flex`，不是静态拼字；三帧均断言无伪轮/步、无缓存率、无上下文百分比，并检查 dock 高 26、位于卡片后、中心轴相同和连接点颜色。

| 证据 | 实测 |
|---|---|
| [1920×1032 正常连接](../plugins/dsstatusbar/testdata/fg5-dock-1920x1032.png) | dock `x=12 y=970 w=1896 h=26`；cardCenter=960，dockCenter=960 |
| [1182×832 正常连接](../plugins/dsstatusbar/testdata/fg5-dock-1182x832.png) | dock `x=12 y=770 w=1158 h=26`；cardCenter=591，dockCenter=591 |
| [1182×832 断线](../plugins/dsstatusbar/testdata/fg5-disconnected-1182x832.png) | 同一 dock 几何；红点“已断开”，状态“连不上宿主，重试中…” |

复跑并保留图片：

```bash
FG5_CAPTURE_DIR="$PWD/plugins/dsstatusbar/testdata" \
  go test -count=1 -run TestFG5HeadlessDockAndConnection -v ./plugins/dsstatusbar
```

## 6. 交接给 Lead

1. 先冻结并接入 C-STATS，再点亮 Activity、缓存率和 ContextMeter；不要从现有窗口消息或现有 `UsageTotal` 反推缺失语义。
2. 若目标包含官方第三方 dock 注册、右侧 ContextMeter seat 或移除 24px 连接行，按 §3 的共享集成点串行实施；本包没有假装完成这些部分。
3. 真宿主验收必须补 tail=1/50、A/B、压缩回落和 stale epoch；本包的组件实图不替代这些契约证据。
