# 设置·内置插件页（#29 / #42 / #43）的结构证据与实现台账

> 建立时间：2026-11-14 · 对应 `docs/official-parity.md` 第 29 / 42 / 43 行。
>
> **这份文件回答三个问题**：
> 1. 官方 rc.2 的「内置插件」页到底是什么结构（组件层级 / slot / CSS 几何 / locale 文案）？
> 2. 本仓实现落在哪些 `file:line`，哪些值是从官方原文学来的？
> 3. 哪几处**没做**，各自缺的是什么（HIDE 台账）。

**证据口径**：rc.2 起官方**随包发 `src/**` 原件**（未压缩 TS/TSX + CSS Module + 全语言 locale），
比任何二手结构文档都强。锚点记法 `OFF:<包简称>/<src 内相对路径>:<行号>`，
`OFF` 根 = `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`
（gitignored 的本机解包树，与主库 `$DSH_HOME/dsh-ui-official/` 互为参照）。
宿主侧锚点记 `HOST:`，根是宿主仓 `origin/main`。

---

## 0. 三个包各是什么（先分清面，避免把三件事混成一件）

| # | 官方包 | 它负责的**面** | 本仓结论 |
|---:|---|---|---|
| 29 | `dsh-client-ui-plugin-manager` | **侧栏「插件」主面板页**：安装 / 更新 / 启停 / 卸载 / 版本 / 详情 / 配置页（17 文件，`src/client/PluginManagerPage.tsx` 为主） | **PARTIAL**（本轮不闭合：见 §4） |
| 42 | `dsh-client-ui-settings-plugin-inventory` | **设置·内置插件**里的**只读清单页**（5 文件 / 580 行 tab + 512 行 CSS） | **IMPLEMENTED** |
| 43 | `dsh-client-ui-settings-plugins` | **设置·内置插件**的**外壳**：导航项、标题、简介、`settings.plugins.tab` tab 条 | **IMPLEMENTED** |

> 关键事实（本轮纠正了审计里一句不成立的描述）：
> **rc.2 里 `settings.plugins.tab` 只有一个贡献方**——就是只读清单
> （`OFF:ui-settings-plugin-inventory/src/client/index.ts:56-63` 注册 `id: 'all', order: 10`）。
> 全仓 grep `settings.plugins.tab` 的注册点只有这一处
> （其余命中是 `ui-settings` 的契约声明、`ui-settings-plugins` 的台账读取、
> `cordis-client-runner` 的 slot 目录示例）。
> 所以官方 rc.2 的 plugins 设置页**就是单 tab、不画 tab 条**（`PluginsSettingsSection.tsx:57-58`），
> **不存在**「内置/第三方分 tab」这个东西——审计 #43 的那句描述是 rc.1 口径的残留。

---

## 1. #43 外壳：结构与几何（逐条）

官方组件（`OFF:ui-settings-plugins/src/client/PluginsSettingsSection.tsx:33-118`）：

```
div.section
├─ h2.heading                    t('title')
├─ p.intro                       t('intro')
├─ tabs==0 ? p.empty : tabs==1 ? div.panel{ slot('settings.plugins.tab', only) }
                            : (div.tabs[role=tablist] + tabpanel 组)
```

CSS（`PluginsSettingsSection.module.css`，逐行）：`.section{gap:12px,max-width:760px}` (:3-9) /
`.heading{font-size:18px;font-weight:600}` (:11-15) / `.intro{font-size:13px;color:tertiary}` (:17-21) /
`.tabs{gap:22px;border-bottom:.5px;margin-top:2px}` (:23-29) /
`.tab{padding:7px 1px 9px;font-size:13px;line-height:20px}` (:31-41) /
`.tab[data-active]::after{height:2px;bottom:-1px}` (:53-63) / `.panel{padding-top:2px}` (:72-75)。

locale 中文（`OFF:ui-settings-plugins/src/client/locales.ts:16-22`，逐字）：
`nav 内置插件` / `title 内置插件` / `intro 查看内置部署的插件列表` / `tabs 插件视图` /
`empty 本部署没有开放任何插件视图。`

**本仓落点**

| 项 | 落点 | 说明 |
|---|---|---|
| 导航项 | `plugins/dssettings/official_tree.go:83`（`ID: "plugins", Order: 15, Title: "内置插件"`；Order/Title 逐字对齐 `index.ts:76-84` 的 `order:15` + `t('nav')`） | 本轮未改（本来就对） |
| 标题 / 简介 | `plugins/dssettings/official_plugins_draw.go`（`drawPluginsPage`：`section.Title` + `zhPluginsIntro`） | 文案常量 `official_plugins.go` |
| tab 台账 | `plugins/dssettings/official_plugins.go` 的 `PluginsTab` / `DefaultPluginsTabs()` / `sortedPluginsTabs` / `activePluginsTab` | 按 `order` 升序、未知 id 回退第一项，与官方 `:39,:58` 同判据 |
| tab 条 | `drawPluginsTabs`：`len(tabs)>1` 才画；选中态 2px 下划线 | 单 tab（官方 rc.2 的真实形态）时**不画** |
| 几何 | `official_geometry.go` 的 `PluginsSettingsSection.*` 行（8 行） | 全部 `aligned`，锚点都是上面那份 CSS 的行号 |

---

## 2. #42 只读清单页：结构、数据、几何

### 2.1 数据结构（官方 typert Remote）

`OFF:dsh-host-plugin-inventory/src/types.ts:16-77`（`PluginInventoryEntry` / `PluginInventorySnapshot`）
+ `src/index.ts:50-121`（`pluginInventory.list` 的真实现：读离 `ctx.loader.entries()` 的**非 group** 条目）。

宿主侧（`HOST:internal/transport/handlers_plugin_inventory_official.go:26-69`，
适配层 `HOST:cmd/dshd/serve_plugins_official.go:54-74`，条目生成 `:188-225`）：

```jsonc
{ "managementAvailable": true,
  "entries": [ { "entryId": "<bundle name>", "moduleName": "<manifest entry>",
                 "enabled": true, "fiberPhase": "active" | null, "diagnostic": "…"? } ] }
// meta **不产出**（宿主插件包没有官方 title/description/icon）
// agentPresets **不产出**（本仓 modes 不是 Cordis 插件组合 roster）
```

客户端：`internal/apiclient/plugin_inventory.go`（逐字段对齐官方类型；`meta.title` 的
`string | {en,[locale]}` 两种形态都认，中文优先）；契约快照
`testdata/api.snapshot.json` 的 `pluginInventory.list`（batch 8）。

### 2.2 组件结构与默认态

`OFF:ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.tsx`：

| 行为 | 官方行号 | 本仓落点 |
|---|---|---|
| 状态机 loading / error / ready | :48-51,443-461 | `PluginsState`（另有 `unknown`：宿主没接这条能力，官方没有这个态，本仓必须有） |
| 搜索过滤（模块名/entryId/标题/简介四字段小写包含） | :102-108,464-474 | `pluginMatches` |
| 分段：会话插件（presets）+ 全局插件 | :478-575 | `PluginGroup`；宿主不产出 presets -> 只有全局段 |
| 全局段默认**收起**，搜索强制展开 | :315-317 | `BuildPluginsSection` 的 `globalOpen` |
| 会话段默认展开 | :316 | 未实现（见 §4-H1） |
| 排序：`failed` 先，其余 Loader 顺序 | :295-301 | `BuildPluginsSection` 的两段遍历 |
| 卡片手风琴（同时只开一张） | :140,325-327 | `pluginsExpanded` + `PluginCard.Expanded` |
| 卡片：标题 / 简介 / entryId 角标 / chevron | :123-178 | `drawPluginCard` |
| 标题回退：`meta.title` 缺席 -> 短模块名 | :69-76,93-100 | `pluginShortName` / `pluginTexts` |
| 角标省略：剥 `include:` 后与标题相同则不画 | :83-86 | `pluginEntryChip` |
| 状态标签只在偏离默认态时画 | :236-249 | `PluginCard.Tag`（enabled 不画） |
| 阶段点只给 pending/loading/unloading | :206-234 | `pluginPhaseShowsDot` |
| 详情 `CardFacts`：完整名称 + 配置状态 (+ 运行状态) | :180-204,409-422 | `drawPluginDetails` / `drawPluginFact` |
| 空清单 `暂无插件。` / 无命中 `没有匹配的插件。` | :475-476 | `PluginsPanel.Empty` / `EmptySearch` |
| 失败 `暂时无法读取插件。` + 重试 | :454-461 | `ShowRetry` -> `Act.PluginInventoryRefresh` |

locale 中文（`OFF:ui-settings-plugin-inventory/src/client/locales.ts:4-42`，逐字落成常量）：
`tab 插件列表` / `loading 正在读取插件…` / `error 暂时无法读取插件。` / `retry 重试` /
`search 搜索插件` / `empty 暂无插件。` / `emptySearch 没有匹配的插件。` /
`metadataError 包元信息错误：{error}` / `presetTitle 会话插件` / `presetSubtitle 由 Agent 预设按会话组成` /
`globalTitle 全局插件` / `globalSubtitle 系统与所有会话共用` / `enabledTag 已启用` / `disabledTag 已停用` /
`conditionalTag 条件启用` / `presetEnabledTag 预设中启用` / `failedTag 启动失败` /
`moduleLabel 完整名称` / `fromPreset 来自` / `condition 禁用条件` / `configuration 配置状态` /
`runtime 运行状态` / `unobserved 未运行` / `pending 等待依赖` / `loadingPhase 加载中` /
`active 运行中` / `failed 启动失败` / `unloading 卸载中`。

### 2.3 几何（官方 CSS -> 本仓常量，全部 `aligned`）

`OFF:ui-settings-plugin-inventory/src/client/PluginInventorySettingsTab.module.css`：
`.section{max-width:760,gap:14}` (:2-10) / `.catalog{gap:12}` (:47-51) /
`.search input{height:36,padding:0 34 0 36,font-size:13}` (:67-78) / `.search>svg{left:12}` (:61-65) /
`.cards{gap:10,grid-template-columns:repeat(2,minmax(0,1fr))}` (:89-96) /
`@container(max-width:520px){单列}` (:501-505) / `.cardContent{padding:12px 14px,min-height:52,gap:2}` (:124-140) /
`.cardMainRow{gap:12}` (:163-169) / `.cardTitle{14/20}` (:178-184) / `.cardDescription{12/18}` (:186-196) /
`.cardMeta{padding-top:6}` (:204-208) / `.cardIdentity{12/18,padding:1px 6px}` (:213-228) /
`.cardTrailing{gap:8}` (:230-236) / `.group{gap:10}` (:245-249) /
`.groupTitleRow{gap:8,min-height:36}` (:253-258) / `.group+.group{padding-top:14}` (:260-263) /
`.groupTitle{14/22}` (:296-301) / `.groupSub{margin:-6 0 0 20;12/18}` (:303-312) /
`.groupBody{gap:10}` (:359-363) / `.cardDetails{padding:10px 14px 12px}` (:437-441) /
`.details{grid-template-columns:76px 1fr;gap:6px 10px;margin-top:8}` (:449-454) /
`.details dt{11/17}` (:460-464) / `.details dd{12/17}` (:466-473) /
`.status{13/20}` (:17-22) / `.brokenNote{padding:8px 10px;12/18}` (:366-376) /
`.failure button{border:.5px,padding:4px 10px}` (:37-45)。

**本轮顺手修掉了 4 行错的几何事实**（上一轮凭印象写的，锚点指错了行）：

| 行 | 旧值 | 官方实际 | 依据 |
|---|---:|---:|---|
| `.groupTitle` font-size | 13 | **14** | CSS:296-301 |
| `.groupTitle` line-height | 20 | **22** | CSS:296-301 |
| `.group` gap | 8 | **10** | CSS:245-249 |
| 单列断点 | 640 | **520**（且是 `@container`，不是视口断点） | CSS:501-505 |
| `.card` padding 锚点 | `:101`（`.card` 本身没有 padding） | 值 12 对、锚点改为 `.cardContent` `:124-140`，并补 `padding-inline 14` | CSS:124-140 |

### 2.4 真实数据路径（这是"不是空壳"的判据）

| 层 | 落点 | 证据 |
|---|---|---|
| 宿主方法 | `HOST:internal/transport/handlers_plugin_inventory_official.go:29`（`MethodPluginInventoryList = "pluginInventory.list"`）、`:89-97` | 宿主 origin/main 已实现 |
| 客户端调用 | `internal/apiclient/plugin_inventory.go` `PluginInventoryList` | `internal/apiclient/plugin_inventory_test.go`（零参数裸对象 / null 与缺席不同 / 501 不吞） |
| 壳缓存 | `cmd/app/plugin_inventory_wiring.go`（同步缓存 + 首次问触发后台拉取 + 失败保留上次 + `Invalidate`） | — |
| 动作面 | `pluginapi.Actions.PluginInventory` / `.PluginInventoryRefresh`；字面量接线在 `cmd/app/main.go` | `cmd/app/actions_wired_test.go` 强制"插件用到的每个 Action 壳都必须接线"（本轮 44 用 / 45 接） |
| 端到端 | `cmd/app/plugin_inventory_e2e_test.go`：httptest 宿主按**真 wire 形状**应答 -> 快照 -> 面板卡片，三态（有清单 / 501 / 空清单）全钉 | `TestPluginInventoryChainRealHost` 断言页面上真的有 `plugin.wasm` / `已启用` / `已停用` / `运行中` |
| 契约快照 | `testdata/api.snapshot.json` 新增 `pluginInventory.list`（batch8=1，total 50->51） | `TestSnapshotCoversEveryCalledMethod` / `TestSnapshotCountsMatchMethods` |
| 渲染 | `plugins/dssettings/official_plugins_draw.go` | `plugins/dssettings/official_plugins_test.go` 12 条 |

---

## 3. 与官方的**有意差异**（登记，不掩盖）

| 差异 | 官方 | 本仓 | 理由 |
|---|---|---|---|
| 图标 | sprite `IconChevronDownOutlineRegular` / `IconSearchOutlineRegular`（TSX:7-8,164,465） | 文字字形 `▾`/`▸`/`⌕` | 本仓没有该 sprite 的导出；仓规禁止手绘冒充官方图标（与 `dssidebar` 搜索框同处理） |
| 卡片 focus 环 | `:focus-visible{outline:focus-ring}` | 换成品牌色描边 | Viva 没有 CSS outline 层 |
| danger/info 标签底色 | `Tag tone="danger"` 用 error 容器色 | `BgLayer3` 底 + `Error` 文字 | 本仓 `Theme` 只有 success/warn 的 tertiary 容器色，没有 error/info 的 |
| failed 卡描边 | `color-mix(error 45%, transparent)` | 纯 `Error` 描边 | 无混色 API |
| tab 台账 | cordis slot registry（跨插件贡献） | 包内有序台账（`PluginsTab` + `DefaultPluginsTabs`） | 本仓设置面本来就是壳内固定 section 表（`#37` 已登记），台账是这套架构里的等价物 |

---

## 4. HIDE 台账（每条都写清"缺什么"，不是"画不出来"）

| ID | 没做的东西 | 缺的能力/证据 | 现状表达 |
|---|---|---|---|
| **H1** | 会话插件段（`presetTitle` / `presetSubtitle` / preset switcher / `enabledIn` / `matchesInOtherPresets` / `condition` / `fromPreset`） | **宿主不产出 `agentPresets`**（`HOST:handlers_plugin_inventory_official.go:20-23` 明确写理由：本仓 modes 不是 Cordis 插件组合 roster） | 段不画（官方同样只在 roster 存在时画）；万一宿主将来产出，页面显示 `PresetGap` 自检文案，不静默少一段 |
| **H2** | 可配置管理页（`plugins.tab` 的第二条 tab） | 归 **#29**：需要 `pluginManager.*` 的写动作（宿主已有 `installBundle`/`updateBundle`/`setBundleEnabled`/`removeBundle`/`listVersions`/`readBundleDetail`/`listBundles`/`listPlugins` 与三个 install 进度流），本仓 GUI 一条都没接 | tab 台账里**不注册**死 tab；测试里注册一条会走 `TabMissing` 明确文案 |
| **H3** | loading 骨架卡（4 张 `skeletonCard` + pulse 动画，CSS:399-422,485-499） | 壳的清单缓存形状区分不了"宿主没接"与"首次拉取在飞"（`PluginInventory` 同步返回缓存，两种都表现为 nil）；官方那两态是 React 的 Promise 状态 | unknown 态给文案 + 重试（对两态都是正确下一步） |
| **H4** | `clientSync` 段（`clientSyncing` / `clientSyncFailed` / `clientSyncRetry`，TSX:429-442） | 官方读的是**浏览器端模块图**（`ctx.modules.entries.state`，动态加载 JS bundle 的同步状态）。本仓插件是编进二进制的 Go 包，没有"本页面的插件同步"这件事 | 整段不画（画了就是凭空捏一个状态） |
| **H5** | tab 条键盘导航（ArrowLeft/Right/Home/End，官方 :77-91） | 多 tab 才需要；产品当前单 tab（官方 rc.2 也单 tab），无第二个 tab 可走 | 单 tab 下不可达；H2 闭合时一并补 |
| **H6** | 段标题键盘展开（官方是真 `<button>`，可 Tab 聚焦回车） | 本仓面板的键盘模型是"↑↓ 在卡片间移动 + Enter 展开卡片"，段头只有鼠标点击 | 已登记；不影响鼠标路径 |
| **H7** | `meta.icon` 渲染 | 官方清单页**也不渲染** icon（TSX 只读 title/description/error），宿主也不产出 meta | 无需实现（不是规避） |

---

## 5. 计数与结论

- **#42** `settings-plugin-inventory`：PARTIAL -> **IMPLEMENTED**（只读清单页在真数据路径上；H1/H3/H4 是官方那份页面的三个分支，各有登记）。
- **#43** `settings-plugins`：PARTIAL -> **IMPLEMENTED**（nav/title/intro/tab 台账/单 tab 语义/几何全部对齐；"分 tab"在 rc.2 不存在，见 §0 的纠正）。
- **#29** `plugin-manager`：**仍 PARTIAL**。它的面是侧栏主面板的管理页（安装/更新/启停/卸载/版本/详情），
  与本轮的设置清单页**不是同一个东西**；本轮把"设置侧那条链"接通并把宿主已有的写方法列清（H2），
  但没有接任何一条写动作——按仓规不得因此上调状态。

分母口径：`docs/evidence/official-ui-plugins-0.2.2-rc.2.txt` 的 66 行，脚本 `scripts/parity-count.py --check` 动态核对。
本轮合计变化：✅ 28 -> 30，🟡 21 -> 19（总数仍 66，零缺零多）。
