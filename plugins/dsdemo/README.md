# 写一个第三方插件（10 分钟）

> 这份说明的每一句都对应仓库里**能跑**的代码：本例就是 `plugins/dsdemo/demo.go`，
> 它挂在 `shell.overlay`（浮层槽）上，F12 或点「×」收起。
> 你写的插件和内置插件走的是**同一条装配路径**，没有"内部 API"这种东西。

## 1. 十行骨架

```go
package hello

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
)

type Plugin struct{ theme *pluginapi.Theme }

func (*Plugin) ID() string         { return "hello.counter" }        // 全局唯一
func (*Plugin) Provides() []string { return nil }                    // 不给别人提供能力
func (*Plugin) Requires() []string {                                 // 只说它要什么
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	return nil
}

func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{ID: "hello", Slot: pluginapi.SlotSidebarFooter, Order: 90, Draw: p.draw}}
}

func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	f.Text("hello").FontSize(p.theme.Fonts.Caption).Color(p.theme.LabelSecondary)
}
```

壳里只多一行：`&hello.Plugin{}`（放进 `kernel.Load(...)` 的参数表）。
装配器按 `Requires` 拓扑排序，`ds-core` 一定先就位，所以 `Inject` 不会返回 nil。

## 2. 能挂的槽位（照抄 `pluginapi/plugin.go`，共 12 个）

| 常量 | 值 | 位置 | 现在谁在用 |
|---|---|---|---|
| `SlotShellRoot` | `shell.root` | 整个窗口的根 | `ds-shell`（框架插件自己占，第三方不要挂） |
| `SlotShellLeading` | `shell.leading` | 左上角座（macOS 红绿灯旁） | 空 |
| `SlotShellBottom` | `shell.bottom` | 横跨三列的整行 | `ds-statusbar` |
| `SlotShellOverlay` | `shell.overlay` | 覆盖整个框架的浮层 | **`ds-demo`（本样例）** |
| `SlotSidebarHeader` | `sidebar.header` | 左栏顶 | `ds-sidebar` |
| `SlotSidebarList` | `sidebar.list` | 左栏主体 | `ds-sidebar` |
| `SlotSidebarFooter` | `sidebar.footer` | 左栏底 | `ds-sidebar`、`hello.counter` |
| `SlotMainHeader` | `main.header` | 中栏顶 | `ds-topbar` |
| `SlotMainBody` | `main.body` | 中栏主体 | `ds-conversation` |
| `SlotMainComposer` | `main.composer` | 中栏底（输入区） | `ds-composer` |
| `SlotRightbar` | `rightbar` | 右栏 | `ds-rightbar` |
| `SlotStatusbar` | `statusbar` | 旧版状态栏槽（已由 `shell.bottom` 取代） | 空 |

同一个槽里按 `Painter.Order` 从小到大画。想在自己的盒子里继续开子槽位，
用 `Painter.Children`（见 `ds-shell`：它在 `shell.root` 里切出三栏，再把 frame 交给下一层）。

## 3. 两个服务键

| 键 | 值类型 | 谁提供 | 为什么几乎每个插件都要 |
|---|---|---|---|
| `pluginapi.KeyTheme`（`"ui.theme"`） | `*pluginapi.Theme` | `ds-core` | 颜色/圆角/字号全在这里。**插件里不许写十六进制色号**——写死一处，换主题就漏一块 |
| `pluginapi.KeyPrimitives`（`"ui.primitives"`） | `pluginapi.Primitives`（接口） | `ds-core` | 行、分组头、徽标、圆点、分隔线、28px 圆钮。形状问它，别自己拼尺寸 |

取值都是**类型断言**（`ctx.Inject(...).(*pluginapi.Theme)`），键写错就是启动即 panic——
这是故意的：宁可启动失败，也不要静默少画一块。

## 4. 三条最容易踩的坑

1. **`f.Bounds().Width` 在槽位里是整窗宽，不是本槽宽度**（实测 1180）。
   要按像素定宽只能用 `env.MainWidth`（壳算好的主栏宽度）；否则内容会画到窗口外，
   实测两次：气泡跑到右上角、工具卡的长命令画到右栏上。
2. **列表行不要用 `Height(x).Shrink(0)`**：整行会塌成 0 高、什么都没画出来（分组头正常、
   会话行全没了）。用 `MinHeight` + `PadY`。
3. **交互 id 一律 `env.Tree.ID(env.Feat, "名字")`**，不要写数字；`Hit(id, "")` 的标签
   **必须留空**只当点击区，传标题会让它自己再渲染一遍、和下面的 Text 打架。

## 5. 验收

```bash
export PATH="/c/Program Files/Go/bin:$PATH"
CGO_ENABLED=0 go build ./...            # 硬前提：不能引入 cgo（viva 靠 purego）
CGO_ENABLED=0 go vet ./plugins/dsdemo/
gofmt -l plugins/dsdemo/                # 必须空

# 无头看效果（窗口会挪到屏幕外，抓完自动退出）
./tmp/app.exe -addr http://127.0.0.1:7788 -shot docs/x.png
```

启动日志里应该出现你的 ID：

```
INFO 插件已装配 id=ds-demo provides=[] requires="[ui.theme ui.primitives]"
```
