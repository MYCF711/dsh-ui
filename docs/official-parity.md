# 官方部件清单 vs 我们的

> 来源：`tmp/official/` 下从官方 asar 提取的 `@deepseek-ai/dsh-client-ui-*` 包（59 个）。
> 我们有 **22 个插件目录**，对上官方 55 个部件里的 20 个（+ 7 个只做了一半）。
> **"几何对齐 ≠ 像官方"**，差距在功能面。

## 官方 59 个部件（按用途分组）

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
- ❌ `sidebar-browser`
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
- ❌ `entry-customization`
- ⬜ `deepseek-model-label-1017`（**N/A**：官方是 1017 那次发布的临时角标，不是产品能力）

### 插件体系（3）

- ✅ `plugin-manager`
- ❌ `plugin-market`
- ❌ `settings-plugin-inventory`

### 会话能力（13）

- ✅ `agent-preset`（`dssettings` 的 `agent-presets` 页：真名册卡片 + 「设为新任务默认」写 `agent-preset-registry.selectedDefault`；证据见 [official-agent-preset-evidence-rc2.md](official-agent-preset-evidence-rc2.md)）
- ✅ `approval`（审批条 + 三按钮；允许/拒绝走真实 `session.approve`）
- ✅ `chat`（`dsconversation` 的消息区；不等于官方的细分能力）
- ✅ `conversation`
- ✅ `goal`（`dsgoal`：GoalBar 停靠 composer 上方，数据来自宿主 `session.setGoal`）
- ❌ `plan`
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
| 13 | `dsh-client-ui-creation-recommendations` | ❌ MISSING | 无 |
| 14 | `dsh-client-ui-deliverables` | ✅ IMPLEMENTED | `plugins/dsdeliverables`（写过的文件 + **按文件 before/after diff**）+ `dsconversation/file_refs.go` + **`internal/apiclient/changes.go`（`/api/changes.summary\|diff\|open` 裸路由）**；证据 [official-deliverables-changes-evidence-rc2.md](official-deliverables-changes-evidence-rc2.md) |
| 15 | `dsh-client-ui-directory-picker-browse` | ✅ IMPLEMENTED | `plugins/dsdirpicker` + `internal/apiclient/dirpicker.go` |
| 16 | `dsh-client-ui-directory-picker-native` | ❌ MISSING | 无 |
| 17 | `dsh-client-ui-entry-customization` | ❌ MISSING | 无 |
| 18 | `dsh-client-ui-file-actions` | ❌ MISSING | 无 |
| 19 | `dsh-client-ui-file-drag` | ❌ MISSING | 无 |
| 20 | `dsh-client-ui-goal` | ✅ IMPLEMENTED | `plugins/dsgoal`（4 文件 / 364 行） |
| 21 | `dsh-client-ui-input-trigger` | ✅ IMPLEMENTED | `plugins/dscomposer/trigger.go` + `internal/apiclient/references.go`（`@` 文件/会话 + `/` 技能；detect/分组/Tab drill/pick 路由） |
| 22 | `dsh-client-ui-jobs` | ✅ IMPLEMENTED | `plugins/dsjobs`（官方 `conversation.session.header.actions` order 20 触发器 + `shell.overlay` 弹层；旧三栏 `sidebar.footer` 清单）+ `internal/apiclient/jobs.go`（`jobs.output`/`jobs.stop`）+ `cmd/app/jobs_actions.go` |
| 23 | `dsh-client-ui-layout` | ✅ IMPLEMENTED | `internal/uikernel` 骨架 + `plugins/dsshell` |
| 24 | `dsh-client-ui-message-feedback` | ✅ IMPLEMENTED | `plugins/dsconversation/feedback.go` + `internal/apiclient/message_feedback.go` + `cmd/app/reference_wiring.go`（durable + 弹窗 + 冲突收敛 + toast） |
| 25 | `dsh-client-ui-model-selection` | ✅ IMPLEMENTED | `internal/uikernel/official_model_menu.go` + `dscomposer` |
| 26 | `dsh-client-ui-open-in-app` | ✅ IMPLEMENTED | `plugins/dsopen`（3 文件 / 163 行） |
| 27 | `dsh-client-ui-permission-presets` | ✅ IMPLEMENTED | `dscomposer` + `dssettings` |
| 28 | `dsh-client-ui-plan` | ❌ MISSING | 无 |
| 29 | `dsh-client-ui-plugin-manager` | ✅ IMPLEMENTED | `plugins/dssidebar/plugins_page.go`（侧栏「插件」管理页：列表/详情/启停/卸载）+ `internal/apiclient/plugin_manager.go` + `cmd/app/plugin_manager.go`；证据 [official-plugin-manager-evidence-rc2.md](official-plugin-manager-evidence-rc2.md) |
| 30 | `dsh-client-ui-plugin-market` | ❌ MISSING | 无 |
| 31 | `dsh-client-ui-plugin-recommendations` | ❌ MISSING | 无 |
| 32 | `dsh-client-ui-reference` | ✅ IMPLEMENTED | `plugins/dsconversation/usertext.go`（file/folder/session chip + PreviewFile）+ `plugins/dscomposer/trigger.go` + `internal/reficon` |
| 33 | `dsh-client-ui-renderer` | ✅ IMPLEMENTED | `internal/uikernel`（槽 dispatch） |
| 34 | `dsh-client-ui-schedule` | ✅ IMPLEMENTED | `plugins/dssidebar/schedule_page.go`（任务管理页）+ `internal/apiclient/schedule.go`（`schedule.catalog/edit/run/history`）+ `cmd/app/schedule_catalog.go`；侧栏摘要仍在 `plugins/dsschedule` |
| 35 | `dsh-client-ui-secret-mask` | ✅ IMPLEMENTED | `internal/secretmask`（官方格式表）+ `plugins/dsconversation/usertext.go`（逐值显示/隐藏） |
| 36 | `dsh-client-ui-session` | ✅ IMPLEMENTED | `internal/apiclient` + `cmd/app` |
| 37 | `dsh-client-ui-settings` | ✅ IMPLEMENTED | `plugins/dssettings`（14 文件 / 5470 行） |
| 38 | `dsh-client-ui-settings-account` | 🟡 PARTIAL | `dssettings` 的 `account` section |
| 39 | `dsh-client-ui-settings-agent-loop` | ❌ MISSING | 无 |
| 40 | `dsh-client-ui-settings-general` | ✅ IMPLEMENTED | `dssettings` 的 general 页 |
| 41 | `dsh-client-ui-settings-models` | 🟡 PARTIAL | `dssettings` 的 models 页 |
| 42 | `dsh-client-ui-settings-plugin-inventory` | ✅ IMPLEMENTED | `plugins/dssettings/official_plugins.go` + `official_plugins_draw.go` + `internal/apiclient/plugin_inventory.go` |
| 43 | `dsh-client-ui-settings-plugins` | ✅ IMPLEMENTED | `plugins/dssettings/official_tree.go:83` + `official_plugins*.go`（外壳 + tab 台账） |
| 44 | `dsh-client-ui-settings-session-log` | 🟡 PARTIAL | `dssettings` 的 general item `session-log-upload-toast` |
| 45 | `dsh-client-ui-settings-shell` | ❌ MISSING | 无 |
| 46 | `dsh-client-ui-settings-subagent` | ❌ MISSING | 无 |
| 47 | `dsh-client-ui-settings-web-search` | ❌ MISSING | 无 |
| 48 | `dsh-client-ui-shortcuts` | ✅ IMPLEMENTED | `plugins/dsshortcuts`（2 文件 / 213 行） |
| 49 | `dsh-client-ui-sidebar` | ✅ IMPLEMENTED | `plugins/dssidebar`（5 文件 / 1418 行） |
| 50 | `dsh-client-ui-sidebar-browser` | ❌ MISSING | 无（HIDE：官方是右栏沙箱 HTTP(S) 浏览器 tab。**tab 注册表已建**（2026-11-15，[official-view-tabs-evidence-rc2.md](official-view-tabs-evidence-rc2.md)），但本仓**没有 iframe/Electron webview 这一层**——Go 即时模式界面里嵌不了网页；此前的 `dssidebar` 搜索框是误映射） |
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
| 63 | `dsh-experimental-client-ui-cot-translation` | ❌ MISSING | 无 |
| 64 | `dsh-experimental-client-ui-forge` | ❌ MISSING | 无 |
| 65 | `dsh-experimental-client-ui-stats` | ✅ IMPLEMENTED | **`plugins/dsstatusbar/stats_view.go`（官方 `conversation.view` `order:40` 的 `stats` 页面本体：逐 turn/step 11 列 + 总用时条 + 全量/窗口合计；帧投影对齐官方 `stats-fold.ts` / `stats-usage.ts` / `stats-format.ts`）** + `plugins/dsstatusbar/statusbar.go:129-130` 注册；证据 [official-attachment-stats-evidence-rc2.md](official-attachment-stats-evidence-rc2.md)。**剩余差异**：compaction/非 step 事件行、三个工具栏开关、加载更早分页未做 |
| 66 | `dsh-experimental-client-ui-voice-input` | ❌ MISSING | 无 |

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

## 合计：官方 66 个部件 —— ✅ 44 · 🟡 5 · ⬜ 0（不该有） · ❌ 17


> 这五个数是**从下面那张表里数出来的**（脚本按 `^- [状态] \`名字\`` 去重统计），
> 不是手填的。上一次手填的是"我们有 10 个 · 缺 53 个"——**已经错了很久**，
> 因为每次补完一个部件都没人回头改这个数。所以这次把它写成"数出来的"。

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

**这解释了"为什么差得很远"的一半**：59 个官方部件里，相当一部分依赖宿主提供的能力面，界面单方面补不出来。

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
