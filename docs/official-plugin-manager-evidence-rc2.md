# 插件管理页（#29 `dsh-client-ui-plugin-manager`）的结构证据与实现台账

> 建立时间：2026-11-15 · 对应 [official-parity.md](official-parity.md) 第 29 行（🟡 PARTIAL → ✅ IMPLEMENTED）。
>
> **这份文件回答四个问题**：
> 1. 官方 rc.2 的 `dsh-client-ui-plugin-manager` 是什么结构（组件层级 / slot / CSS 几何 / locale 文案）？
> 2. 官方消费哪几个 `pluginManager.*` 方法、数据形状是什么？
> 3. 本仓实现落在哪些 `file:line`，哪些值是从官方原文学来的？
> 4. 哪几处**与官方不同 / 没做**，各自缺的是什么？

**证据口径**：rc.2 起官方**随包发 `src/**` 原件**。锚点 `OFF:<包简称>/<src 内相对路径>:<行号>`，
`OFF` 根 = `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`；
宿主侧锚点 `HOST:`，根是宿主仓 `origin/main` 的干净导出（本仓 `tmp/GO8-RC2/_audit-tree`）。

---

## 0. 官方包是什么（一个侧栏入口 + 一个主区页面）

| 文件 | 作用 |
|---|---|
| `OFF:pm/src/client/index.ts`（175 行） | 注册 locale、`sidebar.panellist` 的 **Plugins** 入口（id `plugins`、label `t('panel')`、order 0）与 `main` 面板；`pluginChanges` / `pluginNavigation` 两个跨插件服务 |
| `OFF:pm/src/client/PluginManagerPage.tsx`（1926 行） | 列表页（页头 + 分组卡片 + 开关）、详情页（面包屑 + 事实 + 行 + 文档）、安装对话框、更新对话框、卸载确认、toast |
| `OFF:pm/src/client/PluginManagerPage.module.css`（2318 行） | 全部几何（本文 px 值逐条注行号） |
| `OFF:pm/src/client/manager-store.ts`（1581 行） | 页面状态机：`packages` / `busy` / `install` / `update` / `confirm` / `notice` / 版本检查 |
| `OFF:pm/src/client/locales.ts`（2385 行） | 8 种语言；zh 是本文文案的唯一来源 |
| `OFF:pm/src/client/presentation.ts`（189 行） | `managementText` / `packageText` / `noticeText`：错误码与动作失败→官方句子的映射表 |
| `OFF:pm/src/client/BundleDetailSection.tsx` | 详情页的「详细介绍」段（`DetailDocument` 渲染宿主 markdown） |

**入口归属**：官方页面同时注册 `main`（主区）与 `sidebar.panellist`（侧栏入口）——
本仓的对应物是 `plugins/dssidebar/frame.go` 的 `panelRoutes`（id `plugins`、label `插件`、
order 0）与 `drawActivePanel` 的主列落地，**入口三要素逐字一致**，这一版只换了页面本体。

---

## 1. 页面结构（官方）

```
.page                                     OFF:PluginManagerPage.module.css:9-36（--plugin-page-inset clamp(24,4vw,48)；max-width 960）
├─ .pageBar > .pageHead                   :44-160/.162-181（padding-top:28px）
│  ├─ .pageTitle 20/28/500                :188-193
│  ├─ .pageIntro 13/20 label-secondary    :195-203
│  └─ .toolbar（AddPluginMenu / 刷新）     :213-224（TSX:60-108）
├─ .pageBody（滚动）                       :64-86（gap:32px；padding-bottom:48px）
│  ├─ 状态行 .status/.failure/.empty       :256-276
│  └─ section.group                        :278-296
│     ├─ .groupTitle 14/22/500（bundlesTitle / basicTitle / extensionsTitle）
│     └─ ul.cards（gap:2）                 :367-378
│        └─ li.card > .cardHead（padding:8；gap:14）  :379-412
│           ├─ .cardIcon 48×48（0.5px border-l3）      :398-411
│           ├─ .cardMain（gap:4）→ .titleRow（gap:8）+ .cardDesc 13/18
│           └─ .cardEnd（gap:8）→ Switch 36×20
└─ 详情 .detail                            :1409-1428
   ├─ .detailTop > .crumb（12.5px，chevron rotate 90°）  :1428-1465
   ├─ .detailMain（margin-top:20；gap:8）：标题 20/28/500、版本标签、名称 12/18、简介 14/22
   ├─ .detailSections（gap:32；margin-top:32）            :1521-1532
   │  ├─ .detailSection（gap:12）+ .sectionTitle 14/20/500
   │  └─ .facts div（min-height:44；padding:8 2；底部 0.5px border-l2）  :1554-1598
   └─ .rows/.row（bundle patch 行；本仓恒空）
```

**分组由数据决定**：`mine = listed.filter(!official)`、`official = listed.filter(official)`、
`basicCards` 来自 `plugins.item` 槽贡献方（TSX:1670-1700）。本仓宿主 `listBundles`
只有 profile bundle（`official=false`），且没有任何 `plugins.item` 贡献方 →
官方此时只渲染 `renderGroup('bundles', t('bundlesTitle'), mine.map(packageCard))`
（TSX:1846-1852），tab 条在 `tabs.length === 0` 时整段不画（TSX:1789-1795）。

---

## 2. 数据面：官方 `pluginManager.*`（15 个方法）→ 本仓真接的 4 个

| 方法 | 官方形状 | 本仓宿主 | 本仓客户端 |
|---|---|---|---|
| `listBundles` | `BundleInfo[]`（`HOST:handlers_pluginmanager_official.go:655`） | 真（`serve_plugins_official.go:148-176`） | `internal/apiclient/plugin_manager.go` `PluginBundles` |
| `readBundleDetail(name,language)` | `{status:found\|none,origin,path,markdown}` | 真（`HOST:internal/pluginmgr/official/adapter.go:607-625`） | `PluginReadBundleDetail` |
| `setBundleEnabled(name,enabled)` | `ChangeResult`（`HOST::536`） | 真（`serve_plugins_official.go:105-120`） | `PluginSetBundleEnabled` |
| `removeBundle(name)` | `ChangeResult` | 真（`serve_plugins_official.go:122-130`） | `PluginRemoveBundle` |

**本仓 bundle 的真实形状**（`HOST:cmd/dshd/serve_plugins_official.go:152-176`）：
`official=false`、`availability="profile"`、`installed=true`、`removable=true`、
`updatable=false`（不产出 `update`）、`rows=[]`、`overrides=[]`、`source` 是本地目录 /
`link:` spec、`version` 来自 `state.json`。**没有** `meta` / `installTarget` /
`listedByDefault` / `readOnlyReason`——界面不画它们对应的行。

**数据流**（与任务页同一条先例）：

```
pluginManager.listBundles ──▶ cmd/app/plugin_manager.go（缓存 + 动作后重拉）
   └─ Actions.PluginBundles ─▶ plugins/dssidebar/plugins_page.go（列表/详情渲染）
pluginManager.readBundleDetail ─▶ Actions.PluginBundleDetail ─▶ 详情页「详细介绍」段
pluginManager.setBundleEnabled / removeBundle ─▶ Actions.PluginSetBundleEnabled / PluginRemoveBundle
   └─ 失败 ─▶ Actions.PluginActionError(action,name) ─▶ 页顶一行官方文案
```

**刷新边界**：本仓 Transport **没有** `plugin-manager/changed` 推送（官方 `index.ts` 的
`ctx.remote.$on` 一族）→ 改为「打开面板时拉一次、每个成功动作后重拉、失败给重试」。
这是**已登记的差异**，不是遗漏。

---

## 3. 本仓实现（file:line）

| 层 | 落点 | 说明 |
|---|---|---|
| 客户端 | `internal/apiclient/plugin_manager.go` | 4 个方法 + `BundleInfo`/`ChangeResult`/`BundleDetail` 形状；`Title()`/`DescriptionText()` 是官方 `packageText` 的回退链；`Failed()`/`ErrorCode()` 把 `application='failed'` 的管理性失败暴露出来 |
| 契约快照 | `testdata/api.snapshot.json`（batch13） | `counts.total` 64 → **68**，各批之和 = total（守卫 `TestSnapshotCountsMatchMethods`） |
| 壳数据面 | `cmd/app/plugin_manager.go` | 缓存 + 重拉 + 两条写动作 + 失败动作名；501 → `unavailable` 单独记一位 |
| 壳接线 | `cmd/app/main.go`（Actions 字面量） | 7 个回调 |
| 插件契约 | `pluginapi/plugin.go`（`Actions`） | `PluginBundles` / `PluginBundlesRefresh` / `PluginBundleDetail` / `PluginBundleDetailRefresh` / `PluginSetBundleEnabled` / `PluginRemoveBundle` / `PluginActionError` |
| 页面 | `plugins/dssidebar/plugins_page.go` | 列表页 + 详情页 + 开关 + 卸载（二次点击确认）+ 三段（详细介绍/包含的组件/来源信息） |
| 路由 | `plugins/dssidebar/frame.go` | `drawPluginsPanel` 改为委托 `drawPluginManagerPage`；入口 id/label/order 不变 |
| 测试 | `plugins/dssidebar/plugins_page_test.go`、`internal/apiclient/plugin_manager_test.go` | 状态映射、文案映射、不乐观更新、两帧真画（卡片/unavailable） |

**文案**：全部取 `OFF:pm/src/client/locales.ts` 的 zh 段（`title`/`intro`/`loading`/`error`/
`unavailable`/`retry`/`empty`/`bundlesTitle`/`statusProblem`/`statusNotRunning`/`versionTag`/
`sourceTitle`/`sourceSpec`/`sourceVersion`/`sourceBuiltIn`/`partsLabel`/`partsEmpty`/
`backToList`/`crumbRoot`/`uninstall`/`reasonLabel`/`detailTitle`/`detailLoading`/`detailFailed`/
`confirmUninstallTitle`/`confirmUninstallDescription`/`confirmUninstall`/`cancel`/`close`/
`failedEnable`/`failedDisable`/`failedUninstall` 与 `reason*` 段）。
**错误码→句子的映射**照 `OFF:pm/src/client/presentation.ts:31-95` 的 `CODE_KEYS` /
`managementText` 逐条移植（`plugins_page.go` 的 `pluginReasonText`）。

**几何**：`plugins_page.go` 顶部的常量块逐条注官方 CSS 行号；Switch 36×20/旋钮 16×16
沿用本仓 `plugins/dssettings/official_panel.go:555-566` 的既有画法（官方 `Switch` 的尺寸）。

---

## 4. 与官方不同 / 没做（各自缺什么）

| 项 | 官方 | 本仓 | 为什么 |
|---|---|---|---|
| **添加插件**（InstallDialog） | 页头主按钮 + `plugins.add.actions` 菜单 + 规格输入 + 检查 + pnpm 终端块 + 进度 + 取消 + 构建审批（TSX:1034-1535） | **HIDE** | 本仓没有 pnpm 终端流、构建审批与 registry 选择面；宿主 `installBundle` 对 JS/Cordis 形态是结构化失败（`HOST:cmd/dshd/serve_plugins_official.go:232-254`）。画半个对话框就是自造交互 |
| **更新**（UpdateDialog / 版本列表） | `update` 三态 + `listVersions` + registry 检查 | **HIDE** | 本仓 `listBundles` 恒不产出 `update`；`listVersions` 对本仓本地目录包回 `not-updatable`（没有 registry 版本源，`HOST:internal/pluginmgr/official/adapter.go:135-163`） |
| **多 tab**（`plugins.tab`） | 贡献方注册的 tab 条 | 不画 | 没有任何贡献方；官方也是"零贡献方不画 tab 条"（TSX:1789-1795） |
| **官方插件配置卡片**（`plugins.item` / config 槽） | 官方包的配置页 | 不画 | 本仓槽位表里没有这 9 个 `plugins.*` 槽，零贡献方 |
| **卸载确认** | `Modal` + 两个按钮 | 行内确认条 + 「同一按钮再点一次」 | 本仓按仓规不新造 Modal；与侧栏删除会话、任务页删除同一先例 |
| **详细介绍渲染** | `DetailDocument` 的 Markdown 布局（代码块/图片） | **纯文本逐行**画宿主返回的真实 markdown | markdown 布局要走 `Draw`+`AfterDraw` 两段并使用 `components.MarkdownView`（dsconversation 那套），本轮不引入这条链；内容不假，只是样式不同 |
| **卡片图标** | `PluginArtworkDefault` 精灵图 / manifest icon data URL | 48×48 边框框 + 标题首字（**derived**） | 本仓没有官方 sprite 导出、宿主也不产出 `meta.icon`；按仓规不手绘冒充官方图标 |
| **卡片点击面** | 标题按钮 `::after` 铺满整张卡 | 只有标题文字可点开详情 | viva 没有"绝对定位铺满"的扩展层；开关必须保住自己的命中区 |
| **`restart-required` / `overridden` / `missing` 三态** | 有 | 不画 | 宿主不产出（`HOST:handlers_pluginmanager_official.go:22-32` 明写只取 applied/cancelled/failed） |
| **dangle 提示 / toast** | `Toast` 组件 | 页顶一行、8 秒自清 | 与任务页同一先例；文案仍取官方 `failed*` 三句 |

---

## 5. 证据复验命令

```bash
# 官方页面结构（行号可解析）
wc -l tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/dsh-client-ui-plugin-manager/src/client/PluginManagerPage.tsx
# 宿主 15 个方法常量 + 生产装配
grep -n 'MethodPluginManager' tmp/GO8-RC2/_audit-tree/internal/transport/handlers_pluginmanager_official.go | head
grep -n 'AttachOfficialPluginManagerAPI' tmp/GO8-RC2/_audit-tree/cmd/dshd/serve.go
# 本仓
python scripts/parity-count.py --check
go test ./plugins/dssidebar/ ./internal/apiclient/ -run 'Plugin|Snapshot' -count=1
```
