# 界面线插件 API（v0）

> 这份是**移植手册**：把官方 55 个 `dsh-client-ui-*` 部件一个个搬成我们自己的插件时，
> 每个部件都照这里的形状写。改契约先改这份文档，再改代码。

## 0. 一句话

官方那 55 个界面包里，**48 个依赖同一个 `@deepseek-ai/dsh-client-ui-primitives`**，
而 `primitives` 和 `dsh-client-store` / `dsh-client-ui-slots` 都不单独存在于磁盘上
（它们被打进 `dsh-web-frontend/dist/assets/index-*.js`）。
**这就是「根本依赖插件」的官方证据**：所有部件都长在同一套 primitives 上。

我们照这个结构落：`ds-core`（令牌 + 基础组件）是根，其余部件插件只依赖它。

```
       ┌────────────── 壳 cmd/app（薄）──────────────┐
       │  装配插件 · 给槽位 · 递一帧快照              │
       └──────────────┬───────────────────────────────┘
                      │ pluginapi（契约，零实现）
       ┌──────────────┴───────────────┐
       │  uikernel（作用域/装配/依赖） │
       └──────────────┬───────────────┘
        ┌─────────────┴──────────────┐
        │ ds-core  ui.theme+primitives│  ← 根本依赖插件
        └─────────────┬──────────────┘
    ┌─────────┬───────┴────┬──────────┬───────────┐
 ds-sidebar ds-conversation ds-composer ds-statusbar …（每个部件一个插件）
```

## 1. 三个包

| 包 | 职责 | 相当于宿主线 |
|---|---|---|
| `internal/pluginapi` | 全部接口 + 数据类型，**零实现、零 viva 组件依赖** | `internal/spi` |
| `internal/uikernel` | 作用域 / 能力表 / 拓扑装配 | `internal/kernel` |
| `plugins/dscore` | 设计令牌（照抄官方 `--dsw-*`）+ 基础组件画法 | 官方 `dsh-client-ui-primitives` |

## 2. 加一个部件插件（照抄这段）

```go
package dsxxx

type Plugin struct {
    theme *pluginapi.Theme
    prims pluginapi.Primitives
}

func (*Plugin) ID() string           { return "ds-xxx" }
func (*Plugin) Provides() []string   { return nil }
func (*Plugin) Requires() []string   { return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives} }

func (p *Plugin) Register(ctx pluginapi.Context) error {
    p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
    p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
    return nil
}

func (p *Plugin) Panels() []pluginapi.Painter {
    return []pluginapi.Painter{
        {ID: "body", Slot: pluginapi.SlotMainBody, Order: 20, Draw: p.drawBody},
    }
}

func (p *Plugin) drawBody(f *flex.Frame, env *pluginapi.Env) {
    t := p.theme
    f.Text("…").FontSize(t.Fonts.Body).Color(t.LabelPrimary)
}

var (
    _ pluginapi.Plugin = (*Plugin)(nil)
    _ pluginapi.Panels = (*Plugin)(nil)
)
```

壳里只加一行：`m.kernel.Load(&dscore.Plugin{Dark: true}, &dsxxx.Plugin{}, …)`。
依赖顺序由装配器按 `Requires` 算，**不要手排**。

## 3. 槽位（插件挂在哪）

| 槽位 | 位置 | 谁在用（已按代码核实） |
|---|---|---|
| `sidebar.header` | 左栏顶部 | `ds-sidebar`（新会话 + 全部收起） |
| `sidebar.list` | 左栏主体 | `ds-sidebar`（工作区分组 + 会话行） |
| `sidebar.footer` | 左栏底部 | **目前没人用**：常量在，壳没有 `drawSlot` 调用，插件也没挂（预留给模型选择 / 连接状态） |
| `main.header` | 主区顶部 | `ds-topbar`（标题 / 模式 / 共 N 条） |
| `main.body` | 主区主体 | `ds-conversation`，**同一槽挂两个 painter**：`stream`（Order 20，`Draw` + `AfterDraw`）与 `approval`（Order 30） |
| `main.composer` | 主区底部 | `ds-composer`（输入框 + 发送） |
| `rightbar` | 右栏；**无插件时不占 300px** | `ds-rightbar`（工具调用时间线） |
| `statusbar` | 窗口底部（绝对定位 + 24px 占位行） | `ds-statusbar`（轮次 / 步数 / token / 就绪） |

壳目前只画 7 个槽位：`sidebar.header` `sidebar.list` `main.header` `main.body` `main.composer` `rightbar` `statusbar`。

同一个槽里按 `Painter.Order` 从小到大画（小的贴上）。`main.body` 就是这么把审批气泡压在消息流上面的。
一个 `Painter` 可以只有 `AfterDraw`（布局后覆画），`Draw` 与 `AfterDraw` 至少实现一个，否则装配器直接报错。

## 4. 服务键

| 键 | 值类型 | 提供者 | 说明 |
|---|---|---|---|
| `ui.theme` | `*pluginapi.Theme`（指针，**不是**值） | `ds-core` | 颜色 / 圆角 / 字号；`Dark()` 与 `Light()` 两套 |
| `ui.primitives` | `pluginapi.Primitives`（**接口**，由 `*dscore.Primitives` 实现） | `ds-core` | `ListRow` / `SectionHeader` / `Badge` / `Dot` / `Divider` |

两点按代码核实过：

- 下游插件拿它们的方式是**类型断言**，写错类型会在 `Register` 里当场 panic，不会静默降级：
  `ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)`、`ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)`。
- `ui.primitives` 故意是接口而不是 `*dscore.Primitives`：正常的下游插件**不需要 import dscore**，
  只认契约包就能画。当前唯一的例外是 `ds-conversation`——它为
  `dscore.VivaTheme(theme)` 而 import 了 `plugins/dscore`，用来算 Markdown 的
  `MarkdownLook`（那要 `ui.Theme`，不在服务键里）。要消掉这个例外，正确做法是给
  `Primitives` 加一个 `MarkdownLook()`，而不是让更多插件去 import dscore。

要加新键：先在 `pluginapi` 里定义常量并写进本表，再让提供者 `Provide`。同一个键被两个插件提供是装配错误。

## 5. 规矩（违反即返工）

1. **插件里不许出现十六进制色号**。要颜色问 `theme`，要形状问 `prims`。
   写死色号的那一刻，换主题就漏这一块——「外表要像」就断在这。
2. **不许 import `cmd/app`**，不许拿全局单例。插件只认 `pluginapi` + viva。
3. **不许自己算全局坐标**：`Draw` 拿到的 `f` 已经是槽位。要宽度用 `f.Bounds()`。
4. **不许在 `Draw` 里加锁 / 发请求**：数据是壳在锁内拷好的快照（`env.Data`）。
5. **交互 id 一律 `env.Tree.ID(env.Feat, "名字")`**，不许写死数字。
6. **列表行不要用 `Height` + `Shrink(0)`**：实测会整行塌成 0 高、什么都不画
   （见 §7 坑 1）。用 `MinHeight` + `PadY`。
7. **`Hit(id, "")` 的标签必须留空**，只当点击区；传标题会让它自己再渲染一遍。

## 6. 已移植 / 待移植

| 状态 | 部件 | 插件 ID | 官方对应包 |
|---|---|---|---|
| ✅ 已移植 | 设计令牌 + 基础组件 | `ds-core` | `dsh-client-ui-primitives`（打包内） |
| ✅ 已移植 | 左栏会话列表（工作区分组 + 折叠 + 搜索 + 相对时间） | `ds-sidebar` | `dsh-client-ui-session` + `dsh-client-ui-layout` 的 280px 列 |
| ✅ 已移植 | 主区消息流（Markdown / 工具卡 / 审批） | `ds-conversation` | `dsh-client-ui-chat` `dsh-client-ui-conversation` `dsh-client-ui-approval` |
| ✅ 已移植 | 输入区 | `ds-composer` | `dsh-client-ui-chat` 的 composer |
| ✅ 已移植 | 底部状态栏 | `ds-statusbar` | `dsh-client-ui-layout` 的状态行 |
| ✅ 已移植 | 主区顶部条（标题 / 工作区 / 模式） | `ds-topbar` | `dsh-client-ui-layout` 顶栏 + `model-selection` + `permission-presets` |
| ⏳ 之后 | 右栏轨迹 / 任务板 | `ds-rightbar` | `dsh-client-ui-plan` `dsh-client-ui-goal` |

依赖分层（官方 `client.js` 的 `require` 实测）：

- **第 0 层**：`primitives`、（`store`、`slots` 是运行时）
- **第 1 层**：48 个只依赖 primitives 的部件（approval / attachment / tool / reference / skill / subagent / settings-* …）
- **第 2 层**：`chat`、`conversation`、`commands`、`deliverables`、`message-feedback`、`agent-preset`（依赖 store + primitives）
- **第 3 层**：`layout`（把上面这些摆成整页）

所以移植顺序必须是：**ds-core → 第 1 层 → 第 2 层 → layout 的壳**。反过来做一定返工。

## 7. 坑（都真踩过）

1. **`f.Row().Height(34).Shrink(0)` 会整行塌掉**：分组头（`Height(26)`，没有 Shrink）正常，
   会话行（`Height(34).Shrink(0)`）全部不显示，只有选中那一行因为换了 `Background` 才露出来。
   改成 `MinHeight(34).PadY(6)` 立刻正常。**没有高度就用 MinHeight + 内边距**。
2. **插件初始化必须发生在 `newConfig(m)` 之前**。把装配代码插错位置（插到了
   `refreshSession` 里）会导致 `m.side == nil`，`newConfig` 里解引用直接 panic，
   窗口都起不来——现象是「进程秒退、日志里一串 nil pointer」。
3. **槽位里 `f.Bounds().Width` 与百分比都按整窗解析**（实测 1180）。要按像素定宽只能用
   `env.MainWidth`（壳算好的主区宽度 = 窗口 − 侧栏 − 右栏）。踩过两次：气泡被推到窗口右上角、
   工具卡的长命令画到右栏上。
4. **`HoverBackground` 只在悬停时着色**，非悬停的行是透明的（这是对的）。
   排查「行不显示」时不要拿它当可见性证据，先用 `Background` 验一遍布局。

4. **多个代理并行写同一个库会互相踩**：两个代理同时改 `cmd/mockd` 的同一批文件时，
   会出现「刚写的文件被删、对面写的文件语法坏掉」的来回，实测代价十几分钟。
   **并行只适合一人一个插件目录**；同一个文件（尤其 `cmd/*/main.go`）必须串行。

## 8. 验收

```bash
export PATH="/c/Program Files/Go/bin:$PATH"
CGO_ENABLED=0 go build ./...                  # 必须过

# 无头验证（推荐）：窗口一起就把自己挪到屏幕外，抓完图直接退出，用户看不到弹窗
./tmp/app.exe -addr http://127.0.0.1:7788 -tail 60   -session <id> -shot docs/<名字>.png

# 需要亲眼看手感时才起可见窗口
./tmp/app.exe -addr http://127.0.0.1:7788
```

插件装配日志会打印顺序与依赖，**顺序不对就是 `Requires` 写错了**：

```
INFO 插件已装配 id=ds-core    provides="[ui.theme ui.primitives]" requires=[]
INFO 插件已装配 id=ds-sidebar provides=[] requires="[ui.theme ui.primitives]"
```

## 9. 插件可选能力

`Plugin` 本体只有四个方法（`ID` / `Provides` / `Requires` / `Register`）。**要不要画、要不要键盘、
要不要输入框**都是额外实现一个可选接口，壳按接口探测，实现与否都合法。

| 可选接口 | 方法 | 实现它 = 什么 | 不实现 = 什么 |
|---|---|---|---|
| `Panels` | `Panels() []Painter` | 会往槽位里画东西 | 纯服务提供者（`ds-core` 就是这种） |
| `Fields` | `Fields() []*flex.Field` | 要接键盘输入 | 收不到任何字符（壳只把登记过的 `Field` 交给 viva） |
| `KeyHandler` | `HandleKey(ev ui.KeyEvent) bool` | 想抢按键 | 按键与你无关 |

### `KeyHandler` 的转发规则（按代码核实）

壳在 `cfg.OnKey` 里做三件事，**没有任何具体快捷键的判断**：

1. `ev.Release` 直接丢弃（只管按下，否则一次按键会触发两次）；
2. 调 `kernel.HandleKey(ev)`：按**装配顺序**（= 依赖顺序）逐个问插件，**第一个返回 true 的赢**；
3. 谁也不认就返回空 `EventResult`，交还系统默认处理。

现在的分工：

| 插件 | 按键 | 条件 | 动作 |
|---|---|---|---|
| `ds-sidebar` | `↑` `↓` | `Modifiers == 0` | 在**分组 + 过滤 + 折叠之后**的可见顺序里上下切会话 |
| `ds-sidebar` | `Home` `End` | `Modifiers == 0` | 跳第一个 / 最后一个 |
| `ds-conversation` | `Esc` | `Modifiers == 0` | 拒绝当前审批（**注意：`Allow`/`Deny` 还没接线，见下**） |
| `ds-conversation` | `Ctrl+R` | `ModifierControl` | 重发上一条用户消息 |
| `ds-conversation` | `Ctrl+Shift+C` | `ModifierControl\|ModifierShift` | 复制最后一条助手回复 |

**字母键的坑**：viva 只给功能键定义常量（`KeyUp` / `KeyEnter` / `KeyEscape` …），
**没有 `KeyC` 这种东西**。可打印键要靠 `ev.Text` / `ev.Typed` 判断，且 `Text` 是
"不受修饰键影响"的字符——按住 Shift 打出来的仍是 `"c"`，所以必须用 `EqualFold`
两边（`Text` 与 `Typed`）都比较。`ds-conversation` 里封了 `isKey(ev, "c")` 就是这件事。

### `Actions` 的全部字段（逐个对照 `internal/pluginapi/plugin.go`）

| 字段 | 签名 | 壳是否接线 | 说明 |
|---|---|---|---|
| `OpenSession` | `func(id string)` | ✅ | 切会话 |
| `NewSession` | `func()` | ✅ | 新建并打开 |
| `Send` | `func(text string)` | ✅ | 发出去；文字由画输入框的插件给 |
| `Copy` | `func(text string)` | ✅ | 壳调 `app.SetClipboardText`，成功后状态栏显示「已复制 N 字」 |
| `Retry` | `func()` | ✅ | 重发最后一条用户消息 |
| `Notify` | `func(msg string)` | ✅ | 让壳在状态栏说一句话（插件不直接写状态栏，那是别的插件的地盘） |
| `SetDraft` | `func(text string)` | ❌ 未接线（nil） | 没有插件在用 |
| `Allow` | `func()` | ❌ 未接线（nil） | **已知缺口**：审批气泡的「允许」按钮点了没反应 |
| `Deny` | `func()` | ❌ 未接线（nil） | 同上；`Esc` 也因为同一个原因无效 |

**所以插件里调动作前必须判空**（`if env.Act.Allow != nil {...}`）——现在这些分支不会 panic，
但会静默无效果。接线点在 `cmd/app/main.go` 的 `m.actions = pluginapi.Actions{...}`。

## 10. 调试开关（`cmd/app` 的真实 flag）

全部来自 `cmd/app/main.go` 的 `flag.*` 声明，括号里是默认值。

| flag | 类型（默认值） | 用途 |
|---|---|---|
| `-addr` | string（`http://127.0.0.1:7788`） | Transport base URL |
| `-tail` | int（40） | `session.load` 拉多少条尾巴 |
| `-limit` | int（0） | 只渲染最后 N 条（0 = 全部），量视口虚拟化收益用 |
| `-session` | string（空） | 启动后直接打开这个会话 id |
| `-title` | string（`dsh-ui`） | 窗口标题；并行跑多个实例时用来区分（截图按标题找窗口） |
| `-shot` | string（空） | **无头验证**：窗口一出现就挪到屏幕外，`PrintWindow` 抓成 PNG 后 `os.Exit(0)`，并在日志里打工作集 |
| `-keys` | string（空） | 依次注入按键（`down,up,home,end,esc`），走真实的 `kernel.HandleKey`，用来验键盘路由 |
| `-collapse` | bool（false） | 启动时把所有工作区分组收起（截折叠态） |
| `-expand-think` | bool（false） | 把思考块默认展开（截展开态） |
| `-demo` | bool（false） | 在消息列表里插假的工具卡，验渲染 |
| `-smoke` | bool（false） | 启动后自动发一条消息，验 send 链路 |

**两条边界要记住**：

- `-keys` 注入的是 `kernel.HandleKey`，**绕过了操作系统 → `cfg.OnKey` 那一段**，
  所以它只能证明"插件路由对了"，不能证明"真按键盘有用"（后者要人拿窗口试）。
- `-demo` / `-smoke` 会往会话里写东西，只在 mockd 上跑；对着真宿主别开。

## 11. 第三方插件怎么接进来（仓库外，已实测）

官方 DSH 的做法是「第三方包往预留的 slot 里一插就有」；我们是同一套：

```go
// 一个独立 module：example.com/dsh-hello
package hello

import (
    "github.com/MYCF711/dsh-ui/pluginapi"   // ← 公开契约包，不是 internal/
    "github.com/vivaui/viva/flex"
)

type Plugin struct{ theme *pluginapi.Theme; prims pluginapi.Primitives }

func (*Plugin) ID() string         { return "hello.counter" }
func (*Plugin) Provides() []string { return nil }
func (*Plugin) Requires() []string { return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives} }

func (p *Plugin) Register(ctx pluginapi.Context) error {
    p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
    p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
    return nil
}

func (p *Plugin) Panels() []pluginapi.Painter {
    return []pluginapi.Painter{
        {ID: "hello", Slot: pluginapi.SlotSidebarFooter, Order: 90, Draw: p.draw},
    }
}
```

壳里只多一行：`&hello.Plugin{}`。装配日志会多出 `hello.counter`。

### 两条硬约束（都踩过）

1. **必须用公开契约包 `pluginapi`，不能用 `internal/pluginapi`** —— Go 禁止外部 module
   import `internal/`。`internal/pluginapi` 现在是公开包的**别名层**（只有 `type X = pub.X`），
   所以第三方插件对象能直接交给 `Load`，两边是**同一组类型**，不是两份定义。
2. **槽位名是契约**（`pluginapi.Slot*` 常量）。挂在没有占用方的槽上会被静默忽略——
   想确认自己挂上了没有，看装配日志与 `Env.Has`。

样例在 `../dsh-ui-plugins/hello/`（独立 module，`replace` 到本仓），
2026-11-03 实测：装配日志 `[ds-core ds-shell … hello.counter]`，左栏底部多一行
「第三方插件：hello.counter 已接上」。
