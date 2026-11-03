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

| 槽位 | 位置 | 谁在用 / 计划 |
|---|---|---|
| `sidebar.header` | 左栏顶部 | `ds-sidebar`（新会话 + 搜索） |
| `sidebar.list` | 左栏主体 | `ds-sidebar`（工作区分组 + 会话行） |
| `sidebar.footer` | 左栏底部 | 待移植（模型选择 / 连接状态） |
| `main.header` | 主区顶部 | 待移植（标题 + 面包屑 + 权限状态） |
| `main.body` | 主区主体 | 待移植（`ds-conversation`：消息流 + 工具卡） |
| `main.composer` | 主区底部 | 待移植（`ds-composer`：输入框 + 发送） |
| `rightbar` | 右栏 | 待移植（任务板 / 轨迹） |
| `statusbar` | 窗口底部 | `ds-statusbar`（轮次 / 步数 / 条数 / 就绪） |
| `main.header` | 主区顶部 | `ds-topbar`（标题 / 模式 / 共 N 条） |
| `rightbar` | 右侧栏（无插件时不占位） | `ds-rightbar`（工具调用时间线） |

同一个槽里按 `Painter.Order` 从小到大画（小的贴上）。

## 4. 服务键

| 键 | 值类型 | 提供者 | 说明 |
|---|---|---|---|
| `ui.theme` | `*pluginapi.Theme` | `ds-core` | 颜色 / 圆角 / 字号 |
| `ui.primitives` | `pluginapi.Primitives` | `ds-core` | ListRow / SectionHeader / Badge / Dot / Divider |

要加新键：先在 `pluginapi` 里定义常量并写进本表，再让提供者 `Provide`。

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
