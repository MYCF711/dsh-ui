# `dsh-client-ui-plan`（#28）落地证据 · rc.2

> 2026-11-15 · 配合 [official-parity.md](official-parity.md) 的官方对账表读。
>
> 口径：`OFF:<包简称>/<src 相对路径>:<行号>`，`OFF` 根 =
> `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`；`HOST:` = 宿主仓 DSH-GO。
>
> **结论：🟡 PARTIAL —— 官方四个座位里两个真实现、两个按 HIDE 登记（缺数据/前置，不自造）。**

---

## 1. 官方这个包到底是四个座位

官方客户端半边（`OFF:plan/src/client/index.ts:52-118`）一共注册五处，去重后是四个可见座位 + 一个资源提供器：

| # | 官方座位 | 官方做什么 | 本仓 |
|---|---|---|---|
| ① | `conversation.chat.turnTail`（list/session，`OFF:index.ts:79-84`）| 每轮尾部的**计划卡**：图标槽 + 标题 + `计划 · Markdown` + `打开`，点开在右栏读全文（`OFF:PlanCard.tsx:43-62`）| ✅ `plugins/dsplan/cards.go` |
| ② | `sidebar.right.pane.tab`（keyed/session，key=`@deepseek-ai/dsh-client-ui-plan`，`OFF:index.ts:109-111`）| 右栏**计划全文**（完整 Markdown，`overflow:auto`；`OFF:PlanPreview.tsx:20-40`）| ✅ `plugins/dsplan/preview.go`（key=`plan`，见 §4 差异 2）|
| ③ | `conversation.input.plan`（single，`OFF:index.ts:115-129`）| 计划模式 chip：`useProjection('plan')` 为真时出现，点一下跑 `/plan off`（`OFF:PlanModeControl.tsx:22-62`）| ❌ HIDE，见 §2.1 |
| ④ | `conversation.plan-review.actions`（list/session，`OFF:index.ts:85-101`）| 待批计划的「查看全文」链接，挂在 ui-user-questions 的 plan-review 提问卡里（`OFF:user-questions/src/client/index.ts:390`）| ❌ HIDE，见 §2.2 |
| — | `ctx.resources.register(planResourceProvider)`（`OFF:index.ts:66`）| 按 `dsh-resource://plan/<session>/<callId>` 从会话历史里翻出那次调用的正文（`OFF:plan-resource.ts:33-70`）| ✅ 同一份事实的直接读取：`plugins/dsplan/plan.go`（见 §3）|

## 2. 两个 HIDE：缺的是数据，不是界面

### 2.1 `conversation.input.plan`（计划模式 chip）

官方这一格读的是宿主的 **`plan` 会话投影**：

```ts
// OFF:dsh-plan-mode/src/types.ts:21-24
export interface PlanProjection { active: boolean; pending: boolean }
```

`active` 由日志里的 `plan/mode` 事件折叠，`pending` 由 `/plan` 命令生命周期折叠
（同文件 `:26-41` 的 `PlanUnitState` 注释逐字写着）。**本仓宿主没有这个投影**：

- `session.values` 的投影清单里没有 `plan`（`HOST:internal/transport/handlers_session_official.go:930-987`
  只有 title / actionCards / todos / subagent / subagentTiming / workingDirectory /
  permissions / contextPressure / modelSelection / agentPreset / subagentCatalog）；
- 宿主也没有 `/plan` 命令与 `plan/mode` 事件：计划模式是**工具闸门 + 落盘状态**
  （`HOST:modules/tool/plan/plan.go:1-30`、`:104-140`，「按会话落盘 `<home>/state/plan/<id>.json`」），
  客户端拿不到那一份。

缺了 `{active,pending}` 就画不出"现在是不是计划模式"，更没法决定 chip 出不出现——
按仓规 HIDE，不拿别的东西冒充（例如拿最后一条 `enter_plan_mode` 调用推断"当前状态"，
那是猜，且退出计划模式的路径不止一条）。

### 2.2 `conversation.plan-review.actions`（待批计划的「查看全文」）

官方这个链接**不是独立卡片**，它挂在 ui-user-questions 的 composer 接管里：

```ts
// OFF:dsh-client-ui-user-questions/src/client/index.ts:382-393
ctx.slots.inject('conversation.composer', () => ctx.slots.register({
  name: 'conversation.composer',
  select: ({pendingInteraction}) => pendingInteraction instanceof PendingQuestion ? pendingInteraction : null,
  children: {'conversation.plan-review.actions': {kind: 'list', scope: 'session'}},
}, QuestionComposer))
```

而且它的 props 来自那条 `intent.kind === 'plan-review'` 的待答问题（同文件 `:9-11`：
"One entry, two presentations: … a `plan-review` intent as the plan decision card"）。

**本仓没有那条提问卡**：`plugins/dsquestions` 只认 `ask_user_question`
（`plugins/dsquestions/questions.go:30-52`），宿主 `exit_plan_mode` 也**不产生问题**——
它直接 `Result{Text: 方案正文, Stop: true}` 结束这一轮（`HOST:modules/tool/plan/plan.go:218-240`）。
没有父卡片，这个链接就无处安放 → 注册上去也永不出现 → HIDE，登记在此。

> 这一格的缺主在**宿主**（要不要把 `exit_plan_mode` 变成一个带 `plan-review` intent 的待答问题），
> 不在本包。做法是宿主侧加一条待答问题或事件，然后再回补这里。

## 3. 数据路径：为什么不需要新宿主方法

官方 `planResourceProvider` 从**会话历史**里逐页翻找那次调用：

```ts
// OFF:plan-resource.ts:45-58
const first = await remote.events({address: sessionAddress}, signal)
const throughSeq = first.value.cursor
… for (const entry of page.records) { const plan = submittedPlan(entry.event); if (plan?.callId === target.callId) {…} }
```

`submittedPlan` 的判据（`OFF:plan.ts:33-48`）是：事件名 ∈ {`tool/call`, `tool/ptc-dispatch-start`,
`tool/ptc-dispatch`}、`data.name === 'exit_plan_mode'`、`data.arguments` 解析出 `plan` 字符串、
markdown 第一行有一级标题。

宿主写的是同一份事实：`HOST:internal/session/log.go:168-176` 的 `tool/call` 帧带
`turn/step/callId/name/arguments`（`arguments` 是模型给的 JSON **文本**）。而本仓客户端
**已经把这段帧缓存好了**：`pluginapi.Snapshot.Frames` 就是 `session.history` 的 `frames[]`
（`pluginapi/plugin.go:461-473` 的注释逐字说明）。

所以本包**一个宿主方法都没加**，`testdata/api.snapshot.json` 的 `counts`/`methods` 不变。

## 4. 与官方的差异（逐条登记，不假装搬了）

1. **Turn 尾座位是会话级，不是按轮**：官方 `conversation.chat.turnTail` 是 per-turn
   （`OFF:PlanCard.tsx:43-44` 的 `usePlans(String(turn.turn))`）。本仓那个座位壳每帧只派发
   一次（`internal/uikernel/official_slots.go:428-431` + `shell_skeleton.go:894`），
   所以按「最新一轮」取（`plan.go` 的 `latestTurn`）——与 `plugins/dsactioncards` 的
   同一取舍、同一登记（那份卡片也写着"per-turn 座位"这条没搬）。
2. **右栏 tab 没有 navigation 地址**：官方一份 tab 对应一个 `dsh-resource://plan/…`
   （`ctx.sidebarRight.openResource`，`OFF:index.ts:71-78`）。本仓 keyed 槽的 key 只有
   **类型**（`RightTabPlan = "plan"`，与 `plugins/dsterminal` 的 `RightTabTerminal` 同一现状），
   所以"看哪一份"由插件状态承载：点卡片写入地址（`cards.go` 的 `open`），没点过时退到
   「本会话最新一份计划」——否则用户点开这个 tab 只会看到「计划预览不可用」，而会话里明明有计划。
   这一条是**本仓多出来的一个状态**，官方没有（它的 tab 一定由地址打开）。
3. **图标槽用文字顶替**：官方是 `FileTypeIcon kind="markdown" size={20}`（`OFF:PlanCard.tsx:60`）
   的 SVG sprite；本仓没有那份素材，按仓规不手画 SVG，40×40 槽里放 10px 的 `MD` 两字母——
   与 `plugins/dsattachment/rail.go:169-172` 同一先例。
4. **卡片底色/描边的令牌**：官方 `.card` 用 `--dsw-alias-card-fill` / `:hover` 用
   `--dsw-alias-card-fill-hover` / `.cardIcon` 用 `--dsw-alias-card-tile-fill`
   （`OFF:PlanPreview.module.css:3-5,19`）。`pluginapi.Theme` 没有这三个令牌，用同层的
   `BgLayer1`（卡片）/`BgLayer2`（瓦片）顶替——与 `dsactioncards` 的卡片同一映射（那里也是
   `Background(t.BgLayer1)`）。代价：hover 变色不画（viva 的 Box 没有 hover 背景态）。
5. **`.cardOpen` 的 `--dsw-alias-button-floating-fill`** 同样没有令牌，用 `BgLayer2` 顶替。
6. **`sidebar.right.pane.tab.title` 座位不注册**：官方还注册了一个"图标 + 标题"的 tab 标题体
   （`OFF:index.ts:112-114`），但本仓 official 档骨架**不派发**那个槽
   （`internal/uikernel/shell_skeleton.go:1067-1082` 只派发 tab 体），注册了永远不画；
   标题落在 tab 芯片的 `Label`（官方 `title: () => t('preview.title')`）上。
7. **没有 loading / failed / expired 三态**：官方那三种是**异步资源读取**的产物
   （`OFF:PlanPreview.tsx:27-31`：`resource.status === 'none'/'failed'` + 临时预览地址）。
   本仓数据同步可读、也没有临时预览，所以这三格到不了——`zhLoading`/`zhFailed`/`zhExpired`
   常量保留（"文案只用官方的"这一条不破坏），但不画假状态。
8. **正文里的相对路径链接点不开**：只认明确 scheme（http/https/mailto，仓里
   `mdlite.LinkTargetForURL` 的既有判据）。官方把相对路径当工作区文件打开，本仓没有那条解析。
9. **计划模式的 `/plan off` 动作没有**（见 §2.1）：本包不提供"退出计划模式"的入口。
10. **帧里没有 `turn` 时只画最后一份**：`turn` 缺失会让所有计划都落在 Turn 0，
    "最新一轮"就失去意义。此时本包只留最后提交的那一份（把整个会话的计划堆到当前轮
    尾部会错位），而不是全画。宿主 `tool/call` 一直写 `turn`（`HOST:internal/session/log.go:168-176`），
    所以这一条只在老宿主/夹具上生效。
11. **切会话时地址按"没选过"处理**：右栏 tab 是全局的，插件状态里的
    `dsh-resource://plan/<旧会话>/<callId>` 换会话后在本会话里没有意义，
    本包不报"找不到"，而是退到本会话最新一份计划（本会话没有 → `计划预览不可用`）。

## 5. 本仓落点

| 文件 | 内容 |
|---|---|
| `plugins/dsplan/plan.go` | 帧解析（`SubmittedPlans` / `submittedPlan` / `planTitle`）、地址编解码（`planAddress` / `parsePlanAddress` / `uriComponent`）、取轮（`latestTurn`） |
| `plugins/dsplan/cards.go` | Turn 尾计划卡（官方几何常量 + 文案 + 点击 → 切右栏 tab/展开右栏） |
| `plugins/dsplan/preview.go` | 右栏计划全文（`MarkdownView` 布局 + AfterDraw 覆画、滚动视口裁剪、链接打开、四种状态） |
| `plugins/dsplan/plugin.go` | 两个座位的注册（`conversation.chat.turnTail` / `sidebar.right.pane.tab`） |
| `cmd/app/main.go` | 一行 blank import（`_ "…/plugins/dsplan"`） |

## 6. 测试与门禁

`plugins/dsplan` **18 条**测试，全部走真实无头绘制（`ui.NewPage` + 真 `gfx.Canvas` + 真主题）：

- 帧解析 4 条：只认 `exit_plan_mode`、七类畸形输入不算计划、PTC 形态可解析、
  标题规则 10 条逐条对官方正则；
- 地址 2 条：往返（含 `:` `/` `?` `#` `$&+,;=:@`）、九种不合约地址；
- 取轮 2 条：只留最新一轮；帧里没有 `turn` 时只留最后一份；
- 卡片 6 条：只画最新一轮、空数据一个字不画、点击 → `SelectRightTab('plan')` +
  右栏收起时展开 + 记住地址、右栏已展开不再 toggle、壳没接动作时仍记地址、无 `turn` 只画最后一份；
- 全文 5 条：Markdown 真渲染、点哪份看哪份、四种状态用官方原文（含切会话回退）、
  视口外正文不画（高度截断）、明确 scheme 链接交给壳打开。
