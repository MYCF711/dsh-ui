# `dsh-client-ui-plan`（#28）落地证据 · rc.2

> 2026-11-17 更新；配合 [official-parity.md](official-parity.md) 阅读。
>
> `OFF` = `DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`；`HOST` = DSH-GO。
>
> **结论：🟡 PARTIAL。四个可见座位均有真实数据与动作，plan-review 与 `/plan off` 已闭环；剩余差异在 §4，不把这两项继续记成 HIDE。**

## 1. 官方座位与本仓落点

| 官方座位 | 官方依据 | 本仓 |
|---|---|---|
| `conversation.chat.turnTail` | 每轮尾计划卡，`OFF:dsh-client-ui-plan/src/client/PlanCard.tsx:43-60` | `plugins/dsplan/cards.go`（既有） |
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

1. **逐轮卡**：本仓 `latestTurn` 只画最新一轮，官方按每个 Turn 的 `turnDataSource` 逐轮派发。
2. **navigation / 多实例**：右栏仍由插件保存单个 `openAddress`，不是官方每 occurrence 的
   navigation；缺 subagent plan 地址与 `sidebar.right.pane.tab.title` 派发。临时 review 地址本轮已补。
3. **异步资源态**：本仓同步读帧，没有官方异步 resource 的 loading/failed；临时 expired 已真实实现。
4. **相对链接**：只认明确 scheme（http/https/mailto），尚未把相对路径解析成工作区文件。
5. **视觉细节**：Markdown 文件图标、plan/close glyph、部分几何与官方 sprite 仍有差异。
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
| `cmd/app/{main,wave3_wiring}.go` | 快照、动作、子槽与投影缓存接线 |

## 6. 测试

- 宿主：提交保持 active；Approve 只提交一次并退出；Keep planning/custom 保持 active；
  畸形答案拒绝；持久失败回滚；`/plan off` 状态与投影、附件官方错误、on 不假 steer；
  projection 不再把 exit result 当退出。
- GUI：投影坏形状未知；chip 两态与 `/plan off`/失败；plan-review narrowing、官方
  `multiSelect` wire、回答后隐藏；父卡批准原文、失败保留、要求修改不代为回答；自动仅一次、
  手动重开、durable/临时地址及 expired；apiclient null/error/bool 与会话隔离。
- 最终门禁退出码见本轮 `docs/progress.md` 与交付报告；未 commit/push。
