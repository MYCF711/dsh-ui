# 官方 4 条插件设置页（`plugins.item`）在本仓的落地（rc.2）

> 2026-11-15。对应账本 `docs/official-parity.md` 的 **#39 / #45 / #46**（本轮）与 **#47**（下一批）。
> 前置：宿主 PR [MYCF711/dsh-go#41](https://github.com/MYCF711/dsh-go/pull/41) 分支 `feat/host-settings4`
> 已把 8 条命名空间发出来（`settings.describe` 11 → 19 行），见宿主
> `docs/settings-namespaces-official-pages.md`。**没有那 8 条，这 4 页在官方客户端里
> 根本不会挂载**（`ctx.configForms.whileServed([...])`），所以此前 4 条在账本上是 ❌。

## 1. 落点：为什么画在「插件」页而不是设置面板

官方这 4 页不是 `settings.page` 那种顶级 section，而是 **`plugins.item` 槽的卡片**：
插件管理页把官方插件的配置注册成一张卡，卡片打开后是一页表单。

| 官方包 | 注册点（`src/**` 原文） | id | order |
|---|---|---|---|
| `dsh-client-ui-settings-subagent` | `src/client/index.ts:85-92` | `subagent` | 10 |
| `dsh-client-ui-settings-web-search` | `src/client/index.ts:72-74` | `web-search` | 20 |
| `dsh-client-ui-settings-agent-loop` | `src/client/index.ts:47-49` | `agent-loop` | 30 |
| `dsh-client-ui-settings-shell` | `src/client/index.ts:50-52` | `shell` | 40 |

卡片落在插件管理页的 `basic` 组（`renderGroup('basic', t('basicTitle'), basicCards)`，
`dsh-client-ui-plugin-manager/src/client/PluginManagerPage.tsx:1855`；组标题
`basicTitle: '内置插件配置'`，同包 `locales.ts:34`），**组序在「实验性插件」与
「已安装插件」之前**（同文件 1855-1857）。卡片 head 是 `CardHead`（同文件 584-595），
打开后是 `ItemDetail`（同文件 635-666：面包屑 → 标题/一句话 → `data-plugin-config` 段）。

本仓落点：

- 数据模型 `plugins/dssidebar/settings_items.go`
- 绘制 `plugins/dssidebar/settings_items_draw.go`
- 列表接入 `plugins/dssidebar/plugins_page.go` 的 `drawPluginBody`（组）与
  `drawPluginManagerPage`（详情路由）
- 测试 `settings_items_test.go`（状态机/投影/几何台账）、
  `settings_items_visual_test.go`（真 Viva：真 Paint → 真 HitTest → 真 Click → 真写）

存在条件照官方 `whileServed`：宿主**服务**了该卡绑定的任一命名空间才出现，
否则**整张卡不存在**（不是灰色、不是禁用）。

## 2. 三张卡：命名空间 → 字段 → 官方文案（逐字）

### #39 `agent-loop`

| 项 | 值 | 锚点 |
|---|---|---|
| ns | `agent-loop` | `agent-loop-card-controller.ts:13` |
| 字段 | `maxParallelToolCalls` | `AgentLoopCard.tsx:36`（`props.edit('maxParallelToolCalls', …)`） |
| 卡片标题 | `Agent 循环` | `locales.ts:36` `title` |
| 一句话（summary） | `控制 Agent 派发工具调用的方式。` | `locales.ts:37` `description` |
| 字段 label | `并行工具调用数` | `locales.ts:38` `maxParallel` |
| 字段 hint | `同一步内最多同时运行多少个可并行的调用。` | `locales.ts:39` `maxParallelHint` |
| input id | `plugin-config-agent-loop-parallel` | `AgentLoopCard.tsx:30` |

### #45 `shell`（`bash-sandbox` / `pwsh-sandbox`）

| 项 | 值 | 锚点 |
|---|---|---|
| ns | `pwsh-sandbox`（服务时），否则 `bash-sandbox` | `shell-card-controller.ts:10-12`；绑定判据 `index.ts:51` `served.has(PWSH_NS) ? pwsh : bash` |
| 字段 | `timeoutMs` / `maxOutputBytes` | `shell-card-controller.ts:45` |
| 卡片标题 | `终端` | `locales.ts:32` `title` |
| 一句话 | `限制每条命令最多能跑多久、最多输出多少内容。` | `locales.ts:33` `description` |
| `timeoutMs` label/hint | `命令超时（毫秒）` / `单条命令允许运行多久，超时即终止。` | `locales.ts:34-35` |
| `maxOutputBytes` label/hint | `单流输出上限（字节）` / `超出部分会转存到临时文件，而不是被丢弃。` | `locales.ts:36-37` |
| input id | `plugin-config-shell-timeout` / `plugin-config-shell-output` | `ShellCard.tsx:28,41` |

### #46 `subagent`（本轮只做「运行限制」一节）

| 项 | 值 | 锚点 |
|---|---|---|
| ns | `subagent` | `index.ts:50` |
| section 标题 | `运行限制` | `locales.ts:74` `subagentLimitsTitle` |
| 字段 | `maxDepth` / `maxActiveSubagents` | `subagent-limits-card-controller.ts:48` |
| 卡片标题 / 一句话 | `子智能体` / `设置子智能体的递归层级、数量和模型。` | `locales.ts:72-73` |
| `maxDepth` label / invalid | `最大递归深度` / `请输入不小于 0 的整数。` | `locales.ts:75,84` |
| `maxDepth` help | label `最大递归深度说明`；正文 `限制 Agent 创建子智能体的递归层级。` + 表（0 `禁用子智能体`、1 `仅允许主 Agent 创建子智能体`）+ 尾段 `如果某个工具单独设置了最大递归深度，以该工具的设置为准。` | `locales.ts:76-80` |
| `maxActiveSubagents` label / invalid | `子智能体并行数量上限` / `请输入不小于 1 的整数。` | `locales.ts:81,85` |
| `maxActiveSubagents` help | label `子智能体并行数量上限说明`；正文 `同一主 Agent 下，所有递归层级同时存活的子智能体总数，主 Agent 不计入。达到上限时，新的启动请求会被拒绝。` | `locales.ts:82-83` |
| 解析判据 | `Number.isSafeInteger(value) && value >= minimum`（0 / 1） | `subagent-limits-card-controller.ts:28-39` |
| input id | `plugin-config-subagent-depth` / `plugin-config-subagent-capacity` | `SubagentLimitsFields.tsx:24,48` |

表单框架文案（三页逐字相同）：`已覆盖` / `恢复默认` / `本部署的设置为只读。` /
`该插件当前未加载，暂时无法配置。` / `保存` / `保存中…` /
`本部署没有接受这些值，已保留供你修改。` / `请填数字；留空表示使用默认值。`
（`agent-loop` locales:40-47、`shell` locales:38-45、`subagent` locales:65-71）。

## 3. 表单语义：照官方 `SettingsFormModel` 实现（不是"差不多"）

官方 `SettingsValueField` / `SettingsForm` / `SettingsFormModel` 是
`@deepseek-ai/dsh-client-ui-primitives`，**不随 rc.2 单独发包**（package.json 里只有
devDependency）。但它的编译产物进了官方 web 前端 dist chunk，本仓从交付物里取原文：

- CSS：`@deepseek-ai/dsh-web-frontend/dist/assets/index-CSAdOBx6.css`
- JS：`@deepseek-ai/dsh-web-frontend/dist/assets/index-ghOpC9uV.js`

两者的原文副本落在 `tmp/GO8-RC2/_gs4/front-index.{css,js}`（临时文件，不入库）。

状态机（`SettingsFormModel` 的 `format/parse/field/plan/save` 逐条落到 Go）：

```text
format(v)  = typeof v === 'number' ? String(v) : ''            ← 缺失不许变 0
parse(t)   = t.trim()==='' ? clear : (isFinite(Number(t)) ? set(Number(t)) : invalid)
           limitField 另加：Number.isSafeInteger(v) && v >= min
field(f)   无草稿 → {text: format(current), overridden: stored(f)}
           有草稿 → {text: 草稿, overridden: parse.kind==='set', invalid: parse===undefined}
resetField(f) → 草稿 = {text: format(default), clear: true}
plan()     clear → stored 才发 unset；文本==format(current) → 跳过；
           invalid → 只记 invalid 不发；parse=clear → unset；否则 set
save()     plan 空 / 只读 / invalid / saving → 不动
           → mutate(ops, baselineRevision)；全部成功才清草稿；失败置 failed（画 saveFailed）
```

本仓对应：`formatItemNumber` / `parseItemField` / `buildSettingsItemField`（`field()`）/
`settingsItemPlan`（`plan()`）/ `applySettingsItemSave`（`save()`）。写路径是宿主
`settings.mutate`（一次提交一批 `set`/`unset`，带 `sections[].revision` 做乐观锁），
成功后壳自己重读 describe（`cmd/app/settings_panel_wiring.go` 的 `settingsMutate`）。

## 4. 几何台账（官方 CSS 原文 ↔ 本仓常量）

台账在 `settings_items_draw.go` 的 `settingsItemGeometry()`，测试
`TestSettingsItemGeometryMatchesOfficialCSS` 逐行断言 `Official == Impl` 且锚点非空。
关键几行：

| 节点 | 属性 | 官方值 | 官方原文 |
|---|---|---|---|
| `SettingsValueField.field` | padding-block / gap | 12 / 6 | `._field_181ew_3{padding:12px 0; gap:6px}` |
| `.field + .field` | 分隔线 | 0.5px `border-l2` | `._field_181ew_3+._field_181ew_3{border-top:.5px solid var(--dsw-alias-border-l2)}` |
| `.head` | gap | 8 | `._head_181ew_14{gap:8px}` |
| `.label` | font-size / weight | 13 / 500 | `._label_181ew_20` |
| `.badges` | gap | 8 | `._badges_181ew_88` |
| `.reset` | font-size | 12 | `._reset_181ew_94` |
| `.input` | height / padding-inline / font-size / border | 34 / 12 / 13 / 0.5 | `._input_181ew_115` |
| `.input[aria-invalid]` | 描边色 | `state-error-primary` | 同处 |
| `.hint` / `.invalid` | font-size / line-height | 12 / 1.5 | `._hint_181ew_148` / `._invalid_181ew_141` |
| `.help` | padding-top / font-size / line-height | 10 / 12 / 1.6 | `._help_181ew_41` |
| `Tag[data-tone=neutral]`（「已覆盖」） | padding-inline / font-size / line-height | 8 / 11 / 17 | `._tag_brmue_4` |
| `SettingsForm.footer` | padding-top / gap | 16 / 8 | `._footer_ft7vz_16` |
| `.failed` | font-size / line-height / 色 | 12 / 1.5 / `label-error` | `._failed_ft7vz_23` |
| `.readOnly / .unavailable` | margin-bottom / font-size | 12 / 12 | `._readOnly_ft7vz_8` |
| `.save` | padding / font-size / 底/字 | 5×14 / 13 / `label-primary` / `bg-layer-3` | `._save_ft7vz_32` |
| `SubagentCard.section` | padding-block | 16 | `SubagentCard.module.css:1` |
| `SubagentCard.heading` | font-size / weight | 13 / 600 | 同处 `:2` |

页面容器另有官方 `.page{padding:0 clamp(24px,4vw,48px) 48px}`
（`PluginManagerPage.module.css`），此前本仓只做了里层 `max-width:960px`、没有外层内缩，
本轮一并补上（`pluginPagePadX`；这一条同时修正 #29 那一页的贴边）。

## 5. 与官方的差异（逐条登记，不冒充一致）

| # | 差异 | 原因 | 影响 |
|---|---|---|---|
| ① | **帮助折行按钮不画**，帮助正文常显 | 官方是 24×24 的 `helpButton` + `IconQuestionCircle`（primitives）；本机官方素材库里没有这枚图标（`dsh-web-frontend/dist` 只内联图元，本仓 `plugins/dsassets` 未收录），按仓规不手绘冒充 | 信息不丢（正文照画），少一次点击 |
| ② | `maxDepth` 帮助里的 `<table>` 画成两行文字 | 本仓没有表格组件 | 内容逐字，版式不同 |
| ③ | 卡片图标是首字字形 | 官方 `itemArtwork(id)` 是精灵图；本仓宿主不产出（与 `plugins_page.go` 的 bundle 卡同一先例） | 图标非官方 |
| ④ | 「运行限制」两列网格改成单列 | 官方 `.limits{repeat(auto-fit,minmax(min(220px,100%),1fr))}`；本仓按 960 内容宽 `- 2×24` 内缩后两列会把 34px 输入框挤到 220px 以下，中文 label 会截断 | 版式差异，内容一致 |

## 6. HIDE / 未做（缺真实能力或本轮不做）

1. **#46 的「模型选择」一节（`subagent-model-selection-settings`）本轮 HIDE。**
   两条真实阻断都在宿主侧：
   - 该命名空间的 `allowedModels` 是**对象数组**，而宿主 `sections[].fields[]` 只投影
     `string`/`boolean`/`integer` 三种标量（`internal/settingsform/registry.go:14-21`），
     值根本到不了界面；只照 schema 画一个空列表就是编内容。
   - 本仓宿主这条命名空间**没有消费方**（`docs/settings-namespaces-official-pages.md` §4.2）。
   官方页面本来就是"服务了就画、没服务就不画"逐节渲染
   （`SubagentCard.tsx:32-49` 的 `limits.available ? … : null`），所以本仓画「运行限制」
   一节在结构上是官方形状，不是缺块——缺的是这一节背后的能力。
2. **#47 `web-search`（`web` / `web-deepseek-api` / `web-search-deepseek`）下一批。**
   它是 4 条里最大的一条（3 个命名空间、9+7 个字段，含 secret 槽位 `apiKey`、
   enum 型 provider 选择、URL/Env 字段），且 `apiKey` 只报"设没设"
   （`SettingsNamespaceView.secrets`）。按质量优先放在这一批之后单独做。
3. **`bash-sandbox` 的两个字段写进去不改变行为**（宿主侧缺口，不是界面缺口）：
   宿主 bash 执行器没有"前台总超时/每路输出上限"这两个语义，见
   `docs/settings-namespaces-official-pages.md` §4.1。界面照官方发布面画，能读写、能往返。

## 7. 证据与门禁

- 模型/投影/状态机/几何：`go test ./plugins/dssidebar -run SettingsItem -v`（9 条）
- 真绘制 + 真点击 + 真写：`settings_items_visual_test.go`（5 条，真 Viva headless；
  `SETTINGS_ITEMS_CAPTURE_DIR` 非空时落 PNG）
- 截图（不入库，跑测试时落盘）：`SETTINGS_ITEMS_CAPTURE_DIR=<dir> go test ./plugins/dssidebar
  -run 'TestSettingsItem' ` 产出 `settings-items-group.png`（组 + 三张卡 + 组序）与
  `settings-item-detail-subagent.png`（一节两字段 + 「已覆盖」/「恢复默认」+ 「保存」）
- 账本：`python scripts/parity-count.py --check`（本轮后 ✅ 47 · 🟡 6 · ❌ 13）
