# 多视图座位（会话 view / 右栏 tab）—— rc.2 机制与落点

> 2026-11-15。这一份是**机制**的取证与落点台账，不是某个部件的完成声明。
> 它回答的问题：「官方一个位置、多个视图轮流上」这件事，官方是什么形状，
> 我们建在哪，缺什么。
>
> 上一批（`tmp/GO8-RC2/_gp3/subagent-dossier.md:344`）查明 `#54` / `#59` / `#65`
> 三条 PARTIAL **共同卡在这两个机制上**：右栏是单槽单占用者（骨架
> `has→return`，没有 tab 体系），会话区没有 view-tab 机制。这一份记录机制落地后的状态。

## 1. 官方形状（逐条 file:line）

### 1.1 `conversation.view` —— 会话主区的视图

| 项 | 官方事实 | 锚点 |
|---|---|---|
| 槽声明 | `'conversation.view': { kind: 'list'; scope: 'session'; owner: ConvViewOwnerProps }` | `dsh-client-ui-conversation/src/client/contract/slots.ts:285` |
| 名录形状 | `interface ViewTab { id: string; label: string }`，由注册选项投影 | 同包 `contract/views.ts:8` |
| 一次只画一个 | `renderSlot('conversation.view', {...}, { only: viewId })` | 同包 `skeleton/DefaultConversationViews.tsx:36-41` |
| 激活判据 | `resolveActiveView(tabs, selectedId)`：偏好命中 → 用它；否则找 `chat`；都没有 → `undefined`（**不挑别的**） | 同包 `view-selection.ts:15-21`；`DEFAULT_VIEW_ID='chat'` 在 `:4` |
| 页签行何时出现 | `showTabs = !hideChrome && tabs.length > 1` | 同包 `skeleton/ConversationSession.tsx:119` |
| 页签行内容 | `visibleTabs.map(...)`，选中项 `css.tabActive`，点击 `selectView(id)` | 同包 `skeleton/ConversationSession.tsx:222-247` |
| `.tabs` / `.viewTabs` / `.tab` 几何 | `gap:16px 20px` / `gap:32px; min-height:25px` / `min-height:35px; margin:-10px -10px 0; padding:10px 10px 9px; font-size:13px; line-height:16px`；选中条 `2px` | 同包 `skeleton/ConversationRoot.module.css:225-314` |
| 注册例子（chat） | `id:'chat', order:0, label:()=>t('view.chat')` | `dsh-client-ui-chat/src/client/apply.ts:237-242`；`view.chat` 中文 `对话` 在 `src/client/locale.ts:74` |
| 注册例子（trajectory） | `id:'trajectory', order:10, label:()=>t('view.trajectory')` | `dsh-client-ui-trajectory/src/client/index.ts:80-86`；中文 `轨迹` 在 `src/client/locales.ts:8` |
| 注册例子（stats） | `id:'stats', order:40, label:()=>t('view.stats')` | `dsh-experimental-client-ui-stats/src/client/index.ts:75-81`；中文 `统计` 在 `src/client/locales.ts:8` |
| More 菜单 | `conversation.session.header.views.actions`（`kind:'list'`，有注册项才显示 More） | `dsh-client-ui-conversation/src/client/contract/slots.ts:283` |

### 1.2 `sidebar.right.pane.tab` —— 右栏面板的 tab

| 项 | 官方事实 | 锚点 |
|---|---|---|
| 座位链 | `rightbar`（root/single）← RightbarRoot；它声明 `rightbar.session`（session/single）← RightbarSeat；Seat 再声明两枚 keyed 座位 | `dsh-client-ui-sidebar-right/src/client/contract/slots.ts:43`、`:59`、`:73` |
| key 语义 | 「dispatched with the `id` of the type in force for `tab.kind`」——**key 是 tab 类型的 `id`，不是 kind** | 同文件 `:59-64` |
| 标题座位 | `sidebar.right.pane.tab.title`，同一个 key；没注册就退回注册时捕获的 title | 同文件 `:73-78` |
| 两阶段注册 | ① 类型进 `ctx.sidebarRightTabs`；② 体进 keyed 座位，同 kind | `dsh-client-ui-sidebar-right/src/client/index.ts:22-26`（注释）、`:290-308` |
| 类型定义 | `{ id, kind, title: () => string, priority?: 'extension'\|'builtin'\|'fallback', patterns?, canOpen?, guide? }` | 同包 `tab-registry.ts:106-…` |
| 默认页 | `defaultSeed(tabs)`：名录里**恰好一个** guide 条目就播种它，0 或 ≥2 个则上 guide | 同包 `contract/seed.ts:26-33` |
| 注册例子（terminal） | `{ id:'@deepseek-ai/dsh-client-ui-sidebar-terminal', kind:'terminal', priority:'builtin', title:()=>t('title') }`；体注册在 `sidebar.right.pane.tab` 的 key = 类型 `id` | `dsh-client-ui-sidebar-terminal/src/client/index.ts:72-75`、`:95-99`；中文 `标题=终端` 在 `src/client/locales.ts:15` |
| 芯片行几何 | `.tabStrip{display:flex; gap:4px; align-items:center; height:28px; padding:10px 6px 0 var(--dsh-dockkit-strip-inline-start,10px)}`（外高 38） | `dsh-web-frontend/dist/assets/index-CSAdOBx6.css` 的 `_123xe_` 模块（dockkit 编译产物；源码包不在随包 `src/**` 里） |
| 芯片几何 | `.tab{min-width:80px; max-width:170px; height:28px; padding:0 10px; font-size:13px; line-height:1; border-radius:radius-sm; color:label-secondary}`；选中 `.tabActive{color:label-primary; background:var(--dsw-alias-markdown-tag)}` | 同上 |
| 芯片分隔 | `.slot{width:10px; height:28px}` + `:before{width:.5px; height:14px; background:border-l4}`；紧邻选中芯片的那条**不画**（`.tabActive + .slot` 规则覆盖） | 同上 |
| 38px 的独立佐证 | 「the height of the Sidebar's tab strip and header row (ui-dockkit `.tabStrip` 38px + the pane header's 38px)」 | `dsh-client-ui-conversation/src/client/skeleton/ConversationRoot.module.css:17-23`（注释） |
| 选中底色的令牌取值 | `--dsw-alias-markdown-tag: var(--dsw-static-neutral-bluish-75)`（亮）/ `-850`（暗） | `dsh-client-ui-theme/src/styles/design-platform.css:272`、`:424` |

## 2. 本仓落点

### 2.1 契约层（`pluginapi/`）

| 形状 | 位置 |
|---|---|
| keyed 槽的 key / 选择器文案 / 字形 | `pluginapi/plugin.go:200-215`（`Painter.Key` / `Label` / `Icon`） |
| 选择器条目的公共形状 | `pluginapi/plugin.go:272-283`（`type Tab{Key,Label,Icon,Order}`） |
| 壳交给插件的**名录 + 激活项** | `pluginapi/plugin.go:318-330`（`Env.Views` / `ActiveView` / `RightTabs` / `ActiveRightTab`） |
| keyed 子槽派发 | `pluginapi/plugin.go:332-345`（`Env.DrawKeyed func(f ui.C, slot, key)`；**必须带 f**，见那里的注释） |
| 选择动作 | `pluginapi/plugin.go:821-831`（`Actions.SelectView` / `SelectRightTab`） |
| 官方四个选择器座位常量 | `pluginapi/plugin.go:136-166`（`SlotRightbarSession` / `SlotRightPaneTab` / `SlotRightPaneTabTitle` / `SlotConvView`） |
| 官方三个 view id | `pluginapi/plugin.go:168-179`（`ViewChat` / `ViewTrajectory` / `ViewStats`） |

### 2.2 机制层（`internal/uikernel/`）

| 机制 | 位置 |
|---|---|
| 名录投影（同 key 合并 / 按 (Order, 装配顺序) 排 / 空 key 不进名录） | `internal/uikernel/view_tabs.go:92-131` |
| 激活判据（官方 `resolveActiveView` 逐字移植） | `internal/uikernel/view_tabs.go:137-158` |
| 右栏激活判据（官方 `defaultSeed` 移植，**差异见下**） | `internal/uikernel/view_tabs.go:162-176` |
| 会话页签行绘制 | `internal/uikernel/view_tabs.go:183-231` |
| 右栏芯片行绘制（dockkit `.tabStrip` + `.tab` + `.slot`） | `internal/uikernel/view_tabs.go:237-280` |
| keyed 派发（只画命中的那一份；key 空 = 不画，**不回退成全画**） | `internal/uikernel/kernel.go:248-296` |
| 官方骨架：页签行 + 按 ActiveView 派发 | `internal/uikernel/shell_skeleton.go:848-858`、`:876-889` |
| 官方骨架：右栏按名录派发 | `internal/uikernel/shell_skeleton.go:1063-1088` |
| `ShellOptions` 的两个名录与激活项 | `internal/uikernel/shell_skeleton.go:146-154` |

### 2.3 壳层（`cmd/app/`）

| 机制 | 位置 |
|---|---|
| 名录投影 + 激活项填进 `Env` | `cmd/app/main.go`（`env.Has = …` 之后的六行） |
| keyed 派发接到内核 | `cmd/app/main.go`（`m.view.Draw` 回调开头） |
| `Env` 名录进 `ShellOptions` | `cmd/app/shell.go`（`shellOptionsFor`） |
| 选中项的**唯一出处**与闸门 | `cmd/app/view_tabs_wiring.go:45-135` |

### 2.4 占用方

| 谁 | 注册了什么 | 位置 |
|---|---|---|
| `ds-topbar` | 页签行**按名录画**（此前写死 chat/trajectory 两枚） | `plugins/dstopbar/topbar.go:236-280` |
| `ds-rightbar` | 右栏 tab 类型 `timeline` 的体 + 右栏面板（画芯片行 + 派发选中项）+ `conversation.view` 的 `trajectory` 条目 | `plugins/dsrightbar/rightbar.go:89-160` |
| `ds-conversation` | `conversation.view` 的 `chat` 条目（`order:0`，官方 ui-chat 的注册顺序） | `plugins/dsconversation/conversation.go:258-268` |
| `ds-agent-team` | chat 视图的**第二块画**（同 key，label 留空） | `plugins/dsagentteam/parent_containers.go:120-121` |
| `ds-terminal` | 右栏 tab 类型 `terminal`（#54） | `plugins/dsterminal/terminal.go:58-90` |

### 2.5 令牌

`Theme.MarkdownTag ← --dsw-alias-markdown-tag`（`pluginapi/theme.go`；取值 `plugins/dscore/dscore.go:152`（暗 `neutral-bluish-850`）、`:221`（亮 `neutral-bluish-75`））。
官方那边它是「Markdown 行内标记底色」，但 dockkit 的**选中芯片底**用的就是它 —— 名字与用途不一致是官方的事实，不是我们的选择。

## 3. 与官方的差异 / 缺口（**未做，如实登记**）

| # | 缺口 | 为什么现在不做 | 后果 |
|---|---|---|---|
| G1 | **guide tab**（`sidebar.right.tab.guide` / `.guide.entry` / `.guide.actions`）没有占用方 | 官方 `defaultSeed` 在「0 或 ≥2 个条目」时上 guide；我们没有 guide 页 | 右栏多条目时退化成「按名录第一条」（`ResolveActiveRightTab` 注释里写明），与官方 `defaultSeed` 不一致 |
| G2 | `priority` 覆盖机制（`extension` / `builtin` / `fallback`）未建 | 本仓没有第三方 tab 类型来抢同一个 kind | 名录 key 直接用官方 `kind`（不用包名 `id`）；出现第二种实现时必须先补这一层 |
| G3 | 关闭处理器 / 多实例（`multiple` / `keepMounted` / `registerCloseHandler`） | 与 G2 同源：没有第二个 tab 类型，也没有页签关闭动作 | 芯片上不画关闭按钮（官方 `.tabClose` 在 hover/选中时出现） |
| G4 | `conversation.session.header.views.actions`（More 菜单）只有骨架座位，没有占用方 | 页签行放得下时官方也不画 More | 页签多到放不下时会被裁切，而不是折进 More |
| G5 | 拖 dock / 浮动面板 / 分屏（dockkit 的 pane 体系） | 与右栏 tab 机制相邻但是另一件事 | 右栏仍是一列，不能拆 |
| G6 | `conversation.view` 在**旧三栏档**只提供名录，不派发 | 旧档主区正文归 `dsconversation` 的 `main.body`（`BridgeLegacy`），两条路互斥 | 旧档下 `trajectory` 靠 `beginTrajectoryView` 的视口偏移让位（既有实现），不是 keyed 派发 |

## 4. 机器断言

| 测试 | 守什么 |
|---|---|
| `internal/uikernel/view_tabs_test.go:TestViewTabsMergesSameKeyAndSortsByOrder` | 同 key 合并成一条、按 Order 排 |
| `…:TestRightTabsOrderIsRegistrationOrderAtSameOrder` | 同 Order 按装配顺序（稳定，不吃 map 随机序） |
| `…:TestTabRosterSkipsEmptyKey` | 空 key 不进名录 |
| `…:TestResolveActiveViewFollowsOfficialRule` | 官方三条分支；**没有 chat 时不挑别的** |
| `…:TestResolveActiveRightTabFollowsDefaultSeed` | 单条目直接上、偏好优先、失效落第一条、空名录报"没有" |
| `…:TestDrawSlotKeyedOnlyDrawsMatchingKey` | keyed 只画命中；**空 key 不画**（不回退成全画） |
| `…:TestKeyedOrderIsPaintOrder` | 同 key 多块画按 Order 升序 |
| `…:TestRightTabStripGeometryMatchesOfficialDockkitCSS` | 芯片数值逐条 = 官方 CSS（含 28+10=38） |
| `…:TestSkeletonDispatchesOnlyActiveView` / `…RightbarDispatchesOnlyActiveTab` | 骨架按名录派发，不再"全画一遍" |
| `…:TestDrawRightTabsDrawsStripAndSelects` / `…DrawViewTabsDrawsChipsAndSelects` | 芯片真的画出来、真的点得动 |
| `cmd/app/view_tabs_wiring_test.go` | 闸门：名录外的 key 不写；失效偏好解析回名录内 |
| `plugins/dsrightbar/rightbar_tabs_test.go` | **产品路径**：两枚芯片、只派发选中项、点芯片走 `Actions.SelectRightTab` |

## 5. 三条视图的现状（#65 已落地，2026-11-15）

| 条 | 现在的状态 | 还缺什么 |
|---|---|---|
| `#54 dsh-client-ui-sidebar-terminal` | 座位已接：终端是右栏的 `terminal` tab 类型（官方形状）；右栏展开时用它，收起时退回本包已有的 F5 浮层 | **终端本体**：官方是 VT 交互终端（每会话多终端、agent 打开的终端、沙箱徽标、重连、控制权移交）。本仓 `internal/ansi` 只剥转义序列、按纯文本画最后 N 行，也**没有** `webTerminals` 那一层 |
| `#59 dsh-client-ui-trajectory` | 已注册为 `conversation.view` 的 `trajectory` 条目（`order:10`，与官方一致），名录/页签由注册表驱动 | 官方那种分组/折叠层级；旧三栏档仍靠视口偏移让位（G6） |
| `#65 dsh-experimental-client-ui-stats` | **✅ 本轮已落地**：`plugins/dsstatusbar/stats_view.go` 注册 `{stats, order:40}` 并画出逐 turn/step 的 11 列表格 + 总用时条 + 全量/窗口合计（证据 [official-attachment-stats-evidence-rc2.md](official-attachment-stats-evidence-rc2.md)） | 剩余差异：compaction/非 step 事件行、三个工具栏开关、加载更早分页（同证据 §5） |
