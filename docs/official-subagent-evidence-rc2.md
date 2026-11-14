# 子代理目录（#56 `dsh-client-ui-subagent`）的补齐证据与实现台账

> 建立时间：2026-11-15 · 对应 [official-parity.md](official-parity.md) 第 56 行（🟡 PARTIAL → ✅ IMPLEMENTED）。
>
> **这份文件回答四个问题**：
> 1. 官方 rc.2 的 `dsh-client-ui-subagent` 结构是什么（面板本体 / 行 / 嵌套 / 键盘）？
> 2. 行上的每个值来自哪份宿主真值？
> 3. 本仓这一轮补了什么、落在哪些 `file:line`？
> 4. 哪几处**没做 / 与官方不同**，各自缺的是什么（宿主缺真值 vs GUI 缺承载面）？

**锚点**：`OFF:` = `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/dsh-client-ui-subagent/src/client/`；
`HOST:` = 宿主仓 `origin/main`（本仓干净导出 `tmp/GO8-RC2/_audit-tree`）；`GUI:` = 本仓。

---

## 0. 官方包是什么（一个 header action + 一个 portal 面板）

| 文件 | 作用 |
|---|---|
| `OFF/index.ts`（117 行） | 注册 `conversation.session.header.actions` 的 `subagent-catalog`（id/order 见 :94-107）与 `..subagent.footer`（`OFF/slots.ts:6-9`） |
| `OFF/SubagentHeaderLineage.tsx`（923 行） | 触发器（计数）→ `createPortal(MenuSurface)` 面板 → `CatalogRows` 递归树（每行 6 个元素）→ `renderFooter()` |
| `OFF/SubagentHeaderLineage.module.css`（528 行） | 全部几何（`.row` 247-265、`.children` 432-476、`.metrics` 372-387…） |
| `OFF/locales.ts`（415 行） | 8 种语言；zh 是唯一文案来源 |
| `OFF/SubagentReadOnlyComposer.tsx`、`OFF/sidebar-chat/index.tsx` | 另外两个官方贡献面（见 §4） |

**面板本体**（`OFF/*.tsx:782-827`）：`sectionHeader`（需 `hasSharedTasks`）→ `sectionEmpty` →
`div[role=tree]` 的 `CatalogRows{level:1}` → `renderFooter?()`。
本仓这一轮补的是**树与行本体**；footer 早已落地（`plugins/dsagentteam` 的 TeamBoard）。

---

## 1. 一行（`.row`）的六个元素与数据来源

| # | 元素 | 官方行为 | 本仓这一轮 |
|---|---|---|---|
| 1 | `.disclosure`（14×17，svg 12×12 rotate 90°） | `canExpand` 时画，点击 `toggleBranch`（`:628-635`） | ✅ 画 `▸/▾`，点击切换；`reserveDisclosure`（:259）让兄弟叶子行留位 |
| 2 | `.rowActivitySlot`（14×17） | `StateDot` 三态：`running ? ongoing : completed ? done : idle`（:381-383） | ❌ **不画**：官方三态需要"该子会话是否在跑"的真值；宿主 `subagentCatalog` 没有它（`HOST:internal/transport/handlers_session_official.go:361-369`），画中性点就是把"不知道"说成"未运行" |
| 3 | `.content` | `.label`（当前会话 500）+ `.secondaryLabel`（`description = label \|\| summary.title`，:290-293） | ✅ label；secondary 用 `session.list` 的**真实会话标题**（`GUI:catalog.go` 的 `catalogSessionTitle`） |
| 4 | `.metrics`（10px/15px，两行网格） | 第 1 行 `tokens.total`；第 2 行 compact 时长（title = exact 时长，:390-402） | ✅ token + 时长两行（`GUI:catalog_metrics.go`） |
| 5 | 尾列 | 当前会话 → `currentCheck` ✓；external → 空；否则 `open.sidebar` 按钮 | ✅ 当前会话画 ✓（判据 `entry.id === Snapshot.Cur`）；`open.sidebar` ❌（见 §4） |
| 6 | `.children`（递归 `level+1`） | 展开时画下一层（:438-459） | ✅ 递归画（缩进 16px；连接线不画，见 §4） |

**行的 aria-label** = `primaryLabel · mode · metrics`（:301-322）。本仓只有一个可访问名
（`Hit(id,label)`），所以这句落在命中区 label 上（`GUI:catalog.go` 的 `catalogRowAria`）；
`role=tree/treeitem/aria-level/aria-expanded/aria-current` **无对应 API**（登记）。

---

## 2. 三个值的算法（逐行移植，不许重排判定顺序）

| 值 | 官方算法 | 本仓落点 |
|---|---|---|
| token | `tokens.total` = `formatTokens(四桶之和)`；`<1000` 原样、`<1e6` → `{value}K`（≥100 取整，否则 1 位小数）、否则 `{value}M`（:69-77, :311-313） | `catalogFormatTokens` / `catalogTokenMetric` |
| 时长 | `activityDuration = settledMs + (active ? max(0,(running ? now : active.through) − since) : 0)`（:89-104）；`formatDuration` 多档（:130-160）；≥1 天走 `exactDays`（:162-178） | `SubagentTiming.DurationMs` + `catalogDurationCompact/Exact` |
| 可展开 | `mode !== 'external'` ∧ 子目录 `entries.length > 0`（∧ 两个 migration-required 判据，本仓无此真值）（:209-213） | `catalogCanExpand` |

时长里的 `running` 官方来自会话 status；本仓宿主没有这一位，所以**优先用宿主给的
`active.through`**（真值、不依赖客户端时钟），只有它缺失时才用 `now` 兜底——这条差异
写在 `GUI:plugins/dsagentteam/types.go` 的函数注释里。

---

## 3. 本仓这一轮补的东西（file:line）

| 层 | 落点 | 说明 |
|---|---|---|
| 客户端契约 | `internal/apiclient/projections.go` | 新增 `SubagentIdentity()`（`{mode,label,seq}`）与 `SubagentTiming()`（`{settledMs,active?,lastTurnCompleted?}`）+ `DurationMs` |
| 壳数据面 | `cmd/app/wave3_wiring.go` | `fetchChildProjections`：按目录项逐个拉**子会话自己的** `session.projections`（30s 节流、6s 超时、串行），缓存键 = 子会话 id；目录换批时清掉旧项；切父会话不串数据。`feed` 把 `Children` 单向喂进 Store |
| 运行时 | `plugins/dsagentteam/types.go`、`store.go` | `View.Children`（子投影）+ `SubagentTiming`/`ChildProjection`；`CatalogExpanded/ToggleCatalogBranch/ResetCatalogExpanded`（收起递归整支，关目录清空——官方 `closeBranch`/`changeOpen(false)`） |
| 行渲染 | `plugins/dsagentteam/catalog.go` | `drawCatalogRows` 递归；disclosure、metrics 两行、当前会话 ✓、子会话真实标题；`drawCatalogRow` 的 aria |
| 纯函数 | `plugins/dsagentteam/catalog_metrics.go` | 上面 §2 的四个算法 + zh 文案（`locales.ts:14-27,40-41` 逐字） |
| 键盘 | `plugins/dsagentteam/catalog.go` | `HandleKey`：目录开着时 Esc 关闭并消费（官方 document keydown 的 Escape，:649-653） |
| 测试 | `plugins/dsagentteam/catalog_enrich_test.go`、`internal/apiclient/projections_test.go`、`cmd/app/wave3_wiring_test.go` | 算法表、展开递归、指标/对勾/标题**真画到**、Esc；契约解码；壳侧子投影拉取 + 串会话隔离 + 换批清理 |

---

## 4. 没做 / 与官方不同（各自缺什么）

| 项 | 官方 | 本仓 | 缺的是 |
|---|---|---|---|
| activity 三态圆点 + `progress.*` 进度段 | 会话 status 的 `running` + `lastTurnCompleted`（:519-523、:227-244） | 不画 | **宿主真值**：`subagentCatalog` 没有 running；`subagentTiming.lastTurnCompleted` 只覆盖"最近一轮成功收尾"，据此只能判 completed/inactive 两类，画不出 running——宁可不画 |
| 方向键 / Home / End / Enter | DOM 焦点模型 + `role=treeitem` 的 tabIndex（:688-707） | 不消费 | **GUI 缺承载面**：viva 没有 `document.activeElement`/焦点树，只能用自管 `focusIndex` 建模（下一轮可做） |
| hover 150ms 开 / 120ms 关、pinned 固定 | CSS 容器查询判 `allowsHoverOpen`（:591-612, :725-727） | 只做点击开合 | 无容器查询与 document 级计时器；hover 开合需要帧失效 + 自管 Timer（下一轮可做） |
| `.children` 的 0.5px 连接线 / 横线 | CSS 伪元素（`:440-476`） | 只有 16px 缩进 | viva 无伪元素；单边 0.5px 线没有对应 API |
| `open.sidebar`（在侧边栏打开子会话） | 右栏 tab 体系（`sidebar.right.pane.tab`） | 不画 | **GUI 缺基础设施**：`SlotRightbar` 是单占用者槽，`dsrightbar` 的 painter 不派发子槽，骨架 `has→return`（`GUI:internal/uikernel/shell_skeleton.go:678-684`）——要补需先建资源协议 + 右栏 tab 注册表 |
| header lineage（子会话面包屑/切换器） | 由 `address.parentSessionId` → 扫 `projectionsBySession` → `summary.parentId` 三级求父（`OFF:…tsx`） | 不做 | **宿主真值**：本仓 `subagent` 投影没有 parentSessionId，`session.list` 也没有 origin/parentId（`HOST:…:375-380,1088`；`HOST:api.go:370-377`）——只能靠"从父目录打开时记 parent"这种纯 GUI 近似，本轮不做 |
| 只读 composer（`readonly.*`） | 按 `subagent.address.mode` 为 one-shot/unknown 时接管 `conversation.composer` | 不做 | **宿主恒不触发**：本仓真实创建的子代理全是 continuable（`HOST:…:1086-1088`），gate 永远为假 |
| sidebar-chat tab | `sidebar.right.pane.tab` 上注册 `subagentchat` | 不做 | 同 `open.sidebar`：右栏没有 tab 体系 |
| ARIA（`role=tree` 等）与 CSS anchor/portal 定位 | 浏览器语义 | 不可复现 | viva 无 ARIA 属性面/CSS anchor；面板位置沿用既有常量推导（`GUI:catalog.go:431-437`） |

---

## 5. 证据复验命令

```bash
# 官方
wc -l tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/dsh-client-ui-subagent/src/client/SubagentHeaderLineage.tsx
# 宿主真值
grep -n 'OfficialSubagentTiming\|OfficialSubagentCatalogEntry\|OfficialSubagentIdentity' \\
  tmp/GO8-RC2/_audit-tree/internal/transport/handlers_session_official.go
# 本仓
go test ./plugins/dsagentteam/ ./internal/apiclient/ -run 'Catalog|Subagent|Projections' -count=1
go test ./cmd/app/ -run 'TestWave3' -count=1
python scripts/parity-count.py --check
```
