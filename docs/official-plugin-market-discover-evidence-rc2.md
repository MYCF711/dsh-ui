# `#30 dsh-client-ui-plugin-market` · A 块与 B 块证据（rc.2）

> 日期：2026-11-15。范围：**A 块**（`plugins.tab` 的 Discover tab 骨架 + 索引源增删）与
> **B 块**（搜索框 + 分类/标签/排序筛选工具栏 + `entries[]` 全字段结果列表）。
> C 块（条目详情 markdown + 版本列表）上一批已落地，证据见
> [official-plugin-market-detail-evidence-rc2.md](official-plugin-market-detail-evidence-rc2.md)。
>
> 前置：宿主 PR `MYCF711/dsh-go#46`（分支 `feat/host-market-shape`）把
> `pluginMarket.search` / `listIndexes` / `setIndexes` / `pluginRegistryProbe.fastest`
> 从结构化 501 改成真实现，结果形状对齐官方（详见 §1）。
>
> 官方源码：`tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/dsh-client-ui-plugin-market/src/**`
> （rc.2 随包发 `src/`，所以下面的行号能直接对上）。

---

## 1. 数据面：宿主 → apiclient（三个方法）

| 方法 | 官方契约 | 宿主实现 | 客户端 |
|---|---|---|---|
| `pluginMarket.listIndexes` | `MarketIndexSummary[]`，**无参数**（`OFF:market-store.ts:295-299`） | `HOST:internal/transport/handlers_plugin_market_official.go:337-351` + `HOST:cmd/dshd/serve_plugin_market.go:94-107`；真实现是 `HOST:internal/market/store.go:220-249` 的持久化表 + 逐源抓文档 | `internal/apiclient.PluginMarketListIndexes`（`plugin_market_discover.go`）。title/description 是 `string|localized` union，复用 `PluginLocalizedText` |
| `pluginMarket.search` | `search(url, query)` → `MarketSearchResult`（`OFF:market-store.ts:381`、schema `dsh-plugin-market/lib/typert.host.js`） | `HOST:handlers_plugin_market_official.go:370-390`（平铺 `url` + `query`；sort/offset/limit 必填、limit 1..100、未知键 400）+ `HOST:cmd/dshd/serve_plugin_market.go:142-190` | `internal/apiclient.PluginMarketSearch`；结果字段与官方同名，`nextOffset` 末页**显式 null** |
| `pluginMarket.setIndexes` | `setIndexes(urls)` → `void`（`OFF:market-store.ts:405`） | `HOST:handlers_plugin_market_official.go:353-369`（读 `urls`；非法 URL 400 `bad_request`）+ `HOST:cmd/dshd/serve_plugin_market.go:112-121` | `internal/apiclient.PluginMarketSetIndexes`（结果 `{}`，不取载荷） |

失败投影与官方客户端同判据：

- 列表读：只有 `fetch` / `changed` 两档可达（索引非法也回 `index_unavailable`，见 §5 缺口 2）。
- 索引源写回：`bad_request` → `invalid`；其余 → `other` + 宿主原文（`duplicate` / `unavailable` 宿主产不出）。

`pluginRegistryProbe.fastest` **不调**：官方客户端只拿它决定 `npmFileSources`
（`OFF:market-store.ts:270-274`），而宿主 `readEntryDetail` 不读那个选项（C 块已登记）。

---

## 2. A 块：tab 骨架 + 索引源增删

### 2.1 座位：`plugins.tab` 的 `discover`

官方就是这个包注册的 tab：

```
OFF:../dsh-client-ui-plugin-market/src/client/index.ts:55-57
     ctx.slots.register({name:'plugins.tab', id:'discover', label: () => t('title'), …}, DiscoverSection)
OFF:../dsh-client-ui-plugin-manager/src/client/PluginManagerPage.tsx:1732-1749
     页面页头画 tab 条：第一格永远是 installed，其后是 plugins.tab 的贡献方
OFF:PluginManagerPage.tsx:1577-1580
     选中的贡献方离开台账 → 落回 installed
OFF:PluginManagerPage.tsx:1750-1760
     贡献方内容画在页头 + tab 条下面的 tabpanel 里
```

本仓落点（数据 → 容器 → 渲染）：

```
dsmarket.Plugin.Register
  → ctx.Inject(pluginapi.KeyPluginsTabs)          （Plugins 页属主 dssidebar 提供）
  → ctx.Provide(ui.market.discover, *Discover)    （tab 本体）
  → ctx.Provide(ui.market.detail,   *Page)        （C 块详情页，A 块挂进列表的点击路径）
  → tabs.RegisterPluginsTab(p.discover)
dssidebar.Plugin.Register
  → ctx.Provide(ui.plugins.tabs, pluginapi.PluginsTabs(p))
dssidebar.drawPluginManagerPage
  → activePluginTab()：台账里没有就落回 installed（官方同判据）
  → drawPluginTabBar()：`ui.Tabs`，installed 在前、discover 在后；台账为空整段不画
  → drawDiscoverTab()：页头 + tab 条带页内距；tab 体交给 *Discover（自带滚动/内距）
```

为什么要服务键而不是互相 import：本仓插件只经服务键耦合（`$DSH_HOME/AGENTS.md`
的 R2 同源规矩，`pluginapi/plugin.go` 顶部注释写着）。**反方向**也做了限制：
侧栏**不** `Requires` 市场插件（否则只有 `ds-core + 侧栏` 的最小装配会起不来），
而是由`ds-market` 在自己的 `Requires` 里要求 `ui.plugins.tabs`——最小装配少一个
tab，生产装配一个不少。

### 2.2 索引源列表 + 增删

| 官方 | 本仓 |
|---|---|
| 索引菜单（`DiscoverSection.tsx:493-511`）+ 底部「管理索引源…」（`:504-507`） | `Discover.drawMarketMenu`（`plugins/dsmarket/discover.go`）：`ui.Menu`，选项 = `listIndexes` 的每个源（`indexLabel`：标题 → host → 原样 URL，`OFF:presentation.ts:106-115`） |
| 索引源对话框（`:318-350`）：标题 / 说明 / 已添加列表 / URL 输入框 + 添加 / 信任提示 | `Discover.drawSourcesDialog`：`ui.ModalWith`（宽 480，官方 `.dialog{width:min(480px,…)}`）+ 列表行（标题/URL/错误 + 移除）+ `TextField` + 「添加」+ `sourcesTrust` 提示条 |
| 空态（`:585-605`）：「还没有插件索引」+ 说明 + 「添加索引源」 | `Discover.drawEmpty`；按钮打开同一个对话框 |
| 写回（`market-store.ts:405-415`）：成功后重读清单，新加的源被选中 | 壳 `cmd/app/market_discover.go` 的 `marketSetIndexes` → 成功后 `refreshMarketIndexes` → 选中项按官方规则（新加的最后一个 → 原来的 → 第一个） |

数据面：壳的状态机是官方 `DiscoverController` 的移植——

```
OFF:market-store.ts:295-311   选中项规则            → cmd/app/market_discover.go refreshMarketIndexes
OFF:market-store.ts:318-360   分页 + fetchedAt 钉住  → runMarketLoad
OFF:market-store.ts:326-330   声明表裁剪后重读        → marketDeclaredQuery + runMarketLoad 尾巴
OFF:market-store.ts:363-380   loadMore（文档变了重读）→ runMarketMore
OFF:market-store.ts:405-415   写回成功后重读清单      → marketSetIndexes
OFF:DiscoverSection.tsx:44,455-459  250ms 输入去抖    → marketSearch（marketSearchDebounce）
```

---

## 3. B 块：搜索 + 筛选工具栏 + 结果列表

工具栏五件套（`OFF:DiscoverSection.tsx:492-560`）逐件对上：

| 官方 | 本仓 | 说明 |
|---|---|---|
| 索引菜单按钮（`.marketButton`，max-width 240，左移 8） | `drawMarketMenu` | 箭头用仓库已有的 `dsassets.ChevronDownOutline` |
| 搜索框（`.search{flex:1;min-width:120}`） | `ui.TextFieldID`（真输入框，由 `Plugin.Fields()` 登记） | 打字走官方 `withText`：空→有字把「默认顺序」切成「最佳匹配」，清空切回；用户选过的顺序不动（`OFF:presentation.ts:24-34`） |
| 标签过滤（`Menu`，`filter` / `filterCount`） | `drawFilterMenu` | 只有索引声明了 labels 才画（官方 `declarations.labels.length === 0 ? null`） |
| 排序（`sortOptions` + 方向切换） | `drawSortMenu` + `chooseSortItems` | 选项：有文本才有「最佳匹配」、默认顺序、发布时间、每个声明的排序键；`published`/`key` 额外有「改为升序/降序」（`OFF:DiscoverSection.tsx:470-495`） |
| 刷新 | `drawRefresh` | 加载中禁用 |
| 分类胶囊行（`allCategories` + 每个分类） | `drawCategories` | 只有声明了 categories 才画 |

结果列表（`OFF:DiscoverSection.tsx:150-165`、`:561-600`）：

```
listing.Value.entries[]  →  EntryFromAPI（分类/标签标题从索引声明表查，OFF:DiscoverSection.tsx:265-300）
                         →  卡片：48px 图标框 + 标题（点开详情）+ 声明标签 Tag
                                + 说明（1 行）+ meta（`{version} · {author} · {date}`）
listing.Value.Total==0   →  官方两句空态：`emptyIndex`（无过滤）/ `noMatches`（有过滤）
listing.Failure          →  「无法读取此索引」+ 重试（旧结果仍画着，官方同样保留）
listing.Value.NextOffset →  「显示更多」
```

卡片与详情用的 Entry 是**同一份投影**（`EntryFromAPI`），所以点卡片进详情时
版本列表、来源 facts、兼容性提示都直接用官方字段，不再换算一遍。

---

## 4. 测试与守卫

| 层 | 文件 | 钉住什么 |
|---|---|---|
| 客户端契约 | `internal/apiclient/plugin_market_discover_test.go` | 三个方法的入参形状（`{}` / 平铺 `url`+`query` / `urls`）、`string|localized` union、npm/git 两个 source 分支、compatibility union、末页 `nextOffset:null`、四档 sourceFailure |
| 壳状态机 | `cmd/app/market_discover_test.go` | 空清单 vs 未接线、两页拼接 + `fetchedAt` 钉住、声明表裁剪后重读、失败保留旧值 + 重试、写回 invalid 档 + 选中新源、250ms 去抖只发最后一次 |
| 插件投影 | `plugins/dsmarket/discover_projection_test.go` | 声明标题查找（不拿 id 冒充标题）、声明排序、`withText`、空态两分、排序菜单、标签开关、卡片 meta |
| 真实像素 + 点击 | `plugins/dsmarket/discover_visual_test.go` | 工具栏/分类行/两张卡片真的画出来；搜索框能点能打字并发出 query；点分类真的按分类过滤；索引菜单列出全部源；点卡片进详情、面包屑「发现」回列表；空态 + 对话框；添加/移除真的写回；两种失败行 + 重试 |
| Plugins 页骨架 | `plugins/dssidebar/plugins_tab_test.go` | 台账随贡献变化、选中项回落、无贡献方不画 tab 条、发现页替换 tab 体而页头仍在 |
| 接线守卫 | `cmd/app/actions_wired_test.go`（既有） | 本批 10 个新 Action 全部在 `cmd/app/main.go` 接线 |

契约快照：`testdata/api.snapshot.json` 新增 3 个方法（batch19），
`counts.total` 81 = 各批之和（`internal/apiclient/snapshot_methods_test.go` 守着）。

---

## 5. 缺口与按仓规 HIDE 的差异（逐条登记）

### 5.1 宿主侧（GUI 补不了，也不假装能）

1. **`listIndexes` 没有配置索引源时回 503 `registry_unavailable`，官方回空数组**
   （`HOST:cmd/dshd/serve_plugin_market.go:94-99` vs `OFF:market-store.ts:295-299`）。
   不认它，全新 profile 上永远只能画「无法读取索引源」，而「添加索引源」按钮只在空态出现
   ——用户没法添加第一个源。客户端按「`registry_unavailable` 且消息不含『未接线』」认成空清单
   （`apiclient.PluginMarketNoIndexes`），**要宿主改成回 `[]`**。
2. **索引文档非法没有独立错误码**：store 把它和「取不到」一起回 `index_unavailable`
   （`HOST:cmd/dshd/serve_plugin_market.go:146-149`），所以官方 `readInvalid`（"此索引的格式无效"）
   这一档画不出来，统一画 `readFailed`。要么宿主补 `plugin-market/invalid-index`，要么接受这条差。
3. **没有 `plugin-market/changed` / `plugin-manager/changed` 事件**（`OFF:client/index.ts:44-49`
   的两个 `$on`）。后果：别的窗口改了索引源/装了插件，这一页不会自己刷新——只能重开面板或点刷新。
   本仓的刷新边界沿用插件管理页先例（首次问拉一次 + 显式刷新 + 失败重试）。
4. **`installBundle` / `updateBundle` 是结构化 501**，所以卡片右侧的安装/更新按钮与
   `openInstalled`（在已安装中查看）**都不画**（`OFF:DiscoverSection.tsx:102-135`）。
5. **没有 `GET /api/plugin-market/detail-file`**，详情文档里的相对图片仍不解析（C 块已登记）。
6. 官方 `MarketInstallation.status` 的 `provided` 宿主不下发（本仓 `not-installed` 起步），
   卡片上那一格状态标签因此不画。

### 5.2 GUI 侧（缺素材/缺能力，按 HIDE 处理）

1. **条目图标**：官方 `.image` 加载 `entry.icon` 的 URL/data URL，缺省走
   `PluginArtworkDefault` 精灵图（`OFF:DiscoverSection.tsx:47,150-165`）。本仓没有图片加载器、
   也没有那枚精灵 → 48px 框内画**标题首字**（derived 兜底，与 `plugins/dssidebar` 的插件卡、
   折线同一先例）。框的尺寸/描边/圆角都是官方的。
2. **图标按钮文字化**：官方工具栏的刷新是 `IconRefreshOutlineRegular`、索引源移除是
   `IconTrashOutlineRegular`、过滤是 `IconSlidersTwoOutlineRegular`（那几枚 sprite 本机 rc.2
   没有导出）→ 刷新画「刷新」、移除画「移除」（命中区标签照官方 `removeSourceNamed`），
   过滤只画文字。唯一有导出的 `IconChevronDownOutline` 用于两个菜单触发器。
3. **骨架**：官方 `ListSkeleton` 是无字占位（`aria-busy`）。本仓画 4 行 48px 骨架块，
   并在上面加一行官方 `loading`（"正在读取索引"）——没有无障碍树，纯骨架看不出"在读"。
4. **工具栏不换行**：官方 `.toolbar{flex-wrap:wrap}`；Viva 的 `ui.Flow` 支持换行但不支持
   `flex:1` 子项，两者不可兼得，取 Row + `Grow` 的搜索框（主区常见宽度 902 下与官方一致）。
   窄宽度下按钮会先被挤压（文案省略），不换行。
5. **`Pill`/`Tag` 度量**：官方这两枚在 `ui-primitives`，那份包**随包不发 CSS**，沿用仓库
   已冻结的同一份官方度量（18px 高 / 7px 内距 / 10px 字号，见 `dsmarket/detail.go` 注释）。
6. **卡片悬停底色不画**：官方 `.card:hover{background:interactive-bg-hover}`；本仓没有逐卡
   鼠标悬停态（只在点击命中上区分）。

---

## 6. 账本变化

`docs/official-parity.md`：

- 条目 `plugin-market`：🟡 → **✅**（A/B/C 三块齐；剩余差异即 §5 的 HIDE 清单）。
- 总表第 30 行：🟡 PARTIAL → **✅ IMPLEMENTED**。
- 合计行重算：`✅ 50 · 🟡 10 · ⬜ 0 · ❌ 6`
  （`python scripts/parity-count.py --check` = PASS）。
