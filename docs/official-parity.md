# 官方部件清单 vs 我们的

> 来源：`tmp/official/` 下从官方 asar 提取的 `@deepseek-ai/dsh-client-ui-*` 包（59 个）。
> 我们有 **22 个插件目录**，对上官方 55 个部件里的 20 个（+ 7 个只做了一半）。
> **"几何对齐 ≠ 像官方"**，差距在功能面。

## 官方 59 个部件（按用途分组）

### 设置类（11）

- ❌ `settings`
- ❌ `settings-account`
- ❌ `settings-agent-loop`
- ❌ `settings-general`
- ❌ `settings-models`
- ❌ `settings-plugin-inventory`
- ❌ `settings-plugins`
- ❌ `settings-session-log`
- ❌ `settings-shell`
- ❌ `settings-subagent`
- ❌ `settings-web-search`

### 侧栏类（6）

- ✅ `sidebar`（`dssidebar`：工作区分组 + 搜索 + 状态点 + **两次点击删除会话**——宿主早就有 `session.delete`，界面一直没接；现在三条都接了：`session.delete`（侧栏两次点击）/ `session.compact`（F2「压缩上下文」）/ `session.fork`（F2「分支这个会话」，**整份复制**））
- ❌ `sidebar-browser`
- ✅ `sidebar-documentpreview`（`dsfiles` 的预览浮层；16KB 上限、20 行截断）
- ✅ `sidebar-files`（`dsfiles`；`workspace.list(path)` + `fs.read`，可进子目录）
- ✅ `sidebar-right`（`dsshell` 的 `SlotRightbar` + `ToggleRightbar` + `ui.rightbarWidth`；收起后 corner 可重开）
- ✅ `sidebar-terminal`（`dsterminal`：F5 面板 + F6 发命令，宿主 `terminal.*` + `internal/ansi`）

### composer 控件（7）

- ✅ `model-selection`（F2 命令面板；数据来自宿主 `models.list` → 网关 `/v1/models`）
- ✅ `permission-presets`（F2 命令面板 5 条；宿主 `session.setPermission` + 白名单）
- ❌ `attachment`
- ✅ `commands`（F2 命令面板；A 类时做的）
- 🟡 `input-trigger`（`dscomposer` 的 `@` 候选菜单；`/` 走 F2。**不是通用的候选弹层管道**，见末尾第 4 条）
- ❌ `entry-customization`
- ⬜ `deepseek-model-label-1017`（**N/A**：官方是 1017 那次发布的临时角标，不是产品能力）

### 插件体系（3）

- ❌ `plugin-manager`
- ❌ `plugin-market`
- ❌ `settings-plugin-inventory`

### 会话能力（13）

- ❌ `agent-preset`
- ✅ `approval`（审批条 + 三按钮；允许/拒绝走真实 `session.approve`）
- ✅ `chat`（`dsconversation` 的消息区；不等于官方的细分能力）
- ✅ `conversation`
- ✅ `goal`（`dsgoal`：GoalBar 停靠 composer 上方，数据来自宿主 `session.setGoal`）
- ❌ `plan`
- ✅ `schedule`（`dsschedule`：侧栏「定时任务」，宿主 `schedule.list`）
- ✅ `jobs`（`dsjobs`：侧栏「后台任务」，宿主 `jobs.list` + 5 秒轮询）
- ❌ `subagent`
- ❌ `skill`
- ✅ `user-questions`（`dsquestions`：F7/F8 或点击选选项）
- ❌ `workflow-run`
- 🟡 `message-feedback`（`dsconversation/feedback.go`：每条助手消息 👍/👎 + 落盘 `feedback.<会话>.<下标>`；**没有官方的反馈弹窗与文字原因**）

### 内容呈现（17）

- 🟡 `reference`（`dscomposer` 的 `@` 补全：识别 + 候选 + 补全做了，**变成引用块没做**，见末尾第 4 条）
- ⬜ `renderer`（**N/A**：官方的 *"Browser UI renderer: React slot bindings"* 是 React 专用；我们是 Go+Viva，槽位绑定在 `internal/uikernel`）
- 🟡 `deliverables`（`dsdeliverables` + `dsconversation`：**2/4**，见末尾「仍然缺」第 6 条）
- ✅ `trajectory`（`dsrightbar`：按轮分组的轨迹栏）
- ✅ `tool`（`dstool`：工具卡片 + 展开参数/结果）
- ❌ `directory-picker-browse`
- ❌ `directory-picker-native`
- ✅ `open-in-app`（两半都有：`dsopen` 会话头部「打开」→ `OpenExternal(CWD)`；`dsfiles` 预览卡片「用默认程序打开」→ `OpenFile(file:…)`；**目录走平台文件管理器**——`app.OpenPath` 明确拒绝目录（`is not a regular file`），而「打开工作目录」正是目录，所以两条路必须分开走）。官方的"split button（列出已装应用）"没有，我们是单个按钮）
- ❌ `workspace`
- ✅ `theme`（`dscore` 的 ds-dark / ds-light 两套 + **运行时切换**：F2 命令面板三条 深色/浅色/跟随系统，`-theme` 也收 `system`；官方的 "Appearance 设置行" 我们没有，入口在命令面板）
- ✅ `shortcuts`（`dsshortcuts`：F1 键位表，**带守卫**——从 `plugins/` 的 AST 里扫出所有 `ui.KeyXxx`，表漏记一个就红；官方的"录制"和"本地改键"没做）
- 🟡 `secret-mask`（`internal/secretmask` + `dsconversation/mask.go`：用户消息**显示**遮罩。只用户消息、只显示，复制仍是原文）
- ⬜ `session`（**N/A**：官方的 *"Session Controller adapter for React"* 是 React 适配层；我们的会话状态在壳里）
- ❌ `cordis`
- 🟡 `brand-official`（`dsbrand`：官方鲸鱼 logo 矢量 + `ui.brand` 可整体替换；**"sidebar slots occupants"那层没有**）
- ✅ `layout`（`dsshell`：侧栏/中栏/右栏/底栏，可直接拖分隔线）
- ✅ `sidebar-right`（`dsshell` 的 `SlotRightbar` + `ToggleRightbar` + `ui.rightbarWidth`；收起后 corner 可重开）

## 合计：官方 55 个部件 —— ✅ 21 · 🟡 6 · ⬜ 3（不该有） · ❌ 25

> 这五个数是**从下面那张表里数出来的**（脚本按 `^- [状态] \`名字\`` 去重统计），
> 不是手填的。上一次手填的是"我们有 10 个 · 缺 53 个"——**已经错了很久**，
> 因为每次补完一个部件都没人回头改这个数。所以这次把它写成"数出来的"。

## 缺的里面，用户最能直接感知的

### 已补掉（2026-11-04）

| 部件 | 现在是什么 | 数据来源 |
|---|---|---|
| `model-selection` | F2 命令面板列出网关的真实模型 | 宿主 `models.list` → 网关 `GET /v1/models` |
| `permission-presets` | F2 命令面板 5 条（4 个值 + 恢复默认） | 宿主 `session.setPermission`（白名单） |
| `sidebar-files` | 侧栏文件树，可进子目录 | 宿主 `workspace.list(path)` + `fs.read` |
| `sidebar-documentpreview` | 点文件 → 右上浮层看全文 | 同上（16KB 上限、20 行截断） |
| `shortcuts` | F1 键位表 | 本仓真实的 `HandleKey` 绑定 |

### 仍然缺

1. **`attachment`** —— 没有附件入口（画了就是死按钮，所以没画）
2. **`settings*`（9 个）** —— 完全没有设置界面，改配置只能改 JSON
3. **`sidebar-terminal`** —— 官方侧栏能开终端
4. **`reference`（`@` 文件引用）** —— **做了一半**（`plugins/dscomposer/mention.go`）。
   官方是两个包：`input-trigger`（"Input trigger pipeline: '/' and '@' detection,
   candidate menu, pick routing to registered sources"）和 `reference`（"Unified Web
   @file and @session reference source"）。

   | 官方那一层 | 我们 |
   |---|---|
   | `@` 识别 + 候选菜单 + 选中补全 | ✅ 敲 `@` 后按前缀列候选（目录在前、大小写不敏感），点一行或按 Enter 补进草稿 |
   | 候选来自"注册的 source" | 🟡 只接了**文件**（`Actions.FSList`，和侧栏文件树同一个接口）。**没有会话引用**（官方 `@` 也能引会话/subagent） |
   | 把 `@path` 变成**引用块** | ❌ 补全出来的是**纯文本**，跟着 prompt 一起发出去 |

   **最后一行是这一项只能记 🟡 的原因**：补全不是引用。引用要有身份（附件 id、
   会话 id），要进宿主的独立字段——而现在的 Transport 契约里没有"附件/引用"这个概念。
   要真做，得先加跨仓契约（`session.prompt` 收 `references`，或 `attachment.*` 一族），
   那是宿主线的活。

   为什么仍然值得做：**它现在就有用**。模型看见 `@plugins/dsopen/open.go` 会自己去 read，
   而用户少打一长串路径、也少打错。只是别把它说成"做到了 @ 引用"。
5. **`jobs.output` 的界面消费** —— 宿主侧通了，但**只在轮次进行中可用**（每轮一个子进程，跑完就退）；做成"点开看输出"会是**大多数时候点了报错**的按钮，所以没做
6. **`deliverables`** —— **只做了 1/4**。官方包的 description 写得很清楚：
   > Changed-files card with per-file comparison tabs, delivery cards, and clickable final-response file references

   | 官方能力 | 我们 |
   |---|---|
   | Changed-files card | ✅ `dsdeliverables`（从 `write`/`edit` 工具参数提路径） |
   | **per-file comparison tabs**（点文件看 diff） | ❌ 需要宿主提供「改之前长什么样」——`fs.read` 只给当前内容，没有历史 |
   | **delivery cards** | ❌ 不清楚它的触发条件（可能是回复里显式的交付声明） |
   | **clickable final-response file references** | ✅ 回复正文里"工具真写过的路径"变成可点链接（`dsconversation/file_refs.go`：只认 `write`/`edit` 参数里的明确路径，不猜 bash/未知路径/URL/行内码/围栏代码；`#L24-L30` 留在显示与 fragment，不进文件路径） |

   **最值得补的是第 4 项**：它不需要新数据源，只需要在渲染 assistant 文本时识别路径。
   第 2 项要动宿主（`git diff` 或文件快照），第 3 项连语义都还没弄清。

## 说明

- ✅ 只表示"我们有一个同名/近义的插件"，**不表示功能等价**（比如我们的 `dsconversation` 远不如官方 `conversation` + `chat` + `trajectory` + `tool` 细分后的能力）
- 这个清单是**方向图**，不是派工单；真要做要按 UIPLAN 补项、逐项验收
## 补第一项时挖到的根因：**宿主没有"改模型/改权限"的 API**

我本来想先补最痛的 `model-selection` / `permission-presets`（把 composer 上那两枚只读文本变成能点开的下拉）。
查下来发现做不了，**不是界面没写，是宿主没给**：

| 能力 | 宿主现状 | 结论 |
|---|---|---|
| `config.set` | 只写 `<home>/state/ui-config.json`（**UI 配置**，键值随便存，宿主不解释） | 不能改模型/权限 |
| 模型 | `session.load` 的 `Model` 字段（omitempty，要有 `request/context` 才有），**只读** | 无设置入口 |
| 权限 | 宿主**启动参数** `--policy`，也回在 `session.load.permission`，**只读** | 无设置入口 |
| `session.create` | 有 `mode` 参数 | 只能建会话时定，不能改现有会话 |

所以这两枚要变成下拉，**必须先有跨仓契约**：
- 要么加 `session.setModel` / `session.setPermission`（改当前会话）
- 要么让 `session.prompt` 收 `model` / `permission` 覆盖（改下一轮）

按 AGENTS §3「跨线需求只写 `docs/transport-contract.md`」，这属于**需求**，不属于我能单方面实现的界面功能。
**我没有为了让 composer 看起来更满而画一个点不动的下拉**——那正是 AGENTS 明确禁止的"死按钮"。

**同样受此限制的还有**：`attachment`（没有上传通道）、`settings*`（9 个，宿主没有对应读写面）、
`sidebar-files` / `sidebar-terminal`（宿主有 `fs.read`/`fs.write`，但没有目录列举与终端会话）。

**这解释了"为什么差得很远"的一半**：59 个官方部件里，相当一部分依赖宿主提供的能力面，界面单方面补不出来。

## 53 个缺口按「谁挡着」分类

分三类，只有第一类我能**现在动手**。

### A. 界面自己就能做（不依赖宿主新 API）—— 8 个

| 部件 | 为什么现在能做 |
|---|---|
| `shortcuts` | 纯 UI：一个键位说明面板，键位是界面自己定的 |
| `theme` | `dscore` 已有 `Dark()`，加亮色/跟随系统就行 |
| `message-feedback` | 消息下方的 👍/👎，本地记录即可（不承诺上报） |
| `trajectory` | **我们已有**（`dsrightbar`），只差官方那种分组/折叠层级 |
| `tool` | 工具卡的独立插件化（现在写在 `dsconversation` 里） |
| `commands` | 命令面板，动作走现成的 `Actions` |
| `layout` | 三栏求解已有（`dsshell`），差的是可拖拽分栏 |
| `brand-official` | 官方鲸鱼 logo 已转（`brand_path.go`），差的是品牌包扩展点 |

### B. 宿主已有能力面，界面没接 —— 6 个

| 部件 | 宿主已有什么 | 缺什么 |
|---|---|---|
| `sidebar-files` | `fs.read` / `fs.write` / `workspace.list` | **目录列举**（`workspace.list` 只给顶层） |
| `sidebar-documentpreview` | `fs.read` | 只需要渲染 |
| `reference` | `workspace.list`（已用） | 补全够用；**变成引用块**要新增"附件/引用"契约 |
| `deliverables` | `fs.write` | 需要"哪些是交付物"的约定 |
| `jobs` | 后台任务概念已有 | 需要 `jobs.list`/`jobs.output` 的跨线方法 |
| `schedule` | 无 | 需要定时任务的存储与方法 |

### C. 必须宿主先加跨线契约 —— 39 个（含 11 个设置类）

`settings*`(11) · `model-selection` · `permission-presets` · `attachment` · `plugin-manager` ·
`plugin-market` · `settings-plugin-inventory` · `sidebar-terminal` · `sidebar-browser` ·
`sidebar-right` · `subagent` · `skill` · `agent-preset` · `goal` · `plan` · `user-questions` ·
`workflow-run` · `secret-mask` · `open-in-app` · `entry-customization` · `directory-picker-*` ·
`input-trigger` · `deepseek-model-label-1017` · `renderer` · `workspace` · `session` · `cordis` ·
`approval`(我们已有简化版) · `chat`

**结论**：真正能靠界面补齐的是 **A 类 8 个 + B 类 6 个**；C 类 39 个必须先有宿主契约——
这也解释了为什么"看起来差得远"却一时补不上：**大头在宿主那一侧**。

### 我这一轮选哪个动手

**`shortcuts`**（A 类第一个）：纯 UI、零依赖、用户能立刻感知（现在按 F12 收起 ds-demo 这类提示散落在各处，
没有一个地方能查"我还能按什么键"）。做完要能说清每个快捷键**真实绑定在哪里**，不编。

## A 类 8 个的实际进度与难度修正（做完 4 个之后的复盘）

| 部件 | 状态 | 实际难度 | 教训 |
|---|---|---|---|
| `shortcuts` | ✅ | 中 | 卡在**重绘**，不是布局。`-keys` 注入绕过 `OnKey` 的 Invalidate |
| `theme` | ✅ | 低 | `dscore.Light()` 早就存在，只是 `fixedDark: true` 没人能选 |
| `commands` | ✅ | 中 | 卡在 `Row` 写成 `Column`（一行 vs 多行） |
| `message-feedback` | ✅ | 中 | 关键不是按钮，是**边界**：没有上报通道就不许写"已上报" |
| `layout` | 🟡 拖拽完成 | **高** | 拖拽本身不难，**持久化要穿三层契约**（apiclient → Actions → cmd/app） |
| `trajectory` | 🟡 派单中 | 中 | 已有平铺版，差分组折叠 |
| `tool` | 🟡 派单中 | **高** | 抽取时要"按类型分派"的结构，简单搬代码等于没拆 |
| `brand-official` | ⬜ | 中 | 要先把 `brand_path.go` 从 dsconversation 抽出来 |

**修正一条之前的判断**：我最初把 A 类说成"界面自己能做、不依赖宿主"——**`layout` 的持久化就依赖宿主**（`config.get/set` 那三层契约是我这轮补的）。
所以 A 类准确的说法是「**不依赖宿主新增业务 API**」，而不是"完全不碰契约"。

## 这 4 个做完之后的共同点

**三个都卡在同一类地方**：不是"能不能画出来"，而是**"画出来了但没人看见"**——
- `shortcuts`：状态切了、没重绘
- `commands`：状态切了、没重绘（同一原因）
- `message-feedback`：按钮画了，但"记录"这个动作没有真实通道（差点就要写假的"已上报"）

**"能画"和"能看见/能生效"是两个问题**，后者更难，也更少人验。

## B 类的确切阻塞点（逐个查过，不是猜）

我本想先做 `sidebar-files`，查下来卡在**宿主没给工作区根**。逐项实测（真宿主 `curl`）：

| 宿主方法 | 实测结果 | 能否支撑界面 |
|---|---|---|
| `workspace.list` | `{"entries":[{"name":".git","isDir":true,"size":0}, ...]}` | **只给 name，没有 path、也没有"列子目录"的入参** |
| `fs.read` | `{"path":"D:/.../go.mod","content":...}` 通了（**必须 Windows 路径**，MSYS 的 `/d/...` 报"文件不存在"） | 能读，但**要先知道完整路径** |
| `session.load` | `keys: [messages, total, truncated, permission, usageTotal, contextTokens, journalEpoch, lastSeq]` | **没有 `cwd`** |

**所以 `sidebar-files` 走不通的链条是**：

```
workspace.list 给 name（无 path）
  ↓
要读文件得拼「工作区根 + name」
  ↓
界面不知道工作区根（session.load 没有 cwd）
  ↓
fs.read 无法调用
```

**要解开只需宿主给一样东西**（二选一）：
1. `workspace.list` 每项补 `path`（或加 `list(path)` 入参）
2. `session.load` 回 `cwd`

**这不是"界面没做"，是"做不了"**。我把它写死在这里，免得下一轮又有人从头查一遍。

**B 类其余几个的阻塞**（同样逐项查过）：

| 部件 | 阻塞 |
|---|---|
| `deliverables` | 没有"哪些文件算交付物"的约定——`fs.write` 只知道写了什么，不知道哪个是交付品 |
| `reference` | 补全已做（`workspace.list` 够用）；**引用块**缺的是"附件/引用"这一层契约——没有身份、没有独立字段，`@path` 只能是一段文本 |
| `jobs` | 没有 `jobs.list` / `jobs.output` 这两个方法 |
| `schedule` | 完全没有定时任务的存储与方法 |
| `sidebar-documentpreview` | 只差上面那个"工作区根"（有根就能读、能预览） |

**结论**：B 类 6 个**全部**卡在宿主侧。界面这边能做的只有"等"——所以我不会假装在推进它们。

---

## 2026-11-04 账本审计：九处从 ❌ 改掉

起因是补 `@` 补全时发现清单上 `secret-mask` 还写着 ❌——**而它两个提交前就接好线了**。
顺手把剩下的 ❌ 逐条对着代码核了一遍，九处不对。

| 部件 | 原来 | 现在 | 依据 |
|---|---|---|---|
| `secret-mask` | ❌ | 🟡 | `internal/secretmask` + `dsconversation/mask.go`：用户消息**显示**遮罩（只用户消息、只显示） |
| `message-feedback` | ❌ | 🟡 | `dsconversation/feedback.go`：每条助手消息 👍/👎，落盘 `feedback.<会话>.<下标>`；**没有反馈弹窗** |
| `open-in-app` | ❌ | 🟡 | `plugins/dsopen/open.go`：会话头部「打开」→ `OpenExternal(CWD)`；**文档预览那半没做** |
| `brand-official` | ❌ | 🟡 | `plugins/dsbrand/brand.go`：官方 logo 矢量 + `ui.brand` 可替换；**sidebar slots occupants 没做** |
| `input-trigger` | ❌ | 🟡 | `plugins/dscomposer/mention.go`：`@` 候选菜单；`/` 走 F2。**不是通用候选弹层管道** |
| `sidebar-right` | ❌ | ✅ | `SlotRightbar` + `ToggleRightbar` + `ui.rightbarWidth` + 收起后 corner 重开 |
| `session` | ❌ | ⬜ | 官方原文是 *"Session Controller adapter **for React**"*——React 适配层，我们没有也不需要 |
| `renderer` | ❌ | ⬜ | 官方原文是 *"Browser UI renderer: **React** slot bindings"*——同上 |
| `deepseek-model-label-1017` | ❌ | ⬜ | 官方原文是 *"**Temporary** 1017 release badge"*——一次发布的临时角标，不是产品能力 |

**为什么能错这么久**：清单是手写的，而每补完一个部件时，
"把清单那一行改掉"从来不是任务的一部分——它总被当成收尾的杂事。
于是账本只会往"比实际更差"的方向漂，而且漂了没人发现（因为没人会去"测试"文档）。

**现在怎么防**：`python scripts/parity-count.py --check`（CI 里跑）。
合计那一行是**数出来的**，条目和合计对不上就红。守卫本身做过反证：
把 `attachment` 临时改成 ✅ → `FAIL 合计行和条目对不上`，改回去 → `PASS`。

⚠️ 这一条只防"合计与条目不一致"，**防不了"条目写错"**——
上面那九处是人对代码核出来的，脚本做不到。所以状态行仍然要靠人诚实。
