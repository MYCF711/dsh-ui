# 官方部件清单 vs 我们的

> 来源：官方 asar 提取的 `@deepseek-ai/dsh-client-ui-*` 包（**62 个**）+ `dsh-experimental-client-ui-*`（**4 个**）= **66 个**（rc.2 口径，见 §权威清单）。
> 我们有 **36 个插件目录**（判据：`plugins/*/register.go`，由 `cmd/app/plugins_assembled_test.go` 的 `TestEveryPluginDirIsImported` 双向守着）；账本对官方 **66 个部件**逐 id 判状态（分母由 `scripts/parity-count.py --check` 动态读官方清单并断言零缺零多）。
> **"几何对齐 ≠ 像官方"**，差距在功能面。

## 官方 66 个部件（按用途分组）

### 设置类（11）

- 🟡 `settings`（`dssettings` 浮层：F2 命令面板末行「设置」/ Shift+F2 直达，Esc 关闭。**不是官方的 11 页树**——它只把壳真接上了的动作摊成四块：外观 / 模型 / 权限 / 会话。缺一个 Action 就整块不画，当前值不知道就一行都不标选中）
- 🟡 `settings-account`（`dssettings/official_account.go` + `official_account_avatar.go`：真 profile/contact、充值/赠金余额（官方精度 + 「前往开放平台查看」下划线链接）、官方三态头像图片加载、登录/取消/登出、usage/top-up；缺口是 `account.watch` 流、两类 onboarding、`shell.quota-notice`、Platform 内嵌页、contactUs，见 #38）
- ❌ `settings-agent-loop`
- 🟡 `settings-general`（只有**主题**：深 / 浅 / 跟随系统，走 `Actions.SetTheme`，选完当场生效。语言、启动行为、快捷键这些通用项没有入口）
- 🟡 `settings-models`（`plugins/dssettings/official_models*.go`：provider 目录真实 join + 模型目录分组 + `model.defaultModel` 写 + retryPolicy 卡内草稿；缺 API key/baseURL/displayName+api/模型目录编辑器/EditorFooter 三态/发现测试，见 #41）
- ❌ `settings-plugin-inventory`
- ❌ `settings-plugins`
- ✅ `settings-session-log`（2026-11-16 补齐 general Switch + shell.overlay 结果 Toast；宿主上传方缺失仍如实返回失败，不伪报上传成功）
- ❌ `settings-shell`
- ❌ `settings-subagent`
- ❌ `settings-web-search`

### 侧栏类（6）

- ✅ `sidebar`（`dssidebar`：工作区分组 + 搜索 + 状态点 + **两次点击删除会话**——宿主早就有 `session.delete`，界面一直没接；现在三条都接了：`session.delete`（侧栏两次点击）/ `session.compact`（F2「压缩上下文」）/ `session.fork`（F2「分支这个会话」，**整份复制**））
- 🟡 `sidebar-browser`（`plugins/dsbrowser`：多实例 occurrence、独立导航/WebView、关闭与两层恢复已接；重启显示「上次打开」恢复卡，不自动开页。2026-11-17 补齐 focus-within 前往键 / 「URL 已变化」角标 / unknown 配色 / hover 变色 / 错误行底色与行高 / border-box 高 38 / unknown 禁外部打开；`window.open` 带 URL 的应用内新标签已随壳契约补齐（`RightTabParams`/`OnNewWindow`，见 #50 行）；剩下的页面生命周期、下载与存储分区见 #50 行）
- ✅ `sidebar-documentpreview`（`dsfiles` 的预览浮层；16KB 上限、20 行截断）
- ✅ `sidebar-files`（`dsfiles`；`workspace.list(path)` + `fs.read`，可进子目录）
- ✅ `sidebar-right`（`dsshell` 的 `SlotRightbar` + `ToggleRightbar` + `ui.rightbarWidth`；收起后 corner 可重开）
- 🟡 `sidebar-terminal`（`dsterminal`：F5 面板 + F6 发命令，宿主 `terminal.*` + `internal/ansi`；结论与 #54 行同一口径，见该行）

### composer 控件（7）

- ✅ `model-selection`（F2 命令面板；数据来自宿主 `models.list` → 网关 `/v1/models`）
- ✅ `permission-presets`（F2 命令面板 5 条；宿主 `session.setPermission` + 白名单）
- ❌ `attachment`
- ✅ `commands`（F2 命令面板；A 类时做的）
- ✅ `input-trigger`（`plugins/dscomposer/trigger.go`：`@` 文件/会话 + `/` 技能三源注册表、官方 detect（引号路径 + URL 斜杠 carve-out）、按 source 分组菜单、Enter/Tab/↑↓/Esc；数据面见 `internal/apiclient/references.go`）
- 🟡 `entry-customization`（`plugins/dsentrycustom`：左栏 More 的未固定条目行（行首面板图标 + 行尾「固定」按钮）+ 会话头 More 的折起 view 行（active 带对勾）+ 「浏览更多插件」（`plugins/dssidebar/panelapi` = 官方 `pluginNavigation` 等价物，服务在册才画）。「创建你的插件」已接 cordis preset + 区域 prompt + `whenDraftExists:'append'`（真实建会话、只灌草稿不发送）；右栏 guide 无宿主，按 HIDE 登记）
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
- 🟡 `cordis`（`plugins/dscordis` 面板 + `internal/apiclient/cordis.go` + `cmd/app/cordis_wiring.go`；**`tool.view.cordis` 业务视图与 4 张历史工具卡 HIDE**，逐条见官方包对账表 #12）
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
| 12 | `dsh-client-ui-cordis` | 🟡 PARTIAL | **2026-11-17 本轮：跨会话水位 bug 已修**：原先 `cmd/app/cordis_wiring.go` 的 `eventSeq` 是全局单值，而 seq 由宿主在**每会话各自的 journal** 追加时分配（`DSH-GO/docs/transport-contract.md` §3）；A 会话的高 seq 会把 B 会话的低 seq 新事件判成「已消费」，表现为「B 里 cordis 激活了、面板不刷新」，不报错不丢帧。现改为 `journalWatermark`（`cmd/app/cordis_wiring.go:66-140`），键是 `(sessionID, journalEpoch)`：跨会话不互相压制，宿主重启换 journal 世代后 seq 从 1 重来也不被旧水位压住；空 sessionID **整段跳过**（没有可信序号域就不记水位）。调用点 `cmd/app/main.go:2046-2049` 传 `cur, framesEpoch, frames`；回归用例 `TestCordisWatermarkIsPerSession`/`IsPerEpoch`/`SkipsUnknownSession`，已用「退回全局单值」变异体验过**必红**（两条同时红）。<br>**仍保持 🟡 的四条剩余差异（都卡在宿主形态，不是没做完）**：① **`tool.view.cordis` 业务视图**——官方是动态 `client.js` 里的 **React 组件**（`slots.ts:23-37`、`CordisRunRow.tsx:119-130`），Go+Viva 没有 JS 组件宿主，加载不了插件包里的 JS，画出来是空框；**能补的条件**：出现能在页面里执行插件 `client.js` 的 JS 宿主（等于引入 JS 运行时，与 R6 冲突）。② **4 张 `cordis_*` 历史工具卡**——唯一数据源是已记录的 tool/call+tool/result 帧（`card-model.ts:95-160`）；本宿主注册 0 个 cordis 工具，且**官方 rc.2 自己也不注册**（`dsh-cordis-host-runner/README.md:119`「registers no tool or prompt」）⇒ **上游就没有生产者**；**能补的条件**：官方给这三个动词补模型工具。③ **Cordis 审批**（approve/decline + `awaiting-approval` + `runHostHalf` 后两个参数）——宿主无 Cordis 审批队列，`cordis/request-run`/`request-run-resolved` 两条帧**宿主不发**；状态派生**已按官方写成**（`plugins/dscordis/status.go:166-173` 先判 `awaiting-approval`），只是宿主永不产生它；**能补的条件**：宿主出现审批队列并真发那两条帧。④ **Client 半的加载与结算**（`getClientCode`+`settleUserRun`）——没有 JS 宿主就没有「页面加载回执」，调 settle 等于编造成功；连带运行按钮只对 `hasClientHalf==false` 的包画（`status.ts:40-44` 的 `loaded` 集合本仓恒为空，这是官方语义的直接结果）。**能补的条件**：同①。<br>**如实缺席的字段**（界面必须处理空值，不补默认值）：`agentId` 可能空串、`purpose` 恒空、`packages` 恒 1 项（故 `CordisPanel.tsx:267` 版本选择器不出现）、`nextPackageId` 不出现、`waitingFor` 恒 `[]`、`currentPackageId` 在 `client-pending` 期间不推进。**另一条已知窗口**：事件只进**归属会话** journal，壳只订阅当前会话，别的会话的激活要等切过去才看到；缓解是每次开面板与每个生命周期动作后主动重读（`Actions.CordisRefresh`），不是轮询。**为什么仍不是 ✅**：缺的那一半由宿主形态决定（①③④），②连上游都没有。证据 [official-cordis-evidence-rc2.md](official-cordis-evidence-rc2.md)。<br>**原实现记录**：**面板已落地（2026-11-15）**：`plugins/dscordis`（`sidebar.footer.action` 触发器 Order 60 + `shell.overlay` 面板 Order 46，420px / `max-height:60vh` / header 44px / 行 `padding:14px 12px 10px` / 动作按钮 28×28，几何逐条取 `CordisPanel.module.css`）+ `internal/apiclient/cordis.go`（4 方法：`inventory` / `stopFromPanel` / `undefineFromPanel` / `runHostHalf`，官方 wire 类型投影）+ `cmd/app/cordis_wiring.go`（全局清单缓存 + **2 条真事件帧** `cordis/dynamic-package` / `cordis/dynamic-retract` 的 seq 水位 + 三个动词的 goroutine 落地）。**做了**：状态派生逐行照 `status.ts:31-45` 的 `cordisVisibleStatus` 与 `CordisPanel.tsx:62-75` 的 `visiblePanelStatus`（状态点映射同 `:77-86`）；分组照官方 `agentId === 当前会话`（本宿主 `agentId` 可能是空串 → 落「其他会话」，**不猜归属**）；两条事件帧走**真 WS**：宿主 `emitFrameAt` 写归属会话 journal → 壳扫 `Snapshot.Frames` 比 seq 水位 → 触发一次后台 `inventory` 重读（官方 `$on(...) → inventory.refresh()` 的等价物，`index.ts:75-80`）；同一帧重放/补页**不重复读**；读失败**保留旧行** + 记宿主原文（官方 `inventory.ts:88-99` 同一取舍）；`stopFromPanel` 的 `not-running` 按官方 `index.ts:49` 折算成成功、`plugin-missing` 才报错；`undefineFromPanel` 成功后那一帧行立刻消失（官方 `inventory.retire()`，`index.ts:106-110`）+ removed 粘性 + 重新出现时摘除；`runHostHalf` 只发前 4 个参数（`requestId`/`approveFutureVersions` 属模型侧审批路径，本宿主无审批队列，官方面板路径同样传 null/false），`mode` 按官方 `CordisPanel.tsx:248-249` 算。**HIDE 六项（逐条登记，不是"以后再说"）**：① **`tool.view.cordis` 业务视图**——官方是动态 `client.js` 的 **React 组件**（`slots.ts:23-37`、`CordisRunRow.tsx:119-130`），Go+Viva **没有 JS 组件宿主**，不执行插件包里的 JS，画出来是空框；② **4 张 `cordis_*` 历史工具卡**（`cordis_define`/`cordis_run`/`cordis_stop`/`cordis_undefine`）——唯一数据源是**已记录的** tool/call+tool/result 帧（`card-model.ts:95-160`），而本宿主注册 **0 个 cordis 工具**（`DSH-GO/docs/host-stable-gap-register.md:412`）、官方 rc.2 自己也不注册（`dsh-cordis-host-runner/README.md:119`「registers no tool or prompt」）⇒ 没有生产者，座位永远空；③ **Client 半的加载与结算**（`getClientCode` + `settleUserRun`）——没有 JS 宿主就没有"页面加载回执"，调用 settle 就是编造成功；④ **审批**（approve/decline 两按钮 + `awaiting-approval` 态）——宿主无 Cordis 审批队列，`cordis/request-run` 不发、`approvalRequestId`/`requiresApproval` 不出现；⑤ **"运行"按钮对"有 Client 半的包"**——官方 `onRun` 会连带加载 Client 半并结算，本仓只起 Host 半、起了永久停在 `client-pending`，所以只对 `hasClientHalf==false` 的包画运行按钮；⑥ **渲染失败那一行**（`.rowError` 的 render 分支）——本页不加载动态组件，没有失败可报（`reportRenderFailure` 也不调）。**如实缺席的字段**（界面必须能处理空值，不补默认值）：`agentId` 可能空串、`purpose` 恒空、`packages` 恒 1 项、`nextPackageId` 不出现、`waitingFor` 恒 `[]`、`currentPackageId` 在 `client-pending` 期间不推进、`cordis/request-run`/`request-run-resolved` **宿主不发**。**为什么不是 ✅**：官方这条包的另一半（工具卡 + Package 业务视图）**没有 JS 组件宿主就跑不起来**——`tool.view.cordis` 整块 HIDE、4 张历史卡没有生产者。缺的那一半不是"没做完"，是宿主形态决定的。证据 [official-cordis-evidence-rc2.md](official-cordis-evidence-rc2.md)；快照新增 4 方法（98 → 102） |
| 13 | `dsh-client-ui-creation-recommendations` | ✅ IMPLEMENTED | **`plugins/dscreationreco`（`create_personalized_page` 卡片正文：header + info 说明块 + summary/预填内容折叠 + 位置四项菜单 + 「自行撰写」textarea + 四个动词 + **11 档 phase** + 8 条 reason 文案）+ 复用 `pluginapi/action_card_bodies.go`（正文注册表）与 `plugins/dsactioncards`（按 kind 派发）**；**零新增宿主方法**（官方 `inject` 里只有 locale/actionCards，卡片数据全在 props 上；`testdata/api.snapshot.json` 不变）；证据 [official-creation-recommendations-evidence-rc2.md](official-creation-recommendations-evidence-rc2.md)。**`refused{creator-unavailable}` 逐字走官方兜底档 `unavailable`**（"暂时无法执行，请到插件页面查看"——官方 REASON_KEYS 只有 8 条，其余 reason 一律读作 unavailable，见证据文档 §4）。**剩余差异（4 处，均登记在证据文档 §7）**：flex-wrap 无对等物 / `textarea maxLength=1000` 无对等物 / 菜单无"当前项高亮" / ARIA 角色无对等物。**info 图标已是官方 `IconWarningTriangleOutlineRegular` 原文**（rc.2 sprite `warning-triangle-outline`，viewBox=16，圆弧按标准 arc→cubic 换算，见 `plugins/dscreationreco/icons.go`）；几何/字号按官方 CSS 逐条对齐（info 按钮 28×28、小按钮 12、位置按钮 13、说明块底 `bg-module-platform`） |
| 14 | `dsh-client-ui-deliverables` | ✅ IMPLEMENTED | `plugins/dsdeliverables`（写过的文件 + **按文件 before/after diff**）+ `dsconversation/file_refs.go` + **`internal/apiclient/changes.go`（`/api/changes.summary\|diff\|open` 裸路由）**；证据 [official-deliverables-changes-evidence-rc2.md](official-deliverables-changes-evidence-rc2.md) |
| 15 | `dsh-client-ui-directory-picker-browse` | ✅ IMPLEMENTED | `plugins/dsdirpicker` + `internal/apiclient/dirpicker.go` |
| 16 | `dsh-client-ui-directory-picker-native` | 🟡 PARTIAL | **seam / 首屏 directoryFlow / armed / onPick(workspaceId) 已接；剩宿主创建边界与人工验收**。`internal/apiclient/dirpicker.go`：`ProbeDirectoryPicker` 按宿主 seam 判据二选一（打**只读**的 `directoryPicker.list`——200 ⇒ browse；501 + `error.code=not_implemented` + message 前缀 `directory-picker/unavailable:` + 信封顶层 `details.capability=native` ⇒ native；其它一切 ⇒ 失败。**绝不拿 `pick` 当探针**，那会真弹原生框）+ `DirectoryPick`（`directoryPicker.pick`，官方结果 `string|null`：null = 操作员取消、不是错误；这条请求走**无整体超时**的客户端副本，原生框是人工交互，8s 传输超时会把它打断）+ `CallStatus`（`Call` 多交一个 HTTP 状态，组合判据里的 501 是协议事实而不是该拿去猜的文本）。`cmd/app/dirpicker_wiring.go`：启动时**探一次并缓存**，三种结局——browse 走 `plugins/dsdirpicker` 原路径不变；native 点「添加工作区」直接弹原生框、选到路径后 `workspace.create` 采用它（取消静默、失败把宿主原文交状态栏）；**一套都不可用 ⇒ `Actions.OpenDirectoryPicker = nil` ⇒ 侧栏整块隐藏入口**（不静默退化成某一套）。宿主 seam：MYCF711/dsh-go#47（`dshd serve --directory-picker=native|browse` / `DSH_DIRECTORY_PICKER`，未声明取官方兜底 browse）。证据：`internal/apiclient/dirpicker_test.go`、`cmd/app/dirpicker_wiring_test.go`；快照新增 `directoryPicker.pick`（83 → 84）。**2026-11-16 本轮补 GUI 三项**：① `plugins/dsconversation/hero.go` 落地首屏 WorkspaceChip/WorkspacePicker（pending → owner → cwd 桥接 → placeholder，ready 空 registry 直接升起目录流，已有工作区单选 + 添加项，busy 禁用）；② native 与 browse 都有 open 防重入，native 三种结局后 re-arm；③ `WorkspaceCreate` 解码 `{workspace,created}`，两种后端统一把返回的 WorkspaceView 交给 `adoptPickedWorkspace`，再按 `onPick(workspaceId)` 复用经 `session.load` 证实 `Total==0`、cwd 匹配且未归档的空白会话。**2026-11-17 本轮补上官方 `workspace.add` 快捷键（`Ctrl+O`）**：官方在 `ui-workspace/src/client/shortcuts.ts:85-89` 注册 `register('workspace.add', …, 'KeyO', ['primary'], ['primary','alt'], …)`，`resolve()` 返回 `{status:'handled', run}` 或 `{status:'blocked', reason}`，reason 逐字来自 `locales.ts:16-17`：`shortcut.noPicker`「目录选择器不可用」/ `shortcut.directoryBusy`「正在选择或添加工作区」，判据是 `:68-70` 的 `addReason()`（先判 noPicker 再判 busy）。本仓原先**没有这个键位**，根因是只接了 viva 的 `OnKey`——那层只送具名键，字母是 `KeyUnknown`+`Text`（`viva@gfx/input.go:141-144`），`HandleKey` 里根本拿不到 `Ctrl+O`。本轮接 `app.Config.OnShortcut`（`viva@app/app.go:130-142`，在原生文本处理**之前**收带修饰键的可打印键）→ `cmd/app/main.go:2207-2218` → `cmd/app/dirpicker_wiring.go` 的 `handleDirectoryAddShortcut`；禁用理由由壳算好交给界面（`Snapshot.DirectoryPickerAddBlocked`，`directoryAddBlockedReason` 逐行照 `addReason()`，`pluginapi/plugin.go:641-664`），`plugins/dsshortcuts` 在 F1 表列出 `Ctrl+O` 并在被挡住时显示那句原文。用例 `TestDirectoryAddBlockedReasonMatchesOfficial`/`TestDirectoryAddShortcutRecognisesChord`/`…BlockedWhenNoPicker`/`…BlockedWhenBusy`（`cmd/app/dirpicker_wiring_test.go`）钉住字面量、顺序与「不该吃的 chord 一律放行」。<br>**仍保持 🟡 的剩余差异（D）**：宿主 `session.create` 只解 `mode`（`DSH-GO/internal/transport/server.go:1211-1227`；本仓快照同文），空工作区没有可复用空白会话时无法在该 workspace 新建，GUI 明确报错而不在进程 cwd 假建；需宿主支持官方 `{workspaceId,sessionId?}` 创建/复用语义后才能升 ✅。ENV_BOUNDARY 仅是本轮不弹窗的无人值守限制，真机能力仍是 `IFileOpenDialog`；仍需有交互桌面者人工执行 `DSH_NATIVE_PICKER=1 go test ./internal/native -run TestRealPickerProbe -v`，不能用绕过或假探针替代。 |
| 17 | `dsh-client-ui-entry-customization` | 🟡 PARTIAL | **2026-11-17 本轮：左栏行尾「悬停三态」已补（绘制级，值全取官方 CSS）**：官方 `@media (hover:hover)` 那块（`OFF:EntryMenu.module.css:20-32`）是**三条反向规则**，逐条落地——静止 = 固定按钮 `opacity:0`（:26）+ 对勾 `opacity:1`（:27，覆盖 :12 的 0）；行悬停 = 固定按钮 `opacity:1`（:28）+ 对勾 `opacity:0`（:30）；对勾 `position:absolute;right:9px;top:50%;translateY(-50%)`（:7-14）+ 14×14（`._itemIcon svg,._check_70kcw_265{14px}` 夹 16→14）；`.pinButton:hover:not(:disabled){color:label-primary;background:0 0}`（:22-25）；祖先级 `.sidebarEntry:hover>div:first-child>button{background:interactive-bg-hover}`（:21，指针落在固定按钮上时整行底色也亮）；**两层文案**也都画了——可见浮条 `Tooltip label={t('pinTooltip')}`（「固定到左侧栏」，`:54`）+ 无障碍名 `aria-label={t('pin',{name})}`（「将{name}固定到侧边栏」，`:55`）。判据：CSS 的 `:hover` 沿祖先链传播，而 viva `Canvas.Hovered()` 只给**最上面一个**命中身份（v0.6.4 `gfx/canvas.go:1962`），故比「行 **或** 行尾按钮」；`pin-outline` 尾 path 是 sprite 的 `fill="currentColor"`，描边与填充必须同一个颜色变量（旧实现把填充层钉死 tertiary，悬停后只有描边变亮）。落点 `plugins/dsentrycustom/entry_menu.go`（`drawSidebarActiveMark`/`drawInlineActiveMark`/`drawPinButton`/`sidebarEntryHovered`），证据 [official-entry-menu-evidence-rc2.md](official-entry-menu-evidence-rc2.md) §4.7，断言 `TestSidebarHoverSwapsPinAndActiveMark`（三态对翻）/`TestPinButtonHoverChangesIconColor`（换色+按钮不铺底色+整行底色)/`TestSidebarActiveMarkGeometry`（right 9/竖直居中）/`TestPinButtonKeepsOfficialTwoLineCopy`（浮条+无障碍名）/`TestConversationActiveMarkIgnoresHover`（会话头恒可见）。<br>**仍是 🟡：三条真差异**：①「创建你的插件」**行在、效果不完整**——官方 `startSession(undefined,{agentPreset:'cordis',prompt,whenDraftExists:'append'})`（`OFF:src/client/index.ts:47-52`）本仓三字段都发（区域 prompt 逐字 `OFF:locales.ts:18-19`，`plugins/dsentrycustom` 的 `drawCreateRow`），但宿主只把 `mode` 写进会话头 `header.agentPreset`、**没有 cordis preset 组合与 Creator 运行时/工具链**（`DSH-GO/cmd/dshd/serve_create.go:52-65`、`DSH-GO/internal/creation/service.go:39-44`），点到手是 default 会话；补它只能改宿主（DSH-GO），本仓到此为止。②右栏 guide 的 More 卡片不注册：`sidebar.right.tab.guide.actions` 无派发方（guide 面未落地，账本 #53）。③键盘模态那半边不画（`:29,:31` 的 `html[data-input-modality='keyboard'] … :focus-within`）：本仓无「全局输入模态」位，且 `:focus-within`（行内**任一后代**有焦点）与 viva 的 `FocusVisible()==id`（单个身份）不是同一判据；补的前提是 viva 给出行内焦点或输入模态读口。<br>**已落地的本体（沿革）**：`plugins/dsentrycustom`（本仓可派发的两处菜单行：左栏 `sidebar.panellist.actions`、会话头 `conversation.session.header.views.actions`）+ `plugins/dssidebar/panelapi`（官方 `pluginNavigation` 的等价物）+ 宿主补的两个 owner props（`pluginapi.EntryMenuEntry.Glyph` = `sidebar.panel.icon` 工厂槽投影；会话头 entries 补 `active`）；行几何逐条取 rc.2 产物（`_item_70kcw_173` min-height 34/gap 6/padding 6,8/radius md、`_itemIcon_70kcw_233` 14×14、`_separator_70kcw_157` .5px+margin 3 8、`.pinAnchor{right:6}` 22×22、`.entryName{padding-right:28}`）。 |
| 18 | `dsh-client-ui-file-actions` | 🟡 PARTIAL | **2026-11-17 本轮：`stat` 前置的置灰态已补**：官方 file 菜单的 `resolve` **先** `workspaceFiles.stat`，失败时**照样列出**三条动作、逐条 `disabled:!available`（`OFF:context-menus.tsx:50-60` + `:71/:75/:79`）。本仓同一条判据，三档口径都钉住：stat 成功→可点 / stat 失败→**仍列出+全灰**（不是藏菜单）/ 壳没接这条探测→按可点画（登记为差异，不静默降级）。链路：`pluginapi.Actions.WorkspaceFileStat`（`pluginapi/plugin.go`）→ 插件 `plugins/dsfileactions` 的 `resolveFile` 压成 `Disabled` → 壳 `cmd/app/file_actions.go` 的 `workspaceFileStat`（1 秒上限，超时=不可用）→ 新客户端方法 `internal/apiclient.WorkspaceFileStatOf`（只有 `sessionId`+`path`、**无 options**：宿主 `officialParams.finish()` 是 DisallowUnknownFields，见 `HOST:internal/transport/handlers_workspace_official.go:820-843,1212-1242`）。契约快照 `testdata/api.snapshot.json` 新增 `workspaceFiles.stat`（batch24，counts.total 121→122，`snapshot_methods_test.go` 的方法表与计数两处一起守）。`image` 族**不做** stat——官方那一族（`:85-105`）全文没有任何 `disabled`。断言：`TestFileMenuDisabledWhenStatFails`（三档）/`TestImageMenuNeverDisabled`/`TestWorkspaceFileStatProbe`（真 wire 形状+不带 options）/`TestWorkspaceFileStatWithoutClientIsUnavailable`。证据 [official-file-actions-evidence-rc2.md](official-file-actions-evidence-rc2.md) §3/§4.5。<br>**仍是 🟡：四类真差异**：①「复制图片」不画，**唯一平台阻断项**——viva v0.6.4 剪贴板**只有读图片**（`app/clipboard_image.go:22-58` 的 `ClipboardImagePNG`/`HasClipboardImage`），平台层也只有解码侧（`internal/platform/clipboard_image.go` 的 `CheckClipboardPNG`/`TrimClipboardPNG`、`clipboard_dib_windows.go`），**写图片**只活在测试 Driver（`app/driver_system.go:531`）；`SetClipboardFiles` 是复制文件、语义不同不许冒充。唯一补法 = 上游导出生产级「写图片到剪贴板」（Windows 侧 `RegisterClipboardFormat("PNG")` 或 `CF_DIBV5`），按 Viva 线纪律先最小复现→issue→PR，**不做本地 Win32 shim 冒充**。②`image` target **没有生产方**（本行不能升 ✅ 的直接原因之一）：官方两个生产方都是**渲染出来的图片面**——`OFF:dsh-client-ui-attachment/src/MessageImage.tsx:115-118`（消息图片附件，`{url,name,mediaType}` 无 sessionId/path）与 `OFF:dsh-client-ui-chat/src/client/chat/ChatView.tsx:75-79`（markdown 图片，带 sessionId/url/path）；本仓消息区把 image 附件画成**空 64×64 占位**（`plugins/dsconversation/official_transcript_view.go:357-364`），也没有 markdown 图片渲染 → 没有可绑的图片面。补它要先让图片真上屏：viva 的 `ui.Image(c,gfx.Bitmap)` 现成（头像在用 `plugins/dssettings/official_account.go:266`），缺「取字节+解码」链，属 #6/#19 范围。③只有 URL 的 image target 不给动作：官方 `read` 缺省时走 `fetch(source.url)`（`OFF:files.ts:23-36`，只认 http/https/blob/data/dsh-app；跨域还要桌面壳的 `readClipboardImage`），本仓 UI 侧没有这条 URL→字节链，所以宁可不给动作也不画必失败项；补法 = 宿主给一条 URL 取数入口，或 `dsh-app:` 类 URL 在本仓有解析链（今天两条都没有）。④架构性：`Resolve` 是同步签名（官方是 async Promise），所以本次 stat 与保存链都在 **UI 线程同步 HTTP**（各带 1 秒/5 秒上限）——这是「viva 绘制帧里没有事件循环」的同一条代价，不是漏做。<br>**两处旧登记已纠正**：三枚菜单图标其实**已接**（`plugins/dsassets/context_menu.go` 的 `file-actions.copy/download/save`，逐条对 sprite 原文；动作用 `IconCopy/Download/SaveOutlineRegular`）；「在文件夹显示」的卡点**不是宿主**——`session.openWorkspacePath({action:'reveal'})` 宿主已有（`DSH-GO/cmd/dshd/serve_native_api.go:79-90`、`DSH-GO/internal/transport/handlers_native_official.go:233`），缺的是①`internal/apiclient` 没这个方法 ②壳里**没有可承载后续动作的浮条**（`Actions.Notify` 只收字符串，而 `dsnotifications` 的卡片本身能画动作：`plugins/dsnotifications/card.go:152-160`）。**旧实现记录（保留考古）**：`plugins/dsfileactions`（`file` order 200 / `image` order 100 两段菜单：`下载`/`另存为…`，文案逐字 `OFF:locales.ts:5-16`）+ `pluginapi/context_menu.go`（`file`/`image` 两个 kind + 官方逐族字段）+ `plugins/dsfiles`（文件行是 `file` target 的唯一生产方，带会话+工作区相对路径）+ `internal/apiclient/workspace_files.go`（`workspaceFiles.readBytes`，base64 解码）+ `cmd/app/file_actions.go`（壳侧：原生另存/系统下载目录/状态栏反馈）+ `pluginapi.Actions.SaveWorkspaceFile`。 |
| 19 | `dsh-client-ui-file-drag` | ❌ MISSING | **2026-11-16 纠正包归属**：`plugins/dsattachment/overlay.go` 对应 #6 `dsh-client-ui-attachment` 的 `ComposerAttachments → DropOverlay`（官方 `attachment/src/client/ComposerAttachments.tsx:45-60`），不能记作本包实现。官方 file-drag 的 `ctx.fileDrag.actions.prepare/read/reportFailure`（`file-drag/src/client/index.ts:13-49`）、应用内 token/DownloadURL 与桌面 prepare/start/release/cancel（`transfers.ts:32-117,120-154`）、`file-drag.feedback` 失败 Toast（`feedback.tsx:18-25`）本仓均未实现；Viva OnFileDrop 是 OS 拖入、DragSource 是应用内拖放，均非 OS IDropSource 拖出。插画/blur/fade 继续归 #6，不给本包凑 PARTIAL。证据 [清扫报告](official-partial-sweep-2026-11-16.md) |
| 20 | `dsh-client-ui-goal` | ✅ IMPLEMENTED | `plugins/dsgoal`（4 文件 / 364 行） |
| 21 | `dsh-client-ui-input-trigger` | ✅ IMPLEMENTED | `plugins/dscomposer/trigger.go` + `internal/apiclient/references.go`（`@` 文件/会话 + `/` 技能；detect/分组/Tab drill/pick 路由） |
| 22 | `dsh-client-ui-jobs` | ✅ IMPLEMENTED | `plugins/dsjobs`（官方 `conversation.session.header.actions` order 20 触发器 + `shell.overlay` 弹层；旧三栏 `sidebar.footer` 清单）+ `internal/apiclient/jobs.go`（`jobs.output`/`jobs.stop`）+ `cmd/app/jobs_actions.go` |
| 23 | `dsh-client-ui-layout` | ✅ IMPLEMENTED | `internal/uikernel` 骨架 + `plugins/dsshell` |
| 24 | `dsh-client-ui-message-feedback` | ✅ IMPLEMENTED | `plugins/dsconversation/feedback.go` + `internal/apiclient/message_feedback.go` + `cmd/app/reference_wiring.go`（durable + 弹窗 + 冲突收敛 + toast） |
| 25 | `dsh-client-ui-model-selection` | ✅ IMPLEMENTED | `internal/uikernel/official_model_menu.go` + `dscomposer` |
| 26 | `dsh-client-ui-open-in-app` | ✅ IMPLEMENTED | `plugins/dsopen`（3 文件 / 163 行） |
| 27 | `dsh-client-ui-permission-presets` | ✅ IMPLEMENTED | `dscomposer` + `dssettings` |
| 28 | `dsh-client-ui-plan` | 🟡 PARTIAL | **2026-11-17 已闭合本轮两项**：① `internal/apiclient/projections.go` 提取真实 `plan`；`dsplan/chip.go` 按 `pending ? !active : active` 显示，`commands.execute` 调宿主真 `/plan off`（状态文件 + durable `plan/mode`，失败显示官方错误行，不乐观退出）。② 宿主 `exit_plan_mode` 持久 `view.questions`（`intent.kind=plan-review`）且提交后仍 active；`dsquestions/plan_review.go` 父卡按官方 strip/summary/footer 呈现，Approve 经 `userQuestions.answer` 逐字回传并真实退出，Keep planning/custom 保持 active，「要求修改」只关闭卡片交回 composer；`dsplan/review.go` 自动一次/手动查看全文、durable 与临时 `plan-review` 地址及 expired 均已接。**②-b 本轮补**：官方 chip 有两条不同文案 —— `aria-label={t('chip.on.aria')}`（`OFF:PlanModeControl.tsx:56`，文案 `OFF:locales.ts:18`）与 `title={t('chip.on.title')}`（`:57`，文案 `:19`）；本仓原先拿 title 文案顶了无障碍名，现 `dsplan/chip.go` 的 `Hit` 改用官方 aria 文案（`cards.go` 新增 `zhChipAria`），title 文案在 Viva 里没有承载面（无原生 title 属性）。**仍为 🟡 的剩余差异（本轮逐条核实后精确化）**：③ 逐轮渲染 —— 官方 `PlanCards` 每轮 Turn 尾各渲染一次（`OFF:PlanCard.tsx:43-45`，props 带 `turn`）；本仓 transcript 已有逐轮节点 `ChatNodeTurnTail`（`plugins/dsconversation/official_transcript_nodes.go:68`、`official_transcript_view.go:1066` 的 `drawTurnTail`），卡住的是**座位 dispatch 不带轮次**：`pluginapi.Env` 无 turn 字段、`internal/uikernel/shell_skeleton.go:894` 每帧只在会话尾 dispatch 一次，所以本包只能 `latestTurn()` 取最大轮（`plugins/dsplan/plan.go:200-224`）；要补得扩座位契约（pluginapi Env/dispatch 带 turn，牵动 uikernel 与 dsconversation），单包内无法闭合。④ 右栏 tab 只有类型、没有 navigation —— 官方每次 `openResource(planAddress(...))` 打开一份地址（`OFF:index.ts:65-78`、`plan.ts:56-65`），同一类型可并存多份；本仓 keyed 槽的 key 只有类型（`internal/uikernel/view_tabs.go:1-22` 的注册/激活/选择模型），选哪份计划由插件状态承载（`plugins/dsplan/preview.go:95-98`），且**没有 subagent 地址**：官方 `sessions.subagentAddress` 的 `subagent/父/子/mode/callId` 五段地址（`plan.ts:58-60`、`:78-81`）在本仓不可解析，`plugins/dsplan/plan.go:243-278` 只认两段。⑤ **本轮补掉一半**：卡片的 `FileTypeIcon(kind="markdown")` 原先烘了 `deepseek-500`，而官方两个颜色都靠继承 —— 底板 `fill="currentColor"`（= `.card` 的 `--dsw-alias-label-primary`，`OFF:PlanPreview.module.css:17`）、折角/字 `neutral-00` 白（70%/100%，`design-platform.css:40`）；现 `dsplan/cards.go` 的 `planMarkdownIcon(t)` 每帧按主题色现建（`deepseek-500` 只是静态色板 `design-platform.css:28`，官方该部件没引用）。**仍缺**：右栏 tab 芯片的 `FileTypeIcon kind="other" size={16}` 与资源标题（`OFF:PlanPreview.tsx:47-54`）。卡在两头：`Tab.Label` 是注册期静态串（`pluginapi/plugin.go:341-343`）画不出计划标题，`Tab.IconImage` 的图层 Fill 只被 tint 乘 alpha（viva v0.6.4 `gfx/icon.go:526-529`）烘主题色会错色 —— 要内核 tab 芯片支持“按帧用主题色重建图标 + 动态标题”才能补。⑥ 计划正文的相对链接点不开 —— 官方 MarkdownText 把相对路径当工作区文件打开，本仓只认明确 scheme（`plugins/dsplan/preview.go:225-238` + `internal/mdlite.LinkTargetForURL`），缺“相对路径→工作区文件”的解析锚点。**已核实为非差异（不再计入剩余差异）**：`preview.loading` / `preview.failed` 两态在本仓不适用（数据是同步 journal 帧、没有异步资源，`OFF:PlanPreview.tsx:28-34` 的四态在此只会落到 unavailable/notFound/invalidAddress/expired）；卡片几何逐条对上 `PlanPreview.module.css:1-36`；`.cardOpen` 底色用 `BgLayer2`，数值等于官方 `--dsw-alias-button-floating-fill`（浅 `neutral-bluish-00` `design-platform.css:207` / 深 `850` `:359`），非视觉差异。证据 [official-plan-evidence-rc2.md](official-plan-evidence-rc2.md)；历史分类见 [清扫报告](official-partial-sweep-2026-11-16.md) |
| 29 | `dsh-client-ui-plugin-manager` | ✅ IMPLEMENTED | `plugins/dssidebar/plugins_page.go`（侧栏「插件」管理页：列表/详情/启停/卸载）+ `internal/apiclient/plugin_manager.go` + `cmd/app/plugin_manager.go`；证据 [official-plugin-manager-evidence-rc2.md](official-plugin-manager-evidence-rc2.md) |
| 30 | `dsh-client-ui-plugin-market` | ✅ IMPLEMENTED | **A/B/C 三块齐**：`plugins/dsmarket/discover.go`（官方 `plugins.tab` discover 页：索引菜单 / 搜索框 / 标签过滤 / 排序菜单含方向切换 / 刷新 / 分类胶囊 / 卡片 / 骨架 / 两种空态 / 显示更多 / 索引源对话框）+ `plugins/dsmarket/discover_projection.go`（官方 `presentation.ts` 的 query/声明排序/菜单 id/sourceFacts 等价物）+ `plugins/dssidebar/plugins_tabs.go`（`plugins.tab` 台账 + tab 条 + 贡献方座位，官方 `PluginManagerPage.tsx:1732-1760`）+ `internal/apiclient/plugin_market_discover.go`（`listIndexes`/`search`/`setIndexes`，形状与官方逐字段一致）+ `cmd/app/market_discover.go`（壳状态机，照 `market-store.ts` 移植）+ `plugins/dsmarket/detail.go`（C 块详情页，`ui.market.detail` 服务已挂上）；证据 [official-plugin-market-discover-evidence-rc2.md](official-plugin-market-discover-evidence-rc2.md)。**剩余差异 12 条（HIDE，宿主缺口 6 + 素材/能力 6，逐条见该文件 §5）**：安装/更新/在已安装中查看不画（宿主 501）／详情相对图片不出（无 `detail-file` 路由）／条目图标图不出（无图片加载器与官方精灵图，框内画标题首字）／刷新·移除·过滤三枚 sprite 无导出（文字化）／骨架不带动画／工具栏不换行（Viva `Flow` 不支持 flex:1）／`Pill`·`Tag` 用仓库已冻结的同一份官方度量／卡片悬停底色不画／`readInvalid` 档不可达（宿主把非法索引也回 `index_unavailable`）／无 `plugin-market/changed` 事件（不自刷新）／`listIndexes` 空配置回 503（客户端认成空清单，待宿主改回 `[]`）
| 31 | `dsh-client-ui-plugin-recommendations` | ✅ IMPLEMENTED | **`plugins/dspluginreco`（`enable_official_plugin` 卡片正文：header/目录标题+描述/状态行/footer 两个按钮）+ `pluginapi/action_card_bodies.go`（正文注册表）+ `plugins/dsactioncards`（注册表提供方与按 kind 派发）+ `internal/apiclient/recommendations.go`（`pluginRecommendations.describe`）+ `cmd/app/official_panels_wiring.go`（describe + 查看详情接线）**；证据 [official-plugin-recommendations-evidence-rc2.md](official-plugin-recommendations-evidence-rc2.md)。**剩余差异**：`.details:focus-visible` 的 2px 内描边不画（viva 的 `Ring` 画在外侧，形状对不上）；`.header`/`.footer` 的 `flex-wrap` 无对等物；`aria-describedby` 无 ARIA 关系表。几何/令牌已逐条对齐官方 `Card.module.css:1-13`（骨架条、整块按钮、module-platform 底色均已落地，见证据文档 §3.1） |
| 32 | `dsh-client-ui-reference` | ✅ IMPLEMENTED | `plugins/dsconversation/usertext.go`（file/folder/session chip + PreviewFile）+ `plugins/dscomposer/trigger.go` + `internal/reficon` |
| 33 | `dsh-client-ui-renderer` | ✅ IMPLEMENTED | `internal/uikernel`（槽 dispatch） |
| 34 | `dsh-client-ui-schedule` | ✅ IMPLEMENTED | `plugins/dssidebar/schedule_page.go`（任务管理页）+ `internal/apiclient/schedule.go`（`schedule.catalog/edit/run/history`）+ `cmd/app/schedule_catalog.go`；侧栏摘要仍在 `plugins/dsschedule` |
| 35 | `dsh-client-ui-secret-mask` | ✅ IMPLEMENTED | `internal/secretmask`（官方格式表）+ `plugins/dsconversation/usertext.go`（逐值显示/隐藏） |
| 36 | `dsh-client-ui-session` | ✅ IMPLEMENTED | `internal/apiclient` + `cmd/app` |
| 37 | `dsh-client-ui-settings` | ✅ IMPLEMENTED | `plugins/dssettings`（14 文件 / 5470 行） |
| 38 | `dsh-client-ui-settings-account` | 🟡 PARTIAL | **2026-11-17 补三处可见件**：① **头像图片加载**已接真实现——`dssettings/official_account_avatar.go` 逐条复刻官方 `account-avatar.ts:11-92` 的三态机（`setUrl` 空/未变即空操作 `:24-33`、`retry` 只重试 failed `:36-38`、`reset`/`dispose` 丢弃回调 `:50-60`、`load` 先发 loading `:75`）、`fetch` 不带 Referer（官方 `referrerPolicy='no-referrer'` `:66`）、解码成 `gfx.Bitmap` 后只有 `status==='ready'` 才 `ui.Image`（官方 `AccountAvatar.tsx:15-18`），失败/未就绪画 `IconUserOutlineMedium size=16`（`:18`，几何 `AccountAvatarIconSize=16`）。② **赠金/充值行的「前往开放平台查看」下划线链接**已接（官方 `AccountSection.tsx:186-188,200-201` 的第三分支 + `.unavailableLink{text-underline-offset:3px}` `AccountSection.module.css:123-141`；目的地取 `state.links.usageUrl` 不是钱包响应，官方注释 `:125-131`），未登录时改回官方 `balanceSignedOut`「登录后查看」（`locales.ts:66`）。③ **gener 侧 FontFamily 输入行**确认**不属于本包**，已交 `#13 settings-general` 记录。**仍不升 ✅的五条真差异（逐条精确）**：(a) `account.watch` 长连流未接——`internal/apiclient/account.go` 只有一次性 `account.getState`，壳 `cmd/app` 30s 轮询；挡住项是宿主 `/ws` 没有 `account/watch` 订阅 type（白名单只有 v4/text/turn/approval/reset/pong），所以 push 失效、`credentials-changed`、`frame.generation` 三条都无处落地；(b) **两类 onboarding 全缺**——官方 `AccountOnboarding.tsx`（`settings.models.sign-in` 槽）+ `DesktopOnboarding` 五步（`DesktopOnboarding.tsx:1-125`、`onboarding-state.ts:246`），本仓既无该槽也无 `ui-settings-account` 命名空间的 configForms，故不画；(c) **`shell.quota-notice` 槽（`ACCOUNT_QUOTA` Modal）未接**——官方 `AccountQuotaNotice.tsx:1-81`，本仓内核无该槽；(d) **Platform page 内嵌页缺**——官方走 Electron `dshPlatform` 原生 WebContentsView（`platform-pages.ts:1-26`、`AccountPlatformHost.tsx:221-227`），Go GUI 无该桥，只能 `OpenExternal` 到系统浏览器，故 `.linkButton` 的「页内打开」语义退化为「外开」（这条是形状差异不是缺失）；(e) **`contactUs` 缺**——官方 `contact-url.ts:173-202` 用 `__DSH_CONTACT_CONFIG__` 注入的飞书问卷 + `dshDesktop.deviceInfo()`，本仓既无该全局也无设备信息桥，按「不自造」不画按钮（AccountMenu 的 `contact` 项随之 HIDE）。逐条见 [清扫报告](official-partial-sweep-2026-11-16.md#38-dsh-client-ui-settings-account--保持-) |
| 39 | `dsh-client-ui-settings-agent-loop` | ✅ IMPLEMENTED | **`plugins/dssidebar/settings_items.go` + `settings_items_draw.go`**（插件管理页「内置插件配置」组的 `agent-loop` 卡：`maxParallelToolCalls` 一个数值字段，官方 lock/`hint` 逐字，写走 `settings.mutate`）+ `plugins/dssidebar/plugins_page.go`（组与详情接线）；证据 [official-settings-pages-evidence-rc2.md](official-settings-pages-evidence-rc2.md) §2.1 |
| 40 | `dsh-client-ui-settings-general` | ✅ IMPLEMENTED | `dssettings` 的 general 页 |
| 41 | `dsh-client-ui-settings-models` | 🟡 PARTIAL | **2026-11-17 核实 `ProviderEditor` 字段面**：官方 `ProviderEditor.tsx` 的**全部控件类型**已逐项核到位，与账上此前的「完整 ProviderEditor 全部字段类型」粗描述相比，真实剩余只有下面 6 条（**没有**「schema 驱动文本/code 控件」这种说法——官方并不是按 schema 渲染，而是按 adapter 家族 `layoutOf(ns)` 手写两套固定字段，`ProviderEditor.tsx:170-175`）：(a) **API key 单字段**（`ApiKeyInput.tsx:1-68` 的写-only 输入 + `keyPlaceholder`/`keyPlaceholderFallback` 提示 + `apiKey.ts` 校验 + `operations.storeCredential`→`credentials.set`）：`internal/apiclient/credentials.go` 的两条方法已在，但 `official_models*.go` 没有这张卡，所以密钥读写仍缺；(b) **`baseURL` 文本输入**（`ProviderEditor.tsx:551-573`，placeholder 三分支 `deepSeekBaseUrl`/`baseUrlPlaceholder`/`baseUrlConfigured`/`baseUrlDefault`，`locales.ts:56-62`）；(c) **`displayName` + `api` 两个 `ownsIdentity` 字段**（仅手声明 pi-ai 路由，`ProviderEditor.tsx:525-549,575-598`，`api` 是 `<select>` + `customApiUnset` 空选项）；(d) **模型目录编辑器**（DeepSeek 侧 `DeepSeekModelsEditor.tsx:1-256` 的 id/name/contextWindow/maxTokens 行，pi-ai 侧 `ModelListEditor.tsx:1-457` 的 `ModelInputTypes.tsx` 输入类型复选 + `ModelReasoningEfforts.tsx` reasoning 档位）；(e) **`EditorFooter` 三态**（`EditorFooter.tsx:1-67`：Apply / Applying / Cancel，Cancel 文案在密钥移除已应用时变 `close`）；(f) **发现测试**（`ModelListEditor` 的 `probe`→适配器 endpoint 询问）。**已实现**：provider 目录真实 join、模型目录分组、`model.defaultModel` 写、retryPolicy 卡内草稿（Cancel 丢弃 / Apply 一次最小 `settings.mutate` 带开卡 revision / 冲突保留草稿，`official_models_retry.go`）、`settingsPathUnresolvable` 之外的未知 namespace 提示（`advancedHint`）。**挡住项**：`credentials.set/describe` 宿主已通但 Models 页未接（可补，属工作量而非缺能力）；`probe`（发现测试）要宿主 `llm` 侧的 endpoint 询问方法，本仓宿主未提供同名方法，属真缺能力。逐条见 [清扫报告](official-partial-sweep-2026-11-16.md) |
| 42 | `dsh-client-ui-settings-plugin-inventory` | ✅ IMPLEMENTED | `plugins/dssettings/official_plugins.go` + `official_plugins_draw.go` + `internal/apiclient/plugin_inventory.go` |
| 43 | `dsh-client-ui-settings-plugins` | ✅ IMPLEMENTED | `plugins/dssettings/official_tree.go:83` + `official_plugins*.go`（外壳 + tab 台账） |
| 44 | `dsh-client-ui-settings-session-log` | ✅ IMPLEMENTED | **本轮 B 补齐官方两块**：general Switch `session-log-deepseek`（官方 description + describe 驱动的 whileServed + busy 防重入）与 `shell.overlay/session-log-upload-toast`（官方 saved/failed 文案、自动 dismiss、代际隔离）；落点 `plugins/dssettings/official_tree.go:325`、`official_panel.go:529,653`、`session_log.go:16-140`、`cmd/app/settings_panel_wiring.go:116-137`。**宿主上传方仍未做**：`enabled=true` 返回 501（`DSH-GO/internal/settingsform/client_ui_schema.go:824-846`），UI 按官方失败 Toast 呈现，不伪报上传成功。证据 [清扫报告](official-partial-sweep-2026-11-16.md) |
| 45 | `dsh-client-ui-settings-shell` | ✅ IMPLEMENTED | 同一落点的 `shell` 卡（`timeoutMs` / `maxOutputBytes`，按官方 `served.has(PWSH_NS) ? pwsh : bash` 绑定 `pwsh-sandbox`/`bash-sandbox`）；证据 [official-settings-pages-evidence-rc2.md](official-settings-pages-evidence-rc2.md) §2.2 |
| 46 | `dsh-client-ui-settings-subagent` | 🟡 PARTIAL | 同一落点的 `subagent` 卡：**「运行限制」与「模型选择」两节均已落地**（2026-11-17）。`allowedModels` 从 `SettingsNamespaceView.Value` 原始 JSON 投影；enabled Switch、按 provider 分组候选、失效路由保留/移除、loading/error/partial/empty、至少一项校验、revision conflict、同 revision 一次 `settings.mutate` 写 `enabled + allowedModels` 全部接通。落点 `plugins/dssidebar/settings_items_subagent{,_draw}.go`，真 Viva 点击/保存证据见 [official-settings-pages-evidence-rc2.md](official-settings-pages-evidence-rc2.md) §9。**仍为 🟡（D，本轮把差异写精确）**：官方这一包的可见面有**宿主行为消费方** —— `dsh-tool-subagent/src/model-selection-settings.ts:54-76` 在 `agent/created` 把授权路由写成 durable `subagent/model-selection-policy`，`model-selection-state.ts:33-47` 投影成 `subagentModelSelectionPolicy`，委派时由 `model-selection.ts:100-150` 要求 provider+model 成对并校验白名单，另有 `list-models.ts:100-135` 的 `list_subagent_models` 工具。本仓目前只有**可读写表单**：`allowedModels` 已落 `internal/settingsform/subagent_schema.go:44,124-149`，但没有任何消费方（同文件 `:49-68` 自登记），委派侧 `subRunner.model` 固定父模型（`cmd/dshd/subagent.go:33`）、`subagent` 工具 schema 无 `provider/model/reasoning_effort`（`modules/tool/subagent/subagent.go:42-60`），也没有该 durable 事件与投影、也没有目录工具。**卡在哪**：这四处都在宿主（DSH-GO）仓，GUI 侧本仓已无缺口，补不了。**什么条件下能补**：前提已具备一半 —— 宿主已有真实模型目录与按会话选择面（`cmd/dshd/serve_session_api_t04.go:449-520` 的 `session.modelCatalog` + `h.selections`），所以可在宿主侧按序补：设置→会话授权事件→投影→工具三参数校验→子代理路由生效，再把 `list_subagent_models` 挂进工具表。表单可写不等于委派时真能选模型，这条不闭合就不改 ✅。 |
| 47 | `dsh-client-ui-settings-web-search` | ✅ IMPLEMENTED | 同一落点的 `web-search` 卡（`order:20`，`plugins/dssidebar/settings_items_web.go` + `settings_items_web_draw.go`）：两个 provider 下拉 + 灰面板（`web-deepseek-api` 的 `count` / `searchBilling` / `fetchBilling`，`web-search-deepseek` 的 `baseURL` / `maxUses` / 密钥）+ 官方「已被 … 覆盖」锁 + provider 清单失败提示与「重试」；provider 清单走 `web.listProviders`，密钥走 `credentials.describe`/`credentials.set`（**不进 settings**）。证据 [official-settings-pages-evidence-rc2.md](official-settings-pages-evidence-rc2.md) §8。**缺口（宿主侧，已登记）**：legacy 搜索 provider 只读环境变量（`modules/search/deepseek.go:77`），所以密钥写进凭据库后**不改变搜索行为**——界面照官方发布面画，能读写、能往返 |
| 48 | `dsh-client-ui-shortcuts` | ✅ IMPLEMENTED | `plugins/dsshortcuts`（4 文件 / 473 行）；几何与样式逐项对照官方 `Reference.module.css`（`official_geometry.go` 带 file:line），关闭图标复用官方 sprite |
| 49 | `dsh-client-ui-sidebar` | ✅ IMPLEMENTED | `plugins/dssidebar`（5 文件 / 1418 行） |
| 50 | `dsh-client-ui-sidebar-browser` | 🟡 PARTIAL | **2026-11-16 多实例与恢复已补齐**：`plugins/dsbrowser` 注册官方 `multiple:true`；`cmd/app/right_tab_occurrences.go` 按 Session 管理独立 occurrence / activeTabId / minted，芯片行官方 add/close SVG 按钮真实新建与关闭，单实例类型仍复用。每个 `(sessionId,tabId)` 独立保存 `BrowserNavigation`、`WebPage`、地址草稿与 late-load 状态，芯片标题显示各自 hostname。两层恢复均走既有 `config.get/config.set`：`dsh.sidebar-right.v1` 恢复 tab 名录/顺序/选中项（全关后的空态也恢复），`dsh.sidebar-browser.v1` 按 `byTab` 恢复 `entries/index/request/navigation/failure`；重启只显示官方「上次打开 / 恢复页面」卡，**不自动 Open**，点击后才创建 WebView，revision +1 且不重复历史；关闭先 Close page、forget checkpoint，再删布局。官方依据 `definition.tsx:19`、`BrowserController.ts:176-197,205-228`、`store.ts:20-33`、`BrowserBody.tsx:118-125`、`sidebar-right/service.ts:519-539`。机器证据：[多实例与恢复证据](official-sidebar-browser-occurrences-evidence-rc2.md)。**2026-11-17 补齐工具条视觉与交互（逐条对官方 CSS）**：① `.addressGo` 只在 `.addressBox:focus-within` 显形（`Browser.module.css:72-83`），判据取 `gfx.Canvas.Focused()`（旧注释说『没有聚焦判据』是错的）；② 新增 `.addressChanged`「URL 已变化」角标（`:98-113`，top 6 / right 8 / padding-left 8 / 11px-16px / bg-layer-2，聚焦隐去）；③ unknown 态地址框配色（`:85-96`：label-tertiary+bg-layer-2，聚焦回 label-primary+bg-layer-1）；④ `.address:focus` 1px 描边（`:115-118`，token 即 `t.Info`=`--dsw-alias-state-business-primary`；Viva 的 `Ring` 画在盒外、官方 `outline-offset:-1px` 内缩，差 1px）；⑤ `.tool` hover 变色（`:37-42` label-secondary→label-primary）；⑥ `.failure` 8% 错误色底 + 行高 14（`:218-224`，`--dsw-font-xxxs-11`=11/14），`.limit` 行高同改（`:234-244`）；⑦ `.toolbar` 回到官方 border-box 总高 38（描边算在线内，原实现 38.5）；⑧ 地址 unknown 时「在系统浏览器中打开」置灰（`BrowserBody.tsx:73,99`）；⑨ 后退/前进/刷新清 `addressFailure`（`BrowserController.ts:163-168`）、载具错误行与地址拒绝对行并列（`BrowserBody.tsx:112-115`）、恢复卡只在无载具错误时画（`:126`）。测试：`plugins/dsbrowser/toolbar_test.go`。**仍为 🟡 的剩余差异（精确）**：① ~~`window.open/new-window` 不能开成带 URL 的应用内新标签~~ **2026-11-17 已闭合（壳契约补齐，本项消失）**：官方 `openRequested → openTab('browser',{params:{url},revealIfOpened:false})`（`ElectronWebViewImpl.ts:155-158`、`BrowserController.ts:54-59`、`BrowserBody.tsx:62-70`）现在逐条落地——契约新增 `pluginapi.RightTabParams`（= 官方 `SidebarRightTabParamsMap`，`contract/params.ts:31-46`；browser 那一支 `index.ts:32-37` 是 `{url?: string}`）、`Actions.OpenRightTabParams(kind,params)`、`Env.RightTabParams`（= 官方 `tab.navigation.params`，`contract/slots.ts:109-121`）、`WebOpenOptions.OnNewWindow`（= 官方 `BrowserPageOptions.openRequested`）；壳 `cmd/app/web_pages.go` 把 viva 的 `NewWindow` 交回插件（返回 false 表示应用内接下，不再另开系统窗口），`plugins/dsbrowser` 按官方 `openRequested` 先过地址白名单（拒则记 `addressFailure`）再 `OpenRightTabParams('browser',{url})`，新 occurrence 首帧消费 `Env.RightTabParams.URL` 作 initialUrl（官方 `start(initialUrl)` 一次性语义，`BrowserController.ts:90-98`）；壳 `cmd/app/right_tab_occurrences.go` 按 occurrence 存参数、关闭即丢、不持久化（官方 navigation 不属于持久化 tab 记录）。测试 `plugins/dsbrowser/window_test.go`（新标签带 URL / 非法拒开 / 壳没接通道时交回平台默认 / 参数只消费一次）、`cmd/app/right_tab_params_test.go`。② 页面生命周期：官方 Electron 有 dom-ready / did-navigate(-in-page) / did-start-navigation / did-start-stop-loading / page-title-updated / did-fail-load / render-process-gone（`ElectronWebViewImpl.ts:159-186`），本仓只吃 `WebLoad` 的 Done（Start/Committed 可用但未接），`did-fail-load` 的 code/description 与 `getTitle` 在 Viva 契约里不存在（`viva@v0.6.4/app/web.go:186-191,278-306`）→ 补不了，`load.failed.detail` 文案因此不可达。③ 下载：官方 iframe 沙箱无 `allow-downloads`（`IframePresentation.ts:7`）即拒，本仓同（`web_pages.go:65`），此条与官方 iframe 载具一致；桌面载具主进程实现不在 rc.2 npm 快照内 → **未核实**，不冒充已对齐。④ 存储分区：官方按 workspace 分 storage account（`electron/workspace.ts:31-32`），Viva `WebViewConfig` 无分区字段（`app/web.go:86-127`）→ 补不了。⑤ 后退/前进走应用历史（`navigation.go:88-98`）而非引擎历史：官方 iframe 载具同语义，Electron 载具用原生 `canGoBack/canGoForward`（`ElectronWebViewImpl.ts:225-226`）而无 API 可取 → 保持 iframe 语义。⑥ `.tool:disabled` 色：官方用 `--dsw-alias-label-quaternary`，该 token 在 rc.2 **全部产物里只有引用没有定义**（`design-platform.css` 0 处赋值、`app.asar` 内 4 处 `var(--dsw-alias-label-quaternary)` 全是用例），按 CSS 层叠属 invalid-at-computed-value、回退成继承的 label-primary；本仓用 `LabelDimmed` 让禁用看得出来（有意，不是漏做）。⑦ 地球赤道 `stroke-linecap="square"`：`gfx.Icon` 只有一个 `Cap` 字段（`gfx/path.go:66-74`），`dsassets.Build` 冲突即不画 → 单枚图标补不了。⑧ `.restoreTitle/.restoreUrl` 在本仓按 3/4 行截断：Viva 无 `overflow-wrap:anywhere`、`.restore` 也无滚动容器。⑨ `application-origin` 判据在原生应用不可达（无 document origin），`error.application-origin` 文案因此不可达。⑩ 右栏机制面（归 #53）：guide 入口、`browser.new`=Ctrl+T、tab 内 badge、`.tabTitle{gap:5}` 未核实（dockkit 包不在 rc.2 快照）。⑪ 持久化键含 sid、值多一层 `bySession`（`persistence.go:17-27`），与官方 `dsh.sidebar-browser.v1` 根 `{byTab}` 形状不同（内部约定差异，不影响可见）。官方 optional sandbox 不算缺口，未新增宿主 API 方法 |
| 51 | `dsh-client-ui-sidebar-documentpreview` | ✅ IMPLEMENTED | `plugins/dsfiles` 预览浮层 |
| 52 | `dsh-client-ui-sidebar-files` | ✅ IMPLEMENTED | `plugins/dsfiles` |
| 53 | `dsh-client-ui-sidebar-right` | ✅ IMPLEMENTED | `dsrightbar` + `dsshell` + uikernel；右栏现在是**两阶段注册 + tab 条**（2026-11-15）：类型/体分开注册、按名录派发选中项，见 [official-view-tabs-evidence-rc2.md](official-view-tabs-evidence-rc2.md) §2.4 |
| 54 | `dsh-client-ui-sidebar-terminal` | 🟡 PARTIAL | `plugins/dsterminal` + `internal/ansi`；官方 tab 座位已接。宿主用户终端 create/attach/list/close/environment/shells/rename/retain/watch 与 PTY input/resize 已有（`DSH-GO/internal/transport/handlers_job_terminal_official.go:89-100`）。**2026-11-17 补齐**：① 契约加 `TerminalViewState.Issue`（官方 `dsh-api-terminal-controller/src/client/model.ts:19,47` 的五档分类），壳只给 key、界面按官方 locale 渲染——修掉**内部 key 泄漏**（原先把 `missingTerminal`/`attachmentEnded` 当错误原文打成「终端错误：missingTerminal」）；② `missingTerminal` 画官方 `.empty` 空态（`terminal.module.css:7-8`、`terminal.tsx:36-45`：24px 内边距 / gap 16 / `.emptyMessage` 28ch+1.6 / 主要按钮「新建终端」），其余四档 issue 用官方中文文案（`locales.ts:21-25`）；③ `phase=failed` 且 `info===undefined` 不再印「不可用」（`terminal.tsx:46-52` 的状态链里没有这一支）；④ `.name` 补 `max-width:100%`（`TerminalTitle.module.css:2`）；⑤ tab 芯片图标与文字同色（`TerminalIcon.tsx:9` 的 `currentColor` + `TerminalTitle.module.css:1` 的 inherit）→ 改在壳 `internal/uikernel/view_tabs.go`（选中 label-primary / 未选中 label-secondary）。测试：`plugins/dsterminal/issue_test.go`。**仍为 🟡（整条不能升 ✅）的剩余差异，逐项写清缺什么 / 被什么挡住**：① Agent 与沙箱徽标、控制权移交、`typed` 语义：**数据源为零**——宿主 `aligned()` 把 origin/control 固定为 user`（DSH-GO/internal/transport/handlers_job_terminal_official.go:267-271）`、无 agentLaunch/openForAgent 路径，`terminal.input` handler 只解析 id/data 并丢掉 `typed`（`DSH-GO/internal/transport/handlers_terminal.go:39-63`）；补的条件：宿主补 agent 终端生产者 + typed 透传，缺此两项徽标与控制权移交无从渲染。② ~~tab 内徽标缺壳契约、per-tab 参数缺 `OpenRightTab` 的 params~~ **2026-11-17 已闭合契约面**：`pluginapi.Tab.Badge`（+ `TabBadge`/`TabBadgeTone`/`RightTabBadgeProvider`，形状逐条来自 `TerminalTitle.tsx:17-20,44-50` 与 primitives `Tag` 的 `._tag_brmue_4` 几何）由壳在投影名录时按帧问插件，`internal/uikernel/view_tabs.go` 在芯片标题**之前**画那枚 `Tag`（`.badge{display:inline-flex;flex:none}`）；`Actions.OpenRightTabParams` + `Env.RightTabParams`（官方 `SidebarRightTabParamsMap`：browser `{url?}` / terminal `{terminalId}|{shellPath}`，`index.ts:32-37` + `face.ts:20-24`）已接，壳在 ensure 终端时消费 `shellPath`（`cmd/app/terminal_wiring.go` 的 `pickTerminalShell`，回退规则照官方 `launchShells` 的 `selectedShell`）。**仍缺**：双击 tab 改名（`RightTabTitle` 只读返回 string，缺芯片编辑通道）；`terminalId` 参数的消费（官方是「接管一条 Agent 已开的终端」，与 ① 的数据源是同一处宿主缺口，见本行 ①）。证据 `internal/uikernel/view_badge_test.go`、`plugins/dsterminal/guide_test.go`、`cmd/app/right_tab_params_test.go`、`cmd/app/right_tab_badge_test.go`。③ VT 面：无 xterm 等价物，`internal/ansi` 只剥 CSI/OSC；光标四形态+blink（`terminal.module.css:12-19`）、ANSI 调色与 OSC 10/11/12（`terminal-theme.ts:22-50`）、selection、scrollback、rows 自适应都缺，resize 卡在**客户端没有上行通道**：宿主 PTY resize 已在（`remote.mux` 的 `resize` 帧 → `DSH-GO/cmd/dshd/serve_terminal_api.go:611-624` → `terminal_pty.go:768`），但 HTTP 面没有 `terminal.resize` 方法（`internal/transport/api.go:86` 只有 `terminal.input`），本仓 `internal/apiclient` 的 `TerminalStream` 只有 Next/NextRoster/NextRetained、无 Send → 要接先要有 mux 上行客户端。→ 本质差异，不是漏一两处。④ ~~`TerminalGuide` 整组未实现，卡在 `pluginapi` 未导出 `sidebar.right.tab.guide.entry`~~ **2026-11-17 已实现（本项从「未实现」降为两处已登记的可见差异）**：`pluginapi` 新增三个槽常量 `SlotRightGuide` / `SlotRightGuideActions` / `SlotRightGuideEntry`（`contract/slots.ts:79-99`）、owner props `GuideEntryProps`（= 官方 `SidebarRightGuideEntryOwnerProps`，`slots.ts:236-242`）、`Env.GuideEntry`、`Env.HasKey`（keyed 座位「这个 key 有人吗」，官方每张卡都带 `fallback`，`GuideBody.tsx:150-157`）、`Painter.Guide []TabGuide`（= 官方 `SidebarRightTabDefinition.guide`，`tab-registry.ts:66-135`）；壳侧 `internal/uikernel/view_guide.go` 投影名录（`GuideEntries`）并按 keyed 座位逐卡派发 + 官方胶囊 fallback，`plugins/dsterminal/guide.go` 按 `TerminalGuide.module.css:1-12` 画卡片本体（56 高 / 14×20 padding / `Radii.XL` 圆角 / 26 图标 / 14-1.4 标题 / 11-1.4 描述 / 20 trigger+14 chevron / loading-ready-failed-empty 四态菜单），操作面走新增的 `Actions.TerminalLaunchShells`（= 官方 `webTerminals.launchShells`，`api-terminal-controller/src/client/index.ts:169-185`）与 `Actions.TerminalSelectShell`（= 官方 `selectShell`）。**两处仍然不同（不算漏做）**：① 卡片图标用本包已登记的 16-box 终端字形按官方 26px 画，官方那枚 `PluginArtworkTerminal` **没有登记素材**；② 官方菜单是 primitives `Menu`（portal 到窗口层），Viva 没有门户层，本仓画成贴 trigger 的绝对定位面板（面/行的度量取仓库已冻结的官方菜单档，见 `plugins/dsterminal/guide.go` 常量表）。向导页本体仍只到「入口卡列」：页首 56px 罗盘（`.hero`）、More 卡片（`guide.actions`）、固定/取消固定（entryPins）归 #53。证据 `internal/uikernel/view_guide_test.go`、`plugins/dsterminal/guide_test.go`。⑤ `terminal.new`=Ctrl+` 与 `shortcut.noSession` 缺快捷键注册表；`.status` 的 `flex-wrap:wrap` 缺 Viva 容器 wrap。⑥ `inputFull`/`invalidOutput`/`terminalLimit` 只有文案、没有触发点：壳多数路径仍塞自由文本（`cmd/app/terminal_wiring.go` 的输入上限与画面帧错误），只有 `missingTerminal`/`attachmentEnded` 走分类。⑦ 空态里那枚 36×36 终端插画未画（只存在于官方产物，无可登记素材）；`close` 语义仍与官方不同（官方 handler 同步 void + 后台重试队列，本仓返回 error 阻止关 tab），`retainTabs` 的全量保命面也未接。 |
| 55 | `dsh-client-ui-skill` | ✅ IMPLEMENTED | `plugins/dsskill`（7 文件 / 2238 行） |
| 56 | `dsh-client-ui-subagent` | ✅ IMPLEMENTED | 目录行本体：`plugins/dsagentteam/catalog.go`（disclosure/递归层级/token+时长指标/当前会话对勾/Esc）+ `catalog_metrics.go` + 壳侧逐子会话投影 `cmd/app/wave3_wiring.go`；footer（TeamBoard）仍是既有落点；证据 [official-subagent-evidence-rc2.md](official-subagent-evidence-rc2.md) |
| 57 | `dsh-client-ui-theme` | ✅ IMPLEMENTED | `dscore` + `cmd/app/theme.go` |
| 58 | `dsh-client-ui-tool` | ✅ IMPLEMENTED | `plugins/dstool`（7 文件 / 930 行） |
| 59 | `dsh-client-ui-trajectory` | ✅ IMPLEMENTED | `plugins/dsrightbar`；**2026-11-16 原六项差异已收口**：原始 Frames 折成 system/user/context/compacted/assistant/tool/subtool 完整账本；assistant calls + PTC 递归 subcalls + Turn/Assistant/Tool 三套折叠；四模式三轨 timeline（duration/actual-time、TTFT/解码、缩放/拖选/平移/Esc）；request/options/usage/timing/schema/diff 多页 Inspector；resident 50 条增页后接真实 `Actions.LoadOlder`，完整路径不再限最近 80 条；删除 `main.body` 视口偏移桥，只走 `conversation.view`/`order:10` 与右栏正式座位。Raw result 无 live watch 时按官方显示 unavailable，不伪造历史缓存。逐条官方 `src/**:line`、实现与测试证据：[official-trajectory-evidence-rc2.md](official-trajectory-evidence-rc2.md)。 |
| 60 | `dsh-client-ui-user-questions` | ✅ IMPLEMENTED | `plugins/dsquestions`（3 文件 / 423 行） |
| 61 | `dsh-client-ui-workflow-run` | ✅ IMPLEMENTED | `dsconversation` 的 workflow-run 节点 |
| 62 | `dsh-client-ui-workspace` | ✅ IMPLEMENTED | `internal/apiclient/fs.go` + `dssidebar` + `plugins/dssidebar/workspace_actions.go`（`workspace.registry`/`rename`/`delete`，#10 contextMenu 入口）+ **`plugins/dssidebar/grouping.go`（按 `registry.SessionIDs` 归属与手动序、「未分组」、registry 空时回退 cwd）+ `drag.go`（拖拽 → `workspace.insertSessionBefore`）**；证据 [official-workspace-order-evidence-rc2.md](official-workspace-order-evidence-rc2.md) |
| 63 | `dsh-experimental-client-ui-cot-translation` | 🟡 PARTIAL | **2026-11-17 本轮（feat/pv-cot-forge）：补 3 条、状态不变（仍 🟡）**。① **正文改走官方那条 Markdown 工厂**：`OFF:TranslationBody.tsx:60` 是 `renderFactorySlot('conversation.chat.reasoning.content',{text,running})`，默认实现就是 `MarkdownText variant="compact"`（`OFF:dsh-client-ui-chat/src/client/chat/ReasoningContent.tsx:14-17`）；本仓等价物是排版后覆画的 reasoning 专用 slot：`plugins/dsconversation/cot_translation.go` 的 `drawCotBody` + `layoutReasoningMarkdown`（字号 `theme.Fonts.Secondary` = compact，代码块工具条走既有 `paintMarkdown`）。② **provider 不可用时入口不再隐藏**：旧代码把「设置项缺失」当「功能缺失」（`cotToolbarAvailable` 要求偏好 provider 在清单内）；官方同一座位照样挂，只加一句 `providerUnavailable`（`OFF:TranslationSettings.tsx:58-60`、`OFF:locales.ts:31` 中文逐字），现按官方改（`cotProviderUnavailable` + 常量 `cotLabelProviderUnavailable`）。③ **pending 从一行文字改成 spinner**：官方是 `IconLoadingOutlineRegular size=14` + `aria-label=translating`（`OFF:TranslationBody.tsx:53-55`），现画 `ui.Spinner`；工具栏几何按 `OFF:Translation.module.css:1-13` 补齐（`min-height:28` / `gap:4` / `margin-bottom:4` / `.viewToggle{margin-inline-start:-10px}` / `.failure{font-size:13px}`）。**仍为 🟡 的剩余差异 5 条（逐条写清缺什么、被什么挡住、什么条件下能补）**：① **spinner 图形仍非官方图标**：Viva `ui.Spinner` 是 16px 八辐条，官方是 14px `IconLoadingOutlineRegular` 单色描边圆环；补的条件——把官方那张 SVG 的 viewBox + 命令抄进 `plugins/dsassets`（同 `checkIcon` 的做法），再把 `ui.Spinner` 换成 `ui.Icon`。② **非译文的思考正文仍不是 Markdown**：本仓思考块展开态正文（没挂译文工具栏时）仍 `ui.Wrap` 纯文本（`reasoning.go` 那条既有刻意差异），官方两条都走同一个 content 工厂；补的条件——把 `reasoning.go` 展开态正文也切到 `layoutReasoningMarkdown`，会动 reasoning 全套 golden，需单独一轮。③ **原文仍可带 `<signature>`**：展开态原文是 `reasoningBodyText` = `msg.Text`，签名块只在**翻译输入**里被 `reasoningDisplayText` 剥掉；官方正文拿到的是宿主已剥的 `text`。补的条件——宿主投影剥签名，或本仓展开态正文改用 `cotSourceText(msg)`（同样动 golden）。④ **收起不取消、缓存常驻**：官方 `ReasoningTranslation.dispose()` 在 unmount 时 `controller.abort()`（`OFF:translation.ts:104-107`），译文只活在 disclosure 挂载期间；本仓状态键是 MessageKey，收起后不再推进（`conversation.go` 只在展开态调 `cotAdvance`）但**状态与 cache 不释放**，反复展开命中缓存不再付费（刻意的省钱行为，语义与官方相反）。补的条件——收起时清 `p.cot[key]`，并同时钉住「重新展开会重翻」（这是付费行为变更，要单独立测）。⑤ **设置写入已接通刷链，但缺官方的实时事件**：宿主 `settings.form.set/reset` 写 `cot-translation` 后 `cmd/app/settings_panel_wiring.go:164,207` 已调 `cotTranslationRefresh()`；重连 `main.go:1037`、账号 `account_settings.go:38`、凭据 `web_provider_wiring.go:177` 也都会刷新（对应官方那六个 `$on`）；**缺的是官方 `settings/document-updated` / `llm/adapters-updated` / `credentials/*-updated` 的 WS 事件**——本仓没有事件订阅，只能靠上述五个触发点。补的条件——宿主上这些事件（属宿主能力，不在 GUI 侧）。**旧登记本轮复核仍在、未改**：切点空白规则只认 ASCII（`OFF:translation.ts:20` 切点是 `\s+\S*$`，本仓 `cotCutPoint` 只找 ASCII 空白）；本仓空配置仍默认全装（`internal/uiplugins/loader.go:45,52-64`），实验部件无独立启停/卸载生命周期（官方 `OFF:lib/index.js:42,127` 默认关闭）。**设置页那一半已落地**（旧账列的 D 档缺口已消）：`plugins/dssidebar/cot_translation_settings*.go`（provider/targetLanguage 两字段、缺失值保留为 disabled option + `— 不可用`、`settings.form.mutate` 带 revision fence），见 `cot_translation_settings_test.go`/`_visual_test.go`。**原实现记录（2026-11-16 清扫，历史口径）**：**2026-11-16 清扫：D+生命周期/G，差异不止 2 条**：宿主仍无 provider/targetLanguage 设置 namespace 与刷新链；正文非官方 Markdown+代码工具条、原文可带 `<signature>`、收起不取消、缓存常驻、pending 非 spinner、provider 不可用入口误隐藏、空白规则只认 ASCII。**本仓实际默认开启**：空配置全装（`internal/uiplugins/loader.go:45,52-64`），ds-conversation 无条件 append，展开思考即自动翻译（`conversation.go:672-680`、`cot_translation.go:282-298`），需要实验部件独立启停/卸载生命周期。**原实现记录（默认启停与剩余清单以上述为准）**：**官方默认关闭**（`OFF:../dsh-official-bundles/lib/index.js:42` 在 `OPTIONAL_BUNDLES`、`:127` 在 `EXPERIMENTAL_BUNDLES`；`PROFILE_TEMPLATES:4-10` 五个模板一个都没有），本仓原记录所称“默认不启用”已被本轮代码核查推翻。**本体落地**：官方 `conversation.chat.reasoning.body` 座位——`plugins/dsconversation/cot_translation.go` + `conversation.go:665-685`（展开的思考里画工具栏 + 正文两态：`查看译文`/`查看原文` · `正在翻译思考内容` · `翻译暂不可用，已显示原文` + `重试`），分段/拼接/顺序推进逐条对齐官方 `reasoningFragments` / `emit()` / `run()`（`OFF:src/client/translation.ts:20-113`）；数据面 `internal/apiclient/cot_translation.go`（`cotTranslation.limits` / `cotTranslation.translate`，两方法都是宿主**真实现**，走本机模型网关）+ `cmd/app/cot_translation_wiring.go`（limits 同步缓存 + 首问后台拉；translate 只发起、终态按 jobID 取走即清空）+ `pluginapi/plugin.go` 三条 Action；契约进 `testdata/api.snapshot.json`（batch 19）。测试：`cot_translation_test.go`（分段/代理对/UTF-16/拼接/状态机/失败/重试/入口可用性）、`cot_translation_visual_test.go`（真 Viva 绘制+点击，两尺寸、pump 到条件成立、失败与重试、宿主没接时不画）、`cmd/app/cot_translation_wiring_test.go`（真 HTTP：缓存/取走即清空/失败映射/参数形状）。**原记录剩余缺口（2 条；本轮已补查更多，见行首）**：① 官方第二处座位 `plugins.bundle.config` 的 provider/targetLanguage 表单 **HIDE**——宿主没有 `cot-translation` 命名空间的设置写入路径（`grep -rn "cot-translation" ../DSH-GO/internal ../DSH-GO/cmd` 零命中，`limits` 固定回 `deepseek-official`/`auto`）；② 官方正文走 `renderFactorySlot(conversation.chat.reasoning.content)`（Markdown + 代码块工具条），本仓原文/译文同为 `ui.Wrap` 纯文本（沿用 `reasoning.go` 那条既有刻意差异）。逐条证据、依赖表与三包考古结论：[official-experimental3-evidence-rc2.md](official-experimental3-evidence-rc2.md) |
| 64 | `dsh-experimental-client-ui-forge` | 🟡 PARTIAL | **2026-11-17 本轮（feat/pv-cot-forge）：补了 diff 双行号与 diff 底色，状态不变（仍 🟡）**。① **diff 双行号列已落地**：官方每行是四轨 `.line`（旧号 · 新号 · 符号 · 正文，`OFF:src/client/ChangesTab.module.css:604`；单侧文件 :915/:916），行号由 hunk 坐标现算，口径与 `OFF:src/client/diff-rows.ts:53-68` 的 `hunkRows()` 逐条一致（上下文两边都数、删行只数旧侧、加行只数新侧、`\ No newline at end of file` 不是一行）；列宽取 `OFF:src/client/DiffFile.tsx:194-195` 的 `calc(max(2,最大行号位数)ch + 12px)`；`singleSide` 取 `DiffFile.tsx:190`（新增=R / 删除=L）。实现 `plugins/dsforge/changes.go` 的 `changesDiffRowOf`/`drawDiffRow`/`drawLineNo`/`changesNumberW`/`changesSingleSide`。② **diff 底色与行号槽已落地**：新增 `pluginapi.Theme.FileDiffAdded{Bg,Gutter,Marker}` / `FileDiffDeleted{Bg,Gutter,Marker}`，取值逐条来自 `dsh-client-ui-theme/src/styles/design-platform.css:281-286`（亮）/`:433-438`（暗），铺法按 `ChangesTab.module.css:675-692`（`.add`/`.del` 铺 bg、`.number` 铺 gutter 且文字用 marker、`.sign` 用 marker、`.context .text` 用 label-secondary）。旧账里「无 diff 底色令牌 / 无行号列」两条差异**就此消除**。**仍为 🟡 的剩余差异（逐条写清；旧账里的 ①–⑤ 与 A 档 10 条按本轮复核重述）**：① **DiffFile 不看 split/wrap**：官方 `data-split`（左右对比，`DiffFile.tsx` 的 `splitRows()` 配对 + `.pair`/`.cell`，`ChangesTab.module.css:613-645`）与 `data-wrap`（`:657-664` 的 `pre-wrap` + `overflow-wrap:anywhere`）本仓都没有——Viva 的可变宽网格做不到「一列 max-content 横向滚动 + 另一列按宽度换行」两态并存；补的条件——Viva 提供横向可滚动的 min-content 轨道。**未变更行展开**也缺（官方 `segmentsOf()` 的 gap 可点开，`OFF:diff-rows.ts:150-199`；本仓只画「N 行未变更」提示，不可展开）。② **文件树左栏缺**：官方 `.treePane` + `--forge-tree-width`（`FileTreePane.tsx`/`file-tree.ts`），本仓右栏没有可拖宽分栏。③ **CR（变更请求）数据面仍 HIDE**：全部 CR 卡片内容与会话列表字形都没有可见实现。三条硬阻断（**本轮复核成立**）：G1 `changeRequestStates` 官方是 `mode:'stream'` 零参数帧（`OFF:lib/typert.host.js` 的 `changeRequestStates`），宿主是 unary 且只认 env；G2 `forgePanel.follow` 流的是绑定清单帧而不是官方 `ForgePanelState`（`OFF:lib/typert.host.js:1752` vs `HOST:internal/transport/handlers_forge_panel_state.go:300-345`），工作树 progress/missing 两张卡与 toggle 的 blocker 层没有读出口；G3 凭据链三套互不相通（`internal/forge` 的 env / account / relay）。**SubmitReview 三 radio（评论 / 批准 / 请求更改）本轮查实是「宿主有、GUI 缺、但缺得合理」，不伪造**：数据面**真的在**——`HOST:internal/transport/handlers_forge_write_official.go:219` 收 `request.kind=review` 且校验 `event∈{comment,approve,request-changes}`，`cmd/dshd/serve_forge_write.go:32` 已接 `forge.WriteProvider.SubmitReview`（`internal/forge/write.go:210-256`，先比 headSha、stale 即拒、`reviewFailure("unknown")` 语义齐），**但 `internal/apiclient` 没有 `forgePanel.submitReview` 客户端**（`grep -n submitReview internal/apiclient/` 零命中），而且它的入口是 CR tab 的 `ChangeRequestCard`（`OFF:ChangesTab.tsx:835` 的 `SubmitReview` 只在 `data.kind==='change'` 时渲染），而 CR 数据面本身 HIDE（G1/G2）。**所以官方那三个 radio 现在没有任何合法的可见宿主**——在本地补丁（`data.kind==='local'`）上挂一个只含批准/请求更改的假弹窗就是伪造审阅操作，不干。补的条件——G1/G2 修好后，按官方三个 radio 一比一实现（`OFF:SubmitReview.tsx:77-83`：`comment`/`approve`/`request-changes`，`own && value!=='comment'` 时禁用并显示 `review.ownChangeRequest`；`request-changes` 时 summary 必填）。④ **CommitMenu / BaseMenu 仍缺**：官方提交选择器（`CommitMenu.tsx` 186 行 + `commitMenu`）与 hero 的基准分支菜单（`BaseMenu.tsx` 180 行）都没做；本仓 hero 只画「新建工作树」勾选 + `基于 X` 文案。宿主 `forgePanel.localCommits` 与 `diffBases` 都已实现（`internal/apiclient/forge.go:272,287`），**只差 GUI**。⑤ **localText 未消费**：宿主 `forgePanel.localText` 在（`internal/apiclient/forge.go:255`），GUI 因为不支持 split/wrap 也没有「整份文件」视图，没有调用点。⑥ 其余旧账差异**本轮未动、仍然成立**：座位（`conversation.session.header.utilities` 按钮、`shell.overlay` Toast）、刷新图标、分支图标、blocker 挡板、无创建进度卡、已读只在本进程（官方 `host-viewed.ts` 走 `changeRequestViewed` + `setFileViewed`）。**原实现记录（2026-11-16 清扫，历史口径）**：**2026-11-16 清扫：D+G，旧清单需扩写**：① localDiff 已于 2026-11-17 补全：branch 走 worktree/远端默认或显式 base 的 fork-point，commits 范围真实比较；文件带官方 layer/digest/status/previousPath，untracked 有 hunks/digest 且 omitted 按 maxUntrackedFiles=100 计算，已读“已变更”恢复 digest 判据；BaseMenu/CommitMenu 仍属④未做；② follow/changeRequestStates 形状与状态来源仍不符，但 REST/GraphQL/写入已接真实 HTTP，旧“provider 占位”删除，凭据整合仍分裂；③ 漏登 `plugins.row.config` 的 `retainOnArchive` 设置页，宿主无 worktree 设置 namespace；④ CR 主体、CommitMenu、localText、未变更行、文件树、split/wrap、基准分支菜单、journal 恢复、guide 入口等大片 GUI 未做；⑤ 本仓空配置同样默认全装 ds-forge，并非默认不启用。逐条见 [清扫报告](official-partial-sweep-2026-11-16.md)。**原实现/差异考古**：**A 档 14 方法已做，CR 相关因 G1/G2/G3 HIDE**（官方默认关闭：`OFF:dsh-official-bundles/lib/index.js:48` / `:125`）。**做出来的三块**：`plugins/dsforge`（`conversation.hero.options` 工作树勾选 / `conversation.input.dock` 本地变更卡 / `sidebar.right.pane.tab` 的 Changes 视图本地那一半）+ `internal/apiclient/forge.go`（`branches`/`localDiff`/`localText`/`requestWorktree`/`cancelWorktree`/`retryWorktree`/`stopCreation`/`restoreWorktree`/`continueInProject`/`track`/`dismiss`/`diffBases`/`localCommits`/`worktreeCleanup` 共 14 条，快照 batch 21）+ `cmd/app/forge_wiring.go`（缓存/异步读/动作/会话世代判定）。数据源全是宿主真实现：`HOST:cmd/dshd/serve_forge_git.go:54/62/74`、`serve_worktree_api.go:173`、`serve_forge_state.go`、`serve_forge_rc2.go:235/257/330`。**HIDE（#64b）**：全部 CR 卡片内容与会话列表字形（G1：官方 `changeRequestStates` 是 `mode:'stream'` 零参数帧，宿主走 unary + 只认 env；G2：早期记录称 provider 占位；本轮核实已接真实 HTTP，剩余是流形状/状态来源/凭据整合；G3：凭据链三套互不相通）、`gh auth login` 横幅，外加**本轮新查出的 G6**：`forgePanel.follow` 流的是绑定清单帧而不是官方 `ForgePanelState`（`OFF:lib/typert.host.js:1752` vs `HOST:internal/transport/handlers_forge_panel_state.go:300-345`），所以工作树 progress/missing 两张卡与 toggle 的 blocker 层没有读出口，一并 HIDE。**剩余差异 10 条**（座位、范围菜单位置、无文件树/左右对比、无 diff 底色令牌、刷新图标、分支图标、blocker 挡板、无创建进度、已读只在本进程）逐条见 [official-forge-a-evidence-rc2.md](official-forge-a-evidence-rc2.md) §5-§6。 |
| 65 | `dsh-experimental-client-ui-stats` | ✅ IMPLEMENTED | **`plugins/dsstatusbar/stats_view.go`（官方 `conversation.view` `order:40` 的 `stats` 页面本体：逐 turn/step 11 列 + 总用时条 + 全量/窗口合计；帧投影对齐官方 `stats-fold.ts` / `stats-usage.ts` / `stats-format.ts`）** + `plugins/dsstatusbar/statusbar.go:129-130` 注册；证据 [official-attachment-stats-evidence-rc2.md](official-attachment-stats-evidence-rc2.md)。**剩余差异**：compaction/非 step 事件行、三个工具栏开关、加载更早分页未做 |
| 66 | `dsh-experimental-client-ui-voice-input` | ❌ MISSING | **官方默认关闭**（`OFF:dsh-official-bundles/lib/index.js:41` / `:98` / `:122`）。**HIDE + 登记缺什么**：宿主这半边**是真的**——`HOST:cmd/dshd/serve_speech.go:5-60` 的 `newSpeechAPI()` 接 `internal/speech`（受管 C# helper 跑 Windows System.Speech），6 方法全实现、路由 `HOST:internal/transport/handlers_speech_official.go:86-93`（`handlers_media_market_official.go` 里那 4 条旧 501 已被这个分支取代）。**缺的是客户端采集三样**：① 麦克风枚举与录音（官方 `OFF:src/client/audio.ts:59-63` `getUserMedia` + `:43,95` `MediaRecorder`；本仓 Go+Viva 没有 DOM/WebAudio，`grep -ln "WASAPI\|waveIn\|Microphone" cmd internal plugins` 零命中，基线 `CGO_ENABLED=0`）；② 重采样到 16 kHz 单声道（官方 `:152` `OfflineAudioContext(1,…,16000)`）；③ WAV 编码 + base64（`speech/transcribe` 的入参）。录音按钮 / 波形 / 麦克风选择 / 设置对话框全依赖这三样，一样都没有就画不出真东西。证据：[official-experimental3-evidence-rc2.md](official-experimental3-evidence-rc2.md) §4**本轮查证补充（2026-11-15，结论仍 HIDE 但依据换成实证）**：① **viva 无音频能力**：`app/app.go:126-160` 的 `Config` 事件钩子只有 `OnPointer/OnKey/OnShortcut/OnModifierTap/OnScroll/OnFileDrop` 六个，**无音频**；`grep -rniE "audio|microphone" viva/app` 命中全是无关（`recordDrive` 是外观录制、`shortcutCapturer` 是快捷键）。② **CI 强制 `CGO_ENABLED=0`**（`.github/workflows/ci.yml:70` 注明「本仓刻意不依赖 C 工具链」）→ 不能直接 cgo 调 WASAPI。③ **但有先例**：宿主 `internal/speech/engine.go:1-10` 为同类问题（COM 在 Go exec 管道下挂起，t108 实测 60s 不返回）选了**受管 C# helper exe**（超时+取消+kill+退出等待）。**同一条路可用于音频采集**：独立 helper 做 WASAPI 采集 + 16kHz 重采样 + WAV+base64，GUI 侧只发一次请求拿结果。**判定：可行，但需要新增一个组件（helper 源码+编译+管理+测试），为账本 1 条条件不划算** —— 保留 ❌ 并在此写下实现路径，将来若 voice-input 从「官方默认关闭的实验包」升为正式能力再按此路径做。 |

> #39 / #45 / #46 / #47 四行的**页面落点、逐字文案、表单状态机、几何台账与差异**、以及
> #46 的模型选择落地与仍存宿主缺口、#47 的三命名空间/密钥路径与缺口：[official-settings-pages-evidence-rc2.md](official-settings-pages-evidence-rc2.md)。
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
> `#59` 已于 2026-11-16 补齐完整账本；`#54` 仍差页面本体。

## 合计：官方 66 个部件 —— ✅ 52 · 🟡 12 · ⬜ 0（不该有） · ❌ 2

> 2026-11-16 原 15 条 PARTIAL 的逐条证据与分类：[official-partial-sweep-2026-11-16.md](official-partial-sweep-2026-11-16.md)。

> 剩下 1 条 MISSING（`#19 file-drag`、`#66 voice-input`）的**逐条核查结论、精确阻断点与下一批排序**（A 档"宿主已在只差界面" /
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
