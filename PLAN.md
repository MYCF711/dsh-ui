# PLAN · UI 线 spike（3 天）

> **本文件是 dsh-go 界面线的起点。** 宿主侧规划在 `../DSH-GO/UIPLAN.md`，那里 §2.8（WebView 六种参数）与 §2.9（Go GPU 原生家族）是本次 spike 的全部依据。
> **判定只用数字，不用印象。** 每条实验都有"通过线"，写死在下面。

---

## 0. 一句话

**用 3 天证明或证伪：`Viva`（Go GPU 原生）能不能在 Windows 上，把一段真实会话（中文 + 代码 + emoji + 中文输入法）画到能用，并且内存 ≤ 60 MB。**

通过 → 走原生，插件走 WASM，需要网页时才挂系统 WebView。
不通过 → 退回 WebView 路线（`UIPLAN.md` §2.8），那份方案一行没删。

---

## 1. 已知条件（2026-11-03 实测）

| 项 | 值 | 影响 |
|---|---|---|
| Go | **1.27.3** windows/amd64 | Viva 要求 1.26+，**满足** |
| CGO | **`CGO_ENABLED=0`** | 关键未知：Viva 的 Windows 后端走 cgo 还是纯 `syscall` |
| C 编译器 | **无**（`gcc`/`cc`/`clang` 都不在 PATH） | 若必须要 cgo → 得装 mingw-w64（约 200 MB，可接受但要记账） |
| WebView2 Runtime | **已装**（155/156） | 退路 A 随时可用 |
| gh | 已登录 `MYCF711` | spike 通过后可建私有仓库 |

---

## 2. 三个实验

### E1 · 能不能跑起来（0.5 天）

| 步骤 | 动作 | 通过线 |
|---|---|---|
| 1 | `go install github.com/vivaui/viva/cmd/viva@v0.2.3` | 成功 |
| 2 | `viva doctor` | 打出环境报告，明确告诉我们缺什么 |
| 3 | `viva init hello && cd hello && viva build .` | **在 `CGO_ENABLED=0` 下能编译出 exe** |
| 4 | 双击 exe | 窗口出现，不闪退 |
| 5 | 记录 exe 体积 | **≤ 25 MB** |

**如果第 3 步失败**（报缺 cgo / 缺 DLL）：
- 先试 `CGO_ENABLED=1` + 装 mingw-w64 → **记录"宿主要求一个 C 工具链"这个成本**，它会影响分发（用户机器上要不要装 gcc？大概率不用，只要编译期有）。
- 若装了也编不过 → **E1 判负**，直接跳 E3 的免编译路径（只读文档 + 看 Mayoi 源码，判断它靠什么工作）。

### E2 · 能不能画出我们的东西（1.5 天，最关键）

拿**真实素材**渲染，不用假数据：

| # | 渲染对象 | 通过线 |
|---|---|---|
| 1 | 一段 40 行的**中文 + 英文混排** Markdown（含标题/列表/引用/粗体/行内代码） | 中文**不出豆腐块**，标点不跳、行高一致 |
| 2 | 一个 **Go 代码块 60 行**，带 Syntax Highlight | 等宽对齐、缩进不歪、能横向滚动 |
| 3 | 一条 **工具调用卡片**（标题 + 折叠 + 等宽输出 20 行） | 圆角/边框/折叠动画不掉帧 |
| 4 | **emoji**（😀 🚀 ✅）与 **CJK 全角标点** | emoji 是彩色字形，不是方框 |
| 5 | **中文输入法**：在输入框里用系统 IME 打"你好世界" | 候选框位置跟光标、组合态可回退、不用盲打 |
| 6 | **长列表滚动**：1000 条消息 | 滚动 60 fps，**掉帧 ≤ 1%** |
| 7 | **窗口缩放** | 布局跟随，不闪白、不重排错位 |

**截图存 `docs/e2-*.png`。第 5 条是生死线**——IME 不过，Windows 用户直接判死。

### E3 · 够不够省（1 天）

| 指标 | 通过线 | 怎么量 |
|---|---:|---|
| 空闲内存（窗口开着，不动） | **≤ 60 MB** | 任务管理器 / `Get-Process` |
| 活跃内存（1000 条消息滚动中） | **≤ 120 MB** | 同上 |
| 进程数 | **1** | 同上（这是对官方 9 进程的直接回答） |
| 冷启动到窗口可见 | **≤ 300 ms** | 秒表 × 10 次取中位 |
| 空闲 CPU | **≤ 0.5%** | 让它静置 60 s 看平均 |
| exe 体积（含我们的 UI） | **≤ 25 MB** | `ls -l` |

---

## 3. 判定表

| 结果 | 判定 | 下一步 |
|---|---|---|
| E1 ✅ E2 ✅ E3 ✅ | **走原生** | 建 `app/`，开始 U0 契约 + U1 最小界面 |
| E1 ✅ E2 ⚠️（IME 或滚动不过） | **有条件** | 记录具体哪条不过；评估 WebView 只承载"输入框"这一块（Viva 支持旁边挂 WebView） |
| E1 ✅ E2 ❌ | **否决原生主界面** | 回 A 路线：WebView + TS（`UIPLAN.md` §2.8） |
| E1 ❌（编不出） | **成本否决** | 记录"Windows 需要 C 工具链"；除非用户接受，否则回 A |
| E3 ❌（内存 > 120 MB） | **押注失败** | 回 A；同时记录"Viva 在 Windows 上并不省"，这是对社区有用的数据 |

**任一实验失败都要留证据**：报错原文、截图、命令。**"看起来不行"不算结论。**

---

## 4. 如果 Viva 不行：备选顺序

| 序 | 方案 | 什么时候用 |
|---|---|---|
| 1 | **MyGo**（`egoist/mygo`） | Viva 编不过，但 MyGo 有 Mayoi（18 MB 代码编辑器，299★）背书 |
| 2 | **Viva + 旁边挂 WebView2** | 只有 IME 或富文本编辑不过，其余都能画 |
| 3 | **WebView + TS** | 前两条都死。回到 `UIPLAN.md` §2.8，那份方案一行没删 |

**不做**：Fyne / Gio（纹理与排版质量不匹配本项目）、回 Electron（那是起点动因）。

---

## 5. 与宿主的接口（现在就定死，避免返工）

```
dshd（Go，已有）                    dshui（本仓，新）
   ├ HTTP  POST /api/<method>   ←── 一元调用（会话/文件/配置）
   ├ WS    /api/remote.mux      ←── 所有流（会话事件/助手 chunk/终端）
   └ embed ui/dist              ←── 静态资源（走原生则不需要）
```

**12 个 method 先定这些**（与 `UIPLAN.md` §4 一致，不膨胀）：

```
session.list / create / history / prompt / cancel / delete / fork / load
workspace.list
fs.read / fs.write
config.get / set
```

**契约快照**：`testdata/api.snapshot.json`，改它必须出现在 PR diff 里（对齐宿主 R4）。

---

## 6. 3 天时间表

| 天 | 上午 | 下午 | 晚上 |
|---|---|---|---|
| **D1** | E1 全部 5 步 | E1 结论 + 如有 cgo 问题，评估 mingw 成本 | 出 `docs/e1-result.md` |
| **D2** | E2 的 1–4（静态渲染） | E2 的 5–7（IME / 滚动 / 缩放） | 出 `docs/e2-result.md` + 截图 |
| **D3** | E3 六项实测（每项 10 次取中位） | 写判定表、更新 `UIPLAN.md` §2.9 | 出 `docs/verdict.md`，决定去留 |

---

## 7. 完成定义

- [ ] `docs/e1-result.md`、`docs/e2-result.md`、`docs/verdict.md` 三份都在
- [ ] E2 的 7 张截图都在 `docs/`
- [ ] E3 六项数字全部实测并注明工具
- [ ] `UIPLAN.md` §2.9 的"作者自述、我未实测"字样被真实数据替换
- [ ] 仓库决定：推 `MYCF711/dsh-ui` 或删掉目录

**没做到这三份文档 = 没做完。**

---

## 8. 分层搭建：像搭高塔，一层只依赖它下面那层

> 用户 2026-11-03 定调：**「像搭建高塔一样，一层层搭，能并行开工就并行；模板是重构 DSH 的 GUI，
> 但外观几乎原生；每个部件都插件化，第三方插件直接接入预留的接口就加进去了。」**

这条线的目标是**机制**：官方那 55 个 `dsh-client-ui-*` 是它的粒度，我们抄的是**层与接口**，
不是逐像素复刻每一个面板。外观照官方令牌与组件规格（`--dsw-*` 令牌、28px 圆形图标按钮、
`radius sm/md/lg/xl`、气泡 20px、行 36px…），因为这些是「看起来像原生」的全部依据。

### 各层（下面的层不知道上面有谁）

| 层 | 是什么 | 代码 | 依赖方向 | 能不能并行 |
|---|---|---|---|---|
| **L0 壳** | 开窗、装配、每帧递一份快照、按槽位排布 | `cmd/app` | 只调 L1 | 不可（关键路径） |
| **L1 运行时** | 服务键、生命周期、依赖拓扑、按键转发、槽位收集 | `internal/pluginapi` + `internal/uikernel` | 不依赖任何插件 | 不可（关键路径） |
| **L2 根本依赖插件** | 官方设计令牌 + 基础图元（圆钮/行/徽标/分隔线/chip/气泡/代码块） | `plugins/dscore` | 只依赖 L1 | 不可（其他都等它） |
| **L3 框架插件** | `shell` 槽：三栏几何（侧栏 280 / 中栏 ≥400 / 右栏首次 45%）、`shell.bottom` 整行、`shell.overlay` | `plugins/dsshell`（待建） | 依赖 L2 | 与 L4 并行 |
| **L4 部件插件** | 官方一个包 → 我们一个插件，挂在 L3 的槽上 | `plugins/ds*` | 依赖 L2（+L3 的槽名） | **全部并行** |
| **L5 第三方插件** | 外部作者只要 `Requires(ui.theme, ui.primitives)` + 声明 `Slot` 就接得进来 | 仓库外 | 只依赖 L1 契约 | — |

### 槽位（L3 提供、L4/L5 消费）

| 槽位 | 谁在用 | 说明 |
|---|---|---|
| `shell.leading` | 空 | 窗口左上角（macOS 红绿灯旁）；Windows 暂无 |
| `shell.bottom` | `ds-statusbar` | 横跨三列的整行；**占用方自己声明高度**，空着不占位 |
| `shell.overlay` | 空 | 覆盖整个框架的浮层（弹窗/菜单） |
| `sidebar.header` / `sidebar.list` / `sidebar.footer` | `ds-sidebar` | 左栏三段 |
| `main.header` / `main.body` / `main.composer` | `ds-topbar` / `ds-conversation` / `ds-composer` | 中栏三段 |
| `rightbar` | `ds-rightbar` | 右栏（无占用方时整列不占位） |
| `main:<panelID>` | — | 多面板（keyed）：官方 `main` 是 keyed slot，不同面板共用一个位置 |

### 并行单元（互不依赖，可同时开工）

L4 里每个部件只依赖 L2 的图元和 L3 的槽名，**彼此之间没有任何依赖**，所以可以真并行：
`ds-sidebar`、`ds-conversation`、`ds-composer`、`ds-statusbar`、`ds-topbar`、`ds-rightbar`。
合并规则：一人一个插件目录 + 不改 `cmd/app`；需要新图元就加进 `dscore`（一人一处，冲突最小）。

### 当前进度（对应上面的层）

> 2026-11-03 15:35 复核（每条都有截图/命令作证，明细在 `docs/progress.md` 与 `docs/official-specs.md`）。

| 层 | 状态 | 证据 |
|---|---|---|
| **L0 壳** | ✅ | `cmd/app` 833 行；`-size WxH` 可验不同窗口尺寸下的几何 |
| **L1 运行时** | ✅ | `pluginapi`（公开契约）+ `internal/uikernel`（拓扑装配 + 槽位树）；`internal/uiplugins`（白名单 / `dirs`） |
| **L2 根本依赖** | ✅ | `plugins/dscore`：令牌 + **8 个图元**（ListRow / CircleButton / Chip / Bubble / SectionHeader / Divider / Hairline / StatusBarRow）；几何全部对得上官方 CSS 原文 |
| **L3 框架插件** | ✅ | `plugins/dsshell`：三栏（侧栏 280 / 中栏 ≥400 / 右栏 45%）+ `shell.bottom` 整行 + **`shell.overlay`**；`Env.Geometry` 由壳发布 |
| **L4 部件** | ✅ 六个 | `ds-sidebar` `ds-conversation` `ds-composer` `ds-statusbar` `ds-topbar` `ds-rightbar` |
| **L5 第三方** | ✅ 两条通道 | ① 编译期：`cmd/plugingen` 扫目录生成 blank import，加插件**不改 `cmd/app`**；② 运行时：`internal/uidecl` + `plugins/dsdecl`，**一个 JSON = 一个只读部件，不重编译**。样例：`plugins/dsdemo`（`shell.overlay`）+ 仓库外 `../dsh-ui-plugins/hello` |

已闭合的三个真 bug（都是本轮合并验证抓到的）：
1. `afterDraw` 的 Markdown 覆画用整窗做 `Clip` → 消息盖住顶栏 / 宽表格横穿右栏；
2. `shell.bottom` 用绝对定位 → 状态栏不从 x=0 开始、不压矮三列；
3. 侧栏缺 `Shrink(0)` → 被 flex 从 280 压到 239，行底色横穿正文。
