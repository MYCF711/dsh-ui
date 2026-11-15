# Cordis 动态插件面板（rc.2 证据，账本 #12）

> 权威原文：官方 rc.2 随包 `src/**`（`OFF:` 前缀指向
> `DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`），宿主侧契约指向
> `DSH-GO` 分支 `feat/host-cordis-rest`（PR #50）的 `cmd/dshd/serve_cordis_api.go`。
> 本文只写**已落地**的东西与**登记在案**的 HIDE / 差异；没有落地的每一条都写清依据。

## 1. 官方形状（逐处）

`dsh-client-ui-cordis` 是"动态生成插件的**历史卡片** + 一块**全局控制面板**"，
它自己**不注册任何工具**（`README.md:119`「This package registers no tool or prompt」）。

| 面 | 官方座位 | 形状 | 锚点 |
|---|---|---|---|
| 面板触发器 | `sidebar.footer.action`（list / root） | 一枚 42px 行：插件图标 + `panel.trigger` + 运行计数；`data-cordis-badge` / `data-cordis-approval-badge` | `src/client/index.ts:86-114`、`CordisPanel.tsx:485-504` |
| 面板本体 | 同上（`createPortal` 到 body） | `position:fixed`，宽 420 / `max-width:calc(100vw-24px)` / `max-height:60vh`；header 44px（`padding:10px 12px`），body `padding:0 12px 12px` 且**只有它滚动** | `CordisPanel.module.css:84-131` |
| 行 | 面板内 `<li>` | `padding:14px 12px 10px` / gap 8 / `border:0.5px` / radius 12；四段：rowHead（id+name+状态）、versionPicker（`packages.length>1` 才有）、rowDetail（purpose + 动作按钮）、transition（`nextPackageId` 才有） | `CordisPanel.module.css:171-272`、`CordisPanel.tsx:251-453` |
| 动作按钮 | 行内 | 28×28 圆形、`gap:10`、`color:label-tertiary`、hover 换 `interactive-bg-hover` | `CordisPanel.module.css:318-345` |
| 状态点 | rowHead 行尾 | `idle`→idle / `client-pending`→ongoing / `running`→done / `awaiting-approval`→warning / `failed`→error | `CordisPanel.tsx:77-86` |
| 4 张历史工具卡 | `tool.call.toolview` keyed（`cordis_define` / `cordis_run` / `cordis_stop` / `cordis_undefine`） | 从**已记录的** tool/call + tool/result 帧派生（`card-model.ts:95-160`） | `src/client/index.ts:117-145` |
| Package 业务视图 | `tool.view.cordis`（keyed / session，`CordisRunRow` 的 children） | 动态 `client.js` 里的 **React 组件** | `slots.ts:23-37`、`CordisRunRow.tsx:119-130` |

### 1.1 状态派生（面板 / 卡片的唯一算法）

```
cordisVisibleStatus(row, packageId, loaded)          status.ts:31-45
  activeRun 不存在 或 packageId != activeRun.packageId  → 'idle'
  pkg.hasClientHalf !== true                            → 'running'
  loaded 含 (pluginId, packageId, pluginRunId)          → 'running'
  否则                                                  → 'client-pending'

visiblePanelStatus(row, selectedPackageId, loaded)    CordisPanel.tsx:62-75
  activity.phase=='awaiting-approval' || latest.status=='awaiting-approval' → 'awaiting-approval'
  latest.status=='failed' && latest.packageId==selected                      → 'failed'
  activeRun 不存在                                                            → 'idle'
  否则 → cordisVisibleStatus(...)
```

`loaded` 是"**本页已加载的 Client 半**"（`DynamicCordisLivePackage[]`）——它是页面自己的
状态，不是宿主事实。

### 1.2 数据从哪来（宿主侧已对齐官方，PR MYCF711/dsh-go#50）

| 面 | 宿主方法 | 官方形状锚点 |
|---|---|---|
| 清单 | `dynamicCordisRunner.inventory` | `dsh-cordis-host-runner/src/types.ts:233-251` |
| 停止 | `dynamicCordisRunner.stopFromPanel(agent, pluginId)` | `:308-310` |
| 移除 | `dynamicCordisRunner.undefineFromPanel(agent, pluginId)` | `:254-256` |
| 起 Host 半 | `dynamicCordisRunner.runHostHalf(agent, pluginId, packageId, mode, requestId, approveFutureVersions)` | `:313-323`；zod 入参 `lib/typert.host.js:146-157` |
| 两条真事件帧 | `cordis/dynamic-package`（激活成功）/ `cordis/dynamic-retract`（stop / undefine 撤下） | `HOST:cmd/dshd/serve_cordis_api.go:440-455` |

**本宿主如实缺席**（界面必须能处理空值，不许补默认值）：

| 字段 | 本宿主的真值 | 依据 |
|---|---|---|
| `agentId` | **可能是空串** | 没有 define 通道；归属只来自激活/面板调用时的 `sessionId`。空串按官方语义落「其他会话」 |
| `packages[].purpose` | 恒空串 | 盘上 `package.json` 没有这个字段 |
| `packages` | 恒 1 项 | 一个插件目录只有一个版本 |
| `nextPackageId` | 不出现 | 本宿主不做失败/进行中的版本切换 |
| `latestRun.approvalRequestId` / `requiresApproval` | 不出现 | 没有 Cordis 审批队列 |
| `host.waitingFor` / `client.waitingFor` | 恒 `[]` | 没有 Cordis 服务/Fiber 依赖图；与官方 `missingFor()` 在"不缺"时返回 `[]` 同义 |
| `currentPackageId` | 只在**结算成功**（或无 Client 半的原位 commit）后出现 | `client-pending` 期间不推进 |
| `cordis/request-run`、`request-run-resolved` | **不发** | 同上，没有审批队列 |

## 2. 本仓落点

| 东西 | 文件 | 说明 |
|---|---|---|
| 只读数据面契约 | `pluginapi/cordis.go`（`CordisView`）+ `pluginapi/plugin.go`（`Snapshot.Cordis`） | 官方 `CordisInventorySnapshot` 的投影：`Known` / `Rows` / `Removed` / `Error` / `EventSeq` |
| 生命周期动词 | `pluginapi/plugin.go` 的 `Actions.CordisRun / CordisStop / CordisRemove / CordisRefresh` | 官方面板三个 `RowAction` + `onRefresh`；`nil` = 壳没接 → 不画对应按钮 |
| 客户端 | `internal/apiclient/cordis.go` | 4 个方法 + 官方 wire 类型投影；业务失败（`ok:false`）保留在结果里，不折成 error |
| 壳侧数据面 | `cmd/app/cordis_wiring.go` | 全局清单缓存 + `cordis/*` 事件帧水位 + 三个动词的 goroutine 落地 |
| 面板本体 | `plugins/dscordis/plugin.go` / `panel.go` / `status.go` / `locale.go` / `geometry.go` / `register.go` | 两个 painter：`sidebar.footer.action`（触发器，Order 60）+ `shell.overlay`（面板浮层，Order 46） |
| 断言 | `plugins/dscordis/panel_test.go`、`cmd/app/cordis_wiring_test.go`、`internal/apiclient/cordis_test.go` | 见 §5 |

### 2.1 事件怎么驱动界面（真的走 WS，不是轮询）

```
宿主 publish(emitFrameAt) → 归属会话 Transport journal → GUI WS / session.history
  → cmd/app 的 frames[]（Snapshot.Frames）
  → cordisWatchEventsLocked 比 seq 水位 → 触发 cordisRefresh(force)
  → dynamicCordisRunner.inventory → m.cordis.rows → Snapshot.Cordis
  → plugins/dscordis 面板下一帧画新值
```

官方同形状：`ctx.remote.$on('cordis/dynamic-package', () => inventory.refresh())`
（`src/client/index.ts:75-80`）——事件里**不带标签**，只能整份重读。

**如实登记的局限**：事件只进**归属会话**的 journal，本仓只订阅当前会话 →
别的会话的激活事件要等切过去才看到。缓解两层（不是听天由命）：面板每次打开、
以及每次生命周期动作之后都主动重读一次清单。

## 3. HIDE 清单（逐条，附依据）

| # | HIDE 的东西 | 依据 |
|---|---|---|
| 1 | **`tool.view.cordis` 业务视图** | 官方是动态 `client.js` 里的 React 组件（`slots.ts:23-37`、`CordisRunRow.tsx:119-130`）。Go+Viva **没有 JS 组件宿主**：本仓不执行插件包里的 JS，也就没有组件可挂。画一个空框 = 假能力 |
| 2 | **4 张 `cordis_*` 历史工具卡** | 唯一数据源是**已记录的** tool/call + tool/result 帧（`card-model.ts:95-160`），而本宿主注册 **0 个 cordis 工具**（`HOST:docs/host-stable-gap-register.md:412`），官方 rc.2 自己也不注册（`dsh-cordis-host-runner/README.md:119`）。没有生产者 → 这些座位永远空着 → 不注册 painter |
| 3 | **Client 半的加载与结算**（`getClientCode` + `settleUserRun`） | 没有 JS 宿主就没有"页面加载回执"；调用 settle 就是**编造成功**。结果：有 Client 半的包激活后永远停在 `client-pending`——面板如实显示这一态，不写成 running |
| 4 | **审批（approve / decline 两个按钮 + `awaiting-approval` 态）** | 宿主没有 Cordis 审批队列，`cordis/request-run` 不发、`approvalRequestId` 不出现。按钮画出来也无处可点 |
| 5 | **"运行"按钮对"有 Client 半的包"** | 官方 `onRun → startUserRun` 会连带加载 Client 半并结算；本仓只能起 Host 半，起了就永久停在 client-pending。所以运行按钮只对 `hasClientHalf == false` 的包画 |
| 6 | **渲染失败那一行**（`.rowError` 的 render 分支） | 本页不加载任何动态组件 → 不会有渲染失败可报（`reportRenderFailure` 也不调） |

## 4. 登记在案的差异

- **面板定位**：官方是 `position:fixed` + 开局量触发器矩形（`CordisPanel.tsx:140-151`）。
  本仓浮层画在 `shell.overlay` 上，按官方同一套锚点参数放置；viva 没有
  `anchor()`/`position:fixed`，锚点偏移用**常量推导**（与 `plugins/dsagentteam`
  的 catalog 面板同一做法），登记为几何近似。
- **`purpose` 恒空**：面板那一格就空着（官方对空串也是空着，`purpose.missing` 只在
  Define 卡里用——那张卡本仓 HIDE）。
- **`agentId` 空串**：按官方分组判据 `agentId === 当前会话 id` 落「其他会话」，
  **不猜归属**。
- **版本选择器 / transition 块**：条件照官方写，但本宿主的数据永远不会命中
  （`packages` 恒 1 项、`nextPackageId` 不出现）——不为此编状态。
- **图标**：官方用 `IconCordisPluginOutlineRegular` / `IconPlayOutlineRegular` /
  `IconStopFillRegular` / `IconTrashOutlineRegular` / `IconCheckOutlineRegular` /
  `IconCloseOutlineRegular`。本仓无该 sprite 时**只留 14×14 槽宽、不手画 SVG**
  （与 `plugins/dsagentteam` 既有做法一致）。

## 5. 断言（防回退）

| 判据 | 测试 |
|---|---|
| inventory 官方行形状 + 缺席字段保持零值 | `internal/apiclient/cordis_test.go` |
| 面板路径用 `sessionId`、`runHostHalf` 用 `agentId` 且不发 `requestId`/`approveFutureVersions` | 同上 |
| `cordis/*` 事件帧触发重读、同一帧重放不重复读 | `cmd/app/cordis_wiring_test.go` |
| 读失败保留旧清单 + 记宿主原文 | 同上 |
| `stop` 的 `not-running` 折算成功、`plugin-missing` 才报错 | 同上 |
| `undefine` 成功后那一帧行立刻消失 + removed 粘性 + 重新出现摘除 | 同上 |
| 面板两态：`nil` 整块不画 / `Known=false` 画「读取中…」不画「还没有定义任何插件」 | `plugins/dscordis/panel_test.go` |
| 分组按 `agentId`（含空串落「其他会话」）、动作按钮显示条件 | 同上 |
