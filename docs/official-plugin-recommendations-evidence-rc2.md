# 官方插件推荐卡（#31 `dsh-client-ui-plugin-recommendations`）的结构证据与实现台账

> 建立时间：2026-11-15 · 对应 [official-parity.md](official-parity.md) 第 31 行（❌ MISSING → ✅ IMPLEMENTED）。
>
> **这份文件回答四个问题**：
> 1. 官方 rc.2 的 `dsh-client-ui-plugin-recommendations` 是什么结构（组件层级 / slot / CSS 几何 / locale 文案）？
> 2. 官方消费哪个宿主方法、数据形状是什么？
> 3. 本仓实现落在哪些 `file:line`，哪些值是从官方原文学来的？
> 4. 哪几处**与官方不同 / 没做**，各自缺的是什么？

**证据口径**：rc.2 起官方**随包发 `src/**` 原件**。锚点 `OFF:<包简称>/<src 内相对路径>:<行号>`，
`OFF` 根 = `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`；
宿主侧锚点 `HOST:`，根是宿主仓的干净导出。

---

## 0. 官方包是什么（**不是一张独立页面，而是一种卡片的正文**）

| 文件 | 作用 |
|---|---|
| `OFF:pr/src/client/index.ts`（13 行） | 浏览器半侧入口：`apply` = `mountRecommendations(ctx, remote)` |
| `OFF:pr/src/client/mount.ts`（49 行） | 挂 Remote → 只在 `remote.pluginRecommendations` 可用后注册正文；**正文注册 = 本文件的全部可见行为** |
| `OFF:pr/src/client/Card.tsx`（159 行） | `PluginCardBody`：header（`推荐插件` + `官方` 徽标）/ 标题+描述 / 状态行 / footer（scope + 重试 / 暂不需要 / 启用） |
| `OFF:pr/src/client/Card.module.css`（12 行） | 全部几何（本文 px 值逐条注行号） |
| `OFF:pr/src/client/locales.ts:2-28` | zh 字典 **27 条**（本仓只许用这 27 条原文） |
| `OFF:pr/src/index.ts`（宿主半侧） | 空 `apply()`：这个包在 Host 侧没有行为 |

**座位归属**：卡片框与 turnTail 槽属于 `dsh-client-ui-action-cards`
（`OFF:pr/src/client/index.ts:13` 的 `inject` 里没有 slots——本包**不占槽**）。
它只做一件事：调用动作卡框架的 `ctx.actionCards.register({action:'enable_official_plugin', …})`
（`OFF:pr/src/client/mount.ts:60-63`），把自己的正文交给框架按 action 派发。

本仓的对应物因此是**三件**：

| 官方那层 | 本仓落点 |
|---|---|
| `ctx.actionCards` 服务（注册表） | `pluginapi/action_card_bodies.go`（服务键 `ui.actionCardBodies` + `ActionCardBody` / `ActionCardBodyRegistry`）+ `plugins/dsactioncards`（提供方） |
| 框架按 action 派发（`useBodies → body.render`，`OFF:ac/src/client/Card.tsx:99-118`） | `plugins/dsactioncards/cards.go` 的 `drawCardTail` → `BodyFor(action)` → `drawKindCard` |
| 本包（正文本体） | `plugins/dspluginreco/plugin.go` |

---

## 1. 卡片结构（官方 → 本仓几何）

```
section.card                                   OFF:ac/Card.tsx:110（框由框架画）
├─ header                                       OFF:pr/Card.module.css:1  padding 10px 20px / 12px / lh 1.5 / bg-module-platform
│  ├─ span 推荐插件（heading）                    OFF:pr/locales.ts:3
│  └─ span 官方（official badge）                 OFF:pr/locales.ts:4（无底色/无边框，只是同行右侧一格字）
└─ .body                                        OFF:pr/Card.module.css:3  padding 20px
   ├─ .details（整块按钮：标题 + 描述）             :4-7  margin -8px / padding 8px / width calc(100% + 16px)
   │  ├─ strong 目录标题 16/500/lh1.5             :5
   │  └─ span   目录描述 13/lh1.7/margin-top 8     :6（描述到达前是 .placeholder 骨架条 :7）
   ├─ .status（role=status / role=alert）         :10  margin-top 16 / padding-top 12 / border-top .5 / 12 / lh 1.6
   └─ .footer（只在 offered 时画）                :8  margin-top 16 / padding-top 16 / border-top .5 / gap 12
      ├─ .scope 启用后可在所有会话中使用            locales.ts:11
      └─ .actions（gap 8）                        :9
         ├─ [重试]（只有 lookupProblem 时）        locales.ts:21
         ├─ [暂不需要]（ghost）                    locales.ts:9
         └─ [启用] / [重试启用]（primary）          locales.ts:8 / :18
```

本仓几何常量全部在 `plugins/dspluginreco/plugin.go` 的 `── 几何 ──` 段，每条注 `Card.module.css` 行号。
**viva 的 Box 没有单边框**，所以 `.status` / `.footer` 的 `border-top .5px` 用一条
`ui.Divider(f).Height(0.5)` 的分隔线盒表达同一个几何（与 `dsconversation` 画 `hr` 同一先例）。

---

## 2. 数据面（官方 → 宿主 → 本仓）

| 官方 | 宿主真实现 | 本仓客户端 |
|---|---|---|
| `ctx.remote.pluginRecommendations.describe(sessionId, callId)`（`OFF:pr/mount.ts:40-44`） | `HOST:internal/transport/handlers_plugin_recommendations_official.go:53-84`（handler）+ `HOST:internal/recommendations/service.go:120-181`（真实现：目录来自 pluginMarket 索引、启用态来自 pluginManager state） | `internal/apiclient/recommendations.go` 的 `PluginRecommendationDescribe` |
| `ctx.actionCards.act(callId, verb, payload)`（框架提供） | `HOST:internal/actioncards/service.go`（kind 已注册 `enable_official_plugin`，动词 `enable` / `dismiss`） | 由 `dsactioncards` 的 `actVerb` 走 `actionCards.act`（本包只调 `props.Act`） |
| `ctx.pluginNavigation.openBundle(packageName)`（`OFF:pr/mount.ts:53-58`） | 无宿主方法（纯界面导航） | `plugins/dssidebar/plugins_page.go` 的 `OpenPluginDetail(name)`，由壳 `cmd/app/official_panels_wiring.go` 转一手 |

**wire 形状（判别联合）**：

```jsonc
{"result":{"status":"available","title":"…","description":"…","enabled":false}}
{"result":{"status":"refused","reason":"unknown|unavailable|access-denied|not-live-root|closing"}}
```

入参**平铺**：`{"sessionId":"…","callId":"…"}`（宿主也收 `agentId` / `id` 别名，见
`HOST:internal/transport/handlers_agent_official.go:433-442`）。已进
`testdata/api.snapshot.json`（`counts.total` 72 → **73**）。

---

## 3. 状态机（官方 `Card.tsx:104-159` → 本仓 `stateOf` / 分支）

| 官方变量 | 本仓 | 语义 |
|---|---|---|
| `catalog`（插件生命周期缓存） | `Plugin.catalog` | 同一插件的后一张卡立刻有文案，不等自己的查询 |
| `lookup`（undefined / null / 结果） | `lookup{done,lost,details}` | `null` = 回复丢失，与"被拒"分开 |
| `settled` | `settled := phase != "" && !openPhases[phase]` | `failed` / `unconfirmed` 仍算开放（官方 `OPEN_PHASES`） |
| `statusKey(phase, data)` | `statusOf` + `enabled` 的 `restart-required` 细化 | 6 种状态行 |
| `failure`（errorCode） | `incompatible-version` → 版本不兼容；其它 → 打开详情检查 | 两条专属文案 |
| `lookupProblem` | settled 之后不再提示；`lost` → 无法读取；`refused` → 五条理由之一 | |
| `offered` / `enable` | `offered = !settled && !(available && enabled)`；`enableShown = offered && (available \|\| loading)` | |

**异步规矩**：查询挂在"这一帧需要它"的时刻发起（`ensureLookup`），落地后
`env.Act.Invalidate()` 请壳重绘；测试用 `run` 同步执行 + `pumpUntil` 反复画帧到条件成立，
**不在发起之后立刻读结果**（本仓对异步结果的既有测试纪律）。

---

## 4. 与官方不同 / 没做（照实登记）

| # | 官方 | 本仓 | 缺的是什么 |
|---|---|---|---|
| 1 | `.placeholder` 骨架条（`Card.module.css:7`，`--dsw-alias-bg-skeleton`）占位，避免描述到达时卡片跳高 | **不画** | 主题里没有 skeleton 令牌的对等物；画一条自选灰色就是自造颜色。代价：描述到达时卡片长高一行 |
| 2 | `.details` 是整块按钮（`width:calc(100% + 16px)` + `:hover` 背景 + focus ring） | **标题行可点**（Hit label = `查看详情：<标题>`） | viva 没有"块级按钮 + hover 背景"的可验证等价物；可点面积是官方子集，没有多出官方没有的动作 |
| 3 | `aria-describedby` 把描述绑给按钮 | 无 | viva 没有 ARIA 关系表；读屏语义由 Hit 的 label 承担 |
| 4 | `remote` / `pluginNavigation` 缺失时**整个插件不 mount** | 插件仍装配，但 `SetSource(nil)` 时**只画标题+简介、不给任何按钮** | 与官方等价的安全退化：不画点了没反应的「启用」 |
| 5 | `useId` / React 的 `pending` 局部态 | `props.Acting` / `LastResult` / `ActError` 由框架回灌 | 状态归属不同（框架持有），行为一致 |

---

## 5. 验收

| 项 | 结果 |
|---|---|
| `python scripts/parity-count.py --check` | PASS（66 = ✅45 / 🟡5 / ❌16） |
| `CGO_ENABLED=0 go build ./...` | 0 |
| `CGO_ENABLED=0 go vet ./...` | 0 |
| `CGO_ENABLED=0 go test ./... -count=1` | 0（41 包 / 0 FAIL） |
| `gofmt -l cmd internal plugins pluginapi` | 空 |
| 新增测试 | `internal/apiclient/recommendations_test.go`（4 条）、`plugins/dsactioncards/bodies_test.go`（6 条）、`plugins/dspluginreco/plugin_test.go`（11 条，含走真 `uikernel.Load` 的注册装配） |
| 契约快照 | `testdata/api.snapshot.json` 加 `pluginRecommendations.describe`；`counts.total` 72→73、`batch15` 1→2 |
