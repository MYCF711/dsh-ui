# 自动化任务（#34 `dsh-client-ui-schedule`）的结构证据与实现台账

> 建立时间：2026-11-14 · 对应 `docs/official-parity.md` 第 34 行。
> 本轮把「自动化任务」从占位面板补成**接真数据的官方任务管理页**（列表/筛选/搜索/
> 详情规则/投递记录/三条宿主动作）。未做面在 §4 逐条登记，不藏。

**证据口径**：rc.2 官方随包发 `src/**` 原件（TS/TSX + CSS Module + 8 语言 locale）。
锚点记法 `OFF:ui-schedule/<src 内相对路径>:<行号>`，`OFF` 根 =
`tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/dsh-client-ui-schedule/`
（gitignored 的本机解包树）。宿主锚点记 `HOST:`，根是宿主仓 `origin/main`。

---

## 0. 官方包是什么（35 个 src 文件，一个主页面 + 四个投影面）

| 表面 | 官方源 | 说明 |
|---|---|---|
| 任务管理页（main 面板 + `sidebar.panellist`） | `OFF:src/client/TaskManagerPage.tsx:1-374` | 官方描述 "Host task management page"：标题/新建、状态筛选、搜索、任务行、详情（规则/记录） |
| 详情 | `OFF:src/client/TaskDetail.tsx:340-470` | 顶条 tab（规则/记录）+ 规则字段 + 记录历史 + 删除确认 Modal |
| 会话头提醒目录 | `OFF:src/client/ScheduleCatalogAction.tsx:1-205` | `conversation.session.header.utilities` 的 icon-only 触发器 + 弹层 |
| `schedule_create` 转录卡 | `OFF:src/client/ScheduleCreateCard.tsx:1-86` | `conversation.chat.turnTail` 的 Turn 级卡片 |
| 会话行时钟标记 / hover 卡 | `OFF:src/client/SessionScheduleMark.tsx:1-59`、`SessionScheduleHover.tsx:1-79` | `sidebar.session.row.leading` / `.hover` 座位 |
| 右侧栏任务 tab | `OFF:src/client/ScheduleTaskTab.tsx`、`definition.ts`、`task-tab-bindings.ts` | `sidebarRightTabs` + `sidebar.right.pane.tab` 座位 |

- 文案唯一来源：`OFF:src/client/task-manager-locales.ts:171-330`（zh）与
  `OFF:src/client/frequency-locales.ts:14-76`（频率/星期/单位/相对时间）。
  **本实现里出现的每一句中文都能在这两份表里逐字找到。**
- 数据面：`OFF:src/client/index.ts:130-160` 的 `remote.schedule.catalog / edit / run /
  history / deliveryLimits`；本仓等价物是 `pluginapi.Actions.ScheduleCatalog /
  ScheduleHistory / ScheduleDelete / ScheduleSetActive / ScheduleRun`。
- 页面几何全部出自 `OFF:src/client/TaskManagerPage.module.css`（960px 内容列、
  28px 标题上边距、24px 下边距、28px 筛选 chip / 14px 圆角、36px 搜索框 / 12px
  圆角、详情 47% 宽 + 左侧 0.5px l4 线、44px 详情顶条、24px gutter）。
  本仓常量在 `plugins/dssidebar/schedule_page.go:99-127`，每个值注释了出处。

## 1. 数据形状：官方 catalog 条目 → 宿主 → 本仓客户端

官方 `ScheduleCatalogEntry` 是 7 个 kind 的 union（after / at / every / daily /
weekly / cron / rrule）。宿主 `HOST:internal/transport/handlers_schedule_official.go:220-270`
用「一个结构 + omitempty」表达它，`normalizeScheduleEntry`（`:520-560`）按 kind
清掉别的分支字段。本仓客户端 `internal/apiclient/schedule.go:60-95` 的
`ScheduleEntry` **逐字对齐宿主 tag**；union 判别不另造类型，`decodeScheduleOutcome`
（`:247-270`）按 `code` / `deleted` / `kind` 三键分流。

三件不能吞的事（`internal/apiclient/schedule.go` 顶部注释 → 测试
`internal/apiclient/schedule_test.go`）：

1. `schedule.catalog` 未接线是 **501**，不是空数组（"查不了"≠"没有任务"）；
2. `schedule_not_found` / `schedule_conflict` / `schedule_execution_pending` /
   `delivery_cursor_not_found` 是 **HTTP 200 的 union miss**，不是传输错误；
3. 只有 `{id,deleted:true}` 代表真删掉。

## 2. 本仓实现落点

| 层 | 文件 | 作用 |
|---|---|---|
| 契约客户端 | `internal/apiclient/schedule.go:148-234` | `ScheduleCatalog` / `ScheduleEdit`(delete, patch.active) / `ScheduleRun` / `ScheduleHistory`；union 解码 |
| 契约快照 | `testdata/api.snapshot.json`（batch11：4 个方法，total 57→61） | 参数/结果逐字段，带宿主 `file:line` |
| 壳数据面 | `cmd/app/schedule_catalog.go:50-278` | catalog/history 缓存、首次问触发拉取、动作后重拉、动作失败交插件画官方文案 |
| 壳动作 | `cmd/app/main.go:2490-2498` | 8 条 `Actions` 注入 |
| 页面 | `plugins/dssidebar/schedule_page.go:135-560` | 官方列表/筛选/搜索/详情（规则+记录）/三条动作 |
| 侧栏入口 | `plugins/dssidebar/frame.go:74`（路由已在册） | 点「自动化任务」打开并**主动拉一次目录**（Transport 无 `schedule/changed` 推送） |
| 侧栏摘要（旧） | `plugins/dsschedule/schedule.go` | 仍走宿主扩展面 `schedule.list`，不动 |

**数据链**：`schedule.catalog`（壳缓存）→ `filteredScheduleRows`（状态/搜索三字段/
下次运行升序）→ 行（标题 + 状态·频率·下次运行）→ 详情（规则读只读字段；记录走
`schedule.history` 分页 + `nextBefore` 续读）。

**动作链（全部不乐观更新）**：

- 暂停/恢复 → `schedule.edit {id, action:update, patch:{active}, expectedUpdatedAt}`
- 立即执行 → `schedule.run {id}`
- 删除 → `schedule.edit {id, action:delete}`（界面两次点击确认）
- 任何成功/miss 都重拉 catalog（miss = 别处改过，收敛而不是报错）；
  失败时壳记动作 id，插件画官方那一句（`无法删除任务` / `无法保存，已保留修改，请重试`
  / `无法将任务加入队列，请重试`），宿主原文进状态栏。

## 3. 验收

- `internal/apiclient/schedule_test.go`：裸数组无参数、union 三态、patch/乐观锁、
  分页 `before`/`nextBefore`、limit 越界本地拦截。
- `plugins/dssidebar/schedule_page_test.go`：状态筛选、搜索三字段（标题/内容/会话 id）、
  下次运行取 `deliveryFloorEndsAt` 与 `scheduledAt` 更晚者、频率文案 8 种、
  空态文案按筛选/搜索、绝对/相对时间形状、**无头渲染**（画标题/筛选/行；
  点行出详情规则；点「任务运行记录」出记录）。
- 门禁：build / vet / gofmt / `go test ./...` / `parity-count --check` 全过（见交付报告）。

## 4. 与官方不同 / 没做（逐条，不藏）

1. **手动创建表单没做**。官方 `TaskManagerPage.tsx` 的「新建」主入口是"新建会话"
   （本仓已接 `Actions.NewSession`）；另一条「手动创建」要日期/时钟/时区整套选择器
   （`DatePicker/ClockPicker/PickerPopover` 三个组件 + 一整套规则校验文案），
   本仓没有这些组件，硬画日期输入就是自造交互。**缺的是组件，不是数据**：
   `schedule.create` 宿主已真实现。
2. **规则只读**。官方对 active 任务允许改名称/内容/时间（`TaskEditorFields`）。
   本仓只读展示（名称/内容/运行时间/关联会话）。原因同上：编辑时间需要
   `DatePicker/ClockPicker`；编辑名称/内容需要表单提交 + `expectedUpdatedAt` 冲突
   处置（`host.conflict` / `host.reapply` 两段 UI 也没做）。
3. **⋯ 菜单没做**。官方把「立即执行 / 删除」收进 `detail.more` 菜单；本仓没有
   通用 Menu 组件（`pluginapi.Primitives` 只有 ListRow/Badge/Dot/Divider/…）。
   三个动作以官方同名文字按钮直排在详情顶条（位置差异，文案无差异）。
4. **删除确认用两次点击**，不是官方 Modal（`TaskDetail.tsx:462-476`）。
   与侧栏删除会话同一条先例：本仓没有可复用的对话框组件不动手造；
   官方确认文案（标题/描述/确认/取消）在详情里原样摊开。
5. **记录时间按记录自带偏移渲染**（官方按浏览器时区）。本仓是 Go 原生界面，
   没有浏览器 `Intl`；按存储偏移渲染比按进程时区更接近"规则本来表达的时刻"。
6. **相对时间文案是手写格式**（`(2小时后)` / `(已逾期 3分钟)`），形状照
   `formatScheduleRelative`（`OFF:schedule-format.ts:481-496`），但没有 ICU 的
   语言复数规则——中文两种形态同字，本仓只做 zh。
7. **未做的四个投影面**（下一批）：会话头提醒目录（`ScheduleCatalogAction`）、
   `schedule_create` 转录卡（`ScheduleCreateCard`，需要 `conversation.chat.turnTail`
   Turn 级座位）、会话行时钟标记（`sidebar.session.row.leading/hover`）、
   右侧栏任务 tab（我们的右栏没有 tab 系统，见 #34 缺口）。
8. **没有 `schedule/changed` 推送**：Transport 没有这个事件，所以目录只在
   打开面板 / 动作后 / 手动重试时刷新（官方由 Host 推送）。极少数情况下列表会
   比宿主晚一拍——这是传输层缺口，界面已用"动作后必重拉"兜住自己发起的那部分。
