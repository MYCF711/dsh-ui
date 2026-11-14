# 官方 0.2.2-rc.2 · 19 条 MISSING UI 包的结构证据表

> 建立时间：2026-11-14 · 对应 `docs/official-parity.md` 里 ❌ MISSING 的 19 行。
>
> **这份文件回答一个问题**：这 19 个官方包，我们**有没有可照做的结构证据**？
>
> 结论先说：**19/19 都有**。但不是从主库 `structure/` 来的 —— 主库对 66 包的覆盖是
> DEEP 8 / MENTION 25 / NONE 33（`$DSH_HOME/dsh-ui-official/structure/official-ui-packs-coverage-rc2.md:15-20`），
> 这 19 条在其中的分布是 **DEEP 0 / MENTION 4 / NONE 15**。
>
> 证据来自**另一处更硬的地方**：rc.2 起官方**随包发 `src/**` 原件**（不再是 rc.1 那种压缩 bundle）。
> 解包树 `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/<包>/src/**` 里是
> 未压缩的 TS/TSX + CSS Module，**组件层级、slot 名字、几何值、文案全部可读、可锚行号**。
> 这是比任何二手结构文档都强的一手证据。

---

## 0. 证据口径（三档，与主库同判据）

| 档 | 判据 | 本表用法 |
|---|---|---|
| **DEEP-EQ** | 官方 `src/**` 里能读出**该包自己的**渲染实现（组件层级 / slot 注册面 / CSS 几何 / locale 文案之一以上），且带 `file:line` | **可照做** |
| **MENTION-EQ** | 只有 slot 声明，渲染是 renderless（等别人填），或数据全靠外部服务 | **不足以单独成面** |
| **NONE-EQ** | `src/**` 不存在或为空 | 本表无此档 |

**注意 `DEEP-EQ` 不等于「该做」**：能不能真做还要过第二道门 ——
**真实数据路径**（官方 src 读的每个数据源，要么落在真宿主已实现的方法上，要么落在本窗口真实状态上）。
本表的「判定」列就是这两道门一起过的结果，判据写在每条的「差什么」里。

**锚点记法**：`OFF:<包简称>/<src 内相对路径>:<行号>`。
`OFF` 根 = `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`。
`HOST` 根 = `tmp/GO8-RC2/_audit-tree/`（宿主仓 `origin/main` 的干净导出，`55bfa61`）。

---

## 1. 总表（19 条，顺序与 `docs/official-parity.md` 对账表一致）

| # | 官方包 | src 文件/行 | slot 面 | 数据来源 | 宿主真有？ | GUI 有槽？ | 判定 |
|---:|---|---:|---|---|---|---|---|
| 2 | `action-cards` | 9 / 476 | `conversation.chat.turnTail`、`tool.call.toolview` | `actionCards.view`/`.act` | **是** | 否 | **本仓已实现** |
| 10 | `context-menu` | 11 / 749 | `contextMenu.button`(factory)、`contextMenu.content`/`.icon`(keyed)、`shell.overlay` | 无（纯 UI 基础设施） | n/a | 仅 `shell.overlay` | **HIDE** |
| 12 | `cordis` | 18 / 2833 | `sidebar.footer.action`、`tool.call.toolview` | `dynamicCordisRunner.*` ×12 | **是** | 仅 footer | **HIDE** |
| 13 | `creation-recommendations` | 7 / 631 | 无自有槽（注册进 `actionCards`） | `ctx.actionCards` | **否**（传递） | 否 | **HIDE** |
| 15 | `directory-picker-browse` | 5 / 1745 | `conversation.hero.workspace.directoryFlow`、`sidebar.workspaces.directoryFlow` | `directoryPicker.list/createDirectory` | **是** | 自有模态 | **本仓已实现** |
| 16 | `directory-picker-native` | 3 / 117 | 同上一对洞 | `directoryPicker.pick` / Viva `app.OpenDirectory` | **是** | 无 | **HIDE** |
| 17 | `entry-customization` | 5 / 258 | `sidebar.panellist.actions`、`conversation.session.header.views.actions`、`sidebar.right.tab.guide.actions` | `uiWorkspace.startSession`、`pluginNavigation` | **否** | 三个槽都在 | **HIDE** |
| 18 | `file-actions` | 6 / 424 | `contextMenu.icon` | `workspaceFiles.*`、`session.openWorkspacePath` | **是** | 否（依赖 #10） | **HIDE** |
| 19 | `file-drag` | 5 / 274 | `shell.overlay` | 无（拖出侧，需 OS 拖源） | n/a | 是 | **HIDE** |
| 28 | `plan` | 14 / 904 | `conversation.input.plan`、`conversation.chat.turnTail`、`conversation.plan-review.actions`、`sidebar.right.pane.tab(+.title)` | `session.page`/`.commands`/`.events` | **是** | 2/5 | **可做（受限）** |
| 30 | `plugin-market` | 8 / 2482 | `plugins.tab` | `pluginMarket.*`、`pluginRegistryProbe` | **是** | 否 | **HIDE** |
| 31 | `plugin-recommendations` | 6 / 472 | 无自有槽（注册进 `actionCards`） | `pluginRecommendations.describe` | **是**（目录空） | 否 | **HIDE** |
| 39 | `settings-agent-loop` | 5 / 310 | `plugins.item` | `agent-loop` 命名空间 | **否** | 否 | **HIDE** |
| 45 | `settings-shell` | 5 / 348 | `plugins.item` | `bash-sandbox`/`pwsh-sandbox` 命名空间 | **否** | 否 | **HIDE** |
| 46 | `settings-subagent` | 12 / 1376 | `plugins.item` | `subagent`+`subagent-model-selection-settings` 命名空间 | **否** | 否 | **HIDE** |
| 47 | `settings-web-search` | 6 / 1602 | `plugins.item` | `web`/`web-deepseek-api`/`web-search-deepseek` 命名空间 + `web.listProviders` + `credentials.*` | **部分** | 否 | **HIDE** |
| 63 | `cot-translation` | 11 / 781 | `conversation.chat.reasoning.body`、`plugins.bundle.config` | `cotTranslation.translate`/`.limits` | **是** | 否 | **可做** |
| 64 | `forge` | 85 / 20583 | 11 个槽（见 §2） | `forgePanel.*` ×31 | **否**（15 个 501） | 多数无 | **HIDE** |
| 66 | `voice-input` | 15 / 1826 | `conversation.input.activity`、`plugins.bundle.activation/config`、`dsh.voice-input.microphone` | `speech.*` | **是** | 仅 activity | **HIDE**（缺客户端录音） |

**分堆计数（任务要求的第一刀）**

| 堆 | 条数 | 依据 |
|---|---:|---|
| 主库 `structure/` 判 **DEEP** | **0** | 主库 DEEP 只有 8 包（conversation/chat/layout/sidebar-right/schedule/theme/account-notifications/agent-team），与本表 19 条零交集 |
| 主库判 **MENTION** | **4** | `settings-agent-loop`、`settings-shell`、`settings-subagent`、`settings-web-search`（`official-ui-packs-coverage-rc2.md:48` 一行列出、只有 slot/行位置） |
| 主库判 **NONE** | **15** | 其余 15 条在 `:62-68` 的 NONE 清单里 |
| **本轮补到的结构证据** | **19/19** | 官方 `src/**` 原件，见 §2 逐条 |

---

## 2. 逐条证据（可照做的部分：组件/几何/文案锚点）

> 每条只记「能照做」的关键锚点；完整文件清单见每条的 `src` 文件数/行数。
> **文案一律列出中文字典原文**（来自各包 `src/client/locales.ts`），实现时逐字使用。

### #2 `action-cards`（9 文件 / 476 行）— **可做**
- 自有槽：`conversation.chat.turnTail`（官方声明 `OFF:ui-conversation/src/client/contract/slots.ts:492`
  逐字：`{ kind: 'list'; scope: 'session'; owner: TurnTailOwnerProps }`）、`tool.call.toolview`
- 组件：`OFF:action-cards/src/client/Card.tsx:87-150`（卡体 + `propose_action` 行）、
  `ProposeRow.tsx:26-43`（提议行四态）、`registry.ts:1-106`、`mount.ts:1-63`
- 文案键：`row.preparing/running/ok/stopped/error/title/inspect`（`OFF:action-cards/src/client/locales.ts:1-82`）
- 数据：`actionCards.view` + `actionCards.act`
- **宿主真有**（**这条我最初判错了，此处按代码更正**）：
  - `HOST:cmd/dshd/serve.go:926` `accountAPI.ActionCardAct = actionCardAct(*home)`
  - `HOST:cmd/dshd/serve.go:941` `attachActionCardsView(srv, *home)` —— 在产品启动路径上
  - `HOST:cmd/dshd/serve_action_cards.go:55-57` `srv.AttachActionCardsView(actioncards.Open(home).ViewMap)`
  - `HOST:internal/actioncards/`：`service.go:217`（`View`）、`:226`（`ViewMap`）、`:238`（`Act`）、
    `:214`（`Tool() spi.Tool` → **`propose_action` 工具真的注册了**），另有 `projection.go`/`registry.go`/`store.go`/`tool.go`
  - 状态语义 `HOST:internal/actioncards/service.go:94-107`：`accepted` / `settled` / `refused` / `unconfirmed`
- **⚠ 两处会把人带偏的过期注释**（本条差点因此被误判 HIDE）：
  `HOST:internal/transport/handlers_account_official.go:1478` 与
  `HOST:internal/transport/handlers_session_official.go:899-900` 都写着"本仓没有 action card 运行时"。
  那是**`nil` 时的 fallback 错误文案**，不是当前产品状态。**判据一律看挂载点，不看注释。**
- **差什么**：GUI 缺这两个槽 + `actionCards.view`/`.act` 的客户端通道。实现落点：`plugins/dsactioncards`（本轮新增）

### #10 `context-menu`（11 文件 / 749 行）— **HIDE**
- 它是**纯 UI 基础设施**，不是独立可见面：`apply()` 提供 `contextMenus` 服务 + 注册一枚
  `shell.overlay` 的 `MenuOverlay`（`OFF:context-menu/src/client/index.ts:180-188`）。
- 工厂槽 `contextMenu.button` 的语义（`OFF:context-menu/src/client/index.ts:37-40`）：
  > 「Ellipsis 按钮**仅当它的目标 kind 有已注册贡献者时才显示**」
  即**没有消费方就没有可见节点** —— 这正是「空槽不占位」。
- 官方消费方（`official-ui-packs-coverage-rc2.md:127`）：`ui-conversation` 的
  `ConversationSession.tsx:211`、`ui-workspace` 的 `Rows.tsx:325,752`。
- **差什么**：GUI 里没有任何插件注册 `contextMenu.button` 的贡献；且官方那两个消费方
  （会话行右键、工作区行右键）在**主库无结构证据**（`ui-workspace` 是 MENTION，`ui-conversation`
  虽有 DEEP 文档但未覆盖右键菜单面）。→ 做出来是**渲染不出来的基础设施**，且消费方几何要自造。
- **怎么补**：先给 `ui-conversation` / `ui-workspace` 的右键菜单面取证（哪里挂 `contextMenu.button`、
  菜单项几何），再实现 `contextMenus` 服务 + 至少一个真实消费方。

### #12 `cordis`（18 文件 / 2833 行）— **HIDE**
- 自有槽：`sidebar.footer.action`（GUI 已有 `uikernel.OffSidebarFooterActions`）、`tool.call.toolview`（GUI 无）
- 组件：`OFF:cordis/src/client/CordisPanel.tsx:1-507`、`CordisDefineRow.tsx`、几何
  `OFF:cordis/src/client/CordisPanel.module.css:1-366`、`CordisDefineRow.module.css:1-309`
- 文案：`OFF:cordis/src/client/locales.ts:1-453`（动作 `action.approve/approveOnce/approvePlugin/decline/inspect/remove/retry/rollback/run/stop`，正文 `body.clientCode/copy/copied/hostCode`）
- 数据：`dynamicCordisRunner.*` 12 方法
- **宿主真有**：`HOST:cmd/dshd/serve_cordis_api.go:3-8` 原文「PARITY-CORDIS-C3B: dynamicCordisRunner 12 方法的真实宿主实现 … 没有 501、没有静态成功」
  （注意：`HOST:internal/transport/handlers_media_market_official.go:37-40` 里那句「11 个未实现、按 R6 不做」是 **rc.1 时期的旧注释**，
  已被 `serve_cordis_api.go` 的实现取代——**别被它带偏**）
- **差什么（比"宿主有 12 个方法"更要紧的一层）**：`dynamicCordisRunner.*` 的 12 个方法确实有真实现，
  **但产生卡片的那四个工具在宿主里根本不存在** —— `cordis_define` / `cordis_run` / `cordis_stop` /
  `cordis_undefine` 在 `HOST` 全仓 Go 里 `grep` **0 命中**，且 `serve_cordis_api.go` 的 `RegisterGo`
  在 `HOST:cmd/dshd` 里**没有生产调用者**。→ `inventory` 在生产中恒空，面板画出来就是空表。
  另外业务视图要求在客户端执行 Package 的 JS，与红线 R6（宿主不嵌 JS 运行时）冲突。
  GUI 还缺 `tool.call.toolview` 槽 + 12 个客户端通道。
- **怎么补**：宿主先落那四个 cordis 工具并真的注册到工具面；之后本仓补槽与通道。

### #13 `creation-recommendations`（7 文件 / 631 行）— **HIDE**
- **无自有槽**：`mount.ts` 只做 `ctx.actionCards.register({ action: 'create_personalized_page', Body })`
  （`OFF:creation-recommendations/src/client/mount.ts:24-27`），即它是 **`ui-action-cards` 的一个 body 贡献者**。
- 组件：`OFF:creation-recommendations/src/client/Card.tsx:1-159`、`Card.module.css:1-28`；文案 `locales.ts:1-402`
- **差什么**：**传递依赖**——它没有 `actionCards` 服务就无处注册（#2 已判 HIDE）。
- **怎么补**：随 #2 一起；宿主先有 action-card 运行时。

### #15 `directory-picker-browse`（5 文件 / 1745 行）— **本仓已实现**
- 自有槽：`conversation.hero.workspace.directoryFlow`、`sidebar.workspaces.directoryFlow`（`OFF:.../src/client/index.ts:60-66`）
  —— 两个洞**都由 `ui-workspace` 声明**，本包只做填充者。
- 组件：`OFF:.../src/client/DirectoryBrowser.tsx:1-1052`（Miller 双列 + 面包屑 + 新建文件夹子对话框）
- 几何（逐值来自 `OFF:.../src/client/DirectoryBrowser.module.css`）：
  - `.dialog.dialog`：`width min(680px,100%)`、`height min(500px, calc(100dvh - 32px))`（`:6-11`）
  - `.header`：`padding 16px 14px 8px 24px`、`gap 8`、下边框 `.5px`（`:27-33`）
  - `.title`：`font-size 16 / line-height 24 / weight 510 / min-height 28`（`:35-43`）
  - `.crumb`：`font-size 13 / line-height 20 / weight 500 / max-width 160`（`:160-172`）
  - `.content`：`padding 16px 16px 16px 24px`（`:212-221`）；`.column`：`min-width 256 / gap 2 / padding-right 8`（`:226-235`）
  - `.row`：`height 28 / gap 4 / padding 4`（`:262-274`）；`.footerBar`：`padding 16px 24px`、上边框 `.5px`（`:338-345`）
  - 新建文件夹子对话框：`width min(380px,100%)`、`.createBody padding 22px 24px 20px`、`.createInput height 44`（`:404-441`）
- 文案（zh，`OFF:.../src/client/index.ts` 的字典）：`browser.title` 选择工作区目录 · `browser.home` 主目录 ·
  `browser.newFolder` 新建文件夹 · `browser.folderName` 文件夹名称 · `browser.createIn` 在"{name}"中新建文件夹 ·
  `browser.untitledFolder` 未命名文件夹 · `browser.create` 创建 · `browser.cancel` 取消 · `browser.open` 打开 ·
  `browser.editPath` 编辑路径 · `browser.loading` 加载中… · `browser.truncated` 文件夹过多，仅显示开头部分。 · `browser.showHidden` 显示隐藏文件
- **宿主真有**：`directoryPicker.list` / `directoryPicker.createDirectory`
  handler `HOST:internal/transport/handlers_workspace_official.go:1282`、`:1316`；
  真实现 `HOST:cmd/dshd/serve_workspace_api.go:1668`（`pickerList`，真 `os.ReadDir`）、`:611-612`（挂载 `List`/`CreateDirectory`）；
  结果形状 `HOST:internal/transport/handlers_workspace_official.go:286-299`
- **注意**：`workspace.create`（真实现 `HOST:cmd/dshd/serve_workspace_api.go:583`）与 GUI 现有的
  `workspace.list` **不是同一个东西**——GUI 的 `workspace.list` 是本仓自定契约（列目录条目，
  `internal/apiclient/fs.go:8-12` 有说明），官方的工作区注册表是 `workspace.registry`
  （`HOST:internal/transport/handlers_workspace_registry.go:21`）。**别混用**。
- 实现落点：`plugins/dsdirpicker`（本轮新增）

### #16 `directory-picker-native`（3 文件 / 117 行）— **HIDE**
- **renderless**：`NativeDirectoryFlow` 返回 `null`（`OFF:directory-picker-native/src/client/flow.ts:27` 的
  `export function NativeDirectoryFlow(...): ReactElement | null`，组件体只有 `useEffect` + `return null`）。
  它唯一的作用是往 #15 那两个洞**注入一个 `pick()` 实现**（`OFF:.../src/client/index.ts:32-37`）。
- **差什么**：官方是靠「装哪个包」来决定 directory-flow 用哪个 backend。本仓插件系统没有这种
  「同一洞的二选一 backend」机制；装上 browse 之后 native 没有可见落点。
  **给它硬造一个入口（比如按钮旁边加个下拉二选一）就是自造交互**。
- **怎么补**：先在 GUI 上做出官方 `ui-workspace` 的 directory-flow backend 选择面（需 `ui-workspace`
  结构证据，主库现为 MENTION），再把 `app.OpenDirectory`（Viva v0.6.0 `app/system.go:180`
  `func OpenDirectory(OpenDirectoryOptions) (string, error)`）接成第二个 backend。
  **能力本身已经在了**（`HOST:cmd/dshd/serve_native_api.go:58` 挂了 `PickDirectory`，Windows 走 `IFileOpenDialog`），缺的是结构证据。

### #17 `entry-customization`（5 文件 / 258 行）— **HIDE**
- 自有槽（三个都注册同一份菜单行）：`sidebar.panellist.actions`、`conversation.session.header.views.actions`、
  `sidebar.right.tab.guide.actions`（`OFF:entry-customization/src/client/index.ts:58-72`）。**这三个槽 GUI 都在册。**
- 组件：`OFF:entry-customization/src/client/EntryMenuContent.tsx:1-71`；几何 `EntryMenu.module.css:1-32`
- 文案（zh）：`pinTooltip` 固定到左侧栏 · `pin` 将{name}固定到侧边栏 · `install` 浏览更多插件 ·
  `create` 创建你的插件 · `createPrompt.sidebar/conversation/rightbar` 三段创作提示（`locales.ts:16-25`）
- **差什么**（两条都致命）：
  1. 它填的是三个 **More 菜单**（"…"按钮弹的菜单）。GUI 里这三个槽只是座位，
     **没有对应的 More 菜单容器**（官方容器在 `ui-sidebar` / `ui-conversation` / `ui-sidebar-right`，主库无该面几何证据）。
  2. `install` 行依赖 `pluginNavigation` 服务（`index.ts:50-56`），宿主 `pluginInventory.list`
     返回 `officialNotImplemented`（`HOST:internal/transport/handlers_plugin_inventory_official.go:35` 附近）；
     `create` 行依赖 `uiWorkspace.startSession({agentPreset:'cordis', …})`，GUI 无 agentPreset 通道。
- **怎么补**：先取证三个 More 菜单容器的几何（或由宿主补 `pluginInventory.list`）。

### #18 `file-actions`（6 文件 / 424 行）— **HIDE**
- 只注册 `contextMenu.icon`（`OFF:file-actions/src/client/index.ts`），即它是 #10 的**消费方**
- 组件：`OFF:file-actions/src/client/context-menus.tsx:1-108`、`files.ts:1-96`；文案 `locales.ts:1-132`
  （`menu.copyImage/download/saveAs/reveal/finder/copied/downloadStarted/downloaded/saved/copyFailed/saveFailed`、
  `path.revealError`）
- 数据：`remote.workspaceFiles`、`remote.session`、`path.reveal*`
- **宿主真有**：`workspaceFiles.list/read/readBytes/stat/changes` 全挂（`HOST:cmd/dshd/serve_workspace_api.go:594-597`）、
  `session.openWorkspacePath`（`HOST:internal/transport/handlers_session_official.go:71`）
- **差什么**：**依赖 #10**。没有 `contextMenus` 服务就没有菜单可注册进。
- **怎么补**：随 #10。

### #19 `file-drag`（5 文件 / 274 行）— **HIDE**（方向判错过一次，此处按代码更正）
- 自有槽：`shell.overlay`（GUI `pluginapi.SlotShellOverlay` 已有）
- **它到底是什么**（读 `src` 才看得清，**别按包名猜成"接收拖入"**）：
  主体是**拖出侧**（drag source）服务 —— `ctx.provide('fileDrag', {actions:{prepare,reportFailure}, read})`
  （`OFF:file-drag/src/client/index.ts:42-46`）；`prepare(source, signal)` 把应用内文件
  准备成一次可拖到 **OS/其它应用**的拖拽（`OFF:file-drag/src/client/transfers.ts:52-90`，
  走 `desktop.prepare(source, requestId)` → `FileDragHandle`）；`read(transfer)` 只是在
  拖回本应用时按 token 认回自己那份字节（`transfers.ts:24-30`）。
  唯一的可见面是**一枚失败 Toast**（`OFF:file-drag/src/client/feedback.tsx:30-32`）。
  **官方没有拖放高亮遮罩。**
- 文案（zh，`OFF:file-drag/src/client/locales.ts:2`）：`failed` = `拖放失败，请重试`
- **差什么（客户端能力边界，不是宿主欠账）**：Viva v0.6.0 只有**接收**侧 ——
  `WM_DROPFILES` / `DragAcceptFiles`（`viva@v0.6.0/internal/platform/windows/drop_windows.go:18-40`）、
  `app.Config.OnFileDrop`（`viva@v0.6.0/app/app.go:150-156`）、`gfx.FileDropEvent.Paths`（`gfx/input.go:262-274`）；
  **没有任何发起拖拽的能力**（全树无 `DoDragDrop` / `IDropSource`）。
  而官方这个包要的正是"发起拖出"。→ 接收侧能力**不能替代**它。
- **怎么补**：Viva 侧需先有 OS 拖源（`IDropSource`/`DoDragDrop`），或按上游 issue 路径登记。
  按本仓规矩不得在插件里拼一个假拖出。
- **顺带说明**：官方**"接收拖入"**的包不是这个，是 `ui-attachment` ——
  它有 `OFF:ui-attachment/src/client/drop-events.ts`（`DataTransfer` + `onDrop`）。
  那条属于 `docs/official-parity.md` 里 #6 `attachment` 的 PARTIAL 缺口，不属本表。

### #28 `plan`（14 文件 / 904 行）— **可做（受限）**
- 自有槽：`conversation.input.plan`（GUI 有 `OffInputPlan`）、`conversation.chat.turnTail`（**GUI 无**）、
  `conversation.plan-review.actions`（**GUI 无**）、`sidebar.right.pane.tab` + `.title`（GUI 有）
- 组件：`OFF:plan/src/client/PlanCard.tsx:1-83`、`PlanModeControl.tsx`、`PlanPreview.tsx`、
  几何 `PlanModeControl.module.css:1-82`；文案 `locales.ts:1-164`
  （`chip.label/on.title/on.aria/exitFailed`、`preview.*` 全族、`markdown.footnotes`、`codeBlock.*`）
- 数据：`remote.page`、`remote.commands`、`remote.events`、`remote.session`
- **宿主真有**：`session.page` 走 `HOST:cmd/dshd/serve_official_wire.go:173-178`（L1 `session.page`，与
  `HOST:internal/transport/handlers_session_official.go:384-390` 同一套字段名）、`commands.list/execute`
  （`HOST:internal/transport/handlers_agent_official.go:41-42`）
- **差什么**：两个 GUI 没有的槽（`turnTail` 要挂在 dsconversation 的轮次尾部；`plan-review.actions` 要在审批条上）。

### #30 `plugin-market`（8 文件 / 2482 行）— **HIDE**
- 自有槽：`plugins.tab`（GUI 无）
- 组件：`OFF:plugin-market/src/client/DiscoverSection.tsx:1-644`、`DiscoverSection.module.css:1-451`、`market-store.ts:1-415`
- 文案：`locales.ts:1-748`
- 数据：`remote.pluginMarket` ×3、`remote.pluginRegistryProbe`
- **差什么（宿主其实是好的，缺的是 UI 容器 + 索引内容）**：`pluginMarket.*` 四方法与 `pluginRegistryProbe`
  在宿主**全是真实现** —— `HOST:cmd/dshd/serve_plugin_market.go:53`（`newPluginMarketAPI`）、
  `:67`（`ListIndexes`）、`:83`（`SetIndexes`）、`:107`（`Search`）、`:234`（`ReadEntryDetail`）。
  ⚠ `HOST:internal/transport/handlers_media_market_official.go:30-34` 那段「pluginMarket 未实现」是 **rc.1 的旧注释**，
  下一轮不要再被它误导。
  真正卡住的是两条：① **索引源默认空** —— `HOST:internal/market/store.go:90-104` 从
  `<home>/state/plugin-market-indexes.json` 读，文件不存在时 URLs 为空，`Search` 无源可查；
  ② GUI 缺 `plugins.tab` 的 owner 与 `install/update/openPackage` 三个回调 —— 它们在
  `dsh-client-ui-plugin-manager` 里（`OFF:ui-plugin-manager/src/client/slot-contract.ts:105-124`），
  而那个包在本仓是 PARTIAL 静态壳。
- **怎么补**：先补 plugin-manager 的 `plugins.tab` owner 与三个回调（属 #29 PARTIAL 的同一条链），
  索引源由用户在设置里配；两者都在了这张卡才有内容。

### #31 `plugin-recommendations`（6 文件 / 472 行）— **HIDE**
- **无自有槽**：`mount.ts` 注册进 `actionCards`（同 #13 的形态）
- 数据：`pluginRecommendations.describe`
- **差什么**：① 传递依赖 `actionCards` 的卡体座位（#2，本仓已在做）；② `describe` 在宿主**是真的**
  （`HOST:internal/recommendations/service.go:120`，接线 `HOST:cmd/dshd/serve.go:902-905`），
  **但它的目录来自市场索引**：索引源为空时恒返回 `refused{unavailable}`
  （`HOST:internal/recommendations/service.go:153-156,181-208`）。→ 卡片只能显示"不可用" = 死卡。
  （`HOST:internal/transport/handlers_media_market_official.go:854` 那句"未接线"同样是旧注释。）
- **怎么补**：随 #30 的市场索引源；以及 #2 的卡体座位。

### #39 / #45 / #46 / #47 四个 `settings-*`（合计 28 文件 / 3636 行）— **全部 HIDE**
- **四个的共同形态**（这是最容易误判的地方）：它们**都不注册 `settings.section`**，而是注册
  **`plugins.item`**（插件详情页的一个配置卡片），数据走各自的**设置命名空间**：
  | 包 | 槽 | 真实命名空间（**逐字**） | 额外数据源 |
  |---|---|---|---|
  | `settings-agent-loop` | `plugins.item` | `agent-loop` | — |
  | `settings-shell` | `plugins.item` | **`bash-sandbox` / `pwsh-sandbox`**（不是 `shell`！） | — |
  | `settings-subagent` | `plugins.item` | `subagent` + `subagent-model-selection-settings` | `remote.session` |
  | `settings-web-search` | `plugins.item` | `web` / `web-deepseek-api` / `web-search-deepseek` | `remote.web`、`remote.credentials` |

  > 命名空间名逐字取自各包 controller：`agent-loop-card-controller.ts:13,44`、`shell-card-controller.ts:10,12`、
  > `subagent-model-selection-card-controller.ts`、`web-search-card-controller.ts`。
  > **`settings-shell` 是个坑**：包名带 `shell`，但它读的是**沙箱**命名空间，不是某个叫 `shell` 的命名空间。
- 组件与几何（可照做，留档备用）：
  `OFF:settings-agent-loop/src/client/AgentLoopCard.tsx:1-41` + `agent-loop-card-controller.ts:1-63`、文案 `locales.ts:1-146`；
  `OFF:settings-shell/src/client/ShellCard.tsx:1-55` + `shell-card-controller.ts:1-67`、文案 `locales.ts:1-163`；
  `OFF:settings-subagent/src/client/SubagentModelSelectionFields.tsx:1-128` + `.module.css:1-138` +
  `subagent-model-selection-card-controller.ts:1-356`、文案 `locales.ts:1-343`；
  `OFF:settings-web-search/src/client/WebSearchCard.tsx:1-294` + `.module.css:1-150` +
  `web-search-card-controller.ts:1-669`、文案 `locales.ts:1-403`
- **差什么（硬判据，四个共用）**：宿主 `settings.describe` 只服务 **6 个** UI 命名空间 ——
  `HOST:internal/settingsform/client_ui_schema.go:55-67` 原文只有 `locale` / `ui-theme` / `ui-chat` /
  `ui-settings` / `ui-conversation` / `session-log-deepseek`；另有 `llm`
  （`HOST:internal/settingsform/provider_schema.go:48`）与 `permission`（`HOST:internal/settingsform/settingsform.go:14`），
  合计 8 个。**这四个包要的 `agent-loop` / `bash-sandbox`+`pwsh-sandbox` / `subagent`+`subagent-model-selection-settings` /
  `web`+`web-deepseek-api`+`web-search-deepseek`，在宿主全仓 Go 里零命中。** → 页内无处取数，值恒空。
  另外 `settings.*` 只有 6 个方法（describe/mutate/form.*），**没有"按命名空间读任意配置"的后门**，绕不过去。
- **#47 的例外与边界**：它的两个**辅助**数据源在宿主是真的 —— `web.listProviders`
  （`HOST:internal/transport/handlers_web_providers_rc2.go:35`，挂载入口 `:138-141`）、
  `credentials.describe/set/unset`（`HOST:internal/transport/handlers_settings_official.go:57`）。
  但**主数据源 `settings.webSearch` 命名空间不存在**，而卡片主体（启用开关、maxUses、baseURL）
  全在那个命名空间里。→ **辅助源齐、主源缺 = 仍然 HIDE**，不能只画半张卡。
- **怎么补**：宿主在 `client_ui_schema.go` 补这四个命名空间（并让 `settings.describe` 报出来），
  然后本仓补 `plugins.item` 槽 + 四个卡片。**补的是一处宿主契约，不是四处 UI。**

### #63 `cot-translation`（11 文件 / 781 行）— **可做**
- 自有槽：`conversation.chat.reasoning.body`（**GUI 无**）、`plugins.bundle.config`（GUI 无）
- 组件：`OFF:cot-translation/src/client/translation.ts:1-114`、`TranslationSettings.tsx:1-67`、`mount.ts:1-129`
- 文案：`OFF:cot-translation/src/client/locales.ts:1-179`
- 数据：`cotTranslation.translate`、`.limits`
- **宿主真有**：`HOST:internal/transport/handlers_media_market_official.go:22-26` 原文：
  > 「`cotTranslation.translate` —— **真实现**：把文本送真实模型网关（`dshgo/internal/llm`，与 `cmd/dshd` 同一套 env：`DSHD_LLM_BASE` / `DSHD_LLM_KEY` / `DSHD_LLM_MODEL` / `DSH_GO_MODEL`）做一次翻译，返回模型真实输出。」
  `limits` 另有独立实现文件 `HOST:internal/transport/handlers_cot_translation_limits_official.go:31`
- **差什么**：GUI 缺 `conversation.chat.reasoning.body` 槽（思考块正文的 body 工厂）—— 该槽要在
  `plugins/dsconversation` 的思考块渲染里开。

### #64 `forge`（85 文件 / 20583 行）— **HIDE**
- 自有槽（11 个）：`conversation.hero.options`、`conversation.input.dock`、`conversation.session.header.utilities`、
  `sidebar.right.pane.tab`、`.title`、`sidebar.session.row.hover`、`.trailing`、`shell.overlay`、
  `plugins.row.config`、`forge-panel-state`、`forge_panel_change_requests`
- 组件：`OFF:forge/src/client/ChangesTab.tsx:1-1288`、`ForgeTab.module.css:1-627`、`ChangesTab.module.css:1-984`
- 文案：`OFF:forge/src/client/locales.ts:1-3446`（这是 19 条里最大的一个）
- 数据：`forgePanel.*` **31 个方法**
- **差什么**（比「5/35」更精确的账）：宿主只有 **5 个**方法有挂载体 —— `HOST:cmd/dshd/serve_forge_state.go:1-6`
  的挂载表只有 `Track` / `Dismiss` / `Refresh` / `Snapshot` / `Wait`；**31 个里 15 个只有结构化 501**
  （`HOST:internal/transport/handlers_forge_liveshare_official.go:411-412` 与逐条 `note` `:239-400`），
  另 **12 个需要 GitHub 凭据**（`HOST:cmd/dshd/serve_forge_rc2.go:57`）。
  面板要的 git 读取 / code host / worktree / panel 状态机（约 2079 行宿主逻辑）宿主一行没有
  （`HOST:internal/transport/handlers_forge_liveshare_official.go:217,259,266,285,299,306`）。
  → diff / change request / worktree 三块主体全无数据。**不自造。**
- **怎么补**：宿主先落 forge 的 git/worktree/CR 真实后端（服务端另有 gitee/github 两类远端）。

### #66 `voice-input`（15 文件 / 1826 行）— **HIDE**
- 自有槽：`conversation.input.activity`（GUI 有 `OffInputActivity`）、`plugins.bundle.activation`、`.config`、
  `dsh.voice-input.microphone`
- 组件：`OFF:voice-input/src/client/VoiceInput.tsx:1-182`、`MicrophonePicker.tsx:1-140`、
  `PreparationCard.tsx:1-165`、`audio.ts:1-188`
- 文案：`OFF:voice-input/src/client/locales.ts:1-834`（下载/错误族 `download.*`、`downloadAdvice.*`）
- 数据：`speech.catalog` / `.configure` / `.prepare` / `.transcribe` / `.cancelPreparation` / `.follow`
- **差什么（注意：这里宿主是好的，缺的是客户端）**：六个 `speech.*` 在宿主**全有真实现且已接线**
  —— `HOST:cmd/dshd/serve.go:940`、`HOST:cmd/dshd/serve_speech.go:17-58`、`HOST:internal/speech/engine.go`。
  卡的**不是**服务端，是**录音**：官方客户端录音整链走浏览器专有 Web Audio
  （`OFF:voice-input/src/client/audio.ts:59,63,75,91,95,152`），而 **Go+Viva 没有任何音频采集能力**
  （`viva@v0.6.0` 全树 `grep -i audio` 只命中 .deb MIME 分类串；本仓 `grep -ri "Mic\|Record\|Audio"` 0 命中）。
  麦克风图标官方素材库里也没有。
  → **客户端缺采集通道**，不是"宿主欠账"。这条与 #66 的 MENTION/NONE 判定无关，是能力边界。
- **怎么补**：Viva 侧需先有音频采集（或宿主改成"上传音频文件"形态）；按本仓规矩，这类能力缺失应
  作为上游需求登记，而不是在插件里自己拼一个假录音。

---

## 3. 这套证据把「结构证据覆盖率」推到了多少

主库判据（`official-ui-packs-coverage-rc2.md:9-11`）：**DEEP = 有专题文档给层级/slot/几何/行为之一以上且带 `file:line`**。

| 口径 | DEEP 数 | /66 | 说明 |
|---|---:|---:|---|
| 本轮之前（主库 `structure/`） | **8** | **12%** | 见审计 `tmp/GO8-RC2/_audit/ui-face.md:355-363` |
| 本轮之后（主库 8 + 本表 19 条一手 `src/**` 证据） | **27** | **41%** | 本表 §2 每条都带 `OFF:…:行号`，可逐条复跑 |

**这个 41% 的口径要读准**：
- 它只加上了 **`docs/official-parity.md` 原本判 ❌ MISSING 的这 19 条**；
- **MENTION 那 25 条里剩下的 21 条**（除去本表 4 个 `settings-*`）本轮**没有动**，仍是 MENTION；
- 所以 27 = 8（原 DEEP）+ 19（本表），不是「主库被改过」。主库 `structure/` 本轮**一个字没改**
  （它不在本仓，也不在本次可改范围）。
- 复跑算式：
  ```bash
  # 本表里带 OFF 锚点的包数（每节一个 ### 标题）
  grep -cE '^### #' D:/DeepSeek Harness/default-workspace/dsh-ui-rc2/docs/official-ui-structure-evidence-rc2.md   # → 19
  # 主库 DEEP
  sed -n '/^## 2. DEEP/,/^## 3./p' "$DSH_HOME/dsh-ui-official/structure/official-ui-packs-coverage-rc2.md" | grep -c '^| `dsh-'   # → 8
  ```

---

## 4. 与本表配套的实现状态

| 包 | 判定 | 落点 |
|---|---|---|
| `action-cards` | **已实现** | `plugins/dsactioncards`、`internal/apiclient/action_cards.go`、`internal/uikernel/official_slots.go` 两条新槽 |
| `directory-picker-browse` | **已实现** | `plugins/dsdirpicker`、`internal/apiclient/dirpicker.go`、`cmd/app/dirpicker_wiring.go` |
| `plan` / `cot-translation` | 可做，本轮未做 | 缺的槽与通道见 §2 各条 |
| 其余 15 条 | HIDE | 缺的证据/能力见 §2 各条 |

> 顺带补掉的 **PARTIAL #6 `attachment`**：`conversation.input.attachments` 的**第一个真实占用方**
> （`plugins/dsattachment` + `internal/apiclient/attachments.go`）+ 拖入通道
> （`dsattachment.HandleFileDrop` ← Viva `app.Config.OnFileDrop`）。
> 审计 U6「rail 槽有没有占用方」在此闭合：**此前只有 dispatch、零注册者**。

**HIDE 不是"以后再说"的托辞**：每条的「差什么 / 怎么补」都指到具体的宿主文件行号或具体的结构证据缺口。
按本仓铁律（`$DSH_HOME/AGENTS.md`「官方 UI 结构树与素材库」），**缺证据就 HIDE，不自造**。
