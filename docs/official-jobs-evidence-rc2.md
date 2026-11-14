# 后台任务（#22 `dsh-client-ui-jobs`）的结构证据与实现台账

> 建立时间：2026-11-14 · 对应 `docs/official-parity.md` 第 22 行。
>
> **这份文件回答四个问题**：
> 1. 官方 rc.2 的 `dsh-client-ui-jobs` 到底是什么结构（组件层级 / slot / CSS 几何 / locale 文案）？
> 2. 官方消费的是哪几个客户端方法、数据形状是什么？
> 3. 本仓实现落在哪些 `file:line`，哪些值是从官方原文学来的？
> 4. 哪几处**与官方不同 / 没做**，各自缺的是什么？

**证据口径**：rc.2 起官方**随包发 `src/**` 原件**（未压缩 TS/TSX + CSS Module + 全语言 locale）。
锚点记法 `OFF:<包简称>/<src 内相对路径>:<行号>`，`OFF` 根 =
`tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`
（gitignored 的本机解包树，与主库 `$DSH_HOME/dsh-ui-official/` 互为参照）。
宿主侧锚点记 `HOST:`，根是宿主仓 `origin/main`。

---

## 0. 官方包是什么（4 个文件，一个 header action）

| 文件 | 作用 |
|---|---|
| `OFF:ui-jobs/src/client/index.ts`（117 行） | 注册 locale + **`conversation.session.header.actions` 的一个 action**（`id: 'job-list'` :39、`order: 20` :41，注册块 :35-60） |
| `OFF:ui-jobs/src/client/JobListAction.tsx`（923 行） | 触发器 + 弹层 + 行 + 输出面板 + 两段式停止；`useJobs/useJobObserved` 只读 `ctx.jobs` 服务（:1-8） |
| `OFF:ui-jobs/src/client/JobListAction.module.css` | 全部几何（本文所有 px 值都出自这里，行号逐条注） |
| `OFF:ui-jobs/src/client/locales.ts` | 8 种语言；zh 是本文文案的唯一来源 |

**它自己不含传输状态**：`inject = ['jobs', 'slots', 'locale']`（`index.ts:31`），
`ctx.jobs.watchRows(sessionId)` / `ctx.jobs.observe(sessionId, id)` / `ctx.jobs.kill(sessionId, id)`
（`index.ts:35-60`）。本仓的等价面是壳的 `pluginapi.Actions.JobsList / JobsOutput / JobsStop`。

---

## 1. 数据形状：官方 `JobView` vs 本仓 `jobs.list` 行

官方（`OFF:dsh-jobs/src/view.ts:64-110`）：

```
JobView { id, kind, label, owner?, outputLimitBytes?, status('running'|'stopping'|'completed'|'killed'|'failed'),
          progress?, detail?, startedAt, finishedAt?, output{total, earliest, spillPaths?} }
```

本仓宿主（`HOST:internal/transport/handlers_jobs.go:16-27`，即 `jobs.list` 的一行）：

```
JobInfo { id, sessionId?, command, done, exit_code, bytes }
```

**差集就是"不许画"的清单**：`kind` / `progress` / `detail` / `startedAt` / `finishedAt` /
`output.total|earliest|spillPaths` 本仓都没有 → 本仓不画 kind 角标、不画持续时长、不画
`stopping`/`killed` 两态、不按 `output.total>0` 判断"可展开"。本仓可展开的判据是
**行存在且 `sessionId` 非空**（`HOST:handlers_jobs.go:109-112` 的生产要求），展开后读不到就画错误行。

---

## 2. 触发器（官方 `JobListAction.tsx:568-595`）

| 项 | 官方值 | 出处 |
|---|---|---|
| 高度 | `min-height: 28px` | CSS `.trigger` :1-24 |
| 内边距 / 间距 | `padding: 3px 2px`；`gap: 4px` | 同上 |
| 字号 / 行高 | `12px / 18px`，色 `label-tertiary` | 同上 |
| 活动槽 | `14×14` | CSS `.activitySlot` :26-34 |
| 空闲态图标 | `IconLayersOutlineRegular size=14` | `JobListAction.tsx:587-588` |
| 有活任务 | 活动槽改画 `StateDot state="ongoing"`（文字宽档） | `:589`; CSS `.textModeActivity` |
| 计数文案 | `{count} 个后台任务`；`count = 有活任务 ? 活任务数 : 可见任务数` | `:500-503`；`locales.ts:8-9` |
| 空清单 | `visibleCount === 0 → return null`（**整个触发器不渲染**） | `:500` |
| 交互 | 点击开合；`ArrowDown` 打开；窄容器（≤460px）退化成 28×28 图标按钮 | `:568-595`；CSS `@container` :39-77 |

**本仓**：`plugins/dsjobs/official_jobs.go:drawTrigger`（28/3/2/4/12/18/14 全部按上表；
空闲画 layers 图标、有活任务画 ongoing 点；计数同一条公式；空清单直接 return）。
**没做**：`@container` 窄档（viva 没有容器查询）→ 计数恒显示（登记为 deviation）；
hover 150ms 打开（没有 hover 计时设施）；Tooltip。

图标素材：官方 sprite 表在 `dsh-web-frontend/dist/assets/index-ghOpC9uV.js` 的
`"layers-outline":{size:16,svg:…}` / `"stop-fill":{…}` 段；本仓把 SVG `a`（圆弧）命令按
**标准 arc→cubic（≤90°/段）**换算成 `gfx.PathCommand`（换算脚本：`tmp/GO8-RC2/_gp2/svg2gfx.txt`），
viewBox 保持官方 16，显示尺寸按官方 `size` 传（14 / 12）→ `plugins/dsjobs/icons.go`。

---

## 3. 弹层（官方 `JobListAction.tsx:596-635` + CSS `.menu`）

| 项 | 官方值 | 出处 |
|---|---|---|
| 容器 | `position: fixed`；`width: 500px`；`max-width: min(560px, 100vw-32px)`；`max-height: min(480px, 100dvh-140px)` | CSS `.menu` :84-109 |
| 锚点 | `top: max(12px, anchor(bottom)+5px)`；`left: clamp(16px, anchor(left), …)`；`right: 12px` | 同上 |
| 层级 | `z-index: 100`（"above the right sidebar, below modal overlays"） | 同上 |
| 行为 | 菜单外 `pointerdown` 关闭；`Escape` 关闭（document 级）；点内部**固定**不自动关 | `JobListAction.tsx:411`、`:414-421` |
| 主体 | `.menuBody{display:flex;flex-direction:column;gap:1px;padding:4px;overflow:auto}` | CSS :111-121（`JobListAction.tsx:606`） |
| 段头 | `.sectionHeader{margin:4px 4px 1px;padding:4px 0 3px;border-top:.5px solid border-l1;font-size:11px;line-height:16px}`（第一段无上边框） | CSS :123-140 |
| 段头按钮 | `.sectionToggle/.sectionClear{gap:3px;padding:2px 4px;border-radius:sm;font-size:11px;line-height:16px}` | CSS :142 起；JSX `:613-628` |
| 已结束段 | 折叠按钮 = chevron(12) + `已结束 {count}`；右侧 `清空`；默认展开条件 `settledOpen ?? 活任务数===0` | `JobListAction.tsx:611-628`、`:363` |
| 清空 | 客户端隐藏（`clearedKeys`），注册表记录不动 | `:505-518` |

**本仓**：`official_jobs.go:drawPanel/drawPanelBackdrop/drawSettledHeader`。
修正值：`top` 用官方 header 几何推导 `10 + 30/2 + 28/2 = 39`，再 `+5`（与 `dsagentteam/catalog.go`
同一推导）；`right` 贴 16px（官方是 `left: clamp(...)` 的锚点，viva 无 CSS anchor）；
遮罩是全窗透明命中区（官方 document pointerdown 的等价物）；`Escape` 走插件 `HandleKey`。
**没做**：hover 自动开合、`position-try-fallbacks`（窗口贴边翻转）、焦点环路（Tab 桥接）、
`sectionChevron` 的 CSS 旋转（viva 的 `gfx.Icon` 没有旋转变换 → 用同尺寸三角形文字，登记为 deviation）。

---

## 4. 行（官方 `JobListAction.tsx:180-330` + CSS `.rowLine/.row`）

| 项 | 官方值 | 出处 |
|---|---|---|
| 行容器 | `display:flex;gap:6px;align-items:stretch` | CSS `.rowLine` :187 起 |
| 活任务卡 | `padding:4px 8px 4px 2px;border-radius:md;background:fill-l2` | CSS `.rowLineLive` :196 起 |
| 行本体 | `min-height:28px;padding:4px 7px;gap:6px;font-size:12px;line-height:17px;border-radius:md` | CSS `.row` :311 起 |
| 已结束行 | 文字色 `label-tertiary` | CSS `.rowSettled` :350 起 |
| 主标签 | 等宽、单行省略（`.label{font-family:mono;ellipsis}`） | CSS `.label` :391 起 |
| 状态 | `.status{font-size:10px;line-height:16px;label-tertiary;max-width:60%}` | CSS `.status` :402 起 |
| 时长 | `.duration{font-size:11px;line-height:18px;tabular-nums}` | CSS `.duration` :413 起 |
| 展开箭头 | `.chevronBox{20×20;border:.5px solid border-l2;border-radius:sm;background:menu}` + `IconChevronDownOutlineRegular size=12`（CSS 旋转 -90°/0） | CSS `.chevronBox` :236；JSX :221/:235 |
| 状态点 | `StateDot`（running/stopping/completed/killed/failed） | `JobListAction.tsx:209`、`:228` |

**本仓**：`official_jobs.go:drawOfficialItem`（6 / 4-8-4-2 / 28 / 4-7 / 12-17 全部按上表；
活任务有填充卡；已结束行用 tertiary）。**不画** kind 角标、时长、`stopping`/`killed`
（宿主没有这些真值，见 §1）；状态点用 `prims.Dot`，颜色按 done/exit_code 映射
（运行中=link、正常结束=success、非零=error——官方五态色板在主题里只有这三档可用，
登记为 deviation）。展开箭头用同尺寸三角形文字（同上）。

---

## 5. 输出面板（官方 `JobListAction.tsx:290-309` + CSS `.panel`）

| 项 | 官方值 | 出处 |
|---|---|---|
| 位置 | 行下方（`<li class=item>` 内） | `:290-309`（`TerminalBlock` :297） |
| 外边距 | `.panel{margin:0 8px 8px}` | CSS :430 |
| 输出区高度 | `--dsl-terminal-output-max-height: 288px` | CSS :437 |
| 内容槽 | 终端块 `--dsl-terminal-gutter: 12px` | CSS :445 |
| 断流 / 空 / 丢弃 | `output.error` = `实时输出流中断：{error}`；`terminal.noOutput` = `（无输出）`；`output.gap` = `……较早的输出已丢弃……` | `locales.ts:30-31`、`:20` |
| 观察流 | 展开 = `ctx.jobs.observe(sessionId, id)`（**流**，收起即停） | `:438-448` |

本仓宿主没有 NDJSON 观察流，等价物是 **unary 分页** `jobs.output`：
`cursor/nextCursor/hasMore/total/truncated`，单位 **rune**（`HOST:handlers_jobs.go:87-160`，
`DefaultJobOutputLimit=16*1024`、`MaxJobOutputLimit=64*1024`、越界游标 `ErrBadJobCursor`）。
所以本仓是"按需读页 + 加载更多"，不是持续推流——**这是与官方的接口形状差异**，不是少画了个按钮。

**本仓**：`jobs.go:drawOutputBlock`（288 高由官方弹层传、侧栏 220；`truncated` 单独一行、
读失败画 `实时输出流中断：…` + 重试、空输出画 `（无输出）`、`hasMore` 画 `加载更多`、
`复制` 走 `Actions.Copy`）。**没做**：`stdout/stderr/log` 通道着色（宿主 `jobs.output` 只回一段
文本，没有 `JobChunk.channel`）；`gapBefore` 的字节级缺口标记（只有整体 `truncated`）。

---

## 6. 停止（官方 `JobListAction.tsx:474-499` + CSS `.stop`）

| 项 | 官方值 | 出处 |
|---|---|---|
| 按钮 | `20×20`、`border:.5px solid border-l2`、`border-radius:sm`、`background:menu` | CSS `.stop` :250 |
| 图标 | `IconStopFillRegular size=10`（实心方块） | `JobListAction.tsx:280-287` |
| 两段式 | 第一击 `armed`；3 秒内第二击才发 `killJob`；`KILL_ARM_MS=3_000` | `:55`、`:481-499` |
| 确认态 | 变宽成带字 pill（`padding:0 7px;gap:4px;error 45% 描边 + 12% 底`） | CSS `.stopArmed` :280 |
| 失败态 | `停止失败`（`KILL_FAILED_MS` 后自动消失） | `:58`、`:244-247` |
| 可停判据 | 只有 `status==='running'` 的行有按钮 | `:244-285`、`:474-479` |

**本仓**：`drawOfficialStop`（20×20 / 两段式 / 3 秒窗口 / 确认态用错误色描边 + 抬高一层的底色，
因为 viva 没有 `color-mix`）。**没做**：`pending`/`failed` 两个行内态——`Actions.JobsStop`
是 fire-and-forget，失败只写状态栏（没有结果回来就不画"停止失败"，否则是编事实）。
可停判据 = `!done && sessionId != "" && Actions.JobsStop != nil`。

---

## 7. 本仓落点（含旧三栏档）

| 面 | 文件 | 说明 |
|---|---|---|
| 官方槽触发器 + 弹层 | `plugins/dsjobs/official_jobs.go`、`icons.go` | `conversation.session.header.actions` order 20；`shell.overlay` order 45 |
| 旧三栏档清单 | `plugins/dsjobs/jobs.go` | `sidebar.footer` order 70（derived；官方子槽在该档不被 dispatch） |
| 客户端方法 | `internal/apiclient/jobs.go` | `jobs.list`（含 `sessionId`）/ `jobs.output` / `jobs.stop` |
| 动作接线 | `cmd/app/jobs_actions.go`、`cmd/app/main.go` | 输出累计缓存 + 后台拉取 + 停止异步 + 状态栏 |
| 契约快照 | `testdata/api.snapshot.json` | `jobs.output` 形状更新（rune 游标）、新增 `jobs.stop` |
| 守卫测试 | `plugins/dsjobs/*_test.go`、`cmd/app/jobs_actions_test.go`、`internal/apiclient/jobs_test.go` | 真绘制 + 真 HTTP + 快照计数 |

**必须一起读的一条现状**：本仓默认 `-shell auto` 会落到旧三栏（`dsshell` 占着 `shell.root`）；
即使 `-shell official`，`shellOptionsFor` 的 `BridgeLegacy` 判据是
`kernel.Has(sidebar.list) || kernel.Has(main.body)`（`cmd/app/shell.go:137-139`），
旧部件未迁完时为真 → **`conversation.session.header.actions` 这一帧不会被 dispatch**。
所以今天的可见入口是 `sidebar.footer` 清单（同样能读输出、能停）；
官方 header 触发器在旧部件迁走、兼容桥关掉之后自动生效（`officialSlotsLive()`
每帧读 `ShellTier`，不是登记时写死）。

---

## 8. 与官方的差异台账（逐条，全部是"本仓能力面不足"而不是"没看官方"）

| # | 差异 | 原因 | 能否消 |
|---:|---|---|---|
| 1 | 无 kind 角标 / 持续时长 | 宿主 `JobInfo` 没有 `kind` / `startedAt` / `finishedAt` | 要宿主补字段 |
| 2 | 无 `stopping` / `killed` 两态 | 宿主不暴露 `JobView.status` | 要宿主补状态机 |
| 3 | 输出是**分页拉取**不是**持续观察流** | 宿主没有 `jobs.watch`/chunk 流；只有 unary `jobs.output` | 要宿主补流式方法 |
| 4 | 无 stdout/stderr 通道色、无字节级 gap | 宿主 `jobs.output` 只回一段文本 + `truncated` | 要宿主补 `JobChunk` |
| 5 | 停止失败无行内红字 | `Actions.JobsStop` 不回结果（壳写状态栏） | 要壳把结果回传（additive 动作面） |
| 6 | 无 hover 150ms 开合 / 无焦点环路 / 无窗口贴边翻转 | viva 无 hover 计时、CSS anchor、`position-try-fallbacks` | 可做但价值低（登记） |
| 7 | 计数在窄容器不隐藏 | viva 没有 `@container` | 可做（宽度判断），未做 |
| 8 | 展开箭头是文字三角形 | `gfx.Icon` 无旋转变换 | 可做（预旋转一套路径），未做 |
| 9 | 面板右贴 16px 而非 `clamp(anchor)` | 无 CSS anchor | 可做（按 header 实际矩形算），未做 |

**结论**：`#22` 的两个真能力（读输出、停止）已接上并有生产调用者，
官方槽位（header action + 弹层）与旧三栏档都有真入口；剩下 9 条差异全部登记为
**宿主能力面不足或渲染器能力不足**，不是漏看官方结构。
