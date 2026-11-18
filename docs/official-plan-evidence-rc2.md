# `dsh-client-ui-plan`（#28）落地证据 · rc.2

> 2026-11-17 更新；配合 [official-parity.md](official-parity.md) 阅读。
>
> `OFF` = `DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`；`HOST` = DSH-GO。
>
> **结论：🟡 PARTIAL。四个可见座位均有真实数据与动作，plan-review 与 `/plan off` 已闭环；剩余差异在 §4，不把这两项继续记成 HIDE。**

## 1. 官方座位与本仓落点

| 官方座位 | 官方依据 | 本仓 |
|---|---|---|
| `conversation.chat.turnTail` | 每轮尾计划卡，`OFF:dsh-client-ui-plan/src/client/PlanCard.tsx:43-60`；座位一轮一次（`OFF:.../ui-chat/src/client/chat/register-node-renderers.ts:88-97`，owner props `contract/slots.ts:60-64`） | `plugins/dsplan/cards.go` + `pluginapi/turn_tail.go` + `internal/uikernel/{turns.go,shell_skeleton.go}`（2026-11-18 按轮派发） |
| `sidebar.right.pane.tab` | 右栏完整 Markdown，`OFF:PlanPreview.tsx:18-40` | `plugins/dsplan/preview.go`（既有 + 临时 review） |
| `conversation.input.plan` | 真实 plan 投影，点击 `/plan off`，`OFF:PlanModeControl.tsx:19-68` | `plugins/dsplan/chip.go`（本轮） |
| `conversation.plan-review.actions` | 自动一次/手动「查看全文」，`OFF:PlanCard.tsx:62-82`、`OFF:index.ts:97-108` | `plugins/dsplan/review.go`（本轮） |
| plan-review 父卡 | 单个 `intent.kind=plan-review` 问题，`OFF:dsh-client-ui-user-questions/src/client/contract/slots.ts:69-101` | `internal/planreview/` + `plugins/dsquestions/plan_review.go`（本轮） |
| plan 资源 | 从历史中的 exit 调用取计划，`OFF:plan.ts:33-48`、`OFF:plan-resource.ts` | `plugins/dsplan/plan.go`（既有） |

## 2. `/plan off` 闭环

官方 chip 只认宿主投影（`OFF:dsh-plan-mode/src/types.ts:21-24`），目标态为
`pending ? !active : active`（`OFF:PlanModeControl.tsx:32-34`）；点击执行
`remote.commands.execute(sessionId, '/plan off', [])`，`undefined` 表示 unknown command，
错误进入原位错误行（`OFF:index.ts:116-126`）。官方命令的附件拒绝与退出语义见
`OFF:dsh-plan-mode/src/index.ts:232-279`。

本仓链路：

1. `internal/apiclient/projections.go` 严格解 `{active,pending}`，缺键/null/坏类型=未知；
   `cmd/app/wave3_wiring.go` 按会话缓存，`Snapshot.Plan` 每帧交给 dsplan。
2. `dsplan/chip.go` 只在目标态为真且 `ExecuteCommand` 已接线时绘制；点击后台执行
   `/plan off`，leaving/locked 防重入，失败画官方 `chip.exitFailed`，成功等下一份宿主投影消失。
3. `internal/apiclient/agent_actions.go` 区分 Transport 错误、`result.kind=error` 与
   null/undefined；null 在壳上变成 `unknown command: /plan off`，不伪报成功。
4. 宿主注册 `plan` 命令。`/plan off` 真实删除 `<home>/state/plan/<session>.json` 并追加
   durable `plan/mode{active:false}`；日志失败回滚状态，重复 off 不伪造第二条事件。
   `command/run` + `plan/mode` + `command/done` 后投影收敛 `{active:false,pending:false}`。
   `/plan on` 未接真实 steer，明确 `not_implemented`，不冒充本轮完成。

## 3. plan-review 闭环

官方 exit 工具**提交时不清状态**，而是提出单个问题：id=`plan-review`、detail=计划 Markdown、
`intent={kind:'plan-review',approve:'Approve',callId}`、Approve/Keep planning 两项
（`OFF:dsh-plan-mode/src/index.ts:72-84,320-362`）。父卡是 warn strip + 摘要 + footer，
批准逐字回传选项 label（`OFF:dsh-client-ui-user-questions/src/client/PlanReviewPanel.tsx:48-90`）。

本仓链路：

1. 宿主 `exit_plan_mode` 校验 active、Markdown 一级标题与真实 callId，返回 `Stop=true`
   和持久 `view.questions + awaiting_answer`；active 保持 true。agent 工具循环通过 ctx
   绑定 callId，重载会话仍能从 tool/result.view 恢复同一问题。
2. `internal/planreview` 按官方 narrowing 判定；`dsquestions/plan_review.go`
   画父卡。「同意执行」经 `userQuestions.answer` 回传
   `{answers:[{id:'plan-review',selected:['Approve']}]}`；宿主先真实清状态并追加
   `plan/mode`，再把答案送入真实 prompt 通道。Keep planning 与非空 custom 保持 active。
   「要求修改」只本地关闭卡片，输入卡仍在，用户反馈作为下一 prompt，不替人提交。
3. `dsplan/review.go` 自动打开一次并保留手动「查看全文」。有 callId 打开 durable
   `dsh-resource://plan/...`；无 callId 才用 `dsh-resource://plan-review/<session>/<window:request>`
   与内存正文，参数缺失时显示官方 expired。手动收起右栏后不会被下一帧重新自动打开。
4. 回答通道复用并泛化宿主 `userQuestions.answer`：durable tool/result 之后已有 user/message
   或已被另一客户端回答时返回 false，不重复投递；并发回答串行化。审批与 `/plan off`
   共用同一份状态文件 + `plan/mode` 事务，失败响亮且状态回滚。

## 4. 仍为 PARTIAL 的差异

1. **逐轮卡 —— 2026-11-18 已闭合**：座位现在一轮一次（`pluginapi.TurnProps` / `Env.Turn`，
   `internal/uikernel/turns.go` 的 `TurnTails` + `shell_skeleton.go` 的 `dispatchTurnTails`，
   判据与 `plugins/dsconversation/official_transcript_projection.go:805-837` 同一张分派表），
   `plugins/dsplan/cards.go` 只画 `env.Turn` 那一轮；`latestTurn` 降为「没有轮次基线」的回退。
   **残留**：骨架没有 `conversation.chat.node` 工厂，座位派发点仍在正文列尾部，不是嵌在每一轮
   尾节点里——内容逐轮正确，位置仍是列尾。
2. **navigation / 多实例**：右栏仍由插件保存单个 `openAddress`，不是官方每 occurrence 的
   navigation；缺 subagent plan 地址与 `sidebar.right.pane.tab.title` 派发。临时 review 地址本轮已补。
3. **异步资源态**：本仓同步读帧，没有官方异步 resource 的 loading/failed；临时 expired 已真实实现。
4. **相对链接**：只认明确 scheme（http/https/mailto），尚未把相对路径解析成工作区文件。
5. **视觉细节**：2026-11-17 已按 rc.2 CSS 复核卡片、chip、预览与审阅卡（§7）；
   busy 时 StateDot 已落官方 14px 尺寸，但仍是静态点，未复刻官方 1.5s spinner 动效。
6. **帧缺 turn 的取舍**：此时只留最后一份；宿主正常 `tool/call` 一直写 turn。
7. **切会话**：durable 地址属于旧会话时退到本会话最新计划；临时地址跨会话显示 expired。

## 5. 主要文件

| 文件 | 内容 |
|---|---|
| `HOST:modules/tool/plan/plan.go` | 提交仍 active、持久 plan-review view、官方选项与 callId |
| `HOST:cmd/dshd/serve_agent_api.go` | plan-review 回答校验、Approve/Keep/custom、去重与真实提交 |
| `HOST:cmd/dshd/serve_commands_api.go` | 真 `/plan off` 与 plan/mode 事务 |
| `HOST:internal/session/plan_projection.go` | exit 不再伪装退出；plan/mode 为退出真值 |
| `internal/apiclient/{projections,agent_actions}.go` | plan 投影、commands.execute、userQuestions.answer |
| `internal/planreview/review.go` | 官方 narrowing、pending 与 Markdown 摘要 |
| `plugins/dsquestions/plan_review.go` | 父审阅卡、批准、错误、要求修改 |
| `plugins/dsplan/{chip,review,preview}.go` | chip、自动/手动全文、durable/临时预览 |
| `pluginapi/turn_tail.go` + `internal/pluginapi/alias.go` | 按轮座位 owner props（`TurnProps` / `Env.Turn`）契约 |
| `internal/uikernel/{turns.go,shell_skeleton.go}` | journal 帧 → 轮次表（`TurnTails`）；骨架按轮 dispatch + 派发后恢复 |
| `internal/apiclient/action_cards.go` | `ProposalForTurn`（官方 `proposalOf`）；`LatestProposal` 降为回退 |
| `plugins/dsactioncards/cards.go` | 按轮取卡、dismiss 带自己 callId、交互 id 带轮次、brief 展开态按 callId |
| `cmd/app/{main,wave3_wiring}.go` | 快照、动作、子槽与投影缓存接线 |

## 6. 测试

- 宿主：提交保持 active；Approve 只提交一次并退出；Keep planning/custom 保持 active；
  畸形答案拒绝；持久失败回滚；`/plan off` 状态与投影、附件官方错误、on 不假 steer；
  projection 不再把 exit result 当退出。
- GUI：投影坏形状未知；chip 两态与 `/plan off`/失败；plan-review narrowing、官方
  `multiSelect` wire、回答后隐藏；父卡批准原文、失败保留、要求修改不代为回答；自动仅一次、
  手动重开、durable/临时地址及 expired；apiclient null/error/bool 与会话隔离。
- 逐轮卡（2026-11-18）：`internal/uikernel/turn_seat_test.go`（`TurnTails` 分派表 + 骨架按轮派发/恢复/
  无基线回退）、`plugins/dsplan/turn_cards_test.go`（两轮各一份计划 → 两帧各画自己那一轮；
  没有提交的轮一个像素都不画）、`internal/apiclient/action_cards_turn_test.go`（按轮取卡 + 同轮最小 callId +
  回退档不变）。变异：把 `turnPlans` 改回 `latestTurn` → `TestCardsRenderEachTurnSeparately` 红
  （第 1 轮那帧画出第 3 轮的卡）；把 `dispatchTurnTails` 改回每帧一次 →
  `TestFirstScreenDispatchesTurnTailPerTurn` 红（0 次按轮派发）。
- 最终门禁退出码见本轮 `docs/progress.md` 与交付报告；未 commit/push。

## 7. 视觉复刻（2026-11-17 · feat/vc-plan）

> 官方 CSS：`dsh-client-ui-plan/src/client/PlanPreview.module.css:1-43`、
> `PlanModeControl.module.css:3-82`；审阅父卡：
> `dsh-client-ui-user-questions/src/client/PlanReviewPanel.module.css:1-109`。
> 共享 Button 是默认 `size="md"`，不是旧实现自选的 32px 小按钮：
> `dsh-web-frontend/dist/assets/index-CSAdOBx6.css:1` 的
> `_button_1ulci_2/_md_1ulci_23/_icon_1ulci_78`。

| 部分 | 旧值 → 官方值 | 本仓 |
|---|---|---|
| 计划卡 | 60px、8/10 padding、10 gap、40px icon、13/20 标题、10/16 说明、28px 打开按钮原已一致；补 `CardFill/CardFillHover/CardTileFill` 与 120ms ease、2px focus ring + 3px offset | `plugins/dsplan/cards.go`、`plugins/dscore/dscore.go` |
| 计划 chip | 无 min-width → **34px**；无 glyph → **14px** plan/close；`BgLayer3` → **business-tertiary**；补 hover 6% 与 2px ring + 2px offset | `plugins/dsplan/chip.go`、`focus.go` |
| 全文预览 | 桌面仍 **20/24/40**；补整窗 ≤767px 的 **16/18/32** | `plugins/dsplan/preview.go` |
| 审阅按钮 | **32→36px** 高、**12→14px** 横内距、**13/20→14/22** 字/行、radius **SM→MD**；discuss **gap 6**、icon slot **16** / glyph **14**；disabled **0.6→0.4** | `plugins/dsquestions/plan_review.go` |
| 审阅卡 | 补 ≤720px footer **8/12/10 + flex-end**、描述 **2 行截断**、warn primary、StateDot **10/14**、overflow clip 与官方 elevation-panel | 同上 |
| 审阅链接 | 保留 gap 4 / 14/20，补 **14px** 官方 chevron、hover primary 与 2px ring + 3px offset | `plugins/dsplan/review.go` |

图标全部来自官方 sprite `dsh-web-frontend/dist/assets/index-ghOpC9uV.js:129`
（sha256 `061de8cefa15e1fbb87d301463ad4ebb79259b197ca08e83406f6997c1af8bb8`）：
`IconPlanOutlineRegular`、`IconCloseCircleFillRegular`、`IconChevronRightOutlineRegular`、
`IconEditOutlineRegular`；Markdown 卡仍复用既有 `FileTypeMarkdown` 三层原件。
close 外圈的 SVG `A` 由 `viva icons` 编译，未手画。几何与 sprite 结构反证测试在
`plugins/dsplan/geometry_test.go`、`plugins/dsquestions/plan_review_geometry_test.go`、
`plugins/dsassets/plan_test.go`。官方 rc.2 没有“计划项勾选框”DOM/CSS，本轮不新增该结构。

## 8. 逐轮座位改造（2026-11-18 · feat/pv-28-turn）

官方那个座位一轮一次（`turn-tail` 节点的 children，`TurnTailNodeView.tsx:43-49` 每节点
renderSlot 一次），owner props 带这一轮（`contract/slots.ts:60-64`）。改造分四层：

| 层 | 改动 | 锚点 |
|---|---|---|
| 契约 | `pluginapi.TurnProps{Turn,Seq}` + `Env.Turn *TurnProps`（只在按轮 dispatch 期间非 nil，写者恢复） | `pluginapi/turn_tail.go`、`pluginapi/plugin.go` 的 `Env` |
| 壳 | `TurnTails(frames)` 取轮次表（与 transcript 投影同一张 `turn/end` 分派表）；`dispatchTurnTails` 按轮各派一次 | `internal/uikernel/turns.go`、`shell_skeleton.go` |
| dsplan | `turnPlans` 只画 `env.Turn` 那一轮；`latestTurn` 降回退 | `plugins/dsplan/{cards,plan}.go` |
| dsactioncards | `ProposalForTurn`（= 官方 `proposalOf`），dismiss 带自己 callId、交互 id 带轮次、展开态按 callId | `internal/apiclient/action_cards.go`、`plugins/dsactioncards/cards.go` |

**残留**：座位派发点仍在正文列尾部（骨架没有 `conversation.chat.node` 工厂），不是嵌在
每一轮尾节点里；内容逐轮正确、位置仍是列尾，这一条如实登记在 `docs/official-parity.md` 的 #28 行。

## 9. 右栏 tab 的 navigation / 多实例 / 五段地址 + 芯片图标与资源标题（2026-11-18 · feat/pv-28-tabs）

这一轮闭合账本 #28 的第 ④⑤ 两条。官方两侧的契约原文先摆出来，再看本仓落点。

### 9.1 ④ 官方：身份是**地址**，不是类型

| 官方 | 原文 | 说明 |
|---|---|---|
| `OFF:dsh-client-ui-sidebar-right/src/client/contract/slots.ts:165-177` | `openResource(address, options?)` | 资源型 open：`params` 走 `SidebarRightResourceParams`，另有 `destination` |
| `OFF:dsh-client-ui-sidebar-right/src/client/tab-registry.ts:140-160` | `readonly contentId: string` +「Two opens of the same address are the same tab, which is what makes opening idempotent.」 | 地址就是 tab 身份、打开幂等 |
| `OFF:dsh-client-ui-sidebar-right/src/client/contract/slots.ts:110-122` | `SidebarRightTabNavigation{address, params, revision}` | 每次导航 `revision+1`，`params` 是这次 open 的实参 |
| `OFF:dsh-client-ui-plan/src/client/index.ts:65-69` | 类型注册 `{id, kind:'plan', patterns:['dsh-resource://plan/**','dsh-resource://plan-review/**'], priority:'builtin', canOpen, title}` | 一个 kind 认多个地址前缀 |
| `OFF:dsh-client-ui-plan/src/client/index.ts:70-76` | `ctx.sidebarRight.openResource(planAddress({session, callId}))` | 每点一张计划卡就按地址开一份 |
| `OFF:dsh-client-ui-plan/src/client/plan.ts:56-62` | `kind==='session' ? [sessionId, callId] : ['subagent', parentSessionId, childSessionId, mode, callId]` | **五段地址** |
| `OFF:dsh-client-ui-plan/src/client/plan.ts:69-87` | 段数只认 2 或 5；5 段时 `parts[0]==='subagent'` 且 `parts[3] ∈ {one-shot, continuable, unknown}` | 解析校验 |
| `OFF:dsh-client-ui-plan/src/client/index.ts:72-74` | `ctx.sessions.subagentAddress(sessionId)` | 五段的父/子/mode 从这里来 |

**subagent 五段地址具体是哪五段**：`subagent` · **父会话 id** · **子会话 id** ·
**mode**（`one-shot` / `continuable` / `unknown`）· **callId**。计划帧落在**子**会话
日志里，所以解析出的"内容所在会话"是 `parts[2]`（官方 `OFF:plan-resource.ts:45`
拿 `target.session` 去 `remote.events` 读 history）。

**params 能不能带 `planId` 之类**：官方 `openResource` 的第二实参只有
`SidebarRightResourceParams`（`contract/params.ts`），`ui-plan` 只用过一条 ——
临时审阅 `{'plan-review': {planReview: {markdown, title}}}`（`OFF:review-preview.ts:5-10`、
`OFF:index.ts:102-104`）；**durable 计划的身份在地址里，不在 params 里**。本仓同理：
`dsplan` 打开计划时不带 params（零值），只有临时审阅走内存文档。

### 9.2 ④ 本仓落点

| 层 | 改动 |
|---|---|
| 契约 | `pluginapi.Tab.Address`（= `contentId` / `navigation.address`）、`Env.RightTabAddress`、`Actions.OpenRightResource(kind, address, params)`、`RightTabAddressTitleProvider`（官方标题座位 owner props 带地址） |
| 壳 | `cmd/app/right_tab_occurrences.go`：`rightTabRecord.Address` 落盘；`openRightTabAt(kind,address,params)` 按 (kind,address) 幂等、不同地址各自一枚、非 multiple 类型认领空地址占位 tab；`projectRightTabs` 对带地址记录免于「同 kind 单枚」；`decodeRightTabSurface` 拒绝同一 (kind,address) 两枚；`rightTabAddressFor` → `Env.RightTabAddress` |
| dsplan | `cards.go` 的 `open` 改走 `OpenRightResource`；`preview.go` 的 `resolve` 先读 `env.RightTabAddress`；`plan.go` 的 `planAddressFor` / `parsePlanAddressFull` 实现 2 段 + 5 段全契约 |

**残留（宿主侧）**：五段可解析、可构造，但**当前不会生产**——宿主 `subagent` 投影只有
`{mode,label,seq}`，没有 `parentSessionId`（`internal/apiclient/projections.go` 的
`SubagentIdentity` 注释），而官方五段正是靠 `sessions.subagentAddress` 拿父会话 id。

### 9.3 ⑤ 官方：芯片 = `FileTypeIcon kind="other" 16` + 资源标题

`OFF:dsh-client-ui-plan/src/client/PlanPreview.tsx:47-53`（`PlanTitle`）：

```tsx
const tab = useTabInfo()
const resource = useResource<'plan'>(tab.tab.navigation.address)
return <><FileTypeIcon kind="other" size={16} className={css.titleIcon} />{plan?.title ?? tab.tab.title}</>
```

- **资源标题从哪来**：`plan.title` = 计划正文首个 `# 标题`（`OFF:plan.ts:45-48` 的
  `/^#\s+(\S[^\r\n]*)/`）；取不到退回 `tab.tab.title`，即类型注册的
  `title: () => t('preview.title')`（中文「计划」）。
- **什么时候变**：地址变了（换一份计划）或那一份内容变了；`ui-plan` 的计划正文是不可变
  的调用参数，所以同一个 callId 的标题不会变。
- **图标颜色**：`FileTypeIcon` 是 `clsx(css.icon, css[resolvedType], className)`
  （`OFF:ui-primitives/src/FileTypeIcon.tsx:26-33`），其中
  `._icon_1wejo_1{color:var(--dsh-file-type-icon-color,var(--dsh-file-type-default-color))}`、
  `._other_1wejo_32{--dsh-file-type-default-color: var(--dsw-static-neutral-bluish-300)}`；
  `other` 是 muted（`FileTypeIcon.tsx:233`），折角 `var(--dsw-static-neutral-400)`（`:162-163`）。
  **两层都是静态色板**，`.titleIcon` 编译产物只有 `{flex:none}`（`lLdGaq_titleIcon`），
  不覆盖任何颜色变量 → 预建一份就与官方同色，**不需要按帧重建**。

### 9.4 ⑤ 本仓落点

| 层 | 改动 |
|---|---|
| 素材 | `plugins/dsassets/files.go` 新增 `FileTypeOther()`（复用仓内已在的 other/muted 图层，底板 `0xcfd3d6` = `--dsw-static-neutral-bluish-300`、折角 `0xa2a4a6` = `--dsw-static-neutral-400`） |
| 注册 | `plugins/dsplan/plugin.go` 的 `plan-preview` 画带 `IconImage: planTabIcon()` + `IconSize: 16` + `IconMarginRight: 0` |
| 标题 | `plugins/dsplan/tab_chip.go` 的 `RightTabTitleFor(sessionID, tabID, address)`：计划地址 → 本帧缓存的计划标题；临时审阅地址 → 内存文档标题；跨会话 / 取不到 → 空串（壳保留类型标题） |

**残留（一帧延迟）**：壳在 `m.view.Draw` 之前投影名录（`env.RightTabs = m.rightTabsFor()`），
而计划标题要等本帧 `drawCards`/`drawPreview` 从帧里算出来，所以刚提交那一帧仍显示类型标题。

### 9.5 测试与变异

- 壳：`cmd/app/right_tab_resource_test.go` —— 同址幂等 / 异址并存（不靠 `Multiple`）/
  空地址退化 / 落盘还原 + 唯一性拒绝 / 资源标题覆盖类型标题且不串址。
- 插件：`plugins/dsplan/plan_address_subagent_test.go` —— 五段往返（含需转义字符）+
  官方校验逐条拒绝 + 五段地址真定位到**子**会话的帧；`plugins/dsplan/tab_chip_test.go` ——
  图标两层颜色/viewBox/无描边 + `IconSize==16` + `IconMarginRight==0` + 素材同源 +
  标题取值顺序（计划/审阅/失效/跨会话/非计划地址/空地址）。
- 变异（每条都先改坏、跑出红、再还原）：`IconSize` 16→14 红；
  `openRightResource` 去掉按地址查找 → 幂等红；`projectRightTabs` 回旧去重 → 并存红；
  `parsePlanAddressFull` 去掉 mode 闭集 → 校验红；回退成只认两段 → 五段红；
  `RightTabTitleFor` 恒空 → 标题红。
