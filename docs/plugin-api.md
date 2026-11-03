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

5. **多个代理并行写同一个库会互相踩**：两个代理同时改 `cmd/mockd` 的同一批文件时，
   会出现「刚写的文件被删、对面写的文件语法坏掉」的来回，实测代价十几分钟。
   **并行只适合一人一个插件目录**；同一个文件（尤其 `cmd/*/main.go`）必须串行。

6. **`AfterDraw` 里画东西时，裁剪区必须是「自己的视口」，不是 `env.Bounds`**（这条最贵，
   因为它看起来完全正常，只在某些内容高度下暴露）：
   - `Paint` 的 `Clip` 用整窗 = 等于不裁。排在视口外面的 Markdown 会画到别处去：
     短会话滚到底时**顶栏整条被消息盖住**（截图里「顶栏消失」），宽表格会**横穿右栏**。
   - 正确做法：在 `Draw` 阶段把 `f.ScrollArea(...)` 返回的 `flex.Box` **存起来**，
     到 `AfterDraw` 再 `Rect()`——那时候布局已完成，拿到的是**视口矩形**。
     实测 `view=(251,286 489x382)`，而 slot 自己的 `Y` 可能是负数（滚出视口上方）。
   - 两个边界都要用视口：`Y=view.Y`、`Height=view.Height`、
     `Width=view.X+view.Width-r.X`（**不能用 `env.Bounds.Width`**）。
   - 兜底：拿不到视口（`Width<=0`）时退回整窗——宁可多画，不可不画。
7. **`shell.overlay` 一旦没人画，插件就是「装了但看不见」**。`ds-demo` 第一版就是这样：
   装配日志里有 `ds-demo`，界面上什么都没有。框架插件（`dsshell`）必须显式
   `if env.Has(SlotShellOverlay) { drawSlot(...) }`，而且要放在**最后**画（靠后才盖在上面）。
   **第三方挂槽之前先确认这个槽真的被画**——`Env.Has` 只能证明"有占用方"，
   不能证明"有人在画"。

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
| `ds-composer` | `Enter` / `Esc` | `Modifiers == 0` | 审批可见时：允许 / 拒绝；**没有审批时 `Esc` = 取消当前 turn** |
| `ds-conversation` | `Esc` | `Modifiers == 0` | 旧路径：清审批气泡（composer 装配在前，正常抢不到这个键） |
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
| `Approve` | `func(requestID, optionID string)` | ✅ | **审批的正路**：两个 id 原样转发给 `session.approve`。插件不许自己造 id |
| `Cancel` | `func()` | ✅ | 打断当前会话的 turn（`session.cancel`）；宿主对没有 turn 的调用幂等 |
| `Allow` | `func()` | ✅ | **旧接口**，只在 `Approve == nil` 时兜底（它没有 id，宿主那边无从对应） |
| `Deny` | `func()` | ✅ | 同上 |

**所以插件里调动作前必须判空**（`if env.Act.Approve != nil {...}`）。接线点在
`cmd/app/main.go` 的 `m.actions = pluginapi.Actions{...}`。

### 审批：新插件请只用 `Approve`

`Snapshot.Approval` 的字段全部来自宿主 `approval/request` 帧，**零值 = 不知道**：

| 帧键 | 字段 | 不显示的条件 |
|---|---|---|
| `data.requestId` | `RequestID` | 空 → 回传不出去，按钮必须画成不可点 |
| `data.toolCall.title` | `Title`（冒号前缀解析成 `Tool`） | 空 → 不画标题行 |
| `data.toolCall.kind` | `Kind` | 空 → 按 other 处理，**不是风险等级** |
| `data.toolCall.rawInput` | `Args` | 空 → 回退 `Cmd` → 都空不画参数行 |
| `data.toolCall.content[].content.text` | `Detail` | 空 → 回退 `Reason` |
| `data.options[]` | `Options` | 空 → 按钮点不动（**不许编 optionId**） |

三条硬规则（真机踩过）：

1. **`requestId` 必须原样回传**，界面不许造；
2. **本地气泡消失不等于已批准**——只有 `Approve` 回 200 且 `accepted` 才算；
3. **宿主给几个选项就画几个**：`allow_once` / `allow_always` / `reject_once` 三个时
   只画两个，用户就永远点不到「这个会话里以后都允许」（`planApproval` 为此专门
   留了 `AllowAlways` 槽）。

**注意 optionId 的两种写法**：宿主代码用连字符（`allow-once`），H13 文档用下划线
（`allow_once`）。所以**按 `kind` 匹配，别按 id 字面量硬编码**。

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

## 12. 写一个第三方插件（10 分钟）

> 完整说明在 **`plugins/dsdemo/README.md`**（含骨架代码、槽位表、服务键、三条坑、验收命令）。
> 这一节只讲"为什么它算证据"。

`plugins/dsdemo/` 是一个**最小但完整**的插件：`ds-demo`，`Requires(ui.theme, ui.primitives)`，
挂 `SlotShellOverlay`，右上角一个小浮层，`F12` 收起（实现 `KeyHandler`）。

它证明的是三件事，缺一条都不成立：

1. **不碰壳**：`cmd/app` 里只多了一行 `&dsdemo.Plugin{},`（加 import），没有第二个改动。
2. **不碰内核**：`internal/uikernel` 与 `internal/pluginapi` 一行没动；它只 import 公开的
   `pluginapi` 和 viva。
3. **只靠声明就出现**：`Panels()` 里写 `Slot` + `Order` + `Draw`，装配器就把它画在那块位置上。

内嵌（`plugins/dsdemo`）和仓库外（`../dsh-ui-plugins/hello`，独立 module + `replace`）
两种接法**走的是同一条装配路径**——这就是「一切皆插件」在界面线上的落地形态：
不需要改壳、不需要注册表、不需要在别处声明"我存在"。

验收（无头，不弹窗）：

```bash
CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go vet ./plugins/dsdemo/
./tmp/app.exe -addr http://127.0.0.1:7788 -shot /tmp/demo.png   # 看右上角浮层
```

装配日志应出现：

```
INFO 插件已装配 id=ds-demo provides=[] requires="[ui.theme ui.primitives]"
```

## 13. 编译期外部插件：目录扫描 + 代码生成

> 2026-11-03 实测。这一节只讲**编译期**接入：把仓库外 module 弄进二进制。
> 它替掉了以前的两步手工活 —— 手改 `cmd/app` 的 import、手改 `go.mod` 的 require/replace。

### 13.1 配置形状（向后兼容）

`ui-plugins.json` 多了一个 `dirs`，老的 `{"plugins":[…]}` 一个字都不用改：

```json
{
  "plugins": [{"id": "ds-demo", "enabled": true}],
  "dirs": ["../dsh-ui-plugins"]
}
```

两个字段各管一段，互不影响：

| 字段 | 谁读 | 管什么 |
|---|---|---|
| `plugins[]` | 运行时 `internal/uiplugins.Load` | 装哪些**已经编译进二进制**的插件（白名单 / `enabled:false`） |
| `dirs[]` | 编译期 `cmd/plugingen` | 去哪些**父目录**里找外部 module、生成 blank import |

`dirs` 的解析规则（写死，别猜）：

- 每一项**相对配置文件所在目录**解析；配置路径为空时相对当前工作目录。
- 绝对路径原样用。
- 所以同一句 `"../dsh-ui-plugins"`，配置放仓库根是对的，放 `tmp/` 下就要写成 `"../../dsh-ui-plugins"`：

  ```json
  {"dirs":["../../dsh-ui-plugins"]}      // 配置在 tmp/ 下（本机实测用的就是这份）
  {"dirs":["D:/.../dsh-ui-plugins"]}     // 绝对路径，等价
  ```
- 目录不存在或不是目录 → **报错并列全所有缺的路径**（不会静默跳过）：

  ```
  plugingen: 配置里列的插件目录不存在或不是目录（1 个）:
    D:\DeepSeek Harness\default-workspace\dsh-ui\dsh-ui-plugins
  （dirs 里每一项都相对配置文件所在目录解析：D:\DeepSeek Harness\default-workspace\dsh-ui\tmp）
  ```

### 13.2 插件目录的认定规则（写死在这里）

扫的是每个 `dirs` 项的**直接子目录**，父目录本身不参与。一个子目录算插件，要同时满足：

1. 目录里有 `go.mod`（module 根包 = 该 module 的 import 路径）；
2. 根包 import 了公开契约包 `github.com/MYCF711/dsh-ui/pluginapi`；
3. 可选 `plugin.json` 覆盖第 1、2 条的推断：

   ```json
   {"id": "hello.counter", "module": "example.com/dsh-hello", "package": "hello"}
   ```

   - 没有 `plugin.json`：module 路径取 `go.mod` 的 `module` 行，包名从源码解析。
   - 有 `plugin.json`：`module` / `package` 以它为准（`package` 与源码包名不符 → 报错）。
   - `id` 只是给日志和文档用的标签；真正的 id 由插件 `init` 里的 `pluginapi.Register` 决定。

不满足的子目录**不报错**，只打一行原因（父目录里躺 README、脚本、半成品是常态）：

```
  跳过 gen              没有 go.mod
  跳过 notplug          根包没有 import github.com/MYCF711/dsh-ui/pluginapi
  + plug             module=example.com/scratch-plug package=scratchplug  (go.mod)
  + plug2            module=example.com/plug-two     package=plugtwo       (plugin.json)
```

同一个 module 出现在两个插件目录 → 报错（否则就是重复注册，装配时必崩）。

### 13.3 命令

```bash
go run ./cmd/plugingen -config <ui-plugins.json> [-o cmd/app/plugins_gen.go] [-mod <dir>] [-dry-run]
```

| flag | 默认 | 说明 |
|---|---|---|
| `-config` | 空 | 只读它的 `dirs`；空 = 不接任何外部插件 |
| `-o` | `cmd/app/plugins_gen.go` | 生成文件路径 |
| `-mod` | 从 `-o` 所在目录向上找 `go.mod` | 要改的 `go.mod` 所在目录 |
| `-dry-run` | false | 只打印计划：不写文件、不改 `go.mod` |

两件事都**幂等**：

- 生成文件内容相同 → 「内容相同，未重写」（构建缓存不白掉）。
- `go.mod` 只补缺的 `require` / `replace`；已有的一律不动。`replace` 已存在但指向别处 → 只提醒，**不覆盖**（那是人的决定）。
- 补的时候走 `go mod edit -require=<mod>@v0.0.0 -replace=<mod>=<绝对路径>`，路径一律正斜杠，含空格时 Go 自己加引号：

  ```
  replace example.com/scratch-plug => "D:/DeepSeek Harness/.../plugengen-scratch/plug"
  ```

`-dry-run` 的真实输出（有外部插件时）：

```
== plugingen ==
配置      tmp/ui-plugins.json
插件目录  D:\DeepSeek Harness\default-workspace\dsh-ui-plugins
  + hello            module=example.com/dsh-hello package=hello  (go.mod)
生成文件  cmd/app/plugins_gen.go
go.mod    D:\DeepSeek Harness\default-workspace\dsh-ui\go.mod
  example.com/dsh-hello
    = require 已存在 v0.0.0
    = replace 已存在 => ../dsh-ui-plugins/hello

--- 生成的 cmd/app/plugins_gen.go（dry-run，未写入）---
// Code generated by cmd/plugingen. DO NOT EDIT.
package main

import (
	_ "example.com/dsh-hello"
)

结果: dry-run，什么都没改
```

### 13.4 生成文件的角色

`cmd/app/plugins_gen.go` 是**壳唯一的「外部插件」入口**，一行手写的 import 都不该有：

```go
// Code generated by cmd/plugingen. DO NOT EDIT.
package main

import (
	_ "example.com/dsh-hello"
)
```

- 没有外部插件时生成一个**空的 import 块** `import ()` —— 语法合法、照样编译。
- **入库的默认就是这一份空的**：别人 clone 下来、不跑生成器，`CGO_ENABLED=0 go build ./...` 也能过（不依赖 `../dsh-ui-plugins` 存在）。要接外部插件就自己跑一次生成器，然后重新编译。
- 生成器写文件用 LF（`os.WriteFile` 写 `\n`，Windows 上也不会变 CRLF），内容过 `go/format`，所以 `gofmt -l` 永远干净。
- 别手改它：下次生成直接覆盖。

`cmd/app/main.go` 里因此**没有**外部插件的 import 了 —— §11 里那句「壳里只多一行 `&hello.Plugin{}`」是旧办法，现在是「壳里一行都没有」。

### 13.5 验收（三个场景，可复现）

```bash
export PATH="/c/Program Files/Go/bin:$PATH"
cd "D:/DeepSeek Harness/default-workspace/dsh-ui"
```

**A. 有外部插件**（配置在 `tmp/`，所以相对路径要多退一层）：

```bash
echo '{"dirs":["../../dsh-ui-plugins"]}' > tmp/ui-plugins.json
go run ./cmd/plugingen -config tmp/ui-plugins.json -dry-run   # 先看计划
go run ./cmd/plugingen -config tmp/ui-plugins.json
cat cmd/app/plugins_gen.go                                   # 里面应有一行 _ "example.com/dsh-hello"

CGO_ENABLED=0 go build ./... && go vet ./...
CGO_ENABLED=0 go build -o tmp/app-b2.exe ./cmd/app
./tmp/app-b2.exe -addr http://127.0.0.1:7788 -tail 60 -shot tmp/gen-on.png -shot-wait 5
```

启动日志里必须出现**外部插件的 id**（这是「真的装配进来了」唯一的证据）：

```
INFO 插件已装配 id=hello.counter provides=[] requires="[ui.theme ui.primitives]"
插件 [ds-core ds-shell hello.counter ds-composer ds-demo …]，主题 ds-dark
```

**B. 无外部插件**（回到入库默认）：

```bash
echo '{"dirs":[]}' > tmp/ui-plugins-empty.json
go run ./cmd/plugingen -config tmp/ui-plugins-empty.json   # 生成 import ()
CGO_ENABLED=0 go build ./cmd/app && echo B_OK
```

**C. 目录不存在**：`dirs` 里写一个不存在的路径，必须**非零退出**且错误里带上那个路径（见 §13.1 的报错原文）。

**D. go.mod 自动接入**（第一次接一个全新 module 时才会用到，可用一次性 scratch module 验）：
在 scratch 里 `go.mod` 只写 `module`，跑一次生成器，应看到 `+ require … （已添加）` / `+ replace … （已添加）`；
再跑一次应变成 `= require 已存在` / `= replace 已存在` 且 `go.mod` 一个字节都不变。

### 13.6 边界：只管编译期

- 本节解决的是「**怎么让一个仓库外 module 进二进制**」。它不加载 `.so/.dll`、不做进程隔离、也不改变装配顺序（顺序仍由 `Requires` 拓扑排序决定）。
- **运行时声明式插件是另一条独立机制**（不归本节管，也不由本节生成任何文件）。两者不互相替代：
  编译期插件是「写 Go、编译进来」，声明式插件是「不重新编译、运行时声明部件」。一个插件作者该选哪条，取决于他能不能改这个仓库的 `go.mod`。
- `dirs` 改了**必须重跑生成器并重新编译**；`plugins[]` 改了不用（它是运行时读的）。这两条别混。
- 一个容易踩的坑：**blank import 只在包自己有 `init` 注册时才有效**。`../dsh-ui-plugins/hello` 一度只有 `Plugin` 类型、没有 `init`，`_ "example.com/dsh-hello"` 是空操作，装配日志里根本没有 `hello.counter`。它现在有一份 `register.go`：

  ```go
  func init() { pluginapi.Register("hello.counter", func() pluginapi.Plugin { return &Plugin{} }) }
  ```

  第三方插件必须自带这一段（和 `plugins/dsdemo/register.go` 一模一样），否则接进去了也不会出现。

## 14. 运行时声明式插件：写一个 JSON 就多一块，不重新编译

> 2026-11-03 实测。§13 解决「怎么把一个 **Go 写的**插件弄进二进制」；本节解决**不写 Go、不重新编译**的那条路：
> 往一个目录里放 `hello.json`，重启界面就多一行。两条通道互不替代，也别混用同一种心理模型。

| | §13 编译期插件 | §14 运行时声明式插件 |
|---|---|---|
| 作者写什么 | 一个 `go.mod` + Go 源码 | 一个 `*.json` |
| 什么时候生效 | 生成器 + 重新编译 | 重启界面（或改完目录里的文件再重启） |
| 能不能有逻辑 | 能（任意 Go） | **不能**：只读、只画、无交互、无状态 |
| 改一次的成本 | 编译一次 | 存个文件 |

实现在两个包里，分工别找错地方：

| 路径 | 职责 |
|---|---|
| `internal/uidecl` | JSON 形状、校验、模板变量、四种画法（纯逻辑，可单测，不 import 内核） |
| `plugins/dsdecl` | 一个普通插件：`init` 里 `pluginapi.Register("ds-decl", …)`，`Register()` 时扫目录，`Panels()` 把所有声明摊平成 `[]pluginapi.Painter` |

`cmd/app` 里为它只多了一行 `_ "github.com/MYCF711/dsh-ui/plugins/dsdecl"` —— 和 `plugins/dsdemo` 同级的一次性成本，**之后再加声明部件，Go 一行都不用改**。

### 14.1 JSON 形状（一个文件 = 一个部件）

```json
{
  "id": "hello.badge",
  "slot": "sidebar.footer",
  "order": 90,
  "kind": "dot",
  "text": "外部 JSON 插件 · {sessions} 个会话"
}
```

| 字段 | 必填 | 缺省 | 说明 |
|---|---|---|---|
| `id` | ✅ | — | 全局唯一；跨文件重复 → 报错（并指出是哪两个文件） |
| `slot` | ✅ | — | 必须是契约里的槽位**字符串值**（`sidebar.footer`）或**常量名**（`SlotSidebarFooter`）；不认识 → 报错并列出全部可用槽位 |
| `order` | | `100` | 同一槽位里小的先画（= 贴在上面）。写 `0` 就是 0，不是"没写" |
| `kind` | | `label` | `label` / `keyvalue` / `dot` / `list` |
| `text` | ✅ | — | 主内容，支持模板变量（见 14.3）。`list` 里也可以直接带换行 |
| `label` | | — | **只给 `keyvalue`**：左侧标签。其它 kind 写了它 → 报错（不静默忽略） |
| `items` | | — | **只给 `list`**：多行内容，优先于 `text` |
| `color` | | 按 kind | 见 14.4；不认识 → 报错并列出可用颜色 |
| `height` | | `22`（`list` 由内容撑开） | 这一块的高度（像素）；负数 → 报错 |
| `comment` | | — | 允许写备注，不生效也不报错 |

两种画法的完整样子：

```json
{"id":"more.kv","slot":"rightbar","kind":"keyvalue","label":"当前会话","text":"{cur_title}","color":"label-primary"}
{"id":"more.list","slot":"main.header","kind":"list","items":["a · {sessions} 个会话","b · {messages} 条消息"],"color":"label-caption"}
```

`height` 缺省值 `22` 不是拍脑袋：它等于侧栏底部那行（`dssidebar.drawFooter`）的高度，混在一起才对得齐。

### 14.2 四种 kind 的画法（写死，别期待更多）

| kind | 长什么样 | 默认颜色 |
|---|---|---|
| `label` | 一行文字 | `label-secondary` |
| `keyvalue` | 左标签 + `Spacer` 顶开 + 右值；空间不够时**被省略的永远是右值**（标签 `Shrink(0)`） | 值用 `label-primary`，标签固定 `label-tertiary` |
| `dot` | 6px 状态圆点（`prims.Dot`）+ 一行文字 | 圆点 `success`；**文字固定 `label-secondary`** |
| `list` | 每行 18px、行距 2；空行整行跳过（不占高度） | `label-secondary` |

规矩和内置插件完全一样：字号取 `Theme.Fonts.Caption`，圆角/圆点问 `prims`，颜色问 `theme` —— **JSON 里没有写十六进制色号的入口**，这是故意的（见 §5）。

### 14.3 模板变量（只有这五个）

| 变量 | 值 | 取不到时 |
|---|---|---|
| `{sessions}` | 会话总数 | `0` |
| `{cur_title}` | 当前会话标题 | 空串 |
| `{messages}` | 当前会话消息数（壳给的 `Total`；为 0 时退化成 `len(Msgs)`） | `0` |
| `{status}` | 壳的状态栏文字 | 空串 |
| `{tokens}` | 上下文 token 数（`Usage.TotalTokens`，按 `k`/`M` 压缩，和状态栏同一档） | 空串（**`Usage.Known()` 为假时一个字都不显示**，不写 `0` 假数字） |

两条硬规则：

1. **认不出来的变量一律换成空串**，绝不把 `{xxx}` 原样漏到界面上。界面上出现花括号，用户只会以为界面坏了，而且不知道坏在哪。
2. `{{` / `}}` 是转义的字面量 `{` / `}`，要写 JSON 片段这类文字得这么写。

### 14.4 颜色（只有这八个）

`label-primary` `label-secondary` `label-tertiary` `label-caption` `success` `warn` `info` `error`

分别映射到 `pluginapi.Theme` 的 `LabelPrimary` / `LabelSecondary` / `LabelTertiary` / `LabelCaption` / `Success` / `Warn` / `Info` / `Error`。写别的（`green`、`#ff0000`）→ 报错。

### 14.5 报错语义：凡是「写了却不生效」都炸出来

这类配置通道最典型的坑是「我以为挂上了」。所以下面这些**一律**在装配期报错，壳会 `log.Fatalf`，错误信息里**一定带文件名**：

- 文件读不了 / JSON 坏 / 缺 `id` / 缺 `slot` / 缺 `text`；
- 槽位、`kind`、颜色不认识（并列出可用值）；
- 字段写错（`"tetx"` 这类）：`DisallowUnknownFields`，直接报 `unknown field`；
- `label` 用在非 `keyvalue`、`items` 用在非 `list`（写了却不会生效）；
- 两个文件抢同一个 `id`。

实测两条原文（`tmp/app-b1.exe`，非零退出）：

```
插件装配失败: uikernel: 插件 ds-decl 注册失败: …\tmp\decl-bad\typo.json: 未知槽位 "sidebar.foter"；可用槽位: main.body, main.composer, main.header, rightbar, shell.bottom, shell.leading, shell.overlay, shell.root, sidebar.footer, sidebar.header, sidebar.list, statusbar
```

```
插件装配失败: uikernel: 插件 ds-decl 注册失败: …\tmp\decl-bad\typo.json: JSON 解析失败: invalid character '}' looking for beginning of object key string（可用字段: id/slot/order/kind/text/label/items/color/height/comment）
```

**唯一的「静默」是目录不存在**（返回 0 个部件、0 个错误）——绝大多数机器根本不会建这个目录，为它报错等于每次启动都吓唬用户一次。

### 14.6 目录与环境变量

| | |
|---|---|
| 环境变量 | `DSH_UI_DECL_DIR`（`uidecl.EnvDir`） |
| 默认值 | `<os.UserConfigDir()>/dsh-ui/ui-plugins.d`，Windows 上就是 `%APPDATA%\dsh-ui\ui-plugins.d` |
| 扫什么 | 该目录下的 `*.json`（大小写不敏感；子目录、`.` 开头的文件一律跳过） |
| 顺序 | `os.ReadDir` 按文件名排序，所以画出来的顺序跨机器一致（截图对比要用） |

用环境变量而不是再加一个 `cmd/app` 的 flag：flag 表是壳的，往里加东西就等于把"加一个部件"的成本又推回代码里；环境变量在**外面**，测试 / 截图 / 多开各用各的目录。

### 14.7 验收（无头，不弹窗）

```bash
export PATH="/c/Program Files/Go/bin:$PATH"
cd "D:/DeepSeek Harness/default-workspace/dsh-ui"

mkdir -p tmp/decl tmp/decl-empty
cat > tmp/decl/hello.json <<'EOF'
{"id":"hello.badge","slot":"sidebar.footer","order":90,"kind":"dot","text":"外部 JSON 插件 · {sessions} 个会话"}
EOF

CGO_ENABLED=0 go build ./... && go vet ./...
CGO_ENABLED=0 go build -o tmp/app-b1.exe ./cmd/app

DSH_UI_DECL_DIR=$PWD/tmp/decl       ./tmp/app-b1.exe -shot tmp/decl-on.png  -shot-wait 6
DSH_UI_DECL_DIR=$PWD/tmp/decl-empty ./tmp/app-b1.exe -shot tmp/decl-off.png -shot-wait 6
```

单测（纯逻辑，CI 里裸跑）：

```bash
go test ./internal/uidecl/ ./plugins/dsdecl/
```

启动日志里每个声明一行，这是「我到底挂上了没有」的唯一权威答案：

```
INFO ds-decl: 声明部件 hello.badge: dot @ sidebar.footer order=90 ← …\tmp\decl\hello.json
INFO 插件已装配 id=ds-decl provides=[] requires="[ui.theme ui.primitives]"
```

两张截图的差异（实测 diff）：**侧栏底部多了一行**。内置那行「N 个会话 / 已连接…」被顶上去 22px，新的「● 外部 JSON 插件 · 206 个会话」落在最底；侧栏列表因为少了 22px 可视高度，行数跟着变（差异包围盒 `x[20..265] y[504..789]`）。

### 14.8 边界

- 声明部件**只读**：不接键盘、没有点击、不发请求、不持有状态。要交互就去写 Go 插件（§2 / §11），这条通道刻意不做"半个插件"。
- 它**不参与** `ui-plugins.json` 的白名单：白名单管的是"哪些编译进来的插件装不装"，声明部件由 `ds-decl` 这一个插件统一挂。要关掉它，就在白名单里 `{"id":"ds-decl","enabled":false}`。
- 声明目录里放一万个 JSON 就是一万块画，谁也不拦你；单帧代价没量过（未验证）。
- 声明部件之间没有顺序依赖，只有 `order`；`id` 只用于报错和日志，不用于引用。
