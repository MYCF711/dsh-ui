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
| `OFF:pr/src/client/Card.module.css`（13 行） | 全部几何（本文 px 值逐条注行号：1 header / 2 badge / 3 body / 4 details / 5-6 details 的 hover 与 focus-visible / 7 title / 8 description / 9 placeholder / 10 footer / 11 scope / 12 actions / 13 status） |
| `OFF:pr/src/client/locales.ts:2-28` | zh 字典 **25 条**（25 个键在 :3-27；本仓只许用这 25 条原文） |
| `OFF:pr/src/index.ts`（宿主半侧） | 空 `apply()`：这个包在 Host 侧没有行为 |

**座位归属**：卡片框与 turnTail 槽属于 `dsh-client-ui-action-cards`
（`OFF:pr/src/client/index.ts:13` 的 `inject` 里没有 slots——本包**不占槽**）。
它只做一件事：调用动作卡框架的 `ctx.actionCards.register({action:'enable_official_plugin', …})`
（`OFF:pr/src/client/mount.ts:45-48`），把自己的正文交给框架按 action 派发。

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
├─ header                                       OFF:pr/Card.module.css:1  padding 10px 20px / gap 12px / 12px / lh 1.5 / bg-module-platform
│  ├─ span 推荐插件（heading）                    OFF:pr/locales.ts:3
│  └─ span 官方（official badge）                 OFF:pr/locales.ts:4（:2 只有 font-size:12px；无底色/无边框，只是同行右侧一格字）
└─ .body                                        OFF:pr/Card.module.css:3  padding 20px
   ├─ .details（整块按钮：标题 + 描述）             :4   margin -8px / padding 8px / width calc(100% + 16px) / radius sm(8)
   │                                              :5-6 hover 与 focus-visible 底 = interactive-bg-hover（focus 另有 inset 2px 描边）
   │  ├─ strong 目录标题 16/500/lh1.5             :7
   │  └─ span   目录描述 13/lh1.7/margin-top 8     :8（描述到达前是 .placeholder 骨架条 :9）
   │     └─ .placeholder 50% / 0.8em / radius xs(4) / bg-skeleton   :9
   ├─ .status（role=status / role=alert）         :13 margin-top 16 / padding-top 12 / border-top .5 / 12 / lh 1.6
   └─ .footer（只在 offered 时画）                :10 margin-top 16 / padding-top 16 / border-top .5 / gap 12
      ├─ .scope 启用后可在所有会话中使用            :11 12 / lh 1.5；文案 locales.ts:11
      └─ .actions（gap 8 / margin-left auto）     :12
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
| `ctx.remote.pluginRecommendations.describe(sessionId, callId)`（`OFF:pr/mount.ts:37-40`） | `HOST:internal/transport/handlers_plugin_recommendations_official.go:53-84`（handler）+ `HOST:internal/recommendations/service.go:120-181`（真实现：目录来自 pluginMarket 索引、启用态来自 pluginManager state） | `internal/apiclient/recommendations.go` 的 `PluginRecommendationDescribe` |
| `ctx.actionCards.act(callId, verb, payload)`（框架提供） | `HOST:internal/actioncards/service.go`（kind 已注册 `enable_official_plugin`，动词 `enable` / `dismiss`） | 由 `dsactioncards` 的 `actVerb` 走 `actionCards.act`（本包只调 `props.Act`） |
| `ctx.pluginNavigation.openBundle(packageName)`（`OFF:pr/mount.ts:41`） | 无宿主方法（纯界面导航） | `plugins/dssidebar/plugins_page.go` 的 `OpenPluginDetail(name)`，由壳 `cmd/app/official_panels_wiring.go` 转一手 |

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

## 3.1 本轮几何对齐（官方值 vs 旧值，2026-11-17）

**官方包存在**：`@deepseek-ai/dsh-client-ui-plugin-recommendations@0.2.2-rc.2`，随包带
`src/client/Card.module.css`（13 行）与 `src/client/Card.tsx`（159 行）原件，所以下面每个
数值都有 `file:line`；圆角取自 `dsh-client-ui-theme/src/styles/base.css:18-19`。

| 项 | 官方值（出处） | 旧本仓 | 现在 |
|---|---|---|---|
| `.header` 底 | `--dsw-alias-bg-module-platform`（`Card.module.css:1`；design-platform.css:189 浅 `#f9fafb` / :341 深 `#353638`） | `Theme.BgLayer3`（浅 `#ffffff` / 深 `#353638`）——**只有浅色错** | `Theme.BgModulePlatform` |
| `.header` 行列间距 | `gap:12px`（:1） | 无 gap | `Gap(12)` |
| `.details` 命中盒 | `margin:-8px` + `width:calc(100% + 16px)`（:4）→ 640 宽画布上 左 12 / 宽 616 | 左 20 / 宽 600（无外扩） | 左 12 / 宽 616 |
| 标题左缘 | 20（与 `.body` 内容框左缘对齐：`margin:-8px` 与 `padding:8px` 相抵） | 28（多内缩 8px） | 20 |
| 标题文字宽 | 600（616 − 左右各 8） | 584 | 600 |
| `.details` 圆角 | `--dsw-radius-sm` = 8（:4；base.css:19） | 无圆角 | 8 |
| `.details` hover / focus-visible 底 | `--dsw-alias-interactive-bg-hover`（:5-6） | 无 | `Theme.Hover`（= `#ffffff14` 深 / `#2631480f` 浅） |
| `.placeholder` 骨架条 | `width:50%` / `height:0.8em`(=10.4px) / `--dsw-radius-xs`(=4) / `--dsw-alias-bg-skeleton`（:9） | **不画**（描述到达时卡片跳高一行） | 画；占同一行盒 22.1px，到达前后 `.details` 高 70.1 不变 |
| 可点范围 | 整块 `.details`（`:4` 是 `<button>`，Card.tsx:131-137） | 只有标题行 | 整块（盖住描述行） |
| 图标 | 无（`src/client/` 下零 SVG / Icon 引用） | 无 | 无（不手画） |

---

## 4. 与官方不同 / 没做（照实登记）

| # | 官方 | 本仓 | 缺的是什么 |
|---|---|---|---|
| 1 | `.details:focus-visible` 的 `box-shadow: inset 0 0 0 2px var(--dsw-focus-ring-color, var(--dsw-alias-state-business-primary))`（`Card.module.css:6`；宽度出处 focus.css:16 `--dsw-focus-ring-width:2px`） | **只画底色，不画 2px 描边** | viva 的 `ui.Look.Ring` 画在元素**外侧**（`ui/style.go:92-96`「around the outside」），画出来会溢出 `.details` 的 -8px 外扩，与官方 inset 描边不是一回事；不自造一个近似形状 |
| 2 | `.header` / `.footer` 的 `flex-wrap: wrap`（`Card.module.css:1`/`:10`） | 两侧按官方同一行摆；极窄时不折行 | viva 的 `ui.Row` 不换行（与 dscreationreco 登记的同一条限制） |
| 3 | `aria-describedby` 把描述绑给按钮 | 无 | viva 没有 ARIA 关系表；读屏语义由 Hit 的 label 承担 |
| 4 | `remote` / `pluginNavigation` 缺失时**整个插件不 mount** | 插件仍装配，但 `SetSource(nil)` 时**只画标题+简介、不给任何按钮** | 与官方等价的安全退化：不画点了没反应的「启用」 |
| 5 | `useId` / React 的 `pending` 局部态 | `props.Acting` / `LastResult` / `ActError` 由框架回灌 | 状态归属不同（框架持有），行为一致 |
| 6 | `transition: background-color 120ms ease`（`Card.module.css:4`） | 不设时长 | viva 的 `Hover`/`Focus` 状态切换有一帧插值，官方那个时长没有对等开关 |

---

## 5. 验收

| 项 | 结果 |
|---|---|
| `python scripts/parity-count.py --check` | PASS（66 = ✅52 / 🟡12 / ⬜0 / ❌2，2026-11-17 复跑） |
| `CGO_ENABLED=0 go build ./...` | 0 |
| `CGO_ENABLED=0 go vet ./...` | 0 |
| `CGO_ENABLED=0 go test ./... -count=1` | 0（53 包 ok / 0 FAIL） |
| `gofmt -l cmd internal plugins pluginapi` | 空 |
| `python scripts/doc-links.py` / `scripts/scripts-index.py` | PASS / PASS |
| 新增测试 | `internal/apiclient/recommendations_test.go`（4 条）、`plugins/dsactioncards/bodies_test.go`（6 条）、`plugins/dspluginreco/plugin_test.go`（11 条，含走真 `uikernel.Load` 的注册装配）、`plugins/dspluginreco/geometry_test.go`（6 条：官方 px 常量表 / `.details` 外扩 / 骨架条占位 / header 令牌 / 整块命中区 / 零图标） |
| 契约快照 | `testdata/api.snapshot.json` 加 `pluginRecommendations.describe`；`counts.total` 72→73、`batch15` 1→2 |
