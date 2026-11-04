# 设置浮层（最小可操作版）

> owner：SETTINGS-MINIMAL（**A 方案** continuation）· 2026-11-04 · base **`9290bed`** + 当前共享导出
> （FILE-REFS / WP-C1 / C2 / C3 / WP-D1 / STATS-WIRE / REASONING / OPEN-IN-APP）
> 口径：**只做"点了真的会发生什么"的那几行**，不做官方的 11 页设置。
> A 方案入口：**F2 命令面板 → 末行「设置」→ 真实 `dssettings.Open()`**；`Shift+F2` 保留为备用直达。

---

## 0. 一句话

**F2** 打开命令面板 → **最后一行「设置」** → Enter / 点击，弹一块卡片（`shell.overlay`）；
**Shift+F2** 保留为备用直达。两条路径调的都是同一个 `dssettings.Open()`，不是两套实现。
卡片把壳**真接上了**的设置动作摊成「外观 / 模型 / 权限 / 会话」四块：外观走
`Actions.SetTheme`（深/浅/跟随系统）、模型走 `Actions.ModelsList` + `SetModel`、
权限走 `Snapshot.Permission` + `SetPermission`（宿主白名单 deny/ask/allow/auto）、
会话 id / 标题 / cwd 只读。
**缺一个 Action 就整块不画；当前值不知道就一行都不标选中。** Esc 关闭，换主题当场生效。

---

## 1. F2 命令面板里的「设置」行（A 方案，已落地）

任务书给的是 **A 方案**：F2 命令面板末尾追加一条「设置」，不另造第二套设置面。
已按 A 落地，并且**只加了 `plugins/dscommands/commands.go` + 它的定向测试**；
设置浮层本体、`cmd/app` 装配与 `Shift+F2` 入口都是前一轮的成果，本轮一行没改。

### 1.1 接法：服务键，不是 import、不是包级注册表

`plugins/dssettings/register.go` 早就留好了 `ui.settings.open`（`Provides` + `func() bool`）：

```go
// dssettings.Register 里
ctx.Provide(ServiceKey, func() bool { return p.Open() })
```

`dscommands` 在 `Register` 里用 **comma-ok** 取这个**可选**能力（没有就保持 nil）：

```go
const settingsServiceKey = "ui.settings.open" // 与 dssettings.ServiceKey 一致（测试守着）

if fn, ok := ctx.Inject(settingsServiceKey).(func() bool); ok && fn != nil {
    p.openSettings = func() { fn() }
}
```

`commands()` 在**最后**追加；`p.openSettings == nil` 时什么都不加：

```go
if p.openSettings != nil {
    out = append(out, Command{"设置", "外观 / 模型 / 权限 / 会话", p.openSettings})
}
```

判据是**服务在不在**，不是字符串匹配、不是包级全局；`Action` 就是服务本体
（`dssettings.Open()`，与 Shift+F2 走同一条路径）。服务缺失 / 值类型不对 / typed nil
→ **零新增、零 panic**（`settings_test.go` 用四种坏值钉住）。

### 1.2 为什么必须排在最后

命令是**按位置**选的：`scripts/accept-theme.sh` 用 `F2 → ↓×9 → Enter` 选「主题：浅色」。
所以「设置」只能追加在末尾（无模型命令时是第 19 条；有模型命令时它是"最后的最后"，
前面 18 条的位置仍然一个不动）；原有 18 条的名字、提示、顺序**逐字不变**，
由 `TestNoSettingsServiceKeepsCommandTableVerbatim` 整表比对（不是只数条数）。

### 1.3 顺序与按键（两条不冲突）

- `ds-settings` 仍排在装配顺序**最前**（`cmd/app/main.go` 的实例前置 +
  `cmd/app/settings_order_test.go` 守着）：① 它的 `Register` 先跑，`ds-commands`
  后面才 `Inject` 得到 `ui.settings.open`；② 浮层开着时 `Esc` 先被它消费，
  不会漏给 composer 变成"停止 turn"。
- `ds-settings.HandleKey` **只精确匹配 `Shift+F2`**（以及浮层开着时的 Esc/↑↓/Enter）。
  普通 F2 它返回 false，命令面板照常收到——所以 **F2 主线可用**，Shift+F2 只是备用。
- 可发现性：F2 是产品主动线（用户本来就会开面板），Shift+F2 是知道的人的捷径；
  两者不互斥，也不存在"抢 F2"。

### 1.4 一个边界（写清楚，不假装没有）

命令面板只看"服务在不在"，**不复制** dssettings 的分区判据（插件之间不互相 import）。
所以"装了 dssettings、但这一帧一个真实设置动作都没有"时，行仍在、点下去 `Open()`
返回 false、不打开空卡片。产品里 `cmd/app` 恒接 `SetTheme`，这条边界不会出现；
真正的"没有可操作项"门禁仍在 dssettings 自己手里（§2 第 1 条）。

---

## 2. 入口与交互

| 操作 | 行为 | 备注 |
|---|---|---|
| `F2` | 开 / 关命令面板 | 命令面板自己的键；「设置」是面板里**最后一行** |
| F2 面板：`↓` 到末行 → `Enter`（或鼠标点「设置」） | 打开设置浮层 | 仅当 `ui.settings.open` 服务在时才有这一行；执行 = `dssettings.Open()` |
| `Shift+F2` | 开 / 关设置浮层（备用直达） | 只有 `ModifierShift` 精确匹配；普通 F2 / Ctrl+F2 一律放行 |
| `Esc` | 关闭（浮层开着时） | 关着时**放行**——那是审批拒绝 / 停止 turn 的键 |
| `↑` / `↓` | 在**可点行**之间移光标 | 只读行（会话 id 等）不占号；打开时**没有光标** |
| `Enter` | 执行光标那一行 | 没有光标时什么都不做 |
| 鼠标点行 | 执行该行 | 真实命中区（`prims.ListRow(...).Hit(id).Clicked()`） |
| 右上角 `×` | 关闭 | 28px 圆钮，和 dsdemo 同一颗 `prims.CircleButton` |

三层门禁（这是**浮层画面**的门禁；F2 面板那一行是否出现由 §1.1 的服务存在性决定），
缺一个都不出现：

1. **没有任何真实设置动作**（`SetTheme` / `SetPermission` / `ModelsList`+`SetModel` 全空）
   → Shift+F2 不响应；已经开着也会被下一帧收掉（壳中途撤能力时不留空卡片）。
2. **某一块的动作缺失** → 那一块**整块不画**（不是画成灰的）。
3. **当前值未知** → 那几行照常可点，但**没有任何一行带 ✓**（不伪造选中）。

状态切换后靠 `Actions.Notify` 请求重绘（`setStatus` 会写状态栏 + `Invalidate`）：
`HandleKey` 跑在绘制之外，改完 `shown` 不会有任何一帧自动发生——这是本仓
`dsshortcuts` / `dscommands` 都踩过的同一个坑，不是这里发明的。顺带的副作用正好
满足"操作反馈走既有状态提示"：打开写「设置已打开 · Esc 关闭」，关闭写「设置已关闭」。

---

## 3. 分区与真值来源

| 分区 | 出现条件 | 行 | 当前值来自 | 未知时 |
|---|---|---|---|---|
| 外观 | `Actions.SetTheme != nil` | 深色 / 浅色 / 跟随系统（`dark`/`light`/`system`） | `ConfigGet()["ui.theme"]`，**并用这一帧真正解析出来的 `Theme.Dark` 核对** | 一行都不选中，写「当前：未知（…）」 |
| 模型 | `Actions.ModelsList != nil && SetModel != nil` **且列表非空** | 最多 6 个模型 + 「默认模型」 | `Snapshot.Model`（宿主 `session.load` 的最近模型名） | 一行都不选中，写「当前：未知（…）」；知道但不在列表里会写「（不在可选列表里）」 |
| 权限 | `Actions.SetPermission != nil` | 只读 / 每次都问我 / 自动允许 / 按模式自动（`deny`/`ask`/`allow`/`auto`）+ 恢复默认 | `Snapshot.Permission` | 一行都不选中，写「当前：未知（…）」；值不在白名单里原样显示「（不在界面认识的四档里）」 |
| 会话 | `Snapshot.Cur != ""` | id / 标题 / 工作目录 —— **全部只读** | `Cur`、`Sessions[].Title`、`CWD` | 缺哪个就不画哪一行（不写"未知"占位） |

几条刻意的判断：

- **主题的"当前"为什么要核对 `Theme.Dark`**：`-theme light` 是显式启动参数，会压过
  落盘的 `ui.theme`（`cmd/app/theme.go` 的 `themeExplicit`）。这时盘上留着旧值，
  直接拿它当"当前选中"就是伪造。只有"配置说是深/浅 **且** 这一帧真的是那个明暗"
  才标 ✓；`system` 不用核对（解析成深或浅都对）。
- **模型行回传的是 `models.list` 的原样 id**（带渠道前缀），去前缀只发生在**显示**上；
  同名不同渠道时保留完整 id。点「默认模型」回传空串 = 清除覆盖、回到宿主 `--model`。
- **权限只有宿主白名单里的四个词** + 空串（清除覆盖）。界面**不自己算**权限、
  不拿会话 mode 顶替（`Snapshot.Permission` 的契约就是这么写的）。
- **「恢复默认」不参与选中态**：快照只说"当前是什么"，说不出"是不是覆盖出来的"。
- **模型列表上限 6**：真宿主的 `models.list` 是几十条，浮层不是列表页；超出的部分
  **不画成死行**，只在 Note 里写「另有 N 个未列出」。列表为空（壳没拉到）时整块不画
  ——只画一个「默认模型」会让人以为可选模型只有一个。

---

## 4. 不画什么

- 不画官方那 11 页（没有宿主侧的配置面，抄页数就是空壳）。
- 不画"栏宽/快捷键/隐私/关于"这些**没有动作**的东西。
- 不画死按钮：每一行背后都是一个非 nil 的 `Actions.*`；缺就整块消失。
- 不写伪配置：本插件**从不调用 `ConfigSet`**。主题的落盘是壳的 `SetTheme` 自己做的
  （`ui.theme`），插件不重复写、不发明新键。

---

## 5. 焦点 / 快捷键为什么不会和 composer、@ 菜单打架

1. **不实现 `Fields()`**：不注册任何 `flex.Field`，文本焦点永远在 composer。
   打开设置之后继续打字，字进的是输入框（和 F2 命令面板同一层行为）。
   @ 候选菜单的判定只看草稿文本，本插件不碰草稿。
2. **按键精确匹配**：
   - 普通 F2 / Ctrl+F2 / Ctrl+Shift+F2 → `false`（命令面板的键，一个字节不碰）
   - 关着时的 Esc / ↑ / ↓ / Enter → `false`（审批拒绝、停止 turn、发送、侧栏切会话全照旧）
   - 带修饰键的方向键 / Enter → `false`（Ctrl+Shift+C 这类组合留给别的插件）
3. **装配顺序**：`cmd/app/main.go` 把 `ds-settings` **排在插件列表最前**
   （`plugins = append([]pluginapi.Plugin{m.settings}, plugins...)`）。
   `kernel.HandleKey` 按装配顺序转发、第一个返回 `true` 的赢——排在最前，浮层开着时
   Esc 才会先关浮层，而不是被 `dscomposer` 当成"停止当前 turn"。
   这条由 `cmd/app/settings_order_test.go` 守着：`ds-settings` 必须排在
   `ds-composer` / `ds-commands` / `ds-sidebar` 之前，否则测试直接点名。
   同一个顺序也是 F2「设置」行的前提：`ds-settings.Register` 里的 `Provide`
   必须先于 `ds-commands.Register` 里的 `Inject`（`uikernel.Load` 按列表顺序装配
   满足依赖的插件，实测装配序仍是 `… ds-shell ds-settings ds-commands …`）。

---

## 6. 证据

### 6.1 单测（`plugins/dssettings`，`go test ./plugins/dssettings/`）

| 测试 | 钉住什么 |
|---|---|
| `TestNoSettingsActionsMeansNoEntryAndNoPanel` | 没有设置动作 → Shift+F2 不响应、`Open()` 返回 false、一块都不画 |
| `TestPlainF2AlwaysFallsThrough` | 普通 F2 / Ctrl+F2 / Ctrl+Shift+F2 一律放行 |
| `TestShiftF2OpensAndEscapeCloses` | 开 → 关；关着时 Esc 必须放行 |
| `TestMissingActionHidesWholeSection` | 5 种缺 Action 的组合，各自只出该出的块 |
| `TestThemeRowsCarryTheirOwnValue` | 三个主题行的闭包各带各的值（不是循环变量那个经典坑） |
| `TestThemeSelectionOnlyFromTruth` | 8 种配置 × 解析结果组合：不匹配/没有/不认识的词 → 一行都不选中 |
| `TestModelRowsAreTheRealIDs` | 回传原样 id；重名保留渠道；默认 = 空串 |
| `TestModelSelectionOnlyFromSnapshot` | 知道才选中；不知道不选；不在列表写清 |
| `TestModelListCappedWithoutDeadRows` | 超上限不画死行，只写「另有 3 个未列出」 |
| `TestPermissionRowsCarryHostWhitelistValues` | 五条各带各的值（含空串） |
| `TestPermissionSelectionOnlyFromSnapshot` | 未知 / 不在白名单 → 不选中并说明 |
| `TestSessionSectionIsReadOnly` | 会话行不可点；缺 cwd/title 就不画那一行 |
| `TestKeyboardMovesOnlyBetweenClickableRows` | 关闭时放行；打开时打开态 -1 光标、回绕、Enter、组合键放行 |
| `TestOpenRequiresRealActions` / `TestShortModelKeepsChannelForDuplicates` | 入口门禁 / 显示名规则 |
| `TestSettingsOverlayDoesNotFightComposerKeys`（视觉文件内） | 浮层开着时也只吃 Esc/↑↓/Enter |
| `TestSettingsTextHasNoFakeCurrentState` | 文本里不许出现编出来的「当前：深色/浅色」 |

**先红后绿（实测）**：这组测试钉的是**规格**（先定"旧实现上必须红"，再写实现）。
旧实现（base `9290bed`）里根本没有 `plugins/dssettings` 包；为了证明失败在**行为**上
而不是"包不存在"，把同一份测试放进 base 的分离工作树，配一个"设置不存在"的替身
（`HandleKey` 永远 false、`Open()` false、`BuildSections` 返回 nil），跑了同一条命令：

```
git worktree add --detach <tmp>/dsh-ui-red-check 9290bed
cp plugins/dssettings/settings_test.go <tmp>/dsh-ui-red-check/plugins/dssettings/
# 再加一个同样接口、空行为的 stub.go
cd <tmp>/dsh-ui-red-check && CGO_ENABLED=0 go test ./plugins/dssettings/
```

结果 **exit 1，28 条 FAIL**（`tmp/settings-red.log`），失败点全部是行为断言：

```
--- FAIL: TestShiftF2OpensAndEscapeCloses
--- FAIL: TestMissingActionHidesWholeSection/只有主题   （外观分区不在）
--- FAIL: TestThemeRowsCarryTheirOwnValue               （点哪个都收不到值）
--- FAIL: TestThemeSelectionOnlyFromTruth/…             （一个 ✓ 都没有）
--- FAIL: TestModelRowsAreTheRealIDs / TestModelSelectionOnlyFromSnapshot / …
--- FAIL: TestPermissionRowsCarryHostWhitelistValues / PermissionSelectionOnlyFromSnapshot
--- FAIL: TestSessionSectionIsReadOnly / TestKeyboardMovesOnlyBetweenClickableRows
--- FAIL: TestOpenRequiresRealActions
```

换成当前实现后同一条命令全绿（`ok github.com/MYCF711/dsh-ui/plugins/dssettings`）。
工作树用完即删（`git worktree remove --force`），不留在仓库里。

### 6.1b F2「设置」行单测（`plugins/dscommands`，`go test ./plugins/dscommands/`）

| 测试 | 钉住什么 |
|---|---|
| `TestSettingsServiceAppearsLastAndIsExecutable` | 服务在 → 命令表 19 条（本组脚本动作没有模型命令）、**末行**「设置」、提示含四块；直接执行两次 = 服务被调两次；再走真实 `F2 → ↓ 到末行 → Enter`，服务被调第三次且面板收起 |
| `TestNoSettingsServiceKeepsCommandTableVerbatim` | 服务缺失 → 仍 18 条，**名字 + 提示逐字**等于基线；原有 `NewSession` / `Cancel` 动作照旧被调到 |
| `TestMalformedSettingsServiceIsIgnored` | `string` / `int` / typed-nil `func() bool` / `func()` 四种坏值 → 不 panic、零新增，其余命令不变 |

服务键防漂移：测试里直接比对 `settingsServiceKey == dssettings.ServiceKey`
（生产代码仍不 import 任何插件，只有测试这么比）。

**先红后绿（实测）**：在 base `9290bed` + 上一轮设置 patch 的分离工作树里，放一份
只验行为的红测试（不引用新字段，所以能编译；失败点必须是**行为**而不是编译）：

```
git worktree add --detach <tmp>/dsh-ui-settings-a-redcheck 9290bed
git -C <tmp>/dsh-ui-settings-a-redcheck apply settings-min-final.patch
cp <红测试> <tmp>/dsh-ui-settings-a-redcheck/plugins/dscommands/settings_red_test.go
cd <tmp>/dsh-ui-settings-a-redcheck && CGO_ENABLED=0 go test ./plugins/dscommands/ \
  -run TestRedSettingsRowIsLastAndExecutable
```

结果 **exit 1**（`tmp/settings-a/red.log`），失败点是行为：

```
settings_red_test.go:35: F2 面板末行不是「设置」（服务已提供）：
[新建会话 … 压缩上下文 分支这个会话]
--- FAIL: TestRedSettingsRowIsLastAndExecutable
```

当前实现下同一组断言（并入上表主用例）全绿；工作树用完即删。

### 6.2 两尺寸真实 Paint / HitTest / Click（`plugins/dssettings/settings_visual_test.go`）

真 Viva 无头驱动 + 真 `dsshell`（走真 `shell.overlay` 槽）+ 真 `dscore` 令牌，
两个尺寸各一遍完整链路：**Paint → 真实按键 Shift+F2 → 真实 HitTest → 真实 Click → 再 Paint**。

```
SETTINGS_CAPTURE_DIR=tmp/settings-capture go test ./plugins/dssettings/ -run TestSettingsOverlay -v
```

断言（两个尺寸都过）：

- 打开前：没有「设置」卡片，也没有任何设置行命中区
- Shift+F2 之后：`设置/外观/模型/权限/会话/深色/浅色/跟随系统/恢复默认` 全部画出来，
  当前值三处（`当前：b` / `当前：ask` / `当前：未知`）都在
- **✓ 恰好 2 个**（模型 `b` + 权限 `ask`）；外观是未知态，**没有**冒充选中
- `HitTest(行中心)` 必须打回它自己的 id，再点
- 点「浅色」：`SetTheme("light")` **收到一次、值是 light**；主题对象就地换（`Dark == false`）；
  浮层不关
- 点模型：收到 `group/deepseek-v4.1`（原样 id，带渠道）；点权限：收到 `allow`
- 卡片最下面一条可点行在窗口内（没被切掉）
- Esc 之后浮层关、文本消失

**像素**（整图平均亮度，0–255）：

| 尺寸 | 打开（深） | 点浅色后 | 关闭 |
|---|---|---|---|
| 1182×832 | **32.9** | **252.6** | — |
| 1920×1032 | **27.0** | **253.8** | — |

同一个进程里只点了「浅色」一行，亮度差 ≈ 7.7–9.4 倍——这是"运行时立刻生效"的像素证据，
不是"图里有东西"。截图：`tmp/settings-capture/settings-{open-dark,open-light,closed}-{1182x832,1920x1032}.png`。

### 6.3 真进程（`cmd/app`，真窗口，窗口挪到屏幕外）

```
go build -o tmp/settings-app/app.exe ./cmd/app
tmp/settings-app/app.exe -shot tmp/settings-app/shot-keys.png -shot-wait 8 \
  -keys shift+f2 -keys-after 2 -size 1182x832
```

日志与像素：

| 场景 | 参数 | 日志 | 截图亮度 |
|---|---|---|---|
| 打开 | `-keys shift+f2` | `key 17 mods=1 handled=true` | **33.3**（`shot-keys.png`） |
| 关闭 | `-keys shift+f2,esc` | `key 17 mods=1 handled=true`、`key 4 mods=0 handled=true` | **26.0**（`shot-esc.png`，卡片确实没了） |
| 换主题 | `-theme dark -keys shift+f2,down,down,enter` | `主题：light（实际 light）` | **251.2**（`shot-light-by-key.png`） |
| 1920×1032 打开 | `-open-settings`（同一 `Open()`，不模拟热键） | `设置浮层已打开` | **27.6**（`shot-open-flag.png`） |

第三行是**产品路径**：Shift+F2 打开 → ↓↓ 落到「浅色」→ Enter → `Actions.SetTheme` →
壳排队 → 下一帧 Paint 才换 → 全窗变浅。没有宿主在跑，所以状态栏同时出现
`保存设置失败: 连不上 http://127.0.0.1:7788`——这正是"失败走既有状态提示"的样子
（本插件不另造一套反馈）。

装配顺序日志（同一次运行）：

```
插件 [ds-brand ds-core ds-shell ds-settings ds-commands ds-composer …]
```

上面那组是上一轮 `Shift+F2` 的备用入口证据，仍可复跑。**本轮 A 方案的主线证据**
（F2 面板 → 末行「设置」→ Enter → 真浮层）单独列：

| 场景 | 参数 | 日志 | 截图 / 几何 |
|---|---|---|---|
| 1182×832 开面板 | `-keys f2` | `key 17 mods=0 handled=true` | `shot-f2-1182.png`：19 条命令，**末行**「设置 / 外观 / 模型 / 权限 / 会话」；面板与空盘基线的差异 bbox `(798,96)-(1158,623)`（宽 360 = 贴右 24） |
| 1182×832 F2→设置 | `-keys f2,↓×18,enter` | `key 2 mods=0 handled=true`；侧栏 + 状态栏「设置已打开 · Esc 关闭」 | `shot-f2-enter-1182.png`：卡片 bbox `(311,104)-(871,553)`（宽 560，left=(1182−560)/2=311） |
| 1920×1032 F2→设置 | 同上，`-size 1920x1032` | 同上 | `shot-f2-enter-1920.png`：卡片 bbox `(680,104)-(1240,553)`（left=(1920−560)/2=680） |
| Esc 关闭 | `-keys f2,↓×18,enter,esc` | `key 4 mods=0 handled=true`；状态「设置已关闭」 | `shot-f2-enter-esc-1182.png`：卡片消失，与基线的差异只剩底部状态栏文字 |

几何怎么算的：同一版本、同一尺寸再抓一张**不注入按键**的 `shot-base-*.png`，
用 `ImageChops.difference`（阈值 18）取差异包围盒；卡片左右边界、560 的设计宽、
命令面板 360 宽 / 贴右 24 都是精确值。Esc 被谁消费不靠 `handled=true` 猜
（composer 接线的 Esc 也会返回 true），而是靠「设置已关闭」这行**只有
`dssettings.Close()` 会写**的反馈，加上卡片确实消失。18 个 ↓ = 从第 1 条
（光标初始在 0）走到第 19 条末尾；没有模型命令时表就是 19 条（无宿主时
`models.list` 为空，模型命令一条都不出现）。

### 6.4 全仓

```
CGO_ENABLED=0 go build ./...   # exit 0
CGO_ENABLED=0 go vet ./...     # exit 0
CGO_ENABLED=0 go test ./...    # exit 0，33 个包全部 ok（含 dscommands / panels_order 等既有守卫）
```

本轮复跑记录：`tmp/settings-a/full.log`（三段各自 `EXIT=0`）。

---

## 7. 复跑命令（一条一条抄）

```bash
cd "C:/Users/Administrator/AppData/Local/Temp/dsh-ui-settings-min-isolated"
export PATH="/c/Program Files/Go/bin:$PATH" GOPROXY=https://goproxy.cn,direct

# 1) dscommands 定向（A 方案：F2 末行「设置」；含服务缺失/坏值）
CGO_ENABLED=0 go test ./plugins/dscommands/ -run 'TestSettings|TestNoSettings|TestMalformed' -v

# 2) 设置浮层单测 + 两尺寸真实绘制（截图落在 tmp/settings-capture）
SETTINGS_CAPTURE_DIR="$PWD/tmp/settings-capture" \
  CGO_ENABLED=0 go test ./plugins/dssettings/ -v

# 3) 装配顺序守卫
CGO_ENABLED=0 go test ./cmd/app/ -run TestSettingsPluginIsInstalled -v

# 4) 全仓
CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./...

# 5) 真窗口 F2 主线：面板 → ↓×18 → Enter（两尺寸）+ Esc 关 + 基线
CGO_ENABLED=0 go build -o tmp/settings-a/app.exe ./cmd/app
DOWNS=$(printf 'down,%.0s' {1..17})down
./tmp/settings-a/app.exe -shot tmp/settings-a/shot-f2-1182.png -shot-wait 8 \
  -keys f2 -keys-after 3 -size 1182x832
./tmp/settings-a/app.exe -shot tmp/settings-a/shot-f2-enter-1182.png -shot-wait 9 \
  -keys "f2,$DOWNS,enter" -keys-after 3 -size 1182x832
./tmp/settings-a/app.exe -shot tmp/settings-a/shot-f2-enter-1920.png -shot-wait 9 \
  -keys "f2,$DOWNS,enter" -keys-after 3 -size 1920x1032
./tmp/settings-a/app.exe -shot tmp/settings-a/shot-f2-enter-esc-1182.png -shot-wait 10 \
  -keys "f2,$DOWNS,enter,esc" -keys-after 3 -size 1182x832
# 几何基线：同一版本同一尺寸，不注入按键
./tmp/settings-a/app.exe -shot tmp/settings-a/shot-base-1182.png -shot-wait 7 -size 1182x832
./tmp/settings-a/app.exe -shot tmp/settings-a/shot-base-1920.png -shot-wait 7 -size 1920x1032

# 6) 上一轮 Shift+F2 备用入口（仍可复跑）
CGO_ENABLED=0 go build -o tmp/settings-app/app.exe ./cmd/app
./tmp/settings-app/app.exe -shot tmp/settings-app/shot-keys.png -shot-wait 8 \
  -keys shift+f2 -keys-after 2 -size 1182x832
./tmp/settings-app/app.exe -shot tmp/settings-app/shot-esc.png -shot-wait 8 \
  -keys shift+f2,esc -keys-after 2 -size 1182x832
./tmp/settings-app/app.exe -theme dark -shot tmp/settings-app/shot-light-by-key.png -shot-wait 8 \
  -keys shift+f2,down,down,enter -keys-after 2 -size 1182x832
```

---

## 8. 未验证 / 已知限制

1. **真机鼠标点击没有 OS 级证据**：无头驱动里的 Press/Release/Click 是 Viva 的真实指针
   事件，但不是 `PostMessage` 到真窗口；`-shot` 路径没有鼠标注入。所以"点在真窗口的
   「设置」行上"这一半是 headless 证据 + 真进程的键盘证据，不是 OS 鼠标。
2. **真进程的按键是 `-keys` 注入**（直接调 `kernel.HandleKey` + 手动 `Invalidate`），
   不是 OS 级键盘消息；它证明的是插件路由与画面，不证明真键盘事件链。
3. **真宿主（dshd）没跑**：本轮没有起宿主，所以 `models.list` 的真列表、`Snapshot.Model`
   的真值、`session.setPermission` 的真实回包都没在真链路上看过。模型/权限分区的
   "有数据"形态由单测 + 装配了假数据的无头视觉测试证明。
4. **"无服务 → 没有「设置」行"只在单测里验过**：产品里 `ds-settings` 是内置插件且由壳
   强制实例注入（白名单也 skip 不掉），所以真进程里拍不到"没有设置行"的对照截图；
   该形态由 `TestNoSettingsServiceKeepsCommandTableVerbatim` / `TestMalformed…` 证明。
5. **F2 面板的键盘选择只验了"↓×18 到末行"**：从第 1 条一路走到第 19 条；没有单独
   截"输入关键词筛选到只剩「设置」再 Enter"的图（`filtered()` 与命令表同一份数据）。
6. **主题"当前值"的两种未知形态**（没落盘 / 与 `-theme` 不一致）只验了"不选中 + 写未知"，
   没验"用户手动改 `ui-config.json` 写了个怪值"以外的边界。
7. **多显示器 / 非 100% DPI**：只在 1.0 scale 的无头驱动与 1182×832、1920×1032 两个尺寸下量过。
8. **官方 11 页设置的外观对齐**：没做，也不在本轮范围（只借了官方卡片的圆角/底色令牌）。
9. `shell.overlay` 的 `Order=100` 是新增的一层（99 是命令面板）。`cmd/app/panels_order_test.go`
   只保证同槽不撞号，不保证"100 之上以后没人加"——以后加层的人得自己看这个数字。
10. **F2 行的门禁边界**（§1.4）：命令面板只判"服务在不在"，不判"这一帧有没有设置动作"；
    产品里 `SetTheme` 恒接所以总打得开，但"装了插件、零动作"的壳会看到一行点了不打开的
    「设置」——这是本轮刻意接受的最小接法（不 import dssettings、不复制它的判据）。

---

## 9. 文件与 rev

新增：

- `plugins/dssettings/register.go` —— 注册 + `ui.settings.open` 服务键（F2 行就是接的它）
- `plugins/dssettings/model.go` —— 纯数据模型：哪块画、每行什么值、当前值从哪来
- `plugins/dssettings/settings.go` —— 插件本体：按键、绘制、键盘光标
- `plugins/dssettings/settings_test.go` —— 15 组行为单测（不画窗口）
- `plugins/dssettings/settings_visual_test.go` —— 3 组：两尺寸真实 Paint/HitTest/Click、按键边界、文本反证
- `plugins/dscommands/settings_test.go` —— A 方案定向测试：服务在 → 末行可执行；服务缺失 / 坏值 → 零新增、其余命令逐字不变
- `docs/settings.md` —— 本文件

修改：

- `plugins/dscommands/commands.go` —— **A 方案（本轮唯一新增的写入路径）**：`Register` 用
  comma-ok `Inject` `ui.settings.open`（可选能力），`commands()` 在**末尾**追加
  `Command{"设置", "外观 / 模型 / 权限 / 会话", openSettings}`；其余 18 条命令零改动
- `cmd/app/main.go` —— 空白 import → 实例注入；`ds-settings` 排装配最前；`-open-settings`；
  `-keys` 支持 `shift+f2`（`parseKeys` 改成返回 `ui.KeyEvent`，否则修饰键会被丢掉）
- `cmd/app/main_test.go` —— `TestParseKeys` 跟着改，并新增 `shift+f2` 的断言
- `cmd/app/settings_order_test.go`（新）—— 装配顺序守卫

未改动：`apiclient` / `pluginapi` / `internal/*` / `plugins/dssettings`（本轮 A 一行没动）
/ 其他所有插件 / 共享契约 / 宿主。
