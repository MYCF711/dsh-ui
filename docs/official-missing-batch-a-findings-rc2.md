# GUI 侧 16 条 MISSING：本轮核查结论与下一批排序（A 批）

> 2026-11-15 · 配合 [official-parity.md](official-parity.md) 的官方对账表读。
>
> 本文只写**核查过的事实 + 精确阻断点**，不做实现承诺。每条都给了可复核的锚点，
> 下一批开工不必再从头翻 `src/**`。
>
> 口径：`OFF:<包简称>/<src 相对路径>:<行号>`，`OFF` 根 =
> `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`；`HOST:` = 宿主仓。

---

## 0. 本轮已闭合（1 条）

| id | 包 | 状态变化 | 证据 |
|---|---|---|---|
| #31 | `dsh-client-ui-plugin-recommendations` | ❌ → ✅ | [official-plugin-recommendations-evidence-rc2.md](official-plugin-recommendations-evidence-rc2.md) |

实现同时补上了动作卡框架一直缺的那一格（官方 `ctx.actionCards` 注册表 → 本仓
`pluginapi.KeyActionCardBodies`），所以**下一个「kind 自有正文」的包不需要再动框架**。

---

## 1. 逐条核查结果（按"能不能真做"排序）

### A 档：宿主能力已在，只差界面（下一批首选）

| id | 包 | 官方形态 | 宿主已有什么 | 还差什么 |
|---|---|---|---|---|
| #18 | `dsh-client-ui-file-actions` | 右键菜单里的「复制图片 / 下载 / 另存为」（`OFF:fa/src/client/context-menus.tsx:54-104`，两个 target：`file` / `image`），动作实体在 `OFF:fa/src/client/files.ts`（MIME 归一 + 落盘命名） | `workspaceFiles.readBytes` / `workspaceFiles.stat` 真实现（`HOST:internal/transport/handlers_workspace_official.go:69-70`、`:545`）；`session.openWorkspacePath` 的 `action:'reveal'`（`HOST:internal/transport/handlers_native_official.go`）；viva 有 `app.SaveFileAsync`（原生另存对话框） | ①`pluginapi/context_menu.go` 只有 `workspace`/`session` 两个 target，要加 `file`/`image`；②`apiclient` 还没有 `workspaceFiles.*` 客户端；③**复制图片没有原生写入口**：viva 只导出 `ClipboardImagePNG`（读），写只有测试用的 `Driver.SetClipboardImage` —— 所以「复制图片」这一格要么缺、要么改用 `SetClipboardFiles`（那是**复制文件**，语义不同，不许冒充）；④`ui.Controls` 侧要在 `dsfiles` 文件行与消息里的图片瓦片挂菜单 |

### B 档：要一次架构判断（不是薄层，别按"小包"排期）

| id | 包 | 真实形态 | 阻断点 |
|---|---|---|---|
| #16 | `dsh-client-ui-directory-picker-native` | **不是一个独立对话框，是 browse 选择器的替代后端**：两个 `single` 洞 `conversation.hero.workspace.directoryFlow` / `sidebar.workspaces.directoryFlow`（`OFF:ws/src/client/contract/slots.ts:137-139`）的正文本体，每次 `open` 边沿调一次 `pick()`，上报唯一结果（`OFF:dpn/src/client/flow.ts:20-90`） | 官方两个后端**互斥**：`dsh-host-directory-picker-auto` 在启动时二选一，同时挂会「重复 `directoryPicker` 服务、`single` 类 slot 中重复 client 流程」直接报错（`OFF:host-directory-picker-auto/README.zh.md` 末节）。本仓把 browse（#15 ✅）直接接成了"点了就开对话框"，**没有这两个洞、也没有"后端选择"这一层**。要补 #16 = 先补 seam（洞 + 启动时判定 + 二选一），否则两条会同时在跑或互相顶掉 |

### C 档：宿主/框架缺前置，客户端单方面做就是假实现

| id | 包 | 缺的前置（已核） |
|---|---|---|
| #13 | `dsh-client-ui-creation-recommendations` | 本包只是 `create_personalized_page` 的卡片正文（`OFF:cr/src/client/mount.ts:17-27`）。**宿主没有这个 kind**：`HOST:internal/actioncards` 只注册了 `enable_official_plugin`（`HOST:internal/recommendations/service.go:34`、`:70-80`）。宿主侧还缺 `dsh-creation-recommendations` / `dsh-personalized-recommendations` 两条线的 kind + 动词（create/undo/restore/cancel）。先做界面 = 永远不出现的死代码 |
| #39 #45 #46 #47 | `settings-agent-loop` / `-shell` / `-subagent` / `-web-search` | 四页都是 `configForms` 的 `plugins.item` 卡片（`OFF:sal/src/client/index.ts:44-52` 等），**只在宿主服务该命名空间时才挂载**（`whileServed([...])`）。宿主 `settingsform` 只注册了 `permission` / `model`（`HOST:internal/settingsform/settingsform.go:14-15`、`registry.go:180-188`，`llm` 走 provider 面），**没有** `agent-loop` / `bash` / `pwsh` / `subagent` / `web` / `web-deepseek-api` / `web-search-deepseek`。且本仓还缺两级前置：`configForms` 服务、插件页的 `plugins.item` 槽（`plugins/dssidebar/plugins_page.go:37` 已登记该缺口） |
| #17 | `dsh-client-ui-entry-customization` | 三个「更多」菜单（`OFF:ec/src/client/index.ts:52-76` 注册进 `sidebar.panellist.actions` / `conversation.session.header.views.actions` / `sidebar.right.tab.guide.actions`——**本仓三个槽已声明**：`internal/uikernel/official_slots.go:99,129,185`）。但「创建你的插件」= `uiWorkspace.startSession(undefined,{agentPreset:'cordis',prompt})`（`OFF:ec/src/client/index.ts:46`），而**宿主没有 `cordis` 预设**（`grep 'cordis' internal/agent cmd/dshd \| preset` 为空）→ 那一行点了必然失败。另需 unpinned 入口清单 + pin 控件（两个都不是现有能力） |

### D 档：没有对应层，按 HIDE 登记（不要试图用别的东西冒充）

| id | 包 | 为什么不能做 |
|---|---|---|
| #19 | `dsh-client-ui-file-drag` | 官方是 **HTML5 拖出**：`DataTransfer.setData('DownloadURL', …)` + `dragend`（`OFF:fd/src/client/transfers.ts:150-190`），Desktop 侧走 Electron 的 `DesktopFileDrag` 原生桥。Go+Viva 没有 HTML5 `DataTransfer`，也没有系统级拖出源（OLE `IDropSource`）的公开 API；`app.SetClipboardFiles` 是**复制文件**，不是拖拽（语义不同，不许冒充） |
| #50 | `dsh-client-ui-sidebar-browser` | 已在账本里登记（右栏沙箱浏览器 tab；本仓没有 webview/iframe 层） |

---

## 2. 未深入核查的（下一批自己开局先看这几行）

| id | 线索（只做了初查，未定论） |
|---|---|
| #12 | `dsh-client-ui-cordis`（15 文件 / 85KB）。宿主**有** `dynamicCordisRunner.*` 六个方法（`HOST:internal/transport/handlers_media_market_official.go:72-78`），说明这条线在宿主侧是活的；界面侧要先读 `OFF:cordis/src/client/**` 的页面骨架再估 |
| #28 | `dsh-client-ui-plan`（12 文件 / 37KB）：`PlanModeControl` + `PlanCard` + `plan-resource`。要先确认宿主有 plan 相关投影，否则同 C 档 |
| #30 | `dsh-client-ui-plugin-market`（7 文件 / 100KB）。宿主 `pluginMarket.search` / `readEntryDetail` / `setIndexes` 三个方法在路由里是真分支（`HOST:...handlers_media_market_official.go:206-211`），但 100KB 的界面不是一轮的量 |
| #63 #66 | `cot-translation` / `voice-input` 的宿主方法**都在**（`cotTranslation.translate`、`speech.prepare|transcribe|configure|cancelPreparation`，见同一文件的 method 表）。比 C 档乐观，但都要工业级 UI（实时字幕 / 录音状态机） |
| #64 | `dsh-experimental-client-ui-forge`（860KB）——单包比其余全部加起来还大，必须单独立项 |
