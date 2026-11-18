# `dsh-client-ui-sidebar-terminal` 逐条收口（rc.2 / 2026-11-18）

> 账本行：[`official-parity.md`](official-parity.md) 的 #54。本文是那一行的**长版证据**，
> 逐条写清「已闭合 / 仍缺 / 被什么挡住 / 什么条件下能做」。
>
> 方法：不沿用旧账的结论。每一条先回官方产物（`OFF:`）与 viva v0.6.4 源码复核，
> 再回本仓代码看现状；能闭合的闭合并给出官方 `file:line`，闭不了的写到"下一个具体动作"。
>
> `OFF:` 前缀指向 rc.2 只读取证树
> `DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`（包清单与来源台账：
> 本仓 [`docs/evidence/official-ui-plugins-0.2.2-rc.2.txt`](evidence/official-ui-plugins-0.2.2-rc.2.txt)
> 与 `$DSH_HOME/dsh-ui-official/SOURCES.md`）。
> `HOST:` 前缀指向 **DSH-GO 仓的 `origin/main`**（2026-11-18 取证时本地 checkout 落后
> 35 个提交，agent 终端生产者只存在于 `origin/main`；本文所有 HOST 行号都按
> `git show origin/main:<path>` 核过）。

## 0. 结论表

| # | 条目 | 现在 | 判据 |
|---|---|---|---|
| ① | Agent / 沙箱徽标、控制权、`typed` 语义 | **已闭合**（宿主产 → 壳接管 → 插件画，三段都有测试） | 见 §1；`cmd/app/terminal_agent_test.go` + `plugins/dsterminal/panel_flow_badge_test.go` |
| ② | 芯片双击改名 | **已闭合** | 见 §2；`internal/uikernel/view_tabs_rename_test.go` + `cmd/app/terminal_shortcut_test.go` + `plugins/dsterminal/rename_test.go` |
| ③ | VT 面（光标 / 调色 / selection / scrollback） | **仍缺（本质差异，不是 Viva 限制）** | 见 §3；缺 VT 解析器 + 单元格网格 |
| ④ | `resize`（`write` 同理） | **仍缺（客户端没有官方 mux 上行）** | 见 §4；`internal/apiclient` 没有 `/api/remote.mux` 客户端 |
| ⑤ | `TerminalGuide` 向导卡 | **部分闭合（2/3：图标 + 快捷键条）** | 见 §5；菜单载体仍是手绘占布局面板——Viva 不向插件导出 Popup，`ui.Menu` 又把阴影硬编码成 `gfx.ShadowMD`（Viva 限制，outbox 草案 `vivaui-viva-popup-menu-shadow.md`） |
| ⑥ | 快捷键 / `.status` 折行 | **a 已闭合、b 已接；仍无快捷键注册表** | 见 §6；`ui.Flow` 双向断言；`Ctrl+Backquote` 已进窗口级 chord |

## 1. ① Agent / 沙箱徽标（已闭合）

### 1.1 官方那一支

```text
OFF:dsh-client-ui-sidebar-terminal/src/client/TerminalTitle.tsx:17-20,44-50
  agentControl = info.origin === 'agent' && info.control === 'agent'
  sandboxMode  = info.origin === 'agent' ? info.sandboxMode : undefined
  agentControl → <Tag tone="info">Agent</Tag>（title = agentHint [+ 沙箱提示]）
  sandboxMode  → <Tag tone="neutral">沙箱</Tag>（title = readOnlyHint / workspaceWriteHint）
  一次只有一枚：Agent 优先，沙箱是「被 Agent 打开、还没被接管」那一档
OFF:dsh-client-ui-sidebar-terminal/src/client/index.ts:103-112     openAgentTerminals
  origin !== 'agent' || state !== 'running' || isRepresented → skip
  openTabIn(sessionId, 'terminal', { params: { terminalId: info.id } })
OFF:dsh-client-ui-sidebar-terminal/src/client/index.ts:116-126     closeEndedAgentTerminals
  受控集合（origin==='agent' && control==='agent'）上一轮有、本轮没了 → 关掉绑定到它的 tab
OFF:dsh-api-terminal-controller/src/client/index.ts:104-119        ClientTerminals.view
  saved  = terminalId ?? bindings.get(sessionId, contentId)
  id     = saved ?? randomUUID();  create = saved === undefined
```

### 1.2 三段链的现状

| 段 | 状态 | 证据 |
|---|---|---|
| 宿主**产** `origin=agent`/`control=agent`/`sandboxMode` | 已落地（DSH-GO `origin/main` #64/#65/#66） | `HOST:cmd/dshd/terminal_pty_agent.go:191-193` 建 entry 时显式写三项；`HOST:internal/transport/handlers_job_terminal_official.go:266-282` 的 `aligned()` 只给**空值**补 `user`，绝不覆盖已有值、不替 user 终端编 `sandboxMode` |
| 宿主**通** `typed`（控制权移交） | 已落地 | `HOST:internal/transport/handlers_terminal.go:46,61-65` 把 `typed` 交给 `TerminalInputScoped`；`HOST:cmd/dshd/terminal_pty_agent_test.go` 的 `TestAgentTerminalTypedTakeoverBlocksAgentClose` 钉住 agent→user 的单向移交 |
| 壳**拿**（Agent 终端变成可见 tab、按 `terminalId` 接管） | 本轮补齐 | `cmd/app/terminal_wiring.go:565` `syncAgentTerminals`（开/关两支）、`:241` `terminalHostID`（`params.terminalId` 优先）、`:662` `releaseTerminalWatchesExcept`；`cmd/app/main.go` 的 `openSession` 起停 watch |
| 插件**画**（官方 `Tag`） | dsh-ui#145 已落地 | `plugins/dsterminal/guide.go:145` `RightTabBadge`；壳在 `cmd/app/right_tab_occurrences.go` 按帧问 `RightTabBadgeProvider`，`internal/uikernel/view_tabs.go:576` 在标题**之前**画 |

两个关键点：

1. **roster 流跟着"被展示的会话"起停**。官方的 `AgentTerminalOpener` 是个只返回 null
   的组件，挂在 `conversation.session.header.actions` 上，会话头显示期间才有 watch
   （`OFF:…/index.ts:128-143`）；壳没有组件卸载这一层，所以在 `openSession` 里显式
   `releaseTerminalWatchesExcept` + `ensureTerminalWatch`。**没有这一步，Agent 终端
   永远不会有 tab**——原来的 watch 只在已经有终端 tab 时才起（鸡生蛋）。
2. **接管的 tab 不能再 create**。原来 `runTerminalView` 找不到 roster 里同 id 的终端就
   `terminal.create{ID: tabID}`，于是"接管"实际变成"新开一条 user 终端"，徽标永远
   显示不出来。现在 `terminalHostID` 优先读 `params.terminalId`；找不到那条终端时进
   `missingTerminal` **空态**（官方 `terminal.tsx:36-45`），绝不用 user 终端冒充。

### 1.3 测试与变异

- `cmd/app/terminal_agent_test.go`
  - `TestAgentTerminalRosterOpensTabThatAdoptsTheHostTerminal`：roster 出现 agent 终端 →
    铸 occurrence 且 `params.terminalId` 为该 id → `attach` 用**那个** id →
    `terminal.create` **一次都没被调** → 快照里 `origin/control/sandboxMode` 三项与宿主回包一致。
  - `TestAgentTerminalTabClosesWhenTheAgentEndsIt`：受控终端离开 roster → tab 与视图缓存都消失。
- `plugins/dsterminal/panel_flow_badge_test.go::TestRightTabBadgeReadsShellSnapshot`：
  壳给的快照 → 官方 `Tag`（Agent 档 hint = agentHint + 沙箱提示；沙箱档 neutral）。
- 变异（实测变红后还原）：`terminalHostID` 恒 `return tabID` → 主用例红
  （"接管路径调了 terminal.create"）。

### 1.4 本条仍有的边界（不改变"徽标已闭合"的判定）

- 界面侧的键盘级 `typed` 仍然只有 F6 的"发命令"一条路径（`plugins/dsterminal/terminal.go:109-112`，
  真实人员手势 → `typed=true`）。xterm 级别的 onData → typed 属 §3 的 VT 面，不在本条。
- 沙箱两档在宿主上"没有 argv 包装器就如实 501"（`HOST:cmd/dshd/terminal_pty_agent.go` 头部注释）：
  徽标能显示 `sandboxMode`，但那种终端本身起不来。这是宿主沙箱层的既定边界，不是徽标数据面的缺口。

## 2. ② 芯片双击改名（已闭合）

```text
OFF:dsh-client-ui-sidebar-terminal/src/client/TerminalTitle.tsx:2
  「Render the terminal name, editable in place on double-click.」
OFF:…TerminalTitle.tsx:25-36
  chip = label.closest('[data-dockkit-tab], [data-dockkit-float-grip]')
  chip.addEventListener('dblclick', () => { cancelled.current = false; setEditing(true) })
OFF:…TerminalTitle.tsx:51-59
  editing ? <input className={css.name} maxLength={120}
                   onBlur={next = value.trim(); !cancelled && next !== '' && next !== title && void view(tab.id).rename(next)}
                   onKeyDown={Enter → blur; Escape → cancelled = true; blur} />
          : <span className={css.title}>{title}</span>
OFF:…TerminalTitle.module.css:2
  .name{width:120px;min-width:48px;max-width:100%;padding:0 4px;color:inherit;font:inherit;
        background:var(--dsw-alias-bg-l1);border:0.5px solid var(--dsw-alias-border-l3);
        border-radius:var(--dsw-radius-sm);outline:none}
```

本仓落地（契约 → 壳 → 插件三层）：

| 层 | 落点 | 说明 |
|---|---|---|
| 契约 | `pluginapi/plugin.go:593` 的 `RightTabRenameProvider`；`:1571` 的 `Actions.RenameRightTab` | 前者带 `kind`，`owned=false` = 不认领（同 `RightTabCloseHandler` 约定）；后者无返回值，与官方 `void view(...).rename(next)` 同形 |
| 壳 | `internal/uikernel/view_tabs.go:373` `DrawRightTabsEditable` | `.name` 几何逐值取 `TerminalTitle.module.css:2`；`DoubleClicked()` 进编辑态；回车/失焦提交（trim、空串与同值不提交） |
| 壳 | `cmd/app/right_tab_rename.go:29/71` | 把 kind 交给认领的插件、失败进状态栏；`cancelRightTabRename` 处理 Esc |
| 插件 | `plugins/dsterminal/guide.go:410` | `RightTabRename` → `Actions.TerminalRenameOfficial` → 宿主 `terminal.rename` |

两个只有踩过才知道的点：

1. **Esc 归壳**。viva 把 Esc 交给窗口的 `OnKey`，不送输入框的 `OnKey`；而那一刻必须先把它
   从插件手里截下来，否则一次"取消改名"会顺手拒掉一条审批（`dscomposer` 的 Esc = `reject_once`）。
2. **改名框必须登记进 `ui.Fields`**。`app.Config.TextInput` 只认登记过的 `*ui.Field`
   （`viva@v0.6.4/ui/elements.go:301-315`）；壳的 `fields` 列表原来只装 `m.draft` 与插件字段，
   漏了壳自己持有的 `m.rightTabRename.Field` 就是"能进编辑态、一个字打不进"。

测试：`internal/uikernel/view_tabs_rename_test.go`（双击进编辑态、回车提交/trim/空串不提交、
`.name` 几何、壳没接动作时不画输入框、取消=End 不提交）；`cmd/app/terminal_shortcut_test.go`
的 `TestCancelRightTabRenameSwallowsEscape`；`plugins/dsterminal/rename_test.go`
（kind 归属、标题透传、宿主错误原样回错）。
变异：去掉 `DoubleClicked()` 分支 → 双击/提交三组用例全红。

## 3. ③ VT 面（仍缺，但**不是 Viva 限制**）

缺的是整块模拟器，不是漏一两处渲染：

| 官方面 | 出处在官方 | 本仓 |
|---|---|---|
| 光标四形态（block/outline/bar/underline）+ 1s step-end blink | `terminal.module.css:12-19`、`terminal-cursor.ts` | 无 |
| ANSI 16/256 调色、OSC 10/11/12 | `terminal-theme.ts:22-50` | 无（`internal/ansi` 只剥 CSI/OSC） |
| selection（拖选 + 复制） | xterm 原生 | 无 |
| scrollback / rows 自适应 | xterm `scrollback` + `environment.maxRows` | 只有 18 行可读文本窗口（`plugins/dsterminal/terminal.go` 的 `maxLines`） |
| 真实 VT 解析（SGR/DEC 私有模式/alt screen/鼠标上报/宽字符） | xterm | 无 |

**为什么不判成 Viva 限制**（v0.6.4 源码逐条核过）：

| 旧账/前任的说法 | 实际 | 出处 |
|---|---|---|
| "缺 Viva 容器 wrap（`flex-wrap`）" | `ui.Flow()` 就是会换行的 Row，语义与 CSS `flex-wrap` 一致，`LineGap` 单独设行距 | `viva@v0.6.4/ui/frame.go:533-543`、`website/content/guides/zh/declarative-layout.md:36` |
| "双击没有判据" | `Box.DoubleClicked()`（`gfx.ClickInfo.Clicks >= 2`） | `viva@v0.6.4/ui/box.go:547-552` |
| "打开编辑框后没法聚焦" | `Box.AutoFocus()`：首次出现的那一帧拿到键盘 | `viva@v0.6.4/ui/style.go:151-172` |
| "`overflow-wrap:anywhere` 做不到" | `WrapLines` 对超过整行的 token 会按 `/ \-_.&?=` 折，再不行按字素簇折 | `viva@v0.6.4/gfx/wrap.go:14-19` |
| **"Viva 没有门户层"（前任新写，说法不准确）** | 框架**内部**有 `Popup()` 与完整的 `ui.Menu`；但**插件**拿不到 `Popup`，`ui.Menu` 的阴影又写死 `gfx.ShadowMD`。真实的 Viva 限制是"插件级 popup + 可换阴影"这一条（见 §5.1） | `viva@v0.6.4/ui/frame.go:507-517`、`ui/page.go:72-119`、`components/dropdown_menu.go:772-774` |

**唯一真的缺口**：viva 只有**输入框内**的选区（`ui.Field`），画出来的 `ui.Text` 没有只读
文本选区。但终端是**等宽网格**，选区可以在网格上自己算（行列 → 矩形），高亮与复制都由
插件做，不依赖 Viva 的选区能力。

**什么条件下能做**：① 引入一个 Go 的 VT 模拟器（解析 SGR/DEC 私有模式/alt screen/鼠标上报/
半格宽），把 `TerminalViewState.Screen` 从"剥完 ANSI 的字符串"换成单元格网格；② 用网格渲染
（`Mono()` + 固定 13px 行盒，现有实现已经是这个档）；③ 光标 blink 用 `time.Ticker` +
`Actions.Invalidate`（官方 `@keyframes` 1s step-end 的等价物）；④ 选区/复制在网格上算。
这是一整块独立工作量，不能靠壳层小修完成。

## 4. ④ `resize` / `write` 上行（仍缺：客户端没有官方 mux）

```text
HOST:cmd/dshd/serve_terminal_api.go:602                     officialTerminalAttachStream.Push
  收 {type:'write'|'resize', sequence, data/cols/rows}，校验 sequence 连续、只有最新 attach
  持有输入权，resize 调 api.Resize（PTY resize 已在）
HOST:internal/transport/handlers_job_terminal_official.go:467   OfficialTerminalUplink
  上行接口——只有 mux 面能拿到（HTTP 一条请求一条响应，没有上行通道）
OFF:dsh-api-terminal-controller/src/client/model.ts:244-252
  resize(cols, rows) → send(attachment, { type:'resize', sequence: attachment.sequence+1, cols, rows }, 0)
```

本仓：`internal/apiclient` 只有**会话事件** WS（`internal/apiclient/mux.go`，路径 `/ws`），
**没有 `/api/remote.mux` 客户端**（`grep -rn "remote.mux" internal/apiclient/` 零命中），
所以 `TerminalStream` 只有 `Next/NextRoster/NextRetained`，没有 `Send`。
**不许为了它自造 `terminal.resize`**：官方没有这个方法，自造就等于两套 wire。

`write` 的现状：走 HTTP `terminal.input`（宿主 additive 的投影，`internal/apiclient/terminal.go:128-132`），
`typed` 已透传；这是本仓在 mux 落地前的临时上行，不是官方通道。`resize` 没有对应
additive 方法，因此完全缺。

**什么条件下能做**：写官方 typert mux 客户端（WS `/api/remote.mux` + open/ack/uplink 帧 +
256 KiB 上行配额 + 单 socket 约束），然后把 `write` 与 `resize` 一起搬上去。这是一整块
独立工作量，与 ③ 无关，两者可以并行。

## 5. ⑤ `TerminalGuide` 向导卡（部分闭合：2/3）

官方卡片与它的三段外围（`TerminalGuide.tsx:32-83`）：

- 图标：`icon: PluginArtworkTerminal`（`index.ts:74`）→ 36×36 viewBox、两条
  `stroke="#679EFE" stroke-width="3.5"` 的 path（`dsh-web-frontend/dist/assets/index-ghOpC9uV.js`
  的 `xK` 组件）。
- 菜单：`Menu ... portal autoFocus align="end"`（`TerminalGuide.tsx:62-77`），四态
  loading / ready / failed+retry / empty（`:50-54`）。
- 快捷键条：`{shortcut !== undefined && shortcut.keys.length > 0 && <ShortcutKeys ... />}`（`:81`），
  plain 档 `._keys_38b9q_1{gap:3px;font-size:12px;line-height:16px;color:label-tertiary}`，
  每个元素一枚 `<kbd>`（bundle 的 `Sp`）。

本轮补齐的两处：

| 处 | 落点 | 说明 |
|---|---|---|
| 图标素材 | `plugins/dsassets/terminal.go` | `PluginArtworkTerminal()` 两条 path 逐字来自官方 bundle；卡片 `plugins/dsterminal/guide.go:175` 的 `guideIcon()` 按官方 26px 画，素材解析失败才退回旧 16-box 字形 |
| 快捷键条 | `plugins/dsterminal/guide.go:242` `drawGuideShortcut` + `pluginapi.Actions.ShortcutKeys` + `cmd/app/terminal_shortcut.go:117` | 键序 `{"Ctrl","+","反引号"}` 来自官方 `terminal.new` 默认值（`index.ts:57-63`）；`"+"` 单独成元素（bundle `Sp` 的 separator） |

### 5.1 菜单载体仍是差异（**Viva 限制**，已写 outbox 草案）

官方 `Menu` 是 portal 到窗口层的原语。Viva v0.6.4 的两个入口都不合用：

1. **`frame.Popup()` 不导出到 `ui.C`**：`Popup()` 是 `*frame` 的方法
   （`viva@v0.6.4/ui/frame.go:507-517`），`ui.Ctx` 只导出
   `Key/KeyInt/Part/After/Theme/Bounds/Canvas/ID/PopupLayer`（`ui/page.go:72-119`）；
   插件声明 `ui.Popup()` 是编译错误（实测 `undefined: ui.Popup`，见 §5.2 复现 A）。
2. **`ui.Menu` 的菜单面阴影硬编码 `gfx.ShadowMD`**
   （`viva@v0.6.4/components/dropdown_menu.go:772-774`），没有 `Look`/`Options`
   覆盖口。用它就等于把官方 `--dsw-elevation-prominent` + `--dsw-alias-border-l1`
   退回 Tailwind 预设——`plugins/dscore` 的阴影守卫会红（正确）。

所以本仓保留手绘面板，并**用官方令牌画阴影**
（`dscore.PaintElevation(…, Shadows.Prominent, BorderL1)`，`plugins/dsterminal/guide.go`
的 `drawGuideMenu`），代价是：① 面板作为卡片列的子元素会占布局，打开时把下面内容
往下推；② 没有 `ui.Menu` 自带的方向键/外部点击关闭/Esc。

**解除条件**：Viva 导出 `Popup`（或给 `Menu` 一个阴影/描边覆盖口）后，把面板换成
popup 载体，行为与几何即可一并按官方补齐。复现与 issue 草稿：
`$DSH_HOME/outbox/vivaui-viva-popup-menu-shadow.md`（**未发上游**）。

### 5.2 复现（实测）

```go
// tmp/GO8-RC2/_t54f/viva-popup-repro/main.go —— 在 dsh-ui 工作树里 go build
func view(f ui.C) {
	anchor := ui.Row(f).Width(20).Height(20)
	_ = ui.Popup().BelowBox(anchor, 4, ui.End) // undefined: ui.Popup
}
```

```text
$ go build ./tmp/GO8-RC2/_t54f/viva-popup-repro/
tmp/GO8-RC2/_t54f/viva-popup-repro/main.go:14:9: undefined: ui.Popup   (exit 1)
```

### 5.3 测试

`plugins/dsterminal/guide_test.go::TestTerminalGuideCardDrawsOfficialShortcutKeys`、
`TestOfficialTerminalGuideGeometry`、`TestTerminalGuideMenuDrawsShellStates`；
`cmd/app/terminal_shortcut_test.go::TestShortcutKeysCatalog`；
`plugins/dsassets/terminal_test.go`（36 / 3.5 / 两条 path / #679EFE）。

**仍有的非阻塞差异**：官方 `aria-keyshortcuts={shortcut?.aria}`（`TerminalGuide.tsx:56`）；
Viva 的无障碍树没有这个属性口，本仓只在视觉上画键位。

## 6. ⑥ 快捷键与 `.status` 折行

### 6.1 `.status{flex-wrap:wrap}`（已闭合）

```text
OFF:dsh-client-ui-sidebar-terminal/src/client/terminal.module.css:5
  .status{display:flex;flex-wrap:wrap;align-items:center;gap:8px;padding:5px 12px;
          background:var(--dsw-alias-bg-l2)}
```

落点 `plugins/dsterminal/terminal.go:209`：`ui.Row` → `ui.Flow`（viva 的 flex-wrap 等价物）。
证据 `plugins/dsterminal/panel_flow_badge_test.go::TestTerminalStatusWrapsLikeOfficialFlow`：
180px 宽里状态文字与按钮**折行**（差 ≥ 一行），720px 宽里**同一行**（差 < 一行）——双向判据。
变异回 `ui.Row` 即红。

### 6.2 `terminal.new` = `Ctrl+Backquote`（已接）

```text
OFF:dsh-client-ui-sidebar-terminal/src/client/index.ts:55-70
  ctx.shortcuts.register({ id:'terminal.new', label: t('new'), aliases:['new terminal','shell'],
    defaults: { 'desktop:macos'|'desktop:windows'|'desktop:linux'|'web:macos'|'web:windows':
                { code:'Backquote', modifiers:['control'] } },   ← web 没有 linux 档
    regions:['page','editable','terminal'], modals: [],
    resolve: target === undefined ? { status:'blocked', reason: t('shortcut.noSession') }
                                  : { status:'handled', run: () => ctx.sidebarRight.openTabFromTarget('terminal', target) } })
OFF:…/locales.ts:9   'shortcut.noSession': '请先选择会话'
```

落点 `cmd/app/terminal_shortcut.go:54`（handler）与 `:97`（`shortcutHandler` =
`app.Config.OnShortcut` 的唯一实现，与 `Ctrl+O` 同一层：viva 只把带修饰键的可打印键送到
`OnShortcut`，`viva@v0.6.4/app/app.go:130-142`）。`plugins/dsshortcuts` 的键位表加了这一行。

**仍无快捷键注册表**（官方 `ctx.shortcuts.catalog`）：官方的
`regions:['page','editable','terminal']` 与 `modals:[]` 两档门控落不了地——本仓对任何焦点
都响应（等价于官方三档的**并集**，不会漏、只可能多），也没有"模态打开时不响应"这一条
（壳没有模态登记表，设置浮层是 `plugins/dssettings` 自己的绘制状态）。

### 6.3 其它仍然不同的登记项（本轮未动）

- `inputFull` / `invalidOutput` / `terminalLimit` 三档只有文案，没有触发点（壳多数路径仍塞自由文本）。
- 空态里那枚 36×36 终端插画未画（只存在于官方产物，无可登记素材）。
- `close` 语义：官方 handler 同步 void + 后台重试队列（`TerminalCleanup.tsx` 那一组），本仓返回
  error 阻止关 tab。
- `retainTabs` 的全量保命面未接（官方 `index.ts:34-39` 的 `window-holds`）。

## 7. 本轮门禁

```text
python scripts/parity-count.py --check                    PASS
CGO_ENABLED=0 go build ./...                              exit 0
go vet ./...                                              exit 0
CGO_ENABLED=0 go test ./... -count=1                      exit 0
test -z "$(gofmt -l cmd internal plugins pluginapi)"      exit 0（空）
python scripts/doc-links.py                               exit 0
python scripts/scripts-index.py                           exit 0
```
