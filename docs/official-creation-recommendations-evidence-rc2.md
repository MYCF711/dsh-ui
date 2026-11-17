# 官方个性化页面卡（#13 `dsh-client-ui-creation-recommendations`）的结构证据与实现台账

> 建立时间：2026-11-15 · 对应 [official-parity.md](official-parity.md) 第 13 行（❌ MISSING → ✅ IMPLEMENTED）。
>
> **这份文件回答五个问题**：
> 1. 官方 rc.2 的 `dsh-client-ui-creation-recommendations` 是什么结构（组件层级 / CSS 几何 / locale 文案 / 状态机）？
> 2. 它消费哪些宿主方法、数据从哪来？（答案：**一条 Remote 都不用**）
> 3. 本仓实现落在哪些 `file:line`，哪些值是从官方原文学来的？
> 4. 宿主生产路径会回的 `refused{creator-unavailable}` 在官方文案里怎么显示？（答案：官方兜底档 `unavailable`）
> 5. 哪几处**与官方不同 / 没做**，各自缺的是什么？

**证据口径**：rc.2 起官方**随包发 `src/**` 原件**。锚点 `OFF:<包简称>/<src 内相对路径>:<行号>`，
`OFF` 根 = `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`；宿主侧锚点 `HOST:`，
根是宿主仓（`MYCF711/dsh-go`，本轮契约来自 PR #45 的分支 `feat/host-unblock-gui`，
commit `0b065cf` 的 `internal/creation/**`）。

---

## 0. 官方包是什么（**不是页面，是一种卡片的正文**）

| 文件 | 行数 | 作用 |
|---|---|---|
| `OFF:cr/src/index.ts` | 4 | 宿主半侧空 `apply()`：这个包在 Host 侧没有行为 |
| `OFF:cr/src/client/index.ts` | 2 | 浏览器入口：`export { apply, inject } from './mount.ts'` |
| `OFF:cr/src/client/mount.ts` | 28 | `inject: ['locale','actionCards']`（`:15`）→ 注册 locale（`:23`）与正文（`:24-27`） |
| `OFF:cr/src/client/Card.tsx` | 159 | `PageCardBody`：header / info 说明 / summary / 预填内容 / 位置选择 / 两套控件 |
| `OFF:cr/src/client/Card.module.css` | 28 | 全部几何（本文 px 值逐条注行号；`:1-16` 主结构，`:17-28` `.controls .footer` / `.locationRow` / `.locationLabel` / `.locationButton` / `.locationMenu` / `.locationOption` / `.customLocation`+textarea 与焦点、占位态） |
| `OFF:cr/src/client/locales.ts` | 402 | 8 语言；zh 字典在 `:44-78`（**58 条**，本仓只许用这 58 条原文） |
| `OFF:cr/src/client/info-store.ts` | 8 | `infoDismissed` 是**跨会话持久**偏好（`persist: { name: 'dsh.creation-recommendations.info-dismissed' }`，`:7`） |

**座位归属**：卡片框与 turnTail 槽属于 `dsh-client-ui-action-cards`（本包 `inject` 里没有
slots）。本包只做一件事：把正文交给框架按 action 派发
（`OFF:cr/src/client/mount.ts:24-27` 的 `ctx.actionCards.register({ action:'create_personalized_page', locale, Body })`）。

本仓对应物：

| 官方那层 | 本仓落点 |
|---|---|
| `ctx.actionCards` 注册表 | `pluginapi/action_card_bodies.go` + `plugins/dsactioncards`（提供方） |
| 框架按 action 派发 | `plugins/dsactioncards/cards.go` 的 `drawCardTail` → `BodyFor(action)` → `drawKindCard` |
| 本包（正文本体） | **`plugins/dscreationreco/`**（`plugin.go` 1168 行 / `icons.go` 72 行 / `register.go` 10 行 / `plugin_test.go` 727 行） |
| 壳接线 | `cmd/app/main.go` 一行 blank import（**没有 SetSource**，理由见 §2） |

---

## 1. 卡片结构（官方 → 本仓几何）

```
section.card                                    OFF:ac/Card.tsx:110（框由框架画，本包不加内边距）
├─ .header                     cr/Card.module.css:1   padding 12px 20px 0 / 12px / lh 1.5 / 12 gap（行 8 / 列 12）
│  ├─ .heading（gap 4）        :2
│  │  ├─ 个性化页面            locales.ts:45  heading
│  │  └─ info 按钮（28×28）     :3  .infoButton{width:28px;padding:0} + sm 原语 height 28 + 14px 图标
│  │                             （官方 IconWarningTriangleOutlineRegular，viewBox=16；Tooltip=creationInfo，展开时 disabled）
│  └─ .phase（role=status）    :8  color label-primary ← 11 档文案（§3）
├─ .info（默认展开）            :4  margin 12px 20px 0 / padding 12 / radius md / bg-module-platform / 13 / lh 1.7
│  ├─ p 用创造模式制作新页面…     locales.ts:47  creationMethod
│  ├─ p（strong 500）创建可能耗时… locales.ts:48  creationCost + :7 `.info strong{font-weight:500}`
│  ├─ p 点击「创建」后在后台运行…  locales.ts:49  creationBackground
│  └─ p 撤销后，源码仍保留…       locales.ts:62  sourceRetained（**只有 data.bundleName 存在时**）
└─ .body                       :9  padding 12px 20px 20px
   ├─ summary = strong 标题 16/500/lh1.5 + 折叠 brief   OFF:ac/Card.tsx:112-115（框架拼好交给 body）
   ├─ .content（有 initialContent 时）          :10 margin-top 12
   │  ├─ .contentLabel 预填内容                  :11 12 / lh 1.6
   │  └─ Folded(text)                            OFF:ac/Card.tsx:43-67（260 拉丁字宽折叠 + 展开内容/收起内容）
   ├─ .status 应用结果行（application ≠ applied 且 phase ≠ failed）  :16 12 / lh 1.6
   ├─ .status 动作反馈行（error，role=alert）    同上（refused/unconfirmed/transport）
   ├─ offer ? .controls : .placement            Card.tsx:135 / :151
   │  ├─ .controls              :17 margin-top 16 / padding-top 16 / border-top .5 / gap 12
   │  │  ├─ .locationRow（gap 8）:19
   │  │  │  ├─ 添加到（12 / lh 1.6）              :20
   │  │  │  └─ 位置按钮（outline sm，13px，chevron 14）:21-22 + Menu 264 宽四项
   │  │  ├─ [自行撰写时] 你希望放在哪里？ + textarea（min-height 72 / padding 8 10 / 13 / lh 1.7）
   │  │  │                                        :23-25（`flex:1 0 100%` → 独占一行）
   │  │  └─ .footer / .actions（gap 8, margin-left auto）  :14-15 + :18 `.controls .footer{margin:0 0 0 auto}`
   │  │     ├─ [暂不需要]（ghost）                 locales.ts:54  dismiss
   │  │     └─ [创建] / [重新创建] / [开始创建…]（primary）locales.ts:57 / :58 / :59
   │  └─ .placement              :12 margin-top 16 / padding-top 12 / border-top .5
   │     ├─ 添加到 · <位置>        Card.tsx:153（`data.customLocation ?? t(data.slot ?? details.slot)`）
   │     ├─ [取消创建] 仅 creating（ghost → 正在取消…）locales.ts:60 / :61
   │     ├─ [撤销] 仅 installed|kept（ghost）      locales.ts:63
   │     └─ [恢复] 仅 removed（primary → 恢复中…）  locales.ts:64 / :65
```

焦点可见性（textarea 的 `:focus-visible{outline:1px solid state-business-primary}`，
`cr/Card.module.css:26`）由 viva 的输入框自带焦点态承担，不另画。

---

## 2. 数据面：**一条新增宿主方法都没有**

官方 `mount.ts:15` 的 `inject` 只有 `locale` 与 `actionCards`——**没有 `remote`**。
所以这张卡的数据全部来自 props，动作走共享的 `actionCards.act`：

| 卡片要什么 | 官方来源 | 本仓来源 |
|---|---|---|
| `slot`（推荐位置）/ `initialContent` | `proposal.details`（`cr/Card.tsx:19` 的 detailsSchema） | `pluginapi.ActionCardBodyProps.Details`（框架从 `session.projections.actionCards` 投影） |
| 11 档 phase | `decision.phase`（`cr/Card.tsx:26-29`） | `props.Phase` + `props.HasDecision` |
| `slot` / `customLocation` / `bundleName` / `application` | `decision.data`（`cr/Card.tsx:20-23` 的 dataSchema） | `props.Data` |
| 四个动词 + 共享 dismiss | `act(verb, payload)`（`cr/Card.tsx:96`） | `props.Act(verb, payload)` → `dsactioncards.actVerb(callID, …)` → `apiclient.ActionCardAct` |
| 在飞标记 / 四变体结果 | `pending` / `act()` 的返回值 | `props.Acting` / `props.LastResult` / `props.ActError` |
| `infoDismissed` 持久偏好 | localStorage（`cr/info-store.ts:7`） | `Actions.ConfigGet/ConfigSet`，键 **`ui.creationInfoDismissed`**（宿主 `state/ui-config.json`，与 `ui.sidebarWidth` 同一处） |

**宿主侧证据**（生产路径上真实注册了这些 kind / 常量）：

| 锚点 | 内容 |
|---|---|
| `HOST:internal/creation/constants.go` | `Action = "create_personalized_page"`、`Skill = "personalized-pages"`、三个 slot、四个 Verb、9 个 Phase、5 个 Application、`ReservedPrefix = "@dsh-local/view-"`、`StatusFields` |
| `HOST:internal/creation/service.go` | `New()` 注册 kind（`Verbs` / `OpenPhases` / `DelegateVerbs` 四个动词全交 kind 自己按 phase 判） |
| `HOST:internal/creation/service.go` | `create()`：location 校验 → liveRoot 校验 → **`s.creator == nil` 时在写任何文件之前** `Refused(ReasonCreatorUnavailable)` |
| `HOST:internal/creation/service.go` | `undo/cancel/restore` 各自按 phase 分派（removed → settled、installed/kept → 真卸载…） |
| `HOST:internal/creation/skill.go` | `personalized-pages` skill 落 `<home>/skills/personalized-pages/SKILL.md`（无 skill 时 `propose_action` 会以 `ACTION_CARD_SKILL` 拒绝，卡片永远出不来） |
| `HOST:internal/actioncards/` | kind 注册表与四变体 `act`（accepted/settled/refused/unconfirmed） |

因此 **`testdata/api.snapshot.json` 不变**（本次没有新增任何 `internal/apiclient` 方法）。

---

## 3. 11 档 phase：逐档对照（`cr/Card.tsx:26-29` 的 `PHASE_KEYS`）

| phase | 官方文案（zh） | 卡片形态 | 来源 |
|---|---|---|---|
| `dismissed` | 已忽略 | 只画 placement 行（无按钮） | `locales.ts:66` |
| `replaced` | 已替换 | 同上 | `locales.ts:67` |
| `creating` | 正在后台创建 | placement + **[取消创建]** | `locales.ts:68` / `:60` |
| `installing` | 正在安装… | placement（无按钮） | `locales.ts:70` |
| `cancelled` | 已取消 | **openPhases**：位置行 + [暂不需要] + [重新创建] | `locales.ts:73` |
| `installed` | 已安装 | placement + **[撤销]** | `locales.ts:74` / `:63` |
| `kept` | 已保留 | 与 installed 同（撤销可用） | `locales.ts:75` |
| `removed` | 已移除 | placement + **[恢复]** | `locales.ts:76` / `:64` |
| `failed` | 创建失败，可重试 | **openPhases**：可重试 | `locales.ts:77` |
| `interrupted` | 创建中断，可重试 | **openPhases** | `locales.ts:78` |
| `unconfirmed` | 结果待确认，请重试 | **openPhases** | `locales.ts:79` |

- 官方 `openPhases`（`cr/Card.tsx:87`）= `cancelled | failed | interrupted | unconfirmed`，与宿主
  `creation.OpenPhases()` **逐值相同**。9 档宿主 phase 全部落在上面 11 行里（`dismissed`/`replaced`
  是共享 phase，正文接管后由正文画——所以正文必须自己覆盖它们）。
- `application` 四档另有专属文案（`cr/Card.tsx:35-37`）：`restart-required` → 变更已保存，重启
  DSH 后生效 / `overridden` → 其他配置覆盖了本次变更 / `failed` → 上一步更改未生效，请到插件页查看 /
  `cancelled` → 上一步更改已取消。**`applied` 不画**；`phase === 'failed'` 时也不叠加（`cr/Card.tsx:130-131`）。
- **没有缺证据的档**：11 档 + 4 条 application 全部有官方原文对应，本包**没有 HIDE 任何 phase**。

---

## 4. `refused{creator-unavailable}` 怎么显示（本条是最容易做错的一格）

官方 `REASON_KEYS`（`cr/Card.tsx:30-33`）只有 **8 条**，`creator-unavailable` **不在里面**；
而 `cr/Card.tsx:98` 的判据是：

```ts
else if (result.status === 'refused' || result.status === 'unconfirmed')
  setError(keyOf(REASON_KEYS, result.reason) ?? 'unavailable')
```

文件头那句注释就是规则本身：**"Refusal reasons with their own copy; any other reason reads as
unavailable."** 所以宿主的扩展 reason `creator-unavailable` 落官方兜底档：

> **暂时无法执行，请到插件页面查看**（`cr/locales.ts:57` 的 `unavailable`）

实现位置：`plugins/dscreationreco/plugin.go` 的 `errorText()`——`refused`/`unconfirmed` 时取
`reasonText[reason]`，取不到就 `zhUnavailable`。三条性质：

1. **显示在错误行，不是成功态**：卡片仍停在「位置行 + 暂不需要 + 创建」上，**不会**出现
   `已安装` / `已安装` / `正在后台创建` / `正在安装…` 任何一个（测试
   `TestCreatorUnavailableShowsOfficialUnavailableCopy` 逐条钉住）。
2. **不另写中文**：不出现"宿主没有生成器"这类自造句子（那 58 条以外没有官方文案）。
   真实原因由宿主日志（`HOST:internal/creation/service.go` 的 `logf`）与 skill 文档
   （`HOST:internal/creation/skill.go` 明确写「点 Create 会得到 refused{creator-unavailable}」）承担。
3. **登记的措辞落差**：`unavailable` 的字面指引是"到插件页查看"，而本宿主的实际原因是
   "生产接线没有 Creator"。这是**官方兜底档的语义宽度**（任何未列出的 reason 都读这一句），
   改它就等于改写官方文案，所以不改、只登记。

其余 8 条 reason 逐条对应（测试 `TestRefusedReasonsUseTheirOwnCopy` 逐条钉住）：

| reason | 官方文案 |
|---|---|
| `unknown` | 这条推荐已不可用 |
| `access-denied` | 当前连接没有修改插件的权限 |
| `not-live-root` | 请打开原会话后使用这张卡片 |
| `closing` | 创建插件正在停止，请等待重新加载后再试 |
| `unavailable` | 暂时无法执行，请到插件页面查看 |
| `invalid-location` | 请选择位置，或填写不超过 1,000 字的位置说明 |
| `read-only` | 当前会话为只读模式，创建页面需要切换权限预设 |
| `source-missing` | 页面文件已不在，需要重新提案 |

动作**调用失败**（回复丢失 / HTTP 失败）走官方 `transport`（`cr/Card.tsx:97` 的
`if (result === undefined) setError('transport')`）：本仓 apiclient 把丢包报成 error，框架以
`ActError` 交给正文，所以 `ActError != "" → transport`。

---

## 5. 四个动词与 placement 的形状

官方 `run()`（`cr/Card.tsx:90-101`）：

```ts
const placement = { slot, ...location === 'custom' ? { customLocation: customLocation.trim() } : {} }
const result = await act(verb, verb === 'create' || verb === 'dismiss' ? placement : undefined)
if (verb === 'create' || verb === 'dismiss') { infoDismissed.set(true); setInfo(false) }
```

本仓逐条对应（`plugin.go` 的 `runOffer` / `runVerb` / `fire`）：

| 动词 | payload | 触发形态 | 测试 |
|---|---|---|---|
| `create` | `{slot}` + 选了「自行撰写」时 `{customLocation: trim 后}` | primary 按钮，空自定义位置时 disabled | `TestCreateSendsSlotPlacement` / `TestCustomLocationDrivesTextareaAndPayload` |
| `dismiss` | 同上（宿主 `dismissal` 回调把用户选的位置记下来） | ghost 按钮 | 同上 + `TestOfferActions…` |
| `undo` | **不带** | 只有 `installed`/`kept` | `TestInstalledAndRemovedPhases` |
| `cancel` | **不带** | 只有 `creating` | `TestCreatingPhaseOffersCancel` |
| `restore` | **不带** | 只有 `removed` | `TestInstalledAndRemovedPhases` |

在飞文案（官方 `pending === verb` 三分支）：`create` → 开始创建…、`cancel` → 正在取消…、
`restore` → 恢复中…（测试 `TestPendingVerbShowsItsOwnProgressCopy`）。
**这一档必须靠本地记住动词**：框架只给 `Acting bool`，不给"是哪个动词在飞"。

---

## 6. 控件几何表（CSS 行 → 本仓常量）

| CSS（`cr/Card.module.css`） | 本仓常量（`plugins/dscreationreco/plugin.go`） |
|---|---|
| `.header` padding/字号/行高/gap | `createdHeaderPadTop/PadX/Font/LineH/GapColumn` |
| `.heading` gap 4、`.infoButton` 28（+ sm 的 height 28）、图标 14（viewBox 16） | `createdHeadingGap` / `createdInfoButtonW/ButtonH` / `createdInfoIcon` |
| `.info` margin/padding/13/lh1.7、`p+p` 4、`strong` 500 | `createdInfoMarginTop/Pad/Font/LineH/ParaGap/Strong` |
| `.body` padding 12 20 20 | `createdBodyPadTop/PadX/PadBottom` |
| `.content` 12、`.contentLabel` 12/lh1.6 | `createdContentMarginTop/LabelFont/LabelLineH` |
| `.placement` 16/12/border .5/gap 12 | `createdPlacementMarginTop/PadTop/BorderWidth/RowGap` |
| `.description` 12/lh1.6 | `createdDescriptionFont/LineH` |
| `.footer` gap 12 / margin-top 12、`.actions` gap 8 | `createdFooterGap/MarginTop` / `createdActionsGap` |
| `.controls` 16/16/border .5/gap 12 | `createdControlsMarginTop/PadTop/Gap` |
| `.locationRow` gap 8、`.locationLabel` 12/lh1.6、`.locationButton` gap 8 + **13px**、`.locationMenu` 264 | `createdLocationRowGap` / `createdLocationLabelFont/LineH` / `createdLocationButtonFont` / `createdLocationMenuWidth` |
| `.customLocation` gap 6、textarea 72 / 13 / lh1.7 | `createdCustomGap/Height/Font/LineH` |
| 卡片框 `.text`（13/lh1.7/margin-top 8）与 `.toggle`（12/min-height 24） | `createdTextMarginTop/Font/LineH` / `createdToggleMarginTop/MinHeight/Font` |
| 折叠预算 `FOLD_WIDTH = 260`（`OFF:ac/Card.tsx:41`） | `foldWidth = 260` |

按钮字号与高度取自官方**按钮原语**（`OFF:web/dist/assets/index-CSAdOBx6.css` 的 primitives
`_button_1ulci_2` 块：`._sm_1ulci_28{height:28px;font-size:12px;line-height:18px;padding:0 10px;
border-radius:var(--dsw-radius-sm)}`；官方 `dsh-client-ui-primitives` 不随包发 src，这份 CSS 是唯一可核原件）：

| 官方原语 | 本仓做法 |
|---|---|
| `.sm` 的 height 28 / padding 0 10 / radius-sm(8) | viva `Button.Small()` 自带（高度 28、内边距 10、`Radii.SM`=8） |
| `.sm` 的 font-size 12 | `createdSmallButtonFont = 12` → 每个小按钮 `.FontSize(12)`（viva 默认按主题 FontSizeM=14 画字，差 2px） |
| `.locationButton` 覆盖成 font-size 13 | `createdLocationButtonFont = 13` → 位置按钮 `.FontSize(13)` |
| `.infoButton` 是 `size=sm` + 宽 28 | `createdInfoButtonW/H = 28` → 28×28 方形（漏掉高度整条 header 会矮 10px：40→30） |

说明块底色是 `.info{background:var(--dsw-alias-bg-module-platform)}`（`:4`）→ `noticeFill()` 取
`Theme.BgModulePlatform`；**不是** `bg-layer-3`（亮色 #ffffff vs #f9fafb，见 `design-platform.css:183/189`）。

两个官方值**没有对应绘制参数**（留作证据，见 `plugin.go` 的「证据常量」注释块）：
`.header{gap:8px 12px}` 的行间距 8（viva `ui.Row` 不换行，用不到）、
`.locationOption > span + span{font-size:12px}`（菜单第二行由 `ui.MenuItems.Description` 排版）。

---

## 7. 与官方不同 / 没做的地方（**逐条登记，不假装搬了**）

| # | 差异 | 为什么 | 影响 |
|---|---|---|---|
| 1 | **flex-wrap 没有对等物**：官方 `.header`/`.controls`/`.footer` 都是 `flex-wrap: wrap`，viva `ui.Row` 不换行 | viva 的容器布局没有 wrap 档 | `.controls` 按官方的**换行顺序**摆成「位置行 + 按钮」/「textarea」/「按钮右对齐」三行；极窄窗口官方会继续折行，本仓不会 |
| 2 | **`textarea maxLength={1000}` 没有对等物**（`cr/Card.tsx:140`） | viva `ui.TextArea` 不收长度上限 | 超长不静默截断：宿主 `parseLocation` 判 trim 后 1..1000，超了回 `refused{invalid-location}`，卡片按官方文案提示 |
| 3 | ~~info 按钮的图标不是官方那一枚~~ → **已改回官方那一枚** | 旧登记说「rc.2 解包树里只有使用处、没有 path 数据」——**这条结论是错的**：rc.2 sprite `index-ghOpC9uV.js`（sha256 `061de8ce…`、偏移 717952）里 `warning-triangle-outline` 带完整 SVG 正文（viewBox 16、stroke-width 1、round）。原文已存 `assets/official/icons/tools/warning-triangle.jsx`，`icons.go` 按标准 arc→cubic 换算那三段 r=1.33 圆弧（`tmp/GO8-RC2/_vccr/svg2gfx_warning_triangle.{py,txt}`，换算到真实圆弧的最大径向误差 4.1e-5 单位） | 图标已是官方形状；仍不同的是官方 `Tooltip` 在 info 展开时 `disabled`，本仓按同一判据**条件挂载** |
| 4 | **菜单没有"当前项高亮"** | 官方 `Menu selectedId={value}`（`cr/Card.tsx:52`）；viva `ui.Menu` 只有 `Chosen` 单选圆点 / `Checked` 勾选，那会多画官方没有的标记 | 不标（当前值在触发按钮的文字上可见） |
| 5 | **chevron 画在按钮外侧** | 官方 chevron 在 `Button` **内部**（`.locationButton{gap:8px}`）；viva 的 `ui.Button` 放不进子节点 | 同一行紧邻按钮右侧；点击面积 = 按钮那部分（不夸大） |
| 6 | **ARIA 关系没有对等物** | 官方 `role=status`（phase / 应用结果）、`role=alert`（错误行）、`role=note`（info 块）、`aria-controls/expanded` 都是 DOM 属性 | 只保留可交互元素的无障碍名（各按钮 `Hit` label = 官方文案）；纯文本行不带角色 |
| 7 | **`Hit(id,label)` 的 label 同时是可见文字** | viva 的命中区 label 与节点文字是同一个字段（实测写 `添加到: 右侧栏` 会把按钮文字改掉） | 位置按钮用可见文字（`右侧栏`）当无障碍名，`添加到` 由左侧那格文字承担（官方的 `aria-label` 全串做不到） |
| 8 | **grapheme 切分** | 官方用 `Intl.Segmenter` 按字素算宽度，Go 侧按 rune 算（与 dsactioncards 同一处差异） | 只影响折叠出现的位置，**不改写原文** |
| 9 | **菜单项内部三条声明由 viva `MenuItems` 决定**：`.locationMenu{max-width:calc(100vw - 24px)}`（:22）、`.locationOption{gap:2px;line-height:1.5}`（:23）、`.locationOption > span + span{font-size:12px}`（:24） | `ui.MenuItems` 只有 `WidthRange`（已按 :22 的 width 给 264），没有 max-width、行距与第二行字号的参数 | 四项菜单照常可选、第二行仍由 `Description` 画成次要色；极窄窗口下菜单宽度不会收到 `100vw-24` |

**没有 HIDE 任何一格**：11 档 phase、4 条 application、8 条 reason、4 个动词、位置四项、
预填内容、info 说明块全部有官方原文并已绘制。

---

## 8. 验证（`go test ./plugins/dscreationreco/ -count=1`，25 组）

| 测试 | 钉住什么 |
|---|---|
| `TestRegistersIntoActionCardRegistry` | 走**真装配**（`uikernel.Load`）：ds-actioncards 的注册表里确实有 `create_personalized_page` |
| `TestNewCardDrawsOfficialFrame` | 新卡：标题 + 11 句说明 + 预填内容 + 位置行 + 两个按钮；**没有** phase 行 |
| `TestInfoNoticeTogglesAndPersists` | info 默认展开 → 点一次收起并落盘 `ui.creationInfoDismissed=1` → 再点展开 |
| `TestDismissedNoticeStaysClosed` | 偏好读过一次 `1` 之后默认收起 |
| `TestLocationMenuDeclaresOfficialChoices` | 菜单四项、官方**顺序**、每项第二行 hint |
| `TestCreateSendsSlotPlacement` | `create` 的 payload = `{slot}`；**不带** `customLocation`；点创建也落盘偏好 |
| `TestCustomLocationDrivesTextareaAndPayload` | 自行撰写 → 输入框出现；空着时创建**不可点**；非空带 **trim 后**的 `customLocation`，`slot` 仍是推荐值 |
| `TestCreatorUnavailableShowsOfficialUnavailableCopy` | `refused{creator-unavailable}` → 官方 `unavailable` 文案；**不出现**任何成功文案；仍可重试 |
| `TestRefusedReasonsUseTheirOwnCopy` | 8 条 reason 各画自己的官方文案 |
| `TestActErrorShowsTransportCopy` | 回复丢失/调用失败 → 官方 `transport`；**宿主错误原文不进卡片** |
| `TestEveryPhaseHasItsOfficialCopy` | 11 档 phase 逐档文案（先断言表里就是 11 条） |
| `TestCreatingPhaseOffersCancel` | `creating` → [取消创建]；动词 `cancel` **不带** payload |
| `TestInstalledAndRemovedPhases` | `installed`/`kept` → [撤销] + 「添加到 · 全局页面」；`removed` → [恢复] |
| `TestApplicationOutcomeCopy` | 四档 application 文案；`phase=failed` 不叠加；`applied` 不画 |
| `TestPlacementShowsCustomLocationText` | placement 行画用户写的位置原文 |
| `TestInvalidDetailsDrawsSummaryOnly` | details 坏 → 只画 summary，不给动作（官方 `:109`） |
| `TestLongBriefFoldsWithOfficialLabels` | 超长 brief → 「展开」（`action-cards` 的 12 条 zh 之一）→ 点开变「收起」并画全文 |
| `TestCardStateResetsOnAnotherCard` | 换卡整份重置（官方每卡一个组件实例） |
| `TestChooseLocationUpdatesSelection` | 选中四项之后按钮文字与状态都对（弹层项身份在 viva 内部，测试走 `chooseLocation` 这个真接缝） |
| `TestOpenPhasesRelabelCreateAsRetry` | 四个 openPhases 的卡按钮读作「重新创建」，且位置行/暂不需要仍在 |
| `TestPendingVerbShowsItsOwnProgressCopy` | 三个动词的进行时文案（开始创建… / 正在取消… / 恢复中…），结果落地后回落 |
| `TestMenuHiddenWhileBusy` | 动作在飞时位置菜单不出现（官方 `open && !disabled`），落地后回来 |
| `TestOfficialWarningTriangleIcon` | info 图标是官方 `warning-triangle-outline` 原文：viewBox=16、stroke=1、cap/join round、14 条命令逐条一致（圆弧已换算） |
| `TestOfficialCreationButtons` | 按钮几何 = 官方值：info 按钮 28×28、小按钮字号 12、位置按钮字号 13、info 13/lh22.1/padding 12 |
| `TestInfoBlockUsesModulePlatform` | 说明块底色取 `BgModulePlatform`（取 `BgLayer3` 在亮色档必红） |

---

## 9. 缺口与后续（都不阻塞本条）

1. **卡片能不能出现，取决于模型是否 `propose_action`**：`personalized-pages` skill 由接线方
   `EnsureSkill` 落到 `<home>/skills/personalized-pages/SKILL.md`；生产接线里没有 Creator，
   skill 文本**如实**写着"点 Create 会得到 refused{creator-unavailable}"。所以本条的界面是
   **真实的拒绝态展示**，而不是"点一下其实能建页面"。
2. ~~`icon-warning-triangle` 待导出~~ **已导出并接线**（本轮 vc-creationreco）：原文在
   `assets/official/icons/tools/warning-triangle.jsx`（rc.2 sprite，**viewBox=16 不是 14**；
   `size=14` 只是显示尺寸），`icons.go` 的 `warningIcon()` 就是它；主库
   `$DSH_HOME/dsh-ui-official/assets/icons/` 侧仍是待同步项（镜像先落，记在 manifest 的 `mirrorDrift`）。
3. **`reconcile` 的界面侧**：宿主没有后台任务（kind 回调是同步的），`creating`/`installing`
   只在**注入了 Creator 的接线**上真的出现；那时 `cancel` 会回已有 decision（宿主
   `creation.Service.cancel`），卡片按官方 `settled` 语义停在原地——本包不需要额外改动。
