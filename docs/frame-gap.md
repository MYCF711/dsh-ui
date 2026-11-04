# FRAME-GAP：官方当前窗口 vs `f007fad` 结构 / 规格缺口矩阵

> owner：FRAME-GAP；审计基线：UI `f007fadd18964a1f2dad0aefa9a939855ee23bcc`（短名 `f007fad`）。
>
> 结论先行：下一批先做 **窗口 Chrome、品牌/导航侧栏、会话 header + 页签 + 右栏、composer 控件、dock 指标** 五包。设置、账号、充值、更新不进入这批；它们既不能解释当前“第一眼不像 DSH”，也会把高可见度缺口拖成低价值大工程。

## 0. 证据口径与边界

- 官方像素只采用 [`official-current.png`](../tmp/visual-measurement/official-current.png)（1920×1032，SHA-256 `bf5d96fee4bf71c18fb1acc2158d4280f11e6a1c111615e858d87a0625ee2d52`）。其 [`meta.json`](../tmp/visual-measurement/official-current.meta.json#L1-L31) 记录了官方进程 `D:\DeepSeek Harness\Desktop\DeepSeek Harness.exe`、窗口标题、`PrintWindow(PW_RENDERFULLCONTENT=2)`、DWM bounds 和空 `window_operations`；[`source-audit.json`](../tmp/visual-measurement/source-audit.json#L59-L77) 将其归类为 `official_dsh_electron_current_window`。
- 这是一张**当前官方会话**截图，不是同状态官方首屏。本文只用它确认部件的存在、层级、选中/开合状态和相对位置；**不计算、不引用整屏相似度、差异百分比或逐像素评分**。空态、首屏和容器数值只取官方 DOM/CSS/JS，不从这张当前会话正文反推。
- 官方实现证据直接读安装态 `D:/DeepSeek Harness/Desktop/resources/app.asar`，并使用已提取的 [`tmp/official/`](../tmp/official/) 与 [`tmp/asar-css/`](../tmp/asar-css/)。`app.asar::<path>` 表示归档内路径，不表示另有一个已解压目录。
- 当前实现固定审 `f007fad`。审计时 HEAD 与工作树已有后续改动（例如侧栏搜索结果、`dsshortcuts`、滚动修复），**不并入本基线结论**。基线侧栏/壳行号以 `git show f007fad:<path>` 为准；链接用于定位源码，后续工作树可能已移动行号。
- 分类：**UI** = 现有快照/动作足够；**行为** = 需要本地路由、焦点、弹层、OS 窗口集成，但不必新增 Transport；**契约** = 必须补宿主字段/方法或明确的扩展描述，不能用静态字、假数字或死按钮冒充。

## 1. 最快提升“像 DSH”的 5 个包

| 优先级 | 包 | 为什么先做 | 本批可交付的真实结果 |
|---:|---|---|---|
| 1 | **FG-1 Windows Chrome** | 40px 顶栏和「应用 / 编辑」是官方桌面壳最稳定的识别特征；当前完全没有这一层 | 40px 客户区顶栏、两枚真实菜单、28px 按钮与窗口控制；编辑动作走真实焦点/剪贴板 |
| 2 | **FG-2 品牌 + 导航 + 工作区侧栏** | 左栏是持续可见的最大识别面；当前从“新建会话”直接开始，缺品牌、38px 新建按钮和 36px 导航行 | 官方品牌行、新建按钮、可路由面板导航、工作区 header；无真实面板的入口保持隐藏 |
| 3 | **FG-3 会话 header + 页签 + 右栏** | 当前用“工作区 / 标题 + 共 N 条 · mode + 模型/权限”替代了官方结构，并在有工具消息时自动吃掉右栏 | 76px active header、对话/轨迹真实切换、28px corner 展开钮、右栏默认收起；团队/子智能体无真值不画 |
| 4 | **FG-4 Composer 控件行** | 输入卡是视线落点；当前缺命令入口、权限图标、模型 chevron 和官方矢量发送钮，权限还显示裸 `allow` | 对齐已有卡片几何、真实模型/权限文案、真实发送/停止；命令和附件只有在动作存在时才出现 |
| 5 | **FG-5 Composer dock + 指标真值** | 官方签名是卡片下方的轮次/步数、总 token/缓存命中、上下文环；当前却是一条 24px 全宽状态栏，且轮/步口径不同 | 把 dock 放回 composer 下方；显示已有真值，撤掉错误轮/步；缓存命中和上下文百分比必须等分母/容量契约 |

**并行纪律**：五包的插件 owner paths 不重叠。`cmd/app` 只归 FG-1；`pluginapi`、已有 `internal/apiclient/client.go`、双仓快照和 [`transport-contract.md`](../../DSH-GO/docs/transport-contract.md) 由 Lead 串行集成，任何并行包都不得把它们当隐式写入范围。需要新宿主能力的部分列在 §4，不与“纯 UI 已完成”混报。

## 2. 结构 / 规格矩阵

| 部件 | 官方源码与规格 | `f007fad` 当前位置与差距 | UI / 行为 / 契约 | 用户影响 | 工作包 |
|---|---|---|---|---|---|
| **应用菜单 / 窗口 Chrome** | `app.asar::lib/main.js:11937-11949`：Windows `titleBarStyle:hidden`、`titleBarOverlay.height=40`；`app.asar::lib/preload-app.cjs:454-579`：Shadow DOM `role=menubar`，两枚按钮来自 `application` / `edit`。同文件 460-469：host 高 `--dsh-windows-titlebar-height`，按钮 **28px 高、10px 横 padding、6px 圆角、14px 字、menubar gap 2**；496-541 负责原生 popup 与恢复编辑器焦点。`app.asar::lib/main.js:12836-12856` 经 `dsh-desktop:windows-menu` 弹出应用菜单，`:12721-12725` 是编辑菜单。 | [`cmd/app/main.go`](../cmd/app/main.go) 的 `newConfig`（基线 1064 起）只设置普通 `Title/Width/Height/MinWidth/MinHeight`；[`dsshell`](../plugins/dsshell/shell.go) 从内容区直接切三栏。没有 40px caption、应用/编辑菜单、焦点保存或 popup。 | **UI+行为**：顶栏、菜单外观、编辑命令可做；无边框窗口/最小化/最大化/关闭和编辑器焦点恢复需 L0。关于、更新、CLI、退出确认属于壳能力，不在这批伪造。 | 第一眼不像官方桌面应用；菜单操作与键盘焦点行为也缺失。 | **FG-1** |
| **品牌侧栏 / 新会话** | [`ui-brand-official`](../tmp/official/dsh-client-ui-brand-official/client.js#L15-L39) 把官方鱼标和 `BrandWordmark` 分别注入 `sidebar.brand.mark/name`。[`ui-sidebar`](../tmp/official/dsh-client-ui-sidebar/client.js#L396-L459) 是 `logoRow > brand + toggle`；[`SidebarRoot CSS`](../tmp/asar-css/128__SidebarRoot.module.css.mjs.css) 的 Windows `logoRow=40px`、`brandIdentity=24px/gap8`、`brandName=18/24`；新会话 **38px 高、12px 圆角、14/22 500、Windows 上 margin-top 8 / bottom 12**。 | [`dssidebar/sidebar.go`](../plugins/dssidebar/sidebar.go) 基线 `drawHeader:163-198` 只有 **32px 高**的“+ 新会话”和收起钮；没有品牌行。官方鱼路径虽已存在于 [`brand_path.go`](../plugins/dsbrand/brand_path.go)，但只服务主区 [`hero.go`](../plugins/dsconversation/hero.go)，没有侧栏 wordmark。 | **UI**：品牌、几何和已有 NewSession 可直接做；侧栏收起状态与顶栏/菜单起点联动属**行为**。品牌无动作时画非按钮，不造点击承诺。 | 左侧品牌识别完全缺失，新会话纵向节奏也明显不同。 | **FG-2** |
| **插件 / 自动化 / 压缩 / 更多导航** | [`ui-sidebar`](../tmp/official/dsh-client-ui-sidebar/client.js#L190-L313) 维护 panel 列表和 pin；同文件 469-499 渲染入口与“更多”。CSS：`panelRow min-height 36 / gap8 / padding 7px 8px / radius-md / line 22`，`panelList gap2 / bottom8`。内置注册点：[`plugin-manager`](../tmp/official/dsh-client-ui-plugin-manager/client.js#L4353-L4356) 的 `PANEL_ID=plugins`、同文件 4484-4490 `order:0`；[`schedule`](../tmp/official/dsh-client-ui-schedule/client.js#L7362-L7364) 的 `PANEL_ID=schedules`、同文件 7484-7490 `order:10`。**特别校正**：本轮 asar 文案检索只在 `dsh-command-compact/locale/zh.json` 找到“上下文压缩命令”，没有找到其 `sidebar.panellist` 注册；截图里的“上下文压缩”应按当前 profile/扩展入口待证，不硬编码成官方固定核心项。 | [`pluginapi`](../pluginapi/plugin.go#L110-L132) 只有单个 `main.body`，没有 keyed main panel / activePanel；[`Actions`](../pluginapi/plugin.go#L367-L415) 没有 SelectPanel。侧栏 header 后直接是工作区列表，插件、自动化、更多均不存在。 | **UI+行为**：本地 panel 描述、选择、pin、返回会话可做；插件只读清单可从 kernel 真值做。自动化 catalog/执行、压缩页数据与操作需**契约**；扩展入口需统一 panel descriptor，不能为截图特判。 | 当前不像同一产品的导航模型；也无法从侧栏进入插件/任务页。 | **FG-2**；自动化/压缩行为见 §4 |
| **工作区 header / 搜索 / 会话行** | [`WorkspaceBrowser`](../tmp/official/dsh-client-ui-workspace/client.js#L3826-L3964)：`sectionHeader > label + search + viewOptions + workspace.add`；[`WorkspaceBrowser CSS`](../tmp/asar-css/155__WorkspaceBrowser.module.css.mjs.css)：header **36px**、搜索展开 **30px**、图标钮 **28×28**。行规格见 [`official-specs.md`](../docs/official-specs.md)：project 34、session 32、标题 14/20、时间 10/16、slot 16×20、行间 2 / 组间 4。 | [`dssidebar/sidebar.go`](../plugins/dssidebar/sidebar.go) 基线 `drawList:214+` 已有 cwd 分组、真实标题/时间/选中态和 30px 搜索框，这是可保留基础；但搜索始终展开，没有 36px section header、视图选项、添加工作区按钮，基线也没有官方 search result card / leading slot。 | **UI**：section header、折叠搜索、现有列表密度可继续做；视图切换是本地**行为**。添加工作区要 OS picker + workspace mutation；跨会话语义搜索要宿主 search，不得拿标题过滤冒充。 | 现有列表已经“像列表”，但缺少官方工作区工具带和状态图标，找会话/加目录能力不完整。 | **FG-2**；添加/全文搜索见 §4 |
| **用户区 / 设置 footer** | [`SidebarRoot`](../tmp/official/dsh-client-ui-sidebar/client.js#L513-L520) 的 foot 依次是 `sidebar.footer.action`、`sidebar.settings`；[`settings-general`](../tmp/official/dsh-client-ui-settings-general/client.js#L59-L90) 的 trigger **42px**；[`AccountMenu CSS`](../tmp/asar-css/081__AccountMenu.module.css.mjs.css)：整行 **44px**、avatar **24×24**、gap8、14px 字；[`AccountMenu.js`](../tmp/official/dsh-client-ui-settings-account/client.js#L1770-L1907) 负责设置、联系、登出等真实菜单。 | [`dssidebar/sidebar.go`](../plugins/dssidebar/sidebar.go) 基线 `drawFooter:200-212` 是 **22px** 的“N 个会话 / 已连接…”；[`SlotSidebarFooter`](../pluginapi/plugin.go#L117-L120) 虽存在，但没有设置或账号实现。 | 设置面板是**行为**；头像、脱敏账号、登录/登出需账号**契约**。没有账号真值时隐藏，不画假头像/假手机号。**本批不做**。 | 视觉有差异，但远低于品牌、header、composer；账号不是当前主工作流瓶颈。 | 后置，不占前五包 |
| **会话标题 / preset / 团队 / 子智能体标签** | [`ConversationSessionHeader`](../tmp/official/dsh-client-ui-conversation/client.js#L18131-L18229) 是 `titleCluster: crumbs + headerActions + utilities + corner`，不是把模式、消息数、模型都塞进 crumb。[`AgentPresetLabel`](../tmp/official/dsh-client-ui-agent-preset/client.js#L971-L1020)：只读 preset，**22px 高、12/22、max180**；注册 `order:-10`。团队来自 `app.asar::dsh/node_modules/@deepseek-ai/dsh-experimental-client-ui-agent-team/lib/client.js:399,465-497`，`TeamAction` 注册 `order:-20`，CSS 在同文件 12（min-height28、12px 字、gap5）；子智能体数量见 [`ui-subagent`](../tmp/official/dsh-client-ui-subagent/client.js#L603-L623) 和注册 983-989（`order:10`）。 | [`dstopbar/topbar.go`](../plugins/dstopbar/topbar.go#L181-L245) 画 `工作区 / 会话标题`，再接 `共 N 条 · mode` 以及 `模型 · 审批 permission`；缺少官方 headerActions 分层，团队/子智能体无数据。现有 `Session.Mode` 可保留为真值，但不能未经语义确认就翻成“标准模式”。 | **UI**：标题、slots 几何、已知 preset 的只读 label；**契约**：`agentPreset` 产品名映射、`agentTeam`、`subagentCatalog` 与对应导航。团队/数量未知必须整段隐藏。 | 当前信息层级与官方不同，且把模型/审批重复放在标题与 composer；团队用户看不到真实团队状态。 | **FG-3**；团队/子智能体数据见 §4 |
| **对话 / 轨迹 / 主题鱼页签** | [`ConversationRoot CSS`](../tmp/asar-css/036__ConversationRoot.module.css.mjs.css)：active header **min-height 76、padding 10px 28px 0 20px**，titleRow 30，tabs **height25、margin-top10、gap20**，active tab 用 `border-l4`；[`ViewTabs`](../tmp/official/dsh-client-ui-conversation/client.js#L18084-L18125) 按 registry 渲染。对话由 [`ui-chat`](../tmp/official/dsh-client-ui-chat/client.js#L13256-L13268) 注册 `id=chat/order0`；轨迹由 [`ui-trajectory`](../tmp/official/dsh-client-ui-trajectory/client.js#L8205-L8238) 注册 `id=trajectory/order10`。**“主题鱼”在 asar 未找到注册源或文案**，按运行时扩展 view 待证，不纳入官方固定三页签承诺。 | [`dstopbar`](../plugins/dstopbar/topbar.go#L24-L76) 只移植了无 tabs 的 50/40/10px header；没有 view registry、activeView 或 tabs。轨迹被固定放到 [`dsrightbar/rightbar.go`](../plugins/dsrightbar/rightbar.go)，不能在主区切换。 | **UI+行为**：对话/轨迹可用现有消息和工具时间线做本地 view 切换；按会话记住 view，切 view 不重载、不停流。第三方“主题鱼”需要扩展 view descriptor + 实际插件，不能画死页签。 | 官方核心导航缺失；用户无法在主区看轨迹，当前 header 高度也错。 | **FG-3** |
| **右上 actions / utilities / corner** | [`ConversationSessionHeader`](../tmp/official/dsh-client-ui-conversation/client.js#L18176-L18230) 分为 `headerActions`、`headerUtilities`、`headerCorner`。utilities 已有 [`open-in-app`](../tmp/official/dsh-client-ui-open-in-app/client.js#L838-L852) 与 [`schedule`](../tmp/official/dsh-client-ui-schedule/client.js#L7509-L7527)；corner 的右栏展开钮见 [`ExpandButton`](../tmp/official/dsh-client-ui-sidebar-right/client.js#L655-L691)，**28px**、收起时才显示，注册点在同文件 5079-5084。右栏自己的全屏/收起按钮见同文件 1477-1512。刷新是当前右页 command（同文件 274-326），不是无条件会话刷新。 | [`dstopbar/topbar.go`](../plugins/dstopbar/topbar.go#L247-L281) 只有一个 corner，用 `>` / `»` 文本 glyph 调 `ToggleRightbar`；没有 actions/utilities 分层、矢量图标、右栏全屏。更严重的是 [`cmd/app/main.go`](../cmd/app/main.go) 基线 `syncRightbarLocked:175-187` 在任意 tool 消息存在时自动展开右栏；官方当前截图证明“有工具历史”并不必然展开。 | **UI+行为**：28px 展开/收起、右栏默认关闭、按会话记住开合、全屏可做。打开外部应用要 OS app discovery；任务动作要 schedule；刷新要目标页命令，均需对应**契约/页面能力**，不能统一点成 reload。 | 当前右栏会无谓压缩正文；官方动作语义被简化成一个开合 glyph。 | **FG-3** |
| **Composer 附件 / 命令入口** | [`InputBar CSS`](../tmp/asar-css/039__InputBar.module.css.mjs.css)：`add` **28×28 圆形**，只在没有 action 时隐藏；它是 `toggleCommandMenu`，不是附件钮，证据见 [`InputBar.js`](../tmp/official/dsh-client-ui-conversation/client.js#L19153-L19196)。附件另有 `conversation.input.attachments`，注册见 [`ui-attachment`](../tmp/official/dsh-client-ui-attachment/client.js#L889-L920)；文件卡 **240×64**（`FileCard.module.css`），上传/发送 orchestration 见 [`conversation`](../tmp/official/dsh-client-ui-conversation/client.js#L4664-L4745)。 | [`dscomposer/composer.go`](../plugins/dscomposer/composer.go#L21-L29) 明确因无动作而不画加号；生产代码没有附件 rail、上传状态、图片 draft 或附件 prompt。当前 [`Snapshot`](../pluginapi/plugin.go#L227-L280) 也没有附件/命令目录。 | **UI**：28px 按钮、菜单和附件 rail 外观；**行为**：命令选择、焦点恢复；**契约**：命令目录、文件上传、draft attachment id、带附件 prompt。没有真实动作时继续隐藏，不把“+”画成死按钮。 | 用户不能发图/文件，也缺少官方输入区左侧视觉锚点。 | **FG-4** 做 UI 壳与真实命令接入；上传见 §4 |
| **Composer 权限 / 模型** | [`PermissionSelect CSS`](../tmp/asar-css/061__PermissionSelect.module.css.mjs.css)：trigger **28px 高、max220、13/20 500、14px 权限图标 + chevron**；官方 `full-access` 文案“完全权限”见 [`permission-presets`](../tmp/official/dsh-client-ui-permission-presets/client.js#L155-L171)。[`ModelSelect CSS`](../tmp/asar-css/059__ModelSelect.module.css.mjs.css)：trigger **28px 高、13/20 400、max min(360px,45cqw)、chevron**；模型 catalog/setSelection 见 [`model-selection`](../tmp/official/dsh-client-ui-model-selection/client.js#L711-L779) 与同文件 1903-1922。权限切换经 `/permission`，见 [`permission-presets`](../tmp/official/dsh-client-ui-permission-presets/client.js#L753-L823)。 | [`dscomposer/composer.go`](../plugins/dscomposer/composer.go#L379-L410) 已有真实 `Snapshot.Permission/Model`，但画成无图标、无 chevron 的只读文本；权限直接显示 `allow/ask/...`，不是产品文案。相同模型/权限还重复出现在顶栏。 | **UI**：按明确映射显示产品名、图标、28px trigger、去掉顶栏重复；**契约**：可选模型/catalog、effort、权限 catalog 和 mutation。没有 mutation 时保持只读，不画可展开假象。 | 当前裸枚举值很不像官方，且用户不能切换模型/权限。 | **FG-4**；切换能力见 §4 |
| **Composer 发送 / 停止** | [`InputBar CSS`](../tmp/asar-css/039__InputBar.module.css.mjs.css)：`primary` **34×34、999 圆角、button-info-fill、translateY(-2px)**；[`InputBar.js`](../tmp/official/dsh-client-ui-conversation/client.js#L19318-L19332) 按 stop/disabled 切换行为。 | [`dscomposer/composer.go`](../plugins/dscomposer/composer.go#L411-L450) 已有 34px 发送、真实 Send/Cancel、空白禁发；几何基本可复用，但图标是 `↑` / `■` 文本 glyph，不是官方矢量。 | **UI**：矢量图标、hover/disabled/busy 视觉；发送/取消已有真实**行为**，不要重写协议。 | 这是当前最接近官方的控件；主要影响精致度，不应挤占附件/模型契约时间。 | **FG-4** |
| **底部轮次 / 步数 / 速率** | 官方不是出厂 `shell.bottom`，而是 [`conversation.composer.dock`](../tmp/official/dsh-client-ui-chat/client.js#L13346-L13362) 的 `ActivityPill(order0)`。[`StatsPills CSS`](../tmp/asar-css/028__StatsPills.module.css.mjs.css)：secondary 12/20、pill padding `1px 8px`、gap6。[`deriveStats`](../tmp/official/dsh-client-ui-chat/client.js#L7435-L7472)：**turns 是唯一 turn 数，steps 是 assistant/step 数**，速率是 decodeTokens/decodeMs；优先读 whole-log `sessionStats`，窗口 fold 只是 fallback。 | [`dsstatusbar/statusbar.go`](../plugins/dsstatusbar/statusbar.go#L73-L125) 在 24px 全宽 `shell.bottom` 左侧显示；同文件 76-84 却把 **assistant 条数当“轮”、tool 条数当“步”**。当前消息没有 turn id / decodeMs，tail 也不能代表 whole-log。 | **UI**：移动到 composer dock、pill 排版；**契约**：`sessionStats{turns,steps,decodeTokens,decodeMs}` 或等价的 whole-log 真值。契约前撤掉错误轮/步，不能用现有计数换皮。 | 位置不像只是表面问题；当前数字会误导用户判断工作量和速度。 | **FG-5**；统计真值见 §4 |
| **底部总用量 / 缓存命中** | 官方 `UsagePill(order1)` 与 Activity 同处 composer dock，注册证据同 [`ui-chat`](../tmp/official/dsh-client-ui-chat/client.js#L13346-L13362)。[`UsagePill`](../tmp/official/dsh-client-ui-chat/client.js#L7633-L7672) 显示 whole-log `totalTokens`，有 `cacheReadTokens` 才显示缓存命中；[`cacheHitPercent`](../tmp/official/dsh-client-ui-chat/client.js#L7495-L7505) 的分母是 **uncachedInput + cacheRead + cacheWrite**，不是 `totalTokens`。 | [`dsstatusbar/statusbar.go`](../plugins/dsstatusbar/statusbar.go#L141-L167) 显示 `↑input ↓output` 和 `上下文 N`，已有 `UsageTotal.totalTokens/cacheReadTokens`，但没有官方 `billedInput/cacheWrite` 语义，不能直接抄分母。 | **UI**：总 token / 缓存命中布局；**契约**：明确 input 是否 billedInput、补 cacheWrite 或官方等价分母。缺分母时只显示总量，不编缓存百分比。 | 当前指标形状与官方不同；错误缓存率会比缺字段更糟。 | **FG-5**；usage 语义见 §4 |
| **右下上下文环 / 百分比** | [`ContextMeter`](../tmp/official/dsh-client-ui-conversation/client.js#L18681-L18748) 是 composer dock 右侧独立部件：SVG **14×14、stroke2**，trigger 13/20、padding `1px 8px`；[`contextOccupancy`](../tmp/official/dsh-client-ui-conversation/client.js#L18640-L18648) 必须同时有 `usedTokens` 与 `contextWindow`，否则不画。点击面板还读 `contextBreakdown`。 | 当前只有 `ContextTokens`，[`LoadResult`](../internal/apiclient/client.go#L94-L107) 没有 `contextWindow/breakdown`；状态栏右侧实际画的是连接状态点和“就绪/已断开”，并非上下文环。 | **UI**：14px 环和弹层；**契约**：`contextPressure{usedTokens,contextWindow}`，明细另需 breakdown。只有 token 数时不能 invent 百分比。连接状态可保留在连接 UI，但不冒充官方上下文环。 | 缺官方最醒目的右下签名；编造百分比会误导压缩/续聊决策。 | **FG-5**；容量契约见 §4 |

## 3. 五包任务书（owner paths + 验收）

### FG-1 · Windows Chrome（最高优先）

- **owner paths**：`cmd/app/**`、`plugins/dsshell/**`、新增 `plugins/dschrome/**`。其他四包不得改这些路径。
- **范围**：40px Windows caption；应用/编辑两个 28px 菜单按钮；菜单 gap2、10px 横 padding、6px 圆角、14px 字；native window controls；编辑器 focus/selection 保存恢复；内容区从 caption 下开始，侧栏 logo 与菜单起点联动。
- **不做**：设置、账号、更新、CLI 管理；没有真实 handler 的菜单项不列。
- **验收**：
  1. `go test ./cmd/app ./plugins/dsshell ./plugins/dschrome`；菜单打开、Esc/外点关闭、焦点回到原输入框且 selection 不变；编辑项只调用真实动作。
  2. 1920×1032 与 1182×832 原生截图各一张：caption 40px、菜单钮 28px，关键容器与官方 CSS 偏差 ≤8px；下方三栏总高不被重复扣减。
  3. 最小化/最大化/关闭走 OS 真行为；应用菜单仅展示已实现项。证据记录版本、命令、退出码和未验证项，不用官方当前会话截图做整屏评分。

### FG-2 · 品牌 + 导航 + 工作区侧栏

- **owner paths**：`plugins/dssidebar/**`、新增 `plugins/dsbrand/**`、新增 `plugins/dspanels/**`。
- **范围**：40px 品牌行、官方鱼标 + wordmark；38px 新会话；36px panelRow / gap2 / bottom8；本地 activePanel 与 pin 状态；36px 工作区 header、可折叠 30px 搜索；保留真实会话行。插件页可先读 kernel 的真实装配清单，自动化/压缩没有真实 panel 时不显示入口。
- **验收**：
  1. `go test ./plugins/dssidebar ./plugins/dsbrand ./plugins/dspanels`；品牌字节/路径测试继续通过；NewSession 仍只调一次真实动作。
  2. 点击已注册 panel 会切换主区，返回会话后 session id、草稿、滚动位置不丢；“更多”pin/unpin 改变真实入口顺序；无 panel 的入口为零死按钮。
  3. 两档尺寸截图：品牌 40、新建 38、导航 36、工作区 header 36、搜索 30，偏差 ≤8px；1182 下列表不被 footer 挤出可视区。
  4. 账号/设置 footer 不在本包伪造；自动化、压缩和全文搜索未接契约前明确列为未验证/未实现。

### FG-3 · 会话 header + 对话/轨迹 + 右栏

- **owner paths**：`plugins/dstopbar/**`、`plugins/dsconversation/**`、`plugins/dsrightbar/**`。右栏默认策略所需的 `cmd/app` glue 由 FG-1 按本包接口一次性接入。
- **范围**：active header 76px；titleRow 30；tabs 25 / top10 / gap20；对话/轨迹按会话切换；28px corner 展开钮；右栏默认收起，用户明确操作后按会话记忆；移除顶栏重复的模型/权限和“共 N 条”占位。preset/team/subagent 只有真值才渲染。
- **验收**：
  1. `go test ./plugins/dstopbar ./plugins/dsconversation ./plugins/dsrightbar`；对话→轨迹→对话不调用整页 load、不取消流、不改 draft；两个会话各自记住 view。
  2. 含 tool 消息的历史会话首次打开右栏仍收起；点击 corner 才展开，切走再回来保持用户选择；右栏全屏/恢复正常。
  3. 两档尺寸截图：active header 76、titleRow 30、tabs 25、corner 28，偏差 ≤8px；无团队/子智能体字段时对应位置为空而非假标签。
  4. “主题鱼”无实际扩展 view 时不出现；本包验收只承诺官方 chat/trajectory 两个真实页签。

### FG-4 · Composer 控件行

- **owner paths**：`plugins/dscomposer/**`、新增 `plugins/dscommands/**`、新增 `plugins/dsattachment/**`、新增 `plugins/dsmodel/**`、新增 `plugins/dspermission/**`。
- **范围**：保留官方 card/input/row 几何；命令钮 28、权限/模型 trigger 28、发送 34 并上移 2；真实权限产品文案与图标、真实模型名、矢量发送/停止；命令菜单具备焦点恢复。附件和切换动作有接口但无宿主实现时，生产 UI 不显示可点承诺。
- **验收**：
  1. `go test ./plugins/dscomposer ./plugins/dscommands ./plugins/dsattachment ./plugins/dsmodel ./plugins/dspermission`；空草稿不发送、发送一次、busy 停止、IME 不被按钮抢焦点。
  2. `allow` 等已知 policy 只在有明确映射时显示产品名，未知原样/隐藏；空 model/permission 不画；没有 mutation 时没有 chevron 下拉假象。
  3. 命令目录为空则 `add` 不画；目录有项时点击 28px 按钮能执行一项真实 action。附件上传未接契约前不出现附件钮/rail。
  4. 两档尺寸截图与容器量测：28/34px 控件、卡片 max-width 与 16px clearance 偏差 ≤8px；发送图标不再依赖字体 glyph 猜测。

### FG-5 · Composer dock + 指标真值

- **owner paths**：`plugins/dsstatusbar/**`；统计专用新文件可放 `internal/apiclient/stats.go` 与宿主 `internal/transport/serve_stats.go`，但共享 client、SPI、snapshot、契约文档只能由 Lead 串行改。
- **范围**：把 Activity/Usage/Context 三类部件放到 composer 下方 dock，不再默认占 24px 全宽 `shell.bottom`；保留连接状态的独立出口。第一批先显示已有 `UsageTotal.totalTokens` 等真值；错误轮/步立即撤下。sessionStats、缓存分母、contextWindow 到位后再点亮对应 pill/环。
- **验收**：
  1. `go test ./plugins/dsstatusbar`；空值、tail 截断、压缩回落、跨会话切换均不显示上一会话数字；未知不补 0。
  2. dock 位于卡片下方并与卡片同中心轴，容器 `padding-top:4 / gap:12 / justify-center`，pill 12/20、`1px 8px`、gap6；无占用时 `BottomHeight=0`，主区不多一条全宽黑带。
  3. 契约后 golden：`turns` 按唯一 turn、`steps` 按 assistant/step、TPS 用 decodeTokens/decodeMs；缓存分母与官方三项一致；`usedTokens/contextWindow` 缺任一即隐藏环。
  4. 真宿主同会话 tail=1/50 的 whole-log 数字一致，A/B 会话不串；保留命令、退出码、dshd/UI rev 和未验项。不能只凭截图里的数字判定公式正确。

## 4. 后续行为 / 宿主契约队列（不冒充纯 UI）

| 后继项 | 必须补的真值/行为 | 验收锚点 |
|---|---|---|
| **C-STATS** | whole-log `sessionStats`；`billedInput` 或 `uncachedInput+cacheRead+cacheWrite`；`contextPressure{usedTokens,contextWindow}`，可选 breakdown | 同会话不同 tail 不变；官方公式 golden；缺字段隐藏；A/B 不串 |
| **C-COMPOSER-RW** | model catalog + selection mutation；permission catalog + 当前 preset mutation；命令 catalog | 选择后下一轮真实生效并能在 load/projection 回读；失败保留原选择，不本地假装成功 |
| **C-ATTACH** | 文件选择/拖放、上传进度/失败重试、draft id、附件读取、带附件 prompt | 图片/文件真实进入同一会话并能重载；失败草稿不丢；不只显示缩略图 |
| **C-PANELS** | 通用 `PanelDescriptor{id,label,order,kind}` 与 activePanel；schedule catalog/history/action；压缩页真实数据/action | 插件、自动化、扩展 panel 同一路由；返回会话保留现场；无注册即无入口 |
| **C-SESSION-CONTEXT** | preset 产品名映射、agentTeam、subagentCatalog 与导航 | 团队/数量与宿主 catalog 一致；点击打开正确 root/child；未知整段隐藏 |
| **C-WORKSPACE** | OS 目录选择、workspace add/remove、归档/pin/rename、跨会话搜索 | 目录真实出现在 workspace.list；操作后可重载；标题过滤不得标成语义搜索 |
| **C-EXTENSION-VIEWS** | 第三方 view descriptor 与实际 painter/数据；用于“主题鱼”等运行时扩展 | 注册才出现、卸载即消失、按会话保留选择；不将当前 profile 扩展写成官方固定页签 |

这些契约仍须按仓库规程进入 [`../DSH-GO/docs/transport-contract.md`](../../DSH-GO/docs/transport-contract.md)，先同步双仓 snapshot/golden，再各改实现。本文件只是缺口审计，未替契约 owner 冻结字段。

## 5. Lead 集成顺序与明确降级

1. **并行开工**：FG-1/2/3/4/5 各自插件与纯逻辑测试；共享 `pluginapi`、apiclient、快照和 Transport 不并行写。
2. **串行集成**：FG-1 先落 caption / shell geometry 与右栏 glue；Lead 再接 panel/view 路由、composer actions、stats fields；最后做同一真宿主联合验收。
3. **没有真值时的统一画面**：品牌、布局、既有标题/会话/模型/权限可以画；团队数、缓存率、上下文百分比、附件状态、自动化任务不画。不得用截图里的 `3 个子智能体`、`97%`、`34%` 等当前会话值做固定 mock 冒充产品。
4. **低优先隔离**：设置、账号、登录/充值、更新、快捷键说明不占这五包 owner 的时间；待上述高可见结构闭合后再按真实工作流排期。
5. **证据边界**：本审计只新建本文档，未改源码、未构建、未启动/终止进程、未 commit/push。后续包的截图只证明对应几何/状态，不替代真宿主行为验收。
