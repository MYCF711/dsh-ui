# 官方部件清单 vs 我们的

> 来源：官方 asar 提取的 `@deepseek-ai/dsh-client-ui-*` 包（**62 个**）+ `dsh-experimental-client-ui-*`（**4 个**）= **66 个**（rc.2 口径，见 §权威清单）。
> 我们有 **36 个插件目录**（判据：`plugins/*/register.go`，由 `cmd/app/plugins_assembled_test.go` 的 `TestEveryPluginDirIsImported` 双向守着）；账本对官方 **66 个部件**逐 id 判状态（分母由 `scripts/parity-count.py --check` 动态读官方清单并断言零缺零多）。
> **"几何对齐 ≠ 像官方"**，差距在功能面。

## 官方 66 个部件（按用途分组）

### 设置类（11）

- 🟡 `settings`（`dssettings` 浮层：F2 命令面板末行「设置」/ Shift+F2 直达，Esc 关闭。**不是官方的 11 页树**——它只把壳真接上了的动作摊成四块：外观 / 模型 / 权限 / 会话。缺一个 Action 就整块不画，当前值不知道就一行都不标选中）
- ❌ `settings-account`
- ❌ `settings-agent-loop`
- 🟡 `settings-general`（只有**主题**：深 / 浅 / 跟随系统，走 `Actions.SetTheme`，选完当场生效。语言、启动行为、快捷键这些通用项没有入口）
- 🟡 `settings-models`（列出**真实**网关模型（`models.list`，带 owned_by）+ 「恢复默认」；没有 provider/密钥/默认模型这些配置项，也不能在这里看用量）
- ❌ `settings-plugin-inventory`
- ❌ `settings-plugins`
- ❌ `settings-session-log`
- ❌ `settings-shell`
- ❌ `settings-subagent`
- ❌ `settings-web-search`

### 侧栏类（6）

- ✅ `sidebar`（`dssidebar`：工作区分组 + 搜索 + 状态点 + **两次点击删除会话**——宿主早就有 `session.delete`，界面一直没接；现在三条都接了：`session.delete`（侧栏两次点击）/ `session.compact`（F2「压缩上下文」）/ `session.fork`（F2「分支这个会话」，**整份复制**））
- 🟡 `sidebar-browser`（`plugins/dsbrowser`：右栏 `browser` tab = 官方 HTTP(S) 沙箱浏览器。地址白名单/历史状态机/工具条逐条搬官方 `src/client/**`，网页本体走 Viva 的 webview 宿主（`app.WebHost` + `canvas.NativeView`）。缺多实例 tab 与"上次打开"恢复卡，见 #50 行）
- ✅ `sidebar-documentpreview`（`dsfiles` 的预览浮层；16KB 上限、20 行截断）
- ✅ `sidebar-files`（`dsfiles`；`workspace.list(path)` + `fs.read`，可进子目录）
- ✅ `sidebar-right`（`dsshell` 的 `SlotRightbar` + `ToggleRightbar` + `ui.rightbarWidth`；收起后 corner 可重开）
- ✅ `sidebar-terminal`（`dsterminal`：F5 面板 + F6 发命令，宿主 `terminal.*` + `internal/ansi`）

### composer 控件（7）

- ✅ `model-selection`（F2 命令面板；数据来自宿主 `models.list` → 网关 `/v1/models`）
- ✅ `permission-presets`（F2 命令面板 5 条；宿主 `session.setPermission` + 白名单）
- ❌ `attachment`
- ✅ `commands`（F2 命令面板；A 类时做的）
- ✅ `input-trigger`（`plugins/dscomposer/trigger.go`：`@` 文件/会话 + `/` 技能三源注册表、官方 detect（引号路径 + URL 斜杠 carve-out）、按 source 分组菜单、Enter/Tab/↑↓/Esc；数据面见 `internal/apiclient/references.go`）
- 🟡 `entry-customization`（`plugins/dsentrycustom`：左栏 More 的未固定条目行（行首面板图标 + 行尾「固定」按钮）+ 会话头 More 的折起 view 行（active 带对勾）+ 「浏览更多插件」（`plugins/dssidebar/panelapi` = 官方 `pluginNavigation` 等价物，服务在册才画）。「创建你的插件」等宿主 `session.create` 收 `agentPreset` 才补；右栏 guide 无宿主，按 HIDE 登记）
- ⬜ `deepseek-model-label-1017`（**N/A**：官方是 1017 那次发布的临时角标，不是产品能力）

### 插件体系（3）

- ✅ `plugin-manager`
- ✅ `plugin-market`（`plugins/dsmarket`：官方 `plugins.tab` 的 **discover tab 已注册**——索引源列表/增删（`pluginMarket.listIndexes`/`setIndexes`，真实现）+ **搜索框与分类/标签/排序筛选工具栏**（索引声明的 `metadata.categories/labels/sortKeys` 全用上）+ `entries[]` 官方全字段结果卡与「显示更多」+ 条目**详情页**（详情 markdown + 官方版本列表 + 来源 facts）。数据面 `internal/apiclient/plugin_market*.go` → 壳状态机 `cmd/app/market_discover.go`（照官方 `market-store.ts` 移植：分页/`fetchedAt` 钉住/声明表裁剪/写回失败档）→ `plugins/dsmarket/discover.go`。剩余差异 11 条（安装/更新 501、`detail-file` 路由缺、图标图与四枚 sprite 无导出、`plugin-market/changed` 事件缺、`listIndexes` 空配置回 503 等）逐条登记在 [official-plugin-market-discover-evidence-rc2.md](official-plugin-market-discover-evidence-rc2.md)）
- ❌ `settings-plugin-inventory`

### 会话能力（13）

- ✅ `agent-preset`（`dssettings` 的 `agent-presets` 页：真名册卡片 + 「设为新任务默认」写 `agent-preset-registry.selectedDefault`；证据见 [official-agent-preset-evidence-rc2.md](official-agent-preset-evidence-rc2.md)）
- ✅ `approval`（审批条 + 三按钮；允许/拒绝走真实 `session.approve`）
- ✅ `chat`（`dsconversation` 的消息区；不等于官方的细分能力）
- ✅ `conversation`
- ✅ `goal`（`dsgoal`：GoalBar 停靠 composer 上方，数据来自宿主 `session.setGoal`）
- 🟡 `plan`（`plugins/dsplan`：**Turn 尾计划卡**（点开在右栏读全文）+ **右栏计划全文**（完整 Markdown、滚动裁剪、链接走壳打开）；数据直接从 journal 帧里的 `exit_plan_mode` 调用参数读，零新宿主方法。缺两座：计划模式 chip 要宿主 `plan` 投影（本仓没有）、待批计划的「查看全文」要 plan-review 待答问题（本仓 `exit_plan_mode` 不产生问题）——两者几何/文案证据齐但数据缺，按 HIDE 登记；证据见 [official-plan-evidence-rc2.md](official-plan-evidence-rc2.md)）
- ✅ `schedule`（`dsschedule`：侧栏「定时任务」，宿主 `schedule.list`）
- ✅ `jobs`（`dsjobs`：官方档 header 触发器 + 500px 弹层 / 旧三栏档侧栏清单；`jobs.list` 5 秒轮询 + `jobs.output` rune 游标分页 + `jobs.stop` 两段式；证据见 [official-jobs-evidence-rc2.md](official-jobs-evidence-rc2.md)）
- ✅ `subagent`
- ✅ `skill`（`dsskill`：**专用工具行** `SkillRow` 四态 + 两棵树 + 展开说明卡；**`/name` 用户文本规则**逐条实现；**胶囊** `SkillReference`。工具行**零宿主依赖**——官方 `skillRowModel` 只从 durable 的 call/result slice 推。**仍缺**：斜杠候选源（`inputTriggers` 注册表）与 `openSkill`（`skills/list`），见 B 类）
- ✅ `user-questions`（`dsquestions`：F7/F8 或点击选选项）
- ❌ `workflow-run`
- ✅ `message-feedback`（`plugins/dsconversation/feedback.go` + `internal/apiclient/message_feedback.go` + `cmd/app/reference_wiring.go`：官方七分类弹窗、`messageFeedback.list/put/delete` durable 写入、`ifVersion` 乐观并发 + `version-conflict` 用 `current` 就地收敛、成功/失败 toast。**本仓适配**：助手消息无宿主 messageId → 用持久 v4 seq 作 `v4:<seq>` 身份，见报告差异 1）

### 内容呈现（17）

- ✅ `reference`（`plugins/dsconversation/usertext.go`：`@path` / `@[label]` 加 `dsh-session:` 的 wire 引用 在用户消息里画成 file/folder/session chip，file chip 点击走 `Actions.PreviewFile`；`plugins/dscomposer/trigger.go` 的候选原样插入官方 wire mention；图标转写自 rc.2 dist，见 `docs/official-icon-sprite-register.md`）
- ⬜ `renderer`（**N/A**：官方的 *"Browser UI renderer: React slot bindings"* 是 React 专用；我们是 Go+Viva，槽位绑定在 `internal/uikernel`）
- ✅ `deliverables`（`dsdeliverables` + `dsconversation`：**3/4**——per-file diff 已接 `/api/changes.summary|diff|open`；证据见 [official-deliverables-changes-evidence-rc2.md](official-deliverables-changes-evidence-rc2.md)）
- ✅ `trajectory`（`dsrightbar`：按轮分组的轨迹栏）
- ✅ `tool`（`dstool`：工具卡片 + 展开参数/结果）
- ❌ `directory-picker-browse`
- ❌ `directory-picker-native`
- ✅ `open-in-app`（两半都有：`dsopen` 会话头部「打开」→ `OpenExternal(CWD)`；`dsfiles` 预览卡片「用默认程序打开」→ `OpenFile(file:…)`；**目录走平台文件管理器**——`app.OpenPath` 明确拒绝目录（`is not a regular file`），而「打开工作目录」正是目录，所以两条路必须分开走）。官方的"split button（列出已装应用）"没有，我们是单个按钮）
- ✅ `workspace`（`dssidebar`：分组按 `registry.SessionIDs` 归属与手动序 + 拖拽写 `workspace.insertSessionBefore`；证据见 [official-workspace-order-evidence-rc2.md](official-workspace-order-evidence-rc2.md)）
- ✅ `theme`（`dscore` 的 ds-dark / ds-light 两套 + **运行时切换**：F2 命令面板三条 深色/浅色/跟随系统，`-theme` 也收 `system`；官方的 "Appearance 设置行" 我们没有，入口在命令面板）
- ✅ `shortcuts`（`dsshortcuts`：F1 键位表，**带守卫**——从 `plugins/` 的 AST 里扫出所有 `ui.KeyXxx`，表漏记一个就红；官方的"录制"和"本地改键"没做）
- ✅ `secret-mask`（`internal/secretmask`：官方 `credential-formats.ts` 的私钥块/JWT/URL 口令/provider 前缀表 + 本仓高熵兜底；`plugins/dsconversation/usertext.go`：逐值遮罩 + 眼睛按钮原位显示/隐藏，长度不变（复制仍是原文））
- ⬜ `session`（**N/A**：官方的 *"Session Controller adapter for React"* 是 React 适配层；我们的会话状态在壳里）
- ❌ `cordis`
- 🟡 `brand-official`（`dsbrand`：官方鲸鱼 logo 矢量 + `ui.brand` 可整体替换；**"sidebar slots occupants"那层没有**）
- ✅ `layout`（`dsshell`：侧栏/中栏/右栏/底栏，可直接拖分隔线）
- ✅ `sidebar-right`（`dsshell` 的 `SlotRightbar` + `ToggleRightbar` + `ui.rightbarWidth`；收起后 corner 可重开）

## 官方包逐条对账（**机器校验**，`scripts/parity-count.py --check` 守）

本节是**权威清单**：官方 rc.2 的 66 个 `dsh-client-ui-*` 包逐条给状态与落点。
来源 `docs/evidence/official-ui-plugins-0.2.2-rc.2.txt`（66 行，sha256 `9e910e4e97ef8f18`）。

> 2026-11-14 之前，本账本记的是 rc.1 时代的 **55** 个部件，而门禁只校验
> 「合计行 == 本文条目数」——**陈旧分母照样拿到绿**。现在分母**动态读官方清单**，
> 且逐 id 断言集合相等（漏一个、多一个都红）。

| # | 官方包 | 状态 | 本仓落点 |
|---|---|---|---|
| 1 | `dsh-client-ui-account-notifications` | ✅ IMPLEMENTED | `plugins/dsnotifications` |
| 2 | `dsh-client-ui-action-cards` | ✅ IMPLEMENTED | `plugins/dsactioncards` + `internal/apiclient/action_cards.go` |
| 3 | `dsh-client-ui-agent-preset` | ✅ IMPLEMENTED | `plugins/dssettings` 的 `agent-presets` 页（真名册卡片网格 + 「设为新任务默认」写 `agent-preset-registry.selectedDefault`）+ `internal/apiclient/agent_presets.go`（`agentPresets.list/read`）+ `cmd/app/agent_preset_wiring.go`；证据 [official-agent-preset-evidence-rc2.md](official-agent-preset-evidence-rc2.md)。**剩余差异**：会话内 seat（新任务 chip / 会话头标签）与「查看配置」composition viewer 未接 |
| 4 | `dsh-client-ui-agent-team` | ✅ IMPLEMENTED | `plugins/dsagentteam` |
| 5 | `dsh-client-ui-approval` | ✅ IMPLEMENTED | `dscomposer` + 壳 |
| 6 | `dsh-client-ui-attachment` | ✅ IMPLEMENTED | uikernel 槽 + `dscomposer` + `dsconversation` + **`plugins/dsattachment`（rail / 拖入 / 三态 / 翻页）** + **`internal/apiclient/prompt_parts.go`（`fileUploads.upload` + `session.prompt` 的附件在前、文本最后）** + **`cmd/app/wpc1_send.go`（发送前把 ready 卡换回执，成功后 `Commit`）**；证据 [official-attachment-stats-evidence-rc2.md](official-attachment-stats-evidence-rc2.md)。**剩余差异**：图片缩略图/灯箱无像素解码路径（消息区 64×64 占位瓦片），非图片附件全真 |
| 7 | `dsh-client-ui-brand-official` | ✅ IMPLEMENTED | `plugins/dsbrand` |
| 8 | `dsh-client-ui-chat` | ✅ IMPLEMENTED | `plugins/dsconversation`（18 文件 / 8054 行） |
| 9 | `dsh-client-ui-commands` | ✅ IMPLEMENTED | `plugins/dscommands`（2 文件 / 423 行） |
| 10 | `dsh-client-ui-context-menu` | ✅ IMPLEMENTED | `plugins/dscontextmenu`（服务+官方槽）+ `dssidebar` 消费（workspace 组头右键/省略号、session 行省略号）；证据 [official-context-menu-workspace-evidence-rc2.md](official-context-menu-workspace-evidence-rc2.md) |
| 11 | `dsh-client-ui-conversation` | ✅ IMPLEMENTED | `plugins/dsconversation` + `internal/uikernel` |
| 12 | `dsh-client-ui-cordis` | ❌ MISSING | 无 |
| 13 | `dsh-client-ui-creation-recommendations` | ✅ IMPLEMENTED | **`plugins/dscreationreco`（`create_personalized_page` 卡片正文：header + info 说明块 + summary/预填内容折叠 + 位置四项菜单 + 「自行撰写」textarea + 四个动词 + **11 档 phase** + 8 条 reason 文案）+ 复用 `pluginapi/action_card_bodies.go`（正文注册表）与 `plugins/dsactioncards`（按 kind 派发）**；**零新增宿主方法**（官方 `inject` 里只有 locale/actionCards，卡片数据全在 props 上；`testdata/api.snapshot.json` 不变）；证据 [official-creation-recommendations-evidence-rc2.md](official-creation-recommendations-evidence-rc2.md)。**`refused{creator-unavailable}` 逐字走官方兜底档 `unavailable`**（"暂时无法执行，请到插件页面查看"——官方 REASON_KEYS 只有 8 条，其余 reason 一律读作 unavailable，见证据文档 §4）。**剩余差异（5 处，均登记在证据文档 §7）**：flex-wrap 无对等物 / `textarea maxLength=1000` 无对等物 / info 图标用官方 `IconInfoOutlineRegular` 顶替未导出的 warning-triangle / 菜单无"当前项高亮" / ARIA 角色无对等物 |
| 14 | `dsh-client-ui-deliverables` | ✅ IMPLEMENTED | `plugins/dsdeliverables`（写过的文件 + **按文件 before/after diff**）+ `dsconversation/file_refs.go` + **`internal/apiclient/changes.go`（`/api/changes.summary\|diff\|open` 裸路由）**；证据 [official-deliverables-changes-evidence-rc2.md](official-deliverables-changes-evidence-rc2.md) |
| 15 | `dsh-client-ui-directory-picker-browse` | ✅ IMPLEMENTED | `plugins/dsdirpicker` + `internal/apiclient/dirpicker.go` |
| 16 | `dsh-client-ui-directory-picker-native` | ❌ MISSING | 无 |
| 17 | `dsh-client-ui-entry-customization` | 🟡 PARTIAL | **本体已落地（2026-11-15）**：`plugins/dsentrycustom`（三处菜单**行**的内容插件）+ `plugins/dssidebar/panelapi`（官方 `pluginNavigation` 客户端服务的等价物，ds-sidebar 提供 `OpenList()`）+ 两个宿主补的 owner props（`pluginapi.EntryMenuEntry.Glyph` = 官方 `sidebar.panel.icon` 工厂槽的投影；会话头 entries 补 `active`）。**做了**：① 左栏未固定条目行（`entries.filter(!pinned)`，行首面板图标，行尾 22×22「固定」按钮 `pin-outline` 14，点了 `onDismiss()+onPinnedChange(id,true)`）；② 会话头折起 view 行（`moreEntries = entries`，active 行尾 `check-outline`，无固定按钮）；③ 「浏览更多插件」（`plugin-pinwheel-outline` 16，服务在册才画，点了 `onDismiss()+openList()`）。行几何逐条取 rc.2 产物：`_item_70kcw_173`（min-height 34 / gap 6 / padding 6,8 / 13-20 / radius md / hover `interactive-bg-hover`）、`_itemIcon_70kcw_233`（14×14，svg 同夹 14）、`_separator_70kcw_157`（.5px, margin 3 8）、`EntryMenu.module.css`（`.pinAnchor{right:6}` 22×22 / `.pinButton{color:label-tertiary}` / `.entryName{padding-right:28}`）。**HIDE 三项（逐条登记，不是"以后再说"）**：「创建你的插件」整行——宿主 `internal/transport/server.go` 的 `handleSessionCreate` 只解 `Mode`、`opt.Create` 签名是 `func(mode string)`，收不了 `agentPreset:'cordis'`（画出来点了只会建出没有预设的会话）；右栏 guide 的 More 卡片——guide 面未落地（#53），`sidebar.right.tab.guide.actions` 没有派发方；左栏行尾的对勾——官方 `.sidebarEntry .activeMark{opacity:0}` 只在 hover 出现，viva 没有"本帧是否悬停"的读口，按非 hover 态画。证据 [official-entry-menu-evidence-rc2.md](official-entry-menu-evidence-rc2.md) **（此前考古，一并保留）** **无实现；本轮考古推翻上一批的两条判断、确认第三条**（详见下方「#17 考古」）。三个 owner 座位**都在**（`internal/uikernel/official_slots.go:255` `sidebar.panellist.actions` / `:312` `conversation.session.header.views.actions` / `:466` `sidebar.right.tab.guide.actions`，且骨架都 dispatch）；`pluginNavigation` **不是宿主能力**（是 plugin-manager 的客户端服务 `ctx.reflect.provide`，`dsh-client-ui-plugin-manager/src/client/index.ts:50-60`、`:153`），本仓等价物就是 `plugins/dssidebar/plugins_page.go` 那个插件页；真正缺的是 **Creator 预设**：`uiWorkspace.startSession(undefined,{agentPreset:'cordis'})` 要宿主 `session.create` 收 `agentPreset`，而本仓 `internal/transport/server.go:952` 的 `handleSessionCreate` **只读 `mode`**、`opt.Create` 签名是 `func(mode string)`（`server.go:175`）。此外三处"More 菜单宿主"本仓都还没有（官方在 `SidebarRoot.tsx:123-146` 的 `PanelMoreMenu`），没有宿主就没有可放的行——这是本条的真正前置 |
| 18 | `dsh-client-ui-file-actions` | 🟡 PARTIAL | **`plugins/dsfileactions`（`file` order 200 / `image` order 100 两段菜单：`下载` / `另存为…`）** + `pluginapi/context_menu.go`（新增 `file`/`image` 两个 kind + 官方逐族字段）+ `plugins/dsfiles`（文件行是 `file` target 的生产方，带会话 + 工作区相对路径）+ `internal/apiclient/workspace_files.go`（`workspaceFiles.readBytes`，base64 解码）+ `cmd/app/file_actions.go`（壳侧：原生另存 / 系统下载目录 / 状态栏反馈）+ `pluginapi.Actions.SaveWorkspaceFile`；证据 [official-file-actions-evidence-rc2.md](official-file-actions-evidence-rc2.md)。**剩余差异**：「复制图片」不画（viva 无剪贴板图片**写**入口）、`image` target 今天没有生产方（消息区图片是占位瓦片）、「在文件夹显示」没接（宿主无 `openWorkspacePath`）、没有 `stat` 前置的置灰态 |
| 19 | `dsh-client-ui-file-drag` | ❌ MISSING | 无 |
| 20 | `dsh-client-ui-goal` | ✅ IMPLEMENTED | `plugins/dsgoal`（4 文件 / 364 行） |
| 21 | `dsh-client-ui-input-trigger` | ✅ IMPLEMENTED | `plugins/dscomposer/trigger.go` + `internal/apiclient/references.go`（`@` 文件/会话 + `/` 技能；detect/分组/Tab drill/pick 路由） |
| 22 | `dsh-client-ui-jobs` | ✅ IMPLEMENTED | `plugins/dsjobs`（官方 `conversation.session.header.actions` order 20 触发器 + `shell.overlay` 弹层；旧三栏 `sidebar.footer` 清单）+ `internal/apiclient/jobs.go`（`jobs.output`/`jobs.stop`）+ `cmd/app/jobs_actions.go` |
| 23 | `dsh-client-ui-layout` | ✅ IMPLEMENTED | `internal/uikernel` 骨架 + `plugins/dsshell` |
| 24 | `dsh-client-ui-message-feedback` | ✅ IMPLEMENTED | `plugins/dsconversation/feedback.go` + `internal/apiclient/message_feedback.go` + `cmd/app/reference_wiring.go`（durable + 弹窗 + 冲突收敛 + toast） |
| 25 | `dsh-client-ui-model-selection` | ✅ IMPLEMENTED | `internal/uikernel/official_model_menu.go` + `dscomposer` |
| 26 | `dsh-client-ui-open-in-app` | ✅ IMPLEMENTED | `plugins/dsopen`（3 文件 / 163 行） |
| 27 | `dsh-client-ui-permission-presets` | ✅ IMPLEMENTED | `dscomposer` + `dssettings` |
| 28 | `dsh-client-ui-plan` | 🟡 PARTIAL | **`plugins/dsplan`（`cards.go` Turn 尾计划卡 → `preview.go` 右栏计划全文 `<->` `plan.go` 帧解析/地址）**；数据面零新增（journal `tool/call` 的 `exit_plan_mode` 参数，`pluginapi.Snapshot.Frames`）。**缺两座（HIDE）**：`conversation.input.plan` 要宿主 `plan` 投影 `{active,pending}`（`session.values` 没有）、`conversation.plan-review.actions` 要 plan-review 待答问题（宿主 `exit_plan_mode` 直接 `Stop`，不产生问题）；证据 [official-plan-evidence-rc2.md](official-plan-evidence-rc2.md) |
| 29 | `dsh-client-ui-plugin-manager` | ✅ IMPLEMENTED | `plugins/dssidebar/plugins_page.go`（侧栏「插件」管理页：列表/详情/启停/卸载）+ `internal/apiclient/plugin_manager.go` + `cmd/app/plugin_manager.go`；证据 [official-plugin-manager-evidence-rc2.md](official-plugin-manager-evidence-rc2.md) |
| 30 | `dsh-client-ui-plugin-market` | ✅ IMPLEMENTED | **A/B/C 三块齐**：`plugins/dsmarket/discover.go`（官方 `plugins.tab` discover 页：索引菜单 / 搜索框 / 标签过滤 / 排序菜单含方向切换 / 刷新 / 分类胶囊 / 卡片 / 骨架 / 两种空态 / 显示更多 / 索引源对话框）+ `plugins/dsmarket/discover_projection.go`（官方 `presentation.ts` 的 query/声明排序/菜单 id/sourceFacts 等价物）+ `plugins/dssidebar/plugins_tabs.go`（`plugins.tab` 台账 + tab 条 + 贡献方座位，官方 `PluginManagerPage.tsx:1732-1760`）+ `internal/apiclient/plugin_market_discover.go`（`listIndexes`/`search`/`setIndexes`，形状与官方逐字段一致）+ `cmd/app/market_discover.go`（壳状态机，照 `market-store.ts` 移植）+ `plugins/dsmarket/detail.go`（C 块详情页，`ui.market.detail` 服务已挂上）；证据 [official-plugin-market-discover-evidence-rc2.md](official-plugin-market-discover-evidence-rc2.md)。**剩余差异 12 条（HIDE，宿主缺口 6 + 素材/能力 6，逐条见该文件 §5）**：安装/更新/在已安装中查看不画（宿主 501）／详情相对图片不出（无 `detail-file` 路由）／条目图标图不出（无图片加载器与官方精灵图，框内画标题首字）／刷新·移除·过滤三枚 sprite 无导出（文字化）／骨架不带动画／工具栏不换行（Viva `Flow` 不支持 flex:1）／`Pill`·`Tag` 用仓库已冻结的同一份官方度量／卡片悬停底色不画／`readInvalid` 档不可达（宿主把非法索引也回 `index_unavailable`）／无 `plugin-market/changed` 事件（不自刷新）／`listIndexes` 空配置回 503（客户端认成空清单，待宿主改回 `[]`）
| 31 | `dsh-client-ui-plugin-recommendations` | ✅ IMPLEMENTED | **`plugins/dspluginreco`（`enable_official_plugin` 卡片正文：header/目录标题+描述/状态行/footer 两个按钮）+ `pluginapi/action_card_bodies.go`（正文注册表）+ `plugins/dsactioncards`（注册表提供方与按 kind 派发）+ `internal/apiclient/recommendations.go`（`pluginRecommendations.describe`）+ `cmd/app/official_panels_wiring.go`（describe + 查看详情接线）**；证据 [official-plugin-recommendations-evidence-rc2.md](official-plugin-recommendations-evidence-rc2.md)。**剩余差异**：目录描述到达前的骨架条不画（主题无 `--dsw-alias-bg-skeleton` 对等令牌，不自造灰色）；「查看详情」是标题行可点（官方 `.details` 是整块按钮） |
| 32 | `dsh-client-ui-reference` | ✅ IMPLEMENTED | `plugins/dsconversation/usertext.go`（file/folder/session chip + PreviewFile）+ `plugins/dscomposer/trigger.go` + `internal/reficon` |
| 33 | `dsh-client-ui-renderer` | ✅ IMPLEMENTED | `internal/uikernel`（槽 dispatch） |
| 34 | `dsh-client-ui-schedule` | ✅ IMPLEMENTED | `plugins/dssidebar/schedule_page.go`（任务管理页）+ `internal/apiclient/schedule.go`（`schedule.catalog/edit/run/history`）+ `cmd/app/schedule_catalog.go`；侧栏摘要仍在 `plugins/dsschedule` |
| 35 | `dsh-client-ui-secret-mask` | ✅ IMPLEMENTED | `internal/secretmask`（官方格式表）+ `plugins/dsconversation/usertext.go`（逐值显示/隐藏） |
| 36 | `dsh-client-ui-session` | ✅ IMPLEMENTED | `internal/apiclient` + `cmd/app` |
| 37 | `dsh-client-ui-settings` | ✅ IMPLEMENTED | `plugins/dssettings`（14 文件 / 5470 行） |
| 38 | `dsh-client-ui-settings-account` | 🟡 PARTIAL | `dssettings` 的 `account` section |
| 39 | `dsh-client-ui-settings-agent-loop` | ✅ IMPLEMENTED | **`plugins/dssidebar/settings_items.go` + `settings_items_draw.go`**（插件管理页「内置插件配置」组的 `agent-loop` 卡：`maxParallelToolCalls` 一个数值字段，官方 lock/`hint` 逐字，写走 `settings.mutate`）+ `plugins/dssidebar/plugins_page.go`（组与详情接线）；证据 [official-settings-pages-evidence-rc2.md](official-settings-pages-evidence-rc2.md) §2.1 |
| 40 | `dsh-client-ui-settings-general` | ✅ IMPLEMENTED | `dssettings` 的 general 页 |
| 41 | `dsh-client-ui-settings-models` | 🟡 PARTIAL | `dssettings` 的 models 页 |
| 42 | `dsh-client-ui-settings-plugin-inventory` | ✅ IMPLEMENTED | `plugins/dssettings/official_plugins.go` + `official_plugins_draw.go` + `internal/apiclient/plugin_inventory.go` |
| 43 | `dsh-client-ui-settings-plugins` | ✅ IMPLEMENTED | `plugins/dssettings/official_tree.go:83` + `official_plugins*.go`（外壳 + tab 台账） |
| 44 | `dsh-client-ui-settings-session-log` | 🟡 PARTIAL | `dssettings` 的 general item `session-log-upload-toast` |
| 45 | `dsh-client-ui-settings-shell` | ✅ IMPLEMENTED | 同一落点的 `shell` 卡（`timeoutMs` / `maxOutputBytes`，按官方 `served.has(PWSH_NS) ? pwsh : bash` 绑定 `pwsh-sandbox`/`bash-sandbox`）；证据 [official-settings-pages-evidence-rc2.md](official-settings-pages-evidence-rc2.md) §2.2 |
| 46 | `dsh-client-ui-settings-subagent` | 🟡 PARTIAL | 同一落点的 `subagent` 卡：**「运行限制」一节已落地**（`maxDepth` / `maxActiveSubagents` + 官方 help 正文 + `limitField` 整数/下限判据）；**「模型选择」一节 HIDE**——宿主把该命名空间的 `allowedModels`（对象数组）投影不进 `sections[].fields`（只支持三种标量），且本仓宿主这条命名空间无消费方。理由见 [official-settings-pages-evidence-rc2.md](official-settings-pages-evidence-rc2.md) §6.1 |
| 47 | `dsh-client-ui-settings-web-search` | ✅ IMPLEMENTED | 同一落点的 `web-search` 卡（`order:20`，`plugins/dssidebar/settings_items_web.go` + `settings_items_web_draw.go`）：两个 provider 下拉 + 灰面板（`web-deepseek-api` 的 `count` / `searchBilling` / `fetchBilling`，`web-search-deepseek` 的 `baseURL` / `maxUses` / 密钥）+ 官方「已被 … 覆盖」锁 + provider 清单失败提示与「重试」；provider 清单走 `web.listProviders`，密钥走 `credentials.describe`/`credentials.set`（**不进 settings**）。证据 [official-settings-pages-evidence-rc2.md](official-settings-pages-evidence-rc2.md) §8。**缺口（宿主侧，已登记）**：legacy 搜索 provider 只读环境变量（`modules/search/deepseek.go:77`），所以密钥写进凭据库后**不改变搜索行为**——界面照官方发布面画，能读写、能往返 |
| 48 | `dsh-client-ui-shortcuts` | ✅ IMPLEMENTED | `plugins/dsshortcuts`（2 文件 / 213 行） |
| 49 | `dsh-client-ui-sidebar` | ✅ IMPLEMENTED | `plugins/dssidebar`（5 文件 / 1418 行） |
| 50 | `dsh-client-ui-sidebar-browser` | 🟡 PARTIAL | **`plugins/dsbrowser`（4 文件 / 900 行）**：右栏 tab 类型 `browser`（`sidebar.right.pane.tab` + `.title`，同 key）+ `url.go`（官方 `url.ts` 45 行逐条搬：16 KiB 上限 / 隐式 scheme 补 `https://` / http(s) 白名单 / 凭据拒 / 应用自身 origin 拒 / `title=hostname`）+ `navigation.go`（官方 `BrowserNavigation.ts` 状态机：100 条上限、前进分支截断、loading→known→unknown）+ 工具条（38px、后退/前进/刷新/地址框/前往/系统浏览器打开）+ `AfterDraw` 里 `canvas.NativeView(page.ID(), rect)`。**载具是 Viva v0.6.4 自带的 webview 宿主**（`app/web.go:40` `WebHost`、`app.go:225` `Config.Web`、`gfx/native_view.go:32` `Canvas.NativeView`），本机 `EdgeWebView\Application` 有 156/157 三份 WebView2 Runtime。**上一批的 HIDE 理由（"本仓没有 iframe/webview 这一层"）是错的**。剩余差异：① 无法多实例（官方 `multiple:true`，本仓 tab 机制没有多实例/关闭，见 [official-view-tabs-evidence-rc2.md](official-view-tabs-evidence-rc2.md) G3）；② 官方 `.restore` 恢复卡没有（应用侧历史只在内存，重启即空，官方 web 载具同样如此，桌面载具才按 workspace 持久化）；③ 沙箱开关不暴露——官方 `BrowserFrame.sandbox` 是**可选**的（`BrowserFrame.ts:20` "Undefined when this provider does not expose a sandbox control"），本仓策略固定为开；④ `Navigage`/`NewWindow` 折成"窗口外打开"与"拒下载"（官方 iframe 沙箱 `IframePresentation.ts:96-99` 无 `allow-downloads`）；⑤ 地址栏"前往"按钮常显（官方 `:focus-within` 才显示），地球图标赤道线的 `stroke-linecap="square"` 画成 butt（半个描边宽，14px 下不可见） |
| 51 | `dsh-client-ui-sidebar-documentpreview` | ✅ IMPLEMENTED | `plugins/dsfiles` 预览浮层 |
| 52 | `dsh-client-ui-sidebar-files` | ✅ IMPLEMENTED | `plugins/dsfiles` |
| 53 | `dsh-client-ui-sidebar-right` | ✅ IMPLEMENTED | `dsrightbar` + `dsshell` + uikernel；右栏现在是**两阶段注册 + tab 条**（2026-11-15）：类型/体分开注册、按名录派发选中项，见 [official-view-tabs-evidence-rc2.md](official-view-tabs-evidence-rc2.md) §2.4 |
| 54 | `dsh-client-ui-sidebar-terminal` | 🟡 PARTIAL | `plugins/dsterminal` + `internal/ansi`；**官方座位已接**（2026-11-15）：终端是右栏的 `terminal` tab 类型，右栏展开时用它、收起时退回 F5 浮层。剩余缺口是**终端本体**（VT 交互、多终端、agent 终端、沙箱徽标）——见 [official-view-tabs-evidence-rc2.md](official-view-tabs-evidence-rc2.md) §5 |
| 55 | `dsh-client-ui-skill` | ✅ IMPLEMENTED | `plugins/dsskill`（7 文件 / 2238 行） |
| 56 | `dsh-client-ui-subagent` | ✅ IMPLEMENTED | 目录行本体：`plugins/dsagentteam/catalog.go`（disclosure/递归层级/token+时长指标/当前会话对勾/Esc）+ `catalog_metrics.go` + 壳侧逐子会话投影 `cmd/app/wave3_wiring.go`；footer（TeamBoard）仍是既有落点；证据 [official-subagent-evidence-rc2.md](official-subagent-evidence-rc2.md) |
| 57 | `dsh-client-ui-theme` | ✅ IMPLEMENTED | `dscore` + `cmd/app/theme.go` |
| 58 | `dsh-client-ui-tool` | ✅ IMPLEMENTED | `plugins/dstool`（7 文件 / 930 行） |
| 59 | `dsh-client-ui-trajectory` | 🟡 PARTIAL | `plugins/dsrightbar`；已注册为 `conversation.view` 的 `trajectory` 条目（`order:10`，与官方一致），页签行由注册表驱动（2026-11-15）。剩余缺口是官方的分组/折叠层级——见 [official-view-tabs-evidence-rc2.md](official-view-tabs-evidence-rc2.md) §5 |
| 60 | `dsh-client-ui-user-questions` | ✅ IMPLEMENTED | `plugins/dsquestions`（3 文件 / 423 行） |
| 61 | `dsh-client-ui-workflow-run` | ✅ IMPLEMENTED | `dsconversation` 的 workflow-run 节点 |
| 62 | `dsh-client-ui-workspace` | ✅ IMPLEMENTED | `internal/apiclient/fs.go` + `dssidebar` + `plugins/dssidebar/workspace_actions.go`（`workspace.registry`/`rename`/`delete`，#10 contextMenu 入口）+ **`plugins/dssidebar/grouping.go`（按 `registry.SessionIDs` 归属与手动序、「未分组」、registry 空时回退 cwd）+ `drag.go`（拖拽 → `workspace.insertSessionBefore`）**；证据 [official-workspace-order-evidence-rc2.md](official-workspace-order-evidence-rc2.md) |
| 63 | `dsh-experimental-client-ui-cot-translation` | 🟡 PARTIAL | **官方默认关闭**（`OFF:../dsh-official-bundles/lib/index.js:42` 在 `OPTIONAL_BUNDLES`、`:127` 在 `EXPERIMENTAL_BUNDLES`；`PROFILE_TEMPLATES:4-10` 五个模板一个都没有），本仓照官方形状做成"部件在、默认不启用"。**本体落地**：官方 `conversation.chat.reasoning.body` 座位——`plugins/dsconversation/cot_translation.go` + `conversation.go:665-685`（展开的思考里画工具栏 + 正文两态：`查看译文`/`查看原文` · `正在翻译思考内容` · `翻译暂不可用，已显示原文` + `重试`），分段/拼接/顺序推进逐条对齐官方 `reasoningFragments` / `emit()` / `run()`（`OFF:src/client/translation.ts:20-113`）；数据面 `internal/apiclient/cot_translation.go`（`cotTranslation.limits` / `cotTranslation.translate`，两方法都是宿主**真实现**，走本机模型网关）+ `cmd/app/cot_translation_wiring.go`（limits 同步缓存 + 首问后台拉；translate 只发起、终态按 jobID 取走即清空）+ `pluginapi/plugin.go` 三条 Action；契约进 `testdata/api.snapshot.json`（batch 19）。测试：`cot_translation_test.go`（分段/代理对/UTF-16/拼接/状态机/失败/重试/入口可用性）、`cot_translation_visual_test.go`（真 Viva 绘制+点击，两尺寸、pump 到条件成立、失败与重试、宿主没接时不画）、`cmd/app/cot_translation_wiring_test.go`（真 HTTP：缓存/取走即清空/失败映射/参数形状）。**剩余缺口（2 条）**：① 官方第二处座位 `plugins.bundle.config` 的 provider/targetLanguage 表单 **HIDE**——宿主没有 `cot-translation` 命名空间的设置写入路径（`grep -rn "cot-translation" ../DSH-GO/internal ../DSH-GO/cmd` 零命中，`limits` 固定回 `deepseek-official`/`auto`）；② 官方正文走 `renderFactorySlot(conversation.chat.reasoning.content)`（Markdown + 代码块工具条），本仓原文/译文同为 `ui.Wrap` 纯文本（沿用 `reasoning.go` 那条既有刻意差异）。逐条证据、依赖表与三包考古结论：[official-experimental3-evidence-rc2.md](official-experimental3-evidence-rc2.md) |
| 64 | `dsh-experimental-client-ui-forge` | 🟡 PARTIAL | **A 档 14 方法已做，CR 相关因 G1/G2/G3 HIDE**（官方默认关闭：`OFF:dsh-official-bundles/lib/index.js:48` / `:125`）。**做出来的三块**：`plugins/dsforge`（`conversation.hero.options` 工作树勾选 / `conversation.input.dock` 本地变更卡 / `sidebar.right.pane.tab` 的 Changes 视图本地那一半）+ `internal/apiclient/forge.go`（`branches`/`localDiff`/`localText`/`requestWorktree`/`cancelWorktree`/`retryWorktree`/`stopCreation`/`restoreWorktree`/`continueInProject`/`track`/`dismiss`/`diffBases`/`localCommits`/`worktreeCleanup` 共 14 条，快照 batch 21）+ `cmd/app/forge_wiring.go`（缓存/异步读/动作/会话世代判定）。数据源全是宿主真实现：`HOST:cmd/dshd/serve_forge_git.go:54/62/74`、`serve_worktree_api.go:173`、`serve_forge_state.go`、`serve_forge_rc2.go:235/257/330`。**HIDE（#64b）**：全部 CR 卡片内容与会话列表字形（G1：官方 `changeRequestStates` 是 `mode:'stream'` 零参数帧，宿主走 unary + 只认 env；G2：`refresh`/`follow` 的 provider 是占位；G3：凭据链三套互不相通）、`gh auth login` 横幅，外加**本轮新查出的 G6**：`forgePanel.follow` 流的是绑定清单帧而不是官方 `ForgePanelState`（`OFF:lib/typert.host.js:1752` vs `HOST:internal/transport/handlers_forge_panel_state.go:300-345`），所以工作树 progress/missing 两张卡与 toggle 的 blocker 层没有读出口，一并 HIDE。**剩余差异 11 条**（座位、范围菜单位置、无文件树/左右对比、无 diff 底色令牌、刷新图标、分支图标、blocker 挡板、无创建进度、已读只在本进程、`localDiff` 不认 base）逐条见 [official-forge-a-evidence-rc2.md](official-forge-a-evidence-rc2.md) §5-§6。 |
| 65 | `dsh-experimental-client-ui-stats` | ✅ IMPLEMENTED | **`plugins/dsstatusbar/stats_view.go`（官方 `conversation.view` `order:40` 的 `stats` 页面本体：逐 turn/step 11 列 + 总用时条 + 全量/窗口合计；帧投影对齐官方 `stats-fold.ts` / `stats-usage.ts` / `stats-format.ts`）** + `plugins/dsstatusbar/statusbar.go:129-130` 注册；证据 [official-attachment-stats-evidence-rc2.md](official-attachment-stats-evidence-rc2.md)。**剩余差异**：compaction/非 step 事件行、三个工具栏开关、加载更早分页未做 |
| 66 | `dsh-experimental-client-ui-voice-input` | ❌ MISSING | **官方默认关闭**（`OFF:dsh-official-bundles/lib/index.js:41` / `:98` / `:122`）。**HIDE + 登记缺什么**：宿主这半边**是真的**——`HOST:cmd/dshd/serve_speech.go:5-60` 的 `newSpeechAPI()` 接 `internal/speech`（受管 C# helper 跑 Windows System.Speech），6 方法全实现、路由 `HOST:internal/transport/handlers_speech_official.go:86-93`（`handlers_media_market_official.go` 里那 4 条旧 501 已被这个分支取代）。**缺的是客户端采集三样**：① 麦克风枚举与录音（官方 `OFF:src/client/audio.ts:59-63` `getUserMedia` + `:43,95` `MediaRecorder`；本仓 Go+Viva 没有 DOM/WebAudio，`grep -ln "WASAPI\|waveIn\|Microphone" cmd internal plugins` 零命中，基线 `CGO_ENABLED=0`）；② 重采样到 16 kHz 单声道（官方 `:152` `OfflineAudioContext(1,…,16000)`）；③ WAV 编码 + base64（`speech/transcribe` 的入参）。录音按钮 / 波形 / 麦克风选择 / 设置对话框全依赖这三样，一样都没有就画不出真东西。证据：[official-experimental3-evidence-rc2.md](official-experimental3-evidence-rc2.md) §4 |

> #39 / #45 / #46 / #47 四行的**页面落点、逐字文案、表单状态机、几何台账与差异**、以及
> #46 的 HIDE 理由、#47 的三命名空间/密钥路径与缺口：[official-settings-pages-evidence-rc2.md](official-settings-pages-evidence-rc2.md)。
> 它们不是设置面板的 section，而是插件管理页 `plugins.item` 的卡片（官方锚点见该文件 §1）。
>
> #29 / #42 / #43 三行的完整结构与几何证据（官方 `src/**` 锚点、逐字 locale、HIDE 台账）：[official-plugins-page-evidence-rc2.md](official-plugins-page-evidence-rc2.md)。其中 #43 的「内置/第三方分 tab」在 rc.2 **不存在**——`settings.plugins.tab` 全仓只有只读清单一个贡献方。
>
> #22 的结构与几何证据、与官方的 9 条差异台账：[official-jobs-evidence-rc2.md](official-jobs-evidence-rc2.md)。
>
> #34 的结构与几何证据、与官方的差异台账：[official-schedule-evidence-rc2.md](official-schedule-evidence-rc2.md)。未做面：会话头提醒目录、`schedule_create` 转录卡、会话行时钟标记、手动创建表单、可编辑规则、⋯ 菜单（理由与缺口见该文件 §4）。

> **多视图座位（会话 view / 右栏 tab）机制**（2026-11-15）：官方那两处「一个位置、多个视图轮流上」的座位，
> 本仓现在有注册表 + 按名录派发 + 选择器绘制三件套，`conversation.view` 与 `sidebar.right.pane.tab` 都是真的
> keyed/list 派发（此前骨架 `has→return`，这两族槽**永不 dispatch**）。逐条证据、落点、与官方的 6 条缺口：
> [official-view-tabs-evidence-rc2.md](official-view-tabs-evidence-rc2.md)。机制落地时 `#54`/`#59`/`#65`
> 都还是 🟡；`#65` 的页面本体已于 2026-11-15 落地（[official-attachment-stats-evidence-rc2.md](official-attachment-stats-evidence-rc2.md)），
> `#54`/`#59` 仍差页面本体。

## 合计：官方 66 个部件 —— ✅ 50 · 🟡 12 · ⬜ 0（不该有） · ❌ 4

> 剩下 5 条 MISSING 的**逐条核查结论、精确阻断点与下一批排序**（A 档"宿主已在只差界面" /
> B 档"要一次架构判断" / C 档"宿主缺前置" / D 档"按 HIDE 登记"）见
> [official-missing-batch-a-findings-rc2.md](official-missing-batch-a-findings-rc2.md)。


> 这五个数是**从下面那张表里数出来的**（脚本按 `^- [状态] \`名字\`` 去重统计），
> 不是手填的。上一次手填的是"我们有 10 个 · 缺 53 个"——**已经错了很久**，
> 因为每次补完一个部件都没人回头改这个数。所以这次把它写成"数出来的"。

## #17 考古（2026-11-15）：上一批三条判断里，两条错、一条对

上一批给 `dsh-client-ui-entry-customization` 的结论是：
「需插件贡献侧栏条目 + Creator 预设 + `pluginNavigation`（本仓三者皆无）」。逐条核：

| # | 上一批的判断 | 考古结论 | 证据 |
|---|---|---|---|
| 1 | 需"插件贡献侧栏条目" | **对，但只看了一半** | 三个 owner 座位**都已存在**并且骨架都 dispatch：`internal/uikernel/official_slots.go:255`（`sidebar.panellist.actions`，list/root）、`:312`（`conversation.session.header.views.actions`）、`:466`（`sidebar.right.tab.guide.actions`）；派发点 `internal/uikernel/shell_skeleton.go:754`、`:854`、`:1090`。缺的**不是座位**，是座位**外面那层菜单**——官方三处都挂在各自区域的 More 菜单里（`dsh-client-ui-sidebar/src/client/SidebarRoot.tsx:123-146` 的 `PanelMoreMenu`，行内容由 `EntryMenuContent.tsx:35-70` 画），本仓三处都没有 More 菜单宿主 |
| 2 | 需 `pluginNavigation`（本仓无） | **错** | `pluginNavigation` 不是宿主能力，是 plugin-manager 的**客户端服务**：`ctx.reflect.provide('pluginNavigation', {openList, openBundle})`（`OFF:dsh-client-ui-plugin-manager/src/client/index.ts:153-166`，类型声明同文件 `:50-60`）。entry-customization 只是在它**存在时**多画一行"浏览更多插件"（`src/client/index.ts:32-39` 的 `ctx.inject(['pluginNavigation'], …)`）。本仓等价物是现成的：插件管理页 = `plugins/dssidebar/plugins_page.go`（`panelRoutes()` 的 `plugins` 路由），"打开插件列表"就是选中那个面板 |
| 3 | 需 Creator 预设（本仓无） | **对** | `createPlugin` 走 `uiWorkspace.startSession(undefined, {agentPreset: 'cordis', prompt, whenDraftExists: 'append'})`（`OFF:dsh-client-ui-entry-customization/src/client/index.ts:47-52`）。`agentPreset` 一路传到 `openWorkspace(target, beforeOpen, {agentPreset})`（`OFF:dsh-client-ui-workspace/src/client/navigation.ts:337-372`），落点是宿主建会话时要认 preset。本仓宿主**建会话不认 preset**：`internal/transport/server.go:952` 的 `handleSessionCreate` 只解 `Mode`，`internal/transport/server.go:175` 的 `opt.Create` 签名是 `func(mode string) (string, error)`。`internal/session/log.go:30` 虽然有 `CreateSession(home, cwd, id, agentPreset)`，但没有一条方法能把 `agentPreset` 从请求送到它 |

**顺带纠正一处**：`pinned`（"固定到左侧栏"）也**不是**宿主能力——官方 `createPanelPins()` 只存在内存里，
"Ids of unpinned panel launchers held in memory until plugin reload"（`OFF:dsh-client-ui-sidebar/src/client/panel-pins.ts:9-33` +
`contract/slots.ts:219`），reload 即丢，没有任何持久化方法。

**结论**：本条的真正前置不是"宿主加方法"（那只影响 Creator 那一行），而是 **三个区域的 More 菜单宿主 +
面板进场/固定状态**——那属于 `dsh-client-ui-sidebar`（#49）/ `dsh-client-ui-conversation`（#7）/
`dsh-client-ui-sidebar-right`（#53）的 chrome，三条在账本上已是 ✅，但它们的 More 菜单/固定这两块
官方 chrome 本仓没搬。按 HIDE 登记，不自己发明一个"更多"按钮放在别处冒充。

## 缺的里面，用户最能直接感知的

### 已补掉（2026-11-04）

| 部件 | 现在是什么 | 数据来源 |
|---|---|---|
| `model-selection` | F2 命令面板列出网关的真实模型 | 宿主 `models.list` → 网关 `GET /v1/models` |
| `permission-presets` | F2 命令面板 5 条（4 个值 + 恢复默认） | 宿主 `session.setPermission`（白名单） |
| `sidebar-files` | 侧栏文件树，可进子目录 | 宿主 `workspace.list(path)` + `fs.read` |
| `sidebar-documentpreview` | 点文件 → 右上浮层看全文 | 同上（16KB 上限、20 行截断） |
| `shortcuts` | F1 键位表 | 本仓真实的 `HandleKey` 绑定 |

### 仍然缺

1. **`attachment`** —— 没有附件入口（画了就是死按钮，所以没画）
2. **`settings*`（9 个）** —— 完全没有设置界面，改配置只能改 JSON
3. **`sidebar-terminal`** —— 官方侧栏能开终端
4. **`reference` / `input-trigger`** —— **2026-11-14 已接**（原先是"补全不是引用"）。
   现状与剩余差异：

   | 官方那一层 | 我们 |
   |---|---|
   | `@` 识别 + 候选菜单 + 选中 | ✅ `plugins/dscomposer/trigger.go`：`@` 文件（宿主 `fileReferences.list`）+ 会话（`sessionReferenceResolver.candidates`）+ `/` 技能（`skills.list`）三源；官方 detect 的引号路径与 URL 斜杠 carve-out；Enter/Tab/↑↓/Esc |
   | 把 `@path` 变成**引用 chip** | ✅ `plugins/dsconversation/usertext.go`：用户文本里的 `@path` / `@[label]` 加 `dsh-session:` 的 wire 引用 画成 file/folder/session chip；file chip 点击走 `Actions.PreviewFile` |
   | 结构化编辑器节点 | ❌ 本仓输入框是**纯文本**（viva `ui.Field` 没有 DecoratorNode），pick 的结果是官方 wire 文本形式，发送出去的仍是纯文本；官方 React editor 的 chip 节点无法在 Go 侧一比一复刻 |

   **最后一行是剩余差异**，但它不影响"引用可识别、可显示、可点击"这三件事；
   发送层仍是纯文本（这与官方"plain-text-reference decision"一致：官方也把
   `ref` 作为文本发出去）。
5. **`jobs.output` 的界面消费** —— **2026-11-14 已做**（#22）：官方档 header 触发器 + 弹层、旧三栏档侧栏清单都能展开读输出（rune 游标分页、`truncated`/错误/加载更多/复制），跑着的行有两段式停止。之前不做的那条理由（"轮次结束就报错"）没有消失，但**报错是可显示的事实**（官方 `output.error` 也是一行错误），所以不再构成"不做"的理由
6. **`deliverables`** —— **3/4**（2026-11-15：per-file diff 已接，`/api/changes.summary|diff|open`）。官方包的 description 写得很清楚：
   > Changed-files card with per-file comparison tabs, delivery cards, and clickable final-response file references

   | 官方能力 | 我们 |
   |---|---|
   | Changed-files card | ✅ `dsdeliverables`（从 `write`/`edit` 工具参数提路径） |
   | **per-file comparison tabs**（点文件看 diff） | ✅ **2026-11-15 已接**：宿主 `workspace/changes` 帧的 `seq` → `/api/changes.summary`（文件级 added/deleted）→ `/api/changes.diff`（`index` 定位）→ 浮层逐行 hunk；`binary`/`oversized` 只显示官方状态词、零行 hunk；404 退回全文预览并写明「这一轮没有变更记录」。证据 [official-deliverables-changes-evidence-rc2.md](official-deliverables-changes-evidence-rc2.md) |
   | **delivery cards** | ❌ 不清楚它的触发条件（可能是回复里显式的交付声明） |
   | **clickable final-response file references** | ✅ 回复正文里"工具真写过的路径"变成可点链接（`dsconversation/file_refs.go`：只认 `write`/`edit` 参数里的明确路径，不猜 bash/未知路径/URL/行内码/围栏代码；`#L24-L30` 留在显示与 fragment，不进文件路径） |

   **剩下的是第 3 项**：官方 delivery card 的触发条件仍是"回复里显式的交付声明"，本仓没有那份语义约定，不猜。
   （第 2 项 per-file diff 与第 4 项可点击文件引用都已接，见上表。）

## 说明

- ✅ 只表示"我们有一个同名/近义的插件"，**不表示功能等价**（比如我们的 `dsconversation` 远不如官方 `conversation` + `chat` + `trajectory` + `tool` 细分后的能力）
- 这个清单是**方向图**，不是派工单；真要做要按 UIPLAN 补项、逐项验收
## 补第一项时挖到的根因：**宿主没有"改模型/改权限"的 API**

我本来想先补最痛的 `model-selection` / `permission-presets`（把 composer 上那两枚只读文本变成能点开的下拉）。
查下来发现做不了，**不是界面没写，是宿主没给**：

| 能力 | 宿主现状 | 结论 |
|---|---|---|
| `config.set` | 只写 `<home>/state/ui-config.json`（**UI 配置**，键值随便存，宿主不解释） | 不能改模型/权限 |
| 模型 | `session.load` 的 `Model` 字段（omitempty，要有 `request/context` 才有），**只读** | 无设置入口 |
| 权限 | 宿主**启动参数** `--policy`，也回在 `session.load.permission`，**只读** | 无设置入口 |
| `session.create` | 有 `mode` 参数 | 只能建会话时定，不能改现有会话 |

所以这两枚要变成下拉，**必须先有跨仓契约**：
- 要么加 `session.setModel` / `session.setPermission`（改当前会话）
- 要么让 `session.prompt` 收 `model` / `permission` 覆盖（改下一轮）

按 AGENTS §3「跨线需求只写 `docs/transport-contract.md`」，这属于**需求**，不属于我能单方面实现的界面功能。
**我没有为了让 composer 看起来更满而画一个点不动的下拉**——那正是 AGENTS 明确禁止的"死按钮"。

**同样受此限制的还有**：`attachment`（没有上传通道）、`settings*`（9 个，宿主没有对应读写面）、
`sidebar-files` / `sidebar-terminal`（宿主有 `fs.read`/`fs.write`，但没有目录列举与终端会话）。

**这解释了"为什么差得很远"的一半**：66 个官方部件里，相当一部分依赖宿主提供的能力面，界面单方面补不出来。

## 53 个缺口按「谁挡着」分类

分三类，只有第一类我能**现在动手**。

### A. 界面自己就能做（不依赖宿主新 API）—— 8 个

| 部件 | 为什么现在能做 |
|---|---|
| `shortcuts` | 纯 UI：一个键位说明面板，键位是界面自己定的 |
| `theme` | `dscore` 已有 `Dark()`，加亮色/跟随系统就行 |
| `message-feedback` | ✅ 已升级为宿主 durable（`messageFeedback.*`）+ 官方弹窗与冲突收敛 |
| `trajectory` | **我们已有**（`dsrightbar`），只差官方那种分组/折叠层级 |
| `tool` | 工具卡的独立插件化（现在写在 `dsconversation` 里） |
| `commands` | 命令面板，动作走现成的 `Actions` |
| `layout` | 三栏求解已有（`dsshell`），差的是可拖拽分栏 |
| `brand-official` | 官方鲸鱼 logo 已转（`brand_path.go`），差的是品牌包扩展点 |

### B. 宿主已有能力面，界面没接 —— 6 个

| 部件 | 宿主已有什么 | 缺什么 |
|---|---|---|
| `sidebar-files` | `fs.read` / `fs.write` / `workspace.list` | **目录列举**（`workspace.list` 只给顶层） |
| `sidebar-documentpreview` | `fs.read` | 只需要渲染 |
| `reference` | ✅ `workspace.list` + `fileReferences.list` + `sessionReferenceResolver.candidates` | 已接（2026-11-14）：候选/插入/chip 全通；仅结构化编辑器节点不复刻 |
| `deliverables` | `fs.write` | 需要"哪些是交付物"的约定 |
| `jobs` | 后台任务概念已有；`jobs.list`/`jobs.output`/`jobs.stop` 现在都是真方法 | ✅ 已接（2026-11-14，#22）；仅剩宿主没有 `JobView.kind/status/startedAt` 等字段 → 不画 kind/时长/stopping/killed |
| `schedule` | 无 | 需要定时任务的存储与方法 |
| `skill` | **工具行不需要任何方法**（官方 `skillRowModel` 只读 durable 的 call/result slice） | 斜杠候选源要 `skills/list`（`SkillEntry{name, description, modelInvocable, path}`）；`openSkill` 还要 `path` 才能 `sidebarRight.openResource` |

### C. 必须宿主先加跨线契约 —— 38 个（含 11 个设置类）

`settings*`(11) · `model-selection` · `permission-presets` · `attachment` · `plugin-manager` ·
`plugin-market` · `settings-plugin-inventory` · `sidebar-terminal` · `sidebar-browser` ·
`sidebar-right` · `subagent` · `agent-preset` · `goal` · `plan` · `user-questions` ·
`workflow-run` · `secret-mask` · `open-in-app` · `entry-customization` · `directory-picker-*` ·
`input-trigger` · `deepseek-model-label-1017` · `renderer` · `workspace` · `session` · `cordis` ·
`approval`(我们已有简化版) · `chat`

**结论**：真正能靠界面补齐的是 **A 类 8 个 + B 类 6 个**；C 类 39 个必须先有宿主契约——
这也解释了为什么"看起来差得远"却一时补不上：**大头在宿主那一侧**。

### 我这一轮选哪个动手

**`shortcuts`**（A 类第一个）：纯 UI、零依赖、用户能立刻感知（现在按 F12 收起 ds-demo 这类提示散落在各处，
没有一个地方能查"我还能按什么键"）。做完要能说清每个快捷键**真实绑定在哪里**，不编。

## A 类 8 个的实际进度与难度修正（做完 4 个之后的复盘）

| 部件 | 状态 | 实际难度 | 教训 |
|---|---|---|---|
| `shortcuts` | ✅ | 中 | 卡在**重绘**，不是布局。`-keys` 注入绕过 `OnKey` 的 Invalidate |
| `theme` | ✅ | 低 | `dscore.Light()` 早就存在，只是 `fixedDark: true` 没人能选 |
| `commands` | ✅ | 中 | 卡在 `Row` 写成 `Column`（一行 vs 多行） |
| `message-feedback` | ✅ | 中 | 关键不是按钮，是**边界**：没有上报通道就不许写"已上报" |
| `layout` | 🟡 拖拽完成 | **高** | 拖拽本身不难，**持久化要穿三层契约**（apiclient → Actions → cmd/app） |
| `trajectory` | 🟡 派单中 | 中 | 已有平铺版，差分组折叠 |
| `tool` | 🟡 派单中 | **高** | 抽取时要"按类型分派"的结构，简单搬代码等于没拆 |
| `brand-official` | ⬜ | 中 | 要先把 `brand_path.go` 从 dsconversation 抽出来 |

**修正一条之前的判断**：我最初把 A 类说成"界面自己能做、不依赖宿主"——**`layout` 的持久化就依赖宿主**（`config.get/set` 那三层契约是我这轮补的）。
所以 A 类准确的说法是「**不依赖宿主新增业务 API**」，而不是"完全不碰契约"。

## 这 4 个做完之后的共同点

**三个都卡在同一类地方**：不是"能不能画出来"，而是**"画出来了但没人看见"**——
- `shortcuts`：状态切了、没重绘
- `commands`：状态切了、没重绘（同一原因）
- `message-feedback`：按钮画了，但"记录"这个动作没有真实通道（差点就要写假的"已上报"）

**"能画"和"能看见/能生效"是两个问题**，后者更难，也更少人验。

## B 类的确切阻塞点（逐个查过，不是猜）

我本想先做 `sidebar-files`，查下来卡在**宿主没给工作区根**。逐项实测（真宿主 `curl`）：

| 宿主方法 | 实测结果 | 能否支撑界面 |
|---|---|---|
| `workspace.list` | `{"entries":[{"name":".git","isDir":true,"size":0}, ...]}` | **只给 name，没有 path、也没有"列子目录"的入参** |
| `fs.read` | `{"path":"D:/.../go.mod","content":...}` 通了（**必须 Windows 路径**，MSYS 的 `/d/...` 报"文件不存在"） | 能读，但**要先知道完整路径** |
| `session.load` | `keys: [messages, total, truncated, permission, usageTotal, contextTokens, journalEpoch, lastSeq]` | **没有 `cwd`** |

**所以 `sidebar-files` 走不通的链条是**：

```
workspace.list 给 name（无 path）
  ↓
要读文件得拼「工作区根 + name」
  ↓
界面不知道工作区根（session.load 没有 cwd）
  ↓
fs.read 无法调用
```

**要解开只需宿主给一样东西**（二选一）：
1. `workspace.list` 每项补 `path`（或加 `list(path)` 入参）
2. `session.load` 回 `cwd`

**这不是"界面没做"，是"做不了"**。我把它写死在这里，免得下一轮又有人从头查一遍。

**B 类其余几个的阻塞**（同样逐项查过）：

| 部件 | 阻塞 |
|---|---|
| `deliverables` | 没有"哪些文件算交付物"的约定——`fs.write` 只知道写了什么，不知道哪个是交付品 |
| `reference` | 补全已做（`workspace.list` 够用）；**引用块**缺的是"附件/引用"这一层契约——没有身份、没有独立字段，`@path` 只能是一段文本 |
| `jobs` | ✅ 已解（2026-11-14）：`jobs.list` / `jobs.output` / `jobs.stop` 都已在生产接线并有调用者，见 #22 |
| `schedule` | 完全没有定时任务的存储与方法 |
| `sidebar-documentpreview` | 只差上面那个"工作区根"（有根就能读、能预览） |

**结论**：B 类 6 个**全部**卡在宿主侧。界面这边能做的只有"等"——所以我不会假装在推进它们。

---

## 2026-11-04 账本审计：九处从 ❌ 改掉

起因是补 `@` 补全时发现清单上 `secret-mask` 还写着 ❌——**而它两个提交前就接好线了**。
顺手把剩下的 ❌ 逐条对着代码核了一遍，九处不对。

| 部件 | 原来 | 现在 | 依据 |
|---|---|---|---|
| `secret-mask` | ❌ | 🟡 | `internal/secretmask` + `dsconversation/mask.go`：用户消息**显示**遮罩（只用户消息、只显示） |
| `message-feedback` | ❌ | 🟡 | `dsconversation/feedback.go`：每条助手消息 👍/👎，落盘 `feedback.<会话>.<下标>`；**没有反馈弹窗** |
| `open-in-app` | ❌ | 🟡 | `plugins/dsopen/open.go`：会话头部「打开」→ `OpenExternal(CWD)`；**文档预览那半没做** |
| `brand-official` | ❌ | 🟡 | `plugins/dsbrand/brand.go`：官方 logo 矢量 + `ui.brand` 可替换；**sidebar slots occupants 没做** |
| `input-trigger` | ❌ | 🟡 | `plugins/dscomposer/mention.go`：`@` 候选菜单；`/` 走 F2。**不是通用候选弹层管道** |
| `sidebar-right` | ❌ | ✅ | `SlotRightbar` + `ToggleRightbar` + `ui.rightbarWidth` + 收起后 corner 重开 |
| `session` | ❌ | ⬜ | 官方原文是 *"Session Controller adapter **for React**"*——React 适配层，我们没有也不需要 |
| `renderer` | ❌ | ⬜ | 官方原文是 *"Browser UI renderer: **React** slot bindings"*——同上 |
| `deepseek-model-label-1017` | ❌ | ⬜ | 官方原文是 *"**Temporary** 1017 release badge"*——一次发布的临时角标，不是产品能力 |

**为什么能错这么久**：清单是手写的，而每补完一个部件时，
"把清单那一行改掉"从来不是任务的一部分——它总被当成收尾的杂事。
于是账本只会往"比实际更差"的方向漂，而且漂了没人发现（因为没人会去"测试"文档）。

**现在怎么防**：`python scripts/parity-count.py --check`（CI 里跑）。
合计那一行是**数出来的**，条目和合计对不上就红。守卫本身做过反证：
把 `attachment` 临时改成 ✅ → `FAIL 合计行和条目对不上`，改回去 → `PASS`。

⚠️ 这一条只防"合计与条目不一致"，**防不了"条目写错"**——
上面那九处是人对代码核出来的，脚本做不到。所以状态行仍然要靠人诚实。
