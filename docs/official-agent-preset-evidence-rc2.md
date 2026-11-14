# Agent 预设（#3 `dsh-client-ui-agent-preset`）的接线证据与实现台账

> 建立时间：2026-11-15 · 对应 [official-parity.md](official-parity.md) 第 3 行（🟡 PARTIAL → ✅ IMPLEMENTED）。
>
> **这份文件回答四个问题**：
> 1. 官方 rc.2 的 `dsh-client-ui-agent-preset` 有哪些面、这一页的结构与几何是什么？
> 2. 卡片上的每个值来自哪份宿主真值？
> 3. 本仓这一轮补了什么、落在哪些 `file:line`？
> 4. 哪几处**没做 / 与官方不同**，各自缺的是什么（宿主缺真值 vs GUI 缺承载面）？

**锚点**：`OFF:` = `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`；`HOST:` = 宿主仓 `MYCF711/dsh-go` 的 `feat/host-contract-gaps`（PR #35，契约说明见该仓 `docs/transport-contract.md` §15.2）；`GUI:` = 本仓。

---

## 0. 官方包是什么（一个 roster，三个面）

`package.json` 的 description 逐字：

> Agent-preset surfaces: the default for later sessions, this session's seat, and the composition editor

| 面 | 官方落点 | 本仓这一轮 |
|---|---|---|
| **后续会话的默认** | `settings.section` 的 `agent-presets` 页（`OFF:dsh-client-ui-agent-preset/src/client/index.ts:283-290`）→ `AgentPresetSection.tsx` | ✅ **这一轮补的就是它** |
| 本会话的 seat（新任务 chip / 会话头只读标签） | `AgentPresetSeat.tsx` + `AgentPresetLabel.tsx` | ❌ 未接（见 §3） |
| 组合编辑器（「查看配置」读一份 preset 的声明正文） | `AgentPresetSection.tsx:170-176` 的 `Modal` + `agentPresets.read` | ❌ 未接（见 §3） |

写路径只有一个：**设置命名空间的一个字段**，不是 preset 自身的属性。

```ts
// OFF:dsh-client-ui-agent-preset/src/client/settings-store.ts:19-26
const response = await ctx.remote.settings.update(
  AGENT_PRESET_SETTINGS_NS, { selectedDefault: id }, expectedRevision)
// :13  export const AGENT_PRESET_SETTINGS_NS = 'agent-preset-registry'
```

宿主侧同源：`HOST:internal/settingsform/agent_preset_schema.go`（命名空间声明，
`schema`/`sections`/`base` 三处同构造）+ `HOST:cmd/dshd/serve_settings.go`
（读写落点；写路径调 `validateAgentPresetSelection`，未知 preset id 当场 400）。
`defaultId = selectedDefault ?? default` 的**唯一解析点**是
`HOST:cmd/dshd/agent_preset_default.go:29-46` 的 `selectedAgentPresetDefault`。

---

## 1. 这一页的结构与几何（逐条对 rc.2 原文）

| 官方节点 | 属性 | 值 | 本仓常量 | 锚点 |
|---|---|---|---|---|
| `.section` | `gap` / `max-width` | 12 / 720 | `AgentPresetSectionGap` / `AgentPresetSectionMaxWidth` | `OFF:.../AgentPresetSection.module.css:1-7` |
| `.title` | `font-size` / `font-weight` | 18 / 600 | `AgentPresetTitleFontSize` / `AgentPresetTitleWeight` | `:9-13` |
| `.intro` | `font-size` | 13 | `AgentPresetIntroFontSize` | `:15-18` |
| `.cards` | `grid-template-columns` / `gap` | `repeat(auto-fill,minmax(268px,1fr))` / 12 | `AgentPresetCardMinW` / `AgentPresetCardsGap` | `:44-51` |
| `.card` | `border` | `0.5px solid` | `AgentPresetCardBorderWidth` | `:54-56` |
| `.cardMain` | `padding` / `gap` | `14px 16px 12px` / 12 | `AgentPresetCardMainPadTop/Bottom/X` / `…MainGap` | `:179-192` |
| `.cardHead` / `.cardIdentity` | `gap` | 12 / 6 | `…HeadGap` / `…IdentityGap` | `:204-207`、`:215-220` |
| `.cardName` | `font-size` / `line-height` | 15 / 1.4 | `AgentPresetCardNameFontSize` / `…NameLn`(=21, derived) | `:223-227` |
| `.cardId` | `font-size` / `line-height` | 11 / 21（等宽） | `AgentPresetCardIDFontSize` / `…IDLn` | `:259-265` |
| `.cardDesc` | `font-size` / `line-height` | 13 / 1.55 | `AgentPresetCardDescFontSize` / `…DescLn`(=20, derived) | `:240-244` |
| `.cardFoot` | `padding` / `gap` | `6px 10px` / 2 | `AgentPresetCardFootPadY/X` / `…FootGap` | `:271-279` |
| `.error` | `font-size` | 12 | `AgentPresetErrorFontSize` | `:384-388` |

以上 25 行全部进了 `GUI:plugins/dssettings/official_geometry.go` 的
`OfficialSettingsGeometry`（两条无单位 line-height 标 `derived` 并把 px 换算写进
`Formula`），由 `TestGeometryTableIsSelfConsistent` 守着。

网格落成：`columns = ⌊(width + 12) / (268 + 12)⌋`（容器 720 宽 → 2 列），
见 `GUI:plugins/dssettings/official_agent_presets_draw.go` 的 `drawAgentPresetCards`。

---

## 2. 每个值的来源

| 卡片元素 | 官方 | 本仓来源 |
|---|---|---|
| 标题 / 简介 | `locales.ts` 的 `nav` / `sectionIntro` | 官方 zh **逐字**（`GUI:…/official_agent_presets.go` 顶部常量） |
| 卡片名 | `presetDisplayText(row, t).name`（内置走字典，自定义走 `name ?? id`） | 宿主 `agentPresets.list` 的 `name`，缺席回落成 `id` |
| 卡片描述 | 同上 `.description`，缺席 `noDescription`「暂无描述。」 | 宿主 `description`，缺席渲染 `zhAgentPresetNoDesc` |
| 右上角 id | `row.id` | 宿主 `id`（逐字） |
| 「新任务默认」角标 | `row.isDefault` → `t('inUse')` | 宿主 `isDefault` —— **客户端不许自己拿设置文档推**（两条读取路径一旦不一致，角标说 A、建会话用 B） |
| 卡片的可访问名 | `` `${selectionAction}: ${display.name}` ``（`:139-140`） | 同一条公式（`agentPresetCardLabel`，官方两条 zh 逐字） |
| 点卡片写什么 | `settings.update('agent-preset-registry', {selectedDefault: id}, rev)` | `settings.form.set`，`ns=agent-preset-registry`、`path=["selectedDefault"]`、值 = 被点那条的 id、`expectedRevision` = describe 里该命名空间的 revision |

数据通路（三条独立能力，**不混**）：

```
HOST agentPresets.list ──► GUI internal/apiclient/agent_presets.go:68 AgentPresetsList
                        ──► cmd/app/agent_preset_wiring.go:37 refreshAgentPresets（缓存）
                        ──► :63 agentPresetRosterSnapshot（同步快照，首问触发后台拉）
                        ──► pluginapi.Actions.AgentPresetRoster
                        ──► official_agent_presets.go:88 buildAgentPresetsSection
                        ──► official_agent_presets_draw.go:94 drawAgentPresetsPage
写：点卡片 ─► official_agent_presets_draw.go:266 applyAgentPresetDefault
          ─► SettingsWriter.SettingsFormSet（壳 settingsFormSet）
          ─► HOST settings.form.set ─► validateAgentPresetSelection（未知 id 400）
          ─► 成功后 Actions.AgentPresetRefresh → 重读名册（角标搬家）
```

`agentPresetWriteTarget`（`official_agent_presets.go:154`）以 **describe 的事实**
判定写路径成不成立：命名空间不在 → 整页仍列名册但卡片不可点并写明原因；
命名空间在而字段标只读 → 同样禁写。**不画点不动的卡片**。

---

## 3. 没做 / 与官方不同（逐条写清缺什么）

1. **不画「内置 / 自定义」分组头。** 官方判据是
   `isBuiltInPreset = name === undefined && id ∈ {standard,ptc,minimal,cordis}`
   （`OFF:dsh-agent-preset-registry/src/display.ts:52-58`）。本仓宿主的 roster 每条都带
   `name`（`modes.Builtin` 有自己的显示名），id 是
   `default/read-only/workspace-write/danger-full-access` —— 按官方那条 fold，
   **全部**条目都会被判成「自定义」。那是错的标签，所以只画一个卡片网格。
   要真分组需要宿主 wire 加 `builtin` 字段（跨线契约，本轮未提）。

2. **卡片页脚只画「设为新任务默认」一枚动作。** 官方页脚还有：
   - 「模式说明 / 如何使用」（`PresetGuideDialog`，文案按官方四个 preset id 写）——
     本仓 id 不同，套上去就是编内容，**不画**；
   - 「查看配置」（`agentPresets.read` + 只读 `Modal`）—— 是另一条动作面与一个模态，
     本仓这一轮未接，**不画死按钮**。宿主侧的方法已就绪
     （`HOST:internal/transport/handlers_agent_official.go:1676-1690`），GUI 侧只差
     `pluginapi.Actions` 的一条读动作 + 模态承载。

3. **会话内 seat（新任务 chip / 会话头标签）未接。** 官方 `AgentPresetSeat.tsx` 挂在
   `conversation.hero` / 会话头槽上；本仓这两个槽的消费面属另一批任务，本轮不动。

4. **色板差异（已登记，不是随手取色）。** 官方选中卡用
   `--dsw-alias-border-selected`、卡片底用 `--dsw-alias-settings-card-fill`；
   本仓 `pluginapi.Theme` 没有这两枚 token，分别取 `BgLayer1`（与内置插件页同一枚卡片底）
   与 `BrandPrimary`（与内置插件页的键盘焦点同一枚）。

5. **id 缺席的行被丢掉，不画。** `selectedDefault` 要写的就是 id；没有 id 的条目
   写不出去。名册里**全部**条目都没有 id 时整页显示「未知」并说明原因，
   不画一张点了必然失败的卡。

---

## 4. 验收（本仓可复跑）

```bash
export PATH="/c/Program Files/Go/bin:$PATH"
CGO_ENABLED=0 go test ./plugins/dssettings/... -count=1 -run AgentPresets -v
```

- 模型四条状态（未接线 / 空名册 / 有名册 / id 无法写出）各一条；
- 卡片顺序 == 宿主返回顺序、`InUse` 只有宿主点名的那一条；
- 写目标 = `agent-preset-registry.selectedDefault`，值 = 被点那条 id，
  `expectedRevision` = describe 的 revision；
- 失败（`invalid_enum`）**不报成功、不刷新名册**；无 writer 时 `WriteFailed` 且文案说清；
- **真实绘制**（真 Paint → 真 HitTest → 真 Click）：`official_agent_presets_visual_test.go`
  断言名册真的画在屏幕上、点卡片真的写出那条写请求、写成功后刷新入口被调一次、
  未接线时画的是「未知」而不是空网格。
