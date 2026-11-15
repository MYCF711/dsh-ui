# `dsh-client-ui-plugin-market`（#30）C 块落地证据 · rc.2

> 2026-11-15 · 配合 [official-parity.md](official-parity.md) 的官方对账表读。
>
> 口径：`OFF:<包简称>/<src 相对路径>:<行号>`，`OFF` 根 =
> `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`；`HOST:` = 宿主仓 DSH-GO。
>
> **结论：🟡 PARTIAL —— 只做了详情页（markdown + 版本列表）这一块（C 块）；
> A 块（Discover tab 骨架 / 索引源增删）与 B 块（搜索 + 分类/标签/排序工具栏）
> 仍在等宿主改形状，本轮不做。**

> ## 更新（2026-11-15）：A 块与 B 块已落地，本文的"没做"与三处差异已过期
>
> 宿主 PR `MYCF711/dsh-go#46` 补齐了 `pluginMarket.search` / `listIndexes` / `setIndexes`
> 的官方形状，GUI 侧 A/B 两块同批落地，**`#30` 现在是 ✅**。本文保留为 C 块当时的现场，
> 但下面几处结论已被取代（新证据：[official-plugin-market-discover-evidence-rc2.md](official-plugin-market-discover-evidence-rc2.md)）：
>
> - §0 的表格里 A/B 两行写"不做/等宿主"——**已做**；
> - §3「为什么不注册槽位」——**discover tab 已注册**（`plugins/dssidebar/plugins_tabs.go`
>   的 `plugins.tab` 台账 + `ui.market.discover` 贡献方）；
> - §4 差异 3「分类/标签/索引名三行通常不画」——**现在画**（索引 `metadata.*` 已解析）；
> - §4 差异 11「面包屑没有返回目标时不画」——**现在有返回目标**（详情页从 Discover 列表点开，
>   面包屑回到列表，`dsmarket/discover.go` 的 `SetBack`）。
>
> 仍然有效的是 §4 其余差异（安装/更新 501、`detail-file` 路由缺、`origin` 无 `locale` 档、
> 日期两档、primitives 不发 CSS 等）。

---

## 0. 为什么 `#30` 被拆成三块、为什么先做 C

| 块 | 内容 | 卡的宿主缺口 | 本轮 |
|---|---|---|---|
| A | `plugins.tab` 的 `discover` tab 骨架 + 索引源增删 | `pluginMarket.listIndexes` 的 `title` 恒为 map（官方允许 string）、`setIndexes` 无 `plugin-market/changed` 事件 | **不做** |
| B | 搜索列表 + 分页 + 分类/标签/排序工具栏 | `pluginMarket.search` 结果形状（宿主回 `{index,plugins,total,offset,nextOffset,fetchedAt}`，官方要 `{url,title,metadata,fetchedAt,entries,total,nextOffset}`），且索引文档不解析 `metadata.categories` / `labels` / `sortKeys` | **不做** |
| C | **条目详情 markdown + 版本列表** | 无：`pluginMarket.readEntryDetail` 的结果形状与官方**完全对得上** | **做了** |

C 块能独立做的判据是 M3 这条方法的形状：

- 官方结果（`OFF:dsh-plugin-market/lib/typert.host.js:30-38`）：
  `{status:'found', origin:'locale'|'readme'|'repository', markdown, document} | {status:'none'}`
- 宿主结果（`HOST:internal/transport/handlers_plugin_market_official.go:65-70`）：
  `OfficialMarketEntryDetail{status, origin, markdown, document}` —— 同集合、同判别联合；
  真实现见 `HOST:cmd/dshd/serve_plugin_market.go:234-275`（索引内联 README → 这一版的
  git 仓库 README → 老索引的本地化 description/title 兜底）。
- 分发顺序没问题：`HOST:internal/transport/server.go:631-640` 的 `pluginMarket` 分支
  **在**旧 media 501 分支之前，所以上面那份实现赢（宿主 gap register 里"这两方法还是
  501"的记载是陈旧条目，见 `HOST:docs/host-stable-gap-register.md:452-455`）。

## 1. 官方详情页到底是什么结构

`OFF:dsh-client-ui-plugin-market/src/client/DiscoverSection.tsx:265-355` 的 `EntryDetail`：

```
.detail                     display:flex; flex-direction:column; gap:16px
├─ .crumb                   gap:6px; font-size:12.5px; color:label-tertiary
│                           （图标 IconChevronDownOutlineRegular + rotate(90deg)）
├─ .detailHead              align-items:center; gap:14px
│  ├─ .main (flex:1; column; gap:2px; min-width:0)
│  │  ├─ .titleRow (wrap; gap:8px): .detailTitle(20/28,500) + Tag(版本)
│  │  └─ .meta (12/18, tertiary)   ← 仅当 title ≠ name 时画包名
│  └─ EntryAction（安装/更新/在已安装中查看 —— 见 §4 缺口 1，本仓不画）
├─ .detailDescription       14/22; color:label-secondary
├─ .notice                  padding:10px 12px; radius-md; bg:warn-tertiary; 12/18
│                           （仅当这一版与当前 DSH 不兼容）
├─ .detailDocument          margin-top:16px；空了整段不占位
├─ .detailSection           版本：.sectionTitle(14/20,500) + ul.versions
│  每行 .versionRow         align-items:center; gap:8px; padding:8px 0; border-bottom .5px
│     ├─ .main → .titleRow: .strong(版本号 14/20,500) + 若干 Tag
│     └─ .meta             发布时间 / 已豁免 / 不兼容说明
└─ .detailSection           来源：dl.facts（每行 min-height:44px; padding:8px 2px;
                            border-bottom .5px；dt 13/20 secondary、dd 13/20 右对齐；
                            code 2px 6px 圆角 4 的胶囊）
```

版本行的**四类标签**与**三档说明**（`DiscoverSection.tsx:166-215`）：

| 条件 | 标签 | 说明行 |
|---|---|---|
| `version === defaultVersion` | `推荐`（tone info） | — |
| SemVer 预发布 | `预发布`（tone neutral） | — |
| `installation.installedVersion === version` | `当前已安装`（tone success） | — |
| `!compatibility.compatible` | `不兼容`（warning）/ `已豁免`（warning，exempted 时） | 兼容 → `Intl` medium 日期；豁免 → `exempted`；不兼容 → `incompatible{runtime,range}` |

详情文档本体是 `PluginDetailSection`（`OFF:…/PluginDetailSection.tsx:25-47`）：按
`pluginMarket.readEntryDetail` 的判别联合分四个状态（loading / ready / ready-无正文 /
error），错误文案按六个失败档取（`OFF:…/locales.ts:87-92` 的 FAILURE_KEYS）。

## 2. 数据 → 容器 → 渲染（本仓）

```
pluginMarket.readEntryDetail（宿主真实现）
  → internal/apiclient.PluginMarketReadEntryDetail（唯一新增宿主方法，形状 ✅）
  → plugins/dsmarket.Page.Draw / AfterDraw（面包屑 / 头 / 说明 / 提示 / 文档 / 版本 / facts）
  ← 版本列表来自打开详情页的那一层交进来的 Entry（官方 = search 的 entries[]，本仓 = B 块）
```

- **markdown 用本仓现成的 `components.MarkdownView`**（`mdlite.Highlight` 上色 +
  `MarkdownPaint` 覆画），与 `plugins/dsplan/preview.go`、`plugins/dsconversation`
  同一套。**不自己解析 markdown**（仓规）。
- 先量后画：正文高度在 Draw 阶段用 `MarkdownView.Layout` 量出来，正文靠占位撑开，
  版本列表与 facts 排在它下面；正文在 AfterDraw 里覆画并用滚动视口裁剪
  （与 `plugins/dsplan/preview.go` 同一条理由：viva 的绘制顺序由布局树决定）。

## 3. 座位缺口：为什么这一块**不注册槽位**

官方这条详情页的座位是 `plugins.tab`（id `discover`）里的内容
（`OFF:…/client/index.ts:55-57`）——它是 **Plugins 页里的一个 tab**。本仓的 Plugins 页
（`plugins/dssidebar/plugins_page.go`）今天**没有 tab 贡献方机制**（它自己的文件头写着
"本仓没有贡献方，按官方「没有贡献方就不画 tab 条」的规则整段不画"），那条框架是 A 块。

所以 C 块交的是**可画的那一半**：服务键 `ui.market.detail`（值是 `*Page`），
插件**不占任何槽位**、不画东西、不影响装配。数据面**已经接好**——详情读走
`Actions.MarketEntryDetail` / `MarketEntryDetailRetry`（壳侧 `cmd/app/market_detail.go`
的缓存 + 真 HTTP），由 `TestEveryUsedActionIsWired` 守着"插件在用就一定有接线"。
A 块落地时只要：`Inject("ui.market.detail")` → `Page.Open(entry, marketURL)` →
在它的 tab 里 `Page.Draw` / `Page.AfterDraw`。

**不注册一个永远 dispatch 不到的座位**：仓里对"登记了但画不到"有明确先例
（`plugins/dsgoal/official_seat.go` 文件头），那是不报错、界面空白的最难查的一类故障。

## 4. 与官方的差异（逐条登记，不假装搬了）

| # | 差异 | 依据 / 为什么不补 |
|---|---|---|
| 1 | **安装 / 更新按钮不画** | 官方 `EntryAction`（`DiscoverSection.tsx:102-135`）走 `pluginManager.installBundle`/`updateBundle`，宿主这两个**结构化 501**（`HOST:internal/transport/handlers_pluginmanager_official.go:167-168,279-281`：没有下载源与安装事务）。画一个注定失败的按钮比不画更糟 |
| 2 | **详情文档里的相对图片不出** | 官方走 `GET /api/plugin-market/detail-file?document=&path=`（`OFF:…/market-store.ts:147,230-234`），宿主没有这条路由（`grep -rn 'detail-file' cmd/ internal/` = 0 命中）。不给假图 |
| 3 | **分类 / 标签 / 索引名三行通常不画** | 要官方 `listing.metadata.categories` / `.labels` 的声明标题与 `listing.title`，而本仓索引文档不解析 `metadata.*`（B 块的宿主缺口）。`Entry` 留了字段，数据一到就画（不是 HIDE 掉代码路径） |
| 4 | **不兼容提示条不画警告图标** | 官方那枚是 `IconWarningOutlineRegular`（`DiscoverSection.tsx:329-336`）；本仓没有这枚 sprite 的导出（`plugins/dsassets` 的注册表里没有 warning 三角），按仓规不手绘冒充 |
| 5 | **`DetailDocument` 自己的外框不画**（标题行 / 复制按钮 / 换行开关） | 那个原语在 `@deepseek-ai/dsh-client-ui-primitives` 里，而那份包**随包不发 CSS**（本机 rc.2 的 node_modules 里没有它），几何没有证据。只画正文 + 三行状态文案（都是官方 zh 原文）；失败后的「重试」沿用本仓既有**文字按钮**约定（28px 高、半径 MD、`BgLayer3` 底，与 `plugins/dssidebar/plugins_page.go` 的详情动作按钮同一形状） |
| 6 | **`origin` 只会有 `readme` / `repository`** | 官方的 `locale` 档（插件清单里的本地化长文）宿主不产出。客户端三种都解出来，将来给出时不丢 |
| 7 | **日期只有 zh / 其余两档** | 官方是 `Intl.DateTimeFormat(dateStyle:'medium')`；Go 没有 CLDR 数据。实现真正会跑到的两档（zh → `2026年1月1日`，其余 → `Jan 1, 2026`），其余语言退回英文格式 |
| 8 | **面包屑图标用文字字形 `‹`** | 官方是 `IconChevronDownOutlineRegular` 旋转 90°，本仓无 sprite 导出。与 `plugins/dssidebar` 的 `‹`/`▾` 同一条先例 |
| 9 | **页面左右内距由本页自带** | 官方 `.page{padding:0 clamp(24px,4vw,48px) 48px}` + `.page > *{max-width:960px}` 是 **Plugins 页**那一层的；本页是那个页里的 tab 体，所以自带同样的内距与宽度上限。A 块挂载时**不要**再套一层 `.page` 内距（会双份） |
| 10 | **详情缓存不按连接重置作废** | 官方在 `connection/reset` 上清详情读（`OFF:…/client/index.ts:46-49` 的 `detailsReset()`），因为重启后的宿主不再认得旧 `document` id。本仓那个 id **只用于**拼 `detail-file` 的图片地址，而那条路由没有、图片也不解析（缺口 2），所以不挂一个永远没人调的清空钩子；等 detail-file 落地再补 |
| 11 | **面包屑没有返回目标时不画** | 官方的 `backToList` 是"回到 Discover 列表"，而本仓的列表（A 块）还不存在。画一个点了没反应的按钮比不画更糟（同 `OpenPluginDetail` 返回 false 时不许画入口） |

## 5. 本仓落点

| 文件 | 内容 |
|---|---|
| `internal/apiclient/plugin_market.go` | `pluginMarket.readEntryDetail` 客户端（判别联合 + 六档失败映射）、`PluginMarketDetailFailureOf` |
| `internal/apiclient/plugin_market_test.go` | 平铺入参、`none` 不是错误、六档映射逐条、本仓三种错误码都落 `other` |
| `plugins/dsmarket/market.go` | 条目/版本投影（官方字段名）+ 官方 `presentedVersion` / `prerelease` / `sourceFacts` / 版本行标签与说明 / `formatDate` |
| `plugins/dsmarket/detail.go` | 详情页几何常量（每条带官方 CSS 行号）+ `Page.Draw`（面包屑/头/说明/提示/版本/facts） |
| `plugins/dsmarket/document.go` | 详情文档五态 + `MarkdownView` 排版/覆画/裁剪/链接 + 六档失败文案 |
| `plugins/dsmarket/locale.go` | 官方 zh 字典里属于本页的键（逐字，常量名 = 官方键名） |
| `plugins/dsmarket/plugin.go` / `register.go` | 服务键 `ui.market.detail`（不注册槽位，理由见 §3） |
| `testdata/api.snapshot.json` | 新增 `pluginMarket.readEntryDetail`（batch 18；`counts.total` 77 → 78） |
| `cmd/app/market_detail.go` | 壳侧数据面：`readEntryDetail` 的缓存 + 真 HTTP + 六档失败投影（`Actions.MarketEntryDetail` / `MarketEntryDetailRetry`） |
| `cmd/app/market_detail_test.go` | 真 HTTP：首帧不直接给结果、同一次读只发一次、失败档是官方 `other`、重试再发一次 |
| `pluginapi/plugin.go` | 两个新 Action（`MarketEntryDetail` / `MarketEntryDetailRetry`），形状与 `PluginBundleDetail` 对齐 |
| `cmd/app/main.go` | 一行 blank import（`_ "…/plugins/dsmarket"`）+ 两条 Action 接线 |

## 6. 测试与门禁

`plugins/dsmarket` 三条测试文件 + `internal/apiclient` 一条：

- `market_test.go`（8 条）：默认版本兜底、SemVer 预发布、npm/git 来源事实、
  版本行标签顺序与三档说明、多键要求范围顺序稳定、日期两档 + 空值、标题兜底；
- `document_test.go`（7 条）：五态投影、`none` 不是错误、只有壳接了重读才画重试、
  六档文案逐条、详情读身份带 url/name/version；
- `detail_visual_test.go`（3 条，真实无头像素 + pump 到条件成立）：
  官方结构全画（标题/版本 Tag/说明/提示条/markdown 正文/三条版本/四类标签/facts）、
  **否定判据**（没返回目标不画面包屑、安装/更新按钮一个都没有）、关掉后一帧不画；
- `internal/apiclient/plugin_market_test.go`（5 条）：入参平铺、`none` 非错误、六档映射。

门禁（全部通过，输出贴在 PR/交付说明）：

```text
python scripts/parity-count.py --check   → PASS  （66 个包逐 id 对齐）
CGO_ENABLED=0 go build ./...             → 0
CGO_ENABLED=0 go vet ./...               → 0
CGO_ENABLED=0 go test ./... -count=1      → ok（48 个包 / 0 FAIL）
gofmt -l cmd internal plugins pluginapi  → 空
python scripts/doc-links.py --check      → PASS
python scripts/scripts-index.py --check  → PASS
```

## 7. 交给谁

1. **宿主线（HS）**：`pluginMarket.search` 结果改官方形状 + 索引文档解析
   `metadata.categories` / `labels` / `sortKeys`（B 块前置）；`GET /api/plugin-market/detail-file`
   （§4 缺口 2）；`plugin-market/changed` / `plugin-manager/changed` 两个事件（A/B 的失效信号）。
2. **GUI 线 A 块**：Plugins 页的 `plugins.tab` 贡献方机制 + `discover` tab；
   取本仓 `ui.market.detail` 服务键挂详情页。
3. **GUI 线 B 块**：搜索列表/分页/工具栏；把 `entries[]` 投影成 `dsmarket.Entry`
   （官方字段名已在 `plugins/dsmarket/market.go` 定好，B 只需要解码）。**C 的渲染层不用改。**
